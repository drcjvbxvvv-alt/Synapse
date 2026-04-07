package config

import (
	"github.com/shaia/Synapse/pkg/logger"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 應用配置結構
type Config struct {
	Server          ServerConfig          `mapstructure:"server"`
	Database        DatabaseConfig        `mapstructure:"database"`
	JWT             JWTConfig             `mapstructure:"jwt"`
	Log             LogConfig             `mapstructure:"log"`
	K8s             K8sConfig             `mapstructure:"k8s"`
	Security        SecurityConfig        `mapstructure:"security"`
	Observability   ObservabilityConfig   `mapstructure:"observability"`
}

// ObservabilityConfig 可觀測性配置
type ObservabilityConfig struct {
	Enabled      bool   `mapstructure:"enabled"`       // false = 完全關閉
	MetricsPath  string `mapstructure:"metrics_path"`  // 預設 /metrics
	MetricsToken string `mapstructure:"metrics_token"` // 空 = 不驗證
	HealthPath   string `mapstructure:"health_path"`   // 預設 /healthz
	ReadyPath    string `mapstructure:"ready_path"`    // 預設 /readyz
}

// ServerConfig 伺服器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig 資料庫配置
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	DSN      string `mapstructure:"dsn"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	Charset  string `mapstructure:"charset"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	ExpireTime int    `mapstructure:"expire_time"`
}

// LogConfig 日誌配置
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // text | json（預設 text，由 LOG_FORMAT env 控制）
}

// K8sConfig Kubernetes配置
type K8sConfig struct {
	DefaultNamespace   string `mapstructure:"default_namespace"`
	InformerSyncTimeout int   `mapstructure:"informer_sync_timeout"` // seconds
}

// SecurityConfig 安全相關配置
type SecurityConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

// Load 載入配置（純環境變數模式）
func Load() *Config {
	// 設定預設值
	setDefaults()

	// 先載入 .env 到系統環境變數
	if err := godotenv.Load(); err != nil {
		logger.Info("未找到 .env 檔案，使用系統環境變數: %v", err)
	}

	// 讀取環境變數
	viper.AutomaticEnv()

	// 繫結伺服器環境變數
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.mode", "SERVER_MODE")

	// 繫結資料庫環境變數
	_ = viper.BindEnv("database.driver", "DB_DRIVER")
	_ = viper.BindEnv("database.dsn", "DB_DSN")
	_ = viper.BindEnv("database.host", "DB_HOST")
	_ = viper.BindEnv("database.port", "DB_PORT")
	_ = viper.BindEnv("database.username", "DB_USERNAME")
	_ = viper.BindEnv("database.password", "DB_PASSWORD")
	_ = viper.BindEnv("database.database", "DB_DATABASE")
	_ = viper.BindEnv("database.charset", "DB_CHARSET")

	// 繫結 JWT 環境變數
	_ = viper.BindEnv("jwt.secret", "JWT_SECRET")
	_ = viper.BindEnv("jwt.expire_time", "JWT_EXPIRE_TIME")

	// 繫結日誌環境變數
	_ = viper.BindEnv("log.level", "LOG_LEVEL")
	_ = viper.BindEnv("log.format", "LOG_FORMAT")

	// 繫結 K8s 環境變數
	_ = viper.BindEnv("k8s.default_namespace", "K8S_DEFAULT_NAMESPACE")
	_ = viper.BindEnv("k8s.informer_sync_timeout", "INFORMER_SYNC_TIMEOUT")

	// 繫結安全環境變數
	_ = viper.BindEnv("security.encryption_key", "ENCRYPTION_KEY")

	// 繫結可觀測性環境變數
	_ = viper.BindEnv("observability.enabled", "OBSERVABILITY_ENABLED")
	_ = viper.BindEnv("observability.metrics_path", "METRICS_PATH")
	_ = viper.BindEnv("observability.metrics_token", "METRICS_TOKEN")
	_ = viper.BindEnv("observability.health_path", "HEALTH_PATH")
	_ = viper.BindEnv("observability.ready_path", "READY_PATH")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		logger.Fatal("配置解析失敗: %v", err)
	}

	// 安全檢查：JWT Secret 預設值警告
	if config.JWT.Secret == "synapse-secret" {
		if config.Server.Mode == "release" {
			logger.Fatal("安全風險: 生產環境必須設定 JWT_SECRET 環境變數，不能使用預設值")
		} else {
			logger.Warn("安全警告: JWT_SECRET 使用預設值，請在生產環境中設定自定義金鑰")
		}
	}

	logger.Info("配置載入完成: server.port=%d, server.mode=%s, db.driver=%s, log.level=%s",
		config.Server.Port, config.Server.Mode, config.Database.Driver, config.Log.Level)

	return &config
}

// setDefaults 設定預設配置值
func setDefaults() {
	// 伺服器預設配置
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")

	// 資料庫預設配置
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.dsn", "./data/synapse.db")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.username", "root")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.database", "synapse")
	viper.SetDefault("database.charset", "utf8mb4")

	// JWT預設配置
	viper.SetDefault("jwt.secret", "synapse-secret")
	viper.SetDefault("jwt.expire_time", 24) // 24小時

	// 日誌預設配置
	viper.SetDefault("log.level", "info")

	// K8s預設配置
	viper.SetDefault("k8s.default_namespace", "default")
	viper.SetDefault("k8s.informer_sync_timeout", 30) // 30 seconds

	// 安全預設配置（空字串表示禁用加密）
	viper.SetDefault("security.encryption_key", "")

	// 可觀測性預設配置
	viper.SetDefault("observability.enabled", true)
	viper.SetDefault("observability.metrics_path", "/metrics")
	viper.SetDefault("observability.metrics_token", "")
	viper.SetDefault("observability.health_path", "/healthz")
	viper.SetDefault("observability.ready_path", "/readyz")
}
