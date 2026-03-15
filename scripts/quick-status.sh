#!/bin/bash
# =============================================================================
# NAS-OS 快速状态检查脚本 v2.75.0
# =============================================================================
# 用途：一键查看系统关键状态，适合运维巡检
# 用法：./quick-status.sh [--json]
# =============================================================================

set -euo pipefail

VERSION="2.75.0"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# 配置
API_URL="${API_URL:-http://localhost:8080}"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
OUTPUT_JSON="${1:-}" = "--json" && OUTPUT_JSON=true || OUTPUT_JSON=false

# 状态收集
check_service() {
    local status="unknown"
    local pid=""
    
    if pgrep -x nasd > /dev/null 2>&1; then
        pid=$(pgrep -x nasd | head -1)
        status="running"
    else
        status="stopped"
    fi
    
    echo "$status:$pid"
}

check_api() {
    if curl -sf --max-time 3 "${API_URL}/api/v1/health" > /dev/null 2>&1; then
        echo "healthy"
    else
        echo "unhealthy"
    fi
}

check_disk() {
    local data_usage=""
    if [ -d "$DATA_DIR" ]; then
        data_usage=$(df -h "$DATA_DIR" 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
    else
        data_usage="N/A"
    fi
    
    local root_usage=$(df -h / 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
    
    echo "root=$root_usage data=$data_usage"
}

check_memory() {
    local mem_info=$(free -m 2>/dev/null | awk 'NR==2 {printf "total=%d used=%d free=%d", $2, $3, $7}')
    echo "$mem_info"
}

check_uptime() {
    uptime -p 2>/dev/null || uptime | awk -F'up ' '{print $2}' | awk -F',' '{print $1}'
}

# 主逻辑
main() {
    local service_info=$(check_service)
    local service_status=$(echo "$service_info" | cut -d: -f1)
    local service_pid=$(echo "$service_info" | cut -d: -f2)
    
    local api_status=$(check_api)
    local disk_info=$(check_disk)
    local mem_info=$(check_memory)
    local uptime_info=$(check_uptime)
    
    if [ "$OUTPUT_JSON" = "true" ]; then
        cat <<EOF
{
  "version": "$VERSION",
  "timestamp": "$(date -Iseconds 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z')",
  "service": {
    "status": "$service_status",
    "pid": "$service_pid"
  },
  "api": {
    "status": "$api_status",
    "url": "$API_URL"
  },
  "disk": {
    "root_usage": "$(echo "$disk_info" | grep -o 'root=[^ ]*' | cut -d= -f2)%",
    "data_usage": "$(echo "$disk_info" | grep -o 'data=[^ ]*' | cut -d= -f2)%"
  },
  "memory": {
    "total_mb": $(echo "$mem_info" | grep -o 'total=[0-9]*' | cut -d= -f2),
    "used_mb": $(echo "$mem_info" | grep -o 'used=[0-9]*' | cut -d= -f2),
    "free_mb": $(echo "$mem_info" | grep -o 'free=[0-9]*' | cut -d= -f2)
  },
  "uptime": "$uptime_info"
}
EOF
        exit 0
    fi
    
    # 文本输出
    echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}${CYAN}║       NAS-OS Quick Status v${VERSION}       ║${NC}"
    echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${NC}"
    echo ""
    
    # 服务状态
    echo -e "${BOLD}服务状态${NC}"
    echo -e "  nasd:  $([ "$service_status" = "running" ] && echo -e "${GREEN}● 运行中${NC} (PID: $service_pid)" || echo -e "${RED}○ 已停止${NC}")"
    echo -e "  API:   $([ "$api_status" = "healthy" ] && echo -e "${GREEN}● 健康${NC}" || echo -e "${RED}○ 异常${NC}")"
    echo ""
    
    # 系统资源
    echo -e "${BOLD}系统资源${NC}"
    local root_pct=$(echo "$disk_info" | grep -o 'root=[^ ]*' | cut -d= -f2)
    local data_pct=$(echo "$disk_info" | grep -o 'data=[^ ]*' | cut -d= -f2)
    
    echo -e "  磁盘 (/):     $root_pct%"
    echo -e "  磁盘 (data):  $data_pct%"
    
    local mem_total=$(echo "$mem_info" | grep -o 'total=[0-9]*' | cut -d= -f2)
    local mem_used=$(echo "$mem_info" | grep -o 'used=[0-9]*' | cut -d= -f2)
    local mem_free=$(echo "$mem_info" | grep -o 'free=[0-9]*' | cut -d= -f2)
    local mem_pct=$((mem_used * 100 / mem_total))
    
    echo -e "  内存:         ${mem_used}MB / ${mem_total}MB (${mem_pct}%)"
    echo ""
    
    # 运行时间
    echo -e "${BOLD}运行时间${NC}"
    echo -e "  $uptime_info"
    echo ""
    
    # 健康指示
    echo -e "${BOLD}健康状态${NC}"
    if [ "$service_status" = "running" ] && [ "$api_status" = "healthy" ]; then
        echo -e "  ${GREEN}✓ 系统运行正常${NC}"
    else
        echo -e "  ${YELLOW}⚠ 存在问题，请检查${NC}"
    fi
}

main "$@"