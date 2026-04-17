package services

import (
	"context"
	"fmt"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CordonNode 封鎖節點（標記為不可排程）
func (c *K8sClient) CordonNode(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取節點
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("獲取節點失敗: %v", err)
	}

	// 檢查節點是否已經被封鎖
	if node.Spec.Unschedulable {
		return nil // 節點已經被封鎖，無需操作
	}

	// 標記節點為不可排程
	node.Spec.Unschedulable = true

	// 更新節點
	_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("封鎖節點失敗: %v", err)
	}

	return nil
}

// GetNodeMetrics 獲取節點資源使用情況
func (c *K8sClient) GetNodeMetrics(nodeName string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 獲取節點資訊
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取節點資訊失敗: %v", err)
	}

	// 獲取節點上的所有Pod
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("獲取節點Pod列表失敗: %v", err)
	}

	// 計算節點資源容量
	cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
	memoryCapacity := node.Status.Capacity.Memory().Value()
	allocatableCPU := node.Status.Allocatable.Cpu().MilliValue()
	allocatableMemory := node.Status.Allocatable.Memory().Value()

	// 計算Pod請求的資源總量
	var requestedCPU, requestedMemory int64
	var runningPodCount int

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPodCount++

			// 累加Pod中所有容器請求的資源
			for _, container := range pod.Spec.Containers {
				if container.Resources.Requests != nil {
					if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
						requestedCPU += cpu.MilliValue()
					}
					if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
						requestedMemory += memory.Value()
					}
				}
			}
		}
	}

	// 計算資源使用率
	cpuUsagePercent := 0.0
	memoryUsagePercent := 0.0

	if allocatableCPU > 0 {
		cpuUsagePercent = math.Min(100, float64(requestedCPU)/float64(allocatableCPU)*100)
	}

	if allocatableMemory > 0 {
		memoryUsagePercent = math.Min(100, float64(requestedMemory)/float64(allocatableMemory)*100)
	}

	// 如果無法獲取請求資源資訊，使用Pod數量估算
	if requestedCPU == 0 || requestedMemory == 0 {
		if runningPodCount > 0 {
			// 根據執行中的Pod數量估算使用率
			cpuUsagePercent = math.Min(95, float64(runningPodCount)*8)    // 假設每個Pod平均使用8%的CPU
			memoryUsagePercent = math.Min(90, float64(runningPodCount)*6) // 假設每個Pod平均使用6%的記憶體
		}
	}

	return map[string]interface{}{
		"cpuUsage":    cpuUsagePercent,
		"memoryUsage": memoryUsagePercent,
		"podCount":    runningPodCount,
		"resources": map[string]interface{}{
			"cpu": map[string]interface{}{
				"capacity":    cpuCapacity,
				"allocatable": allocatableCPU,
				"requested":   requestedCPU,
			},
			"memory": map[string]interface{}{
				"capacity":    memoryCapacity,
				"allocatable": allocatableMemory,
				"requested":   requestedMemory,
			},
		},
	}, nil
}

// GetAllNodesMetrics 獲取所有節點的資源使用情況
func (c *K8sClient) GetAllNodesMetrics() (map[string]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 獲取所有節點
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取節點列表失敗: %v", err)
	}

	// 獲取所有Pod
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取Pod列表失敗: %v", err)
	}

	// 按節點分組Pod
	nodePodsMap := make(map[string][]corev1.Pod)
	for _, pod := range pods.Items {
		if pod.Spec.NodeName != "" {
			nodePodsMap[pod.Spec.NodeName] = append(nodePodsMap[pod.Spec.NodeName], pod)
		}
	}

	// 計算每個節點的資源使用情況
	result := make(map[string]map[string]interface{})
	for _, node := range nodes.Items {
		nodePods := nodePodsMap[node.Name]

		// 計算節點資源容量
		cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
		memoryCapacity := node.Status.Capacity.Memory().Value()
		allocatableCPU := node.Status.Allocatable.Cpu().MilliValue()
		allocatableMemory := node.Status.Allocatable.Memory().Value()

		// 計算Pod請求的資源總量
		var requestedCPU, requestedMemory int64
		var runningPodCount int

		for _, pod := range nodePods {
			if pod.Status.Phase == corev1.PodRunning {
				runningPodCount++

				// 累加Pod中所有容器請求的資源
				for _, container := range pod.Spec.Containers {
					if container.Resources.Requests != nil {
						if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
							requestedCPU += cpu.MilliValue()
						}
						if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
							requestedMemory += memory.Value()
						}
					}
				}
			}
		}

		// 計算資源使用率
		cpuUsagePercent := 0.0
		memoryUsagePercent := 0.0

		if allocatableCPU > 0 {
			cpuUsagePercent = math.Min(100, float64(requestedCPU)/float64(allocatableCPU)*100)
		}

		if allocatableMemory > 0 {
			memoryUsagePercent = math.Min(100, float64(requestedMemory)/float64(allocatableMemory)*100)
		}

		// 如果無法獲取請求資源資訊，使用Pod數量估算
		if requestedCPU == 0 || requestedMemory == 0 {
			if runningPodCount > 0 {
				// 根據執行中的Pod數量估算使用率
				cpuUsagePercent = math.Min(95, float64(runningPodCount)*8)    // 假設每個Pod平均使用8%的CPU
				memoryUsagePercent = math.Min(90, float64(runningPodCount)*6) // 假設每個Pod平均使用6%的記憶體
			}
		}

		result[node.Name] = map[string]interface{}{
			"cpuUsage":    cpuUsagePercent,
			"memoryUsage": memoryUsagePercent,
			"podCount":    runningPodCount,
			"resources": map[string]interface{}{
				"cpu": map[string]interface{}{
					"capacity":    cpuCapacity,
					"allocatable": allocatableCPU,
					"requested":   requestedCPU,
				},
				"memory": map[string]interface{}{
					"capacity":    memoryCapacity,
					"allocatable": allocatableMemory,
					"requested":   requestedMemory,
				},
			},
		}
	}

	return result, nil
}

// UncordonNode 解封節點（標記為可排程）
func (c *K8sClient) UncordonNode(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取節點
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("獲取節點失敗: %v", err)
	}

	// 檢查節點是否已經可排程
	if !node.Spec.Unschedulable {
		return nil // 節點已經可排程，無需操作
	}

	// 標記節點為可排程
	node.Spec.Unschedulable = false

	// 更新節點
	_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("解封節點失敗: %v", err)
	}

	return nil
}

// DrainNode 驅逐節點上的Pod
func (c *K8sClient) DrainNode(nodeName string, options map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // 驅逐操作可能需要更長時間
	defer cancel()

	// 獲取節點
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("獲取節點失敗: %v", err)
	}

	// 1. 首先封鎖節點，防止新的Pod排程到該節點
	if !node.Spec.Unschedulable {
		node.Spec.Unschedulable = true
		_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("封鎖節點失敗: %v", err)
		}
	}

	// 解析選項
	ignoreDaemonSets := true
	if val, ok := options["ignoreDaemonSets"]; ok {
		ignoreDaemonSets = val.(bool)
	}

	deleteLocalData := false
	if val, ok := options["deleteLocalData"]; ok {
		deleteLocalData = val.(bool)
	}

	force := false
	if val, ok := options["force"]; ok {
		force = val.(bool)
	}

	gracePeriodSeconds := int64(30)
	if val, ok := options["gracePeriodSeconds"]; ok {
		if intVal, ok := val.(float64); ok {
			gracePeriodSeconds = int64(intVal)
		}
	}

	// 2. 獲取節點上的所有Pod
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return fmt.Errorf("獲取節點上的Pod失敗: %v", err)
	}

	// 3. 驅逐Pod
	for _, pod := range pods.Items {
		// 如果忽略DaemonSet，檢查Pod是否由DaemonSet控制
		if ignoreDaemonSets {
			isDaemonSet := false
			for _, owner := range pod.OwnerReferences {
				if owner.Kind == "DaemonSet" {
					isDaemonSet = true
					break
				}
			}
			if isDaemonSet {
				continue // 跳過DaemonSet管理的Pod
			}
		}

		// 檢查Pod是否使用emptyDir卷
		if !deleteLocalData {
			hasEmptyDir := false
			for _, volume := range pod.Spec.Volumes {
				if volume.EmptyDir != nil {
					hasEmptyDir = true
					break
				}
			}
			if hasEmptyDir && !force {
				return fmt.Errorf("pod %s/%s 使用emptyDir卷，需要設定deleteLocalData=true或force=true", pod.Namespace, pod.Name)
			}
		}

		// 刪除Pod
		deleteOptions := metav1.DeleteOptions{}
		if gracePeriodSeconds >= 0 {
			deleteOptions.GracePeriodSeconds = &gracePeriodSeconds
		}

		err = c.clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, deleteOptions)
		if err != nil {
			if !force {
				return fmt.Errorf("驅逐Pod %s/%s 失敗: %v", pod.Namespace, pod.Name, err)
			}
			// 如果設定了force，則忽略錯誤繼續執行
		}
	}

	return nil
}
