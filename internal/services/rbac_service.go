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

// ========== 動態使用者 RBAC 管理 ==========

// UserRBACConfig 使用者 RBAC 配置
type UserRBACConfig struct {
	UserID         uint
	PermissionType string   // admin, ops, dev, readonly, custom
	Namespaces     []string // ["*"] 表示全部, ["ns1", "ns2"] 表示部分
	ClusterRoleRef string   // 自定義權限時使用的 ClusterRole 名稱
}

// GetUserServiceAccountName 獲取使用者專屬 SA 名稱
func GetUserServiceAccountName(userID uint) string {
	return fmt.Sprintf("synapse-user-%d-sa", userID)
}

// GetUserRoleBindingName 獲取使用者 RoleBinding 名稱
func GetUserRoleBindingName(userID uint, permissionType string) string {
	return fmt.Sprintf("synapse-user-%d-%s", userID, permissionType)
}

// GetUserClusterRoleBindingName 獲取使用者 ClusterRoleBinding 名稱
func GetUserClusterRoleBindingName(userID uint, permissionType string) string {
	return fmt.Sprintf("synapse-user-%d-%s-cluster", userID, permissionType)
}

// hasAllNamespaces 檢查命名空間列表是否包含全部權限
func hasAllNamespaces(namespaces []string) bool {
	for _, ns := range namespaces {
		if ns == "*" {
			return true
		}
	}
	return false
}

// EnsureUserRBAC 確保使用者的 RBAC 資源存在
// 根據權限配置自動建立 SA 和繫結
func (s *RBACService) EnsureUserRBAC(clientset *kubernetes.Clientset, config *UserRBACConfig) error {
	ctx := context.Background()
	hasAllAccess := hasAllNamespaces(config.Namespaces)

	// 獲取對應的 ClusterRole
	clusterRoleName := config.ClusterRoleRef
	if clusterRoleName == "" {
		clusterRoleName = rbac.GetClusterRoleByPermissionType(config.PermissionType)
	}

	// admin 和 ops 使用固定 SA，不需要動態建立
	if config.PermissionType == "admin" || config.PermissionType == "ops" {
		logger.Info("admin/ops 使用固定 SA，無需動態建立", "userID", config.UserID, "permissionType", config.PermissionType)
		return nil
	}

	// dev/readonly 全部命名空間時使用固定 SA
	if (config.PermissionType == "dev" || config.PermissionType == "readonly") && hasAllAccess {
		logger.Info("全部命名空間使用固定 SA，無需動態建立", "userID", config.UserID, "permissionType", config.PermissionType)
		return nil
	}

	// 需要動態建立使用者專屬 SA 和繫結
	saName := GetUserServiceAccountName(config.UserID)
	logger.Info("建立使用者專屬 RBAC", "userID", config.UserID, "saName", saName, "permissionType", config.PermissionType, "namespaces", config.Namespaces)

	// 1. 建立使用者專屬 SA
	if err := s.ensureUserServiceAccount(ctx, clientset, saName); err != nil {
		return fmt.Errorf("建立使用者 SA 失敗: %w", err)
	}

	// 2. 根據命名空間範圍建立繫結
	if hasAllAccess {
		// 全部命名空間：建立 ClusterRoleBinding
		bindingName := GetUserClusterRoleBindingName(config.UserID, config.PermissionType)
		if err := s.ensureUserClusterRoleBinding(ctx, clientset, bindingName, clusterRoleName, saName); err != nil {
			return fmt.Errorf("建立 ClusterRoleBinding 失敗: %w", err)
		}
	} else {
		// 部分命名空間：為每個命名空間建立 RoleBinding
		bindingName := GetUserRoleBindingName(config.UserID, config.PermissionType)
		for _, namespace := range config.Namespaces {
			if namespace == "" || namespace == "*" {
				continue
			}
			if err := s.EnsureRoleBinding(clientset, namespace, bindingName, clusterRoleName, saName, rbac.SynapseNamespace); err != nil {
				return fmt.Errorf("建立 RoleBinding(%s) 失敗: %w", namespace, err)
			}
		}
	}

	return nil
}

// ensureUserServiceAccount 建立使用者專屬 SA
func (s *RBACService) ensureUserServiceAccount(ctx context.Context, clientset *kubernetes.Clientset, saName string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: rbac.SynapseNamespace,
			Labels:    rbac.GetSynapseLabels(),
		},
	}

	_, err := clientset.CoreV1().ServiceAccounts(rbac.SynapseNamespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = clientset.CoreV1().ServiceAccounts(rbac.SynapseNamespace).Create(ctx, sa, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			logger.Info("建立使用者 SA 成功", "saName", saName)
			return nil
		}
		return err
	}

	logger.Info("使用者 SA 已存在", "saName", saName)
	return nil
}

// ensureUserClusterRoleBinding 建立使用者 ClusterRoleBinding
func (s *RBACService) ensureUserClusterRoleBinding(ctx context.Context, clientset *kubernetes.Clientset, bindingName, clusterRoleName, saName string) error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   bindingName,
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

	existing, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, bindingName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = clientset.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			logger.Info("建立使用者 ClusterRoleBinding 成功", "bindingName", bindingName)
			return nil
		}
		return err
	}

	// 更新
	existing.Subjects = crb.Subjects
	existing.RoleRef = crb.RoleRef
	_, err = clientset.RbacV1().ClusterRoleBindings().Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// CleanupUserRBAC 清理使用者的 RBAC 資源
func (s *RBACService) CleanupUserRBAC(clientset *kubernetes.Clientset, userID uint, permissionType string, namespaces []string) error {
	ctx := context.Background()
	saName := GetUserServiceAccountName(userID)

	logger.Info("清理使用者 RBAC 資源", "userID", userID, "saName", saName)

	// 1. 刪除 ClusterRoleBinding
	crbName := GetUserClusterRoleBindingName(userID, permissionType)
	if err := clientset.RbacV1().ClusterRoleBindings().Delete(ctx, crbName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		logger.Warn("刪除 ClusterRoleBinding 失敗", "name", crbName, "error", err)
	}

	// 2. 刪除 RoleBinding（每個命名空間）
	rbName := GetUserRoleBindingName(userID, permissionType)
	for _, namespace := range namespaces {
		if namespace == "" || namespace == "*" {
			continue
		}
		if err := clientset.RbacV1().RoleBindings(namespace).Delete(ctx, rbName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			logger.Warn("刪除 RoleBinding 失敗", "namespace", namespace, "name", rbName, "error", err)
		}
	}

	// 3. 檢查是否還有其他權限使用這個 SA，如果沒有則刪除 SA
	// 暫時不刪除 SA，因為可能還有其他叢集的權限
	// TODO: 可以新增引用計數邏輯

	return nil
}

// GetEffectiveServiceAccount 獲取使用者應該使用的 SA 名稱
func (s *RBACService) GetEffectiveServiceAccount(config *UserRBACConfig) string {
	hasAllAccess := hasAllNamespaces(config.Namespaces)

	switch config.PermissionType {
	case "admin":
		return rbac.SAClusterAdmin
	case "ops":
		return rbac.SAOps
	case "dev":
		if hasAllAccess {
			return rbac.SADev
		}
		return GetUserServiceAccountName(config.UserID)
	case "readonly":
		if hasAllAccess {
			return rbac.SAReadonly
		}
		return GetUserServiceAccountName(config.UserID)
	case "custom":
		return GetUserServiceAccountName(config.UserID)
	default:
		return rbac.SAReadonly
	}
}
