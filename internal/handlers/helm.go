package handlers

import (
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"helm.sh/helm/v3/pkg/release"
)

// ReleaseResponse Helm Release 回應結構
type ReleaseResponse struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Chart      string `json:"chart"`
	Version    string `json:"version"`
	AppVersion string `json:"app_version"`
	Status     string `json:"status"`
	Revision   int    `json:"revision"`
	UpdatedAt  string `json:"updated_at"`
}

// HelmHandler Helm 處理器
type HelmHandler struct {
	clusterService *services.ClusterService
	helmService    *services.HelmService
}

// NewHelmHandler 建立 Helm 處理器
func NewHelmHandler(clusterService *services.ClusterService, helmSvc *services.HelmService) *HelmHandler {
	return &HelmHandler{
		clusterService: clusterService,
		helmService:    helmSvc,
	}
}

// toReleaseResponse 將 release.Release 轉換為 ReleaseResponse
func toReleaseResponse(r *release.Release) ReleaseResponse {
	chartName := ""
	version := ""
	appVersion := ""
	if r.Chart != nil && r.Chart.Metadata != nil {
		chartName = r.Chart.Metadata.Name + "-" + r.Chart.Metadata.Version
		version = r.Chart.Metadata.Version
		appVersion = r.Chart.Metadata.AppVersion
	}
	updatedAt := ""
	if !r.Info.LastDeployed.IsZero() {
		updatedAt = r.Info.LastDeployed.UTC().Format("2006-01-02T15:04:05Z")
	}
	return ReleaseResponse{
		Name:       r.Name,
		Namespace:  r.Namespace,
		Chart:      chartName,
		Version:    version,
		AppVersion: appVersion,
		Status:     string(r.Info.Status),
		Revision:   r.Version,
		UpdatedAt:  updatedAt,
	}
}

// ListReleases 列出 Helm Releases
// GET /clusters/:id/helm/releases?namespace=
func (h *HelmHandler) ListReleases(c *gin.Context) {
	clusterIDStr := c.Param("id")
	namespace := c.Query("namespace")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	releases, err := h.helmService.ListReleases(cluster, namespace)
	if err != nil {
		logger.Error("列出 Helm Releases 失敗", "cluster", cluster.Name, "error", err)
		response.InternalError(c, "列出 Helm Releases 失敗: "+err.Error())
		return
	}

	items := make([]ReleaseResponse, 0, len(releases))
	for _, r := range releases {
		items = append(items, toReleaseResponse(r))
	}

	response.List(c, items, int64(len(items)))
}

// GetRelease 取得單一 Helm Release 詳情
// GET /clusters/:id/helm/releases/:namespace/:name
func (h *HelmHandler) GetRelease(c *gin.Context) {
	clusterIDStr := c.Param("id")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	rel, err := h.helmService.GetRelease(cluster, namespace, name)
	if err != nil {
		logger.Error("取得 Helm Release 失敗", "cluster", cluster.Name, "name", name, "error", err)
		response.NotFound(c, "Helm Release 不存在: "+err.Error())
		return
	}

	resp := toReleaseResponse(rel)
	if rel.Config != nil {
		response.OK(c, gin.H{
			"name":        resp.Name,
			"namespace":   resp.Namespace,
			"chart":       resp.Chart,
			"version":     resp.Version,
			"app_version": resp.AppVersion,
			"status":      resp.Status,
			"revision":    resp.Revision,
			"updated_at":  resp.UpdatedAt,
			"values":      rel.Config,
			"notes":       rel.Info.Notes,
		})
		return
	}
	response.OK(c, resp)
}

// GetReleaseHistory 取得 Helm Release 歷史版本
// GET /clusters/:id/helm/releases/:namespace/:name/history
func (h *HelmHandler) GetReleaseHistory(c *gin.Context) {
	clusterIDStr := c.Param("id")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	history, err := h.helmService.GetHistory(cluster, namespace, name)
	if err != nil {
		logger.Error("取得 Helm Release 歷史失敗", "cluster", cluster.Name, "name", name, "error", err)
		response.InternalError(c, "取得歷史失敗: "+err.Error())
		return
	}

	type HistoryEntry struct {
		Revision   int    `json:"revision"`
		UpdatedAt  string `json:"updated_at"`
		Status     string `json:"status"`
		Chart      string `json:"chart"`
		AppVersion string `json:"app_version"`
		Description string `json:"description"`
	}

	items := make([]HistoryEntry, 0, len(history))
	for _, r := range history {
		chart := ""
		appVersion := ""
		if r.Chart != nil && r.Chart.Metadata != nil {
			chart = r.Chart.Metadata.Name + "-" + r.Chart.Metadata.Version
			appVersion = r.Chart.Metadata.AppVersion
		}
		updatedAt := ""
		description := ""
		if r.Info != nil {
			if !r.Info.LastDeployed.IsZero() {
				updatedAt = r.Info.LastDeployed.UTC().Format("2006-01-02T15:04:05Z")
			}
			description = r.Info.Description
		}
		items = append(items, HistoryEntry{
			Revision:    r.Version,
			UpdatedAt:   updatedAt,
			Status:      string(r.Info.Status),
			Chart:       chart,
			AppVersion:  appVersion,
			Description: description,
		})
	}

	response.OK(c, items)
}

// GetReleaseValues 取得 Helm Release 的 values
// GET /clusters/:id/helm/releases/:namespace/:name/values?all=true
func (h *HelmHandler) GetReleaseValues(c *gin.Context) {
	clusterIDStr := c.Param("id")
	namespace := c.Param("namespace")
	name := c.Param("name")
	allValues := c.Query("all") == "true"

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	vals, err := h.helmService.GetValues(cluster, namespace, name, allValues)
	if err != nil {
		logger.Error("取得 Helm Release Values 失敗", "cluster", cluster.Name, "name", name, "error", err)
		response.InternalError(c, "取得 values 失敗: "+err.Error())
		return
	}

	response.OK(c, vals)
}

// InstallRelease 安裝 Helm Release
// POST /clusters/:id/helm/releases
func (h *HelmHandler) InstallRelease(c *gin.Context) {
	clusterIDStr := c.Param("id")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	var req services.InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數無效: "+err.Error())
		return
	}

	rel, err := h.helmService.InstallRelease(cluster, req)
	if err != nil {
		logger.Error("安裝 Helm Release 失敗", "cluster", cluster.Name, "release", req.ReleaseName, "error", err)
		response.InternalError(c, "安裝失敗: "+err.Error())
		return
	}

	response.Created(c, toReleaseResponse(rel))
}

// UpgradeRelease 升級 Helm Release
// PUT /clusters/:id/helm/releases/:namespace/:name
func (h *HelmHandler) UpgradeRelease(c *gin.Context) {
	clusterIDStr := c.Param("id")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	var req services.UpgradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數無效: "+err.Error())
		return
	}

	rel, err := h.helmService.UpgradeRelease(cluster, namespace, name, req)
	if err != nil {
		logger.Error("升級 Helm Release 失敗", "cluster", cluster.Name, "release", name, "error", err)
		response.InternalError(c, "升級失敗: "+err.Error())
		return
	}

	response.OK(c, toReleaseResponse(rel))
}

// RollbackRelease 回滾 Helm Release
// POST /clusters/:id/helm/releases/:namespace/:name/rollback
func (h *HelmHandler) RollbackRelease(c *gin.Context) {
	clusterIDStr := c.Param("id")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	var body struct {
		Revision int `json:"revision"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "請求參數無效: "+err.Error())
		return
	}

	if err := h.helmService.RollbackRelease(cluster, namespace, name, body.Revision); err != nil {
		logger.Error("回滾 Helm Release 失敗", "cluster", cluster.Name, "release", name, "error", err)
		response.InternalError(c, "回滾失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "回滾成功"})
}

// UninstallRelease 解除安裝 Helm Release
// DELETE /clusters/:id/helm/releases/:namespace/:name
func (h *HelmHandler) UninstallRelease(c *gin.Context) {
	clusterIDStr := c.Param("id")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	if err := h.helmService.UninstallRelease(cluster, namespace, name); err != nil {
		logger.Error("解除安裝 Helm Release 失敗", "cluster", cluster.Name, "release", name, "error", err)
		response.InternalError(c, "解除安裝失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "解除安裝成功"})
}

// ListRepos 列出所有 Helm Repository
// GET /helm/repos
func (h *HelmHandler) ListRepos(c *gin.Context) {
	repos, err := h.helmService.ListRepos()
	if err != nil {
		logger.Error("列出 Helm Repos 失敗", "error", err)
		response.InternalError(c, "列出 Repos 失敗: "+err.Error())
		return
	}

	response.OK(c, repos)
}

// AddRepo 新增 Helm Repository
// POST /helm/repos
func (h *HelmHandler) AddRepo(c *gin.Context) {
	var body struct {
		Name     string `json:"name" binding:"required"`
		URL      string `json:"url" binding:"required"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "請求參數無效: "+err.Error())
		return
	}

	helmRepo, err := h.helmService.AddRepo(body.Name, body.URL, body.Username, body.Password)
	if err != nil {
		logger.Error("新增 Helm Repo 失敗", "name", body.Name, "error", err)
		response.InternalError(c, "新增 Repo 失敗: "+err.Error())
		return
	}

	response.Created(c, helmRepo)
}

// RemoveRepo 刪除 Helm Repository
// DELETE /helm/repos/:name
func (h *HelmHandler) RemoveRepo(c *gin.Context) {
	name := c.Param("name")

	if err := h.helmService.RemoveRepo(name); err != nil {
		logger.Error("刪除 Helm Repo 失敗", "name", name, "error", err)
		response.NotFound(c, "刪除 Repo 失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "刪除成功"})
}

// SearchCharts 搜尋 Chart
// GET /helm/repos/charts?keyword=
func (h *HelmHandler) SearchCharts(c *gin.Context) {
	keyword := c.Query("keyword")

	charts, err := h.helmService.SearchCharts(keyword)
	if err != nil {
		logger.Error("搜尋 Helm Charts 失敗", "keyword", keyword, "error", err)
		response.InternalError(c, "搜尋 Charts 失敗: "+err.Error())
		return
	}

	response.OK(c, charts)
}

