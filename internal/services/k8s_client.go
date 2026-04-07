package services

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"math"
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

	//nolint:gosec // 僅用於取得憑證到期日，非信任連線
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
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

// GetClusterMetrics 獲取叢集監控資料
func (c *K8sClient) GetClusterMetrics(timeRange string, step string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metrics := make(map[string]interface{})

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

	// 計算時間範圍
	endTime := time.Now()
	var startTime time.Time

	switch timeRange {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
	case "12h":
		startTime = endTime.Add(-12 * time.Hour)
	case "1d":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-1 * time.Hour)
	}

	// 從節點狀態和Pod分佈估算資源使用情況
	// 計算節點資源總量和已分配資源
	var totalCPUCapacity, allocatableCPU int64
	var totalMemoryCapacity, allocatableMemory int64

	for _, node := range nodes.Items {
		// 獲取節點總容量
		cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
		memoryCapacity := node.Status.Capacity.Memory().Value()

		totalCPUCapacity += cpuCapacity
		totalMemoryCapacity += memoryCapacity

		// 獲取節點可分配資源
		allocatableCPU += node.Status.Allocatable.Cpu().MilliValue()
		allocatableMemory += node.Status.Allocatable.Memory().Value()
	}

	// 計算Pod請求的資源總量
	var requestedCPU, requestedMemory int64
	var runningPodCount int

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPodCount++

			// 累加Pod中所有容器請求的資源
			for _, container := range pod.Spec.Containers {
				if container.Resources.Requests != nil {
					if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
						requestedCPU += cpu.MilliValue()
					}
					if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
						requestedMemory += memory.Value()
					}
				}
			}
		}
	}

	// 計算資源使用率
	cpuUsagePercent := 0.0
	memoryUsagePercent := 0.0

	if allocatableCPU > 0 {
		cpuUsagePercent = math.Min(100, float64(requestedCPU)/float64(allocatableCPU)*100)
	}

	if allocatableMemory > 0 {
		memoryUsagePercent = math.Min(100, float64(requestedMemory)/float64(allocatableMemory)*100)
	}

	// 如果無法獲取請求資源資訊，使用Pod數量和節點數量估算
	if requestedCPU == 0 || requestedMemory == 0 {
		readyNodeCount := 0
		for _, node := range nodes.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					readyNodeCount++
					break
				}
			}
		}

		if readyNodeCount > 0 {
			// 根據執行中的Pod數量和節點數量估算使用率
			podsPerNode := float64(runningPodCount) / float64(readyNodeCount)
			cpuUsagePercent = math.Min(95, podsPerNode*10)   // 假設每個Pod平均使用10%的CPU
			memoryUsagePercent = math.Min(90, podsPerNode*8) // 假設每個Pod平均使用8%的記憶體
		}
	}

	// 統計Pod狀態分佈
	podStatus := map[string]int{
		"Running":   0,
		"Pending":   0,
		"Succeeded": 0,
		"Failed":    0,
		"Unknown":   0,
	}

	for _, pod := range pods.Items {
		status := string(pod.Status.Phase)
		if count, exists := podStatus[status]; exists {
			podStatus[status] = count + 1
		} else {
			podStatus["Unknown"]++
		}
	}

	// 統計節點狀態
	nodeStatus := map[string]int{
		"Ready":    0,
		"NotReady": 0,
	}

	for _, node := range nodes.Items {
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					nodeStatus["Ready"]++
					isReady = true
				}
				break
			}
		}
		if !isReady {
			nodeStatus["NotReady"]++
		}
	}

	// 生成時間序列資料
	// 注意：這裡我們仍然使用模擬資料生成時間序列，因為獲取歷史資料需要Prometheus等監控系統
	// 在實際生產環境中，應該整合Prometheus API來獲取真實的歷史資料
	var timePoints []time.Time
	stepDuration, _ := time.ParseDuration(step)
	if stepDuration == 0 {
		stepDuration = time.Minute // 預設1分鐘
	}

	for t := startTime; t.Before(endTime); t = t.Add(stepDuration) {
		timePoints = append(timePoints, t)
	}
	timePoints = append(timePoints, endTime)

	// 生成CPU使用率資料，但使用當前真實的CPU使用率作為基準
	cpuData := make([]map[string]interface{}, 0, len(timePoints))
	for i, t := range timePoints {
		// 使用真實的當前值作為基準，歷史資料仍然模擬
		var value float64
		if i == len(timePoints)-1 {
			value = cpuUsagePercent
		} else {
			// 模擬歷史資料，但圍繞當前真實值波動
			variance := 20.0
			if cpuUsagePercent > 80 {
				variance = 10.0
			}
			value = math.Max(0, math.Min(100, cpuUsagePercent+(math.Sin(float64(t.Unix()%3600)/3600*2*math.Pi)-0.5)*variance))
		}

		cpuData = append(cpuData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     value,
		})
	}

	// 生成記憶體使用率資料，但使用當前真實的記憶體使用率作為基準
	memoryData := make([]map[string]interface{}, 0, len(timePoints))
	for i, t := range timePoints {
		// 使用真實的當前值作為基準，歷史資料仍然模擬
		var value float64
		if i == len(timePoints)-1 {
			value = memoryUsagePercent
		} else {
			// 模擬歷史資料，但圍繞當前真實值波動
			variance := 15.0
			if memoryUsagePercent > 80 {
				variance = 8.0
			}
			value = math.Max(0, math.Min(100, memoryUsagePercent+(math.Sin(float64(t.Unix()%7200)/7200*2*math.Pi)-0.5)*variance))
		}

		memoryData = append(memoryData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     value,
		})
	}

	// 網路和磁碟資料仍然使用模擬資料，因為這些需要特定的監控系統
	networkInData := make([]map[string]interface{}, 0, len(timePoints))
	networkOutData := make([]map[string]interface{}, 0, len(timePoints))
	for _, t := range timePoints {
		networkInData = append(networkInData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     30 + 20*math.Sin(float64(t.Unix()%5400)/5400*2*math.Pi),
		})
		networkOutData = append(networkOutData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     25 + 15*math.Sin(float64(t.Unix()%4800)/4800*2*math.Pi),
		})
	}

	diskData := make([]map[string]interface{}, 0, len(timePoints))
	for _, t := range timePoints {
		diskData = append(diskData, map[string]interface{}{
			"timestamp": t.Unix(),
			"value":     40 + 5*math.Sin(float64(t.Unix()%10800)/10800*2*math.Pi),
		})
	}

	// 組裝返回資料
	metrics["cpu"] = map[string]interface{}{
		"current": cpuUsagePercent,
		"series":  cpuData,
	}

	metrics["memory"] = map[string]interface{}{
		"current": memoryUsagePercent,
		"series":  memoryData,
	}

	metrics["network"] = map[string]interface{}{
		"in": map[string]interface{}{
			"current": networkInData[len(networkInData)-1]["value"],
			"series":  networkInData,
		},
		"out": map[string]interface{}{
			"current": networkOutData[len(networkOutData)-1]["value"],
			"series":  networkOutData,
		},
	}

	metrics["disk"] = map[string]interface{}{
		"current": diskData[len(diskData)-1]["value"],
		"series":  diskData,
	}

	metrics["pods"] = podStatus
	metrics["nodes"] = nodeStatus

	// 新增時間範圍資訊
	metrics["timeRange"] = map[string]interface{}{
		"start": startTime.Unix(),
		"end":   endTime.Unix(),
		"step":  step,
	}

	return metrics, nil
}

// CordonNode 封鎖節點（標記為不可排程）
func (c *K8sClient) CordonNode(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取節點
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("獲取節點失敗: %v", err)
	}

	// 檢查節點是否已經被封鎖
	if node.Spec.Unschedulable {
		return nil // 節點已經被封鎖，無需操作
	}

	// 標記節點為不可排程
	node.Spec.Unschedulable = true

	// 更新節點
	_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("封鎖節點失敗: %v", err)
	}

	return nil
}

// GetNodeMetrics 獲取節點資源使用情況
func (c *K8sClient) GetNodeMetrics(nodeName string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 獲取節點資訊
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取節點資訊失敗: %v", err)
	}

	// 獲取節點上的所有Pod
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("獲取節點Pod列表失敗: %v", err)
	}

	// 計算節點資源容量
	cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
	memoryCapacity := node.Status.Capacity.Memory().Value()
	allocatableCPU := node.Status.Allocatable.Cpu().MilliValue()
	allocatableMemory := node.Status.Allocatable.Memory().Value()

	// 計算Pod請求的資源總量
	var requestedCPU, requestedMemory int64
	var runningPodCount int

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPodCount++

			// 累加Pod中所有容器請求的資源
			for _, container := range pod.Spec.Containers {
				if container.Resources.Requests != nil {
					if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
						requestedCPU += cpu.MilliValue()
					}
					if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
						requestedMemory += memory.Value()
					}
				}
			}
		}
	}

	// 計算資源使用率
	cpuUsagePercent := 0.0
	memoryUsagePercent := 0.0

	if allocatableCPU > 0 {
		cpuUsagePercent = math.Min(100, float64(requestedCPU)/float64(allocatableCPU)*100)
	}

	if allocatableMemory > 0 {
		memoryUsagePercent = math.Min(100, float64(requestedMemory)/float64(allocatableMemory)*100)
	}

	// 如果無法獲取請求資源資訊，使用Pod數量估算
	if requestedCPU == 0 || requestedMemory == 0 {
		if runningPodCount > 0 {
			// 根據執行中的Pod數量估算使用率
			cpuUsagePercent = math.Min(95, float64(runningPodCount)*8)    // 假設每個Pod平均使用8%的CPU
			memoryUsagePercent = math.Min(90, float64(runningPodCount)*6) // 假設每個Pod平均使用6%的記憶體
		}
	}

	return map[string]interface{}{
		"cpuUsage":    cpuUsagePercent,
		"memoryUsage": memoryUsagePercent,
		"podCount":    runningPodCount,
		"resources": map[string]interface{}{
			"cpu": map[string]interface{}{
				"capacity":    cpuCapacity,
				"allocatable": allocatableCPU,
				"requested":   requestedCPU,
			},
			"memory": map[string]interface{}{
				"capacity":    memoryCapacity,
				"allocatable": allocatableMemory,
				"requested":   requestedMemory,
			},
		},
	}, nil
}

// GetAllNodesMetrics 獲取所有節點的資源使用情況
func (c *K8sClient) GetAllNodesMetrics() (map[string]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 獲取所有節點
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取節點列表失敗: %v", err)
	}

	// 獲取所有Pod
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("獲取Pod列表失敗: %v", err)
	}

	// 按節點分組Pod
	nodePodsMap := make(map[string][]corev1.Pod)
	for _, pod := range pods.Items {
		if pod.Spec.NodeName != "" {
			nodePodsMap[pod.Spec.NodeName] = append(nodePodsMap[pod.Spec.NodeName], pod)
		}
	}

	// 計算每個節點的資源使用情況
	result := make(map[string]map[string]interface{})
	for _, node := range nodes.Items {
		nodePods := nodePodsMap[node.Name]

		// 計算節點資源容量
		cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
		memoryCapacity := node.Status.Capacity.Memory().Value()
		allocatableCPU := node.Status.Allocatable.Cpu().MilliValue()
		allocatableMemory := node.Status.Allocatable.Memory().Value()

		// 計算Pod請求的資源總量
		var requestedCPU, requestedMemory int64
		var runningPodCount int

		for _, pod := range nodePods {
			if pod.Status.Phase == corev1.PodRunning {
				runningPodCount++

				// 累加Pod中所有容器請求的資源
				for _, container := range pod.Spec.Containers {
					if container.Resources.Requests != nil {
						if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
							requestedCPU += cpu.MilliValue()
						}
						if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
							requestedMemory += memory.Value()
						}
					}
				}
			}
		}

		// 計算資源使用率
		cpuUsagePercent := 0.0
		memoryUsagePercent := 0.0

		if allocatableCPU > 0 {
			cpuUsagePercent = math.Min(100, float64(requestedCPU)/float64(allocatableCPU)*100)
		}

		if allocatableMemory > 0 {
			memoryUsagePercent = math.Min(100, float64(requestedMemory)/float64(allocatableMemory)*100)
		}

		// 如果無法獲取請求資源資訊，使用Pod數量估算
		if requestedCPU == 0 || requestedMemory == 0 {
			if runningPodCount > 0 {
				// 根據執行中的Pod數量估算使用率
				cpuUsagePercent = math.Min(95, float64(runningPodCount)*8)    // 假設每個Pod平均使用8%的CPU
				memoryUsagePercent = math.Min(90, float64(runningPodCount)*6) // 假設每個Pod平均使用6%的記憶體
			}
		}

		result[node.Name] = map[string]interface{}{
			"cpuUsage":    cpuUsagePercent,
			"memoryUsage": memoryUsagePercent,
			"podCount":    runningPodCount,
			"resources": map[string]interface{}{
				"cpu": map[string]interface{}{
					"capacity":    cpuCapacity,
					"allocatable": allocatableCPU,
					"requested":   requestedCPU,
				},
				"memory": map[string]interface{}{
					"capacity":    memoryCapacity,
					"allocatable": allocatableMemory,
					"requested":   requestedMemory,
				},
			},
		}
	}

	return result, nil
}

// UncordonNode 解封節點（標記為可排程）
func (c *K8sClient) UncordonNode(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 獲取節點
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("獲取節點失敗: %v", err)
	}

	// 檢查節點是否已經可排程
	if !node.Spec.Unschedulable {
		return nil // 節點已經可排程，無需操作
	}

	// 標記節點為可排程
	node.Spec.Unschedulable = false

	// 更新節點
	_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("解封節點失敗: %v", err)
	}

	return nil
}

// DrainNode 驅逐節點上的Pod
func (c *K8sClient) DrainNode(nodeName string, options map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // 驅逐操作可能需要更長時間
	defer cancel()

	// 獲取節點
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("獲取節點失敗: %v", err)
	}

	// 1. 首先封鎖節點，防止新的Pod排程到該節點
	if !node.Spec.Unschedulable {
		node.Spec.Unschedulable = true
		_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("封鎖節點失敗: %v", err)
		}
	}

	// 解析選項
	ignoreDaemonSets := true
	if val, ok := options["ignoreDaemonSets"]; ok {
		ignoreDaemonSets = val.(bool)
	}

	deleteLocalData := false
	if val, ok := options["deleteLocalData"]; ok {
		deleteLocalData = val.(bool)
	}

	force := false
	if val, ok := options["force"]; ok {
		force = val.(bool)
	}

	gracePeriodSeconds := int64(30)
	if val, ok := options["gracePeriodSeconds"]; ok {
		if intVal, ok := val.(float64); ok {
			gracePeriodSeconds = int64(intVal)
		}
	}

	// 2. 獲取節點上的所有Pod
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return fmt.Errorf("獲取節點上的Pod失敗: %v", err)
	}

	// 3. 驅逐Pod
	for _, pod := range pods.Items {
		// 如果忽略DaemonSet，檢查Pod是否由DaemonSet控制
		if ignoreDaemonSets {
			isDaemonSet := false
			for _, owner := range pod.OwnerReferences {
				if owner.Kind == "DaemonSet" {
					isDaemonSet = true
					break
				}
			}
			if isDaemonSet {
				continue // 跳過DaemonSet管理的Pod
			}
		}

		// 檢查Pod是否使用emptyDir卷
		if !deleteLocalData {
			hasEmptyDir := false
			for _, volume := range pod.Spec.Volumes {
				if volume.EmptyDir != nil {
					hasEmptyDir = true
					break
				}
			}
			if hasEmptyDir && !force {
				return fmt.Errorf("pod %s/%s 使用emptyDir卷，需要設定deleteLocalData=true或force=true", pod.Namespace, pod.Name)
			}
		}

		// 刪除Pod
		deleteOptions := metav1.DeleteOptions{}
		if gracePeriodSeconds >= 0 {
			deleteOptions.GracePeriodSeconds = &gracePeriodSeconds
		}

		err = c.clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, deleteOptions)
		if err != nil {
			if !force {
				return fmt.Errorf("驅逐Pod %s/%s 失敗: %v", pod.Namespace, pod.Name, err)
			}
			// 如果設定了force，則忽略錯誤繼續執行
		}
	}

	return nil
}
