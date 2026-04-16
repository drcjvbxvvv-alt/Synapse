# Synapse CICD 完整集成指南

本指南展示如何将 **Synapse 项目**、**两个 SaaS Java 应用** 和 **CICD 基础设施** 整合到一个完整的端到端流程中。

## 🏗️ 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                     Git Repository (GitLab)                 │
├─────────────────────────────────────────────────────────────┤
│  main branch                                                │
│  ├── .gitlab-ci.yml         (Synapse CICD 配置)            │
│  ├── deploy/k8s/                                           │
│  │   ├── synapse-deployment.yaml                           │
│  │   ├── saas-java-a-deployment.yaml                       │
│  │   └── saas-java-b-deployment.yaml                       │
│  ├── deploy/docker/                                        │
│  │   ├── backend/Dockerfile                                │
│  │   ├── frontend/Dockerfile                               │
│  │   ├── saas-java-a/Dockerfile                            │
│  │   └── saas-java-b/Dockerfile                            │
│  └── src/                                                  │
│      ├── cmd/ (后端 Go 程式碼)                                │
│      ├── internal/                                         │
│      └── ui/ (前端 React 程式碼)                              │
└─────────────────────────────────────────────────────────────┘
                            │
                   (Webhook trigger)
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                 GitLab CI (Pipeline)                        │
├─────────────────────────────────────────────────────────────┤
│ Stages:                                                     │
│  1️⃣ Lint       (golangci-lint, ESLint)                     │
│  2️⃣ Test       (go test, npm test)                         │
│  3️⃣ Build      (go build, npm run build)                   │
│  4️⃣ Push       (docker push to Harbor)                     │
│  5️⃣ Deploy     (update K8s config, trigger ArgoCD)        │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│               Harbor (Docker Registry)                      │
├─────────────────────────────────────────────────────────────┤
│  synapse/backend:sha256 → synapse/backend:latest            │
│  synapse/frontend:sha256 → synapse/frontend:latest          │
│  saas/java-a:sha256 → saas/java-a:latest                   │
│  saas/java-b:sha256 → saas/java-b:latest                   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│            Git Repository (K8s Config Update)              │
├─────────────────────────────────────────────────────────────┤
│  deploy/k8s/synapse-deployment.yaml                        │
│  image: harbor.local/synapse/backend:new-sha              │
└─────────────────────────────────────────────────────────────┘
                            │
                   (Git Webhook to ArgoCD)
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   ArgoCD (GitOps)                           │
├─────────────────────────────────────────────────────────────┤
│  1. Detect Git changes                                     │
│  2. Diff with cluster state                                │
│  3. Apply manifests (kubectl apply)                        │
│  4. Monitor health & sync status                           │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│            Kubernetes Cluster (Production)                  │
├─────────────────────────────────────────────────────────────┤
│  Namespace: synapse                                        │
│  ├── Deployment: synapse-backend (Pod 1, Pod 2)           │
│  ├── Deployment: synapse-frontend (Pod 1, Pod 2)          │
│  ├── StatefulSet: synapse-mysql (Pod 0)                   │
│  ├── Service: synapse-backend, synapse-frontend           │
│  ├── ConfigMap: synapse-backend-config                    │
│  ├── Secret: synapse-secrets                              │
│  ├── HPA: synapse-backend, synapse-frontend               │
│  └── Ingress: synapse.local                               │
│                                                             │
│  Namespace: saas-java-a                                    │
│  ├── Deployment: saas-java-a                              │
│  ├── Service, HPA, RBAC ...                               │
│                                                             │
│  Namespace: saas-java-b                                    │
│  ├── Deployment: saas-java-b                              │
│  ├── Service, HPA, RBAC ...                               │
└─────────────────────────────────────────────────────────────┘
```

## 🚀 分钟级快速启动

### 前置条件
- Docker 和 Docker Compose
- Kubernetes 集群（minikube / Docker Desktop K8s / 实际集群）
- `kubectl` 和 `helm` 命令行工具
- Go 1.25+ 和 Node.js 22+

### 第 1 步：启动 CICD 基础设施（2 分钟）

```bash
cd deploy/

# 启动 GitLab、Harbor、ArgoCD
docker compose -f docker-compose-cicd.yaml up -d

# 等待服务启动
sleep 30

# 验证
docker compose ps

# 访问服务
# GitLab:  http://localhost          (admin / Gitlab@2026)
# Harbor:  http://localhost:8080     (admin / Harbor@2026)
# ArgoCD:  http://localhost:8081     (admin / <initial password>)
```

### 第 2 步：初始化 ArgoCD（1 分钟）

```bash
# 获取初始密码
ARGOCD_PASSWORD=$(docker exec argocd argocd admin initial-password -n argocd | head -1)
echo "ArgoCD password: $ARGOCD_PASSWORD"

# 登录（可选，用于命令行）
argocd login localhost:8081 \
  --username admin \
  --password $ARGOCD_PASSWORD \
  --insecure

# 或在 Web UI 登录：http://localhost:8081
```

### 第 3 步：部署 Synapse 和应用到 K8s（2 分钟）

```bash
# 部署 Synapse 本体
kubectl apply -f deploy/k8s/synapse-deployment.yaml

# 部署两个 SaaS Java 应用
kubectl apply -f deploy/k8s/saas-java-a-deployment.yaml
kubectl apply -f deploy/k8s/saas-java-b-deployment.yaml

# 验证部署
kubectl get pods -n synapse -w
kubectl get pods -n saas-java-a -w
kubectl get pods -n saas-java-b -w
```

### 第 4 步：配置 ArgoCD Applications（1 分钟）

```bash
# 部署 ArgoCD Application（自动同步 Synapse）
kubectl apply -f deploy/examples/argocd-synapse-application.yaml
kubectl apply -f deploy/examples/argocd-application-example.yaml

# 查看应用状态
argocd app list
argocd app get synapse
```

## 📊 完整工作流示例

### 场景 1：修改 Synapse 程式碼并自动部署

```bash
# 1. 修改 Synapse 后端程式碼
vi cmd/main.go
vi internal/handlers/example.go

# 2. 提交并推送
git add .
git commit -m "feat: add new API endpoint"
git push origin main

# 3. GitLab 自动触发 CI
# .gitlab-ci.yml 执行：
#   - Lint (golangci-lint)
#   - Test (go test)
#   - Build (go build)
#   - Push (docker push to harbor.local/synapse/backend:sha256)
#   - Deploy (update deploy/k8s/synapse-deployment.yaml)

# 4. Git webhook 通知 ArgoCD
# ArgoCD 自动：
#   - 检测 deploy/k8s/synapse-deployment.yaml 变更
#   - 对比 Git 和集群状态
#   - 应用新镜像版本
#   - 滚动更新 Pod

# 5. 验证部署
kubectl rollout status deployment/synapse-backend -n synapse
curl http://localhost/healthz

# ✅ 完成！新程式碼已自动部署到生产环境
```

### 场景 2：修改前端程式碼

```bash
# 1. 修改 React 程式碼
vi ui/src/pages/Dashboard.tsx

# 2. 提交并推送
git add .
git commit -m "feat: redesign dashboard UI"
git push origin main

# 3. GitLab CI 执行：
#   - Lint (npm run lint)
#   - Test (npm run test:coverage)
#   - Build (npm run build)
#   - Push (docker push to harbor.local/synapse/frontend:sha256)

# 4. ArgoCD 自动同步前端

# 5. 验证
kubectl logs -n synapse deployment/synapse-frontend -f
curl http://localhost
```

### 场景 3：部署 saas-java-a 应用

```bash
# 1. 创建 saas-java-a 项目（在 GitLab 中）

# 2. 上传程式碼和部署文件
# 项目结构：
# saas-java-a/
# ├── src/
# ├── pom.xml
# ├── .gitlab-ci.yml  (如 deploy/examples/gitlab-ci-example.yml)
# ├── Dockerfile      (如 deploy/docker/saas-java-a/Dockerfile)

# 3. 推送程式碼
git push origin main

# 4. GitLab CI 自动：
#   - 编译 Maven 项目
#   - 构建 Docker 镜像
#   - 推送到 Harbor

# 5. 手动或自动部署到 K8s
kubectl set image deployment/saas-java-a \
  app=harbor.local/saas/java-a:new-sha \
  -n saas-java-a

# 或配置 ArgoCD 自动同步
kubectl apply -f - << EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: saas-java-a
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/saas-java-a
    targetRevision: main
    path: deploy/k8s
  destination:
    server: https://kubernetes.default.svc
    namespace: saas-java-a
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
EOF
```

## 🔧 GitLab CI/CD 配置说明

### 环境变量配置

在 GitLab 项目设置中添加：

```
Settings → CI/CD → Variables
```

| 变量 | 值 | 说明 |
|------|-----|------|
| `HARBOR_USERNAME` | admin | Harbor 用户名 |
| `HARBOR_PASSWORD` | Harbor@2026 | Harbor 密码 |
| `ARGOCD_TOKEN` | <token> | ArgoCD API Token |
| `KUBE_CONFIG_BASE64` | <encoded> | Base64 kubeconfig |

### 获取 ARGOCD_TOKEN

```bash
argocd account generate-token --account argocd --duration 0
```

### 获取 KUBE_CONFIG_BASE64

```bash
cat ~/.kube/config | base64 | tr -d '\n'
```

## 📈 监控和排查

### 查看 CI 日志

```bash
# GitLab Web UI
# https://localhost/your-project/-/pipelines

# 或查看特定 job 日志
gitlab-runner verify
```

### 查看 ArgoCD 同步状态

```bash
# Web UI
# http://localhost:8081/applications

# 命令行
argocd app get synapse
argocd app logs synapse
argocd app wait synapse --sync
```

### 查看 K8s 部署

```bash
# 查看 Pod
kubectl get pods -A -w

# 查看 Deployment
kubectl get deployments -A

# 查看事件
kubectl get events -n synapse --sort-by='.lastTimestamp'

# 查看日志
kubectl logs -n synapse deployment/synapse-backend -f
```

### 常见问题排查

#### 1. 镜像拉取失败

```bash
# 检查镜像拉取凭证
kubectl get secret -n synapse

# 创建 Harbor 凭证
kubectl create secret docker-registry regcred \
  --docker-server=harbor.local \
  --docker-username=admin \
  --docker-password=Harbor@2026 \
  -n synapse

# 在 deployment 中添加
imagePullSecrets:
- name: regcred
```

#### 2. MySQL 连接失败

```bash
# 检查 MySQL 状态
kubectl get pods -n synapse | grep mysql

# 查看 MySQL 日志
kubectl logs -n synapse synapse-mysql-0

# 测试连接
kubectl run -it --rm debug --image=mysql:8.0 --restart=Never -- \
  mysql -h synapse-mysql.synapse.svc.cluster.local -u synapse -p synapse123
```

#### 3. ArgoCD 同步失败

```bash
# 检查 Git 凭证
argocd repo list

# 手动同步
argocd app sync synapse

# 查看同步详情
argocd app wait synapse --sync
```

## 🔄 手动回滚

### 回滚 Synapse

```bash
# 方法 1：使用 ArgoCD（推荐）
argocd app rollback synapse <revision-id>

# 方法 2：使用 kubectl
kubectl rollout undo deployment/synapse-backend -n synapse

# 方法 3：通过 Git 回滚（GitOps）
git revert HEAD
git push origin main
# ArgoCD 自动同步
```

## 📚 文件清单

| 文件 | 说明 | 位置 |
|------|------|------|
| `.gitlab-ci.yml` | Synapse CICD 配置 | 项目根目录 |
| `Makefile` | 本地开发脚本 | 项目根目录 |
| `docker-compose-cicd.yaml` | CICD 基础设施 | `deploy/` |
| `Dockerfile` (后端) | 后端镜像构建 | `deploy/docker/backend/` |
| `Dockerfile` (前端) | 前端镜像构建 | `deploy/docker/frontend/` |
| `synapse-deployment.yaml` | K8s 部署配置 | `deploy/k8s/` |
| `saas-java-a-deployment.yaml` | Java 应用 A | `deploy/k8s/` |
| `saas-java-b-deployment.yaml` | Java 应用 B | `deploy/k8s/` |
| `argocd-synapse-application.yaml` | ArgoCD 应用配置 | `deploy/examples/` |
| `gitlab-ci-example.yml` | Java 应用 CI 示例 | `deploy/examples/` |

## 🎯 最佳实践

### 1. 程式碼评审流程

```
Feature Branch
    ↓
Push to GitLab
    ↓
创建 MR (Merge Request)
    ↓
CI 自动运行 (lint, test, build)
    ↓
❌ CI 失败? → 修复并重新推送
    ↓
✅ CI 通过? → 程式碼评审 (Code Review)
    ↓
❌ 评审未通过? → 修改并推送
    ↓
✅ 评审通过? → 合并到 main
    ↓
自动部署到生产环境（ArgoCD）
```

### 2. 版本管理

```bash
# 使用 Git tags 管理版本
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# CI 会自动为 tag 构建镜像
# harbor.local/synapse/backend:v1.0.0
```

### 3. 环境隔离

```
Development (本地)
    ↓ git push feature/*
Staging (dev branch)
    ↓ CI runs on dev
    ↓ ArgoCD syncs to staging namespace
    ↓ Manual approval
Production (main branch)
    ↓ git merge main
    ↓ CI runs on main
    ↓ ArgoCD syncs to production namespace
```

### 4. 故障恢复

```
生产问题
    ↓
通知团队 (Slack/Email)
    ↓
立即回滚到上一个稳定版本
    ↓
调查根本原因
    ↓
修复并推送
    ↓
通过 staging 验证
    ↓
推送到 production
```

## 📖 相关资源

- [Synapse CICD 快速开始](./CICD_QUICKSTART.md)
- [K8s 部署指南](./k8s/README.md)
- [GitLab CI/CD 文档](https://docs.gitlab.com/ee/ci/)
- [ArgoCD 文档](https://argo-cd.readthedocs.io/)
- [Harbor 用户指南](https://goharbor.io/docs/)

---

**最后更新**：2026-04-16  
**版本**：1.0
