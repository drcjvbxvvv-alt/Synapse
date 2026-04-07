package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/metrics"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gorm.io/gorm"
)

// K8sClientProvider 避免 services ↔ k8s 迴圈引用的介面
type K8sClientProvider interface {
	GetK8sClient(cluster *models.Cluster) (*K8sClient, error)
}

// EventAlertService Event 告警規則服務
type EventAlertService struct {
	db *gorm.DB
}

// NewEventAlertService 建立服務
func NewEventAlertService(db *gorm.DB) *EventAlertService {
	return &EventAlertService{db: db}
}

// ListRules 取得規則列表
func (s *EventAlertService) ListRules(clusterID uint, page, pageSize int) ([]models.EventAlertRule, int64, error) {
	var rules []models.EventAlertRule
	var total int64

	q := s.db.Model(&models.EventAlertRule{}).Where("cluster_id = ?", clusterID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&rules).Error
	return rules, total, err
}

// CreateRule 建立規則
func (s *EventAlertService) CreateRule(rule *models.EventAlertRule) error {
	return s.db.Create(rule).Error
}

// UpdateRule 更新規則
func (s *EventAlertService) UpdateRule(rule *models.EventAlertRule) error {
	return s.db.Save(rule).Error
}

// DeleteRule 刪除規則
func (s *EventAlertService) DeleteRule(id uint) error {
	return s.db.Delete(&models.EventAlertRule{}, id).Error
}

// ToggleRule 切換啟用/停用
func (s *EventAlertService) ToggleRule(id uint, enabled bool) error {
	return s.db.Model(&models.EventAlertRule{}).Where("id = ?", id).Update("enabled", enabled).Error
}

// ListHistory 取得告警歷史
func (s *EventAlertService) ListHistory(clusterID uint, page, pageSize int) ([]models.EventAlertHistory, int64, error) {
	var items []models.EventAlertHistory
	var total int64

	q := s.db.Model(&models.EventAlertHistory{}).Where("cluster_id = ?", clusterID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	err := q.Order("triggered_at DESC").Offset(offset).Limit(pageSize).Find(&items).Error
	return items, total, err
}

// getAllActiveRulesForCluster 取得某叢集所有啟用中的規則
func (s *EventAlertService) getAllActiveRulesForCluster(clusterID uint) ([]models.EventAlertRule, error) {
	var rules []models.EventAlertRule
	err := s.db.Where("cluster_id = ? AND enabled = ?", clusterID, true).Find(&rules).Error
	return rules, err
}

// recordHistory 記錄告警觸發歷史
func (s *EventAlertService) recordHistory(history *models.EventAlertHistory) {
	if err := s.db.Create(history).Error; err != nil {
		logger.Error("記錄告警歷史失敗", "error", err)
	}
}

// ---- Background Worker ----

// EventAlertWorker 後臺事件掃描工作器
type EventAlertWorker struct {
	db          *gorm.DB
	k8sProvider K8sClientProvider
	clusterSvc  *ClusterService
	ticker      *time.Ticker
	stopCh      chan struct{}
	metrics     *metrics.WorkerMetrics
	// 已觸發紀錄的去重 key: clusterID|ruleID|involvedObj|reason → last trigger time
	seen map[string]time.Time
}

// SetMetrics attaches Prometheus worker metrics.
func (w *EventAlertWorker) SetMetrics(m *metrics.WorkerMetrics) { w.metrics = m }

// NewEventAlertWorker 建立工作器
func NewEventAlertWorker(db *gorm.DB, k8sProvider K8sClientProvider, clusterSvc *ClusterService) *EventAlertWorker {
	return &EventAlertWorker{
		db:          db,
		k8sProvider: k8sProvider,
		clusterSvc:  clusterSvc,
		stopCh:      make(chan struct{}),
		seen:        make(map[string]time.Time),
	}
}

// Start 啟動後臺工作器（每 60 秒掃描一次）
func (w *EventAlertWorker) Start() {
	w.ticker = time.NewTicker(60 * time.Second)
	go func() {
		// 啟動後立即掃描一次
		w.scan()
		for {
			select {
			case <-w.ticker.C:
				w.scan()
			case <-w.stopCh:
				w.ticker.Stop()
				return
			}
		}
	}()
	logger.Info("Event 告警工作器已啟動")
}

// Stop 停止工作器
func (w *EventAlertWorker) Stop() {
	close(w.stopCh)
}

// scan 掃描所有叢集的 K8s Events 並與規則比對
func (w *EventAlertWorker) scan() {
	var run *metrics.WorkerRun
	if w.metrics != nil {
		run = w.metrics.Start("event_alert")
	}

	svc := NewEventAlertService(w.db)
	clusters, err := w.clusterSvc.GetAllClusters()
	if err != nil {
		logger.Error("Event 告警：取得叢集列表失敗", "error", err)
		if run != nil {
			run.Done(err)
		}
		return
	}

	for _, cluster := range clusters {
		rules, err := svc.getAllActiveRulesForCluster(cluster.ID)
		if err != nil || len(rules) == 0 {
			continue
		}

		k8sClient, err := w.k8sProvider.GetK8sClient(cluster)
		if err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		events, err := k8sClient.GetClientset().CoreV1().Events("").List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			continue
		}

		now := time.Now()
		// 僅處理最近 5 分鐘內的事件
		cutoff := now.Add(-5 * time.Minute)

		for _, event := range events.Items {
			if event.LastTimestamp.IsZero() {
				continue
			}
			if event.LastTimestamp.Time.Before(cutoff) {
				continue
			}

			for _, rule := range rules {
				if !w.matchRule(&event, &rule) {
					continue
				}

				dedupeKey := fmt.Sprintf("%d|%d|%s/%s|%s",
					cluster.ID, rule.ID,
					event.InvolvedObject.Kind, event.InvolvedObject.Name,
					event.Reason)

				// 同一規則 + 同一物件 + 同一 reason，30 分鐘內只觸發一次
				if t, ok := w.seen[dedupeKey]; ok && now.Sub(t) < 30*time.Minute {
					continue
				}
				w.seen[dedupeKey] = now

				involvedObj := fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name)
				notifyResult := w.notify(&rule, &event, involvedObj)

				svc.recordHistory(&models.EventAlertHistory{
					RuleID:      rule.ID,
					ClusterID:   cluster.ID,
					RuleName:    rule.Name,
					Namespace:   event.Namespace,
					EventReason: event.Reason,
					EventType:   event.Type,
					Message:     event.Message,
					InvolvedObj: involvedObj,
					NotifyResult: notifyResult,
					TriggeredAt: now,
				})
			}
		}
	}
	if run != nil {
		run.Done(nil)
	}
}

// matchRule 判斷事件是否符合規則
func (w *EventAlertWorker) matchRule(event *corev1.Event, rule *models.EventAlertRule) bool {
	// 命名空間過濾
	if rule.Namespace != "" && rule.Namespace != "_all_" && rule.Namespace != event.Namespace {
		return false
	}
	// EventType 過濾
	if rule.EventType != "" && !strings.EqualFold(rule.EventType, event.Type) {
		return false
	}
	// EventReason 過濾（支援多個 reason 逗號分隔）
	if rule.EventReason != "" {
		reasons := strings.Split(rule.EventReason, ",")
		matched := false
		for _, r := range reasons {
			if strings.EqualFold(strings.TrimSpace(r), event.Reason) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	// 次數門檻
	if int(event.Count) < rule.MinCount {
		return false
	}
	return true
}

// notify 傳送通知並回傳結果
func (w *EventAlertWorker) notify(rule *models.EventAlertRule, event *corev1.Event, involvedObj string) string {
	if rule.NotifyURL == "" {
		return "no_url"
	}

	payload := map[string]interface{}{
		"rule":        rule.Name,
		"clusterID":   rule.ClusterID,
		"namespace":   event.Namespace,
		"involvedObj": involvedObj,
		"reason":      event.Reason,
		"type":        event.Type,
		"count":       event.Count,
		"message":     event.Message,
		"triggeredAt": time.Now().Format(time.RFC3339),
	}

	// Telegram 格式
	if rule.NotifyType == "telegram" {
		payload = map[string]interface{}{
			"text": fmt.Sprintf("*[Synapse 告警]* %s\n叢集: %d | 命名空間: `%s` | 物件: `%s`\n原因: `%s` | 次數: %d\n訊息: %s",
				rule.Name, rule.ClusterID, event.Namespace, involvedObj,
				event.Reason, event.Count, event.Message),
			"parse_mode": "Markdown",
		}
	}

	// Slack 格式
	if rule.NotifyType == "slack" {
		payload = map[string]interface{}{
			"text": fmt.Sprintf(":warning: *[Synapse 告警]* %s\n叢集: %d | 命名空間: `%s` | 物件: `%s`\n原因: `%s` | 次數: %d\n訊息: %s",
				rule.Name, rule.ClusterID, event.Namespace, involvedObj,
				event.Reason, event.Count, event.Message),
		}
	}

	// Microsoft Teams 格式（Incoming Webhook）
	if rule.NotifyType == "teams" {
		payload = map[string]interface{}{
			"type": "message",
			"attachments": []map[string]interface{}{{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"type":    "AdaptiveCard",
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"version": "1.2",
					"body": []map[string]interface{}{
						{"type": "TextBlock", "text": fmt.Sprintf("[Synapse 告警] %s", rule.Name), "weight": "bolder", "size": "medium"},
						{"type": "TextBlock", "text": fmt.Sprintf("叢集: %d | 命名空間: %s | 物件: %s", rule.ClusterID, event.Namespace, involvedObj), "wrap": true},
						{"type": "TextBlock", "text": fmt.Sprintf("原因: %s | 次數: %d", event.Reason, event.Count), "wrap": true},
						{"type": "TextBlock", "text": fmt.Sprintf("訊息: %s", event.Message), "wrap": true, "color": "attention"},
					},
				},
			}},
		}
	}

	data, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", rule.NotifyURL, bytes.NewReader(data))
	if err != nil {
		return "failed"
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Event 告警通知失敗", "rule", rule.Name, "error", err)
		return "failed"
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "sent"
	}
	return fmt.Sprintf("http_%d", resp.StatusCode)
}
