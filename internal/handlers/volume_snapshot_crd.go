package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// ═══════════════════════════════════════════════════════════════════════════
// VolumeSnapshot
// ═══════════════════════════════════════════════════════════════════════════

// CheckVolumeSnapshotCRD 偵測 VolumeSnapshot CRD 是否安裝
// GET /clusters/:clusterID/volume-snapshots/status
func (h *VolumeSnapshotHandler) CheckVolumeSnapshotCRD(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	installed := isCRDPresent(ctx, dyn, volumeSnapshotGVR)
	response.OK(c, map[string]interface{}{"installed": installed})
}

// ListVolumeSnapshots 列出命名空間下的快照
// GET /clusters/:clusterID/volume-snapshots?namespace=&pvc=
func (h *VolumeSnapshotHandler) ListVolumeSnapshots(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	namespace := c.DefaultQuery("namespace", "")
	pvcFilter := c.Query("pvc")

	ctx, cancel := ctx30s(c)
	defer cancel()

	list, err := dyn.Resource(volumeSnapshotGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 VolumeSnapshot 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0)
	for _, obj := range list.Items {
		info := snapshotToInfo(&obj)
		if pvcFilter != "" && info["sourcePVC"] != pvcFilter {
			continue
		}
		items = append(items, info)
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// CreateVolumeSnapshot 對指定 PVC 建立快照
// POST /clusters/:clusterID/volume-snapshots
// Body: { "name", "namespace", "pvcName", "snapshotClassName" }
func (h *VolumeSnapshotHandler) CreateVolumeSnapshot(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}

	var req struct {
		Name              string `json:"name" binding:"required"`
		Namespace         string `json:"namespace" binding:"required"`
		PVCName           string `json:"pvcName" binding:"required"`
		SnapshotClassName string `json:"snapshotClassName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1",
			"kind":       "VolumeSnapshot",
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": req.Namespace,
			},
			"spec": buildSnapshotSpec(req.PVCName, req.SnapshotClassName),
		},
	}

	ctx, cancel := ctx30s(c)
	defer cancel()

	created, err := dyn.Resource(volumeSnapshotGVR).Namespace(req.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		response.InternalError(c, "建立 VolumeSnapshot 失敗: "+err.Error())
		return
	}

	logger.Info("建立 VolumeSnapshot", "namespace", req.Namespace, "name", req.Name, "pvc", req.PVCName)
	response.OK(c, snapshotToInfo(created))
}

// DeleteVolumeSnapshot 刪除快照
// DELETE /clusters/:clusterID/volume-snapshots/:namespace/:name
func (h *VolumeSnapshotHandler) DeleteVolumeSnapshot(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	ctx, cancel := ctx30s(c)
	defer cancel()

	if err := dyn.Resource(volumeSnapshotGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		response.InternalError(c, "刪除 VolumeSnapshot 失敗: "+err.Error())
		return
	}
	logger.Info("刪除 VolumeSnapshot", "namespace", namespace, "name", name)
	response.OK(c, map[string]string{"message": "刪除成功"})
}

// ListVolumeSnapshotClasses 列出快照類別
// GET /clusters/:clusterID/volume-snapshot-classes
func (h *VolumeSnapshotHandler) ListVolumeSnapshotClasses(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	ctx, cancel := ctx30s(c)
	defer cancel()

	list, err := dyn.Resource(volumeSnapshotClassGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 VolumeSnapshotClass 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, obj := range list.Items {
		spec, _ := obj.Object["spec"].(map[string]interface{})
		driver, _ := spec["driver"].(string)
		deletionPolicy, _ := spec["deletionPolicy"].(string)
		isDefault := obj.GetAnnotations()["snapshot.storage.kubernetes.io/is-default-class"] == "true"
		items = append(items, map[string]interface{}{
			"name":           obj.GetName(),
			"driver":         driver,
			"deletionPolicy": deletionPolicy,
			"isDefault":      isDefault,
			"createdAt":      obj.GetCreationTimestamp().Time,
		})
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}
