# Synapse 專案風險評估報告

> **版本**：v1.0  
> **建立日期**：2026-04-17  
> **評估範圍**：`internal/`、`pkg/`、`cmd/`、`main.go`、`go.mod`  
> **評估方法**：靜態程式碼分析、架構圖審查、相依套件稽核

---

## 目錄

1. [執行摘要](#1-執行摘要)
2. [風險矩陣總覽](#2-風險矩陣總覽)
3. [安全風險（Security Risks）](#3-安全風險)
4. [架構風險（Architecture Risks）](#4-架構風險)
5. [運維風險（Operational Risks）](#5-運維風險)
6. [資料風險（Data Risks）](#6-資料風險)
7. [相依套件風險（Dependency Risks）](#7-相依套件風險)
8. [改善建議與優先順序](#8-改善建議與優先順序)
9. [追蹤與複查](#9-追蹤與複查)
10. [Helm v4 升級方案](#10-helm-v4-升級方案)

---

## 1. 執行摘要

Synapse 是一個多叢集 Kubernetes 管理平台，整體安全控制相對成熟（JWT 演算法白名單、欄位加密、TLS 配置化），但在**運維穩健性**與**架構可維護性**存在需要優先處理的風險。

| 風險類別 | 高 | 中 | 低 |
|----------|----|----|-----|
| 安全      | 0  | 2  | 3  |
| 架構      | 1  | 3  | 1  |
| 運維      | 1  | 2  | 1  |
| 資料      | 0  | 2  | 1  |
| 相依套件  | 0  | 2  | 1  |
| **合計**  | **2** | **11** | **7** |

**最高優先改善項目**：
1. 背景 Worker 優雅關閉缺口（`main.go` 未呼叫各 worker 的 `.Stop()`）
2. CORS 萬用字元 `*` 與 `Allow-Credentials: true` 同時啟用
3. `golang.org/x/crypto` 版本過舊（v0.49.0）

---

## 2. 風險矩陣總覽

| # | 風險項目 | 嚴重度 | 可能性 | 優先級 | 狀態 |
|---|----------|--------|--------|--------|------|
| R-01 | 背景 Worker 無優雅關閉 | 🔴 高 | 高 | **P0** | ✅ 已修復 |
| R-02 | CORS 萬用字元 + 憑證 | 🟠 中 | 中 | **P1** | 待修 |
| R-03 | `golang.org/x/crypto` 過舊 | 🟠 中 | 低 | **P1** | 待修 |
| R-04 | `k8s.io/*` 版本落後 | 🟡 低 | 低 | P2 | ✅ 已修復（v0.35.4） |
| R-15 | Helm SDK v3 → v4 升級 | 🟠 中 | 低 | P2 | ⏳ Phase 0 已完成（見第 10 節） |
| R-05 | Pipeline Scheduler 過大（1338 行）| 🟠 中 | 高 | **P1** | ✅ 已修復（拆為 9 檔） |
| R-06 | 多個服務檔案超過 600 行 | 🟡 低 | 中 | P2 | ✅ 已修復（Round 1+2 全 18 檔拆分完畢） |
| R-07 | 多表寫入缺少 Transaction | 🟠 中 | 中 | **P1** | 待修 |
| R-08 | InsecureSkipVerify 未完整配置化 | 🟠 中 | 低 | P2 | 部分已處理 |
| R-09 | Handler 中使用 `context.Background()` | 🟠 中 | 高 | P2 | ✅ 已修復 |
| R-10 | K8s namespace/name 輸入驗證不一致 | 🟡 低 | 中 | P2 | 待修 |
| R-11 | Soft Delete 查詢不一致 | 🟠 中 | 中 | P2 | 待稽核 |
| R-12 | JWT 演算法白名單 | 🟢 低 | 低 | — | ✅ 已控制 |
| R-13 | TLS 配置化政策 | 🟢 低 | 低 | — | ✅ 已控制 |
| R-14 | 欄位加密（Kubeconfig/Token）| 🟢 低 | 低 | — | ✅ 已控制 |

---

## 3. 安全風險

### R-02 — CORS 萬用字元與憑證標頭同時啟用

**嚴重度**：🟠 中  
**檔案**：`internal/middleware/cors.go:24,65`

**問題描述**：

```go
// cors.go:24
c.Header("Access-Control-Allow-Credentials", "true")

// cors.go:65 — 當 CORS_ALLOWED_ORIGINS=* 時
if allowed == "*" || allowed == origin {
    return true
}
```

當環境變數設定 `CORS_ALLOWED_ORIGINS=*` 時，伺服器同時回傳：
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Credentials: true
```

這違反了 [W3C CORS 規範](https://www.w3.org/TR/cors/#resource-requests)：當 `Allow-Credentials: true` 時，`Allow-Origin` 不可為萬用字元 `*`。雖然現代瀏覽器會拒絕此回應，但某些舊版客戶端或代理可能不做檢查，導致跨站憑證外洩。

**改善建議**：

```go
// 在 isOriginAllowed 中拒絕萬用字元（當憑證模式開啟時）
func isOriginAllowed(origin string, allowedOrigins []string) bool {
    if len(allowedOrigins) == 0 {
        return isDevOrigin(origin)
    }
    for _, allowed := range allowedOrigins {
        if allowed == "*" {
            // 萬用字元不允許與 credentials 同時使用
            logger.Warn("CORS: wildcard origin is unsafe with credentials; skipping")
            continue
        }
        if allowed == origin {
            return true
        }
    }
    return false
}
```

---

### R-08 — InsecureSkipVerify 部分使用場景

**嚴重度**：🟠 中（個案評估後為可接受風險）  
**檔案**：`internal/services/alertmanager_service.go:34`、`prometheus_service.go:32`、`argocd_service.go:653`、`k8s_client.go:466`

**問題描述**：

目前有 23 處使用 `InsecureSkipVerify: true`，其中 4 處未透過全域 TLS 政策（`tlsPolicy` 機制），而是直接硬編碼：

| 檔案 | 用途 | 是否有 nolint 說明 |
|------|------|--------------------|
| `alertmanager_service.go:34` | AlertManager HTTP 通訊 | ✅ `#nosec G402` |
| `prometheus_service.go:32` | Prometheus HTTP 通訊 | ✅ `#nosec G402` |
| `argocd_service.go:653` | ArgoCD HTTPS 通訊 | ✅ `#nosec G402` |
| `k8s_client.go:466` | 讀取憑證過期時間 | ✅ `#nosec G402` |

雖然均有說明，但 `alertmanager_service.go` 和 `prometheus_service.go` 的跳過是固定的，無法透過配置關閉，使用者若有內部 CA 將無法正確驗證。

**改善建議**：

將 AlertManager 和 Prometheus 的 TLS 設定讀取自叢集配置，使其與 K8s 客戶端的 `tlsPolicy` 一致，允許使用者提供 CA 憑證。

---

### R-10 — K8s namespace/name 輸入驗證不一致

**嚴重度**：🟡 低  
**檔案**：`internal/handlers/` 多個 handler

**問題描述**：

部分 handler 直接將使用者輸入的 namespace 和 name 傳入 K8s API，缺乏統一的驗證邏輯（如長度限制 253 字元、不含特殊字元）。雖然 K8s API 本身會拒絕非法請求，但錯誤訊息可能洩漏內部細節。

**改善建議**：

在 `internal/handlers/common.go` 建立統一驗證函式：

```go
// validateK8sName 驗證 Kubernetes 資源名稱格式
func validateK8sName(name string) error {
    if len(name) == 0 || len(name) > 253 {
        return fmt.Errorf("name must be 1-253 characters")
    }
    // RFC 1123 subdomain
    if !k8sNameRegex.MatchString(name) {
        return fmt.Errorf("name must match [a-z0-9.-]")
    }
    return nil
}
```

---

### R-12 ✅ — JWT 演算法白名單（已控制）

**嚴重度**：🟢 低（已妥善處理）  
**檔案**：`internal/middleware/auth.go`

實作了演算法白名單（僅允許 HS256）、Token 黑名單、Issuer/Audience 驗證，防止演算法替換攻擊（Algorithm Substitution Attack）。**無需改善**。

---

### R-13 ✅ — TLS 配置化政策（已控制）

**嚴重度**：🟢 低  
**檔案**：`internal/services/k8s_client.go:31-43`

全域 `tlsPolicy`（`strict`/`warn`/`skip`）由叢集模型控制，預設為 `strict`。**無需改善**。

---

### R-14 ✅ — 敏感欄位加密（已控制）

**嚴重度**：🟢 低  
**檔案**：`internal/models/cluster.go`、`pkg/crypto/`

Kubeconfig、SA Token、CA 憑證透過 AES-256-GCM 加密儲存，`json:"-"` 確保不會序列化到 API 回應。**無需改善**。

---

## 4. 架構風險

### R-05 — Pipeline Scheduler 過大（1338 行）

**嚴重度**：🟠 中（架構債）  
**檔案**：`internal/services/pipeline_scheduler.go`（1338 行）

**問題描述**：

`PipelineScheduler` 同時承擔：
1. Pipeline 觸發（cron、webhook、手動）
2. 三層並行限制管理（全局 / per-pipeline / per-cluster）
3. K8s Job 建立與追蹤
4. Rollback 邏輯
5. 通知廣播
6. 恢復邏輯

單一檔案包含 6 個責任，變更時影響面廣，測試難以獨立進行。

**其他超過 600 行的服務檔案**：

| 檔案 | 行數 | 主要責任 |
|------|------|----------|
| `pipeline_scheduler.go` | 1338 | Pipeline 排程與執行 |
| `k8s_client.go` | 1102 | K8s 客戶端 + TLS 政策 |
| `overview_service.go` | 1079 | 多叢集拓樸彙總 |
| `pipeline_step_types.go` | 1000 | 50+ Pipeline 步驟類型定義 |
| `ai_tools.go` | 963 | AI 工具呼叫 + Prompt 組裝 |
| `permission_service.go` | 948 | RBAC + Repository 層遷移中 |
| `argocd_service.go` | 859 | ArgoCD Sync + Repo 管理 |
| `prometheus_service.go` | 857 | Prometheus 查詢 + Dashboard 生成 |

**改善建議**：

分拆 `pipeline_scheduler.go` 為：
- `pipeline_scheduler_core.go` — 觸發邏輯、cron 排程
- `pipeline_scheduler_concurrency.go` — 三層並行限制
- `pipeline_scheduler_executor.go` — K8s Job 建立與執行

---

### R-06 — 服務層過大（整體架構債）

**嚴重度**：🟡 低  
**狀態**：✅ 已修復（Round 1 + Round 2）

#### Round 1（之前修復）
- `pipeline_scheduler.go` 1338 行 → 拆為 9 檔（R-05 作業的一部分）
- Top-3 最大檔拆分（overview_service、ai_tools 初步整理等）

#### Round 2（本次修復，2026-04-17）
共 15 個超過 600 行的服務檔全部拆分完畢：

| 原始檔案 | 原行數 | 拆出子檔案 |
|---|---|---|
| `ai_tools.go` | 963 | `ai_tools_pod.go`, `ai_tools_deploy.go`, `ai_tools_infra.go` |
| `permission_service.go` | 951 | `permission_group.go`, `permission_cluster.go`, `permission_query.go` |
| `argocd_service.go` | 859 | `argocd_apps.go` |
| `prometheus_service.go` | 857 | `prometheus_cluster.go`, `prometheus_workload.go` |
| `resource_service.go` | 854 | `resource_efficiency.go`, `resource_trend.go` |
| `cost_service.go` | 849 | `cost_worker.go` |
| `prometheus_queries.go` | 796 | `prometheus_queries_pod.go`, `prometheus_queries_workload.go` |
| `alertmanager_service.go` | 762 | `alertmanager_config.go` |
| `om_service.go` | 749 | `om_diagnose.go` |
| `grafana_service.go` | 687 | `grafana_dashboards.go` |
| `rbac_service.go` | 685 | `rbac_user.go` |
| `audit_service.go` | 682 | `audit_session.go` |
| `pipeline_job_watcher.go` | 651 | `pipeline_job_watcher_sync.go` |
| `compliance_service.go` | 628 | `compliance_violations.go` |
| `cluster_service.go` | 625 | `cluster_service_stats.go` |

所有拆分均為純程式碼搬移（零邏輯變更），`package services` 不變，無外部 import 變動。`go build ./...` 及 `go test ./internal/services/...` 全部通過。

---

### R-09 — Handler 中使用 `context.Background()`

**嚴重度**：🟠 中  
**狀態**：✅ 已修復（2026-04-17）

**修復內容**（7 處修正，保留 8 處合法用法）：

| 檔案 | 修正內容 |
|---|---|
| `pod_logs.go` | `req.Stream(context.Background())` → 使用已有的 `ctx` |
| `multicluster_sync.go` | `executeSync` 增加 `ctx context.Context` 參數，移除內部的 `ctx := context.Background()` |
| `multicluster_syncpolicy.go` | 更新呼叫端傳入 `ctx` |
| `notify_channel.go` | `sendTestNotification` 增加 `ctx` 參數，從 handler 傳入 `c.Request.Context()` |
| `pipeline_handler.go` | 審計 goroutine 加入 10 秒獨立 timeout（不用請求 ctx，因 handler 返回後 request ctx 即取消） |
| `pipeline_run_handler.go` | 同上 |
| `kubectl_pod_terminal.go` | `ensureKubectlPod` / `waitForPodRunningWithProgress` / `updateLastActivity` 增加 `ctx context.Context` 參數；在 handler 中以 `c.Request.Context()` 建立帶 3 分鐘 timeout 的 setup context |
| `pod_terminal_exec.go` | `tryExecShell` 使用 `session.Context` 作為 parent |

**保留為合法 `context.Background()` 的情形**：
- WebSocket 會話 context（`pod_terminal_ws.go`, `kubectl_terminal_ws.go`, `log_center_stream.go`, `pod_logs.go`）：WebSocket session 需要自己管理生命週期
- 背景 cleanup worker（`kubectl_pod_terminal.go:cleanupIdlePods`）：非請求觸發
- `SendToChannel`：package-level 函式供背景 worker 使用

---

## 5. 運維風險

### R-01 — 背景 Worker 無優雅關閉 🔴

**嚴重度**：🔴 高  
**檔案**：`main.go:170-172`、`internal/router/router.go:260-340`

**問題描述**：

`main.go` 在接收 SIGTERM/SIGINT 後僅呼叫：
```go
k8sMgr.Stop()          // ✅ K8s Informer 已關閉
srv.Shutdown(ctx)      // ✅ HTTP Server 已關閉
tracing.Shutdown(ctx)  // ✅ OTel 已關閉
sqlDB.Close()          // ✅ DB 連線已關閉
```

**但以下 Worker 均有 `.Stop()` 方法卻未被呼叫**：

| Worker | 啟動位置 | Stop() 存在 | 是否被呼叫 |
|--------|----------|-------------|------------|
| `eventAlertWorker` | `router.go:269` | ✅ | ❌ **未呼叫** |
| `costWorker` | `router.go:270` | ✅ | ❌ **未呼叫** |
| `logRetentionWorker` | `router.go:271` | 待確認 | ❌ **未呼叫** |
| `certExpiryWorker` | `router.go:272` | ✅ | ❌ **未呼叫** |
| `imageIndexWorker` | `router.go:273` | ✅ | ❌ **未呼叫** |
| `pipelineScheduler` | `router.go:338` | ✅ | ❌ **未呼叫** |

**潛在影響**：
- Pipeline Scheduler 可能在 mid-flight 時被殺掉，導致 K8s Job 孤立（已建立但無人追蹤）
- Event Alert Worker 可能丟失尚未持久化的告警
- Image Index Worker 可能寫入不完整的索引資料

**改善方案**：

修改 `router.Setup()` 的回傳簽章或透過 `main.go` 收集所有 Stoppable：

```go
// router.go：回傳 Stoppable 列表
type Stoppable interface {
    Stop()
}

func Setup(...) (*gin.Engine, *k8s.ClusterInformerManager, []Stoppable) {
    // ...
    stoppables := []Stoppable{
        eventAlertWorker,
        costWorker,
        certExpiryWorker,
        imageIndexWorker,
        pipelineScheduler,
    }
    return r, k8sMgr, stoppables
}

// main.go：優雅關閉時呼叫所有 Stop()
for _, s := range stoppables {
    s.Stop()
}
```

---

### R-03 — 複合式 Readiness Check 不完整 ✅ 已修復

**嚴重度**：🟠 中  
**檔案**：`internal/router/router.go:104-149`  
**修復日期**：2026-04-17

**修復內容**：

`PipelineScheduler` 新增兩個方法：
- `IsAlive() bool`：透過 `atomic.Bool` 追蹤 `loop()` goroutine 是否存活
- `QueueDepth(ctx) (int64, error)`：查詢排隊中的 Pipeline Run 數量

`/readyz` 增加 `pipeline_scheduler` 檢查項目：

```go
checks["pipeline_scheduler"] = gin.H{
    "status":      "ok",       // "error" 若 loop goroutine 已死亡
    "alive":       true,
    "queue_depth": 0,
}
```

Scheduler loop 死亡 → `/readyz` 回傳 503，K8s 可重啟 Pod。

---

### R-04a — K8s API 無重試機制 ✅ 已修復

**嚴重度**：🟡 低  
**檔案**：`internal/services/k8s_client.go`、`pipeline_job_watcher.go`  
**修復日期**：2026-04-17

**修復內容**：

新增 `internal/services/k8s_retry.go`，提供：
- `isRetryableK8sError(err)` — 判斷是否為可重試的暫時性錯誤（ServerTimeout / TooManyRequests / ServiceUnavailable / InternalError / net.Timeout）
- `k8sRetry[T](ctx, opName, func)` — 使用 `cenkalti/backoff/v5` 指數退避，最多 4 次嘗試（初始 200ms，最大 5s，30s 硬上限）；非可重試錯誤立即回傳（`backoff.Permanent`）

套用至 `pipeline_job_builder.go` 的四個 K8s 寫入呼叫：
- `EnsureRunSecret` → `Secrets.Create`
- `SubmitJob` → `Jobs.Create`
- `EnsureImagePullSecret` → `Secrets.Create`
- `SetSecretOwnerRef` → `Secrets.Get` + `Secrets.Update`

---

## 6. 資料風險

### R-07 — 多表寫入缺少 Transaction ✅ 已修復

**嚴重度**：🟠 中  
**修復日期**：2026-04-17

**調查結果**：風險評估原本列出三個場景；實際調查後確認兩個真實問題，一個誤判：

| 場景 | 狀態 | 說明 |
|------|------|------|
| Pipeline Run + StepRun 建立 | ✅ 已修復 | `executeRunAsync` StepRun 建立迴圈改用 Transaction |
| ConfigVersion MAX(version)+CREATE | ✅ 已修復 | `saveVersion` 兩個語句改用 Transaction |
| Rollout 權重更新 | N/A | `RolloutService` 為純 K8s CRD 操作，無 DB 寫入 |

**修復內容**：

1. **`pipeline_scheduler.go:executeRunAsync`** — StepRun 建立迴圈改用 `db.Transaction()`，確保所有 StepRun 原子性建立，失敗時不留孤立記錄。

2. **`config_version_service.go:saveVersion`** — `SELECT COALESCE(MAX(version),0)+1` 與 `CREATE` 改在同一 Transaction 內執行，防止並發時產生重複版本號。

---

### R-11 — Soft Delete 查詢作用域不一致 ✅ 已修復

**嚴重度**：🟠 中  
**修復日期**：2026-04-17  
**統計**：39 個檔案使用軟刪除模式

**調查結果**：

1. **`db.Raw()` 呼叫**（`cost_service.go`）：查詢對象為 `resource_snapshots`，該 Model 無 `DeletedAt` 欄位，無軟刪除洩漏風險。
2. **`Preload()` 行為**：GORM v2 會在 Preload 的目標 Model 上自動套用軟刪除 Scope，不需額外處理。
3. **`IsDeleted bool` 模型**：全庫搜尋後無此模式，評估文件描述的風險不存在。
4. **`WithContext(ctx)` 缺失**：`permission_service.go` 的 legacy DB 路徑有多處查詢未傳遞 context，已全部修正。

**修復內容**：

- `permission_service.go`：所有 legacy `*gorm.DB` 路徑查詢補上 `.WithContext(ctx)`，包含：
  - `DeleteUserGroup`、`GetUserGroup`、`ListUserGroups`、`CreateClusterPermission`、`UpdateClusterPermission`
  - `ListClusterPermissions`、`ListAllClusterPermissions`、`GetUserClusterPermission`、`getDefaultPermission`
  - `GetUserAccessibleClusterIDs`、`BatchDeleteClusterPermissions`、`ListUsers`、`GetUser`
- 新增稽核測試（`permission_service_softdelete_test.go`）6 個測試：
  - `TestListUsers_SoftDeleteFilter`、`TestListUsers_ExcludesSoftDeleted`
  - `TestListUserGroups_SoftDeleteFilter`
  - `TestListClusterPermissions_SoftDeleteFilter`、`TestListClusterPermissions_Empty`
  - `TestListAllClusterPermissions_SoftDeleteFilter`
  - 每個測試以 sqlmock 正規表示式 `"deleted_at" IS NULL` 驗證 GORM 生成正確的軟刪除篩選 SQL。

---

## 7. 相依套件風險

### R-03 — `golang.org/x/crypto` 版本過舊 ✅ 已修復

**嚴重度**：🟠 中  
**修復日期**：2026-04-17

| 套件 | 修復前 | 修復後 |
|------|--------|--------|
| golang.org/x/crypto | v0.49.0 | **v0.50.0** |
| golang.org/x/net | v0.52.0 | **v0.53.0** |
| golang.org/x/sys | v0.42.0 | **v0.43.0** |
| golang.org/x/text | v0.35.0 | **v0.36.0** |
| golang.org/x/term | v0.41.0 | **v0.42.0** |

> 升級前實際版本 v0.49.0 為最新前一版，風險較原始評估描述為低。一次性升至最新，`go build ./...` 及全套測試零錯誤。

---

### R-04 — `k8s.io/*` 版本落後 ✅ 已修復

**嚴重度**：🟡 低  
**修復日期**：2026-04-17

**版本對比**：

| 套件 | 修復前 | 修復後 | 最新穩定 |
|------|--------|--------|---------|
| k8s.io/api | v0.29.3 | **v0.32.13** | v0.35.4 |
| k8s.io/apimachinery | v0.29.3 | **v0.32.13** | v0.35.4 |
| k8s.io/client-go | v0.29.3 | **v0.32.13** | v0.35.4 |
| k8s.io/apiextensions-apiserver | v0.29.3 | **v0.32.13** | v0.35.4 |
| k8s.io/apiserver | v0.29.3 | **v0.32.13** | v0.35.4 |
| k8s.io/cli-runtime | v0.29.3 | **v0.32.13** | v0.35.4 |
| k8s.io/component-base | v0.29.3 | **v0.32.13** | v0.35.4 |
| k8s.io/kubectl | v0.29.3 | **v0.32.13** | v0.35.4 |

**選擇 v0.32.13 理由**：13 個 patch release，對應 Kubernetes 1.32（2024/12 發布），是當前最穩定的 minor 版本。v0.35.x 是最新但僅 4 個 patch；可於下次升級週期評估 v0.33–v0.35。

**升級結果**：`go build ./...` 零錯誤，`go test ./internal/services/ ./internal/handlers/` 全數通過，零 API 破壞性變更。

---

### 相依套件掃描建議

目前尚未執行 CVE 掃描，建議加入 CI 流程：

```bash
# 安裝 govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# 掃描已知漏洞
govulncheck ./...
```

---

## 8. 改善建議與優先順序

### P0 — 本週內修復（高風險、低成本）

| # | 任務 | 影響 | 預估工時 |
|---|------|------|----------|
| 1 | **修復 Worker 優雅關閉** — `main.go` 加入 Stoppable 介面，在 SIGTERM 時呼叫所有 Worker 的 `.Stop()` | 防止資料遺失、K8s Job 孤立 | 2h |
| 2 | **修復 CORS 萬用字元邏輯** — `cors.go` 拒絕 `allowed == "*"` 與 credentials 同時啟用 | 防止憑證洩漏 | 0.5h |

### P1 — 兩週內處理（中風險）

| # | 任務 | 影響 | 預估工時 |
|---|------|------|----------|
| 3 | **升級 `golang.org/x/crypto`** | 關閉潛在 CVE 曝露 | 1h |
| 4 | **補齊多表 Transaction** — Pipeline Run 建立、Config Version 儲存 | 防止不一致狀態 | 4h |
| 5 | **Handler 中 `context.Background()` 替換** — 改用 `c.Request.Context()` + timeout | 請求追蹤、取消傳播 | 3h |
| 6 | **加入 CVE 掃描至 CI** — `govulncheck` + `nancy` | 持續監控依賴安全 | 1h |

### P2 — 一個月內處理（架構優化）

| # | 任務 | 影響 | 預估工時 |
|---|------|------|----------|
| 7 | **分拆 `pipeline_scheduler.go`** — 拆為 core/concurrency/executor | 降低修改風險 | 8h |
| 8 | **升級 `k8s.io/*` 至 v0.31.x** | 取得新 API 支援 | 4h（含測試） |
| 9 | **Soft Delete 稽核** — 統一軟刪除作用域，補充測試 | 防止資料復活 | 4h |
| 10 | **K8s API 重試機制** — 使用 `cenkalti/backoff` 包裝關鍵 K8s 呼叫 | 提高 Pipeline 穩健性 | 3h |
| 11 | **加入 Pipeline Scheduler 健康檢查** — 整合至 `/readyz` | 加速故障偵測 | 2h |

---

## 9. 追蹤與複查

| 項目 | 負責人 | 目標日期 | 狀態 |
|------|--------|----------|------|
| R-01 Worker 優雅關閉 | — | 2026-04-17 | ✅ 已完成 |
| R-03 Readiness Check 完整化 | — | 2026-04-17 | ✅ 已完成 |
| R-02 CORS 修正 | — | 2026-04-24 | 🔲 待開始 |
| R-03 (dep) crypto 升級 | — | 2026-04-17 | ✅ 已完成 (x/crypto v0.50.0) |
| R-07 Transaction 補齊 | — | 2026-04-17 | ✅ 已完成 |
| R-09 context 替換 | — | 2026-04-17 | ✅ 已完成 |
| CI CVE 掃描 | — | 2026-04-30 | 🔲 待開始 |
| R-05 Scheduler 分拆 | — | 2026-05-15 | 🔲 待開始 |
| R-04 K8s API 重試 | — | 2026-04-17 | ✅ 已完成 |
| R-04 (dep) k8s 版本升級 | — | 2026-04-17 | ✅ 已完成 (v0.32.13) |
| R-11 Soft Delete 稽核 | — | 2026-05-15 | 🔲 待開始 |

| R-15 Helm v4 升級 Phase 0 | — | 2026-04-17 | ✅ 已完成 |
| R-15 Helm v4 升級 Phase 1–5 | — | 2026-05-15 | 🔲 等待 Review |

**下次複查日期**：2026-05-01

---

## 10. Helm v4 升級方案

> **建立日期**：2026-04-17  
> **狀態**：⏳ Phase 0 已完成，等待 Review 後繼續 Phase 1–5  
> **預估工時**：4–6 小時（含整合測試，Phase 0 驗證後上調——回傳值介面化比預期複雜）

### 10.1 現況

| 項目 | 詳細 |
|------|------|
| 當前版本 | `helm.sh/helm/v3 v3.20.1` |
| 目標版本 | `helm.sh/helm/v4 v4.1.1` |
| 影響檔案 | `internal/services/helm_service.go`（574 行）、`internal/handlers/helm.go`（434 行） |
| 使用的 Helm Action | Install、Upgrade、Uninstall、Rollback、List、Get（Status/Values/History）|
| 未使用功能 | OCI Registry、Plugins、Post-renderer（零風險）|

Synapse 的 Helm 用量極為集中：**只有 2 個檔案**使用 Helm SDK，爆炸半徑小。

---

### 10.2 Helm v4 主要破壞性變更 × Synapse 影響評估（Phase 0 已驗證）

> Phase 0 驗證日期：2026-04-17  
> 驗證方式：下載 `helm.sh/helm/v4@v4.1.1` 原始碼，逐一比對 API 簽章

| # | 破壞性變更 | Synapse 影響 | 處理難度 |
|---|------------|-------------|---------|
| B1 | Module path：`helm.sh/helm/v3` → `helm.sh/helm/v4` | 2 個檔案全部 import 需替換 | 🟢 低（批次取代） |
| B2 | **`Run()` 簽章不變** — v4 的 `Run()` 參數與 v3 完全相同，**無新增 `ctx` 參數**（原始估計錯誤）| 無需修改 `Run()` 呼叫 | ✅ 無影響 |
| B3 | **`Configuration.Init()` 移除 logger 參數** — v3: `Init(getter, ns, driver, log)` → v4: `Init(getter, ns, driver)` | `helm_service.go:144` 需移除第 4 個 `func(format string, v ...interface{}){}` 參數 | 🟢 低 |
| B4 | **回傳型別介面化** — `*release.Release` → `release.Releaser`（空介面）| 所有接收 `Run()` 回傳值的程式碼需 type assert 為 `*v1release.Release`，或改用 `release.Accessor` 介面 | 🔴 **高（核心變更）** |
| B5 | **套件路徑重組** — `release.Release` → `release/v1.Release`；`chart.Chart` → `chart/v2.Chart`；`repo.IndexFile` → `repo/v1.IndexFile` | import 路徑全部需調整 | 🟡 中 |
| B6 | **`loader.Load()` 回傳介面化** — `*chart.Chart` → `chart.Charter`（空介面）| `helm_service.go` L217, L285 的 `loader.Load()` 回傳值可直接傳入 `Run()`（因 `Run()` 也接收 `Charter`），**無需轉型** | 🟢 低 |
| B7 | **`Info.Status` 型別移動** — `release.Status` → `release/common.Status`（仍為 `string` 底層型別）| `helm.go:58` 的 `string(r.Info.Status)` 仍有效 | 🟢 低 |
| B8 | **`Info.LastDeployed` 型別不變** — 仍為 `time.Time` | `helm.go:49` 的 `.IsZero()` 和 `.UTC().Format()` 不受影響 | ✅ 無影響 |
| B9 | `RESTClientGetter` 介面位置不變 — 仍在 `k8s.io/cli-runtime/pkg/genericclioptions` | `restClientGetter` 的 4 個方法無需修改 | ✅ 無影響 |
| B10 | `chart.Metadata` 移至 `chart/v2.Metadata` — 欄位 `Name`/`Version`/`AppVersion` 不變 | `helm.go:44-46`、`helm_service.go:252-254` 使用的欄位均存在 | 🟢 低（僅 import 路徑） |
| B11 | Go 版本要求 `>= 1.25.0` | 當前 `go 1.25.0` ✅ 符合 | ✅ 無影響 |

**整體風險修正**：原始估計為「🟡 低–中」，Phase 0 驗證後調整為 **🟠 中**。

原因：原本以為主要工作是補 `ctx` 參數（機械性修改），但實際**最大變更是回傳值介面化（B4）**——所有 action 的 `Run()` 回傳 `Releaser`（空介面）而非 `*Release` struct，需要在 service 層或 handler 層進行 type assertion 或改用 `Accessor` 介面。

---

### 10.3 六階段最小衝擊升級方案

#### Phase 0：預備驗證 ✅ 已完成（2026-04-17）

下載 `helm.sh/helm/v4@v4.1.1` 原始碼，逐一比對所有 Synapse 使用到的 API 簽章。

**驗證結果（3 個核心問題）**：

- [x] `Run()` 的第一個參數是否為 `context.Context`？ → **否！v4 的 `Run()` 參數與 v3 完全相同**（原始估計錯誤）
- [x] `RESTClientGetter` 介面在哪個套件？ → **不變**，仍在 `k8s.io/cli-runtime/pkg/genericclioptions`
- [x] `release.Release` 的 `Info.Status` 欄位是否更名？ → **型別移至 `release/common.Status`**，但底層仍為 `string`，`string()` 轉換仍有效

**新發現的重大變更**：

1. **回傳值介面化**：所有 `action.*.Run()` 回傳 `Releaser`（空介面 `interface{}`）而非 `*Release`
   - 需使用 `release.NewAccessor(rel)` 取得 `Accessor` 介面來讀取欄位
   - 或 type assert 為 `*v1release.Release` 直接存取 struct 欄位
2. **套件路徑重組**：`release.Release` → `release/v1.Release`；`chart.Chart` → `chart/v2.Chart`；`repo.*` → `repo/v1.*`
3. **`Configuration.Init()` 簽章變更**：移除第 4 個 logger 參數
4. **Go 版本要求**：`>= 1.25.0`（當前 `1.25.0` ✅）

---

#### Phase 1：升級相依套件 ✅ 已完成（2026-04-17）

```bash
go get helm.sh/helm/v4@v4.1.1
go mod tidy
```

**實際執行結果**：`go get` 成功下載 v4.1.1 並升級了 7 個間接依賴（`fatih/color`、`go-openapi/*`、`mailru/easyjson`、`prometheus/procfs`、`kustomize/kyaml`）。但 `go mod tidy` 會自動移除 v4（因尚無程式碼 import），因此 **Phase 1 與 Phase 2 需合併執行**——先改 import 再 `go mod tidy`。

`go build ./...` 通過 ✅，現有程式碼不受影響。

---

#### Phase 2：替換 Import 路徑 ✅ 已完成（2026-04-17）

**`internal/services/helm_service.go` import 變更**：

```go
// v3 → v4
"helm.sh/helm/v3/pkg/action"        → "helm.sh/helm/v4/pkg/action"          // 套件名不變
"helm.sh/helm/v3/pkg/chart/loader"   → "helm.sh/helm/v4/pkg/chart/loader"   // 套件名不變
"helm.sh/helm/v3/pkg/release"        → v1release "helm.sh/helm/v4/pkg/release/v1"  // 套件名為 v1，需 alias
"helm.sh/helm/v3/pkg/repo"           → "helm.sh/helm/v4/pkg/repo/v1"        // 套件名仍為 repo，無需 alias
```

**`internal/handlers/helm.go` import 變更**：

```go
"helm.sh/helm/v3/pkg/release" → v1release "helm.sh/helm/v4/pkg/release/v1"
```

**型別名稱更新**：兩個檔案中所有 `*release.Release` → `*v1release.Release`（共 7 處）。

**go.mod 結果**：`helm.sh/helm/v3 v3.20.1` 被 `go mod tidy` 自動移除，替換為 `helm.sh/helm/v4 v4.1.1`。

**build 狀態**：預期失敗（10 個錯誤），全部屬於 Phase 3 範疇：
- `Init()` 多餘 logger 參數（1 處）
- `Run()` 回傳 `Releaser` 介面需 type assert（9 處）

---

#### Phase 3：修復 API 破壞性變更 ✅ 已完成（2026-04-17）

**3a. `Configuration.Init()` 移除 logger 參數**

```go
// 修改前（v3）— L144
actionConfig.Init(getter, namespace, "secret", func(format string, v ...interface{}) {})

// 修改後（v4）— 移除第 4 個參數
actionConfig.Init(getter, namespace, "secret")
```

**3b. 回傳值介面化處理（核心變更）**

v4 所有 `action.*.Run()` 回傳 `Releaser`（空介面）而非 `*Release`。需在 service 層將回傳值轉換回具體型別。

**策略選擇**：

| 策略 | 說明 | 優點 | 缺點 |
|------|------|------|------|
| A. Type Assert | `rel.(*v1release.Release)` | 最少改動、handler 不變 | 耦合 v1 實作 |
| B. Accessor 介面 | `release.NewAccessor(rel)` | future-proof、官方推薦 | handler 需大幅改寫 |

**建議採用策略 A**（最小改動），在 service 層統一轉型：

```go
// 修改前（v3）— service 直接回傳 *release.Release
func (s *HelmService) GetRelease(...) (*release.Release, error) {
    return statusAction.Run(name)
}

// 修改後（v4）— Run() 回傳 Releaser，service 轉回 *v1release.Release
func (s *HelmService) GetRelease(...) (*v1release.Release, error) {
    rel, err := statusAction.Run(name)
    if err != nil {
        return nil, err
    }
    r, ok := rel.(*v1release.Release)
    if !ok {
        return nil, fmt.Errorf("unexpected release type: %T", rel)
    }
    return r, nil
}
```

需處理的 9 個 `.Run()` 呼叫（**參數不變，僅回傳值需轉型**）：

| Action | 行號 | v3 回傳 | v4 回傳 | 轉型方式 |
|--------|------|--------|--------|---------|
| `List.Run()` | L162 | `[]*release.Release` | `[]ri.Releaser` | 迴圈 type assert |
| `Status.Run(name)` | L173 | `*release.Release` | `ri.Releaser` | 單一 type assert |
| `History.Run(name)` | L185 | `[]*release.Release` | `[]ri.Releaser` | 迴圈 type assert |
| `GetValues.Run(name)` | L197 | `map[string]interface{}` | `map[string]interface{}` | ✅ **不變** |
| `Install.Run(chrt, vals)` | L233 | `*release.Release` | `ri.Releaser` | 單一 type assert |
| `Status.Run(name)` | L245 | `*release.Release` | `ri.Releaser` | 單一 type assert |
| `Upgrade.Run(name, chrt, vals)` | L299 | `*release.Release` | `ri.Releaser` | 單一 type assert |
| `Rollback.Run(name)` | L311 | `error` | `error` | ✅ **不變** |
| `Uninstall.Run(name)` | L322 | `*UninstallReleaseResponse` | `*release.UninstallReleaseResponse` | import 路徑調整 |

**3c. handler 層調整**

`helm.go` 的 `toReleaseResponse()` 接收 `*release.Release`。若 service 層用策略 A 已轉型為 `*v1release.Release`，handler 只需將型別改為 `*v1release.Release`，欄位存取方式完全不變（`r.Name`、`r.Info.Status`、`r.Chart.Metadata.Name` 等）。

**3d. DB 查詢補上 `.WithContext(ctx)`（附帶修正）**

`helm_service.go` 有 6 處 DB 查詢（L259, L329, L343, L351, L364, L480）均缺少 `.WithContext(ctx)`。
同步為所有 service method 新增 `ctx context.Context` 參數，並修正 DB 查詢：

```go
// 修改前
func (s *HelmService) ListRepos() ([]models.HelmRepository, error) {
    s.db.Find(&repos)
}

// 修改後
func (s *HelmService) ListRepos(ctx context.Context) ([]models.HelmRepository, error) {
    s.db.WithContext(ctx).Find(&repos)
}
```

同步更新 `helm.go` handler 中所有呼叫端，傳入 `c.Request.Context()`。

**3e. `RESTClientGetter` — ✅ 無需修改**

Phase 0 已確認介面位置（`genericclioptions.RESTClientGetter`）和方法簽章完全不變。

**Phase 3 執行摘要**：

| 子項目 | 修改數 | 說明 |
|--------|--------|------|
| 3a. Init() 移除 logger | 1 處 | `helm_service.go:144` |
| 3b. Run() 回傳值 type assert | 7 處 | 新增 `toRelease()` / `toReleases()` 輔助函式 |
| 3c. handler 型別更新 | 0 處 | service 層已轉型，handler 不變 |
| 3d. ctx + WithContext | 13+6=19 處 | 13 個 service method 加 ctx，6 個 DB 查詢加 WithContext |
| 3e. RESTClientGetter | 0 處 | 不變 |

`go build ./...` ✅ | `go test ./internal/services/... ./internal/handlers/...` ✅ | `go vet` ✅

---

#### Phase 4：建置與單元測試 ✅ 已完成（2026-04-17）

```bash
go build ./...    # ✅ 零錯誤
go vet ./...      # ✅ 零警告
go test ./...     # ✅ 全部通過（28 個套件，0 失敗）
```

確認 `go.mod` / `go.sum` 中無 `helm.sh/helm/v3` 殘留引用。

---

#### Phase 5：整合測試（1–2 小時）

在真實（或 kind）叢集上驗證完整流程：

```
□ 安裝 Chart（Install）
□ 升級已安裝的 Release（Upgrade）
□ 查詢 Release 狀態（Get Status）
□ 查詢 Release Values（Get Values）
□ 查詢 Release 歷史（Get History）
□ Rollback 到前一版（Rollback）
□ 解除安裝（Uninstall）
□ List 所有 Releases（List）
```

---

### 10.4 回溯方案

若整合測試失敗或發現未知問題：

```bash
# 1. 還原 go.mod / go.sum
git checkout go.mod go.sum

# 2. 還原兩個源碼檔
git checkout internal/services/helm_service.go internal/handlers/helm.go

# 3. 重新建置確認回到 v3
go build ./...
```

回溯成本極低，因所有變更集中在 **2 個源碼檔** + 相依套件清單。

---

### 10.5 不升級的風險

| 風險 | 說明 |
|------|------|
| v3 安全性更新 | Helm v3 已進入維護模式；未來 CVE 修補可能僅限 v4 |
| API 棄用 | 部分 Helm v3 API 不再推薦，可能影響長期相容性 |
| 生態系整合 | Chart Museum、OCI Registry 在 v4 有較好支援 |

**建議**：因影響範圍極小，建議在下次 K8s 升級週期一併執行，避免雙重維護成本。

---

*文件版本：v1.0 — 初版風險評估（2026-04-17）*
