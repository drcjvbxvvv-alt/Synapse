# ==========================================
# Synapse Makefile
# ==========================================

.PHONY: help dev dev-backend dev-frontend build build-backend build-frontend test test-backend test-frontend test-e2e test-e2e-ui lint lint-backend lint-frontend docker-build docker-push docker-up docker-down docker-logs docker-ps helm-package docs swagger clean version

# 变量
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REGISTRY ?= docker.io
IMAGE_NAME ?= registry.cn-hangzhou.aliyuncs.com/clay-wangzhi/synapse
COMPOSE_CMD := docker compose

# 颜色
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m

# 默认目标
.DEFAULT_GOAL := help

## help: 显示帮助信息
help:
	@echo ""
	@echo "$(BLUE)╔═══════════════════════════════════════════════════════════╗$(NC)"
	@echo "$(BLUE)║         Synapse Makefile 命令                          ║$(NC)"
	@echo "$(BLUE)╚═══════════════════════════════════════════════════════════╝$(NC)"
	@echo ""
	@echo "$(GREEN)開發命令:$(NC)"
	@echo "  make dev            - 啟動完整開發環境（MySQL + 後端 + 前端）"
	@echo "  make dev-backend    - 啟動 MySQL + 後端（不含前端）"
	@echo "  make dev-frontend   - 啟動前端開發伺服器"
	@echo "  make dev-mysql      - 僅啟動 MySQL + Adminer"
	@echo "  make dev-stop       - 停止所有開發服務"
	@echo "  make dev-reset      - 清除 MySQL volume 並重新初始化"
	@echo ""
	@echo "$(GREEN)构建命令:$(NC)"
	@echo "  make build          - 构建前后端"
	@echo "  make build-backend  - 构建后端"
	@echo "  make build-frontend - 构建前端"
	@echo ""
	@echo "$(GREEN)测试命令:$(NC)"
	@echo "  make test           - 运行所有测试"
	@echo "  make test-backend   - 运行后端测试"
	@echo "  make test-frontend  - 运行前端测试"
	@echo "  make test-e2e       - 运行 E2E 测试（Playwright headless）"
	@echo "  make test-e2e-ui    - 运行 E2E 测试（Playwright UI 模式）"
	@echo ""
	@echo "$(GREEN)代码检查:$(NC)"
	@echo "  make lint           - 运行代码检查"
	@echo "  make lint-backend   - 检查后端代码"
	@echo "  make lint-frontend  - 检查前端代码"
	@echo ""
	@echo "$(GREEN)Docker 命令:$(NC)"
	@echo "  make docker-build   - 构建 Docker 镜像"
	@echo "  make docker-push    - 推送 Docker 镜像"
	@echo "  make docker-up      - 启动 Docker Compose 服务"
	@echo "  make docker-down    - 停止 Docker Compose 服务"
	@echo "  make docker-logs    - 查看 Docker Compose 日志"
	@echo ""
	@echo "$(GREEN)部署命令:$(NC)"
	@echo "  make helm-package   - 打包 Helm Chart"
	@echo ""
	@echo "$(GREEN)其他命令:$(NC)"
	@echo "  make docs           - 生成文档"
	@echo "  make swagger        - 生成 Swagger 文档"
	@echo "  make clean          - 清理构建产物"
	@echo ""
	@echo "$(YELLOW)当前版本: $(VERSION)$(NC)"
	@echo ""

# ==========================================
# 开发命令
# ==========================================

## dev: 啟動完整開發環境（MySQL + 後端 + 前端）
dev:
	@bash scripts/dev.sh

## dev-backend: 啟動 MySQL + 後端（不含前端）
dev-backend:
	@bash scripts/dev.sh --backend-only

## dev-frontend: 啟動前端開發伺服器
dev-frontend:
	@bash scripts/dev.sh --frontend-only

## dev-mysql: 僅啟動 MySQL + Adminer
dev-mysql:
	@echo "$(BLUE)啟動 MySQL 開發環境...$(NC)"
	docker compose -f docker-compose.dev.yml up -d
	@echo "$(GREEN)MySQL 已啟動$(NC)"
	docker compose -f docker-compose.dev.yml ps

## dev-stop: 停止所有開發服務
dev-stop:
	@bash scripts/dev.sh --stop

## dev-reset: 清除 MySQL volume 並重新初始化
dev-reset:
	@bash scripts/dev.sh --reset

# ==========================================
# 构建命令
# ==========================================

## build: 构建前端并嵌入后端（单二进制）
build: build-frontend build-backend
	@echo "$(GREEN)构建完成: bin/synapse（前端已嵌入）$(NC)"

## build-backend: 构建后端（需要先构建前端）
build-backend:
	@echo "$(BLUE)构建后端...$(NC)"
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.Version=$(VERSION)" -o bin/synapse .
	@echo "$(GREEN)后端构建完成: bin/synapse$(NC)"

## build-frontend: 构建前端到 ui/dist
build-frontend:
	@echo "$(BLUE)构建前端...$(NC)"
	cd ui && npm ci && npm run build
	@echo "$(GREEN)前端构建完成: ui/dist/$(NC)"

# ==========================================
# 测试命令
# ==========================================

## test: 运行所有测试
test: test-backend test-frontend
	@echo "$(GREEN)所有测试完成$(NC)"

## test-backend: 运行后端测试
test-backend:
	@echo "$(BLUE)运行后端测试...$(NC)"
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)后端测试完成，覆盖率报告: coverage.html$(NC)"

## test-frontend: 运行前端测试
test-frontend:
	@echo "$(BLUE)运行前端测试...$(NC)"
	cd ui && npm run test 2>/dev/null || echo "$(YELLOW)前端测试尚未配置$(NC)"

## test-e2e: 运行 E2E 测试（headless）
test-e2e:
	@echo "$(BLUE)启动后端并运行 E2E 测试...$(NC)"
	@bash scripts/start-test-backend.sh
	cd ui && npx playwright test; EXIT_CODE=$$?; \
	bash ../scripts/stop-test-backend.sh; \
	exit $$EXIT_CODE

## test-e2e-ui: 以 UI 模式运行 E2E 测试
test-e2e-ui:
	@echo "$(BLUE)启动后端并打开 Playwright UI...$(NC)"
	@bash scripts/start-test-backend.sh
	cd ui && npx playwright test --ui; \
	bash ../scripts/stop-test-backend.sh

# ==========================================
# 代码检查
# ==========================================

## lint: 运行所有代码检查
lint: lint-backend lint-frontend
	@echo "$(GREEN)代码检查完成$(NC)"

## lint-backend: 检查后端代码
lint-backend:
	@echo "$(BLUE)检查后端代码...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(YELLOW)请安装 golangci-lint$(NC)" && exit 1)
	golangci-lint run ./...

## lint-frontend: 检查前端代码
lint-frontend:
	@echo "$(BLUE)检查前端代码...$(NC)"
	cd ui && npm run lint

# ==========================================
# Docker 命令
# ==========================================

## docker-build: 构建 Docker 镜像（前端嵌入后端，单镜像）
docker-build:
	@echo "$(BLUE)构建 Docker 镜像...$(NC)"
	docker build -t $(IMAGE_NAME):$(VERSION) -t $(IMAGE_NAME):latest .
	@echo "$(GREEN)镜像构建完成$(NC)"
	@echo "  $(IMAGE_NAME):$(VERSION)"

## docker-push: 推送 Docker 镜像
docker-push:
	@echo "$(BLUE)推送 Docker 镜像...$(NC)"
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest
	@echo "$(GREEN)镜像推送完成$(NC)"

## docker-up: 启动 Docker Compose 服务
docker-up:
	@echo "$(BLUE)启动 Docker Compose 服务...$(NC)"
	$(COMPOSE_CMD) up -d
	@echo "$(GREEN)服务已启动$(NC)"
	$(COMPOSE_CMD) ps

## docker-down: 停止 Docker Compose 服务
docker-down:
	@echo "$(BLUE)停止 Docker Compose 服务...$(NC)"
	$(COMPOSE_CMD) down
	@echo "$(GREEN)服务已停止$(NC)"

## docker-logs: 查看 Docker Compose 日志
docker-logs:
	$(COMPOSE_CMD) logs -f

## docker-ps: 查看 Docker Compose 状态
docker-ps:
	$(COMPOSE_CMD) ps

# ==========================================
# 部署命令
# ==========================================

## helm-package: 打包 Helm Chart
helm-package:
	@echo "$(BLUE)打包 Helm Chart...$(NC)"
	@if [ -d "deploy/helm/synapse" ]; then \
		helm package deploy/helm/synapse -d dist/; \
		echo "$(GREEN)Helm Chart 打包完成$(NC)"; \
	else \
		echo "$(YELLOW)Helm Chart 目录不存在，请先创建$(NC)"; \
	fi

# ==========================================
# 其他命令
# ==========================================

## docs: 生成文档
docs:
	@echo "$(BLUE)生成文档...$(NC)"
	@echo "$(YELLOW)文档生成功能待实现$(NC)"

## swagger: 生成 Swagger 文档
swagger:
	@echo "$(BLUE)生成 Swagger 文档...$(NC)"
	@which swag > /dev/null || (echo "$(YELLOW)请安装 swag: go install github.com/swaggo/swag/cmd/swag@latest$(NC)" && exit 1)
	swag init -g cmd/main.go -o docs/api
	@echo "$(GREEN)Swagger 文档生成完成: docs/api/$(NC)"

## clean: 清理构建产物
clean:
	@echo "$(BLUE)清理构建产物...$(NC)"
	rm -rf bin/
	rm -rf ui/dist/assets ui/dist/*.js ui/dist/*.css ui/dist/*.ico ui/dist/*.svg ui/dist/*.png
	rm -rf coverage.out coverage.html
	rm -rf dist/
	docker image prune -f
	@echo "$(GREEN)清理完成$(NC)"

## version: 显示版本信息
version:
	@echo "Version: $(VERSION)"
