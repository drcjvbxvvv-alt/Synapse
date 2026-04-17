# CICD 快速入門指南

> 適用版本：Synapse CICD M13a–M17（Pipeline 獨立實體架構）

---

## 本地開發環境快速啟動

使用 `deploy/docker-compose-cicd.yaml` 一鍵啟動本地 CI/CD 基礎設施：

```bash
docker compose -f deploy/docker-compose-cicd.yaml up -d
```

> GitLab 初始化需要約 2–3 分鐘，請耐心等待。

### 服務清單

| 服務 | URL | 帳號 | 密碼 |
|------|-----|------|------|
| GitLab | http://localhost:8929 | root | Gitlab@2026 |
| Container Registry | http://localhost:5001 | — | — |
| ArgoCD | http://localhost:8081 | admin | 見下方 |
| GitLab SSH | `ssh://localhost:2222` | — | — |

> GitLab 首次啟動約需 3–5 分鐘完成初始化，看到頁面前請耐心等候。

### ArgoCD 初始密碼

```bash
docker exec argocd argocd admin initial-password 2>/dev/null | head -1
```

### Push Image 到本地 Registry

```bash
docker tag myapp:latest localhost:5001/myapp:latest
docker push localhost:5001/myapp:latest
```

> **注意**：若 port 已被佔用，修改 `deploy/docker-compose-cicd.yaml` 中對應的 host port。

### 停止環境

```bash
docker compose -f deploy/docker-compose-cicd.yaml down
```

---

## Synapse 原生 CI 設定流程（本地 GitLab）

> 以下以本地 docker-compose 環境為例，依序完成 5 個步驟。

### Step 1：匯入 Cluster

前往 **叢集管理 → 匯入叢集**，貼上目標 K8s 叢集的 kubeconfig。

- Pipeline Run 的 Steps 以 K8s Job 形式跑在此 cluster 上
- 沒有 Cluster 無法執行任何 Pipeline Run

### Step 2：新增 Git Provider

前往 **設定 → Git Providers → 新增**

| 欄位 | 值 |
|------|-----|
| 類型 | GitLab |
| Base URL | `http://localhost:8929` |
| Access Token | GitLab root → User Settings → Access Tokens → 建立（勾選 `api` scope） |

### Step 3：新增 Registry

前往 **設定 → Registries → 新增**

| 欄位 | 值 |
|------|-----|
| URL | `localhost:5001` |
| Insecure TLS | ✓（本地 registry 無 TLS） |

### Step 4：建立 Project

前往 **設定 → Projects → 新增**

| 欄位 | 值 |
|------|-----|
| Git Provider | 剛才建立的 GitLab |
| Repo URL | `http://localhost:8929/root/<repo-name>.git` |

Project 是 Webhook 路由的關鍵：git push 進來時，系統依 `repo_url` 找到對應 Project，再觸發訂閱它的 Pipeline。

### Step 5：建立 Pipeline

前往 **Pipelines → 建立**，綁定上面的 Project，以 `examples/java-demo` 為例：

```json
{
  "steps": [
    {
      "name": "build",
      "type": "build-image",
      "config": {
        "context": ".",
        "dockerfile": "Dockerfile",
        "destination": "localhost:5001/java-demo:${GIT_SHA}"
      }
    },
    {
      "name": "deploy",
      "type": "deploy",
      "depends_on": ["build"],
      "config": {
        "manifests": ["k8s/deployment.yaml", "k8s/service.yaml"],
        "namespace": "default"
      }
    }
  ]
}
```

### Step 6：設定 Webhook（自動觸發）

GitLab repo → Settings → Webhooks → 新增：

```
URL:    http://<synapse-host>/api/webhook/git
Events: Push events
```

> 若只用手動觸發測試，可跳過此步驟，直接在 Pipeline 頁面點擊 **手動執行**。

---

## 準備 Git Repo 內容（前置條件）

在建立 Pipeline 之前，需要依序完成以下設定。

Synapse CICD **不會自動生成 K8s 資源**，以下檔案需自行準備並放入 git repo：

| 用途 | 需要的檔案 |
|------|-----------|
| 建立 image | `Dockerfile` |
| `deploy` Step | `k8s/deployment.yaml`、`k8s/service.yaml`（含 namespace） |
| `deploy-helm` Step | Helm Chart 目錄或 chart repo |
| `gitops-sync` Step | GitOps manifest repo（獨立 repo 存放 yaml） |
| `deploy-argocd-sync` | ArgoCD Application 需已存在於叢集 |

> **Namespace 也需要預先建立**，或在 yaml 中包含 `kind: Namespace`。
>
> 可直接使用 `examples/java-demo/` 作為測試起點。

### 推送範例應用

```bash
cp -r examples/java-demo /tmp/java-demo
cd /tmp/java-demo
git init
git remote add origin http://localhost:8929/root/java-demo.git
git add .
git commit -m "init"
git push -u origin main
```

> 先在 GitLab 建立空白 repo：http://localhost:8929/projects/new

---

## Monorepo 微服務架構（多 Pipeline + Path Filter）

> 適用場景：一個 Git repo 包含多個微服務，推送時只觸發變動服務的 Pipeline。

### Repo 目錄結構

```
saas-uat-repo/
├── services/
│   ├── user-service/
│   │   ├── Dockerfile
│   │   ├── k8s/
│   │   └── src/
│   ├── order-service/
│   │   ├── Dockerfile
│   │   ├── k8s/
│   │   └── src/
│   └── gateway/
│       ├── Dockerfile
│       ├── k8s/
│       └── src/
└── shared/              ← 公共程式庫，變動時觸發所有 Pipeline
```

### 設定方式

1. 建立 **一個 Project**，綁定整個 monorepo URL
2. 為每個微服務建立 **獨立 Pipeline**
3. 在每個 Pipeline 的 Trigger 中設定 `path_filter` 精確限定觸發範圍

### Pipeline Steps 範例（user-service）

```json
[
  {
    "name": "build",
    "type": "build-image",
    "config": {
      "context": "services/user-service",
      "dockerfile": "services/user-service/Dockerfile",
      "destination": "localhost:5001/user-service:latest"
    }
  },
  {
    "name": "deploy",
    "type": "deploy",
    "depends_on": ["build"],
    "config": {
      "manifests": ["services/user-service/k8s/deployment.yaml"],
      "namespace": "default"
    }
  }
]
```

### Trigger 設定（triggers_json）

```json
[
  {
    "type": "webhook",
    "repo": "root/saas-uat-repo",
    "branch": "main",
    "events": ["push"],
    "path_filter": ["services/user-service/**", "shared/**"],
    "cluster_id": 1,
    "namespace": "default"
  }
]
```

### Path Filter 匹配規則

| Pattern | 匹配範例 | 說明 |
|---------|---------|------|
| `services/user-service/**` | `services/user-service/src/main.go` | 遞迴匹配子目錄 |
| `shared/**` | `shared/lib/auth.go` | 公共程式庫變動 |
| `**/*.go` | `pkg/util/helper.go` | 任意層級的 .go 檔案 |
| `Dockerfile` | `Dockerfile` | 完全匹配 |

### 觸發隔離效果

| 推送的變更 | 觸發的 Pipeline |
|-----------|----------------|
| `services/user-service/src/main.go` | 只觸發 user-service |
| `services/order-service/pom.xml` | 只觸發 order-service |
| `shared/lib/auth.go` | 觸發 **所有** Pipeline |
| `README.md` | 不觸發任何 Pipeline |

### Git Repo 連線驗證

建立 Project 時，Synapse 會自動透過 Git Provider 的 API 驗證 repo URL 是否可連線。
若 token 無權限或 repo 不存在，會回傳錯誤並阻止建立。

---

## 前置條件（通用）

在建立第一條 Pipeline 之前，需要依序完成以下設定。

### 1. 匯入 Cluster

前往 **叢集管理 → 匯入叢集**，貼上目標 K8s 叢集的 kubeconfig。

- Pipeline Run 執行時，Steps 以 K8s Job 形式跑在指定 Cluster 上
- 沒有 Cluster 無法執行任何 Pipeline Run

### 2. 設定 Git Provider

前往 **設定 → Git Providers**，新增以下任一：

- GitHub（Personal Access Token 或 GitHub App）
- GitLab（Access Token）

Git Provider 用於：
- Webhook 接收 push / tag / PR 事件
- `build-image` Step 拉取原始碼

### 3. 設定 Container Registry

前往 **設定 → Registries**，新增 Docker Hub、Harbor、ECR 或其他 OCI Registry。

`build-image` Step 需要知道 image 要 push 到哪裡。

### 4. 建立 Project

前往 **設定 → Projects**，填入 Git repo URL（如 `https://github.com/org/repo`）並綁定 Git Provider。

Project 是 Webhook 路由的關鍵：git push 進來時，系統依 `repo_url` 找到對應 Project，再觸發訂閱它的 Pipeline。

### 5. 設定 Webhook（若需要自動觸發）

在 GitHub / GitLab 的 repo 設定頁面新增 Webhook：

```
URL:          https://<synapse-host>/api/webhook/git
Content-Type: application/json
Events:       Push、Tag、Pull Request（視需求）
```

> 若只用 Cron Trigger 或手動觸發，可跳過此步驟。

---

## 準備 Git Repo 內容

Synapse CICD **不會自動生成 K8s 資源**，以下檔案需自行準備並放入 git repo：

| 用途 | 需要的檔案 |
|------|-----------|
| 建立 image | `Dockerfile` |
| `deploy` Step | `k8s/deployment.yaml`、`k8s/service.yaml`（含 namespace） |
| `deploy-helm` Step | Helm Chart 目錄或 chart repo |
| `gitops-sync` Step | GitOps manifest repo（獨立 repo 存放 yaml） |
| `deploy-argocd-sync` | ArgoCD Application 需已存在於叢集 |

> **Namespace 也需要預先建立**，或在 yaml 中包含 `kind: Namespace`。

---

## 建立 Pipeline

### 最小 Pipeline 範例（build → scan → deploy）

```json
{
  "steps": [
    {
      "name": "build",
      "type": "build-image",
      "config": {
        "context": ".",
        "dockerfile": "Dockerfile",
        "destination": "registry.example.com/myapp:${GIT_SHA}"
      }
    },
    {
      "name": "scan",
      "type": "trivy-scan",
      "depends_on": ["build"],
      "config": {
        "image": "registry.example.com/myapp:${GIT_SHA}",
        "severity": "CRITICAL,HIGH",
        "exit_on_fail": true
      }
    },
    {
      "name": "deploy",
      "type": "deploy",
      "depends_on": ["scan"],
      "config": {
        "manifests": ["k8s/deployment.yaml", "k8s/service.yaml"],
        "namespace": "production"
      }
    }
  ]
}
```

### 可用 Step 類型

| Step Type | 用途 |
|-----------|------|
| `build-image` | 用 Kaniko 建立 container image |
| `push-image` | 用 crane 重新 tag / push image |
| `build-jar` | 用 Maven/Gradle 建立 Java artifact |
| `trivy-scan` | Trivy 漏洞掃描 |
| `deploy` | kubectl apply（直接部署 yaml） |
| `deploy-helm` | Helm upgrade --install |
| `deploy-argocd-sync` | 觸發 ArgoCD Application sync |
| `deploy-rollout` | 觸發 Argo Rollouts canary/blue-green |
| `rollout-promote` | Argo Rollouts promote |
| `rollout-abort` | Argo Rollouts abort |
| `rollout-status` | 等待 Argo Rollouts 達到指定狀態 |
| `gitops-sync` | git commit + push manifest 到 GitOps repo |
| `smoke-test` | HTTP smoke test 驗證 endpoint |
| `approval` | 手動審批閘（暫停 Pipeline 等待人工確認） |
| `notify` | 發送 Webhook 通知（Slack、Teams、generic） |
| `run-script` / `shell` | 執行自訂 shell script |
| `custom` | 自訂 image + command |

---

## 典型起步流程（最小路徑）

```
步驟 1：匯入 Cluster
步驟 2：設定 Git Provider + Registry
步驟 3：建立 Project（綁定 repo URL）
步驟 4：在 git repo 準備 Dockerfile + k8s yaml
步驟 5：建立 Pipeline（定義 Steps + Trigger）
步驟 6：設定 Webhook（或用 Cron / 手動觸發）
步驟 7：Push → 自動觸發 Pipeline Run → 觀察 Step 執行狀態
```

---

## 觸發方式

| 觸發類型 | 說明 |
|---------|------|
| **Git Push** | 設定 Webhook，指定 branch pattern（如 `main`、`release/*`） |
| **Git Tag** | 設定 Webhook，指定 tag pattern（如 `v*`） |
| **Cron** | 定時觸發，填入 cron expression（如 `0 2 * * *`） |
| **手動** | 在 Pipeline 頁面點擊 Run，可覆寫參數 |

---

## Synapse CICD 的邊界

| Synapse 負責 | 需要自行準備 |
|-------------|-------------|
| 接收 Webhook、路由到 Pipeline | Dockerfile |
| 建立並監控 K8s Job | K8s manifest yaml / Helm Chart |
| Build image、scan、deploy | Namespace 預先建立 |
| 跨環境 Image Promotion | ArgoCD Application（若用 ArgoCD） |
| 審批閘、通知 | GitOps manifest repo（若用 gitops-sync） |

Synapse CICD 是 **Pipeline 執行引擎**，不是 IaC 工具或 K8s 資源管理器。
