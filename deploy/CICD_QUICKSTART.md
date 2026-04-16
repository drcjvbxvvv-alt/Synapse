# Synapse CICD 自动化部署指南

完整的 Synapse 项目自动化部署流程：从程式碼提交到生产环境的端到端 CICD 管道。

## 📋 架构概览

```
┌─────────────────┐
│  Git 程式碼提交    │
│  (GitLab)       │
└────────┬────────┘
         │ Webhook trigger
         ▼
┌─────────────────┐
│  CI 流程        │
│  • 程式碼检查      │
│  • 单元测试      │
│  • 构建镜像      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  推送到 Harbor  │
│  镜像仓库       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  更新 K8s 配置   │
│  (git push)     │
└────────┬────────┘
         │ Git webhook
         ▼
┌─────────────────┐
│  ArgoCD 自动同步│
│  • 检测配置变更  │
│  • 应用到集群    │
│  • 监控部署状态  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  生产环境       │
│  Synapse 运行   │
└─────────────────┘
```

## 🚀 快速开始（5 分钟）

### 1. 启动 CICD 基础设施

```bash
cd deploy/

# 启动 GitLab、Harbor、ArgoCD
docker compose -f docker-compose-cicd.yaml up -d

# 查看服务状态
docker compose ps
```

**访问地址**：

| 服务 | URL | 用户名 | 密码 |
|------|-----|--------|------|
| GitLab | http://localhost | admin | Gitlab@2026 |
| Harbor | http://localhost:8080 | admin | Harbor@2026 |
| ArgoCD | http://localhost:8081 | admin | (见下方初始化) |

### 2. 初始化 ArgoCD

```bash
# 获取初始密码
docker exec -it argocd argocd admin initial-password -n argocd

# 登录
argocd login localhost:8081 --username admin

# 或在 Web UI 登录：http://localhost:8081
```

### 3. 配置 GitLab Runner

```bash
# 在 GitLab 中注册 Runner
docker exec -it gitlab-runner gitlab-runner register \
  --url http://gitlab.local/ \
  --registration-token <registration-token-from-gitlab> \
  --executor docker \
  --docker-image docker:latest \
  --docker-privileged

# 或使用已配置的 Runner（docker-compose 中已包含）
```

### 4. 部署 Synapse 到 K8s

```bash
# 方式 A：手动部署（用于测试）
kubectl apply -f deploy/k8s/synapse-deployment.yaml

# 方式 B：通过 ArgoCD（推荐生产环境）
kubectl apply -f deploy/examples/argocd-synapse-application.yaml

# 验证部署
kubectl get pods -n synapse
kubectl get svc -n synapse
```

### 5. 验证应用

```bash
# 检查后端
kubectl logs -n synapse deployment/synapse-backend -f

# 检查前端
kubectl logs -n synapse deployment/synapse-frontend -f

# 端口转发（本地测试）
kubectl port-forward -n synapse svc/synapse-frontend 8080:80
kubectl port-forward -n synapse svc/synapse-backend 8081:8080

# 访问应用
# 前端：http://localhost:8080
# 后端 API：http://localhost:8081
```

## 📦 CICD 流程详解

### Stage 1: 程式碼检查 (Lint)

```bash
# 后端：Go 程式碼检查
golangci-lint run ./cmd ./internal ./pkg

# 前端：ESLint + i18n 检查
npm run lint
npm run i18n-lint
```

**触发条件**：
- ✅ 所有分支（main、feature/* 等）
- ✅ MR（Merge Request）

### Stage 2: 单元测试 (Test)

```bash
# 后端：Go 单元测试
go test -v -cover ./cmd/... ./internal/...

# 前端：Jest 单元测试
npm run test:coverage
```

**触发条件**：
- ✅ main 分支
- ✅ MR

**覆盖率报告**：
- 后端：`coverage.out`
- 前端：`ui/coverage/`

### Stage 3: 构建 (Build)

```bash
# 后端：编译 Go 二进制
make build VERSION=${CI_COMMIT_SHA}

# 前端：Vite 构建
npm run build
```

**触发条件**：
- ✅ main 分支
- ✅ Git tags

**产物**：
- `bin/synapse`（后端）
- `ui/dist/`（前端）

### Stage 4: 推送镜像 (Push)

```bash
# 后端镜像
docker build -f deploy/docker/backend/Dockerfile -t harbor.local/synapse/backend:${SHA} .
docker push harbor.local/synapse/backend:${SHA}
docker tag harbor.local/synapse/backend:${SHA} harbor.local/synapse/backend:latest
docker push harbor.local/synapse/backend:latest

# 前端镜像
docker build -f deploy/docker/frontend/Dockerfile -t harbor.local/synapse/frontend:${SHA} .
docker push harbor.local/synapse/frontend:${SHA}
docker tag harbor.local/synapse/frontend:${SHA} harbor.local/synapse/frontend:latest
docker push harbor.local/synapse/frontend:latest
```

**触发条件**：
- ✅ main 分支
- ✅ Git tags

### Stage 5: 部署 (Deploy)

#### 方案 A：自动更新 K8s 配置

```bash
# 更新镜像版本
sed -i "s|image: .*synapse/backend:.*|image: harbor.local/synapse/backend:${SHA}|g" \
  deploy/k8s/synapse-deployment.yaml

# 提交到 Git
git add deploy/k8s/synapse-deployment.yaml
git commit -m "ci: update Synapse images to ${SHA}"
git push origin main
```

#### 方案 B：触发 ArgoCD 同步

```bash
# 手动同步（手动触发 CI 时）
curl -X POST \
  -H "Authorization: Bearer ${ARGOCD_TOKEN}" \
  "http://argocd.local:8081/api/v1/applications/synapse/sync"

# ArgoCD 自动监听 Git 变更，自动同步
```

**触发条件**：
- ✅ main 分支

## 🔧 配置和环境变量

### GitLab CI 变量（CI/CD Settings → Variables）

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `HARBOR_USERNAME` | Harbor 用户名 | admin |
| `HARBOR_PASSWORD` | Harbor 密码 | Harbor@2026 |
| `ARGOCD_TOKEN` | ArgoCD API Token | <token> |
| `KUBE_CONFIG_BASE64` | Base64 kubeconfig | <encoded> |

### 获取 ARGOCD_TOKEN

```bash
argocd account generate-token --account argocd --duration 0
```

### 获取 KUBE_CONFIG_BASE64

```bash
cat ~/.kube/config | base64 | tr -d '\n'
```

## 📊 监控部署

### 查看 CICD 日志

```bash
# GitLab CI 日志（Web UI）
# https://localhost/Synapse/-/pipelines

# 或命令行
gitlab-runner verify
gitlab-runner --debug run
```

### 查看 ArgoCD 部署状态

```bash
# 命令行
argocd app get synapse
argocd app logs synapse

# Web UI
# http://localhost:8081/applications/synapse
```

### 查看 K8s 部署

```bash
# 查看 Pod 状态
kubectl get pods -n synapse -w

# 查看 Deployment 状态
kubectl rollout status deployment/synapse-backend -n synapse

# 查看事件日志
kubectl describe deployment synapse-backend -n synapse

# 查看容器日志
kubectl logs -n synapse deployment/synapse-backend --tail=100 -f
```

## 🐛 常见问题与排查

### 问题 1：镜像推送失败

```bash
# 检查 Harbor 连接
docker login harbor.local -u admin -p Harbor@2026

# 查看 Harbor 日志
docker logs harbor

# 确认网络
docker network inspect cicd
```

### 问题 2：K8s 部署失败

```bash
# 检查镜像拉取
kubectl describe pod <pod-name> -n synapse

# 查看初始化容器日志
kubectl logs <pod-name> -c wait-for-mysql -n synapse

# 检查配置挂载
kubectl exec -it <pod-name> -n synapse -- cat /app/config/config.yaml
```

### 问题 3：ArgoCD 同步失败

```bash
# 检查 Git 凭证
argocd repo list
argocd repo get https://github.com/your-org/Synapse.git

# 查看同步详情
argocd app get synapse --refresh
argocd app logs synapse

# 手动触发同步
argocd app sync synapse --prune
```

### 问题 4：MySQL 连接失败

```bash
# 检查 MySQL Pod
kubectl get pods -n synapse | grep mysql

# 查看 MySQL 日志
kubectl logs -n synapse synapse-mysql-0

# 测试连接
kubectl run -it --rm debug --image=mysql:8.0 --restart=Never -- \
  mysql -h synapse-mysql -u synapse -p synapse123 -e "SELECT 1"
```

## 🔄 手动回滚

### 快速回滚到上一个版本

```bash
# 方法 1：使用 ArgoCD（推荐）
argocd app rollback synapse

# 方法 2：手动更新 K8s 配置
kubectl set image deployment/synapse-backend \
  backend=harbor.local/synapse/backend:previous-tag \
  -n synapse

# 方法 3：通过 Git 回滚
git revert HEAD
git push origin main
# ArgoCD 自动同步
```

## 📈 性能监控

### Prometheus 指标

```bash
# 获取 metrics
curl http://localhost:8080/metrics

# 常用指标
# http_requests_total  - HTTP 请求总数
# http_request_duration_seconds - 请求耗时
# go_goroutines - Go 协程数
```

### Grafana 仪表板

```bash
# 访问 Grafana
# http://localhost:3000
# 用户名：admin
# 密码：admin

# 配置数据源：Prometheus
# http://prometheus:9090
```

## 🎯 完整工作流示例

### 场景：开发新功能并自动部署

```bash
# 1. 创建功能分支
git checkout -b feature/new-dashboard

# 2. 修改程式碼
vi ui/src/pages/Dashboard.tsx
vi internal/handlers/dashboard.go

# 3. 提交并推送
git add .
git commit -m "feat: add new dashboard page"
git push origin feature/new-dashboard

# 4. 创建 MR（GitLab Web UI）
# https://localhost/Synapse/-/merge_requests

# 5. 等待 CI 检查
# - 程式碼检查 (Lint)
# - 单元测试 (Test)
# - 构建 (Build) - 可选
# ✓ 全部通过后才能合并

# 6. 合并 MR
# GitLab Web UI 或命令行

# 7. 自动部署（main 分支）
# - CI 构建
# - 推送镜像
# - 更新 K8s 配置
# - ArgoCD 自动同步
# - 部署到生产环境

# 8. 验证部署
kubectl rollout status deployment/synapse-backend -n synapse
kubectl logs -n synapse deployment/synapse-backend -f

# 9. 监控应用
argocd app get synapse
curl http://synapse.local/api/health
```

## 🚀 优化建议

### 1. 加速构建

```bash
# 使用 Docker buildx 并行构建
docker buildx build --platform linux/amd64,linux/arm64 ...

# 启用 Go 模块缓存
export GOMODCACHE=/root/go/pkg/mod

# 前端增量构建
npm ci --prefer-offline --no-audit
```

### 2. 优化镜像大小

```bash
# 后端：多阶段构建（已实现）
# 前端：nginx alpine + gzip 压缩（已实现）

# 检查镜像大小
docker images | grep synapse
```

### 3. 安全加固

```bash
# 扫描镜像漏洞（可选）
trivy image harbor.local/synapse/backend:latest

# 启用镜像签名
cosign sign --key cosign.key harbor.local/synapse/backend:latest

# 使用 Pod Security Policy
kubectl apply -f deploy/k8s/pod-security-policy.yaml
```

## 📚 相关文件

| 文件 | 说明 |
|------|------|
| `.gitlab-ci.yml` | GitLab CI/CD 主配置 |
| `deploy/docker/backend/Dockerfile` | 后端镜像构建 |
| `deploy/docker/frontend/Dockerfile` | 前端镜像构建 |
| `deploy/k8s/synapse-deployment.yaml` | K8s 部署配置 |
| `deploy/examples/argocd-synapse-application.yaml` | ArgoCD 应用配置 |
| `Makefile` | 本地构建脚本 |

## 🔗 相关资源

- [GitLab CI/CD 文档](https://docs.gitlab.com/ee/ci/)
- [Harbor 用户指南](https://goharbor.io/docs/)
- [ArgoCD 文档](https://argo-cd.readthedocs.io/)
- [Kubernetes 部署最佳实践](https://kubernetes.io/docs/concepts/configuration/overview/)

---

**最后更新**：2026-04-16
**版本**：1.0
