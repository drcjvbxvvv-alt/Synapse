package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ImportCluster 匯入叢集
//
// @Summary     匯入叢集（平台管理員）
// @Tags        clusters
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} models.Cluster
// @Failure     400 {object} response.ErrorBody
// @Failure     403 {object} response.ErrorBody
// @Router      /clusters/import [post]
func (h *ClusterHandler) ImportCluster(c *gin.Context) {
	logger.Info("匯入叢集")

	// 獲取請求參數
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		ApiServer   string `json:"apiServer"`
		Kubeconfig  string `json:"kubeconfig"`
		Token       string `json:"token"`
		CaCert      string `json:"caCert"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	logger.Info("叢集匯入請求", "name", req.Name, "apiServer", maskURL(req.ApiServer))

	// 驗證參數
	if req.Kubeconfig == "" && (req.ApiServer == "" || req.Token == "") {
		response.BadRequest(c, "請提供kubeconfig或者API Server地址和訪問令牌")
		return
	}

	var k8sClient *services.K8sClient
	var err error

	// 根據提供的參數建立Kubernetes客戶端
	if req.Kubeconfig != "" {
		k8sClient, err = services.NewK8sClientFromKubeconfig(req.Kubeconfig)
		if err != nil {
			logger.Error("從kubeconfig建立客戶端失敗", "error", err)
			response.BadRequest(c, fmt.Sprintf("kubeconfig格式錯誤: %v", err))
			return
		}
	} else {
		k8sClient, err = services.NewK8sClientFromToken(req.ApiServer, req.Token, req.CaCert)
		if err != nil {
			logger.Error("從Token建立客戶端失敗", "error", err)
			response.BadRequest(c, fmt.Sprintf("連線配置錯誤: %v", err))
			return
		}
	}

	// 測試連線
	clusterInfo, err := k8sClient.TestConnection()
	if err != nil {
		logger.Error("連線測試失敗", "error", err)
		response.BadRequest(c, fmt.Sprintf("連線測試失敗: %v", err))
		return
	}

	// 獲取 API Server 地址：如果使用 kubeconfig，從配置中解析
	apiServer := req.ApiServer
	if apiServer == "" && req.Kubeconfig != "" {
		// 從 kubeconfig 解析出的配置中獲取 API Server 地址
		restConfig := k8sClient.GetRestConfig()
		if restConfig != nil && restConfig.Host != "" {
			apiServer = restConfig.Host
			logger.Info("從 kubeconfig 中解析出 API Server", "apiServer", maskURL(apiServer))
		}
	}

	// P2-2：RBAC 危險程度評估（非阻塞，失敗不影響匯入）
	rbacCtx, rbacCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer rbacCancel()
	rbacSummary := k8sClient.CheckRBACSummary(rbacCtx)

	// P2-1：取得 API Server 憑證到期日（非阻塞，失敗不影響匯入）
	var certExpireAt *time.Time
	if expiry, err := k8sClient.GetAPIServerCertExpiry(); err == nil {
		certExpireAt = expiry
	} else {
		logger.Warn("無法取得 API Server 憑證到期日", "error", err)
	}

	// 建立叢集模型
	cluster := &models.Cluster{
		Name:               req.Name,
		APIServer:          apiServer,
		KubeconfigEnc:      req.Kubeconfig,
		SATokenEnc:         req.Token,
		CAEnc:              req.CaCert,
		Version:            clusterInfo.Version,
		Status:             clusterInfo.Status,
		Labels:             "{}",
		MonitoringConfig:   "{}",
		AlertManagerConfig: "{}",
		CertExpireAt:       certExpireAt,
		CreatedBy:          1, // 臨時設定為1，後續需要從JWT中獲取使用者ID
	}

	// 儲存到資料庫
	err = h.clusterService.CreateCluster(c.Request.Context(), cluster)
	if err != nil {
		logger.Error("儲存叢集資訊失敗", "error", err)
		response.InternalError(c, "儲存叢集資訊失敗: "+err.Error())
		return
	}

	// 返回新建立的叢集資訊（含 RBAC 警告供前端提示）
	newCluster := gin.H{
		"id":           cluster.ID,
		"name":         cluster.Name,
		"apiServer":    cluster.APIServer,
		"version":      cluster.Version,
		"status":       cluster.Status,
		"createdAt":    cluster.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"rbacWarnings": rbacSummary,
	}
	if certExpireAt != nil {
		newCluster["certExpireAt"] = certExpireAt.Format("2006-01-02T15:04:05Z")
	}

	response.OK(c, newCluster)
}

// TestConnection 測試叢集連線
//
// @Summary     測試叢集 API Server 連線
// @Tags        clusters
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200
// @Failure     400 {object} response.ErrorBody
// @Router      /clusters/test-connection [post]
func (h *ClusterHandler) TestConnection(c *gin.Context) {
	logger.Info("測試叢集連線")

	// 獲取請求參數
	var req struct {
		ApiServer  string `json:"apiServer"`
		Kubeconfig string `json:"kubeconfig"`
		Token      string `json:"token"`
		CaCert     string `json:"caCert"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("參數繫結錯誤: %v", err)
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 驗證參數
	if req.Kubeconfig == "" && (req.ApiServer == "" || req.Token == "") {
		response.BadRequest(c, "請提供kubeconfig或者API Server地址和訪問令牌")
		return
	}

	var k8sClient *services.K8sClient
	var err error

	// 根據提供的參數建立Kubernetes客戶端
	if req.Kubeconfig != "" {
		// 使用kubeconfig建立客戶端
		k8sClient, err = services.NewK8sClientFromKubeconfig(req.Kubeconfig)
		if err != nil {
			logger.Error("從kubeconfig建立客戶端失敗", "error", err)
			response.BadRequest(c, fmt.Sprintf("kubeconfig格式錯誤: %v", err))
			return
		}
	} else {
		// 使用API Server和Token建立客戶端
		k8sClient, err = services.NewK8sClientFromToken(req.ApiServer, req.Token, req.CaCert)
		if err != nil {
			logger.Error("從Token建立客戶端失敗", "error", err)
			response.BadRequest(c, fmt.Sprintf("連線配置錯誤: %v", err))
			return
		}
	}

	// 測試連線並獲取叢集資訊
	clusterInfo, err := k8sClient.TestConnection()
	if err != nil {
		logger.Error("連線測試失敗", "error", err)
		response.BadRequest(c, fmt.Sprintf("連線測試失敗: %v", err))
		return
	}

	testResult := gin.H{
		"version":    clusterInfo.Version,
		"nodeCount":  clusterInfo.NodeCount,
		"readyNodes": clusterInfo.ReadyNodes,
		"status":     clusterInfo.Status,
	}

	response.OK(c, testResult)
}
