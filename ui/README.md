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

## 系統分析報告

> 本節為對 Synapse 程式庫的深度技術審查，涵蓋可靠度、實用性、架構、安全性等九個維度，基於實際原始碼分析，誠實呈現優勢與已知缺陷。

### 總覽評分

| 維度 | 評分 | 核心結論 |
|------|------|----------|
| 可靠度 | 7/10 | 主路徑錯誤處理完整；Runbooks 載入失敗會 panic 崩潰整個服務 |
| 實用性 | 8/10 | 涵蓋 90% 日常 K8s 操作；Mesh 指標與部分成本計算為 stub |
| 可用性 | 7/10 | API 回應格式一致；TLS 預設跳過驗證未在 UI 警示 |
| 誠實性 | 6/10 | 功能列表大致符實；Istio 指標豐富度與測試覆蓋率未公開揭示 |
| 系統架構 | 8/10 | 分層清晰、依賴注入到位；服務層缺乏介面抽象，難以單元測試 |
| 穩定度 | 7/10 | 並發控制正確；工具函式使用 panic 代替 error return |
| 程式碼品質 | 6/10 | 日誌與錯誤包裝一致；YAML handler 重複度高，長方法未拆分 |
| 效能 | 8/10 | Informer 快取 + 連線池設計完善；Pod 操作存在 N+1 風險 |
| 安全性 | 5/10 | 審計與 RBAC 完整；**kubeconfig 明文儲存為重大漏洞** |

---

### 1. 可靠度 (Reliability)

**優勢**
- 所有 K8s 操作帶 `context.WithTimeout`（30 秒），防止無限等待（`internal/services/k8s_client.go`）
- Informer 快取同步等待有 timeout 守衛（`internal/k8s/manager.go`）
- Gin `Recovery()` middleware 兜住未預期 panic，避免服務中斷

**已知缺陷**
- `internal/runbooks/runbooks.go`：JSON 解析失敗直接呼叫 `panic()`，會讓整個程序崩潰，應改為 error return
- `internal/services/mesh_service.go`：`enrichWithMetrics()` 為完整 stub，Istio RPS / 錯誤率 / P99 欄位永遠為 0，但呼叫端不知情
- 部分 handler 同時記錄日誌又回傳錯誤，另一些則只做其中一件事，行為不一致

---

### 2. 實用性 (Practicality)

**優勢**
- Deployment / StatefulSet / DaemonSet / Job / CronJob / Argo Rollouts 完整覆蓋
- Web Terminal（Pod exec、kubectl、SSH）、日誌串流、YAML 編輯器均已實作
- AI 診斷整合 4 個 Provider，含敏感資料過濾

**已知缺陷**
- **成本分析**：`internal/services/cost_service.go` 有佔位符註解「實際實作需要 metrics-server API；此處為預留 placeholder」，部分成本數字可能為估算值
- **Mesh 拓撲指標**：`MeshNode.RPS / ErrorRate / P99` 欄位已定義但永遠為零（`internal/services/mesh_service.go:enrichWithMetrics`）
- **多處 `context.TODO()`**：`internal/handlers/namespace.go` 有 14 處 `context.TODO()`，生產環境應改為帶 deadline 的 context

---

### 3. 可用性 (Usability)

**優勢**
- 統一回應格式（`internal/response/` 模組），前端解析一致
- WebSocket 重連採指數退避（`ui/src/hooks/useWebSocket.ts`），最大 10 次、上限 30 秒，防止連線風暴
- RBAC 權限粒度細到命名空間級別

**已知缺陷**
- 403 錯誤僅顯示「無権存取」，不告知使用者實際可存取的命名空間範圍
- `internal/services/k8s_client.go`：預設跳過 TLS 驗證（`InsecureSkipVerify: true`），介面上沒有任何提示，使用者可能不知道流量未受保護

---

### 4. 誠實性 (Honesty / Accuracy)

**與實際相符之處**
- 工作負載、儲存、設定、RBAC、AI、GitOps、Helm、CRD 功能均已實作，與 README 一致
- 審計日誌、SIEM Webhook、Terminal 回放均有實際 handler 支撐

**需要補充說明之處**
- Service Mesh 拓撲的流量指標（RPS、P99、錯誤率）目前為預留欄位，資料未實際填入
- 後端測試覆蓋率約 25%（47 個 handler 對應約 12 個測試檔），前端測試覆蓋率更低，有別於企業級平台通常的期待

---

### 5. 系統架構 (Architecture)

**優勢**
- Handler → Service → K8s Manager → client-go 分層清晰，無循環依賴
- `Router.Setup()` 集中建立所有服務實例並注入，依賴關係明確（`internal/router/router.go`）
- `ClusterInformerManager` 封裝 Informer 生命週期，含閒置 GC（2 小時），防止記憶體洩漏

**已知缺陷**
- 服務層均為具體型別，無介面定義，難以注入 mock 進行單元測試
- 每個 handler 直接持有 `*gorm.DB`，無統一的 repository 層；跨表事務邊界不清晰
- `internal/middleware/permission.go` 中硬編碼 `username == "admin"` 超級管理員邏輯，應改為基於 role 的設計

---

### 6. 穩定度與可靠性 (Stability)

**優勢**
- `ClusterInformerManager` 使用 `sync.RWMutex` 保護叢集 map，讀多寫少場景下效能與安全性兼顧
- 閒置叢集 GC goroutine（`internal/k8s/manager.go`）防止長時間累積的資源洩漏
- 所有背景 Worker（EventAlert、Cost、LogRetention）在 `Router.Setup()` 統一啟動，生命週期可管理

**已知缺陷**
- `internal/runbooks/runbooks.go` 的 `panic` 在服務啟動時若 Runbook JSON 格式有誤，會使整個程序崩潰
- 高並發下無請求佇列保護：K8s client 的 QPS/Burst 設定（100/200）不足時，超出部分會直接回傳錯誤而非排隊等待

---

### 7. 程式碼品質 (Code Quality)

**優勢**
- `fmt.Errorf("...: %w", err)` 錯誤包裝模式全面使用，方便追蹤根源
- `pkg/logger` 結構化日誌含 key-value context，方便 log aggregation
- 多語言（i18n）覆蓋所有主要 UI 字串

**已知缺陷**
- `ui/src/services/yaml*Service.ts`（yamlDeploymentService、yamlStatefulSetService 等）內容高度重複，約 80% 程式碼可抽共用
- `internal/handlers/deployment.go`：`ListDeployments` 方法超過 110 行，應拆分為子函式
- WebSocket buffer size（1024）硬編碼於多處，未定義為具名常數

---

### 8. 效能與資源消耗 (Performance)

**優勢**
- K8s Informer 無週期性 resync（`resyncPeriod = 0`），避免不必要的記憶體寫入壓力
- HTTP 連線池：`MaxIdleConnsPerHost = 100`（`internal/services/k8s_client.go`）
- 所有列表 API 均有分頁（預設 pageSize=20），防止大型叢集回應過大
- Gzip 壓縮套用於所有非 WebSocket 路由（`internal/router/router.go`）

**已知缺陷**
- Pod 相關操作可能觸發 N+1 問題：列出 Pod 後，再為每個 Pod 個別查詢 metrics/events
- 全域搜尋為跨多叢集即時查詢，叢集數量多時延遲線性增長，無結果快取層
- 啟動時 `GetAllClusters()` 將所有叢集資料載入記憶體（`internal/router/router.go`），叢集數量大時影響啟動速度

---

### 9. 安全性 (Security)

**優勢**
- JWT 驗證、RBAC 中介層、命名空間粒度權限控制完整
- AI 查詢前自動過濾 PEM 憑證、Secret 值、含 `password/token/key` 的環境變數（`internal/services/ai_sanitizer.go`）
- 操作稽核日誌覆蓋所有寫入操作，含使用者、資源、結果
- 登入端點有 Rate Limiting（5次/分鐘，鎖定15分鐘）

**重大風險（需在生產環境修復後才可部署）**

> ⚠️ **CRITICAL**：`internal/handlers/cluster.go` 中的 kubeconfig、SA Token、CA 憑證在程式碼層面標記為待加密（TODO），目前以明文儲存於資料庫。任何能存取資料庫的人可取得所有叢集的完整憑證。

> ⚠️ **HIGH**：`internal/services/k8s_client.go` 預設 `InsecureSkipVerify: true`，除非提供 CA 憑證，否則與 K8s API Server 的通訊不驗證憑證，易受中間人攻擊。

**其他安全建議**
- 定期輪換 `ENCRYPTION_KEY` 與 `JWT_SECRET`，並確保這兩個環境變數在生產環境中已設定
- 考慮為 kubeconfig 加入欄位級加密（已有 `pkg/crypto` AES-256-GCM 套件，應在 GORM hooks 中啟用）

---

### 生產部署前必做清單

```
[ ] 確認 ENCRYPTION_KEY 已設定，且 kubeconfig 欄位加密已啟用
[ ] 確認 JWT_SECRET 已設定（release 模式下系統會強制驗證）
[ ] 評估是否為各叢集提供 CA 憑證以啟用 TLS 驗證
[ ] 將 runbooks.go 的 panic 改為 error return
[ ] 將 namespace.go 的 context.TODO() 改為帶 timeout 的 context
[ ] 若使用 Mesh 拓撲，告知使用者 RPS/ErrorRate/P99 指標目前未實作
[ ] 定期備份資料庫（含所有叢集憑證）並加密備份檔案
```

---

## 授權

本專案以 MIT License 授權開源。
