# CD 部署設計方案 A — Pipeline 內建 Deploy Step

> 版本：v1.0 | 日期：2026-04-18 | 狀態：待 Review

---

## 1. 設計目標

每個環境一個 Pipeline，構建和部署在同一個 Run 中完成。防止部署錯環境。

### 適用場景

```
saas-dev Pipeline     → build → scan → deploy to dev namespace
saas-staging Pipeline → build → scan → approval → deploy to staging namespace  
saas-prod Pipeline    → build → scan → approval → deploy to prod namespace
```

### 核心原則

| 原則 | 說明 |
|------|------|
| 環境隔離 | 一個 Pipeline 綁定一個環境，不會部署錯 |
| 流程完整 | 一個 Run 完成 build → scan → approve → deploy 全流程 |
| 簡單直覺 | 使用者只需「手動觸發」，不需要額外選擇部署目標 |
| 可差異化 | 不同環境的 Pipeline 可以有不同的 steps（prod 多一道審核） |

---

## 2. 使用者流程

### 2.1 初始設定

使用者為同一個服務建立三個 Pipeline：

```
┌──────────────────────────────────────────────────────┐
│  Pipeline 管理                                        │
│                                                       │
│  saas-dev      v3  構建: docker-desktop/dev      [觸發] │
│  saas-staging  v3  構建: docker-desktop/staging  [觸發] │
│  saas-prod     v3  構建: docker-desktop/prod     [觸發] │
└──────────────────────────────────────────────────────┘
```

每個 Pipeline 設定：
- **相同**：關聯 Project（同一個 Git Repo）、build-image config
- **不同**：構建環境（Namespace）、deploy 步驟的目標 Namespace、是否開審核

### 2.2 Pipeline Steps 配置

**saas-dev**（不需審核，直接部署）：
```json
[
  {
    "name": "build",
    "type": "build-image",
    "config": {
      "destination": "192.168.0.137:5100/saas/repo:dev",
      "registry": "local-harbor"
    }
  },
  {
    "name": "deploy",
    "type": "deploy",
    "depends_on": ["build"],
    "config": {
      "manifests": ["k8s/deployment.yaml", "k8s/service.yaml"],
      "namespace": "dev"
    }
  }
]
```

**saas-staging**（需審核）：
```json
[
  {
    "name": "build",
    "type": "build-image",
    "config": {
      "destination": "192.168.0.137:5100/saas/repo:staging",
      "registry": "local-harbor"
    }
  },
  {
    "name": "deploy",
    "type": "deploy",
    "depends_on": ["build"],
    "config": {
      "manifests": ["k8s/deployment.yaml", "k8s/service.yaml"],
      "namespace": "staging"
    }
  }
]
```
+ `approval_enabled = true`（deploy 前自動插入審核）

**saas-prod**（需審核 + 安全掃描必須通過）：
```json
[
  {
    "name": "build",
    "type": "build-image",
    "config": {
      "destination": "192.168.0.137:5100/saas/repo:prod",
      "registry": "local-harbor"
    }
  },
  {
    "name": "deploy",
    "type": "deploy",
    "depends_on": ["build"],
    "config": {
      "manifests": ["k8s/deployment.yaml", "k8s/service.yaml"],
      "namespace": "prod"
    }
  }
]
```
+ `approval_enabled = true` + `scan_enabled = true`

### 2.3 執行流程

```
使用者點擊 saas-prod「觸發」
       ↓
  [build-image] 構建 + 推送 Harbor
       ↓
  [scan-build] 自動安全掃描（scan_enabled）
       ↓
  [approve-deploy] 自動審核閘門（approval_enabled）
       ↓  使用者在 UI 點擊「核准」
  [deploy] kubectl apply -n prod
       ↓
  ✅ 完成
```

---

## 3. 技術實作（方案 A 特有的改動）

### 3.1 Deploy Step 的映像替換問題

**問題**：`k8s/deployment.yaml` 中的 `image` 欄位需要替換為剛構建的映像。

**解決方案**：Deploy step 的 command 中自動注入映像替換邏輯。

```bash
# 自動產生的 deploy command
# 1. 替換 manifest 中所有 IMAGE_PLACEHOLDER 為實際映像
sed -i "s|IMAGE_PLACEHOLDER|192.168.0.137:5100/saas/repo:dev|g" k8s/deployment.yaml

# 2. 或者使用 kubectl set image（不修改檔案）
kubectl apply -f k8s/deployment.yaml -f k8s/service.yaml -n dev
kubectl set image deployment/saas-app saas-app=192.168.0.137:5100/saas/repo:dev -n dev
```

**推薦做法**：在 `DeployConfig` 中新增 `image` 欄位，deploy step 自動執行 `kubectl set image`。

```json
{
  "name": "deploy",
  "type": "deploy",
  "config": {
    "manifests": ["k8s/deployment.yaml", "k8s/service.yaml"],
    "namespace": "dev",
    "image": "$BUILD_IMAGE",
    "workload": "deployment/saas-app",
    "container": "saas-app"
  }
}
```

`$BUILD_IMAGE` 在執行時自動替換為同 Run 中 build-image step 的 destination。

### 3.2 Deploy Job 的執行環境

**關鍵問題**：Deploy Job 跑在 `build_cluster_id` + `build_namespace`，但 `kubectl apply` 要操作目標 Namespace。

**情境分析**：

| 場景 | Build 叢集 | Deploy 目標 | 解法 |
|------|-----------|------------|------|
| 同叢集同 NS | docker-desktop/dev | docker-desktop/dev | kubectl -n dev（直接） |
| 同叢集不同 NS | docker-desktop/ci | docker-desktop/prod | kubectl -n prod（需 RBAC） |
| 不同叢集 | cluster-a/ci | cluster-b/prod | 需注入 kubeconfig（複雜） |

**Phase 1 只支援同叢集**：Deploy Job 跑在 build 叢集，kubectl 指向同叢集的不同 Namespace。

需要的 RBAC：
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: synapse-deployer
subjects:
  - kind: ServiceAccount
    name: default
    namespace: ci-builds   # build_namespace
roleRef:
  kind: ClusterRole
  name: edit              # 或自訂 Role
  apiGroup: rbac.authorization.k8s.io
```

### 3.3 Deploy Step Command 生成改動

修改 `generateDeployCommand`：

```go
func generateDeployCommand(step *StepDef) ([]string, []string) {
    var cfg DeployConfig
    parseJSON(step.Config, &cfg)

    cmd := ""

    // 1. kubectl apply manifests
    cmd += "kubectl apply"
    for _, m := range cfg.GetManifests() {
        cmd += " -f " + m
    }
    if cfg.Namespace != "" {
        cmd += " -n " + cfg.Namespace
    }

    // 2. 如果有 image + workload，自動 set image
    if cfg.Image != "" && cfg.Workload != "" {
        container := cfg.Container
        if container == "" {
            container = cfg.Workload  // 預設 container 名 = workload 名
        }
        cmd += fmt.Sprintf(" && kubectl set image %s %s=%s -n %s",
            cfg.Workload, container, cfg.Image, cfg.Namespace)
    }

    return []string{"/bin/sh", "-c", cmd}, nil
}
```

### 3.4 DeployConfig 擴充

```go
type DeployConfig struct {
    Manifest  string   `json:"manifest"`
    Manifests []string `json:"manifests,omitempty"`
    Namespace string   `json:"namespace"`
    DryRun    bool     `json:"dry_run"`
    // 新增
    Image     string   `json:"image"`      // 部署映像（$BUILD_IMAGE 自動替換）
    Workload  string   `json:"workload"`   // e.g. "deployment/saas-app"
    Container string   `json:"container"`  // container name（預設 = workload name）
}
```

### 3.5 $BUILD_IMAGE 變數替換

在 `executeStepWithRetry` 中，如果 step type 是 `deploy`，從同 Run 的 build-image step 取得 destination：

```go
// 在 executeStepWithRetry 中，提交 Job 前
if step.Type == "deploy" {
    buildImage := s.getBuildImageFromRun(ctx, run.ID)
    if buildImage != "" {
        sr.ConfigJSON = strings.ReplaceAll(sr.ConfigJSON, "$BUILD_IMAGE", buildImage)
    }
}
```

```go
func (s *PipelineScheduler) getBuildImageFromRun(ctx context.Context, runID uint) string {
    var buildSR models.StepRun
    if err := s.db.WithContext(ctx).
        Where("pipeline_run_id = ? AND step_type = 'build-image' AND status = 'success'", runID).
        First(&buildSR).Error; err != nil {
        return ""
    }
    var cfg BuildImageConfig
    if err := json.Unmarshal([]byte(buildSR.ConfigJSON), &cfg); err != nil {
        return ""
    }
    return cfg.Destination
}
```

### 3.6 Deploy Step 的 ServiceAccount

Deploy step 需要 `automountServiceAccountToken = true`，現有邏輯已處理：

```go
// pipeline_job_builder.go 已有
if b.isDeployStepType(input.StepRun.StepType) && cfg.ServiceAccount != "" {
    automount = true
}
```

但需要讓使用者不填 `service_account` 也能用預設的。改為：deploy 類型一律開啟 automount。

---

## 4. 前端改動

### 4.1 Pipeline 編輯器 — Deploy 表單

表單模式中新增 deploy step 的表單欄位：

```
構建步驟                        [表單模式] [JSON]
─────────────────────────────────────────
映像倉庫:     [local-harbor ▼]
映像名稱:     [saas/repo:dev    ]
Dockerfile:  [Dockerfile       ]
Build Context: [.              ]

─── 部署設定（可選）─────────────────────
☑ 啟用部署

Manifest 路徑:
  [k8s/deployment.yaml          ] [✕]
  [k8s/service.yaml             ] [✕]
  [+ 新增]

目標 Namespace: [dev              ]
Workload:       [deployment/saas-app]
Container:      [saas-app          ]
```

### 4.2 Run 列表 — 區分 CI 和 Deploy Run

新增 `trigger_type = 'deploy'` 的 i18n：
```
手動 → 手動觸發（CI）
部署 → 部署
```

### 4.3 Run 詳情 — 部署步驟的卡片

Deploy Run 的 DAG 卡片：
```
[build-image] → [scan-build] → [approve-deploy] → [deploy]
     ✅              ✅              ⏸ 等待核准        ⏳
```

---

## 5. 實作步驟

### Phase 1：後端 Deploy Step 增強（1 小時）
1. [ ] `DeployConfig` 新增 `image`、`workload`、`container` 欄位
2. [ ] `generateDeployCommand` 支援 `kubectl set image`
3. [ ] Deploy step 的 `automountServiceAccountToken` 預設開啟
4. [ ] `$BUILD_IMAGE` 變數替換邏輯

### Phase 2：前端 Deploy 表單（1 小時）
5. [ ] Pipeline 編輯器表單模式新增 deploy 區塊
6. [ ] Deploy 區塊的欄位：manifests、namespace、workload、container
7. [ ] 啟用部署開關（控制 deploy step 是否包含在 JSON 中）
8. [ ] 表單 ↔ JSON 雙向同步更新

### Phase 3：測試驗證（30 分鐘）
9. [ ] 建立 saas-dev Pipeline，包含 build + deploy
10. [ ] 觸發 Run，驗證 build → deploy 完整流程
11. [ ] 驗證 kubectl apply + set image 結果
12. [ ] 驗證 approval_enabled 時的審核流程

---

## 6. 風險評估

### 6.1 高風險

| 風險 | 影響 | 緩解措施 |
|------|------|---------|
| ServiceAccount 權限不足 | kubectl apply 403 Forbidden | 文件說明 RBAC 設定；錯誤時清楚顯示權限問題 |
| $BUILD_IMAGE 替換失敗 | 部署了舊映像或失敗 | 替換前驗證非空；替換後 log 完整 command |
| 同一個 Pipeline 部署到錯的 Namespace | 影響其他環境 | Namespace 寫在 deploy config 裡，不是使用者選的 |

### 6.2 中風險

| 風險 | 影響 | 緩解措施 |
|------|------|---------|
| Manifest 不存在 | kubectl apply 失敗 | 日誌中顯示 file not found |
| 映像 tag 衝突（dev/staging 用同一個 tag） | 部署了錯誤版本 | 建議使用者用 git SHA 或時間戳作為 tag |
| Build 成功但 Deploy 失敗 | 映像推送了但沒部署 | Run 狀態顯示 failed，使用者可重跑 |

### 6.3 低風險

| 風險 | 影響 | 緩解措施 |
|------|------|---------|
| 三個 Pipeline 設定不一致 | 環境差異 | 使用者自行管理；未來可用 Pipeline Template |
| Deploy step 被跳過（dependencies not met） | 沒有部署 | UI 顯示 skipped 狀態 |

---

## 7. 方案 A vs 方案 B 對比

| 維度 | 方案 A（內建 Deploy） | 方案 B（構建部署分離） |
|------|---------------------|---------------------|
| 操作步驟 | 一鍵觸發，全自動 | 兩步：先構建，再手動部署 |
| 環境安全 | 高（Pipeline 綁定環境） | 中（部署時使用者選環境，可能選錯） |
| 靈活性 | 低（一個 Pipeline 一個環境） | 高（一次構建，部署到任意環境） |
| Pipeline 數量 | 多（每環境一個） | 少（一個 Pipeline + 部署到多環境） |
| 適用團隊 | 小團隊、固定環境 | 大團隊、動態環境 |
| 實作複雜度 | 低 | 中 |
| 回滾 | 重新觸發舊版本 Run | 選擇歷史 Run 重新部署 |

### 建議

**兩個方案不衝突，可以並存**：
- 方案 A 是 Pipeline JSON 裡有 deploy step → 構建完自動部署
- 方案 B 是 Pipeline JSON 裡沒有 deploy step → 構建完手動點「部署」

使用者根據場景選擇：
- 固定環境、流程確定 → 用方案 A
- 靈活部署、多環境切換 → 用方案 B

**先實作方案 A**（改動小，現有機制可複用），方案 B 作為後續迭代。

---

## 8. 驗收標準

- [ ] Pipeline JSON 中可包含 deploy step（type=deploy）
- [ ] Deploy step 的 command 包含 kubectl apply + kubectl set image
- [ ] $BUILD_IMAGE 自動替換為同 Run 的 build-image 產出映像
- [ ] Deploy step 預設開啟 automountServiceAccountToken
- [ ] Pipeline 編輯器表單模式可開關「啟用部署」
- [ ] 表單模式可設定 manifests、namespace、workload、container
- [ ] approval_enabled 開啟時，deploy 前自動插入審核
- [ ] Run 詳情頁顯示 deploy step 的 kubectl 日誌
- [ ] Build 成功 + Deploy 失敗時，Run 狀態為 failed
