package handlers

import (
	"context"
	"strconv"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// getIngresses 獲取Ingresses
func (h *IngressHandler) getIngresses(clientset kubernetes.Interface, namespace string) ([]IngressInfo, error) {
	var ingressList *networkingv1.IngressList
	var err error

	if namespace == "" || namespace == "_all_" {
		ingressList, err = clientset.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	} else {
		ingressList, err = clientset.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	ingresses := make([]IngressInfo, 0, len(ingressList.Items))
	for _, ing := range ingressList.Items {
		ingresses = append(ingresses, h.convertToIngressInfo(&ing))
	}

	return ingresses, nil
}

// convertToIngressInfo 轉換為IngressInfo
func (h *IngressHandler) convertToIngressInfo(ing *networkingv1.Ingress) IngressInfo {
	// 轉換規則
	rules := make([]IngressRuleInfo, 0, len(ing.Spec.Rules))
	for _, rule := range ing.Spec.Rules {
		paths := make([]IngressPathInfo, 0)
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				pathType := ""
				if path.PathType != nil {
					pathType = string(*path.PathType)
				}

				servicePort := ""
				if path.Backend.Service != nil {
					if path.Backend.Service.Port.Number > 0 {
						servicePort = strconv.Itoa(int(path.Backend.Service.Port.Number))
					} else {
						servicePort = path.Backend.Service.Port.Name
					}
				}

				paths = append(paths, IngressPathInfo{
					Path:     path.Path,
					PathType: pathType,
					ServiceName: func() string {
						if path.Backend.Service != nil {
							return path.Backend.Service.Name
						}
						return ""
					}(),
					ServicePort: servicePort,
				})
			}
		}

		rules = append(rules, IngressRuleInfo{
			Host:  rule.Host,
			Paths: paths,
		})
	}

	// 轉換TLS
	tls := make([]IngressTLSInfo, 0, len(ing.Spec.TLS))
	for _, t := range ing.Spec.TLS {
		tls = append(tls, IngressTLSInfo{
			Hosts:      t.Hosts,
			SecretName: t.SecretName,
		})
	}

	// 轉換LoadBalancer狀態
	lbStatus := make([]LoadBalancerStatus, 0, len(ing.Status.LoadBalancer.Ingress))
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		lbStatus = append(lbStatus, LoadBalancerStatus{
			IP:       lb.IP,
			Hostname: lb.Hostname,
		})
	}

	return IngressInfo{
		Name:             ing.Name,
		Namespace:        ing.Namespace,
		IngressClassName: ing.Spec.IngressClassName,
		Rules:            rules,
		TLS:              tls,
		LoadBalancer:     lbStatus,
		CreatedAt:        ing.CreationTimestamp.Time,
		Labels:           ing.Labels,
		Annotations:      ing.Annotations,
	}
}

// filterIngresses 過濾Ingresses
func (h *IngressHandler) filterIngresses(ingresses []IngressInfo, ingressClass, search string) []IngressInfo {
	filtered := make([]IngressInfo, 0)
	for _, ing := range ingresses {
		// IngressClass過濾
		if ingressClass != "" {
			if ing.IngressClassName == nil || *ing.IngressClassName != ingressClass {
				continue
			}
		}

		// 搜尋過濾
		if search != "" {
			searchLower := strings.ToLower(search)
			matched := false

			// 匹配名稱和命名空間
			if strings.Contains(strings.ToLower(ing.Name), searchLower) ||
				strings.Contains(strings.ToLower(ing.Namespace), searchLower) {
				matched = true
			}

			// 匹配Host和路徑
			for _, rule := range ing.Rules {
				if strings.Contains(strings.ToLower(rule.Host), searchLower) {
					matched = true
					break
				}
				for _, path := range rule.Paths {
					if strings.Contains(strings.ToLower(path.ServiceName), searchLower) ||
						strings.Contains(strings.ToLower(path.Path), searchLower) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}

			if !matched {
				continue
			}
		}

		filtered = append(filtered, ing)
	}
	return filtered
}
