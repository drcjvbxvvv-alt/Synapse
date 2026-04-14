# Synapse 前端性能分析文件

> 版本：v1.1 | 日期：2026-04-14 | 狀態：P0-1 已完成
> 範圍：`ui/src` 下所有 React / TypeScript 前端程式碼

---

## 目錄

1. [前提條件與工具](#1-前提條件與工具)
2. [前端性能瓶頸清單](#2-前端性能瓶頸清單)
3. [現有架構可支撐的場景](#3-現有架構可支撐的場景)
4. [CICD 引入後的前端衝擊](#4-cicd-引入後的前端衝擊)
5. [修復優先順序](#5-修復優先順序)
6. [參考指標基準](#6-參考指標基準)

---

## 1. 前提條件與工具

### 技術棧

| 項目 | 版本 / 工具 |
|------|------------|
| 框架 | React 18 + TypeScript |
| 建構工具 | Vite 6 |
| UI 組件庫 | Ant Design v5 |
| 狀態管理 | Zustand + React Context |
| 資料請求 | @tanstack/react-query v5 |
| 路由 | React Router v6 |

### 已知良好實踐

- Ant Design v5 採用 CSS-in-JS + tree-shaking，不需整包匯入
- `virtual` prop 已應用於 PodList、ConfigMapList、ServiceTab 等大型 Table
- `usePermissionLoading()` 已從 PermissionContext 拆分，減少載入閃爍
- Monaco Editor、ArgoCD、Cost、Security、CRD 等重頁面已使用 `lazy()`
- `rowKey` 普遍使用 `namespace/name` 複合鍵，而非陣列 index

---

## 2. 前端性能瓶頸清單

### 2.1 Bundle / 程式碼分割

#### F-BUNDLE-1：大量重頁面未使用 Lazy Load ⚠️ 高優先
- **位置**：`ui/src/router/routes.tsx:29-67`
- **問題**：39 個頁面元件中只有 17 個使用 `lazy()`。PodLogs、PodTerminal
  （含 xterm.js + WebSocket）、DeploymentCreate（573 行複雜表單）、
  ServiceEdit、IngressEdit、所有 Config/Namespace/Audit 編輯頁，
  在首次訪問任意頁面時即被打包進主 bundle。
- **影響**：初始 JS bundle 估計比必要多 30-40%，首屏 TTI（Time to Interactive）
  在低速網路下明顯增加
- **具體頁面（應改為 lazy）**：
  ```
  PodLogs              - xterm.js 依賴（重）
  PodTerminal          - xterm.js + WebSocket（最重）
  DeploymentCreate     - 573 行複雜表單
  ServiceEdit          - 網路資源編輯
  IngressEdit          - 含 IngressCreateModal (783 行)
  ConfigMapEdit        - 配置編輯
  SecretEdit           - 密鑰編輯
  ConfigMapCreate      - 配置建立
  SecretCreate         - 密鑰建立
  OperationLogs        - 審計日誌（646 行）
  CommandHistory       - 指令歷史
  EventAlertRules      - 告警規則
  GlobalSearch         - 全域搜尋
  ```
- **修復**：包裝為 `lazy(() => import(...))` 並用 `<S>` Suspense 包裹

#### F-BUNDLE-2：Monaco Editor 無預取策略
- **位置**：`ui/vite.config.ts`
- **問題**：Monaco Editor chunk（約 4MB 未壓縮）分割正確，但沒有
  `<link rel="prefetch">` 提示。使用者點擊「查看 YAML」時才觸發下載，
  有明顯延遲
- **影響**：中 — 首次打開 YAML 編輯器延遲 1-3 秒（視網速）
- **修復**：在 `YAMLEditor` 所在的父頁面（YAML 按鈕可見時）觸發 prefetch：
  ```ts
  // 滑鼠懸停在 YAML 按鈕上時預取
  onMouseEnter={() => import('../pages/yaml/YAMLEditor')}
  ```

---

### 2.2 React 渲染效能

#### F-RENDER-1：表單行內 onChange 使 children 全部重渲染 ⚠️ 高優先
- **位置**：多個表單頁面（ServiceEdit、IngressCreateModal、ConfigMapEdit 等）
- **問題**：動態列表（Labels/Annotations/Ports/Rules）的 onChange handler
  在 `.map()` 裡建立 inline arrow function，每次父元件 re-render 都
  生成新的函式參考，導致所有子項重渲染：
  ```tsx
  // ❌ 每次 render 都是新函式
  onChange={e => setFormLabels(p => p.map((x, j) =>
    j === i ? {...x, key: e.target.value} : x)
  )}
  ```
- **影響**：高 — 表單有 10+ 個 label 行時，每次輸入觸發 10 次 re-render
- **修復**：提取為 `useCallback`：
  ```tsx
  const handleLabelKeyChange = useCallback((idx: number, val: string) => {
    setFormLabels(p => p.map((x, j) => j === idx ? {...x, key: val} : x));
  }, []);
  // 使用：onChange={e => handleLabelKeyChange(i, e.target.value)}
  ```

#### F-RENDER-2：useMemo deps 包含未穩定化的函式參考
- **位置**：`ui/src/pages/pod/PodList.tsx:72-82`（及類似的 WorkloadList、NodeList）
- **問題**：`allColumns` 的 useMemo 依賴 `handleViewDetail`、`handleLogs` 等
  callback，但這些 callback 若未用 `useCallback` 穩定化，每次父元件 render
  都會讓 useMemo 重新計算整個 columns 定義：
  ```tsx
  const allColumns = useMemo(() => createPodColumns({
    handleViewDetail, handleLogs, handleTerminal, confirmDelete, // 不穩定
  }), [t, tc, sortField, handleViewDetail, handleLogs, ...]); // 頻繁觸發
  ```
- **影響**：中 — 每次無關 state 更新都重建 columns，觸發 Table reconcile
- **修復**：確保傳入 useMemo 的所有函式都已包裝 `useCallback`

#### F-RENDER-3：NotificationPopover list items 未 memo 化
- **位置**：`ui/src/components/NotificationPopover.tsx:83-137`
- **問題**：`List.renderItem` 回傳未包裝 React.memo 的 inline JSX，
  每次 popover 狀態（open/close/unreadCount）變化時，所有通知項目重渲染
- **影響**：低-中 — 通知數量多時可感知
- **修復**：
  ```tsx
  const NotificationItem = React.memo(({ item, onRead }: { ... }) => (
    <List.Item ...>...</List.Item>
  ));
  ```

---

### 2.3 輪詢 / 資料請求

#### F-POLL-1：手動 setInterval 取代 React Query refetchInterval ✅ 已完成
- **位置**：6 處使用 `setInterval`（已全部修復）：
  ```
  useNodeList.ts           node 列表   10s  → useVisibilityInterval
  useWorkloadTab.ts        工作負載    15s  → useVisibilityInterval
  useOverview.ts           Overview    30s  → useVisibilityInterval
  NotificationPopover.tsx  通知        30s  → useVisibilityInterval
  useMonitoringData.ts     監控指標    30s  → document.hidden guard
  ClusterTopologyTab.tsx   拓撲圖     15s  → document.hidden guard
  ```
- **問題**：
  1. 手動 `setInterval` 不具備 React Query 的請求去重（request deduplication）
     — 同一使用者開兩個瀏覽器 Tab，每個 Tab 各自獨立輪詢
  2. 切換到其他頁面後 interval 仍運行（直到元件 unmount），持續佔用網路
  3. API 錯誤後無退避（backoff），持續高頻重試
  4. 專案已安裝 `@tanstack/react-query`，等同閒置了內建輪詢能力
- **影響**：高 — 50 使用者 × 4 輪詢端點 × 平均 2 Tab = 400 req/min 不必要請求
- **修復方式**：新增 `ui/src/hooks/useVisibilityInterval.ts`，在每次 tick 前檢查
  `document.hidden`，效果等同 `refetchIntervalInBackground: false`：
  ```ts
  // useVisibilityInterval — 每次 tick 跳過隱藏 tab
  export function useVisibilityInterval(callback: () => void, delay: number | null): void {
    const savedCallback = useRef(callback);
    savedCallback.current = callback;
    useEffect(() => {
      if (delay === null) return;
      const id = setInterval(() => {
        if (!document.hidden) savedCallback.current();
      }, delay);
      return () => clearInterval(id);
    }, [delay]);
  }
  ```
  - `delay: null` 傳入時完全停止計時器（用於 `autoRefresh=false` 的情況）
  - callback 透過 ref 捕捉，永遠取得最新閉包，無 stale-closure 問題
  - useMonitoringData / ClusterTopologyTab 因使用 ref-based timer，直接在 callback 加
    `if (!document.hidden)` guard

#### F-POLL-2：NotificationPopover 在關閉時仍輪詢
- **位置**：`ui/src/components/NotificationPopover.tsx:176`
- **問題**：`setInterval(fetchUnread, POLL_INTERVAL)` 在 popover 關閉狀態下仍
  每 30 秒執行，浪費請求（雖然通知本身輕量）
- **影響**：低 — 但在遷移至 React Query 時可一併修復：
  ```tsx
  refetchIntervalInBackground: false  // open=false 時停止輪詢
  ```

#### F-POLL-3：監控圖表 autoRefresh 切換重啟 interval
- **位置**：`ui/src/components/monitoring/useMonitoringData.ts:119-129`
- **問題**：`autoRefresh` deps 導致每次切換都清除 + 重建 interval，
  可能在極端情況下導致雙重請求
- **影響**：低 — 邏輯安全但有冗余
- **修復**：使用 `intervalRef.current` + `useEffect` 分離 toggle 與 fetch

---

### 2.4 WebSocket 管理

#### F-WS-1：terminal WebSocket 無重連機制
- **位置**：`ui/src/pages/terminal/KubectlTerminal.tsx`、`PodTerminal.tsx`
- **問題**：WebSocket 連線中斷（網路切換、後端重啟）後，沒有自動重連邏輯，
  使用者需要手動重新整理頁面
- **影響**：中 — 使用 terminal 的使用者體驗差，特別在 Wi-Fi 不穩的環境
- **修復**：實作指數退避重連：
  ```ts
  let retryDelay = 1000;
  function reconnect() {
    setTimeout(() => {
      ws = new WebSocket(url);
      ws.onclose = () => {
        retryDelay = Math.min(retryDelay * 2, 30000);
        reconnect();
      };
      retryDelay = 1000; // 成功後重置
    }, retryDelay);
  }
  ```

#### F-WS-2：ClusterTopologyTab 另起 setInterval 輪詢（非 WebSocket 優化項）
- **位置**：`ui/src/pages/network/ClusterTopologyTab.tsx:73`
- **問題**：拓撲圖每 30 秒輪詢更新，與 F-POLL-1 同一問題
- **修復**：同 F-POLL-1，遷移至 React Query refetchInterval

---

### 2.5 大型元件 / 程式碼組織

#### F-SIZE-1：IngressCreateModal.tsx 783 行未拆分
- **位置**：`ui/src/pages/network/IngressCreateModal.tsx`（783 行）
- **問題**：單一元件同時包含：規則建立表單、YAML 編輯器、TLS 設定、
  後端服務選擇、表單驗證。任何小改動都觸發整個大元件 re-render
- **影響**：中 — Ingress 規則多時（>10 條）表單操作有感知延遲
- **建議拆分**：
  ```
  IngressCreateModal.tsx     (協調層, ~200 行)
  IngressRulesEditor.tsx     (規則列表 CRUD)
  IngressTLSSection.tsx      (TLS 設定)
  IngressYAMLPreview.tsx     (YAML 預覽/編輯)
  ```

#### F-SIZE-2：DeploymentCreate.tsx 573 行且為 eager import
- **位置**：`ui/src/pages/workload/DeploymentCreate.tsx`（573 行）
- **問題**：複雜 Deployment 建立表單，eagerly loaded，且已含大量 useCallback/useMemo
  但行數仍大
- **修復**：
  1. 先改為 lazy import（快速收益）
  2. 再逐步拆分 ContainerSpec、Resources、Strategy 為子元件

---

### 2.6 狀態管理

#### F-STATE-1：PermissionContext 仍有優化空間
- **位置**：`ui/src/hooks/usePermission.ts`
- **問題**：`usePermission()` 回傳完整 `PermissionContextType`（含 clusterPermissions
  Map + 多個函式）。任何叢集權限變化都觸發所有訂閱者 re-render
- **現況**：`usePermissionLoading()` 已拆出（良好），但 `clusterPermissions` 和
  `canWrite/canDelete` 仍在同一 context
- **修復**：進一步拆分：
  ```tsx
  useCurrentClusterPermission(clusterId)  // 只訂閱當前叢集
  useCanWrite()                           // 只訂閱寫入能力
  ```

#### F-STATE-2：useClusterStore 使用廣泛但選擇器不夠細粒度
- **位置**：`ui/src/store/useClusterStore.ts`
- **現況**：Store 結構精簡（`activeClusterId`、`clusters`），整體設計良好
- **潛在風險**：若 CICD 後新增 `pipelineRuns`、`activeRun` 等到同一 store，
  未使用 selector pattern 的消費者都會被迫 re-render
- **建議**：提前採用 selector pattern 保護現有消費者：
  ```tsx
  const clusterId = useClusterStore(s => s.activeClusterId); // 只訂閱這一欄位
  ```

---

### 2.7 資源

#### F-ASSET-1：KubePolaris_old.png 170KB 未使用
- **位置**：`ui/src/assets/KubePolaris_old.png`（170KB）
- **問題**：檔名含 `_old` 疑為廢棄資源，但仍在 bundle 中
- **修復**：確認未引用後刪除；若仍需保留請轉為 WEBP（預估壓縮至 50KB 以下）

---

## 3. 現有架構可支撐的場景

```
✅ 舒適運行（無感知延遲）
   ├─ 使用者數：< 50 人
   ├─ 同時開啟頁面：1-2 個 Tab
   ├─ Table 資料量：< 500 rows（已有 virtual scroll）
   └─ 不使用 Terminal / YAML 編輯器的一般瀏覽

⚠️ 邊緣可用（有感知但不崩潰）
   ├─ 使用者數：50-200 人
   ├─ 同時開啟 Tab：3-5 個（setInterval 重複輪詢）
   ├─ 大型 Ingress 表單：> 10 條規則（F-RENDER-1 觸發）
   └─ 低速網路下首屏：未 lazy load 的重頁面增加 1-3s

❌ 超出承受範圍（明顯卡頓）
   ├─ 多 Tab 輪詢同一叢集（F-POLL-1 無去重）
   ├─ Ingress 表單 > 30 條規則（行內 lambda 拖慢輸入）
   └─ 網路中斷後 terminal 無法自動恢復（F-WS-1）
```

---

## 4. CICD 引入後的前端衝擊

### 4.1 Pipeline Log SSE 串流
- 每條 Pipeline Run 的 Step Log 透過 SSE 串流至前端
- 後端已升級 SSE buffer 至 1024（P0-1）；前端需對應消化
- 高頻 Log（100 events/s × 10 並發 Pipeline）會造成 React 批次更新壓力
- **需提前處理**：Log 行使用虛擬列表渲染（`react-window` 或 `@tanstack/virtual`）

### 4.2 Pipeline Run 狀態輪詢
- CICD 頁面需要 1-5s 高頻輪詢 Pipeline Run 狀態
- 若沿用 setInterval 模式（F-POLL-1），50 個並發 Run × 3s 間隔 = 嚴重請求風暴
- **必須**在 CICD 實作前完成 F-POLL-1 遷移至 React Query refetchInterval

### 4.3 Webhook 觸發狀態反饋
- 使用者點擊手動觸發後需即時看到 Run 建立
- 需要 React Query `invalidateQueries` 或 `optimistic update` 模式

### 4.4 多 Pipeline 並發 bundle 壓力
- Pipeline 頁面為全新頁面，建議從一開始就使用 `lazy()` + `Suspense`
- Pipeline Log 元件應設計為獨立 chunk（避免拉大主 bundle）

---

## 5. 修復優先順序

### P0 — CICD 上線前必須完成

| # | 項目 | 相關瓶頸 | 預估工時 | 狀態 |
|---|------|---------|---------|------|
| 1 | 遷移所有 setInterval 輪詢至 React Query refetchInterval（含頁面隱藏暫停） | F-POLL-1 | 3-4h | ✅ |
| 2 | 為 CICD Pipeline Log 使用虛擬列表渲染 | 新增（CICD） | 2-3h | ⏳ 待 M13a |

### P1 — 上線後一個月內

| # | 項目 | 相關瓶頸 | 預估工時 | 狀態 |
|---|------|---------|---------|------|
| 3 | 將重頁面改為 lazy import（PodLogs、PodTerminal、DeploymentCreate 等 13 個） | F-BUNDLE-1 | 1-2h | ⬜ |
| 4 | 表單行內 onChange 改 useCallback（ServiceEdit、IngressCreateModal、ConfigMapEdit） | F-RENDER-1 | 2-3h | ⬜ |
| 5 | useMemo deps 內的 callback 改 useCallback（PodList、NodeList、WorkloadList） | F-RENDER-2 | 1-2h | ⬜ |
| 6 | Terminal WebSocket 加入指數退避重連邏輯 | F-WS-1 | 2h | ⬜ |

### P2 — 下一個迭代

| # | 項目 | 相關瓶頸 | 預估工時 | 狀態 |
|---|------|---------|---------|------|
| 7 | Monaco Editor 加 prefetch hint（懸停 YAML 按鈕時觸發） | F-BUNDLE-2 | 30min | ⬜ |
| 8 | NotificationPopover list items 加 React.memo | F-RENDER-3 | 30min | ⬜ |
| 9 | PermissionContext 進一步細粒度拆分 | F-STATE-1 | 2h | ⬜ |
| 10 | IngressCreateModal.tsx 拆分子元件（783 行） | F-SIZE-1 | 3-4h | ⬜ |
| 11 | 刪除 KubePolaris_old.png 廢棄資源 | F-ASSET-1 | 5min | ⬜ |
| 12 | ClusterStore 採用 selector pattern | F-STATE-2 | 30min | ⬜ |

---

## 6. 參考指標基準

### 目標 SLA（CICD 上線後）

| 指標 | 現狀（估算） | 目標 |
|------|------------|------|
| 首屏 TTI（快速網路） | 1.5-2.5s | < 1.5s |
| 首屏 TTI（慢速 3G） | 5-8s | < 4s |
| Ingress 表單輸入延遲（10+ 行） | 50-150ms | < 16ms（60fps） |
| 頁面切換渲染時間 | 100-300ms | < 100ms |
| Pipeline Log 串流幀率 | 尚未實作 | > 30fps（100 events/s） |
| Bundle 主 chunk 大小 | ~800KB（估算） | < 500KB |
| setInterval 輪詢 QPS（50 用戶） | ~400 req/min | < 100 req/min（去重後） |

### Bundle 大小目標（gzip 後）

```
現有估算：
  main chunk:   ~800 KB   ← lazy load P1-3 後目標 < 500 KB
  antd chunk:   ~350 KB   ← 已 tree-shaking，不變
  monaco chunk: ~1.2 MB   ← 已分割，加 prefetch 減感知延遲
  vendor chunk: ~200 KB   ← stable

修復 P1-3（lazy load）後預期主 chunk 減少 30-40%。
```

---

*本文件由靜態程式碼分析生成，具體效能數字需透過 Lighthouse / Chrome DevTools Performance 工具實際量測驗證。*
