package handlers

import (
	"github.com/gin-gonic/gin"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
)

// ResourceYAMLHandler 通用資源YAML處理器
type ResourceYAMLHandler struct {
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewResourceYAMLHandler 建立通用資源YAML處理器
func NewResourceYAMLHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *ResourceYAMLHandler {
	return &ResourceYAMLHandler{
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// ResourceYAMLApplyRequest 資源YAML應用請求
type ResourceYAMLApplyRequest struct {
	YAML   string `json:"yaml" binding:"required"`
	DryRun bool   `json:"dryRun"`
}

// ResourceYAMLResponse 資源YAML響應
type ResourceYAMLResponse struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace,omitempty"`
	Kind            string `json:"kind"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	IsCreated       bool   `json:"isCreated"` // true: 建立, false: 更新
}

// createK8sClient 獲取快取的 K8s 客戶端
func (h *ResourceYAMLHandler) createK8sClient(cluster *models.Cluster) (*services.K8sClient, error) {
	return h.k8sMgr.GetK8sClient(cluster)
}

// prepareK8sClient 通用初始化：解析 clusterID → 獲取叢集 → 建立客戶端
func (h *ResourceYAMLHandler) prepareK8sClient(c *gin.Context) (*services.K8sClient, bool) {
	clusterID := c.Param("clusterID")
	id, err := parseClusterID(clusterID)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return nil, false
	}
	cluster, err := h.clusterService.GetCluster(id)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return nil, false
	}
	k8sClient, err := h.createK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "建立K8s客戶端失敗: "+err.Error())
		return nil, false
	}
	return k8sClient, true
}

// respondWithYAML 通用 YAML GET 響應：清理 ManagedFields、設定 TypeMeta、序列化
func respondWithYAML(c *gin.Context, obj interface{}) {
	yamlBytes, err := sigsyaml.Marshal(obj)
	if err != nil {
		response.InternalError(c, "轉換YAML失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"yaml": string(yamlBytes)})
}
