package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	kubectlPodNamespace    = "synapse-system"
	kubectlPodImage        = "ahernshaiaa/kubectl:v0.1"
	kubectlPodPrefix       = "synapse-kubectl-"
	kubectlIdleTimeout     = 1 * time.Hour
	kubectlCleanupInterval = 10 * time.Minute
)

// KubectlPodTerminalHandler kubectl Pod 終端處理器
type KubectlPodTerminalHandler struct {
	clusterService *services.ClusterService
	auditService   *services.AuditService
	k8sMgr         *k8s.ClusterInformerManager
	podTerminal    *PodTerminalHandler
	activeSessions map[string]int // podName -> activeConnections
	sessionsMutex  sync.RWMutex
	upgrader       websocket.Upgrader
}

// NewKubectlPodTerminalHandler 建立 kubectl Pod 終端處理器
func NewKubectlPodTerminalHandler(clusterService *services.ClusterService, auditService *services.AuditService, k8sMgr *k8s.ClusterInformerManager) *KubectlPodTerminalHandler {
	h := &KubectlPodTerminalHandler{
		clusterService: clusterService,
		auditService:   auditService,
		k8sMgr:         k8sMgr,
		podTerminal:    NewPodTerminalHandler(clusterService, auditService, k8sMgr),
		activeSessions: make(map[string]int),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return middleware.IsOriginAllowed(origin)
			},
		},
	}

	// 啟動後臺清理任務
	go h.startCleanupWorker()

	return h
}

// HandleKubectlPodTerminal 處理 kubectl Pod 終端請求
func (h *KubectlPodTerminalHandler) HandleKubectlPodTerminal(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	userID := c.GetUint("user_id")

	// 獲取使用者的叢集權限，確定使用哪個 ServiceAccount
	permissionType := "readonly" // 預設只讀權限
	var namespaces []string
	var customRoleRef string

	if perm, exists := c.Get("cluster_permission"); exists {
		if cp, ok := perm.(*models.ClusterPermission); ok && cp != nil {
			permissionType = cp.PermissionType
			namespaces = cp.GetNamespaceList()
			customRoleRef = cp.CustomRoleRef
		}
	}

	// 使用 RBACService 獲取有效的 ServiceAccount
	rbacSvc := services.NewRBACService()
	rbacConfig := &services.UserRBACConfig{
		UserID:         userID,
		PermissionType: permissionType,
		Namespaces:     namespaces,
		ClusterRoleRef: customRoleRef,
	}
	serviceAccount := rbacSvc.GetEffectiveServiceAccount(rbacConfig)

	logger.Info("使用者kubectl終端權限", "userID", userID, "permissionType", permissionType, "namespaces", namespaces, "serviceAccount", serviceAccount)

	// 獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(uint(clusterID))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	client := k8sClient.GetClientset()

	podName := fmt.Sprintf("%s%d-%s", kubectlPodPrefix, userID, permissionType)
	sessionKey := fmt.Sprintf("%s-%s", clusterIDStr, podName)

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("kubectl終端升級WebSocket失敗", "error", err)
		return
	}

	var sessionCountAdded bool
	defer func() {
		if sessionCountAdded {
			h.sessionsMutex.Lock()
			h.activeSessions[sessionKey]--
			if h.activeSessions[sessionKey] <= 0 {
				delete(h.activeSessions, sessionKey)
			}
			h.sessionsMutex.Unlock()
		}
		_ = conn.Close()
	}()

	h.sendKubectlPrep(conn, "正在準備 kubectl 終端 Pod，請稍候…")

	beforeCreate := func() {
		h.sendKubectlPrep(conn, "正在叢集中建立 kubectl 終端 Pod（首次連線可能需要拉取映像，耗時取決於網路）…")
	}
	if err := h.ensureKubectlPod(client, podName, userID, serviceAccount, permissionType, beforeCreate); err != nil {
		logger.Error("建立kubectl Pod失敗", "error", err, "podName", podName)
		h.sendTerminalJSON(conn, "error", fmt.Sprintf("建立 kubectl Pod 失敗: %v", err))
		return
	}

	if err := h.waitForPodRunningWithProgress(client, podName, conn); err != nil {
		logger.Error("等待Pod執行失敗", "error", err, "podName", podName)
		h.sendTerminalJSON(conn, "error", fmt.Sprintf("等待 Pod 就緒失敗: %v", err))
		return
	}

	h.updateLastActivity(client, podName)

	h.sessionsMutex.Lock()
	h.activeSessions[sessionKey]++
	h.sessionsMutex.Unlock()
	sessionCountAdded = true

	logger.Info("kubectl Pod終端連線", "cluster", cluster.Name, "pod", podName, "user", userID)

	h.podTerminal.RunPodTerminalWithConn(
		conn,
		cluster,
		clusterIDStr,
		kubectlPodNamespace,
		podName,
		"kubectl",
		userID,
		services.TerminalTypeKubectl,
	)
}

func (h *KubectlPodTerminalHandler) sendKubectlPrep(conn *websocket.Conn, text string) {
	_ = conn.WriteJSON(PodTerminalMessage{Type: "kubectl_prep", Data: text})
}

func (h *KubectlPodTerminalHandler) sendTerminalJSON(conn *websocket.Conn, msgType, data string) {
	_ = conn.WriteJSON(PodTerminalMessage{Type: msgType, Data: data})
}

func describeKubectlPodProgress(pod *corev1.Pod) string {
	var parts []string
	if pod.Status.Phase != "" {
		parts = append(parts, fmt.Sprintf("Pod 階段：%s", pod.Status.Phase))
	}
	for _, ics := range pod.Status.InitContainerStatuses {
		if ics.State.Waiting != nil {
			w := ics.State.Waiting
			s := fmt.Sprintf("初始化容器 %s：%s", ics.Name, w.Reason)
			if w.Message != "" {
				s += " — " + strings.TrimSpace(w.Message)
			}
			parts = append(parts, s)
		}
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			w := cs.State.Waiting
			s := fmt.Sprintf("容器 %s：%s", cs.Name, w.Reason)
			if w.Message != "" {
				s += " — " + strings.TrimSpace(w.Message)
			}
			parts = append(parts, s)
		} else if cs.State.Running != nil {
			parts = append(parts, fmt.Sprintf("容器 %s：已啟動", cs.Name))
		}
	}
	if pod.Status.Reason != "" {
		parts = append(parts, fmt.Sprintf("狀態說明：%s", pod.Status.Reason))
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Status == corev1.ConditionFalse && cond.Message != "" {
			parts = append(parts, fmt.Sprintf("%s：%s", cond.Type, cond.Message))
		}
	}
	if len(parts) == 0 {
		if pod.Status.Phase != "" {
			return fmt.Sprintf("Pod 階段：%s（詳情尚未上報）", pod.Status.Phase)
		}
		return "等待 Pod 狀態上報…"
	}
	return strings.Join(parts, " | ")
}

// ensureKubectlPod 確保 kubectl Pod 存在；即將在叢集中新建 Pod 時會呼叫 beforeCreate（用於向前端推送提示）
func (h *KubectlPodTerminalHandler) ensureKubectlPod(client *kubernetes.Clientset, podName string, userID uint, serviceAccount string, permissionType string, beforeCreate func()) error {
	ctx := context.Background()

	// 確保命名空間存在
	if err := ensureNamespace(ctx, client, kubectlPodNamespace); err != nil {
		return fmt.Errorf("建立命名空間 %s 失敗: %w", kubectlPodNamespace, err)
	}

	// 檢查 Pod 是否已存在
	existingPod, err := client.CoreV1().Pods(kubectlPodNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		// Pod 存在
		if existingPod.Status.Phase == corev1.PodRunning {
			logger.Info("複用已存在的kubectl Pod", "pod", podName, "sa", serviceAccount)
			return nil // 可以複用
		}
		if existingPod.Status.Phase == corev1.PodFailed || existingPod.Status.Phase == corev1.PodSucceeded {
			// 刪除舊 Pod，重新建立
			logger.Info("刪除已終止的kubectl Pod", "pod", podName, "phase", existingPod.Status.Phase)
			_ = client.CoreV1().Pods(kubectlPodNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
			time.Sleep(2 * time.Second)
		}
		// 如果是 Pending 狀態，繼續等待
		if existingPod.Status.Phase == corev1.PodPending {
			return nil
		}
	}

	if !errors.IsNotFound(err) && err != nil {
		return err
	}

	// 建立新 Pod，使用對應權限的 ServiceAccount
	logger.Info("建立新的kubectl Pod", "pod", podName, "user", userID, "sa", serviceAccount, "permissionType", permissionType)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: kubectlPodNamespace,
			Labels: map[string]string{
				"app":             "synapse-kubectl",
				"user-id":         fmt.Sprintf("%d", userID),
				"permission-type": permissionType,
			},
			Annotations: map[string]string{
				"synapse.io/last-activity":   time.Now().Format(time.RFC3339),
				"synapse.io/permission-type": permissionType,
				"synapse.io/service-account": serviceAccount,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: serviceAccount, // 使用對應權限的 ServiceAccount
			Containers: []corev1.Container{{
				Name:    "kubectl",
				Image:   kubectlPodImage,
				Command: []string{"sleep", "infinity"},
				Stdin:   true,
				TTY:     true,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	if beforeCreate != nil {
		beforeCreate()
	}
	_, err = client.CoreV1().Pods(kubectlPodNamespace).Create(ctx, pod, metav1.CreateOptions{})
	return err
}

// waitForPodRunningWithProgress 等待 Pod 進入 Running，並透過 WebSocket 推送與上次不同的進度摘要（含映像拉取、容器 Waiting 原因等）
func (h *KubectlPodTerminalHandler) waitForPodRunningWithProgress(client *kubernetes.Clientset, podName string, conn *websocket.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	lastSent := ""
	for {
		pod, err := client.CoreV1().Pods(kubectlPodNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if pod.Status.Phase == corev1.PodRunning {
			return nil
		}

		if pod.Status.Phase == corev1.PodFailed {
			return fmt.Errorf("pod啟動失敗: %s", pod.Status.Message)
		}

		desc := describeKubectlPodProgress(pod)
		if desc != "" && desc != lastSent {
			lastSent = desc
			h.sendKubectlPrep(conn, desc)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("等待Pod執行超時")
		case <-time.After(1 * time.Second):
		}
	}
}

// updateLastActivity 更新 Pod 最後活動時間
func (h *KubectlPodTerminalHandler) updateLastActivity(client *kubernetes.Clientset, podName string) {
	ctx := context.Background()
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"synapse.io/last-activity":"%s"}}}`,
		time.Now().Format(time.RFC3339)))

	_, err := client.CoreV1().Pods(kubectlPodNamespace).Patch(ctx, podName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		logger.Error("更新Pod活動時間失敗", "error", err, "pod", podName)
	}
}

// startCleanupWorker 啟動後臺清理任務
func (h *KubectlPodTerminalHandler) startCleanupWorker() {
	ticker := time.NewTicker(kubectlCleanupInterval)
	logger.Info("kubectl Pod清理任務已啟動", "interval", kubectlCleanupInterval)

	for range ticker.C {
		h.cleanupIdlePods()
	}
}

// cleanupIdlePods 清理空閒的 kubectl Pod
func (h *KubectlPodTerminalHandler) cleanupIdlePods() {
	// 獲取所有叢集
	clusters, err := h.clusterService.GetAllClusters()
	if err != nil {
		logger.Error("獲取叢集列表失敗", "error", err)
		return
	}

	for _, cluster := range clusters {
		h.cleanupClusterIdlePods(cluster)
	}
}

// cleanupClusterIdlePods 清理指定叢集的空閒 Pod
func (h *KubectlPodTerminalHandler) cleanupClusterIdlePods(cluster *models.Cluster) {
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		return
	}
	client := k8sClient.GetClientset()

	ctx := context.Background()
	pods, err := client.CoreV1().Pods(kubectlPodNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=synapse-kubectl",
	})
	if err != nil {
		return
	}

	for _, pod := range pods.Items {
		// 檢查是否有活躍會話
		sessionKey := fmt.Sprintf("%d-%s", cluster.ID, pod.Name)
		h.sessionsMutex.RLock()
		activeCount := h.activeSessions[sessionKey]
		h.sessionsMutex.RUnlock()

		if activeCount > 0 {
			continue // 有活躍連線，不清理
		}

		// 檢查空閒時間
		lastActivityStr := pod.Annotations["synapse.io/last-activity"]
		if lastActivityStr == "" {
			continue
		}

		lastActivity, err := time.Parse(time.RFC3339, lastActivityStr)
		if err != nil {
			continue
		}

		if time.Since(lastActivity) > kubectlIdleTimeout {
			logger.Info("清理空閒kubectl Pod", "cluster", cluster.Name, "pod", pod.Name, "idleTime", time.Since(lastActivity))
			_ = client.CoreV1().Pods(kubectlPodNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		}
	}
}
