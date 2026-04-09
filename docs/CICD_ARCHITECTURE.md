# Synapse CI/CD 架構設計文件

> 版本：v2.0 | 日期：2026-04-09 | 狀態：設計中（重構）
> 對應里程碑：M13（CI Pipeline 引擎）、M14（Git 整合）、M15（Registry 整合）、M16（原生 GitOps）、M17（環境流水線）
> 相關文件：[ARCHITECTURE_REVIEW.md](./ARCHITECTURE_REVIEW.md)、[PLANNING.md](../PLANNING.md)

---

## v2.0 變更記錄（對 v1.0 的修正）

本次重構源自對 v1.0 的全面審查。v1.0 作為第一版骨架可讀性好，但在實作前必須補齊以下關鍵缺口，否則進入實作後會大幅重工：

| 類別 | v1.0 問題 | v2.0 修正 |
|------|----------|----------|
| 宣稱誤導 | 「零額外元件」過於絕對 | 改為「執行引擎零額外安裝」，明列 Step image 屬於基礎設施依賴 |
| Trivy 路徑矛盾 | v1 說 K8s Job，但現有 `trivy_service.go` 在 host 跑 `exec.Command` | 新增 §14 「Trivy 遷移與雙軌共存」章節 |
| 跨叢集執行 | 未定義 Pipeline Job 的 watcher 如何跨遠端叢集運作 | 新增 §7.5「跨叢集執行路徑」 |
| Webhook 安全 | 未指定路由群組、rate limit、replay protection | 新增 §10.1「公開端點安全設計」 |
| Pipeline RBAC | 完全未設計 | 新增 §7.6「Pipeline RBAC 與執行身份」 |
| Secrets 管理 | v1 把敏感資料塞進 `steps_json` | 新增 §7.3「Secrets 管理」與 `pipeline_secrets` 表 |
| Pod 安全基線 | 未提 SecurityContext / NetworkPolicy / Resource Limits | 新增 §7.7「Pipeline Pod Security Baseline」 |
| Artifact | 無產出追蹤設計 | 新增 §7.4「Workspace 與 Artifact」與 `pipeline_artifacts` 表 |
| 並發控制 | 假設建立 Run 立刻執行 | 新增 §7.8「執行佇列與並發控制」 |
| Concurrency Group | 無 | 新增 §10.3「Concurrency Group 串聯」 |
| 重試與重跑 | 無 | 新增 §7.10「失敗處理：重試、rerun、取消」 |
| Log 持久化 | 只提 SSE 串流 | 新增 §7.11「Log 持久化與查詢」 |
| Garbage Collection | 無 | 新增 §7.12「資源清理策略」 |
| 版本快照 | `steps_json` 無版本，歷史 Run 無法重現 | 新增 `pipeline_versions` 表 + `pipeline_runs.snapshot_id` |
| 通知整合 | v1 只提 AlertManager/DingTalk/Email 文字 | 改為明確復用現有 `NotifyChannel` 模型 |
| ArgoCD 邊界 | 原生 GitOps 與既有 ArgoCD 代理關係不清 | 新增 §12.1「與 ArgoCD 代理的邊界」 |
| AI 整合 | 未整合現有 AI 診斷 | 新增 §16.3「Pipeline 失敗 → AI 根因分析」 |
| M13 拆分 | 單一 8 週里程碑風險過大 | 拆成 M13a（4W 核心執行引擎）+ M13b（4W 進階 Steps 與 UX） |
| Observability | 無 metrics / audit 章節 | 新增 §16「Observability」 |
| 失敗模式 | 無 | 新增 §17「失敗模式與故障恢復」 |
| YAML Schema | 無 | 新增附錄 A「Pipeline YAML Schema」 |

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
21. [附錄 A：Pipeline YAML Schema](#附錄-apipeline-yaml-schema)
22. [附錄 B：與 ARCHITECTURE_REVIEW.md 對應關係](#附錄-b與-architecture_reviewmd-對應關係)

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
| `ApprovalRequest` | `internal/models/approval.go` | M17 Promotion 擴充現有模型，新增 `ResourceKind='Pipeline'`、`Action='promote'` 類型 |
| `ImageScanResult` | `internal/models/security.go` | Trivy Step 直接寫入此表，新增 `scan_source`、`pipeline_run_id`、`step_run_id` 欄位 |
| `ClusterInformerManager` | `internal/k8s/` | Pipeline Job watcher 走此 manager 取 remote cluster client，不新開 Informer 池 |
| `AuditService` | `internal/services/audit_service.go` | Pipeline CRUD / 手動觸發 / 取消 全部 opLog 記錄 |
| `OperationAudit` middleware | `internal/middleware/` | 自動涵蓋 Pipeline 所有寫入動作 |
| `pkg/crypto` AES-256-GCM | `pkg/crypto/aesgcm.go` | `pipeline_secrets.value_enc`、`registries.password_enc` 加密 |
| Helm Release 管理（M4） | `internal/handlers/helm_release.go` | `deploy-helm` Step 直接調用 `HelmService`，部署歷程關聯 Pipeline Run |
| AI 診斷（M5/M7） | `internal/services/ai_*` | Pipeline 失敗時提供「AI 根因分析」按鈕，傳入 failed step 的 log + 錯誤碼 |
| ArgoCD 代理 | `internal/services/argocd_service.go` | `deploy-argocd-sync` Step 觸發 Sync，與原生 GitOpsApp（M16）共存 |
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

**Schema 相容性重點（v1 遺漏）：** `ImageScanResult` 需先新增 `scan_source VARCHAR(20)` 欄位並加預設值 `manual`，過渡方案與未來 Pipeline Step 才能共用同一張表。

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
| `Workspace` | Steps 間共享的工作目錄（預設 emptyDir，可升級為 PVC） |
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
1. Scheduler 在提交 K8s Job 前解析 `${{ secrets.* }}` 引用
2. 以 Pipeline Run 為單位建立暫時 K8s Secret（`generateName: pr-run-{id}-`）
3. Step Pod `envFrom: secretRef: ...` 掛載
4. StepRun 結束後 Secret 自動刪除（`ownerRef` 指向 Job）
5. **Log Scrubber：** log 串流時用正則過濾已知 secret 值 → 以 `***REDACTED***` 取代

**權限：**
- `global` scope：PlatformAdmin only
- `cluster` scope：該叢集 Writer
- `pipeline` scope：該 Pipeline 建立者或 Writer
- 讀取「原始值」永遠不開放 API，只能重新設定

### 7.4 Workspace 與 Artifact

**Workspace**：Steps 間共享的工作目錄。

| 模式 | 儲存介質 | 適用場景 |
|------|---------|---------|
| `emptyDir` | Node 本地 | 同 Node 排程的 Steps（預設）|
| `pvc` | 動態建立 PVC | 跨 Node 或需要持久化（例：scan 結果保留 24h） |

Workspace 在 `PipelineRun` 結束後自動釋放，pvc 模式有 `retentionHours` 可設定延長。

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
  automountServiceAccountToken: true         # 僅限使用預設 SA，否則 false
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

**例外處理（需明確宣告）：**
- **Kaniko** 需要寫入 root filesystem → 該 Step 的 `readOnlyRootFilesystem: false`，需 PlatformAdmin 批准並記錄稽核
- **某些 build 工具** 需要 `/var/run/docker.sock`：**不允許**，改用 Kaniko/BuildKit rootless 模式

**NetworkPolicy（在目標 Namespace 套用）：**
- 預設：egress 僅允許 DNS + Git providers + Registries + Synapse backend 自身
- 可於 Pipeline 定義加白名單 host

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

### 8.4 程式碼結構

```
internal/
  handlers/
    pipeline.go                 ← CRUD + Run handlers
    pipeline_secret.go
    pipeline_log.go             ← SSE + 歷史 log 查詢
  services/
    pipeline_service.go         ← 業務邏輯
    pipeline_executor.go        ← Scheduler + JobBuilder
    pipeline_watcher.go         ← JobWatcher（搭配 ClusterInformerManager）
    pipeline_secret_service.go
    pipeline_log_service.go
  models/
    pipeline.go                 ← Pipeline / PipelineVersion / PipelineRun / StepRun
    pipeline_secret.go
    pipeline_artifact.go
    pipeline_log.go
  workers/
    pipeline_gc_worker.go       ← 清理 K8s Job + Workspace PVC
    pipeline_log_retain.go      ← 走現有 LogRetentionWorker 擴充
```

### 8.5 完成指標

- 建立一個 3-step Pipeline（build-image → scan → deploy），手動觸發成功
- 同 Pipeline 並發上限 = 1 時，第二次觸發正確排隊
- Pipeline 修改後，歷史 Run 仍能看到當時的 steps 定義
- PipelineSecret 加密儲存，Log 內不出現明文
- Run 結束 1 小時後 K8s Job 已被清理
- 跨叢集執行：Pipeline 目標為遠端匯入的叢集時，Job 建立在該叢集並正確監聽
- 所有 Pipeline 寫入動作出現在操作稽核

---

## 9. M13b — 進階 Steps 與使用者體驗（4 週）

**里程碑目標：** 補齊生產級 Pipeline 所需的進階能力與完整 UI。

### 9.1 涵蓋範圍

- [x] 進階 Step 類型：`build-jar`（Maven/Gradle）、`trivy-scan`、`push-image`、`deploy-helm`、`deploy-argocd-sync`、`approval`、`notify`
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

---

## 10. M14 — Git 整合與 Webhook 觸發

**估計工作量：4 週** | **優先級：🔴 高**

### 10.1 公開端點安全設計（v2.0 重寫）

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

### 12.1 與 ArgoCD 代理的邊界（v2.0 新增）

Synapse 已經有 ArgoCD 代理（`internal/services/argocd_service.go`），M16 新增原生 GitOps 後必須定義清楚兩者的關係：

| 場景 | 建議 |
|------|------|
| 組織已有 ArgoCD 集中管理 | 繼續使用 ArgoCD 代理，M16 原生 GitOps 不啟用 |
| 組織想精簡元件（不想裝 ArgoCD） | 使用 M16 原生 GitOps |
| 組織同時有兩者 | **不同 GitOpsApp 選擇不同後端**，單一 App 不能混用 |
| Pipeline 的 deploy Step | 支援三選一：`deploy`（kubectl apply）/ `deploy-argocd-sync`（觸發代理）/ `gitops-sync`（原生）|

**前端導航：**
- 「GitOps 應用」頁面顯示**兩種來源**合併列表（`source: argocd` / `source: native`）
- ArgoCD 來源的 App 點進詳情走 ArgoCD 代理 API
- 原生來源的 App 走 M16 API

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

### 13.2 ApprovalRequest 擴充（v2.0 新增細節）

現有 `ApprovalRequest` 模型（`internal/models/approval.go`）針對 K8s 資源動作（scale / delete / update / apply）。M17 需要擴充：

```go
// 現有欄位不動，新增：
type ApprovalRequest struct {
    gorm.Model
    // ... 原有欄位
    ResourceKind  string  // 原本：Deployment/StatefulSet/DaemonSet
                          // 新增：Pipeline / PipelineRun / Environment
    Action        string  // 原本：scale / delete / update / apply
                          // 新增：promote / production-gate
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
  └─ No  → 建立 ApprovalRequest(Action=promote, ResourceKind=Pipeline)
                ↓
           審核人收到通知（NotifyChannel）
                ↓
           核准 → 建立 staging PipelineRun
           拒絕 → 記錄原因，通知觸發者
    ↓
staging 部署成功
    ↓
Production Gate（永遠需要人工審核，Action=production-gate）
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

**這是 v2.0 新增的章節，補上 v1.0 最大的矛盾。**

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
- `ImageScanResult` 新增 `scan_source`（manual / ci_push / informer / pipeline）
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
| `GET /clusters/:id/security/scans` | 列出 manual 來源 | 列出全部來源，新增 `scan_source` 欄位供過濾 |
| `GET /clusters/:id/security/scans/:id` | 含 resultJson | 不變 |

---

## 15. 通知整合（復用 NotifyChannel）

**v1.0 缺口：** 描述上寫「複用 AlertManager / DingTalk / Email」但沒說整合到 `NotifyChannel`。

**v2.0 明確規範：**

### 15.1 事件來源

| 事件 | 預設等級 | 來源 |
|------|---------|------|
| PipelineRun started | Info | PipelineExecutor |
| PipelineRun succeeded | Info | PipelineExecutor |
| PipelineRun failed | Error | PipelineExecutor |
| StepRun failed | Warning | JobWatcher |
| Trivy scan critical | Critical | TrivyScanStep |
| ApprovalRequest created (promote) | Info | PromotionService |
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

**v2.0 新增章節**，對齊 [ARCHITECTURE_REVIEW.md](./ARCHITECTURE_REVIEW.md) 的 P1-10 OpenTelemetry 方向。

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

**v2.0 新增章節。** 設計階段就思考壞掉怎麼辦，而不是事故後補救。

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
- [ ] JobWatcher（訂閱 Informer Job 事件）
- [ ] Log 雙層儲存（SSE + pipeline_logs）
- [ ] Log Scrubber（過濾 secret 值）
- [ ] GC Worker（K8s Job + Workspace PVC）

**Week 4：基本 Steps 與 API**
- [ ] Step 類型：`build-image`（Kaniko）、`deploy`（kubectl apply）、`run-script`
- [ ] Pipeline CRUD API + 手動觸發 API
- [ ] Cancel / Rerun API
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
- [ ] AI 根因分析按鈕 + context 組裝
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
- [ ] ApprovalRequest 擴充（Action=promote/production-gate + pipeline_run_id）
- [ ] Promotion 邏輯（自動 / 人工審核）
- [ ] 冒煙測試 Step 整合
- [ ] Production Gate 通知

### Trivy 二階段遷移（併入 M13b/Post-M13）

- [ ] Phase 2：新增 TrivyScanStep（K8s Job 模式）
- [ ] Phase 3：`trivy_service.go` 的 host exec 改為建立 K8s Job
- [ ] Phase 4：`trivy-db-cache` PVC + 每日更新 CronJob

---

**總估計工作量：**
- M13a（4W）+ M13b（4W）+ M14（4W）+ M15（3W）+ M16（6W）+ M17（5W）= **26 週**
- 可並行：M14 可在 M13a 完成後開始；M15 可在 M13a 完成後開始
- Trivy 遷移 Phase 3/4 併入 M13b post-work，不計入主線

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
    type: emptyDir               # emptyDir | pvc
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

**文件結束。** 本版本為 v2.0 設計稿，實作前請與團隊 review 並確認章節編號與 M13a/M13b 拆分是否合適。
