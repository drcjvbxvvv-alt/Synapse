# Synapse 系統規劃書

> 版本：v1.4 | 日期：2026-04-03 | 狀態：進行中
> 已完成項目請見 [COMPLETED.md](./COMPLETED.md)

---

## 目錄

1. [系統現況總覽](#1-系統現況總覽)
2. [待解決技術債](#2-待解決技術債)
3. [邊界天花板分析](#3-邊界天花板分析)
4. [待實作優化](#4-待實作優化)
5. [待實作功能](#5-待實作功能)
6. [里程碑規劃](#6-里程碑規劃)
7. [平台演進方向：全能 CI/CD DevOps 平台](#7-平台演進方向全能-cicd-devops-平台)
8. [附錄](#8-附錄)

---

## 1. 系統現況總覽

Synapse 是以 Go 1.25（Gin）+ React 19（Ant Design）構建的企業級 Kubernetes 多叢集管理平台。後端以單一二進位檔嵌入前端靜態資源，支援 SQLite（開發）與 MySQL 8（生產）雙資料庫，整合 Prometheus / Grafana / AlertManager / ArgoCD，提供 Web Terminal（Pod Exec、kubectl、Node SSH）。

**目前實作的主要功能：**

| 領域 | 功能 |
|------|------|
| 叢集管理 | 多叢集匯入（kubeconfig / Token）、健康狀態、總覽指標 |
| 工作負載 | Deployment / StatefulSet / DaemonSet / Job / CronJob / Argo Rollouts |
| 自動擴縮 | HPA CRUD、VPA 支援（動態 client）、PDB 管理 |
| 設定管理 | ConfigMap / Secret CRUD + 版本歷史 + 回滾 |
| 網路管理 | Service / Ingress CRUD、NetworkPolicy CRUD + 拓撲圖 + 建立精靈 |
| 儲存管理 | PVC / PV / StorageClass |
| 命名空間 | 建立、ResourceQuota / LimitRange CRUD、刪除、保護機制 |
| 使用者 RBAC | 多租戶、叢集 / 命名空間粒度、LDAP 整合 |
| 監控告警 | Prometheus 指標、Grafana 儀表板、AlertManager、K8s Event 告警規則 |
| GitOps | ArgoCD 應用管理與同步、Argo Rollouts 操控 |
| 日誌 | 操作日誌、Web Terminal 指令稽核、Loki / Elasticsearch 外部查詢、SIEM 匯出 |
| 全域搜尋 | 跨叢集資源搜尋、跨叢集工作負載視圖、Image Tag 全域搜尋 |
| AI 運維 | AI 診斷、多 Provider、NL Query、YAML 生成、Runbook 自動附加 |
| 安全 | AES-256-GCM 加密、Rate Limiting、Login 鎖定、WebSocket Origin 驗證 |
| 稽核 | 操作稽核日誌、Terminal 會話回放、部署審批工作流、SIEM Webhook 推送 |
| 成本 | 資源成本分析（Prometheus + fallback）、CSV 匯出 |
| 合規 | Trivy 映像掃描、CIS kube-bench、Gatekeeper 違規統計 |
| Port-Forward | 後端 SPDY tunnel、活躍 session 管理 |
| CI/CD | Helm Release 管理 |
| 國際化 | zh-TW、en-US、zh-CN |

---

## 2. 待解決技術債

> **所有技術債已於 2026-04-03 完成，本章節保留供記錄。** 詳細說明見 [COMPLETED.md](./COMPLETED.md#4-已修復缺陷)。

---

## 3. 邊界天花板分析

### 3.1 規模上限

| 維度 | 目前天花板 | 根本原因 | 改善方向 |
|------|-----------|---------|---------|
| **叢集數量** | ~20 個 | 每叢集建立獨立 Informer（記憶體 O(n) 增長），Goroutine 洩漏風險 | Informer 池化 + Lazy 初始化 + 閒置叢集 GC |
| **單叢集 Pod 數** | ~5,000 個 | Informer 全量快取於記憶體，列表頁一次回傳 | 分頁快取 + 伺服器端分頁 |
| **並行 Web Terminal** | ~50 個 | 每個 Terminal 佔用 goroutine + WebSocket 連線 | 連線池 + 心跳管理 + 閒置超時 |
| **Log 串流** | 依 K8s API 上限 | 直接 proxy K8s log stream，無緩衝 | 引入 log 中間緩衝層（如 Loki） |
| **並行 API 請求** | ~200 QPS | 無 rate limit，K8s client 無連線池設定 | 限流 + K8s client 連線池調優 |
| **資料庫規模** | SQLite ~1GB / MySQL 無硬限 | 操作日誌、稽核日誌無分區 | 日誌表按月分區 + 資料保留策略 |

### 3.2 功能邊界

| 功能領域 | 現有邊界 | 說明 |
|---------|---------|------|
| **CI/CD Pipeline** | 無 | 依賴外部 ArgoCD，無原生 Pipeline |
| **多租戶隔離** | 命名空間粒度 | 無跨叢集租戶策略（Project 概念待實作） |
| **叢集生命週期** | 無 | 不支援叢集佈建（僅匯入已有叢集） |
| **成本分析** | CPU/MEM 請求量 | 實際用量需 Prometheus；無 PVC 成本 |
| **備份還原** | 無 | 無 etcd 備份 / Velero 整合（M10 延後） |

---

## 4. 待實作優化

> **所有效能優化已於 2026-04-03 完成。** 詳細說明見 [COMPLETED.md](./COMPLETED.md#6-已完成效能優化)。

---

## 5. 待實作功能

> **M11 已於 2026-04-03 完成。** 詳細說明見 [COMPLETED.md](./COMPLETED.md#m11networkpolicy-拓撲內聯編輯--策略模擬--2026-04-03)。

---

### 5.2 多叢集工作流程（M8，5 週）

> **目標：** 打通叢集間協作壁壘，支援工作負載遷移與配置同步。

**待實作任務：**

| 任務 | 檔案 | 週次 |
|------|------|------|
| `SyncPolicy` 資料模型 | `internal/models/sync_policy.go` | W1 |
| 配置同步 API（CRUD + 觸發） + Worker | `internal/services/sync_service.go` | W1–W2 |
| 工作負載遷移後端邏輯（取 YAML → 目標叢集 Apply） | `internal/handlers/workload_migrate.go` | W2–W3 |
| 遷移精靈前端（3 步驟：選叢集 → 資源檢查 → 確認執行） | `ui/src/pages/cluster/MigrateWizard.tsx` | W3–W4 |
| 配置同步管理前端（策略 CRUD + 手動觸發 + 歷史紀錄） | `ui/src/pages/cluster/SyncPolicies.tsx` | W4–W5 |
| 三語 i18n | — | W5 |

**資料模型：**
```go
type SyncPolicy struct {
    ID              uint
    Name            string
    SourceClusterID uint
    SourceNamespace string
    ResourceType    string  // "ConfigMap" / "Secret"
    ResourceNames   string  // JSON 陣列
    TargetClusters  string  // JSON 陣列（叢集 ID）
    ConflictPolicy  string  // "overwrite" / "skip"
    Schedule        string  // Cron 表達式，空字串表示手動
    LastSyncAt      time.Time
}
```

**完成指標：** 可將 staging 叢集的 Deployment 遷移到 production 叢集；ConfigMap 同步至 3 個叢集成功率 100%。

---

> **M12 已於 2026-04-03 完成。** 詳細說明見 [COMPLETED.md](./COMPLETED.md#m12service-mesh-視覺化istio-2026-04-03)。

---

### 5.4 備份與 Velero 整合（M10 附加，延後至 M16 後）

> **決策：** ZIP 匯出已移除（GitOps 取代）。Velero 附加整合保留，待 M16 完成後評估。

- [ ] Velero 安裝偵測（`GET /clusters/:id/backup/velero-status`）
- [ ] Backup/Restore CRD CRUD（複用 CRD 通用介面）
- [ ] 前端備份狀態頁

---

### 5.5 CLI 工具（延後至 M16 後重新規劃）

> **理由：** M13（CI Pipeline）、M14（Git 整合）、M16（GitOps）完成前，CLI 核心使用場景（`pipeline run`、`deploy`、`env promote`）尚未存在，現在設計必然大幅重工。

**技術方案：** `cobra` + `viper`，獨立 Go 二進位，`~/.synapse/config.yaml`
**估計工作量（M16 後）：** 4 週

---

### 5.6 Project 多租戶模型（獨立 Sprint）

> **現況：** 多租戶透過 `ClusterPermission` 實現，無明確的租戶/組織層級，大規模管理困難。

**方向：**
- 引入 **Project（專案）** 概念：一個 Project 對應一組 叢集+命名空間+成員
- Project 管理員可自助管理成員和配額
- 命名空間自助申請流程（Dev 申請 → 管理員審核 → 自動建立 + 配額）

**估計工作量：** 4 週（架構層面升級，需獨立 Sprint）

---

## 6. 里程碑規劃

### 功能完成狀態總覽

| 里程碑 | 功能 | 狀態 | 優先級 | 估計工作量 |
|--------|------|------|--------|-----------|
| M1 | 安全強化 | ✅ 已完成 | — | — |
| M2 | 穩定性與效能 | ✅ 已完成 | — | — |
| M3 | 可觀測性 | ✅ 已完成 | — | — |
| M4 | Helm Release 管理 | ✅ 已完成 | — | — |
| M5 | AI 診斷 + CRD + NetworkPolicy + Event 告警 | ✅ 已完成 | — | — |
| M6 | 資源成本分析 | ✅ 已完成 | — | — |
| M7 | AI 深度運維 | ✅ 已完成 | — | — |
| M8 | **多叢集工作流程** | 🔲 待實作 | 🟢 低 | 5 週 |
| M9 | 合規性與安全掃描 | ✅ 已完成 | — | — |
| M10 | ~~備份匯出 + CLI 工具~~ → Velero 附加（M16 後）+ CLI（M16 後重新規劃） | ⏸ 延後 | 低 | 重新評估 |
| M11 | NetworkPolicy 拓撲內聯編輯 + 策略模擬 | ✅ 已完成 | — | — |
| M12 | Service Mesh 視覺化（Istio） | ✅ 已完成 | — | — |
| M13 | **原生 CI Pipeline 引擎** | 🔲 待實作 | 🔴 高 | 8 週 |
| M14 | **Git 整合 + Webhook 觸發** | 🔲 待實作 | 🔴 高 | 4 週 |
| M15 | **映像 Registry 整合** | 🔲 待實作 | 🟡 中 | 3 週 |
| M16 | **原生輕量 GitOps** | 🔲 待實作 | 🟡 中 | 6 週 |
| M17 | **環境管理 + Promotion 流水線** | 🔲 待實作 | 🟢 低 | 5 週 |

**待實作總估計：約 31 週（M8 + M13–M17）**

### 建議實作順序

```
現在（管理平台）
    │ M11 ✅ NetworkPolicy 模擬
    │ M12 ✅ Service Mesh 視覺化
    ▼
M13 CI Pipeline 引擎（8 週，最大缺口，平台演進關鍵）
    │
    ▼
M14 Git Webhook（4 週，CI 自動化觸發）
    │
    ▼
M15 Registry 整合（3 週，Pipeline 產物管理）
    │
    ▼
M16 原生 GitOps（6 週，CD 能力內建化）
    │
    ▼
M17 環境流水線（5 週，企業多環境 Promotion Gate）
    │
    ▼
目標（全能 CI/CD DevOps 平台）
```

---

## 7. 平台演進方向：全能 CI/CD DevOps 平台

> **戰略目標：** 從「K8s 多叢集管理工具」演進為「端到端 CI/CD DevOps 平台」，具備與 GitLab CI + ArgoCD + Rancher 組合相競爭的完整能力，以單一二進位、零外部依賴為核心競爭優勢。

### 7.1 現況差距分析

| 能力維度 | 現況 | 差距 |
|---------|------|------|
| GitOps / CD | 代理外部 ArgoCD（需額外安裝） | 無原生 CD |
| CI Pipeline | **完全沒有** | 最大缺口 |
| Git 整合 | 無 | 無 Webhook、無 Repo 連結 |
| 映像建置 / Registry | 無 | 無 Build 能力、無 Registry 管理 |
| 環境流水線 | 僅 Namespace 粒度 | 無 dev → staging → prod 概念 |

### 7.2 架構路線

**採用混合路線（C）：** 原生輕量 Pipeline 覆蓋 80% 使用場景（Build → Push → Deploy），進階場景透過插件接入 Tekton/Jenkins。

### 7.3 M13 — 原生 CI Pipeline 引擎（8 週）

> 以 K8s Job / Pod 作為 Pipeline 執行單元，定義儲存在 Synapse DB，執行時動態建立 K8s Job。

**資料模型：** `Pipeline`（定義，DAG steps）→ `PipelineRun`（執行記錄）→ `StepRun`（步驟狀態 + K8s Job 對應）

**執行引擎：**
```
1. 建立 PipelineRun 記錄
2. 解析步驟 DAG（依賴關係）
3. 按拓撲序依次提交 K8s Job（image/command/env/resource limits/workspace PVC）
4. Watch Job 狀態，即時更新 StepRun
5. Job 完成後串流 Pod 日誌
6. 所有步驟成功 → success；任一失敗 → 取消後續，failed
```

**API：**
```
GET/POST /pipelines                           Pipeline CRUD
GET      /pipelines/:id/runs                  執行歷史
POST     /pipelines/:id/run                   手動觸發
GET      /pipelines/:id/runs/:runId/steps/:step/logs  步驟日誌（SSE 串流）
POST     /pipelines/:id/runs/:runId/cancel    取消執行
```

**前端：**
```
ui/src/pages/pipeline/
  ├── PipelineList.tsx         列表（狀態燈、最後執行時間）
  ├── PipelineEditor.tsx       步驟卡片 + YAML 雙模式編輯器
  ├── PipelineRunDetail.tsx    DAG 進度圖 + 步驟狀態
  └── StepLogViewer.tsx        步驟日誌串流（SSE，複用 Terminal 樣式）
```

### 7.4 M14 — Git 整合 + Webhook 觸發（4 週）

**支援 Provider：** GitHub（App/PAT）、GitLab（Webhook Token）、Gitea（自架優先）

**Webhook 流程：**
```
Git Push → POST /webhooks/:provider/:token
  → 驗證 HMAC signature
  → 比對 Pipeline 的 GitRepo + GitBranch（glob）
  → 建立 PipelineRun（TriggerBy="webhook:sha"）
  → 回傳 202 Accepted
```

### 7.5 M15 — 映像 Registry 整合（3 週）

**支援：** Harbor（首選）、Docker Hub、阿里雲 / AWS ECR / GCR（標準 Docker Registry API v2）

**功能：** Registry 連線設定、Repository + Tag 瀏覽、Tag 保留策略、漏洞掃描觸發、Pipeline 步驟自動注入 `imagePullSecret`

### 7.6 M16 — 原生輕量 GitOps（6 週）

**Layer 1（內建）：** 定義 GitOpsApp（Git Repo + 路徑 + 目標叢集）→ 定期 Diff → Auto Sync 或 Drift 通知，支援 Kustomize overlay 和 Helm Chart。

**Layer 2（升級）：** 現有 ArgoCD 代理保留；新增 ArgoCD App Health 聚合到主儀表板；Pipeline 部署步驟可選「觸發 ArgoCD Sync」。

### 7.7 M17 — 環境管理 + Promotion 流水線（5 週）

**環境概念：** `dev → staging → production`，每個環境對應叢集 + 命名空間 + 自動/人工 Promote 策略。

**Promotion 流程：**
```
Pipeline 執行成功 → 部署到 dev → 自動（或等待審核）Promote to staging
  → smoke test（可選）→ 人工審核（Production Gate）→ 部署到 production
```

---

## 8. 附錄

### 技術選型備選

| 需求 | 第一選擇 | 備選 | 備註 |
|------|---------|------|------|
| 狀態管理 | @tanstack/react-query | SWR | React Query 生態更完整 |
| 拓撲圖 | ReactFlow v12 | @antv/g6 | ReactFlow 對 React 整合更佳，內建 Dagre 佈局 |
| 日誌系統 | `slog`（標準庫） | zap | Go 1.21+ slog 是官方解 |
| 追蹤 | OpenTelemetry | Jaeger SDK | OTel 為業界標準 |
| NP 策略模擬 | 自實作 Go selector matching | kube-networkpolicies | K8s NP 語義不複雜，自實作可控無外部依賴 |
| Istio 流量資料 | Prometheus `istio_requests_total` | Kiali API | Prometheus 已為現有依賴；Kiali 需額外部署 |
| CI Pipeline 執行引擎 | K8s Job（原生，零額外元件） | Tekton Pipelines | K8s Job 已是現有依賴 |
| Pipeline 步驟間產物共享 | `emptyDir` / PVC（K8s 原生） | MinIO | 簡單場景用 emptyDir；需持久化時用 PVC |
| Git Provider 整合 | 自實作 Webhook handler | go-github SDK | 各 Provider Webhook 格式差異不大，自實作可控 |
| GitOps Diff 引擎 | `k8s.io/apimachinery` strategic merge | controller-runtime | 輕量場景無需完整 controller 框架 |
| Kustomize 支援 | `sigs.k8s.io/kustomize/api` | shell exec | Go SDK 無需主機安裝 kustomize 二進位 |
| Container Registry | 標準 Docker Registry HTTP API v2 | go-containerregistry | Harbor 額外 API 單獨呼叫 |
| CLI 框架 | cobra + viper | urfave/cli | cobra 生態最大，kubectl/helm 皆採用 |
