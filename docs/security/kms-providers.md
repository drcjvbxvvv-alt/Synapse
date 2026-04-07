# Synapse KMS Provider 設定指南

Synapse 的加密金鑰來源由 `KeyProvider` 介面抽象，支援以下四種模式：

| Provider | 適用場景 | 需要外部依賴 |
|----------|---------|------------|
| `env` | 開發、小型部署 | 無 |
| `file` | VM 生產環境（搭配 systemd LoadCredential） | 無 |
| `vault` | 企業內部，已有 HashiCorp Vault | 無（direct HTTP） |
| `aws_secretsmanager` | 雲端部署（AWS） | 需啟用 build tag |

---

## 1. Env Provider（預設）

最簡單，適合本地開發。金鑰直接寫在環境變數或 `.env` 檔。

```bash
# 生成金鑰
openssl rand -hex 32

# 設定
export ENCRYPTION_KEY=<your-hex-key>
# 或 .env 檔案
ENCRYPTION_KEY=<your-hex-key>
```

> ⚠️ **不建議用於生產環境**：`/proc/<pid>/environ` 可被同主機有權限的程序讀取。

---

## 2. File Provider

金鑰存於獨立檔案，搭配 `systemd LoadCredential` 可達到更好的隔離性。

```bash
# 生成並保護金鑰檔案
openssl rand -hex 32 > /etc/synapse/secrets/encryption.key
chmod 400 /etc/synapse/secrets/encryption.key
chown root:root /etc/synapse/secrets/encryption.key
```

```yaml
# config.yaml
security:
  key_provider:
    type: file
  encryption_key_file: /etc/synapse/secrets/encryption.key
```

或透過環境變數：

```bash
export KEY_PROVIDER_TYPE=file
export ENCRYPTION_KEY_FILE=/etc/synapse/secrets/encryption.key
```

搭配 systemd LoadCredential 的設定詳見 `docs/deploy/vm-observability.md`。

---

## 3. HashiCorp Vault Provider

使用 Vault KV v2 secrets engine 儲存金鑰，Synapse 啟動時透過 HTTP API 取得。

### Vault 設定

```bash
# 1. 啟用 KV v2 secret engine（若尚未啟用）
vault secrets enable -path=secret kv-v2

# 2. 儲存加密金鑰
vault kv put secret/synapse/keys encryption_key=$(openssl rand -hex 32)

# 3. 建立最小權限 Policy
cat > /tmp/synapse-policy.hcl << 'EOF'
path "secret/data/synapse/keys" {
  capabilities = ["read"]
}
EOF
vault policy write synapse-key-reader /tmp/synapse-policy.hcl

# 4. 建立 Token（或使用 AppRole，見下方）
vault token create -policy=synapse-key-reader -ttl=8760h
```

### Synapse 設定

```yaml
# config.yaml
security:
  key_provider:
    type: vault
    vault_addr: "https://vault.example.com"      # 或 env: VAULT_ADDR
    vault_token: "hvs.xxxxx"                     # 或 env: VAULT_TOKEN（建議）
    vault_secret_path: "secret/data/synapse/keys"
    vault_secret_field: "encryption_key"
    vault_tls_skip: false                        # 生產環境保持 false
```

```bash
# 建議以環境變數傳入 token，避免寫入設定檔
export VAULT_ADDR=https://vault.example.com
export VAULT_TOKEN=hvs.xxxxx
export KEY_PROVIDER_TYPE=vault
export VAULT_SECRET_PATH=secret/data/synapse/keys
export VAULT_SECRET_FIELD=encryption_key
```

### 驗證

```bash
# 手動測試 API
curl -H "X-Vault-Token: $VAULT_TOKEN" \
     $VAULT_ADDR/v1/secret/data/synapse/keys | jq '.data.data'
```

---

## 4. AWS Secrets Manager Provider

> **目前為 build-tag 選項**，需要額外步驟啟用。

### 啟用步驟

```bash
# 1. 加入 AWS SDK v2 依賴
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/secretsmanager

# 2. 編輯 pkg/crypto/provider_aws.go：
#    - 移除第一行的 `//go:build ignore`
#    - 取消 import 的注釋，加入實際實作：
```

`pkg/crypto/provider_aws.go` 參考實作：

```go
//go:build aws_secretsmanager

package crypto

import (
    "context"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type AWSSecretsManagerProvider struct {
    region     string
    secretName string
    field      string
}

func NewAWSSecretsManagerProvider(region, secretName, field string) *AWSSecretsManagerProvider {
    return &AWSSecretsManagerProvider{region: region, secretName: secretName, field: field}
}

func (p *AWSSecretsManagerProvider) GetKey(ctx context.Context) (string, error) {
    cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(p.region))
    if err != nil {
        return "", fmt.Errorf("AWSSecretsManagerProvider: 載入 AWS 設定失敗: %w", err)
    }
    client := secretsmanager.NewFromConfig(cfg)
    result, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(p.secretName),
    })
    if err != nil {
        return "", fmt.Errorf("AWSSecretsManagerProvider: 取得 secret 失敗: %w", err)
    }
    if result.SecretString == nil {
        return "", fmt.Errorf("AWSSecretsManagerProvider: secret 值為空")
    }
    // 若 secret 為 JSON 格式，解析指定欄位
    var data map[string]string
    if err := json.Unmarshal([]byte(*result.SecretString), &data); err == nil {
        if key, ok := data[p.field]; ok {
            return key, nil
        }
        return "", fmt.Errorf("AWSSecretsManagerProvider: secret 中找不到欄位 %q", p.field)
    }
    // 純字串 secret
    return *result.SecretString, nil
}
```

```bash
# 3. 編譯時加入 build tag
CGO_ENABLED=0 go build -tags aws_secretsmanager -o synapse .
```

### AWS 設定

```yaml
security:
  key_provider:
    type: aws_secretsmanager
    aws_region: us-east-1
    aws_secret_name: synapse/encryption-key
    aws_secret_field: value
```

```bash
# IAM Policy（最小權限）
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["secretsmanager:GetSecretValue"],
    "Resource": "arn:aws:secretsmanager:us-east-1:*:secret:synapse/encryption-key*"
  }]
}
```

---

## 5. 金鑰輪換

無論使用哪種 provider，更換金鑰後需重新加密資料庫：

```bash
# 停止服務
systemctl stop synapse

# 輪換金鑰（新金鑰從任意 provider 取得）
ENCRYPTION_KEY=<OLD_KEY> ./synapse admin rotate-key --new-key <NEW_KEY>
# 或從檔案
ENCRYPTION_KEY=<OLD_KEY> ./synapse admin rotate-key --new-key-file /path/to/new.key

# 更新金鑰來源（env / file / vault）
# 重啟服務
systemctl start synapse
```
