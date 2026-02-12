---
sidebar_position: 1
---

# Docker 部署

使用 Docker 和 Docker Compose 部署 KubePolaris，适合快速体验和开发测试环境。

## 前置要求

- Docker 20.10+
- Docker Compose 2.0+
- 至少 4GB 可用内存
- 至少 20GB 可用磁盘空间

## 一条命令快速体验

```bash
docker run --rm -p 8080:8080 registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/kubepolaris:latest
```

访问 `http://localhost:8080`，使用默认账号 `admin / KubePolaris@2026` 登录。

:::tip
以上方式使用内置 SQLite，适合快速体验。生产环境建议使用下方 Docker Compose 部署，搭配 MySQL + Grafana。
:::

## Docker Compose 部署

### 1. 获取代码

```bash
git clone https://github.com/clay-wangzhi/KubePolaris.git
```

### 2. 启动服务

```bash
cd KubePolaris

# 配置环境变量
cp .env.example .env
vim .env  # 设置密码

# 启动服务
docker compose up -d
```

### 3. 验证部署

```bash
# 查看服务状态
docker ps

# 健康检查
curl http://localhost:8080/healthz
```

访问 `http://<服务器IP>` 开始使用。

## Docker Compose 配置

### 默认配置

```yaml title="docker-compose.yaml"
services:
  mysql:
    image: registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/mysql:8.0
    container_name: kubepolaris-mysql
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      MYSQL_DATABASE: kubepolaris
      MYSQL_USER: kubepolaris
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
    # ...

  kubepolaris:
    build:
      context: .
      dockerfile: Dockerfile
    image: registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/kubepolaris:${VERSION:-latest}
    container_name: kubepolaris
    restart: unless-stopped
    depends_on:
      mysql:
        condition: service_healthy
      grafana:
        condition: service_healthy
    ports:
      - "${APP_PORT:-80}:8080"
    # ...

  grafana:
    image: registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/grafana:10.2.0
    # ...

  grafana-init:
    image: registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/curl:8.16.0
    # 自动生成 Grafana API Key
    # ...
```

完整配置请参考项目根目录的 [docker-compose.yaml](https://github.com/clay-wangzhi/KubePolaris/blob/main/docker-compose.yaml)。


## 数据备份

### 备份数据库

```bash
# 备份
docker exec kubepolaris-mysql mysqldump -u root -p kubepolaris > backup_$(date +%Y%m%d).sql

# 恢复
cat backup_20260107.sql | docker exec -i kubepolaris-mysql mysql -u root -p kubepolaris
```

### 备份数据卷

```bash
# 备份
docker run --rm -v kubepolaris_mysql_data:/data -v $(pwd):/backup alpine tar czf /backup/mysql_data.tar.gz /data

# 恢复
docker run --rm -v kubepolaris_mysql_data:/data -v $(pwd):/backup alpine tar xzf /backup/mysql_data.tar.gz -C /
```

## 升级

### 升级到新版本

```bash
# 拉取最新代码
git pull origin main

# 重新构建并启动
docker compose up -d --build

# 查看日志确认升级成功
docker compose logs -f kubepolaris
```

### 回滚

```bash
# 使用指定版本
export VERSION=v1.0.0
docker compose up -d
```

## 常见问题

### 端口被占用

```bash
# 查看端口占用
lsof -i :8080

# 修改 .env 中的 APP_PORT 配置
APP_PORT=9090  # 使用 9090 端口
```

### 容器无法访问网络

```bash
# 检查 Docker 网络
docker network ls
docker network inspect kubepolaris_default

# 重建网络
docker compose down
docker network prune
docker compose up -d
```

### 数据库连接失败

```bash
# 检查 MySQL 容器状态
docker logs kubepolaris-mysql

# 测试连接
docker exec -it kubepolaris-mysql mysql -u root -p

# 检查环境变量
docker exec kubepolaris env | grep DB_
```

## 下一步

- [Kubernetes 部署](./kubernetes) - 生产环境推荐
- [配置说明](../getting-started/configuration) - 详细配置项

