package services

import (
	"context"
	"time"

	"github.com/shaia/Synapse/internal/models"
	corev1 "k8s.io/api/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"gorm.io/gorm"
)

// K8sInformerManager is the subset of k8s.ClusterInformerManager needed by resource/cost services.
// Using an interface breaks the import cycle: services ↔ k8s.
type K8sInformerManager interface {
	EnsureSync(ctx context.Context, cluster *models.Cluster, timeout time.Duration) error
	PodsLister(clusterID uint) corev1listers.PodLister
	NodesLister(clusterID uint) corev1listers.NodeLister
	DeploymentsLister(clusterID uint) appsv1listers.DeploymentLister
	StatefulSetsLister(clusterID uint) appsv1listers.StatefulSetLister
	DaemonSetsLister(clusterID uint) appsv1listers.DaemonSetLister
}

// ---- 資料結構 ----

// ResourceMetrics CPU（millicores）與記憶體（MiB）的組合
type ResourceMetrics struct {
	CPUMillicores float64 `json:"cpu_millicores"`
	MemoryMiB     float64 `json:"memory_mib"`
}

// OccupancyPercent 資源佔用百分比
type OccupancyPercent struct {
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
}

// ClusterResourceSnapshot 叢集即時資源佔用快照（由 K8s Informer 即時計算）
type ClusterResourceSnapshot struct {
	ClusterID   uint             `json:"cluster_id"`
	CollectedAt time.Time        `json:"collected_at"`
	Allocatable ResourceMetrics  `json:"allocatable"`
	Requested   ResourceMetrics  `json:"requested"`
	Occupancy   OccupancyPercent `json:"occupancy"`
	Headroom    ResourceMetrics  `json:"headroom"`
	NodeCount   int              `json:"node_count"`
	PodCount    int              `json:"pod_count"`
	HasMetrics  bool             `json:"has_metrics"` // 保留，Phase 2 加入 Prometheus 效率資料時使用
}

// NamespaceOccupancy 命名空間資源佔用（相對叢集 Allocatable）
type NamespaceOccupancy struct {
	Namespace        string  `json:"namespace"`
	CPURequest       float64 `json:"cpu_request_millicores"`
	MemoryRequest    float64 `json:"memory_request_mib"`
	CPUOccupancy     float64 `json:"cpu_occupancy_percent"`    // 佔叢集 allocatable 的 %
	MemoryOccupancy  float64 `json:"memory_occupancy_percent"`
	PodCount         int     `json:"pod_count"`
}

// ClusterResourceSummary 跨叢集彙總中，單個叢集的摘要
type ClusterResourceSummary struct {
	ClusterID          uint    `json:"cluster_id"`
	ClusterName        string  `json:"cluster_name"`
	CPUOccupancy       float64 `json:"cpu_occupancy_percent"`
	MemoryOccupancy    float64 `json:"memory_occupancy_percent"`
	AllocatableCPU     float64 `json:"allocatable_cpu_millicores"`
	AllocatableMemory  float64 `json:"allocatable_memory_mib"`
	RequestedCPU       float64 `json:"requested_cpu_millicores"`
	RequestedMemory    float64 `json:"requested_memory_mib"`
	NodeCount          int     `json:"node_count"`
	PodCount           int     `json:"pod_count"`
	InformerReady      bool    `json:"informer_ready"`
}

// GlobalResourceOverview 跨叢集全平台資源彙總
type GlobalResourceOverview struct {
	CollectedAt      time.Time                `json:"collected_at"`
	ClusterCount     int                      `json:"cluster_count"`
	ReadyCount       int                      `json:"ready_count"` // Informer 就緒的叢集數
	AvgCPUOccupancy  float64                  `json:"avg_cpu_occupancy_percent"`
	AvgMemOccupancy  float64                  `json:"avg_memory_occupancy_percent"`
	Clusters         []ClusterResourceSummary `json:"clusters"`
}

// ---- ResourceService ----

// ResourceService 資源治理服務（Phase 1：K8s API；Phase 2 加入 Prometheus）
type ResourceService struct {
	db         *gorm.DB
	k8sMgr     K8sInformerManager
	clusterSvc *ClusterService
	promSvc    *PrometheusService
	monCfgSvc  *MonitoringConfigService
}

// NewResourceService 建立服務
func NewResourceService(db *gorm.DB, k8sMgr K8sInformerManager, clusterSvc *ClusterService, promSvc *PrometheusService, monCfgSvc *MonitoringConfigService) *ResourceService {
	return &ResourceService{db: db, k8sMgr: k8sMgr, clusterSvc: clusterSvc, promSvc: promSvc, monCfgSvc: monCfgSvc}
}

// GetSnapshot 取得叢集即時資源佔用快照
func (s *ResourceService) GetSnapshot(cluster *models.Cluster) (*ClusterResourceSnapshot, error) {
	if err := s.k8sMgr.EnsureSync(context.Background(), cluster, 5*time.Second); err != nil {
		return nil, err
	}

	alloc, nodeCount := s.sumAllocatable(cluster.ID)
	req, podCount := s.sumRequests(cluster.ID)

	occupancyCPU := 0.0
	if alloc.CPUMillicores > 0 {
		occupancyCPU = req.CPUMillicores / alloc.CPUMillicores * 100
	}
	occupancyMem := 0.0
	if alloc.MemoryMiB > 0 {
		occupancyMem = req.MemoryMiB / alloc.MemoryMiB * 100
	}

	return &ClusterResourceSnapshot{
		ClusterID:   cluster.ID,
		CollectedAt: time.Now(),
		Allocatable: alloc,
		Requested:   req,
		Occupancy:   OccupancyPercent{CPU: occupancyCPU, Memory: occupancyMem},
		Headroom: ResourceMetrics{
			CPUMillicores: alloc.CPUMillicores - req.CPUMillicores,
			MemoryMiB:     alloc.MemoryMiB - req.MemoryMiB,
		},
		NodeCount: nodeCount,
		PodCount:  podCount,
	}, nil
}

// GetNamespaceOccupancy 取得各命名空間資源佔用明細（相對叢集 Allocatable）
func (s *ResourceService) GetNamespaceOccupancy(cluster *models.Cluster) ([]NamespaceOccupancy, error) {
	if err := s.k8sMgr.EnsureSync(context.Background(), cluster, 5*time.Second); err != nil {
		return nil, err
	}

	alloc, _ := s.sumAllocatable(cluster.ID)

	pods, err := s.k8sMgr.PodsLister(cluster.ID).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	type nsAgg struct {
		cpuReq float64
		memReq float64
		pods   int
	}
	nsMap := make(map[string]*nsAgg)

	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		ns := pod.Namespace
		if _, ok := nsMap[ns]; !ok {
			nsMap[ns] = &nsAgg{}
		}
		nsMap[ns].pods++
		for _, c := range pod.Spec.Containers {
			nsMap[ns].cpuReq += float64(c.Resources.Requests.Cpu().MilliValue())
			nsMap[ns].memReq += float64(c.Resources.Requests.Memory().Value()) / 1024 / 1024
		}
	}

	result := make([]NamespaceOccupancy, 0, len(nsMap))
	for ns, agg := range nsMap {
		cpuOcc, memOcc := 0.0, 0.0
		if alloc.CPUMillicores > 0 {
			cpuOcc = agg.cpuReq / alloc.CPUMillicores * 100
		}
		if alloc.MemoryMiB > 0 {
			memOcc = agg.memReq / alloc.MemoryMiB * 100
		}
		result = append(result, NamespaceOccupancy{
			Namespace:       ns,
			CPURequest:      agg.cpuReq,
			MemoryRequest:   agg.memReq,
			CPUOccupancy:    cpuOcc,
			MemoryOccupancy: memOcc,
			PodCount:        agg.pods,
		})
	}

	// 按 CPU 佔用排序（高到低）
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].CPURequest > result[j-1].CPURequest; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}

	return result, nil
}

// GetGlobalOverview 取得跨叢集全平台資源彙總
func (s *ResourceService) GetGlobalOverview(ctx context.Context) (*GlobalResourceOverview, error) {
	clusters, err := s.clusterSvc.GetAllClusters(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]ClusterResourceSummary, 0, len(clusters))
	totalCPUOcc := 0.0
	totalMemOcc := 0.0
	readyCount := 0

	for _, cluster := range clusters {
		snap, err := s.GetSnapshot(cluster)
		if err != nil {
			// Informer 未就緒，回傳零值摘要
			summaries = append(summaries, ClusterResourceSummary{
				ClusterID:     cluster.ID,
				ClusterName:   cluster.Name,
				InformerReady: false,
			})
			continue
		}
		readyCount++
		totalCPUOcc += snap.Occupancy.CPU
		totalMemOcc += snap.Occupancy.Memory
		summaries = append(summaries, ClusterResourceSummary{
			ClusterID:         cluster.ID,
			ClusterName:       cluster.Name,
			CPUOccupancy:      snap.Occupancy.CPU,
			MemoryOccupancy:   snap.Occupancy.Memory,
			AllocatableCPU:    snap.Allocatable.CPUMillicores,
			AllocatableMemory: snap.Allocatable.MemoryMiB,
			RequestedCPU:      snap.Requested.CPUMillicores,
			RequestedMemory:   snap.Requested.MemoryMiB,
			NodeCount:         snap.NodeCount,
			PodCount:          snap.PodCount,
			InformerReady:     true,
		})
	}

	avgCPU, avgMem := 0.0, 0.0
	if readyCount > 0 {
		avgCPU = totalCPUOcc / float64(readyCount)
		avgMem = totalMemOcc / float64(readyCount)
	}

	return &GlobalResourceOverview{
		CollectedAt:     time.Now(),
		ClusterCount:    len(clusters),
		ReadyCount:      readyCount,
		AvgCPUOccupancy: avgCPU,
		AvgMemOccupancy: avgMem,
		Clusters:        summaries,
	}, nil
}

// ---- 內部輔助函式 ----

// sumAllocatable 彙總叢集所有 Ready Node 的可分配資源
func (s *ResourceService) sumAllocatable(clusterID uint) (ResourceMetrics, int) {
	nodes, err := s.k8sMgr.NodesLister(clusterID).List(labels.Everything())
	if err != nil || len(nodes) == 0 {
		return ResourceMetrics{}, 0
	}

	var cpuTotal, memTotal float64
	nodeCount := 0
	for _, node := range nodes {
		// 跳過不可調度的 Node
		if node.Spec.Unschedulable {
			continue
		}
		// 確認 Ready 條件
		ready := false
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		if !ready {
			continue
		}
		cpuTotal += float64(node.Status.Allocatable.Cpu().MilliValue())
		memTotal += float64(node.Status.Allocatable.Memory().Value()) / 1024 / 1024
		nodeCount++
	}

	return ResourceMetrics{CPUMillicores: cpuTotal, MemoryMiB: memTotal}, nodeCount
}

// sumRequests 彙總叢集所有 Running/Pending Pod 的資源 requests
func (s *ResourceService) sumRequests(clusterID uint) (ResourceMetrics, int) {
	pods, err := s.k8sMgr.PodsLister(clusterID).List(labels.Everything())
	if err != nil {
		return ResourceMetrics{}, 0
	}

	var cpuTotal, memTotal float64
	podCount := 0
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		podCount++
		for _, c := range pod.Spec.Containers {
			cpuTotal += float64(c.Resources.Requests.Cpu().MilliValue())
			memTotal += float64(c.Resources.Requests.Memory().Value()) / 1024 / 1024
		}
	}

	return ResourceMetrics{CPUMillicores: cpuTotal, MemoryMiB: memTotal}, podCount
}
