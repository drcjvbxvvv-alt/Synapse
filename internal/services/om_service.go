package services

import (
	"context"
	"fmt"
	"sort"
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

	// 獲取監控配置
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err != nil || config.Type == "disabled" {
		// 沒有監控資料，從 K8s 獲取基本資訊
		return s.getResourceTopFromK8s(ctx, clientset, req, limit)
	}

	now := time.Now().Unix()

	// 根據資源型別和級別構建查詢
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
		logger.Error("查詢資源 Top N 失敗", "error", err)
		return response, nil
	}

	// 解析結果
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

		// 計算使用率（如果有 limit 資料）
		// 這裡簡化處理，可以後續擴充套件查詢 limit 資料
		response.Items = append(response.Items, topItem)
	}

	return response, nil
}

// getResourceTopFromK8s 從 K8s 獲取資源 Top N（無監控資料時）
func (s *OMService) getResourceTopFromK8s(ctx context.Context, clientset *kubernetes.Clientset, req *models.ResourceTopRequest, limit int) (*models.ResourceTopResponse, error) {
	response := &models.ResourceTopResponse{
		Type:      req.Type,
		Level:     req.Level,
		Items:     []models.ResourceTopItem{},
		QueryTime: time.Now().Unix(),
	}

	// 獲取 Pod 列表
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
			Usage:     float64(item.request), // 無監控資料時用 request 代替
			Unit:      unit,
		}
		response.Items = append(response.Items, topItem)
	}

	return response, nil
}

// GetControlPlaneStatus 獲取控制面元件狀態
func (s *OMService) GetControlPlaneStatus(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) (*models.ControlPlaneStatusResponse, error) {
	response := &models.ControlPlaneStatusResponse{
		Overall:    "healthy",
		Components: []models.ControlPlaneComponent{},
		CheckTime:  time.Now().Unix(),
	}

	// 獲取 kube-system 命名空間下的 Pod
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 kube-system Pod 列表失敗", "error", err)
		return response, nil
	}

	// 定義要檢查的控制面元件
	componentTypes := []string{"kube-apiserver", "kube-scheduler", "kube-controller-manager", "etcd"}

	componentsMap := make(map[string]*models.ControlPlaneComponent)

	for _, componentType := range componentTypes {
		componentsMap[componentType] = &models.ControlPlaneComponent{
			Name:          componentType,
			Type:          strings.TrimPrefix(componentType, "kube-"),
			Status:        "unknown",
			Message:       "未檢測到該元件",
			LastCheckTime: time.Now().Unix(),
			Instances:     []models.ComponentInstance{},
		}
	}

	// 遍歷 Pod，匹配控制面元件
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

				// 更新元件整體狀態
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
						component.Message = "元件執行正常"
					} else {
						component.Status = "unhealthy"
						component.Message = "元件 Pod 未就緒"
					}
				} else {
					component.Status = "unhealthy"
					component.Message = fmt.Sprintf("元件 Pod 狀態: %s", pod.Status.Phase)
				}
				break
			}
		}
	}

	// 獲取監控配置，查詢元件指標
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err == nil && config.Type != "disabled" {
		s.enrichComponentMetrics(ctx, config, componentsMap)
	}

	// 組裝響應
	unhealthyCount := 0
	for _, component := range componentsMap {
		if component.Status == "unhealthy" {
			unhealthyCount++
		}
		response.Components = append(response.Components, *component)
	}

	// 確定整體狀態
	if unhealthyCount > 0 {
		if unhealthyCount >= len(componentTypes)/2 {
			response.Overall = "unhealthy"
		} else {
			response.Overall = "degraded"
		}
	}

	return response, nil
}

// enrichComponentMetrics 從 Prometheus 獲取元件指標
func (s *OMService) enrichComponentMetrics(ctx context.Context, config *models.MonitoringConfig, componentsMap map[string]*models.ControlPlaneComponent) {
	now := time.Now().Unix()

	// API Server 指標
	if apiserver, ok := componentsMap["kube-apiserver"]; ok {
		apiserver.Metrics = &models.ComponentMetrics{}

		// 請求速率
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(rate(apiserver_request_total[5m]))",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.RequestRate = val
			}
		}

		// 錯誤率
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(rate(apiserver_request_total{code=~\"5..\"}[5m])) / sum(rate(apiserver_request_total[5m])) * 100",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.ErrorRate = val
			}
		}

		// 延遲
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "histogram_quantile(0.99, sum(rate(apiserver_request_duration_seconds_bucket[5m])) by (le)) * 1000",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.Latency = val
			}
		}
	}

	// Etcd 指標
	if etcd, ok := componentsMap["etcd"]; ok {
		etcd.Metrics = &models.ComponentMetrics{}

		// Leader 狀態
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "max(etcd_server_has_leader)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.LeaderStatus = val == 1
			}
		}

		// 資料庫大小
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(etcd_mvcc_db_total_size_in_bytes)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.DBSize = val
			}
		}

		// 成員數量
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "count(etcd_server_has_leader)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.MemberCount = int(val)
			}
		}
	}

	// Scheduler 指標
	if scheduler, ok := componentsMap["kube-scheduler"]; ok {
		scheduler.Metrics = &models.ComponentMetrics{}

		// 佇列長度
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

// min 返回兩個整數中的較小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
