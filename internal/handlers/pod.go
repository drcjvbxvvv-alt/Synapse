package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/config"
	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PodHandler Pod處理器
type PodHandler struct {
	db             *gorm.DB
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
	upgrader       websocket.Upgrader
}

// NewPodHandler 建立Pod處理器
func NewPodHandler(db *gorm.DB, cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *PodHandler {
	return &PodHandler{
		db:             db,
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return middleware.IsOriginAllowed(origin)
			},
			ReadBufferSize:  wsBufferSize,
			WriteBufferSize: wsBufferSize,
		},
	}
}

// PodInfo Pod資訊
type PodInfo struct {
	Name              string                  `json:"name"`
	Namespace         string                  `json:"namespace"`
	Status            string                  `json:"status"`
	Phase             string                  `json:"phase"`
	NodeName          string                  `json:"nodeName"`
	PodIP             string                  `json:"podIP"`
	HostIP            string                  `json:"hostIP"`
	RestartCount      int32                   `json:"restartCount"`
	CreatedAt         time.Time               `json:"createdAt"`
	Labels            map[string]string       `json:"labels"`
	Annotations       map[string]string       `json:"annotations"`
	OwnerReferences   []metav1.OwnerReference `json:"ownerReferences"`
	Containers        []ContainerInfo         `json:"containers"`
	InitContainers    []ContainerInfo         `json:"initContainers"`
	Conditions        []PodCondition          `json:"conditions"`
	QOSClass          string                  `json:"qosClass"`
	ServiceAccount    string                  `json:"serviceAccount"`
	Priority          *int32                  `json:"priority,omitempty"`
	PriorityClassName string                  `json:"priorityClassName,omitempty"`
}

// ContainerInfo 容器資訊
type ContainerInfo struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Ready        bool              `json:"ready"`
	RestartCount int32             `json:"restartCount"`
	State        ContainerState    `json:"state"`
	Resources    ContainerResource `json:"resources"`
	Ports        []ContainerPort   `json:"ports"`
}

// ContainerState 容器狀態
type ContainerState struct {
	State     string     `json:"state"`
	Reason    string     `json:"reason,omitempty"`
	Message   string     `json:"message,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
}

// ContainerResource 容器資源
type ContainerResource struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// ContainerPort 容器連接埠
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// PodCondition Pod條件
type PodCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastProbeTime      time.Time `json:"lastProbeTime,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// GetPods 獲取Pod列表
func (h *PodHandler) GetPods(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	nodeName := c.Query("nodeName")
	labelSelector := c.Query("labelSelector")
	fieldSelector := c.Query("fieldSelector")
	search := c.Query("search") // 新增搜尋參數
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	logger.Info("獲取Pod列表: cluster=%s, namespace=%s, node=%s, search=%s", clusterId, namespace, nodeName, search)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	if clusterID == 0 {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 確保 informer 快取就緒
	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}
	// label 選擇器
	sel := labels.Everything()
	if labelSelector != "" {
		if s, err := labels.Parse(labelSelector); err == nil {
			sel = s
		}
	}
	// 節點過濾（支援 nodeName 或 fieldSelector=spec.nodeName=xxx）
	nodeFilter := ""
	if nodeName != "" {
		nodeFilter = nodeName
	} else if fieldSelector != "" && strings.HasPrefix(fieldSelector, "spec.nodeName=") {
		nodeFilter = strings.TrimPrefix(fieldSelector, "spec.nodeName=")
	}

	// 獲取使用者允許訪問的命名空間
	allowedNamespaces, hasAllAccess := middleware.GetAllowedNamespaces(c)

	var pods []PodInfo
	if namespace != "" {
		// 使用者指定了命名空間，檢查權限
		if !hasAllAccess && !middleware.HasNamespaceAccess(c, namespace) {
			response.Forbidden(c, fmt.Sprintf("無權訪問命名空間: %s", namespace))
			return
		}

		podObjs, err := h.k8sMgr.PodsLister(cluster.ID).Pods(namespace).List(sel)
		if err != nil {
			response.InternalError(c, "讀取Pod快取失敗: "+err.Error())
			return
		}
		filtered := make([]corev1.Pod, 0, len(podObjs))
		for _, p := range podObjs {
			if nodeFilter == "" || p.Spec.NodeName == nodeFilter {
				filtered = append(filtered, *p)
			}
		}
		pods = h.convertPodsToInfo(filtered)
	} else if hasAllAccess {
		// 有全部命名空間權限，返回所有Pod
		podObjs, err := h.k8sMgr.PodsLister(cluster.ID).List(sel)
		if err != nil {
			response.InternalError(c, "讀取Pod快取失敗: "+err.Error())
			return
		}
		filtered := make([]corev1.Pod, 0, len(podObjs))
		for _, p := range podObjs {
			if nodeFilter == "" || p.Spec.NodeName == nodeFilter {
				filtered = append(filtered, *p)
			}
		}
		pods = h.convertPodsToInfo(filtered)
	} else {
		// 只有部分命名空間權限，遍歷有權限的命名空間
		allPods := make([]corev1.Pod, 0)
		for _, ns := range allowedNamespaces {
			// 跳過萬用字元命名空間，後面單獨處理
			if strings.HasSuffix(ns, "*") {
				continue
			}
			podObjs, err := h.k8sMgr.PodsLister(cluster.ID).Pods(ns).List(sel)
			if err != nil {
				continue // 跳過出錯的命名空間
			}
			for _, p := range podObjs {
				if nodeFilter == "" || p.Spec.NodeName == nodeFilter {
					allPods = append(allPods, *p)
				}
			}
		}

		// 處理萬用字元命名空間匹配（如 "app-*"）
		for _, ns := range allowedNamespaces {
			if strings.HasSuffix(ns, "*") {
				prefix := strings.TrimSuffix(ns, "*")
				// 獲取所有 Pod，然後過濾匹配的命名空間
				podObjs, err := h.k8sMgr.PodsLister(cluster.ID).List(sel)
				if err != nil {
					continue
				}
				for _, p := range podObjs {
					if strings.HasPrefix(p.Namespace, prefix) {
						if nodeFilter == "" || p.Spec.NodeName == nodeFilter {
							allPods = append(allPods, *p)
						}
					}
				}
			}
		}

		// 去重（如果有多個規則匹配到同一個 Pod）
		seen := make(map[string]bool)
		uniquePods := make([]corev1.Pod, 0)
		for _, p := range allPods {
			key := p.Namespace + "/" + p.Name
			if !seen[key] {
				seen[key] = true
				uniquePods = append(uniquePods, p)
			}
		}

		pods = h.convertPodsToInfo(uniquePods)
	}

	// 搜尋過濾
	if search != "" {
		filteredPods := make([]PodInfo, 0)
		searchLower := strings.ToLower(search)
		for _, pod := range pods {
			if strings.Contains(strings.ToLower(pod.Name), searchLower) ||
				strings.Contains(strings.ToLower(pod.Namespace), searchLower) ||
				strings.Contains(strings.ToLower(pod.NodeName), searchLower) {
				filteredPods = append(filteredPods, pod)
			}
		}
		pods = filteredPods
	}

	// 按建立時間排序（最新的在前）
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].CreatedAt.After(pods[j].CreatedAt)
	})

	// 分頁處理
	total := len(pods)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pagedPods := pods[start:end]

	response.PagedList(c, pagedPods, int64(total), page, pageSize)
}

// GetPod 獲取Pod詳情
func (h *PodHandler) GetPod(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Pod詳情: %s/%s/%s", clusterId, namespace, name)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	if clusterID == 0 {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用 informer+lister 獲取Pod詳情
	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}
	pod, err := h.k8sMgr.PodsLister(cluster.ID).Pods(namespace).Get(name)
	if err != nil {
		response.NotFound(c, "Pod不存在: "+err.Error())
		return
	}

	podInfo := h.convertPodToInfo(*pod)

	response.OK(c, gin.H{
		"pod": podInfo,
		"raw": pod,
	})
}

// DeletePod 刪除Pod
func (h *PodHandler) DeletePod(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("刪除Pod: %s/%s/%s", clusterId, namespace, name)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 刪除Pod
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = k8sClient.GetClientset().CoreV1().Pods(namespace).Delete(ctx, name, deleteOptions)
	if err != nil {
		response.InternalError(c, "刪除失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// GetPodLogs 獲取Pod日誌
func (h *PodHandler) GetPodLogs(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	container := c.Query("container")
	follow := c.Query("follow") == "true"
	previous := c.Query("previous") == "true"
	tailLines := c.Query("tailLines")
	sinceSeconds := c.Query("sinceSeconds")

	logger.Info("獲取Pod日誌: %s/%s/%s, container=%s", clusterId, namespace, name, container)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 構建日誌選項
	logOptions := &corev1.PodLogOptions{
		Follow:   follow,
		Previous: previous,
	}

	if container != "" {
		logOptions.Container = container
	}

	if tailLines != "" {
		if lines, err := strconv.ParseInt(tailLines, 10, 64); err == nil {
			logOptions.TailLines = &lines
		}
	}

	if sinceSeconds != "" {
		if seconds, err := strconv.ParseInt(sinceSeconds, 10, 64); err == nil {
			logOptions.SinceSeconds = &seconds
		}
	}

	// 獲取日誌
	req := k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(name, logOptions)
	logs, err := req.Stream(ctx)
	if err != nil {
		response.InternalError(c, "獲取日誌失敗: "+err.Error())
		return
	}
	defer func() {
		_ = logs.Close()
	}()

	// 如果是follow模式，返回錯誤提示使用WebSocket
	if follow {
		response.BadRequest(c, "流式日誌請使用WebSocket連線: /ws/clusters/:clusterID/pods/:namespace/:name/logs")
		return
	}

	// 讀取日誌內容
	buf := make([]byte, 4096)
	var logContent string
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			logContent += string(buf[:n])
		}
		if err != nil {
			break
		}
	}

	response.OK(c, gin.H{
		"logs": logContent,
	})
}

// convertPodsToInfo 轉換Pod列表為PodInfo
func (h *PodHandler) convertPodsToInfo(pods []corev1.Pod) []PodInfo {
	var podInfos []PodInfo
	for _, pod := range pods {
		podInfos = append(podInfos, h.convertPodToInfo(pod))
	}
	return podInfos
}

// convertPodToInfo 轉換Pod為PodInfo
func (h *PodHandler) convertPodToInfo(pod corev1.Pod) PodInfo {
	// 計算重啟次數
	var restartCount int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		restartCount += containerStatus.RestartCount
	}

	// 轉換容器資訊
	containers := make([]ContainerInfo, 0, len(pod.Spec.Containers))
	for i, container := range pod.Spec.Containers {
		containerInfo := ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
			Resources: ContainerResource{
				Requests: make(map[string]string),
				Limits:   make(map[string]string),
			},
		}

		// 資源資訊
		if container.Resources.Requests != nil {
			for k, v := range container.Resources.Requests {
				containerInfo.Resources.Requests[string(k)] = v.String()
			}
		}
		if container.Resources.Limits != nil {
			for k, v := range container.Resources.Limits {
				containerInfo.Resources.Limits[string(k)] = v.String()
			}
		}

		// 連接埠資訊
		for _, port := range container.Ports {
			containerInfo.Ports = append(containerInfo.Ports, ContainerPort{
				Name:          port.Name,
				ContainerPort: port.ContainerPort,
				Protocol:      string(port.Protocol),
			})
		}

		// 狀態資訊
		if i < len(pod.Status.ContainerStatuses) {
			status := pod.Status.ContainerStatuses[i]
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount

			if status.State.Running != nil {
				containerInfo.State = ContainerState{
					State:     "Running",
					StartedAt: &status.State.Running.StartedAt.Time,
				}
			} else if status.State.Waiting != nil {
				containerInfo.State = ContainerState{
					State:   "Waiting",
					Reason:  status.State.Waiting.Reason,
					Message: status.State.Waiting.Message,
				}
			} else if status.State.Terminated != nil {
				containerInfo.State = ContainerState{
					State:     "Terminated",
					Reason:    status.State.Terminated.Reason,
					Message:   status.State.Terminated.Message,
					StartedAt: &status.State.Terminated.StartedAt.Time,
				}
			}
		}

		containers = append(containers, containerInfo)
	}

	// 轉換Init容器資訊
	initContainers := make([]ContainerInfo, 0, len(pod.Spec.InitContainers))
	for i, container := range pod.Spec.InitContainers {
		containerInfo := ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
			Resources: ContainerResource{
				Requests: make(map[string]string),
				Limits:   make(map[string]string),
			},
		}

		// 狀態資訊
		if i < len(pod.Status.InitContainerStatuses) {
			status := pod.Status.InitContainerStatuses[i]
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
		}

		initContainers = append(initContainers, containerInfo)
	}

	// 轉換條件資訊
	conditions := make([]PodCondition, 0, len(pod.Status.Conditions))
	for _, condition := range pod.Status.Conditions {
		conditions = append(conditions, PodCondition{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			LastProbeTime:      condition.LastProbeTime.Time,
			LastTransitionTime: condition.LastTransitionTime.Time,
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	return PodInfo{
		Name:              pod.Name,
		Namespace:         pod.Namespace,
		Status:            h.getPodStatus(pod),
		Phase:             string(pod.Status.Phase),
		NodeName:          pod.Spec.NodeName,
		PodIP:             pod.Status.PodIP,
		HostIP:            pod.Status.HostIP,
		RestartCount:      restartCount,
		CreatedAt:         pod.CreationTimestamp.Time,
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
		OwnerReferences:   pod.OwnerReferences,
		Containers:        containers,
		InitContainers:    initContainers,
		Conditions:        conditions,
		QOSClass:          string(pod.Status.QOSClass),
		ServiceAccount:    pod.Spec.ServiceAccountName,
		Priority:          pod.Spec.Priority,
		PriorityClassName: pod.Spec.PriorityClassName,
	}
}

// getPodStatus 獲取Pod狀態
func (h *PodHandler) getPodStatus(pod corev1.Pod) string {
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}

	switch pod.Status.Phase {
	case corev1.PodPending:
		// 檢查是否有容器在等待
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil {
				if containerStatus.State.Waiting.Reason == "ImagePullBackOff" ||
					containerStatus.State.Waiting.Reason == "ErrImagePull" {
					return containerStatus.State.Waiting.Reason
				}
			}
		}
		return "Pending"
	case corev1.PodRunning:
		// 檢查是否所有容器都就緒
		ready := 0
		total := len(pod.Status.ContainerStatuses)
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Ready {
				ready++
			} else if containerStatus.State.Waiting != nil {
				return containerStatus.State.Waiting.Reason
			} else if containerStatus.State.Terminated != nil {
				return containerStatus.State.Terminated.Reason
			}
		}
		if ready == total {
			return "Running"
		}
		return fmt.Sprintf("NotReady (%d/%d)", ready, total)
	case corev1.PodSucceeded:
		return "Completed"
	case corev1.PodFailed:
		return "Failed"
	default:
		return string(pod.Status.Phase)
	}
}

// GetPodNamespaces 獲取Pod的命名空間列表
func (h *PodHandler) GetPodNamespaces(c *gin.Context) {
	clusterId := c.Param("clusterID")

	logger.Info("獲取Pod命名空間列表: cluster=%s", clusterId)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 確保 informer 快取就緒
	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	// 獲取所有Pod的命名空間
	sel := labels.Everything()
	pods, err := h.k8sMgr.PodsLister(cluster.ID).List(sel)
	if err != nil {
		logger.Error("讀取Pod快取失敗", "error", err)
		response.InternalError(c, "獲取命名空間列表失敗: "+err.Error())
		return
	}

	// 收集唯一的命名空間
	namespaceSet := make(map[string]bool)
	for _, pod := range pods {
		namespaceSet[pod.Namespace] = true
	}

	// 轉換為切片並排序
	var namespaces []string
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	// 如果沒有找到命名空間，返回預設的
	if len(namespaces) == 0 {
		namespaces = []string{"default", "kube-system", "kube-public", "kube-node-lease"}
	}

	response.OK(c, namespaces)
}

// GetPodNodes 獲取Pod的節點列表
func (h *PodHandler) GetPodNodes(c *gin.Context) {
	clusterId := c.Param("clusterID")

	logger.Info("獲取Pod節點列表: cluster=%s", clusterId)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 確保 informer 快取就緒
	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	// 獲取所有Pod的節點
	sel := labels.Everything()
	pods, err := h.k8sMgr.PodsLister(cluster.ID).List(sel)
	if err != nil {
		logger.Error("讀取Pod快取失敗", "error", err)
		response.InternalError(c, "獲取節點列表失敗: "+err.Error())
		return
	}

	// 收集唯一的節點名稱
	nodeSet := make(map[string]bool)
	for _, pod := range pods {
		if pod.Spec.NodeName != "" {
			nodeSet[pod.Spec.NodeName] = true
		}
	}

	// 轉換為切片並排序
	var nodes []string
	for node := range nodeSet {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	response.OK(c, nodes)
}

// StreamPodLogs WebSocket流式傳輸Pod日誌
func (h *PodHandler) StreamPodLogs(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	container := c.Query("container")
	previous := c.Query("previous") == "true"
	tailLines := c.Query("tailLines")
	sinceSeconds := c.Query("sinceSeconds")

	logger.Info("WebSocket流式獲取Pod日誌: %s/%s/%s, container=%s", clusterId, namespace, name, container)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 升級到WebSocket連線
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("升級WebSocket連線失敗", "error", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	// 傳送連線成功訊息
	err = conn.WriteJSON(map[string]interface{}{
		"type":    "connected",
		"message": "WebSocket連線已建立",
	})
	if err != nil {
		logger.Error("傳送連線訊息失敗", "error", err)
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		_ = conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"message": "獲取K8s客戶端失敗: " + err.Error(),
		})
		return
	}

	// 建立上下文 - 使用WithCancel而不是WithTimeout，因為WebSocket需要長時間執行
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 構建日誌選項
	logOptions := &corev1.PodLogOptions{
		Follow:   true, // 流式模式
		Previous: previous,
	}

	if container != "" {
		logOptions.Container = container
	}

	if tailLines != "" {
		if lines, err := strconv.ParseInt(tailLines, 10, 64); err == nil {
			logOptions.TailLines = &lines
		}
	}

	if sinceSeconds != "" {
		if seconds, err := strconv.ParseInt(sinceSeconds, 10, 64); err == nil {
			logOptions.SinceSeconds = &seconds
		}
	}

	// 獲取日誌流（使用快取的clientset）
	req := k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(name, logOptions)
	logStream, err := req.Stream(context.Background())
	if err != nil {
		_ = conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"message": "獲取日誌流失敗: " + err.Error(),
		})
		return
	}
	defer func() {
		_ = logStream.Close()
	}()

	// 建立讀取器
	reader := bufio.NewReader(logStream)

	// 啟動goroutine讀取客戶端訊息（用於處理關閉連線）
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				logger.Info("WebSocket連線關閉", "error", err)
				cancel()
				_ = logStream.Close() // 主動關閉日誌流
				return
			}
		}
	}()

	// 傳送日誌開始訊息
	err = conn.WriteJSON(map[string]interface{}{
		"type":    "start",
		"message": "開始接收日誌流",
	})
	if err != nil {
		logger.Error("傳送開始訊息失敗", "error", err)
		return
	}

	// 流式讀取併傳送日誌
	for {
		select {
		case <-ctx.Done():
			// 連線被關閉
			_ = conn.WriteJSON(map[string]interface{}{
				"type":    "closed",
				"message": "日誌流已關閉",
			})
			return
		default:
			// 讀取一行日誌
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// 日誌流正常結束
					_ = conn.WriteJSON(map[string]interface{}{
						"type":    "end",
						"message": "日誌流已結束",
					})
					return
				}

				// 檢查是否是因為stream被關閉（客戶端斷開連線）
				// 包含 "closed"、"canceled" 或 "cancel" 的錯誤都是正常的斷開
				errStr := err.Error()
				if strings.Contains(errStr, "closed") ||
					strings.Contains(errStr, "canceled") ||
					strings.Contains(errStr, "cancel") {
					logger.Info("日誌流停止: 連線已關閉或取消")
					return
				}

				// 檢查是否是context取消
				if ctx.Err() != nil {
					logger.Info("日誌流停止: context取消")
					return
				}

				// 其他錯誤才記錄ERROR
				logger.Error("讀取日誌失敗", "error", err)
				_ = conn.WriteJSON(map[string]interface{}{
					"type":    "error",
					"message": "讀取日誌失敗: " + err.Error(),
				})
				return
			}

			// 傳送日誌內容
			err = conn.WriteJSON(map[string]interface{}{
				"type": "log",
				"data": line,
			})
			if err != nil {
				// WebSocket傳送失敗，客戶端可能已斷開
				logger.Info("傳送日誌失敗，客戶端可能已斷開", "error", err)
				return
			}
		}
	}
}
