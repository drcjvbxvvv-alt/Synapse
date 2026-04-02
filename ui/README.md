# Synapse — 企業級 Kubernetes DevOps 平台

> 多叢集管理 · AI 輔助運維 · 成本分析 · 完整可觀測性

Synapse 是一個開源的企業級 Kubernetes 多叢集管理平台，前端基於 React 19 + TypeScript + Ant Design 5，後端基於 Go + Gin + GORM 構建。目標是讓開發、運維、SRE 團隊在單一入口完成日常 K8s 工作，從資源管理、監控告警到 AI 診斷、成本分析，無需在多個工具之間切換。

---

## 功能總覽

### 🏗 多叢集管理
- **叢集清單**：匯入多個 K8s 叢集（支援 kubeconfig / 手動設定），統一展示健康狀態、版本、節點數、Pod 數及資源利用率
- **叢集詳情**：即時監控 CPU / 記憶體 / Pod 用量趨勢；叢集事件 Timeline；一鍵測試連線
- **全域儀表板**：跨叢集彙總異常工作負載、告警摘要、資源分佈、版本分佈
- **叢集指標**：整合 Prometheus，支援自定義查詢面板

---

### 🧩 工作負載管理

#### Deployment
- 列表 / 建立 / 縮放副本數 / 刪除
- 詳情頁含：Pod 實例、容器資訊、歷史版本（ReplicaSet）、服務 / Ingress 關聯、監控圖表、事件
- HPA 自動擴縮：建立 / 編輯 / 刪除 HorizontalPodAutoscaler，CPU 目標利用率設定

#### StatefulSet / DaemonSet
- 列表 / 縮放 / 刪除；詳情含關聯 Pod、事件

#### Job / CronJob
- 列表 / 建立 / 刪除；CronJob 暫停 / 恢復

#### Argo Rollouts（漸進式交付）
- Canary / Blue-Green 發布狀態展示
- Promote（推進一步 / 全部推進）/ Abort（中止）操控
- AnalysisRun 列表
- 拓撲視覺化

---

### 📦 Pod 管理
- 跨命名空間 Pod 列表，含狀態、資源用量、節點資訊
- Pod 詳情：容器列表、資源 Requests / Limits、掛載卷、環境變數、事件
- **即時日誌**：WebSocket 串流，支援多容器切換、搜尋關鍵字、自動捲動
- **Web Terminal**：透過 kubectl exec 直接進入容器互動
- AI 診斷：一鍵觸發 AI 分析 OOMKilled / CrashLoopBackOff / ImagePullBackOff 等問題

---

### 🌐 網路管理

#### Service
- ClusterIP / NodePort / LoadBalancer 列表；建立 / 編輯 / 刪除
- Endpoints 詳情

#### Ingress
- 列表 / 建立 / 編輯 / 刪除；支援多 Host / Path / TLS

#### NetworkPolicy
- 完整 CRUD；**拓撲視覺化**（以 D3 / 力導向圖呈現 Pod 間允許 / 拒絕流量）
- **建立精靈**：逐步引導設定 Ingress / Egress 規則，降低設定門檻

---

### 🗂 設定管理（ConfigMap / Secret）
- 列表 / 建立 / 編輯 / 刪除
- YAML 編輯器（Monaco Editor）內嵌，語法高亮 + Diff 預覽
- **版本歷史**：每次 Update 前自動快照；ConfigMap 支援一鍵回滾至任意版本；Secret 僅記錄 key 列表（值不儲存，安全考量）

---

### 💾 儲存管理
- PersistentVolume / PersistentVolumeClaim / StorageClass 列表
- PVC 建立 / 刪除；PVC 與 PV 綁定狀態視覺化

---

### 🏷 命名空間管理
- 列表 / 建立 / 刪除；標籤 / 注解管理
- **ResourceQuota CRUD**：建立 / 編輯 / 刪除命名空間資源配額（CPU、記憶體、Pod 數等）
- **LimitRange CRUD**：設定 Container / Pod 預設資源限制

---

### 🖥 節點管理
- 節點清單：狀態、角色、版本、CPU / 記憶體利用率
- 節點詳情：Pod 分佈、資源分配、系統資訊、事件
- 運維操作：Cordon / Uncordon / Drain

---

### 📊 監控與告警

#### Prometheus / Grafana 整合
- 監控設定（Prometheus URL + Grafana URL + API Key）
- 直接在平台內嵌入 Grafana Panel / Dashboard iframe
- 自定義 PromQL 查詢面板

#### AlertManager 整合
- 告警列表 / 告警組 / 統計圖
- Silences 管理（建立 / 刪除靜默規則）
- AlertManager 設定 YAML 編輯 + 驗證 + 一鍵套用
- 接收器（Receivers）管理

#### 自定義 Event 告警規則
- 依 K8s Event Reason / Namespace / 工作負載設定觸發條件
- 通知渠道：Webhook / DingTalk / **Slack** / **Microsoft Teams**（Adaptive Card 格式）
- 30 分鐘去重防抖；告警歷史紀錄

---

### 📋 日誌中心

#### K8s 容器日誌串流
- 多 Pod / 多容器同時串流（WebSocket）
- 日誌級別過濾（Error / Warn / Info / Debug）+ 關鍵字搜尋
- 自動捲動 / 暫停 / 清除

#### K8s Event 日誌
- Warning / Normal 事件列表；命名空間 / 關鍵字過濾；週期性自動重整

#### 日誌搜尋
- 跨叢集日誌全文搜尋，支援時間範圍篩選

#### 外部日誌源（Loki / Elasticsearch）
- **Loki 整合**：設定 Loki URL 後，支援 LogQL 查詢、時間範圍篩選
- **Elasticsearch 整合**：設定 ES URL / Index，支援 Lucene Query String 語法
- 日誌源 CRUD 管理（支援 Basic Auth / API Key 認證）
- 查詢結果以統一表格格式呈現（時間 / 級別 / 命名空間 / Pod / 訊息）

---

### 🔐 安全管理
- **Image 掃描（Trivy 整合）**：工作負載映像漏洞掃描；CVE 清單（Critical / High / Medium / Low）
- **CIS Benchmark（kube-bench）**：叢集基準評分；逐項檢查通過 / 失敗狀態
- **安全儀表板**：掃描結果彙總、高危漏洞排行

---

### 💰 成本分析
- **定價設定**：CPU / 記憶體單價（USD / TWD / CNY / JPY）
- **命名空間成本**：Bar Chart + 排行表；本月估算費用
- **工作負載成本**：按 Deployment / StatefulSet 分攤，含利用率進度條
- **趨勢分析**：6 個月歷史成本 Line Chart
- **浪費識別**：低利用率工作負載（CPU / Mem 利用率 < 10%）
- **CSV 匯出**：按月匯出成本報表

---

### 🤖 AI 智慧運維

#### AI 聊天助手
- 多 Provider 支援：OpenAI / Azure OpenAI / Anthropic Claude / Ollama（本地）
- **Tool Calling**：AI 可自動呼叫 K8s API 查詢 Pod、Deployment、事件、日誌等資訊
- 敏感資料過濾：Secret 值 / 含 password、token、key 的環境變數 → `[REDACTED]`
- 浮動面板設計，可在任意頁面叫出
- YAML 程式碼區塊自動偵測，提供「複製 YAML」與「套用至叢集」按鈕

#### AI 診斷
- Pod / Deployment 詳情頁一鍵觸發診斷，自動帶入資源狀態 prompt
- 診斷到 OOMKilled / CrashLoopBackOff / ImagePullBackOff 時，自動附帶 Runbook 解決步驟

#### 自然語言查詢（NL Query）
- 輸入中文自然語言（如「列出所有 OOMKilled 的 Pod」）
- AI 解析意圖 → 選擇適當 K8s 工具 → 執行查詢 → 摘要回傳

#### YAML 生成助手
- 輸入 `/yaml` 前綴描述需求（如 `/yaml 建立 nginx Deployment，2 副本，80 Port`）
- AI 回傳可直接套用的完整 YAML

#### Runbook 知識庫
- 10 個常見場景（OOMKilled / CrashLoopBackOff / 節點 NotReady 等）
- 支援關鍵字搜尋；以 Collapse 展開步驟式解決流程

---

### 🔄 GitOps — ArgoCD 整合
- **連線設定**：ArgoCD Server URL + Token 設定，連線測試
- **應用列表**：健康狀態 / 同步狀態 / 最後部署時間
- **操作**：Sync（同步）/ Rollback（回滾）/ Delete（刪除）
- **應用詳情**：資源樹狀結構、同步歷史

---

### ⎈ Helm 套件管理
- **Release 列表**：版本 / 狀態 / 命名空間 / 更新時間
- **安裝**：從倉庫選 Chart，填入 Values YAML
- **升級 / 回滾 / 刪除**
- **Values 查看**：User Values / All Values 對比
- **Chart 倉庫管理**：新增 / 刪除 Helm Repository

---

### 🧩 CRD 自定義資源
- 自動發現所有已安裝 CRD
- 通用 CRD 實例列表（dynamic client）
- 基本刪除操作

---

### 🖥 Web Terminal
- **Pod Exec Terminal**：直接進入 Pod 容器 Shell（WebSocket，完整 TTY）
- **kubectl Terminal**：叢集級 kubectl Shell，支援任意 kubectl 指令
- **SSH Terminal**：透過設定好的 SSH Key，直接 SSH 到節點（支援多跳板機）
- 會話記錄：指令歷史 / 操作時間 / 使用者追蹤

---

### 🔍 全域搜尋
- 跨叢集搜尋 Pod / Deployment / Service 等資源
- 依名稱 / 命名空間篩選；搜尋結果直接導航至詳情頁

---

### 👤 使用者與權限管理

#### 使用者管理
- 本地帳號 CRUD；LDAP 帳號同步
- 狀態管理（啟用 / 停用）；密碼重設
- 最後登入時間追蹤

#### 使用者組
- 群組 CRUD；成員新增 / 移除
- 組層級叢集權限設定

#### 叢集權限
- 四種角色：Admin / Ops / Dev / ReadOnly + 自定義
- 命名空間粒度（`["*"]` 或指定命名空間列表）
- 支援用戶層級與用戶組層級綁定

#### K8s RBAC
- ClusterRole / Role / ClusterRoleBinding / RoleBinding 查看
- 一鍵建立 / 解除權限綁定

---

### 🔒 稽核與合規

#### 操作日誌
- 所有 API 操作自動記錄（使用者 / 操作類型 / 資源 / 執行結果）
- 支援按時間範圍、操作類型、使用者篩選

#### Terminal 會話稽核
- 記錄 kubectl / Pod Exec / SSH 三種 Terminal 的所有指令
- 會話列表 + 指令詳情逐筆查詢

---

### ⚙️ 系統設定
- **LDAP 整合**：LDAP Server URL / Bind DN / 用戶 / 群組搜尋設定
- **AI 設定**：Provider 切換（OpenAI / Azure / Claude / Ollama）；API Key / Endpoint / 模型設定
- **Grafana 整合**：Grafana URL / API Key / Dashboard ID 設定
- **系統安全**：JWT Secret 強制設定；AES-256-GCM 憑證欄位加密（`ENCRYPTION_KEY` 環境變數）
- **多叢集同步策略**：設定 ConfigMap / Secret 跨叢集同步規則（來源 / 目標 / 衝突策略 / Cron）

---

## 技術架構

### 後端
| 技術 | 用途 |
|------|------|
| **Go 1.22+** | 後端主語言 |
| **Gin** | HTTP 框架 |
| **GORM** | ORM（支援 SQLite / MySQL） |
| **client-go** | Kubernetes API 客戶端 |
| **helm.sh/helm/v3** | Helm SDK |
| **gorilla/websocket** | WebSocket（Terminal / 日誌串流） |
| **golang-jwt/jwt** | JWT 認證 |
| **bcrypt** | 密碼雜湊 |
| **AES-256-GCM** | kubeconfig 欄位加密 |

### 前端
| 技術 | 用途 |
|------|------|
| **React 19** | UI 框架 |
| **TypeScript 5.8** | 型別安全 |
| **Vite 7** | 建構工具 |
| **Ant Design 5** | UI 元件庫 |
| **Monaco Editor** | YAML / 程式碼編輯器 |
| **@monaco-editor/react** | Monaco Diff Viewer |
| **recharts** | 圖表（成本趨勢 / 監控） |
| **react-window** | 大列表虛擬捲動 |
| **@tanstack/react-query** | API 快取與狀態管理 |
| **react-force-graph-2d** | NetworkPolicy 拓撲視覺化 |
| **dayjs** | 時間處理 |
| **i18next** | 多語言（繁中 / 簡中 / 英文） |

---

## 快速開始

### 環境需求
- Node.js >= 18
- Go >= 1.22
- kubectl（可選，用於 kubectl Terminal 功能）

### 前端開發
```bash
cd ui
npm install
npm run dev
# 訪問 http://localhost:5173
```

### 後端開發
```bash
# 複製設定檔
cp config.yaml.example config.yaml

# 啟動（預設使用 SQLite）
go run main.go

# 預設管理員帳號：admin / Synapse@2026
```

### 建構生產版本
```bash
# 前端建構（嵌入後端二進位）
cd ui && npm run build

# 後端建構（含嵌入前端靜態資源）
go build -o synapse main.go

# 啟動
./synapse
```

### 環境變數
| 變數 | 說明 | 預設值 |
|------|------|--------|
| `ENCRYPTION_KEY` | kubeconfig AES-256-GCM 加密金鑰（必填，生產環境） | — |
| `JWT_SECRET` | JWT 簽名密鑰（release 模式必填） | — |
| `DB_DRIVER` | 資料庫驅動（`sqlite` / `mysql`） | `sqlite` |
| `DB_DSN` | SQLite 路徑或 MySQL DSN | `./data/synapse.db` |
| `LOG_FORMAT` | 日誌格式（`text` / `json`） | `text` |
| `INFORMER_SYNC_TIMEOUT` | K8s Informer 同步超時（秒） | `30` |

---

## 專案結構

```
Synapse/
├── main.go                    # 應用入口
├── internal/
│   ├── handlers/              # HTTP Handler（業務邏輯入口）
│   ├── services/              # 服務層（核心業務邏輯）
│   ├── models/                # GORM 資料模型
│   ├── middleware/            # Gin Middleware（Auth / CORS / Rate Limit / Audit）
│   ├── router/                # 路由設定
│   ├── k8s/                   # K8s 客戶端封裝 + Informer 管理
│   ├── database/              # 資料庫初始化與遷移
│   ├── config/                # 設定結構
│   └── response/              # 統一回應格式
├── pkg/
│   ├── logger/                # 結構化日誌（slog）
│   └── crypto/                # AES-256-GCM 加密套件
├── runbooks/                  # AI Runbook 知識庫（JSON）
└── ui/                        # React 前端
    └── src/
        ├── pages/             # 頁面元件
        ├── services/          # API 呼叫層
        ├── layouts/           # 主佈局
        ├── components/        # 共用元件
        └── utils/             # 工具函式
```

---

## 預設帳號

| 欄位 | 值 |
|------|-----|
| 帳號 | `admin` |
| 密碼 | `Synapse@2026` |

> ⚠️ 生產環境請立即修改預設密碼，並設定 `ENCRYPTION_KEY` 與 `JWT_SECRET`。

---

## 授權

本專案以 MIT License 授權開源。
