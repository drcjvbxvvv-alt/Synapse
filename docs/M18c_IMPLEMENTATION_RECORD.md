# M18c 實作紀錄 — Jenkins Adapter

| 項目        | 內容                                                |
| ----------- | --------------------------------------------------- |
| 里程碑      | M18c — Jenkins Adapter（後端）                       |
| 狀態        | ✅ 完成（2026-04-16）                                |
| 對應 ADR    | [ADR-015](./adr/ADR-015-CI-Engine-Adapter-Pattern.md) |
| 使用指南    | [CI_ENGINE_ADAPTERS.md §12](./CI_ENGINE_ADAPTERS.md) |
| 前置里程碑  | [M18a](./M18a_IMPLEMENTATION_RECORD.md)（Framework）、[M18b](./M18b_IMPLEMENTATION_RECORD.md)（GitLab Adapter）|

---

## 1. 交付摘要

**一句話描述：** Synapse 新增 Jenkins Adapter，透過 REST API + CSRF Crumb 保護機制觸發、查詢、取消、取日誌、列舉 artifacts，並原生支援 Jenkins 的 Queue → Build 兩階段排程。

### 核心產出

| 層級             | 產出物                                                              |
| ---------------- | ------------------------------------------------------------------- |
| **HTTP 客戶端**   | `client.go`（Basic Auth、TLS、5 MiB 限制、crumb 自動重試）           |
| **CSRF Crumb**   | `crumb.go`（執行緒安全快取，403 時自動重取 + 單次重試）              |
| **錯誤映射**      | `errors.go`（Jenkins HTTP 狀態 → `engine.Err*` sentinel）            |
| **DTO**          | `types.go`（`jenkinsBuild`、`queueItem`、`crumbResponse` 等）         |
| **Adapter**      | `adapter.go`（Type/IsAvailable/Version/Capabilities，用 `X-Jenkins` header 取版本） |
| **Config 解析**   | `config.go`（`ExtraConfig.JobPath`、`buildJobURLPath` 轉換 folder 巢狀） |
| **狀態映射**      | `status.go`（Jenkins `result`+`building` → 6 種 RunPhase）          |
| **Trigger**      | `trigger.go`（POST + Queue 輪詢，`queue:<id>` 降級）                  |
| **GetRun**       | `runs.go`（同時支援 build number 與 `queue:*` RunID）                |
| **Cancel**       | `cancel.go`（queue/cancelItem 或 build/stop，雙路徑）                |
| **StreamLogs**   | `logs.go`（`progressiveText` 快照）                                  |
| **GetArtifacts** | `artifacts.go`（從 build JSON 抽取 `artifacts[]`）                    |
| **Factory 註冊**  | `register.go` + `routes_ci_engine.go`                               |

---

## 2. 階段實作流程

| 階段       | 內容                                                                 | 測試數 | 狀態 |
| ---------- | -------------------------------------------------------------------- | ------ | ---- |
| Stage 1    | HTTP 客戶端 + CSRF Crumb 快取 + 錯誤映射                              | 21     | ✅    |
| Stage 2    | Adapter 骨架（Type/IsAvailable/Version/Capabilities）                 | 20     | ✅    |
| Stage 3    | Trigger（含 Queue 輪詢）+ GetRun（含 `queue:*` 自動 follow）+ 狀態映射 | 22     | ✅    |
| Stage 4    | Cancel（雙路徑）+ StreamLogs + GetArtifacts                           | 22     | ✅    |
| Stage 5    | Factory 註冊 + 啟動整合                                               | 5 + 1  | ✅    |
| Stage 6    | 文件更新（ADR-015、CI_ENGINE_ADAPTERS、本檔）                         | —      | ✅    |

**合計：105 個 jenkins 套件內測試 + 1 個跨套件整合測試，全部通過（含 `-race`）。**

---

## 3. 檔案清單

### 3.1 新增檔案（17 個）

```
internal/services/pipeline/engine/jenkins/
├── types.go              # API DTOs（Build、QueueItem、Crumb）
├── errors.go             # HTTP status → sentinel + isCSRFError
├── crumb.go              # 執行緒安全的 CSRF crumb 快取
├── client.go             # HTTP 客戶端 + doJSON + doMutation + doRaw
├── config.go             # ExtraConfig.JobPath + buildJobURLPath
├── adapter.go            # Adapter struct、Type、IsAvailable、Version、Capabilities
├── status.go             # Jenkins (result, building) → RunPhase
├── trigger.go            # POST + Queue 輪詢 + build page URL 構建
├── runs.go               # GetRun by build number / queue id
├── cancel.go             # queue/cancelItem 或 build/stop
├── logs.go               # progressiveText snapshot
├── artifacts.go          # 從 build JSON 抽 artifacts
├── register.go           # Register / MustRegister
├── client_test.go        # 21 個測試
├── adapter_test.go       # 20 個測試
├── trigger_test.go       # 10 個測試
├── runs_test.go          # 12 個測試
├── cancel_test.go        # 8 個測試
├── logs_test.go          # 7 個測試
├── artifacts_test.go     # 7 個測試
└── register_test.go      # 5 個測試

internal/services/
└── ci_engine_service_jenkins_test.go   # 1 個端到端整合測試

docs/
└── M18c_IMPLEMENTATION_RECORD.md       # 本檔
```

### 3.2 修改檔案（2 個）

| 檔案                                         | 修改內容                                       |
| -------------------------------------------- | ---------------------------------------------- |
| `internal/router/routes_ci_engine.go`        | 啟動時追加 `jenkins.Register(engine.Default())` |
| `docs/adr/ADR-015-CI-Engine-Adapter-Pattern.md`、`docs/CI_ENGINE_ADAPTERS.md` | M18c 全標 ✅；新增 §12 使用指引 |

---

## 4. 測試與品質驗證

### 4.1 測試統計

| 套件                                        | 測試數 |
| ------------------------------------------- | ------ |
| `internal/services/pipeline/engine/jenkins`  | 105    |
| `internal/services`（M18c 整合測試）         | 1      |
| **M18c 新增合計**                            | **106** |

CI 引擎子系統累計測試（M18a + M18b + M18c）：**76 + 73 + 106 = 255 個**。

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
- [x] 遵循 `CLAUDE.md §5` Context 使用（所有方法第一參數 ctx）
- [x] 遵循 `CLAUDE.md §8` Observer Pattern（`IsAvailable()` 永不回錯）
- [x] 遵循 `CLAUDE.md §10` 安全規則
  - [x] `InsecureSkipVerify` 走設定注入（附 `//nolint:gosec` 註解）
  - [x] Basic Auth 僅透過 `req.SetBasicAuth`，不進 URL、不進 log
  - [x] CSRF crumb 只在記憶體快取、不寫 DB
  - [x] HTTP 錯誤預覽截斷 512 bytes
- [x] HTTP response body 5 MiB 上限
- [x] `crumbCache` 使用 `sync.Mutex` 保障並發安全
- [x] `-race` 測試通過

---

## 5. 關鍵設計決策

### 5.1 為何 CSRF Crumb 自動重試而非讓呼叫端處理？

Crumb 過期是 Jenkins 常見運維場景（controller 重啟、token 輪換）。若讓呼叫端每次 Trigger/Cancel 都自己處理 403 retry，會造成：

- 每個 Adapter 方法都重複一樣的重試 boilerplate
- 呼叫端無法區分「crumb 過期」與「權限不足」—— 兩者都是 403

方案：在 `doMutation` 層級集中處理。第一次 403 → 清快取、重新取 crumb、重試一次。若第二次還是 403，視為真正的權限錯誤，回 `ErrUnauthorized`。這讓 Adapter 其它程式碼寫得像「CSRF 不存在」一樣。

### 5.2 為何 Trigger 回傳可能是 `queue:<id>` 而非等到 build number？

Jenkins 的 Queue 可能塞很久（executor 全滿）。如果 `Trigger` 同步等到 build number 出現，會：

- 前端 `POST /pipelines/:id/runs` 可能 block 超過 30 s
- ctx 超時後呼叫端不知道 run 到底有沒有建立

方案：設 10 s 輪詢預算，超過就回 `queue:<queue_id>` 讓呼叫端有個 opaque RunID 之後繼續 poll。`GetRun` 認得這個前綴，自動 follow 到 build。這樣「立即建立」與「長時間排隊」用同一個契約表達。

### 5.3 為何 `GetRun` 收到 `queue:*` 會自動 follow 成 build number？

符合最小驚訝原則：呼叫端傳入 `queue:55`，若 Jenkins 已經分派成 build 77，回傳 `RunID=77` 的完整建構狀態。呼叫端下次 poll 時若存下新 RunID，之後就直接查 build 端點，不再繞 Queue。

若 Queue 項被 cancel，回傳 `RunID=queue:55`、`Phase=Cancelled`、`Raw="queue-cancelled"` —— 保留原始 ID 讓 UI 可以區分「build 被 cancel」vs「build 還沒出生就 cancel」。

### 5.4 為何 Cancel 有兩條路徑？

Jenkins 的 build 生命週期：`queued → running → finished`。取消方式：

- 已 running → `POST /job/:path/:num/stop`（graceful stop，允許清理 steps）
- 還在 queue → `POST /queue/cancelItem?id=...`（連 build 都不會建立）

Adapter 根據 RunID 自動判斷：`queue:*` 走第一條，數字走第二條。第二條還會先 `GetRun` 看一下狀態，若已 terminal 就回 `ErrAlreadyTerminal` 避免多餘的 HTTP 呼叫。

### 5.5 為何 `UNSTABLE` 映射成 Failed？

Jenkins 有三種「完成但不完美」的結果：

- `SUCCESS`：全部通過
- `UNSTABLE`：建置成功但測試失敗
- `FAILURE`：建置本身失敗

大多數 CI gating 流程（「MR 必須 green 才能合入」「production 部署前必須 green」）都不應把 UNSTABLE 視為成功。為了安全預設，映射為 `RunPhaseFailed`。

未來可在 CIEngineConfig 加一個 `treat_unstable_as` 欄位讓使用者自訂（`success` / `failed`）；M18c 不處理，保持簡單。

### 5.6 為何 `X-Jenkins` header 缺失時回 `"unknown"` 而非錯誤？

反向代理（Nginx / Traefik / cloud LB）有時會默默移除非白名單 header。若 Version() 因此失敗，整個 `IsAvailable()` 跟著回 `false`，UI 會標 Jenkins 為離線 —— 誤導。

折衷：回傳 sentinel 字串 `"unknown"`，讓 IsAvailable() 仍為 true；UI 可選擇性地把 `"unknown"` 顯示為黃色警告。

### 5.7 為何 Adapter 對 `stepID` 參數忽略？

Jenkins 的日誌單位是 **build**，不是 **stage**。要取 per-stage log 必須用 Blue Ocean REST API 或 Pipeline Stage View 插件 —— 兩者都不保證每個 Jenkins 安裝都有。

M18c 選擇簡化：`StreamLogs(runID, stepID)` 忽略 `stepID`，回傳整個 build log。`CIEngineAdapter` 介面的一致性由 GitLab adapter 保留（GitLab 的 `stepID` 必填，是 job id）。

---

## 6. 安全設計重點

### 6.1 憑證處理鏈

```
CIEngineConfig.Username + CIEngineConfig.Token (Token 加密)
    ↓ AfterFind hook 自動解密 Token
in-memory Username + Token
    ↓ req.SetBasicAuth(Username, Token)  (每個請求)
Jenkins API (HTTP Authorization: Basic base64(u:p))
```

- Token 以 `json:"-"` 遮蔽，API 回應永不洩漏
- Basic Auth 的密碼部分 **不進 URL**、**不進 log**、**不進 crumb 快取**
- HTTP 錯誤預覽 512 bytes 上限

### 6.2 CSRF Crumb 快取

- 快取只在記憶體（`*crumbCache` 是 `*client` 的一個欄位）
- 跨 adapter 實例不共享（兩個 CIEngineConfig 各自有 crumb）
- `invalidate()` 立即清除（403 retry 路徑）

### 6.3 TLS / 大小上限

同 M18b：TLS 1.2+、`InsecureSkipVerify` 使用者選擇、`CABundle` PEM 支援、response body 5 MiB 上限。

---

## 7. 向後相容性

| 項目                                 | 保證                                                              |
| ------------------------------------ | ----------------------------------------------------------------- |
| `engine` 套件                        | 不變                                                              |
| Native / GitLab Adapter              | 不受影響                                                          |
| `CIEngineService` / `CIEngineHandler`| 不需變更                                                          |
| 既有 API 端點                        | 無變更                                                            |
| DB Schema                            | 無變更（Jenkins 用 `username` 欄位 + `token` 欄位，都已存在）      |

---

## 8. 遺留事項與後續工作

### 8.1 M18c 範圍內延後項目

- **Stage 級日誌**：需接 Blue Ocean 或 Pipeline Stage View API（M18c 只提供 build 級）
- **增量日誌**：利用 `X-More-Data` / `X-Text-Size` + `start=<offset>` 實現真正 progressive 推送
- **Artifact 大小**：目前 metadata 無 size；可在 M18c follow-up 加 HEAD 請求補完
- **Webhook 反向整合**：Jenkins → Synapse 的 build completion 通知（節省輪詢）
- **多分支 Pipeline**：目前 `job_path` 只指向單一 job；Multibranch Pipeline 需要 `job_path:branch` 的擴充格式

### 8.2 M18c-UI（前端）任務

- 「系統設定 → CI 引擎」頁面加上 Jenkins 選項
- Pipeline 建立 / 編輯時，`engine_type=jenkins` 顯示 `job_path` 欄位
- Run 詳情頁針對 Jenkins 顯示 queue id（runID 以 `queue:` 開頭時）

### 8.3 後續里程碑

| 里程碑  | 範圍                            | 預估 |
| ------- | ------------------------------- | ---- |
| M18b-UI | GitLab 連線設定前端             | 1 週 |
| M18c-UI | Jenkins 連線設定前端            | 1 週 |
| M18d    | Tekton Adapter                  | 2 週 |
| M18e    | Argo Workflows / GitHub Actions | 2 週 |

---

## 9. Lessons Learned（實作心得）

1. **CSRF Crumb 是「看起來小、行為隱密」的功能**
   若沒有在 client 層實作，每個 POST 方法都需自己記得加 crumb。把它隱藏在 `doMutation` 背後，讓 trigger.go / cancel.go 的程式碼讀起來乾淨很多。

2. **Queue 輪詢要設 soft timeout**
   一開始寫法是「阻塞到有 build number 或呼叫端 ctx 取消」。實測下來 Jenkins 在壓測下排 1 分鐘很常見，阻塞 1 分鐘對 Synapse API 是災難。改成 10 s soft timeout + `queue:<id>` RunID 後，前端體驗順暢多了。

3. **UNSTABLE 的映射是價值取向**
   沒有「中立」的技術選擇：要嘛把 UNSTABLE 當 success（使用者可能合入了壞 code），要嘛當 failed（誤擋一些可能合入的 MR）。M18c 選了後者，但把決策記錄在程式碼註解與本文件，方便未來重新檢視。

4. **sqlmock 配 httptest 做整合測試特別適合 Adapter**
   `ci_engine_service_jenkins_test.go` 一行驗證「sqlmock 餵 row + httptest 回 Jenkins 版本 → ListAvailableEngines 應該看到 jenkins.Available=true」——把三個層級（DB、Adapter、Service）一次 cover。

5. **複用 M18b 模式加速很多**
   `mapHTTPStatus` / `Register` / `MustRegister` / `parseExtra` / `buildXxxURL` 等 pattern 直接從 GitLab 複製過來改名字。M18d Tekton 應該又能走同樣路線。

---

## 10. 檢查清單（驗收用）

- [x] `go build ./...` 乾淨
- [x] `go vet ./...` 乾淨
- [x] 所有 106 個 M18c 新增測試通過（含 `-race`）
- [x] M18a + M18b 既有測試**未受影響**
- [x] 其他套件測試未受影響
- [x] `ADR-015` M18c 項目全標 ✅
- [x] `CI_ENGINE_ADAPTERS.md` §12 新增 Jenkins 使用指引
- [x] 本實作紀錄文件完成

---

**決策者：** Architecture Team
**實作者：** Architecture Team
**最後更新：** 2026-04-16
