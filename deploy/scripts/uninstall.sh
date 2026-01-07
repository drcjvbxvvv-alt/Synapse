#!/bin/bash
# ==========================================
# KubePolaris 卸载脚本
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
    echo -e "${RED}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║         ⚠️  KubePolaris 卸载程序                           ║${NC}"
    echo -e "${RED}╚═══════════════════════════════════════════════════════════╝${NC}"
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

# 备份数据
backup_data() {
    log_info "备份数据..."
    
    BACKUP_DIR="$PROJECT_ROOT/backups"
    BACKUP_FILE="${BACKUP_DIR}/kubepolaris_uninstall_$(date +%Y%m%d_%H%M%S).sql"
    
    mkdir -p "$BACKUP_DIR"
    
    # 从 .env 文件读取密码
    local ENV_FILE="$DEPLOY_DIR/docker-compose/.env"
    if [ -f "$ENV_FILE" ]; then
        source "$ENV_FILE"
    fi
    
    cd "$DEPLOY_DIR/docker-compose"
    
    # 检查 MySQL 容器是否运行
    if $COMPOSE_CMD ps mysql | grep -q "running"; then
        $COMPOSE_CMD exec -T mysql mysqldump -u root -p"${MYSQL_ROOT_PASSWORD:-root123456}" kubepolaris > "$BACKUP_FILE" 2>/dev/null
        
        if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
            log_success "数据库备份完成: $BACKUP_FILE"
        else
            log_warn "数据库备份可能失败"
        fi
    else
        log_warn "MySQL 容器未运行，跳过数据库备份"
    fi
}

# 停止容器
stop_containers() {
    log_info "停止容器..."
    
    cd "$DEPLOY_DIR/docker-compose"
    $COMPOSE_CMD down
    
    log_success "容器已停止"
}

# 删除数据卷
remove_volumes() {
    log_info "删除数据卷..."
    
    # 获取所有相关卷
    VOLUMES=$(docker volume ls -q | grep -E "kubepolaris" || true)
    
    if [ -n "$VOLUMES" ]; then
        echo "$VOLUMES" | xargs docker volume rm
        log_success "数据卷已删除"
    else
        log_info "没有找到相关数据卷"
    fi
}

# 删除镜像
remove_images() {
    log_info "删除镜像..."
    
    # 删除 KubePolaris 相关镜像
    IMAGES=$(docker images --format "{{.Repository}}:{{.Tag}}" | grep -E "kubepolaris" || true)
    
    if [ -n "$IMAGES" ]; then
        echo "$IMAGES" | xargs docker rmi -f
        log_success "镜像已删除"
    else
        log_info "没有找到相关镜像"
    fi
}

# 删除网络
remove_networks() {
    log_info "删除网络..."
    
    NETWORKS=$(docker network ls -q --filter name=kubepolaris || true)
    
    if [ -n "$NETWORKS" ]; then
        echo "$NETWORKS" | xargs docker network rm 2>/dev/null || true
        log_success "网络已删除"
    else
        log_info "没有找到相关网络"
    fi
}

# 清理配置文件
cleanup_configs() {
    log_info "清理配置文件..."
    
    local ENV_FILE="$DEPLOY_DIR/docker-compose/.env"
    
    # 询问是否删除 .env 文件
    if [ -f "$ENV_FILE" ]; then
        read -p "是否删除 .env 配置文件？[y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -f "$ENV_FILE"
            log_success ".env 文件已删除"
        else
            log_info "保留 .env 文件"
        fi
    fi
    
    # 询问是否删除 Grafana secrets
    if [ -d "$DEPLOY_DIR/docker/grafana/secrets" ]; then
        read -p "是否删除 Grafana secrets？[y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$DEPLOY_DIR/docker/grafana/secrets"/*
            log_success "Grafana secrets 已删除"
        else
            log_info "保留 Grafana secrets"
        fi
    fi
}

# 显示卸载完成信息
show_complete_info() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║               ✅ 卸载完成！                                ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    if [ -d "$PROJECT_ROOT/backups" ] && [ "$(ls -A "$PROJECT_ROOT/backups" 2>/dev/null)" ]; then
        echo -e "${BLUE}备份文件保存在:${NC} $PROJECT_ROOT/backups/"
        ls -la "$PROJECT_ROOT/backups/"
        echo ""
    fi
    
    echo -e "${BLUE}如需重新安装，请运行:${NC}"
    echo -e "  ${YELLOW}$DEPLOY_DIR/scripts/install.sh${NC}"
    echo ""
}

# 主函数
main() {
    print_banner
    
    detect_compose_cmd
    
    # 警告确认
    echo -e "${RED}警告: 此操作将删除 KubePolaris 的所有容器、数据和配置！${NC}"
    echo ""
    read -p "是否确认卸载？请输入 'YES' 继续: " CONFIRM
    
    if [ "$CONFIRM" != "YES" ]; then
        log_info "卸载已取消"
        exit 0
    fi
    
    echo ""
    
    # 询问是否备份
    read -p "是否在卸载前备份数据？[Y/n] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
        backup_data
    fi
    
    # 执行卸载步骤
    stop_containers
    
    # 询问是否删除数据卷
    read -p "是否删除数据卷（包含数据库数据）？[y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        remove_volumes
    else
        log_info "保留数据卷"
    fi
    
    # 询问是否删除镜像
    read -p "是否删除 Docker 镜像？[y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        remove_images
    else
        log_info "保留镜像"
    fi
    
    remove_networks
    cleanup_configs
    
    show_complete_info
}

# 运行
main "$@"
