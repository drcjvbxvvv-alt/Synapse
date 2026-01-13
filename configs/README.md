# 配置文件说明

## 📁 文件结构

```
configs/
├── config.yaml          # 实际配置文件（不提交到 Git）
├── config.yaml.example  # 配置模板文件（提交到 Git）
└── README.md           # 本说明文件
```

## 🔧 使用方法

### 方式 1: 使用自动安装脚本（推荐）

运行安装脚本会自动生成配置文件：

```bash
cd deploy/scripts
./install.sh
```

脚本会自动：
- 生成随机密码
- 创建 `config.yaml` 文件
- 创建 `.env` 文件
- 设置正确的文件权限

### 方式 2: 手动配置

#### Docker Compose 部署

```bash
# 1. 复制模板文件
cp config.yaml.example config.yaml

# 2. 编辑配置文件（主要修改密码）
vim config.yaml

# 3. 设置文件权限
chmod 600 config.yaml
```

#### 本地开发

```bash
# 1. 复制模板文件
cp config.yaml.example config.yaml

# 2. 修改数据库连接信息
vim config.yaml
# 将 database.host 改为 localhost
# 将 database.password 改为你的 MySQL 密码

# 3. 修改 Grafana API Key 文件路径
# 将 grafana.api_key_file 改为相对路径:
#   api_key_file: "./deploy/docker/grafana/secrets/grafana_api_key"
```

## ⚙️ 配置项说明

### 服务器配置 (server)

| 配置项 | 说明 | 默认值 | 环境变量 |
|--------|------|--------|----------|
| `port` | 服务监听端口 | `8080` | `SERVER_PORT` |
| `mode` | 运行模式 | `debug` | `SERVER_MODE` |

**mode 取值:**
- `debug`: 开发模式（详细日志）
- `release`: 生产模式（精简日志）

### 数据库配置 (database)

| 配置项 | 说明 | 默认值 | 环境变量 |
|--------|------|--------|----------|
| `driver` | 数据库类型 | `mysql` | `DB_DRIVER` |
| `host` | 数据库主机 | `127.0.0.1` | `DB_HOST` |
| `port` | 数据库端口 | `3306` | `DB_PORT` |
| `username` | 数据库用户名 | `kubepolaris` | `DB_USERNAME` |
| `password` | 数据库密码 | - | `DB_PASSWORD` |
| `database` | 数据库名称 | `kubepolaris` | `DB_DATABASE` |
| `charset` | 字符集 | `utf8mb4` | `DB_CHARSET` |

### JWT 配置 (jwt)

| 配置项 | 说明 | 默认值 | 环境变量 |
|--------|------|--------|----------|
| `secret` | JWT 签名密钥 | - | `JWT_SECRET` |
| `expire_time` | Token 过期时间（小时） | `24` | `JWT_EXPIRE_TIME` |

**安全建议:**
- 使用强随机字符串作为 `secret`
- 生产环境建议设置为 32 字符以上

### 日志配置 (log)

| 配置项 | 说明 | 默认值 | 环境变量 |
|--------|------|--------|----------|
| `level` | 日志级别 | `info` | `LOG_LEVEL` |

**level 取值:**
- `debug`: 调试级别（最详细）
- `info`: 信息级别
- `warn`: 警告级别
- `error`: 错误级别（最精简）

### Kubernetes 配置 (k8s)

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `default_namespace` | 默认命名空间 | `default` |

### Grafana 配置 (grafana)

| 配置项 | 说明 | 默认值 | 环境变量 |
|--------|------|--------|----------|
| `enabled` | 是否启用 Grafana 集成 | `true` | `GRAFANA_ENABLED` |
| `url` | Grafana 访问地址 | `http://localhost:3000` | `GRAFANA_URL` |
| `api_key` | Grafana API Key | - | `GRAFANA_API_KEY` |
| `api_key_file` | API Key 文件路径 | - | `GRAFANA_API_KEY_FILE` |

**API Key 配置说明:**
- `api_key`: 直接配置 API Key（不推荐，明文存储）
- `api_key_file`: 从文件读取 API Key（推荐，更安全）
- 优先级: `api_key_file` > `api_key`

**路径说明:**
- Docker 环境: `/app/grafana/secrets/grafana_api_key`
- 本地开发: `./deploy/docker/grafana/secrets/grafana_api_key`

## 🔒 安全注意事项

1. **不要提交敏感信息到版本库**
   - `config.yaml` 已被 `.gitignore` 忽略
   - 只提交 `config.yaml.example` 模板文件

2. **保护配置文件权限**
   ```bash
   chmod 600 config.yaml
   ```

3. **使用强密码**
   - 数据库密码建议 16 字符以上
   - JWT Secret 建议 32 字符以上

4. **生产环境建议**
   - 使用环境变量覆盖配置（更安全）
   - 定期轮换密码和密钥
   - 启用 TLS/SSL 加密

## 🌍 环境变量覆盖

配置优先级（从高到低）：

1. **环境变量** ← 最高优先级
2. `config.yaml` 文件
3. 代码中的默认值

示例：

```bash
# 使用环境变量覆盖数据库配置
export DB_HOST=mysql-server
export DB_PASSWORD=my-secret-password
export JWT_SECRET=my-jwt-secret

# 启动应用（会使用环境变量）
./kubepolaris
```

## 📝 配置检查清单

部署前请确认：

- [ ] 已修改所有 `CHANGE_ME` 占位符
- [ ] 数据库连接信息正确
- [ ] JWT Secret 使用强随机字符串
- [ ] 日志级别适合当前环境
- [ ] Grafana 配置正确（如启用）
- [ ] 文件权限设置为 600

## 🆘 常见问题

### Q: config.yaml 找不到怎么办？

A: 运行安装脚本自动生成，或从模板复制：
```bash
cp config.yaml.example config.yaml
```

### Q: 环境变量不生效？

A: 检查：
1. 环境变量名称是否正确（见上表）
2. 是否在启动应用前设置
3. 是否使用了正确的导出方式

### Q: 本地开发时连接不上 MySQL？

A: 修改 `database.host`:
- Docker Compose: `mysql`（服务名）
- 本地开发: `localhost` 或 `127.0.0.1`

### Q: Grafana API Key 文件找不到？

A: 检查：
1. docker-compose 是否正常启动
2. grafana-init 容器是否运行成功
3. 路径是否正确（容器内 vs 宿主机）

## 📚 参考资料

- [Viper 配置管理](https://github.com/spf13/viper)
- [Docker Compose 环境变量](https://docs.docker.com/compose/environment-variables/)
- [KubePolaris 部署文档](../../README.md)

