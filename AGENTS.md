# AI 开发者指南 - KubePolaris（北辰）

> 本文档专为 AI 开发者设计，提供项目的完整技术上下文和开发指导。

## 🎯 项目概览

**项目类型**: 企业级 Kubernetes 集群管理平台  
**架构模式**: 前后端分离  
**开发状态**: 核心功能已完成，Pod 终端功能需要完善  
**技术栈**: React + TypeScript + Go + MySQL + Kubernetes

## 🏗️ 技术架构详情

### 前端架构 (k8s-management-frontend/)
```
技术栈:
- React 19.1.1 + TypeScript 5.8.3
- Vite 7.1.2 (构建工具)
- Ant Design 5.x (UI 组件库)
- React Router DOM 7.8.0 (路由)
- Axios 1.11.0 (HTTP 客户端)
- Monaco Editor 4.7.0 (代码编辑器)
- xterm.js 5.3.0 (终端组件)

项目结构:
src/
├── components/          # 通用组件
├── pages/              # 页面组件
│   ├── cluster/        # 集群管理页面
│   ├── node/           # 节点管理页面
│   ├── pod/            # Pod 管理页面
│   ├── workload/       # 工作负载页面
│   ├── search/         # 搜索页面
│   └── yaml/           # YAML 编辑页面
├── services/           # API 服务层
├── types/              # TypeScript 类型定义
└── utils/              # 工具函数
```

### 后端架构 (k8s-management-backend/)
```
技术栈:
- Go 1.24.0 + Gin 1.9.1 (Web 框架)
- GORM 1.30.1 + MySQL 8.x (数据库)
- k8s.io/client-go 0.29.0 (Kubernetes 客户端)
- Gorilla WebSocket 1.5.3 (WebSocket)
- Viper 1.17.0 (配置管理)
- JWT (身份认证)

项目结构:
internal/
├── handlers/           # HTTP 处理器
├── services/           # 业务服务层
├── models/             # 数据模型
├── middleware/         # 中间件
├── config/             # 配置管理
├── database/           # 数据库层
└── router/             # 路由配置
```

## 📊 功能模块状态

### ✅ 已完成模块
1. **集群管理** (`handlers/cluster.go`, `pages/cluster/`)
   - 集群导入、列表、详情、删除
   - 集群连接测试和状态监控
   - API: `/api/clusters/*`

2. **节点管理** (`handlers/node.go`, `pages/node/`)
   - 节点列表、详情、监控
   - 节点操作 (Cordon/Uncordon/Drain)
   - API: `/api/clusters/:id/nodes/*`

3. **Pod 管理** (`handlers/pod.go`, `pages/pod/`)
   - Pod 列表、详情、日志查看
   - Pod 删除操作
   - API: `/api/clusters/:id/pods/*`

4. **工作负载管理** (`handlers/workload.go`, `pages/workload/`)
   - Deployment/StatefulSet/DaemonSet 等管理
   - 扩缩容、YAML 编辑
   - API: `/api/clusters/:id/workloads/*`

5. **全局搜索** (`handlers/search.go`, `pages/search/`)
   - 跨集群资源搜索
   - API: `/api/search`

6. **终端功能** (部分完成)
   - ✅ Kubectl 终端 (`handlers/kubectl_terminal.go`)
   - ✅ SSH 终端 (`handlers/ssh_terminal.go`)
   - 🚧 Pod 终端 (`handlers/terminal.go` - 需要完善)

### 🚧 需要完善的功能

#### Pod 终端 WebSocket 实现
**当前状态**: 基础框架已存在，但 WebSocket 实现不完整  
**文件位置**: `internal/handlers/terminal.go`  
**问题**: `PodTerminal` 方法只是占位符，需要完整的 WebSocket 实现

**需要实现的功能**:
```go
// 需要在 terminal.go 中实现
func (h *TerminalHandler) PodTerminal(c *gin.Context) {
    // 1. 参数解析和验证
    // 2. WebSocket 升级
    // 3. K8s Pod Exec 连接
    // 4. 双向数据流转发
    // 5. 会话审计记录
}
```

**WebSocket 路由**: `ws.GET("/clusters/:clusterId/pods/:namespace/:name/terminal", terminalHandler.PodTerminal)`

## 🔧 开发环境和工具

### 启动命令
```bash
# 后端 (端口 8080)
cd k8s-management-backend && go run main.go

# 前端 (端口 5173)
cd k8s-management-frontend && npm run dev
```

### 关键配置文件
- 后端配置: `configs/config.yaml`
- 数据库初始化: `setup_mysql.sql`
- 前端配置: `vite.config.ts`

### 重要服务类
- **K8s 客户端**: `internal/services/k8s_client.go`
- **集群服务**: `internal/services/cluster_service.go`
- **路由配置**: `internal/router/router.go`

## 📋 开发任务状态

### 当前里程碑 (第 5-6 周)
- [x] 集群管理功能完成
- [x] 节点管理功能完成  
- [x] Pod 管理基础功能完成
- [x] 工作负载管理完成
- [x] 全局搜索完成
- [ ] **Pod 终端 WebSocket 功能** (当前重点)
- [ ] 实时日志流优化
- [ ] 监控图表集成

### 下一阶段任务
- [ ] 用户认证和权限管理
- [ ] 审计功能完善
- [ ] 性能优化和缓存
- [ ] 安全加固

## 🎨 代码规范

### 前端规范
```typescript
// 组件命名: PascalCase
const ComponentName: React.FC<Props> = ({ prop1, prop2 }) => {
  // 使用 TypeScript 类型定义
  // 遵循 React Hooks 模式
  // 使用 Ant Design 组件
};

// 文件命名
- 组件: PascalCase.tsx
- 工具: camelCase.ts
- 常量: UPPER_CASE.ts
```

### 后端规范
```go
// 处理器结构
func (h *Handler) MethodName(c *gin.Context) {
    // 1. 参数验证
    // 2. 业务逻辑调用
    // 3. 统一响应格式
    c.JSON(200, gin.H{
        "code": 200,
        "message": "success",
        "data": result,
    })
}

// 错误处理
if err != nil {
    c.JSON(500, gin.H{
        "code": 500,
        "message": err.Error(),
        "data": nil,
    })
    return
}
```

## 🔍 关键实现细节

### WebSocket 终端实现模式
```go
// 参考 kubectl_terminal.go 的实现模式
func (h *TerminalHandler) PodTerminal(c *gin.Context) {
    // 1. WebSocket 升级
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    
    // 2. K8s Exec 连接
    req := clientset.CoreV1().RESTClient().Post().
        Resource("pods").
        Name(podName).
        Namespace(namespace).
        SubResource("exec")
    
    // 3. SPDY 执行器
    exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
    
    // 4. 流处理和转发
    // 5. 会话审计
}
```

### 数据库模型
```go
// 主要模型位置: internal/models/
type Cluster struct {
    ID        uint   `gorm:"primaryKey"`
    Name      string `gorm:"unique;not null"`
    APIServer string `gorm:"not null"`
    // ... 其他字段
}

type TerminalSession struct {
    ID        uint   `gorm:"primaryKey"`
    UserID    uint   `gorm:"not null"`
    ClusterID uint   `gorm:"not null"`
    // ... 审计字段
}
```

## 🚨 注意事项

### 安全考虑
- Kubernetes 凭证加密存储
- WebSocket 连接认证
- 操作审计记录
- RBAC 权限控制

### 性能优化
- K8s 客户端连接池
- 内存缓存 (TTL)
- 分页查询
- WebSocket 连接管理

### 错误处理
- 统一错误响应格式
- 前端错误边界
- 日志记录和监控
- 用户友好的错误提示

## 📚 参考资源

### 技术文档
- [Kubernetes client-go](https://github.com/kubernetes/client-go)
- [Gin Web Framework](https://gin-gonic.com/)
- [Ant Design React](https://ant.design/)
- [xterm.js](https://xtermjs.org/)

### 项目文档
- `plan.md` - 详细开发计划
- `task-list.md` - 任务清单和进度
- `prototypes/` - 原型设计文档

## 🎯 AI 开发建议

### 优先级任务
1. **完善 Pod 终端功能** - 当前最重要的任务
2. **优化 WebSocket 实现** - 提升实时通信稳定性
3. **完善错误处理** - 提升用户体验
4. **性能优化** - 支持大规模集群

### 开发策略
- 基于现有代码结构进行扩展
- 参考已完成的终端实现 (kubectl_terminal.go)
- 保持代码风格一致性
- 注重安全性和性能

### 测试建议
- 单元测试覆盖核心逻辑
- 集成测试验证 API 功能
- WebSocket 连接稳定性测试
- 多集群环境兼容性测试

---

**最后更新**: 2024-08-26  
**项目状态**: 核心功能完成，Pod 终端功能待完善  
**下一步**: 完善 WebSocket 终端实现，优化用户体验