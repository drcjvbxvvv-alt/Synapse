package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ─── GVRs ───────────────────────────────────────────────────────────────────

var (
	volumeSnapshotGVR      = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	volumeSnapshotClassGVR = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses"}

	veleroBackupGVR   = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "backups"}
	veleroRestoreGVR  = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "restores"}
	veleroScheduleGVR = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "schedules"}
)

// ─── Handler ────────────────────────────────────────────────────────────────

type VolumeSnapshotHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

func NewVolumeSnapshotHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *VolumeSnapshotHandler {
	return &VolumeSnapshotHandler{clusterService: clusterService, k8sMgr: k8sMgr}
}

// dynClient 共用輔助：取得 dynamic client
func (h *VolumeSnapshotHandler) dynClient(c *gin.Context) (dynamic.Interface, bool) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return nil, false
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return nil, false
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return nil, false
	}
	dyn, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		response.InternalError(c, "建立 dynamic client 失敗: "+err.Error())
		return nil, false
	}
	return dyn, true
}

func ctx30s(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 30*time.Second)
}

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

// ─── Helpers ────────────────────────────────────────────────────────────────

func buildSnapshotSpec(pvcName, snapshotClassName string) map[string]interface{} {
	spec := map[string]interface{}{
		"source": map[string]interface{}{
			"persistentVolumeClaimName": pvcName,
		},
	}
	if snapshotClassName != "" {
		spec["volumeSnapshotClassName"] = snapshotClassName
	}
	return spec
}

func snapshotToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})

	source, _ := spec["source"].(map[string]interface{})
	sourcePVC, _ := source["persistentVolumeClaimName"].(string)
	snapshotClassName, _ := spec["volumeSnapshotClassName"].(string)

	readyToUse, _ := status["readyToUse"].(bool)
	restoreSize, _ := status["restoreSize"].(string)
	boundContentName, _ := status["boundVolumeSnapshotContentName"].(string)

	// error message if any
	errMsg := ""
	if errObj, ok := status["error"].(map[string]interface{}); ok {
		errMsg, _ = errObj["message"].(string)
	}

	return map[string]interface{}{
		"name":              obj.GetName(),
		"namespace":         obj.GetNamespace(),
		"sourcePVC":         sourcePVC,
		"snapshotClassName": snapshotClassName,
		"readyToUse":        readyToUse,
		"restoreSize":       restoreSize,
		"boundContentName":  boundContentName,
		"error":             errMsg,
		"createdAt":         obj.GetCreationTimestamp().Time,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Velero
// ═══════════════════════════════════════════════════════════════════════════

// veleroNS detects the namespace where Velero is installed (default: velero)
func veleroNS(c *gin.Context) string {
	ns := c.DefaultQuery("veleroNS", "velero")
	return ns
}

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
		BackupName          string   `json:"backupName" binding:"required"`
		RestoreName         string   `json:"restoreName"`
		VeleroNS            string   `json:"veleroNS"`
		IncludedNamespaces  []string `json:"includedNamespaces"`
		ExcludedNamespaces  []string `json:"excludedNamespaces"`
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
		"backupName":                req.BackupName,
		"restorePVs":                true,
		"preserveNodePorts":         false,
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
		Name               string                 `json:"name" binding:"required"`
		Schedule           string                 `json:"schedule" binding:"required"` // cron
		VeleroNS           string                 `json:"veleroNS"`
		Paused             bool                   `json:"paused"`
		IncludedNamespaces []string               `json:"includedNamespaces"`
		StorageLocation    string                 `json:"storageLocation"`
		TTL                string                 `json:"ttl"` // e.g. "720h0m0s"
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

// ─── Velero info converters ──────────────────────────────────────────────────

func veleroBackupToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})

	phase, _ := status["phase"].(string)
	startTimestamp, _ := status["startTimestamp"].(string)
	completionTimestamp, _ := status["completionTimestamp"].(string)
	expiration, _ := status["expiration"].(string)
	progress, _ := status["progress"].(map[string]interface{})

	includedNS, _ := spec["includedNamespaces"].([]interface{})
	storageLocation, _ := spec["storageLocation"].(string)
	ttl, _ := spec["ttl"].(string)

	return map[string]interface{}{
		"name":                obj.GetName(),
		"namespace":           obj.GetNamespace(),
		"phase":               phase,
		"includedNamespaces":  includedNS,
		"storageLocation":     storageLocation,
		"ttl":                 ttl,
		"startTimestamp":      startTimestamp,
		"completionTimestamp": completionTimestamp,
		"expiration":          expiration,
		"progress":            progress,
		"createdAt":           obj.GetCreationTimestamp().Time,
	}
}

func veleroRestoreToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})

	backupName, _ := spec["backupName"].(string)
	phase, _ := status["phase"].(string)
	warnings, _ := status["warnings"].(int64)
	errors, _ := status["errors"].(int64)

	return map[string]interface{}{
		"name":       obj.GetName(),
		"namespace":  obj.GetNamespace(),
		"backupName": backupName,
		"phase":      phase,
		"warnings":   warnings,
		"errors":     errors,
		"createdAt":  obj.GetCreationTimestamp().Time,
	}
}

func veleroScheduleToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})

	schedule, _ := spec["schedule"].(string)
	paused, _ := spec["paused"].(bool)
	lastBackup, _ := status["lastBackup"].(string)
	phase, _ := status["phase"].(string)

	tmpl, _ := spec["template"].(map[string]interface{})
	storageLocation, _ := tmpl["storageLocation"].(string)
	ttl, _ := tmpl["ttl"].(string)

	return map[string]interface{}{
		"name":            obj.GetName(),
		"namespace":       obj.GetNamespace(),
		"schedule":        schedule,
		"paused":          paused,
		"phase":           phase,
		"lastBackup":      lastBackup,
		"storageLocation": storageLocation,
		"ttl":             ttl,
		"createdAt":       obj.GetCreationTimestamp().Time,
	}
}
