# CD 部署設計方案 — 構建與部署分離

> 版本：v1.0 | 日期：2026-04-18 | 狀態：待 Review

---

## 1. 設計目標

一次構建，多環境部署。使用者在 CI 構建成功後，手動選擇部署到哪個叢集/Namespace。

### 核心原則

| 原則 | 說明 |
|------|------|
| 構建一次 | 同一個映像部署到 dev/staging/prod，不重複構建 |
| 部署可控 | 使用者明確決定何時、部署到哪裡 |
| 可審核 | 部署前可插入審核閘門（approval_enabled） |
| 可追溯 | 每次部署記錄：誰、什麼時候、部署了什麼映像、到哪個叢集 |
| 漸進式 | 不破壞現有 CI 流程，新增部署功能 |

---

## 2. 使用者流程

```
Step 1: 使用者觸發 CI 構建
  Pipeline Run #8: build → scan → ✅ 成功
  映像: 192.168.0.137:5100/saas/repo:latest
  掃描: 2C 16H 17M 12L

Step 2: 使用者在 Run 列表看到構建成功，點擊「部署」
  ┌─────────────────────────────────┐
  │  部署 — Run #8                   │
  │                                  │
  │  映像: saas/repo:latest          │
  │  目標叢集: [docker-desktop ▼]    │
  │  目標 Namespace: [production  ]  │
  │  Manifest 來源:                  │
  │    ○ 專案內 (k8s/*.yaml)         │
  │    ○ 手動輸入 YAML               │
  │                                  │
  │  Manifest 路徑:                  │
  │  [k8s/deployment.yaml       ]    │
  │  [k8s/service.yaml          ]    │
  │                                  │
  │        [取消]  [部署]            │
  └─────────────────────────────────┘

Step 3: 系統建立 Deploy Run
  Pipeline Run #9 (type=deploy):
    - git clone（取得 manifest 檔案）
    - kubectl apply -f k8s/deployment.yaml -f k8s/service.yaml -n production
    - 更新 deployment 的 image 為構建產出的映像

Step 4: 部署結果
  Run 列表:
  #9 | 成功 | 部署 | production | saas/repo:latest
  #8 | 成功 | 手動 | 2C 16H 17M | —
```

---

## 3. 資料模型

### 3.1 PipelineRun 擴充

現有 `pipeline_runs` 表已有 `trigger_type` 欄位，新增 `deploy` 類型：

```sql
-- trigger_type 新增值：'deploy'
-- 新增欄位：
ALTER TABLE pipeline_runs ADD COLUMN deploy_image VARCHAR(512);        -- 部署的映像（從 CI run 的 build-image 產出取得）
ALTER TABLE pipeline_runs ADD COLUMN deploy_from_run_id BIGINT;        -- 來源 CI Run ID
ALTER TABLE pipeline_runs ADD COLUMN deploy_manifests TEXT;             -- manifest 路徑列表（JSON array）
```

### 3.2 不新增表

部署是一種特殊的 Pipeline Run（`trigger_type = 'deploy'`），複用現有的：
- `pipeline_runs` — 記錄部署任務
- `step_runs` — 記錄 deploy step 執行狀態
- `pipeline_logs` — 記錄 kubectl apply 日誌

---

## 4. API 設計

### 4.1 觸發部署

```
POST /api/v1/pipelines/:pipelineID/deploy
```

Request:
```json
{
  "from_run_id": 8,                              // 來源 CI Run（必填）
  "cluster_id": 1,                                // 目標叢集（必填）
  "namespace": "production",                      // 目標 Namespace（必填）
  "manifests": ["k8s/deployment.yaml", "k8s/service.yaml"],  // manifest 路徑（可選，預設用 Pipeline 設定）
  "image_override": ""                            // 映像覆蓋（可選，預設用 CI run 的產出映像）
}
```

Response:
```json
{
  "run_id": 9,
  "status": "queued",
  "message": "deploy triggered"
}
```

### 4.2 取得可部署的 Run 列表

```
GET /api/v1/pipelines/:pipelineID/deployable-runs
```

回傳所有 CI 構建成功的 Run（有 build-image artifact），供部署 Modal 選擇。

Response:
```json
{
  "items": [
    {
      "run_id": 8,
      "image": "192.168.0.137:5100/saas/repo:latest",
      "built_at": "2026-04-18T20:30:00Z",
      "scan_result": { "critical": 2, "high": 16 }
    }
  ]
}
```

---

## 5. 後端實作計劃

### 5.1 Model 變更

```go
// pipeline_runs 新增欄位
type PipelineRun struct {
    // ... 現有欄位 ...
    DeployImage     string `json:"deploy_image" gorm:"size:512"`
    DeployFromRunID *uint  `json:"deploy_from_run_id" gorm:"index"`
    DeployManifests string `json:"deploy_manifests" gorm:"type:text"` // JSON array
}
```

### 5.2 Deploy Handler

新增 `DeployPipeline` handler：

```go
func (h *PipelineRunHandler) DeployPipeline(c *gin.Context) {
    // 1. 驗證 from_run_id 存在且狀態是 success
    // 2. 從 CI run 的 build-image step 取得產出映像
    // 3. 建立 deploy PipelineRun（trigger_type = 'deploy'）
    // 4. 建構 deploy step JSON：
    //    - git clone（取得 manifest 檔案）
    //    - sed 替換 manifest 中的 image 為實際映像
    //    - kubectl apply
    // 5. EnqueueRun
}
```

### 5.3 Deploy Step 執行邏輯

Deploy step 需要：
1. **Git clone** — init container 取得專案的 manifest 檔案
2. **替換映像** — 把 manifest 中的 image placeholder 替換為實際構建產出的映像
3. **kubectl apply** — 部署到目標叢集的目標 Namespace

替換映像的方式：
```bash
# 在 deploy step 的 command 中
sed -i "s|IMAGE_PLACEHOLDER|192.168.0.137:5100/saas/repo:latest|g" k8s/deployment.yaml
kubectl apply -f k8s/deployment.yaml -f k8s/service.yaml -n production
```

或使用 `kustomize`：
```bash
kubectl apply -k k8s/ -n production
```

### 5.4 Deploy Step 與 Build 環境的差異

| | Build Run | Deploy Run |
|---|---|---|
| cluster_id | Pipeline.build_cluster_id | 使用者選擇的目標叢集 |
| namespace | Pipeline.build_namespace | 使用者選擇的目標 Namespace |
| Job 執行位置 | build 叢集 | **build 叢集**（Job 本身跑在 build 叢集，但 kubectl apply 指向目標叢集） |
| ServiceAccount | 不需要 | 需要有目標叢集的 deploy 權限 |

**重要決策**：Deploy Job 跑在哪裡？

- **選項 A**：Deploy Job 跑在目標叢集 → 簡單，kubectl 直接用 in-cluster config
- **選項 B**：Deploy Job 跑在 build 叢集 → 需要注入目標叢集的 kubeconfig

**推薦選項 A**：Deploy Job 跑在目標叢集，使用 in-cluster ServiceAccount。
- 原因：不需要跨叢集傳遞 kubeconfig，安全性更好
- 前提：目標叢集需要有能執行 Job 的 Namespace + 有 deploy 權限的 ServiceAccount

---

## 6. 前端實作計劃

### 6.1 Run 列表頁新增「部署」按鈕

在每個**構建成功**的 Run 行，增加「部署」操作按鈕：

```
Run ID | 狀態 | 觸發方式 | 安全掃描    | 操作
#8     | 成功 | 手動     | 2C 16H 17M | [查看] [部署]
#9     | 成功 | 部署     | —          | [查看]
```

### 6.2 部署 Modal

```tsx
<DeployModal
  open={deployOpen}
  onClose={() => setDeployOpen(false)}
  pipeline={pipeline}
  sourceRun={selectedRun}      // CI Run
  onSuccess={() => refetch()}
/>
```

Modal 內容：
- 映像名稱（唯讀，從 CI run 取得）
- 目標叢集（下拉選擇）
- 目標 Namespace（輸入）
- Manifest 路徑（多選 tags input，預設從 Pipeline 設定讀取）

### 6.3 Run 列表區分 CI 和 Deploy

`trigger_type` 欄位增加 `deploy` 類型的顯示：
- `手動` → CI 構建
- `部署` → 部署操作（顯示部署目標 Namespace + 映像）

---

## 7. 實作步驟（按順序）

### Phase 1：後端基礎（1-2 小時）
1. [ ] Model：`PipelineRun` 新增 `deploy_image`、`deploy_from_run_id`、`deploy_manifests`
2. [ ] DB Migration：新增欄位
3. [ ] Handler：`DeployPipeline` — 觸發部署 Run
4. [ ] Handler：`ListDeployableRuns` — 列出可部署的 CI Run
5. [ ] Route：註冊新路由

### Phase 2：部署執行邏輯（1-2 小時）
6. [ ] Scheduler：處理 `trigger_type=deploy` 的 Run
7. [ ] Deploy step command：git clone → sed 替換 image → kubectl apply
8. [ ] Deploy Job 的 ServiceAccount 設定（automountServiceAccountToken=true）
9. [ ] Deploy step 的 security context（需要 kubectl 權限）

### Phase 3：前端（1-2 小時）
10. [ ] `DeployModal` 元件
11. [ ] Run 列表加「部署」按鈕
12. [ ] Run 列表區分 CI/Deploy 顯示
13. [ ] 部署詳情頁（點擊 deploy run 查看 kubectl apply 日誌）

### Phase 4：審核整合（30 分鐘）
14. [ ] 部署前插入 approval gate（如果 approval_enabled 開啟）
15. [ ] 前端 Run 詳情頁顯示審核狀態

---

## 8. 風險評估

### 8.1 高風險

| 風險 | 影響 | 緩解措施 |
|------|------|---------|
| Deploy Job 權限不足 | kubectl apply 失敗 | 文件說明需要建立 ServiceAccount + ClusterRoleBinding |
| Manifest 中映像替換錯誤 | 部署了錯誤的映像 | 用精確的 sed pattern + 驗證替換結果 |
| 目標叢集不可達 | 部署失敗 | 部署前先驗證叢集連線 |

### 8.2 中風險

| 風險 | 影響 | 緩解措施 |
|------|------|---------|
| 同一映像重複部署 | 資源浪費但無害 | 前端顯示已部署狀態 |
| Manifest 格式錯誤 | kubectl apply 失敗 | 日誌中顯示 kubectl 錯誤訊息 |
| 掃描有 Critical 漏洞但仍部署 | 安全風險 | 部署 Modal 顯示掃描結果警告 |

### 8.3 低風險

| 風險 | 影響 | 緩解措施 |
|------|------|---------|
| Deploy Run 被誤取消 | 部分資源已部署 | kubectl apply 是冪等的，重新部署即可 |
| 多人同時部署同一映像 | 資源衝突 | Pipeline 的 concurrency_policy 已有控制 |

---

## 9. 不在此階段實作的功能

| 功能 | 原因 | 未來里程碑 |
|------|------|-----------|
| 自動部署（CI 成功後自動 deploy） | 需要環境管理（M17） | M17 |
| 多環境晉升（dev → staging → prod） | 需要 Environment model | M17 |
| Rollback（回滾到上一個版本） | 已有基礎，但需要 deploy 歷史 | M17 |
| Helm deploy | 需要 Helm chart 管理 | 按需 |
| ArgoCD sync | 已有 adapter，但需要 GitOps 整合 | M16 |

---

## 10. 驗收標準

- [ ] 使用者可在構建成功的 Run 上點擊「部署」
- [ ] 部署 Modal 顯示映像名稱、掃描結果摘要、叢集/Namespace 選擇
- [ ] 部署建立新的 Run（trigger_type=deploy）
- [ ] 部署 Run 執行 kubectl apply 並顯示日誌
- [ ] 部署成功後 Run 列表顯示部署狀態
- [ ] approval_enabled 開啟時，部署前需人工審核
- [ ] 部署失敗時顯示 kubectl 錯誤訊息
