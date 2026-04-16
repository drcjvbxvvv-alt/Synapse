# 多阶段构建：编译 + 静态服务
# Dockerfile for Synapse Frontend (React + Vite)

# ─── Stage 1: Build ───────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /build

# 复制项目文件
COPY ui/package.json ui/package-lock.json ./

# 安装依赖
RUN npm ci --frozen-lockfile

# 复制源程式碼
COPY ui/ .

# 构建应用
RUN npm run build && \
    ls -la dist/

# ─── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM nginx:alpine

# 移除默认配置
RUN rm -f /etc/nginx/conf.d/default.conf

# 创建自定义 nginx 配置（支持 SPA 路由）
RUN cat > /etc/nginx/conf.d/default.conf << 'EOF'
server {
    listen 80;
    server_name _;

    # 压缩启用
    gzip on;
    gzip_vary on;
    gzip_types text/plain text/css text/xml text/javascript
               application/x-javascript application/xml+rss
               application/javascript application/json;

    # SPA 路由重定向
    location / {
        root /usr/share/nginx/html;
        try_files $uri $uri/ /index.html;

        # 缓存控制
        expires -1;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    # 静态资源缓存（JS、CSS、字体）
    location ~* \.(js|css|svg|png|jpg|jpeg|gif|ico|woff|woff2|ttf|eot)$ {
        root /usr/share/nginx/html;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # 健康检查端点
    location /health {
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }

    # 拒绝访问 .git、.env 等
    location ~ /\. {
        deny all;
    }
}
EOF

# 创建非 root 用户
RUN addgroup -S nginx-user && adduser -S nginx-user -G nginx-user || true

WORKDIR /usr/share/nginx/html

# 从构建镜像复制构建产物
COPY --from=builder /build/dist/ /usr/share/nginx/html/

# 权限设置
RUN chown -R nginx:nginx /usr/share/nginx/html && \
    chmod -R 755 /usr/share/nginx/html

# 暴露端口
EXPOSE 80

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost/health || exit 1

# 启动 nginx
CMD ["nginx", "-g", "daemon off;"]
