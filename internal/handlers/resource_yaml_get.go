package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/shaia/Synapse/internal/response"
)

// GetConfigMapYAML 獲取ConfigMap的YAML
func (h *ResourceYAMLHandler) GetConfigMapYAML(c *gin.Context) {
	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	cm, err := k8sClient.GetClientset().CoreV1().ConfigMaps(c.Param("namespace")).Get(ctx, c.Param("name"), metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "ConfigMap不存在: "+err.Error())
		return
	}
	clean := cm.DeepCopy()
	clean.ManagedFields = nil
	clean.Annotations = filterAnnotations(clean.Annotations)
	clean.APIVersion = "v1"
	clean.Kind = "ConfigMap"
	respondWithYAML(c, clean)
}

// GetSecretYAML 獲取Secret的YAML
func (h *ResourceYAMLHandler) GetSecretYAML(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	id, err := parseClusterID(clusterID)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(id)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	k8sClient, err := h.createK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "建立K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	secret, err := k8sClient.GetClientset().CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Secret不存在: "+err.Error())
		return
	}

	cleanSecret := secret.DeepCopy()
	cleanSecret.ManagedFields = nil
	cleanSecret.Annotations = filterAnnotations(cleanSecret.Annotations)
	cleanSecret.APIVersion = "v1"
	cleanSecret.Kind = "Secret"

	yamlBytes, err := sigsyaml.Marshal(cleanSecret)
	if err != nil {
		response.InternalError(c, "轉換YAML失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"yaml": string(yamlBytes),
	})
}

// GetServiceYAMLClean 獲取乾淨的Service YAML（用於編輯）
func (h *ResourceYAMLHandler) GetServiceYAMLClean(c *gin.Context) {
	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	svc, err := k8sClient.GetClientset().CoreV1().Services(c.Param("namespace")).Get(ctx, c.Param("name"), metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Service不存在: "+err.Error())
		return
	}
	clean := svc.DeepCopy()
	clean.ManagedFields = nil
	clean.Annotations = filterAnnotations(clean.Annotations)
	clean.APIVersion = "v1"
	clean.Kind = "Service"
	respondWithYAML(c, clean)
}

// GetIngressYAMLClean 獲取乾淨的Ingress YAML（用於編輯）
func (h *ResourceYAMLHandler) GetIngressYAMLClean(c *gin.Context) {
	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	ing, err := k8sClient.GetClientset().NetworkingV1().Ingresses(c.Param("namespace")).Get(ctx, c.Param("name"), metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Ingress不存在: "+err.Error())
		return
	}
	clean := ing.DeepCopy()
	clean.ManagedFields = nil
	clean.Annotations = filterAnnotations(clean.Annotations)
	clean.APIVersion = "networking.k8s.io/v1"
	clean.Kind = "Ingress"
	respondWithYAML(c, clean)
}

// GetPVCYAMLClean 獲取乾淨的PVC YAML（用於編輯）
func (h *ResourceYAMLHandler) GetPVCYAMLClean(c *gin.Context) {
	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	pvc, err := k8sClient.GetClientset().CoreV1().PersistentVolumeClaims(c.Param("namespace")).Get(ctx, c.Param("name"), metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "PVC不存在: "+err.Error())
		return
	}
	clean := pvc.DeepCopy()
	clean.ManagedFields = nil
	clean.Annotations = filterAnnotations(clean.Annotations)
	clean.APIVersion = "v1"
	clean.Kind = "PersistentVolumeClaim"
	respondWithYAML(c, clean)
}

// GetPVYAMLClean 獲取乾淨的PV YAML（用於編輯）
func (h *ResourceYAMLHandler) GetPVYAMLClean(c *gin.Context) {
	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	pv, err := k8sClient.GetClientset().CoreV1().PersistentVolumes().Get(ctx, c.Param("name"), metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "PV不存在: "+err.Error())
		return
	}
	clean := pv.DeepCopy()
	clean.ManagedFields = nil
	clean.Annotations = filterAnnotations(clean.Annotations)
	clean.APIVersion = "v1"
	clean.Kind = "PersistentVolume"
	respondWithYAML(c, clean)
}

// GetStorageClassYAMLClean 獲取乾淨的StorageClass YAML（用於編輯）
func (h *ResourceYAMLHandler) GetStorageClassYAMLClean(c *gin.Context) {
	k8sClient, ok := h.prepareK8sClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	sc, err := k8sClient.GetClientset().StorageV1().StorageClasses().Get(ctx, c.Param("name"), metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "StorageClass不存在: "+err.Error())
		return
	}
	clean := sc.DeepCopy()
	clean.ManagedFields = nil
	clean.Annotations = filterAnnotations(clean.Annotations)
	clean.APIVersion = "storage.k8s.io/v1"
	clean.Kind = "StorageClass"
	respondWithYAML(c, clean)
}
