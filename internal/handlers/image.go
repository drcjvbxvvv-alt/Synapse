package handlers

import (
	"context"
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
)

// ImageHandler 映像索引處理器
type ImageHandler struct {
	imageSvc      *services.ImageIndexService
	clusterSvc    *services.ClusterService
	permissionSvc *services.PermissionService
	k8sMgr        *k8s.ClusterInformerManager
}

func NewImageHandler(imageSvc *services.ImageIndexService, clusterSvc *services.ClusterService, permissionSvc *services.PermissionService, k8sMgr *k8s.ClusterInformerManager) *ImageHandler {
	return &ImageHandler{imageSvc: imageSvc, clusterSvc: clusterSvc, permissionSvc: permissionSvc, k8sMgr: k8sMgr}
}

// parseImageParts 分離映像名稱與 tag
func parseImageParts(image string) (name, tag string) {
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		return image[:idx], image[idx+1:]
	}
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return image, "latest"
	}
	afterColon := image[lastColon+1:]
	if strings.Contains(afterColon, "/") {
		return image, "latest"
	}
	return image[:lastColon], afterColon
}

// SyncImages POST /api/v1/images/sync
func (h *ImageHandler) SyncImages(c *gin.Context) {
	userID := c.GetUint("user_id")

	clusterIDs, isAll, err := h.permissionSvc.GetUserAccessibleClusterIDs(userID)
	if err != nil {
		response.InternalError(c, "取得叢集權限失敗: "+err.Error())
		return
	}

	total := 0
	now := time.Now()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	var clusters []*models.Cluster
	if isAll {
		clusters, err = h.clusterSvc.GetAllClusters(ctx)
	} else if len(clusterIDs) > 0 {
		clusters, err = h.clusterSvc.GetClustersByIDs(ctx, clusterIDs)
	}
	if err != nil {
		response.InternalError(c, "取得叢集列表失敗: "+err.Error())
		return
	}

	for _, cluster := range clusters {
		k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
		if err != nil {
			logger.Warn("SyncImages: 取得 k8s client 失敗", "cluster", cluster.Name, "err", err)
			continue
		}
		clientset := k8sClient.GetClientset()

		h.imageSvc.DeleteByCluster(ctx, cluster.ID)

		var entries []models.ImageIndex

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

		if err := h.imageSvc.BulkCreate(ctx, entries); err != nil {
			logger.Warn("SyncImages: 寫入索引失敗", "cluster", cluster.Name, "err", err)
		} else {
			total += len(entries)
		}
	}

	logger.Info("映像索引同步完成", "total", total)
	response.OK(c, gin.H{"message": "同步完成", "indexed": total})
}

// SearchImages GET /api/v1/images/search
func (h *ImageHandler) SearchImages(c *gin.Context) {
	page, limit := parsePagination(c)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	items, total, err := h.imageSvc.SearchImages(ctx, services.SearchImagesParams{
		Query:     c.Query("q"),
		Tag:       c.Query("tag"),
		Namespace: c.Query("namespace"),
		ClusterID: c.Query("cluster"),
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		response.InternalError(c, "搜尋失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "limit": limit})
}

// GetImageSyncStatus GET /api/v1/images/status
func (h *ImageHandler) GetImageSyncStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	count, lastSyncAt := h.imageSvc.GetSyncStatus(ctx)
	response.OK(c, gin.H{
		"indexed":    count,
		"lastSyncAt": lastSyncAt,
	})
}

// parsePagination 解析 page / limit 參數（共用）
func parsePagination(c *gin.Context) (int, int) {
	page := 1
	limit := 20
	if p, err := parseInt(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := parseInt(c.Query("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	return page, limit
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
