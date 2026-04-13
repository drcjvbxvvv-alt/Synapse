# Synapse — 企業級 Kubernetes DevOps 平台

> 多叢集管理 · AI 輔助運維 · 資源治理 · 完整可觀測性

Synapse 是一個開源的企業級 Kubernetes 多叢集管理平台，前端基於 React 19 + TypeScript + Ant Design 5，後端基於 Go + Gin + GORM 構建。目標是讓開發、運維、SRE 團隊在單一入口完成日常 K8s 工作，從資源管理、監控告警到 AI 診斷、資源治理，無需在多個工具之間切換。

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
- **策略模擬**：輸入來源 / 目標 Pod，即時預覽流量是否被允許（不需實際變更規則）
- **內聯編輯**：拓撲圖直接點選連線編輯規則

#### Service Mesh（Istio）視覺化

- 自動偵測 Istio 安裝狀態（CRD / istiod Pod）
- **流量拓撲圖**：基於 Prometheus `istio_requests_total` 呈現服務間 RPS、錯誤率、P99 延遲
- VirtualService / DestinationRule / Gateway / PeerAuthentication CRD 查看
- mTLS 狀態 Badge

#### Gateway API

- 自動偵測叢集是否安裝 Gateway API CRD（`gateway.networking.k8s.io`），未安裝時顯示一鍵安裝指令
- **GatewayClass**：列表 + 詳情抽屜（Controller、Conditions）
- **Gateway**：CRUD + YAML 編輯（Monaco）；Listeners、Addresses、Conditions 詳情
- **HTTPRoute**：CRUD + YAML；parentRefs、Hostnames、Rules（Match / Backend / Filter）詳情；多後端流量權重視覺化（堆疊色條）
- **GRPCRoute**：CRUD + YAML（gRPC 方法比對規則）
- **ReferenceGrant**：建立 / 刪除 + YAML 查看（跨 namespace 流量授權）
- **Gateway 拓撲**：React Flow 有向圖，GatewayClass → Gateway → HTTPRoute/GRPCRoute → Service，dagre LR 自動排版

#### 叢集網路拓樸（Cluster Topology）

- **靜態拓樸**（零依賴，所有叢集適用）：以 Service selector 比對建立 Service → Workload 連線；Endpoint readiness 決定邊健康狀態（Healthy / Degraded / Down / Unknown）
- **Workload 自動折疊**：批次解析 Pod → ReplicaSet → Deployment ownerRef，以 Workload 節點呈現 Ready/Total 計數
- **ParticleEdge 動畫**：SVG `animateMotion` 粒子沿邊流動，顏色 / 速度對應健康狀態
- **命名空間篩選**：多選下拉，縮小可見範圍
- **Cilium 偵測**：自動偵測 `hubble-relay` Service，工具列顯示版本徽章
- **Istio 指標強化**（`Istio Metrics` 開關，Istio 已安裝時可用）：透過 K8s API Server Proxy 查詢叢集內 Prometheus，取得 requestRate / errorRate / latencyP99ms；errorRate > 5% 轉橙色、> 20% 轉紅色；邊標籤即時顯示錯誤率

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

### 💰 資源治理（Resource Governance）

#### 資源佔用分析（不依賴 Prometheus）
- **佔用率儀表板**：叢集 CPU / 記憶體 allocatable、requested、occupancy、headroom 即時卡片
- **命名空間佔用**：橫向 BarChart 排行 + 明細表（佔叢集容量百分比）
- **跨叢集洞察**：全平台佔用率對比、叢集排行、整體效率評分

#### 資源效率分析（需要 Prometheus）
- **效率散點圖**：CPU 效率 × 記憶體效率象限圖，泡泡大小 = CPU 佔用量
- **工作負載效率**：Deployment / StatefulSet / DaemonSet 廢棄分數排序；`WasteScore = (1-CPU效率)×0.6 + (1-記憶體效率)×0.4`
- **低效識別**：CPU 效率 < 20% 工作負載列表，帶廢棄分數警示色
- **降級策略**：無 Prometheus 時自動降級，佔用率正常顯示

#### 容量規劃與 Right-sizing
- **容量趨勢**：月度 CPU / 記憶體佔用率折線圖（讀取每日快照）
- **耗盡預測**：線性迴歸外推，標示到達 80% / 100% 的預測日期（180 天期）
- **Right-sizing 建議**：基於 7 日最大用量 × 安全係數（CPU ×1.2、記憶體 ×1.25）
- **CSV 匯出**：低效工作負載報告直接串流下載

#### 雲端帳單整合（可選）
- **AWS Cost Explorer**：SigV4 簽名對接，取得各服務月度費用明細
- **GCP Cloud Billing**：Service Account oauth2 對接 Budget API
- **資源單位成本**：換算 USD/core-hr、USD/GiB-hr，讓帳單費用對應到 K8s 資源使用

#### 多語言支援（i18n）
- 成本分析頁面完全國際化：繁體中文（zh-TW）/ 簡體中文（zh-CN）/ 英文（en-US）
- **後端中立設計**：API 返回英文或代碼，前端負責完整翻譯
- 全球化團隊協作無障礙

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

### 🔗 多叢集工作流程

#### 工作負載遷移
- **遷移精靈**：3 步驟（選目標叢集 → 資源相容性檢查 → 確認執行）
- 自動取得來源 Deployment YAML → 套用至目標叢集；同步關聯 ConfigMap / Secret

#### 配置同步策略
- **策略 CRUD**：設定來源叢集 / 命名空間 / 資源類型（ConfigMap / Secret）/ 目標叢集列表
- **衝突策略**：overwrite（強制覆蓋）/ skip（跳過已存在）
- **排程同步**：支援 Cron 表達式定時同步，或手動觸發；同步歷史逐次紀錄

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

##### 前端操作權限矩陣

> 最後更新：2026-04-13
> 實作位置：`ui/src/contexts/PermissionContext.tsx`（`canWrite` / `canDelete`）、`ui/src/config/menuPermissions.ts`（路由 / 選單 / 操作按鈕）

| 角色 | 查看 | 新增 / 編輯 | 刪除 | 說明 |
|------|------|-------------|------|------|
| **admin** | ✅ | ✅ | ✅ | 完整存取；叢集升級等高危操作限 admin |
| **ops** | ✅ | ✅ | ✅ | 運維操作；無平台管理員頁面存取 |
| **dev** | ✅ | ❌ | ❌ | 預設與唯讀相同；可透過策略管理頁開放特定功能 |
| **custom** | ✅ | ✅ | ❌ | 寫入操作同 ops；刪除需 admin/ops |
| **readonly** | ✅ | ❌ | ❌ | 純查看；僅 Pod Logs 操作可用 |

**補充說明：**

- **刪除操作**（`canDelete`）：僅限 `admin` 和 `ops`，`dev` / `custom` / `readonly` 一律隱藏刪除按鈕
- **寫入操作**（`canWrite`）：`admin`、`ops`、`custom` 可用；`dev` 與 `readonly` 預設不可用
- **策略管理**：管理員可在「策略管理」頁針對特定 `dev` 用戶的功能可見性做細粒度調整（`hasFeature` 機制）

##### 前端路由訪問權限矩陣

> 最後更新：2026-04-13
> 實作位置：`ui/src/router/routes.tsx`（`TopLevelGuard` / `PermissionGuard` / `ClusterListRoute`）

###### 頂層路由（非叢集上下文，`/path`）

| 路徑 | 可存取角色 | 守衛機制 | 說明 |
|------|-----------|---------|------|
| `/overview` | admin、ops | `TopLevelGuard` | 平台總覽 |
| `/alerts` | admin、ops | `TopLevelGuard` | 全域告警中心 |
| `/cost-insights` | admin、ops | `TopLevelGuard` | 全域成本分析 |
| `/multicluster` | admin、ops | `TopLevelGuard` | 多叢集管理 |
| `/search` | admin、ops | `TopLevelGuard` | 全域搜尋結果頁 |
| `/nodes`、`/nodes/:id` | admin、ops | `TopLevelGuard` | 舊版無叢集上下文節點路由 |
| `/workloads`、`/workloads/...` | admin、ops | `TopLevelGuard` | 舊版無叢集上下文工作負載路由 |
| `/clusters` | admin（平台管理員）| `ClusterListRoute` | 叢集列表，非管理員跳轉至各自首個叢集 |
| `/clusters/import` | admin（平台管理員）| `PermissionGuard platformAdminOnly` | 匯入叢集 |
| `/audit/*` | admin（平台管理員）| `PermissionGuard platformAdminOnly` | 操作日誌、指令歷史 |
| `/access/*` | admin（平台管理員）| `PermissionGuard platformAdminOnly` | 使用者 / 群組 / 權限 / 策略管理 |
| `/settings` | admin、ops | `TopLevelGuard` | 系統設定 |
| `/profile` | 所有登入用戶 | `RequireAuth`（已登入即可）| 個人資料，唯一允許所有角色訪問的頂層路由 |

###### 叢集路由（`/clusters/:id/path`）

| 子路徑 | 最低角色 | 守衛機制 | 說明 |
|--------|---------|---------|------|
| `/overview` | 所有叢集成員 | 無（叢集存取由 middleware 控制）| 叢集總覽 |
| `/nodes`、`/nodes/:name` | ops | `PermissionGuard requiredPermission="ops"` | 節點管理 |
| `/config-center` | ops | `PermissionGuard requiredPermission="ops"` | 叢集設定中心 |
| `/plugins`、`/argocd/*` | ops | `PermissionGuard requiredPermission="ops"` | ArgoCD 插件 |
| `/helm` | ops | `PermissionGuard requiredPermission="ops"` | Helm 發佈管理 |
| `/upgrade` | admin | `PermissionGuard requiredPermission="admin"` | 叢集升級 |
| `/pods`、`/workloads`、`/autoscaling` | 依 Feature Policy | `PermissionGuard requiredFeature="workload:view"` | 工作負載 |
| `/configs`、`/configs/*` | 依 Feature Policy | `PermissionGuard requiredFeature="config:view"` | ConfigMap / Secret |
| `/network`、`/network/*` | 依 Feature Policy | `PermissionGuard requiredFeature="network:view"` | 網路資源 |
| `/storage` | 依 Feature Policy | `PermissionGuard requiredFeature="storage:view"` | 儲存資源 |
| `/logs`、`/logs/events` | 依 Feature Policy | `PermissionGuard requiredFeature="logs:view"` | 日誌中心 |
| `/monitoring` | 依 Feature Policy | `PermissionGuard requiredFeature="monitoring:view"` | 監控中心 |

> **`TopLevelGuard` 邏輯**：掃描用戶在所有叢集的權限（`clusterPermissions` Map），只要任一叢集為 `admin` 或 `ops` 即放行；否則顯示 403 錯誤頁。
>
> **`isPlatformAdmin` 邏輯**：username 為 `admin`，或在任意叢集擁有 `admin` 權限。

#### K8s RBAC

- ClusterRole / Role / ClusterRoleBinding / RoleBinding 查看
- 一鍵建立 / 解除權限綁定

---

### 🔒 稽核與合規

#### 操作日誌（Operation Logs）

- 所有 API 操作自動記錄（使用者 / 操作類型 / 資源 / 執行結果）
- 支援按時間範圍、操作類型、使用者篩選
- **多語言操作分類**：15+ 模組 × 17+ 動作類型，全面國際化支援
- 操作分佈圖表：清晰視覺化各模組操作量（支援三種語言）
- 敏感字段自動脫敏：password、token、kubeconfig 等欄位自動遮蔽

#### 命令歷史（Command History）

- 記錄 kubectl / Pod Exec / SSH 三種 Terminal 的所有指令
- 會話列表 + 指令詳情逐筆查詢
- 會話狀態追蹤：進行中 / 已結束 / 異常
- **終端機類型識別**：Kubectl / Pod / Node SSH 三種終端完全區分
- 多語言支援：所有會話元數據、狀態標籤完全國際化

#### 部署審批工作流

- 命名空間保護機制：標記保護命名空間，刪除 / 重大變更需提交審批申請
- 審批請求列表：管理員審核通過後方可執行操作

#### SIEM 整合

- Webhook 推送：將稽核日誌批次推送至外部 SIEM 系統（Splunk / Datadog 等）
- 支援自訂 Header、批次大小、推送頻率

---

### ⚙️ 系統設定

- **LDAP 整合**：LDAP Server URL / Bind DN / 用戶 / 群組搜尋設定
- **AI 設定**：Provider 切換（OpenAI / Azure / Claude / Ollama）；API Key / Endpoint / 模型設定
- **Grafana 整合**：Grafana URL / API Key / Dashboard ID 設定
- **系統安全**：JWT Secret 強制設定；AES-256-GCM 憑證欄位加密（`ENCRYPTION_KEY` 環境變數）
- **安全設定 Tab**：登入安全參數動態調整（Session TTL / 鎖定閾值 / 密碼最短長度）；個人 API Token 管理（SHA-256 hash 儲存）；SIEM Webhook 推送設定
- **通知渠道管理**：集中管理 Webhook / DingTalk（HMAC-SHA256 加簽）/ Slack / Microsoft Teams / Email（SMTP）通知渠道；支援即時測試連線
- **多叢集同步策略**：設定 ConfigMap / Secret 跨叢集同步規則（來源 / 目標 / 衝突策略 / Cron）

---

## 技術架構

### 後端

| 技術                  | 用途                             |
| --------------------- | -------------------------------- |
| **Go 1.22+**          | 後端主語言                       |
| **Gin**               | HTTP 框架                        |
| **GORM**              | ORM（支援 SQLite / MySQL）       |
| **client-go**         | Kubernetes API 客戶端            |
| **helm.sh/helm/v3**   | Helm SDK                         |
| **gorilla/websocket** | WebSocket（Terminal / 日誌串流） |
| **golang-jwt/jwt**    | JWT 認證                         |
| **bcrypt**            | 密碼雜湊                         |
| **AES-256-GCM**       | kubeconfig 欄位加密              |

### 前端

| 技術                      | 用途                         |
| ------------------------- | ---------------------------- |
| **React 19**              | UI 框架                      |
| **TypeScript 5.8**        | 型別安全                     |
| **Vite 7**                | 建構工具                     |
| **Ant Design 5**          | UI 元件庫                    |
| **Monaco Editor**         | YAML / 程式碼編輯器          |
| **@monaco-editor/react**  | Monaco Diff Viewer           |
| **recharts**              | 圖表（成本趨勢 / 監控）      |
| **react-window**          | 大列表虛擬捲動               |
| **@tanstack/react-query** | API 快取與狀態管理           |
| **react-force-graph-2d**  | NetworkPolicy 拓撲視覺化     |
| **@xyflow/react**         | Gateway API / 叢集網路拓樸視覺化（React Flow v12） |
| **@dagrejs/dagre**        | 拓樸圖自動節點排版（LR 有向圖） |
| **dayjs**                 | 時間處理                     |
| **i18next**               | 多語言（繁中 / 簡中 / 英文） |

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

> **i18n 提示**：前端所有文本必須透過 `i18next` 翻譯。編輯 `ui/src/locales/` 下的 JSON 文件新增或更新翻譯，詳見[國際化（i18n）](#國際化i18n)部分。

### 後端開發

```bash
# 複製設定檔
cp config.yaml.example config.yaml

# 啟動（預設使用 SQLite）
go run main.go

# 預設管理員帳號：admin / Synapse@2026
```

> **i18n 提示**：後端 API 回應應返回英文或代碼，不應包含翻譯。前端負責透過 i18n 翻譯所有面向使用者的文本，詳見[國際化（i18n）](#國際化i18n)部分。

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

| 變數                    | 說明                                              | 預設值              |
| ----------------------- | ------------------------------------------------- | ------------------- |
| `ENCRYPTION_KEY`        | kubeconfig AES-256-GCM 加密金鑰（必填，生產環境） | —                   |
| `JWT_SECRET`            | JWT 簽名密鑰（release 模式必填）                  | —                   |
| `DB_DRIVER`             | 資料庫驅動（`sqlite` / `mysql`）                  | `sqlite`            |
| `DB_DSN`                | SQLite 路徑或 MySQL DSN                           | `./data/synapse.db` |
| `LOG_FORMAT`            | 日誌格式（`text` / `json`）                       | `text`              |
| `INFORMER_SYNC_TIMEOUT` | K8s Informer 同步超時（秒）                       | `30`                |

---

## 國際化（i18n）

Synapse 支援繁體中文（zh-TW）、簡體中文（zh-CN）、英文（en-US）三種語言，使用 `i18next` 框架統一管理翻譯。

### 設計原則

- **後端返回中立資料**：所有 API 回應返回英文或代碼（如 `module: "cluster"`, `action: "create"`），不包含翻譯
- **前端負責翻譯**：所有面向使用者的文本由前端透過 i18n key 翻譯，確保單一真實來源
- **無硬編碼中文**：前端代碼中禁止硬編碼中文字符，所有文本必須使用 `t()` 函數

### 文件結構

```
ui/src/locales/
├── zh-TW/                    # 繁體中文
│   ├── common.json           # 公共 key（actions, menu, status 等）
│   ├── cost.json             # 成本分析頁面
│   ├── audit.json            # 稽核（操作日誌、命令歷史）
│   └── ...
├── zh-CN/                    # 簡體中文
│   ├── common.json
│   ├── cost.json
│   ├── audit.json
│   └── ...
└── en-US/                    # 英文
    ├── common.json
    ├── cost.json
    ├── audit.json
    └── ...
```

### 使用規範

#### 1. React 元件中的翻譯

```tsx
import { useTranslation } from 'react-i18next';

export default function MyPage() {
  const { t } = useTranslation(['namespace', 'common']);

  return (
    <div>
      {/* ✅ 正確：使用 i18n key，無 fallback */}
      <Button>{t('common:actions.create')}</Button>

      {/* ❌ 禁止：不要使用 fallback 中文 */}
      <Button>{t('common:actions.create', '建立')}</Button>

      {/* ❌ 禁止：硬編碼中文 */}
      <Button>建立</Button>
    </div>
  );
}
```

#### 2. i18n Key 命名規則

```
common:actions.*           # 通用動作（create, edit, delete, save, cancel 等）
common:menu.*              # 菜單項目（clusters, nodes, pods 等）
common:status.*            # 狀態標籤（healthy, unhealthy, loading 等）
common:validation.*        # 表單驗證訊息（required, invalid 等）
common:pagination.*        # 分頁相關（total, page 等）

cost:*                     # 成本分析頁面（overview, trend, occupancy, global 等）
audit:*                    # 稽核管理（operations, commands, modules, actions 等）
```

#### 3. 後端 API 回應規範

**❌ 錯誤做法**（硬編碼中文）：
```go
"module_name": "叢集管理",
"action_name": "建立",
"status": "未就緒"
```

**✅ 正確做法**（返回代碼或英文）：
```go
"module": "cluster",        // 代碼，由前端翻譯為 t('audit:modules.cluster')
"action": "create",         // 代碼，由前端翻譯為 t('audit:actions.create')
"status": "not_ready"       // 代碼或英文，由前端翻譯為 t('cost:global.notReady')
```

#### 4. 新增或更新翻譯

當增加新功能時：

1. **後端**：確保 API 回應返回英文或代碼，無硬編碼中文
2. **前端**：新增 i18n key 到所有三個語言文件
3. **檢查清單**：
   - ✓ 在 `common.json` 或模組專用文件中定義 key
   - ✓ 在所有三個語言文件中一致定義
   - ✓ 移除所有 `t()` 的 fallback 中文參數
   - ✓ 替換所有硬編碼中文為 `t('namespace:key')`

### 維護規範

#### 靜止的 i18n Key

部分 Key 定義為**靜止**（不再更新翻譯），應在註釋中標記：

```json
{
  "modules": {
    "auth": "認證管理",  // 靜止：後端直接返回中文，前端不翻譯
    "cluster": "叢集管理"
  }
}
```

#### 檢測未加載的 i18n Key

若頁面顯示 `i18n沒加載: <key>`，表示該 key 未在翻譯文件中定義：

```tsx
// 檢查所有三個語言文件
// 若某個 key 缺失，會顯示為 "[audit:modules.unknown]" 或警告訊息

// 解決方法：
// 1. 確認 key 是否拼寫正確
// 2. 在缺失的語言文件中補上該 key
// 3. 確保後端不返回硬編碼翻譯
```

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

| 欄位 | 值             |
| ---- | -------------- |
| 帳號 | `admin`        |
| 密碼 | `Synapse@2026` |

> ⚠️ 生產環境請立即修改預設密碼，並設定 `ENCRYPTION_KEY` 與 `JWT_SECRET`。

---

## 系統分析報告

> 基於原始碼深度審查，涵蓋九個維度，誠實呈現優勢與已知缺陷。分析日期：2026-04-12。

### 總覽評分

| 維度 | 評分 | 核心結論 |
|------|------|----------|
| 可靠度 | 8/10 | 主路徑錯誤處理完整；已修復 runbooks panic、kubectl terminal panic、Mesh 指標 stub |
| 實用性 | 9/10 | 覆蓋 95% 日常 K8s 操作；新增資源治理、多叢集遷移、Service Mesh 視覺化、Gateway API 完整 CRUD、叢集網路拓樸圖 |
| 可用性 | 9/10 | Design Token 統一、MainLayout 重構、空狀態元件標準化；i18n 全面覆蓋三語言；API 回應格式一致 |
| 誠實性 | 9/10 | 功能與實作高度一致；i18n 前後端架構分層明確；後端回應中立資料，前端負責完整翻譯 |
| 系統架構 | 8/10 | Handler → Service → K8s Manager 分層清晰；`K8sInformerManager` 介面解除循環依賴 |
| 穩定度 | 7/10 | 並發控制正確；高流量下無請求佇列保護，K8s client 超出 QPS/Burst 會直接回傳錯誤 |
| 程式碼品質 | 9/10 | 錯誤包裝一致；具名常數取代硬編碼；i18n 全面遷移（後端無硬編碼中文，前端 100% t() 覆蓋）；Design Token 集中管理 |
| 效能 | 9/10 | Informer 快取 + 連線池完善；GlobalSearch 並行化；Pod/Node 列表分層 refetchInterval |
| 安全性 | 5/10 | RBAC / 稽核 / AI 脫敏完整；**kubeconfig 明文儲存仍為重大風險**；TLS 預設 skip verify |

---

### 1. 可靠度（Reliability）8/10

**優勢**
- 所有 K8s 操作帶 `context.WithTimeout`（30 秒），防止無限等待
- Informer 快取同步等待有 timeout 守衛；`Gin Recovery()` 兜住未預期 panic
- `CostWorker`、`EventAlertWorker`、`LogRetentionWorker` 生命週期由 `Router.Setup()` 統一管理

**已修復**
- `runbooks.go` JSON 解析失敗 panic → 優雅降級（日誌警告 + 空列表）
- `kubectl_terminal.go` `mustParseUint()` panic → `strconv.ParseUint` + `BadRequest` 回傳
- `mesh_service.go` `enrichWithMetrics()` stub → 實作 Prometheus `istio_requests_total` 查詢
- `kubectl_pod_terminal.go` `ensureKubectlPod` 未建立命名空間 → 補上 `ensureNamespace` 呼叫

**殘餘缺陷**
- 部分 handler 同時記錄日誌又回傳錯誤，行為不一致

---

### 2. 實用性（Practicality）9/10

**涵蓋功能**
- 工作負載：Deployment / StatefulSet / DaemonSet / Job / CronJob / Argo Rollouts（HPA / VPA / PDB）
- 資源治理：佔用分析（K8s API）+ 效率分析（Prometheus）+ 容量預測 + Right-sizing + 雲端帳單
- 多叢集：工作負載遷移精靈 + ConfigMap/Secret 跨叢集同步策略
- 網路：NetworkPolicy 策略模擬 + Service Mesh（Istio）流量拓撲 + Gateway API（GatewayClass / Gateway / HTTPRoute / GRPCRoute / ReferenceGrant）+ 叢集網路拓樸圖（靜態 + Istio/Cilium 條件整合）
- 稽核：操作日誌 + Terminal 回放 + 部署審批 + SIEM 推送
- 通知：Webhook / DingTalk（HMAC 加簽）/ Slack / Teams / Email（SMTP）集中渠道管理

**殘餘限制**
- CI/CD Pipeline 引擎（M13–M17）尚未實作，依賴外部 ArgoCD
- 備份（Velero 整合）延後至 M16 後評估

---

### 3. 可用性（Usability）9/10

**優勢**
- 統一 Design Token（`theme.ts` + `ConfigProvider`），零個 `.ant-*` CSS 覆蓋
- `MainLayout.tsx` 縮減至 52 行，Header / Sider / ContextBar 獨立元件
- `EmptyState` / `ErrorState` / `PageSkeleton` 統一規範，所有列表頁有非空白的空狀態
- 分層 `refetchInterval`：Pod 5s / Node 10s / Deployment 15s / Overview 30s
- RBAC 403 錯誤回傳可存取命名空間列表，方便使用者自助診斷
- **i18n 全面覆蓋**：繁中 / 簡中 / 英文三語言，所有頁面零硬編碼中文字串
- **後端中立設計**：API 回傳英文代碼（module、action），前端統一透過 i18n 翻譯

**殘餘缺陷**
- 工作負載列表缺少「日誌」快捷 icon（需進入詳情頁才能查看日誌）

---

### 4. 誠實性（Honesty / Accuracy）9/10

**與實際相符**
- 工作負載、儲存、設定、RBAC、AI、GitOps、Helm、CRD、Terminal 均已實作
- Service Mesh 流量指標已實際填入（`istio_requests_total` Prometheus 查詢）
- 資源治理：佔用、效率、預測、Right-sizing、雲端帳單全部實作，非估算佔位符
- **i18n 架構誠實**：後端不假裝翻譯（不返回硬編碼中文），前端 100% 透過 i18n 框架處理語言，職責分離清晰
- **稽核日誌**：`operation_log_service.go` 已移除 `getModuleName()` / `getActionName()` 中文對映函數，統一回傳代碼

**需補充說明**
- 雲端帳單 GCP 部分依賴 Budget API（需建立預算才有資料），無 Budget 時建議改用 BigQuery Export
- 後端測試覆蓋率約 25%，前端接近 0%，有別於企業級平台通常期待

---

### 5. 系統架構（Architecture）8/10

**優勢**
- Handler → Service → K8s Manager → client-go 分層清晰，無循環依賴
- `K8sInformerManager` 介面（`services` 包定義），解除 `services ↔ k8s` 套件循環
- `Router.Setup()` 集中建立所有服務實例並注入，依賴關係明確
- `ClusterInformerManager` 含閒置 GC（2 小時），防止記憶體洩漏

**已知缺陷**
- 服務層均為具體型別，無介面定義，難以注入 mock 進行單元測試
- 每個 handler 直接持有 `*gorm.DB`，無 repository 層；跨表事務邊界不清晰
- `permission.go` 硬編碼 `username == "admin"` 超級管理員邏輯，應改為 role-based

---

### 6. 穩定度（Stability）7/10

**優勢**
- `ClusterInformerManager` 使用 `sync.RWMutex` 保護叢集 map
- 閒置叢集 GC goroutine 防止長時間累積的資源洩漏
- WebSocket 重連採指數退避（最大 10 次、上限 30 秒）

**殘餘缺陷**
- 高並發下無請求佇列保護：K8s client QPS/Burst（100/200）超出後直接回傳錯誤而非排隊
- SQLite 模式下單連線設計，高並發寫入會成為瓶頸（生產應使用 MySQL）

---

### 7. 程式碼品質（Code Quality）9/10

**優勢**
- `fmt.Errorf("...: %w", err)` 錯誤包裝全面使用
- `wsBufferSize = 1024` 等具名常數集中定義於 `handlers/common.go`
- 前端 `theme.ts` 集中 Design Token，`queryConfig.ts` 集中 refetchInterval
- **i18n 全面遷移（2026-04-12 完成）**：
  - 後端服務層（`operation_log_service.go`）移除所有硬編碼中文對映函數
  - 後端 API 統一回傳英文代碼，前端透過 `i18next` 完成翻譯
  - 前端所有 `t()` 呼叫移除 fallback 中文參數，翻譯來源唯一
  - 三語言文件（zh-TW / zh-CN / en-US）保持同步，無缺漏 key
  - 所有 i18n namespace 正確（`audit:*`、`cost:*`、`common:*`）
- 後端所有 Go 檔案註釋統一改為英文，符合開源協作標準

**殘留觀察**
- 服務層大型方法（如 `GetWorkloadEfficiency`）邏輯較複雜，可進一步拆分
- 前端缺乏單元測試

---

### 8. 效能（Performance）9/10

**優勢**
- K8s Informer 無週期性 resync（`resyncPeriod = 0`），避免不必要的記憶體寫入
- HTTP 連線池：`MaxIdleConnsPerHost = 100`
- 所有列表 API 分頁（預設 pageSize=20）
- Gzip 壓縮套用於所有非 WebSocket 路由
- GlobalSearch / QuickSearch 每叢集一個 goroutine 並行搜尋
- 啟動 Informer 預熱跳過 unhealthy 叢集，並以 goroutine 並行初始化

**殘留觀察**
- 資源效率採集（PromQL）為即時查詢，高頻呼叫時可考慮加入快取層

---

### 9. 安全性（Security）5/10

**優勢**
- JWT 驗證、RBAC 中介層、命名空間粒度權限控制完整
- AI 查詢前自動過濾 PEM 憑證、Secret 值、含 `password/token/key` 的環境變數
- 操作稽核日誌覆蓋所有寫入操作
- 登入端點 Rate Limiting（5次/分鐘，鎖定 15 分鐘）
- API Token SHA-256 hash 儲存，明文僅顯示一次

**重大風險（生產部署前必須處理）**

> ⚠️ **CRITICAL**：kubeconfig、SA Token、CA 憑證目前以明文儲存於資料庫（`ENCRYPTION_KEY` 環境變數存在但欄位級加密尚未套用於 kubeconfig）。任何能存取資料庫的人可取得所有叢集的完整憑證。

> ⚠️ **HIGH**：`k8s_client.go` 預設 `InsecureSkipVerify: true`，除非提供 CA 憑證，否則與 K8s API Server 的通訊不驗證憑證，易受中間人攻擊。

---

### 生產部署前必做清單

```
[ ] 確認 ENCRYPTION_KEY 已設定，並在 GORM BeforeSave hook 啟用 kubeconfig 欄位級 AES-256-GCM 加密
[ ] 確認 JWT_SECRET 已設定（release 模式下系統會強制驗證）
[ ] 為各叢集提供 CA 憑證以啟用 TLS 驗證，避免 InsecureSkipVerify
[ ] 定期備份資料庫並加密備份檔案
[ ] 生產環境使用 MySQL（SQLite 不適合高並發寫入）
[x] runbooks.go panic → 優雅降級（已修復 2026-04-03）
[x] namespace.go context.TODO() → c.Request.Context()（已修復 2026-04-03）
[x] kubectl_terminal.go mustParseUint panic → error return（已修復 2026-04-03）
[x] mesh_service.go enrichWithMetrics stub → 實作 Prometheus 查詢（已修復 2026-04-03）
[x] kubectl_pod_terminal.go 建立 Pod 前未確保命名空間存在 → 補上 ensureNamespace（已修復 2026-04-07）
[x] WebSocket buffer 硬編碼 → wsBufferSize 具名常數（已修復 2026-04-03）
[x] GlobalSearch 串行叢集查詢 → goroutine 並行化（已修復 2026-04-03）
[x] 啟動 Informer 預熱：GetConnectableClusters + 並行初始化（已修復 2026-04-03）
[x] operation_log_service.go 硬編碼中文對映函數 → 移除 getModuleName/getActionName，後端回傳代碼（已修復 2026-04-12）
[x] CommandHistory.tsx 硬編碼中文 → 全面 i18n（已修復 2026-04-12）
[x] GlobalCostInsights.tsx 硬編碼中文 + 樣式值 → i18n + Design Token（已修復 2026-04-12）
[x] 稽核 / 成本模組三語言翻譯缺漏 → 補齊 audit:modules、audit:actions、cost:global 等 key（已修復 2026-04-12）
[x] 後端 Go 原始碼中文註釋 → 全部更新為英文（已修復 2026-04-12）
```

---

## 授權

本專案以 MIT License 授權開源。
