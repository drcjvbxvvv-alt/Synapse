package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
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

// veleroNS detects the namespace where Velero is installed (default: velero)
func veleroNS(c *gin.Context) string {
	ns := c.DefaultQuery("veleroNS", "velero")
	return ns
}
