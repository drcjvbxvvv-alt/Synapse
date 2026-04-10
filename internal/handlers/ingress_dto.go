package handlers

import "time"

// IngressInfo Ingress資訊
type IngressInfo struct {
	Name             string               `json:"name"`
	Namespace        string               `json:"namespace"`
	IngressClassName *string              `json:"ingressClassName,omitempty"`
	Rules            []IngressRuleInfo    `json:"rules"`
	TLS              []IngressTLSInfo     `json:"tls,omitempty"`
	LoadBalancer     []LoadBalancerStatus `json:"loadBalancer,omitempty"`
	CreatedAt        time.Time            `json:"createdAt"`
	Labels           map[string]string    `json:"labels"`
	Annotations      map[string]string    `json:"annotations"`
}

// IngressRuleInfo Ingress規則資訊
type IngressRuleInfo struct {
	Host  string            `json:"host"`
	Paths []IngressPathInfo `json:"paths"`
}

// IngressPathInfo Ingress路徑資訊
type IngressPathInfo struct {
	Path        string `json:"path"`
	PathType    string `json:"pathType"`
	ServiceName string `json:"serviceName"`
	ServicePort string `json:"servicePort"`
}

// IngressTLSInfo Ingress TLS資訊
type IngressTLSInfo struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secretName"`
}

// LoadBalancerStatus 負載均衡器狀態
type LoadBalancerStatus struct {
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// CreateIngressRequest 建立Ingress請求
type CreateIngressRequest struct {
	Namespace string           `json:"namespace" binding:"required"`
	YAML      string           `json:"yaml,omitempty"`     // YAML方式建立
	FormData  *IngressFormData `json:"formData,omitempty"` // 表單方式建立
}

// IngressFormData Ingress表單資料
type IngressFormData struct {
	Name             string                `json:"name" binding:"required"`
	IngressClassName *string               `json:"ingressClassName,omitempty"`
	Rules            []IngressRuleFormData `json:"rules" binding:"required"`
	TLS              []IngressTLSFormData  `json:"tls,omitempty"`
	Labels           map[string]string     `json:"labels,omitempty"`
	Annotations      map[string]string     `json:"annotations,omitempty"`
}

// IngressRuleFormData Ingress規則表單資料
type IngressRuleFormData struct {
	Host  string                `json:"host"`
	Paths []IngressPathFormData `json:"paths" binding:"required"`
}

// IngressPathFormData Ingress路徑表單資料
type IngressPathFormData struct {
	Path        string `json:"path" binding:"required"`
	PathType    string `json:"pathType" binding:"required"` // Prefix, Exact, ImplementationSpecific
	ServiceName string `json:"serviceName" binding:"required"`
	ServicePort int32  `json:"servicePort" binding:"required"`
}

// IngressTLSFormData Ingress TLS表單資料
type IngressTLSFormData struct {
	Hosts      []string `json:"hosts" binding:"required"`
	SecretName string   `json:"secretName" binding:"required"`
}
