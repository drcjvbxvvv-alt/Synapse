# Synapse VM 部署可觀測性指南

> 適用：直接在 Linux 虛擬機器或裸機上以二進位檔方式運行 Synapse（非容器）。

---

## 架構總覽

```
Synapse Process（VM）
  ├── GET /metrics       ←── Prometheus scrape（static_configs）
  ├── GET /healthz       ←── systemd ExecStartPost / HAProxy check
  ├── GET /readyz        ←── Uptime Kuma / Blackbox Exporter
  └── stdout JSON log   ──►  Promtail（tail log file）──► Loki

VM Host
  └── node_exporter     ←── Prometheus scrape（另一個 job，採集 CPU/MEM/Disk/IO）
```

---

## 1. systemd Unit 範本

建立 `/etc/systemd/system/synapse.service`：

```ini
[Unit]
Description=Synapse Kubernetes Management Platform
After=network.target mysql.service
Wants=mysql.service

[Service]
Type=simple
User=synapse
Group=synapse
WorkingDirectory=/opt/synapse
ExecStart=/opt/synapse/synapse
Restart=on-failure
RestartSec=5s

# 環境變數（亦可使用 EnvironmentFile=/opt/synapse/.env）
Environment=SERVER_PORT=8080
Environment=SERVER_MODE=release
Environment=LOG_FORMAT=json
Environment=LOG_LEVEL=info
Environment=DB_DRIVER=mysql
Environment=DB_HOST=127.0.0.1
Environment=DB_PORT=3306
Environment=DB_USERNAME=synapse
Environment=DB_PASSWORD=your-db-password
Environment=DB_DATABASE=synapse
Environment=JWT_SECRET=your-jwt-secret
Environment=ENCRYPTION_KEY=your-32-byte-hex-key

# 可觀測性設定
Environment=OBSERVABILITY_ENABLED=true
Environment=METRICS_TOKEN=your-scrape-token     # 留空則不驗證
Environment=HEALTH_PATH=/healthz
Environment=READY_PATH=/readyz

# 日誌輸出到 journald（搭配 Promtail systemd source 或重定向到檔案）
StandardOutput=journal
StandardError=journal
SyslogIdentifier=synapse

# 資源限制
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

啟用並啟動：

```bash
sudo systemctl daemon-reload
sudo systemctl enable synapse
sudo systemctl start synapse
sudo systemctl status synapse
```

存活探針（systemd ExecStartPost）：

```ini
ExecStartPost=/bin/bash -c 'for i in $(seq 1 10); do curl -sf http://localhost:8080/healthz && exit 0 || sleep 2; done; exit 1'
```

---

## 2. 將 stdout JSON 日誌重定向至檔案（可選）

若不使用 journald，可重定向至檔案：

```ini
StandardOutput=append:/var/log/synapse/synapse.log
StandardError=append:/var/log/synapse/synapse.log
```

建立目錄：

```bash
sudo mkdir -p /var/log/synapse
sudo chown synapse:synapse /var/log/synapse
```

---

## 3. Prometheus scrape_configs 範本

`/etc/prometheus/prometheus.yml`：

```yaml
scrape_configs:
  # Synapse 應用層指標
  - job_name: synapse
    static_configs:
      - targets:
          - "synapse-host:8080"
        labels:
          env: production
          component: synapse
    # 若設定了 METRICS_TOKEN：
    # authorization:
    #   credentials: your-scrape-token
    metrics_path: /metrics
    scrape_interval: 15s

  # VM 主機指標（需安裝 node_exporter）
  - job_name: node
    static_configs:
      - targets:
          - "synapse-host:9100"
        labels:
          env: production
          host: synapse-vm
    scrape_interval: 15s
```

---

## 4. Promtail 設定（JSON log pipeline）

`/etc/promtail/config.yml`：

```yaml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: synapse
    static_configs:
      - targets:
          - localhost
        labels:
          job: synapse
          host: synapse-vm
          __path__: /var/log/synapse/synapse.log

    pipeline_stages:
      # 解析 JSON 格式日誌
      - json:
          expressions:
            level: level
            msg: msg
            time: time
            request_id: request_id
            method: method
            path: path
            status: status
            latency: latency

      # 將 level 提取為 Loki label（便於過濾）
      - labels:
          level:

      # 將 time 欄位設為日誌時間戳
      - timestamp:
          source: time
          format: RFC3339Nano
```

若使用 journald source（不寫檔案）：

```yaml
  - job_name: synapse-journal
    journal:
      labels:
        job: synapse
      matches: _SYSTEMD_UNIT=synapse.service
    pipeline_stages:
      - json:
          expressions:
            level: level
            msg: msg
      - labels:
          level:
```

---

## 5. logrotate 範本

`/etc/logrotate.d/synapse`：

```
/var/log/synapse/synapse.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    create 0640 synapse synapse
    postrotate
        # 讓 synapse 重新開啟日誌檔案（若使用 reopen-on-SIGHUP）
        # systemctl kill --signal=SIGHUP synapse
    endscript
}
```

---

## 6. HAProxy 健康探針範本

```haproxy
backend synapse_backend
    balance roundrobin
    option httpchk GET /healthz
    http-check expect status 200
    server synapse1 synapse-host:8080 check inter 5s fall 3 rise 2
```

---

## 7. Grafana Datasource 接入

1. 進入 Grafana → Connections → Add data source
2. 選擇 **Prometheus**，URL 填入 Prometheus 位址（如 `http://prometheus:9090`）
3. 選擇 **Loki**，URL 填入 Loki 位址（如 `http://loki:3100`）
4. Import dashboard：使用 `deploy/monitoring/synapse-dashboard.json`

---

## 8. 常用 PromQL 查詢

```promql
# HTTP 請求率
rate(synapse_http_requests_total[5m])

# p99 延遲
histogram_quantile(0.99, rate(synapse_http_request_duration_seconds_bucket[5m]))

# 活躍 WebSocket 連線
sum(synapse_websocket_connections_active) by (type)

# DB 查詢延遲 p95
histogram_quantile(0.95, rate(synapse_db_query_duration_seconds_bucket[5m])) by (operation)

# Worker 最後執行時間（距現在多久）
time() - synapse_worker_last_run_timestamp

# Informer 未同步的叢集
synapse_cluster_informer_synced == 0

# Goroutine 數量
go_goroutines{job="synapse"}

# Heap 記憶體使用
go_memstats_heap_inuse_bytes{job="synapse"}
```
