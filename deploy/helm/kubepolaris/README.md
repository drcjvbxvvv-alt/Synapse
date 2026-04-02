# Synapse Helm Chart

[![Version](https://img.shields.io/badge/version-1.0.0-blue)](https://github.com/clay-wangzhi/Synapse)
[![Type](https://img.shields.io/badge/type-application-informational)](https://helm.sh/docs/topics/charts/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](https://github.com/clay-wangzhi/Synapse/blob/main/LICENSE)

企业级 Kubernetes 多集群管理平台 Helm Chart

## 📖 简介

Synapse 是一个现代化的 Kubernetes 集群管理平台，提供直观的 Web 界面来管理和监控多个 Kubernetes 集群。

**主要特性:**

- ✅ 多集群统一管理
- ✅ 工作负载管理（Deployment/StatefulSet/DaemonSet 等）
- ✅ Pod 管理和日志查看
- ✅ 节点管理和操作
- ✅ Web 终端（Pod/Kubectl/SSH）
- ✅ Prometheus/Grafana 集成
- ✅ RBAC 权限控制
- ✅ 操作审计日志

## 🚀 快速开始

### 前置要求

- Kubernetes 1.20+
- Helm 3.0+
- PV provisioner（如果启用持久化存储）

### 添加 Helm 仓库

```bash
helm repo add synapse https://clay-wangzhi.github.io/Synapse
helm repo update
```

### 安装 Chart

```bash
# 基础安装
helm install synapse synapse/synapse \
  --namespace synapse \
  --create-namespace

# 查看安装状态
helm status synapse -n synapse
```

### 访问应用

```bash
# 使用 port-forward 访问
kubectl port-forward -n synapse svc/synapse-frontend 8080:80

# 在浏览器中打开
# http://localhost:8080

# 默认登录信息
# 用户名: admin
# 密码: Synapse@2026
```

## 📋 配置

### values.yaml 关键配置

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `backend.replicaCount` | 后端副本数 | `2` |
| `frontend.replicaCount` | 前端副本数 | `2` |
| `mysql.internal.enabled` | 启用内置 MySQL | `true` |
| `mysql.external.enabled` | 使用外部 MySQL | `false` |
| `ingress.enabled` | 启用 Ingress | `false` |
| `security.jwtSecret` | JWT 密钥（可选，留空自动生成） | `""` |
| `rbac.create` | 创建 RBAC 资源 | `true` |

### 完整配置

查看 [values.yaml](./values.yaml) 获取所有可配置参数。

## 🎯 部署场景

### 场景 1: 基础部署（使用内置 MySQL）

```bash
helm install synapse synapse/synapse \
  -n synapse \
  --set security.jwtSecret="your-secure-jwt-secret-at-least-32-chars"
```

### 场景 2: 使用外部数据库

```bash
# 1. 创建数据库 Secret
kubectl create secret generic synapse-mysql \
  --from-literal=password=your_mysql_password \
  -n synapse

# 2. 安装
helm install synapse synapse/synapse \
  -n synapse \
  --set mysql.internal.enabled=false \
  --set mysql.external.enabled=true \
  --set mysql.external.host=mysql.example.com \
  --set mysql.external.database=synapse \
  --set mysql.external.username=synapse \
  --set mysql.external.existingSecret=synapse-mysql \
  --set security.jwtSecret="your-secure-jwt-secret"
```

### 场景 3: 启用 Ingress

创建 `values-ingress.yaml`:

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: synapse.example.com
      paths:
        - path: /
          pathType: Prefix
          backend: frontend
        - path: /api
          pathType: Prefix
          backend: backend
  tls:
    - secretName: synapse-tls
      hosts:
        - synapse.example.com

security:
  jwtSecret: "your-secure-jwt-secret-at-least-32-chars"
```

```bash
helm install synapse synapse/synapse \
  -n synapse \
  -f values-ingress.yaml
```

### 场景 4: 高可用部署

使用预配置的 HA 配置：

```bash
helm install synapse synapse/synapse \
  -n synapse \
  -f values-ha.yaml
```

或查看 [values-ha.yaml](./values-ha.yaml) 自定义配置。

### 场景 5: 生产环境部署

```bash
helm install synapse synapse/synapse \
  -n synapse \
  -f values-production.yaml \
  --set mysql.external.host=your-mysql-host \
  --set security.jwtSecret="$(openssl rand -base64 32)"
```

## 🔧 升级

### 升级 Chart

```bash
# 更新仓库
helm repo update

# 查看可用版本
helm search repo synapse --versions

# 升级到最新版本
helm upgrade synapse synapse/synapse \
  -n synapse \
  -f values.yaml

# 查看升级历史
helm history synapse -n synapse
```

### 回滚

```bash
# 查看历史版本
helm history synapse -n synapse

# 回滚到指定版本
helm rollback synapse 1 -n synapse
```

## 🗑️ 卸载

```bash
# 卸载 Chart
helm uninstall synapse -n synapse

# 删除 PVC（注意：会删除所有数据）
kubectl delete pvc -l app.kubernetes.io/instance=synapse -n synapse

# 删除命名空间
kubectl delete namespace synapse
```

## 🔍 故障排查

### 查看 Pod 状态

```bash
kubectl get pods -n synapse
kubectl describe pod -l app.kubernetes.io/instance=synapse -n synapse
```

### 查看日志

```bash
# 后端日志
kubectl logs -f -l app.kubernetes.io/component=backend -n synapse

# 前端日志
kubectl logs -f -l app.kubernetes.io/component=frontend -n synapse

# MySQL 日志
kubectl logs -f -l app.kubernetes.io/component=mysql -n synapse
```

### 查看事件

```bash
kubectl get events -n synapse --sort-by='.lastTimestamp'
```

### 常见问题

#### Pod 一直 Pending

检查存储和资源：

```bash
# 检查 PVC 状态
kubectl get pvc -n synapse

# 检查节点资源
kubectl describe nodes
```

#### 数据库连接失败

检查 MySQL 配置：

```bash
# 查看 MySQL Pod
kubectl get pod -l app.kubernetes.io/component=mysql -n synapse

# 查看 Secret
kubectl get secret -n synapse
kubectl describe secret synapse-mysql -n synapse
```

#### 后端无法启动

检查配置和依赖：

```bash
# 查看后端日志
kubectl logs -l app.kubernetes.io/component=backend -n synapse --tail=100

# 检查 ConfigMap
kubectl describe configmap synapse-config -n synapse

# 检查环境变量
kubectl exec -it deployment/synapse-backend -n synapse -- env | grep -E "DB|JWT"
```

## 🧪 测试

运行 Helm 测试：

```bash
helm test synapse -n synapse
```

## 📊 监控

### Prometheus ServiceMonitor

启用 Prometheus 监控：

```yaml
monitoring:
  serviceMonitor:
    enabled: true
    interval: 30s
```

### Grafana 集成

集成 Grafana 仪表盘：

```yaml
grafana:
  external:
    enabled: true
    url: "https://grafana.example.com"
    apiKey: "your-api-key"
```

## 🔒 安全

### 密钥管理

**推荐做法：**

1. 使用已有的 Secret：

```bash
# 创建 Secret
kubectl create secret generic synapse-secrets \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  -n synapse

# 使用 existing Secret
helm install synapse synapse/synapse \
  -n synapse \
  --set security.existingSecret=synapse-secrets
```

2. 使用外部密钥管理工具（如 Vault、Sealed Secrets）

### RBAC 权限

Chart 会自动创建必要的 RBAC 资源。可以通过 `rbac.rules` 自定义权限。

## 📚 文档

- [官方文档](https://synapse.clay-wangzhi.com/docs)
- [快速开始](https://synapse.clay-wangzhi.com/docs/getting-started/quick-start)
- [配置指南](https://synapse.clay-wangzhi.com/docs/admin-guide/deployment)
- [API 文档](https://synapse.clay-wangzhi.com/docs/api/overview)

## 🤝 贡献

欢迎贡献！请查看 [CONTRIBUTING.md](https://github.com/clay-wangzhi/Synapse/blob/main/CONTRIBUTING.md)

## 📄 许可证

Apache License 2.0 - 查看 [LICENSE](https://github.com/clay-wangzhi/Synapse/blob/main/LICENSE)

## 🙏 致谢

感谢所有贡献者和使用者！

---

**维护者:** Synapse Team
**联系方式:** support@synapse.io
**主页:** https://synapse.clay-wangzhi.com
