# Synapse CICD 架构总结

完整的 **Synapse 项目自动化 CICD 部署流程** 快速参考指南。

## 📁 创建的文件清单

### 1️⃣ 核心 CICD 配置

| 文件 | 说明 | 路径 |
|------|------|------|
| `.gitlab-ci.yml` | Synapse CI/CD 主配置（Lint→Test→Build→Push→Deploy） | 项目根目录 |
| `Makefile` | 本地开发和构建脚本 | 项目根目录 |

### 2️⃣ Docker 构建文件

| 文件 | 用途 | 路径 |
|------|------|------|
| `backend/Dockerfile` | Synapse 后端多阶段构建（Go → Alpine） | `deploy/docker/` |
| `frontend/Dockerfile` | Synapse 前端多阶段构建（Node → Nginx） | `deploy/docker/` |
| `saas-java-a/Dockerfile` | Java 应用 A 构建 | `deploy/docker/` |
| `saas-java-b/Dockerfile` | Java 应用 B 构建 | `deploy/docker/` |

### 3️⃣ Kubernetes 部署配置

| 文件 | 组件 | 路径 |
|------|------|------|
| `synapse-deployment.yaml` | Synapse 完整部署（后端 + 前端 + MySQL） | `deploy/k8s/` |
| `saas-java-a-deployment.yaml` | Java 应用 A K8s 配置 | `deploy/k8s/` |
| `saas-java-b-deployment.yaml` | Java 应用 B K8s 配置 | `deploy/k8s/` |
| `README.md` | K8s 部署详细指南 | `deploy/k8s/` |

### 4️⃣ GitOps 配置

| 文件 | 说明 | 路径 |
|------|------|------|
| `argocd-synapse-application.yaml` | ArgoCD 应用配置（自动同步 Synapse） | `deploy/examples/` |
| `argocd-application-example.yaml` | ArgoCD 通用应用模板 | `deploy/examples/` |

### 5️⃣ CICD 基础设施

| 文件 | 说明 | 路径 |
|------|------|------|
| `docker-compose-cicd.yaml` | GitLab + Harbor + ArgoCD 完整栈 | `deploy/` |

### 6️⃣ 文档

| 文件 | 内容 | 路径 |
|------|------|------|
| `CICD_QUICKSTART.md` | Synapse CICD 快速开始指南 | `deploy/` |
| `INTEGRATION_GUIDE.md` | Synapse + SaaS 应用完整集成指南 | `deploy/` |
| `CICD_SUMMARY.md` | 本文档 | `deploy/` |
| `QUICKSTART.md` | SaaS 应用快速开始 | `deploy/examples/` |

## 🚀 完整流程（5 分钟快速开始）

### Step 1: 启动基础设施

```bash
cd deploy/
docker compose -f docker-compose-cicd.yaml up -d

# 等待服务启动
sleep 30

# 验证
docker compose ps
```

### Step 2: 部署应用到 K8s

```bash
# Synapse 本体
kubectl apply -f deploy/k8s/synapse-deployment.yaml

# SaaS 应用
kubectl apply -f deploy/k8s/saas-java-a-deployment.yaml
kubectl apply -f deploy/k8s/saas-java-b-deployment.yaml

# 验证
kubectl get pods -A
```

### Step 3: 配置 ArgoCD

```bash
# 部署 ArgoCD 应用配置（自动同步）
kubectl apply -f deploy/examples/argocd-synapse-application.yaml

# 查看状态
argocd app get synapse
```

### Step 4: 验证部署

```bash
# Synapse 前端
kubectl port-forward -n synapse svc/synapse-frontend 8080:80
# 访问：http://localhost:8080

# Synapse API
kubectl port-forward -n synapse svc/synapse-backend 8081:8080
# 访问：http://localhost:8081/healthz
```

## 📊 CICD 工作流

```
Developer: git push
    ↓
GitLab: 触发 CI Pipeline
    ├─ 1. Lint: 程式碼检查 (golangci-lint, ESLint)
    ├─ 2. Test: 单元测试 (go test, npm test)
    ├─ 3. Build: 编译构建 (go build, npm run build)
    ├─ 4. Push: 推送镜像 (docker push → Harbor)
    └─ 5. Deploy: 更新配置 (git push deploy/k8s/synapse-deployment.yaml)
    ↓
Git Webhook 通知 ArgoCD
    ↓
ArgoCD: 自动同步
    ├─ Detect: 检测 Git 变更
    ├─ Diff: 对比集群状态
    ├─ Apply: kubectl apply 新配置
    └─ Monitor: 监控部署状态
    ↓
Kubernetes: 滚动更新
    ├─ 拉取新镜像
    ├─ 启动新 Pod
    ├─ 健康检查
    └─ 删除旧 Pod
    ↓
✅ 完成：应用自动部署到生产环境
```

## 🔧 常用命令

### 本地开发

```bash
# 完整 CI 流程（本地模拟）
make ci

# 仅 lint
make lint

# 仅测试
make test

# 构建镜像
make docker-all

# 推送镜像
make docker-push

# 启动 CICD 基础设施
make docker-up
make docker-down
```

### Kubernetes 操作

```bash
# 部署
kubectl apply -f deploy/k8s/synapse-deployment.yaml

# 查看状态
kubectl get pods -n synapse -w
kubectl get svc -n synapse

# 查看日志
kubectl logs -n synapse deployment/synapse-backend -f

# 端口转发
kubectl port-forward -n synapse svc/synapse-frontend 8080:80

# 滚动更新
kubectl rollout restart deployment/synapse-backend -n synapse

# 回滚
kubectl rollout undo deployment/synapse-backend -n synapse
```

### ArgoCD 操作

```bash
# 查看应用状态
argocd app get synapse

# 手动同步
argocd app sync synapse

# 查看日志
argocd app logs synapse

# 回滚
argocd app rollback synapse <revision>
```

## 🐳 Docker 镜像

### 构建的镜像

| 镜像 | 来源 | Harbor 路径 |
|------|------|-----------|
| `synapse:backend-latest` | `deploy/docker/backend/Dockerfile` | `harbor.local/synapse/backend:latest` |
| `synapse:frontend-latest` | `deploy/docker/frontend/Dockerfile` | `harbor.local/synapse/frontend:latest` |
| `saas/java-a:latest` | `deploy/docker/saas-java-a/Dockerfile` | `harbor.local/saas/java-a:latest` |
| `saas/java-b:latest` | `deploy/docker/saas-java-b/Dockerfile` | `harbor.local/saas/java-b:latest` |

### Dockerfile 多阶段构建

#### 后端（Go + Alpine）
```
Stage 1: golang:1.25-alpine
  - go mod download
  - go build
  ↓
Stage 2: alpine:latest
  - 仅包含二进制文件
  - 健康检查
  - 非 root 用户
```

#### 前端（Node + Nginx）
```
Stage 1: node:22-alpine
  - npm ci
  - npm run build
  ↓
Stage 2: nginx:alpine
  - 仅包含 dist 文件
  - Gzip 压缩
  - SPA 路由配置
```

## 📋 K8s 资源清单

### Synapse 部署包含

```yaml
# 命名空间
Namespace: synapse

# 计算资源
Deployment: synapse-backend    (2 replicas, HPA: 2-10)
Deployment: synapse-frontend   (2 replicas, HPA: 2-5)
StatefulSet: synapse-mysql     (1 replica)

# 网络
Service: synapse-backend       (ClusterIP)
Service: synapse-frontend      (LoadBalancer)
Service: synapse-mysql         (ClusterIP, headless)
Ingress: synapse               (synapse.local)

# 配置
ConfigMap: synapse-backend-config
ConfigMap: synapse-frontend-config
Secret: synapse-secrets        (加密密钥、凭证)

# RBAC
ServiceAccount: synapse
ClusterRole: synapse
ClusterRoleBinding: synapse

# 自动扩缩
HPA: synapse-backend           (CPU 70%, Memory 80%)
HPA: synapse-frontend          (CPU 80%)
```

## 🌍 访问地址

### 开发环境

| 服务 | 地址 | 用户名 | 密码 |
|------|------|--------|------|
| GitLab | http://localhost | admin | Gitlab@2026 |
| Harbor | http://localhost:8080 | admin | Harbor@2026 |
| ArgoCD | http://localhost:8081 | admin | (初始密码) |

### 生产环境（K8s）

| 服务 | 方式 | 地址 |
|------|------|------|
| Synapse 前端 | Ingress | http://synapse.local |
| Synapse API | Service | http://synapse-backend:8080 |
| MySQL | Service | synapse-mysql.synapse.svc.cluster.local:3306 |

## 📊 性能和可靠性指标

### 构建时间

| 阶段 | 耗时 |
|------|------|
| Lint | 2-3 分钟 |
| Test | 5-10 分钟 |
| Build | 3-5 分钟 |
| Push | 2-3 分钟 |
| **总计** | **12-21 分钟** |

### 镜像大小

| 镜像 | 大小 |
|------|------|
| synapse:backend | ~150MB |
| synapse:frontend | ~50MB |
| saas-java-a | ~400MB |
| saas-java-b | ~400MB |

### 部署时间

| 操作 | 耗时 |
|------|------|
| 初始部署 | 2-3 分钟 |
| 滚动更新 | 1-2 分钟 |
| 自动扩缩 | 30-60 秒 |

## ✅ 部署检查清单

- [ ] Docker Compose 服务正常运行
- [ ] kubectl 集群连接正常
- [ ] Harbor 镜像推送成功
- [ ] K8s Namespace 创建成功
- [ ] Synapse Pod 状态为 Running
- [ ] MySQL 初始化完成
- [ ] 应用健康检查通过
- [ ] Ingress 配置正确
- [ ] ArgoCD 应用同步成功
- [ ] 前端页面可访问
- [ ] API 端点响应正常

## 🔒 安全配置

### 已实现

- ✅ 非 root 用户运行容器
- ✅ 只读 root 文件系统（前端）
- ✅ 敏感数据存储在 Secret
- ✅ 健康检查（Liveness + Readiness）
- ✅ 资源限制（CPU + Memory）
- ✅ Pod 反亲和性（避免单点故障）
- ✅ RBAC 权限控制

### 建议增强

- 启用 Pod Security Policy / Pod Security Standards
- 实施网络策略（NetworkPolicy）
- 启用镜像扫描（Trivy）
- 配置 HTTPS/TLS
- 启用审计日志
- 定期备份 MySQL 数据

## 📚 文档导航

| 文档 | 适用场景 |
|------|---------|
| `CICD_QUICKSTART.md` | Synapse 项目的 CICD 快速开始 |
| `INTEGRATION_GUIDE.md` | Synapse + SaaS 应用的完整集成 |
| `deploy/k8s/README.md` | K8s 部署详细操作指南 |
| `deploy/examples/QUICKSTART.md` | SaaS 应用快速开始 |
| `Makefile` | 本地开发命令参考 |

## 🆘 快速问题排查

| 问题 | 排查步骤 |
|------|---------|
| 镜像推送失败 | 检查 Harbor 连接：`docker login harbor.local` |
| K8s 部署失败 | 查看事件：`kubectl describe pod <pod>` |
| ArgoCD 同步失败 | 检查 Git：`argocd repo get <repo>` |
| MySQL 连接错误 | 查看日志：`kubectl logs synapse-mysql-0` |
| 应用无法启动 | 检查镜像：`docker image inspect harbor.local/synapse/backend` |

## 📞 获取帮助

```bash
# 查看 Makefile 命令
make help

# 查看 kubectl 资源
kubectl get all -n synapse

# 查看 ArgoCD 应用
argocd app list

# 查看 GitLab Pipeline
gitlab-runner --debug run
```

## 🎯 后续优化方向

1. **性能优化**
   - [ ] 启用镜像层缓存
   - [ ] 使用 Harbor 镜像加速
   - [ ] 前端 CDN 分发

2. **可靠性**
   - [ ] 配置多副本 MySQL（Replication）
   - [ ] 启用 Redis 缓存
   - [ ] 实施灾难恢复计划（DR）

3. **监控告警**
   - [ ] 部署 Prometheus + Grafana
   - [ ] 配置 AlertManager
   - [ ] 集成 Slack 通知

4. **安全加固**
   - [ ] 启用 TLS/HTTPS
   - [ ] 配置 OAuth2 认证
   - [ ] 镜像漏洞扫描

5. **成本优化**
   - [ ] 资源预留和限制调优
   - [ ] 使用 Spot Instances
   - [ ] 定期清理镜像

---

**版本**：1.0  
**最后更新**：2026-04-16  
**维护者**：Synapse 团队
