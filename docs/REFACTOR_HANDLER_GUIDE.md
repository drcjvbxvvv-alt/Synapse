# Handler 改造指南 — Repository 層遷移

- 版本：v1.2（2026-04-09）
- 適用範圍：P0-4b（Phase 1 試點 3 domain — **已完成**）／P0-4c（Phase 2 全面推廣 — **Batch 1/2 進行中**）
- 前置閱讀：
  - `docs/adr/0001-repository-layer.md` — 決策與邊界（**必讀**）
  - `docs/ARCHITECTURE_REVIEW.md` §一.P0-4、§五.5.2 動作 1、§十一.2、§十五.2
  - `CLAUDE.md` 第 2、3、5、6 節 — Handler / Service / Context / GORM 規範
- 相關程式碼：
  - `internal/repositories/repository.go` — `Repository[T]` 介面
  - `internal/repositories/base.go` — `BaseRepository[T]` 泛型實作
  - `internal/repositories/errors.go` — `ErrNotFound` / `ErrAlreadyExists` / `ErrInvalidArgument`
  - `internal/features/features.go` — `FlagRepositoryLayer`

---

## 目錄

1. [目的與 Scope](#1-目的與-scope)
2. [分層契約（修改前後）](#2-分層契約修改前後)
3. [單 Handler 遷移 12 步驟檢核表](#3-單-handler-遷移-12-步驟檢核表)
4. [標準範例：從 raw GORM 到 Repository](#4-標準範例從-raw-gorm-到-repository)
5. [Pilot 範圍（Phase 1 / P0-4b）](#5-pilot-範圍phase-1--p0-4b)
6. [Feature Flag 使用準則](#6-feature-flag-使用準則)
7. [測試需求](#7-測試需求)
8. [Code Review 紅線](#8-code-review-紅線)
9. [常見陷阱與對策](#9-常見陷阱與對策)
10. [Definition of Done](#10-definition-of-done)
11. [全量推廣清單（Phase 2 / P0-4c）](#11-全量推廣清單phase-2--p0-4c)

---

## 1. 目的與 Scope

本指南為「handler / service 直接持有 `*gorm.DB`」遷移到 Repository 層提供**可逐檔執行**的操作手冊。任何改動都必須符合 ADR-0001 所定下的四項原則：

1. Handler **不得** import `gorm.io/gorm`。
2. Service **不得** 注入 `*gorm.DB`；改注入 Repository interface。
3. Repository **不得** 包含商業規則（那是 service 的工作）。
4. `.WithContext(ctx)` 由 `BaseRepository` 統一強制，不得在 repo 以外再呼叫。

本指南**不處理**下列事項，它們由其他 ADR / PR 負責：

- Service interface 化（P1-3 / ADR-0005）
- Router 拆分（P1-2 / ADR-0004）
- 錯誤語意統一管道（§五.5.2 動作 5）

---

## 2. 分層契約（修改前後）

### 修改前（違規）

```
┌─────────┐      ┌─────────┐      ┌────────┐
│ handler │────▶│ service │────▶│  gorm  │
└────┬────┘     └─────────┘      └────────┘
     │                               ▲
     └───────────────────────────────┘
     (handler 直接碰 *gorm.DB，service 被繞過)
```

症狀：

- `handler.go` 裡有 `h.db.Where(...).Find(...)`
- `service.go` 裡有 `s.db.WithContext(ctx).First(...)`
- 兩層都寫死 GORM 細節、沒有共用查詢

### 修改後（目標）

```
┌─────────┐    ┌─────────┐    ┌─────────────┐    ┌──────┐    ┌────┐
│ handler │──▶│ service │──▶│ repository │──▶│ gorm │──▶│ db │
└─────────┘    └─────────┘    └─────────────┘    └──────┘    └────┘
```

不變性（invariants）：

- **handler** 只看 service 回傳的 DTO 或 `apierrors.AppError`，永遠不 import `gorm.io/gorm`。
- **service** 持有 `ClusterRepository`（或 interface），用 `ctx context.Context` 當第一個參數。
- **repository** 每個方法 `session(ctx)` 開頭，錯誤以 `ErrNotFound` / `ErrAlreadyExists` 對外。
- **model** 不變（仍是 GORM struct）。

---

## 3. 單 Handler 遷移 12 步驟檢核表

改一個 handler 檔案時，請逐條勾選。任何一步未過就不得 merge。

```
□  1. 讀完目標 handler + 對應 service，列出所有 DB 呼叫點
□  2. 在 internal/repositories/<domain>.go 建立 domain repo
       - struct: *BaseRepository[T] + 必要的 domain method
       - 建構子 NewXxxRepository(db *gorm.DB) *XxxRepository
□  3. Domain method 簽章：第一參數 ctx context.Context；回傳 (*T, error) 或 ([]*T, error)
□  4. Domain method 內部只呼叫 BaseRepository 或 r.DB(ctx)；不得保留 r.db
□  5. Service 新增 repo 欄位；建構子參數加入 repo
       - 過渡期：保留舊的 db 欄位，依 FlagRepositoryLayer 分流
□  6. Service 方法逐條遷移：gorm.ErrRecordNotFound → repositories.ErrNotFound
       - apierrors 回傳碼維持不變（ErrClusterNotFound 等）
□  7. Handler 移除所有 `h.db.*` 呼叫，呼叫 service 取 DTO
□  8. Handler 移除 `gorm.io/gorm` 與 `errors` 裡對 gorm 的 import
□  9. Router DI：routes_<domain>.go 把 db 改成 NewXxxRepository(db)，再傳給 service
□ 10. 新增 repo 單元測試（sqlmock）— 覆蓋 happy path + NotFound + 一個錯誤路徑
□ 11. 新增 service 單元測試（mockgen 或手寫 fake repo）— 至少覆蓋原本最重要的 3 條路徑
□ 12. 本地驗證：
       - go build ./internal/... ./pkg/... ./cmd/...
       - go vet ./internal/...
       - go test ./internal/repositories/... ./internal/services/... ./internal/handlers/...
       - golangci-lint run（若已建立）
```

---

## 4. 標準範例：從 raw GORM 到 Repository

以 **Cluster 的 `GetCluster(id)`** 為例，實戰示範一次改造。

### 改造前（handler）— 違規

```go
// internal/handlers/cluster.go
func (h *ClusterHandler) GetCluster(c *gin.Context) {
    id, _ := strconv.Atoi(c.Param("id"))
    var cluster models.Cluster
    if err := h.db.First(&cluster, id).Error; err != nil { // ❌ 在 handler 直接用 gorm
        response.NotFound(c, "cluster not found")
        return
    }
    response.OK(c, cluster)
}
```

### 改造步驟

#### (1) 新增 repo 檔案

```go
// internal/repositories/cluster_repository.go
package repositories

import (
    "context"

    "gorm.io/gorm"

    "github.com/shaia/Synapse/internal/models"
)

// ClusterRepository 聚焦 *models.Cluster 的資料存取。
type ClusterRepository struct {
    *BaseRepository[models.Cluster]
}

func NewClusterRepository(db *gorm.DB) *ClusterRepository {
    return &ClusterRepository{
        BaseRepository: NewBaseRepository[models.Cluster](db),
    }
}

// FindByName 查詢 cluster.name 唯一索引。
func (r *ClusterRepository) FindByName(ctx context.Context, name string) (*models.Cluster, error) {
    return r.FindOne(ctx, "name = ?", name)
}

// GetConnectable 僅回傳可用（is_active=true 且未軟刪）的 cluster 列表。
func (r *ClusterRepository) GetConnectable(ctx context.Context) ([]*models.Cluster, error) {
    return r.Find(ctx, "is_active = ?", true)
}
```

#### (2) Service 改為持有 repo

```go
// internal/services/cluster_service.go（節錄）
type ClusterService struct {
    repo   *repositories.ClusterRepository
    // 過渡期保留 db 用於未遷移路徑
    db     *gorm.DB
    // ...
}

func NewClusterService(db *gorm.DB, repo *repositories.ClusterRepository) *ClusterService {
    return &ClusterService{db: db, repo: repo}
}

func (s *ClusterService) GetCluster(ctx context.Context, id uint) (*models.Cluster, error) {
    if features.IsEnabled(features.FlagRepositoryLayer) {
        c, err := s.repo.Get(ctx, id)
        if err != nil {
            if errors.Is(err, repositories.ErrNotFound) {
                return nil, apierrors.ErrClusterNotFound(id)
            }
            return nil, fmt.Errorf("cluster service get: %w", err)
        }
        return c, nil
    }
    // ── 舊路徑：待 FlagRepositoryLayer 全面開啟後刪除 ──
    var c models.Cluster
    if err := s.db.WithContext(ctx).First(&c, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, apierrors.ErrClusterNotFound(id)
        }
        return nil, fmt.Errorf("cluster service get: %w", err)
    }
    return &c, nil
}
```

#### (3) Handler 去 gorm

```go
// internal/handlers/cluster.go
func (h *ClusterHandler) GetCluster(c *gin.Context) {
    id, err := parseClusterID(c.Param("id"))
    if err != nil {
        response.BadRequest(c, "invalid cluster ID")
        return
    }
    ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
    defer cancel()

    cluster, err := h.clusterService.GetCluster(ctx, id)
    if err != nil {
        if ae, ok := apierrors.As(err); ok {
            c.JSON(ae.HTTPStatus, response.ErrorBody{
                Error: response.ErrorDetail{Code: ae.Code, Message: ae.Message},
            })
            return
        }
        response.InternalError(c, err.Error())
        return
    }
    response.OK(c, cluster)
}
```

#### (4) Router 注入

```go
// internal/router/routes_cluster.go
clusterRepo := repositories.NewClusterRepository(d.db)
clusterSvc  := services.NewClusterService(d.db, clusterRepo)
clusterH    := handlers.NewClusterHandler(clusterSvc)
```

#### (5) 測試

新增 `internal/repositories/cluster_repository_test.go`（sqlmock）覆蓋 `FindByName` happy path 與 NotFound，並在 `internal/services/cluster_service_test.go` 用 fake/mocked repo 覆蓋 `GetCluster` 的兩條 flag 路徑。

---

## 5. Pilot 範圍（Phase 1 / P0-4b）

> **✅ 狀態（2026-04-09）**：三個 pilot domain 遷移完成。以下內容保留為 P0-4c 全量推廣參考；「實際經驗」區塊記錄了與原計畫的落差，P0-4c 可直接套用。

Phase 1 只動下列 3 domain，其他 handler **保持現狀**直到 Phase 2 才動。

| Domain | 對應 handler | 對應 service | 需要的 domain method | 特殊注意 |
|--------|-------------|-------------|---------------------|---------|
| **Cluster** | `internal/handlers/cluster.go`（List / Get / Import / Update / Delete） | `ClusterService` | `FindByName(name)`, `GetConnectable()`, `ListWithPermission(userID)` | 有 `allCache`（30s TTL）— 必須搬到 service 層並在 `Create/Update/Delete` 後清掉 |
| **User** | `internal/handlers/user.go`、`user_create.go` | `UserService` | `FindByUsername(username)`, `ListUsers(opts ListOptions)` | 密碼 hash 仍在 service；`PasswordHash` 欄位不得進 repo 回傳的 DTO |
| **Permission** | `internal/handlers/permission.go` | `PermissionService` | `FindByUserCluster(userID, clusterID)`, `ListByUser(userID)` | `gorm.ErrDuplicatedKey` → `ErrAlreadyExists` → `apierrors.ErrPermissionDuplicate` 的三段翻譯 |

### 為什麼挑這 3 domain？

1. **Cluster**：所有其他 handler 的前置（resolve cluster → get k8s client），改好即可驗證整條 DI 鏈。
2. **User**：純 DB，不碰 K8s，是最乾淨的 Repository 範例。
3. **Permission**：唯一會踩多表交易 + 唯一鍵衝突的 domain，用來驗證 `Transaction` 與 `ErrAlreadyExists` sentinel。

### Pilot 各 domain 的檢核要點

**Cluster**
```
□ ClusterRepository 至少實作：Get, FindByName, GetConnectable, List
□ ClusterService.allCache 清除時機：Create / Update / Delete 後
□ Kubeconfig / SAToken / CA 加密解密仍由 model BeforeSave/AfterFind 掌管
□ Handler 刪乾淨 gorm import
```

**User**
```
□ UserRepository 至少實作：Get, FindByUsername, List(opts)
□ ListOptions 使用 Where + Page + PageSize + OrderBy（不要回到手刻 offset）
□ PasswordHash 欄位：service 層負責在 DTO 轉換時剝除
□ Handler 刪乾淨 gorm import
```

**Permission**
```
□ PermissionRepository 至少實作：Get, FindByUserCluster, ListByUser
□ Transaction：service 用 repo.Transaction(ctx, fn)，不得直接 s.db.Transaction
□ gorm.ErrDuplicatedKey → ErrAlreadyExists（在 repository 內轉）
□ apierrors.ErrPermissionDuplicate 由 service 層包裝
□ Handler 刪乾淨 gorm import
```

### 5.1 Pilot 實際經驗（P0-4b 回饋，供 P0-4c 沿用）

實作三個 pilot domain 時踩到的幾個與原計畫不符之處，列在這裡讓後續批次不必重跑一次：

**(a) Context 不強制改 API 簽章**
- `GetCluster` 有 150+ 呼叫點、`PermissionService` 的多數方法有 30+ 呼叫點。一次把簽章改成吃 `ctx` 會爆量連帶改動，風險遠大於收益。
- **對策**：對外 API 暫時維持 ctx-less，service 內部用 `context.Background()` 過渡；完整 ctx propagation 列入 P0-4c 的尾聲或另開子任務。
- **P0-4c 套用**：遇到呼叫點 > 20 的方法，先跳過簽章改動，只做內部 repo 分流；最後一次性 sweep 改簽章。

**(b) Aggregate 界線要狠 — 不要跨 repo 交叉查詢**
- `PermissionService.GetUserAccessibleClusterIDs` 同時要查 `User` 與 `ClusterPermission`，但 `User` 不屬於 `PermissionRepository` aggregate。
- **對策**：User lookup **刻意留在 `s.db`**（不進 repo），只讓 permission 相關查詢走 `PermissionRepository`。Service 層混用兩條路徑沒問題，重點是 repo 介面乾淨。
- 同理 `PermissionService.ListUsers` / `GetUser` 也留在 `s.db`，那是 UserService 的事。
- **P0-4c 套用**：設計 repo 方法時先問「這個查詢的主對象屬於哪個 aggregate」，跨 aggregate 的 join/lookup 留在 service 層組合，不硬塞進 repo。

**(c) Handler 要的不是 `*gorm.DB`，是已注入的 service**
- `ClusterHandler` 原本留著 `db *gorm.DB` 只為了臨時 `New...Service(h.db)` 做一次查詢。
- **對策**：在 handler struct 中注入目標 service（本例是 `monitoringCfgSvc` + `promService`），整個 `db` 欄位可以刪。Handler 不應保留任何 `*gorm.DB`。
- **P0-4c 套用**：每個 handler 做 grep `*gorm.DB`，把倖存的用法都改成 service 注入；`NewXxxHandler` 簽章跟著瘦身。

**(d) sqlmock 的 GORM SQL 兩個固定陷阱**
- GORM 會把外層 WHERE 包一層額外括號：`WHERE (a = ?) AND deleted_at IS NULL`；多條件時則是 `WHERE ((a = ? OR b = ?)) AND ...`。測試字串要跟著加括號。
- `ORDER BY` 會補 `ASC` 關鍵字：實際 SQL 是 `ORDER BY id ASC LIMIT ?`，不是 `ORDER BY id LIMIT ?`。
- Soft-delete 的刪除是 `UPDATE SET deleted_at=?`，**不是** `DELETE FROM`。
- Preload：只要被查的結果裡有非 nil 的 FK，GORM 會自動打 preload query；mock 必須為每個被觸發的 preload 都註冊 expectation，否則 `ExpectationsWereMet()` 會炸。
- **P0-4c 套用**：寫測試先 `go test -run TestXxx -v` 看實際 SQL 再回頭改 expectation，比反覆猜寫快。

**(e) `useRepo()` helper 是必需品**
- 每個 service method 開頭都要一段「如果有 repo 且 flag 開了，走新路徑，否則走舊路徑」。重複寫 20 次後會漏一個。
- **對策**：在 service struct 加 `useRepo()` method，統一成 `if s.useRepo() { ... } else { ... }`。
```go
func (s *ClusterService) useRepo() bool {
    return s.repo != nil && features.IsEnabled(features.FlagRepositoryLayer)
}
```
- **P0-4c 套用**：新增 repo 欄位時一併加 `useRepo()`，不要用匿名 if 拼。

**(f) Repo 回傳 `[]*T`，service 常要 `[]T`**
- `BaseRepository[T].List` 回傳 `[]*T`；舊 service API 很多是 `[]T`（值型別）。每個 list 方法都要寫一段 deref loop。
- **對策**：接受這段 boilerplate，或在 service 內寫一個小 helper `deref[T any](xs []*T) []T`。pilot 沒統一收進 `internal/services/common.go`，各 service 各寫一次。
- **P0-4c 套用**：如果後面持續重複，可以考慮抽一個 generic helper；pilot 階段不足以支持抽象。

---

## 6. Feature Flag 使用準則

`FlagRepositoryLayer`（env: `SYNAPSE_FLAG_USE_REPO_LAYER`）是 pilot 期間的護欄。

### 原則

1. **Service 分流**：service 方法內部用 `features.IsEnabled(features.FlagRepositoryLayer)` 切換新舊路徑。
2. **一次一個 domain**：一個 PR 只動一個 domain，便於 rollback。
3. **預設關閉**：production 預設 `SYNAPSE_FLAG_USE_REPO_LAYER=false`（不設）。
4. **灰度驗證**：在 staging 環境先開啟一週，觀察 error rate 與 DB query 數量沒有異常才能打開 prod。
5. **過期刪除**：pilot 全部穩定後 **2 週內**，刪掉 flag 與舊路徑；過期未刪者需在 PR 描述解釋。

### 範例

```go
if features.IsEnabled(features.FlagRepositoryLayer) {
    return s.getViaRepo(ctx, id)
}
return s.getLegacy(ctx, id)
```

### 不要這樣做

```go
// ❌ 在 repository / handler 裡檢查 flag — 應該只在 service 層
if features.IsEnabled(features.FlagRepositoryLayer) { ... }

// ❌ 把 flag 寫進 config — flag 不是 config，生命週期不同
cfg.UseRepositoryLayer
```

---

## 7. 測試需求

Pilot 3 domain 的最低標準：

| 層級 | 新增測試數 | 覆蓋重點 |
|------|-----------|---------|
| Repository | 每個 domain ≥ 5 個 sqlmock 測試 | Get Success / NotFound / Count / UpdateFields / 一個 domain-specific query |
| Service | 每個 domain ≥ 5 個測試（fake repo 或 gomock） | 主要方法 happy path + NotFound + 業務規則 + 兩條 flag 路徑（舊路徑可用 sqlmock） |
| Handler | 每個 domain ≥ 3 個測試 | 200 / 400 / 404 三條主路徑，使用 `gin.CreateTestContext` |

### Fake Repository 範本（推薦做法）

若不想引入 mockgen，可以手寫 fake。例如：

```go
// internal/services/cluster_service_test.go
type fakeClusterRepo struct {
    getFn func(ctx context.Context, id uint) (*models.Cluster, error)
}

func (f *fakeClusterRepo) Get(ctx context.Context, id uint) (*models.Cluster, error) {
    return f.getFn(ctx, id)
}
// ... 其餘方法回傳 nil / zero，測試只用到 Get
```

### gomock 做法（若選擇這條路）

```
go install go.uber.org/mock/mockgen@latest
mockgen -source=internal/repositories/cluster_repository.go \
        -destination=internal/repositories/mocks/cluster_repository_mock.go \
        -package=mocks
```

**二擇一**即可，不要混用。Pilot 階段建議先手寫 fake（依賴少、學習成本低）；進入 P0-4c 若 domain 數量膨脹再考慮 gomock。

---

## 8. Code Review 紅線

PR 出現下列任一項就退件：

```
1. handler 還 import "gorm.io/gorm"
2. handler 內還有 h.db.*
3. service 直接 return gorm.ErrRecordNotFound（必須先翻譯）
4. repository method 沒有 ctx 參數
5. repository method 內部沒走 r.session(ctx) 或 r.DB(ctx)
6. repository 內寫商業邏輯（例：檢查 user role、限制資源數）
7. 漏加 FlagRepositoryLayer 分流 → 直接覆蓋舊路徑
8. 新 repo / service 檔案沒有對應測試
9. 用 context.Background() 在 handler 或 service 取代 ctx
10. 用 h.db.WithContext(context.Background()) 繞過 ctx 檢查
```

---

## 9. 常見陷阱與對策

### 陷阱 1：`gorm.ErrRecordNotFound` 漏翻譯

**症狀**：handler 收到 bare `gorm.ErrRecordNotFound`，response 變成 500。

**對策**：在 repository 層（非 service）統一翻譯為 `ErrNotFound`。`BaseRepository.Get` / `FindOne` 已經做過；自訂查詢必須自己包。

```go
func (r *ClusterRepository) FindByName(ctx context.Context, name string) (*models.Cluster, error) {
    var c models.Cluster
    err := r.DB(ctx).Where("name = ?", name).First(&c).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("find by name: %w", err)
    }
    return &c, nil
}
```

### 陷阱 2：Transaction 裡用錯 repo 實例

**症狀**：Transaction 回呼內仍用外層 `r`，寫入的資料未進入交易，一旦 rollback 就分家。

**對策**：callback 收到 `tx *gorm.DB`，透過 `repo.WithTx(tx)` 產生交易版 repo。

```go
err := s.permRepo.Transaction(ctx, func(tx *gorm.DB) error {
    txRepo := s.permRepo.WithTx(tx) // ← 必須用這個
    if err := txRepo.Create(ctx, perm); err != nil {
        return err
    }
    if err := txRepo.Update(ctx, other); err != nil {
        return err
    }
    return nil
})
```

### 陷阱 3：忘了把 `allCache` 搬家

**症狀**：Cluster 改名後，UI 顯示舊名 30 秒。

**對策**：Cache 應該貼在 service 層而非 repository（repository 不含狀態）。所有寫入型方法（Create/Update/Delete）結束後 `s.invalidateCache()`。

### 陷阱 4：忘記關 Feature Flag 舊路徑

**症狀**：舊路徑與新路徑同時存在數個月，修 bug 要改兩次。

**對策**：ADR-0001 明定「過渡期 2 週」。PR 描述必須寫出刪除日期；超過 deadline 的分支列入 tech-debt。

### 陷阱 5：Pagination 被 preload 弄亂

**症狀**：`List` 方法在 count 時多了 preload，count 變成 join 後的值而非 master row 數。

**對策**：`BaseRepository.List` 已經把 count 與 data 拆成兩條 query，不要在 domain repo 自己 join count。如一定要，用 `repo.DB(ctx).Raw(...)` escape hatch 並另外寫單元測試守住。

### 陷阱 6：Repository 吃到 handler 層業務規則

**症狀**：`ClusterRepository.GetForUser(userID, clusterID)` 裡判斷 `user.Role`。

**對策**：權限判斷是 service / middleware 的工作。Repository 只能回答「有沒有」「是什麼」，不能回答「該不該」。

---

## 10. Definition of Done

一個 domain 遷移完成，需同時滿足：

```
□ 所有 handler 方法呼叫 service，未直接碰 gorm
□ service 透過 repo 存取（舊路徑由 FlagRepositoryLayer 保護）
□ repo + service 測試覆蓋率 ≥ 60%（用 go test -cover）
□ go test ./internal/... 全過
□ go vet ./internal/... 無錯
□ go build ./internal/... ./pkg/... ./cmd/... 成功
□ 本 domain 在 staging 環境開啟 flag 驗證 ≥ 3 天
□ ARCHITECTURE_REVIEW.md §一.P0-4 進度標記更新
□ 對應 ADR-0001 Related Work 表格的 checkbox 勾選
```

---

## 11. 全量推廣清單（Phase 2 / P0-4c）

Pilot 穩定後，剩餘 37 個 handler 依下列批次推進。每批次 5~10 個檔案，一個 PR 一個批次。

> **P0-4c 現況（2026-04-09）**：Wave 1 已清除 21 個 handler 的 dead-db 欄位；Wave 2 修正 8 個 inline service 建構；以下清單為剩餘需 service 萃取的 handler。

### Batch 1：純 DB 型（優先，最易）
```
✅ permission_group.go    → permission.go (PermissionService) 已完成
✅ user_group.go          → permission.go (PermissionService) 已完成
□  platform_setting.go   → system_setting.go 仍有 1 個 h.db（getMonitoringClusters）
✅ notification.go        → NotificationService ✅ 已完成
□  notification_pref.go  → notify_channel.go 已完成（NotifyChannelService）✅
✅ audit.go / audit_ro.go → AuditService 已注入 ✅
□  feature_flag.go       → 與 features package 結合，無 handler 層 DB 直接存取
✅ siem.go               → SIEMService ✅ 已完成
✅ system_security.go    → SystemSecurityService ✅ 已完成
```

### Batch 2：K8s 讀為主 + 少量 DB（中等）
```
✅ deployment.go / statefulset.go / daemonset.go → dead-db 已清除 ✅
✅ pod.go / service.go / ingress.go              → dead-db 已清除 ✅
□  configmap.go / secret.go                     → 尚有 4 / 3 個 h.db（版本 / 憑證查詢）
✅ namespace.go / node.go / event.go / pvc.go   → dead-db 已清除 ✅
```

### Batch 3：複合型（最後，最難）
```
□ approval.go        → 14 個 h.db（審批工作流，multi-table）
□ helm.go            → 6 個 h.db（Helm repo 持久化）
□ image.go           → 5 個 h.db（映像索引）
□ log_source.go      → 6 個 h.db（日誌來源配置）
□ multicluster.go    → 10 個 h.db（跨叢集 sync policy）
□ portforward.go     → 4 個 h.db（Port-Forward 會話）
□ system_setting.go  → 1 個 h.db（monitoring clusters query）
```

Batch 3 的檔案普遍有多表交易或外部系統整合，改造時一律走 `Transaction` + `WithTx` 套路，並補 integration test（真 DB）。

---

## 附錄 A：快速參考

### 錯誤翻譯表

| gorm 原始錯誤 | repository 回傳 | service 包裝 | handler 回應 |
|--------------|----------------|-------------|-------------|
| `gorm.ErrRecordNotFound` | `ErrNotFound` | `apierrors.ErrClusterNotFound(id)` | 404 |
| `gorm.ErrDuplicatedKey` | `ErrAlreadyExists` | `apierrors.ErrPermissionDuplicate(...)` | 409 |
| `nil`（查到） | `*T, nil` | `*Info, nil` | 200 |
| 其他 | `fmt.Errorf("repository xxx: %w", err)` | `fmt.Errorf("service xxx: %w", err)` | 500 |

### Timeout 建議

| 操作 | context.WithTimeout 長度 |
|------|------------------------|
| 單列 Get | 10s |
| List / Count | 15s |
| Transaction（多表） | 30s |
| 大量寫入 batch | 60s |

### 引用路徑速查

```go
import (
    "context"
    "errors"
    "fmt"

    "gorm.io/gorm"

    "github.com/shaia/Synapse/internal/apierrors"
    "github.com/shaia/Synapse/internal/features"
    "github.com/shaia/Synapse/internal/models"
    "github.com/shaia/Synapse/internal/repositories"
)
```

---

## 版本紀錄

| 版本 | 日期 | 變更 |
|------|------|------|
| v1.0 | 2026-04-09 | 初版：對應 P0-4a 完成（interface + base + flag + ADR-0001） |
| v1.1 | 2026-04-09 | P0-4b 完成：新增 §5.1「Pilot 實際經驗」6 條回饋（context 簽章、aggregate 界線、handler db 注入、sqlmock 陷阱、useRepo helper、deref 樣板），供 P0-4c 沿用 |
| v1.2 | 2026-04-09 | P0-4c Batch 1/2 進行中：Wave 1 移除 21 個 K8s handler dead-db；Wave 2 修正 8 個 inline service 建構；Wave 3 Batch 1 建立 NotificationService、NotifyChannelService、SIEMService、SystemSecurityService（handler → service 拆離）。剩餘 9 個 handler（approval/configmap/secret/helm/image/log_source/multicluster/portforward/system_setting）屬 Batch 2/3，排入下一輪。 |
