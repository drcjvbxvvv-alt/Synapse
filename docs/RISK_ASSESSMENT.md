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
| R-04 | `k8s.io/*` 版本落後 | 🟡 低 | 低 | P2 | ✅ 已修復 |
| R-05 | Pipeline Scheduler 過大（1338 行）| 🟠 中 | 高 | **P1** | 架構債 |
| R-06 | 多個服務檔案超過 600 行 | 🟡 低 | 中 | P2 | 架構債 |
| R-07 | 多表寫入缺少 Transaction | 🟠 中 | 中 | **P1** | 待修 |
| R-08 | InsecureSkipVerify 未完整配置化 | 🟠 中 | 低 | P2 | 部分已處理 |
| R-09 | Handler 中使用 `context.Background()` | 🟠 中 | 高 | P2 | 待修 |
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

`services/` 社群（2350 個節點）是專案中最大的 Community，有成為 God Package 的風險。目前已知欲改進但尚未完成的遷移：

- `permission_service.go` 同時包含舊 DB 邏輯與新 Repository 層（雙路徑並行）
- 建議按子領域分組：`services/pipeline/`、`services/k8s/`、`services/ai/`

---

### R-09 — Handler 中使用 `context.Background()`

**嚴重度**：🟠 中  
**統計**：`internal/handlers/` 中有 **17 處**使用 `context.Background()`（非測試檔案）

**問題描述**：

Handler 中的非同步 goroutine 應繼承請求 context 並加上 timeout，而非使用 `context.Background()`，否則：
- 請求取消時非同步操作仍繼續執行（浪費資源）
- 分散式追蹤（OTel）無法跨越非同步邊界
- 超時不會傳播

**正確模式**：

```go
// ❌ 目前做法（handler 中）
go func() {
    ctx := context.Background()
    // ...
}()

// ✅ 改善做法
reqCtx := c.Request.Context()
go func() {
    ctx, cancel := context.WithTimeout(reqCtx, 30*time.Second)
    defer cancel()
    // ...
}()
```

> **例外**：背景 worker（非請求觸發）使用 `context.Background()` 是正確的。

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

### R-11 — Soft Delete 查詢作用域不一致

**嚴重度**：🟠 中  
**統計**：39 個檔案使用軟刪除模式

**問題描述**：

GORM 的 `DeletedAt gorm.DeletedAt` 欄位會自動加上 `WHERE deleted_at IS NULL` 篩選，但有以下風險：

1. **自訂查詢**（`db.Raw()`）不自動套用軟刪除篩選
2. **關聯預載入**（`Preload()`）對軟刪除的子記錄行為未統一驗證
3. 部分模型使用 `IsDeleted bool` 而非 GORM 標準 `DeletedAt`，需手動篩選

**改善建議**：

- 統一採用 GORM 標準 `DeletedAt gorm.DeletedAt` 軟刪除欄位
- 禁止在業務邏輯層使用 `db.Raw()`（違反 CLAUDE.md 規範）
- 建立軟刪除稽核測試：確認所有列表查詢不回傳已刪除記錄

---

## 7. 相依套件風險

### R-03 — `golang.org/x/crypto` 版本過舊

**嚴重度**：🟠 中  
**目前版本**：`v0.49.0`  
**最新版本**：`v0.36.0+`（含多項 TLS/HKDF 修正）

**改善方式**：

```bash
go get golang.org/x/crypto@latest
go mod tidy
go test ./...
```

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
| R-03 (dep) crypto 升級 | — | 2026-04-30 | 🔲 待開始 |
| R-07 Transaction 補齊 | — | 2026-04-17 | ✅ 已完成 |
| R-09 context 替換 | — | 2026-04-30 | 🔲 待開始 |
| CI CVE 掃描 | — | 2026-04-30 | 🔲 待開始 |
| R-05 Scheduler 分拆 | — | 2026-05-15 | 🔲 待開始 |
| R-04 K8s API 重試 | — | 2026-04-17 | ✅ 已完成 |
| R-04 (dep) k8s 版本升級 | — | 2026-04-17 | ✅ 已完成 (v0.32.13) |
| R-11 Soft Delete 稽核 | — | 2026-05-15 | 🔲 待開始 |

**下次複查日期**：2026-05-01

---

*文件版本：v1.0 — 初版風險評估（2026-04-17）*
