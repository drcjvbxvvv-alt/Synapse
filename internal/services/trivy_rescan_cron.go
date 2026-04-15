package services

import (
	"context"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// TrivyRescanCron — 定期重掃現有映像（CICD_ARCHITECTURE §5, §20 近期路線圖）
//
// 設計：
//   - 每 interval（預設 24h）掃描一次所有已知映像
//   - 查詢最近 completed 的掃描結果中未在 rescanAge 內重掃的映像
//   - 對每個映像呼叫 TrivyService.TriggerScan（內建 dedup 防止重複）
//   - 限制每輪最多重掃 maxPerRound 個映像，避免過度負載
// ---------------------------------------------------------------------------

const (
	// DefaultRescanInterval 預設重掃間隔。
	DefaultRescanInterval = 24 * time.Hour

	// DefaultRescanAge 映像上次掃描超過此時間才需重掃。
	DefaultRescanAge = 24 * time.Hour

	// DefaultMaxPerRound 每輪最多重掃的映像數量。
	DefaultMaxPerRound = 100
)

// TrivyRescanCron 定期重掃背景 goroutine。
type TrivyRescanCron struct {
	db       *gorm.DB
	trivySvc *TrivyService
	interval time.Duration
	age      time.Duration
	maxPer   int
	stopCh   chan struct{}
}

// NewTrivyRescanCron 建立 TrivyRescanCron。
func NewTrivyRescanCron(db *gorm.DB, trivySvc *TrivyService) *TrivyRescanCron {
	return &TrivyRescanCron{
		db:       db,
		trivySvc: trivySvc,
		interval: DefaultRescanInterval,
		age:      DefaultRescanAge,
		maxPer:   DefaultMaxPerRound,
		stopCh:   make(chan struct{}),
	}
}

// SetInterval 設定重掃間隔（測試用）。
func (c *TrivyRescanCron) SetInterval(d time.Duration) {
	c.interval = d
}

// SetRescanAge 設定映像需重掃的年齡閾值（測試用）。
func (c *TrivyRescanCron) SetRescanAge(d time.Duration) {
	c.age = d
}

// SetMaxPerRound 設定每輪最大重掃數（測試用）。
func (c *TrivyRescanCron) SetMaxPerRound(n int) {
	c.maxPer = n
}

// Start 啟動背景 goroutine。
func (c *TrivyRescanCron) Start() {
	go c.loop()
	logger.Info("trivy rescan cron started",
		"interval", c.interval,
		"rescan_age", c.age,
		"max_per_round", c.maxPer,
	)
}

// Stop 停止背景 goroutine。
func (c *TrivyRescanCron) Stop() {
	close(c.stopCh)
}

func (c *TrivyRescanCron) loop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.rescanAll()
		}
	}
}

// rescanAll 查詢需要重掃的映像並逐一觸發掃描。
func (c *TrivyRescanCron) rescanAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	stale := c.findStaleImages(ctx)
	if len(stale) == 0 {
		logger.Debug("trivy rescan cron: no stale images found")
		return
	}

	logger.Info("trivy rescan cron: starting rescan",
		"stale_count", len(stale),
	)

	scanned := 0
	for _, img := range stale {
		select {
		case <-c.stopCh:
			return
		default:
		}

		_, err := c.trivySvc.TriggerScan(img.ClusterID, img.Namespace, img.PodName, img.ContainerName, img.Image)
		if err != nil {
			logger.Warn("trivy rescan cron: trigger failed",
				"image", img.Image,
				"cluster_id", img.ClusterID,
				"error", err,
			)
			continue
		}
		scanned++
	}

	logger.Info("trivy rescan cron: round completed",
		"triggered", scanned,
		"total_stale", len(stale),
	)
}

// staleImage 代表需要重掃的映像記錄。
type staleImage struct {
	ClusterID     uint
	Namespace     string
	PodName       string
	ContainerName string
	Image         string
}

// findStaleImages 查詢最近一次掃描時間超過 rescanAge 的映像。
// 使用子查詢取得每個 (cluster_id, image) 的最新掃描時間。
func (c *TrivyRescanCron) findStaleImages(ctx context.Context) []staleImage {
	cutoff := time.Now().Add(-c.age)

	var results []staleImage
	err := c.db.WithContext(ctx).
		Model(&models.ImageScanResult{}).
		Select("cluster_id, namespace, pod_name, container_name, image, MAX(scanned_at) as last_scan").
		Where("status = ?", "completed").
		Group("cluster_id, image").
		Having("MAX(scanned_at) < ? OR MAX(scanned_at) IS NULL", cutoff).
		Order("MAX(scanned_at) ASC").
		Limit(c.maxPer).
		Find(&results).Error

	if err != nil {
		logger.Error("trivy rescan cron: query stale images failed", "error", err)
		return nil
	}

	return results
}

// FindStaleImagesForTest 暴露 findStaleImages 供測試使用。
func (c *TrivyRescanCron) FindStaleImagesForTest(ctx context.Context) []staleImage {
	return c.findStaleImages(ctx)
}
