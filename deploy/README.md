# KubePolaris 部署指南

本目录包含 KubePolaris 的所有部署相关文件。

## 📁 目录结构

```
deploy/
├── docker/                    # Docker 相关配置
│   ├── kubepolaris/          # KubePolaris 镜像构建
│   │   ├── Dockerfile        # 一体化镜像（前后端合一）
│   │   ├── Dockerfile.backend   # 后端镜像
│   │   ├── Dockerfile.frontend  # 前端镜像
│   │   ├── nginx.conf           # 一体化镜像 Nginx 配置
│   │   ├── nginx-frontend.conf  # 前端镜像 Nginx 配置
│   │   └── entrypoint.sh        # 启动脚本
│   ├── mysql/                # MySQL 配置
│   │   ├── conf/            # MySQL 配置文件
│   │   └── init/            # 初始化 SQL 脚本
│   └── grafana/              # Grafana 配置
│       ├── dashboards/       # 预置 Dashboard
│       ├── provisioning/     # 自动配置
│       └── secrets/          # API Key 等密钥
├── docker-compose/           # Docker Compose 文件
│   ├── docker-compose.yml    # 开发环境
│   └── docker-compose.prod.yml  # 生产环境
├── scripts/                  # 部署脚本
│   ├── install.sh           # 一键安装
│   ├── upgrade.sh           # 升级脚本
│   └── uninstall.sh         # 卸载脚本
└── yaml/                     # Kubernetes YAML 文件（未来）
```

## 🚀 快速开始

### 方式一：使用安装脚本（推荐）

```bash
# 一键安装
./deploy/scripts/install.sh

# 升级
./deploy/scripts/upgrade.sh

# 卸载
./deploy/scripts/uninstall.sh
```

### 方式二：使用 Docker Compose

```bash
# 进入 docker-compose 目录
cd deploy/docker-compose

# 复制并编辑环境变量
cp ../../.env.example .env
vim .env

# 启动开发环境
docker-compose up -d

# 启动生产环境
docker-compose -f docker-compose.prod.yml up -d
```

### 方式三：使用 Makefile

```bash
# 在项目根目录执行
make install    # 安装
make docker-up  # 启动服务
make docker-down # 停止服务
```

## 📦 镜像说明

| 镜像 | 用途 | 端口 |
|------|------|------|
| `kubepolaris/kubepolaris` | 一体化镜像（前后端合一） | 80, 8080 |
| `kubepolaris/backend` | 后端 API 服务 | 8080 |
| `kubepolaris/frontend` | 前端静态服务 | 80 |

## 🔧 环境变量

主要环境变量（在 `.env` 文件中配置）：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `MYSQL_ROOT_PASSWORD` | MySQL root 密码 | - |
| `MYSQL_USER` | MySQL 用户名 | kubepolaris |
| `MYSQL_PASSWORD` | MySQL 密码 | - |
| `JWT_SECRET` | JWT 密钥 | - |
| `GRAFANA_ADMIN_PASSWORD` | Grafana 管理员密码 | - |

## 📊 服务访问

- **KubePolaris**: http://localhost:80
- **API**: http://localhost:8080
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

