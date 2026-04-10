package handlers

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// SearchHandler 搜尋處理器
type SearchHandler struct {
	cfg           *config.Config
	k8sMgr        *k8s.ClusterInformerManager
	clusterSvc    *services.ClusterService
	permissionSvc *services.PermissionService
}

// NewSearchHandler 建立搜尋處理器
func NewSearchHandler(cfg *config.Config, k8sMgr *k8s.ClusterInformerManager, clusterSvc *services.ClusterService, permSvc *services.PermissionService) *SearchHandler {
	return &SearchHandler{
		cfg:           cfg,
		k8sMgr:        k8sMgr,
		clusterSvc:    clusterSvc,
		permissionSvc: permSvc,
	}
}

// SearchResult 搜尋結果結構
type SearchResult struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitempty"`
	ClusterID   string `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	IP          string `json:"ip,omitempty"`
	Kind        string `json:"kind,omitempty"`
}

// GlobalSearch 全域性搜尋
func (h *SearchHandler) GlobalSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		response.BadRequest(c, "搜尋關鍵詞不能為空")
		return
	}

	logger.Info("全域性搜尋: %s", query)

	// 獲取使用者可訪問的叢集
	clusters, err := h.getAccessibleClusters(c.Request.Context(), c.GetUint("user_id"))
	if err != nil {
		logger.Error("獲取叢集列表失敗", "error", err)
		response.InternalError(c, "獲取叢集列表失敗")
		return
	}

	var (
		results []SearchResult
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	queryLower := strings.ToLower(query)

	// 搜尋叢集本身（快速，無需並行）
	for _, cluster := range clusters {
		if strings.Contains(strings.ToLower(cluster.Name), queryLower) {
			clusterIDStr := strconv.FormatUint(uint64(cluster.ID), 10)
			results = append(results, SearchResult{
				Type:        "cluster",
				ID:          clusterIDStr,
				Name:        cluster.Name,
				ClusterID:   clusterIDStr,
				ClusterName: cluster.Name,
				Status:      cluster.Status,
				Description: cluster.APIServer,
			})
		}
	}

	// 搜尋節點、Pod 和工作負載：每個叢集並行執行
	for _, cluster := range clusters {
		wg.Add(1)
		go func(cl *models.Cluster) {
			defer wg.Done()
			clusterResults := h.searchClusterResources(cl, query, queryLower)
			mu.Lock()
			results = append(results, clusterResults...)
			mu.Unlock()
		}(cluster)
	}
	wg.Wait()

	// 計算統計資訊
	stats := struct {
		Cluster  int `json:"cluster"`
		Node     int `json:"node"`
		Pod      int `json:"pod"`
		Workload int `json:"workload"`
	}{}

	for _, result := range results {
		switch result.Type {
		case "cluster":
			stats.Cluster++
		case "node":
			stats.Node++
		case "pod":
			stats.Pod++
		case "workload":
			stats.Workload++
		}
	}

	response.OK(c, gin.H{
		"results": results,
		"total":   len(results),
		"stats":   stats,
	})
}

// getAccessibleClusters 獲取使用者可訪問的叢集列表
func (h *SearchHandler) getAccessibleClusters(ctx context.Context, userID uint) ([]*models.Cluster, error) {
	clusterIDs, isAll, err := h.permissionSvc.GetUserAccessibleClusterIDs(userID)
	if err != nil {
		return nil, err
	}
	if isAll {
		return h.clusterSvc.GetAllClusters(ctx)
	}
	if len(clusterIDs) == 0 {
		return []*models.Cluster{}, nil
	}
	return h.clusterSvc.GetClustersByIDs(ctx, clusterIDs)
}

// getNodeStatus 獲取節點狀態
func (h *SearchHandler) getNodeStatus(node interface{}) string {
	// 這裡需要根據實際的節點結構來獲取狀態
	// 由於我們使用的是 interface{}，這裡簡化處理
	return "Ready"
}

// getNodeIP 獲取節點的主要IP地址
func (h *SearchHandler) getNodeIP(node interface{}) string {
	// 這裡需要根據實際的節點結構來獲取IP
	// 由於我們使用的是 interface{}，這裡簡化處理
	return ""
}
