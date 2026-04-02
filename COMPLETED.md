# Synapse 已完成計劃記錄

> 版本：v1.3 | 最後更新：2026-04-02
> 本文件記錄所有已完成或已決策的規劃項目，供歷史參考。
> 進行中項目請見 [PLANNING.md](./PLANNING.md)

---

## 目錄

1. [已完成里程碑](#1-已完成里程碑)
2. [§8.3 強化優先序矩陣完成記錄](#2-83-強化優先序矩陣完成記錄)
3. [已修復缺陷](#3-已修復缺陷)
4. [已決策（不實作）](#4-已決策不實作)
5. [已完成新功能規劃](#5-已完成新功能規劃)

---

## 1. 已完成里程碑

### Milestone 1：安全強化 ✅（2026-04-02）

- [x] AES-256-GCM 憑證加密（`pkg/crypto` + Cluster GORM hooks，`ENCRYPTION_KEY` env 控制）
- [x] JWT secret 強制驗證（release 模式使用預設值 → `logger.Fatal` 強制退出）
- [x] Login rate limiting + 帳號鎖定（`middleware/rate_limit.go`，5次/分鐘，鎖定15分鐘）
- [x] WebSocket Origin 驗證（所有 6 個 WS handler 已使用 `middleware.IsOriginAllowed()`）
- [x] RequestID middleware（`middleware/request_id.go`，注入 `X-Request-ID`）

**新增 / 修改檔案：**
`pkg/crypto/crypto.go`、`internal/models/cluster.go`（GORM hooks）、`main.go`（`crypto.Init`）、`internal/config/config.go`（`ENCRYPTION_KEY`、release 模式 JWT 強制）、`internal/middleware/rate_limit.go`、`internal/middleware/request_id.go`、`internal/router/router.go`

---

### Milestone 2：穩定性與效能 ✅（2026-04-02）

- [x] Informer 同步超時可設定化（`INFORMER_SYNC_TIMEOUT` env，預設 30 秒）
- [x] 叢集指標 fallback 邏輯（`fetchPodStats()` 從 K8s API 取得實際 Pod 計數）
- [x] SQLite WAL 啟用（`_journal_mode=WAL` 已在 `database.go` 啟用）
- [x] 完整 API 分頁（permission 列表改 response.List，cluster 修正假分頁）
- [x] React Query 導入（@tanstack/react-query v5，QueryClientProvider，OperationLogs + PodList 遷移）
- [x] 大型列表虛擬捲動（16 個 Table 加入 `virtual` + `scroll.y=600`）

**新增 / 修改檔案：**
`internal/k8s/manager.go`（`SetSyncTimeout`）、`internal/services/cluster_service.go`（`fetchPodStats`）、`internal/database/database.go`（WAL DSN）

---

### Milestone 3：可觀測性 ✅（2026-04-02）

- [x] Prometheus `/metrics` endpoint（`middleware/metrics.go`，API 延遲直方圖、請求計數器）
- [x] 健康檢查深化（`/readyz` 真實 DB ping，`/healthz` liveness）
- [x] 結構化日誌（`pkg/logger` 改用 slog，`LOG_FORMAT=json|text` env 切換）
- [x] 稽核日誌完整查詢（`GetAuditLogs` 委派 OperationLogService.List，支援完整篩選）
- [x] 錯誤碼化（`internal/apierrors` 套件，AppError 攜帶 HTTPStatus + Code；Service 層回傳結構化錯誤）

---

### Milestone 4：Helm Release 管理 ✅（2026-04-02）

- [x] Helm SDK 整合（`helm.sh/helm/v3` v3.14.4 + `k8s.io/cli-runtime`）
- [x] Release 列表 / 詳情頁（含 Status Tag 顏色、namespace 篩選）
- [x] 安裝 / 升級 / 回滾 / 刪除（REST API + 前端 Modal）
- [x] Chart Repository 管理（CRUD + DB 持久化）
- [x] Values 查看（user values / all values）

---

### Milestone 5：AI 診斷 + CRD + NetworkPolicy + Event 告警 ✅（2026-04-02）

- [x] AI 診斷 UI（Pod / Workload 詳情頁「AI 診斷」按鈕，`ai:diagnose` 事件驅動浮動面板）
- [x] 多 AI Provider 設定頁（OpenAI / Azure OpenAI / Anthropic Claude / Ollama）
- [x] CRD 自動發現與通用列表（`handlers/crd.go` + 動態客戶端；`CRDList` / `CRDResources` 前端頁面）
- [x] NetworkPolicy 管理介面（`handlers/networkpolicy.go` + 動態 CRUD；`NetworkPolicyTab`；三語 i18n）
- [x] Event 告警規則引擎（`models/event_alert.go` + `services/event_alert_service.go` + `handlers/event_alert.go`；後台 Worker 每 60 秒掃描；Webhook / DingTalk 通知；30 分鐘去重）
- [x] NetworkPolicy 視覺化拓撲圖（ReactFlow v12 + Dagre 佈局）
- [x] NetworkPolicy 規則建立精靈（3 步驟精靈）

---

### Milestone 6：資源成本分析 ✅（2026-04-02）

> **目標：** 讓多租戶叢集的資源費用透明化，提供命名空間/工作負載級別的成本分攤依據。

- [x] `CostConfig` / `ResourceSnapshot` 資料模型與 AutoMigrate（`internal/models/cost.go`）
- [x] `CostWorker`（每日 00:05 UTC，從 Prometheus 查詢 CPU/Mem request + usage，按命名空間聚合後存快照；無 Prometheus 則跳過）
- [x] REST API 8 支端點（`GET/PUT config`、`overview`、`namespaces`、`workloads`、`trend`、`waste`、`export`）
- [x] 前端成本儀表板 5 個 Tab（總覽卡 × 4、命名空間 Bar Chart + 排行表、工作負載分頁表 + 利用率進度條、6 個月趨勢 Line Chart、浪費識別表）
- [x] 定價設定 Modal（CPU 單價 / 記憶體單價 / 幣別 USD/TWD/CNY/JPY）
- [x] CSV 匯出（`GET /cost/export?month=YYYY-MM`，Content-Disposition attachment）
- [x] 三語 i18n（zh-TW / zh-CN / en-US，`cost.json`）
- [x] 安裝 `recharts` 圖表庫；`MainLayout.tsx` 新增 `DollarOutlined` 側邊欄入口

---

### Milestone 7：AI 深度運維 ✅（2026-04-02）

> **目標：** 從「AI 輔助診斷」升級為「AI 主動運維助手」，支援自然語言查詢與 YAML 生成。

- [x] 敏感資料過濾（`internal/services/ai_sanitizer.go`：Secret data/stringData 值、含 password/token/key 的 env var 值、PEM 憑證 → `[REDACTED]`）
- [x] NL Query 端點（`POST /clusters/:id/ai/nl-query`；AI 解析自然語言 → 選擇並呼叫最適工具 → AI 摘要；`internal/handlers/ai_nlquery.go`）
- [x] YAML 生成助手（AI Chat 系統 Prompt 新增 YAML 生成指示；`/yaml` 前綴觸發；`AIChatMessage.tsx` 自動偵測 yaml 程式碼區塊並顯示「複製 YAML」與「套用至叢集」按鈕）
- [x] Runbook 知識庫（`internal/runbooks/runbooks.json`，10 個常見場景嵌入二進位；`GET /ai/runbooks?reason=OOMKilled` 支援關鍵字搜尋）
- [x] Runbook 自動附加（`AIChatPanel.tsx` 在 AI 診斷回應後偵測 OOMKilled / CrashLoopBackOff 等關鍵字，自動呼叫 Runbook API 並以 Collapse 展開步驟）
- [x] AI Chat 系統 Prompt 改為繁體中文；前端 `/query` 指令開啟 NL Query Modal
- [x] `AIChatInput.tsx` 新增指令快捷標籤（`/yaml`、`/query`）與輸入提示

---

### Milestone 9：合規性與安全掃描 ✅（2026-04-02）

> **目標：** 提供叢集安全基線評估，協助企業滿足 SOC2 / 等保合規要求。

- [x] `ImageScanResult` / `BenchResult` 資料模型
- [x] Trivy 映像掃描整合（exec 模式，非同步 goroutine）
- [x] 非同步掃描任務管理（觸發 → 輪詢狀態 → 結果儲存）
- [x] CIS kube-bench 評分（在叢集建立 Job → 解析輸出 → 儲存評分）
- [x] Gatekeeper 違規統計儀表板（利用 CRD 介面，dynamic client）
- [x] 前端安全儀表板（三分頁：漏洞掃描 / CIS 基準 / Gatekeeper）
- [x] 三語 i18n（zh-TW / en-US / zh-CN）

**新增檔案：** `internal/models/security.go`、`internal/services/trivy_service.go`、`internal/services/bench_service.go`、`internal/services/gatekeeper_service.go`、`internal/handlers/security.go`、`ui/src/pages/security/SecurityDashboard.tsx`

---

## 2. §8.3 強化優先序矩陣完成記錄

### Phase A（2026-04-02）：4/4 項 ✅

| 功能 | 狀態 | 主要檔案 |
|------|------|---------|
| HPA CRUD（A1） | ✅ | `internal/handlers/hpa.go`、`ScalingTab.tsx` |
| YAML Apply Diff 顯示（A2） | ✅ | `YAMLEditor.tsx`、`ResourceYAMLEditor.tsx`（Monaco DiffEditor） |
| Slack / Teams 通知（A3） | ✅ | `event_alert_service.go`（Slack text + Teams Adaptive Card） |
| Argo Rollouts 操控（A4） | ✅ | `rollout.go`（Promote/PromoteFull/Abort/GetAnalysisRuns）、`RolloutDetail.tsx` |

### Phase B（2026-04-02）：3/3 項 ✅

| 功能 | 狀態 | 主要檔案 |
|------|------|---------|
| Loki / Elasticsearch 實際查詢整合（B1） | ✅ | `services/loki_service.go`、`services/elasticsearch_service.go`、`handlers/log_source.go`、`LogCenter.tsx` |
| ConfigMap / Secret 版本歷史（B2） | ✅ | `models/config_version.go`、`handlers/configmap.go`、`handlers/secret.go`、`ConfigMapDetail.tsx`、`SecretDetail.tsx` |
| ResourceQuota / LimitRange CRUD（B3） | ✅ | `handlers/namespace.go`（+8 handlers）、`NamespaceDetail.tsx` |

### Phase C（2026-04-02）：4/4 項 ✅

| 功能 | 狀態 | 主要檔案 |
|------|------|---------|
| 部署審批工作流（C1） | ✅ | `models/approval.go`（NamespaceProtection + ApprovalRequest）、`handlers/approval.go`、`ApprovalCenter.tsx` |
| 跨叢集統一工作負載視圖（C2） | ✅ | `handlers/cross_cluster.go`、`CrossClusterWorkloads.tsx` |
| VPA 支援（C3） | ✅ | `handlers/vpa.go`（dynamic client）、`ScalingTab.tsx` VPA Card |
| Image Tag 全域搜尋（C4） | ✅ | `models/image_index.go`、`handlers/image.go`、`ImageSearch.tsx` |

### Phase D（2026-04-02）：5/6 項 ✅

| 功能 | 狀態 | 主要檔案 |
|------|------|---------|
| Port-Forward 管理（D1） | ✅ | `models/portforward.go`、`handlers/portforward.go`（SPDY）、`PortForwardPanel.tsx` |
| Project 多租戶模型（D2） | ❌ 跳過 | 架構層面升級，需獨立 Sprint（4 週工作量） |
| Deployment 保護機制（D3） | ✅ | `ProtectedConfirm.tsx`（受保護命名空間確認 Modal） |
| PDB 管理（D4） | ✅ | `handlers/pdb.go`（policy/v1）、`PDBPanel.tsx` |
| Terminal 會話錄製回放（D5） | ✅ | `SessionReplay.tsx`（基於 TerminalCommand 紀錄逐步播放） |
| 稽核日誌 SIEM 匯出（D6） | ✅ | `models/siem.go`、`handlers/siem.go`（Webhook + JSON 批次匯出）、`SIEMConfig.tsx` |

---

## 3. 已修復缺陷

### 安全性缺陷（全部已修復）

| 編號 | 問題 | 修復方式 |
|------|------|---------|
| S1 | 叢集憑證明文儲存 | `pkg/crypto`（AES-256-GCM），Cluster GORM BeforeSave/AfterFind hooks |
| S2 | JWT Secret 使用預設值警告但未強制 | `config.go` release 模式呼叫 `logger.Fatal`（`os.Exit(1)`） |
| S3 | WebSocket Origin 驗證不完整 | 所有 6 個 WS handler 已使用 `middleware.IsOriginAllowed()` |
| S4 | Rate Limiting 未實作 | `middleware/rate_limit.go`，IP + 用戶名雙維度，5次/分鐘，鎖定15分鐘 |

### 功能缺陷（部分已修復）

| 編號 | 問題 | 狀態 |
|------|------|------|
| F1 | 叢集指標回傳硬編碼 0 | ✅ 部分修復：`fetchPodStats()` 取得真實 Pod 數量；CPU/MEM 仍需 Prometheus（保留 TODO） |
| F3 | Informer 快取同步超時過短 | ✅ 修復：`INFORMER_SYNC_TIMEOUT` env，預設 30 秒 |
| F4 | 無請求追蹤 ID | ✅ 修復：`middleware/request_id.go`，自動注入 UUID v4 |

### 架構技術債（已完成部分）

| 編號 | 問題 | 狀態 |
|------|------|------|
| A4 | SQLite 不支援並行寫入 | ✅ 確認：`_journal_mode=WAL&_foreign_keys=on` 已在 database.go 啟用 |

---

## 4. 已決策（不實作）

| 功能 | 決策日期 | 理由 |
|------|---------|------|
| OAuth2 / OIDC 整合 | 2026-04-02 | 現有 LDAP + 本地帳號已滿足目標使用情境；OIDC 引入的複雜度超過收益。如需 SSO，未來評估直接接入 K8s OIDC（kube-apiserver `--oidc-*`） |
| SAML 支援 | 2026-04-02 | OIDC 已不實作，SAML 同步排除 |
| ZIP 備份匯出 | 2026-04-02 | M16 GitOps 落地後，Git 即是備份來源，ZIP 匯出需求消失 |
| Project 多租戶模型（D2） | 2026-04-02 | 架構層面升級，需獨立 Sprint（4 週工作量），延後處理 |

---

## 5. 已完成新功能規劃

### §5.1 Helm Release 管理 → ✅ M4 完成

Release 列表、安裝/升級/回滾/刪除、Chart Repository 管理、Values 查看。

### §5.2 NetworkPolicy 視覺化管理 → ✅ M5 部分完成

- NetworkPolicy CRUD ✅
- 流量規則視覺化拓撲圖（ReactFlow + Dagre） ✅
- 規則建立精靈（3 步驟） ✅
- 規則衝突檢測 ← 未實作（不在 M5 範圍）
- 拓撲圖內聯編輯 + 策略模擬 ← 移至 M11（待實作）

### §5.3 資源成本分析 → ✅ M6 完成

### §5.4 K8s Event 告警規則 → ✅ M5 完成

### §5.5 AI 能力升級 → ✅ M7 完成

敏感資料過濾、NL Query、YAML 生成助手、Runbook 知識庫、Runbook 自動附加。

### §5.6 CRD 通用管理介面 → ✅ M5 完成

### §5.8 合規性與安全掃描 → ✅ M9 完成

Trivy 映像掃描、kube-bench CIS 基準、Gatekeeper 違規統計。
