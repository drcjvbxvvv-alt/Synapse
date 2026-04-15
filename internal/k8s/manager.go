package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/metrics"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"

	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	rolloutsclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	rolloutsinformers "github.com/argoproj/argo-rollouts/pkg/client/informers/externalversions"
	rolloutslisters "github.com/argoproj/argo-rollouts/pkg/client/listers/rollouts/v1alpha1"
)

type ClusterRuntime struct {
	clusterID    uint                // 所屬叢集 ID
	cluster      *models.Cluster     // 原始叢集 model（用於自動重啟）
	k8sClient    *services.K8sClient // 快取的 K8sClient（包含 clientset 和 rest.Config）
	clientset    *kubernetes.Clientset
	factory      informers.SharedInformerFactory
	startedAt    time.Time // informer 啟動時間（偵測卡住用）
	lastAccessAt time.Time // 最後存取時間，用於閒置 GC

	startOnce    sync.Once
	started      bool
	synced       bool
	lastSyncedAt time.Time // 最後快取同步完成時間（用於 informer_last_sync_age_seconds 指標）

	stopCh   chan struct{}
	stopOnce sync.Once

	// Argo Rollouts typed informer (if CRD present)
	rolloutEnabled      bool
	rolloutsClientset   *rolloutsclientset.Clientset
	rolloutsFactory     rolloutsinformers.SharedInformerFactory
	rolloutInformer     cache.SharedIndexInformer
	rolloutLister       rolloutslisters.RolloutLister
	rolloutGroupVersion schema.GroupVersion
}

// discoveryEntry 快取單一叢集的 Discovery 結果
type discoveryEntry struct {
	gv    schema.GroupVersion
	found bool
	at    time.Time
}

const discoveryTTL = 5 * time.Minute

// ClusterInformerManager 統一管理各叢集的 Informer 生命週期與快取訪問
type ClusterInformerManager struct {
	mu             sync.RWMutex
	clusters       map[uint]*ClusterRuntime
	syncTimeout    time.Duration        // configurable cache-sync timeout (default 30s)
	k8sMetrics     *metrics.K8sMetrics // optional; nil = disabled
	discoveryCache map[uint]discoveryEntry // Discovery API 結果快取（TTL 5 分鐘）
}

func NewClusterInformerManager() *ClusterInformerManager {
	return &ClusterInformerManager{
		clusters:       make(map[uint]*ClusterRuntime),
		syncTimeout:    30 * time.Second,
		discoveryCache: make(map[uint]discoveryEntry),
	}
}

// SetMetrics attaches Prometheus K8s metrics to the manager.
func (m *ClusterInformerManager) SetMetrics(km *metrics.K8sMetrics) {
	m.k8sMetrics = km
}

// GetSyncAges 回傳各叢集距最後快取同步的秒數（供 Prometheus Collector 使用）。
// 若叢集從未成功同步，回傳 -1。
func (m *ClusterInformerManager) GetSyncAges() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]float64, len(m.clusters))
	for id, rt := range m.clusters {
		key := fmt.Sprintf("%d", id)
		if rt.lastSyncedAt.IsZero() {
			result[key] = -1
		} else {
			result[key] = time.Since(rt.lastSyncedAt).Seconds()
		}
	}
	return result
}

// SetSyncTimeout overrides the default cache-sync timeout.
// Must be called before any cluster is registered.
func (m *ClusterInformerManager) SetSyncTimeout(d time.Duration) {
	if d > 0 {
		m.syncTimeout = d
	}
}

// EnsureForCluster 確保指定叢集的 informer 已建立並啟動
func (m *ClusterInformerManager) EnsureForCluster(cluster *models.Cluster) (*ClusterRuntime, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rt, ok := m.clusters[cluster.ID]; ok {
		rt.lastAccessAt = time.Now()
		return rt, nil
	}

	// 使用統一入口建立 K8s 客戶端（複用認證/容錯邏輯）
	kc, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		return nil, fmt.Errorf("為叢集建立客戶端失敗: %w", err)
	}

	clientset := kc.GetClientset()
	// resync 為 0 表示關閉週期性全量 Resync，降低壓力
	factory := informers.NewSharedInformerFactory(clientset, 0)

	rt := &ClusterRuntime{
		clusterID:    cluster.ID,
		cluster:      cluster,
		k8sClient:    kc,
		clientset:    clientset,
		factory:      factory,
		stopCh:       make(chan struct{}),
		startedAt:    time.Now(),
		lastAccessAt: time.Now(),
	}

	// 預建立需要的 informer（pods/nodes/ns/services/deployments）
	_ = factory.Core().V1().Pods().Informer()
	_ = factory.Core().V1().Nodes().Informer()
	_ = factory.Core().V1().Namespaces().Informer()
	_ = factory.Core().V1().Services().Informer()
	_ = factory.Core().V1().ConfigMaps().Informer()
	_ = factory.Core().V1().Secrets().Informer()
	_ = factory.Apps().V1().Deployments().Informer()
	_ = factory.Apps().V1().StatefulSets().Informer()
	_ = factory.Apps().V1().DaemonSets().Informer()
	_ = factory.Batch().V1().Jobs().Informer()

	// Detect and setup Argo Rollouts typed informer if CRD exists（結果快取 5 分鐘）
	if gv, found := m.cachedHasArgoRollouts(cluster.ID, clientset); found {
		cfg := kc.GetRestConfig()
		if cfg != nil {
			if roc, err := rolloutsclientset.NewForConfig(cfg); err != nil {
				logger.Error("建立 Argo Rollouts client 失敗", "error", err)
			} else {
				rt.rolloutsClientset = roc
				rt.rolloutsFactory = rolloutsinformers.NewSharedInformerFactory(roc, 0)
				informer := rt.rolloutsFactory.Argoproj().V1alpha1().Rollouts()
				rt.rolloutInformer = informer.Informer()
				rt.rolloutLister = informer.Lister()
				rt.rolloutGroupVersion = gv
				rt.rolloutEnabled = true
			}
		}
	}

	// 啟動
	rt.startOnce.Do(func() {
		factory.Start(rt.stopCh)
		if rt.rolloutsFactory != nil {
			rt.rolloutsFactory.Start(rt.stopCh)
		}
		rt.started = true
	})

	m.clusters[cluster.ID] = rt
	if m.k8sMetrics != nil {
		m.k8sMetrics.ClustersActive.Set(float64(len(m.clusters)))
	}
	return rt, nil
}

// waitForSync 等待本叢集的快取同步就緒（首次可能需要幾十到數百毫秒，取決於資源規模）
func (m *ClusterInformerManager) waitForSync(ctx context.Context, rt *ClusterRuntime) bool {
	if rt.synced {
		return true
	}
	syncCh := make(chan struct{})
	go func() {
		// 等待需要的 Informer 同步
		syncedFuncs := []cache.InformerSynced{
			rt.factory.Core().V1().Pods().Informer().HasSynced,
			rt.factory.Core().V1().Nodes().Informer().HasSynced,
			rt.factory.Core().V1().Namespaces().Informer().HasSynced,
			rt.factory.Core().V1().Services().Informer().HasSynced,
			rt.factory.Core().V1().ConfigMaps().Informer().HasSynced,
			rt.factory.Core().V1().Secrets().Informer().HasSynced,
			rt.factory.Apps().V1().Deployments().Informer().HasSynced,
			rt.factory.Apps().V1().StatefulSets().Informer().HasSynced,
			rt.factory.Apps().V1().DaemonSets().Informer().HasSynced,
			rt.factory.Batch().V1().Jobs().Informer().HasSynced,
			}
		if rt.rolloutEnabled && rt.rolloutInformer != nil {
			syncedFuncs = append(syncedFuncs, rt.rolloutInformer.HasSynced)
		}
		ok := cache.WaitForCacheSync(rt.stopCh, syncedFuncs...)
		if ok {
			rt.synced = true
			rt.lastSyncedAt = time.Now()
			if m.k8sMetrics != nil {
				m.k8sMetrics.InformerSynced.WithLabelValues(fmt.Sprintf("%d", rt.clusterID)).Set(1)
			}
		}
		close(syncCh)
	}()
	select {
	case <-ctx.Done():
		return false
	case <-syncCh:
		return rt.synced
	}
}

// GetOverviewSnapshot 從本地快取即時彙總概覽（不觸發遠端 List）
func (m *ClusterInformerManager) GetOverviewSnapshot(ctx context.Context, clusterID uint) (*OverviewSnapshot, error) {
	m.mu.RLock()
	rt, ok := m.clusters[clusterID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("叢集 %d 未初始化 informer", clusterID)
	}

	// 等待快取同步（使用可設定的逾時，預設 30 秒）
	sctx, cancel := context.WithTimeout(ctx, m.syncTimeout)
	defer cancel()
	if !m.waitForSync(sctx, rt) {
		return nil, fmt.Errorf("informer 快取尚未就緒")
	}

	snap := &OverviewSnapshot{ClusterID: clusterID}

	// Pods
	pods, err := rt.factory.Core().V1().Pods().Lister().List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("讀取快取 pods 失敗: %w", err)
	}
	snap.Pods = len(pods)

	// Nodes
	nodes, err := rt.factory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("讀取快取 nodes 失敗: %w", err)
	}
	snap.Nodes = len(nodes)

	// Namespaces
	namespaces, err := rt.factory.Core().V1().Namespaces().Lister().List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("讀取快取 namespaces 失敗: %w", err)
	}
	snap.Namespace = len(namespaces)

	// Deployments
	deploys, err := rt.factory.Apps().V1().Deployments().Lister().List(labels.Everything())
	if err != nil {
		logger.Error("讀取快取 deployments 失敗", "error", err)
	} else {
		snap.Deployments = len(deploys)
	}

	// StatefulSets
	statefulsets, err := rt.factory.Apps().V1().StatefulSets().Lister().List(labels.Everything())
	if err != nil {
		logger.Error("讀取快取 statefulsets 失敗", "error", err)
	} else {
		snap.StatefulSets = len(statefulsets)
	}

	// DaemonSets
	daemonsets, err := rt.factory.Apps().V1().DaemonSets().Lister().List(labels.Everything())
	if err != nil {
		logger.Error("讀取快取 daemonsets 失敗", "error", err)
	} else {
		snap.DaemonSets = len(daemonsets)
	}

	// Jobs
	jobs, err := rt.factory.Batch().V1().Jobs().Lister().List(labels.Everything())
	if err != nil {
		logger.Error("讀取快取 jobs 失敗", "error", err)
	} else {
		snap.Jobs = len(jobs)
	}

	// Rollouts
	if rt.rolloutEnabled && rt.rolloutLister != nil {
		rollouts, err := rt.rolloutLister.List(labels.Everything())
		if err != nil {
			logger.Error("讀取快取 rollouts 失敗", "error", err)
		} else {
			snap.Rollouts = len(rollouts)
		}
	}

	return snap, nil
}

// EnsureAndWait 確保指定叢集的 informer 啟動並等待快取同步
func (m *ClusterInformerManager) EnsureAndWait(ctx context.Context, cluster *models.Cluster, timeout time.Duration) (*ClusterRuntime, error) {
	rt, err := m.EnsureForCluster(cluster)
	if err != nil {
		return nil, err
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if !m.waitForSync(wctx, rt) {
		return nil, fmt.Errorf("informer 快取尚未就緒")
	}
	return rt, nil
}

// EnsureSync 等待叢集 Informer 就緒，只返回 error（供 services 層的介面使用）
func (m *ClusterInformerManager) EnsureSync(ctx context.Context, cluster *models.Cluster, timeout time.Duration) error {
	_, err := m.EnsureAndWait(ctx, cluster, timeout)
	return err
}

// PodsLister 返回 Pods 的 Lister
func (m *ClusterInformerManager) PodsLister(clusterID uint) corev1listers.PodLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Core().V1().Pods().Lister()
	}
	return nil
}

// NodesLister 返回 Nodes 的 Lister
func (m *ClusterInformerManager) NodesLister(clusterID uint) corev1listers.NodeLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Core().V1().Nodes().Lister()
	}
	return nil
}

// NamespacesLister 返回 Namespaces 的 Lister
func (m *ClusterInformerManager) NamespacesLister(clusterID uint) corev1listers.NamespaceLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Core().V1().Namespaces().Lister()
	}
	return nil
}

// ServicesLister 返回 Services 的 Lister
func (m *ClusterInformerManager) ServicesLister(clusterID uint) corev1listers.ServiceLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Core().V1().Services().Lister()
	}
	return nil
}

// ConfigMapsLister 返回 ConfigMaps 的 Lister
func (m *ClusterInformerManager) ConfigMapsLister(clusterID uint) corev1listers.ConfigMapLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Core().V1().ConfigMaps().Lister()
	}
	return nil
}

// SecretsLister 返回 Secrets 的 Lister
func (m *ClusterInformerManager) SecretsLister(clusterID uint) corev1listers.SecretLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Core().V1().Secrets().Lister()
	}
	return nil
}

// DeploymentsLister 返回 Deployments 的 Lister
func (m *ClusterInformerManager) DeploymentsLister(clusterID uint) appsv1listers.DeploymentLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Apps().V1().Deployments().Lister()
	}
	return nil
}

// StatefulSetsLister 返回 StatefulSets 的 Lister
func (m *ClusterInformerManager) StatefulSetsLister(clusterID uint) appsv1listers.StatefulSetLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Apps().V1().StatefulSets().Lister()
	}
	return nil
}

// DaemonSetsLister 返回 DaemonSets 的 Lister
func (m *ClusterInformerManager) DaemonSetsLister(clusterID uint) appsv1listers.DaemonSetLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Apps().V1().DaemonSets().Lister()
	}
	return nil
}

// JobsLister 返回 Jobs 的 Lister
func (m *ClusterInformerManager) JobsLister(clusterID uint) batchv1listers.JobLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Batch().V1().Jobs().Lister()
	}
	return nil
}

// cachedHasArgoRollouts 帶 TTL 快取的 Argo Rollouts CRD 偵測。
// 呼叫端必須已持有 m.mu 寫鎖（在 EnsureForCluster 內呼叫）。
func (m *ClusterInformerManager) cachedHasArgoRollouts(clusterID uint, cs *kubernetes.Clientset) (schema.GroupVersion, bool) {
	if entry, ok := m.discoveryCache[clusterID]; ok && time.Since(entry.at) < discoveryTTL {
		return entry.gv, entry.found
	}
	gv, found := hasArgoRollouts(cs)
	m.discoveryCache[clusterID] = discoveryEntry{gv: gv, found: found, at: time.Now()}
	return gv, found
}

// InvalidateDiscoveryCache 清除指定叢集的 Discovery 快取（叢集更新時呼叫）
func (m *ClusterInformerManager) InvalidateDiscoveryCache(clusterID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.discoveryCache, clusterID)
}

// hasArgoRollouts 探測是否存在 argoproj.io 的 rollouts 資源，返回其 GroupVersion
func hasArgoRollouts(cs *kubernetes.Clientset) (schema.GroupVersion, bool) {
	groups, resources, err := cs.Discovery().ServerGroupsAndResources()
	_ = groups // 未直接使用
	if err != nil && len(resources) == 0 {
		return schema.GroupVersion{}, false
	}
	for _, rl := range resources {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			continue
		}
		if gv.Group == "argoproj.io" {
			for _, r := range rl.APIResources {
				if r.Name == "rollouts" {
					return gv, true
				}
			}
		}
	}
	return schema.GroupVersion{}, false
}

// RolloutsLister 返回 Argo Rollouts 的 GenericLister（若 CRD 存在）
func (m *ClusterInformerManager) RolloutsLister(clusterID uint) rolloutslisters.RolloutLister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok && rt.rolloutEnabled && rt.rolloutLister != nil {
		return rt.rolloutLister
	}
	return nil
}

// PodInformer 返回指定叢集的 Pod SharedIndexInformer（供 TrivyPodWatcher 註冊事件處理器）。
func (m *ClusterInformerManager) PodInformer(clusterID uint) cache.SharedIndexInformer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.factory.Core().V1().Pods().Informer()
	}
	return nil
}

// GetK8sClient 獲取指定叢集的快取 K8sClient（複用 Informer 管理器中已建立的客戶端，避免重複建立）
func (m *ClusterInformerManager) GetK8sClient(cluster *models.Cluster) (*services.K8sClient, error) {
	rt, err := m.EnsureForCluster(cluster)
	if err != nil {
		return nil, err
	}
	return rt.k8sClient, nil
}

// GetK8sClientByID 根據叢集 ID 獲取已快取的 K8sClient（叢集必須已透過 EnsureForCluster 初始化）
func (m *ClusterInformerManager) GetK8sClientByID(clusterID uint) *services.K8sClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.clusters[clusterID]; ok {
		return rt.k8sClient
	}
	return nil
}

// StartGC 啟動閒置叢集資源回收 Goroutine；interval 為掃描週期，maxIdle 為最大閒置時長
func (m *ClusterInformerManager) StartGC(interval, maxIdle time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			m.mu.Lock()
			now := time.Now()
			for id, rt := range m.clusters {
				if now.Sub(rt.lastAccessAt) > maxIdle {
					logger.Info("GC：停止閒置叢集 informer", "clusterID", id, "idleDuration", now.Sub(rt.lastAccessAt))
					rt.stopOnce.Do(func() { close(rt.stopCh) })
					delete(m.clusters, id)
				}
			}
			m.mu.Unlock()
		}
	}()
}

// StopForCluster 停止指定叢集的 informer（刪除叢集時呼叫）
func (m *ClusterInformerManager) StopForCluster(clusterID uint) {
	m.mu.Lock()
	rt, ok := m.clusters[clusterID]
	if ok {
		delete(m.clusters, clusterID)
		if m.k8sMetrics != nil {
			m.k8sMetrics.ClustersActive.Set(float64(len(m.clusters)))
		}
	}
	m.mu.Unlock()

	if ok && rt != nil {
		logger.Info("停止叢集 informer", "clusterID", clusterID)
		// 使用 sync.Once 確保只關閉一次，避免重複關閉導致 panic
		rt.stopOnce.Do(func() {
			close(rt.stopCh)
		})
		logger.Info("叢集 informer 已停止", "clusterID", clusterID)
	}
}

// HealthCheck 回傳所有已登錄叢集的 Informer 健康快照。
// 純記憶體讀取，無網路呼叫，適合在 /readyz 端點中呼叫。
func (m *ClusterInformerManager) HealthCheck() map[uint]InformerHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[uint]InformerHealth, len(m.clusters))
	for id, rt := range m.clusters {
		result[id] = InformerHealth{
			ClusterID:  id,
			Started:    rt.started,
			Synced:     rt.synced,
			StartedAt:  rt.startedAt,
			LastAccess: rt.lastAccessAt,
		}
	}
	return result
}

// StartHealthWatcher 啟動背景 goroutine，週期性偵測已啟動但長期未同步的 informer，
// 並自動重啟它們。
//
//   - interval: 檢查週期（建議 1 分鐘）
//   - stuckThreshold: informer 啟動後允許的最長未同步時間（建議 5 分鐘）
//
// 此方法應在 StartGC 之後呼叫，只呼叫一次。
func (m *ClusterInformerManager) StartHealthWatcher(interval, stuckThreshold time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			m.restartStuckInformers(stuckThreshold)
		}
	}()
}

// restartStuckInformers finds clusters whose informer has been started but not
// synced for longer than stuckThreshold, stops them, and re-initialises.
// Called by StartHealthWatcher; exported for testability.
func (m *ClusterInformerManager) restartStuckInformers(stuckThreshold time.Duration) {
	// Collect candidates under read lock — avoid holding write lock during network ops.
	type candidate struct {
		id      uint
		cluster *models.Cluster
	}

	m.mu.RLock()
	var stuck []candidate
	now := time.Now()
	for id, rt := range m.clusters {
		if rt.started && !rt.synced && now.Sub(rt.startedAt) > stuckThreshold {
			stuck = append(stuck, candidate{id: id, cluster: rt.cluster})
		}
	}
	m.mu.RUnlock()

	for _, c := range stuck {
		logger.Warn("informer 卡住，嘗試自動重啟",
			"clusterID", c.id,
			"stuckDuration", now.Sub(func() time.Time {
				m.mu.RLock()
				defer m.mu.RUnlock()
				if rt, ok := m.clusters[c.id]; ok {
					return rt.startedAt
				}
				return now
			}()).Round(time.Second),
		)

		m.StopForCluster(c.id)

		if c.cluster == nil {
			logger.Warn("跳過重啟：叢集 model 不可用", "clusterID", c.id)
			continue
		}

		if _, err := m.EnsureForCluster(c.cluster); err != nil {
			logger.Error("自動重啟 informer 失敗", "clusterID", c.id, "error", err)
		} else {
			logger.Info("informer 已自動重啟", "clusterID", c.id)
		}
	}
}

// Stop 關閉所有叢集的 informer（應用退出時呼叫）
func (m *ClusterInformerManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rt := range m.clusters {
		rt.stopOnce.Do(func() {
			close(rt.stopCh)
		})
		delete(m.clusters, id)
	}
}
