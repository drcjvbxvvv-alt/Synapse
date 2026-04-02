# Synapse 系統規劃書

> 版本：v1.3 | 日期：2026-04-02 | 狀態：進行中

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

**已完成（2026-04-02 §8.3 Phase A）：**
- ✅ HPA CRUD（A1）— `internal/handlers/hpa.go`、路由、`ScalingTab.tsx` 完整建立/編輯/刪除流程
- ✅ YAML Apply Diff 顯示（A2）— 確認 `YAMLEditor.tsx` 與 `ResourceYAMLEditor.tsx` 已完整實作 Monaco DiffEditor 差異預覽
- ✅ Slack / Teams 通知（A3）— `event_alert_service.go` 新增 Slack text + Teams Adaptive Card 格式
- ✅ Argo Rollouts 操控（A4）— `rollout.go` 新增 Promote / PromoteFull / Abort / GetAnalysisRuns，`RolloutDetail.tsx` 新增操作按鈕

**未完成（下一批次）：**
- Helm Release 管理（M4）
- §8.3 Phase B：Loki/ES 查詢整合、ConfigMap/Secret 版本歷史、ResourceQuota/LimitRange CRUD

---

## 目錄

1. [系統現況總覽](#1-系統現況總覽)
2. [已知缺陷與技術債](#2-已知缺陷與技術債)
3. [邊界天花板分析](#3-邊界天花板分析)
4. [解決方案與優化計劃](#4-解決方案與優化計劃)
5. [新功能規劃](#5-新功能規劃)
6. [優先序與里程碑](#6-優先序與里程碑)
7. [平台演進方向：全能 CI/CD DevOps 平台](#7-平台演進方向全能-cicd-devops-平台)
8. [系統反思：不足之處與強化方向](#8-系統反思不足之處與強化方向)

---

## 1. 系統現況總覽

Synapse 是以 Go 1.25（Gin）+ React 19（Ant Design）構建的企業級 Kubernetes 多叢集管理平台。後端以單一二進位檔嵌入前端靜態資源，支援 SQLite（開發）與 MySQL 8（生產）雙資料庫，整合 Prometheus / Grafana / AlertManager / ArgoCD，提供 Web Terminal（Pod Exec、kubectl、Node SSH）。

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
  你是 Synapse K8s 查詢助手。
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

### 5.9 📦 備份與災難恢復（附加項，非核心路線）

**重新評估（2026-04-02）：**

- **Phase 1（ZIP 匯出）已移除：** M16 原生 GitOps 落地後，所有工作負載配置的「來源」即是 Git Repo，從叢集反向匯出 YAML 的需求消失。GitOps 本身就是最好的「備份」。
- **Phase 2（Velero）保留為附加項：** Velero 解決的是**有狀態資料備份**（PVC 資料、資料庫快照），與 GitOps 不重疊，仍有價值，但不列入核心里程碑，改為 M16 完成後的可選整合。

**保留範圍（Velero，附加）：**
- 偵測叢集是否已安裝 Velero（查詢 `velero` namespace）
- 透過 K8s CRD 管理 Velero `Backup` / `Schedule` / `Restore` 資源（複用 CRD 通用介面）
- 備份列表 + 狀態 + 觸發還原

```
GET /clusters/:id/backup/velero-status    偵測 Velero 安裝狀態
```

**etcd 快照：** 僅限 Self-managed 叢集，優先級最低，暫不實作。

**實作難度：** ⭐⭐（Velero 附加整合）
**前置條件：** M16 原生 GitOps 完成後再評估是否實作

---

### 5.10 🖥️ CLI 工具（延後，待 M13–M16 穩定後重新定義）

**重新評估（2026-04-02）：**

CLI 工具的方向正確，但**現在規劃範圍錯誤**，原始設計僅涵蓋管理功能（cluster/pod/helm/cost），未考慮即將實作的 DevOps 能力。

**延後理由：**
- M13（CI Pipeline）、M14（Git 整合）、M16（GitOps）完成前，CLI 的核心使用場景（`pipeline run`、`deploy`、`env promote`）尚未存在
- 在功能介面未穩定時設計 CLI 必然導致大幅重工
- CLI 應在 DevOps 功能穩定後一次設計完整指令集

**未來範圍（M13–M16 完成後重新規劃）：**
```
synapse login --server https://... --token <token>

# 管理
synapse cluster list
synapse pod list [--namespace <ns>] [--cluster <id>]
synapse helm list / upgrade / rollback

# CI/CD（M13+ 後新增）
synapse pipeline list
synapse pipeline run <name> [--cluster <id>]
synapse pipeline logs <run-id>

# GitOps（M16+ 後新增）
synapse app list
synapse app sync <name>
synapse app diff <name>

# 環境（M17+ 後新增）
synapse env list
synapse env promote <app> --from staging --to production
```

**技術方案（不變）：** `cobra` + `viper`，獨立 Go 二進位，`~/.synapse/config.yaml`

**前置條件：** M16 完成後重新規劃並實作
**估計工作量：** 4 週（屆時範圍更大，涵蓋 DevOps 全指令）

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
| M7 | **AI 深度運維**（NL Query / YAML 生成 / Runbook） | ✅ 已完成 | 中 | — |
| M8 | **多叢集工作流程**（遷移 / 配置同步） | ✅ 已完成 | 低 | **5 週** |
| M9 | **合規性與安全掃描**（Trivy / kube-bench） | ✅ 已完成 | 低 | **6 週** |
| M10 | ~~備份匯出 + CLI 工具~~ → **Velero 附加整合**（ZIP 移除；CLI 延後至 M16 後） | ⏸ 延後 | 低 | **重新評估中** |
| M11 | **NetworkPolicy 拓撲內聯編輯 + 策略模擬**（移除拖拉建立，保留編輯現有規則 + YAML 預覽 + 模擬） | 🔲 待實作 | 中 | **2 週** |
| M12 | **Service Mesh 視覺化（Istio 拓撲 + 流量管理）** | 🔲 待實作 | 中 | **5 週** |
| — | NetworkPolicy 視覺化拓撲圖 + 精靈 | ✅ 已完成 | 中 | **3 週** |
| **M13** | **原生 CI Pipeline 引擎**（K8s Job 驅動，DAG 步驟，日誌串流） | 🔲 待實作 | 🔴 高 | **8 週** |
| **M14** | **Git 整合 + Webhook 觸發**（GitHub/GitLab/Gitea） | 🔲 待實作 | 🔴 高 | **4 週** |
| **M15** | **映像 Registry 整合**（Harbor/DockerHub，Tag 管理） | 🔲 待實作 | 🟡 中 | **3 週** |
| **M16** | **原生輕量 GitOps**（Kustomize/Helm，ArgoCD 整合升級） | 🔲 待實作 | 🟡 中 | **6 週** |
| **M17** | **環境管理 + Promotion 流水線**（dev/staging/prod，人工審核閘門） | 🔲 待實作 | 🟢 低 | **5 週** |

**待實作總估計（含 DevOps 演進）：約 59 週（~15 個月）**
（M10 ZIP 移除 -1 週；CLI 延後不計；M11 縮減 -1 週）

### 建議實作順序

```
影響度  ↑
高      │ ✅ M1 安全    ✅ M4 Helm
        │ ✅ M2 效能    ✅ M5 AI/CRD/NP/告警
        │
中      │ ✅ M3 可觀測  ✅ M6 成本分析
        │ ✅ M7 AI 深度   🔲 M11 NP 視覺化編輯+模擬（次優先）
        │               🔲 M12 Service Mesh（Istio 拓撲+流量管理）
低      │               🔲 M8 多叢集
        │               ⏸ M10 延後重定義
        │               ✅ M9 合規掃描
        └─────────────────────────────────→ 實作難度
                  低          中          高
```

**推薦下一步：M11（NetworkPolicy 拓撲內聯編輯 + 策略模擬）**
- 縮減後僅 2 週，建立於已有 ReactFlow 基礎設施之上，邊際成本低
- 策略模擬（Apply 前預覽）是明確差異化功能，Wizard + 拓撲圖已夠用不需再做拖拉建立
- M12（Service Mesh）依賴 Istio 部署，建議 M11 後再評估目標用戶的 Istio 採用率
- 長期優先：M13 CI Pipeline 是平台演進為 DevOps 平台的最關鍵缺口

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

#### Milestone 7：AI 深度運維（4 週）✅ 已完成

> **目標：** 從「AI 輔助診斷」升級為「AI 主動運維助手」，支援自然語言查詢與 YAML 生成。

- [x] 敏感資料過濾（`internal/services/ai_sanitizer.go`：Secret data/stringData 值、含 password/token/key 的 env var 值、PEM 憑證 → `[REDACTED]`；在 AI Chat 工具呼叫結果回傳 AI 前自動過濾）
- [x] NL Query 端點（`POST /clusters/:id/ai/nl-query`；AI 解析自然語言 → 選擇並呼叫最適工具 → 執行查詢 → AI 摘要；`internal/handlers/ai_nlquery.go`）
- [x] YAML 生成助手（AI Chat 系統 Prompt 新增 YAML 生成指示；輸入 `/yaml` 前綴觸發；`AIChatMessage.tsx` 自動偵測 yaml 程式碼區塊並顯示「複製 YAML」與「套用至叢集」按鈕）
- [x] Runbook 知識庫（`internal/runbooks/runbooks.json`，10 個常見場景嵌入二進位；`GET /ai/runbooks?reason=OOMKilled` 支援關鍵字搜尋）
- [x] Runbook 自動附加（`AIChatPanel.tsx` 在 AI 診斷回應完成後偵測 OOMKilled / CrashLoopBackOff / ImagePullBackOff 等關鍵字，自動呼叫 Runbook API 並在訊息下方以 Collapse 展開步驟）
- [x] AI Chat 系統 Prompt 改為繁體中文；前端 `/query` 指令開啟 NL Query Modal
- [x] `AIChatInput.tsx` 新增指令快捷標籤（`/yaml`、`/query`）與輸入提示

**完成指標：** 輸入「列出所有 OOMKilled 的 Pod」可正確回傳結果；YAML 生成輸出可直接複製或一鍵跳轉 YAML Apply 頁面；診斷到 CrashLoopBackOff 時自動附帶 Runbook。

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

#### Milestone 9：合規性與安全掃描（6 週）✅ 已完成

> **目標：** 提供叢集安全基線評估，協助企業滿足 SOC2 / 等保合規要求。

| 任務 | 檔案 | 週次 |
|------|------|------|
| `ImageScanResult` 資料模型 | `internal/models/security.go` | W1 |
| Trivy 整合（exec 模式 + Server 模式） | `internal/services/trivy_service.go` | W1–W2 |
| 映像掃描 API + 非同步掃描狀態輪詢 | `internal/handlers/security.go` | W2 |
| kube-bench Job 管理（建立 Job → 等待 → 解析結果） | `internal/services/bench_service.go` | W3–W4 |
| Gatekeeper 儀表板（違規統計，利用 CRD 介面） | `internal/services/gatekeeper_service.go` | W4 |
| 前端安全儀表板（掃描結果 + 基準分數 + 嚴重漏洞列表） | `ui/src/pages/security/SecurityDashboard.tsx` | W5–W6 |

- [x] ImageScanResult / BenchResult 資料模型
- [x] Trivy 映像掃描整合（exec 模式，非同步 goroutine）
- [x] 非同步掃描任務管理（觸發 → 輪詢狀態 → 結果儲存）
- [x] CIS kube-bench 評分（在叢集建立 Job → 解析輸出 → 儲存評分）
- [x] Gatekeeper 違規統計儀表板（利用 CRD 介面，dynamic client）
- [x] 前端安全儀表板（三分頁：漏洞掃描 / CIS 基準 / Gatekeeper）
- [x] 三語 i18n（zh-TW / en-US / zh-CN）

**完成指標：** 可對指定工作負載觸發映像掃描並檢視 CVE 列表；kube-bench 可顯示 PASS/FAIL 統計；Gatekeeper 違規可動態列出。

---

#### ~~Milestone 10：備份匯出 + CLI 工具~~ ⏸ 延後重新定義

> **重新評估結論（2026-04-02）：** 原始 M10 範圍已拆分重組，不再作為獨立里程碑實作。

| 子項目 | 新去向 | 理由 |
|--------|--------|------|
| ZIP 備份匯出 | ❌ 移除 | M16 GitOps 落地後，Git 即是備份來源，ZIP 匯出需求消失 |
| Velero 整合 | ⏸ M16 後附加實作 | 解決有狀態資料（PVC）備份，與 GitOps 不重疊，但非核心路線 |
| CLI 工具 | ⏸ M16 後重新規劃 | 原始指令集未涵蓋 Pipeline/Deploy/Promote，M13–M16 穩定後一次設計完整版 |

**Velero 附加整合（M16 後，~2 週）：**
- [ ] Velero 安裝偵測（`GET /clusters/:id/backup/velero-status`）
- [ ] Backup/Restore CRD CRUD（複用 CRD 通用介面）
- [ ] 前端備份狀態頁

**CLI 重新規劃（M16 後，~4 週）：**
- [ ] 完整指令集設計（涵蓋 cluster/pod/helm/pipeline/app/env）
- [ ] cobra + viper 框架，`~/.synapse/config.yaml`
- [ ] GitHub Release 自動編譯（Linux/macOS/Windows）

---

#### Milestone 11：NetworkPolicy 拓撲內聯編輯 + 策略模擬（2 週）🔲 待實作

> **重新評估（2026-04-02）：** 原始「完整視覺化編輯器（拖拉新增節點/連線）」與已有 Wizard 功能重疊，ROI 低。縮減為：**拓撲圖上直接編輯現有規則** + **YAML 預覽** + **Apply 前策略模擬**，工作量從 3 週降為 2 週。

**移除項目及理由：**

| 移除項目 | 理由 |
|---------|------|
| 拖拉新增節點/連線以建立規則 | Wizard 已提供完整的結構化建立流程，兩套建立入口並存增加維護成本 |
| 拓撲圖節點增刪 | 節點由叢集實際 Pod/Namespace 決定，手動增刪無意義 |

**保留並實作：**

| 任務 | 檔案 | 週次 |
|------|------|------|
| 拓撲圖「編輯模式」：點選現有連線修改 port/protocol | `NetworkPolicyTopology.tsx`（擴充） | W1 |
| 規則邊 Modal（點選連線 → 修改 ports / protocol / peer） | `NetworkPolicyEdgeModal.tsx`（新增） | W1 |
| YAML 預覽面板（編輯後即時顯示生成的 NetworkPolicy YAML） | `NetworkPolicyTopology.tsx` | W1 |
| 策略模擬後端（NP selector matching 引擎） | `internal/handlers/networkpolicy.go`（擴充） | W1–W2 |
| 策略模擬前端（來源/目標 label + port → ALLOW/DENY + 決策路徑） | `NetworkPolicySimulator.tsx`（新增） | W2 |
| 三語 i18n | — | W2 |

**後端新增 API：**
```
POST /clusters/:id/networkpolicies/simulate
  Body: { fromPodLabels: {}, toPodLabels: {}, port: 80, protocol: "TCP" }
  Response: {
    allowed: bool,
    reason: "matched_policy" | "default_deny" | "no_policies",
    matchedPolicies: [{ name, namespace, direction, rule }],
    path: [{ from, to, decision }]   // 用於前端高亮路徑
  }
```

**模擬引擎邏輯（自實作，無外部依賴）：**
```go
// 1. 取得叢集所有 NetworkPolicy
// 2. 找出目標 Pod（toPodLabels selector matching）
// 3. 是否有任何 NP 選中目標 Pod？
//    - 無 → default allow（K8s 預設），回傳 ALLOW
//    - 有 → 進入規則評估
// 4. 逐條 ingress rule 評估：peer 是否包含來源 Pod？port 是否匹配？
//    - 有匹配 → ALLOW，附上匹配的策略名稱與規則編號
//    - 無匹配 → DENY（default deny when NP exists）
// 5. 同理評估 egress（來源 Pod 方向）
```

**前端 UI 設計：**
```
┌─────────────────────────────────────────────────────────┐
│  [檢視模式] [編輯模式] [模擬模式]              [儲存] [取消] │
├─────────────────────────────┬───────────────────────────┤
│                             │  ＜編輯模式＞              │
│   ReactFlow 畫布            │  點選連線即可編輯規則        │
│                             │  ┌─────────────────────┐  │
│  [frontend] ──→ [backend]   │  │ Protocol: [TCP ▾]   │  │
│  （點選此連線）               │  │ Port:     [8080   ] │  │
│                             │  │ Direction: Ingress  │  │
│                             │  └─────────────────────┘  │
│                             │  YAML 預覽 ▼              │
│                             │  podSelector:             │
│                             │    matchLabels:           │
│                             │      app: backend         │
└─────────────────────────────┴───────────────────────────┘

＜模擬模式＞
來源 label: [app=frontend]  目標 label: [app=backend]  埠: [8080]  [模擬]
──────────────────────────────────────────────────────────────
✅ 允許  ← 策略 "allow-frontend" (default) Ingress 規則 #1 匹配
   評估策略：[allow-frontend ✅]  [deny-all ❌ 未選中此 Pod]
```

- [ ] 拓撲圖編輯模式（點選連線 → 修改 port/protocol，不做新增節點）
- [ ] 規則邊 Modal（`NetworkPolicyEdgeModal.tsx`）
- [ ] YAML 預覽面板（編輯即時同步，Apply 前確認）
- [ ] `POST /simulate` 後端模擬引擎（selector matching，自實作）
- [ ] 前端策略模擬 UI（`NetworkPolicySimulator.tsx`，ALLOW/DENY + 決策依據）
- [ ] 三語 i18n

**完成指標：** 可在拓撲圖點選連線修改 port 並看到 YAML 即時更新；輸入來源/目標 Pod 標籤模擬後正確顯示 ALLOW/DENY 及決策規則。

---

#### Milestone 12：Service Mesh 視覺化（Istio 拓撲 + 流量管理）（5 週）🔲 待實作

> **目標：** 為已安裝 Istio 的叢集提供服務依賴拓撲圖、即時流量指標視覺化，以及 VirtualService/DestinationRule 的友善管理介面。未安裝 Istio 時優雅降級（僅顯示 K8s 服務依賴推斷）。

**功能範圍：**

| 子功能 | 說明 | 資料來源 |
|--------|------|---------|
| Istio 安裝偵測 | 查詢 `istio-system` 是否存在 istiod Deployment | K8s API |
| 服務拓撲圖 | 服務節點 + 有向呼叫邊（有 Istio 才有邊的流量數據） | Prometheus + K8s |
| 流量指標 | 邊上顯示 RPS / 錯誤率 / P99 延遲 | Prometheus |
| VirtualService 管理 | 列表 + 建立 + 編輯（流量比例滑桿）+ 刪除 | K8s CRD API |
| DestinationRule 管理 | 列表 + 建立 + 編輯（熔斷 / 負載均衡策略表單）+ 刪除 | K8s CRD API |
| Gateway 管理 | 列表 + CRUD（複用 CRD 通用介面，不另開發表單） | K8s CRD API |

**技術方案：**

| 元件 | 方案 | 說明 |
|------|------|------|
| 服務拓撲渲染 | ReactFlow v12（複用已有依賴） | 節點 = Service，邊 = 流量關係 |
| 流量資料 | Prometheus 查詢 `istio_requests_total` | 無 Prometheus 時降級為靜態推斷 |
| CRD 操作 | K8s dynamic client（複用 CRD 通用 handler） | 不硬編碼 Istio CRD schema |
| VirtualService 表單 | 自訂表單（HTTPRoute + 流量比例）| 比 raw YAML 更易用 |
| DestinationRule 表單 | 自訂表單（outlierDetection + loadBalancer）| 熔斷設定視覺化 |

**後端新增 API：**
```
GET  /clusters/:id/service-mesh/status
  Response: { installed: bool, version: string, prometheusAvailable: bool }

GET  /clusters/:id/service-mesh/topology?namespace=&timeRange=5m
  Response: {
    nodes: [{ id, name, namespace, type: "service"|"external", podCount, labels }],
    edges: [{ source, target, rps, errorRate, p99Latency }]
  }

GET  /clusters/:id/service-mesh/virtual-services?namespace=
POST /clusters/:id/service-mesh/virtual-services
PUT  /clusters/:id/service-mesh/virtual-services/:namespace/:name
DELETE /clusters/:id/service-mesh/virtual-services/:namespace/:name

GET  /clusters/:id/service-mesh/destination-rules?namespace=
POST /clusters/:id/service-mesh/destination-rules
PUT  /clusters/:id/service-mesh/destination-rules/:namespace/:name
DELETE /clusters/:id/service-mesh/destination-rules/:namespace/:name
```

**Prometheus 查詢邏輯：**
```go
// services/mesh_service.go

// 取得服務間流量矩陣（5 分鐘滾動視窗）
queryRPS := `sum(rate(istio_requests_total[5m])) by (source_workload, destination_workload, destination_service_namespace)`
queryErrors := `sum(rate(istio_requests_total{response_code=~"5.."}[5m])) by (source_workload, destination_workload)`
queryP99 := `histogram_quantile(0.99, sum(rate(istio_request_duration_milliseconds_bucket[5m])) by (le, source_workload, destination_workload))`

// 無 Prometheus 時 fallback：
// 掃描所有 Service 的 selector，與 Deployment env/volumeMount 中的 Service 名稱比對，推斷靜態依賴
```

**前端新增元件：**
```
ui/src/pages/network/
  ├── ServiceMeshTab.tsx          主頁籤（拓撲圖 + 流量管理入口）
  ├── ServiceTopologyGraph.tsx    ReactFlow 服務拓撲圖（複用 NetworkPolicyTopology 架構）
  ├── VirtualServiceList.tsx      VS 列表 + 建立/編輯 Modal
  ├── VirtualServiceForm.tsx      HTTP Route 流量比例設定表單
  ├── DestinationRuleList.tsx     DR 列表 + 建立/編輯 Modal
  └── meshService.ts              API service
```

**拓撲圖 UI 設計：**
```
┌──────────────────────────────────────────────────────────────────┐
│ 命名空間 [default ▾]  時間範圍 [5m ▾]  [重新整理]  [流量管理 ▾] │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│   [frontend]──3.2 RPS / 0.1% err──▶[backend]──▶[database]      │
│       │                               │                          │
│       └──1.1 RPS / 0%──▶[auth-svc]   └──▶[cache]               │
│                                                                  │
│   ● 節點顏色：綠=健康 / 橙=錯誤率>1% / 紅=錯誤率>5%              │
│   ● 邊粗細：代表 RPS 大小                                        │
│   ● 點選節點：顯示該服務的 VS/DR 設定                            │
└──────────────────────────────────────────────────────────────────┘
```

**VirtualService 流量比例表單（金絲雀發布場景）：**
```
服務: [reviews ▾]   Host: reviews

HTTP Route 規則：
  ┌─────────────────────────────────────────┐
  │ Destination: reviews-v1   Weight: [70]% │
  │ Destination: reviews-v2   Weight: [30]% │
  │ [+ 新增 Destination]                    │
  └─────────────────────────────────────────┘
超時設定: [5s]   重試次數: [3]
```

**實作任務：**

| 任務 | 檔案 | 週次 |
|------|------|------|
| Istio 偵測 + Prometheus 查詢（RPS/錯誤率/P99） | `internal/services/mesh_service.go` | W1 |
| Topology API + 靜態推斷 fallback | `internal/handlers/mesh.go` | W1–W2 |
| VirtualService / DestinationRule CRUD API | `internal/handlers/mesh.go` | W2 |
| 前端服務拓撲圖（ReactFlow，節點顏色/邊粗細代表指標） | `ServiceTopologyGraph.tsx` | W3 |
| 前端 VS 流量比例表單（金絲雀發布場景） | `VirtualServiceForm.tsx` | W3–W4 |
| 前端 DR 熔斷設定表單 | `DestinationRuleList.tsx` | W4 |
| 未安裝 Istio 時降級提示 UI | `ServiceMeshTab.tsx` | W4 |
| 三語 i18n | — | W5 |

- [ ] Istio 安裝偵測（`GET /service-mesh/status`）
- [ ] Prometheus 查詢流量矩陣（RPS / 錯誤率 / P99 延遲）
- [ ] 靜態服務依賴推斷（無 Prometheus fallback）
- [ ] VirtualService CRUD + 流量比例表單
- [ ] DestinationRule CRUD + 熔斷策略表單
- [ ] Gateway 管理（複用 CRD 通用介面）
- [ ] 前端服務拓撲圖（ReactFlow，節點/邊指標視覺化）
- [ ] 未安裝 Istio 優雅降級
- [ ] 三語 i18n

**完成指標：** 已安裝 Istio 的叢集可顯示服務依賴拓撲並標示 RPS；可透過表單設定 VirtualService 流量比例（金絲雀發布）；未安裝 Istio 的叢集顯示靜態推斷拓撲而非空白頁。

---

---

## 7. 平台演進方向：全能 CI/CD DevOps 平台

> **戰略目標：** 從「K8s 多叢集管理工具」演進為「端到端 CI/CD DevOps 平台」，使 Synapse 具備與 GitLab CI + ArgoCD + Rancher 組合相競爭的完整能力，且以單一二進位、零外部依賴為核心競爭優勢。

### 7.1 現況差距分析

| 能力維度 | 現況 | 差距 |
|---------|------|------|
| GitOps / CD | 代理外部 ArgoCD（需額外安裝） | 無原生 CD，強依賴外部 |
| CI Pipeline | **完全沒有** | 最大缺口 |
| Git 整合 | 無 | 無 Webhook、無 Repo 連結 |
| 映像建置 / Registry | 無 | 無 Build 能力、無 Registry 管理 |
| 環境流水線 | 僅 Namespace 粒度 | 無 dev → staging → prod 概念 |
| Secret / 環境變數管理 | K8s Secret CRUD | 無跨環境注入、無 Vault 整合 |
| 部署策略 | Argo Rollouts（基礎） | 藍綠/金絲雀不完整 |
| 稽核 / 合規 | 操作日誌 | 無 Pipeline 執行稽核 |

### 7.2 架構路線選擇

三條可行路線，各有取捨：

| 路線 | 說明 | 優點 | 缺點 |
|------|------|------|------|
| **A. 整合路線** | 接入 Tekton / Jenkins / GitLab CI 等成熟工具 | 開發量低，功能成熟 | 部署複雜度高，外部依賴多 |
| **B. 原生路線** | 自研 Pipeline 引擎，完全內建 | 體驗一致，單一二進位優勢最大化 | 工作量極大，需維護 Pipeline runtime |
| **C. 混合路線（建議）** | 原生輕量 Pipeline + 保留 ArgoCD/Tekton 作進階選項 | 快速交付核心場景，進階用戶可擴充 | 需設計良好的抽象層 |

**建議採用路線 C**：先做原生輕量 Pipeline 覆蓋 80% 使用場景（Build → Push → Deploy），進階場景透過插件接入 Tekton/Jenkins。

### 7.3 目標平台架構

```
┌─────────────────────────────────────────────────────────────────┐
│                        Synapse DevOps Platform                  │
├────────────┬────────────┬────────────┬────────────┬────────────┤
│  程式碼    │  CI        │  映像      │  CD        │  運行時    │
│  管理      │  Pipeline  │  Registry  │  GitOps    │  管理      │
│            │            │            │            │            │
│ Git Webhook│ Pipeline   │ Harbor/    │ 原生 GitOps│ 現有 K8s   │
│ Repo 連結  │ 定義 (YAML)│ Docker Hub │ + ArgoCD   │ 管理功能   │
│ PR 觸發    │ 步驟引擎   │ 整合       │ 整合       │            │
│ 分支策略   │ 日誌串流   │ 漏洞掃描   │ 同步狀態   │            │
└────────────┴────────────┴────────────┴────────────┴────────────┘
         ↑                      ↑                      ↑
    Git Provider            K8s Job                ArgoCD /
  (GitHub/GitLab/          (Pipeline               原生 GitOps
    Gitea)                  Runtime)
```

### 7.4 Phase 1：CI Pipeline 引擎（核心，M13）

> **目標：** 讓使用者可以在 Synapse 定義並執行 CI Pipeline，不需要安裝任何額外工具。

**設計策略：** 以 K8s Job / Pod 作為 Pipeline 的執行單元，Pipeline 定義儲存在 Synapse DB，執行時動態建立 K8s Job。

**資料模型：**
```go
// internal/models/pipeline.go

type Pipeline struct {
    ID          uint
    Name        string
    ClusterID   uint
    Namespace   string
    Description string
    Trigger     string      // "manual" | "webhook" | "schedule"
    GitRepo     string      // https://github.com/org/repo
    GitBranch   string      // main / feature/* (glob)
    StepsJSON   string      // JSON 序列化的 []PipelineStep
    EnvVarsJSON string      // JSON 序列化的環境變數（加密）
    Enabled     bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type PipelineStep struct {
    Name       string            // "build"
    Image      string            // "docker:24"
    Commands   []string          // ["docker build -t ...", "docker push ..."]
    Env        map[string]string // 步驟專屬環境變數
    DependsOn  []string          // 前置步驟（DAG）
    Timeout    int               // 秒，0 = 不限
    RetryCount int
}

type PipelineRun struct {
    ID         uint
    PipelineID uint
    Status     string    // "pending" | "running" | "success" | "failed" | "cancelled"
    TriggerBy  string    // "manual:user_id" | "webhook:sha" | "schedule"
    GitSHA     string
    GitBranch  string
    StartedAt  time.Time
    FinishedAt *time.Time
    Duration   int       // 秒
}

type StepRun struct {
    ID            uint
    PipelineRunID uint
    StepName      string
    Status        string
    JobName       string   // K8s Job 名稱
    LogPodName    string   // 用於串流日誌
    StartedAt     time.Time
    FinishedAt    *time.Time
    ExitCode      *int
}
```

**後端 Pipeline 執行引擎：**
```go
// internal/services/pipeline_runner.go

// 1. 建立 PipelineRun 記錄
// 2. 解析步驟 DAG（依賴關係）
// 3. 按拓撲序依次提交 K8s Job：
//    - Job spec 包含：image、command、env、resource limits
//    - 掛載 workspace PVC（步驟間共享產物）
//    - 注入 Synapse 系統環境變數（SYNAPSE_COMMIT_SHA、SYNAPSE_BRANCH 等）
// 4. Watch Job 狀態，即時更新 StepRun
// 5. Job 完成後串流 Pod 日誌至 DB（或保留 Pod 供即時串流）
// 6. 所有步驟成功 → PipelineRun.Status = "success"
//    任一步驟失敗 → 取消後續步驟，Status = "failed"
```

**API 端點：**
```
GET    /pipelines                          Pipeline 列表
POST   /pipelines                          建立 Pipeline
GET    /pipelines/:id                      Pipeline 詳情（含步驟定義）
PUT    /pipelines/:id                      更新 Pipeline
DELETE /pipelines/:id                      刪除 Pipeline
POST   /pipelines/:id/run                  手動觸發執行
GET    /pipelines/:id/runs                 執行歷史列表
GET    /pipelines/:id/runs/:runId          執行詳情（含步驟狀態）
GET    /pipelines/:id/runs/:runId/steps/:step/logs  步驟日誌（支援 SSE 串流）
POST   /pipelines/:id/runs/:runId/cancel   取消執行中的 Run
```

**前端頁面：**
```
ui/src/pages/pipeline/
  ├── PipelineList.tsx         Pipeline 列表（狀態燈、最後執行時間）
  ├── PipelineEditor.tsx       Pipeline 定義編輯器（步驟卡片 + YAML 雙模式）
  ├── PipelineRunList.tsx      執行歷史列表
  ├── PipelineRunDetail.tsx    執行詳情（DAG 進度圖 + 步驟狀態）
  ├── StepLogViewer.tsx        步驟日誌串流（SSE，複用 Terminal 樣式）
  └── pipelineService.ts
```

**Pipeline 執行詳情 UI：**
```
執行 #42  main @ a3f9c12  ⏱ 3m 21s  ✅ 成功

[clone] ──▶ [test] ──▶ [build] ──▶ [push] ──▶ [deploy]
  ✅ 12s      ✅ 48s     ✅ 1m32s    ✅ 28s      ✅ 21s

▼ build 步驟日誌
  Step 1/4 : FROM golang:1.25-alpine
  Step 2/4 : WORKDIR /app
  ...
```

### 7.5 Phase 2：Git 整合與 Webhook 觸發（M14）

> **目標：** 連結 Git Provider，實現 Push/PR 事件自動觸發 Pipeline。

**支援 Git Provider：**
- GitHub（App / PAT）
- GitLab（Webhook Token）
- Gitea（自架，優先支援，符合私有部署場景）
- Bitbucket（後續）

**資料模型：**
```go
type GitProvider struct {
    ID          uint
    Name        string   // "公司 GitLab"
    Type        string   // "github" | "gitlab" | "gitea"
    BaseURL     string   // https://gitlab.company.com
    Token       string   // 加密儲存（AES-256-GCM）
    WebhookSecret string // HMAC 驗證 secret
    CreatedAt   time.Time
}
```

**Webhook 流程：**
```
Git Push → POST /webhooks/:provider/:token
  → 驗證 HMAC signature
  → 解析 event type（push / pull_request / tag）
  → 比對 Pipeline 的 GitRepo + GitBranch（glob）
  → 建立 PipelineRun（TriggerBy="webhook:sha"，帶 GitSHA/Branch）
  → 回傳 202 Accepted
```

**API 端點：**
```
GET    /git-providers                  Git Provider 列表
POST   /git-providers                  新增
PUT    /git-providers/:id              更新
DELETE /git-providers/:id              刪除
POST   /git-providers/:id/test         測試連線
GET    /git-providers/:id/repos        列出可用 Repo（供 Pipeline 編輯器選擇）
POST   /webhooks/:providerType/:secret 接收 Webhook（公開端點，無需 Auth）
```

### 7.6 Phase 3：映像 Registry 整合（M15）

> **目標：** 管理 Container Registry，Pipeline 建置後自動推送，並在工作負載詳情頁顯示映像漏洞資訊。

**支援 Registry：**
- Harbor（企業私有，首選）
- Docker Hub
- 阿里雲 / AWS ECR / GCR（透過標準 Docker Registry API）

**功能範圍：**
- Registry 連線設定（URL / 帳密 / TLS）
- Repository 瀏覽（Repo 列表 → Tag 列表 → Manifest 詳情）
- 映像 Tag 管理（刪除舊 Tag、設定保留策略）
- 漏洞掃描觸發（對接 Harbor 內建掃描 或 Trivy Server）
- Pipeline 步驟中自動注入 Registry 憑證（`imagePullSecret`）

```go
type ContainerRegistry struct {
    ID         uint
    Name       string
    Type       string   // "harbor" | "dockerhub" | "ecr" | "generic"
    URL        string
    Username   string
    Password   string  // 加密儲存
    Insecure   bool    // 允許自簽憑證
    IsDefault  bool    // Pipeline 預設 push 目標
}
```

### 7.7 Phase 4：原生 GitOps / CD（M16）

> **目標：** 提供不依賴 ArgoCD 的輕量 GitOps 能力，同時保留 ArgoCD 整合作為進階選項。

**兩層設計：**

**Layer 1 — 輕量 GitOps（內建，無外部依賴）：**
- 定義「Application」：Git Repo + 路徑 + 目標叢集/命名空間
- 定期（或 Webhook 觸發）比對 Git 中的 YAML 與叢集實際狀態（Diff）
- 自動同步（Apply）或僅通知差異（Drift Detection）
- 支援 Kustomize overlay（疊加 base + overlays/prod）
- 支援 Helm Chart（結合 M4 已有能力）

```go
type GitOpsApp struct {
    ID              uint
    Name            string
    ClusterID       uint
    Namespace       string
    GitProviderID   uint
    RepoURL         string
    RepoPath        string   // manifests/prod/
    TargetRevision  string   // main / v1.2.3 / HEAD
    ToolType        string   // "raw" | "kustomize" | "helm"
    HelmValuesPath  string   // 僅 helm 模式
    SyncPolicy      string   // "manual" | "auto"
    PruneResources  bool     // 是否刪除 Git 中移除的資源
    LastSyncAt      *time.Time
    LastSyncStatus  string
    DriftDetected   bool
}
```

**Layer 2 — ArgoCD 深度整合（現有代理模式升級）：**
- 現有 ArgoCD 代理 API 保留
- 新增 ArgoCD App Health 聚合到 Synapse 主儀表板
- Pipeline 部署步驟可選「觸發 ArgoCD Sync」作為部署方式

### 7.8 Phase 5：環境管理與部署流水線（M17）

> **目標：** 建立 dev → staging → prod 的環境概念，支援跨環境的 Promotion 流程。

**環境（Environment）模型：**
```go
type Environment struct {
    ID          uint
    Name        string   // "dev" | "staging" | "production"
    ClusterID   uint
    Namespace   string
    Type        string   // "development" | "staging" | "production"
    Order       int      // Promotion 順序（0=dev, 1=staging, 2=prod）
    AutoPromote bool     // 上一環境成功後自動 Promote
    RequireApproval bool // 部署前需人工審核
    Labels      string   // JSON，用於資源識別
}
```

**Promotion 流程：**
```
Pipeline 執行成功
  → 部署到 dev 環境
  → 自動（或等待審核）Promote to staging
  → 執行 smoke test（可選 Pipeline 步驟）
  → 人工審核（Production Gate）
  → 部署到 production
```

**前端環境流水線視圖：**
```
myapp 部署流水線

dev          staging       production
[健康 ✅]  →  [等待審核 ⏳]  →  [未部署 ⬜]
v1.2.3        v1.2.2           v1.2.1
              [審核通過] [拒絕]
```

### 7.9 Gap 總結與里程碑規劃

| 里程碑 | 功能 | 優先級 | 預估工作量 | 依賴 |
|--------|------|--------|-----------|------|
| **M13** | **原生 CI Pipeline 引擎**（K8s Job 驅動，手動觸發，DAG 步驟，日誌串流） | 🔴 高 | 8 週 | — |
| **M14** | **Git 整合 + Webhook 觸發**（GitHub/GitLab/Gitea，Push/PR 事件驅動 Pipeline） | 🔴 高 | 4 週 | M13 |
| **M15** | **映像 Registry 整合**（Harbor/DockerHub，Tag 管理，Pipeline 推送憑證注入） | 🟡 中 | 3 週 | M13 |
| **M16** | **原生輕量 GitOps**（Git Repo → K8s Diff + Apply，Kustomize/Helm 支援，ArgoCD 整合升級） | 🟡 中 | 6 週 | M14 |
| **M17** | **環境管理 + Promotion 流水線**（dev/staging/prod 環境概念，人工審核閘門） | 🟢 低 | 5 週 | M13, M16 |

**DevOps 演進總估計：約 26 週（6.5 個月）**

### 7.10 實作建議順序

```
現在（管理平台）
    │
    ▼
M13 CI Pipeline 引擎（最大差距，優先補足）
    │  ↳ 使用者可在平台內定義並執行 Build/Test/Deploy 步驟
    ▼
M14 Git Webhook 整合（自動化觸發，CI 才有完整價值）
    │  ↳ Push 到 main → 自動觸發 Pipeline → 部署到 dev
    ▼
M15 Registry 整合（Pipeline 產物管理）
    ▼
M16 原生 GitOps（CD 能力內建化，擺脫 ArgoCD 強依賴）
    ▼
M17 環境流水線（企業級多環境管理，Promotion Gate）
    │
    ▼
目標（全能 CI/CD DevOps 平台）
```

**此時 Synapse = Rancher（叢集管理）+ GitLab CI（Pipeline）+ ArgoCD（GitOps）合一，以單一二進位交付。**

---

## 8. 系統反思：不足之處與強化方向

> **反思基準（2026-04-02）：** 系統目前有 47 個後端 handler、85+ 個前端頁面，功能廣度已達 MVP 水準。本章從「做了但沒做好」與「完全缺失但高需求」兩個維度進行反思，並提出強化方向。

### 8.1 「有做但沒做好」— 深度不足的功能

#### 🔴 HPA 只能看，不能管

**現況：** `GetDeploymentHPA` 僅列出現有 HPA 設定，無法新增、修改、刪除。
**問題：** 用戶看到 HPA 卻無法調整 min/max replicas 或 target CPU，必須回到 kubectl 操作，破壞平台的完整性。
**強化方向：**
- HPA CRUD（建立、編輯、刪除）
- 視覺化 HPA 狀態（目前副本數 vs 目標，Scaling 事件時間軸）
- 支援自定義指標（Custom Metrics）

---

#### 🔴 Loki / Elasticsearch 設定存在但無實際查詢整合

**現況：** `LogSourceConfig` 模型支援 `loki` / `elasticsearch` 類型，但後端僅儲存連線設定，並未實作對這些系統的查詢邏輯。前端 LogCenter 顯示的是 K8s 容器日誌串流，不是集中式日誌查詢。
**問題：** 生產環境普遍使用 Loki 或 ES 集中管理日誌，現況等同於「假整合」— 設定了但什麼都做不了。
**強化方向：**
- 實作 Loki HTTP API 查詢（`/loki/api/v1/query_range`，LogQL 支援）
- 實作 Elasticsearch Query DSL 基礎查詢
- LogCenter 統一入口：K8s 串流 + Loki + ES 三個來源切換
- 關鍵字搜尋、時間範圍篩選、正規表達式過濾

---

#### 🔴 Argo Rollouts 管理功能淺薄

**現況：** Rollout 列表/詳情/YAML/刪除/縮放，以及觀看 Pod/Service/Ingress/Events，但缺乏核心操控能力。
**問題：** Argo Rollouts 的核心價值在於金絲雀/藍綠部署的**逐步推進與回滾控制**，現有介面沒有這些操作。
**強化方向：**
- Promote（推進到下一步）/ Full Promote（一次推進到底）
- Abort（中止金絲雀，回滾到 stable）
- Analysis Run 狀態視覺化（自動化指標分析結果）
- 部署進度時間軸（每一步的推進時間、指標狀態）

---

#### 🟡 通知系統只覆蓋 K8s Event 告警，部署操作無通知

**現況：** Event Alert Rules 可觸發 Webhook / DingTalk / Email，但僅針對 K8s Events。
**問題：** 用戶最需要的通知場景（部署成功/失敗、擴縮容、安全掃描結果、Pipeline 完成）完全沒有。
**強化方向：**
- 通知觸發點擴充：部署操作、安全掃描、成本超標、節點異常
- 渠道新增：**Slack**（國際市場必備）、**Microsoft Teams**（企業市場）
- 通知規則引擎：指定叢集/命名空間/嚴重程度 → 路由到不同渠道

---

#### 🟡 多租戶沒有「租戶」實體，隔離依賴人工維護

**現況：** 多租戶透過 `ClusterPermission`（叢集 + 命名空間 + 權限等級）實現，沒有明確的租戶/組織層級。
**問題：** 規模稍大時，管理員需手動逐一設定每個用戶的每個叢集權限，無法批次管理。沒有自助式命名空間申請。
**強化方向：**
- 引入 **Project（專案）** 概念：一個 Project 對應一組 叢集+命名空間+成員
- Project 管理員可自助管理成員和配額
- 命名空間自助申請流程（Dev 申請 → 管理員審核 → 自動建立 + 配額）

---

#### 🟡 ResourceQuota / LimitRange 缺乏 CRUD

**現況：** Namespace 詳情頁展示配額用量，但無法建立或修改 `ResourceQuota` 與 `LimitRange`。
**問題：** 多租戶環境的核心管控工具無法在平台內操作。
**強化方向：**
- ResourceQuota / LimitRange 的建立、編輯、刪除
- 配額用量視覺化（已用 / 總量 進度條，按 CPU/Memory/PVC 分類）
- 配額超限預警（接近 80% 時告警）

---

#### 🟡 YAML Apply 沒有 Dry-run / Diff 預覽

**現況：** `YAMLEditor` 直接 Apply，無法預覽變更影響。
**問題：** 生產環境直接 Apply 風險高，用戶需要先知道「這個 YAML 會改變什麼」。
**強化方向：**
- Apply 前執行 `kubectl diff`（Server-side dry-run）
- 顯示 Diff（新增/修改/刪除的欄位）
- 可選：Apply 前需確認 diff 結果

---

#### 🟢 Terminal 會話只記錄指令，沒有錄製回放

**現況：** `TerminalSession` + `TerminalCommand` 記錄執行的指令文字，但無法重現操作過程。
**強化方向：** 基於 `asciinema` 格式錄製 Terminal 輸出，支援會話回放（安全稽核重要場景）

---

### 8.2 「完全缺失但高需求」— 重要功能空白

#### 🔴 無 OAuth2 / OIDC 整合

**現況：** 僅支援本地帳號與 LDAP。
**影響：** 無法接入 Google Workspace、GitHub、GitLab、Keycloak、Okta 等主流 SSO 方案，企業導入門檻高。
**方案：** 整合 OIDC（OpenID Connect），一套接入所有支援 OIDC 的 IdP：
```go
// internal/auth/oidc.go
// 依賴 golang.org/x/oauth2 + coreos/go-oidc
// 配置：ClientID, ClientSecret, IssuerURL, RedirectURL
// 流程：Browser → /auth/oidc/login → IdP → /auth/oidc/callback → JWT
```

---

#### 🔴 無部署審批工作流

**現況：** 任何有權限的用戶都可直接部署到生產環境，無審核機制。
**影響：** 生產環境變更缺乏管控，無法滿足 SOC2、ISO 27001 變更管理要求。
**方案：**
- 對特定命名空間（如 `production`）標記為「需要審批」
- 部署操作觸發審批請求（Approval Request）
- 審批人收到通知 → 平台內批准/拒絕
- 審批記錄納入稽核日誌
- 可設定審批逾時自動拒絕

---

#### 🔴 無 VPA（Vertical Pod Autoscaler）支援

**現況：** 只有 HPA 唯讀檢視。
**影響：** 無法幫助用戶識別資源 requests/limits 設定不合理的工作負載（過高浪費、過低 OOM）。
**方案：**
- VPA 物件 CRUD（若叢集已安裝 VPA controller）
- Recommendation 顯示（VPA 建議的 CPU/Memory requests）
- 與成本分析（M6）整合：VPA 建議 + 成本影響估算

---

#### 🔴 無跨叢集統一工作負載視圖

**現況：** 每個功能都需先選擇叢集，無法跨叢集搜尋「所有叢集中名為 api-server 的 Deployment」。
**影響：** 管理 10+ 個叢集時，排查問題需逐一切換叢集，效率極低。
**方案：**
- 全域工作負載搜尋（搜尋 Deployment/Pod/Service 名稱，跨所有叢集）
- 統一儀表板：「異常工作負載」跨叢集聚合視圖
- 現有 `QuickSearch` / `GlobalSearch` 已有基礎，需擴充至工作負載維度

---

#### 🟡 無 Port-Forward 功能

**現況：** 提供 Web Terminal，但沒有 Port-Forward（將本地埠轉發到 Pod 埠）。
**影響：** 調試服務時，用戶只能進入 Terminal 用 curl，無法直接在瀏覽器存取 Pod 的 HTTP 服務。
**方案：**
- 後端建立 K8s Port-Forward tunnel，透過 WebSocket 代理給瀏覽器
- 前端顯示「轉發地址」，用戶可直接點開
- 使用場景：直接在瀏覽器預覽 Pod 內的 Web UI（Prometheus、Grafana、自訂服務）

---

#### 🟡 無 ConfigMap / Secret 版本歷史

**現況：** ConfigMap/Secret CRUD 完整，但修改後無法查看歷史版本或回滾。
**影響：** 配置變更是常見故障來源，無歷史記錄意味著無法快速回滾。
**方案：**
- 每次 Update 前儲存舊版本快照至 DB（`config_history` 表）
- 版本列表 + Diff 視圖（新舊版本並排比較）
- 一鍵回滾到指定版本

---

#### 🟡 無 PodDisruptionBudget 管理

**現況：** 無 PDB 相關功能。
**影響：** 節點維護（drain）時可能意外終止過多 Pod，PDB 是防護關鍵，但用戶無法在平台內設定。
**方案：** PDB CRUD + 在 Deployment 詳情頁顯示關聯的 PDB 狀態

---

#### 🟡 無 Image Tag 管理

**現況：** 工作負載詳情顯示當前映像，但無法跨叢集查詢「哪個叢集在跑 nginx:1.21」。
**影響：** 漏洞修補時，無法快速定位受影響的工作負載。
**方案：**
- 映像索引：定期掃描所有工作負載的 container image，存入 DB
- 全域映像搜尋：輸入 `nginx:1.21` → 顯示所有使用此映像的工作負載（跨叢集）
- 與安全掃描（M9）整合：掃描結果直接關聯到使用該映像的工作負載

---

#### 🟡 無 Deployment 保護機制

**現況：** 無任何防止誤操作的機制。
**影響：** 生產環境誤刪 Deployment 或誤縮容至 0，無任何阻擋。
**方案：**
- 命名空間保護標記（標記 `production` 為受保護）
- 受保護命名空間的刪除/縮容操作需二次確認 + 輸入資源名稱
- 設定「維護窗口」（允許變更的時間段），窗口外的操作被阻擋或需審批

---

#### 🟢 無 SAML 支援

**影響：** 部分老牌企業（銀行、製造業）使用 SAML IdP（AD FS、Shibboleth），OIDC 無法涵蓋。
**方案：** 整合 `crewjam/saml` 套件，提供 SAML SP 功能（優先序低於 OIDC）

---

#### 🟢 無稽核日誌 SIEM 匯出

**現況：** 操作日誌完整但只能在平台內查詢，無法匯出到 Splunk / ELK / Datadog。
**方案：** Webhook 推送模式（每條稽核日誌即時 POST 到外部 SIEM），或批次匯出 JSON

---

### 8.3 強化優先序矩陣

| 優先級 | 功能 | 類別 | 工作量 | 狀態 |
|--------|------|------|--------|------|
| 🔴 P0 | Loki / ES 實際查詢整合 | 深度不足 | 3 週 | Phase B |
| 🔴 P0 | OAuth2 / OIDC 整合 | 完全缺失 | 2 週 | Phase C |
| 🔴 P0 | HPA CRUD | 深度不足 | 1 週 | ✅ **完成** |
| 🔴 P0 | 部署審批工作流 | 完全缺失 | 3 週 | Phase C |
| 🟡 P1 | Argo Rollouts 操控（Promote/Abort/Analysis） | 深度不足 | 2 週 | ✅ **完成** |
| 🟡 P1 | 通知渠道擴充（Slack / Teams） | 深度不足 | 1 週 | ✅ **完成** |
| 🟡 P1 | YAML Apply Dry-run / Diff | 深度不足 | 1 週 | ✅ **完成**（已存在） |
| 🟡 P1 | ConfigMap/Secret 版本歷史 | 完全缺失 | 2 週 | Phase B |
| 🟡 P1 | 跨叢集統一工作負載視圖 | 完全缺失 | 2 週 | Phase B |
| 🟡 P1 | ResourceQuota / LimitRange CRUD | 深度不足 | 1 週 | Phase B |
| 🟡 P1 | VPA 支援 | 完全缺失 | 2 週 | Phase C |
| 🟡 P1 | Image Tag 全域搜尋 | 完全缺失 | 2 週 | Phase C |
| 🟢 P2 | Port-Forward | 完全缺失 | 2 週 | Phase D |
| 🟢 P2 | Project 多租戶模型 | 架構升級 | 4 週 | Phase D |
| 🟢 P2 | Deployment 保護機制 | 完全缺失 | 1 週 | Phase D |
| 🟢 P2 | PodDisruptionBudget 管理 | 完全缺失 | 1 週 | Phase D |
| 🟢 P2 | Terminal 會話錄製回放 | 深度不足 | 2 週 | Phase D |
| 🟢 P2 | 稽核日誌 SIEM 匯出 | 完全缺失 | 1 週 | Phase D |

**Phase A 進度（2026-04-02 完成）：** 4/4 項 ✅

### 8.4 核心反思結論

> 1. **廣度夠但深度不足：** 日誌、HPA、Rollouts、通知等功能都「有做」但停在 MVP 水準，用戶在生產環境使用時很快就會碰壁。
> 2. **認證系統是採用瓶頸：** 只有 LDAP + 本地帳號，現代企業 80% 使用 OIDC，OAuth2/OIDC 是優先度最高的補足項。
> 3. **缺乏生產環境保護機制：** 無審批工作流、無命名空間保護、無變更窗口，對於真正想把 Synapse 用在生產環境的企業是最大的風險點。
> 4. **跨叢集能力是差異化機會：** 多數競品（Kuboard、Lens）是單叢集或弱多叢集設計，統一工作負載視圖、跨叢集映像索引是 Synapse 的獨特競爭點，應加強而非停留在現狀。

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
| NP 策略模擬 | 自實作 Go selector matching | kube-networkpolicies | K8s NP 語義不複雜，自實作可控且無外部依賴 |
| Istio 流量資料 | Prometheus `istio_requests_total` | Kiali API | Prometheus 已為現有依賴；Kiali 需額外部署 |
| Service Mesh 拓撲渲染 | ReactFlow v12（複用現有） | @antv/g6 | 已有 @xyflow/react 依賴，零額外安裝成本 |
| CI Pipeline 執行引擎 | K8s Job（原生，零額外元件） | Tekton Pipelines | K8s Job 已是現有依賴；Tekton 需額外 CRD 安裝 |
| Pipeline 步驟間產物共享 | `emptyDir` / PVC（K8s 原生） | MinIO | 簡單場景用 emptyDir；需持久化時用 PVC |
| Git Provider 整合 | 自實作 Webhook handler | go-github SDK | 各 Provider Webhook 格式差異不大，自實作可控 |
| GitOps Diff 引擎 | `k8s.io/apimachinery` strategic merge | controller-runtime | 輕量場景無需完整 controller 框架 |
| Kustomize 支援 | `sigs.k8s.io/kustomize/api` | shell exec | Go SDK 無需主機安裝 kustomize 二進位 |
| Container Registry | 標準 Docker Registry HTTP API v2 | go-containerregistry | Docker Registry API 通用；Harbor 額外 API 單獨呼叫 |

---

## 附錄 B：反思總結

Synapse 在功能廣度上已達到相當完整的 MVP 水準，具備與 Rancher、Kuboard 競爭的基礎。然而，以下三點是達到企業生產級標準的關鍵差距：

1. **安全性：** 憑證明文儲存是目前最大的阻礙，必須在推廣給更多使用者前解決。
2. **可規模化：** Informer 架構在叢集數量超過 30 個後會面臨記憶體壓力，需設計懶載入與回收機制。
3. **功能深度：** Helm 管理、成本分析是企業用戶最常見的需求，補足這兩項可大幅提升實用性。

平台的技術選型（Go + React + 單一二進位）是正確的，降低了部署複雜度，是重要的競爭優勢，應持續強化這個特性（例如：零設定的 SQLite 模式更完善）。
