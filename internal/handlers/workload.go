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

	rolloutsclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// WorkloadHandler 工作负载处理器
type WorkloadHandler struct {
	db             *gorm.DB
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewWorkloadHandler 创建工作负载处理器
func NewWorkloadHandler(db *gorm.DB, cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *WorkloadHandler {
	return &WorkloadHandler{
		db:             db,
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// WorkloadType 工作负载类型
type WorkloadType string

const (
	WorkloadTypeDeployment  WorkloadType = "Deployment"
	WorkloadTypeRollout     WorkloadType = "Rollout"
	WorkloadTypeStateless   WorkloadType = "Stateless"
	WorkloadTypeStatefulSet WorkloadType = "StatefulSet"
	WorkloadTypeDaemonSet   WorkloadType = "DaemonSet"
	WorkloadTypeJob         WorkloadType = "Job"
	WorkloadTypeCronJob     WorkloadType = "CronJob"
)

// WorkloadInfo 工作负载信息
type WorkloadInfo struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Type              WorkloadType      `json:"type"`
	Status            string            `json:"status"`
	Replicas          int32             `json:"replicas"`
	ReadyReplicas     int32             `json:"readyReplicas"`
	AvailableReplicas int32             `json:"availableReplicas"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreatedAt         time.Time         `json:"createdAt"`
	Images            []string          `json:"images"`
	Selector          map[string]string `json:"selector"`
	Strategy          string            `json:"strategy,omitempty"`
	Schedule          string            `json:"schedule,omitempty"` // For CronJob
}

// ScaleRequest 扩缩容请求
type ScaleRequest struct {
	Replicas int32 `json:"replicas" binding:"required,min=0"`
}

// YAMLApplyRequest YAML应用请求
type YAMLApplyRequest struct {
	YAML   string `json:"yaml" binding:"required"`
	DryRun bool   `json:"dryRun"`
}

// GetWorkloads 获取工作负载列表
func (h *WorkloadHandler) GetWorkloads(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	workloadType := c.Query("type")
	searchName := c.Query("search") // 新增搜索参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	logger.Info("获取工作负载列表: cluster=%s, namespace=%s, type=%s, search=%s", clusterId, namespace, workloadType, searchName)

	// 从集群服务获取集群信息
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
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "informer 未就绪: " + err.Error()})
		return
	}

	var workloads []WorkloadInfo
	sel := labels.Everything()

	// Deployments
	if workloadType == "" || workloadType == string(WorkloadTypeDeployment) || workloadType == string(WorkloadTypeStateless) {
		if namespace != "" {
			deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).Deployments(namespace).List(sel)
			if err != nil {
				logger.Error("读取Deployment缓存失败", "error", err)
			} else {
				for _, d := range deps {
					workloads = append(workloads, h.convertDeploymentToWorkloadInfo(d))
				}
			}
		} else {
			deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).List(sel)
			if err != nil {
				logger.Error("读取Deployment缓存失败", "error", err)
			} else {
				for _, d := range deps {
					workloads = append(workloads, h.convertDeploymentToWorkloadInfo(d))
				}
			}
		}
	}

	// Rollouts
	if workloadType == "" || workloadType == string(WorkloadTypeRollout) || workloadType == string(WorkloadTypeStateless) {
		if namespace != "" {
			// 过滤 Ns
			rs, err := h.k8sMgr.RolloutsLister(cluster.ID).Rollouts(namespace).List(sel)
			if err != nil {
				logger.Error("读取Rollout缓存失败", "error", err)
			} else {
				for _, r := range rs {
					workloads = append(workloads, h.convertRolloutToWorkloadInfo(r))
				}
			}
		} else {
			rs, err := h.k8sMgr.RolloutsLister(cluster.ID).List(sel)
			if err != nil {
				logger.Error("读取Rollout缓存失败", "error", err)
			} else {
				for _, r := range rs {
					workloads = append(workloads, h.convertRolloutToWorkloadInfo(r))
				}
			}
		}
	}

	// StatefulSets
	if workloadType == string(WorkloadTypeStatefulSet) {
		if l := h.k8sMgr.StatefulSetsLister(cluster.ID); l != nil {
			if namespace != "" {
				items, err := l.StatefulSets(namespace).List(sel)
				if err != nil {
					logger.Error("读取StatefulSet缓存失败", "error", err)
				} else {
					for _, it := range items {
						workloads = append(workloads, h.convertStatefulSetToWorkloadInfo(it))
					}
				}
			} else {
				items, err := l.List(sel)
				if err != nil {
					logger.Error("读取StatefulSet缓存失败", "error", err)
				} else {
					for _, it := range items {
						workloads = append(workloads, h.convertStatefulSetToWorkloadInfo(it))
					}
				}
			}
		}
	}

	// DaemonSets
	if workloadType == string(WorkloadTypeDaemonSet) {
		if l := h.k8sMgr.DaemonSetsLister(cluster.ID); l != nil {
			if namespace != "" {
				items, err := l.DaemonSets(namespace).List(sel)
				if err != nil {
					logger.Error("读取DaemonSet缓存失败", "error", err)
				} else {
					for _, it := range items {
						workloads = append(workloads, h.convertDaemonSetToWorkloadInfo(it))
					}
				}
			} else {
				items, err := l.List(sel)
				if err != nil {
					logger.Error("读取DaemonSet缓存失败", "error", err)
				} else {
					for _, it := range items {
						workloads = append(workloads, h.convertDaemonSetToWorkloadInfo(it))
					}
				}
			}
		}
	}

	// Jobs
	if workloadType == string(WorkloadTypeJob) {
		if namespace != "" {
			items, err := h.k8sMgr.JobsLister(cluster.ID).Jobs(namespace).List(sel)
			if err != nil {
				logger.Error("读取Job缓存失败", "error", err)
			} else {
				for _, it := range items {
					workloads = append(workloads, h.convertJobToWorkloadInfo(it))
				}
			}
		} else {
			items, err := h.k8sMgr.JobsLister(cluster.ID).List(sel)
			if err != nil {
				logger.Error("读取Job缓存失败", "error", err)
			} else {
				for _, it := range items {
					workloads = append(workloads, h.convertJobToWorkloadInfo(it))
				}
			}
		}
	}

	// CronJobs
	// if workloadType == string(WorkloadTypeCronJob) {
	// 	if namespace != "" {
	// 		items, err := h.k8sMgr.CronJobsLister(cluster.ID).CronJobs(namespace).List(sel)
	// 		if err != nil {
	// 			logger.Error("读取CronJob缓存失败", "error", err)
	// 		} else {
	// 			for _, it := range items {
	// 				workloads = append(workloads, h.convertCronJobToWorkloadInfo(it))
	// 			}
	// 		}
	// 	} else {
	// 		items, err := h.k8sMgr.CronJobsLister(cluster.ID).List(sel)
	// 		if err != nil {
	// 			logger.Error("读取CronJob缓存失败", "error", err)
	// 		} else {
	// 			for _, it := range items {
	// 				workloads = append(workloads, h.convertCronJobToWorkloadInfo(it))
	// 			}
	// 		}
	// 	}
	// }

	// 按名称搜索过滤
	if searchName != "" {
		var filteredWorkloads []WorkloadInfo
		searchLower := strings.ToLower(searchName)
		for _, workload := range workloads {
			if strings.Contains(strings.ToLower(workload.Name), searchLower) {
				filteredWorkloads = append(filteredWorkloads, workload)
			}
		}
		workloads = filteredWorkloads
	}

	// 按创建时间排序（最新的在前）
	sort.Slice(workloads, func(i, j int) bool {
		return workloads[i].CreatedAt.After(workloads[j].CreatedAt)
	})

	// 分页处理
	total := len(workloads)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pagedWorkloads := workloads[start:end]

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"items":    pagedWorkloads,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// GetWorkload 获取工作负载详情
func (h *WorkloadHandler) GetWorkload(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	workloadType := c.Query("type")

	logger.Info("获取工作负载详情: %s/%s/%s/%s", clusterId, workloadType, namespace, name)

	// 从集群服务获取集群信息
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

	var workload interface{}
	var workloadInfo WorkloadInfo

	// genAI_main_start
	switch WorkloadType(workloadType) {
	case WorkloadTypeDeployment:
		deployment, err := k8sClient.GetClientset().AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工作负载不存在: " + err.Error(),
			})
			return
		}
		// 确保设置 APIVersion 和 Kind
		deployment.APIVersion = "apps/v1"
		deployment.Kind = "Deployment"
		workload = deployment
		workloadInfo = h.convertDeploymentToWorkloadInfo(deployment)

	case WorkloadTypeRollout:
		rolloutsClient, err := rolloutsclientset.NewForConfig(k8sClient.GetRestConfig())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "创建Argo Rollouts客户端失败: " + err.Error(),
			})
			return
		}
		rollout, err := rolloutsClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工作负载不存在: " + err.Error(),
			})
			return
		}
		// 确保设置 APIVersion 和 Kind
		rollout.APIVersion = "argoproj.io/v1alpha1"
		rollout.Kind = "Rollout"
		workload = rollout
		workloadInfo = h.convertRolloutToWorkloadInfo(rollout)

	case WorkloadTypeStatefulSet:
		statefulSet, err := k8sClient.GetClientset().AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工作负载不存在: " + err.Error(),
			})
			return
		}
		// 确保设置 APIVersion 和 Kind
		statefulSet.APIVersion = "apps/v1"
		statefulSet.Kind = "StatefulSet"
		workload = statefulSet
		workloadInfo = h.convertStatefulSetToWorkloadInfo(statefulSet)

	case WorkloadTypeDaemonSet:
		daemonSet, err := k8sClient.GetClientset().AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工作负载不存在: " + err.Error(),
			})
			return
		}
		// 确保设置 APIVersion 和 Kind
		daemonSet.APIVersion = "apps/v1"
		daemonSet.Kind = "DaemonSet"
		workload = daemonSet
		workloadInfo = h.convertDaemonSetToWorkloadInfo(daemonSet)

	case WorkloadTypeJob:
		job, err := k8sClient.GetClientset().BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工作负载不存在: " + err.Error(),
			})
			return
		}
		// 确保设置 APIVersion 和 Kind
		job.APIVersion = "batch/v1"
		job.Kind = "Job"
		workload = job
		workloadInfo = h.convertJobToWorkloadInfo(job)

	case WorkloadTypeCronJob:
		cronJob, err := k8sClient.GetClientset().BatchV1beta1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "工作负载不存在: " + err.Error(),
			})
			return
		}
		// 确保设置 APIVersion 和 Kind
		cronJob.APIVersion = "batch/v1beta1"
		cronJob.Kind = "CronJob"
		workload = cronJob
		workloadInfo = h.convertCronJobToWorkloadInfo(cronJob)

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的工作负载类型: " + workloadType,
		})
		return
	}
	// genAI_main_end

	// 获取关联的Pod
	pods, err := h.getWorkloadPods(ctx, k8sClient, namespace, workloadInfo.Selector)
	if err != nil {
		logger.Error("获取工作负载Pod失败", "error", err)
		pods = []interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"workload": workloadInfo,
			"raw":      workload,
			"pods":     pods,
		},
	})
}

// ScaleWorkload 扩缩容工作负载
func (h *WorkloadHandler) ScaleWorkload(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	workloadType := c.Query("type")

	var req ScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	logger.Info("扩缩容工作负载: %s/%s/%s/%s to %d", clusterId, workloadType, namespace, name, req.Replicas)

	// 从集群服务获取集群信息
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

	switch WorkloadType(workloadType) {
	case WorkloadTypeDeployment:
		err = h.scaleDeployment(ctx, k8sClient, namespace, name, req.Replicas)
	case WorkloadTypeStatefulSet:
		err = h.scaleStatefulSet(ctx, k8sClient, namespace, name, req.Replicas)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "该工作负载类型不支持扩缩容: " + workloadType,
		})
		return
	}

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
		Action:       "scale_workload",
		ResourceType: "workload",
		ResourceRef:  fmt.Sprintf(`{"cluster_id":"%s","type":"%s","namespace":"%s","name":"%s"}`, clusterId, workloadType, namespace, name),
		Result:       "success",
		Details:      fmt.Sprintf("扩缩容工作负载 %s/%s 到 %d 副本", namespace, name, req.Replicas),
	}
	h.db.Create(&auditLog)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "扩缩容成功",
		"data":    nil,
	})
}

// genAI_main_start
// ApplyYAML 应用YAML配置
func (h *WorkloadHandler) ApplyYAML(c *gin.Context) {
	clusterId := c.Param("clusterID")

	var req YAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	logger.Info("应用YAML配置: cluster=%s, dryRun=%v, yaml长度=%d", clusterId, req.DryRun, len(req.YAML))

	// 从集群服务获取集群信息
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 解析YAML到map[string]interface{}
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
	apiVersion := objMap["apiVersion"].(string)

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

	logger.Info("解析YAML: kind=%s, apiVersion=%s, namespace=%s, name=%s", kind, apiVersion, namespace, name)

	var result interface{}
	var applyErr error

	if req.DryRun {
		// DryRun模式，只验证不实际应用
		applyErr = h.applyYAMLToCluster(ctx, k8sClient, req.YAML, namespace, true)
		if applyErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "YAML验证失败: " + applyErr.Error(),
			})
			return
		}

		result = map[string]interface{}{
			"dryRun":     true,
			"valid":      true,
			"kind":       kind,
			"apiVersion": apiVersion,
			"namespace":  namespace,
			"name":       name,
			"message":    "YAML格式正确，可以安全应用",
		}
	} else {
		// 实际应用YAML
		applyErr = h.applyYAMLToCluster(ctx, k8sClient, req.YAML, namespace, false)
		if applyErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "YAML应用失败: " + applyErr.Error(),
			})
			return
		}

		result = map[string]interface{}{
			"applied":    true,
			"kind":       kind,
			"apiVersion": apiVersion,
			"namespace":  namespace,
			"name":       name,
			"message":    fmt.Sprintf("成功应用 %s/%s", kind, name),
		}
	}

	// 记录审计日志
	if !req.DryRun {
		auditLog := models.AuditLog{
			UserID:       1, // TODO: 从上下文获取用户ID
			Action:       "apply_yaml",
			ResourceType: "yaml",
			ResourceRef:  fmt.Sprintf(`{"cluster_id":"%s","kind":"%s","namespace":"%s","name":"%s"}`, clusterId, kind, namespace, name),
			Result:       "success",
			Details:      fmt.Sprintf("应用YAML配置: %s/%s in %s", kind, name, namespace),
		}
		h.db.Create(&auditLog)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "YAML应用成功",
		"data":    result,
	})
}

// genAI_main_end

// DeleteWorkload 删除工作负载
func (h *WorkloadHandler) DeleteWorkload(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	workloadType := c.Param("type")

	logger.Info("删除工作负载: %s/%s/%s/%s", clusterId, workloadType, namespace, name)

	// 从集群服务获取集群信息
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

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	switch WorkloadType(workloadType) {
	case WorkloadTypeDeployment:
		err = k8sClient.GetClientset().AppsV1().Deployments(namespace).Delete(ctx, name, deleteOptions)
	case WorkloadTypeStatefulSet:
		err = k8sClient.GetClientset().AppsV1().StatefulSets(namespace).Delete(ctx, name, deleteOptions)
	case WorkloadTypeDaemonSet:
		err = k8sClient.GetClientset().AppsV1().DaemonSets(namespace).Delete(ctx, name, deleteOptions)
	case WorkloadTypeJob:
		err = k8sClient.GetClientset().BatchV1().Jobs(namespace).Delete(ctx, name, deleteOptions)
	case WorkloadTypeCronJob:
		err = k8sClient.GetClientset().BatchV1().CronJobs(namespace).Delete(ctx, name, deleteOptions)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的工作负载类型: " + workloadType,
		})
		return
	}

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
		Action:       "delete_workload",
		ResourceType: "workload",
		ResourceRef:  fmt.Sprintf(`{"cluster_id":"%s","type":"%s","namespace":"%s","name":"%s"}`, clusterId, workloadType, namespace, name),
		Result:       "success",
		Details:      fmt.Sprintf("删除工作负载 %s/%s", namespace, name),
	}
	h.db.Create(&auditLog)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除成功",
		"data":    nil,
	})
}

// GetWorkloadNamespaces 获取工作负载的命名空间列表
func (h *WorkloadHandler) GetWorkloadNamespaces(c *gin.Context) {
	clusterId := c.Param("clusterID")
	workloadType := c.Query("type")

	logger.Info("获取工作负载命名空间列表: cluster=%s, type=%s", clusterId, workloadType)

	// 从集群服务获取集群信息
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
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "informer 未就绪: " + err.Error()})
		return
	}

	namespaceSet := make(map[string]bool)
	sel := labels.Everything()

	// 根据类型获取命名空间
	switch WorkloadType(workloadType) {
	case WorkloadTypeStateless:
		// Deployments
		deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).List(sel)
		if err == nil {
			for _, d := range deps {
				namespaceSet[d.Namespace] = true
			}
		}
		// Rollouts
		rs, err := h.k8sMgr.RolloutsLister(cluster.ID).List(sel)
		if err == nil {
			for _, r := range rs {
				namespaceSet[r.Namespace] = true
			}
		}
	case WorkloadTypeRollout:
		rs, err := h.k8sMgr.RolloutsLister(cluster.ID).List(sel)
		if err == nil {
			for _, r := range rs {
				namespaceSet[r.Namespace] = true
			}
		}
	case WorkloadTypeDeployment:
		deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).List(sel)
		if err == nil {
			for _, d := range deps {
				namespaceSet[d.Namespace] = true
			}
		}
	case WorkloadTypeStatefulSet:
		if l := h.k8sMgr.StatefulSetsLister(cluster.ID); l != nil {
			items, err := l.List(sel)
			if err == nil {
				for _, it := range items {
					namespaceSet[it.Namespace] = true
				}
			}
		}
	case WorkloadTypeDaemonSet:
		if l := h.k8sMgr.DaemonSetsLister(cluster.ID); l != nil {
			items, err := l.List(sel)
			if err == nil {
				for _, it := range items {
					namespaceSet[it.Namespace] = true
				}
			}
		}
	case WorkloadTypeJob:
		items, err := h.k8sMgr.JobsLister(cluster.ID).List(sel)
		if err == nil {
			for _, it := range items {
				namespaceSet[it.Namespace] = true
			}
		}
	case WorkloadTypeCronJob:
		// CronJobs 暂时注释掉
		// if l := h.k8sMgr.CronJobsLister(cluster.ID); l != nil {
		// 	items, err := l.List(sel)
		// 	if err == nil {
		// 		for _, it := range items {
		// 			namespaceSet[it.Namespace] = true
		// 		}
		// 	}
		// }
	default:
		// 获取所有类型的命名空间
		// Deployments
		deps, err := h.k8sMgr.DeploymentsLister(cluster.ID).List(sel)
		if err == nil {
			for _, d := range deps {
				namespaceSet[d.Namespace] = true
			}
		}
		// Rollouts
		rs, err := h.k8sMgr.RolloutsLister(cluster.ID).List(sel)
		if err == nil {
			for _, r := range rs {
				namespaceSet[r.Namespace] = true
			}
		}
		// StatefulSets
		if l := h.k8sMgr.StatefulSetsLister(cluster.ID); l != nil {
			items, err := l.List(sel)
			if err == nil {
				for _, it := range items {
					namespaceSet[it.Namespace] = true
				}
			}
		}
		// DaemonSets
		if l := h.k8sMgr.DaemonSetsLister(cluster.ID); l != nil {
			items, err := l.List(sel)
			if err == nil {
				for _, it := range items {
					namespaceSet[it.Namespace] = true
				}
			}
		}
		// Jobs
		items, err := h.k8sMgr.JobsLister(cluster.ID).List(sel)
		if err == nil {
			for _, it := range items {
				namespaceSet[it.Namespace] = true
			}
		}
	}

	// 转换为切片并排序
	var namespaces []string
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	// 如果没有找到命名空间，返回默认的
	if len(namespaces) == 0 {
		namespaces = []string{"default", "kube-system", "kube-public", "kube-node-lease"}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    namespaces,
	})
}
