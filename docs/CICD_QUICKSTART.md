# CICD 快速入門指南

> 適用版本：Synapse CICD M13a–M17（Pipeline 獨立實體架構）

---

## 前置條件

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
