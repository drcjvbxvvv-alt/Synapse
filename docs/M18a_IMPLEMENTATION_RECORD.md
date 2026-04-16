# M18a 實作紀錄 — CI 引擎 Adapter 框架

| 項目        | 內容                                                |
| ----------- | --------------------------------------------------- |
| 里程碑      | M18a — Adapter Framework                            |
| 狀態        | ✅ 完成（2026-04-16）                                |
| 對應 ADR    | [ADR-015](./adr/ADR-015-CI-Engine-Adapter-Pattern.md) |
| 使用指南    | [CI_ENGINE_ADAPTERS.md](./CI_ENGINE_ADAPTERS.md)    |
| 預估工時    | 2 週                                                |

---

## 1. 交付摘要

**一句話描述：** Synapse 新增 `CIEngineAdapter` 介面與 Native 實作，讓後續 M18b–M18e 可以直接實作 GitLab / Jenkins / Tekton / Argo / GitHub 等外部 CI 引擎而不動框架。

### 核心產出

| 層級     | 產出物                                                       |
| -------- | ------------------------------------------------------------ |
| **介面** | `CIEngineAdapter` 統一介面 + `EngineCapabilities` 能力宣告    |
| **錯誤** | 6 個 sentinel errors（`ErrNotFound` / `ErrUnauthorized` / …）|
| **工廠** | 並發安全的 `Factory` + `AdapterBuilder` 註冊機制              |
| **模型** | `CIEngineConfig`（加密憑證）+ `Pipeline.EngineType/ConfigID`  |
| **實作** | `NativeAdapter`（包裝既有 PipelineExecutor，透過 `NativeRunner` 介面解耦）|
| **服務** | `CIEngineService`（CRUD + 跨引擎探測）                        |
| **HTTP** | 6 個端點（`/api/v1/ci-engines/status` + CRUD）               |
| **路由** | 掛載 `PlatformAdminRequired` 中介層                           |

---

## 2. 階段實作流程

採「最小可行切片」分 6 階段交付，每階段都有獨立測試保證品質：

| 階段       | 內容                                               | 測試數 | 狀態 |
| ---------- | -------------------------------------------------- | ------ | ---- |
| Stage 1    | `CIEngineAdapter` 介面 + 類型 + sentinel errors    | 14     | ✅    |
| Stage 2    | `Pipeline` 擴充 + `CIEngineConfig` 模型             | 11     | ✅    |
| Stage 3    | `EngineFactory` 工廠 + 註冊機制                     | 14     | ✅    |
| Stage 4    | `NativeAdapter`（含 `NativeRunner` 介面）            | 17     | ✅    |
| Stage 5    | `CIEngineService` + HTTP Handler + 路由             | 20     | ✅    |
| Stage 6    | 文件更新（ADR / 使用指南 / 實作紀錄）                | —      | ✅    |

**總計：76 個單元測試，全部通過（含 `-race`）。**

---

## 3. 檔案清單

### 3.1 新增檔案（15 個）

#### 框架層

```
internal/services/pipeline/engine/
├── types.go                             # EngineType、Capabilities、TriggerRequest、RunStatus
├── adapter.go                           # CIEngineAdapter 介面
├── errors.go                            # Sentinel errors
├── factory.go                           # Factory + 註冊機制
├── native.go                            # NativeAdapter + NativeRunner 介面
├── types_test.go                        # 10 個測試
├── adapter_test.go                      # 4 個測試（含介面契約）
├── factory_test.go                      # 14 個測試（含並發 -race）
└── native_test.go                       # 17 個測試
```

#### 模型層

```
internal/models/
├── ci_engine_config.go                  # CIEngineConfig 模型 + Request DTO
├── ci_engine_config_test.go             # 8 個測試
├── ci_engine_config_helpers_test.go     # JSON helper（僅測試用）
└── pipeline_engine_test.go              # 3 個測試
```

#### 服務層

```
internal/services/
├── ci_engine_service.go                 # CRUD + ListAvailableEngines
└── ci_engine_service_test.go            # 11 個測試（sqlmock）
```

#### Handler 層

```
internal/handlers/
├── ci_engine_handler.go                 # HTTP 入口（5-step flow）
└── ci_engine_handler_test.go            # 9 個測試
```

#### 路由層

```
internal/router/
└── routes_ci_engine.go                  # 路由 + PlatformAdminRequired
```

#### 文件

```
docs/
├── adr/ADR-015-CI-Engine-Adapter-Pattern.md   # 架構決策（Accepted）
├── CI_ENGINE_ADAPTERS.md                      # 使用與開發指南
└── M18a_IMPLEMENTATION_RECORD.md              # 本文件
```

### 3.2 修改檔案（3 個）

| 檔案                                            | 修改內容                                                 |
| ----------------------------------------------- | -------------------------------------------------------- |
| `internal/models/pipeline.go`                   | `Pipeline` 新增 `EngineType`（預設 `"native"`）、`EngineConfigID` |
| `internal/router/router.go`                     | 掛載 `registerCIEngineRoutes(protected, &deps)`          |
| `docs/CICD_ARCHITECTURE.md`                     | §1 設計原則與 §19 技術選型 加入 Adapter Pattern 引用     |

---

## 4. 測試與品質驗證

### 4.1 測試統計

| 套件                                              | 測試數 |
| ------------------------------------------------- | ------ |
| `internal/services/pipeline/engine`（框架）        | 45     |
| `internal/models`（CIEngineConfig + Pipeline 擴充） | 11     |
| `internal/services`（CIEngineService）            | 11     |
| `internal/handlers`（CIEngineHandler）            | 9      |
| **總計**                                          | **76** |

### 4.2 驗證指令

```bash
# 建置
go build ./...                              # ✅ 乾淨

# 靜態檢查
go vet ./...                                # ✅ 乾淨

# 單元測試（含 race detector）
go test ./internal/services/pipeline/engine/... \
        ./internal/models/... \
        ./internal/services/... \
        ./internal/handlers/... \
    -count=1 -race -timeout=180s            # ✅ 全部通過
```

### 4.3 品質檢查清單

- [x] 遵循 `CLAUDE.md §2` Handler 5-step flow（parse → cluster/service → ctx → call → response）
- [x] 遵循 `CLAUDE.md §4` 錯誤處理（`fmt.Errorf(...%w...)` + `apierrors.AppError`）
- [x] 遵循 `CLAUDE.md §5` Context 使用（所有 handler 使用 `c.Request.Context()` + WithTimeout）
- [x] 遵循 `CLAUDE.md §6` GORM（所有查詢 `.WithContext(ctx)`）
- [x] 遵循 `CLAUDE.md §8` Observer Pattern（探測失敗不阻塞 UI）
- [x] 遵循 `CLAUDE.md §9` 日誌（所有狀態變更點寫 `logger.Info`，不含憑證）
- [x] 遵循 `CLAUDE.md §10` 安全規則（憑證 AES-256-GCM 加密、`json:"-"` 遮蔽）
- [x] 遵循 `CLAUDE.md §11` 路由註冊（`internal/router/routes_ci_engine.go`，不在 handler 內定義路由）
- [x] 並發安全（Factory 使用 `sync.RWMutex`，`-race` 測試通過）
- [x] 向後相容（既有 Pipeline 預設 `EngineType="native"`，行為不變）

---

## 5. 關鍵設計決策

### 5.1 為何引入 `NativeRunner` 介面？

**問題：** Native Adapter 需要呼叫既有 `pipeline_service`，但 `pipeline_service` 依賴眾多 internal 套件，直接 import 會造成循環依賴。

**解法：** 在 `engine` 套件內定義 `NativeRunner` 小介面（Trigger / GetRun / Cancel 3 個方法），由 `CIEngineService` 啟動時注入具體實作。

```go
// internal/services/pipeline/engine/native.go
type NativeRunner interface {
    Trigger(ctx context.Context, req *TriggerRequest) (*TriggerResult, error)
    GetRun(ctx context.Context, runID string) (*RunStatus, error)
    Cancel(ctx context.Context, runID string) error
}
```

**效果：** `engine` 套件不依賴任何 Synapse 內部邏輯，可被任何上層套件引用；測試時只需提供 `fakeRunner`。

### 5.2 為何 Factory 同時提供單例與建構函式？

- `engine.Default()` → 生產環境用的全域單例，便於 `init()` 註冊 Adapter
- `engine.NewFactory()` → 測試用的獨立實例，避免測試間互相污染

```go
// 生產
engine.RegisterNative(engine.Default(), runner, synapse.Version)

// 測試
f := engine.NewFactory()
engine.RegisterNative(f, fakeRunner{}, "v-test")
svc := services.NewCIEngineService(db, f)
```

### 5.3 為何 `engine_type` 建立後不可變更？

一條 Pipeline 可能有數十筆 `PipelineRun` 歷史紀錄。若 `CIEngineConfig.EngineType` 從 `gitlab` 改成 `jenkins`，歷史 run 仍標示 GitLab run ID，但新 run 會在 Jenkins 產生，造成 UI 無法連貫呈現。

**決策：** Update 時驗證 `req.EngineType == existing.EngineType`，差異時回 `CI_ENGINE_TYPE_IMMUTABLE`（400）；使用者應建立新 config。

### 5.4 為何憑證空字串不覆蓋現有值？

UI 的「編輯」流程：預載入時憑證欄位顯示為 `******`（或完全不回傳），使用者若未修改就按儲存，前端會送出空字串。若服務層把空字串寫回 DB，等於把憑證清空。

**解法：** `CIEngineConfigRequest.ApplyTo()` 明確判斷：空字串保留原值，非空字串才覆寫。

```go
if r.Token != "" {
    m.Token = r.Token
}
```

---

## 6. 安全設計重點

### 6.1 憑證加密

- 加密欄位：`Token` / `Password` / `WebhookSecret` / `CABundle`
- 演算法：AES-256-GCM（復用 `pkg/crypto`，與 `cluster.kubeconfig_enc` 同 KeyProvider）
- 加密時機：`BeforeSave` GORM hook
- 解密時機：`AfterFind` / `AfterCreate` / `AfterUpdate` hook
- 回退機制：`crypto.IsEnabled()` 為 false 時 hook no-op（便於測試）

### 6.2 JSON 遮蔽

所有敏感欄位使用 `json:"-"`：

```go
Token         string `json:"-" gorm:"type:text"`
Password      string `json:"-" gorm:"type:text"`
WebhookSecret string `json:"-" gorm:"type:text"`
CABundle      string `json:"-" gorm:"type:text"`
```

並在 `ci_engine_config_test.go` 加入回歸測試：

```go
func TestCIEngineConfig_SensitiveFieldsAreJSONHidden(t *testing.T) {
    cfg := &CIEngineConfig{Token: "SECRET", /* ... */}
    buf, _ := json.Marshal(cfg)
    if containsAny(string(buf), []string{"SECRET"}) {
        t.Fatalf("sensitive value leaked")
    }
}
```

### 6.3 權限隔離

| 端點                                     | 權限要求                | 理由                              |
| ---------------------------------------- | ----------------------- | --------------------------------- |
| `GET /api/v1/ci-engines/status`          | 已登入使用者            | 供 Pipeline 作者查詢可選引擎      |
| `GET /api/v1/ci-engines`                 | Platform Admin          | 管理清單含敏感 metadata            |
| `POST / PUT / DELETE /api/v1/ci-engines` | Platform Admin          | 憑證管理權責集中                  |

實作於 `internal/router/routes_ci_engine.go`：

```go
admin := protected.Group("/ci-engines")
admin.Use(middleware.PlatformAdminRequired(d.db))
```

---

## 7. 向後相容性

| 項目                                 | 保證                                                      |
| ------------------------------------ | --------------------------------------------------------- |
| 既有 `Pipeline` 資料                 | 預設 `EngineType="native"`，行為完全不變                   |
| 既有 `PipelineRun` / `StepRun` 資料  | 模型**完全未動**                                           |
| 既有 API 端點                        | 無任何變更                                                |
| 既有 Pipeline Scheduler / Executor   | 無任何變更                                                |
| DB Schema                            | 僅新增欄位 / 表，不刪除、不改名                           |

遷移指令（生產環境部署時執行）：

```sql
ALTER TABLE pipelines
    ADD COLUMN engine_type VARCHAR(20) NOT NULL DEFAULT 'native',
    ADD COLUMN engine_config_id BIGINT NULL;
CREATE INDEX idx_pipelines_engine_type ON pipelines(engine_type);
CREATE INDEX idx_pipelines_engine_config_id ON pipelines(engine_config_id);

CREATE TABLE ci_engine_configs (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    engine_type VARCHAR(20) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    endpoint VARCHAR(500),
    auth_type VARCHAR(20),
    username VARCHAR(100),
    token TEXT,
    password TEXT,
    webhook_secret TEXT,
    ca_bundle TEXT,
    cluster_id BIGINT,
    extra_json TEXT,
    insecure_skip_verify BOOLEAN DEFAULT FALSE,
    last_checked_at TIMESTAMP,
    last_healthy BOOLEAN DEFAULT FALSE,
    last_version VARCHAR(50),
    last_error TEXT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX idx_ci_engine_configs_engine_type ON ci_engine_configs(engine_type);
CREATE INDEX idx_ci_engine_configs_cluster_id ON ci_engine_configs(cluster_id);
CREATE INDEX idx_ci_engine_configs_deleted_at ON ci_engine_configs(deleted_at);
```

---

## 8. 遺留事項與後續工作

### 8.1 M18a 範圍內延後項目

- **Native `StreamLogs`**：目前回空 `io.ReadCloser`（既有 SSE 端點仍可用）；M18b 統一改走 Adapter
- **Native `GetArtifacts`**：目前回空 slice；M18b 接通 `PipelineArtifact` 儲存層

### 8.2 M18b 以後的主要任務

| 里程碑 | 工時 | 主要項目                                                     |
| ------ | ---- | ------------------------------------------------------------ |
| M18b   | 2 週 | GitLab CI Adapter（API v4 + webhook 雙向）+ 前端 UI           |
| M18c   | 2 週 | Jenkins Adapter（API + CSRF Crumb）+ 前端 UI                 |
| M18d   | 2 週 | Tekton Adapter（dynamic client + PipelineRun CRD）+ 前端引導 |
| M18e   | 2 週 | Argo Workflows / GitHub Actions                              |

### 8.3 開放問題（留待未來決策）

1. 跨引擎 Pipeline（Jenkins build → Native deploy）—— M18 不支援
2. Adapter Prometheus metrics（`synapse_ci_adapter_triggers_total` 等）—— M18b 一併加入
3. 多叢集 Tekton —— `CIEngineConfig.ClusterID` 指向叢集，Adapter 透過 dynamic client 建立資源

---

## 9. Lessons Learned（實作心得）

1. **「最小可行切片」比「一口氣寫完」好追蹤**
   分 6 階段交付，每階段都能獨立 `go test` 通過；任何階段有問題可立即 rollback。

2. **測試先寫可以鎖定契約**
   例：`TestNativeAdapter_Capabilities` 把 `EngineCapabilities` 內容固定為測試值，將來若改動能力會觸發測試失敗，強迫 review。

3. **`NativeRunner` 介面避免循環依賴**
   Go 的嚴格 import cycle 檢查在大型專案中是設計槓桿：當發現循環依賴，通常代表抽象層級不對。

4. **`engine.Default()` vs `engine.NewFactory()` 是測試友善設計**
   允許測試完全脫離全域狀態，避免 `go test -race` 時的測試交互污染。

5. **空字串保留現有值是 UX 陷阱**
   UI 的「編輯表單」幾乎一定會遇到「密碼欄該不該顯示、該不該回傳」的問題；服務層做好邊界處理比前端各自判斷更穩定。

---

## 10. 檢查清單（驗收用）

- [x] `go build ./...` 乾淨
- [x] `go vet ./...` 乾淨
- [x] 所有 76 個單元測試通過（含 `-race`）
- [x] 既有 `services` / `handlers` / `models` / `router` 測試未受影響
- [x] `ADR-015` 狀態改為 `Accepted`，M18a 項目全標 ✅
- [x] `CI_ENGINE_ADAPTERS.md` 使用指南完成
- [x] `CICD_ARCHITECTURE.md` 交叉引用更新
- [x] 本實作紀錄文件完成

---

**決策者：** Architecture Team
**實作者：** Architecture Team
**最後更新：** 2026-04-16
