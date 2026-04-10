package models

// SystemRole 系統層級角色 — 不綁定叢集，用於判定使用者在整個平台的身分
// 與 ClusterPermission 是正交的：平台管理員自動獲得所有叢集的 admin 權限，
// 而一般 user 的叢集權限完全由 ClusterPermission 決定。
const (
	// RoleUser 一般使用者（預設值）
	// 只能訪問被明確授權的叢集/命名空間
	RoleUser = "user"

	// RolePlatformAdmin 平台管理員
	// 可管理所有叢集、使用者、權限、系統設定，但不能直接改動系統級配置
	// （例如加密金鑰、資料庫）。對所有叢集自動擁有 admin 權限型別
	RolePlatformAdmin = "platform_admin"

	// RoleSystemAdmin 系統管理員（保留給未來使用）
	// 擴展至可管理加密金鑰輪替、審計策略、備份還原等平台基礎設施層操作
	RoleSystemAdmin = "system_admin"
)

// IsValidSystemRole 驗證傳入字串是否為有效的系統角色
func IsValidSystemRole(role string) bool {
	switch role {
	case RoleUser, RolePlatformAdmin, RoleSystemAdmin:
		return true
	default:
		return false
	}
}

// IsPlatformAdmin 判斷使用者是否具備平台管理員或系統管理員角色
// 提供統一判斷入口，中介軟體與服務層不應再以字串比對 Username
func (u *User) IsPlatformAdmin() bool {
	return u.SystemRole == RolePlatformAdmin || u.SystemRole == RoleSystemAdmin
}
