# 使用 Jenkins 作为 CICD 编排器（可选）

如果你想使用 Jenkins 而不是 GitLab 内置的 CI/CD，或者想在现有的 GitLab CI 之外添加 Jenkins，本指南将展示如何集成。

## 🏗️ 完整 CICD 架构（带 Jenkins）

```
┌──────────────────┐
│   Git (GitLab)   │
└────────┬─────────┘
         │ Webhook
         ▼
┌──────────────────────────┐
│     Jenkins CI/CD        │
│  ┌────────────────────┐  │
│  │ Stage 1: Lint      │  │
│  │ Stage 2: Test      │  │
│  │ Stage 3: Build     │  │
│  │ Stage 4: Push      │  │
│  │ Stage 5: Deploy    │  │
│  └────────────────────┘  │
└────────┬─────────────────┘
         │
         ▼
┌──────────────────┐
│   Harbor         │
│  (镜像仓库)      │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│   Git Config     │
│  (K8s YAML)      │
└────────┬─────────┘
         │ Git Webhook
         ▼
┌──────────────────┐
│   ArgoCD         │
│ (GitOps)         │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Kubernetes      │
│ (应用部署)       │
└──────────────────┘
```

## 📦 Docker Compose 部署 Jenkins

### 1. 添加 Jenkins 到 docker-compose-cicd.yaml

```yaml
version: '3.8'

services:
  # ... 现有的 GitLab、Harbor、ArgoCD ...

  jenkins:
    image: jenkins/jenkins:lts
    container_name: jenkins
    restart: always
    ports:
      - "8082:8080"      # Jenkins Web UI
      - "50000:50000"    # Jenkins Agent 通信端口
    environment:
      JENKINS_OPTS: |
        -Dhudson.security.csrf.protection.enabled=false
        -Djenkins.install.runSetupWizard=false
    volumes:
      - jenkins-data:/var/jenkins_home
      - /var/run/docker.sock:/var/run/docker.sock  # Docker-in-Docker
    networks:
      - cicd

volumes:
  jenkins-data:
  # ... 其他 volumes ...

networks:
  cicd:
    driver: bridge
```

### 2. 启动 Jenkins

```bash
cd deploy/
docker compose -f docker-compose-cicd.yaml up -d jenkins

# 初始化 Jenkins（获取管理员密码）
docker exec jenkins cat /var/jenkins_home/secrets/initialAdminPassword
```

访问：http://localhost:8082

## 🔧 Jenkinsfile 配置

在项目根目录创建 `Jenkinsfile`：

```groovy
pipeline {
    agent any

    environment {
        HARBOR_REGISTRY = "harbor.local"
        HARBOR_USERNAME = credentials('harbor-username')
        HARBOR_PASSWORD = credentials('harbor-password')
        ARGOCD_TOKEN = credentials('argocd-token')
        IMAGE_TAG = "${BUILD_NUMBER}-${GIT_COMMIT.take(7)}"
        BACKEND_IMAGE = "${HARBOR_REGISTRY}/synapse/backend:${IMAGE_TAG}"
        FRONTEND_IMAGE = "${HARBOR_REGISTRY}/synapse/frontend:${IMAGE_TAG}"
        JAVA_A_IMAGE = "${HARBOR_REGISTRY}/saas/java-a:${IMAGE_TAG}"
        JAVA_B_IMAGE = "${HARBOR_REGISTRY}/saas/java-b:${IMAGE_TAG}"
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
                script {
                    echo "Git Commit: ${GIT_COMMIT}"
                    echo "Build Number: ${BUILD_NUMBER}"
                }
            }
        }

        stage('Lint') {
            parallel {
                stage('Go Lint') {
                    steps {
                        script {
                            sh '''
                                golangci-lint run ./cmd ./internal ./pkg --timeout 5m || true
                            '''
                        }
                    }
                }
                stage('Frontend Lint') {
                    steps {
                        script {
                            sh '''
                                cd ui
                                npm ci --frozen-lockfile
                                npm run lint || true
                            '''
                        }
                    }
                }
            }
        }

        stage('Test') {
            parallel {
                stage('Backend Test') {
                    steps {
                        script {
                            sh '''
                                go test -v -coverprofile=coverage.out ./cmd/... ./internal/... ./pkg/...
                            '''
                        }
                    }
                }
                stage('Frontend Test') {
                    steps {
                        script {
                            sh '''
                                cd ui
                                npm run test:run || true
                            '''
                        }
                    }
                }
            }
        }

        stage('Build') {
            parallel {
                stage('Backend Build') {
                    steps {
                        script {
                            sh '''
                                mkdir -p bin
                                go build -o bin/synapse ./cmd/main.go
                            '''
                        }
                    }
                }
                stage('Frontend Build') {
                    steps {
                        script {
                            sh '''
                                cd ui
                                npm run build
                            '''
                        }
                    }
                }
                stage('Java-A Build') {
                    steps {
                        script {
                            sh '''
                                cd saas-java-a
                                mvn clean package -DskipTests
                            '''
                        }
                    }
                }
                stage('Java-B Build') {
                    steps {
                        script {
                            sh '''
                                cd saas-java-b
                                mvn clean package -DskipTests
                            '''
                        }
                    }
                }
            }
        }

        stage('Docker Build & Push') {
            parallel {
                stage('Backend Docker') {
                    steps {
                        script {
                            sh '''
                                docker login -u ${HARBOR_USERNAME} -p ${HARBOR_PASSWORD} ${HARBOR_REGISTRY}
                                docker build -f deploy/docker/backend/Dockerfile -t ${BACKEND_IMAGE} .
                                docker push ${BACKEND_IMAGE}
                                docker tag ${BACKEND_IMAGE} ${HARBOR_REGISTRY}/synapse/backend:latest
                                docker push ${HARBOR_REGISTRY}/synapse/backend:latest
                                docker logout
                            '''
                        }
                    }
                }
                stage('Frontend Docker') {
                    steps {
                        script {
                            sh '''
                                docker login -u ${HARBOR_USERNAME} -p ${HARBOR_PASSWORD} ${HARBOR_REGISTRY}
                                docker build -f deploy/docker/frontend/Dockerfile -t ${FRONTEND_IMAGE} .
                                docker push ${FRONTEND_IMAGE}
                                docker tag ${FRONTEND_IMAGE} ${HARBOR_REGISTRY}/synapse/frontend:latest
                                docker push ${HARBOR_REGISTRY}/synapse/frontend:latest
                                docker logout
                            '''
                        }
                    }
                }
                stage('Java-A Docker') {
                    steps {
                        script {
                            sh '''
                                docker login -u ${HARBOR_USERNAME} -p ${HARBOR_PASSWORD} ${HARBOR_REGISTRY}
                                docker build -f deploy/docker/saas-java-a/Dockerfile -t ${JAVA_A_IMAGE} saas-java-a/
                                docker push ${JAVA_A_IMAGE}
                                docker tag ${JAVA_A_IMAGE} ${HARBOR_REGISTRY}/saas/java-a:latest
                                docker push ${HARBOR_REGISTRY}/saas/java-a:latest
                                docker logout
                            '''
                        }
                    }
                }
                stage('Java-B Docker') {
                    steps {
                        script {
                            sh '''
                                docker login -u ${HARBOR_USERNAME} -p ${HARBOR_PASSWORD} ${HARBOR_REGISTRY}
                                docker build -f deploy/docker/saas-java-b/Dockerfile -t ${JAVA_B_IMAGE} saas-java-b/
                                docker push ${JAVA_B_IMAGE}
                                docker tag ${JAVA_B_IMAGE} ${HARBOR_REGISTRY}/saas/java-b:latest
                                docker push ${HARBOR_REGISTRY}/saas/java-b:latest
                                docker logout
                            '''
                        }
                    }
                }
            }
        }

        stage('Update K8s Config') {
            steps {
                script {
                    sh '''
                        # 配置 Git
                        git config --global user.email "jenkins@synapse.local"
                        git config --global user.name "Jenkins CI/CD"

                        # 更新镜像版本
                        sed -i "s|image: .*synapse/backend:.*|image: ${BACKEND_IMAGE}|g" deploy/k8s/synapse-deployment.yaml
                        sed -i "s|image: .*synapse/frontend:.*|image: ${FRONTEND_IMAGE}|g" deploy/k8s/synapse-deployment.yaml
                        sed -i "s|image: .*saas/java-a:.*|image: ${JAVA_A_IMAGE}|g" deploy/k8s/saas-java-a-deployment.yaml
                        sed -i "s|image: .*saas/java-b:.*|image: ${JAVA_B_IMAGE}|g" deploy/k8s/saas-java-b-deployment.yaml

                        # 提交并推送
                        git add deploy/k8s/
                        git commit -m "ci: update all apps to ${IMAGE_TAG}" || true
                        git push origin main
                    '''
                }
            }
        }

        stage('Trigger ArgoCD Sync') {
            steps {
                script {
                    sh '''
                        curl -X POST \\
                            -H "Authorization: Bearer ${ARGOCD_TOKEN}" \\
                            -H "Content-Type: application/json" \\
                            "http://argocd.local:8081/api/v1/applications/synapse/sync" \\
                            -d '{
                                "strategy": "auto",
                                "syncOptions": ["CreateNamespace=true"],
                                "dryRun": false,
                                "prune": true
                            }' || true
                    '''
                }
            }
        }
    }

    post {
        always {
            // 清理 Docker 登录
            sh 'docker logout || true'
        }
        success {
            echo "✅ Pipeline Success: ${BUILD_NUMBER}"
        }
        failure {
            echo "❌ Pipeline Failed: ${BUILD_NUMBER}"
        }
    }
}
```

## 🔐 Jenkins 凭证配置

在 Jenkins 中添加凭证：

**Manage Jenkins → Manage Credentials → Add Credentials**

1. **harbor-username**（类型：Username with password）
   - Username: `admin`
   - Password: `Harbor@2026`

2. **harbor-password**（类型：Secret text）
   - Secret: `Harbor@2026`

3. **argocd-token**（类型：Secret text）
   - Secret: `<your-argocd-token>`

## 🚀 创建 Jenkins Pipeline 任务

1. **创建新任务**
   - Jenkins 主页 → New Item
   - 输入名称：`Synapse-CICD`
   - 选择 Pipeline

2. **配置 Pipeline**
   - General → GitHub project
   - URL: `https://github.com/your-org/Synapse`

3. **构建触发器**
   - GitHub hook trigger for GITScm polling
   - 或 Poll SCM: `H/5 * * * *`（每 5 分钟轮询一次）

4. **Pipeline**
   - Definition: Pipeline script from SCM
   - SCM: Git
   - Repository URL: `https://github.com/your-org/Synapse.git`
   - Branch: `*/main`
   - Script Path: `Jenkinsfile`

5. **保存并构建**

## 🔗 GitLab 与 Jenkins 集成

在 GitLab 中设置 Webhook：

**项目 → Settings → Webhooks**

```
URL: http://jenkins.local:8082/github-webhook/
Trigger: Push events
SSL verification: 取消勾选（仅用于测试）
```

## 📊 GitLab CI vs Jenkins 混合使用

如果想同时使用两者：

### .gitlab-ci.yml（轻任务）
```yaml
stages:
  - lint
  - notify

lint:
  stage: lint
  script:
    - echo "Quick lint only"

notify-jenkins:
  stage: notify
  script:
    - curl -X POST http://jenkins.local:8082/job/Synapse-CICD/buildWithParameters
```

### Jenkinsfile（重任务）
```groovy
// 完整的构建、测试、推送、部署
```

## ✅ 检查清单

- [ ] Jenkins 容器启动
- [ ] Jenkins 初始密码获取
- [ ] 安装必要插件（Git、Docker、Kubernetes）
- [ ] 添加凭证（Harbor、ArgoCD）
- [ ] 创建 Pipeline 任务
- [ ] 配置 Git Webhook
- [ ] 首次手动构建成功
- [ ] Webhook 自动触发测试

## 🆚 选择指南

### 使用 GitLab CI 如果：
- ✅ 项目已在 GitLab 中
- ✅ 想要简单的开箱即用方案
- ✅ 团队规模小
- ✅ 功能需求不复杂

### 使用 Jenkins 如果：
- ✅ 需要复杂的工作流编排
- ✅ 需要更多的自定义和灵活性
- ✅ 需要与多个 Git 平台集成
- ✅ 已有 Jenkins 基础设施
- ✅ 需要高级的任务调度

### 同时使用两者如果：
- ✅ GitLab CI 处理快速检查
- ✅ Jenkins 处理重型构建任务
- ✅ 需要最大的灵活性

---

**总结**：

在现有架构中：
- GitLab CI 已经包含了 Jenkins 的所有核心功能
- 如果需要更多控制，可以添加 Jenkins
- 两者都指向同一个 Harbor → ArgoCD → Kubernetes 的流程

**推荐**：对于 Synapse 项目，**GitLab CI 足够了**，除非有特殊需求才考虑 Jenkins。

最后更新：2026-04-16
