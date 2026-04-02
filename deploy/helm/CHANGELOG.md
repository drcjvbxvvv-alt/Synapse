# Helm Charts 实现 - 变更日志

## [1.0.1] - 2026-01-27

### 修复 🐛

#### 前端只读文件系统问题
- 🐛 修复前端 Nginx 在只读根文件系统下无法启动的问题
- ✅ 为前端容器添加了必要的临时卷挂载：
  - `/tmp` - Nginx 临时文件目录
  - `/var/cache/nginx` - Nginx 缓存目录
  - `/var/run` - 运行时 PID 文件目录
- 🔒 保持了 `readOnlyRootFilesystem: true` 的安全配置

#### 修改文件
- 📝 `templates/frontend-deployment.yaml` - 添加 emptyDir 卷挂载
- 📝 `Chart.yaml` - 版本号更新至 1.0.1
- 📝 `README.md` - 添加故障排查指南

#### 新增文档
- 📚 `UPGRADE_GUIDE.md` - Helm Chart 升级指南

### 技术细节

**问题分析:**
- 错误信息: `nginx: [emerg] mkdir() "/tmp/client_temp" failed (30: Read-only file system)`
- 根本原因: 启用 `readOnlyRootFilesystem: true` 后，Nginx 无法创建临时目录
- 影响版本: v1.0.0

**解决方案:**
```yaml
volumeMounts:
  - name: tmp
    mountPath: /tmp
  - name: nginx-cache
    mountPath: /var/cache/nginx
  - name: nginx-run
    mountPath: /var/run

volumes:
  - name: tmp
    emptyDir: {}
  - name: nginx-cache
    emptyDir: {}
  - name: nginx-run
    emptyDir: {}
```

**升级指引:**
```bash
# Helm 升级
helm upgrade synapse ./deploy/helm/synapse -n synapse

# 验证
kubectl rollout status deployment/synapse-frontend -n synapse
```

---

## [1.0.0] - 2026-01-23

### 新增 🎉

#### Helm Chart 完整实现
- ✨ 创建完整的 Helm Chart 结构（25 个文件）
- ✨ 支持 Kubernetes 1.20+ 部署
- ✨ 提供 3 种配置模式（default/ha/production）

#### Chart 文件
- 📦 Chart.yaml - Chart 元数据定义
- 📦 values.yaml - 默认配置（9.5KB）
- 📦 values-ha.yaml - 高可用配置
- 📦 values-production.yaml - 生产环境配置
- 📦 .helmignore - Helm 忽略规则

#### Kubernetes 资源模板
- 🔧 ConfigMap - 应用配置管理
- 🔐 Secret - 密钥管理（JWT、MySQL）
- 👤 ServiceAccount - 服务账号
- 🛡️ RBAC - ClusterRole + ClusterRoleBinding
- 💾 MySQL StatefulSet + Service + PVC
- 🔙 Backend Deployment + Service
- 🎨 Frontend Deployment + Service
- 🌐 Ingress - 外部访问
- 📈 HPA - 水平自动扩缩容
- 🛡️ PDB - Pod 中断预算
- 🧪 Test - 连接测试

#### 辅助工具
- 🚀 quick-deploy.sh - 一键快速部署脚本（可执行）
- 📋 NOTES.txt - 安装后提示信息
- 🔧 _helpers.tpl - 模板辅助函数

#### 文档
- 📚 deploy/helm/README.md - Helm 部署总指南
- 📚 deploy/helm/synapse/README.md - Chart 详细文档（8KB）
- 📚 deploy/helm/IMPLEMENTATION_REPORT.md - 实现报告

#### Makefile 集成
- ⚙️ `make helm-lint` - 验证 Chart 语法
- ⚙️ `make helm-package` - 打包 Chart
- ⚙️ `make helm-install` - 快速安装
- ⚙️ `make helm-uninstall` - 卸载 Chart

### 功能特性 ✨

#### 部署模式
- 🔹 基础模式 - 内置 MySQL，适合开发测试
- 🔹 高可用模式 - 3 副本 + 反亲和 + HPA
- 🔹 生产模式 - 外部数据库 + Ingress + 完整监控

#### 配置选项
- ⚙️ 副本数配置（Backend/Frontend）
- ⚙️ 资源限制配置（CPU/Memory）
- ⚙️ 存储配置（PVC/StorageClass）
- ⚙️ 网络配置（Ingress/Service）
- ⚙️ 安全配置（RBAC/Secret）
- ⚙️ 监控集成（Prometheus/Grafana）
- ⚙️ 节点调度（NodeSelector/Affinity/Tolerations）

#### 高级特性
- 🔄 自动扩缩容（HPA）
- 🛡️ Pod 中断预算（PDB）
- 🔐 完整的 RBAC 权限控制
- 📊 监控集成支持
- 🌐 Ingress 支持（Nginx/Traefik）
- 💾 持久化存储支持
- 🔧 健康检查和就绪探针
- 🧪 Helm 测试支持

### 更新 📝

#### 项目文档
- 📝 更新 README.md - 添加 Kubernetes 部署说明
- 📝 更新 Makefile - 添加 Helm 相关命令和帮助信息

#### 部署说明
- 📝 README.md: 添加 Helm Chart 快速部署方式
- 📝 Makefile help: 添加 Helm 命令说明

### 技术细节 🔧

#### 模板功能
- ✅ 条件渲染（内置/外部 MySQL）
- ✅ 循环遍历（Ingress paths）
- ✅ 变量引用（辅助函数）
- ✅ 密钥管理（existingSecret 支持）
- ✅ 镜像配置（registry/repository/tag）
- ✅ 资源计算（requests/limits）

#### 最佳实践
- ✅ 遵循 Helm Chart 最佳实践
- ✅ 使用模板辅助函数（_helpers.tpl）
- ✅ 支持自定义配置覆盖
- ✅ 提供合理的默认值
- ✅ 完善的标签和选择器
- ✅ 健康检查和探针配置
- ✅ 安全上下文配置

### 部署场景 🎯

#### 场景 1: 快速体验
```bash
cd deploy/helm/synapse
./quick-deploy.sh
```

#### 场景 2: 基础部署
```bash
helm install synapse ./deploy/helm/synapse \
  -n synapse --create-namespace \
  --set security.jwtSecret="your-secret"
```

#### 场景 3: 高可用部署
```bash
helm install synapse ./deploy/helm/synapse \
  -n synapse -f values-ha.yaml
```

#### 场景 4: 生产环境
```bash
helm install synapse ./deploy/helm/synapse \
  -n synapse -f values-production.yaml
```

### 验证 ✅

#### 语法验证
- ✅ Helm lint 通过
- ✅ 模板渲染测试通过
- ✅ Kubernetes API 验证通过

#### 功能验证
- ✅ Secret 自动生成
- ✅ ConfigMap 正确渲染
- ✅ RBAC 权限配置正确
- ✅ Service 端口配置正确
- ✅ Ingress 路由配置正确
- ✅ 健康检查配置正确

### 文档完整性 📚

#### Chart 文档
- ✅ README.md - 完整使用说明
- ✅ values.yaml - 详细参数注释
- ✅ NOTES.txt - 安装后提示
- ✅ 部署场景示例

#### 项目文档
- ✅ deploy/helm/README.md - 总体指南
- ✅ IMPLEMENTATION_REPORT.md - 实现报告
- ✅ 更新项目主 README.md

### 影响范围 📊

#### 新增目录
```
deploy/helm/
├── README.md (新增)
├── IMPLEMENTATION_REPORT.md (新增)
└── synapse/ (新增)
    ├── 25 个文件
    └── templates/ (17 个模板)
```

#### 修改文件
- 📝 README.md - 添加 K8s 部署说明
- 📝 Makefile - 添加 4 个 Helm 命令

### 兼容性 🔄

#### Kubernetes 版本
- ✅ Kubernetes 1.20+
- ✅ Helm 3.0+

#### 功能兼容
- ✅ 与 Docker Compose 部署并存
- ✅ 与文档规划完全一致
- ✅ 支持现有配置迁移

### 后续工作 🚀

#### 短期
- [ ] 在真实集群中测试
- [ ] 添加更多监控集成
- [ ] 完善 Grafana 自动配置

#### 中期
- [ ] 发布到 Helm 仓库
- [ ] CI/CD 自动测试
- [ ] 更多配置选项

#### 长期
- [ ] 支持 PostgreSQL
- [ ] 多监控系统支持
- [ ] 插件化架构

---

**作者:** Synapse Team  
**日期:** 2026-01-23  
**版本:** 1.0.0  
**状态:** ✅ 完成
