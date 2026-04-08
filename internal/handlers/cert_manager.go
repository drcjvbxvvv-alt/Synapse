package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

var (
	certGVR = schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "certificates",
	}
	issuerGVR = schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "issuers",
	}
	clusterIssuerGVR = schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "clusterissuers",
	}
)

// CertManagerHandler cert-manager 資源管理處理器
type CertManagerHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewCertManagerHandler 建立 cert-manager 處理器
func NewCertManagerHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *CertManagerHandler {
	return &CertManagerHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// CertificateSummary 憑證摘要
type CertificateSummary struct {
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Ready      bool     `json:"ready"`
	SecretName string   `json:"secretName"`
	Issuer     string   `json:"issuer"`
	IssuerKind string   `json:"issuerKind"`
	DNSNames   []string `json:"dnsNames"`
	NotBefore  string   `json:"notBefore,omitempty"`
	NotAfter   string   `json:"notAfter,omitempty"`
	DaysLeft   int      `json:"daysLeft"`
	Status     string   `json:"status"` // Valid / Expiring / Expired / NotReady
}

// IssuerSummary Issuer / ClusterIssuer 摘要
type IssuerSummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind"`
	Ready     bool   `json:"ready"`
	Type      string `json:"type"` // ACME / SelfSigned / CA / Vault / Venafi
}

// CertManagerStatusResponse cert-manager 安裝狀態
type CertManagerStatusResponse struct {
	Installed bool `json:"installed"`
}

func (h *CertManagerHandler) dynamicClient(c *gin.Context, clusterIDStr string) (dynamic.Interface, error) {
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		return nil, err
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		return nil, err
	}
	k8sClient, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(k8sClient.GetRestConfig())
}

// CheckCertManagerStatus 檢查 cert-manager 是否安裝
func (h *CertManagerHandler) CheckCertManagerStatus(c *gin.Context) {
	dynClient, err := h.dynamicClient(c, c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無法連線到叢集: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	_, err = dynClient.Resource(certGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		logger.Info("cert-manager 未偵測到", "reason", err.Error())
	}
	response.OK(c, CertManagerStatusResponse{Installed: err == nil})
}

// ListCertificates 列出所有命名空間的 Certificate
func (h *CertManagerHandler) ListCertificates(c *gin.Context) {
	dynClient, err := h.dynamicClient(c, c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無法連線到叢集: "+err.Error())
		return
	}

	namespace := c.Query("namespace")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	var list interface{ GetItems() []interface{ GetName() string } }
	_ = list

	var raw interface{}
	if namespace != "" && namespace != "all" {
		raw, err = dynClient.Resource(certGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		raw, err = dynClient.Resource(certGVR).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		logger.Error("ListCertificates 失敗", "error", err)
		response.InternalError(c, "獲取 Certificate 列表失敗: "+err.Error())
		return
	}

	items, ok := extractUnstructuredItems(raw)
	if !ok {
		response.InternalError(c, "解析 Certificate 列表失敗")
		return
	}

	certs := make([]CertificateSummary, 0, len(items))
	now := time.Now()

	for _, item := range items {
		obj, ok2 := item.(map[string]interface{})
		if !ok2 {
			continue
		}
		cert := parseCertificate(obj, now)
		certs = append(certs, cert)
	}

	response.List(c, certs, int64(len(certs)))
}

// ListIssuers 列出 Issuer 與 ClusterIssuer
func (h *CertManagerHandler) ListIssuers(c *gin.Context) {
	dynClient, err := h.dynamicClient(c, c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無法連線到叢集: "+err.Error())
		return
	}

	namespace := c.Query("namespace")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	var issuers []IssuerSummary

	// Namespaced Issuers
	var issuerRaw interface{}
	if namespace != "" && namespace != "all" {
		issuerRaw, err = dynClient.Resource(issuerGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		issuerRaw, err = dynClient.Resource(issuerGVR).List(ctx, metav1.ListOptions{})
	}
	if err == nil {
		if items, ok2 := extractUnstructuredItems(issuerRaw); ok2 {
			for _, item := range items {
				if obj, ok3 := item.(map[string]interface{}); ok3 {
					issuers = append(issuers, parseIssuer(obj, "Issuer"))
				}
			}
		}
	}

	// ClusterIssuers (cluster-scoped)
	clusterIssuerRaw, clErr := dynClient.Resource(clusterIssuerGVR).List(ctx, metav1.ListOptions{})
	if clErr == nil && clusterIssuerRaw != nil {
		if items, ok2 := extractUnstructuredItems(clusterIssuerRaw); ok2 {
			for _, item := range items {
				if obj, ok3 := item.(map[string]interface{}); ok3 {
					issuers = append(issuers, parseIssuer(obj, "ClusterIssuer"))
				}
			}
		}
	}

	if issuers == nil {
		issuers = []IssuerSummary{}
	}
	response.List(c, issuers, int64(len(issuers)))
}

// --- helpers ---

func extractUnstructuredItems(raw interface{}) ([]interface{}, bool) {
	m, ok := raw.(interface {
		UnstructuredContent() map[string]interface{}
	})
	if !ok {
		return nil, false
	}
	content := m.UnstructuredContent()
	itemsRaw, ok := content["items"]
	if !ok {
		return nil, false
	}
	items, ok := itemsRaw.([]interface{})
	return items, ok
}

func parseCertificate(obj map[string]interface{}, now time.Time) CertificateSummary {
	meta, _ := obj["metadata"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})
	status, _ := obj["status"].(map[string]interface{})

	cert := CertificateSummary{
		Name:      strVal(meta, "name"),
		Namespace: strVal(meta, "namespace"),
	}

	// spec fields
	cert.SecretName = strVal(spec, "secretName")
	if issuerRef, ok := spec["issuerRef"].(map[string]interface{}); ok {
		cert.Issuer = strVal(issuerRef, "name")
		cert.IssuerKind = strVal(issuerRef, "kind")
	}
	if dnsRaw, ok := spec["dnsNames"].([]interface{}); ok {
		for _, d := range dnsRaw {
			if s, ok2 := d.(string); ok2 {
				cert.DNSNames = append(cert.DNSNames, s)
			}
		}
	}

	// status conditions → ready
	if conditions, ok := status["conditions"].([]interface{}); ok {
		for _, c := range conditions {
			if cm, ok2 := c.(map[string]interface{}); ok2 {
				if cm["type"] == "Ready" && cm["status"] == "True" {
					cert.Ready = true
				}
			}
		}
	}

	// notBefore / notAfter
	cert.NotBefore = strVal(status, "notBefore")
	cert.NotAfter = strVal(status, "notAfter")

	// compute days left & status string
	if cert.NotAfter != "" {
		if t, err := time.Parse(time.RFC3339, cert.NotAfter); err == nil {
			cert.DaysLeft = int(t.Sub(now).Hours() / 24)
			switch {
			case cert.DaysLeft < 0:
				cert.Status = "Expired"
			case cert.DaysLeft <= 30:
				cert.Status = "Expiring"
			case cert.Ready:
				cert.Status = "Valid"
			default:
				cert.Status = "NotReady"
			}
		}
	} else if !cert.Ready {
		cert.Status = "NotReady"
	} else {
		cert.Status = "Valid"
	}

	return cert
}

func parseIssuer(obj map[string]interface{}, kind string) IssuerSummary {
	meta, _ := obj["metadata"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})
	status, _ := obj["status"].(map[string]interface{})

	issuer := IssuerSummary{
		Name:      strVal(meta, "name"),
		Namespace: strVal(meta, "namespace"),
		Kind:      kind,
	}

	// detect type from spec keys
	for _, t := range []string{"acme", "selfSigned", "ca", "vault", "venafi"} {
		if _, ok := spec[t]; ok {
			switch t {
			case "acme":
				issuer.Type = "ACME"
			case "selfSigned":
				issuer.Type = "SelfSigned"
			case "ca":
				issuer.Type = "CA"
			case "vault":
				issuer.Type = "Vault"
			case "venafi":
				issuer.Type = "Venafi"
			}
			break
		}
	}

	// ready from conditions
	if conditions, ok := status["conditions"].([]interface{}); ok {
		for _, c := range conditions {
			if cm, ok2 := c.(map[string]interface{}); ok2 {
				if cm["type"] == "Ready" && cm["status"] == "True" {
					issuer.Ready = true
				}
			}
		}
	}

	return issuer
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}
