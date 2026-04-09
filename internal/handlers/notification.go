package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// NotificationHandler 消息通知處理器
type NotificationHandler struct {
	svc *services.NotificationService
}

func NewNotificationHandler(svc *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// NotificationItem 通知項目（含叢集名稱）
type NotificationItem struct {
	ID           uint      `json:"id"`
	RuleName     string    `json:"ruleName"`
	ClusterID    uint      `json:"clusterId"`
	ClusterName  string    `json:"clusterName"`
	Namespace    string    `json:"namespace"`
	EventReason  string    `json:"eventReason"`
	EventType    string    `json:"eventType"`
	Message      string    `json:"message"`
	InvolvedObj  string    `json:"involvedObj"`
	NotifyResult string    `json:"notifyResult"`
	IsRead       bool      `json:"isRead"`
	TriggeredAt  time.Time `json:"triggeredAt"`
}

// ListNotifications GET /notifications
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	ctx := c.Request.Context()

	histories, err := h.svc.ListRecent(ctx, 50)
	if err != nil {
		logger.Error("list notifications failed", "error", err)
		response.InternalError(c, err.Error())
		return
	}

	// build cluster-name lookup
	clusterIDSet := make(map[uint]struct{}, len(histories))
	for _, h := range histories {
		clusterIDSet[h.ClusterID] = struct{}{}
	}
	ids := make([]uint, 0, len(clusterIDSet))
	for id := range clusterIDSet {
		ids = append(ids, id)
	}
	clusterNames, err := h.svc.ClusterNames(ctx, ids)
	if err != nil {
		logger.Error("fetch cluster names failed", "error", err)
		clusterNames = map[uint]string{}
	}

	items := make([]NotificationItem, 0, len(histories))
	unread := 0
	for _, hist := range histories {
		if !hist.IsRead {
			unread++
		}
		items = append(items, NotificationItem{
			ID:           hist.ID,
			RuleName:     hist.RuleName,
			ClusterID:    hist.ClusterID,
			ClusterName:  clusterNames[hist.ClusterID],
			Namespace:    hist.Namespace,
			EventReason:  hist.EventReason,
			EventType:    hist.EventType,
			Message:      hist.Message,
			InvolvedObj:  hist.InvolvedObj,
			NotifyResult: hist.NotifyResult,
			IsRead:       hist.IsRead,
			TriggeredAt:  hist.TriggeredAt,
		})
	}

	response.OK(c, gin.H{
		"items":       items,
		"total":       len(items),
		"unreadCount": unread,
	})
}

// MarkRead PUT /notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的通知 ID")
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// MarkAllRead PUT /notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	if err := h.svc.MarkAllRead(c.Request.Context()); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// UnreadCount GET /notifications/unread-count
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	count, err := h.svc.CountUnread(c.Request.Context())
	if err != nil {
		logger.Error("count unread notifications failed", "error", err)
		count = 0
	}
	response.OK(c, gin.H{"unreadCount": count})
}
