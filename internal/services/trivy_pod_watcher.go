package services

import (
	"sync"
	"time"

	"github.com/shaia/Synapse/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// ---------------------------------------------------------------------------
// TrivyPodWatcher — Pod 上線自動觸發 Trivy 掃描（CICD_ARCHITECTURE §5 方案 B）
//
// 設計：
//   - 掛載到 ClusterInformerManager 的 Pod Informer OnAdd/OnUpdate
//   - 偵測 Pod 中的新映像（Running 狀態且有 imageID）
//   - 防抖：同一 image+clusterID 在 debounceWindow 內只觸發一次
//   - 使用既有 TrivyService.TriggerScan（含內建 dedup）
// ---------------------------------------------------------------------------

const (
	// TrivyScanSourceInformer 標記來源為 informer 自動掃描。
	TrivyScanSourceInformer = "informer"

	// defaultDebounceWindow 防抖時間窗口。
	defaultDebounceWindow = 5 * time.Minute
)

// TrivyPodWatcher 監控 Pod 事件並自動觸發 Trivy 掃描。
type TrivyPodWatcher struct {
	trivySvc       *TrivyService
	clusterID      uint
	debounceWindow time.Duration

	mu       sync.Mutex
	seen     map[string]time.Time // key: "image" → last trigger time
	stopCh   chan struct{}
	gcTicker *time.Ticker
}

// NewTrivyPodWatcher 建立 TrivyPodWatcher。
func NewTrivyPodWatcher(trivySvc *TrivyService, clusterID uint) *TrivyPodWatcher {
	return &TrivyPodWatcher{
		trivySvc:       trivySvc,
		clusterID:      clusterID,
		debounceWindow: defaultDebounceWindow,
		seen:           make(map[string]time.Time),
		stopCh:         make(chan struct{}),
	}
}

// SetDebounceWindow 設定防抖窗口（測試用）。
func (w *TrivyPodWatcher) SetDebounceWindow(d time.Duration) {
	w.debounceWindow = d
}

// Register 將事件處理器掛載到 Pod Informer。
func (w *TrivyPodWatcher) Register(podInformer cache.SharedIndexInformer) {
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.onPodAdd,
		UpdateFunc: w.onPodUpdate,
	})

	// 啟動 GC goroutine 清理過期的 debounce 記錄
	w.gcTicker = time.NewTicker(10 * time.Minute)
	go w.gcLoop()

	logger.Info("trivy pod watcher registered",
		"cluster_id", w.clusterID,
		"debounce_window", w.debounceWindow,
	)
}

// Stop 停止 GC goroutine。
func (w *TrivyPodWatcher) Stop() {
	close(w.stopCh)
	if w.gcTicker != nil {
		w.gcTicker.Stop()
	}
}

func (w *TrivyPodWatcher) onPodAdd(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}
	w.scanPodImages(pod)
}

func (w *TrivyPodWatcher) onPodUpdate(oldObj, newObj interface{}) {
	pod, ok := newObj.(*corev1.Pod)
	if !ok {
		return
	}
	w.scanPodImages(pod)
}

// scanPodImages 提取 Pod 中已 Running 容器的映像並觸發掃描。
func (w *TrivyPodWatcher) scanPodImages(pod *corev1.Pod) {
	// 只處理 Running 或已排程的 Pod
	if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
		return
	}

	// 跳過系統命名空間
	if isSystemNamespace(pod.Namespace) {
		return
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.ImageID == "" {
			continue // 容器尚未拉取映像
		}
		w.triggerIfNew(pod.Namespace, pod.Name, cs.Name, cs.Image)
	}

	// 也檢查 init containers
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.ImageID == "" {
			continue
		}
		w.triggerIfNew(pod.Namespace, pod.Name, cs.Name, cs.Image)
	}
}

// triggerIfNew 在防抖窗口內只觸發一次。
func (w *TrivyPodWatcher) triggerIfNew(namespace, podName, containerName, image string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	if lastSeen, ok := w.seen[image]; ok && now.Sub(lastSeen) < w.debounceWindow {
		return // 在防抖窗口內，跳過
	}

	w.seen[image] = now

	// 非同步觸發掃描，不阻塞 informer 回調
	go func() {
		_, err := w.trivySvc.TriggerScan(w.clusterID, namespace, podName, containerName, image)
		if err != nil {
			logger.Warn("informer auto-scan trigger failed",
				"cluster_id", w.clusterID,
				"image", image,
				"error", err,
			)
		} else {
			logger.Debug("informer auto-scan triggered",
				"cluster_id", w.clusterID,
				"image", image,
				"namespace", namespace,
			)
		}
	}()
}

// gcLoop 週期性清理過期的 debounce 記錄。
func (w *TrivyPodWatcher) gcLoop() {
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.gcTicker.C:
			w.cleanExpired()
		}
	}
}

func (w *TrivyPodWatcher) cleanExpired() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	for image, lastSeen := range w.seen {
		if now.Sub(lastSeen) > w.debounceWindow*2 {
			delete(w.seen, image)
		}
	}
}

// isSystemNamespace 判斷是否為系統命名空間。
func isSystemNamespace(ns string) bool {
	switch ns {
	case "kube-system", "kube-public", "kube-node-lease", "local-path-storage":
		return true
	}
	return false
}

// SeenCount 返回 debounce 快取中的映像數量（測試用）。
func (w *TrivyPodWatcher) SeenCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.seen)
}

// IsSeen 檢查映像是否在 debounce 快取中（測試用）。
func (w *TrivyPodWatcher) IsSeen(image string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.seen[image]
	return ok
}
