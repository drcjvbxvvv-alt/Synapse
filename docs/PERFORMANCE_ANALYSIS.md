# Synapse 性能分析文件

> 版本：v1.3 | 日期：2026-04-14 | 狀態：P0/P2 已實作（P0-3/4 待 CICD；P2-12 Config-only）
> 範圍：現有系統瓶頸 + CICD 架構（M13-M17）引入後的衝擊評估

---

## 目錄

1. [前提條件與假設](#1-前提條件與假設)
2. [現有性能瓶頸清單](#2-現有性能瓶頸清單)
3. [現有架構可支撐的場景](#3-現有架構可支撐的場景)
4. [CICD 引入後的性能衝擊](#4-cicd-引入後的性能衝擊)
5. [修復優先順序](#5-修復優先順序)
6. [參考指標基準](#6-參考指標基準)

---

## 1. 前提條件與假設

### 資料庫

**SQLite 已棄用，生產環境強制使用 MySQL。**

SQLite 因單一連線限制（`SetMaxOpenConns(1)`）無法支撐任何並發寫入場景，在 CICD
工作負載下更是完全不可用。以下所有分析均以 MySQL 為基準，
MySQL 連線池設定為 `MaxIdleConns=10`、`MaxOpenConns=100`。

### 分析環境假設

| 項目 | 基準值 |
|------|--------|
| 叢集數量 | 10-50 個 |
| 同時在線使用者 | 50-200 人 |
| MySQL 連線池 | 10 idle / 100 open |
| 部署模式 | 單副本（預設） |
| K8s API Server | 正常負載 |

---

## 2. 現有性能瓶頸清單

### 2.1 資料庫層

#### B-DB-1：`username` 欄位缺少索引
- **位置**：`internal/models/user.go`
- **問題**：Auth 查詢以 `username` 為條件但無索引，導致全表掃描
- **影響**：中 — 使用者數超過 1,000 筆時每次登入明顯變慢
- **修復**：`gorm:"uniqueIndex"` 加在 `Username` 欄位

#### B-DB-2：預設叢集權限 N+1 查詢 ✅ 已修復（P2）
- **位置**：`internal/services/permission_service.go`（`GetUserAllClusterPermissions()`）
- **問題**：legacy DB path 執行 4 條 DB round-trip（userGroups、permissions、allClusters、user），
  且 userGroups 查詢的 error 被靜默忽略；allClusters 拉取全部欄位
- **影響**：中 — 叢集數量與使用者數增長時記憶體線性膨脹，且 4 條序列查詢累積延遲
- **修復**：
  - 使用 GORM subquery 將 userGroups + permissions 合併為單一查詢（4→3 round-trips）
  - `allClusters` 改為 `Select("id, name, version, status, ...")` 減少資料傳輸
  - 所有查詢加入 `WithContext(ctx)` 支援 tracing 與 cancellation
  - 修復 userGroups 查詢 error 被靜默忽略的問題

#### B-DB-3：Helm UpgradeRelease 逐 Repo 查詢
- **位置**：`internal/services/helm_service.go:248-254`
- **問題**：升級前逐一確認 Chart 是否存在於各 Repo，N 個 Repo → N 次查詢
- **影響**：低-中 — Helm 操作低頻，但 Repo 數量多時明顯
- **修復**：批次查詢或在記憶體中過濾

### 2.2 Kubernetes 客戶端層

#### B-K8S-1：Discovery API 呼叫未快取 ✅ 已修復
- **位置**：`internal/k8s/manager.go`（`cachedHasArgoRollouts()`）
- **問題**：每次叢集 Informer 初始化都呼叫 `ServerGroupsAndResources()`
  偵測 Argo Rollouts CRD，結果未快取。大型叢集 CRD 多時此呼叫耗時 5-15 秒
- **影響**：中 — 多叢集環境下重啟或 Informer 重建時阻塞時間可觀
- **修復**：`ClusterInformerManager` 新增 `discoveryCache map[uint]discoveryEntry`，
  TTL 5 分鐘。新增 `InvalidateDiscoveryCache(clusterID)` 供叢集更新時主動失效

#### B-K8S-2：Informer 首次 sync 最長等待 30 秒
- **位置**：`internal/k8s/manager.go:160-196`
- **問題**：`waitForSync()` 阻塞呼叫端，直到 Informer 完成或 timeout
- **影響**：低 — 有 timeout 保護，不影響後續請求，屬可接受設計
- **狀態**：已知限制，暫不修復

### 2.3 HTTP 層

#### B-HTTP-1：HTTP Server 缺少逾時設定 ✅ 已修復
- **位置**：`main.go:135`
- **問題**：未設定 `ReadTimeout`、`WriteTimeout`、`IdleTimeout`，
  慢速客戶端可長期佔用連線
- **影響**：中 — 高並發下連線耗盡風險
- **修復**：`ReadTimeout=30s`、`WriteTimeout=120s`（配合 AI SSE 最長逾時）、
  `IdleTimeout=120s`

#### B-HTTP-2：API 端點缺少 Rate Limit ✅ 已修復（P0）
- **位置**：`internal/middleware/api_rate_limit.go`
- **問題**：Rate Limit 僅保護登入失敗（暴力破解防護），
  所有 API 端點無流量保護，高頻請求無背壓
- **影響**：中 — 惡意或異常客戶端可打爆昂貴端點（如 Overview、多叢集查詢）
- **修復**：
  - 新增 `APIRateLimit(name, maxPerMin)` 滑動視窗中介層（1 分鐘視窗，per-user/per-IP）
  - 全域保護：`protected` 群組套用 300 req/min（`internal/router/router.go`）
  - AI 端點強化：`/:clusterID/ai` 群組額外套用 30 req/min（`routes_system_ai.go`）
  - 待 CICD M14 Webhook 端點實作時，加入 per-repo 100 req/min 的獨立桶（沿用同框架）

#### B-HTTP-3：Rate Limiter Memory backend 不支援多副本
- **位置**：`internal/middleware/rate_limiter_memory.go:31-72`
- **問題**：Memory backend 使用 `sync.Mutex`，每次 auth 操作串行化；
  多副本部署時各副本狀態不一致（同一 IP 在不同副本有不同的失敗計數）
- **影響**：低-中 — 單副本可接受；多副本部署需切換 Redis backend
- **修復**：設定 `RATE_LIMITER_BACKEND=redis`（已支援，僅需設定）

### 2.4 AI / 串流層

#### B-AI-1：SSE channel buffer 過小 ✅ 已修復（P0 完整修復）
- **位置**：`internal/services/ai_provider.go`
- **問題**：SSE 事件 channel buffer 固定為 64 個事件。
  工具呼叫回傳大型結果時 sender 阻塞，工具執行被迫暫停等待客戶端消費
- **影響**：中 — 大型 Log 分析或 CI 即時 Log 串流場景下必定觸發
- **修復**：
  - P1：buffer 從 64 提升至 512
  - P0：buffer 進一步提升至 1024；加入背壓降級機制——
    content-only chunk（無 FinishReason、無 ToolCalls）buffer 滿時以非阻塞 `default:` 跳過，
    確保客戶端過慢時 goroutine 不死鎖；critical event（FinishReason、ToolCalls、Done、Error）
    仍使用阻塞 select 保證必達

#### B-AI-2：大型工具結果滯留記憶體 ✅ 已修復
- **位置**：`internal/handlers/ai_chat.go`（`truncateToolResult()`）
- **問題**：工具執行結果（可能 >1MB）在 sanitize 前完整存於記憶體，
  10 個並發 AI 工作階段同時處理大型 Log 時記憶體壓力明顯
- **影響**：中 — 並發 AI 工作階段多時記憶體峰值可達數百 MB
- **修復**：`truncateToolResult()` 保留前 500 行 + 後 100 行，超過 512KB 時
  位元組層級截斷，插入截斷提示讓 LLM 知道內容已省略

### 2.5 並發層

#### B-CONC-1：多叢集 Overview 無限 goroutine
- **位置**：`internal/services/overview_service.go:554-600`
- **問題**：對每個叢集啟動一個 goroutine 拉取監控資料，無上限控制。
  50 個叢集 → 50 個 goroutine 同時活躍
- **影響**：中 — goroutine 數量與叢集數線性增長，記憶體與排程器競爭加劇
- **修復**：加入 worker pool（建議上限 10-20 個 worker）

#### B-CONC-2：Log Aggregator 無限 goroutine
- **位置**：`internal/services/log_aggregator.go:43-52`
- **問題**：同 B-CONC-1，每個 Log 目標啟動一個 goroutine
- **影響**：中
- **修復**：同 B-CONC-1，加入 worker pool

#### B-CONC-3：Helm SearchCharts 串行 HTTP 請求 ✅ 已修復（P2）
- **位置**：`internal/services/helm_service.go`（`SearchCharts()`）
- **問題**：搜尋 Chart 時對每個 Repo 串行拉取 `index.yaml`，
  N 個 Repo → N 個阻塞 HTTP 請求。10 個 Repo 時搜尋可能需要 5-10 秒
- **影響**：中 — Repo 數量增加時體感明顯
- **修復**：改為並行 goroutine，semaphore 上限 5 個並發 HTTP 請求；
  `sync.WaitGroup` 等待所有結果後合併，單一 Repo 失敗不影響其他

#### B-CONC-4：Helm 每次請求建立新 HTTP Client ✅ 已修復（P2）
- **位置**：`internal/services/helm_service.go`（`fetchRepoIndex()`、`downloadChart()`）
- **問題**：`fetchRepoIndex()` 和 `downloadChart()` 每次建立新的 `&http.Client{}`，
  無法複用連線池，產生不必要的 TLS handshake
- **影響**：低 — Helm 操作低頻，影響有限
- **修復**：新增 package-level singleton `helmHTTPClient`（30s timeout、MaxIdleConnsPerHost=5），
  兩處 HTTP 呼叫均改用此 singleton

---

## 3. 現有架構可支撐的場景

```
✅ 舒適運行（無明顯感知延遲）
   ├─ 叢集數：1-20 個
   ├─ 每叢集 Pod 數：< 500
   ├─ 同時在線使用者：< 50 人
   ├─ AI Chat 並發工作階段：5-10 個
   └─ Helm Repo 數：< 5 個

⚠️ 邊緣可用（部分功能有感知延遲，不崩潰）
   ├─ 叢集數：20-50 個（Overview 頁刷新 3-8 秒）
   ├─ 同時在線使用者：50-100 人
   ├─ Helm Repo 數：5-15 個（Chart 搜尋 3-10 秒）
   └─ AI Chat 並發工作階段：10-20 個（SSE buffer 偶爾阻塞）

❌ 超出承受範圍（系統不穩定或崩潰）
   ├─ 叢集數：> 50 個並發 Overview 刷新
   ├─ 同時在線使用者：> 200 人（無 API Rate Limit）
   ├─ 大量 Webhook 入站（無保護）
   └─ AI Chat 並發工作階段：> 30 個（記憶體 + SSE buffer 雙重壓力）
```

---

## 4. CICD 引入後的性能衝擊

### 4.1 新增壓力來源概覽

```
現有系統壓力模型（讀多寫少）：
  K8s API  ──── [Informer Cache] ──── 前端
  MySQL    ──── [低頻寫入]      ──── 叢集管理 / 使用者設定

CICD 引入後（讀寫混合，高頻寫入）：
  K8s API  ──── [Informer Cache + 高頻 Job CRUD] ──── 前端
  MySQL    ──── [高頻寫入：Run / Step / Artifact]──── Pipeline 引擎
  SSE      ──── [Log 串流 × 並發 Pipeline 數]    ──── 前端 Console
  Webhook  ──── [burst 入站，無限流控]            ──── Pipeline 觸發
```

### 4.2 K8s Job 寫入壓力（M13a）

每條 Pipeline Run 的 K8s 操作：

```
每條 Pipeline × 每個 Step：
  ├─ POST   /apis/batch/v1/namespaces/{ns}/jobs        （創建 Job）
  ├─ GET    /apis/batch/v1/namespaces/{ns}/jobs/{name} （輪詢狀態）
  ├─ GET    /api/v1/namespaces/{ns}/pods/{name}/log    （讀取 Log）
  └─ DELETE /apis/batch/v1/namespaces/{ns}/jobs/{name} （清理）

並發場景估算：
  10 Pipeline 並發 × 5 Steps = 50 個 Job 同時創建
  → 50 條 Log 串流同時打開
  → K8s API Server QPS 峰值：50-200 req/s
```

> **風險**：目前 K8s Client 無針對高頻寫入的批次策略或 retry backoff，
> API Server 過載時無降級保護。

### 4.3 資料庫寫入頻率（M13a / M13b）

```
現有寫入 QPS（估算）：< 10 req/s
CICD 後寫入 QPS（估算）：

  每次 Pipeline 觸發（1 次）：
    INSERT pipeline_runs              × 1
    INSERT pipeline_step_runs         × N steps
    UPDATE pipeline_step_runs         × N steps × 2（started + finished）
    INSERT pipeline_artifacts         × M artifacts
    INSERT audit_logs                 × N steps

  10 Pipeline 並發 × 5 Steps × 4 次 DB 操作：
    峰值寫入 QPS ≈ 200 req/s

  高峰（100 Pipeline 並發）：
    峰值寫入 QPS ≈ 2,000 req/s
```

> MySQL 100 連線池在 200 QPS 下充裕；2,000 QPS 需要連線池調整
> 或加入寫入佇列（write queue）。

### 4.4 SSE 串流壓力（M13a / M13b）

| 場景 | 現有 | CICD 後 |
|------|------|---------|
| SSE 用途 | AI Chat 文字（幾百 bytes/event） | AI Chat + Pipeline Log（每行 log 1 event） |
| 並發 SSE 連線 | 5-20 個 | 5-20（AI）+ 10-50（Pipeline Log） |
| 單連線 event 速率 | ~5 events/s | Log 密集輸出可達 100 events/s |
| 64-event buffer 撐得住嗎 | ✅ | ❌ 必定阻塞 |

> **P0 問題**：SSE buffer 必須在 CICD 上線前提升，
> 並加入背壓降級（buffer 滿時跳過非關鍵 event 或通知客戶端）。

### 4.5 Webhook 入站壓力（M14）

```
GitHub push event → N 個符合條件的 Pipeline 觸發

場景：monorepo 有 10 個 Pipeline 監聽同一 branch
  → 1 次 push = 10 條 Pipeline 同時啟動
  → 10 × 5 Steps = 50 個 K8s Job 瞬間創建

現有保護：無 Webhook Rate Limit，無 Pipeline 並發上限
風險：CI/CD abuse 或 webhook storm 直接打爆系統
```

### 4.6 Goroutine 爆炸風險（M13a）

若 CICD Pipeline 引擎沿用目前「per-item goroutine」模式：

```
現有 goroutine 壓力：
  50 叢集 × 1 goroutine = 50 goroutines（Overview）

CICD 後 goroutine 壓力（最壞情況）：
  100 Pipeline 並發 × 5 Steps × 1 goroutine = 500 goroutines
  + 50 叢集 Overview                         =  50 goroutines
  + 50 SSE Log 串流                          =  50 goroutines
  ──────────────────────────────────────────────────────────
  總計                                        ≈ 600 goroutines 同時活躍
```

> Go 的 goroutine 記憶體初始約 8KB，600 goroutines ≈ 4.8MB（可接受），
> 但 goroutine 排程競爭和各自持有的 K8s/DB 連線才是真正瓶頸。
> 必須使用 worker pool 限制實際並發數。

### 4.7 衝擊摘要表

| 壓力來源 | 現有影響 | CICD 後影響 | 嚴重度變化 |
|----------|----------|-------------|-----------|
| DB 寫入 QPS | < 10 | 峰值 200-2,000 | 低 → **高** |
| K8s API 寫入 | 幾乎無 | 50-200 req/s（並發 Job） | 無 → **高** |
| SSE 串流壓力 | 低 | 高（Log 密集輸出） | 低 → **高** |
| Goroutine 數量 | 50 | 500-600 | 中 → **高** |
| Webhook 入站 | 無 | burst 無保護 | 無 → **高** |
| Memory 峰值 | ~100MB | ~400-800MB（估算） | 低 → **中** |

---

## 5. 修復優先順序

### P0 — CICD 上線前必須完成

| # | 項目 | 相關瓶頸 | 預估工時 | 狀態 |
|---|------|---------|---------|------|
| 1 | SSE buffer 提升至 1024，加背壓降級（content chunk 可丟、critical event 阻塞保達） | B-AI-1 | 2h | ✅ 已實作 |
| 2 | API Rate Limit 中介層（全域 300/min、AI 30/min；Webhook per-repo 100/min 待 M14） | B-HTTP-2 | 2h | ✅ 已實作（Webhook 待 M14） |
| 3 | Pipeline goroutine 使用 worker pool（上限可設定，預設 20） | B-CONC-1 | 3-4h | ⏳ 待 CICD M13a 實作時加入 |
| 4 | K8s Job 操作加入 retry with exponential backoff | 新增 | 2h | ⏳ 待 CICD M13a 實作時加入 |

### P1 — 上線後一個月內

| # | 項目 | 相關瓶頸 | 預估工時 | 狀態 |
|---|------|---------|---------|------|
| 5 | K8s Discovery 結果快取（TTL 5 分鐘） | B-K8S-1 | 1-2h | ✅ 已實作 |
| 6 | HTTP Server 加 ReadTimeout / WriteTimeout / IdleTimeout | B-HTTP-1 | 30min | ✅ 已實作 |
| 7 | ~~`username` 欄位加 DB unique index~~ | B-DB-1 | — | ✅ 早已存在（`uniqueIndex` 於 models/user.go:12） |
| 8 | 大型工具結果截斷（前 500 行 + 後 100 行，上限 512KB） | B-AI-2 | 1h | ✅ 已實作 |
| 8a | SSE channel buffer 64 → 512 | B-AI-1（部分） | 30min | ✅ 已實作 |
| 9 | Pipeline 並發上限設定（預設 50，可設定） | 新增 | 1h | ⏳ 待 CICD M13a 實作時加入 |

### P2 — 下一個迭代

| # | 項目 | 相關瓶頸 | 預估工時 | 狀態 |
|---|------|---------|---------|------|
| 10 | Helm SearchCharts 改為並行 HTTP 請求（semaphore=5） | B-CONC-3 | 2h | ✅ 已實作 |
| 11 | 預設叢集權限合併 userGroups subquery、WithContext、Select 最小欄位 | B-DB-2 | 2h | ✅ 已實作 |
| 12 | Rate Limiter 切換 Redis backend | B-HTTP-3 | Config | ℹ️ Config-only：設定 `RATE_LIMITER_BACKEND=redis`（已支援） |
| 13 | Helm HTTP Client 改 singleton（30s timeout） | B-CONC-4 | 30min | ✅ 已實作 |

---

## 6. 參考指標基準

### 目標 SLA（CICD 上線後）

| 指標 | 目標 |
|------|------|
| Pipeline 觸發到首個 Step 啟動 | < 3 秒 |
| Step Log 首行出現在前端 | < 2 秒 |
| 並發 Pipeline 支撐數（預設） | 50 條 |
| Webhook 入站 Rate Limit | 100 req/min per repo |
| K8s API 錯誤率（429 Too Many Requests） | < 1% |
| DB 寫入 p99 延遲 | < 100ms |
| SSE 連線維持時間 | Pipeline 執行全程 + 30s |

### MySQL 連線池建議調整（CICD 上線後）

```
現有：MaxIdleConns=10, MaxOpenConns=100
建議：MaxIdleConns=25, MaxOpenConns=200
理由：CICD 高峰寫入 QPS 200+ 時，100 連線可能不足以覆蓋
     Pipeline Step 狀態更新的並發需求
```

---

*本文件由 Claude Code 根據程式碼靜態分析生成，數字為估算值，實際性能需壓力測試驗證。*
