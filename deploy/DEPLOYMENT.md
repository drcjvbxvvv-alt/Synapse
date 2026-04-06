# Synapse 部署操作手冊

## 📦 部署方式

Synapse 支援三種部署方式：

| 方式 | 適用場景 | 複雜度 |
|------|----------|--------|
| **Docker（單容器）** | 快速體驗、CI 環境 | ⭐ |
| **Docker Compose** | 開發 / 測試 / 小型正式環境 | ⭐⭐ |
| **Kubernetes Helm** | 正式環境、高可用 | ⭐⭐⭐ |

---

## ☸️ Kubernetes Helm 部署（推薦正式環境）

### 方式一：透過 Helm 倉庫安裝（推薦）

```bash
# 1. 新增 Helm 倉庫
helm repo add synapse https://clay-wangzhi.github.io/Synapse
helm repo update

# 2. 查看可用版本
helm search repo synapse

# 3. 安裝（使用預設設定）
helm install synapse synapse/synapse \
  -n synapse --create-namespace

# 4. 或自訂設定安裝
helm install synapse synapse/synapse \
  -n synapse --create-namespace \
  --set mysql.auth.rootPassword=your-root-password \
  --set mysql.auth.password=your-password \
  --set backend.config.jwt.secret=your-jwt-secret

# 5. 查看安裝狀態
helm status synapse -n synapse
kubectl get pods -n synapse
```

### 方式二：從原始碼安裝

```bash
# 1. 複製專案
git clone https://github.com/clay-wangzhi/Synapse.git
cd Synapse

# 2. 安裝
helm install synapse ./deploy/helm/kubepolaris \
  -n synapse --create-namespace \
  -f ./deploy/helm/kubepolaris/values.yaml
```

### Helm 常用設定參數

詳細設定請參考 [Helm Chart README](./helm/kubepolaris/README.md)

| 參數 | 說明 | 預設值 |
|------|------|--------|
| `mysql.auth.rootPassword` | MySQL root 密碼 | `synapse-root` |
| `mysql.auth.password` | 應用資料庫密碼 | `synapse123` |
| `backend.config.jwt.secret` | JWT 密鑰 | 隨機產生 |
| `ingress.enabled` | 是否啟用 Ingress | `true` |
| `ingress.hosts[0].host` | 網域名稱 | `synapse.local` |
| `grafana.enabled` | 是否啟用內建 Grafana | `true` |

### 升級與卸載

```bash
# 升級
helm repo update
helm upgrade synapse synapse/synapse -n synapse

# 回滾
helm rollback synapse 1 -n synapse

# 卸載
helm uninstall synapse -n synapse
```

---

## 🐳 Docker Compose 部署（開發 / 測試）

### 前置需求

- Docker 20.10+
- Docker Compose V2（docker compose plugin）
- 至少 4 GB 可用記憶體
- 至少 10 GB 可用磁碟空間

### 快速開始

```bash
# 1. 複製專案
git clone https://github.com/clay-wangzhi/Synapse.git
cd Synapse

# 2. 設定環境變數
cp .env.example .env
vim .env   # 修改密碼等設定

# 3. 啟動所有服務
docker compose up -d

# 4. 查看狀態
docker compose ps
```

### 服務存取

啟動完成後：

- **Synapse**：http://localhost
  - 預設帳號：`admin` / `Synapse@2026`
- **Grafana**：http://localhost:3000
  - 預設帳號：`admin` / 查看 `.env` 中的 `GRAFANA_ADMIN_PASSWORD`

---

## 📋 環境變數說明（.env）

| 變數名稱 | 說明 | 預設值 | 必填 |
|----------|------|--------|------|
| `MYSQL_ROOT_PASSWORD` | MySQL root 密碼 | — | ✅ |
| `MYSQL_PASSWORD` | 應用資料庫密碼 | — | ✅ |
| `JWT_SECRET` | JWT 簽名密鑰 | — | ✅ |
| `GRAFANA_ADMIN_PASSWORD` | Grafana 管理員密碼 | — | ✅ |
| `MYSQL_PORT` | MySQL 連接埠 | `3306` | ❌ |
| `APP_PORT` | 應用對外連接埠 | `80` | ❌ |
| `GRAFANA_PORT` | Grafana 連接埠 | `3000` | ❌ |
| `SERVER_MODE` | 執行模式（debug/release） | `release` | ❌ |
| `LOG_LEVEL` | 日誌層級 | `info` | ❌ |
| `VERSION` | 映像版本 | `latest` | ❌ |

---

## 🔒 安全最佳實踐

### 產生強隨機密碼

```bash
# MySQL 密碼（16 字元）
openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16

# JWT Secret（32 字元）
openssl rand -base64 32

# Grafana 密碼（12 字元）
openssl rand -base64 12 | tr -dc 'a-zA-Z0-9' | head -c 12
```

### 檔案權限

```bash
# .env 僅允許擁有者讀寫
chmod 600 .env

# secrets 目錄權限
chmod 700 deploy/docker/grafana/secrets
```

### 正式環境檢查清單

- ✅ 使用強隨機密碼（16 字元以上）
- ✅ 定期輪換密碼與密鑰
- ✅ 啟用 HTTPS / TLS
- ✅ 設定防火牆規則
- ✅ 啟用稽核日誌（Synapse 系統設定 → 安全設定）
- ✅ 定期備份資料
- ✅ 使用 Secrets 管理工具（如 Vault / Sealed Secrets）

---

## 🛠️ 常用操作

### 查看日誌

```bash
# 所有服務
docker compose logs -f

# 指定服務
docker compose logs -f synapse
docker compose logs -f mysql
docker compose logs -f grafana
```

### 重啟服務

```bash
docker compose restart          # 所有服務
docker compose restart synapse  # 指定服務
```

### 停止服務

```bash
docker compose stop             # 停止（保留資料）
docker compose down             # 停止並刪除容器（保留資料卷）
docker compose down -v          # 停止並刪除所有內容（含資料）
```

### 更新服務

```bash
git pull origin main
docker compose up -d --build
docker compose ps
```

### 資料備份

```bash
# 備份 MySQL
docker compose exec mysql mysqldump -u root -p synapse > backup_$(date +%Y%m%d).sql

# 備份 Grafana
docker compose exec grafana tar czf - /var/lib/grafana > grafana-backup.tar.gz
```

### 資料還原

```bash
# 還原 MySQL
docker compose exec -T mysql mysql -u root -p synapse < backup.sql

# 還原 Grafana
docker compose exec -T grafana tar xzf - -C / < grafana-backup.tar.gz
docker compose restart grafana
```

---

## 🐛 故障排查

### 服務無法啟動

```bash
# 查看 Docker 狀態
docker info
docker compose ps

# 查看錯誤日誌
docker compose logs synapse
docker compose logs mysql
```

**常見原因**：
1. **連接埠衝突**：修改 `.env` 中的連接埠設定
2. **記憶體不足**：確保至少 4 GB 可用記憶體
3. **磁碟空間不足**：清理 Docker 快取 `docker system prune -a`

### MySQL 連線失敗

```bash
# 確認 MySQL 狀態
docker compose exec mysql mysqladmin ping -h localhost

# 重置 MySQL（⚠️ 會清除所有資料）
docker compose down
docker volume rm synapse-mysql-data
docker compose up -d mysql
```

### Grafana API Key 問題

```bash
# 確認 API Key 檔案
ls -la deploy/docker/grafana/secrets/grafana_api_key

# 重新初始化
docker compose up -d grafana-init
docker compose logs grafana-init
```

---

## 📊 監控與維運

### 健康檢查

```bash
docker compose ps
curl http://localhost/healthz          # Synapse
curl http://localhost:3000/api/health  # Grafana
```

### 資源監控

```bash
docker stats       # 容器資源使用
docker system df   # 磁碟使用
```

---

## 🔄 版本升級

```bash
# 1. 備份資料
docker compose exec mysql mysqldump -u root -p synapse > backup_$(date +%Y%m%d).sql

# 2. 拉取最新程式碼
git pull origin main

# 3. 重新建置並啟動
docker compose up -d --build

# 4. 驗證服務
docker compose ps
curl http://localhost/healthz
```

---

## 📚 相關文件

- [環境變數設定範本](../.env.example)
- [Helm Chart 文件](./helm/kubepolaris/README.md)

---

**最後更新**：2026-04-06
**文件版本**：v2.1.0
