package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// Init 初始化資料庫連線（PostgreSQL）
func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	gormConfig := &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Warn),
	}

	dsn := buildDSN(cfg)

	// 自動建立資料庫（若不存在）
	if err := ensureDatabaseExists(cfg); err != nil {
		logger.Warn("無法自動建立資料庫，請確保資料庫已存在", "error", err)
	}

	logger.Info("連線 PostgreSQL 資料庫: %s@%s:%d/%s", cfg.Username, cfg.Host, cfg.Port, cfg.Database)
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("連線 PostgreSQL 資料庫失敗: %w", err)
	}

	// 設定連線池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("獲取資料庫連線失敗: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 執行版本化遷移
	if err := RunMigrations(dsn); err != nil {
		return nil, fmt.Errorf("資料庫遷移失敗: %w", err)
	}

	// 初始化種子資料
	runSeeds(db)

	logger.Info("資料庫連線成功 (PostgreSQL)")
	return db, nil
}

// buildDSN 組裝 PostgreSQL DSN
func buildDSN(cfg config.DatabaseConfig) string {
	if cfg.DSN != "" {
		return cfg.DSN
	}

	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, sslMode)

	if cfg.SSLRootCert != "" {
		dsn += fmt.Sprintf(" sslrootcert=%s", cfg.SSLRootCert)
	}

	return dsn
}

// ensureDatabaseExists 連線至預設 postgres 資料庫，建立目標資料庫（若不存在）
func ensureDatabaseExists(cfg config.DatabaseConfig) error {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	adminDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, sslMode)

	sqlDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return fmt.Errorf("連線 postgres 管理資料庫失敗: %w", err)
	}
	defer sqlDB.Close()

	var exists bool
	err = sqlDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.Database).Scan(&exists)
	if err != nil {
		return fmt.Errorf("查詢資料庫是否存在失敗: %w", err)
	}

	if !exists {
		// 資料庫名稱不能用參數化查詢，但此值來自配置檔而非使用者輸入
		_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %q ENCODING 'UTF8'", cfg.Database))
		if err != nil {
			return fmt.Errorf("建立資料庫失敗: %w", err)
		}
		logger.Info("資料庫 %s 建立成功", cfg.Database)
	}

	return nil
}

// runSeeds 建立預設管理員、系統設定等初始資料（idempotent — 已存在則跳過）。
func runSeeds(db *gorm.DB) {
	backfillSystemRole(db)
	createDefaultUser(db)
	createDefaultSystemSettings(db)
	createDefaultPermissions(db)
}

// backfillSystemRole 將既有的 username=admin 使用者升級為 platform_admin
func backfillSystemRole(db *gorm.DB) {
	result := db.Model(&models.User{}).
		Where("username = ? AND (system_role = ? OR system_role = ?)", "admin", "", models.RoleUser).
		Update("system_role", models.RolePlatformAdmin)
	if result.Error != nil {
		logger.Warn("回填 admin SystemRole 失敗", "error", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		logger.Info("已將既有 admin 使用者升級為 platform_admin", "rows", result.RowsAffected)
	}
}

// createDefaultPermissions 建立預設使用者組
func createDefaultPermissions(db *gorm.DB) {
	defaultGroups := []models.UserGroup{
		{Name: "運維組", Description: "運維團隊成員，擁有運維權限"},
		{Name: "開發組", Description: "開發團隊成員，擁有開發權限"},
		{Name: "只讀組", Description: "只讀權限使用者組"},
	}

	for _, group := range defaultGroups {
		var existing models.UserGroup
		result := db.Where("name = ?", group.Name).First(&existing)
		if result.Error == nil {
			continue
		}
		if err := db.Create(&group).Error; err != nil {
			logger.Error("建立使用者組失敗: %v", err)
		} else {
			logger.Info("建立預設使用者組: %s", group.Name)
		}
	}
}

// createDefaultUser 建立預設管理員使用者
func createDefaultUser(db *gorm.DB) {
	var user models.User
	result := db.Where("username = ?", "admin").First(&user)
	if result.Error == nil {
		logger.Info("預設管理員使用者已存在，跳過建立")
		return
	}
	if result.Error != gorm.ErrRecordNotFound {
		logger.Error("查詢預設使用者失敗: %v", result.Error)
		return
	}

	salt := "synapse_salt"

	initPassword := os.Getenv("SYNAPSE_ADMIN_PASSWORD")
	if initPassword == "" {
		initPassword = "Synapse@2026" // #nosec G101 -- intentional fallback default; set SYNAPSE_ADMIN_PASSWORD env var in production
		logger.Warn("⚠  SYNAPSE_ADMIN_PASSWORD 未設定，使用內建預設密碼（生產環境請設定此環境變數）")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(initPassword+salt), 12)
	if err != nil {
		logger.Error("生成密碼雜湊失敗: %v", err)
		return
	}

	user = models.User{
		Username:     "admin",
		PasswordHash: string(hashedPassword),
		Salt:         salt,
		Email:        "admin@synapse.io",
		DisplayName:  "管理員",
		AuthType:     "local",
		Status:       "active",
		SystemRole:   models.RolePlatformAdmin,
	}
	if err := db.Create(&user).Error; err != nil {
		logger.Error("建立預設使用者失敗: %v", err)
	} else {
		logger.Info("預設管理員使用者建立成功（帳號: admin，密碼來源: %s）",
			func() string {
				if os.Getenv("SYNAPSE_ADMIN_PASSWORD") != "" {
					return "SYNAPSE_ADMIN_PASSWORD 環境變數"
				}
				return "內建預設值"
			}(),
		)
	}
}

// createDefaultSystemSettings 建立預設系統設定
func createDefaultSystemSettings(db *gorm.DB) {
	var count int64
	db.Model(&models.SystemSetting{}).Where("config_key = ?", "ldap_config").Count(&count)
	if count == 0 {
		defaultLDAPConfig := models.GetDefaultLDAPConfig()
		ldapConfigJSON, _ := json.Marshal(defaultLDAPConfig)

		setting := &models.SystemSetting{
			ConfigKey: "ldap_config",
			Value:     string(ldapConfigJSON),
			Type:      "ldap",
		}

		if err := db.Create(setting).Error; err != nil {
			logger.Error("建立預設LDAP配置失敗: %v", err)
		} else {
			logger.Info("預設LDAP配置建立成功")
		}
	}
}

