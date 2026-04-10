# Contributing to Synapse

感謝你對 Synapse 的貢獻！本文件說明如何提交 PR、撰寫程式碼，以及如何讓你的貢獻順利合入主線。

---

## 目錄

1. [快速開始](#1-快速開始)
2. [分支命名規範](#2-分支命名規範)
3. [Commit 訊息規範](#3-commit-訊息規範)
4. [Pull Request 流程](#4-pull-request-流程)
5. [Code Review Checklist](#5-code-review-checklist)
6. [後端開發規範摘要](#6-後端開發規範摘要)
7. [前端開發規範摘要](#7-前端開發規範摘要)
8. [測試規範](#8-測試規範)
9. [安全規範](#9-安全規範)

---

## 1. 快速開始

```bash
# 1. Fork + Clone
git clone https://github.com/shaia/Synapse.git
cd Synapse

# 2. 啟用 pre-commit hook（P0 安全保護）
git config core.hooksPath .githooks

# 3. 安裝開發工具
go install github.com/securego/gosec/v2/cmd/gosec@latest  # 安全掃描
GOTOOLCHAIN=local go install github.com/swaggo/swag/cmd/swag@v1.16.3  # API 文件
go install github.com/air-verse/air@latest  # 後端熱重載

# 4. 啟動開發環境
make dev-mysql       # 啟動 MySQL
make dev-air         # 後端熱重載（修改 .go 自動重建）
cd ui && npm run dev # 前端開發伺服器

# 5. 提交前執行全量驗證
make check           # go test + go vet + gosec + grep 安全檢查
```

---

## 2. 分支命名規範

```
feat/<short-description>     新功能
fix/<short-description>      Bug 修正
refactor/<short-description> 重構（無功能變更）
test/<short-description>     新增或修正測試
docs/<short-description>     文件更新
chore/<short-description>    工具、依賴、CI/CD 變更
```

範例：`feat/node-taint-management`、`fix/ws-econnreset`

---

## 3. Commit 訊息規範

遵循 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <subject>

<body>（可選，解釋 why，不是 what）

Co-Authored-By: Name <email>
```

**type** 選項：`feat` `fix` `refactor` `test` `docs` `chore` `perf` `ci`

**scope** 範例：`auth` `cluster` `node` `frontend` `ws` `swagger`

**規則：**
- subject 使用祈使句（"add X"，不是 "added X"）
- 不超過 72 字元
- Breaking change 在 body 加 `BREAKING CHANGE: <description>`

---

## 4. Pull Request 流程

1. **建立 PR** 前先確認：
   - `make check` 全綠（go test + go vet + gosec + grep 安全檢查）
   - `make swag` 若有修改 API endpoint
   - 新功能須附測試（service ≥ 30%、handler ≥ 20% 是 Phase 1 目標）

2. **PR 標題** 格式與 commit 相同：`feat(auth): add refresh token endpoint`

3. **PR 描述** 必填：
   - 功能說明（What）
   - 實作原因（Why）
   - 測試方式（How to test）
   - 截圖（UI 變更時）

4. **Review 標準**：至少 1 位 reviewer approve 才能 merge

5. **Merge 策略**：Squash merge（保持 main 歷史整潔）

---

## 5. Code Review Checklist

Reviewer 請逐項確認，標記 ✅ 後才 approve：

### 後端

```
□ handler 遵循 5 步流程：parse → cluster → ctx → K8s/service → response
□ 所有 error 都用 fmt.Errorf("operation: %w", err) 包裝
□ K8s error 用 k8serrors.IsNotFound / IsForbidden 等正確分類
□ 無 context.Background() 在 handler（改用 c.Request.Context()）
□ DB 查詢都有 .WithContext(ctx)
□ 新增路由在 internal/router/routes_*.go，不在 handler 內
□ 敏感欄位（token, kubeconfig, password）有 json:"-" tag
□ 無 username == "admin" 硬編碼
□ 無 InsecureSkipVerify: true 未加 #nosec 注解
□ K8s optional component 用 IsInstalled() 偵測，不直接假設存在
□ 每個 state-changing 操作入口有 logger.Info()
□ 有對應 swagger 注解（@Summary @Tags @Router）
```

### 前端

```
□ 所有間距/顏色使用 token.xxx（不硬編碼）
□ Form label 是純文字，說明放 tooltip
□ 刪除操作使用 Popconfirm，不用 Modal.confirm
□ 狀態顯示使用 <StatusTag status={...} />
□ 空狀態使用 <EmptyState />
□ 所有面向使用者的文字使用 t()（i18n）
□ WebSocket 使用 tokenManager.getToken()，不直接讀 localStorage
□ 正確處理 WS onclose/onerror（顯示錯誤訊息）
□ API 呼叫不多套一層 .data（response.OK 直接回傳 T）
```

### 安全

```
□ 無敏感資訊寫入 log（token / kubeconfig / password / salt）
□ 用戶輸入用於 K8s label selector 前有 labels.Parse() 驗證
□ 新增 endpoint 有權限檢查（ClusterAccessRequired / PlatformAdminRequired）
□ gosec 掃描無新 HIGH/CRITICAL issue
```

---

## 6. 後端開發規範摘要

完整規範見 `CLAUDE.md`（或 `.claude/CLAUDE.md`）。

| 規則 | 說明 |
|------|------|
| Handler 5 步流程 | parse → cluster → ctx → K8s → response |
| Service 方法簽章 | `func (s *XxxService) Method(ctx context.Context, ...) (T, error)` |
| 錯誤包裝 | `fmt.Errorf("operation: %w", err)` |
| DB 查詢 | 一律帶 `.WithContext(ctx)` |
| 路由 | 在 `internal/router/routes_*.go`，不在 handler |
| 新 handler | 不注入 `*gorm.DB`，走 Service 層 |
| 新 service | 在 `internal/services/interfaces.go` 定義 interface |

---

## 7. 前端開發規範摘要

完整規範見 `ui/CLAUDE.md`（或 `ui/.claude/CLAUDE.md`）。

| 規則 | 說明 |
|------|------|
| 設計 token | `const { token } = theme.useToken();` |
| Form | `layout="vertical"`，label 純文字 |
| 刪除確認 | `<Popconfirm>` |
| 狀態顯示 | `<StatusTag status={...} />` |
| 空狀態 | `<EmptyState />` |
| Token 存取 | `tokenManager.getToken()`，不直接讀 localStorage |
| API 回應解包 | `response.OK()` 直接回傳 `T`，前端不需 `.data` |

---

## 8. 測試規範

### 後端

```bash
# 執行全部測試
go test ./...

# 查看覆蓋率報告
go test -coverprofile=coverage.out ./internal/services/...
go tool cover -html=coverage.out
```

**測試檔案命名**：`internal/services/xxx_service_test.go`、`internal/handlers/xxx_test.go`

**測試套件模板（sqlmock）**：

```go
type XxxServiceTestSuite struct {
    suite.Suite
    db      *gorm.DB
    mock    sqlmock.Sqlmock
    service *XxxService
}

func (s *XxxServiceTestSuite) SetupTest() {
    db, mock, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
    gormDB, _ := gorm.Open(mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true}),
        &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
    s.db, s.mock, s.service = gormDB, mock, NewXxxService(gormDB)
}

func (s *XxxServiceTestSuite) TearDownTest() {
    assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func TestXxxServiceTestSuite(t *testing.T) { suite.Run(t, new(XxxServiceTestSuite)) }
```

**Phase 1 覆蓋率目標**：
- Service 層 ≥ 30%（`go test -cover ./internal/services/...`）
- Handler 層 ≥ 20%（`go test -cover ./internal/handlers/...`）

### 前端

```bash
cd ui && npm run test
```

---

## 9. 安全規範

- **不提交**：`.env`、kubeconfig、token、private key
- **不 log**：token、password、kubeconfig、salt
- **不使用** `InsecureSkipVerify: true` 未加 `// #nosec G402`
- **不使用** `username == "admin"` 判斷超管，改用 `user.SystemRole`
- **pre-commit hook** 已自動攔截上述違規

發現安全問題請私下聯絡 maintainer，不要直接開 issue。

---

## 問題 / 討論

歡迎在 GitHub Issues 提問，或參考 `docs/ARCHITECTURE_REVIEW.md` 了解目前技術債與改善計畫。
