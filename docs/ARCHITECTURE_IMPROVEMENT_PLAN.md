# 現有架構改善方案 — 基於生產集群方案文檔 v1.1 與 CICD_ARCHITECTURE v2.3

> 版本：v1.0 | 日期：2026-04-14 | 狀態：待 Review
> 輸入文件：`k8s集群整体方案文档_v1.1.docx`、`CICD_ARCHITECTURE.md v2.3`

---

## 目錄

1. [分析背景](#1-分析背景)
2. [生產集群方案與 Synapse CICD 架構的對齊差距](#2-生產集群方案與-synapse-cicd-架構的對齊差距)
3. [改善方案一覽](#3-改善方案一覽)
4. [P0 — 必須立即對齊的改善項](#4-p0--必須立即對齊的改善項)
5. [P1 — CICD 架構調整建議](#5-p1--cicd-架構調整建議)
6. [P2 — 運維與治理層改善](#6-p2--運維與治理層改善)
7. [P3 — 未來演進方向](#7-p3--未來演進方向)
8. [修訂 CICD_ARCHITECTURE.md 的具體建議](#8-修訂-cicd_architecturemd-的具體建議)
9. [風險評估](#9-風險評估)
10. [改善優先級矩陣](#10-改善優先級矩陣)

---

## 1. 分析背景

**生產集群方案文檔 v1.1** 定義了一套完整的 Kubernetes 生產環境架構：

- **基礎設施**：3 Master + HAProxy、Rocky Linux 10.1、containerd 2.2.1
- **網路**：Cilium 1.19.1（eBPF、kubeProxyReplacement）
- **服務治理**：Istio 1.29.0 Ambient 模式
- **入口**：Istio Gateway + Gateway API 1.5.0
- **存儲**：vSphere CSI 3.6.0
- **可觀測**：Prometheus + Grafana 12.0.1 + Kiali
- **發布**：ArgoCD v3.3.3 + Argo Rollouts v1.8.4（GitOps + 灰度發布）

**CICD_ARCHITECTURE.md v2.3** 設計了 Synapse 從 K8s 管理工具演進為端到端 DevSecOps 平台的路線（M13–M17）。

本文檔分析兩份文件的**交集、衝突與缺口**，制定改善方案。

---

## 2. 生產集群方案與 Synapse CICD 架構的對齊差距

### 2.1 已對齊的部分

| 維度 | 生產方案 | CICD 架構 | 狀態 |
|------|---------|----------|------|
| K8s Job 作為 CI 執行單元 | K8s 1.34.4 原生 Job | ADR-001 確認 K8s Job 引擎 | ✅ 對齊 |
| 動態存儲 | vSphere CSI PVC | §7.4 Workspace PVC 模式 | ✅ 對齊 |
| 多叢集管理 | 3 Master HA | §7.5 跨叢集執行路徑 | ✅ 對齊 |
| 可觀測 | Prometheus + Grafana | §16 Observability metrics | ✅ 對齊 |
| ArgoCD | ArgoCD v3.3.3 | §12.1 ArgoCD 代理邊界 | ✅ 對齊 |

### 2.2 需要對齊的差距

| # | 差距描述 | 影響級別 | 詳見 |
|---|---------|---------|------|
| G1 | 生產方案採用 **Argo Rollouts** 做灰度發布，CICD 架構的 M16/M17 完全沒提到 Rollouts 整合 | 🔴 高 | §4.1 |
| G2 | 生產方案使用 **Istio Ambient + Gateway API** 做入口，CICD 架構的 deploy Step 只提 `kubectl apply` / `helm upgrade`，未考慮 Gateway API 路由權重更新 | 🔴 高 | §4.2 |
| G3 | 生產方案要求 **Cilium NetworkPolicy**，CICD 架構 §7.7 的 NetworkPolicy 描述過於簡略，未對齊 Cilium 原生 CiliumNetworkPolicy CRD | 🟡 中 | §4.3 |
| G4 | 生產方案明確要求 **etcd 備份**，CICD 架構的災備策略缺少 Pipeline 相關資料的備份計劃 | 🟡 中 | §6.1 |
| G5 | 生產方案強調 **Cilium 與 Istio Ambient 的職責邊界**（`socketLB.hostNamespaceOnly=true` 等），Pipeline Pod 的網路行為可能觸發衝突 | 🔴 高 | §4.4 |
| G6 | 生產方案列出 **Argo Rollouts v1.8.4** 作為版本基線，CICD 架構 M16 的原生 GitOps 與 Rollouts 的能力有重疊和衝突 | 🔴 高 | §5.1 |
| G7 | 生產方案使用 **chrony 時間同步**，CICD 架構的 Webhook Replay Protection 依賴時間戳，未考慮跨節點時鐘偏差 | 🟢 低 | §6.2 |
| G8 | 生產方案的存儲層採用 **WaitForFirstConsumer** 綁定模式，Pipeline Workspace PVC 的調度需要考慮拓撲約束 | 🟡 中 | §4.5 |

---

## 3. 改善方案一覽

```
┌─────────────────────────────────────────────────────────────────────┐
│                        改善方案全景圖                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  P0 — 必須立即對齊                                                   │
│  ├── G1: Argo Rollouts 整合方案                                      │
│  ├── G2: Gateway API 路由權重更新機制                                  │
│  ├── G5: Pipeline Pod 在 Cilium + Istio Ambient 下的網路行為規範       │
│  └── G6: M16 原生 GitOps 與 Rollouts 的邊界重新定義                    │
│                                                                     │
│  P1 — CICD 架構調整                                                  │
│  ├── G3: Pipeline NetworkPolicy 改用 CiliumNetworkPolicy             │
│  ├── G8: Workspace PVC 拓撲約束                                      │
│  └── 灰度發布 Step 類型設計                                           │
│                                                                     │
│  P2 — 運維與治理                                                     │
│  ├── G4: Pipeline 資料備份策略                                        │
│  ├── G7: 時鐘偏差容忍度                                               │
│  └── 版本兼容矩陣維護                                                 │
│                                                                     │
│  P3 — 未來演進                                                       │
│  ├── Canary 分析自動化（Prometheus 指標判定）                           │
│  ├── Flagger 可選整合                                                 │
│  └── Progressive Delivery 儀表板                                     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 4. P0 — 必須立即對齊的改善項

### 4.1 G1: Argo Rollouts 整合方案

**現狀問題：**

生產集群方案明確採用 ArgoCD v3.3.3 + Argo Rollouts v1.8.4 構建灰度發布體系，但 CICD_ARCHITECTURE.md 的部署相關設計（M13 的 `deploy` Step、M16 的原生 GitOps、M17 的 Promotion）完全沒有提到 Argo Rollouts。這意味著：

1. M13 的 `deploy` Step 只能做 Deployment 的 `kubectl apply`，無法觸發 Rollout 的灰度策略
2. M16 原生 GitOps 的 diff 引擎不認識 `Rollout` CRD
3. M17 的 Promotion 流水線與 Rollouts 的 promote 機制存在功能重疊

**改善方案：**

#### 4.1.1 新增 `deploy-rollout` Step 類型（歸入 M13b）

```yaml
steps:
  - name: canary-deploy
    type: deploy-rollout
    config:
      rollout_name: backend-service
      namespace: app-prod
      image: harbor.internal/app/backend:${GIT_SHA}
      strategy: canary              # canary / bluegreen
      auto_promote_delay: 300       # 秒，自動晉升延遲
      analysis:
        enabled: true
        prometheus_url: http://prometheus.monitoring:9090
        success_condition: "result[0] < 0.05"  # 5xx 比率 < 5%
        metric_query: |
          sum(rate(http_requests_total{status=~"5.*",app="backend"}[5m]))
          /
          sum(rate(http_requests_total{app="backend"}[5m]))
```

**實作要點：**
- 使用動態客戶端（`k8s.io/client-go/dynamic`）操作 `argoproj.io/v1alpha1` Rollout CRD
- 需先通過 Observer Pattern 偵測 Argo Rollouts 是否安裝（遵循 CLAUDE.md §8）
- Rollout 狀態 watch 走 `ClusterInformerManager`

#### 4.1.2 ArgoCD + Rollouts 的協同定義

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│  Synapse     │────▶│  ArgoCD      │────▶│  Argo Rollouts   │
│  Pipeline    │     │  (Sync)      │     │  (灰度控制)       │
│              │     │              │     │                  │
│ deploy-      │     │ Application  │     │ Rollout Resource │
│ argocd-sync  │     │ Sync         │     │ Canary/BlueGreen │
└──────────────┘     └──────────────┘     └──────────────────┘
                           │
                     sync 觸發後
                     Rollout Controller
                     自動接管灰度流程
```

**決策建議：** Synapse Pipeline 的部署 Step 負責**觸發**，Argo Rollouts 負責**執行灰度策略**。Synapse 不應重新實作灰度邏輯，而是提供 Rollout 狀態的可視化與 promote/abort 操作。

### 4.2 G2: Gateway API 路由權重更新

**現狀問題：**

生產方案使用 Istio Gateway + Gateway API 管理入口流量。灰度發布時，Argo Rollouts 需要更新 `HTTPRoute` 的 `backendRefs` 權重來實現流量切分。CICD 架構的 deploy Step 沒有考慮這個場景。

**改善方案：**

在 `deploy-rollout` Step 的 Rollout 定義中，明確支持 Gateway API trafficRouting：

```yaml
# Argo Rollout 定義（由 GitOps 管理，Synapse Pipeline 觸發 sync）
spec:
  strategy:
    canary:
      canaryService: backend-canary
      stableService: backend-stable
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: backend-route
            namespace: app-prod
      steps:
        - setWeight: 10
        - pause: { duration: 5m }
        - setWeight: 50
        - pause: { duration: 10m }
        - setWeight: 100
```

**Synapse 層面需要做的：**

1. **Rollout 狀態監控面板**：在 Pipeline Run 詳情頁展示灰度進度（權重百分比、當前步驟、分析結果）
2. **手動操作按鈕**：Promote（確認全量）、Abort（回滾）、Pause/Resume
3. **API 端點**：
   ```
   POST /clusters/:clusterID/rollouts/:namespace/:name/promote
   POST /clusters/:clusterID/rollouts/:namespace/:name/abort
   POST /clusters/:clusterID/rollouts/:namespace/:name/retry
   GET  /clusters/:clusterID/rollouts/:namespace/:name/status
   ```

### 4.3 G3: Pipeline NetworkPolicy 對齊 Cilium

**現狀問題：**

CICD 架構 §7.7 提到「NetworkPolicy（在目標 Namespace 套用）」，但生產方案使用 Cilium，應優先使用 `CiliumNetworkPolicy` 以獲得 L7 策略能力。

**改善方案：**

Pipeline JobBuilder 應根據叢集網路插件自動選擇 NetworkPolicy 類型：

```go
// 偵測邏輯
func (b *JobBuilder) selectNetworkPolicyKind(ctx context.Context, clientset kubernetes.Interface) string {
    // 優先使用 CiliumNetworkPolicy（L7 能力）
    _, err := clientset.Discovery().ServerResourcesForGroupVersion("cilium.io/v2")
    if err == nil {
        return "CiliumNetworkPolicy"
    }
    // 降級為標準 NetworkPolicy
    return "NetworkPolicy"
}
```

### 4.4 G5: Pipeline Pod 在 Cilium + Istio Ambient 環境的網路行為

**現狀問題：**

生產方案特別強調 Cilium 與 Istio Ambient 的兼容參數（`socketLB.hostNamespaceOnly=true`、`cni.exclusive=false`、`l7Proxy=false`）。Pipeline Pod 如果被納入 Istio Ambient mesh，可能導致：

1. Pipeline Pod 的網路流量被 ztunnel 攔截，增加延遲
2. 與外部 Registry（Harbor）的連線可能受 mTLS 影響
3. DNS 解析路徑改變

**改善方案：**

#### 4.4.1 Pipeline Namespace 排除 Ambient Mesh

```yaml
# Pipeline 執行命名空間的 label
apiVersion: v1
kind: Namespace
metadata:
  name: synapse-pipeline-jobs
  labels:
    istio.io/dataplane-mode: none    # 排除 Ambient mesh
    purpose: pipeline-execution
```

**在 JobBuilder 中確保：**

```go
// JobBuilder 建立 Job 時，確認目標 namespace 有正確的 label
func (b *JobBuilder) ensurePipelineNamespaceLabels(ctx context.Context, ns string) error {
    // 如果 Pipeline 在專屬 namespace 執行，確保排除 mesh
    // 如果在業務 namespace 執行，不修改（尊重現有 mesh 配置）
}
```

#### 4.4.2 在 CICD 架構中新增注意事項

Pipeline Pod 的 SecurityContext 章節（§7.7）需新增：

> **Istio Ambient 環境注意：** 若 Pipeline Job 執行於已納入 Ambient mesh 的 Namespace，build-image（Kaniko）和 push-image 步驟可能因 ztunnel 的 mTLS 攔截而連不到外部 Registry。建議：
> 1. 使用專屬的 `synapse-pipeline-jobs` namespace 並排除 mesh
> 2. 或在 Pipeline Pod 上加 annotation `ambient.istio.io/redirection: disabled`

### 4.5 G8: Workspace PVC 拓撲約束

**現狀問題：**

生產方案的 vSphere CSI 使用 `WaitForFirstConsumer` 綁定模式，表示 PVC 只在 Pod 調度到具體 Node 後才綁定。Pipeline 的多 Step 場景（1 Step = 1 K8s Job）中，後續 Step 的 Job 必須調度到**與 PVC 相同的 Node**。

**改善方案：**

```go
// JobBuilder 中增加 nodeAffinity 確保同一 Run 的 Jobs 調度到同一 Node
func (b *JobBuilder) buildJobSpec(run *PipelineRun, step *StepDef) *batchv1.Job {
    // 如果使用 PVC workspace，加上 Node 親和性
    if run.WorkspaceType == "pvc" && run.BoundNodeName != "" {
        job.Spec.Template.Spec.NodeSelector = map[string]string{
            "kubernetes.io/hostname": run.BoundNodeName,
        }
    }
}
```

**或者使用 `ReadWriteMany` 存儲（如果 vSphere CSI 支持）：**

Pipeline 的 Workspace PVC 應在文檔中明確：
- vSphere CSI 的 `ReadWriteOnce` PVC 需要 Node 親和性約束
- 若集群有 NFS/CephFS 等支持 `ReadWriteMany` 的 StorageClass，優先使用
- 在 Pipeline 定義中允許指定 `storageClassName`

---

## 5. P1 — CICD 架構調整建議

### 5.1 G6: M16 原生 GitOps 與 Argo Rollouts 的邊界重新定義

**核心問題：**

CICD 架構 M16 設計了「原生輕量 GitOps」（Diff 引擎 + Auto Sync + Drift Detection），生產方案已有 ArgoCD v3.3.3。兩者功能重疊嚴重。加上 Argo Rollouts，三者的職責邊界需要重新定義。

**建議調整：**

| 場景 | 推薦方案 | 原因 |
|------|---------|------|
| 已有 ArgoCD + Rollouts 的生產環境 | **不啟用 M16 原生 GitOps**，僅做 ArgoCD + Rollouts 的代理與可視化 | 避免雙控制器衝突 |
| 輕量環境、不想裝 ArgoCD | 啟用 M16 原生 GitOps，但不支持灰度（只做 `kubectl apply`） | 簡化場景 |
| 需要灰度但不想裝 ArgoCD | M16 原生 GitOps + **Argo Rollouts 獨立使用**（不經 ArgoCD） | Rollouts 可獨立於 ArgoCD 運行 |

**CICD 架構修訂建議：**

在 §12.1 的邊界表中新增 Argo Rollouts 行：

```markdown
| 場景 | ArgoCD | M16 原生 GitOps | Argo Rollouts | Synapse 角色 |
|------|--------|----------------|---------------|-------------|
| 全功能生產環境 | ✅ 已裝 | ❌ 不啟用 | ✅ 已裝 | 代理 + 可視化 + 觸發 |
| 精簡環境（無灰度） | ❌ | ✅ 啟用 | ❌ | 原生 GitOps |
| 精簡 + 灰度 | ❌ | ✅ 啟用 | ✅ 獨立裝 | 原生 GitOps + Rollout 狀態監控 |
```

### 5.2 新增 `deploy-rollout` Step 類型規格

歸入 M13b 或作為 M13c 獨立交付（建議 2 週）：

**新增 Step 類型：**

| Step 類型 | 說明 | 依賴 |
|-----------|------|------|
| `deploy-rollout` | 更新 Rollout 的 image，觸發灰度發布流程 | Argo Rollouts CRD |
| `rollout-promote` | 手動或自動 promote 灰度到下一步 | Argo Rollouts CRD |
| `rollout-abort` | 中止灰度、回滾到 stable | Argo Rollouts CRD |
| `rollout-status` | 等待 Rollout 達到目標狀態（作為 pipeline 閘門） | Argo Rollouts CRD |

**資料模型擴展：**

`step_runs` 表新增：

```sql
ALTER TABLE step_runs ADD COLUMN rollout_status VARCHAR(50);  -- healthy/degraded/paused/progressing
ALTER TABLE step_runs ADD COLUMN rollout_weight INT;           -- 當前灰度權重百分比
```

### 5.3 灰度發布與 Istio Gateway API 的整合路徑

生產方案使用 Gateway API + Argo Rollouts 的 Gateway API 插件實現流量切分。Synapse 需要能夠展示這些資訊：

**新增 API 端點（歸入現有 Network/Gateway 模組）：**

```
GET /clusters/:clusterID/rollouts                              # 列出所有 Rollout
GET /clusters/:clusterID/rollouts/:namespace/:name             # Rollout 詳情（含灰度進度）
GET /clusters/:clusterID/rollouts/:namespace/:name/analysis    # 分析結果
```

**前端面板設計：**

```
┌─────────────────────────────────────────────────────────┐
│  Rollout: backend-service                    Canary     │
│  ─────────────────────────────────────────────────────── │
│                                                         │
│  Stable (v2.1.0)  ████████████████████░░░░  80%         │
│  Canary (v2.2.0)  ████░░░░░░░░░░░░░░░░░░░  20%         │
│                                                         │
│  步驟: [10%] → [20% ←當前] → [50%] → [100%]             │
│                                                         │
│  Analysis:                                              │
│  ✅ 5xx-rate: 0.02% (< 5% threshold)                   │
│  ✅ latency-p99: 230ms (< 500ms threshold)              │
│                                                         │
│  [Promote 全量] [Pause] [Abort 回滾]                     │
└─────────────────────────────────────────────────────────┘
```

---

## 6. P2 — 運維與治理層改善

### 6.1 G4: Pipeline 資料備份策略

生產方案要求建立 etcd 定期備份機制。CICD 架構新增了大量 DB 表（pipelines、pipeline_versions、pipeline_runs 等），需要配套的備份策略。

**建議：**

| 資料類型 | 備份頻率 | 保留期 | 方式 |
|---------|---------|-------|------|
| `pipelines` + `pipeline_versions` | 每日 | 90 天 | DB logical dump |
| `pipeline_secrets`（加密態） | 每日 | 90 天 | DB logical dump（已加密，安全） |
| `pipeline_runs` + `step_runs` | 每週 | 30 天 | DB logical dump + 舊資料歸檔 |
| `pipeline_logs` | 不備份 | 依 retention 策略（預設 30 天） | 超期自動清理 |
| Workspace PVC | 不備份 | Run 結束即釋放 | N/A |

### 6.2 G7: Webhook 時鐘偏差容忍度

生產方案使用 chrony 同步時間，但跨節點仍可能有毫秒級偏差。Webhook Replay Protection 的 5 分鐘 TTL 已足夠容忍，但建議：

- 在 `ReplayGuard.Check()` 中增加 ±30 秒的時鐘偏差容忍
- 日誌記錄觀測到的最大時鐘偏差，作為運維監控指標

### 6.3 版本兼容矩陣

生產方案強調「統一維護版本兼容矩陣」。建議 CICD 架構新增兼容矩陣章節：

| Synapse 版本 | K8s | Cilium | Istio | ArgoCD | Argo Rollouts | vSphere CSI | Gateway API CRD |
|-------------|-----|--------|-------|--------|---------------|-------------|-----------------|
| v1.x (當前) | 1.28–1.34 | 1.14+ | 1.20+ (Ambient) | v2.8+ | N/A | 3.0+ | 1.0+ |
| v2.x (M13+) | 1.30–1.34 | 1.16+ | 1.24+ (Ambient) | v3.0+ | v1.6+ | 3.4+ | 1.2+ |

---

## 7. P3 — 未來演進方向

### 7.1 Canary 分析自動化

結合生產方案的 Prometheus + Grafana，Pipeline 的 `deploy-rollout` Step 可以在灰度期間自動查詢 Prometheus 判定是否 promote：

```yaml
steps:
  - name: canary-analysis
    type: rollout-status
    config:
      timeout: 30m
      success_criteria:
        - prometheus_query: 'rate(http_requests_total{status=~"5.*"}[5m]) / rate(http_requests_total[5m])'
          threshold: 0.05
          comparison: less_than
        - prometheus_query: 'histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))'
          threshold: 0.5
          comparison: less_than
      on_success: promote
      on_failure: abort
```

### 7.2 Progressive Delivery 儀表板

在 Synapse 的「集中戰情室」中新增 Progressive Delivery 視圖，整合：

- 灰度發布當前狀態（所有叢集的 Rollout 狀態聚合）
- 歷史灰度記錄（成功率、平均灰度時長、最常見回滾原因）
- 與 Kiali 的服務拓撲聯動（灰度期間的流量分佈可視化）

### 7.3 Header-Based 灰度路由

生產方案提到「結合網關請求 Header 實現用戶級灰度發布」。Synapse 可以在前端提供：

- 灰度規則設定 UI（匹配 Header → 路由到 Canary）
- 灰度測試入口（生成帶特定 Header 的 curl 命令）
- 灰度用戶名單管理

---

## 8. 修訂 CICD_ARCHITECTURE.md 的具體建議

### 8.1 新增章節

| 建議新增章節 | 位置 | 內容 |
|------------|------|------|
| §12.2.5 Argo Rollouts 整合 | M16 章節內 | Rollout CRD 操作、狀態監控、promote/abort API |
| §7.7.1 Istio Ambient 環境注意事項 | Pod Security Baseline 章節後 | mesh 排除、ztunnel 影響、annotation 配置 |
| §7.4.1 vSphere CSI 拓撲約束 | Workspace 章節後 | WaitForFirstConsumer 對多 Step PVC 的影響 |
| §19.1 版本兼容矩陣 | 技術選型章節後 | 各組件最低 / 推薦版本 |
| §9.1.1 deploy-rollout Step 類型 | M13b 進階 Steps | Rollout 觸發、狀態等待、自動分析 |

### 8.2 修訂現有章節

| 章節 | 修訂內容 |
|------|---------|
| §3 與現有系統的關係 | 新增 Argo Rollouts 行：Pipeline `deploy-rollout` Step 觸發 Rollout，Synapse 提供狀態可視化 |
| §4 現況差距分析 | 新增「灰度發布」維度：現況=依賴外部 ArgoCD+Rollouts → 目標=Synapse 提供 Rollout 狀態監控 + promote/abort 操作 |
| §7.7 Pod Security Baseline | NetworkPolicy 部分改為：偵測 Cilium → 使用 CiliumNetworkPolicy；降級為標準 NetworkPolicy |
| §12.1 ArgoCD 邊界 | 擴展表格，新增 Argo Rollouts 列，明確三者（ArgoCD / 原生 GitOps / Rollouts）的共存規則 |
| §17 失敗模式矩陣 | 新增：Rollout 分析失敗 → 自動 abort 後的 Pipeline 狀態處理 |
| §20 實作路線圖 | M13b 新增 `deploy-rollout` Step；或新增 M13c（2 週）專門處理 Rollouts 整合 |
| §22 效能 SLA | 新增 Rollout 狀態查詢延遲目標 |
| ADR | 新增 ADR-010：灰度發布採 Argo Rollouts 代理模式，不自行實作灰度邏輯 |

### 8.3 ADR-010 草案：灰度發布策略

```
ADR-010：灰度發布採 Argo Rollouts 代理模式

狀態：Proposed（2026-04-14）

Context：
  - 生產集群方案已採用 Argo Rollouts v1.8.4 + Gateway API trafficRouting
  - Synapse M16 原生 GitOps 設計了輕量 diff + sync，但沒有灰度能力
  - 自行實作灰度邏輯（權重調整、分析判定、自動回滾）工作量約 8 週，且質量難以匹敵成熟方案

Decision：
  - Synapse 不自行實作灰度發布邏輯
  - 通過動態客戶端操作 Rollout CRD，提供：觸發、狀態監控、promote、abort
  - 灰度策略（canary steps、analysis template）由使用者在 Rollout YAML 中定義
  - Synapse Pipeline 的 deploy-rollout Step 負責更新 image 並等待 Rollout 完成

Alternatives considered：
  - 自行實作灰度（Gateway API 權重更新 + Prometheus 分析）：工時過長、維護負擔大
  - 不支持灰度（只做 kubectl apply）：不符合生產集群方案需求
  - 整合 Flagger：與 Argo Rollouts 功能重疊，增加使用者選擇困惑

Consequences：
  ✅ 復用成熟的灰度引擎，品質有保障
  ✅ 與生產方案完全對齊
  ❌ 依賴 Argo Rollouts CRD 安裝（遵循 Observer Pattern 偵測）
  ❌ 不支持 Rollouts 的環境需降級為普通 Deployment 部署
```

---

## 9. 風險評估

| 風險 | 影響 | 機率 | 緩解措施 |
|------|------|------|---------|
| Argo Rollouts API 跨版本不相容 | Synapse 的 Rollout 操作失敗 | 低 | 支持 v1alpha1 並 watch upstream release notes |
| Pipeline Pod 被 Istio Ambient mesh 攔截 | Build/push 步驟超時或失敗 | 中 | 預設 Pipeline namespace 排除 mesh |
| vSphere CSI PVC 跨 Node 調度失敗 | 多 Step Pipeline workspace 無法共享 | 中 | 文檔明確、JobBuilder 加 nodeSelector |
| M16 原生 GitOps 與 ArgoCD 雙控制器 | 資源被雙方同時 reconcile | 高 | 嚴格互斥：同一 app 不能同時被兩者管理 |
| Cilium NetworkPolicy 與 Pipeline egress 衝突 | Pipeline 無法連到 Registry/Git | 中 | JobBuilder 自動創建必要的 egress 規則 |

---

## 10. 改善優先級矩陣

| 優先級 | 項目 | 預估工時 | 前置依賴 | 建議排程 |
|--------|------|---------|---------|---------|
| **P0-1** | ADR-010 + deploy-rollout Step 設計 | 1 天 | 無 | 立即（Review 通過後） |
| **P0-2** | §12.1 邊界重新定義（ArgoCD / GitOps / Rollouts） | 0.5 天 | P0-1 | 立即 |
| **P0-3** | §7.7.1 Istio Ambient 注意事項 | 0.5 天 | 無 | 立即 |
| **P0-4** | §7.7 NetworkPolicy → CiliumNetworkPolicy 偵測 | 1 天 | 無 | M13a Week 2 |
| **P1-1** | deploy-rollout Step 實作 | 2 週 | M13a 完成 | M13b 或 M13c |
| **P1-2** | Rollout 狀態監控 API + 前端面板 | 1 週 | P1-1 | M13b Week 7 |
| **P1-3** | Workspace PVC 拓撲約束 | 2 天 | M13a Week 2 | M13a Week 3 |
| **P2-1** | Pipeline 資料備份策略文檔 | 0.5 天 | 無 | 隨時 |
| **P2-2** | 版本兼容矩陣 | 0.5 天 | 無 | 隨時 |
| **P3-1** | Canary 分析自動化 | 2 週 | P1-1 | Post-M13 |
| **P3-2** | Progressive Delivery 儀表板 | 2 週 | P1-2 | Post-M13 |

---

## 附錄：文件修訂 Checklist

在 Review 通過後，需要修訂 `CICD_ARCHITECTURE.md` 的以下位置：

- [ ] §3 新增 Argo Rollouts 行
- [ ] §4 新增灰度發布差距行
- [ ] §7.4 新增 PVC 拓撲約束注意事項
- [ ] §7.7 新增 Istio Ambient 注意事項 + CiliumNetworkPolicy 偵測
- [ ] §9.1 新增 deploy-rollout / rollout-promote / rollout-abort / rollout-status Step 類型
- [ ] §12.1 擴展邊界表（ArgoCD / 原生 GitOps / Argo Rollouts 三方）
- [ ] §17 新增 Rollout 相關失敗模式
- [ ] §19 新增版本兼容矩陣
- [ ] §20 路線圖新增 Rollouts 整合任務
- [ ] §21 新增 ADR-010
- [ ] §25 新增 Rollouts 整合任務的模型指派
