# Grafana 整合指南

本目錄包含 Synapse 搭配 Grafana 的相關設定檔。

## 📁 目錄結構

```
deploy/docker/grafana/
├── dashboards/           # 預置 Dashboard JSON
│   ├── k8s-cluster-overview.json   # 叢集總覽
│   ├── k8s-workload-detail.json    # 工作負載詳情
│   └── k8s-pod-detail.json         # Pod 詳情
├── provisioning/         # Grafana 自動佈建設定
│   ├── dashboards/
│   │   └── dashboards.yaml         # Dashboard 自動載入設定
│   └── datasources/
│       └── prometheus.yaml         # Prometheus 資料來源設定
└── secrets/              # API Key 等密鑰（不進入版本庫）
    └── grafana_api_key
```

## 🚀 啟動 Grafana

Grafana 透過根目錄的 `docker-compose.yaml` 統一管理：

```bash
# 在專案根目錄執行
docker compose up -d grafana

# 查看日誌
docker compose logs -f grafana
```

## 🔗 存取 Grafana

啟動後開啟瀏覽器：

- **URL**：http://localhost:3000
- **帳號**：`admin`
- **密碼**：查看 `.env` 中的 `GRAFANA_ADMIN_PASSWORD`

## ⚙️ 設定 Prometheus 資料來源

修改 `deploy/docker/grafana/provisioning/datasources/prometheus.yaml`，填入正確的 Prometheus 位址：

```yaml
datasources:
  - name: Prometheus
    type: prometheus
    url: http://your-prometheus:9090   # 改為實際位址
    isDefault: true
```

常見 Prometheus 位址：
- Kubernetes 叢集內：`http://prometheus-server:9090`
- Docker 本機：`http://host.docker.internal:9090`
- 遠端位址：`http://your-prometheus-ip:9090`

修改後重啟 Grafana：

```bash
docker compose restart grafana
```

## 📊 預置 Dashboard

Synapse 內建三個 Dashboard，啟動時自動載入：

| Dashboard | 說明 |
|-----------|------|
| `k8s-cluster-overview.json` | 叢集總覽（CPU / 記憶體 / 節點狀態） |
| `k8s-workload-detail.json` | 工作負載詳情（Deployment / StatefulSet 等） |
| `k8s-pod-detail.json` | Pod 詳情（單一 Pod 監控指標） |

也可從 Grafana 官方社群匯入其他 Dashboard：

| Dashboard ID | 名稱 | 用途 |
|---|---|---|
| **315** | Kubernetes Cluster Monitoring | 叢集概覽 |
| **6417** | Kubernetes Pod Monitoring | Pod 監控 |
| **13770** | Node Exporter Full | 節點監控 |

## 🔑 取得 Grafana API Key

Synapse 的系統設定 → Grafana 設定 需要填入 API Key（Service Account Token）：

1. 登入 Grafana → **Administration → Service Accounts**
2. 建立新 Service Account，角色選 **Admin**
3. 建立 Token，複製後填入 Synapse

或使用 Grafana API 快速產生：

```bash
curl -s -X POST http://admin:your-password@localhost:3000/api/serviceaccounts \
  -H "Content-Type: application/json" \
  -d '{"name":"synapse","role":"Admin"}' | jq .id
# 取得 ID 後
curl -s -X POST http://admin:your-password@localhost:3000/api/serviceaccounts/{id}/tokens \
  -H "Content-Type: application/json" \
  -d '{"name":"synapse-token"}' | jq .key
```

## 🖼️ 啟用 iframe 嵌入

Synapse 在工作負載詳情頁面以 iframe 嵌入 Grafana 圖表，需確保 Grafana 已啟用 embedding：

**Docker Compose 環境**（於 `docker-compose.yaml` 中設定環境變數）：

```yaml
environment:
  - GF_SECURITY_ALLOW_EMBEDDING=true
```

**grafana.ini 方式**：

```ini
[security]
allow_embedding = true
```

## 🐛 故障排查

### Grafana 無法連線 Prometheus

1. 確認 Prometheus 正在執行
2. 修正 `provisioning/datasources/prometheus.yaml` 中的 URL
3. 若 Prometheus 在 Docker 中，使用 `host.docker.internal`
4. 重啟 Grafana：`docker compose restart grafana`

### iframe 無法顯示圖表

- 確認已設定 `GF_SECURITY_ALLOW_EMBEDDING=true`
- 確認 Synapse 系統設定中的 Grafana URL 與瀏覽器可存取的位址一致
- 確認 Dashboard UID 與 Panel ID 正確

### 資料來源測試失敗

```bash
# 手動測試 Prometheus 連線
curl http://your-prometheus:9090/api/v1/query?query=up
```
