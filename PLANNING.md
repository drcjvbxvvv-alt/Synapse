# Synapse 系統規劃書

> 版本：v1.5 | 日期：2026-04-06 | 狀態：進行中
> 已完成項目請見 [COMPLETED.md](./COMPLETED.md)

---

## 目錄

1. [系統現況總覽](#1-系統現況總覽)
2. [待解決技術債](#2-待解決技術債)
3. [邊界天花板分析](#3-邊界天花板分析)
4. [待實作優化](#4-待實作優化)
5. [待實作功能](#5-待實作功能)
   - [5.17 工作負載內嵌 Prometheus 指標](#517-工作負載內嵌-prometheus-指標sprint2-週-待實作)
   - [5.18 cert-manager 憑證管理](#518-cert-manager-憑證管理sprint2-週-待實作)
   - [5.19 彈性伸縮深化（KEDA / Karpenter / CAS）](#519-彈性伸縮深化sprint3-週-待實作)
   - [5.20 策略與合規深化（Kyverno / PSA / RBAC）](#520-策略與合規深化sprint3-週-待實作)
   - [5.21 叢集運維工具箱](#521-叢集運維工具箱sprint2-週-待實作)
   - [5.22 VolumeSnapshot + Velero 深化](#522-volumesnapshot--velero-深化sprint2-週-待實作)
   - [5.23 臨時偵錯容器 UI](#523-臨時偵錯容器ephemeral-debug-containers-uisprint1-週-待實作)
   - [5.24 映像安全深化（Trivy + Falco）](#524-映像安全深化sprint2-週-待實作)
   - [5.25 YAML 自動回滾機制](#525-yaml-自動回滾機制sprint2-週-待實作)
6. [里程碑規劃](#6-里程碑規劃)
7. [平台演進方向：全能 CI/CD DevOps 平台](#7-平台演進方向全能-cicd-devops-平台)
8. [附錄](#8-附錄)

---

## 1. 系統現況總覽

Synapse 是以 Go 1.25（Gin）+ React 19（Ant Design）構建的企業級 Kubernetes 多叢集管理平台。後端以單一二進位檔嵌入前端靜態資源，支援 SQLite（開發）與 MySQL 8（生產）雙資料庫，整合 Prometheus / Grafana / AlertManager / ArgoCD，提供 Web Terminal（Pod Exec、kubectl、Node SSH）。

**目前實作的主要功能：**

| 領域 | 功能 |
|------|------|
| 叢集管理 | 多叢集匯入（kubeconfig / Token）、健康狀態、總覽指標 |
| 工作負載 | Deployment / StatefulSet / DaemonSet / Job / CronJob / Argo Rollouts |
| 自動擴縮 | HPA CRUD、VPA 支援（動態 client）、PDB 管理 |
| 設定管理 | ConfigMap / Secret CRUD + 版本歷史 + 回滾 |
| 網路管理 | Service / Ingress CRUD、NetworkPolicy CRUD + 拓撲圖 + 建立精靈 |
| 儲存管理 | PVC / PV / StorageClass |
| 命名空間 | 建立、ResourceQuota / LimitRange CRUD、刪除、保護機制 |
| 使用者 RBAC | 多租戶、叢集 / 命名空間粒度、LDAP 整合 |
| 監控告警 | Prometheus 指標、Grafana 儀表板、AlertManager、K8s Event 告警規則 |
| GitOps | ArgoCD 應用管理與同步、Argo Rollouts 操控 |
| 日誌 | 操作日誌、Web Terminal 指令稽核、Loki / Elasticsearch 外部查詢、SIEM 匯出 |
| 全域搜尋 | 跨叢集資源搜尋、跨叢集工作負載視圖、Image Tag 全域搜尋 |
| AI 運維 | AI 診斷、多 Provider、NL Query、YAML 生成、Runbook 自動附加 |
| 安全 | AES-256-GCM 加密、Rate Limiting、Login 鎖定、WebSocket Origin 驗證 |
| 稽核 | 操作稽核日誌、Terminal 會話回放、部署審批工作流、SIEM Webhook 推送 |
| 成本 | 資源成本分析（Prometheus + fallback）、CSV 匯出 |
| 合規 | Trivy 映像掃描、CIS kube-bench、Gatekeeper 違規統計 |
| Port-Forward | 後端 SPDY tunnel、活躍 session 管理 |
| CI/CD | Helm Release 管理 |
| 國際化 | zh-TW、en-US、zh-CN |

---

## 2. 待解決技術債

> **所有技術債已於 2026-04-03 完成，本章節保留供記錄。** 詳細說明見 [COMPLETED.md](./COMPLETED.md#4-已修復缺陷)。

---

## 3. 邊界天花板分析

### 3.1 規模上限

| 維度 | 目前天花板 | 根本原因 | 改善方向 |
|------|-----------|---------|---------|
| **叢集數量** | ~20 個 | 每叢集建立獨立 Informer（記憶體 O(n) 增長），Goroutine 洩漏風險 | Informer 池化 + Lazy 初始化 + 閒置叢集 GC |
| **單叢集 Pod 數** | ~5,000 個 | Informer 全量快取於記憶體，列表頁一次回傳 | 分頁快取 + 伺服器端分頁 |
| **並行 Web Terminal** | ~50 個 | 每個 Terminal 佔用 goroutine + WebSocket 連線 | 連線池 + 心跳管理 + 閒置超時 |
| **Log 串流** | 依 K8s API 上限 | 直接 proxy K8s log stream，無緩衝 | 引入 log 中間緩衝層（如 Loki） |
| **並行 API 請求** | ~200 QPS | 無 rate limit，K8s client 無連線池設定 | 限流 + K8s client 連線池調優 |
| **資料庫規模** | SQLite ~1GB / MySQL 無硬限 | 操作日誌、稽核日誌無分區 | 日誌表按月分區 + 資料保留策略 |

### 3.2 功能邊界

| 功能領域 | 現有邊界 | 說明 |
|---------|---------|------|
| **CI/CD Pipeline** | 無 | 依賴外部 ArgoCD，無原生 Pipeline |
| **多租戶隔離** | 命名空間粒度 | 無跨叢集租戶策略（Project 概念待實作） |
| **叢集生命週期** | 無 | 不支援叢集佈建（僅匯入已有叢集） |
| **成本分析** | CPU/MEM 請求量 | 實際用量需 Prometheus；無 PVC 成本 |
| **備份還原** | 無 | 無 etcd 備份 / Velero 整合（M10 延後） |

---

## 4. 待實作優化

> **所有效能優化已於 2026-04-03 完成。** 詳細說明見 [COMPLETED.md](./COMPLETED.md#6-已完成效能優化)。

---

## 5. 待實作功能

> **M11 已於 2026-04-03 完成。** 詳細說明見 [COMPLETED.md](./COMPLETED.md#m11networkpolicy-拓撲內聯編輯--策略模擬--2026-04-03)。

---

### 5.2 多叢集工作流程（M8，5 週）

> **目標：** 打通叢集間協作壁壘，支援工作負載遷移與配置同步。

**待實作任務：**

| 任務 | 檔案 | 週次 |
|------|------|------|
| `SyncPolicy` 資料模型 | `internal/models/sync_policy.go` | W1 |
| 配置同步 API（CRUD + 觸發） + Worker | `internal/services/sync_service.go` | W1–W2 |
| 工作負載遷移後端邏輯（取 YAML → 目標叢集 Apply） | `internal/handlers/workload_migrate.go` | W2–W3 |
| 遷移精靈前端（3 步驟：選叢集 → 資源檢查 → 確認執行） | `ui/src/pages/cluster/MigrateWizard.tsx` | W3–W4 |
| 配置同步管理前端（策略 CRUD + 手動觸發 + 歷史紀錄） | `ui/src/pages/cluster/SyncPolicies.tsx` | W4–W5 |
| 三語 i18n | — | W5 |

**資料模型：**
```go
type SyncPolicy struct {
    ID              uint
    Name            string
    SourceClusterID uint
    SourceNamespace string
    ResourceType    string  // "ConfigMap" / "Secret"
    ResourceNames   string  // JSON 陣列
    TargetClusters  string  // JSON 陣列（叢集 ID）
    ConflictPolicy  string  // "overwrite" / "skip"
    Schedule        string  // Cron 表達式，空字串表示手動
    LastSyncAt      time.Time
}
```

**完成指標：** 可將 staging 叢集的 Deployment 遷移到 production 叢集；ConfigMap 同步至 3 個叢集成功率 100%。

---

> **M12 已於 2026-04-03 完成。** 詳細說明見 [COMPLETED.md](./COMPLETED.md#m12service-mesh-視覺化istio-2026-04-03)。

---

### 5.4 備份與 Velero 整合（M10 附加，延後至 M16 後）

> **決策：** ZIP 匯出已移除（GitOps 取代）。Velero 附加整合保留，待 M16 完成後評估。

- [ ] Velero 安裝偵測（`GET /clusters/:id/backup/velero-status`）
- [ ] Backup/Restore CRD CRUD（複用 CRD 通用介面）
- [ ] 前端備份狀態頁

---

### 5.5 CLI 工具（延後至 M16 後重新規劃）

> **理由：** M13（CI Pipeline）、M14（Git 整合）、M16（GitOps）完成前，CLI 核心使用場景（`pipeline run`、`deploy`、`env promote`）尚未存在，現在設計必然大幅重工。

**技術方案：** `cobra` + `viper`，獨立 Go 二進位，`~/.synapse/config.yaml`
**估計工作量（M16 後）：** 4 週

---

### 5.7 安全設定 Tab 完善（小型 Sprint，2 週）

> **目標：** 將現有分散的安全功能整合至「系統設定 → 安全設定」Tab，補充缺失的安全管理功能，消除佔位符狀態。

**現況：**
- `安全設定` Tab（`SystemSettings.tsx:67-79`）目前顯示「功能開發中」佔位符。
- `SIEMConfig.tsx` 已完整實作，但**未掛載**至任何 Tab，無法從 UI 存取。
- 登入鎖定閾值、Session TTL、密碼最低長度等安全參數目前**硬碼**於後端。

**待實作任務：**

| 任務 | 檔案 | 週次 |
|------|------|------|
| 建立 `SecuritySettings.tsx`，掛載至安全設定 Tab | `ui/src/pages/settings/SecuritySettings.tsx` + `SystemSettings.tsx` | W1 |
| 接入現有 `SIEMConfig.tsx`（Section 1：稽核日誌推送） | `ui/src/pages/settings/SecuritySettings.tsx` | W1 |
| 登入安全設定後端 API | `internal/handlers/system_security.go` + `internal/models/system_config.go` | W1 |
| 登入安全設定前端（Section 2：Session / 鎖定 / 密碼政策） | `ui/src/pages/settings/SecuritySettings.tsx` | W1–W2 |
| API Token 管理後端（CRUD + SHA-256 hash 儲存） | `internal/handlers/api_token.go` + `internal/models/api_token.go` | W2 |
| API Token 管理前端（Section 3：Token 列表 + 建立 Modal + 撤銷） | `ui/src/pages/settings/SecuritySettings.tsx` | W2 |
| 補齊三語 i18n（zh-TW / en-US） | `ui/src/locales/*/settings.json` | W2 |

**安全設定 Tab 結構（`SecuritySettings.tsx`）：**

```
SecuritySettings.tsx
├── Section 1：SIEM / 稽核日誌推送
│   └── 複用現有 SIEMConfig.tsx（Webhook URL、認證 Header、批次匯出）
│
├── Section 2：登入安全
│   ├── Session 逾時（分鐘，預設 480）
│   ├── 登入失敗鎖定閾值（次數，預設 5）
│   ├── 鎖定持續時間（分鐘，預設 30）
│   └── 密碼最短長度（預設 8）
│
└── Section 3：API Token 管理
    ├── Token 列表（名稱、建立時間、最後使用時間、到期日、權限範圍）
    ├── 建立 Token（Modal：名稱 + Scopes + 到期日 → 僅顯示一次明文）
    └── 撤銷 Token（二次確認）
```

**資料模型：**

```go
// internal/models/system_config.go（擴充現有 SystemConfig 或新增欄位）
type SystemSecurityConfig struct {
    SessionTTLMinutes      int `json:"session_ttl_minutes"`       // 預設 480
    LoginFailLockThreshold int `json:"login_fail_lock_threshold"` // 預設 5
    LockDurationMinutes    int `json:"lock_duration_minutes"`     // 預設 30
    PasswordMinLength      int `json:"password_min_length"`       // 預設 8
}

// internal/models/api_token.go
type APIToken struct {
    ID         uint       `gorm:"primaryKey"`
    UserID     uint       `gorm:"not null;index"`
    Name       string     `gorm:"not null"`
    TokenHash  string     `gorm:"not null;uniqueIndex"` // SHA-256，不儲存明文
    Scopes     string     // JSON 陣列：["read","write","admin"]
    ExpiresAt  *time.Time
    LastUsedAt *time.Time
    CreatedAt  time.Time
}
```

**API 端點：**

```
GET  /system/security/config          取得安全設定（PlatformAdmin only）
PUT  /system/security/config          更新安全設定（PlatformAdmin only）

GET    /users/me/tokens               列出個人 API Token（不含 hash）
POST   /users/me/tokens               建立 Token（回傳一次明文，之後不可再取）
DELETE /users/me/tokens/:id           撤銷 Token
```

**完成指標：**
- 安全設定 Tab 有完整可操作內容，不再顯示佔位符。
- SIEM Webhook 推送可從 UI 設定與測試。
- 管理員可動態調整登入鎖定參數，無需重啟服務。
- 使用者可自助建立、查看、撤銷個人 API Token。

---

### 5.8 通知設定 Tab 完善（小型 Sprint，2–3 週）✅ 已完成

> **目標：** 消除「通知設定」佔位符，建立**集中式通知渠道管理**，解決現有 Event 告警規則 URL 分散問題，並補全缺失的 Email/SMTP 通知。

#### 現況診斷

| 現象 | 位置 | 影響 |
|------|------|------|
| 通知設定 Tab 顯示「功能開發中」 | `SystemSettings.tsx:81-89` | 無可操作內容 |
| 通知 URL 直接嵌入每條告警規則 | `EventAlertRule.NotifyURL` | 渠道變更需逐條修改規則 |
| `email` 通知類型無後端實作 | `event_alert_service.go:notify()` | 設為 email 的規則靜默失敗 |
| DingTalk 無加簽（HMAC-SHA256）支援 | `event_alert_service.go:270-278` | 生產環境 DingTalk 安全模式無法使用 |
| 無渠道測試入口 | — | 設定完才知道是否有效 |

#### 兩條通知鏈路現況

```
通知鏈路 A（K8s Event）：
  EventAlertWorker（60 秒掃描）→ matchRule() → notify()（直接推送 per-rule URL）→ recordHistory()

通知鏈路 B（Prometheus）：
  AlertManager（叢集級別代理）→ 外部 Alertmanager 接管路由（Synapse 不介入 receiver 設定）
```

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| `NotifyChannel` 資料模型 | `internal/models/notify_channel.go` | W1 |
| 通知渠道 CRUD + 測試 API | `internal/handlers/notify_channel.go` | W1 |
| AutoMigrate 加入 NotifyChannel | `internal/database/database.go` | W1 |
| 路由註冊（PlatformAdmin） | `internal/router/routes_system.go` | W1 |
| Email/SMTP 通知後端實作 | `internal/services/event_alert_service.go` | W2 |
| DingTalk 加簽支援（HMAC-SHA256） | `internal/services/event_alert_service.go` | W2 |
| `EventAlertRule` 新增可選 `ChannelID`（向下相容） | `internal/models/event_alert.go` | W2 |
| 前端 `NotificationSettings.tsx`（渠道列表 + 新增 Modal + 測試） | `ui/src/pages/settings/NotificationSettings.tsx` | W2–W3 |
| `SystemSettings.tsx` 替換佔位符 | `ui/src/pages/settings/SystemSettings.tsx` | W3 |
| 前端 `notifyChannelService.ts` | `ui/src/services/notifyChannelService.ts` | W2 |
| 補齊三語 i18n（zh-TW / en-US） | `ui/src/locales/*/settings.json` | W3 |

#### 資料模型

```go
// internal/models/notify_channel.go
type NotifyChannel struct {
    ID        uint           `json:"id" gorm:"primaryKey"`
    Name      string         `json:"name" gorm:"size:100;uniqueIndex;not null"`
    Type      string         `json:"type" gorm:"size:20;not null"` // webhook/dingtalk/slack/teams/email
    Config    string         `json:"config" gorm:"type:text"`      // JSON（含加密敏感欄位）
    Enabled   bool           `json:"enabled" gorm:"default:true"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// Config 各類型 JSON 結構：
// WebhookChannelConfig  { url, headers? }
// DingTalkChannelConfig { webhook_url, secret? }  // secret = HMAC-SHA256 加簽
// SlackChannelConfig    { webhook_url }
// TeamsChannelConfig    { webhook_url }
// EmailChannelConfig    { smtp_host, smtp_port, use_tls, username, password*, from, to }
// *password 以 AES-256-GCM 加密儲存（複用 SSHSettingService 加密模式）

// EventAlertRule 新增可選欄位（向下相容，ChannelID 有值則用渠道，否則退回 NotifyURL）
// ChannelID *uint `json:"channelId,omitempty" gorm:"index"`
```

#### API 端點

```
GET    /system/notify-channels              列出渠道（PlatformAdmin）
POST   /system/notify-channels              新增渠道（PlatformAdmin）
GET    /system/notify-channels/:id          取得渠道（PlatformAdmin）
PUT    /system/notify-channels/:id          更新渠道（PlatformAdmin）
DELETE /system/notify-channels/:id          刪除渠道（PlatformAdmin）
POST   /system/notify-channels/:id/test     測試渠道連線（PlatformAdmin）
```

#### 通知設定 Tab 結構

```
NotificationSettings.tsx
├── Section 1：通知渠道管理
│   ├── 渠道列表（名稱、類型、狀態 Tag、測試、編輯、刪除）
│   ├── 新增渠道 Modal（名稱 + 類型選擇 → 動態切換設定表單）
│   │   ├── Webhook   → URL + 自訂 Header
│   │   ├── DingTalk  → Webhook URL + 可選 Secret（加簽）
│   │   ├── Slack     → Incoming Webhook URL
│   │   ├── Teams     → Incoming Webhook URL
│   │   └── Email     → SMTP Host/Port/TLS + 帳密 + 寄件人 + 收件人
│   └── 測試連線按鈕（即時推送測試訊息）
│
└── Section 2：說明
    └── 告知使用者如何在 Event 告警規則引擎中引用已設定的渠道
        （連結至各叢集 /clusters/:id/event-alerts）
```

#### 可複用現有資產

| 資產 | 複用方式 |
|------|---------|
| `event_alert_service.go:notify()` 各渠道格式化邏輯 | 重構抽取為 `SendToChannel(ch *NotifyChannel, payload)` 共用函數 |
| `siem.go:TestSIEMWebhook()` 測試推送邏輯 | 泛化為渠道測試 handler |
| SSH 設定的 AES-256-GCM 加密模式 | Email 密碼 / DingTalk Secret 加密儲存 |
| `SecuritySettings.tsx` Section + Card 佈局 | 前端直接套用 |

#### 完成指標
- 通知設定 Tab 有完整可操作內容，不再顯示佔位符。
- 可建立 Webhook / DingTalk（含加簽）/ Slack / Teams / Email 渠道，並即時測試。
- Event 告警規則可引用預設渠道（ChannelID），不再需要每條填 URL。
- Email 通知正常傳送（`notifyResult = "sent"`）。

---

### 5.9 前端設計系統統一與體驗優化（Sprint，5 週）

> **目標：** 以 Ant Design v5 原生 Theme Token 取代所有 `.ant-*` CSS 覆蓋，統一設計語言，修復體驗破口，降低長期維護成本。

#### 現況診斷

| 問題 | 位置 | 影響 |
|------|------|------|
| 492 行自訂 CSS 覆蓋 Ant Design 內部 class（`.ant-*`） | `index.css`、`App.css` | AntD v5 已轉 CSS-in-JS，`.ant-*` 選擇器部分失效，樣式行為不可預測 |
| `MainLayout.tsx` 以 `document.createElement('style')` 注入帶 `!important` 的 CSS | `MainLayout.tsx:60–140` | 樣式無法被 lint/TypeScript 型別系統捕捉，維護困難 |
| Header 同時容納：搜尋 + 叢集選擇器 + 語言切換 + AI Chat + 使用者選單 | `MainLayout.tsx` Header | 資訊密度過高，視線無主次，小視窗下溢出 |
| MainLayout.tsx 單檔 25KB，含選單、Header、路由邏輯、樣式注入 | `MainLayout.tsx` | 可讀性差，任何 UI 修改都需動到核心 Layout |
| 叢集範疇頁面無明確 Context Indicator（我在哪個叢集） | 所有叢集頁面 | 使用者同時操作多叢集時容易誤操作 |
| React Query 所有資源統一 `staleTime: 30s` | 全域查詢設定 | Pod/Node 狀態 30 秒落後，事故排查時看到舊資料 |
| 空狀態（no data）、錯誤狀態（fetch failed）無統一規範 | 各頁面分散實作 | 部分頁面空白一片，部分頁面顯示原始錯誤訊息 |

---

#### Phase 1：Design Token 統一（W1）✅ 已完成（2026-04-06）

**核心原則：** 刪除所有 `.ant-*` class 覆蓋，改用 Ant Design v5 `ConfigProvider` 的 `theme.token` 與 `theme.components` 統一定義。

**待實作任務：**

| 任務 | 檔案 | 說明 |
|------|------|------|
| 建立 `theme.ts` 集中定義 Design Token | `ui/src/config/theme.ts` | 定義 colorPrimary、borderRadius、boxShadow 等全域 token |
| 以 `ConfigProvider` 包裹 App，套用 `theme.ts` | `ui/src/App.tsx` | 取代所有 `.ant-*` 全域 CSS 覆蓋 |
| 刪除 `index.css` 中所有 `.ant-*` 選擇器規則（共約 230 行） | `ui/src/index.css` | 保留 `*`, `body`, `#root` 等非 AntD 的全域 reset |
| 刪除 `App.css` 中所有 `.ant-*` 選擇器規則（共約 80 行） | `ui/src/App.css` | `.page-header`、`.stats-card`、`.toolbar` 等自訂 class 保留 |
| 修正字型 stack（移除簡體字型優先順序） | `ui/src/index.css` | `'PingFang TC'` 優先，移除 `'Microsoft YaHei'` 等簡體字型 |

**`theme.ts` 結構：**

```typescript
// ui/src/config/theme.ts
import type { ThemeConfig } from 'antd';

export const synapseTheme: ThemeConfig = {
  token: {
    colorPrimary: '#006eff',
    colorPrimaryHover: '#1a7aff',
    borderRadius: 8,
    borderRadiusLG: 12,
    colorBgLayout: '#f5f7fa',
    colorBgContainer: '#ffffff',
    colorBorder: '#e8eaec',
    colorTextBase: '#333333',
    colorTextSecondary: '#666666',
    fontFamily: "-apple-system, BlinkMacSystemFont, 'PingFang TC', 'Hiragino Sans GB', Arial, sans-serif",
    fontSize: 14,
    lineHeight: 1.5,
    boxShadow: '0 1px 4px 0 rgba(0, 0, 0, 0.08)',
    boxShadowSecondary: '0 4px 12px 0 rgba(0, 0, 0, 0.12)',
  },
  components: {
    Layout: {
      headerBg: '#ffffff',
      siderBg: '#ffffff',
      bodyBg: '#f5f7fa',
    },
    Menu: {
      itemBorderRadius: 8,
      itemSelectedBg: '#006eff',
      itemSelectedColor: '#ffffff',
      itemHoverBg: '#f0f6ff',
      itemHoverColor: '#006eff',
    },
    Button: {
      borderRadius: 8,
      primaryShadow: '0 2px 4px 0 rgba(0, 110, 255, 0.3)',
    },
    Card: {
      borderRadius: 12,
    },
    Table: {
      headerBg: '#f8f9fa',
      borderRadius: 12,
    },
    Input: {
      borderRadius: 8,
    },
    Tag: {
      borderRadius: 6,
    },
  },
};
```

---

#### Phase 2：MainLayout 重構（W2）✅ 已完成（2026-04-06）

**目標：** 拆分 25KB 的 MainLayout.tsx，移除動態 CSS 注入，Header 瘦身。

**待實作任務：**

| 任務 | 檔案 | 說明 |
|------|------|------|
| 將 Header 拆出為獨立元件 | `ui/src/layouts/AppHeader.tsx` | 包含：Logo、ClusterSelector、SearchDropdown、右側工具區 |
| 將 Sidebar 拆出為獨立元件 | `ui/src/layouts/AppSider.tsx` | 包含：選單建構、路由匹配、openKeys 狀態 |
| 移除 `document.createElement('style')` 動態注入 | `MainLayout.tsx:132–140` | 改為 CSS Module 或 `theme.components` 覆蓋 |
| Header 右側重整：次要功能移入 User Dropdown | `AppHeader.tsx` | 語言切換移入個人選單；AI Chat 改為 Header 右側 icon-only Button + Drawer |
| 新增叢集範疇 Context Bar | `ui/src/layouts/ClusterContextBar.tsx` | 顯示於 Header 下方，包含：當前叢集名稱 + 健康狀態 Badge + 叢集切換捷徑 |

**重構後 Header 佈局：**

```
┌─────────────────────────────────────────────────────────────────┐
│ [Logo] [Synapse]   [叢集選擇器▾]   [搜尋框]        [AI✦] [👤▾] │
└─────────────────────────────────────────────────────────────────┘
  ↑ 左側品牌       ↑ 核心導覽         ↑ 全域搜尋    ↑ 工具  ↑使用者

User Dropdown 內容：
  - 個人資料
  - API Token 管理
  - 語言切換（zh-TW / en-US）
  - 深色模式切換（預留）
  - 登出
```

**Context Bar（僅叢集範疇頁顯示）：**

```
┌──────────────────────────────────────────────────────────────────┐
│ 叢集：production-k8s  ● Healthy  /  工作負載 / Deployment 列表   │
└──────────────────────────────────────────────────────────────────┘
  ↑ 叢集 context                     ↑ Breadcrumb（自動產生）
```

---

#### Phase 3：資料新鮮度（W3）✅ 已完成（2026-04-06）

**問題：** 全站統一 `staleTime: 30000` 對即時性敏感資源不適用。

**分層 staleTime 策略：**

| 資源類型 | staleTime | 說明 |
|---------|-----------|------|
| Pod 列表 / 狀態 | 5s | 事故排查核心資料 |
| Node 列表 / 狀態 | 10s | 資源壓力變化相對緩慢 |
| Deployment / StatefulSet 列表 | 15s | 部署操作後需快速反映 |
| ConfigMap / Secret 列表 | 60s | 配置異動頻率低 |
| Helm Release 列表 | 30s | 預設值 |
| 使用者 / 權限資料 | 120s | 幾乎不變 |
| Overview Stats / Alert 統計 | 30s | 儀表板可接受 |

**待實作任務：**

| 任務 | 檔案 | 說明 |
|------|------|------|
| 建立 `queryConfig.ts` 定義各資源 staleTime 常數 | `ui/src/config/queryConfig.ts` | 集中管理，避免各頁面自行散落數字 |
| Pod 列表頁改用 `refetchInterval: 5000` | `ui/src/pages/pod/PodList.tsx` | 5 秒自動刷新，附加 `refetchOnWindowFocus: true` |
| Node 列表頁改用 `refetchInterval: 10000` | `ui/src/pages/node/NodeList.tsx` | — |
| Deployment 列表改用 `refetchInterval: 15000` | `ui/src/pages/workload/WorkloadList.tsx` | — |
| Overview stats 改用 `refetchInterval: 30000` | `ui/src/pages/overview/Overview.tsx` | — |
| 新增全域 `onError` handler：fetch 失敗時顯示統一 Toast | `ui/src/config/queryClient.ts` | 目前各頁面錯誤處理不一致 |

---

#### Phase 4：空狀態與回饋一致性（W4）✅

**目標：** 建立空狀態、錯誤狀態、載入狀態的統一規範元件。

**已完成任務：**

| 任務 | 檔案 | 狀態 |
|------|------|------|
| 建立 `EmptyState` 元件（支援 type / icon / 說明 / 行動按鈕） | `ui/src/components/EmptyState.tsx` | ✅ 完成 |
| 建立 `ErrorState` 元件（網路錯誤 / 無權限 / 叢集離線 / 未知） | `ui/src/components/ErrorState.tsx` | ✅ 完成 |
| 建立 `PageSkeleton` 元件（table / detail / cards 三種 variant） | `ui/src/components/PageSkeleton.tsx` | ✅ 完成 |
| 批量替換設定頁 / 詳情頁裸 `<Spin>` 為 `PageSkeleton` | AISettings / SSHSettings / LDAPSettings / GrafanaSettings / ConfigMap / Secret / ServiceEdit / IngressEdit（11 個頁面） | ✅ 完成 |
| AlertCenter / GlobalAlertCenter Empty + action 替換為 `EmptyState` | `pages/alert/AlertCenter.tsx` / `GlobalAlertCenter.tsx` | ✅ 完成 |

**元件設計規範：**

```
EmptyState type:
  no-data        → [InboxOutlined]        目前沒有資料
  no-permission  → [LockOutlined]         無存取權限
  offline        → [DisconnectOutlined]   叢集無法連線
  not-configured → [SettingOutlined]      尚未設定，引導至設定頁

ErrorState type:
  network    → 資料載入失敗（重試按鈕）
  permission → 無存取權限
  offline    → 叢集無法連線
  unknown    → 發生錯誤

PageSkeleton variant:
  table  → 工具列 + 搜尋 + 表格列（列表頁）
  detail → 頁面標題 + 表單欄位（設定 / 詳情頁）
  cards  → 統計卡片格線（Dashboard 類）
```

---

#### Phase 5：Cost 頁面與高頻操作優化（W5）✅ 已完成

**Cost 頁面：** 目前有前端模組但資料來源不完整，誤導使用者預期。

| 任務 | 說明 | 狀態 |
|------|------|------|
| 在 Cost 頁面加入資料來源說明 Banner | 明確標示「成本數據來源：Prometheus resource request，不含實際帳單費用」，避免誤解 | ✅ |
| 無 Prometheus 時顯示設定引導 | 偵測 Prometheus 未設定時，顯示 EmptyState 引導頁 | ✅ |

**高頻操作路徑優化（依使用者行為頻率排序）：**

| 操作 | 現況問題 | 優化方向 | 狀態 |
|------|---------|---------|------|
| 查看 Pod 日誌 | 從工作負載頁須點 3 層才到日誌 | 工作負載列表每行加「日誌」快捷 icon | 🔲 待實作 |
| 重啟 Deployment | 需進入詳情頁才能操作 | 列表頁加入行內「重啟」按鈕（Popconfirm 二次確認） | ✅ |
| 複製 Pod 名稱 / Service ClusterIP | 直接點擊無法複製 | Pod 名稱加入 `<Typography.Text copyable>` | ✅ |
| 切換叢集後回到同功能頁 | 切換叢集後跳回首頁 | ClusterSelector 已保留當前路由 path（僅替換 clusterID） | ✅ 已有 |

---

#### 完成指標

- `index.css` 中零個 `.ant-*` 選擇器
- `App.css` 中零個 `.ant-*` 選擇器
- `MainLayout.tsx` 行數 < 200（邏輯移至子元件）
- 零個 `document.createElement('style')` 動態注入
- Pod / Node 列表頁有 `refetchInterval`，不依賴 staleTime
- 所有列表頁有非空白的空狀態畫面
- Cost 頁面有資料來源說明，無 Prometheus 時有引導

---

### 5.6 Project 多租戶模型（獨立 Sprint）

> **現況：** 多租戶透過 `ClusterPermission` 實現，無明確的租戶/組織層級，大規模管理困難。

**方向：**
- 引入 **Project（專案）** 概念：一個 Project 對應一組 叢集+命名空間+成員
- Project 管理員可自助管理成員和配額
- 命名空間自助申請流程（Dev 申請 → 管理員審核 → 自動建立 + 配額）

**估計工作量：** 4 週（架構層面升級，需獨立 Sprint）

---

### 5.10 OIDC / SSO 整合（Sprint，3–4 週）🔴 高優先級

> **現況：** `auth_service.go` 只支援 `local` / `ldap` 兩種 auth_type，企業常用的 Google / Azure AD / Okta 無法接入。

#### 現況診斷

| 問題 | 位置 | 影響 |
|------|------|------|
| auth_type switch 只有 local / ldap | `internal/services/auth_service.go:53` | 企業用戶必須手動建帳號 |
| 無 Authorization Code Flow | — | 無法接入 SSO |
| 無 PKCE 支援 | — | 公開客戶端不安全 |
| 無 callback 路由 | `internal/router/` | OIDC 登入無法完成 |

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| `OIDCConfig` 資料模型（provider / client_id / client_secret / issuer_url / scopes） | `internal/models/system_config.go` | W1 |
| OIDC 設定 API（CRUD + 連線測試） | `internal/handlers/oidc.go` | W1 |
| Authorization Code Flow 後端（`/auth/oidc/login` → redirect → `/auth/oidc/callback`） | `internal/handlers/oidc.go` | W1–W2 |
| PKCE（code_challenge / code_verifier）支援 | `internal/handlers/oidc.go` | W2 |
| JWT claims 映射（sub / email / groups → Synapse User） | `internal/services/auth_service.go` | W2 |
| 自動佈建帳號（首次 OIDC 登入自動建立 User，status=active） | `internal/services/auth_service.go` | W2–W3 |
| 群組映射（OIDC groups claim → Synapse UserGroup） | `internal/services/auth_service.go` | W3 |
| 系統設定前端（OIDC 設定 Tab：Provider / Issuer URL / Client ID / Secret / Scopes） | `ui/src/pages/settings/OIDCSettings.tsx` | W3–W4 |
| 登入頁新增「SSO 登入」按鈕 | `ui/src/pages/auth/Login.tsx` | W4 |
| 三語 i18n | — | W4 |

#### 支援的 Provider

| Provider | Issuer URL 範例 |
|----------|----------------|
| Google | `https://accounts.google.com` |
| Azure AD | `https://login.microsoftonline.com/{tenant}/v2.0` |
| Okta | `https://{domain}.okta.com/oauth2/default` |
| GitHub (OAuth2) | `https://github.com` |
| Keycloak | `https://{host}/realms/{realm}` |
| 自定義 OIDC | 任意符合 OIDC Discovery 規範的 issuer |

#### 資料模型

```go
// internal/models/system_config.go（擴充）
type OIDCConfig struct {
    Enabled      bool     `json:"enabled"`
    ProviderName string   `json:"provider_name"`   // 顯示名稱，如 "Google"
    IssuerURL    string   `json:"issuer_url"`
    ClientID     string   `json:"client_id"`
    ClientSecret string   `json:"client_secret"`   // AES-256-GCM 加密儲存
    Scopes       []string `json:"scopes"`           // ["openid","email","profile","groups"]
    // 自動佈建
    AutoProvision    bool   `json:"auto_provision"`    // 首次登入自動建帳號
    GroupsClaim      string `json:"groups_claim"`      // groups claim 欄位名稱
    EmailDomainAllow string `json:"email_domain_allow"` // 限制 email domain，空=不限
}
```

#### API 端點

```
GET  /auth/oidc/login              發起 OIDC 登入（redirect to provider）
GET  /auth/oidc/callback           接收 provider callback，換 token，建立 session
GET  /system/oidc/config           取得 OIDC 設定（PlatformAdmin）
PUT  /system/oidc/config           更新 OIDC 設定（PlatformAdmin）
POST /system/oidc/test             測試連線（驗證 issuer discovery）
```

#### 完成指標
- 使用者可點「SSO 登入」按鈕，透過 Google/Azure AD 完成登入
- 首次登入自動建立 Synapse 帳號，groups claim 自動映射至 UserGroup
- Client Secret 以 AES-256-GCM 加密儲存，`GET /config` 不回傳明文

---

### 5.11 全局 API Rate Limiting（Sprint，1 週）🔴 高優先級

> **現況：** `internal/middleware/rate_limit.go` 只保護登入端點（5次/分鐘），所有其他 API 無流量保護。

#### 現況診斷

| 問題 | 影響 |
|------|------|
| 叢集刪除、批量刪除 API 無限速 | 任意已登入用戶可高頻呼叫破壞性操作 |
| PromQL 查詢、日誌串流 API 無限速 | 可對 Prometheus/ES 發起壓力攻擊 |
| WebSocket 連線無數量上限 | 惡意用戶可建立大量 Terminal 佔用資源 |

#### 待實作任務

| 任務 | 檔案 | 說明 |
|------|------|------|
| 引入 per-user 滑動視窗限流（`golang.org/x/time/rate`） | `internal/middleware/rate_limit.go` | 每用戶每分鐘 API 呼叫上限（預設 300） |
| 引入 per-IP 全局限流 | `internal/middleware/rate_limit.go` | 每 IP 每分鐘 600 次（防爬蟲） |
| 高危 API 專屬限速（破壞性操作） | `internal/middleware/rate_limit.go` | DELETE / batch-delete 每用戶每分鐘 20 次 |
| WebSocket 並行連線數上限 | `internal/handlers/common.go` | 每用戶最多 10 個並行 WebSocket（含 terminal + log） |
| 限流後回傳標準 HTTP 429 + Retry-After header | `internal/middleware/rate_limit.go` | 前端可顯示「請求過於頻繁」提示 |
| 前端 429 攔截 | `ui/src/utils/api.ts` | HTTP 429 時顯示 Toast 提示並延遲重試 |

#### 限流分層策略

```
全局（per-IP）：  600 req/min  → 超出 → 429（防爬蟲 / DDoS）
認證用戶（per-user）：
  一般 API：      300 req/min
  PromQL 查詢：   60  req/min  （避免壓垮 Prometheus）
  破壞性操作：    20  req/min  （DELETE / PATCH scale / batch）
  WebSocket：     10  並行連線
未認證：
  登入：          5 次/min（已有）
  其他：          30 req/min
```

---

### 5.12 AlertManager Receiver 完整管理（Sprint，2 週）✅ 已完成（2026-04-07）

> **現況：** `alertmanager_service.go` 只實作 `GetReceivers()`（讀取），無法新增/修改/刪除 receiver，使用者仍需 SSH 手動改 YAML。

#### 現況診斷

| 問題 | 位置 | 影響 |
|------|------|------|
| Receiver CRUD 缺失 | `alertmanager_service.go` | 告警渠道變更需 SSH 進伺服器 |
| 無 Receiver 前端 UI | — | 使用者無法自助設定告警目標 |
| Alertmanager config 修改需 reload | — | 需呼叫 `POST /-/reload` 套用 |

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| `GetAlertmanagerConfig()`：取得完整 config YAML | `internal/services/alertmanager_service.go` | W1 |
| `UpdateAlertmanagerConfig()`：PUT config + reload | `internal/services/alertmanager_service.go` | W1 |
| `CreateReceiver()` / `UpdateReceiver()` / `DeleteReceiver()` | `internal/services/alertmanager_service.go` | W1–W2 |
| `TestReceiver()`：發送測試告警至指定 receiver | `internal/services/alertmanager_service.go` | W2 |
| Handler 層 CRUD + Test 端點 | `internal/handlers/alert.go` | W2 |
| 前端 Receiver 管理頁（叢集維度，Tab 整合至監控頁） | `ui/src/pages/monitor/ReceiverManagement.tsx` | W2 |
| 支援 receiver 類型：email / slack / webhook / pagerduty / dingtalk | — | W2 |

#### API 端點

```
GET    /clusters/:id/alertmanager/receivers          列出 receiver
POST   /clusters/:id/alertmanager/receivers          新增 receiver
PUT    /clusters/:id/alertmanager/receivers/:name    更新 receiver
DELETE /clusters/:id/alertmanager/receivers/:name    刪除 receiver
POST   /clusters/:id/alertmanager/receivers/:name/test  測試推送
```

#### 完成指標
- 使用者可在 UI 新增 Slack / Email receiver，不需 SSH
- 測試按鈕可即時確認告警推送是否成功
- Config 變更後自動 reload Alertmanager

---

### 5.13 映像索引自動同步 Worker（小型 Sprint，3 天）🟡 中優先級

> **現況：** `SyncImages` API 存在，但需手動呼叫觸發，無 cron 自動更新，索引會隨叢集部署而過期。

#### 待實作任務

| 任務 | 檔案 | 說明 |
|------|------|------|
| `ImageIndexWorker` — 每小時增量掃描 | `internal/services/image_index_worker.go` | 複用 `CostWorker` 框架；只更新有變動的工作負載 |
| 增量更新邏輯（比對 ResourceVersion 避免全量重建） | `internal/services/image_index_worker.go` | 降低 K8s API 壓力 |
| `Router.Setup()` 啟動 Worker | `internal/router/router.go` | 統一生命週期管理 |
| 前端「映像索引」頁新增「上次同步時間」+ 手動觸發按鈕 | `ui/src/pages/workload/ImageSearch.tsx` | 補齊狀態可見性 |

---

### 5.14 工作負載列表日誌快捷入口（小型 Sprint，2 天）🟢 低優先級

> **現況：** 從工作負載列表查看 Pod 日誌需進入詳情頁再選 Pod，路徑過長（3 層點擊）。

#### 待實作任務

| 任務 | 檔案 | 說明 |
|------|------|------|
| Deployment 列表行尾加「日誌」icon 按鈕 | `ui/src/pages/workload/DeploymentTab.tsx` | 點擊開 Drawer，內嵌 Pod 選擇器 + 日誌串流 |
| StatefulSet / DaemonSet 同步補齊 | 對應 Tab 檔案 | 統一體驗 |
| `LogDrawer` 元件（Pod 下拉選擇 + 即時日誌串流） | `ui/src/components/LogDrawer.tsx` | 複用現有 WebSocket 日誌邏輯 |

---

### 5.15 Synapse 自身可觀測性（Sprint，2 週）✅ 已完成（2026-04-07）

> **現況：** 平台監控所有 K8s 叢集，但 Synapse 本身無 Prometheus metrics endpoint，無法自監控。
> 支援 **VM 和容器雙部署情境**，應用層指標完全一致，差異只在基礎設施（scrape 設定、log 收集方式）。

---

#### 架構決策

| 決策項目 | 選擇 | 理由 |
|---------|------|------|
| Registry 方案 | **自訂 Registry（方案 B）** | 與現有 `routeDeps` 注入模式一致；測試時可獨立實例化，不污染全域 |
| Metrics 開關 | **config 設定，部署時決定** | `observability.enabled: false` 時不建立 Registry、不暴露端點 |
| `/metrics` 驗證 | **可選 Bearer Token** | `observability.metrics_token: ""` 留空 = 不驗證；有值 = 需帶 Authorization header |
| GORM 指標 | **RegisterCallback（Before/After）** | 無額外 package；統一掛 Create/Update/Delete/Query |
| HTTP 指標 label | **`c.FullPath()` 模板路徑** | 避免 `:clusterID` 等動態參數造成高基數 |
| Log 格式 | **JSON**（`LOG_FORMAT=json`） | 已有 `slog.NewJSONHandler`，只需設定環境變數；Promtail/Filebeat 可直接解析 |
| Worker 指標 | **所有 Worker 統一納入** | `EventAlertWorker`、`CostWorker`、`LogRetentionWorker`；共用同一組 label |

---

#### 新增設定結構

```go
// internal/config/config.go 新增
type ObservabilityConfig struct {
    Enabled      bool   `mapstructure:"enabled"`       // false = 完全關閉，不暴露任何端點
    MetricsPath  string `mapstructure:"metrics_path"`  // 預設 /metrics
    MetricsToken string `mapstructure:"metrics_token"` // 空 = 不驗證
    HealthPath   string `mapstructure:"health_path"`   // 預設 /healthz
    ReadyPath    string `mapstructure:"ready_path"`    // 預設 /readyz
}
```

`config.yaml` 範例：
```yaml
observability:
  enabled: true
  metrics_path: /metrics
  metrics_token: ""          # 填入值後需帶 Authorization: Bearer <token>
  health_path: /healthz
  ready_path: /readyz
```

---

#### Metrics Registry 設計（`internal/metrics/`）

```
internal/metrics/
├── registry.go       # Registry struct + New() + Handler()
├── http.go           # HTTP 請求指標
├── websocket.go      # WebSocket 連線指標
├── database.go       # GORM callback hook + 指標定義
├── worker.go         # Worker 執行指標（共用 label）
└── k8s.go            # Informer / K8s API 指標
```

**Registry struct：**
```go
type Registry struct {
    reg        *prometheus.Registry
    HTTP       *HTTPMetrics
    WebSocket  *WSMetrics
    DB         *DBMetrics
    Worker     *WorkerMetrics
    K8s        *K8sMetrics
}

func New() *Registry          // 建立並註冊所有指標（含 GoCollector + ProcessCollector）
func (r *Registry) Handler() http.Handler  // promhttp.HandlerFor(r.reg, ...)
```

---

#### 指標清單

**HTTP 層（Gin middleware）**

| 指標名稱 | 類型 | Labels | 說明 |
|---------|------|--------|------|
| `synapse_http_requests_total` | Counter | method, path, status | 請求總數 |
| `synapse_http_request_duration_seconds` | Histogram | method, path | 延遲分佈（buckets: 0.01~10s） |
| `synapse_http_requests_in_flight` | Gauge | — | 當前進行中請求數 |

**WebSocket 層**

| 指標名稱 | 類型 | Labels | 說明 |
|---------|------|--------|------|
| `synapse_websocket_connections_active` | Gauge | type | 當前連線數（pod-exec / kubectl / ssh / log-stream） |
| `synapse_websocket_connections_total` | Counter | type | 累計連線建立數 |
| `synapse_websocket_errors_total` | Counter | type | 連線錯誤數 |

**資料庫層（GORM Before/After callback）**

| 指標名稱 | 類型 | Labels | 說明 |
|---------|------|--------|------|
| `synapse_db_query_duration_seconds` | Histogram | operation | 查詢延遲（create/update/delete/query） |
| `synapse_db_slow_queries_total` | Counter | operation | 慢查詢（> 500ms） |
| `synapse_db_errors_total` | Counter | operation | DB 錯誤數 |

**Background Workers**

| 指標名稱 | 類型 | Labels | 說明 |
|---------|------|--------|------|
| `synapse_worker_last_run_timestamp` | Gauge | worker | 最後執行時間（unix） |
| `synapse_worker_run_duration_seconds` | Gauge | worker | 最後一次執行耗時 |
| `synapse_worker_errors_total` | Counter | worker | 累計錯誤次數 |

worker label 值：`cost` / `event_alert` / `log_retention`

**K8s / Informer 層**

| 指標名稱 | 類型 | Labels | 說明 |
|---------|------|--------|------|
| `synapse_k8s_clusters_active` | Gauge | — | 目前已初始化 Informer 的叢集數 |
| `synapse_k8s_api_requests_total` | Counter | cluster_id, resource | K8s API 呼叫總數 |
| `synapse_k8s_api_errors_total` | Counter | cluster_id, resource | K8s API 錯誤數 |

**Go runtime + Process（自動）**

由 `collectors.NewGoCollector()` + `collectors.NewProcessCollector()` 自動提供：
goroutine 數、heap、GC pause、fd 數、RSS 記憶體——無需額外實作。

---

#### Health / Ready Endpoint 設計

```
GET /healthz → 200（永遠快速回應，只要 process 存活）
{
  "status": "ok",
  "uptime": "3h12m44s"
}

GET /readyz → 200 or 503
{
  "status": "degraded",          // ok | degraded
  "checks": {
    "database":   { "status": "ok",      "latency_ms": 2 },
    "k8s":        { "status": "ok",      "clusters": 3, "unreachable": 0 },
    "prometheus": { "status": "warn",    "message": "未設定，部分功能不可用" }
  }
}
```

`/readyz` 回 503 情境：DB 無法連線，或所有叢集均不可達。

---

#### 端點安全規則

| `metrics_token` 值 | `/metrics` 行為 |
|-------------------|----------------|
| `""`（空） | 直接回傳，不驗證 |
| `"abc123"` | 需帶 `Authorization: Bearer abc123`，否則 401 |

`/healthz` 和 `/readyz` **永遠不驗證**，供 LB / systemd 探針使用。

---

#### 待實作任務

| 任務 | 檔案 | 週次 | 狀態 |
|------|------|------|------|
| 新增 `ObservabilityConfig` 至 `Config` struct | `internal/config/config.go` | W1 | ✅ |
| 新增 `internal/metrics/registry.go` — `Registry` struct + `New()` + `Handler()` | `internal/metrics/registry.go` | W1 | ✅ |
| HTTP 指標定義 + Gin middleware（`c.FullPath()` label） | `internal/metrics/http.go` + `internal/middleware/metrics.go` | W1 | ✅ |
| WebSocket 指標定義 | `internal/metrics/websocket.go` | W1 | ✅ |
| GORM Before/After callback 掛載（4 種 operation） | `internal/metrics/database.go` | W1 | ✅ |
| Worker 指標定義；`CostWorker`、`EventAlertWorker`、`LogRetentionWorker` 各自埋點 | `internal/metrics/worker.go` | W2 | ✅ |
| K8s / Informer 指標定義；在 `ClusterInformerManager` 埋點 | `internal/metrics/k8s.go` | W2 | ✅ |
| `Registry` 注入 `routeDeps`；路由中掛載 `/metrics`（可選 token 驗證）、`/healthz`、`/readyz` | `internal/router/deps.go` + `router.go` | W2 | ✅ |
| `ObservabilityConfig` 開關邏輯：`enabled: false` 時跳過所有初始化 | `internal/router/router.go` | W2 | ✅ |
| 補充 `config.yaml` 範例 + 環境變數說明 | `deploy/config.example.yaml` | W2 | ✅ |
| Prometheus Alert Rules YAML | `deploy/monitoring/synapse-alerts.yaml` | W2 | ✅ |
| Grafana Dashboard JSON | `deploy/monitoring/synapse-dashboard.json` | W2 | 待辦（低優先） |
| **VM 部署指南**：systemd unit 範本、Prometheus static_configs 範本、Promtail config 範本、logrotate 範本 | `docs/deploy/vm-observability.md` | W2 | ✅ |

---

#### VM 部署鏈路（文件範本清單）

```
Synapse Process（VM）
  ├── /metrics          ←  Prometheus scrape（static_configs）
  ├── /healthz          ←  systemd ExecStartPost / HAProxy check
  ├── /readyz           ←  Uptime Kuma / Blackbox Exporter
  └── stdout JSON log   →  Promtail（tail log file）→ Loki

VM Host
  └── node_exporter     ←  Prometheus scrape（另一個 job，採集 CPU/MEM/Disk）
```

`docs/deploy/vm-observability.md` 將包含：
- systemd `synapse.service` 完整範本
- `prometheus.yml` scrape_configs 範本（含 node_exporter + synapse 兩個 job）
- Promtail JSON pipeline 設定（含 level / request_id label extraction）
- logrotate `/etc/logrotate.d/synapse` 範本
- Grafana datasource 接入說明

---

#### 完成指標

- `go tool pprof` 可在任何環境接入 Synapse process（bonus：`/debug/pprof` 僅限 debug mode）
- Prometheus 可正常 scrape `/metrics`，PromQL 查到上述所有指標
- `/healthz` 回 200；DB 斷線時 `/readyz` 回 503
- `enabled: false` 時，所有 observability 端點均不存在（`404` 或根本未註冊路由）
- VM 和 K8s 使用完全相同的二進位，僅靠設定檔切換行為

---

### 5.16 Kubeconfig 安全強化（Sprint，2 週）✅ 全部完成（P0/P1/P2/P3）

> **背景：** 雖已實作 AES-256-GCM 欄位加密，但現況有多個關鍵缺陷：
> ENCRYPTION_KEY 未強制、KDF 過弱、SQLite 無存取控制、TLS 預設跳過。
> 規劃細節見 [`docs/security/kubeconfig-security-plan.md`](./docs/security/kubeconfig-security-plan.md)

---

#### 現況風險評估

| 嚴重度 | 問題 | 位置 |
|--------|------|------|
| 🔴 C-1 | `ENCRYPTION_KEY` 未設定只有 Warn，不阻止啟動，導致 production 明文儲存 | `main.go:37` |
| 🔴 C-2 | `./data/synapse.db` 無 chmod 限制，同主機任何程序可直接讀取 | `database.go` |
| 🔴 C-3 | KDF 使用 `SHA-256` 單次雜湊，可被快速暴力破解（< 1ns/hash） | `pkg/crypto/crypto.go` |
| 🟠 H-1 | K8s TLS 驗證預設 `Insecure: true`，暴露 MITM 攻擊面 | `k8s_client.go:85` |
| 🟠 H-2 | 解密後 kubeconfig 字串長期留在 heap，無主動清零 | `k8s_client.go` |
| 🟠 H-4 | 金鑰輪換機制不存在，金鑰洩漏無法緊急應對 | — |
| 🟡 M-1 | 日誌可能意外記錄敏感 URL 片段 | `handlers/cluster.go:133` |
| 🟡 M-2 | 匯入 Token 未驗證 RBAC 最小化，允許 cluster-admin Token 靜默匯入 | `handlers/cluster.go` |

---

#### P0：立即修復（Week 1）✅ 已完成（2026-04-07）

**P0-1：啟動時強制 ENCRYPTION_KEY**

新增 `app.env` 設定（`production` / `development`），生產環境若未設定 `ENCRYPTION_KEY` 則 `Fatal` 拒絕啟動。

```go
// main.go
if !crypto.IsEnabled() {
    if cfg.App.Env == "development" {
        logger.Warn("【開發模式】ENCRYPTION_KEY 未設定，憑證以明文儲存（禁止用於正式環境）")
    } else {
        logger.Fatal("ENCRYPTION_KEY 未設定。正式環境必須提供加密金鑰，拒絕啟動")
    }
}
```

**P0-2：升級 KDF — SHA-256 → HKDF-SHA256**

```go
// pkg/crypto/crypto.go
import "golang.org/x/crypto/hkdf"

r := hkdf.New(sha256.New, []byte(rawKey), nil, []byte("synapse-db-field-encryption-v1"))
key := make([]byte, 32)
io.ReadFull(r, key)
globalKey = key
```

> ⚠️ KDF 升級後必須執行 `synapse admin re-encrypt` 遷移舊資料（見 P1-2）

**P0-3：SQLite 檔案 chmod 600**

```go
// database.go — initSQLite 成功後
os.MkdirAll(dir, 0700)
os.Chmod(dsn, 0600)
```

**P0-4：TLS 策略設定（`strict` / `warn` / `skip`）**

新增 `security.k8s_tls_policy` config，`production` 建議 `warn`，最終目標 `strict`。

---

#### P1：本週內完成（Week 1–2）✅ 已完成（2026-04-07）

| 項目 | 狀態 | 內容 |
|------|------|------|
| P1-1 | ✅ | `ENCRYPTION_KEY_FILE` 支援（比 env var 安全，避免 `/proc/PID/environ` 洩漏） |
| P1-2 | ⏭️ 跳過 | DB 將重建，不需要 re-encrypt migration |
| P1-3 | ✅ | `synapse admin rotate-key --new-key-file` CLI（`cmd/admin/rotate_key.go`） |
| P1-4 | ✅ | `zeroString()` + defer 清零 kubeconfig / token（`k8s_client.go`） |
| P1-5 | ✅ | `maskURL()` 過濾日誌中的 API Server URL（`handlers/cluster.go`） |

---

#### P2：下個迭代 ✅ 已完成（2026-04-07）

| 項目 | 狀態 | 內容 |
|------|------|------|
| P2-1 | ✅ | `CertExpiryWorker`（`internal/services/cert_expiry_worker.go`）：每日 09:00 掃描，30/7/1 天前通知所有已啟用渠道；匯入時自動 TLS dial 取得到期日存入 `CertExpireAt` |
| P2-2 | ✅ | `CheckRBACSummary()`（k8s_client.go）：SelfSubjectAccessReview 評估 6 種高危權限，ImportCluster response 新增 `rbacWarnings` 欄位 |
| P2-3 | ✅ | MySQL TLS：`DB_TLS_ENABLED` / `DB_TLS_CA_CERT` 設定，`gomysql.RegisterTLSConfig("synapse", ...)` MinVersion TLS 1.2 |
| P2-4 | ✅ | `docs/deploy/vm-observability.md` 新增 systemd `LoadCredential` 完整說明（金鑰生成、Unit 範例、驗證指令、舊版 fallback） |

---

#### P3：長期規劃 ✅ 已完成（2026-04-07）

| 項目 | 內容 | 狀態 |
|------|------|------|
| P3-1 | 可插拔 KMS 介面（`KeyProvider` interface，支援 Vault / AWS KMS） | ✅ `pkg/crypto/provider.go`：EnvKeyProvider、FileKeyProvider、VaultKeyProvider（direct HTTP）、AWS stub |
| P3-2 | 加密 SQLite（SQLCipher，需 CGO，高安全場景） | ✅ `sqlite_plain.go` / `sqlite_cipher.go` build tag 抽象，`docs/security/sqlcipher-build.md` |

---

#### 測試計畫

| 測試 | 驗證方式 |
|------|---------|
| 未設定 ENCRYPTION_KEY + env=production | Fatal 被呼叫（unit test） |
| SQLite 建立後 mode 0600 | `os.Stat().Mode()` assert |
| HKDF 與 SHA-256 產生不同 key | `TestKDFUpgrade` |
| Re-encrypt 後新 key 可解密 | `TestReEncryptMigration` |
| TLS strict 拒絕無 CA cert | `TestTLSPolicyStrict` |
| API response 不含 kubeconfig 欄位 | `TestClusterAPIResponseNoSecrets` |

---

### 5.17 工作負載內嵌 Prometheus 指標（Sprint，2 週）✅ 已完成（2026-04-08）

> **現況：** 叢集概覽頁有整體資源圖表，但 Deployment/Pod 詳情頁無法直接看到該工作負載的 CPU/MEM 趨勢，必須跳出到 Grafana。這是最常見的運維痛點。

---

#### 待實作任務

| 任務 | 檔案 | 週次 | 狀態 |
|------|------|------|------|
| 後端 `GET /clusters/:id/deployments/:ns/:name/metrics` | `internal/handlers/monitoring.go` `GetWorkloadMetrics` | W1 | ✅ 早已存在 |
| `PrometheusService.QueryWorkloadMetrics()` | `internal/services/prometheus_service.go` | W1 | ✅ 早已存在 |
| 路由已涵蓋 deployments / statefulsets / daemonsets / rollouts / jobs / cronjobs | `internal/router/routes_cluster.go` | W1 | ✅ 早已存在 |
| 前端 `WorkloadMetricsChart` 元件（`@ant-design/plots` Line 圖 + Segmented 時間切換） | `ui/src/components/WorkloadMetricsChart.tsx` | W2 | ✅ |
| Deployment 詳情頁新增「效能指標」Tab | `ui/src/pages/workload/DeploymentDetail.tsx` | W2 | ✅ |
| StatefulSet / DaemonSet 詳情（WorkloadDetail）修正錯誤 URL，改用 `WorkloadMetricsChart` | `ui/src/pages/workload/WorkloadDetail.tsx` | W2 | ✅ |
| 時間範圍切換器（15m / 1h / 3h / 24h）+ Prometheus 未設定提示卡 | `WorkloadMetricsChart` | W2 | ✅ |

---

#### API 設計

```
GET /api/v1/clusters/:clusterId/workloads/:namespace/:name/metrics
  ?range=1h          # 15m / 1h / 3h / 24h
  &step=60           # 秒，auto-computed by range
  &type=deployment   # deployment / statefulset / daemonset

Response:
{
  "cpu": [{ "time": 1712345678, "value": 0.42 }, ...],
  "memory": [{ "time": 1712345678, "value": 134217728 }, ...]
}
```

---

#### Data Model（後端）

```go
type WorkloadMetricsResponse struct {
    CPU    []MetricPoint `json:"cpu"`
    Memory []MetricPoint `json:"memory"`
}
type MetricPoint struct {
    Time  int64   `json:"time"`
    Value float64 `json:"value"`
}
```

---

#### 完成指標

- Deployment 詳情頁顯示 CPU / Memory 折線圖，時間軸與 Grafana 資料一致
- 切換時間範圍不需重整頁面
- Prometheus 未設定時，圖表顯示「未設定 Prometheus 資料來源」提示卡

---

### 5.18 cert-manager 憑證管理（Sprint，2 週）✅ 已完成（2026-04-08）

> **現況：** Ingress TLS 在 K8s 叢集中廣泛使用 cert-manager，但 Synapse 完全看不到 `Certificate` / `Issuer` / `ClusterIssuer` CRD，憑證即將到期也無告警。

---

#### 待實作任務

| 任務 | 檔案 | 週次 | 狀態 |
|------|------|------|------|
| Dynamic client 查詢 cert-manager CRD（無需 typed client 依賴） | `internal/handlers/cert_manager.go` | W1 | ✅ |
| `GET /clusters/:id/cert-manager/status` 偵測安裝狀態 | 同上 | W1 | ✅ |
| `GET /clusters/:id/cert-manager/certificates` 憑證列表 | 同上 | W1 | ✅ |
| `GET /clusters/:id/cert-manager/issuers` Issuer+ClusterIssuer 列表 | 同上 | W1 | ✅ |
| 路由注入 | `internal/router/routes_cluster.go` | W1 | ✅ |
| 前端憑證管理頁（Certificate + Issuer 雙 Tab，到期狀態 Badge / 天數 Tag） | `ui/src/pages/security/CertificateList.tsx` | W2 | ✅ |
| 未安裝 cert-manager 顯示安裝引導 Alert | 同上 | W2 | ✅ |
| 路由 `clusters/:id/certificates` + 側邊欄「憑證管理」項 | `App.tsx` + `AppSider.tsx` | W2 | ✅ |

---

#### Data Model

```go
type CertificateSummary struct {
    Name       string    `json:"name"`
    Namespace  string    `json:"namespace"`
    Ready      bool      `json:"ready"`
    SecretName string    `json:"secretName"`
    Issuer     string    `json:"issuer"`
    DNSNames   []string  `json:"dnsNames"`
    NotBefore  time.Time `json:"notBefore"`
    NotAfter   time.Time `json:"notAfter"`
    DaysLeft   int       `json:"daysLeft"`
}
```

---

#### 完成指標

- 憑證列表顯示所有 `Certificate` 資源及到期狀態
- `DaysLeft ≤ 30` 自動標記 `Expiring`，觸發告警渠道通知
- 無 cert-manager 安裝時，頁面顯示「未偵測到 cert-manager，請先安裝」

---

### 5.19 彈性伸縮深化（Sprint，3 週）✅ 已完成（2026-04-08）

> **現況：** HPA 已支援，KEDA / Karpenter / Cluster Autoscaler 已整合至統一彈性伸縮頁。

#### 已實作內容

| 任務 | 檔案 | 狀態 |
|------|------|------|
| KEDA CRD 偵測 + ScaledObject / ScaledJob 列表 API | `internal/handlers/autoscaling.go` | ✅ |
| Karpenter CRD 偵測 + NodePool / NodeClaim 列表 API | `internal/handlers/autoscaling.go` | ✅ |
| Cluster Autoscaler 偵測 + ConfigMap 狀態讀取 | `internal/handlers/autoscaling.go` | ✅ |
| 路由註冊（/keda、/karpenter、/cas） | `internal/router/routes_cluster.go` | ✅ |
| 前端 API 服務層 | `ui/src/services/autoscalingService.ts` | ✅ |
| 統一「彈性伸縮」頁（HPA / KEDA / Karpenter / CAS 四 Tab） | `ui/src/pages/workload/AutoscalingPage.tsx` | ✅ |
| 側邊選單「彈性伸縮」入口 + 路由 | `AppSider.tsx`, `App.tsx` | ✅ |
| 三語 i18n（zh-TW / en-US / zh-CN） | `ui/src/locales/*/workload.json` | ✅ |

**未安裝時顯示引導說明（含 helm 安裝指令），已安裝時直接列出資源。**

---

### 5.20 策略與合規深化（Sprint，3 週）🔲 待實作

> **現況：** §5.9（M9）已有基礎安全掃描。但 Kyverno / OPA Gatekeeper 策略違規、Pod Security Admission 分析、RBAC 風險評分尚未實作，這是企業客戶合規的核心需求。

---

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| Kyverno `PolicyReport` / `ClusterPolicyReport` Informer | `internal/k8s/informers/policy_report.go` | W1 |
| OPA Gatekeeper `ConstraintTemplate` + Constraint 列表 API | `internal/handlers/gatekeeper.go` | W1 |
| 前端策略違規列表（按嚴重度分組 / 資源連結） | `ui/src/pages/security/PolicyViolationList.tsx` | W2 |
| PSA（Pod Security Admission）分析：掃描命名空間 label，標記 `privileged` / `baseline` / `restricted` | `internal/services/psa_scanner.go` | W2 |
| RBAC 風險評分引擎（`cluster-admin` binding、wildcard verbs、secrets get 等）| `internal/services/rbac_risk.go` | W3 |
| 前端合規儀表板（違規統計 + PSA 熱力圖 + RBAC 風險 Top 10） | `ui/src/pages/security/ComplianceDashboard.tsx` | W3 |

---

#### Data Model

```go
type PolicyViolation struct {
    PolicyName string `json:"policyName"`
    RuleName   string `json:"ruleName"`
    Resource   string `json:"resource"`     // "Deployment/default/nginx"
    Namespace  string `json:"namespace"`
    Severity   string `json:"severity"`     // high / medium / low
    Message    string `json:"message"`
    Source     string `json:"source"`       // kyverno / gatekeeper
}

type RBACRiskItem struct {
    Subject     string   `json:"subject"`   // ServiceAccount / User / Group
    Namespace   string   `json:"namespace"`
    RiskScore   int      `json:"riskScore"` // 0-100
    RiskReasons []string `json:"riskReasons"`
}
```

---

#### 完成指標

- Kyverno / Gatekeeper 違規顯示在合規儀表板，可下鑽到具體資源
- RBAC 風險評分可辨識 `cluster-admin` 綁定及 wildcard 動詞濫用
- PSA 分析顯示每個命名空間的安全等級，`privileged` 命名空間標記警告

---

### 5.21 叢集運維工具箱（Sprint，2 週）🔲 待實作

> **現況：** K8s 版本升級前需掃描 deprecated API、控制面元件健康狀況（etcd / scheduler / controller-manager）無從查看、節點維護（cordon/drain）無 GUI 工作流。

---

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| Deprecated API 掃描（使用 `pluto` 邏輯或自實作 API version map） | `internal/services/deprecated_api_scanner.go` | W1 |
| `GET /clusters/:id/deprecated-apis` — 列出所有部署中的 deprecated 資源 | `internal/handlers/cluster_ops.go` | W1 |
| 控制面健康 API（etcd metrics / `/readyz` / component status） | `internal/handlers/control_plane.go` | W1 |
| 節點 Cordon / Drain / Uncordon API（包含 drain grace period 設定） | `internal/handlers/node_ops.go` | W2 |
| 前端節點操作確認 Modal（顯示受影響 Pod 清單） | `ui/src/pages/cluster/NodeMaintenanceModal.tsx` | W2 |
| 前端「叢集健康」Tab（控制面狀態 + deprecated API 報告） | `ui/src/pages/cluster/ClusterHealthTab.tsx` | W2 |

---

#### 完成指標

- Deprecated API 報告顯示目前叢集部署中使用已棄用 API 的資源清單，含對應的替換版本
- 控制面元件 `etcd` / `scheduler` / `controller-manager` 健康狀態可見
- Cordon/Drain 操作前顯示影響評估，Drain 完成後狀態自動更新

---

### 5.22 VolumeSnapshot + Velero 深化（Sprint，2 週）🔲 待實作

> **現況：** §5.4（StorageClass / PVC）已完成。但 VolumeSnapshot CRD 和 Velero Backup / Restore 完全不可見，而這是 Stateful 應用的核心保護機制。

---

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| `VolumeSnapshot` / `VolumeSnapshotClass` Informer | `internal/k8s/informers/volume_snapshot.go` | W1 |
| `GET /clusters/:id/volume-snapshots` 列表 + 詳情 API | `internal/handlers/volume_snapshot.go` | W1 |
| 前端 PVC 詳情頁新增「快照」Tab（列出關聯快照 + 建立快照按鈕） | `ui/src/pages/storage/PVCDetail.tsx` | W1 |
| Velero `Backup` / `Restore` / `Schedule` CRD 列表 API | `internal/handlers/velero.go` | W2 |
| 前端 Velero 備份中心（Backup 列表 / 狀態 / 觸發 Restore） | `ui/src/pages/storage/VeleroBackupCenter.tsx` | W2 |
| Backup 排程（Schedule）管理 CRUD | 同上 | W2 |

---

#### 完成指標

- VolumeSnapshot 可在 PVC 詳情頁觸發建立並查看快照列表
- Velero Backup 狀態（`Completed` / `Failed` / `InProgress`）即時可見
- Restore 操作可從 UI 觸發並追蹤進度

---

### 5.23 臨時偵錯容器（Ephemeral Debug Containers）UI（Sprint，1 週）🔲 待實作

> **現況：** K8s 1.23+ 正式支援 `kubectl debug` 注入 ephemeral container，但 Synapse 的 Pod 詳情頁沒有 GUI 觸發入口，進階排查只能靠 terminal 手動輸入指令。

---

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| 後端 `POST /clusters/:id/pods/:ns/:name/debug` — 注入 ephemeral container | `internal/handlers/pod_debug.go` | W1 |
| 選擇偵錯映像（預設清單：`busybox` / `nicolaka/netshoot` / `alpine` / 自訂）| 同上 | W1 |
| 前端 Pod 詳情頁「偵錯」按鈕 → 設定 Modal（映像 / target container） | `ui/src/pages/workload/PodDebugModal.tsx` | W1 |
| 注入成功後自動開啟 Web Terminal 連線至 ephemeral container | `ui/src/pages/workload/PodDetail.tsx` | W1 |

---

#### API 設計

```
POST /api/v1/clusters/:clusterId/pods/:namespace/:name/debug
{
  "image": "nicolaka/netshoot",
  "targetContainer": "app",      // optional, share PID namespace
  "name": "debugger-abc123"      // auto-generated if empty
}

Response: { "containerName": "debugger-abc123" }
```

---

#### 完成指標

- Pod 詳情頁一鍵注入偵錯容器，自動建立 Web Terminal 連線
- ephemeral container 列於 Pod 詳情容器清單，有 `ephemeral` badge 標記
- 叢集版本 < 1.23 時按鈕 disabled 並顯示版本要求

---

### 5.24 映像安全深化（Sprint，2 週）🔲 待實作

> **現況：** §5.9 已有基礎 Trivy 掃描入口，但 CVE 結果沒有內嵌到工作負載詳情頁；Falco 執行期告警完全不可見。

---

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| Workload 詳情頁「安全掃描」Tab（顯示映像 CVE 統計 + Trivy 觸發按鈕） | `ui/src/pages/workload/WorkloadSecurityTab.tsx` | W1 |
| `GET /clusters/:id/workloads/:ns/:name/security` — 聚合該工作負載所有 container 的 Trivy 結果 | `internal/handlers/workload_security.go` | W1 |
| CVE 列表可過濾（嚴重度 / 是否有修復版本） | 前端同上 | W1 |
| Falco `FalcoAlert` 事件 Informer（透過 Falco sidekick HTTP output）| `internal/services/falco_receiver.go` | W2 |
| `GET /clusters/:id/falco-alerts` 列表 API（含 rule / output / priority） | `internal/handlers/falco.go` | W2 |
| 前端 Falco 執行期告警列表（安全 Tab 下）+ 告警渠道轉發整合 | `ui/src/pages/security/FalcoAlertList.tsx` | W2 |

---

#### Data Model

```go
type FalcoAlert struct {
    Time      time.Time `json:"time"`
    Rule      string    `json:"rule"`
    Priority  string    `json:"priority"`   // Emergency / Alert / Critical / Error / Warning / Notice
    Output    string    `json:"output"`
    Source    string    `json:"source"`     // syscall / k8s_audit
    ClusterID uint      `json:"clusterId"`
    Namespace string    `json:"namespace"`
    PodName   string    `json:"podName"`
}
```

---

#### 完成指標

- Deployment 詳情頁安全 Tab 顯示各容器的 CVE 分佈（Critical / High / Medium / Low）
- Falco 告警即時出現在安全告警列表，Priority=Critical 觸發已設定的告警渠道
- 無 Falco sidekick 設定時，顯示設定引導（HTTP output URL = `https://synapse/falco-events`）

---

### 5.25 YAML 自動回滾機制（Sprint，2 週）🔲 待實作

> **目標：** 編輯 YAML 並 Apply 後，若導致 K8s 工作負載異常，系統自動回滾至套用前版本，並即時通知使用者。重要原則：條件謹慎設計，不能太激進。

#### 觸發條件設計

| 條件 | 行為 | 說明 |
|------|------|------|
| `CrashLoopBackOff` | ✅ 自動回滾 | 容器持續崩潰重啟 |
| `ImagePullBackOff` / `ErrImagePull` | ✅ 自動回滾 | 映像無法拉取 |
| `OOMKilled`（含 restartCount > 0）| ✅ 自動回滾 | 記憶體不足被終止 |
| `ProgressDeadlineExceeded` | ✅ 自動回滾 | Deployment 超過進度截止時間 |
| Available replicas < desired | ⚠️ 通知不回滾 | 可能正在滾動更新中 |
| Pod Pending | ⚠️ 通知不回滾 | 可能資源不足，等待節點 |
| Readiness probe failing | ⚠️ 通知不回滾 | 應用可能啟動較慢 |
| `spec.replicas == 0` | ❌ 不干預 | 使用者手動縮為 0 |
| `dryRun == true` | ❌ 不干預 | 預演模式不啟動監控 |

#### 總體架構

```
前端 YAML 編輯器
       │  apply YAML（非 dryRun）
       ▼
  ApplyYAML Handler
       │
       ├─ 1. 擷取並保存「舊版 YAML 快照」（in-memory）
       ├─ 2. 執行 Apply（k8s API）
       ├─ 3. 返回 { jobID, ... } 給前端
       └─ 4. 啟動 RollbackWatcher goroutine
                      │
                      │ 每 10s 輪詢，最多 5 分鐘（首次延遲 15s）
                      ├─ 檢查 Deployment/StatefulSet/DaemonSet conditions
                      ├─ 檢查 Pod container statuses
                      │
                      ├─ [自動回滾條件] → Apply 舊版 YAML → 推送 SSE 事件
                      ├─ [通知條件]     → 推送警告 SSE 事件，不動作
                      └─ [健康/超時]    → 推送 completed SSE 事件，結束

前端 SSE 訂閱 /rollback-watch/:jobID
       └─ 接收事件 → 更新 YAML Editor 狀態列 / 全域 Notification
```

#### 後端資料結構

```go
// internal/services/rollback_watcher.go

type WatchEventType string

const (
  EventChecking   WatchEventType = "checking"
  EventTrigger    WatchEventType = "triggered"   // 準備回滾
  EventRolledBack WatchEventType = "rolled_back" // 回滾完成
  EventWarning    WatchEventType = "warning"     // 通知但不回滾
  EventHealthy    WatchEventType = "healthy"     // 確認健康，結束
  EventTimeout    WatchEventType = "timeout"     // 超時，結束
  EventError      WatchEventType = "error"
)

type WatchEvent struct {
  Type      WatchEventType `json:"type"`
  Reason    string         `json:"reason"`          // CrashLoopBackOff / ProgressDeadlineExceeded...
  PodName   string         `json:"podName,omitempty"`
  Message   string         `json:"message"`
  Timestamp time.Time      `json:"timestamp"`
}

type RollbackJob struct {
  JobID        string
  ClusterID    uint
  Namespace    string
  Name         string
  Kind         string         // Deployment | StatefulSet | DaemonSet
  PreviousYAML string         // 回滾用快照（in-memory，TTL 10 分鐘）
  EventCh      chan WatchEvent
  CancelFn     context.CancelFunc
}
```

#### RollbackWatcher 核心邏輯

```go
func (w *RollbackWatcher) Run(ctx context.Context) {
  ticker := time.NewTicker(10 * time.Second)
  timeout := time.After(5 * time.Minute)
  time.Sleep(15 * time.Second)  // 等待滾動更新開始，避免誤判

  for {
    select {
    case <-ctx.Done():
      return
    case <-timeout:
      w.emit(EventTimeout, "", "", "監控超時，未偵測到異常")
      return
    case <-ticker.C:
      w.emit(EventChecking, "", "", "檢查中...")
      reason, podName, autoRollback, warn := w.checkHealth(ctx)
      switch {
      case autoRollback:
        w.emit(EventTrigger, reason, podName, "偵測到異常，準備回滾")
        if err := w.applyRollback(ctx); err != nil {
          w.emit(EventError, reason, podName, "回滾失敗: "+err.Error())
        } else {
          w.emit(EventRolledBack, reason, podName, "已自動回滾至上一版本")
        }
        return
      case warn:
        w.emit(EventWarning, reason, podName, "警告："+reason+"（不自動回滾）")
      default:
        if w.isHealthy(ctx) {
          w.emit(EventHealthy, "", "", "部署健康，結束監控")
          return
        }
      }
    }
  }
}
```

#### 防護閘門（Guards）

```go
func shouldWatch(spec replicas *int32, dryRun bool) bool {
  if dryRun { return false }                             // DryRun 不監控
  if spec != nil && *spec == 0 { return false }         // 手動縮為 0 不干預
  return true
}
```

#### API 端點

```
# Apply + 開始監控（修改現有端點）
POST /api/clusters/:cid/deployments/:ns/:name/apply-yaml
     Body: { yaml, dryRun }
     Response: { jobID, ... }

# SSE 監控流（新增）
GET  /api/clusters/:cid/rollback-watch/:jobID
     Content-Type: text/event-stream

# 手動回滾（新增，可選）
POST /api/clusters/:cid/deployments/:ns/:name/rollback
     Body: { revision }
```

SSE 格式沿用現有 `ai_chat.go` 的 `text/event-stream` 模式。

#### 前端狀態機與 UI

YAML Editor 底部狀態列：

```
idle       →  （無顯示）
applying   →  正在套用 YAML...
watching   →  ● 監控中...  已過 00:32  [取消監控]
healthy    →  ✓ 部署健康，監控結束
rolled_back → ⚠ CrashLoopBackOff (pod/xxx-abc) — 已自動回滾 ✓
warning    →  ⚠ Pending — 已通知，不自動回滾
timeout    →  ⏱ 監控超時（5 分鐘），未偵測到異常
error      →  ✕ 回滾失敗，請手動處理
```

回滾發生時額外推送全域 `notification.error`：

```
❌ 已自動回滾：my-deployment
原因：CrashLoopBackOff (pod/my-deployment-xxx-yyy)
已回滾至套用前版本。  [查看詳情]
```

#### 支援資源範圍（第一版）

| 資源 | 自動回滾 | 通知 |
|------|----------|------|
| Deployment | ✅ | ✅ |
| StatefulSet | ✅ | ✅ |
| DaemonSet | ✅ | ✅ |
| Argo Rollout | ❌（使用原生 abort） | — |
| ConfigMap / Secret | ❌（Pod restart 不可預期） | — |
| Service / Ingress | ❌（無 Pod 狀態） | — |

#### 待實作任務

| 任務 | 檔案 | 週次 |
|------|------|------|
| `RollbackWatcher` + health check | `internal/services/rollback_watcher.go` | W1 |
| `RollbackRegistry`（in-memory job store + TTL 清理） | `internal/services/rollback_registry.go` | W1 |
| Deployment ApplyYAML 加快照 + 啟動 watcher + SSE 端點 | `internal/handlers/deployment.go` | W1 |
| StatefulSet / DaemonSet 同步擴展 | `internal/handlers/statefulset.go`, `daemonset.go` | W1–W2 |
| 前端 SSE 服務層 | `ui/src/services/rollbackWatchService.ts` | W2 |
| YAMLEditor 加狀態列 + SSE 訂閱 | `ui/src/pages/yaml/YAMLEditor.tsx` | W2 |
| 三語 i18n | `ui/src/locales/*/workload.json` | W2 |

**關鍵設計決策：**

| 決策 | 選擇 | 理由 |
|------|------|------|
| 快照儲存 | in-memory（TTL 10 分鐘） | 回滾窗口短，無需持久化 |
| 推送通道 | SSE（沿用 ai_chat.go 模式） | 基礎設施已存在，實作簡單 |
| 監控間隔 | 10s | 兼顧即時性與 k8s API 壓力 |
| 監控超時 | 5 分鐘 | 對齊 K8s 預設 progressDeadlineSeconds |
| 首次延遲 | 15s | 避免滾動更新中短暫不一致觸發誤判 |
| OOMKilled 閾值 | restartCount > 0 | 一次 OOM 即觸發，防止反覆崩潰 |

**完成指標：**
- 更新錯誤映像標籤的 Deployment 後，5 分鐘內自動回滾，前端顯示「ImagePullBackOff — 已自動回滾」
- 更新導致 OOMKilled 的 Deployment 後，偵測後立即回滾
- DryRun 模式下不啟動監控
- 手動縮為 0 replicas 後更新 YAML 不觸發回滾

---

## 6. 里程碑規劃

### 功能完成狀態總覽

| 里程碑 | 功能 | 狀態 | 優先級 | 估計工作量 |
|--------|------|------|--------|-----------|
| — | **Kubeconfig 安全強化**（P0～P3 全部完成：HKDF KDF + 強制加密 + TLS 策略 + rotate-key CLI + CertExpiry Worker + RBAC Summary + MySQL TLS + systemd LoadCredential + 可插拔 KMS + SQLCipher build tag） | ✅ 已完成（2026-04-07） | 🔴 CRITICAL | 2 週 |
| M1 | 安全強化 | ✅ 已完成 | — | — |
| M2 | 穩定性與效能 | ✅ 已完成 | — | — |
| M3 | 可觀測性 | ✅ 已完成 | — | — |
| M4 | Helm Release 管理 | ✅ 已完成 | — | — |
| M5 | AI 診斷 + CRD + NetworkPolicy + Event 告警 | ✅ 已完成 | — | — |
| M6 | 資源成本分析 | ✅ 已完成 | — | — |
| M7 | AI 深度運維 | ✅ 已完成 | — | — |
| M8 | **多叢集工作流程** | ✅ 已完成 | — | — |
| M9 | 合規性與安全掃描 | ✅ 已完成 | — | — |
| — | **安全設定 Tab 完善**（SIEM 接入 + 登入安全設定 + API Token） | ✅ 已完成 | — | — |
| — | **通知設定 Tab 完善**（集中渠道管理 + DingTalk HMAC-SHA256 加簽） | ✅ 已完成 | 🟡 中 | 2–3 週 |
| M10 | ~~備份匯出 + CLI 工具~~ → Velero 附加（M16 後）+ CLI（M16 後重新規劃） | ⏸ 延後 | 低 | 重新評估 |
| M11 | NetworkPolicy 拓撲內聯編輯 + 策略模擬 | ✅ 已完成 | — | — |
| M12 | Service Mesh 視覺化（Istio） | ✅ 已完成 | — | — |
| M13 | **原生 CI Pipeline 引擎** | 🔲 待實作 | 🔴 高 | 8 週 |
| M14 | **Git 整合 + Webhook 觸發** | 🔲 待實作 | 🔴 高 | 4 週 |
| M15 | **映像 Registry 整合** | 🔲 待實作 | 🟡 中 | 3 週 |
| M16 | **原生輕量 GitOps** | 🔲 待實作 | 🟡 中 | 6 週 |
| M17 | **環境管理 + Promotion 流水線** | 🔲 待實作 | 🟢 低 | 5 週 |
| — | **前端設計系統統一與體驗優化**（§5.9） | ✅ 已完成（工作負載列表日誌快捷 icon 除外） | — | — |
| M18 | **工作負載內嵌 Prometheus 指標**（§5.17） | ✅ 已完成（2026-04-08） | 🟡 中 | 2 週 |
| M19 | **cert-manager 憑證管理**（§5.18） | ✅ 已完成（2026-04-08） | 🟡 中 | 2 週 |
| M20 | **彈性伸縮深化 KEDA / Karpenter / CAS**（§5.19） | ✅ 已完成（2026-04-08） | 🟡 中 | 3 週 |
| M21 | **策略與合規深化 Kyverno / PSA / RBAC 風險**（§5.20） | 🔲 待實作 | 🟡 中 | 3 週 |
| M22 | **叢集運維工具箱 Deprecated API / Drain**（§5.21） | 🔲 待實作 | 🟡 中 | 2 週 |
| M23 | **VolumeSnapshot + Velero 深化**（§5.22） | 🔲 待實作 | 🟢 低 | 2 週 |
| M24 | **臨時偵錯容器 UI**（§5.23） | 🔲 待實作 | 🟡 中 | 1 週 |
| M25 | **映像安全深化 Trivy + Falco**（§5.24） | 🔲 待實作 | 🟡 中 | 2 週 |

**待實作總估計：約 26 週（M13–M17）+ 17 週（M18–M25）= 43 週**

### 建議實作順序

```
現在（管理平台）
    │ M8  ✅ 多叢集工作流程（遷移精靈 + 配置同步）
    │ M11 ✅ NetworkPolicy 模擬
    │ M12 ✅ Service Mesh 視覺化
    │ §5.9 ✅ 前端設計系統統一（Design Token / MainLayout 重構 / 資料新鮮度 / 空狀態）
    ▼
M13 CI Pipeline 引擎（8 週，最大缺口，平台演進關鍵）
    │
    ▼
M14 Git Webhook（4 週，CI 自動化觸發）
    │
    ▼
M15 Registry 整合（3 週，Pipeline 產物管理）
    │
    ▼
M16 原生 GitOps（6 週，CD 能力內建化）
    │
    ▼
M17 環境流水線（5 週，企業多環境 Promotion Gate）
    │
    ▼
目標（全能 CI/CD DevOps 平台）
```

---

## 7. 平台演進方向：全能 CI/CD DevOps 平台

> **詳細架構設計請見 [CICD_ARCHITECTURE.md](./CICD_ARCHITECTURE.md)**

### 戰略目標

從「K8s 多叢集管理工具」演進為「端到端 DevSecOps 平台」。GitLab 僅作為程式碼倉庫，Pipeline 定義、執行、Trivy 安全掃描、Harbor 推送、K8s 部署、通知告警，全部由 Synapse 集中管控。

### 整體流程摘要

```
git push → GitLab（純 Repo）
    → Synapse Webhook（M14）
    → Pipeline 引擎（M13）：Build → Trivy 掃描 → Push Harbor → Deploy K8s
    → Synapse 集中戰情室：Pipeline 狀態 / CVE 結果 / 部署狀態 / 通知
```

### 里程碑對應

| 里程碑 | 內容 | 工作量 |
|--------|------|--------|
| M13 | 原生 CI Pipeline 引擎（K8s Job 驅動） | 8 週 |
| M14 | Git 整合 + Webhook 觸發（GitLab / GitHub / Gitea）| 4 週 |
| M15 | 映像 Registry 整合（Harbor 為主）| 3 週 |
| M16 | 原生輕量 GitOps（CD）| 6 週 |
| M17 | 環境管理 + Promotion 流水線（dev → staging → prod）| 5 週 |

### 近期過渡方案（不需等 M13）

在 M13 完成前，可透過以下方式讓 Synapse 先具備戰情室效果：

1. **GitLab CI 推送掃描結果**：GitLab CI 跑 Trivy 後呼叫 `POST /security/scans`，結果集中到 Synapse
2. **Informer 自動掃描**：Pod 上線時自動觸發 Trivy，不需手動操作

詳見 [CICD_ARCHITECTURE.md §4](./CICD_ARCHITECTURE.md#4-近期過渡方案不需等-m13)

---

## 8. 附錄

### 技術選型備選

| 需求 | 第一選擇 | 備選 | 備註 |
|------|---------|------|------|
| 狀態管理 | @tanstack/react-query | SWR | React Query 生態更完整 |
| 拓撲圖 | ReactFlow v12 | @antv/g6 | ReactFlow 對 React 整合更佳，內建 Dagre 佈局 |
| 日誌系統 | `slog`（標準庫） | zap | Go 1.21+ slog 是官方解 |
| 追蹤 | OpenTelemetry | Jaeger SDK | OTel 為業界標準 |
| NP 策略模擬 | 自實作 Go selector matching | kube-networkpolicies | K8s NP 語義不複雜，自實作可控無外部依賴 |
| Istio 流量資料 | Prometheus `istio_requests_total` | Kiali API | Prometheus 已為現有依賴；Kiali 需額外部署 |
| CLI 框架 | cobra + viper | urfave/cli | cobra 生態最大，kubectl/helm 皆採用 |

> CI/CD 相關技術選型（Pipeline 引擎、Registry、Git Provider、GitOps Diff）請見 [CICD_ARCHITECTURE.md §13](./CICD_ARCHITECTURE.md#13-技術選型)
