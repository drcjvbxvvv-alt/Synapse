package services

import (
	"context"
	"fmt"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetClusterMetrics 獲取叢集監控資料
func (c *K8sClient) GetClusterMetrics(timeRange string, step string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metrics := make(map[string]interface{})

	// 獲取節點資訊
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取節點資訊失敗: %v", err)
	}

	// 獲取Pod資訊
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取Pod資訊失敗: %v", err)
	}

	// 計算時間範圍
	endTime := time.Now()
	var startTime time.Time

	switch timeRange {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
	case "12h":
		startTime = endTime.Add(-12 * time.Hour)
	case "1d":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-1 * time.Hour)
	}

	// 從節點狀態和Pod分佈估算資源使用情況
	// 計算節點資源總量和已分配資源
	var totalCPUCapacity, allocatableCPU int64
	var totalMemoryCapacity, allocatableMemory int64

	for _, node := range nodes.Items {
		// 獲取節點總容量
		cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
		memoryCapacity := node.Status.Capacity.Memory().Value()

		totalCPUCapacity += cpuCapacity
		totalMemoryCapacity += memoryCapacity

		// 獲取節點可分配資源
		allocatableCPU += node.Status.Allocatable.Cpu().MilliValue()
		allocatableMemory += node.Status.Allocatable.Memory().Value()
	}

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

	// 如果無法獲取請求資源資訊，使用Pod數量和節點數量估算
	if requestedCPU == 0 || requestedMemory == 0 {
		readyNodeCount := 0
		for _, node := range nodes.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					readyNodeCount++
					break
				}
			}
		}

		if readyNodeCount > 0 {
			// 根據執行中的Pod數量和節點數量估算使用率
			podsPerNode := float64(runningPodCount) / float64(readyNodeCount)
			cpuUsagePercent = math.Min(95, podsPerNode*10)   // 假設每個Pod平均使用10%的CPU
			memoryUsagePercent = math.Min(90, podsPerNode*8) // 假設每個Pod平均使用8%的記憶體
		}
	}

	// 統計Pod狀態分佈
	podStatus := map[string]int{
		"Running":   0,
		"Pending":   0,
		"Succeeded": 0,
		"Failed":    0,
		"Unknown":   0,
	}

	for _, pod := range pods.Items {
		status := string(pod.Status.Phase)
		if count, exists := podStatus[status]; exists {
			podStatus[status] = count + 1
		} else {
			podStatus["Unknown"]++
		}
	}

	// 統計節點狀態
	nodeStatus := map[string]int{
		"Ready":    0,
		"NotReady": 0,
	}

	for _, node := range nodes.Items {
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					nodeStatus["Ready"]++
					isReady = true
				}
				break
			}
		}
		if !isReady {
			nodeStatus["NotReady"]++
		}
	}

	// 生成時間序列資料
	// 注意：這裡我們仍然使用模擬資料生成時間序列，因為獲取歷史資料需要Prometheus等監控系統
	// 在實際生產環境中，應該整合Prometheus API來獲取真實的歷史資料
	var timePoints []time.Time
	stepDuration, _ := time.ParseDuration(step)
	if stepDuration == 0 {
		stepDuration = time.Minute // 預設1分鐘
	}

	for t := startTime; t.Before(endTime); t = t.Add(stepDuration) {
		timePoints = append(timePoints, t)
	}
	timePoints = append(timePoints, endTime)

	// 生成CPU使用率資料，但使用當前真實的CPU使用率作為基準
	cpuData := make([]map[string]interface{}, 0, len(timePoints))
	for i, t := range timePoints {
		// 使用真實的當前值作為基準，歷史資料仍然模擬
		var value float64
		if i == len(timePoints)-1 {
			value = cpuUsagePercent
		} else {
			// 模擬歷史資料，但圍繞當前真實值波動
			variance := 20.0
			if cpuUsagePercent > 80 {
				variance = 10.0
			}
			value = math.Max(0, math.Min(100, cpuUsagePercent+(math.Sin(float64(t.Unix()%3600)/3600*2*math.Pi)-0.5)*variance))
		}

		cpuData = append(cpuData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     value,
		})
	}

	// 生成記憶體使用率資料，但使用當前真實的記憶體使用率作為基準
	memoryData := make([]map[string]interface{}, 0, len(timePoints))
	for i, t := range timePoints {
		// 使用真實的當前值作為基準，歷史資料仍然模擬
		var value float64
		if i == len(timePoints)-1 {
			value = memoryUsagePercent
		} else {
			// 模擬歷史資料，但圍繞當前真實值波動
			variance := 15.0
			if memoryUsagePercent > 80 {
				variance = 8.0
			}
			value = math.Max(0, math.Min(100, memoryUsagePercent+(math.Sin(float64(t.Unix()%7200)/7200*2*math.Pi)-0.5)*variance))
		}

		memoryData = append(memoryData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     value,
		})
	}

	// 網路和磁碟資料仍然使用模擬資料，因為這些需要特定的監控系統
	networkInData := make([]map[string]interface{}, 0, len(timePoints))
	networkOutData := make([]map[string]interface{}, 0, len(timePoints))
	for _, t := range timePoints {
		networkInData = append(networkInData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     30 + 20*math.Sin(float64(t.Unix()%5400)/5400*2*math.Pi),
		})
		networkOutData = append(networkOutData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     25 + 15*math.Sin(float64(t.Unix()%4800)/4800*2*math.Pi),
		})
	}

	diskData := make([]map[string]interface{}, 0, len(timePoints))
	for _, t := range timePoints {
		diskData = append(diskData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     40 + 5*math.Sin(float64(t.Unix()%10800)/10800*2*math.Pi),
		})
	}

	// 組裝返回資料
	metrics["cpu"] = map[string]interface{}{
		"current": cpuUsagePercent,
		"series":  cpuData,
	}

	metrics["memory"] = map[string]interface{}{
		"current": memoryUsagePercent,
		"series":  memoryData,
	}

	metrics["network"] = map[string]interface{}{
		"in": map[string]interface{}{
			"current": networkInData[len(networkInData)-1]["value"],
			"series":  networkInData,
		},
		"out": map[string]interface{}{
			"current": networkOutData[len(networkOutData)-1]["value"],
			"series":  networkOutData,
		},
	}

	metrics["disk"] = map[string]interface{}{
		"current": diskData[len(diskData)-1]["value"],
		"series":  diskData,
	}

	metrics["pods"] = podStatus
	metrics["nodes"] = nodeStatus

	// 新增時間範圍資訊
	metrics["timeRange"] = map[string]interface{}{
		"start": startTime.Unix(),
		"end":   endTime.Unix(),
		"step":  step,
	}

	return metrics, nil
}
