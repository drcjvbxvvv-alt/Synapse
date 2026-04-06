# Synapse Helm Chart 部署指南

## 📁 目錄結構

```
deploy/helm/
└── kubepolaris/               # Chart 主目錄（chart name: synapse）
    ├── Chart.yaml             # Chart 元資料
    ├── values.yaml            # 預設設定值
    ├── values-ha.yaml         # 高可用設定範例
    ├── values-production.yaml # 正式環境設定範例
    ├── README.md              # Chart 使用說明
    ├── quick-deploy.sh        # 快速部署腳本
    ├── .helmignore            # Helm 忽略清單
    └── templates/             # Kubernetes 模板檔案
        ├── NOTES.txt          # 安裝後提示訊息
        ├── _helpers.tpl       # 模板輔助函式
        ├── configmap.yaml     # 設定檔
        ├── secret.yaml        # 密鑰
        ├── serviceaccount.yaml
        ├── rbac.yaml          # RBAC 權限
        ├── mysql-statefulset.yaml
        ├── mysql-service.yaml
        ├── mysql-pvc.yaml
        ├── backend-deployment.yaml
        ├── backend-service.yaml
        ├── frontend-deployment.yaml
        ├── frontend-service.yaml
        ├── ingress.yaml
        ├── hpa.yaml           # 水平自動擴展
        ├── pdb.yaml           # Pod 中斷預算
        └── tests/
            └── test-connection.yaml
```

## 🚀 快速開始

### 方式一：使用快速部署腳本（推薦）

```bash
cd deploy/helm/kubepolaris
./quick-deploy.sh
```

腳本會自動：
- ✅ 檢查環境依賴（kubectl、helm）
- ✅ 產生安全密鑰
- ✅ 建立命名空間
- ✅ 部署所有元件
- ✅ 等待服務就緒

### 方式二：手動 Helm 部署

```bash
# 1. 建立命名空間
kubectl create namespace synapse

# 2. 安裝 Chart
helm install synapse ./deploy/helm/kubepolaris \
  --namespace synapse \
  --set security.jwtSecret="$(openssl rand -base64 32)"

# 3. 查看狀態
helm status synapse -n synapse
kubectl get pods -n synapse
```

### 方式三：使用 Makefile

```bash
make helm-lint       # 驗證 Chart
make helm-install    # 安裝
make helm-package    # 打包
make helm-uninstall  # 卸載
```

## 🎯 部署場景

### 場景 1：基礎部署（開發 / 測試）

使用內建 MySQL，最小資源設定：

```bash
helm install synapse ./deploy/helm/kubepolaris \
  -n synapse \
  --create-namespace \
  --set security.jwtSecret="your-secure-jwt-secret"
```

### 場景 2：正式環境部署（外部資料庫）

```bash
# 建立 Secret
kubectl create secret generic synapse-mysql \
  --from-literal=password=your_mysql_password \
  -n synapse

kubectl create secret generic synapse-secrets \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  -n synapse

# 安裝
helm install synapse ./deploy/helm/kubepolaris \
  -n synapse \
  -f ./deploy/helm/kubepolaris/values-production.yaml \
  --set mysql.external.host=your-mysql-host.example.com
```

### 場景 3：高可用部署

3 副本 + 反親和 + 自動擴縮容：

```bash
helm install synapse ./deploy/helm/kubepolaris \
  -n synapse \
  -f ./deploy/helm/kubepolaris/values-ha.yaml
```

## 📊 設定說明

### 核心設定參數

| 參數 | 描述 | 預設值 |
|------|------|--------|
| `backend.replicaCount` | 後端副本數 | `2` |
| `frontend.replicaCount` | 前端副本數 | `2` |
| `mysql.internal.enabled` | 啟用內建 MySQL | `true` |
| `mysql.external.enabled` | 使用外部 MySQL | `false` |
| `grafana.enabled` | 啟用內建 Grafana | `true` |
| `grafana.dashboards.enabled` | 啟用 Dashboard 自動匯入 | `true` |
| `grafana.datasource.prometheusUrl` | Prometheus 資料來源位址 | `http://your-prometheus:9090` |
| `ingress.enabled` | 啟用 Ingress | `false` |
| `security.jwtSecret` | JWT 密鑰（必填） | `""` |
| `rbac.create` | 建立 RBAC 資源 | `true` |
| `autoscaling.backend.enabled` | 啟用後端 HPA | `false` |
| `podDisruptionBudget.enabled` | 啟用 PDB | `false` |

### Grafana 設定

#### 停用內建 Grafana（使用外部 Grafana）

```bash
helm install synapse ./deploy/helm/kubepolaris \
  -n synapse \
  --set grafana.enabled=false \
  --set grafana.external.enabled=true \
  --set grafana.external.url="http://your-grafana:3000"
```

#### 設定 Prometheus 資料來源

```bash
helm install synapse ./deploy/helm/kubepolaris \
  -n synapse \
  --set grafana.datasource.prometheusUrl="http://prometheus-server:9090"
```

#### 啟用 Grafana 持久化儲存

```bash
helm install synapse ./deploy/helm/kubepolaris \
  -n synapse \
  --set grafana.persistence.enabled=true \
  --set grafana.persistence.size=5Gi \
  --set grafana.persistence.storageClass=your-storage-class
```

#### 完整 Grafana 設定範例

```yaml
# values-custom.yaml
grafana:
  enabled: true
  adminPassword: "your-secure-password"
  persistence:
    enabled: true
    size: 5Gi
    storageClass: "standard"
  dashboards:
    enabled: true
  datasource:
    prometheusUrl: "http://prometheus-kube-prometheus-prometheus:9090"
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 250m
      memory: 256Mi
```

```bash
helm install synapse ./deploy/helm/kubepolaris \
  -n synapse \
  -f values-custom.yaml
```

### 設定檔說明

| 檔案 | 用途 |
|------|------|
| `values.yaml` | 預設設定，適合開發測試 |
| `values-ha.yaml` | 高可用設定，適合正式環境 |
| `values-production.yaml` | 正式環境範本，需依需求調整 |

## 🔧 常用操作

### 查看狀態

```bash
helm status synapse -n synapse
kubectl get pods -n synapse
kubectl get svc -n synapse
kubectl get all -n synapse
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

### 存取應用

```bash
kubectl port-forward -n synapse svc/synapse-frontend 8080:80
# 開啟 http://localhost:8080
# 帳號：admin / Synapse@2026
```

### 升級

```bash
helm upgrade synapse ./deploy/helm/kubepolaris \
  -n synapse \
  -f values.yaml

helm history synapse -n synapse    # 查看升級歷史
helm rollback synapse 1 -n synapse # 回滾
```

### 卸載

```bash
helm uninstall synapse -n synapse

# 刪除 PVC（⚠️ 會刪除所有資料）
kubectl delete pvc -l app.kubernetes.io/instance=synapse -n synapse

kubectl delete namespace synapse
```

## 🧪 測試

```bash
# 執行 Helm 測試
helm test synapse -n synapse

# 手動測試連線
kubectl run test-connection --rm -i --tty \
  --image=busybox:1.36 \
  --restart=Never \
  -n synapse \
  -- wget -qO- synapse-backend:8080/healthz
```

## 📦 打包與發佈

```bash
# 驗證 Chart
helm lint ./deploy/helm/kubepolaris

# 打包
helm package ./deploy/helm/kubepolaris -d dist/

# 產生倉庫索引
helm repo index dist/ --url https://clay-wangzhi.github.io/Synapse

# 渲染模板（驗證用）
helm template synapse ./deploy/helm/kubepolaris \
  --namespace synapse \
  --set security.jwtSecret="test-secret" \
  > rendered.yaml

kubectl apply --dry-run=client -f rendered.yaml
```

## 🔍 故障排查

### Pod 無法啟動

```bash
kubectl describe pod -l app.kubernetes.io/instance=synapse -n synapse
kubectl logs -l app.kubernetes.io/instance=synapse -n synapse --all-containers=true
```

### 資料庫連線失敗

```bash
kubectl get pod -l app.kubernetes.io/component=mysql -n synapse
kubectl get secret synapse-mysql -n synapse -o yaml

kubectl run mysql-client --rm -i --tty \
  --image=mysql:8.0 \
  --restart=Never \
  -n synapse \
  -- mysql -h synapse-mysql -u synapse -p
```

### Ingress 無法存取

```bash
kubectl describe ingress synapse -n synapse
kubectl get pods -n ingress-nginx
```

## 📚 參考文件

- [Helm 官方文件](https://helm.sh/docs/)
- [Kubernetes 官方文件](https://kubernetes.io/docs/)
- [Chart README](./kubepolaris/README.md)

---

**維護者**：Synapse Team
**更新時間**：2026-04-06
