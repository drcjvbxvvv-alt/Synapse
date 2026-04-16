# ADR-015：CI 引擎採 Adapter Pattern 支援可選外部依賴

| 項目       | 內容                                                                       |
| ---------- | -------------------------------------------------------------------------- |
| **狀態**   | Accepted — M18a–M18e ✅ 全部完成（2026-04-16）                             |
| **作者**   | Architecture Team                                                          |
| **相關 ADR** | ADR-001（K8s Job 執行引擎）、ADR-005（Trivy 雙軌）、ADR-006（PipelineSecret） |
| **相關文件** | [CICD_ARCHITECTURE.md](../CICD_ARCHITECTURE.md)、[CI_ENGINE_ADAPTERS.md](../CI_ENGINE_ADAPTERS.md)、[M18a](../M18a_IMPLEMENTATION_RECORD.md)、[M18b](../M18b_IMPLEMENTATION_RECORD.md)、[M18c](../M18c_IMPLEMENTATION_RECORD.md)、[M18d](../M18d_IMPLEMENTATION_RECORD.md)、[M18e](../M18e_IMPLEMENTATION_RECORD.md) |

---

## 1. Context（背景）

### 1.1 現況

[CICD_ARCHITECTURE.md](../CICD_ARCHITECTURE.md) 中 CI 執行引擎的三種設計：

| 層級         | 實作                               | 是否必要        |
| ------------ | ---------------------------------- | --------------- |
| **預設內建** | Synapse PipelineExecutor + K8s Job | ✅ 必要（ADR-001） |
| **過渡方案** | GitLab CI 推送掃描結果到 Synapse    | 🟡 可選（§5）    |
| **進階擴充** | Tekton / Jenkins（僅於原則提及）    | ⚪ 未設計       |

### 1.2 問題

使用者技術棧多樣，強制「只能用 Synapse 內建 Pipeline」會造成三類阻礙：

1. **既有資產無法復用**
   - 客戶已有 Jenkins Master + 數百條 Jenkinsfile
   - 客戶已有 GitLab Premium + 完整 `.gitlab-ci.yml` 流水線
   - 客戶已有 Tekton Controller + TaskBundle 生態
   - 要求「全部遷移到 Synapse」是不切實際的採用門檻

2. **技術選型鎖定（vendor lock-in）**
   - ADR-001 宣告「零額外元件」同時也排除了「既有投資」
   - Synapse 成為孤島：資料進得來，但執行必須走 Synapse 引擎

3. **內建引擎的定位不明**
   - 對已有成熟 CI 的客戶：內建引擎是多餘的
   - 對沒有 CI 的客戶：內建引擎是 fast path
   - 現有架構未區分這兩種使用情境

### 1.3 機會

`CLAUDE.md §8 「Observer Pattern for Optional Components」` 已為 Istio、KEDA、Cilium、Gateway API 等叢集擴充元件建立了可選依賴的典範。**CI 工具同樣應該套用此模式。**

---

## 2. Decision（決策）

### 2.1 核心決策

**將 CI 執行引擎抽象為 `CIEngineAdapter` 介面，Synapse 同時支援：**

| 引擎              | 類型       | 里程碑   | 用途                                      |
| ----------------- | ---------- | -------- | ----------------------------------------- |
| **Native**（內建）| `native`   | M18a ✅  | 基於 K8s Job 的 PipelineExecutor（預設） |
| GitLab CI         | `gitlab`   | M18b ✅  | 觸發 GitLab Pipeline、接收結果            |
| Jenkins           | `jenkins`  | M18c ✅  | 觸發 Jenkins Job、接收結果                |
| Tekton            | `tekton`   | M18d ✅  | 建立 PipelineRun CRD、讀取結果            |
| Argo Workflows    | `argo`     | M18e ✅  | 建立 Workflow CRD、讀取結果               |
| GitHub Actions    | `github`   | M18e ✅  | 觸發 workflow_dispatch、接收結果          |

使用者可在**每條 Pipeline 層級獨立選擇**要用哪個引擎，不是全系統一刀切。

### 2.2 Adapter 介面設計

```go
// internal/services/pipeline/engine/adapter.go

// CIEngineAdapter 是所有 CI 引擎的統一介面。
type CIEngineAdapter interface {
    // ── 能力探測（Observer Pattern） ─────────────────────────────
    Type() EngineType
    IsAvailable(ctx context.Context) bool        // 絕不回傳錯誤（UI 不阻塞）
    Version(ctx context.Context) (string, error)
    Capabilities() EngineCapabilities            // 冪等且無 I/O

    // ── 執行控制 ──────────────────────────────────────────────
    Trigger(ctx context.Context, req *TriggerRequest) (*TriggerResult, error)
    GetRun(ctx context.Context, runID string) (*RunStatus, error)
    Cancel(ctx context.Context, runID string) error

    // ── 日誌與產物 ─────────────────────────────────────────────
    StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error)
    GetArtifacts(ctx context.Context, runID string) ([]*Artifact, error)
}

// EngineCapabilities 描述引擎支援的功能集。
type EngineCapabilities struct {
    SupportsDAG          bool // 複雜 DAG 拓撲
    SupportsMatrix       bool // 矩陣建置
    SupportsArtifacts    bool // Artifact 管理
    SupportsSecrets      bool // Secret 注入
    SupportsCaching      bool // 快取機制
    SupportsApprovals    bool // 人工審核閘門
    SupportsNotification bool // 內建通知
    SupportsLiveLog      bool // 即時日誌串流
}
```

### 2.3 資料模型擴充

```go
// internal/models/pipeline.go（擴充欄位）

type Pipeline struct {
    // ... 既有欄位 ...

    // CI 引擎選擇：預設 "native"，對應內建 K8s Job 引擎。
    // 其他值（gitlab / jenkins / tekton / argo / github）會委派給 Adapter。
    EngineType     string `gorm:"size:20;not null;default:'native';index"`
    EngineConfigID *uint  `gorm:"index"` // 外部引擎必填；native 為 nil
}

// internal/models/ci_engine_config.go（新增）

type CIEngineConfig struct {
    ID         uint   `gorm:"primaryKey"`
    Name       string `gorm:"size:100;not null;uniqueIndex"`
    EngineType string `gorm:"size:20;not null;index"`
    Enabled    bool   `gorm:"not null;default:true"`

    // 連線資訊
    Endpoint string `gorm:"size:500"`

    // 認證（均使用 pkg/crypto AES-256-GCM 加密儲存）
    AuthType      string
    Username      string
    Token         string `json:"-" gorm:"type:text"`
    Password      string `json:"-" gorm:"type:text"`
    WebhookSecret string `json:"-" gorm:"type:text"`
    CABundle      string `json:"-" gorm:"type:text"`

    ClusterID *uint  `gorm:"index"` // Tekton / Argo：指向 Synapse 管理的叢集
    ExtraJSON string `gorm:"type:text"`

    InsecureSkipVerify bool

    // 健康追蹤
    LastCheckedAt *time.Time
    LastHealthy   bool
    LastVersion   string `gorm:"size:50"`
    LastError     string `gorm:"type:text"`

    CreatedBy uint
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt
}
```

### 2.4 Observer Pattern 實踐

遵循 `CLAUDE.md §8`，每個 Adapter 都須「優雅降級」：

- **絕不因為單一引擎故障而回 5xx**，讓使用者頁面可完整載入
- 探測設置 **短超時（5 秒）** 避免慢引擎拖累整頁
- 單一 Adapter 建構失敗 → 僅該引擎標示離線，其他引擎正常顯示

```go
// 示意：探測引擎狀態時一律避免阻塞 UI
func (s *CIEngineService) ListAvailableEngines(ctx context.Context) ([]*EngineStatus, error) {
    // native 永遠可用（in-process 實作）
    out := []*EngineStatus{buildNativeStatus(ctx, s.factory)}

    // 每個外部引擎以獨立短超時探測，失敗僅寫入 Error 欄位
    for _, cfg := range loadConfigs(ctx) {
        out = append(out, probeExternalEngine(ctx, s.factory, cfg))
    }
    return out, nil
}
```

---

## 3. Alternatives Considered（替代方案）

| 方案                                       | 否決原因                                                          |
| ------------------------------------------ | ----------------------------------------------------------------- |
| A. 只提供 Native Engine，不支援外部 CI     | 客戶既有 Jenkins/GitLab 投資無法復用；採用門檻高                   |
| B. 廢掉 Native Engine，僅保留 Adapter      | 違反 ADR-001「零額外元件」原則；小客戶失去 fast path             |
| C. 每個引擎各做獨立 Handler/Service，不抽象 | 程式碼重複；新增引擎要改一堆地方；違反開閉原則                    |
| D. 用 Pipeline as Code（YAML）取代多引擎   | YAML 能描述 Pipeline，但無法「觸發別人的 Jenkins」；用途不同      |
| **E. 採用本 ADR 的 Adapter Pattern**       | **✅ 被選中**                                                     |

---

## 4. Consequences（影響）

### ✅ 正面影響

1. **降低採用門檻**
   - 既有 Jenkins/GitLab 客戶可「先接入 Synapse 戰情室」，不需立即遷移
   - Synapse 的價值主張從「取代 CI」變成「統一戰情室 + 可選 CI」

2. **強化「可插拔」原則**
   - 符合 `CLAUDE.md §8` Observer Pattern
   - 與 Istio / KEDA / Cilium 的處理方式一致，降低認知負擔

3. **保留 Native Engine 的價值**
   - 小團隊 / 新專案：直接用 Native，零依賴
   - 大企業 / 既有系統：用 Adapter 接既有 CI

4. **擴充性**
   - 未來新增 CircleCI / Drone / Bamboo 只需新增 Adapter 實作
   - 不影響核心 `Pipeline` / `PipelineRun` / `StepRun` 資料模型

### ❌ 負面影響與緩解

| 問題                                 | 緩解策略                                                                               |
| ------------------------------------ | -------------------------------------------------------------------------------------- |
| Adapter 介面難以涵蓋所有引擎特性      | 透過 `Capabilities()` 宣告能力；UI 隱藏不支援的選項                                    |
| 外部 CI 日誌延遲比 Native 高          | `StreamLogs` 以「非同步輪詢 + 增量推送」設計；UI 標示來源                              |
| 外部 CI 的 Step 狀態粒度不一致        | 抽象為統一狀態機（pending / running / success / failed / cancelled）；原始狀態放 `Raw` |
| 認證憑證管理複雜                     | 復用 `pkg/crypto` + `PipelineSecret` 機制（ADR-006）                                   |
| 維護多個 Adapter 的測試矩陣           | 每個 Adapter 提供 mock server；CI 跑 contract test                                     |

### 🔓 Escape Hatch（逃生門）

- 某個 Adapter 維護成本過高 → 標記 `deprecated`，保留既有 PipelineRun 紀錄，停止新建
- Native Engine 永遠是降級備援，不因 Adapter 故障而影響基本功能

---

## 5. 實作里程碑

### M18a — Adapter Framework（2 週）✅ **完成**

> 交付摘要：**76 個單元測試全通過（含 `-race`），`go build ./...` / `go vet ./...` 乾淨。**
> 詳細紀錄見 [M18a_IMPLEMENTATION_RECORD.md](../M18a_IMPLEMENTATION_RECORD.md)。

| 項目 | 交付物 | 測試 |
| ---- | ------ | ---- |
| ✅ `CIEngineAdapter` 介面與 `EngineCapabilities` | `internal/services/pipeline/engine/{adapter,types,errors}.go` | 14 |
| ✅ `EngineFactory` 與註冊機制 | `internal/services/pipeline/engine/factory.go` | 14 |
| ✅ 擴充 `Pipeline`、新增 `CIEngineConfig` 模型 | `internal/models/{pipeline,ci_engine_config}.go` | 11 |
| ✅ Native Engine Adapter（含 `NativeRunner` 介面） | `internal/services/pipeline/engine/native.go` | 17 |
| ✅ CIEngineService + HTTP Handler + 路由 | `internal/{services,handlers,router}/ci_engine*.go` | 20 |

### M18b — GitLab CI Adapter（2 週）✅ **完成**

> 交付摘要：**后端 76 個單元測試全通過（含 `-race`）+ 1 個端到端整合測試，`go build ./...` / `go vet ./...` 乾淨。**
> 詳細紀錄見 [M18b_IMPLEMENTATION_RECORD.md](../M18b_IMPLEMENTATION_RECORD.md)。

| 項目 | 交付物 | 測試 |
| ---- | ------ | ---- |
| ✅ GitLab API v4 HTTP 客戶端（PRIVATE-TOKEN 認證、TLS、錯誤映射） | `internal/services/pipeline/engine/gitlab/{client,errors,types}.go` | 18 |
| ✅ Adapter 骨架（Type、IsAvailable、Version、Capabilities） | `internal/services/pipeline/engine/gitlab/{adapter,config}.go` | 11 |
| ✅ Trigger + GetRun（含 GitLab status → RunPhase 映射） | `internal/services/pipeline/engine/gitlab/{trigger,runs,status}.go` | 18 |
| ✅ Cancel + StreamLogs + GetArtifacts | `internal/services/pipeline/engine/gitlab/{cancel,logs,artifacts}.go` | 19 |
| ✅ Factory 註冊（`Register` / `MustRegister`） + 路由啟動時自動註冊 | `internal/services/pipeline/engine/gitlab/register.go`、`internal/router/routes_ci_engine.go` | 5 + 1（整合） |
| 🟡 前端 GitLab 連線設定 UI | （延後至 M18b-UI 小節，由前端團隊認領） | — |

### M18c — Jenkins Adapter（2 週）✅ **完成**

> 交付摘要：**後端 106 個單元測試全通過（含 `-race`）+ 1 個端到端整合測試，`go build ./...` / `go vet ./...` 乾淨。**
> 詳細紀錄見 [M18c_IMPLEMENTATION_RECORD.md](../M18c_IMPLEMENTATION_RECORD.md)。

| 項目 | 交付物 | 測試 |
| ---- | ------ | ---- |
| ✅ Jenkins API 客戶端（Basic Auth、TLS、CSRF Crumb 快取 + 自動重試） | `internal/services/pipeline/engine/jenkins/{client,crumb,errors,types}.go` | 21 |
| ✅ Adapter 骨架（含 `X-Jenkins` header 版本讀取） | `internal/services/pipeline/engine/jenkins/{adapter,config}.go` | 20 |
| ✅ Trigger（POST `/job/:path/buildWithParameters` + Queue 輪詢） | `internal/services/pipeline/engine/jenkins/{trigger,status,runs}.go` | 22 |
| ✅ Cancel + StreamLogs（`progressiveText`） + GetArtifacts | `internal/services/pipeline/engine/jenkins/{cancel,logs,artifacts}.go` | 22 |
| ✅ Factory 註冊 + 啟動時整合 | `internal/services/pipeline/engine/jenkins/register.go`、`internal/router/routes_ci_engine.go` | 5 + 1（整合） |
| 🟡 前端 Jenkins 連線設定 UI | （延後至 M18c-UI） | — |

### M18d — Tekton Adapter（2 週）✅ **完成**

> 交付摘要：**後端 78 個單元測試全通過（含 `-race`）+ 1 個端到端整合測試，`go build ./...` / `go vet ./...` 乾淨。**
> 詳細紀錄見 [M18d_IMPLEMENTATION_RECORD.md](../M18d_IMPLEMENTATION_RECORD.md)。

| 項目 | 交付物 | 測試 |
| ---- | ------ | ---- |
| ✅ CRD 類型 + `ClusterResolver` 介面（解耦 internal/k8s）| `internal/services/pipeline/engine/tekton/{types,cluster,errors,config}.go` | 20 |
| ✅ Adapter 骨架 + Discovery 偵測 tekton.dev/v1 | `internal/services/pipeline/engine/tekton/adapter.go` | 13 |
| ✅ Trigger（建立 PipelineRun unstructured，自生成 name）+ GetRun（合併 TaskRuns） | `internal/services/pipeline/engine/tekton/{trigger,runs,status}.go` | 22 |
| ✅ Cancel（PATCH spec.status=Cancelled）+ GetArtifacts（pipelineResults） | `internal/services/pipeline/engine/tekton/{cancel,artifacts}.go` | 16 |
| 🟡 StreamLogs 以 `ErrUnsupported` 回應（Pod log streaming 需 typed clientset，延後至 M18d follow-up） | `internal/services/pipeline/engine/tekton/logs.go` | 2 |
| ✅ Factory 註冊（帶 ClusterResolver）+ router 整合（`tektonClusterResolver` 包裝 k8sMgr） | `internal/services/pipeline/engine/tekton/register.go`、`internal/router/tekton_cluster_resolver.go` | 6 + 1（整合） |
| 🟡 前端 Tekton 連線設定 UI | （延後至 M18d-UI） | — |

### M18e — Argo Workflows + GitHub Actions（2 週）✅ **完成**

> 交付摘要：**Argo 86 個 + GitHub 57 個單元測試 + 2 個端到端整合測試，全部通過（含 `-race`）。**
> 詳細紀錄見 [M18e_IMPLEMENTATION_RECORD.md](../M18e_IMPLEMENTATION_RECORD.md)。

| 項目 | 交付物 | 測試 |
| ---- | ------ | ---- |
| ✅ Argo Workflows Adapter（dynamic client 建立 Workflow CRD、status.nodes → Steps） | `internal/services/pipeline/engine/argo/*.go` | 86 |
| ✅ GitHub Actions Adapter（Bearer auth、workflow_dispatch、API 2022-11-28） | `internal/services/pipeline/engine/github/*.go` | 57 |
| ✅ `tektonClusterResolver` 重命名為共用 `k8sClusterResolver`（Go structural typing 同時滿足兩個介面） | `internal/router/k8s_cluster_resolver.go` | — |
| ✅ Factory 註冊 + 啟動整合 | `internal/router/routes_ci_engine.go`（+ argo/github 註冊） | 2（整合） |
| 🟡 Argo StreamLogs | 延後至 M18e follow-up（同 Tekton：需 typed `kubernetes.Interface`） | — |
| ✅ GitHub StreamLogs | 實作 `/actions/jobs/:job_id/logs` | — |
| 🟡 前端 Argo / GitHub 連線設定 UI | （延後至 M18e-UI） | — |

**總工時估計：** 約 10 週已完成（M18a–M18e），可並行開發

---

## 6. 設定範例

### 6.1 使用者視角：為 Pipeline 選擇引擎

```yaml
# pipeline.yaml
name: build-saas-java-a
engine: gitlab                     # native / gitlab / jenkins / tekton / argo / github
engine_config: gitlab-main         # 對應 CIEngineConfig.Name

trigger:
  type: webhook
  events: [push]

# engine: native 時使用 Synapse 原生 steps
# engine: gitlab 時對應到 GitLab 專案的 .gitlab-ci.yml
gitlab:
  project_id: 123
  ref: main
  variables:
    IMAGE_TAG: ${{ gitlab.sha }}
```

### 6.2 UI 互動流程

```
建立 Pipeline
    ↓
選擇引擎（下拉選單）
    ├─ Native（K8s Job）  ← 預設、無依賴
    ├─ GitLab CI          ← 需先新增 CI Engine 設定
    ├─ Jenkins            ← 需先新增 CI Engine 設定
    ├─ Tekton（已偵測）   ← 叢集內已安裝 CRD
    └─ Argo Workflows（未偵測）  ← 引導安裝
    ↓
若選擇外部引擎但未設定 → 引導到「系統設定 → CI 引擎」頁面
    ↓
填寫 Pipeline 細節（僅顯示該引擎支援的欄位；依 Capabilities 動態渲染）
```

---

## 7. 對現有架構的影響

### 7.1 檔案變更清單

| 檔案                                                | 變更 | 說明                                              |
| --------------------------------------------------- | ---- | ------------------------------------------------- |
| `internal/models/pipeline.go`                       | 擴充 | 新增 `EngineType`、`EngineConfigID` 欄位           |
| `internal/models/ci_engine_config.go`               | 新增 | `CIEngineConfig` 模型 + 加密 hooks                 |
| `internal/services/pipeline/engine/`                | 新增 | Adapter 介面、Factory、Native 實作                 |
| `internal/services/ci_engine_service.go`            | 新增 | CRUD + 探測                                       |
| `internal/handlers/ci_engine_handler.go`            | 新增 | HTTP 入口（5-step flow）                          |
| `internal/router/routes_ci_engine.go`               | 新增 | 路由 + PlatformAdminRequired 中介層               |
| `internal/router/router.go`                         | 修改 | 掛載 `registerCIEngineRoutes`                      |
| `ui/src/pages/ciEngines/`                           | 待辦 | CI 引擎管理頁面（M18b 伴隨 GitLab Adapter 交付）  |

### 7.2 向後相容

- 既有 Pipeline 預設 `EngineType = "native"`，行為不變
- 不強制遷移；使用者自行選擇何時切換
- `PipelineRun` / `StepRun` / `PipelineLog` 執行紀錄模型**完全不動**

---

## 8. 開放問題

1. **跨引擎 Pipeline？**
   使用者是否需要「先用 Jenkins Build → 再用 Native Deploy」的混合流？
   **暫定決策：** M18 不支援，保留未來擴充空間。

2. **Adapter 觀測指標（Metrics）？**
   每個 Adapter 匯出 Prometheus metrics：`synapse_ci_adapter_triggers_total{engine, status}`，便於判斷使用分佈與失敗率。

3. **多叢集 Tekton？**
   Tekton 安裝在 Synapse 管理的哪個叢集？
   **暫定決策：** Tekton Adapter 的 `CIEngineConfig.ClusterID` 指向叢集，透過該叢集的 dynamic client 建立資源。

---

## 9. 追蹤

- **里程碑：** M18a ✅ 完成；M18b–M18e 待排程
- **相關程式碼：** `internal/services/pipeline/engine/`
- **相關測試：** `internal/services/pipeline/engine/*_test.go`（每個 Adapter 需提供 mock server）

---

**決策者：** Architecture Team
**最後更新：** 2026-04-16
