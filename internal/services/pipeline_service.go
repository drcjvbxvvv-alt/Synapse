package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// PipelineService — Pipeline CRUD + 不可變版本快照
// ---------------------------------------------------------------------------

// PipelineService 管理 Pipeline 定義與版本快照。
type PipelineService struct {
	db *gorm.DB
}

// NewPipelineService 建立 PipelineService。
func NewPipelineService(db *gorm.DB) *PipelineService {
	return &PipelineService{db: db}
}

// DB 回傳底層 gorm.DB（供 handler 查詢 StepRun 等附屬資料）。
func (s *PipelineService) DB() *gorm.DB {
	return s.db
}

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// CreatePipelineRequest 建立 Pipeline 請求。
type CreatePipelineRequest struct {
	Name              string `json:"name" binding:"required,max=255"`
	Description       string `json:"description"`
	ClusterID         uint   `json:"cluster_id" binding:"required"`
	Namespace         string `json:"namespace" binding:"required,max=253"`
	ConcurrencyGroup  string `json:"concurrency_group"`
	ConcurrencyPolicy string `json:"concurrency_policy" binding:"omitempty,oneof=cancel_previous queue reject"`
	MaxConcurrentRuns int    `json:"max_concurrent_runs"`
}

// UpdatePipelineRequest 更新 Pipeline 請求。
type UpdatePipelineRequest struct {
	Description       *string `json:"description"`
	ConcurrencyGroup  *string `json:"concurrency_group"`
	ConcurrencyPolicy *string `json:"concurrency_policy"`
	MaxConcurrentRuns *int    `json:"max_concurrent_runs"`
}

// CreateVersionRequest 建立不可變版本快照請求。
type CreateVersionRequest struct {
	StepsJSON     string `json:"steps_json" binding:"required"`
	TriggersJSON  string `json:"triggers_json"`
	EnvJSON       string `json:"env_json"`
	RuntimeJSON   string `json:"runtime_json"`
	WorkspaceJSON string `json:"workspace_json"`
}

// ListPipelinesParams Pipeline 列表查詢參數。
type ListPipelinesParams struct {
	ClusterID uint
	Namespace string
	Search    string
	Page      int
	PageSize  int
}

// ---------------------------------------------------------------------------
// Pipeline CRUD
// ---------------------------------------------------------------------------

// CreatePipeline 建立 Pipeline 定義。
func (s *PipelineService) CreatePipeline(ctx context.Context, req *CreatePipelineRequest, createdBy uint) (*models.Pipeline, error) {
	pipeline := &models.Pipeline{
		Name:              req.Name,
		Description:       req.Description,
		ClusterID:         req.ClusterID,
		Namespace:         req.Namespace,
		ConcurrencyGroup:  req.ConcurrencyGroup,
		ConcurrencyPolicy: req.ConcurrencyPolicy,
		MaxConcurrentRuns: req.MaxConcurrentRuns,
		CreatedBy:         createdBy,
	}

	if pipeline.ConcurrencyPolicy == "" {
		pipeline.ConcurrencyPolicy = models.ConcurrencyPolicyCancelPrevious
	}
	if pipeline.MaxConcurrentRuns <= 0 {
		pipeline.MaxConcurrentRuns = 1
	}

	if err := s.db.WithContext(ctx).Create(pipeline).Error; err != nil {
		if isDuplicateKeyError(err) {
			return nil, apierrors.ErrPipelineDuplicateName()
		}
		return nil, fmt.Errorf("create pipeline: %w", err)
	}

	logger.Info("pipeline created",
		"pipeline_id", pipeline.ID,
		"name", pipeline.Name,
		"cluster_id", pipeline.ClusterID,
		"namespace", pipeline.Namespace,
	)
	return pipeline, nil
}

// GetPipeline 取得單一 Pipeline。
func (s *PipelineService) GetPipeline(ctx context.Context, id uint) (*models.Pipeline, error) {
	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).First(&pipeline, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.ErrPipelineNotFound()
		}
		return nil, fmt.Errorf("get pipeline %d: %w", id, err)
	}
	return &pipeline, nil
}

// PipelineWithTriggers 包含 Pipeline 及其當前版本的觸發規則。
type PipelineWithTriggers struct {
	Pipeline     models.Pipeline
	TriggersJSON string
}

// ListPipelinesWithTriggers 列出所有有 webhook 觸發規則的 Pipeline。
// 透過 JOIN pipeline_versions 取得當前版本的 triggers_json。
func (s *PipelineService) ListPipelinesWithTriggers(ctx context.Context) ([]PipelineWithTriggers, error) {
	type row struct {
		models.Pipeline
		TriggersJSON string `gorm:"column:triggers_json"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("pipelines p").
		Select("p.*, pv.triggers_json").
		Joins("INNER JOIN pipeline_versions pv ON pv.id = p.current_version_id").
		Where("p.deleted_at IS NULL AND p.current_version_id IS NOT NULL").
		Where("pv.triggers_json IS NOT NULL AND pv.triggers_json != '' AND pv.triggers_json != '[]' AND pv.triggers_json != 'null'").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list pipelines with triggers: %w", err)
	}

	result := make([]PipelineWithTriggers, 0, len(rows))
	for _, r := range rows {
		result = append(result, PipelineWithTriggers{
			Pipeline:     r.Pipeline,
			TriggersJSON: r.TriggersJSON,
		})
	}
	return result, nil
}

// ListPipelines 列出 Pipeline（分頁、篩選、搜尋）。
func (s *PipelineService) ListPipelines(ctx context.Context, params *ListPipelinesParams) ([]models.Pipeline, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.Pipeline{})

	if params.ClusterID > 0 {
		query = query.Where("cluster_id = ?", params.ClusterID)
	}
	if params.Namespace != "" {
		query = query.Where("namespace = ?", params.Namespace)
	}
	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", search, search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count pipelines: %w", err)
	}

	var pipelines []models.Pipeline
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("id DESC").
		Offset(offset).Limit(params.PageSize).
		Find(&pipelines).Error; err != nil {
		return nil, 0, fmt.Errorf("list pipelines: %w", err)
	}
	return pipelines, total, nil
}

// UpdatePipeline 更新 Pipeline 定義（不影響已建立的版本快照）。
func (s *PipelineService) UpdatePipeline(ctx context.Context, id uint, req *UpdatePipelineRequest) (*models.Pipeline, error) {
	pipeline, err := s.GetPipeline(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Description != nil {
		pipeline.Description = *req.Description
	}
	if req.ConcurrencyGroup != nil {
		pipeline.ConcurrencyGroup = *req.ConcurrencyGroup
	}
	if req.ConcurrencyPolicy != nil {
		pipeline.ConcurrencyPolicy = *req.ConcurrencyPolicy
	}
	if req.MaxConcurrentRuns != nil {
		pipeline.MaxConcurrentRuns = *req.MaxConcurrentRuns
	}

	if err := s.db.WithContext(ctx).Save(pipeline).Error; err != nil {
		return nil, fmt.Errorf("update pipeline %d: %w", id, err)
	}

	logger.Info("pipeline updated", "pipeline_id", id)
	return pipeline, nil
}

// DeletePipeline 軟刪除 Pipeline（關聯的 Versions 和 Runs 保留供審計）。
func (s *PipelineService) DeletePipeline(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.Pipeline{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete pipeline %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return apierrors.ErrPipelineNotFound()
	}

	logger.Info("pipeline deleted", "pipeline_id", id)
	return nil
}

// ---------------------------------------------------------------------------
// Version 快照（不可變 + hash dedupe）
// ---------------------------------------------------------------------------

// CreateVersion 建立不可變版本快照。
// 若內容 hash 與既有版本相同，直接復用該版本而不建立新記錄。
func (s *PipelineService) CreateVersion(ctx context.Context, pipelineID uint, req *CreateVersionRequest, createdBy uint) (*models.PipelineVersion, error) {
	// 確認 Pipeline 存在
	if _, err := s.GetPipeline(ctx, pipelineID); err != nil {
		return nil, err
	}

	// 計算內容 hash
	hash := computeVersionHash(req)

	// Transaction 保證 hash dedupe + 版本建立 + current_version_id 更新原子性
	version := &models.PipelineVersion{}
	deduplicated := false

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Hash dedupe：若已存在相同內容版本，直接復用
		var existing models.PipelineVersion
		hashErr := tx.Where("pipeline_id = ? AND hash_sha256 = ?", pipelineID, hash).
			First(&existing).Error
		if hashErr == nil {
			// 確保 current_version_id 指向此版本
			if err := tx.Model(&models.Pipeline{}).
				Where("id = ?", pipelineID).
				Update("current_version_id", existing.ID).Error; err != nil {
				return fmt.Errorf("update current version pointer: %w", err)
			}
			*version = existing
			deduplicated = true
			return nil
		}
		if !errors.Is(hashErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check version hash: %w", hashErr)
		}

		// 取得下一個版本號
		var maxVersion int
		if err := tx.Model(&models.PipelineVersion{}).
			Where("pipeline_id = ?", pipelineID).
			Select("COALESCE(MAX(version), 0)").
			Scan(&maxVersion).Error; err != nil {
			return fmt.Errorf("query max version for pipeline %d: %w", pipelineID, err)
		}

		*version = models.PipelineVersion{
			PipelineID:    pipelineID,
			Version:       maxVersion + 1,
			StepsJSON:     req.StepsJSON,
			TriggersJSON:  req.TriggersJSON,
			EnvJSON:       req.EnvJSON,
			RuntimeJSON:   req.RuntimeJSON,
			WorkspaceJSON: req.WorkspaceJSON,
			HashSHA256:    hash,
			CreatedBy:     createdBy,
		}

		if err := tx.Create(version).Error; err != nil {
			return fmt.Errorf("create pipeline version: %w", err)
		}
		if err := tx.Model(&models.Pipeline{}).
			Where("id = ?", pipelineID).
			Update("current_version_id", version.ID).Error; err != nil {
			return fmt.Errorf("update current version pointer: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if deduplicated {
		logger.Info("pipeline version deduplicated",
			"pipeline_id", pipelineID,
			"version_id", version.ID,
			"hash", hash,
		)
	} else {
		logger.Info("pipeline version created",
			"pipeline_id", pipelineID,
			"version", version.Version,
			"hash", hash,
		)
	}
	return version, nil
}

// GetVersion 取得指定版本。
func (s *PipelineService) GetVersion(ctx context.Context, pipelineID uint, versionNum int) (*models.PipelineVersion, error) {
	var version models.PipelineVersion
	err := s.db.WithContext(ctx).
		Where("pipeline_id = ? AND version = ?", pipelineID, versionNum).
		First(&version).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.ErrPipelineVersionNotFound()
		}
		return nil, fmt.Errorf("get pipeline version: %w", err)
	}
	return &version, nil
}

// ListVersions 列出 Pipeline 的所有版本（按版本號降序）。
func (s *PipelineService) ListVersions(ctx context.Context, pipelineID uint) ([]models.PipelineVersion, error) {
	// 確認 Pipeline 存在
	if _, err := s.GetPipeline(ctx, pipelineID); err != nil {
		return nil, err
	}

	var versions []models.PipelineVersion
	if err := s.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("version DESC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list pipeline versions: %w", err)
	}
	return versions, nil
}

// GetStepRun 查詢單一 StepRun。
func (s *PipelineService) GetStepRun(ctx context.Context, stepRunID uint) (*models.StepRun, error) {
	var sr models.StepRun
	if err := s.db.WithContext(ctx).First(&sr, stepRunID).Error; err != nil {
		return nil, fmt.Errorf("get step run %d: %w", stepRunID, err)
	}
	return &sr, nil
}

// ---------------------------------------------------------------------------
// Pipeline Run 查詢
// ---------------------------------------------------------------------------

// ListPipelineRunsParams 列出 Run 的查詢參數。
type ListPipelineRunsParams struct {
	PipelineID uint
	Status     string // 篩選狀態（可選）
	Page       int
	PageSize   int
}

// ListPipelineRuns 列出 Pipeline 的執行記錄。
func (s *PipelineService) ListPipelineRuns(ctx context.Context, params *ListPipelineRunsParams) ([]models.PipelineRun, int64, error) {
	query := s.db.WithContext(ctx).Where("pipeline_id = ?", params.PipelineID)
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	var total int64
	if err := query.Model(&models.PipelineRun{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count pipeline runs: %w", err)
	}

	var runs []models.PipelineRun
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("created_at DESC").
		Offset(offset).Limit(params.PageSize).
		Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list pipeline runs: %w", err)
	}
	return runs, total, nil
}

// GetPipelineRun 取得單一 Run 詳情。
func (s *PipelineService) GetPipelineRun(ctx context.Context, runID uint) (*models.PipelineRun, error) {
	var run models.PipelineRun
	if err := s.db.WithContext(ctx).First(&run, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &apierrors.AppError{
				Code:       apierrors.CodePipelineNotFound,
				HTTPStatus: 404,
				Message:    fmt.Sprintf("pipeline run %d not found", runID),
			}
		}
		return nil, fmt.Errorf("get pipeline run %d: %w", runID, err)
	}
	return &run, nil
}

// ListStepRuns 列出 Run 的所有 Step 記錄（按 step_index 升序）。
func (s *PipelineService) ListStepRuns(ctx context.Context, runID uint) ([]models.StepRun, error) {
	var steps []models.StepRun
	if err := s.db.WithContext(ctx).
		Where("pipeline_run_id = ?", runID).
		Order("step_index ASC").
		Find(&steps).Error; err != nil {
		return nil, fmt.Errorf("list step runs for run %d: %w", runID, err)
	}
	return steps, nil
}

// ---------------------------------------------------------------------------
// Approval Step
// ---------------------------------------------------------------------------

// ApproveStepRun 批准 Approval Step。
func (s *PipelineService) ApproveStepRun(ctx context.Context, stepRunID uint, approvedBy string) error {
	return ApproveStep(ctx, s.db, stepRunID, approvedBy)
}

// RejectStepRun 拒絕 Approval Step。
func (s *PipelineService) RejectStepRun(ctx context.Context, stepRunID uint, rejectedBy string, reason string) error {
	return RejectStep(ctx, s.db, stepRunID, rejectedBy, reason)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// computeVersionHash 計算版本內容的 SHA-256 hash。
// 將所有內容欄位排序拼接後 hash，確保相同內容產生相同 hash。
func computeVersionHash(req *CreateVersionRequest) string {
	parts := []struct{ key, val string }{
		{"env", req.EnvJSON},
		{"runtime", req.RuntimeJSON},
		{"steps", req.StepsJSON},
		{"triggers", req.TriggersJSON},
		{"workspace", req.WorkspaceJSON},
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].key < parts[j].key })

	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.key)
		b.WriteByte(':')
		b.WriteString(p.val)
		b.WriteByte('\n')
	}

	sum := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("%x", sum)
}

// isDuplicateKeyError is defined in token_blacklist_service.go (shared within package).
