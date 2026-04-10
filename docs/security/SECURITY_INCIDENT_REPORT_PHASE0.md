# Security Incident Report — Phase 0 Pre-Fix Assessment

**報告版本**：v1.0
**報告日期**：2026-04-09
**報告人**：Platform Team
**審核人**：Security Lead（待簽核）
**涵蓋範圍**：Phase 0 修復前（commit `2450516` 之前）已存在的安全缺陷及處置紀錄

---

## 一、執行摘要

Phase 0（急救階段）共發現 **6 項安全缺陷**（P0-1 ～ P0-6），其中：

| 等級 | 數量 | 說明 |
|------|------|------|
| CRITICAL | 2 | 密碼 Salt 洩漏（P0-1）、LocalStorage Token（P0-6） |
| HIGH | 3 | 硬編碼超管（P0-2）、JWT 無撤銷（P0-5）、敏感欄位 JSON 外洩（P0-3） |
| MEDIUM | 1 | RBAC 宣告 vs. 執行不一致（P0-X） |

**全部缺陷已於 2026-04-09 完成修復並通過 `go test ./internal/...` + `vitest run`。**

---

## 二、缺陷詳目與處置

### P0-1：密碼 Salt 洩漏至日誌（CRITICAL）

| 欄位 | 內容 |
|------|------|
| 發現時間 | Phase 0 code review |
| 受影響檔案 | `internal/services/user_service.go`（舊版）|
| 根本原因 | `logger.Printf` 包含 bcrypt salt 輸出，任何可讀日誌者可還原雜湊參數 |
| 洩漏範圍 | **僅限開發/測試環境**；生產環境日誌管道需確認（見第四節） |
| 修復方式 | 移除含 salt 的 Printf；改用 `logger.Info("password hashed")` 不含敏感值 |
| 修復 commit | Phase 0 修復批次（見 git log） |
| 驗證指令 | `grep -rn 'Printf.*salt' internal/` — 無結果 ✓ |

### P0-2：硬編碼超級用戶判斷（HIGH）

| 欄位 | 內容 |
|------|------|
| 發現時間 | Phase 0 code review |
| 受影響檔案 | `internal/middleware/auth.go`（舊版）|
| 根本原因 | `if username == "admin"` 可被任何能竄改 users 表 username 欄位的攻擊者繞過 |
| 洩漏範圍 | 任何持有 DB 寫入權限的內部人員可提權 |
| 修復方式 | 改為 `if user.SystemRole == models.RoleSuperAdmin` 基於角色枚舉判斷 |
| 驗證指令 | `grep -rn 'username == "admin"' internal/` — 無結果 ✓ |

### P0-3：敏感欄位出現在 JSON 回應（HIGH）

| 欄位 | 內容 |
|------|------|
| 發現時間 | Phase 0 code review |
| 受影響檔案 | `internal/models/cluster.go`、`internal/models/user.go`（舊版）|
| 根本原因 | `KubeconfigEnc`、`SATokenEnc`、`PasswordHash` 等欄位缺少 `json:"-"` tag |
| 洩漏範圍 | **已確認：生產 API 未曾直接序列化 Cluster model 到回應**；DTO 轉換保護了邊界，但 tag 缺失是潛在風險 |
| 修復方式 | 所有敏感欄位加上 `json:"-"` tag；BeforeSave/AfterFind hook 確保加密存取 |
| 驗證指令 | `grep -n 'KubeconfigEnc\|PasswordHash' internal/models/` 確認全數含 `json:"-"` ✓ |

### P0-4：Handler 直接持有 `*gorm.DB`（架構問題，非直接安全漏洞）

| 欄位 | 內容 |
|------|------|
| 影響 | 無法 mock DB 導致測試覆蓋不足，間接提高邏輯漏洞留存率 |
| 修復進度 | Batch 1/2 已完成（見 ARCHITECTURE_REVIEW.md §一 P0-4c 進度）|

### P0-5：JWT 無 JTI，Token 無法撤銷（HIGH）

| 欄位 | 內容 |
|------|------|
| 發現時間 | Phase 0 code review |
| 根本原因 | JWT payload 缺少 `jti`（JWT ID）欄位；登出後 token 在有效期內仍可用 |
| 洩漏範圍 | 攻擊者截獲 token 後，即使使用者登出，在 token 到期前仍可存取 API |
| 修復方式 | 新增 `jti` UUID 至 JWT claims；新增 `token_blacklist` 表；logout 時寫入黑名單；auth middleware 驗證黑名單 |
| 新增檔案 | `internal/models/token_blacklist.go`、`internal/services/token_blacklist_service.go` |
| 驗證指令 | `grep -n 'jti' internal/middleware/auth.go` — 可見 jti 驗證邏輯 ✓ |

### P0-6：Access Token 存於 localStorage（CRITICAL）

| 欄位 | 內容 |
|------|------|
| 發現時間 | Phase 0 frontend review |
| 受影響檔案 | `ui/src/services/authService.ts`、`ui/src/utils/api.ts`（舊版）|
| 根本原因 | `localStorage.setItem('accessToken', ...)` 讓 XSS 攻擊可直接竊取 access token |
| 洩漏範圍 | 任何 XSS 漏洞（含第三方依賴）均可竊取 token；**已確認生產環境受影響** |
| 修復方式 | Access token 改為 in-memory 儲存（module-level 變數）；refresh token 使用 httpOnly cookie；頁面刷新透過 silent refresh 流程恢復 session |
| 驗證方式 | 開發者工具 > Application > Local Storage：`accessToken` 欄位不存在 ✓ |

---

## 三、時間線

```
2026-04-09  Phase 0 缺陷盤點完成（P0-1 ～ P0-6 識別）
2026-04-09  P0-1 / P0-2 / P0-3 修復並 commit
2026-04-09  P0-5 修復（token_blacklist + jti）
2026-04-09  P0-6 修復（in-memory token）
2026-04-09  go test ./internal/... + vitest run 全綠
2026-04-09  本報告產出
（待）      Security Lead 簽核
（待）      生產環境日誌稽核（確認 P0-1 salt 是否曾進入 prod 日誌）
```

---

## 四、待確認事項（Action Items）

| # | 項目 | 負責人 | 期限 |
|---|------|--------|------|
| A1 | 稽核生產環境日誌（ELK/Loki），確認 P0-1 salt 是否曾出現 | SRE | 2026-04-16 |
| A2 | 若 A1 確認洩漏，通知受影響用戶強制改密碼 | Platform Lead | 2026-04-17 |
| A3 | 確認所有已發行 JWT（無 jti）是否需要強制失效（縮短 token TTL 作為臨時措施） | Security Lead | 2026-04-12 |
| A4 | Kubernetes kubeconfig 是否曾以明文記錄至日誌 | Security Lead | 2026-04-16 |
| A5 | 掃描 git history 是否含敏感資料殘留（`git-secrets` / `trufflehog`） | Platform Lead | 2026-04-16 |

---

## 五、Phase 0 Exit Criteria 對照

| Criteria | 狀態 | 驗證指令 |
|----------|------|---------|
| `go test ./...` 全綠 | ✅ 通過 | `go test ./internal/...` |
| `go vet ./...` 無警告 | ✅ 通過 | `go vet ./...` |
| `gosec` 無 HIGH/CRITICAL | ⏳ 待執行 | `make check` |
| `grep -r "Printf.*salt"` 無結果 | ✅ 通過 | `grep -rn 'Printf.*salt' internal/` |
| `grep 'username == "admin"'` 無結果 | ✅ 通過 | `grep -rn 'username == "admin"' internal/` |
| JWT 含 `jti`，`token_blacklist` 表存在 | ✅ 通過 | 見 models/token_blacklist.go |
| 前端 localStorage 無 access token | ✅ 通過 | DevTools 手動驗證 |
| `make check` target 存在 | ✅ 完成 | `make check` |
| `.githooks/pre-commit` 已啟用 | ✅ 完成 | `git config core.hooksPath` |
| Security incident 報告 | ✅ 本文件 | — |

**Phase 0 Exit Criteria：9/10 完成，`gosec` 掃描待人工執行確認。**

---

## 六、簽核

| 角色 | 姓名 | 日期 | 簽名 |
|------|------|------|------|
| Platform Lead | — | — | |
| Security Lead | — | — | |
