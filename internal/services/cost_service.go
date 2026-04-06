package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---- CostService ----

// CostService 資源成本分析服務
type CostService struct {
	db         *gorm.DB
	promSvc    *PrometheusService
	monitorSvc *MonitoringConfigService
}

// NewCostService 建立服務
func NewCostService(db *gorm.DB) *CostService {
	return &CostService{
		db:         db,
		promSvc:    NewPrometheusService(),
		monitorSvc: NewMonitoringConfigService(db),
	}
}

// GetConfig 取得定價設定（不存在時回傳預設值）
func (s *CostService) GetConfig(clusterID uint) (*models.CostConfig, error) {
	var cfg models.CostConfig
	err := s.db.Where("cluster_id = ?", clusterID).First(&cfg).Error
	if err == gorm.ErrRecordNotFound {
		return &models.CostConfig{
			ClusterID:       clusterID,
			CpuPricePerCore: 0.048,
			MemPricePerGiB:  0.006,
			Currency:        "USD",
		}, nil
	}
	return &cfg, err
}

// UpsertConfig 新增或更新定價設定
func (s *CostService) UpsertConfig(cfg *models.CostConfig) error {
	return s.db.Where("cluster_id = ?", cfg.ClusterID).
		Assign(models.CostConfig{
			CpuPricePerCore: cfg.CpuPricePerCore,
			MemPricePerGiB:  cfg.MemPricePerGiB,
			Currency:        cfg.Currency,
			UpdatedAt:       time.Now(),
		}).
		FirstOrCreate(cfg).Error
}

// ---- 查詢方法 ----

// CostItem 成本條目（命名空間或工作負載）
type CostItem struct {
	Name        string  `json:"name"`
	CpuRequest  float64 `json:"cpu_request"`  // millicores
	CpuUsage    float64 `json:"cpu_usage"`    // millicores
	CpuUtil     float64 `json:"cpu_util"`     // 0–100%
	MemRequest  float64 `json:"mem_request"`  // MiB
	MemUsage    float64 `json:"mem_usage"`    // MiB
	MemUtil     float64 `json:"mem_util"`     // 0–100%
	PodCount    int     `json:"pod_count"`
	EstCost     float64 `json:"est_cost"`     // 估算費用（依定價計算）
	Currency    string  `json:"currency"`
}

// CostOverview 叢整合本總覽
type CostOverview struct {
	Month         string     `json:"month"`          // "2026-04"
	TotalCost     float64    `json:"total_cost"`
	Currency      string     `json:"currency"`
	TopNamespace  string     `json:"top_namespace"`
	WastePercent  float64    `json:"waste_percent"`
	SnapshotCount int        `json:"snapshot_count"`
	Config        *models.CostConfig `json:"config"`
}

// TrendPoint 月度趨勢資料點
type TrendPoint struct {
	Month     string             `json:"month"`
	Breakdown []NamespaceCostRow `json:"breakdown"`
	Total     float64            `json:"total"`
}

// NamespaceCostRow 命名空間費用列
type NamespaceCostRow struct {
	Namespace string  `json:"namespace"`
	Cost      float64 `json:"cost"`
}

// WasteItem 浪費識別條目
type WasteItem struct {
	Namespace   string  `json:"namespace"`
	Workload    string  `json:"workload"`
	CpuRequest  float64 `json:"cpu_request"`
	CpuAvgUsage float64 `json:"cpu_avg_usage"`
	CpuUtil     float64 `json:"cpu_util"`
	MemRequest  float64 `json:"mem_request"`
	MemAvgUsage float64 `json:"mem_avg_usage"`
	MemUtil     float64 `json:"mem_util"`
	WastedCost  float64 `json:"wasted_cost"`
	Currency    string  `json:"currency"`
	Days        int     `json:"days"`
}

// calcCost 依定價計算每日費用（CPU millicores + MiB → USD）
func calcCost(cfg *models.CostConfig, cpuMillicores, memMiB float64) float64 {
	// 1 core = 1000 millicores；1 GiB = 1024 MiB
	cpuHours := (cpuMillicores / 1000.0) * 24
	memGiBHours := (memMiB / 1024.0) * 24
	return cpuHours*cfg.CpuPricePerCore + memGiBHours*cfg.MemPricePerGiB
}

// GetOverview 取得本月叢整合本總覽
func (s *CostService) GetOverview(clusterID uint, month string) (*CostOverview, error) {
	cfg, err := s.GetConfig(clusterID)
	if err != nil {
		return nil, err
	}

	start, end, err := parseMonth(month)
	if err != nil {
		return nil, err
	}

	type row struct {
		Namespace  string
		CpuRequest float64
		MemRequest float64
		Days       int
	}

	var rows []struct {
		Namespace  string
		CpuRequest float64
		MemRequest float64
		Days       int
	}
	err = s.db.Raw(`
		SELECT namespace,
		       AVG(cpu_request) AS cpu_request,
		       AVG(mem_request) AS mem_request,
		       COUNT(DISTINCT date) AS days
		FROM resource_snapshots
		WHERE cluster_id = ? AND date >= ? AND date < ?
		GROUP BY namespace
	`, clusterID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var total float64
	topCost := -1.0
	topNS := ""
	for _, r := range rows {
		c := calcCost(cfg, r.CpuRequest, r.MemRequest) * float64(r.Days)
		total += c
		if c > topCost {
			topCost = c
			topNS = r.Namespace
		}
	}

	// 計算浪費比例（CPU request 使用率 < 10% 的工作負載）
	var wastedCPUReq float64
	var totalCPUReq float64
	s.db.Raw(`
		SELECT SUM(cpu_request) AS total,
		       SUM(CASE WHEN cpu_usage / NULLIF(cpu_request,0) < 0.1 THEN cpu_request ELSE 0 END) AS wasted
		FROM resource_snapshots
		WHERE cluster_id = ? AND date >= ? AND date < ?
	`, clusterID, start, end).Row().Scan(&totalCPUReq, &wastedCPUReq)

	wastePercent := 0.0
	if totalCPUReq > 0 {
		wastePercent = wastedCPUReq / totalCPUReq * 100
	}

	var snapshotCount int64
	s.db.Model(&models.ResourceSnapshot{}).
		Where("cluster_id = ? AND date >= ? AND date < ?", clusterID, start, end).
		Count(&snapshotCount)

	return &CostOverview{
		Month:         month,
		TotalCost:     total,
		Currency:      cfg.Currency,
		TopNamespace:  topNS,
		WastePercent:  wastePercent,
		SnapshotCount: int(snapshotCount),
		Config:        cfg,
	}, nil
}

// GetNamespaceCosts 取得命名空間成本排行
func (s *CostService) GetNamespaceCosts(clusterID uint, month string) ([]CostItem, error) {
	cfg, err := s.GetConfig(clusterID)
	if err != nil {
		return nil, err
	}
	start, end, err := parseMonth(month)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		Namespace  string
		CpuRequest float64
		CpuUsage   float64
		MemRequest float64
		MemUsage   float64
		PodCount   int
		Days       int
	}
	err = s.db.Raw(`
		SELECT namespace,
		       AVG(cpu_request) AS cpu_request,
		       AVG(cpu_usage)   AS cpu_usage,
		       AVG(mem_request) AS mem_request,
		       AVG(mem_usage)   AS mem_usage,
		       SUM(pod_count)   AS pod_count,
		       COUNT(DISTINCT date) AS days
		FROM resource_snapshots
		WHERE cluster_id = ? AND date >= ? AND date < ?
		GROUP BY namespace
		ORDER BY cpu_request DESC
	`, clusterID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]CostItem, 0, len(rows))
	for _, r := range rows {
		cpuUtil := 0.0
		if r.CpuRequest > 0 {
			cpuUtil = r.CpuUsage / r.CpuRequest * 100
		}
		memUtil := 0.0
		if r.MemRequest > 0 {
			memUtil = r.MemUsage / r.MemRequest * 100
		}
		items = append(items, CostItem{
			Name:       r.Namespace,
			CpuRequest: r.CpuRequest,
			CpuUsage:   r.CpuUsage,
			CpuUtil:    cpuUtil,
			MemRequest: r.MemRequest,
			MemUsage:   r.MemUsage,
			MemUtil:    memUtil,
			PodCount:   r.PodCount,
			EstCost:    calcCost(cfg, r.CpuRequest, r.MemRequest) * float64(r.Days),
			Currency:   cfg.Currency,
		})
	}
	return items, nil
}

// GetWorkloadCosts 取得工作負載成本明細（支援命名空間篩選 + 分頁）
func (s *CostService) GetWorkloadCosts(clusterID uint, month, namespace string, page, pageSize int) ([]CostItem, int64, error) {
	cfg, err := s.GetConfig(clusterID)
	if err != nil {
		return nil, 0, err
	}
	start, end, err := parseMonth(month)
	if err != nil {
		return nil, 0, err
	}

	q := s.db.Model(&models.ResourceSnapshot{}).
		Select(`namespace, workload,
			AVG(cpu_request) AS cpu_request,
			AVG(cpu_usage)   AS cpu_usage,
			AVG(mem_request) AS mem_request,
			AVG(mem_usage)   AS mem_usage,
			SUM(pod_count)   AS pod_count,
			COUNT(DISTINCT date) AS days`).
		Where("cluster_id = ? AND date >= ? AND date < ?", clusterID, start, end).
		Group("namespace, workload")

	if namespace != "" && namespace != "_all_" {
		q = q.Where("namespace = ?", namespace)
	}

	var total int64
	s.db.Raw("SELECT COUNT(*) FROM (?) AS sub", q).Scan(&total)

	offset := (page - 1) * pageSize
	var rows []struct {
		Namespace  string
		Workload   string
		CpuRequest float64
		CpuUsage   float64
		MemRequest float64
		MemUsage   float64
		PodCount   int
		Days       int
	}
	err = q.Order("cpu_request DESC").Offset(offset).Limit(pageSize).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]CostItem, 0, len(rows))
	for _, r := range rows {
		cpuUtil, memUtil := 0.0, 0.0
		if r.CpuRequest > 0 {
			cpuUtil = r.CpuUsage / r.CpuRequest * 100
		}
		if r.MemRequest > 0 {
			memUtil = r.MemUsage / r.MemRequest * 100
		}
		items = append(items, CostItem{
			Name:       fmt.Sprintf("%s / %s", r.Namespace, r.Workload),
			CpuRequest: r.CpuRequest,
			CpuUsage:   r.CpuUsage,
			CpuUtil:    cpuUtil,
			MemRequest: r.MemRequest,
			MemUsage:   r.MemUsage,
			MemUtil:    memUtil,
			PodCount:   r.PodCount,
			EstCost:    calcCost(cfg, r.CpuRequest, r.MemRequest) * float64(r.Days),
			Currency:   cfg.Currency,
		})
	}
	return items, total, nil
}

// GetTrend 取得最近 N 個月成本趨勢（按命名空間分組）
func (s *CostService) GetTrend(clusterID uint, months int) ([]TrendPoint, error) {
	cfg, err := s.GetConfig(clusterID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var points []TrendPoint

	for i := months - 1; i >= 0; i-- {
		t := now.AddDate(0, -i, 0)
		month := t.Format("2006-01")
		start, end, _ := parseMonth(month)

		var rows []struct {
			Namespace  string
			CpuRequest float64
			MemRequest float64
			Days       int
		}
		s.db.Raw(`
			SELECT namespace,
			       AVG(cpu_request) AS cpu_request,
			       AVG(mem_request) AS mem_request,
			       COUNT(DISTINCT date) AS days
			FROM resource_snapshots
			WHERE cluster_id = ? AND date >= ? AND date < ?
			GROUP BY namespace
		`, clusterID, start, end).Scan(&rows)

		var breakdown []NamespaceCostRow
		total := 0.0
		for _, r := range rows {
			c := calcCost(cfg, r.CpuRequest, r.MemRequest) * float64(r.Days)
			total += c
			breakdown = append(breakdown, NamespaceCostRow{Namespace: r.Namespace, Cost: c})
		}
		points = append(points, TrendPoint{Month: month, Breakdown: breakdown, Total: total})
	}
	return points, nil
}

// GetWaste 取得資源浪費識別報告（CPU 使用率持續 < 10% 的工作負載）
func (s *CostService) GetWaste(clusterID uint) ([]WasteItem, error) {
	cfg, err := s.GetConfig(clusterID)
	if err != nil {
		return nil, err
	}

	// 取最近 7 天快照
	since := time.Now().AddDate(0, 0, -7).UTC().Truncate(24 * time.Hour)
	var rows []struct {
		Namespace   string
		Workload    string
		CpuRequest  float64
		CpuAvgUsage float64
		MemRequest  float64
		MemAvgUsage float64
		Days        int
	}
	err = s.db.Raw(`
		SELECT namespace, workload,
		       AVG(cpu_request)  AS cpu_request,
		       AVG(cpu_usage)    AS cpu_avg_usage,
		       AVG(mem_request)  AS mem_request,
		       AVG(mem_usage)    AS mem_avg_usage,
		       COUNT(DISTINCT date) AS days
		FROM resource_snapshots
		WHERE cluster_id = ? AND date >= ? AND cpu_request > 0
		GROUP BY namespace, workload
		HAVING AVG(cpu_usage) / AVG(cpu_request) < 0.1
		ORDER BY cpu_request DESC
	`, clusterID, since).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]WasteItem, 0, len(rows))
	for _, r := range rows {
		cpuUtil, memUtil := 0.0, 0.0
		if r.CpuRequest > 0 {
			cpuUtil = r.CpuAvgUsage / r.CpuRequest * 100
		}
		if r.MemRequest > 0 {
			memUtil = r.MemAvgUsage / r.MemRequest * 100
		}
		wasted := calcCost(cfg, r.CpuRequest-r.CpuAvgUsage, 0) * float64(r.Days)
		items = append(items, WasteItem{
			Namespace:   r.Namespace,
			Workload:    r.Workload,
			CpuRequest:  r.CpuRequest,
			CpuAvgUsage: r.CpuAvgUsage,
			CpuUtil:     cpuUtil,
			MemRequest:  r.MemRequest,
			MemAvgUsage: r.MemAvgUsage,
			MemUtil:     memUtil,
			WastedCost:  wasted,
			Currency:    cfg.Currency,
			Days:        r.Days,
		})
	}
	return items, nil
}

// ExportCSV 匯出指定月份工作負載成本為 CSV（回傳 bytes）
func (s *CostService) ExportCSV(clusterID uint, month string) ([]byte, error) {
	items, _, err := s.GetWorkloadCosts(clusterID, month, "", 1, 10000)
	if err != nil {
		return nil, err
	}
	cfg, _ := s.GetConfig(clusterID)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Namespace/Workload", "CPU Request(m)", "CPU Usage(m)", "CPU Util%",
		"Mem Request(MiB)", "Mem Usage(MiB)", "Mem Util%", "Pod Count",
		fmt.Sprintf("Est Cost(%s)", cfg.Currency)})
	for _, item := range items {
		_ = w.Write([]string{
			item.Name,
			strconv.FormatFloat(item.CpuRequest, 'f', 2, 64),
			strconv.FormatFloat(item.CpuUsage, 'f', 2, 64),
			strconv.FormatFloat(item.CpuUtil, 'f', 1, 64),
			strconv.FormatFloat(item.MemRequest, 'f', 2, 64),
			strconv.FormatFloat(item.MemUsage, 'f', 2, 64),
			strconv.FormatFloat(item.MemUtil, 'f', 1, 64),
			strconv.Itoa(item.PodCount),
			strconv.FormatFloat(item.EstCost, 'f', 4, 64),
		})
	}
	w.Flush()
	return buf.Bytes(), nil
}

// ---- CostWorker ----

// CostWorker 每日資源快照後臺工作器
type CostWorker struct {
	db         *gorm.DB
	promSvc    *PrometheusService
	monitorSvc *MonitoringConfigService
	clusterSvc *ClusterService
	stopCh     chan struct{}
	ticker     *time.Ticker
}

// NewCostWorker 建立工作器
func NewCostWorker(db *gorm.DB, clusterSvc *ClusterService) *CostWorker {
	return &CostWorker{
		db:         db,
		promSvc:    NewPrometheusService(),
		monitorSvc: NewMonitoringConfigService(db),
		clusterSvc: clusterSvc,
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
	clusters, err := w.clusterSvc.GetAllClusters()
	if err != nil {
		logger.Error("成本快照：取得叢集列表失敗", "error", err)
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

	logger.Info("成本快照完成（Prometheus）", "cluster_id", clusterID, "namespaces", len(nsMap))
}

// snapshotFromK8s 無 Prometheus 時，從 K8s API 取得 resource requests（無 usage）
func (w *CostWorker) snapshotFromK8s(clusterID uint, date time.Time) {
	// 僅記錄 request，usage 保持 0（在告警中顯示 N/A）
	// 實際實作需要 metrics-server API；此處為預留 placeholder
	logger.Debug("成本快照：叢集無 Prometheus 設定，跳過快照", "cluster_id", clusterID)
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

// ---- 工具函式 ----

// parseMonth 將 "2026-04" 解析為月份起訖時間（UTC）
func parseMonth(month string) (start, end time.Time, err error) {
	if month == "" {
		now := time.Now().UTC()
		month = now.Format("2006-01")
	}
	start, err = time.Parse("2006-01", month)
	if err != nil {
		return
	}
	start = start.UTC()
	end = start.AddDate(0, 1, 0)
	return
}
