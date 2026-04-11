package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shaia/Synapse/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SecretSprawlReport Secret 蔓延分析報告
type SecretSprawlReport struct {
	Namespace     string             `json:"namespace"`
	TotalSecrets  int                `json:"total_secrets"`
	OrphanedCount int                `json:"orphaned_count"`
	OverExposed   int                `json:"over_exposed_count"`
	Items         []SecretSprawlItem `json:"items"`
}

// SecretSprawlItem 單個 Secret 的分析結果
type SecretSprawlItem struct {
	Name       string           `json:"name"`
	Namespace  string           `json:"namespace"`
	Type       string           `json:"type"`
	Keys       []string         `json:"keys"`
	Age        string           `json:"age"`
	MountedBy  []SecretConsumer `json:"mounted_by"`
	MountCount int              `json:"mount_count"`
	Status     string           `json:"status"` // "active", "orphaned", "over_exposed"
}

// SecretConsumer Secret 的使用者
type SecretConsumer struct {
	Kind      string `json:"kind"`       // Pod, Deployment, StatefulSet, DaemonSet
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	MountType string `json:"mount_type"` // "volume", "env", "envFrom"
}

// SecurityAuditService 安全審計服務
type SecurityAuditService struct{}

// NewSecurityAuditService 建立安全審計服務
func NewSecurityAuditService() *SecurityAuditService {
	return &SecurityAuditService{}
}

// ScanSecretSprawl 掃描 Secret 蔓延情況
func (s *SecurityAuditService) ScanSecretSprawl(ctx context.Context, clientset kubernetes.Interface, namespace string) (*SecretSprawlReport, error) {
	// 1. List all secrets in namespace (or all namespaces)
	secrets, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list secrets in %q: %w", namespace, err)
	}

	// 2. List all pods to find secret consumers
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods in %q: %w", namespace, err)
	}

	// 3. Build secret -> consumers map
	consumerMap := make(map[string][]SecretConsumer) // key: namespace/secretName
	for i := range pods.Items {
		ownerKind, ownerName := resolveOwner(&pods.Items[i])
		s.findSecretConsumers(&pods.Items[i], ownerKind, ownerName, consumerMap)
	}

	// 4. Build report items
	var items []SecretSprawlItem
	orphanedCount := 0
	overExposedCount := 0

	for i := range secrets.Items {
		secret := &secrets.Items[i]

		// Skip default service account tokens and helm secrets
		if isSystemSecret(secret) {
			continue
		}

		key := secret.Namespace + "/" + secret.Name
		consumers := consumerMap[key]

		// Deduplicate consumers by owner
		consumers = deduplicateConsumers(consumers)

		status := "active"
		if len(consumers) == 0 {
			status = "orphaned"
			orphanedCount++
		} else if len(consumers) > 5 {
			status = "over_exposed"
			overExposedCount++
		}

		var keys []string
		for k := range secret.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		age := formatAge(secret.CreationTimestamp.Time)

		items = append(items, SecretSprawlItem{
			Name:       secret.Name,
			Namespace:  secret.Namespace,
			Type:       string(secret.Type),
			Keys:       keys,
			Age:        age,
			MountedBy:  consumers,
			MountCount: len(consumers),
			Status:     status,
		})
	}

	// Sort: orphaned first, then over-exposed, then active
	sort.Slice(items, func(i, j int) bool {
		order := map[string]int{"orphaned": 0, "over_exposed": 1, "active": 2}
		return order[items[i].Status] < order[items[j].Status]
	})

	nsLabel := namespace
	if nsLabel == "" {
		nsLabel = "all"
	}

	logger.Info("secret sprawl scan completed",
		"namespace", nsLabel,
		"total", len(items),
		"orphaned", orphanedCount,
		"over_exposed", overExposedCount,
	)

	return &SecretSprawlReport{
		Namespace:     nsLabel,
		TotalSecrets:  len(items),
		OrphanedCount: orphanedCount,
		OverExposed:   overExposedCount,
		Items:         items,
	}, nil
}

func (s *SecurityAuditService) findSecretConsumers(pod *corev1.Pod, ownerKind, ownerName string, consumerMap map[string][]SecretConsumer) {
	ns := pod.Namespace

	// Check volume mounts
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil {
			key := ns + "/" + vol.Secret.SecretName
			consumerMap[key] = append(consumerMap[key], SecretConsumer{
				Kind:      ownerKind,
				Name:      ownerName,
				Namespace: ns,
				MountType: "volume",
			})
		}
	}

	// Check env and envFrom in all containers
	allContainers := make([]corev1.Container, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	allContainers = append(allContainers, pod.Spec.Containers...)
	allContainers = append(allContainers, pod.Spec.InitContainers...)

	for _, container := range allContainers {
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				key := ns + "/" + env.ValueFrom.SecretKeyRef.Name
				consumerMap[key] = append(consumerMap[key], SecretConsumer{
					Kind:      ownerKind,
					Name:      ownerName,
					Namespace: ns,
					MountType: "env",
				})
			}
		}
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				key := ns + "/" + envFrom.SecretRef.Name
				consumerMap[key] = append(consumerMap[key], SecretConsumer{
					Kind:      ownerKind,
					Name:      ownerName,
					Namespace: ns,
					MountType: "envFrom",
				})
			}
		}
	}
}

func resolveOwner(pod *corev1.Pod) (string, string) {
	if len(pod.OwnerReferences) == 0 {
		return "Pod", pod.Name
	}
	owner := pod.OwnerReferences[0]
	// For ReplicaSet, resolve to Deployment
	if owner.Kind == "ReplicaSet" {
		// Extract deployment name by removing the RS hash suffix
		parts := strings.Split(owner.Name, "-")
		if len(parts) > 1 {
			return "Deployment", strings.Join(parts[:len(parts)-1], "-")
		}
	}
	return owner.Kind, owner.Name
}

func isSystemSecret(secret *corev1.Secret) bool {
	name := secret.Name
	// Skip service account tokens
	if secret.Type == corev1.SecretTypeServiceAccountToken {
		return true
	}
	// Skip Helm release secrets
	if strings.HasPrefix(name, "sh.helm.release.") {
		return true
	}
	// Skip default token secrets
	if strings.HasPrefix(name, "default-token-") {
		return true
	}
	return false
}

func deduplicateConsumers(consumers []SecretConsumer) []SecretConsumer {
	seen := make(map[string]bool)
	var result []SecretConsumer
	for _, c := range consumers {
		key := c.Kind + "/" + c.Namespace + "/" + c.Name + "/" + c.MountType
		if !seen[key] {
			seen[key] = true
			result = append(result, c)
		}
	}
	return result
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	if d.Hours() < 24 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dm", days/30)
}
