---
sidebar_position: 4
---

# 节点管理

本文档介绍如何在 Synapse 中查看和管理 Kubernetes 集群节点。

## 查看节点

### 节点列表

进入 **节点管理** 页面查看所有节点：

| 信息 | 说明 |
|------|------|
| 节点名称 | 节点主机名 |
| 状态 | Ready/NotReady/Unknown |
| 角色 | Master/Worker |
| 版本 | kubelet 版本 |
| 内部 IP | 节点内网 IP |
| OS | 操作系统 |
| 内核版本 | Linux 内核版本 |
| 容器运行时 | Docker/containerd/CRI-O |
| CPU/内存 | 资源容量和使用率 |
| Pod 数量 | 运行中的 Pod 数 |

### 节点状态

| 状态 | 说明 |
|------|------|
| 🟢 **Ready** | 节点健康，可调度 Pod |
| 🔴 **NotReady** | 节点不健康 |
| ⚪ **Unknown** | 无法获取状态 |
| 🟡 **SchedulingDisabled** | 已禁止调度 |

### 节点详情

点击节点名称进入详情页：

#### 概览
- 基本信息（名称、IP、创建时间）
- 系统信息（OS、架构、内核版本）
- 容器运行时信息
- 资源容量和可分配量

#### 资源使用
- CPU 使用率
- 内存使用率
- 磁盘使用率
- Pod 数量

#### Pod 列表
运行在该节点上的所有 Pod。

#### 标签和注解
节点的所有标签和注解。

#### 污点
节点配置的污点列表。

#### 状态条件
- Ready
- MemoryPressure
- DiskPressure
- PIDPressure
- NetworkUnavailable

#### 事件
节点相关的事件记录。

## 节点操作

### Cordon（禁止调度）

将节点标记为不可调度：

1. 点击节点行的 **禁止调度** 按钮
2. 确认操作

效果：
- 新 Pod 不会调度到该节点
- 现有 Pod 继续运行
- 节点状态显示 `SchedulingDisabled`

使用场景：
- 准备维护节点
- 节点即将下线

等同于：
```bash
kubectl cordon <node-name>
```

### Uncordon（恢复调度）

恢复节点调度：

1. 点击 **恢复调度** 按钮
2. 确认操作

等同于：
```bash
kubectl uncordon <node-name>
```

### Drain（排空节点）

安全地迁移节点上的 Pod：

1. 点击 **排空节点** 按钮
2. 配置排空选项：
   - **忽略 DaemonSet**: 是否忽略 DaemonSet 管理的 Pod
   - **强制删除**: 是否强制删除本地存储的 Pod
   - **超时时间**: 等待 Pod 优雅终止的时间
3. 确认执行

效果：
- 节点被标记为不可调度
- 现有 Pod 被驱逐到其他节点
- DaemonSet Pod 默认保留

使用场景：
- 节点维护
- 节点下线
- 系统升级

等同于：
```bash
kubectl drain <node-name> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --timeout=300s
```

:::warning 注意
排空操作会驱逐节点上的 Pod，请确保有足够的资源在其他节点运行这些 Pod。
:::

### SSH 终端

直接 SSH 到节点：

1. 点击 **SSH** 按钮
2. 选择认证方式（密码/密钥）
3. 在 Web 终端中操作

详见 [终端访问](./terminal-access)。

### 管理标签

添加或删除节点标签：

1. 进入节点详情 → **标签**
2. 点击 **编辑**
3. 添加/修改/删除标签
4. 保存

常用标签示例：
```yaml
node-type: compute
zone: cn-beijing-a
environment: production
team: platform
```

### 管理污点

添加或删除节点污点：

1. 进入节点详情 → **污点**
2. 点击 **编辑**
3. 添加/修改/删除污点
4. 保存

污点格式：
```yaml
key: value:Effect
```

Effect 类型：
- `NoSchedule`: 不调度新 Pod
- `PreferNoSchedule`: 尽量不调度
- `NoExecute`: 驱逐现有 Pod

示例：
```bash
# 添加污点
kubectl taint nodes <node-name> key=value:NoSchedule

# 删除污点
kubectl taint nodes <node-name> key-
```

## 节点监控

### 内置监控

查看基础指标：

- CPU 使用率
- 内存使用率
- 磁盘使用率
- 网络流量
- Pod 数量

### Prometheus 指标

配置 Prometheus 后可查看：

- 详细资源使用趋势
- 节点负载历史
- IO 等待时间
- 网络连接数

### 告警规则

常见节点告警：

| 告警 | 条件 | 严重程度 |
|------|------|---------|
| 节点 NotReady | 状态持续 5 分钟 | Critical |
| CPU 使用率高 | > 80% 持续 15 分钟 | Warning |
| 内存使用率高 | > 85% 持续 15 分钟 | Warning |
| 磁盘使用率高 | > 90% | Warning |
| Pod 数量接近上限 | > 90% 容量 | Warning |

## 问题诊断

### 节点 NotReady

常见原因：

1. **kubelet 问题**
   ```bash
   # 检查 kubelet 状态
   systemctl status kubelet
   journalctl -u kubelet -n 100
   ```

2. **网络问题**
   - 检查节点网络连通性
   - 检查 CNI 插件状态

3. **资源耗尽**
   - 检查磁盘空间
   - 检查内存使用

4. **证书问题**
   - 检查证书是否过期

### DiskPressure

磁盘空间不足：

1. 清理无用镜像
   ```bash
   docker system prune -af
   # 或
   crictl rmi --prune
   ```

2. 清理日志
   ```bash
   journalctl --vacuum-time=3d
   ```

3. 扩展磁盘空间

### MemoryPressure

内存不足：

1. 检查内存泄漏的 Pod
2. 驱逐低优先级 Pod
3. 增加节点内存

### 网络问题

1. 检查网络插件状态
2. 检查 iptables 规则
3. 检查 DNS 解析

## 最佳实践

### 节点规划

- 生产环境至少 3 个 Master 节点
- Worker 节点数量根据负载规划
- 预留足够的资源余量

### 标签策略

使用标签组织节点：

```yaml
# 区域
topology.kubernetes.io/zone: cn-beijing-a

# 节点类型
node-type: compute
node-type: gpu
node-type: memory-optimized

# 环境
environment: production

# 团队
team: platform
```

### 污点使用

合理使用污点：

```yaml
# Master 节点
node-role.kubernetes.io/master:NoSchedule

# GPU 节点
nvidia.com/gpu:NoSchedule

# 专用节点
dedicated=special:NoSchedule
```

### 资源预留

配置系统预留资源：

```yaml
# kubelet 配置
kubeReserved:
  cpu: 200m
  memory: 500Mi
systemReserved:
  cpu: 200m
  memory: 500Mi
evictionHard:
  memory.available: 500Mi
  nodefs.available: 10%
```

## 下一步

- [终端访问](./terminal-access) - SSH 到节点
- [监控告警](./monitoring-alerting) - 配置节点监控

