package handlers

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

// wsBufferSize WebSocket 讀寫緩衝區大小（位元組）
const wsBufferSize = 1024

// ScaleRequest 擴縮容請求
type ScaleRequest struct {
	Replicas int32 `json:"replicas" binding:"required,min=0"`
}

// YAMLApplyRequest YAML應用請求
type YAMLApplyRequest struct {
	YAML   string `json:"yaml" binding:"required"`
	DryRun bool   `json:"dryRun"`
}

// parseClusterID 解析叢集ID字串為uint
func parseClusterID(clusterIDStr string) (uint, error) {
	id, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("無效的叢集ID: %s", clusterIDStr)
	}
	return uint(id), nil
}

// parseIntQuery 從 query string 解析整數，失敗時返回預設值
func parseIntQuery(c *gin.Context, key string, def int) int {
	if v, err := strconv.Atoi(c.Query(key)); err == nil && v > 0 {
		return v
	}
	return def
}

// parsePageSize 解析並強制限制 pageSize 在 [1, maxPageSize]，防止超大請求 DoS
const maxPageSize = 200

func parsePageSize(c *gin.Context, def int) int {
	v := parseIntQuery(c, "pageSize", def)
	if v > maxPageSize {
		v = maxPageSize
	}
	return v
}

// parsePage 解析 page（>= 1）
func parsePage(c *gin.Context) int {
	v := parseIntQuery(c, "page", 1)
	if v < 1 {
		return 1
	}
	return v
}

// warnLargeDataset 當 informer 快取資源總數超過 500 時，回傳 X-Large-Dataset: true header，
// 提示前端顯示「建議縮小命名空間範圍」提示。
const largeDatasetThreshold = 500

func warnLargeDataset(c *gin.Context, total int) {
	if total > largeDatasetThreshold {
		c.Header("X-Large-Dataset", "true")
	}
}

// parseFloatQuery 從 query string 解析浮點數，失敗時返回預設值
func parseFloatQuery(c *gin.Context, key string, def float64) float64 {
	if v, err := strconv.ParseFloat(c.Query(key), 64); err == nil {
		return v
	}
	return def
}
