package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	rolloutslisters "github.com/argoproj/argo-rollouts/pkg/client/listers/rollouts/v1alpha1"
	"gorm.io/gorm"
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
		return s.clusterService.GetClustersByIDs(ctx, filterIDs)
	}
	return s.clusterService.GetAllClusters(ctx)
}

// ========== 服務方法 ==========

// GetOverviewStats 獲取總覽統計資料
func (s *OverviewService) GetOverviewStats(ctx context.Context) (*OverviewStatsResponse, error) {
	clusters, err := s.getClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	stats := &OverviewStatsResponse{
		VersionDistribution: make([]VersionDistribution, 0),
	}
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

