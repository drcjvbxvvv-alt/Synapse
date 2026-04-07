// Package admin 提供 Synapse 管理員 CLI 工具。
//
// 用法：
//
//	synapse admin rotate-key --new-key <NEW_KEY>
//	synapse admin rotate-key --new-key-file /path/to/new.key
package admin

import (
	"fmt"
	"os"
	"strings"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/database"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/crypto"
	"github.com/shaia/Synapse/pkg/logger"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// NewRotateKeyCmd 建立 rotate-key 子命令
func NewRotateKeyCmd() *cobra.Command {
	var newKey string
	var newKeyFile string
	var batchSize int

	cmd := &cobra.Command{
		Use:   "rotate-key",
		Short: "以新金鑰重新加密資料庫中所有叢集憑證",
		Long: `rotate-key 讀取所有 Cluster 記錄，用當前金鑰（ENCRYPTION_KEY / ENCRYPTION_KEY_FILE）
解密後，以新金鑰重新加密並寫回資料庫。

金鑰洩漏緊急應對流程：
  1. 停止 Synapse 服務
  2. 執行：synapse admin rotate-key --new-key <NEW_KEY>
  3. 更新環境變數 ENCRYPTION_KEY=<NEW_KEY>
  4. 重啟 Synapse 服務`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRotateKey(newKey, newKeyFile, batchSize)
		},
	}

	cmd.Flags().StringVar(&newKey, "new-key", "", "新加密金鑰（直接傳入，建議僅用於測試）")
	cmd.Flags().StringVar(&newKeyFile, "new-key-file", "", "從檔案讀取新加密金鑰（建議用法）")
	cmd.Flags().IntVar(&batchSize, "batch-size", 50, "每批次處理筆數（失敗時可降低）")

	return cmd
}

func runRotateKey(newKey, newKeyFile string, batchSize int) error {
	// ── 1. 解析新金鑰 ──────────────────────────────────────────────────────
	if newKey == "" && newKeyFile != "" {
		data, err := os.ReadFile(newKeyFile)
		if err != nil {
			return fmt.Errorf("無法讀取新金鑰檔案 %s: %w", newKeyFile, err)
		}
		newKey = strings.TrimSpace(string(data))
	}
	if newKey == "" {
		return fmt.Errorf("必須透過 --new-key 或 --new-key-file 提供新金鑰")
	}

	// ── 2. 解析當前金鑰（舊金鑰） ─────────────────────────────────────────
	cfg := config.Load()
	oldKey := cfg.Security.EncryptionKey
	if oldKey == "" && cfg.Security.EncryptionKeyFile != "" {
		data, err := os.ReadFile(cfg.Security.EncryptionKeyFile)
		if err != nil {
			return fmt.Errorf("無法讀取當前金鑰檔案: %w", err)
		}
		oldKey = strings.TrimSpace(string(data))
	}
	if oldKey == "" {
		return fmt.Errorf("當前 ENCRYPTION_KEY 未設定，無法執行金鑰輪換")
	}
	if oldKey == newKey {
		return fmt.Errorf("新金鑰與當前金鑰相同，無需輪換")
	}

	// ── 3. 初始化舊金鑰解密器 ─────────────────────────────────────────────
	crypto.Init(oldKey)
	if !crypto.IsEnabled() {
		return fmt.Errorf("當前金鑰初始化失敗")
	}

	// ── 4. 建立新金鑰加密器（獨立實例） ───────────────────────────────────
	newCrypto := crypto.NewInstance(newKey)

	// ── 5. 連線資料庫 ─────────────────────────────────────────────────────
	db, err := database.Init(cfg.Database)
	if err != nil {
		return fmt.Errorf("資料庫連線失敗: %w", err)
	}

	// ── 6. 計算總筆數 ─────────────────────────────────────────────────────
	var total int64
	db.Model(&models.Cluster{}).Unscoped().Count(&total)
	if total == 0 {
		fmt.Println("資料庫中沒有叢集記錄，無需輪換。")
		return nil
	}
	fmt.Printf("共 %d 筆叢集記錄，批次大小 %d\n", total, batchSize)

	// ── 7. 分批處理 ───────────────────────────────────────────────────────
	var (
		offset  int
		success int
		failed  int
	)

	for {
		var clusters []models.Cluster
		if err := db.Unscoped().Offset(offset).Limit(batchSize).Find(&clusters).Error; err != nil {
			return fmt.Errorf("讀取叢集記錄失敗（offset=%d）: %w", offset, err)
		}
		if len(clusters) == 0 {
			break
		}

		txErr := db.Transaction(func(tx *gorm.DB) error {
			for i := range clusters {
				c := &clusters[i]
				if err := reencryptCluster(c, newCrypto, tx); err != nil {
					logger.Error("重新加密失敗，跳過", "cluster_id", c.ID, "name", c.Name, "error", err)
					failed++
					continue
				}
				success++
			}
			return nil
		})
		if txErr != nil {
			return fmt.Errorf("批次交易失敗（offset=%d）: %w", offset, txErr)
		}

		offset += batchSize
		fmt.Printf("  進度：%d / %d（失敗：%d）\n", success+failed, total, failed)
	}

	// ── 8. 結果摘要 ───────────────────────────────────────────────────────
	fmt.Printf("\n金鑰輪換完成：成功 %d 筆，失敗 %d 筆\n", success, failed)
	if failed > 0 {
		fmt.Println("⚠️  有失敗記錄，請檢查日誌並重試。在所有記錄成功前，請保留舊金鑰。")
		return fmt.Errorf("部分記錄輪換失敗（%d 筆）", failed)
	}
	fmt.Println("✓  請立即更新 ENCRYPTION_KEY / ENCRYPTION_KEY_FILE 為新金鑰並重啟服務。")
	return nil
}

// reencryptCluster 用新加密器重新加密單筆叢集的敏感欄位，並以 UPDATE 寫回。
// 注意：此函式繞過 GORM BeforeSave hook（hook 使用全域 crypto），直接操作欄位。
func reencryptCluster(c *models.Cluster, newCrypto *crypto.Instance, tx *gorm.DB) error {
	// 解密（全域舊金鑰，由 AfterFind hook 已完成）
	// models.Cluster.AfterFind 在 db.Find 時已自動解密，c 的欄位此時為明文。

	// 用新金鑰加密
	encKubeconfig, err := newCrypto.Encrypt(c.KubeconfigEnc)
	if err != nil {
		return fmt.Errorf("kubeconfig 加密失敗: %w", err)
	}
	encCA, err := newCrypto.Encrypt(c.CAEnc)
	if err != nil {
		return fmt.Errorf("CA cert 加密失敗: %w", err)
	}
	encToken, err := newCrypto.Encrypt(c.SATokenEnc)
	if err != nil {
		return fmt.Errorf("SA token 加密失敗: %w", err)
	}

	// 只更新三個加密欄位，跳過 BeforeSave hook（避免用舊金鑰再次加密）
	return tx.Model(c).Unscoped().UpdateColumns(map[string]interface{}{
		"kubeconfig_enc": encKubeconfig,
		"ca_enc":         encCA,
		"sa_token_enc":   encToken,
	}).Error
}
