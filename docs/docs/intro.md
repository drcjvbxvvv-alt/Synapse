---
sidebar_position: 1
slug: /
---

# 欢迎使用 Synapse

**Synapse**（北辰）是一个开源的企业级 Kubernetes 多集群管理平台，致力于简化 Kubernetes 的日常运维和管理工作，让团队能够更专注于业务创新。

> 「北辰」取自《论语》：「为政以德，譬如北辰，居其所而众星共之。」寓意 Synapse 作为 Kubernetes 集群管理的核心枢纽，统一管理、协调各个集群。

## ✨ 核心特性

### 🌐 多集群统一管理
- **集中管理** - 一个控制台管理所有 Kubernetes 集群
- **跨云支持** - 支持公有云、私有云、混合云、边缘集群
- **统一视图** - 跨集群的资源搜索和查看

### 📊 可视化工作负载管理
- **直观界面** - 可视化展示 Deployment、StatefulSet、DaemonSet 等
- **YAML 编辑** - 内置 Monaco Editor，支持语法高亮和自动补全
- **表单编辑** - 无需编写 YAML，通过表单完成配置

### 📈 实时监控与告警
- **Prometheus 集成** - 实时采集集群和应用指标
- **Grafana 面板** - 内嵌监控大盘，无需切换
- **智能告警** - AlertManager 集成，多渠道告警通知

### 💻 Web 终端
- **Pod 终端** - 直接在浏览器中连接 Pod 容器
- **SSH 终端** - 无需本地工具，直接 SSH 到节点
- **Kubectl 终端** - 在线执行 kubectl 命令

### 🔐 细粒度权限控制
- **RBAC 管理** - 基于角色的访问控制
- **自定义角色** - 支持创建自定义权限角色
- **资源级控制** - 精确到集群、命名空间、资源类型

### 🔄 GitOps 集成
- **ArgoCD 集成** - 支持 GitOps 工作流
- **应用管理** - 可视化管理 ArgoCD Applications
- **同步状态** - 实时查看同步状态和健康状态

### 📝 日志中心
- **实时日志** - Pod 日志实时查看
- **日志聚合** - 多 Pod 日志聚合展示
- **日志下载** - 支持日志导出下载

### 🔍 全局搜索
- **跨集群搜索** - 一键搜索所有集群资源
- **智能过滤** - 支持按资源类型、命名空间过滤
- **快捷跳转** - 搜索结果直达详情页

## 🏗️ 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Web Browser                          │
│                    (React + TypeScript)                     │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                     Synapse Backend                      │
│                      (Go + Gin + GORM)                       │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │  Auth   │  │  API    │  │ WebSock │  │    Services     │ │
│  │ Handler │  │ Handler │  │ Handler │  │ (K8s/Prom/Git)  │ │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │
└────────────────────────────────┬────────────────────────────┘
                                 │
          ┌──────────────────────┼──────────────────────┐
          ▼                      ▼                      ▼
   ┌────────────┐         ┌────────────┐         ┌────────────┐
   │   MySQL    │         │ Kubernetes │         │ Prometheus │
   │  Database  │         │  Clusters  │         │  / Grafana │
   └────────────┘         └────────────┘         └────────────┘
```

## 🚀 快速开始

### 使用 Docker Compose

```bash
# 克隆仓库
git clone https://github.com/clay-wangzhi/Synapse.git
cd synapse

# 启动服务
docker-compose up -d

# 访问 http://localhost:8080
```

### 使用 Helm

```bash
# 添加 Helm 仓库
helm repo add synapse https://clay-wangzhi.github.io/Synapse

# 安装
helm install synapse synapse/synapse \
  -n synapse --create-namespace
```

详细安装说明请查看 [安装指南](./getting-started/installation)。

## 📖 文档导航

| 文档 | 说明 |
|------|------|
| [快速开始](./getting-started/quick-start) | 5 分钟快速体验 |
| [安装指南](./getting-started/installation) | 详细安装部署说明 |
| [用户指南](./user-guide/cluster-management) | 功能使用说明 |
| [管理员指南](./admin-guide/deployment) | 运维管理指南 |
| [常见问题](./faq) | FAQ 解答 |

## 🤝 参与贡献

我们欢迎所有形式的贡献，包括但不限于：

- 🐛 提交 Bug 报告
- 💡 提出新功能建议
- 📝 完善文档
- 🔧 提交代码 PR

请查看 [贡献指南](https://github.com/clay-wangzhi/Synapse/blob/main/CONTRIBUTING.md) 了解详情。

## 📄 开源协议

Synapse 采用 [Apache License 2.0](https://github.com/clay-wangzhi/Synapse/blob/main/LICENSE) 开源协议。

## 🌟 Star History

如果 Synapse 对你有帮助，请给我们一个 Star ⭐️

[![Star History Chart](https://api.star-history.com/svg?repos=clay-wangzhi/Synapse&type=Date)](https://star-history.com/#clay-wangzhi/Synapse&Date)

