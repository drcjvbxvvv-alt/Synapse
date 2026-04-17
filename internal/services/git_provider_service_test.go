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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newGitProviderService(t *testing.T) (*GitProviderService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := NewGitProviderService(gormDB)
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

var gitProviderCols = []string{
	"id", "name", "type", "base_url",
	"access_token_enc", "webhook_secret_enc", "webhook_token",
	"enabled", "created_by", "created_at", "updated_at", "deleted_at",
}

func gitProviderRow(mock sqlmock.Sqlmock) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(gitProviderCols).AddRow(
		1, "github-corp", "github", "https://github.com",
		"", "", "abcdef1234567890",
		true, 1, now, now, nil,
	)
}

// ---------------------------------------------------------------------------
// CreateProvider
// ---------------------------------------------------------------------------

func TestGitProviderService_Create_Success(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .git_providers.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	p := newGitProvider()
	err := svc.CreateProvider(context.Background(), p)
	assert.NoError(t, err)
}

func TestGitProviderService_Create_InvalidType(t *testing.T) {
	svc, _, cleanup := newGitProviderService(t)
	defer cleanup()

	p := newGitProvider()
	p.Type = "bitbucket"
	err := svc.CreateProvider(context.Background(), p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid git provider type")
}

func TestGitProviderService_Create_DBError(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .git_providers.`).
		WillReturnError(gorm.ErrInvalidData)
	mock.ExpectRollback()

	err := svc.CreateProvider(context.Background(), newGitProvider())
	assert.Error(t, err)
}

func TestGitProviderService_Create_AutoGeneratesToken(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .git_providers.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	mock.ExpectCommit()

	p := newGitProvider()
	p.WebhookToken = "" // no token provided
	err := svc.CreateProvider(context.Background(), p)
	assert.NoError(t, err)
	// token was auto-generated
	assert.NotEmpty(t, p.WebhookToken)
	assert.Equal(t, 64, len(p.WebhookToken))
}

// ---------------------------------------------------------------------------
// GetProvider
// ---------------------------------------------------------------------------

func TestGitProviderService_Get_Success(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(gitProviderRow(mock))

	got, err := svc.GetProvider(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "github-corp", got.Name)
	assert.Equal(t, "github", got.Type)
}

func TestGitProviderService_Get_NotFound(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	got, err := svc.GetProvider(context.Background(), 999)
	assert.Error(t, err)
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// GetProviderByWebhookToken
// ---------------------------------------------------------------------------

func TestGitProviderService_GetByToken_Success(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(gitProviderRow(mock))

	got, err := svc.GetProviderByWebhookToken(context.Background(), "abcdef1234567890")
	require.NoError(t, err)
	assert.Equal(t, "github-corp", got.Name)
}

func TestGitProviderService_GetByToken_NotFound(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	got, err := svc.GetProviderByWebhookToken(context.Background(), "nosuchtoken")
	assert.Error(t, err)
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// ListProviders
// ---------------------------------------------------------------------------

func TestGitProviderService_List_Empty(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "type", "base_url", "webhook_token", "enabled", "created_by", "created_at", "updated_at"}),
	)

	providers, err := svc.ListProviders(context.Background())
	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestGitProviderService_List_Multiple(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	now := time.Now()
	cols := []string{"id", "name", "type", "base_url", "webhook_token", "enabled", "created_by", "created_at", "updated_at"}
	rows := sqlmock.NewRows(cols).
		AddRow(1, "github-corp", "github", "https://github.com", "token1", true, 1, now, now).
		AddRow(2, "gitlab-corp", "gitlab", "https://gitlab.com", "token2", true, 1, now, now)

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	providers, err := svc.ListProviders(context.Background())
	require.NoError(t, err)
	assert.Len(t, providers, 2)
	assert.Equal(t, "github-corp", providers[0].Name)
	assert.Equal(t, "gitlab-corp", providers[1].Name)
}

// ---------------------------------------------------------------------------
// UpdateProvider
// ---------------------------------------------------------------------------

func TestGitProviderService_Update_Success(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .git_providers.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.UpdateProvider(context.Background(), 1, map[string]interface{}{"name": "new-name"})
	assert.NoError(t, err)
}

func TestGitProviderService_Update_InvalidType(t *testing.T) {
	svc, _, cleanup := newGitProviderService(t)
	defer cleanup()

	err := svc.UpdateProvider(context.Background(), 1, map[string]interface{}{"type": "bitbucket"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid git provider type")
}

func TestGitProviderService_Update_NotFound(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .git_providers.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.UpdateProvider(context.Background(), 999, map[string]interface{}{"name": "x"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGitProviderService_Update_DBError(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .git_providers.`).WillReturnError(gorm.ErrInvalidData)
	mock.ExpectRollback()

	err := svc.UpdateProvider(context.Background(), 1, map[string]interface{}{"name": "x"})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// DeleteProvider
// ---------------------------------------------------------------------------

func TestGitProviderService_Delete_Success(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .git_providers.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteProvider(context.Background(), 1)
	assert.NoError(t, err)
}

func TestGitProviderService_Delete_NotFound(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .git_providers.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.DeleteProvider(context.Background(), 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// RegenerateWebhookToken
// ---------------------------------------------------------------------------

func TestGitProviderService_RegenerateToken_Success(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .git_providers.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	token, err := svc.RegenerateWebhookToken(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 64, len(token))
}

func TestGitProviderService_RegenerateToken_NotFound(t *testing.T) {
	svc, mock, cleanup := newGitProviderService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .git_providers.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	_, err := svc.RegenerateWebhookToken(context.Background(), 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// extractRepoPath
// ---------------------------------------------------------------------------

func TestExtractRepoPath(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		repoURL  string
		expected string
		wantErr  bool
	}{
		{"GitLab standard", "http://localhost:8929", "http://localhost:8929/root/my-repo.git", "root/my-repo", false},
		{"without .git suffix", "http://localhost:8929", "http://localhost:8929/root/my-repo", "root/my-repo", false},
		{"with tree path", "http://localhost:8929", "http://localhost:8929/root/saas-uat/-/tree/main", "root/saas-uat", false},
		{"GitHub HTTPS", "https://github.com", "https://github.com/org/service.git", "org/service", false},
		{"nested group", "https://gitlab.com", "https://gitlab.com/company/team/project.git", "company/team/project", false},
		{"case insensitive", "HTTP://LOCALHOST:8929", "http://localhost:8929/root/app", "root/app", false},
		{"empty path", "http://localhost:8929", "http://localhost:8929/", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractRepoPath(tt.baseURL, tt.repoURL)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// ---------------------------------------------------------------------------
// fixture
// ---------------------------------------------------------------------------

func newGitProvider() *models.GitProvider {
	return &models.GitProvider{
		Name:         "github-corp",
		Type:         "github",
		BaseURL:      "https://github.com",
		WebhookToken: "existingtoken1234567890abcdef",
		Enabled:      true,
		CreatedBy:    1,
	}
}
