package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PodHandler Pod處理器
type PodHandler struct {
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
	upgrader       websocket.Upgrader
}

// NewPodHandler 建立Pod處理器
func NewPodHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *PodHandler {
	return &PodHandler{
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

// GetPods 獲取Pod列表
func (h *PodHandler) GetPods(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	nodeName := c.Query("nodeName")
	labelSelector := c.Query("labelSelector")
	fieldSelector := c.Query("fieldSelector")
	search := c.Query("search")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	logger.Info("獲取Pod列表: cluster=%s, namespace=%s, node=%s, search=%s", clusterId, namespace, nodeName, search)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	sel := labels.Everything()
	if labelSelector != "" {
		if s, err := labels.Parse(labelSelector); err == nil {
			sel = s
		}
	}

	nodeFilter := ""
	if nodeName != "" {
		nodeFilter = nodeName
	} else if fieldSelector != "" && strings.HasPrefix(fieldSelector, "spec.nodeName=") {
		nodeFilter = strings.TrimPrefix(fieldSelector, "spec.nodeName=")
	}

	allowedNamespaces, hasAllAccess := middleware.GetAllowedNamespaces(c)

	var pods []PodInfo
	if namespace != "" {
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
		allPods := make([]corev1.Pod, 0)
		for _, ns := range allowedNamespaces {
			if strings.HasSuffix(ns, "*") {
				continue
			}
			podObjs, err := h.k8sMgr.PodsLister(cluster.ID).Pods(ns).List(sel)
			if err != nil {
				continue
			}
			for _, p := range podObjs {
				if nodeFilter == "" || p.Spec.NodeName == nodeFilter {
					allPods = append(allPods, *p)
				}
			}
		}

		for _, ns := range allowedNamespaces {
			if strings.HasSuffix(ns, "*") {
				prefix := strings.TrimSuffix(ns, "*")
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

	sort.Slice(pods, func(i, j int) bool {
		return pods[i].CreatedAt.After(pods[j].CreatedAt)
	})

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

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

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

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

// GetPodNamespaces 獲取Pod的命名空間列表
func (h *PodHandler) GetPodNamespaces(c *gin.Context) {
	clusterId := c.Param("clusterID")

	logger.Info("獲取Pod命名空間列表: cluster=%s", clusterId)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	sel := labels.Everything()
	pods, err := h.k8sMgr.PodsLister(cluster.ID).List(sel)
	if err != nil {
		logger.Error("讀取Pod快取失敗", "error", err)
		response.InternalError(c, "獲取命名空間列表失敗: "+err.Error())
		return
	}

	namespaceSet := make(map[string]bool)
	for _, pod := range pods {
		namespaceSet[pod.Namespace] = true
	}

	var namespaces []string
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	if len(namespaces) == 0 {
		namespaces = []string{"default", "kube-system", "kube-public", "kube-node-lease"}
	}

	response.OK(c, namespaces)
}

// GetPodNodes 獲取Pod的節點列表
func (h *PodHandler) GetPodNodes(c *gin.Context) {
	clusterId := c.Param("clusterID")

	logger.Info("獲取Pod節點列表: cluster=%s", clusterId)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if h.k8sMgr == nil {
		response.ServiceUnavailable(c, "K8s informer 管理器未初始化")
		return
	}
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	sel := labels.Everything()
	pods, err := h.k8sMgr.PodsLister(cluster.ID).List(sel)
	if err != nil {
		logger.Error("讀取Pod快取失敗", "error", err)
		response.InternalError(c, "獲取節點列表失敗: "+err.Error())
		return
	}

	nodeSet := make(map[string]bool)
	for _, pod := range pods {
		if pod.Spec.NodeName != "" {
			nodeSet[pod.Spec.NodeName] = true
		}
	}

	var nodes []string
	for node := range nodeSet {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	response.OK(c, nodes)
}
