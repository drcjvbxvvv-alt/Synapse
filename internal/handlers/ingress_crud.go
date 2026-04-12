package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// CreateIngress 建立Ingress
func (h *IngressHandler) CreateIngress(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var req CreateIngressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var ingress *networkingv1.Ingress

	// 根據建立方式選擇處理邏輯
	if req.YAML != "" {
		// YAML方式建立
		ingress, err = h.createIngressFromYAML(ctx, clientset, req.Namespace, req.YAML)
	} else if req.FormData != nil {
		// 表單方式建立
		ingress, err = h.createIngressFromForm(ctx, clientset, req.Namespace, req.FormData)
	} else {
		response.BadRequest(c, "必須提供YAML或表單資料")
		return
	}

	if err != nil {
		logger.Error("建立Ingress失敗", "error", err, "clusterId", clusterID)
		if k8serrors.IsInvalid(err) || k8serrors.IsAlreadyExists(err) {
			response.BadRequest(c, fmt.Sprintf("建立Ingress失敗: %v", err))
		} else {
			response.InternalError(c, fmt.Sprintf("建立Ingress失敗: %v", err))
		}
		return
	}

	logger.Info("Ingress建立成功", "clusterId", clusterID, "namespace", ingress.Namespace, "name", ingress.Name)
	response.OK(c, h.convertToIngressInfo(ingress))
}

// UpdateIngress 更新Ingress
func (h *IngressHandler) UpdateIngress(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req CreateIngressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var ingress *networkingv1.Ingress

	// 根據更新方式選擇處理邏輯
	if req.YAML != "" {
		// YAML方式更新
		ingress, err = h.updateIngressFromYAML(ctx, clientset, namespace, name, req.YAML)
	} else if req.FormData != nil {
		// 表單方式更新
		ingress, err = h.updateIngressFromForm(ctx, clientset, namespace, name, req.FormData)
	} else {
		response.BadRequest(c, "必須提供YAML或表單資料")
		return
	}

	if err != nil {
		logger.Error("更新Ingress失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("更新Ingress失敗: %v", err))
		return
	}

	logger.Info("Ingress更新成功", "clusterId", clusterID, "namespace", ingress.Namespace, "name", ingress.Name)
	response.OK(c, h.convertToIngressInfo(ingress))
}

// createIngressFromYAML 從YAML建立Ingress
func (h *IngressHandler) createIngressFromYAML(ctx context.Context, clientset kubernetes.Interface, namespace, yamlContent string) (*networkingv1.Ingress, error) {
	var ingress networkingv1.Ingress
	if err := yaml.Unmarshal([]byte(yamlContent), &ingress); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	// 確保namespace正確
	if ingress.Namespace == "" {
		ingress.Namespace = namespace
	}

	createdIngress, err := clientset.NetworkingV1().Ingresses(ingress.Namespace).Create(ctx, &ingress, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdIngress, nil
}

// createIngressFromForm 從表單建立Ingress
func (h *IngressHandler) createIngressFromForm(ctx context.Context, clientset kubernetes.Interface, namespace string, formData *IngressFormData) (*networkingv1.Ingress, error) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        formData.Name,
			Namespace:   namespace,
			Labels:      formData.Labels,
			Annotations: formData.Annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: formData.IngressClassName,
		},
	}

	// 新增規則
	rules := make([]networkingv1.IngressRule, 0, len(formData.Rules))
	for _, r := range formData.Rules {
		paths := make([]networkingv1.HTTPIngressPath, 0, len(r.Paths))
		for _, p := range r.Paths {
			pathType := networkingv1.PathType(p.PathType)
			paths = append(paths, networkingv1.HTTPIngressPath{
				Path:     p.Path,
				PathType: &pathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: p.ServiceName,
						Port: networkingv1.ServiceBackendPort{
							Number: p.ServicePort,
						},
					},
				},
			})
		}

		rules = append(rules, networkingv1.IngressRule{
			Host: r.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		})
	}
	ingress.Spec.Rules = rules

	// 新增TLS
	if len(formData.TLS) > 0 {
		tls := make([]networkingv1.IngressTLS, 0, len(formData.TLS))
		for _, t := range formData.TLS {
			tls = append(tls, networkingv1.IngressTLS{
				Hosts:      t.Hosts,
				SecretName: t.SecretName,
			})
		}
		ingress.Spec.TLS = tls
	}

	createdIngress, err := clientset.NetworkingV1().Ingresses(namespace).Create(ctx, ingress, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdIngress, nil
}

// updateIngressFromYAML 從YAML更新Ingress
func (h *IngressHandler) updateIngressFromYAML(ctx context.Context, clientset kubernetes.Interface, namespace, name, yamlContent string) (*networkingv1.Ingress, error) {
	var ingress networkingv1.Ingress
	if err := yaml.Unmarshal([]byte(yamlContent), &ingress); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	// 獲取現有Ingress
	existingIngress, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// 保留ResourceVersion
	ingress.ResourceVersion = existingIngress.ResourceVersion
	ingress.Namespace = namespace
	ingress.Name = name

	updatedIngress, err := clientset.NetworkingV1().Ingresses(namespace).Update(ctx, &ingress, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedIngress, nil
}

// updateIngressFromForm 從表單更新Ingress
func (h *IngressHandler) updateIngressFromForm(ctx context.Context, clientset kubernetes.Interface, namespace, name string, formData *IngressFormData) (*networkingv1.Ingress, error) {
	// 獲取現有Ingress
	existingIngress, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// 更新Spec
	existingIngress.Spec.IngressClassName = formData.IngressClassName

	// 更新規則
	rules := make([]networkingv1.IngressRule, 0, len(formData.Rules))
	for _, r := range formData.Rules {
		paths := make([]networkingv1.HTTPIngressPath, 0, len(r.Paths))
		for _, p := range r.Paths {
			pathType := networkingv1.PathType(p.PathType)
			paths = append(paths, networkingv1.HTTPIngressPath{
				Path:     p.Path,
				PathType: &pathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: p.ServiceName,
						Port: networkingv1.ServiceBackendPort{
							Number: p.ServicePort,
						},
					},
				},
			})
		}

		rules = append(rules, networkingv1.IngressRule{
			Host: r.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		})
	}
	existingIngress.Spec.Rules = rules

	// 更新TLS
	if len(formData.TLS) > 0 {
		tls := make([]networkingv1.IngressTLS, 0, len(formData.TLS))
		for _, t := range formData.TLS {
			tls = append(tls, networkingv1.IngressTLS{
				Hosts:      t.Hosts,
				SecretName: t.SecretName,
			})
		}
		existingIngress.Spec.TLS = tls
	} else {
		existingIngress.Spec.TLS = nil
	}

	// 更新Labels和Annotations
	if formData.Labels != nil {
		existingIngress.Labels = formData.Labels
	}
	if formData.Annotations != nil {
		existingIngress.Annotations = formData.Annotations
	}

	updatedIngress, err := clientset.NetworkingV1().Ingresses(namespace).Update(ctx, existingIngress, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedIngress, nil
}
