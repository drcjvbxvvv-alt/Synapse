// Package crypto — KeyProvider 介面與實作。
//
// KeyProvider 讓加密金鑰的來源可插拔：
//   - EnvKeyProvider   直接從環境變數或設定值讀取（預設）
//   - FileKeyProvider  從本地檔案讀取（搭配 systemd LoadCredential）
//   - VaultKeyProvider HashiCorp Vault KV v2（direct HTTP，無 SDK 依賴）
//
// AWS Secrets Manager 支援需額外 build tag，詳見 docs/security/kms-providers.md。
package crypto

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// KeyProvider 是加密金鑰來源的抽象介面。
// GetKey 在應用啟動時被呼叫一次；實作應在 ctx 到期前返回。
type KeyProvider interface {
	GetKey(ctx context.Context) (string, error)
}

// ── EnvKeyProvider ────────────────────────────────────────────────────────────

// EnvKeyProvider 直接使用已在記憶體中的金鑰字串（來自環境變數或設定檔）。
// 這是最簡單的 provider，適用於開發環境或使用 systemd EnvironmentFile 的場景。
type EnvKeyProvider struct {
	key string
}

// NewEnvKeyProvider 以明文金鑰字串建立 provider。
func NewEnvKeyProvider(key string) *EnvKeyProvider {
	return &EnvKeyProvider{key: key}
}

func (p *EnvKeyProvider) GetKey(_ context.Context) (string, error) {
	if p.key == "" {
		return "", fmt.Errorf("EnvKeyProvider: ENCRYPTION_KEY 未設定")
	}
	return p.key, nil
}

// ── FileKeyProvider ──────────────────────────────────────────────────────────

// FileKeyProvider 從本地檔案讀取金鑰，適用於：
//   - systemd LoadCredential（$CREDENTIALS_DIRECTORY/encryption-key）
//   - 權限為 400 的金鑰檔案（/etc/synapse/secrets/encryption.key）
type FileKeyProvider struct {
	path string
}

// NewFileKeyProvider 以金鑰檔案路徑建立 provider。
func NewFileKeyProvider(path string) *FileKeyProvider {
	return &FileKeyProvider{path: path}
}

func (p *FileKeyProvider) GetKey(_ context.Context) (string, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return "", fmt.Errorf("FileKeyProvider: 無法讀取金鑰檔案 %s: %w", p.path, err)
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("FileKeyProvider: 金鑰檔案 %s 內容為空", p.path)
	}
	return key, nil
}

// ── VaultKeyProvider ─────────────────────────────────────────────────────────

// VaultKeyProvider 從 HashiCorp Vault KV v2 secrets engine 讀取加密金鑰。
//
// 所需設定：
//
//	security:
//	  key_provider:
//	    type: vault
//	    vault_addr: "https://vault.example.com"     # env: VAULT_ADDR
//	    vault_token: ""                              # env: VAULT_TOKEN
//	    vault_secret_path: "secret/data/synapse/keys"
//	    vault_secret_field: "encryption_key"
//	    vault_tls_skip: false
//
// Vault 需要：
//  1. 啟用 KV v2 secret engine（`vault secrets enable -path=secret kv-v2`）
//  2. 建立 secret（`vault kv put secret/synapse/keys encryption_key=<hex>`）
//  3. Policy：`path "secret/data/synapse/keys" { capabilities = ["read"] }`
type VaultKeyProvider struct {
	addr       string
	token      string
	secretPath string // KV v2 path，例如 "secret/data/synapse/keys"
	field      string // secret 內的欄位名稱，例如 "encryption_key"
	skipVerify bool
}

// NewVaultKeyProvider 建立 Vault provider。
func NewVaultKeyProvider(addr, token, secretPath, field string, skipVerify bool) *VaultKeyProvider {
	return &VaultKeyProvider{
		addr:       strings.TrimRight(addr, "/"),
		token:      token,
		secretPath: strings.TrimLeft(secretPath, "/"),
		field:      field,
		skipVerify: skipVerify,
	}
}

func (p *VaultKeyProvider) GetKey(ctx context.Context) (string, error) {
	if p.token == "" {
		p.token = os.Getenv("VAULT_TOKEN")
	}
	if p.addr == "" {
		p.addr = strings.TrimRight(os.Getenv("VAULT_ADDR"), "/")
	}
	if p.addr == "" {
		return "", fmt.Errorf("VaultKeyProvider: VAULT_ADDR 未設定")
	}
	if p.token == "" {
		return "", fmt.Errorf("VaultKeyProvider: VAULT_TOKEN 未設定")
	}

	url := p.addr + "/v1/" + p.secretPath

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: p.skipVerify}, // #nosec G402 -- user-controlled via VaultKeyProvider.skipVerify config field
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("VaultKeyProvider: 建立請求失敗: %w", err)
	}
	req.Header.Set("X-Vault-Token", p.token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("VaultKeyProvider: 請求 Vault 失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("VaultKeyProvider: 認證失敗（HTTP %d），請確認 VAULT_TOKEN 有效且有讀取 %s 的權限",
			resp.StatusCode, p.secretPath)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("VaultKeyProvider: Vault 回傳 HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("VaultKeyProvider: 讀取回應失敗: %w", err)
	}

	// KV v2 回應結構：{"data": {"data": {"field": "value"}, "metadata": {...}}}
	var result struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("VaultKeyProvider: 解析 Vault 回應失敗: %w", err)
	}

	key, ok := result.Data.Data[p.field]
	if !ok {
		return "", fmt.Errorf("VaultKeyProvider: secret %s 中找不到欄位 %q（可用欄位: %v）",
			p.secretPath, p.field, mapKeys(result.Data.Data))
	}
	if key == "" {
		return "", fmt.Errorf("VaultKeyProvider: secret %s[%s] 值為空", p.secretPath, p.field)
	}
	return key, nil
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
