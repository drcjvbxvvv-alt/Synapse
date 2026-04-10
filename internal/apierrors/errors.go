// Package apierrors defines structured application errors that carry an HTTP
// status code and a machine-readable error code.  Handlers convert these into
// a consistent JSON error response; the frontend can use the code for i18n.
package apierrors

import (
	"errors"
	"net/http"
)

// AppError is a structured error carrying an HTTP status code and a
// machine-readable code string that the frontend can use for i18n.
type AppError struct {
	Code       string
	HTTPStatus int
	Message    string
}

func (e *AppError) Error() string { return e.Message }

// As is a convenience wrapper around errors.As for AppError.
func As(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// ---- Error code constants ----

const (
	// Auth — machine-readable error codes sent to the frontend; not credentials.
	CodeAuthInvalidCredentials = "AUTH_INVALID_CREDENTIALS" // #nosec G101 -- error code string, not a secret
	CodeAuthAccountDisabled    = "AUTH_ACCOUNT_DISABLED"
	CodeAuthUnsupportedType    = "AUTH_UNSUPPORTED_TYPE"
	CodeAuthTokenFailed        = "AUTH_TOKEN_FAILED"        // #nosec G101 -- error code string
	CodeAuthWrongPassword      = "AUTH_WRONG_PASSWORD"      // #nosec G101 -- error code string
	CodeAuthLDAPReadonly       = "AUTH_LDAP_READONLY"
	CodeAuthLDAPNotEnabled     = "AUTH_LDAP_NOT_ENABLED"
	CodeAuthLDAPFailed         = "AUTH_LDAP_FAILED"

	// User
	CodeUserNotFound          = "USER_NOT_FOUND"
	CodeUserDuplicateUsername = "USER_DUPLICATE_USERNAME"
	CodeUserAdminProtected    = "USER_ADMIN_PROTECTED"
	CodeUserInvalidStatus     = "USER_INVALID_STATUS"

	// Group
	CodeGroupNotFound       = "GROUP_NOT_FOUND"
	CodeGroupHasPermissions = "GROUP_HAS_PERMISSIONS"

	// Permission
	CodePermissionNotFound           = "PERMISSION_NOT_FOUND"
	CodePermissionDuplicate          = "PERMISSION_DUPLICATE"
	CodePermissionInvalidType        = "PERMISSION_INVALID_TYPE"
	CodePermissionCustomRoleRequired = "PERMISSION_CUSTOM_ROLE_REQUIRED"
	CodePermissionAmbiguousTarget    = "PERMISSION_AMBIGUOUS_TARGET"

	// Cluster
	CodeClusterNotFound      = "CLUSTER_NOT_FOUND"
	CodeClusterDuplicateName = "CLUSTER_DUPLICATE_NAME"

	// Generic
	CodeBadRequest         = "BAD_REQUEST"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeInternalError      = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// ---- Constructor functions ----

// Auth errors

func ErrAuthInvalidCredentials() *AppError {
	return &AppError{Code: CodeAuthInvalidCredentials, HTTPStatus: http.StatusUnauthorized, Message: "使用者名稱或密碼錯誤"}
}

func ErrAuthAccountDisabled() *AppError {
	return &AppError{Code: CodeAuthAccountDisabled, HTTPStatus: http.StatusForbidden, Message: "使用者賬號已被禁用"}
}

func ErrAuthUnsupportedType() *AppError {
	return &AppError{Code: CodeAuthUnsupportedType, HTTPStatus: http.StatusBadRequest, Message: "不支援的認證型別"}
}

func ErrAuthTokenFailed() *AppError {
	return &AppError{Code: CodeAuthTokenFailed, HTTPStatus: http.StatusInternalServerError, Message: "JWT token生成失敗"}
}

func ErrAuthWrongPassword() *AppError {
	return &AppError{Code: CodeAuthWrongPassword, HTTPStatus: http.StatusUnauthorized, Message: "原密碼錯誤"}
}

func ErrAuthLDAPReadonly() *AppError {
	return &AppError{Code: CodeAuthLDAPReadonly, HTTPStatus: http.StatusForbidden, Message: "LDAP使用者不能在此修改密碼"}
}

func ErrAuthLDAPNotEnabled() *AppError {
	return &AppError{Code: CodeAuthLDAPNotEnabled, HTTPStatus: http.StatusBadRequest, Message: "LDAP認證未啟用"}
}

// User errors

func ErrUserNotFound() *AppError {
	return &AppError{Code: CodeUserNotFound, HTTPStatus: http.StatusNotFound, Message: "使用者不存在"}
}

func ErrUserDuplicateUsername() *AppError {
	return &AppError{Code: CodeUserDuplicateUsername, HTTPStatus: http.StatusConflict, Message: "使用者名稱已存在"}
}

func ErrUserAdminProtected() *AppError {
	return &AppError{Code: CodeUserAdminProtected, HTTPStatus: http.StatusForbidden, Message: "不能刪除或停用平台管理員"}
}

func ErrUserInvalidStatus() *AppError {
	return &AppError{Code: CodeUserInvalidStatus, HTTPStatus: http.StatusBadRequest, Message: "無效的狀態值"}
}

// Group errors

func ErrGroupNotFound() *AppError {
	return &AppError{Code: CodeGroupNotFound, HTTPStatus: http.StatusNotFound, Message: "使用者組不存在"}
}

func ErrGroupDuplicateName() *AppError {
	return &AppError{Code: "GROUP_DUPLICATE_NAME", HTTPStatus: http.StatusConflict, Message: "使用者組名稱已存在"}
}

func ErrGroupHasPermissions() *AppError {
	return &AppError{Code: CodeGroupHasPermissions, HTTPStatus: http.StatusConflict, Message: "該使用者組還有關聯的權限配置，請先刪除相關權限"}
}

// Permission errors

func ErrPermissionDuplicate() *AppError {
	return &AppError{Code: CodePermissionDuplicate, HTTPStatus: http.StatusConflict, Message: "已有權限配置"}
}

func ErrPermissionInvalidType() *AppError {
	return &AppError{Code: CodePermissionInvalidType, HTTPStatus: http.StatusBadRequest, Message: "無效的權限型別"}
}

func ErrPermissionCustomRoleRequired() *AppError {
	return &AppError{Code: CodePermissionCustomRoleRequired, HTTPStatus: http.StatusBadRequest, Message: "自定義權限必須指定ClusterRole"}
}

func ErrPermissionAmbiguousTarget() *AppError {
	return &AppError{Code: CodePermissionAmbiguousTarget, HTTPStatus: http.StatusBadRequest, Message: "不能同時指定使用者和使用者組"}
}

func ErrPermissionNotFound() *AppError {
	return &AppError{Code: CodePermissionNotFound, HTTPStatus: http.StatusNotFound, Message: "權限配置不存在"}
}

// Cluster errors

func ErrClusterNotFound() *AppError {
	return &AppError{Code: CodeClusterNotFound, HTTPStatus: http.StatusNotFound, Message: "叢集不存在"}
}

func ErrClusterDuplicateName() *AppError {
	return &AppError{Code: CodeClusterDuplicateName, HTTPStatus: http.StatusConflict, Message: "叢集名稱已存在"}
}

// Generic errors

func ErrInternal(msg string) *AppError {
	return &AppError{Code: CodeInternalError, HTTPStatus: http.StatusInternalServerError, Message: msg}
}

func ErrBadRequest(msg string) *AppError {
	return &AppError{Code: CodeBadRequest, HTTPStatus: http.StatusBadRequest, Message: msg}
}

func ErrForbidden(msg string) *AppError {
	return &AppError{Code: CodeForbidden, HTTPStatus: http.StatusForbidden, Message: msg}
}

func ErrUnauthorized(msg string) *AppError {
	return &AppError{Code: CodeUnauthorized, HTTPStatus: http.StatusUnauthorized, Message: msg}
}
