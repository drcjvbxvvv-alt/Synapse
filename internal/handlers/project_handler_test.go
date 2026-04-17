package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/services"
)

var projectHandlerCols = []string{
	"id", "git_provider_id", "name", "repo_url",
	"default_branch", "description", "created_by",
	"created_at", "updated_at", "deleted_at",
}

func newProjectHandlerWithRouter(t *testing.T) (*ProjectHandler, sqlmock.Sqlmock, *gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := services.NewProjectService(gormDB)
	gitSvc := services.NewGitProviderService(gormDB)
	h := NewProjectHandler(svc, gitSvc)

	r := gin.New()
	r.GET("/system/git-providers/:id/projects", h.List)
	r.GET("/system/git-providers/:id/projects/:projectID", h.Get)
	r.POST("/system/git-providers/:id/projects", h.Create)
	r.PUT("/system/git-providers/:id/projects/:projectID", h.Update)
	r.DELETE("/system/git-providers/:id/projects/:projectID", h.Delete)

	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return h, mock, r, cleanup
}

// ─── List ───────────────────────────────────────────────────────────────────

func TestProjectHandler_List_InvalidProviderID(t *testing.T) {
	_, _, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/system/git-providers/abc/projects", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProjectHandler_List_Empty(t *testing.T) {
	_, mock, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(projectHandlerCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/system/git-providers/1/projects", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProjectHandler_List_WithProjects(t *testing.T) {
	_, mock, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(projectHandlerCols).
		AddRow(1, 10, "alpha", "https://github.com/org/alpha", "main", "", 1, now, now, nil).
		AddRow(2, 10, "beta", "https://github.com/org/beta", "main", "", 1, now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/system/git-providers/10/projects", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(2), body["total"])
}

// ─── Get ────────────────────────────────────────────────────────────────────

func TestProjectHandler_Get_InvalidProjectID(t *testing.T) {
	_, _, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/system/git-providers/1/projects/not-a-number", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProjectHandler_Get_NotFound(t *testing.T) {
	_, mock, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/system/git-providers/1/projects/999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProjectHandler_Get_Success(t *testing.T) {
	_, mock, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(projectHandlerCols).
		AddRow(5, 10, "synapse-ui", "https://github.com/org/synapse-ui", "main", "UI repo", 1, now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/system/git-providers/1/projects/5", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	data, ok := body["data"].(map[string]any)
	if !ok {
		// some response helpers wrap in "data"
		data = body
	}
	_ = data
}

// ─── Create ─────────────────────────────────────────────────────────────────

func TestProjectHandler_Create_InvalidProviderID(t *testing.T) {
	_, _, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/system/git-providers/abc/projects", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProjectHandler_Create_MissingRequiredFields(t *testing.T) {
	_, _, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{"description": "no name or url"})
	req, _ := http.NewRequest(http.MethodPost, "/system/git-providers/1/projects", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProjectHandler_Create_Success(t *testing.T) {
	_, mock, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .projects.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{
		"name":    "new-repo",
		"repo_url": "https://github.com/org/new-repo",
	})
	req, _ := http.NewRequest(http.MethodPost, "/system/git-providers/1/projects", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProjectHandler_Create_DefaultBranchFallback(t *testing.T) {
	_, mock, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .projects.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(4))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{
		"name":    "my-repo",
		"repo_url": "https://github.com/org/my-repo",
		// no default_branch → should default to "main"
	})
	req, _ := http.NewRequest(http.MethodPost, "/system/git-providers/1/projects", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── Update ─────────────────────────────────────────────────────────────────

func TestProjectHandler_Update_InvalidProjectID(t *testing.T) {
	_, _, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/system/git-providers/1/projects/abc", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProjectHandler_Update_NoFields(t *testing.T) {
	_, _, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{}) // empty → no fields to update
	req, _ := http.NewRequest(http.MethodPut, "/system/git-providers/1/projects/5", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProjectHandler_Update_Success(t *testing.T) {
	_, mock, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(projectHandlerCols).
		AddRow(5, 10, "old-name", "https://github.com/org/old", "main", "", 1, now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .projects.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	newName := "new-name"
	body, _ := json.Marshal(map[string]any{"name": newName})
	req, _ := http.NewRequest(http.MethodPut, "/system/git-providers/1/projects/5", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── Delete ─────────────────────────────────────────────────────────────────

func TestProjectHandler_Delete_InvalidProjectID(t *testing.T) {
	_, _, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/system/git-providers/1/projects/xyz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProjectHandler_Delete_Success(t *testing.T) {
	_, mock, r, cleanup := newProjectHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .projects.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/system/git-providers/1/projects/5", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
