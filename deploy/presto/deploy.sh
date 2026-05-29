#!/bin/bash
# NAS-OS Presto 高速传输部署脚本
# 版本: v2.400.0
#
# 使用方式:
#   ./deploy.sh --install    # 安装 Presto
#   ./deploy.sh --upgrade    # 升级 Presto
#   ./deploy.sh --uninstall  # 卸载 Presto
#   ./deploy.sh --status     # 查看状态
#   ./deploy.sh --logs       # 查看日志

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 检查依赖
check_dependencies() {
    log_step "检查依赖..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose 未安装，请先安装 Docker Compose"
        exit 1
    fi
    
    log_info "依赖检查通过"
}

# 创建必要目录
create_directories() {
    log_step "创建必要目录..."
    
    mkdir -p ./configs
    mkdir -p ./logs
    mkdir -p ./monitoring/grafana/provisioning/datasources
    mkdir -p ./monitoring/grafana/provisioning/dashboards
    
    # 创建数据目录（需要 sudo）
    if [ ! -d "/var/lib/presto" ]; then
        log_info "创建 Presto 数据目录..."
        sudo mkdir -p /var/lib/presto
        sudo chown -R 1000:1000 /var/lib/presto
    fi
    
    log_info "目录创建完成"
}

# 复制配置文件
copy_configs() {
    log_step "复制配置文件..."
    
    if [ ! -f ".env" ]; then
        if [ -f ".env.example" ]; then
            cp .env.example .env
            log_info "已创建 .env 配置文件，请根据实际环境修改"
        fi
    else
        log_info ".env 配置文件已存在"
    fi
    
    if [ ! -f "./configs/presto.yaml" ]; then
        if [ -f "./presto.yaml" ]; then
            cp ./presto.yaml ./configs/presto.yaml
            log_info "已复制 Presto 配置文件"
        fi
    else
        log_info "Presto 配置文件已存在"
    fi
    
    log_info "配置文件准备完成"
}

# 安装 Presto
install() {
    log_info "=========================================="
    log_info "  NAS-OS Presto 高速传输部署"
    log_info "=========================================="
    
    check_dependencies
    create_directories
    copy_configs
    
    log_step "拉取镜像..."
    docker compose -f docker-compose.presto.yml pull
    
    log_step "启动服务..."
    docker compose -f docker-compose.presto.yml up -d
    
    log_info "=========================================="
    log_info "  部署完成！"
    log_info "=========================================="
    echo ""
    log_info "访问地址:"
    log_info "  - Presto API: http://localhost:8090"
    log_info "  - Prometheus: http://localhost:9091"
    log_info "  - Grafana:    http://localhost:3001"
    echo ""
    log_info "默认 Grafana 账号:"
    log_info "  - 用户名: admin"
    log_info "  - 密码:   presto123"
    echo ""
    log_info "常用命令:"
    log_info "  - 查看状态: docker compose -f docker-compose.presto.yml ps"
    log_info "  - 查看日志: docker compose -f docker-compose.presto.yml logs -f"
    log_info "  - 停止服务: docker compose -f docker-compose.presto.yml down"
}

# 升级 Presto
upgrade() {
    log_info "=========================================="
    log_info "  升级 NAS-OS Presto"
    log_info "=========================================="
    
    check_dependencies
    
    log_step "备份当前配置..."
    cp .env .env.backup.$(date +%Y%m%d%H%M%S) 2>/dev/null || true
    
    log_step "拉取最新镜像..."
    docker compose -f docker-compose.presto.yml pull
    
    log_step "重启服务..."
    docker compose -f docker-compose.presto.yml up -d
    
    log_info "升级完成！"
}

# 卸载 Presto
uninstall() {
    log_info "=========================================="
    log_info "  卸载 NAS-OS Presto"
    log_info "=========================================="
    
    read -p "确定要卸载 Presto 吗？(y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "取消卸载"
        exit 0
    fi
    
    log_step "停止服务..."
    docker compose -f docker-compose.presto.yml down
    
    log_step "清理数据..."
    read -p "是否删除数据目录？(y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker volume rm presto-data 2>/dev/null || true
        sudo rm -rf /var/lib/presto
        log_info "数据目录已删除"
    fi
    
    log_info "卸载完成！"
}

# 查看状态
status() {
    log_info "Presto 服务状态:"
    docker compose -f docker-compose.presto.yml ps
    
    echo ""
    log_info "资源使用:"
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" nas-presto nas-presto-prometheus nas-presto-grafana nas-presto-alertmanager 2>/dev/null || true
}

# 查看日志
logs() {
    docker compose -f docker-compose.presto.yml logs -f
}

# 主函数
main() {
    cd "$(dirname "$0")"
    
    case "${1:-}" in
        --install|-i)
            install
            ;;
        --upgrade|-u)
            upgrade
            ;;
        --uninstall|--remove)
            uninstall
            ;;
        --status|-s)
            status
            ;;
        --logs|-l)
            logs
            ;;
        *)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --install, -i      安装 Presto"
            echo "  --upgrade, -u      升级 Presto"
            echo "  --uninstall        卸载 Presto"
            echo "  --status, -s       查看状态"
            echo "  --logs, -l         查看日志"
            echo ""
            exit 1
            ;;
    esac
}

main "$@"
