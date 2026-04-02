# KubePolaris 系統規劃書

> 版本：v1.1 | 日期：2026-04-02 | 狀態：進行中

## 實作記錄（2026-04-02）

| 項目 | 狀態 | 新增 / 修改檔案 |
|------|------|----------------|
| AES-256-GCM 憑證欄位加密 | ✅ 完成 | `pkg/crypto/crypto.go`、`internal/models/cluster.go`（GORM hooks）、`main.go`（`crypto.Init`）、`internal/config/config.go`（`ENCRYPTION_KEY`） |
| JWT secret release 強制驗證 | ✅ 確認已實作 | `internal/config/config.go`（logger.Fatal） |
| Login Rate Limiting + 帳號鎖定 | ✅ 完成 | `internal/middleware/rate_limit.go`、`internal/router/router.go`（套用 LoginRateLimit） |
| WebSocket Origin 驗證 | ✅ 確認已實作 | 所有 6 個 WS handler 已用 `middleware.IsOriginAllowed()` |
| RequestID Middleware | ✅ 完成 | `internal/middleware/request_id.go`、`internal/router/router.go` |
| Informer 同步逾時可設定化 | ✅ 完成 | `internal/k8s/manager.go`（`SetSyncTimeout`）、`internal/router/router.go`（`INFORMER_SYNC_TIMEOUT`）、`internal/config/config.go` |
| 叢集 Pod 指標 Fallback | ✅ 完成 | `internal/services/cluster_service.go`（`fetchPodStats`） |
| SQLite WAL 啟用 | ✅ 確認已實作 | `internal/database/database.go`（`_journal_mode=WAL`） |
| Prometheus Metrics Endpoint | ✅ 完成 | `internal/middleware/metrics.go`、`internal/router/router.go`（`/metrics` route） |
| /readyz 健康深化 | ✅ 完成 | `internal/router/router.go`（DB ping） |

**已完成（2026-04-02 第二批次）：**
- ✅ 結構化日誌（slog / JSON，LOG_FORMAT env 切換）
- ✅ 稽核日誌完整查詢（GetAuditLogs 委派 OperationLogService.List）
- ✅ 完整 API 分頁（permission 列表、cluster 列表修正假分頁）
- ✅ 前端 React Query 導入（@tanstack/react-query v5，QueryClientProvider）
- ✅ 前端列表虛擬捲動（16 個 Table 加入 virtual + scroll.y=600）

**未完成（下一批次）：**
- Helm Release 管理（M4）

---

## 目錄

1. [系統現況總覽](#1-系統現況總覽)
2. [已知缺陷與技術債](#2-已知缺陷與技術債)
3. [邊界天花板分析](#3-邊界天花板分析)
4. [解決方案與優化計劃](#4-解決方案與優化計劃)
5. [新功能規劃](#5-新功能規劃)
6. [優先序與里程碑](#6-優先序與里程碑)

---

## 1. 系統現況總覽

KubePolaris 是以 Go 1.25（Gin）+ React 19（Ant Design）構建的企業級 Kubernetes 多叢集管理平台。後端以單一二進位檔嵌入前端靜態資源，支援 SQLite（開發）與 MySQL 8（生產）雙資料庫，整合 Prometheus / Grafana / AlertManager / ArgoCD，提供 Web Terminal（Pod Exec、kubectl、Node SSH）。

**目前實作的主要功能：**

| 領域 | 功能 |
|------|------|
| 叢集管理 | 多叢集匯入（kubeconfig / Token）、健康狀態、總覽指標 |
| 工作負載 | Deployment / StatefulSet / DaemonSet / Job / CronJob / Argo Rollouts |
| 設定管理 | ConfigMap / Secret CRUD |
| 網路管理 | Service / Ingress CRUD |
| 儲存管理 | PVC / PV / StorageClass |
| 命名空間 | 建立、配額檢視、刪除 |
| 使用者 RBAC | 多租戶、叢集 / 命名空間粒度、LDAP 整合 |
| 監控告警 | Prometheus 指標、Grafana 儀表板、AlertManager |
| GitOps | ArgoCD 應用管理與同步 |
| 日誌稽核 | 操作日誌、Web Terminal 指令稽核 |
| 全域搜尋 | 跨叢集資源搜尋 |
| 國際化 | zh-TW、en-US |

---

## 2. 已知缺陷與技術債

### 2.1 安全性缺陷（HIGH）

#### ~~🔴 S1 — 叢集憑證明文儲存~~ ✅ 已修復（2026-04-02）
**修復：** 建立 `pkg/crypto` 套件（AES-256-GCM），在 `Cluster` 模型加入 `BeforeSave` / `AfterCreate` / `AfterUpdate` / `AfterFind` GORM hooks，透過 `ENCRYPTION_KEY` 環境變數控制加密。未設定金鑰時靜默降級為明文（向下相容）。

---

#### ~~🔴 S2 — JWT Secret 使用預設值警告但未強制~~ ✅ 已確認已修復
**狀態：** `config.go` 中 release 模式使用預設 secret 時已呼叫 `logger.Fatal`（內部呼叫 `os.Exit(1)`）。

---

#### ~~🟡 S3 — WebSocket Origin 驗證不完整~~ ✅ 已確認已實作
**狀態：** 所有 6 個 WebSocket handler（pod、kubectl、kubectl-pod、ssh、log、pod-logs）均已使用 `middleware.IsOriginAllowed()`，從 `CORS_ALLOWED_ORIGINS` 環境變數讀取白名單。

---

#### ~~🟡 S4 — Rate Limiting 未實作~~ ✅ 已修復（2026-04-02）
**修復：** 建立 `internal/middleware/rate_limit.go`，對 Login 端點套用 IP + 使用者名稱雙維度限流（5次/分鐘），超限後鎖定 15 分鐘，回傳 HTTP 429。

---

### 2.2 功能缺陷（MEDIUM）

#### ~~🟡 F1 — 叢集指標回傳硬編碼 0~~ ✅ 部分修復（2026-04-02）
**修復：** 新增 `fetchPodStats()` 函式，透過 K8s clientset API 取得真實 Pod 數量（total、running），使用 15 秒超時避免阻塞。CPU/MEM 用量仍需 Prometheus 整合（保留 TODO）。

---

#### 🟡 F2 — 操作稽核日誌查詢未完整實作
**問題：**
`audit.go:25` 有 `// TODO` 標記，操作日誌的進階篩選（時間範圍、使用者、模組、動作）未完整實作。

---

#### ~~🟡 F3 — Informer 快取同步超時過短~~ ✅ 已修復（2026-04-02）
**修復：** `ClusterInformerManager` 新增 `syncTimeout` 欄位（預設 30 秒），透過 `SetSyncTimeout()` 方法設定；路由器讀取 `INFORMER_SYNC_TIMEOUT` 環境變數並套用。`GetOverviewSnapshot` 的硬編碼 2 秒已改用 `m.syncTimeout`。

---

#### ~~🟡 F4 — 無請求追蹤 ID（Trace ID）~~ ✅ 已修復（2026-04-02）
**修復：** 建立 `internal/middleware/request_id.go`，每次請求注入 `X-Request-ID`（若客戶端未提供則自動生成 UUID v4），已掛載至全域中間件鏈。

---

### 2.3 架構技術債（LOW~MEDIUM）

#### 🟢 A1 — 前端狀態管理缺乏全域快取
**問題：**
使用原生 `axios` + `useState`，相同資料在不同頁面重複請求，無快取、無樂觀更新、無背景更新。

**解決方案：** 引入 `@tanstack/react-query`，統一伺服器狀態管理、自動重試、快取失效策略。

---

#### 🟢 A2 — 後端錯誤訊息未國際化
**問題：**
所有後端錯誤訊息均為中文字串硬編碼，API 回傳的 `message` 在英文介面顯示中文，破壞使用體驗。

---

#### 🟢 A3 — router.go 單一檔案過大（31 KB）
**問題：**
所有路由定義在單一檔案，超過 800 行，維護困難。

**解決方案：** 按領域拆分（`cluster_routes.go`、`workload_routes.go`、`auth_routes.go` 等）。

---

#### ~~🟢 A4 — SQLite 不支援並行寫入~~ ✅ 已確認已實作
**狀態：** `database.go` 的 SQLite DSN 已包含 `_journal_mode=WAL&_foreign_keys=on`，WAL 模式在初始化時即自動啟用。

---

## 3. 邊界天花板分析

### 3.1 規模上限

| 維度 | 目前天花板 | 根本原因 | 改善方向 |
|------|-----------|---------|---------|
| **叢集數量** | ~20 個 | 每叢集建立獨立 Informer（記憶體 O(n) 增長），Goroutine 洩漏風險 | Informer 池化 + Lazy 初始化 + 閒置叢集 GC |
| **單叢集 Pod 數** | ~5,000 個 | Informer 全量快取於記憶體，列表頁一次回傳 | 分頁快取 + 伺服器端分頁 |
| **並行 Web Terminal** | ~50 個 | 每個 Terminal 佔用 goroutine + WebSocket 連線 | 連線池 + 心跳管理 + 閒置超時 |
| **Log 串流** | 依 K8s API 上限 | 直接 proxy K8s log stream，無緩衝 | 引入 log 中間緩衝層（如 Loki） |
| **並行 API 請求** | ~200 QPS | 無 rate limit，K8s client 無連線池設定 | 限流 + K8s client 連線池調優 |
| **資料庫規模** | SQLite ~1GB / MySQL 無硬限 | 操作日誌、稽核日誌無分區 | 日誌表按月分區 + 資料保留策略 |

### 3.2 功能邊界

| 功能領域 | 現有邊界 | 說明 |
|---------|---------|------|
| **工作負載管理** | 不支援自訂 CRD | 僅內建 K8s 資源 + Argo Rollouts |
| **CI/CD Pipeline** | 無 | 依賴外部 ArgoCD，無原生 Pipeline |
| **多租戶隔離** | 命名空間粒度 | 無跨叢集租戶策略，無成本分攤 |
| **Helm 部署** | 無 | 不支援 Helm Release 管理 |
| **網路策略** | 無 NetworkPolicy UI | 僅 Service / Ingress |
| **叢集生命週期** | 無 | 不支援叢集佈建（僅匯入已有叢集） |
| **事件告警** | 依賴 AlertManager | 無 K8s Event 告警規則 |
| **成本分析** | 無 | 無資源成本估算 |
| **備份還原** | 無 | 無 etcd 備份 / Velero 整合 |

---

## 4. 解決方案與優化計劃

### 4.1 安全強化（Phase 1，優先）

```
┌─────────────────────────────────────────────┐
│ 安全強化路線圖                                │
├──────────┬──────────────────────────────────┤
│ 週次     │ 任務                              │
├──────────┼──────────────────────────────────┤
│ Week 1   │ AES-256-GCM 欄位加密              │
│          │ - 新增 pkg/crypto 套件             │
│          │ - Cluster 模型加解密 Hook          │
│          │ - 遷移腳本（存量資料加密）          │
├──────────┼──────────────────────────────────┤
│ Week 2   │ JWT / Auth 加固                   │
│          │ - release 模式強制自訂 secret      │
│          │ - Login 端點 rate limiting         │
│          │ - 帳號鎖定策略（5 次失敗 → 15min） │
├──────────┼──────────────────────────────────┤
│ Week 3   │ WebSocket 安全                    │
│          │ - Origin 白名單驗證               │
│          │ - Terminal session token（短效）   │
│          │ - SSH 私鑰加密儲存                 │
└──────────┴──────────────────────────────────┘
```

### 4.2 可觀測性強化

**目標：** 完整的請求鏈路追蹤、結構化日誌、應用指標。

```go
// 新增 middleware/tracing.go
func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.GetHeader("X-Request-ID")
        if id == "" {
            id = uuid.New().String()
        }
        c.Set("request_id", id)
        c.Header("X-Request-ID", id)
        c.Next()
    }
}
```

**實作項目：**
- [x] RequestID middleware（`middleware/request_id.go`，注入 `X-Request-ID`）
- [x] Prometheus `/metrics` endpoint（`middleware/metrics.go`，記錄 API 延遲直方圖、請求計數器、Informer 同步狀態）
- [x] 健康檢查深化（`/readyz` 真實 DB ping；`/healthz` liveness）
- [ ] 結構化日誌（`slog` 取代現有 logger，JSON 格式，待實作）
- [ ] 分散式追蹤（OpenTelemetry + Jaeger，中期目標）

### 4.3 效能優化

#### 後端

| 優化項目 | 現況 | 目標 | 方案 |
|---------|------|------|------|
| Informer 同步超時 | ~~5 秒硬編碼~~ → **✅ 30 秒可設定** | 30 秒（可設定） | `INFORMER_SYNC_TIMEOUT` env 已套用 |
| SQLite WAL | ~~未啟用~~ → **✅ 已啟用** | 啟用 | `_journal_mode=WAL` 已在 DSN 中 |
| K8s Client 連線池 | 預設值 | 調優 | `MaxIdleConnsPerHost=100`，`Timeout=30s` |
| 閒置叢集資源回收 | 無 | GC 機制 | 超過 N 分鐘無請求的叢集 Informer 暫停 |
| API 分頁 | 部分實作 | 全面強制 | 所有列表端點加入 `page`/`pageSize`/`total` |
| 資料庫索引 | 未審查 | 加入複合索引 | 操作日誌：`(user_id, created_at)`；叢集資源：`(cluster_id, namespace)` |

#### 前端

| 優化項目 | 現況 | 目標 | 方案 |
|---------|------|------|------|
| 伺服器狀態管理 | axios + useState | 統一快取 | 引入 `@tanstack/react-query` |
| Bundle 大小 | 未審查 | 減少首屏 JS | Monaco Editor、xterm.js 改為動態載入 |
| 列表頁虛擬捲動 | 無 | 萬筆資料流暢 | Ant Design Table `virtual` 屬性 |
| WebSocket 重連 | 基本實作 | 自動重連 + 指數退避 | 統一 WebSocket hook |
| 錯誤邊界 | ErrorBoundary 存在 | 細粒度 | 每個功能區塊獨立 ErrorBoundary |

### 4.4 後端錯誤訊息國際化

**問題：** 所有後端 message 為硬編碼中文，API 消費端（CLI、其他系統）無法自行翻譯。

**方案：**

```go
// 統一使用錯誤碼，讓前端翻譯
type APIError struct {
    Code    string `json:"code"`     // "CLUSTER_NOT_FOUND"
    Message string `json:"message"`  // fallback 說明
}

// 前端 i18n 依錯誤碼翻譯
// errors.json: { "CLUSTER_NOT_FOUND": "找不到指定叢集" }
```

### 4.5 資料庫改善

```sql
-- 操作日誌分區（MySQL）
ALTER TABLE operation_logs
PARTITION BY RANGE (YEAR(created_at) * 100 + MONTH(created_at)) (
    PARTITION p202601 VALUES LESS THAN (202602),
    PARTITION p202602 VALUES LESS THAN (202603),
    ...
);

-- 資料保留策略（保留 90 天）
-- 定期清理 Cron job（Go scheduler）
```

---

## 5. 新功能規劃

### 5.1 🔧 Helm Release 管理（高優先）

**背景：** 大量企業使用 Helm 部署應用，目前平台缺乏 Helm 感知，需手動透過 kubectl 操作。

**功能範圍：**
- Helm Release 列表（按叢集 / 命名空間）
- 安裝 / 升級 / 回滾 / 解除安裝
- Values 編輯器（YAML + 表單雙模式）
- Release 歷史版本與差異比對
- Chart Repository 管理（新增、更新）
- 整合 Helm Diff Plugin 預覽變更

**技術方案：** 後端使用 `helm.sh/helm/v3` SDK；前端複用現有 YAML 編輯器元件。

---

### 5.2 🌐 NetworkPolicy 視覺化管理（高優先）

**背景：** NetworkPolicy 是 Kubernetes 零信任網路的關鍵，但 YAML 撰寫困難且難以直觀理解流量規則。

**功能範圍：**
- ✅ NetworkPolicy 列表與 CRUD（已完成）
- 流量規則視覺化拓撲圖（節點 = Pod/Namespace，有向邊 = 允許流量）
- 規則建立精靈（3 步驟：定義 PodSelector → 設定 Ingress/Egress 規則 → 預覽確認）
- 規則衝突檢測（後端靜態分析：重疊 selector + 矛盾 allow/deny）

**技術方案（待實作部分）：**

| 元件 | 方案 | 說明 |
|------|------|------|
| 拓撲圖渲染 | `reactflow` v12 | 已在 Appendix A 選定，Dagre 佈局 |
| 節點資料來源 | `GET /clusters/:id/networkpolicies/topology` | 後端組合 NP + Pod label 資訊 |
| 衝突檢測 | `GET /clusters/:id/networkpolicies/conflicts` | 後端靜態分析，回傳衝突對清單 |
| 精靈步驟 | Ant Design Steps + Form | 最終輸出為 YAML → 複用現有儲存邏輯 |

**後端新增 API：**
```
GET  /clusters/:id/networkpolicies/topology          拓撲節點/邊資料
GET  /clusters/:id/networkpolicies/conflicts         衝突規則清單
POST /clusters/:id/networkpolicies/wizard-validate   精靈步驟驗證
```

**前端新增元件：**
```
ui/src/pages/network/NetworkPolicyTopology.tsx   ReactFlow 拓撲圖
ui/src/pages/network/NetworkPolicyWizard.tsx     3 步驟建立精靈
```

**實作難度：** ⭐⭐⭐⭐（高，ReactFlow 整合 + 衝突邏輯）
**估計工作量：** 3 週

---

### 5.3 💰 資源成本分析（中優先）✅ 已完成

**背景：** 多叢集環境下，各團隊資源用量不透明，缺乏成本分攤依據。企業用戶強需求，是與 Rancher/Kuboard 差異化的關鍵功能。

**功能範圍：**
- 依命名空間 / 工作負載 / 標籤維度的 CPU / Memory 用量統計
- 可設定單價模型（CPU 核 / 小時、GB 記憶體 / 小時，或對應雲廠商定價）
- 月度成本趨勢折線圖（最近 6 個月）
- 資源浪費識別（CPU 用量持續 < 請求量 10% 超過 7 天）
- 成本報表匯出（CSV）

**資料模型：**
```go
// internal/models/cost.go

type CostConfig struct {
    ID           uint    // 每個叢集一筆
    ClusterID    uint
    CpuPricePerCore  float64  // USD / core / hour，例如 0.048
    MemPricePerGiB   float64  // USD / GiB / hour，例如 0.006
    Currency     string  // "USD" / "TWD" / "CNY"
    UpdatedAt    time.Time
}

type ResourceSnapshot struct {
    ID          uint
    ClusterID   uint
    Namespace   string
    Workload    string  // "Deployment/app-name"
    Date        time.Time  // 每日快照（精確到日）
    CpuRequest  float64  // 單位：millicores
    CpuUsage    float64
    MemRequest  float64  // 單位：MiB
    MemUsage    float64
    PodCount    int
}
```

**後端架構：**

```
services/cost_service.go
  ├── CostWorker（每日 00:05 執行）
  │     ├── 從 Prometheus 查詢 container_cpu_usage_seconds_total
  │     │   + kube_pod_container_resource_requests（CPU/Memory）
  │     └── 若無 Prometheus → fallback 到 K8s metrics-server
  ├── CalcCost(snapshot, config) float64
  ├── DetectWaste(clusterID) []WasteReport
  └── ExportCSV(clusterID, month) []byte
```

**API 端點：**
```
GET  /clusters/:id/cost/config           讀取定價設定
PUT  /clusters/:id/cost/config           更新定價設定
GET  /clusters/:id/cost/overview         叢集總覽（本月估算 + 上月對比）
GET  /clusters/:id/cost/namespaces       命名空間成本排行（支援 ?month=2026-04）
GET  /clusters/:id/cost/workloads        工作負載成本明細（支援 ?namespace=&page=）
GET  /clusters/:id/cost/trend            6 個月趨勢（按命名空間堆疊）
GET  /clusters/:id/cost/waste            浪費識別報告
GET  /clusters/:id/cost/export           下載 CSV（?month=2026-04）
```

**前端頁面：**
```
ui/src/pages/cost/
  ├── CostDashboard.tsx      主儀表板（總覽卡 + 排行榜 + 趨勢圖）
  ├── CostConfig.tsx         定價設定 Modal
  ├── WasteReport.tsx        浪費識別列表
  └── costService.ts         API service
```

**前端 UI 構成：**
- 頂部：本月預估總費用 / 最高命名空間 / 浪費百分比 — 3 個 Statistic Card
- 左側：命名空間成本 Bar Chart（Ant Design Charts / recharts）
- 右側：月度趨勢 Line Chart（堆疊，不同命名空間用不同顏色）
- 底部：工作負載明細 Table（含 CPU 利用率進度條、記憶體利用率、估算成本）
- 浪費頁籤：低利用率工作負載列表 + 建議縮容動作

**依賴條件：**
- Prometheus 已部署（若無則退化為 requests-only 計算，不含實際用量）
- metrics-server 作為備援資料來源

**實作難度：** ⭐⭐⭐（中，主要是 Prometheus 查詢與資料模型設計）
**估計工作量：** 4 週

---

### 5.4 🔔 K8s Event 告警規則（中優先）✅ 已完成

---

### 5.5 🤖 AI 能力升級（中優先）

**背景：** AI 診斷按鈕與多 Provider 設定已完成。待完成部分為更深度的智慧運維能力。

**已完成：**
- ✅ Pod / Workload AI 診斷（透過 ai:diagnose 事件驅動）
- ✅ 多 AI Provider（OpenAI / Azure / Anthropic / Ollama）

**待實作功能：**

#### 5.5.1 自然語言 K8s 查詢（NL Query）

**設計方案：**
```
使用者輸入 → AI 解析意圖 → 生成結構化查詢 → 執行 K8s API → 回傳結果

系統 Prompt 範例：
  你是 KubePolaris K8s 查詢助手。
  將使用者問題轉換為 JSON 查詢規格：
  { "resource": "pods", "filter": {"restartCount": {">": 5}}, "namespace": "_all_" }
```

**後端新增：**
```go
// POST /ai/nl-query
// 接收 { "question": "列出所有重啟超過 5 次的 Pod" }
// AI 產生 QuerySpec → handlers/ai_query.go 執行對應 K8s API
// 回傳結構化資源列表
```

**支援查詢類型（第一版）：**
- Pod（重啟次數、狀態、命名空間）
- Deployment（副本數、映像版本）
- Node（狀態、標籤、CPU/記憶體壓力）
- Event（類型、原因、時間範圍）

#### 5.5.2 YAML 生成助手

**設計方案：**
```
使用者描述 → AI 以 K8s 專家角色生成 YAML → 前端 Monaco 預覽 → 可直接套用
```

**前端整合：**
- 在 AI Chat 面板新增 `/yaml` 指令模式
- 輸出的 YAML 可一鍵複製或直接送往 `POST /clusters/:id/yaml/apply`

#### 5.5.3 敏感資料過濾

**設計方案：**
```go
// internal/services/ai_sanitizer.go
// 在組裝 AI prompt 前，過濾所有 Secret data 欄位值
// 替換為 "[REDACTED: secret/my-secret]"
func SanitizeContext(ctx string) string
```

過濾規則：
- Secret `data` 欄位值 → `[REDACTED]`
- 含 `password`/`token`/`key` 的環境變數值 → `[REDACTED]`
- PEM 憑證內容 → `[REDACTED: certificate]`

#### 5.5.4 Runbook 自動推薦

**設計方案：**
- 內建 Runbook 知識庫（JSON 檔案，隨二進位嵌入）
- AI 診斷回應後，後端從知識庫比對 reason/message keyword，附加 runbook 連結
- 第一版 10 個常見場景：OOMKilled、CrashLoopBackOff、ImagePullBackOff、Evicted、NodeNotReady、PVCPending、CPUThrottling、FailedScheduling、DiskPressure、NetworkPolicy 阻擋

**API：**
```
GET /ai/runbooks?reason=OOMKilled     回傳相關 Runbook 列表
```

**實作難度（整體）：** ⭐⭐⭐⭐（高，NL Query 需 prompt engineering 迭代）
**估計工作量：** 4 週

---

### 5.6 📋 CRD 通用管理介面（中優先）✅ 已完成

---

### 5.7 🔄 多叢集工作流程（低優先）

**背景：** 多叢集管理的核心價值在於跨叢集協作，目前平台叢集間彼此隔離，無任何協同操作。

**功能範圍：**

#### 5.7.1 跨叢集工作負載遷移精靈

**流程：**
```
選擇來源叢集 → 選擇工作負載 → 選擇目標叢集 → 命名空間映射
→ 資源檢查（目標叢集是否有足夠 CPU/記憶體）→ 確認 → 執行遷移
```

**後端邏輯：**
```go
// POST /workloads/migrate
// 1. 從來源叢集取得 Deployment YAML（去除 status/resourceVersion）
// 2. 在目標叢集建立 Namespace（若不存在）
// 3. 同步相依 ConfigMap / Secret（可選）
// 4. Apply Deployment 至目標叢集
// 5. 回傳遷移結果（成功/失敗詳情）
```

#### 5.7.2 多叢集配置同步

**設計方案：**
- 定義「同步策略」：來源叢集 + 來源命名空間 + 資源類型 + 目標叢集列表
- 支援手動觸發或定時（Cron）同步
- 衝突策略：`overwrite` / `skip` / `merge`

**資料模型：**
```go
type SyncPolicy struct {
    ID              uint
    Name            string
    SourceClusterID uint
    SourceNamespace string
    ResourceType    string  // "ConfigMap" / "Secret"
    ResourceNames   string  // JSON 陣列
    TargetClusters  string  // JSON 陣列（叢集 ID）
    ConflictPolicy  string  // "overwrite" / "skip"
    Schedule        string  // Cron 表達式，空字串表示手動
    LastSyncAt      time.Time
}
```

#### 5.7.3 跨叢集流量策略（Istio 整合，僅查看）

**第一版範圍（保守）：**
- 查看叢集內已安裝的 Istio VirtualService / ServiceEntry CRD
- 複用 CRD 通用介面（5.6 已完成）實現，不額外開發

**實作難度：** ⭐⭐⭐（中，遷移精靈最複雜，同步邏輯次之）
**估計工作量：** 5 週

---

### 5.8 🛡️ 合規性與安全掃描（低優先）

**背景：** 企業合規需求（SOC2、等保三級）要求定期評估 K8s 叢集安全狀態。

**功能範圍與實作策略：**

#### 5.8.1 Trivy 映像掃描整合

**設計方案：**
- 後端透過 `os/exec` 呼叫 `trivy image <image>` 取得掃描結果（需主機已安裝 Trivy）
- 或呼叫 Trivy Server API（`trivy server --listen 0.0.0.0:4954`）
- 掃描結果存入 DB，關聯至工作負載

```go
type ImageScanResult struct {
    ID            uint
    ClusterID     uint
    Namespace     string
    Workload      string
    Image         string
    ScannedAt     time.Time
    CriticalCount int
    HighCount     int
    MediumCount   int
    ResultJSON    string  // 完整 Trivy JSON 結果
}
```

**API：**
```
POST /clusters/:id/security/scan           觸發掃描（非同步）
GET  /clusters/:id/security/scan-results   掃描結果列表
GET  /clusters/:id/security/scan-results/:workload  特定工作負載結果
```

#### 5.8.2 CIS Kubernetes Benchmark 評分

**設計方案：**
- 在目標叢集建立短暫 `kube-bench` Job（使用官方 `aquasec/kube-bench` 映像）
- 等待 Job 完成，取得 logs，解析 JSON 輸出
- 儲存評分摘要（PASS/FAIL/WARN 計數）

**評分維度：** Master Node、Worker Node、etcd、Control Plane、Policies

#### 5.8.3 OPA/Gatekeeper 策略管理

**設計方案（利用已有 CRD 介面）：**
- `ConstraintTemplate` / `Constraint` CRD 通過 5.6 通用介面管理
- 新增「Gatekeeper 儀表板」：統計各策略違規次數（從 Constraint status 讀取）

**實作難度：** ⭐⭐⭐⭐（高，Trivy 整合與 kube-bench Job 管理最複雜）
**估計工作量：** 6 週

---

### 5.9 📦 備份與災難恢復（低優先）

**背景：** 生產環境需要定期備份，但 Velero 部署門檻高，第一版聚焦於「輕量匯出」。

**功能範圍（分兩階段）：**

#### Phase 1：工作負載配置匯出（無外部依賴）

**設計方案：**
- 選擇叢集 + 命名空間 → 後端依序取出所有資源 YAML → 打包成 ZIP 下載
- 資源清單：Deployment、StatefulSet、DaemonSet、Service、Ingress、ConfigMap（不含 Secret 值）、PVC（不含資料）、HPA、CronJob
- 提供 `manifest.json` 索引，記錄資源版本與匯出時間

```
GET /clusters/:id/backup/export?namespace=prod    下載 ZIP 包
```

#### Phase 2：Velero 整合（需叢集已安裝 Velero）

**設計方案：**
- 透過 K8s CRD 管理 Velero `Backup` / `Schedule` / `Restore` 資源
- 複用 CRD 通用介面（5.6）實現基礎 CRUD
- 新增 Velero 專屬儀表板：備份清單 + 狀態 + 觸發還原

```go
// 偵測叢集是否安裝 Velero
// GET /clusters/:id/backup/velero-status
// 回傳 { installed: bool, version: string }
```

**etcd 快照：** 僅限 Self-managed 叢集（EKS/GKE/AKS 不適用），優先級最低，暫不實作。

**實作難度：** ⭐⭐（Phase 1 低；Phase 2 中）
**估計工作量：** Phase 1：1 週；Phase 2：3 週

---

### 5.10 🖥️ CLI 工具（低優先）

**背景：** 提供 CLI 工具可讓 DevOps 工程師在 CI/CD pipeline 中使用平台功能，無需開啟瀏覽器。

**功能範圍：**

**技術方案：**
- 使用 `cobra` + `viper` 框架，獨立 Go 二進位（`cmd/cli/main.go`）
- 設定檔：`~/.kubepolaris/config.yaml`（server URL + JWT token）
- 輸出格式：`--output table|json|yaml`

**指令設計：**
```
kubepolaris login --server https://... --token <token>
kubepolaris cluster list
kubepolaris cluster use <id>

kubepolaris pod list [--namespace <ns>] [--cluster <id>]
kubepolaris pod logs <name> --namespace <ns>

kubepolaris workload list [--type deployment|statefulset]
kubepolaris workload rollout restart <name> --namespace <ns>
kubepolaris workload rollout undo <name> --namespace <ns>

kubepolaris helm list [--namespace <ns>]
kubepolaris helm upgrade <release> --chart <repo/chart> --values values.yaml

kubepolaris yaml apply -f manifest.yaml --cluster <id>

kubepolaris cost overview [--month 2026-04]
```

**CI/CD 整合範例：**
```yaml
# GitHub Actions
- name: Deploy to Production
  run: |
    kubepolaris helm upgrade myapp --chart stable/myapp \
      --set image.tag=${{ github.sha }} \
      --cluster prod --namespace production
```

**分發方式：**
- `go build` 單一二進位，無外部依賴
- GitHub Releases 提供 Linux / macOS / Windows 預編譯版本
- `go install github.com/clay-wangzhi/KubePolaris/cmd/cli@latest`

**實作難度：** ⭐⭐（低，主要是 REST API 封裝）
**估計工作量：** 3 週

---

## 6. 優先序與里程碑

### 功能完成狀態總覽

| 里程碑 | 功能 | 狀態 | 優先級 | 估計工作量 |
|--------|------|------|--------|-----------|
| M1 | 安全強化（加密/JWT/Rate Limit/WS） | ✅ 已完成 | 高 | — |
| M2 | 穩定性與效能（Informer/WAL/分頁/虛擬捲動） | ✅ 已完成 | 高 | — |
| M3 | 可觀測性（Prometheus/slog/錯誤碼） | ✅ 已完成 | 中 | — |
| M4 | Helm Release 管理 | ✅ 已完成 | 高 | — |
| M5 | AI 診斷 + CRD + NetworkPolicy CRUD + Event 告警 | ✅ 已完成 | 中 | — |
| M6 | **資源成本分析** | ✅ 已完成 | 中 | — |
| M7 | **AI 深度運維**（NL Query / YAML 生成 / Runbook） | 🔲 待實作 | 中 | **4 週** |
| M8 | **多叢集工作流程**（遷移 / 配置同步） | 🔲 待實作 | 低 | **5 週** |
| M9 | **合規性與安全掃描**（Trivy / kube-bench） | 🔲 待實作 | 低 | **6 週** |
| M10 | **備份匯出 + CLI 工具** | 🔲 待實作 | 低 | **4 週** |
| — | NetworkPolicy 視覺化拓撲圖 + 精靈 | 🔲 待實作 | 中 | **3 週** |

**待實作總估計：約 26 週（6.5 個月）**

### 建議實作順序

```
影響度  ↑
高      │ ✅ M1 安全    ✅ M4 Helm
        │ ✅ M2 效能    ✅ M5 AI/CRD/NP/告警
        │
中      │ ✅ M3 可觀測  ✅ M6 成本分析
        │               🔲 M7 AI 深度（優先）
        │               🔲 NP 視覺化（次優先）
        │
低      │               🔲 M8 多叢集
        │               🔲 M10 CLI
        │               🔲 M9 合規掃描
        └─────────────────────────────────→ 實作難度
                  低          中          高
```

**推薦下一步：M7（AI 深度運維）**
- M6 已完成，自然銜接 AI 能力深化
- NL Query + YAML 生成可大幅提升平台差異化競爭力
- 敏感資料過濾是安全合規的必要前置項目

### 里程碑規劃

#### Milestone 1：安全強化（4 週）✅ 已完成
- [x] AES-256-GCM 憑證加密（`pkg/crypto` + Cluster GORM hooks，`ENCRYPTION_KEY` env 控制）
- [x] JWT secret 強制驗證（release 模式使用預設值 → `logger.Fatal` 強制退出）
- [x] Login rate limiting + 帳號鎖定（`middleware/rate_limit.go`，5次/分鐘，鎖定15分鐘）
- [x] WebSocket Origin 驗證（所有 6 個 WS handler 已使用 `middleware.IsOriginAllowed()`）
- [x] RequestID middleware（`middleware/request_id.go`，注入 `X-Request-ID`）

**完成指標：** gosec 掃描 0 個高危問題；Pen test 無嚴重發現。

#### Milestone 2：穩定性與效能（4 週）🔄 部分完成
- [x] Informer 同步超時可設定化（`INFORMER_SYNC_TIMEOUT` env，預設 30 秒）
- [x] 叢集指標 fallback 邏輯（`fetchPodStats()` 從 K8s API 取得實際 Pod 計數）
- [x] SQLite WAL 啟用（`_journal_mode=WAL` 已在 database.go 啟用）
- [x] 完整 API 分頁（permission 列表改 response.List，cluster 修正假分頁）
- [x] React Query 導入（@tanstack/react-query v5，QueryClientProvider，OperationLogs + PodList 遷移）
- [x] 大型列表虛擬捲動（16 個 Table 加入 virtual + scroll.y=600）

**完成指標：** 20 叢集 / 5000 Pod 場景下 API P95 < 200ms。

#### Milestone 3：可觀測性（3 週）🔄 部分完成
- [x] 應用 Prometheus metrics endpoint（`/metrics`，`middleware/metrics.go`，記錄 API 延遲、請求數）
- [x] 健康檢查深化（`/readyz` 真實 DB ping，`/healthz` liveness）
- [x] 結構化日誌（`pkg/logger` 改用 slog，LOG_FORMAT=json|text）
- [x] 稽核日誌完整查詢（`GetAuditLogs` 委派 OperationLogService.List，支援完整篩選）
- [x] 錯誤碼化（`internal/apierrors` 套件，AppError 攜帶 HTTPStatus + Code；`response.FromError` 統一轉換；Service 層回傳結構化錯誤取代字串比對）

**完成指標：** 所有操作可透過日誌全鏈路追蹤。

#### Milestone 4：Helm 管理（6 週）✅
- [x] Helm SDK 整合（helm.sh/helm/v3 v3.14.4 + k8s.io/cli-runtime）
- [x] Release 列表 / 詳情頁（含 Status Tag 顏色、namespace 篩選）
- [x] 安裝 / 升級 / 回滾 / 刪除（REST API + 前端 Modal）
- [x] Chart Repository 管理（CRUD + DB 持久化）
- [x] Values 查看（user values / all values）

**完成指標：** 可替代 `helm list`、`helm upgrade`、`helm rollback` 日常操作。

#### Milestone 5：AI 與 CRD（8 週）✅ 已完成
- [x] AI 診斷 UI 完整開放（Pod / Workload 詳情頁新增「AI 診斷」按鈕，透過自定義事件驅動浮動 AI 面板；AIChatPanel 監聽 ai:diagnose 事件並自動帶入診斷 prompt）
- [x] 多 AI 提供者設定頁（支援 OpenAI / Azure OpenAI / Anthropic Claude / Ollama；AIConfig 新增 api_version 欄位；Provider 切換自動填入預設端點與模型選單；ai_provider.go 處理各 Provider 的 URL、Auth 頭與 Anthropic 獨立格式）
- [x] CRD 自動發現與通用列表（`handlers/crd.go` + 動態客戶端；前端 CRDList / CRDResources 頁面；側邊欄 CRD 管理入口）
- [x] NetworkPolicy 管理介面（`handlers/networkpolicy.go` + 動態 CRUD；前端 NetworkPolicyTab；網路管理頁新增第三個 Tab；三語 i18n）
- [x] Event 告警規則引擎（`models/event_alert.go` + `services/event_alert_service.go` + `handlers/event_alert.go`；後台 Worker 每 60 秒掃描 K8s Events；Webhook / DingTalk 通知；30 分鐘去重；前端 EventAlertRules 頁面含規則 CRUD + 告警歷史兩分頁）

**完成指標：** 平台支援 4 家 AI Provider；CRD/NetworkPolicy 可完整管理；告警通知端對端可用。

---

#### Milestone 6：資源成本分析（4 週）✅ 已完成

> **目標：** 讓多租戶叢集的資源費用透明化，提供命名空間/工作負載級別的成本分攤依據。

- [x] `CostConfig` / `ResourceSnapshot` 資料模型與 AutoMigrate（`internal/models/cost.go`）
- [x] `CostWorker`（每日 00:05 UTC，從 Prometheus 查詢 CPU/Mem request + usage，按命名空間聚合後存快照；無 Prometheus 則跳過）
- [x] REST API 8 支端點（`GET/PUT config`、`overview`、`namespaces`、`workloads`、`trend`、`waste`、`export`）
- [x] 前端成本儀表板 5 個 Tab（總覽卡 × 4、命名空間 Bar Chart + 排行表、工作負載分頁表 + 利用率進度條、6 個月趨勢 Line Chart、浪費識別表）
- [x] 定價設定 Modal（CPU 單價 / 記憶體單價 / 幣別 USD/TWD/CNY/JPY）
- [x] CSV 匯出（`GET /cost/export?month=YYYY-MM`，Content-Disposition attachment）
- [x] 三語 i18n（zh-TW / zh-CN / en-US，`cost.json`）
- [x] 安裝 `recharts` 圖表庫；`MainLayout.tsx` 新增 `DollarOutlined` 側邊欄入口

**完成指標：** 可在叢集儀表板看到本月估算費用；命名空間成本排行可正常顯示；低利用率工作負載可識別。

---

#### Milestone 7：AI 深度運維（4 週）🔲 待實作

> **目標：** 從「AI 輔助診斷」升級為「AI 主動運維助手」，支援自然語言查詢與 YAML 生成。

| 任務 | 檔案 | 週次 |
|------|------|------|
| 敏感資料過濾器（Secret/Token 值 → `[REDACTED]`） | `internal/services/ai_sanitizer.go` | W1 |
| NL Query：系統 Prompt 設計 + QuerySpec 解析 + K8s API 執行 | `internal/handlers/ai_query.go` | W1–W2 |
| YAML 生成助手：系統 Prompt + `/yaml` 指令模式 | `internal/handlers/ai_yaml.go` | W2 |
| Runbook 知識庫 JSON（10 個常見場景） | `internal/assets/runbooks.json` | W3 |
| Runbook 比對 API + 診斷回應附加 Runbook 連結 | `internal/services/runbook_service.go` | W3 |
| 前端 NL Query UI（AI 面板新增查詢模式切換） | `ui/src/components/AIChat/AIChatPanel.tsx` | W3–W4 |
| 前端 YAML 生成介面（輸出至 Monaco + 一鍵 Apply） | `ui/src/components/AIChat/AIYamlPanel.tsx` | W4 |

- [ ] 敏感資料過濾（Secret data / 含 password 的 env var → `[REDACTED]`）
- [ ] 自然語言 K8s 查詢（NL → QuerySpec → K8s API → 結構化結果）
- [ ] YAML 生成助手（描述 → 完整 K8s YAML，可直接 Apply）
- [ ] Runbook 知識庫（10 個常見場景，JSON 嵌入二進位）
- [ ] 診斷回應自動附帶相關 Runbook 連結
- [ ] 三語 i18n

**完成指標：** 輸入「列出所有 OOMKilled 的 Pod」可正確回傳結果；YAML 生成輸出可直接 Apply 不報錯。

---

#### Milestone 8：多叢集工作流程（5 週）🔲 待實作

> **目標：** 打通叢集間協作壁壘，支援工作負載遷移與配置同步。

| 任務 | 檔案 | 週次 |
|------|------|------|
| `SyncPolicy` 資料模型 | `internal/models/sync_policy.go` | W1 |
| 配置同步 API（CRUD + 觸發） + Worker | `internal/services/sync_service.go` | W1–W2 |
| 工作負載遷移後端邏輯（取 YAML → 目標叢集 Apply） | `internal/handlers/workload_migrate.go` | W2–W3 |
| 遷移精靈前端（3 步驟：選叢集 → 資源檢查 → 確認執行） | `ui/src/pages/cluster/MigrateWizard.tsx` | W3–W4 |
| 配置同步管理前端（策略 CRUD + 手動觸發 + 歷史紀錄） | `ui/src/pages/cluster/SyncPolicies.tsx` | W4–W5 |

- [ ] SyncPolicy 資料模型（來源叢集 / 資源 / 目標叢集 / 衝突策略 / Cron）
- [ ] 配置同步服務（手動 + 定時，支援 ConfigMap / Secret）
- [ ] 工作負載遷移精靈後端（YAML 取出 → 清理 metadata → 目標叢集 Apply）
- [ ] 前端遷移精靈（3 步驟精靈 + 資源配額預檢）
- [ ] 前端同步策略管理頁
- [ ] 三語 i18n

**完成指標：** 可將 staging 叢集的 Deployment 遷移到 production 叢集；ConfigMap 同步至 3 個叢集成功率 100%。

---

#### Milestone 9：合規性與安全掃描（6 週）🔲 待實作

> **目標：** 提供叢集安全基線評估，協助企業滿足 SOC2 / 等保合規要求。

| 任務 | 檔案 | 週次 |
|------|------|------|
| `ImageScanResult` 資料模型 | `internal/models/security.go` | W1 |
| Trivy 整合（exec 模式 + Server 模式） | `internal/services/trivy_service.go` | W1–W2 |
| 映像掃描 API + 非同步掃描狀態輪詢 | `internal/handlers/security.go` | W2 |
| kube-bench Job 管理（建立 Job → 等待 → 解析結果） | `internal/services/bench_service.go` | W3–W4 |
| Gatekeeper 儀表板（違規統計，利用 CRD 介面） | `ui/src/pages/security/GatekeeperDashboard.tsx` | W4 |
| 前端安全儀表板（掃描結果 + 基準分數 + 嚴重漏洞列表） | `ui/src/pages/security/SecurityDashboard.tsx` | W5–W6 |

- [ ] ImageScanResult 資料模型
- [ ] Trivy 映像掃描整合（支援 exec 呼叫與 Trivy Server API 兩種模式）
- [ ] 非同步掃描任務管理（觸發 → 輪詢狀態 → 結果儲存）
- [ ] CIS kube-bench 評分（在叢集建立 Job → 解析輸出 → 儲存評分）
- [ ] Gatekeeper 違規統計儀表板（利用 CRD 介面）
- [ ] 前端安全儀表板（漏洞嚴重度分佈 + 基準評分 + 詳情 Drawer）
- [ ] 三語 i18n

**完成指標：** 可對指定工作負載觸發映像掃描並檢視 CVE 列表；kube-bench 可顯示 PASS/FAIL 統計。

---

#### Milestone 10：備份匯出 + CLI 工具（4 週）🔲 待實作

> **目標：** Phase 1 輕量備份不依賴外部工具；CLI 工具支援 CI/CD 整合。

| 任務 | 檔案 | 週次 |
|------|------|------|
| 命名空間資源 ZIP 匯出 API | `internal/handlers/backup.go` | W1 |
| Velero 狀態偵測 + Backup/Restore CRD 管理 | `internal/handlers/velero.go` | W1–W2 |
| 前端備份頁（匯出按鈕 + Velero 備份列表） | `ui/src/pages/backup/BackupPage.tsx` | W2 |
| CLI 框架（cobra + viper，`cmd/cli/main.go`） | `cmd/cli/` | W2–W3 |
| CLI 指令實作（login/cluster/pod/helm/yaml apply/cost） | `cmd/cli/commands/` | W3–W4 |
| GitHub Actions workflow + Release 編譯 | `.github/workflows/release-cli.yml` | W4 |

- [ ] 工作負載配置 ZIP 匯出（無外部依賴，Phase 1）
- [ ] Velero 整合（偵測安裝狀態 + Backup/Restore CRD CRUD，Phase 2）
- [ ] 前端備份管理頁（匯出按鈕 + Velero 備份列表 + 還原觸發）
- [ ] CLI 工具框架（cobra + viper，獨立二進位）
- [ ] CLI 核心指令（login / cluster / pod / helm / yaml / cost）
- [ ] CI/CD 整合文件 + GitHub Release 自動編譯

**完成指標：** `kubepolaris pod list --cluster prod` 可正常輸出；ZIP 匯出包含所有 Deployment/Service/ConfigMap YAML。

---

## 附錄 A：技術選型備選

| 需求 | 第一選擇 | 備選 | 備註 |
|------|---------|------|------|
| 欄位加密 | AES-256-GCM（自實作） | `age`、Vault Transit | 依部署環境選擇 |
| 狀態管理 | @tanstack/react-query | SWR | React Query 生態更完整 |
| 拓撲圖 | ReactFlow v12 | @antv/g6 | ReactFlow 對 React 整合更佳，內建 Dagre 佈局 |
| 日誌系統 | `slog`（標準庫） | zap | Go 1.21+ slog 是官方解 |
| 追蹤 | OpenTelemetry | Jaeger SDK | OTel 為業界標準 |
| Helm | helm.sh/helm/v3 | — | 官方 SDK，唯一選擇 |
| 成本圖表 | recharts | Ant Design Charts | recharts 體積小、API 簡潔 |
| 映像掃描 | Trivy CLI / Server | Grype | Trivy 生態完整，支援 OCI / 映像 / 設定掃描 |
| K8s 基準評估 | kube-bench（Job 模式） | kube-hunter | kube-bench 對應 CIS Benchmark，業界標準 |
| CLI 框架 | cobra + viper | urfave/cli | cobra 生態最大，kubectl/helm 皆採用 |
| ZIP 打包 | `archive/zip`（標準庫） | — | 無需外部依賴 |

---

## 附錄 B：反思總結

KubePolaris 在功能廣度上已達到相當完整的 MVP 水準，具備與 Rancher、Kuboard 競爭的基礎。然而，以下三點是達到企業生產級標準的關鍵差距：

1. **安全性：** 憑證明文儲存是目前最大的阻礙，必須在推廣給更多使用者前解決。
2. **可規模化：** Informer 架構在叢集數量超過 30 個後會面臨記憶體壓力，需設計懶載入與回收機制。
3. **功能深度：** Helm 管理、成本分析是企業用戶最常見的需求，補足這兩項可大幅提升實用性。

平台的技術選型（Go + React + 單一二進位）是正確的，降低了部署複雜度，是重要的競爭優勢，應持續強化這個特性（例如：零設定的 SQLite 模式更完善）。
