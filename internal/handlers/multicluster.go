package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"gorm.io/gorm"
)

// MultiClusterHandler 多叢集工作流程處理器
type MultiClusterHandler struct {
	db             *gorm.DB
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewMultiClusterHandler 建立多叢集處理器
func NewMultiClusterHandler(db *gorm.DB, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *MultiClusterHandler {
	return &MultiClusterHandler{db: db, clusterService: clusterService, k8sMgr: k8sMgr}
}

// ─── 遷移 ────────────────────────────────────────────────────────────────────

// MigrateRequest 遷移請求
type MigrateRequest struct {
	SourceClusterID uint   `json:"sourceClusterId" binding:"required"`
	SourceNamespace string `json:"sourceNamespace" binding:"required"`
	WorkloadKind    string `json:"workloadKind" binding:"required"` // Deployment / StatefulSet / DaemonSet
	WorkloadName    string `json:"workloadName" binding:"required"`
	TargetClusterID uint   `json:"targetClusterId" binding:"required"`
	TargetNamespace string `json:"targetNamespace" binding:"required"`
	SyncConfigMaps  bool   `json:"syncConfigMaps"`
	SyncSecrets     bool   `json:"syncSecrets"`
}

// MigrateCheckRequest 遷移預檢請求（同 MigrateRequest）
type MigrateCheckRequest = MigrateRequest

// MigrateCheckResult 預檢結果
type MigrateCheckResult struct {
	Feasible        bool    `json:"feasible"`
	Message         string  `json:"message"`
	WorkloadCPUReq  float64 `json:"workloadCpuReq"`  // millicores
	WorkloadMemReq  float64 `json:"workloadMemReq"`  // MiB
	TargetFreeCPU   float64 `json:"targetFreeCpu"`   // millicores
	TargetFreeMem   float64 `json:"targetFreeMem"`   // MiB
	ConfigMapCount  int     `json:"configMapCount"`
	SecretCount     int     `json:"secretCount"`
}

// MigrateResult 遷移結果
type MigrateResult struct {
	Success         bool     `json:"success"`
	WorkloadCreated bool     `json:"workloadCreated"`
	ConfigMapsSynced []string `json:"configMapsSynced"`
	SecretsSynced   []string `json:"secretsSynced"`
	Message         string   `json:"message"`
}

// getClientByID 根據叢集 ID 取得 K8s 客戶端
func (h *MultiClusterHandler) getClientByID(id uint) (kubernetes.Interface, error) {
	cluster, err := h.clusterService.GetCluster(id)
	if err != nil {
		return nil, fmt.Errorf("叢集 %d 不存在: %v", id, err)
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("取得叢集 %d 客戶端失敗: %v", id, err)
	}
	return k8sClient.GetClientset(), nil
}

// MigrateCheck POST /multicluster/migrate/check — 遷移預檢
func (h *MultiClusterHandler) MigrateCheck(c *gin.Context) {
	var req MigrateCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	srcClient, err := h.getClientByID(req.SourceClusterID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	dstClient, err := h.getClientByID(req.TargetClusterID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	result := MigrateCheckResult{}

	// 1. 取得工作負載資源請求量
	cpuReq, memReq, cmNames, secNames, err := h.getWorkloadResources(ctx, srcClient, req)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "WORKLOAD_NOT_FOUND", err.Error())
		return
	}
	result.WorkloadCPUReq = cpuReq
	result.WorkloadMemReq = memReq
	result.ConfigMapCount = len(cmNames)
	result.SecretCount = len(secNames)

	// 2. 計算目標叢集可用資源
	freeCPU, freeMem, err := h.calcFreeResources(ctx, dstClient)
	if err != nil {
		logger.Warn("無法取得目標叢集資源", "error", err)
		// 不阻斷，降級為可行
		result.Feasible = true
		result.Message = "無法取得目標叢集資源用量，請確認目標叢集有足夠資源"
		response.OK(c, result)
		return
	}
	result.TargetFreeCPU = freeCPU
	result.TargetFreeMem = freeMem

	if cpuReq > freeCPU || memReq > freeMem {
		result.Feasible = false
		result.Message = fmt.Sprintf("目標叢集資源不足（需要 CPU %.0fm / MEM %.0fMiB，可用 CPU %.0fm / MEM %.0fMiB）",
			cpuReq, memReq, freeCPU, freeMem)
	} else {
		result.Feasible = true
		result.Message = "資源充足，可以執行遷移"
	}
	response.OK(c, result)
}

// Migrate POST /multicluster/migrate — 執行遷移
func (h *MultiClusterHandler) Migrate(c *gin.Context) {
	var req MigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	srcClient, err := h.getClientByID(req.SourceClusterID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	dstClient, err := h.getClientByID(req.TargetClusterID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	result := MigrateResult{}

	// 1. 確保目標命名空間存在
	if err := ensureNamespace(ctx, dstClient, req.TargetNamespace); err != nil {
		response.InternalError(c, fmt.Sprintf("建立命名空間失敗: %v", err))
		return
	}

	// 2. 同步 ConfigMap
	if req.SyncConfigMaps {
		synced, err := h.syncConfigMaps(ctx, srcClient, dstClient, req.SourceNamespace, req.TargetNamespace, req.WorkloadName)
		if err != nil {
			logger.Warn("同步 ConfigMap 部分失敗", "error", err)
		}
		result.ConfigMapsSynced = synced
	}

	// 3. 同步 Secret
	if req.SyncSecrets {
		synced, err := h.syncSecrets(ctx, srcClient, dstClient, req.SourceNamespace, req.TargetNamespace, req.WorkloadName)
		if err != nil {
			logger.Warn("同步 Secret 部分失敗", "error", err)
		}
		result.SecretsSynced = synced
	}

	// 4. 遷移工作負載
	if err := h.migrateWorkload(ctx, srcClient, dstClient, req); err != nil {
		response.InternalError(c, fmt.Sprintf("遷移工作負載失敗: %v", err))
		return
	}
	result.WorkloadCreated = true
	result.Success = true
	result.Message = fmt.Sprintf("%s/%s 已成功遷移至叢集 %d 的命名空間 %s",
		req.WorkloadKind, req.WorkloadName, req.TargetClusterID, req.TargetNamespace)

	response.OK(c, result)
}

// ─── 同步策略 CRUD ────────────────────────────────────────────────────────────

// ListSyncPolicies GET /multicluster/sync-policies
func (h *MultiClusterHandler) ListSyncPolicies(c *gin.Context) {
	var policies []models.SyncPolicy
	if err := h.db.Order("id desc").Find(&policies).Error; err != nil {
		response.InternalError(c, "查詢同步策略失敗")
		return
	}
	response.List(c, policies, int64(len(policies)))
}

// CreateSyncPolicy POST /multicluster/sync-policies
func (h *MultiClusterHandler) CreateSyncPolicy(c *gin.Context) {
	var policy models.SyncPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.db.Create(&policy).Error; err != nil {
		response.InternalError(c, "建立同步策略失敗")
		return
	}
	response.Created(c, policy)
}

// GetSyncPolicy GET /multicluster/sync-policies/:id
func (h *MultiClusterHandler) GetSyncPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "無效 ID")
		return
	}
	var policy models.SyncPolicy
	if err := h.db.First(&policy, id).Error; err != nil {
		response.NotFound(c, "同步策略不存在")
		return
	}
	response.OK(c, policy)
}

// UpdateSyncPolicy PUT /multicluster/sync-policies/:id
func (h *MultiClusterHandler) UpdateSyncPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "無效 ID")
		return
	}
	var policy models.SyncPolicy
	if err := h.db.First(&policy, id).Error; err != nil {
		response.NotFound(c, "同步策略不存在")
		return
	}
	if err := c.ShouldBindJSON(&policy); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	policy.ID = uint(id)
	if err := h.db.Save(&policy).Error; err != nil {
		response.InternalError(c, "更新同步策略失敗")
		return
	}
	response.OK(c, policy)
}

// DeleteSyncPolicy DELETE /multicluster/sync-policies/:id
func (h *MultiClusterHandler) DeleteSyncPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "無效 ID")
		return
	}
	if err := h.db.Delete(&models.SyncPolicy{}, id).Error; err != nil {
		response.InternalError(c, "刪除同步策略失敗")
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// TriggerSync POST /multicluster/sync-policies/:id/trigger
func (h *MultiClusterHandler) TriggerSync(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "無效 ID")
		return
	}
	var policy models.SyncPolicy
	if err := h.db.First(&policy, id).Error; err != nil {
		response.NotFound(c, "同步策略不存在")
		return
	}

	now := time.Now()
	hist := models.SyncHistory{
		PolicyID:    policy.ID,
		TriggeredBy: "manual",
		StartedAt:   now,
	}

	status, message, details := h.executeSync(&policy)
	hist.Status = status
	hist.Message = message
	hist.Details = details
	finished := time.Now()
	hist.FinishedAt = &finished

	h.db.Create(&hist)

	// 更新策略最後同步狀態
	h.db.Model(&policy).Updates(map[string]interface{}{
		"last_sync_at":     now,
		"last_sync_status": status,
	})

	response.OK(c, hist)
}

// GetSyncHistory GET /multicluster/sync-policies/:id/history
func (h *MultiClusterHandler) GetSyncHistory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "無效 ID")
		return
	}
	var history []models.SyncHistory
	h.db.Where("policy_id = ?", id).Order("id desc").Limit(50).Find(&history)
	response.List(c, history, int64(len(history)))
}

// ─── 內部輔助函式 ──────────────────────────────────────────────────────────────

// getWorkloadResources 取得工作負載的資源請求量及相依的 ConfigMap/Secret 名稱
func (h *MultiClusterHandler) getWorkloadResources(
	ctx context.Context, client kubernetes.Interface, req MigrateRequest,
) (cpuMillicores, memMiB float64, cmNames, secNames []string, err error) {
	switch req.WorkloadKind {
	case "Deployment":
		dep, e := client.AppsV1().Deployments(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if e != nil {
			return 0, 0, nil, nil, fmt.Errorf("找不到 Deployment %s: %v", req.WorkloadName, e)
		}
		cpuMillicores, memMiB = sumContainerRequests(dep.Spec.Template.Spec.Containers)
		cmNames, secNames = extractEnvRefs(dep.Spec.Template.Spec)
	case "StatefulSet":
		sts, e := client.AppsV1().StatefulSets(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if e != nil {
			return 0, 0, nil, nil, fmt.Errorf("找不到 StatefulSet %s: %v", req.WorkloadName, e)
		}
		cpuMillicores, memMiB = sumContainerRequests(sts.Spec.Template.Spec.Containers)
		cmNames, secNames = extractEnvRefs(sts.Spec.Template.Spec)
	case "DaemonSet":
		ds, e := client.AppsV1().DaemonSets(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if e != nil {
			return 0, 0, nil, nil, fmt.Errorf("找不到 DaemonSet %s: %v", req.WorkloadName, e)
		}
		cpuMillicores, memMiB = sumContainerRequests(ds.Spec.Template.Spec.Containers)
		cmNames, secNames = extractEnvRefs(ds.Spec.Template.Spec)
	default:
		err = fmt.Errorf("不支援的工作負載型別: %s", req.WorkloadKind)
	}
	return
}

// sumContainerRequests 加總所有容器的資源請求
func sumContainerRequests(containers []corev1.Container) (cpuMillicores, memMiB float64) {
	for _, c := range containers {
		if req := c.Resources.Requests; req != nil {
			if cpu, ok := req[corev1.ResourceCPU]; ok {
				cpuMillicores += float64(cpu.MilliValue())
			}
			if mem, ok := req[corev1.ResourceMemory]; ok {
				memMiB += float64(mem.Value()) / (1024 * 1024)
			}
		}
	}
	return
}

// extractEnvRefs 從 PodSpec 提取 ConfigMap 和 Secret 引用名稱
func extractEnvRefs(spec corev1.PodSpec) (cmNames, secNames []string) {
	cmSet := map[string]bool{}
	secSet := map[string]bool{}
	for _, c := range spec.Containers {
		for _, env := range c.Env {
			if env.ValueFrom != nil {
				if r := env.ValueFrom.ConfigMapKeyRef; r != nil {
					cmSet[r.Name] = true
				}
				if r := env.ValueFrom.SecretKeyRef; r != nil {
					secSet[r.Name] = true
				}
			}
		}
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef != nil {
				cmSet[ef.ConfigMapRef.Name] = true
			}
			if ef.SecretRef != nil {
				secSet[ef.SecretRef.Name] = true
			}
		}
	}
	for v := range cmSet {
		cmNames = append(cmNames, v)
	}
	for v := range secSet {
		secNames = append(secNames, v)
	}
	return
}

// calcFreeResources 計算叢集可用資源（allocatable - requested）
func (h *MultiClusterHandler) calcFreeResources(ctx context.Context, client kubernetes.Interface) (freeCPU, freeMem float64, err error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, 0, err
	}
	var totalCPU, totalMem float64
	for _, n := range nodes.Items {
		if cpu, ok := n.Status.Allocatable[corev1.ResourceCPU]; ok {
			totalCPU += float64(cpu.MilliValue())
		}
		if mem, ok := n.Status.Allocatable[corev1.ResourceMemory]; ok {
			totalMem += float64(mem.Value()) / (1024 * 1024)
		}
	}

	pods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: "status.phase=Running"})
	if err != nil {
		return 0, 0, err
	}
	var usedCPU, usedMem float64
	for _, pod := range pods.Items {
		for _, c := range pod.Spec.Containers {
			if req := c.Resources.Requests; req != nil {
				if cpu, ok := req[corev1.ResourceCPU]; ok {
					usedCPU += float64(cpu.MilliValue())
				}
				if mem, ok := req[corev1.ResourceMemory]; ok {
					usedMem += float64(mem.Value()) / (1024 * 1024)
				}
			}
		}
	}
	return totalCPU - usedCPU, totalMem - usedMem, nil
}

// ensureNamespace 確保命名空間存在，不存在則建立
func ensureNamespace(ctx context.Context, client kubernetes.Interface, ns string) error {
	_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}, metav1.CreateOptions{})
	return err
}

// syncConfigMaps 同步與工作負載關聯的 ConfigMap
func (h *MultiClusterHandler) syncConfigMaps(
	ctx context.Context, src, dst kubernetes.Interface, srcNS, dstNS, workload string,
) ([]string, error) {
	cms, err := src.CoreV1().ConfigMaps(srcNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var synced []string
	for _, cm := range cms.Items {
		// 跳過系統 ConfigMap
		if cm.Name == "kube-root-ca.crt" {
			continue
		}
		clean := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cm.Name, Namespace: dstNS, Labels: cm.Labels, Annotations: cm.Annotations},
			Data:       cm.Data,
			BinaryData: cm.BinaryData,
		}
		_, err := dst.CoreV1().ConfigMaps(dstNS).Get(ctx, cm.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.CoreV1().ConfigMaps(dstNS).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			_, err = dst.CoreV1().ConfigMaps(dstNS).Update(ctx, clean, metav1.UpdateOptions{})
		}
		if err == nil {
			synced = append(synced, cm.Name)
		}
	}
	return synced, nil
}

// syncSecrets 同步與工作負載關聯的 Secret（跳過 SA token 等系統型別）
func (h *MultiClusterHandler) syncSecrets(
	ctx context.Context, src, dst kubernetes.Interface, srcNS, dstNS, workload string,
) ([]string, error) {
	secrets, err := src.CoreV1().Secrets(srcNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var synced []string
	for _, sec := range secrets.Items {
		if sec.Type == corev1.SecretTypeServiceAccountToken ||
			sec.Type == "kubernetes.io/dockerconfigjson" && sec.Name == "default" {
			continue
		}
		clean := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: sec.Name, Namespace: dstNS, Labels: sec.Labels, Annotations: sec.Annotations},
			Type:       sec.Type,
			Data:       sec.Data,
		}
		_, err := dst.CoreV1().Secrets(dstNS).Get(ctx, sec.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.CoreV1().Secrets(dstNS).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			_, err = dst.CoreV1().Secrets(dstNS).Update(ctx, clean, metav1.UpdateOptions{})
		}
		if err == nil {
			synced = append(synced, sec.Name)
		}
	}
	return synced, nil
}

// migrateWorkload 遷移工作負載到目標叢集
func (h *MultiClusterHandler) migrateWorkload(ctx context.Context, src, dst kubernetes.Interface, req MigrateRequest) error {
	switch req.WorkloadKind {
	case "Deployment":
		dep, err := src.AppsV1().Deployments(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clean := cleanDeployment(dep, req.TargetNamespace)
		_, err = dst.AppsV1().Deployments(req.TargetNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.AppsV1().Deployments(req.TargetNamespace).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			clean.ResourceVersion = ""
			_, err = dst.AppsV1().Deployments(req.TargetNamespace).Update(ctx, clean, metav1.UpdateOptions{})
		}
		return err
	case "StatefulSet":
		sts, err := src.AppsV1().StatefulSets(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clean := cleanStatefulSet(sts, req.TargetNamespace)
		_, err = dst.AppsV1().StatefulSets(req.TargetNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.AppsV1().StatefulSets(req.TargetNamespace).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			clean.ResourceVersion = ""
			_, err = dst.AppsV1().StatefulSets(req.TargetNamespace).Update(ctx, clean, metav1.UpdateOptions{})
		}
		return err
	case "DaemonSet":
		ds, err := src.AppsV1().DaemonSets(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clean := cleanDaemonSet(ds, req.TargetNamespace)
		_, err = dst.AppsV1().DaemonSets(req.TargetNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.AppsV1().DaemonSets(req.TargetNamespace).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			clean.ResourceVersion = ""
			_, err = dst.AppsV1().DaemonSets(req.TargetNamespace).Update(ctx, clean, metav1.UpdateOptions{})
		}
		return err
	}
	return fmt.Errorf("不支援的工作負載型別: %s", req.WorkloadKind)
}

// cleanDeployment 去除執行時欄位，準備跨叢集 apply
func cleanDeployment(dep *appsv1.Deployment, targetNS string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dep.Name,
			Namespace:   targetNS,
			Labels:      dep.Labels,
			Annotations: filterAnnotations(dep.Annotations),
		},
		Spec: dep.Spec,
	}
}

func cleanStatefulSet(sts *appsv1.StatefulSet, targetNS string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        sts.Name,
			Namespace:   targetNS,
			Labels:      sts.Labels,
			Annotations: filterAnnotations(sts.Annotations),
		},
		Spec: sts.Spec,
	}
}

func cleanDaemonSet(ds *appsv1.DaemonSet, targetNS string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ds.Name,
			Namespace:   targetNS,
			Labels:      ds.Labels,
			Annotations: filterAnnotations(ds.Annotations),
		},
		Spec: ds.Spec,
	}
}

// filterAnnotations 過濾掉 kubectl/k8s 系統管理用 annotation
func filterAnnotations(ann map[string]string) map[string]string {
	skip := map[string]bool{
		"kubectl.kubernetes.io/last-applied-configuration": true,
		"deployment.kubernetes.io/revision":                true,
	}
	result := map[string]string{}
	for k, v := range ann {
		if !skip[k] {
			result[k] = v
		}
	}
	return result
}

// ─── 同步執行 ─────────────────────────────────────────────────────────────────

type syncDetailItem struct {
	ClusterID uint   `json:"clusterId"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// executeSync 執行配置同步策略，回傳 status / message / details JSON
func (h *MultiClusterHandler) executeSync(policy *models.SyncPolicy) (status, message, details string) {
	var targetIDs []uint
	if err := json.Unmarshal([]byte(policy.TargetClusters), &targetIDs); err != nil {
		return "failed", "解析目標叢集列表失敗: " + err.Error(), ""
	}
	var resourceNames []string
	_ = json.Unmarshal([]byte(policy.ResourceNames), &resourceNames)

	srcClient, err := h.getClientByID(policy.SourceClusterID)
	if err != nil {
		return "failed", "取得來源叢集客戶端失敗: " + err.Error(), ""
	}

	ctx := context.Background()
	results := make([]syncDetailItem, 0, len(targetIDs))
	successCount := 0

	for _, tid := range targetIDs {
		dstClient, err := h.getClientByID(tid)
		if err != nil {
			results = append(results, syncDetailItem{ClusterID: tid, Status: "failed", Message: err.Error()})
			continue
		}

		if err := ensureNamespace(ctx, dstClient, policy.SourceNamespace); err != nil {
			results = append(results, syncDetailItem{ClusterID: tid, Status: "failed", Message: "建立命名空間失敗: " + err.Error()})
			continue
		}

		var syncErr error
		switch policy.ResourceType {
		case "ConfigMap":
			syncErr = h.syncSpecificConfigMaps(ctx, srcClient, dstClient, policy.SourceNamespace, resourceNames, policy.ConflictPolicy)
		case "Secret":
			syncErr = h.syncSpecificSecrets(ctx, srcClient, dstClient, policy.SourceNamespace, resourceNames, policy.ConflictPolicy)
		default:
			syncErr = fmt.Errorf("不支援的資源型別: %s", policy.ResourceType)
		}

		if syncErr != nil {
			results = append(results, syncDetailItem{ClusterID: tid, Status: "failed", Message: syncErr.Error()})
		} else {
			results = append(results, syncDetailItem{ClusterID: tid, Status: "success", Message: "同步成功"})
			successCount++
		}
	}

	detailsBytes, _ := json.Marshal(results)
	details = string(detailsBytes)

	if successCount == len(targetIDs) {
		return "success", fmt.Sprintf("成功同步至 %d 個叢集", successCount), details
	} else if successCount > 0 {
		return "partial", fmt.Sprintf("部分成功（%d/%d）", successCount, len(targetIDs)), details
	}
	return "failed", "所有目標叢集同步失敗", details
}

func (h *MultiClusterHandler) syncSpecificConfigMaps(
	ctx context.Context, src, dst kubernetes.Interface, ns string, names []string, conflictPolicy string,
) error {
	for _, name := range names {
		cm, err := src.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("取得 ConfigMap %s 失敗: %v", name, err)
		}
		clean := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cm.Name, Namespace: ns, Labels: cm.Labels},
			Data:       cm.Data,
			BinaryData: cm.BinaryData,
		}
		_, getErr := dst.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(getErr) {
			_, err = dst.CoreV1().ConfigMaps(ns).Create(ctx, clean, metav1.CreateOptions{})
		} else if getErr == nil {
			if conflictPolicy == "skip" {
				continue
			}
			_, err = dst.CoreV1().ConfigMaps(ns).Update(ctx, clean, metav1.UpdateOptions{})
		} else {
			err = getErr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *MultiClusterHandler) syncSpecificSecrets(
	ctx context.Context, src, dst kubernetes.Interface, ns string, names []string, conflictPolicy string,
) error {
	for _, name := range names {
		sec, err := src.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("取得 Secret %s 失敗: %v", name, err)
		}
		clean := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: sec.Name, Namespace: ns, Labels: sec.Labels},
			Type:       sec.Type,
			Data:       sec.Data,
		}
		_, getErr := dst.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(getErr) {
			_, err = dst.CoreV1().Secrets(ns).Create(ctx, clean, metav1.CreateOptions{})
		} else if getErr == nil {
			if conflictPolicy == "skip" {
				continue
			}
			_, err = dst.CoreV1().Secrets(ns).Update(ctx, clean, metav1.UpdateOptions{})
		} else {
			err = getErr
		}
		if err != nil {
			return err
		}
	}
	return nil
}
