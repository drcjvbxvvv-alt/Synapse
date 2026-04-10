package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/shaia/Synapse/internal/response"
)

// ApplyConfigMapYAML 應用ConfigMap YAML
func (h *ResourceYAMLHandler) ApplyConfigMapYAML(c *gin.Context) {
	var req ResourceYAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}

	// 解析YAML
	var cm corev1.ConfigMap
	if err := yaml.Unmarshal([]byte(req.YAML), &cm); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	// 驗證kind
	if cm.Kind != "" && cm.Kind != "ConfigMap" {
		response.BadRequest(c, "YAML型別錯誤，期望ConfigMap，實際為: "+cm.Kind)
		return
	}

	if cm.Namespace == "" {
		cm.Namespace = "default"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if req.DryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	// 嘗試獲取現有資源
	existing, err := clientset.CoreV1().ConfigMaps(cm.Namespace).Get(ctx, cm.Name, metav1.GetOptions{})
	var result *corev1.ConfigMap
	isCreated := false

	if err == nil {
		// 資源存在，執行更新
		cm.ResourceVersion = existing.ResourceVersion
		result, err = clientset.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, &cm, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "更新ConfigMap失敗: "+err.Error())
			return
		}
	} else {
		// 資源不存在，執行建立
		isCreated = true
		result, err = clientset.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, &cm, metav1.CreateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "建立ConfigMap失敗: "+err.Error())
			return
		}
	}

	response.OK(c, ResourceYAMLResponse{
		Name:            result.Name,
		Namespace:       result.Namespace,
		Kind:            "ConfigMap",
		ResourceVersion: result.ResourceVersion,
		IsCreated:       isCreated,
	})
}

// ApplySecretYAML 應用Secret YAML
func (h *ResourceYAMLHandler) ApplySecretYAML(c *gin.Context) {
	var req ResourceYAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}

	var secret corev1.Secret
	if err := yaml.Unmarshal([]byte(req.YAML), &secret); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	if secret.Kind != "" && secret.Kind != "Secret" {
		response.BadRequest(c, "YAML型別錯誤，期望Secret，實際為: "+secret.Kind)
		return
	}

	if secret.Namespace == "" {
		secret.Namespace = "default"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if req.DryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	existing, err := clientset.CoreV1().Secrets(secret.Namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	var result *corev1.Secret
	isCreated := false

	if err == nil {
		secret.ResourceVersion = existing.ResourceVersion
		result, err = clientset.CoreV1().Secrets(secret.Namespace).Update(ctx, &secret, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "更新Secret失敗: "+err.Error())
			return
		}
	} else {
		isCreated = true
		result, err = clientset.CoreV1().Secrets(secret.Namespace).Create(ctx, &secret, metav1.CreateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "建立Secret失敗: "+err.Error())
			return
		}
	}

	response.OK(c, ResourceYAMLResponse{
		Name:            result.Name,
		Namespace:       result.Namespace,
		Kind:            "Secret",
		ResourceVersion: result.ResourceVersion,
		IsCreated:       isCreated,
	})
}

// ApplyServiceYAML 應用Service YAML
func (h *ResourceYAMLHandler) ApplyServiceYAML(c *gin.Context) {
	var req ResourceYAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}

	var svc corev1.Service
	if err := yaml.Unmarshal([]byte(req.YAML), &svc); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	if svc.Kind != "" && svc.Kind != "Service" {
		response.BadRequest(c, "YAML型別錯誤，期望Service，實際為: "+svc.Kind)
		return
	}

	if svc.Namespace == "" {
		svc.Namespace = "default"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if req.DryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	existing, err := clientset.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
	var result *corev1.Service
	isCreated := false

	if err == nil {
		// Service更新時需要保留ClusterIP
		svc.ResourceVersion = existing.ResourceVersion
		svc.Spec.ClusterIP = existing.Spec.ClusterIP
		svc.Spec.ClusterIPs = existing.Spec.ClusterIPs
		result, err = clientset.CoreV1().Services(svc.Namespace).Update(ctx, &svc, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "更新Service失敗: "+err.Error())
			return
		}
	} else {
		isCreated = true
		result, err = clientset.CoreV1().Services(svc.Namespace).Create(ctx, &svc, metav1.CreateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "建立Service失敗: "+err.Error())
			return
		}
	}

	response.OK(c, ResourceYAMLResponse{
		Name:            result.Name,
		Namespace:       result.Namespace,
		Kind:            "Service",
		ResourceVersion: result.ResourceVersion,
		IsCreated:       isCreated,
	})
}

// ApplyIngressYAML 應用Ingress YAML
func (h *ResourceYAMLHandler) ApplyIngressYAML(c *gin.Context) {
	var req ResourceYAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}

	var ing networkingv1.Ingress
	if err := yaml.Unmarshal([]byte(req.YAML), &ing); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	if ing.Kind != "" && ing.Kind != "Ingress" {
		response.BadRequest(c, "YAML型別錯誤，期望Ingress，實際為: "+ing.Kind)
		return
	}

	if ing.Namespace == "" {
		ing.Namespace = "default"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if req.DryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	existing, err := clientset.NetworkingV1().Ingresses(ing.Namespace).Get(ctx, ing.Name, metav1.GetOptions{})
	var result *networkingv1.Ingress
	isCreated := false

	if err == nil {
		ing.ResourceVersion = existing.ResourceVersion
		result, err = clientset.NetworkingV1().Ingresses(ing.Namespace).Update(ctx, &ing, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "更新Ingress失敗: "+err.Error())
			return
		}
	} else {
		isCreated = true
		result, err = clientset.NetworkingV1().Ingresses(ing.Namespace).Create(ctx, &ing, metav1.CreateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "建立Ingress失敗: "+err.Error())
			return
		}
	}

	response.OK(c, ResourceYAMLResponse{
		Name:            result.Name,
		Namespace:       result.Namespace,
		Kind:            "Ingress",
		ResourceVersion: result.ResourceVersion,
		IsCreated:       isCreated,
	})
}

// ApplyPVCYAML 應用PVC YAML
func (h *ResourceYAMLHandler) ApplyPVCYAML(c *gin.Context) {
	var req ResourceYAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}

	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal([]byte(req.YAML), &pvc); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	if pvc.Kind != "" && pvc.Kind != "PersistentVolumeClaim" {
		response.BadRequest(c, "YAML型別錯誤，期望PersistentVolumeClaim，實際為: "+pvc.Kind)
		return
	}

	if pvc.Namespace == "" {
		pvc.Namespace = "default"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if req.DryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	existing, err := clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	var result *corev1.PersistentVolumeClaim
	isCreated := false

	if err == nil {
		pvc.ResourceVersion = existing.ResourceVersion
		// PVC的spec大部分欄位是不可變的，只能更新某些欄位
		pvc.Spec.VolumeName = existing.Spec.VolumeName
		result, err = clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(ctx, &pvc, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "更新PVC失敗: "+err.Error())
			return
		}
	} else {
		isCreated = true
		result, err = clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, &pvc, metav1.CreateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "建立PVC失敗: "+err.Error())
			return
		}
	}

	response.OK(c, ResourceYAMLResponse{
		Name:            result.Name,
		Namespace:       result.Namespace,
		Kind:            "PersistentVolumeClaim",
		ResourceVersion: result.ResourceVersion,
		IsCreated:       isCreated,
	})
}

// ApplyPVYAML 應用PV YAML
func (h *ResourceYAMLHandler) ApplyPVYAML(c *gin.Context) {
	var req ResourceYAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}

	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal([]byte(req.YAML), &pv); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	if pv.Kind != "" && pv.Kind != "PersistentVolume" {
		response.BadRequest(c, "YAML型別錯誤，期望PersistentVolume，實際為: "+pv.Kind)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if req.DryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	existing, err := clientset.CoreV1().PersistentVolumes().Get(ctx, pv.Name, metav1.GetOptions{})
	var result *corev1.PersistentVolume
	isCreated := false

	if err == nil {
		pv.ResourceVersion = existing.ResourceVersion
		result, err = clientset.CoreV1().PersistentVolumes().Update(ctx, &pv, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "更新PV失敗: "+err.Error())
			return
		}
	} else {
		isCreated = true
		result, err = clientset.CoreV1().PersistentVolumes().Create(ctx, &pv, metav1.CreateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "建立PV失敗: "+err.Error())
			return
		}
	}

	response.OK(c, ResourceYAMLResponse{
		Name:            result.Name,
		Kind:            "PersistentVolume",
		ResourceVersion: result.ResourceVersion,
		IsCreated:       isCreated,
	})
}

// ApplyStorageClassYAML 應用StorageClass YAML
func (h *ResourceYAMLHandler) ApplyStorageClassYAML(c *gin.Context) {
	var req ResourceYAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}

	var sc storagev1.StorageClass
	if err := yaml.Unmarshal([]byte(req.YAML), &sc); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	if sc.Kind != "" && sc.Kind != "StorageClass" {
		response.BadRequest(c, "YAML型別錯誤，期望StorageClass，實際為: "+sc.Kind)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if req.DryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	existing, err := clientset.StorageV1().StorageClasses().Get(ctx, sc.Name, metav1.GetOptions{})
	var result *storagev1.StorageClass
	isCreated := false

	if err == nil {
		sc.ResourceVersion = existing.ResourceVersion
		result, err = clientset.StorageV1().StorageClasses().Update(ctx, &sc, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "更新StorageClass失敗: "+err.Error())
			return
		}
	} else {
		isCreated = true
		result, err = clientset.StorageV1().StorageClasses().Create(ctx, &sc, metav1.CreateOptions{DryRun: dryRunOpt})
		if err != nil {
			response.InternalError(c, "建立StorageClass失敗: "+err.Error())
			return
		}
	}

	response.OK(c, ResourceYAMLResponse{
		Name:            result.Name,
		Kind:            "StorageClass",
		ResourceVersion: result.ResourceVersion,
		IsCreated:       isCreated,
	})
}
