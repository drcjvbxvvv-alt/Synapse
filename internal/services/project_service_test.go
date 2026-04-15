package services

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/models"
)

// projectCols returns the column list used in SELECT queries for Project.
func projectCols() []string {
	return []string{
		"id", "git_provider_id", "name", "repo_url",
		"default_branch", "description", "created_by",
		"created_at", "updated_at", "deleted_at",
	}
}

func newProjectSvcDB(t *testing.T) (*ProjectService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	svc := NewProjectService(gormDB)
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

// ─── ListProjects ──────────────────────────────────────────────────────────

func TestProjectService_ListProjects_Empty(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(projectCols()))

	projects, err := svc.ListProjects(context.Background(), 1)
	assert.NoError(t, err)
	assert.Empty(t, projects)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectService_ListProjects_Multiple(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(projectCols()).
		AddRow(1, 10, "alpha", "https://github.com/org/alpha", "main", "", 1, now, now, nil).
		AddRow(2, 10, "beta", "https://github.com/org/beta", "main", "desc", 1, now, now, nil)

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	projects, err := svc.ListProjects(context.Background(), 10)
	assert.NoError(t, err)
	assert.Len(t, projects, 2)
	assert.Equal(t, "alpha", projects[0].Name)
	assert.Equal(t, "beta", projects[1].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectService_ListProjects_DBError(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidDB)

	projects, err := svc.ListProjects(context.Background(), 1)
	assert.Error(t, err)
	assert.Nil(t, projects)
	assert.Contains(t, err.Error(), "list projects for provider")
}

// ─── GetProject ────────────────────────────────────────────────────────────

func TestProjectService_GetProject_Found(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(projectCols()).
		AddRow(5, 10, "my-repo", "https://github.com/org/my-repo", "main", "", 2, now, now, nil)

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	p, err := svc.GetProject(context.Background(), 5)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, uint(5), p.ID)
	assert.Equal(t, "my-repo", p.Name)
}

func TestProjectService_GetProject_NotFound(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	p, err := svc.GetProject(context.Background(), 999)
	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Contains(t, err.Error(), "not found")
}

// ─── GetProjectByRepoURL ────────────────────────────────────────────────────

func TestProjectService_GetProjectByRepoURL_Found(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(projectCols()).
		AddRow(3, 10, "synapse", "https://github.com/org/synapse", "main", "", 1, now, now, nil)

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	p, err := svc.GetProjectByRepoURL(context.Background(), "https://github.com/org/synapse")
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "https://github.com/org/synapse", p.RepoURL)
}

func TestProjectService_GetProjectByRepoURL_NotFound(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	p, err := svc.GetProjectByRepoURL(context.Background(), "https://github.com/org/missing")
	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Contains(t, err.Error(), "not found")
}

// ─── CreateProject ─────────────────────────────────────────────────────────

func TestProjectService_CreateProject_Success(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .projects.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	project := &models.Project{
		GitProviderID: 10,
		Name:          "new-project",
		RepoURL:       "https://github.com/org/new-project",
		DefaultBranch: "main",
		CreatedBy:     1,
	}

	err := svc.CreateProject(context.Background(), project)
	assert.NoError(t, err)
	assert.Equal(t, uint(1), project.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectService_CreateProject_DBError(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .projects.`).WillReturnError(gorm.ErrInvalidDB)
	mock.ExpectRollback()

	project := &models.Project{
		GitProviderID: 10,
		Name:          "dup-project",
		RepoURL:       "https://github.com/org/dup",
		CreatedBy:     1,
	}

	err := svc.CreateProject(context.Background(), project)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create project")
}

// ─── UpdateProject ─────────────────────────────────────────────────────────

func TestProjectService_UpdateProject_Success(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(projectCols()).
		AddRow(7, 10, "old-name", "https://github.com/org/old", "main", "", 1, now, now, nil)

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .projects.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	p, err := svc.UpdateProject(context.Background(), 7, map[string]any{
		"name":        "new-name",
		"description": "updated",
	})
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectService_UpdateProject_NotFound(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	p, err := svc.UpdateProject(context.Background(), 999, map[string]any{"name": "x"})
	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Contains(t, err.Error(), "not found")
}

// ─── DeleteProject ─────────────────────────────────────────────────────────

func TestProjectService_DeleteProject_Success(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .projects.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteProject(context.Background(), 5)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectService_DeleteProject_DBError(t *testing.T) {
	svc, mock, cleanup := newProjectSvcDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .projects.`).WillReturnError(gorm.ErrInvalidDB)
	mock.ExpectRollback()

	err := svc.DeleteProject(context.Background(), 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete project")
}
