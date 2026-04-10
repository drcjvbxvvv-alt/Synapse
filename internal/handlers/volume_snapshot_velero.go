package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// ═══════════════════════════════════════════════════════════════════════════
// Velero
// ═══════════════════════════════════════════════════════════════════════════

// CheckVelero 偵測 Velero 是否安裝
// GET /clusters/:clusterID/velero/status?veleroNS=velero
func (h *VolumeSnapshotHandler) CheckVelero(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	ns := veleroNS(c)
	// Velero backups are namespace-scoped within the velero namespace
	_, err := dyn.Resource(veleroBackupGVR).Namespace(ns).List(ctx, metav1.ListOptions{Limit: 1})
	installed := err == nil
	response.OK(c, map[string]interface{}{"installed": installed, "namespace": ns})
}

// ListVeleroBackups 列出 Velero Backup
// GET /clusters/:clusterID/velero/backups?veleroNS=velero
func (h *VolumeSnapshotHandler) ListVeleroBackups(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	ctx, cancel := ctx30s(c)
	defer cancel()

	list, err := dyn.Resource(veleroBackupGVR).Namespace(veleroNS(c)).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 Velero Backup 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, obj := range list.Items {
		items = append(items, veleroBackupToInfo(&obj))
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// TriggerRestore 從指定 Backup 建立 Restore
// POST /clusters/:clusterID/velero/restores
// Body: { "backupName", "restoreName", "veleroNS", "includedNamespaces", "excludedNamespaces" }
func (h *VolumeSnapshotHandler) TriggerRestore(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}

	var req struct {
		BackupName         string   `json:"backupName" binding:"required"`
		RestoreName        string   `json:"restoreName"`
		VeleroNS           string   `json:"veleroNS"`
		IncludedNamespaces []string `json:"includedNamespaces"`
		ExcludedNamespaces []string `json:"excludedNamespaces"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	ns := req.VeleroNS
	if ns == "" {
		ns = "velero"
	}
	restoreName := req.RestoreName
	if restoreName == "" {
		restoreName = fmt.Sprintf("%s-restore-%d", req.BackupName, time.Now().Unix())
	}

	spec := map[string]interface{}{
		"backupName":        req.BackupName,
		"restorePVs":        true,
		"preserveNodePorts": false,
	}
	if len(req.IncludedNamespaces) > 0 {
		spec["includedNamespaces"] = req.IncludedNamespaces
	}
	if len(req.ExcludedNamespaces) > 0 {
		spec["excludedNamespaces"] = req.ExcludedNamespaces
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "velero.io/v1",
			"kind":       "Restore",
			"metadata":   map[string]interface{}{"name": restoreName, "namespace": ns},
			"spec":       spec,
		},
	}

	ctx, cancel := ctx30s(c)
	defer cancel()

	created, err := dyn.Resource(veleroRestoreGVR).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		response.InternalError(c, "建立 Restore 失敗: "+err.Error())
		return
	}
	logger.Info("觸發 Velero Restore", "backup", req.BackupName, "restore", restoreName)
	response.OK(c, veleroRestoreToInfo(created))
}

// ListVeleroRestores 列出 Velero Restore
// GET /clusters/:clusterID/velero/restores?veleroNS=velero
func (h *VolumeSnapshotHandler) ListVeleroRestores(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	ctx, cancel := ctx30s(c)
	defer cancel()

	list, err := dyn.Resource(veleroRestoreGVR).Namespace(veleroNS(c)).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 Velero Restore 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, obj := range list.Items {
		items = append(items, veleroRestoreToInfo(&obj))
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// ListVeleroSchedules 列出 Velero Schedule
// GET /clusters/:clusterID/velero/schedules?veleroNS=velero
func (h *VolumeSnapshotHandler) ListVeleroSchedules(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	ctx, cancel := ctx30s(c)
	defer cancel()

	list, err := dyn.Resource(veleroScheduleGVR).Namespace(veleroNS(c)).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 Velero Schedule 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, obj := range list.Items {
		items = append(items, veleroScheduleToInfo(&obj))
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// CreateVeleroSchedule 建立 Velero Schedule
// POST /clusters/:clusterID/velero/schedules
// Body: { "name", "schedule"(cron), "template", "veleroNS", "paused" }
func (h *VolumeSnapshotHandler) CreateVeleroSchedule(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}

	var req struct {
		Name               string   `json:"name" binding:"required"`
		Schedule           string   `json:"schedule" binding:"required"` // cron
		VeleroNS           string   `json:"veleroNS"`
		Paused             bool     `json:"paused"`
		IncludedNamespaces []string `json:"includedNamespaces"`
		StorageLocation    string   `json:"storageLocation"`
		TTL                string   `json:"ttl"` // e.g. "720h0m0s"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	ns := req.VeleroNS
	if ns == "" {
		ns = "velero"
	}

	template := map[string]interface{}{}
	if len(req.IncludedNamespaces) > 0 {
		template["includedNamespaces"] = req.IncludedNamespaces
	}
	if req.StorageLocation != "" {
		template["storageLocation"] = req.StorageLocation
	}
	if req.TTL != "" {
		template["ttl"] = req.TTL
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "velero.io/v1",
			"kind":       "Schedule",
			"metadata":   map[string]interface{}{"name": req.Name, "namespace": ns},
			"spec": map[string]interface{}{
				"schedule": req.Schedule,
				"paused":   req.Paused,
				"template": template,
			},
		},
	}

	ctx, cancel := ctx30s(c)
	defer cancel()

	created, err := dyn.Resource(veleroScheduleGVR).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		response.InternalError(c, "建立 Schedule 失敗: "+err.Error())
		return
	}
	logger.Info("建立 Velero Schedule", "name", req.Name, "schedule", req.Schedule)
	response.OK(c, veleroScheduleToInfo(created))
}

// DeleteVeleroSchedule 刪除 Velero Schedule
// DELETE /clusters/:clusterID/velero/schedules/:name?veleroNS=velero
func (h *VolumeSnapshotHandler) DeleteVeleroSchedule(c *gin.Context) {
	dyn, ok := h.dynClient(c)
	if !ok {
		return
	}
	ns := veleroNS(c)
	name := c.Param("name")
	ctx, cancel := ctx30s(c)
	defer cancel()

	if err := dyn.Resource(veleroScheduleGVR).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		response.InternalError(c, "刪除 Schedule 失敗: "+err.Error())
		return
	}
	response.OK(c, map[string]string{"message": "刪除成功"})
}
