# Pipeline Rollback 實作記錄

## 目標

允許使用者對任何歷史成功 Run 發起「回滾」，系統會跳過所有 build/scan Step，直接取用該 Run 的映像 Artifacts，重新執行 deploy Step，將服務還原至指定版本。

---

## 階段總覽

| 階段 | 範圍 | 狀態 |
|------|------|------|
| Stage 1 | 資料層：模型欄位 + Migration | ✅ 完成 |
| Stage 2 | 後端：Rollback API + Scheduler 邏輯 | ✅ 完成 |
| Stage 3 | 前端：回滾按鈕 + Badge + i18n | ✅ 完成 |

---

## Stage 1 — 資料層 ✅

### 變更檔案

| 檔案 | 說明 |
|------|------|
| `internal/models/pipeline.go` | 新增 `TriggerTypeRollback = "rollback"` 常數；`PipelineRun` 新增 `RollbackOfRunID *uint` 欄位 |
| `internal/database/migrations/postgres/001_baseline.up.sql` | 在 `pipeline_runs` 定義加入 `rollback_of_run_id bigint` 欄位與 partial index（供全新安裝） |
| `internal/database/migrations/postgres/002_rollback.up.sql` | ALTER TABLE 新增欄位 + index（供現有安裝升級） |
| `internal/database/migrations/postgres/002_rollback.down.sql` | 回滾 Migration（DROP COLUMN） |
| `internal/models/pipeline_rollback_test.go` | 單元測試：常數值、omitempty、JSON 序列化 |

### 資料模型變更

```go
// 新常數
TriggerTypeRollback = "rollback"

// PipelineRun 新欄位
RollbackOfRunID *uint `json:"rollback_of_run_id,omitempty"`
```

### DB Schema 變更

```sql
-- pipeline_runs 新增
rollback_of_run_id bigint  -- 指向被回滾的原始成功 RunID

-- Partial index（僅索引非空值，最小化空間使用）
CREATE INDEX idx_pipeline_runs_rollback_of_run_id
  ON pipeline_runs (rollback_of_run_id)
  WHERE rollback_of_run_id IS NOT NULL;
```

### 設計說明

- `RollbackOfRunID` 與 `RerunFromID` 語義不同：
  - `RerunFromID`：從某個失敗 Run 的指定 Step 繼續跑（可能重新 build）
  - `RollbackOfRunID`：完全跳過 build/scan，直接用歷史 Run 的映像重跑 deploy
- `omitempty` 確保一般 Run 的 JSON 回應不含此欄位，不破壞既有 API 合約
- 使用 partial index 節省索引空間（絕大多數 run 的此欄位為 NULL）

---

## Stage 2 — 後端 API ✅

### 端點

```
POST /api/v1/pipelines/:pipelineID/runs/:runID/rollback
```

### 請求體（選填）

```json
{
  "cluster_id": 1,     // 覆蓋目標叢集（空 = 沿用來源 Run 的叢集）
  "namespace": "prod"  // 覆蓋目標 namespace（空 = 沿用來源 Run 的 namespace）
}
```

### 回應（202 Accepted）

```json
{
  "run_id": 123,
  "rollback_of_run": 100,
  "snapshot_id": 5,
  "status": "queued",
  "message": "pipeline rollback triggered"
}
```

### 變更檔案

| 檔案 | 說明 |
|------|------|
| `internal/services/pipeline_step_types.go` | 新增 `IsDeployStepType()` 套件級函式（原為 `JobBuilder` 私有方法） |
| `internal/services/pipeline_job_builder.go` | `isDeployStepType` 委派至 `IsDeployStepType()` |
| `internal/services/pipeline_scheduler.go` | 新增 `loadRollbackArtifacts`、`copyArtifactsToRollbackRun`；DAG 初始化加入 rollback skip 邏輯；run 完成後複製 artifacts |
| `internal/handlers/pipeline_run_handler.go` | 新增 `RollbackRun` handler（`RollbackRequest` + 完整 5-step 流程） |
| `internal/router/routes_pipeline.go` | 新增 `POST .../rollback` 路由 |
| `internal/services/pipeline_rollback_test.go` | `TestIsDeployStepType` 驗證所有 deploy/non-deploy 分類 |
| `internal/handlers/pipeline_run_handler_test.go` | 5 個 handler 測試：invalid ID、not found、wrong pipeline、not success、happy path |

### Scheduler 行為

1. DAG 初始化時：
   - non-deploy Step（build-image、trivy-scan、approval 等）→ `initialStatus = skipped`
   - deploy Step（deploy、deploy-helm、deploy-rollout、gitops-sync 等）→ 正常執行
2. 若來源 Run 有 `kind=image` 的 artifact：覆蓋 deploy StepRun 的 `Image` 欄位（注入舊映像 digest）
3. Rollback Run 成功完成後：自動複製來源 Run 的所有 artifacts（審計追蹤）

---

## Stage 3 — 前端 ✅

### 變更檔案

| 檔案 | 說明 |
|------|------|
| `ui/src/services/pipelineService.ts` | `PipelineRun.trigger_type` 加入 `'rollback'`；新增 `rollback_of_run_id?: number \| null`；新增 `rollbackRun()` 方法 |
| `ui/src/pages/pipeline/PipelineRunDetail.tsx` | 新增 `rollbackMutation`；成功 Run 顯示回滾按鈕（`Popconfirm` + `RollbackOutlined`）；`rollback` trigger type 顯示橙色 Tag；若 `rollback_of_run_id` 存在則顯示來源 Run 連結 |
| `ui/src/locales/en-US/pipeline.json` | 新增 `run.triggerType.rollback`、`runDetail.rollback*` 相關鍵值 |
| `ui/src/locales/zh-CN/pipeline.json` | 同上（簡體中文） |
| `ui/src/locales/zh-TW/pipeline.json` | 同上（繁體中文） |

### UI 行為

- 成功 Run 的操作列顯示「回滾至此版本」按鈕（`RollbackOutlined` icon）
- 點擊按鈕彈出 `Popconfirm` 確認，確認後呼叫 `POST .../rollback`
- 回滾成功後自動導航至新 Run 的詳情頁
- Rollback Run 的 trigger_type Tag 顯示為橙色
- Rollback Run 的 Descriptions 中顯示「來源 Run」連結，可直接導航回被回滾的原始 Run

### i18n 鍵值

| 鍵值 | 說明 |
|------|------|
| `pipeline:run.triggerType.rollback` | Trigger 類型標籤文字 |
| `pipeline:runDetail.rollback` | 回滾按鈕文字 |
| `pipeline:runDetail.rollbackConfirmTitle` | Popconfirm 標題（含 Run ID） |
| `pipeline:runDetail.rollbackSuccess` | 成功訊息（含新 Run ID） |
| `pipeline:runDetail.rollbackFailed` | 失敗訊息 |
| `pipeline:runDetail.rollbackSource` | Descriptions 標籤文字 |
| `pipeline:runDetail.rollbackSourceRun` | Descriptions 連結文字（含原始 Run ID） |
