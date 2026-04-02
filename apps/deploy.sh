#!/bin/bash
# NAS-OS Docker 应用部署脚本
# 对标 TrueNAS Docker Apps 简化部署体验
#
# 使用方式：
#   ./apps/deploy.sh <app-name> [options]
#
# 选项：
#   --list        列出所有可用应用
#   --info        显示应用详细信息
#   --dry-run     仅显示部署命令，不执行
#   --stack       部署推荐组合（如 media_center）
#
# v2.374.0 工部优化：简化应用部署流程

set -e

# ==================== 配置 ====================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATES_DIR="${SCRIPT_DIR}/templates"
CONFIG_DIR="${NAS_CONFIG_DIR:-/var/lib/nas-os/config}"
DATA_DIR="${NAS_DATA_DIR:-/var/lib/nas-os/data}"
LOG_DIR="${NAS_LOG_DIR:-/var/lib/nas-os/logs/apps}"
NETWORK_NAME="nas-app-network"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ==================== 帮助信息 ====================
show_help() {
    echo "NAS-OS Docker 应用部署工具 v2.374.0"
    echo ""
    echo "使用方式:"
    echo "  $0 <app-name>              部署指定应用"
    echo "  $0 --list                  列出所有可用应用"
    echo "  $0 --info <app-name>       显示应用详细信息"
    echo "  $0 --stack <stack-name>    部署推荐组合"
    echo "  $0 --dry-run <app-name>    仅显示部署命令"
    echo ""
    echo "示例:"
    echo "  $0 plex                    部署 Plex 媒体服务器"
    echo "  $0 --list                  列出所有应用"
    echo "  $0 --stack media_center    部署家庭影院组合"
    echo ""
    echo "应用模板目录: $TEMPLATES_DIR"
}

# ==================== 列出应用 ====================
list_apps() {
    echo -e "${BLUE}NAS-OS 可用应用列表${NC}"
    echo ""
    
    # 从 index.yml 解析分类和应用
    if [ -f "$TEMPLATES_DIR/index.yml" ]; then
        echo -e "${GREEN}媒体服务:${NC}"
        echo "  • plex        - Plex 媒体服务器"
        echo "  • jellyfin    - Jellyfin 开源媒体服务器"
        echo "  • homeassistant - Home Assistant 智能家居"
        echo ""
        
        echo -e "${GREEN}存储服务:${NC}"
        echo "  • nextcloud   - 私有云盘"
        echo "  • vaultwarden - 密码管理器"
        echo ""
        
        echo -e "${GREEN}数据库:${NC}"
        echo "  • postgres    - PostgreSQL 数据库"
        echo "  • redis       - Redis 缓存"
        echo "  • qdrant      - Qdrant 向量数据库"
        echo ""
        
        echo -e "${GREEN}网络服务:${NC}"
        echo "  • nginx       - Nginx 反向代理"
        echo "  • portainer   - Docker 管理界面"
        echo "  • transmission - BT 下载客户端"
        echo ""
        
        echo -e "${YELLOW}推荐组合:${NC}"
        echo "  • media_center    - 家庭影院（plex + transmission）"
        echo "  • private_cloud   - 私有云盘（nextcloud + postgres + redis）"
        echo "  • smart_home      - 智能家居（homeassistant + postgres）"
        echo "  • password_manager - 密码管理（vaultwarden）"
        echo "  • ai_stack        - AI 服务（qdrant + redis）"
    else
        echo "⚠️ index.yml 不存在，列出模板目录:"
        ls -1 "$TEMPLATES_DIR"/*.yml 2>/dev/null | grep -v "_template" | sed 's/.*\///' | sed 's/.yml$//' | while read app; do
            echo "  • $app"
        done
    fi
}

# ==================== 应用详情 ====================
show_info() {
    local app="$1"
    local template="$TEMPLATES_DIR/${app}.yml"
    
    if [ ! -f "$template" ]; then
        echo -e "${RED}❌ 应用 '$app' 不存在${NC}"
        echo "可用应用: $(ls -1 "$TEMPLATES_DIR"/*.yml 2>/dev/null | grep -v "_template" | sed 's/.*\///' | sed 's/.yml$//' | tr '\n' ' ')"
        exit 1
    fi
    
    echo -e "${BLUE}应用信息: $app${NC}"
    echo ""
    
    # 解析模板基本信息
    echo -e "${GREEN}模板文件:${NC} $template"
    echo -e "${GREEN}端口配置:${NC}"
    grep -E "^\s*-[[:space:]]*[\"']?[0-9]+:" "$template" | sed 's/.*-\s*//' | while read port; do
        echo "  $port"
    done
    
    echo ""
    echo -e "${GREEN}资源需求:${NC}"
    grep -E "(cpus|memory)" "$template" | head -4 | while read line; do
        echo "  $line"
    done
    
    echo ""
    echo -e "${GREEN}健康检查:${NC}"
    grep -A3 "healthcheck:" "$template" | head -5
}

# ==================== 创建网络 ====================
ensure_network() {
    if ! docker network ls | grep -q "$NETWORK_NAME"; then
        echo -e "${YELLOW}创建应用网络: $NETWORK_NAME${NC}"
        docker network create \
            --driver bridge \
            --subnet 172.30.0.0/16 \
            --label "nas-os.network=apps" \
            "$NETWORK_NAME"
    else
        echo -e "${GREEN}✓ 网络 $NETWORK_NAME 已存在${NC}"
    fi
}

# ==================== 创建目录 ====================
ensure_dirs() {
    local app="$1"
    
    echo -e "${YELLOW}创建应用目录${NC}"
    
    mkdir -p "$CONFIG_DIR/$app"
    mkdir -p "$DATA_DIR/$app"
    mkdir -p "$LOG_DIR/$app"
    
    echo "  ✓ 配置目录: $CONFIG_DIR/$app"
    echo "  ✓ 数据目录: $DATA_DIR/$app"
    echo "  ✓ 日志目录: $LOG_DIR/$app"
}

# ==================== 部署应用 ====================
deploy_app() {
    local app="$1"
    local dry_run="${2:-false}"
    local template="$TEMPLATES_DIR/${app}.yml"
    
    if [ ! -f "$template" ]; then
        echo -e "${RED}❌ 应用 '$app' 不存在${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}部署应用: $app${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    
    # 确保网络存在
    ensure_network
    
    # 创建应用目录
    ensure_dirs "$app"
    
    # 设置环境变量
    export NAS_CONFIG_DIR="$CONFIG_DIR"
    export NAS_DATA_DIR="$DATA_DIR"
    export NAS_LOG_DIR="$LOG_DIR"
    
    # 显示部署信息
    echo ""
    echo -e "${GREEN}模板文件:${NC} $template"
    echo -e "${GREEN}配置目录:${NC} $CONFIG_DIR/$app"
    echo -e "${GREEN}数据目录:${NC} $DATA_DIR/$app"
    echo -e "${GREEN}日志目录:${NC} $LOG_DIR/$app"
    echo ""
    
    if [ "$dry_run" = "true" ]; then
        echo -e "${YELLOW}[DRY-RUN] 部署命令:${NC}"
        echo "docker compose -f $template up -d"
        echo ""
        echo -e "${YELLOW}[DRY-RUN] 查看状态:${NC}"
        echo "docker compose -f $template ps"
        echo ""
        echo -e "${YELLOW}[DRY-RUN] 查看日志:${NC}"
        echo "docker compose -f $template logs -f"
    else
        echo -e "${GREEN}开始部署...${NC}"
        docker compose -f "$template" up -d
        
        echo ""
        echo -e "${GREEN}✅ 部署完成${NC}"
        echo ""
        echo "查看状态: docker compose -f $template ps"
        echo "查看日志: docker compose -f $template logs -f"
        echo "停止服务: docker compose -f $template down"
        echo "更新服务: docker compose -f $template pull && docker compose -f $template up -d"
    fi
}

# ==================== 部署组合 ====================
deploy_stack() {
    local stack="$1"
    local dry_run="${2:-false}"
    
    echo -e "${BLUE}部署组合: $stack${NC}"
    echo ""
    
    case "$stack" in
        media_center)
            echo "部署家庭影院组合: Plex + Transmission"
            deploy_app "plex" "$dry_run"
            deploy_app "transmission" "$dry_run"
            ;;
        private_cloud)
            echo "部署私有云盘组合: Nextcloud + PostgreSQL + Redis"
            deploy_app "postgres" "$dry_run"
            deploy_app "redis" "$dry_run"
            sleep 5  # 等待数据库启动
            deploy_app "nextcloud" "$dry_run"
            ;;
        smart_home)
            echo "部署智能家居组合: Home Assistant + PostgreSQL"
            deploy_app "postgres" "$dry_run"
            sleep 5
            deploy_app "homeassistant" "$dry_run"
            ;;
        password_manager)
            echo "部署密码管理: Vaultwarden"
            deploy_app "vaultwarden" "$dry_run"
            ;;
        ai_stack)
            echo "部署 AI 服务栈: Qdrant + Redis"
            deploy_app "redis" "$dry_run"
            deploy_app "qdrant" "$dry_run"
            ;;
        *)
            echo -e "${RED}❌ 组合 '$stack' 不存在${NC}"
            echo "可用组合: media_center, private_cloud, smart_home, password_manager, ai_stack"
            exit 1
            ;;
    esac
}

# ==================== 主逻辑 ====================
main() {
    # 检查 Docker
    if ! command -v docker &>/dev/null; then
        echo -e "${RED}❌ Docker 未安装${NC}"
        exit 1
    fi
    
    # 检查 Docker Compose
    if ! docker compose version &>/dev/null; then
        echo -e "${RED}❌ Docker Compose 未安装${NC}"
        exit 1
    fi
    
    # 解析参数
    case "${1:-}" in
        -h|--help)
            show_help
            exit 0
            ;;
        -l|--list)
            list_apps
            exit 0
            ;;
        -i|--info)
            if [ -z "${2:-}" ]; then
                echo -e "${RED}❌ 请指定应用名称${NC}"
                exit 1
            fi
            show_info "$2"
            exit 0
            ;;
        -d|--dry-run)
            if [ -z "${2:-}" ]; then
                echo -e "${RED}❌ 请指定应用名称${NC}"
                exit 1
            fi
            deploy_app "$2" "true"
            exit 0
            ;;
        -s|--stack)
            if [ -z "${2:-}" ]; then
                echo -e "${RED}❌ 请指定组合名称${NC}"
                exit 1
            fi
            deploy_stack "$2" "${3:-false}"
            exit 0
            ;;
        "")
            show_help
            exit 0
            ;;
        *)
            deploy_app "$1" "false"
            exit 0
            ;;
    esac
}

main "$@"