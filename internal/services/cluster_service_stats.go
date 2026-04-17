package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// GetClusterStats 獲取叢集統計資訊
func (s *ClusterService) GetClusterStats(ctx context.Context) (*models.ClusterStats, error) {
	var stats models.ClusterStats

	totalCount, err := s.countClusters(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("統計總叢集數失敗: %w", err)
	}
	stats.TotalClusters = int(totalCount)

	healthyCount, err := s.countClusters(ctx, "healthy")
	if err != nil {
		return nil, fmt.Errorf("統計健康叢集數失敗: %w", err)
	}
	stats.HealthyClusters = int(healthyCount)

	unhealthyCount, err := s.countClusters(ctx, "unhealthy")
	if err != nil {
		return nil, fmt.Errorf("統計異常叢集數失敗: %w", err)
	}
	stats.UnhealthyClusters = int(unhealthyCount)

	// 獲取所有叢集的實時指標統計
	clusters, err := s.GetAllClusters(ctx)
	if err != nil {
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

// countClusters is a small helper that keeps GetClusterStats tidy by picking
// the right dual-path count based on the feature flag.
func (s *ClusterService) countClusters(ctx context.Context, status string) (int64, error) {
	if s.useRepo() {
		return s.repo.CountByStatus(ctx, status)
	}
	q := s.db.WithContext(ctx).Model(&models.Cluster{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	return total, q.Count(&total).Error
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
//
// Still uses Save() directly — ClusterMetrics has a compound key (cluster_id
// is the logical primary) and GORM's Save() handles the upsert semantics
// implicitly. The Repository layer has no dedicated metrics API; this call
// keeps the legacy path until a ClusterMetricsRepository shows up.
func (s *ClusterService) UpdateClusterMetrics(ctx context.Context, clusterID uint, metrics *models.ClusterMetrics) error {
	return s.db.WithContext(ctx).Save(metrics).Error
}

// GetClusterMetrics 獲取叢集指標
func (s *ClusterService) GetClusterMetrics(ctx context.Context, clusterID uint) (*models.ClusterMetrics, error) {
	var metrics models.ClusterMetrics
	if err := s.db.WithContext(ctx).Where("cluster_id = ?", clusterID).First(&metrics).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

// ListMonitoringClusters returns all clusters that have monitoring enabled,
// converted to the DataSourceClusterInfo shape used by GrafanaService.
func (s *ClusterService) ListMonitoringClusters(ctx context.Context) []DataSourceClusterInfo {
	var clusters []models.Cluster
	if err := s.db.WithContext(ctx).
		Select("name, monitoring_config").
		Where("monitoring_config != '' AND monitoring_config IS NOT NULL").
		Find(&clusters).Error; err != nil {
		logger.Error("查詢叢集監控配置失敗", "error", err)
		return nil
	}

	result := make([]DataSourceClusterInfo, 0, len(clusters))
	for _, cluster := range clusters {
		var config models.MonitoringConfig
		if err := json.Unmarshal([]byte(cluster.MonitoringConfig), &config); err != nil {
			continue
		}
		if config.Type == "disabled" || config.Endpoint == "" {
			continue
		}
		result = append(result, DataSourceClusterInfo{
			ClusterName:   cluster.Name,
			PrometheusURL: config.Endpoint,
		})
	}
	return result
}
