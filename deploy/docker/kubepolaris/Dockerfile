# ==========================================
# KubePolaris 多阶段构建 Dockerfile
# 前后端一体化镜像
# ==========================================

# ==========================================
# Stage 1: Build Frontend
# ==========================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app/ui

# 增加 Node.js 堆内存限制（解决构建时 OOM 问题）
ENV NODE_OPTIONS="--max-old-space-size=4096"

# 安装依赖（利用缓存层）
COPY ui/package*.json ./
RUN npm ci --registry=https://registry.npmmirror.com

# 复制源代码并构建
COPY ui/ ./
RUN npm run build

# ==========================================
# Stage 2: Build Backend
# ==========================================
FROM golang:1.24-alpine AS backend-builder

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# 设置 Go 代理
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOOS=linux

# 复制依赖文件并下载依赖（利用缓存层）
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码并构建
COPY . .
RUN go build -ldflags="-s -w" -o kubepolaris ./cmd/main.go

# ==========================================
# Stage 3: Production Image
# ==========================================
FROM alpine:3.19

LABEL maintainer="KubePolaris Team"
LABEL description="KubePolaris - Enterprise Kubernetes Multi-Cluster Management Platform"
LABEL version="1.0.0"

# 安装必要的运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    nginx \
    curl \
    && rm -rf /var/cache/apk/*

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1000 kubepolaris && \
    adduser -u 1000 -G kubepolaris -s /bin/sh -D kubepolaris

WORKDIR /app

# 复制后端二进制文件
COPY --from=backend-builder /app/kubepolaris /app/
COPY --from=backend-builder /app/configs /app/configs

# 复制前端静态文件
COPY --from=frontend-builder /app/ui/dist /usr/share/nginx/html

# 复制 Nginx 配置
COPY deploy/docker/kubepolaris/nginx.conf /etc/nginx/nginx.conf

# 复制启动脚本
COPY deploy/docker/kubepolaris/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# 创建必要的目录
RUN mkdir -p /app/logs /app/data /var/log/nginx /var/lib/nginx/tmp && \
    chown -R kubepolaris:kubepolaris /app /var/log/nginx /var/lib/nginx /usr/share/nginx/html && \
    chmod -R 755 /var/lib/nginx

# 暴露端口
# 80: Nginx (前端)
# 8080: Backend API
EXPOSE 80 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/healthz || exit 1

# 使用启动脚本
ENTRYPOINT ["/app/entrypoint.sh"]

