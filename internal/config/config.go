package config

import (
	"github.com/clay-wangzhi/KubePolaris/pkg/logger"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	K8s      K8sConfig      `mapstructure:"k8s"`
	Grafana  GrafanaConfig  `mapstructure:"grafana"`
}

// GrafanaConfig Grafana 功能开关（URL 和 API Key 已迁移到系统设置配置中心）
type GrafanaConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig 数据库配置
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

// LogConfig 日志配置
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// K8sConfig Kubernetes配置
type K8sConfig struct {
	DefaultNamespace string `mapstructure:"default_namespace"`
}

// Load 加载配置（纯环境变量模式）
func Load() *Config {
	// 设置默认值
	setDefaults()

	// 先加载 .env 到系统环境变量
	if err := godotenv.Load(); err != nil {
		logger.Info("未找到 .env 文件，使用系统环境变量: %v", err)
	}

	// 读取环境变量
	viper.AutomaticEnv()

	// 绑定服务器环境变量
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.mode", "SERVER_MODE")

	// 绑定数据库环境变量
	_ = viper.BindEnv("database.driver", "DB_DRIVER")
	_ = viper.BindEnv("database.dsn", "DB_DSN")
	_ = viper.BindEnv("database.host", "DB_HOST")
	_ = viper.BindEnv("database.port", "DB_PORT")
	_ = viper.BindEnv("database.username", "DB_USERNAME")
	_ = viper.BindEnv("database.password", "DB_PASSWORD")
	_ = viper.BindEnv("database.database", "DB_DATABASE")
	_ = viper.BindEnv("database.charset", "DB_CHARSET")

	// 绑定 JWT 环境变量
	_ = viper.BindEnv("jwt.secret", "JWT_SECRET")
	_ = viper.BindEnv("jwt.expire_time", "JWT_EXPIRE_TIME")

	// 绑定日志环境变量
	_ = viper.BindEnv("log.level", "LOG_LEVEL")

	// 绑定 K8s 环境变量
	_ = viper.BindEnv("k8s.default_namespace", "K8S_DEFAULT_NAMESPACE")

	// 绑定 Grafana 环境变量（仅保留功能开关，连接配置已迁移到配置中心）
	_ = viper.BindEnv("grafana.enabled", "GRAFANA_ENABLED")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		logger.Fatal("配置解析失败: %v", err)
	}
	logger.Info("配置: %+v", config)

	return &config
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 服务器默认配置
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")

	// 数据库默认配置
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.dsn", "./data/kubepolaris.db")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.username", "root")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.database", "kubepolaris")
	viper.SetDefault("database.charset", "utf8mb4")

	// JWT默认配置
	viper.SetDefault("jwt.secret", "kubepolaris-secret")
	viper.SetDefault("jwt.expire_time", 24) // 24小时

	// 日志默认配置
	viper.SetDefault("log.level", "info")

	// K8s默认配置
	viper.SetDefault("k8s.default_namespace", "default")

	// Grafana 默认配置（仅功能开关）
	viper.SetDefault("grafana.enabled", false)
}
