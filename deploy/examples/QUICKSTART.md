# CICD 流程快速开始指南

本指南将引导你在 Synapse 中快速搭建完整的 CICD 流程：GitLab → Harbor → ArgoCD。

## 🚀 快速步骤

### 1. 启动 CICD 组件 (Docker Compose)

```bash
cd deploy/
docker compose -f docker-compose-cicd.yaml up -d

# 查看状态
docker compose ps
```

**访问地址**：
| 服务 | URL | 用户名 | 密码 |
|------|-----|--------|------|
| GitLab | http://localhost | admin | Gitlab@2026 |
| Harbor | http://localhost:8080 | admin | Harbor@2026 |
| ArgoCD | http://localhost:8081 | admin | (见下方) |

### 2. 初始化 ArgoCD 管理员密码

```bash
# 进入 ArgoCD 容器
docker exec -it argocd sh

# 获取自动生成的密码
argocd admin initial-password -n argocd

# 或设置新密码
argocd account update-password --current-password <old-pass> --new-password <new-pass>
```

### 3. 创建示例项目结构

```bash
# 创建两个最小的 Spring Boot 应用
mkdir -p saas-java-a saas-java-b

# 在每个目录下创建 pom.xml
cat > saas-java-a/pom.xml << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
                             http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.saas</groupId>
    <artifactId>java-a</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>
    <name>saas-java-a</name>
    
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>2.7.0</version>
        <relativePath/>
    </parent>
    
    <properties>
        <java.version>11</java.version>
    </properties>
    
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-actuator</artifactId>
        </dependency>
    </dependencies>
    
    <build>
        <plugins>
            <plugin>
                <groupId>org.springframework.boot</groupId>
                <artifactId>spring-boot-maven-plugin</artifactId>
            </plugin>
        </plugins>
    </build>
</project>
EOF
```

### 4. 创建最小 Spring Boot 应用

```bash
# 创建应用入口
mkdir -p saas-java-a/src/main/java/com/saas
mkdir -p saas-java-a/src/main/resources

cat > saas-java-a/src/main/java/com/saas/Application.java << 'EOF'
package com.saas;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
@RestController
public class Application {
    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }
    
    @GetMapping("/")
    public String hello() {
        return "Hello from saas-java-a";
    }
    
    @GetMapping("/version")
    public String version() {
        return "1.0.0";
    }
}
EOF

# 创建应用配置
cat > saas-java-a/src/main/resources/application.properties << 'EOF'
spring.application.name=saas-java-a
spring.profiles.active=prod
server.port=8080
logging.level.root=INFO
management.endpoints.web.exposure.include=health,metrics,prometheus
EOF
```

### 5. 构建并推送到 Harbor

```bash
# 构建应用
cd saas-java-a
mvn clean package

# 构建 Docker 镜像
docker build -f ../deploy/docker/saas-java-a/Dockerfile -t harbor.local/saas/java-a:latest .

# 推送到 Harbor（确保 Harbor 容器可访问）
docker login harbor.local -u admin -p Harbor@2026
docker push harbor.local/saas/java-a:latest
```

### 6. 部署到 Kubernetes

```bash
# 如果使用本地 K8s 集群（如 Docker Desktop、minikube）
kubectl apply -f deploy/k8s/saas-java-a-deployment.yaml
kubectl apply -f deploy/k8s/saas-java-b-deployment.yaml

# 验证部署
kubectl get pods -n saas-java-a
kubectl get svc -n saas-java-a
kubectl logs -n saas-java-a deployment/saas-java-a -f
```

### 7. 配置 GitLab CI/CD

1. **在 GitLab 中创建项目**
   - 登录 http://localhost
   - 新建项目 → 输入 `Synapse` 作为项目名

2. **添加 .gitlab-ci.yml**
   - 复制 `deploy/examples/gitlab-ci-example.yml` 为 `.gitlab-ci.yml`
   - 修改仓库和镜像地址为实际值

3. **设置 Runner（可选）**
   ```bash
   # GitLab 容器已包含 Runner，无需额外配置
   docker exec -it gitlab-runner gitlab-runner register
   ```

### 8. 配置 ArgoCD

1. **登录 ArgoCD**
   ```bash
   argocd login localhost:8081 --username admin --password <your-password>
   ```

2. **添加 Git 仓库源**
   ```bash
   argocd repo add https://github.com/your-org/Synapse \
     --username git-user \
     --password git-password
   ```

3. **创建 Application**
   ```bash
   kubectl apply -f deploy/examples/argocd-application-example.yaml
   ```

4. **在 UI 中检查同步状态**
   - 访问 http://localhost:8081
   - 查看 `saas-java-a` 和 `saas-java-b` 应用的部署状态

## 📊 完整流程演示

### 场景：更新应用并自动部署

```bash
# 1. 修改应用程式碼
vi saas-java-a/src/main/java/com/saas/Application.java

# 2. 提交并推送到 GitLab
git add .
git commit -m "chore: update java-a version"
git push origin main

# 3. GitLab CI 自动触发：
#    - 编译应用
#    - 构建 Docker 镜像
#    - 推送到 Harbor
#    - 触发 ArgoCD 同步

# 4. 查看部署进度
kubectl rollout status deployment/saas-java-a -n saas-java-a

# 5. 验证新版本
kubectl logs -n saas-java-a deployment/saas-java-a -f
```

## 🔧 常见操作

### 查看 Harbor 镜像

```bash
# 进入 Harbor 容器（可选）
docker exec -it harbor sh

# 或通过 UI：http://localhost:8080
# 用户名：admin
# 密码：Harbor@2026
```

### 查看 GitLab 日志

```bash
docker logs -f gitlab
```

### 查看 ArgoCD 事件

```bash
kubectl describe application saas-java-a -n argocd
argocd app logs saas-java-a
```

### 手动同步应用

```bash
argocd app sync saas-java-a
argocd app sync saas-java-b
```

## 🐛 故障排查

### Harbor 连接失败

```bash
# 检查 Harbor 健康状态
docker compose ps

# 查看 Harbor 日志
docker compose logs harbor

# 确认网络连接
docker network inspect cicd
```

### GitLab Runner 失败

```bash
# 查看 Runner 日志
docker logs gitlab-runner

# 检查 Docker daemon
docker info
```

### ArgoCD 无法拉取 Git 仓库

```bash
# 验证 SSH 密钥
argocd repo list

# 重新配置仓库
argocd repo rm https://github.com/your-org/Synapse
argocd repo add https://github.com/your-org/Synapse --ssh-private-key-path ~/.ssh/id_rsa
```

## 📝 环境变量配置

在 GitLab CI/CD 中设置必要的变量：

1. **进入项目设置** → CI/CD → Variables
2. **添加以下变量**：

| 变量名 | 值 |
|--------|-----|
| `KUBE_CONFIG_BASE64` | Base64 编码的 kubeconfig |
| `ARGOCD_TOKEN` | ArgoCD API Token |
| `HARBOR_PASSWORD` | Harbor 密码 |

**获取 KUBE_CONFIG_BASE64**：
```bash
cat ~/.kube/config | base64 | tr -d '\n'
```

**获取 ARGOCD_TOKEN**：
```bash
argocd account generate-token --account argocd --duration 0
```

## 🎯 下一步

- 配置 Prometheus 和 Grafana 监控应用
- 设置告警规则
- 配置自动回滚策略
- 实现蓝绿部署或金丝雀发布

## 📚 参考资源

- [GitLab CI/CD 文档](https://docs.gitlab.com/ee/ci/)
- [Harbor 用户指南](https://goharbor.io/docs/2.0/)
- [ArgoCD 文档](https://argo-cd.readthedocs.io/)
- [Kubernetes 部署最佳实践](https://kubernetes.io/docs/concepts/configuration/overview/)

---

**最后更新**：2026-04-16
