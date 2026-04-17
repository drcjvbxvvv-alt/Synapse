package services

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/metrics"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"gorm.io/gorm"
)

// ---- CostWorker ----

// CostWorker 每日資源快照後臺工作器
type CostWorker struct {
	db         *gorm.DB
	promSvc    *PrometheusService
	monitorSvc *MonitoringConfigService
	clusterSvc *ClusterService
	k8sMgr     K8sInformerManager
	stopCh     chan struct{}
	ticker     *time.Ticker
	metrics    *metrics.WorkerMetrics
}

// SetMetrics attaches Prometheus worker metrics.
func (w *CostWorker) SetMetrics(m *metrics.WorkerMetrics) { w.metrics = m }

// NewCostWorker 建立工作器
func NewCostWorker(db *gorm.DB, clusterSvc *ClusterService, k8sMgr K8sInformerManager) *CostWorker {
	return &CostWorker{
		db:         db,
		promSvc:    NewPrometheusService(),
		monitorSvc: NewMonitoringConfigService(db),
		clusterSvc: clusterSvc,
		k8sMgr:     k8sMgr,
		stopCh:     make(chan struct{}),
	}
}

// Start 啟動後臺工作器（每天 00:05 執行一次快照）
func (w *CostWorker) Start() {
	go func() {
		// 計算距離下次 00:05 UTC 的等待時間
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, time.UTC)
		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}

		logger.Info("成本快照工作器已啟動", "first_run_at", next.Format(time.RFC3339))

		timer := time.NewTimer(time.Until(next))
		defer timer.Stop()

		select {
		case <-timer.C:
		case <-w.stopCh:
			return
		}

		// 首次執行
		w.snapshot()

		// 之後每 24 小時執行
		w.ticker = time.NewTicker(24 * time.Hour)
		defer w.ticker.Stop()
		for {
			select {
			case <-w.ticker.C:
				w.snapshot()
			case <-w.stopCh:
				return
			}
		}
	}()
}

// Stop 停止工作器
func (w *CostWorker) Stop() {
	close(w.stopCh)
}

// snapshot 對所有叢集拍攝資源快照
func (w *CostWorker) snapshot() {
	var run *metrics.WorkerRun
	if w.metrics != nil {
		run = w.metrics.Start("cost")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	clusters, err := w.clusterSvc.GetAllClusters(ctx)
	if err != nil {
		logger.Error("成本快照：取得叢集列表失敗", "error", err)
		if run != nil {
			run.Done(err)
		}
		return
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)

	for _, cluster := range clusters {
		monCfg, err := w.monitorSvc.GetMonitoringConfig(cluster.ID)
		if err != nil || monCfg.Type == "disabled" {
			// 無 Prometheus → 嘗試從 K8s metrics-server 取得
			w.snapshotFromK8s(cluster.ID, today)
			continue
		}
		w.snapshotFromPrometheus(cluster.ID, today, monCfg)
	}

	if run != nil {
		run.Done(nil)
	}
}

// snapshotFromPrometheus 從 Prometheus 取得 CPU/Memory request + usage，按 namespace+workload 分組
func (w *CostWorker) snapshotFromPrometheus(clusterID uint, date time.Time, monCfg *models.MonitoringConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now().Unix()

	// CPU request（millicores）by namespace + workload
	cpuReqResp, err := w.promSvc.QueryPrometheus(ctx, monCfg, &models.MetricsQuery{
		Query: `sum(kube_pod_container_resource_requests{resource="cpu",container!=""}) by (namespace, pod) * 1000`,
		Start: now, End: now, Step: "5m",
	})
	if err != nil {
		logger.Warn("成本快照：查詢 CPU request 失敗", "cluster_id", clusterID, "error", err)
		return
	}

	// CPU usage（millicores）by namespace
	cpuUsageResp, _ := w.promSvc.QueryPrometheus(ctx, monCfg, &models.MetricsQuery{
		Query: `sum(rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])) by (namespace) * 1000`,
		Start: now, End: now, Step: "5m",
	})

	// Memory request（MiB）by namespace
	memReqResp, _ := w.promSvc.QueryPrometheus(ctx, monCfg, &models.MetricsQuery{
		Query: `sum(kube_pod_container_resource_requests{resource="memory",container!=""}) by (namespace) / 1048576`,
		Start: now, End: now, Step: "5m",
	})

	// Memory usage（MiB）by namespace
	memUsageResp, _ := w.promSvc.QueryPrometheus(ctx, monCfg, &models.MetricsQuery{
		Query: `sum(container_memory_working_set_bytes{container!="",container!="POD"}) by (namespace) / 1048576`,
		Start: now, End: now, Step: "5m",
	})

	// Pod count by namespace
	podResp, _ := w.promSvc.QueryPrometheus(ctx, monCfg, &models.MetricsQuery{
		Query: `count(kube_pod_info) by (namespace)`,
		Start: now, End: now, Step: "5m",
	})

	// 組合資料（以命名空間為 key 聚合）
	type nsData struct {
		cpuReq, cpuUsage, memReq, memUsage float64
		podCount                            int
	}
	nsMap := make(map[string]*nsData)

	extractVal := func(resp *models.MetricsResponse) map[string]float64 {
		out := make(map[string]float64)
		if resp == nil {
			return out
		}
		for _, r := range resp.Data.Result {
			ns, _ := r.Metric["namespace"]
			if len(r.Values) == 0 {
				continue
			}
			var val float64
			fmt.Sscanf(fmt.Sprintf("%v", r.Values[0][1]), "%f", &val)
			out[ns] += val
		}
		return out
	}

	cpuReqs := extractVal(cpuReqResp)
	cpuUsages := extractVal(cpuUsageResp)
	memReqs := extractVal(memReqResp)
	memUsages := extractVal(memUsageResp)
	pods := extractVal(podResp)

	for ns, req := range cpuReqs {
		if _, ok := nsMap[ns]; !ok {
			nsMap[ns] = &nsData{}
		}
		nsMap[ns].cpuReq += req
	}
	for ns, v := range cpuUsages {
		if _, ok := nsMap[ns]; !ok {
			nsMap[ns] = &nsData{}
		}
		nsMap[ns].cpuUsage = v
	}
	for ns, v := range memReqs {
		if _, ok := nsMap[ns]; !ok {
			nsMap[ns] = &nsData{}
		}
		nsMap[ns].memReq = v
	}
	for ns, v := range memUsages {
		if _, ok := nsMap[ns]; !ok {
			nsMap[ns] = &nsData{}
		}
		nsMap[ns].memUsage = v
	}
	for ns, v := range pods {
		if _, ok := nsMap[ns]; !ok {
			nsMap[ns] = &nsData{}
		}
		nsMap[ns].podCount = int(v)
	}

	for ns, d := range nsMap {
		snap := &models.ResourceSnapshot{
			ClusterID:  clusterID,
			Namespace:  ns,
			Workload:   "_namespace_total_",
			Date:       date,
			CpuRequest: d.cpuReq,
			CpuUsage:   d.cpuUsage,
			MemRequest: d.memReq,
			MemUsage:   d.memUsage,
			PodCount:   d.podCount,
		}
		w.upsertSnapshot(snap)
	}

	// 同時儲存叢集級別佔用快照（K8s Informer）
	w.snapshotClusterOccupancy(clusterID, date)

	logger.Info("成本快照完成（Prometheus）", "cluster_id", clusterID, "namespaces", len(nsMap))
}

// snapshotFromK8s 無 Prometheus 時，從 K8s Informer 取得 resource requests（usage 保持 0）
func (w *CostWorker) snapshotFromK8s(clusterID uint, date time.Time) {
	if w.k8sMgr == nil {
		logger.Debug("成本快照：k8sMgr 未初始化，跳過 K8s 快照", "cluster_id", clusterID)
		return
	}

	// 取得 Pods（使用本地 Informer 快取，不直接呼叫 API Server）
	podLister := w.k8sMgr.PodsLister(clusterID)
	if podLister == nil {
		logger.Debug("成本快照：叢集 Informer 未就緒，跳過 K8s 快照", "cluster_id", clusterID)
		return
	}
	pods, err := podLister.List(labels.Everything())
	if err != nil {
		logger.Warn("成本快照：取得 Pod 列表失敗", "cluster_id", clusterID, "error", err)
		return
	}

	// 按 Namespace 彙總 requests
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

	for ns, agg := range nsMap {
		snap := &models.ResourceSnapshot{
			ClusterID:  clusterID,
			Namespace:  ns,
			Workload:   "_namespace_total_",
			Date:       date,
			CpuRequest: agg.cpuReq,
			CpuUsage:   0, // 無 Prometheus，usage 保持 0
			MemRequest: agg.memReq,
			MemUsage:   0,
			PodCount:   agg.pods,
		}
		w.upsertSnapshot(snap)
	}

	// 同時儲存叢集級別佔用快照
	w.snapshotClusterOccupancy(clusterID, date)

	logger.Info("成本快照完成（K8s API）", "cluster_id", clusterID, "namespaces", len(nsMap))
}

// snapshotClusterOccupancy 儲存叢集級別佔用快照到 cluster_occupancy_snapshots
func (w *CostWorker) snapshotClusterOccupancy(clusterID uint, date time.Time) {
	nodeLister := w.k8sMgr.NodesLister(clusterID)
	podLister := w.k8sMgr.PodsLister(clusterID)
	if nodeLister == nil || podLister == nil {
		return
	}

	nodes, _ := nodeLister.List(labels.Everything())
	pods, _ := podLister.List(labels.Everything())

	var allocCPU, allocMem float64
	nodeCount := 0
	for _, node := range nodes {
		if node.Spec.Unschedulable {
			continue
		}
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				allocCPU += float64(node.Status.Allocatable.Cpu().MilliValue())
				allocMem += float64(node.Status.Allocatable.Memory().Value()) / 1024 / 1024
				nodeCount++
				break
			}
		}
	}

	var reqCPU, reqMem float64
	podCount := 0
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		podCount++
		for _, c := range pod.Spec.Containers {
			reqCPU += float64(c.Resources.Requests.Cpu().MilliValue())
			reqMem += float64(c.Resources.Requests.Memory().Value()) / 1024 / 1024
		}
	}

	snap := &models.ClusterOccupancySnapshot{
		ClusterID:         clusterID,
		Date:              date,
		AllocatableCPU:    allocCPU,
		AllocatableMemory: allocMem,
		RequestedCPU:      reqCPU,
		RequestedMemory:   reqMem,
		NodeCount:         nodeCount,
		PodCount:          podCount,
	}
	w.db.Where("cluster_id = ? AND date = ?", clusterID, date).
		Assign(*snap).FirstOrCreate(snap)
}

// upsertSnapshot 插入快照（同一叢集 + 命名空間 + 日期只保留一筆）
func (w *CostWorker) upsertSnapshot(snap *models.ResourceSnapshot) {
	err := w.db.Where("cluster_id = ? AND namespace = ? AND workload = ? AND date = ?",
		snap.ClusterID, snap.Namespace, snap.Workload, snap.Date).
		Assign(*snap).FirstOrCreate(snap).Error
	if err != nil {
		logger.Error("成本快照：儲存快照失敗", "error", err)
	}
}
