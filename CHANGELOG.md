# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

#### 🌐 Gateway API（全新功能）
- 自動偵測叢集是否安裝 Gateway API CRD（`gateway.networking.k8s.io`），未安裝時顯示引導與一鍵安裝指令
- **GatewayClass**：列表 + 詳情抽屜（Controller、Accepted Conditions）
- **Gateway**：完整 CRUD + YAML 編輯（Monaco）；Listeners / Addresses / Conditions 詳情抽屜；關聯 HTTPRoute 計數
- **HTTPRoute**：完整 CRUD + YAML；parentRefs / Hostnames / Match Conditions / Backend Rules 詳情；多後端流量**權重視覺化**（堆疊百分比色條）
- **GRPCRoute**：完整 CRUD + YAML（gRPC Service / Method 比對規則）
- **ReferenceGrant**：建立 / 刪除 + YAML 查看（跨 namespace 流量授權管理）
- **Gateway 拓撲圖**：React Flow 有向圖，GatewayClass → Gateway → HTTPRoute/GRPCRoute → Service，dagre LR 自動排版，可拖曳節點 + MiniMap

#### 🗺 叢集網路拓樸（全新功能）
- **Phase A — 靜態拓樸**（零依賴，所有叢集適用）
  - 批次解析 Pod → ReplicaSet → Deployment ownerRef，以 Workload 節點呈現 Ready/Total 計數
  - 透過 Service selector 比對建立 Service → Workload 連線
  - Endpoint readiness 決定邊健康狀態（Healthy / Degraded / Down / Unknown）
  - **ParticleEdge 動畫**：SVG `animateMotion` 粒子沿邊流動，顏色與速度對應健康狀態
  - 命名空間多選篩選
- **Phase B — 條件式整合**（依叢集安裝狀態自動啟用）
  - **Cilium 偵測**：偵測 `hubble-relay` Service，工具列顯示版本徽章
  - **Istio 指標強化**（`Istio Metrics` 開關）：透過 K8s API Server Proxy 查詢叢集內 Prometheus，取得 `requestRate` / `errorRate` / `latencyP99ms`；errorRate > 5% 邊轉橙色、> 20% 轉紅色；邊標籤即時顯示錯誤率百分比

#### 📊 資源治理（Resource Governance）
- **Phase 1 — 佔用分析**（不依賴 Prometheus）
  - 叢集 CPU / 記憶體 allocatable、requested、occupancy、headroom 即時卡片
  - 命名空間橫向 BarChart 排行 + 明細表
  - 跨叢集洞察：全平台佔用率對比、叢集排行、整體效率評分
- **Phase 2 — 效率分析**（需要 Prometheus）
  - CPU 效率 × 記憶體效率散點圖，泡泡大小 = CPU 佔用量
  - 工作負載 WasteScore 排序（廢棄分數 = `(1-CPU效率)×0.6 + (1-Mem效率)×0.4`）
  - 低效工作負載列表（CPU 效率 < 20%）
- **Phase 3 — 容量規劃**
  - 月度 CPU / 記憶體佔用率折線圖（每日快照）
  - 線性迴歸外推：標示到達 80% / 100% 的預測日期（180 天期）
  - Right-sizing 建議（7 日最大用量 × 安全係數 CPU ×1.2 / Mem ×1.25）
  - CSV 匯出低效工作負載報告
- **Phase 4 — 雲端帳單**
  - AWS Cost Explorer（SigV4 簽名）
  - GCP Cloud Billing（Service Account oauth2）
  - 資源單位成本：USD/core-hr、USD/GiB-hr

#### 🔔 告警管理
- **AlertManager Receiver CRUD**：Webhook / Email / Slack / PagerDuty Receiver 建立 / 編輯 / 刪除 + 即時測試

#### 🔐 Kubeconfig 安全強化（§5.16）
- AES-256-GCM 加密儲存 kubeconfig（`ENCRYPTION_KEY` 環境變數）
- 記憶體中解密，靜態資料全程加密
- P0–P3 全部完成：加密遷移、存取審計、TLS 策略（strict / warn / skip）、連線測試隔離

#### 📡 自身可觀測性（§5.15）
- 自訂 Prometheus 指標：HTTP 請求量 / 延遲 / WebSocket 連線數 / DB 查詢時間 / K8s API 呼叫次數 / Worker 執行狀態
- 增強 `/health` 端點（DB、K8s 叢集連線健康詳情）
- `/metrics` 端點暴露全部自訂指標

#### 🔔 通知渠道
- **Telegram** 通知渠道支援（Bot Token / Chat ID）
- **Microsoft Teams** Adaptive Card 格式推送

#### 🛠 開發環境
- `scripts/dev.sh`：一鍵啟動後端 + 前端開發環境，支援 `--mysql-only` 選項
- MySQL Docker Compose（`docker-compose.mysql.yml`）
- `db/init.sql`：MySQL / SQLite 初始化腳本

### Fixed
- `secret`：建立 service-account-token 類型 Secret 時 500 錯誤
- `table`：所有列表頁 rowSelection 補 `columnWidth: 48` 對齊
- `gateway`：操作欄 i18n 鍵值回傳物件而非字串（`common:actions` 衝突）
- `gateway`：工具列建立按鈕未靠右（補 `flex: 1` spacer）
- `gateway`：`GatewayForm` / `HTTPRouteForm` TS7053 metadata 索引型別錯誤
- `gateway`：`ServiceMeshTab` 遺漏 `App.useApp()` 導致 message 未定義
- `i18n`：`zh-TW/network.json` 多餘結尾逗號導致 Vite JSON 解析失敗
- `db`：`Cluster.BeforeSave` 空 JSON 欄位未補 `{}` 導致 MySQL 寫入失敗
- `ui`：叢集詳情統計卡片、節點統計卡片改為淡雅白底設計

---

## [1.0.0] - 2026-01-07

### Added

#### 🏗 多叢集管理
- 匯入多個 K8s 叢集（支援 kubeconfig / 手動設定），統一展示健康狀態、版本、節點數
- 叢集詳情：即時監控 CPU / 記憶體 / Pod 用量趨勢；叢集事件 Timeline
- 全域儀表板：跨叢集彙總異常工作負載、告警摘要、資源分佈

#### 🧩 工作負載管理
- Deployment / StatefulSet / DaemonSet / Job / CronJob 完整 CRUD
- HPA 自動擴縮建立 / 編輯 / 刪除
- Argo Rollouts Canary / Blue-Green 狀態展示與操控（Promote / Abort）
- YAML 在線編輯器（Monaco Editor）+ Diff 預覽

#### 📦 Pod 管理
- 跨命名空間 Pod 列表；詳情含容器、資源、掛載卷、事件
- 即時日誌 WebSocket 串流（多容器切換、關鍵字搜尋）
- Web Terminal（kubectl exec）

#### 🌐 網路管理
- Service / Ingress 完整 CRUD
- NetworkPolicy CRUD + 拓撲視覺化 + 建立精靈 + 策略模擬
- Service Mesh（Istio）流量拓撲圖（RPS / 錯誤率 / P99 延遲）

#### 🗂 設定管理
- ConfigMap / Secret 完整 CRUD + 版本歷史 + 一鍵回滾

#### 💾 儲存管理
- PVC / PV / StorageClass 列表；PVC 建立 / 刪除

#### 🏷 命名空間管理
- 列表 / 建立 / 刪除；ResourceQuota / LimitRange CRUD

#### 🖥 節點管理
- 節點清單 + 詳情；Cordon / Uncordon / Drain；SSH Terminal

#### 📊 監控與告警
- Prometheus / Grafana 整合；自定義 PromQL 查詢面板
- AlertManager 告警列表 / Silences 管理 / 設定 YAML 編輯
- 自定義 Event 告警規則（Webhook / DingTalk / Slack / Teams）

#### 📋 日誌中心
- K8s 容器日誌串流；Event 日誌；Loki / Elasticsearch 整合

#### 🔐 安全管理
- Trivy 映像漏洞掃描；CIS Benchmark（kube-bench）

#### 🔗 多叢集工作流程
- 工作負載遷移精靈；ConfigMap / Secret 跨叢集同步策略

#### 🔄 GitOps
- ArgoCD 連線 + 應用管理（Sync / Rollback / Delete）

#### ⎈ Helm
- Release 列表 / 安裝 / 升級 / 回滾；Chart 倉庫管理

#### 🤖 AI 智慧運維
- 多 Provider AI 聊天（OpenAI / Azure / Claude / Ollama）+ Tool Calling
- AI 診斷（OOMKilled / CrashLoopBackOff 等）+ Runbook 知識庫
- 自然語言查詢（NL Query）；YAML 生成助手

#### 👤 使用者與權限
- 使用者 / 群組 CRUD；四角色 RBAC（Admin / Ops / Dev / ReadOnly）；LDAP 整合

#### 🔒 稽核與合規
- 操作日誌；Terminal 會話稽核；部署審批工作流；SIEM Webhook 推送

#### 🔍 全域搜尋
- 跨叢集 Pod / Deployment / Service 搜尋

### Security
- JWT 認證；bcrypt 密碼雜湊；CORS 設定；操作稽核日誌

---

## Version History

| 版本 | 發布日期 | 重點功能 |
|------|----------|----------|
| Unreleased | — | Gateway API、叢集網路拓樸、資源治理、Kubeconfig 加密、自身可觀測性 |
| 1.0.0 | 2026-01-07 | 初始發布：多叢集管理、工作負載、監控告警、AI 運維 |
