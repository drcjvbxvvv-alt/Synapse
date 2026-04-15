package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// GitOpsHandler — GitOps Application CRUD + Merged List + Diff（M16）
//
// 端點：
//   GET    /clusters/:clusterID/gitops/apps          — 合併列表（native + argocd）
//   GET    /clusters/:clusterID/gitops/apps/:id      — 取得單一 App
//   POST   /clusters/:clusterID/gitops/apps          — 建立 App
//   PUT    /clusters/:clusterID/gitops/apps/:id      — 更新 App
//   DELETE /clusters/:clusterID/gitops/apps/:id      — 刪除 App
//   GET    /clusters/:clusterID/gitops/apps/:id/diff — 取得最近 Diff 結果
//   POST   /clusters/:clusterID/gitops/apps/:id/sync — 手動觸發同步
//   GET    /clusters/:clusterID/gitops/clone-pvcs    — 列出 clone cache PVC
// ---------------------------------------------------------------------------

// GitOpsHandler 管理 GitOps 應用。
type GitOpsHandler struct {
	gitopsSvc *services.GitOpsService
	argoCDSvc *services.ArgoCDService
}

// NewGitOpsHandler 建立 GitOpsHandler。
func NewGitOpsHandler(
	gitopsSvc *services.GitOpsService,
	argoCDSvc *services.ArgoCDService,
) *GitOpsHandler {
	return &GitOpsHandler{
		gitopsSvc: gitopsSvc,
		argoCDSvc: argoCDSvc,
	}
}

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// CreateGitOpsAppRequest 建立 GitOps 應用的請求。
type CreateGitOpsAppRequest struct {
	Name             string `json:"name" binding:"required,max=255"`
	Source           string `json:"source" binding:"required"`    // native / argocd
	GitProviderID    *uint  `json:"git_provider_id,omitempty"`
	RepoURL          string `json:"repo_url,omitempty"`
	Branch           string `json:"branch,omitempty"`
	Path             string `json:"path,omitempty"`
	RenderType       string `json:"render_type" binding:"required"` // raw / kustomize / helm
	HelmValues       string `json:"helm_values,omitempty"`
	Namespace        string `json:"namespace" binding:"required"`
	SyncPolicy       string `json:"sync_policy" binding:"required"` // auto / manual
	SyncInterval     int    `json:"sync_interval"`
	NotifyChannelIDs string `json:"notify_channel_ids,omitempty"` // JSON array
}

// UpdateGitOpsAppRequest 更新 GitOps 應用的請求。
type UpdateGitOpsAppRequest struct {
	RepoURL          *string `json:"repo_url,omitempty"`
	Branch           *string `json:"branch,omitempty"`
	Path             *string `json:"path,omitempty"`
	RenderType       *string `json:"render_type,omitempty"`
	HelmValues       *string `json:"helm_values,omitempty"`
	SyncPolicy       *string `json:"sync_policy,omitempty"`
	SyncInterval     *int    `json:"sync_interval,omitempty"`
	NotifyChannelIDs *string `json:"notify_channel_ids,omitempty"`
}

// MergedAppInfo 合併列表中的統一 App 資訊。
type MergedAppInfo struct {
	ID           uint   `json:"id,omitempty"`            // native only
	Name         string `json:"name"`
	Source       string `json:"source"`                  // native / argocd
	Namespace    string `json:"namespace"`
	RepoURL      string `json:"repo_url,omitempty"`
	Branch       string `json:"branch,omitempty"`
	Path         string `json:"path,omitempty"`
	RenderType   string `json:"render_type,omitempty"`
	SyncPolicy   string `json:"sync_policy,omitempty"`
	Status       string `json:"status"`
	SyncStatus   string `json:"sync_status,omitempty"`   // argocd
	HealthStatus string `json:"health_status,omitempty"` // argocd
	DiffSummary  string `json:"diff_summary,omitempty"`  // native
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// ListMerged 合併列表：native GitOps Apps + ArgoCD Applications。
// GET /clusters/:clusterID/gitops/apps?source=native|argocd
func (h *GitOpsHandler) ListMerged(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	sourceFilter := c.Query("source") // optional: native / argocd / ""(all)

	var merged []MergedAppInfo

	// Native GitOps Apps
	if sourceFilter == "" || sourceFilter == "native" {
		nativeApps, err := h.gitopsSvc.ListApps(c.Request.Context(), clusterID, "native")
		if err != nil {
			logger.Warn("gitops handler: failed to list native apps",
				"cluster_id", clusterID, "error", err)
		} else {
			for _, app := range nativeApps {
				merged = append(merged, MergedAppInfo{
					ID:          app.ID,
					Name:        app.Name,
					Source:      "native",
					Namespace:   app.Namespace,
					RepoURL:     app.RepoURL,
					Branch:      app.Branch,
					Path:        app.Path,
					RenderType:  app.RenderType,
					SyncPolicy:  app.SyncPolicy,
					Status:      app.Status,
					DiffSummary: app.StatusMessage,
				})
			}
		}
	}

	// ArgoCD Applications（代理）
	if sourceFilter == "" || sourceFilter == "argocd" {
		argoApps, err := h.argoCDSvc.ListApplications(c.Request.Context(), clusterID)
		if err != nil {
			// ArgoCD 未配置或不可用 → graceful degradation
			logger.Debug("gitops handler: argocd not available",
				"cluster_id", clusterID, "error", err)
		} else {
			for _, app := range argoApps {
				merged = append(merged, MergedAppInfo{
					Name:         app.Name,
					Source:       "argocd",
					Namespace:    app.Destination.Namespace,
					RepoURL:      app.Source.RepoURL,
					Branch:       app.Source.TargetRevision,
					Path:         app.Source.Path,
					SyncStatus:   app.SyncStatus,
					HealthStatus: app.HealthStatus,
					Status:       argoSyncToStatus(app.SyncStatus),
				})
			}
		}
	}

	response.List(c, merged, int64(len(merged)))
}

// Get 取得單一 native GitOps App。
// GET /clusters/:clusterID/gitops/apps/:id
func (h *GitOpsHandler) Get(c *gin.Context) {
	_, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid app ID")
		return
	}

	app, err := h.gitopsSvc.GetApp(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "gitops app not found")
		return
	}

	response.OK(c, app)
}

// Create 建立 native GitOps App。
// POST /clusters/:clusterID/gitops/apps
func (h *GitOpsHandler) Create(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	var req CreateGitOpsAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	// 從 context 取得 user ID（middleware 注入）
	userID, _ := c.Get("userID")
	uid, _ := userID.(uint)

	app := &models.GitOpsApp{
		Name:             req.Name,
		Source:           req.Source,
		GitProviderID:    req.GitProviderID,
		RepoURL:          req.RepoURL,
		Branch:           req.Branch,
		Path:             req.Path,
		RenderType:       req.RenderType,
		HelmValues:       req.HelmValues,
		ClusterID:        clusterID,
		Namespace:        req.Namespace,
		SyncPolicy:       req.SyncPolicy,
		SyncInterval:     req.SyncInterval,
		NotifyChannelIDs: req.NotifyChannelIDs,
		CreatedBy:        uid,
		Status:           models.GitOpsStatusUnknown,
	}

	if app.SyncInterval == 0 {
		app.SyncInterval = 300 // 預設 5 分鐘
	}

	if err := h.gitopsSvc.CreateApp(c.Request.Context(), app); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	logger.Info("gitops app created",
		"app_id", app.ID,
		"name", app.Name,
		"source", app.Source,
		"cluster_id", clusterID,
	)

	response.OK(c, app)
}

// Update 更新 native GitOps App。
// PUT /clusters/:clusterID/gitops/apps/:id
func (h *GitOpsHandler) Update(c *gin.Context) {
	_, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid app ID")
		return
	}

	var req UpdateGitOpsAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.RepoURL != nil {
		updates["repo_url"] = *req.RepoURL
	}
	if req.Branch != nil {
		updates["branch"] = *req.Branch
	}
	if req.Path != nil {
		updates["path"] = *req.Path
	}
	if req.RenderType != nil {
		updates["render_type"] = *req.RenderType
	}
	if req.HelmValues != nil {
		updates["helm_values"] = *req.HelmValues
	}
	if req.SyncPolicy != nil {
		updates["sync_policy"] = *req.SyncPolicy
	}
	if req.SyncInterval != nil {
		updates["sync_interval"] = *req.SyncInterval
	}
	if req.NotifyChannelIDs != nil {
		updates["notify_channel_ids"] = *req.NotifyChannelIDs
	}

	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}

	if err := h.gitopsSvc.UpdateApp(c.Request.Context(), uint(id), updates); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	logger.Info("gitops app updated", "app_id", id)
	response.OK(c, gin.H{"message": "updated"})
}

// Delete 刪除 native GitOps App。
// DELETE /clusters/:clusterID/gitops/apps/:id
func (h *GitOpsHandler) Delete(c *gin.Context) {
	_, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid app ID")
		return
	}

	if err := h.gitopsSvc.DeleteApp(c.Request.Context(), uint(id)); err != nil {
		response.NotFound(c, err.Error())
		return
	}

	logger.Info("gitops app deleted", "app_id", id)
	response.OK(c, gin.H{"message": "deleted"})
}

// GetDiff 取得最近的 Diff 結果。
// GET /clusters/:clusterID/gitops/apps/:id/diff
func (h *GitOpsHandler) GetDiff(c *gin.Context) {
	_, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid app ID")
		return
	}

	app, err := h.gitopsSvc.GetApp(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "gitops app not found")
		return
	}

	response.OK(c, gin.H{
		"app_id":           app.ID,
		"app_name":         app.Name,
		"status":           app.Status,
		"status_message":   app.StatusMessage,
		"last_diff_at":     app.LastDiffAt,
		"last_diff_result": app.LastDiffResult,
		"last_synced_at":   app.LastSyncedAt,
	})
}

// TriggerSync 手動觸發同步（更新狀態為 syncing）。
// POST /clusters/:clusterID/gitops/apps/:id/sync
func (h *GitOpsHandler) TriggerSync(c *gin.Context) {
	_, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid app ID")
		return
	}

	// 檢查 app 存在
	app, err := h.gitopsSvc.GetApp(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "gitops app not found")
		return
	}

	if app.Source != models.GitOpsSourceNative {
		response.BadRequest(c, "only native gitops apps support manual sync trigger; use ArgoCD sync for argocd apps")
		return
	}

	// 設定狀態為 syncing（reconciler 會在下一個 tick 處理）
	if err := h.gitopsSvc.UpdateSyncStatus(c.Request.Context(), uint(id), models.GitOpsStatusSyncing, "manual sync triggered"); err != nil {
		response.InternalError(c, "failed to trigger sync: "+err.Error())
		return
	}

	logger.Info("gitops manual sync triggered", "app_id", id, "app_name", app.Name)
	response.OK(c, gin.H{"message": "sync triggered"})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// argoSyncToStatus 將 ArgoCD 的 SyncStatus 映射為統一狀態。
func argoSyncToStatus(syncStatus string) string {
	switch syncStatus {
	case "Synced":
		return models.GitOpsStatusSynced
	case "OutOfSync":
		return models.GitOpsStatusDrifted
	default:
		return models.GitOpsStatusUnknown
	}
}
