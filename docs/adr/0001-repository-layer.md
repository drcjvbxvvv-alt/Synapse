# ADR-0001: Repository 層導入與邊界

- 狀態: **Accepted**（2026-04-09）
- 作者: @ahern
- 相關 Phase: Phase 1, P0-4
- 相關文件: `docs/ARCHITECTURE_REVIEW.md` §一.P0-4 / §十一.2 / §十二.6

---

## Context（背景）

Synapse 目前 40 個 handler 普遍直接持有 `*gorm.DB`，違反 `CLAUDE.md` 第 1、2、3、5、6 節規定的分層原則。

核查結果（`ARCHITECTURE_REVIEW.md` §一.P0-4）：

- 40 個 handler 檔注入 `*gorm.DB`
- 150+ 處 raw GORM 呼叫（`.Where/.First/.Find/.Create/.Update/.Delete/.Save`）出現在 handler
- 僅 29 處 `WithContext` 散佈在全專案
- 84 處 `context.Background()` 出現在 handler

症狀：

1. **Context 遺失** — DB 查詢與請求生命週期脫鉤，使用者取消請求後查詢仍在跑，浪費連線。
2. **無法 mock 單元測試** — Handler / Service 單測都得起真實 DB 或大量 sqlmock 樣板。
3. **商業邏輯漏到 handler** — Service 層被繞過，同一條 query 散落在多個 handler 檔案。
4. **換 DB driver 困難** — GORM 細節全面滲透到 handler 與 service。

---

## Decision（決策）

引入 `internal/repositories/` 層作為 data-access 抽象：

### 1. 通用介面與基底實作

- `Repository[T any]` 介面：定義單一 model 的 CRUD、分頁、Transaction、WithTx、DB escape hatch。
- `BaseRepository[T]`：以 GORM 實作所有 `Repository[T]` 方法，**強制每個方法 `.WithContext(ctx)`**，domain 實作以 embedding 方式繼承。
- 錯誤抽象：`ErrNotFound` / `ErrAlreadyExists` / `ErrInvalidArgument` 三個 sentinel，不讓 handler / service 看見 `gorm.ErrRecordNotFound`。

### 2. 分層契約

```
handler → service → repository → gorm → db
```

- **handler** 不再 import `gorm.io/gorm`；錯誤以 `apierrors.AppError` 呈現。
- **service** 持有 Repository interface（而非 `*gorm.DB`），商業邏輯在此。
- **repository** 只負責資料存取，禁止含商業規則。
- **models** 仍舊是 GORM struct，repository 與之直接互動。

### 3. Feature Flag 推廣

- 新 flag：`features.FlagRepositoryLayer`（env: `SYNAPSE_FLAG_USE_REPO_LAYER`）。
- 過渡期 service 可同時持有 `*gorm.DB` 和 Repository，依 flag 切換路徑，穩定後再刪除舊路徑。
- 預期移除日期：P0-4c 完成後 2 週（P0-4 全面推廣後）。

### 4. 試點範圍（Phase 1）

Cluster / User / Permission 三個 domain：

| Domain | Repo interface | 底層 | 對應 handler |
|--------|---------------|------|-------------|
| Cluster | `ClusterRepository` | `BaseRepository[models.Cluster]` + `FindByName`、`GetConnectable` | `handlers/cluster.go` |
| User | `UserRepository` | `BaseRepository[models.User]` + `FindByUsername` | `handlers/user.go` |
| Permission | `PermissionRepository` | `BaseRepository[models.ClusterPermission]` + 多條件查詢 | `handlers/permission.go` |

### 5. 全面推廣（Phase 2，P0-4c）

剩餘 40 個 handler 依 `docs/REFACTOR_HANDLER_GUIDE.md` 的檢核表逐一改寫。

---

## Alternatives（替代方案）

| 方案 | 否決理由 |
|------|---------|
| 直接把 DB 邏輯塞進 Service，不做 Repository 層 | 違反單一職責；service 仍難以 mock；換 DB 依然不可能。 |
| 改用 ent / sqlc 重寫 DAO | 遷移成本過大；要重新生成既有 20+ 個 table 的 DAO；不在本 Phase 預算。 |
| 繼續保持現狀，純靠 code review 修正 | Review 無法阻擋新增違規，30+ 工程師協作時會失控。 |

---

## Consequences（後果）

### 正面

- ✅ Handler / Service 可用 `mockgen` / `gomock` 自動產生 mock，寫 unit test 無需 sqlmock 樣板。
- ✅ `.WithContext(ctx)` 由 `BaseRepository` 統一強制，無法遺漏。
- ✅ DB 實作（GORM → ent / sqlc / raw sql）未來可替換，call-site 只需換 DI。
- ✅ 錯誤語意統一：domain 代碼只處理 `ErrNotFound` 三個 sentinel。

### 負面

- ❌ 多一層介面，簡單 CRUD 的程式碼行數增加。
- ❌ Go generics 在某些情境（embedded generic interface + extra method）有語法限制，需要小心設計。
- ⚠ 需要 code review 守住「不要把商業邏輯漏到 Repository」的紅線。

### 風險

- 大規模改動期間若未嚴守 feature flag，可能同時存在兩條資料路徑、難以除錯 → 以 `FlagRepositoryLayer` 隔離。
- 既有 service 層的快取（如 `ClusterService.allCache`）在重構時容易忘記搬移 → 逐 domain 改寫時列入 checklist。

---

## Related Work（關聯工作）

- **P0-4a（本 ADR）**：`internal/repositories/` 骨架建立 ✅（2026-04-09）
- **P0-4b**：Cluster / User / Permission Repository 實作（排期中）
- **P0-4c**：全面推廣到 40 handler（Phase 2）
- **P1-3**：Service Interface 化，搭配 Repository 使 handler / service / repo 三層皆可獨立 mock

---

## References

- `internal/repositories/repository.go` — Repository 介面定義
- `internal/repositories/base.go` — BaseRepository 泛型實作
- `internal/repositories/errors.go` — sentinel 錯誤
- `internal/features/features.go` — Feature Flag 機制
- `docs/REFACTOR_HANDLER_GUIDE.md` — Handler 遷移檢核表
- `docs/ARCHITECTURE_REVIEW.md` §一.P0-4、§十一.2、§十二.6
