# KubePolaris 部署指南

## 📦 部署方式

KubePolaris 支持多种部署方式：

1. **Docker Compose 部署**（推荐用于开发/测试）
2. **Kubernetes Helm 部署**（推荐用于生产环境）
3. **二进制部署**（适用于特殊场景）

---

## ☸️ Kubernetes Helm 部署（推荐生产环境）

### 方式一：通过 Helm 仓库安装（推荐）

```bash
# 1. 添加 Helm 仓库
helm repo add kubepolaris https://clay-wangzhi.github.io/KubePolaris
helm repo update

# 2. 搜索可用版本
helm search repo kubepolaris

# 3. 安装（使用默认配置）
helm install kubepolaris kubepolaris/kubepolaris \
  -n kubepolaris --create-namespace

# 4. 或者自定义配置安装
helm install kubepolaris kubepolaris/kubepolaris \
  -n kubepolaris --create-namespace \
  --set mysql.auth.rootPassword=your-root-password \
  --set mysql.auth.password=your-password \
  --set backend.config.jwt.secret=your-jwt-secret

# 5. 查看安装状态
helm status kubepolaris -n kubepolaris
kubectl get pods -n kubepolaris
```

### 方式二：从源码安装

```bash
# 1. 克隆项目
git clone https://github.com/clay-wangzhi/KubePolaris.git
cd KubePolaris

# 2. 安装
helm install kubepolaris ./deploy/helm/kubepolaris \
  -n kubepolaris --create-namespace \
  -f ./deploy/helm/kubepolaris/values.yaml
```

### Helm 配置说明

详细配置请参考 [Helm Chart README](./helm/kubepolaris/README.md)

常用配置项：

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `mysql.auth.rootPassword` | MySQL root 密码 | `kubepolaris-root` |
| `mysql.auth.password` | 应用数据库密码 | `kubepolaris123` |
| `backend.config.jwt.secret` | JWT 密钥 | 随机生成 |
| `ingress.enabled` | 是否启用 Ingress | `true` |
| `ingress.hosts[0].host` | 域名 | `kubepolaris.local` |
| `grafana.enabled` | 是否启用内置 Grafana | `true` |

### 升级和卸载

```bash
# 升级
helm repo update
helm upgrade kubepolaris kubepolaris/kubepolaris -n kubepolaris

# 卸载
helm uninstall kubepolaris -n kubepolaris
```

---

## 🐳 Docker 部署（开发/测试）

### 一条命令快速体验

```bash
docker run --rm -p 8080:8080 registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/kubepolaris:latest
```

访问 `http://localhost:8080`，默认账号 `admin / KubePolaris@2026`。

> 使用内置 SQLite，无需外部依赖。生产环境建议使用下方 Docker Compose + MySQL 部署。

### Docker Compose 部署

#### 前置要求

- Docker 20.10+
- Docker Compose V2 (docker compose plugin)
- 至少 4GB 可用内存
- 至少 10GB 可用磁盘空间

### 快速开始

```bash
# 1. 克隆项目
git clone https://github.com/clay-wangzhi/KubePolaris.git
cd KubePolaris

# 2. 配置环境变量
cp .env.example .env
vim .env  # 修改密码等配置

# 3. 创建 Grafana secrets 目录
mkdir -p deploy/docker/grafana/secrets

# 4. 启动所有服务
docker compose up -d

# 5. 查看状态
docker compose ps
```

### 访问应用

启动完成后，访问：

- **KubePolaris**: http://localhost
  - 默认账号: `admin`
  - 默认密码: `KubePolaris@2026`

- **Grafana**: http://localhost:3000
  - 默认账号: `admin`
  - 默认密码: 查看 `.env` 文件中的 `GRAFANA_ADMIN_PASSWORD`

---

## 📋 配置说明

### 环境变量配置 (.env)

| 变量名 | 说明 | 默认值 | 必填 |
|--------|------|--------|------|
| `MYSQL_ROOT_PASSWORD` | MySQL root 密码 | - | ✅ |
| `MYSQL_PASSWORD` | 应用数据库密码 | - | ✅ |
| `JWT_SECRET` | JWT 签名密钥 | - | ✅ |
| `GRAFANA_ADMIN_PASSWORD` | Grafana 管理员密码 | - | ✅ |
| `MYSQL_PORT` | MySQL 端口 | `3306` | ❌ |
| `APP_PORT` | 应用对外端口 | `80` | ❌ |
| `GRAFANA_PORT` | Grafana 端口 | `3000` | ❌ |
| `SERVER_MODE` | 运行模式 (debug/release) | `release` | ❌ |
| `LOG_LEVEL` | 日志级别 | `info` | ❌ |
| `VERSION` | 镜像版本 | `latest` | ❌ |

---

## 🔒 安全最佳实践

### 1. 密码安全

**生成强随机密码**:
```bash
# MySQL 密码（16 字符）
openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16

# JWT Secret（32 字符）
openssl rand -base64 32

# Grafana 密码（12 字符）
openssl rand -base64 12 | tr -dc 'a-zA-Z0-9' | head -c 12
```

### 2. 文件权限

```bash
# .env 文件只允许所有者读写
chmod 600 .env

# secrets 目录权限
chmod 700 deploy/docker/grafana/secrets
```

### 3. 生产环境建议

- ✅ 使用强随机密码（16+ 字符）
- ✅ 定期轮换密码和密钥
- ✅ 启用 HTTPS/TLS
- ✅ 配置防火墙规则
- ✅ 启用审计日志
- ✅ 定期备份数据
- ✅ 使用 Secrets 管理工具（如 Vault）

---

## 🛠️ 常用操作

### 查看日志

```bash
# 查看所有服务日志
docker compose logs -f

# 查看特定服务日志
docker compose logs -f kubepolaris
docker compose logs -f mysql
docker compose logs -f grafana
```

### 重启服务

```bash
# 重启所有服务
docker compose restart

# 重启特定服务
docker compose restart kubepolaris
```

### 停止服务

```bash
# 停止服务（保留数据）
docker compose stop

# 停止并删除容器（保留数据卷）
docker compose down

# 停止并删除所有内容（包括数据）
docker compose down -v
```

### 更新服务

```bash
# 拉取最新代码
git pull origin main

# 重新构建并启动
docker compose up -d --build

# 查看更新状态
docker compose ps
```

### 数据备份

```bash
# 备份 MySQL 数据
docker compose exec mysql mysqldump -u root -p kubepolaris > backup.sql

# 备份 Grafana 数据
docker compose exec grafana tar czf - /var/lib/grafana > grafana-backup.tar.gz
```

### 数据恢复

```bash
# 恢复 MySQL 数据
docker compose exec -T mysql mysql -u root -p kubepolaris < backup.sql

# 恢复 Grafana 数据
docker compose exec -T grafana tar xzf - -C / < grafana-backup.tar.gz
docker compose restart grafana
```

---

## 🐛 故障排查

### 服务无法启动

**检查 Docker 状态**:
```bash
docker info
docker compose ps
```

**查看错误日志**:
```bash
docker compose logs kubepolaris
docker compose logs mysql
```

**常见问题**:
1. **端口冲突**: 修改 `.env` 中的端口配置
2. **内存不足**: 确保至少 4GB 可用内存
3. **磁盘空间不足**: 清理 Docker 缓存 `docker system prune -a`

### MySQL 连接失败

```bash
# 检查 MySQL 状态
docker compose exec mysql mysqladmin ping -h localhost

# 重置 MySQL
docker compose down
docker volume rm kubepolaris-mysql-data
docker compose up -d mysql
```

### Grafana API Key 问题

```bash
# 检查 API Key 文件
ls -la deploy/docker/grafana/secrets/grafana_api_key

# 重新生成 API Key
docker compose up -d grafana-init
docker compose logs grafana-init
```

---

## 📊 监控和维护

### 健康检查

```bash
# 检查所有服务健康状态
docker compose ps

# 手动测试健康检查
curl http://localhost/healthz          # KubePolaris
curl http://localhost:3000/api/health  # Grafana
```

### 资源监控

```bash
# 查看容器资源使用
docker stats

# 查看磁盘使用
docker system df
```

---

## 🔄 升级指南

```bash
# 1. 备份数据
docker compose exec mysql mysqldump -u root -p kubepolaris > backup_$(date +%Y%m%d).sql

# 2. 拉取最新代码
git pull origin main

# 3. 重新构建并启动
docker compose up -d --build

# 4. 验证服务
docker compose ps
curl http://localhost/healthz
```

---

## 📚 相关文档

- [环境变量配置模板](../.env.example)
- [Helm Chart 文档](./helm/kubepolaris/README.md)

---

**最后更新**: 2026-02-12
**文档版本**: v2.0.0
