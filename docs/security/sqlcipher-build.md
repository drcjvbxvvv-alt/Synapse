# SQLCipher Build Guide

SQLCipher 為 SQLite 提供 AES-256 at-rest 加密，需 CGO 和額外依賴。
預設 build（`//go:build !sqlcipher`）使用標準 SQLite，無 at-rest 加密。

---

## 前置條件

### 1. 安裝 SQLCipher 開發函式庫

```bash
# Ubuntu/Debian
apt-get install libsqlcipher-dev

# macOS
brew install sqlcipher

# RHEL/CentOS
yum install sqlcipher-devel
```

### 2. 加入 Go 依賴

```bash
go get github.com/nicowillis/go-sqlcipher/v4
```

### 3. 啟用 sqlite_cipher.go 的 import

編輯 `internal/database/sqlite_cipher.go`，取消 import 行的注釋：

```go
// 將此行的注釋移除：
_ "github.com/nicowillis/go-sqlcipher/v4"
```

---

## 編譯

```bash
CGO_ENABLED=1 go build -tags sqlcipher -o synapse .
```

Docker 多階段建置範例：

```dockerfile
FROM golang:1.22-bookworm AS builder
RUN apt-get update && apt-get install -y libsqlcipher-dev gcc
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -tags sqlcipher -o synapse .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y libsqlcipher0 && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/synapse /usr/local/bin/synapse
```

---

## 設定

```bash
# 資料庫通行短語（獨立於 ENCRYPTION_KEY，用於 SQLite 檔案加密）
export DB_PASSPHRASE=<your-strong-passphrase>
```

```yaml
# config.yaml（無需額外設定，DB_PASSPHRASE 環境變數即可）
database:
  driver: sqlite
  dsn: ./data/synapse.db
```

> **注意**：`DB_PASSPHRASE` 與 `ENCRYPTION_KEY` 是兩個獨立的金鑰：
> - `ENCRYPTION_KEY`：用於 AES-256-GCM 欄位加密（kubeconfig、token 等）
> - `DB_PASSPHRASE`：用於 SQLCipher at-rest 加密（整個 SQLite 檔案）

---

## 驗證

```bash
# 驗證已使用 SQLCipher 開啟資料庫
sqlite3 ./data/synapse.db .tables
# 預期輸出：Error: file is not a database（代表已加密）

# 使用 sqlcipher 工具驗證
sqlcipher ./data/synapse.db
sqlite> PRAGMA key='your-passphrase';
sqlite> .tables
# 正確輸出資料表列表
```

---

## 從未加密遷移至 SQLCipher

```bash
# 停止服務
systemctl stop synapse

# 使用 sqlcipher 遷移現有資料庫
sqlcipher ./data/synapse_encrypted.db
sqlite> ATTACH DATABASE './data/synapse.db' AS plaintext KEY '';
sqlite> SELECT sqlcipher_export('main');
sqlite> DETACH DATABASE plaintext;
sqlite> PRAGMA key='your-passphrase';

# 替換原始資料庫
mv ./data/synapse.db ./data/synapse.db.bak
mv ./data/synapse_encrypted.db ./data/synapse.db

# 以 sqlcipher build 重啟
DB_PASSPHRASE=your-passphrase systemctl start synapse
```
