package config

import (
	"github.com/shaia/Synapse/pkg/logger"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 應用配置結構
type Config struct {
	App             AppConfig             `mapstructure:"app"`
	Server          ServerConfig          `mapstructure:"server"`
	Database        DatabaseConfig        `mapstructure:"database"`
	JWT             JWTConfig             `mapstructure:"jwt"`
	Log             LogConfig             `mapstructure:"log"`
	K8s             K8sConfig             `mapstructure:"k8s"`
	Security        SecurityConfig        `mapstructure:"security"`
	Observability   ObservabilityConfig   `mapstructure:"observability"`
	Redis           RedisConfig           `mapstructure:"redis"`
	RateLimiter     RateLimiterConfig     `mapstructure:"rate_limiter"`
	Tracing         TracingConfig         `mapstructure:"tracing"`
}

// TracingConfig holds OpenTelemetry distributed tracing settings.
type TracingConfig struct {
	// Enabled toggles tracing. Set OTEL_ENABLED=true to activate.
	// Defaults to false — no spans are emitted until explicitly enabled.
	Enabled bool `mapstructure:"enabled"`

	// Endpoint is the OTLP gRPC receiver address (host:port).
	// Examples: "jaeger:4317", "otel-collector:4317"
	// Set via OTEL_EXPORTER_OTLP_ENDPOINT.
	Endpoint string `mapstructure:"endpoint"`

	// ServiceName identifies this service in traces.
	// Set via OTEL_SERVICE_NAME (default: "synapse").
	ServiceName string `mapstructure:"service_name"`

	// ServiceVersion is embedded in the OTel resource.
	// Set via OTEL_SERVICE_VERSION (default: "dev").
	ServiceVersion string `mapstructure:"service_version"`

	// SamplingRate controls trace sampling (0.0 = never, 1.0 = always).
	// Set via OTEL_SAMPLING_RATE (default: 1.0 when tracing is enabled).
	SamplingRate float64 `mapstructure:"sampling_rate"`
}

// RedisConfig holds connection settings for the optional Redis backend.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`     // host:port, e.g. "localhost:6379"
	Password string `mapstructure:"password"` // empty = no auth
	DB       int    `mapstructure:"db"`       // Redis logical DB index (0–15)
}

// RateLimiterConfig selects the rate-limiter backend.
type RateLimiterConfig struct {
	// Backend selects the implementation: "memory" (default) or "redis".
	// Set RATE_LIMITER_BACKEND=redis to enable cross-pod rate limiting.
	Backend string `mapstructure:"backend"`
}

// AppConfig 應用執行環境配置
type AppConfig struct {
	Env string `mapstructure:"env"` // production | development（預設 production）
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
	Driver     string `mapstructure:"driver"`
	DSN        string `mapstructure:"dsn"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Database   string `mapstructure:"database"`
	Charset    string `mapstructure:"charset"`
	TLSEnabled bool   `mapstructure:"tls_enabled"` // MySQL TLS（env: DB_TLS_ENABLED）
	TLSCACert  string `mapstructure:"tls_ca_cert"` // CA 憑證路徑（env: DB_TLS_CA_CERT）
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
	EncryptionKey     string            `mapstructure:"encryption_key"`      // 直接提供金鑰（env: ENCRYPTION_KEY）
	EncryptionKeyFile string            `mapstructure:"encryption_key_file"` // 從檔案讀取金鑰（env: ENCRYPTION_KEY_FILE）
	K8sTLSPolicy      string            `mapstructure:"k8s_tls_policy"`      // strict | warn | skip（預設 warn）
	KeyProvider       KeyProviderConfig `mapstructure:"key_provider"`        // P3-1 可插拔 KMS 介面
}

// KeyProviderConfig 決定加密金鑰的來源
type KeyProviderConfig struct {
	// Type 選擇 provider：env | file | vault | aws_secretsmanager
	// 當 Type 為空時，自動依 EncryptionKey / EncryptionKeyFile 退回 env/file 模式
	Type string `mapstructure:"type"` // env: KEY_PROVIDER_TYPE

	// Vault KV v2（type=vault）
	VaultAddr        string `mapstructure:"vault_addr"`         // env: VAULT_ADDR
	VaultToken       string `mapstructure:"vault_token"`        // env: VAULT_TOKEN
	VaultSecretPath  string `mapstructure:"vault_secret_path"`  // e.g., secret/data/synapse/keys
	VaultSecretField string `mapstructure:"vault_secret_field"` // e.g., encryption_key
	VaultTLSSkip     bool   `mapstructure:"vault_tls_skip"`     // env: VAULT_TLS_SKIP

	// AWS Secrets Manager（type=aws_secretsmanager）
	// 需要啟用 pkg/crypto/provider_aws.go，詳見 docs/security/kms-providers.md
	AWSRegion      string `mapstructure:"aws_region"`       // env: AWS_REGION
	AWSSecretName  string `mapstructure:"aws_secret_name"`  // e.g., synapse/encryption-key
	AWSSecretField string `mapstructure:"aws_secret_field"` // e.g., value
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
	_ = viper.BindEnv("database.tls_enabled", "DB_TLS_ENABLED")
	_ = viper.BindEnv("database.tls_ca_cert", "DB_TLS_CA_CERT")

	// 繫結 JWT 環境變數
	_ = viper.BindEnv("jwt.secret", "JWT_SECRET")
	_ = viper.BindEnv("jwt.expire_time", "JWT_EXPIRE_TIME")

	// 繫結日誌環境變數
	_ = viper.BindEnv("log.level", "LOG_LEVEL")
	_ = viper.BindEnv("log.format", "LOG_FORMAT")

	// 繫結 K8s 環境變數
	_ = viper.BindEnv("k8s.default_namespace", "K8S_DEFAULT_NAMESPACE")
	_ = viper.BindEnv("k8s.informer_sync_timeout", "INFORMER_SYNC_TIMEOUT")

	// 繫結應用環境變數
	_ = viper.BindEnv("app.env", "APP_ENV")

	// 繫結安全環境變數
	_ = viper.BindEnv("security.encryption_key", "ENCRYPTION_KEY")
	_ = viper.BindEnv("security.encryption_key_file", "ENCRYPTION_KEY_FILE")
	_ = viper.BindEnv("security.k8s_tls_policy", "K8S_TLS_POLICY")

	// 繫結 KeyProvider 環境變數
	_ = viper.BindEnv("security.key_provider.type", "KEY_PROVIDER_TYPE")
	_ = viper.BindEnv("security.key_provider.vault_addr", "VAULT_ADDR")
	_ = viper.BindEnv("security.key_provider.vault_token", "VAULT_TOKEN")
	_ = viper.BindEnv("security.key_provider.vault_secret_path", "VAULT_SECRET_PATH")
	_ = viper.BindEnv("security.key_provider.vault_secret_field", "VAULT_SECRET_FIELD")
	_ = viper.BindEnv("security.key_provider.vault_tls_skip", "VAULT_TLS_SKIP")
	_ = viper.BindEnv("security.key_provider.aws_region", "AWS_REGION")
	_ = viper.BindEnv("security.key_provider.aws_secret_name", "AWS_SECRET_NAME")
	_ = viper.BindEnv("security.key_provider.aws_secret_field", "AWS_SECRET_FIELD")

	// 繫結可觀測性環境變數
	_ = viper.BindEnv("observability.enabled", "OBSERVABILITY_ENABLED")
	_ = viper.BindEnv("observability.metrics_path", "METRICS_PATH")
	_ = viper.BindEnv("observability.metrics_token", "METRICS_TOKEN")
	_ = viper.BindEnv("observability.health_path", "HEALTH_PATH")
	_ = viper.BindEnv("observability.ready_path", "READY_PATH")

	// 繫結 Redis 環境變數
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")

	// 繫結 Rate Limiter 環境變數
	_ = viper.BindEnv("rate_limiter.backend", "RATE_LIMITER_BACKEND")

	// 繫結 Tracing 環境變數
	_ = viper.BindEnv("tracing.enabled", "OTEL_ENABLED")
	_ = viper.BindEnv("tracing.endpoint", "OTEL_EXPORTER_OTLP_ENDPOINT")
	_ = viper.BindEnv("tracing.service_name", "OTEL_SERVICE_NAME")
	_ = viper.BindEnv("tracing.service_version", "OTEL_SERVICE_VERSION")
	_ = viper.BindEnv("tracing.sampling_rate", "OTEL_SAMPLING_RATE")

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

	// 資料庫預設配置（SQLite 已棄用，生產環境強制使用 MySQL）
	viper.SetDefault("database.driver", "mysql")
	viper.SetDefault("database.dsn", "")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.username", "root")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.database", "synapse")
	viper.SetDefault("database.charset", "utf8mb4")
	viper.SetDefault("database.tls_enabled", false)
	viper.SetDefault("database.tls_ca_cert", "")

	// JWT預設配置
	viper.SetDefault("jwt.secret", "synapse-secret")
	viper.SetDefault("jwt.expire_time", 24) // 24小時

	// 日誌預設配置
	viper.SetDefault("log.level", "info")

	// K8s預設配置
	viper.SetDefault("k8s.default_namespace", "default")
	viper.SetDefault("k8s.informer_sync_timeout", 30) // 30 seconds

	// 應用環境預設配置
	viper.SetDefault("app.env", "production")

	// 安全預設配置
	viper.SetDefault("security.encryption_key", "")
	viper.SetDefault("security.encryption_key_file", "")
	viper.SetDefault("security.k8s_tls_policy", "warn")
	viper.SetDefault("security.key_provider.type", "")
	viper.SetDefault("security.key_provider.vault_secret_field", "encryption_key")
	viper.SetDefault("security.key_provider.aws_secret_field", "value")

	// 可觀測性預設配置
	viper.SetDefault("observability.enabled", true)
	viper.SetDefault("observability.metrics_path", "/metrics")
	viper.SetDefault("observability.metrics_token", "")
	viper.SetDefault("observability.health_path", "/healthz")
	viper.SetDefault("observability.ready_path", "/readyz")

	// Redis 預設配置
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Rate Limiter 預設配置（memory = 單機模式，無需 Redis）
	viper.SetDefault("rate_limiter.backend", "memory")

	// Tracing 預設配置（預設關閉，需明確設 OTEL_ENABLED=true）
	viper.SetDefault("tracing.enabled", false)
	viper.SetDefault("tracing.endpoint", "")
	viper.SetDefault("tracing.service_name", "synapse")
	viper.SetDefault("tracing.service_version", "dev")
	viper.SetDefault("tracing.sampling_rate", 1.0)
}
