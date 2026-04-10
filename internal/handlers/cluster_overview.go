package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// GetClusterOverview 獲取叢集概覽資訊
func (h *ClusterHandler) GetClusterOverview(c *gin.Context) {
	clusterID := c.Param("clusterID")
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 優先使用 Informer 快取（方案C）
	// 確保本叢集的 informer 已初始化並啟動
	if _, err := h.k8sMgr.EnsureForCluster(cluster); err == nil {
		if snap, err := h.k8sMgr.GetOverviewSnapshot(c.Request.Context(), cluster.ID); err == nil {
			// 獲取容器子網IP資訊
			containerSubnetIPs, err := h.getContainerSubnetIPs(c.Request.Context(), cluster)
			if err != nil {
				logger.Error("獲取容器子網IP資訊失敗", "error", err)
				// 不返回錯誤，只是不顯示容器子網資訊
			} else {
				// 轉換型別
				snap.ContainerSubnetIPs = &k8s.ContainerSubnetIPs{
					TotalIPs:     containerSubnetIPs.TotalIPs,
					UsedIPs:      containerSubnetIPs.UsedIPs,
					AvailableIPs: containerSubnetIPs.AvailableIPs,
				}
			}

			response.OK(c, snap)
			return
		}
	}

	// 如果 informer 方式失敗，返回錯誤
	response.ServiceUnavailable(c, "叢集資訊獲取失敗")
}

// getContainerSubnetIPs 獲取容器子網IP資訊
func (h *ClusterHandler) getContainerSubnetIPs(ctx context.Context, cluster *models.Cluster) (*models.ContainerSubnetIPs, error) {
	// 獲取監控配置（使用注入的服務，不再從 h.db 重新構造）
	config, err := h.monitoringCfgSvc.GetMonitoringConfig(cluster.ID)
	if err != nil {
		return nil, fmt.Errorf("獲取監控配置失敗: %w", err)
	}

	// 如果監控功能被禁用，返回空資訊
	if config.Type == "disabled" {
		return nil, fmt.Errorf("監控功能已禁用")
	}

	// 查詢容器子網IP資訊（使用注入的 Prometheus 服務）
	return h.promService.QueryContainerSubnetIPs(ctx, config)
}

// GetClusterMetrics 獲取叢集監控資料
func (h *ClusterHandler) GetClusterMetrics(c *gin.Context) {
	id := c.Param("clusterID")
	logger.Info("獲取叢集監控資料: %s", id)

	// 獲取請求參數
	rangeParam := c.DefaultQuery("range", "1h")
	step := c.DefaultQuery("step", "1m")

	// 從資料庫獲取叢集
	clusterID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(clusterID))
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err)
		response.InternalError(c, "獲取叢集監控資料失敗: "+err.Error())
		return
	}

	// 獲取叢集監控資料
	metrics, err := k8sClient.GetClusterMetrics(rangeParam, step)
	if err != nil {
		logger.Error("獲取叢集監控資料失敗", "error", err)
		response.InternalError(c, "獲取叢集監控資料失敗: "+err.Error())
		return
	}

	response.OK(c, metrics)
}

// getClusterNodeInfo 獲取叢集節點資訊
func (h *ClusterHandler) getClusterNodeInfo(cluster *models.Cluster) (int, int) {
	// 使用 informer+lister 讀取節點並統計（不直連 API）
	if _, err := h.k8sMgr.EnsureAndWait(context.Background(), cluster, 5*time.Second); err != nil {
		logger.Error("informer 未就緒", "error", err)
		return 0, 0
	}
	nodes, err := h.k8sMgr.NodesLister(cluster.ID).List(labels.Everything())
	if err != nil {
		logger.Error("讀取節點快取失敗", "error", err)
		return 0, 0
	}
	nodeCount := len(nodes)
	readyNodes := 0
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				readyNodes++
				break
			}
		}
	}
	return nodeCount, readyNodes
}

// getClusterResourceUsage 獲取叢集 CPU 和記憶體使用率
func (h *ClusterHandler) getClusterResourceUsage(ctx context.Context, cluster *models.Cluster) (float64, float64) {
	if h.promService == nil || h.monitoringCfgSvc == nil {
		return 0, 0
	}

	// 獲取叢集的監控配置
	config, err := h.monitoringCfgSvc.GetMonitoringConfig(cluster.ID)
	if err != nil || config.Type == "disabled" {
		return 0, 0
	}

	// 設定時間範圍（最近 5 分鐘）
	now := time.Now().Unix()
	start := now - 300
	step := "1m"

	var cpuUsage, memoryUsage float64

	// 查詢 CPU 使用率
	cpuQuery := &models.MetricsQuery{
		Query: "(1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m]))) * 100",
		Start: start,
		End:   now,
		Step:  step,
	}
	if resp, err := h.promService.QueryPrometheus(ctx, config, cpuQuery); err == nil {
		if val := extractLatestValueFromResponse(resp); val >= 0 {
			cpuUsage = val
		}
	}

	// 查詢記憶體使用率
	memQuery := &models.MetricsQuery{
		Query: "(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100",
		Start: start,
		End:   now,
		Step:  step,
	}
	if resp, err := h.promService.QueryPrometheus(ctx, config, memQuery); err == nil {
		if val := extractLatestValueFromResponse(resp); val >= 0 {
			memoryUsage = val
		}
	}

	return cpuUsage, memoryUsage
}

// extractLatestValueFromResponse 從 Prometheus range query 響應中提取最新值
func extractLatestValueFromResponse(resp *models.MetricsResponse) float64 {
	if resp == nil || len(resp.Data.Result) == 0 {
		return -1
	}
	result := resp.Data.Result[0]
	// 優先從 Values (range query) 中獲取最後一個值
	if len(result.Values) > 0 {
		lastValue := result.Values[len(result.Values)-1]
		if len(lastValue) >= 2 {
			if strVal, ok := lastValue[1].(string); ok {
				var f float64
				if _, err := fmt.Sscanf(strVal, "%f", &f); err == nil {
					return f
				}
			}
		}
	}
	// 相容 instant query 的 Value 格式
	if len(result.Value) >= 2 {
		if val, ok := result.Value[1].(string); ok {
			var f float64
			if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
				return f
			}
		}
	}
	return -1
}
