# Synapse CICD 中集成 Java 应用部署

在 Synapse 系统的 CICD 流程中自动化部署 `saas-java-a` 和 `saas-java-b` 两个应用。

## 📋 项目结构

```
Synapse Repository (单一 Git 项目)
├── .gitlab-ci.yml                    (Synapse + Java 应用统一 CICD 配置)
├── cmd/                              (Synapse 后端)
├── ui/                               (Synapse 前端)
├── saas-java-a/                      (Java 应用 A)
│   ├── src/
│   ├── pom.xml
│   └── (无需单独的 .gitlab-ci.yml)
├── saas-java-b/                      (Java 应用 B)
│   ├── src/
│   ├── pom.xml
│   └── (无需单独的 .gitlab-ci.yml)
├── deploy/
│   ├── docker/
│   │   ├── backend/
│   │   ├── frontend/
│   │   ├── saas-java-a/
│   │   └── saas-java-b/
│   └── k8s/
│       ├── synapse-deployment.yaml
│       ├── saas-java-a-deployment.yaml
│       └── saas-java-b-deployment.yaml
```

## 🚀 工作流

```
git push (main 分支)
         ↓
.gitlab-ci.yml 触发 Pipeline
         ├─ Lint
         │  ├─ golangci-lint (Synapse 后端)
         │  └─ ESLint (Synapse 前端)
         ├─ Build
         │  ├─ go build (Synapse 后端)
         │  ├─ npm run build (Synapse 前端)
         │  ├─ mvn clean package (Java-A)
         │  └─ mvn clean package (Java-B)
         ├─ Push
         │  ├─ docker push synapse/backend
         │  ├─ docker push synapse/frontend
         │  ├─ docker push saas/java-a
         │  └─ docker push saas/java-b
         └─ Deploy
            ├─ 更新 deploy/k8s/synapse-deployment.yaml
            ├─ 更新 deploy/k8s/saas-java-a-deployment.yaml
            ├─ 更新 deploy/k8s/saas-java-b-deployment.yaml
            └─ git push (触发 ArgoCD)
                     ↓
            ArgoCD 自动同步
                     ↓
            ✅ Kubernetes 自动部署所有应用
```

## 📦 CICD 配置详解

### 1️⃣ 构建阶段（Build）

`.gitlab-ci.yml` 中添加 Java 应用构建：

```yaml
build:maven:java-a:
  stage: build
  image: maven:3.8-openjdk-11-slim
  only:
    - main
    - tags
  script:
    - cd saas-java-a
    - mvn clean package -DskipTests
  artifacts:
    paths:
      - saas-java-a/target/*.jar
    expire_in: 1 day

build:maven:java-b:
  stage: build
  image: maven:3.8-openjdk-11-slim
  only:
    - main
    - tags
  script:
    - cd saas-java-b
    - mvn clean package -DskipTests
  artifacts:
    paths:
      - saas-java-b/target/*.jar
    expire_in: 1 day
```

### 2️⃣ 推送阶段（Push）

`.gitlab-ci.yml` 中添加 Java 应用镜像推送：

```yaml
push:docker:java-a:
  stage: push
  image: docker:latest
  services:
    - docker:dind
  only:
    - main
    - tags
  dependencies:
    - build:maven:java-a
  before_script:
    - docker login -u $HARBOR_USERNAME -p $HARBOR_PASSWORD $HARBOR_REGISTRY
  script:
    - docker build -f deploy/docker/saas-java-a/Dockerfile \
        -t ${HARBOR_REGISTRY}/saas/java-a:${IMAGE_TAG} saas-java-a/
    - docker push ${HARBOR_REGISTRY}/saas/java-a:${IMAGE_TAG}
    - docker tag ${HARBOR_REGISTRY}/saas/java-a:${IMAGE_TAG} \
        ${HARBOR_REGISTRY}/saas/java-a:latest
    - docker push ${HARBOR_REGISTRY}/saas/java-a:latest

push:docker:java-b:
  stage: push
  image: docker:latest
  services:
    - docker:dind
  only:
    - main
    - tags
  dependencies:
    - build:maven:java-b
  before_script:
    - docker login -u $HARBOR_USERNAME -p $HARBOR_PASSWORD $HARBOR_REGISTRY
  script:
    - docker build -f deploy/docker/saas-java-b/Dockerfile \
        -t ${HARBOR_REGISTRY}/saas/java-b:${IMAGE_TAG} saas-java-b/
    - docker push ${HARBOR_REGISTRY}/saas/java-b:${IMAGE_TAG}
    - docker tag ${HARBOR_REGISTRY}/saas/java-b:${IMAGE_TAG} \
        ${HARBOR_REGISTRY}/saas/java-b:latest
    - docker push ${HARBOR_REGISTRY}/saas/java-b:latest
```

### 3️⃣ 部署阶段（Deploy）

`.gitlab-ci.yml` 中的 Deploy 阶段自动更新 K8s 配置：

```yaml
deploy:update:k8s:config:
  stage: deploy
  image: alpine:latest
  only:
    - main
  script:
    - apk add --no-cache git curl jq

    # 配置 Git
    - git config --global user.email "cicd@synapse.local"
    - git config --global user.name "GitLab CI/CD"

    # 更新所有应用镜像版本
    - sed -i "s|image: .*synapse/backend:.*|image: $BACKEND_IMAGE|g" \
        deploy/k8s/synapse-deployment.yaml
    - sed -i "s|image: .*synapse/frontend:.*|image: $FRONTEND_IMAGE|g" \
        deploy/k8s/synapse-deployment.yaml
    - sed -i "s|image: .*saas/java-a:.*|image: ${HARBOR_REGISTRY}/saas/java-a:${IMAGE_TAG}|g" \
        deploy/k8s/saas-java-a-deployment.yaml
    - sed -i "s|image: .*saas/java-b:.*|image: ${HARBOR_REGISTRY}/saas/java-b:${IMAGE_TAG}|g" \
        deploy/k8s/saas-java-b-deployment.yaml

    # 提交并推送（触发 ArgoCD）
    - git add deploy/k8s/
    - git commit -m "ci: update all apps to ${IMAGE_TAG}"
    - git push origin main
```

## ☸️ Kubernetes 部署配置

### saas-java-a-deployment.yaml

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: saas-java-a

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: saas-java-a
  namespace: saas-java-a
spec:
  replicas: 2
  selector:
    matchLabels:
      app: saas-java-a
  template:
    metadata:
      labels:
        app: saas-java-a
    spec:
      containers:
      - name: app
        image: harbor.local/saas/java-a:latest  # 由 CI/CD 自动更新
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /actuator/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10

---
apiVersion: v1
kind: Service
metadata:
  name: saas-java-a
  namespace: saas-java-a
spec:
  type: ClusterIP
  selector:
    app: saas-java-a
  ports:
  - port: 80
    targetPort: 8080
```

### saas-java-b-deployment.yaml

同 saas-java-a，但替换名称为 `saas-java-b`

## 🔄 ArgoCD 自动同步

创建 `argocd-synapse-applications.yaml`：

```yaml
---
# Synapse 应用
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
      prune: true
      selfHeal: true

---
# Java 应用 A
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: saas-java-a
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/Synapse.git
    targetRevision: main
    path: deploy/k8s
  destination:
    server: https://kubernetes.default.svc
    namespace: saas-java-a
  syncPolicy:
    automated:
      prune: true
      selfHeal: true

---
# Java 应用 B
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: saas-java-b
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/Synapse.git
    targetRevision: main
    path: deploy/k8s
  destination:
    server: https://kubernetes.default.svc
    namespace: saas-java-b
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## 🚀 快速开始

### Step 1: 准备项目结构

```bash
# 假设已有 Synapse 项目
cd /path/to/Synapse

# 添加 Java 应用
mkdir -p saas-java-a saas-java-b

# 从现有项目复制（或新建）
# 确保有 pom.xml 和 src/ 目录
```

### Step 2: 启动 CICD 基础设施

```bash
make docker-up
# 或
cd deploy && docker compose -f docker-compose-cicd.yaml up -d
```

### Step 3: 部署初始应用到 K8s

```bash
# 部署 Synapse 和 Java 应用
kubectl apply -f deploy/k8s/synapse-deployment.yaml
kubectl apply -f deploy/k8s/saas-java-a-deployment.yaml
kubectl apply -f deploy/k8s/saas-java-b-deployment.yaml

# 配置 ArgoCD
kubectl apply -f deploy/examples/argocd-synapse-applications.yaml
```

### Step 4: 提交代码

```bash
# 添加所有文件到 Git
git add .
git commit -m "feat: add saas-java-a and saas-java-b to Synapse CICD"
git push origin main
```

**自动触发**：
1. ✅ GitLab CI Pipeline 执行（Lint → Test → Build → Push → Deploy）
2. ✅ 所有应用镜像推送到 Harbor
3. ✅ K8s 配置文件更新
4. ✅ ArgoCD 检测到 Git 变更
5. ✅ 所有应用自动部署到 Kubernetes

## 📊 验证部署

```bash
# 查看 Pipeline 执行
# https://localhost/Synapse/-/pipelines

# 查看 Harbor 镜像
# http://localhost:8080

# 查看 ArgoCD 应用状态
argocd app get synapse
argocd app get saas-java-a
argocd app get saas-java-b

# 查看 K8s Pod
kubectl get pods -A

# 查看应用日志
kubectl logs -n saas-java-a deployment/saas-java-a -f
kubectl logs -n saas-java-b deployment/saas-java-b -f
```

## 🔧 修改应用代码并自动部署

### 场景：修改 saas-java-a 代码

```bash
# 1. 修改代码
vi saas-java-a/src/main/java/com/saas/Application.java

# 2. 提交并推送
git add .
git commit -m "feat: update Java-A version"
git push origin main

# 3. 自动触发的流程（无需手动操作）：
#    ✅ Maven 编译 saas-java-a
#    ✅ Docker 构建镜像
#    ✅ 推送到 Harbor
#    ✅ 更新 deploy/k8s/saas-java-a-deployment.yaml
#    ✅ Git push 触发 ArgoCD
#    ✅ ArgoCD 自动同步
#    ✅ K8s 滚动更新 Pod

# 4. 监控部署进度
kubectl rollout status deployment/saas-java-a -n saas-java-a
```

## 🐛 常见问题

### Q: Java 应用编译失败怎么办？

A: 在 `.gitlab-ci.yml` 中添加测试：

```yaml
test:java:
  stage: test
  image: maven:3.8-openjdk-11-slim
  script:
    - cd saas-java-a
    - mvn clean test
```

### Q: 想要跳过某个应用的部署？

A: 在 `.gitlab-ci.yml` 中添加条件：

```yaml
push:docker:java-a:
  only:
    - main
    - tags
  # 只有提交信息包含 "java-a" 时才执行
  # script: ...
```

### Q: 如何回滚特定应用？

A: 使用 git revert 和 ArgoCD：

```bash
# 查看历史版本
git log --oneline deploy/k8s/saas-java-a-deployment.yaml

# 回滚到上一个版本
git revert HEAD

# 或使用 ArgoCD
argocd app rollback saas-java-a
```

## 📚 相关文件

- **`.gitlab-ci.yml`** - 完整 CICD 配置（Synapse + Java 应用）
- **`deploy/k8s/synapse-deployment.yaml`** - Synapse K8s 配置
- **`deploy/k8s/saas-java-a-deployment.yaml`** - Java-A K8s 配置
- **`deploy/k8s/saas-java-b-deployment.yaml`** - Java-B K8s 配置
- **`deploy/docker/saas-java-a/Dockerfile`** - Java-A Docker 镜像
- **`deploy/docker/saas-java-b/Dockerfile`** - Java-B Docker 镜像

---

**关键点**：
- ✅ 一个 `.gitlab-ci.yml` 管理所有应用
- ✅ 自动构建、推送、部署所有应用
- ✅ ArgoCD 自动同步 K8s 配置
- ✅ 无需手动部署命令

**最后更新**：2026-04-16
