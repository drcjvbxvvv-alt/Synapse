# 細粒度功能權限管理 — 設計規劃書

> 版本：v1.1 | 建立日期：2026-04-12
> 狀態：**全部實作完成** ✅

---

## 目錄

1. [背景與目標](#1-背景與目標)
2. [核心設計原則](#2-核心設計原則)
3. [功能鍵值分類表](#3-功能鍵值分類表)
4. [權限上限規則（Ceiling Rule）](#4-權限上限規則ceiling-rule)
5. [資料庫設計](#5-資料庫設計)
6. [後端 API 設計](#6-後端-api-設計)
7. [前端 PermissionContext 變更](#7-前端-permissioncontext-變更)
8. [前端新頁面設計](#8-前端新頁面設計)
9. [現有元件更新計劃](#9-現有元件更新計劃)
10. [導航與路由](#10-導航與路由)
11. [實作順序](#11-實作順序)
12. [未解決問題（待決策）](#12-未解決問題待決策)

---

## 1. 背景與目標

### 現況

目前 Synapse 的存取控制採用粗粒度的「類型-命名空間」模型：

```
permission_type: admin | ops | dev | readonly | custom
namespaces: ["*"] | ["ns-a", "ns-b"]
```

這套系統可以控制「哪個使用者能進入哪個叢集的哪些命名空間」，但無法針對特定功能做限制。例如：

- 無法讓某個 `ops` 使用者**無法使用 AI 助手**
- 無法讓某個 `dev` 使用者**禁止開啟終端機**
- 無法讓某個 `dev` 使用者**禁止匯出資料**

### 目標

新增一層「**功能開關（Feature Policy）**」，讓平台管理員能針對每個 `ClusterPermission` 記錄額外設定允許/禁止特定功能，實現細粒度控制。

---

## 2. 核心設計原則

### 原則一：Feature Policy 只能「收緊」，不能「放寬」

Feature Policy 是對基礎權限類型的**額外限制層**，永遠不能讓使用者取得超出其 `permission_type` 所能給予的上限。

```
最終有效權限 = permission_type 基礎能力 ∩ feature_policy 允許集合
```

舉例：

- `readonly` 使用者的 `export` 功能預設關閉（readonly 本就不允許匯出）
- `dev` 使用者的 `pod:terminal` 預設開啟，但可被 feature policy 收緊至關閉
- `dev` 使用者即使 feature policy 允許 `node:terminal`，仍然無效（dev 基礎類型上限不含 node 終端）

### 原則二：預設行為與現況一致（向後相容）

若某個 `ClusterPermission` 沒有設定 `feature_policy`（欄位為 NULL），則等同於「採用該 permission_type 的預設開啟集合」，**不影響現有使用者體驗**。

### 原則三：平台管理員專屬操作

Feature Policy 的配置入口只對 **Platform Admin**（`system_role = platform_admin`）開放，叢集管理員無法修改。

---

## 3. 功能鍵值分類表

以下為 feature policy 中使用的功能鍵值（`feature_key`）。

### 3.1 工作負載類

| Feature Key      | 說明                   | admin | ops | dev | readonly |
| ---------------- | ---------------------- | :---: | :-: | :-: | :------: |
| `workload:view`  | 查看工作負載列表/詳情  |   ✓   |  ✓  |  ✓  |    ✓     |
| `workload:write` | 建立/編輯/刪除工作負載 |   ✓   |  ✓  |  ✓  |    ✗     |

### 3.2 網路類

| Feature Key     | 說明                   | admin | ops | dev | readonly |
| --------------- | ---------------------- | :---: | :-: | :-: | :------: |
| `network:view`  | 查看網路資源           |   ✓   |  ✓  |  ✓  |    ✓     |
| `network:write` | 建立/編輯/刪除網路資源 |   ✓   |  ✓  |  ✓  |    ✗     |

### 3.3 儲存類

| Feature Key     | 說明                   | admin | ops | dev | readonly |
| --------------- | ---------------------- | :---: | :-: | :-: | :------: |
| `storage:view`  | 查看儲存資源           |   ✓   |  ✓  |  ✓  |    ✓     |
| `storage:write` | 建立/編輯/刪除儲存資源 |   ✓   |  ✓  |  ✗  |    ✗     |

### 3.4 節點類

| Feature Key   | 說明                      | admin | ops | dev | readonly |
| ------------- | ------------------------- | :---: | :-: | :-: | :------: |
| `node:view`   | 查看節點列表/詳情         |   ✓   |  ✓  |  ✗  |    ✓     |
| `node:manage` | Cordon / Uncordon / Drain |   ✓   |  ✗  |  ✗  |    ✗     |

### 3.5 設定類

| Feature Key    | 說明                    | admin | ops | dev | readonly |
| -------------- | ----------------------- | :---: | :-: | :-: | :------: |
| `config:view`  | 查看 ConfigMap / Secret |   ✓   |  ✓  |  ✓  |    ✓     |
| `config:write` | 編輯 ConfigMap / Secret |   ✓   |  ✓  |  ✓  |    ✗     |

### 3.6 終端機類

| Feature Key     | 說明             | admin | ops | dev | readonly |
| --------------- | ---------------- | :---: | :-: | :-: | :------: |
| `terminal:pod`  | 進入 Pod 終端機  |   ✓   |  ✓  |  ✓  |    ✗     |
| `terminal:node` | 進入 Node 終端機 |   ✓   |  ✓  |  ✗  |    ✗     |

### 3.7 可觀測性類

| Feature Key       | 說明         | admin | ops | dev | readonly |
| ----------------- | ------------ | :---: | :-: | :-: | :------: |
| `logs:view`       | 查看容器 Log |   ✓   |  ✓  |  ✓  |    ✓     |
| `monitoring:view` | 查看監控圖表 |   ✓   |  ✓  |  ✓  |    ✓     |

### 3.8 Helm 類

| Feature Key  | 說明                     | admin | ops | dev | readonly |
| ------------ | ------------------------ | :---: | :-: | :-: | :------: |
| `helm:view`  | 查看 Helm 應用           |   ✓   |  ✓  |  ✗  |    ✓     |
| `helm:write` | 安裝/更新/刪除 Helm 應用 |   ✓   |  ✓  |  ✗  |    ✗     |

### 3.9 工具類

| Feature Key    | 說明                     | admin | ops | dev | readonly |
| -------------- | ------------------------ | :---: | :-: | :-: | :------: |
| `export`       | 匯出資源清單（CSV/JSON） |   ✓   |  ✓  |  ✓  |    ✗     |
| `ai_assistant` | 使用 AI 助手             |   ✓   |  ✓  |  ✓  |    ✗     |

> **✓ = 基礎類型預設允許，可被 feature policy 收緊**
> **✗ = 基礎類型上限不允許，feature policy 無法開啟**

---

## 4. 權限上限規則（Ceiling Rule）

### 各 permission_type 的上限集合

以下定義每個基礎類型「最多能允許哪些 feature keys」（即 ceiling）。

```go
// internal/models/feature_policy.go（新建）

var FeatureCeilings = map[string][]string{
    "admin": {
        "workload:view", "workload:write",
        "network:view", "network:write",
        "storage:view", "storage:write",
        "node:view", "node:manage",
        "config:view", "config:write",
        "terminal:pod", "terminal:node",
        "logs:view", "monitoring:view",
        "helm:view", "helm:write",
        "export", "ai_assistant",
    },
    "ops": {
        "workload:view", "workload:write",
        "network:view", "network:write",
        "storage:view",              // storage:write 不在 ops 上限
        "node:view",                 // node:manage 不在 ops 上限
        "config:view", "config:write",
        "terminal:pod", "terminal:node",
        "logs:view", "monitoring:view",
        "helm:view", "helm:write",
        "export", "ai_assistant",
    },
    "dev": {
        "workload:view", "workload:write",
        "network:view", "network:write",
        "config:view", "config:write",
        "terminal:pod",              // terminal:node 不在 dev 上限
        "logs:view", "monitoring:view",
        "export", "ai_assistant",
        // storage、node、helm 不在 dev 上限
    },
    "readonly": {
        "workload:view",
        "network:view",
        "storage:view",
        "node:view",
        "config:view",
        "logs:view", "monitoring:view",
        "helm:view",
        // 無任何 :write、terminal、export、ai_assistant
    },
    "custom": {
        // custom 類型由 K8s RBAC 決定，feature policy 不受上限約束
        // （所有 feature key 均視為在上限內）
    },
}
```

### 實施位置

- **後端**：`PATCH /permissions/:id/features` 寫入時，過濾掉超出 ceiling 的 keys
- **後端**：`GET /permissions/my-features` 回傳時，交集 ceiling 後再交集 policy
- **前端**：`hasFeature()` 僅是 UI 隱藏，後端仍是最終防線

---

## 5. 資料庫設計

### 5.1 修改 `ClusterPermission` 模型

在 `internal/models/permission.go` 中，為 `ClusterPermission` 新增一個欄位：

```go
type ClusterPermission struct {
    // ... 現有欄位不變 ...

    // 新增欄位：功能開關策略（JSON 格式）
    // NULL 表示採用 permission_type 的預設集合
    // 例：{"terminal:pod": false, "ai_assistant": false}
    FeaturePolicy string `json:"feature_policy" gorm:"type:text"`
}
```

### 5.2 FeaturePolicy 的 JSON 結構

```json
{
  "terminal:pod": false,
  "terminal:node": true,
  "ai_assistant": false,
  "export": true
}
```

**規則說明：**

- 僅需儲存「與預設值不同」的鍵值對，未列出的 key 沿用該 `permission_type` 的預設值
- `true` = 允許（前提是在 ceiling 內）
- `false` = 禁止

### 5.3 遷移腳本

```sql
-- 只新增欄位，不影響現有資料（NULL = 使用預設值，向後相容）
ALTER TABLE cluster_permissions ADD COLUMN feature_policy TEXT;
```

對應 GORM AutoMigrate：在 `internal/database/migrate.go` 的 `AutoMigrate` 呼叫中，`ClusterPermission` 已包含新欄位，即可自動完成遷移。

### 5.4 MyPermissionsResponse 擴充

在 `internal/models/permission.go` 中：

```go
type MyPermissionsResponse struct {
    ClusterID      uint     `json:"cluster_id"`
    ClusterName    string   `json:"cluster_name"`
    PermissionType string   `json:"permission_type"`
    PermissionName string   `json:"permission_name"`
    Namespaces     []string `json:"namespaces"`
    AllowedActions []string `json:"allowed_actions"`
    CustomRoleRef  string   `json:"custom_role_ref,omitempty"`

    // 新增欄位：使用者在此叢集的最終有效功能集合
    // 已套用 ceiling + feature_policy 的交集，前端直接使用
    AllowedFeatures []string `json:"allowed_features"`
}
```

---

## 6. 後端 API 設計

### 6.1 現有 API 修改

#### `GET /api/v1/permissions/my`（現有）

**修改**：在組裝 `MyPermissionsResponse` 時，額外計算並填充 `allowed_features`。

計算邏輯：

1. 取得該 `permission_type` 的 ceiling 集合
2. 讀取 `feature_policy` JSON（若為 NULL 則用空 map）
3. 從 ceiling 中，過濾掉 feature_policy 中 `false` 的 key
4. 結果即為 `allowed_features`

```go
func computeAllowedFeatures(permType string, featurePolicyJSON string) []string {
    ceiling := FeatureCeilings[permType]
    if permType == "custom" {
        // custom 類型：所有 feature 均視為允許（由 RBAC 控管）
        return AllFeatureKeys()
    }

    policy := map[string]bool{}
    if featurePolicyJSON != "" {
        _ = json.Unmarshal([]byte(featurePolicyJSON), &policy)
    }

    result := []string{}
    for _, key := range ceiling {
        // 若 policy 中明確設 false，則排除；否則允許
        if v, exists := policy[key]; exists && !v {
            continue
        }
        result = append(result, key)
    }
    return result
}
```

### 6.2 新增 API

#### `GET /api/v1/permissions/:id/features`

取得指定 `ClusterPermission` 的 feature policy 設定（管理員介面用）。

**權限要求**：Platform Admin

**Response：**

```json
{
  "permission_id": 42,
  "permission_type": "dev",
  "ceiling": ["workload:view", "workload:write", "terminal:pod", ...],
  "policy": {
    "terminal:pod": false,
    "ai_assistant": false
  },
  "effective": ["workload:view", "workload:write", "network:view", ...]
}
```

#### `PATCH /api/v1/permissions/:id/features`

更新指定 `ClusterPermission` 的 feature policy。

**權限要求**：Platform Admin

**Request Body：**

```json
{
  "policy": {
    "terminal:pod": false,
    "ai_assistant": false,
    "export": true
  }
}
```

**後端處理：**

1. 驗證 Platform Admin 身份
2. 讀取現有 `ClusterPermission`
3. 過濾掉 policy 中超出 ceiling 的 keys（防止繞過限制）
4. 更新 `feature_policy` 欄位
5. 回傳更新後的 effective features

**Response：**

```json
{
  "effective": ["workload:view", "workload:write", "network:view", ...]
}
```

### 6.3 路由註冊

在 `internal/router/routes_permission.go`（現有或新建）中：

```go
func registerFeaturePolicyRoutes(rg *gin.RouterGroup, d *routeDeps) {
    handler := handlers.NewFeaturePolicyHandler(d.clusterSvc, d.db)

    fp := rg.Group("/permissions")
    fp.Use(middleware.PlatformAdminRequired(d.db))
    {
        fp.GET("/:id/features",   handler.GetFeaturePolicy)
        fp.PATCH("/:id/features", handler.UpdateFeaturePolicy)
    }
}
```

---

## 7. 前端 PermissionContext 變更

### 7.1 更新 `MyPermissionsResponse` 型別

在 `ui/src/types/index.ts`（或 `permission.ts`）中，新增 `allowed_features` 欄位：

```typescript
export interface MyPermissionsResponse {
  cluster_id: number;
  cluster_name: string;
  permission_type: PermissionType;
  permission_name: string;
  namespaces: string[];
  allowed_actions: string[];
  custom_role_ref?: string;

  // 新增：後端已計算好的有效功能集合
  allowed_features: string[];
}
```

### 7.2 新增 `hasFeature()` 到 `PermissionContextType`

在 `ui/src/contexts/PermissionContext.ts` 中：

```typescript
export interface PermissionContextType {
  // ... 現有方法不變 ...

  // 新增：檢查當前叢集是否允許指定功能
  hasFeature: (key: string, clusterId?: number | string) => boolean;
}
```

### 7.3 在 `PermissionProvider` 中實作 `hasFeature`

在 `ui/src/contexts/PermissionProvider.tsx` 中：

```typescript
const hasFeature = useCallback(
  (key: string, clusterId?: number | string): boolean => {
    let permission: MyPermissionsResponse | null = null;

    if (clusterId) {
      const id =
        typeof clusterId === "string" ? parseInt(clusterId, 10) : clusterId;
      permission = clusterPermissions.get(id) || null;
    } else {
      permission = currentClusterPermission;
    }

    if (!permission) return false;

    // allowed_features 由後端計算（ceiling ∩ feature_policy）
    const features = permission.allowed_features ?? [];
    return features.includes(key);
  },
  [clusterPermissions, currentClusterPermission],
);
```

---

## 8. 前端新頁面設計

### 8.1 頁面位置

`ui/src/pages/access/FeaturePolicyPage.tsx`

路由：`/access/feature-policy`

### 8.2 頁面入口

在「訪問控制」側邊欄選單中新增：

```
訪問控制
  ├── 叢集權限
  ├── 使用者組
  └── 功能權限策略  ← 新增（僅 Platform Admin 可見）
```

### 8.3 頁面設計

#### 整體佈局

```
[頁面標題：功能權限策略]
[副標題：為每個叢集權限記錄設定允許的功能範圍]

[叢集選擇器]  ← Select，選擇要管理的叢集

[權限記錄表]
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ 使用者/群組  │ 類型  │ 終端:Pod │ 終端:Node │ 匯出 │ AI助手 │ ... │ 操作 │
├────────────────────────────────────────────────────────────────────────────────────────┤
│ user-a       │ dev   │   ✓      │    —      │  ✓   │  ✗     │ ... │ 編輯 │
│ user-b       │ ops   │   ✓      │    ✓      │  ✓   │  ✓     │ ... │ 編輯 │
│ group-x      │ readonly│  —     │    —      │  —   │  —     │ ... │ 編輯 │
└────────────────────────────────────────────────────────────────────────────────────────┘

圖例：✓ 允許  ✗ 已限制  — 基礎類型不支援（無法修改）
```

#### 欄位說明

| 符號                 | 說明                                           |
| -------------------- | ---------------------------------------------- |
| ✓（綠色 Switch ON）  | 在 ceiling 內，且 feature policy 允許          |
| ✗（紅色 Switch OFF） | 在 ceiling 內，但 feature policy 已禁止        |
| —（灰色，disabled）  | 不在此 permission_type 的 ceiling 內，無法操作 |

#### 編輯互動

點擊「編輯」按鈕後，彈出 Drawer（寬度 640）：

```
[Drawer 標題：功能設定 — user-a (dev)]

終端機操作
  Pod 終端機   [Switch ON/OFF]   說明：允許使用者進入 Pod 執行終端
  Node 終端機  [Switch 灰色]     說明：dev 類型不支援此功能

工具
  匯出功能     [Switch ON/OFF]   說明：允許匯出資源清單
  AI 助手      [Switch ON/OFF]   說明：允許使用 AI 智能助理

[取消]  [儲存設定]
```

> 灰色（disabled）的 Switch 附有 Tooltip 說明「此功能超出 dev 類型的權限上限」

### 8.4 API 呼叫時序

```
1. 頁面載入 → GET /api/v1/clusters（取得叢集列表）
2. 使用者選擇叢集 → GET /api/v1/permissions?cluster_id=X（取得該叢集所有 permission 記錄）
3. 點擊「編輯」→ GET /api/v1/permissions/:id/features（取得 ceiling + policy）
4. 使用者調整 Switch 後點「儲存」→ PATCH /api/v1/permissions/:id/features（更新 policy）
5. 儲存成功後，呼叫 refreshPermissions()（若當前使用者自己的權限有異動）
```

---

## 9. 現有元件更新計劃

### 9.1 目前使用 `canWrite()` 控制的「非寫入操作」需改為 `hasFeature()`

下表列出需要從 `canWrite()` 替換為 `hasFeature()` 的位置：

| 元件                                        | 目前控制方式     | 改為                          |
| ------------------------------------------- | ---------------- | ----------------------------- |
| `AIChatPanel.tsx`                           | `!isReadonly()`  | `hasFeature('ai_assistant')`  |
| `pod/columns.tsx` - Terminal 按鈕           | `canWrite` param | `hasFeature('terminal:pod')`  |
| `workload/tabs/InstancesTab.tsx` - Terminal | `canWrite()`     | `hasFeature('terminal:pod')`  |
| `node/columns.tsx` - Node Terminal 按鈕     | `canWrite` param | `hasFeature('terminal:node')` |
| `node/NodeList.tsx` - 匯出按鈕              | `canWrite()`     | `hasFeature('export')`        |
| `storage/*Tab.tsx` - 匯出按鈕               | `canWrite()`     | `hasFeature('export')`        |
| `network/IngressTab.tsx` - 匯出按鈕         | `canWrite()`     | `hasFeature('export')`        |
| 其他所有 List 頁面匯出按鈕                  | `canWrite()`     | `hasFeature('export')`        |

> **注意**：`canWrite()` 對於「建立/編輯/刪除」等真正的寫入操作繼續使用，不需要改動。

### 9.2 `createNodeColumns` / `getIngressColumns` 等 column factory 參數調整

由於 column factory 是純函數（非 React component），無法直接呼叫 `usePermission()`，需在父元件中取值後傳入：

**目前：**

```typescript
createNodeColumns({ ..., canWrite: canWrite() })
```

**調整後：**

```typescript
createNodeColumns({
  ...,
  canWrite: canWrite(),
  canTerminalPod:  hasFeature('terminal:pod'),
  canTerminalNode: hasFeature('terminal:node'),
})
```

---

## 10. 導航與路由

### 10.1 路由新增

在 `ui/src/routes.tsx`（或路由配置文件）中：

```typescript
{
  path: '/access/feature-policy',
  element: (
    <PlatformAdminGuard>
      <FeaturePolicyPage />
    </PlatformAdminGuard>
  ),
}
```

### 10.2 側邊欄新增

在 AppSider 訪問控制菜單項中，新增：

```typescript
{
  key: '/access/feature-policy',
  label: t('nav.featurePolicy'),
  icon: <ControlOutlined />,
  // 僅 Platform Admin 可見
  hidden: !isPlatformAdmin,
}
```

---

## 11. 實作順序

建議按以下順序實作，每個階段可獨立測試：

### Phase 1：後端資料層 ✅ 完成

1. ✅ 新增 `feature_policy` 欄位到 `ClusterPermission` 模型（`internal/models/permission.go`）
2. ✅ 在 `MyPermissionsResponse` 加入 `allowed_features` 欄位
3. ✅ 建立 `FeatureCeilings` 常量和 `ComputeAllowedFeatures()` 函數（`internal/models/feature_policy.go`）
4. ✅ 修改 `GET /permissions/my` handler，填充 `allowed_features`
5. ✅ 資料庫遷移腳本（`internal/database/migrations/mysql/007_feature_policy.up.sql`）

### Phase 2：後端管理 API ✅ 完成

6. ✅ 建立 `FeaturePolicyHandler`（GetFeaturePolicy + UpdateFeaturePolicy）（`internal/handlers/permission_feature_policy.go`）
7. ✅ 在 `routes_system.go` 的 `clusterPerms` group 中註冊路由
8. ✅ 撰寫 handler 測試（`internal/handlers/permission_feature_policy_test.go`）

### Phase 3：前端 PermissionContext 擴充 ✅ 完成

9. ✅ 更新 `MyPermissionsResponse` TypeScript 型別（`src/types/index.ts`）
10. ✅ 在 `PermissionContextType` 新增 `hasFeature()` 方法（`src/contexts/PermissionContext.ts`）
11. ✅ 在 `PermissionProvider` 實作 `hasFeature()` 邏輯（`src/contexts/PermissionContext.tsx`）

### Phase 4：現有元件遷移 ✅ 完成

11. 將 AI 助手、終端機、匯出按鈕從 `canWrite()/isReadonly()` 改為 `hasFeature()`
    - `layouts/MainLayout.tsx` — `<AIChatPanel />` 加 `hasFeature('ai_assistant')` 守衛
    - `pages/pod/columns.tsx` — 新增 `canTerminalPod?: boolean` 參數，控制 Terminal 按鈕顯示
    - `pages/pod/PodList.tsx` — 傳入 `canTerminalPod: hasFeature('terminal:pod')`，匯出按鈕加守衛
    - `pages/node/columns.tsx` — 新增 `canTerminalNode?: boolean` 參數，控制 Terminal 按鈕顯示
    - `pages/node/NodeList.tsx` — 傳入 `canTerminalNode: hasFeature('terminal:node')`，匯出按鈕加守衛
    - `pages/workload/tabs/InstancesTab.tsx` — Terminal 按鈕加 `hasFeature('terminal:pod')` 守衛
    - `pages/workload/{Deployment,StatefulSet,DaemonSet,CronJob,Job,ArgoRollout}Tab.tsx` (6 個) — 匯出按鈕加守衛
    - `pages/network/{ServiceTab,IngressTab}.tsx` — 匯出按鈕加守衛
    - `pages/storage/{PVTab,PVCTab,StorageClassTab}.tsx` — 匯出按鈕加守衛
    - `pages/namespace/NamespaceList.tsx` — 匯出按鈕加守衛
    - `pages/config/{ConfigMapList,SecretList}.tsx` — 匯出按鈕加守衛

### Phase 5：功能策略管理頁面 ✅ 完成

12. 建立 `FeaturePolicyPage.tsx`
13. 建立對應的 API service 方法
14. 新增路由和側邊欄項目
15. 新增 i18n 鍵值

### Phase 6：Bug Fix — 權限即時生效 + 統一無權限通知 ✅ 完成

**問題一：管理員修改策略後，已登入使用者的功能按鈕未即時更新**

根本原因：`PermissionContext` 僅在 App 啟動時載入一次權限，進入叢集時讀取舊快取。

修正：
- ✅ `PermissionContext.tsx` — `refreshPermissions` 改為 stable（空 deps），新增響應式 `useEffect([clusterPermissions, currentClusterId])` 確保 `currentClusterPermission` 任何時候都和最新資料同步
- ✅ `ClusterSelector.tsx` — 每次進入叢集（`currentClusterId` 變動）都呼叫 `refreshPermissions()`，後台改策略無需使用者重新整理即可生效

**問題二：無權限操作的錯誤通知各頁面不一致**

修正：
- ✅ 新建 `src/utils/permissionError.ts` — 匯出 `showPermissionDenied(feature?)` 統一通知函式，使用 `notification.warning`，`key: 'permission-denied'` 防止重複堆疊
- ✅ `src/utils/api.ts` — axios 攔截器加入 HTTP 403 全域處理，自動呼叫 `showPermissionDenied()`，所有 API 層無權限操作一律顯示統一通知
- ✅ `src/hooks/withPermission.tsx` — 移除硬編碼中文，改用 i18n key
- ✅ 三個語系檔（`zh-TW`、`zh-CN`、`en-US`）新增 `messages.permissionDenied`、`messages.permissionDeniedDesc`、`messages.permissionDeniedFeature` 鍵值

---

## 12. 未解決問題（待決策）

以下問題需要在開始實作前確認：

### Q1：feature policy 的作用域是「叢集層級」還是「整個平台層級」？

#### 整個平台層級

**目前設計**：feature policy 附加在 `ClusterPermission` 上，代表「某使用者在某叢集的功能限制」。

**備選方案**：建立一個全域的 `UserFeaturePolicy` 表，跨叢集統一設定。

**影響**：如果同一使用者在不同叢集需要不同的功能開關，應維持現設計（per ClusterPermission）；如果希望「全平台統一」，則需備選方案。

---

### Q2：`export` 功能的範圍？

目前所有 List 頁面都有匯出按鈕，是否用單一 `export` key 統一控制，還是細分為：

- `export:workload`
- `export:network`
- `export:storage`
- `export:node`

#### 細分

---

### Q3：`monitoring:view` 是否需要？

目前前端的監控圖表是否已有單獨的 feature 開關需求？若暫時不需要，可從 Phase 5 defer。

#### 需要，目前無開關

---

### Q4：Feature Policy 管理頁面是否顯示所有叢集，或僅顯示管理員有 admin 權限的叢集？

#### 不用，我在新增權限的時候就可以限制叢集和命名空間了。

---

_如有任何批注或修改意見，請直接在此文件標記後回覆，確認後即可開始 Phase 1 實作。_
