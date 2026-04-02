---
sidebar_position: 1
---

# 快速开始

本指南将帮助你在 5 分钟内快速体验 Synapse 的核心功能。

## 前置要求

- Docker 20.10+ 和 Docker Compose 2.0+
- 至少一个可用的 Kubernetes 集群（用于导入管理）
- 8GB+ 可用内存

## 第一步：启动 Synapse

### 最快体验：一条命令启动

```bash
docker run --rm -p 8080:8080 registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/synapse:latest
```

访问 `http://localhost:8080` 即可使用。

:::tip 说明
以上方式使用内置 SQLite，无需任何外部依赖，适合快速体验。生产环境建议使用下方 Docker Compose 方式部署。
:::

### 推荐方式：Docker Compose

```bash
# 克隆项目
git clone https://github.com/clay-wangzhi/Synapse.git
cd Synapse

# 配置环境变量
cp .env.example .env
vim .env  # 设置密码

# 启动服务
docker compose up -d
```

等待约 1-2 分钟，所有服务启动完成后，访问：

- **Web 界面**: `http://<服务器IP>`

## 第二步：登录系统

使用默认管理员账号登录：

| 项目 | 值 |
|------|-----|
| 用户名 | `admin` |
| 密码 | `Synapse@2026` |

:::warning 安全提示
首次登录后请立即修改默认密码！
:::

## 第三步：导入集群

1. 点击左侧菜单 **集群管理** → **导入集群**
2. 填写集群信息：
   - **集群名称**: 为集群起一个易识别的名称
   - **API Server**: Kubernetes API 服务器地址，如 `https://192.168.1.100:6443`
   - **认证方式**: 选择 `kubeconfig` 或 `Token`
3. 上传 kubeconfig 文件或填写 Token
4. 点击 **测试连接** 验证配置
5. 确认无误后点击 **保存**

```yaml title="示例: kubeconfig 文件"
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: <base64-encoded-ca-cert>
    server: https://192.168.1.100:6443
  name: my-cluster
contexts:
- context:
    cluster: my-cluster
    user: admin
  name: my-cluster
current-context: my-cluster
users:
- name: admin
  user:
    token: <your-token>
```

## 第四步：浏览集群资源

成功导入集群后，你可以：

### 查看集群总览

在 **总览** 页面查看：
- 集群资源使用情况（CPU/内存/存储）
- 节点状态分布
- 工作负载概览
- 近期事件

### 管理工作负载

在 **工作负载** 页面：
- 查看 Deployment、StatefulSet、DaemonSet 等
- 执行扩缩容操作
- 查看 Pod 状态和日志
- 编辑 YAML 配置

### 使用 Web 终端

1. 进入任意 Pod 详情页
2. 点击 **终端** 按钮
3. 选择容器（多容器 Pod）
4. 开始在浏览器中操作容器

## 第五步：配置监控告警（可选）

Synapse 支持集成 Prometheus 和 Altermanager

1. 进入 **集群** → **配置中心**
2. 填写 Prometheus 地址，如 `http://prometheus:9090`
3. 填写 Alertmanager 地址，如 `http://alertmanager:9093`
4. 保存配置

配置完成后，你可以在各个资源详情页查看监控图表。

## 下一步

恭喜你完成了快速开始！接下来你可以：

- 📖 阅读 [详细安装指南](./installation) 了解生产环境部署
- 🔧 查看 [配置说明](./configuration) 了解所有配置项
- 📊 学习 [用户指南](../user-guide/cluster-management) 掌握所有功能

## 常见问题

### Docker Compose 启动失败

检查端口是否被占用：

```bash
# 检查 8080 端口
lsof -i :8080

# 如果被占用，修改 docker-compose.yml 中的端口映射
```

### 无法连接到集群

1. 确认 API Server 地址可从 Synapse 容器访问
2. 检查防火墙设置
3. 验证 kubeconfig/Token 是否有效

```bash
# 在容器内测试连接
docker exec -it synapse-backend curl -k https://your-api-server:6443/healthz
```

### 登录后页面空白

清除浏览器缓存后重试：

```bash
# 或者使用无痕模式访问
```

如果问题持续，请查看 [故障排查指南](../admin-guide/troubleshooting) 或在 [GitHub Issues](https://github.com/clay-wangzhi/Synapse/issues) 提问。

