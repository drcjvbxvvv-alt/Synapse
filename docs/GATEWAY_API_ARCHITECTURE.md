# Synapse Gateway API 架構設計文件

> 版本：v1.4 | 日期：2026-04-08 | 狀態：Phase 1、Phase 2 & Phase 3 已實作；Phase 4 規劃中
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
13. [Phase 4：叢集網路拓撲圖（規劃中）](#13-phase-4叢集網路拓撲圖規劃中)

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

### Phase 3：進階功能 ✅ 已完成（2026-04-07）

- [x] `GRPCRoute` 支援（列表 + YAML 直編 CRUD，Monaco editor）
- [x] `ReferenceGrant` 管理頁面（列表 + 建立 + 刪除 + YAML 檢視）
- [x] 流量分割視覺化（多 backend 時顯示比例色條 + 百分比圖例）
- [x] RBAC 範本整合（Ops/Dev 寫入權限；Readonly 唯讀；Admin 已有 `*`）
- [x] Gateway 拓撲圖（@xyflow/react + dagre 自動佈局，GatewayClass → Gateway → Routes → Services）

**新增 GVR**：`GRPCRouteGVR`（v1）、`ReferenceGrantGVR`（v1beta1）

**新增後端**：
- `gateway_service.go`：`ListGRPCRoutes`、`GetGRPCRoute`、`GetGRPCRouteYAML`、CRUD；`ListReferenceGrants`、`GetReferenceGrantYAML`、`CreateReferenceGrant`、`DeleteReferenceGrant`；`GetTopology`
- `gateway.go`：對應 handler 方法
- `routes_cluster.go`：`/grpcroutes`、`/referencegrants`、`/gateway/topology` 路由

**新增前端**：
- `GRPCRouteList.tsx`：列表 + 行內 YAML 編輯器 Modal
- `ReferenceGrantList.tsx`：列表 + 建立 Modal + YAML 檢視
- `GatewayTopology.tsx`：React Flow 拓撲圖，dagre LR 佈局
- `HTTPRouteDrawer.tsx`：多 backend 時加入流量分割進度條

**RBAC**：
- `synapse-ops`：Gateway API 全資源 CRUD
- `synapse-dev`：Gateway/Route CRUD，GatewayClass 唯讀
- `synapse-readonly`：Gateway API 全資源唯讀

---

---

## 13. Phase 4：叢集網路拓撲圖（規劃中）

> 狀態：📋 規劃中 | 預計版本：v1.5

### 13.1 背景與目標

Gateway API 拓撲圖（Phase 3）僅呈現 **入口流量鏈路**（GatewayClass → Gateway → Route → Service）。
Phase 4 目標是擴展為 **全叢集網路視圖**，讓操作者能在單一畫面觀察：

- Pod 之間的通信關係與連線健康狀態
- Service 與其背後 Pod（Endpoint）的就緒情況
- NetworkPolicy 對流量的允許 / 封鎖效果
- 外部流量（Ingress / Gateway）的完整入口鏈路

---

### 13.2 資料來源與限制分析

Kubernetes 原生 **不追蹤即時 Pod-to-Pod 連線**，需按可用資料分層：

| 層次 | K8s 資源 | 能提供的資訊 | 永遠可用 |
|------|---------|------------|---------|
| 靜態拓撲 | Pods + Services + EndpointSlices | Service ↔ Pod 對應、Pod 健康 | ✅ |
| 策略層 | NetworkPolicy | 靜態推論允許 / 封鎖路徑 | ✅（需裝 NetworkPolicy） |
| 即時流量 | Istio Envoy sidecar | 真實連線數、延遲、錯誤率 | ⚠️（需 Istio） |
| 即時流量 | Cilium Hubble API | 核心層連線追蹤 | ⚠️（需 Cilium CNI） |

Phase 4 主線（Phase A）僅依賴 **Kubernetes 原生 API**，不要求任何 CNI/mesh。
進階功能（Phase C/D）在偵測到 Istio/Cilium 後自動啟用。

---

### 13.3 視覺設計

#### 節點類型

```
┌──────────────────────────────────────────────────────┐
│  namespace: frontend  （群組框）                      │
│                                                       │
│   ╭──────────╮   ╭──────────╮                       │
│   │  Pod     │   │  Pod     │  ● Running（綠）       │
│   │ web-1 ●  │   │ web-2 ●  │  ◑ Pending（橙）      │
│   ╰────┬─────╯   ╰────┬─────╯  ✕ Failed（紅）       │
│        └──────┬────────┘                              │
│         ╔═════╧══════╗                               │
│         ║ Service    ║  ← 雙線框                     │
│         ║  web-svc   ║                               │
│         ╚═════╤══════╝                               │
│               │                                       │
│    [Ingress / Gateway]  ← 菱形入口節點               │
└───────────────┼──────────────────────────────────────┘
                │  NetworkPolicy: DENY（紅色✕線）
┌───────────────┼──────────────────────────────────────┐
│  namespace: backend                                   │
│         ╔═════╧══════╗                               │
│         ║ Service    ║                               │
│         ║  api-svc   ║                               │
│         ╚═════╤══════╝                               │
│   ╭──────────╮│                                      │
│   │  Pod     ├╯                                      │
│   │ api-1 ●  │                                       │
│   ╰──────────╯                                       │
└──────────────────────────────────────────────────────┘
```

#### 節點樣式規格

| 節點類型 | 形狀 | 色彩 | 說明 |
|---------|------|------|------|
| Namespace | 群組背景框 | 淡灰邊框 | 內含所屬資源 |
| Pod（Running） | 圓形 | 綠色 `#52c41a` | 外圈脈衝動畫 |
| Pod（Pending） | 圓形 | 橙色 `#fa8c16` | 外圈緩慢閃爍 |
| Pod（Failed） | 圓形 | 紅色 `#ff4d4f` | 靜止無動畫 |
| Service | 圓角矩形 | 藍色 `#1677ff` | 顯示 ClusterIP |
| Ingress/Gateway | 菱形 / 六角形 | 紫色 `#722ed1` | 入口節點 |

---

### 13.4 邊（Edge）動態動畫設計

#### 邊類型與動畫規則

| 邊類型 | 觸發條件 | 視覺效果 |
|--------|---------|---------|
| **就緒連線** | Service → Ready Pod | 綠色粒子流動（速度正常） |
| **降級連線** | Service → NotReady Pod | 橙色虛線慢速流動 |
| **無端點** | Service 無任何 Endpoint | 灰色靜止虛線 |
| **Policy Allow** | NetworkPolicy 顯式允許 | 藍色實線（可切換顯示） |
| **Policy Deny** | NetworkPolicy 封鎖 | 紅色靜止線 + ✕ 中間標記 |
| **入口流量** | Ingress/Gateway → Service | 紫色粒子流動（由外向內） |
| **即時高流量** | Istio metrics > 閾值 | 粒子加速 + 線條加粗 |
| **即時錯誤** | Istio error rate > 5% | 紅色粒子 + 邊標籤顯示錯誤率 |

#### 粒子流動實作（自訂 Edge 元件）

使用 SVG `animateMotion` 沿 React Flow 的邊路徑運動，避免引入額外動畫庫：

```tsx
// CustomParticleEdge.tsx（概念）
const CustomParticleEdge = ({ id, sourceX, sourceY, targetX, targetY, data }) => {
  const [edgePath] = getSmoothStepPath({ sourceX, sourceY, targetX, targetY });
  const particleCount = data.traffic === 'high' ? 3 : 1;
  const duration = data.status === 'degraded' ? '3s' : '1.5s';
  const color = { ready: '#52c41a', degraded: '#fa8c16', denied: '#ff4d4f' }[data.status];

  return (
    <g>
      <path id={id} d={edgePath} stroke={color} strokeWidth={1.5} fill="none" />
      {Array.from({ length: particleCount }).map((_, i) => (
        <circle key={i} r={3} fill={color} opacity={0.8}>
          <animateMotion
            dur={duration}
            repeatCount="indefinite"
            begin={`${(i / particleCount) * parseFloat(duration)}s`}
          >
            <mpath href={`#${id}`} />
          </animateMotion>
        </circle>
      ))}
    </g>
  );
};
```

#### 脈衝節點動畫（Pod Running）

```css
/* Pod Running 外圈脈衝 */
@keyframes pod-pulse {
  0%   { r: 18px; opacity: 0.6; }
  100% { r: 28px; opacity: 0; }
}
.pod-running-ring {
  animation: pod-pulse 2s ease-out infinite;
}
```

---

### 13.5 規模問題與解決方案

大叢集可能有數百到數千個 Pod，不加限制會導致畫面無法使用。

#### 強制過濾策略

1. **Namespace 多選**（最多 5 個）：避免全叢集一次載入
2. **Pod 摺疊（Rollup）模式**：同一 `ownerReference`（ReplicaSet/StatefulSet）的 Pod 預設折疊為一個「工作負載節點」，顯示健康比例；展開才顯示個別 Pod
3. **節點數量限制**：超過 200 個節點時顯示警告，建議縮小 namespace 範圍

#### 摺疊節點設計

```
┌────────────────────────────┐
│ Deployment: web            │  ← 摺疊視圖
│ ● 3/3 Running              │
│ [展開查看 Pod]              │
└────────────────────────────┘

展開後：
┌───────────────────────────────────────┐
│ ●web-abc12  ●web-def34  ●web-ghi56   │
└───────────────────────────────────────┘
```

---

### 13.6 後端 API 設計

#### 新增 endpoint

```
GET /api/v1/clusters/:clusterID/network/topology
    ?namespaces=frontend,backend   // 必填，逗號分隔，最多 5 個
    &rollup=true                   // 是否摺疊同 owner 的 Pod（預設 true）
    &includeGateway=true           // 是否包含 Gateway API 節點
    &includeIngress=true           // 是否包含 Ingress 節點
    &includePolicy=true            // 是否包含 NetworkPolicy 邊
```

#### Response DTO

```go
type ClusterNetworkTopology struct {
    Namespaces []string             `json:"namespaces"`
    Nodes      []NetworkNode        `json:"nodes"`
    Edges      []NetworkEdge        `json:"edges"`
    Stats      NetworkTopologyStats `json:"stats"`
}

type NetworkNode struct {
    ID        string            `json:"id"`
    Kind      string            `json:"kind"`      // Pod | Workload | Service | Namespace | Ingress | Gateway
    Name      string            `json:"name"`
    Namespace string            `json:"namespace"`
    Status    string            `json:"status"`    // Running | Pending | Failed | Ready | NotReady
    // Pod-specific
    PodIP     string            `json:"podIP,omitempty"`
    OwnerKind string            `json:"ownerKind,omitempty"` // Deployment | StatefulSet | DaemonSet
    OwnerName string            `json:"ownerName,omitempty"`
    // Workload rollup
    ReadyCount int              `json:"readyCount,omitempty"`
    TotalCount int              `json:"totalCount,omitempty"`
    // Service-specific
    ClusterIP string            `json:"clusterIP,omitempty"`
    Labels    map[string]string `json:"labels,omitempty"`
}

type NetworkEdge struct {
    Source   string `json:"source"`
    Target   string `json:"target"`
    Kind     string `json:"kind"`   // endpoint | policy-allow | policy-deny | ingress | gateway
    Status   string `json:"status"` // ready | degraded | blocked
    // Policy-specific
    PolicyName      string `json:"policyName,omitempty"`
    PolicyNamespace string `json:"policyNamespace,omitempty"`
}

type NetworkTopologyStats struct {
    TotalPods       int `json:"totalPods"`
    RunningPods     int `json:"runningPods"`
    TotalServices   int `json:"totalServices"`
    TotalPolicies   int `json:"totalPolicies"`
}
```

#### 後端組裝邏輯

```
1. 並行拉取（goroutine）：
   - k8s.ListPods(namespaces)
   - k8s.ListServices(namespaces)
   - k8s.ListEndpointSlices(namespaces)
   - k8s.ListNetworkPolicies(namespaces)
   - [可選] ListIngresses / ListGateways

2. 建立 Service → Pod 對應：
   - EndpointSlice.endpoints[].targetRef.name → Pod
   - 標記 conditions[].ready = true/false

3. NetworkPolicy 靜態推論：
   - 針對每對 (source Pod, target Service)，
     遍歷 NetworkPolicy 的 ingress/egress selectors，
     判斷是否有明確 allow 或 deny（無 NetworkPolicy = allow all）

4. rollup=true 時：
   - 按 ownerReference 分組 Pod
   - 輸出 kind=Workload 節點，附 readyCount/totalCount
   - 邊改為 Service → Workload

5. 回傳 ClusterNetworkTopology
```

---

### 13.7 前端元件設計

#### 元件樹

```
NetworkTopologyPage.tsx          ← 獨立頁面（Network tab 新增入口）
  ├── TopologyFilterBar.tsx      ← Namespace 多選、圖層切換、Rollup 開關
  ├── NetworkTopologyGraph.tsx   ← React Flow 畫布主體
  │     ├── NamespaceGroupNode   ← 自訂 Node：namespace 群組背景
  │     ├── WorkloadNode         ← 自訂 Node：Deployment/StatefulSet 摺疊
  │     ├── PodNode              ← 自訂 Node：單一 Pod（含脈衝動畫）
  │     ├── ServiceNode          ← 自訂 Node：Service
  │     ├── IngressGatewayNode   ← 自訂 Node：入口節點
  │     └── ParticleEdge         ← 自訂 Edge：粒子流動動畫
  └── NodeDetailPanel.tsx        ← 點擊節點後顯示詳情 Drawer
```

#### 圖層切換（Filter Bar）

```
[Namespace: frontend ✕] [backend ✕]  [+ 新增]
────────────────────────────────────────────
圖層：[✓ Services] [✓ Pods] [✓ Policies] [✓ Ingress/GW]
模式：[● 摺疊工作負載]  [○ 展開所有 Pod]
[⟳ 自動刷新：15s ▾]
```

#### 自動刷新策略

- 預設每 **15 秒**輪詢一次 `/network/topology`
- 比對前後 diff（新增/移除/狀態變更節點/邊）
- 狀態變更的節點閃爍 highlight 0.5 秒
- 使用者拖動節點後暫停自動刷新，顯示「已暫停，點此恢復」

---

### 13.8 NetworkPolicy 靜態推論演算法

NetworkPolicy 沒有「Deny」資源，邏輯為：
- **有 NetworkPolicy 選中的 Pod** → 預設拒絕所有，只允許規則中明確放行的流量
- **無 NetworkPolicy 選中的 Pod** → 允許所有流量

推論步驟（後端）：

```
for each (sourcePod, targetService):
  targetPods = endpointSlice[targetService].pods
  for each targetPod in targetPods:
    ingressPolicies = NetworkPolicies where podSelector matches targetPod
    if len(ingressPolicies) == 0:
      edge.status = "allow-all"  // 無 policy，全通
    else:
      allowed = false
      for policy in ingressPolicies:
        if policy.ingress rules match sourcePod:
          allowed = true; break
      edge.status = allowed ? "policy-allow" : "policy-deny"
```

---

### 13.9 進階整合（Phase C/D，條件式啟用）

#### Istio 整合（若已安裝）

偵測方式：呼叫 Istio 的 `/api/v1/namespaces/istio-system/pods`，存在即啟用。

新增資料來源：Prometheus（Istio 預設 metrics）
```
istio_request_total{...}          → 連線數
istio_request_duration_milliseconds_p99{...} → 延遲 P99
istio_request_total{response_code=~"5.."}    → 錯誤率
```

EdgeData 擴展：
```go
type NetworkEdge struct {
    // ...（原有欄位）
    RequestRate  float64 `json:"requestRate,omitempty"`   // req/s
    LatencyP99   float64 `json:"latencyP99Ms,omitempty"`  // ms
    ErrorRate    float64 `json:"errorRate,omitempty"`     // 0.0-1.0
}
```

粒子動畫對應：
- `requestRate` → 粒子數量（1-5 顆）
- `latencyP99` > 500ms → 粒子顏色轉橙色
- `errorRate` > 5% → 粒子顏色轉紅色，邊標籤顯示錯誤率

#### Cilium Hubble 整合（若已安裝）

偵測方式：`hubble-relay` service 是否存在於 `kube-system`。

Hubble 提供真實連線追蹤，無需推論：
- 直接回傳已觀察到的流量對（source, destination, verdict: forwarded/dropped）
- 可顯示每條邊的「封包數 / 丟包率」

---

### 13.10 實作分階段計畫

| Phase | 功能 | 後端工作 | 前端工作 | 依賴 |
|-------|------|---------|---------|------|
| **A**（主線） | Service + Endpoints + Pod 靜態拓撲 + 粒子動畫 | `network_topology_service.go`、`/network/topology` endpoint | `NetworkTopologyGraph.tsx`、`ParticleEdge`、`PodNode`、`ServiceNode` | 無（K8s 原生） |
| **A+** | Namespace 群組框 + Pod 摺疊（Workload rollup）| 後端 rollup 邏輯 | `NamespaceGroupNode`、`WorkloadNode`、展開互動 | Phase A |
| **B** | NetworkPolicy 靜態推論覆蓋層 | NetworkPolicy 推論演算法（後端） | Policy 邊顯示、圖層開關 | Phase A（已有 NetworkPolicy API） |
| **C** | Istio 真實流量指標 | Prometheus 查詢封裝 | 粒子速度/顏色動態對應 | Istio 已安裝 |
| **D** | Cilium Hubble 連線追蹤 | Hubble relay API 呼叫 | 封包數/丟包率顯示 | Cilium 已安裝 |

#### Phase A 具體檔案清單

**後端**
```
internal/services/network_topology_service.go    ← 新增
internal/handlers/network_topology.go            ← 新增
internal/router/routes_cluster.go                ← 新增 1 條路由
```

**前端**
```
ui/src/pages/network/NetworkTopologyPage.tsx     ← 新增
ui/src/pages/network/NetworkTopologyGraph.tsx    ← 新增（React Flow 畫布）
ui/src/pages/network/nodes/PodNode.tsx           ← 新增
ui/src/pages/network/nodes/ServiceNode.tsx       ← 新增
ui/src/pages/network/nodes/WorkloadNode.tsx      ← 新增
ui/src/pages/network/nodes/NamespaceGroupNode.tsx ← 新增
ui/src/pages/network/edges/ParticleEdge.tsx      ← 新增（SVG animateMotion）
ui/src/pages/network/TopologyFilterBar.tsx       ← 新增
ui/src/pages/network/NodeDetailPanel.tsx         ← 新增
ui/src/services/networkTopologyService.ts        ← 新增
ui/src/pages/network/NetworkList.tsx             ← 修改（新增入口 tab）
ui/src/locales/*/network.json                    ← 新增 i18n 鍵值
```

---

### 13.11 技術選型決策（Phase 4）

| 決策 | 選擇 | 理由 |
|------|------|------|
| 節點/邊渲染 | `@xyflow/react`（已安裝 v12） | Phase 3 已驗證，自訂 Node/Edge 彈性高 |
| 佈局演算法 | `@dagrejs/dagre`（已安裝） | 有向圖層次佈局，適合 namespace → service → pod |
| 粒子動畫 | 純 SVG `animateMotion` | 無額外依賴，CPU 消耗低於 JS 動畫 |
| 輪詢 vs WebSocket | 輪詢（15s interval） | 避免 long-lived 連線複雜度；K8s watch API 可在後續版本替換 |
| NetworkPolicy 推論 | 後端靜態分析 | 前端不做 Pod selector 比對邏輯，保持輕量 |
| Istio/Cilium 偵測 | 偵測特定 Service/Pod 存在 | 與 Phase 3 Gateway API 偵測模式一致 |
| 大叢集規模限制 | namespace 過濾 + rollup 模式 | 避免單次回傳數千節點造成瀏覽器卡頓 |

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
