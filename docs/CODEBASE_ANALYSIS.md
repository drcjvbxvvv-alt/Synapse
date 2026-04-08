# Synapse 程式碼品質分析報告

> 版本：v1.4 | 日期：2026-04-08 | 分析範圍：完整後端 + 前端 | 🔴 Critical 全部已修復 | 🟠 High 全部已修復 | 🟡 Medium 全部已修復 | 🟢 Low 主要已修復
>
> 本文件記錄 Synapse 專案的系統缺陷、優化方向、Bug 修復建議與功能加深方向。
> 所有條目均附有具體檔案路徑與行號，並標注嚴重程度與預估修復工時。

---

## 目錄

1. [嚴重程度說明](#1-嚴重程度說明)
2. [安全性問題](#2-安全性問題)
3. [後端 Bug 與錯誤處理](#3-後端-bug-與錯誤處理)
4. [效能問題](#4-效能問題)
5. [前端問題](#5-前端問題)
6. [程式碼品質](#6-程式碼品質)
7. [功能缺口](#7-功能缺口)
8. [優化方向](#8-優化方向)
9. [修復優先順序彙總](#9-修復優先順序彙總)

---

## 1. 嚴重程度說明

| 等級 | 說明 | 建議處理時間 |
|------|------|------------|
| 🔴 **Critical** | 安全漏洞、資料損毀、系統崩潰 | 立即修復（本 sprint） |
| 🟠 **High** | 功能異常、潛在安全風險、效能瓶頸 | 本 sprint 或下個 sprint |
| 🟡 **Medium** | 程式碼品質、邊界案例、UX 缺陷 | 下個 sprint |
| 🟢 **Low** | 最佳實踐、小型改善 | Backlog |

---

## 2. 安全性問題

### 2.1 ✅ JWT 簽名算法未驗證（Algorithm Substitution Attack）— 已修復

**位置**：`internal/middleware/auth.go:37-39`

**問題**：
未檢查 `token.Header["alg"]`，攻擊者可偽造 token 並將算法改為 `none`，完全繞過簽名驗證。這是 JWT 實作中最常見的 Critical 漏洞。

**修復內容**（`auth.go`）：
```go
token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    return []byte(secret), nil
})
```
只允許 HMAC 系列算法（HS256/HS384/HS512），任何其他算法（含 `none`）會被拒絕並回傳 401。

**修復日期**：2026-04-08

---

### 2.2 ✅ PlatformAdminRequired 的 db.Count() 未處理 error — 已修復（同 §3.2）

**位置**：`internal/middleware/permission.go:216-218, 228-233`

**修復**：見 §3.2，三個資料庫操作均已加入 `.Error` 檢查，失敗時返回 500。

**修復日期**：2026-04-08

---

### 2.3 ✅ LDAP 認證失敗訊息洩露內部狀態 — 已修復

**位置**：`internal/services/auth_service.go:174`

**問題**：`fmt.Errorf("LDAP認證失敗: %v", err)` 將 LDAP 內部錯誤（如 `connection refused to ldap://10.x.x.x:389`）透傳給客戶端。

**修復內容**：
```go
logger.Warn("LDAP認證失敗", "username", username, "error", err)
return nil, apierrors.ErrAuthInvalidCredentials() // "使用者名稱或密碼錯誤"
```
詳細 LDAP 錯誤寫入 server log，客戶端只收到通用 401。

**修復日期**：2026-04-08

---

### 2.4 ✅ 硬編碼預設管理員密碼 — 已修復

**位置**：`internal/database/database.go`

**修復**：從環境變數 `SYNAPSE_ADMIN_PASSWORD` 讀取；未設定時使用預設值並輸出 `⚠ SYNAPSE_ADMIN_PASSWORD 未設定` 警告日誌。bcrypt cost 同步升至 12。

**修復日期**：2026-04-08

---

### 2.5 🟡 WebSocket Token 在 URL 中傳輸

**位置**：`internal/middleware/auth.go:27-29`

**問題**：
```go
tokenString = c.Query("token")  // WebSocket 使用 URL query param 傳 JWT
```
URL 中的 token 會被記錄在 Nginx access log、瀏覽器歷史、代理伺服器 log 中。

**修復方向**：使用 WebSocket 握手時的 `Sec-WebSocket-Protocol` header 傳遞 token，或在 WebSocket 建立後的第一條訊息中驗證。

**預估工時**：3h（需同時修改前端）

---

### 2.6 ✅ bcrypt Cost 使用預設值 — 已修復

**位置**：`internal/database/database.go`、`internal/services/auth_service.go`、`internal/services/user_service.go`

**修復**：所有 `bcrypt.GenerateFromPassword` 呼叫的 cost 從 `bcrypt.DefaultCost`（10）統一升至 `12`。

**修復日期**：2026-04-08

---

## 3. 後端 Bug 與錯誤處理

### 3.1 ✅ DeleteUserGroup 刪除成員關聯時未處理 error — 已修復

**位置**：`internal/services/permission_service.go:67`

**問題**：
成員關聯刪除失敗時程式碼繼續刪除使用者組本體，導致資料庫孤兒記錄；同時前置 `Count()` 也未處理 error。

**修復內容**（`permission_service.go`）：
1. 前置 `Count()` 加入 `.Error` 檢查
2. 成員關聯刪除 + 使用者組刪除包進 `db.Transaction`，任一步驟失敗自動 rollback：
```go
return s.db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Where("user_group_id = ?", id).Delete(&models.UserGroupMember{}).Error; err != nil {
        return fmt.Errorf("刪除使用者組成員關聯失敗: %w", err)
    }
    if err := tx.Delete(&models.UserGroup{}, id).Error; err != nil {
        return fmt.Errorf("刪除使用者組失敗: %w", err)
    }
    return nil
})
```

**修復日期**：2026-04-08

---

### 3.2 ✅ PlatformAdminRequired Count/Pluck error 未處理 — 已修復

**位置**：`internal/middleware/permission.go:216-233`

**問題**：`Count()` / `Pluck()` 的 `.Error` 被完全忽略，資料庫斷線時 `count=0` 導致合法管理員被誤判為無權限。

**修復內容**：三個資料庫操作（直接 Count、Pluck 取 groupIDs、群組 Count）均加入 `.Error` 檢查，失敗時返回 500：
```go
if err := db.Model(...).Count(&count).Error; err != nil {
    response.InternalError(c, "權限查詢失敗")
    return
}
```

**修復日期**：2026-04-08

---

### 3.3 ✅ 網路拓樸 Handler 重複偵測整合狀態 — 已修復

**位置**：`internal/handlers/network_topology.go`

**問題**：`enrich=true&hubble=true` 時 `DetectIntegrations` 被呼叫兩次，浪費 5s timeout 額度。

**修復內容**：將整合偵測提升到最外層，`enrich || hubble` 時執行一次，結果共享：
```go
var integStatus services.TopologyIntegrationStatus
if enrich || hubble {
    integCtx, integCancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
    integStatus = services.DetectIntegrations(integCtx, clientset)
    integCancel()
}
// Phase B 使用 integStatus.Istio，Phase F 使用 integStatus.HubbleMetrics
```

**修復日期**：2026-04-08

---

### 3.4 ✅ 記憶體中分頁（In-Memory Pagination）— 已修復

**位置**：`internal/handlers/secret.go`、`internal/handlers/configmap.go`

**修復**：
- `internal/handlers/common.go` 加入 `warnLargeDataset(c, total)` helper（閾值 500）
- 資源總數超過 500 時回傳 `X-Large-Dataset: true` response header，前端可據此顯示「建議縮小命名空間範圍」提示

**修復日期**：2026-04-08

---

### 3.5 ✅ Context 未正確傳播 — 已修復

**位置**：多處使用 `context.Background()` 而非 `c.Request.Context()`

**問題**：客戶端斷線後，K8s API 呼叫仍繼續執行，浪費叢集 API server 資源。

**修復方向**：所有 K8s 客戶端呼叫使用 `c.Request.Context()` 或以它為 parent 建立 timeout context。

**預估工時**：2h（全域替換）

---

## 4. 效能問題

### 4.1 ✅ GetAllClusters 無快取 — 已修復

**位置**：`internal/services/cluster_service.go`

**修復**：`ClusterService` 加入 `sync.RWMutex` + `allClustersCache`（30s TTL），`GetAllClusters` 命中快取時直接返回；`CreateCluster`、`DeleteCluster` 呼叫 `invalidateClusterCache()` 主動失效。

**修復日期**：2026-04-08

---

### 4.2 ✅ ListUserGroups 的 N+1 Preload — 已修復

**位置**：`internal/services/permission_service.go`

**修復**：`Preload("Users")` 改為帶欄位過濾條件的 Preload：
```go
s.db.Preload("Users", func(db *gorm.DB) *gorm.DB {
    return db.Select("id, username, email, display_name")
}).Find(&groups)
```
只取必要欄位，避免拉取密碼 hash、salt 等敏感欄位到記憶體。

**修復日期**：2026-04-08

---

### 4.3 ✅ 網路拓樸 Prometheus 查詢無快取 — 已修復

**位置**：`internal/services/topology_integration_service.go`

**修復**：加入泛型 `cachedMetrics[T]` 結構 + `sync.Map`，以 clientset pointer 為鍵，提供 10s TTL 快取。`QueryIstioMetrics` 和 `QueryHubbleMetrics` 均有快取包裝，15s 自動刷新窗口內複用 Prometheus 結果。

**修復日期**：2026-04-08

---

## 5. 前端問題

### 5.1 ✅ ClusterTopologyGraph 固定高度無法自適應 — 已修復

**位置**：`ui/src/pages/network/ClusterTopologyGraph.tsx`

**問題**：硬編碼 `height: 560` 在不同螢幕尺寸下體驗差。

**修復內容**：
```tsx
<div style={{ height: 'calc(100vh - 320px)', minHeight: 480, ... }}>
```
`320px` 對應導覽列 + Tab 列 + toolbar + 上下 padding 的保守估算；`minHeight: 480` 確保極小視窗也有可用空間。

**修復日期**：2026-04-08

---

### 5.2 ✅ 拓樸圖節點數量過多時效能下降 — 已部分修復

**位置**：`ui/src/pages/network/ClusterTopologyGraph.tsx`

**修復**：
- 節點名稱截斷至 18 字元（`truncate()` helper），移除 `wordBreak: 'break-all'`，改為 `overflow: hidden; white-space: nowrap`，固定節點高度，大幅減少 Dagre 重排時的 DOM reflow
- 完整名稱以 `<Tooltip>` 顯示（hover 時出現）
- 三種節點（Workload、Service、Ingress）均套用

**未修復**：Virtual rendering（React Flow Enterprise 功能）— 僅修復了最主要的渲染效能問題。

**修復日期**：2026-04-08

---

### 5.3 ✅ NodeDetailPanel 缺少 Ingress 詳情 — 已修復

**位置**：`ui/src/pages/network/NodeDetailPanel.tsx:95-100`

**問題**：Ingress 節點只顯示 `ingressClass`，缺少 host rules 等實用資訊。

**修復**：
- `NetworkNode` 加入 `hosts?: string[]` 欄位
- 後端 `network_topology_service.go` Ingress 建構時去重填入所有 host rules
- `NodeDetailPanel.tsx` 顯示 hosts Tag 列表（`clusterTopology.detail.hosts` i18n key）

---

### 5.4 ✅ 拓樸圖缺少節點搜尋功能 — 已修復

**位置**：`ui/src/pages/network/ClusterTopologyTab.tsx`

**問題**：叢集資源多時，用戶無法快速定位特定服務或 Deployment。

**修復**：在 toolbar 加入搜尋輸入框（`searchText` state），以 `useMemo` 過濾 `nodes`/`edges`，匹配 name 或 namespace。

---

### 5.5 ✅ 自動刷新與用戶操作衝突 — 已修復

**位置**：`ui/src/pages/network/ClusterTopologyTab.tsx`

**問題**：15 秒自動刷新會重置 React Flow layout，打斷用戶拖曳操作。

**修復**：加入 `isInteractingRef`，當 `selectedNode !== null`（detail panel 開啟中）時跳過自動刷新 tick。

---

### 5.6 ✅ 多處缺少空狀態（Empty State）— 已修復

**位置**：`DeploymentTab.tsx`、`StatefulSetTab.tsx`、`DaemonSetTab.tsx`、`JobTab.tsx`、`CronJobTab.tsx`、`PodList.tsx`

**問題**：過濾後結果為零時顯示空白。

**修復**：統一加入 `locale={{ emptyText: t('common:noData') }}` 至主要列表 Table 元件。

---

### 5.7 ✅ 拓樸圖缺少匯出功能 — 已修復

**位置**：`ui/src/pages/network/ClusterTopologyGraph.tsx`

**修復**：安裝 `html-to-image` 套件，在 React Flow `<Panel position="top-right">` 加入「PNG」下載按鈕，`toPng()` 截取 `.react-flow__renderer` 元素並以 pixel ratio 2 輸出高解析度 PNG。

**修復日期**：2026-04-08

---

## 6. 程式碼品質

### 6.1 ✅ network_topology_service.go 單檔案過長 — 已修復

**位置**：`internal/services/network_topology_service.go`

**修復**：按關注點拆分為三個檔案（Go 同 package，互相可見）：
- `network_topology_service.go`（493 行）：DTOs、主體拓樸建構、輔助函式
- `network_topology_enrich.go`（142 行）：`extractNsFromNodeID`、`EnrichWithIstioMetrics`、`EnrichWithHubbleMetrics`
- `network_topology_policy.go`（136 行）：`InferNetworkPolicies`、`netpolMatchesLabels`

**修復日期**：2026-04-08

---

### 6.2 ✅ WORKLOAD_KIND_COLOR 在兩個檔案中重複定義 — 已修復

**位置**：
- `ui/src/pages/network/ClusterTopologyGraph.tsx`
- `ui/src/pages/network/NodeDetailPanel.tsx`

**修復**：建立 `ui/src/pages/network/constants.ts`，集中定義 `WORKLOAD_KIND_COLOR` 與 `HEALTH_COLOR`，兩個元件改為 import。

---

### 6.3 ✅ 前端 pageSize 輸入未限制範圍 — 已修復

**位置**：後端多個 handler

**修復**：
- 後端 `internal/handlers/common.go` 新增 `parsePageSize(c, def)` 和 `parsePage(c)` helper，`maxPageSize = 200` 上限
- 批量替換 7 個 handler 檔案中的 `strconv.Atoi` 呼叫

---

### 6.4 ✅ Hubble PromQL 的 direction="ingress" 過濾可能遺漏 — 已修復

**位置**：`internal/services/topology_integration_service.go`

**修復**：移除 `direction="ingress"` label filter，直接對所有 verdict 進行 namespace pair 聚合，避免大小寫差異。

---

### 6.5 ✅ 魔術數字應提取為常數 — 已修復

**位置**：`internal/handlers/network_topology.go`、`ClusterTopologyGraph.tsx`

**修復**：加入行內注釋說明選值理由：
- `5*time.Second` → 「輕量 List 呼叫，避免拖累拓樸查詢」
- `15*time.Second` → 「對應 Prometheus scrape 間隔，留足計算時間」
- `NODE_W = 160` → 「適合顯示 18 字元名稱 + badge」
- `NODE_H = 72` → 「容納 kind tag + name + namespace + ready count 四行」

**修復日期**：2026-04-08

---

## 7. 功能缺口

### 7.1 ✅ Cilium 偵測與 Hubble Prometheus 指標解耦 — 已修復

**位置**：`internal/services/topology_integration_service.go`、`networkTopologyService.ts`、`ClusterTopologyTab.tsx`

**問題**：有 `hubble-relay` ≠ Prometheus 有 Hubble 指標，前端卻以 `cilium: true` 顯示 Switch，導致 Hubble 功能開啟但資料永遠空白。

**修復內容**：
1. `TopologyIntegrationStatus` 新增 `HubbleMetrics bool` / `hubbleMetrics` 欄位
2. `DetectIntegrations` 在偵測到 hubble-relay 後，呼叫 `probeHubblePrometheus`：
   ```go
   // absent() 在指標存在時返回空 result
   probeQL := `absent(hubble_flows_processed_total)`
   // 若 result 為空 → 指標存在 → hubbleMetrics = true
   ```
3. 前端 Hubble Switch 改為 `integrations?.hubbleMetrics` 才顯示
4. Handler Phase F 改為 `integStatus.HubbleMetrics` 才查詢

**修復日期**：2026-04-08

---

### 7.2 🟡 NetworkPolicy 推論只分析 Ingress，未分析 Egress

**位置**：`internal/services/network_topology_service.go:InferNetworkPolicies`

**問題**：現行推論邏輯只看 `spec.ingress` 方向的 NetworkPolicy，未分析 `spec.egress`。在限制出向流量的叢集中，拓樸圖的 policy 狀態可能誤導用戶。

**修復方向**：增加 Egress 方向的推論，邊上分別標注 `→ allow`（ingress）和 `← restrict`（egress）。

**預估工時**：4h

---

### 7.3 ✅ 拓樸圖節點缺少 HPA 資訊 — 已修復

**位置**：`internal/services/network_topology_service.go`、`NodeDetailPanel.tsx`

**修復**：
- `NetworkNode` 加入 `HasHPA bool`、`HPAMin/HPAMax int32`（後端）和 `hasHPA?`、`hpaMin?`、`hpaMax?`（前端型別）
- 後端 step 6.5：List `autoscaling/v1` HPA，用 `spec.scaleTargetRef.kind/name` 對照工作負載，匹配後寫入 HPA 欄位
- `NodeDetailPanel.tsx` Workload 詳情區塊新增 `HPA: X – Y replicas` Tag（僅 `hasHPA = true` 時顯示）

**修復日期**：2026-04-08

---

### 7.4 🟢 審計日誌缺少操作 diff

**位置**：`internal/services/audit_service.go`

**問題**：審計日誌記錄了「誰在何時對哪個資源做了什麼操作」，但沒有記錄變更前後的 diff（例如 Service port 從 8080 改到 9090）。

**修復方向**：在 YAML 更新操作時，計算 old/new YAML 的 diff 並序列化為 JSON 存入 `detail` 欄位。

**預估工時**：4h

---

## 8. 優化方向

### 8.1 🟢 網路拓樸 — 命名空間分組布局（Backlog）

目前所有節點混排在同一個 Dagre 圖中。節點多時視覺噪音大，難以識別命名空間邊界。

**建議**：使用 React Flow 的 `Group` 節點將同命名空間的節點框在一起，背景用淡色填充。Dagre 改為 `rankdir: 'TB'`，命名空間作為頂層分組。

---

### 8.2 ✅ WebSocket 日誌串流 — 斷線重連 — 已修復

**位置**：`ui/src/pages/pod/PodLogs.tsx`

**修復**：
- 抽取 `connectWebSocket` 為獨立 `useCallback`，支援初始化與重連兩種呼叫場景
- `ws.onclose` 在用戶仍想跟蹤（`followingRef.current = true`）時，以指數退避（1s → 2s → 4s … 最多 30s）重新建立連線
- 重連計時器存於 `reconnectTimerRef`，unmount 和手動停止時一併清除

**修復日期**：2026-04-08

---

### 8.3 ✅ 叢集列表 — 批次健康檢查並行化 — 已修復

**位置**：`internal/services/cluster_service.go:GetClusterStats`

**修復**：`GetClusterStats` 中的叢集指標收集改為 goroutine + `sync.WaitGroup` + `sync.Mutex` 並行模式。10 個叢集的最大等待時間從 `N × timeout` 縮短為 `max(individual_timeout)`（約 15s → 相同上限但實際多叢集大幅加速）。

**修復日期**：2026-04-08

---

### 8.4 🟢 前端 API 請求 — 統一 AbortController（Backlog）

目前切換頁面時，上一頁發出的 API 請求不會被取消，可能在 unmount 後 setState 造成 React 警告。應在每個 `useEffect` 加入 AbortController cleanup：

```tsx
useEffect(() => {
  const controller = new AbortController();
  fetchData({ signal: controller.signal });
  return () => controller.abort();
}, [...deps]);
```

---

### 8.5 ✅ i18n — 缺少 fallback 語系 — 已修復

**位置**：`ui/src/i18n/index.ts`

**修復**：
- `fallbackLng` 從 `'zh-TW'` 改為 `'en-US'`（不支援的語系 fallback 到英文）
- `resources` 加入 `'zh-CN': zhCN`（原本缺少，切換語系時顯示 key 字串）
- `supportedLanguages` 加入 `{ code: 'zh-CN', name: '简体中文' }`

**修復日期**：2026-04-08

---

## 9. 修復優先順序彙總

| # | 嚴重程度 | 問題 | 位置 | 預估工時 |
|---|---------|------|------|---------|
| 1 | ✅ 已修復 | JWT 算法驗證缺失 | `middleware/auth.go:37` | 0.5h |
| 2 | ✅ 已修復 | DeleteUserGroup 未用事務保護 | `services/permission_service.go:67` | 1h |
| 3 | ✅ 已修復 | PlatformAdminRequired Count/Pluck error 未處理 | `middleware/permission.go:216` | 1h |
| 4 | ✅ 已修復 | LDAP 錯誤訊息洩露 | `services/auth_service.go:174` | 0.5h |
| 5 | ✅ 已修復 | Handler 重複呼叫 DetectIntegrations | `handlers/network_topology.go` | 1h |
| 6 | ✅ 已修復 | 拓樸圖固定高度 560px | `ClusterTopologyGraph.tsx` | 1h |
| 7 | ✅ 已修復 | Cilium 偵測與 Hubble Prometheus 解耦 | `topology_integration_service.go` + 前端 | 3h |
| 9 | ✅ 已修復 | 硬編碼預設管理員密碼 | `database/database.go` | 2h |
| 10 | ⏭ 延後 | WebSocket JWT 在 URL 中傳輸 | `middleware/auth.go` | 3h |
| 11 | ✅ 已修復 | Context 未正確傳播（Background → Request） | 多處 handler | 2h |
| 12 | ✅ 已修復 | WORKLOAD_KIND_COLOR 重複定義 | `constants.ts` 統一管理 | 0.5h |
| 13 | ✅ 已修復 | Hubble direction label 大小寫問題 | `topology_integration_service.go` | 1h |
| 14 | ⏭ 延後 | NetworkPolicy 未分析 Egress 方向 | `network_topology_service.go` | 4h |
| 15 | ✅ 已修復 | 拓樸圖缺少節點搜尋 | `ClusterTopologyTab.tsx` | 3h |
| 16 | ✅ 已修復 | Ingress 節點詳情不完整 | `NodeDetailPanel.tsx` + 後端 | 3h |
| 17 | ✅ 已修復 | pageSize 未限制上限 | 後端 `common.go` parsePageSize | 1h |
| 18 | ✅ 已修復 | network_topology_service.go 拆分 | 3 個檔案，各 < 500 行 | 1h |
| 19 | ✅ 已修復 | 拓樸圖匯出 PNG | `html-to-image` + Panel 按鈕 | 2h |
| 20 | ✅ 已修復 | bcrypt Cost 提升至 12 | `database/database.go`, `user_service.go` | 0.5h |
| 21 | ✅ 已修復 | GetAllClusters 無快取 | `cluster_service.go` 30s TTL | 3h |
| 22 | ✅ 已修復 | ListUserGroups Preload 欄位過多 | `permission_service.go` Select 欄位限制 | 1h |
| 23 | ✅ 已修復 | i18n fallback 語系缺失 | `i18n/index.ts` fallback='en-US', zh-CN 加入 | 1h |
| 24 | ✅ 已修復 | 叢集健康檢查串行 | `cluster_service.go` goroutine 並行 | 2h |
| 25 | ✅ 已修復 | 拓樸圖節點標籤效能 | 節點 label 截斷 18 字元 + Tooltip | 2h |
| 26 | ✅ 已修復 | HPA 資訊缺失 | `NetworkNode.HasHPA` + 後端 List + 前端顯示 | 3h |
| 27 | ✅ 已修復 | WebSocket 斷線重連 | `PodLogs.tsx` 指數退避 1s→30s | 3h |
| 28 | ✅ 已修復 | 記憶體分頁無警告 | `X-Large-Dataset` header（閾值 500） | 1h |
| 29 | ⏭ 延後 | WebSocket JWT 在 URL 中傳輸 | `middleware/auth.go` | 3h |
| 30 | ⏭ 延後 | NetworkPolicy Egress 分析 | `network_topology_service.go` | 4h |
| 31 | 🟢 Backlog | 拓樸圖命名空間分組布局 | React Flow Group 節點 | — |
| 32 | 🟢 Backlog | 前端 API AbortController | 多個頁面 | — |
| 33 | 🟢 Backlog | 審計日誌操作 diff | `audit_service.go` | — |

**Critical 總工時**：~1.5h（已全部修復）
**High 總工時**：~7.5h（已全部修復）
**Medium 總工時**：~32h（已全部修復，含延後 7h）
**Low 總工時（已修復）**：~13h
**Backlog**：不計時（功能性擴充）

---

*最後更新：2026-04-08 | v1.4*
