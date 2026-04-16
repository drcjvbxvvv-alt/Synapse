# Synapse CICD — Java 應用使用指南

> 版本：v1.0 | 更新日期：2026-04-16
> 適用對象：Java SaaS 應用的 DevOps 工程師

---

## 目錄

1. [前置需求](#1-前置需求)
2. [Dockerfile](#2-dockerfile)
3. [Synapse 原生 CI（推薦）](#3-synapse-原生-ci推薦)
4. [GitLab CI 設定（外部引擎）](#4-gitlab-ci-設定外部引擎)
5. [Kubernetes 部署規格](#5-kubernetes-部署規格)
6. [Harbor 映像倉庫](#6-harbor-映像倉庫)
7. [GitOps（ArgoCD）](#7-gitopsargocd)
8. [故障排查](#8-故障排查)

---

## 1. 前置需求

### 工具版本

| 工具 | 版本要求 |
|------|----------|
| JDK  | 11 或 17 |
| Maven | 3.8+ |
| Docker | 24+ |
| GitLab Runner | 16.x+ |

### GitLab CI 變數（Settings → CI/CD → Variables）

| 變數名稱 | 說明 | Masked |
|----------|------|--------|
| `HARBOR_REGISTRY` | Harbor 地址（如 `harbor.local`） | 否 |
| `HARBOR_USERNAME` | Robot Account 名稱 | 否 |
| `HARBOR_PASSWORD` | Robot Account Token | **是** |

---

## 2. Dockerfile

### 多階段構建（Maven → OpenJDK-slim）

```dockerfile
# deploy/docker/saas-java-a/Dockerfile

# Stage 1: Build
FROM maven:3.9-eclipse-temurin-17 AS builder
WORKDIR /app

# 先複製 pom.xml 利用 Layer Cache
COPY pom.xml .
RUN mvn dependency:go-offline -q

COPY src ./src
RUN mvn package -DskipTests -q

# Stage 2: Runtime（最小化映像）
FROM eclipse-temurin:17-jre-alpine
WORKDIR /app

# 建立非 root 使用者
RUN addgroup -S app && adduser -S app -G app
USER app

COPY --from=builder /app/target/*.jar app.jar

EXPOSE 8080

ENTRYPOINT ["java", \
  "-XX:+UseContainerSupport", \
  "-XX:MaxRAMPercentage=75.0", \
  "-jar", "app.jar"]
```

### JVM 容器記憶體最佳化說明

| JVM 旗標 | 說明 |
|----------|------|
| `-XX:+UseContainerSupport` | 讀取 cgroup 記憶體限制（預設開啟於 JDK 11+）|
| `-XX:MaxRAMPercentage=75.0` | 使用容器可用記憶體的 75% 作為 Heap 上限 |

---

## 3. Synapse 原生 CI（推薦）

Synapse 內建 CI 引擎，不需要外部 GitLab Runner 或 Jenkins。
Pipeline 以 YAML 定義，由 Synapse 排程並以 **Kubernetes Job** 執行每個步驟。

### 3.1 與外部引擎的差異

| | Synapse 原生 CI | GitLab CI / Jenkins |
|--|-----------------|---------------------|
| 需要額外元件 | 否（內建） | 需要 Runner / Agent |
| Pipeline 存放 | Synapse DB（版本快照） | `.gitlab-ci.yml` / Jenkinsfile |
| 執行環境 | K8s Job（Kaniko 無需 Docker Daemon） | Docker-in-Docker / Agent |
| 適合場景 | 新專案、無現有 CI 基礎設施 | 已有 GitLab / Jenkins 的組織 |

### 3.2 Pipeline YAML 格式

```yaml
apiVersion: synapse.io/v1
kind: Pipeline
metadata:
  name: saas-java-a-build         # Pipeline 名稱，DNS 相容
spec:
  description: Build and deploy SaaS Java A

  # 並發控制：同分支只跑一個，舊的自動取消
  concurrency:
    group: build-${BRANCH}
    policy: cancel_previous         # cancel_previous | queue | reject

  # 所有步驟共用的環境變數
  env:
    HARBOR_REGISTRY: "harbor.local"
    HARBOR_PROJECT: "saas"
    APP_NAME: "java-a"
    NAMESPACE: "saas-java-a"

  # 觸發條件
  triggers:
    - type: webhook
      provider: gitlab              # gitlab | github
      repo: your-org/saas-java-a
      branch: main
      events:
        - push
      path_filter:                  # 只有這些路徑變更才觸發
        - "src/**"
        - "pom.xml"
        - "Dockerfile"

    - type: schedule
      cron: "0 2 * * *"            # 每日凌晨 2 點夜間構建

  # 共用工作目錄（步驟間共享原始碼與產出物）
  workspace:
    type: pvc
    size: "4Gi"
    retention_hours: 24

  steps:
    # ── 步驟 1：Maven 編譯打包 ────────────────────────────
    - name: build-jar
      type: build-jar
      image: maven:3.9-eclipse-temurin-17
      timeout: "20m"
      config:
        build_tool: maven
        goals: clean package -DskipTests
        java_version: "17"
      env:
        MAVEN_OPTS: "-XX:+TieredCompilation -XX:TieredStopAtLevel=1"

    # ── 步驟 2：Kaniko 構建映像（無需 Docker Daemon）─────
    - name: build-image
      type: build-image
      depends_on:
        - build-jar
      timeout: "20m"
      config:
        context: .
        dockerfile: deploy/docker/saas-java-a/Dockerfile
        destination: "${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${APP_NAME}:${CI_COMMIT_SHA}"
        cache: true
        cache_repo: "${HARBOR_REGISTRY}/cache"

    # ── 步驟 3：Trivy 漏洞掃描 ───────────────────────────
    - name: scan-image
      type: trivy-scan
      depends_on:
        - build-image
      config:
        image: "${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${APP_NAME}:${CI_COMMIT_SHA}"
        severity: "HIGH,CRITICAL"
        ignore_unfixed: true
      on_failure: abort             # 有 CRITICAL 漏洞就停止

    # ── 步驟 4：推送到 Harbor ────────────────────────────
    - name: push-image
      type: push-image
      depends_on:
        - scan-image
      config:
        source: "${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${APP_NAME}:${CI_COMMIT_SHA}"
        tags:
          - "${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${APP_NAME}:latest"
      env:
        REGISTRY_USERNAME: ${{ secrets.HARBOR_USERNAME }}
        REGISTRY_PASSWORD: ${{ secrets.HARBOR_PASSWORD }}

    # ── 步驟 5：更新 GitOps YAML（image tag）────────────
    - name: gitops-sync
      type: gitops-sync
      depends_on:
        - push-image
      config:
        repo: "https://gitlab.local/your-org/Synapse.git"
        branch: main
        files:
          - path: deploy/k8s/saas-java-a-deployment.yaml
            replacements:
              - pattern: "image: harbor.local/saas/java-a:.*"
                value: "image: harbor.local/saas/java-a:${CI_COMMIT_SHA}"
        commit_message: "ci: update java-a to ${CI_COMMIT_SHA} [skip ci]"
      env:
        GIT_USERNAME: ${{ secrets.GIT_USERNAME }}
        GIT_PASSWORD: ${{ secrets.GIT_PASSWORD }}

    # ── 步驟 6：部署後 Smoke Test ────────────────────────
    - name: smoke-test
      type: smoke-test
      depends_on:
        - gitops-sync
      timeout: "5m"
      config:
        url: "https://saas-java-a.example.com/actuator/health"
        method: GET
        expected_status: 200
        retries: 10
        retry_interval: 15          # 等待 ArgoCD 同步完成

  # 通知（Slack / Teams / Webhook）
  notifications:
    on_success:
      channels: [1]
    on_failure:
      channels: [1, 2]
    on_scan_critical:
      channels: [3]
```

### 3.3 步驟類型速查

| 步驟類型 | 用途 | 預設映像 |
|----------|------|----------|
| `build-jar` | Maven / Gradle 打包 | `maven:3.9-eclipse-temurin-17` |
| `build-image` | 構建容器映像（Kaniko） | `gcr.io/kaniko-project/executor:v1.23` |
| `trivy-scan` | 漏洞掃描 | `aquasec/trivy:latest` |
| `push-image` | 推送 / Retag 映像 | `gcr.io/go-containerregistry/crane` |
| `gitops-sync` | Git commit + push 更新 YAML | `alpine/git` |
| `deploy` | `kubectl apply` 直接部署 | `bitnami/kubectl` |
| `smoke-test` | HTTP 健康檢查 | `curlimages/curl` |
| `approval` | 人工審核閘道 | — |
| `run-script` | 自訂 shell 腳本 | 自行指定 |
| `notify` | 發送通知（Slack / Teams） | — |

### 3.4 Secret 引用

Pipeline YAML 中使用 `${{ secrets.NAME }}` 引用 Secrets（在 Synapse → Pipeline → Secrets 管理）：

```yaml
env:
  HARBOR_PASSWORD: ${{ secrets.HARBOR_PASSWORD }}
  GIT_TOKEN:       ${{ secrets.GITLAB_CI_TOKEN }}
```

### 3.5 在 Synapse 建立 Pipeline

1. Synapse → Pipeline → 新增
2. 選擇引擎類型：**原生（Native）**
3. 貼上上方 YAML → 儲存
4. 進入 Pipeline → Secrets → 新增 `HARBOR_USERNAME`、`HARBOR_PASSWORD`、`GIT_USERNAME`、`GIT_PASSWORD`
5. 手動觸發第一次執行驗證，或等待 Webhook 觸發

---

## 4. GitLab CI 設定（外部引擎）

### 完整 `.gitlab-ci.yml`

```yaml
stages:
  - lint
  - test
  - build
  - docker-build
  - scan
  - push
  - deploy

variables:
  HARBOR_REGISTRY: "harbor.local"
  HARBOR_PROJECT: "saas"
  APP_NAME: "java-a"          # 每個應用修改這裡
  JAVA_VERSION: "17"
  MAVEN_OPTS: "-Dmaven.repo.local=$CI_PROJECT_DIR/.m2/repository"

# Maven 快取（加速 dependency 下載）
.maven-cache: &maven-cache
  cache:
    key: "$CI_PROJECT_NAME-maven"
    paths:
      - .m2/repository

# ── 1. 靜態分析 ─────────────────────────────────────────────
lint:java:
  stage: lint
  image: maven:3.9-eclipse-temurin-17
  <<: *maven-cache
  script:
    - mvn validate checkstyle:check -q
  only:
    - merge_requests
    - main

# ── 2. 單元測試 ─────────────────────────────────────────────
test:java:
  stage: test
  image: maven:3.9-eclipse-temurin-17
  <<: *maven-cache
  script:
    - mvn test -q
  coverage: '/Total.*?([0-9]{1,3})%/'
  artifacts:
    when: always
    reports:
      junit: target/surefire-reports/TEST-*.xml
    paths:
      - target/site/jacoco/

# ── 3. 打包 JAR ─────────────────────────────────────────────
build:jar:
  stage: build
  image: maven:3.9-eclipse-temurin-17
  <<: *maven-cache
  script:
    - mvn package -DskipTests -q
  artifacts:
    paths:
      - target/*.jar
    expire_in: 1 hour

# ── 4. Docker Build ─────────────────────────────────────────
docker:build:
  stage: docker-build
  image: docker:24
  services:
    - docker:24-dind
  dependencies:
    - build:jar
  variables:
    IMAGE: "$HARBOR_REGISTRY/$HARBOR_PROJECT/$APP_NAME:$CI_COMMIT_SHORT_SHA"
  script:
    - docker build
        -f deploy/docker/saas-$APP_NAME/Dockerfile
        -t $IMAGE
        .
  only:
    - main
    - tags

# ── 5. Trivy 安全掃描 ────────────────────────────────────────
scan:trivy:
  stage: scan
  image:
    name: aquasec/trivy:latest
    entrypoint: [""]
  variables:
    IMAGE: "$HARBOR_REGISTRY/$HARBOR_PROJECT/$APP_NAME:$CI_COMMIT_SHORT_SHA"
  script:
    - trivy image
        --exit-code 1
        --severity HIGH,CRITICAL
        --no-progress
        --ignore-unfixed
        $IMAGE
  allow_failure: false      # CRITICAL 漏洞阻擋 Pipeline
  only:
    - main
    - tags

# ── 6. 推送到 Harbor ─────────────────────────────────────────
push:harbor:
  stage: push
  image: docker:24
  services:
    - docker:24-dind
  variables:
    IMAGE: "$HARBOR_REGISTRY/$HARBOR_PROJECT/$APP_NAME:$CI_COMMIT_SHORT_SHA"
  before_script:
    - docker login -u $HARBOR_USERNAME -p $HARBOR_PASSWORD $HARBOR_REGISTRY
  script:
    - docker push $IMAGE
    - docker tag  $IMAGE $HARBOR_REGISTRY/$HARBOR_PROJECT/$APP_NAME:latest
    - docker push       $HARBOR_REGISTRY/$HARBOR_PROJECT/$APP_NAME:latest
  only:
    - main
    - tags

# ── 7. 更新 GitOps YAML ──────────────────────────────────────
deploy:update-gitops:
  stage: deploy
  image: alpine/git:latest
  variables:
    K8S_YAML: "deploy/k8s/saas-$APP_NAME-deployment.yaml"
    IMAGE: "$HARBOR_REGISTRY/$HARBOR_PROJECT/$APP_NAME"
  script:
    - sed -i
        "s|image: $IMAGE:.*|image: $IMAGE:$CI_COMMIT_SHORT_SHA|"
        $K8S_YAML
    - git config user.email "ci@synapse.local"
    - git config user.name  "GitLab CI"
    - git add $K8S_YAML
    - git diff --staged --quiet ||
        git commit -m "ci: update $APP_NAME to $CI_COMMIT_SHORT_SHA [skip ci]"
    - git push
        "https://gitlab-ci-token:${CI_JOB_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git"
        HEAD:main
  only:
    - main
    - tags
```

---

## 5. Kubernetes 部署規格

```yaml
# deploy/k8s/saas-java-a-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: saas-java-a
  namespace: saas-java-a
spec:
  replicas: 2
  selector:
    matchLabels:
      app: saas-java-a
  template:
    metadata:
      labels:
        app: saas-java-a
    spec:
      imagePullSecrets:
        - name: harbor-regcred
      containers:
        - name: app
          image: harbor.local/saas/java-a:latest   # CI 自動更新此行
          ports:
            - containerPort: 8080
          resources:
            requests:
              memory: "512Mi"
              cpu: "250m"
            limits:
              memory: "1Gi"
              cpu: "1000m"
          livenessProbe:
            httpGet:
              path: /actuator/health/liveness
              port: 8080
            initialDelaySeconds: 60
            periodSeconds: 15
          readinessProbe:
            httpGet:
              path: /actuator/health/readiness
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          env:
            - name: SPRING_PROFILES_ACTIVE
              value: "prod"
            - name: JAVA_TOOL_OPTIONS
              value: "-XX:+UseContainerSupport -XX:MaxRAMPercentage=75.0"
      # 反親和性：分散到不同節點
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: saas-java-a
                topologyKey: kubernetes.io/hostname
---
# HPA 自動擴縮
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: saas-java-a
  namespace: saas-java-a
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: saas-java-a
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

### Spring Boot Actuator 設定（必要）

```yaml
# src/main/resources/application-prod.yaml
management:
  endpoints:
    web:
      exposure:
        include: health, prometheus
  endpoint:
    health:
      probes:
        enabled: true          # 啟用 liveness / readiness 端點
      show-details: never
  health:
    livenessState:
      enabled: true
    readinessState:
      enabled: true
```

---

## 6. Harbor 映像倉庫

### 建立 imagePullSecret

```bash
# 為每個 Namespace 建立拉取憑證
for NS in saas-java-a saas-java-b; do
  kubectl create namespace $NS --dry-run=client -o yaml | kubectl apply -f -
  kubectl create secret docker-registry harbor-regcred \
    --docker-server=harbor.local \
    --docker-username=ci-robot \
    --docker-password=<ROBOT_TOKEN> \
    -n $NS
done
```

### 映像命名規則

| 環境 | Tag 格式 | 範例 |
|------|----------|------|
| CI 自動部署 | `${CI_COMMIT_SHORT_SHA}` | `harbor.local/saas/java-a:a1b2c3d4` |
| 正式 Release | `v{major}.{minor}.{patch}` | `harbor.local/saas/java-a:v1.2.3` |
| 最新版本 | `latest` | `harbor.local/saas/java-a:latest` |

---

## 7. GitOps（ArgoCD）

```yaml
# deploy/examples/argocd-java-a-application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: saas-java-a
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://gitlab.local/your-org/Synapse.git
    targetRevision: main
    path: deploy/k8s
    directory:
      include: "saas-java-a-*.yaml"
  destination:
    server: https://kubernetes.default.svc
    namespace: saas-java-a
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

```bash
# 套用 Application
kubectl apply -f deploy/examples/argocd-java-a-application.yaml

# 手動同步
argocd app sync saas-java-a

# 查看狀態
argocd app get saas-java-a
```

---

## 8. 故障排查

### Pod 啟動失敗（ImagePullBackOff）

```bash
kubectl describe pod -n saas-java-a <pod-name>
# → 確認 harbor-regcred secret 存在且 token 未過期

kubectl get secret harbor-regcred -n saas-java-a
```

### Pod CrashLoopBackOff（JVM OOM）

```bash
# 查看 Log
kubectl logs -n saas-java-a <pod-name> --previous | grep -E "OutOfMemory|OOM"

# 調整記憶體限制或 MaxRAMPercentage
# limits.memory: "1Gi" → "2Gi"
```

### Trivy 掃描阻擋 Pipeline

```bash
# 本地確認漏洞詳情
trivy image --severity HIGH,CRITICAL harbor.local/saas/java-a:latest

# 常見修復方式
# 1. 升級基礎映像：FROM eclipse-temurin:17-jre-alpine → :21-jre-alpine
# 2. 升級依賴版本（pom.xml）
# 3. 對確認不影響的 CVE 加入 .trivyignore
```

### Pipeline 推送 Harbor 失敗（401）

```bash
# 確認 Robot Account Token
curl -u ci-robot:<TOKEN> https://harbor.local/api/v2.0/projects

# 在 GitLab 重新設定 HARBOR_PASSWORD 變數
```
