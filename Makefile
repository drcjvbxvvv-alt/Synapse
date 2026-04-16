# Synapse Makefile
# 本地开发和构建脚本

.PHONY: help build build-docker build-docker-all test lint clean dev docker-up docker-down

# 默认目标
.DEFAULT_GOAL := help

# 版本信息
VERSION ?= $(shell git describe --tags --always --dirty)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# 目录
BIN_DIR := bin
BACKEND_DIR := .
FRONTEND_DIR := ui

# 输出二进制文件名
BINARY := synapse

# 构建标志
LDFLAGS := -ldflags="-X github.com/shaia/Synapse/pkg/version.Version=$(VERSION) \
                      -X github.com/shaia/Synapse/pkg/version.GitCommit=$(GIT_COMMIT) \
                      -X github.com/shaia/Synapse/pkg/version.BuildTime=$(BUILD_TIME)"

# ─── Help ──────────────────────────────────────────────────────────────────

help:  ## 显示此帮助信息
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk \
	'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── 后端构建 ──────────────────────────────────────────────────────────────

build: ## 本地构建后端二进制文件
	@echo "🔨 Building backend... (Version: $(VERSION))"
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
		$(LDFLAGS) \
		-o $(BIN_DIR)/$(BINARY) \
		./cmd/main.go
	@echo "✅ Binary saved to $(BIN_DIR)/$(BINARY)"
	@ls -lh $(BIN_DIR)/$(BINARY)

# ─── 前端构建 ──────────────────────────────────────────────────────────────

build-frontend: ## 构建前端
	@echo "🔨 Building frontend..."
	cd $(FRONTEND_DIR) && npm run build
	@echo "✅ Frontend built to $(FRONTEND_DIR)/dist/"

# ─── Docker 镜像构建 ────────────────────────────────────────────────────────

docker-backend: ## 构建后端 Docker 镜像
	@echo "🐳 Building backend Docker image..."
	docker build -f deploy/docker/backend/Dockerfile \
		-t synapse:backend-$(VERSION) \
		-t synapse:backend-latest \
		.
	@echo "✅ Image: synapse:backend-latest"

docker-frontend: ## 构建前端 Docker 镜像
	@echo "🐳 Building frontend Docker image..."
	docker build -f deploy/docker/frontend/Dockerfile \
		-t synapse:frontend-$(VERSION) \
		-t synapse:frontend-latest \
		.
	@echo "✅ Image: synapse:frontend-latest"

docker-all: docker-backend docker-frontend ## 构建所有 Docker 镜像

docker-push: ## 推送镜像到 Harbor
	@echo "📤 Pushing images to Harbor..."
	docker tag synapse:backend-latest harbor.local/synapse/backend:$(VERSION)
	docker tag synapse:backend-latest harbor.local/synapse/backend:latest
	docker push harbor.local/synapse/backend:$(VERSION)
	docker push harbor.local/synapse/backend:latest
	docker tag synapse:frontend-latest harbor.local/synapse/frontend:$(VERSION)
	docker tag synapse:frontend-latest harbor.local/synapse/frontend:latest
	docker push harbor.local/synapse/frontend:$(VERSION)
	docker push harbor.local/synapse/frontend:latest
	@echo "✅ Images pushed successfully"

# ─── 测试 ──────────────────────────────────────────────────────────────────

test: ## 运行后端单元测试
	@echo "🧪 Running backend tests..."
	go test -v -coverprofile=coverage.out ./cmd/... ./internal/... ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

test-frontend: ## 运行前端单元测试
	@echo "🧪 Running frontend tests..."
	cd $(FRONTEND_DIR) && npm run test:run

# ─── 代码质量 ──────────────────────────────────────────────────────────────

lint: ## 运行后端代码检查
	@echo "🔍 Running golangci-lint..."
	golangci-lint run ./cmd ./internal ./pkg --timeout=5m

lint-frontend: ## 运行前端 ESLint
	@echo "🔍 Running ESLint..."
	cd $(FRONTEND_DIR) && npm run lint

# ─── Docker Compose ─────────────────────────────────────────────────────────

docker-up: ## 启动 CICD 基础设施（Docker Compose）
	@echo "🐳 Starting Docker Compose services..."
	cd deploy && docker compose -f docker-compose-cicd.yaml up -d
	@echo ""
	@echo "✅ Services started:"
	@echo "   GitLab:  http://localhost"
	@echo "   Harbor:  http://localhost:8080"
	@echo "   ArgoCD:  http://localhost:8081"

docker-down: ## 停止 CICD 基础设施
	@echo "🛑 Stopping Docker Compose services..."
	cd deploy && docker compose -f docker-compose-cicd.yaml down

docker-logs: ## 查看 Docker Compose 日志
	cd deploy && docker compose -f docker-compose-cicd.yaml logs -f

# ─── Kubernetes 部署 ────────────────────────────────────────────────────────

k8s-deploy: ## 部署 Synapse 到 Kubernetes
	@echo "📦 Deploying to Kubernetes..."
	kubectl apply -f deploy/k8s/synapse-deployment.yaml
	@echo "✅ Deployment started"

k8s-status: ## 查看 K8s 部署状态
	@echo "📊 Deployment status:"
	kubectl get pods -n synapse -w

k8s-logs: ## 查看 K8s 应用日志
	@echo "📋 Backend logs:"
	kubectl logs -n synapse deployment/synapse-backend -f

# ─── 清理 ──────────────────────────────────────────────────────────────────

clean: ## 清理构建产物
	@echo "🧹 Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	cd $(FRONTEND_DIR) && rm -rf dist node_modules .next
	@echo "✅ Cleaned"

clean-all: clean ## 完全清理

# ─── 其他工具 ──────────────────────────────────────────────────────────────

version: ## 显示版本信息
	@echo "Version: $(VERSION)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

# ─── CI/CD 模拟 ────────────────────────────────────────────────────────────

ci: lint test build docker-all ## 运行完整 CI 流程（本地模拟）
	@echo "✅ CI pipeline completed successfully"

quick-start: docker-up ## 快速启动完整 CICD 环境
	@echo ""
	@echo "🎉 Quick start completed!"
	@echo ""
	@echo "Next steps:"
	@echo "1. Push code to GitLab (auto-triggers CI)"
	@echo "2. Monitor ArgoCD: http://localhost:8081"
	@echo "3. Check deployment: kubectl get pods -n synapse"
