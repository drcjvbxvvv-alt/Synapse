#!/bin/sh
# ==========================================
# KubePolaris 启动脚本
# 用于一体化镜像启动前后端服务
# ==========================================

set -e

echo "========================================"
echo "  KubePolaris - Starting Services"
echo "========================================"

# 等待 MySQL 连接就绪（如果配置了）
if [ -n "$DB_HOST" ]; then
    echo "⏳ Waiting for MySQL to be ready..."
    max_retries=30
    retry_count=0
    while ! nc -z $DB_HOST ${DB_PORT:-3306} 2>/dev/null; do
        retry_count=$((retry_count + 1))
        if [ $retry_count -ge $max_retries ]; then
            echo "❌ Failed to connect to MySQL after $max_retries attempts"
            exit 1
        fi
        echo "  Waiting for MySQL... ($retry_count/$max_retries)"
        sleep 2
    done
    echo "✅ MySQL is ready!"
fi

# 启动后端服务
echo "🚀 Starting backend service..."
/app/kubepolaris &
BACKEND_PID=$!

# 等待后端服务就绪
sleep 3
if ! kill -0 $BACKEND_PID 2>/dev/null; then
    echo "❌ Backend service failed to start"
    exit 1
fi
echo "✅ Backend service started (PID: $BACKEND_PID)"

# 启动 Nginx
echo "🚀 Starting Nginx..."
nginx -g 'daemon off;' &
NGINX_PID=$!

echo "✅ Nginx started (PID: $NGINX_PID)"
echo ""
echo "========================================"
echo "  KubePolaris is running!"
echo "  Frontend: http://localhost:80"
echo "  Backend:  http://localhost:8080"
echo "========================================"

# 等待任一进程退出
wait -n $BACKEND_PID $NGINX_PID

# 如果任一进程退出，则终止另一个
echo "⚠️  One of the services has stopped, shutting down..."
kill $BACKEND_PID 2>/dev/null || true
kill $NGINX_PID 2>/dev/null || true
exit 1

