# Synapse CICD 完整架構指南

> 版本：v1.0 | 更新日期：2026-04-16
> 適用對象：DevOps 工程師、平台管理員

---

## 目錄

1. [架構總覽](#1-架構總覽)
2. [第三方元件依賴](#2-第三方元件依賴)
3. [環境安裝](#3-環境安裝)
4. [元件設定](#4-元件設定)
5. [完整 CICD 流程](#5-完整-cicd-流程)
6. [GitLab CI 設定範例](#6-gitlab-ci-設定範例)
7. [Synapse CI 引擎設定](#7-synapse-ci-引擎設定)
8. [GitOps 部署（ArgoCD）](#8-gitops-部署argocd)
9. [Kubernetes 部署規格](#9-kubernetes-部署規格)
10. [監控與告警](#10-監控與告警)
11. [故障排查](#11-故障排查)
12. [目錄結構說明](#12-目錄結構說明)

---

## 1. 架構總覽

```
┌─────────────────────────────────────────────────────────────────┐
│                         Developer                               │
│                      git push / MR                              │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                       GitLab CE                                 │
│              原始碼管理 + CI/CD Webhook 觸發                     │
└───┬───────────────────────────────────────────────┬─────────────┘
    │ Webhook                                       │ API
    ▼                                               ▼
┌────────────────────┐                  ┌───────────────────────┐
│  GitLab Runner /   │                  │   Synapse             │
│  Jenkins Agent     │                  │   CI 引擎管理平台      │
│                    │                  │   (M18a–M19c)         │
│  ① Lint            │                  │                       │
│  ② Unit Test       │                  │  · 設定 GitLab/Jenkins│
│  ③ Build Binary    │                  │    /Tekton/Argo 引擎  │
│  ④ Docker Build    │                  │  · 觸發 Run / 查看    │
│  ⑤ Trivy Scan      │                  │    Log / Artifact     │
│  ⑥ Harbor Push     │                  └───────────────────────┘
└────────┬───────────┘
         │ image tag
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Harbor                                    │
│              私有映像倉庫 + 映像掃描 + 安全策略                   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ Git commit (更新 YAML image tag)
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                       ArgoCD                                    │
│              GitOps 自動同步 K8s 叢集狀態                        │
└──────────────────────────┬──────────────────────────────────────┘
                           │ kubectl apply
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes 叢集                               │
│   synapse-backend  synapse-frontend  saas-java-a  saas-java-b   │
└─────────────────────────────────────────────────────────────────┘
                           │ metrics
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│               Prometheus + Grafana + AlertManager               │
└─────────────────────────────────────────────────────────────────┘
```

### 資料流說明

| 步驟 | 動作 | 負責元件 |
|------|------|----------|
| 1 | 開發者推送程式碼或建立 MR | GitLab |
| 2 | 觸發 Pipeline（Webhook → Runner） | GitLab CI |
| 3 | 靜態分析、單元測試 | Runner + golangci-lint / ESLint / Maven |
| 4 | 編譯二進位 / 打包 JAR | Runner |
| 5 | 多階段 Docker Build | Runner + Docker-in-Docker |
| 6 | Trivy 漏洞掃描映像 | Trivy |
| 7 | 推送映像到 Harbor | Runner → Harbor |
| 8 | 更新 K8s YAML 中的 image tag 並 git push | Runner |
| 9 | ArgoCD 偵測 Git 變更，自動同步 | ArgoCD |
| 10 | Kubernetes 滾動更新 | K8s API Server |

---

## 2. 第三方元件依賴

### 2.1 必要元件

| 元件 | 版本建議 | 用途 | 授權 |
|------|----------|------|------|
| **GitLab CE** | 16.x+ | 原始碼倉庫 + CI/CD 觸發 | MIT |
| **GitLab Runner** | 16.x+ | 執行 CI Pipeline Job | MIT |
| **Harbor** | 2.10+ | 私有 Docker 映像倉庫 | Apache 2.0 |
| **ArgoCD** | 2.9+ | GitOps 持續部署 | Apache 2.0 |
| **Kubernetes** | 1.26+ | 容器編排平台 | Apache 2.0 |
| **PostgreSQL** | 14+ | Synapse 主資料庫 | PostgreSQL |
| **Docker** | 24+ | 容器建置引擎 | Apache 2.0 |

### 2.2 選用元件

| 元件 | 版本建議 | 用途 |
|------|----------|------|
| **Trivy** | 0.50+ | 映像漏洞掃描 |
| **Jenkins** | 2.440+ | 替代 GitLab CI 作為 CI 引擎 |
| **Tekton Pipelines** | 0.57+ | K8s 原生 CI 引擎 |
| **Argo Workflows** | 3.5+ | K8s 原生工作流 CI 引擎 |
| **Prometheus** | 2.50+ | 指標收集 |
| **Grafana** | 10.x+ | 指標視覺化 |
| **AlertManager** | 0.27+ | 告警通知 |

### 2.3 建置工具依賴

| 工具 | 用途 |
|------|------|
| Go 1.22+ | Synapse 後端編譯 |
| Node.js 20+ / npm 10+ | Synapse 前端建置 |
| Maven 3.8+ / JDK 11+ | Java SaaS 應用建置 |
| golangci-lint 1.57+ | Go 靜態分析 |
| ESLint 8.x+ | TypeScript 靜態分析 |

---

## 3. 環境安裝

### 3.1 使用 Docker Compose 快速啟動（開發 / 測試）

```bash
# 複製專案
git clone https://github.com/your-org/Synapse.git
cd Synapse

# 啟動完整 CICD 基礎設施
docker compose -f deploy/docker-compose-cicd.yaml up -d

# 等待服務就緒（約 2–3 分鐘）
docker compose -f deploy/docker-compose-cicd.yaml ps
```

服務啟動後的存取地址：

| 服務 | URL | 預設帳號 | 預設密碼 |
|------|-----|----------|----------|
| GitLab | http://localhost | root | Gitlab@2026 |
| Harbor | http://localhost:8080 | admin | Harbor@2026 |
| ArgoCD | http://localhost:8081 | admin | 見下方初始化步驟 |

**初始化 ArgoCD 管理員密碼：**

```bash
# 取得初始密碼
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d

# 或透過 Docker 執行（若 ArgoCD 在 docker-compose）
docker exec argocd argocd admin initial-password
```

### 3.2 安裝 GitLab Runner

```bash
# macOS
brew install gitlab-runner

# Linux（Debian/Ubuntu）
curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash
sudo apt-get install gitlab-runner

# 註冊 Runner
gitlab-runner register \
  --url http://localhost \
  --token <YOUR_RUNNER_TOKEN> \
  --executor docker \
  --docker-image alpine:latest \
  --description "Synapse CI Runner"
```

### 3.3 安裝 Trivy（映像掃描）

```bash
# macOS
brew install aquasecurity/trivy/trivy

# Linux
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin

# 驗證
trivy --version
```

### 3.4 Kubernetes 叢集前置條件

```bash
# 確認叢集可用
kubectl cluster-info
kubectl get nodes

# 安裝 ArgoCD
kubectl create namespace argocd
kubectl apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# 等待 ArgoCD 就緒
kubectl wait --for=condition=available --timeout=300s \
  deployment/argocd-server -n argocd
```

---

## 4. 元件設定

### 4.1 Harbor 設定

#### 建立 Project

```bash
# 使用 Harbor CLI 或 Web UI
# Web UI: http://localhost:8080 → New Project

# 建立 synapse project（用於存放 Synapse 自身映像）
PROJECT_NAME="synapse"
curl -X POST http://localhost:8080/api/v2.0/projects \
  -u admin:Harbor@2026 \
  -H "Content-Type: application/json" \
  -d '{"project_name":"'"$PROJECT_NAME"'","public":false}'

# 建立 saas project（用於存放業務應用映像）
curl -X POST http://localhost:8080/api/v2.0/projects \
  -u admin:Harbor@2026 \
  -H "Content-Type: application/json" \
  -d '{"project_name":"saas","public":false}'
```

#### 建立 Robot Account（CI 用）

```
Harbor Web UI → 管理員 → Robot Accounts → 新增
  名稱: ci-robot
  描述: GitLab CI 推送帳號
  權限: Push / Pull / 刪除 Tag
  到期: 永不到期（或設定合理期限）
```

儲存產生的 Token，後續設定為 GitLab CI 的 `HARBOR_ROBOT_TOKEN` 變數。

#### 啟用 Trivy 掃描（Harbor 內建）

```
Harbor Web UI → 系統管理 → 安全 → 漏洞掃描
  → 啟用自動掃描（推送後掃描）
  → 設定掃描週期：每日
```

### 4.2 GitLab CI 變數設定

在 GitLab 專案 → Settings → CI/CD → Variables 新增：

| 變數名稱 | 說明 | Masked | Protected |
|----------|------|--------|-----------|
| `HARBOR_REGISTRY` | Harbor 地址（如 `harbor.local`） | 否 | 否 |
| `HARBOR_USERNAME` | Robot Account 名稱 | 否 | 是 |
| `HARBOR_PASSWORD` | Robot Account Token | **是** | 是 |
| `KUBE_CONFIG` | base64 後的 kubeconfig | **是** | 是 |
| `ARGOCD_SERVER` | ArgoCD Server 地址 | 否 | 否 |
| `ARGOCD_AUTH_TOKEN` | ArgoCD API Token | **是** | 是 |

```bash
# 產生 KUBE_CONFIG 變數值
cat ~/.kube/config | base64 -w 0
```

### 4.3 ArgoCD 設定

#### 新增 Git 倉庫

```bash
argocd repo add https://gitlab.local/your-org/Synapse.git \
  --username gitlab-ci \
  --password <YOUR_TOKEN> \
  --insecure-skip-server-verification
```

#### 建立 Application（見 `examples/argocd-synapse-application.yaml`）

```bash
kubectl apply -f deploy/examples/argocd-synapse-application.yaml
kubectl apply -f deploy/examples/argocd-application-example.yaml
```

---

## 5. 完整 CICD 流程

### 5.1 流程圖

```
開發者 git push
    │
    ▼
┌─────────────────────────────────────────────────────┐
│  Stage 1: lint                                      │
│  ┌──────────────────┐  ┌───────────────────────┐   │
│  │ golangci-lint    │  │ ESLint (TypeScript)   │   │
│  │ (Synapse 後端)   │  │ (Synapse 前端)        │   │
│  └──────────────────┘  └───────────────────────┘   │
│  ┌──────────────────┐                               │
│  │ Maven validate   │                               │
│  │ (Java 應用)      │                               │
│  └──────────────────┘                               │
└─────────────────────────────────────────────────────┘
    │ 全部通過
    ▼
┌─────────────────────────────────────────────────────┐
│  Stage 2: test                                      │
│  ┌──────────────────┐  ┌───────────────────────┐   │
│  │ go test -race    │  │ vitest / jest         │   │
│  │ (後端單元測試)   │  │ (前端單元測試)        │   │
│  └──────────────────┘  └───────────────────────┘   │
│  ┌──────────────────┐  ┌───────────────────────┐   │
│  │ mvn test (java-a)│  │ mvn test (java-b)     │   │
│  └──────────────────┘  └───────────────────────┘   │
└─────────────────────────────────────────────────────┘
    │ 全部通過
    ▼
┌─────────────────────────────────────────────────────┐
│  Stage 3: build                                     │
│  ┌──────────────────┐  ┌───────────────────────┐   │
│  │ go build         │  │ npm run build         │   │
│  │ → synapse-server │  │ → dist/               │   │
│  └──────────────────┘  └───────────────────────┘   │
│  ┌──────────────────┐  ┌───────────────────────┐   │
│  │ mvn package      │  │ mvn package           │   │
│  │ → java-a.jar     │  │ → java-b.jar          │   │
│  └──────────────────┘  └───────────────────────┘   │
└─────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────┐
│  Stage 4: docker-build                              │
│                                                     │
│  多階段 Dockerfile Build：                           │
│                                                     │
│  synapse/backend:${CI_COMMIT_SHORT_SHA}             │
│    FROM golang:1.22 AS builder                      │
│    COPY . .                                         │
│    RUN go build -o synapse-server ./cmd/server      │
│    FROM alpine:3.19                                 │
│    COPY --from=builder /app/synapse-server /app/    │
│                                                     │
│  synapse/frontend:${CI_COMMIT_SHORT_SHA}            │
│    FROM node:20 AS builder → npm run build          │
│    FROM nginx:alpine → COPY dist/ /usr/share/nginx/ │
│                                                     │
│  saas/java-a:${CI_COMMIT_SHORT_SHA}                 │
│  saas/java-b:${CI_COMMIT_SHORT_SHA}                 │
│    FROM openjdk:11-jre-slim → COPY *.jar app.jar    │
└─────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────┐
│  Stage 5: scan（安全掃描）                           │
│                                                     │
│  trivy image --exit-code 1 \                        │
│    --severity HIGH,CRITICAL \                       │
│    harbor.local/synapse/backend:${TAG}              │
│                                                     │
│  發現 CRITICAL 漏洞 → Pipeline 失敗（阻擋合併）      │
│  發現 HIGH 漏洞 → 警告（可設定是否阻擋）             │
│  無漏洞 → 繼續推送                                  │
└─────────────────────────────────────────────────────┘
    │ 掃描通過
    ▼
┌─────────────────────────────────────────────────────┐
│  Stage 6: push（推送到 Harbor）                      │
│                                                     │
│  docker push harbor.local/synapse/backend:${TAG}    │
│  docker push harbor.local/synapse/frontend:${TAG}   │
│  docker push harbor.local/saas/java-a:${TAG}        │
│  docker push harbor.local/saas/java-b:${TAG}        │
│                                                     │
│  docker tag ... harbor.local/.../latest             │
│  docker push ... latest                             │
└─────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────┐
│  Stage 7: deploy（更新 GitOps 倉庫）                 │
│                                                     │
│  # 更新 YAML 中的映像 Tag                           │
│  sed -i "s|image: harbor.local/.*/backend:.*|       │
│    image: harbor.local/synapse/backend:${TAG}|"     │
│    deploy/k8s/synapse-deployment.yaml               │
│                                                     │
│  git add deploy/k8s/                               │
│  git commit -m "ci: update image tag to ${TAG}"     │
│  git push                                           │
└─────────────────────────────────────────────────────┘
    │ Git push
    ▼
┌─────────────────────────────────────────────────────┐
│  ArgoCD 自動同步（約 3 分鐘輪詢或立即 Webhook）      │
│                                                     │
│  偵測到 deploy/k8s/ YAML 有變更                     │
│  → kubectl apply（滾動更新）                        │
│  → 等待 Deployment ready                            │
│  → 健康檢查通過 → Sync 狀態: Synced                  │
└─────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────┐
│  Kubernetes 滾動更新                                 │
│                                                     │
│  舊 Pod 保持服務 → 新 Pod 啟動                       │
│  readinessProbe 通過 → 流量切換到新 Pod             │
│  舊 Pod 優雅終止（terminationGracePeriodSeconds）    │
│                                                     │
│  Zero-downtime ✅                                   │
└─────────────────────────────────────────────────────┘
```

### 5.2 各應用的映像命名規則

| 應用 | Harbor 路徑 | Tag 格式 |
|------|-------------|----------|
| Synapse 後端 | `harbor.local/synapse/backend` | `${CI_COMMIT_SHORT_SHA}` |
| Synapse 前端 | `harbor.local/synapse/frontend` | `${CI_COMMIT_SHORT_SHA}` |
| Java 應用 A | `harbor.local/saas/java-a` | `${CI_COMMIT_SHORT_SHA}` |
| Java 應用 B | `harbor.local/saas/java-b` | `${CI_COMMIT_SHORT_SHA}` |

正式 release 另外打 `v1.2.3` tag，tag 推送自動觸發 release pipeline。

---

## 6. GitLab CI 設定範例

完整設定請參考專案根目錄的 `.gitlab-ci.yml`。核心結構如下：

```yaml
# .gitlab-ci.yml 結構概覽
stages:
  - lint         # 靜態分析
  - test         # 單元測試
  - build        # 編譯 / 打包
  - docker-build # 多階段 Docker Build
  - scan         # Trivy 安全掃描
  - push         # 推送到 Harbor
  - deploy       # 更新 GitOps YAML

variables:
  HARBOR_REGISTRY: "harbor.local"
  GO_VERSION: "1.22"
  NODE_VERSION: "20"
  JAVA_VERSION: "11"

# ── 靜態分析 ────────────────────────────────────────
lint:go:
  stage: lint
  image: golangci/golangci-lint:v1.57
  script:
    - golangci-lint run ./...

lint:frontend:
  stage: lint
  image: node:20-alpine
  script:
    - cd ui && npm ci && npm run lint

# ── 單元測試 ────────────────────────────────────────
test:go:
  stage: test
  image: golang:1.22-alpine
  script:
    - go test -race -coverprofile=coverage.out ./...
    - go tool cover -func=coverage.out
  coverage: '/total:\s+\(statements\)\s+(\d+\.\d+)%/'

test:frontend:
  stage: test
  image: node:20-alpine
  script:
    - cd ui && npm ci && npm run test

# ── Docker Build ────────────────────────────────────
docker:build:backend:
  stage: docker-build
  image: docker:24
  services: [docker:24-dind]
  script:
    - docker build
        -f deploy/docker/backend/Dockerfile
        -t $HARBOR_REGISTRY/synapse/backend:$CI_COMMIT_SHORT_SHA
        .

# ── Trivy 安全掃描 ───────────────────────────────────
scan:trivy:backend:
  stage: scan
  image:
    name: aquasec/trivy:latest
    entrypoint: [""]
  script:
    - trivy image
        --exit-code 1
        --severity HIGH,CRITICAL
        --no-progress
        $HARBOR_REGISTRY/synapse/backend:$CI_COMMIT_SHORT_SHA
  allow_failure: false   # CRITICAL 漏洞阻擋 Pipeline

# ── 推送到 Harbor ────────────────────────────────────
push:harbor:backend:
  stage: push
  image: docker:24
  services: [docker:24-dind]
  before_script:
    - docker login -u $HARBOR_USERNAME -p $HARBOR_PASSWORD $HARBOR_REGISTRY
  script:
    - docker push $HARBOR_REGISTRY/synapse/backend:$CI_COMMIT_SHORT_SHA
    - docker tag  $HARBOR_REGISTRY/synapse/backend:$CI_COMMIT_SHORT_SHA
                  $HARBOR_REGISTRY/synapse/backend:latest
    - docker push $HARBOR_REGISTRY/synapse/backend:latest

# ── GitOps 更新 ──────────────────────────────────────
deploy:update-gitops:
  stage: deploy
  image: alpine/git:latest
  only: [main, tags]
  script:
    - |
      sed -i "s|image: $HARBOR_REGISTRY/synapse/backend:.*|image: $HARBOR_REGISTRY/synapse/backend:$CI_COMMIT_SHORT_SHA|" \
        deploy/k8s/synapse-deployment.yaml
    - git config user.email "ci@synapse.local"
    - git config user.name  "GitLab CI"
    - git add deploy/k8s/
    - git diff --staged --quiet || git commit -m "ci: deploy $CI_COMMIT_SHORT_SHA [skip ci]"
    - git push "https://gitlab-ci-token:${CI_JOB_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git" HEAD:main
```

---

## 7. Synapse CI 引擎設定

Synapse 平台本身管理多個外部 CI 引擎連線，支援以下引擎類型：

### 7.1 引擎類型一覽

| 引擎 | 用途 | 認證方式 |
|------|------|----------|
| **GitLab CI** | GitLab 原生 Pipeline | API Token / PAT |
| **Jenkins** | Jenkins Job 觸發 | Basic Auth（帳號＋密碼）|
| **Tekton** | K8s 原生 CI Pipeline | Kubeconfig（叢集存取）|
| **Argo Workflows** | K8s 工作流 CI | Kubeconfig（叢集存取）|
| **GitHub Actions** | GitHub Workflow 觸發 | PAT Token |

### 7.2 在 Synapse 設定 CI 引擎

1. 登入 Synapse → 設定 → CI 引擎
2. 點擊「新增」，選擇引擎類型
3. 填入連線資訊：

**GitLab CI 範例：**
```
名稱:           company-gitlab
引擎類型:       GitLab CI
端點 URL:       https://gitlab.example.com
API Token:      glpat-xxxxxxxxxxxxxxxxxxxx
GitLab Project ID: 42
預設分支:       main
```

**Jenkins 範例：**
```
名稱:           company-jenkins
引擎類型:       Jenkins
端點 URL:       https://jenkins.example.com
使用者名稱:     admin
API Token:      <Jenkins API Token>
Job 路徑:       saas-apps/java-a
```

**Tekton 範例：**
```
名稱:           prod-tekton
引擎類型:       Tekton
目標叢集:       production-cluster
Pipeline 名稱:  build-and-push
命名空間:       tekton-pipelines
Service Account: pipeline
```

### 7.3 CI 引擎 API 端點

Synapse 提供以下 REST API 操作 CI 引擎執行：

```
GET    /api/v1/ci-engines/status              # 所有引擎健康狀態
GET    /api/v1/ci-engines                     # 列出所有引擎設定
POST   /api/v1/ci-engines                     # 新增引擎設定

POST   /api/v1/ci-engines/:id/runs            # 觸發執行
GET    /api/v1/ci-engines/:id/runs/:runId     # 查詢執行狀態
DELETE /api/v1/ci-engines/:id/runs/:runId     # 取消執行
GET    /api/v1/ci-engines/:id/runs/:runId/logs      # 取得 Log（text/plain）
GET    /api/v1/ci-engines/:id/runs/:runId/artifacts # 取得產出物列表
```

觸發範例：
```bash
curl -X POST https://synapse.example.com/api/v1/ci-engines/1/runs \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "ref": "main",
    "variables": {
      "DEPLOY_ENV": "production",
      "IMAGE_TAG": "v1.2.3"
    }
  }'
```

---

## 8. GitOps 部署（ArgoCD）

### 8.1 倉庫結構約定

```
deploy/k8s/
├── synapse-deployment.yaml        # Synapse 本體
├── saas-java-a-deployment.yaml    # Java 應用 A
└── saas-java-b-deployment.yaml    # Java 應用 B
```

CI Pipeline 在 push 階段自動更新 YAML 內的 `image:` 欄位，commit 後 ArgoCD 偵測變更自動同步。

### 8.2 ArgoCD Application 設定

```yaml
# deploy/examples/argocd-synapse-application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: synapse
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/Synapse.git
    targetRevision: main
    path: deploy/k8s
  destination:
    server: https://kubernetes.default.svc
    namespace: synapse
  syncPolicy:
    automated:
      prune: true      # 自動刪除 Git 中不存在的資源
      selfHeal: true   # 手動修改會被 ArgoCD 還原
    syncOptions:
      - CreateNamespace=true
```

```bash
# 手動觸發同步
argocd app sync synapse

# 查看同步狀態
argocd app get synapse

# 查看 diff
argocd app diff synapse
```

### 8.3 同步策略說明

| 策略 | 說明 | 建議環境 |
|------|------|----------|
| `automated + selfHeal` | 完全 GitOps，任何手動變更都會被還原 | Production |
| `automated` | 自動同步但允許手動 patch | Staging |
| 手動同步 | 需要人工確認才部署 | 有審核需求的環境 |

---

## 9. Kubernetes 部署規格

### 9.1 Synapse 後端

```yaml
# 關鍵設定節錄（完整見 deploy/k8s/synapse-deployment.yaml）
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"

livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
```

### 9.2 Java SaaS 應用

```yaml
# 關鍵設定節錄（完整見 deploy/k8s/saas-java-a-deployment.yaml）
resources:
  requests:
    memory: "512Mi"
    cpu: "250m"
  limits:
    memory: "1Gi"
    cpu: "1000m"

# HPA 自動擴縮
autoscaling:
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

# 反親和性（分散到不同節點）
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          topologyKey: kubernetes.io/hostname
```

### 9.3 Harbor 映像拉取 Secret

```bash
# 為各 Namespace 建立映像拉取憑證
for NS in synapse saas-java-a saas-java-b; do
  kubectl create secret docker-registry harbor-regcred \
    --docker-server=harbor.local \
    --docker-username=ci-robot \
    --docker-password=<ROBOT_TOKEN> \
    -n $NS
done
```

在 Deployment 中引用：
```yaml
spec:
  template:
    spec:
      imagePullSecrets:
        - name: harbor-regcred
```

### 9.4 滾動更新操作

```bash
# 手動更新映像版本
kubectl set image deployment/synapse-backend \
  backend=harbor.local/synapse/backend:v1.2.3 \
  -n synapse

# 查看滾動更新進度
kubectl rollout status deployment/synapse-backend -n synapse

# 回滾上一版本
kubectl rollout undo deployment/synapse-backend -n synapse

# 回滾到指定版本
kubectl rollout history deployment/synapse-backend -n synapse
kubectl rollout undo deployment/synapse-backend --to-revision=3 -n synapse
```

---

## 10. 監控與告警

### 10.1 Prometheus 指標端點

| 應用 | 指標路徑 | Port |
|------|----------|------|
| Synapse 後端 | `/metrics` | 8080 |
| Java 應用（Spring Boot） | `/actuator/prometheus` | 8080 |

### 10.2 關鍵監控指標

| 指標 | 告警閾值 | 說明 |
|------|----------|------|
| `go_goroutines` | > 10000 | Goroutine 洩漏 |
| `http_request_duration_seconds{p99}` | > 2s | API 回應延遲 |
| `http_requests_total{status=~"5.."}` | error rate > 1% | 伺服器錯誤率 |
| `process_resident_memory_bytes` | > 450MB | 記憶體使用 |
| `jvm_memory_used_bytes` | > 800MB | JVM 記憶體（Java） |

### 10.3 AlertManager 告警規則

```yaml
# deploy/monitoring/synapse-alerts.yaml
groups:
  - name: synapse-cicd
    rules:
      - alert: CIEnginePipelineFailureRate
        expr: |
          rate(ci_engine_runs_total{status="failed"}[5m]) /
          rate(ci_engine_runs_total[5m]) > 0.3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "CI engine failure rate > 30%"

      - alert: HarborStorageLow
        expr: harbor_storage_used_bytes / harbor_storage_total_bytes > 0.85
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "Harbor storage > 85%"
```

```bash
# 套用告警規則
kubectl apply -f deploy/monitoring/synapse-alerts.yaml
```

---

## 11. 故障排查

### 11.1 Pipeline 失敗排查

**問題：Docker Build 失敗**
```bash
# 在 Runner 機器上測試 Build
docker build -f deploy/docker/backend/Dockerfile . --progress=plain

# 常見原因
# 1. Go module cache 未命中 → 檢查 CI cache 設定
# 2. npm install 網路逾時 → 設定 npm registry 鏡像
# 3. Maven 下載慢 → 設定 settings.xml 使用內部 Maven 鏡像
```

**問題：Trivy 掃描阻擋**
```bash
# 本地執行掃描確認漏洞詳情
trivy image --severity HIGH,CRITICAL harbor.local/synapse/backend:latest

# 查看 CVE 詳情並升級基礎映像
# 例如：FROM alpine:3.19 → FROM alpine:3.20
```

**問題：Harbor 推送 401 Unauthorized**
```bash
# 確認 Robot Account Token 未過期
curl -u ci-robot:<TOKEN> https://harbor.local/api/v2.0/projects

# 重新建立 Robot Account 並更新 GitLab CI 變數
```

**問題：ArgoCD Sync 失敗**
```bash
# 查看同步錯誤
argocd app get synapse --show-operation

# 常見原因：YAML 格式錯誤
kubectl apply --dry-run=client -f deploy/k8s/synapse-deployment.yaml
```

### 11.2 K8s 部署問題

**Pod 無法啟動（ImagePullBackOff）**
```bash
kubectl describe pod -n synapse <pod-name>
# 確認 imagePullSecrets 已設定且 token 有效

# 測試拉取映像
kubectl run test --image=harbor.local/synapse/backend:latest --restart=Never -n synapse
```

**Pod CrashLoopBackOff**
```bash
# 查看當前 log
kubectl logs -n synapse <pod-name>

# 查看上一次崩潰 log
kubectl logs -n synapse <pod-name> --previous

# 查看 Event
kubectl get events -n synapse --sort-by='.lastTimestamp'
```

**資料庫連線失敗（`relation does not exist`）**
```bash
# 確認 Migration 是否執行
# Synapse 啟動時自動執行 RunMigrations()
# 若失敗，查看啟動 log
kubectl logs -n synapse deployment/synapse-backend | grep -i migration
```

### 11.3 CI 引擎連線問題

```bash
# 透過 Synapse API 檢查引擎健康狀態
curl -H "Authorization: Bearer <TOKEN>" \
  https://synapse.example.com/api/v1/ci-engines/status

# 回應範例
# {"items":[{"type":"gitlab","available":true,"version":"16.9.0"},
#           {"type":"jenkins","available":false,"message":"connection refused"}]}
```

---

## 12. 目錄結構說明

```
deploy/
├── CICD_GUIDE.md               ← 本文件（完整指南）
├── DEPLOYMENT.md               ← Synapse 本體部署（Helm / Docker / K8s）
├── docker-compose-cicd.yaml    ← 一鍵啟動完整 CICD 基礎設施
│
├── docker/                     ← Dockerfile 集合
│   ├── backend/Dockerfile      │ Synapse 後端多階段構建（Go → Alpine）
│   ├── frontend/Dockerfile     │ Synapse 前端多階段構建（Node → Nginx）
│   ├── saas-java-a/Dockerfile  │ Java 應用 A（Maven → OpenJDK-slim）
│   └── saas-java-b/Dockerfile  ← Java 應用 B
│
├── k8s/                        ← Kubernetes 部署 YAML（由 CI 自動更新 image tag）
│   ├── synapse-deployment.yaml │ Synapse 後端 + 前端 + ConfigMap
│   ├── saas-java-a-deployment.yaml  │ Java-A Deployment + HPA + RBAC
│   ├── saas-java-b-deployment.yaml  │ Java-B Deployment + HPA + RBAC
│   └── README.md               ← K8s 部署操作說明
│
├── examples/                   ← 範本與參考配置
│   ├── gitlab-ci-example.yml   │ Java 應用 GitLab CI 完整範例
│   ├── argocd-synapse-application.yaml  │ Synapse ArgoCD Application
│   ├── argocd-application-example.yaml  │ 通用 ArgoCD Application 範本
│   └── QUICKSTART.md           ← 5 分鐘快速開始
│
├── helm/                       ← Synapse Helm Chart
│   └── kubepolaris/            ← Chart 目錄（values.yaml、templates/）
│
├── monitoring/                 ← Prometheus 告警規則
│   └── synapse-alerts.yaml     ← CI 引擎 + 系統告警規則
│
├── sql/                        ← 資料庫相關
│   └── init_mysql.sql          ← MySQL 初始化腳本（Docker Compose 用）
│
├── config.example.yaml         ← Synapse 設定檔範本
├── INTEGRATION_GUIDE.md        ← Synapse + SaaS 應用完整集成指南
├── JENKINS_INTEGRATION.md      ← Jenkins 替代 GitLab CI 的設定指南
└── JAVA_APPS_INTEGRATION.md    ← Java 應用納入統一 Pipeline 的說明
```

---

## 附錄：快速命令速查

```bash
# ── 啟動 CICD 基礎設施 ──────────────────────────────────
docker compose -f deploy/docker-compose-cicd.yaml up -d
docker compose -f deploy/docker-compose-cicd.yaml ps

# ── Harbor ──────────────────────────────────────────────
docker login harbor.local -u admin -p Harbor@2026
docker push harbor.local/synapse/backend:v1.0.0

# ── ArgoCD ──────────────────────────────────────────────
argocd login localhost:8081 --username admin --insecure
argocd app sync synapse
argocd app get synapse

# ── K8s 部署操作 ─────────────────────────────────────────
kubectl apply -f deploy/k8s/synapse-deployment.yaml
kubectl get pods -A
kubectl rollout status deployment/synapse-backend -n synapse
kubectl rollout undo deployment/synapse-backend -n synapse

# ── Trivy 掃描 ───────────────────────────────────────────
trivy image --severity HIGH,CRITICAL harbor.local/synapse/backend:latest

# ── Synapse CI 引擎狀態 ──────────────────────────────────
curl -H "Authorization: Bearer $TOKEN" \
  https://synapse.example.com/api/v1/ci-engines/status
```
