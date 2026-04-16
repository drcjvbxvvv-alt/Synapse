# 多阶段构建：构建 + 运行时
# Dockerfile for saas-java-a (minimal Spring Boot application)

# ─── Build Stage ──────────────────────────────────────────────────────────────
FROM maven:3.8-openjdk-11-slim as builder

WORKDIR /build

# 复制源程式碼（假设目录结构：saas-java-a/）
COPY . .

# 编译并打包
RUN mvn clean package -DskipTests && \
    ls -la target/

# ─── Runtime Stage ────────────────────────────────────────────────────────────
FROM openjdk:11-jre-slim

# 非 root 用户
RUN groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app

# 从构建镜像复制 JAR 文件
COPY --from=builder /build/target/*.jar app.jar

# 应用配置目录
COPY config/ /app/config/ 2>/dev/null || true

# 所有权设置
RUN chown -R appuser:appuser /app

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:8080/actuator/health || exit 1

# JVM 参数优化
ENV JAVA_OPTS="-Xms256m -Xmx512m -XX:+UseG1GC -XX:+UnlockExperimentalVMOptions -XX:G1NewCollectionPercentage=30 -XX:G1MaxNewGenTaskPercent=40"

# 启动应用
ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar app.jar"]
