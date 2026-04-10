package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

/*
*
GetClusterEvents 獲取叢集 K8s 事件列表
GET /api/v1/clusters/:clusterID/events?search=xxx&type=Normal|Warning
返回前端定義的 K8sEvent 陣列（不分頁）
*/
func (h *ClusterHandler) GetClusterEvents(c *gin.Context) {
	idStr := c.Param("clusterID")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err)
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	cs := k8sClient.GetClientset()

	// 拉取所有命名空間的 core/v1 Event
	evList, err := cs.CoreV1().Events("").List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取K8s事件失敗", "error", err)
		response.InternalError(c, "獲取K8s事件失敗: "+err.Error())
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	ftype := strings.TrimSpace(c.Query("type"))

	out := make([]gin.H, 0, len(evList.Items))
	for _, e := range evList.Items {
		// 型別過濾
		if ftype != "" && !strings.EqualFold(e.Type, ftype) {
			continue
		}
		// 關鍵字過濾（物件kind/name/ns、reason、message）
		if search != "" {
			s := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(e.InvolvedObject.Kind), s) &&
				!strings.Contains(strings.ToLower(e.InvolvedObject.Name), s) &&
				!strings.Contains(strings.ToLower(e.InvolvedObject.Namespace), s) &&
				!strings.Contains(strings.ToLower(e.Reason), s) &&
				!strings.Contains(strings.ToLower(e.Message), s) {
				continue
			}
		}

		// 發生時間優先順序：lastTimestamp > eventTime > firstTimestamp > metadata.creationTimestamp
		var lastTS string
		if !e.LastTimestamp.IsZero() {
			lastTS = e.LastTimestamp.Time.UTC().Format(time.RFC3339)
		} else if !e.EventTime.IsZero() {
			lastTS = e.EventTime.Time.UTC().Format(time.RFC3339)
		} else if !e.FirstTimestamp.IsZero() {
			lastTS = e.FirstTimestamp.Time.UTC().Format(time.RFC3339)
		} else if !e.CreationTimestamp.IsZero() {
			lastTS = e.ObjectMeta.CreationTimestamp.Time.UTC().Format(time.RFC3339)
		}

		out = append(out, gin.H{
			"metadata": gin.H{
				"uid":       string(e.UID),
				"name":      e.Name,
				"namespace": e.Namespace,
				"creationTimestamp": func() string {
					if e.CreationTimestamp.IsZero() {
						return ""
					}
					return e.CreationTimestamp.Time.UTC().Format(time.RFC3339)
				}(),
			},
			"involvedObject": gin.H{
				"kind":       e.InvolvedObject.Kind,
				"name":       e.InvolvedObject.Name,
				"namespace":  e.InvolvedObject.Namespace,
				"uid":        string(e.InvolvedObject.UID),
				"apiVersion": e.InvolvedObject.APIVersion,
				"fieldPath":  e.InvolvedObject.FieldPath,
			},
			"type":    e.Type,
			"reason":  e.Reason,
			"message": e.Message,
			"source":  gin.H{"component": e.Source.Component, "host": e.Source.Host},
			"firstTimestamp": func() string {
				if e.FirstTimestamp.IsZero() {
					return ""
				}
				return e.FirstTimestamp.Time.UTC().Format(time.RFC3339)
			}(),
			"lastTimestamp": lastTS,
			"eventTime": func() string {
				if e.EventTime.IsZero() {
					return ""
				}
				return e.EventTime.Time.UTC().Format(time.RFC3339)
			}(),
			"count": e.Count,
		})
	}

	response.OK(c, out)
}
