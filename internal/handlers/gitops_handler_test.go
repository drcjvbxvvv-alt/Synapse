package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// Validation tests — no real service needed
// ---------------------------------------------------------------------------

func TestGitOpsHandler_Get_InvalidClusterID(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "clusterID", Value: "abc"}}

	handler.Get(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_Get_InvalidAppID(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "clusterID", Value: "1"},
		{Key: "id", Value: "xyz"},
	}

	handler.Get(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_Create_InvalidClusterID(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "clusterID", Value: "not-a-number"}}

	handler.Create(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_Create_MissingRequiredFields(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "clusterID", Value: "1"}}
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Create(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_Update_InvalidAppID(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "clusterID", Value: "1"},
		{Key: "id", Value: "bad"},
	}

	handler.Update(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_Update_EmptyBody(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "clusterID", Value: "1"},
		{Key: "id", Value: "1"},
	}
	c.Request = httptest.NewRequest("PUT", "/", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Update(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty update, got %d", w.Code)
	}
}

func TestGitOpsHandler_Delete_InvalidAppID(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "clusterID", Value: "1"},
		{Key: "id", Value: "not-a-number"},
	}

	handler.Delete(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_GetDiff_InvalidAppID(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "clusterID", Value: "1"},
		{Key: "id", Value: "abc"},
	}

	handler.GetDiff(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_TriggerSync_InvalidAppID(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "clusterID", Value: "1"},
		{Key: "id", Value: "bad"},
	}

	handler.TriggerSync(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_ListMerged_InvalidClusterID(t *testing.T) {
	handler := &GitOpsHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "clusterID", Value: "abc"}}

	handler.ListMerged(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// argoSyncToStatus
// ---------------------------------------------------------------------------

func TestArgoSyncToStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Synced", "synced"},
		{"OutOfSync", "drifted"},
		{"Unknown", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		got := argoSyncToStatus(tt.input)
		if got != tt.want {
			t.Errorf("argoSyncToStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
