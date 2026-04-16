# Synapse CICD — Java 應用使用指南

> 版本：v1.0 | 更新日期：2026-04-16
> 適用對象：Java SaaS 應用的 DevOps 工程師

---

## 目錄

1. [前置需求](#1-前置需求)
2. [Dockerfile](#2-dockerfile)
3. [GitLab CI 設定](#3-gitlab-ci-設定)
4. [Kubernetes 部署規格](#4-kubernetes-部署規格)
5. [Harbor 映像倉庫](#5-harbor-映像倉庫)
6. [GitOps（ArgoCD）](#6-gitopsargocd)
7. [故障排查](#7-故障排查)

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

## 3. GitLab CI 設定

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

## 4. Kubernetes 部署規格

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

## 5. Harbor 映像倉庫

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

## 6. GitOps（ArgoCD）

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

## 7. 故障排查

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
