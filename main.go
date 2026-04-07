package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/shaia/Synapse/cmd/admin"
	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/database"
	"github.com/shaia/Synapse/internal/router"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/crypto"
	"github.com/shaia/Synapse/pkg/logger"
)

//go:embed all:ui/dist
var staticFS embed.FS

// buildKeyProvider 依設定選擇並初始化 KeyProvider（P3-1 可插拔 KMS 介面）。
// 優先順序：key_provider.type > encryption_key_file > encryption_key
func buildKeyProvider(cfg *config.Config) crypto.KeyProvider {
	sec := cfg.Security
	kp := sec.KeyProvider

	switch kp.Type {
	case "vault":
		logger.Info("金鑰來源：HashiCorp Vault", "addr", kp.VaultAddr, "path", kp.VaultSecretPath)
		return crypto.NewVaultKeyProvider(
			kp.VaultAddr, kp.VaultToken,
			kp.VaultSecretPath, kp.VaultSecretField,
			kp.VaultTLSSkip,
		)
	case "aws_secretsmanager":
		logger.Fatal("AWS Secrets Manager provider 尚未啟用，請參考 docs/security/kms-providers.md 的啟用步驟")
		return nil // unreachable
	case "file":
		logger.Info("金鑰來源：檔案", "path", sec.EncryptionKeyFile)
		return crypto.NewFileKeyProvider(sec.EncryptionKeyFile)
	case "env", "":
		// 自動偵測：有 key file 優先，否則用 inline key
		if sec.EncryptionKeyFile != "" {
			logger.Info("金鑰來源：檔案（自動偵測）", "path", sec.EncryptionKeyFile)
			return crypto.NewFileKeyProvider(sec.EncryptionKeyFile)
		}
		return crypto.NewEnvKeyProvider(sec.EncryptionKey)
	default:
		logger.Fatal("不支援的 KEY_PROVIDER_TYPE: %s（支援: env | file | vault | aws_secretsmanager）", kp.Type)
		return nil // unreachable
	}
}

func main() {
	// admin subcommand 攔截：synapse admin <subcommand>
	if len(os.Args) >= 2 && os.Args[1] == "admin" {
		rootCmd := &cobra.Command{
			Use:           "synapse",
			SilenceUsage:  true,
			SilenceErrors: true,
		}
		adminCmd := &cobra.Command{Use: "admin", Short: "管理員工具"}
		adminCmd.AddCommand(admin.NewRotateKeyCmd())
		rootCmd.AddCommand(adminCmd)
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, "錯誤:", err)
			os.Exit(1)
		}
		return
	}

	// 初始化配置
	cfg := config.Load()

	// 初始化日誌
	logger.Init(cfg.Log.Level)

	// 初始化欄位加密：透過可插拔 KeyProvider 取得金鑰（P3-1）
	provider := buildKeyProvider(cfg)
	encKey, err := provider.GetKey(context.Background())
	if err != nil {
		encKey = "" // provider 回傳錯誤視同未設定金鑰
		logger.Warn("KeyProvider 取得金鑰失敗: %v", err)
	}
	crypto.Init(encKey)
	if crypto.IsEnabled() {
		logger.Info("欄位加密已啟用（AES-256-GCM + HKDF-SHA256）")
	} else if cfg.App.Env == "development" {
		logger.Warn("【開發模式】ENCRYPTION_KEY 未設定，叢集憑證將以明文儲存（禁止用於正式環境）")
	} else {
		logger.Fatal("ENCRYPTION_KEY 未設定。正式環境必須設定 ENCRYPTION_KEY 或 ENCRYPTION_KEY_FILE，拒絕啟動。如需在開發環境停用加密，請設定 APP_ENV=development")
	}

	// 初始化 K8s TLS 策略（P0-4）
	services.InitTLSPolicy(cfg.Security.K8sTLSPolicy)

	// 初始化資料庫連線
	db, err := database.Init(cfg.Database)
	if err != nil {
		logger.Fatal("資料庫初始化失敗: %v", err)
	}

	// 設定 Gin 模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化路由
	r, k8sMgr := router.Setup(db, cfg, staticFS)

	// 建立 HTTP 伺服器
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 啟動伺服器
	go func() {
		logger.Info("伺服器啟動於連接埠: %d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("伺服器啟動失敗: %v", err)
		}
	}()

	// 等待中斷訊號以優雅地關閉伺服器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("正在關閉伺服器...")

	// 設定 5 秒逾時後強制關閉伺服器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("伺服器強制關閉: %v", err)
	}

	// 關閉 K8s Informer 管理器
	k8sMgr.Stop()
	logger.Info("K8s Informer 管理器已關閉")

	// 關閉資料庫連線
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
		logger.Info("資料庫連線已關閉")
	}

	logger.Info("伺服器已退出")
}
