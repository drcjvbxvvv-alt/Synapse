#!/bin/bash
# ============================================================================
# KubePolaris Helm Chart 快速部署脚本
# ============================================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认值
NAMESPACE="${NAMESPACE:-kubepolaris}"
RELEASE_NAME="${RELEASE_NAME:-kubepolaris}"
CHART_PATH="$(cd "$(dirname "$0")" && pwd)"

# 日志函数
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
    echo -e "${BLUE}║   ${GREEN}██╗  ██╗██╗   ██╗██████╗ ███████╗██████╗  ██████╗ ${BLUE}    ║${NC}"
    echo -e "${BLUE}║   ${GREEN}██║ ██╔╝██║   ██║██╔══██╗██╔════╝██╔══██╗██╔═══██╗${BLUE}   ║${NC}"
    echo -e "${BLUE}║   ${GREEN}█████╔╝ ██║   ██║██████╔╝█████╗  ██████╔╝██║   ██║${BLUE}   ║${NC}"
    echo -e "${BLUE}║   ${GREEN}██╔═██╗ ██║   ██║██╔══██╗██╔══╝  ██╔═══╝ ██║   ██║${BLUE}   ║${NC}"
    echo -e "${BLUE}║   ${GREEN}██║  ██╗╚██████╔╝██████╔╝███████╗██║     ╚██████╔╝${BLUE}   ║${NC}"
    echo -e "${BLUE}║   ${GREEN}╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚══════╝╚═╝      ╚═════╝ ${BLUE}   ║${NC}"
    echo -e "${BLUE}║                                                           ║${NC}"
    echo -e "${BLUE}║       ${NC}KubePolaris Helm Chart Quick Deploy${BLUE}                ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    # 检查 kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl 未安装，请先安装 kubectl"
        exit 1
    fi
    log_success "kubectl 已安装: $(kubectl version --client -o json | grep gitVersion | head -1 | cut -d'"' -f4)"
    
    # 检查 helm
    if ! command -v helm &> /dev/null; then
        log_error "helm 未安装，请先安装 Helm 3.0+"
        exit 1
    fi
    log_success "Helm 已安装: $(helm version --short)"
    
    # 检查 Kubernetes 连接
    if ! kubectl cluster-info &> /dev/null; then
        log_error "无法连接到 Kubernetes 集群"
        exit 1
    fi
    log_success "Kubernetes 集群连接正常"
}

# 生成密钥
generate_secrets() {
    log_info "生成安全密钥..."
    
    if [ -z "$JWT_SECRET" ]; then
        JWT_SECRET=$(openssl rand -base64 32)
        log_success "JWT Secret 已生成"
    fi
    
    if [ -z "$MYSQL_ROOT_PASSWORD" ]; then
        MYSQL_ROOT_PASSWORD=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)
        log_success "MySQL Root Password 已生成"
    fi
    
    if [ -z "$MYSQL_PASSWORD" ]; then
        MYSQL_PASSWORD=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)
        log_success "MySQL Password 已生成"
    fi
}

# 创建命名空间
create_namespace() {
    log_info "创建命名空间 ${NAMESPACE}..."
    
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_warn "命名空间 ${NAMESPACE} 已存在"
    else
        kubectl create namespace "$NAMESPACE"
        log_success "命名空间 ${NAMESPACE} 创建成功"
    fi
}

# 部署应用
deploy_app() {
    log_info "部署 KubePolaris..."
    
    helm upgrade --install "$RELEASE_NAME" "$CHART_PATH" \
        --namespace "$NAMESPACE" \
        --set security.jwtSecret="$JWT_SECRET" \
        --set mysql.internal.rootPassword="$MYSQL_ROOT_PASSWORD" \
        --set mysql.internal.password="$MYSQL_PASSWORD" \
        --wait \
        --timeout 10m
    
    log_success "KubePolaris 部署完成"
}

# 等待服务就绪
wait_for_ready() {
    log_info "等待服务就绪..."
    
    kubectl wait --for=condition=ready pod \
        -l app.kubernetes.io/instance="$RELEASE_NAME" \
        -n "$NAMESPACE" \
        --timeout=300s || {
        log_warn "部分 Pod 可能仍在启动中"
    }
    
    log_success "服务已就绪"
}

# 显示访问信息
show_access_info() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║               🎉 部署成功！                                ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BLUE}访问方式:${NC}"
    echo "  kubectl port-forward -n $NAMESPACE svc/${RELEASE_NAME}-frontend 8080:80"
    echo "  访问: http://localhost:8080"
    echo ""
    echo -e "${BLUE}默认登录信息:${NC}"
    echo "  用户名: admin"
    echo "  密码: KubePolaris@2026"
    echo ""
    echo -e "${YELLOW}⚠️  首次登录后请立即修改默认密码！${NC}"
    echo ""
    echo -e "${BLUE}查看服务状态:${NC}"
    echo "  kubectl get pods -n $NAMESPACE"
    echo ""
    echo -e "${BLUE}查看日志:${NC}"
    echo "  kubectl logs -f -l app.kubernetes.io/component=backend -n $NAMESPACE"
    echo ""
    echo -e "${BLUE}卸载:${NC}"
    echo "  helm uninstall $RELEASE_NAME -n $NAMESPACE"
    echo ""
}

# 主函数
main() {
    print_banner
    
    check_dependencies
    generate_secrets
    create_namespace
    deploy_app
    wait_for_ready
    show_access_info
}

# 运行
main "$@"
