package handlers

import (
	"strconv"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ApprovalHandler 審批工作流處理器
type ApprovalHandler struct {
	db         *gorm.DB
	clusterSvc *services.ClusterService
}

func NewApprovalHandler(db *gorm.DB, clusterSvc *services.ClusterService) *ApprovalHandler {
	return &ApprovalHandler{db: db, clusterSvc: clusterSvc}
}

// ApprovalCreateRequest 建立審批請求的 payload
type ApprovalCreateRequest struct {
	Namespace    string `json:"namespace" binding:"required"`
	ResourceKind string `json:"resourceKind" binding:"required"`
	ResourceName string `json:"resourceName" binding:"required"`
	Action       string `json:"action" binding:"required"` // scale / delete / update / apply
	Payload      string `json:"payload"` // 原始操作 JSON
	ExpiresInH   int    `json:"expiresInHours"` // 逾時時數，預設 24
}

// CreateApprovalRequest 建立審批請求
// POST /api/v1/clusters/:clusterID/approvals
func (h *ApprovalHandler) CreateApprovalRequest(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	var req ApprovalCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	requesterID := c.GetUint("user_id")
	requesterName, _ := c.Get("username")

	expireHours := req.ExpiresInH
	if expireHours <= 0 {
		expireHours = 24
	}

	ar := &models.ApprovalRequest{
		ClusterID:     clusterID,
		ClusterName:   cluster.Name,
		Namespace:     req.Namespace,
		ResourceKind:  req.ResourceKind,
		ResourceName:  req.ResourceName,
		Action:        req.Action,
		RequesterID:   requesterID,
		RequesterName: requesterName.(string),
		Status:        "pending",
		Payload:       req.Payload,
		ExpiresAt:     time.Now().Add(time.Duration(expireHours) * time.Hour),
	}

	if err := h.db.Create(ar).Error; err != nil {
		response.InternalError(c, "建立審批請求失敗: "+err.Error())
		return
	}

	logger.Info("建立審批請求", "cluster", cluster.Name, "ns", req.Namespace, "resource", req.ResourceName, "action", req.Action)
	response.OK(c, ar)
}

// ListApprovalRequests 列出審批請求
// GET /api/v1/approvals?status=pending&clusterID=1
func (h *ApprovalHandler) ListApprovalRequests(c *gin.Context) {
	status := c.Query("status")
	clusterIDStr := c.Query("clusterID")

	db := h.db.Model(&models.ApprovalRequest{})
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if clusterIDStr != "" {
		if id, err := strconv.ParseUint(clusterIDStr, 10, 64); err == nil {
			db = db.Where("cluster_id = ?", uint(id))
		}
	}

	// 自動過期：pending 且 expires_at < now → expired
	h.db.Model(&models.ApprovalRequest{}).
		Where("status = ? AND expires_at < ?", "pending", time.Now()).
		Update("status", "expired")

	var items []models.ApprovalRequest
	if err := db.Order("created_at desc").Find(&items).Error; err != nil {
		response.InternalError(c, "查詢審批請求失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// ApproveRequest 核准審批
// PUT /api/v1/approvals/:id/approve
func (h *ApprovalHandler) ApproveRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的 ID")
		return
	}

	var ar models.ApprovalRequest
	if err := h.db.First(&ar, id).Error; err != nil {
		response.NotFound(c, "審批請求不存在")
		return
	}
	if ar.Status != "pending" {
		response.BadRequest(c, "該請求已處理（狀態: "+ar.Status+"）")
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)

	approverID := c.GetUint("user_id")
	approverName, _ := c.Get("username")
	now := time.Now()

	updates := map[string]interface{}{
		"status":        "approved",
		"approver_id":   approverID,
		"approver_name": approverName.(string),
		"reason":        body.Reason,
		"approved_at":   &now,
	}
	if err := h.db.Model(&ar).Updates(updates).Error; err != nil {
		response.InternalError(c, "更新審批狀態失敗: "+err.Error())
		return
	}

	logger.Info("審批透過", "id", id, "approver", approverName)
	response.OK(c, gin.H{"message": "已核准", "id": id})
}

// RejectRequest 拒絕審批
// PUT /api/v1/approvals/:id/reject
func (h *ApprovalHandler) RejectRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的 ID")
		return
	}

	var ar models.ApprovalRequest
	if err := h.db.First(&ar, id).Error; err != nil {
		response.NotFound(c, "審批請求不存在")
		return
	}
	if ar.Status != "pending" {
		response.BadRequest(c, "該請求已處理（狀態: "+ar.Status+"）")
		return
	}

	var body struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "必須填寫拒絕原因")
		return
	}

	approverID := c.GetUint("user_id")
	approverName, _ := c.Get("username")

	updates := map[string]interface{}{
		"status":        "rejected",
		"approver_id":   approverID,
		"approver_name": approverName.(string),
		"reason":        body.Reason,
	}
	if err := h.db.Model(&ar).Updates(updates).Error; err != nil {
		response.InternalError(c, "更新審批狀態失敗: "+err.Error())
		return
	}

	logger.Info("審批拒絕", "id", id, "approver", approverName)
	response.OK(c, gin.H{"message": "已拒絕", "id": id})
}

// GetPendingCount 取得待審批數量（導航列 badge 用）
// GET /api/v1/approvals/pending-count
func (h *ApprovalHandler) GetPendingCount(c *gin.Context) {
	// 先過期
	h.db.Model(&models.ApprovalRequest{}).
		Where("status = ? AND expires_at < ?", "pending", time.Now()).
		Update("status", "expired")

	var count int64
	h.db.Model(&models.ApprovalRequest{}).Where("status = ?", "pending").Count(&count)
	response.OK(c, gin.H{"count": count})
}

// --- Namespace Protection ---

// GetNamespaceProtections 取得叢集的命名空間保護設定
// GET /api/v1/clusters/:clusterID/namespace-protections
func (h *ApprovalHandler) GetNamespaceProtections(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var items []models.NamespaceProtection
	if err := h.db.Where("cluster_id = ?", clusterID).Find(&items).Error; err != nil {
		response.InternalError(c, "查詢失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}

// SetNamespaceProtection 設定命名空間保護（upsert）
// PUT /api/v1/clusters/:clusterID/namespace-protections/:namespace
func (h *ApprovalHandler) SetNamespaceProtection(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespace := c.Param("namespace")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var body struct {
		RequireApproval bool   `json:"requireApproval"`
		Description     string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	var np models.NamespaceProtection
	result := h.db.Where("cluster_id = ? AND namespace = ?", clusterID, namespace).First(&np)
	if result.Error != nil {
		// 新建
		np = models.NamespaceProtection{
			ClusterID:       clusterID,
			Namespace:       namespace,
			RequireApproval: body.RequireApproval,
			Description:     body.Description,
		}
		if err := h.db.Create(&np).Error; err != nil {
			response.InternalError(c, "建立保護設定失敗: "+err.Error())
			return
		}
	} else {
		// 更新
		if err := h.db.Model(&np).Updates(map[string]interface{}{
			"require_approval": body.RequireApproval,
			"description":      body.Description,
		}).Error; err != nil {
			response.InternalError(c, "更新保護設定失敗: "+err.Error())
			return
		}
	}

	response.OK(c, np)
}

// GetNamespaceProtectionStatus 查詢單一命名空間保護狀態
// GET /api/v1/clusters/:clusterID/namespace-protections/:namespace
func (h *ApprovalHandler) GetNamespaceProtectionStatus(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespace := c.Param("namespace")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var np models.NamespaceProtection
	if err := h.db.Where("cluster_id = ? AND namespace = ?", clusterID, namespace).First(&np).Error; err != nil {
		// 無設定視為未保護
		response.OK(c, gin.H{"requireApproval": false, "description": ""})
		return
	}
	response.OK(c, gin.H{"requireApproval": np.RequireApproval, "description": np.Description})
}
