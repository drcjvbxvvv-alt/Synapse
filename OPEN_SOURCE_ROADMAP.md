# KubePolaris 开源成熟度路线图

> 本文档记录 KubePolaris 项目开源化的完整计划，按阶段逐步实现。
> 
> **目标**: 成为热门的企业级 Kubernetes 多集群管理平台
> 
> **创建时间**: 2026-01-07
> **最后更新**: 2026-01-07

---

## 📊 当前状态评估

| 维度 | 当前状态 | 成熟度 | 目标 |
|------|---------|--------|------|
| 核心功能 | ✅ 完整 | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| 测试覆盖 | ✅ 框架已建立 | ⭐⭐⭐ | ⭐⭐⭐⭐ (80%+) |
| CI/CD | ✅ 已完成 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| 容器化部署 | ✅ 完成 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| K8s 部署 | ❌ 无 Helm Chart | ⭐ | ⭐⭐⭐⭐⭐ |
| 文档 | 🔶 基础 README | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| 国际化 | ❌ 仅中文 | ⭐ | ⭐⭐⭐⭐ |
| API 文档 | ❌ 无 Swagger | ⭐ | ⭐⭐⭐⭐⭐ |
| 安全加固 | 🔶 基础 | ⭐⭐ | ⭐⭐⭐⭐ |
| 可观测性 | 🔶 基础日志 | ⭐⭐ | ⭐⭐⭐⭐ |

---

## 🔴 Phase 1：开源必备（预计 2-3 周）

### 1.1 测试体系建立

#### 1.1.1 后端测试

- [x] **添加测试框架依赖** ✅ (2026-01-07)
  - [x] `github.com/stretchr/testify` - 断言库
  - [x] `github.com/golang/mock` - Mock 框架
  - [x] `github.com/DATA-DOG/go-sqlmock` - 数据库 Mock

- [x] **Handler 层单元测试** ✅ (2026-01-07)
  ```
  internal/handlers/
  ├── cluster_test.go   ✅
  ├── pod_test.go       ✅
  ├── node_test.go      ✅
  ├── auth_test.go      ✅
  └── ...
  ```

- [x] **Service 层单元测试** ✅ (2026-01-07)
  ```
  internal/services/
  ├── cluster_service_test.go     ✅
  ├── permission_service_test.go  ✅
  └── ...
  ```

- [x] **集成测试** ✅ (2026-01-07)
  ```
  internal/integration/
  ├── integration_test.go      ✅
  └── api_integration_test.go  ✅
  ```

- [ ] **测试覆盖率目标**: >= 80% (待完善更多测试用例)

#### 1.1.2 前端测试

- [x] **添加测试依赖到 package.json** ✅ (2026-01-07)
  ```json
  {
    "devDependencies": {
      "@testing-library/react": "^16.x",
      "@testing-library/jest-dom": "^6.x",
      "@testing-library/user-event": "^14.x",
      "vitest": "^3.x",
      "@vitest/coverage-v8": "^3.x",
      "@vitest/ui": "^3.x",
      "msw": "^2.x",
      "jsdom": "^26.x"
    }
  }
  ```

- [x] **配置 Vitest** ✅ (2026-01-07)
  - [x] 创建 `vitest.config.ts`
  - [x] 创建 `ui/src/setupTests.ts`

- [x] **组件测试** ✅ (2026-01-07)
  ```
  ui/src/components/__tests__/
  ├── NamespaceSelector.test.tsx  ✅
  ├── PermissionGuard.test.tsx    ✅
  ├── SearchDropdown.test.tsx     ✅
  └── ...
  ```

- [ ] **页面测试** (待完善)
  ```
  ui/src/pages/__tests__/
  ├── cluster/ClusterList.test.tsx
  ├── pod/PodList.test.tsx
  └── ...
  ```

- [x] **Service 测试** ✅ (2026-01-07)
  ```
  ui/src/services/__tests__/
  ├── clusterService.test.ts  ✅ (13/13 通过)
  ├── authService.test.ts     ✅ (9/9 通过)
  ├── podService.test.ts      ✅ (10/11 通过)
  └── ...
  ```

---

### 1.2 CI/CD 流水线

#### 1.2.1 GitHub Actions 工作流

- [x] **创建 `.github/workflows/ci.yml`** ✅
  ```yaml
  # 主 CI 流程
  - 后端: lint -> test -> build
  - 前端: lint -> typecheck -> test -> build
  - 集成测试
  ```

- [x] **创建 `.github/workflows/release.yml`** ✅
  - 自动版本发布
  - Docker 镜像构建推送
  - Helm Chart 发布

- [x] **创建 `.github/workflows/security-scan.yml`** ✅
  - Trivy 容器扫描
  - gosec Go 安全扫描
  - npm audit

- [x] **创建 `.github/workflows/docs-deploy.yml`** ✅
  - 文档自动部署到 GitHub Pages

#### 1.2.2 GitHub 配置文件

- [x] **Issue Templates** ✅
  ```
  .github/ISSUE_TEMPLATE/
  ├── bug_report.md
  ├── feature_request.md
  └── config.yml
  ```

- [x] **PR Template** ✅
  - `.github/PULL_REQUEST_TEMPLATE.md`

- [x] **其他配置** ✅
  - [x] `.github/CODEOWNERS`
  - [x] `.github/dependabot.yml`

---

### 1.3 容器化部署

#### 1.3.1 Docker 支持

- [x] **创建 `Dockerfile`** (多阶段构建) ✅
  ```dockerfile
  # Stage 1: Build frontend
  # Stage 2: Build backend
  # Stage 3: Production image
  ```
  - 文件: `deploy/docker/Dockerfile`

- [x] **创建 `Dockerfile.backend`** (后端单独) ✅
  - 文件: `deploy/docker/Dockerfile.backend`

- [x] **创建 `Dockerfile.frontend`** (前端 Nginx) ✅
  - 文件: `deploy/docker/Dockerfile.frontend`
  - Nginx 配置: `deploy/docker/nginx.conf`, `deploy/docker/nginx-frontend.conf`

- [x] **创建 `.dockerignore`** ✅

- [x] **创建 `docker-compose.yml`** (本地开发) ✅
  - MySQL
  - KubePolaris Backend
  - KubePolaris Frontend
  - Prometheus (可选)
  - Grafana (包含自动初始化)

- [x] **创建 `docker-compose.prod.yml`** (生产环境) ✅

#### 1.3.2 Helm Chart

- [ ] **创建 Helm Chart 结构**
  ```
  deploy/helm/kubepolaris/
  ├── Chart.yaml
  ├── values.yaml
  ├── values-production.yaml
  ├── templates/
  │   ├── _helpers.tpl
  │   ├── deployment.yaml
  │   ├── service.yaml
  │   ├── ingress.yaml
  │   ├── configmap.yaml
  │   ├── secret.yaml
  │   ├── serviceaccount.yaml
  │   ├── rbac.yaml
  │   └── hpa.yaml (可选)
  └── README.md
  ```

- [ ] **支持的配置项**
  - 副本数
  - 资源限制
  - Ingress 配置
  - 数据库连接
  - 持久化存储

#### 1.3.3 部署脚本

- [x] **创建 `scripts/install.sh`** - 一键安装 ✅
- [x] **创建 `scripts/upgrade.sh`** - 升级脚本 ✅
- [x] **创建 `scripts/uninstall.sh`** - 卸载脚本 ✅

---

### 1.4 文档完善

#### 1.4.1 项目根目录文档

- [ ] **CONTRIBUTING.md** - 贡献指南
  - 开发环境搭建
  - 代码规范
  - PR 流程
  - Commit 规范

- [ ] **CODE_OF_CONDUCT.md** - 行为准则

- [ ] **CHANGELOG.md** - 变更日志
  - 使用 [Keep a Changelog](https://keepachangelog.com/) 格式
  - 配合 semantic versioning

- [ ] **SECURITY.md** - 安全政策
  - 漏洞报告流程
  - 安全联系方式

- [ ] **ROADMAP.md** - 公开路线图

#### 1.4.2 用户文档

```
docs/
├── README.md                    # 文档首页
├── getting-started/
│   ├── quick-start.md          # 5分钟快速开始
│   ├── installation.md         # 详细安装指南
│   │   ├── docker.md
│   │   ├── kubernetes.md
│   │   └── source.md
│   └── configuration.md        # 配置说明
├── user-guide/
│   ├── cluster-management.md   # 集群管理
│   ├── workload-management.md  # 工作负载管理
│   ├── pod-management.md       # Pod 管理
│   ├── monitoring-alerting.md  # 监控告警
│   ├── terminal-access.md      # 终端访问
│   └── rbac-permissions.md     # 权限管理
├── admin-guide/
│   ├── deployment.md           # 部署指南
│   ├── upgrade.md              # 升级指南
│   ├── backup-restore.md       # 备份恢复
│   ├── security.md             # 安全加固
│   ├── high-availability.md    # 高可用部署
│   └── troubleshooting.md      # 故障排查
└── faq.md                       # 常见问题
```

- [ ] 创建以上文档结构
- [ ] 编写快速开始指南
- [ ] 编写安装指南

---

### 1.5 API 文档（Swagger）

- [ ] **添加 Swagger 依赖**
  ```go
  // go.mod
  github.com/swaggo/swag
  github.com/swaggo/gin-swagger
  github.com/swaggo/files
  ```

- [ ] **添加 Swagger 注解**
  - [ ] 为所有 Handler 添加注解
  - [ ] 定义请求/响应结构

- [ ] **集成 Swagger UI**
  - 访问路径: `/api/docs`

- [ ] **生成 OpenAPI 规范文件**
  - `docs/api/openapi.yaml`

---

### 1.6 Makefile

- [x] **创建 `Makefile`** ✅
  ```makefile
  # 常用命令
  make dev          # 启动开发环境
  make build        # 构建项目
  make test         # 运行测试
  make lint         # 代码检查
  make docker-build # 构建 Docker 镜像
  make helm-package # 打包 Helm Chart
  make docs         # 生成文档
  make swagger      # 生成 Swagger 文档
  make clean        # 清理构建产物
  ```

---

## 🟡 Phase 2：企业级特性（预计 4-6 周）

### 2.1 国际化（i18n）

#### 2.1.1 前端国际化

- [ ] **添加 i18n 依赖**
  ```json
  {
    "dependencies": {
      "react-i18next": "^14.x",
      "i18next": "^23.x",
      "i18next-browser-languagedetector": "^7.x"
    }
  }
  ```

- [ ] **创建语言文件结构**
  ```
  ui/src/locales/
  ├── en-US/
  │   ├── common.json       # 通用文本
  │   ├── menu.json         # 菜单
  │   ├── cluster.json      # 集群相关
  │   ├── workload.json     # 工作负载相关
  │   ├── pod.json          # Pod 相关
  │   ├── node.json         # 节点相关
  │   ├── monitoring.json   # 监控相关
  │   ├── auth.json         # 认证相关
  │   └── errors.json       # 错误信息
  ├── zh-CN/
  │   └── ... (同上)
  └── index.ts
  ```

- [ ] **配置 i18n**
  - [ ] 创建 `ui/src/i18n.ts`
  - [ ] 在 `App.tsx` 中集成

- [ ] **改造现有组件**
  - [ ] 使用 `useTranslation` hook
  - [ ] 替换硬编码文本

- [ ] **语言切换功能**
  - [ ] 添加语言切换组件
  - [ ] 持久化语言选择

#### 2.1.2 后端国际化

- [ ] **错误信息国际化**
  ```
  internal/i18n/
  ├── en-US.yaml
  ├── zh-CN.yaml
  └── loader.go
  ```

- [ ] **API 响应国际化**
  - 根据请求头 `Accept-Language` 返回对应语言

---

### 2.2 OAuth2/OIDC 集成

- [ ] **添加 OIDC 依赖**
  ```go
  github.com/coreos/go-oidc/v3
  golang.org/x/oauth2
  ```

- [ ] **支持的 Provider**
  - [ ] 通用 OIDC Provider
  - [ ] Keycloak
  - [ ] Dex
  - [ ] Okta
  - [ ] GitHub OAuth

- [ ] **配置项**
  ```yaml
  auth:
    oidc:
      enabled: true
      issuer: https://keycloak.example.com/realms/kubepolaris
      client_id: kubepolaris
      client_secret: xxx
      redirect_uri: https://kubepolaris.example.com/callback
      scopes:
        - openid
        - profile
        - email
  ```

- [ ] **前端登录流程改造**
  - [ ] OAuth 登录按钮
  - [ ] 回调处理

---

### 2.3 多租户支持

- [ ] **数据模型**
  ```go
  // models/tenant.go
  type Tenant struct {
      ID          uint
      Name        string
      DisplayName string
      Quotas      TenantQuotas
      CreatedAt   time.Time
  }
  
  type TenantQuotas struct {
      MaxClusters   int
      MaxNamespaces int
      MaxPods       int
  }
  ```

- [ ] **租户隔离**
  - [ ] 数据隔离（数据库级别）
  - [ ] 集群权限隔离
  - [ ] Namespace 隔离

- [ ] **租户管理界面**
  - [ ] 租户列表
  - [ ] 租户创建/编辑/删除
  - [ ] 租户配额管理
  - [ ] 租户成员管理

---

### 2.4 成本分析

- [ ] **成本数据收集**
  - [ ] 资源使用量采集
  - [ ] 单价配置

- [ ] **成本模型**
  ```go
  type ResourceCost struct {
      ClusterID    uint
      Namespace    string
      WorkloadName string
      CPUCost      float64
      MemoryCost   float64
      StorageCost  float64
      NetworkCost  float64
      TotalCost    float64
      Period       string // daily, weekly, monthly
  }
  ```

- [ ] **成本报表**
  - [ ] 按集群统计
  - [ ] 按命名空间统计
  - [ ] 按工作负载统计
  - [ ] 趋势图表

- [ ] **成本告警**
  - [ ] 预算设置
  - [ ] 超支告警

---

### 2.5 可观测性增强

#### 2.5.1 结构化日志

- [ ] **日志格式改造**
  ```go
  // JSON 格式日志
  {
    "timestamp": "2026-01-07T10:00:00Z",
    "level": "info",
    "message": "cluster imported",
    "request_id": "abc-123",
    "user_id": 1,
    "cluster_id": 5,
    "duration_ms": 150
  }
  ```

- [ ] **Request ID 支持**
  - [ ] 中间件注入 Request ID
  - [ ] 全链路传递

#### 2.5.2 Metrics 暴露

- [ ] **添加 Prometheus client**
  ```go
  github.com/prometheus/client_golang
  ```

- [ ] **暴露指标**
  ```
  /metrics 端点
  
  # 请求相关
  kubepolaris_http_requests_total{method, path, status}
  kubepolaris_http_request_duration_seconds{method, path}
  
  # WebSocket 相关
  kubepolaris_websocket_connections_active
  kubepolaris_websocket_messages_total
  
  # 业务相关
  kubepolaris_clusters_total{status}
  kubepolaris_cluster_operations_total{operation, cluster}
  kubepolaris_k8s_client_requests_total{cluster, resource}
  kubepolaris_k8s_client_request_duration_seconds{cluster, resource}
  ```

#### 2.5.3 Tracing（可选）

- [ ] **OpenTelemetry 集成**
  ```go
  go.opentelemetry.io/otel
  go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin
  ```

---

### 2.6 UI/UX 优化

- [ ] **深色模式**
  - [ ] 全局主题切换
  - [ ] localStorage 持久化
  - [ ] 系统主题自动适配

- [ ] **快捷键支持**
  - [ ] `Ctrl+K` / `Cmd+K` - 全局搜索
  - [ ] `Ctrl+/` - 快捷键帮助
  - [ ] 导航快捷键

- [ ] **引导教程**
  - [ ] 首次使用引导
  - [ ] 功能介绍 Tour

---

## 🟢 Phase 3：社区运营（持续进行）

### 3.1 README 优化

- [ ] **README.md 重写**
  - [ ] 添加项目 Logo
  - [ ] 添加 Banner 图
  - [ ] 添加徽章 (CI/Coverage/License/Stars)
  - [ ] 添加功能截图
  - [ ] 添加 GIF 演示
  - [ ] 特性亮点
  - [ ] 快速开始
  - [ ] 文档链接
  - [ ] 贡献者展示

- [ ] **README_zh.md** - 中文版本

### 3.2 宣传物料

```
assets/
├── logo/
│   ├── logo-full-light.svg     # 完整 Logo (亮色背景)
│   ├── logo-full-dark.svg      # 完整 Logo (暗色背景)
│   ├── logo-icon.svg           # 图标
│   └── logo-icon.png           # 图标 PNG
├── screenshots/
│   ├── 01-overview.png         # 总览面板
│   ├── 02-cluster-list.png     # 集群列表
│   ├── 03-workload-detail.png  # 工作负载详情
│   ├── 04-terminal.png         # 终端功能
│   ├── 05-monitoring.png       # 监控面板
│   └── 06-alerts.png           # 告警中心
├── demo/
│   ├── demo.gif                # 功能演示 GIF
│   └── demo-video.mp4          # 完整演示视频
├── architecture/
│   ├── architecture.png        # 架构图
│   └── architecture.drawio     # 可编辑源文件
└── social/
    ├── twitter-card.png        # Twitter 分享图
    └── og-image.png            # Open Graph 图片
```

### 3.3 Demo 环境

- [ ] **在线 Demo**
  - [ ] 部署只读演示环境
  - [ ] 提供测试账号
  - [ ] 自动重置数据

- [ ] **Playground**（可选）
  - 可交互的沙箱环境

### 3.4 社区渠道

- [ ] **开启 GitHub Discussions**
- [ ] **创建交流群**
  - Slack / Discord
  - 微信群 / 钉钉群

- [ ] **社交媒体**
  - Twitter / X
  - 技术博客

### 3.5 技术内容输出

- [ ] **技术博客**
  - 项目介绍
  - 技术架构解析
  - 最佳实践

- [ ] **视频教程**
  - 安装部署
  - 功能演示
  - 开发教程

---

## 🔵 Phase 4：高级功能（长期规划）

### 4.1 功能增强

- [ ] **资源配额管理**
  - Namespace 级别配额
  - 配额使用统计

- [ ] **网络策略管理**
  - NetworkPolicy CRUD
  - 可视化编辑

- [ ] **Service Mesh 集成**
  - Istio 可视化
  - 流量管理

- [ ] **备份恢复**
  - Velero 集成
  - 备份策略配置

- [ ] **集群生命周期管理**
  - Cluster API 集成
  - 集群创建向导
  - 集群升级向导

### 4.2 插件系统

```
plugins/
├── interface.go            # 插件接口定义
├── loader.go               # 插件加载器
├── registry.go             # 插件注册表
└── examples/
    ├── cost-analysis/      # 成本分析插件
    ├── security-audit/     # 安全审计插件
    └── custom-dashboard/   # 自定义大盘
```

### 4.3 Webhook 支持

- [ ] **事件 Webhook**
  - 集群事件
  - 工作负载事件
  - 告警事件

- [ ] **通知渠道**
  - 钉钉
  - 企业微信
  - Slack
  - 自定义 HTTP

---

## 📁 目标项目结构

```
KubePolaris/
├── .github/
│   ├── workflows/
│   │   ├── ci.yml
│   │   ├── release.yml
│   │   ├── security-scan.yml
│   │   └── docs-deploy.yml
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.md
│   │   ├── feature_request.md
│   │   └── config.yml
│   ├── PULL_REQUEST_TEMPLATE.md
│   ├── CODEOWNERS
│   └── dependabot.yml
├── cmd/
│   └── main.go
├── internal/
│   ├── handlers/
│   │   ├── *_test.go          # 测试文件
│   │   └── ...
│   ├── services/
│   │   ├── *_test.go          # 测试文件
│   │   └── ...
│   ├── models/
│   ├── middleware/
│   ├── i18n/                   # 国际化
│   └── integration/            # 集成测试
├── ui/
│   ├── src/
│   │   ├── locales/           # 国际化资源
│   │   └── ...
│   ├── __tests__/             # 测试文件
│   ├── vitest.config.ts
│   └── setupTests.ts
├── deploy/                     # ✅ 部署相关文件
│   ├── docker/                # Docker 相关配置
│   │   ├── kubepolaris/       # KubePolaris Dockerfile
│   │   │   ├── Dockerfile
│   │   │   ├── Dockerfile.backend
│   │   │   └── Dockerfile.frontend
│   │   ├── mysql/             # MySQL 配置
│   │   └── grafana/           # Grafana 配置
│   ├── docker-compose/        # Docker Compose 文件
│   │   ├── docker-compose.yml
│   │   └── docker-compose.prod.yml
│   ├── scripts/               # 部署脚本
│   │   ├── install.sh
│   │   ├── upgrade.sh
│   │   └── uninstall.sh
│   └── yaml/                  # K8s YAML 文件
├── docs/
│   ├── getting-started/
│   ├── user-guide/
│   ├── admin-guide/
│   ├── developer-guide/
│   ├── api/
│   │   └── openapi.yaml
│   └── README.md
├── e2e/                        # E2E 测试
├── assets/
│   ├── logo/
│   ├── screenshots/
│   └── demo/
├── .env.example
├── Makefile
├── README.md
├── README_zh.md
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
├── CHANGELOG.md
├── SECURITY.md
├── ROADMAP.md
├── OPEN_SOURCE_ROADMAP.md      # 本文件
└── LICENSE
```

---

## 📈 进度追踪

### Phase 1 进度

| 任务 | 状态 | 负责人 | 开始时间 | 完成时间 | 备注 |
|------|------|--------|----------|----------|------|
| 后端测试框架 | ✅ 已完成 | AI | 2026-01-07 | 2026-01-07 | testify/sqlmock + Handler/Service/集成测试示例 |
| 前端测试框架 | ✅ 已完成 | AI | 2026-01-07 | 2026-01-07 | Vitest + Testing Library + 组件/Service测试示例 |
| CI/CD 流水线 | ✅ 已完成 | - | 2026-01-07 | 2026-01-07 | 包含 CI/Release/Security/Docs 工作流 |
| Dockerfile | ✅ 已完成 | - | 2026-01-07 | 2026-01-07 | 多阶段构建+前后端分离镜像 |
| docker-compose | ✅ 已完成 | - | 2026-01-07 | 2026-01-07 | 开发环境+生产环境，含 Grafana |
| 部署脚本 | ✅ 已完成 | - | 2026-01-07 | 2026-01-07 | install/upgrade/uninstall |
| Helm Chart | ⏳ 待开始 | - | - | - | |
| CONTRIBUTING.md | ⏳ 待开始 | - | - | - | |
| CHANGELOG.md | ⏳ 待开始 | - | - | - | |
| Swagger 集成 | ⏳ 待开始 | - | - | - | |
| Makefile | ✅ 已完成 | - | 2026-01-07 | 2026-01-07 | 开发/构建/测试/部署命令 |

### Phase 2 进度

| 任务 | 状态 | 负责人 | 开始时间 | 完成时间 | 备注 |
|------|------|--------|----------|----------|------|
| 前端国际化 | ⏳ 待开始 | - | - | - | |
| 后端国际化 | ⏳ 待开始 | - | - | - | |
| OAuth2/OIDC | ⏳ 待开始 | - | - | - | |
| 多租户 | ⏳ 待开始 | - | - | - | |
| 成本分析 | ⏳ 待开始 | - | - | - | |
| Metrics 暴露 | ⏳ 待开始 | - | - | - | |
| 深色模式 | ⏳ 待开始 | - | - | - | |

---

## 📝 注意事项

1. **每一步都要做好**
   - 代码质量优先
   - 完善的测试
   - 清晰的文档

2. **保持向后兼容**
   - API 版本管理
   - 数据库迁移脚本
   - 配置兼容

3. **安全第一**
   - 定期安全扫描
   - 及时修复漏洞
   - 安全最佳实践

4. **社区友好**
   - 快速响应 Issue
   - 友好的 PR Review
   - 定期发布更新

---

## 🔗 参考项目

以下是一些优秀的开源 K8s 管理平台，可以参考其实践：

- [Rancher](https://github.com/rancher/rancher) - 多集群管理
- [Kubesphere](https://github.com/kubesphere/kubesphere) - 云原生平台
- [Lens](https://github.com/lensapp/lens) - K8s IDE
- [Headlamp](https://github.com/headlamp-k8s/headlamp) - K8s UI
- [Kubernetes Dashboard](https://github.com/kubernetes/dashboard) - 官方 Dashboard

---

> 💡 **提示**: 建议按照 Phase 顺序执行，每完成一个阶段做一次版本发布。


