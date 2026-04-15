package services

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/models"
)

func newCostBudgetService(t *testing.T) (*CostBudgetService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := NewCostBudgetService(gormDB)
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

var budgetCols = []string{
	"id", "cluster_id", "namespace",
	"cpu_cores_limit", "memory_gi_b_limit", "monthly_cost_limit",
	"alert_threshold", "enabled", "created_at", "updated_at",
}

func budgetRow(ns string, cpu, mem, cost, threshold float64, enabled bool) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(budgetCols).AddRow(
		1, 1, ns, cpu, mem, cost, threshold, enabled, now, now,
	)
}

// ─── ListBudgets ──────────────────────────────────────────────────────────────

func TestCostBudgetService_List_Empty(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(budgetCols))

	budgets, err := svc.ListBudgets(context.Background(), 1)
	require.NoError(t, err)
	assert.Empty(t, budgets)
}

func TestCostBudgetService_List_Multiple(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(budgetCols).
		AddRow(1, 1, "default", 4.0, 8.0, 100.0, 0.8, true, now, now).
		AddRow(2, 1, "production", 8.0, 16.0, 200.0, 0.9, true, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	budgets, err := svc.ListBudgets(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, budgets, 2)
	assert.Equal(t, "default", budgets[0].Namespace)
	assert.Equal(t, "production", budgets[1].Namespace)
}

func TestCostBudgetService_List_DBError(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidData)

	_, err := svc.ListBudgets(context.Background(), 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list budgets")
}

// ─── GetBudget ────────────────────────────────────────────────────────────────

func TestCostBudgetService_Get_Success(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(budgetRow("default", 4.0, 8.0, 0, 0.8, true))

	b, err := svc.GetBudget(context.Background(), 1, "default")
	require.NoError(t, err)
	assert.Equal(t, "default", b.Namespace)
	assert.Equal(t, 4.0, b.CPUCoresLimit)
}

func TestCostBudgetService_Get_NotFound(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetBudget(context.Background(), 1, "nosuchns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get budget")
}

// ─── UpsertBudget ─────────────────────────────────────────────────────────────

func TestCostBudgetService_Upsert_DefaultsThreshold(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	// FirstOrCreate: SELECT returns 0 rows, then INSERT
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(budgetCols))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .namespace_budgets.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	b := &models.NamespaceBudget{
		ClusterID:      1,
		Namespace:      "default",
		CPUCoresLimit:  2.0,
		AlertThreshold: 0, // invalid — should be defaulted to 0.8
	}
	err := svc.UpsertBudget(context.Background(), b)
	assert.NoError(t, err)
	assert.Equal(t, 0.8, b.AlertThreshold)
}

func TestCostBudgetService_Upsert_ValidThreshold(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	// FirstOrCreate: SELECT finds existing
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows(budgetCols).AddRow(1, 1, "default", 4.0, 8.0, 0, 0.9, true, now, now),
	)
	// No INSERT — just UPDATE via save
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .namespace_budgets.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	b := &models.NamespaceBudget{
		ClusterID:      1,
		Namespace:      "default",
		CPUCoresLimit:  4.0,
		AlertThreshold: 0.9,
	}
	err := svc.UpsertBudget(context.Background(), b)
	assert.NoError(t, err)
	assert.Equal(t, 0.9, b.AlertThreshold)
}

// ─── DeleteBudget ─────────────────────────────────────────────────────────────

func TestCostBudgetService_Delete_Success(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM .namespace_budgets.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteBudget(context.Background(), 1, "default")
	assert.NoError(t, err)
}

func TestCostBudgetService_Delete_NotFound(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM .namespace_budgets.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.DeleteBudget(context.Background(), 1, "nosuchns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "budget not found")
}

func TestCostBudgetService_Delete_DBError(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM .namespace_budgets.`).WillReturnError(gorm.ErrInvalidData)
	mock.ExpectRollback()

	err := svc.DeleteBudget(context.Background(), 1, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete budget")
}

// ─── CheckBudgets (pure logic) ────────────────────────────────────────────────

func TestCostBudgetService_CheckBudgets_AllOK(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(budgetCols).
		AddRow(1, 1, "default", 4.0, 8.0, 100.0, 0.8, true, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	usageMap := map[string]*NamespaceUsage{
		"default": {CPUMillicores: 1000, MemoryMiB: 512, EstCost: 10.0},
	}
	results, err := svc.CheckBudgets(context.Background(), 1, usageMap)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "ok", results[0].Status)
}

func TestCostBudgetService_CheckBudgets_CPUWarning(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(budgetCols).
		AddRow(1, 1, "default", 2.0, 0, 0, 0.8, true, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	// 1.8 cores / 2.0 limit = 90% → > 80% threshold → warning
	usageMap := map[string]*NamespaceUsage{
		"default": {CPUMillicores: 1800},
	}
	results, err := svc.CheckBudgets(context.Background(), 1, usageMap)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "warning", results[0].Status)
}

func TestCostBudgetService_CheckBudgets_CPUExceeded(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(budgetCols).
		AddRow(1, 1, "default", 2.0, 0, 0, 0.8, true, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	// 2.5 cores / 2.0 limit = 125% → exceeded
	usageMap := map[string]*NamespaceUsage{
		"default": {CPUMillicores: 2500},
	}
	results, err := svc.CheckBudgets(context.Background(), 1, usageMap)
	require.NoError(t, err)
	assert.Equal(t, "exceeded", results[0].Status)
}

func TestCostBudgetService_CheckBudgets_MemoryExceeded(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(budgetCols).
		AddRow(1, 1, "default", 0, 4.0, 0, 0.8, true, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	// 5 GiB used / 4 GiB limit → exceeded
	usageMap := map[string]*NamespaceUsage{
		"default": {MemoryMiB: 5120}, // 5 GiB
	}
	results, err := svc.CheckBudgets(context.Background(), 1, usageMap)
	require.NoError(t, err)
	assert.Equal(t, "exceeded", results[0].Status)
}

func TestCostBudgetService_CheckBudgets_CostWarning(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(budgetCols).
		AddRow(1, 1, "default", 0, 0, 100.0, 0.8, true, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	// $85 / $100 limit = 85% → > 80% threshold → warning
	usageMap := map[string]*NamespaceUsage{
		"default": {EstCost: 85.0},
	}
	results, err := svc.CheckBudgets(context.Background(), 1, usageMap)
	require.NoError(t, err)
	assert.Equal(t, "warning", results[0].Status)
}

func TestCostBudgetService_CheckBudgets_DisabledSkipped(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(budgetCols).
		AddRow(1, 1, "default", 2.0, 0, 0, 0.8, false, now, now) // enabled=false
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	usageMap := map[string]*NamespaceUsage{
		"default": {CPUMillicores: 9999},
	}
	results, err := svc.CheckBudgets(context.Background(), 1, usageMap)
	require.NoError(t, err)
	assert.Empty(t, results) // disabled namespace is skipped
}

func TestCostBudgetService_CheckBudgets_NoUsageData(t *testing.T) {
	svc, mock, cleanup := newCostBudgetService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(budgetCols).
		AddRow(1, 1, "default", 4.0, 8.0, 100.0, 0.8, true, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	// No usage data → all zeros → "ok"
	results, err := svc.CheckBudgets(context.Background(), 1, map[string]*NamespaceUsage{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "ok", results[0].Status)
}
