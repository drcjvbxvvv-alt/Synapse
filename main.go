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

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/database"
	"github.com/shaia/Synapse/internal/router"
	"github.com/shaia/Synapse/pkg/crypto"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
)

//go:embed all:ui/dist
var staticFS embed.FS

func main() {
	// 初始化配置
	cfg := config.Load()

	// 初始化日志
	logger.Init(cfg.Log.Level)

	// 初始化欄位加密（若未設定 ENCRYPTION_KEY 則靜默跳過）
	crypto.Init(cfg.Security.EncryptionKey)
	if crypto.IsEnabled() {
		logger.Info("欄位加密已啟用（AES-256-GCM）")
	} else {
		logger.Warn("安全提示: ENCRYPTION_KEY 未設定，叢集憑證將以明文儲存於資料庫")
	}

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
