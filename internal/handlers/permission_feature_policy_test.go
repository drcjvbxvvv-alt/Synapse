package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services"
)

// ─── fixture ──────────────────────────────────────────────────────────────────

type featurePolicyFixture struct {
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	handler *PermissionHandler
	router  *gin.Engine
}

func newFeaturePolicyFixture(t *testing.T) *featurePolicyFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	permSvc := services.NewPermissionService(db, nil)
	h := NewPermissionHandler(permSvc, nil, nil)

	r := gin.New()
	r.GET("/permissions/cluster-permissions/:id/features", h.GetFeaturePolicy)
	r.PATCH("/permissions/cluster-permissions/:id/features", h.UpdateFeaturePolicy)

	return &featurePolicyFixture{db: db, mock: mock, handler: h, router: r}
}

// expectGetPermission sets up mock expectations for fetching a ClusterPermission by ID.
func (f *featurePolicyFixture) expectGetPermission(id uint, permType, featurePolicy string) {
	uid := uint(1)
	rows := sqlmock.NewRows([]string{
		"id", "cluster_id", "user_id", "user_group_id", "permission_type",
		"namespaces", "custom_role_ref", "feature_policy", "created_at", "updated_at", "deleted_at",
	}).AddRow(id, 1, uid, nil, permType, `["*"]`, "", featurePolicy, nil, nil, nil)
	f.mock.ExpectQuery(`SELECT .* FROM "cluster_permissions"`).
		WithArgs(id, 1).
		WillReturnRows(rows)
	// Preload("Cluster")
	f.mock.ExpectQuery(`SELECT .* FROM "clusters"`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "test-cluster"))
	// Preload("User")
	f.mock.ExpectQuery(`SELECT .* FROM "users"`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow(uid, "testuser"))
	// Preload("UserGroup") — user_group_id is nil so no query expected
}

// expectUpdatePermission sets up mock expectations for updating feature_policy.
// PostgreSQL GORM may try to upsert preloaded associations before updating
// the target row, so we need to handle those association saves.
func (f *featurePolicyFixture) expectUpdatePermission(id uint) {
	f.mock.ExpectBegin()
	// Association save: INSERT INTO "clusters" ... ON CONFLICT DO NOTHING RETURNING "id"
	f.mock.ExpectQuery(`INSERT INTO "clusters"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	// Association save: INSERT INTO "users" ... ON CONFLICT DO NOTHING RETURNING "id"
	f.mock.ExpectQuery(`INSERT INTO "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	f.mock.ExpectExec(`UPDATE "cluster_permissions"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectCommit()
}

func fpDoRequest(r *gin.Engine, method, url string, body interface{}) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, url, bytes.NewReader(b))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func uintStr(id uint) string {
	return func() string {
		b, _ := json.Marshal(id)
		return string(b)
	}()
}

// ─── GetFeaturePolicy ─────────────────────────────────────────────────────────

func TestGetFeaturePolicy_ValidDevPermission(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.expectGetPermission(1, models.PermissionTypeDev, "")

	w := fpDoRequest(fix.router, http.MethodGet,
		"/permissions/cluster-permissions/1/features", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var body FeaturePolicyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Equal(t, uint(1), body.PermissionID)
	assert.Equal(t, models.PermissionTypeDev, body.PermissionType)
	assert.ElementsMatch(t, models.FeatureCeilings[models.PermissionTypeDev], body.Ceiling)
	assert.ElementsMatch(t, models.FeatureCeilings[models.PermissionTypeDev], body.Effective)
	assert.Empty(t, body.Policy)
}

func TestGetFeaturePolicy_WithExistingPolicy(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.expectGetPermission(1, models.PermissionTypeDev, `{"terminal:pod":false}`)

	w := fpDoRequest(fix.router, http.MethodGet,
		"/permissions/cluster-permissions/1/features", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var body FeaturePolicyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Equal(t, map[string]bool{models.FeatureTerminalPod: false}, body.Policy)
	assert.NotContains(t, body.Effective, models.FeatureTerminalPod)
	assert.Contains(t, body.Effective, models.FeatureWorkloadView)
}

func TestGetFeaturePolicy_InvalidID(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	w := fpDoRequest(fix.router, http.MethodGet,
		"/permissions/cluster-permissions/abc/features", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetFeaturePolicy_NotFound(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.mock.ExpectQuery(`SELECT .* FROM "cluster_permissions"`).
		WithArgs(999, 1).
		WillReturnRows(sqlmock.NewRows(nil))

	w := fpDoRequest(fix.router, http.MethodGet,
		"/permissions/cluster-permissions/999/features", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── UpdateFeaturePolicy ──────────────────────────────────────────────────────

func TestUpdateFeaturePolicy_DisableTerminalPod(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.expectGetPermission(1, models.PermissionTypeDev, "")
	fix.expectUpdatePermission(1)

	w := fpDoRequest(fix.router, http.MethodPatch,
		"/permissions/cluster-permissions/1/features",
		map[string]interface{}{"policy": map[string]bool{models.FeatureTerminalPod: false}})
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Effective []string `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotContains(t, resp.Effective, models.FeatureTerminalPod)
	assert.Contains(t, resp.Effective, models.FeatureWorkloadView)
}

func TestUpdateFeaturePolicy_CeilingEnforced_DevHasTerminalNode(t *testing.T) {
	// Dev ceiling is allFeatureKeys(), so terminal:node is allowed.
	// Explicitly enabling it should keep it in the effective set.
	fix := newFeaturePolicyFixture(t)
	fix.expectGetPermission(1, models.PermissionTypeDev, "")
	fix.expectUpdatePermission(1)

	w := fpDoRequest(fix.router, http.MethodPatch,
		"/permissions/cluster-permissions/1/features",
		map[string]interface{}{"policy": map[string]bool{models.FeatureTerminalNode: true}})
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Effective []string `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Effective, models.FeatureTerminalNode)
}

func TestUpdateFeaturePolicy_ReadonlyCannotGetExport(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.expectGetPermission(1, models.PermissionTypeReadonly, "")
	fix.expectUpdatePermission(1)

	w := fpDoRequest(fix.router, http.MethodPatch,
		"/permissions/cluster-permissions/1/features",
		map[string]interface{}{"policy": map[string]bool{models.FeatureExport: true}})
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Effective []string `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotContains(t, resp.Effective, models.FeatureExport)
}

func TestUpdateFeaturePolicy_InvalidBody(t *testing.T) {
	fix := newFeaturePolicyFixture(t)

	req, _ := http.NewRequest(http.MethodPatch,
		"/permissions/cluster-permissions/1/features",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	fix.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
