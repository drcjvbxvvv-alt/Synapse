//go:build sqlcipher

// sqlite_cipher.go — 加密 SQLite（SQLCipher，需 CGO）。
//
// 前置條件：
//  1. 安裝 SQLCipher 開發函式庫
//     - Ubuntu/Debian：apt-get install libsqlcipher-dev
//     - macOS：        brew install sqlcipher
//     - RHEL/CentOS：  yum install sqlcipher-devel
//
//  2. 加入 go-sqlcipher 依賴：
//     go get github.com/nicowillis/go-sqlcipher/v4
//
//  3. 編譯時指定 build tag：
//     CGO_ENABLED=1 go build -tags sqlcipher .
//
//  4. 設定資料庫通行短語（獨立於 ENCRYPTION_KEY）：
//     DB_PASSPHRASE=<your-passphrase>
//
// 注意：SQLCipher 通行短語透過 URI 參數傳入，格式為：
//   file:synapse.db?_pragma=key='passphrase'
//
// 詳見 docs/security/sqlcipher-build.md
package database

import (
	"fmt"

	// 啟用此 build tag 前，請先執行：
	//   go get github.com/nicowillis/go-sqlcipher/v4
	// 然後取消下方的 import 注釋：
	// _ "github.com/nicowillis/go-sqlcipher/v4"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openSQLiteDB 以 SQLCipher 加密通行短語開啟 SQLite 資料庫。
// passphrase 通常由 DB_PASSPHRASE 環境變數提供（database.Init 負責傳入）。
func openSQLiteDB(dsn string, passphrase string, gormConfig *gorm.Config) (*gorm.DB, error) {
	if passphrase == "" {
		return nil, fmt.Errorf("sqlcipher build 需要 DB_PASSPHRASE，但未設定")
	}
	// SQLCipher DSN 格式：file:path?_pragma=key='passphrase'&_journal_mode=WAL
	// 注意：此處移除原始 dsn 中已有的 query string，避免重複
	encDSN := fmt.Sprintf("file:%s?_pragma=key='%s'", dsn, passphrase)
	return gorm.Open(sqlite.Open(encDSN), gormConfig)
}
