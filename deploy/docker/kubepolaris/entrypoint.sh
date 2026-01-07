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

# 创建配置文件（如果使用环境变量）
if [ -n "$DB_HOST" ]; then
    echo "📝 Generating config from environment variables..."
    cat > /app/configs/config.yaml << EOF
server:
  port: ${SERVER_PORT:-8080}
  mode: ${SERVER_MODE:-release}

database:
  driver: mysql
  host: ${DB_HOST:-127.0.0.1}
  port: ${DB_PORT:-3306}
  username: ${DB_USERNAME:-kubepolaris}
  password: ${DB_PASSWORD:-kubepolaris}
  database: ${DB_DATABASE:-kubepolaris}
  charset: utf8mb4

jwt:
  secret: ${JWT_SECRET:-k8s-management-secret-key}
  expire_time: ${JWT_EXPIRE_TIME:-24}

log:
  level: ${LOG_LEVEL:-info}

k8s:
  default_namespace: ${K8S_DEFAULT_NAMESPACE:-default}

grafana:
  enabled: ${GRAFANA_ENABLED:-true}
  url: ${GRAFANA_URL:-http://grafana:3000}
  api_key: "${GRAFANA_API_KEY:-}"
  api_key_file: "${GRAFANA_API_KEY_FILE:-./grafana/secrets/grafana_api_key}"
EOF
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

