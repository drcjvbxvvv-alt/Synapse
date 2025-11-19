/** genAI_main_start */
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"kubepolaris/internal/config"
	"kubepolaris/internal/k8s"
	"kubepolaris/internal/models"
	"kubepolaris/internal/services"
	"kubepolaris/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

// DeploymentHandler Deployment处理器
type DeploymentHandler struct {
	db             *gorm.DB
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewDeploymentHandler 创建Deployment处理器
func NewDeploymentHandler(db *gorm.DB, cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *DeploymentHandler {
	return &DeploymentHandler{
		db:             db,
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// DeploymentInfo Deployment信息
type DeploymentInfo struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Status            string            `json:"status"`
	Replicas          int32             `json:"replicas"`
	ReadyReplicas     int32             `json:"readyReplicas"`
	AvailableReplicas int32             `json:"availableReplicas"`
	UpdatedReplicas   int32             `json:"updatedReplicas"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreatedAt         time.Time         `json:"createdAt"`
	Images            []string          `json:"images"`
	Selector          map[string]string `json:"selector"`
	Strategy          string            `json:"strategy"`
}

// ListDeployments 获取Deployment列表
func (h *DeploymentHandler) ListDeployments(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	searchName := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	logger.Info("获取Deployment列表: cluster=%s, namespace=%s, search=%s", clusterId, namespace, searchName)

	// 获取集群信息
	clusterID := parseClusterID(clusterId)
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "集群不存在",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 确保 informer 缓存就绪
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "informer 未就绪: " + err.Error(),
		})
		return
	}

	var deployments []DeploymentInfo
	sel := labels.Everything()

	// 从Informer缓存读取
	if namespace != "" {
		deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).Deployments(namespace).List(sel)
		if err != nil {
			logger.Error("读取Deployment缓存失败", "error", err)
		} else {
			for _, d := range deps {
				deployments = append(deployments, h.convertToDeploymentInfo(d))
			}
		}
	} else {
		deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).List(sel)
		if err != nil {
			logger.Error("读取Deployment缓存失败", "error", err)
		} else {
			for _, d := range deps {
				deployments = append(deployments, h.convertToDeploymentInfo(d))
			}
		}
	}

	// 搜索过滤
	if searchName != "" {
		var filtered []DeploymentInfo
		searchLower := strings.ToLower(searchName)
		for _, dep := range deployments {
			if strings.Contains(strings.ToLower(dep.Name), searchLower) {
				filtered = append(filtered, dep)
			}
		}
		deployments = filtered
	}

	// 排序
	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].CreatedAt.After(deployments[j].CreatedAt)
	})

	// 分页
	total := len(deployments)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedDeployments := deployments[start:end]

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"items":    pagedDeployments,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// GetDeployment 获取Deployment详情
func (h *DeploymentHandler) GetDeployment(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("获取Deployment详情: %s/%s/%s", clusterId, namespace, name)

	clusterID := parseClusterID(clusterId)
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "集群不存在",
		})
		return
	}

	// 创建K8s客户端
	var k8sClient *services.K8sClient
	if cluster.KubeconfigEnc != "" {
		k8sClient, err = services.NewK8sClientFromKubeconfig(cluster.KubeconfigEnc)
	} else {
		k8sClient, err = services.NewK8sClientFromToken(cluster.APIServer, cluster.SATokenEnc, cluster.CAEnc)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建K8s客户端失败: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Deployment不存在: " + err.Error(),
		})
		return
	}

	// 获取关联的Pods
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})
	if err != nil {
		logger.Error("获取Deployment关联Pods失败", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"deployment": h.convertToDeploymentInfo(deployment),
			"raw":        deployment,
			"pods":       pods,
		},
	})
}

// GetDeploymentNamespaces 获取包含Deployment的命名空间列表
func (h *DeploymentHandler) GetDeploymentNamespaces(c *gin.Context) {
	clusterId := c.Param("clusterID")

	clusterID := parseClusterID(clusterId)
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "集群不存在",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 确保 informer 缓存就绪
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "informer 未就绪: " + err.Error(),
		})
		return
	}

	// 从Informer读取所有Deployments并统计命名空间
	sel := labels.Everything()
	deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).List(sel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "读取Deployment缓存失败: " + err.Error(),
		})
		return
	}

	// 统计每个命名空间的Deployment数量
	nsCount := make(map[string]int)
	for _, dep := range deps {
		nsCount[dep.Namespace]++
	}

	// 转换为列表格式
	type NamespaceInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var namespaces []NamespaceInfo
	for ns, count := range nsCount {
		namespaces = append(namespaces, NamespaceInfo{
			Name:  ns,
			Count: count,
		})
	}

	// 按名称排序
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    namespaces,
	})
}

// ScaleDeployment 扩缩容Deployment
func (h *DeploymentHandler) ScaleDeployment(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req ScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	logger.Info("扩缩容Deployment: %s/%s/%s to %d", clusterId, namespace, name, req.Replicas)

	clusterID := parseClusterID(clusterId)
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "集群不存在",
		})
		return
	}

	// 创建K8s客户端
	var k8sClient *services.K8sClient
	if cluster.KubeconfigEnc != "" {
		k8sClient, err = services.NewK8sClientFromKubeconfig(cluster.KubeconfigEnc)
	} else {
		k8sClient, err = services.NewK8sClientFromToken(cluster.APIServer, cluster.SATokenEnc, cluster.CAEnc)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建K8s客户端失败: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	scale, err := clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取Deployment Scale失败: " + err.Error(),
		})
		return
	}

	scale.Spec.Replicas = req.Replicas
	_, err = clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "扩缩容失败: " + err.Error(),
		})
		return
	}

	// 记录审计日志
	auditLog := models.AuditLog{
		UserID:       1, // TODO: 从上下文获取用户ID
		Action:       "scale_deployment",
		ResourceType: "deployment",
		ResourceRef:  fmt.Sprintf(`{"cluster_id":"%s","namespace":"%s","name":"%s"}`, clusterId, namespace, name),
		Result:       "success",
		Details:      fmt.Sprintf("扩缩容Deployment %s/%s 到 %d 个副本", namespace, name, req.Replicas),
	}
	h.db.Create(&auditLog)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "扩缩容成功",
	})
}

// ApplyYAML 应用Deployment YAML
func (h *DeploymentHandler) ApplyYAML(c *gin.Context) {
	clusterId := c.Param("clusterID")

	var req YAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	logger.Info("应用Deployment YAML: cluster=%s, dryRun=%v", clusterId, req.DryRun)

	clusterID := parseClusterID(clusterId)
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "集群不存在",
		})
		return
	}

	// 创建K8s客户端
	var k8sClient *services.K8sClient
	if cluster.KubeconfigEnc != "" {
		k8sClient, err = services.NewK8sClientFromKubeconfig(cluster.KubeconfigEnc)
	} else {
		k8sClient, err = services.NewK8sClientFromToken(cluster.APIServer, cluster.SATokenEnc, cluster.CAEnc)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建K8s客户端失败: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 解析YAML
	var objMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.YAML), &objMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "YAML格式错误: " + err.Error(),
		})
		return
	}

	// 验证必要字段
	if objMap["apiVersion"] == nil || objMap["kind"] == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "YAML缺少必要字段: apiVersion 或 kind",
		})
		return
	}

	kind := objMap["kind"].(string)
	if kind != "Deployment" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "YAML类型错误，期望Deployment，实际为: " + kind,
		})
		return
	}

	// 获取metadata
	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "YAML缺少 metadata 字段",
		})
		return
	}

	name, _ := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	// 应用YAML
	result, err := h.applyYAML(ctx, k8sClient, req.YAML, namespace, req.DryRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "YAML应用失败: " + err.Error(),
		})
		return
	}

	// 记录审计日志
	if !req.DryRun {
		auditLog := models.AuditLog{
			UserID:       1, // TODO: 从上下文获取用户ID
			Action:       "apply_yaml",
			ResourceType: "deployment",
			ResourceRef:  fmt.Sprintf(`{"cluster_id":"%s","namespace":"%s","name":"%s"}`, clusterId, namespace, name),
			Result:       "success",
			Details:      fmt.Sprintf("应用Deployment YAML: %s/%s", namespace, name),
		}
		h.db.Create(&auditLog)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "YAML应用成功",
		"data":    result,
	})
}

// DeleteDeployment 删除Deployment
func (h *DeploymentHandler) DeleteDeployment(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("删除Deployment: %s/%s/%s", clusterId, namespace, name)

	clusterID := parseClusterID(clusterId)
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "集群不存在",
		})
		return
	}

	// 创建K8s客户端
	var k8sClient *services.K8sClient
	if cluster.KubeconfigEnc != "" {
		k8sClient, err = services.NewK8sClientFromKubeconfig(cluster.KubeconfigEnc)
	} else {
		k8sClient, err = services.NewK8sClientFromToken(cluster.APIServer, cluster.SATokenEnc, cluster.CAEnc)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建K8s客户端失败: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	err = clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除失败: " + err.Error(),
		})
		return
	}

	// 记录审计日志
	auditLog := models.AuditLog{
		UserID:       1, // TODO: 从上下文获取用户ID
		Action:       "delete_deployment",
		ResourceType: "deployment",
		ResourceRef:  fmt.Sprintf(`{"cluster_id":"%s","namespace":"%s","name":"%s"}`, clusterId, namespace, name),
		Result:       "success",
		Details:      fmt.Sprintf("删除Deployment: %s/%s", namespace, name),
	}
	h.db.Create(&auditLog)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除成功",
	})
}

// 辅助方法：转换Deployment到DeploymentInfo
func (h *DeploymentHandler) convertToDeploymentInfo(d *appsv1.Deployment) DeploymentInfo {
	status := "Running"
	if d.Status.Replicas == 0 {
		status = "Stopped"
	} else if d.Status.AvailableReplicas < d.Status.Replicas {
		status = "Degraded"
	}

	// 提取镜像列表
	var images []string
	for _, container := range d.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	// 策略
	strategy := string(d.Spec.Strategy.Type)

	return DeploymentInfo{
		ID:                fmt.Sprintf("%s/%s", d.Namespace, d.Name),
		Name:              d.Name,
		Namespace:         d.Namespace,
		Status:            status,
		Replicas:          *d.Spec.Replicas,
		ReadyReplicas:     d.Status.ReadyReplicas,
		AvailableReplicas: d.Status.AvailableReplicas,
		UpdatedReplicas:   d.Status.UpdatedReplicas,
		Labels:            d.Labels,
		Annotations:       d.Annotations,
		CreatedAt:         d.CreationTimestamp.Time,
		Images:            images,
		Selector:          d.Spec.Selector.MatchLabels,
		Strategy:          strategy,
	}
}

// 辅助方法：应用YAML
func (h *DeploymentHandler) applyYAML(ctx context.Context, k8sClient *services.K8sClient, yamlContent string, namespace string, dryRun bool) (interface{}, error) {
	// 创建解码器
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失败: %w", err)
	}

	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil, fmt.Errorf("无法转换为Deployment类型")
	}

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if dryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	// 尝试获取现有资源
	existing, err := clientset.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
	if err == nil {
		// 资源存在，执行更新
		deployment.ResourceVersion = existing.ResourceVersion
		result, err := clientset.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// 资源不存在，执行创建
	result, err := clientset.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{DryRun: dryRunOpt})
	if err != nil {
		return nil, err
	}
	return result, nil
}

/** genAI_main_end */
