package handlers

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// getServices 獲取Services
func (h *ServiceHandler) getServices(clientset kubernetes.Interface, namespace string) ([]ServiceInfo, error) {
	var serviceList *corev1.ServiceList
	var err error

	if namespace == "" || namespace == "_all_" {
		serviceList, err = clientset.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
	} else {
		serviceList, err = clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	services := make([]ServiceInfo, 0, len(serviceList.Items))
	for _, svc := range serviceList.Items {
		services = append(services, h.convertToServiceInfo(&svc))
	}

	return services, nil
}

// convertToServiceInfo 轉換為ServiceInfo
func (h *ServiceHandler) convertToServiceInfo(svc *corev1.Service) ServiceInfo {
	ports := make([]ServicePort, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		ports = append(ports, ServicePort{
			Name:       p.Name,
			Protocol:   string(p.Protocol),
			Port:       p.Port,
			TargetPort: h.getTargetPortString(p.TargetPort),
			NodePort:   p.NodePort,
		})
	}

	lbIngress := make([]LoadBalancerIngress, 0, len(svc.Status.LoadBalancer.Ingress))
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		lbIngress = append(lbIngress, LoadBalancerIngress{
			IP:       ing.IP,
			Hostname: ing.Hostname,
		})
	}

	return ServiceInfo{
		Name:                svc.Name,
		Namespace:           svc.Namespace,
		Type:                string(svc.Spec.Type),
		ClusterIP:           svc.Spec.ClusterIP,
		ExternalIPs:         svc.Spec.ExternalIPs,
		Ports:               ports,
		Selector:            svc.Spec.Selector,
		SessionAffinity:     string(svc.Spec.SessionAffinity),
		LoadBalancerIP:      svc.Spec.LoadBalancerIP,
		LoadBalancerIngress: lbIngress,
		ExternalName:        svc.Spec.ExternalName,
		CreatedAt:           svc.CreationTimestamp.Time,
		Labels:              svc.Labels,
		Annotations:         svc.Annotations,
	}
}

// getTargetPortString 獲取目標連接埠字串
func (h *ServiceHandler) getTargetPortString(targetPort intstr.IntOrString) string {
	if targetPort.Type == intstr.Int {
		return strconv.Itoa(int(targetPort.IntVal))
	}
	return targetPort.StrVal
}

// filterServices 過濾Services
func (h *ServiceHandler) filterServices(services []ServiceInfo, serviceType, search string) []ServiceInfo {
	filtered := make([]ServiceInfo, 0)
	for _, svc := range services {
		// 型別過濾
		if serviceType != "" && svc.Type != serviceType {
			continue
		}

		// 搜尋過濾
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(svc.Name), searchLower) &&
				!strings.Contains(strings.ToLower(svc.Namespace), searchLower) &&
				!strings.Contains(strings.ToLower(svc.ClusterIP), searchLower) {
				continue
			}
		}

		filtered = append(filtered, svc)
	}
	return filtered
}

// convertEndpointsInfo 轉換Endpoints資訊
func (h *ServiceHandler) convertEndpointsInfo(endpoints *corev1.Endpoints) gin.H {
	subsets := make([]gin.H, 0, len(endpoints.Subsets))
	for _, subset := range endpoints.Subsets {
		addresses := make([]gin.H, 0, len(subset.Addresses))
		for _, addr := range subset.Addresses {
			addresses = append(addresses, gin.H{
				"ip":       addr.IP,
				"nodeName": addr.NodeName,
				"targetRef": func() gin.H {
					if addr.TargetRef != nil {
						return gin.H{
							"kind":      addr.TargetRef.Kind,
							"name":      addr.TargetRef.Name,
							"namespace": addr.TargetRef.Namespace,
						}
					}
					return nil
				}(),
			})
		}

		ports := make([]gin.H, 0, len(subset.Ports))
		for _, port := range subset.Ports {
			ports = append(ports, gin.H{
				"name":     port.Name,
				"port":     port.Port,
				"protocol": string(port.Protocol),
			})
		}

		subsets = append(subsets, gin.H{
			"addresses": addresses,
			"ports":     ports,
		})
	}

	return gin.H{
		"name":      endpoints.Name,
		"namespace": endpoints.Namespace,
		"subsets":   subsets,
	}
}
