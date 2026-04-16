# K8s CICD 测试环境部署指南

本目录包含用于 Synapse CICD 流程测试的 Kubernetes 部署文件。

## 📋 文件说明

### Docker Compose 方式

**文件**：`../docker-compose-cicd.yaml`

快速启动 GitLab、Harbor 和 ArgoCD：

```bash
cd ../
docker compose -f docker-compose-cicd.yaml up -d
```

**访问地址**：
- GitLab：http://localhost  (admin / Gitlab@2026)
- Harbor：http://localhost:8080  (admin / Harbor@2026)
- ArgoCD：http://localhost:8081  (admin / 需运行脚本初始化密码)

### Kubernetes 部署文件

#### 1. saas-java-a-deployment.yaml
最小的 Java Spring Boot 应用 A，包含：
- Namespace: `saas-java-a`
- 2 个副本（支持 HPA 自动扩缩）
- Health check (liveness & readiness probe)
- 资源限制与请求
- RBAC ServiceAccount
- ConfigMap 配置管理
- Pod 反亲和性（避免同节点调度）

**镜像要求**：`harbor.local/saas/java-a:latest`

#### 2. saas-java-b-deployment.yaml
最小的 Java Spring Boot 应用 B（同 A 的配置）

**镜像要求**：`harbor.local/saas/java-b:latest`

## 🚀 快速开始

### 前置条件
- Kubernetes 集群（1.21+）
- `kubectl` 命令行工具
- Harbor 私有镜像仓库已部署
- 已构建并推送应用镜像

### 步骤 1：部署 saas-java-a

```bash
kubectl apply -f saas-java-a-deployment.yaml
```

验证部署：
```bash
kubectl get pods -n saas-java-a
kubectl get svc -n saas-java-a
kubectl get hpa -n saas-java-a
```

### 步骤 2：部署 saas-java-b

```bash
kubectl apply -f saas-java-b-deployment.yaml
```

验证部署：
```bash
kubectl get pods -n saas-java-b
kubectl get svc -n saas-java-b
```

### 步骤 3：查看应用日志

```bash
# saas-java-a
kubectl logs -n saas-java-a deployment/saas-java-a -f

# saas-java-b
kubectl logs -n saas-java-b deployment/saas-java-b -f
```

### 步骤 4：配置 ArgoCD GitOps

1. 在 ArgoCD 添加 Git 仓库源
2. 创建 Application，指向本目录
3. ArgoCD 会自动同步并部署应用

**示例 ArgoCD Application**：
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: saas-java-a
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/Synapse
    targetRevision: main
    path: deploy/k8s
  destination:
    server: https://kubernetes.default.svc
    namespace: saas-java-a
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## 🔧 常见操作

### 扩缩副本数

```bash
# 手动扩展到 5 个副本
kubectl scale deployment saas-java-a --replicas=5 -n saas-java-a

# 查看 HPA 状态
kubectl get hpa -n saas-java-a
kubectl describe hpa saas-java-a -n saas-java-a
```

### 更新镜像版本

```bash
kubectl set image deployment/saas-java-a \
  app=harbor.local/saas/java-a:v1.1.0 \
  -n saas-java-a
```

### 滚动更新

```bash
kubectl rollout status deployment/saas-java-a -n saas-java-a
kubectl rollout history deployment/saas-java-a -n saas-java-a
kubectl rollout undo deployment/saas-java-a -n saas-java-a
```

### 查看应用健康状态

```bash
kubectl describe pod -n saas-java-a
kubectl exec -it <pod-name> -n saas-java-a -- /bin/sh
```

## 🛠️ 自定义配置

### 修改副本数

编辑 YAML 文件：
```yaml
spec:
  replicas: 3  # 修改此值
```

### 修改资源限制

```yaml
resources:
  requests:
    memory: "512Mi"  # 增加内存请求
    cpu: "500m"
  limits:
    memory: "1Gi"    # 增加内存限制
    cpu: "1000m"
```

### 修改 HPA 配置

```yaml
spec:
  minReplicas: 1       # 最小副本数
  maxReplicas: 20      # 最大副本数
  metrics:
  - resource:
      name: cpu
      target:
        averageUtilization: 50  # CPU 触发阈值
```

### 修改应用配置

编辑 ConfigMap：
```bash
kubectl edit configmap saas-java-a-config -n saas-java-a
```

重启 Pod 使配置生效：
```bash
kubectl rollout restart deployment/saas-java-a -n saas-java-a
```

## 📊 监控指标

### Prometheus 监控

应用暴露 Prometheus metrics 端点：`/actuator/prometheus`

配置 Prometheus：
```yaml
scrape_configs:
- job_name: 'saas-java-a'
  kubernetes_sd_configs:
  - role: pod
    namespaces:
      names:
      - saas-java-a
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
    action: keep
    regex: true
  - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
    action: replace
    target_label: __metrics_path__
    regex: (.+)
```

## 🐛 故障排查

### Pod 无法启动

```bash
# 查看详细错误信息
kubectl describe pod <pod-name> -n saas-java-a

# 查看容器日志
kubectl logs <pod-name> -n saas-java-a
kubectl logs <pod-name> -n saas-java-a --previous  # 前一个容器日志
```

### 镜像拉取失败

```bash
# 创建 Harbor 凭证 Secret
kubectl create secret docker-registry regcred \
  --docker-server=harbor.local \
  --docker-username=admin \
  --docker-password=Harbor@2026 \
  -n saas-java-a

# 在部署中添加 imagePullSecrets
imagePullSecrets:
- name: regcred
```

### 健康检查失败

```bash
# 直接测试端点
kubectl port-forward svc/saas-java-a 8080:80 -n saas-java-a
curl http://localhost:8080/actuator/health
```

## 📝 CICD 集成

### GitLab CI/CD 流程示例

`.gitlab-ci.yml` 示例：
```yaml
stages:
  - build
  - push
  - deploy

build:
  stage: build
  image: maven:3.8-openjdk-11
  script:
    - mvn clean package
  artifacts:
    paths:
      - target/*.jar

push:
  stage: push
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker login -u admin -p Harbor@2026 harbor.local
    - docker build -t harbor.local/saas/java-a:$CI_COMMIT_SHA .
    - docker push harbor.local/saas/java-a:$CI_COMMIT_SHA

deploy:
  stage: deploy
  image: alpine/k8s:1.24.0
  script:
    - kubectl set image deployment/saas-java-a app=harbor.local/saas/java-a:$CI_COMMIT_SHA -n saas-java-a
```

## 📚 参考资源

- [Kubernetes 官方文档](https://kubernetes.io/docs/)
- [ArgoCD 文档](https://argo-cd.readthedocs.io/)
- [Spring Boot 健康检查](https://spring.io/guides/gs/actuator-service/)

---

**最后更新**：2026-04-16
