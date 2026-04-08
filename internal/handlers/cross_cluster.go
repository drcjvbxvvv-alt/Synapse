package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gorm.io/gorm"
)

// CrossClusterHandler 跨叢集統一工作負載檢視
type CrossClusterHandler struct {
	db            *gorm.DB
	clusterSvc    *services.ClusterService
	permissionSvc *services.PermissionService
	k8sMgr        *k8s.ClusterInformerManager
}

func NewCrossClusterHandler(db *gorm.DB, clusterSvc *services.ClusterService, permissionSvc *services.PermissionService, k8sMgr *k8s.ClusterInformerManager) *CrossClusterHandler {
	return &CrossClusterHandler{db: db, clusterSvc: clusterSvc, permissionSvc: permissionSvc, k8sMgr: k8sMgr}
}

// WorkloadSummary 跨叢集工作負載摘要
type WorkloadSummary struct {
	ClusterID   uint              `json:"clusterId"`
	ClusterName string            `json:"clusterName"`
	Namespace   string            `json:"namespace"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Replicas    int32             `json:"replicas"`
	Ready       int32             `json:"ready"`
	Images      []string          `json:"images"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"createdAt"`
	Status      string            `json:"status"` // healthy / degraded
}

func containerImages(containers []corev1.Container) []string {
	imgs := make([]string, 0, len(containers))
	for _, c := range containers {
		imgs = append(imgs, c.Image)
	}
	return imgs
}

func (h *CrossClusterHandler) getAccessibleClusters(userID uint) ([]*models.Cluster, error) {
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
		return nil, err
	}
	return clusters, nil
}

// ListCrossClusterWorkloads 跨叢集列出工作負載
// GET /api/v1/workloads?kind=Deployment&name=api&namespace=default&cluster=1
func (h *CrossClusterHandler) ListCrossClusterWorkloads(c *gin.Context) {
	userID := c.GetUint("user_id")
	filterKind := c.Query("kind")
	filterName := c.Query("name")
	filterNS := c.Query("namespace")
	filterCluster := c.Query("cluster")

	clusters, err := h.getAccessibleClusters(userID)
	if err != nil {
		response.InternalError(c, "取得叢集列表失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	var results []WorkloadSummary

	for _, cluster := range clusters {
		if filterCluster != "" {
			cid, _ := parseInt(filterCluster)
			if cluster.ID != uint(cid) {
				continue
			}
		}

		k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
		if err != nil {
			continue
		}
		cs := k8sClient.GetClientset()
		ns := filterNS

		if filterKind == "" || filterKind == "Deployment" {
			list, err := cs.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
			if err == nil {
				for _, d := range list.Items {
					if filterName != "" && !strings.Contains(d.Name, filterName) {
						continue
					}
					status := "healthy"
					if d.Status.ReadyReplicas < d.Status.Replicas {
						status = "degraded"
					}
					results = append(results, WorkloadSummary{
						ClusterID: cluster.ID, ClusterName: cluster.Name,
						Namespace: d.Namespace, Kind: "Deployment", Name: d.Name,
						Replicas: d.Status.Replicas, Ready: d.Status.ReadyReplicas,
						Images: containerImages(d.Spec.Template.Spec.Containers),
						Labels: d.Labels, CreatedAt: d.CreationTimestamp.Time, Status: status,
					})
				}
			}
		}

		if filterKind == "" || filterKind == "StatefulSet" {
			list, err := cs.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
			if err == nil {
				for _, ss := range list.Items {
					if filterName != "" && !strings.Contains(ss.Name, filterName) {
						continue
					}
					status := "healthy"
					if ss.Status.ReadyReplicas < ss.Status.Replicas {
						status = "degraded"
					}
					results = append(results, WorkloadSummary{
						ClusterID: cluster.ID, ClusterName: cluster.Name,
						Namespace: ss.Namespace, Kind: "StatefulSet", Name: ss.Name,
						Replicas: ss.Status.Replicas, Ready: ss.Status.ReadyReplicas,
						Images: containerImages(ss.Spec.Template.Spec.Containers),
						Labels: ss.Labels, CreatedAt: ss.CreationTimestamp.Time, Status: status,
					})
				}
			}
		}

		if filterKind == "" || filterKind == "DaemonSet" {
			list, err := cs.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
			if err == nil {
				for _, ds := range list.Items {
					if filterName != "" && !strings.Contains(ds.Name, filterName) {
						continue
					}
					status := "healthy"
					if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
						status = "degraded"
					}
					results = append(results, WorkloadSummary{
						ClusterID: cluster.ID, ClusterName: cluster.Name,
						Namespace: ds.Namespace, Kind: "DaemonSet", Name: ds.Name,
						Replicas: ds.Status.DesiredNumberScheduled, Ready: ds.Status.NumberReady,
						Images: containerImages(ds.Spec.Template.Spec.Containers),
						Labels: ds.Labels, CreatedAt: ds.CreationTimestamp.Time, Status: status,
					})
				}
			}
		}
	}

	response.OK(c, gin.H{"items": results, "total": len(results)})
}

// GetCrossClusterStats 跨叢集工作負載統計
// GET /api/v1/workloads/stats
func (h *CrossClusterHandler) GetCrossClusterStats(c *gin.Context) {
	userID := c.GetUint("user_id")
	clusters, err := h.getAccessibleClusters(userID)
	if err != nil {
		response.InternalError(c, "取得叢集列表失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	type ClusterStats struct {
		ClusterID    uint   `json:"clusterId"`
		ClusterName  string `json:"clusterName"`
		Deployments  int    `json:"deployments"`
		StatefulSets int    `json:"statefulSets"`
		DaemonSets   int    `json:"daemonSets"`
		Degraded     int    `json:"degraded"`
	}

	stats := make([]ClusterStats, 0, len(clusters))
	for _, cluster := range clusters {
		k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
		if err != nil {
			continue
		}
		cs := k8sClient.GetClientset()
		st := ClusterStats{ClusterID: cluster.ID, ClusterName: cluster.Name}

		if list, err := cs.AppsV1().Deployments("").List(ctx, metav1.ListOptions{}); err == nil {
			st.Deployments = len(list.Items)
			for _, d := range list.Items {
				if d.Status.ReadyReplicas < d.Status.Replicas {
					st.Degraded++
				}
			}
		}
		if list, err := cs.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{}); err == nil {
			st.StatefulSets = len(list.Items)
			for _, ss := range list.Items {
				if ss.Status.ReadyReplicas < ss.Status.Replicas {
					st.Degraded++
				}
			}
		}
		if list, err := cs.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{}); err == nil {
			st.DaemonSets = len(list.Items)
			for _, ds := range list.Items {
				if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
					st.Degraded++
				}
			}
		}
		stats = append(stats, st)
	}
	response.OK(c, gin.H{"clusters": stats})
}
