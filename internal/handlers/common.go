package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
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

// translateBindingError 將 Go validator 錯誤轉換為使用者友善的訊息。
// fieldNames 將 struct 欄位名映射為顯示名稱。
func translateBindingError(err error, fieldNames map[string]string) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return "請求參數無效"
	}

	var msgs []string
	for _, fe := range ve {
		field := fe.Field()
		if name, ok := fieldNames[field]; ok {
			field = name
		}
		switch fe.Tag() {
		case "required":
			msgs = append(msgs, field+" 為必填欄位")
		case "max":
			msgs = append(msgs, field+" 超過最大長度 "+fe.Param())
		case "oneof":
			msgs = append(msgs, field+" 必須為 "+strings.ReplaceAll(fe.Param(), " ", " / "))
		case "url":
			msgs = append(msgs, field+" 格式不正確")
		default:
			msgs = append(msgs, field+" 驗證失敗")
		}
	}
	return strings.Join(msgs, "；")
}

// parseFloatQuery 從 query string 解析浮點數，失敗時返回預設值
func parseFloatQuery(c *gin.Context, key string, def float64) float64 {
	if v, err := strconv.ParseFloat(c.Query(key), 64); err == nil {
		return v
	}
	return def
}
