# Synapse Gateway API 架構設計文件

> 版本：v1.2 | 日期：2026-04-07 | 狀態：Phase 1 & Phase 2 已實作
> 對應里程碑：M-GW（Gateway API 整合）

---

## 目錄

1. [背景與動機](#1-背景與動機)
2. [Gateway API 資源模型](#2-gateway-api-資源模型)
3. [整體架構](#3-整體架構)
4. [後端設計](#4-後端設計)
5. [前端設計](#5-前端設計)
6. [資料模型與型別定義](#6-資料模型與型別定義)
7. [API 路由設計](#7-api-路由設計)
8. [版本相容策略](#8-版本相容策略)
9. [錯誤處理與邊界情況](#9-錯誤處理與邊界情況)
10. [RBAC 權限設計](#10-rbac-權限設計)
11. [實作路線圖](#11-實作路線圖)
12. [技術選型決策](#12-技術選型決策)

---

## 1. 背景與動機

### 為什麼要整合 Gateway API

Kubernetes Ingress API 存在以下根本限制：

| 問題 | 說明 |
|------|------|
| 角色混用 | 基礎設施配置（TLS、後端協定）與應用路由混在同一資源 |
| 功能貧乏 | 流量權重、Header 改寫、鏡流等進階功能依賴 annotation，缺乏標準 |
| 可攜性差 | 不同 Ingress controller 的 annotation 互不相容 |
| 無 cross-namespace | 不支援 Route 跨 namespace 引用 Service |

**Gateway API（`gateway.networking.k8s.io`）** 自 Kubernetes 1.28 起進入 GA（`v1`），已被
Contour、Cilium、Istio、Nginx、Traefik、Kong 等主流 controller 支援，是官方認定的 Ingress 長期接班人。

### Synapse 的目標

- 讓用戶在 Synapse UI 內完整管理 Gateway API 資源（CRUD + YAML）
- 自動偵測叢集是否安裝 Gateway API CRD，未安裝時提供引導
- 與現有 Ingress 頁面並列，逐步引導用戶遷移
- Phase 1 以唯讀展示為主，Phase 2 補全 CRUD，Phase 3 增加進階功能

---

## 2. Gateway API 資源模型

### 資源層級與角色分工

```
┌─────────────────────────────────────────────────────────┐
│  基礎設施管理員 (Infra Admin)                             │
│                                                          │
│  GatewayClass  (cluster-scoped)                          │
│  ├── 綁定特定 controller (e.g. nginx, istio, contour)    │
│  └── 描述 controller 能力（參數 ParametersRef）           │
└────────────────────────┬────────────────────────────────┘
                         │ 1:N
┌────────────────────────▼────────────────────────────────┐
│  叢集操作員 (Cluster Operator)                            │
│                                                          │
│  Gateway  (namespaced)                                   │
│  ├── Listeners[]  定義 port/protocol/TLS                 │
│  ├── Addresses[]  分配的外部 IP/hostname                  │
│  └── Status.Conditions  (Accepted / Programmed)         │
└────────────────────────┬────────────────────────────────┘
                         │ N:N（透過 parentRefs）
┌────────────────────────▼────────────────────────────────┐
│  應用開發者 (App Developer)                               │
│                                                          │
│  HTTPRoute / GRPCRoute / TCPRoute / TLSRoute             │
│  ├── parentRefs[]   → Gateway (可跨 namespace)           │
│  ├── hostnames[]    → 主機名稱比對                        │
│  ├── rules[]        → 匹配條件 + 後端服務 + 過濾器         │
│  └── Status.Conditions  (Accepted / ResolvedRefs)        │
│                                                          │
│  ReferenceGrant  (namespaced)                            │
│  └── 授權其他 namespace 的資源引用本 namespace 的 Service  │
└─────────────────────────────────────────────────────────┘
```

### 核心資源一覽

| 資源 | API Group | 範疇 | 版本 | 說明 |
|------|-----------|------|------|------|
| `GatewayClass` | `gateway.networking.k8s.io` | cluster | v1 GA | controller 類型定義 |
| `Gateway` | `gateway.networking.k8s.io` | namespace | v1 GA | 流量入口點 |
| `HTTPRoute` | `gateway.networking.k8s.io` | namespace | v1 GA | HTTP/HTTPS 路由 |
| `GRPCRoute` | `gateway.networking.k8s.io` | namespace | v1 GA | gRPC 路由 |
| `TCPRoute` | `gateway.networking.k8s.io` | namespace | v1alpha2 | TCP 路由 |
| `TLSRoute` | `gateway.networking.k8s.io` | namespace | v1alpha2 | TLS passthrough |
| `ReferenceGrant` | `gateway.networking.k8s.io` | namespace | v1beta1 | 跨 namespace 授權 |

---

## 3. 整體架構

```
┌──────────────────────────────────────────────────────────────────┐
│  Synapse UI (React + Ant Design)                                  │
│                                                                    │
│  pages/network/                                                    │
│  ├── NetworkList.tsx          ← 加入 Gateway API 分頁 Tab          │
│  ├── GatewayClassList.tsx     ← 新增                              │
│  ├── GatewayList.tsx          ← 新增                              │
│  ├── GatewayDrawer.tsx        ← 新增（詳情 + Listeners + Routes） │
│  ├── HTTPRouteList.tsx        ← 新增                              │
│  ├── HTTPRouteDrawer.tsx      ← 新增（詳情 + Rules 展示）          │
│  ├── HTTPRouteForm.tsx        ← 新增（建立/編輯表單）              │
│  └── ReferenceGrantList.tsx   ← 新增（Phase 3）                   │
└──────────────────────┬───────────────────────────────────────────┘
                       │ REST API  /api/v1/clusters/:id/...
┌──────────────────────▼───────────────────────────────────────────┐
│  Synapse Backend (Go + Gin)                                        │
│                                                                    │
│  internal/handlers/gateway.go       ← 新增 GatewayHandler         │
│  internal/services/gateway_service.go ← 新增 GatewayService       │
│  internal/router/routes_cluster.go  ← 注入 gateway 路由組          │
└──────────────────────┬───────────────────────────────────────────┘
                       │ dynamic client (k8s.io/client-go/dynamic)
┌──────────────────────▼───────────────────────────────────────────┐
│  Kubernetes Cluster                                                │
│  ├── gateway.networking.k8s.io/v1  (CRD，需預先安裝)              │
│  ├── GatewayClass / Gateway / HTTPRoute / GRPCRoute               │
│  └── ReferenceGrant (v1beta1)                                      │
└──────────────────────────────────────────────────────────────────┘
```

### 與現有架構的整合點

| 整合點 | 說明 |
|--------|------|
| `K8sClient.GetRestConfig()` | 取得 `*rest.Config`，建立 `dynamic.Interface` |
| `K8sClient.GetClientset().Discovery()` | 用於偵測 Gateway API CRD 是否存在 |
| `handlers/mesh.go` 的 dynamic client 模式 | 直接複用，避免重新發明輪子 |
| `handlers/crd.go` 的 unstructured 處理模式 | 複用 `unstructured.Unstructured` 序列化邏輯 |
| 現有 `response` 工具函式 | 統一錯誤與成功回應格式 |
| `NetworkList.tsx` Tab 結構 | 加入 "Gateway API" Tab（已安裝才顯示） |

---

## 4. 後端設計

### 4.1 GVR 定義

```go
// internal/services/gateway_service.go

var (
    gatewayClassGVR = schema.GroupVersionResource{
        Group:    "gateway.networking.k8s.io",
        Version:  "v1",
        Resource: "gatewayclasses",
    }
    gatewayGVR = schema.GroupVersionResource{
        Group:    "gateway.networking.k8s.io",
        Version:  "v1",
        Resource: "gateways",
    }
    httpRouteGVR = schema.GroupVersionResource{
        Group:    "gateway.networking.k8s.io",
        Version:  "v1",
        Resource: "httproutes",
    }
    grpcRouteGVR = schema.GroupVersionResource{
        Group:    "gateway.networking.k8s.io",
        Version:  "v1",
        Resource: "grpcroutes",
    }
    referenceGrantGVR = schema.GroupVersionResource{
        Group:    "gateway.networking.k8s.io",
        Version:  "v1beta1",
        Resource: "referencegrants",
    }
)
```

### 4.2 GatewayService 結構

```go
type GatewayService struct {
    dynClient dynamic.Interface
    discovery discovery.DiscoveryInterface
}

func NewGatewayService(k8sClient *K8sClient) (*GatewayService, error) {
    dynClient, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
    if err != nil {
        return nil, err
    }
    return &GatewayService{
        dynClient: dynClient,
        discovery: k8sClient.GetClientset().Discovery(),
    }, nil
}
```

### 4.3 CRD 可用性偵測

在 Handler 初始化時或每次請求前呼叫，結果可快取 5 分鐘。

```go
// IsGatewayAPIAvailable 檢查 Gateway API CRD 是否已安裝
// 逐版本 fallback：v1 → v1beta1 → v1alpha2
func (s *GatewayService) IsGatewayAPIAvailable(ctx context.Context) (availableVersion string, ok bool) {
    for _, version := range []string{"v1", "v1beta1", "v1alpha2"} {
        group := "gateway.networking.k8s.io/" + version
        _, err := s.discovery.ServerResourcesForGroupVersion(group)
        if err == nil {
            return version, true
        }
    }
    return "", false
}
```

### 4.4 資源操作（以 HTTPRoute 為例）

```go
// ListHTTPRoutes 列出指定 namespace 的 HTTPRoute（空字串 = 全 namespace）
func (s *GatewayService) ListHTTPRoutes(ctx context.Context, namespace string) ([]HTTPRouteItem, error) {
    var list *unstructured.UnstructuredList
    var err error

    if namespace == "" {
        list, err = s.dynClient.Resource(httpRouteGVR).List(ctx, metav1.ListOptions{})
    } else {
        list, err = s.dynClient.Resource(httpRouteGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
    }
    if err != nil {
        return nil, err
    }

    result := make([]HTTPRouteItem, 0, len(list.Items))
    for _, item := range list.Items {
        result = append(result, toHTTPRouteItem(item))
    }
    return result, nil
}

// CreateHTTPRoute 建立 HTTPRoute
func (s *GatewayService) CreateHTTPRoute(ctx context.Context, namespace string, body map[string]interface{}) error {
    obj := &unstructured.Unstructured{Object: body}
    obj.SetAPIVersion("gateway.networking.k8s.io/v1")
    obj.SetKind("HTTPRoute")
    _, err := s.dynClient.Resource(httpRouteGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
    return err
}
```

### 4.5 GatewayHandler

```go
// internal/handlers/gateway.go

type GatewayHandler struct {
    db         *gorm.DB
    cfg        *config.Config
    clusterSvc *services.ClusterService
    k8sMgr     *services.K8sManager
}

// getGatewayService 取得 dynamic client，並確認 Gateway API 可用
func (h *GatewayHandler) getGatewayService(c *gin.Context) (*services.GatewayService, bool) {
    clusterID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
    k8sClient, err := h.k8sMgr.GetClient(uint(clusterID))
    if err != nil {
        response.NotFound(c, "叢集不存在或連線失敗")
        return nil, false
    }
    svc, err := services.NewGatewayService(k8sClient)
    if err != nil {
        response.InternalError(c, "建立 Gateway client 失敗: "+err.Error())
        return nil, false
    }
    _, ok := svc.IsGatewayAPIAvailable(c.Request.Context())
    if !ok {
        response.BadRequest(c, "此叢集尚未安裝 Gateway API CRD。請參考 https://gateway-api.sigs.k8s.io/guides/#installing-gateway-api")
        return nil, false
    }
    return svc, true
}
```

---

## 5. 前端設計

### 5.1 頁面結構（新增至 `pages/network/`）

```
pages/network/
├── NetworkList.tsx              ← 加入 "Gateway API" Tab（條件顯示）
├── GatewayAPITab.tsx            ← 新增：Gateway API 根 Tab
│   ├── GatewayClassList.tsx     ← GatewayClass 列表（唯讀）
│   ├── GatewayList.tsx          ← Gateway 列表
│   ├── GatewayDrawer.tsx        ← Gateway 詳情側邊欄
│   ├── HTTPRouteList.tsx        ← HTTPRoute 列表
│   ├── HTTPRouteDrawer.tsx      ← HTTPRoute 詳情側邊欄
│   └── HTTPRouteForm.tsx        ← 建立/編輯表單
├── gatewayTypes.ts              ← TypeScript 型別定義
└── gatewayService.ts            ← API 呼叫封裝
```

### 5.2 GatewayClass 列表（唯讀）

```
┌─────────────────────────────────────────────────────────────────┐
│  GatewayClass                                              [說明] │
├──────────────────┬───────────────────────────────┬──────────────┤
│  名稱             │  Controller                   │  接受狀態     │
├──────────────────┼───────────────────────────────┼──────────────┤
│  nginx           │  gateway.nginx.org/nginx       │  ● Accepted  │
│  istio           │  istio.io/gateway-controller   │  ● Accepted  │
│  contour         │  projectcontour.io/gateway-ctrl│  ○ Unknown   │
└──────────────────┴───────────────────────────────┴──────────────┘
```

### 5.3 Gateway 列表

```
┌───────────────────────────────────────────────────────────────────┐
│  Gateways             Namespace [all ▼]              [+ 建立]      │
├──────────┬────────────┬─────────────────┬──────────┬─────────────┤
│  名稱     │  Namespace │  GatewayClass   │  Listeners│  狀態       │
├──────────┼────────────┼─────────────────┼──────────┼─────────────┤
│  my-gw   │  infra     │  nginx          │  http:80  │  ● Ready    │
│          │            │                 │  https:443│             │
│  api-gw  │  api       │  istio          │  http:80  │  ● Ready    │
└──────────┴────────────┴─────────────────┴──────────┴─────────────┘
```

### 5.4 Gateway 詳情 Drawer

```
┌─────────────────────────────────────────────┐
│  my-gateway                          [YAML] │
│  Namespace: infra  GatewayClass: nginx      │
├─────────────────────────────────────────────┤
│  Conditions                                 │
│  ✓ Accepted   由 nginx controller 接受       │
│  ✓ Programmed 已完成程式設定                  │
├─────────────────────────────────────────────┤
│  Listeners (2)                              │
│  ┌──────┬──────┬──────────┬──────────────┐  │
│  │名稱   │ Port │ Protocol │ 狀態          │  │
│  ├──────┼──────┼──────────┼──────────────┤  │
│  │ http │  80  │ HTTP     │  ● Ready     │  │
│  │https │ 443  │ HTTPS    │  ● Ready     │  │
│  └──────┴──────┴──────────┴──────────────┘  │
├─────────────────────────────────────────────┤
│  外部地址                                    │
│  192.168.1.100 (IPAddress)                  │
├─────────────────────────────────────────────┤
│  關聯 HTTPRoutes (3)                         │
│  • shop-route  (namespace: shop)            │
│  • api-route   (namespace: api)             │
│  • admin-route (namespace: admin)           │
└─────────────────────────────────────────────┘
```

### 5.5 HTTPRoute 詳情 Drawer

```
┌─────────────────────────────────────────────┐
│  shop-route                          [YAML] │
│  Namespace: shop                            │
├─────────────────────────────────────────────┤
│  主機名稱                                    │
│  shop.example.com                           │
├─────────────────────────────────────────────┤
│  父 Gateway                                 │
│  infra/my-gateway  (listener: http)         │
│  Conditions: ✓ Accepted  ✓ ResolvedRefs     │
├─────────────────────────────────────────────┤
│  路由規則 (2)                                │
│                                             │
│  Rule 1                                     │
│    匹配: PathPrefix /api                    │
│    過濾: RequestHeaderModifier              │
│          Add: X-Request-ID: {uuid}          │
│    後端:  api-svc:3000  權重 100            │
│                                             │
│  Rule 2  （canary 流量分割）                 │
│    匹配: PathPrefix /                       │
│    後端:  shop-svc-v1:8080  權重 90         │
│           shop-svc-v2:8080  權重 10         │
└─────────────────────────────────────────────┘
```

### 5.6 HTTPRoute 建立表單

```
┌──────────────────────────────────────────────┐
│  建立 HTTPRoute                               │
├──────────────────────────────────────────────┤
│  名稱         [ shop-route              ]    │
│  Namespace    [ shop               ▼   ]    │
│  主機名稱      [ shop.example.com        ]   │
│               [+ 新增主機名稱]               │
├──────────────────────────────────────────────┤
│  父 Gateway                                  │
│  Namespace  [ infra         ▼ ]             │
│  Gateway    [ my-gateway    ▼ ]             │
│  Listener   [ http (port 80)▼ ]             │
│  [+ 新增父 Gateway]                          │
├──────────────────────────────────────────────┤
│  路由規則                                    │
│  ┌────────────────────────────────────────┐  │
│  │ Rule 1                    [- 刪除規則] │  │
│  │  匹配條件                              │  │
│  │  路徑: [PathPrefix ▼] [ /api        ] │  │
│  │  Header: [ 名稱 ] [Exact ▼] [ 值 ]   │  │
│  │  [+ 新增 Header 匹配]                  │  │
│  │                                        │  │
│  │  過濾器（可選）                         │  │
│  │  [RequestHeaderModifier ▼] [+ 新增]   │  │
│  │                                        │  │
│  │  後端服務                               │  │
│  │  [api-svc ▼] 埠 [3000] 權重 [100]     │  │
│  │  [+ 新增後端]                          │  │
│  └────────────────────────────────────────┘  │
│  [+ 新增規則]                                │
├──────────────────────────────────────────────┤
│               [取消]  [建立]                 │
└──────────────────────────────────────────────┘
```

### 5.7 未安裝 Gateway API 時的引導頁面

```
┌─────────────────────────────────────────────────────┐
│                                                      │
│  ⚠  此叢集尚未安裝 Gateway API                        │
│                                                      │
│  Gateway API 是 Kubernetes 官方的下一代流量管理標準，  │
│  取代 Ingress，提供更強大的路由、TLS 與跨 namespace    │
│  流量控制能力。                                       │
│                                                      │
│  安裝方式：                                          │
│  ┌──────────────────────────────────────────────┐   │
│  │ kubectl apply -f https://github.com/         │   │
│  │ kubernetes-sigs/gateway-api/releases/latest/ │   │
│  │ standard-install.yaml                        │   │
│  └──────────────────────────────────────────────┘   │
│                                                      │
│  [複製指令]   [查看官方文件]   [重新偵測]              │
│                                                      │
└─────────────────────────────────────────────────────┘
```

---

## 6. 資料模型與型別定義

### 6.1 Go 後端（Response DTOs）

```go
// internal/services/gateway_service.go

type GatewayClassItem struct {
    Name          string             `json:"name"`
    Controller    string             `json:"controller"`
    Description   string             `json:"description"`
    AcceptedStatus string            `json:"acceptedStatus"` // "Accepted" | "Unknown" | "InvalidParameters"
    CreatedAt     string             `json:"createdAt"`
}

type GatewayListener struct {
    Name     string `json:"name"`
    Port     int32  `json:"port"`
    Protocol string `json:"protocol"` // HTTP | HTTPS | TLS | TCP | UDP
    Hostname string `json:"hostname"`  // 可為空
    TLSMode  string `json:"tlsMode"`   // Terminate | Passthrough
    Status   string `json:"status"`    // Ready | Detached | Conflicted | ...
}

type GatewayAddress struct {
    Type  string `json:"type"`  // IPAddress | Hostname
    Value string `json:"value"`
}

type GatewayItem struct {
    Name         string            `json:"name"`
    Namespace    string            `json:"namespace"`
    GatewayClass string            `json:"gatewayClass"`
    Listeners    []GatewayListener `json:"listeners"`
    Addresses    []GatewayAddress  `json:"addresses"`
    Conditions   []K8sCondition    `json:"conditions"`
    RouteCount   int               `json:"routeCount"`
    CreatedAt    string            `json:"createdAt"`
}

type HTTPRouteRule struct {
    Matches  []map[string]interface{} `json:"matches"`   // 保留原始 unstructured
    Filters  []map[string]interface{} `json:"filters"`
    Backends []HTTPRouteBackend       `json:"backends"`
}

type HTTPRouteBackend struct {
    Name      string `json:"name"`
    Namespace string `json:"namespace"` // 可跨 namespace（需 ReferenceGrant）
    Port      int32  `json:"port"`
    Weight    int32  `json:"weight"`
}

type HTTPRouteParentStatus struct {
    GatewayNamespace string         `json:"gatewayNamespace"`
    GatewayName      string         `json:"gatewayName"`
    SectionName      string         `json:"sectionName"` // listener 名稱
    Conditions       []K8sCondition `json:"conditions"`
}

type HTTPRouteItem struct {
    Name        string                  `json:"name"`
    Namespace   string                  `json:"namespace"`
    Hostnames   []string                `json:"hostnames"`
    ParentRefs  []HTTPRouteParentStatus `json:"parentRefs"`
    Rules       []HTTPRouteRule         `json:"rules"`
    Conditions  []K8sCondition          `json:"conditions"`
    CreatedAt   string                  `json:"createdAt"`
}

type K8sCondition struct {
    Type    string `json:"type"`
    Status  string `json:"status"`  // "True" | "False" | "Unknown"
    Reason  string `json:"reason"`
    Message string `json:"message"`
}
```

### 6.2 TypeScript 前端型別（`gatewayTypes.ts`）

```typescript
// ui/src/pages/network/gatewayTypes.ts

export interface GatewayClassItem {
  name: string;
  controller: string;
  description: string;
  acceptedStatus: 'Accepted' | 'Unknown' | 'InvalidParameters';
  createdAt: string;
}

export interface GatewayListener {
  name: string;
  port: number;
  protocol: 'HTTP' | 'HTTPS' | 'TLS' | 'TCP' | 'UDP';
  hostname?: string;
  tlsMode?: 'Terminate' | 'Passthrough';
  status: string;
}

export interface GatewayAddress {
  type: 'IPAddress' | 'Hostname';
  value: string;
}

export interface K8sCondition {
  type: string;
  status: 'True' | 'False' | 'Unknown';
  reason: string;
  message: string;
}

export interface GatewayItem {
  name: string;
  namespace: string;
  gatewayClass: string;
  listeners: GatewayListener[];
  addresses: GatewayAddress[];
  conditions: K8sCondition[];
  routeCount: number;
  createdAt: string;
}

export interface HTTPRouteBackend {
  name: string;
  namespace?: string;
  port: number;
  weight: number;
}

export interface HTTPRouteRule {
  matches: Record<string, unknown>[];
  filters: Record<string, unknown>[];
  backends: HTTPRouteBackend[];
}

export interface HTTPRouteParentStatus {
  gatewayNamespace: string;
  gatewayName: string;
  sectionName: string;
  conditions: K8sCondition[];
}

export interface HTTPRouteItem {
  name: string;
  namespace: string;
  hostnames: string[];
  parentRefs: HTTPRouteParentStatus[];
  rules: HTTPRouteRule[];
  conditions: K8sCondition[];
  createdAt: string;
}

// 建立 HTTPRoute 的表單值
export interface HTTPRouteFormValues {
  name: string;
  namespace: string;
  hostnames: string[];
  parentRefs: Array<{
    gatewayNamespace: string;
    gatewayName: string;
    sectionName?: string;
  }>;
  rules: Array<{
    matches: Array<{
      pathType: 'Exact' | 'PathPrefix' | 'RegularExpression';
      pathValue: string;
      headers: Array<{ name: string; type: string; value: string }>;
    }>;
    backends: Array<{
      name: string;
      port: number;
      weight: number;
    }>;
  }>;
}
```

---

## 7. API 路由設計

在 `internal/router/routes_cluster.go` 中新增以下路由組（跟現有 `ingresses` 群組並列）：

```
// Gateway API 路由組
GET  /clusters/:id/gateway/status                     檢查 Gateway API 是否可用
GET  /clusters/:id/gatewayclasses                     列出 GatewayClass（cluster-scoped）
GET  /clusters/:id/gatewayclasses/:name               取得 GatewayClass 詳情

GET  /clusters/:id/gateways                           列出全 namespace Gateway
GET  /clusters/:id/gateways/:namespace/:name          取得 Gateway 詳情
GET  /clusters/:id/gateways/:namespace/:name/yaml     取得 Gateway YAML
POST /clusters/:id/gateways                           建立 Gateway
PUT  /clusters/:id/gateways/:namespace/:name          更新 Gateway
DELETE /clusters/:id/gateways/:namespace/:name        刪除 Gateway

GET  /clusters/:id/httproutes                         列出全 namespace HTTPRoute
GET  /clusters/:id/httproutes/:namespace/:name        取得 HTTPRoute 詳情
GET  /clusters/:id/httproutes/:namespace/:name/yaml   取得 HTTPRoute YAML
POST /clusters/:id/httproutes                         建立 HTTPRoute
PUT  /clusters/:id/httproutes/:namespace/:name        更新 HTTPRoute
DELETE /clusters/:id/httproutes/:namespace/:name      刪除 HTTPRoute

GET  /clusters/:id/grpcroutes                         列出 GRPCRoute（Phase 3）
GET  /clusters/:id/referencegrants                    列出 ReferenceGrant（Phase 3）
```

---

## 8. 版本相容策略

Gateway API 在不同版本的 Kubernetes 叢集上有不同的成熟度：

| 叢集版本 | 可用 Gateway API 版本 | 策略 |
|----------|----------------------|------|
| K8s ≥ 1.28 + 安裝 Standard CRD | v1 (GA) | 優先使用 |
| K8s ≥ 1.24 + 安裝 Experimental CRD | v1beta1 | Fallback |
| 未安裝 Gateway API CRD | — | 顯示安裝引導頁 |

### 版本探測流程

```
1. 呼叫 GET /clusters/:id/gateway/status
2. 後端依序嘗試：v1 → v1beta1 → v1alpha2
3. 回傳 { available: true, version: "v1" } 或 { available: false }
4. 前端根據回傳決定：
   - available=true  → 正常顯示 Gateway API Tab
   - available=false → 顯示安裝引導 UI
```

### GVR 動態切換（後端）

```go
func (s *GatewayService) resolveGVR(base schema.GroupVersionResource) schema.GroupVersionResource {
    for _, v := range []string{"v1", "v1beta1", "v1alpha2"} {
        candidate := schema.GroupVersionResource{
            Group:    base.Group,
            Version:  v,
            Resource: base.Resource,
        }
        _, err := s.discovery.ServerResourcesForGroupVersion(candidate.Group + "/" + v)
        if err == nil {
            return candidate
        }
    }
    return base // 維持原版本，讓呼叫方處理錯誤
}
```

---

## 9. 錯誤處理與邊界情況

| 情境 | 後端回應 | 前端處理 |
|------|----------|----------|
| Gateway API CRD 未安裝 | `400 此叢集尚未安裝 Gateway API CRD` | 顯示安裝引導 UI |
| Route 引用不存在的 Gateway | 回傳 Route，`conditions` 中含 `Accepted=False` | 用橘色警示顯示 condition |
| 跨 namespace 引用缺少 ReferenceGrant | `conditions.ResolvedRefs=False, reason=RefNotPermitted` | 顯示 ReferenceGrant 提示 |
| controller 尚未處理（Programmed=False） | 回傳 conditions | 以灰色「Pending」狀態顯示 |
| 刪除有 Route 綁定的 Gateway | 僅刪除 Gateway，Route 自動 `Accepted=False` | 刪除前 Warning dialog 提示關聯 Routes |
| `kubectl config view` 匯出的 REDACTED kubeconfig | `400 憑證資料被遮蔽` | （已在 `k8s_client.go` 處理） |

---

## 10. RBAC 權限設計

### 最小所需 ClusterRole（唯讀）

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: synapse-gateway-viewer
rules:
  - apiGroups: ["gateway.networking.k8s.io"]
    resources:
      - gatewayclasses
      - gateways
      - httproutes
      - grpcroutes
      - tcproutes
      - tlsroutes
      - referencegrants
    verbs: ["get", "list", "watch"]
```

### 完整 CRUD（編輯者）

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: synapse-gateway-editor
rules:
  - apiGroups: ["gateway.networking.k8s.io"]
    resources:
      - gateways
      - httproutes
      - grpcroutes
      - referencegrants
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["gateway.networking.k8s.io"]
    resources: ["gatewayclasses"]
    verbs: ["get", "list", "watch"]  # GatewayClass 僅 Infra Admin 可寫
```

### Synapse 現有 RBAC 模板整合

在 `internal/templates/rbac/clusterroles.go` 中，將上述兩個 ClusterRole 加入
現有的「viewer」和「editor」範本，確保叢集接入時可選擇是否安裝 Gateway API 權限。

---

## 11. 實作路線圖

### Phase 1：唯讀展示 ✅ 已完成（2026-04-07）

- [x] `internal/services/gateway_service.go`：GVR 定義 + CRD 偵測 + List/Get 方法
- [x] `internal/handlers/gateway.go`：Handler（GET 系列）
- [x] 注入路由至 `routes_cluster.go`
- [x] `ui/src/pages/network/gatewayTypes.ts`：型別定義
- [x] `ui/src/services/gatewayService.ts`：API 呼叫
- [x] `GatewayClassList.tsx`：列表頁（唯讀）
- [x] `GatewayList.tsx` + `GatewayDrawer.tsx`：列表 + 詳情
- [x] `HTTPRouteList.tsx` + `HTTPRouteDrawer.tsx`：列表 + 詳情
- [x] `GatewayAPITab.tsx`：根 Tab，含未安裝引導 UI
- [x] `NetworkList.tsx`：加入 Gateway API Tab
- [x] i18n：zh-TW / zh-CN / en-US 三語系

### Phase 2：基本 CRUD ✅ 已完成（2026-04-07）

- [x] `gateway_service.go` 補充 Create/Update/Delete 方法
- [x] Handler 補充 POST / PUT / DELETE
- [x] `GatewayForm.tsx`：Gateway 建立/編輯表單（Form + YAML 雙模式，Monaco editor）
- [x] `HTTPRouteForm.tsx`：HTTPRoute 建立/編輯表單（含動態 rules builder）
- [x] YAML 直編（複用現有 Monaco editor 機制）
- [x] 刪除確認 dialog（modal.confirm）
- [x] `GatewayList.tsx` / `HTTPRouteList.tsx`：整合建立/編輯/刪除按鈕

### Phase 3：進階功能

- [ ] `GRPCRoute` 支援（列表 + 詳情 + CRUD）
- [ ] `ReferenceGrant` 管理頁面
- [ ] 流量分割視覺化（canary 百分比圓餅圖 / 進度條）
- [ ] RBAC 範本整合（`clusterroles.go`）
- [ ] Gateway 拓撲圖（GatewayClass → Gateway → Routes → Services）

---

## 12. 技術選型決策

| 決策 | 選擇 | 理由 |
|------|------|------|
| K8s client | `k8s.io/client-go/dynamic` | Gateway API 是 CRD，無 typed clientset；與現有 `mesh.go` / `crd.go` 一致 |
| 型別定義 | `unstructured.Unstructured` + 自定義 DTO | 避免引入 `sigs.k8s.io/gateway-api` SDK（會拉入大量依賴），手動解析關鍵欄位即可 |
| 版本策略 | 執行時探測（v1 → v1beta1 fallback） | 支援舊叢集，不在 build time 鎖定版本 |
| 前端表單 | 動態陣列表單（Ant Design Form.List） | Rules 為巢狀陣列，需動態增刪，Form.List 最適合 |
| YAML 直編 | 複用現有 Monaco editor + YAML apply 模式 | 保持一致性，進階用戶可繞過表單 |
| CRD 偵測快取 | 每個請求即時呼叫 Discovery API | 先不快取，未來視效能需求加入 5 分鐘 in-memory cache |
