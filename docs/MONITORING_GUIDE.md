# KubePolaris 监控功能使用指南

## 概述

KubePolaris 现在支持 Prometheus 监控集成，可以为每个集群配置独立的监控数据源，支持直接连接 Prometheus 或使用统一数据源（如 VictoriaMetrics）。

## 功能特性

- ✅ **多数据源支持**：支持 Prometheus、VictoriaMetrics 等监控系统
- ✅ **灵活配置**：每个集群可独立配置监控数据源
- ✅ **多种认证**：支持 Basic Auth、Bearer Token、mTLS 认证
- ✅ **统一标签**：支持通过标签区分不同集群的监控数据
- ✅ **实时监控**：集群、节点、Pod 级别的实时监控图表
- ✅ **配置管理**：图形化界面管理监控配置

## 监控配置

### 1. 配置类型

#### 禁用监控
```json
{
  "type": "disabled"
}
```

#### Prometheus 直接连接
```json
{
  "type": "prometheus",
  "endpoint": "http://prometheus-server:9090",
  "auth": {
    "type": "basic",
    "username": "admin",
    "password": "admin"
  }
}
```

#### VictoriaMetrics 统一数据源
```json
{
  "type": "victoriametrics",
  "endpoint": "http://victoriametrics:8428",
  "auth": {
    "type": "bearer",
    "token": "your-token"
  },
  "labels": {
    "cluster": "cluster-name",
    "environment": "prod"
  }
}
```

### 2. 认证方式

#### Basic Auth
```json
{
  "type": "basic",
  "username": "admin",
  "password": "password"
}
```

#### Bearer Token
```json
{
  "type": "bearer",
  "token": "your-bearer-token"
}
```

#### mTLS（暂未实现）
```json
{
  "type": "mtls",
  "certFile": "/path/to/cert.pem",
  "keyFile": "/path/to/key.pem",
  "caFile": "/path/to/ca.pem"
}
```

### 3. 标签配置

标签用于在统一数据源中区分不同集群的监控数据：

```json
{
  "labels": {
    "cluster": "cluster-name",      // 必需：集群标识
    "environment": "prod",          // 可选：环境标识
    "region": "us-east-1",          // 可选：地域标识
    "team": "platform"              // 可选：团队标识
  }
}
```

## API 接口

### 监控配置管理

#### 获取监控配置
```http
GET /api/v1/clusters/{clusterId}/monitoring/config
```

#### 更新监控配置
```http
PUT /api/v1/clusters/{clusterId}/monitoring/config
Content-Type: application/json

{
  "type": "prometheus",
  "endpoint": "http://prometheus:9090",
  "auth": {
    "type": "basic",
    "username": "admin",
    "password": "admin"
  }
}
```

#### 测试监控连接
```http
POST /api/v1/clusters/{clusterId}/monitoring/test-connection
Content-Type: application/json

{
  "type": "prometheus",
  "endpoint": "http://prometheus:9090",
  "auth": {
    "type": "basic",
    "username": "admin",
    "password": "admin"
  }
}
```

### 监控数据查询

#### 集群监控指标
```http
GET /api/v1/clusters/{clusterId}/monitoring/metrics?range=1h&step=1m&clusterName=cluster-name
```

#### 节点监控指标
```http
GET /api/v1/clusters/{clusterId}/nodes/{nodeName}/metrics?range=1h&step=1m&clusterName=cluster-name
```

#### Pod 监控指标
```http
GET /api/v1/clusters/{clusterId}/pods/{namespace}/{podName}/metrics?range=1h&step=1m&clusterName=cluster-name
```

#### 获取监控模板
```http
GET /api/v1/monitoring/templates
```

## 监控指标

### 集群级别指标

- **CPU 使用率**：`sum(rate(container_cpu_usage_seconds_total[5m])) / sum(machine_cpu_cores)`
- **内存使用率**：`sum(container_memory_working_set_bytes) / sum(machine_memory_bytes)`
- **网络流量**：入站/出站网络流量统计
- **存储使用率**：存储空间使用情况
- **Pod 统计**：总 Pod 数、运行中、等待中、失败的 Pod 数量

### 节点级别指标

- **CPU 使用率**：`rate(node_cpu_seconds_total{mode!="idle"}[5m])`
- **内存使用率**：`(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes))`
- **网络流量**：节点网络入站/出站流量
- **存储使用率**：`(1 - (node_filesystem_avail_bytes / node_filesystem_size_bytes))`

### Pod 级别指标

- **CPU 使用率**：`rate(container_cpu_usage_seconds_total[5m])`
- **内存使用率**：`container_memory_working_set_bytes`
- **网络流量**：Pod 网络入站/出站流量

## 使用步骤

### 1. 配置监控数据源

1. 进入集群详情页面
2. 点击"监控配置"标签页
3. 选择监控类型（Prometheus/VictoriaMetrics/禁用）
4. 配置监控端点和认证信息
5. 设置集群标签（统一数据源需要）
6. 点击"测试连接"验证配置
7. 点击"保存配置"保存设置

### 2. 查看监控图表

1. 进入集群详情页面
2. 点击"监控概览"标签页
3. 选择时间范围和步长
4. 查看 CPU、内存、网络、存储等指标图表

### 3. 节点监控

1. 进入节点详情页面
2. 点击"监控概览"标签页
3. 查看节点级别的监控指标

### 4. Pod 监控

1. 进入 Pod 详情页面
2. 点击"监控"标签页
3. 查看 Pod 级别的监控指标

## 部署建议

### Prometheus 部署

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
    - job_name: 'kubernetes-nodes'
      kubernetes_sd_configs:
      - role: node
    - job_name: 'kubernetes-pods'
      kubernetes_sd_configs:
      - role: pod
```

### VictoriaMetrics 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: victoriametrics
spec:
  replicas: 1
  selector:
    matchLabels:
      app: victoriametrics
  template:
    metadata:
      labels:
        app: victoriametrics
    spec:
      containers:
      - name: victoriametrics
        image: victoriametrics/victoria-metrics:latest
        ports:
        - containerPort: 8428
        args:
        - -storageDataPath=/victoria-metrics-data
        - -retentionPeriod=1y
```

## 故障排除

### 常见问题

1. **连接测试失败**
   - 检查监控端点 URL 是否正确
   - 验证认证信息是否正确
   - 确认网络连通性

2. **监控数据为空**
   - 检查 Prometheus 是否正在收集指标
   - 验证标签配置是否正确
   - 确认时间范围设置

3. **图表不显示**
   - 检查浏览器控制台错误
   - 验证 API 响应格式
   - 确认数据源有数据

### 调试工具

使用测试脚本验证配置：

```bash
go run cmd/test_monitoring.go
```

## 最佳实践

1. **标签命名**：使用一致的标签命名规范
2. **认证安全**：使用强密码和定期轮换 Token
3. **网络隔离**：监控数据源应部署在安全网络环境中
4. **数据保留**：合理设置监控数据保留期
5. **告警配置**：结合 Prometheus AlertManager 设置告警规则

## 更新日志

- **v1.0.0**：初始版本，支持 Prometheus 和 VictoriaMetrics 监控集成
- 支持集群、节点、Pod 级别的监控图表
- 支持多种认证方式和标签配置
- 提供图形化配置管理界面
