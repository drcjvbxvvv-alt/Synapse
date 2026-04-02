---
sidebar_position: 4
---

# 升级指南

本文档说明如何将 Synapse 升级到新版本。

## 升级前准备

### 1. 备份数据

**强烈建议在升级前备份数据库！**

```bash
# Docker Compose 部署
docker exec synapse-mysql mysqldump -u root -p synapse > backup_$(date +%Y%m%d_%H%M%S).sql

# Kubernetes 部署
kubectl exec -it deployment/synapse-mysql -n synapse -- mysqldump -u root -p synapse > backup_$(date +%Y%m%d_%H%M%S).sql
```

### 2. 查看变更日志

访问 [CHANGELOG](https://github.com/clay-wangzhi/Synapse/blob/main/CHANGELOG.md) 了解版本变更：

- **Breaking Changes**: 不兼容的变更，需要特别注意
- **New Features**: 新功能
- **Bug Fixes**: 问题修复
- **Migration Guide**: 迁移指南（如有）

### 3. 检查兼容性

确认新版本与当前环境的兼容性：

- 数据库版本要求
- Kubernetes 版本要求
- 配置文件变更

## Docker Compose 升级

### 方式一：使用 latest 标签

```bash
# 拉取最新镜像
docker-compose pull

# 重启服务
docker-compose up -d

# 查看日志确认升级成功
docker-compose logs -f backend
```

### 方式二：指定版本号

```bash
# 编辑 .env 文件
echo "VERSION=v1.2.0" >> .env

# 或直接修改 docker-compose.yml
# image: synapse/synapse-backend:v1.2.0

# 拉取新版本
docker-compose pull

# 重启服务
docker-compose up -d
```

### 回滚

```bash
# 使用旧版本镜像
export VERSION=v1.1.0
docker-compose up -d

# 或从备份恢复数据库
cat backup_20260107_120000.sql | docker exec -i synapse-mysql mysql -u root -p synapse
```

## Kubernetes (Helm) 升级

### 1. 更新 Helm 仓库

```bash
helm repo update synapse
```

### 2. 查看可用版本

```bash
helm search repo synapse --versions
```

### 3. 查看升级差异

```bash
# 使用 helm diff 插件（需要安装）
helm diff upgrade synapse synapse/synapse \
  -n synapse \
  -f values.yaml

# 或使用 --dry-run
helm upgrade synapse synapse/synapse \
  -n synapse \
  -f values.yaml \
  --dry-run
```

### 4. 执行升级

```bash
# 升级到最新版本
helm upgrade synapse synapse/synapse \
  -n synapse \
  -f values.yaml

# 升级到指定版本
helm upgrade synapse synapse/synapse \
  -n synapse \
  -f values.yaml \
  --version 1.2.0
```

### 5. 验证升级

```bash
# 查看升级历史
helm history synapse -n synapse

# 查看 Pod 状态
kubectl get pods -n synapse -w

# 查看日志
kubectl logs -f deployment/synapse-backend -n synapse
```

### 回滚

```bash
# 查看历史版本
helm history synapse -n synapse

# 回滚到上一个版本
helm rollback synapse -n synapse

# 回滚到指定版本
helm rollback synapse 2 -n synapse
```

## 数据库迁移

Synapse 使用 GORM 的 AutoMigrate 功能，大多数情况下数据库结构会自动更新。

### 自动迁移

应用启动时会自动执行数据库迁移。查看日志确认迁移成功：

```bash
# Docker Compose
docker-compose logs backend | grep -i migration

# Kubernetes
kubectl logs deployment/synapse-backend -n synapse | grep -i migration
```

### 手动迁移

某些大版本升级可能需要手动执行迁移脚本：

```bash
# 查看迁移脚本
ls migrations/

# 执行迁移
mysql -u root -p synapse < migrations/v1.2.0.sql
```

## 版本升级路径

### 跨版本升级

建议按顺序逐版本升级，避免直接跨多个大版本：

```
v1.0.0 → v1.1.0 → v1.2.0 → v2.0.0
```

### 特殊版本说明

#### v1.x → v2.0.0

重大升级，需要注意：

1. 配置文件格式变更
2. API 路径变更
3. 数据库结构变更

详细迁移指南请参考 [v2.0.0 迁移指南](https://github.com/clay-wangzhi/Synapse/releases/tag/v2.0.0)。

## 升级检查清单

升级前：
- [ ] 阅读 CHANGELOG
- [ ] 备份数据库
- [ ] 记录当前版本
- [ ] 检查配置兼容性
- [ ] 通知相关人员

升级中：
- [ ] 拉取新版本镜像/Chart
- [ ] 更新配置文件（如需要）
- [ ] 执行升级命令
- [ ] 等待服务就绪

升级后：
- [ ] 验证服务可用
- [ ] 检查功能正常
- [ ] 确认日志无错误
- [ ] 测试关键功能
- [ ] 更新文档

## 常见问题

### 升级后无法启动

1. 检查日志获取错误信息
2. 确认配置文件兼容性
3. 检查数据库连接
4. 尝试回滚到上一版本

### 数据库迁移失败

1. 检查数据库连接配置
2. 确认用户权限足够
3. 手动执行迁移脚本
4. 从备份恢复

### 配置不兼容

1. 对比新旧版本配置模板
2. 更新配置文件格式
3. 添加必需的新配置项

## 下一步

- [故障排查](../admin-guide/troubleshooting) - 解决升级问题
- [备份恢复](../admin-guide/backup-restore) - 数据备份策略

