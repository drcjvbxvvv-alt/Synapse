# Synapse 全面架構反思報告

> 日期: 2026-04-12
> 範圍: 後端 Go + 前端 React + API 設計 + 基礎設施 + 安全

---

## 專案規模一覽

| 維度 | 數量 |
|------|------|
| Go 檔案 | 333 個, ~66,600 行 |
| 前端元件 | 59 共用 + 251 頁面檔 |
| API 端點 | 334 個 |
| 資料模型 | 125 個 struct |
| 服務檔案 | 84 (後端) + 65 (前端) |
| 測試檔案 | 41 個 |
| i18n 語系 | 3 語系, 28 namespace |
| 路由檔案 | 16 個 routes_*.go |
| 中介層 | 11 個 middleware |

---

## 架構健康度評分

| 面向 | 分數 | 說明 |
|------|------|------|
| 後端分層 | **9/10** | 依賴規則零違規，handler→service→model 完全合規 |
| API 一致性 | **10/10** | 1,075 處 response helper，零 raw `c.JSON()` |
| 安全性 | **9/10** | AES-256-GCM 加密、JWT 黑名單、RBAC、審計 hash chain |
| 中介層 | **9/10** | 11 個 middleware，關注點分離良好 |
| 前端元件化 | **6/10** | 核心元件存在但抽象不足，大量頁面級重複 |
| 狀態管理 | **7/10** | Zustand 精簡，但過度依賴 useState/useEffect (1,043 處) |
| 型別安全 | **8/10** | 少量 `any`，YAML 解析處較弱 |
| 測試覆蓋 | **5/10** | 41 個測試檔 vs 333 Go + 310 前端檔，覆蓋率偏低 |
| 部署架構 | **9/10** | 多階段 Docker、non-root、健康檢查、CI/CD 完整 |
| 技術債 | **7/10** | TODO 僅 1 處，但有結構性債務需處理 |

---

## P0 — 必須修復的問題 ✅ 已完成 (2026-04-12)

### 1. Handler 中 `context.Background()` 誤用 ✅ 已修復

> 初始評估 11 處，實際修復 **73 處**（橫跨 24 個 handler 檔案）。

破壞請求取消語義，可能導致請求已斷開但 K8s API 呼叫仍在執行，浪費資源甚至造成 goroutine 洩漏。

**已修復檔案清單:**

| 檔案 | 修復數 |
|------|--------|
| `configmap_crud.go` | 6 |
| `configmap_handler.go` | 2 |
| `cluster_overview.go` + `cluster_handler.go` | 1 + callers |
| `cost_budget.go` | 1 |
| `storage_class.go` | 4 |
| `secret_crud.go` | 4 |
| `ingress_crud.go` | 6 |
| `node_operations.go` | 2 |
| `storage_pvc.go` | 6 |
| `service_crud.go` | 6 |
| `networkpolicy_handler.go` | 6 |
| `storage_pv.go` | 4 |
| `ingress_handler.go` | 4 |
| `service_handler.go` | 5 |
| `secret_handler.go` | 2 |
| `node_detail.go` | 1 |
| `node_list.go` | 2 |
| `networkpolicy_analysis.go` | 1 |
| `networkpolicy_simulate.go` | 1 |
| `networkpolicy_topology.go` | 1 |
| `ingress_converter.go` | 2 |
| `service_converter.go` | 2 |
| `multicluster_handler.go` | 2 |
| `system_setting_grafana.go` | 1 |

**修復方式**: 全部改為 `context.WithTimeout(c.Request.Context(), 30s/60s)` + `defer cancel()`

**未修改（WebSocket/串流連線，context.Background() 為正確用法）:**
- `kubectl_pod_terminal.go`, `pod_logs.go`, `log_center_stream.go`, `pod_terminal_ws.go`, `kubectl_terminal_ws.go`
- `notify_channel.go` — 同時被背景 worker 呼叫，無 gin.Context

---

### 2. 裸 error return 無上下文包裝 ✅ 已修復

> 初始評估 30+ 處，實際修復 **31 處**（3 個 service 檔案）。

違反錯誤處理 Rule 1 — 錯誤缺乏呼叫鏈上下文，debug 時無法追蹤來源。

**已修復清單:**

| 檔案 | 修復數 |
|------|--------|
| `argocd_service.go` | 25 處 — GetConfig、SaveConfig、所有 HTTP 操作方法 |
| `audit_service.go` | 3 處 — CreateSession、CloseSession、RecordCommand |
| `alertmanager_service.go` | 5 處 — GetAlertStats、GetFullReceivers、CreateReceiver、UpdateReceiver、DeleteReceiver |

**修復方式**: 全部加 `fmt.Errorf("operation context: %w", err)`

---

## P1 — 結構性技術債

### 3. 前端 App.tsx 單體 (18,276 行)

路由定義、Auth 初始化、全局狀態全塞在一個檔案。極難維護和 review。

**建議拆分**:
- `src/config/routes.ts` — 路由定義
- `src/hooks/useAuthInit.ts` — Auth 初始化邏輯
- `src/App.tsx` — 僅保留 Provider 組合和頂層 layout

---

### 4. 前端表格/表單重複模式 ⚠️ 基建完成，遷移進行中

**表格重複** — 62 個 table 實作重複以下邏輯:
- 搜尋/篩選 UI
- 分頁設定 (標準 20 items/page)
- Column 定義 + status/action rendering
- Modal/Drawer 詳情展示

**表單重複** — 17 個 form 實作重複以下邏輯:
- YAML ↔ Form 雙向同步
- Dry-run 預檢
- Diff modal 差異比對
- Labels/Annotations key-value 陣列輸入

**基建** (Phase 2 完成):
- ✅ `useTableList` hook (`ui/src/hooks/useTableList.ts`) — 客戶端搜尋/篩選
- ✅ `<TableListLayout>` (`ui/src/components/TableListLayout.tsx`) — Card + toolbar + Table
- ✅ `<FormModal>` (`ui/src/components/FormModal.tsx`) — 標準 form + modal 封裝
- ✅ `<KeyValueEditor>` (`ui/src/components/KeyValueEditor.tsx`) — key-value 陣列輸入

**已遷移頁面** (2026-04-12):

| 檔案 | 原始行數 | 遷移後 | 節省 | 套用元件 |
|------|---------|--------|------|---------|
| `access/UserManagement.tsx` | 447 | 403 | -44 | `TableListLayout` + `FormModal` ×2 |
| `audit/OperationLogs.tsx` | 646 | 646 | — | `Card variant="borderless"` ×5 |
| `audit/CommandHistory.tsx` | 570 | 570 | — | `Card variant="borderless"` ×6 |
| 23 個 pages 檔案 | — | — | -50 | `bordered={false}` → `variant="borderless"` 全域替換 |

**剩餘遷移任務** (增量進行):
- `TableListLayout` 尚有 ~59 個 table 頁面可套用（ServiceTab / IngressTab / StorageClassTab 為 Tab 元件不適用）
- `FormModal` 尚有 ~15 個 modal form 可套用（優先：HelmList, ReceiverManagement）
- `useTableList` 適用於使用客戶端搜尋的頁面（非伺服器端分頁）
- `useMultiSearch` + `MultiSearchBar` 已全面完成（第三輪遷移已完成所有剩餘 19 個檔案）

---

### 5. 前端缺失的共用元件 ⚠️ 部分完成

| 缺失元件 | 狀態 | 用途 |
|----------|------|------|
| `<FormModal>` | ✅ 已建立 | 標準 form + modal 封裝，統一 footer (Cancel/Save) |
| `<KeyValueEditor>` | ✅ 已建立 | Labels/Annotations 鍵值對輸入 |
| `<TableListLayout>` | ✅ 已建立 | 標準 table + filter + action 佈局 |
| `<YAMLDiffModal>` | ✅ 已建立 | YAML 編輯 + 變更差異比對 |
| `<RuleBuilder>` | ✅ 已建立 | NetworkPolicy / Ingress 規則建構器 |

---

### 6. 後端 God-Service 風險

單一 service 職責過重，應拆分以提升可測試性和可維護性。

| Service | 原行數 | 拆分後 | 結果 |
|---------|--------|--------|------|
| `prometheus_service.go` | 1,792 | `prometheus_service.go` (857) + `prometheus_builders.go` (174) + `prometheus_queries.go` (796) | ✅ 已拆分 |
| `om_service.go` | 1,195 | `om_service.go` (749) + `om_resource.go` (467) | ✅ 已拆分 |
| `gateway_service.go` | 1,107 | `gateway_service.go` (499) + `gateway_types.go` (177) + `gateway_converters.go` (451) | ✅ 已拆分 |

**拆分策略（同 receiver 跨檔案，不改介面）**:
- `prometheus_builders.go`: URL 建構、Auth、時間解析、Label Selector 方法
- `prometheus_queries.go`: 所有 private `query*` 及 `extractNodeName` 方法
- `om_resource.go`: `GetResourceTop`、`GetControlPlaneStatus`、`enrichComponentMetrics`、`min`
- `gateway_types.go`: GVR 變數定義 + 所有 DTO struct 定義
- `gateway_converters.go`: `to*Item` 轉換函式 + `gw*` 工具函式

---

## P2 — 改善項目

### 7. 前端大型元件需拆分 ✅ 已完成 (2026-04-13)

| 檔案 | 原行數 | 拆分後 | 提取的子元件 |
|------|--------|--------|-------------|
| `PVCTab.tsx` | 625 | ~565 | 共用 `<YamlViewModal>` + `<ColumnSettingsDrawer>` |
| `PVTab.tsx` | 612 | ~552 | 共用 `<YamlViewModal>` + `<ColumnSettingsDrawer>` |
| `ArgoCDApplicationsPage.tsx` | 666 | **469** | `<ArgoCDAppFormModal>` + `<ArgoCDAppDetailDrawer>` |
| `SecurityDashboard.tsx` | 656 | **65** | `<ImageScanTab>` + `<BenchTab>` + `<GatekeeperTab>` + `<SeverityTag>` |
| `CompliancePage.tsx` | — | — | 檔案不存在，架構報告資訊有誤 |
| `StorageClassTab.tsx` | 590 | 542 | `<StorageClassYAMLModal>` + `<StorageClassColumnSettingsDrawer>` |
| `YAMLEditor.tsx` | 654 | **424** | `<YAMLSubmitBar>` + `<YAMLEditorPane>` + `<YAMLDiffView>` |
| `NetworkPolicyForm.tsx` | — | — | `<RuleBuilder>` 已在 P1-5 完成 |
| `ConfigMapCreate.tsx` | 622 | **417** | `<ConfigMapFormBody>` |
| `SecretEdit.tsx` | 601 | **446** | `<SecretEditFormPanel>` |

**新增共用元件** (`ui/src/components/`):
- `YamlViewModal.tsx` — 泛用 YAML 預覽 Modal（PVCTab + PVTab 共用）
- `ColumnSettingsDrawer.tsx` — 泛用欄位可見性 Drawer（PVCTab + PVTab 共用）

**全部 `npx tsc --noEmit` 零錯誤。**

---

### 8. 測試覆蓋率不足

41 個測試檔 vs 整體 643+ 原始檔案，覆蓋率約 6.4%。

**優先補測試的模組** (按風險排序):

| 優先級 | 模組 | 原因 |
|--------|------|------|
| P0 | middleware/auth.go | 認證核心，錯誤影響全局 |
| P0 | middleware/permission.go | 權限控制，漏洞即安全事件 |
| P1 | services/cluster_service.go | 叢集管理核心 |
| P1 | services/user_service.go | 使用者管理 + 密碼處理 |
| P1 | pkg/crypto/ | 加密邏輯正確性 |
| P2 | 前端 EmptyState, StatusTag, PermissionGuard | 共用元件，影響全站 |
| P2 | 前端 useTableList ✅ 已建立 | 新抽象需要測試保護 |

---

### 9. 前端命名不一致 ✅ 已完成 (2026-04-13)

**已修復項目:**

| 問題 | 修復前 | 修復後 | 影響檔案 |
|------|--------|--------|---------|
| PascalCase const export | `CloudBillingService` | `cloudBillingService` | `cloudBillingService.ts` + `useCostDashboard.ts` |
| PascalCase const export | `CostService` | `costService` | `costService.ts` + `useCostDashboard.ts` + `WorkloadCostTab.tsx` |
| PascalCase const export | `EventAlertService` | `eventAlertService` | `eventAlertService.ts` + `EventAlertRules.tsx` |
| PascalCase const export | `WorkloadYamlService` | `workloadYamlService` | `workloadYamlService.ts` + `WorkloadCreateModal.tsx` + `DeploymentCreate.tsx` |
| 頁面檔小寫開頭 | `kubectlTerminal.tsx` | `KubectlTerminal.tsx` | `pages/terminal/` + `router/routes.tsx` |

**說明:**
- Service 檔案名稱全部已是 camelCase（`podService.ts`、`clusterService.ts` 等）✅
- **Class** export 保持 PascalCase（`PodService`、`StorageService` 等）— 這是 TypeScript class 命名慣例，無需修改 ✅
- **const object** export 已全數改為 camelCase
- React 元件頁面檔全部改為 PascalCase 開頭

**已確立命名規範:**
- Service const export: `camelCase` (e.g., `export const podService = {}`)
- Service class export: `PascalCase` (e.g., `export class PodService {}`)
- 列表頁: `*List.tsx`
- Tab 子頁: `*Tab.tsx`
- 表單頁: `*Form.tsx` / `*Create.tsx` / `*Edit.tsx`
- 所有 React 元件檔: PascalCase 開頭

**全部 `npx tsc --noEmit` 零錯誤。**

---

### 10. 前端效能優化機會 ✅ 已完成 (2026-04-13)

**已完成:**

| 優化項 | 實作內容 |
|--------|----------|
| ESLint strict 規則 | 啟用 `@typescript-eslint/no-explicit-any: error`、`no-unused-vars: error`、`react-hooks/exhaustive-deps: error` |
| React.memo | 已套用至 StatusTag、EmptyState、ClusterSelector、NamespaceSelector、LanguageSwitcher、SearchDropdown |
| 191 處 ESLint 違規清零 | 跨 80 個檔案：`any` → `unknown`、補 hook deps、移除未使用 import、修正 react-refresh exports |
| hook deps 正確性 | 36 個 useCallback/useEffect 補齊 `t`、`message` 等 deps；5 個迴圈風險函式加 eslint-disable + 說明 |
| `any` 型別消除 | 新增 `ResourceQuotaItem`、`LimitRangeItem`、`SyncPolicyFormValues` 等介面取代 `any[]` |

**最終結果**: `npx eslint src --max-warnings 0` ✅ 零錯誤零警告；`npx tsc --noEmit` ✅ 零型別錯誤

**待後續處理 (非緊急):**

| 問題 | 影響 | 建議 |
|------|------|------|
| 1,043 處 `useState/useEffect` | 部分元件不必要 re-render | 抽取 custom hooks，使用 `useMemo`/`useCallback` |
| PermissionContext.tsx 9,069 行 | Context re-render 影響子樹 | 拆分為多個 Context 或改用 Zustand slice |

---

## 架構優勢 (保持不變)

這些是專案已經做對的事，應作為未來開發的基準線:

1. **後端分層完美合規** — handler→service→model 依賴方向零違規
2. **API response 100% 一致** — 全部使用 response helper，結構統一
3. **安全架構成熟** — AES-256-GCM 欄位加密 + JWT 黑名單 + 審計 SHA-256 hash chain + 可插拔 KMS (env/file/Vault/AWS)
4. **Optional Component 優雅降級** — Discovery API 偵測，組件不存在時不報錯
5. **可插拔基礎設施** — Rate limiter (memory/Redis)、KMS provider、Auth backend (local/LDAP)
6. **部署就緒** — 單一二進位 + go:embed 前端、多階段 Docker、non-root、GitHub Actions CI/CD
7. **日誌合規** — 零 fmt.Println/log.Printf，全部使用結構化 logger
8. **路由集中管理** — 16 個 routes_*.go，handler 不定義路由
9. **InsecureSkipVerify 全數有 nolint 註解和正當理由**
10. **Lazy loading** — 26 個前端頁面使用 React.lazy() code splitting

---

## 實作進度追蹤

| Phase | 內容 | 狀態 | 完成日 |
|-------|------|------|--------|
| P0 / Phase 1 | context.Background() 修復 (73 處) + 裸 error wrapping (33 處) | ✅ 完成 | 2026-04-12 |
| Phase 2 | 前端基建抽取：useTableList, TableListLayout, FormModal, KeyValueEditor, routes split | ✅ 完成 | 2026-04-12 |
| P1-4 遷移 第一輪 | UserManagement FormModal+TableListLayout; 23 頁面 bordered→variant; 全域 50 處 Card 修正 | ✅ 完成 | 2026-04-12 |
| P1-4 遷移 第二輪 | useMultiSearch hook + applyMultiSearch utility + MultiSearchBar 元件; 遷移 ServiceTab / IngressTab / StorageClassTab; 修正 3 個 size="middle"→"small" | ✅ 完成 | 2026-04-12 |
| P1-4 遷移 第三輪 | 遷移剩餘 19 個檔案：6 個 domain hooks (useWorkloadTab / useNodeList / useConfigMapList / useSecretList / useNamespaceList / usePodList)；6 個 workload tabs (DeploymentTab / CronJobTab / JobTab / DaemonSetTab / StatefulSetTab / ArgoRolloutTab)；5 個 list 頁面 (NodeList / NamespaceList / PodList / SecretList / ConfigMapList)；2 個 standalone tabs (PVTab / PVCTab)；全部 TypeScript 通過 | ✅ 完成 | 2026-04-12 |
| P1-5 共用元件 | 建立 `<YAMLDiffModal>`（從 ResourceYAMLEditor 提取內嵌 diff Modal）；建立 `<RuleBuilder>`（從 NetworkPolicyForm 提取 RuleEditor）；更新兩個消費端使用新元件；TypeScript 通過 | ✅ 完成 | 2026-04-12 |
| P1-6 God-Service 拆分 | prometheus_service (1792→857+174+796)；om_service (1195→749+467)；gateway_service (1107→499+177+451)；同 receiver 跨檔案拆分，零介面變更，全部 `go build ./internal/...` 通過 | ✅ 完成 | 2026-04-12 |
| Phase 3 | 後端 God-Service 拆分 (prometheus_service, om_service) | ✅ 已完成（P1-6）| 2026-04-12 |
| P2-7 前端大型元件拆分 | PVCTab/PVTab/ArgoCD/SecurityDashboard/StorageClassTab/YAMLEditor/ConfigMapCreate/SecretEdit；新增 YamlViewModal + ColumnSettingsDrawer 共用元件；零 TS 錯誤 | ✅ 完成 | 2026-04-13 |
| P2-9 前端命名統一 | 修正 4 個 PascalCase const 服務 export（CostService/CloudBillingService/EventAlertService/WorkloadYamlService）；重命名 kubectlTerminal.tsx → KubectlTerminal.tsx；更新 6 個消費方檔案；零 TS 錯誤 | ✅ 完成 | 2026-04-13 |
| P2-10 前端效能優化 | ESLint strict 規則啟用；React.memo 套用至 6 個共用元件；191 處 ESLint 違規清零（80 個檔案）；`any`→`unknown` 型別強化；hook deps 正確性修復；零 lint 錯誤零 TS 錯誤 | ✅ 完成 | 2026-04-13 |
| Phase 4 | 品質提升：測試補齊、TypeScript strict | ⬜ 待開始 | — |

---

## 建議重構路線圖

```
Phase 1 — P0 修復 ✅ 完成 (2026-04-12)
├── ✅ context.Background() → c.Request.Context() (73 處，24 個 handler 檔)
└── ✅ 裸 error return → fmt.Errorf wrapping (33 處：argocd 25, audit 3, alertmanager 5)

Phase 2 — 前端基建 ✅ 完成 (2026-04-12)
├── ✅ 抽取 useTableList (`ui/src/hooks/useTableList.ts`)
├── ✅ 抽取 <TableListLayout> (`ui/src/components/TableListLayout.tsx`)
├── ✅ 抽取 <FormModal> (`ui/src/components/FormModal.tsx`)
├── ✅ 拆分 App.tsx 路由到 `ui/src/router/routes.tsx` + `RequireAuth.tsx`
└── ✅ 建立 <KeyValueEditor> (`ui/src/components/KeyValueEditor.tsx`)

Phase 3 — 後端拆分 (預估 2-3 天)
├── prometheus_service → QueryBuilder + MetricsAggregator
├── om_service → HealthDiagnostic + ResourceRanker
└── 補齊 middleware 測試 (auth, permission)

Phase 4 — 品質提升 (持續)
├── 補齊核心 service 測試
├── ✅ 統一前端命名規範 (P2-9 完成，2026-04-13)
├── 啟用 strict TypeScript 規則
├── ✅ 拆分大型前端元件 (P2-7 完成，2026-04-13)
└── ✅ 前端效能優化 (P2-10 完成，2026-04-13)：ESLint strict、React.memo、191 處違規清零
```

---

## 附錄: 合規檢查清單

### 後端合規 (CLAUDE.md 14-point checklist)

| # | 檢查項 | 狀態 |
|---|--------|------|
| 1 | Handler 5-step flow | ✅ 合規 — 73 處 context.Background() 已全數修復 |
| 2 | Error wrapping | ✅ 合規 — 33 處裸 return 已全數修復 |
| 3 | K8s error mapping | 合規 |
| 4 | context.WithTimeout 使用 c.Request.Context() | ✅ 合規 — 已修復 |
| 5 | DB 查詢 .WithContext(ctx) | 合規 |
| 6 | YAML output strip ManagedFields | 合規 |
| 7 | Optional component IsInstalled() | 合規 |
| 8 | InsecureSkipVerify 有 nolint | 合規 (5/5) |
| 9 | 無 username == "admin" 硬編碼 | 合規 |
| 10 | 敏感欄位不出現在 log/JSON | 合規 |
| 11 | 路由在 routes_*.go 註冊 | 合規 |
| 12 | state-changing 操作有 logger.Info | 合規 |

### 安全合規

| 檢查項 | 狀態 |
|--------|------|
| SQL Injection | 合規 — GORM 參數化查詢 |
| 硬編碼密鑰 | 合規 — 全部環境變數，生產模式阻止預設值 |
| JWT 演算法白名單 | 合規 — 僅 HS256 |
| Token 撤銷機制 | 合規 — blacklist service |
| 審計日誌完整性 | 合規 — SHA-256 hash chain |
| 欄位加密 | 合規 — BeforeSave/AfterFind hooks |
