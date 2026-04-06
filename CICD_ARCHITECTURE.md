# Synapse CI/CD 架構設計文件

> 版本：v1.0 | 日期：2026-04-06 | 狀態：設計中
> 對應里程碑：M13（CI Pipeline 引擎）、M14（Git 整合）、M15（Registry 整合）、M16（原生 GitOps）、M17（環境流水線）

---

## 目錄

1. [戰略目標與設計原則](#1-戰略目標與設計原則)
2. [整體架構流程](#2-整體架構流程)
3. [現況差距分析](#3-現況差距分析)
4. [近期過渡方案（不需等 M13）](#4-近期過渡方案不需等-m13)
5. [M13 — 原生 CI Pipeline 引擎](#5-m13--原生-ci-pipeline-引擎)
6. [M14 — Git 整合與 Webhook 觸發](#6-m14--git-整合與-webhook-觸發)
7. [M15 — 映像 Registry 整合（Harbor）](#7-m15--映像-registry-整合harbor)
8. [M16 — 原生輕量 GitOps（CD）](#8-m16--原生輕量-gitopscd)
9. [M17 — 環境管理與 Promotion 流水線](#9-m17--環境管理與-promotion-流水線)
10. [安全掃描整合（Trivy）](#10-安全掃描整合trivy)
11. [通知與告警整合](#11-通知與告警整合)
12. [資料模型總覽](#12-資料模型總覽)
13. [技術選型](#13-技術選型)
14. [實作路線圖](#14-實作路線圖)

---

## 1. 戰略目標與設計原則

### 戰略目標

從「K8s 多叢集管理工具」演進為「端到端 DevSecOps 平台」，讓使用者在 **Synapse 單一介面**完成從 code commit 到生產部署的完整流程，不需要在 GitLab CI / Harbor / ArgoCD / 監控平台之間切換。

GitLab 在此架構中僅作為 **程式碼倉庫**，其餘 Pipeline 定義、執行、掃描、部署、通知，全部由 Synapse 集中管控。

### 設計原則

| 原則 | 說明 |
|------|------|
| **集中戰情室** | 所有 Pipeline 狀態、掃描結果、部署記錄、告警，統一在 Synapse 查看 |
| **零額外元件（核心路徑）** | CI 執行引擎以 K8s Job 為單元，無需安裝 Tekton / Jenkins |
| **安全左移** | Trivy 掃描內建於 Pipeline，掃描失敗則阻止 push to Harbor |
| **審核閘門** | 每個環境晉升（dev → staging → prod）可設定人工審核，複用現有 Approval Workflow |
| **漸進式演進** | 近期過渡方案不依賴 M13，GitLab CI 仍可跑，結果推進 Synapse；M13 完成後無縫接管 |
| **可插拔** | 進階場景（複雜 DAG、特殊 build 工具）支援接入外部 Tekton/Jenkins |

---

## 2. 整體架構流程

### 目標架構（M13–M17 全完成後）

```
開發者 git push
    │
    ▼
GitLab（純 Repo）
    │  Webhook（push event）
    ▼
Synapse Webhook Receiver（M14）
    │  比對 Pipeline 綁定規則（Repo + Branch glob）
    ▼
Synapse Pipeline 引擎（M13）
    │
    ├─ Step 1: Build .jar（K8s Job / Maven / Gradle）
    │
    ├─ Step 2: Build Container Image（K8s Job / Kaniko）
    │
    ├─ Step 3: Trivy 安全掃描（K8s Job / trivy image）
    │       │
    │       ├─ CRITICAL/HIGH 超標？
    │       │       ├─ Yes → Pipeline Failed
    │       │       │         ├─ 通知開發者（Synapse Alert / DingTalk / Email）
    │       │       │         └─ 記錄錯誤詳情到 Synapse DB
    │       │       └─ No  → 繼續
    │
    ├─ Step 4: Push Image to Harbor（M15）
    │           └─ 自動注入 imagePullSecret
    │
    └─ Step 5: Deploy to K8s（kubectl apply / helm upgrade）
                └─ 可選：觸發 ArgoCD Sync（M16）
    │
    ▼
Synapse 集中戰情室
    ├─ Pipeline 執行狀態（成功 / 失敗 / 進行中）
    ├─ Trivy 掃描結果（CVE 列表、嚴重程度分佈）
    ├─ Deployment 狀態（Pod 數、Ready 狀態）
    ├─ 告警通知（掃描失敗、部署失敗、Pod CrashLoop）
    └─ 操作稽核（誰在何時觸發了哪條 Pipeline）
```

### 環境晉升流程（M17）

```
Pipeline 成功
    ↓
自動部署到 dev 環境
    ↓
冒煙測試（可選步驟）通過
    ↓
自動 Promote 到 staging
    ↓
人工審核（Approval Workflow）
    ↓
部署到 production
```

---

## 3. 現況差距分析

| 能力維度 | 現況 | 目標狀態 | 對應里程碑 |
|---------|------|---------|-----------|
| CI Pipeline 執行 | **完全沒有** | K8s Job 驅動的原生 Pipeline 引擎 | M13 |
| Git Webhook 接收 | 無 | 支援 GitLab / GitHub / Gitea | M14 |
| 映像建置 | 無 | Kaniko in K8s，無需 Docker daemon | M13/M15 |
| Trivy 掃描 | 僅手動觸發，獨立於部署流程 | 內建於 Pipeline，掃描失敗阻止部署 | M13 |
| Harbor 整合 | 無 | Registry 管理 + Tag 瀏覽 + 自動 Push | M15 |
| GitOps / CD | 代理外部 ArgoCD | 原生 GitOps + 保留 ArgoCD 代理 | M16 |
| 環境流水線 | 僅 Namespace 粒度 | dev → staging → prod，含人工審核閘門 | M17 |
| 掃描結果集中查看 | 有（手動掃描結果存 DB） | Pipeline 掃描結果自動記錄，可從 Pipeline Run 詳情跳轉 | M13 |
| 部署通知 | Prometheus Alert / K8s Event Alert | Pipeline 失敗、掃描失敗、部署失敗均觸發通知 | M13/M14 |

---

## 4. 近期過渡方案（不需等 M13）

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
          \"pod_name\": \"$CI_PROJECT_NAME\"
        }"
```

**效果：** Synapse 安全掃描頁可看到所有 CI 觸發的掃描結果，不再需要手動輸入映像名稱。

### 方案 B：Informer 自動掃描（Pod 上線時）

在 `ClusterInformerManager` 的 Pod `OnAdd / OnUpdate` 回調中，偵測到新映像時自動呼叫 `TrivyService.TriggerScan()`：

```go
// 偽代碼
func (m *ClusterInformerManager) onPodAdd(obj interface{}) {
    pod := obj.(*v1.Pod)
    for _, container := range pod.Spec.Containers {
        if isNewImage(container.Image) {
            m.trivyService.TriggerScan(clusterID, pod.Namespace, pod.Name, container.Name, container.Image)
        }
    }
}
```

**效果：** 叢集內所有正在運行的映像都會自動掃描，包含不走 GitLab 部署的情況。

> 方案 A 與 B 不互斥，建議同時實施作為 M13 前的橋接方案。

---

## 5. M13 — 原生 CI Pipeline 引擎

**估計工作量：8 週** | **優先級：🔴 高**

### 5.1 核心概念

| 概念 | 說明 |
|------|------|
| `Pipeline` | Pipeline 定義，包含 Steps DAG、觸發條件、環境變數 |
| `PipelineRun` | 一次具體執行記錄，包含觸發來源（手動 / Webhook）、狀態、開始/結束時間 |
| `StepRun` | 每個 Step 的執行記錄，對應一個 K8s Job，包含狀態、日誌、耗時 |
| `Workspace` | Steps 間共享的工作目錄（K8s PVC 或 emptyDir） |

### 5.2 執行引擎流程

```
觸發（手動 / Webhook）
    ↓
建立 PipelineRun（status: pending）
    ↓
解析 Steps DAG（拓撲排序）
    ↓
依序提交 K8s Job（image / command / env / resource limits / workspace mount）
    ↓
Watch Job 狀態（Informer 或 polling）
    ↓
即時更新 StepRun 狀態 + 串流 Pod 日誌（SSE）
    ↓
所有 Steps 成功 → PipelineRun: success
任一 Step 失敗  → 取消後續 Steps → PipelineRun: failed → 觸發通知
```

### 5.3 內建 Step 類型

| Step 類型 | 說明 | 實作方式 |
|----------|------|---------|
| `build-jar` | Maven / Gradle 編譯 | K8s Job（maven:3.9 / gradle:8）|
| `build-image` | 容器映像建置 | K8s Job（gcr.io/kaniko-project/executor）|
| `trivy-scan` | 映像漏洞掃描 | K8s Job（aquasec/trivy），結果寫入 DB |
| `push-image` | Push 到 Registry | Kaniko 內建 / 獨立 K8s Job |
| `deploy` | kubectl apply / helm upgrade | K8s Job（bitnami/kubectl / alpine/helm）|
| `run-tests` | 單元測試 / 整合測試 | K8s Job（任意映像）|
| `notify` | 發送通知 | 後端直接呼叫（不需 K8s Job）|
| `custom` | 自定義指令 | K8s Job（任意映像 + 任意指令）|

### 5.4 Trivy 掃描步驟設計

```yaml
# Pipeline 步驟定義範例
steps:
  - name: security-scan
    type: trivy-scan
    image: aquasec/trivy:latest
    config:
      target: "{{ .steps.build-image.outputs.image }}"  # 上一步產出的映像
      severity_threshold: HIGH          # 超過此等級則 fail
      ignore_unfixed: true              # 忽略尚無修復版本的 CVE
    on_failure: abort                   # 失敗時終止整條 Pipeline
```

掃描結果自動關聯到 `ImageScanResult` 表，並可從 Pipeline Run 詳情頁直接跳轉查看完整 CVE 清單。

### 5.5 API 設計

```
# Pipeline CRUD
GET    /api/v1/pipelines                            列表
POST   /api/v1/pipelines                            建立
GET    /api/v1/pipelines/:id                        詳情
PUT    /api/v1/pipelines/:id                        更新
DELETE /api/v1/pipelines/:id                        刪除

# 執行
POST   /api/v1/pipelines/:id/run                    手動觸發
GET    /api/v1/pipelines/:id/runs                   執行歷史
GET    /api/v1/pipelines/:id/runs/:runId             執行詳情
POST   /api/v1/pipelines/:id/runs/:runId/cancel      取消

# 日誌（SSE 串流）
GET    /api/v1/pipelines/:id/runs/:runId/steps/:step/logs

# Webhook 入口（M14）
POST   /api/v1/webhooks/:provider/:token
```

### 5.6 前端頁面設計

```
ui/src/pages/pipeline/
  ├── PipelineList.tsx         列表（狀態燈、最後執行時間、觸發者）
  ├── PipelineEditor.tsx       步驟卡片編輯器 + YAML 雙模式
  ├── PipelineRunList.tsx      執行歷史列表
  ├── PipelineRunDetail.tsx    DAG 進度圖 + 各步驟狀態卡片
  └── StepLogViewer.tsx        步驟日誌串流（SSE，複用 Terminal 樣式）
```

**PipelineRunDetail DAG 視覺化：**

```
[Build JAR] ──→ [Build Image] ──→ [Trivy Scan] ──→ [Push Harbor] ──→ [Deploy]
  ✅ 2m3s           ✅ 4m12s          ❌ 0m45s
                                    ↑
                              CRITICAL: CVE-2024-1234
                              HIGH: 3 個漏洞
                              → Pipeline 已中止
```

### 5.7 資料模型

```sql
-- Pipeline 定義
CREATE TABLE pipelines (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    cluster_id  BIGINT,                    -- 執行所在叢集
    namespace   VARCHAR(255),              -- K8s Job 建立在此 Namespace
    steps_json  TEXT NOT NULL,             -- Steps DAG JSON 定義
    triggers    TEXT,                      -- Webhook 觸發條件 JSON
    created_by  BIGINT,
    created_at  DATETIME,
    updated_at  DATETIME
);

-- Pipeline 執行記錄
CREATE TABLE pipeline_runs (
    id           BIGINT PRIMARY KEY AUTO_INCREMENT,
    pipeline_id  BIGINT NOT NULL,
    status       VARCHAR(50),              -- pending/running/success/failed/cancelled
    trigger_by   VARCHAR(255),             -- "manual:user_id" / "webhook:gitlab:sha"
    started_at   DATETIME,
    finished_at  DATETIME,
    git_sha      VARCHAR(64),
    git_branch   VARCHAR(255),
    created_at   DATETIME
);

-- 步驟執行記錄
CREATE TABLE step_runs (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    pipeline_run_id BIGINT NOT NULL,
    step_name       VARCHAR(255),
    status          VARCHAR(50),           -- pending/running/success/failed/skipped
    k8s_job_name    VARCHAR(255),          -- 對應的 K8s Job 名稱
    started_at      DATETIME,
    finished_at     DATETIME,
    exit_code       INT,
    error_message   TEXT,
    outputs_json    TEXT                   -- 步驟輸出（如 image digest）
);
```

---

## 6. M14 — Git 整合與 Webhook 觸發

**估計工作量：4 週** | **優先級：🔴 高**

### 6.1 支援 Provider

| Provider | Webhook 驗證方式 | 觸發事件 |
|---------|---------------|---------|
| GitLab | X-Gitlab-Token（Secret Token）| Push、Merge Request、Tag |
| GitHub | X-Hub-Signature-256（HMAC-SHA256）| Push、Pull Request、Release |
| Gitea | X-Gitea-Signature（HMAC-SHA256）| Push、Pull Request |

### 6.2 Webhook 接收流程

```
POST /api/v1/webhooks/:provider/:token
    ↓
驗證 HMAC Signature（防偽造）
    ↓
解析 Payload（branch / sha / author / changed files）
    ↓
查詢綁定此 Repo + Branch 的 Pipeline
    ↓
評估觸發條件（branch glob / path filter / event type）
    ↓
建立 PipelineRun（TriggerBy="webhook:gitlab:abc123"）
    ↓
回傳 202 Accepted（Webhook 要求快速回應）
    ↓
非同步執行 Pipeline
```

### 6.3 Pipeline 觸發條件設計

```yaml
triggers:
  - type: webhook
    provider: gitlab
    repo: "company/backend-service"
    branch: "main"           # 支援 glob，如 "release/*"
    events:
      - push
      - merge_request
    path_filter:             # 可選：只有特定路徑變動才觸發
      - "src/**"
      - "Dockerfile"
```

### 6.4 前端頁面設計

```
ui/src/pages/settings/GitIntegration.tsx
    ├── Git Provider 清單（GitLab / GitHub / Gitea）
    ├── 新增 Provider（URL + Token + Webhook Secret）
    └── 連線測試

Pipeline Editor → Triggers 設定區塊
    ├── 選擇 Git Provider
    ├── 輸入 Repo 路徑
    ├── Branch 規則（glob）
    └── 複製 Webhook URL（貼到 GitLab 設定）
```

---

## 7. M15 — 映像 Registry 整合（Harbor）

**估計工作量：3 週** | **優先級：🟡 中**

### 7.1 功能範圍

| 功能 | 說明 |
|------|------|
| Registry 連線設定 | Harbor URL / 使用者名稱 / 密碼 / TLS 驗證，加密儲存 |
| Repository 瀏覽 | 列出 Project → Repository → Tag |
| Tag 詳情 | 映像大小、建立時間、Digest、Labels |
| Tag 保留策略設定 | 保留最新 N 個 Tag，自動清理舊 Tag |
| Trivy 掃描觸發 | 從 Tag 詳情頁直接觸發掃描 |
| Pipeline 整合 | `push-image` Step 自動使用設定的 Registry 憑證 |
| imagePullSecret 注入 | Deploy Step 自動在目標 Namespace 建立 Secret |

### 7.2 支援 Registry

| Registry | 支援方式 |
|---------|---------|
| Harbor | Docker Registry API v2 + Harbor 專屬 API（Project / Robot Account）|
| Docker Hub | Docker Registry API v2 |
| 阿里雲 ACR | Docker Registry API v2 |
| AWS ECR | Docker Registry API v2 + AWS SDK（token 刷新）|
| GCR / GAR | Docker Registry API v2 + GCP OAuth |

### 7.3 資料模型

```sql
CREATE TABLE registries (
    id           BIGINT PRIMARY KEY AUTO_INCREMENT,
    name         VARCHAR(255) NOT NULL,   -- 顯示名稱
    type         VARCHAR(50),             -- harbor/dockerhub/ecr/gcr
    url          VARCHAR(512),
    username     VARCHAR(255),
    password_enc TEXT,                    -- AES-256-GCM 加密
    insecure_tls BOOLEAN DEFAULT FALSE,
    created_at   DATETIME
);
```

---

## 8. M16 — 原生輕量 GitOps（CD）

**估計工作量：6 週** | **優先級：🟡 中**

### 8.1 設計分層

**Layer 1（原生內建）：** 輕量 GitOps，覆蓋 80% 使用場景
- 定義 `GitOpsApp`（Git Repo + 路徑 + 目標叢集 + Namespace）
- 定期 Diff（預設每 5 分鐘）
- 可設定 Auto Sync 或 Drift 通知
- 支援純 YAML manifest、Kustomize overlay、Helm Chart

**Layer 2（升級路徑）：** 現有 ArgoCD 代理保留
- ArgoCD App Health 聚合到主儀表板
- Pipeline 部署步驟可選「觸發 ArgoCD Sync」而非直接 kubectl apply
- 適合已有 ArgoCD 的組織平滑遷移

### 8.2 Diff 引擎

```
Git Repo（目標狀態）
    ↓ clone / pull
Render（Kustomize build / helm template）
    ↓
K8s API（實際狀態）
    ↓
Strategic Merge Diff（k8s.io/apimachinery）
    ↓
有差異？
  ├─ Auto Sync ON  → kubectl apply
  └─ Auto Sync OFF → Drift 通知（Synapse Alert）
```

### 8.3 資料模型

```sql
CREATE TABLE gitops_apps (
    id             BIGINT PRIMARY KEY AUTO_INCREMENT,
    name           VARCHAR(255),
    git_provider   BIGINT,                -- FK → git_providers
    repo_url       VARCHAR(512),
    branch         VARCHAR(255),
    path           VARCHAR(512),          -- 在 repo 中的路徑
    render_type    VARCHAR(50),           -- raw/kustomize/helm
    helm_values    TEXT,                  -- Helm values override JSON
    cluster_id     BIGINT,
    namespace      VARCHAR(255),
    sync_policy    VARCHAR(50),           -- auto/manual
    sync_interval  INT DEFAULT 300,       -- 秒
    last_synced_at DATETIME,
    last_diff_at   DATETIME,
    status         VARCHAR(50)            -- synced/drifted/error
);
```

---

## 9. M17 — 環境管理與 Promotion 流水線

**估計工作量：5 週** | **優先級：🟢 低**

### 9.1 環境概念

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

### 9.2 Promotion 流程

```
Pipeline 執行成功（dev 部署完成）
    ↓
冒煙測試 Step（可選，timeout 10m）
    ↓
auto_promote = true？
  ├─ Yes → 自動建立 staging PipelineRun
  └─ No  → 建立 Approval Request（複用現有 Approval Workflow §8.3）
                ↓
           審核人收到通知（DingTalk / Email）
                ↓
           核准 → 建立 staging PipelineRun
           拒絕 → 記錄原因，通知觸發者
    ↓
staging 部署成功
    ↓
Production Gate（永遠需要人工審核）
    ↓
部署到 production
```

### 9.3 資料模型

```sql
CREATE TABLE environments (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    name            VARCHAR(255),          -- dev/staging/production
    pipeline_id     BIGINT,                -- 關聯的 Pipeline
    cluster_id      BIGINT,
    namespace       VARCHAR(255),
    order_index     INT,                   -- 晉升順序
    auto_promote    BOOLEAN DEFAULT FALSE,
    approval_required BOOLEAN DEFAULT FALSE,
    approver_ids    TEXT                   -- 核准人 user_id 清單 JSON
);
```

---

## 10. 安全掃描整合（Trivy）

### 10.1 掃描觸發時機（完整策略）

| 時機 | 方式 | 實作狀態 |
|------|------|---------|
| Pipeline 執行時（trivy-scan Step）| 內建 Step，K8s Job 執行 | M13 實作 |
| Pod 上線時（Informer 自動觸發）| `OnAdd/OnUpdate` 回調 | 近期過渡方案 B |
| GitLab CI 推送結果 | API `POST /security/scans` | 近期過渡方案 A |
| 使用者手動觸發 | 安全掃描頁面 | ✅ 已實作 |
| 定期重掃 | Cron goroutine（每 24h）| 建議未來加入 |

### 10.2 掃描結果關聯

```
PipelineRun
  └─ StepRun（trivy-scan）
        └─ scan_result_id → ImageScanResult
                              ├─ CVE 清單
                              ├─ CRITICAL / HIGH / MEDIUM / LOW 計數
                              └─ 原始 trivy JSON
```

從 Pipeline Run 詳情頁可直接跳轉到完整 CVE 清單，不需要切換到安全掃描頁面另行查找。

### 10.3 掃描失敗處理策略

```yaml
# Pipeline Step 設定
- name: trivy-scan
  type: trivy-scan
  config:
    severity_threshold: HIGH      # CRITICAL 或 HIGH 存在則失敗
    ignore_unfixed: true          # 忽略尚無修補版本的漏洞
    exit_code_on_failure: 1
  on_failure: abort               # 終止整條 Pipeline
  notifications:
    - channel: dingtalk           # 失敗時通知渠道
      template: "映像 {{.Image}} 掃描失敗，{{.CriticalCount}} 個 CRITICAL 漏洞"
```

---

## 11. 通知與告警整合

Pipeline 事件複用 Synapse 現有通知系統（AlertManager / DingTalk / Email）：

| 事件 | 通知類型 | 建議渠道 |
|------|---------|---------|
| Pipeline 觸發成功 | Info | （可選，避免噪音）|
| Pipeline 步驟失敗 | Warning | DingTalk / Email |
| Trivy 掃描發現 CRITICAL | Critical | DingTalk + Email |
| Pipeline 整體失敗 | Error | DingTalk + Email |
| Approval 等待審核 | Info | 審核人 DingTalk / Email |
| 環境 Drift 偵測 | Warning | DingTalk |
| 部署到 Production | Info | 稽核日誌 + 通知 |

---

## 12. 資料模型總覽

```
pipelines ──────────────────────────────────────────┐
    │ 1:N                                            │
pipeline_runs ────────────────────────────────┐     │
    │ 1:N                                     │     │
step_runs                                     │     │
    │ 0:1                                     │     │
    └─→ image_scan_results                    │     │
                                              │     │
environments ──────────────────────── FK ────┘     │
    │ FK                                            │
clusters                                           │
                                                   │
git_providers ──────────────────────── FK ─────────┘
    │
gitops_apps

registries ←── pipeline steps（push-image）
```

---

## 13. 技術選型

| 需求 | 選擇 | 理由 |
|------|------|------|
| CI 執行引擎 | K8s Job（原生）| 零額外元件，已是現有依賴；複雜場景可接 Tekton |
| 映像建置 | Kaniko | 無需 Docker daemon，在 K8s Pod 內安全執行 |
| Steps 間產物共享 | emptyDir（同 Node）/ PVC（跨 Node）| 簡單場景用 emptyDir，需持久化時用 PVC |
| 日誌串流 | SSE（Server-Sent Events）| 複用 Terminal 樣式，單向串流適合日誌場景 |
| Git Webhook 驗證 | 自實作 HMAC handler | 各 Provider 格式差異不大，無需引入外部 SDK |
| GitOps Diff 引擎 | `k8s.io/apimachinery` strategic merge | 已是現有依賴，語義正確 |
| Kustomize 支援 | `sigs.k8s.io/kustomize/api` Go SDK | 無需主機安裝 kustomize 二進位 |
| Registry API | 標準 Docker Registry HTTP API v2 | Harbor / DockerHub / ECR 均相容 |
| 進階 Pipeline（插件）| Tekton Pipelines | 複雜 DAG、特殊 build 工具場景 |

---

## 14. 實作路線圖

### 近期（不依賴 M13，2–3 週）

- [ ] 近期過渡方案 A：GitLab CI 推送掃描結果 API 文件與測試
- [ ] 近期過渡方案 B：Informer Pod OnAdd/OnUpdate 自動觸發 Trivy 掃描
- [ ] 定期重掃 Cron goroutine（每 24h 重掃現有映像）

### M13 — CI Pipeline 引擎（8 週）

**Week 1–2：核心資料模型與執行引擎**
- [ ] `pipelines` / `pipeline_runs` / `step_runs` 資料表建立
- [ ] Pipeline CRUD API
- [ ] 基本執行引擎（K8s Job 提交 + Watch 狀態）

**Week 3–4：內建 Step 類型**
- [ ] `build-jar`（Maven / Gradle）
- [ ] `build-image`（Kaniko）
- [ ] `trivy-scan`（掃描結果關聯 ImageScanResult）
- [ ] `push-image`（Harbor push）
- [ ] `deploy`（kubectl apply）

**Week 5–6：日誌串流與前端**
- [ ] SSE 日誌串流 API
- [ ] `PipelineList.tsx`
- [ ] `PipelineEditor.tsx`（步驟卡片 + YAML 雙模式）
- [ ] `PipelineRunDetail.tsx`（DAG 進度圖）
- [ ] `StepLogViewer.tsx`

**Week 7–8：通知整合與測試**
- [ ] Pipeline 事件通知（DingTalk / Email）
- [ ] 手動觸發完整 E2E 測試
- [ ] 文件更新

### M14 — Git Webhook（4 週）

- [ ] Webhook Receiver（GitLab / GitHub / Gitea）
- [ ] HMAC 驗證
- [ ] Pipeline 觸發條件設定 UI
- [ ] Git Provider 連線設定頁面

### M15 — Registry 整合（3 週）

- [ ] Harbor API 整合
- [ ] Repository / Tag 瀏覽 UI
- [ ] Tag 保留策略
- [ ] Pipeline imagePullSecret 自動注入

### M16 — 原生 GitOps（6 週）

- [ ] GitOpsApp CRUD
- [ ] Diff 引擎（YAML manifest / Kustomize / Helm）
- [ ] Auto Sync / Drift 通知
- [ ] ArgoCD 代理整合保留

### M17 — 環境流水線（5 週）

- [ ] Environment 資料模型與 CRUD
- [ ] Promotion 邏輯（自動 / 人工審核）
- [ ] 複用 Approval Workflow
- [ ] Production Gate 通知

---

**總估計工作量：M13（8W）+ M14（4W）+ M15（3W）+ M16（6W）+ M17（5W）= 26 週**
（可並行部分：M14 可在 M13 Week 5 後開始；M15 可在 M13 Week 3 後開始）
