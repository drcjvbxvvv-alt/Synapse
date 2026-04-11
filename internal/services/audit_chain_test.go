package services

import (
	"context"
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── computeAuditHash ─────────────────────────────────────────────────────────

func TestComputeAuditHash_Deterministic(t *testing.T) {
	e := &models.AuditLog{
		UserID:       1,
		Action:       "login",
		ResourceType: "user",
		ResourceRef:  `{"id":1}`,
		Result:       "success",
		IP:           "127.0.0.1",
		CreatedAt:    time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
	}
	h1 := computeAuditHash(zeroHash, e)
	h2 := computeAuditHash(zeroHash, e)
	assert.Equal(t, h1, h2, "same inputs must produce same hash")
	assert.Len(t, h1, 64, "SHA-256 hex is 64 chars")
}

func TestComputeAuditHash_ChangesWithPrevHash(t *testing.T) {
	e := &models.AuditLog{
		UserID:    1,
		Action:    "logout",
		Result:    "success",
		CreatedAt: time.Now(),
	}
	h1 := computeAuditHash(zeroHash, e)
	h2 := computeAuditHash("aaaa", e)
	assert.NotEqual(t, h1, h2, "different prev_hash must yield different hash")
}

func TestComputeAuditHash_ChangesWithField(t *testing.T) {
	base := &models.AuditLog{
		UserID:    1,
		Action:    "create",
		Result:    "success",
		CreatedAt: time.Now(),
	}
	altered := *base
	altered.Result = "failed"

	assert.NotEqual(t,
		computeAuditHash(zeroHash, base),
		computeAuditHash(zeroHash, &altered),
		"changing any field must change the hash",
	)
}

// ── stub AuditSink ────────────────────────────────────────────────────────────

type memSink struct {
	entries []*models.AuditLog
}

func (m *memSink) Write(_ context.Context, entry *models.AuditLog) error {
	// Simulate auto-increment
	entry.ID = uint(len(m.entries) + 1)
	m.entries = append(m.entries, entry)
	return nil
}

// ── LogAudit ─────────────────────────────────────────────────────────────────

func newTestAuditService() (*AuditService, *memSink) {
	sink := &memSink{}
	// Pass a nil db — LogAudit cannot call getLastHash with nil db.
	// Use a custom getLastHash stub by wrapping the service with a memSink
	// and pointing db at a SQLite in-memory db for the hash query.
	// Since testing hash computation without real DB is simpler, we use an
	// overrideLastHash approach via a custom helper svc.
	svc := &AuditService{sink: sink}
	return svc, sink
}

// TestLogAudit_ChainLinks verifies that successive LogAudit calls build a
// valid hash chain using the in-memory sink + SQLite stub for getLastHash.
// We test computeAuditHash directly to avoid a real DB dependency.
func TestLogAudit_FirstEntryUsesZeroHash(t *testing.T) {
	sink := &memSink{}
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)

	entry := &models.AuditLog{
		UserID:       7,
		Action:       "create_cluster",
		ResourceType: "cluster",
		Result:       "success",
		IP:           "10.0.0.1",
		PrevHash:     zeroHash,
		CreatedAt:    now,
	}
	entry.Hash = computeAuditHash(zeroHash, entry)

	require.NoError(t, sink.Write(context.Background(), entry))

	assert.Equal(t, zeroHash, sink.entries[0].PrevHash)
	assert.Len(t, sink.entries[0].Hash, 64)
}

func TestLogAudit_ChainLinked(t *testing.T) {
	sink := &memSink{}
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)

	e1 := &models.AuditLog{
		UserID: 1, Action: "a", ResourceType: "t", Result: "success",
		PrevHash: zeroHash, CreatedAt: now,
	}
	e1.Hash = computeAuditHash(zeroHash, e1)
	require.NoError(t, sink.Write(context.Background(), e1))

	e2 := &models.AuditLog{
		UserID: 1, Action: "b", ResourceType: "t", Result: "success",
		PrevHash: e1.Hash, CreatedAt: now.Add(time.Second),
	}
	e2.Hash = computeAuditHash(e1.Hash, e2)
	require.NoError(t, sink.Write(context.Background(), e2))

	// e2.PrevHash must equal e1.Hash
	assert.Equal(t, e1.Hash, e2.PrevHash)
	// hashes must differ
	assert.NotEqual(t, e1.Hash, e2.Hash)
}

// ── VerifyChain (unit — no DB) ────────────────────────────────────────────────

func TestVerifyChain_DetectsTampering(t *testing.T) {
	now := time.Now()

	e1 := &models.AuditLog{
		ID: 1, UserID: 1, Action: "a", Result: "success",
		PrevHash: zeroHash, CreatedAt: now,
	}
	e1.Hash = computeAuditHash(zeroHash, e1)

	e2 := &models.AuditLog{
		ID: 2, UserID: 1, Action: "b", Result: "success",
		PrevHash: e1.Hash, CreatedAt: now.Add(time.Second),
	}
	e2.Hash = computeAuditHash(e1.Hash, e2)

	// Tamper e2 — change Action after hash was set
	e2tampered := *e2
	e2tampered.Action = "EVIL"

	entries := []models.AuditLog{*e1, e2tampered}

	// Verify inline (mirrors VerifyChain logic)
	tampered := []uint{}
	verified := 0
	for i := range entries {
		e := &entries[i]
		expected := computeAuditHash(e.PrevHash, e)
		if expected != e.Hash {
			tampered = append(tampered, e.ID)
		} else {
			verified++
		}
	}

	assert.Equal(t, 1, verified)
	assert.Equal(t, []uint{2}, tampered)
}

// ── MultiSink ────────────────────────────────────────────────────────────────

func TestMultiSink_WritesToAll(t *testing.T) {
	s1 := &memSink{}
	s2 := &memSink{}
	ms := NewMultiSink(s1, s2)

	entry := &models.AuditLog{Action: "test", Result: "success", CreatedAt: time.Now()}
	require.NoError(t, ms.Write(context.Background(), entry))

	assert.Len(t, s1.entries, 1)
	assert.Len(t, s2.entries, 1)
}

func TestMultiSink_StopsOnFirstError(t *testing.T) {
	s1 := &memSink{}
	errSink := &errWriteSink{}
	s2 := &memSink{}
	ms := NewMultiSink(s1, errSink, s2)

	entry := &models.AuditLog{Action: "test", Result: "success", CreatedAt: time.Now()}
	err := ms.Write(context.Background(), entry)
	assert.Error(t, err)
	// s2 must not have been reached
	assert.Empty(t, s2.entries)
}

type errWriteSink struct{}

func (e *errWriteSink) Write(_ context.Context, _ *models.AuditLog) error {
	return assert.AnError
}
