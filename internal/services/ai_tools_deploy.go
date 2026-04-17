package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/shaia/Synapse/pkg/logger"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (e *ToolExecutor) listDeployments(clusterID uint, namespace string) (string, error) {
	lister := e.listerProvider.DeploymentsLister(clusterID)
	if lister == nil {
		return "", fmt.Errorf("叢集 Informer 未就緒")
	}

	type deploySummary struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Replicas  int32  `json:"replicas"`
		Ready     int32  `json:"ready"`
		Available int32  `json:"available"`
		Age       string `json:"age"`
		Images    string `json:"images"`
	}

	var deploys []*appsv1.Deployment
	var err error
	if namespace != "" {
		deploys, err = lister.Deployments(namespace).List(labels.Everything())
	} else {
		deploys, err = lister.List(labels.Everything())
	}
	if err != nil {
		return "", fmt.Errorf("列出 Deployment 失敗: %w", err)
	}

	result := make([]deploySummary, 0, len(deploys))
	for _, d := range deploys {
		var replicas int32 = 1
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		result = append(result, deploySummary{
			Name:      d.Name,
			Namespace: d.Namespace,
			Replicas:  replicas,
			Ready:     d.Status.ReadyReplicas,
			Available: d.Status.AvailableReplicas,
			Age:       formatAge(d.CreationTimestamp.Time),
			Images:    getContainerImages(d.Spec.Template.Spec.Containers),
		})
	}

	data, _ := json.Marshal(map[string]interface{}{
		"total":       len(result),
		"deployments": result,
	})
	return string(data), nil
}

func (e *ToolExecutor) getDeploymentDetail(ctx context.Context, clusterID uint, namespace, name string) (string, error) {
	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("獲取 Deployment 失敗: %w", err)
	}

	conditions := make([]map[string]string, 0)
	for _, cond := range deploy.Status.Conditions {
		conditions = append(conditions, map[string]string{
			"type":    string(cond.Type),
			"status":  string(cond.Status),
			"reason":  cond.Reason,
			"message": cond.Message,
		})
	}

	detail := map[string]interface{}{
		"name":              deploy.Name,
		"namespace":         deploy.Namespace,
		"replicas":          deploy.Spec.Replicas,
		"readyReplicas":     deploy.Status.ReadyReplicas,
		"availableReplicas": deploy.Status.AvailableReplicas,
		"updatedReplicas":   deploy.Status.UpdatedReplicas,
		"strategy":          deploy.Spec.Strategy.Type,
		"labels":            deploy.Labels,
		"images":            getContainerImages(deploy.Spec.Template.Spec.Containers),
		"conditions":        conditions,
		"age":               formatAge(deploy.CreationTimestamp.Time),
	}

	data, _ := json.Marshal(detail)
	return string(data), nil
}

func (e *ToolExecutor) scaleDeployment(ctx context.Context, clusterID uint, namespace, name string, replicas int, confirmed bool) (string, error) {
	if !confirmed {
		return fmt.Sprintf(`{"action":"scale_deployment","namespace":"%s","name":"%s","target_replicas":%d,"status":"awaiting_confirmation","message":"請確認是否將 %s/%s 的副本數調整為 %d？"}`,
			namespace, name, replicas, namespace, name, replicas), nil
	}

	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	scale, err := clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("獲取 Deployment scale 失敗: %w", err)
	}

	if replicas < 0 || replicas > math.MaxInt32 {
		return "", fmt.Errorf("副本數 %d 超出有效範圍", replicas)
	}
	scale.Spec.Replicas = int32(replicas) // #nosec G115 -- 已做邊界檢查
	_, err = clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("擴縮容失敗: %w", err)
	}

	logger.Info("AI 工具執行擴縮容", "deployment", fmt.Sprintf("%s/%s", namespace, name), "replicas", replicas)
	return fmt.Sprintf(`{"status":"success","message":"已將 %s/%s 的副本數調整為 %d"}`, namespace, name, replicas), nil
}

func (e *ToolExecutor) restartDeployment(ctx context.Context, clusterID uint, namespace, name string, confirmed bool) (string, error) {
	if !confirmed {
		return fmt.Sprintf(`{"action":"restart_deployment","namespace":"%s","name":"%s","status":"awaiting_confirmation","message":"請確認是否重啟 %s/%s？"}`,
			namespace, name, namespace, name), nil
	}

	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("獲取 Deployment 失敗: %w", err)
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = clientset.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("重啟 Deployment 失敗: %w", err)
	}

	logger.Info("AI 工具執行重啟", "deployment", fmt.Sprintf("%s/%s", namespace, name))
	return fmt.Sprintf(`{"status":"success","message":"已觸發 %s/%s 滾動重啟"}`, namespace, name), nil
}
