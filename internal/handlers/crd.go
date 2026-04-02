package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/clay-wangzhi/KubePolaris/internal/k8s"
	"github.com/clay-wangzhi/KubePolaris/internal/response"
	"github.com/clay-wangzhi/KubePolaris/internal/services"
	"github.com/clay-wangzhi/KubePolaris/pkg/logger"
)

// CRDInfo 描述一個已安裝的 CRD
type CRDInfo struct {
	Name       string `json:"name"`        // e.g. "certificates.cert-manager.io"
	Group      string `json:"group"`       // e.g. "cert-manager.io"
	Version    string `json:"version"`     // e.g. "v1"
	Kind       string `json:"kind"`        // e.g. "Certificate"
	Plural     string `json:"plural"`      // e.g. "certificates"
	Namespaced bool   `json:"namespaced"`
}

// CRDResourceItem 通用 CRD 資源摘要
type CRDResourceItem struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace,omitempty"`
	UID       string                 `json:"uid"`
	Created   string                 `json:"created"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Status    interface{}            `json:"status,omitempty"`
	Spec      map[string]interface{} `json:"spec,omitempty"`
}

// builtinGroups 是 Kubernetes 內建 API 組，不視為 CRD
var builtinGroups = map[string]bool{
	"":                                  true,
	"apps":                              true,
	"batch":                             true,
	"extensions":                        true,
	"networking.k8s.io":                 true,
	"storage.k8s.io":                    true,
	"rbac.authorization.k8s.io":         true,
	"policy":                            true,
	"autoscaling":                       true,
	"apiextensions.k8s.io":              true,
	"admissionregistration.k8s.io":      true,
	"coordination.k8s.io":               true,
	"discovery.k8s.io":                  true,
	"events.k8s.io":                     true,
	"flowcontrol.apiserver.k8s.io":      true,
	"node.k8s.io":                       true,
	"scheduling.k8s.io":                 true,
	"certificates.k8s.io":               true,
	"authentication.k8s.io":             true,
	"authorization.k8s.io":              true,
	"metrics.k8s.io":                    true,
	"apiregistration.k8s.io":            true,
	"internal.apiserver.k8s.io":         true,
}

// CRDHandler CRD 自動發現與通用列表處理器
type CRDHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewCRDHandler 創建 CRD 處理器
func NewCRDHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *CRDHandler {
	return &CRDHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// ListCRDs 自動發現叢集中所有非內建 API 群組下的 CRD
func (h *CRDHandler) ListCRDs(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "无效的集群ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "集群不存在")
		return
	}

	k8sClient, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		response.ServiceUnavailable(c, "无法连接到集群: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, resourceLists, err := k8sClient.GetClientset().Discovery().ServerGroupsAndResources()
	if err != nil && len(resourceLists) == 0 {
		response.InternalError(c, "获取集群资源列表失败")
		return
	}

	var crds []CRDInfo
	seen := map[string]bool{}

	for _, rl := range resourceLists {
		if rl == nil {
			continue
		}
		gv, parseErr := schema.ParseGroupVersion(rl.GroupVersion)
		if parseErr != nil {
			continue
		}
		if builtinGroups[gv.Group] {
			continue
		}

		for _, r := range rl.APIResources {
			// 跳過子資源（如 /status、/scale）
			if strings.Contains(r.Name, "/") {
				continue
			}
			// 每個 group+plural 只保留一個版本
			key := gv.Group + "/" + r.Name
			if seen[key] {
				continue
			}
			seen[key] = true

			crds = append(crds, CRDInfo{
				Name:       r.Name + "." + gv.Group,
				Group:      gv.Group,
				Version:    gv.Version,
				Kind:       r.Kind,
				Plural:     r.Name,
				Namespaced: r.Namespaced,
			})
		}
	}

	_ = ctx // context used for future extensions
	logger.Info("CRD 發現: cluster=%d, count=%d", clusterID, len(crds))
	response.List(c, crds, int64(len(crds)))
}

// ListCRDResources 列出特定 CRD 的所有資源實例
func (h *CRDHandler) ListCRDResources(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "无效的集群ID")
		return
	}

	group := c.Query("group")
	version := c.Query("version")
	plural := c.Query("plural")
	namespace := c.Query("namespace")

	if group == "" || version == "" || plural == "" {
		response.BadRequest(c, "缺少必要参数: group, version, plural")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "集群不存在")
		return
	}

	k8sClient, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		response.ServiceUnavailable(c, "无法连接到集群: "+err.Error())
		return
	}

	dynamicClient, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		response.InternalError(c, "创建动态客户端失败")
		return
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var list *unstructured.UnstructuredList
	if namespace != "" && namespace != "all" {
		list, err = dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		list, err = dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		response.InternalError(c, "获取资源列表失败: "+err.Error())
		return
	}

	items := make([]CRDResourceItem, 0, len(list.Items))
	for _, item := range list.Items {
		ri := CRDResourceItem{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			UID:       string(item.GetUID()),
			Created:   item.GetCreationTimestamp().UTC().Format(time.RFC3339),
			Labels:    item.GetLabels(),
		}
		if status, ok := item.Object["status"]; ok {
			ri.Status = status
		}
		if spec, ok := item.Object["spec"]; ok {
			if specMap, ok := spec.(map[string]interface{}); ok {
				ri.Spec = specMap
			}
		}
		items = append(items, ri)
	}

	response.List(c, items, int64(len(items)))
}

// GetCRDResource 取得單個 CRD 資源實例的完整 YAML/JSON
func (h *CRDHandler) GetCRDResource(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "无效的集群ID")
		return
	}

	group := c.Query("group")
	version := c.Query("version")
	plural := c.Query("plural")
	namespace := c.Param("namespace")
	name := c.Param("name")

	if group == "" || version == "" || plural == "" {
		response.BadRequest(c, "缺少必要参数: group, version, plural")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "集群不存在")
		return
	}

	k8sClient, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		response.ServiceUnavailable(c, "无法连接到集群: "+err.Error())
		return
	}

	dynamicClient, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		response.InternalError(c, "创建动态客户端失败")
		return
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var obj *unstructured.Unstructured
	if namespace != "" && namespace != "_" {
		obj, err = dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		response.NotFound(c, "资源不存在: "+err.Error())
		return
	}

	response.OK(c, obj.Object)
}

// DeleteCRDResource 刪除單個 CRD 資源實例
func (h *CRDHandler) DeleteCRDResource(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "无效的集群ID")
		return
	}

	group := c.Query("group")
	version := c.Query("version")
	plural := c.Query("plural")
	namespace := c.Param("namespace")
	name := c.Param("name")

	if group == "" || version == "" || plural == "" {
		response.BadRequest(c, "缺少必要参数: group, version, plural")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "集群不存在")
		return
	}

	k8sClient, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		response.ServiceUnavailable(c, "无法连接到集群: "+err.Error())
		return
	}

	dynamicClient, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		response.InternalError(c, "创建动态客户端失败")
		return
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	deletePolicy := metav1.DeletePropagationForeground
	opts := metav1.DeleteOptions{PropagationPolicy: &deletePolicy}

	if namespace != "" && namespace != "_" {
		err = dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, opts)
	} else {
		err = dynamicClient.Resource(gvr).Delete(ctx, name, opts)
	}
	if err != nil {
		response.InternalError(c, "删除资源失败: "+err.Error())
		return
	}

	logger.Info("CRD 資源已刪除: cluster=%d, group=%s, plural=%s, ns=%s, name=%s",
		clusterID, group, plural, namespace, name)
	response.NoContent(c)
}
