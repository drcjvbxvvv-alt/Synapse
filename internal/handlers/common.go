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

// parseFloatQuery 從 query string 解析浮點數，失敗時返回預設值
func parseFloatQuery(c *gin.Context, key string, def float64) float64 {
	if v, err := strconv.ParseFloat(c.Query(key), 64); err == nil {
		return v
	}
	return def
}
