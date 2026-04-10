package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/shaia/Synapse/internal/models"
)

// syncConfigMaps 同步與工作負載關聯的 ConfigMap
func (h *MultiClusterHandler) syncConfigMaps(
	ctx context.Context, src, dst kubernetes.Interface, srcNS, dstNS, workload string,
) ([]string, error) {
	cms, err := src.CoreV1().ConfigMaps(srcNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var synced []string
	for _, cm := range cms.Items {
		// 跳過系統 ConfigMap
		if cm.Name == "kube-root-ca.crt" {
			continue
		}
		clean := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cm.Name, Namespace: dstNS, Labels: cm.Labels, Annotations: cm.Annotations},
			Data:       cm.Data,
			BinaryData: cm.BinaryData,
		}
		_, err := dst.CoreV1().ConfigMaps(dstNS).Get(ctx, cm.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.CoreV1().ConfigMaps(dstNS).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			_, err = dst.CoreV1().ConfigMaps(dstNS).Update(ctx, clean, metav1.UpdateOptions{})
		}
		if err == nil {
			synced = append(synced, cm.Name)
		}
	}
	return synced, nil
}

// syncSecrets 同步與工作負載關聯的 Secret（跳過 SA token 等系統型別）
func (h *MultiClusterHandler) syncSecrets(
	ctx context.Context, src, dst kubernetes.Interface, srcNS, dstNS, workload string,
) ([]string, error) {
	secrets, err := src.CoreV1().Secrets(srcNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var synced []string
	for _, sec := range secrets.Items {
		if sec.Type == corev1.SecretTypeServiceAccountToken ||
			sec.Type == "kubernetes.io/dockerconfigjson" && sec.Name == "default" {
			continue
		}
		clean := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: sec.Name, Namespace: dstNS, Labels: sec.Labels, Annotations: sec.Annotations},
			Type:       sec.Type,
			Data:       sec.Data,
		}
		_, err := dst.CoreV1().Secrets(dstNS).Get(ctx, sec.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.CoreV1().Secrets(dstNS).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			_, err = dst.CoreV1().Secrets(dstNS).Update(ctx, clean, metav1.UpdateOptions{})
		}
		if err == nil {
			synced = append(synced, sec.Name)
		}
	}
	return synced, nil
}

// migrateWorkload 遷移工作負載到目標叢集
func (h *MultiClusterHandler) migrateWorkload(ctx context.Context, src, dst kubernetes.Interface, req MigrateRequest) error {
	switch req.WorkloadKind {
	case "Deployment":
		dep, err := src.AppsV1().Deployments(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clean := cleanDeployment(dep, req.TargetNamespace)
		_, err = dst.AppsV1().Deployments(req.TargetNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.AppsV1().Deployments(req.TargetNamespace).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			clean.ResourceVersion = ""
			_, err = dst.AppsV1().Deployments(req.TargetNamespace).Update(ctx, clean, metav1.UpdateOptions{})
		}
		return err
	case "StatefulSet":
		sts, err := src.AppsV1().StatefulSets(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clean := cleanStatefulSet(sts, req.TargetNamespace)
		_, err = dst.AppsV1().StatefulSets(req.TargetNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.AppsV1().StatefulSets(req.TargetNamespace).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			clean.ResourceVersion = ""
			_, err = dst.AppsV1().StatefulSets(req.TargetNamespace).Update(ctx, clean, metav1.UpdateOptions{})
		}
		return err
	case "DaemonSet":
		ds, err := src.AppsV1().DaemonSets(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clean := cleanDaemonSet(ds, req.TargetNamespace)
		_, err = dst.AppsV1().DaemonSets(req.TargetNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = dst.AppsV1().DaemonSets(req.TargetNamespace).Create(ctx, clean, metav1.CreateOptions{})
		} else if err == nil {
			clean.ResourceVersion = ""
			_, err = dst.AppsV1().DaemonSets(req.TargetNamespace).Update(ctx, clean, metav1.UpdateOptions{})
		}
		return err
	}
	return fmt.Errorf("不支援的工作負載型別: %s", req.WorkloadKind)
}

// executeSync 執行配置同步策略，回傳 status / message / details JSON
func (h *MultiClusterHandler) executeSync(policy *models.SyncPolicy) (status, message, details string) {
	var targetIDs []uint
	if err := json.Unmarshal([]byte(policy.TargetClusters), &targetIDs); err != nil {
		return "failed", "解析目標叢集列表失敗: " + err.Error(), ""
	}
	var resourceNames []string
	_ = json.Unmarshal([]byte(policy.ResourceNames), &resourceNames)

	srcClient, err := h.getClientByID(policy.SourceClusterID)
	if err != nil {
		return "failed", "取得來源叢集客戶端失敗: " + err.Error(), ""
	}

	ctx := context.Background()
	results := make([]syncDetailItem, 0, len(targetIDs))
	successCount := 0

	for _, tid := range targetIDs {
		dstClient, err := h.getClientByID(tid)
		if err != nil {
			results = append(results, syncDetailItem{ClusterID: tid, Status: "failed", Message: err.Error()})
			continue
		}

		if err := ensureNamespace(ctx, dstClient, policy.SourceNamespace); err != nil {
			results = append(results, syncDetailItem{ClusterID: tid, Status: "failed", Message: "建立命名空間失敗: " + err.Error()})
			continue
		}

		var syncErr error
		switch policy.ResourceType {
		case "ConfigMap":
			syncErr = h.syncSpecificConfigMaps(ctx, srcClient, dstClient, policy.SourceNamespace, resourceNames, policy.ConflictPolicy)
		case "Secret":
			syncErr = h.syncSpecificSecrets(ctx, srcClient, dstClient, policy.SourceNamespace, resourceNames, policy.ConflictPolicy)
		default:
			syncErr = fmt.Errorf("不支援的資源型別: %s", policy.ResourceType)
		}

		if syncErr != nil {
			results = append(results, syncDetailItem{ClusterID: tid, Status: "failed", Message: syncErr.Error()})
		} else {
			results = append(results, syncDetailItem{ClusterID: tid, Status: "success", Message: "同步成功"})
			successCount++
		}
	}

	detailsBytes, _ := json.Marshal(results)
	details = string(detailsBytes)

	if successCount == len(targetIDs) {
		return "success", fmt.Sprintf("成功同步至 %d 個叢集", successCount), details
	} else if successCount > 0 {
		return "partial", fmt.Sprintf("部分成功（%d/%d）", successCount, len(targetIDs)), details
	}
	return "failed", "所有目標叢集同步失敗", details
}

func (h *MultiClusterHandler) syncSpecificConfigMaps(
	ctx context.Context, src, dst kubernetes.Interface, ns string, names []string, conflictPolicy string,
) error {
	for _, name := range names {
		cm, err := src.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("取得 ConfigMap %s 失敗: %v", name, err)
		}
		clean := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cm.Name, Namespace: ns, Labels: cm.Labels},
			Data:       cm.Data,
			BinaryData: cm.BinaryData,
		}
		_, getErr := dst.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(getErr) {
			_, err = dst.CoreV1().ConfigMaps(ns).Create(ctx, clean, metav1.CreateOptions{})
		} else if getErr == nil {
			if conflictPolicy == "skip" {
				continue
			}
			_, err = dst.CoreV1().ConfigMaps(ns).Update(ctx, clean, metav1.UpdateOptions{})
		} else {
			err = getErr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *MultiClusterHandler) syncSpecificSecrets(
	ctx context.Context, src, dst kubernetes.Interface, ns string, names []string, conflictPolicy string,
) error {
	for _, name := range names {
		sec, err := src.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("取得 Secret %s 失敗: %v", name, err)
		}
		clean := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: sec.Name, Namespace: ns, Labels: sec.Labels},
			Type:       sec.Type,
			Data:       sec.Data,
		}
		_, getErr := dst.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(getErr) {
			_, err = dst.CoreV1().Secrets(ns).Create(ctx, clean, metav1.CreateOptions{})
		} else if getErr == nil {
			if conflictPolicy == "skip" {
				continue
			}
			_, err = dst.CoreV1().Secrets(ns).Update(ctx, clean, metav1.UpdateOptions{})
		} else {
			err = getErr
		}
		if err != nil {
			return err
		}
	}
	return nil
}
