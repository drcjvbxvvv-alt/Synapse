package services

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"github.com/shaia/Synapse/internal/metrics"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"gorm.io/gorm"
)

// ImageIndexWorker 每小時增量掃描所有叢集工作負載映像並更新索引。
// 使用 Informer Lister（快取）取代直接 K8s API 呼叫，降低 API Server 壓力。
// 增量策略：以 ResourceVersion 判斷工作負載是否變動，只更新有差異的條目。
type ImageIndexWorker struct {
	db         *gorm.DB
	clusterSvc *ClusterService
	k8sMgr     K8sInformerManager
	stopCh     chan struct{}
	metrics    *metrics.WorkerMetrics

	// rvCache: clusterID → kind → "namespace/name" → resourceVersion
	// 用於判斷工作負載是否自上次同步後有變動
	rvCache map[uint]map[string]map[string]string
	mu      sync.Mutex
}

// NewImageIndexWorker 建立映像索引 Worker
func NewImageIndexWorker(db *gorm.DB, clusterSvc *ClusterService, k8sMgr K8sInformerManager) *ImageIndexWorker {
	return &ImageIndexWorker{
		db:         db,
		clusterSvc: clusterSvc,
		k8sMgr:     k8sMgr,
		stopCh:     make(chan struct{}),
		rvCache:    make(map[uint]map[string]map[string]string),
	}
}

// SetMetrics 注入 Prometheus worker 指標（可選）
func (w *ImageIndexWorker) SetMetrics(m *metrics.WorkerMetrics) { w.metrics = m }

// Start 啟動後台 Worker：立即執行一次，之後每小時執行
func (w *ImageIndexWorker) Start() {
	go func() {
		logger.Info("映像索引 Worker 已啟動，立即執行首次同步")
		w.sync()

		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.sync()
			case <-w.stopCh:
				logger.Info("映像索引 Worker 已停止")
				return
			}
		}
	}()
}

// Stop 停止 Worker
func (w *ImageIndexWorker) Stop() {
	close(w.stopCh)
}

// sync 掃描所有叢集並增量更新映像索引
func (w *ImageIndexWorker) sync() {
	var run *metrics.WorkerRun
	if w.metrics != nil {
		run = w.metrics.Start("image_index")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	clusters, err := w.clusterSvc.GetAllClusters(ctx)
	if err != nil {
		logger.Warn("映像索引 Worker：取得叢集列表失敗", "err", err)
		if run != nil {
			run.Done(err)
		}
		return
	}

	now := time.Now()
	totalIndexed := 0

	for _, cluster := range clusters {
		n, err := w.syncCluster(cluster, now)
		if err != nil {
			logger.Warn("映像索引 Worker：叢集同步失敗", "cluster", cluster.Name, "err", err)
			continue
		}
		totalIndexed += n
	}

	logger.Info("映像索引 Worker 同步完成", "clusters", len(clusters), "indexed", totalIndexed)
	if run != nil {
		run.Done(nil)
	}
}

// syncCluster 增量同步單一叢集，回傳本次更新條目數
func (w *ImageIndexWorker) syncCluster(cluster *models.Cluster, now time.Time) (int, error) {
	clusterID := cluster.ID

	// 確保 Informer 快取已同步（最多等 30s）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := w.k8sMgr.EnsureSync(ctx, cluster, 30*time.Second); err != nil {
		return 0, err
	}

	w.mu.Lock()
	if w.rvCache[clusterID] == nil {
		w.rvCache[clusterID] = map[string]map[string]string{
			"Deployment":  {},
			"StatefulSet": {},
			"DaemonSet":   {},
		}
	}
	w.mu.Unlock()

	total := 0
	total += w.processDeployments(cluster, now)
	total += w.processStatefulSets(cluster, now)
	total += w.processDaemonSets(cluster, now)
	return total, nil
}

// ── Deployment ──────────────────────────────────────────────────────────────

func (w *ImageIndexWorker) processDeployments(cluster *models.Cluster, now time.Time) int {
	lister := w.k8sMgr.DeploymentsLister(cluster.ID)
	if lister == nil {
		return 0
	}
	deps, err := lister.List(labels.Everything())
	if err != nil {
		return 0
	}

	w.mu.Lock()
	rvMap := w.rvCache[cluster.ID]["Deployment"]
	w.mu.Unlock()

	currentKeys := make(map[string]bool, len(deps))
	updated := 0

	for _, d := range deps {
		key := d.Namespace + "/" + d.Name
		currentKeys[key] = true

		rv := d.ResourceVersion
		if rvMap[key] == rv {
			continue // 未變動，跳過
		}

		// 更新此 Deployment 的索引條目
		entries := deploymentsToEntries(d, cluster, now)
		if err := w.upsertWorkload(cluster.ID, "Deployment", d.Namespace, d.Name, entries); err == nil {
			rvMap[key] = rv
			updated += len(entries)
		}
	}

	// 刪除已不存在的 Deployment 索引
	w.deleteStale(cluster.ID, "Deployment", currentKeys, rvMap)
	return updated
}

// ── StatefulSet ─────────────────────────────────────────────────────────────

func (w *ImageIndexWorker) processStatefulSets(cluster *models.Cluster, now time.Time) int {
	lister := w.k8sMgr.StatefulSetsLister(cluster.ID)
	if lister == nil {
		return 0
	}
	sss, err := lister.List(labels.Everything())
	if err != nil {
		return 0
	}

	w.mu.Lock()
	rvMap := w.rvCache[cluster.ID]["StatefulSet"]
	w.mu.Unlock()

	currentKeys := make(map[string]bool, len(sss))
	updated := 0

	for _, ss := range sss {
		key := ss.Namespace + "/" + ss.Name
		currentKeys[key] = true

		rv := ss.ResourceVersion
		if rvMap[key] == rv {
			continue
		}

		entries := statefulSetsToEntries(ss, cluster, now)
		if err := w.upsertWorkload(cluster.ID, "StatefulSet", ss.Namespace, ss.Name, entries); err == nil {
			rvMap[key] = rv
			updated += len(entries)
		}
	}

	w.deleteStale(cluster.ID, "StatefulSet", currentKeys, rvMap)
	return updated
}

// ── DaemonSet ───────────────────────────────────────────────────────────────

func (w *ImageIndexWorker) processDaemonSets(cluster *models.Cluster, now time.Time) int {
	lister := w.k8sMgr.DaemonSetsLister(cluster.ID)
	if lister == nil {
		return 0
	}
	dss, err := lister.List(labels.Everything())
	if err != nil {
		return 0
	}

	w.mu.Lock()
	rvMap := w.rvCache[cluster.ID]["DaemonSet"]
	w.mu.Unlock()

	currentKeys := make(map[string]bool, len(dss))
	updated := 0

	for _, ds := range dss {
		key := ds.Namespace + "/" + ds.Name
		currentKeys[key] = true

		rv := ds.ResourceVersion
		if rvMap[key] == rv {
			continue
		}

		entries := daemonSetsToEntries(ds, cluster, now)
		if err := w.upsertWorkload(cluster.ID, "DaemonSet", ds.Namespace, ds.Name, entries); err == nil {
			rvMap[key] = rv
			updated += len(entries)
		}
	}

	w.deleteStale(cluster.ID, "DaemonSet", currentKeys, rvMap)
	return updated
}

// ── DB helpers ───────────────────────────────────────────────────────────────

// upsertWorkload 以「先刪後插」的方式更新單一工作負載的映像條目
func (w *ImageIndexWorker) upsertWorkload(clusterID uint, kind, ns, name string, entries []models.ImageIndex) error {
	return w.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"cluster_id = ? AND workload_kind = ? AND namespace = ? AND workload_name = ?",
			clusterID, kind, ns, name,
		).Delete(&models.ImageIndex{}).Error; err != nil {
			return err
		}
		if len(entries) > 0 {
			return tx.CreateInBatches(entries, 50).Error
		}
		return nil
	})
}

// deleteStale 刪除 rvMap 中已不在 currentKeys 的條目（代表工作負載已被刪除）
func (w *ImageIndexWorker) deleteStale(clusterID uint, kind string, currentKeys map[string]bool, rvMap map[string]string) {
	for key := range rvMap {
		if currentKeys[key] {
			continue
		}
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		ns, name := parts[0], parts[1]
		w.db.Where(
			"cluster_id = ? AND workload_kind = ? AND namespace = ? AND workload_name = ?",
			clusterID, kind, ns, name,
		).Delete(&models.ImageIndex{})
		delete(rvMap, key)
	}
}

// ── 轉換函式（從 K8s 物件萃取映像條目）───────────────────────────────────────

func deploymentsToEntries(d *appsv1.Deployment, cluster *models.Cluster, now time.Time) []models.ImageIndex {
	var entries []models.ImageIndex
	for _, ctr := range d.Spec.Template.Spec.Containers {
		imgName, imgTag := parseImagePartsWorker(ctr.Image)
		entries = append(entries, models.ImageIndex{
			ClusterID: cluster.ID, ClusterName: cluster.Name,
			Namespace: d.Namespace, WorkloadKind: "Deployment", WorkloadName: d.Name,
			ContainerName: ctr.Name, Image: ctr.Image,
			ImageName: imgName, ImageTag: imgTag, LastSyncAt: now,
		})
	}
	return entries
}

func statefulSetsToEntries(ss *appsv1.StatefulSet, cluster *models.Cluster, now time.Time) []models.ImageIndex {
	var entries []models.ImageIndex
	for _, ctr := range ss.Spec.Template.Spec.Containers {
		imgName, imgTag := parseImagePartsWorker(ctr.Image)
		entries = append(entries, models.ImageIndex{
			ClusterID: cluster.ID, ClusterName: cluster.Name,
			Namespace: ss.Namespace, WorkloadKind: "StatefulSet", WorkloadName: ss.Name,
			ContainerName: ctr.Name, Image: ctr.Image,
			ImageName: imgName, ImageTag: imgTag, LastSyncAt: now,
		})
	}
	return entries
}

func daemonSetsToEntries(ds *appsv1.DaemonSet, cluster *models.Cluster, now time.Time) []models.ImageIndex {
	var entries []models.ImageIndex
	for _, ctr := range ds.Spec.Template.Spec.Containers {
		imgName, imgTag := parseImagePartsWorker(ctr.Image)
		entries = append(entries, models.ImageIndex{
			ClusterID: cluster.ID, ClusterName: cluster.Name,
			Namespace: ds.Namespace, WorkloadKind: "DaemonSet", WorkloadName: ds.Name,
			ContainerName: ctr.Name, Image: ctr.Image,
			ImageName: imgName, ImageTag: imgTag, LastSyncAt: now,
		})
	}
	return entries
}

// parseImagePartsWorker 分離映像名稱與 tag（與 handlers/image.go 相同邏輯，避免跨套件依賴）
func parseImagePartsWorker(image string) (name, tag string) {
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		return image[:idx], image[idx+1:]
	}
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return image, "latest"
	}
	afterColon := image[lastColon+1:]
	if strings.Contains(afterColon, "/") {
		return image, "latest"
	}
	return image[:lastColon], afterColon
}
