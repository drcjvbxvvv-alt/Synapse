package database

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	gomysql "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// currentDriver 儲存當前使用的資料庫驅動型別
var currentDriver string

// GetCurrentDriver 返回當前使用的資料庫驅動型別
func GetCurrentDriver() string {
	return currentDriver
}

// Init 初始化資料庫連線
// 支援 MySQL 和 SQLite 兩種資料庫驅動
func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	// 配置 GORM 日誌：只記錄慢查詢與錯誤，避免 AutoMigrate 時大量 I/O
	gormConfig := &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Warn),
	}

	var db *gorm.DB
	var err error

	// 根據配置的驅動型別選擇資料庫
	driver := cfg.Driver
	if driver == "" {
		driver = "sqlite" // 預設使用 SQLite
	}
	currentDriver = driver

	switch driver {
	case "sqlite":
		db, err = initSQLite(cfg, gormConfig)
	case "mysql":
		db, err = initMySQL(cfg, gormConfig)
	default:
		return nil, fmt.Errorf("不支援的資料庫驅動: %s，請使用 'sqlite' 或 'mysql'", driver)
	}

	if err != nil {
		return nil, err
	}

	// 獲取底層的 sql.DB 物件來配置連線池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("獲取資料庫連線失敗: %w", err)
	}

	// 設定連線池參數
	if driver == "mysql" {
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		// SQLite 使用單連線模式以避免鎖衝突
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	// 執行資料庫遷移
	// MySQL（生產環境）：使用 golang-migrate 執行版本化 SQL 遷移檔案。
	// SQLite（開發環境）：使用 GORM AutoMigrate（無生產風險）。
	if driver == "mysql" {
		if err := RunMigrations(db, driver); err != nil {
			return nil, fmt.Errorf("資料庫遷移失敗: %w", err)
		}
	} else {
		if err := autoMigrate(db); err != nil {
			return nil, fmt.Errorf("資料庫遷移失敗: %w", err)
		}
	}

	// 初始化種子資料（兩種驅動都需要執行）
	if driver == "mysql" {
		runSeeds(db)
	}

	logger.Info("資料庫連線成功 (驅動: %s)", driver)
	return db, nil
}

// initSQLite 初始化 SQLite 資料庫連線
func initSQLite(cfg config.DatabaseConfig, gormConfig *gorm.Config) (*gorm.DB, error) {
	// 獲取資料庫檔案路徑
	dbPath := cfg.DSN
	if dbPath == "" {
		dbPath = "./data/synapse.db"
	}

	// 確保目錄存在，限制為 owner-only 存取
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("建立資料庫目錄失敗: %w", err)
		}
	}

	logger.Info("連線 SQLite 資料庫: %s", dbPath)

	// SQLite 連線參數：啟用 WAL 模式提升併發效能，啟用外來鍵約束
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=on", dbPath)

	// DB_PASSPHRASE 供 sqlcipher build tag 使用（plain build 忽略此值）
	passphrase := os.Getenv("DB_PASSPHRASE")
	db, err := openSQLiteDB(dsn, passphrase, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("連線 SQLite 資料庫失敗: %w", err)
	}

	// 限制 DB 檔案為 owner-only 可讀寫（600），防止同主機其他程序直接讀取
	if err := os.Chmod(dbPath, 0600); err != nil {
		logger.Warn("無法設定資料庫檔案權限，請手動執行 chmod 600 %s：%v", dbPath, err)
	}

	return db, nil
}

// initMySQL 初始化 MySQL 資料庫連線（支援 TLS）
func initMySQL(cfg config.DatabaseConfig, gormConfig *gorm.Config) (*gorm.DB, error) {
	// 若啟用 TLS，向 go-sql-driver 註冊自訂 TLS 設定
	tlsParam := ""
	if cfg.TLSEnabled {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if cfg.TLSCACert != "" {
			caCert, err := os.ReadFile(cfg.TLSCACert)
			if err != nil {
				return nil, fmt.Errorf("讀取 MySQL TLS CA 憑證失敗 (%s): %w", cfg.TLSCACert, err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("MySQL TLS CA 憑證格式無效: %s", cfg.TLSCACert)
			}
			tlsCfg.RootCAs = pool
		} else {
			// 無 CA 時仍啟用 TLS，但不驗證伺服器憑證（加密但無驗證）
			logger.Warn("MySQL TLS 已啟用但未提供 CA 憑證，伺服器身分不受驗證", "host", cfg.Host)
			tlsCfg.InsecureSkipVerify = true //nolint:gosec
		}
		if err := gomysql.RegisterTLSConfig("synapse", tlsCfg); err != nil {
			return nil, fmt.Errorf("註冊 MySQL TLS 設定失敗: %w", err)
		}
		tlsParam = "&tls=synapse"
		logger.Info("MySQL TLS 已啟用", "host", cfg.Host)
	}

	// 先連線到 MySQL 伺服器（不指定資料庫）以建立資料庫
	dsnWithoutDB := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=True&loc=Local%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Charset, tlsParam)

	logger.Info("連線MySQL伺服器: %s@%s:%d", cfg.Username, cfg.Host, cfg.Port)
	tempDB, err := gorm.Open(mysql.Open(dsnWithoutDB), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("連線MySQL伺服器失敗: %w", err)
	}

	// 建立資料庫（如果不存在）
	createDBSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", cfg.Database)
	if err := tempDB.Exec(createDBSQL).Error; err != nil {
		return nil, fmt.Errorf("建立資料庫失敗: %w", err)
	}
	logger.Info("資料庫 %s 建立成功或已存在", cfg.Database)

	// 連線到具體的資料庫
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.Charset, tlsParam)

	logger.Info("連線MySQL資料庫: %s@%s:%d/%s", cfg.Username, cfg.Host, cfg.Port, cfg.Database)
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("連線資料庫失敗: %w", err)
	}

	return db, nil
}

// autoMigrate 使用 GORM AutoMigrate 建立/更新資料庫表。
// 僅用於 SQLite 開發環境。MySQL 生產環境改用 RunMigrations（golang-migrate）。
func autoMigrate(db *gorm.DB) error {
	db.Exec("PRAGMA foreign_keys = OFF")

	// 按依賴順序遷移表
	err := db.AutoMigrate(
		&models.User{},
		&models.Cluster{},
		&models.ClusterMetrics{},
		&models.TerminalSession{},
		&models.TerminalCommand{},
		&models.AuditLog{},
		&models.OperationLog{},
		&models.SystemSetting{},
		&models.ArgoCDConfig{},
		&models.UserGroup{},
		&models.UserGroupMember{},
		&models.ClusterPermission{},
		&models.AIConfig{},
		&models.HelmRepository{},
		&models.EventAlertRule{},
		&models.EventAlertHistory{},
		&models.CostConfig{},
		&models.ResourceSnapshot{},
		&models.ClusterOccupancySnapshot{},
		&models.CloudBillingConfig{},
		&models.CloudBillingRecord{},
		&models.ImageScanResult{},
		&models.BenchResult{},
		&models.SyncPolicy{},
		&models.SyncHistory{},
		&models.ConfigVersion{},
		&models.LogSourceConfig{},
		&models.NamespaceProtection{},
		&models.ApprovalRequest{},
		&models.ImageIndex{},
		&models.PortForwardSession{},
		&models.SIEMWebhookConfig{},
		&models.APIToken{},
		&models.NotifyChannel{},
		&models.TokenBlacklist{},
		&models.FeatureFlag{},
	)

	db.Exec("PRAGMA foreign_keys = ON")

	if err == nil {
		runSeeds(db)
	}
	return err
}

// runSeeds 建立預設管理員、系統設定等初始資料（idempotent — 已存在則跳過）。
// 在 autoMigrate（SQLite）和 RunMigrations（MySQL）之後都需呼叫。
func runSeeds(db *gorm.DB) {
	backfillSystemRole(db)
	createDefaultUser(db)
	createTestClusters(db)
	createDefaultSystemSettings(db)
	createDefaultPermissions(db)
}

// backfillSystemRole 將既有的 username=admin 使用者升級為 platform_admin
// 此為 P0-2 遷移邏輯：為移除硬編碼 "username == admin" 判斷做準備
// 一次性操作，後續啟動會是 no-op（因為 SystemRole 已非預設值）
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

// createDefaultPermissions 建立預設權限配置
func createDefaultPermissions(db *gorm.DB) {
	// 檢查是否已有權限配置
	var count int64
	db.Model(&models.ClusterPermission{}).Count(&count)
	if count > 0 {
		return
	}

	// 獲取管理員使用者
	var adminUser models.User
	if err := db.Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		logger.Error("未找到管理員使用者，跳過權限配置: %v", err)
		return
	}

	// 獲取所有叢集
	var clusters []models.Cluster
	if err := db.Find(&clusters).Error; err != nil {
		logger.Error("獲取叢集列表失敗: %v", err)
		return
	}

	// 為管理員使用者在所有叢集建立管理員權限
	for _, cluster := range clusters {
		permission := &models.ClusterPermission{
			ClusterID:      cluster.ID,
			UserID:         &adminUser.ID,
			PermissionType: models.PermissionTypeAdmin,
			Namespaces:     `["*"]`,
		}

		if err := db.Create(permission).Error; err != nil {
			logger.Error("建立叢集權限失敗: cluster=%s, error=%v", cluster.Name, err)
		} else {
			logger.Info("建立預設管理員權限: user=%s, cluster=%s", adminUser.Username, cluster.Name)
		}
	}

	// 建立預設使用者組
	defaultGroups := []models.UserGroup{
		{Name: "運維組", Description: "運維團隊成員，擁有運維權限"},
		{Name: "開發組", Description: "開發團隊成員，擁有開發權限"},
		{Name: "只讀組", Description: "只讀權限使用者組"},
	}

	for _, group := range defaultGroups {
		var existing models.UserGroup
		result := db.Where("name = ?", group.Name).First(&existing)
		if result.Error == nil {
			continue // 已存在，跳過
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
	// 先查，只有不存在才做 bcrypt（避免每次啟動都跑 ~100ms 的 hash 運算）
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

	// 優先從環境變數讀取初始密碼，未設定則使用內建預設值並警告
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
		// 建立預設LDAP配置
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

// createTestClusters 建立測試叢集資料
func createTestClusters(db *gorm.DB) {
	var count int64
	db.Model(&models.Cluster{}).Count(&count)
	if count == 0 {
		// 建立測試叢集
		testClusters := []*models.Cluster{
			{
				Name:      "dev-cluster",
				APIServer: "https://dev-k8s-api.example.com:6443",
				Version:   "v1.28.2",
				Status:    "healthy",
				Labels:    `{"env":"dev","team":"backend"}`,
				KubeconfigEnc: `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://dev-k8s-api.example.com:6443
    insecure-skip-tls-verify: true
  name: dev-cluster
contexts:
- context:
    cluster: dev-cluster
    user: dev-user
  name: dev-context
current-context: dev-context
users:
- name: dev-user
  user:
    token: fake-token-for-testing`,
			},
			{
				Name:      "prod-cluster",
				APIServer: "https://prod-k8s-api.example.com:6443",
				Version:   "v1.28.1",
				Status:    "healthy",
				Labels:    `{"env":"prod","team":"ops"}`,
				KubeconfigEnc: `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://prod-k8s-api.example.com:6443
    insecure-skip-tls-verify: true
  name: prod-cluster
contexts:
- context:
    cluster: prod-cluster
    user: prod-user
  name: prod-context
current-context: prod-context
users:
- name: prod-user
  user:
    token: fake-token-for-testing`,
			},
			{
				Name:      "test-cluster",
				APIServer: "https://test-k8s-api.example.com:6443",
				Version:   "v1.27.8",
				Status:    "unhealthy",
				Labels:    `{"env":"test","team":"qa"}`,
				KubeconfigEnc: `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-k8s-api.example.com:6443
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: fake-token-for-testing`,
			},
		}

		for _, cluster := range testClusters {
			if err := db.Create(cluster).Error; err != nil {
				logger.Error("建立測試叢集失敗: %v", err)
			} else {
				logger.Info("測試叢集建立成功: %s", cluster.Name)
			}
		}
	}
}
