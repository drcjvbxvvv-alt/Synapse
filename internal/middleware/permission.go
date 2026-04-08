package middleware

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
)

// PermissionMiddleware 權限中介軟體
type PermissionMiddleware struct {
	permissionService *services.PermissionService
}

// NewPermissionMiddleware 建立權限中介軟體
func NewPermissionMiddleware(permissionService *services.PermissionService) *PermissionMiddleware {
	return &PermissionMiddleware{
		permissionService: permissionService,
	}
}

// ClusterAccessRequired 叢集訪問權限檢查
// 檢查使用者是否有權限訪問指定叢集
func (m *PermissionMiddleware) ClusterAccessRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")
		if userID == 0 {
			response.Unauthorized(c, "未登入")
			return
		}

		// 獲取叢集ID
		clusterIDStr := c.Param("clusterID")
		if clusterIDStr == "" {
			clusterIDStr = c.Param("clusterId")
		}
		if clusterIDStr == "" {
			// 沒有叢集ID參數，跳過檢查
			c.Next()
			return
		}

		clusterID, err := strconv.ParseUint(clusterIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "無效的叢集ID")
			return
		}

		// 檢查權限
		permission, err := m.permissionService.GetUserClusterPermission(userID, uint(clusterID))
		if err != nil {
			response.Forbidden(c, "無權限訪問該叢集")
			return
		}

		// 將權限資訊存入上下文
		c.Set("cluster_permission", permission)
		c.Set("cluster_id", uint(clusterID))
		c.Next()
	}
}

// NamespaceAccessRequired 命名空間訪問權限檢查
// 需要在 ClusterAccessRequired 之後使用
func (m *PermissionMiddleware) NamespaceAccessRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 獲取權限資訊
		permissionInterface, exists := c.Get("cluster_permission")
		if !exists {
			response.Forbidden(c, "無叢集訪問權限")
			return
		}

		permission := permissionInterface.(*models.ClusterPermission)

		// 獲取命名空間參數
		namespace := c.Param("namespace")
		if namespace == "" {
			namespace = c.Query("namespace")
		}

		// 如果有命名空間參數，檢查權限
		if namespace != "" && !services.HasNamespaceAccess(permission, namespace) {
			response.Forbidden(c, namespaceForbiddenMsg(namespace, permission.GetNamespaceList()))
			return
		}

		c.Next()
	}
}

// ActionRequired 操作權限檢查
// 檢查使用者是否有權限執行指定操作
func (m *PermissionMiddleware) ActionRequired(actions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 獲取權限資訊
		permissionInterface, exists := c.Get("cluster_permission")
		if !exists {
			response.Forbidden(c, "無叢集訪問權限")
			return
		}

		permission := permissionInterface.(*models.ClusterPermission)

		// 檢查所有要求的操作權限
		for _, action := range actions {
			if !services.CanPerformAction(permission, action) {
				response.Forbidden(c, "權限不足，無法執行此操作")
				return
			}
		}

		c.Next()
	}
}

// AdminRequired 管理員權限檢查
// 只有管理員權限才能訪問
func (m *PermissionMiddleware) AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 獲取權限資訊
		permissionInterface, exists := c.Get("cluster_permission")
		if !exists {
			response.Forbidden(c, "無叢集訪問權限")
			return
		}

		permission := permissionInterface.(*models.ClusterPermission)

		if permission.PermissionType != models.PermissionTypeAdmin {
			response.Forbidden(c, "需要管理員權限")
			return
		}

		c.Next()
	}
}

// WriteRequired 寫權限檢查
// 只讀權限無法透過
func (m *PermissionMiddleware) WriteRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 獲取權限資訊
		permissionInterface, exists := c.Get("cluster_permission")
		if !exists {
			response.Forbidden(c, "無叢集訪問權限")
			return
		}

		permission := permissionInterface.(*models.ClusterPermission)

		if permission.PermissionType == models.PermissionTypeReadonly {
			response.Forbidden(c, "只讀權限無法執行寫操作")
			return
		}

		c.Next()
	}
}

// AutoWriteCheck 自動寫權限檢查
// 對於 POST/PUT/DELETE/PATCH 請求自動檢查寫權限
func (m *PermissionMiddleware) AutoWriteCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只對寫操作進行權限檢查
		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			c.Next()
			return
		}

		// 獲取權限資訊
		permissionInterface, exists := c.Get("cluster_permission")
		if !exists {
			// 如果沒有權限資訊，說明 ClusterAccessRequired 沒有執行或失敗
			response.Forbidden(c, "無叢集訪問權限")
			return
		}

		permission := permissionInterface.(*models.ClusterPermission)

		// 只讀權限無法執行寫操作
		if permission.PermissionType == models.PermissionTypeReadonly {
			response.Forbidden(c, "只讀權限無法執行寫操作，請聯絡管理員獲取更高權限")
			return
		}

		c.Next()
	}
}

// PlatformAdminRequired 平臺管理員權限檢查
// 判定邏輯：使用者名稱為 admin，或使用者（直接/透過使用者組）在任意叢集擁有 admin 權限型別
func PlatformAdminRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")
		username := c.GetString("username")
		if userID == 0 {
			response.Unauthorized(c, "未登入")
			return
		}

		if username == "admin" {
			c.Next()
			return
		}

		// 檢查使用者是否直接擁有 admin 權限
		var count int64
		if err := db.Model(&models.ClusterPermission{}).
			Where("user_id = ? AND permission_type = ?", userID, models.PermissionTypeAdmin).
			Count(&count).Error; err != nil {
			response.InternalError(c, "權限查詢失敗")
			return
		}
		if count > 0 {
			c.Next()
			return
		}

		// 檢查使用者所在使用者組是否擁有 admin 權限
		var groupIDs []uint
		if err := db.Model(&models.UserGroupMember{}).
			Where("user_id = ?", userID).
			Pluck("user_group_id", &groupIDs).Error; err != nil {
			response.InternalError(c, "使用者組查詢失敗")
			return
		}
		if len(groupIDs) > 0 {
			if err := db.Model(&models.ClusterPermission{}).
				Where("user_group_id IN ? AND permission_type = ?", groupIDs, models.PermissionTypeAdmin).
				Count(&count).Error; err != nil {
				response.InternalError(c, "使用者組權限查詢失敗")
				return
			}
			if count > 0 {
				c.Next()
				return
			}
		}

		response.Forbidden(c, "需要平臺管理員權限")
	}
}

// GetClusterPermission 從上下文獲取叢集權限
func GetClusterPermission(c *gin.Context) *models.ClusterPermission {
	permissionInterface, exists := c.Get("cluster_permission")
	if !exists {
		return nil
	}
	permission, ok := permissionInterface.(*models.ClusterPermission)
	if !ok {
		return nil
	}
	return permission
}

// GetCurrentUserID 從上下文獲取當前使用者ID
func GetCurrentUserID(c *gin.Context) uint {
	return c.GetUint("user_id")
}

// GetAllowedNamespaces 獲取使用者允許訪問的命名空間列表
// 返回: 命名空間列表, 是否有全部命名空間權限
func GetAllowedNamespaces(c *gin.Context) ([]string, bool) {
	permission := GetClusterPermission(c)
	if permission == nil {
		return []string{}, false
	}

	namespaces := permission.GetNamespaceList()
	for _, ns := range namespaces {
		if ns == "*" {
			return namespaces, true
		}
	}
	return namespaces, false
}

// HasNamespaceAccess 檢查是否有訪問指定命名空間的權限
func HasNamespaceAccess(c *gin.Context, namespace string) bool {
	permission := GetClusterPermission(c)
	if permission == nil {
		return false
	}
	return services.HasNamespaceAccess(permission, namespace)
}

// FilterNamespaces 過濾命名空間列表，只返回使用者有權限訪問的
func FilterNamespaces(c *gin.Context, namespaces []string) []string {
	permission := GetClusterPermission(c)
	if permission == nil {
		return []string{}
	}

	if services.HasAllNamespaceAccess(permission) {
		return namespaces
	}

	filtered := make([]string, 0)
	for _, ns := range namespaces {
		if services.HasNamespaceAccess(permission, ns) {
			filtered = append(filtered, ns)
		}
	}
	return filtered
}

// GetEffectiveNamespace 獲取有效的命名空間查詢參數
// 如果使用者請求的命名空間不在權限範圍內，返回空字串和false
// 如果使用者有全部權限，返回原始請求的命名空間
// 如果使用者沒有指定命名空間但只有部分權限，返回權限範圍內的第一個命名空間
func GetEffectiveNamespace(c *gin.Context, requestedNs string) (string, bool) {
	permission := GetClusterPermission(c)
	if permission == nil {
		return "", false
	}

	// 如果有全部權限
	if services.HasAllNamespaceAccess(permission) {
		return requestedNs, true
	}

	allowedNs := permission.GetNamespaceList()

	if requestedNs != "" {
		if services.HasNamespaceAccess(permission, requestedNs) {
			return requestedNs, true
		}
		return "", false // 無權訪問請求的命名空間
	}

	// 使用者沒有指定命名空間，返回空字串讓後續邏輯處理
	// 後續邏輯會遍歷使用者有權限的所有命名空間
	if len(allowedNs) > 0 {
		return "", true // 表示需要遍歷多個命名空間
	}

	return "", false
}

// NamespacePermissionInfo 命名空間權限資訊
type NamespacePermissionInfo struct {
	HasAllAccess      bool     // 是否有全部命名空間權限
	AllowedNamespaces []string // 允許的命名空間列表
	RequestedNs       string   // 請求的命名空間
	HasAccess         bool     // 是否有權限訪問
}

// CheckNamespacePermission 檢查命名空間權限
// 返回權限資訊和是否應該繼續處理
func CheckNamespacePermission(c *gin.Context, requestedNs string) (*NamespacePermissionInfo, bool) {
	info := &NamespacePermissionInfo{
		RequestedNs: requestedNs,
	}

	allowedNs, hasAll := GetAllowedNamespaces(c)
	info.HasAllAccess = hasAll
	info.AllowedNamespaces = allowedNs

	// 如果使用者指定了命名空間，檢查權限
	if requestedNs != "" {
		if hasAll || HasNamespaceAccess(c, requestedNs) {
			info.HasAccess = true
			return info, true
		}
		info.HasAccess = false
		return info, false // 無權訪問
	}

	// 沒有指定命名空間
	info.HasAccess = true
	return info, true
}

// FilterResourcesByNamespace 通用的命名空間過濾函式
// 用於過濾任何包含 Namespace 欄位的資源列表
// getNamespace: 從資源物件中獲取命名空間的函式
func FilterResourcesByNamespace[T any](c *gin.Context, resources []T, getNamespace func(T) string) []T {
	allowedNs, hasAll := GetAllowedNamespaces(c)

	// 如果有全部權限，直接返回
	if hasAll {
		return resources
	}

	// 過濾資源
	filtered := make([]T, 0, len(resources))
	for _, r := range resources {
		ns := getNamespace(r)
		if matchNamespace(ns, allowedNs) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// matchNamespace 檢查命名空間是否匹配權限列表
func matchNamespace(namespace string, allowedNamespaces []string) bool {
	for _, ns := range allowedNamespaces {
		if ns == "*" || ns == namespace {
			return true
		}
		// 萬用字元匹配
		if len(ns) > 1 && ns[len(ns)-1] == '*' {
			prefix := ns[:len(ns)-1]
			if len(namespace) >= len(prefix) && namespace[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

// namespaceForbiddenMsg 產生包含可存取命名空間提示的 403 錯誤訊息
func namespaceForbiddenMsg(requested string, allowed []string) string {
	hint := strings.Join(allowed, ", ")
	if len(allowed) == 0 || (len(allowed) == 1 && allowed[0] == "") {
		hint = "（無，請聯絡管理員設定權限）"
	}
	return fmt.Sprintf("無權存取命名空間 %q；您可存取的命名空間：%s", requested, hint)
}

// ForbiddenNS 回傳帶命名空間存取提示的 403 回應
// 供 handler 在呼叫 CheckNamespacePermission 後使用，取代硬編碼錯誤訊息
func ForbiddenNS(c *gin.Context, info *NamespacePermissionInfo) {
	response.Forbidden(c, namespaceForbiddenMsg(info.RequestedNs, info.AllowedNamespaces))
}
