package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ── DTOs ────────────────────────────────────────────────────────────────────

// SLOStatus is the live SLI / error-budget snapshot for one SLO.
type SLOStatus struct {
	SLOID               uint    `json:"slo_id"`
	SLIValue            float64 `json:"sli_value"`             // 0-1; NaN serialised as null via custom marshal
	SLIPercent          float64 `json:"sli_percent"`           // SLIValue * 100
	ErrorBudgetTotal    float64 `json:"error_budget_total"`    // 1 - target
	ErrorBudgetUsed     float64 `json:"error_budget_used"`     // 1 - SLI
	ErrorBudgetRemaining float64 `json:"error_budget_remaining"` // clipped to [0,1]
	BurnRate1h          float64 `json:"burn_rate_1h"`
	BurnRate6h          float64 `json:"burn_rate_6h"`
	BurnRate24h         float64 `json:"burn_rate_24h"`
	BurnRateWindow      float64 `json:"burn_rate_window"` // over the full SLO window
	Status              string  `json:"status"`           // "ok" | "warning" | "critical" | "unknown"
	HasData             bool    `json:"has_data"`
	// ChaosActive is true when a Chaos Mesh experiment is actively injecting in
	// the same namespace as this SLO. When true, alerts should be suppressed.
	ChaosActive bool `json:"chaos_active"`
}

// ── Service ──────────────────────────────────────────────────────────────────

// SLOService manages SLO CRUD and live status calculation.
type SLOService struct {
	db               *gorm.DB
	prometheusSvc    *PrometheusService
	monitoringCfgSvc *MonitoringConfigService
}

// NewSLOService wires dependencies.
func NewSLOService(db *gorm.DB, promSvc *PrometheusService, monCfgSvc *MonitoringConfigService) *SLOService {
	return &SLOService{
		db:               db,
		prometheusSvc:    promSvc,
		monitoringCfgSvc: monCfgSvc,
	}
}

// ── CRUD ─────────────────────────────────────────────────────────────────────

// ListSLOs returns all non-deleted SLOs for a cluster, optionally filtered by namespace.
func (s *SLOService) ListSLOs(ctx context.Context, clusterID uint, namespace string) ([]models.SLO, error) {
	q := s.db.WithContext(ctx).
		Select("id, cluster_id, name, description, namespace, sli_type, prom_query, total_query, target, window, burn_rate_warning, burn_rate_critical, enabled, created_at, updated_at").
		Where("cluster_id = ?", clusterID).
		Order("name")

	if namespace != "" {
		q = q.Where("namespace = ?", namespace)
	}

	var items []models.SLO
	if err := q.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list SLOs for cluster %d: %w", clusterID, err)
	}
	return items, nil
}

// GetSLO returns a single SLO by ID, verifying it belongs to clusterID.
func (s *SLOService) GetSLO(ctx context.Context, clusterID, id uint) (*models.SLO, error) {
	var slo models.SLO
	if err := s.db.WithContext(ctx).
		Where("id = ? AND cluster_id = ?", id, clusterID).
		First(&slo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("SLO %d not found: %w", id, err)
		}
		return nil, fmt.Errorf("get SLO %d: %w", id, err)
	}
	return &slo, nil
}

// CreateSLO inserts a new SLO record.
func (s *SLOService) CreateSLO(ctx context.Context, slo *models.SLO) error {
	if err := s.db.WithContext(ctx).Create(slo).Error; err != nil {
		return fmt.Errorf("create SLO: %w", err)
	}
	logger.Info("SLO created", "cluster_id", slo.ClusterID, "name", slo.Name, "id", slo.ID)
	return nil
}

// UpdateSLO applies partial updates to an existing SLO.
func (s *SLOService) UpdateSLO(ctx context.Context, clusterID, id uint, updates *models.SLO) (*models.SLO, error) {
	slo, err := s.GetSLO(ctx, clusterID, id)
	if err != nil {
		return nil, err
	}

	// Apply updateable fields
	slo.Name = updates.Name
	slo.Description = updates.Description
	slo.Namespace = updates.Namespace
	slo.SLIType = updates.SLIType
	slo.PromQuery = updates.PromQuery
	slo.TotalQuery = updates.TotalQuery
	slo.Target = updates.Target
	slo.Window = updates.Window
	slo.BurnRateWarning = updates.BurnRateWarning
	slo.BurnRateCritical = updates.BurnRateCritical
	slo.Enabled = updates.Enabled

	if err := s.db.WithContext(ctx).Save(slo).Error; err != nil {
		return nil, fmt.Errorf("update SLO %d: %w", id, err)
	}
	logger.Info("SLO updated", "cluster_id", clusterID, "id", id, "name", slo.Name)
	return slo, nil
}

// DeleteSLO soft-deletes an SLO.
func (s *SLOService) DeleteSLO(ctx context.Context, clusterID, id uint) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND cluster_id = ?", id, clusterID).
		Delete(&models.SLO{})
	if result.Error != nil {
		return fmt.Errorf("delete SLO %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("SLO %d not found: %w", id, gorm.ErrRecordNotFound)
	}
	logger.Info("SLO deleted", "cluster_id", clusterID, "id", id)
	return nil
}

// ── Status calculation ────────────────────────────────────────────────────────

// GetSLOStatus queries Prometheus and computes SLI, error budget, and burn rates.
func (s *SLOService) GetSLOStatus(ctx context.Context, clusterID, sloID uint) (*SLOStatus, error) {
	slo, err := s.GetSLO(ctx, clusterID, sloID)
	if err != nil {
		return nil, err
	}

	cfg, err := s.monitoringCfgSvc.GetMonitoringConfig(clusterID)
	if err != nil || cfg == nil || cfg.Type == "disabled" {
		return &SLOStatus{SLOID: sloID, Status: "unknown", HasData: false}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Evaluate SLI at short windows for burn rates and over the full SLO window.
	windows := map[string]string{
		"1h":          "1h",
		"6h":          "6h",
		"24h":         "24h",
		slo.Window:    slo.Window,
	}

	sliByWindow := make(map[string]float64)
	for key, w := range windows {
		sli, calcErr := s.evalSLI(queryCtx, cfg, slo, w)
		if calcErr != nil {
			logger.Warn("SLI eval failed", "slo_id", sloID, "window", w, "error", calcErr)
			sliByWindow[key] = math.NaN()
		} else {
			sliByWindow[key] = sli
		}
	}

	windowSLI := sliByWindow[slo.Window]
	hasData := !math.IsNaN(windowSLI)

	st := &SLOStatus{
		SLOID:   sloID,
		HasData: hasData,
		Status:  "unknown",
	}

	if hasData {
		ebTotal := 1.0 - slo.Target
		ebUsed := 1.0 - windowSLI
		ebRemaining := 1.0 - (ebUsed / ebTotal) // fraction of budget left
		if ebRemaining < 0 {
			ebRemaining = 0
		}

		st.SLIValue = windowSLI
		st.SLIPercent = windowSLI * 100
		st.ErrorBudgetTotal = ebTotal
		st.ErrorBudgetUsed = ebUsed
		st.ErrorBudgetRemaining = ebRemaining
		st.BurnRateWindow = burnRate(windowSLI, slo.Target)
		st.BurnRate1h = burnRate(sliByWindow["1h"], slo.Target)
		st.BurnRate6h = burnRate(sliByWindow["6h"], slo.Target)
		st.BurnRate24h = burnRate(sliByWindow["24h"], slo.Target)

		// Status: critical → warning → ok
		switch {
		case st.BurnRate1h >= slo.BurnRateCritical || st.BurnRate6h >= slo.BurnRateCritical:
			st.Status = "critical"
		case st.BurnRate1h >= slo.BurnRateWarning || st.BurnRate6h >= slo.BurnRateWarning:
			st.Status = "warning"
		default:
			st.Status = "ok"
		}
	}

	return st, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// evalSLI evaluates the SLI ratio for a given window.
// If TotalQuery is set: SLI = sum(rate(PromQuery[w])) / sum(rate(TotalQuery[w]))
// Otherwise: PromQuery must return a ratio (0-1) directly.
func (s *SLOService) evalSLI(ctx context.Context, cfg *models.MonitoringConfig, slo *models.SLO, window string) (float64, error) {
	goodExpr := substituteWindow(slo.PromQuery, window)

	if slo.TotalQuery == "" {
		// Direct ratio query
		v, err := s.prometheusSvc.QueryInstantScalar(ctx, cfg, goodExpr)
		if err != nil {
			return math.NaN(), fmt.Errorf("query SLI ratio: %w", err)
		}
		return v, nil
	}

	// Ratio query: good / total
	totalExpr := substituteWindow(slo.TotalQuery, window)

	good, err := s.prometheusSvc.QueryInstantScalar(ctx, cfg, goodExpr)
	if err != nil {
		return math.NaN(), fmt.Errorf("query good events: %w", err)
	}
	total, err := s.prometheusSvc.QueryInstantScalar(ctx, cfg, totalExpr)
	if err != nil {
		return math.NaN(), fmt.Errorf("query total events: %w", err)
	}
	if math.IsNaN(good) || math.IsNaN(total) || total == 0 {
		return math.NaN(), nil
	}
	sli := good / total
	if sli > 1.0 {
		sli = 1.0 // clamp rounding errors
	}
	return sli, nil
}

// substituteWindow replaces the $window placeholder in a PromQL expression.
func substituteWindow(expr, window string) string {
	return strings.ReplaceAll(expr, "$window", window)
}

// burnRate computes (1 - sli) / (1 - target).
// Returns NaN when sli is NaN or target == 1.
func burnRate(sli, target float64) float64 {
	if math.IsNaN(sli) || target >= 1.0 {
		return math.NaN()
	}
	return (1.0 - sli) / (1.0 - target)
}
