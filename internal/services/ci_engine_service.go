// Package services – CIEngineService manages external CI engine connection
// profiles (GitLab, Jenkins, Tekton, …) and probes their availability through
// the engine adapter factory.
//
// The built-in Native engine does not need a row in ci_engine_configs; it is
// always registered with the factory at startup.
package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
	"github.com/shaia/Synapse/pkg/logger"
)

// CIEngineService is the service-layer facade for CRUD + health probing of
// external CI engine configurations.
type CIEngineService struct {
	db      *gorm.DB
	factory *engine.Factory
}

// NewCIEngineService constructs the service. A nil factory falls back to the
// package-level engine.Default() singleton; tests typically pass an isolated
// factory to avoid shared mutable state.
func NewCIEngineService(db *gorm.DB, f *engine.Factory) *CIEngineService {
	if f == nil {
		f = engine.Default()
	}
	return &CIEngineService{db: db, factory: f}
}

// Factory exposes the underlying engine factory, primarily for handler-level
// capability introspection.
func (s *CIEngineService) Factory() *engine.Factory { return s.factory }

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

// List returns all registered external CI engine configurations. Credentials
// are already decrypted by the model's AfterFind hook; callers must NOT log
// the returned objects' sensitive fields.
func (s *CIEngineService) List(ctx context.Context) ([]*models.CIEngineConfig, error) {
	var rows []*models.CIEngineConfig
	if err := s.db.WithContext(ctx).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list ci engine configs: %w", err)
	}
	return rows, nil
}

// Get fetches a single configuration by id.
func (s *CIEngineService) Get(ctx context.Context, id uint) (*models.CIEngineConfig, error) {
	var cfg models.CIEngineConfig
	err := s.db.WithContext(ctx).First(&cfg, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &apierrors.AppError{
				Code:       "CI_ENGINE_NOT_FOUND",
				HTTPStatus: http.StatusNotFound,
				Message:    fmt.Sprintf("ci engine config %d not found", id),
			}
		}
		return nil, fmt.Errorf("get ci engine config %d: %w", id, err)
	}
	return &cfg, nil
}

// Create validates the request and persists a new configuration.
func (s *CIEngineService) Create(ctx context.Context, req *models.CIEngineConfigRequest, createdBy uint) (*models.CIEngineConfig, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	cfg := &models.CIEngineConfig{CreatedBy: createdBy, Enabled: true}
	req.ApplyTo(cfg)

	// Prevent creating rows for the native engine — it's in-process and
	// needs no connection profile.
	if engine.EngineType(cfg.EngineType) == engine.EngineNative {
		return nil, &apierrors.AppError{
			Code:       "CI_ENGINE_NATIVE_NOT_STORED",
			HTTPStatus: http.StatusBadRequest,
			Message:    "native engine does not require a stored configuration",
		}
	}

	if err := s.db.WithContext(ctx).Create(cfg).Error; err != nil {
		return nil, fmt.Errorf("create ci engine config: %w", err)
	}
	logger.Info("ci engine config created",
		"id", cfg.ID,
		"engine_type", cfg.EngineType,
		"name", cfg.Name,
		"created_by", createdBy,
	)
	return cfg, nil
}

// Update mutates an existing configuration. Empty credential fields preserve
// the stored value (see CIEngineConfigRequest.ApplyTo).
func (s *CIEngineService) Update(ctx context.Context, id uint, req *models.CIEngineConfigRequest) (*models.CIEngineConfig, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	cfg, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	// Engine type immutability: switching an existing config from GitLab to
	// Jenkins would silently break every Pipeline referencing it. Force
	// callers to create a new entry instead.
	if req.EngineType != "" && req.EngineType != cfg.EngineType {
		return nil, &apierrors.AppError{
			Code:       "CI_ENGINE_TYPE_IMMUTABLE",
			HTTPStatus: http.StatusBadRequest,
			Message:    "engine_type cannot be changed; create a new config instead",
		}
	}
	req.ApplyTo(cfg)

	if err := s.db.WithContext(ctx).Save(cfg).Error; err != nil {
		return nil, fmt.Errorf("update ci engine config %d: %w", id, err)
	}
	logger.Info("ci engine config updated",
		"id", cfg.ID,
		"engine_type", cfg.EngineType,
		"name", cfg.Name,
	)
	return cfg, nil
}

// Delete removes a configuration. The caller is responsible for ensuring no
// Pipeline references the config (FK guard handled by DB or a higher layer).
func (s *CIEngineService) Delete(ctx context.Context, id uint) error {
	cfg, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(cfg).Error; err != nil {
		return fmt.Errorf("delete ci engine config %d: %w", id, err)
	}
	logger.Info("ci engine config deleted", "id", id, "engine_type", cfg.EngineType)
	return nil
}

// ---------------------------------------------------------------------------
// Probing
// ---------------------------------------------------------------------------

// EngineStatus is the probe result returned by ListAvailableEngines. It mixes
// both built-in (Native) and externally configured engines.
type EngineStatus struct {
	Type         string                    `json:"type"`
	ConfigID     *uint                     `json:"config_id,omitempty"`
	Name         string                    `json:"name"`
	Available    bool                      `json:"available"`
	Default      bool                      `json:"default,omitempty"`
	Version      string                    `json:"version,omitempty"`
	Capabilities *engine.EngineCapabilities `json:"capabilities,omitempty"`
	Error        string                    `json:"error,omitempty"`
}

// ListAvailableEngines returns the status of every engine known to Synapse:
// the Native engine (always present) plus every configured external engine.
//
// Failures in probing a single engine are NEVER surfaced as errors to the
// caller; they appear inside EngineStatus.Error (Observer Pattern, CLAUDE §8).
func (s *CIEngineService) ListAvailableEngines(ctx context.Context) ([]*EngineStatus, error) {
	out := make([]*EngineStatus, 0, 1)

	// Native engine — always reported, even if the adapter is not yet wired.
	native := &EngineStatus{
		Type:      string(engine.EngineNative),
		Name:      "Native (K8s Job)",
		Available: false,
		Default:   true,
	}
	if s.factory.IsRegistered(engine.EngineNative) {
		if a, err := s.factory.BuildNative(); err == nil {
			native.Available = a.IsAvailable(ctx)
			if v, verr := a.Version(ctx); verr == nil {
				native.Version = v
			}
			caps := a.Capabilities()
			native.Capabilities = &caps
		} else {
			native.Error = err.Error()
		}
	} else {
		native.Error = "native adapter not registered"
	}
	out = append(out, native)

	// External engines — one row per CIEngineConfig.
	var cfgs []*models.CIEngineConfig
	if err := s.db.WithContext(ctx).Find(&cfgs).Error; err != nil {
		return nil, fmt.Errorf("load ci engine configs: %w", err)
	}
	for _, cfg := range cfgs {
		item := &EngineStatus{
			Type:      cfg.EngineType,
			ConfigID:  &cfg.ID,
			Name:      cfg.Name,
			Available: false,
		}
		adapter, err := s.factory.Build(cfg)
		if err != nil {
			item.Error = err.Error()
			out = append(out, item)
			continue
		}
		// Short per-engine timeout so a slow / hung engine can't delay the
		// whole page load.
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		item.Available = adapter.IsAvailable(probeCtx)
		if v, verr := adapter.Version(probeCtx); verr == nil {
			item.Version = v
		}
		caps := adapter.Capabilities()
		item.Capabilities = &caps
		cancel()
		out = append(out, item)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Run operations — thin facade over CIEngineAdapter
// ---------------------------------------------------------------------------

// TriggerRun builds the adapter for the engine config identified by id, then
// calls Trigger. Returns the engine's TriggerResult on success.
func (s *CIEngineService) TriggerRun(ctx context.Context, id uint, req *engine.TriggerRequest) (*engine.TriggerResult, error) {
	if req == nil {
		return nil, &apierrors.AppError{
			Code: "CI_ENGINE_RUN_REQUEST_NIL", HTTPStatus: http.StatusBadRequest,
			Message: "trigger request body is required",
		}
	}
	a, err := s.buildAdapter(ctx, id)
	if err != nil {
		return nil, err
	}
	res, err := a.Trigger(ctx, req)
	if err != nil {
		return nil, mapEngineError(err, "trigger run")
	}
	logger.Info("ci engine run triggered",
		"config_id", id,
		"run_id", res.RunID,
		"external_id", res.ExternalID,
	)
	return res, nil
}

// GetRun fetches the current status of an active run from the engine.
func (s *CIEngineService) GetRun(ctx context.Context, id uint, runID string) (*engine.RunStatus, error) {
	if runID == "" {
		return nil, &apierrors.AppError{
			Code: "CI_ENGINE_RUN_ID_REQUIRED", HTTPStatus: http.StatusBadRequest,
			Message: "run_id is required",
		}
	}
	a, err := s.buildAdapter(ctx, id)
	if err != nil {
		return nil, err
	}
	status, err := a.GetRun(ctx, runID)
	if err != nil {
		return nil, mapEngineError(err, "get run")
	}
	return status, nil
}

// CancelRun requests cancellation of a run via the engine adapter.
func (s *CIEngineService) CancelRun(ctx context.Context, id uint, runID string) error {
	if runID == "" {
		return &apierrors.AppError{
			Code: "CI_ENGINE_RUN_ID_REQUIRED", HTTPStatus: http.StatusBadRequest,
			Message: "run_id is required",
		}
	}
	a, err := s.buildAdapter(ctx, id)
	if err != nil {
		return err
	}
	if err := a.Cancel(ctx, runID); err != nil {
		return mapEngineError(err, "cancel run")
	}
	logger.Info("ci engine run cancel requested", "config_id", id, "run_id", runID)
	return nil
}

// StreamLogs opens a log stream for the given run and optional step. The
// returned ReadCloser MUST be closed by the caller.
func (s *CIEngineService) StreamLogs(ctx context.Context, id uint, runID, stepID string) (io.ReadCloser, error) {
	if runID == "" {
		return nil, &apierrors.AppError{
			Code: "CI_ENGINE_RUN_ID_REQUIRED", HTTPStatus: http.StatusBadRequest,
			Message: "run_id is required",
		}
	}
	a, err := s.buildAdapter(ctx, id)
	if err != nil {
		return nil, err
	}
	rc, err := a.StreamLogs(ctx, runID, stepID)
	if err != nil {
		return nil, mapEngineError(err, "stream logs")
	}
	return rc, nil
}

// GetArtifacts returns the artifact list for a completed run.
func (s *CIEngineService) GetArtifacts(ctx context.Context, id uint, runID string) ([]*engine.Artifact, error) {
	if runID == "" {
		return nil, &apierrors.AppError{
			Code: "CI_ENGINE_RUN_ID_REQUIRED", HTTPStatus: http.StatusBadRequest,
			Message: "run_id is required",
		}
	}
	a, err := s.buildAdapter(ctx, id)
	if err != nil {
		return nil, err
	}
	arts, err := a.GetArtifacts(ctx, runID)
	if err != nil {
		return nil, mapEngineError(err, "get artifacts")
	}
	return arts, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildAdapter is the shared path: load config from DB → build adapter.
func (s *CIEngineService) buildAdapter(ctx context.Context, id uint) (engine.CIEngineAdapter, error) {
	cfg, err := s.Get(ctx, id)
	if err != nil {
		return nil, err // already an AppError
	}
	a, err := s.factory.Build(cfg)
	if err != nil {
		return nil, mapEngineError(err, "build adapter")
	}
	return a, nil
}

// mapEngineError translates engine sentinel errors into structured AppErrors so
// the HTTP layer can map them to the right status codes.
func mapEngineError(err error, op string) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, engine.ErrNotFound):
		return &apierrors.AppError{
			Code: "CI_ENGINE_RUN_NOT_FOUND", HTTPStatus: http.StatusNotFound,
			Message: fmt.Sprintf("%s: %s", op, err.Error()),
		}
	case errors.Is(err, engine.ErrInvalidInput):
		return &apierrors.AppError{
			Code: "CI_ENGINE_RUN_INVALID_INPUT", HTTPStatus: http.StatusBadRequest,
			Message: fmt.Sprintf("%s: %s", op, err.Error()),
		}
	case errors.Is(err, engine.ErrUnavailable):
		return &apierrors.AppError{
			Code: "CI_ENGINE_UNAVAILABLE", HTTPStatus: http.StatusServiceUnavailable,
			Message: fmt.Sprintf("%s: engine unavailable: %s", op, err.Error()),
		}
	case errors.Is(err, engine.ErrUnsupported):
		return &apierrors.AppError{
			Code: "CI_ENGINE_NOT_SUPPORTED", HTTPStatus: http.StatusNotImplemented,
			Message: fmt.Sprintf("%s: not supported by this engine", op),
		}
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// validateRequest performs input validation that does not depend on DB state.
// DB-level uniqueness violations are surfaced as AppErrors by the caller.
func validateRequest(req *models.CIEngineConfigRequest) error {
	if req == nil {
		return &apierrors.AppError{
			Code: "CI_ENGINE_REQUEST_NIL", HTTPStatus: http.StatusBadRequest,
			Message: "request body is required",
		}
	}
	if req.Name == "" {
		return &apierrors.AppError{
			Code: "CI_ENGINE_NAME_REQUIRED", HTTPStatus: http.StatusBadRequest,
			Message: "name is required",
		}
	}
	if !engine.EngineType(req.EngineType).IsValid() {
		return &apierrors.AppError{
			Code: "CI_ENGINE_TYPE_INVALID", HTTPStatus: http.StatusBadRequest,
			Message: fmt.Sprintf("unknown engine_type %q", req.EngineType),
		}
	}
	return nil
}
