package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func newCIEngineHandlerTest(t *testing.T) (*CIEngineHandler, sqlmock.Sqlmock, *gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)

	f := engine.NewFactory()
	svc := services.NewCIEngineService(gormDB, f)
	h := NewCIEngineHandler(svc)

	r := gin.New()
	// Simulate AuthMiddleware having populated user_id (avoids pulling in real
	// auth middleware for unit tests).
	r.Use(func(c *gin.Context) { c.Set("user_id", uint(42)) })
	r.GET("/api/v1/ci-engines/status", h.ListAvailable)
	r.GET("/api/v1/ci-engines", h.List)
	r.GET("/api/v1/ci-engines/:id", h.Get)
	r.POST("/api/v1/ci-engines", h.Create)
	r.PUT("/api/v1/ci-engines/:id", h.Update)
	r.DELETE("/api/v1/ci-engines/:id", h.Delete)

	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return h, mock, r, cleanup
}

// ---------------------------------------------------------------------------
// ListAvailable (native-only)
// ---------------------------------------------------------------------------

func TestCIEngineHandler_ListAvailable_OnlyNative(t *testing.T) {
	_, mock, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	// ListAvailableEngines queries ci_engine_configs to list external
	// configs; return empty.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/status", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 1, body.Total)
	assert.Equal(t, "native", body.Items[0]["type"])
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCIEngineHandler_Create_BadJSON(t *testing.T) {
	_, _, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ci-engines", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestCIEngineHandler_Create_InvalidEngineType(t *testing.T) {
	_, _, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	payload := map[string]any{
		"name":        "bad",
		"engine_type": "circleci",
	}
	buf, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ci-engines", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "CI_ENGINE_TYPE_INVALID")
}

func TestCIEngineHandler_Create_Success(t *testing.T) {
	_, mock, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	payload := map[string]any{
		"name":        "gitlab-prod",
		"engine_type": "gitlab",
		"endpoint":    "https://gitlab.example.com",
		"auth_type":   "token",
		"token":       "pat-123",
	}
	buf, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ci-engines", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(1), body["id"])
	// Credentials must NOT leak back.
	for _, forbidden := range []string{"token", "password", "webhook_secret", "ca_bundle"} {
		if _, present := body[forbidden]; present {
			t.Fatalf("forbidden field %q leaked: %v", forbidden, body)
		}
	}
}

// ---------------------------------------------------------------------------
// Get / Update / Delete
// ---------------------------------------------------------------------------

func TestCIEngineHandler_Get_InvalidID(t *testing.T) {
	_, _, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/not-a-number", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestCIEngineHandler_Get_NotFound(t *testing.T) {
	_, mock, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WithArgs(uint(999), 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "CI_ENGINE_NOT_FOUND")
}

func TestCIEngineHandler_Update_TypeImmutable(t *testing.T) {
	_, mock, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "name", "engine_type"}).
		AddRow(1, "gl", "gitlab")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WithArgs(uint(1), 1).
		WillReturnRows(rows)

	payload := map[string]any{
		"name":        "gl",
		"engine_type": "jenkins", // forbidden change
	}
	buf, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/ci-engines/1", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "CI_ENGINE_TYPE_IMMUTABLE")
}

func TestCIEngineHandler_Delete_NoContent(t *testing.T) {
	_, mock, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "name", "engine_type"}).
		AddRow(1, "gl", "gitlab")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WithArgs(uint(1), 1).
		WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "ci_engine_configs" SET "deleted_at"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/ci-engines/1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
}

// ---------------------------------------------------------------------------
// List (external configs)
// ---------------------------------------------------------------------------

func TestCIEngineHandler_List_Empty(t *testing.T) {
	_, mock, r, cleanup := newCIEngineHandlerTest(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"total":0`)
}
