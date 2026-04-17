package services

import (
	"context"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"

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
