package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"gorm.io/gorm"
)

// AuditSink is the write target for audit log entries.
// Implement this interface to fan out writes (DB, webhook, file, Kafka, …).
type AuditSink interface {
	Write(ctx context.Context, entry *models.AuditLog) error
}

// ── DBSink ────────────────────────────────────────────────────────────────────

// DBSink writes audit log entries to the database.
type DBSink struct {
	db *gorm.DB
}

// NewDBSink creates a DBSink backed by the given database.
func NewDBSink(db *gorm.DB) *DBSink {
	return &DBSink{db: db}
}

// Write persists the entry to the audit_logs table.
func (s *DBSink) Write(ctx context.Context, entry *models.AuditLog) error {
	if err := s.db.WithContext(ctx).Create(entry).Error; err != nil {
		return fmt.Errorf("audit db sink: write: %w", err)
	}
	return nil
}

// ── MultiSink ─────────────────────────────────────────────────────────────────

// MultiSink fans out writes to every registered AuditSink in order.
// The first error aborts the remaining sinks and is returned to the caller.
type MultiSink struct {
	sinks []AuditSink
}

// NewMultiSink creates a MultiSink from one or more sinks.
func NewMultiSink(sinks ...AuditSink) *MultiSink {
	return &MultiSink{sinks: sinks}
}

// Write calls Write on every contained sink.
func (m *MultiSink) Write(ctx context.Context, entry *models.AuditLog) error {
	for _, s := range m.sinks {
		if err := s.Write(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
