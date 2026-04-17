package services

import (
	"time"

	"github.com/shaia/Synapse/internal/models"
)

// ---- Phase 3：趨勢、預測、Right-sizing ----

// CapacityTrendPoint 月度容量佔用趨勢資料點（來自 cluster_occupancy_snapshots）
type CapacityTrendPoint struct {
	Month              string  `json:"month"`                  // "2026-01"
	CPUOccupancyPct    float64 `json:"cpu_occupancy_percent"`
	MemoryOccupancyPct float64 `json:"memory_occupancy_percent"`
	NodeCount          int     `json:"node_count"`
}

// ForecastResult 容量耗盡預測（線性迴歸）
type ForecastResult struct {
	BasedOnMonths    int     `json:"based_on_months"`
	CurrentCPUPct    float64 `json:"current_cpu_percent"`
	CurrentMemoryPct float64 `json:"current_memory_percent"`
	CPU80PctDate     *string `json:"cpu_80_percent_date"`    // null = 預測範圍內不到達
	CPU100PctDate    *string `json:"cpu_100_percent_date"`
	Memory80PctDate  *string `json:"memory_80_percent_date"`
	Memory100PctDate *string `json:"memory_100_percent_date"`
}

// RightSizingRecommendation 工作負載 Right-sizing 建議
type RightSizingRecommendation struct {
	CPUMillicores float64 `json:"cpu_recommended_millicores"`
	MemoryMiB     float64 `json:"memory_recommended_mib"`
	Confidence    string  `json:"confidence"` // "medium"（有 Prometheus 資料時）
}

// GetTrend 讀取 cluster_occupancy_snapshots，按月彙總佔用率趨勢
func (s *ResourceService) GetTrend(cluster *models.Cluster, months int) ([]CapacityTrendPoint, error) {
	if months <= 0 {
		months = 6
	}
	since := time.Now().AddDate(0, -months, 0)

	var snaps []models.ClusterOccupancySnapshot
	if err := s.db.Where("cluster_id = ? AND date >= ?", cluster.ID, since).
		Order("date ASC").Find(&snaps).Error; err != nil {
		return nil, err
	}

	type monthAgg struct {
		cpuPct    float64
		memPct    float64
		nodeCount int
		count     int
	}
	aggs := make(map[string]*monthAgg)
	order := make([]string, 0)

	for _, sn := range snaps {
		m := sn.Date.Format("2006-01")
		if _, ok := aggs[m]; !ok {
			aggs[m] = &monthAgg{}
			order = append(order, m)
		}
		a := aggs[m]
		a.count++
		a.nodeCount = sn.NodeCount
		if sn.AllocatableCPU > 0 {
			a.cpuPct += sn.RequestedCPU / sn.AllocatableCPU * 100
		}
		if sn.AllocatableMemory > 0 {
			a.memPct += sn.RequestedMemory / sn.AllocatableMemory * 100
		}
	}

	result := make([]CapacityTrendPoint, 0, len(order))
	for _, m := range order {
		a := aggs[m]
		if a.count == 0 {
			continue
		}
		result = append(result, CapacityTrendPoint{
			Month:              m,
			CPUOccupancyPct:    a.cpuPct / float64(a.count),
			MemoryOccupancyPct: a.memPct / float64(a.count),
			NodeCount:          a.nodeCount,
		})
	}
	return result, nil
}

// GetForecast 基於歷史快照做線性迴歸，預測容量耗盡日期
func (s *ResourceService) GetForecast(cluster *models.Cluster, days int) (*ForecastResult, error) {
	if days <= 0 {
		days = 180
	}
	trend, err := s.GetTrend(cluster, 6)
	if err != nil {
		return nil, err
	}
	n := len(trend)
	result := &ForecastResult{BasedOnMonths: n}
	if n == 0 {
		return result, nil
	}
	result.CurrentCPUPct = trend[n-1].CPUOccupancyPct
	result.CurrentMemoryPct = trend[n-1].MemoryOccupancyPct
	if n < 2 {
		return result, nil
	}

	cpuSlope, cpuIntercept := linReg(n, func(i int) float64 { return trend[i].CPUOccupancyPct })
	memSlope, memIntercept := linReg(n, func(i int) float64 { return trend[i].MemoryOccupancyPct })

	lastMonth, _ := time.Parse("2006-01", trend[n-1].Month)
	currentIdx := float64(n - 1)
	maxMonths := float64(days) / 30.0

	predictDate := func(slope, intercept, target float64) *string {
		if slope <= 0 {
			return nil
		}
		xTarget := (target - intercept) / slope
		monthsAhead := xTarget - currentIdx
		if monthsAhead < 0 || monthsAhead > maxMonths {
			return nil
		}
		t := lastMonth.AddDate(0, int(monthsAhead+0.5), 0)
		dateStr := t.Format("2006-01-02")
		return &dateStr
	}

	result.CPU80PctDate = predictDate(cpuSlope, cpuIntercept, 80)
	result.CPU100PctDate = predictDate(cpuSlope, cpuIntercept, 100)
	result.Memory80PctDate = predictDate(memSlope, memIntercept, 80)
	result.Memory100PctDate = predictDate(memSlope, memIntercept, 100)
	return result, nil
}

// linReg 對 n 個點做最小二乘線性迴歸，x = index（0..n-1）
func linReg(n int, getY func(int) float64) (slope, intercept float64) {
	fn := float64(n)
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		x, y := float64(i), getY(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := fn*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / fn
	}
	slope = (fn*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / fn
	return
}
