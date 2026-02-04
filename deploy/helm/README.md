# KubePolaris Helm Chart 部署指南

## 📚 文档目录

本目录包含 KubePolaris 的 Kubernetes Helm Chart 部署资源。

### 📁 目录结构

```
deploy/helm/kubepolaris/
├── Chart.yaml                    # Chart 元数据
├── values.yaml                   # 默认配置值
├── values-ha.yaml                # 高可用配置示例
├── values-production.yaml        # 生产环境配置示例
├── README.md                     # Chart 使用说明
├── quick-deploy.sh               # 快速部署脚本
├── .helmignore                   # Helm 忽略文件
└── templates/                    # Kubernetes 模板文件
    ├── NOTES.txt                 # 安装后显示的信息
    ├── _helpers.tpl              # 模板辅助函数
    ├── configmap.yaml            # 配置文件
    ├── secret.yaml               # 密钥
    ├── serviceaccount.yaml       # 服务账号
    ├── rbac.yaml                 # RBAC 权限
    ├── mysql-statefulset.yaml    # MySQL StatefulSet
    ├── mysql-service.yaml        # MySQL Service
    ├── mysql-pvc.yaml            # MySQL PVC
    ├── backend-deployment.yaml   # 后端 Deployment
    ├── backend-service.yaml      # 后端 Service
    ├── frontend-deployment.yaml  # 前端 Deployment
    ├── frontend-service.yaml     # 前端 Service
    ├── ingress.yaml              # Ingress
    ├── hpa.yaml                  # 水平扩展
    ├── pdb.yaml                  # Pod 中断预算
    └── tests/                    # 测试
        └── test-connection.yaml
```

## 🚀 快速开始

### 方式一：使用快速部署脚本（推荐）

```bash
cd deploy/helm/kubepolaris
./quick-deploy.sh
```

脚本会自动：
- ✅ 检查环境依赖（kubectl、helm）
- ✅ 生成安全密钥
- ✅ 创建命名空间
- ✅ 部署所有组件
- ✅ 等待服务就绪

### 方式二：使用 Helm 手动部署

```bash
# 1. 创建命名空间
kubectl create namespace kubepolaris

# 2. 安装 Chart
helm install kubepolaris deploy/helm/kubepolaris \
  --namespace kubepolaris \
  --set security.jwtSecret="$(openssl rand -base64 32)"

# 3. 查看状态
helm status kubepolaris -n kubepolaris
kubectl get pods -n kubepolaris
```

### 方式三：使用 Makefile

```bash
# 验证 Chart
make helm-lint

# 安装
make helm-install

# 打包
make helm-package

# 卸载
make helm-uninstall
```

## 🎯 部署场景

### 场景 1: 基础部署（开发/测试）

使用内置 MySQL，最小资源配置：

```bash
helm install kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  --create-namespace \
  --set security.jwtSecret="your-secure-jwt-secret"
```

### 场景 2: 生产环境部署

使用外部数据库和 Ingress：

```bash
# 创建 Secret
kubectl create secret generic kubepolaris-mysql \
  --from-literal=password=your_mysql_password \
  -n kubepolaris

kubectl create secret generic kubepolaris-secrets \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  -n kubepolaris

# 安装
helm install kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  -f deploy/helm/kubepolaris/values-production.yaml \
  --set mysql.external.host=your-mysql-host.example.com
```

### 场景 3: 高可用部署

3 副本 + 反亲和 + 自动扩缩容：

```bash
helm install kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  -f deploy/helm/kubepolaris/values-ha.yaml
```

## 📊 配置说明

### 核心配置参数

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `backend.replicaCount` | 后端副本数 | `2` |
| `frontend.replicaCount` | 前端副本数 | `2` |
| `mysql.internal.enabled` | 启用内置 MySQL | `true` |
| `mysql.external.enabled` | 使用外部 MySQL | `false` |
| `grafana.enabled` | 启用内置 Grafana | `true` |
| `grafana.dashboards.enabled` | 启用 Dashboard 自动导入 | `true` |
| `grafana.datasource.prometheusUrl` | Prometheus 数据源地址 | `http://your-prometheus:9090` |
| `ingress.enabled` | 启用 Ingress | `false` |
| `security.jwtSecret` | JWT 密钥（必填） | `""` |
| `rbac.create` | 创建 RBAC 资源 | `true` |
| `autoscaling.backend.enabled` | 启用后端 HPA | `false` |
| `podDisruptionBudget.enabled` | 启用 PDB | `false` |

### Grafana 配置

KubePolaris 内置了 Grafana 用于监控可视化。Grafana 的配置包括：

#### 1. 启用/禁用 Grafana

```bash
# 禁用内置 Grafana（使用外部 Grafana）
helm install kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  --set grafana.enabled=false \
  --set grafana.external.enabled=true \
  --set grafana.external.url="http://your-grafana:3000"
```

#### 2. 配置 Prometheus 数据源

Grafana 需要连接到 Prometheus 来获取监控数据：

```bash
# 设置 Prometheus 地址
helm install kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  --set grafana.datasource.prometheusUrl="http://prometheus-server:9090"
```

#### 3. Dashboard 自动导入

Chart 包含了 3 个预定义的 Dashboard：
- **K8s 集群总览**: 集群整体资源使用情况
- **Pod 详情监控**: Pod 级别的详细监控
- **工作负载详情**: Deployment/StatefulSet 等工作负载监控

这些 Dashboard 会在部署时自动导入到 Grafana 的 `KubePolaris` 文件夹中。

如果不需要自动导入 Dashboard：

```bash
helm install kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  --set grafana.dashboards.enabled=false
```

#### 4. 持久化存储

默认情况下，Grafana 使用 emptyDir，重启后数据会丢失。生产环境建议启用持久化：

```bash
helm install kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  --set grafana.persistence.enabled=true \
  --set grafana.persistence.size=5Gi \
  --set grafana.persistence.storageClass=your-storage-class
```

#### 5. 访问 Grafana

Grafana 默认通过子路径 `/grafana/` 访问：

```bash
# Port Forward 方式
kubectl port-forward -n kubepolaris svc/kubepolaris-grafana 3000:3000

# 访问地址: http://localhost:3000/grafana/
# 默认用户名: admin
# 默认密码: 在安装时通过 --set grafana.adminPassword 设置
```

#### 6. Grafana 配置示例

```yaml
# values-custom.yaml
grafana:
  enabled: true
  
  # 管理员密码
  adminPassword: "your-secure-password"
  
  # 持久化存储
  persistence:
    enabled: true
    size: 5Gi
    storageClass: "standard"
  
  # Dashboard 配置
  dashboards:
    enabled: true
  
  # Prometheus 数据源
  datasource:
    prometheusUrl: "http://prometheus-kube-prometheus-prometheus:9090"
  
  # 资源限制
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 250m
      memory: 256Mi
```

然后使用自定义配置安装：

```bash
helm install kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  -f values-custom.yaml
```

### 配置文件说明

- **values.yaml**: 默认配置，适合开发测试
- **values-ha.yaml**: 高可用配置，适合生产环境
- **values-production.yaml**: 生产配置示例，需自定义

## 🔧 常用操作

### 查看状态

```bash
# Helm release 状态
helm status kubepolaris -n kubepolaris

# Pod 状态
kubectl get pods -n kubepolaris

# 服务状态
kubectl get svc -n kubepolaris

# 所有资源
kubectl get all -n kubepolaris
```

### 查看日志

```bash
# 后端日志
kubectl logs -f -l app.kubernetes.io/component=backend -n kubepolaris

# 前端日志
kubectl logs -f -l app.kubernetes.io/component=frontend -n kubepolaris

# MySQL 日志
kubectl logs -f -l app.kubernetes.io/component=mysql -n kubepolaris
```

### 访问应用

```bash
# Port Forward
kubectl port-forward -n kubepolaris svc/kubepolaris-frontend 8080:80

# 浏览器访问
# http://localhost:8080
# 用户名: admin
# 密码: KubePolaris@2026
```

### 升级

```bash
# 升级到新版本
helm upgrade kubepolaris deploy/helm/kubepolaris \
  -n kubepolaris \
  -f values.yaml

# 查看升级历史
helm history kubepolaris -n kubepolaris

# 回滚
helm rollback kubepolaris 1 -n kubepolaris
```

### 卸载

```bash
# 卸载 Chart
helm uninstall kubepolaris -n kubepolaris

# 删除 PVC（注意：会删除所有数据）
kubectl delete pvc -l app.kubernetes.io/instance=kubepolaris -n kubepolaris

# 删除命名空间
kubectl delete namespace kubepolaris
```

## 🧪 测试

```bash
# 运行 Helm 测试
helm test kubepolaris -n kubepolaris

# 手动测试连接
kubectl run test-connection --rm -i --tty \
  --image=busybox:1.36 \
  --restart=Never \
  -n kubepolaris \
  -- wget -qO- kubepolaris-backend:8080/healthz
```

## 📦 打包和发布

### 本地打包

```bash
# 验证 Chart
helm lint deploy/helm/kubepolaris

# 打包
helm package deploy/helm/kubepolaris -d dist/

# 生成索引
helm repo index dist/ --url https://your-repo-url
```

### 模板渲染测试

```bash
# 渲染模板
helm template kubepolaris deploy/helm/kubepolaris \
  --namespace kubepolaris \
  --set security.jwtSecret="test-secret" \
  > rendered.yaml

# 验证渲染结果
kubectl apply --dry-run=client -f rendered.yaml
```

## 🔍 故障排查

### Pod 无法启动

```bash
# 查看 Pod 事件
kubectl describe pod -l app.kubernetes.io/instance=kubepolaris -n kubepolaris

# 查看 Pod 日志
kubectl logs -l app.kubernetes.io/instance=kubepolaris -n kubepolaris --all-containers=true
```

### 数据库连接失败

```bash
# 检查 MySQL Pod
kubectl get pod -l app.kubernetes.io/component=mysql -n kubepolaris

# 检查 Secret
kubectl get secret kubepolaris-mysql -n kubepolaris -o yaml

# 测试数据库连接
kubectl run mysql-client --rm -i --tty \
  --image=mysql:8.0 \
  --restart=Never \
  -n kubepolaris \
  -- mysql -h kubepolaris-mysql -u kubepolaris -p
```

### Ingress 无法访问

```bash
# 检查 Ingress
kubectl describe ingress kubepolaris -n kubepolaris

# 检查 Ingress Controller
kubectl get pods -n ingress-nginx
```

## 📚 参考文档

- [Helm 官方文档](https://helm.sh/docs/)
- [Kubernetes 官方文档](https://kubernetes.io/docs/)
- [KubePolaris 文档](https://kubepolaris.clay-wangzhi.com/docs)
- [Chart README](./kubepolaris/README.md)

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

Apache License 2.0

---

**维护者:** KubePolaris Team  
**更新时间:** 2026-01-23
