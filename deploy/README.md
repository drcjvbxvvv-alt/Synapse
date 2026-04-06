# Synapse 部署指南

本目錄包含 Synapse 的輔助部署設定檔。

> **注意**：`Dockerfile` 與 `docker-compose.yaml` 已置於專案根目錄，方便直接使用。

## 📁 目錄結構

```
專案根目錄/
├── Dockerfile                 # 多階段建置（前後端合一，單一二進位）
├── docker-compose.yaml        # Docker Compose 編排檔
├── .env.example               # 環境變數範本
└── deploy/
    ├── docker/
    │   ├── grafana/           # Grafana 設定
    │   │   ├── dashboards/    # 預置 Dashboard JSON
    │   │   ├── provisioning/  # 自動佈建設定
    │   │   └── secrets/       # API Key 等密鑰
    │   └── mysql/             # MySQL 設定（選用）
    │       ├── conf/          # MySQL 設定檔
    │       └── init/          # 初始化 SQL 腳本
    └── helm/                  # Kubernetes Helm Chart
        └── kubepolaris/       # Chart 主目錄（chart name: synapse）
```

## 🚀 快速開始

### 最快體驗（單一指令）

```bash
docker run --rm -p 8080:8080 registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/synapse:latest
# 瀏覽器開啟 http://localhost:8080
# 預設帳號：admin / Synapse@2026
```

> 內建 SQLite，無需外部依賴。正式環境建議使用 Docker Compose + MySQL。

### Docker Compose 部署

```bash
# 1. 複製專案
git clone https://github.com/clay-wangzhi/Synapse.git
cd Synapse

# 2. 設定環境變數
cp .env.example .env
vim .env

# 3. 啟動所有服務
docker compose up -d

# 4. 查看日誌
docker compose logs -f

# 5. 停止服務
docker compose down
```

## 📦 映像說明

| 映像 | 用途 | 連接埠 |
|------|------|--------|
| `synapse` | 一體化映像（前端透過 go:embed 嵌入後端） | 8080 |

## 🔧 環境變數

主要環境變數（在 `.env` 中設定）：

| 變數 | 說明 | 預設值 |
|------|------|--------|
| `MYSQL_ROOT_PASSWORD` | MySQL root 密碼 | — |
| `MYSQL_PASSWORD` | MySQL 應用密碼 | — |
| `JWT_SECRET` | JWT 簽名密鑰 | — |
| `GRAFANA_ADMIN_PASSWORD` | Grafana 管理員密碼 | — |
| `APP_PORT` | 應用對外連接埠 | `80` |
| `SERVER_MODE` | 執行模式（debug/release） | `release` |

## 📊 服務存取

- **Synapse**：http://localhost（預設連接埠 80）
- **Grafana**：http://localhost:3000

## 📝 注意事項

1. **正式環境**
   - 建議使用外部資料庫
   - 設定 SSL/TLS 憑證
   - 使用強密碼（16 字元以上）

2. **Grafana 資料來源**
   - 需設定外部 Prometheus 位址
   - 修改 `deploy/docker/grafana/provisioning/datasources/prometheus.yaml`

3. **Kubernetes 叢集存取**
   - 掛載 kubeconfig 或使用 ServiceAccount
