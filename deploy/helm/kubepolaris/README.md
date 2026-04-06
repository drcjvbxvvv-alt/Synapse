# Synapse Helm Chart

[![Version](https://img.shields.io/badge/version-1.0.5-blue)](https://github.com/clay-wangzhi/Synapse)
[![Type](https://img.shields.io/badge/type-application-informational)](https://helm.sh/docs/topics/charts/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](https://github.com/clay-wangzhi/Synapse/blob/main/LICENSE)

企業級 Kubernetes 多叢集管理平台 Helm Chart

## 📖 簡介

Synapse 是一個現代化的 Kubernetes 叢集管理平台，提供直觀的 Web 介面來統一管理和監控多個 Kubernetes 叢集。

**主要功能：**

- ✅ 多叢集統一管理
- ✅ 工作負載管理（Deployment / StatefulSet / DaemonSet / Job / CronJob）
- ✅ Pod 管理、日誌查看、終端機
- ✅ 節點管理與操作
- ✅ Web 終端（Pod / Kubectl / SSH）
- ✅ Prometheus / Grafana 整合
- ✅ RBAC 權限控制
- ✅ 操作稽核日誌
- ✅ K8s 事件告警（Webhook / DingTalk / Slack / Teams）
- ✅ 安全設定（Session 逾時 / 登入鎖定 / 密碼政策）
- ✅ API Token 管理
- ✅ 通知渠道集中管理
- ✅ 跨叢集工作負載視圖
- ✅ Helm 倉庫管理

## 🚀 快速開始

### 前置需求

- Kubernetes 1.20+
- Helm 3.0+
- PV Provisioner（啟用持久化儲存時需要）

### 新增 Helm 倉庫

```bash
helm repo add synapse https://clay-wangzhi.github.io/Synapse
helm repo update
```

### 安裝 Chart

```bash
# 基礎安裝
helm install synapse synapse/synapse \
  --namespace synapse \
  --create-namespace

# 查看安裝狀態
helm status synapse -n synapse
```

### 存取應用

```bash
# 使用 port-forward 存取
kubectl port-forward -n synapse svc/synapse-frontend 8080:80

# 開啟瀏覽器
# http://localhost:8080

# 預設登入資訊
# 帳號：admin
# 密碼：Synapse@2026
```

## 📋 設定

### values.yaml 關鍵設定

| 參數 | 描述 | 預設值 |
|------|------|--------|
| `backend.replicaCount` | 後端副本數 | `2` |
| `frontend.replicaCount` | 前端副本數 | `2` |
| `mysql.internal.enabled` | 啟用內建 MySQL | `true` |
| `mysql.external.enabled` | 使用外部 MySQL | `false` |
| `ingress.enabled` | 啟用 Ingress | `false` |
| `security.jwtSecret` | JWT 密鑰（留空自動產生） | `""` |
| `rbac.create` | 建立 RBAC 資源 | `true` |

查看 [values.yaml](./values.yaml) 取得完整設定清單。

## 🎯 部署場景

### 場景 1：基礎部署（內建 MySQL）

```bash
helm install synapse synapse/synapse \
  -n synapse \
  --set security.jwtSecret="your-secure-jwt-secret-at-least-32-chars"
```

### 場景 2：使用外部資料庫

```bash
# 建立資料庫 Secret
kubectl create secret generic synapse-mysql \
  --from-literal=password=your_mysql_password \
  -n synapse

# 安裝
helm install synapse synapse/synapse \
  -n synapse \
  --set mysql.internal.enabled=false \
  --set mysql.external.enabled=true \
  --set mysql.external.host=mysql.example.com \
  --set mysql.external.database=synapse \
  --set mysql.external.username=synapse \
  --set mysql.external.existingSecret=synapse-mysql \
  --set security.jwtSecret="your-secure-jwt-secret"
```

### 場景 3：啟用 Ingress + TLS

建立 `values-ingress.yaml`：

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: synapse.example.com
      paths:
        - path: /
          pathType: Prefix
          backend: frontend
        - path: /api
          pathType: Prefix
          backend: backend
  tls:
    - secretName: synapse-tls
      hosts:
        - synapse.example.com

security:
  jwtSecret: "your-secure-jwt-secret-at-least-32-chars"
```

```bash
helm install synapse synapse/synapse \
  -n synapse \
  -f values-ingress.yaml
```

### 場景 4：高可用部署

```bash
helm install synapse synapse/synapse \
  -n synapse \
  -f values-ha.yaml
```

查看 [values-ha.yaml](./values-ha.yaml) 自訂設定。

### 場景 5：正式環境部署

```bash
helm install synapse synapse/synapse \
  -n synapse \
  -f values-production.yaml \
  --set mysql.external.host=your-mysql-host \
  --set security.jwtSecret="$(openssl rand -base64 32)"
```

## 🔧 升級與維護

### 升級 Chart

```bash
helm repo update

# 查看可用版本
helm search repo synapse --versions

# 升級到最新版本
helm upgrade synapse synapse/synapse \
  -n synapse \
  -f values.yaml

# 查看升級歷史
helm history synapse -n synapse
```

### 回滾

```bash
helm history synapse -n synapse
helm rollback synapse 1 -n synapse
```

## 🗑️ 卸載

```bash
helm uninstall synapse -n synapse

# 刪除 PVC（⚠️ 會刪除所有資料）
kubectl delete pvc -l app.kubernetes.io/instance=synapse -n synapse

kubectl delete namespace synapse
```

## 🔍 故障排查

### 查看 Pod 狀態

```bash
kubectl get pods -n synapse
kubectl describe pod -l app.kubernetes.io/instance=synapse -n synapse
```

### 查看日誌

```bash
# 後端日誌
kubectl logs -f -l app.kubernetes.io/component=backend -n synapse

# 前端日誌
kubectl logs -f -l app.kubernetes.io/component=frontend -n synapse

# MySQL 日誌
kubectl logs -f -l app.kubernetes.io/component=mysql -n synapse
```

### 查看事件

```bash
kubectl get events -n synapse --sort-by='.lastTimestamp'
```

### 常見問題

#### Pod 一直 Pending

```bash
# 確認 PVC 狀態
kubectl get pvc -n synapse

# 確認節點資源
kubectl describe nodes
```

#### 資料庫連線失敗

```bash
kubectl get pod -l app.kubernetes.io/component=mysql -n synapse
kubectl get secret -n synapse
kubectl describe secret synapse-mysql -n synapse
```

#### 後端無法啟動

```bash
kubectl logs -l app.kubernetes.io/component=backend -n synapse --tail=100

# 確認 ConfigMap
kubectl describe configmap synapse-config -n synapse

# 確認環境變數
kubectl exec -it deployment/synapse-backend -n synapse -- env | grep -E "DB|JWT"
```

## 🧪 測試

```bash
helm test synapse -n synapse
```

## 📊 監控

### 啟用 Prometheus ServiceMonitor

```yaml
monitoring:
  serviceMonitor:
    enabled: true
    interval: 30s
```

### 外部 Grafana 整合

```yaml
grafana:
  external:
    enabled: true
    url: "https://grafana.example.com"
    apiKey: "your-api-key"
```

## 🔒 密鑰管理

### 使用既有 Secret（推薦）

```bash
# 建立 Secret
kubectl create secret generic synapse-secrets \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  -n synapse

# 使用 existing Secret 安裝
helm install synapse synapse/synapse \
  -n synapse \
  --set security.existingSecret=synapse-secrets
```

建議搭配 [Vault](https://www.vaultproject.io/) 或 [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) 進行密鑰管理。

### RBAC 權限

Chart 會自動建立必要的 RBAC 資源。可透過 `rbac.rules` 自訂權限。

## 📚 文件

- [官方文件](https://synapse.clay-wangzhi.com/docs)
- [快速開始](https://synapse.clay-wangzhi.com/docs/getting-started/quick-start)
- [設定指南](https://synapse.clay-wangzhi.com/docs/admin-guide/deployment)
- [API 文件](https://synapse.clay-wangzhi.com/docs/api/overview)

## 🤝 貢獻

歡迎貢獻！請查看 [CONTRIBUTING.md](https://github.com/clay-wangzhi/Synapse/blob/main/CONTRIBUTING.md)

## 📄 授權條款

Apache License 2.0 — 查看 [LICENSE](https://github.com/clay-wangzhi/Synapse/blob/main/LICENSE)

---

**維護者**：Synapse Team
**聯絡方式**：support@synapse.io
**首頁**：https://synapse.clay-wangzhi.com
**更新時間**：2026-04-06
