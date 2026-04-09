package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
)

// SIEMService wraps SIEMWebhookConfig and audit-log export DB operations.
type SIEMService struct {
	db *gorm.DB
}

func NewSIEMService(db *gorm.DB) *SIEMService {
	return &SIEMService{db: db}
}

// GetConfig loads the (at most one) SIEMWebhookConfig row.
// Returns a zero-value config (ID == 0) when no row exists yet.
func (s *SIEMService) GetConfig(ctx context.Context) (*models.SIEMWebhookConfig, error) {
	var cfg models.SIEMWebhookConfig
	err := s.db.WithContext(ctx).First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &models.SIEMWebhookConfig{}, nil
		}
		return nil, fmt.Errorf("get SIEM config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig upserts the SIEMWebhookConfig (creates if ID == 0, saves otherwise).
func (s *SIEMService) SaveConfig(ctx context.Context, cfg *models.SIEMWebhookConfig) error {
	var op string
	if cfg.ID == 0 {
		op = "create"
		err := s.db.WithContext(ctx).Create(cfg).Error
		if err != nil {
			return fmt.Errorf("%s SIEM config: %w", op, err)
		}
	} else {
		op = "save"
		err := s.db.WithContext(ctx).Save(cfg).Error
		if err != nil {
			return fmt.Errorf("%s SIEM config: %w", op, err)
		}
	}
	return nil
}

// PushLog asynchronously forwards a single audit log to the SIEM webhook if
// SIEM is enabled. It is a best-effort operation (errors are logged, not returned).
func (s *SIEMService) PushLog(log *models.OperationLog) {
	cfg, err := s.GetConfig(context.Background())
	if err != nil || !cfg.Enabled || cfg.WebhookURL == "" {
		return
	}
	go func() {
		payload := map[string]interface{}{
			"source":       "synapse",
			"eventType":    "audit",
			"timestamp":    log.CreatedAt.UTC().Format(time.RFC3339),
			"username":     log.Username,
			"module":       log.Module,
			"action":       log.Action,
			"method":       log.Method,
			"path":         log.Path,
			"statusCode":   log.StatusCode,
			"success":      log.Success,
			"clusterName":  log.ClusterName,
			"namespace":    log.Namespace,
			"resourceName": log.ResourceName,
			"clientIP":     log.ClientIP,
		}
		b, _ := json.Marshal(payload)
		req, rErr := http.NewRequestWithContext(context.Background(), "POST", cfg.WebhookURL, bytes.NewBuffer(b))
		if rErr != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if cfg.SecretHeader != "" {
			req.Header.Set(cfg.SecretHeader, cfg.SecretValue)
		}
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		resp.Body.Close()
	}()
}

// ExportAuditLogs returns OperationLog rows filtered by optional time bounds.
func (s *SIEMService) ExportAuditLogs(ctx context.Context, start, end *time.Time) ([]models.OperationLog, error) {
	q := s.db.WithContext(ctx).Model(&models.OperationLog{})
	if start != nil {
		q = q.Where("created_at >= ?", *start)
	}
	if end != nil {
		q = q.Where("created_at <= ?", end.Add(24*time.Hour))
	}
	var logs []models.OperationLog
	if err := q.Order("created_at desc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("export audit logs: %w", err)
	}
	return logs, nil
}
