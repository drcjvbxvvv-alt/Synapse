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

**未完成（下一批次）：**
- 結構化日誌（slog / JSON）
- 稽核日誌完整查詢
- 完整 API 分頁強制
- 前端 React Query 導入
- 前端列表虛擬捲動
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
- NetworkPolicy 列表與 CRUD
- 流量規則視覺化（拓撲圖，節點為 Pod/Namespace，邊為允許流量）
- 規則建立精靈（選擇 ingress/egress，定義 selector、port、CIDR）
- 規則衝突檢測與警告

**技術方案：** 前端使用 `@antv/g6` 或 `reactflow` 繪製拓撲圖。

---

### 5.3 💰 資源成本分析（中優先）

**背景：** 多叢集環境下，各團隊資源用量不透明，缺乏成本分攤依據。

**功能範圍：**
- 依命名空間 / 工作負載 / 標籤維度的 CPU / Memory 用量統計
- 與雲端定價整合（AWS / GCP / Azure 節點定價表）
- 月度成本趨勢圖
- 資源浪費識別（長期低用量 Deployment）
- 成本報表匯出（CSV / PDF）

**技術方案：** 後端定期從 Prometheus 拉取用量並持久化；成本計算引入可設定的單價。

---

### 5.4 🔔 K8s Event 告警規則（中優先）

**背景：** AlertManager 整合現有，但 K8s Event（OOMKill、ImagePullBackoff、CrashLoopBackoff）需手動查看，無主動通知。

**功能範圍：**
- Event 類型告警規則設定（OOMKill、Node NotReady、PVC Pending 等）
- 告警通知渠道（Email、Webhook、企業微信/釘釘）
- 告警靜默（Silence）管理
- 告警歷史與確認（Acknowledge）機制

**技術方案：** 後端 Event Informer 訂閱 K8s Events，比對規則後觸發通知。

---

### 5.5 🤖 AI 能力升級（中優先）

**背景：** 平台已有 AI Chat WebSocket 端點，但功能尚未完整開放於 UI。

**功能範圍：**

**智慧診斷：**
- Pod CrashLoop / OOMKill 自動分析（取得日誌 + 事件 → AI 分析根因）
- 部署失敗診斷（YAML 錯誤、資源不足、映像問題）
- 叢集健康異常解讀

**智慧運維：**
- 自然語言查詢（「列出所有重啟超過 5 次的 Pod」→ 轉換為 API 呼叫）
- YAML 生成助手（描述需求 → 生成 Deployment YAML）
- Runbook 自動推薦

**AI 提供者：**
- 支援 OpenAI / Azure OpenAI / Claude / 本地 Ollama
- 敏感資料過濾（不傳送 Secret 明文給 AI）

---

### 5.6 📋 CRD 通用管理介面（中優先）

**背景：** 各叢集可能安裝不同 Operator（如 Cert-Manager、Kafka Operator），目前只能透過 YAML 管理。

**功能範圍：**
- 自動發現叢集內所有 CRD
- 通用資源列表（依 CRD 定義產生欄位）
- 詳情頁（JSON Schema 驅動的表單）
- YAML 讀寫支援
- 常用 Operator CRD 預設欄位設定（Cert-Manager Certificate、Kafka 等）

---

### 5.7 🔄 多叢集工作流程（低優先）

**功能範圍：**
- 跨叢集工作負載遷移精靈
- 多叢集配置同步（ConfigMap / Secret 從主叢集推送到多個叢集）
- 跨叢集流量策略（與 Istio / Linkerd 整合）

---

### 5.8 🛡️ 合規性與安全掃描（低優先）

**功能範圍：**
- OPA / Gatekeeper 策略管理介面
- Pod Security Admission 設定
- Trivy 映像掃描整合（列出高危漏洞的工作負載）
- CIS Kubernetes Benchmark 自動評分
- 合規報告生成

---

### 5.9 📦 備份與災難恢復（低優先）

**功能範圍：**
- Velero 整合（備份排程、還原操作）
- etcd 快照管理
- 工作負載設定匯出（YAML bundle）
- 跨叢集還原精靈

---

### 5.10 🖥️ CLI 工具（低優先）

**功能範圍：**
- `kubepolaris` CLI 指令，功能對應 Web UI 操作
- 支援 kubeconfig 自動同步（從 Web 平台匯出叢集憑證）
- 腳本化 CI/CD 場景（`kubepolaris deploy --cluster prod --namespace app`）

---

## 6. 優先序與里程碑

### 優先序矩陣

```
影響度  ↑
高      │ [S1] 憑證加密   [5.1] Helm 管理
        │ [S2] JWT 強化   [5.4] Event 告警
        │
中      │ [4.2] 可觀測性   [5.2] NetworkPolicy
        │ [F1] 指標修正   [5.5] AI 升級
        │
低      │ [A3] Router 拆分 [5.6] CRD 管理
        │ [4.5] DB 改善   [5.7] 多叢集流程
        └─────────────────────────────────→ 實作難度
                  低          中          高
```

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
- [ ] 完整 API 分頁（列表端點統一 page/pageSize/total，待實作）
- [ ] React Query 導入（前端快取，待實作）
- [ ] 大型列表虛擬捲動（待實作）

**完成指標：** 20 叢集 / 5000 Pod 場景下 API P95 < 200ms。

#### Milestone 3：可觀測性（3 週）🔄 部分完成
- [x] 應用 Prometheus metrics endpoint（`/metrics`，`middleware/metrics.go`，記錄 API 延遲、請求數）
- [x] 健康檢查深化（`/readyz` 真實 DB ping，`/healthz` liveness）
- [ ] 結構化日誌（slog / JSON 格式，待實作）
- [ ] 稽核日誌完整查詢（待實作）
- [ ] 錯誤碼化（API error codes，待實作）

**完成指標：** 所有操作可透過日誌全鏈路追蹤。

#### Milestone 4：Helm 管理（6 週）
- [ ] Helm SDK 整合
- [ ] Release 列表 / 詳情頁
- [ ] 安裝 / 升級 / 回滾 / 刪除
- [ ] Chart Repository 管理
- [ ] Values 差異比對

**完成指標：** 可替代 `helm list`、`helm upgrade`、`helm rollback` 日常操作。

#### Milestone 5：AI 與 CRD（8 週）
- [ ] AI 診斷 UI 完整開放
- [ ] 多 AI 提供者設定頁
- [ ] CRD 自動發現與通用列表
- [ ] NetworkPolicy 管理介面
- [ ] Event 告警規則引擎

---

## 附錄 A：技術選型備選

| 需求 | 第一選擇 | 備選 | 備註 |
|------|---------|------|------|
| 欄位加密 | AES-256-GCM（自實作） | `age`、Vault Transit | 依部署環境選擇 |
| 狀態管理 | @tanstack/react-query | SWR | React Query 生態更完整 |
| 拓撲圖 | ReactFlow | @antv/g6 | ReactFlow 對 React 整合更佳 |
| 日誌系統 | `slog`（標準庫） | zap | Go 1.21+ slog 是官方解 |
| 追蹤 | OpenTelemetry | Jaeger SDK | OTel 為業界標準 |
| Helm | helm.sh/helm/v3 | — | 官方 SDK，唯一選擇 |

---

## 附錄 B：反思總結

KubePolaris 在功能廣度上已達到相當完整的 MVP 水準，具備與 Rancher、Kuboard 競爭的基礎。然而，以下三點是達到企業生產級標準的關鍵差距：

1. **安全性：** 憑證明文儲存是目前最大的阻礙，必須在推廣給更多使用者前解決。
2. **可規模化：** Informer 架構在叢集數量超過 30 個後會面臨記憶體壓力，需設計懶載入與回收機制。
3. **功能深度：** Helm 管理、成本分析是企業用戶最常見的需求，補足這兩項可大幅提升實用性。

平台的技術選型（Go + React + 單一二進位）是正確的，降低了部署複雜度，是重要的競爭優勢，應持續強化這個特性（例如：零設定的 SQLite 模式更完善）。
