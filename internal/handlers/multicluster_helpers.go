package handlers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// getWorkloadResources 取得工作負載的資源請求量及相依的 ConfigMap/Secret 名稱
func (h *MultiClusterHandler) getWorkloadResources(
	ctx context.Context, client kubernetes.Interface, req MigrateRequest,
) (cpuMillicores, memMiB float64, cmNames, secNames []string, err error) {
	switch req.WorkloadKind {
	case "Deployment":
		dep, e := client.AppsV1().Deployments(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if e != nil {
			return 0, 0, nil, nil, fmt.Errorf("找不到 Deployment %s: %v", req.WorkloadName, e)
		}
		cpuMillicores, memMiB = sumContainerRequests(dep.Spec.Template.Spec.Containers)
		cmNames, secNames = extractEnvRefs(dep.Spec.Template.Spec)
	case "StatefulSet":
		sts, e := client.AppsV1().StatefulSets(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if e != nil {
			return 0, 0, nil, nil, fmt.Errorf("找不到 StatefulSet %s: %v", req.WorkloadName, e)
		}
		cpuMillicores, memMiB = sumContainerRequests(sts.Spec.Template.Spec.Containers)
		cmNames, secNames = extractEnvRefs(sts.Spec.Template.Spec)
	case "DaemonSet":
		ds, e := client.AppsV1().DaemonSets(req.SourceNamespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if e != nil {
			return 0, 0, nil, nil, fmt.Errorf("找不到 DaemonSet %s: %v", req.WorkloadName, e)
		}
		cpuMillicores, memMiB = sumContainerRequests(ds.Spec.Template.Spec.Containers)
		cmNames, secNames = extractEnvRefs(ds.Spec.Template.Spec)
	default:
		err = fmt.Errorf("不支援的工作負載型別: %s", req.WorkloadKind)
	}
	return
}

// sumContainerRequests 加總所有容器的資源請求
func sumContainerRequests(containers []corev1.Container) (cpuMillicores, memMiB float64) {
	for _, c := range containers {
		if req := c.Resources.Requests; req != nil {
			if cpu, ok := req[corev1.ResourceCPU]; ok {
				cpuMillicores += float64(cpu.MilliValue())
			}
			if mem, ok := req[corev1.ResourceMemory]; ok {
				memMiB += float64(mem.Value()) / (1024 * 1024)
			}
		}
	}
	return
}

// extractEnvRefs 從 PodSpec 提取 ConfigMap 和 Secret 引用名稱
func extractEnvRefs(spec corev1.PodSpec) (cmNames, secNames []string) {
	cmSet := map[string]bool{}
	secSet := map[string]bool{}
	for _, c := range spec.Containers {
		for _, env := range c.Env {
			if env.ValueFrom != nil {
				if r := env.ValueFrom.ConfigMapKeyRef; r != nil {
					cmSet[r.Name] = true
				}
				if r := env.ValueFrom.SecretKeyRef; r != nil {
					secSet[r.Name] = true
				}
			}
		}
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef != nil {
				cmSet[ef.ConfigMapRef.Name] = true
			}
			if ef.SecretRef != nil {
				secSet[ef.SecretRef.Name] = true
			}
		}
	}
	for v := range cmSet {
		cmNames = append(cmNames, v)
	}
	for v := range secSet {
		secNames = append(secNames, v)
	}
	return
}

// calcFreeResources 計算叢集可用資源（allocatable - requested）
func (h *MultiClusterHandler) calcFreeResources(ctx context.Context, client kubernetes.Interface) (freeCPU, freeMem float64, err error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, 0, err
	}
	var totalCPU, totalMem float64
	for _, n := range nodes.Items {
		if cpu, ok := n.Status.Allocatable[corev1.ResourceCPU]; ok {
			totalCPU += float64(cpu.MilliValue())
		}
		if mem, ok := n.Status.Allocatable[corev1.ResourceMemory]; ok {
			totalMem += float64(mem.Value()) / (1024 * 1024)
		}
	}

	pods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: "status.phase=Running"})
	if err != nil {
		return 0, 0, err
	}
	var usedCPU, usedMem float64
	for _, pod := range pods.Items {
		for _, c := range pod.Spec.Containers {
			if req := c.Resources.Requests; req != nil {
				if cpu, ok := req[corev1.ResourceCPU]; ok {
					usedCPU += float64(cpu.MilliValue())
				}
				if mem, ok := req[corev1.ResourceMemory]; ok {
					usedMem += float64(mem.Value()) / (1024 * 1024)
				}
			}
		}
	}
	return totalCPU - usedCPU, totalMem - usedMem, nil
}

// ensureNamespace 確保命名空間存在，不存在則建立
func ensureNamespace(ctx context.Context, client kubernetes.Interface, ns string) error {
	_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}, metav1.CreateOptions{})
	return err
}

// cleanDeployment 去除執行時欄位，準備跨叢集 apply
func cleanDeployment(dep *appsv1.Deployment, targetNS string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dep.Name,
			Namespace:   targetNS,
			Labels:      dep.Labels,
			Annotations: filterAnnotations(dep.Annotations),
		},
		Spec: dep.Spec,
	}
}

func cleanStatefulSet(sts *appsv1.StatefulSet, targetNS string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        sts.Name,
			Namespace:   targetNS,
			Labels:      sts.Labels,
			Annotations: filterAnnotations(sts.Annotations),
		},
		Spec: sts.Spec,
	}
}

func cleanDaemonSet(ds *appsv1.DaemonSet, targetNS string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ds.Name,
			Namespace:   targetNS,
			Labels:      ds.Labels,
			Annotations: filterAnnotations(ds.Annotations),
		},
		Spec: ds.Spec,
	}
}

// filterAnnotations 過濾掉 kubectl/k8s 系統管理用 annotation
func filterAnnotations(ann map[string]string) map[string]string {
	skip := map[string]bool{
		"kubectl.kubernetes.io/last-applied-configuration": true,
		"deployment.kubernetes.io/revision":                true,
	}
	result := map[string]string{}
	for k, v := range ann {
		if !skip[k] {
			result[k] = v
		}
	}
	return result
}
