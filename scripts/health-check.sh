#!/bin/bash
# NAS-OS 健康检查脚本
# 用于监控和诊断服务状态
#
# v2.44.0 更新（工部优化）：
# - 添加 JSON 输出格式支持
# - 添加告警通知功能
# - 添加 Prometheus 指标输出
# - 增加更多检查项（连接池、API 响应时间）
# - 添加历史记录和趋势分析
# - 支持静默模式和退出码
#
# v2.38.0 新增

set -e

# 配置
API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-8080}"
API_URL="http://${API_HOST}:${API_PORT}"

# 输出格式
OUTPUT_FORMAT="${OUTPUT_FORMAT:-text}"  # text, json, prometheus

# 告警配置
ENABLE_ALERTS="${ENABLE_ALERTS:-false}"
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_EMAIL="${ALERT_EMAIL:-}"

# 阈值配置
DISK_THRESHOLD_WARNING="${DISK_THRESHOLD_WARNING:-80}"
DISK_THRESHOLD_CRITICAL="${DISK_THRESHOLD_CRITICAL:-90}"
MEMORY_THRESHOLD_WARNING="${MEMORY_THRESHOLD_WARNING:-80}"
MEMORY_THRESHOLD_CRITICAL="${MEMORY_THRESHOLD_CRITICAL:-90}"
API_TIMEOUT_MS="${API_TIMEOUT_MS:-5000}"

# 历史记录
HISTORY_DIR="${HISTORY_DIR:-/var/lib/nas-os/health-history}"
HISTORY_RETENTION="${HISTORY_RETENTION:-7}"  # 天

# 静默模式
SILENT="${SILENT:-false}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 检查结果
HEALTHY=true
CHECK_RESULTS=()
ALERTS=()

# 日志函数
log_info() {
    [ "$SILENT" = true ] || echo -e "${BLUE}[INFO]${NC} $1"
}

log_healthy() {
    [ "$SILENT" = true ] || echo -e "${GREEN}[HEALTHY]${NC} $1"
    CHECK_RESULTS+=("healthy:$1")
}

log_unhealthy() {
    [ "$SILENT" = true ] || echo -e "${RED}[UNHEALTHY]${NC} $1"
    HEALTHY=false
    CHECK_RESULTS+=("unhealthy:$1")
    ALERTS+=("$1")
}

log_warn() {
    [ "$SILENT" = true ] || echo -e "${YELLOW}[WARN]${NC} $1"
    CHECK_RESULTS+=("warn:$1")
}

# JSON 输出函数
output_json() {
    local status="healthy"
    [ "$HEALTHY" = false ] && status="unhealthy"
    
    echo "{"
    echo "  \"status\": \"$status\","
    echo "  \"timestamp\": \"$(date -Iseconds)\","
    echo "  \"hostname\": \"$(hostname)\","
    echo "  \"checks\": ["
    local first=true
    for result in "${CHECK_RESULTS[@]}"; do
        level=$(echo "$result" | cut -d: -f1)
        msg=$(echo "$result" | cut -d: -f2-)
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        echo -n "    {\"level\": \"$level\", \"message\": \"$msg\"}"
    done
    echo ""
    echo "  ],"
    echo "  \"alerts\": ["
    local first=true
    for alert in "${ALERTS[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        echo -n "    \"$alert\""
    done
    echo ""
    echo "  ]"
    echo "}"
}

# Prometheus 指标输出
output_prometheus() {
    local status=1
    [ "$HEALTHY" = false ] && status=0
    
    echo "# HELP nas_os_health_status Overall health status (1=healthy, 0=unhealthy)"
    echo "# TYPE nas_os_health_status gauge"
    echo "nas_os_health_status $status"
    
    echo "# HELP nas_os_health_checks_total Total number of health checks performed"
    echo "# TYPE nas_os_health_checks_total counter"
    echo "nas_os_health_checks_total ${#CHECK_RESULTS[@]}"
    
    echo "# HELP nas_os_health_alerts Number of active alerts"
    echo "# TYPE nas_os_health_alerts gauge"
    echo "nas_os_health_alerts ${#ALERTS[@]}"
}

# 发送告警
send_alerts() {
    if [ "$ENABLE_ALERTS" != true ] || [ ${#ALERTS[@]} -eq 0 ]; then
        return 0
    fi
    
    local message="NAS-OS 健康告警: ${#ALERTS[@]} 个问题"
    for alert in "${ALERTS[@]}"; do
        message="$message\n- $alert"
    done
    
    # Webhook 告警
    if [ -n "$ALERT_WEBHOOK" ]; then
        curl -sf -X POST -H "Content-Type: application/json" \
            -d "{\"text\": \"$message\", \"timestamp\": \"$(date -Iseconds)\"}" \
            "$ALERT_WEBHOOK" 2>/dev/null || true
    fi
    
    # 邮件告警
    if [ -n "$ALERT_EMAIL" ] && command -v mailx &> /dev/null; then
        echo -e "$message" | mailx -s "NAS-OS 健康告警" "$ALERT_EMAIL" 2>/dev/null || true
    fi
}

# 保存历史记录
save_history() {
    mkdir -p "$HISTORY_DIR" 2>/dev/null || return 0
    local history_file="${HISTORY_DIR}/health-$(date +%Y%m%d).json"
    
    local healthy_str="true"
    [ "$HEALTHY" = false ] && healthy_str="false"
    
    echo "{\"time\": \"$(date -Iseconds)\", \"healthy\": $healthy_str, \"alerts\": ${#ALERTS[@]}}" >> "$history_file" 2>/dev/null || true
    
    # 清理旧记录
    find "$HISTORY_DIR" -name "health-*.json" -mtime +$HISTORY_RETENTION -delete 2>/dev/null || true
}

# 检查进程
check_process() {
    log_info "检查进程..."
    
    if pgrep -x nasd > /dev/null 2>&1; then
        local pid=$(pgrep -x nasd)
        local mem=$(ps -o rss= -p $pid 2>/dev/null | awk '{printf "%.1f MB", $1/1024}')
        local cpu=$(ps -o %cpu= -p $pid 2>/dev/null | awk '{printf "%.1f%%", $1}')
        log_healthy "进程运行中 (PID: $pid, 内存: $mem, CPU: $cpu)"
    else
        log_unhealthy "进程未运行"
    fi
}

# 检查 API 健康端点
check_api_health() {
    log_info "检查 API 健康端点..."
    
    local start_time=$(date +%s%3N 2>/dev/null || echo "0")
    local response=$(curl -sf --max-time 10 "${API_URL}/api/v1/health" 2>/dev/null)
    local exit_code=$?
    local end_time=$(date +%s%3N 2>/dev/null || echo "0")
    local response_time=$((end_time - start_time))
    
    if [ $exit_code -eq 0 ]; then
        local status=$(echo "$response" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        if [ "$status" = "healthy" ]; then
            if [ $response_time -gt $API_TIMEOUT_MS ]; then
                log_warn "API 健康: $status (响应时间: ${response_time}ms > ${API_TIMEOUT_MS}ms)"
            else
                log_healthy "API 健康: $status (响应时间: ${response_time}ms)"
            fi
        else
            log_warn "API 状态: $status"
        fi
    else
        log_unhealthy "API 健康端点不可达"
    fi
}

# 检查 API 响应时间
check_api_response_time() {
    log_info "检查 API 响应时间..."
    
    local endpoints=(
        "/api/v1/health"
        "/api/v1/system/status"
        "/api/v1/metrics"
    )
    
    local total_time=0
    local count=0
    local slow_endpoints=""
    
    for endpoint in "${endpoints[@]}"; do
        local start=$(date +%s%3N 2>/dev/null || echo "0")
        if curl -sf --max-time 5 "${API_URL}${endpoint}" > /dev/null 2>&1; then
            local end=$(date +%s%3N 2>/dev/null || echo "0")
            local time=$((end - start))
            total_time=$((total_time + time))
            count=$((count + 1))
            
            if [ $time -gt $API_TIMEOUT_MS ]; then
                slow_endpoints="$slow_endpoints $endpoint(${time}ms)"
            fi
        fi
    done
    
    if [ $count -gt 0 ]; then
        local avg_time=$((total_time / count))
        if [ -n "$slow_endpoints" ]; then
            log_warn "慢接口: $slow_endpoints (平均: ${avg_time}ms)"
        else
            log_healthy "API 平均响应时间: ${avg_time}ms"
        fi
    else
        log_warn "无法测量 API 响应时间"
    fi
}

# 检查系统状态 API
check_system_status() {
    log_info "检查系统状态..."
    
    local response=$(curl -sf "${API_URL}/api/v1/system/status" 2>/dev/null)
    
    if [ $? -eq 0 ]; then
        local version=$(echo "$response" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
        local uptime=$(echo "$response" | grep -o '"uptime":"[^"]*"' | cut -d'"' -f4)
        log_healthy "版本: $version, 运行时间: $uptime"
    else
        log_warn "系统状态 API 不可达"
    fi
}

# 检查端口
check_ports() {
    log_info "检查端口..."
    
    # Web UI
    if ss -tuln 2>/dev/null | grep -q ":${API_PORT} "; then
        log_healthy "端口 $API_PORT (Web UI) 监听中"
    else
        log_unhealthy "端口 $API_PORT (Web UI) 未监听"
    fi
    
    # SMB
    if ss -tuln 2>/dev/null | grep -q ":445 "; then
        log_healthy "端口 445 (SMB) 监听中"
    else
        log_warn "端口 445 (SMB) 未监听"
    fi
    
    # NFS
    if ss -tuln 2>/dev/null | grep -q ":2049 "; then
        log_healthy "端口 2049 (NFS) 监听中"
    else
        log_warn "端口 2049 (NFS) 未监听"
    fi
}

# 检查磁盘空间
check_disk_space() {
    log_info "检查磁盘空间..."
    
    local data_dir="/var/lib/nas-os"
    
    if [ -d "$data_dir" ]; then
        local usage=$(df -h "$data_dir" 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
        local avail=$(df -h "$data_dir" 2>/dev/null | awk 'NR==2 {print $4}')
        
        if [ -n "$usage" ]; then
            if [ $usage -lt $DISK_THRESHOLD_WARNING ]; then
                log_healthy "磁盘使用: ${usage}% (可用: $avail)"
            elif [ $usage -lt $DISK_THRESHOLD_CRITICAL ]; then
                log_warn "磁盘使用: ${usage}% (可用: $avail)"
            else
                log_unhealthy "磁盘使用: ${usage}% (可用: $avail)"
            fi
        else
            log_warn "无法获取磁盘使用信息"
        fi
    else
        log_warn "数据目录不存在: $data_dir"
    fi
}

# 检查数据库
check_database() {
    log_info "检查数据库..."
    
    local db_path="/var/lib/nas-os/nas-os.db"
    
    if [ -f "$db_path" ]; then
        if command -v sqlite3 &> /dev/null; then
            if sqlite3 "$db_path" "PRAGMA integrity_check;" 2>/dev/null | grep -q "ok"; then
                local size=$(du -h "$db_path" 2>/dev/null | cut -f1)
                log_healthy "数据库正常 (大小: $size)"
            else
                log_unhealthy "数据库完整性检查失败"
            fi
        else
            log_warn "sqlite3 未安装，跳过数据库检查"
        fi
    else
        log_warn "数据库文件不存在: $db_path"
    fi
}

# 检查日志
check_logs() {
    log_info "检查最近日志..."
    
    if command -v journalctl &> /dev/null; then
        local errors=$(journalctl -u nas-os --no-pager -n 100 2>/dev/null | grep -i "error" | wc -l)
        local warns=$(journalctl -u nas-os --no-pager -n 100 2>/dev/null | grep -i "warn" | wc -l)
        
        if [ $errors -gt 0 ]; then
            log_warn "最近 100 条日志中有 $errors 个错误"
        fi
        
        if [ $warns -gt 0 ]; then
            log_info "最近 100 条日志中有 $warns 个警告"
        fi
        
        if [ $errors -eq 0 ] && [ $warns -eq 0 ]; then
            log_healthy "日志无错误或警告"
        fi
    else
        log_warn "journalctl 不可用，跳过日志检查"
    fi
}

# 检查系统资源
check_resources() {
    log_info "检查系统资源..."
    
    # 内存
    local mem_total=$(free -m 2>/dev/null | awk '/^Mem:/{print $2}')
    local mem_used=$(free -m 2>/dev/null | awk '/^Mem:/{print $3}')
    
    if [ -n "$mem_total" ] && [ -n "$mem_used" ]; then
        local mem_pct=$((mem_used * 100 / mem_total))
        
        if [ $mem_pct -lt $MEMORY_THRESHOLD_WARNING ]; then
            log_healthy "内存使用: ${mem_pct}% (${mem_used}MB / ${mem_total}MB)"
        elif [ $mem_pct -lt $MEMORY_THRESHOLD_CRITICAL ]; then
            log_warn "内存使用: ${mem_pct}% (${mem_used}MB / ${mem_total}MB)"
        else
            log_unhealthy "内存使用: ${mem_pct}% (${mem_used}MB / ${mem_total}MB)"
        fi
    fi
    
    # CPU 负载
    local load=$(cat /proc/loadavg 2>/dev/null | awk '{print $1}')
    local cores=$(nproc 2>/dev/null || echo "1")
    
    if [ -n "$load" ]; then
        if (( $(echo "$load < $cores" | bc -l 2>/dev/null || echo "1") )); then
            log_healthy "CPU 负载: $load (核心数: $cores)"
        else
            log_warn "CPU 负载: $load (核心数: $cores)"
        fi
    fi
}

# 快速检查（仅检查关键项）
quick_check() {
    check_process
    check_api_health
    check_ports
}

# 完整检查
full_check() {
    local start_time=$(date +%s)
    
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo ""
        echo "==================================="
        echo "NAS-OS 健康检查 v2.44.0"
        echo "==================================="
        echo ""
    fi
    
    check_process
    check_api_health
    check_api_response_time
    check_system_status
    check_ports
    check_disk_space
    check_database
    check_resources
    check_logs
    
    # 保存历史
    save_history
    
    # 发送告警
    send_alerts
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        output_json
    elif [ "$OUTPUT_FORMAT" = "prometheus" ]; then
        output_prometheus
    else
        echo ""
        echo "==================================="
        echo "检查耗时: ${duration}s"
        
        if [ "$HEALTHY" = true ]; then
            echo -e "${GREEN}状态: 健康${NC}"
        else
            echo -e "${RED}状态: 不健康${NC}"
            if [ ${#ALERTS[@]} -gt 0 ]; then
                echo ""
                echo "告警项:"
                for alert in "${ALERTS[@]}"; do
                    echo "  - $alert"
                done
            fi
        fi
    fi
    
    if [ "$HEALTHY" = true ]; then
        exit 0
    else
        exit 1
    fi
}

# 监控模式（持续检查）
monitor_mode() {
    local interval="${1:-30}"
    
    log_info "进入监控模式 (间隔: ${interval}s, Ctrl+C 退出)"
    
    while true; do
        clear
        echo "$(date '+%Y-%m-%d %H:%M:%S')"
        echo ""
        quick_check
        echo ""
        sleep "$interval"
    done
}

# 帮助信息
show_help() {
    cat <<EOF
NAS-OS 健康检查工具 v2.44.0

用法: $0 <command> [options]

命令:
  quick      快速检查（进程、API、端口）
  full       完整检查
  monitor    持续监控模式
  help       显示帮助

选项:
  --json           JSON 格式输出
  --prometheus     Prometheus 指标格式输出
  --silent         静默模式（仅退出码）
  --alert          启用告警通知

环境变量:
  API_HOST                  API 主机 (默认: localhost)
  API_PORT                  API 端口 (默认: 8080)
  OUTPUT_FORMAT             输出格式 (text/json/prometheus)
  DISK_THRESHOLD_WARNING    磁盘警告阈值 (默认: 80%)
  DISK_THRESHOLD_CRITICAL   磁盘严重阈值 (默认: 90%)
  MEMORY_THRESHOLD_WARNING  内存警告阈值 (默认: 80%)
  MEMORY_THRESHOLD_CRITICAL 内存严重阈值 (默认: 90%)
  API_TIMEOUT_MS            API 响应超时 (默认: 5000ms)
  ALERT_WEBHOOK             告警 Webhook URL
  ALERT_EMAIL               告警邮件地址

示例:
  $0 quick
  $0 full
  $0 full --json
  $0 monitor 60
  OUTPUT_FORMAT=prometheus $0 full

退出码:
  0 - 健康
  1 - 不健康
EOF
}

# 解析参数
CMD=""
ARGS=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --json)
            OUTPUT_FORMAT="json"
            SILENT=true
            shift
            ;;
        --prometheus)
            OUTPUT_FORMAT="prometheus"
            SILENT=true
            shift
            ;;
        --silent)
            SILENT=true
            shift
            ;;
        --alert)
            ENABLE_ALERTS=true
            shift
            ;;
        quick|full|monitor|help|-h|--help)
            CMD="$1"
            shift
            ;;
        *)
            ARGS+=("$1")
            shift
            ;;
    esac
done

# 主入口
case "${CMD:-}" in
    quick)
        quick_check
        ;;
    full)
        full_check
        ;;
    monitor)
        monitor_mode "${ARGS[0]:-30}"
        ;;
    -h|--help|help)
        show_help
        ;;
    "")
        full_check
        ;;
    *)
        show_help
        exit 1
        ;;
esac