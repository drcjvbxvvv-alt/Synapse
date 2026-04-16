# CI 引擎 Adapter — 使用與開發指南

| 項目       | 內容                                                |
| ---------- | --------------------------------------------------- |
| 版本       | M18a（Framework）+ M18b（GitLab）+ M18c（Jenkins）+ M18d（Tekton）完成 |
| 日期       | 2026-04-16                                          |
| 對應 ADR   | [ADR-015](./adr/ADR-015-CI-Engine-Adapter-Pattern.md) |
| 實作紀錄   | [M18a](./M18a_IMPLEMENTATION_RECORD.md)、[M18b](./M18b_IMPLEMENTATION_RECORD.md)、[M18c](./M18c_IMPLEMENTATION_RECORD.md)、[M18d](./M18d_IMPLEMENTATION_RECORD.md) |

---

## 1. 概觀

Synapse 將 CI 執行引擎抽象為 `CIEngineAdapter` 介面，**同一個 Synapse 部署可同時支援多種 CI 引擎**，每條 Pipeline 獨立選擇自己要用的引擎。

| 引擎                        | `engine_type` | 狀態    | 備註                           |
| --------------------------- | ------------- | ------- | ------------------------------ |
| **Native**（內建 K8s Job）   | `native`      | ✅ M18a  | 預設，無外部依賴                |
| **GitLab CI**               | `gitlab`      | ✅ M18b  | API v4 + PRIVATE-TOKEN 認證    |
| **Jenkins**                 | `jenkins`     | ✅ M18c  | Basic Auth + CSRF Crumb + Queue 輪詢 |
| **Tekton**                  | `tekton`      | ✅ M18d  | K8s-native CRD（PipelineRun + TaskRun） |
| Argo Workflows              | `argo`        | 🔜 M18e | 已預留介面                     |
| GitHub Actions              | `github`      | 🔜 M18e | 已預留介面                     |

### 設計原則

1. **預設零依賴** — Native Adapter 是 in-process 實作，無須安裝任何外部 CI 工具
2. **可插拔** — 透過 `CIEngineAdapter` 介面接入任一主流 CI 工具
3. **優雅降級** — 遵循 `CLAUDE.md §8` Observer Pattern，單一引擎故障不阻塞 UI
4. **每條 Pipeline 獨立選擇** — 不是全系統一刀切，使用者可靈活組合

---

## 2. 架構圖

```
┌──────────────────────────────────────────────┐
│      CIEngineHandler（HTTP API）              │
└──────────────────┬───────────────────────────┘
                   │
┌──────────────────▼───────────────────────────┐
│      CIEngineService                         │
│   • CRUD on ci_engine_configs                │
│   • ListAvailableEngines() 探測所有引擎      │
└──────────────────┬───────────────────────────┘
                   │
┌──────────────────▼───────────────────────────┐
│      engine.Factory（singleton 或 DI）        │
│   • Register(EngineType, AdapterBuilder)     │
│   • Build(cfg) → CIEngineAdapter             │
└──────────────────┬───────────────────────────┘
                   │
       ┌───────────┴───────────┬───────────────┐
       ▼                       ▼               ▼
┌─────────────┐    ┌─────────────────────┐  ┌─────────────┐
│  Native     │    │  GitLab / Jenkins / │  │  您的自訂   │
│  Adapter    │    │  Tekton / Argo / GH │  │  Adapter    │
│  (M18a ✅)  │    │  (M18b+)            │  │             │
└─────────────┘    └─────────────────────┘  └─────────────┘
```

---

## 3. HTTP API

### 3.1 探測所有引擎可用性

```http
GET /api/v1/ci-engines/status
```

回傳每個引擎的可用性、版本、能力宣告：

```json
{
  "items": [
    {
      "type": "native",
      "name": "Native (K8s Job)",
      "available": true,
      "default": true,
      "version": "v1.2.3",
      "capabilities": {
        "supports_dag": true,
        "supports_matrix": true,
        "supports_artifacts": true,
        "supports_secrets": true,
        "supports_caching": false,
        "supports_approvals": true,
        "supports_notification": true,
        "supports_live_log": true
      }
    },
    {
      "type": "gitlab",
      "config_id": 1,
      "name": "gitlab-main",
      "available": false,
      "error": "gitlab connection refused"
    }
  ],
  "total": 2
}
```

**權限：** 所有已登入使用者可呼叫（供 Pipeline 作者在組合 Run 時查詢）。

### 3.2 管理外部引擎連線設定

> ⚠️ 下列端點均需 **Platform Admin** 權限（因涉及憑證管理）。

| 方法     | 端點                        | 說明                                          |
| -------- | --------------------------- | --------------------------------------------- |
| `POST`   | `/api/v1/ci-engines`        | 新增外部引擎設定                              |
| `GET`    | `/api/v1/ci-engines`        | 列出所有外部引擎設定                          |
| `GET`    | `/api/v1/ci-engines/:id`    | 取得單筆設定                                  |
| `PUT`    | `/api/v1/ci-engines/:id`    | 更新設定（`engine_type` 不可變更）            |
| `DELETE` | `/api/v1/ci-engines/:id`    | 刪除設定                                      |

**新增範例：**

```json
POST /api/v1/ci-engines
{
  "name": "gitlab-main",
  "engine_type": "gitlab",
  "endpoint": "https://gitlab.example.com",
  "auth_type": "token",
  "token": "glpat-xxxxxxxx",
  "insecure_skip_verify": false
}
```

**重要行為：**

- ✅ `native` 類型**不允許**建立連線設定（內建引擎不需要外部連線）
- ✅ 空字串憑證（例：`"token": ""`）**保留資料庫現有值**，以支援「編輯不需重填密碼」的 UI 行為
- ✅ 憑證欄位（`Token` / `Password` / `WebhookSecret` / `CABundle`）採 `pkg/crypto` AES-256-GCM 加密儲存
- ✅ 所有 JSON 回應**永遠不會包含**敏感欄位（模型使用 `json:"-"` 標籤）

---

## 4. 為 Pipeline 指定引擎

在 Pipeline 中只需指定 `engine_type` 與（外部引擎時）`engine_config_id`：

```json
{
  "name": "build-saas-java-a",
  "engine_type": "gitlab",
  "engine_config_id": 1,
  "steps": [ ]
}
```

| `engine_type` | `engine_config_id` | 行為                                   |
| ------------- | ------------------ | -------------------------------------- |
| `"native"`（預設） | `null`         | 使用內建 K8s Job 引擎                   |
| `"gitlab"`    | 必填               | 透過指定的 GitLab 連線設定執行          |
| `"jenkins"`   | 必填               | 透過指定的 Jenkins 連線設定執行         |
| 其他外部類型   | 必填               | 同上                                   |

---

## 5. 新增自訂 Adapter（開發者指引）

### 5.1 基本骨架

實作 `CIEngineAdapter` 介面並向 Factory 註冊：

```go
package myengine

import (
    "context"
    "io"

    "github.com/shaia/Synapse/internal/models"
    "github.com/shaia/Synapse/internal/services/pipeline/engine"
)

type MyAdapter struct { /* ... */ }

func (a *MyAdapter) Type() engine.EngineType { return "my-engine" }

func (a *MyAdapter) IsAvailable(ctx context.Context) bool { /* ... */ }

func (a *MyAdapter) Version(ctx context.Context) (string, error) { /* ... */ }

func (a *MyAdapter) Capabilities() engine.EngineCapabilities {
    return engine.EngineCapabilities{
        SupportsDAG:          true,
        SupportsSecrets:      true,
        SupportsLiveLog:      true,
    }
}

func (a *MyAdapter) Trigger(ctx context.Context, req *engine.TriggerRequest) (*engine.TriggerResult, error) { /* ... */ }
func (a *MyAdapter) GetRun(ctx context.Context, runID string) (*engine.RunStatus, error) { /* ... */ }
func (a *MyAdapter) Cancel(ctx context.Context, runID string) error { /* ... */ }
func (a *MyAdapter) StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error) { /* ... */ }
func (a *MyAdapter) GetArtifacts(ctx context.Context, runID string) ([]*engine.Artifact, error) { /* ... */ }

func init() {
    engine.Default().MustRegister("my-engine",
        func(cfg *models.CIEngineConfig) (engine.CIEngineAdapter, error) {
            return &MyAdapter{ /* 由 cfg 讀取 Endpoint、Token… */ }, nil
        },
    )
}
```

### 5.2 實作契約

| 要求                  | 說明                                                                      |
| --------------------- | ------------------------------------------------------------------------- |
| **並發安全**           | 所有方法必須可並行呼叫（Factory 會在多個請求間共用 Adapter 實例）          |
| **尊重 ctx**          | 必須在 `ctx.Done()` 後立即返回，且所有下游呼叫須傳遞 ctx                   |
| **Observer Pattern**  | `IsAvailable()` 禁止回傳 error；連線失敗時回傳 `false`，內部記 `logger.Warn` |
| **Sentinel errors**   | 透過 `fmt.Errorf(...%w...)` 包裝 `engine.ErrNotFound` 等，供 handler 映射 HTTP |
| **Capabilities 穩定** | `Capabilities()` 必須冪等且無 I/O（Factory 可快取）                       |
| **Log scrubbing**     | 不得將連線憑證、Token 寫入 log                                            |

### 5.3 Sentinel Errors 對應表

| 錯誤                | HTTP Status | 用途                                          |
| ------------------- | ----------- | --------------------------------------------- |
| `ErrNotFound`       | 404         | 找不到 run / artifact                         |
| `ErrInvalidInput`   | 400         | 請求格式錯誤                                  |
| `ErrUnauthorized`   | 401         | 引擎拒絕憑證                                  |
| `ErrUnavailable`    | 502 / 503   | 引擎離線或回傳 5xx                            |
| `ErrUnsupported`    | 501         | Adapter 未實作該功能（搭配 `Capabilities()`） |
| `ErrAlreadyTerminal` | —（資訊性） | `Cancel` 時 run 已結束                         |

### 5.4 測試要求

- 每個 Adapter 提供 mock server（httptest + fixture JSON）
- Contract test 覆蓋：Trigger / GetRun / Cancel / 錯誤映射 / ctx 取消
- `-race` 測試並發呼叫的安全性

參考範例：`internal/services/pipeline/engine/native_test.go`（17 個測試）。

---

## 6. Observer Pattern 優雅降級

遵循 [`CLAUDE.md §8`](../CLAUDE.md)，實作上須確保：

1. `IsAvailable()` 失敗 → UI 顯示「離線」圖示，**不阻塞頁面載入**
2. `ListAvailableEngines()` 為每個引擎設 **5 秒短超時** → 慢引擎不會拖累整頁
3. 某個 Adapter `Build()` 失敗 → 其他引擎正常顯示，失敗者的錯誤寫入 `EngineStatus.Error`
4. Native 引擎永遠回 `available=true`（in-process，與 Synapse 同生命週期）

---

## 7. 資料模型

### 7.1 `ci_engine_configs` 表

| 欄位                   | 類型          | 備註                                           |
| ---------------------- | ------------- | ---------------------------------------------- |
| `id`                   | uint          | PK                                             |
| `name`                 | varchar(100)  | 唯一索引                                       |
| `engine_type`          | varchar(20)   | 建立後**不可變更**                              |
| `enabled`              | bool          | 預設 true                                      |
| `endpoint`             | varchar(500)  | 引擎 URL                                       |
| `auth_type`            | varchar(20)   | `token` / `basic` / `kubeconfig` / `service_acct` |
| `username`             | varchar(100)  |                                                |
| `token`                | text          | **加密**（AES-256-GCM）                         |
| `password`             | text          | **加密**                                       |
| `webhook_secret`       | text          | **加密**                                       |
| `ca_bundle`            | text          | **加密**（選用 PEM）                            |
| `cluster_id`           | uint（nullable） | Tekton / Argo 用，指向 Synapse 管理的叢集       |
| `extra_json`           | text          | 引擎特定設定（JSON blob）                      |
| `insecure_skip_verify` | bool          | 預設 false                                     |
| `last_checked_at`      | timestamp     | 探測更新                                       |
| `last_healthy`         | bool          |                                                |
| `last_version`         | varchar(50)   |                                                |
| `last_error`           | text          |                                                |
| `created_by`           | uint          | 建立者 user id                                 |
| `created_at` / `updated_at` / `deleted_at` | —   | GORM 標準欄位                                   |

### 7.2 `pipelines` 表新增欄位

```sql
ALTER TABLE pipelines
    ADD COLUMN engine_type VARCHAR(20) NOT NULL DEFAULT 'native',
    ADD COLUMN engine_config_id BIGINT NULL;
CREATE INDEX idx_pipelines_engine_type ON pipelines(engine_type);
CREATE INDEX idx_pipelines_engine_config_id ON pipelines(engine_config_id);
```

---

## 8. 檔案結構（M18a）

```
internal/services/pipeline/engine/     # 新目錄：Adapter 框架
├── types.go                           # EngineType、Capabilities、TriggerRequest、RunStatus …
├── adapter.go                         # CIEngineAdapter 介面
├── errors.go                          # Sentinel errors
├── factory.go                         # Factory + 註冊機制
├── native.go                          # NativeAdapter + NativeRunner 介面
├── types_test.go
├── adapter_test.go
├── factory_test.go
└── native_test.go

internal/models/
├── ci_engine_config.go                # CIEngineConfig 模型 + Request DTO
├── ci_engine_config_test.go
├── ci_engine_config_helpers_test.go
├── pipeline.go                        # 擴充 EngineType、EngineConfigID
└── pipeline_engine_test.go

internal/services/
├── ci_engine_service.go               # CRUD + ListAvailableEngines
└── ci_engine_service_test.go

internal/handlers/
├── ci_engine_handler.go               # HTTP handlers（5-step flow）
└── ci_engine_handler_test.go

internal/router/
├── router.go                          # 掛載 registerCIEngineRoutes
└── routes_ci_engine.go                # 路由 + PlatformAdminRequired
```

---

## 9. 常見問題（FAQ）

### Q1：為什麼 Native Adapter 的 `StreamLogs` 回空 reader？

M18a 是框架階段；既有的 `/api/v1/pipeline-runs/:id/logs` SSE 端點照常運作。
M18b 會將所有日誌串流統一經過 Adapter 介面。

### Q2：為什麼 `engine_type` 不能更新？

若允許更新，會使所有引用該設定的 Pipeline 歷史 run 與新 run 位於不同引擎，造成狀態混亂。
請改為**建立新的 CIEngineConfig**。

### Q3：可以同時啟用多種引擎嗎？

可以。每條 Pipeline 獨立選引擎，互不干擾。

### Q4：Native 引擎會離線嗎？

不會 — Native 是 in-process 實作，與 Synapse 本體同生命週期，`IsAvailable()` 恆回 `true`。

### Q5：未設定的外部引擎在 `/status` 會出現嗎？

只有 **已存在 `CIEngineConfig` 紀錄** 的外部引擎才會列出。
從未設定過的引擎（例如首次部署）不會出現，UI 應提供「新增」引導。

### Q6：如何在測試中使用獨立 Factory？

```go
f := engine.NewFactory()
engine.RegisterNative(f, fakeRunner{}, "v-test")
svc := services.NewCIEngineService(db, f) // 傳入獨立 factory
```

生產代碼使用 `engine.Default()` 單例；測試用 `engine.NewFactory()` 避免共用狀態。

### Q7：Adapter 能取得 Synapse 管理的叢集嗎？

`CIEngineConfig.ClusterID` 欄位專為 Tekton / Argo 這類「部署在 Synapse 管理叢集內」的引擎設計。
Adapter 可在 `AdapterBuilder` 中讀取 `cfg.ClusterID` 並透過 `ClusterInformerManager.GetK8sClient()` 取得 dynamic client。

---

## 10. 安全檢查清單

建立或審核 CI 引擎設定時，請確認：

- [ ] 憑證欄位（`Token` / `Password` / `WebhookSecret` / `CABundle`）由 GORM hook 加密儲存
- [ ] JSON 回應**絕不**包含敏感欄位（`json:"-"` 標籤正確）
- [ ] `InsecureSkipVerify=true` 僅限實驗 / 內網環境；正式環境使用 `CABundle`
- [ ] 所有 `/api/v1/ci-engines` 寫入端點由 `PlatformAdminRequired` 保護
- [ ] `Create` / `Update` 事件寫入 `logger.Info`，**但禁止包含憑證值**
- [ ] 刪除前驗證沒有 Pipeline 引用此設定（FK 守護，M18b 實作）

---

## 11. GitLab CI Adapter 使用指引（M18b）

### 11.1 快速開始

**1. 建立 GitLab 連線設定（`POST /api/v1/ci-engines`）：**

```json
{
  "name": "gitlab-main",
  "engine_type": "gitlab",
  "endpoint": "https://gitlab.example.com",
  "auth_type": "token",
  "token": "glpat-xxxxxxxx",
  "extra_json": "{\"project_id\":42,\"default_ref\":\"main\"}"
}
```

- `endpoint` 填 GitLab 根網址（不含 `/api/v4`，Adapter 會自動補上）
- `token` 是 Personal Access Token 或 Project Access Token，需具備 `api` scope
- `extra_json` 必填：
  - `project_id`（數字）— GitLab 專案 ID，`Trigger` / `GetRun` / `Cancel` 必需
  - `default_ref`（選填）— 當觸發請求未指定 `ref` 時的預設分支 / tag / commit

**2. 為 Pipeline 指定 GitLab：**

```json
{
  "name": "build-saas-java-a",
  "engine_type": "gitlab",
  "engine_config_id": 1
}
```

### 11.2 對應 GitLab API 端點

| Adapter 方法      | GitLab API 呼叫                                                    | 備註                                            |
| ----------------- | ------------------------------------------------------------------ | ----------------------------------------------- |
| `IsAvailable()`   | `GET /api/v4/version`                                              | 5 秒短超時，失敗回 `false`（不拋例外）          |
| `Version()`       | `GET /api/v4/version`                                              | 回傳 `version` 欄位                             |
| `Trigger()`       | `POST /api/v4/projects/:id/pipeline`                               | `ref` 從 TriggerRequest 或 `extra.default_ref`  |
| `GetRun()`        | `GET /api/v4/projects/:id/pipelines/:id` + `/pipelines/:id/jobs`   | Jobs 失敗時仍回 pipeline 狀態（Raw 標注）       |
| `Cancel()`        | `POST /api/v4/projects/:id/pipelines/:id/cancel`                   | 已結束 pipeline 回 `ErrAlreadyTerminal`          |
| `StreamLogs()`    | `GET /api/v4/projects/:id/jobs/:job_id/trace`                      | `stepID` 需為 GitLab job id（數字字串）         |
| `GetArtifacts()`  | `GET /api/v4/projects/:id/pipelines/:id/jobs`                      | 過濾出帶 `artifacts_file` 的 jobs               |

### 11.3 狀態映射（GitLab → RunPhase）

| GitLab Status                                                                    | RunPhase           |
| -------------------------------------------------------------------------------- | ------------------ |
| `created`、`waiting_for_resource`、`preparing`、`pending`、`scheduled`、`manual` | `RunPhasePending`  |
| `running`                                                                        | `RunPhaseRunning`  |
| `success`                                                                        | `RunPhaseSuccess`  |
| `failed`                                                                         | `RunPhaseFailed`   |
| `canceled`、`skipped`                                                            | `RunPhaseCancelled`|
| （未知狀態）                                                                     | `RunPhaseUnknown`  |

### 11.4 錯誤對應

GitLab HTTP 回應透過 `gitlab/errors.go:mapHTTPStatus` 轉成 sentinel：

| GitLab HTTP | Sentinel            | HTTP Status 回前端 |
| ----------- | ------------------- | ------------------ |
| 401         | `ErrUnauthorized`   | 401                |
| 403         | `ErrUnauthorized`   | 401                |
| 404         | `ErrNotFound`       | 404                |
| 400 / 422   | `ErrInvalidInput`   | 400                |
| 5xx / 其他  | `ErrUnavailable`    | 502 / 503          |
| `Cancel` 收到 400 | `ErrAlreadyTerminal` | —（資訊性） |

### 11.5 目前限制（後續可補強）

- **Jobs 分頁**：GitLab 預設每頁 20 個 jobs；大型 pipeline 目前只取第一頁。建議 pipeline 設計保持 jobs < 20，或在 M18b follow-up 加入 pagination。
- **增量日誌**：`StreamLogs` 目前回傳一次性快照；未使用 `Range: bytes=N-` 做增量拉取。前端每次呼叫都會拿到完整內容。
- **Artifact 下載**：`GetArtifacts` 只提供 metadata + 導向 GitLab 頁面的 URL，不代理下載二進位檔。
- **Webhook 反向通道**：GitLab → Synapse 的事件推送尚未加入（Synapse 主動輪詢 `GetRun` 仍可運作）。

## 12. Jenkins Adapter 使用指引（M18c）

### 12.1 快速開始

**1. 建立 Jenkins 連線設定（`POST /api/v1/ci-engines`）：**

```json
{
  "name": "jenkins-main",
  "engine_type": "jenkins",
  "endpoint": "https://jenkins.example.com",
  "auth_type": "basic",
  "username": "ci-bot",
  "token": "11abcdef1234567890abcdef1234567890",
  "extra_json": "{\"job_path\":\"saas/java-a\"}"
}
```

- `endpoint` 填 Jenkins 根網址（無 `/api` 後綴）
- `username` + `token` 組成 HTTP Basic Auth（Jenkins API Token 請於 Jenkins 個人頁面產生）
- `extra_json` 必填：
  - `job_path`（字串）— Jenkins job 路徑，支援 folder 巢狀。例：`"saas/java-a"` → `/job/saas/job/java-a`

**2. 為 Pipeline 指定 Jenkins：**

```json
{
  "name": "build-saas-java-a",
  "engine_type": "jenkins",
  "engine_config_id": 2
}
```

### 12.2 對應 Jenkins API 端點

| Adapter 方法     | Jenkins API 呼叫                                                             | 備註                                                     |
| ---------------- | ---------------------------------------------------------------------------- | -------------------------------------------------------- |
| `IsAvailable()`  | `GET /api/json`                                                              | 5 秒短超時；讀 `X-Jenkins` header                         |
| `Version()`      | `GET /api/json`                                                              | 同上，header 缺失時回 `"unknown"` 而非錯誤                 |
| `Trigger()`      | `POST /job/:path/buildWithParameters` → `GET /queue/item/:id/api/json` 輪詢 | 10 秒內拿到 build number 回 `<num>`；超時回 `queue:<id>` |
| `GetRun()`       | `GET /job/:path/:build/api/json`（或 `/queue/item/:id/api/json`）            | `queue:<id>` 會自動 follow 成最新 build                   |
| `Cancel()`       | `POST /job/:path/:build/stop`（建置中）<br>`POST /queue/cancelItem?id=...`（佇列）| 已結束的 build 回 `ErrAlreadyTerminal`                    |
| `StreamLogs()`   | `GET /job/:path/:build/logText/progressiveText?start=0`                      | `text/plain` 快照；`stepID` 參數目前忽略                  |
| `GetArtifacts()` | `GET /job/:path/:build/api/json`                                             | 從 `artifacts[]` 抽取；`queue:*` runID 回空陣列           |

### 12.3 狀態映射（Jenkins → RunPhase）

| Jenkins `result` / `building`           | RunPhase            |
| --------------------------------------- | ------------------- |
| `building: true`（任何 result）         | `RunPhaseRunning`   |
| `"SUCCESS"`                             | `RunPhaseSuccess`   |
| `"FAILURE"` / `"UNSTABLE"`              | `RunPhaseFailed`    |
| `"ABORTED"`                             | `RunPhaseCancelled` |
| `""`（尚未分派）                         | `RunPhasePending`   |
| 其他（如 `NOT_BUILT`）                  | `RunPhaseUnknown`   |

> 注意：`UNSTABLE` 映射為 **Failed**，因為絕大多數 gating 流程不將測試失敗視為成功。未來可在 CIEngineConfig 提供 knob 讓使用者重新分類。

### 12.4 CSRF Crumb 處理

Jenkins（≥ 2.176）對 POST 要求攜帶 `Jenkins-Crumb` header。Adapter 的 crumb 機制：

1. 首次 POST 前自動 `GET /crumbIssuer/api/json`，把 crumb 與其 header 名稱（通常是 `Jenkins-Crumb`）快取
2. 之後所有 POST/DELETE/PUT 都帶上這個 crumb
3. 收到 **403**（crumb 過期）→ 清快取、重新取 crumb、**重試一次**（最多）
4. 若 Jenkins 關閉了 CSRF 保護（`crumbIssuer` 回 404）→ 後續 POST 不帶 crumb

使用者 **不需要** 手動取 crumb，也不需要重啟 Synapse 配合 controller 重啟。

### 12.5 Queue 機制（runID 兩種形態）

Jenkins 先把觸發排進 queue，排到才分配 build number。`Trigger()` 的回傳 `RunID` 因此有兩種形態：

| RunID 形態      | 意義                                        | 使用時機                                        |
| --------------- | ------------------------------------------- | ----------------------------------------------- |
| `"100"`（數字） | 已取得 build number                         | Trigger 內部 10 s 內 Queue 已分派好，正常情況   |
| `"queue:55"`    | 還在 queue，Queue Item ID = 55              | 叢集負載高、executor 都忙時才會出現             |

`GetRun()` 接收 `queue:*` 會自動 follow：若 queue item 已分派，回傳最新 build 狀態並把 RunID 升級為 build number。

### 12.6 錯誤對應

| Jenkins HTTP | Sentinel            | 備註                                        |
| ------------ | ------------------- | ------------------------------------------- |
| 401          | `ErrUnauthorized`   |                                             |
| 403          | `ErrUnauthorized`   | 可能是權限不足，也可能是 CSRF crumb 失效   |
| 404          | `ErrNotFound`       |                                             |
| 400          | `ErrInvalidInput`   |                                             |
| 5xx / 其他  | `ErrUnavailable`    |                                             |
| `Cancel` 的已結束 build | `ErrAlreadyTerminal` | 資訊性錯誤                       |

### 12.7 目前限制（後續可補強）

- **Stage 級別日誌**：M18c 只拉 build 級別 `progressiveText`；若要 per-stage log 需接 Blue Ocean 或 Pipeline Stage View 插件
- **增量日誌**：尚未利用 `X-More-Data` / `X-Text-Size` header 做 `start=<offset>` 增量拉取
- **Artifact 大小**：inline metadata 無 size 欄位；需要 HEAD 下載 URL 才能得到（或接 Blue Ocean API）
- **Webhook 反向通道**：Jenkins → Synapse 事件推送尚未加入（輪詢 `GetRun` 仍可運作）

## 13. Tekton Adapter 使用指引（M18d）

### 13.1 快速開始

**1. 確認目標叢集已安裝 Tekton（`tekton.dev/v1` CRD）：**

```bash
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
```

**2. 建立 Tekton 連線設定：**

```json
POST /api/v1/ci-engines
{
  "name": "tekton-main",
  "engine_type": "tekton",
  "cluster_id": 7,
  "extra_json": "{\"pipeline_name\":\"build-saas-java-a\",\"namespace\":\"ci-tekton\",\"service_account_name\":\"pipeline-runner\"}"
}
```

- **`endpoint` / `username` / `token` 全部不需要** — Tekton 是 K8s-native，透過 `cluster_id` 借用 Synapse 管理叢集的 kubeconfig
- `cluster_id` **必填**：指向已 import 至 Synapse 的叢集（Tekton 安裝在此叢集中）
- `extra_json`：
  - `pipeline_name`（必填）— 要觸發的 Tekton `Pipeline` CR 名稱
  - `namespace`（必填）— `PipelineRun` 與 `TaskRun` 所在命名空間
  - `service_account_name`（選填）— Pipeline 執行用的 ServiceAccount

**3. 為 Pipeline 指定 Tekton：**

```json
{
  "name": "build-saas-java-a",
  "engine_type": "tekton",
  "engine_config_id": 3
}
```

### 13.2 對應 Tekton 資源 / API

| Adapter 方法     | K8s / Tekton 操作                                                              | 備註                                           |
| ---------------- | ------------------------------------------------------------------------------ | ---------------------------------------------- |
| `IsAvailable()`  | Discovery API → `ServerResourcesForGroupVersion("tekton.dev/v1")`              | 5 秒超時；CRD 未安裝回 `false`                  |
| `Version()`      | 同上；回傳 `"tekton.dev/v1"` 字串                                                | 簡化設計（未區分 controller minor version）     |
| `Trigger()`      | `CREATE pipelineruns.tekton.dev/v1`（dynamic client）                          | Adapter 端自生成 name `synapse-run-<5-hex>`    |
| `GetRun()`       | `GET pipelineruns/:name` + `LIST taskruns?labelSelector=tekton.dev/pipelineRun=…` | TaskRun list 失敗時仍回 pipeline 狀態           |
| `Cancel()`       | `PATCH pipelineruns/:name`：`{"spec":{"status":"Cancelled"}}`                   | 已 terminal 回 `ErrAlreadyTerminal`             |
| `StreamLogs()`   | ❌ **M18d 未實作** — 需接 CoreV1 Pod log API                                    | 回傳 `ErrUnsupported`（HTTP 501）               |
| `GetArtifacts()` | 從 `status.pipelineResults[]` 抽取                                             | Kind="result"；value 放 `Digest` 欄位           |

### 13.3 狀態映射（Tekton Succeeded Condition → RunPhase）

Tekton 使用 knative `Conditions[]` 模式，其中 `type=Succeeded` 是權威狀態：

| Succeeded.Status | Succeeded.Reason                     | RunPhase            |
| ---------------- | ------------------------------------ | ------------------- |
| `True`           | any                                  | `RunPhaseSuccess`   |
| `False`          | `Cancelled` / `PipelineRunCancelled` | `RunPhaseCancelled` |
| `False`          | 其他                                 | `RunPhaseFailed`    |
| `Unknown`        | `Pending` / `PipelineRunPending`     | `RunPhasePending`   |
| `Unknown`        | 其他                                 | `RunPhaseRunning`   |
| （尚未產生）     | —                                    | `RunPhasePending`   |

### 13.4 Synapse-managed 標籤

Adapter 建立的每個 `PipelineRun` 都會帶下列 label，方便運維篩選：

| Label                          | 值                                          |
| ------------------------------ | ------------------------------------------- |
| `app.kubernetes.io/managed-by` | `synapse-ci-adapter`                        |
| `synapse.io/run-id`            | `<TriggerRequest.SnapshotID>`（0 時略）     |
| `synapse.io/pipeline-id`       | `<TriggerRequest.PipelineID>`（0 時略）     |

### 13.5 ClusterResolver 抽象

Tekton 不像 GitLab/Jenkins 透過 HTTP REST，它**透過 K8s dynamic client** 讀寫 CRD。為避免 `engine/tekton` 套件循環依賴 `internal/k8s`，框架定義了 `ClusterResolver` 介面：

```go
type ClusterResolver interface {
    Dynamic(clusterID uint) (dynamic.Interface, error)
    Discovery(clusterID uint) (discovery.DiscoveryInterface, error)
}
```

Router 層（`internal/router/tekton_cluster_resolver.go`）實作此介面並將 `ClusterInformerManager` 包裝成 resolver。單元測試可用 `client-go/dynamic/fake` + `client-go/discovery/fake` 注入 mock。

### 13.6 錯誤對應

K8s API 錯誤透過 `mapK8sError` 轉換為 sentinel：

| k8serrors 判別                      | Sentinel            | 典型情境                          |
| ----------------------------------- | ------------------- | --------------------------------- |
| `IsNotFound`                        | `ErrNotFound`       | PipelineRun 不存在                 |
| `IsUnauthorized`                    | `ErrUnauthorized`   | kubeconfig token 過期              |
| `IsForbidden`                       | `ErrUnauthorized`   | RBAC 不足                          |
| `IsBadRequest` / `IsInvalid`        | `ErrInvalidInput`   | PipelineRun schema 錯              |
| `IsConflict` / `IsAlreadyExists`    | `ErrInvalidInput`   | 衝突或重複建立                     |
| `IsServerTimeout` / `IsServiceUnavailable` / `IsInternalError` / `IsTooManyRequests` | `ErrUnavailable` | API server 問題 |
| 其他（dial / DNS）                  | `ErrUnavailable`    | 網路問題                           |
| Cancel 的 terminal run              | `ErrAlreadyTerminal` | 已結束的 run                      |

### 13.7 目前限制（後續可補強）

- **StreamLogs**：需要擴充 `ClusterResolver` 加上 `Kubernetes(clusterID) (kubernetes.Interface, error)`，然後在 `logs.go` 實作「List TaskRuns → 取 pod name → CoreV1 Pods(ns).GetLogs(pod).Stream(ctx)」。M18d follow-up。
- **Workspace 檔案型 artifacts**：Tekton Workspaces（PVC / ConfigMap）存放檔案；需執行階段把結果 copy 到特定位置才能被 `pipelineResults` 表達。目前只支援 results。
- **v1beta1 支援**：M18d 只偵測 `tekton.dev/v1`；仍在 v1beta1 的客戶需先升級 Tekton。
- **PipelineRun spec 覆寫**：`spec.timeouts`、`spec.params[].default` 等尚未透過 Adapter 覆寫；`PipelineRun` 預設沿用 `Pipeline` CRD 設定。
- **Multi-tenancy**：同 namespace 中多個 Synapse 實例會共用 `app.kubernetes.io/managed-by` label，需擴充為 instance-id 以區分。

## 14. Argo Workflows Adapter 使用指引（M18e）

### 14.1 快速開始

**1. 在目標叢集安裝 Argo Workflows：**

```bash
kubectl create namespace argo
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/latest/download/install.yaml
```

**2. 建立 Argo 連線設定：**

```json
POST /api/v1/ci-engines
{
  "name": "argo-main",
  "engine_type": "argo",
  "cluster_id": 7,
  "extra_json": "{\"workflow_template_name\":\"build-app\",\"namespace\":\"ci-argo\",\"service_account_name\":\"workflow-runner\"}"
}
```

- **不需 `endpoint` / `username` / `token`** — 同 Tekton，透過 `cluster_id` 借用叢集 kubeconfig
- `extra_json.workflow_template_name`（必填）— 引用的 `WorkflowTemplate` 名稱
- `extra_json.namespace`（必填）— `Workflow` + 子 Pod 所在命名空間
- `extra_json.service_account_name`（選填）— 設定 `spec.serviceAccountName`

### 14.2 對應 Argo 資源

| Adapter 方法     | K8s 操作                                                              | 備註                                          |
| ---------------- | --------------------------------------------------------------------- | --------------------------------------------- |
| `IsAvailable()`  | Discovery: `ServerResourcesForGroupVersion("argoproj.io/v1alpha1")`   | 5 秒超時                                      |
| `Trigger()`      | `CREATE workflows.argoproj.io/v1alpha1` 帶 `spec.workflowTemplateRef` | Adapter 自生成 name `synapse-run-<5-hex>`     |
| `GetRun()`       | `GET workflows/:name`（無需 List，所有 nodes 已內嵌於 status）         | node tree 壓平成 StepStatus 陣列              |
| `Cancel()`       | `PATCH workflows/:name` with `{"spec":{"shutdown":"Terminate"}}`      | 已 terminal 回 `ErrAlreadyTerminal`            |
| `StreamLogs()`   | ❌ 同 Tekton，回 `ErrUnsupported`（M18e follow-up）                    |                                               |
| `GetArtifacts()` | 遍歷 `status.nodes[].outputs.artifacts[]`，支援 http/s3/gcs/oss/azure | 多 backend 支援，優先 http URL                |

### 14.3 狀態映射（Argo status.phase → RunPhase）

| Argo Phase  | RunPhase            |
| ----------- | ------------------- |
| `Pending`   | `RunPhasePending`   |
| `Running`   | `RunPhaseRunning`   |
| `Succeeded` | `RunPhaseSuccess`   |
| `Failed`    | `RunPhaseFailed`    |
| `Error`     | `RunPhaseFailed`    |
| （未產生）  | `RunPhasePending`   |
| 其他        | `RunPhaseUnknown`   |

## 15. GitHub Actions Adapter 使用指引（M18e）

### 15.1 快速開始

**1. 在 GitHub 建立 PAT（或 GitHub App token）：** 需 `repo` + `workflow` scope

**2. 建立 GitHub 連線設定：**

```json
POST /api/v1/ci-engines
{
  "name": "github-main",
  "engine_type": "github",
  "endpoint": "",
  "auth_type": "token",
  "token": "ghp_xxxxxxxxxx",
  "extra_json": "{\"owner\":\"my-org\",\"repo\":\"my-repo\",\"workflow_id\":\"build.yml\",\"default_ref\":\"main\"}"
}
```

- `endpoint` 空字串 → 使用 `https://api.github.com`（公開）。GHE 填 `https://github.example.com`
- `token` 為 PAT / Fine-grained PAT / GitHub App installation token
- `extra_json.owner` + `repo` 指向 GitHub 儲存庫
- `extra_json.workflow_id` 可為檔名（`build.yml`）或數字 ID
- `extra_json.default_ref`（選填）— TriggerRequest.Ref 未給時使用

### 15.2 對應 GitHub REST API

| Adapter 方法     | GitHub API                                                                        | 備註                                          |
| ---------------- | --------------------------------------------------------------------------------- | --------------------------------------------- |
| `IsAvailable()`  | `GET /meta`                                                                       | 公開端點，快速連線測試                        |
| `Version()`      | 固定回傳 `2022-11-28`（`X-GitHub-Api-Version` 值）                                 | GitHub 無運行版本 header                      |
| `Trigger()`      | `POST /actions/workflows/:id/dispatches`（204）→ 輪詢 `/actions/runs` 拿 run_id    | 10 秒超時，超時回 `dispatch:<ref>@<epoch>`    |
| `GetRun()`       | `GET /actions/runs/:id` + `/jobs`；識別 `dispatch:*` 並自動解析                    | jobs 失敗時仍回 run-level 狀態                |
| `Cancel()`       | `POST /actions/runs/:id/cancel`                                                   | 409 Conflict → `ErrAlreadyTerminal`            |
| `StreamLogs()`   | `GET /actions/jobs/:job_id/logs`（text/plain）                                     | **M18e 完整實作**；stepID = GitHub job id      |
| `GetArtifacts()` | `GET /actions/runs/:id/artifacts`                                                 | 含 archive_download_url + size 欄位            |

### 15.3 狀態映射

| Status                                         | Conclusion                                  | RunPhase            |
| ---------------------------------------------- | ------------------------------------------- | ------------------- |
| `queued` / `requested` / `waiting` / `pending` | —                                           | `RunPhasePending`   |
| `in_progress`                                  | —                                           | `RunPhaseRunning`   |
| `completed`                                    | `success`                                   | `RunPhaseSuccess`   |
| `completed`                                    | `failure` / `timed_out` / `startup_failure` | `RunPhaseFailed`    |
| `completed`                                    | `cancelled` / `skipped`                     | `RunPhaseCancelled` |
| `completed`                                    | `action_required`                           | `RunPhasePending`   |
| `completed`                                    | `neutral` / `stale`                         | `RunPhaseUnknown`   |

### 15.4 Dispatch → Run ID 發現機制

GitHub `workflow_dispatch` 回 204 **不帶 run id**。Adapter 策略：

1. 觸發前記下 cutoff time（當下 -10s，容忍時鐘偏移）
2. POST `/dispatches` 取得 204
3. 輪詢 `/actions/workflows/:id/runs?event=workflow_dispatch&branch=<ref>` 最多 10 秒
4. 找到 `created_at >= cutoff` 的 run 即為目標
5. 超時 → 回傳 `dispatch:<ref>@<epoch>` 占位符；`GetRun()` 認得這個前綴並自動解析

### 15.5 錯誤對應

| GitHub HTTP      | Sentinel            | 備註                                      |
| ---------------- | ------------------- | ----------------------------------------- |
| 401              | `ErrUnauthorized`   |                                           |
| 403              | `ErrUnauthorized`   | 可能是權限不足，也可能是 rate-limit       |
| 404              | `ErrNotFound`       |                                           |
| 400 / 422        | `ErrInvalidInput`   | 422 常見於未知的 workflow input           |
| 5xx / 其他       | `ErrUnavailable`    |                                           |
| Cancel 收到 409  | `ErrAlreadyTerminal` | Run 已 completed                          |

### 15.6 目前限制

- **Rate-limit 重試**：未實作；需要時在高階層加 exponential backoff
- **Enterprise PAT scope**：GHE 可能需要 `workflow` + `repo:status` 等額外 scope
- **Log 增量**：一次性拉完整 job log；GitHub `/logs` 不支援 `Range` 增量
- **前端 UI**：M18e-UI 延後

## 16. 後續里程碑

| 里程碑           | 範圍                             | 狀態     |
| ---------------- | -------------------------------- | -------- |
| M18a             | Adapter Framework + Native       | ✅ 完成   |
| M18b             | GitLab CI Adapter                | ✅ 完成（後端）|
| M18c             | Jenkins Adapter                  | ✅ 完成（後端）|
| M18d             | Tekton Adapter                   | ✅ 完成（後端，除 StreamLogs）|
| M18e             | Argo Workflows + GitHub Actions  | ✅ 完成（後端）|
| M18b-UI          | GitLab 連線設定前端              | 🟡 待排程 |
| M18c-UI          | Jenkins 連線設定前端             | 🟡 待排程 |
| M18d-UI          | Tekton 連線設定前端              | 🟡 待排程 |
| M18e-UI          | Argo + GitHub 連線設定前端       | 🟡 待排程 |
| M18d/e Follow-up | Tekton / Argo StreamLogs（Pod log）| 🟡 待排程 |

---

**最後更新：** 2026-04-16
