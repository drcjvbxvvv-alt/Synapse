package services

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shaia/Synapse/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTagRetentionService(t *testing.T) (*TagRetentionService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	svc := NewTagRetentionService(gormDB, nil) // registrySvc not needed for CRUD tests
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

var retentionCols = []string{
	"id", "registry_id", "repository_match", "tag_match",
	"retention_type", "keep_count", "keep_days", "keep_pattern",
	"enabled", "cron_expr", "last_run_at", "last_run_result",
	"created_by", "created_at", "updated_at", "deleted_at",
}

func retentionRow(id, registryID uint) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(retentionCols).AddRow(
		id, registryID, "myapp/*", "*",
		"keep_last_n", 10, 0, "",
		true, "", nil, "",
		1, now, now, nil,
	)
}

func makeTags(names []string, ages []int) []RegistryTag {
	tags := make([]RegistryTag, len(names))
	for i, name := range names {
		tags[i] = RegistryTag{
			Name:      name,
			CreatedAt: time.Now().AddDate(0, 0, -ages[i]),
		}
	}
	return tags
}

func TestEvalKeepLastN(t *testing.T) {
	tags := makeTags([]string{"v1", "v2", "v3", "v4", "v5"}, []int{5, 4, 3, 2, 1})

	t.Run("keep 3", func(t *testing.T) {
		keep, remove := evalKeepLastN(tags, 3)
		assert.Len(t, keep, 3)
		assert.Len(t, remove, 2)
		// Newest first
		assert.Equal(t, "v5", keep[0].Name)
		assert.Equal(t, "v4", keep[1].Name)
		assert.Equal(t, "v3", keep[2].Name)
	})

	t.Run("keep more than available", func(t *testing.T) {
		keep, remove := evalKeepLastN(tags, 10)
		assert.Len(t, keep, 5)
		assert.Empty(t, remove)
	})

	t.Run("keep 0 means keep all", func(t *testing.T) {
		keep, remove := evalKeepLastN(tags, 0)
		assert.Len(t, keep, 5)
		assert.Empty(t, remove)
	})

	t.Run("empty tags", func(t *testing.T) {
		keep, remove := evalKeepLastN(nil, 3)
		assert.Empty(t, keep)
		assert.Empty(t, remove)
	})
}

func TestEvalKeepByAge(t *testing.T) {
	tags := makeTags([]string{"old", "medium", "new"}, []int{30, 10, 1})

	t.Run("keep 15 days", func(t *testing.T) {
		keep, remove := evalKeepByAge(tags, 15)
		assert.Len(t, keep, 2)
		assert.Len(t, remove, 1)
		assert.Equal(t, "old", remove[0].Name)
	})

	t.Run("keep 5 days", func(t *testing.T) {
		keep, remove := evalKeepByAge(tags, 5)
		assert.Len(t, keep, 1)
		assert.Len(t, remove, 2)
		assert.Equal(t, "new", keep[0].Name)
	})

	t.Run("keep 0 means keep all", func(t *testing.T) {
		keep, remove := evalKeepByAge(tags, 0)
		assert.Len(t, keep, 3)
		assert.Empty(t, remove)
	})
}

func TestEvalKeepByRegex(t *testing.T) {
	tags := makeTags([]string{"v1.0.0", "v2.0.0", "latest", "dev-abc", "v3.0.0-rc1"}, []int{5, 4, 3, 2, 1})

	t.Run("keep semver pattern", func(t *testing.T) {
		keep, remove := evalKeepByRegex(tags, `^v\d+\.\d+\.\d+$`)
		assert.Len(t, keep, 2) // v1.0.0, v2.0.0
		assert.Len(t, remove, 3)
	})

	t.Run("keep all v* tags", func(t *testing.T) {
		keep, remove := evalKeepByRegex(tags, `^v`)
		assert.Len(t, keep, 3) // v1.0.0, v2.0.0, v3.0.0-rc1
		assert.Len(t, remove, 2)
	})

	t.Run("empty pattern keeps all", func(t *testing.T) {
		keep, remove := evalKeepByRegex(tags, "")
		assert.Len(t, keep, 5)
		assert.Empty(t, remove)
	})

	t.Run("invalid regex keeps all", func(t *testing.T) {
		keep, remove := evalKeepByRegex(tags, "[invalid")
		assert.Len(t, keep, 5)
		assert.Empty(t, remove)
	})
}

func TestEvaluateRetention_Dispatch(t *testing.T) {
	tags := makeTags([]string{"a", "b", "c"}, []int{3, 2, 1})

	t.Run("keep_last_n", func(t *testing.T) {
		policy := &models.TagRetentionPolicy{RetentionType: models.RetentionKeepLastN, KeepCount: 2}
		keep, remove := evaluateRetention(policy, tags)
		assert.Len(t, keep, 2)
		assert.Len(t, remove, 1)
	})

	t.Run("keep_by_age", func(t *testing.T) {
		policy := &models.TagRetentionPolicy{RetentionType: models.RetentionKeepByAge, KeepDays: 2}
		keep, remove := evaluateRetention(policy, tags)
		assert.Len(t, keep, 1)
		assert.Len(t, remove, 2)
	})

	t.Run("keep_by_regex", func(t *testing.T) {
		policy := &models.TagRetentionPolicy{RetentionType: models.RetentionKeepByRegex, KeepPattern: "^[ab]$"}
		keep, remove := evaluateRetention(policy, tags)
		assert.Len(t, keep, 2)
		assert.Len(t, remove, 1)
	})

	t.Run("unknown type keeps all", func(t *testing.T) {
		policy := &models.TagRetentionPolicy{RetentionType: "unknown"}
		keep, remove := evaluateRetention(policy, tags)
		assert.Len(t, keep, 3)
		assert.Empty(t, remove)
	})
}

func TestValidateRetentionPolicy(t *testing.T) {
	t.Run("valid keep_last_n", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepLastN,
			KeepCount:     5,
		})
		assert.NoError(t, err)
	})

	t.Run("invalid keep_last_n zero", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepLastN,
			KeepCount:     0,
		})
		assert.Error(t, err)
	})

	t.Run("valid keep_by_age", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepByAge,
			KeepDays:      30,
		})
		assert.NoError(t, err)
	})

	t.Run("valid keep_by_regex", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepByRegex,
			KeepPattern:   `^v\d+`,
		})
		assert.NoError(t, err)
	})

	t.Run("invalid regex", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepByRegex,
			KeepPattern:   "[invalid",
		})
		assert.Error(t, err)
	})

	t.Run("invalid type", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: "bad",
		})
		assert.Error(t, err)
	})

	t.Run("valid cron", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepLastN,
			KeepCount:     5,
			CronExpr:      "0 3 * * 0",
		})
		assert.NoError(t, err)
	})

	t.Run("invalid cron", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepLastN,
			KeepCount:     5,
			CronExpr:      "bad cron",
		})
		assert.Error(t, err)
	})
}

// ─── TagRetentionService DB CRUD ─────────────────────────────────────────────

func TestTagRetentionService_CreatePolicy_Success(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .tag_retention_policies.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	policy := &models.TagRetentionPolicy{
		RegistryID:      1,
		RepositoryMatch: "myapp/*",
		TagMatch:        "*",
		RetentionType:   "keep_last_n",
		KeepCount:       10,
		Enabled:         true,
		CreatedBy:       1,
	}
	err := svc.CreatePolicy(context.Background(), policy)
	require.NoError(t, err)
}

func TestTagRetentionService_CreatePolicy_InvalidType(t *testing.T) {
	svc, _, cleanup := newTagRetentionService(t)
	defer cleanup()

	policy := &models.TagRetentionPolicy{
		RegistryID:      1,
		RepositoryMatch: "*",
		RetentionType:   "invalid_type",
		CreatedBy:       1,
	}
	err := svc.CreatePolicy(context.Background(), policy)
	assert.Error(t, err)
}

func TestTagRetentionService_GetPolicy_Success(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(retentionRow(1, 5))

	policy, err := svc.GetPolicy(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, uint(5), policy.RegistryID)
	assert.Equal(t, "keep_last_n", policy.RetentionType)
}

func TestTagRetentionService_GetPolicy_NotFound(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetPolicy(context.Background(), 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get tag retention policy")
}

func TestTagRetentionService_ListPolicies_Empty(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(retentionCols))

	policies, err := svc.ListPolicies(context.Background(), 1)
	require.NoError(t, err)
	assert.Empty(t, policies)
}

func TestTagRetentionService_ListPolicies_Multiple(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(retentionCols).
		AddRow(1, 1, "myapp/*", "*", "keep_last_n", 10, 0, "", true, "", nil, "", 1, now, now, nil).
		AddRow(2, 1, "legacy/*", "*", "keep_by_age", 0, 30, "", true, "", nil, "", 1, now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	policies, err := svc.ListPolicies(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, policies, 2)
}

func TestTagRetentionService_UpdatePolicy_Success(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .tag_retention_policies.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.UpdatePolicy(context.Background(), 1, map[string]interface{}{"keep_count": 20})
	assert.NoError(t, err)
}

func TestTagRetentionService_UpdatePolicy_NotFound(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .tag_retention_policies.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.UpdatePolicy(context.Background(), 999, map[string]interface{}{"keep_count": 5})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTagRetentionService_DeletePolicy_Success(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .tag_retention_policies.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeletePolicy(context.Background(), 1)
	assert.NoError(t, err)
}

func TestTagRetentionService_DeletePolicy_NotFound(t *testing.T) {
	svc, mock, cleanup := newTagRetentionService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .tag_retention_policies.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.DeletePolicy(context.Background(), 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
