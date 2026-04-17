package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/templates/rbac"
	"github.com/shaia/Synapse/pkg/logger"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RBACService handles RBAC operations
type RBACService struct{}

// NewRBACService creates a new RBACService
func NewRBACService() *RBACService {
	return &RBACService{}
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Resource string `json:"resource"`
	Name     string `json:"name"`
	Action   string `json:"action"` // created, updated, skipped
	Error    string `json:"error,omitempty"`
}

// SyncPermissionsResult represents the overall sync result
type SyncPermissionsResult struct {
	Success bool          `json:"success"`
	Results []*SyncResult `json:"results"`
	Message string        `json:"message"`
}

// SyncPermissions syncs all Synapse RBAC resources to a cluster
func (s *RBACService) SyncPermissions(clientset *kubernetes.Clientset) (*SyncPermissionsResult, error) {
	ctx := context.Background()
	results := make([]*SyncResult, 0)
	hasError := false

	// 1. Create namespace
	nsResult := s.ensureNamespace(ctx, clientset)
	results = append(results, nsResult)
	if nsResult.Error != "" {
		hasError = true
	}

	// 2. Create ClusterRoles
	for _, cr := range rbac.GetAllClusterRoles() {
		result := s.ensureClusterRole(ctx, clientset, cr)
		results = append(results, result)
		if result.Error != "" {
			hasError = true
		}
	}

	// 3. Create ServiceAccounts
	saNames := []string{rbac.SAClusterAdmin, rbac.SAOps, rbac.SADev, rbac.SAReadonly}
	for _, saName := range saNames {
		result := s.ensureServiceAccount(ctx, clientset, saName)
		results = append(results, result)
		if result.Error != "" {
			hasError = true
		}
	}

	// 4. Create ClusterRoleBindings for admin and ops (they always have cluster-wide access)
	adminBinding := s.ensureClusterRoleBinding(ctx, clientset, "synapse-admin-binding", rbac.ClusterRoleClusterAdmin, rbac.SAClusterAdmin)
	results = append(results, adminBinding)
	if adminBinding.Error != "" {
		hasError = true
	}

	opsBinding := s.ensureClusterRoleBinding(ctx, clientset, "synapse-ops-binding", rbac.ClusterRoleOps, rbac.SAOps)
	results = append(results, opsBinding)
	if opsBinding.Error != "" {
		hasError = true
	}

	// Dev and readonly bindings are created dynamically based on user permissions

	message := "權限同步完成"
	if hasError {
		message = "權限同步完成，但有部分錯誤"
	}

	return &SyncPermissionsResult{
		Success: !hasError,
		Results: results,
		Message: message,
	}, nil
}

// ensureNamespace creates the Synapse namespace if it doesn't exist
func (s *RBACService) ensureNamespace(ctx context.Context, clientset *kubernetes.Clientset) *SyncResult {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   rbac.SynapseNamespace,
			Labels: rbac.GetSynapseLabels(),
		},
	}

	existing, err := clientset.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			if err != nil {
				return &SyncResult{Resource: "Namespace", Name: ns.Name, Action: "error", Error: err.Error()}
			}
			return &SyncResult{Resource: "Namespace", Name: ns.Name, Action: "created"}
		}
		return &SyncResult{Resource: "Namespace", Name: ns.Name, Action: "error", Error: err.Error()}
	}

	// Update labels if needed
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[rbac.LabelManagedBy] = rbac.LabelValue
	_, err = clientset.CoreV1().Namespaces().Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return &SyncResult{Resource: "Namespace", Name: ns.Name, Action: "error", Error: err.Error()}
	}
	return &SyncResult{Resource: "Namespace", Name: ns.Name, Action: "updated"}
}

// ensureClusterRole creates or updates a ClusterRole
func (s *RBACService) ensureClusterRole(ctx context.Context, clientset *kubernetes.Clientset, cr *rbacv1.ClusterRole) *SyncResult {
	existing, err := clientset.RbacV1().ClusterRoles().Get(ctx, cr.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = clientset.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
			if err != nil {
				logger.Error("Failed to create ClusterRole", "error", err)
				return &SyncResult{Resource: "ClusterRole", Name: cr.Name, Action: "error", Error: err.Error()}
			}
			return &SyncResult{Resource: "ClusterRole", Name: cr.Name, Action: "created"}
		}
		return &SyncResult{Resource: "ClusterRole", Name: cr.Name, Action: "error", Error: err.Error()}
	}

	// Update
	existing.Rules = cr.Rules
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[rbac.LabelManagedBy] = rbac.LabelValue
	_, err = clientset.RbacV1().ClusterRoles().Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return &SyncResult{Resource: "ClusterRole", Name: cr.Name, Action: "error", Error: err.Error()}
	}
	return &SyncResult{Resource: "ClusterRole", Name: cr.Name, Action: "updated"}
}

// ensureServiceAccount creates or updates a ServiceAccount
func (s *RBACService) ensureServiceAccount(ctx context.Context, clientset *kubernetes.Clientset, name string) *SyncResult {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rbac.SynapseNamespace,
			Labels:    rbac.GetSynapseLabels(),
		},
	}

	existing, err := clientset.CoreV1().ServiceAccounts(rbac.SynapseNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = clientset.CoreV1().ServiceAccounts(rbac.SynapseNamespace).Create(ctx, sa, metav1.CreateOptions{})
			if err != nil {
				return &SyncResult{Resource: "ServiceAccount", Name: name, Action: "error", Error: err.Error()}
			}
			return &SyncResult{Resource: "ServiceAccount", Name: name, Action: "created"}
		}
		return &SyncResult{Resource: "ServiceAccount", Name: name, Action: "error", Error: err.Error()}
	}

	// Update labels
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[rbac.LabelManagedBy] = rbac.LabelValue
	_, err = clientset.CoreV1().ServiceAccounts(rbac.SynapseNamespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return &SyncResult{Resource: "ServiceAccount", Name: name, Action: "error", Error: err.Error()}
	}
	return &SyncResult{Resource: "ServiceAccount", Name: name, Action: "updated"}
}

// ensureClusterRoleBinding creates or updates a ClusterRoleBinding
func (s *RBACService) ensureClusterRoleBinding(ctx context.Context, clientset *kubernetes.Clientset, name, clusterRoleName, saName string) *SyncResult {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: rbac.GetSynapseLabels(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: rbac.SynapseNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
	}

	existing, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = clientset.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
			if err != nil {
				return &SyncResult{Resource: "ClusterRoleBinding", Name: name, Action: "error", Error: err.Error()}
			}
			return &SyncResult{Resource: "ClusterRoleBinding", Name: name, Action: "created"}
		}
		return &SyncResult{Resource: "ClusterRoleBinding", Name: name, Action: "error", Error: err.Error()}
	}

	// Update
	existing.Subjects = crb.Subjects
	existing.RoleRef = crb.RoleRef
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[rbac.LabelManagedBy] = rbac.LabelValue
	_, err = clientset.RbacV1().ClusterRoleBindings().Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return &SyncResult{Resource: "ClusterRoleBinding", Name: name, Action: "error", Error: err.Error()}
	}
	return &SyncResult{Resource: "ClusterRoleBinding", Name: name, Action: "updated"}
}

// EnsureRoleBinding creates or updates a RoleBinding for namespace-scoped permissions
func (s *RBACService) EnsureRoleBinding(clientset *kubernetes.Clientset, namespace, bindingName, clusterRoleName, saName, saNamespace string) error {
	ctx := context.Background()

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: namespace,
			Labels:    rbac.GetSynapseLabels(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: saNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
	}

	existing, err := clientset.RbacV1().RoleBindings(namespace).Get(ctx, bindingName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = clientset.RbacV1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{})
			return err
		}
		return err
	}

	// Update
	existing.Subjects = rb.Subjects
	existing.RoleRef = rb.RoleRef
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[rbac.LabelManagedBy] = rbac.LabelValue
	_, err = clientset.RbacV1().RoleBindings(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// DeleteRoleBinding deletes a RoleBinding
func (s *RBACService) DeleteRoleBinding(clientset *kubernetes.Clientset, namespace, bindingName string) error {
	ctx := context.Background()
	err := clientset.RbacV1().RoleBindings(namespace).Delete(ctx, bindingName, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// GetSyncStatus checks the sync status of Synapse RBAC resources
func (s *RBACService) GetSyncStatus(clientset *kubernetes.Clientset) (*SyncStatusResult, error) {
	ctx := context.Background()
	result := &SyncStatusResult{
		Synced:    true,
		Resources: make([]*ResourceStatus, 0),
	}

	// Check namespace
	_, err := clientset.CoreV1().Namespaces().Get(ctx, rbac.SynapseNamespace, metav1.GetOptions{})
	nsStatus := &ResourceStatus{Resource: "Namespace", Name: rbac.SynapseNamespace}
	if err != nil {
		nsStatus.Exists = false
		result.Synced = false
	} else {
		nsStatus.Exists = true
	}
	result.Resources = append(result.Resources, nsStatus)

	// Check ClusterRoles
	for _, cr := range rbac.GetAllClusterRoles() {
		_, err := clientset.RbacV1().ClusterRoles().Get(ctx, cr.Name, metav1.GetOptions{})
		crStatus := &ResourceStatus{Resource: "ClusterRole", Name: cr.Name}
		if err != nil {
			crStatus.Exists = false
			result.Synced = false
		} else {
			crStatus.Exists = true
		}
		result.Resources = append(result.Resources, crStatus)
	}

	// Check ServiceAccounts
	saNames := []string{rbac.SAClusterAdmin, rbac.SAOps, rbac.SADev, rbac.SAReadonly}
	for _, saName := range saNames {
		_, err := clientset.CoreV1().ServiceAccounts(rbac.SynapseNamespace).Get(ctx, saName, metav1.GetOptions{})
		saStatus := &ResourceStatus{Resource: "ServiceAccount", Name: saName}
		if err != nil {
			saStatus.Exists = false
			result.Synced = false
		} else {
			saStatus.Exists = true
		}
		result.Resources = append(result.Resources, saStatus)
	}

	return result, nil
}

// SyncStatusResult represents the sync status
type SyncStatusResult struct {
	Synced    bool              `json:"synced"`
	Resources []*ResourceStatus `json:"resources"`
}

// ResourceStatus represents a single resource status
type ResourceStatus struct {
	Resource string `json:"resource"`
	Name     string `json:"name"`
	Exists   bool   `json:"exists"`
}

// GetServiceAccountToken gets the token for a ServiceAccount
func (s *RBACService) GetServiceAccountToken(clientset *kubernetes.Clientset, saName string) (string, error) {
	ctx := context.Background()

	// Get the ServiceAccount
	sa, err := clientset.CoreV1().ServiceAccounts(rbac.SynapseNamespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get ServiceAccount: %w", err)
	}

	// For Kubernetes 1.24+, we need to create a token manually
	// First, try to find an existing secret
	for _, ref := range sa.Secrets {
		secret, err := clientset.CoreV1().Secrets(rbac.SynapseNamespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		if token, ok := secret.Data["token"]; ok {
			return string(token), nil
		}
	}

	// If no secret found, create a token using TokenRequest API (K8s 1.22+)
	tokenRequest, err := clientset.CoreV1().ServiceAccounts(rbac.SynapseNamespace).CreateToken(
		ctx,
		saName,
		&authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				ExpirationSeconds: int64Ptr(3600), // 1 hour
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}

	return tokenRequest.Status.Token, nil
}

func int64Ptr(i int64) *int64 {
	return &i
}

// CreateCustomClusterRole creates a custom ClusterRole
func (s *RBACService) CreateCustomClusterRole(clientset *kubernetes.Clientset, name string, rules []rbacv1.PolicyRule) error {
	ctx := context.Background()

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: rbac.GetSynapseLabels(),
		},
		Rules: rules,
	}

	_, err := clientset.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
	return err
}

// CreateCustomRole creates a custom Role in a namespace
func (s *RBACService) CreateCustomRole(clientset *kubernetes.Clientset, namespace, name string, rules []rbacv1.PolicyRule) error {
	ctx := context.Background()

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    rbac.GetSynapseLabels(),
		},
		Rules: rules,
	}

	_, err := clientset.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{})
	return err
}

// ListClusterRoles lists all ClusterRoles
func (s *RBACService) ListClusterRoles(clientset *kubernetes.Clientset) ([]rbacv1.ClusterRole, error) {
	ctx := context.Background()

	list, err := clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

// ListRoles lists all Roles in a namespace
func (s *RBACService) ListRoles(clientset *kubernetes.Clientset, namespace string) ([]rbacv1.Role, error) {
	ctx := context.Background()

	list, err := clientset.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

// DeleteClusterRole deletes a ClusterRole
func (s *RBACService) DeleteClusterRole(clientset *kubernetes.Clientset, name string) error {
	ctx := context.Background()
	return clientset.RbacV1().ClusterRoles().Delete(ctx, name, metav1.DeleteOptions{})
}

// DeleteRole deletes a Role
func (s *RBACService) DeleteRole(clientset *kubernetes.Clientset, namespace, name string) error {
	ctx := context.Background()
	return clientset.RbacV1().Roles(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

