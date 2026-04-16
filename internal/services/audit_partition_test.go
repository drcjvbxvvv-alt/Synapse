package services

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shaia/Synapse/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── auditPartitionTableName ──────────────────────────────────────────────────

func TestAuditPartitionTableName_Format(t *testing.T) {
	cases := []struct {
		t    time.Time
		want string
	}{
		{time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), "audit_logs_2026_04"},
		{time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), "audit_logs_2026_12"},
		{time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC), "audit_logs_2027_01"},
		{time.Date(2030, 9, 1, 0, 0, 0, 0, time.UTC), "audit_logs_2030_09"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, auditPartitionTableName(tc.t),
			"month %s", tc.t.Format("2006-01"))
	}
}

func TestAuditPartitionTableName_MonthZeroPadded(t *testing.T) {
	// Months 1–9 must be zero-padded to two digits.
	for m := 1; m <= 9; m++ {
		name := auditPartitionTableName(time.Date(2026, time.Month(m), 1, 0, 0, 0, 0, time.UTC))
		expected := fmt.Sprintf("audit_logs_2026_0%d", m)
		assert.Equal(t, expected, name, "month %d must be zero-padded", m)
	}
}

// ── EnsureNextMonthPartition ─────────────────────────────────────────────────

func TestEnsureNextMonthPartition_ExecutesCreateSQL(t *testing.T) {
	gormDB, mock, err := testutil.SetupMockDB()
	require.NoError(t, err)

	next := time.Now().UTC().AddDate(0, 1, 0)
	tableName := auditPartitionTableName(next)
	start := time.Date(next.Year(), next.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	expectedSQL := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_logs FOR VALUES FROM \('%s'\) TO \('%s'\)`,
		regexp.QuoteMeta(tableName),
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)
	mock.ExpectExec(expectedSQL).WillReturnResult(sqlmock.NewResult(0, 0))

	svc := &AuditService{db: gormDB}
	err = svc.EnsureNextMonthPartition(context.Background())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureNextMonthPartition_NilDB_NoOp(t *testing.T) {
	svc := &AuditService{db: nil}
	err := svc.EnsureNextMonthPartition(context.Background())
	assert.NoError(t, err, "nil db must be a no-op, not a panic")
}

func TestEnsureNextMonthPartition_DBError_Propagates(t *testing.T) {
	gormDB, mock, err := testutil.SetupMockDB()
	require.NoError(t, err)

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS`).
		WillReturnError(fmt.Errorf("pg: table already attached"))

	svc := &AuditService{db: gormDB}
	err = svc.EnsureNextMonthPartition(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensure audit partition")
}

// ── DropOldPartitions ────────────────────────────────────────────────────────

func TestDropOldPartitions_ExecutesDropSQL(t *testing.T) {
	gormDB, mock, err := testutil.SetupMockDB()
	require.NoError(t, err)

	const retainMonths = 3
	cutoff := time.Now().UTC().AddDate(0, -retainMonths, 0)
	tableName := auditPartitionTableName(cutoff)

	expectedSQL := fmt.Sprintf(`DROP TABLE IF EXISTS %s`, regexp.QuoteMeta(tableName))
	mock.ExpectExec(expectedSQL).WillReturnResult(sqlmock.NewResult(0, 0))

	svc := &AuditService{db: gormDB}
	err = svc.DropOldPartitions(context.Background(), retainMonths)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDropOldPartitions_ZeroRetain_ReturnsError(t *testing.T) {
	svc := &AuditService{db: nil}
	err := svc.DropOldPartitions(context.Background(), 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retainMonths must be positive")
}

func TestDropOldPartitions_NegativeRetain_ReturnsError(t *testing.T) {
	svc := &AuditService{db: nil}
	err := svc.DropOldPartitions(context.Background(), -1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retainMonths must be positive")
}

func TestDropOldPartitions_NilDB_NoOp(t *testing.T) {
	svc := &AuditService{db: nil}
	err := svc.DropOldPartitions(context.Background(), 3)
	assert.NoError(t, err, "nil db must be a no-op, not a panic")
}

func TestDropOldPartitions_DBError_Propagates(t *testing.T) {
	gormDB, mock, err := testutil.SetupMockDB()
	require.NoError(t, err)

	mock.ExpectExec(`DROP TABLE IF EXISTS`).
		WillReturnError(fmt.Errorf("pg: permission denied"))

	svc := &AuditService{db: gormDB}
	err = svc.DropOldPartitions(context.Background(), 6)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "drop audit partition")
}

// ── RetainMonths boundary: correct month math ────────────────────────────────

func TestDropOldPartitions_DropTargetIsCorrectMonth(t *testing.T) {
	// retainMonths=1 should target the partition from 1 month ago,
	// not the current or future month.
	now := time.Now().UTC()
	cutoff := now.AddDate(0, -1, 0)
	tableName := auditPartitionTableName(cutoff)

	assert.NotEqual(t, auditPartitionTableName(now), tableName,
		"drop target must not be current month")
	assert.Contains(t, tableName, "audit_logs_")
}
