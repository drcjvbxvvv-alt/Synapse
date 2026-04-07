//go:build !sqlcipher

// sqlite_plain.go — 標準 SQLite（pure-Go，無 CGO，無加密）。
// 若需 SQLite at-rest 加密，請改用 sqlcipher build tag：
//
//	CGO_ENABLED=1 go build -tags sqlcipher .
//
// 詳見 docs/security/sqlcipher-build.md
package database

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// openSQLiteDB 開啟標準（未加密）SQLite 資料庫。
func openSQLiteDB(dsn string, _ string, gormConfig *gorm.Config) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(dsn), gormConfig)
}
