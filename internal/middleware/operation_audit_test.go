package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/constants"
)

func TestParseRoute_PipelineCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		method       string
		path         string
		wantModule   string
		wantAction   string
		wantResType  string
		wantResName  string
	}{
		{
			name:        "create pipeline",
			method:      "POST",
			path:        "/api/v1/clusters/1/pipelines",
			wantModule:  constants.ModulePipeline,
			wantAction:  constants.ActionCreate,
			wantResType: "pipeline",
		},
		{
			name:        "update pipeline",
			method:      "PUT",
			path:        "/api/v1/clusters/1/pipelines/5",
			wantModule:  constants.ModulePipeline,
			wantAction:  "", // determined by HTTP method
			wantResType: "pipeline",
			wantResName: "5",
		},
		{
			name:        "delete pipeline",
			method:      "DELETE",
			path:        "/api/v1/clusters/1/pipelines/5",
			wantModule:  constants.ModulePipeline,
			wantAction:  "", // determined by HTTP method
			wantResType: "pipeline",
			wantResName: "5",
		},
		{
			name:        "trigger run",
			method:      "POST",
			path:        "/api/v1/clusters/1/pipelines/5/runs",
			wantModule:  constants.ModulePipeline,
			wantAction:  constants.ActionTrigger,
			wantResType: "pipeline_run",
			wantResName: "5",
		},
		{
			name:        "cancel run",
			method:      "POST",
			path:        "/api/v1/clusters/1/pipelines/5/runs/10/cancel",
			wantModule:  constants.ModulePipeline,
			wantAction:  constants.ActionCancel,
			wantResType: "pipeline_run",
			wantResName: "5",
		},
		{
			name:        "rerun pipeline",
			method:      "POST",
			path:        "/api/v1/clusters/1/pipelines/5/runs/10/rerun",
			wantModule:  constants.ModulePipeline,
			wantAction:  constants.ActionRerun,
			wantResType: "pipeline_run",
			wantResName: "5",
		},
		{
			name:        "create version",
			method:      "POST",
			path:        "/api/v1/clusters/1/pipelines/5/versions",
			wantModule:  constants.ModulePipeline,
			wantAction:  constants.ActionCreate,
			wantResType: "pipeline_version",
			wantResName: "5",
		},
		{
			name:        "create pipeline secret",
			method:      "POST",
			path:        "/api/v1/clusters/1/pipeline-secrets",
			wantModule:  constants.ModulePipeline,
			wantAction:  constants.ActionCreate,
			wantResType: "pipeline_secret",
		},
		{
			name:        "update pipeline secret",
			method:      "PUT",
			path:        "/api/v1/clusters/1/pipeline-secrets/3",
			wantModule:  constants.ModulePipeline,
			wantAction:  "", // determined by HTTP method
			wantResType: "pipeline_secret",
			wantResName: "3",
		},
		{
			name:        "webhook trigger",
			method:      "POST",
			path:        "/api/v1/webhooks/pipelines/5/trigger",
			wantModule:  constants.ModulePipeline,
			wantAction:  constants.ActionTrigger,
			wantResType: "pipeline_webhook",
			wantResName: "5",
		},
		{
			name:        "approve step",
			method:      "POST",
			path:        "/api/v1/clusters/1/pipelines/5/runs/10/steps/20/approve",
			wantModule:  constants.ModulePipeline,
			wantAction:  "approve",
			wantResType: "pipeline_step",
			wantResName: "5",
		},
		{
			name:        "reject step",
			method:      "POST",
			path:        "/api/v1/clusters/1/pipelines/5/runs/10/steps/20/reject",
			wantModule:  constants.ModulePipeline,
			wantAction:  "reject",
			wantResType: "pipeline_step",
			wantResName: "5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tc.method, tc.path, nil)

			module, action, resType, resName := parseRoute(c, tc.path)

			if module != tc.wantModule {
				t.Errorf("module: got %q, want %q", module, tc.wantModule)
			}
			if action != tc.wantAction {
				t.Errorf("action: got %q, want %q", action, tc.wantAction)
			}
			if resType != tc.wantResType {
				t.Errorf("resourceType: got %q, want %q", resType, tc.wantResType)
			}
			if tc.wantResName != "" && resName != tc.wantResName {
				t.Errorf("resourceName: got %q, want %q", resName, tc.wantResName)
			}
		})
	}
}

func TestParseRoute_ExistingModulesUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		wantModule string
	}{
		{"login", "/api/v1/auth/login", constants.ModuleAuth},
		{"cluster import", "/api/v1/clusters/import", constants.ModuleCluster},
		{"node cordon", "/api/v1/clusters/1/nodes/mynode/cordon", constants.ModuleNode},
		{"deployment scale", "/api/v1/clusters/1/deployments/default/nginx/scale", constants.ModuleWorkload},
		{"argocd sync", "/api/v1/clusters/1/argocd/applications/myapp/sync", constants.ModuleArgoCD},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)

			module, _, _, _ := parseRoute(c, tc.path)
			if module != tc.wantModule {
				t.Errorf("module: got %q, want %q", module, tc.wantModule)
			}
		})
	}
}

func TestGuessResourceType_Pipeline(t *testing.T) {
	// guessResourceType is a last-resort fallback — it picks the first matching segment.
	// For paths under /clusters/..., it returns "cluster" because that appears first.
	// Pipeline paths are handled by explicit route rules (tested above), not this fallback.
	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/pipelines", "pipelin"}, // "es" suffix stripped by guessResourceType
		{"/api/v1/pipeline-secrets/3", "pipeline-secret"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := guessResourceType(tc.path)
			if got != tc.want {
				t.Errorf("guessResourceType(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
