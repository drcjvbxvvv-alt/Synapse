package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"gorm.io/gorm"
)

const clusterListCacheTTL = 30 * time.Second

type allClustersCache struct {
	clusters  []*models.Cluster
	expiresAt time.Time
}

// StoredCluster 儲存的叢集資訊結構體
type StoredCluster struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	ApiServer     string            `json:"apiServer"`
	Version       string            `json:"version"`
	Status        string            `json:"status"`
	Labels        map[string]string `json:"labels"`
	CreatedAt     time.Time         `json:"createdAt"`
	LastHeartbeat time.Time         `json:"lastHeartbeat"`
}

// ClusterService 叢集服務
type ClusterService struct {
	db       *gorm.DB
	cacheMu  sync.RWMutex
	allCache *allClustersCache
}

// NewClusterService 建立叢集服務
func NewClusterService(db *gorm.DB) *ClusterService {
	return &ClusterService{db: db}
}

// invalidateClusterCache 清除叢集列表快取（Create/Delete 時呼叫）
func (s *ClusterService) invalidateClusterCache() {
	s.cacheMu.Lock()
	s.allCache = nil
	s.cacheMu.Unlock()
}

// CreateCluster 建立叢集
func (s *ClusterService) CreateCluster(cluster *models.Cluster) error {
	// 設定建立時間
	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()
	cluster.LastHeartbeat = &cluster.CreatedAt

	// 確保 MonitoringConfig 是有效的 JSON，避免 MySQL JSON 欄位報錯
	if cluster.MonitoringConfig == "" {
		cluster.MonitoringConfig = "{}"
	}
	// 驗證 MonitoringConfig 是否為有效的 JSON
	if cluster.MonitoringConfig != "" {
		var testJSON interface{}
		if err := json.Unmarshal([]byte(cluster.MonitoringConfig), &testJSON); err != nil {
			// 如果不是有效的 JSON，設定為空物件
			cluster.MonitoringConfig = "{}"
		}
	}

	// 確保 AlertManagerConfig 是有效的 JSON，避免 MySQL JSON 欄位報錯
	if cluster.AlertManagerConfig == "" {
		cluster.AlertManagerConfig = "{}"
	}
	// 驗證 AlertManagerConfig 是否為有效的 JSON
	if cluster.AlertManagerConfig != "" {
		var testJSON interface{}
		if err := json.Unmarshal([]byte(cluster.AlertManagerConfig), &testJSON); err != nil {
			// 如果不是有效的 JSON，設定為空物件
			cluster.AlertManagerConfig = "{}"
		}
	}

	// 儲存到資料庫
	if err := s.db.Create(cluster).Error; err != nil {
		logger.Error("建立叢集失敗", "error", err)
		return fmt.Errorf("建立叢集失敗: %w", err)
	}

	s.invalidateClusterCache()
	logger.Info("叢集建立成功", "id", cluster.ID, "name", cluster.Name)
	return nil
}

// GetCluster 獲取單個叢集
func (s *ClusterService) GetCluster(id uint) (*models.Cluster, error) {
	var cluster models.Cluster
	if err := s.db.First(&cluster, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("叢集不存在: %d", id)
		}
		return nil, fmt.Errorf("獲取叢集失敗: %w", err)
	}
	return &cluster, nil
}

// GetAllClusters 獲取所有叢集（附 30 秒 TTL 快取，Create/Delete 時自動失效）
func (s *ClusterService) GetAllClusters() ([]*models.Cluster, error) {
	s.cacheMu.RLock()
	if s.allCache != nil && time.Now().Before(s.allCache.expiresAt) {
		result := s.allCache.clusters
		s.cacheMu.RUnlock()
		return result, nil
	}
	s.cacheMu.RUnlock()

	var clusters []*models.Cluster
	if err := s.db.Find(&clusters).Error; err != nil {
		logger.Error("獲取叢集列表失敗", "error", err)
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	s.cacheMu.Lock()
	s.allCache = &allClustersCache{clusters: clusters, expiresAt: time.Now().Add(clusterListCacheTTL)}
	s.cacheMu.Unlock()
	return clusters, nil
}

// GetConnectableClusters 獲取可連線的叢集（排除 unhealthy 狀態），用於 Informer 預熱
func (s *ClusterService) GetConnectableClusters() ([]*models.Cluster, error) {
	var clusters []*models.Cluster
	if err := s.db.Where("status != ?", "unhealthy").Find(&clusters).Error; err != nil {
		logger.Error("獲取可連線叢集列表失敗", "error", err)
		return nil, fmt.Errorf("獲取可連線叢集列表失敗: %w", err)
	}
	return clusters, nil
}

// UpdateClusterStatus 更新叢集狀態
func (s *ClusterService) UpdateClusterStatus(id uint, status string, version string) error {
	now := time.Now()
	result := s.db.Model(&models.Cluster{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":         status,
		"version":        version,
		"last_heartbeat": &now,
		"updated_at":     now,
	})

	if result.Error != nil {
		return fmt.Errorf("更新叢集狀態失敗: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("叢集不存在: %d", id)
	}

	return nil
}

// DeleteCluster 刪除叢集
func (s *ClusterService) DeleteCluster(id uint) error {
	// 使用事務確保資料一致性
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 檢查叢集是否存在
		var cluster models.Cluster
		if err := tx.First(&cluster, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("叢集不存在: %d", id)
			}
			return fmt.Errorf("查詢叢集失敗: %w", err)
		}

		// 2. 刪除關聯的叢集權限（硬刪除）
		if err := tx.Unscoped().Where("cluster_id = ?", id).Delete(&models.ClusterPermission{}).Error; err != nil {
			logger.Error("刪除叢集權限失敗", "cluster_id", id, "error", err)
			return fmt.Errorf("刪除叢集權限失敗: %w", err)
		}
		logger.Info("已刪除叢集關聯的權限", "cluster_id", id)

		// 3. 刪除關聯的終端會話（硬刪除）
		// 先刪除終端命令記錄
		if err := tx.Unscoped().Exec(`
			DELETE FROM terminal_commands 
			WHERE session_id IN (SELECT id FROM terminal_sessions WHERE cluster_id = ?)
		`, id).Error; err != nil {
			logger.Error("刪除終端命令記錄失敗", "cluster_id", id, "error", err)
			return fmt.Errorf("刪除終端命令記錄失敗: %w", err)
		}
		// 再刪除終端會話
		if err := tx.Unscoped().Where("cluster_id = ?", id).Delete(&models.TerminalSession{}).Error; err != nil {
			logger.Error("刪除終端會話失敗", "cluster_id", id, "error", err)
			return fmt.Errorf("刪除終端會話失敗: %w", err)
		}
		logger.Info("已刪除叢集關聯的終端會話", "cluster_id", id)

		// 4. 刪除關聯的 ArgoCD 配置（硬刪除）
		if err := tx.Unscoped().Where("cluster_id = ?", id).Delete(&models.ArgoCDConfig{}).Error; err != nil {
			logger.Error("刪除 ArgoCD 配置失敗", "cluster_id", id, "error", err)
			return fmt.Errorf("刪除 ArgoCD 配置失敗: %w", err)
		}
		logger.Info("已刪除叢集關聯的 ArgoCD 配置", "cluster_id", id)

		// 5. 清空關聯的操作日誌的叢集引用（保留日誌記錄，只清空叢集ID）
		if err := tx.Model(&models.OperationLog{}).Where("cluster_id = ?", id).Update("cluster_id", nil).Error; err != nil {
			logger.Error("清空操作日誌叢集引用失敗", "cluster_id", id, "error", err)
			// 操作日誌清空失敗不阻止刪除
		}

		// 6. 刪除叢集監控指標
		if err := tx.Where("cluster_id = ?", id).Delete(&models.ClusterMetrics{}).Error; err != nil {
			logger.Error("刪除叢集監控指標失敗", "cluster_id", id, "error", err)
			// 監控指標刪除失敗不阻止刪除
		}

		// 7. 硬刪除叢集（使用 Unscoped 繞過軟刪除）
		if err := tx.Unscoped().Delete(&cluster).Error; err != nil {
			return fmt.Errorf("刪除叢集失敗: %w", err)
		}

		s.invalidateClusterCache()
		logger.Info("叢集刪除成功", "id", id, "name", cluster.Name)
		return nil
	})
}

// GetClusterStats 獲取叢集統計資訊
func (s *ClusterService) GetClusterStats() (*models.ClusterStats, error) {
	var stats models.ClusterStats
	var totalCount, healthyCount, unhealthyCount int64

	// 統計總叢集數
	if err := s.db.Model(&models.Cluster{}).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("統計總叢集數失敗: %w", err)
	}
	stats.TotalClusters = int(totalCount)

	// 統計健康叢集數
	if err := s.db.Model(&models.Cluster{}).Where("status = ?", "healthy").Count(&healthyCount).Error; err != nil {
		return nil, fmt.Errorf("統計健康叢集數失敗: %w", err)
	}
	stats.HealthyClusters = int(healthyCount)

	// 統計異常叢集數
	if err := s.db.Model(&models.Cluster{}).Where("status = ?", "unhealthy").Count(&unhealthyCount).Error; err != nil {
		return nil, fmt.Errorf("統計異常叢集數失敗: %w", err)
	}
	stats.UnhealthyClusters = int(unhealthyCount)

	// 獲取所有叢集的實時指標統計
	var clusters []*models.Cluster
	if err := s.db.Find(&clusters).Error; err != nil {
		logger.Error("獲取叢集列表失敗", "error", err)
		return &stats, nil // 返回基礎統計，不因為指標獲取失敗而整體失敗
	}

	// 並行獲取各叢集實時指標（避免串行 K8s API 呼叫導致 N*timeout 延遲）
	var (
		mu          sync.Mutex
		totalNodes  int
		readyNodes  int
		totalPods   int
		runningPods int
		wg          sync.WaitGroup
	)
	for _, cluster := range clusters {
		cluster := cluster // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m := s.getClusterRealTimeMetrics(cluster); m != nil {
				mu.Lock()
				totalNodes += m.NodeCount
				readyNodes += m.ReadyNodes
				totalPods += m.PodCount
				runningPods += m.RunningPods
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	stats.TotalNodes = totalNodes
	stats.ReadyNodes = readyNodes
	stats.TotalPods = totalPods
	stats.RunningPods = runningPods

	return &stats, nil
}

// getClusterRealTimeMetrics 獲取叢集實時指標
func (s *ClusterService) getClusterRealTimeMetrics(cluster *models.Cluster) *models.ClusterMetrics {
	// 如果沒有連線資訊，返回空指標
	if cluster.KubeconfigEnc == "" && cluster.SATokenEnc == "" {
		return nil
	}

	k8sClient, err := NewK8sClientForCluster(cluster)

	if err != nil {
		logger.Error("建立K8s客戶端失敗", "cluster", cluster.Name, "error", err)
		return nil
	}

	// 獲取叢集資訊
	clusterInfo, err := k8sClient.TestConnection()
	if err != nil {
		logger.Error("獲取叢集資訊失敗", "cluster", cluster.Name, "error", err)
		return nil
	}

	// 從 K8s API 統計 Pod 數量（使用 15 秒超時避免阻塞）
	podCount, runningPods := fetchPodStats(k8sClient)
	cpuPct, memPct := fetchResourceMetrics(k8sClient)

	// 建立指標物件
	metrics := &models.ClusterMetrics{
		ClusterID:   cluster.ID,
		NodeCount:   clusterInfo.NodeCount,
		ReadyNodes:  clusterInfo.ReadyNodes,
		PodCount:    podCount,
		RunningPods: runningPods,
		CPUUsage:    cpuPct,
		MemoryUsage: memPct,
		UpdatedAt:   time.Now(),
	}

	return metrics
}

// fetchPodStats lists all pods across namespaces and returns (total, running) counts.
// Uses a 15-second context timeout to avoid blocking the stats call.
func fetchPodStats(kc *K8sClient) (total int, running int) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	podList, err := kc.GetClientset().CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Warn("獲取 Pod 列表失敗", "error", err)
		return 0, 0
	}
	total = len(podList.Items)
	for i := range podList.Items {
		if podList.Items[i].Status.Phase == corev1.PodRunning {
			running++
		}
	}
	return
}

type nodeMetricsList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Usage struct {
			CPU    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"usage"`
	} `json:"items"`
}

func fetchResourceMetrics(kc *K8sClient) (cpuPercent, memPercent float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw, err := kc.GetClientset().RESTClient().Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").
		DoRaw(ctx)
	if err != nil {
		logger.Warn("metrics-server 不可用，CPU/MEM 指標返回 0", "error", err)
		return 0, 0
	}

	var metricsList nodeMetricsList
	if err := json.Unmarshal(raw, &metricsList); err != nil {
		return 0, 0
	}

	nodes, err := kc.GetClientset().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return 0, 0
	}

	var totalCPUCapMillis, totalMemCapKi int64
	for i := range nodes.Items {
		if cpu := nodes.Items[i].Status.Allocatable.Cpu(); cpu != nil {
			totalCPUCapMillis += cpu.MilliValue()
		}
		if mem := nodes.Items[i].Status.Allocatable.Memory(); mem != nil {
			totalMemCapKi += mem.Value() / 1024
		}
	}
	if totalCPUCapMillis == 0 || totalMemCapKi == 0 {
		return 0, 0
	}

	var usedCPUMillis, usedMemKi int64
	for _, item := range metricsList.Items {
		if q, err := resource.ParseQuantity(item.Usage.CPU); err == nil {
			usedCPUMillis += q.MilliValue()
		}
		if q, err := resource.ParseQuantity(item.Usage.Memory); err == nil {
			usedMemKi += q.Value() / 1024
		}
	}

	cpuPercent = float64(usedCPUMillis) / float64(totalCPUCapMillis) * 100
	memPercent = float64(usedMemKi) / float64(totalMemCapKi) * 100
	return
}

// UpdateClusterMetrics 更新叢集指標到資料庫
func (s *ClusterService) UpdateClusterMetrics(clusterID uint, metrics *models.ClusterMetrics) error {
	// 使用UPSERT操作，如果記錄存在則更新，不存在則插入
	return s.db.Save(metrics).Error
}

// GetClusterMetrics 獲取叢集指標
func (s *ClusterService) GetClusterMetrics(clusterID uint) (*models.ClusterMetrics, error) {
	var metrics models.ClusterMetrics
	if err := s.db.Where("cluster_id = ?", clusterID).First(&metrics).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 沒有找到指標記錄，返回nil而不是錯誤
		}
		return nil, fmt.Errorf("獲取叢集指標失敗: %w", err)
	}
	return &metrics, nil
}

// ConvertToStoredCluster 將資料庫模型轉換為儲存格式
func (s *ClusterService) ConvertToStoredCluster(cluster *models.Cluster) *StoredCluster {
	var labels map[string]string
	if cluster.Labels != "" {
		_ = json.Unmarshal([]byte(cluster.Labels), &labels)
	}
	if labels == nil {
		labels = make(map[string]string)
	}

	stored := &StoredCluster{
		ID:          fmt.Sprintf("%d", cluster.ID),
		Name:        cluster.Name,
		Description: "", // 資料庫模型中沒有description欄位，可以後續新增
		ApiServer:   cluster.APIServer,
		Version:     cluster.Version,
		Status:      cluster.Status,
		Labels:      labels,
		CreatedAt:   cluster.CreatedAt,
	}

	if cluster.LastHeartbeat != nil {
		stored.LastHeartbeat = *cluster.LastHeartbeat
	}

	return stored
}

// ConvertFromStoredCluster 將儲存格式轉換為資料庫模型
func (s *ClusterService) ConvertFromStoredCluster(stored *StoredCluster) *models.Cluster {
	labelsJSON := ""
	if len(stored.Labels) > 0 {
		if data, err := json.Marshal(stored.Labels); err == nil {
			labelsJSON = string(data)
		}
	}

	cluster := &models.Cluster{
		Name:      stored.Name,
		APIServer: stored.ApiServer,
		Version:   stored.Version,
		Status:    stored.Status,
		Labels:    labelsJSON,
		CreatedAt: stored.CreatedAt,
	}

	if !stored.LastHeartbeat.IsZero() {
		cluster.LastHeartbeat = &stored.LastHeartbeat
	}

	return cluster
}
