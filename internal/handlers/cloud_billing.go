package handlers

import (
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
	"github.com/gin-gonic/gin"
)

// CloudBillingHandler 雲端帳單整合處理器
type CloudBillingHandler struct {
	svc        *services.CloudBillingService
	clusterSvc *services.ClusterService
}

func NewCloudBillingHandler(svc *services.CloudBillingService, clusterSvc *services.ClusterService) *CloudBillingHandler {
	return &CloudBillingHandler{svc: svc, clusterSvc: clusterSvc}
}

// GetBillingConfig 取得雲端帳單設定
// GET /api/v1/clusters/:clusterID/billing/config
func (h *CloudBillingHandler) GetBillingConfig(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cfg, err := h.svc.GetConfig(clusterID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	// 遮蔽敏感欄位：只顯示是否已設定，不回傳明文
	type safeConfig struct {
		ID                    uint   `json:"id"`
		ClusterID             uint   `json:"cluster_id"`
		Provider              string `json:"provider"`
		AWSAccessKeyID        string `json:"aws_access_key_id"`
		AWSSecretSet          bool   `json:"aws_secret_set"`
		AWSRegion             string `json:"aws_region"`
		AWSLinkedAccountID    string `json:"aws_linked_account_id"`
		GCPProjectID          string `json:"gcp_project_id"`
		GCPBillingAccountID   string `json:"gcp_billing_account_id"`
		GCPServiceAccountSet  bool   `json:"gcp_service_account_set"`
		LastSyncedAt          interface{} `json:"last_synced_at"`
		LastError             string `json:"last_error,omitempty"`
	}
	response.OK(c, safeConfig{
		ID:                   cfg.ID,
		ClusterID:            cfg.ClusterID,
		Provider:             cfg.Provider,
		AWSAccessKeyID:       cfg.AWSAccessKeyID,
		AWSSecretSet:         cfg.AWSSecretAccessKey != "",
		AWSRegion:            cfg.AWSRegion,
		AWSLinkedAccountID:   cfg.AWSLinkedAccountID,
		GCPProjectID:         cfg.GCPProjectID,
		GCPBillingAccountID:  cfg.GCPBillingAccountID,
		GCPServiceAccountSet: cfg.GCPServiceAccountJSON != "",
		LastSyncedAt:         cfg.LastSyncedAt,
		LastError:            cfg.LastError,
	})
}

// UpdateBillingConfig 更新雲端帳單設定
// PUT /api/v1/clusters/:clusterID/billing/config
func (h *CloudBillingHandler) UpdateBillingConfig(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	var req services.UpdateBillingConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求格式錯誤: "+err.Error())
		return
	}
	cfg, err := h.svc.UpdateConfig(clusterID, &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	logger.Info("雲端帳單設定已更新", "cluster_id", clusterID, "provider", cfg.Provider)
	response.OK(c, gin.H{"message": "設定已儲存", "provider": cfg.Provider})
}

// SyncBilling 觸發帳單同步
// POST /api/v1/clusters/:clusterID/billing/sync
func (h *CloudBillingHandler) SyncBilling(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	month := c.Query("month") // optional, default = current month

	if err := h.svc.SyncBilling(clusterID, month); err != nil {
		logger.Warn("帳單同步失敗", "cluster_id", clusterID, "error", err)
		response.InternalError(c, "帳單同步失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "帳單同步完成"})
}

// GetBillingOverview 取得帳單彙總 + 資源單位成本
// GET /api/v1/clusters/:clusterID/billing/overview?month=2026-04
func (h *CloudBillingHandler) GetBillingOverview(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	month := c.Query("month")
	overview, err := h.svc.GetBillingOverview(clusterID, month)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, overview)
}
