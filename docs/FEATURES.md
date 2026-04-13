# Synapse 功能說明手冊

> 版本：v1.1 — 2026-04-12
> 目的：記錄各功能模組的用途、設計決策與使用方式，方便日後查詢。
> 範疇：P0 ~ P2 已完成功能；細粒度功能權限管理（Phase 1–3）

---

## 目錄

- [P0-1 敏感日誌防護](#p0-1-敏感日誌防護)
- [P0-2 SystemRole RBAC（角色型存取控制）](#p0-2-systemrole-rbac角色型存取控制)
- [P0-4 Repository 層 + Feature Flag 雙路徑](#p0-4-repository-層--feature-flag-雙路徑)
- [P0-5 JWT 可撤銷（jti + 黑名單）](#p0-5-jwt-可撤銷jti--黑名單)
- [P0-6 Token 記憶體儲存（XSS 防護）](#p0-6-token-記憶體儲存xss-防護)
- [P1-1 Handler 拆分（< 500 行）](#p1-1-handler-拆分-500-行)
- [P1-5 前端頁面拆分（< 700 行）](#p1-5-前端頁面拆分-700-行)
- [P1-8 Rate Limiter（記憶體 / Redis 雙後端）](#p1-8-rate-limiter記憶體--redis-雙後端)
- [P1-10 OpenTelemetry 分散式追蹤](#p1-10-opentelemetry-分散式追蹤)
- [P2-2 審計日誌雜湊鏈](#p2-2-審計日誌雜湊鏈)
- [P2-3 AES-256-GCM 欄位加密](#p2-3-aes-256-gcm-欄位加密)
- [P2-4 golang-migrate 資料庫遷移](#p2-4-golang-migrate-資料庫遷移)
- [P2-5 Zustand 前端狀態管理](#p2-5-zustand-前端狀態管理)
- [P2-6 DB-backed Feature Flag（功能旗標）](#p2-6-db-backed-feature-flag功能旗標)
- [P2-8 K8s Informer 健康檢查與自動重啟](#p2-8-k8s-informer-健康檢查與自動重啟)
- [P2-9 前端 Bundle Size 監控](#p2-9-前端-bundle-size-監控)
- [細粒度功能權限管理（Feature Policy）](#細粒度功能權限管理feature-policy)

---

## P0-1 敏感日誌防護

**問題**：`auth_service.go` 將使用者的密碼 Salt 輸出到 log，若日誌流轉至 ELK/Loki，Salt 即外洩，降低 bcrypt 抗暴破能力。

**修正**：移除所有含 `salt`、`password`、`token`、`kubeconfig` 的結構化 log key。改用：

```go
logger.Debug("authenticating local user", "username", username)
```

**規則**：CLAUDE.md §9 明文禁止在任何 log level 輸出敏感欄位。新增欄位前須先確認不含憑證語意。

---

## P0-2 SystemRole RBAC（角色型存取控制）

**問題**：原始程式碼以 `username == "admin"` 判斷超管，任何人只要竄改 DB 的 username 即可取得最高權限。

**解決方案**：

- `User` model 新增 `SystemRole string`（`user` / `platform_admin` / `system_admin`）
- `models/role.go` 定義常數與 `IsPlatformAdmin()` helper
- 資料庫啟動時 `backfillSystemRole()` 將既有 `username=admin` 升級為 `platform_admin`

**三種角色**：

| 角色 | 說明 |
|------|------|
| `user` | 一般使用者，只能操作被授權的叢集 |
| `platform_admin` | 平台管理員，可管理使用者、叢集匯入、Feature Flag |
| `system_admin` | 僅供 API Token / 內部程式使用 |

**權限判斷方式**：

```go
// ✅ 正確
if user.SystemRole == models.RolePlatformAdmin { ... }
// 或透過 middleware
middleware.PlatformAdminRequired(d.db)

// ❌ 絕對禁止
if username == "admin" { ... }
```

---

## P0-4 Repository 層 + Feature Flag 雙路徑

**問題**：40 個 handler 直接注入 `*gorm.DB`，150+ 處 raw GORM 呼叫在 handler 層，導致 context 遺失、無法 mock、測試困難。

**解決方案**：引入泛型 `Repository[T]` 介面 + `BaseRepository[T]` 實作，以 Feature Flag `USE_REPO_LAYER` 分流。

**相關檔案**：

| 檔案 | 用途 |
|------|------|
| `internal/repositories/repository.go` | 泛型 14 方法介面、`ListOptions` |
| `internal/repositories/base.go` | 所有方法強制 `.WithContext(ctx)` |
| `internal/repositories/errors.go` | `ErrNotFound` / `ErrAlreadyExists` 等 sentinel |
| `internal/repositories/cluster_repository.go` | Cluster domain 特化方法 |
| `internal/repositories/user_repository.go` | User domain + 分頁篩選 |
| `internal/repositories/permission_repository.go` | 權限/群組三張表 + 事務方法 |

**雙路徑切換**：

```bash
SYNAPSE_FLAG_USE_REPO_LAYER=true   # 啟用 Repository 路徑
# 未設定或 false → 維持舊 s.db 路徑（安全降級）
```

---

## P0-5 JWT 可撤銷（jti + 黑名單）

**問題**：原始 JWT 無 `jti`，Logout 只在前端刪 token，server 端 token 在過期前永遠有效。若 token 洩漏，唯一補救是輪換整個 JWT Secret（影響所有線上 session）。

**解決方案**：

1. JWT Claims 新增 `jti`（UUID v4）、`iss`、`aud`、`nbf`、`iat`、`system_role`
2. `TokenBlacklist` 表：`jti (unique)` + `expires_at (index)` + `revoke_reason`
3. `TokenBlacklistService`：`sync.Map` 本地快取（熱路徑無 DB 查詢）+ DB 持久化；`warmCache` 啟動時預熱
4. `AuthRequired` middleware：驗證簽名算法白名單（HS256）+ `iss`/`aud` + 查黑名單

**Logout 流程**：

```
前端 DELETE /auth/logout
  → AuthRequired middleware 驗證 token、將 jti 寫入 gin context
  → AuthHandler.Logout 呼叫 blacklistSvc.Revoke(jti, exp, userID, TokenRevokeReasonLogout)
  → 後續任何使用此 token 的請求均返回 401
```

**注意**：Refresh Token 雙 Token 機制列為 Phase 1 待辦。

---

## P0-6 Token 記憶體儲存（XSS 防護）

**問題**：`localStorage.getItem('token')` — 任何 XSS 漏洞可直接竊取 token。

**解決方案**：

- `ui/src/services/authService.ts` 的 `tokenManager` 改為**模組作用域記憶體變數**
- `getToken()` / `setToken()` / `removeToken()` 不再碰 localStorage
- 所有原本讀 localStorage 的地方（`aiService.ts`、`logService.ts`、各 Terminal 元件）改用 `tokenManager.getToken()`

**取捨**：頁面重新整理後 token 消失，需重新登入。正式 httpOnly cookie + Refresh Token 方案列為 Phase 1。

---

## P1-1 Handler 拆分（< 500 行）

**問題**：最大 handler 檔案 `rollout.go` 達 1,339 行，混雜 struct、CRUD、ops、converters，難以維護。

**拆分策略**：

| 子檔案後綴 | 內容 |
|-----------|------|
| `*_handler.go` | Handler struct + constructor + DTOs |
| `*_crud.go` | Create / Update / Delete 操作 |
| `*_ops.go` | 狀態變更操作（scale、restart、rollback）|
| `*_converters.go` | K8s object → API response 轉換 |
| `*_helpers.go` | 輔助函式 |
| `*_related.go` | 子資源操作 |

**結果**：134 個 handler 檔案，平均 223 行/檔，最大 477 行（`resource_yaml_apply.go`）。

---

## P1-5 前端頁面拆分（< 700 行）

**問題**：`CostDashboard.tsx` 1,417 行，混雜資料抓取、狀態、UI、業務邏輯。

**拆分模式**：

```
pages/cost/
  CostDashboard.tsx          ← 主頁面（< 300 行）
  columns.tsx                ← TanStack Table column 定義
  constants.ts               ← 常數與型別
  hooks/
    useCostData.ts           ← React Query 資料層
  tabs/
    OverviewTab.tsx          ← 各 Tab 子元件
```

所有 > 700 行頁面均已拆分完成，目前 `ui/src` 無任何檔案超過 700 行。

---

## P1-8 Rate Limiter（記憶體 / Redis 雙後端）

**問題**：`middleware/rate_limit.go` 把登入失敗次數存在 Go 程序內的 `map[string]*loginAttempt`。

**為何 In-Memory 不跨實例**：

假設後端部署 3 個 Pod：

```
攻擊者連續嘗試密碼 → Pod A 記錄 3 次失敗
下一個請求落到 Pod B → Pod B 的 counter = 0
下一個請求落到 Pod C → Pod C 的 counter = 0
```

每個 Pod 各自計數，攻擊者只需輪流打不同 Pod 就能繞過鎖定。**單 Pod 永遠不會觸發鎖定閾值**。

**解決方案**：

```
RateLimiter (interface)
  ├── MemoryRateLimiter  ← 預設，單節點可用，無外部依賴
  └── RedisRateLimiter   ← 多節點部署時使用，共享計數
```

**Redis Key Schema**：
- `rl:fail:{ip}` — 失敗次數（TTL = 鎖定視窗）
- `rl:lock:{ip}` — 鎖定狀態（TTL = 鎖定持續時間）

**Fail-open 設計**：Redis 不可用時自動降級為 In-Memory，並記錄 `logger.Warn`，不中斷服務。

**環境變數**：

```bash
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0
RATE_LIMITER_BACKEND=redis   # 預設 memory
```

**何時需要 Redis**：
- 正式環境多 Pod 部署 → 必須用 Redis
- 開發 / 單節點 → In-Memory 即可

---

## P1-10 OpenTelemetry 分散式追蹤

**問題**：只有 Prometheus metrics，當一個 API 請求跨越 K8s API + DB + Prometheus + ArgoCD 多個系統，無法定位哪一段慢。

**解決方案**：導入 OpenTelemetry SDK，自動為每個請求建立 trace。

**元件**：

| 元件 | 說明 |
|------|------|
| `internal/tracing/tracing.go` | `Setup()` 初始化 TracerProvider；`Shutdown()` 優雅關閉 |
| Gin middleware | `otelgin.Middleware()` 自動 span 每個 HTTP 請求 |
| GORM plugin | `otelgorm.NewPlugin()` 自動 span 每次 DB 查詢 |
| `internal/tracing/http.go` | `NewHTTPClient()` 外部 HTTP 自動注入 `traceparent` header |

**Exporter**：OTLP gRPC → Jaeger / Grafana Tempo（使用者自行部署）

**Fail-open**：OTLP 連線失敗時自動使用 noop provider，不影響服務。

**環境變數**：

```bash
OTEL_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
OTEL_SERVICE_NAME=synapse
OTEL_SAMPLING_RATE=0.1    # 正式建議 0.1，開發可設 1.0
```

**預設關閉**（`OTEL_ENABLED=false`），啟用需搭配本機 Jaeger 或 Grafana Tempo。

---

## P2-2 審計日誌雜湊鏈

**問題**：`OperationLog` 只是普通記錄，管理員若直接修改 DB 可以竄改或刪除稽核記錄，無法追責。

**解決方案**：SHA-256 雜湊鏈（類似 blockchain 概念）。

**運作原理**：

```
Log[n].hash = SHA256(Log[n-1].hash + Log[n].content)
```

每一筆記錄的 hash 依賴前一筆，若任何中間記錄被篡改，後續所有 hash 均失效，可被偵測。

**Schema**（已加入 `001_baseline.up.sql` 的 `audit_logs`）：

```sql
prev_hash varchar(64) NOT NULL DEFAULT '',
hash      varchar(64) NOT NULL DEFAULT '',
KEY idx_audit_logs_hash (hash)
```

**Feature Flag**：`SYNAPSE_FLAG_ENABLE_AUDIT_HASHCHAIN=true`

---

## P2-3 AES-256-GCM 欄位加密

**問題**：原始只有 `Cluster.KubeconfigEnc / CAEnc / SATokenEnc` 加密，其他含敏感資訊的欄位儲存明文。

**已加密的欄位範圍**：

- `LDAPConfig.BindPassword`
- `NotifyChannel.Token` / `SMTPPassword` / `SlackWebhook`
- `SIEMWebhookConfig.Secret`
- `AIConfig.APIKey`
- `ArgoCDConfig.Token`
- `CloudBillingConfig.Credentials`
- `LogSource` 憑證、`SystemSetting` 敏感 blob

**加密機制**：GORM `BeforeSave` / `AfterFind` hook，應用層透明加解密，DB 只儲存密文。

**主金鑰管理**：

```bash
# 生成 32 bytes hex key
openssl rand -hex 32

# 設定到環境變數（唯一需要在環境中管理的主金鑰）
ENCRYPTION_KEY=<64 個十六進位字元>
```

**注意**：`ENCRYPTION_KEY` 一旦遺失，加密欄位永久無法解密。務必備份。

---

## P2-4 golang-migrate 資料庫遷移

**問題**：`gorm.AutoMigrate` 無版本控制，無法回滾，欄位變更不可靠。

**解決方案**：MySQL 生產環境改用 `golang-migrate/migrate/v4`，嵌入式 SQL 檔案。

**Migration 檔案結構**：

```
internal/database/migrations/mysql/
  001_baseline.up.sql       ← 完整 schema（CREATE TABLE IF NOT EXISTS）
  001_baseline.down.sql
  002_audit_hash_chain.up.sql   ← no-op（欄位已在 baseline）
  003_feature_flags.up.sql  ← 建表 + 預填 6 個旗標
  004_namespace_budget.up.sql
  005_cluster_alertmanager.up.sql   ← no-op（欄位已在 baseline）
```

**重要陷阱 — multiStatements 批次執行**：

`golang-migrate` 使用 `multiStatements=true` DSN 時，若某個 migration 檔案含多條 SQL，**所有語句在同一個 `ExecContext` 呼叫中批次執行**。已知行為：migration 001 執行時，後續 migration 002~005 的 SQL 也可能跟著跑（透過 `go-sql-driver` 的 multi-statement 批次）。

**解決策略**：
1. 把所有初始 schema 集中在 `001_baseline.up.sql`
2. 後續 migration 若欄位已在 baseline，改為 no-op（`SELECT 1;`）
3. 新增欄位用獨立 migration 檔案，內容盡量單純（一條 ALTER TABLE）

**DSN 分離**：

```go
// 一般 GORM 連線（不開 multiStatements）
db, _ = gorm.Open(mysql.Open(dsn), ...)

// golang-migrate 專用連線（開 multiStatements）
migDSN = dsn + "&multiStatements=true"
RunMigrations(driver, migDSN)
```

---

## P2-5 Zustand 前端狀態管理

**問題**：全域 UI 狀態（目前叢集、主題、側邊欄）散落在多個 React context，Provider 巢狀層數過多，debug 困難。

**解決方案**：引入 `zustand ^5.0.12`，三個獨立 store：

| Store | 檔案 | 管理內容 |
|-------|------|---------|
| `useSessionStore` | `store/useSessionStore.ts` | 登入使用者 reactive 狀態 |
| `useClusterStore` | `store/useClusterStore.ts` | `activeClusterId` + clusters 清單 |
| `useUIStore` | `store/useUIStore.ts` | `sidebarCollapsed`（persist 到 localStorage）|

**使用方式**：

```ts
// 讀取
const { activeClusterId } = useClusterStore()

// 更新
useClusterStore.getState().setActiveClusterId(id)
```

**PermissionContext 不動**：依 R3-4 風險評估，改寫成本高收益低，新程式碼改用 Zustand stores。

---

## P2-6 DB-backed Feature Flag（功能旗標）

**問題**：上線新功能時沒有逐步放量機制，出問題只能整版 rollback。

**解決方案**：DB 為後端的 Feature Flag，帶 30s TTL 快取。

**優先順序（由高到低）**：
1. 環境變數 `SYNAPSE_FLAG_<FLAG_NAME>=true|false`
2. DB `feature_flags` 表（管理員透過 API 更新，30s TTL）
3. 程式預設值

**已知旗標**：

| 旗標名稱 | 說明 |
|---------|------|
| `use_repo_layer` | P0-4：啟用 Repository 層 |
| `use_split_router` | P1-2：啟用拆分路由模組 |
| `enable_otel_tracing` | P1-10：啟用 OpenTelemetry 追蹤 |
| `use_redis_ratelimit` | P1-8：啟用 Redis rate limiter |
| `use_zustand_store` | P2-5：啟用 Zustand 前端狀態 |
| `enable_audit_hashchain` | P2-2：啟用審計日誌 SHA-256 雜湊鏈 |

**管理 API**（需 `platform_admin`）：

```
GET  /api/v1/system/feature-flags        ← 列出所有旗標
PUT  /api/v1/system/feature-flags/:key   ← 更新旗標值
```

**快取策略**：`DBStore.IsEnabled()` 優先讀記憶體（< 30s）；`PUT` 觸發 `Invalidate()` 使下次請求立即回源。

---

## P2-8 K8s Informer 健康檢查與自動重啟

**問題**：informer 啟動後若叢集 API 壞掉或 RBAC 變更，可能 silently 停擺，前端拿到舊資料。

**解決方案**：

1. `ClusterInformerManager.HealthCheck()` — 純記憶體讀取，曝露各叢集 informer 狀態
2. `/readyz` 整合 informer 狀態（`k8s_informers.total` / `k8s_informers.synced`）
3. `StartHealthWatcher(interval=1m, stuckThreshold=5m)` — 背景 goroutine：

```
每分鐘掃描：started=true && synced=false && age > 5m
  → 呼叫 StopForCluster() + EnsureForCluster() 重啟
```

**範圍說明**：auto-restart 針對「啟動但 5 分鐘未同步」場景（初始 sync 卡住）。已完成同步的 informer 若連線中斷，client-go 本身會自動 reconnect，Synapse 無需額外介入。

---

## P2-9 前端 Bundle Size 監控

**問題**：無 bundle size budget，引入新套件導致首屏變慢而無人知曉。

**解決方案**：

**分析工具**：

```bash
npm run build:analyze   # 產出 dist/stats.html（互動式 treemap）
npm run bundle-size     # 檢查各 chunk 是否超出預算
TOTAL_BUDGET_MB=10 npm run bundle-size  # 自訂預算
```

**預設預算**：
- 總體 < 12 MB
- 非 vendor chunk < 3 MB

**Lazy Loading**：11 個重量頁面改為 `React.lazy` + `Suspense`：

| 頁面 | 原因 |
|------|------|
| `YAMLEditor` | monaco editor（~2MB） |
| `KubectlTerminalPage` | xterm.js（~1MB）|
| `MonitoringCenter`、`CostDashboard` | 大量 chart 套件 |
| `ArgoCDApplicationsPage`、`HelmList` | 非高頻頁面 |

**Chunk 分割策略**（`vite.config.ts`）：`vendor` / `antd` / `monaco` / `charts` / `i18n` / `query` / `router` 七個獨立 chunk，利用瀏覽器快取。

---

## 細粒度功能權限管理（Feature Policy）

> 規劃書：`docs/FEATURE_PERMISSION_PLAN.md`

**問題**：原有「類型-命名空間」粗粒度模型無法針對特定功能（Terminal、匯出、AI 助手）做差異化限制，所有同類型使用者獲得完全相同的功能集合。

**解決方案**：在 `ClusterPermission` 上疊加一層 Feature Policy，讓 Platform Admin 可針對每個權限記錄額外收緊功能開關。

### 架構

```
最終有效功能 = permission_type 基礎 ceiling ∩ feature_policy 允許集合
```

Feature Policy **只能收緊**，永遠無法賦予超出基礎類型上限的功能。

### 後端（Phase 1–2）

| 元件 | 說明 |
|------|------|
| `internal/models/feature_policy.go` | 18 個 feature key 常數、`FeatureCeilings` 對照表、`ComputeAllowedFeatures()` |
| `ClusterPermission.FeaturePolicy` | `TEXT NULL` 欄位，JSON 格式，NULL = 使用預設值 |
| `MyPermissionsResponse.AllowedFeatures` | 後端預算完整有效集合，前端直接使用 |
| `GET /permissions/cluster-permissions/:id/features` | 取得 ceiling / policy / effective（Platform Admin） |
| `PATCH /permissions/cluster-permissions/:id/features` | 更新 feature policy，ceiling 過濾在後端強制執行 |
| `007_feature_policy.up.sql` | `ALTER TABLE cluster_permissions ADD COLUMN feature_policy TEXT NULL` |

**Feature Key 分類**：

| 類別 | Keys |
|------|------|
| 工作負載 | `workload:view` / `workload:write` |
| 網路 | `network:view` / `network:write` |
| 儲存 | `storage:view` / `storage:write` |
| 節點 | `node:view` / `node:manage` |
| 設定 | `config:view` / `config:write` |
| 終端機 | `terminal:pod` / `terminal:node` |
| 可觀測性 | `logs:view` / `monitoring:view` |
| Helm | `helm:view` / `helm:write` |
| 工具 | `export` / `ai_assistant` |

### 前端（Phase 3）

| 元件 | 說明 |
|------|------|
| `MyPermissionsResponse.allowed_features` | 新增 `string[]` 欄位，由後端填充 |
| `PermissionContextType.hasFeature()` | `hasFeature(key, clusterId?) => boolean` |
| `PermissionProvider` | `hasFeature` 實作：查 `allowed_features` 是否包含 key |

**使用方式**：

```tsx
const { hasFeature } = usePermission();

// 隱藏 AI 助手（當功能被 feature policy 禁止時）
{hasFeature('ai_assistant') && <AIChatButton />}

// 隱藏 Pod 終端機
{hasFeature('terminal:pod') && <TerminalButton />}
```

### 向後相容

- `feature_policy` 欄位為 NULL 時，`ComputeAllowedFeatures` 直接回傳該 `permission_type` 的完整 ceiling，現有使用者體驗不受影響
- `allowed_features` 為空陣列時，前端 `hasFeature()` 回傳 `false`，安全降級

---

## 附錄：環境變數快速參考

完整說明見 `.env.example`。常用的功能相關變數：

```bash
# 安全
JWT_SECRET=<強隨機值>
ENCRYPTION_KEY=<openssl rand -hex 32>
SYNAPSE_ADMIN_PASSWORD=<初始管理員密碼>

# Rate Limiter
REDIS_ADDR=127.0.0.1:6379
RATE_LIMITER_BACKEND=redis          # memory（預設）| redis

# OpenTelemetry
OTEL_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
OTEL_SAMPLING_RATE=0.1

# Feature Flag 覆蓋（優先於 DB 設定）
SYNAPSE_FLAG_USE_REPO_LAYER=true
SYNAPSE_FLAG_ENABLE_AUDIT_HASHCHAIN=true
SYNAPSE_FLAG_USE_REDIS_RATELIMIT=true
SYNAPSE_FLAG_ENABLE_OTEl_TRACING=true
```
