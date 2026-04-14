package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// PipelineLogService — Log 分塊持久化 + Log Scrubber
//
// 設計（CICD_ARCHITECTURE §7.11, ADR-012）：
//   - 每塊最大 1MB（maxChunkSize），超過時自動分塊
//   - Log Scrubber 遮蔽已知 Secret 模式，防止敏感資料外洩
//   - 讀取時按 chunk_seq 排序，支援分頁
// ---------------------------------------------------------------------------

const maxChunkSize = 1 << 20 // 1MB

// PipelineLogService 管理 Pipeline 步驟 Log 的持久化與查詢。
type PipelineLogService struct {
	db *gorm.DB
}

// NewPipelineLogService 建立 Log 服務。
func NewPipelineLogService(db *gorm.DB) *PipelineLogService {
	return &PipelineLogService{db: db}
}

// AppendLog 將 log 內容寫入 pipeline_logs，自動分塊。
// secrets 用於 scrubber 遮蔽。
func (s *PipelineLogService) AppendLog(
	ctx context.Context,
	runID, stepRunID uint,
	content string,
	secrets []string,
) error {
	if content == "" {
		return nil
	}

	// Scrub secrets
	scrubbed := ScrubSecrets(content, secrets)

	// 查詢目前最大 chunk_seq
	var maxSeq int
	err := s.db.WithContext(ctx).
		Model(&models.PipelineLog{}).
		Where("step_run_id = ?", stepRunID).
		Select("COALESCE(MAX(chunk_seq), -1)").
		Scan(&maxSeq).Error
	if err != nil {
		return fmt.Errorf("query max chunk_seq: %w", err)
	}

	// 分塊寫入
	seq := maxSeq + 1
	for len(scrubbed) > 0 {
		chunk := scrubbed
		if len(chunk) > maxChunkSize {
			chunk = scrubbed[:maxChunkSize]
		}
		scrubbed = scrubbed[len(chunk):]

		log := &models.PipelineLog{
			PipelineRunID: runID,
			StepRunID:     stepRunID,
			ChunkSeq:      seq,
			Content:       chunk,
		}
		if err := s.db.WithContext(ctx).Create(log).Error; err != nil {
			return fmt.Errorf("create log chunk %d: %w", seq, err)
		}
		seq++
	}
	return nil
}

// GetLogs 讀取指定 StepRun 的全部 Log（按 chunk_seq 排序）。
func (s *PipelineLogService) GetLogs(ctx context.Context, stepRunID uint) ([]models.PipelineLog, error) {
	var logs []models.PipelineLog
	err := s.db.WithContext(ctx).
		Where("step_run_id = ?", stepRunID).
		Order("chunk_seq ASC").
		Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("query step logs: %w", err)
	}
	return logs, nil
}

// GetLogContent 讀取指定 StepRun 的完整 Log 內容（拼接所有 chunk）。
func (s *PipelineLogService) GetLogContent(ctx context.Context, stepRunID uint) (string, error) {
	logs, err := s.GetLogs(ctx, stepRunID)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, l := range logs {
		b.WriteString(l.Content)
	}
	return b.String(), nil
}

// GetLogsSince 從指定 chunk_seq 之後讀取（用於 SSE 補發）。
func (s *PipelineLogService) GetLogsSince(ctx context.Context, stepRunID uint, afterSeq int) ([]models.PipelineLog, error) {
	var logs []models.PipelineLog
	err := s.db.WithContext(ctx).
		Where("step_run_id = ? AND chunk_seq > ?", stepRunID, afterSeq).
		Order("chunk_seq ASC").
		Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("query logs since seq %d: %w", afterSeq, err)
	}
	return logs, nil
}

// ---------------------------------------------------------------------------
// Log Scrubber — 遮蔽已知 Secret 值
// ---------------------------------------------------------------------------

// secretPattern 匹配常見 secret 格式（base64、hex、token-like）
var secretPattern = regexp.MustCompile(
	`(?i)(password|secret|token|api[_-]?key|auth)[=:]\s*\S+`,
)

// ScrubSecrets 將 content 中出現的 secret 值替換為 ***REDACTED***。
// 同時遮蔽常見 secret 模式（password=xxx, token=xxx 等）。
func ScrubSecrets(content string, secrets []string) string {
	result := content
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		result = strings.ReplaceAll(result, secret, "***REDACTED***")
	}
	// 模式比對：遮蔽 password=xxx, token=xxx 等常見格式
	result = secretPattern.ReplaceAllString(result, "***REDACTED***")
	return result
}
