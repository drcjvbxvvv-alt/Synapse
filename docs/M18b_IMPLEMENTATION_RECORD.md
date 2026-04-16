# M18b 實作紀錄 — GitLab CI Adapter

| 項目        | 內容                                                |
| ----------- | --------------------------------------------------- |
| 里程碑      | M18b — GitLab CI Adapter（後端）                    |
| 狀態        | ✅ 完成（2026-04-16）                                |
| 對應 ADR    | [ADR-015](./adr/ADR-015-CI-Engine-Adapter-Pattern.md) |
| 使用指南    | [CI_ENGINE_ADAPTERS.md §11](./CI_ENGINE_ADAPTERS.md) |
| 前置里程碑  | [M18a](./M18a_IMPLEMENTATION_RECORD.md)（Adapter Framework）|

---

## 1. 交付摘要

**一句話描述：** Synapse 新增 GitLab CI Adapter，讓 Pipeline 可直接委派至遠端 GitLab 專案執行，透過 REST API v4 完成觸發、查詢、取消、日誌串流與 artifact 列舉。

### 核心產出

| 層級           | 產出物                                                              |
| -------------- | ------------------------------------------------------------------- |
| **HTTP 客戶端** | `client.go`（PRIVATE-TOKEN 認證、TLS、5 MiB 大小限制、ctx 傳遞）      |
| **錯誤映射**    | `errors.go`（HTTP 狀態 → `engine.Err*` sentinel 的單一決策點）       |
| **DTO**        | `types.go`（`gitlabPipeline`、`gitlabJob`、`gitlabVersion` 等）      |
| **Adapter**    | `adapter.go` + `config.go`（Type / IsAvailable / Version / Capabilities） |
| **狀態映射**    | `status.go`（GitLab 11 種狀態 → 6 種 `RunPhase`）                    |
| **Trigger**    | `trigger.go`（POST `/projects/:id/pipeline` + variables）            |
| **GetRun**     | `runs.go`（pipeline 狀態 + 子 jobs 合併為 `RunStatus`）              |
| **Cancel**     | `cancel.go`（含已終止的 `ErrAlreadyTerminal` 處理）                  |
| **StreamLogs** | `logs.go`（job trace 文字串流）                                      |
| **GetArtifacts** | `artifacts.go`（過濾帶 artifacts 的 jobs + 建構 GitLab job URL）    |
| **Factory 註冊** | `register.go` + `routes_ci_engine.go`（啟動時自動註冊，容錯 `ErrAlreadyRegistered`） |

---

## 2. 階段實作流程

| 階段       | 內容                                                              | 測試數 | 狀態 |
| ---------- | ----------------------------------------------------------------- | ------ | ---- |
| Stage 1    | HTTP 客戶端 + 錯誤映射（`client` / `errors` / `types`）            | 18     | ✅    |
| Stage 2    | Adapter 骨架（Type / IsAvailable / Version / Capabilities）        | 11     | ✅    |
| Stage 3    | Trigger + GetRun + 狀態映射                                       | 18     | ✅    |
| Stage 4    | Cancel + StreamLogs + GetArtifacts                                | 19     | ✅    |
| Stage 5    | Factory 註冊 + CIEngineService 整合測試                           | 5 + 1  | ✅    |
| Stage 6    | 文件更新（ADR-015、CI_ENGINE_ADAPTERS、本檔）                     | —      | ✅    |

**總計：72 個單元測試（gitlab 套件內）+ 1 個跨套件整合測試，全部通過（含 `-race`）。**

---

## 3. 檔案清單

### 3.1 新增檔案（14 個）

```
internal/services/pipeline/engine/gitlab/      # M18b 專用子套件
├── types.go              # API DTOs
├── errors.go             # HTTP status → sentinel mapping
├── client.go             # HTTP 客戶端（TLS、ctx、5 MiB 限制）
├── config.go             # ExtraConfig：project_id、default_ref
├── adapter.go            # Adapter struct、Type、IsAvailable、Version、Capabilities
├── status.go             # GitLab status → RunPhase
├── trigger.go            # POST /projects/:id/pipeline
├── runs.go               # GetRun：pipeline + jobs 合併
├── cancel.go             # POST /pipelines/:id/cancel
├── logs.go               # GET /jobs/:id/trace
├── artifacts.go          # 從 jobs 抽取 artifacts metadata
├── register.go           # Register() / MustRegister()
├── client_test.go        # 18 個測試
├── adapter_test.go       # 11 個測試
├── trigger_test.go       # 11 個測試
├── runs_test.go          # 7 個測試
├── cancel_test.go        # 8 個測試
├── logs_test.go          # 6 個測試
├── artifacts_test.go     # 6 個測試
└── register_test.go      # 5 個測試

internal/services/
└── ci_engine_service_gitlab_test.go   # 1 個端到端整合測試

docs/
└── M18b_IMPLEMENTATION_RECORD.md      # 本檔
```

### 3.2 修改檔案（4 個）

| 檔案                                                    | 修改內容                                                              |
| ------------------------------------------------------- | --------------------------------------------------------------------- |
| `internal/services/pipeline/engine/errors.go`           | 新增 `ErrAlreadyRegistered` sentinel                                  |
| `internal/services/pipeline/engine/factory.go`          | `Register()` 回傳的 error 用 `%w` 包裝 `ErrAlreadyRegistered`          |
| `internal/router/routes_ci_engine.go`                   | 啟動時自動 `gitlab.Register(engine.Default())`，容錯 `ErrAlreadyRegistered` |
| `docs/adr/ADR-015-CI-Engine-Adapter-Pattern.md`         | M18b 範圍全部標 ✅                                                    |
| `docs/CI_ENGINE_ADAPTERS.md`                            | 新增 §11「GitLab CI Adapter 使用指引」                                 |

---

## 4. 測試與品質驗證

### 4.1 測試統計

| 套件                                                            | 測試數 | 說明                                       |
| --------------------------------------------------------------- | ------ | ------------------------------------------ |
| `internal/services/pipeline/engine/gitlab`                      | 72     | HTTP 客戶端 + Adapter + Factory 註冊測試    |
| `internal/services`（`ci_engine_service_gitlab_test.go`）        | 1      | 端到端：註冊 → seed DB → fake GitLab → 呼叫 |
| **M18b 新增合計**                                                | **73** |                                            |

累計所有 CI 引擎相關測試（M18a + M18b）：**76 + 73 = 149 個**。

### 4.2 驗證指令

```bash
go build ./...                              # ✅ 乾淨
go vet ./...                                # ✅ 乾淨
go test ./internal/services/pipeline/engine/... \
        ./internal/services/... \
        ./internal/handlers/... \
        ./internal/models/... \
    -count=1 -race -timeout=180s            # ✅ 全部通過
```

### 4.3 品質檢查清單

- [x] 遵循 `CLAUDE.md §4` 錯誤處理（`fmt.Errorf(...%w...)` 包裝 sentinel）
- [x] 遵循 `CLAUDE.md §5` Context 使用（所有方法第一參數 ctx，每個 http.Request 都攜帶）
- [x] 遵循 `CLAUDE.md §8` Observer Pattern（`IsAvailable()` 永不回錯，失敗 → false）
- [x] 遵循 `CLAUDE.md §9` 日誌（`logger.Warn` 僅用於背景註冊的非致命錯誤，**不含** token）
- [x] 遵循 `CLAUDE.md §10` 安全規則
  - [x] `InsecureSkipVerify` 走設定注入而非硬編碼（含 `//nolint:gosec` 明確註解）
  - [x] `PRIVATE-TOKEN` header 僅在發送請求時設定，不進 URL、不進 log
  - [x] HTTP 錯誤預覽截斷 512 bytes，避免 response body 含敏感資料外洩
- [x] HTTP response body 有 5 MiB 上限（防 OOM）
- [x] HTTP client 有合理超時（15s 預設，探測用 5s）
- [x] `-race` 測試通過

---

## 5. 關鍵設計決策

### 5.1 為何把子套件命名為 `gitlab` 而非 `gitlabadapter`？

短名字更好。Go 慣例鼓勵套件名稱與其主要導出類型互補（`gitlab.Adapter` 讀起來比 `gitlabadapter.Adapter` 自然）。套件分在 `internal/services/pipeline/engine/gitlab/` 目錄下，路徑本身已經足夠識別用途。

### 5.2 為何 `Trigger` 回傳的 `RunID` 與 `ExternalID` 相同？

對 GitLab 這類外部引擎，Synapse 沒有比「GitLab pipeline id」更適合當作 opaque run handle 的東西。將兩者統一讓 `GetRun(runID)` / `Cancel(runID)` 不需要額外的 ID 映射表。

如果之後 Synapse 端想要自己的 `PipelineRun` 紀錄映射到 GitLab run，會由更高層（`pipeline_service`）完成；Adapter 本身只負責「外部系統眼中的 run id」。

### 5.3 為何 `GetRun` 在 jobs 端點失敗時仍回傳 pipeline 狀態？

遵循 `CLAUDE.md §8` Observer Pattern 的精神：**單一下游故障不該阻塞整個查詢**。Jobs 端點失敗（5xx / 網路抖動）時：

1. Pipeline 狀態（`running` / `success` / …）仍能提供給前端
2. 失敗原因寫入 `RunStatus.Raw` 讓運維可診斷
3. `Steps` 為空，UI 會顯示「Jobs 暫時不可用」而非整頁崩潰

### 5.4 為何 `Cancel` 把 400 映射成 `ErrAlreadyTerminal` 而非 `ErrInvalidInput`？

GitLab 的 `POST /pipelines/:id/cancel` 在 pipeline 已結束時會回 400 "Pipeline cannot be canceled"。這本質上是「操作沒意義」而非「請求格式錯誤」。將它翻譯為 `ErrAlreadyTerminal` 讓呼叫端可選擇視為成功（類似 idempotent delete）。

### 5.5 為何 `StreamLogs` 的 `stepID` 必填？

GitLab 的日誌以 **job** 為單位，不支援 pipeline-level aggregated log。Adapter 若接受空 `stepID` 只能任意挑一個 job 或回空內容，兩者都容易誤導。直接回 `ErrInvalidInput` 是最清晰的契約。

M18b-UI 的前端需在 Run 詳情頁列出所有 jobs 並讓使用者選擇。

### 5.6 為何 `Register` 透過 `routes_ci_engine.go` 而非 `init()` 自動註冊？

`init()` 自動註冊會在 **所有** import 該套件的情境下執行 — 包括獨立測試。這會導致測試裡的 `engine.NewFactory()` 無法乾淨地控制要註冊哪些 adapter。

改在啟動程式碼顯式呼叫 `Register`，好處是：

1. 測試可用 `engine.NewFactory()` 建立乾淨的 factory
2. 未來可依據設定檔動態啟用 / 禁用特定 adapter
3. 註冊順序可觀察

### 5.7 為何 `ErrAlreadyRegistered` 是 sentinel 而非字串比對？

早期版本的 `Factory.Register` 只回 `fmt.Errorf("… already registered")`，路由層若想判定「這是冪等情況」只能 `strings.Contains`，脆弱。

加入 `ErrAlreadyRegistered` sentinel 後：

```go
if err := gitlab.Register(f); err != nil {
    if !errors.Is(err, engine.ErrAlreadyRegistered) {
        // 真正的錯誤
    }
}
```

此改動相容 M18a 的現有測試（它們只檢查 `err != nil`）。

---

## 6. 安全設計重點

### 6.1 憑證處理鏈

```
CIEngineConfig.Token (加密)
    ↓ AfterFind hook 自動解密
in-memory Token (plaintext)
    ↓ clientConfig.Token
http.Request.Header "PRIVATE-TOKEN"
    ↓ (每個請求都是全新 header，不持久化)
GitLab API
```

- Token **絕不**出現在 URL、log、錯誤訊息中
- Token 以 `json:"-"` 遮蔽，API 回應永不洩漏
- HTTP 錯誤預覽最多 512 bytes，再長的 body 被截斷（避免 GitLab 在某些錯誤回應中帶回 request body 造成二次洩漏）

### 6.2 TLS 設定

- 預設 `MinVersion: TLS 1.2`
- `InsecureSkipVerify` 走 `CIEngineConfig.InsecureSkipVerify` 開關（不硬編碼）
- `CABundle` 接受 PEM 字串；無效 PEM 會在 `newClient` 階段回 `ErrInvalidInput`，不會到請求時才失敗

### 6.3 Response 大小限制

- 所有 JSON 回應透過 `io.LimitReader` 限制 5 MiB
- `StreamLogs` 的 trace 同樣包裝在 `limitedReadCloser` 內
- 設計目的：防止惡意或故障的 GitLab 回傳巨大 body 拖垮 Synapse

---

## 7. 向後相容性

| 項目                                 | 保證                                                              |
| ------------------------------------ | ----------------------------------------------------------------- |
| M18a 新增的 `engine` 套件            | 接口**完全不動**（僅新增 `ErrAlreadyRegistered` sentinel）        |
| Native Adapter                       | 不受影響                                                          |
| `CIEngineService` / `CIEngineHandler`| 不需變更（Factory 是單例，啟動時追加註冊即可）                     |
| 既有 API 端點                        | 無變更                                                            |
| DB Schema                            | 無變更（M18b 所有欄位皆復用 M18a 已建立的 `ci_engine_configs`）     |

---

## 8. 遺留事項與後續工作

### 8.1 M18b 範圍內延後項目

- **Jobs 分頁**：GitLab 預設每頁 20 job，大型 pipeline 需加 `per_page` + 迴圈
- **增量日誌**：`Range: bytes=N-` 支援，前端才能節省頻寬
- **Artifact 直接下載代理**：目前只回導向連結，未來可加 `/api/v1/ci-engines/:id/runs/:run_id/artifacts/:name/download` 串流代理
- **Webhook 反向整合**：GitLab → Synapse 的 `pipeline` 事件推送可避免輪詢，節省大量 API 呼叫

### 8.2 M18b-UI（前端）任務

- 新增「系統設定 → CI 引擎」頁面支援 GitLab 設定 CRUD
- Pipeline 建立 / 編輯時，`engine_type=gitlab` 要顯示 `project_id` 與 `default_ref` 欄位
- Run 詳情頁需列出 jobs 以便使用者選擇要看哪個 job 的日誌

### 8.3 後續里程碑

| 里程碑  | 範圍                            | 預估 | 狀態 |
| ------- | ------------------------------- | ---- | ---- |
| M18b-UI | GitLab 連線設定前端             | 1 週 | ✅ 完成（2026-04-16）|
| M18c    | Jenkins Adapter + UI            | 2 週 | ✅ 完成（2026-04-16）|
| M18d    | Tekton Adapter + UI             | 2 週 | ✅ 完成（2026-04-16）|
| M18e    | Argo Workflows / GitHub Actions | 2 週 | ✅ 完成（2026-04-16）|

---

## 9. Lessons Learned（實作心得）

1. **分檔比單檔好**
   M18b 把 Adapter 方法拆成 `trigger.go` / `runs.go` / `cancel.go` / `logs.go` / `artifacts.go`，每個檔案 < 100 行。code review 時比單一 500 行檔案好追蹤得多。

2. **httptest 比 mock library 好用**
   GitLab API 契約多樣（text/plain、JSON 陣列、JSON 物件、5xx），直接用 `httptest.NewServer` 配 `http.HandlerFunc` 寫 mock 比任何 stub library 都精準；副作用是每個測試都會啟動一個 port，用 `t.Cleanup` 確保清理。

3. **Error mapping 早一點集中**
   把 HTTP status 對應 sentinel 的邏輯放在一個 `mapHTTPStatus` 函式、一個測試覆蓋所有分支，遠比在每個方法 switch-case 分散判斷穩定。後續要加 Jenkins / Tekton Adapter 時可直接複製這個模式。

4. **「UI 不拋例外」是個持續的契約**
   `IsAvailable()` 看似微不足道，但實際寫起來有誘惑：想把 http err 原樣回傳好讓呼叫端有更多資訊。抵抗住這個誘惑、永遠 `return false` 才能讓 UI 頁面穩定。

5. **Stub 實作先過編譯，再逐步替換**
   Stage 2 放的 `return ErrUnsupported` 占位實作，讓 `_ CIEngineAdapter = (*Adapter)(nil)` compile-time assertion 在 Stage 2 就能保障介面完整；Stage 3-4 逐步把它們換成真實 code — 任何漏實作的方法在切檔時會立刻被發現。

---

## 10. 檢查清單（驗收用）

- [x] `go build ./...` 乾淨
- [x] `go vet ./...` 乾淨
- [x] 所有 73 個 M18b 新增測試通過（含 `-race`）
- [x] M18a 既有 76 個測試**未受影響**
- [x] `services` / `handlers` / `models` / `router` 的其他測試未受影響
- [x] `ADR-015` M18b 項目全標 ✅
- [x] `CI_ENGINE_ADAPTERS.md` §11 新增 GitLab 使用指引
- [x] 本實作紀錄文件完成

---

**決策者：** Architecture Team
**實作者：** Architecture Team
**最後更新：** 2026-04-16
