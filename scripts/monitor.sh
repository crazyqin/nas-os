#!/bin/bash
# NAS-OS 系统监控脚本
# 周期性监控系统资源和服务状态，支持自动告警
#
# v1.0.0 工部创建
#
# 功能:
# - 系统资源监控 (CPU/内存/磁盘/网络)
# - 服务健康检查 (HTTP/TCP/进程)
# - 自动告警通知 (Webhook/邮件/钉钉/飞书)
# - 历史数据记录与趋势分析
# - Prometheus 指标导出
# - 多种运行模式 (单次/守护/定时)
#
# 用法:
#   ./monitor.sh                    # 单次检查
#   ./monitor.sh --daemon          # 守护进程模式
#   ./monitor.sh --cron            # cron 任务模式
#   ./monitor.sh --status          # 查看状态
#   ./monitor.sh --export          # 导出 Prometheus 指标

set -euo pipefail

#===========================================
# 配置
#===========================================

# 版本
VERSION="1.0.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# 监控配置
MONITOR_INTERVAL="${MONITOR_INTERVAL:-60}"           # 监控间隔(秒)
HISTORY_RETENTION="${HISTORY_RETENTION:-30}"         # 历史保留天数
DATA_DIR="${DATA_DIR:-/var/lib/nas-os/monitor}"       # 数据目录
LOG_FILE="${LOG_FILE:-/var/log/nas-os/monitor.log}"   # 日志文件

# 阈值配置 (百分比)
CPU_THRESHOLD_WARNING="${CPU_THRESHOLD_WARNING:-80}"
CPU_THRESHOLD_CRITICAL="${CPU_THRESHOLD_CRITICAL:-95}"
MEMORY_THRESHOLD_WARNING="${MEMORY_THRESHOLD_WARNING:-80}"
MEMORY_THRESHOLD_CRITICAL="${MEMORY_THRESHOLD_CRITICAL:-95}"
DISK_THRESHOLD_WARNING="${DISK_THRESHOLD_WARNING:-80}"
DISK_THRESHOLD_CRITICAL="${DISK_THRESHOLD_CRITICAL:-95}"
LOAD_THRESHOLD_WARNING="${LOAD_THRESHOLD_WARNING:-1.0}"   # 相对于核心数
LOAD_THRESHOLD_CRITICAL="${LOAD_THRESHOLD_CRITICAL:-2.0}"

# 服务配置
SERVICES_CONFIG="${SERVICES_CONFIG:-/etc/nas-os/monitor-services.conf}"

# 告警配置
ALERT_ENABLED="${ALERT_ENABLED:-true}"
ALERT_COOLDOWN="${ALERT_COOLDOWN:-300}"              # 告警冷却时间(秒)
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_EMAIL="${ALERT_EMAIL:-}"
ALERT_DINGTALK="${ALERT_DINGTALK:-}"
ALERT_FEISHU="${ALERT_FEISHU:-}"
ALERT_SMS="${ALERT_SMS:-}"

# Prometheus 指标端口
PROMETHEUS_PORT="${PROMETHEUS_PORT:-9101}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

#===========================================
# 全局变量
#===========================================

METRICS={}
ALERTS=()
LAST_ALERT_TIME={}
PID_FILE="/var/run/nas-os-monitor.pid"

#===========================================
# 工具函数
#===========================================

log() {
    local level="$1"
    shift
    local msg="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    # 写入日志文件 (如果目录存在)
    local log_dir="$(dirname "$LOG_FILE")"
    if [ -d "$log_dir" ] && [ -w "$log_dir" ]; then
        echo "[$timestamp] [$level] $msg" >> "$LOG_FILE" 2>/dev/null || true
    fi
    
    # 控制台输出
    case "$level" in
        ERROR)   echo -e "${RED}[$timestamp] [ERROR]${NC} $msg" >&2 ;;
        WARN)    echo -e "${YELLOW}[$timestamp] [WARN]${NC} $msg" ;;
        INFO)    echo -e "${BLUE}[$timestamp] [INFO]${NC} $msg" ;;
        SUCCESS) echo -e "${GREEN}[$timestamp] [OK]${NC} $msg" ;;
        DEBUG)   [ "${DEBUG:-false}" = "true" ] && echo -e "${CYAN}[$timestamp] [DEBUG]${NC} $msg" ;;
    esac
}

init_data_dir() {
    # 尝试创建数据目录
    if ! mkdir -p "$DATA_DIR"/{history,alerts,metrics} 2>/dev/null; then
        # 回退到临时目录
        DATA_DIR="/tmp/nas-os-monitor"
        mkdir -p "$DATA_DIR"/{history,alerts,metrics} 2>/dev/null || {
            log ERROR "无法创建数据目录: $DATA_DIR"
            return 1
        }
        log WARN "使用临时数据目录: $DATA_DIR"
    fi
}

#===========================================
# 系统资源监控
#===========================================

# CPU 使用率
get_cpu_usage() {
    # 使用 top 命令获取 CPU 使用率 (更可靠)
    if command -v top &>/dev/null; then
        local cpu_idle=$(top -bn1 2>/dev/null | grep -E '^%?Cpu' | awk '{print $8}' | tr -d '%')
        if [ -n "$cpu_idle" ]; then
            # 计算使用率 = 100 - idle
            echo $(awk "BEGIN {printf \"%.0f\", 100 - $cpu_idle}")
            return
        fi
    fi
    
    # 回退方案: 使用 /proc/stat
    local cpu_line=$(head -1 /proc/stat 2>/dev/null)
    if [ -z "$cpu_line" ]; then
        echo 0
        return
    fi
    
    local cpu_values=($cpu_line)
    local user=${cpu_values[1]:-0}
    local nice=${cpu_values[2]:-0}
    local system=${cpu_values[3]:-0}
    local idle=${cpu_values[4]:-0}
    
    local total=$((user + nice + system + idle))
    if [ $total -gt 0 ]; then
        local used=$((user + nice + system))
        echo $((used * 100 / total))
    else
        echo 0
    fi
}

# 内存使用率
get_memory_usage() {
    local mem_info=$(free -m 2>/dev/null | awk '/^Mem:/{print $2,$3,$4,$5,$6}')
    if [ -n "$mem_info" ]; then
        local total=$(echo "$mem_info" | awk '{print $1}')
        local used=$(echo "$mem_info" | awk '{print $2}')
        local cached=$(echo "$mem_info" | awk '{print $6}')
        
        # 实际使用 = used - cached (Linux 会用空闲内存做缓存)
        local actual_used=$((used - cached))
        
        if [ $total -gt 0 ]; then
            echo $((actual_used * 100 / total))
        else
            echo 0
        fi
    else
        echo 0
    fi
}

# 内存详情
get_memory_details() {
    free -m 2>/dev/null | awk '/^Mem:/{printf "total=%d used=%d free=%d cached=%d", $2, $3, $4, $6}'
}

# 磁盘使用率
get_disk_usage() {
    local path="${1:-/}"
    df -h "$path" 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%'
}

# 磁盘详情
get_disk_details() {
    local path="${1:-/}"
    df -h "$path" 2>/dev/null | awk 'NR==2 {printf "total=%s used=%s avail=%s pct=%s", $2, $3, $4, $5}'
}

# 系统负载
get_load_average() {
    cat /proc/loadavg 2>/dev/null | awk '{print $1}'
}

# CPU 核心数
get_cpu_cores() {
    nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo 2>/dev/null || echo 1
}

# 网络流量
get_network_stats() {
    local interface=$(ip route 2>/dev/null | awk '/default/{print $5}' | head -1)
    if [ -n "$interface" ] && [ -d "/sys/class/net/$interface/statistics" ]; then
        local rx=$(cat "/sys/class/net/$interface/statistics/rx_bytes" 2>/dev/null || echo 0)
        local tx=$(cat "/sys/class/net/$interface/statistics/tx_bytes" 2>/dev/null || echo 0)
        echo "interface=$interface rx=$rx tx=$tx"
    else
        echo "interface=unknown rx=0 tx=0"
    fi
}

# 系统运行时间
get_uptime() {
    uptime -p 2>/dev/null || uptime 2>/dev/null | awk -F'up ' '{print $2}' | awk -F',' '{print $1}'
}

#===========================================
# 服务健康检查
#===========================================

# HTTP 健康检查
check_http() {
    local name="$1"
    local url="$2"
    local timeout="${3:-3}"
    local expected_status="${4:-200}"
    
    if ! command -v curl &>/dev/null; then
        echo "status=unknown error=curl_not_found"
        return 2
    fi
    
    local start_time=$(date +%s%3N 2>/dev/null || echo 0)
    local response_code=$(curl -sf -o /dev/null -w "%{http_code}" --max-time "$timeout" "$url" 2>/dev/null || echo "000")
    local end_time=$(date +%s%3N 2>/dev/null || echo 0)
    local response_time=$((end_time - start_time))
    
    if [ "$response_code" = "$expected_status" ]; then
        echo "status=healthy response_time=${response_time}ms"
        return 0
    else
        echo "status=unhealthy response_code=$response_code response_time=${response_time}ms"
        return 1
    fi
}

# TCP 端口检查
check_tcp() {
    local name="$1"
    local host="${2:-localhost}"
    local port="$3"
    local timeout="${4:-3}"
    
    # 优先使用 nc
    if command -v nc &>/dev/null; then
        if nc -z -w "$timeout" "$host" "$port" 2>/dev/null; then
            echo "status=healthy"
            return 0
        else
            echo "status=unhealthy"
            return 1
        fi
    fi
    
    # 回退到 bash 内置的 /dev/tcp
    if (echo >/dev/tcp/"$host"/"$port") 2>/dev/null; then
        echo "status=healthy"
        return 0
    else
        echo "status=unhealthy"
        return 1
    fi
}

# 进程检查
check_process() {
    local name="$1"
    local process_name="$2"
    
    local pid=$(pgrep -x "$process_name" 2>/dev/null | head -1)
    if [ -n "$pid" ]; then
        local cpu=$(ps -o %cpu= -p "$pid" 2>/dev/null | awk '{print $1}' || echo "0")
        local mem=$(ps -o rss= -p "$pid" 2>/dev/null | awk '{printf "%.1f", $1/1024}' || echo "0")
        local uptime=$(ps -o etimes= -p "$pid" 2>/dev/null | awk '{print $1}' || echo "0")
        
        echo "status=healthy pid=$pid cpu=${cpu}% mem=${mem}MB uptime=${uptime}s"
        return 0
    else
        echo "status=unhealthy"
        return 1
    fi
}

# 数据库检查 (SQLite)
check_sqlite() {
    local name="$1"
    local db_path="$2"
    
    if [ ! -f "$db_path" ]; then
        echo "status=unhealthy error=file_not_found"
        return 1
    fi
    
    if command -v sqlite3 &>/dev/null; then
        local result=$(sqlite3 "$db_path" "PRAGMA integrity_check;" 2>&1)
        if [ "$result" = "ok" ]; then
            local size=$(du -h "$db_path" 2>/dev/null | cut -f1)
            echo "status=healthy size=$size"
            return 0
        else
            echo "status=unhealthy error=integrity_check_failed"
            return 1
        fi
    else
        echo "status=unknown error=sqlite3_not_installed"
        return 2
    fi
}

# Docker 容器检查
check_docker() {
    local name="$1"
    local container="$2"
    
    if ! command -v docker &>/dev/null; then
        echo "status=unknown error=docker_not_installed"
        return 2
    fi
    
    local status=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null || echo "unknown")
    
    if [ "$status" = "running" ]; then
        local health=$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || echo "none")
        echo "status=healthy docker_status=$status health=$health"
        return 0
    else
        echo "status=unhealthy docker_status=$status"
        return 1
    fi
}

#===========================================
# 服务配置加载
#===========================================

load_services_config() {
    # 如果配置文件存在，加载它
    if [ -f "$SERVICES_CONFIG" ]; then
        source "$SERVICES_CONFIG"
    else
        # 默认服务配置
        log INFO "使用默认服务配置"
    fi
}

# 默认服务检查
check_default_services() {
    local services_status=()
    
    # 检查 NAS-OS 主进程
    if pgrep -x nasd &>/dev/null; then
        local result=$(check_process "nasd" "nasd")
        services_status+=("nasd:$result")
    else
        services_status+=("nasd:status=not_running")
        add_alert "nasd 进程未运行"
    fi
    
    # 检查 API 端口
    local api_port="${API_PORT:-8080}"
    local result=$(check_tcp "api" "localhost" "$api_port")
    services_status+=("api:$result")
    if [[ "$result" == *"unhealthy"* ]]; then
        add_alert "API 端口 $api_port 不可达"
    fi
    
    # 检查健康端点
    result=$(check_http "health" "http://localhost:${api_port}/api/v1/health")
    services_status+=("health:$result")
    if [[ "$result" == *"unhealthy"* ]]; then
        add_alert "健康检查端点返回异常"
    fi
    
    # 检查数据库
    local db_path="/var/lib/nas-os/nas-os.db"
    if [ -f "$db_path" ]; then
        result=$(check_sqlite "database" "$db_path")
        services_status+=("database:$result")
    fi
    
    # 输出服务状态
    for service in "${services_status[@]}"; do
        log INFO "服务检查: $service"
    done
}

#===========================================
# 告警系统
#===========================================

add_alert() {
    local message="$1"
    local severity="${2:-warning}"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    ALERTS+=("$timestamp [$severity] $message")
}

send_alerts() {
    if [ "$ALERT_ENABLED" != "true" ] || [ ${#ALERTS[@]} -eq 0 ]; then
        return 0
    fi
    
    local hostname=$(hostname)
    local timestamp=$(date -Iseconds)
    
    # 构建告警消息
    local title="NAS-OS 监控告警"
    local body="主机: $hostname\n时间: $timestamp\n\n告警内容:\n"
    for alert in "${ALERTS[@]}"; do
        body="$body- $alert\n"
    done
    
    # 检查冷却时间
    local alert_key=$(echo "${ALERTS[*]}" | md5sum | cut -d' ' -f1)
    local last_alert_file="$DATA_DIR/alerts/last_${alert_key}"
    
    if [ -f "$last_alert_file" ]; then
        local last_time=$(cat "$last_alert_file" 2>/dev/null || echo "0")
        local now=$(date +%s)
        if [ $((now - last_time)) -lt "$ALERT_COOLDOWN" ]; then
            log DEBUG "告警冷却中，跳过发送"
            return 0
        fi
    fi
    
    # 发送 Webhook
    if [ -n "$ALERT_WEBHOOK" ]; then
        curl -sf -X POST -H "Content-Type: application/json" \
            -d "{\"title\": \"$title\", \"body\": \"$body\", \"hostname\": \"$hostname\", \"timestamp\": \"$timestamp\"}" \
            "$ALERT_WEBHOOK" 2>/dev/null && log INFO "Webhook 告警已发送" || true
    fi
    
    # 发送钉钉
    if [ -n "$ALERT_DINGTALK" ]; then
        curl -sf -X POST -H "Content-Type: application/json" \
            -d "{\"msgtype\": \"text\", \"text\": {\"content\": \"$title\n$body\"}}" \
            "$ALERT_DINGTALK" 2>/dev/null && log INFO "钉钉告警已发送" || true
    fi
    
    # 发送飞书
    if [ -n "$ALERT_FEISHU" ]; then
        curl -sf -X POST -H "Content-Type: application/json" \
            -d "{\"msg_type\": \"text\", \"content\": {\"text\": \"$title\n$body\"}}" \
            "$ALERT_FEISHU" 2>/dev/null && log INFO "飞书告警已发送" || true
    fi
    
    # 发送邮件
    if [ -n "$ALERT_EMAIL" ] && command -v mailx &>/dev/null; then
        echo -e "$body" | mailx -s "$title" "$ALERT_EMAIL" 2>/dev/null && log INFO "邮件告警已发送" || true
    fi
    
    # 记录发送时间
    date +%s > "$last_alert_file"
    
    # 保存告警历史
    for alert in "${ALERTS[@]}"; do
        echo "$alert" >> "$DATA_DIR/alerts/history.log"
    done
}

#===========================================
# 历史数据管理
#===========================================

save_metrics() {
    local timestamp=$(date +%s)
    local date_str=$(date '+%Y-%m-%d')
    local metrics_file="$DATA_DIR/history/metrics_${date_str}.json"
    
    # 写入指标
    local record="{\"timestamp\": $timestamp, \"metrics\": $METRICS}"
    echo "$record" >> "$metrics_file"
    
    # 清理旧数据
    find "$DATA_DIR/history" -name "metrics_*.json" -mtime +$HISTORY_RETENTION -delete 2>/dev/null || true
}

get_trend() {
    local metric_name="$1"
    local hours="${2:-24}"
    
    local now=$(date +%s)
    local start=$((now - hours * 3600))
    local total=0
    local count=0
    local max=0
    local min=999999
    
    for file in "$DATA_DIR/history"/metrics_*.json; do
        [ -f "$file" ] || continue
        
        while IFS= read -r line; do
            local ts=$(echo "$line" | grep -o '"timestamp": [0-9]*' | grep -o '[0-9]*')
            [ -z "$ts" ] && continue
            
            if [ "$ts" -ge "$start" ]; then
                local value=$(echo "$line" | grep -o "\"$metric_name\": *[0-9]*" | grep -o '[0-9]*')
                [ -z "$value" ] && continue
                
                total=$((total + value))
                count=$((count + 1))
                [ "$value" -gt "$max" ] && max=$value
                [ "$value" -lt "$min" ] && min=$value
            fi
        done < "$file"
    done
    
    if [ $count -gt 0 ]; then
        local avg=$((total / count))
        echo "avg=$avg max=$max min=$min samples=$count"
    else
        echo "avg=0 max=0 min=0 samples=0"
    fi
}

#===========================================
# Prometheus 指标导出
#===========================================

export_prometheus() {
    local cpu_usage=$(get_cpu_usage)
    local memory_usage=$(get_memory_usage)
    local disk_usage=$(get_disk_usage)
    local load_avg=$(get_load_average)
    local cpu_cores=$(get_cpu_cores)
    local uptime=$(get_uptime | tr -d ',')
    
    cat <<EOF
# HELP nas_os_monitor_version Monitor version info
# TYPE nas_os_monitor_version gauge
nas_os_monitor_version{version="$VERSION"} 1

# HELP nas_os_cpu_usage_percent CPU usage percentage
# TYPE nas_os_cpu_usage_percent gauge
nas_os_cpu_usage_percent $cpu_usage

# HELP nas_os_memory_usage_percent Memory usage percentage
# TYPE nas_os_memory_usage_percent gauge
nas_os_memory_usage_percent $memory_usage

# HELP nas_os_disk_usage_percent Disk usage percentage
# TYPE nas_os_disk_usage_percent gauge
nas_os_disk_usage_percent{path="/"} $disk_usage

# HELP nas_os_load_average System load average
# TYPE nas_os_load_average gauge
nas_os_load_average $load_avg

# HELP nas_os_cpu_cores Number of CPU cores
# TYPE nas_os_cpu_cores gauge
nas_os_cpu_cores $cpu_cores

# HELP nas_os_alerts_total Total number of alerts
# TYPE nas_os_alerts_total gauge
nas_os_alerts_total ${#ALERTS[@]}
EOF
}

start_prometheus_server() {
    if ! command -v nc &>/dev/null; then
        log WARN "nc 不可用，无法启动 Prometheus 服务器"
        return 1
    fi
    
    log INFO "启动 Prometheus 指标服务器端口 $PROMETHEUS_PORT"
    
    while true; do
        {
            echo -e "HTTP/1.1 200 OK\r"
            echo -e "Content-Type: text/plain; version=0.0.4\r"
            echo -e "\r"
            export_prometheus
        } | nc -l -p "$PROMETHEUS_PORT" -q 1 2>/dev/null || sleep 1
    done &
}

#===========================================
# 主监控循环
#===========================================

collect_metrics() {
    ALERTS=()
    
    log INFO "开始收集监控数据..."
    
    # 系统资源
    local cpu_usage=$(get_cpu_usage)
    local memory_usage=$(get_memory_usage)
    local disk_usage=$(get_disk_usage "/")
    local disk_data=$(get_disk_details "/")
    local load_avg=$(get_load_average)
    local cpu_cores=$(get_cpu_cores)
    local network=$(get_network_stats)
    local uptime=$(get_uptime)
    
    # 构建指标 JSON
    METRICS=$(cat <<EOF
{
    "cpu_usage": $cpu_usage,
    "memory_usage": $memory_usage,
    "disk_usage": $disk_usage,
    "load_average": $load_avg,
    "cpu_cores": $cpu_cores,
    "network": "$network",
    "uptime": "$uptime"
}
EOF
)
    
    # CPU 检查
    if [ "$cpu_usage" -ge "$CPU_THRESHOLD_CRITICAL" ]; then
        add_alert "CPU 使用率严重: ${cpu_usage}%" "critical"
    elif [ "$cpu_usage" -ge "$CPU_THRESHOLD_WARNING" ]; then
        add_alert "CPU 使用率警告: ${cpu_usage}%" "warning"
    fi
    
    # 内存检查
    if [ "$memory_usage" -ge "$MEMORY_THRESHOLD_CRITICAL" ]; then
        add_alert "内存使用率严重: ${memory_usage}%" "critical"
    elif [ "$memory_usage" -ge "$MEMORY_THRESHOLD_WARNING" ]; then
        add_alert "内存使用率警告: ${memory_usage}%" "warning"
    fi
    
    # 磁盘检查
    if [ "$disk_usage" -ge "$DISK_THRESHOLD_CRITICAL" ]; then
        add_alert "磁盘使用率严重: ${disk_usage}%" "critical"
    elif [ "$disk_usage" -ge "$DISK_THRESHOLD_WARNING" ]; then
        add_alert "磁盘使用率警告: ${disk_usage}%" "warning"
    fi
    
    # 负载检查
    local load_threshold_warning=$(echo "$LOAD_THRESHOLD_WARNING * $cpu_cores" | bc -l)
    local load_threshold_critical=$(echo "$LOAD_THRESHOLD_CRITICAL * $cpu_cores" | bc -l)
    
    if (( $(echo "$load_avg > $load_threshold_critical" | bc -l) )); then
        add_alert "系统负载严重: ${load_avg}" "critical"
    elif (( $(echo "$load_avg > $load_threshold_warning" | bc -l) )); then
        add_alert "系统负载警告: ${load_avg}" "warning"
    fi
    
    # 服务检查
    check_default_services
    
    # 保存指标
    save_metrics
    
    # 发送告警
    send_alerts
    
    # 输出状态
    echo ""
    echo "==================================="
    echo "NAS-OS 监控报告"
    echo "时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "==================================="
    echo ""
    echo "系统资源:"
    echo "  CPU 使用率:    ${cpu_usage}%"
    echo "  内存使用率:    ${memory_usage}%"
    echo "  磁盘使用率:    ${disk_usage}%"
    echo "  系统负载:      ${load_avg} (${cpu_cores} 核心)"
    echo "  运行时间:      ${uptime}"
    echo ""
    echo "网络: ${network}"
    echo ""
    
    if [ ${#ALERTS[@]} -gt 0 ]; then
        echo -e "${RED}告警 (${#ALERTS[@]}):${NC}"
        for alert in "${ALERTS[@]}"; do
            echo "  - $alert"
        done
        echo ""
    fi
    
    log SUCCESS "监控数据收集完成"
}

#===========================================
# 运行模式
#===========================================

run_once() {
    init_data_dir
    collect_metrics
}

run_daemon() {
    init_data_dir
    
    # 写入 PID 文件
    echo $$ > "$PID_FILE"
    
    # 信号处理
    trap 'log INFO "收到退出信号"; rm -f "$PID_FILE"; exit 0' SIGTERM SIGINT
    
    log INFO "启动守护进程模式 (间隔: ${MONITOR_INTERVAL}s, PID: $$)"
    
    # 可选: 启动 Prometheus 服务器
    if [ "${ENABLE_PROMETHEUS:-false}" = "true" ]; then
        start_prometheus_server
    fi
    
    while true; do
        collect_metrics
        sleep "$MONITOR_INTERVAL"
    done
}

run_cron() {
    # Cron 模式：单次运行，适合放在 crontab 中
    init_data_dir
    collect_metrics
    
    # 如果有告警，返回非零退出码
    if [ ${#ALERTS[@]} -gt 0 ]; then
        return 1
    fi
    return 0
}

show_status() {
    echo "NAS-OS 监控状态 v$VERSION"
    echo ""
    
    # 检查守护进程
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo "守护进程: 运行中 (PID: $pid)"
        else
            echo "守护进程: 已停止 (PID 文件存在但进程不存在)"
        fi
    else
        echo "守护进程: 未运行"
    fi
    
    echo ""
    
    # 显示最近指标
    local today=$(date '+%Y-%m-%d')
    local metrics_file="$DATA_DIR/history/metrics_${today}.json"
    
    if [ -f "$metrics_file" ]; then
        local last_record=$(tail -1 "$metrics_file")
        echo "最近监控数据:"
        echo "$last_record" | python3 -m json.tool 2>/dev/null || echo "$last_record"
    else
        echo "无最近监控数据"
    fi
    
    echo ""
    
    # 显示趋势
    echo "过去 24 小时趋势:"
    echo "  CPU:  $(get_trend "cpu_usage" 24)"
    echo "  内存: $(get_trend "memory_usage" 24)"
    echo "  磁盘: $(get_trend "disk_usage" 24)"
    
    echo ""
    
    # 显示最近告警
    local alert_file="$DATA_DIR/alerts/history.log"
    if [ -f "$alert_file" ]; then
        echo "最近告警:"
        tail -5 "$alert_file"
    else
        echo "无告警记录"
    fi
}

stop_daemon() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            rm -f "$PID_FILE"
            log SUCCESS "守护进程已停止 (PID: $pid)"
        else
            rm -f "$PID_FILE"
            log WARN "PID 文件存在但进程已不存在"
        fi
    else
        log WARN "守护进程未运行"
    fi
}

#===========================================
# 帮助信息
#===========================================

show_help() {
    cat <<EOF
NAS-OS 系统监控脚本 v$VERSION

用法: $0 [选项] [命令]

命令:
  (无参数)        单次检查
  --daemon        守护进程模式 (持续监控)
  --stop          停止守护进程
  --status        显示监控状态
  --export        导出 Prometheus 指标
  --cron          cron 任务模式
  --help, -h      显示帮助

选项:
  --interval N    设置监控间隔 (秒, 默认: 60)
  --prometheus    在守护进程模式下启动 Prometheus 指标服务器
  --json          JSON 格式输出

环境变量:
  MONITOR_INTERVAL          监控间隔 (秒)
  CPU_THRESHOLD_WARNING     CPU 警告阈值 (默认: 80%)
  CPU_THRESHOLD_CRITICAL    CPU 严重阈值 (默认: 95%)
  MEMORY_THRESHOLD_WARNING  内存警告阈值 (默认: 80%)
  MEMORY_THRESHOLD_CRITICAL 内存严重阈值 (默认: 95%)
  DISK_THRESHOLD_WARNING    磁盘警告阈值 (默认: 80%)
  DISK_THRESHOLD_CRITICAL   磁盘严重阈值 (默认: 95%)
  ALERT_ENABLED             启用告警 (默认: true)
  ALERT_WEBHOOK             告警 Webhook URL
  ALERT_EMAIL               告警邮件地址
  ALERT_DINGTALK             钉钉 Webhook
  ALERT_FEISHU              飞书 Webhook
  DATA_DIR                  数据目录
  PROMETHEUS_PORT           Prometheus 指标端口 (默认: 9101)

服务配置文件:
  $SERVICES_CONFIG
  
  格式示例:
    # HTTP 服务检查
    HTTP_CHECKS=(
      "name:url:timeout:expected_status"
      "api:http://localhost:8080/health:5:200"
    )
    
    # TCP 端口检查
    TCP_CHECKS=(
      "name:host:port:timeout"
      "ssh:localhost:22:5"
    )
    
    # 进程检查
    PROCESS_CHECKS=(
      "name:process_name"
      "nasd:nasd"
    )
    
    # Docker 容器检查
    DOCKER_CHECKS=(
      "name:container_name"
      "webui:nas-os-webui"
    )

示例:
  # 单次检查
  $0
  
  # 守护进程模式
  $0 --daemon
  
  # 设置监控间隔为 30 秒
  MONITOR_INTERVAL=30 $0 --daemon
  
  # 显示状态
  $0 --status
  
  # 导出 Prometheus 指标
  $0 --export
  
  # 配置告警
  ALERT_WEBHOOK=https://hooks.slack.com/xxx $0 --daemon

Cron 配置:
  # 每 5 分钟检查一次
  */5 * * * * /path/to/monitor.sh --cron >> /var/log/nas-os/monitor.log 2>&1

EOF
}

#===========================================
# 主入口
#===========================================

main() {
    local mode="once"
    local json_output=false
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --daemon)
                mode="daemon"
                shift
                ;;
            --stop)
                mode="stop"
                shift
                ;;
            --status)
                mode="status"
                shift
                ;;
            --export)
                mode="export"
                shift
                ;;
            --cron)
                mode="cron"
                shift
                ;;
            --help|-h)
                mode="help"
                shift
                ;;
            --interval)
                MONITOR_INTERVAL="$2"
                shift 2
                ;;
            --prometheus)
                ENABLE_PROMETHEUS=true
                shift
                ;;
            --json)
                json_output=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done
    
    # 加载服务配置
    load_services_config
    
    case "$mode" in
        once)
            run_once
            ;;
        daemon)
            run_daemon
            ;;
        stop)
            stop_daemon
            ;;
        status)
            show_status
            ;;
        export)
            export_prometheus
            ;;
        cron)
            run_cron
            ;;
        help)
            show_help
            ;;
        *)
            run_once
            ;;
    esac
}

main "$@"