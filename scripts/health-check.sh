#!/bin/bash
# =============================================================================
# NAS-OS 健康检查脚本 v2.56.0
# =============================================================================
# 用途：检查服务状态、磁盘空间、内存使用，输出 JSON 格式结果
# 用法：./health-check.sh [--json] [--quick] [--threshold-file FILE]
# =============================================================================

set -euo pipefail

# 版本
VERSION="2.56.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 默认配置（可通过阈值文件覆盖）
# =============================================================================

# 服务配置
API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-8080}"
API_URL="http://${API_HOST}:${API_PORT}"

# 磁盘阈值（百分比）
DISK_THRESHOLD_WARNING="${DISK_THRESHOLD_WARNING:-80}"
DISK_THRESHOLD_CRITICAL="${DISK_THRESHOLD_CRITICAL:-90}"

# 内存阈值（百分比）
MEMORY_THRESHOLD_WARNING="${MEMORY_THRESHOLD_WARNING:-80}"
MEMORY_THRESHOLD_CRITICAL="${MEMORY_THRESHOLD_CRITICAL:-90}"

# CPU 阈值（百分比）
CPU_THRESHOLD_WARNING="${CPU_THRESHOLD_WARNING:-80}"
CPU_THRESHOLD_CRITICAL="${CPU_THRESHOLD_CRITICAL:-95}"

# API 响应超时（毫秒）
API_TIMEOUT_MS="${API_TIMEOUT_MS:-5000}"

# 数据目录
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"

# 日志目录
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 阈值配置文件
THRESHOLD_FILE="${THRESHOLD_FILE:-/etc/nas-os/health-threshold.conf}"

# 输出格式：text, json
OUTPUT_FORMAT="${OUTPUT_FORMAT:-text}"

# 快速检查模式
QUICK_MODE="${QUICK_MODE:-false}"

# =============================================================================
# 颜色定义
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# =============================================================================
# 全局状态
# =============================================================================

OVERALL_STATUS="healthy"
CHECKS=()
ERRORS=()
WARNINGS=()

# =============================================================================
# 工具函数
# =============================================================================

# 加载阈值配置
load_threshold_config() {
    if [ -f "$THRESHOLD_FILE" ]; then
        source "$THRESHOLD_FILE"
    fi
}

# 添加检查结果
add_check() {
    local name="$1"
    local status="$2"
    local message="$3"
    local details="${4:-}"
    
    CHECKS+=("name=$name|status=$status|message=$message|details=$details")
    
    if [ "$status" = "critical" ]; then
        ERRORS+=("$message")
        OVERALL_STATUS="unhealthy"
    elif [ "$status" = "warning" ]; then
        WARNINGS+=("$message")
        [ "$OVERALL_STATUS" = "healthy" ] && OVERALL_STATUS="degraded"
    fi
}

# 获取当前时间戳
get_timestamp() {
    date -Iseconds 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z'
}

# =============================================================================
# 检查函数
# =============================================================================

# 检查进程状态
check_process() {
    local process_name="${1:-nasd}"
    
    if pgrep -x "$process_name" > /dev/null 2>&1; then
        local pid=$(pgrep -x "$process_name" | head -1)
        local mem_kb=$(ps -o rss= -p "$pid" 2>/dev/null | awk '{print $1}')
        local mem_mb=$((mem_kb / 1024))
        local cpu_pct=$(ps -o %cpu= -p "$pid" 2>/dev/null | awk '{printf "%.1f", $1}')
        
        add_check "process_$process_name" "healthy" \
            "进程运行中 (PID: $pid)" \
            "pid=$pid memory_mb=$mem_mb cpu_pct=$cpu_pct"
        return 0
    else
        add_check "process_$process_name" "critical" \
            "进程未运行: $process_name"
        return 1
    fi
}

# 检查 TCP 端口
check_tcp_port() {
    local name="$1"
    local port="$2"
    local host="${3:-localhost}"
    
    if ss -tuln 2>/dev/null | grep -q ":${port} "; then
        add_check "port_$name" "healthy" "端口 $port ($name) 监听中"
        return 0
    elif nc -z -w 2 "$host" "$port" 2>/dev/null; then
        add_check "port_$name" "healthy" "端口 $port ($name) 可达"
        return 0
    else
        add_check "port_$name" "critical" "端口 $port ($name) 不可达"
        return 1
    fi
}

# 检查 HTTP 端点
check_http_endpoint() {
    local name="$1"
    local url="$2"
    local expected_status="${3:-200}"
    local timeout="${4:-5}"
    
    if ! command -v curl &>/dev/null; then
        add_check "http_$name" "warning" "curl 不可用，跳过 HTTP 检查"
        return 2
    fi
    
    local start_time=$(date +%s%3N 2>/dev/null || echo "0")
    local response=$(curl -sf -o /dev/null -w "%{http_code}" --max-time "$timeout" "$url" 2>/dev/null || echo "000")
    local end_time=$(date +%s%3N 2>/dev/null || echo "0")
    local response_time=$((end_time - start_time))
    
    if [ "$response" = "$expected_status" ]; then
        if [ $response_time -gt $API_TIMEOUT_MS ]; then
            add_check "http_$name" "warning" \
                "HTTP $name 响应慢 (${response_time}ms > ${API_TIMEOUT_MS}ms)" \
                "response_time_ms=$response_time"
        else
            add_check "http_$name" "healthy" \
                "HTTP $name 正常 (${response_time}ms)" \
                "response_time_ms=$response_time"
        fi
        return 0
    else
        add_check "http_$name" "critical" \
            "HTTP $name 异常 (状态码: $response)" \
            "status_code=$response"
        return 1
    fi
}

# 检查磁盘空间
check_disk_space() {
    local path="${1:-/}"
    
    local info=$(df -h "$path" 2>/dev/null | awk 'NR==2 {print $5,$4,$2,$3}')
    if [ -z "$info" ]; then
        add_check "disk_$path" "warning" "无法获取磁盘信息: $path"
        return 2
    fi
    
    local usage=$(echo "$info" | awk '{print $1}' | tr -d '%')
    local avail=$(echo "$info" | awk '{print $2}')
    local total=$(echo "$info" | awk '{print $3}')
    local used=$(echo "$info" | awk '{print $4}')
    
    local status="healthy"
    if [ "$usage" -ge "$DISK_THRESHOLD_CRITICAL" ]; then
        status="critical"
    elif [ "$usage" -ge "$DISK_THRESHOLD_WARNING" ]; then
        status="warning"
    fi
    
    add_check "disk_$path" "$status" \
        "磁盘 $path 使用率: ${usage}% (可用: $avail)" \
        "usage_pct=$usage available=$avail total=$total used=$used"
    
    [ "$status" = "healthy" ]
}

# 检查内存使用
check_memory() {
    local mem_info=$(free -m 2>/dev/null | awk '/^Mem:/{print $2,$3,$4,$6}')
    if [ -z "$mem_info" ]; then
        add_check "memory" "warning" "无法获取内存信息"
        return 2
    fi
    
    local total=$(echo "$mem_info" | awk '{print $1}')
    local used=$(echo "$mem_info" | awk '{print $2}')
    local cached=$(echo "$mem_info" | awk '{print $4}')
    
    # 实际使用 = used - cached
    local actual_used=$((used - cached > 0 ? used - cached : used))
    local usage=$((actual_used * 100 / total))
    
    local status="healthy"
    if [ "$usage" -ge "$MEMORY_THRESHOLD_CRITICAL" ]; then
        status="critical"
    elif [ "$usage" -ge "$MEMORY_THRESHOLD_WARNING" ]; then
        status="warning"
    fi
    
    add_check "memory" "$status" \
        "内存使用率: ${usage}% (${actual_used}MB / ${total}MB)" \
        "usage_pct=$usage used_mb=$actual_used total_mb=$total cached_mb=$cached"
    
    [ "$status" = "healthy" ]
}

# 检查 CPU 使用率
check_cpu() {
    # 获取 CPU 使用率
    local cpu_idle=$(top -bn1 2>/dev/null | grep -E '^%?Cpu' | awk '{print $8}' | tr -d '%' | head -1)
    
    if [ -z "$cpu_idle" ]; then
        # 回退方案
        cpu_idle=$(vmstat 1 2 2>/dev/null | tail -1 | awk '{print $15}')
    fi
    
    if [ -z "$cpu_idle" ]; then
        add_check "cpu" "warning" "无法获取 CPU 信息"
        return 2
    fi
    
    local usage=$((100 - ${cpu_idle%.*}))
    [ $usage -lt 0 ] && usage=0
    [ $usage -gt 100 ] && usage=100
    
    local status="healthy"
    if [ "$usage" -ge "$CPU_THRESHOLD_CRITICAL" ]; then
        status="critical"
    elif [ "$usage" -ge "$CPU_THRESHOLD_WARNING" ]; then
        status="warning"
    fi
    
    add_check "cpu" "$status" \
        "CPU 使用率: ${usage}%" \
        "usage_pct=$usage idle_pct=$cpu_idle"
    
    [ "$status" = "healthy" ]
}

# 检查系统负载
check_load() {
    local load=$(cat /proc/loadavg 2>/dev/null | awk '{print $1}')
    local cores=$(nproc 2>/dev/null || echo 1)
    
    if [ -z "$load" ]; then
        add_check "load" "warning" "无法获取系统负载"
        return 2
    fi
    
    local load_per_core=$(awk "BEGIN {printf \"%.2f\", $load / $cores}")
    
    local status="healthy"
    if [ $(awk "BEGIN {print ($load > $cores * 2)}") -eq 1 ]; then
        status="critical"
    elif [ $(awk "BEGIN {print ($load > $cores * 1.5)}") -eq 1 ]; then
        status="warning"
    fi
    
    add_check "load" "$status" \
        "系统负载: $load (${cores} 核心, ${load_per_core}/核心)" \
        "load=$load cores=$cores load_per_core=$load_per_core"
    
    [ "$status" = "healthy" ]
}

# 检查数据库
check_database() {
    local db_path="${1:-/var/lib/nas-os/nas-os.db}"
    
    if [ ! -f "$db_path" ]; then
        add_check "database" "warning" "数据库文件不存在: $db_path"
        return 2
    fi
    
    if ! command -v sqlite3 &>/dev/null; then
        add_check "database" "warning" "sqlite3 不可用，跳过数据库检查"
        return 2
    fi
    
    local result=$(sqlite3 "$db_path" "PRAGMA integrity_check;" 2>&1)
    
    if [ "$result" = "ok" ]; then
        local size=$(du -h "$db_path" 2>/dev/null | cut -f1)
        add_check "database" "healthy" \
            "数据库正常 (大小: $size)" \
            "size=$size path=$db_path"
        return 0
    else
        add_check "database" "critical" \
            "数据库完整性检查失败: $result" \
            "path=$db_path"
        return 1
    fi
}

# 检查日志错误
check_logs() {
    local log_file="${1:-$LOG_DIR/nas-os.log}"
    local lines="${2:-100}"
    
    if [ ! -f "$log_file" ]; then
        # 尝试 journalctl
        if command -v journalctl &>/dev/null; then
            local errors=$(journalctl -u nas-os --no-pager -n "$lines" 2>/dev/null | grep -ci "error" || echo "0")
            local warns=$(journalctl -u nas-os --no-pager -n "$lines" 2>/dev/null | grep -ci "warn" || echo "0")
            
            if [ "$errors" -gt 0 ]; then
                add_check "logs" "warning" \
                    "最近 $lines 条日志中有 $errors 个错误" \
                    "errors=$errors warnings=$warns"
            else
                add_check "logs" "healthy" \
                    "日志无错误 (警告: $warns)" \
                    "warnings=$warns"
            fi
            return 0
        fi
        
        add_check "logs" "warning" "日志文件不存在: $log_file"
        return 2
    fi
    
    local errors=$(grep -ci "error" "$log_file" 2>/dev/null | tail -1 || echo "0")
    local warns=$(grep -ci "warn" "$log_file" 2>/dev/null | tail -1 || echo "0")
    
    # 只统计最近的
    errors=$(tail -n "$lines" "$log_file" 2>/dev/null | grep -ci "error" || echo "0")
    warns=$(tail -n "$lines" "$log_file" 2>/dev/null | grep -ci "warn" || echo "0")
    
    if [ "$errors" -gt 0 ]; then
        add_check "logs" "warning" \
            "最近 $lines 行日志中有 $errors 个错误" \
            "errors=$errors warnings=$warns"
    else
        add_check "logs" "healthy" \
            "日志无错误 (警告: $warns)" \
            "warnings=$warns"
    fi
}

# =============================================================================
# 检查流程
# =============================================================================

# 快速检查（仅关键项）
run_quick_check() {
    check_process "nasd"
    check_tcp_port "api" "$API_PORT"
    check_http_endpoint "health" "$API_URL/api/v1/health"
}

# 完整检查
run_full_check() {
    echo "==================================="
    echo "NAS-OS 健康检查 v$VERSION"
    echo "时间: $(get_timestamp)"
    echo "==================================="
    echo ""
    
    # 进程检查
    echo -e "${BLUE}[检查] 进程状态${NC}"
    check_process "nasd"
    echo ""
    
    # 端口检查
    echo -e "${BLUE}[检查] 端口状态${NC}"
    check_tcp_port "api" "$API_PORT"
    check_tcp_port "smb" "445"
    check_tcp_port "nfs" "2049"
    echo ""
    
    # API 检查
    echo -e "${BLUE}[检查] API 端点${NC}"
    check_http_endpoint "health" "$API_URL/api/v1/health"
    check_http_endpoint "ready" "$API_URL/api/v1/ready" "200" "5"
    echo ""
    
    # 系统资源检查
    echo -e "${BLUE}[检查] 系统资源${NC}"
    check_disk_space "/"
    check_disk_space "$DATA_DIR" 2>/dev/null || true
    check_memory
    check_cpu
    check_load
    echo ""
    
    # 数据库检查
    echo -e "${BLUE}[检查] 数据库${NC}"
    check_database
    echo ""
    
    # 日志检查
    echo -e "${BLUE}[检查] 日志${NC}"
    check_logs
    echo ""
}

# =============================================================================
# 输出函数
# =============================================================================

# 输出 JSON 格式
output_json() {
    local status_code=0
    [ "$OVERALL_STATUS" = "unhealthy" ] && status_code=1
    [ "$OVERALL_STATUS" = "degraded" ] && status_code=2
    
    echo "{"
    echo "  \"version\": \"$VERSION\","
    echo "  \"timestamp\": \"$(get_timestamp)\","
    echo "  \"hostname\": \"$(hostname)\","
    echo "  \"status\": \"$OVERALL_STATUS\","
    echo "  \"checks\": ["
    
    local first=true
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local status=$(echo "$check" | grep -o 'status=[^|]*' | cut -d= -f2)
        local message=$(echo "$check" | grep -o 'message=[^|]*' | cut -d= -f2-)
        local details=$(echo "$check" | grep -o 'details=[^|]*' | cut -d= -f2-)
        
        # 转义特殊字符
        message=$(echo "$message" | sed 's/"/\\"/g')
        details=$(echo "$details" | sed 's/"/\\"/g')
        
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        
        printf "    {\"name\": \"%s\", \"status\": \"%s\", \"message\": \"%s\", \"details\": \"%s\"}" \
            "$name" "$status" "$message" "$details"
    done
    
    echo ""
    echo "  ],"
    echo "  \"errors\": ["
    first=true
    for error in "${ERRORS[@]}"; do
        error=$(echo "$error" | sed 's/"/\\"/g')
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        printf "    \"%s\"" "$error"
    done
    echo ""
    echo "  ],"
    echo "  \"warnings\": ["
    first=true
    for warning in "${WARNINGS[@]}"; do
        warning=$(echo "$warning" | sed 's/"/\\"/g')
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        printf "    \"%s\"" "$warning"
    done
    echo ""
    echo "  ]"
    echo "}"
    
    return $status_code
}

# 输出文本格式
output_text() {
    echo ""
    echo "==================================="
    echo "检查结果汇总"
    echo "==================================="
    
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local status=$(echo "$check" | grep -o 'status=[^|]*' | cut -d= -f2)
        local message=$(echo "$check" | grep -o 'message=[^|]*' | cut -d= -f2-)
        
        local color=""
        case "$status" in
            healthy)  color="$GREEN" ;;
            warning)  color="$YELLOW" ;;
            critical) color="$RED" ;;
        esac
        
        printf "  ${color}%-10s${NC} %s\n" "[$status]" "$message"
    done
    
    echo ""
    echo "==================================="
    
    local status_color="$GREEN"
    [ "$OVERALL_STATUS" = "degraded" ] && status_color="$YELLOW"
    [ "$OVERALL_STATUS" = "unhealthy" ] && status_color="$RED"
    
    echo -e "整体状态: ${status_color}${OVERALL_STATUS}${NC}"
    
    if [ ${#ERRORS[@]} -gt 0 ]; then
        echo ""
        echo -e "${RED}错误项 (${#ERRORS[@]}):${NC}"
        for error in "${ERRORS[@]}"; do
            echo "  - $error"
        done
    fi
    
    if [ ${#WARNINGS[@]} -gt 0 ]; then
        echo ""
        echo -e "${YELLOW}警告项 (${#WARNINGS[@]}):${NC}"
        for warning in "${WARNINGS[@]}"; do
            echo "  - $warning"
        done
    fi
    
    echo ""
    
    # 返回退出码
    [ "$OVERALL_STATUS" = "healthy" ] && return 0
    [ "$OVERALL_STATUS" = "degraded" ] && return 2
    return 1
}

# =============================================================================
# 帮助信息
# =============================================================================

show_help() {
    cat <<EOF
NAS-OS 健康检查脚本 v$VERSION

用途：检查服务状态、磁盘空间、内存使用，输出 JSON 格式结果

用法: $0 [选项]

选项:
  --json              JSON 格式输出
  --quick             快速检查模式（仅检查关键项）
  --threshold-file F  指定阈值配置文件
  --help              显示帮助信息

环境变量:
  API_HOST               API 主机 (默认: localhost)
  API_PORT               API 端口 (默认: 8080)
  DISK_THRESHOLD_WARNING 磁盘警告阈值 (默认: 80%)
  DISK_THRESHOLD_CRITICAL 磁盘严重阈值 (默认: 90%)
  MEMORY_THRESHOLD_WARNING 内存警告阈值 (默认: 80%)
  MEMORY_THRESHOLD_CRITICAL 内存严重阈值 (默认: 90%)
  CPU_THRESHOLD_WARNING  CPU 警告阈值 (默认: 80%)
  CPU_THRESHOLD_CRITICAL CPU 严重阈值 (默认: 95%)

阈值配置文件示例 ($THRESHOLD_FILE):
  DISK_THRESHOLD_WARNING=70
  DISK_THRESHOLD_CRITICAL=85
  MEMORY_THRESHOLD_WARNING=75
  MEMORY_THRESHOLD_CRITICAL=90

退出码:
  0 - 健康
  1 - 不健康（有严重问题）
  2 - 降级（有警告但无严重问题）

示例:
  $0                    # 完整检查，文本输出
  $0 --json             # 完整检查，JSON 输出
  $0 --quick            # 快速检查
  $0 --json --quick     # 快速检查，JSON 输出

EOF
}

# =============================================================================
# 主入口
# =============================================================================

main() {
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json)
                OUTPUT_FORMAT="json"
                shift
                ;;
            --quick)
                QUICK_MODE="true"
                shift
                ;;
            --threshold-file)
                THRESHOLD_FILE="$2"
                shift 2
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
    
    # 加载阈值配置
    load_threshold_config
    
    # 执行检查
    if [ "$QUICK_MODE" = "true" ]; then
        run_quick_check
    else
        run_full_check
    fi
    
    # 输出结果
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        output_json
        exit $?
    else
        output_text
        exit $?
    fi
}

main "$@"