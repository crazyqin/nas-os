#!/bin/bash
# =============================================================================
# NAS-OS 健康检查脚本 v2.76.0
# =============================================================================
# 用途：全面检查服务状态、系统资源、存储健康、网络连接
# 用法：./health-check.sh [--json] [--quick] [--full] [--component COMPONENT]
#
# v2.76.0 更新（工部增强）：
# - 添加组件级健康检查
# - 添加存储池健康检查
# - 添加网络连通性检查
# - 添加容器健康检查
# - 添加健康评分系统
# - 添加 Prometheus 指标输出
# - 支持分布式监控集成
# =============================================================================

set -euo pipefail

# 版本
VERSION="2.76.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 默认配置
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

# 负载阈值（相对于 CPU 核心数）
LOAD_THRESHOLD_WARNING="${LOAD_THRESHOLD_WARNING:-1.5}"
LOAD_THRESHOLD_CRITICAL="${LOAD_THRESHOLD_CRITICAL:-2.0}"

# API 响应超时（毫秒）
API_TIMEOUT_MS="${API_TIMEOUT_MS:-5000}"

# 数据目录
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"

# 日志目录
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 阈值配置文件
THRESHOLD_FILE="${THRESHOLD_FILE:-/etc/nas-os/health-threshold.conf}"

# 输出格式：text, json, prometheus
OUTPUT_FORMAT="${OUTPUT_FORMAT:-text}"

# 检查模式：quick, full, component
CHECK_MODE="${CHECK_MODE:-full}"

# 指定检查的组件
CHECK_COMPONENT="${CHECK_COMPONENT:-}"

# Prometheus 指标前缀
METRIC_PREFIX="nas_os_health"

# =============================================================================
# 颜色定义
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# =============================================================================
# 全局状态
# =============================================================================

OVERALL_STATUS="healthy"
HEALTH_SCORE=100
CHECKS=()
ERRORS=()
WARNINGS=()
COMPONENT_SCORES=()

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
    local score="${5:-100}"
    
    CHECKS+=("name=$name|status=$status|message=$message|details=$details|score=$score")
    
    if [ "$status" = "critical" ]; then
        ERRORS+=("$message")
        OVERALL_STATUS="unhealthy"
        HEALTH_SCORE=$((HEALTH_SCORE - 20))
    elif [ "$status" = "warning" ]; then
        WARNINGS+=("$message")
        [ "$OVERALL_STATUS" = "healthy" ] && OVERALL_STATUS="degraded"
        HEALTH_SCORE=$((HEALTH_SCORE - 5))
    fi
}

# 获取当前时间戳
get_timestamp() {
    date -Iseconds 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z'
}

# 计算 JSON 转义
json_escape() {
    echo "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
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
            "pid=$pid memory_mb=$mem_mb cpu_pct=$cpu_pct" "100"
        return 0
    else
        add_check "process_$process_name" "critical" \
            "进程未运行: $process_name" "" "0"
        return 1
    fi
}

# 检查 TCP 端口
check_tcp_port() {
    local name="$1"
    local port="$2"
    local host="${3:-localhost}"
    
    if ss -tuln 2>/dev/null | grep -q ":${port} "; then
        add_check "port_$name" "healthy" "端口 $port ($name) 监听中" "" "100"
        return 0
    elif nc -z -w 2 "$host" "$port" 2>/dev/null; then
        add_check "port_$name" "healthy" "端口 $port ($name) 可达" "" "100"
        return 0
    else
        add_check "port_$name" "critical" "端口 $port ($name) 不可达" "" "0"
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
        add_check "http_$name" "warning" "curl 不可用，跳过 HTTP 检查" "" "50"
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
                "response_time_ms=$response_time" "70"
        else
            add_check "http_$name" "healthy" \
                "HTTP $name 正常 (${response_time}ms)" \
                "response_time_ms=$response_time" "100"
        fi
        return 0
    else
        add_check "http_$name" "critical" \
            "HTTP $name 异常 (状态码: $response)" \
            "status_code=$response" "0"
        return 1
    fi
}

# 检查磁盘空间
check_disk_space() {
    local path="${1:-/}"
    
    local info=$(df -h "$path" 2>/dev/null | awk 'NR==2 {print $5,$4,$2,$3}')
    if [ -z "$info" ]; then
        add_check "disk_$path" "warning" "无法获取磁盘信息: $path" "" "50"
        return 2
    fi
    
    local usage=$(echo "$info" | awk '{print $1}' | tr -d '%')
    local avail=$(echo "$info" | awk '{print $2}')
    local total=$(echo "$info" | awk '{print $3}')
    local used=$(echo "$info" | awk '{print $4}')
    
    local status="healthy"
    local score=100
    
    if [ "$usage" -ge "$DISK_THRESHOLD_CRITICAL" ]; then
        status="critical"
        score=20
    elif [ "$usage" -ge "$DISK_THRESHOLD_WARNING" ]; then
        status="warning"
        score=60
    fi
    
    local safe_path=$(echo "$path" | tr '/' '_')
    add_check "disk_$safe_path" "$status" \
        "磁盘 $path 使用率: ${usage}% (可用: $avail)" \
        "usage_pct=$usage available=$avail total=$total used=$used" "$score"
    
    [ "$status" = "healthy" ]
}

# 检查内存使用
check_memory() {
    local mem_info=$(free -m 2>/dev/null | awk '/^Mem:/{print $2,$3,$4,$6}')
    if [ -z "$mem_info" ]; then
        add_check "memory" "warning" "无法获取内存信息" "" "50"
        return 2
    fi
    
    local total=$(echo "$mem_info" | awk '{print $1}')
    local used=$(echo "$mem_info" | awk '{print $2}')
    local cached=$(echo "$mem_info" | awk '{print $4}')
    
    # 实际使用 = used - cached
    local actual_used=$((used - cached > 0 ? used - cached : used))
    local usage=$((actual_used * 100 / total))
    
    local status="healthy"
    local score=100
    
    if [ "$usage" -ge "$MEMORY_THRESHOLD_CRITICAL" ]; then
        status="critical"
        score=20
    elif [ "$usage" -ge "$MEMORY_THRESHOLD_WARNING" ]; then
        status="warning"
        score=60
    fi
    
    add_check "memory" "$status" \
        "内存使用率: ${usage}% (${actual_used}MB / ${total}MB)" \
        "usage_pct=$usage used_mb=$actual_used total_mb=$total cached_mb=$cached" "$score"
    
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
        add_check "cpu" "warning" "无法获取 CPU 信息" "" "50"
        return 2
    fi
    
    local usage=$((100 - ${cpu_idle%.*}))
    [ $usage -lt 0 ] && usage=0
    [ $usage -gt 100 ] && usage=100
    
    local status="healthy"
    local score=100
    
    if [ "$usage" -ge "$CPU_THRESHOLD_CRITICAL" ]; then
        status="critical"
        score=20
    elif [ "$usage" -ge "$CPU_THRESHOLD_WARNING" ]; then
        status="warning"
        score=60
    fi
    
    add_check "cpu" "$status" \
        "CPU 使用率: ${usage}%" \
        "usage_pct=$usage idle_pct=$cpu_idle" "$score"
    
    [ "$status" = "healthy" ]
}

# 检查系统负载
check_load() {
    local load=$(cat /proc/loadavg 2>/dev/null | awk '{print $1}')
    local cores=$(nproc 2>/dev/null || echo 1)
    
    if [ -z "$load" ]; then
        add_check "load" "warning" "无法获取系统负载" "" "50"
        return 2
    fi
    
    local load_per_core=$(awk "BEGIN {printf \"%.2f\", $load / $cores}")
    
    local status="healthy"
    local score=100
    
    if [ $(awk "BEGIN {print ($load > $cores * $LOAD_THRESHOLD_CRITICAL)}") -eq 1 ]; then
        status="critical"
        score=20
    elif [ $(awk "BEGIN {print ($load > $cores * $LOAD_THRESHOLD_WARNING)}") -eq 1 ]; then
        status="warning"
        score=60
    fi
    
    add_check "load" "$status" \
        "系统负载: $load (${cores} 核心, ${load_per_core}/核心)" \
        "load=$load cores=$cores load_per_core=$load_per_core" "$score"
    
    [ "$status" = "healthy" ]
}

# 检查数据库
check_database() {
    local db_path="${1:-/var/lib/nas-os/nas-os.db}"
    
    if [ ! -f "$db_path" ]; then
        add_check "database" "warning" "数据库文件不存在: $db_path" "" "50"
        return 2
    fi
    
    if ! command -v sqlite3 &>/dev/null; then
        add_check "database" "warning" "sqlite3 不可用，跳过数据库检查" "" "50"
        return 2
    fi
    
    local result=$(sqlite3 "$db_path" "PRAGMA integrity_check;" 2>&1)
    
    if [ "$result" = "ok" ]; then
        local size=$(du -h "$db_path" 2>/dev/null | cut -f1)
        add_check "database" "healthy" \
            "数据库正常 (大小: $size)" \
            "size=$size path=$db_path" "100"
        return 0
    else
        add_check "database" "critical" \
            "数据库完整性检查失败: $result" \
            "path=$db_path" "0"
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
                    "errors=$errors warnings=$warns" "60"
            else
                add_check "logs" "healthy" \
                    "日志无错误 (警告: $warns)" \
                    "warnings=$warns" "100"
            fi
            return 0
        fi
        
        add_check "logs" "warning" "日志文件不存在: $log_file" "" "50"
        return 2
    fi
    
    local errors=$(tail -n "$lines" "$log_file" 2>/dev/null | grep -ci "error" || echo "0")
    local warns=$(tail -n "$lines" "$log_file" 2>/dev/null | grep -ci "warn" || echo "0")
    
    if [ "$errors" -gt 0 ]; then
        add_check "logs" "warning" \
            "最近 $lines 行日志中有 $errors 个错误" \
            "errors=$errors warnings=$warns" "60"
    else
        add_check "logs" "healthy" \
            "日志无错误 (警告: $warns)" \
            "warnings=$warns" "100"
    fi
}

# 检查存储池状态
check_storage_pool() {
    # 检查 Btrfs 状态
    if command -v btrfs &>/dev/null; then
        # 查找 Btrfs 文件系统
        local btrfs_fs=$(findmnt -t btrfs -n -o TARGET 2>/dev/null | head -1)
        
        if [ -n "$btrfs_fs" ]; then
            local device_stats=$(btrfs device stats "$btrfs_fs" 2>/dev/null || true)
            local errors=$(echo "$device_stats" | grep -v "0$" | wc -l)
            
            if [ "$errors" -gt 0 ]; then
                add_check "storage_pool" "critical" \
                    "存储池存在设备错误" \
                    "errors=$errors path=$btrfs_fs" "20"
            else
                add_check "storage_pool" "healthy" \
                    "存储池状态正常" \
                    "path=$btrfs_fs" "100"
            fi
            return 0
        fi
    fi
    
    # 检查 ZFS 状态（如果安装）
    if command -v zpool &>/dev/null; then
        local zpool_status=$(zpool status 2>/dev/null || true)
        if echo "$zpool_status" | grep -q "state: ONLINE"; then
            add_check "storage_pool" "healthy" "ZFS 存储池状态正常" "" "100"
            return 0
        elif echo "$zpool_status" | grep -q "state: DEGRADED"; then
            add_check "storage_pool" "critical" "ZFS 存储池降级" "" "20"
            return 1
        fi
    fi
    
    add_check "storage_pool" "warning" "未检测到存储池" "" "50"
}

# 检查 SMART 状态
check_smart() {
    if ! command -v smartctl &>/dev/null; then
        add_check "smart" "warning" "smartctl 不可用，跳过 SMART 检查" "" "50"
        return 2
    fi
    
    local disks=$(lsblk -d -o NAME 2>/dev/null | grep -E '^sd|^nvme|^vd|^hd' | head -5)
    local smart_issues=0
    
    for disk in $disks; do
        local health=$(smartctl -H /dev/$disk 2>/dev/null || true)
        if ! echo "$health" | grep -qi "PASSED\|OK"; then
            smart_issues=$((smart_issues + 1))
        fi
    done
    
    if [ "$smart_issues" -gt 0 ]; then
        add_check "smart" "critical" \
            "$smart_issues 个磁盘 SMART 检测异常" \
            "issues=$smart_issues" "20"
        return 1
    else
        add_check "smart" "healthy" "所有磁盘 SMART 检测通过" "" "100"
        return 0
    fi
}

# 检查网络连通性
check_network() {
    local targets=("8.8.8.8" "1.1.1.1")
    local failed=0
    
    for target in "${targets[@]}"; do
        if ! ping -c 1 -W 2 "$target" &>/dev/null; then
            failed=$((failed + 1))
        fi
    done
    
    if [ "$failed" -eq ${#targets[@]} ]; then
        add_check "network" "critical" "网络连通性异常" "" "0"
        return 1
    elif [ "$failed" -gt 0 ]; then
        add_check "network" "warning" "部分网络目标不可达" "" "60"
        return 2
    else
        add_check "network" "healthy" "网络连通性正常" "" "100"
        return 0
    fi
}

# 检查 DNS 解析
check_dns() {
    local domains=("github.com" "google.com")
    local failed=0
    
    for domain in "${domains[@]}"; do
        if ! nslookup "$domain" &>/dev/null; then
            failed=$((failed + 1))
        fi
    done
    
    if [ "$failed" -eq ${#domains[@]} ]; then
        add_check "dns" "critical" "DNS 解析失败" "" "0"
        return 1
    elif [ "$failed" -gt 0 ]; then
        add_check "dns" "warning" "部分 DNS 解析失败" "" "60"
        return 2
    else
        add_check "dns" "healthy" "DNS 解析正常" "" "100"
        return 0
    fi
}

# 检查容器状态（如果使用 Docker）
check_containers() {
    if ! command -v docker &>/dev/null; then
        return 0
    fi
    
    local containers=$(docker ps -a --filter "name=nas" --format "{{.Names}}:{{.Status}}" 2>/dev/null || true)
    
    if [ -z "$containers" ]; then
        return 0
    fi
    
    local unhealthy=0
    while IFS=: read -r name status; do
        if ! echo "$status" | grep -qi "running\|healthy"; then
            unhealthy=$((unhealthy + 1))
        fi
    done <<< "$containers"
    
    if [ "$unhealthy" -gt 0 ]; then
        add_check "containers" "warning" \
            "$unhealthy 个容器状态异常" \
            "unhealthy=$unhealthy" "60"
    else
        add_check "containers" "healthy" "所有容器状态正常" "" "100"
    fi
}

# 检查备份状态
check_backup() {
    local backup_dir="${1:-$DATA_DIR/backups}"
    
    if [ ! -d "$backup_dir" ]; then
        add_check "backup" "warning" "备份目录不存在" "" "50"
        return 2
    fi
    
    # 检查最近的备份
    local latest_backup=$(find "$backup_dir" -name "*.tar.gz" -o -name "*.sql" 2>/dev/null | head -1)
    
    if [ -z "$latest_backup" ]; then
        add_check "backup" "warning" "没有找到备份文件" "" "50"
        return 2
    fi
    
    local backup_age=$(($(date +%s) - $(stat -c %Y "$latest_backup" 2>/dev/null || echo "0")))
    local backup_age_hours=$((backup_age / 3600))
    
    if [ "$backup_age_hours" -gt 48 ]; then
        add_check "backup" "warning" \
            "最近备份已超过 48 小时 (${backup_age_hours}h)" \
            "age_hours=$backup_age_hours" "60"
    else
        local backup_size=$(du -h "$latest_backup" 2>/dev/null | cut -f1)
        add_check "backup" "healthy" \
            "备份正常 (大小: $backup_size, ${backup_age_hours}h 前)" \
            "age_hours=$backup_age_hours size=$backup_size" "100"
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
    
    # 存储检查
    echo -e "${BLUE}[检查] 存储健康${NC}"
    check_storage_pool
    check_smart
    echo ""
    
    # 网络检查
    echo -e "${BLUE}[检查] 网络连通${NC}"
    check_network
    check_dns
    echo ""
    
    # 数据库检查
    echo -e "${BLUE}[检查] 数据库${NC}"
    check_database
    echo ""
    
    # 日志检查
    echo -e "${BLUE}[检查] 日志${NC}"
    check_logs
    echo ""
    
    # 容器检查
    echo -e "${BLUE}[检查] 容器${NC}"
    check_containers
    echo ""
    
    # 备份检查
    echo -e "${BLUE}[检查] 备份${NC}"
    check_backup
    echo ""
}

# 组件检查
run_component_check() {
    case "$CHECK_COMPONENT" in
        process)
            check_process "nasd"
            ;;
        network)
            check_tcp_port "api" "$API_PORT"
            check_network
            check_dns
            ;;
        storage)
            check_disk_space "/"
            check_storage_pool
            check_smart
            ;;
        memory)
            check_memory
            ;;
        cpu)
            check_cpu
            check_load
            ;;
        database)
            check_database
            ;;
        backup)
            check_backup
            ;;
        api)
            check_http_endpoint "health" "$API_URL/api/v1/health"
            check_http_endpoint "ready" "$API_URL/api/v1/ready"
            ;;
        *)
            log_error "未知组件: $CHECK_COMPONENT"
            echo "可用组件: process, network, storage, memory, cpu, database, backup, api"
            exit 1
            ;;
    esac
}

# =============================================================================
# 输出函数
# =============================================================================

# 输出 JSON 格式
output_json() {
    local status_code=0
    [ "$OVERALL_STATUS" = "unhealthy" ] && status_code=1
    [ "$OVERALL_STATUS" = "degraded" ] && status_code=2
    
    # 确保健康评分在合理范围内
    [ $HEALTH_SCORE -lt 0 ] && HEALTH_SCORE=0
    [ $HEALTH_SCORE -gt 100 ] && HEALTH_SCORE=100
    
    echo "{"
    echo "  \"version\": \"$VERSION\","
    echo "  \"timestamp\": \"$(get_timestamp)\","
    echo "  \"hostname\": \"$(hostname)\","
    echo "  \"status\": \"$OVERALL_STATUS\","
    echo "  \"health_score\": $HEALTH_SCORE,"
    echo "  \"checks\": ["
    
    local first=true
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local status=$(echo "$check" | grep -o 'status=[^|]*' | cut -d= -f2)
        local message=$(json_escape "$(echo "$check" | grep -o 'message=[^|]*' | cut -d= -f2-)")
        local details=$(json_escape "$(echo "$check" | grep -o 'details=[^|]*' | cut -d= -f2-)")
        local score=$(echo "$check" | grep -o 'score=[^|]*' | cut -d= -f2)
        
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        
        printf "    {\"name\": \"%s\", \"status\": \"%s\", \"message\": \"%s\", \"details\": \"%s\", \"score\": %s}" \
            "$name" "$status" "$message" "$details" "${score:-100}"
    done
    
    echo ""
    echo "  ],"
    echo "  \"errors\": ["
    first=true
    for error in "${ERRORS[@]}"; do
        error=$(json_escape "$error")
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
        warning=$(json_escape "$warning")
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

# 输出 Prometheus 格式
output_prometheus() {
    echo "# HELP ${METRIC_PREFIX}_status Overall health status (1=healthy, 2=degraded, 3=unhealthy)"
    echo "# TYPE ${METRIC_PREFIX}_status gauge"
    
    local status_value=1
    [ "$OVERALL_STATUS" = "degraded" ] && status_value=2
    [ "$OVERALL_STATUS" = "unhealthy" ] && status_value=3
    echo "${METRIC_PREFIX}_status $status_value"
    
    echo "# HELP ${METRIC_PREFIX}_score Overall health score (0-100)"
    echo "# TYPE ${METRIC_PREFIX}_score gauge"
    [ $HEALTH_SCORE -lt 0 ] && HEALTH_SCORE=0
    [ $HEALTH_SCORE -gt 100 ] && HEALTH_SCORE=100
    echo "${METRIC_PREFIX}_score $HEALTH_SCORE"
    
    echo "# HELP ${METRIC_PREFIX}_check_status Individual check status (0=fail, 1=pass, 2=warn)"
    echo "# TYPE ${METRIC_PREFIX}_check_status gauge"
    
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local status=$(echo "$check" | grep -o 'status=[^|]*' | cut -d= -f2)
        local status_value=1
        
        [ "$status" = "warning" ] && status_value=2
        [ "$status" = "critical" ] && status_value=0
        
        echo "${METRIC_PREFIX}_check_status{check=\"$name\"} $status_value"
    done
    
    echo "# HELP ${METRIC_PREFIX}_check_score Individual check score (0-100)"
    echo "# TYPE ${METRIC_PREFIX}_check_score gauge"
    
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local score=$(echo "$check" | grep -o 'score=[^|]*' | cut -d= -f2)
        echo "${METRIC_PREFIX}_check_score{check=\"$name\"} ${score:-100}"
    done
    
    echo "# HELP ${METRIC_PREFIX}_errors_total Total number of errors"
    echo "# TYPE ${METRIC_PREFIX}_errors_total gauge"
    echo "${METRIC_PREFIX}_errors_total ${#ERRORS[@]}"
    
    echo "# HELP ${METRIC_PREFIX}_warnings_total Total number of warnings"
    echo "# TYPE ${METRIC_PREFIX}_warnings_total gauge"
    echo "${METRIC_PREFIX}_warnings_total ${#WARNINGS[@]}"
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
    
    # 确保健康评分在合理范围内
    [ $HEALTH_SCORE -lt 0 ] && HEALTH_SCORE=0
    [ $HEALTH_SCORE -gt 100 ] && HEALTH_SCORE=100
    
    local status_color="$GREEN"
    [ "$OVERALL_STATUS" = "degraded" ] && status_color="$YELLOW"
    [ "$OVERALL_STATUS" = "unhealthy" ] && status_color="$RED"
    
    echo -e "整体状态: ${status_color}${OVERALL_STATUS}${NC}"
    echo -e "健康评分: ${HEALTH_SCORE}/100"
    
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

用途：全面检查服务状态、系统资源、存储健康、网络连接

用法: $0 [选项]

选项:
  --json              JSON 格式输出
  --prometheus        Prometheus 指标格式输出
  --quick             快速检查模式（仅检查关键项）
  --full              完整检查模式（默认）
  --component NAME    检查指定组件
  --threshold-file F  指定阈值配置文件
  --help              显示帮助信息

组件:
  process    进程状态
  network    网络连通性
  storage    存储健康
  memory     内存使用
  cpu        CPU 使用
  database   数据库状态
  backup     备份状态
  api        API 端点

环境变量:
  API_HOST                    API 主机 (默认: localhost)
  API_PORT                    API 端口 (默认: 8080)
  DISK_THRESHOLD_WARNING      磁盘警告阈值 (默认: 80%)
  DISK_THRESHOLD_CRITICAL     磁盘严重阈值 (默认: 90%)
  MEMORY_THRESHOLD_WARNING    内存警告阈值 (默认: 80%)
  MEMORY_THRESHOLD_CRITICAL   内存严重阈值 (默认: 90%)
  CPU_THRESHOLD_WARNING       CPU 警告阈值 (默认: 80%)
  CPU_THRESHOLD_CRITICAL      CPU 严重阈值 (默认: 95%)

退出码:
  0 - 健康
  1 - 不健康（有严重问题）
  2 - 降级（有警告但无严重问题）

示例:
  $0                           # 完整检查，文本输出
  $0 --json                    # 完整检查，JSON 输出
  $0 --prometheus              # Prometheus 指标输出
  $0 --quick                   # 快速检查
  $0 --component storage       # 仅检查存储组件

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
            --prometheus)
                OUTPUT_FORMAT="prometheus"
                shift
                ;;
            --quick)
                CHECK_MODE="quick"
                shift
                ;;
            --full)
                CHECK_MODE="full"
                shift
                ;;
            --component)
                CHECK_MODE="component"
                CHECK_COMPONENT="$2"
                shift 2
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
    case "$CHECK_MODE" in
        quick)
            run_quick_check
            ;;
        full)
            run_full_check
            ;;
        component)
            run_component_check
            ;;
    esac
    
    # 输出结果
    case "$OUTPUT_FORMAT" in
        json)
            output_json
            exit $?
            ;;
        prometheus)
            output_prometheus
            exit 0
            ;;
        *)
            output_text
            exit $?
            ;;
    esac
}

main "$@"