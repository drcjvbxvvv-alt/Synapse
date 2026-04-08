package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"gorm.io/gorm"
)

// PDBHandler PodDisruptionBudget 處理器
type PDBHandler struct {
	db             *gorm.DB
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

func NewPDBHandler(db *gorm.DB, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *PDBHandler {
	return &PDBHandler{db: db, clusterService: clusterService, k8sMgr: k8sMgr}
}

// PDBRequest 建立/更新 PDB 請求
type PDBRequest struct {
	Name           string            `json:"name" binding:"required"`
	Namespace      string            `json:"namespace" binding:"required"`
	Selector       map[string]string `json:"selector" binding:"required"` // matchLabels
	MinAvailable   *string           `json:"minAvailable"`   // 數字或百分比，如 "1" 或 "50%"
	MaxUnavailable *string           `json:"maxUnavailable"` // 數字或百分比，如 "1" 或 "25%"
}

func pdbToInfo(pdb *policyv1.PodDisruptionBudget) map[string]interface{} {
	var minAvail, maxUnavail string
	if pdb.Spec.MinAvailable != nil {
		minAvail = pdb.Spec.MinAvailable.String()
	}
	if pdb.Spec.MaxUnavailable != nil {
		maxUnavail = pdb.Spec.MaxUnavailable.String()
	}

	return map[string]interface{}{
		"name":                       pdb.Name,
		"namespace":                  pdb.Namespace,
		"selector":                   pdb.Spec.Selector.MatchLabels,
		"minAvailable":               minAvail,
		"maxUnavailable":             maxUnavail,
		"currentHealthy":             pdb.Status.CurrentHealthy,
		"desiredHealthy":             pdb.Status.DesiredHealthy,
		"expectedPods":               pdb.Status.ExpectedPods,
		"disruptionsAllowed":         pdb.Status.DisruptionsAllowed,
		"disruptedPods":              pdb.Status.DisruptedPods,
		"observedGeneration":         pdb.Status.ObservedGeneration,
		"createdAt":                  pdb.CreationTimestamp.Time,
	}
}

func buildPDBSpec(req *PDBRequest) policyv1.PodDisruptionBudgetSpec {
	spec := policyv1.PodDisruptionBudgetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: req.Selector,
		},
	}

	if req.MinAvailable != nil {
		v := intstr.Parse(*req.MinAvailable)
		spec.MinAvailable = &v
	} else if req.MaxUnavailable != nil {
		v := intstr.Parse(*req.MaxUnavailable)
		spec.MaxUnavailable = &v
	}

	return spec
}

// ListPDB 列出命名空間下的 PDB
func (h *PDBHandler) ListPDB(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Query("namespace")

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	list, err := k8sClient.GetClientset().PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 PDB 列表失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, pdbToInfo(&list.Items[i]))
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// GetWorkloadPDB 取得工作負載關聯的 PDB（依 matchLabels 比對）
func (h *PDBHandler) GetWorkloadPDB(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	list, err := k8sClient.GetClientset().PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 PDB 列表失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, pdbToInfo(&list.Items[i]))
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// CreatePDB 建立 PDB
func (h *PDBHandler) CreatePDB(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var req PDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}
	if req.MinAvailable == nil && req.MaxUnavailable == nil {
		response.BadRequest(c, "minAvailable 與 maxUnavailable 必須至少填寫一個")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: buildPDBSpec(&req),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	created, err := k8sClient.GetClientset().PolicyV1().PodDisruptionBudgets(req.Namespace).Create(ctx, pdb, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			response.BadRequest(c, fmt.Sprintf("PDB '%s' 已存在", req.Name))
			return
		}
		response.InternalError(c, "建立 PDB 失敗: "+err.Error())
		return
	}

	logger.Info("建立 PDB", "cluster", c.Param("clusterID"), "namespace", req.Namespace, "name", req.Name)
	response.OK(c, pdbToInfo(created))
}

// UpdatePDB 更新 PDB
func (h *PDBHandler) UpdatePDB(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req PDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	existing, err := k8sClient.GetClientset().PolicyV1().PodDisruptionBudgets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "PDB 不存在")
			return
		}
		response.InternalError(c, "取得 PDB 失敗: "+err.Error())
		return
	}

	existing.Spec = buildPDBSpec(&req)
	updated, err := k8sClient.GetClientset().PolicyV1().PodDisruptionBudgets(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "更新 PDB 失敗: "+err.Error())
		return
	}

	logger.Info("更新 PDB", "cluster", c.Param("clusterID"), "namespace", namespace, "name", name)
	response.OK(c, pdbToInfo(updated))
}

// DeletePDB 刪除 PDB
func (h *PDBHandler) DeletePDB(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := k8sClient.GetClientset().PolicyV1().PodDisruptionBudgets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "PDB 不存在")
			return
		}
		response.InternalError(c, "刪除 PDB 失敗: "+err.Error())
		return
	}

	logger.Info("刪除 PDB", "cluster", c.Param("clusterID"), "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "PDB 刪除成功"})
}
