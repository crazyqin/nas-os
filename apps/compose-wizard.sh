#!/bin/bash
# ╔══════════════════════════════════════════════════════════════════════════════╗
# ║                    NAS-OS Compose YAML 向导                                  ║
# ║                         快速生成部署配置                                      ║
# ╚══════════════════════════════════════════════════════════════════════════════╝
#
# 用法: ./compose-wizard.sh [选项]
#
# v2.387.0 工部创建

set -e

TEMPLATE_DIR="$(dirname "$0")/templates"
OUTPUT_DIR="$(dirname "$0")/generated"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_header() {
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║           NAS-OS Compose YAML 向导                           ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${YELLOW}ℹ${NC} $1"
}

# 显示可用应用
list_apps() {
    echo -e "\n${YELLOW}可用应用模板:${NC}\n"
    
    echo -e "${BLUE}媒体服务:${NC}"
    echo "  plex       - Plex 媒体服务器"
    echo "  jellyfin   - Jellyfin 开源媒体服务器"
    echo "  homeassistant - 智能家居平台"
    
    echo -e "\n${BLUE}存储服务:${NC}"
    echo "  nextcloud  - 私有云盘"
    echo "  vaultwarden - 密码管理器"
    
    echo -e "\n${BLUE}数据库:${NC}"
    echo "  postgres   - PostgreSQL 数据库"
    echo "  redis      - Redis 缓存"
    echo "  qdrant     - 向量数据库"
    
    echo -e "\n${BLUE}网络服务:${NC}"
    echo "  nginx      - Web 服务器/反向代理"
    echo "  portainer  - Docker 管理 UI"
    echo "  transmission - BT 下载客户端"
    
    echo ""
}

# 显示推荐组合
list_stacks() {
    echo -e "\n${YELLOW}推荐应用组合:${NC}\n"
    
    echo -e "${BLUE}家庭影院:${NC}"
    echo "  plex + transmission"
    echo "  资源需求: CPU 2.5核, 内存 1.2G"
    
    echo -e "\n${BLUE}私有云盘:${NC}"
    echo "  nextcloud + postgres + redis"
    echo "  资源需求: CPU 2.5核, 内存 1.5G"
    
    echo -e "\n${BLUE}智能家居:${NC}"
    echo "  homeassistant + postgres"
    echo "  资源需求: CPU 2核, 内存 1G"
    
    echo ""
}

# 生成单应用配置
generate_single() {
    local app="$1"
    local template="$TEMPLATE_DIR/${app}.yml"
    
    if [[ ! -f "$template" ]]; then
        print_error "应用 '$app' 模板不存在"
        return 1
    fi
    
    mkdir -p "$OUTPUT_DIR"
    local output="$OUTPUT_DIR/${app}-deploy.yml"
    
    cp "$template" "$output"
    print_success "生成配置: $output"
    print_info "启动命令: docker compose -f $output up -d"
}

# 生成组合配置
generate_stack() {
    local stack="$1"
    mkdir -p "$OUTPUT_DIR"
    local output="$OUTPUT_DIR/${stack}-stack.yml"
    
    case "$stack" in
        "media_center"|"home theater"|"家庭影院")
            cat > "$output" << 'EOF'
# NAS-OS 家庭影院组合配置
# 自动生成 - v2.387.0

include:
  - templates/plex.yml
  - templates/transmission.yml
EOF
            print_success "生成家庭影院配置: $output"
            ;;
        "private_cloud"|"云盘"|"私有云")
            cat > "$output" << 'EOF'
# NAS-OS 私有云盘组合配置
# 自动生成 - v2.387.0

include:
  - templates/nextcloud.yml
  - templates/postgres.yml
  - templates/redis.yml
EOF
            print_success "生成私有云盘配置: $output"
            ;;
        "smart_home"|"智能家居")
            cat > "$output" << 'EOF'
# NAS-OS 智能家居组合配置
# 自动生成 - v2.387.0

include:
  - templates/homeassistant.yml
  - templates/postgres.yml
EOF
            print_success "生成智能家居配置: $output"
            ;;
        *)
            print_error "未知组合: $stack"
            return 1
            ;;
    esac
    
    print_info "启动命令: docker compose -f $output up -d"
}

# 交互式向导
interactive_wizard() {
    print_header
    
    echo -e "\n${YELLOW}请选择部署类型:${NC}"
    echo "  1) 单应用部署"
    echo "  2) 应用组合部署"
    echo "  3) 查看可用应用"
    echo "  4) 查看推荐组合"
    echo "  q) 退出"
    
    read -p "选择 [1-4/q]: " choice
    
    case "$choice" in
        1)
            list_apps
            read -p "输入应用名称: " app
            generate_single "$app"
            ;;
        2)
            list_stacks
            read -p "输入组合名称: " stack
            generate_stack "$stack"
            ;;
        3)
            list_apps
            ;;
        4)
            list_stacks
            ;;
        q|Q)
            echo "退出"
            exit 0
            ;;
        *)
            print_error "无效选择"
            exit 1
            ;;
    esac
}

# 显示帮助
show_help() {
    print_header
    echo "
用法: $0 [命令] [参数]

命令:
  list              显示可用应用和组合
  generate <app>    生成单应用配置
  stack <name>      生成组合配置
  wizard            启动交互式向导
  help              显示此帮助

示例:
  $0 list
  $0 generate plex
  $0 stack media_center
  $0 wizard
"
}

# 主入口
main() {
    local cmd="${1:-wizard}"
    
    case "$cmd" in
        "list"|"ls")
            print_header
            list_apps
            list_stacks
            ;;
        "generate"|"gen"|"g")
            local app="${2:-}"
            if [[ -z "$app" ]]; then
                print_error "请指定应用名称"
                list_apps
                exit 1
            fi
            generate_single "$app"
            ;;
        "stack"|"s")
            local stack="${2:-}"
            if [[ -z "$stack" ]]; then
                print_error "请指定组合名称"
                list_stacks
                exit 1
            fi
            generate_stack "$stack"
            ;;
        "wizard"|"w")
            interactive_wizard
            ;;
        "help"|"h"|"-h"|"--help")
            show_help
            ;;
        *)
            print_error "未知命令: $cmd"
            show_help
            exit 1
            ;;
    esac
}

main "$@"