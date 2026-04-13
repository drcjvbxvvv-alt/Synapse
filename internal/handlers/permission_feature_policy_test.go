package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services"
)

// ─── fixture ──────────────────────────────────────────────────────────────────

type featurePolicyFixture struct {
	db      *gorm.DB
	handler *PermissionHandler
	router  *gin.Engine
}

func newFeaturePolicyFixture(t *testing.T) *featurePolicyFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ClusterPermission{}, &models.Cluster{}, &models.User{}, &models.UserGroup{}))

	// Seed a minimal cluster and user so Preload doesn't fail.
	cluster := models.Cluster{Name: "test-cluster"}
	require.NoError(t, db.Create(&cluster).Error)
	user := models.User{Username: "testuser", PasswordHash: "x", Salt: "x"}
	require.NoError(t, db.Create(&user).Error)

	permSvc := services.NewPermissionService(db, nil)
	h := NewPermissionHandler(permSvc, nil, nil)

	r := gin.New()
	r.GET("/permissions/cluster-permissions/:id/features", h.GetFeaturePolicy)
	r.PATCH("/permissions/cluster-permissions/:id/features", h.UpdateFeaturePolicy)

	return &featurePolicyFixture{db: db, handler: h, router: r}
}

func (f *featurePolicyFixture) seedPermission(t *testing.T, permType, featurePolicy string) *models.ClusterPermission {
	t.Helper()
	uid := uint(1)
	p := &models.ClusterPermission{
		ClusterID:      1,
		UserID:         &uid,
		PermissionType: permType,
		Namespaces:     `["*"]`,
		FeaturePolicy:  featurePolicy,
	}
	require.NoError(t, f.db.Create(p).Error)
	return p
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
	perm := fix.seedPermission(t, models.PermissionTypeDev, "")

	w := fpDoRequest(fix.router, http.MethodGet,
		"/permissions/cluster-permissions/1/features", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var body FeaturePolicyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Equal(t, perm.ID, body.PermissionID)
	assert.Equal(t, models.PermissionTypeDev, body.PermissionType)
	assert.ElementsMatch(t, models.FeatureCeilings[models.PermissionTypeDev], body.Ceiling)
	assert.ElementsMatch(t, models.FeatureCeilings[models.PermissionTypeDev], body.Effective)
	assert.Empty(t, body.Policy)
}

func TestGetFeaturePolicy_WithExistingPolicy(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.seedPermission(t, models.PermissionTypeDev, `{"terminal:pod":false}`)

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
	w := fpDoRequest(fix.router, http.MethodGet,
		"/permissions/cluster-permissions/999/features", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── UpdateFeaturePolicy ──────────────────────────────────────────────────────

func TestUpdateFeaturePolicy_DisableTerminalPod(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.seedPermission(t, models.PermissionTypeDev, "")

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

func TestUpdateFeaturePolicy_CeilingEnforced_DevCannotGetTerminalNode(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.seedPermission(t, models.PermissionTypeDev, "")

	// terminal:node is NOT in dev ceiling — must be silently dropped.
	w := fpDoRequest(fix.router, http.MethodPatch,
		"/permissions/cluster-permissions/1/features",
		map[string]interface{}{"policy": map[string]bool{models.FeatureTerminalNode: true}})
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Effective []string `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotContains(t, resp.Effective, models.FeatureTerminalNode)
}

func TestUpdateFeaturePolicy_ReadonlyCannotGetExport(t *testing.T) {
	fix := newFeaturePolicyFixture(t)
	fix.seedPermission(t, models.PermissionTypeReadonly, "")

	// export is NOT in readonly ceiling — must be silently dropped.
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
	fix.seedPermission(t, models.PermissionTypeDev, "")

	req, _ := http.NewRequest(http.MethodPatch,
		"/permissions/cluster-permissions/1/features",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	fix.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
