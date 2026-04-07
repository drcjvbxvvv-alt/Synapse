package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/glebarez/sqlite"
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

	// 自動遷移資料庫表
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("資料庫遷移失敗: %w", err)
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

	// 確保目錄存在
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("建立資料庫目錄失敗: %w", err)
		}
	}

	logger.Info("連線 SQLite 資料庫: %s", dbPath)

	// SQLite 連線參數：啟用 WAL 模式提升併發效能，啟用外來鍵約束
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("連線 SQLite 資料庫失敗: %w", err)
	}

	return db, nil
}

// initMySQL 初始化 MySQL 資料庫連線
func initMySQL(cfg config.DatabaseConfig, gormConfig *gorm.Config) (*gorm.DB, error) {
	// 先連線到MySQL伺服器（不指定資料庫）來建立資料庫
	dsnWithoutDB := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Charset,
	)

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

	// 現在連線到具體的資料庫
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
	)

	logger.Info("連線MySQL資料庫: %s@%s:%d/%s", cfg.Username, cfg.Host, cfg.Port, cfg.Database)
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("連線資料庫失敗: %w", err)
	}

	return db, nil
}

// autoMigrate 自動遷移資料庫表
func autoMigrate(db *gorm.DB) error {
	// 根據資料庫驅動型別禁用外來鍵約束檢查
	if currentDriver == "mysql" {
		db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	} else if currentDriver == "sqlite" {
		db.Exec("PRAGMA foreign_keys = OFF")
	}

	// 按依賴順序遷移表
	err := db.AutoMigrate(
		&models.User{},
		&models.Cluster{},
		&models.ClusterMetrics{},
		&models.TerminalSession{},
		&models.TerminalCommand{},
		&models.AuditLog{},
		&models.OperationLog{},      // 操作審計日誌表（新增）
		&models.SystemSetting{},     // 系統設定表
		&models.ArgoCDConfig{},      // ArgoCD 配置表
		&models.UserGroup{},         // 使用者組表
		&models.UserGroupMember{},   // 使用者組成員關聯表
		&models.ClusterPermission{}, // 叢集權限表
		&models.AIConfig{},           // AI 配置表
		&models.HelmRepository{},     // Helm Chart 倉庫配置表
		&models.EventAlertRule{},     // K8s Event 告警規則表
		&models.EventAlertHistory{},  // Event 告警歷史紀錄表
		&models.CostConfig{},         // 成本定價設定表
		&models.ResourceSnapshot{},          // 資源每日快照表
		&models.ClusterOccupancySnapshot{},  // 叢集級別佔用快照表（Phase 1）
		&models.CloudBillingConfig{},         // 雲端帳單設定（Phase 4）
		&models.CloudBillingRecord{},         // 雲端帳單記錄（Phase 4）
		&models.ImageScanResult{},    // Trivy 映像掃描結果表
		&models.BenchResult{},        // CIS kube-bench 評分表
		&models.SyncPolicy{},         // 多叢集配置同步策略表
		&models.SyncHistory{},        // 同步歷史紀錄表
		&models.ConfigVersion{},        // ConfigMap/Secret 版本歷史快照表
		&models.LogSourceConfig{},      // 外部日誌源設定表（Loki / Elasticsearch）
		&models.NamespaceProtection{},  // 命名空間保護設定表（審批工作流）
		&models.ApprovalRequest{},      // 部署審批請求表
		&models.ImageIndex{},           // 映像索引表（跨叢集 Image Tag 搜尋）
		&models.PortForwardSession{},   // Port-Forward 會話記錄表
		&models.SIEMWebhookConfig{},    // SIEM Webhook 設定表
		&models.APIToken{},             // 個人 API Token 表
		&models.NotifyChannel{},        // 通知渠道設定表
	)

	// 根據資料庫驅動型別重新啟用外來鍵約束檢查
	if currentDriver == "mysql" {
		db.Exec("SET FOREIGN_KEY_CHECKS = 1")
	} else if currentDriver == "sqlite" {
		db.Exec("PRAGMA foreign_keys = ON")
	}

	// 建立預設管理員使用者和系統設定（如果不存在）
	if err == nil {
		createDefaultUser(db)
		createTestClusters(db)
		createDefaultSystemSettings(db)
		createDefaultPermissions(db)
	}

	return err
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
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Synapse@2026"+salt), bcrypt.DefaultCost)
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
	}
	if err := db.Create(&user).Error; err != nil {
		logger.Error("建立預設使用者失敗: %v", err)
	} else {
		logger.Info("預設管理員使用者建立成功: admin/Synapse@2026")
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
