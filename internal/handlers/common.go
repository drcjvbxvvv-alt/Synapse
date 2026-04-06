package handlers

import (
	"fmt"
	"strconv"
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
