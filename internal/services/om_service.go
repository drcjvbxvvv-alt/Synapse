package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// OMService 運維服務
type OMService struct {
	prometheusSvc       *PrometheusService
	monitoringConfigSvc *MonitoringConfigService
}

// NewOMService 建立運維服務
func NewOMService(prometheusSvc *PrometheusService, monitoringConfigSvc *MonitoringConfigService) *OMService {
	return &OMService{
		prometheusSvc:       prometheusSvc,
		monitoringConfigSvc: monitoringConfigSvc,
	}
}

// GetHealthDiagnosis 獲取叢集健康診斷
func (s *OMService) GetHealthDiagnosis(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) (*models.HealthDiagnosisResponse, error) {
	response := &models.HealthDiagnosisResponse{
		DiagnosisTime:  time.Now().Unix(),
		RiskItems:      []models.RiskItem{},
		Suggestions:    []string{},
		CategoryScores: make(map[string]int),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// 併發執行各項診斷
	// 1. 節點健康診斷
	wg.Add(1)
	go func() {
		defer wg.Done()
		nodeRisks, nodeScore := s.diagnoseNodes(ctx, clientset)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, nodeRisks...)
		response.CategoryScores["node"] = nodeScore
		mu.Unlock()
	}()

	// 2. 工作負載診斷
	wg.Add(1)
	go func() {
		defer wg.Done()
		workloadRisks, workloadScore := s.diagnoseWorkloads(ctx, clientset)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, workloadRisks...)
		response.CategoryScores["workload"] = workloadScore
		mu.Unlock()
	}()

	// 3. 資源診斷
	wg.Add(1)
	go func() {
		defer wg.Done()
		resourceRisks, resourceScore := s.diagnoseResources(ctx, clientset, clusterID)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, resourceRisks...)
		response.CategoryScores["resource"] = resourceScore
		mu.Unlock()
	}()

	// 4. 儲存診斷
	wg.Add(1)
	go func() {
		defer wg.Done()
		storageRisks, storageScore := s.diagnoseStorage(ctx, clientset)
		mu.Lock()
		response.RiskItems = append(response.RiskItems, storageRisks...)
		response.CategoryScores["storage"] = storageScore
		mu.Unlock()
	}()

	// 5. 控制面診斷
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

	// 計算綜合健康評分
	response.HealthScore = s.calculateOverallScore(response.CategoryScores)

	// 確定健康狀態
	response.Status = s.determineHealthStatus(response.HealthScore, response.RiskItems)

	// 生成診斷建議
	response.Suggestions = s.generateSuggestions(response.RiskItems)

	return response, nil
}

// diagnoseNodes 診斷節點健康狀況
func (s *OMService) diagnoseNodes(ctx context.Context, clientset *kubernetes.Clientset) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取節點列表失敗", "error", err)
		return risks, 50
	}

	for _, node := range nodes.Items {
		// 檢查節點狀態
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status != corev1.ConditionTrue {
					risks = append(risks, models.RiskItem{
						ID:          fmt.Sprintf("node-not-ready-%s", node.Name),
						Category:    "node",
						Severity:    "critical",
						Title:       "節點未就緒",
						Description: fmt.Sprintf("節點 %s 處於未就緒狀態", node.Name),
						Resource:    node.Name,
						Solution:    "檢查節點 kubelet 服務狀態，檢視節點系統資源使用情況",
					})
					score -= 20
				}
			}
			if condition.Type == corev1.NodeMemoryPressure && condition.Status == corev1.ConditionTrue {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("node-memory-pressure-%s", node.Name),
					Category:    "node",
					Severity:    "warning",
					Title:       "節點記憶體壓力",
					Description: fmt.Sprintf("節點 %s 存在記憶體壓力", node.Name),
					Resource:    node.Name,
					Solution:    "考慮擴容節點記憶體或遷移部分工作負載",
				})
				score -= 10
			}
			if condition.Type == corev1.NodeDiskPressure && condition.Status == corev1.ConditionTrue {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("node-disk-pressure-%s", node.Name),
					Category:    "node",
					Severity:    "warning",
					Title:       "節點磁碟壓力",
					Description: fmt.Sprintf("節點 %s 存在磁碟壓力", node.Name),
					Resource:    node.Name,
					Solution:    "清理不需要的映像和日誌，或擴充套件磁碟容量",
				})
				score -= 10
			}
			if condition.Type == corev1.NodePIDPressure && condition.Status == corev1.ConditionTrue {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("node-pid-pressure-%s", node.Name),
					Category:    "node",
					Severity:    "warning",
					Title:       "節點PID壓力",
					Description: fmt.Sprintf("節點 %s 存在PID壓力", node.Name),
					Resource:    node.Name,
					Solution:    "檢查是否有異常程序，考慮調整 max-pods 參數",
				})
				score -= 10
			}
		}

		// 檢查節點是否被標記為不可排程
		if node.Spec.Unschedulable {
			risks = append(risks, models.RiskItem{
				ID:          fmt.Sprintf("node-unschedulable-%s", node.Name),
				Category:    "node",
				Severity:    "info",
				Title:       "節點不可排程",
				Description: fmt.Sprintf("節點 %s 已被標記為不可排程", node.Name),
				Resource:    node.Name,
				Solution:    "如果節點維護已完成，請執行 uncordon 操作",
			})
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}
	return risks, score
}

// diagnoseWorkloads 診斷工作負載狀態
func (s *OMService) diagnoseWorkloads(ctx context.Context, clientset *kubernetes.Clientset) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 檢查 Deployment
	deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 Deployment 列表失敗", "error", err)
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
					Title:       "Deployment 副本未就緒",
					Description: fmt.Sprintf("Deployment %s/%s: %d/%d 副本就緒", dep.Namespace, dep.Name, dep.Status.ReadyReplicas, dep.Status.Replicas),
					Resource:    dep.Name,
					Namespace:   dep.Namespace,
					Solution:    "檢查 Pod 事件和日誌，確認容器啟動失敗原因",
				})
			}
		}
	}

	// 檢查 StatefulSet
	statefulSets, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 StatefulSet 列表失敗", "error", err)
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
					Title:       "StatefulSet 副本未就緒",
					Description: fmt.Sprintf("StatefulSet %s/%s: %d/%d 副本就緒", sts.Namespace, sts.Name, sts.Status.ReadyReplicas, sts.Status.Replicas),
					Resource:    sts.Name,
					Namespace:   sts.Namespace,
					Solution:    "檢查 Pod 事件和日誌，確認容器啟動失敗原因",
				})
			}
		}
	}

	// 檢查 DaemonSet
	daemonSets, err := clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 DaemonSet 列表失敗", "error", err)
	} else {
		for _, ds := range daemonSets.Items {
			if ds.Status.NumberUnavailable > 0 {
				risks = append(risks, models.RiskItem{
					ID:          fmt.Sprintf("daemonset-unavailable-%s-%s", ds.Namespace, ds.Name),
					Category:    "workload",
					Severity:    "warning",
					Title:       "DaemonSet 存在不可用副本",
					Description: fmt.Sprintf("DaemonSet %s/%s: %d 個節點上的副本不可用", ds.Namespace, ds.Name, ds.Status.NumberUnavailable),
					Resource:    ds.Name,
					Namespace:   ds.Namespace,
					Solution:    "檢查相關節點和 Pod 狀態",
				})
				score -= 5
			}
		}
	}

	// 檢查異常 Pod
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 Pod 列表失敗", "error", err)
	} else {
		crashLoopCount := 0
		pendingCount := 0
		for _, pod := range pods.Items {
			// 檢查 CrashLoopBackOff
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.RestartCount > 5 {
					if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
						crashLoopCount++
						if crashLoopCount <= 5 { // 只報告前5個
							risks = append(risks, models.RiskItem{
								ID:          fmt.Sprintf("pod-crashloop-%s-%s", pod.Namespace, pod.Name),
								Category:    "workload",
								Severity:    "critical",
								Title:       "Pod 持續崩潰重啟",
								Description: fmt.Sprintf("Pod %s/%s 容器 %s 已重啟 %d 次", pod.Namespace, pod.Name, containerStatus.Name, containerStatus.RestartCount),
								Resource:    pod.Name,
								Namespace:   pod.Namespace,
								Solution:    "檢查容器日誌，排查應用啟動失敗原因",
							})
						}
					}
				}
			}

			// 檢查長時間 Pending
			if pod.Status.Phase == corev1.PodPending {
				pendingDuration := time.Since(pod.CreationTimestamp.Time)
				if pendingDuration > 5*time.Minute {
					pendingCount++
					if pendingCount <= 5 { // 只報告前5個
						risks = append(risks, models.RiskItem{
							ID:          fmt.Sprintf("pod-pending-%s-%s", pod.Namespace, pod.Name),
							Category:    "workload",
							Severity:    "warning",
							Title:       "Pod 長時間處於 Pending 狀態",
							Description: fmt.Sprintf("Pod %s/%s 已 Pending %.0f 分鐘", pod.Namespace, pod.Name, pendingDuration.Minutes()),
							Resource:    pod.Name,
							Namespace:   pod.Namespace,
							Solution:    "檢查是否資源不足或排程約束過嚴",
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

// diagnoseResources 診斷資源使用情況
func (s *OMService) diagnoseResources(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 獲取監控配置
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err != nil || config.Type == "disabled" {
		// 如果沒有配置監控，透過 K8s API 獲取基本資訊
		return s.diagnoseResourcesFromK8s(ctx, clientset)
	}

	now := time.Now().Unix()

	// 查詢叢集 CPU 使用率
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
					Title:       "叢集 CPU 使用率過高",
					Description: fmt.Sprintf("叢集 CPU 使用率達到 %.1f%%", val),
					Solution:    "考慮擴充套件節點或最佳化工作負載",
				})
				score -= 25
			} else if val > 80 {
				risks = append(risks, models.RiskItem{
					ID:          "cluster-cpu-warning",
					Category:    "resource",
					Severity:    "warning",
					Title:       "叢集 CPU 使用率較高",
					Description: fmt.Sprintf("叢集 CPU 使用率達到 %.1f%%", val),
					Solution:    "關注 CPU 使用趨勢，準備擴容計劃",
				})
				score -= 10
			}
		}
	}

	// 查詢叢集記憶體使用率
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
					Title:       "叢集記憶體使用率過高",
					Description: fmt.Sprintf("叢集記憶體使用率達到 %.1f%%", val),
					Solution:    "考慮擴充套件節點記憶體或最佳化記憶體使用",
				})
				score -= 25
			} else if val > 80 {
				risks = append(risks, models.RiskItem{
					ID:          "cluster-memory-warning",
					Category:    "resource",
					Severity:    "warning",
					Title:       "叢集記憶體使用率較高",
					Description: fmt.Sprintf("叢集記憶體使用率達到 %.1f%%", val),
					Solution:    "關注記憶體使用趨勢，準備擴容計劃",
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

// diagnoseResourcesFromK8s 從 K8s API 診斷資源（無監控資料時）
func (s *OMService) diagnoseResourcesFromK8s(ctx context.Context, clientset *kubernetes.Clientset) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 檢查資源配額
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
								Title:       "資源配額使用率過高",
								Description: fmt.Sprintf("命名空間 %s 資源 %s 使用率達到 %.1f%%", quota.Namespace, resource, usageRate),
								Namespace:   quota.Namespace,
								Resource:    quota.Name,
								Solution:    "考慮提高資源配額或最佳化資源使用",
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

// diagnoseStorage 診斷儲存狀態
func (s *OMService) diagnoseStorage(ctx context.Context, clientset *kubernetes.Clientset) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 檢查 PVC 狀態
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 PVC 列表失敗", "error", err)
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
					Title:       "PVC 處於 Pending 狀態",
					Description: fmt.Sprintf("PVC %s/%s 無法繫結到 PV", pvc.Namespace, pvc.Name),
					Resource:    pvc.Name,
					Namespace:   pvc.Namespace,
					Solution:    "檢查是否有可用的 StorageClass 和足夠的儲存資源",
				})
			}
		}
		if pvc.Status.Phase == corev1.ClaimLost {
			risks = append(risks, models.RiskItem{
				ID:          fmt.Sprintf("pvc-lost-%s-%s", pvc.Namespace, pvc.Name),
				Category:    "storage",
				Severity:    "critical",
				Title:       "PVC 丟失繫結",
				Description: fmt.Sprintf("PVC %s/%s 已丟失與 PV 的繫結", pvc.Namespace, pvc.Name),
				Resource:    pvc.Name,
				Namespace:   pvc.Namespace,
				Solution:    "檢查關聯的 PV 狀態，可能需要恢復資料",
			})
			score -= 15
		}
	}

	if pendingPVCCount > 0 {
		score -= min(pendingPVCCount*5, 20)
	}

	// 檢查 PV 狀態
	pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 PV 列表失敗", "error", err)
		return risks, score
	}

	for _, pv := range pvs.Items {
		if pv.Status.Phase == corev1.VolumeFailed {
			risks = append(risks, models.RiskItem{
				ID:          fmt.Sprintf("pv-failed-%s", pv.Name),
				Category:    "storage",
				Severity:    "critical",
				Title:       "PV 狀態異常",
				Description: fmt.Sprintf("PV %s 處於 Failed 狀態", pv.Name),
				Resource:    pv.Name,
				Solution:    "檢查儲存後端狀態和網路連線",
			})
			score -= 15
		}
	}

	if score < 0 {
		score = 0
	}
	return risks, score
}

// diagnoseControlPlane 診斷控制面元件
func (s *OMService) diagnoseControlPlane(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) ([]models.RiskItem, int) {
	risks := []models.RiskItem{}
	score := 100

	// 檢查 kube-system 命名空間下的核心元件
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 kube-system Pod 列表失敗", "error", err)
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
						Title:       fmt.Sprintf("控制面元件 %s 不健康", component),
						Description: fmt.Sprintf("元件 %s (Pod: %s) 狀態: %s", component, pod.Name, pod.Status.Phase),
						Resource:    pod.Name,
						Namespace:   "kube-system",
						Solution:    "檢查元件日誌和配置",
					})
					score -= 20
				}
				break
			}
		}
	}

	// 檢查是否缺少核心元件
	for component, found := range components {
		if !found {
			// 可能是託管叢集，控制面不可見，不算風險
			logger.Info("未找到控制面元件", "component", component)
		}
	}

	// 透過 Prometheus 檢查 etcd 和 apiserver 指標
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err == nil && config.Type != "disabled" {
		now := time.Now().Unix()

		// 檢查 etcd leader 狀態
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
					Title:       "Etcd 無 Leader",
					Description: "Etcd 叢集當前沒有 Leader，叢集可能無法正常工作",
					Solution:    "檢查 etcd 叢集健康狀態和網路連線",
				})
				score -= 30
			}
		}

		// 檢查 apiserver 錯誤率
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
						Title:       "API Server 錯誤率較高",
						Description: fmt.Sprintf("API Server 5xx 錯誤率達到 %.1f%%", val),
						Solution:    "檢查 apiserver 日誌和後端 etcd 狀態",
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

// calculateOverallScore 計算綜合健康評分
func (s *OMService) calculateOverallScore(categoryScores map[string]int) int {
	if len(categoryScores) == 0 {
		return 100
	}

	// 加權平均，控制面和節點權重更高
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

// determineHealthStatus 確定健康狀態
func (s *OMService) determineHealthStatus(score int, risks []models.RiskItem) string {
	// 統計嚴重問題數量
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

// generateSuggestions 生成診斷建議
func (s *OMService) generateSuggestions(risks []models.RiskItem) []string {
	suggestions := []string{}
	categoryCount := make(map[string]int)

	for _, risk := range risks {
		categoryCount[risk.Category]++
	}

	if categoryCount["node"] > 0 {
		suggestions = append(suggestions, "建議檢查節點健康狀態，確保所有節點資源充足且服務正常")
	}
	if categoryCount["workload"] > 0 {
		suggestions = append(suggestions, "建議檢查工作負載狀態，排查 Pod 啟動失敗或持續重啟的原因")
	}
	if categoryCount["resource"] > 0 {
		suggestions = append(suggestions, "建議關注資源使用趨勢，考慮擴容或最佳化資源配置")
	}
	if categoryCount["storage"] > 0 {
		suggestions = append(suggestions, "建議檢查儲存系統狀態，確保 PV/PVC 正常繫結")
	}
	if categoryCount["control_plane"] > 0 {
		suggestions = append(suggestions, "建議檢查控制面元件健康狀態，確保叢集核心功能正常")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "叢集整體執行健康，建議定期進行健康檢查以預防問題")
	}

	return suggestions
}

// GetResourceTop 獲取資源消耗 Top N
