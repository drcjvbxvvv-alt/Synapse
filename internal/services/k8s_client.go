package services

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	rolloutsclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type K8sClient struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// tlsPolicy 控制無 CA 憑證時的行為：strict | warn | skip
// 由 InitTLSPolicy() 在 main.go 啟動時設定，預設 warn。
var tlsPolicy = "warn"

// InitTLSPolicy 設定全域 K8s TLS 策略，應在 main.go 呼叫一次。
func InitTLSPolicy(policy string) {
	switch policy {
	case "strict", "warn", "skip":
		tlsPolicy = policy
	default:
		tlsPolicy = "warn"
	}
}

// zeroString 清零字串底層記憶體（best-effort，縮短敏感資料在 heap 的暴露時間）
func zeroString(s *string) {
	b := []byte(*s)
	for i := range b {
		b[i] = 0
	}
	*s = ""
}

type ClusterInfo struct {
	Version           string `json:"version"`
	NodeCount         int    `json:"nodeCount"`
	ReadyNodes        int    `json:"readyNodes"`
	Status            string `json:"status"`
	PodCount          int    `json:"podCount,omitempty"`
	RunningPods       int    `json:"runningPods,omitempty"`
	CanAccessPods     bool   `json:"canAccessPods,omitempty"`
	CanAccessServices bool   `json:"canAccessServices,omitempty"`
}

// connPoolTransport 為 K8s REST client 注入連線池設定
func connPoolTransport(rt http.RoundTripper) http.RoundTripper {
	if t, ok := rt.(*http.Transport); ok {
		t.MaxIdleConnsPerHost = 100
	}
	return rt
}

// NewK8sClientFromKubeconfig 從kubeconfig建立客戶端
func NewK8sClientFromKubeconfig(kubeconfig string) (*K8sClient, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "client-key-data or client-key must be specified") {
			return nil, fmt.Errorf("解析kubeconfig失敗: 缺少客戶端私鑰資料（client-key-data）。請使用以下指令匯出完整的 kubeconfig：\nkubectl config view --raw --minify --flatten --context=<context-name>")
		}
		if strings.Contains(errStr, "REDACTED") {
			return nil, fmt.Errorf("解析kubeconfig失敗: 憑證資料被遮蔽（REDACTED）。請改用 `kubectl config view --raw` 匯出含原始憑證的 kubeconfig")
		}
		return nil, fmt.Errorf("解析kubeconfig失敗: %v", err)
	}

	// 設定超時、QPS/Burst 與連線池
	config.Timeout = 30 * time.Second
	config.QPS = 100
	config.Burst = 200
	config.WrapTransport = connPoolTransport

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("建立kubernetes客戶端失敗: %v", err)
	}

	return &K8sClient{
		clientset: clientset,
		config:    config,
	}, nil
}

// NewK8sClientFromToken 從API Server和Token建立客戶端。
// TLS 行為由全域 tlsPolicy 控制（strict / warn / skip）。
func NewK8sClientFromToken(apiServer, token, caCert string) (*K8sClient, error) {
	// 確保API Server地址格式正確
	if !strings.HasPrefix(apiServer, "http://") && !strings.HasPrefix(apiServer, "https://") {
		apiServer = "https://" + apiServer
	}

	config := &rest.Config{
		Host:          apiServer,
		BearerToken:   token,
		Timeout:       30 * time.Second,
		QPS:           100,
		Burst:         200,
		WrapTransport: connPoolTransport,
	}

	if caCert != "" {
		// 提供了 CA 憑證，啟用 TLS 驗證
		caCertData, err := base64.StdEncoding.DecodeString(caCert)
		if err != nil {
			caCertData = []byte(caCert) // fallback：嘗試原始 PEM 格式
		}
		config.TLSClientConfig = rest.TLSClientConfig{CAData: caCertData}
	} else {
		// 未提供 CA 憑證，依策略決定行為
		switch tlsPolicy {
		case "strict":
			return nil, fmt.Errorf("TLS 驗證失敗：未提供 CA 憑證，且 K8S_TLS_POLICY=strict。" +
				"請在匯入叢集時填入 CA 憑證，或設定 K8S_TLS_POLICY=warn 以允許繼續")
		case "skip":
			config.TLSClientConfig = rest.TLSClientConfig{Insecure: true}
		default: // warn
			logger.Warn("⚠️  K8s TLS 驗證已停用（未提供 CA 憑證），存在 MITM 風險",
				"apiServer", apiServer,
				"hint", "匯入叢集時填入 CA 憑證，或設定 K8S_TLS_POLICY=strict 強制驗證",
			)
			config.TLSClientConfig = rest.TLSClientConfig{Insecure: true}
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("建立kubernetes客戶端失敗: %v", err)
	}

	return &K8sClient{
		clientset: clientset,
		config:    config,
	}, nil
}

// NewK8sClientForCluster 根據叢集模型建立 K8s 客戶端（統一入口）。
// 敏感字串在使用後立即清零，縮短明文在 heap 的暴露時間。
func NewK8sClientForCluster(cluster *models.Cluster) (*K8sClient, error) {
	if cluster.KubeconfigEnc != "" {
		kubeconfig := cluster.KubeconfigEnc
		defer zeroString(&kubeconfig)
		return NewK8sClientFromKubeconfig(kubeconfig)
	}
	token := cluster.SATokenEnc
	caCert := cluster.CAEnc
	defer zeroString(&token)
	defer zeroString(&caCert)
	return NewK8sClientFromToken(cluster.APIServer, token, caCert)
}

// TestConnection 測試連線並獲取叢集資訊
func (c *K8sClient) TestConnection() (*ClusterInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 測試基本連線 - 獲取叢集版本資訊
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("連線失敗，無法獲取叢集版本: %w", err)
	}

	// 2. 測試權限 - 嘗試獲取節點列表
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("權限不足，無法獲取節點列表: %w", err)
	}

	// 3. 統計節點狀態
	readyNodes := 0
	notReadyNodes := 0
	for _, node := range nodes.Items {
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					readyNodes++
					isReady = true
				}
				break
			}
		}
		if !isReady {
			notReadyNodes++
		}
	}

	// 4. 測試Pod訪問權限
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{Limit: 1})
	canAccessPods := err == nil

	// 5. 測試Service訪問權限
	_, err = c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{Limit: 1})
	canAccessServices := err == nil

	// 6. 確定叢集整體狀態
	status := "healthy"
	if notReadyNodes > 0 {
		if readyNodes == 0 {
			status = "unhealthy"
		} else {
			status = "warning"
		}
	}

	// 7. 獲取叢集基本資訊
	clusterInfo := &ClusterInfo{
		Version:           version.String(),
		NodeCount:         len(nodes.Items),
		ReadyNodes:        readyNodes,
		Status:            status,
		CanAccessPods:     canAccessPods,
		CanAccessServices: canAccessServices,
	}

	// 8. 嘗試獲取更多統計資訊（可選，不影響連線測試結果）
	if canAccessPods && pods != nil {
		// 統計Pod數量（僅在有權限時）
		allPods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err == nil {
			clusterInfo.PodCount = len(allPods.Items)
			runningPods := 0
			for _, pod := range allPods.Items {
				if pod.Status.Phase == corev1.PodRunning {
					runningPods++
				}
			}
			clusterInfo.RunningPods = runningPods
		}
	}

	return clusterInfo, nil
}

// analyzeConnectionError 分析連線錯誤並提供診斷資訊
//
//nolint:unused // 保留用於未來使用
func analyzeConnectionError(err error) string {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "unexpected EOF"):
		return "網路連線意外中斷，可能原因：1) API Server地址錯誤或不可達 2) 網路連線不穩定 3) TLS握手失敗 4) 防火牆阻止連線"
	case strings.Contains(errStr, "connection refused"):
		return "連線被拒絕，API Server可能未執行或連接埠不正確"
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "context deadline exceeded"):
		return "連線超時，可能原因：1) API Server響應過慢 2) 網路延遲過高 3) 防火牆限制 4) 叢集負載過高，建議檢查網路連線和叢集狀態"
	case strings.Contains(errStr, "certificate"):
		return "TLS證書驗證失敗，請檢查CA證書配置或嘗試跳過證書驗證"
	case strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401"):
		return "認證失敗，請檢查Token或kubeconfig中的認證資訊"
	case strings.Contains(errStr, "forbidden") || strings.Contains(errStr, "403"):
		return "權限不足，當前使用者沒有訪問該資源的權限"
	case strings.Contains(errStr, "not found") || strings.Contains(errStr, "404"):
		return "API路徑不存在，請檢查API Server地址和版本"
	case strings.Contains(errStr, "no such host"):
		return "域名解析失敗，請檢查API Server地址是否正確"
	case strings.Contains(errStr, "network is unreachable"):
		return "網路不可達，請檢查網路連線和路由配置"
	default:
		return "未知連線錯誤，請檢查網路連線和叢集配置"
	}
}

// GetClusterOverview 獲取叢集概覽資訊
func (c *K8sClient) GetClusterOverview() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	overview := make(map[string]interface{})

	// 獲取節點資訊
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取節點資訊失敗: %v", err)
	}

	// 獲取Pod資訊
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取Pod資訊失敗: %v", err)
	}

	// 獲取命名空間資訊
	namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取命名空間資訊失敗: %v", err)
	}

	// 統計Pod狀態
	runningPods := 0
	pendingPods := 0
	failedPods := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			runningPods++
		case corev1.PodPending:
			pendingPods++
		case corev1.PodFailed:
			failedPods++
		}
	}

	overview["nodes"] = map[string]interface{}{
		"total": len(nodes.Items),
		"ready": func() int {
			ready := 0
			for _, node := range nodes.Items {
				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
						ready++
						break
					}
				}
			}
			return ready
		}(),
	}

	overview["pods"] = map[string]interface{}{
		"total":   len(pods.Items),
		"running": runningPods,
		"pending": pendingPods,
		"failed":  failedPods,
	}

	overview["namespaces"] = len(namespaces.Items)

	return overview, nil
}

// CreateKubeconfigFromToken 從token和API server建立kubeconfig內容
func CreateKubeconfigFromToken(clusterName, apiServer, token, caCert string) string {
	config := api.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: map[string]*api.Cluster{
			clusterName: {
				Server: apiServer,
			},
		},
		Contexts: map[string]*api.Context{
			clusterName: {
				Cluster:  clusterName,
				AuthInfo: clusterName,
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			clusterName: {
				Token: token,
			},
		},
		CurrentContext: clusterName,
	}

	// 如果提供了CA證書，新增到配置中
	if caCert != "" {
		config.Clusters[clusterName].CertificateAuthorityData = []byte(caCert)
	} else {
		config.Clusters[clusterName].InsecureSkipTLSVerify = true
	}

	// 將配置轉換為YAML字串
	configBytes, _ := clientcmd.Write(config)
	return string(configBytes)
}

// ValidateKubeconfig 驗證kubeconfig格式
func ValidateKubeconfig(kubeconfig string) error {
	_, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	return err
}

// GetClientset 獲取kubernetes客戶端
func (c *K8sClient) GetClientset() *kubernetes.Clientset {
	return c.clientset
}

// GetRestConfig 返回底層 REST 配置（供動態客戶端/Informer 使用）
func (c *K8sClient) GetRestConfig() *rest.Config {
	return c.config
}

// RBACSummary 彙整匯入 Token 的 RBAC 危險程度
type RBACSummary struct {
	IsClusterAdmin bool     `json:"isClusterAdmin"`
	WarningLevel   string   `json:"warningLevel"` // "critical" | "high" | "normal"
	Warnings       []string `json:"warnings"`     // 具體的高危權限描述
}

// CheckRBACSummary 用 SelfSubjectAccessReview 評估當前憑證的 RBAC 危險程度。
// 只做讀取檢查，不修改叢集任何資源。
func (c *K8sClient) CheckRBACSummary(ctx context.Context) *RBACSummary {
	summary := &RBACSummary{WarningLevel: "normal"}

	checks := []struct {
		verb, resource, group, description string
		isCritical                          bool
	}{
		{"*", "*", "*", "cluster-admin（所有資源完整存取）", true},
		{"delete", "nodes", "", "刪除 Node", false},
		{"delete", "namespaces", "", "刪除 Namespace", false},
		{"create", "clusterrolebindings", "rbac.authorization.k8s.io", "建立 ClusterRoleBinding", false},
		{"list", "secrets", "", "列出全叢集 Secret", false},
		{"create", "pods/exec", "", "對任意 Pod 執行命令", false},
	}

	for _, chk := range checks {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Verb:     chk.verb,
					Resource: chk.resource,
					Group:    chk.group,
				},
			},
		}
		result, err := c.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(
			ctx, sar, metav1.CreateOptions{})
		if err != nil || !result.Status.Allowed {
			continue
		}
		summary.Warnings = append(summary.Warnings, chk.description)
		if chk.isCritical {
			summary.IsClusterAdmin = true
			summary.WarningLevel = "critical"
		} else if summary.WarningLevel != "critical" {
			summary.WarningLevel = "high"
		}
	}
	return summary
}

// GetAPIServerCertExpiry 透過 TLS dial 取得 API Server 憑證到期時間。
// 此方法繞過憑證驗證（用於取得到期日），不代表連線受信任。
func (c *K8sClient) GetAPIServerCertExpiry() (*time.Time, error) {
	u, err := url.Parse(c.config.Host)
	if err != nil {
		return nil, fmt.Errorf("解析 API Server 地址失敗: %w", err)
	}
	addr := u.Host
	if !strings.Contains(addr, ":") {
		addr += ":443"
	}

	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true}) // #nosec G402 -- read cert expiry only; connection is not trusted
	if err != nil {
		return nil, fmt.Errorf("TLS 連線 API Server 失敗: %w", err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("API Server 未回傳 TLS 憑證")
	}
	expiry := certs[0].NotAfter
	return &expiry, nil
}

// GetRolloutClient 獲取Argo Rollouts客戶端
func (c *K8sClient) GetRolloutClient() (*rolloutsclientset.Clientset, error) {
	rolloutClient, err := rolloutsclientset.NewForConfig(c.config)
	if err != nil {
		return nil, fmt.Errorf("建立Argo Rollouts客戶端失敗: %w", err)
	}
	return rolloutClient, nil
}

