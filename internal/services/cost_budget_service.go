package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"gorm.io/gorm"
)

// BudgetAlert 預算告警
type BudgetAlert struct {
	Namespace      string  `json:"namespace"`
	Resource       string  `json:"resource"`        // "cpu", "memory", "cost"
	Limit          float64 `json:"limit"`
	Current        float64 `json:"current"`
	UsagePercent   float64 `json:"usage_percent"`   // 0-100
	AlertThreshold float64 `json:"alert_threshold"` // 0-1
	Exceeded       bool    `json:"exceeded"`         // usage > 100%
	Alert          bool    `json:"alert"`            // usage > threshold
}

// BudgetStatus 命名空間預算狀態
type BudgetStatus struct {
	Budget models.NamespaceBudget `json:"budget"`
	Alerts []BudgetAlert          `json:"alerts"`
	Status string                 `json:"status"` // ok, warning, exceeded
}

// CostBudgetService 預算管理服務
type CostBudgetService struct {
	db *gorm.DB
}

// NewCostBudgetService 建立預算服務
func NewCostBudgetService(db *gorm.DB) *CostBudgetService {
	return &CostBudgetService{db: db}
}

// ListBudgets 列出叢集所有預算
func (s *CostBudgetService) ListBudgets(ctx context.Context, clusterID uint) ([]models.NamespaceBudget, error) {
	var budgets []models.NamespaceBudget
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Order("namespace").
		Find(&budgets).Error; err != nil {
		return nil, fmt.Errorf("list budgets for cluster %d: %w", clusterID, err)
	}
	return budgets, nil
}

// GetBudget 取得單一預算
func (s *CostBudgetService) GetBudget(ctx context.Context, clusterID uint, namespace string) (*models.NamespaceBudget, error) {
	var budget models.NamespaceBudget
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ? AND namespace = ?", clusterID, namespace).
		First(&budget).Error; err != nil {
		return nil, fmt.Errorf("get budget for %s in cluster %d: %w", namespace, clusterID, err)
	}
	return &budget, nil
}

// UpsertBudget 新增或更新預算
func (s *CostBudgetService) UpsertBudget(ctx context.Context, budget *models.NamespaceBudget) error {
	if budget.AlertThreshold <= 0 || budget.AlertThreshold > 1 {
		budget.AlertThreshold = 0.8
	}
	return s.db.WithContext(ctx).
		Where("cluster_id = ? AND namespace = ?", budget.ClusterID, budget.Namespace).
		Assign(models.NamespaceBudget{
			CPUCoresLimit:    budget.CPUCoresLimit,
			MemoryGiBLimit:   budget.MemoryGiBLimit,
			MonthlyCostLimit: budget.MonthlyCostLimit,
			AlertThreshold:   budget.AlertThreshold,
			Enabled:          budget.Enabled,
		}).
		FirstOrCreate(budget).Error
}

// DeleteBudget 刪除預算
func (s *CostBudgetService) DeleteBudget(ctx context.Context, clusterID uint, namespace string) error {
	result := s.db.WithContext(ctx).
		Where("cluster_id = ? AND namespace = ?", clusterID, namespace).
		Delete(&models.NamespaceBudget{})
	if result.Error != nil {
		return fmt.Errorf("delete budget for %s in cluster %d: %w", namespace, clusterID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("budget not found for namespace %s", namespace)
	}
	return nil
}

// CheckBudgets 檢查預算狀態（需要傳入當前使用量）
func (s *CostBudgetService) CheckBudgets(ctx context.Context, clusterID uint, usageMap map[string]*NamespaceUsage) ([]BudgetStatus, error) {
	budgets, err := s.ListBudgets(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	var results []BudgetStatus
	for _, b := range budgets {
		if !b.Enabled {
			continue
		}
		usage := usageMap[b.Namespace]
		if usage == nil {
			usage = &NamespaceUsage{}
		}

		var alerts []BudgetAlert
		status := "ok"

		if b.CPUCoresLimit > 0 {
			cpuCores := usage.CPUMillicores / 1000.0
			pct := cpuCores / b.CPUCoresLimit * 100
			alert := BudgetAlert{
				Namespace:      b.Namespace,
				Resource:       "cpu",
				Limit:          b.CPUCoresLimit,
				Current:        cpuCores,
				UsagePercent:   pct,
				AlertThreshold: b.AlertThreshold,
				Exceeded:       pct > 100,
				Alert:          pct > b.AlertThreshold*100,
			}
			alerts = append(alerts, alert)
			if alert.Exceeded {
				status = "exceeded"
			} else if alert.Alert && status != "exceeded" {
				status = "warning"
			}
		}

		if b.MemoryGiBLimit > 0 {
			memGiB := usage.MemoryMiB / 1024.0
			pct := memGiB / b.MemoryGiBLimit * 100
			alert := BudgetAlert{
				Namespace:      b.Namespace,
				Resource:       "memory",
				Limit:          b.MemoryGiBLimit,
				Current:        memGiB,
				UsagePercent:   pct,
				AlertThreshold: b.AlertThreshold,
				Exceeded:       pct > 100,
				Alert:          pct > b.AlertThreshold*100,
			}
			alerts = append(alerts, alert)
			if alert.Exceeded {
				status = "exceeded"
			} else if alert.Alert && status != "exceeded" {
				status = "warning"
			}
		}

		if b.MonthlyCostLimit > 0 && usage.EstCost > 0 {
			pct := usage.EstCost / b.MonthlyCostLimit * 100
			alert := BudgetAlert{
				Namespace:      b.Namespace,
				Resource:       "cost",
				Limit:          b.MonthlyCostLimit,
				Current:        usage.EstCost,
				UsagePercent:   pct,
				AlertThreshold: b.AlertThreshold,
				Exceeded:       pct > 100,
				Alert:          pct > b.AlertThreshold*100,
			}
			alerts = append(alerts, alert)
			if alert.Exceeded {
				status = "exceeded"
			} else if alert.Alert && status != "exceeded" {
				status = "warning"
			}
		}

		results = append(results, BudgetStatus{
			Budget: b,
			Alerts: alerts,
			Status: status,
		})
	}

	return results, nil
}

// NamespaceUsage 命名空間當前使用量（由 handler 傳入）
type NamespaceUsage struct {
	CPUMillicores float64
	MemoryMiB     float64
	EstCost       float64
}
