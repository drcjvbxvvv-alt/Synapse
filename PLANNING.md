# Synapse 系統規劃書

> 版本：v1.4 | 日期：2026-04-03 | 狀態：進行中
> 已完成項目請見 [COMPLETED.md](./COMPLETED.md)

---

## 目錄

1. [系統現況總覽](#1-系統現況總覽)
2. [待解決技術債](#2-待解決技術債)
3. [邊界天花板分析](#3-邊界天花板分析)
4. [待實作優化](#4-待實作優化)
5. [待實作功能](#5-待實作功能)
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

#### Phase 3：資料新鮮度（W3）

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

#### Phase 4：空狀態與回饋一致性（W4）

**目標：** 建立空狀態、錯誤狀態、載入狀態的統一規範元件。

**待實作任務：**

| 任務 | 檔案 | 說明 |
|------|------|------|
| 建立 `EmptyState` 元件（支援 icon + 說明 + 行動按鈕） | `ui/src/components/EmptyState.tsx` | 統一取代各頁面 `<Empty />` 散落用法 |
| 建立 `ErrorState` 元件（顯示錯誤類型 + 重試按鈕） | `ui/src/components/ErrorState.tsx` | 網路錯誤 / 無權限 / 叢集離線 三種模式 |
| 建立 `PageSkeleton` 元件（頁面載入骨架屏） | `ui/src/components/PageSkeleton.tsx` | 取代裸露的 `<Spin />` |
| 補齊各列表頁空狀態文案與引導 | Pod / Node / Workload / Helm 等 | 空資源時顯示「如何建立第一個...」引導連結 |
| 統一 mutation 成功/失敗 Toast | 全域 `queryClient.ts` + 各 mutation | 成功用 `message.success`，失敗用 `message.error`，格式統一 |

**三種空狀態範本：**

```
叢集無資源：  [Kubernetes Icon]  此叢集目前沒有 Pod  [建立工作負載 →]
無權限存取：  [LockOutlined]     您沒有此資源的存取權限
叢集離線：    [DisconnectOutlined] 無法連線至叢集 API，請確認叢集狀態
```

---

#### Phase 5：Cost 頁面與高頻操作優化（W5）

**Cost 頁面：** 目前有前端模組但資料來源不完整，誤導使用者預期。

| 任務 | 說明 |
|------|------|
| 在 Cost 頁面加入資料來源說明 Banner | 明確標示「成本數據來源：Prometheus resource request，不含實際帳單費用」，避免誤解 |
| 無 Prometheus 時顯示設定引導 | 偵測 Prometheus 未設定時，顯示「設定 Prometheus 以啟用成本分析」的引導頁，而非空白或錯誤 |

**高頻操作路徑優化（依使用者行為頻率排序）：**

| 操作 | 現況問題 | 優化方向 |
|------|---------|---------|
| 查看 Pod 日誌 | 從工作負載頁須點 3 層才到日誌 | 工作負載列表每行加「日誌」快捷 icon |
| 重啟 Deployment | 需進入詳情頁才能操作 | 列表頁加入行內「重啟」按鈕（加二次確認 Popconfirm） |
| 複製 Pod 名稱 / Service ClusterIP | 直接點擊無法複製 | 加入 `<Typography.Text copyable>` |
| 切換叢集後回到同功能頁 | 切換叢集後跳回首頁 | 叢集切換後保留當前路由 path（僅替換 clusterID） |

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

## 6. 里程碑規劃

### 功能完成狀態總覽

| 里程碑 | 功能 | 狀態 | 優先級 | 估計工作量 |
|--------|------|------|--------|-----------|
| M1 | 安全強化 | ✅ 已完成 | — | — |
| M2 | 穩定性與效能 | ✅ 已完成 | — | — |
| M3 | 可觀測性 | ✅ 已完成 | — | — |
| M4 | Helm Release 管理 | ✅ 已完成 | — | — |
| M5 | AI 診斷 + CRD + NetworkPolicy + Event 告警 | ✅ 已完成 | — | — |
| M6 | 資源成本分析 | ✅ 已完成 | — | — |
| M7 | AI 深度運維 | ✅ 已完成 | — | — |
| M8 | **多叢集工作流程** | 🔲 待實作 | 🟢 低 | 5 週 |
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
| — | **前端設計系統統一與體驗優化**（§5.9） | 🔲 待實作 | 🟡 中 | 5 週 |

**待實作總估計：約 35–36 週（M8 + M13–M17 + 前端優化）**

### 建議實作順序

```
現在（管理平台）
    │ M11 ✅ NetworkPolicy 模擬
    │ M12 ✅ Service Mesh 視覺化
    ▼
前端體驗優化（5 週，§5.9，可與後端並行）
    │ Phase 1：Design Token 統一（W1）
    │ Phase 2：MainLayout 重構（W2）
    │ Phase 3：資料新鮮度（W3）
    │ Phase 4：空/錯誤狀態（W4）
    │ Phase 5：Cost 與高頻操作（W5）
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

> **戰略目標：** 從「K8s 多叢集管理工具」演進為「端到端 CI/CD DevOps 平台」，具備與 GitLab CI + ArgoCD + Rancher 組合相競爭的完整能力，以單一二進位、零外部依賴為核心競爭優勢。

### 7.1 現況差距分析

| 能力維度 | 現況 | 差距 |
|---------|------|------|
| GitOps / CD | 代理外部 ArgoCD（需額外安裝） | 無原生 CD |
| CI Pipeline | **完全沒有** | 最大缺口 |
| Git 整合 | 無 | 無 Webhook、無 Repo 連結 |
| 映像建置 / Registry | 無 | 無 Build 能力、無 Registry 管理 |
| 環境流水線 | 僅 Namespace 粒度 | 無 dev → staging → prod 概念 |

### 7.2 架構路線

**採用混合路線（C）：** 原生輕量 Pipeline 覆蓋 80% 使用場景（Build → Push → Deploy），進階場景透過插件接入 Tekton/Jenkins。

### 7.3 M13 — 原生 CI Pipeline 引擎（8 週）

> 以 K8s Job / Pod 作為 Pipeline 執行單元，定義儲存在 Synapse DB，執行時動態建立 K8s Job。

**資料模型：** `Pipeline`（定義，DAG steps）→ `PipelineRun`（執行記錄）→ `StepRun`（步驟狀態 + K8s Job 對應）

**執行引擎：**
```
1. 建立 PipelineRun 記錄
2. 解析步驟 DAG（依賴關係）
3. 按拓撲序依次提交 K8s Job（image/command/env/resource limits/workspace PVC）
4. Watch Job 狀態，即時更新 StepRun
5. Job 完成後串流 Pod 日誌
6. 所有步驟成功 → success；任一失敗 → 取消後續，failed
```

**API：**
```
GET/POST /pipelines                           Pipeline CRUD
GET      /pipelines/:id/runs                  執行歷史
POST     /pipelines/:id/run                   手動觸發
GET      /pipelines/:id/runs/:runId/steps/:step/logs  步驟日誌（SSE 串流）
POST     /pipelines/:id/runs/:runId/cancel    取消執行
```

**前端：**
```
ui/src/pages/pipeline/
  ├── PipelineList.tsx         列表（狀態燈、最後執行時間）
  ├── PipelineEditor.tsx       步驟卡片 + YAML 雙模式編輯器
  ├── PipelineRunDetail.tsx    DAG 進度圖 + 步驟狀態
  └── StepLogViewer.tsx        步驟日誌串流（SSE，複用 Terminal 樣式）
```

### 7.4 M14 — Git 整合 + Webhook 觸發（4 週）

**支援 Provider：** GitHub（App/PAT）、GitLab（Webhook Token）、Gitea（自架優先）

**Webhook 流程：**
```
Git Push → POST /webhooks/:provider/:token
  → 驗證 HMAC signature
  → 比對 Pipeline 的 GitRepo + GitBranch（glob）
  → 建立 PipelineRun（TriggerBy="webhook:sha"）
  → 回傳 202 Accepted
```

### 7.5 M15 — 映像 Registry 整合（3 週）

**支援：** Harbor（首選）、Docker Hub、阿里雲 / AWS ECR / GCR（標準 Docker Registry API v2）

**功能：** Registry 連線設定、Repository + Tag 瀏覽、Tag 保留策略、漏洞掃描觸發、Pipeline 步驟自動注入 `imagePullSecret`

### 7.6 M16 — 原生輕量 GitOps（6 週）

**Layer 1（內建）：** 定義 GitOpsApp（Git Repo + 路徑 + 目標叢集）→ 定期 Diff → Auto Sync 或 Drift 通知，支援 Kustomize overlay 和 Helm Chart。

**Layer 2（升級）：** 現有 ArgoCD 代理保留；新增 ArgoCD App Health 聚合到主儀表板；Pipeline 部署步驟可選「觸發 ArgoCD Sync」。

### 7.7 M17 — 環境管理 + Promotion 流水線（5 週）

**環境概念：** `dev → staging → production`，每個環境對應叢集 + 命名空間 + 自動/人工 Promote 策略。

**Promotion 流程：**
```
Pipeline 執行成功 → 部署到 dev → 自動（或等待審核）Promote to staging
  → smoke test（可選）→ 人工審核（Production Gate）→ 部署到 production
```

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
| CI Pipeline 執行引擎 | K8s Job（原生，零額外元件） | Tekton Pipelines | K8s Job 已是現有依賴 |
| Pipeline 步驟間產物共享 | `emptyDir` / PVC（K8s 原生） | MinIO | 簡單場景用 emptyDir；需持久化時用 PVC |
| Git Provider 整合 | 自實作 Webhook handler | go-github SDK | 各 Provider Webhook 格式差異不大，自實作可控 |
| GitOps Diff 引擎 | `k8s.io/apimachinery` strategic merge | controller-runtime | 輕量場景無需完整 controller 框架 |
| Kustomize 支援 | `sigs.k8s.io/kustomize/api` | shell exec | Go SDK 無需主機安裝 kustomize 二進位 |
| Container Registry | 標準 Docker Registry HTTP API v2 | go-containerregistry | Harbor 額外 API 單獨呼叫 |
| CLI 框架 | cobra + viper | urfave/cli | cobra 生態最大，kubectl/helm 皆採用 |
