#!/bin/bash
# =============================================================================
# NAS-OS 服务监控脚本 v2.56.0
# =============================================================================
# 用途：监控 NAS-OS 核心服务，自动重启异常服务，记录日志
# 用法：./service-monitor.sh [--daemon] [--interval SECONDS] [--dry-run]
# =============================================================================

set -euo pipefail

# 版本
VERSION="2.56.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 配置
# =============================================================================

# 监控间隔（秒）
MONITOR_INTERVAL="${MONITOR_INTERVAL:-60}"

# 数据目录
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"

# 日志目录
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
LOG_FILE="${LOG_FILE:-$LOG_DIR/service-monitor.log}"

# PID 文件
PID_FILE="${PID_FILE:-/var/run/nas-os-service-monitor.pid}"

# 最大重启次数（同一服务在 1 小时内）
MAX_RESTART_COUNT="${MAX_RESTART_COUNT:-3}"

# 重启冷却时间（秒）
RESTART_COOLDOWN="${RESTART_COOLDOWN:-300}"

# 检查超时（秒）
CHECK_TIMEOUT="${CHECK_TIMEOUT:-10}"

# 干运行模式（不执行重启）
DRY_RUN="${DRY_RUN:-false}"

# 启用自动重启
AUTO_RESTART="${AUTO_RESTART:-true}"

# 告警配置
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_EMAIL="${ALERT_EMAIL:-}"

# API 配置
API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-8080}"
API_URL="http://${API_HOST}:${API_PORT}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# =============================================================================
# 服务定义
# =============================================================================

# 核心服务列表
# 格式：名称|类型|检查方式|启动命令|描述
SERVICES=(
    "nasd|process|nasd|systemctl restart nasd|NAS-OS 主服务"
    "api|http|${API_URL}/api/v1/health|systemctl restart nasd|API 服务"
    "smb|port|445|systemctl restart smbd|SMB 文件共享"
    "nfs|port|2049|systemctl restart nfs-server|NFS 文件共享"
)

# =============================================================================
# 全局变量
# =============================================================================

RESTART_HISTORY_DIR="$DATA_DIR/monitor/restarts"
mkdir -p "$RESTART_HISTORY_DIR" 2>/dev/null || true

# =============================================================================
# 日志函数
# =============================================================================

log() {
    local level="$1"
    shift
    local msg="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    # 确保日志目录存在
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
    
    # 写入日志文件
    echo "[$timestamp] [$level] $msg" >> "$LOG_FILE" 2>/dev/null || true
    
    # 控制台输出
    case "$level" in
        ERROR)   echo -e "${RED}[$timestamp] [ERROR]${NC} $msg" >&2 ;;
        WARN)    echo -e "${YELLOW}[$timestamp] [WARN]${NC} $msg" ;;
        INFO)    echo -e "${BLUE}[$timestamp] [INFO]${NC} $msg" ;;
        SUCCESS) echo -e "${GREEN}[$timestamp] [OK]${NC} $msg" ;;
    esac
}

# =============================================================================
# 服务检查函数
# =============================================================================

# 检查进程
check_process() {
    local process_name="$1"
    
    if pgrep -x "$process_name" > /dev/null 2>&1; then
        local pid=$(pgrep -x "$process_name" | head -1)
        return 0
    fi
    return 1
}

# 检查端口
check_port() {
    local port="$1"
    local host="${2:-localhost}"
    
    if ss -tuln 2>/dev/null | grep -q ":${port} "; then
        return 0
    fi
    
    # 回退检查
    if command -v nc &>/dev/null; then
        nc -z -w 2 "$host" "$port" 2>/dev/null
        return $?
    fi
    
    return 1
}

# 检查 HTTP 端点
check_http() {
    local url="$1"
    local timeout="${2:-5}"
    
    if ! command -v curl &>/dev/null; then
        return 2  # 无法检查
    fi
    
    local response=$(curl -sf -o /dev/null -w "%{http_code}" --max-time "$timeout" "$url" 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ] || [ "$response" = "204" ]; then
        return 0
    fi
    
    return 1
}

# 检查单个服务
check_service() {
    local name="$1"
    local type="$2"
    local check="$3"
    
    case "$type" in
        process)
            check_process "$check"
            return $?
            ;;
        port)
            check_port "$check"
            return $?
            ;;
        http)
            check_http "$check" "$CHECK_TIMEOUT"
            return $?
            ;;
        *)
            log WARN "未知检查类型: $type"
            return 2
            ;;
    esac
}

# =============================================================================
# 重启管理
# =============================================================================

# 获取重启次数
get_restart_count() {
    local service_name="$1"
    local history_file="$RESTART_HISTORY_DIR/${service_name}.restarts"
    
    if [ ! -f "$history_file" ]; then
        echo 0
        return
    fi
    
    # 统计最近 1 小时内的重启次数
    local one_hour_ago=$(date -d '1 hour ago' +%s 2>/dev/null || date -v-1H +%s 2>/dev/null)
    [ -z "$one_hour_ago" ] && one_hour_ago=$(($(date +%s) - 3600))
    
    local count=0
    while IFS= read -r line; do
        local timestamp=$(echo "$line" | awk '{print $1}')
        local ts=$(date -d "$timestamp" +%s 2>/dev/null || echo "0")
        [ "$ts" -ge "$one_hour_ago" ] && ((count++))
    done < "$history_file"
    
    echo $count
}

# 记录重启
record_restart() {
    local service_name="$1"
    local reason="$2"
    
    mkdir -p "$RESTART_HISTORY_DIR" 2>/dev/null || return
    local history_file="$RESTART_HISTORY_DIR/${service_name}.restarts"
    
    echo "$(date '+%Y-%m-%d %H:%M:%S') $reason" >> "$history_file"
    
    # 清理旧记录（保留最近 24 小时）
    local one_day_ago=$(date -d '1 day ago' +%s 2>/dev/null || date -v-1d +%s 2>/dev/null)
    [ -z "$one_day_ago" ] && one_day_ago=$(($(date +%s) - 86400))
    
    local temp_file=$(mktemp)
    while IFS= read -r line; do
        local timestamp=$(echo "$line" | awk '{print $1" "$2}')
        local ts=$(date -d "$timestamp" +%s 2>/dev/null || echo "0")
        [ "$ts" -ge "$one_day_ago" ] && echo "$line" >> "$temp_file"
    done < "$history_file"
    
    mv "$temp_file" "$history_file" 2>/dev/null || true
}

# 检查是否在冷却期
is_in_cooldown() {
    local service_name="$1"
    local last_restart_file="$RESTART_HISTORY_DIR/${service_name}.last_restart"
    
    if [ ! -f "$last_restart_file" ]; then
        return 1  # 不在冷却期
    fi
    
    local last_restart=$(cat "$last_restart_file" 2>/dev/null || echo "0")
    local now=$(date +%s)
    
    if [ $((now - last_restart)) -lt "$RESTART_COOLDOWN" ]; then
        return 0  # 在冷却期
    fi
    
    return 1  # 不在冷却期
}

# 记录最后重启时间
record_last_restart_time() {
    local service_name="$1"
    mkdir -p "$RESTART_HISTORY_DIR" 2>/dev/null || return
    date +%s > "$RESTART_HISTORY_DIR/${service_name}.last_restart"
}

# 执行重启
restart_service() {
    local name="$1"
    local restart_cmd="$2"
    local reason="$3"
    
    # 检查冷却期
    if is_in_cooldown "$name"; then
        log WARN "服务 $name 在冷却期内，跳过重启"
        return 1
    fi
    
    # 检查重启次数
    local restart_count=$(get_restart_count "$name")
    if [ "$restart_count" -ge "$MAX_RESTART_COUNT" ]; then
        log ERROR "服务 $name 已达到最大重启次数 ($restart_count/$MAX_RESTART_COUNT)，停止自动重启"
        send_alert "$name" "达到最大重启次数" "critical"
        return 1
    fi
    
    # 干运行模式
    if [ "$DRY_RUN" = "true" ]; then
        log INFO "[干运行] 将重启服务: $name (原因: $reason)"
        return 0
    fi
    
    log INFO "重启服务: $name (原因: $reason)"
    
    # 执行重启
    if eval "$restart_cmd" 2>/dev/null; then
        log SUCCESS "服务 $name 重启命令执行成功"
        
        # 记录
        record_restart "$name" "$reason"
        record_last_restart_time "$name"
        
        # 发送告警
        send_alert "$name" "服务已重启" "warning"
        
        return 0
    else
        log ERROR "服务 $name 重启失败"
        return 1
    fi
}

# =============================================================================
# 告警
# =============================================================================

send_alert() {
    local service_name="$1"
    local message="$2"
    local severity="${3:-warning}"
    
    local hostname=$(hostname)
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local full_message="[$hostname] NAS-OS 服务监控告警\n服务: $service_name\n消息: $message\n时间: $timestamp\n严重级别: $severity"
    
    # Webhook
    if [ -n "$ALERT_WEBHOOK" ]; then
        curl -sf -X POST -H "Content-Type: application/json" \
            -d "{\"text\": \"$full_message\", \"service\": \"$service_name\", \"severity\": \"$severity\"}" \
            "$ALERT_WEBHOOK" 2>/dev/null || true
    fi
    
    # 邮件
    if [ -n "$ALERT_EMAIL" ] && command -v mailx &>/dev/null; then
        echo -e "$full_message" | mailx -s "NAS-OS 服务监控告警: $service_name" "$ALERT_EMAIL" 2>/dev/null || true
    fi
}

# =============================================================================
# 监控循环
# =============================================================================

monitor_cycle() {
    local unhealthy_services=()
    local healthy_count=0
    local unhealthy_count=0
    
    for service in "${SERVICES[@]}"; do
        IFS='|' read -r name type check restart_cmd desc <<< "$service"
        
        if check_service "$name" "$type" "$check"; then
            log SUCCESS "$name ($desc): 健康"
            ((healthy_count++))
        else
            local exit_code=$?
            log WARN "$name ($desc): 不健康 (exit: $exit_code)"
            ((unhealthy_count++))
            
            # 尝试自动重启
            if [ "$AUTO_RESTART" = "true" ] && [ "$exit_code" -ne 2 ]; then
                restart_service "$name" "$restart_cmd" "服务检查失败"
            fi
            
            unhealthy_services+=("$name")
        fi
    done
    
    # 输出汇总
    echo ""
    echo "==================================="
    echo "监控汇总: 健康 $healthy_count, 不健康 $unhealthy_count"
    echo "==================================="
    
    if [ ${#unhealthy_services[@]} -gt 0 ]; then
        echo "不健康服务: ${unhealthy_services[*]}"
    fi
    
    return $unhealthy_count
}

# =============================================================================
# 守护进程模式
# =============================================================================

run_daemon() {
    # 检查是否已在运行
    if [ -f "$PID_FILE" ]; then
        local old_pid=$(cat "$PID_FILE")
        if kill -0 "$old_pid" 2>/dev/null; then
            log ERROR "服务监控已在运行 (PID: $old_pid)"
            exit 1
        else
            rm -f "$PID_FILE"
        fi
    fi
    
    # 写入 PID
    echo $$ > "$PID_FILE"
    
    # 信号处理
    trap 'log INFO "收到退出信号"; rm -f "$PID_FILE"; exit 0' SIGTERM SIGINT
    
    log INFO "启动服务监控守护进程 (PID: $$, 间隔: ${MONITOR_INTERVAL}s)"
    
    while true; do
        echo ""
        echo "$(date '+%Y-%m-%d %H:%M:%S') - 开始监控周期"
        monitor_cycle
        sleep "$MONITOR_INTERVAL"
    done
}

# 停止守护进程
stop_daemon() {
    if [ ! -f "$PID_FILE" ]; then
        log WARN "服务监控未运行"
        exit 0
    fi
    
    local pid=$(cat "$PID_FILE")
    
    if kill -0 "$pid" 2>/dev/null; then
        kill "$pid"
        rm -f "$PID_FILE"
        log SUCCESS "服务监控已停止 (PID: $pid)"
    else
        rm -f "$PID_FILE"
        log WARN "PID 文件存在但进程已不存在"
    fi
}

# 显示状态
show_status() {
    echo "NAS-OS 服务监控 v$VERSION"
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
    echo "服务状态:"
    
    for service in "${SERVICES[@]}"; do
        IFS='|' read -r name type check restart_cmd desc <<< "$service"
        
        local status="未知"
        local color="$NC"
        
        if check_service "$name" "$type" "$check"; then
            status="健康"
            color="$GREEN"
        else
            status="不健康"
            color="$RED"
        fi
        
        echo -e "  ${color}$name${NC}: $status ($desc)"
    done
    
    echo ""
    
    # 显示重启历史
    echo "最近重启记录:"
    for service in "${SERVICES[@]}"; do
        IFS='|' read -r name type check restart_cmd desc <<< "$service"
        local history_file="$RESTART_HISTORY_DIR/${name}.restarts"
        
        if [ -f "$history_file" ]; then
            local count=$(wc -l < "$history_file")
            local last=$(tail -1 "$history_file" 2>/dev/null)
            echo "  $name: $count 次重启 (最近: $last)"
        fi
    done
}

# =============================================================================
# 帮助信息
# =============================================================================

show_help() {
    cat <<EOF
NAS-OS 服务监控脚本 v$VERSION

用途：监控 NAS-OS 核心服务，自动重启异常服务，记录日志

用法: $0 [选项] [命令]

命令:
  (无参数)      单次检查
  --daemon      守护进程模式（持续监控）
  --stop        停止守护进程
  --status      显示服务状态
  --help        显示帮助

选项:
  --interval N  设置监控间隔（秒，默认: 60）
  --dry-run     干运行模式（不执行重启）
  --no-restart  禁用自动重启

环境变量:
  MONITOR_INTERVAL   监控间隔（秒）
  MAX_RESTART_COUNT  最大重启次数（默认: 3）
  RESTART_COOLDOWN   重启冷却时间（秒，默认: 300）
  AUTO_RESTART       启用自动重启（默认: true）
  ALERT_WEBHOOK      告警 Webhook URL
  ALERT_EMAIL        告警邮件地址

示例:
  $0                  # 单次检查
  $0 --daemon         # 守护进程模式
  $0 --status         # 显示状态
  $0 --dry-run        # 干运行模式测试

Cron 配置:
  # 每 5 分钟检查一次
  */5 * * * * /path/to/service-monitor.sh >> /var/log/nas-os/service-monitor.log 2>&1

EOF
}

# =============================================================================
# 主入口
# =============================================================================

main() {
    local mode="once"
    
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
            --interval)
                MONITOR_INTERVAL="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN="true"
                shift
                ;;
            --no-restart)
                AUTO_RESTART="false"
                shift
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
    
    # 确保日志目录存在
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
    
    case "$mode" in
        once)
            monitor_cycle
            exit $?
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
        *)
            show_help
            exit 1
            ;;
    esac
}

main "$@"