package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ArgoCDHandler ArgoCD 處理器
type ArgoCDHandler struct {
	db        *gorm.DB
	argoCDSvc *services.ArgoCDService
}

// NewArgoCDHandler 建立 ArgoCD 處理器
func NewArgoCDHandler(db *gorm.DB, argoCDSvc *services.ArgoCDService) *ArgoCDHandler {
	return &ArgoCDHandler{
		db:        db,
		argoCDSvc: argoCDSvc,
	}
}

// GetConfig 獲取 ArgoCD 配置
// @Summary 獲取 ArgoCD 配置
// @Tags ArgoCD/GitOps
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Success 200 {object} models.ArgoCDConfig
// @Router /api/v1/clusters/{clusterID}/argocd/config [get]
func (h *ArgoCDHandler) GetConfig(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	config, err := h.argoCDSvc.GetConfig(c.Request.Context(), uint(clusterID))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 隱藏敏感資訊
	configResp := *config
	configResp.Token = ""
	configResp.Password = ""
	configResp.GitPassword = ""
	configResp.GitSSHKey = ""

	response.OK(c, configResp)
}

// SaveConfig 儲存 ArgoCD 配置
// @Summary 儲存 ArgoCD 配置
// @Tags ArgoCD/GitOps
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param config body models.ArgoCDConfig true "配置資訊"
// @Success 200 {object} gin.H
// @Router /api/v1/clusters/{clusterID}/argocd/config [put]
func (h *ArgoCDHandler) SaveConfig(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 使用請求結構體接收前端資料（包含敏感欄位）
	var req models.ArgoCDConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	// 轉換為資料庫模型
	config := req.ToModel()
	config.ClusterID = uint(clusterID)

	// 如果沒有傳新的密碼/Token，保留原有的
	existing, _ := h.argoCDSvc.GetConfig(c.Request.Context(), uint(clusterID))
	if existing != nil && existing.ID > 0 {
		if config.Token == "" {
			config.Token = existing.Token
		}
		if config.Password == "" {
			config.Password = existing.Password
		}
		if config.GitPassword == "" {
			config.GitPassword = existing.GitPassword
		}
		if config.GitSSHKey == "" {
			config.GitSSHKey = existing.GitSSHKey
		}
	}

	if err := h.argoCDSvc.SaveConfig(c.Request.Context(), config); err != nil {
		response.InternalError(c, "儲存失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "儲存成功"})
}

// TestConnection 測試 ArgoCD 連線
// @Summary 測試 ArgoCD 連線
// @Tags ArgoCD/GitOps
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param config body models.ArgoCDConfig true "配置資訊"
// @Success 200 {object} gin.H
// @Router /api/v1/clusters/{clusterID}/argocd/test-connection [post]
func (h *ArgoCDHandler) TestConnection(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 使用請求結構體接收前端資料（包含敏感欄位）
	var req models.ArgoCDConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤")
		return
	}

	// 轉換為資料庫模型
	config := req.ToModel()
	logger.Info("測試 ArgoCD 連線", "serverURL", config.ServerURL, "authType", config.AuthType, "hasToken", config.Token != "", "hasPassword", config.Password != "")

	// 如果沒有傳認證資訊，嘗試從資料庫獲取（僅作為回退）
	if config.Token == "" && config.Password == "" {
		existing, _ := h.argoCDSvc.GetConfig(c.Request.Context(), uint(clusterID))
		if existing != nil {
			config.Token = existing.Token
			config.Username = existing.Username
			config.Password = existing.Password
			logger.Info("使用資料庫中的認證資訊")
		}
	}

	if err := h.argoCDSvc.TestConnection(c.Request.Context(), config); err != nil {
		response.OK(c, gin.H{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}

	response.OK(c, gin.H{"connected": true})
}

// ListApplications 獲取應用列表
// @Summary 獲取 ArgoCD 應用列表
// @Tags ArgoCD/GitOps
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Success 200 {array} models.ArgoCDApplication
// @Router /api/v1/clusters/{clusterID}/argocd/applications [get]
func (h *ArgoCDHandler) ListApplications(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	apps, err := h.argoCDSvc.ListApplications(c.Request.Context(), uint(clusterID))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, apps, int64(len(apps)))
}

// GetApplication 獲取應用詳情
// @Summary 獲取 ArgoCD 應用詳情
// @Tags ArgoCD/GitOps
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param appName path string true "應用名稱"
// @Success 200 {object} models.ArgoCDApplication
// @Router /api/v1/clusters/{clusterID}/argocd/applications/{appName} [get]
func (h *ArgoCDHandler) GetApplication(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	appName := c.Param("appName")

	app, err := h.argoCDSvc.GetApplication(c.Request.Context(), uint(clusterID), appName)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, app)
}

// CreateApplication 建立應用
// @Summary 建立 ArgoCD 應用
// @Tags ArgoCD/GitOps
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param request body models.CreateApplicationRequest true "建立請求"
// @Success 200 {object} models.ArgoCDApplication
// @Router /api/v1/clusters/{clusterID}/argocd/applications [post]
func (h *ArgoCDHandler) CreateApplication(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var req models.CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	app, err := h.argoCDSvc.CreateApplication(c.Request.Context(), uint(clusterID), &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, app)
}

// UpdateApplication 更新應用
// @Summary 更新 ArgoCD 應用
// @Tags ArgoCD/GitOps
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param appName path string true "應用名稱"
// @Param request body models.CreateApplicationRequest true "更新請求"
// @Success 200 {object} models.ArgoCDApplication
// @Router /api/v1/clusters/{clusterID}/argocd/applications/{appName} [put]
func (h *ArgoCDHandler) UpdateApplication(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	appName := c.Param("appName")

	var req models.CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	app, err := h.argoCDSvc.UpdateApplication(c.Request.Context(), uint(clusterID), appName, &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, app)
}

// SyncApplication 同步應用
// @Summary 同步 ArgoCD 應用
// @Tags ArgoCD/GitOps
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param appName path string true "應用名稱"
// @Param request body models.SyncApplicationRequest false "同步請求"
// @Success 200 {object} gin.H
// @Router /api/v1/clusters/{clusterID}/argocd/applications/{appName}/sync [post]
func (h *ArgoCDHandler) SyncApplication(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	appName := c.Param("appName")

	var req models.SyncApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	if err := h.argoCDSvc.SyncApplication(c.Request.Context(), uint(clusterID), appName, req.Revision); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"message": "同步已觸發"})
}

// DeleteApplication 刪除應用
// @Summary 刪除 ArgoCD 應用
// @Tags ArgoCD/GitOps
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param appName path string true "應用名稱"
// @Param cascade query bool false "是否級聯刪除資源" default(true)
// @Success 200 {object} gin.H
// @Router /api/v1/clusters/{clusterID}/argocd/applications/{appName} [delete]
func (h *ArgoCDHandler) DeleteApplication(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	appName := c.Param("appName")
	cascade := c.Query("cascade") != "false"

	if err := h.argoCDSvc.DeleteApplication(c.Request.Context(), uint(clusterID), appName, cascade); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"message": "刪除成功"})
}

// RollbackApplication 回滾應用
// @Summary 回滾 ArgoCD 應用
// @Tags ArgoCD/GitOps
// @Accept json
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param appName path string true "應用名稱"
// @Param request body models.RollbackApplicationRequest true "回滾請求"
// @Success 200 {object} gin.H
// @Router /api/v1/clusters/{clusterID}/argocd/applications/{appName}/rollback [post]
func (h *ArgoCDHandler) RollbackApplication(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	appName := c.Param("appName")

	var req models.RollbackApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤")
		return
	}

	if err := h.argoCDSvc.RollbackApplication(c.Request.Context(), uint(clusterID), appName, req.RevisionID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"message": "回滾已觸發"})
}

// GetApplicationResources 獲取應用資源樹
// @Summary 獲取 ArgoCD 應用資源樹
// @Tags ArgoCD/GitOps
// @Produce json
// @Param clusterID path int true "叢集ID"
// @Param appName path string true "應用名稱"
// @Success 200 {array} models.ArgoCDResource
// @Router /api/v1/clusters/{clusterID}/argocd/applications/{appName}/resources [get]
func (h *ArgoCDHandler) GetApplicationResources(c *gin.Context) {
	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	appName := c.Param("appName")

	resources, err := h.argoCDSvc.GetApplicationResources(c.Request.Context(), uint(clusterID), appName)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, resources)
}
