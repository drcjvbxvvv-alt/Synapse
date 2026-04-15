# Synapse 第三方軟體服務清單

> 版本：v1.0 | 更新日期：2026-04-15
> 本文件列出 Synapse 平台依賴的所有外部服務與基礎設施。

---

## 目錄

1. [必要服務](#1-必要服務)
2. [基礎設施](#2-基礎設施)
3. [監控與可觀測性](#3-監控與可觀測性)
4. [日誌聚合](#4-日誌聚合)
5. [GitOps 與 CI/CD](#5-gitops-與-cicd)
6. [容器 Registry](#6-容器-registry)
7. [AI / LLM Provider](#7-ai--llm-provider)
8. [通知管道](#8-通知管道)
9. [認證與身份](#9-認證與身份)
10. [金鑰管理（KMS）](#10-金鑰管理kms)
11. [雲端成本](#11-雲端成本)
12. [安全掃描](#12-安全掃描)
13. [稽核整合](#13-稽核整合)
14. [開發工具](#14-開發工具)
15. [統計摘要](#15-統計摘要)

---

## 1. 必要服務

這三個服務是 Synapse 正常運作的硬性依賴，缺少任何一個平台將無法啟動。

| 服務 | 版本要求 | 用途 | 設定環境變數 | 預設 Port |
|------|----------|------|-------------|-----------|
| **PostgreSQL** | v14+ | 主資料庫：使用者帳號、叢集設定、Pipeline 定義、加密憑證、稽核日誌 | `DB_HOST` `DB_PORT` `DB_USERNAME` `DB_PASSWORD` `DB_DATABASE` `DB_SSL_MODE` | 5432 |
| **Kubernetes** | v1.24+ | 被管理的目標叢集（核心功能） | kubeconfig 或 in-cluster Service Account | 443 / 6443 |
| **K8s Service Account** | — | Synapse 自身部署於 K8s 時的 API 認證 | `/var/run/secrets/kubernetes.io/serviceaccount` | — |

---

## 2. 基礎設施

| 服務 | 必要性 | 用途 | 設定環境變數 | 預設 Port |
|------|--------|------|-------------|-----------|
| **Redis** | 選用 | 多副本部署時的分散式 Rate Limiting；單副本模式使用 in-memory 替代 | `REDIS_ADDR` `REDIS_PASSWORD` `REDIS_DB` | 6379 |

---

## 3. 監控與可觀測性

| 服務 | 必要性 | 用途 | 設定位置 | 預設 Port |
|------|--------|------|----------|-----------|
| **Prometheus** | 選用 | 叢集指標查詢（CPU / Memory / Network / 自訂 PromQL） | 系統設定 → 監控來源 | 9090 |
| **Alertmanager** | 選用 | 告警聚合、路由、靜音管理 | 系統設定 → AlertManager | 9093 |
| **Grafana** | 選用 | 儀表板視覺化；Synapse 可自動同步 Prometheus 資料源 | 系統設定 → Grafana（URL + API Key） | 3000 |
| **Jaeger** | 選用 | 分散式 Tracing（OpenTelemetry gRPC OTLP） | `OTEL_EXPORTER_OTLP_ENDPOINT` `OTEL_SERVICE_NAME` `OTEL_SAMPLING_RATE` | 4317 |
| **Grafana Tempo** | 選用 | 分散式 Tracing 替代後端（同 OTLP 設定） | 同 Jaeger | 4317 |

> **啟用 Tracing**：需設定功能旗標 `enable_otel_tracing=true`

---

## 4. 日誌聚合

| 服務 | 必要性 | 用途 | 設定 | 預設 Port |
|------|--------|------|------|-----------|
| **Elasticsearch** | 選用 | 容器日誌與稽核日誌儲存 / 查詢 | `type=elasticsearch` + URL / username / password / API Key | 9200 |
| **Loki** | 選用 | 容器日誌聚合（Grafana 生態，支援 LogQL） | `type=loki` + URL / X-Scope-OrgID / username | 3100 |

---

## 5. GitOps 與 CI/CD

### GitOps 引擎

| 服務 | 必要性 | 用途 | 設定 |
|------|--------|------|------|
| **ArgoCD** | 選用 | GitOps 應用部署管理；支援 token 或 username/password 認證 | 系統設定 → ArgoCD（ServerURL + 認證） |
| **Argo Rollouts** | 選用 | 漸進式交付 CRD（Canary / BlueGreen 策略）；Synapse 自動偵測 CRD 是否安裝 | 無需設定，Discovery API 自動偵測 |

### Git Provider

| 服務 | 必要性 | 用途 | 設定 |
|------|--------|------|------|
| **GitHub** | 選用 | Pipeline Webhook 觸發、Git 操作（支援 GitHub.com 與 GitHub Enterprise） | 系統設定 → Git Provider（type=github + BaseURL + AccessToken + WebhookSecret） |
| **GitLab** | 選用 | 同上（支援 GitLab.com 與自架） | 系統設定 → Git Provider（type=gitlab） |
| **Gitea** | 選用 | 同上（自架 Git 伺服器） | 系統設定 → Git Provider（type=gitea） |

---

## 6. 容器 Registry

| 服務 | 類型 | 用途 |
|------|------|------|
| **Harbor** | 自架 | 企業級 Registry，支援專案管理與 CA Bundle | type=harbor |
| **Docker Hub** | 公有雲 | 公開 / 私有 Docker 映像 | type=dockerhub |
| **AWS ECR** | 公有雲 | Amazon Elastic Container Registry | type=ecr |
| **GCR / GAR** | 公有雲 | Google Container Registry / Artifact Registry | type=gcr |
| **Alibaba ACR** | 公有雲 | 阿里雲容器映像服務 | type=acr |

> 所有 Registry 的 `password` / `ca_bundle` 欄位皆以 AES-256-GCM 加密儲存。

---

## 7. AI / LLM Provider

用於 AI 輔助根因分析（RCA）、日誌分析、安全建議等功能。

| Provider | 必要性 | 模型預設值 | 設定 |
|----------|--------|-----------|------|
| **OpenAI** | 選用 | gpt-4o | `provider=openai` + endpoint + api_key |
| **Anthropic Claude** | 選用 | claude-3-5-sonnet-20241022 | `provider=anthropic` + api_key |
| **Azure OpenAI** | 選用 | 依部署名稱 | `provider=azure` + endpoint + api_version + api_key |
| **Ollama** | 選用 | llama3 | `provider=ollama` + `endpoint=http://localhost:11434` |

> 同一時間只需啟用一個 Provider；未設定 AI 時相關功能自動降級。

---

## 8. 通知管道

Pipeline 事件通知與 Alertmanager 告警路由使用。

| 服務 | 必要性 | 協定 / 設定 | 用途 |
|------|--------|------------|------|
| **Slack** | 選用 | Incoming Webhook URL | Pipeline 完成 / 失敗 / 告警通知 |
| **Telegram** | 選用 | Bot Token + Chat ID | 同上 |
| **Microsoft Teams** | 選用 | Connector Webhook URL | 同上 |
| **Email（SMTP）** | 選用 | smarthost + auth（TLS Port 587 / 465） | Alertmanager 告警信件 |
| **PagerDuty** | 選用 | Service Key | 告警路由至 PagerDuty 事件管理 |
| **Generic Webhook** | 選用 | 任意 HTTPS Endpoint | 自訂 SIEM / 自動化系統整合 |

---

## 9. 認證與身份

| 服務 | 必要性 | 用途 | 設定 |
|------|--------|------|------|
| **LDAP / Active Directory** | 選用 | 使用者 SSO 認證，支援群組對應 | `LDAPConfig`：server / port / BaseDN / BindDN / UserFilter / GroupFilter / TLS |

> 未設定 LDAP 時使用本機帳號認證（預設模式）。

---

## 10. 金鑰管理（KMS）

Synapse 所有加密欄位（kubeconfig、token、密碼）皆使用 AES-256-GCM，主金鑰透過 KeyProvider 管理。

| Provider | 必要性 | 用途 | 設定 |
|----------|--------|------|------|
| **Environment Variable** | 預設 | 主金鑰存於環境變數（開發 / 單機部署） | `ENCRYPTION_KEY`（32 bytes hex） |
| **HashiCorp Vault** | 選用 | 生產環境集中式金鑰管理 | `type=vault` + vault_addr / vault_token / vault_secret_path |
| **AWS Secrets Manager** | 選用 | 雲端 KMS 替代方案 | `type=aws_secretsmanager` + aws_region / aws_secret_name |

---

## 11. 雲端成本

| 服務 | 必要性 | 用途 | 設定 |
|------|--------|------|------|
| **AWS Cost Explorer** | 選用 | AWS 叢集成本分析與報告 | `provider=aws` + aws_access_key_id / aws_secret_access_key / aws_region |
| **GCP Billing API** | 選用 | GCP 叢集成本分析 | `provider=gcp` + gcp_project_id / gcp_billing_account_id / gcp_service_account_json |

---

## 12. 安全掃描

| 服務 | 必要性 | 用途 | 執行模式 |
|------|--------|------|----------|
| **Trivy** | 選用 | 容器映像 CVE 掃描 | host binary 或 K8s Job（可選） |

---

## 13. 稽核整合

| 服務 | 必要性 | 用途 | 設定 |
|------|--------|------|------|
| **Generic SIEM Webhook** | 選用 | 將 Synapse 稽核日誌匯出至外部 SIEM 系統 | webhookURL + secretHeader + secretValue |

---

## 14. 開發工具

僅在開發環境使用，不出現於生產部署。

| 服務 | 用途 | 來源 |
|------|------|------|
| **Adminer** | PostgreSQL Web 管理介面 | `docker-compose.dev.yml`，Port 8080 |
| **Air** | Go 後端熱重載 | `scripts/dev.sh` 自動偵測，需 `go install github.com/air-verse/air@latest` |

---

## 15. 統計摘要

| 類別 | 數量 |
|------|------|
| **必要服務** | 3 |
| **選用服務** | 32+ |
| AI Provider | 4 |
| Container Registry | 5 |
| Git Provider | 3 |
| 通知管道 | 6 |
| 日誌後端 | 2 |
| 可觀測性後端 | 3（Prometheus / Jaeger / Tempo） |
| KMS 方案 | 3（env-var / Vault / AWS SM） |
| 雲端成本 | 2 |

### 最小部署需求

生產環境最小可運作的配置只需：

```
PostgreSQL + Kubernetes（含 Service Account）
```

其餘 32+ 個服務依使用情境按需啟用，不影響平台核心功能。
