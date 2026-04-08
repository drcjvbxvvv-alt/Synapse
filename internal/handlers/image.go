package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gorm.io/gorm"
)

// ImageHandler 映像索引處理器
type ImageHandler struct {
	db             *gorm.DB
	clusterService *services.ClusterService
	permissionSvc  *services.PermissionService
	k8sMgr         *k8s.ClusterInformerManager
}

func NewImageHandler(db *gorm.DB, clusterService *services.ClusterService, permissionSvc *services.PermissionService, k8sMgr *k8s.ClusterInformerManager) *ImageHandler {
	return &ImageHandler{db: db, clusterService: clusterService, permissionSvc: permissionSvc, k8sMgr: k8sMgr}
}

// parseImageParts 分離映像名稱與 tag
func parseImageParts(image string) (name, tag string) {
	// 處理 digest（sha256:...）
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		return image[:idx], image[idx+1:]
	}
	// 最後一個 : 之後是 tag（但需排除 registry:port 的情況）
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return image, "latest"
	}
	// 若 : 後麵包含 / 則屬於 registry:port，沒有 tag
	afterColon := image[lastColon+1:]
	if strings.Contains(afterColon, "/") {
		return image, "latest"
	}
	return image[:lastColon], afterColon
}

// SyncImages 掃描所有可存取叢集的工作負載映像並寫入索引
// POST /api/v1/images/sync
func (h *ImageHandler) SyncImages(c *gin.Context) {
	userID := c.GetUint("user_id")

	clusterIDs, isAll, err := h.permissionSvc.GetUserAccessibleClusterIDs(userID)
	if err != nil {
		response.InternalError(c, "取得叢集權限失敗: "+err.Error())
		return
	}

	var clusters []*models.Cluster
	if isAll {
		clusters, err = h.clusterService.GetAllClusters()
	} else if len(clusterIDs) > 0 {
		err = h.db.Where("id IN ?", clusterIDs).Find(&clusters).Error
	}
	if err != nil {
		response.InternalError(c, "取得叢集列表失敗: "+err.Error())
		return
	}

	total := 0
	now := time.Now()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	for _, cluster := range clusters {
		k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
		if err != nil {
			logger.Warn("SyncImages: 取得 k8s client 失敗", "cluster", cluster.Name, "err", err)
			continue
		}
		clientset := k8sClient.GetClientset()

		// 刪除舊索引
		h.db.Where("cluster_id = ?", cluster.ID).Delete(&models.ImageIndex{})

		var entries []models.ImageIndex

		// Deployments
		deps, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, d := range deps.Items {
				for _, ctr := range d.Spec.Template.Spec.Containers {
					imgName, imgTag := parseImageParts(ctr.Image)
					entries = append(entries, models.ImageIndex{
						ClusterID: cluster.ID, ClusterName: cluster.Name,
						Namespace: d.Namespace, WorkloadKind: "Deployment", WorkloadName: d.Name,
						ContainerName: ctr.Name, Image: ctr.Image,
						ImageName: imgName, ImageTag: imgTag, LastSyncAt: now,
					})
				}
			}
		}

		// StatefulSets
		sss, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, ss := range sss.Items {
				for _, ctr := range ss.Spec.Template.Spec.Containers {
					imgName, imgTag := parseImageParts(ctr.Image)
					entries = append(entries, models.ImageIndex{
						ClusterID: cluster.ID, ClusterName: cluster.Name,
						Namespace: ss.Namespace, WorkloadKind: "StatefulSet", WorkloadName: ss.Name,
						ContainerName: ctr.Name, Image: ctr.Image,
						ImageName: imgName, ImageTag: imgTag, LastSyncAt: now,
					})
				}
			}
		}

		// DaemonSets
		dss, err := clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, ds := range dss.Items {
				for _, ctr := range ds.Spec.Template.Spec.Containers {
					imgName, imgTag := parseImageParts(ctr.Image)
					entries = append(entries, models.ImageIndex{
						ClusterID: cluster.ID, ClusterName: cluster.Name,
						Namespace: ds.Namespace, WorkloadKind: "DaemonSet", WorkloadName: ds.Name,
						ContainerName: ctr.Name, Image: ctr.Image,
						ImageName: imgName, ImageTag: imgTag, LastSyncAt: now,
					})
				}
			}
		}

		if len(entries) > 0 {
			if err := h.db.CreateInBatches(entries, 100).Error; err != nil {
				logger.Warn("SyncImages: 寫入索引失敗", "cluster", cluster.Name, "err", err)
			} else {
				total += len(entries)
			}
		}
	}

	logger.Info("映像索引同步完成", "total", total)
	response.OK(c, gin.H{"message": "同步完成", "indexed": total})
}

// SearchImages 跨叢集搜尋映像
// GET /api/v1/images/search?q=nginx&tag=1.21&cluster=1&namespace=default
func (h *ImageHandler) SearchImages(c *gin.Context) {
	q := c.Query("q")         // 映像名稱（模糊）
	tag := c.Query("tag")     // tag（模糊）
	ns := c.Query("namespace")
	clusterIDStr := c.Query("cluster")

	page, limit := parsePagination(c)

	db := h.db.Model(&models.ImageIndex{})
	if q != "" {
		db = db.Where("image_name LIKE ? OR image LIKE ?", "%"+q+"%", "%"+q+"%")
	}
	if tag != "" {
		db = db.Where("image_tag LIKE ?", "%"+tag+"%")
	}
	if ns != "" {
		db = db.Where("namespace = ?", ns)
	}
	if clusterIDStr != "" {
		db = db.Where("cluster_id = ?", clusterIDStr)
	}

	var total int64
	db.Count(&total)

	var items []models.ImageIndex
	if err := db.Offset((page - 1) * limit).Limit(limit).Order("cluster_name, namespace, workload_name").Find(&items).Error; err != nil {
		response.InternalError(c, "搜尋失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"items": items, "total": total, "page": page, "limit": limit})
}

// GetImageSyncStatus 取得最後同步時間
// GET /api/v1/images/status
func (h *ImageHandler) GetImageSyncStatus(c *gin.Context) {
	var count int64
	var lastSync models.ImageIndex
	h.db.Model(&models.ImageIndex{}).Count(&count)
	h.db.Model(&models.ImageIndex{}).Order("last_sync_at desc").First(&lastSync)

	response.OK(c, gin.H{
		"indexed":    count,
		"lastSyncAt": lastSync.LastSyncAt,
	})
}

// parsePagination 解析 page / limit 參數（共用）
func parsePagination(c *gin.Context) (int, int) {
	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		if v, err := parseInt(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v, err := parseInt(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	return page, limit
}

func parseInt(s string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("not a number: %w", err)
	}
	return v, nil
}
