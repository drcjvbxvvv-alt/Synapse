package handlers

import (
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NotificationHandler 消息通知處理器
type NotificationHandler struct {
	db *gorm.DB
}

func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
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
// 回傳最近 100 筆通知（跨所有叢集），含未讀數量
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	pageSize := 50

	var histories []models.EventAlertHistory
	if err := h.db.Order("triggered_at desc").Limit(pageSize).Find(&histories).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 取得叢集名稱對照表
	clusterIDs := make(map[uint]struct{})
	for _, h := range histories {
		clusterIDs[h.ClusterID] = struct{}{}
	}
	ids := make([]uint, 0, len(clusterIDs))
	for id := range clusterIDs {
		ids = append(ids, id)
	}
	var clusters []models.Cluster
	if len(ids) > 0 {
		h.db.Where("id IN ?", ids).Find(&clusters)
	}
	clusterNames := make(map[uint]string, len(clusters))
	for _, cl := range clusters {
		clusterNames[cl.ID] = cl.Name
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
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的通知 ID")
		return
	}
	if err := h.db.Model(&models.EventAlertHistory{}).
		Where("id = ?", id).
		Update("is_read", true).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// MarkAllRead PUT /notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	if err := h.db.Model(&models.EventAlertHistory{}).
		Where("is_read = ?", false).
		Update("is_read", true).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// UnreadCount GET /notifications/unread-count
// 輕量端點：僅回傳未讀數，供輪詢使用
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	var count int64
	h.db.Model(&models.EventAlertHistory{}).Where("is_read = ?", false).Count(&count)
	response.OK(c, gin.H{"unreadCount": count})
}
