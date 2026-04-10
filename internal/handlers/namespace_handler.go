package handlers

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/services"
)

type NamespaceHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

func NewNamespaceHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *NamespaceHandler {
	return &NamespaceHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// NamespaceResponse 命名空間響應結構
type NamespaceResponse struct {
	Name              string             `json:"name"`
	Status            string             `json:"status"`
	Labels            map[string]string  `json:"labels"`
	Annotations       map[string]string  `json:"annotations"`
	CreationTimestamp string             `json:"creationTimestamp"`
	ResourceQuota     *ResourceQuotaInfo `json:"resourceQuota,omitempty"`
}

// ResourceQuotaInfo 資源配額資訊
type ResourceQuotaInfo struct {
	Hard map[string]string `json:"hard"`
	Used map[string]string `json:"used"`
}

// convertResourceList 轉換資源列表為字串map
func convertResourceList(rl corev1.ResourceList) map[string]string {
	result := make(map[string]string)
	for k, v := range rl {
		result[string(k)] = v.String()
	}
	return result
}
