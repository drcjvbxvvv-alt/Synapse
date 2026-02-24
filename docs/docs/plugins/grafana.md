---
sidebar_position: 3
---

# Grafana 集成

KubePolaris 深度集成 Grafana，支持在统一界面中嵌入监控面板、自动同步 Dashboard 和数据源。

## 两种部署模式

KubePolaris 支持两种 Grafana 部署方式：

| 模式 | 适用场景 | 说明 |
|------|---------|------|
| **内置 Grafana** | Docker Compose 快速部署 | 随 KubePolaris 一起启动，自动初始化 Service Account 和 API Token |
| **外置 Grafana** | 已有 Grafana 实例、Kubernetes 部署 | 连接已有的独立 Grafana 服务，手动配置连接信息 |

:::tip
无论哪种模式，Grafana 连接配置（URL 和 API Key）都在 KubePolaris **系统设置** 页面管理，存储在数据库中。
:::

## 内置 Grafana（Docker Compose）

使用 Docker Compose 部署时，Grafana 会作为内置服务自动启动：

- `grafana` 服务自动配置好嵌入和匿名访问
- `grafana-init` 初始化容器自动创建 Service Account 和 API Token
- API Token 写入 `/secrets/grafana_api_key` 文件供应用读取

部署完成后，在 KubePolaris **系统设置** → **Grafana 配置** 中填写：

| 配置项 | 值 |
|--------|------|
| Grafana 地址 | `http://grafana:3000/grafana/` |
| API Key | 从 `deploy/docker/grafana/secrets/grafana_api_key` 文件中获取 |

```bash
# 查看自动生成的 API Token
cat deploy/docker/grafana/secrets/grafana_api_key
```

## 外置 Grafana

如果你已有独立部署的 Grafana 实例（如通过 Helm 安装在 Kubernetes 集群中，或独立的虚拟机部署），可以将其与 KubePolaris 对接。

### 前置要求

- Grafana 9.0+ 已部署并可访问
- KubePolaris 所在网络能够访问 Grafana 服务地址
- 拥有 Grafana 管理员账号（用于创建 Service Account）

### 步骤一：配置 Grafana 允许嵌入

KubePolaris 通过 iframe 嵌入 Grafana 面板，需要在 Grafana 侧开启以下配置。

#### 方式 A：通过 grafana.ini 配置文件

```ini
[security]
allow_embedding = true

[auth.anonymous]
enabled = true
org_name = Main Org.
org_role = Viewer
```

#### 方式 B：通过环境变量（推荐容器化部署使用）

```yaml
environment:
  GF_SECURITY_ALLOW_EMBEDDING: "true"
  GF_AUTH_ANONYMOUS_ENABLED: "true"
  GF_AUTH_ANONYMOUS_ORG_ROLE: Viewer
  GF_AUTH_ANONYMOUS_ORG_NAME: "Main Org."
```

#### 方式 C：Helm Chart values.yaml

```yaml
grafana:
  grafana.ini:
    security:
      allow_embedding: true
    auth.anonymous:
      enabled: true
      org_name: Main Org.
      org_role: Viewer
```

:::warning
`allow_embedding = true` 和匿名访问是嵌入面板的必要条件。如果不配置匿名访问，嵌入的面板将显示 Grafana 登录页。
:::

### 步骤二：创建 Service Account Token

KubePolaris 通过 Grafana API 自动同步数据源和 Dashboard，需要一个具有 **Admin** 角色的 Service Account Token。

#### 通过 Grafana UI 创建

1. 登录 Grafana 管理后台
2. 进入 **Administration** → **Service Accounts**
3. 点击 **Add service account**
4. 填写信息：
   - **Display name**：`kubepolaris`
   - **Role**：`Admin`（需要创建数据源和导入 Dashboard 的权限）
5. 点击 **Create**
6. 在创建的 Service Account 页面，点击 **Add service account token**
7. 填写 Token 名称（如 `kubepolaris-token`），过期时间可设为 **No expiration**
8. 点击 **Generate token**
9. **立即复制并保存 Token**（关闭对话框后将无法再次查看）

#### 通过 Grafana API 创建

```bash
# 设置 Grafana 地址和管理员凭据
GRAFANA_URL="http://your-grafana:3000"
GRAFANA_USER="admin"
GRAFANA_PASSWORD="your-admin-password"

# 1. 创建 Service Account
SA_RESULT=$(curl -s -X POST \
  -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
  -H "Content-Type: application/json" \
  -d '{"name":"kubepolaris","role":"Admin","isDisabled":false}' \
  "${GRAFANA_URL}/api/serviceaccounts")

SA_ID=$(echo "$SA_RESULT" | grep -o '"id":[0-9]*' | cut -d: -f2)
echo "Service Account ID: $SA_ID"

# 2. 生成 API Token
TOKEN_RESULT=$(curl -s -X POST \
  -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
  -H "Content-Type: application/json" \
  -d '{"name":"kubepolaris-token","secondsToLive":0}' \
  "${GRAFANA_URL}/api/serviceaccounts/${SA_ID}/tokens")

API_KEY=$(echo "$TOKEN_RESULT" | grep -o '"key":"[^"]*"' | cut -d'"' -f4)
echo "API Token: $API_KEY"
```

:::info
`secondsToLive: 0` 表示 Token 永不过期。如需设置过期时间，可改为对应的秒数（如 `86400` 为一天）。
:::

### 步骤三：在 KubePolaris 中配置连接

1. 登录 KubePolaris，进入 **系统设置** → **Grafana 配置**
2. 填写连接信息：

| 配置项 | 说明 | 示例 |
|--------|------|------|
| Grafana 地址 | Grafana 服务的完整 URL | `http://grafana.monitoring.svc:3000` |
| API Key | 步骤二中创建的 Service Account Token | `glsa_xxxxxxxxxxxxxxx` |

3. 点击 **测试连接**，确认连接成功
4. 点击 **保存配置**

:::tip Grafana 地址填写说明
- **Docker 网络内部访问**：`http://grafana:3000`
- **Kubernetes 集群内部访问**：`http://grafana.monitoring.svc.cluster.local:3000`
- **外部访问（通过 Ingress）**：`https://grafana.example.com`
- **带子路径的部署**：`http://grafana:3000/grafana/`

注意：填写的地址需要是 **KubePolaris 后端**能访问到的地址，而非浏览器端地址。
:::

### 步骤四：同步 Dashboard 和数据源

配置连接后，可在 Grafana 配置页面执行以下同步操作：

#### 同步 Dashboard

KubePolaris 内置了预配置的 Dashboard JSON（通过 Go embed 嵌入二进制）：

| Dashboard | 用途 |
|-----------|------|
| K8s Cluster Overview | 集群资源总览 |
| K8s Pod Detail | Pod 详细监控 |
| K8s Workload Detail | 工作负载监控 |

点击 **同步 Dashboard** 按钮，KubePolaris 会自动：
1. 在 Grafana 中创建 `KubePolaris` 文件夹
2. 将所有内置 Dashboard 导入到该文件夹（幂等操作，可重复执行）

#### 同步数据源

KubePolaris 会根据已配置 Prometheus 监控的集群，自动在 Grafana 中创建对应的 Prometheus 数据源：

- 数据源名称格式：`Prometheus-{集群名}`
- 数据源 UID 格式：`prometheus-{集群名小写}`

点击 **同步数据源** 按钮，会为所有已配置 Prometheus 端点的集群创建或更新数据源。

:::info
同步数据源的前提是已在集群管理中为对应集群配置了 Prometheus 地址。进入 **集群管理** → **编辑集群** → **监控配置**，填写 Prometheus 端点地址。
:::

## 常见网络拓扑

### 场景一：全部在 Docker Compose 中

```
浏览器 → KubePolaris(:8080) → Grafana(http://grafana:3000)
                              → Prometheus(http://prometheus:9090)
```

### 场景二：KubePolaris 在 Docker，Grafana 在 Kubernetes

```
浏览器 → KubePolaris(:8080) → Grafana(http://<NodeIP>:30300)
                              → Prometheus(http://<NodeIP>:30900)
```

需要确保 KubePolaris 容器网络能访问到 Kubernetes 集群的 NodePort 或 Ingress。

### 场景三：全部在 Kubernetes 中

```
浏览器 → Ingress → KubePolaris → Grafana(http://grafana.monitoring.svc:3000)
                                → Prometheus(http://prometheus.monitoring.svc:9090)
```

## 故障排查

### 测试连接失败

1. 检查 Grafana 地址是否正确（注意是后端可达的地址）
2. 检查 KubePolaris 到 Grafana 的网络连通性
3. 检查 API Key 是否有效（未过期、角色为 Admin）
4. 查看 KubePolaris 后端日志中的错误信息

```bash
# Docker Compose 查看日志
docker compose logs kubepolaris | grep -i grafana
```

### 嵌入面板不显示 / 显示登录页

1. 确认 Grafana 已启用 `allow_embedding = true`
2. 确认 Grafana 已启用匿名访问 `auth.anonymous.enabled = true`
3. 检查浏览器控制台是否有跨域错误（CORS / X-Frame-Options）
4. 如果使用反向代理（如 Nginx），确保未设置 `X-Frame-Options: DENY`

### Dashboard 同步失败

1. 确认 API Key 角色为 **Admin**（需要创建文件夹和导入 Dashboard 的权限）
2. 检查 Grafana API 是否正常：`curl -H "Authorization: Bearer <token>" http://<grafana>/api/health`
3. 查看 KubePolaris 日志中的具体错误

### 数据源同步后面板无数据

1. 确认集群的 Prometheus 地址填写正确
2. 确认 **Grafana 能访问到** Prometheus 地址（数据源的 Access 模式为 `proxy`，由 Grafana 服务端请求 Prometheus）
3. 在 Grafana 中手动测试数据源连通性：进入 **Data Sources** → 选择对应数据源 → **Test**
