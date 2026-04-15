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
)

func newPipelineSecretService(t *testing.T) (*PipelineSecretService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := NewPipelineSecretService(gormDB)
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

var secretCols = []string{
	"id", "scope", "scope_ref", "name", "value_enc",
	"description", "created_by", "created_at", "updated_at", "deleted_at",
}

func secretRow() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(secretCols).AddRow(
		1, "global", nil, "MY_SECRET", "enc-value", "a secret", 1, now, now, nil,
	)
}

// ─── CreateSecret ─────────────────────────────────────────────────────────────

func TestPipelineSecretService_Create_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	// COUNT check
	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// INSERT
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .pipeline_secrets.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	ref := uint(0)
	secret, err := svc.CreateSecret(context.Background(), &CreateSecretRequest{
		Scope:    "global",
		ScopeRef: &ref,
		Name:     "MY_SECRET",
		Value:    "supersecret",
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, "MY_SECRET", secret.Name)
	assert.Equal(t, "global", secret.Scope)
}

func TestPipelineSecretService_Create_DuplicateName(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	_, err := svc.CreateSecret(context.Background(), &CreateSecretRequest{
		Scope: "global",
		Name:  "MY_SECRET",
		Value: "val",
	}, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名稱已存在")
}

func TestPipelineSecretService_Create_DBError(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .pipeline_secrets.`).
		WillReturnError(gorm.ErrInvalidData)
	mock.ExpectRollback()

	_, err := svc.CreateSecret(context.Background(), &CreateSecretRequest{
		Scope: "global",
		Name:  "BAD",
		Value: "val",
	}, 1)
	assert.Error(t, err)
}

// ─── GetSecret ────────────────────────────────────────────────────────────────

func TestPipelineSecretService_Get_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(secretRow())

	got, err := svc.GetSecret(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "MY_SECRET", got.Name)
}

func TestPipelineSecretService_Get_NotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetSecret(context.Background(), 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// ─── ListSecrets ──────────────────────────────────────────────────────────────

func TestPipelineSecretService_List_NoFilter(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	listCols := []string{"id", "scope", "scope_ref", "name", "description", "created_by", "created_at", "updated_at"}
	now := time.Now()
	rows := sqlmock.NewRows(listCols).
		AddRow(1, "global", nil, "SECRET_A", "desc a", 1, now, now).
		AddRow(2, "pipeline", uint(5), "SECRET_B", "desc b", 1, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	secrets, err := svc.ListSecrets(context.Background(), "", nil)
	require.NoError(t, err)
	assert.Len(t, secrets, 2)
}

func TestPipelineSecretService_List_ByScopeAndRef(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	listCols := []string{"id", "scope", "scope_ref", "name", "description", "created_by", "created_at", "updated_at"}
	now := time.Now()
	rows := sqlmock.NewRows(listCols).
		AddRow(2, "pipeline", uint(5), "SECRET_B", "desc", 1, now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	ref := uint(5)
	secrets, err := svc.ListSecrets(context.Background(), "pipeline", &ref)
	require.NoError(t, err)
	assert.Len(t, secrets, 1)
	assert.Equal(t, "SECRET_B", secrets[0].Name)
}

func TestPipelineSecretService_List_Empty(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	listCols := []string{"id", "scope", "scope_ref", "name", "description", "created_by", "created_at", "updated_at"}
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(listCols))

	secrets, err := svc.ListSecrets(context.Background(), "global", nil)
	require.NoError(t, err)
	assert.Empty(t, secrets)
}

// ─── UpdateSecret ─────────────────────────────────────────────────────────────

func TestPipelineSecretService_Update_Value(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	// GetSecret
	mock.ExpectQuery(`SELECT`).WillReturnRows(secretRow())
	// Save
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipeline_secrets.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	newVal := "newvalue"
	got, err := svc.UpdateSecret(context.Background(), 1, &UpdateSecretRequest{Value: &newVal})
	require.NoError(t, err)
	assert.Equal(t, "newvalue", got.ValueEnc)
}

func TestPipelineSecretService_Update_Description(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(secretRow())
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipeline_secrets.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	newDesc := "updated description"
	got, err := svc.UpdateSecret(context.Background(), 1, &UpdateSecretRequest{Description: &newDesc})
	require.NoError(t, err)
	assert.Equal(t, "updated description", got.Description)
}

func TestPipelineSecretService_Update_NotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	newVal := "val"
	_, err := svc.UpdateSecret(context.Background(), 999, &UpdateSecretRequest{Value: &newVal})
	assert.Error(t, err)
}

// ─── DeleteSecret ─────────────────────────────────────────────────────────────

func TestPipelineSecretService_Delete_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipeline_secrets.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteSecret(context.Background(), 1)
	assert.NoError(t, err)
}

func TestPipelineSecretService_Delete_NotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipeline_secrets.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.DeleteSecret(context.Background(), 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestPipelineSecretService_Delete_DBError(t *testing.T) {
	svc, mock, cleanup := newPipelineSecretService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipeline_secrets.`).WillReturnError(gorm.ErrInvalidData)
	mock.ExpectRollback()

	err := svc.DeleteSecret(context.Background(), 1)
	assert.Error(t, err)
}
