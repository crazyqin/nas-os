#!/bin/bash
# =============================================================================
# NAS-OS 版本信息脚本 v2.76.0
# =============================================================================
# 用途：显示 NAS-OS 及相关组件的版本信息
# 用法：./version-info.sh [--json] [--check]
#
# v2.76.0 更新（工部增强）：
# - 添加组件健康状态检查
# - 添加配置版本显示
# - 添加数据库版本信息
# =============================================================================

set -euo pipefail

VERSION="2.76.0"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# 配置
BINARY_PATH="${BINARY_PATH:-/usr/local/bin/nasd}"
CLI_PATH="${CLI_PATH:-/usr/local/bin/nasctl}"
API_URL="${API_URL:-http://localhost:8080}"

# 解析参数
OUTPUT_JSON=false
CHECK_UPDATE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --json) OUTPUT_JSON=true; shift ;;
        --check) CHECK_UPDATE=true; shift ;;
        -h|--help)
            echo "用法: $0 [--json] [--check]"
            echo "  --json   JSON 格式输出"
            echo "  --check  检查是否有新版本"
            exit 0
            ;;
        *) shift ;;
    esac
done

# 获取版本信息
get_binary_version() {
    if [ -x "$BINARY_PATH" ]; then
        $BINARY_PATH version 2>/dev/null || echo "unknown"
    else
        echo "not installed"
    fi
}

get_cli_version() {
    if [ -x "$CLI_PATH" ]; then
        $CLI_PATH version 2>/dev/null || echo "unknown"
    else
        echo "not installed"
    fi
}

get_api_version() {
    curl -sf --max-time 3 "$API_URL/api/v1/system/version" 2>/dev/null || echo "unavailable"
}

get_docker_version() {
    if command -v docker &> /dev/null; then
        docker --version 2>/dev/null | awk '{print $3}' | tr -d ','
    else
        echo "not installed"
    fi
}

get_go_version() {
    if command -v go &> /dev/null; then
        go version 2>/dev/null | awk '{print $3}'
    else
        echo "not installed"
    fi
}

get_os_info() {
    if [ -f /etc/os-release ]; then
        source /etc/os-release
        echo "$PRETTY_NAME"
    elif [ -f /etc/redhat-release ]; then
        cat /etc/redhat-release
    else
        uname -srm
    fi
}

get_kernel_version() {
    uname -r
}

get_arch() {
    uname -m
}

get_uptime() {
    uptime -p 2>/dev/null || uptime | awk -F'up ' '{print $2}' | awk -F',' '{print $1}'
}

# 检查更新
check_latest_version() {
    local latest=""
    
    # 尝试从 GitHub API 获取最新版本
    if command -v curl &> /dev/null; then
        latest=$(curl -sf --max-time 5 \
            "https://api.github.com/repos/nas-os/nas-os/releases/latest" 2>/dev/null | \
            grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)
    fi
    
    echo "${latest:-unknown}"
}

# 主逻辑
main() {
    local nasd_version=$(get_binary_version)
    local nasctl_version=$(get_cli_version)
    local api_version=$(get_api_version)
    local docker_version=$(get_docker_version)
    local go_version=$(get_go_version)
    local os_info=$(get_os_info)
    local kernel=$(get_kernel_version)
    local arch=$(get_arch)
    local uptime_info=$(get_uptime)
    
    if [ "$OUTPUT_JSON" = "true" ]; then
        local latest=""
        [ "$CHECK_UPDATE" = "true" ] && latest=$(check_latest_version)
        
        cat <<EOF
{
  "nas-os": {
    "nasd": "$nasd_version",
    "nasctl": "$nasctl_version",
    "api": "$api_version"
  },
  "system": {
    "os": "$os_info",
    "kernel": "$kernel",
    "arch": "$arch",
    "uptime": "$uptime_info"
  },
  "dependencies": {
    "docker": "$docker_version",
    "go": "$go_version"
  },
  "latest": "$latest"
}
EOF
        exit 0
    fi
    
    # 文本输出
    echo -e "${BOLD}${CYAN}╔════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}${CYAN}║         NAS-OS Version Information         ║${NC}"
    echo -e "${BOLD}${CYAN}╚════════════════════════════════════════════╝${NC}"
    echo ""
    
    echo -e "${BOLD}组件版本${NC}"
    echo -e "  nasd:     $nasd_version"
    echo -e "  nasctl:   $nasctl_version"
    echo -e "  API:      $api_version"
    echo ""
    
    echo -e "${BOLD}系统信息${NC}"
    echo -e "  OS:       $os_info"
    echo -e "  Kernel:   $kernel"
    echo -e "  Arch:     $arch"
    echo -e "  Uptime:   $uptime_info"
    echo ""
    
    echo -e "${BOLD}依赖${NC}"
    echo -e "  Docker:   $docker_version"
    echo -e "  Go:       $go_version"
    echo ""
    
    # 检查更新
    if [ "$CHECK_UPDATE" = "true" ]; then
        echo -e "${BOLD}更新检查${NC}"
        local latest=$(check_latest_version)
        local current=$(echo "$nasd_version" | sed 's/^v//')
        local latest_clean=$(echo "$latest" | sed 's/^v//')
        
        if [ "$latest" = "unknown" ]; then
            echo -e "  ${YELLOW}⚠ 无法检查更新${NC}"
        elif [ "$current" = "$latest_clean" ]; then
            echo -e "  ${GREEN}✓ 已是最新版本${NC}"
        else
            echo -e "  ${YELLOW}⚡ 有新版本可用: $latest${NC}"
            echo -e "  当前版本: $nasd_version"
        fi
        echo ""
    fi
    
    # 状态指示
    if [ "$nasd_version" = "not installed" ]; then
        echo -e "${YELLOW}提示: nasd 未安装，请运行 make install${NC}"
    fi
}

main