#!/bin/bash
# ==========================================
# KubePolaris 升级脚本
# ==========================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$DEPLOY_DIR")"

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 打印 Banner
print_banner() {
    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║         🔄 KubePolaris 升级程序                           ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# 检测 Docker Compose 命令
detect_compose_cmd() {
    if command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    elif docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    else
        log_error "Docker Compose 未安装"
        exit 1
    fi
}

# 备份数据库
backup_database() {
    log_info "备份数据库..."
    
    BACKUP_DIR="$PROJECT_ROOT/backups"
    BACKUP_FILE="${BACKUP_DIR}/kubepolaris_$(date +%Y%m%d_%H%M%S).sql"
    
    mkdir -p "$BACKUP_DIR"
    
    # 从 .env 文件读取密码
    local ENV_FILE="$DEPLOY_DIR/docker-compose/.env"
    if [ -f "$ENV_FILE" ]; then
        source "$ENV_FILE"
    fi
    
    cd "$DEPLOY_DIR/docker-compose"
    $COMPOSE_CMD exec -T mysql mysqldump -u root -p"${MYSQL_ROOT_PASSWORD:-root123456}" kubepolaris > "$BACKUP_FILE" 2>/dev/null
    
    if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
        log_success "数据库备份完成: $BACKUP_FILE"
    else
        log_warn "数据库备份可能失败，请手动检查"
    fi
}

# 拉取最新代码
pull_latest_code() {
    log_info "拉取最新代码..."
    
    cd "$PROJECT_ROOT"
    
    if [ -d ".git" ]; then
        git fetch origin
        
        CURRENT_BRANCH=$(git branch --show-current)
        log_info "当前分支: $CURRENT_BRANCH"
        
        # 检查是否有未提交的更改
        if ! git diff --quiet; then
            log_warn "检测到未提交的更改"
            read -p "是否继续？这将覆盖本地更改 [y/N] " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                log_info "升级已取消"
                exit 0
            fi
        fi
        
        git pull origin "$CURRENT_BRANCH"
        log_success "代码更新完成"
    else
        log_warn "未检测到 Git 仓库，跳过代码更新"
    fi
}

# 构建新镜像
build_new_images() {
    log_info "构建新镜像..."
    
    cd "$DEPLOY_DIR/docker-compose"
    $COMPOSE_CMD build --no-cache
    
    log_success "镜像构建完成"
}

# 停止旧服务
stop_old_services() {
    log_info "停止旧服务..."
    
    cd "$DEPLOY_DIR/docker-compose"
    $COMPOSE_CMD stop backend frontend
    
    log_success "旧服务已停止"
}

# 启动新服务
start_new_services() {
    log_info "启动新服务..."
    
    cd "$DEPLOY_DIR/docker-compose"
    $COMPOSE_CMD up -d
    
    log_success "新服务已启动"
}

# 健康检查
health_check() {
    log_info "执行健康检查..."
    
    # 等待后端
    for i in {1..30}; do
        if curl -s http://localhost:8080/healthz &> /dev/null; then
            log_success "后端服务正常"
            break
        fi
        if [ $i -eq 30 ]; then
            log_error "后端服务健康检查失败"
            exit 1
        fi
        sleep 2
    done
    
    # 等待前端
    for i in {1..30}; do
        if curl -s http://localhost:80/health &> /dev/null; then
            log_success "前端服务正常"
            break
        fi
        if [ $i -eq 30 ]; then
            log_warn "前端服务可能仍在启动中"
        fi
        sleep 2
    done
}

# 清理旧镜像
cleanup_old_images() {
    log_info "清理旧镜像..."
    
    docker image prune -f
    
    log_success "旧镜像已清理"
}

# 显示升级完成信息
show_complete_info() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║               🎉 升级完成！                                ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BLUE}服务状态:${NC}"
    cd "$DEPLOY_DIR/docker-compose"
    $COMPOSE_CMD ps
    echo ""
    echo -e "${BLUE}查看日志:${NC}"
    echo -e "  ${YELLOW}cd $DEPLOY_DIR/docker-compose && $COMPOSE_CMD logs -f${NC}"
    echo ""
}

# 回滚函数
rollback() {
    log_warn "升级失败，正在回滚..."
    
    # 这里可以添加回滚逻辑
    # 比如使用备份的镜像重新启动
    
    log_info "请手动检查并修复问题"
}

# 主函数
main() {
    print_banner
    
    detect_compose_cmd
    
    # 确认升级
    echo -e "${YELLOW}此操作将升级 KubePolaris 到最新版本${NC}"
    read -p "是否继续？[y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "升级已取消"
        exit 0
    fi
    
    # 备份数据库
    read -p "是否备份数据库？[Y/n] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
        backup_database
    fi
    
    # 执行升级步骤
    trap rollback ERR
    
    pull_latest_code
    build_new_images
    stop_old_services
    start_new_services
    health_check
    cleanup_old_images
    
    show_complete_info
}

# 运行
main "$@"
