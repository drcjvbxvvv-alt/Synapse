package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shaia/Synapse/internal/metrics"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// certExpiryThresholds 依序為 30 天、7 天、1 天，同一 threshold 每天最多通知一次
var certExpiryThresholds = []int{30, 7, 1}

// CertExpiryWorker 每日掃描叢集憑證到期日，並在 30/7/1 天前發送通知。
type CertExpiryWorker struct {
	db      *gorm.DB
	metrics *metrics.WorkerMetrics
	// notified 記錄已通知的 key（"clusterID-days"），避免同一天重複送出
	notified map[string]time.Time
}

// NewCertExpiryWorker 建立 CertExpiryWorker。
func NewCertExpiryWorker(db *gorm.DB) *CertExpiryWorker {
	return &CertExpiryWorker{
		db:       db,
		notified: make(map[string]time.Time),
	}
}

// SetMetrics 注入 Prometheus worker 指標（可在 Start 前呼叫，nil 安全）。
func (w *CertExpiryWorker) SetMetrics(m *metrics.WorkerMetrics) {
	w.metrics = m
}

// Start 啟動背景 goroutine，每天 09:00 本地時間執行一次掃描。
func (w *CertExpiryWorker) Start() {
	go func() {
		// 對齊到下一個 09:00
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// 啟動後立即掃描一次（補上今天）
		w.scan()

		for range ticker.C {
			w.scan()
		}
	}()
}

// scan 掃描所有 CertExpireAt != nil 的叢集，觸發門檻則通知。
func (w *CertExpiryWorker) scan() {
	var run *metrics.WorkerRun
	if w.metrics != nil {
		run = w.metrics.Start("cert_expiry")
	}

	err := w.doScan()

	if run != nil {
		run.Done(err)
	}
}

func (w *CertExpiryWorker) doScan() error {
	var clusters []models.Cluster
	if err := w.db.Where("cert_expire_at IS NOT NULL").Find(&clusters).Error; err != nil {
		return fmt.Errorf("查詢叢集憑證到期資料失敗: %w", err)
	}

	now := time.Now()
	todayKey := now.Format("2006-01-02")

	for i := range clusters {
		c := &clusters[i]
		if c.CertExpireAt == nil {
			continue
		}

		daysLeft := int(time.Until(*c.CertExpireAt).Hours() / 24)

		for _, threshold := range certExpiryThresholds {
			if daysLeft > threshold {
				continue
			}
			key := fmt.Sprintf("%d-%d-%s", c.ID, threshold, todayKey)
			if last, ok := w.notified[key]; ok && time.Since(last) < 20*time.Hour {
				continue // 今天已通知過此門檻
			}
			w.sendNotifications(c, daysLeft)
			w.notified[key] = now
			break // 每個叢集每天只通知最緊迫的門檻一次
		}
	}

	// 清理超過 48 小時的舊 key，防止 map 無限增長
	for k, t := range w.notified {
		if time.Since(t) > 48*time.Hour {
			delete(w.notified, k)
		}
	}

	logger.Info("憑證到期掃描完成", "clusters", len(clusters))
	return nil
}

// sendNotifications 向所有已啟用的通知渠道發送憑證即將到期告警。
func (w *CertExpiryWorker) sendNotifications(cluster *models.Cluster, daysLeft int) {
	var channels []models.NotifyChannel
	if err := w.db.Where("enabled = ?", true).Find(&channels).Error; err != nil {
		logger.Error("查詢通知渠道失敗", "error", err)
		return
	}
	if len(channels) == 0 {
		logger.Warn("憑證即將到期但無已啟用的通知渠道",
			"cluster", cluster.Name, "daysLeft", daysLeft)
		return
	}

	urgency := "⚠️"
	if daysLeft <= 1 {
		urgency = "🚨"
	} else if daysLeft <= 7 {
		urgency = "🔴"
	}

	title := fmt.Sprintf("%s Synapse 告警：叢集憑證即將到期", urgency)
	body := fmt.Sprintf("叢集「%s」的 API Server TLS 憑證將於 %s 到期（剩餘 %d 天）\n請儘速更新憑證以避免連線中斷。",
		cluster.Name,
		cluster.CertExpireAt.Format("2006-01-02"),
		daysLeft,
	)

	for i := range channels {
		ch := &channels[i]
		result := sendCertAlert(ch, title, body, cluster, daysLeft)
		logger.Info("憑證到期通知", "channel", ch.Name, "cluster", cluster.Name,
			"daysLeft", daysLeft, "result", result)
	}
}

// sendCertAlert 依渠道型別格式化並發送通知，返回 "sent" / "failed" / "http_XXX"。
func sendCertAlert(ch *models.NotifyChannel, title, body string, cluster *models.Cluster, daysLeft int) string {
	if ch.WebhookURL == "" {
		return "no_url"
	}

	var payload interface{}

	switch ch.Type {
	case "telegram":
		payload = map[string]interface{}{
			"chat_id":    ch.TelegramChatID,
			"text":       fmt.Sprintf("*%s*\n%s", title, body),
			"parse_mode": "Markdown",
		}
	case "slack":
		payload = map[string]interface{}{
			"text": fmt.Sprintf("*%s*\n%s", title, body),
		}
	case "teams":
		payload = map[string]interface{}{
			"type": "message",
			"attachments": []map[string]interface{}{{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"type":    "AdaptiveCard",
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"version": "1.2",
					"body": []map[string]interface{}{
						{"type": "TextBlock", "text": title, "weight": "bolder", "size": "medium"},
						{"type": "TextBlock", "text": body, "wrap": true, "color": "attention"},
					},
				},
			}},
		}
	default: // webhook
		payload = map[string]interface{}{
			"title":     title,
			"message":   body,
			"cluster":   cluster.Name,
			"daysLeft":  daysLeft,
			"expireAt":  cluster.CertExpireAt.Format("2006-01-02"),
			"alertType": "cert_expiry",
		}
	}

	data, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", ch.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return "failed"
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		logger.Error("憑證到期通知發送失敗", "channel", ch.Name, "error", err)
		return "failed"
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "sent"
	}
	return fmt.Sprintf("http_%d", resp.StatusCode)
}
