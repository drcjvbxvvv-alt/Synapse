# M18d 實作紀錄 — Tekton Adapter

| 項目        | 內容                                                |
| ----------- | --------------------------------------------------- |
| 里程碑      | M18d — Tekton Adapter（後端）                        |
| 狀態        | ✅ 完成（2026-04-16）                                |
| 對應 ADR    | [ADR-015](./adr/ADR-015-CI-Engine-Adapter-Pattern.md) |
| 使用指南    | [CI_ENGINE_ADAPTERS.md §13](./CI_ENGINE_ADAPTERS.md) |
| 前置里程碑  | [M18a](./M18a_IMPLEMENTATION_RECORD.md) / [M18b](./M18b_IMPLEMENTATION_RECORD.md) / [M18c](./M18c_IMPLEMENTATION_RECORD.md) |

---

## 1. 交付摘要

**一句話描述：** Synapse 新增 Tekton Adapter，透過 Kubernetes Dynamic Client 直接操作 `PipelineRun` / `TaskRun` CRD，完成建立、查詢、取消、artifact 抽取——無需 HTTP REST，無需額外認證機制。

### 核心產出

| 層級                 | 產出物                                                                     |
| -------------------- | -------------------------------------------------------------------------- |
| **GVR 常數 / DTO**   | `types.go`（PipelineRun / TaskRun GVR、Succeeded condition 常數、labels）   |
| **抽象介面**         | `cluster.go`（`ClusterResolver`，解耦 `internal/k8s` 的循環依賴）            |
| **錯誤映射**         | `errors.go`（`k8serrors.Is*` → engine sentinel）                            |
| **Config 解析**      | `config.go`（`ExtraConfig.{PipelineName,Namespace,ServiceAccountName}`）    |
| **Adapter**          | `adapter.go`（Type/IsAvailable via Discovery/Version/Capabilities）         |
| **狀態映射**         | `status.go`（Succeeded Condition → 6 種 RunPhase）                          |
| **Trigger**          | `trigger.go`（`CREATE` PipelineRun；Adapter-side name generation）          |
| **GetRun**           | `runs.go`（PipelineRun + labelSelector 查 TaskRuns 組成 Steps）             |
| **Cancel**           | `cancel.go`（`PATCH spec.status="Cancelled"`，已結束回 ErrAlreadyTerminal） |
| **StreamLogs**       | `logs.go`（**M18d 未實作**，ErrUnsupported；原因見 §5.3）                   |
| **GetArtifacts**     | `artifacts.go`（抽取 `status.pipelineResults[]`）                           |
| **Factory 註冊**      | `register.go` + `routes_ci_engine.go` 整合 + router/tekton_cluster_resolver |

---

## 2. 階段實作流程

| 階段       | 內容                                                               | 測試數 | 狀態 |
| ---------- | ------------------------------------------------------------------ | ------ | ---- |
| Stage 1    | CRD 類型 + ClusterResolver 介面 + 錯誤映射 + ExtraConfig             | 20     | ✅    |
| Stage 2    | Adapter 骨架 + Discovery-based IsAvailable / Version                | 13     | ✅    |
| Stage 3    | Trigger（PipelineRun 建立）+ GetRun（含 TaskRun 合併）+ 狀態映射     | 22     | ✅    |
| Stage 4    | Cancel（PATCH spec.status）+ StreamLogs（ErrUnsupported）+ GetArtifacts | 18     | ✅    |
| Stage 5    | Factory 註冊（需要 resolver）+ router 包裝 + 整合測試                 | 6 + 1  | ✅    |
| Stage 6    | 文件更新                                                            | —      | ✅    |

**合計：78 個 tekton 套件內測試 + 1 個跨套件整合測試，全部通過（含 `-race`）。**

---

## 3. 檔案清單

### 3.1 新增檔案（16 個）

```
internal/services/pipeline/engine/tekton/
├── types.go                 # GVR + labels + reason 常數
├── cluster.go               # ClusterResolver 介面
├── errors.go                # k8serrors → engine sentinel 映射
├── config.go                # ExtraConfig + requireTargets
├── adapter.go               # Type / IsAvailable / Version / Capabilities
├── status.go                # mapTektonStatus(succeededCondition)
├── trigger.go               # Create + name generation
├── runs.go                  # GetRun + TaskRun 合併為 StepStatus
├── cancel.go                # PATCH spec.status="Cancelled"
├── logs.go                  # StreamLogs → ErrUnsupported
├── artifacts.go             # pipelineResults → engine.Artifact
├── register.go              # Register / MustRegister（需 resolver）
├── config_test.go           # 5 個測試
├── errors_test.go           # 15 個測試
├── adapter_test.go          # 13 個測試
├── trigger_test.go          # 9 個測試
├── runs_test.go             # 13 個測試
├── cancel_test.go           # 7 個測試
├── logs_test.go             # 2 個測試
├── artifacts_test.go        # 9 個測試
└── register_test.go         # 6 個測試

internal/router/
└── tekton_cluster_resolver.go   # ClusterResolver 實作（router 層）

internal/services/
└── ci_engine_service_tekton_test.go   # 1 個端到端整合測試

docs/
└── M18d_IMPLEMENTATION_RECORD.md      # 本檔
```

### 3.2 修改檔案（3 個）

| 檔案                                                  | 修改內容                                                      |
| ----------------------------------------------------- | ------------------------------------------------------------- |
| `internal/router/routes_ci_engine.go`                 | 啟動時追加 `tekton.Register(engine.Default(), newTektonClusterResolver(d.k8sMgr))` |
| `docs/adr/ADR-015-CI-Engine-Adapter-Pattern.md`       | M18d 項目全標 ✅                                              |
| `docs/CI_ENGINE_ADAPTERS.md`                          | §13 新增 Tekton Adapter 使用指引                              |

---

## 4. 測試與品質驗證

### 4.1 測試統計

| 套件                                        | 測試數 |
| ------------------------------------------- | ------ |
| `internal/services/pipeline/engine/tekton`   | 78     |
| `internal/services`（M18d 整合測試）         | 1      |
| **M18d 新增合計**                            | **79** |

CI 引擎子系統累計測試（M18a + M18b + M18c + M18d）：**76 + 73 + 106 + 79 = 334 個**。

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

- [x] 遵循 `CLAUDE.md §4` 錯誤處理（`fmt.Errorf(...%w...)` 包裝 sentinel，保留 k8s 原始錯誤）
- [x] 遵循 `CLAUDE.md §5` Context 使用（所有方法第一參數 ctx，傳入 dynamic client）
- [x] 遵循 `CLAUDE.md §7` K8s Client 使用（dynamic client for CRDs，對應 §7 的 Path B）
- [x] 遵循 `CLAUDE.md §8` Observer Pattern（`IsAvailable()` 永不回錯，TaskRun list 失敗仍回 Pipeline 狀態）
- [x] 遵循 `CLAUDE.md §10` 安全規則（kubeconfig 由 router 層集中管理，Adapter 不接觸明文）
- [x] 避免循環依賴（`ClusterResolver` 介面 + `internal/router/tekton_cluster_resolver.go` 實作）
- [x] `-race` 測試通過

---

## 5. 關鍵設計決策

### 5.1 為何用 `ClusterResolver` 介面而非直接 import `internal/k8s`？

嘗試過的替代方案：

- **直接 import `internal/k8s`** — 循環依賴：`internal/k8s` 依賴 `internal/services`，而 `services/pipeline/engine/tekton` 也在 `internal/services` 之下（雖不直接，但 router 最終要能同時 import 兩者）。
- **把 ClusterInformerManager 拿出來放到 pkg/** — 會牽動太多下游（`services/argocd_service.go`、`handlers/*` 等）。

最終方案：**介面 + 依賴注入**。`ClusterResolver` 只要 2 個方法（`Dynamic` / `Discovery`），單元測試用 `client-go/dynamic/fake` + `client-go/discovery/fake` 直接塞進去，production 則用 `internal/router/tekton_cluster_resolver.go` 包裝 `ClusterInformerManager`。

附帶好處：未來 M18e 若要加 Argo Workflows Adapter，可 reuse 相同的 `ClusterResolver`。

### 5.2 為何 Adapter 端自生成 PipelineRun name？

Tekton 支援 server-side `metadata.generateName`（讓 K8s 填 random suffix）。但：

- **`dynamicfake` v0.29 不處理 generateName**（實測，見 trigger_test.go 旁註），導致 fake Create 後 `GetName()` 為空
- 真實 K8s 會填 name，但客戶端拿不到 — 需再 Get 一次才知道 name，整個 Trigger 多一次 round trip
- Synapse 的 PipelineRun 沒有特殊排序需求，亂數 suffix 就夠

最終採用：`generateRunName()` 用 `crypto/rand` 生成 5-byte hex suffix，產生 `synapse-run-<10 hex chars>` 格式的 DNS-1123 label。這個 name 直接作為 `metadata.name`，Create 成功即拿到 RunID，無延遲。

### 5.3 為何 `StreamLogs` 回傳 `ErrUnsupported` 而非實作？

Tekton 沒有 HTTP log 端點；log 來自 TaskRun 對應的 Pod 容器。要 streaming 需：

1. `List TaskRuns` → 取 `status.podName`
2. `CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)`

第二步要 typed `kubernetes.Interface`，而 M18d 的 `ClusterResolver` 只暴露 `dynamic.Interface` + `discovery.DiscoveryInterface`。把 `Kubernetes()` 加進介面需要：

1. 擴充 `ClusterResolver` 介面
2. 擴充 `tektonClusterResolver` 實作
3. 為 `stepID` 定義語義（TaskRun 名稱？Step container 名稱？）
4. 處理多容器 Pod 的 log 串接
5. 處理 Pod 尚未產生時的 degenerate case

為了保持 M18d 的「框架對齊」scope 乾淨，我把 StreamLogs 明確標記為 `ErrUnsupported`。前端接到 HTTP 501 時可引導使用者到 Tekton Dashboard，或等 M18d-Follow-up 補上。

### 5.4 為何不區分 `UNSTABLE` 之類的細粒度狀態？

Tekton 的 `Succeeded` condition 只有 True/False/Unknown 三態。具體原因碼（Reason）才能區分「成功」「失敗」「取消」「尚在執行」。

M18d 把 Reason 保留在 `RunStatus.Raw` 供前端顯示，但 Phase 層級只做 5 種（success / failed / cancelled / running / pending）+ 1 種（unknown）的映射。這和 GitLab / Jenkins adapter 一致。

### 5.5 為何用 `managed-by` label 而非 annotation？

Annotation 不能當 labelSelector 的查詢條件。如果想快速列出「Synapse 建立的 PipelineRun」，必須走 label。

三個 label：

- `app.kubernetes.io/managed-by = synapse-ci-adapter` — 標準 K8s recommended label
- `synapse.io/run-id = <SnapshotID>` — 回查 Synapse 執行紀錄
- `synapse.io/pipeline-id = <PipelineID>` — Pipeline 級別彙整

Synapse 自訂 label 使用 `synapse.io/` 前綴，避免和 Tekton 原生 label（`tekton.dev/*`）混淆。

### 5.6 為何 PipelineRun 的 `pipelineResults` 作為 Artifact 而非另設 "result" 類型？

M18a 的 `engine.Artifact` 是抽象介面，設計上涵蓋任何「run 產物」。強行增設一個 `Result` 類型會：

- 讓 frontend / handler 多處理一種類型
- 與 GitLab / Jenkins 的 artifact-as-file 模型脫節

折衷：Tekton artifact 使用 `Kind="result"` 區分（frontend 可據此選 icon），value 放在 `Digest` 欄位（重複利用現有 slot）。Workspace 檔案型 artifacts 是 M18d follow-up 的延伸。

---

## 6. 安全設計重點

### 6.1 認證鏈

Tekton Adapter 完全**不持有**認證資料。流程：

```
CIEngineConfig.ClusterID
    ↓ ClusterResolver.Dynamic(clusterID)
ClusterInformerManager.GetK8sClientByID(clusterID)
    ↓ K8sClient.GetRestConfig()
rest.Config（含 cluster 的 kubeconfig）
    ↓ dynamic.NewForConfig(cfg)
dynamic.Interface
```

- `CIEngineConfig.Token` / `Password` / `WebhookSecret` 對 Tekton **完全不使用**
- K8sClient 本身的憑證由 `Cluster.KubeconfigEnc` 存放（ADR-001 / ADR-006）
- Adapter 一次 session 結束後，`dynamic.Interface` 丟棄；沒有 cookie / token 快取

### 6.2 RBAC 最小權限

Tekton Adapter 需要的 RBAC（on cluster 中由 kubeconfig 的 SA 持有）：

- `pipelineruns.tekton.dev` — create / get / list / patch
- `taskruns.tekton.dev` — list（labelSelector）
- `GET /api/v1/groupversion/tekton.dev/v1` — Discovery 權限

建議把 Synapse 管理叢集的 kubeconfig 綁到一個專用 ServiceAccount，透過 ClusterRole 精確授予上述動詞。

### 6.3 Namespace 隔離

Adapter 的所有 CRD 操作均帶 `namespace`（從 `ExtraConfig.Namespace` 取得）。不使用 `metav1.NamespaceAll`，避免越權看到其他 namespace 的 run。

---

## 7. 向後相容性

| 項目                                 | 保證                                                              |
| ------------------------------------ | ----------------------------------------------------------------- |
| `engine` 套件                        | 不變                                                              |
| Native / GitLab / Jenkins Adapter    | 不受影響                                                          |
| `CIEngineService` / `CIEngineHandler`| 不需變更（新的 `ClusterResolver` 由 router 注入）                  |
| 既有 API 端點                        | 無變更                                                            |
| DB Schema                            | 無變更（Tekton 用 `cluster_id` + `extra_json` 欄位，都已存在）    |

---

## 8. 遺留事項與後續工作

### 8.1 M18d 範圍內延後項目

- **StreamLogs**：擴充 `ClusterResolver` 加 `Kubernetes()`；引入 stepID 語意（建議對應 TaskRun name）
- **Workspace artifact**：從 PVC / ConfigMap 抽取檔案型 artifacts
- **v1beta1 支援**：若客戶 Tekton 停留在 1.4 以前版本
- **觀測 metric**：`synapse_tekton_pipelineruns_created_total{cluster, pipeline}` 等 Prometheus 指標

### 8.2 M18d-UI（前端）任務

- 「系統設定 → CI 引擎」新增 Tekton 類型：下拉選叢集、填 `pipeline_name`、`namespace`、`service_account_name`
- Run 詳情頁針對 Tekton 顯示 TaskRun 清單（每個 Step 對應一個 TaskRun）
- 偵測 tekton.dev CRD：Synapse 可在叢集頁面顯示「此叢集支援 Tekton」徽章

### 8.3 後續里程碑

| 里程碑         | 範圍                            | 預估 |
| -------------- | ------------------------------- | ---- |
| M18b-UI        | GitLab 連線設定前端             | 1 週 |
| M18c-UI        | Jenkins 連線設定前端            | 1 週 |
| M18d-UI        | Tekton 連線設定前端             | 1 週 |
| M18d-Follow-up | Tekton StreamLogs（Pod log）    | 1 週 |
| M18e           | Argo Workflows / GitHub Actions | 2 週 |

---

## 9. Lessons Learned（實作心得）

1. **K8s-native adapter ≠ HTTP-REST adapter 的「另一種皮」**
   原本以為 Tekton Adapter 只要把 `client.go` 換成 `dynamic.Interface` 就好。實際上整個層次結構不同：認證走 kubeconfig、操作走 CRD、沒有 Queue / CSRF 這類 HTTP 世界特有的問題。設計要從「資源模型」而不是「API 動詞」出發。

2. **`dynamicfake` 和真實 K8s 行為不 100% 一致**
   generateName 是第一個踩到的坑；未來可能還有 `resourceVersion`、admission webhook 等差異。原則：**Adapter 應該盡量不依賴「伺服器自動填值」的行為**，自己控制能控制的。

3. **`ClusterResolver` 介面預先抽象是值得的**
   原本可以直接 import `ClusterInformerManager`，試過一次就發現會循環依賴。早點定義介面雖然多寫幾行，但反而簡化了測試（直接 inject fake）。

4. **StreamLogs 直接標 ErrUnsupported 是正確選擇**
   一開始想 "M18d 完整交付就要每個方法都可用"，但實作下來發現 Pod log 要正確處理的邊界太多。與其留下一個脆弱的實作，不如明確標記未支援，讓 follow-up 認真補。文件裡說清楚原因，不會誤導使用者。

5. **測試一個「外部 CI 引擎」其實等於測試一組 CRD round-trip**
   Tekton adapter 的絕大多數測試都是「塞個 unstructured.Unstructured 到 fake dynamic client → 呼叫 adapter → 驗證回應」。這個模式非常穩定，可直接複製到 M18e 的 Argo Workflows adapter。

---

## 10. 檢查清單（驗收用）

- [x] `go build ./...` 乾淨
- [x] `go vet ./...` 乾淨
- [x] 所有 79 個 M18d 新增測試通過（含 `-race`）
- [x] M18a / M18b / M18c 既有測試**未受影響**
- [x] 其他套件測試未受影響
- [x] `ADR-015` M18d 項目全標 ✅
- [x] `CI_ENGINE_ADAPTERS.md` §13 新增 Tekton 使用指引
- [x] 本實作紀錄文件完成

---

**決策者：** Architecture Team
**實作者：** Architecture Team
**最後更新：** 2026-04-16
