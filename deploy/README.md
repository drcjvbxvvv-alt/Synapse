# KubePolaris 部署指南

本目录包含 KubePolaris 的辅助部署配置文件。

> **注意**: `Dockerfile` 和 `docker-compose.yaml` 已移至项目根目录，便于直接使用。

## 📁 目录结构

```
项目根目录/
├── Dockerfile                 # 多阶段构建（前后端合一，单二进制）
├── docker-compose.yaml        # Docker Compose 编排文件
├── .env.example               # 环境变量模板
└── deploy/
    ├── docker/
    │   ├── grafana/           # Grafana 配置
    │   │   ├── dashboards/    # 预置 Dashboard
    │   │   ├── provisioning/  # 自动配置
    │   │   └── secrets/       # API Key 等密钥
    │   └── mysql/             # MySQL 配置（可选）
    │       ├── conf/          # MySQL 配置文件
    │       └── init/          # 初始化 SQL 脚本
    └── helm/                  # Kubernetes Helm Chart
        └── kubepolaris/
```

## 🚀 快速开始

### 最快体验（一条命令）

```bash
docker run --rm -p 8080:8080 registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/kubepolaris:latest

# 访问 http://localhost:8080
# 默认账号: admin / KubePolaris@2026
```

> 使用内置 SQLite，无需外部依赖。生产环境建议使用 Docker Compose + MySQL。

### Docker Compose 部署

```bash
# 1. 克隆项目
git clone https://github.com/clay-wangzhi/KubePolaris.git
cd KubePolaris

# 2. 配置环境变量
cp .env.example .env
vim .env

# 3. 启动所有服务
docker compose up -d

# 4. 查看日志
docker compose logs -f

# 5. 停止服务
docker compose down
```

## 📦 镜像说明

| 镜像 | 用途 | 端口 |
|------|------|------|
| `kubepolaris` | 一体化镜像（前端通过 go:embed 嵌入后端） | 8080 |

## 🔧 环境变量

主要环境变量（在 `.env` 文件中配置）：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `MYSQL_ROOT_PASSWORD` | MySQL root 密码 | - |
| `MYSQL_PASSWORD` | MySQL 应用密码 | - |
| `JWT_SECRET` | JWT 密钥 | - |
| `GRAFANA_ADMIN_PASSWORD` | Grafana 管理员密码 | - |
| `APP_PORT` | 应用对外端口 | `80` |
| `SERVER_MODE` | 运行模式 (debug/release) | `release` |

## 📊 服务访问

- **KubePolaris**: http://localhost (默认端口 80)
- **Grafana**: http://localhost:3000

## 📝 注意事项

1. **生产环境**
   - 建议使用外部数据库
   - 配置 SSL/TLS 证书
   - 使用强密码

2. **Grafana 数据源**
   - 需要配置外部 Prometheus 地址
   - 修改 `deploy/docker/grafana/provisioning/datasources/prometheus.yaml`

3. **Kubernetes 集群访问**
   - 挂载 kubeconfig 或使用 ServiceAccount
