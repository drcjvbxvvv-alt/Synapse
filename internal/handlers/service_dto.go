package handlers

import "time"

// ServiceInfo Service資訊
type ServiceInfo struct {
	Name                string                `json:"name"`
	Namespace           string                `json:"namespace"`
	Type                string                `json:"type"`
	ClusterIP           string                `json:"clusterIP"`
	ExternalIPs         []string              `json:"externalIPs,omitempty"`
	Ports               []ServicePort         `json:"ports"`
	Selector            map[string]string     `json:"selector"`
	SessionAffinity     string                `json:"sessionAffinity"`
	LoadBalancerIP      string                `json:"loadBalancerIP,omitempty"`
	LoadBalancerIngress []LoadBalancerIngress `json:"loadBalancerIngress,omitempty"`
	ExternalName        string                `json:"externalName,omitempty"`
	CreatedAt           time.Time             `json:"createdAt"`
	Labels              map[string]string     `json:"labels"`
	Annotations         map[string]string     `json:"annotations"`
}

// ServicePort Service連接埠資訊
type ServicePort struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort string `json:"targetPort"`
	NodePort   int32  `json:"nodePort,omitempty"`
}

// LoadBalancerIngress 負載均衡器入口資訊
type LoadBalancerIngress struct {
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// CreateServiceRequest 建立Service請求
type CreateServiceRequest struct {
	Namespace string           `json:"namespace" binding:"required"`
	YAML      string           `json:"yaml,omitempty"`     // YAML方式建立
	FormData  *ServiceFormData `json:"formData,omitempty"` // 表單方式建立
}

// ServiceFormData Service表單資料
type ServiceFormData struct {
	Name            string            `json:"name" binding:"required"`
	Type            string            `json:"type" binding:"required"` // ClusterIP, NodePort, LoadBalancer
	Selector        map[string]string `json:"selector"`
	Ports           []ServicePortForm `json:"ports" binding:"required"`
	SessionAffinity string            `json:"sessionAffinity"`
	ExternalIPs     []string          `json:"externalIPs,omitempty"`
	LoadBalancerIP  string            `json:"loadBalancerIP,omitempty"`
	ExternalName    string            `json:"externalName,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

// ServicePortForm Service連接埠表單
type ServicePortForm struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"` // TCP, UDP, SCTP
	Port       int32  `json:"port" binding:"required"`
	TargetPort string `json:"targetPort"` // 可以是數字或字串
	NodePort   int32  `json:"nodePort,omitempty"`
}
