package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"kubepolaris/internal/models"
	"kubepolaris/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// OMService 运维服务
type OMService struct {
	prometheusSvc       *PrometheusService
	monitoringConfigSvc *MonitoringConfigService
}

// NewOMService 创建运维服务
func NewOMService(prometheusSvc *PrometheusService, monitoringConfigSvc *MonitoringConfigService) *OMService {
	return &OMService{
		prometheusSvc:       prometheusSvc,
		monitoringConfigSvc: monitoringConfigSvc,
	}
}

// GetHealthDiagnosis 获取集群健康诊断
func (s *OMService) GetHealthDiagnosis(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) (*models.HealthDiagnosisResponse, error) {
	response := &models.HealthDiagnosisResponse{
		DiagnosisTime:  time.Now().Unix(),
		RiskItems:      []models.RiskItem{},
		Suggestions:    []string{},
		CategoryScores: make(map[string]int),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// 并发执行各项诊断
	// 1. 节点健康诊断
	wg.Add(1)
	go func() {
		defer wg.Done()
		nodeRisks, nodeScore := s.diagnoseNodes(ctx, clientset)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, nodeRisks...)
		response.CategoryScores["node"] = nodeScore
		mu.Unlock()
	}()

	// 2. 工作负载诊断
	wg.Add(1)
	go func() {
		defer wg.Done()
		workloadRisks, workloadScore := s.diagnoseWorkloads(ctx, clientset)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, workloadRisks...)
		response.CategoryScores["workload"] = workloadScore
		mu.Unlock()
	}()

	// 3. 资源诊断
	wg.Add(1)
	go func() {
		defer wg.Done()
		resourceRisks, resourceScore := s.diagnoseResources(ctx, clientset, clusterID)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, resourceRisks...)
		response.CategoryScores["resource"] = resourceScore
		mu.Unlock()
	}()

	// 4. 存储诊断
	wg.Add(1)
	go func() {
		defer wg.Done()
		storageRisks, storageScore := s.diagnoseStorage(ctx, clientset)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, storageRisks...)
		response.CategoryScores["storage"] = storageScore
		mu.Unlock()
	}()

	// 5. 控制面诊断
	wg.Add(1)
	go func() {
		defer wg.Done()
		controlPlaneRisks, controlPlaneScore := s.diagnoseControlPlane(ctx, clientset, clusterID)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, controlPlaneRisks...)
		response.CategoryScores["control_plane"] = controlPlaneScore
		mu.Unlock()
	}()

	wg.Wait()

	// 计算综合健康评分
	response.HealthScore = s.calculateOverallScore(response.CategoryScores)

	// 确定健康状态
	response.Status = s.determineHealthStatus(response.HealthScore, response.RiskItems)

	// 生成诊断建议
	response.Suggestions = s.generateSuggestions(response.RiskItems)

	return response, nil
}

// diagnoseNodes 诊断节点健康状况
func (s *OMService) diagnoseNodes(ctx context.Context, clientset *kubernetes.Clientset) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取节点列表失败", "error", err)
		return risks, 50
	}

	for _, node := range nodes.Items {
		// 检查节点状态
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status != corev1.ConditionTrue {
					risks = append(risks, models.RiskItem{
						ID:          fmt.Sprintf("node-not-ready-%s", node.Name),
						Category:    "node",
						Severity:    "critical",
						Title:       "节点未就绪",
						Description: fmt.Sprintf("节点 %s 处于未就绪状态", node.Name),
						Resource:    node.Name,
						Solution:    "检查节点 kubelet 服务状态，查看节点系统资源使用情况",
					})
					score -= 20
				}
			}
			if condition.Type == corev1.NodeMemoryPressure && condition.Status == corev1.ConditionTrue {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("node-memory-pressure-%s", node.Name),
					Category:    "node",
					Severity:    "warning",
					Title:       "节点内存压力",
					Description: fmt.Sprintf("节点 %s 存在内存压力", node.Name),
					Resource:    node.Name,
					Solution:    "考虑扩容节点内存或迁移部分工作负载",
				})
				score -= 10
			}
			if condition.Type == corev1.NodeDiskPressure && condition.Status == corev1.ConditionTrue {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("node-disk-pressure-%s", node.Name),
					Category:    "node",
					Severity:    "warning",
					Title:       "节点磁盘压力",
					Description: fmt.Sprintf("节点 %s 存在磁盘压力", node.Name),
					Resource:    node.Name,
					Solution:    "清理不需要的镜像和日志，或扩展磁盘容量",
				})
				score -= 10
			}
			if condition.Type == corev1.NodePIDPressure && condition.Status == corev1.ConditionTrue {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("node-pid-pressure-%s", node.Name),
					Category:    "node",
					Severity:    "warning",
					Title:       "节点PID压力",
					Description: fmt.Sprintf("节点 %s 存在PID压力", node.Name),
					Resource:    node.Name,
					Solution:    "检查是否有异常进程，考虑调整 max-pods 参数",
				})
				score -= 10
			}
		}

		// 检查节点是否被标记为不可调度
		if node.Spec.Unschedulable {
			risks = append(risks, models.RiskItem{
				ID:          fmt.Sprintf("node-unschedulable-%s", node.Name),
				Category:    "node",
				Severity:    "info",
				Title:       "节点不可调度",
				Description: fmt.Sprintf("节点 %s 已被标记为不可调度", node.Name),
				Resource:    node.Name,
				Solution:    "如果节点维护已完成，请执行 uncordon 操作",
			})
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}
	return risks, score
}

// diagnoseWorkloads 诊断工作负载状态
func (s *OMService) diagnoseWorkloads(ctx context.Context, clientset *kubernetes.Clientset) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 检查 Deployment
	deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取 Deployment 列表失败", "error", err)
	} else {
		for _, dep := range deployments.Items {
			if dep.Status.Replicas != dep.Status.ReadyReplicas {
				severity := "warning"
				if dep.Status.ReadyReplicas == 0 && *dep.Spec.Replicas > 0 {
					severity = "critical"
					score -= 15
				} else {
					score -= 5
				}
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("deployment-not-ready-%s-%s", dep.Namespace, dep.Name),
					Category:    "workload",
					Severity:    severity,
					Title:       "Deployment 副本未就绪",
					Description: fmt.Sprintf("Deployment %s/%s: %d/%d 副本就绪", dep.Namespace, dep.Name, dep.Status.ReadyReplicas, dep.Status.Replicas),
					Resource:    dep.Name,
					Namespace:   dep.Namespace,
					Solution:    "检查 Pod 事件和日志，确认容器启动失败原因",
				})
			}
		}
	}

	// 检查 StatefulSet
	statefulSets, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取 StatefulSet 列表失败", "error", err)
	} else {
		for _, sts := range statefulSets.Items {
			if sts.Status.Replicas != sts.Status.ReadyReplicas {
				severity := "warning"
				if sts.Status.ReadyReplicas == 0 && *sts.Spec.Replicas > 0 {
					severity = "critical"
					score -= 15
				} else {
					score -= 5
				}
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("statefulset-not-ready-%s-%s", sts.Namespace, sts.Name),
					Category:    "workload",
					Severity:    severity,
					Title:       "StatefulSet 副本未就绪",
					Description: fmt.Sprintf("StatefulSet %s/%s: %d/%d 副本就绪", sts.Namespace, sts.Name, sts.Status.ReadyReplicas, sts.Status.Replicas),
					Resource:    sts.Name,
					Namespace:   sts.Namespace,
					Solution:    "检查 Pod 事件和日志，确认容器启动失败原因",
				})
			}
		}
	}

	// 检查 DaemonSet
	daemonSets, err := clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取 DaemonSet 列表失败", "error", err)
	} else {
		for _, ds := range daemonSets.Items {
			if ds.Status.NumberUnavailable > 0 {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("daemonset-unavailable-%s-%s", ds.Namespace, ds.Name),
					Category:    "workload",
					Severity:    "warning",
					Title:       "DaemonSet 存在不可用副本",
					Description: fmt.Sprintf("DaemonSet %s/%s: %d 个节点上的副本不可用", ds.Namespace, ds.Name, ds.Status.NumberUnavailable),
					Resource:    ds.Name,
					Namespace:   ds.Namespace,
					Solution:    "检查相关节点和 Pod 状态",
				})
				score -= 5
			}
		}
	}

	// 检查异常 Pod
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取 Pod 列表失败", "error", err)
	} else {
		crashLoopCount := 0
		pendingCount := 0
		for _, pod := range pods.Items {
			// 检查 CrashLoopBackOff
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.RestartCount > 5 {
					if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
						crashLoopCount++
						if crashLoopCount <= 5 { // 只报告前5个
							risks = append(risks, models.RiskItem{
								ID:          fmt.Sprintf("pod-crashloop-%s-%s", pod.Namespace, pod.Name),
								Category:    "workload",
								Severity:    "critical",
								Title:       "Pod 持续崩溃重启",
								Description: fmt.Sprintf("Pod %s/%s 容器 %s 已重启 %d 次", pod.Namespace, pod.Name, containerStatus.Name, containerStatus.RestartCount),
								Resource:    pod.Name,
								Namespace:   pod.Namespace,
								Solution:    "检查容器日志，排查应用启动失败原因",
							})
						}
					}
				}
			}

			// 检查长时间 Pending
			if pod.Status.Phase == corev1.PodPending {
				pendingDuration := time.Since(pod.CreationTimestamp.Time)
				if pendingDuration > 5*time.Minute {
					pendingCount++
					if pendingCount <= 5 { // 只报告前5个
						risks = append(risks, models.RiskItem{
							ID:          fmt.Sprintf("pod-pending-%s-%s", pod.Namespace, pod.Name),
							Category:    "workload",
							Severity:    "warning",
							Title:       "Pod 长时间处于 Pending 状态",
							Description: fmt.Sprintf("Pod %s/%s 已 Pending %.0f 分钟", pod.Namespace, pod.Name, pendingDuration.Minutes()),
							Resource:    pod.Name,
							Namespace:   pod.Namespace,
							Solution:    "检查是否资源不足或调度约束过严",
						})
					}
				}
			}
		}
		if crashLoopCount > 0 {
			score -= min(crashLoopCount*5, 30)
		}
		if pendingCount > 0 {
			score -= min(pendingCount*3, 15)
		}
	}

	if score < 0 {
		score = 0
	}
	return risks, score
}

// diagnoseResources 诊断资源使用情况
func (s *OMService) diagnoseResources(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 获取监控配置
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err != nil || config.Type == "disabled" {
		// 如果没有配置监控，通过 K8s API 获取基本信息
		return s.diagnoseResourcesFromK8s(ctx, clientset)
	}

	now := time.Now().Unix()

	// 查询集群 CPU 使用率
	cpuQuery := "(1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m]))) * 100"
	if cpuResp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
		Query: cpuQuery,
		Start: now,
		End:   now,
		Step:  "1m",
	}); err == nil && len(cpuResp.Data.Result) > 0 && len(cpuResp.Data.Result[0].Values) > 0 {
		if val, err := strconv.ParseFloat(fmt.Sprintf("%v", cpuResp.Data.Result[0].Values[0][1]), 64); err == nil {
			if val > 90 {
				risks = append(risks, models.RiskItem{
					ID:          "cluster-cpu-critical",
					Category:    "resource",
					Severity:    "critical",
					Title:       "集群 CPU 使用率过高",
					Description: fmt.Sprintf("集群 CPU 使用率达到 %.1f%%", val),
					Solution:    "考虑扩展节点或优化工作负载",
				})
				score -= 25
			} else if val > 80 {
				risks = append(risks, models.RiskItem{
					ID:          "cluster-cpu-warning",
					Category:    "resource",
					Severity:    "warning",
					Title:       "集群 CPU 使用率较高",
					Description: fmt.Sprintf("集群 CPU 使用率达到 %.1f%%", val),
					Solution:    "关注 CPU 使用趋势，准备扩容计划",
				})
				score -= 10
			}
		}
	}

	// 查询集群内存使用率
	memQuery := "(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100"
	if memResp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
		Query: memQuery,
		Start: now,
		End:   now,
		Step:  "1m",
	}); err == nil && len(memResp.Data.Result) > 0 && len(memResp.Data.Result[0].Values) > 0 {
		if val, err := strconv.ParseFloat(fmt.Sprintf("%v", memResp.Data.Result[0].Values[0][1]), 64); err == nil {
			if val > 90 {
				risks = append(risks, models.RiskItem{
					ID:          "cluster-memory-critical",
					Category:    "resource",
					Severity:    "critical",
					Title:       "集群内存使用率过高",
					Description: fmt.Sprintf("集群内存使用率达到 %.1f%%", val),
					Solution:    "考虑扩展节点内存或优化内存使用",
				})
				score -= 25
			} else if val > 80 {
				risks = append(risks, models.RiskItem{
					ID:          "cluster-memory-warning",
					Category:    "resource",
					Severity:    "warning",
					Title:       "集群内存使用率较高",
					Description: fmt.Sprintf("集群内存使用率达到 %.1f%%", val),
					Solution:    "关注内存使用趋势，准备扩容计划",
				})
				score -= 10
			}
		}
	}

	if score < 0 {
		score = 0
	}
	return risks, score
}

// diagnoseResourcesFromK8s 从 K8s API 诊断资源（无监控数据时）
func (s *OMService) diagnoseResourcesFromK8s(ctx context.Context, clientset *kubernetes.Clientset) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 检查资源配额
	quotas, err := clientset.CoreV1().ResourceQuotas("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, quota := range quotas.Items {
			for resource, used := range quota.Status.Used {
				if hard, ok := quota.Status.Hard[resource]; ok {
					usedVal := used.Value()
					hardVal := hard.Value()
					if hardVal > 0 {
						usageRate := float64(usedVal) / float64(hardVal) * 100
						if usageRate > 90 {
							risks = append(risks, models.RiskItem{
								ID:          fmt.Sprintf("quota-exceeded-%s-%s-%s", quota.Namespace, quota.Name, resource),
								Category:    "resource",
								Severity:    "warning",
								Title:       "资源配额使用率过高",
								Description: fmt.Sprintf("命名空间 %s 资源 %s 使用率达到 %.1f%%", quota.Namespace, resource, usageRate),
								Namespace:   quota.Namespace,
								Resource:    quota.Name,
								Solution:    "考虑提高资源配额或优化资源使用",
							})
							score -= 10
						}
					}
				}
			}
		}
	}

	if score < 0 {
		score = 0
	}
	return risks, score
}

// diagnoseStorage 诊断存储状态
func (s *OMService) diagnoseStorage(ctx context.Context, clientset *kubernetes.Clientset) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 检查 PVC 状态
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取 PVC 列表失败", "error", err)
		return risks, score
	}

	pendingPVCCount := 0
	for _, pvc := range pvcs.Items {
		if pvc.Status.Phase == corev1.ClaimPending {
			pendingPVCCount++
			if pendingPVCCount <= 5 {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("pvc-pending-%s-%s", pvc.Namespace, pvc.Name),
					Category:    "storage",
					Severity:    "warning",
					Title:       "PVC 处于 Pending 状态",
					Description: fmt.Sprintf("PVC %s/%s 无法绑定到 PV", pvc.Namespace, pvc.Name),
					Resource:    pvc.Name,
					Namespace:   pvc.Namespace,
					Solution:    "检查是否有可用的 StorageClass 和足够的存储资源",
				})
			}
		}
		if pvc.Status.Phase == corev1.ClaimLost {
			risks = append(risks, models.RiskItem{
				ID:          fmt.Sprintf("pvc-lost-%s-%s", pvc.Namespace, pvc.Name),
				Category:    "storage",
				Severity:    "critical",
				Title:       "PVC 丢失绑定",
				Description: fmt.Sprintf("PVC %s/%s 已丢失与 PV 的绑定", pvc.Namespace, pvc.Name),
				Resource:    pvc.Name,
				Namespace:   pvc.Namespace,
				Solution:    "检查关联的 PV 状态，可能需要恢复数据",
			})
			score -= 15
		}
	}

	if pendingPVCCount > 0 {
		score -= min(pendingPVCCount*5, 20)
	}

	// 检查 PV 状态
	pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取 PV 列表失败", "error", err)
		return risks, score
	}

	for _, pv := range pvs.Items {
		if pv.Status.Phase == corev1.VolumeFailed {
			risks = append(risks, models.RiskItem{
				ID:          fmt.Sprintf("pv-failed-%s", pv.Name),
				Category:    "storage",
				Severity:    "critical",
				Title:       "PV 状态异常",
				Description: fmt.Sprintf("PV %s 处于 Failed 状态", pv.Name),
				Resource:    pv.Name,
				Solution:    "检查存储后端状态和网络连接",
			})
			score -= 15
		}
	}

	if score < 0 {
		score = 0
	}
	return risks, score
}

// diagnoseControlPlane 诊断控制面组件
func (s *OMService) diagnoseControlPlane(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 检查 kube-system 命名空间下的核心组件
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取 kube-system Pod 列表失败", "error", err)
		return risks, 50
	}

	components := map[string]bool{
		"kube-apiserver":          false,
		"kube-controller-manager": false,
		"kube-scheduler":          false,
		"etcd":                    false,
	}

	for _, pod := range pods.Items {
		for component := range components {
			if strings.Contains(pod.Name, component) {
				if pod.Status.Phase == corev1.PodRunning {
					components[component] = true
				} else {
					risks = append(risks, models.RiskItem{
						ID:          fmt.Sprintf("control-plane-%s-unhealthy", component),
						Category:    "control_plane",
						Severity:    "critical",
						Title:       fmt.Sprintf("控制面组件 %s 不健康", component),
						Description: fmt.Sprintf("组件 %s (Pod: %s) 状态: %s", component, pod.Name, pod.Status.Phase),
						Resource:    pod.Name,
						Namespace:   "kube-system",
						Solution:    "检查组件日志和配置",
					})
					score -= 20
				}
				break
			}
		}
	}

	// 检查是否缺少核心组件
	for component, found := range components {
		if !found {
			// 可能是托管集群，控制面不可见，不算风险
			logger.Info("未找到控制面组件", "component", component)
		}
	}

	// 通过 Prometheus 检查 etcd 和 apiserver 指标
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err == nil && config.Type != "disabled" {
		now := time.Now().Unix()

		// 检查 etcd leader 状态
		etcdQuery := "etcd_server_has_leader"
		if etcdResp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: etcdQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil && len(etcdResp.Data.Result) > 0 {
			hasLeader := false
			for _, result := range etcdResp.Data.Result {
				if len(result.Values) > 0 {
					if val, err := strconv.ParseFloat(fmt.Sprintf("%v", result.Values[0][1]), 64); err == nil && val == 1 {
						hasLeader = true
						break
					}
				}
			}
			if !hasLeader {
				risks = append(risks, models.RiskItem{
					ID:          "etcd-no-leader",
					Category:    "control_plane",
					Severity:    "critical",
					Title:       "Etcd 无 Leader",
					Description: "Etcd 集群当前没有 Leader，集群可能无法正常工作",
					Solution:    "检查 etcd 集群健康状态和网络连接",
				})
				score -= 30
			}
		}

		// 检查 apiserver 错误率
		apiErrorQuery := "sum(rate(apiserver_request_total{code=~\"5..\"}[5m])) / sum(rate(apiserver_request_total[5m])) * 100"
		if apiResp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: apiErrorQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil && len(apiResp.Data.Result) > 0 && len(apiResp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", apiResp.Data.Result[0].Values[0][1]), 64); err == nil {
				if val > 5 {
					risks = append(risks, models.RiskItem{
						ID:          "apiserver-high-error-rate",
						Category:    "control_plane",
						Severity:    "warning",
						Title:       "API Server 错误率较高",
						Description: fmt.Sprintf("API Server 5xx 错误率达到 %.1f%%", val),
						Solution:    "检查 apiserver 日志和后端 etcd 状态",
					})
					score -= 15
				}
			}
		}
	}

	if score < 0 {
		score = 0
	}
	return risks, score
}

// calculateOverallScore 计算综合健康评分
func (s *OMService) calculateOverallScore(categoryScores map[string]int) int {
	if len(categoryScores) == 0 {
		return 100
	}

	// 加权平均，控制面和节点权重更高
	weights := map[string]float64{
		"node":          0.25,
		"workload":      0.20,
		"resource":      0.20,
		"storage":       0.15,
		"control_plane": 0.20,
	}

	var totalWeight float64
	var weightedSum float64

	for category, score := range categoryScores {
		weight := weights[category]
		if weight == 0 {
			weight = 0.1
		}
		totalWeight += weight
		weightedSum += float64(score) * weight
	}

	if totalWeight == 0 {
		return 100
	}

	return int(weightedSum / totalWeight)
}

// determineHealthStatus 确定健康状态
func (s *OMService) determineHealthStatus(score int, risks []models.RiskItem) string {
	// 统计严重问题数量
	criticalCount := 0
	for _, risk := range risks {
		if risk.Severity == "critical" {
			criticalCount++
		}
	}

	if criticalCount > 0 || score < 60 {
		return "critical"
	} else if score < 80 {
		return "warning"
	}
	return "healthy"
}

// generateSuggestions 生成诊断建议
func (s *OMService) generateSuggestions(risks []models.RiskItem) []string {
	suggestions := []string{}
	categoryCount := make(map[string]int)

	for _, risk := range risks {
		categoryCount[risk.Category]++
	}

	if categoryCount["node"] > 0 {
		suggestions = append(suggestions, "建议检查节点健康状态，确保所有节点资源充足且服务正常")
	}
	if categoryCount["workload"] > 0 {
		suggestions = append(suggestions, "建议检查工作负载状态，排查 Pod 启动失败或持续重启的原因")
	}
	if categoryCount["resource"] > 0 {
		suggestions = append(suggestions, "建议关注资源使用趋势，考虑扩容或优化资源配置")
	}
	if categoryCount["storage"] > 0 {
		suggestions = append(suggestions, "建议检查存储系统状态，确保 PV/PVC 正常绑定")
	}
	if categoryCount["control_plane"] > 0 {
		suggestions = append(suggestions, "建议检查控制面组件健康状态，确保集群核心功能正常")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "集群整体运行健康，建议定期进行健康检查以预防问题")
	}

	return suggestions
}

// GetResourceTop 获取资源消耗 Top N
func (s *OMService) GetResourceTop(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint, req *models.ResourceTopRequest) (*models.ResourceTopResponse, error) {
	response := &models.ResourceTopResponse{
		Type:      req.Type,
		Level:     req.Level,
		Items:     []models.ResourceTopItem{},
		QueryTime: time.Now().Unix(),
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	// 获取监控配置
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err != nil || config.Type == "disabled" {
		// 没有监控数据，从 K8s 获取基本信息
		return s.getResourceTopFromK8s(ctx, clientset, req, limit)
	}

	now := time.Now().Unix()

	// 根据资源类型和级别构建查询
	var query string
	var unit string

	switch req.Type {
	case "cpu":
		unit = "cores"
		switch req.Level {
		case "namespace":
			query = "topk(100, sum(rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\"}[5m])) by (namespace))"
		case "workload":
			query = "topk(100, sum(rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\"}[5m])) by (namespace, pod))"
		case "pod":
			query = "topk(100, sum(rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\"}[5m])) by (namespace, pod))"
		}
	case "memory":
		unit = "bytes"
		switch req.Level {
		case "namespace":
			query = "topk(100, sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\"}) by (namespace))"
		case "workload":
			query = "topk(100, sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\"}) by (namespace, pod))"
		case "pod":
			query = "topk(100, sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\"}) by (namespace, pod))"
		}
	case "network":
		unit = "bytes/s"
		switch req.Level {
		case "namespace":
			query = "topk(100, sum(rate(container_network_receive_bytes_total[5m]) + rate(container_network_transmit_bytes_total[5m])) by (namespace))"
		case "workload":
			query = "topk(100, sum(rate(container_network_receive_bytes_total[5m]) + rate(container_network_transmit_bytes_total[5m])) by (namespace, pod))"
		case "pod":
			query = "topk(100, sum(rate(container_network_receive_bytes_total[5m]) + rate(container_network_transmit_bytes_total[5m])) by (namespace, pod))"
		}
	case "disk":
		unit = "bytes"
		switch req.Level {
		case "namespace":
			query = "topk(100, sum(container_fs_usage_bytes{container!=\"\",container!=\"POD\"}) by (namespace))"
		case "workload":
			query = "topk(100, sum(container_fs_usage_bytes{container!=\"\",container!=\"POD\"}) by (namespace, pod))"
		case "pod":
			query = "topk(100, sum(container_fs_usage_bytes{container!=\"\",container!=\"POD\"}) by (namespace, pod))"
		}
	}

	resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
		Query: query,
		Start: now,
		End:   now,
		Step:  "1m",
	})
	if err != nil {
		logger.Error("查询资源 Top N 失败", "error", err)
		return response, nil
	}

	// 解析结果
	type resultItem struct {
		name      string
		namespace string
		usage     float64
	}
	var items []resultItem

	for _, result := range resp.Data.Result {
		if len(result.Values) == 0 {
			continue
		}

		name := ""
		namespace := ""

		if ns, ok := result.Metric["namespace"]; ok {
			namespace = ns
		}
		if pod, ok := result.Metric["pod"]; ok {
			name = pod
		} else if namespace != "" {
			name = namespace
		}

		if name == "" {
			continue
		}

		val, err := strconv.ParseFloat(fmt.Sprintf("%v", result.Values[0][1]), 64)
		if err != nil {
			continue
		}

		items = append(items, resultItem{
			name:      name,
			namespace: namespace,
			usage:     val,
		})
	}

	// 按使用量排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].usage > items[j].usage
	})

	// 取 Top N
	for i := 0; i < len(items) && i < limit; i++ {
		item := items[i]
		topItem := models.ResourceTopItem{
			Rank:      i + 1,
			Name:      item.name,
			Namespace: item.namespace,
			Usage:     item.usage,
			Unit:      unit,
		}

		// 计算使用率（如果有 limit 数据）
		// 这里简化处理，可以后续扩展查询 limit 数据
		response.Items = append(response.Items, topItem)
	}

	return response, nil
}

// getResourceTopFromK8s 从 K8s 获取资源 Top N（无监控数据时）
func (s *OMService) getResourceTopFromK8s(ctx context.Context, clientset *kubernetes.Clientset, req *models.ResourceTopRequest, limit int) (*models.ResourceTopResponse, error) {
	response := &models.ResourceTopResponse{
		Type:      req.Type,
		Level:     req.Level,
		Items:     []models.ResourceTopItem{},
		QueryTime: time.Now().Unix(),
	}

	// 获取 Pod 列表
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return response, err
	}

	type usageData struct {
		name      string
		namespace string
		request   int64
		limit     int64
	}

	var items []usageData

	switch req.Type {
	case "cpu", "memory":
		resourceName := corev1.ResourceCPU
		if req.Type == "memory" {
			resourceName = corev1.ResourceMemory
		}

		switch req.Level {
		case "namespace":
			nsUsage := make(map[string]*usageData)
			for _, pod := range pods.Items {
				if _, ok := nsUsage[pod.Namespace]; !ok {
					nsUsage[pod.Namespace] = &usageData{
						name:      pod.Namespace,
						namespace: pod.Namespace,
					}
				}
				for _, container := range pod.Spec.Containers {
					if req := container.Resources.Requests[resourceName]; !req.IsZero() {
						nsUsage[pod.Namespace].request += req.Value()
					}
					if lim := container.Resources.Limits[resourceName]; !lim.IsZero() {
						nsUsage[pod.Namespace].limit += lim.Value()
					}
				}
			}
			for _, v := range nsUsage {
				items = append(items, *v)
			}

		case "pod":
			for _, pod := range pods.Items {
				item := usageData{
					name:      pod.Name,
					namespace: pod.Namespace,
				}
				for _, container := range pod.Spec.Containers {
					if req := container.Resources.Requests[resourceName]; !req.IsZero() {
						item.request += req.Value()
					}
					if lim := container.Resources.Limits[resourceName]; !lim.IsZero() {
						item.limit += lim.Value()
					}
				}
				if item.request > 0 || item.limit > 0 {
					items = append(items, item)
				}
			}
		}
	}

	// 按 request 值排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].request > items[j].request
	})

	// 取 Top N
	unit := "cores"
	if req.Type == "memory" {
		unit = "bytes"
	}

	for i := 0; i < len(items) && i < limit; i++ {
		item := items[i]
		topItem := models.ResourceTopItem{
			Rank:      i + 1,
			Name:      item.name,
			Namespace: item.namespace,
			Request:   float64(item.request),
			Limit:     float64(item.limit),
			Usage:     float64(item.request), // 无监控数据时用 request 代替
			Unit:      unit,
		}
		response.Items = append(response.Items, topItem)
	}

	return response, nil
}

// GetControlPlaneStatus 获取控制面组件状态
func (s *OMService) GetControlPlaneStatus(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) (*models.ControlPlaneStatusResponse, error) {
	response := &models.ControlPlaneStatusResponse{
		Overall:    "healthy",
		Components: []models.ControlPlaneComponent{},
		CheckTime:  time.Now().Unix(),
	}

	// 获取 kube-system 命名空间下的 Pod
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("获取 kube-system Pod 列表失败", "error", err)
		return response, nil
	}

	// 定义要检查的控制面组件
	componentTypes := []string{"kube-apiserver", "kube-scheduler", "kube-controller-manager", "etcd"}

	componentsMap := make(map[string]*models.ControlPlaneComponent)

	for _, componentType := range componentTypes {
		componentsMap[componentType] = &models.ControlPlaneComponent{
			Name:          componentType,
			Type:          strings.TrimPrefix(componentType, "kube-"),
			Status:        "unknown",
			Message:       "未检测到该组件",
			LastCheckTime: time.Now().Unix(),
			Instances:     []models.ComponentInstance{},
		}
	}

	// 遍历 Pod，匹配控制面组件
	for _, pod := range pods.Items {
		for _, componentType := range componentTypes {
			if strings.Contains(pod.Name, componentType) {
				component := componentsMap[componentType]

				instance := models.ComponentInstance{
					Name:   pod.Name,
					Node:   pod.Spec.NodeName,
					Status: string(pod.Status.Phase),
					IP:     pod.Status.PodIP,
				}
				if pod.Status.StartTime != nil {
					instance.StartTime = pod.Status.StartTime.Unix()
				}
				component.Instances = append(component.Instances, instance)

				// 更新组件整体状态
				if pod.Status.Phase == corev1.PodRunning {
					allReady := true
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status != corev1.ConditionTrue {
							allReady = false
							break
						}
					}
					if allReady {
						component.Status = "healthy"
						component.Message = "组件运行正常"
					} else {
						component.Status = "unhealthy"
						component.Message = "组件 Pod 未就绪"
					}
				} else {
					component.Status = "unhealthy"
					component.Message = fmt.Sprintf("组件 Pod 状态: %s", pod.Status.Phase)
				}
				break
			}
		}
	}

	// 获取监控配置，查询组件指标
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err == nil && config.Type != "disabled" {
		s.enrichComponentMetrics(ctx, config, componentsMap)
	}

	// 组装响应
	unhealthyCount := 0
	for _, component := range componentsMap {
		if component.Status == "unhealthy" {
			unhealthyCount++
		}
		response.Components = append(response.Components, *component)
	}

	// 确定整体状态
	if unhealthyCount > 0 {
		if unhealthyCount >= len(componentTypes)/2 {
			response.Overall = "unhealthy"
		} else {
			response.Overall = "degraded"
		}
	}

	return response, nil
}

// enrichComponentMetrics 从 Prometheus 获取组件指标
func (s *OMService) enrichComponentMetrics(ctx context.Context, config *models.MonitoringConfig, componentsMap map[string]*models.ControlPlaneComponent) {
	now := time.Now().Unix()

	// API Server 指标
	if apiserver, ok := componentsMap["kube-apiserver"]; ok {
		apiserver.Metrics = &models.ComponentMetrics{}

		// 请求速率
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(rate(apiserver_request_total[5m]))",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.RequestRate = val
			}
		}

		// 错误率
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(rate(apiserver_request_total{code=~\"5..\"}[5m])) / sum(rate(apiserver_request_total[5m])) * 100",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.ErrorRate = val
			}
		}

		// 延迟
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "histogram_quantile(0.99, sum(rate(apiserver_request_duration_seconds_bucket[5m])) by (le)) * 1000",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.Latency = val
			}
		}
	}

	// Etcd 指标
	if etcd, ok := componentsMap["etcd"]; ok {
		etcd.Metrics = &models.ComponentMetrics{}

		// Leader 状态
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "max(etcd_server_has_leader)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.LeaderStatus = val == 1
			}
		}

		// 数据库大小
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(etcd_mvcc_db_total_size_in_bytes)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.DBSize = val
			}
		}

		// 成员数量
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "count(etcd_server_has_leader)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.MemberCount = int(val)
			}
		}
	}

	// Scheduler 指标
	if scheduler, ok := componentsMap["kube-scheduler"]; ok {
		scheduler.Metrics = &models.ComponentMetrics{}

		// 队列长度
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(scheduler_pending_pods)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				scheduler.Metrics.QueueLength = int(val)
			}
		}
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
