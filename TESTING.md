# KubePolaris 測試計劃

> 版本：v1.0 | 日期：2026-04-02 | 狀態：計劃中

---

## 一、目標與原則

| 原則 | 說明 |
|------|------|
| 分層測試 | 單元 → 整合 → E2E，由快到慢，由細到粗 |
| 持續整合 | 每次 PR 自動執行單元 + 整合測試 |
| 風險優先 | 權限、加密、多叢集隔離優先覆蓋 |
| 真實依賴 | DB 用 SQLite in-memory；K8s 用 envtest 或 kind |

**覆蓋率目標**

| 層級 | 短期目標 | 長期目標 |
|------|---------|---------|
| 後端 Service 層 | 60% | 80% |
| 後端 Handler 層 | 50% | 70% |
| 前端元件 | 40% | 60% |

---

## 二、測試分層架構

```
E2E（Playwright）          ← 核心流程，跑最少
────────────────────────
Handler 整合測試           ← API 輸入/輸出驗證
────────────────────────
Service 單元測試           ← 業務邏輯，跑最多
────────────────────────
前端元件測試（Vitest）      ← 元件行為、API 格式假設
```

---

## 三、後端測試計劃

### 3.1 Service 單元測試（第一階段）

**工具：** `testing` + `testify/assert` + SQLite in-memory

| 測試檔案 | 測試目標 | 關鍵案例 | 優先級 |
|---------|---------|---------|-------|
| `services/auth_service_test.go` | JWT 生成/驗證、bcrypt | token 過期、密碼錯誤、salt 驗證 | P0 |
| `services/permission_service_test.go` | 多租戶存取控制 | 跨叢集存取被拒、admin 全權、只讀禁止寫入 | P0 |
| `services/cluster_service_test.go` | kubeconfig 加密/解密 | 金鑰輪換、格式錯誤、空 kubeconfig | P0 |
| `services/helm_service_test.go` | Helm 參數驗證 | Chart 不存在、命名空間不合法 | P1 |
| `services/user_service_test.go` | 用戶 CRUD、狀態變更 | 重複 username、停用帳號登入 | P1 |
| `services/audit_service_test.go` | 操作日誌寫入查詢 | 分頁邊界、時間範圍篩選 | P2 |

**共用測試 helper（`testutil/db.go`）：**
```go
// 每個測試開一個獨立 in-memory SQLite，測完自動關閉
func NewTestDB(t *testing.T) *gorm.DB {
    db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    db.AutoMigrate(/* all models */)
    t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
    return db
}
```

---

### 3.2 Handler 整合測試（第二階段）

**工具：** `net/http/httptest` + `gin.SetMode(gin.TestMode)`

| 測試檔案 | 覆蓋 API | 關鍵案例 | 優先級 |
|---------|---------|---------|-------|
| `handlers/auth_handler_test.go` | `POST /auth/login` | 正確登入、密碼錯誤 401、帳號停用 | P0 |
| `handlers/cluster_handler_test.go` | Cluster CRUD | 無 token 401、無權限 403、不存在 404 | P0 |
| `handlers/permission_handler_test.go` | 權限 CRUD | 非 admin 禁止修改、clusterId 不存在 | P0 |
| `handlers/helm_handler_test.go` | Helm Release CRUD | namespace 缺少 400、release 不存在 404 | P1 |
| `handlers/user_handler_test.go` | User CRUD | 非 PlatformAdmin 403、分頁參數邊界 | P1 |
| `handlers/audit_handler_test.go` | 審計查詢 | 時間範圍查詢、非 admin 403 | P2 |

**測試結構範例：**
```go
func TestListClusters_Unauthorized(t *testing.T) {
    router := setupTestRouter(t)
    w := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/api/v1/clusters", nil)
    // 不帶 Authorization header
    router.ServeHTTP(w, req)
    assert.Equal(t, 401, w.Code)
}
```

---

### 3.3 K8s 整合測試（第三階段）

**工具：** `sigs.k8s.io/controller-runtime/pkg/envtest` 或 **kind**

| 測試目標 | 工具 | 觸發時機 |
|---------|------|---------|
| Pod/Node/Deployment 列表 | envtest | PR merge 到 main |
| Helm install/upgrade/rollback | kind（真實叢集） | 每日定時 |
| Informer 快取同步 | envtest | PR merge |
| 多叢集隔離 | kind（兩個叢集） | 每日定時 |

**CI 配置（GitHub Actions）：**
```yaml
- name: Setup kind cluster
  uses: helm/kind-action@v1
  with:
    cluster_name: test-cluster

- name: Run K8s integration tests
  run: go test ./... -tags=integration -timeout 300s
```

---

## 四、前端測試計劃

### 4.1 元件單元測試

**工具：** Vitest + React Testing Library + MSW（mock API）

| 測試檔案 | 測試目標 | 關鍵案例 | 優先級 |
|---------|---------|---------|-------|
| `services/permissionService.test.ts` | API 回應解析 | `{items, total}` vs 陣列、空回應 | P0 |
| `services/helmService.test.ts` | request 不帶 `.data` | 確認直接回傳 T | P0 |
| `components/ErrorBoundary.test.tsx` | 元件錯誤捕捉 | 子元件拋錯顯示 fallback UI | P1 |
| `pages/helm/HelmList.test.tsx` | Release 列表渲染 | 空列表、loading、Status Tag 顏色 | P1 |
| `pages/access/UserManagement.test.tsx` | Table dataSource 型別 | 傳入非陣列不 crash | P1 |
| `contexts/PermissionContext.test.tsx` | 權限 context 提供 | 無權限時隱藏操作按鈕 | P2 |

**MSW handler 範例：**
```ts
// tests/mocks/handlers.ts
rest.get('/api/v1/clusters', (req, res, ctx) =>
  res(ctx.json({ items: mockClusters, total: 2 }))
)
```

---

### 4.2 E2E 測試（第四階段）

**工具：** Playwright + kind 叢集

| 流程 | 步驟 | 優先級 |
|------|------|-------|
| 登入流程 | 開啟首頁 → 輸入 admin/KubePolaris@2026 → 進入 overview | P0 |
| 叢集瀏覽 | 選叢集 → 查看 Pod 列表 → 查看 Node 詳情 | P0 |
| Helm 安裝 | 進入 Helm 頁 → 新增 Repo → 安裝 Release → 確認列表出現 | P1 |
| 權限隔離 | 用只讀帳號登入 → 確認安裝/刪除按鈕不可見 | P1 |
| 審計日誌 | 執行操作 → 進入審計頁 → 確認日誌出現 | P2 |

---

## 五、特殊風險測試項目

這些場景容易被忽略但影響大，**必須覆蓋**：

| 風險 | 測試方式 | 描述 |
|------|---------|------|
| 跨叢集權限洩漏 | 整合測試 | 用戶 A 的 token 不能存取 B 叢集的資源 |
| kubeconfig 解密失敗 | 單元測試 | 金鑰變更後舊資料應報清楚錯誤，而非 panic |
| SQLite → MySQL 行為差異 | 雙 driver CI | AutoMigrate、LIKE 查詢、JSON 欄位 |
| Helm SDK 連線 K8s 失敗 | mock restClientGetter | 叢集離線時應回 503，不應 hang |
| 大量 Pod 虛擬捲動 | 前端效能測試 | 5000 筆資料不應 freeze UI |
| JWT 過期後自動登出 | E2E | token 過期後操作應跳回登入頁 |

---

## 六、執行計劃

| 階段 | 內容 | 時程 | 完成指標 |
|------|------|------|---------|
| **P0 - 第一階段** | Auth / Permission / Cluster Service 單元測試 | 第 1-2 週 | CI 全綠，覆蓋率 ≥ 60% |
| **P1 - 第二階段** | 所有 Handler 整合測試 + 前端 service 測試 | 第 3-4 週 | 所有 API endpoint 有測試 |
| **P2 - 第三階段** | K8s envtest 整合 + 前端元件測試 | 第 5-6 週 | K8s 操作有 mock 叢集覆蓋 |
| **P3 - 第四階段** | Playwright E2E 核心流程 + kind CI | 第 7-8 週 | 5 條核心流程全自動化 |

---

## 七、CI 整合

```yaml
# .github/workflows/test.yml（目標狀態）
jobs:
  backend-unit:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./internal/services/... ./internal/handlers/... -coverprofile=coverage.out
      - run: go tool cover -func=coverage.out | grep total

  frontend-unit:
    runs-on: ubuntu-latest
    steps:
      - run: cd ui && npm test -- --coverage

  integration:
    runs-on: ubuntu-latest
    needs: [backend-unit, frontend-unit]
    steps:
      - uses: helm/kind-action@v1
      - run: go test ./... -tags=integration -timeout 300s

  e2e:
    runs-on: ubuntu-latest
    needs: integration
    if: github.ref == 'refs/heads/main'
    steps:
      - run: cd ui && npx playwright test
```

---

## 八、進度追蹤

| 測試項目 | 狀態 |
|---------|------|
| Auth Service 單元測試 | ✅ 完成（11 tests PASS） |
| Permission Service 單元測試 | ✅ 完成（17 tests PASS） |
| Cluster Service 單元測試 | ✅ 完成（9 tests PASS） |
| User Service 單元測試 | ✅ 完成（8 tests PASS） |
| Helm Service 單元測試 | ⬜ 待實作 |
| Handler 整合測試 | ⬜ 待實作 |
| permissionService.test.ts | ⬜ 待實作 |
| HelmList.test.tsx | ⬜ 待實作 |
| K8s envtest 整合 | ⬜ 待實作 |
| Playwright E2E | ⬜ 待實作 |
| CI workflow 整合 | ⬜ 待實作 |
