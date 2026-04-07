package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"
	corev1 "k8s.io/api/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ---- Phase 2 資料結構 ----

// NamespaceEfficiency 結合 K8s 佔用率 + Prometheus 效率指標
type NamespaceEfficiency struct {
	Namespace            string  `json:"namespace"`
	CPURequestMillicores float64 `json:"cpu_request_millicores"`
	MemoryRequestMiB     float64 `json:"memory_request_mib"`
	CPUUsageMillicores   float64 `json:"cpu_usage_millicores"`
	MemoryUsageMiB       float64 `json:"memory_usage_mib"`
	CPUOccupancy         float64 `json:"cpu_occupancy_percent"`
	MemoryOccupancy      float64 `json:"memory_occupancy_percent"`
	CPUEfficiency        float64 `json:"cpu_efficiency"`    // usage/request, 0-1
	MemoryEfficiency     float64 `json:"memory_efficiency"` // usage/request, 0-1
	PodCount             int     `json:"pod_count"`
	HasMetrics           bool    `json:"has_metrics"`
}

// WorkloadEfficiency 工作負載效率（含 Right-sizing 方向）
type WorkloadEfficiency struct {
	Namespace            string                     `json:"namespace"`
	Name                 string                     `json:"name"`
	Kind                 string                     `json:"kind"`
	Replicas             int32                      `json:"replicas"`
	CPURequestMillicores float64                    `json:"cpu_request_millicores"`
	CPUUsageMillicores   float64                    `json:"cpu_usage_millicores"`
	CPUEfficiency        float64                    `json:"cpu_efficiency"`
	MemoryRequestMiB     float64                    `json:"memory_request_mib"`
	MemoryUsageMiB       float64                    `json:"memory_usage_mib"`
	MemoryEfficiency     float64                    `json:"memory_efficiency"`
	WasteScore           float64                    `json:"waste_score"` // 0-1，越高越浪費
	HasMetrics           bool                       `json:"has_metrics"`
	RightSizing          *RightSizingRecommendation `json:"rightsizing,omitempty"`
}

// ---- Phase 3：趨勢、預測、Right-sizing ----

// CapacityTrendPoint 月度容量佔用趨勢資料點（來自 cluster_occupancy_snapshots）
type CapacityTrendPoint struct {
	Month              string  `json:"month"`                  // "2026-01"
	CPUOccupancyPct    float64 `json:"cpu_occupancy_percent"`
	MemoryOccupancyPct float64 `json:"memory_occupancy_percent"`
	NodeCount          int     `json:"node_count"`
}

// ForecastResult 容量耗盡預測（線性迴歸）
type ForecastResult struct {
	BasedOnMonths    int     `json:"based_on_months"`
	CurrentCPUPct    float64 `json:"current_cpu_percent"`
	CurrentMemoryPct float64 `json:"current_memory_percent"`
	CPU80PctDate     *string `json:"cpu_80_percent_date"`    // null = 預測範圍內不到達
	CPU100PctDate    *string `json:"cpu_100_percent_date"`
	Memory80PctDate  *string `json:"memory_80_percent_date"`
	Memory100PctDate *string `json:"memory_100_percent_date"`
}

// RightSizingRecommendation 工作負載 Right-sizing 建議
type RightSizingRecommendation struct {
	CPUMillicores float64 `json:"cpu_recommended_millicores"`
	MemoryMiB     float64 `json:"memory_recommended_mib"`
	Confidence    string  `json:"confidence"` // "medium"（有 Prometheus 資料時）
}

// WorkloadEfficiencyPage 分頁回應
type WorkloadEfficiencyPage struct {
	Items []WorkloadEfficiency `json:"items"`
	Total int                  `json:"total"`
}

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
func (s *ResourceService) GetGlobalOverview() (*GlobalResourceOverview, error) {
	clusters, err := s.clusterSvc.GetAllClusters()
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

// ---- Phase 2：效率分析 ----

// promInstantQuery 執行 Prometheus 即時查詢（5 分鐘視窗）
func (s *ResourceService) promInstantQuery(ctx context.Context, monCfg *models.MonitoringConfig, query string) (*models.MetricsResponse, error) {
	now := time.Now().Unix()
	return s.promSvc.QueryPrometheus(ctx, monCfg, &models.MetricsQuery{
		Query: query,
		Start: now - 300,
		End:   now,
		Step:  "60",
	})
}

// parseSeriesByLabel 解析多 series Prometheus 回應，以指定 label 為 key 彙總值
func parseSeriesByLabel(resp *models.MetricsResponse, labelName string) map[string]float64 {
	m := make(map[string]float64)
	if resp == nil {
		return m
	}
	for _, r := range resp.Data.Result {
		key := r.Metric[labelName]
		if key == "" {
			continue
		}
		m[key] += extractPromValue(r)
	}
	return m
}

// parseSeriesByNSPod 解析 (namespace, pod) 雙 label 的 Prometheus 回應，key 格式 "namespace/pod"
func parseSeriesByNSPod(resp *models.MetricsResponse) map[string]float64 {
	m := make(map[string]float64)
	if resp == nil {
		return m
	}
	for _, r := range resp.Data.Result {
		ns := r.Metric["namespace"]
		pod := r.Metric["pod"]
		if ns == "" || pod == "" {
			continue
		}
		m[ns+"/"+pod] += extractPromValue(r)
	}
	return m
}

// extractPromValue 從單一 MetricsResult 中取出數值
func extractPromValue(r models.MetricsResult) float64 {
	if len(r.Values) > 0 {
		last := r.Values[len(r.Values)-1]
		if len(last) >= 2 {
			if s, ok := last[1].(string); ok {
				var f float64
				fmt.Sscanf(s, "%f", &f)
				return f
			}
		}
	}
	if len(r.Value) >= 2 {
		if s, ok := r.Value[1].(string); ok {
			var f float64
			fmt.Sscanf(s, "%f", &f)
			return f
		}
	}
	return 0
}

// GetNamespaceEfficiency 取得各命名空間效率分析（K8s 佔用 + Prometheus 使用量）
func (s *ResourceService) GetNamespaceEfficiency(cluster *models.Cluster) ([]NamespaceEfficiency, error) {
	occupancies, err := s.GetNamespaceOccupancy(cluster)
	if err != nil {
		return nil, err
	}

	result := make([]NamespaceEfficiency, len(occupancies))
	for i, occ := range occupancies {
		result[i] = NamespaceEfficiency{
			Namespace:            occ.Namespace,
			CPURequestMillicores: occ.CPURequest,
			MemoryRequestMiB:     occ.MemoryRequest,
			CPUOccupancy:         occ.CPUOccupancy,
			MemoryOccupancy:      occ.MemoryOccupancy,
			PodCount:             occ.PodCount,
		}
	}

	monCfg, err := s.monCfgSvc.GetMonitoringConfig(cluster.ID)
	if err != nil || monCfg.Type == "disabled" {
		return result, nil
	}

	ctx := context.Background()
	// CPU 使用量（millicores）：rate × 1000 轉換
	cpuResp, cpuErr := s.promInstantQuery(ctx, monCfg,
		`sum by (namespace) (rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])) * 1000`)
	cpuMap := parseSeriesByLabel(cpuResp, "namespace")

	// 記憶體使用量（MiB）
	memResp, _ := s.promInstantQuery(ctx, monCfg,
		`sum by (namespace) (container_memory_working_set_bytes{container!="",container!="POD"}) / 1048576`)
	memMap := parseSeriesByLabel(memResp, "namespace")

	hasMetrics := cpuErr == nil
	for i := range result {
		ns := result[i].Namespace
		result[i].CPUUsageMillicores = cpuMap[ns]
		result[i].MemoryUsageMiB = memMap[ns]
		result[i].HasMetrics = hasMetrics
		if result[i].CPURequestMillicores > 0 {
			result[i].CPUEfficiency = result[i].CPUUsageMillicores / result[i].CPURequestMillicores
		}
		if result[i].MemoryRequestMiB > 0 {
			result[i].MemoryEfficiency = result[i].MemoryUsageMiB / result[i].MemoryRequestMiB
		}
	}
	return result, nil
}

// GetWorkloadEfficiency 取得工作負載效率（Deployment/StatefulSet/DaemonSet），支援分頁與 namespace 篩選
func (s *ResourceService) GetWorkloadEfficiency(cluster *models.Cluster, namespace string, page, pageSize int) (*WorkloadEfficiencyPage, error) {
	if err := s.k8sMgr.EnsureSync(context.Background(), cluster, 5*time.Second); err != nil {
		return nil, err
	}
	clusterID := cluster.ID
	podLister := s.k8sMgr.PodsLister(clusterID)

	type wlData struct {
		WorkloadEfficiency
		podKeys []string // "namespace/pod" for Prometheus lookup
	}
	var data []wlData

	addWorkload := func(ns, name, kind string, replicas int32, selector *metav1.LabelSelector) {
		sel, err := metav1.LabelSelectorAsSelector(selector)
		if err != nil {
			return
		}
		pods, err := podLister.Pods(ns).List(sel)
		if err != nil {
			return
		}
		var cpuReq, memReq float64
		podKeys := make([]string, 0, len(pods))
		for _, pod := range pods {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}
			podKeys = append(podKeys, ns+"/"+pod.Name)
			for _, c := range pod.Spec.Containers {
				cpuReq += float64(c.Resources.Requests.Cpu().MilliValue())
				memReq += float64(c.Resources.Requests.Memory().Value()) / 1024 / 1024
			}
		}
		data = append(data, wlData{
			WorkloadEfficiency: WorkloadEfficiency{
				Namespace:            ns,
				Name:                 name,
				Kind:                 kind,
				Replicas:             replicas,
				CPURequestMillicores: cpuReq,
				MemoryRequestMiB:     memReq,
			},
			podKeys: podKeys,
		})
	}

	// Deployments
	deploys, _ := s.k8sMgr.DeploymentsLister(clusterID).List(labels.Everything())
	for _, d := range deploys {
		if namespace != "" && d.Namespace != namespace {
			continue
		}
		replicas := int32(1)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		addWorkload(d.Namespace, d.Name, "Deployment", replicas, d.Spec.Selector)
	}

	// StatefulSets
	ssets, _ := s.k8sMgr.StatefulSetsLister(clusterID).List(labels.Everything())
	for _, ss := range ssets {
		if namespace != "" && ss.Namespace != namespace {
			continue
		}
		replicas := int32(1)
		if ss.Spec.Replicas != nil {
			replicas = *ss.Spec.Replicas
		}
		addWorkload(ss.Namespace, ss.Name, "StatefulSet", replicas, ss.Spec.Selector)
	}

	// DaemonSets
	dsets, _ := s.k8sMgr.DaemonSetsLister(clusterID).List(labels.Everything())
	for _, ds := range dsets {
		if namespace != "" && ds.Namespace != namespace {
			continue
		}
		addWorkload(ds.Namespace, ds.Name, "DaemonSet", ds.Status.NumberReady, ds.Spec.Selector)
	}

	total := len(data)

	// 查詢 Prometheus Pod 用量
	monCfg, err := s.monCfgSvc.GetMonitoringConfig(cluster.ID)
	if err == nil && monCfg.Type != "disabled" {
		ctx := context.Background()
		cpuResp, _ := s.promInstantQuery(ctx, monCfg,
			`sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])) * 1000`)
		cpuMap := parseSeriesByNSPod(cpuResp)

		memResp, _ := s.promInstantQuery(ctx, monCfg,
			`sum by (namespace, pod) (container_memory_working_set_bytes{container!="",container!="POD"}) / 1048576`)
		memMap := parseSeriesByNSPod(memResp)

		for i := range data {
			var cpuUsage, memUsage float64
			found := false
			for _, key := range data[i].podKeys {
				if v, ok := cpuMap[key]; ok {
					cpuUsage += v
					found = true
				}
				if v, ok := memMap[key]; ok {
					memUsage += v
				}
			}
			if found {
				data[i].CPUUsageMillicores = cpuUsage
				data[i].MemoryUsageMiB = memUsage
				data[i].HasMetrics = true
				if data[i].CPURequestMillicores > 0 {
					data[i].CPUEfficiency = cpuUsage / data[i].CPURequestMillicores
				}
				if data[i].MemoryRequestMiB > 0 {
					data[i].MemoryEfficiency = memUsage / data[i].MemoryRequestMiB
				}
			}
		}
	}

	// 計算廢棄分數並排序（廢棄分數高 → 低）
	for i := range data {
		if data[i].HasMetrics {
			data[i].WasteScore = (1-data[i].CPUEfficiency)*0.6 + (1-data[i].MemoryEfficiency)*0.4
			if data[i].WasteScore < 0 {
				data[i].WasteScore = 0
			}
		}
	}

	// Right-sizing：7 日最大用量 × 安全係數（需要 Prometheus）
	if monCfg, err2 := s.monCfgSvc.GetMonitoringConfig(cluster.ID); err2 == nil && monCfg.Type != "disabled" {
		ctx2 := context.Background()
		cpuMaxResp, _ := s.promInstantQuery(ctx2, monCfg,
			`max_over_time(sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m]))[7d:1h]) * 1000`)
		cpuMaxMap := parseSeriesByNSPod(cpuMaxResp)

		memMaxResp, _ := s.promInstantQuery(ctx2, monCfg,
			`max_over_time(sum by (namespace, pod) (container_memory_working_set_bytes{container!="",container!="POD"})[7d:1h]) / 1048576`)
		memMaxMap := parseSeriesByNSPod(memMaxResp)

		for i := range data {
			if !data[i].HasMetrics {
				continue
			}
			var cpuMax, memMax float64
			found := false
			for _, key := range data[i].podKeys {
				if v, ok := cpuMaxMap[key]; ok {
					cpuMax += v
					found = true
				}
				if v, ok := memMaxMap[key]; ok {
					memMax += v
				}
			}
			if found {
				recCPU := cpuMax * 1.2
				if recCPU < 10 {
					recCPU = 10
				}
				recMem := memMax * 1.25
				if recMem < 64 {
					recMem = 64
				}
				data[i].RightSizing = &RightSizingRecommendation{
					CPUMillicores: recCPU,
					MemoryMiB:     recMem,
					Confidence:    "medium",
				}
			}
		}
	}

	sort.Slice(data, func(i, j int) bool {
		// HasMetrics 優先，再按廢棄分數排
		if data[i].HasMetrics != data[j].HasMetrics {
			return data[i].HasMetrics
		}
		return data[i].WasteScore > data[j].WasteScore
	})

	// 分頁
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(data) {
		return &WorkloadEfficiencyPage{Items: []WorkloadEfficiency{}, Total: total}, nil
	}
	end := start + pageSize
	if end > len(data) {
		end = len(data)
	}
	items := make([]WorkloadEfficiency, end-start)
	for i, d := range data[start:end] {
		items[i] = d.WorkloadEfficiency
	}
	return &WorkloadEfficiencyPage{Items: items, Total: total}, nil
}

// GetWasteWorkloads 取得效率低於閾值的工作負載（全叢集掃描，不分頁）
func (s *ResourceService) GetWasteWorkloads(cluster *models.Cluster, cpuThreshold float64) ([]WorkloadEfficiency, error) {
	if cpuThreshold <= 0 {
		cpuThreshold = 0.2
	}
	page, err := s.GetWorkloadEfficiency(cluster, "", 1, 10000)
	if err != nil {
		return nil, err
	}
	result := make([]WorkloadEfficiency, 0)
	for _, wl := range page.Items {
		if wl.HasMetrics && wl.CPUEfficiency < cpuThreshold {
			result = append(result, wl)
		}
	}
	return result, nil
}

// ---- Phase 3：趨勢、預測 ----

// GetTrend 讀取 cluster_occupancy_snapshots，按月彙總佔用率趨勢
func (s *ResourceService) GetTrend(cluster *models.Cluster, months int) ([]CapacityTrendPoint, error) {
	if months <= 0 {
		months = 6
	}
	since := time.Now().AddDate(0, -months, 0)

	var snaps []models.ClusterOccupancySnapshot
	if err := s.db.Where("cluster_id = ? AND date >= ?", cluster.ID, since).
		Order("date ASC").Find(&snaps).Error; err != nil {
		return nil, err
	}

	type monthAgg struct {
		cpuPct    float64
		memPct    float64
		nodeCount int
		count     int
	}
	aggs := make(map[string]*monthAgg)
	order := make([]string, 0)

	for _, sn := range snaps {
		m := sn.Date.Format("2006-01")
		if _, ok := aggs[m]; !ok {
			aggs[m] = &monthAgg{}
			order = append(order, m)
		}
		a := aggs[m]
		a.count++
		a.nodeCount = sn.NodeCount
		if sn.AllocatableCPU > 0 {
			a.cpuPct += sn.RequestedCPU / sn.AllocatableCPU * 100
		}
		if sn.AllocatableMemory > 0 {
			a.memPct += sn.RequestedMemory / sn.AllocatableMemory * 100
		}
	}

	result := make([]CapacityTrendPoint, 0, len(order))
	for _, m := range order {
		a := aggs[m]
		if a.count == 0 {
			continue
		}
		result = append(result, CapacityTrendPoint{
			Month:              m,
			CPUOccupancyPct:    a.cpuPct / float64(a.count),
			MemoryOccupancyPct: a.memPct / float64(a.count),
			NodeCount:          a.nodeCount,
		})
	}
	return result, nil
}

// GetForecast 基於歷史快照做線性迴歸，預測容量耗盡日期
func (s *ResourceService) GetForecast(cluster *models.Cluster, days int) (*ForecastResult, error) {
	if days <= 0 {
		days = 180
	}
	trend, err := s.GetTrend(cluster, 6)
	if err != nil {
		return nil, err
	}
	n := len(trend)
	result := &ForecastResult{BasedOnMonths: n}
	if n == 0 {
		return result, nil
	}
	result.CurrentCPUPct = trend[n-1].CPUOccupancyPct
	result.CurrentMemoryPct = trend[n-1].MemoryOccupancyPct
	if n < 2 {
		return result, nil
	}

	cpuSlope, cpuIntercept := linReg(n, func(i int) float64 { return trend[i].CPUOccupancyPct })
	memSlope, memIntercept := linReg(n, func(i int) float64 { return trend[i].MemoryOccupancyPct })

	lastMonth, _ := time.Parse("2006-01", trend[n-1].Month)
	currentIdx := float64(n - 1)
	maxMonths := float64(days) / 30.0

	predictDate := func(slope, intercept, target float64) *string {
		if slope <= 0 {
			return nil
		}
		xTarget := (target - intercept) / slope
		monthsAhead := xTarget - currentIdx
		if monthsAhead < 0 || monthsAhead > maxMonths {
			return nil
		}
		t := lastMonth.AddDate(0, int(monthsAhead+0.5), 0)
		dateStr := t.Format("2006-01-02")
		return &dateStr
	}

	result.CPU80PctDate = predictDate(cpuSlope, cpuIntercept, 80)
	result.CPU100PctDate = predictDate(cpuSlope, cpuIntercept, 100)
	result.Memory80PctDate = predictDate(memSlope, memIntercept, 80)
	result.Memory100PctDate = predictDate(memSlope, memIntercept, 100)
	return result, nil
}

// linReg 對 n 個點做最小二乘線性迴歸，x = index（0..n-1）
func linReg(n int, getY func(int) float64) (slope, intercept float64) {
	fn := float64(n)
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		x, y := float64(i), getY(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := fn*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / fn
	}
	slope = (fn*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / fn
	return
}
