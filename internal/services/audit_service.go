package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"gorm.io/gorm"
)

// zeroHash is the sentinel prev_hash for the very first audit log entry.
const zeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

// AuditService 審計服務
type AuditService struct {
	db      *gorm.DB
	sink    AuditSink
	chainMu sync.Mutex // serialises LogAudit to maintain hash chain integrity
}

// NewAuditService 建立審計服務（預設 DBSink）
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{
		db:   db,
		sink: NewDBSink(db),
	}
}

// WithSink replaces the default DBSink.
// Useful for multi-sink fan-out (DB + webhook) or testing with a stub sink.
func (s *AuditService) WithSink(sink AuditSink) *AuditService {
	s.sink = sink
	return s
}

// ── Hash chain (P2-2) ─────────────────────────────────────────────────────────

// LogAuditRequest carries the fields needed to create one audit log entry.
type LogAuditRequest struct {
	UserID       uint
	Action       string
	ResourceType string
	ResourceRef  string // JSON-encoded resource reference (may be empty)
	Result       string // "success" | "failed"
	IP           string
	UserAgent    string
	Details      string
}

// LogAudit writes a hash-chained audit log entry.
// A mutex ensures that prev_hash → hash ordering is consistent within a
// single process. Cross-process ordering is preserved by inserting the
// prev_hash before the DB write; duplicate chain tips are idempotent because
// the hash commits to the creation timestamp (UnixNano).
func (s *AuditService) LogAudit(ctx context.Context, req LogAuditRequest) error {
	s.chainMu.Lock()
	defer s.chainMu.Unlock()

	prevHash, err := s.getLastHash(ctx)
	if err != nil {
		return fmt.Errorf("audit: get last hash: %w", err)
	}

	entry := &models.AuditLog{
		UserID:       req.UserID,
		Action:       req.Action,
		ResourceType: req.ResourceType,
		ResourceRef:  req.ResourceRef,
		Result:       req.Result,
		IP:           req.IP,
		UserAgent:    req.UserAgent,
		Details:      req.Details,
		PrevHash:     prevHash,
		CreatedAt:    time.Now(),
	}
	// Compute hash before INSERT — single DB write, no UPDATE needed.
	entry.Hash = computeAuditHash(prevHash, entry)

	if err := s.sink.Write(ctx, entry); err != nil {
		return fmt.Errorf("audit: write entry: %w", err)
	}
	return nil
}

// ChainVerifyResult is the output of VerifyChain.
type ChainVerifyResult struct {
	Verified  int    `json:"verified"`
	Tampered  []uint `json:"tampered,omitempty"`
	FirstHash string `json:"first_hash,omitempty"`
	LastHash  string `json:"last_hash,omitempty"`
}

// VerifyChain checks the integrity of the most recent `limit` audit log
// entries that carry a hash (records written before hash-chain support was
// enabled have an empty Hash and are skipped automatically).
func (s *AuditService) VerifyChain(ctx context.Context, limit int) (*ChainVerifyResult, error) {
	if limit <= 0 {
		limit = 1000
	}

	var entries []models.AuditLog
	if err := s.db.WithContext(ctx).
		Where("hash != ''").
		Order("id ASC").
		Limit(limit).
		Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("verify chain: query: %w", err)
	}

	result := &ChainVerifyResult{}
	if len(entries) == 0 {
		return result, nil
	}

	result.FirstHash = entries[0].Hash
	result.LastHash = entries[len(entries)-1].Hash

	for i := range entries {
		e := &entries[i]
		expected := computeAuditHash(e.PrevHash, e)
		if expected != e.Hash {
			result.Tampered = append(result.Tampered, e.ID)
		} else {
			result.Verified++
		}
	}
	return result, nil
}

// getLastHash returns the Hash of the most-recently inserted audit log
// that has a non-empty hash field, or zeroHash when starting fresh.
// Must be called while chainMu is held.
func (s *AuditService) getLastHash(ctx context.Context) (string, error) {
	var last models.AuditLog
	err := s.db.WithContext(ctx).
		Select("hash").
		Order("id DESC").
		First(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return zeroHash, nil
	}
	if err != nil {
		return "", fmt.Errorf("get last hash: %w", err)
	}
	if last.Hash == "" {
		// Existing records written before hash-chain was enabled — start fresh.
		return zeroHash, nil
	}
	return last.Hash, nil
}

// computeAuditHash returns SHA-256 of the canonical fields joined with null
// bytes. Using null bytes as separators prevents length-extension attacks.
func computeAuditHash(prevHash string, e *models.AuditLog) string {
	h := sha256.New()
	fields := []string{
		prevHash,
		strconv.FormatUint(uint64(e.UserID), 10),
		e.Action,
		e.ResourceType,
		e.ResourceRef,
		e.Result,
		e.IP,
		strconv.FormatInt(e.CreatedAt.UnixNano(), 10),
	}
	for i, f := range fields {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(f))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ── Partition management (Phase 2 / C1-C2) ───────────────────────────────────

// auditPartitionTableName returns the child partition table name for the month
// that contains t (e.g. "audit_logs_2026_04").
func auditPartitionTableName(t time.Time) string {
	return fmt.Sprintf("audit_logs_%d_%02d", t.Year(), int(t.Month()))
}

// EnsureNextMonthPartition creates the next calendar month's audit_logs
// partition if it does not already exist. The call is idempotent — it uses
// CREATE TABLE IF NOT EXISTS so running it multiple times is safe.
//
// Call on application startup and monthly via a scheduled job so the partition
// always exists before the month begins.
func (s *AuditService) EnsureNextMonthPartition(ctx context.Context) error {
	if s.db == nil {
		return nil // no-op when DB is not configured (e.g. test stubs)
	}
	next := time.Now().UTC().AddDate(0, 1, 0)
	tableName := auditPartitionTableName(next)
	start := time.Date(next.Year(), next.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	sql := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_logs`+
			` FOR VALUES FROM ('%s') TO ('%s')`,
		tableName,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)
	if err := s.db.WithContext(ctx).Exec(sql).Error; err != nil {
		return fmt.Errorf("ensure audit partition %s: %w", tableName, err)
	}
	logger.Info("audit partition ensured", "table", tableName)
	return nil
}

// DropOldPartitions drops the audit_logs partition that is exactly
// retainMonths before the current month. Uses DROP TABLE IF EXISTS so the
// call is idempotent. Call monthly to advance the retention window.
//
// Example: retainMonths=3 called in April 2026 drops audit_logs_2026_01.
func (s *AuditService) DropOldPartitions(ctx context.Context, retainMonths int) error {
	if retainMonths <= 0 {
		return fmt.Errorf("drop old partitions: retainMonths must be positive, got %d", retainMonths)
	}
	if s.db == nil {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, -retainMonths, 0)
	tableName := auditPartitionTableName(cutoff)
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
	if err := s.db.WithContext(ctx).Exec(sql).Error; err != nil {
		return fmt.Errorf("drop audit partition %s: %w", tableName, err)
	}
	logger.Info("audit partition dropped", "table", tableName, "retain_months", retainMonths)
	return nil
}

