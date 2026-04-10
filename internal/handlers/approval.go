package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ApprovalHandler 審批工作流處理器
type ApprovalHandler struct {
	approvalSvc *services.ApprovalService
	clusterSvc  *services.ClusterService
}

func NewApprovalHandler(approvalSvc *services.ApprovalService, clusterSvc *services.ClusterService) *ApprovalHandler {
	return &ApprovalHandler{approvalSvc: approvalSvc, clusterSvc: clusterSvc}
}

// ApprovalCreateRequest 建立審批請求的 payload
type ApprovalCreateRequest struct {
	Namespace    string `json:"namespace"    binding:"required"`
	ResourceKind string `json:"resourceKind" binding:"required"`
	ResourceName string `json:"resourceName" binding:"required"`
	Action       string `json:"action"       binding:"required"`
	Payload      string `json:"payload"`
	ExpiresInH   int    `json:"expiresInHours"`
}

// CreateApprovalRequest POST /api/v1/clusters/:clusterID/approvals
func (h *ApprovalHandler) CreateApprovalRequest(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
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

	expireHours := req.ExpiresInH
	if expireHours <= 0 {
		expireHours = 24
	}

	requesterID := c.GetUint("user_id")
	requesterName, _ := c.Get("username")

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.approvalSvc.CreateApprovalRequest(ctx, ar); err != nil {
		response.InternalError(c, "建立審批請求失敗: "+err.Error())
		return
	}

	logger.Info("建立審批請求", "cluster", cluster.Name, "ns", req.Namespace, "resource", req.ResourceName, "action", req.Action)
	response.OK(c, ar)
}

// ListApprovalRequests GET /api/v1/approvals?status=pending&clusterID=1
func (h *ApprovalHandler) ListApprovalRequests(c *gin.Context) {
	status := c.Query("status")
	var clusterIDFilter uint
	if cid, err := strconv.ParseUint(c.Query("clusterID"), 10, 64); err == nil {
		clusterIDFilter = uint(cid) //nolint:gosec // uint64 → uint: value bounded by DB ID range
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	h.approvalSvc.ExpireStaleRequests(ctx)

	items, err := h.approvalSvc.ListApprovalRequests(ctx, status, clusterIDFilter)
	if err != nil {
		response.InternalError(c, "查詢審批請求失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// ApproveRequest PUT /api/v1/approvals/:id/approve
func (h *ApprovalHandler) ApproveRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的 ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	ar, err := h.approvalSvc.GetApprovalRequest(ctx, uint(id)) //nolint:gosec
	if err != nil {
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
	if err := h.approvalSvc.UpdateApprovalRequest(ctx, ar, updates); err != nil {
		response.InternalError(c, "更新審批狀態失敗: "+err.Error())
		return
	}

	logger.Info("審批透過", "id", id, "approver", approverName)
	response.OK(c, gin.H{"message": "已核准", "id": id})
}

// RejectRequest PUT /api/v1/approvals/:id/reject
func (h *ApprovalHandler) RejectRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的 ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	ar, err := h.approvalSvc.GetApprovalRequest(ctx, uint(id)) //nolint:gosec
	if err != nil {
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
	if err := h.approvalSvc.UpdateApprovalRequest(ctx, ar, updates); err != nil {
		response.InternalError(c, "更新審批狀態失敗: "+err.Error())
		return
	}

	logger.Info("審批拒絕", "id", id, "approver", approverName)
	response.OK(c, gin.H{"message": "已拒絕", "id": id})
}

// GetPendingCount GET /api/v1/approvals/pending-count
func (h *ApprovalHandler) GetPendingCount(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	h.approvalSvc.ExpireStaleRequests(ctx)
	count := h.approvalSvc.GetPendingCount(ctx)
	response.OK(c, gin.H{"count": count})
}

// GetNamespaceProtections GET /api/v1/clusters/:clusterID/namespace-protections
func (h *ApprovalHandler) GetNamespaceProtections(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	items, err := h.approvalSvc.ListNamespaceProtections(ctx, clusterID)
	if err != nil {
		response.InternalError(c, "查詢失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}

// SetNamespaceProtection PUT /api/v1/clusters/:clusterID/namespace-protections/:namespace
func (h *ApprovalHandler) SetNamespaceProtection(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")

	var body struct {
		RequireApproval bool   `json:"requireApproval"`
		Description     string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	np, err := h.approvalSvc.UpsertNamespaceProtection(ctx, clusterID, namespace, body.RequireApproval, body.Description)
	if err != nil {
		response.InternalError(c, "設定失敗: "+err.Error())
		return
	}
	response.OK(c, np)
}

// GetNamespaceProtectionStatus GET /api/v1/clusters/:clusterID/namespace-protections/:namespace
func (h *ApprovalHandler) GetNamespaceProtectionStatus(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	np, err := h.approvalSvc.GetNamespaceProtection(ctx, clusterID, namespace)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.OK(c, gin.H{"requireApproval": false, "description": ""})
			return
		}
		response.InternalError(c, "查詢失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"requireApproval": np.RequireApproval, "description": np.Description})
}
