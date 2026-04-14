# Synapse CI/CD 架構設計文件

> 版本：v3.0（Final） | 日期：2026-04-14 | 狀態：定稿，準備進入實作
> 對應里程碑：M13（CI Pipeline 引擎）、M14（Git 整合）、M15（Registry 整合）、M16（原生 GitOps）、M17（環境流水線）
> 相關文件：[ARCHITECTURE_REVIEW.md](./ARCHITECTURE_REVIEW.md)、[PLANNING.md](../PLANNING.md)

---

## 目錄

1. [戰略目標與設計原則](#1-戰略目標與設計原則)
2. [整體架構流程](#2-整體架構流程)
3. [與現有系統的關係](#3-與現有系統的關係)
4. [現況差距分析](#4-現況差距分析)
5. [近期過渡方案（不需等 M13）](#5-近期過渡方案不需等-m13)
6. [Pipeline 引擎核心設計總覽](#6-pipeline-引擎核心設計總覽)
7. [Pipeline 執行模型（M13 核心）](#7-pipeline-執行模型m13-核心)
8. [M13a — 核心執行引擎（4 週）](#8-m13a--核心執行引擎4-週)
9. [M13b — 進階 Steps 與使用者體驗（4 週）](#9-m13b--進階-steps-與使用者體驗4-週)
10. [M14 — Git 整合與 Webhook 觸發](#10-m14--git-整合與-webhook-觸發)
11. [M15 — 映像 Registry 整合](#11-m15--映像-registry-整合)
12. [M16 — 原生輕量 GitOps（CD）](#12-m16--原生輕量-gitopscd)
13. [M17 — 環境管理與 Promotion 流水線](#13-m17--環境管理與-promotion-流水線)
14. [Trivy 整合與遷移](#14-trivy-整合與遷移)
15. [通知整合（復用 NotifyChannel）](#15-通知整合復用-notifychannel)
16. [Observability（Metrics / Audit / Events）](#16-observabilitymetrics--audit--events)
17. [失敗模式與故障恢復](#17-失敗模式與故障恢復)
18. [資料模型總覽](#18-資料模型總覽)
19. [技術選型](#19-技術選型)
20. [實作路線圖](#20-實作路線圖)
    - [20.1 優先順序矩陣與推薦模型](#201-優先順序矩陣與推薦模型)
21. [ADR（架構決策紀錄）](#21-adr架構決策紀錄)
22. [效能 SLA 與容量規劃](#22-效能-sla-與容量規劃)
23. [安全威脅建模（STRIDE-lite）](#23-安全威脅建模stride-lite)
24. [資料遷移腳本示意](#24-資料遷移腳本示意)
25. [附錄 A：Pipeline YAML Schema](#附錄-apipeline-yaml-schema)
26. [附錄 B：與 ARCHITECTURE_REVIEW.md 對應關係](#附錄-b與-architecture_reviewmd-對應關係)
27. [附錄 C：真實 Pipeline 範例集](#附錄-c真實-pipeline-範例集)
28. [附錄 D：Troubleshooting 手冊](#附錄-dtroubleshooting-手冊)

---

## 1. 戰略目標與設計原則

### 戰略目標

從「K8s 多叢集管理工具」演進為「端到端 DevSecOps 平台」，讓使用者在 **Synapse 單一介面**完成從 code commit 到生產部署的完整流程，不需要在 GitLab CI / Harbor / ArgoCD / 監控平台之間切換。

GitLab 在此架構中僅作為 **程式碼倉庫**，其餘 Pipeline 定義、執行、掃描、部署、通知，全部由 Synapse 集中管控。

### 設計原則

| 原則 | 說明 |
|------|------|
| **集中戰情室** | 所有 Pipeline 狀態、掃描結果、部署記錄、告警，統一在 Synapse 查看 |
| **執行引擎零額外安裝** | CI 執行引擎以 K8s Job 為單元，**Synapse 本身不需要額外安裝 Tekton / Jenkins**。Step 內使用的 image（Kaniko、Maven、Trivy 等）屬於基礎設施依賴，需要使用者環境可 pull（私有環境可 mirror 至 Harbor） |
| **安全左移** | Trivy 掃描內建於 Pipeline，掃描失敗則阻止 push to Harbor |
| **審核閘門** | 每個環境晉升（dev → staging → prod）可設定人工審核，**擴充**現有 Approval Workflow |
| **漸進式演進** | 近期過渡方案不依賴 M13，GitLab CI 仍可跑，結果推進 Synapse；M13 完成後無縫接管 |
| **可插拔** | 進階場景（複雜 DAG、特殊 build 工具）支援接入外部 Tekton/Jenkins |
| **復用既有資產** | 優先整合 `NotifyChannel`、`ApprovalRequest`、`ImageScanResult`、`ClusterInformerManager`、`AuditService`、AI 診斷，不另造平行設施 |
| **多租戶隔離** | Pipeline、Secret、Artifact 以「叢集 + 命名空間」為基礎權限邊界，搭配 Synapse 現有 RBAC |
| **安全預設** | 所有 Pipeline Pod 預設 `runAsNonRoot` + `seccompProfile: RuntimeDefault` + `ReadOnlyRootFilesystem`，例外需白名單 |

---

## 2. 整體架構流程

### 目標架構（M13–M17 全完成後）

```
開發者 git push
    │
    ▼
GitLab（純 Repo）
    │  Webhook（push event，HMAC 簽章）
    ▼
Synapse Webhook Receiver（M14）
    │  ① 公開路由群組 /api/v1/webhooks/*（跳過 JWT）
    │  ② HMAC 驗證 + Replay Protection（nonce LRU 5min）
    │  ③ Rate Limit（per-repo，防 DoS）
    │  ④ 比對 Pipeline 綁定規則（Repo + Branch glob + Path filter）
    │  ⑤ Concurrency Group 取消舊 Run（同 branch 覆蓋）
    ▼
Synapse Pipeline 引擎（M13）
    │  ⓐ Pipeline 版本快照 → pipeline_runs.snapshot_id
    │  ⓑ 解析 DAG → 推入執行佇列
    │  ⓒ 依並發上限從佇列取出 → 建立 K8s Job
    │
    ├─ Step 1: Build .jar（K8s Job / Maven / Gradle）
    │       └─ Workspace PVC / Cache Volume
    │
    ├─ Step 2: Build Container Image（K8s Job / Kaniko）
    │       └─ 以 PipelineSecret 注入 Registry 憑證
    │
    ├─ Step 3: Trivy 安全掃描（K8s Job / trivy image）
    │       │  ① 掃描結果寫入 ImageScanResult（scan_source=pipeline）
    │       │  ② 關聯 StepRun.scan_result_id
    │       │
    │       ├─ CRITICAL/HIGH 超標？
    │       │       ├─ Yes → Pipeline Failed
    │       │       │         ├─ 通知（NotifyChannel 路由）
    │       │       │         ├─ 記錄錯誤詳情 + AI 根因分析連結
    │       │       │         └─ 保留 Artifact 供重試
    │       │       └─ No  → 繼續
    │
    ├─ Step 4: Push Image to Harbor（M15）
    │           └─ 自動注入 imagePullSecret 到目標 Namespace
    │
    └─ Step 5: Deploy to K8s（kubectl apply / helm upgrade）
                ├─ 選項 A：直接 Apply 到目標叢集
                └─ 選項 B：觸發原生 GitOpsApp Sync（M16）
    │
    ▼
Synapse 集中戰情室
    ├─ Pipeline 執行狀態（DAG 視覺化 + 即時狀態）
    ├─ Trivy 掃描結果（CVE 列表，關聯到 StepRun）
    ├─ Artifact 歷程（image digest / scan report / build log）
    ├─ Deployment 狀態（Pod 數、Ready 狀態）
    ├─ 告警通知（走 NotifyChannel）
    ├─ 操作稽核（誰在何時觸發了哪條 Pipeline）
    └─ AI 根因分析（失敗 Run 一鍵診斷）
```

### 安全檢查點（縱深防禦）

```
Webhook 進入 ──► HMAC ──► Nonce 去重 ──► Rate Limit ──► Pipeline RBAC ──►
                                                                    │
                    ┌───────────────────────────────────────────────┘
                    ▼
Pipeline 觸發 ──► Step Image 白名單 ──► PipelineSecret 注入 ──► Pod Security Baseline ──►
                                                                    │
                    ┌───────────────────────────────────────────────┘
                    ▼
K8s Job 建立 ──► Resource Limits ──► NetworkPolicy（僅必要外連） ──► ServiceAccount 綁定 ──►
                                                                    │
                    ┌───────────────────────────────────────────────┘
                    ▼
執行輸出 ──► Log Scrubber（遮蔽已知 Secret 模式） ──► 持久化到 pipeline_logs
```

---

## 3. 與現有系統的關係

**不重造輪子**是這次重構的核心原則。下表定義新 CI/CD 模組與既有模組的**消費關係**：

| 既有模組 | 既有檔案 | 新模組使用方式 |
|---------|---------|--------------|
| `NotifyChannel` | `internal/models/notify_channel.go`、`handlers/notify_channel.go` | Pipeline 事件不另建通知子系統，統一走 `POST /notify-channels/:id/send` 內部呼叫 |
| `ApprovalRequest` | `internal/models/approval.go` | M17 Promotion 擴充現有模型，新增 `ResourceKind='Pipeline'`、`Action='promote_environment'` 類型 |
| `ImageScanResult` | `internal/models/security.go` | Trivy Step 直接寫入此表，新增 `scan_source`、`pipeline_run_id`、`step_run_id` 欄位 |
| `ClusterInformerManager` | `internal/k8s/` | Pipeline Job watcher 走此 manager 取 remote cluster client，不新開 Informer 池 |
| `AuditService` | `internal/services/audit_service.go` | Pipeline CRUD / 手動觸發 / 取消 全部 opLog 記錄 |
| `OperationAudit` middleware | `internal/middleware/` | 自動涵蓋 Pipeline 所有寫入動作 |
| `pkg/crypto` AES-256-GCM | `pkg/crypto/aesgcm.go` | `pipeline_secrets.value_enc`、`registries.password_enc` 加密 |
| Helm Release 管理（M4） | `internal/handlers/helm_release.go` | `deploy-helm` Step 直接調用 `HelmService`，部署歷程關聯 Pipeline Run |
| AI 診斷（M5/M7） | `internal/services/ai_*` | Pipeline 失敗時提供「AI 根因分析」按鈕，傳入 failed step 的 log + 錯誤碼 |
| ArgoCD 代理 | `internal/services/argocd_service.go` | `deploy-argocd-sync` Step 觸發 Sync，與原生 GitOpsApp（M16）共存 |
| Argo Rollouts | 叢集已安裝 `argoproj.io/v1alpha1` Rollout CRD | `deploy-rollout` Step 透過動態客戶端更新 Rollout image，觸發灰度流程；Synapse 提供狀態監控 + promote/abort 操作（ADR-010） |
| Rate Limit middleware | `internal/middleware/rate_limit.go` | Webhook 端點需要新增 per-provider + per-repo 限流，整合至 Redis backend（ARCHITECTURE_REVIEW P1-8）|

---

## 4. 現況差距分析

| 能力維度 | 現況 | 目標狀態 | 對應里程碑 |
|---------|------|---------|-----------|
| CI Pipeline 執行 | **完全沒有** | K8s Job 驅動的原生 Pipeline 引擎 | M13a/b |
| Git Webhook 接收 | 無 | 支援 GitLab / GitHub / Gitea | M14 |
| 映像建置 | 無 | Kaniko in K8s，無需 Docker daemon | M13a |
| Trivy 掃描 | **Host exec**（`trivy_service.go:96`）| 雙軌：Pipeline Step（K8s Job） + 既有 host exec（漸進汰除） | M13a + §14 |
| Harbor 整合 | 無 | Registry 管理 + Tag 瀏覽 + 自動 Push | M15 |
| GitOps / CD | 代理外部 ArgoCD | 原生 GitOps + 保留 ArgoCD 代理（明確邊界） | M16 |
| 環境流水線 | 僅 Namespace 粒度 | dev → staging → prod，含人工審核閘門（擴充 ApprovalRequest） | M17 |
| Pipeline Secrets | 無 | 加密儲存 `pipeline_secrets` 表，以 `${{ secrets.XXX }}` 引用 | M13a |
| Pipeline RBAC | 無 | 對應 Synapse 既有 Cluster/Namespace 權限 + Step image 白名單 | M13a |
| Pipeline Audit | 無 | 全部走 `OperationAudit` middleware，操作日誌集中查詢 | M13a |
| Webhook 安全 | 無 | HMAC + Nonce Replay Protection + Rate Limit + 公開路由群組 | M14 |
| 並發控制 | 無 | 每 Pipeline 並發上限 + 全系統並發上限 + Concurrency Group | M13a |
| Log 持久化 | K8s Pod log（Pod 被 GC 即消失） | `pipeline_logs` 表儲存（大 log 可選擇外掛 S3/MinIO） | M13a |
| Job GC | 無 | `ttlSecondsAfterFinished` + 後台 GC worker | M13a |
| 掃描結果集中查看 | 有（手動掃描結果存 DB） | Pipeline 掃描結果自動記錄 + 跨來源統一查詢 | M13b |
| 部署通知 | Prometheus Alert / K8s Event Alert / NotifyChannel | Pipeline 事件走 NotifyChannel，per-pipeline 可路由 | M13b |
| AI 整合 | 現有 AI 診斷 | Pipeline 失敗 → 一鍵根因分析 | M13b |
| 灰度發布 | 依賴外部 ArgoCD + Argo Rollouts，Synapse 無可視化 | `deploy-rollout` Step 觸發灰度 + Rollout 狀態監控 + promote/abort 操作 | M13c |
| Gateway API 流量切分 | Istio Gateway + Gateway API 已部署，Synapse 無整合 | Rollout 使用 Gateway API trafficRouting 插件，Synapse 展示灰度權重 | M13c |
| Pipeline 網路策略 | 無 | 自動偵測 Cilium → 使用 CiliumNetworkPolicy（L7）；降級為標準 NetworkPolicy | M13a |

---

## 5. 近期過渡方案（不需等 M13）

在 M13 完成前，可透過以下方式讓 Synapse 成為「部分戰情室」：

### 方案 A：GitLab CI 推送掃描結果到 Synapse

GitLab CI Pipeline 在 Trivy 掃描後，呼叫 Synapse API：

```yaml
# .gitlab-ci.yml
trivy-scan:
  script:
    - trivy image --format json --output trivy-result.json $IMAGE_TAG
    - |
      curl -X POST "$SYNAPSE_URL/api/v1/clusters/$CLUSTER_ID/security/scans" \
        -H "Authorization: Bearer $SYNAPSE_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
          \"image\": \"$IMAGE_TAG\",
          \"namespace\": \"$NAMESPACE\",
          \"pod_name\": \"$CI_PROJECT_NAME\",
          \"scan_source\": \"ci_push\"
        }"
```

**Schema 相容性重點（v1 遺漏）：** `ImageScanResult` 需先新增 `scan_source VARCHAR(20)` 欄位並加預設值 `host_exec`，過渡方案與未來 Pipeline Step 才能共用同一張表。

**效果：** Synapse 安全掃描頁可看到所有 CI 觸發的掃描結果，不再需要手動輸入映像名稱。

### 方案 B：Informer 自動掃描（Pod 上線時）

在 `ClusterInformerManager` 的 Pod `OnAdd / OnUpdate` 回調中，偵測到新映像時自動呼叫 `TrivyService.TriggerScan()`：

```go
// 偽代碼
func (m *ClusterInformerManager) onPodAdd(obj interface{}) {
    pod := obj.(*v1.Pod)
    for _, container := range pod.Spec.Containers {
        if isNewImage(container.Image) {
            m.trivyService.TriggerScan(clusterID, pod.Namespace, pod.Name,
                container.Name, container.Image, "informer")
        }
    }
}
```

**注意事項：**
- 需要防抖（debounce）避免 Deployment rolling update 多次觸發
- `TriggerScan` 的 dedupe 邏輯已存在於 `trivy_service.go:66-70`，無需重建
- 掃描結果同樣寫 `scan_source='informer'`

> 方案 A 與 B 不互斥，建議同時實施作為 M13 前的橋接方案。過渡期間 Trivy 仍由 Synapse host exec 執行（現狀），M13a 完成後切換為 K8s Job（見 §14）。

---

## 6. Pipeline 引擎核心設計總覽

### 核心概念

| 概念 | 說明 |
|------|------|
| `Pipeline` | Pipeline 定義，包含 Steps DAG、觸發條件、環境變數引用。屬於「叢集 + 命名空間」範疇 |
| `PipelineVersion` | Pipeline 每次編輯產生的不可變版本快照，`PipelineRun` 必引用某一版本 |
| `PipelineRun` | 一次具體執行記錄，含觸發來源、狀態、佇列時間、啟動時間、結束時間 |
| `StepRun` | 每個 Step 的執行記錄，對應一個 K8s Job，包含狀態、K8s Job 名稱、耗時 |
| `Workspace` | Pipeline Run 的工作目錄抽象；跨 Step 共享必須使用 PVC，`emptyDir` 僅限單 Step 暫存 |
| `PipelineSecret` | 加密儲存的 Pipeline 敏感資料，以 `${{ secrets.XXX }}` 引用 |
| `PipelineArtifact` | Pipeline Run 產出物（image digest、jar 檔、scan report 等）|
| `PipelineLog` | Step 執行 log 的持久化記錄 |
| `ConcurrencyGroup` | 同組 Run 互斥，預設同 Pipeline + 同 branch 會取消前一個 Run |

### 高層架構

```
┌─────────────────────────────────────────────────────────────────┐
│                     Synapse Pipeline Subsystem                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐   ┌─────────────┐   ┌────────────────────┐   │
│  │  API Layer   │──▶│   Service    │──▶│  Executor Layer    │   │
│  │  (handlers)  │   │   Layer      │   │  (job builder +    │   │
│  │              │   │              │   │   job watcher)     │   │
│  └──────────────┘   └─────────────┘   └─────────┬──────────┘   │
│         │                  │                    │              │
│         │                  ▼                    ▼              │
│         │          ┌──────────────┐   ┌────────────────────┐   │
│         │          │  Queue       │   │ ClusterInformerMgr │   │
│         │          │  Scheduler   │   │  (remote cluster   │   │
│         │          │              │   │   k8s clients)     │   │
│         │          └──────────────┘   └────────────────────┘   │
│         ▼                                        │              │
│  ┌──────────────┐                               ▼              │
│  │  SSE Stream  │◀──────────── Job Log/Status Updates          │
│  └──────────────┘                                              │
│                                                                 │
│  ┌──────────────┐   ┌─────────────┐   ┌────────────────────┐   │
│  │  GC Worker   │   │ Log Retain  │   │  Artifact Retain   │   │
│  │  (清理 Job)   │   │  Worker     │   │  Worker            │   │
│  └──────────────┘   └─────────────┘   └────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 7. Pipeline 執行模型（M13 核心）

### 7.1 執行引擎流程

```
觸發（手動 / Webhook / Schedule）
    ↓
權限檢查：建立者 + 目標 cluster/namespace write 權限
    ↓
複製當前 Pipeline 定義 → 產生 PipelineVersion 快照（若已存在則復用）
    ↓
建立 PipelineRun（status: queued，關聯 snapshot_id）
    ↓
Concurrency Group 判斷（同 group 舊 Run → 標記 cancelled）
    ↓
排入全系統執行佇列（依優先級 + 先到先服務）
    ↓
Scheduler Loop 檢查並發上限（每 Pipeline / 全系統 / 每叢集）
    ↓
達到執行條件 → status: running，設定 started_at
    ↓
解析 Steps DAG（拓撲排序 + 同層並行分組）
    ↓
依層提交 K8s Job（注入 PipelineSecret、設定 SecurityContext、Resource Limits）
    ↓
JobWatcher（基於目標叢集的 Informer）Watch Job 狀態
    ↓
即時更新 StepRun 狀態 + Log 串流至 SSE + 持久化至 pipeline_logs
    ↓
同層全部成功 → 進入下一層；任一失敗 → 依 on_failure 策略處理
    ↓
所有 Steps 完成：
    ├─ success → PipelineRun: success → Artifact 關聯 → 通知（可選）
    ├─ failure → PipelineRun: failed → 取消未完成 Steps → 通知 + AI 分析連結
    └─ cancel  → PipelineRun: cancelled → 取消未完成 Steps → 記錄取消來源
```

### 7.2 Pipeline 定義版本化

v1.0 的 `steps_json TEXT` 直接存於 `pipelines` 表，Pipeline 被修改後歷史 Run 無法重現。v2.0 拆分：

```sql
CREATE TABLE pipelines (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    cluster_id  BIGINT NOT NULL,
    namespace   VARCHAR(255) NOT NULL,
    current_version_id BIGINT,     -- FK → pipeline_versions.id
    concurrency_group  VARCHAR(255),
    max_concurrent_runs INT DEFAULT 1,
    created_by  BIGINT NOT NULL,
    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME,
    UNIQUE KEY uq_pipeline_name (cluster_id, namespace, name, deleted_at)
);

CREATE TABLE pipeline_versions (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    pipeline_id BIGINT NOT NULL,
    version     INT NOT NULL,               -- 遞增版本號
    steps_json  TEXT NOT NULL,              -- 不可變 Steps DAG JSON
    triggers_json TEXT,                     -- 觸發條件
    env_json    TEXT,                       -- 預設環境變數
    hash_sha256 VARCHAR(64) NOT NULL,       -- 內容 hash，相同內容復用版本
    created_by  BIGINT NOT NULL,
    created_at  DATETIME,
    UNIQUE KEY uq_version (pipeline_id, version)
);
```

`PipelineRun.snapshot_id` 指向 `pipeline_versions.id`，執行中無論 Pipeline 如何被修改，Run 永遠按當時版本執行。

### 7.3 Secrets 管理

**v1.0 缺口：** 把 Git Token、Harbor 帳密塞進 `steps_json` 明文。v2.0 改為：

```sql
CREATE TABLE pipeline_secrets (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    scope       VARCHAR(20) NOT NULL,       -- global / cluster / pipeline
    scope_ref   BIGINT,                     -- cluster_id 或 pipeline_id
    name        VARCHAR(100) NOT NULL,      -- 例：HARBOR_PASSWORD
    value_enc   TEXT NOT NULL,              -- AES-256-GCM 加密（pkg/crypto）
    description VARCHAR(255),
    created_by  BIGINT NOT NULL,
    created_at  DATETIME,
    updated_at  DATETIME,
    UNIQUE KEY uq_scope_name (scope, scope_ref, name)
);
```

**引用語法（Step 內）：**

```yaml
steps:
  - name: build-image
    type: build-image
    image: gcr.io/kaniko-project/executor:v1.20.0
    env:
      DOCKER_USERNAME: ${{ secrets.HARBOR_USERNAME }}
      DOCKER_PASSWORD: ${{ secrets.HARBOR_PASSWORD }}
```

**注入機制：**
1. Scheduler / JobBuilder 在提交某個 Step 的 K8s Job 前解析該 Step 使用到的 `${{ secrets.* }}` 引用
2. 以 **StepRun** 為單位建立暫時 K8s Secret（`generateName: pr-run-{runId}-step-{stepRunId}-`）
3. 僅將該 Step 所需的 keys 寫入 Secret，Step Pod 以 `envFrom: secretRef: ...` 或 `volume.secret` 掛載
4. StepRun 結束後 Secret 自動刪除（`ownerRef` 指向該 Step 對應的 Job）
5. **Log Scrubber：** log 串流時用正則過濾已知 secret 值 → 以 `***REDACTED***` 取代

**權限：**
- `global` scope：PlatformAdmin only
- `cluster` scope：該叢集 Writer
- `pipeline` scope：該 Pipeline 建立者或 Writer
- 讀取「原始值」永遠不開放 API，只能重新設定

### 7.4 Workspace 與 Artifact

**Workspace**：Pipeline Run 的工作目錄抽象。

| 模式 | 儲存介質 | 適用場景 |
|------|---------|---------|
| `emptyDir` | Pod 本地 | 單一 Step 內的暫存 scratch space；**不支援跨 Step 傳檔** |
| `pvc` | 動態建立 PVC | Run 級共享 workspace；凡是 checkout → build → package 這類跨 Step 檔案傳遞都必須使用 |

Workspace 在 `PipelineRun` 結束後自動釋放，pvc 模式有 `retentionHours` 可設定延長。

**重要收斂：** 本文件的執行模型固定為「1 Step = 1 K8s Job」。因此 `emptyDir` 只能存在於該 Job Pod 生命週期內，不能作為預設共享 workspace。若 Pipeline 需要把原始碼、build output、Helm package、Terraform plan 等檔案交給後續 Step，`workspace.type` 必須明確設為 `pvc`。

**Artifact**：Pipeline Run 產出物的**中繼資料**（檔案本體不儲於 DB）。

```sql
CREATE TABLE pipeline_artifacts (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    pipeline_run_id BIGINT NOT NULL,
    step_run_id     BIGINT NOT NULL,
    kind            VARCHAR(50),            -- image / jar / scan_report / yaml / helm_chart
    name            VARCHAR(255),
    reference       TEXT,                   -- image digest / OSS URL / ImageScanResult ID
    size_bytes      BIGINT,
    metadata_json   TEXT,                   -- 額外元資料（如 image labels）
    created_at      DATETIME,
    expires_at      DATETIME,
    INDEX idx_run (pipeline_run_id),
    INDEX idx_kind (kind)
);
```

**保留策略：** `ArtifactRetainWorker` 每小時掃描 `expires_at < now()` 的記錄，僅刪除中繼資料；實體清理依賴對應儲存系統（如 Harbor tag retention policy）。

#### 7.4.1 vSphere CSI 拓撲約束

生產環境使用 vSphere CSI 並設定 `WaitForFirstConsumer` 綁定模式，PVC 只在 Pod 調度到具體 Node 後才綁定。由於本文件的執行模型為「1 Step = 1 K8s Job」，跨 Step 共享 PVC 時需要注意：

**問題：** 第一個 Step 的 Job Pod 綁定 PVC 到 Node-A 後，後續 Step 的 Job 必須也調度到 Node-A，否則無法掛載同一 PVC（`ReadWriteOnce` 限制）。

**JobBuilder 處理方式：**

```go
// 當 workspace.type = pvc 時，記錄首個 Step 的 Node 並傳遞給後續 Steps
func (b *JobBuilder) buildJobSpec(run *PipelineRun, step *StepDef) *batchv1.Job {
    if run.WorkspaceType == "pvc" && run.BoundNodeName != "" {
        job.Spec.Template.Spec.NodeSelector = map[string]string{
            "kubernetes.io/hostname": run.BoundNodeName,
        }
    }
}
```

**替代方案（優先順序）：**
1. 若叢集有支援 `ReadWriteMany` 的 StorageClass（NFS / CephFS），Pipeline 定義中指定 `workspace.storageClassName` 使用該 StorageClass，無需 nodeSelector 約束
2. 若只有 vSphere CSI（`ReadWriteOnce`），JobBuilder 自動加 nodeSelector
3. Pipeline YAML 可透過 `workspace.storageClassName` 顯式指定 StorageClass

### 7.5 跨叢集執行路徑

**v1.0 的重大缺口：** Synapse 是多叢集管理器，Pipeline 的 `cluster_id` 指定執行叢集，但 v1 沒寫 watcher 要如何跨遠端叢集運作。

**v2.0 設計：**

```
Synapse 後端（可跑在 Cluster A）
    │
    │  根據 Pipeline.cluster_id 查 Cluster
    │  → 透過 ClusterInformerManager.GetK8sClient(cluster)
    │
    ├─ 目標是 Cluster A（Synapse 本地）：
    │     直接以 in-cluster client 建立 Job
    │
    └─ 目標是 Cluster B（遠端匯入）：
          透過現有 ClusterInformerManager 取得 Client，建立 Job
          Job Watcher 依賴該叢集的 Informer（JobInformer）
```

**關鍵決策：**
- **不新增獨立的 JobInformer 池**。Synapse 現有 `ClusterInformerManager` 已經管理多叢集 Informer 生命週期。新增一個「Pipeline Job Watcher」訂閱既有 Informer 的 Job 事件即可。
- **Pipeline Job 專屬 Namespace 標籤**：`synapse.io/pipeline-run-id=<id>`，Watcher 用 label selector 過濾。
- **遠端叢集必須已被匯入 Synapse**。未匯入叢集不能當執行目標。
- **叢集失聯時的行為**：若目標叢集 Informer 5 分鐘內無法恢復，PipelineRun 狀態改為 `failed`，錯誤訊息 `target cluster unreachable`。

### 7.6 Pipeline RBAC 與執行身份

**平台層 RBAC**（誰能管理 Pipeline）：

| 動作 | 最低權限 |
|------|---------|
| 列出 Pipeline | 該 cluster + namespace 的 `read` |
| 建立 Pipeline | 該 cluster + namespace 的 `write` |
| 編輯 Pipeline | 該 cluster + namespace 的 `write` |
| 刪除 Pipeline | 該 cluster + namespace 的 `write` + 本人建立 OR PlatformAdmin |
| 手動觸發 Run | 該 cluster + namespace 的 `write` |
| 取消 Run | 該 cluster + namespace 的 `write` |
| 查看 Run log | 該 cluster + namespace 的 `read` |
| 管理 PipelineSecret | 該 scope 的 `write` + 敏感資料操作稽核 |

**執行層身份**（Pipeline Job 在 K8s 內跑時的身份）：

```yaml
# Pipeline 定義裡可設定
runtime:
  service_account: pipeline-runner   # 必須預先存在於目標 namespace
  allowed_step_images:               # 白名單（由 PlatformAdmin 維護）
    - gcr.io/kaniko-project/executor:*
    - aquasec/trivy:*
    - bitnami/kubectl:*
    - maven:3.9-*
```

- 每個 Pipeline 的 Job 使用 **預先建立的 ServiceAccount**，而非 Synapse 自己的 SA
- Synapse 只需要有「建立 Job / Secret / 讀 Pod log」的最小權限
- Step image 必須在 PlatformAdmin 維護的全域白名單內（防止使用者跑任意映像）
- 白名單支援 glob，並可設定「每個 cluster 的例外清單」

### 7.7 Pipeline Pod Security Baseline

**所有 Pipeline Job 預設 PodSpec：**

```yaml
spec:
  automountServiceAccountToken: false        # 預設不掛 token；只有需要呼叫 K8s API 的 Step 才開啟
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: step
      image: ${step.image}
      imagePullPolicy: IfNotPresent
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
      resources:
        limits:
          cpu:    ${step.cpu_limit | default "1000m"}
          memory: ${step.memory_limit | default "2Gi"}
          ephemeral-storage: ${step.disk_limit | default "4Gi"}
        requests:
          cpu:    ${step.cpu_request | default "100m"}
          memory: ${step.memory_request | default "256Mi"}
      volumeMounts:
        - name: workspace
          mountPath: /workspace
        - name: tmp
          mountPath: /tmp
  volumes:
    - name: workspace
      emptyDir: {}
    - name: tmp
      emptyDir: {}
```

若 Step 類型需要呼叫目標叢集 API（例如 `deploy`、`deploy-helm`、`deploy-argocd-sync`、`gitops-sync`），且 `runtime.service_account` 已配置，JobBuilder 才會把 `automountServiceAccountToken` 打開。純 build / test / scan 類型 Step 一律維持 `false`。

若 `workspace.type = pvc`，上方 `workspace` volume 需改為掛載該次 Run 專屬的 PVC；`emptyDir` 只用於 Step 內暫存。

**例外處理（需明確宣告）：**
- **Kaniko** 需要寫入 root filesystem → 該 Step 的 `readOnlyRootFilesystem: false`，需 PlatformAdmin 批准並記錄稽核
- **某些 build 工具** 需要 `/var/run/docker.sock`：**不允許**，改用 Kaniko/BuildKit rootless 模式

**NetworkPolicy（在目標 Namespace 套用）：**
- 預設：egress 僅允許 DNS + Git providers + Registries + Synapse backend 自身
- 可於 Pipeline 定義加白名單 host

#### 7.7.1 Istio Ambient 環境注意事項

生產環境使用 Istio 1.29.0 Ambient 模式，Pipeline Pod 若在已納入 Ambient mesh 的 Namespace 執行，可能受到 ztunnel 的 mTLS 攔截影響：

| 影響 | 說明 |
|------|------|
| Registry 連線 | `build-image`（Kaniko）和 `push-image` 步驟連接外部 Harbor 時，ztunnel 可能攔截流量 |
| DNS 解析 | 解析路徑可能改變，增加延遲 |
| 網路延遲 | 額外的 mTLS 握手增加 build 時間 |

**處理方式（三選一，優先序由高到低）：**

1. **專屬 Pipeline Namespace**（推薦）：使用 `synapse-pipeline-jobs` namespace 並排除 mesh
   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: synapse-pipeline-jobs
     labels:
       istio.io/dataplane-mode: none
   ```

2. **Pod 級別排除**：JobBuilder 在 Pipeline Pod 上加 annotation
   ```yaml
   metadata:
     annotations:
       ambient.istio.io/redirection: disabled
   ```

3. **不處理**：若 Pipeline 在業務 Namespace 執行且需要 mesh 內服務互訪，保留 mesh 納管

**JobBuilder 預設行為：** 對 `build-image`、`push-image`、`trivy-scan` 等需要外部連線的 Step 類型，自動加 annotation `ambient.istio.io/redirection: disabled`。`deploy`、`deploy-helm` 等叢集內操作的 Step 不加此 annotation。

**Cilium 兼容注意：** 生產方案要求 Cilium 與 Istio Ambient 使用特定兼容參數（`socketLB.hostNamespaceOnly=true`、`cni.exclusive=false`、`l7Proxy=false`）。Pipeline Pod 不應改動這些叢集級設定，只在 Pod/Namespace 層級處理。

#### 7.7.2 Cilium NetworkPolicy 自動偵測

生產環境使用 Cilium 作為 CNI，應優先使用 `CiliumNetworkPolicy`（支援 L7 策略）。JobBuilder 自動偵測並選擇對應的 NetworkPolicy 類型：

```go
func (b *JobBuilder) selectNetworkPolicyKind(ctx context.Context, clientset kubernetes.Interface) string {
    _, err := clientset.Discovery().ServerResourcesForGroupVersion("cilium.io/v2")
    if err == nil {
        return "CiliumNetworkPolicy"  // L7 能力，可按 HTTP path/method 控制
    }
    return "NetworkPolicy"  // 標準 K8s NetworkPolicy，僅 L3/L4
}
```

**CiliumNetworkPolicy 範例（Pipeline egress 規則）：**

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: pipeline-step-egress
spec:
  endpointSelector:
    matchLabels:
      synapse.io/pipeline-step: "true"
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
    - toCIDR:
        - 0.0.0.0/0       # Git providers + Registries（生產環境應收窄為具體 IP）
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
```

### 7.8 執行佇列與並發控制

**三級並發限制：**

| 級別 | 預設上限 | 可設定者 |
|------|---------|---------|
| 全系統 | 20 並發 Run | PlatformAdmin（系統設定）|
| 每叢集 | 10 並發 Run | PlatformAdmin per-cluster |
| 每 Pipeline | 1 並發 Run | Pipeline 編輯者 |

**佇列行為：**

```
incoming Run → status: queued（記錄 queued_at）
    ↓
Scheduler Loop（1s tick）：
  - 取出 queued Run
  - 檢查三級並發上限是否允許
  - 允許 → status: running，設定 started_at
  - 不允許 → 留在佇列繼續等待
    ↓
佇列長度 > 全系統上限 × 3 →
  新進 Run 進入「延遲拒絕」模式，status: rejected
  通知觸發者「佇列已滿，請稍後重試」
```

**Metrics：**
- `synapse_pipeline_queue_depth{cluster}` — 目前佇列長度
- `synapse_pipeline_run_wait_seconds` — queued → running 耗時（Histogram）

**M13a 單活限制：**
- Scheduler、Recover、GC Worker 在 M13a 採 **single-active controller** 模式
- Synapse 可以多副本提供 API / UI / SSE，但同一時間只能有一個實例啟用 Pipeline 背景 workers
- 若要支援多實例同時執行背景 workers，必須先補上 DB lease 或 K8s Lease leader election；否則會有雙重調度 / 雙重恢復 / 重複取消風險

### 7.9 Concurrency Group

同一個 git branch 連續 push 時，前一個尚未完成的 Run 應該被取消（避免浪費 build 時間）。

**v2.0 設計：**

- `Pipeline.concurrency_group` 預設為 `${pipeline_id}-${git_branch}`
- 若 Run 已在執行中（running）且新 Run 屬同 group：
  - 舊 Run 設為 `cancelling` → K8s Job 刪除 → `cancelled`
  - 新 Run 照常執行
- 使用者可在 Pipeline 設定 `concurrency_group_policy`:
  - `cancel_previous`（預設）
  - `queue`（排隊等待，不取消）
  - `reject`（拒絕新 Run）

**實作關鍵：** `PipelineRun` 表加 `concurrency_group VARCHAR(255)` index，Scheduler 每次建立 Run 時先查同 group 的 running Run。

### 7.10 失敗處理：重試、rerun、取消

**Step 級別重試：**

```yaml
steps:
  - name: build-image
    type: build-image
    retry:
      max_attempts: 3
      backoff_seconds: 30
      on_exit_codes: [125, 137]   # 僅對特定 exit code 重試
```

**Run 級別 rerun：**

| 動作 | 說明 |
|------|------|
| Rerun All | 建立新 Run，從第一步重跑（繼承 snapshot_id）|
| Rerun from Failed | 建立新 Run，已成功的 Step 跳過，從失敗點開始 |
| Rerun Single Step | 僅重跑特定 Step（需要該 Step 前置 Step 的 Artifact 仍有效）|

**取消：**

- 前端發送 `POST /pipelines/:id/runs/:runId/cancel`
- Scheduler 標記 Run 為 `cancelling`
- JobWatcher 逐一刪除 Step 對應的 K8s Job（`propagationPolicy: Background`）
- Step 收到 SIGTERM，有 `terminationGracePeriodSeconds: 30` 清理時間
- 所有 Job 清理完成 → Run 狀態 `cancelled`

### 7.11 Log 持久化與查詢

**v1.0 缺口：** 只提 SSE 串流，但 Pod 被 K8s GC 後 log 就消失。

**v2.0 雙層儲存：**

```sql
CREATE TABLE pipeline_logs (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    pipeline_run_id BIGINT NOT NULL,
    step_run_id     BIGINT NOT NULL,
    chunk_seq       INT NOT NULL,           -- 分塊序號
    content         LONGTEXT,               -- 單塊最大 1MB
    stored_at       DATETIME,
    INDEX idx_step (step_run_id, chunk_seq)
);
```

**寫入流程：**
1. JobWatcher 開啟 `clientset.CoreV1().Pods(ns).GetLogs(...)` stream
2. 每 500ms / 512KB 一塊持久化到 `pipeline_logs`
3. 同時 fan-out 到 SSE 訂閱者
4. Pod 結束後關閉 stream，寫入最後一塊

**保留策略：**
- 預設 30 天（`LogRetentionWorker` 依保留策略清理）
- 超過單塊 1MB → 自動分塊
- 未來可選掛載 S3/MinIO（僅儲存 reference，不改 API）

**查詢：**
- `GET /pipelines/:id/runs/:runId/steps/:step/logs?follow=true` → 正在執行時走 SSE
- `GET /pipelines/:id/runs/:runId/steps/:step/logs?follow=false` → 從 DB 查完整歷史

### 7.12 資源清理策略

**K8s Job：**
- 設定 `ttlSecondsAfterFinished: 3600`（1 小時）→ K8s 自動清理 Pod
- 同時設定 Synapse `GCWorker` 每 10 分鐘掃 `pipeline-run-id` label 的 orphan Job

**PipelineRun 記錄：**
- 預設保留 90 天
- `PipelineRunRetentionWorker` 依保留策略軟刪（`deleted_at`）
- 過期記錄的 Log、Artifact 中繼資料同步清理

**Workspace PVC：**
- Run 結束 → 立即刪除（預設）
- `retentionHours > 0` 則延遲刪除
- 每日掃描並強制清理逾期 PVC

---

## 8. M13a — 核心執行引擎（4 週）

**里程碑目標：** 4 週後可用 API 建立簡單 Pipeline（build-image + deploy），並在 Synapse 單叢集場景完整運作。

### 8.1 涵蓋範圍

- [x] Pipeline / PipelineVersion / PipelineRun / StepRun / PipelineSecret / PipelineArtifact / PipelineLog 資料模型
- [x] Pipeline CRUD + 版本快照邏輯
- [x] PipelineSecret CRUD + AES-256-GCM 加密
- [x] Step Image 白名單設定（PlatformAdmin 介面）
- [x] 執行佇列 + Scheduler Loop
- [x] JobBuilder（SecurityContext + ResourceLimits + NetworkPolicy hints）
- [x] JobWatcher（復用 ClusterInformerManager）
- [x] Log 雙層儲存（SSE + pipeline_logs）
- [x] GC Worker
- [x] 基本 Step 類型：`build-image`（Kaniko）、`deploy`（kubectl apply）、`run-script`（自訂指令）
- [x] 手動觸發 API（暫不含 Webhook）
- [x] 跨叢集執行路徑基本 path
- [x] OperationAudit 整合

### 8.2 API 設計（M13a）

```
# Pipeline CRUD
GET    /api/v1/clusters/:clusterID/pipelines
POST   /api/v1/clusters/:clusterID/pipelines
GET    /api/v1/clusters/:clusterID/pipelines/:id
PUT    /api/v1/clusters/:clusterID/pipelines/:id
DELETE /api/v1/clusters/:clusterID/pipelines/:id

# 版本
GET    /api/v1/clusters/:clusterID/pipelines/:id/versions
GET    /api/v1/clusters/:clusterID/pipelines/:id/versions/:version

# 執行
POST   /api/v1/clusters/:clusterID/pipelines/:id/run      手動觸發
GET    /api/v1/clusters/:clusterID/pipelines/:id/runs     執行歷史
GET    /api/v1/clusters/:clusterID/pipelines/:id/runs/:runId
POST   /api/v1/clusters/:clusterID/pipelines/:id/runs/:runId/cancel
POST   /api/v1/clusters/:clusterID/pipelines/:id/runs/:runId/rerun
POST   /api/v1/clusters/:clusterID/pipelines/:id/runs/:runId/rerun-failed

# 日誌（SSE + 歷史）
GET    /api/v1/clusters/:clusterID/pipelines/:id/runs/:runId/steps/:step/logs
       ?follow=true|false

# PipelineSecret
GET    /api/v1/pipeline-secrets?scope=global
POST   /api/v1/pipeline-secrets
PUT    /api/v1/pipeline-secrets/:id
DELETE /api/v1/pipeline-secrets/:id
       （value 永遠不回傳明文，只能重新設定）

# Step Image 白名單（PlatformAdmin only）
GET    /api/v1/system/pipeline/allowed-images
PUT    /api/v1/system/pipeline/allowed-images
```

### 8.3 路由註冊位置

依照 [CLAUDE.md §11](../CLAUDE.md#11-route-registration) 規則：

```
internal/router/
  routes_pipeline.go       ← 新增：cluster scoped pipeline CRUD + runs
  routes_pipeline_secret.go ← 新增：pipeline secrets
  routes_webhook.go        ← M14 新增：公開 webhook 端點（跳過 JWT）
```

所有叢集 scoped 路由自動掛載 `ClusterAccessRequired()` + `AutoWriteCheck()`。

### 8.4 程式碼結構（實際）

```
internal/
  handlers/
    pipeline_handler.go         ← Pipeline + Version CRUD
    pipeline_run_handler.go     ← Run trigger / cancel / rerun / list / get
    pipeline_secret_handler.go  ← Secret CRUD
    pipeline_log_handler.go     ← SSE + 歷史 log 查詢
    pipeline_webhook_handler.go ← Webhook 觸發（HMAC + nonce + timestamp）
  services/
    pipeline_service.go         ← 業務邏輯（Pipeline/Version/Run/Step CRUD）
    pipeline_scheduler.go       ← Scheduler loop + 並發控制 + DAG 執行
    pipeline_job_builder.go     ← JobBuilder（K8s Job spec + Secret 注入）
    pipeline_job_watcher.go     ← JobWatcher（Job 狀態同步 + log 收集）
    pipeline_secret_service.go  ← Secret CRUD + AES-256-GCM 加密
    pipeline_log_service.go     ← Log 雙層儲存 + Scrubber
    pipeline_step_types.go      ← Step 類型 registry + validation + command gen
    pipeline_retry.go           ← RetryPolicy（exponential / fixed backoff）
    pipeline_approval.go        ← Approval Step（approve / reject + scheduler 輪詢）
    pipeline_notify_dedup.go    ← 通知去重（5min 視窗 + LRU eviction）
    pipeline_notifier.go        ← Pipeline 事件 → NotifyChannel 路由（slack/telegram/teams/webhook）
    pipeline_rca.go             ← Pipeline 失敗 AI 根因分析（context 組裝 + AI 呼叫）
    pipeline_trigger_match.go   ← Webhook 觸發條件引擎（branch glob + path filter + cron 驗證）
    pipeline_gc_worker.go       ← GC Worker（孤兒 Job + Run 90d + Log 30d）
    pipeline_recover.go         ← 啟動時孤兒 Run 恢復
  models/
    pipeline.go                 ← Pipeline / PipelineVersion / PipelineRun / StepRun
    pipeline_secret.go
    pipeline_artifact.go
    pipeline_log.go
  router/
    routes_cluster_pipeline.go  ← Pipeline + Run + Secret + Log 路由
    routes_webhook.go           ← 公開 Webhook 端點（HMAC 驗證）
```

### 8.5 完成指標

- 建立一個 3-step Pipeline（build-image → scan → deploy），手動觸發成功
- 同 Pipeline 並發上限 = 1 時，第二次觸發正確排隊
- Pipeline 修改後，歷史 Run 仍能看到當時的 steps 定義
- PipelineSecret 加密儲存，Log 內不出現明文
- Run 結束 1 小時後 K8s Job 已被清理
- 跨叢集執行：Pipeline 目標為遠端匯入的叢集時，Job 建立在該叢集並正確監聯
- 所有 Pipeline 寫入動作出現在操作稽核

### 8.6 P0 地基層審計紀錄（2026-04-14）

P0 全 8 項（P0-1 ~ P0-8）+ P1-1、P1-3 實作完成後，對全部原始碼進行交叉審計，
發現並修復以下缺陷。所有修復已通過完整 `go test ./internal/...` 驗證。

#### 8.6.1 已修復缺陷

| # | 嚴重度 | 檔案 | 問題 | 影響 | 修復方式 |
|---|--------|------|------|------|----------|
| 1 | **Critical** | `pipeline_service.go` CreateVersion | `Scan(&maxVersion)` 未檢查 error | DB 查詢失敗時 maxVersion=0，導致版本號碰撞（version=1 重複建立） | 加上 `.Error` 檢查，失敗時回傳 error |
| 2 | **Critical** | `pipeline_scheduler.go` getPipelineMaxConcurrent | DB 查詢缺少 `WithContext(ctx)` | 查詢不帶 context，無法正確傳播 timeout/cancellation，長時間阻塞可能拖慢 scheduler tick | 新增 ctx 參數，透過 `canSchedule(ctx, ...)` 傳入 |
| 3 | **High** | `pipeline_job_watcher.go` syncStepRunStatus | log 中 `old_status` 是覆寫後的 newStatus | 狀態變更日誌無法正確追蹤「從哪個狀態轉移」，影響除錯與稽核 | 覆寫前先暫存 `oldStatus := sr.Status` |
| 4 | **High** | `pipeline_recover.go` finalizeOrphanedRun | `Count(&failedCount)` 未檢查 error | DB 查詢失敗時 failedCount=0，孤兒 Run 誤判為 success | 加上 error check，查詢失敗時保守標記為 failed |
| 5 | **High** | `pipeline_log_service.go` ScrubSecrets | `secretPattern` regex 定義後從未啟用 | Log Scrubber 無法偵測 `password=xxx`、`token=xxx` 等常見洩漏模式，僅靠直接值比對 | 在直接替換後加上 `secretPattern.ReplaceAllString()` |
| 6 | **Medium** | `pipeline_handler.go` GetVersion | 先 `parseIntQuery` 讀 query string，再 `parseUintParam` 讀 path param，邏輯衝突 | `GET /versions/:version` 的 version 永遠從 query string 讀取而非 path，路由無法正常匹配 | 直接使用 `parseUintParam(c, "version")` |
| 7 | **Medium** | `pipeline_secret_handler.go` ListSecrets | 解析 path param `clusterID` 後丟棄（`_ = ref`），scope_ref 從 query string 讀取但入口條件錯誤 | `?scope=pipeline&scope_ref=5` 查詢可能在沒有 clusterID path param 時失效 | 直接從 `c.Query("scope_ref")` 讀取，移除無用的 clusterID 解析 |
| 8 | **Low** | `pipeline_webhook_handler.go` nonceCache | cleanup goroutine `for range ticker.C` 無法停止 | 長期運行時 goroutine 洩漏（每次建立新 handler 時累積一個永久 goroutine） | 加 `stopCh` channel，改為 `select` 迴圈 |

#### 8.6.2 審計確認：非缺陷項目

| 項目 | 審計結論 | 說明 |
|------|----------|------|
| `008_pipeline.up.sql:62` snapshot_id FK 無 ON DELETE CASCADE | **正確設計** | Pipeline Version 是 immutable 快照，刪除版本不應級聯刪除歷史 Run 記錄 |
| `executeRunAsync` 使用 `context.Background()` | **正確設計** | 背景 goroutine 不應繫結 HTTP request context（CLAUDE.md §5 明確規範） |
| `tick()` 10s timeout vs `executeRunAsync` 2h timeout | **正確設計** | 兩者是獨立 context；tick 只負責排程決策，executeRunAsync 用自己的 2h timeout |
| `concurrencyCounts` 在 tick loop 中無鎖 | **正確設計** | counts 是 tick() 內的區域變數，單一 goroutine 順序操作，不存在併發競爭 |
| `stepRuns` map 在 executeRunAsync 中 | **正確設計** | map 是函數內區域變數，僅由單一 goroutine 讀寫，waitForStep 是同步等待 |

#### 8.6.3 第二次審計（2026-04-14）— 已修復缺陷

P0 首次審計修復後，進行第二輪全量交叉審計，發現 4 個新缺陷，均已修復並通過測試。

| # | 嚴重度 | 檔案 | 問題 | 影響 | 修復方式 |
|---|--------|------|------|------|----------|
| 1 | **High** | `pipeline_scheduler.go` executeRunAsync | 6 處 `db.Save()` 未檢查 error | Step Run / Pipeline Run 狀態更新靜默失敗，DB 斷線時狀態停留在 running 永不完結 | 所有 `Save()` 加 `.Error` 檢查 + `logger.Error` |
| 2 | **Medium** | `pipeline_service.go` CreateVersion | Hash 去重檢查在 Transaction 外（TOCTOU 競爭） | 併發建立相同內容版本時，兩個 goroutine 都通過 hash 檢查，建出重複版本 | 將 hash 去重查詢移入 Transaction 內 |
| 3 | **Medium** | `pipeline_recover.go` markRunFailed | bulk `Updates()` 未檢查 error | 取消 active steps 失敗時無日誌，孤兒 step 永遠停留在 running 狀態 | 加 `.Error` 檢查 + `logger.Error` |
| 4 | **Low** | `pipeline.go` Pipeline model | GORM `uniqueIndex:idx_pipeline_name` 含 `DeletedAt` 欄位 | GORM AutoMigrate 生成的 index 包含 `deleted_at` 欄位，與 SQL migration 的 `WHERE deleted_at IS NULL` partial index 定義衝突 | 移除 `DeletedAt` 上的 `uniqueIndex` tag，由 SQL migration 控制 |

---

## 9. M13b — 進階 Steps 與使用者體驗（4 週）

**里程碑目標：** 補齊生產級 Pipeline 所需的進階能力與完整 UI。

### 9.1 涵蓋範圍

- [x] 進階 Step 類型：`build-jar`（Maven/Gradle）、`trivy-scan`、`push-image`、`deploy-helm`、`deploy-argocd-sync`、`approval`、`notify`
- [x] Rollouts Step 類型（M13c）：`deploy-rollout`、`rollout-promote`、`rollout-abort`、`rollout-status`
- [x] Step 級別重試（retry + backoff）
- [x] Rerun from Failed
- [x] Matrix Builds（多組 env 組合）
- [x] Cache 機制（Maven .m2 / npm / Docker layer — 以共享 PVC 實作）
- [x] Workspace PVC 模式
- [x] NotifyChannel 整合（Pipeline 事件路由）
- [x] Pipeline 失敗 → AI 根因分析連結
- [x] 完整前端：PipelineList / Editor / RunList / RunDetail / DAG 視覺化 / LogViewer
- [x] Pipeline YAML 匯入/匯出（對應附錄 A Schema）

### 9.2 前端頁面設計

```
ui/src/pages/pipeline/
  ├── PipelineList.tsx         列表（狀態燈、最後執行、觸發者）
  ├── PipelineEditor.tsx       步驟卡片編輯器 + YAML 雙模式
  ├── PipelineRunList.tsx      執行歷史
  ├── PipelineRunDetail.tsx    DAG 進度圖 + 各步驟卡片
  ├── StepLogViewer.tsx        SSE 串流 + 歷史查詢（複用 Terminal 樣式）
  ├── PipelineSecretManager.tsx Pipeline scope secrets
  └── PipelineAllowedImages.tsx PlatformAdmin 用白名單管理
```

**PipelineRunDetail DAG 視覺化：**

```
[Build JAR] ──→ [Build Image] ──→ [Trivy Scan] ──→ [Push Harbor] ──→ [Deploy]
  ✅ 2m3s           ✅ 4m12s          ❌ 0m45s
                                    ↑
                              CRITICAL: CVE-2024-1234
                              HIGH: 3 個漏洞
                              → Pipeline 已中止
                              [ AI 分析失敗原因 ] [ 重跑從此步驟 ] [ 查看 CVE ]
```

### 9.3 完成指標

- 開發者可透過 UI 以 step 卡片方式組一條完整 Pipeline
- Trivy Step 失敗時，從 Run 詳情頁一鍵跳轉到 ImageScanResult 完整 CVE 清單
- Rerun from Failed 正常運作，已成功的 Step 顯示 `skipped`
- Matrix Build（例：Java 11/17/21 × amd64/arm64 共 6 個並行 Run）成功執行
- Pipeline 失敗通知透過既有 NotifyChannel 送達
- AI 根因分析按鈕可從失敗 Run 跳轉到 AI Chat，自動帶入 failed step log

### 9.4 M13c — Argo Rollouts 整合（2 週）

**里程碑目標：** 讓 Pipeline 可以觸發 Argo Rollouts 灰度發布，並在 Synapse 中監控灰度進度。

**前置條件：** M13a 核心引擎完成；目標叢集已安裝 Argo Rollouts CRD。

#### 9.4.1 新增 Step 類型

| Step 類型 | 說明 | CRD 依賴 |
|-----------|------|----------|
| `deploy-rollout` | 更新 Rollout 的 image，觸發灰度發布流程 | `argoproj.io/v1alpha1` Rollout |
| `rollout-promote` | Promote 灰度到下一步或全量 | 同上 |
| `rollout-abort` | 中止灰度、回滾到 stable 版本 | 同上 |
| `rollout-status` | 等待 Rollout 達到目標狀態（Pipeline 閘門） | 同上 |

**`deploy-rollout` Step 定義範例：**

```yaml
steps:
  - name: canary-deploy
    type: deploy-rollout
    config:
      rollout_name: backend-service
      namespace: app-prod
      image: harbor.internal/app/backend:${GIT_SHA}
      wait_for_ready: true          # 等待 Rollout 達到 Healthy 或 Paused
      timeout: 30m
```

**`rollout-status` Step 搭配 Canary 分析：**

```yaml
steps:
  - name: canary-analysis
    type: rollout-status
    config:
      rollout_name: backend-service
      namespace: app-prod
      timeout: 30m
      expected_status: healthy       # healthy / paused / progressing
      on_timeout: abort              # abort / fail（timeout 時的行為）
```

#### 9.4.2 實作要點

- **Observer Pattern 偵測**：遵循 CLAUDE.md §8，先透過 Discovery API 偵測 `argoproj.io/v1alpha1` 是否安裝，未安裝時 UI 顯示引導卡片
- **動態客戶端**：使用 `k8s.io/client-go/dynamic` 操作 Rollout CRD，不引入 Argo Rollouts Go SDK
- **狀態 Watch**：透過 `ClusterInformerManager` 的動態 Informer 監聽 Rollout 狀態變化
- **Gateway API trafficRouting**：Synapse 不直接操作 HTTPRoute 權重，由 Argo Rollouts Controller 的 Gateway API 插件負責；Synapse 只讀取 HTTPRoute 展示當前權重

#### 9.4.3 API 端點

```
# Rollout 狀態查詢（讀取）
GET  /api/v1/clusters/:clusterID/rollouts
GET  /api/v1/clusters/:clusterID/rollouts/:namespace/:name
GET  /api/v1/clusters/:clusterID/rollouts/:namespace/:name/analysis

# Rollout 操作（寫入）
POST /api/v1/clusters/:clusterID/rollouts/:namespace/:name/promote
POST /api/v1/clusters/:clusterID/rollouts/:namespace/:name/abort
POST /api/v1/clusters/:clusterID/rollouts/:namespace/:name/retry
```

#### 9.4.4 前端面板

```
ui/src/pages/rollout/
  ├── RolloutList.tsx              列表（灰度進度條、策略類型、當前權重）
  ├── RolloutDetail.tsx            詳情（灰度步驟進度 + 分析結果 + 操作按鈕）
  └── RolloutStatusWidget.tsx      嵌入 PipelineRunDetail 的灰度狀態卡片
```

**RolloutDetail 視覺化：**

```
┌─────────────────────────────────────────────────────────┐
│  Rollout: backend-service                    Canary     │
│  ─────────────────────────────────────────────────────── │
│                                                         │
│  Stable (v2.1.0)  ████████████████████░░░░  80%         │
│  Canary (v2.2.0)  ████░░░░░░░░░░░░░░░░░░░  20%         │
│                                                         │
│  步驟: [10%] → [20% ← 當前] → [50%] → [100%]           │
│                                                         │
│  Analysis:                                              │
│  ✅ 5xx-rate: 0.02% (< 5% threshold)                   │
│  ✅ latency-p99: 230ms (< 500ms threshold)              │
│                                                         │
│  [Promote 全量] [Pause] [Abort 回滾]                     │
└─────────────────────────────────────────────────────────┘
```

#### 9.4.5 完成指標

- Pipeline 使用 `deploy-rollout` Step 更新 image 後，Argo Rollouts 正確開始灰度
- Rollout 列表頁展示所有叢集的 Rollout 及灰度進度
- 從 Pipeline Run 詳情頁可一鍵 Promote 或 Abort
- 未安裝 Argo Rollouts 的叢集顯示 NotInstalledCard，不報錯
- `step_runs` 表正確記錄 `rollout_status` 和 `rollout_weight`

---

## 10. M14 — Git 整合與 Webhook 觸發

**估計工作量：4 週** | **優先級：🔴 高**

### 10.1 公開端點安全設計

Webhook 是**少數需要未授權即可接收的 API**。v1.0 未處理安全，v2.0 需明確：

```
internal/router/routes_webhook.go
    ├── 不套用 AuthRequired 中介層
    ├── 套用獨立的 WebhookRateLimit（per-provider + per-repo）
    ├── 套用 WebhookHMACVerify（依 provider 選對應 header）
    └── 套用 WebhookReplayProtection（nonce LRU 5 分鐘）
```

**路由註冊位置：**

```go
// 在 routes.go Setup() 裡獨立掛載，與 /api/v1 群組平行
webhook := r.Group("/api/v1/webhooks")
webhook.Use(middleware.WebhookRateLimit(rl))
webhook.Use(middleware.WebhookHMACVerify(db))   // 依 URL :provider 動態驗
webhook.Use(middleware.WebhookReplayProtection(cache))
{
    webhook.POST("/:provider/:token", handler.HandleWebhook)
}
```

**為何放在 /api/v1/webhooks 而非根路徑？**
保持 API prefix 一致，方便反向代理只對 `/api/` 放行。

### 10.2 HMAC 驗證與 Replay Protection

| Provider | 驗證方式 | Replay Protection |
|---------|---------|------------------|
| GitLab | `X-Gitlab-Token` + optional `X-Gitlab-Event-UUID` | UUID 進 LRU 5 分鐘 |
| GitHub | `X-Hub-Signature-256`（HMAC-SHA256）+ `X-GitHub-Delivery` | Delivery ID 進 LRU |
| Gitea | `X-Gitea-Signature`（HMAC-SHA256）+ `X-Gitea-Delivery` | Delivery ID 進 LRU |

**為何需要 Replay Protection？** 攻擊者錄下一個合法 webhook 包可重放。即使 HMAC 驗證通過，Replay Protection 能確保同一 delivery 只處理一次。

**實作：**
```go
// 簡化示意
type replayCache struct {
    mu   sync.Mutex
    seen *lru.Cache  // key: "provider:delivery_id", value: time.Time
}

func (c *replayCache) CheckAndRemember(key string) bool {
    c.mu.Lock(); defer c.mu.Unlock()
    if _, ok := c.seen.Get(key); ok {
        return false
    }
    c.seen.Add(key, time.Now())
    return true
}
```

**注意：** LRU cache 是 in-memory，多實例部署時需要切換到 Redis backend（對齊 ARCHITECTURE_REVIEW.md P1-8）。

### 10.3 Webhook 接收流程

```
POST /api/v1/webhooks/:provider/:token
    ↓
① WebhookRateLimit（per-provider + per-repo）
    ↓
② 查 git_providers 表找 :token 對應的 Provider 設定（含 HMAC secret）
    ↓
③ HMAC Signature 驗證
    ↓
④ Replay Protection（Delivery ID LRU 查重）
    ↓
⑤ 解析 Payload（branch / sha / author / changed files）
    ↓
⑥ 查詢綁定此 Repo + Branch 的 Pipeline（Pipeline.triggers 條件）
    ↓
⑦ 評估觸發條件（branch glob / path filter / event type）
    ↓
⑧ Concurrency Group 判斷（同 group 舊 Run → 取消）
    ↓
⑨ 建立 PipelineRun（TriggerBy="webhook:gitlab"、TriggeredByUser=system）
    ↓
⑩ 回傳 202 Accepted（Webhook 要求快速回應）
    ↓
非同步執行 Pipeline
```

### 10.4 Git Provider 資料模型

```sql
CREATE TABLE git_providers (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50) NOT NULL,    -- gitlab / github / gitea
    base_url        VARCHAR(512) NOT NULL,
    access_token_enc TEXT,                   -- AES-256-GCM，用於 API 呼叫（clone / MR 操作）
    webhook_secret_enc TEXT,                 -- AES-256-GCM，HMAC 驗證用
    webhook_token   VARCHAR(64) NOT NULL,    -- URL 內的 :token，用於查詢 provider
    created_by      BIGINT NOT NULL,
    created_at      DATETIME,
    updated_at      DATETIME
);
```

### 10.5 Pipeline 觸發條件設計

```yaml
triggers:
  - type: webhook
    provider: gitlab                          # 對應 git_providers.id
    repo: "company/backend-service"
    branch: "main"                            # 支援 glob，如 "release/*"
    events:
      - push
      - merge_request
    path_filter:                              # 可選：只有特定路徑變動才觸發
      - "src/**"
      - "Dockerfile"
  - type: schedule
    cron: "0 2 * * *"                         # 每天凌晨 2 點
```

---

## 11. M15 — 映像 Registry 整合

**估計工作量：3 週** | **優先級：🟡 中**

### 11.1 功能範圍

| 功能 | 說明 |
|------|------|
| Registry 連線設定 | Harbor URL / 使用者名稱 / 密碼 / TLS 驗證，**AES-256-GCM 加密** |
| Repository 瀏覽 | 列出 Project → Repository → Tag |
| Tag 詳情 | 映像大小、建立時間、Digest、Labels |
| Tag 保留策略設定 | 保留最新 N 個 Tag，自動清理舊 Tag |
| Trivy 掃描觸發 | 從 Tag 詳情頁直接觸發掃描（與 Pipeline Step 共用入口）|
| Pipeline 整合 | `push-image` Step 從 `pipeline_secrets` 取用憑證；`pull-image` 自動選最新 Tag |
| imagePullSecret 注入 | Deploy Step 自動在目標 Namespace 建立/更新 Secret |

### 11.2 支援 Registry

| Registry | 支援方式 |
|---------|---------|
| Harbor | Docker Registry API v2 + Harbor 專屬 API（Project / Robot Account）|
| Docker Hub | Docker Registry API v2 |
| 阿里雲 ACR | Docker Registry API v2 |
| AWS ECR | Docker Registry API v2 + AWS SDK（token 刷新）|
| GCR / GAR | Docker Registry API v2 + GCP OAuth |

### 11.3 資料模型

```sql
CREATE TABLE registries (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    name            VARCHAR(255) NOT NULL UNIQUE,
    type            VARCHAR(50),             -- harbor / dockerhub / ecr / gcr
    url             VARCHAR(512) NOT NULL,
    username        VARCHAR(255),
    password_enc    TEXT,                    -- AES-256-GCM
    insecure_tls    BOOLEAN DEFAULT FALSE,   -- 預設 false，符合 CLAUDE.md §10 規則
    ca_bundle_enc   TEXT,                    -- 自簽 CA
    default_project VARCHAR(255),            -- Harbor project 預設值
    created_by      BIGINT NOT NULL,
    created_at      DATETIME,
    updated_at      DATETIME
);
```

### 11.4 與 Pipeline Secrets 的關係

建立 Registry 時同時在 `pipeline_secrets` 建立兩筆對應記錄：
- `REGISTRY_<NAME>_USERNAME`
- `REGISTRY_<NAME>_PASSWORD`

Pipeline Step 可以直接引用：
```yaml
- name: push-image
  type: push-image
  registry: harbor-prod                           # 對應 registries.name
  # 引擎自動注入 REGISTRY_HARBOR_PROD_USERNAME/PASSWORD
```

這樣 Pipeline 不需要知道 Registry 的密碼細節，也不會出現明文。

---

## 12. M16 — 原生輕量 GitOps（CD）

**估計工作量：6 週** | **優先級：🟡 中**

### 12.1 ArgoCD / 原生 GitOps / Argo Rollouts 三方邊界

Synapse 已有 ArgoCD 代理（`internal/services/argocd_service.go`），M16 新增原生 GitOps，生產環境另有 Argo Rollouts。三者的職責邊界必須定義清楚（ADR-010）：

#### 部署後端選擇矩陣

| 場景 | ArgoCD | M16 原生 GitOps | Argo Rollouts | Synapse 角色 |
|------|--------|----------------|---------------|-------------|
| 全功能生產環境（已有 ArgoCD + Rollouts） | ✅ 已裝 | ❌ 不啟用 | ✅ 已裝 | 代理 + 可視化 + 觸發 |
| 精簡環境（無灰度需求） | ❌ | ✅ 啟用 | ❌ | 原生 GitOps |
| 精簡環境 + 灰度需求 | ❌ | ✅ 啟用 | ✅ 獨立裝 | 原生 GitOps + Rollout 狀態監控 |
| 僅 CD 灰度（不需 GitOps） | ❌ | ❌ | ✅ 已裝 | Pipeline `deploy-rollout` Step 直接觸發 |

#### Pipeline deploy Step 選擇

| Step 類型 | 後端 | 灰度能力 | 適用場景 |
|-----------|------|---------|---------|
| `deploy`（kubectl apply） | 直接 K8s API | ❌ | 簡單部署、非生產環境 |
| `deploy-helm`（helm upgrade） | Helm | ❌ | Helm Chart 部署 |
| `deploy-argocd-sync` | ArgoCD 代理 | ✅（若 App 指向 Rollout） | 已有 ArgoCD 的環境 |
| `gitops-sync`（M16 原生） | M16 diff 引擎 | ❌ | 精簡環境 |
| `deploy-rollout`（M13c） | Argo Rollouts 動態客戶端 | ✅ 原生 | 需要灰度的所有場景 |

#### 互斥規則（硬約束）

- **同一個 Application 不能同時被 ArgoCD 和 M16 原生 GitOps 管理**（雙控制器 = 資源被雙方 reconcile）
- **`deploy-argocd-sync` 和 `gitops-sync` 不能在同一 Pipeline 中對同一 App 使用**
- **`deploy-rollout` 可以與 `deploy-argocd-sync` 共存**（ArgoCD sync Rollout YAML → Rollouts Controller 執行灰度）

**前端導航：**
- 「GitOps 應用」頁面顯示**兩種來源**合併列表（`source: argocd` / `source: native`）
- ArgoCD 來源的 App 點進詳情走 ArgoCD 代理 API
- 原生來源的 App 走 M16 API
- 「灰度發布」獨立頁面展示所有叢集的 Rollout 狀態（不論部署方式）

### 12.2 設計分層

**Layer 1（原生內建）：** 輕量 GitOps，覆蓋 80% 使用場景
- 定義 `GitOpsApp`（Git Repo + 路徑 + 目標叢集 + Namespace）
- 定期 Diff（預設每 5 分鐘）
- 可設定 Auto Sync 或 Drift 通知
- 支援純 YAML manifest、Kustomize overlay、Helm Chart

**Layer 2（保留代理）：** 現有 ArgoCD 代理保留
- ArgoCD App Health 聚合到主儀表板
- Pipeline 部署步驟可選「觸發 ArgoCD Sync」而非直接 kubectl apply

### 12.3 Diff 引擎

```
Git Repo（目標狀態）
    ↓ clone / pull（使用 git_providers 中設定的 token）
Render（Kustomize build / helm template）
    ↓
K8s API（實際狀態）— 透過 ClusterInformerManager
    ↓
Strategic Merge Diff（k8s.io/apimachinery）
    ↓
有差異？
  ├─ Auto Sync ON  → kubectl apply
  └─ Auto Sync OFF → Drift 通知（NotifyChannel）
```

**效能考量：**
- Git clone 結果快取在 PVC，每次只 pull 更新
- 同一 Repo 多個 App 共享 clone 目錄
- Diff 計算 timeout 30s（避免大 chart 卡住 worker）

### 12.4 資料模型

```sql
CREATE TABLE gitops_apps (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    name            VARCHAR(255) NOT NULL,
    source          VARCHAR(20) DEFAULT 'native',  -- native / argocd（代理）
    git_provider_id BIGINT,
    repo_url        VARCHAR(512),
    branch          VARCHAR(255),
    path            VARCHAR(512),                  -- 在 repo 中的路徑
    render_type     VARCHAR(50),                   -- raw / kustomize / helm
    helm_values     TEXT,                          -- Helm values override JSON
    cluster_id      BIGINT NOT NULL,
    namespace       VARCHAR(255) NOT NULL,
    sync_policy     VARCHAR(50),                   -- auto / manual
    sync_interval   INT DEFAULT 300,               -- 秒
    last_synced_at  DATETIME,
    last_diff_at    DATETIME,
    status          VARCHAR(50),                   -- synced / drifted / error
    created_by      BIGINT NOT NULL,
    INDEX idx_source (source),
    INDEX idx_status (status)
);
```

---

## 13. M17 — 環境管理與 Promotion 流水線

**估計工作量：5 週** | **優先級：🟢 低**

### 13.1 環境概念

```
Environment（環境）
  ├── name: "dev"
  ├── cluster_id: 1
  ├── namespace: "app-dev"
  └── auto_promote: true      ← 自動晉升到下一個環境

Environment（環境）
  ├── name: "staging"
  ├── cluster_id: 1
  ├── namespace: "app-staging"
  └── auto_promote: false     ← 需人工審核

Environment（環境）
  ├── name: "production"
  ├── cluster_id: 2           ← 可以是不同叢集
  ├── namespace: "app-prod"
  └── approval_required: true ← 必須有審核人簽核
```

### 13.2 ApprovalRequest 擴充

現有 `ApprovalRequest` 模型（`internal/models/approval.go`）針對 K8s 資源動作（scale / delete / update / apply）。M17 需要擴充：

```go
// 現有欄位不動，新增：
type ApprovalRequest struct {
    gorm.Model
    // ... 原有欄位
    ResourceKind  string  // 原本：Deployment/StatefulSet/DaemonSet
                          // 新增：Pipeline / PipelineRun / Environment
    Action        string  // 原本：scale / delete / update / apply
                          // 新增：promote_environment / production_gate
    // 以下為新增欄位
    PipelineRunID *uint   `gorm:"index"`    // 關聯的 Run
    FromEnvironment string               // 例："staging"
    ToEnvironment   string               // 例："production"
}
```

**為何擴充既有模型而非建立新表？**
- Synapse 已有 Approval 列表頁，擴充後所有類型的審核集中一頁
- 現有的通知/批准/拒絕邏輯可以復用
- 對前端的新增工作量最小

### 13.3 Promotion 流程

```
Pipeline 執行成功（dev 部署完成）
    ↓
冒煙測試 Step（可選，timeout 10m）
    ↓
auto_promote = true？
  ├─ Yes → 自動建立 staging PipelineRun
  └─ No  → 建立 ApprovalRequest(Action=promote_environment, ResourceKind=Pipeline)
                ↓
           審核人收到通知（NotifyChannel）
                ↓
           核准 → 建立 staging PipelineRun
           拒絕 → 記錄原因，通知觸發者
    ↓
staging 部署成功
    ↓
Production Gate（永遠需要人工審核，Action=production_gate）
    ↓
部署到 production
```

### 13.4 資料模型

```sql
CREATE TABLE environments (
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    name              VARCHAR(255) NOT NULL,
    pipeline_id       BIGINT NOT NULL,
    cluster_id        BIGINT NOT NULL,
    namespace         VARCHAR(255) NOT NULL,
    order_index       INT NOT NULL,                   -- 晉升順序
    auto_promote      BOOLEAN DEFAULT FALSE,
    approval_required BOOLEAN DEFAULT FALSE,
    approver_ids      TEXT,                           -- 核准人 user_id 清單 JSON
    smoke_test_step_name VARCHAR(255),                -- 冒煙測試用哪個 Step 名稱
    UNIQUE KEY uq_pipeline_env (pipeline_id, name),
    INDEX idx_order (pipeline_id, order_index)
);
```

---

## 14. Trivy 整合與遷移

### 14.1 現況

- `internal/services/trivy_service.go:96` 以 `exec.CommandContext("trivy", "image", ...)` 直接在 Synapse host 執行
- 依賴 `trivy` 二進位安裝在 Synapse 容器 / host（現有做法）
- 掃描結果寫入 `image_scan_results`

### 14.2 問題

| 問題 | 影響 |
|------|------|
| Synapse 容器必須包含 trivy 二進位 | 映像膨脹、升級 trivy 需重新 release Synapse |
| 無法橫向擴充 | 多個掃描任務搶 host 資源 |
| 與 Pipeline trivy-scan Step 行為分歧 | 同一張表出現兩種來源但沒法區分 |
| Trivy DB 更新卡在 Synapse 容器 | 冷啟動時下載慢 |

### 14.3 雙軌過渡策略

**Phase 1（M13a 進行中）：** Schema 相容
- `ImageScanResult` 新增 `scan_source`（`host_exec` / `ci_push` / `informer` / `pipeline`）
- 新增 `pipeline_run_id BIGINT NULL`、`step_run_id BIGINT NULL` 關聯
- `trivy_service.go` 繼續走 host exec（不動）
- 新增 `POST /security/scans` 的 `scan_source` 參數支援（方案 A 需要）

**Phase 2（M13a 完成時）：** K8s Job 化
- 新增 `TrivyScanStep`，直接建立 K8s Job（`aquasec/trivy` image）
- Job 結束後，Synapse backend 讀取 trivy JSON 輸出（透過 Pod log 或 emptyDir → sidecar 上傳）
- 寫入相同 `image_scan_results` 表，`scan_source=pipeline`
- 現有前端 UI 完全不需要改（因為 schema 統一）

**Phase 3（M13b 完成後）：** 決策 host exec 去留
- 若 K8s Job 模式穩定，**逐步移除** `trivy_service.go` 的 host exec path
- 保留 API 入口，但後端改為建立「獨立 K8s Job」掃描
- 移除 Synapse 容器的 trivy 二進位依賴
- Synapse 映像縮小

**Phase 4（M13b 完成後）：** Trivy DB 集中更新
- 建立 `trivy-db-cache` PVC，由專屬 CronJob 每日更新
- 所有 trivy Pod 掛載此 PVC → 加速冷啟動、節省網路流量

### 14.4 API 向下相容

| 端點 | v1 行為 | v2 行為 |
|------|--------|--------|
| `POST /clusters/:id/security/scans` | Host exec 觸發掃描 | Phase 3 後改為建立 K8s Job，回傳格式不變 |
| `GET /clusters/:id/security/scans` | 列出 `host_exec` 來源 | 列出全部來源，新增 `scan_source` 欄位供過濾 |
| `GET /clusters/:id/security/scans/:id` | 含 resultJson | 不變 |

---

## 15. 通知整合（復用 NotifyChannel）

Pipeline 事件統一走 `NotifyChannel`，不另建通知子系統。

### 15.1 事件來源

| 事件 | 預設等級 | 來源 |
|------|---------|------|
| PipelineRun started | Info | PipelineExecutor |
| PipelineRun succeeded | Info | PipelineExecutor |
| PipelineRun failed | Error | PipelineExecutor |
| StepRun failed | Warning | JobWatcher |
| Trivy scan critical | Critical | TrivyScanStep |
| ApprovalRequest created (`promote_environment`) | Info | PromotionService |
| GitOpsApp drifted | Warning | GitOpsDiffEngine |
| Webhook rejected (rate limit / invalid HMAC) | Warning | WebhookHandler |

### 15.2 通知路由

```
Pipeline 事件 → PipelineNotificationDispatcher
                    ↓
        查詢 Pipeline.notify_channel_ids（JSON 陣列）
                    ↓
        為每個 channel_id 呼叫 NotifyChannelService.Send(channelID, payload)
                    ↓
        NotifyChannelService 根據 channel.Type 走對應 adapter
        （webhook / telegram / slack / teams / email）
```

### 15.3 Pipeline 的通知設定

```yaml
# Pipeline 定義
notifications:
  on_success:
    channels: [3, 7]          # channel_id 陣列
  on_failure:
    channels: [3]
  on_scan_critical:
    channels: [3, 5]          # 安全團隊渠道
```

**Pipeline 資料表新增欄位：** `notify_on_success TEXT`、`notify_on_failure TEXT`、`notify_on_scan_critical TEXT`（JSON 陣列）。

### 15.4 避免通知風暴

- 同一事件 5 分鐘內同 channel 不重複發送
- 失敗重試的 Run 不發「started」通知（避免噪音）
- Concurrency group 取消的 Run 不發通知

---

## 16. Observability（Metrics / Audit / Events）

對齊 [ARCHITECTURE_REVIEW.md](./ARCHITECTURE_REVIEW.md) 的 P1-10 OpenTelemetry 方向。

### 16.1 Metrics（Prometheus）

```
# 計數器
synapse_pipeline_runs_total{pipeline_id, status, trigger_type}
synapse_pipeline_step_runs_total{pipeline_id, step_name, status}
synapse_pipeline_webhook_received_total{provider, outcome}
synapse_pipeline_webhook_rejected_total{provider, reason}

# Histogram
synapse_pipeline_run_wait_seconds{pipeline_id}           # queued → running
synapse_pipeline_run_duration_seconds{pipeline_id}       # running → finished
synapse_pipeline_step_duration_seconds{step_type}

# Gauge
synapse_pipeline_queue_depth{cluster_id}
synapse_pipeline_concurrent_runs{cluster_id}
synapse_pipeline_log_storage_bytes
```

**整合方式：** 在 `internal/metrics/` 註冊 `PipelineMetrics`，由 `registry` 初始化時附加（遵循 `router.go:37` 現有 registry 模式）。

### 16.2 Audit

所有 Pipeline 寫入動作自動透過 `OperationAudit` middleware 寫入 `operation_logs`，包含：
- 誰建立/編輯/刪除了 Pipeline
- 誰觸發了 Run
- 誰取消了 Run
- 誰核准了 promote ApprovalRequest
- PipelineSecret 的 CRUD（但不記錄 value）

### 16.3 Pipeline 失敗 → AI 根因分析（差異化亮點）

現有 AI 診斷（M5/M7）已支援自由文字查詢，M13b 新增：
- Pipeline Run 詳情頁：失敗 Step 卡片上有 `[ AI 分析失敗原因 ]` 按鈕
- 點擊後開啟 AI Chat，自動填入：
  - Failed step 的最後 200 行 log
  - K8s Job / Pod describe 輸出
  - Pipeline 定義片段
- 使用者可以直接問「為什麼失敗？」AI 即有充足上下文

這是 Synapse 相對於純 CI 工具（Jenkins / Argo Workflows）的關鍵差異化點。

### 16.4 OpenTelemetry Tracing（未來）

當 ARCHITECTURE_REVIEW P1-10 落地後，Pipeline 執行路徑自動產生 trace：
```
span: pipeline.run
  ├─ span: pipeline.queue_wait
  ├─ span: pipeline.dag.resolve
  ├─ span: step.build-image
  │    ├─ span: k8s.job.create
  │    ├─ span: k8s.job.wait_completion
  │    └─ span: log.persist
  └─ span: step.deploy
       └─ ...
```

---

## 17. 失敗模式與故障恢復

設計階段就思考壞掉怎麼辦，而不是事故後補救。

### 17.1 失敗模式矩陣

| 失敗模式 | 影響 | 偵測 | 恢復 |
|---------|------|------|------|
| Synapse backend 重啟（Run 執行中）| running Run 的 Watcher 中斷 | Scheduler 啟動時掃描 `status=running` 的 Run | 重新 attach 至 K8s Job（label selector），恢復 log 讀取 |
| 目標叢集失聯 | Run 卡在 `running` | Informer health check | 5 分鐘超時 → Run 設為 `failed`，錯誤訊息 `target cluster unreachable` |
| K8s Job 建立失敗 | Step 無法啟動 | JobBuilder 回傳錯誤 | Step 直接 `failed`，記錄原因 |
| K8s Job Pod OOMKilled | Step failed | Pod status phase | 依 retry 策略重試 |
| Log pod 讀取失敗（Pod 消失） | 歷史 log 缺失 | GetLogs 回傳 404 | 記錄「log unavailable」，已持久化的 chunk 仍可查 |
| DB 暫時斷線 | 所有寫入失敗 | GORM error | Scheduler 以指數退避重試，超過 3 次則該 Run 標記 error |
| PipelineSecret 解密失敗 | Step env 無法注入 | crypto 回傳錯誤 | Step 直接 `failed`，錯誤訊息 `failed to decrypt secret: <name>` |
| 佇列長度爆滿 | 新 Run 被拒絕 | QueueDepth metric | 回傳 429，通知 PlatformAdmin |
| Webhook DDoS | Synapse 後端壓力 | Rate limit 觸發計數 | 自動 ban IP 30 分鐘，通知 PlatformAdmin |
| Rollout 分析失敗（v2.4） | 灰度自動 abort，Pod 回滾到 stable | Rollout status 變為 `Degraded` | `deploy-rollout` Step 標記 `failed`，Pipeline 觸發通知，Rollout Controller 自動回滾 |
| Rollout 灰度卡住（v2.4） | 灰度停在某個權重不前進 | `rollout-status` Step timeout | Step 依 `on_timeout` 策略處理（abort 或 fail），Pipeline 記錄當前灰度權重 |
| Argo Rollouts 未安裝（v2.4） | `deploy-rollout` Step 無法建立 | Discovery API 偵測失敗 | Step 直接 `failed`，錯誤訊息 `Argo Rollouts CRD not found in cluster`，UI 顯示安裝引導 |

### 17.2 故障恢復的程式碼入口

```go
// pipeline_executor.go
func (e *PipelineExecutor) Recover(ctx context.Context) error {
    // Startup recovery: re-attach to running runs
    var runs []models.PipelineRun
    if err := e.db.Where("status = ?", "running").Find(&runs).Error; err != nil {
        return fmt.Errorf("query running runs: %w", err)
    }
    for i := range runs {
        if err := e.reattach(&runs[i]); err != nil {
            logger.Warn("failed to reattach pipeline run, marking as error",
                "run_id", runs[i].ID, "error", err)
            e.markRunError(&runs[i], "reattach failed: "+err.Error())
        }
    }
    return nil
}
```

在 `router.go Setup()` 的 background worker 區塊新增：
```go
if err := pipelineExecutor.Recover(context.Background()); err != nil {
    logger.Error("pipeline executor recover failed", "error", err)
}
```

---

## 18. 資料模型總覽

```
pipelines ──────────────────────────────────────────────────┐
    │ 1:N                                                    │
pipeline_versions                                           │
    │ N:1 (current_version_id)                              │
    │                                                        │
pipelines ──────────────────────────────────────────────────┘
    │ 1:N
pipeline_runs ────────────────────────────────┐
    │ 1:N                                     │
step_runs                                     │
    │ 1:N                                     │
    ├─→ pipeline_logs                         │
    │ 0:1                                     │
    ├─→ image_scan_results（scan_source=pipeline）
    │ 1:N                                     │
    └─→ pipeline_artifacts                    │
                                              │
approval_requests ──── (pipeline_run_id) ────┘

environments ──────────────────────── FK ──► pipelines
    │ FK                                       │
clusters                                       │
                                                │
git_providers ──────────────────────── FK ────┘
    │
gitops_apps

registries ←── pipeline steps（push-image）
    │
    └─► pipeline_secrets (REGISTRY_*_USERNAME/PASSWORD)

notify_channels ←── pipeline.notify_on_*（JSON id list）
```

### 新增/修改的表

| 表 | 變更類型 | 說明 |
|---|---------|------|
| `pipelines` | 新增 | 精簡，指向 current_version |
| `pipeline_versions` | 新增 | 不可變版本快照 |
| `pipeline_runs` | 新增 | 含 snapshot_id、concurrency_group、trigger_type、triggered_by_user_id |
| `step_runs` | 新增 | 含 cluster_id、namespace、scan_result_id |
| `pipeline_secrets` | 新增 | AES-256-GCM 加密 |
| `pipeline_artifacts` | 新增 | 產物中繼資料 |
| `pipeline_logs` | 新增 | 持久化 Log 儲存 |
| `git_providers` | 新增 | Git Provider 設定 |
| `registries` | 新增 | Registry 設定 |
| `gitops_apps` | 新增 | 原生 GitOps App |
| `environments` | 新增 | M17 環境流水線 |
| `image_scan_results` | 修改 | 新增 `scan_source`、`pipeline_run_id`、`step_run_id` |
| `approval_requests` | 修改 | 擴充 `ResourceKind` 值域 + `pipeline_run_id`、`from_environment`、`to_environment` |

---

## 19. 技術選型

| 需求 | 選擇 | 理由 |
|------|------|------|
| CI 執行引擎 | K8s Job（原生）| 零額外元件，已是現有依賴；複雜場景可接 Tekton |
| 映像建置 | Kaniko | 無需 Docker daemon，在 K8s Pod 內安全執行 |
| Steps 間產物共享 | emptyDir（同 Node）/ PVC（跨 Node）| 簡單場景用 emptyDir，需持久化時用 PVC |
| 日誌串流 | SSE（Server-Sent Events）| 複用 Terminal 樣式，單向串流適合日誌場景 |
| 日誌持久化 | MySQL LONGTEXT（分塊）| 先用 DB，未來可選掛 S3/MinIO |
| Git Webhook 驗證 | 自實作 HMAC handler | 各 Provider 格式差異不大，無需引入外部 SDK |
| Replay Protection | hashicorp/golang-lru | 已是 Go 生態標準 LRU 實作 |
| GitOps Diff 引擎 | `k8s.io/apimachinery` strategic merge | 已是現有依賴，語義正確 |
| Kustomize 支援 | `sigs.k8s.io/kustomize/api` Go SDK | 無需主機安裝 kustomize 二進位 |
| Registry API | 標準 Docker Registry HTTP API v2 | Harbor / DockerHub / ECR 均相容 |
| Pipeline YAML 解析 | `sigs.k8s.io/yaml` | 已是現有依賴 |
| Pipeline Schema 驗證 | `github.com/xeipuuv/gojsonschema` | JSON Schema 驗證器，Schema 可共用到前端 |
| 秘密加密 | `pkg/crypto`（AES-256-GCM）| 已存在，KeyProvider 可插拔 |
| 進階 Pipeline（插件）| Tekton Pipelines | 複雜 DAG、特殊 build 工具場景 |
| 灰度發布 | Argo Rollouts（動態客戶端） | 復用成熟灰度引擎，不自行實作（ADR-010） |
| 灰度流量切分 | Argo Rollouts Gateway API 插件 | 與生產方案的 Istio Gateway + Gateway API 對齊 |
| Pipeline NetworkPolicy | CiliumNetworkPolicy / NetworkPolicy（自動偵測） | 優先使用 Cilium L7 策略，降級為標準 K8s NetworkPolicy |

### 19.1 版本兼容矩陣

基於生產集群方案文檔 v1.1，定義 Synapse CI/CD 子系統的組件兼容矩陣：

| 組件 | 最低版本 | 推薦版本（生產方案基線） | 說明 |
|------|---------|----------------------|------|
| Kubernetes | 1.30 | 1.34.4 | `ttlSecondsAfterFinished` 需 1.23+；Job indexing 需 1.24+ |
| Cilium | 1.14 | 1.19.1 | CiliumNetworkPolicy L7 需 1.14+；eBPF kubeProxyReplacement 需 1.12+ |
| Istio | 1.22 (Ambient) | 1.29.0 | Ambient 模式需 1.22+；`ambient.istio.io/redirection` annotation 需 1.22+ |
| ArgoCD | 2.8 | 3.3.3 | Application API v1alpha1 相容 |
| Argo Rollouts | 1.6 | 1.8.4 | Gateway API trafficRouting 插件需 1.6+；AnalysisTemplate v1alpha1 |
| vSphere CSI | 3.0 | 3.6.0 | WaitForFirstConsumer 需 3.0+；動態擴容需 3.1+ |
| Gateway API CRD | 1.0 | 1.5.0 | HTTPRoute v1 需 1.0+；GRPCRoute 需 1.1+ |
| containerd | 1.7 | 2.2.1 | SystemdCgroup 對齊 |

**兼容性檢查時機：**
- Synapse 啟動時，透過 Discovery API 檢查各可選組件版本
- 版本不兼容時記錄 `logger.Warn` 並在 UI 系統設定頁提示

---

## 20. 實作路線圖

### 近期（不依賴 M13，2–3 週）

- [ ] `image_scan_results` schema 擴充：`scan_source` + `pipeline_run_id` + `step_run_id`（M13a 前置）
- [ ] 近期過渡方案 A：GitLab CI 推送掃描結果 API 文件與測試
- [ ] 近期過渡方案 B：Informer Pod OnAdd/OnUpdate 自動觸發 Trivy 掃描（含 debounce）
- [ ] 定期重掃 Cron goroutine（每 24h 重掃現有映像）

### M13a — CI 執行引擎核心（4 週）

**Week 1：資料模型與版本快照**
- [ ] `pipelines` / `pipeline_versions` / `pipeline_runs` / `step_runs` 資料表建立 + AutoMigrate
- [ ] `pipeline_secrets` 加密儲存（pkg/crypto）
- [ ] `pipeline_artifacts` / `pipeline_logs` 資料表
- [ ] Pipeline 版本快照邏輯（hash 相同則復用版本）

**Week 2：執行引擎與 JobBuilder**
- [ ] PipelineExecutor（Scheduler loop + 並發控制）
- [ ] JobBuilder（SecurityContext / Resource Limits / PipelineSecret 注入）
- [ ] Step Image 白名單（PlatformAdmin API）
- [ ] 跨叢集執行路徑（ClusterInformerManager 整合）

**Week 3：JobWatcher 與 Log**
- [x] JobWatcher（訂閱 Informer Job 事件）
- [x] Log 雙層儲存（SSE + pipeline_logs）
- [x] Log Scrubber（過濾 secret 值）
- [x] GC Worker（K8s Job + Workspace PVC + Run/Log retention）

**Week 4：基本 Steps 與 API**
- [x] Step 類型：`build-image`（Kaniko）、`deploy`（kubectl apply）、`run-script`
- [x] Pipeline CRUD API + 手動觸發 API
- [x] Cancel / Rerun API
- [ ] OperationAudit 整合
- [ ] M13a E2E 測試：手動觸發一條 build + deploy Pipeline 成功

### M13b — 進階 Steps 與 UX（4 週）

**Week 5–6：進階 Steps**
- [ ] `build-jar`（Maven / Gradle）
- [ ] `trivy-scan`（K8s Job 版本，寫 `scan_source=pipeline`）
- [ ] `push-image`（整合 M15 Registry）
- [ ] `deploy-helm`（整合既有 HelmService）
- [ ] `deploy-argocd-sync`（整合既有 ArgoCDService）
- [ ] `approval` Step（整合 ApprovalRequest）
- [ ] Step 重試（retry + backoff）
- [ ] Rerun from Failed
- [ ] Matrix Builds（同層並行展開）

**Week 7：前端**
- [ ] `PipelineList.tsx` / `PipelineEditor.tsx`（卡片 + YAML 雙模式）
- [ ] `PipelineRunDetail.tsx`（DAG 進度圖）
- [ ] `StepLogViewer.tsx`（SSE + 歷史）
- [ ] `PipelineSecretManager.tsx`
- [ ] `PipelineAllowedImages.tsx`

**Week 8：通知與 AI 整合**
- [ ] NotifyChannel 整合（PipelineNotificationDispatcher）
- [x] AI 根因分析按鈕 + context 組裝
- [ ] Metrics 註冊（Prometheus）
- [ ] 失敗恢復（Executor.Recover on startup）
- [ ] 文件更新

### M14 — Git Webhook（4 週）

- [ ] `git_providers` 資料表 + CRUD API（AES-256-GCM 加密 webhook_secret）
- [ ] 公開路由群組 `/api/v1/webhooks`（跳過 JWT）
- [ ] WebhookRateLimit middleware（per-provider + per-repo）
- [ ] WebhookHMACVerify middleware（支援 gitlab / github / gitea）
- [ ] WebhookReplayProtection middleware（LRU cache）
- [ ] Webhook → Pipeline 觸發條件匹配
- [ ] Concurrency Group 取消邏輯
- [ ] Git Provider 連線設定 UI

### M15 — Registry 整合（3 週）

- [ ] `registries` 資料表（insecure_tls + ca_bundle_enc）
- [ ] Harbor API 整合（Project / Robot Account）
- [ ] Repository / Tag 瀏覽 UI
- [ ] Tag 保留策略
- [ ] `push-image` Step 從 pipeline_secrets 取憑證
- [ ] imagePullSecret 自動注入

### M16 — 原生 GitOps（6 週）

- [ ] `gitops_apps` 資料表（含 `source` 欄位區分 native / argocd）
- [ ] Diff 引擎（YAML / Kustomize / Helm）
- [ ] Git clone 快取 PVC
- [ ] Auto Sync / Drift 通知（NotifyChannel）
- [ ] 前端整合：ArgoCD 代理 + 原生 App 合併列表
- [ ] ArgoCD 代理保留並明確邊界

### M17 — 環境流水線（5 週）

- [ ] `environments` 資料表
- [ ] ApprovalRequest 擴充（Action=promote_environment/production_gate + pipeline_run_id）
- [ ] Promotion 邏輯（自動 / 人工審核）
- [ ] 冒煙測試 Step 整合
- [ ] Production Gate 通知

### M13c — Argo Rollouts 整合（2 週）

- [ ] Rollout CRD Discovery 偵測（Observer Pattern）
- [ ] `deploy-rollout` Step 類型（動態客戶端更新 Rollout image）
- [ ] `rollout-status` Step 類型（等待 Rollout 狀態 + timeout 處理）
- [ ] `rollout-promote` / `rollout-abort` Step 類型
- [ ] Rollout 狀態查詢 API（GET /rollouts）
- [ ] Rollout 操作 API（promote / abort / retry）
- [ ] `step_runs` 新增 `rollout_status` / `rollout_weight` 欄位
- [ ] 前端：RolloutList / RolloutDetail / RolloutStatusWidget（嵌入 RunDetail）
- [ ] 未安裝 Argo Rollouts 時顯示 NotInstalledCard

### Trivy 二階段遷移（併入 M13b/Post-M13）

- [ ] Phase 2：新增 TrivyScanStep（K8s Job 模式）
- [ ] Phase 3：`trivy_service.go` 的 host exec 改為建立 K8s Job
- [ ] Phase 4：`trivy-db-cache` PVC + 每日更新 CronJob

---

**總估計工作量：**
- M13a（4W）+ M13b（4W）+ M13c（2W）+ M14（4W）+ M15（3W）+ M16（6W）+ M17（5W）= **28 週**
- 可並行：M13c 可在 M13a 完成後與 M13b 並行；M14 可在 M13a 完成後開始；M15 可在 M13a 完成後開始
- Trivy 遷移 Phase 3/4 併入 M13b post-work，不計入主線

### 20.1 優先順序矩陣與推薦模型

本矩陣將所有實作任務按**優先序 × 風險等級**分層，並指派 Claude Code 推薦模型。選模原則：

- **Opus**：安全關鍵路徑（加密、公開端點、Pod 安全）、核心狀態機（Scheduler、Watcher、Reconciler）、不可逆設計（DB Schema 定案、ADR）
- **Sonnet**：有明確範本可循的 CRUD handler/service、UI 頁面、API wrapper、測試
- **Haiku**：純資料（seed JSON/YAML、i18n 字串、commit message）

#### P0 — 地基層（錯了會擴散到全部後續 Milestone）

| 優先序 | 任務 | 所屬 | 模型 | 風險說明 |
|-------|------|------|------|---------|
| ✅ P0-1 | Pipeline / PipelineVersion / PipelineRun / StepRun **DB Schema** | M13a W1 | **Opus** | Schema 定案影響 M13–M17 全部，錯了回不來 |
| ✅ P0-2 | PipelineSecret CRUD + AES-256-GCM 加密 | M13a W1 | **Opus** | 密碼學正確性 + KeyProvider 介面 |
| ✅ P0-3 | 執行佇列 + Scheduler Loop（並發控制） | M13a W2 | **Opus** | 並發控制 + 佇列飢餓預防 + 公平調度 |
| ✅ P0-4 | JobBuilder（SecurityContext / ResourceLimits / NetworkPolicy） | M13a W2 | **Opus** | Pod 安全基線，錯一個欄位 = 容器逃逸 |
| ✅ P0-5 | JobWatcher（跨叢集 Informer 狀態機） | M13a W3 | **Opus** | reconnect、事件 dedup、狀態不一致 = 丟 Run |
| ✅ P0-6 | Pipeline 版本快照邏輯（immutable + hash dedupe） | M13a W1 | **Opus** | 歷史 Run 不可重現 |
| ✅ P0-7 | Webhook HMAC / Replay Protection / Rate Limit | M14 | **Opus** | 公開端點被接管 = 整個平台淪陷 |
| ✅ P0-8 | Watcher re-attach + Scheduler Recover（§17） | M13a W4 | **Opus** | 重啟後 Run 資料損毀或雙重執行 |

#### P1 — 核心功能層（影響生產安全與使用者核心體驗）

| 優先序 | 任務 | 所屬 | 模型 | 風險說明 |
|-------|------|------|------|---------|
| ✅ P1-1 | Pipeline CRUD handler + service | M13a W4 | **Sonnet** | 有 CLAUDE.md §13 範本可循 |
| ✅ P1-2 | 基本 Step 類型（build-image / deploy / run-script） + Pipeline Run API | M13a W4 | **Opus** | Step type registry + validation + command gen + Run handler |
| ✅ P1-3 | Log 雙層儲存（SSE + pipeline_logs）+ Log Scrubber | M13a W3 | **Sonnet**（儲存）/ **Opus**（Scrubber） | Scrubber 漏洩 = Secret 外洩 |
| ✅ P1-4 | GC Worker（K8s Job + PVC + Log retention） | M13a W3 | **Opus** | 依 §7.12 策略實作：孤兒 Job 清理 + Run 90d + Log 30d |
| P1-5 | Rollout 狀態機（deploy-rollout / rollout-status） | M13c | **Opus** | 灰度操作錯誤 = 生產事故 |
| P1-6 | GitOps Diff 引擎 + Drift Detection | M16 | **Opus** | production drift 或反向 overwrite |
| P1-7 | Promotion 狀態機 + Policy 引擎 | M17 | **Opus** | 跳關、反向 promote |
| ✅ P1-8 | Approval Step（整合 ApprovalRequest） | M13b W5–6 | **Opus** | 審批狀態機 + waiting_approval 狀態 + approve/reject API |
| ✅ P1-9 | Step 級別重試（retry + exponential backoff） | M13b W5–6 | **Opus** | RetryPolicy + 指數/固定退避 + 最大 10 次 + 5min 上限 |
| ✅ P1-10 | Webhook 觸發條件引擎（branch glob / path filter / cron） | M14 | **Opus** | TriggerRule + EvaluateWebhookTriggers + branch glob/** + path filter + cron 驗證 + 35 測試 |

#### P2 — 進階功能層（標準實作，有範本可循）

| 優先序 | 任務 | 所屬 | 模型 | 說明 |
|-------|------|------|------|------|
| ✅ P2-1 | 進階 Step 類型（trivy-scan / push-image / deploy-helm / deploy-argocd-sync / notify） | M13b W5–6 | **Opus** | Config + validation + command gen，含 30 個測試 |
| P2-2 | GitHub / GitLab / Gitea Provider adapter | M14 | **Sonnet** | Opus 出 GitHub 範本後 Sonnet 複製 |
| P2-3 | Registry CRUD + Harbor / ECR / GCR adapter | M15 | **Sonnet** | API wrapper |
| P2-4 | Credential 加密儲存（Registry） | M15 | **Opus** | 復用 PipelineSecret 加密，key rotation 一致性 |
| P2-5 | GitOps Application CRUD + Reconcile Loop | M16 | **Opus**（Reconcile）/ **Sonnet**（CRUD） | Reconciler 狀態機需 Opus |
| P2-6 | ArgoCD / 原生 GitOps 邊界定義（§12.1） | M16 | **Opus** | 誤判 = 雙寫雙讀，生產事故 |
| P2-7 | rollout-promote / rollout-abort Step | M13c | **Sonnet** | 簡單 API 呼叫 |
| P2-8 | Environment CRUD + Promotion History | M17 | **Sonnet** | 依 §13 結構 |
| ✅ P2-9 | NotifyChannel 整合（Pipeline 事件路由） | M13b W8 | **Opus** | PipelineNotifier + 4 channel formats (slack/telegram/teams/webhook) + dedup 整合 + 19 測試 |
| ✅ P2-10 | 失敗告警去重（通知風暴防護） | M13b W8 | **Opus** | 5min 去重視窗 + LRU eviction + retry/concurrency 抑制 + 11 測試 |

#### P3 — 前端 / UX / 輔助工具

| 優先序 | 任務 | 所屬 | 模型 | 說明 |
|-------|------|------|------|------|
| P3-1 | PipelineList / PipelineEditor（卡片 + YAML 雙模式） | M13b W7 | **Sonnet** | UI 頁面 |
| P3-2 | PipelineRunDetail（DAG 進度圖）+ StepLogViewer（SSE） | M13b W7 | **Sonnet** | UI 頁面 |
| P3-3 | RolloutList / RolloutDetail / RolloutStatusWidget | M13c | **Sonnet** | UI 頁面 |
| P3-4 | Git Provider / Registry / Environment 管理 UI | M14–M17 | **Sonnet** | 表單頁 |
| ✅ P3-5 | AI 根因分析按鈕 + context 組裝 | M13b W8 | **Opus** | PipelineRCAService + BuildContext/Analyze + 失敗 Step log/Job/Pod 收集 + 9 測試 |
| ✅ P3-6 | Prometheus Metrics 註冊（§16） | M13b W8 | **Opus** | 9 指標（4 counter + 3 histogram + 2 gauge）+ convenience helpers + 7 測試 |
| P3-7 | Pipeline YAML Schema（附錄 A） | 跨 Milestone | **Opus** | 一次性定義，前後端共用 |
| P3-8 | Trivy 雙軌遷移 Phase 3–4 | Post-M13 | **Sonnet** | 可延後，現有 host exec 仍可用 |

#### 跨 Milestone 常駐任務

| 任務 | 模型 | 說明 |
|------|------|------|
| ADR 撰寫（§21 全部 10 條） | **Opus** | 架構決策留痕，不可降級 |
| STRIDE 威脅建模（§23） | **Opus** | 安全專業 |
| ✅ DB Migration SQL（§24） | **Opus** | 冪等性 + rollback 腳本（008_pipeline） |
| Pipeline 範例撰寫（附錄 C） | **Sonnet** | 依 YAML schema |
| Troubleshooting runbook（附錄 D） | **Sonnet** | 故障排除 |
| PR Code Review（M13a/M14 階段） | **Opus** | 地基階段不放鬆 |
| PR Code Review（M15/M17 階段） | **Sonnet** | CRUD 為主可降級 |
| 單元測試 | **Sonnet** | 依測試金字塔 |
| E2E 測試（M13 以上） | **Opus** | 測試案例設計 = 安全審查 |
| 種子 YAML / fixture JSON / i18n | **Haiku** | 純資料 |
| commit message | **Haiku** | conventional commit |

#### 模型選擇速查決策樹

```
這個任務會碰到以下任一嗎？
├─ Secrets / crypto / HMAC / JWT                         → Opus
├─ Webhook 未授權端點                                     → Opus
├─ Pod SecurityContext / RBAC                             → Opus
├─ 跨叢集狀態機 / Informer                                → Opus
├─ 並發控制 / 佇列 / Scheduler                            → Opus
├─ GitOps diff / reconcile                               → Opus
├─ Rollout 狀態機 / promote / abort                       → Opus
├─ DB Schema 定案 / Migration SQL                        → Opus
└─ 以上皆否
   ├─ 有 CLAUDE.md §13 handler 範本或現成模板？             → Sonnet
   ├─ 純 YAML / JSON / 範例 / i18n？                      → Haiku
   └─ 其他                                               → Sonnet（預設）
```

#### 紅線規則

以下檔案路徑**無論改動多小，一律 Opus**，不可降級：

- `internal/services/pipeline_executor.go`（Scheduler）
- `internal/services/pipeline_watcher.go`（JobWatcher）
- `internal/services/pipeline_secret*.go`（加解密）
- `internal/services/pipeline_version*.go`（版本快照）
- `internal/services/gitops_*.go`（diff 引擎）
- `internal/services/rollout_*.go`（灰度狀態機）
- `internal/router/routes_webhook.go`（公開端點）
- `internal/middleware/webhook_*.go`（HMAC / replay / rate limit）
- `internal/database/migrations/*.go`（Schema 遷移）

---

## 21. ADR（架構決策紀錄）

ADR（Architecture Decision Record）在此文件內集中管理，每筆記錄格式：**Context → Decision → Alternatives → Consequences**。實作或後續調整需引用 ADR 編號，避免重大設計被悄悄反轉。

### ADR-001：Pipeline 執行引擎採用 K8s Job，不引入 Tekton / Jenkins

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - Synapse 定位為「單一 binary 部署、零額外元件」的 K8s 平台。
  - 目標客群多為中型企業、內網環境，抗拒再安裝 Tekton Controller / Jenkins Master 等額外系統。
  - 現有 `ClusterInformerManager` 已具備多叢集 Job watch 能力，重複利用成本低。
- **Decision：**
  - CI Pipeline 執行單元 = 1 個 K8s Job（或 Pod），由 Synapse `PipelineExecutor` service 直接建立。
  - DAG 調度、重試、artifact、log 持久化與 SSE 串流全部由 Synapse 自行實作於 `internal/services/pipeline/`。
  - Step image（Kaniko、Maven、Trivy 等）屬於「基礎設施依賴」，可由使用者環境的 Harbor mirror 提供，不算 Synapse 元件。
- **Alternatives considered：**
  | 方案 | 否決原因 |
  |------|---------|
  | Tekton | 需安裝 CRDs + Controller + Triggers；生態豐富但違反「零元件」原則 |
  | Jenkins | 架構過重、資源吃緊、與 K8s 整合不夠原生 |
  | Argo Workflows | 需安裝 Controller；DAG 強大但與 Synapse 現有 Job watcher 重疊 |
  | 純 CronJob + custom script | 無法表達 DAG、無重試、無 artifact |
- **Consequences：**
  - ✅ 客戶零額外安裝；Synapse 單一 binary 即可提供 CI/CD。
  - ✅ 複用 `ClusterInformerManager`、`AuditService`、`NotifyChannel`。
  - ❌ 需要自行實作 Tekton 原生支援的功能（workspace、artifact、log persistence、retry），工時約 8 週（M13a+M13b）。
  - ❌ 缺乏 Tekton Hub 生態；使用者要自己維護 Step image。
  - 🔓 **Escape hatch：** §6 明列「若需複雜 DAG 或既有 Tekton 資產，Synapse 支援以 Tekton 外掛方式執行」——保留未來整合空間。
- **追蹤：** `internal/services/pipeline/executor.go`、M13a 里程碑。

---

### ADR-002：M13 拆分為 M13a（核心）+ M13b（進階）

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - v1.0 將 M13 設為單一 8 週里程碑，風險集中在末段整合測試。
  - Pipeline 引擎實際包含兩層：**引擎核心**（Job 生成、DAG、排程）與 **使用者介面**（前端頁面、YAML editor、log 串流 UX）。這兩層可以獨立交付。
- **Decision：**
  - **M13a（4 週）**：核心執行引擎 + 最小 API + CLI 觸發 + 基本日誌。完成後團隊內部即可透過 API 跑出第一條 Pipeline。
  - **M13b（4 週）**：前端頁面、YAML Editor、即時日誌 SSE、Artifact UI、進階 Step types。
  - 每 2 週內部 demo 一次，降低「悶頭寫 8 週、最後一刻整合爆炸」風險。
- **Alternatives considered：**
  - 保持 8 週單一里程碑：風險集中、無法早期驗證。
  - 拆成 3 段（3+3+2 週）：切分過細、交付摩擦大於好處。
- **Consequences：**
  - ✅ M13a 結束即可供 QA 與 early adopter 試用，回饋早期導入設計。
  - ✅ M13b 可依 M13a 的實測數據調整優先級（例如延後 YAML Editor、先做 Log SSE）。
  - ❌ 需要維護兩個里程碑的 Exit Criteria 與驗收清單。
- **追蹤：** PLANNING.md §7、本文件 §8、§9、§20。

---

### ADR-003：Promotion 擴充既有 `ApprovalRequest`，不建新表

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - `internal/models/approval.go` 已有 `ApprovalRequest` 模型，支援 scale/delete/update/apply 等敏感操作的人工審批。
  - M17 Promotion（dev→staging→prod）本質上也是「敏感操作的人工審批」，與現有模型高度重疊。
  - 若另造一張 `promotion_approvals` 表，等於維護兩套審核狀態機、兩份通知邏輯、兩個前端頁面。
- **Decision：**
  - 在 `ApprovalRequest.Action` 中新增類型：`promote_environment`、`production_gate`。
  - 新增可選欄位 `PipelineRunID uint` + `TargetEnvironment string`，以向後相容方式加欄位。
  - 審核流程、通知、稽核日誌全部走既有 `ApprovalService`。
  - 前端「待審核」列表合併顯示，不拆兩個 Tab。
- **Alternatives considered：**
  | 方案 | 否決原因 |
  |------|---------|
  | 建 `promotion_approvals` 新表 | 重複邏輯、兩套通知、兩個前端頁 |
  | 用 Generic `audit_logs` | 無狀態機、無法查「待我審批」 |
  | 每次 promotion 走 `ApprovalRequest` + `PipelineRun.current_approval_id` | 正是本 Decision 採用的方案 |
- **Consequences：**
  - ✅ 單一審核中心：使用者看一個 Tab 就能處理所有待審。
  - ✅ 復用既有通知路由（webhook/telegram/slack/teams）。
  - ❌ `approval_requests` 欄位稍微擴張；需要一次 schema migration。
  - ❌ 舊有 scale/delete 審批的 UI 需同時支援新 Action 類型；渲染邏輯要加 switch。
- **追蹤：** `internal/models/approval.go`、§13.2。

---

### ADR-004：Pipeline 版本以 `pipeline_versions` 快照表管理

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - v1.0 只有 `pipelines.steps_json` 一個欄位，會被後續編輯覆蓋。
  - 問題：歷史 Run 無法重現「當時的 YAML 長什麼樣」；rollback、稽核、比對 diff 都無法做。
- **Decision：**
  - 新增 `pipeline_versions` 表：每次 Pipeline 更新建立一份新版快照（`version`、`yaml_raw`、`steps_json`、`author_id`、`commit_msg`）。
  - `pipelines` 保留 `current_version_id` 指向最新版本。
  - `pipeline_runs` 新增 `snapshot_id uint FK` 指向執行當下的版本，不讀 `pipelines.steps_json`。
- **Alternatives considered：**
  - 用 Git 存 Pipeline YAML（推到 cluster repo）：增加 Git dependency、使用者需管理 repo。
  - 直接在 `pipeline_runs` 塞一份 `steps_json`：每筆 Run 都複製一整份，浪費儲存且無法 dedupe。
  - 用 `gorm` soft delete 保留舊版：無法區分「編輯 5 次」vs 「刪除 1 次」。
- **Consequences：**
  - ✅ 任意歷史 Run 都能 100% 重現。
  - ✅ `pipeline_versions` 可按 hash dedupe（未來優化）。
  - ❌ 多一張表 + 多一層 JOIN；查詢 Run 詳情時要 `LEFT JOIN pipeline_versions`。
  - ❌ schema migration 需要把現有 `pipelines.steps_json` 灌入 `pipeline_versions`（見 §24.1）。
- **追蹤：** §7.2、§18、§24.1。

---

### ADR-005：Trivy 採「host exec + K8s Job」雙軌過渡

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - 現況：`internal/services/trivy_service.go:96` 使用 `exec.CommandContext(ctx, "trivy", "image", ...)`，依賴 Synapse 執行主機本地安裝 trivy。
  - v1.0 CICD 架構文件寫「以 K8s Job 執行 Trivy」，與現況矛盾。
  - 若直接切換到 K8s Job，既有 GitLab CI 推掃描結果的流程會斷掉。
- **Decision：**
  - **Phase 1（立即）**：保留 host exec，但 `image_scan_results` 加 `scan_source` 欄位標記來源（`host_exec` / `ci_push` / `informer` / `pipeline`）。
  - **Phase 2（M13b 末）**：新增 `trivy-scan` Step type，Pipeline 內的掃描走 K8s Job；host exec 仍可用於既有手動 API（backward compat）。
  - **Phase 3（Post-M13）**：host exec 標記為 deprecated，6 個月後移除；`scan_source` 最終只保留仍在使用的來源集合（預期至少包含 `pipeline`，以及過渡期仍可能存在的 `ci_push` / `informer`）。
  - **Phase 4（可選）**：集中 Trivy DB cache（讀取叢集共享 PVC），降低每次 Step 下載 DB 的時間。
- **Alternatives considered：**
  - 直接砍掉 host exec，所有掃描強制走 K8s Job：會破壞現有「手動 trigger scan」功能，使用者無感知情況下突然故障。
  - 永遠保留 host exec：Synapse 會變成「Docker image 內必須裝 trivy」，增加 release 負擔。
- **Consequences：**
  - ✅ 使用者無感知的漸進遷移；每個 Phase 都可 rollback。
  - ✅ 支援多種掃描來源並存，便於與現有 GitLab CI pipeline 共存。
  - ❌ 雙軌期間需要額外測試矩陣；Phase 1-2 期間 `trivy_service.go` 與 `pipeline step trivy-scan` 程式碼有部分重複。
- **追蹤：** §14、`internal/services/trivy_service.go`、`internal/models/security.go`。

---

### ADR-006：PipelineSecret 使用 `pkg/crypto` AES-256-GCM + Ephemeral K8s Secret

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - Pipeline 需要傳遞 Registry password、Git token、cloud credentials 等敏感資料。
  - v1.0 直接把 secret 塞進 `pipelines.steps_json`，明文存 DB，違反 `CLAUDE.md §10` 安全規則。
  - Synapse 現有 `pkg/crypto` 已提供 AES-256-GCM + KeyProvider，`Cluster.KubeconfigEnc` 已採用此機制。
- **Decision：**
  - 新增 `pipeline_secrets` 表，`value_enc` 欄位使用 `pkg/crypto` 加密，與 `cluster.kubeconfig_enc` 同 KeyProvider。
  - Pipeline 執行時：
    1. 解密 `pipeline_secrets.value_enc`。
    2. 為每個 **StepRun** 建立只包含該 Step 所需 keys 的**臨時 K8s Secret**（name = `pipeline-run-<runId>-step-<stepRunId>-secrets`），`ownerReference` 指向對應的 Job。
    3. Step Pod 以 `envFrom.secretRef` 或 `volume.secret` 掛載。
    4. Job 結束 → K8s GC 自動刪除 Secret；另外也設 TTL 保底。
  - 程式內 Log scrubber 過濾 `${{ secrets.XXX }}` 內容，避免 Log 外洩。
- **Alternatives considered：**
  | 方案 | 否決原因 |
  |------|---------|
  | 全用 K8s Secret，不存 DB | 無法跨叢集、無集中管理、無版本追蹤 |
  | 整合 HashiCorp Vault | 增加外部元件依賴，違反 ADR-001 精神 |
  | Sealed Secrets | 需安裝 controller；使用者門檻高 |
- **Consequences：**
  - ✅ 無額外元件；與 Cluster 加密機制一致，易於維護。
  - ✅ Secret 不落入 Pipeline 執行 Pod image，只在 Run 生命週期內暴露。
  - ❌ 密鑰輪替需走 `cmd/admin/rotate-key` CLI，並在 DB 端 rekey 所有 `value_enc`。
  - ❌ K8s API audit log 會看到 ephemeral Secret 的建立/刪除，需跟 SecOps 溝通。
- **追蹤：** `pkg/crypto`、§7.3、ADR-005（共用 KeyProvider）。

---

### ADR-007：Webhook 走獨立公開路由群組

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - Git Provider 的 Webhook 無法攜帶 Synapse JWT，現有 `middleware.AuthRequired` 會全部 401。
  - 直接把 webhook 路徑加入 JWT middleware 的 skip list 會讓路由配置混亂、容易忘記 middleware 順序。
- **Decision：**
  - 新增 `/api/v1/webhooks/*` 獨立 route group（`internal/router/routes_webhook.go`）。
  - 此 group 不套 `AuthRequired`；改套：
    1. `WebhookRateLimit()`（IP-based，預防洪水攻擊）
    2. `HMACVerify()`（provider-specific secret，驗證簽章）
    3. `ReplayProtection()`（LRU cache + 時間窗，防重放）
  - **禁止** 在 `/api/v1/webhooks/*` 下掛任何需要 user context 的 handler。
- **Alternatives considered：**
  - 重用 `/api/v1` 並 skip list：容易誤用；審計難。
  - Webhook 走獨立 port：部署複雜度上升，使用者需配防火牆。
  - 全部 webhook 改為 pull-based（Synapse polling）：高延遲，Git Provider 側看不出效果。
- **Consequences：**
  - ✅ 安全邊界清楚；code review 時一眼能辨認哪個 handler 是「公開 + 無 auth」。
  - ✅ 路由註冊檔獨立，減少 `router.go` 肥大問題（P1-2）。
  - ❌ 需要分別註冊兩組 middleware；router 初始化程式碼略長。
- **追蹤：** §10.1、§10.2、`internal/middleware/webhook_*.go`、`internal/router/routes_webhook.go`。

---

### ADR-008：跨叢集執行復用 `ClusterInformerManager`，不新建 informer pool

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - Pipeline Run 可能指定「在遠端叢集 X 執行 Step」，Synapse 需要在該叢集建立 Job 並 watch 狀態。
  - v1.0 沒有描述這個路徑。若另建一套「Pipeline Informer Pool」，會有兩套 cluster→client 生命週期管理，互相爭用資源。
- **Decision：**
  - Pipeline Executor 透過 `k8sMgr.GetK8sClient(cluster)` 取得 client（與現有 Deployment handler 同路徑）。
  - Job 的 watch 走 `k8sMgr.JobsLister(clusterID)`（若已有 informer）或 fallback 到 `Watch API` direct call。
  - 所有 Pipeline 相關的 client 取得必須經由 `ClusterInformerManager`，禁止自建 `rest.Config`。
- **Alternatives considered：**
  - 為 Pipeline 另建一套 kubeconfig 池：與 `ClusterInformerManager` 職責重疊、記憶體與 fd 浪費。
  - 走 SSH tunnel：增加部署複雜度、網路延遲。
- **Consequences：**
  - ✅ Cluster 連線生命週期、health check、GC 完全共用。
  - ✅ Pipeline 繼承 Cluster RBAC、TLS 設定、Proxy 設定。
  - ❌ 若某叢集 informer lag，Pipeline watch 也會受影響；需監控 `informer_cache_ready` metric。
- **追蹤：** `internal/k8s/cluster_informer_manager.go`、§7.5。

---

### ADR-009：Log 持久化採用「分塊落 DB，可選對接物件儲存」雙層

- **狀態：** Accepted（2026-04-09）
- **Context：**
  - Pipeline Log 需求：即時 SSE 串流 + 歷史查詢 + 失敗分析。
  - 若把完整 log 塞成單一大欄位，很快會膨脹到 100GB 級，查詢與清理都困難。
  - 若只走物件儲存（S3 / MinIO），Synapse 需強制依賴外部儲存，違反零元件精神。
- **Decision：**
  - **Tier 1（DB）**：以 `pipeline_logs` 表分塊持久化完整 log；每塊最大 1 MB，供 SSE 補讀與歷史查詢。
  - **Tier 2（optional）**：當 DB 體積或保留期壓力升高時，可把舊 chunk offload 到 local PVC / S3-compatible backend，DB 保留 metadata / reference，API 不變。
  - 讀取時預設先查 `pipeline_logs`；若 chunk 已 offload，再透過 `LogStore` 透明讀取外部儲存。
- **Alternatives considered：**
  - 全量存單欄位 DB：DB 體積爆炸、備份時間失控。
  - 全量走 S3 + 強制依賴：違反零元件。
  - 只串流不持久化：使用者關掉頁面就看不到；違反「戰情室」原則。
- **Consequences：**
  - ✅ M13a 可先只靠 DB 分塊落地，不引入額外基礎設施。
  - ✅ 以 chunk 為單位清理與重播，較容易處理 SSE 補發與分頁查詢。
  - ❌ Log 查詢有兩個路徑，需要抽象一層 `LogStore interface`。
  - ❌ 若長期完全不啟用 offload，DB 容量壓力仍會上升，需搭配 retention 與告警。
- **追蹤：** §7.11、`internal/services/pipeline/logstore/`。

---

### ADR-010：灰度發布採 Argo Rollouts 代理模式，不自行實作灰度邏輯

- **狀態：** Accepted（2026-04-14）
- **Context：**
  - 生產集群方案已採用 Argo Rollouts v1.8.4 + Gateway API trafficRouting 插件，構建灰度發布體系。
  - Synapse M16 原生 GitOps 設計了輕量 diff + sync，但沒有灰度能力。
  - 自行實作灰度邏輯（權重調整、Prometheus 分析判定、自動回滾）工作量約 8 週，且質量難以匹敵成熟方案。
  - 灰度發布涉及流量切分、指標分析、自動回滾三個高風險子系統，任一出錯等於生產事故。
- **Decision：**
  - Synapse 不自行實作灰度發布邏輯。
  - 透過 `k8s.io/client-go/dynamic` 動態客戶端操作 `argoproj.io/v1alpha1` Rollout CRD，提供：
    1. **觸發**：`deploy-rollout` Step 更新 Rollout image，由 Rollouts Controller 接管灰度流程
    2. **監控**：Watch Rollout 狀態，展示灰度進度（當前權重、步驟、分析結果）
    3. **操作**：Promote（全量）/ Abort（回滾）/ Retry（重試灰度）
  - 灰度策略（canary steps、AnalysisTemplate）由使用者在 Rollout YAML 中定義，Synapse 不介入策略設計。
  - Gateway API 的 HTTPRoute 權重更新由 Argo Rollouts 的 Gateway API 插件自動處理，Synapse 只讀取展示。
- **Alternatives considered：**
  | 方案 | 否決原因 |
  |------|---------|
  | 自行實作灰度（直接操作 Gateway API 權重 + Prometheus 分析） | 工時 ~8W；維護負擔大；生產風險高 |
  | 不支持灰度（只做 kubectl apply Deployment） | 不符合生產集群方案需求，無法利用已部署的 Argo Rollouts |
  | 整合 Flagger | 與 Argo Rollouts 功能重疊；生產方案已選定 Rollouts，不引入競爭方案 |
  | 深度整合 Argo Rollouts Go SDK（`argoproj/argo-rollouts/pkg/client`） | 增加外部 Go module 依賴；動態客戶端已足夠，且更靈活 |
- **Consequences：**
  - ✅ 復用成熟的灰度引擎，品質有保障，工時僅 2 週（M13c）。
  - ✅ 與生產集群方案完全對齊：同一套 Rollouts + Gateway API trafficRouting。
  - ✅ 符合 ADR-001 精神：Argo Rollouts 由使用者環境提供，Synapse 不額外安裝。
  - ❌ 需要目標叢集安裝 Argo Rollouts CRD；未安裝時 Rollouts 功能不可用（Observer Pattern 優雅降級）。
  - ❌ 使用者需自行維護 AnalysisTemplate 和 Rollout YAML；Synapse 不提供灰度策略 wizard。
- **追蹤：** §9.4、§12.1、`internal/handlers/rollout.go`、`internal/services/rollout_service.go`。

---

## 22. 效能 SLA 與容量規劃

本節定義 Pipeline 子系統的**效能基準線**與**容量觸發點**，作為 M13a GA 時的壓測驗收依據，以及運維監控的告警閾值。

### 22.1 效能目標（M13a GA 驗收線）

| 指標 | P50 | P95 | P99 | 備註 |
|------|-----|-----|-----|------|
| Webhook 接收 → `pipeline_runs` 寫入 | 100ms | 500ms | 1s | §10.3 流程，含 HMAC 驗證 |
| `pipeline_runs` 建立 → 首個 Job Pod `Running` | 3s | 10s | 20s | 依 image pull 狀況 |
| Step Log 產生 → SSE 推送到前端 | 500ms | 2s | 5s | §7.11 SSE 延遲 |
| Pipeline 狀態查詢 API（`GET /runs/:id`） | 50ms | 200ms | 500ms | 含 DB + 最近 log chunk / step 狀態 |
| 前端 Run 列表頁首屏載入 | 300ms | 1s | 2s | 100 筆分頁 |
| Pipeline YAML 驗證（`POST /pipelines/validate`） | 50ms | 150ms | 300ms | JSON Schema 驗證 |
| Concurrency Group 衝突判定 | 20ms | 100ms | 200ms | DB + LRU cache |

### 22.2 單 Pipeline 執行限制（預設值，可於 YAML 覆蓋）

| 項目 | 預設值 | 可調範圍 | 理由 |
|------|--------|---------|------|
| 單 Step timeout | 30min | 1min–2h | 多數 build/test/deploy 30min 內完成 |
| 整 Pipeline timeout | 2h | 10min–8h | 超時多半代表卡住或死循環 |
| 單 Step CPU request / limit | 500m / 2 | 100m–8 | 避免 Step 搶爆節點 |
| 單 Step Memory request / limit | 1Gi / 4Gi | 256Mi–16Gi | Kaniko build 通常 ≥ 2Gi |
| 單 Step Log 大小上限 | 100 MB | 1–500 MB | 超過截斷，避免壓垮 DB 與 SSE |
| 單 Pipeline Step 數量上限 | 50 | 1–200 | DAG 節點過多代表該重新拆 Pipeline |
| Workspace（emptyDir）大小 | 4 Gi | 100Mi–20Gi | 避免撐爆 Node |
| Artifact 單項大小 | 1 Gi | 10Mi–10Gi | 大檔應推 Registry 不走 Artifact |

### 22.3 並發吞吐目標

| 場景 | 目標 | 壓力測試方法 |
|------|-----|-------------|
| 單叢集並發 Running pipelines | ≥ 20 | `hey` 工具連發 50 triggers |
| 單 Synapse 實例總並發 pipelines（跨叢集） | ≥ 100 | 模擬 5 叢集各 20 |
| Webhook 峰值 QPS | ≥ 50/s | 符合 GitLab group 50 倉庫同時 push |
| 前端同時連 Log SSE 連線數 | ≥ 50 | 多使用者同時看不同 Run |

### 22.4 容量基準（中型團隊：100 runs/day × 10 steps）

| 資源 | 日增量 | 1 年累積 | 3 年累積 | 觸發擴展點 |
|------|-------|---------|---------|-----------|
| `pipeline_runs` | 100 列 | 36.5 萬 | 110 萬 | 300 萬列後考慮分區 |
| `pipeline_step_runs` | 1,000 列 | 365 萬 | 1,100 萬 | 1,000 萬列後考慮歸檔 |
| `pipeline_logs.content`（分塊後 avg 100 KB / step） | 100 MB | 36.5 GB | 110 GB | DB 表 > 50 GB 啟用物件儲存 offload |
| 完整 Log（gzip 後 ~500 KB avg） | 500 MB | 180 GB | 540 GB | > 200 GB 強制啟用 S3 backend |
| `pipeline_artifacts` metadata | 500 列 | 18 萬 | 55 萬 | 無需特別處理 |
| Artifact 實體（本機 PVC 或 S3） | 10 GB | TTL 7 天 = 70 GB | TTL 7 天 = 70 GB | PVC > 500 GB 強制切 S3 |
| `image_scan_results` | 100 列 | 3.6 萬 | 11 萬 | 無需特別處理 |

### 22.5 瓶頸預警與擴展路徑

| 瓶頸 | 早期訊號 | 短期緩解 | 長期方案 |
|------|---------|---------|---------|
| 單機 Rate Limiter（P1-8） | 多 Synapse 實例部署時 limiter 各自為政 | 臨時 sticky session | 切 Redis backend（`github.com/ulule/limiter/v3/drivers/store/redis`） |
| DB `pipeline_runs` 寫入熱點 | INSERT P95 > 500ms | 加 `(cluster_id, created_at)` 複合索引 | 分區（PostgreSQL partition by range） |
| DB `pipeline_step_runs` 膨脹 | 表 > 1000 萬列 | 歸檔 6 個月前資料到 `*_archive` 表 | 改 ClickHouse/TimescaleDB |
| Log 儲存滿 | PVC > 80% | 手動清 GC | `LogStore` 切 S3 |
| Informer lag（cross-cluster） | `informer_last_sync_age > 30s` | 重啟 informer | 調高 Informer `resyncPeriod`、增加 watch buffer |
| Concurrency Group LRU miss rate 高 | `concurrency_lru_hit_rate < 80%` | 調大 LRU size | 切 Redis pub/sub |

### 22.6 觀測要點（對應 §16 Metrics）

**SLI（Service Level Indicator，必須有 Prometheus 指標）：**
```
pipeline_run_duration_seconds{cluster, result}            histogram
pipeline_run_queue_wait_seconds{cluster}                  histogram
pipeline_step_duration_seconds{step_type, result}         histogram
webhook_ingest_duration_seconds{provider}                 histogram
pipeline_concurrent_running{cluster}                      gauge
log_sse_active_connections                                gauge
log_store_tier{tier}                                      counter  # db / local / s3
```

**SLO 建議（M13a GA 時）：**
```
SLO-1: P95(pipeline_run_queue_wait_seconds) < 10s   （30 日滾動）
SLO-2: 99% 的 pipeline_runs 能成功進入 Running 狀態
SLO-3: Webhook 接收 5xx 率 < 0.1%
SLO-4: Log SSE 斷線率 < 1%
```

**告警閾值（M13a GA 後觀察 2 週再啟用）：**
- `rate(pipeline_run_failed_total[5m]) > 0.3` → 短期 failing run 暴增
- `histogram_quantile(0.95, pipeline_run_queue_wait_seconds) > 30` → 排程壅塞
- `pipeline_concurrent_running > 80% × max_concurrent` → 接近容量上限

---

## 23. 安全威脅建模（STRIDE-lite）

採用 STRIDE 框架的精簡版：對每個敏感元件列出 **Spoofing / Tampering / Repudiation / Information disclosure / Denial of service / Elevation of privilege** 的風險與緩解措施。本節為 Pipeline 子系統的最小威脅模型，**進入生產前必須 review 一次**。

### 23.1 Webhook 接收端點（`/api/v1/webhooks/*`）

| STRIDE | 風險情境 | 緩解（設計 / 實作位置） |
|--------|---------|------------------------|
| **S**poofing | 攻擊者偽造 Git push 事件觸發 Pipeline | HMAC-SHA256 簽章驗證（provider 各自的 secret），`middleware.HMACVerify()` |
| **T**ampering | 中間人修改 payload（如 branch 名） | HTTPS only（TLS 於 LB 層）+ HMAC 簽章覆蓋整個 payload |
| **R**epudiation | 攻擊者否認觸發 | 每次 webhook 寫入 `audit_logs` + `pipeline_runs.trigger_payload_hash` |
| **I**nformation disclosure | Webhook 回應洩漏 Pipeline 配置 | 固定回 `{"received": true}`，不回 payload 細節 |
| **D**enial of service | 洪水攻擊、惡意 replay | 三層防禦：IP Rate Limit（100 req/min）+ LRU replay cache（5 min）+ `max_concurrent_runs` |
| **E**levation of privilege | Webhook 繞過 JWT 觸發敏感操作 | Webhook group 內**禁止**掛需要 user context 的 handler；觸發的 Pipeline 以 `pipeline.service_account` 而非觸發者身份執行 |

### 23.2 Pipeline Pod（Step 執行容器）

| STRIDE | 風險情境 | 緩解 |
|--------|---------|------|
| **S** | 惡意 YAML 指定他人叢集的 ServiceAccount | `pipeline_run.cluster_id` 與 `pipeline.cluster_id` 必須一致；跨叢集執行需明確在 YAML 宣告且經過 RBAC 檢查 |
| **T** | Step 修改 Node 系統檔案 | `readOnlyRootFilesystem: true` + `allowPrivilegeEscalation: false` + `runAsNonRoot: true`（§7.7） |
| **T** | Step 篡改其他 workload 的 Secret/ConfigMap | `automountServiceAccountToken: false` 預設；如需 K8s API 權限，必須在 `pipeline.runtime.service_account` 白名單中明示 |
| **R** | Step 結束後無法追溯做了什麼 | `kubectl exec` 禁止；所有 step 的 `command` 保留在 `pipeline_step_runs.command_snapshot` |
| **I** | Step 日誌夾帶 Secret 明文 | Log scrubber 過濾 `${{ secrets.XXX }}` 實際值（§7.11）；Secret 只以 env 注入，不寫 file（可設定） |
| **I** | Step 連到外部服務洩漏內部 IP/hostname | NetworkPolicy 預設只允許出口到 `allow_egress` 列表；無列表則拒絕外部連線 |
| **D** | 惡意 Step 佔滿叢集資源 | `resources.limits` 強制（無 limit 則套預設 1Gi / 500m）+ `max_concurrent_runs` |
| **D** | 無限 fork pod | Pod-level `pids.max` cgroup（若 Node 支援） |
| **E** | Step 打破 sandbox 取得 Node root | Seccomp `RuntimeDefault` + `runAsNonRoot` + Kaniko 僅限 namespace 白名單 + 敏感叢集可搭 Node taint |
| **E** | Step 讀取其他 Pipeline Run 的檔案 | emptyDir Workspace per-run；Artifact 讀寫有 `pipeline_id + run_id` 檢核 |

### 23.3 Secrets 管理（`pipeline_secrets` + Ephemeral K8s Secret）

| STRIDE | 風險情境 | 緩解 |
|--------|---------|------|
| **S** | 非授權使用者註冊同名 Secret 覆蓋他人 | Secret name 需在 `cluster + namespace` 內唯一；寫入時 RBAC 檢查 |
| **T** | DB 直連寫入明文 value | `value_enc` 欄位強制走 GORM BeforeSave hook + `pkg/crypto`；`value` 欄位不存在 |
| **T** | 加密 Key 被換掉導致 rekey | `KeyProvider` 版本化，`value_enc` 含 key_version prefix |
| **R** | Secret 被誰使用無記錄 | 每次 Pipeline Run 啟動時寫 `audit_logs: secret_referenced` 事件（只記 name，不記 value） |
| **I** | Log 列 env 值 | §7.11 Log scrubber 過濾 |
| **I** | `kubectl get secret -o yaml` 讀到 ephemeral Secret | 該 Secret 綁定 ownerRef → Job 結束即刪；且 RBAC 限定 Pipeline Controller ServiceAccount |
| **D** | 大量 secret 觸發 etcd 壓力 | Ephemeral Secret 只在 Run 生命週期內存在，不超過 50 個 concurrent |
| **E** | Step 透過 mount Secret 取得所有 Pipeline Secret | Step 只能 mount 該 Run YAML 宣告的 secret name；Pipeline Executor 嚴格 allowlist |

### 23.4 GitOps Reconciler（M16）

| STRIDE | 風險情境 | 緩解 |
|--------|---------|------|
| **S** | 假冒 `gitops_apps.source` 指向惡意 repo | Repo URL 需經 Platform Admin 審核；Git clone 使用 readonly deploy key |
| **T** | Reconciler 誤 apply 錯誤 manifest | Dry-run diff 先顯示，使用者確認後 apply（預設）；自動模式需白名單 |
| **T** | 惡意 PR 合併後自動部署 | 保留 manual approval（`sync.policy = manual`）；自動模式需搭配 `production_gate` ADR-003 |
| **R** | 無 sync 歷史 | 每次 sync 寫 `gitops_sync_events` 表 |
| **I** | `git clone` 曝光 token | 使用 deploy key；token 不存 DB 明文 |
| **D** | Reconciler 對 Git server 壓力 | Poll interval 最小 30s；ETag cache 優先 |
| **E** | `kubectl apply` 權限過大 | Reconciler ServiceAccount 僅綁定目標 namespace 的 `edit` Role，禁用 `cluster-admin` |

### 23.5 Webhook Replay Protection 細節

```go
// internal/middleware/webhook_replay.go
type ReplayGuard struct {
    seen *lru.Cache[string, time.Time]  // key = provider+delivery_id
    ttl  time.Duration                  // 5min
}

func (g *ReplayGuard) Check(provider, deliveryID string, sentAt time.Time) error {
    // 1. Reject if timestamp drift > 5min（防 old replay）
    if time.Since(sentAt) > g.ttl {
        return ErrStaleWebhook
    }
    // 2. Reject if already seen
    key := provider + ":" + deliveryID
    if _, ok := g.seen.Get(key); ok {
        return ErrReplayDetected
    }
    g.seen.Add(key, time.Now())
    return nil
}
```

**為何 LRU + TTL 雙層？**
- LRU 防記憶體爆炸（size=10000）。
- TTL 防「正當的 retry webhook」被誤認為 replay（Git 通常 1min 內 retry 3 次）。

### 23.6 資料外洩緩解清單

| 外洩來源 | 緩解 |
|---------|------|
| Log 輸出 | Log scrubber（§7.11）、`pipeline_logs.content` 落 DB 前再 filter 一次 |
| JSON API 回應 | `Secret.Value` 欄位永遠 `json:"-"`；前端只拿 metadata |
| K8s Secret dump | ephemeral secret + ownerRef + RBAC；`kubectl get secret` 需要 `list` verb |
| audit log | `operation_logs.payload` 存之前過濾 `password|token|secret` 鍵 |
| 錯誤訊息 | apierrors 統一 wrap，避免把 `stderr` 原文回傳給使用者 |

---

## 24. 資料遷移腳本示意

本節列出 M13a/M13b/M14 實作時所需的 GORM AutoMigrate + 手動 SQL 範例。遵循 Synapse 現有 `internal/database/migration.go` 模式，**新增欄位優先 AutoMigrate**；既有欄位轉換、資料填充等無法由 GORM 表達的，走 `migration_scripts/` 手動 SQL。

### 24.1 `pipelines.steps_json` → `pipeline_versions` 拆分

**背景：** 現有 `pipelines.steps_json` 只存最新版，遷移後需為歷史 Pipeline 補建第一版快照。

```go
// internal/database/migrations/m13a_pipeline_versions.go
package migrations

import (
    "fmt"

    "gorm.io/gorm"

    "github.com/shaia/Synapse/internal/models"
    "github.com/shaia/Synapse/pkg/logger"
)

// M13aPipelineVersions 建立 pipeline_versions 表並從 pipelines.steps_json 灌入初始版本。
func M13aPipelineVersions(db *gorm.DB) error {
    // ── Step 1: AutoMigrate 新表 ──────────────────────────────────────────
    if err := db.AutoMigrate(&models.PipelineVersion{}); err != nil {
        return fmt.Errorf("auto migrate pipeline_versions: %w", err)
    }

    // ── Step 2: 為每個已存在的 pipeline 建立 version=1 快照 ───────────────
    var pipelines []models.Pipeline
    if err := db.Find(&pipelines).Error; err != nil {
        return fmt.Errorf("fetch existing pipelines: %w", err)
    }

    for _, p := range pipelines {
        // 跳過已有版本的 pipeline（冪等性）
        var count int64
        db.Model(&models.PipelineVersion{}).Where("pipeline_id = ?", p.ID).Count(&count)
        if count > 0 {
            continue
        }

        v := &models.PipelineVersion{
            PipelineID: p.ID,
            Version:    1,
            StepsJSON:  p.StepsJSON, // 舊欄位暫保留，遷移完成後 drop
            YAMLRaw:    "",          // 無原始 YAML 可還原
            AuthorID:   p.CreatedBy, // 若有 created_by，否則為 0
            CommitMsg:  "initial version (migrated from steps_json)",
            CreatedAt:  p.CreatedAt,
        }
        if err := db.Create(v).Error; err != nil {
            return fmt.Errorf("create initial version for pipeline %d: %w", p.ID, err)
        }

        // 指向最新版本
        if err := db.Model(&p).Update("current_version_id", v.ID).Error; err != nil {
            return fmt.Errorf("update current_version_id for pipeline %d: %w", p.ID, err)
        }

        logger.Info("pipeline migrated to versioned storage",
            "pipeline_id", p.ID,
            "version_id", v.ID,
        )
    }

    // ── Step 3: 為 pipeline_runs 補 snapshot_id ───────────────────────────
    //   所有舊 Run 指向 version=1（最接近舊行為的假設）
    if err := db.Exec(`
        UPDATE pipeline_runs r
        SET snapshot_id = (
            SELECT v.id FROM pipeline_versions v
            WHERE v.pipeline_id = r.pipeline_id AND v.version = 1
        )
        WHERE r.snapshot_id IS NULL OR r.snapshot_id = 0
    `).Error; err != nil {
        return fmt.Errorf("backfill pipeline_runs.snapshot_id: %w", err)
    }

    return nil
}
```

**Rollback 策略：**
- `pipelines.steps_json` 欄位保留到 M13b 結束，期間為雙寫（寫新版 + 同步舊欄位）。
- M13b 結束 + 監控穩定 2 週後，才執行 `ALTER TABLE pipelines DROP COLUMN steps_json`。
- 風險：rollback 到 M13a 前版本時，`steps_json` 需要從 `pipeline_versions.current_version_id` 反向同步；提供 `cmd/admin/migrate-downgrade` 子命令。

### 24.2 `image_scan_results` 新增 `scan_source` 欄位

**背景：** ADR-005 要求標記掃描來源，區分 host exec / GitLab CI / Pipeline Step。

```go
// internal/database/migrations/m13b_scan_source.go
func M13bScanSource(db *gorm.DB) error {
    // ── Step 1: AutoMigrate 加欄位 ────────────────────────────────────────
    if err := db.AutoMigrate(&models.ImageScanResult{}); err != nil {
        return fmt.Errorf("auto migrate image_scan_results: %w", err)
    }

    // ── Step 2: 填充歷史資料 ──────────────────────────────────────────────
    //   所有舊記錄都是 host exec 來源（ADR-005 Phase 1 前的唯一路徑）
    if err := db.Exec(`
        UPDATE image_scan_results
        SET scan_source = 'host_exec'
        WHERE scan_source IS NULL OR scan_source = ''
    `).Error; err != nil {
        return fmt.Errorf("backfill scan_source: %w", err)
    }

    // ── Step 3: 加非 null 約束（PostgreSQL） ──────────────────────────────
    if err := db.Exec(`
        ALTER TABLE image_scan_results
        ALTER COLUMN scan_source SET DEFAULT 'host_exec',
        ALTER COLUMN scan_source SET NOT NULL
    `).Error; err != nil {
        // SQLite 不支援 ALTER COLUMN SET NOT NULL；用日誌警告，不拋錯
        logger.Warn("SET NOT NULL skipped (likely SQLite)", "error", err)
    }

    return nil
}
```

### 24.3 `approval_requests` 擴充 Action 類型

**背景：** ADR-003 將 `promote_environment` 與 `production_gate` 加入既有 `ApprovalRequest.Action` 枚舉。

```go
// internal/database/migrations/m17_promotion_approval.go
func M17PromotionApproval(db *gorm.DB) error {
    // AutoMigrate 新欄位（PipelineRunID, TargetEnvironment）
    if err := db.AutoMigrate(&models.ApprovalRequest{}); err != nil {
        return fmt.Errorf("auto migrate approval_requests: %w", err)
    }

    // CHECK constraint（PostgreSQL）— 記錄合法值
    if err := db.Exec(`
        DO $$
        BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM information_schema.check_constraints
                WHERE constraint_name = 'approval_requests_action_check'
            ) THEN
                ALTER TABLE approval_requests
                ADD CONSTRAINT approval_requests_action_check
                CHECK (action IN (
                    'scale', 'delete', 'update', 'apply',
                    'promote_environment', 'production_gate'
                ));
            END IF;
        END $$;
    `).Error; err != nil {
        logger.Warn("approval_requests_action_check skipped", "error", err)
    }

    return nil
}
```

### 24.4 Webhook Replay Cache 表（可選，分散式場景才需要）

**背景：** 單機 LRU 在多 Synapse 實例部署時無法共享；若需跨實例 replay 防護，可落 DB（輕量、不用 Redis）。

```sql
-- 可選：當 Synapse 多實例部署時啟用
CREATE TABLE IF NOT EXISTS webhook_delivery_dedupe (
    provider     VARCHAR(50)  NOT NULL,
    delivery_id  VARCHAR(200) NOT NULL,
    received_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (provider, delivery_id)
);

-- 定期清理（5min TTL）
DELETE FROM webhook_delivery_dedupe
WHERE received_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes';
```

GORM 對應由背景 worker 清理，避免 SQL dialect 不相容：

```go
// internal/services/webhook_dedupe_worker.go
func (w *WebhookDedupeWorker) cleanup(ctx context.Context) {
    cutoff := time.Now().Add(-5 * time.Minute)
    w.db.WithContext(ctx).
        Where("received_at < ?", cutoff).
        Delete(&models.WebhookDeliveryDedupe{})
}
```

### 24.5 遷移執行與回滾 SOP

**執行順序（M13a → M17）：**
```go
// internal/database/migration.go
func Migrate(db *gorm.DB) error {
    migrations := []func(*gorm.DB) error{
        migrations.M13aPipelineVersions,
        migrations.M13bScanSource,
        migrations.M14WebhookModel,          // GitProvider/Webhook 模型
        migrations.M15RegistryModel,         // registry_credentials
        migrations.M16GitOpsModel,           // gitops_apps
        migrations.M17PromotionApproval,
    }
    for _, m := range migrations {
        if err := m(db); err != nil {
            return err
        }
    }
    return nil
}
```

**冪等性要求：**
每個 migration function 必須可重複執行而不損壞資料：
- AutoMigrate 本身冪等。
- 資料填充要先 `SELECT COUNT` 或 `WHERE ... IS NULL` 判斷。
- CHECK constraint 加入前先查 `information_schema`。

**Rollback 機制：**
- 不提供 `Down()` migration（Go-Migrate 風格會增加維護負擔）。
- 改以「前向相容 + 版本化欄位」方式：新欄位默認 nullable，舊程式可忽略；舊欄位保留兩個 release 週期再 drop。
- 緊急 rollback：DB 層 restore from snapshot + 降級 Synapse binary。

**測試策略：**
- `internal/database/migration_test.go` 需覆蓋：
  - 從空 DB 執行完整 migration → 成功
  - 重複執行同一 migration → 冪等
  - 從 v1.0 schema（含舊 `pipelines.steps_json`）遷移 → 資料正確
- CI 在 PR 階段跑 migration test，禁止合併未驗證的 schema 變更。

---

## 附錄 A：Pipeline YAML Schema


```yaml
# synapse-pipeline.yaml
apiVersion: synapse.io/v1
kind: Pipeline
metadata:
  name: backend-service
  cluster: prod-cluster
  namespace: app-dev
spec:
  description: "Backend service CI/CD pipeline"

  concurrency:
    group: "${PIPELINE_NAME}-${GIT_BRANCH}"
    policy: cancel_previous      # cancel_previous | queue | reject

  max_concurrent_runs: 1

  runtime:
    service_account: pipeline-runner
    pod_security:
      run_as_non_root: true
      read_only_root_fs: true    # Kaniko Step 需要例外
    network_policy:
      allow_egress:
        - "gitlab.company.com"
        - "harbor.company.com"

  env:                           # 預設環境變數（全域）
    BUILD_PROFILE: prod

  triggers:
    - type: webhook
      provider: gitlab-main
      repo: "company/backend-service"
      branch: "main"
      events: [push, merge_request]
      path_filter:
        - "src/**"
        - "Dockerfile"

    - type: schedule
      cron: "0 2 * * *"

  workspace:
    type: pvc                    # 跨 Step 共享檔案必須使用 pvc；emptyDir 僅限單 Step scratch
    size: 4Gi
    retention_hours: 0

  steps:
    - name: build-jar
      type: build-jar
      image: maven:3.9-eclipse-temurin-17
      depends_on: []
      env:
        MAVEN_OPTS: "-Xmx1g"
      cache:
        paths:
          - /root/.m2/repository
      command: |
        mvn clean package -DskipTests
      outputs:
        - name: jar-file
          path: target/*.jar

    - name: build-image
      type: build-image
      image: gcr.io/kaniko-project/executor:v1.20.0
      depends_on: [build-jar]
      pod_security:
        read_only_root_fs: false   # Kaniko 例外
      env:
        DOCKER_USERNAME: ${{ secrets.HARBOR_USERNAME }}
        DOCKER_PASSWORD: ${{ secrets.HARBOR_PASSWORD }}
      config:
        context: .
        dockerfile: Dockerfile
        destination: "harbor.company.com/backend/service:${GIT_SHA}"
      outputs:
        - name: image-digest
          from: kaniko-output

    - name: trivy-scan
      type: trivy-scan
      image: aquasec/trivy:0.50.0
      depends_on: [build-image]
      config:
        target: "${{ steps.build-image.outputs.image-digest }}"
        severity_threshold: HIGH
        ignore_unfixed: true
      on_failure: abort
      retry:
        max_attempts: 2
        backoff_seconds: 30

    - name: deploy-staging
      type: deploy
      image: bitnami/kubectl:1.30
      depends_on: [trivy-scan]
      config:
        manifest: k8s/deployment.yaml
        namespace: app-staging
      env:
        IMAGE_TAG: "${GIT_SHA}"

  notifications:
    on_success:
      channels: [3]
    on_failure:
      channels: [3, 5]
    on_scan_critical:
      channels: [3, 5, 7]
```

**JSON Schema 驗證：** 完整 schema 檔案存於 `internal/services/pipeline_schema/v1.json`，後端於 Pipeline 建立/更新時驗證；前端 YAML Editor 透過 `/api/v1/pipeline/schema` 取得同一份 schema 做即時 lint。

---

## 附錄 B：與 ARCHITECTURE_REVIEW.md 對應關係

| ARCHITECTURE_REVIEW 項目 | 在本文件對應 |
|------|------|
| P0-4 handlers 直接注入 `*gorm.DB` | 本文件所有 handlers 遵循 handler → service 分層 |
| P0-5 JWT 無 revocation | Webhook 端點走公開路由群組，不涉及 JWT |
| P0-6 localStorage token | 前端 Pipeline 頁面遵循統一 auth 機制（待 P0-6 修復後自動受益）|
| P1-1 handlers 肥大 | Pipeline handlers 控制在 ~150 行以內，業務邏輯移至 service |
| P1-2 router 單檔過大 | 新增 `routes_pipeline.go` / `routes_webhook.go` 獨立檔案 |
| P1-3 service 缺 interface | PipelineExecutor / JobBuilder / JobWatcher 以 interface 定義，方便 mock 測試 |
| P1-4 無 OpenAPI | Pipeline API 加入 swaggo 註解，作為首批 OpenAPI 覆蓋目標 |
| P1-8 Rate Limiter 單機 | WebhookRateLimit 設計為接口，多實例時切 Redis backend |
| P1-10 OpenTelemetry | Pipeline 子系統為首批加入 trace 的模組 |
| P2-1 多租戶 | Pipeline RBAC 以 cluster + namespace 為邊界，為未來多租戶 Project 概念打基礎 |
| P2-2 audit hash chain | Pipeline 操作全走 OperationAudit，銜接未來 audit 強化 |
| P2-3 欄位加密 | PipelineSecret / Registry / GitProvider 全面使用 pkg/crypto AES-256-GCM |

---

## 附錄 C：真實 Pipeline 範例集

本附錄提供四類常見場景的完整 YAML，作為 M13a/M13b 實作時的驗收範本 + 使用者入門教材。每份範例皆可在本地叢集直接跑通（前提：對應 Step image 可 pull）。

### C.1 Java Spring Boot 後端（Maven build → Kaniko → Trivy → Deploy）

```yaml
apiVersion: synapse.io/v1
kind: Pipeline
metadata:
  name: user-service
  cluster: dev-cluster
  namespace: backend-ci
spec:
  description: "User service: build jar, image, scan, deploy to staging"

  concurrency:
    group: "user-service-${GIT_BRANCH}"
    policy: cancel_previous

  triggers:
    - type: webhook
      provider: gitlab-internal
      repo: "backend/user-service"
      branch: "main"
      events: [push, merge_request]
      path_filter:
        - "src/**"
        - "pom.xml"
        - "Dockerfile"

  runtime:
    service_account: pipeline-runner
    pod_security:
      run_as_non_root: true
      read_only_root_fs: true

  env:
    MAVEN_OPTS: "-Xmx2g -XX:+UseG1GC"

  workspace:
    type: pvc
    size: 6Gi

  steps:
    # 1) 檢出原始碼
    - name: checkout
      type: git-checkout
      depends_on: []
      config:
        repo: "https://gitlab.internal/backend/user-service.git"
        ref: "${GIT_SHA}"
        credentials: "gitlab-readonly"   # 對應 pipeline_secrets

    # 2) Maven 編譯 + 單元測試
    - name: build-and-test
      type: exec
      image: maven:3.9-eclipse-temurin-17
      depends_on: [checkout]
      cache:
        paths:
          - /root/.m2/repository
      command: |
        mvn clean verify -B \
          -DskipITs \
          -Dmaven.test.failure.ignore=false
      outputs:
        - name: jar-artifact
          path: target/*.jar
        - name: test-report
          path: target/surefire-reports/
      on_failure: abort

    # 3) Kaniko 建置映像（需要寫入權限例外）
    - name: build-image
      type: build-image
      image: gcr.io/kaniko-project/executor:v1.20.0
      depends_on: [build-and-test]
      pod_security:
        read_only_root_fs: false        # Kaniko 例外
      env:
        DOCKER_CONFIG: /kaniko/.docker
      config:
        context: "."
        dockerfile: "Dockerfile"
        destination: "harbor.internal/backend/user-service:${GIT_SHA}"
        build_args:
          JAR_FILE: "target/*.jar"
        registry_credentials: "harbor-ci"  # pipeline_secrets
      outputs:
        - name: image-ref
          from: kaniko-image-ref

    # 4) Trivy 掃描（在 K8s Job 內執行，非 host exec）
    - name: security-scan
      type: trivy-scan
      image: aquasec/trivy:0.50.0
      depends_on: [build-image]
      config:
        target: "${{ steps.build-image.outputs.image-ref }}"
        severity_threshold: HIGH
        ignore_unfixed: true
        exit_on_vulnerability: true
      on_failure: abort
      retry:
        max_attempts: 2
        backoff_seconds: 30

    # 5) 部署到 staging
    - name: deploy-staging
      type: deploy
      image: bitnami/kubectl:1.30
      depends_on: [security-scan]
      config:
        manifest: "k8s/staging/deployment.yaml"
        namespace: "user-svc-staging"
        wait_for_rollout: true
        rollout_timeout: 5m
      env:
        IMAGE_TAG: "${GIT_SHA}"

    # 6) Smoke test
    - name: smoke-test
      type: exec
      image: curlimages/curl:8.7.1
      depends_on: [deploy-staging]
      command: |
        for i in 1 2 3 4 5; do
          if curl -sf http://user-svc.user-svc-staging.svc:8080/health; then
            echo "smoke test passed"
            exit 0
          fi
          sleep 5
        done
        exit 1
      retry:
        max_attempts: 3
        backoff_seconds: 10

  notifications:
    on_success:
      channels: [3]     # NotifyChannel ID, 一般頻道
    on_failure:
      channels: [3, 5]  # 加值班頻道
    on_scan_critical:
      channels: [3, 5, 7]  # 加 SecOps
```

**關鍵點：**
- `build-image` step 把 `pod_security.read_only_root_fs` 設 `false`，是整份 YAML 唯一的例外，Pipeline Executor 會 audit log 記錄此例外。
- `smoke-test` 用 `retry` 取代 `sleep` loop，符合 §7.10 重試策略。
- `notifications.on_scan_critical` 為 Pipeline 特有事件，掃描出 CRITICAL 時額外發 SecOps 頻道。

---

### C.2 React + Vite 前端（pnpm build → Kaniko → CDN 同步）

```yaml
apiVersion: synapse.io/v1
kind: Pipeline
metadata:
  name: admin-portal-web
  cluster: dev-cluster
  namespace: frontend-ci
spec:
  description: "React admin portal: build, containerize, sync static assets to CDN"

  concurrency:
    group: "admin-portal-${GIT_BRANCH}"
    policy: cancel_previous

  triggers:
    - type: webhook
      provider: gitlab-internal
      repo: "frontend/admin-portal"
      branch: "main"
      events: [push]
      path_filter:
        - "src/**"
        - "public/**"
        - "package.json"
        - "pnpm-lock.yaml"

  runtime:
    service_account: pipeline-runner
    pod_security:
      run_as_non_root: true
      read_only_root_fs: true

  workspace:
    type: pvc
    size: 4Gi

  steps:
    - name: checkout
      type: git-checkout
      depends_on: []
      config:
        repo: "https://gitlab.internal/frontend/admin-portal.git"
        ref: "${GIT_SHA}"

    - name: install-deps
      type: exec
      image: node:20-alpine
      depends_on: [checkout]
      cache:
        paths:
          - /workspace/.pnpm-store
      command: |
        corepack enable
        pnpm config set store-dir /workspace/.pnpm-store
        pnpm install --frozen-lockfile

    - name: lint-and-type-check
      type: exec
      image: node:20-alpine
      depends_on: [install-deps]
      command: |
        pnpm run lint
        pnpm run type-check
      on_failure: abort

    - name: unit-test
      type: exec
      image: node:20-alpine
      depends_on: [install-deps]
      command: |
        pnpm run test:unit
      outputs:
        - name: coverage-report
          path: coverage/

    - name: build
      type: exec
      image: node:20-alpine
      depends_on: [lint-and-type-check, unit-test]
      env:
        NODE_OPTIONS: "--max-old-space-size=4096"
        VITE_API_BASE: "/api/v1"
      command: |
        pnpm run build
      outputs:
        - name: dist-bundle
          path: dist/

    - name: build-image
      type: build-image
      image: gcr.io/kaniko-project/executor:v1.20.0
      depends_on: [build]
      pod_security:
        read_only_root_fs: false
      config:
        context: "."
        dockerfile: "Dockerfile.nginx"
        destination: "harbor.internal/frontend/admin-portal:${GIT_SHA}"

    - name: sync-cdn
      type: exec
      image: amazon/aws-cli:2.15.0
      depends_on: [build-image]
      env:
        AWS_ACCESS_KEY_ID: ${{ secrets.CDN_ACCESS_KEY }}
        AWS_SECRET_ACCESS_KEY: ${{ secrets.CDN_SECRET_KEY }}
      command: |
        aws s3 sync dist/assets/ s3://cdn-assets/admin-portal/${GIT_SHA}/ \
          --cache-control "public, max-age=31536000, immutable"

  notifications:
    on_failure:
      channels: [3]
```

**關鍵點：**
- 前端 Pipeline 通常沒有 Trivy 掃描（靜態資源），但若 build-image 使用 nginx base，仍建議加 scan step。
- `pnpm` cache 走 `/workspace/.pnpm-store`，因為後續 Step 要共享安裝結果與 build 產物，所以這份 Pipeline 的 workspace 必須是 PVC。

---

### C.3 Helm Chart 發布

```yaml
apiVersion: synapse.io/v1
kind: Pipeline
metadata:
  name: platform-charts
  cluster: ops-cluster
  namespace: helm-publish
spec:
  description: "Helm chart lint, package, push to ChartMuseum"

  triggers:
    - type: webhook
      provider: gitlab-internal
      repo: "platform/helm-charts"
      branch: "main"
      events: [push]
      path_filter:
        - "charts/**"

  runtime:
    service_account: pipeline-runner

  workspace:
    type: pvc
    size: 2Gi

  steps:
    - name: checkout
      type: git-checkout
      depends_on: []
      config:
        repo: "https://gitlab.internal/platform/helm-charts.git"
        ref: "${GIT_SHA}"

    - name: detect-changed-charts
      type: exec
      image: alpine/git:2.43
      depends_on: [checkout]
      command: |
        CHANGED=$(git diff --name-only HEAD~1 HEAD -- charts/ | cut -d'/' -f2 | sort -u)
        echo "${CHANGED}" > /workspace/changed-charts.txt
      outputs:
        - name: changed-list
          path: /workspace/changed-charts.txt

    - name: helm-lint
      type: exec
      image: alpine/helm:3.14.4
      depends_on: [detect-changed-charts]
      command: |
        while read chart; do
          [ -z "${chart}" ] && continue
          helm lint "charts/${chart}"
        done < /workspace/changed-charts.txt
      on_failure: abort

    - name: helm-package
      type: exec
      image: alpine/helm:3.14.4
      depends_on: [helm-lint]
      command: |
        mkdir -p /workspace/packages
        while read chart; do
          [ -z "${chart}" ] && continue
          helm package "charts/${chart}" -d /workspace/packages
        done < /workspace/changed-charts.txt
      outputs:
        - name: helm-packages
          path: /workspace/packages/*.tgz

    - name: push-to-chartmuseum
      type: exec
      image: curlimages/curl:8.7.1
      depends_on: [helm-package]
      env:
        CM_URL: "https://charts.internal"
        CM_TOKEN: ${{ secrets.CHARTMUSEUM_TOKEN }}
      command: |
        for pkg in /workspace/packages/*.tgz; do
          curl -fsSL \
            -H "Authorization: Bearer ${CM_TOKEN}" \
            --data-binary "@${pkg}" \
            "${CM_URL}/api/charts"
        done

  notifications:
    on_success:
      channels: [3]
    on_failure:
      channels: [3, 5]
```

**關鍵點：**
- 示範 **step 間用 workspace 共享檔案**（`detect-changed-charts` → `helm-package`）。
- 示範 **artifact 輸出**（`helm-packages`），可在 Pipeline UI 下載。

---

### C.4 Terraform Infrastructure（terraform plan → 人工審核 → apply）

```yaml
apiVersion: synapse.io/v1
kind: Pipeline
metadata:
  name: prod-infra
  cluster: ops-cluster
  namespace: infra-pipeline
spec:
  description: "Terraform plan for prod infra, requires manual approval before apply"

  triggers:
    - type: webhook
      provider: gitlab-internal
      repo: "infra/terraform-prod"
      branch: "main"
      events: [push, merge_request]
      path_filter:
        - "**.tf"
        - "**.tfvars"

  runtime:
    service_account: infra-pipeline-runner
    pod_security:
      run_as_non_root: true
      read_only_root_fs: false  # Terraform 需要寫 .terraform/ 目錄

  workspace:
    type: pvc
    pvc_name: terraform-state-cache
    size: 2Gi

  steps:
    - name: checkout
      type: git-checkout
      depends_on: []
      config:
        repo: "https://gitlab.internal/infra/terraform-prod.git"
        ref: "${GIT_SHA}"

    - name: tf-init
      type: exec
      image: hashicorp/terraform:1.7
      depends_on: [checkout]
      env:
        AWS_ACCESS_KEY_ID: ${{ secrets.AWS_INFRA_ACCESS_KEY }}
        AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_INFRA_SECRET_KEY }}
      command: |
        terraform init -backend-config=backend.hcl

    - name: tf-validate
      type: exec
      image: hashicorp/terraform:1.7
      depends_on: [tf-init]
      command: |
        terraform validate
      on_failure: abort

    - name: tf-plan
      type: exec
      image: hashicorp/terraform:1.7
      depends_on: [tf-validate]
      env:
        AWS_ACCESS_KEY_ID: ${{ secrets.AWS_INFRA_ACCESS_KEY }}
        AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_INFRA_SECRET_KEY }}
      command: |
        terraform plan -out=tfplan -input=false
        terraform show -no-color tfplan > plan-summary.txt
      outputs:
        - name: tf-plan-file
          path: tfplan
        - name: plan-summary
          path: plan-summary.txt

    # ── 人工審核閘門（走 ApprovalRequest，ADR-003） ─────────────────────────
    - name: approval-gate
      type: approval
      depends_on: [tf-plan]
      config:
        action: production_gate
        message: "Terraform plan ready. Review plan-summary.txt before approving."
        required_approvers: 2     # 至少 2 人點頭
        approver_groups:
          - infra-leads
          - platform-sre
        timeout: 24h              # 24 小時未審核自動取消
      on_timeout: abort

    - name: tf-apply
      type: exec
      image: hashicorp/terraform:1.7
      depends_on: [approval-gate]
      env:
        AWS_ACCESS_KEY_ID: ${{ secrets.AWS_INFRA_ACCESS_KEY }}
        AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_INFRA_SECRET_KEY }}
      command: |
        terraform apply -input=false -auto-approve tfplan
      on_failure:
        notify: true
        # 注意：tf-apply 失敗不做自動 rollback，由 SRE 手動介入

  notifications:
    on_start:
      channels: [10]               # 基礎設施變更頻道
    on_approval_requested:
      channels: [10, 11]           # 加 infra-leads
    on_success:
      channels: [10]
    on_failure:
      channels: [10, 11, 12]       # 加 on-call
```

**關鍵點：**
- 示範 `type: approval` step，銜接 §13.2 ApprovalRequest 擴充。
- `workspace.type: pvc` 示範持久化 workspace（Terraform state cache）。
- `tf-apply.on_failure` 明確說明不自動 rollback；Pipeline 設計哲學是「失敗就停，等人」而非「任意回滾」，避免 infra 進入更壞的狀態。

---

## 附錄 D：Troubleshooting 手冊

本附錄針對 M13a/M13b 上線後**最可能遇到的 10 個問題**提供診斷路徑與程式碼入口。Runbook 風格，每個問題均含：**症狀 → 快速檢查 → 根因 → 修復 → 預防**。

### D.1 Pipeline Pod 卡在 `Pending` 一直不 Running

- **症狀：** `pipeline_runs.status = 'running'` 但 Pod `kubectl get pod -n <ns>` 顯示 `Pending`。
- **快速檢查：**
  ```bash
  kubectl describe pod -n <ns> <pod-name> | tail -30
  ```
- **常見根因：**
  1. Image pull 失敗（私有 registry 缺 imagePullSecret）
  2. Resource request 過大，Node 無法排程
  3. PVC binding 失敗（`workspace.type: pvc` 時）
  4. Node affinity / toleration 不符
- **修復：**
  - Image pull：檢查 `pipeline.runtime.image_pull_secrets`，對應的 K8s Secret 是否存在
  - Resource：調小 `resources.requests.cpu/memory`
  - PVC：確認 StorageClass 存在 + 可動態供應
- **預防：**
  - `internal/services/pipeline/executor.go` 建立 Pod 時先做 dry-run + admission webhook 預檢
  - Metric: `pipeline_pod_pending_seconds`，> 60s 告警

### D.2 Trivy Step 一直 timeout

- **症狀：** `trivy-scan` step 在 `timeout` 後被 kill，error 訊息含 `context deadline exceeded`。
- **快速檢查：** 查 Step log 最後幾行，通常會看到 `Downloading DB...`
- **根因：** Trivy 每次都從 GitHub 下載 vulnerability DB（約 300 MB），網路慢的環境會超時。
- **修復：**
  - 方案 A：調高 step timeout 到 5 min
  - 方案 B（推薦）：叢集內部署 Trivy DB mirror，設定 `TRIVY_DB_REPOSITORY` 環境變數
  - 方案 C：預先 pull `aquasec/trivy-db` 映像到 emptyDir
- **預防：**
  - §14 Phase 4 「集中 Trivy DB cache」即為此問題的長期解
  - Metric: `pipeline_step_duration_seconds{step_type="trivy-scan"}` 觀察趨勢

### D.3 Log SSE 連線 2 秒後斷開

- **症狀：** 前端開 Pipeline Run 詳情頁，log 顯示幾行後不再更新，瀏覽器 DevTools 顯示 `EventSource` 已關閉。
- **快速檢查：** `curl -N "http://synapse/api/v1/clusters/<clusterID>/pipelines/<pipelineID>/runs/<runID>/steps/<step>/logs?follow=true"` 看是否串流正常。
- **常見根因：**
  1. Reverse proxy（nginx）buffering 開啟，切斷 SSE
  2. Gin gzip middleware 包住了 SSE 路徑
  3. 後端 flush 遺漏
- **修復：**
  - nginx 加 `proxy_buffering off; proxy_cache off;`
  - `internal/router/router.go:59` 的 `gzip.WithExcludedPaths(...)` 需擴充所有 Pipeline Log SSE 路徑，例如 `/api/v1/clusters/*/pipelines/*/runs/*/steps/*/logs`
  - Handler 內 `c.Writer.Flush()` 後加 `c.Writer.(http.Flusher).Flush()`
- **預防：**
  - 所有 SSE 路徑加入 gzip exclude list 的統一變數
  - E2E 測試腳本驗證 SSE 持續 10s 不中斷

### D.4 Webhook 500 錯誤，日誌顯示 HMAC mismatch

- **症狀：** GitLab / GitHub webhook 收到 500，Synapse 日誌：`webhook HMAC verification failed`
- **快速檢查：**
  ```bash
  curl -i -X POST http://synapse/api/v1/webhooks/gitlab/<id> \
    -H "X-Gitlab-Token: <token>" \
    -d '{"test":1}'
  ```
- **常見根因：**
  1. Git provider 的 Secret 與 Synapse 側儲存的不一致（使用者忘了更新）
  2. Reverse proxy 改了 body（例如 nginx 壓縮）
  3. 不同 provider 的簽章 header 與驗證格式不同（GitHub 用 `X-Hub-Signature-256`，Gitea 用 `X-Gitea-Signature`；兩者皆為 HMAC-SHA256）
- **修復：**
  - 在 Git provider 側 reset webhook secret，重新設到 Synapse
  - 確認 `middleware.HMACVerify` 支援該 provider 的演算法
  - 檢查 reverse proxy 未對 `/api/v1/webhooks/*` 啟用 body 轉換
- **預防：**
  - Webhook 註冊時提供 `POST /api/v1/webhooks/test` 驗證 endpoint
  - 失敗時返回結構化 error code（`WEBHOOK_HMAC_MISMATCH`）方便前端顯示明確訊息

### D.5 Concurrency Group 沒有取消舊的 Run

- **症狀：** 設定了 `policy: cancel_previous`，但新 Run 開始後舊 Run 仍繼續執行。
- **快速檢查：**
  ```sql
  SELECT id, status, concurrency_group, created_at FROM pipeline_runs
  WHERE concurrency_group = 'user-service-main'
  ORDER BY created_at DESC LIMIT 5;
  ```
- **根因：**
  1. 舊 Run 的 Job 已在 K8s 端執行，取消 DB 狀態不會自動 delete Job
  2. `cancel_previous` 只在 Run **建立時**判定，執行中途若舊 Run 尚在 Pending queue，可能錯過
- **修復：**
  - `PipelineExecutor.cancelPreviousInGroup()` 必須同時：
    1. `UPDATE pipeline_runs SET status='cancelled' WHERE id = <old_id>`
    2. `clientset.BatchV1().Jobs(ns).Delete(ctx, job_name, ...)`
    3. 寫 audit log + 通知
  - 檢查 `internal/services/pipeline/concurrency.go` 是否完整刪除 Job
- **預防：**
  - 寫單元測試：「舊 Running run + 新 trigger → 舊 Job 30s 內消失」
  - Metric: `pipeline_cancellation_latency_seconds`，> 30s 告警

### D.6 `pipeline_runs` 表暴漲，DB 變慢

- **症狀：** DB 大小激增，`pipeline_runs` 查詢 > 1s。
- **快速檢查：**
  ```sql
  SELECT
    date_trunc('day', created_at) AS d,
    count(*) AS runs
  FROM pipeline_runs
  GROUP BY d ORDER BY d DESC LIMIT 14;
  ```
- **根因：** 沒有啟用 GC，或 retention 設太長。
- **修復：**
  - 確認 `internal/services/pipeline/gc_worker.go` 正在執行（log 有 `pipeline gc tick`）
  - 調整 retention：`pipeline_runs` 保留 90 天（status=success）/ 180 天（failed）
  - 若單個 Step 累計 log 超過上限（預設 100 MB）需停止追加後續 chunk，並把 Step 標記為 `log_truncated`
- **預防：**
  - §7.12 GC 策略文件化
  - Metric: `pipeline_runs_total_rows`，超過閾值告警

### D.7 ephemeral K8s Secret 沒被清掉

- **症狀：** `kubectl get secret -n <ns> | grep pipeline-run-` 列出大量已完成 Run 的 Secret。
- **快速檢查：** `kubectl get secret pipeline-run-123-step-456-secrets -o yaml | grep ownerReferences`
- **根因：**
  1. Secret 建立時沒設 `ownerReference`，K8s GC 不會自動刪
  2. owner Job 已被刪除但 cascade delete 未啟用
  3. Run 異常結束（例如 Synapse 重啟）漏清理
- **修復：**
  - `PipelineExecutor.createStepSecret()` 必須設 `metav1.OwnerReference{Controller: ptr.To(true), BlockOwnerDeletion: ptr.To(true)}` 指向該 Step 的 Job
  - 兜底：GC worker 掃描 `namespace` 內所有 `pipeline-run-*` secret，若對應 Run 已結束 > 1h 則刪除
- **預防：**
  - 單元測試：建立 Run → 模擬 Job 完成 → 驗證 Secret 被刪
  - 定期跑 `kubectl get secret -l synapse.io/pipeline-run -A | wc -l`

### D.8 Webhook 重試導致 Pipeline 跑兩次

- **症狀：** 一次 `git push` 觸發兩次 Pipeline Run，GitLab 側顯示 webhook 重試過。
- **根因：** `ReplayProtection` 未生效，或 `delivery_id` 為空。
- **快速檢查：**
  ```bash
  grep "webhook received" synapse.log | grep <delivery_id>
  # 應該只出現一次
  ```
- **修復：**
  - 確認 `middleware.ReplayProtection` 已掛在 webhook route group
  - 不同 provider 的 delivery ID header 不同：
    - GitHub: `X-GitHub-Delivery`
    - GitLab: `X-Gitlab-Event-UUID`（v15+）
    - Gitea: `X-Gitea-Delivery`
  - `WebhookAdapter` 需實作 `GetDeliveryID()` 並 fallback 到 `hash(payload + timestamp)` 若 header 缺失
- **預防：**
  - §10.2 強制要求所有 provider adapter 實作 `GetDeliveryID()`，CI test 覆蓋所有 provider

### D.9 Pipeline YAML 驗證失敗但錯誤訊息看不懂

- **症狀：** 使用者 POST YAML，返回 400 `invalid pipeline schema: (root): Additional property XYZ is not allowed`，不知道 XYZ 是哪裡錯。
- **修復：**
  - JSON Schema 錯誤需經過 user-friendly formatter：
    ```go
    // internal/services/pipeline/validator.go
    func formatSchemaError(err *gojsonschema.ResultError) string {
        field := err.Field()
        desc := err.Description()
        if field == "(root)" {
            return desc
        }
        return fmt.Sprintf("field '%s': %s", field, desc)
    }
    ```
  - 前端 YAML editor 使用 `monaco-yaml` + `synapse-pipeline-schema.json` 做即時 lint，錯誤出現在對應行
- **預防：**
  - API 回應包含 `field` / `line` / `column` 三個欄位
  - 前端錯誤提示顯示 diff-style 的紅線

### D.10 Promotion 審核卡住（超過 24h 仍 pending）

- **症狀：** M17 Promotion 流水線的 `approval-gate` step 一直卡住，使用者抱怨「點了 approve 沒反應」。
- **快速檢查：**
  ```sql
  SELECT id, action, status, resource_name, pipeline_run_id, created_at
  FROM approval_requests
  WHERE action = 'production_gate' AND status = 'pending'
  ORDER BY created_at ASC;
  ```
- **常見根因：**
  1. `ApprovalRequest.status` 已 approved，但 `PipelineExecutor` 的 watcher 未觸發
  2. `required_approvers: 2` 但只有 1 人點
  3. Approval 送出後通知發失敗（NotifyChannel 設定錯）
- **修復：**
  - 檢查 `internal/services/pipeline/approval_watcher.go` 是否在運作（可能 Synapse 重啟後未 resume）
  - Watcher 啟動時需掃描所有 `status=pending` 的 `approval_requests WHERE pipeline_run_id IS NOT NULL`，重新綁定 poller
  - 前端顯示「目前 1/2 批准」的進度，避免使用者以為已完成
- **預防：**
  - Approval watcher 做成背景 worker（不依賴記憶體 map）
  - Metric: `approval_pending_count`，> 10 告警（可能 watcher 失效）

---

### D.11 通用診斷指令速查表

```bash
# 1. 查特定 Run 的完整狀態
curl -sH "Authorization: Bearer $TOKEN" \
  http://synapse/api/v1/clusters/<clusterID>/pipelines/<pipelineID>/runs/<runID> | jq

# 2. 查 Pipeline 關聯的 K8s Job
kubectl get job -n <ns> -l synapse.io/pipeline-run=<id>

# 3. 查 Pipeline Pod 日誌（真實容器）
kubectl logs -n <ns> -l synapse.io/pipeline-run=<id> --all-containers --tail=100

# 4. 查 Synapse 對該 Run 的持久化 log chunk
sqlite3 synapse.db "SELECT step_run_id, chunk_seq, substr(content,1,200) FROM pipeline_logs WHERE pipeline_run_id=<runID> ORDER BY step_run_id, chunk_seq DESC LIMIT 20;"

# 5. 查最近失敗的 runs
sqlite3 synapse.db "SELECT id,pipeline_id,status,error FROM pipeline_runs WHERE status='failed' ORDER BY id DESC LIMIT 20;"

# 6. 強制取消 Run（繞過 API，緊急情況）
sqlite3 synapse.db "UPDATE pipeline_runs SET status='cancelled' WHERE id=<id>;"
kubectl delete job -n <ns> -l synapse.io/pipeline-run=<id>
```

---

**文件結束。** 本文件為 CI/CD 架構定稿版，實作團隊應將本文件與 [ARCHITECTURE_REVIEW.md](./ARCHITECTURE_REVIEW.md) 共同作為 M13–M17 的總綱領。ADR 更動、SLA 調整、威脅模型演進需以 PR 方式回寫本文件。
