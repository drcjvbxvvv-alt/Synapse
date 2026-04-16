# 多阶段构建：编译 + 运行时
# Dockerfile for Synapse Backend (Go + Gin)

# ─── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build

# 安装依赖
RUN apk add --no-cache git make ca-certificates tzdata

# 复制源程式碼
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 编译后端
RUN make build VERSION=${VERSION:-latest} \
    && ls -la bin/

# ─── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata curl

# 创建非 root 用户
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# 从构建镜像复制二进制文件
COPY --from=builder /build/bin/synapse /app/synapse
COPY --from=builder /build/deploy/config.example.yaml /app/config.yaml

# 配置目录
RUN mkdir -p /app/config /app/logs && \
    chown -R appuser:appgroup /app

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:8080/healthz || exit 1

# 启动应用
ENTRYPOINT ["/app/synapse"]
CMD ["--config", "/app/config.yaml"]
