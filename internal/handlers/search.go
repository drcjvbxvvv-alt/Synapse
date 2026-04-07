package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// SearchHandler 搜尋處理器
type SearchHandler struct {
	db            *gorm.DB
	cfg           *config.Config
	k8sMgr        *k8s.ClusterInformerManager
	clusterSvc    *services.ClusterService
	permissionSvc *services.PermissionService
}

// NewSearchHandler 建立搜尋處理器
func NewSearchHandler(db *gorm.DB, cfg *config.Config, k8sMgr *k8s.ClusterInformerManager, clusterSvc *services.ClusterService, permSvc *services.PermissionService) *SearchHandler {
	return &SearchHandler{
		db:            db,
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
	clusters, err := h.getAccessibleClusters(c.GetUint("user_id"))
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

// QuickSearch 快速搜尋（用於頂部搜尋欄）
func (h *SearchHandler) QuickSearch(c *gin.Context) {
	query := c.Query("q")
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}

	if query == "" {
		response.OK(c, gin.H{
			"results": []SearchResult{},
		})
		return
	}

	logger.Info("快速搜尋: %s", query)

	// 獲取使用者可訪問的叢集
	clusters, err := h.getAccessibleClusters(c.GetUint("user_id"))
	if err != nil {
		logger.Error("獲取叢集列表失敗", "error", err)
		response.InternalError(c, "獲取叢集列表失敗")
		return
	}

	// 按資源型別分組儲存結果，確保每種型別都能被搜尋到
	typeResults := map[string][]SearchResult{
		"cluster":  {},
		"node":     {},
		"pod":      {},
		"workload": {},
	}

	// 每種資源型別都有獨立的 limit 限制
	typeLimit := limit

	// 遍歷所有叢集進行搜尋
	for _, cluster := range clusters {
		// 確保叢集的 informer 已初始化
		_, err := h.k8sMgr.EnsureForCluster(cluster)
		if err != nil {
			logger.Error("初始化叢集 informer 失敗", "cluster", cluster.Name, "error", err)
			continue
		}

		clusterIDStr := strconv.FormatUint(uint64(cluster.ID), 10)

		// 搜尋叢集本身
		if len(typeResults["cluster"]) < typeLimit && strings.Contains(strings.ToLower(cluster.Name), strings.ToLower(query)) {
			typeResults["cluster"] = append(typeResults["cluster"], SearchResult{
				Type:        "cluster",
				ID:          clusterIDStr,
				Name:        cluster.Name,
				ClusterID:   clusterIDStr,
				ClusterName: cluster.Name,
				Status:      cluster.Status,
				Description: cluster.APIServer,
			})
		}

		// 搜尋節點
		if len(typeResults["node"]) < typeLimit {
			nodeLister := h.k8sMgr.NodesLister(cluster.ID)
			if nodeLister != nil {
				nodes, err := nodeLister.List(labels.Everything())
				if err == nil {
					for _, node := range nodes {
						if len(typeResults["node"]) >= typeLimit {
							break
						}
						if strings.Contains(strings.ToLower(node.Name), strings.ToLower(query)) {
							typeResults["node"] = append(typeResults["node"], SearchResult{
								Type:        "node",
								ID:          node.Name,
								Name:        node.Name,
								ClusterID:   clusterIDStr,
								ClusterName: cluster.Name,
								Status:      h.getNodeStatus(node),
								Description: node.Spec.PodCIDR,
								IP:          h.getNodeIP(node),
							})
						}
					}
				}
			}
		}

		// 搜尋Pod
		if len(typeResults["pod"]) < typeLimit {
			podLister := h.k8sMgr.PodsLister(cluster.ID)
			if podLister != nil {
				pods, err := podLister.List(labels.Everything())
				if err == nil {
					for _, pod := range pods {
						if len(typeResults["pod"]) >= typeLimit {
							break
						}
						if strings.Contains(strings.ToLower(pod.Name), strings.ToLower(query)) {
							typeResults["pod"] = append(typeResults["pod"], SearchResult{
								Type:        "pod",
								ID:          pod.Name,
								Name:        pod.Name,
								Namespace:   pod.Namespace,
								ClusterID:   clusterIDStr,
								ClusterName: cluster.Name,
								Status:      string(pod.Status.Phase),
								Description: pod.Spec.NodeName,
								IP:          pod.Status.PodIP,
							})
						}
					}
				}
			}
		}

		// 搜尋工作負載
		if len(typeResults["workload"]) < typeLimit {
			// Deployment
			deploymentLister := h.k8sMgr.DeploymentsLister(cluster.ID)
			if deploymentLister != nil {
				deployments, err := deploymentLister.List(labels.Everything())
				if err == nil {
					for _, deployment := range deployments {
						if len(typeResults["workload"]) >= typeLimit {
							break
						}
						if strings.Contains(strings.ToLower(deployment.Name), strings.ToLower(query)) {
							replicas := "1"
							if deployment.Spec.Replicas != nil {
								replicas = strconv.FormatInt(int64(*deployment.Spec.Replicas), 10)
							}
							typeResults["workload"] = append(typeResults["workload"], SearchResult{
								Type:        "workload",
								ID:          deployment.Name,
								Name:        deployment.Name,
								Namespace:   deployment.Namespace,
								ClusterID:   clusterIDStr,
								ClusterName: cluster.Name,
								Status:      "Deployment",
								Kind:        "Deployment",
								Description: replicas,
							})
						}
					}
				}
			}

			// StatefulSet
			if len(typeResults["workload"]) < typeLimit {
				statefulSetLister := h.k8sMgr.StatefulSetsLister(cluster.ID)
				if statefulSetLister != nil {
					statefulSets, err := statefulSetLister.List(labels.Everything())
					if err == nil {
						for _, statefulSet := range statefulSets {
							if len(typeResults["workload"]) >= typeLimit {
								break
							}
							if strings.Contains(strings.ToLower(statefulSet.Name), strings.ToLower(query)) {
								replicas := "1"
								if statefulSet.Spec.Replicas != nil {
									replicas = strconv.FormatInt(int64(*statefulSet.Spec.Replicas), 10)
								}
								typeResults["workload"] = append(typeResults["workload"], SearchResult{
									Type:        "workload",
									ID:          statefulSet.Name,
									Name:        statefulSet.Name,
									Namespace:   statefulSet.Namespace,
									ClusterID:   clusterIDStr,
									ClusterName: cluster.Name,
									Status:      "StatefulSet",
									Kind:        "StatefulSet",
									Description: replicas,
								})
							}
						}
					}
				}
			}

			// DaemonSet
			if len(typeResults["workload"]) < typeLimit {
				daemonSetLister := h.k8sMgr.DaemonSetsLister(cluster.ID)
				if daemonSetLister != nil {
					daemonSets, err := daemonSetLister.List(labels.Everything())
					if err == nil {
						for _, daemonSet := range daemonSets {
							if len(typeResults["workload"]) >= typeLimit {
								break
							}
							if strings.Contains(strings.ToLower(daemonSet.Name), strings.ToLower(query)) {
								typeResults["workload"] = append(typeResults["workload"], SearchResult{
									Type:        "workload",
									ID:          daemonSet.Name,
									Name:        daemonSet.Name,
									Namespace:   daemonSet.Namespace,
									ClusterID:   clusterIDStr,
									ClusterName: cluster.Name,
									Status:      "DaemonSet",
									Kind:        "DaemonSet",
									Description: "DaemonSet",
								})
							}
						}
					}
				}
			}

			// Rollout
			if len(typeResults["workload"]) < typeLimit {
				rolloutLister := h.k8sMgr.RolloutsLister(cluster.ID)
				if rolloutLister != nil {
					rollouts, err := rolloutLister.List(labels.Everything())
					if err == nil {
						for _, rollout := range rollouts {
							if len(typeResults["workload"]) >= typeLimit {
								break
							}
							if strings.Contains(strings.ToLower(rollout.Name), strings.ToLower(query)) {
								typeResults["workload"] = append(typeResults["workload"], SearchResult{
									Type:        "workload",
									ID:          rollout.Name,
									Name:        rollout.Name,
									Namespace:   rollout.Namespace,
									ClusterID:   clusterIDStr,
									ClusterName: cluster.Name,
									Status:      "Rollout",
									Kind:        "Rollout",
									Description: strconv.FormatInt(int64(rollout.Status.AvailableReplicas), 10),
								})
							}
						}
					}
				}
			}
		}
	}

	// 合併所有型別的結果
	var results []SearchResult
	for _, typeResult := range typeResults {
		results = append(results, typeResult...)
	}

	response.OK(c, gin.H{
		"results": results,
	})
}

// searchClusterResources 在單一叢集中搜尋節點、Pod 與工作負載（供並行呼叫）
func (h *SearchHandler) searchClusterResources(cluster *models.Cluster, query, queryLower string) []SearchResult {
	if _, err := h.k8sMgr.EnsureForCluster(cluster); err != nil {
		logger.Error("初始化叢集 informer 失敗", "cluster", cluster.Name, "error", err)
		return nil
	}

	clusterIDStr := strconv.FormatUint(uint64(cluster.ID), 10)
	var results []SearchResult
	sel := labels.Everything()

	// 節點
	if nodeLister := h.k8sMgr.NodesLister(cluster.ID); nodeLister != nil {
		if nodes, err := nodeLister.List(sel); err == nil {
			for _, node := range nodes {
				if strings.Contains(strings.ToLower(node.Name), queryLower) {
					results = append(results, SearchResult{
						Type: "node", ID: node.Name, Name: node.Name,
						ClusterID: clusterIDStr, ClusterName: cluster.Name,
						Status: h.getNodeStatus(node), Description: node.Spec.PodCIDR, IP: h.getNodeIP(node),
					})
					continue
				}
				for _, addr := range node.Status.Addresses {
					if strings.Contains(addr.Address, query) {
						results = append(results, SearchResult{
							Type: "node", ID: node.Name, Name: node.Name,
							ClusterID: clusterIDStr, ClusterName: cluster.Name,
							Status: h.getNodeStatus(node), Description: node.Spec.PodCIDR, IP: addr.Address,
						})
						break
					}
				}
			}
		}
	}

	// Pod
	if podLister := h.k8sMgr.PodsLister(cluster.ID); podLister != nil {
		if pods, err := podLister.List(sel); err == nil {
			for _, pod := range pods {
				if strings.Contains(strings.ToLower(pod.Name), queryLower) || strings.Contains(pod.Status.PodIP, query) {
					results = append(results, SearchResult{
						Type: "pod", ID: pod.Name, Name: pod.Name, Namespace: pod.Namespace,
						ClusterID: clusterIDStr, ClusterName: cluster.Name,
						Status: string(pod.Status.Phase), Description: pod.Spec.NodeName, IP: pod.Status.PodIP,
					})
				}
			}
		}
	}

	// Deployment
	if lister := h.k8sMgr.DeploymentsLister(cluster.ID); lister != nil {
		if items, err := lister.List(sel); err == nil {
			for _, d := range items {
				if strings.Contains(strings.ToLower(d.Name), queryLower) {
					replicas := "1"
					if d.Spec.Replicas != nil {
						replicas = strconv.FormatInt(int64(*d.Spec.Replicas), 10)
					}
					results = append(results, SearchResult{
						Type: "workload", ID: d.Name, Name: d.Name, Namespace: d.Namespace,
						ClusterID: clusterIDStr, ClusterName: cluster.Name,
						Status: "Deployment", Kind: "Deployment", Description: replicas,
					})
				}
			}
		}
	}

	// Rollout
	if lister := h.k8sMgr.RolloutsLister(cluster.ID); lister != nil {
		if items, err := lister.List(sel); err == nil {
			for _, r := range items {
				if strings.Contains(strings.ToLower(r.Name), queryLower) {
					results = append(results, SearchResult{
						Type: "workload", ID: r.Name, Name: r.Name, Namespace: r.Namespace,
						ClusterID: clusterIDStr, ClusterName: cluster.Name,
						Status: "Rollout", Kind: "Rollout",
						Description: strconv.FormatInt(int64(r.Status.AvailableReplicas), 10),
					})
				}
			}
		}
	}

	// StatefulSet
	if lister := h.k8sMgr.StatefulSetsLister(cluster.ID); lister != nil {
		if items, err := lister.List(sel); err == nil {
			for _, s := range items {
				if strings.Contains(strings.ToLower(s.Name), queryLower) {
					replicas := "1"
					if s.Spec.Replicas != nil {
						replicas = strconv.FormatInt(int64(*s.Spec.Replicas), 10)
					}
					results = append(results, SearchResult{
						Type: "workload", ID: s.Name, Name: s.Name, Namespace: s.Namespace,
						ClusterID: clusterIDStr, ClusterName: cluster.Name,
						Status: "StatefulSet", Kind: "StatefulSet", Description: replicas,
					})
				}
			}
		}
	}

	// DaemonSet
	if lister := h.k8sMgr.DaemonSetsLister(cluster.ID); lister != nil {
		if items, err := lister.List(sel); err == nil {
			for _, d := range items {
				if strings.Contains(strings.ToLower(d.Name), queryLower) {
					results = append(results, SearchResult{
						Type: "workload", ID: d.Name, Name: d.Name, Namespace: d.Namespace,
						ClusterID: clusterIDStr, ClusterName: cluster.Name,
						Status: "DaemonSet", Kind: "DaemonSet", Description: "DaemonSet",
					})
				}
			}
		}
	}

	return results
}

// getAccessibleClusters 獲取使用者可訪問的叢集列表
func (h *SearchHandler) getAccessibleClusters(userID uint) ([]*models.Cluster, error) {
	clusterIDs, isAll, err := h.permissionSvc.GetUserAccessibleClusterIDs(userID)
	if err != nil {
		return nil, err
	}
	if isAll {
		return h.clusterSvc.GetAllClusters()
	}
	if len(clusterIDs) == 0 {
		return []*models.Cluster{}, nil
	}
	var clusters []*models.Cluster
	if err := h.db.Where("id IN ?", clusterIDs).Find(&clusters).Error; err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}
	return clusters, nil
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
