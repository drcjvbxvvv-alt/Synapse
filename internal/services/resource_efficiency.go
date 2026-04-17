package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shaia/Synapse/internal/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

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

// WorkloadEfficiencyPage 分頁回應
type WorkloadEfficiencyPage struct {
	Items []WorkloadEfficiency `json:"items"`
	Total int                  `json:"total"`
}

// ---- Prometheus helpers ----

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

// ---- Public efficiency/waste methods ----

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
