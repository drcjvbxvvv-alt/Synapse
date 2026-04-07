package services

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutslisters "github.com/argoproj/argo-rollouts/pkg/client/listers/rollouts/v1alpha1"
	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// InformerListerProvider 定義獲取各類 Lister 的介面（用於解耦 k8s 包）
type InformerListerProvider interface {
	PodsLister(clusterID uint) corev1listers.PodLister
	NodesLister(clusterID uint) corev1listers.NodeLister
	DeploymentsLister(clusterID uint) appsv1listers.DeploymentLister
	StatefulSetsLister(clusterID uint) appsv1listers.StatefulSetLister
	RolloutsLister(clusterID uint) rolloutslisters.RolloutLister
}

// clusterFilterCtxKey 用於在 context 中傳遞叢集過濾條件
type clusterFilterCtxKey struct{}

// ContextWithClusterFilter 將叢集過濾條件注入 context
func ContextWithClusterFilter(ctx context.Context, clusterIDs []uint) context.Context {
	return context.WithValue(ctx, clusterFilterCtxKey{}, clusterIDs)
}

// OverviewService 總覽服務
type OverviewService struct {
	db                 *gorm.DB
	clusterService     *ClusterService
	listerProvider     InformerListerProvider
	promService        *PrometheusService
	monitoringCfgSvc   *MonitoringConfigService
	alertManagerCfgSvc *AlertManagerConfigService
	alertManagerSvc    *AlertManagerService
}

// NewOverviewService 建立總覽服務
func NewOverviewService(
	db *gorm.DB,
	clusterService *ClusterService,
	listerProvider InformerListerProvider,
	promService *PrometheusService,
	monitoringCfgSvc *MonitoringConfigService,
	alertManagerCfgSvc *AlertManagerConfigService,
	alertManagerSvc *AlertManagerService,
) *OverviewService {
	return &OverviewService{
		db:                 db,
		clusterService:     clusterService,
		listerProvider:     listerProvider,
		promService:        promService,
		monitoringCfgSvc:   monitoringCfgSvc,
		alertManagerCfgSvc: alertManagerCfgSvc,
		alertManagerSvc:    alertManagerSvc,
	}
}

// getClusters 獲取叢集列表，如果 context 中有過濾條件則按條件過濾
func (s *OverviewService) getClusters(ctx context.Context) ([]*models.Cluster, error) {
	if filterIDs, ok := ctx.Value(clusterFilterCtxKey{}).([]uint); ok {
		if len(filterIDs) == 0 {
			return []*models.Cluster{}, nil
		}
		var clusters []*models.Cluster
		if err := s.db.Where("id IN ?", filterIDs).Find(&clusters).Error; err != nil {
			return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
		}
		return clusters, nil
	}
	return s.clusterService.GetAllClusters()
}

// ========== 響應結構體 ==========

// OverviewStatsResponse 總覽統計響應
type OverviewStatsResponse struct {
	ClusterStats        ClusterStatsData      `json:"clusterStats"`
	NodeStats           NodeStatsData         `json:"nodeStats"`
	PodStats            PodStatsData          `json:"podStats"`
	VersionDistribution []VersionDistribution `json:"versionDistribution"`
}

// ClusterStatsData 叢集統計
type ClusterStatsData struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
}

// NodeStatsData 節點統計
type NodeStatsData struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	NotReady int `json:"notReady"`
}

// PodStatsData Pod 統計
type PodStatsData struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Pending   int `json:"pending"`
	Failed    int `json:"failed"`
	Succeeded int `json:"succeeded"`
}

// VersionDistribution 版本分佈
type VersionDistribution struct {
	Version  string   `json:"version"`
	Count    int      `json:"count"`
	Clusters []string `json:"clusters"`
}

// ResourceUsageResponse 資源使用率響應
type ResourceUsageResponse struct {
	CPU     ResourceUsageData `json:"cpu"`
	Memory  ResourceUsageData `json:"memory"`
	Storage ResourceUsageData `json:"storage"`
}

// ResourceUsageData 資源使用資料
type ResourceUsageData struct {
	UsagePercent float64 `json:"usagePercent"`
	Used         float64 `json:"used"`
	Total        float64 `json:"total"`
	Unit         string  `json:"unit"`
}

// ResourceDistributionResponse 資源分佈響應
type ResourceDistributionResponse struct {
	PodDistribution    []ClusterResourceCount `json:"podDistribution"`
	NodeDistribution   []ClusterResourceCount `json:"nodeDistribution"`
	CPUDistribution    []ClusterResourceCount `json:"cpuDistribution"`
	MemoryDistribution []ClusterResourceCount `json:"memoryDistribution"`
}

// ClusterResourceCount 叢集資源計數
type ClusterResourceCount struct {
	ClusterID   uint    `json:"clusterId"`
	ClusterName string  `json:"clusterName"`
	Value       float64 `json:"value"`
}

// TrendResponse 趨勢資料響應
type TrendResponse struct {
	PodTrends  []ClusterTrendSeries `json:"podTrends"`
	NodeTrends []ClusterTrendSeries `json:"nodeTrends"`
}

// ClusterTrendSeries 叢集趨勢序列
type ClusterTrendSeries struct {
	ClusterID   uint             `json:"clusterId"`
	ClusterName string           `json:"clusterName"`
	DataPoints  []TrendDataPoint `json:"dataPoints"`
}

// TrendDataPoint 趨勢資料點
type TrendDataPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// AbnormalWorkload 異常工作負載
type AbnormalWorkload struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	ClusterID   uint   `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Type        string `json:"type"`
	Reason      string `json:"reason"`
	Message     string `json:"message"`
	Duration    string `json:"duration"`
	Severity    string `json:"severity"`
}

// GlobalAlertStats 全域性告警統計
type GlobalAlertStats struct {
	Total        int                 `json:"total"`        // 告警總數
	Firing       int                 `json:"firing"`       // 觸發中
	Pending      int                 `json:"pending"`      // 等待中
	Resolved     int                 `json:"resolved"`     // 已解決
	Suppressed   int                 `json:"suppressed"`   // 已抑制
	BySeverity   map[string]int      `json:"bySeverity"`   // 按嚴重程度統計
	ByCluster    []ClusterAlertCount `json:"byCluster"`    // 按叢集統計
	EnabledCount int                 `json:"enabledCount"` // 已啟用告警的叢集數
}

// ClusterAlertCount 叢集告警計數
type ClusterAlertCount struct {
	ClusterID   uint   `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Total       int    `json:"total"`
	Firing      int    `json:"firing"`
}

// ========== 服務方法 ==========

// GetOverviewStats 獲取總覽統計資料
func (s *OverviewService) GetOverviewStats(ctx context.Context) (*OverviewStatsResponse, error) {
	clusters, err := s.getClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	stats := &OverviewStatsResponse{}
	versionMap := make(map[string][]string)

	for _, cluster := range clusters {
		// 叢集健康統計
		switch cluster.Status {
		case "healthy":
			stats.ClusterStats.Healthy++
		case "unhealthy":
			stats.ClusterStats.Unhealthy++
		default:
			stats.ClusterStats.Unknown++
		}

		// 版本分佈
		version := cluster.Version
		if version == "" {
			version = "unknown"
		}
		versionMap[version] = append(versionMap[version], cluster.Name)

		// 從 Informer 獲取 Pod 統計
		if s.listerProvider != nil {
			podLister := s.listerProvider.PodsLister(cluster.ID)
			if podLister != nil {
				pods, err := podLister.List(labels.Everything())
				if err != nil {
					logger.Error("獲取叢集 Pod 列表失敗", "cluster", cluster.Name, "error", err)
				} else {
					for _, pod := range pods {
						stats.PodStats.Total++
						switch pod.Status.Phase {
						case corev1.PodRunning:
							stats.PodStats.Running++
						case corev1.PodPending:
							stats.PodStats.Pending++
						case corev1.PodFailed:
							stats.PodStats.Failed++
						case corev1.PodSucceeded:
							stats.PodStats.Succeeded++
						}
					}
				}
			}

			// 從 Informer 獲取 Node 統計
			nodeLister := s.listerProvider.NodesLister(cluster.ID)
			if nodeLister != nil {
				nodes, err := nodeLister.List(labels.Everything())
				if err != nil {
					logger.Error("獲取叢集節點列表失敗", "cluster", cluster.Name, "error", err)
				} else {
					for _, node := range nodes {
						stats.NodeStats.Total++
						for _, cond := range node.Status.Conditions {
							if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
								stats.NodeStats.Ready++
								break
							}
						}
					}
				}
			}
		}
	}

	stats.NodeStats.NotReady = stats.NodeStats.Total - stats.NodeStats.Ready
	stats.ClusterStats.Total = len(clusters)

	// 轉換版本分佈
	for version, clusterNames := range versionMap {
		stats.VersionDistribution = append(stats.VersionDistribution, VersionDistribution{
			Version:  version,
			Count:    len(clusterNames),
			Clusters: clusterNames,
		})
	}
	// 按數量降序排序
	sort.Slice(stats.VersionDistribution, func(i, j int) bool {
		return stats.VersionDistribution[i].Count > stats.VersionDistribution[j].Count
	})

	return stats, nil
}

// GetResourceDistribution 獲取資源分佈
func (s *OverviewService) GetResourceDistribution(ctx context.Context) (*ResourceDistributionResponse, error) {
	clusters, err := s.getClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	resp := &ResourceDistributionResponse{
		PodDistribution:    make([]ClusterResourceCount, 0),
		NodeDistribution:   make([]ClusterResourceCount, 0),
		CPUDistribution:    make([]ClusterResourceCount, 0),
		MemoryDistribution: make([]ClusterResourceCount, 0),
	}

	if s.listerProvider == nil {
		return resp, nil
	}

	for _, cluster := range clusters {
		clusterID := cluster.ID
		clusterName := cluster.Name

		// Pod 分佈
		if podLister := s.listerProvider.PodsLister(clusterID); podLister != nil {
			pods, err := podLister.List(labels.Everything())
			if err == nil {
				resp.PodDistribution = append(resp.PodDistribution, ClusterResourceCount{
					ClusterID: clusterID, ClusterName: clusterName, Value: float64(len(pods)),
				})
			}
		}

		// Node 分佈 + CPU/Memory 容量
		if nodeLister := s.listerProvider.NodesLister(clusterID); nodeLister != nil {
			nodes, err := nodeLister.List(labels.Everything())
			if err == nil {
				var totalCPU, totalMemory int64
				for _, node := range nodes {
					// CPU: milliCores -> Cores
					cpu := node.Status.Allocatable.Cpu().MilliValue() / 1000
					// Memory: bytes -> GB
					mem := node.Status.Allocatable.Memory().Value() / (1024 * 1024 * 1024)
					totalCPU += cpu
					totalMemory += mem
				}
				resp.NodeDistribution = append(resp.NodeDistribution, ClusterResourceCount{
					ClusterID: clusterID, ClusterName: clusterName, Value: float64(len(nodes)),
				})
				resp.CPUDistribution = append(resp.CPUDistribution, ClusterResourceCount{
					ClusterID: clusterID, ClusterName: clusterName, Value: float64(totalCPU),
				})
				resp.MemoryDistribution = append(resp.MemoryDistribution, ClusterResourceCount{
					ClusterID: clusterID, ClusterName: clusterName, Value: float64(totalMemory),
				})
			}
		}
	}

	// 按 Value 降序排序
	sortByValue := func(list []ClusterResourceCount) {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Value > list[j].Value
		})
	}
	sortByValue(resp.PodDistribution)
	sortByValue(resp.NodeDistribution)
	sortByValue(resp.CPUDistribution)
	sortByValue(resp.MemoryDistribution)

	return resp, nil
}

// GetResourceUsage 獲取資源使用率
func (s *OverviewService) GetResourceUsage(ctx context.Context) (*ResourceUsageResponse, error) {
	clusters, err := s.getClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	var totalCPUUsage, totalMemUsage, totalStorageUsage float64
	var totalCPUCores, totalMemoryGB float64
	var totalStorageBytes, usedStorageBytes float64
	var clusterCount, storageClusterCount int

	for _, cluster := range clusters {
		// 獲取叢集的 MonitoringConfig
		config, err := s.monitoringCfgSvc.GetMonitoringConfig(cluster.ID)
		if err != nil || config.Type == "disabled" {
			logger.Debug("叢集監控未配置或已禁用", "cluster", cluster.Name)
			continue
		}

		// 設定時間範圍（最近 5 分鐘，用於 range query）
		now := time.Now().Unix()
		start := now - 300 // 5 分鐘前
		step := "1m"

		// 查詢 CPU 使用率
		cpuQuery := &models.MetricsQuery{
			Query: "(1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m]))) * 100",
			Start: start,
			End:   now,
			Step:  step,
		}
		if resp, err := s.promService.QueryPrometheus(ctx, config, cpuQuery); err == nil {
			if val := extractLatestValue(resp); val >= 0 {
				totalCPUUsage += val
			}
		}

		// 查詢記憶體使用率
		memQuery := &models.MetricsQuery{
			Query: "(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100",
			Start: start,
			End:   now,
			Step:  step,
		}
		if resp, err := s.promService.QueryPrometheus(ctx, config, memQuery); err == nil {
			if val := extractLatestValue(resp); val >= 0 {
				totalMemUsage += val
			}
		}

		// 查詢儲存使用率（根目錄 /）
		storageUsageQuery := &models.MetricsQuery{
			Query: "avg((1 - node_filesystem_avail_bytes{mountpoint=\"/\"} / node_filesystem_size_bytes{mountpoint=\"/\"}) * 100)",
			Start: start,
			End:   now,
			Step:  step,
		}
		if resp, err := s.promService.QueryPrometheus(ctx, config, storageUsageQuery); err == nil {
			if val := extractLatestValue(resp); val >= 0 {
				totalStorageUsage += val
				storageClusterCount++
			}
		}

		// 查詢儲存總量（根目錄 /）
		storageTotalQuery := &models.MetricsQuery{
			Query: "sum(node_filesystem_size_bytes{mountpoint=\"/\"})",
			Start: start,
			End:   now,
			Step:  step,
		}
		if resp, err := s.promService.QueryPrometheus(ctx, config, storageTotalQuery); err == nil {
			if val := extractLatestValue(resp); val >= 0 {
				totalStorageBytes += val
			}
		}

		// 查詢儲存已用量（根目錄 /）
		storageUsedQuery := &models.MetricsQuery{
			Query: "sum(node_filesystem_size_bytes{mountpoint=\"/\"} - node_filesystem_avail_bytes{mountpoint=\"/\"})",
			Start: start,
			End:   now,
			Step:  step,
		}
		if resp, err := s.promService.QueryPrometheus(ctx, config, storageUsedQuery); err == nil {
			if val := extractLatestValue(resp); val >= 0 {
				usedStorageBytes += val
			}
		}

		// 從 Informer 獲取總資源容量
		if s.listerProvider != nil {
			if nodeLister := s.listerProvider.NodesLister(cluster.ID); nodeLister != nil {
				nodes, _ := nodeLister.List(labels.Everything())
				for _, node := range nodes {
					totalCPUCores += float64(node.Status.Allocatable.Cpu().MilliValue()) / 1000
					totalMemoryGB += float64(node.Status.Allocatable.Memory().Value()) / (1024 * 1024 * 1024)
				}
			}
		}

		clusterCount++
	}

	resp := &ResourceUsageResponse{}

	if clusterCount > 0 {
		avgCPU := totalCPUUsage / float64(clusterCount)
		avgMem := totalMemUsage / float64(clusterCount)

		resp.CPU = ResourceUsageData{
			UsagePercent: avgCPU,
			Used:         totalCPUCores * avgCPU / 100,
			Total:        totalCPUCores,
			Unit:         "核",
		}
		resp.Memory = ResourceUsageData{
			UsagePercent: avgMem,
			Used:         totalMemoryGB * avgMem / 100,
			Total:        totalMemoryGB,
			Unit:         "GB",
		}
	}

	// 儲存使用率
	if storageClusterCount > 0 {
		avgStorage := totalStorageUsage / float64(storageClusterCount)
		totalStorageTB := totalStorageBytes / (1024 * 1024 * 1024 * 1024)
		usedStorageTB := usedStorageBytes / (1024 * 1024 * 1024 * 1024)

		resp.Storage = ResourceUsageData{
			UsagePercent: avgStorage,
			Used:         usedStorageTB,
			Total:        totalStorageTB,
			Unit:         "TB",
		}
	} else {
		resp.Storage = ResourceUsageData{
			UsagePercent: 0,
			Used:         0,
			Total:        0,
			Unit:         "TB",
		}
	}

	return resp, nil
}

// GetTrends 獲取趨勢資料（併發查詢最佳化效能）
func (s *OverviewService) GetTrends(ctx context.Context, timeRange string, step string) (*TrendResponse, error) {
	clusters, err := s.getClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	// 解析時間範圍
	start, end := parseTimeRange(timeRange)
	// 每天一個資料點，使用 1d 步長
	if step == "" {
		step = "1d"
	}

	resp := &TrendResponse{
		PodTrends:  make([]ClusterTrendSeries, 0),
		NodeTrends: make([]ClusterTrendSeries, 0),
	}

	// 使用併發查詢所有叢集
	type trendResult struct {
		ClusterID   uint
		ClusterName string
		PodPoints   []TrendDataPoint
		NodePoints  []TrendDataPoint
	}

	resultCh := make(chan trendResult, len(clusters))
	var wg sync.WaitGroup

	for _, cluster := range clusters {
		wg.Add(1)
		go func(c *models.Cluster) {
			defer wg.Done()
			clusterStart := time.Now()

			// 在 goroutine 內部獲取監控配置
			config, err := s.monitoringCfgSvc.GetMonitoringConfig(c.ID)
			if err != nil || config.Type == "disabled" {
				return
			}

			result := trendResult{
				ClusterID:   c.ID,
				ClusterName: c.Name,
			}

			// Pod 趨勢 - 直接查詢 count，step=1d 已保證每天一個點
			podQuery := &models.MetricsQuery{
				Query: "count(kube_pod_info)",
				Start: start,
				End:   end,
				Step:  step,
			}
			if promResp, err := s.promService.QueryPrometheus(ctx, config, podQuery); err == nil {
				result.PodPoints = extractRangeSeriesWithDefault(promResp)
			}

			// Node 趨勢 - 直接查詢 count
			nodeQuery := &models.MetricsQuery{
				Query: "count(kube_node_info)",
				Start: start,
				End:   end,
				Step:  step,
			}
			if promResp, err := s.promService.QueryPrometheus(ctx, config, nodeQuery); err == nil {
				result.NodePoints = extractRangeSeriesWithDefault(promResp)
			}

			logger.Info("叢集趨勢查詢完成", "cluster", c.Name, "耗時", time.Since(clusterStart).String())

			resultCh <- result
		}(cluster)
	}

	// 等待所有 goroutine 完成後關閉 channel
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集結果
	for result := range resultCh {
		if len(result.PodPoints) > 0 {
			resp.PodTrends = append(resp.PodTrends, ClusterTrendSeries{
				ClusterID:   result.ClusterID,
				ClusterName: result.ClusterName,
				DataPoints:  result.PodPoints,
			})
		}
		if len(result.NodePoints) > 0 {
			resp.NodeTrends = append(resp.NodeTrends, ClusterTrendSeries{
				ClusterID:   result.ClusterID,
				ClusterName: result.ClusterName,
				DataPoints:  result.NodePoints,
			})
		}
	}

	return resp, nil
}

// GetAbnormalWorkloads 獲取異常工作負載
func (s *OverviewService) GetAbnormalWorkloads(ctx context.Context, limit int) ([]AbnormalWorkload, error) {
	clusters, err := s.getClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}

	var workloads []AbnormalWorkload

	if s.listerProvider == nil {
		return workloads, nil
	}

	for _, cluster := range clusters {
		// 檢查 Deployment 副本不一致
		if depLister := s.listerProvider.DeploymentsLister(cluster.ID); depLister != nil {
			deps, err := depLister.List(labels.Everything())
			if err == nil {
				for _, dep := range deps {
					if dep.Spec.Replicas != nil && dep.Status.ReadyReplicas < *dep.Spec.Replicas {
						duration := formatDuration(dep.CreationTimestamp.Time)
						workloads = append(workloads, AbnormalWorkload{
							Name:        dep.Name,
							Namespace:   dep.Namespace,
							ClusterID:   cluster.ID,
							ClusterName: cluster.Name,
							Type:        "Deployment",
							Reason:      "Pod副本不足",
							Message:     fmt.Sprintf("期望 %d 個副本，就緒 %d 個", *dep.Spec.Replicas, dep.Status.ReadyReplicas),
							Duration:    duration,
							Severity:    "warning",
						})
					}
				}
			}
		}

		// 檢查 StatefulSet 副本不一致
		if stsLister := s.listerProvider.StatefulSetsLister(cluster.ID); stsLister != nil {
			stss, err := stsLister.List(labels.Everything())
			if err == nil {
				for _, sts := range stss {
					if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
						duration := formatDuration(sts.CreationTimestamp.Time)
						workloads = append(workloads, AbnormalWorkload{
							Name:        sts.Name,
							Namespace:   sts.Namespace,
							ClusterID:   cluster.ID,
							ClusterName: cluster.Name,
							Type:        "StatefulSet",
							Reason:      "Pod副本不足",
							Message:     fmt.Sprintf("期望 %d 個副本，就緒 %d 個", *sts.Spec.Replicas, sts.Status.ReadyReplicas),
							Duration:    duration,
							Severity:    "warning",
						})
					}
				}
			}
		}

		// 檢查 Argo Rollout 副本不一致或釋出異常
		if rolloutLister := s.listerProvider.RolloutsLister(cluster.ID); rolloutLister != nil {
			rollouts, err := rolloutLister.List(labels.Everything())
			if err == nil {
				for _, rollout := range rollouts {
					reason, msg, severity := detectRolloutIssue(rollout)
					if reason != "" {
						duration := formatDuration(rollout.CreationTimestamp.Time)
						workloads = append(workloads, AbnormalWorkload{
							Name:        rollout.Name,
							Namespace:   rollout.Namespace,
							ClusterID:   cluster.ID,
							ClusterName: cluster.Name,
							Type:        "Rollout",
							Reason:      reason,
							Message:     msg,
							Duration:    duration,
							Severity:    severity,
						})
					}
				}
			}
		}

		// 檢查異常 Pod
		if podLister := s.listerProvider.PodsLister(cluster.ID); podLister != nil {
			pods, err := podLister.List(labels.Everything())
			if err == nil {
				for _, pod := range pods {
					if reason, severity := detectPodIssue(pod); reason != "" {
						duration := formatDuration(pod.CreationTimestamp.Time)
						workloads = append(workloads, AbnormalWorkload{
							Name:        pod.Name,
							Namespace:   pod.Namespace,
							ClusterID:   cluster.ID,
							ClusterName: cluster.Name,
							Type:        "Pod",
							Reason:      reason,
							Duration:    duration,
							Severity:    severity,
						})
					}
				}
			}
		}

		// 限制數量
		if len(workloads) >= limit {
			break
		}
	}

	// 截斷到限制數量
	if len(workloads) > limit {
		workloads = workloads[:limit]
	}

	return workloads, nil
}

// GetGlobalAlertStats 獲取全域性告警統計（聚合所有叢集的告警資料）
func (s *OverviewService) GetGlobalAlertStats(ctx context.Context) (*GlobalAlertStats, error) {
	clusters, err := s.getClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	stats := &GlobalAlertStats{
		BySeverity: make(map[string]int),
		ByCluster:  make([]ClusterAlertCount, 0),
	}

	if s.alertManagerCfgSvc == nil || s.alertManagerSvc == nil {
		logger.Warn("AlertManager 服務未配置，返回空統計")
		return stats, nil
	}

	// 併發獲取各叢集告警
	type clusterResult struct {
		ClusterID   uint
		ClusterName string
		Stats       *models.AlertStats
		Enabled     bool
		Err         error
	}

	resultCh := make(chan clusterResult, len(clusters))
	var wg sync.WaitGroup

	for _, cluster := range clusters {
		wg.Add(1)
		go func(c *models.Cluster) {
			defer wg.Done()

			result := clusterResult{
				ClusterID:   c.ID,
				ClusterName: c.Name,
			}

			// 獲取叢集的 AlertManager 配置
			config, err := s.alertManagerCfgSvc.GetAlertManagerConfig(c.ID)
			if err != nil {
				result.Err = err
				resultCh <- result
				return
			}

			if !config.Enabled {
				result.Enabled = false
				resultCh <- result
				return
			}

			result.Enabled = true

			// 獲取告警統計
			alertStats, err := s.alertManagerSvc.GetAlertStats(ctx, config)
			if err != nil {
				logger.Warn("獲取叢集告警統計失敗", "cluster", c.Name, "error", err)
				result.Err = err
				resultCh <- result
				return
			}

			result.Stats = alertStats
			resultCh <- result
		}(cluster)
	}

	// 等待完成後關閉 channel
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 彙總結果
	for result := range resultCh {
		if !result.Enabled {
			continue
		}

		stats.EnabledCount++

		if result.Stats == nil {
			continue
		}

		// 彙總總數
		stats.Total += result.Stats.Total
		stats.Firing += result.Stats.Firing
		stats.Pending += result.Stats.Pending
		stats.Resolved += result.Stats.Resolved
		stats.Suppressed += result.Stats.Suppressed

		// 彙總按嚴重程度
		for severity, count := range result.Stats.BySeverity {
			stats.BySeverity[severity] += count
		}

		// 記錄每個叢集的告警數
		stats.ByCluster = append(stats.ByCluster, ClusterAlertCount{
			ClusterID:   result.ClusterID,
			ClusterName: result.ClusterName,
			Total:       result.Stats.Total,
			Firing:      result.Stats.Firing,
		})
	}

	// 按告警數排序
	sort.Slice(stats.ByCluster, func(i, j int) bool {
		return stats.ByCluster[i].Firing > stats.ByCluster[j].Firing
	})

	return stats, nil
}

// ========== 輔助函式 ==========

// detectPodIssue 檢測 Pod 異常
func detectPodIssue(pod *corev1.Pod) (string, string) {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			switch cs.State.Waiting.Reason {
			case "ImagePullBackOff", "ErrImagePull":
				return "映像拉取失敗", "critical"
			case "CrashLoopBackOff":
				return "容器崩潰重啟", "critical"
			case "CreateContainerConfigError":
				return "容器配置錯誤", "warning"
			}
		}
		if cs.LastTerminationState.Terminated != nil {
			if cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return "OOM 記憶體溢位", "critical"
			}
		}
	}
	if pod.Status.Phase == corev1.PodPending {
		// 檢查是否 Pending 超過 5 分鐘
		if time.Since(pod.CreationTimestamp.Time) > 5*time.Minute {
			return "排程超時", "warning"
		}
	}
	return "", ""
}

// detectRolloutIssue 檢測 Argo Rollout 異常
func detectRolloutIssue(rollout *rolloutsv1alpha1.Rollout) (string, string, string) {
	// 檢查副本不一致
	if rollout.Spec.Replicas != nil {
		desired := *rollout.Spec.Replicas
		ready := rollout.Status.ReadyReplicas
		if ready < desired {
			return "Pod副本不足", fmt.Sprintf("期望 %d 個副本，就緒 %d 個", desired, ready), "warning"
		}
	}

	// 檢查釋出狀態
	phase := rollout.Status.Phase
	switch phase {
	case rolloutsv1alpha1.RolloutPhaseDegraded:
		return "釋出降級", "Rollout 處於降級狀態", "critical"
	case rolloutsv1alpha1.RolloutPhasePaused:
		// 暫停狀態可能是正常的（金絲雀釋出暫停），檢查是否有異常條件
		for _, cond := range rollout.Status.Conditions {
			if cond.Type == rolloutsv1alpha1.RolloutProgressing && cond.Reason == "ProgressDeadlineExceeded" {
				return "釋出超時", cond.Message, "critical"
			}
		}
	}

	// 檢查 Condition 中的異常
	for _, cond := range rollout.Status.Conditions {
		if cond.Type == rolloutsv1alpha1.RolloutProgressing && cond.Reason == "ProgressDeadlineExceeded" {
			return "釋出超時", cond.Message, "critical"
		}
		if cond.Type == rolloutsv1alpha1.RolloutReplicaFailure {
			return "副本失敗", cond.Message, "critical"
		}
	}

	return "", "", ""
}

// formatDuration 格式化持續時間
func formatDuration(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分鐘", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d小時", int(d.Hours()))
	}
	return fmt.Sprintf("%d天", int(d.Hours()/24))
}

// parseTimeRange 解析時間範圍
func parseTimeRange(timeRange string) (int64, int64) {
	end := time.Now().Unix()
	var start int64
	switch timeRange {
	case "1h":
		start = end - 3600
	case "6h":
		start = end - 6*3600
	case "1d":
		start = end - 24*3600
	case "7d":
		start = end - 7*24*3600
	case "30d":
		start = end - 30*24*3600
	default:
		start = end - 7*24*3600
	}
	return start, end
}

// extractInstantValue 從 Prometheus 響應中提取即時值（用於 instant query）
//nolint:unused // 保留用於未來使用
func extractInstantValue(resp *models.MetricsResponse) float64 {
	if resp == nil || len(resp.Data.Result) == 0 {
		return -1
	}
	result := resp.Data.Result[0]
	if len(result.Value) >= 2 {
		if val, ok := result.Value[1].(string); ok {
			var f float64
			if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
				return f
			}
		}
	}
	return -1
}

// extractLatestValue 從 Prometheus range query 響應中提取最新值
func extractLatestValue(resp *models.MetricsResponse) float64 {
	if resp == nil || len(resp.Data.Result) == 0 {
		return -1
	}
	result := resp.Data.Result[0]
	// 優先從 Values (range query) 中獲取最後一個值
	if len(result.Values) > 0 {
		lastValue := result.Values[len(result.Values)-1]
		if len(lastValue) >= 2 {
			if strVal, ok := lastValue[1].(string); ok {
				var f float64
				if _, err := fmt.Sscanf(strVal, "%f", &f); err == nil {
					return f
				}
			}
		}
	}
	// 相容 instant query 的 Value 格式
	if len(result.Value) >= 2 {
		if val, ok := result.Value[1].(string); ok {
			var f float64
			if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
				return f
			}
		}
	}
	return -1
}

// extractRangeSeries 從 Prometheus 響應中提取範圍序列
//nolint:unused // 保留用於未來使用
func extractRangeSeries(resp *models.MetricsResponse) []TrendDataPoint {
	if resp == nil || len(resp.Data.Result) == 0 {
		return nil
	}
	result := resp.Data.Result[0]
	var points []TrendDataPoint
	for _, v := range result.Values {
		if len(v) >= 2 {
			ts, _ := v[0].(float64)
			var val float64
			if strVal, ok := v[1].(string); ok {
				_, _ = fmt.Sscanf(strVal, "%f", &val)
			}
			points = append(points, TrendDataPoint{
				Timestamp: int64(ts),
				Value:     val,
			})
		}
	}
	return points
}

// extractRangeSeriesWithDefault 從 Prometheus 響應中提取範圍序列，處理 null 值
// 如果某個時間點的值為 null 或無效，使用前一個有效值填充
func extractRangeSeriesWithDefault(resp *models.MetricsResponse) []TrendDataPoint {
	if resp == nil || len(resp.Data.Result) == 0 {
		return nil
	}
	result := resp.Data.Result[0]
	var points []TrendDataPoint
	var lastValidValue float64 = 0

	for _, v := range result.Values {
		if len(v) >= 2 {
			ts, _ := v[0].(float64)
			var val float64
			var valid bool

			if strVal, ok := v[1].(string); ok && strVal != "" && strVal != "NaN" && strVal != "null" {
				n, err := fmt.Sscanf(strVal, "%f", &val)
				valid = (n == 1 && err == nil)
			}

			if valid {
				lastValidValue = val
			} else {
				// 使用前一個有效值
				val = lastValidValue
			}

			points = append(points, TrendDataPoint{
				Timestamp: int64(ts),
				Value:     val,
			})
		}
	}
	return points
}

// 確保 appsv1 包被使用（避免 unused import 錯誤）
var _ *appsv1.Deployment = nil
