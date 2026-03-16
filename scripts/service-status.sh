#!/bin/bash
# =============================================================================
# NAS-OS 服务状态检查脚本 v1.0.0
# =============================================================================
# 用途：检查所有 NAS-OS 相关服务的运行状态
# 用法：./service-status.sh [--json] [--watch]
# =============================================================================

set -euo pipefail

VERSION="1.1.0"
SCRIPT_NAME=$(basename "$0")

# 配置
API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-8080}"
API_URL="http://${API_HOST}:${API_PORT}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 输出格式
OUTPUT_FORMAT="${OUTPUT_FORMAT:-text}"

# =============================================================================
# 服务定义
# =============================================================================

SERVICES=(
    "nas-os:NAS-OS 主服务:8080"
    "nas-prometheus:Prometheus 监控:9090"
    "nas-grafana:Grafana 可视化:3000"
    "nas-alertmanager:Alertmanager 告警:9093"
    "nas-node-exporter:Node Exporter:9100"
    "nas-cadvisor:cAdvisor:8081"
)

# =============================================================================
# 工具函数
# =============================================================================

get_timestamp() {
    date '+%Y-%m-%d %H:%M:%S'
}

check_systemd_service() {
    local service="$1"
    if systemctl is-active "$service" &>/dev/null; then
        return 0
    fi
    return 1
}

check_docker_container() {
    local container="$1"
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${container}$"; then
        return 0
    fi
    return 1
}

get_docker_container_status() {
    local container="$1"
    docker ps -a --filter "name=^${container}$" --format '{{.Status}}' 2>/dev/null || echo "unknown"
}

check_port() {
    local port="$1"
    if ss -tuln 2>/dev/null | grep -q ":${port} "; then
        return 0
    fi
    return 1
}

check_http() {
    local url="$1"
    local timeout="${2:-3}"
    if curl -sf -o /dev/null --max-time "$timeout" "$url" 2>/dev/null; then
        return 0
    fi
    return 1
}

# =============================================================================
# 检查函数
# =============================================================================

check_service() {
    local name="$1"
    local display="$2"
    local port="$3"
    
    local status="unknown"
    local details=""
    
    # 检查 Docker 容器
    if command -v docker &>/dev/null; then
        if check_docker_container "$name"; then
            status="running"
            details=$(get_docker_container_status "$name")
        elif docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^${name}$"; then
            status="stopped"
            details=$(get_docker_container_status "$name")
        fi
    fi
    
    # 检查 systemd 服务
    if [ "$status" = "unknown" ]; then
        if check_systemd_service "$name"; then
            status="running"
            details="systemd service active"
        elif systemctl is-enabled "$name" &>/dev/null 2>&1; then
            status="stopped"
            details="systemd service inactive"
        fi
    fi
    
    # 检查端口
    local port_status=""
    if [ -n "$port" ] && [ "$port" != "0" ]; then
        if check_port "$port"; then
            port_status="listening"
        else
            port_status="not-listening"
        fi
    fi
    
    # 输出
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        echo "{\"name\": \"$name\", \"display\": \"$display\", \"status\": \"$status\", \"port\": $port, \"port_status\": \"$port_status\", \"details\": \"$details\"}"
    else
        local color=""
        case "$status" in
            running) color="$GREEN" ;;
            stopped) color="$RED" ;;
            *)       color="$YELLOW" ;;
        esac
        
        printf "  %-20s ${color}%-10s${NC} Port: %-5s %-15s %s\n" \
            "$display" "[$status]" "$port" "[$port_status]" "$details"
    fi
}

check_api_endpoints() {
    echo ""
    echo -e "${BLUE}[API 端点检查]${NC}"
    
    local endpoints=(
        "health:$API_URL/api/v1/health"
        "ready:$API_URL/api/v1/ready"
        "metrics:$API_URL/api/v1/metrics"
        "status:$API_URL/api/v1/system/status"
    )
    
    for ep in "${endpoints[@]}"; do
        local name="${ep%%:*}"
        local url="${ep#*:}"
        
        if check_http "$url" 2; then
            echo -e "  ${GREEN}✓${NC} $name: $url"
        else
            echo -e "  ${RED}✗${NC} $name: $url"
        fi
    done
}

check_system_resources() {
    echo ""
    echo -e "${BLUE}[系统资源]${NC}"
    
    # CPU
    local cpu_usage=$(top -bn1 2>/dev/null | grep -E '^%?Cpu' | awk '{print $2}' | head -1 || echo "0")
    echo -e "  CPU 使用率: ${CYAN}${cpu_usage}%${NC}"
    
    # 内存
    local mem_info=$(free -m 2>/dev/null | awk '/^Mem:/{print $2,$3,$4}')
    local mem_total=$(echo "$mem_info" | awk '{print $1}')
    local mem_used=$(echo "$mem_info" | awk '{print $2}')
    local mem_avail=$(echo "$mem_info" | awk '{print $3}')
    local mem_pct=$((mem_used * 100 / mem_total))
    
    local mem_color="$GREEN"
    [ "$mem_pct" -gt 80 ] && mem_color="$RED"
    [ "$mem_pct" -gt 60 ] && mem_color="$YELLOW"
    
    echo -e "  内存: ${mem_color}${mem_used}MB / ${mem_total}MB (${mem_pct}%)${NC}"
    
    # 磁盘
    local disk_info=$(df -h / 2>/dev/null | awk 'NR==2 {print $2,$3,$4,$5}')
    local disk_total=$(echo "$disk_info" | awk '{print $1}')
    local disk_used=$(echo "$disk_info" | awk '{print $2}')
    local disk_avail=$(echo "$disk_info" | awk '{print $3}')
    local disk_pct=$(echo "$disk_info" | awk '{print $4}')
    
    local disk_color="$GREEN"
    [ "${disk_pct%\%}" -gt 80 ] && disk_color="$RED"
    [ "${disk_pct%\%}" -gt 60 ] && disk_color="$YELLOW"
    
    echo -e "  磁盘 (/): ${disk_color}${disk_used} / ${disk_total} (${disk_pct})${NC}"
    
    # 负载
    local load=$(cat /proc/loadavg 2>/dev/null | awk '{print $1,$2,$3}')
    echo -e "  系统负载: ${CYAN}${load}${NC}"
    
    # 网络连接统计 (v2.122.0)
    local net_connections=$(ss -s 2>/dev/null | grep -oP 'TCP:.*' | head -1 || echo "N/A")
    echo -e "  网络连接: ${CYAN}${net_connections}${NC}"
}

# 进程详细信息 (v2.122.0)
check_process_details() {
    echo ""
    echo -e "${BLUE}[进程详情]${NC}"
    
    # 检查 nasd 进程
    local nasd_pid=$(pgrep -x "nasd" 2>/dev/null | head -1)
    if [ -n "$nasd_pid" ]; then
        echo -e "  NAS-OS 进程 (${nasd_pid}):"
        
        # 内存使用
        local mem=$(ps -p "$nasd_pid" -o rss --no-headers 2>/dev/null | awk '{printf "%.1f MB", $1/1024}')
        echo -e "    内存: ${CYAN}${mem}${NC}"
        
        # CPU 使用
        local cpu=$(ps -p "$nasd_pid" -o %cpu --no-headers 2>/dev/null | tr -d ' ')
        echo -e "    CPU: ${CYAN}${cpu}%${NC}"
        
        # 运行时间
        local uptime=$(ps -p "$nasd_pid" -o etime --no-headers 2>/dev/null | tr -d ' ')
        echo -e "    运行时间: ${CYAN}${uptime}${NC}"
        
        # 打开的文件数
        local fds=$(ls /proc/"$nasd_pid"/fd 2>/dev/null | wc -l || echo "N/A")
        echo -e "    文件描述符: ${CYAN}${fds}${NC}"
        
        # 线程数
        local threads=$(cat /proc/"$nasd_pid"/status 2>/dev/null | grep Threads | awk '{print $2}' || echo "N/A")
        echo -e "    线程数: ${CYAN}${threads}${NC}"
    else
        echo -e "  ${YELLOW}NAS-OS 进程未运行${NC}"
    fi
}

# 端口详细信息 (v2.122.0)
check_port_details() {
    echo ""
    echo -e "${BLUE}[端口详情]${NC}"
    
    # 检查主要服务端口
    local ports="8080 9090 3000 9093"
    for port in $ports; do
        local listener=$(ss -tlnp 2>/dev/null | grep ":${port}" | awk '{print $5}' | cut -d: -f2 || echo "")
        local process=$(ss -tlnp 2>/dev/null | grep ":${port}" | awk '{print $7}' | cut -d, -f2 || echo "")
        
        if [ -n "$listener" ]; then
            echo -e "  端口 ${port}: ${GREEN}监听中${NC} (${process:-unknown})"
        else
            echo -e "  端口 ${port}: ${YELLOW}未使用${NC}"
        fi
    done
}

check_recent_logs() {
    echo ""
    echo -e "${BLUE}[最近日志]${NC}"
    
    # 尝试 journalctl
    if command -v journalctl &>/dev/null; then
        echo "  最近 5 条错误:"
        journalctl -u nas-os --no-pager -n 50 2>/dev/null | grep -i "error" | tail -5 | while read line; do
            echo -e "    ${RED}$line${NC}"
        done
        
        echo ""
        echo "  最近 5 条警告:"
        journalctl -u nas-os --no-pager -n 50 2>/dev/null | grep -i "warn" | tail -5 | while read line; do
            echo -e "    ${YELLOW}$line${NC}"
        done
    fi
}

# =============================================================================
# 主函数
# =============================================================================

run_check() {
    echo ""
    echo "==================================="
    echo "NAS-OS 服务状态检查 v$VERSION"
    echo "时间: $(get_timestamp)"
    echo "==================================="
    echo ""
    
    echo -e "${BLUE}[服务状态]${NC}"
    for svc in "${SERVICES[@]}"; do
        IFS=':' read -r name display port <<< "$svc"
        check_service "$name" "$display" "$port"
    done
    
    # 进程详情 (v2.122.0)
    check_process_details
    
    # 端口详情 (v2.122.0)
    check_port_details
    
    # 检查 API 端点（如果主服务运行中）
    if check_http "$API_URL/api/v1/health" 2; then
        check_api_endpoints
    fi
    
    # 系统资源
    check_system_resources
    
    # 最近日志（如果有错误）
    check_recent_logs
    
    echo ""
    echo "==================================="
}

watch_mode() {
    local interval="${1:-5}"
    echo "监控模式 (刷新间隔: ${interval}s, Ctrl+C 退出)"
    echo ""
    
    while true; do
        clear
        run_check
        sleep "$interval"
    done
}

show_help() {
    cat <<EOF
NAS-OS 服务状态检查脚本 v$VERSION

用法: $0 [选项]

选项:
  --json     JSON 格式输出
  --watch    持续监控模式
  --help     显示帮助

环境变量:
  API_HOST   API 主机 (默认: localhost)
  API_PORT   API 端口 (默认: 8080)

示例:
  $0              # 检查服务状态
  $0 --json       # JSON 输出
  $0 --watch      # 持续监控

EOF
}

# =============================================================================
# 主入口
# =============================================================================

main() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json)
                OUTPUT_FORMAT="json"
                shift
                ;;
            --watch)
                watch_mode "${2:-5}"
                exit 0
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                shift
                ;;
        esac
    done
    
    run_check
}

main "$@"