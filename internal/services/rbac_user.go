package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/templates/rbac"
	"github.com/shaia/Synapse/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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
