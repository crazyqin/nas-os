#!/bin/bash
# =============================================================================
# NAS-OS 监控守护进程脚本 v1.0.0
# =============================================================================
# 用途：后台监控守护进程，持续监控服务状态、系统资源、自动告警
# 用法：
#   ./monitor-daemon.sh start           # 启动守护进程
#   ./monitor-daemon.sh stop            # 停止守护进程
#   ./monitor-daemon.sh restart         # 重启守护进程
#   ./monitor-daemon.sh status          # 查看状态
#   ./monitor-daemon.sh --foreground    # 前台运行（调试用）
# =============================================================================

set -euo pipefail

# 版本
VERSION="1.0.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 默认配置
# =============================================================================

# PID 和状态文件
PID_FILE="${PID_FILE:-/var/run/nas-os-monitor-daemon.pid}"
STATE_FILE="${STATE_FILE:-/var/lib/nas-os/monitor-daemon.state}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
LOG_FILE="${LOG_FILE:-$LOG_DIR/monitor-daemon.log}"
ALERT_LOG="${ALERT_LOG:-$LOG_DIR/alerts.log}"

# 监控间隔（秒）
MONITOR_INTERVAL="${MONITOR_INTERVAL:-60}"

# 资源阈值（百分比）
CPU_THRESHOLD_WARNING="${CPU_THRESHOLD_WARNING:-80}"
CPU_THRESHOLD_CRITICAL="${CPU_THRESHOLD_CRITICAL:-95}"
MEMORY_THRESHOLD_WARNING="${MEMORY_THRESHOLD_WARNING:-80}"
MEMORY_THRESHOLD_CRITICAL="${MEMORY_THRESHOLD_CRITICAL:-95}"
DISK_THRESHOLD_WARNING="${DISK_THRESHOLD_WARNING:-80}"
DISK_THRESHOLD_CRITICAL="${DISK_THRESHOLD_CRITICAL:-95}"

# 告警配置
ALERT_ENABLED="${ALERT_ENABLED:-true}"
ALERT_COOLDOWN="${ALERT_COOLDOWN:-300}"  # 告警冷却时间（秒）
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_DINGTALK="${ALERT_DINGTALK:-}"
ALERT_FEISHU="${ALERT_FEISHU:-}"

# 服务监控配置
SERVICES_CONFIG="${SERVICES_CONFIG:-/etc/nas-os/monitor-services.conf}"

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

# 关联数组声明（必须在赋值前声明）
declare -A STATE_DATA
declare -A LAST_ALERTS
declare -A ALERT_COUNTERS

# =============================================================================
# 日志函数
# =============================================================================

log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    # 确保日志目录存在
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
    
    # 写入日志文件
    echo "[$timestamp] [$level] $message" >> "$LOG_FILE" 2>/dev/null || true
    
    # 控制台输出
    case "$level" in
        ERROR)
            echo -e "${RED}[ERROR]${NC} ${timestamp} - ${message}" >&2
            ;;
        WARN)
            echo -e "${YELLOW}[WARN]${NC} ${timestamp} - ${message}"
            ;;
        INFO)
            echo -e "${GREEN}[INFO]${NC} ${timestamp} - ${message}"
            ;;
        DEBUG)
            if [ "${DEBUG:-false}" = "true" ]; then
                echo -e "${CYAN}[DEBUG]${NC} ${timestamp} - ${message}"
            fi
            ;;
    esac
}

# =============================================================================
# 工具函数
# =============================================================================

# 检查是否以 root 运行
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        log WARN "建议以 root 权限运行以获取完整监控能力"
    fi
}

# 初始化环境和目录
init_environment() {
    # 创建必要的目录
    local dirs=(
        "$(dirname "$PID_FILE")"
        "$(dirname "$STATE_FILE")"
        "$(dirname "$LOG_FILE")"
        "$(dirname "$ALERT_LOG")"
    )

    for dir in "${dirs[@]}"; do
        if [ -n "$dir" ] && [ "$dir" != "." ]; then
            mkdir -p "$dir" 2>/dev/null || {
                log WARN "无法创建目录: $dir"
            }
        fi
    done

    # 初始化状态数据
    STATE_DATA[checks_total]=0
    STATE_DATA[alerts_sent]=0
    STATE_DATA[start_time]=$(date -Iseconds)
}

# 获取进程状态
get_daemon_status() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            echo "running"
            return 0
        else
            echo "stale"
            return 1
        fi
    else
        echo "stopped"
        return 1
    fi
}

# 写入状态文件
save_state() {
    local state_dir=$(dirname "$STATE_FILE")
    mkdir -p "$state_dir" 2>/dev/null || true

    # 安全获取状态值
    local checks_total="${STATE_DATA[checks_total]:-0}"
    local alerts_sent="${STATE_DATA[alerts_sent]:-0}"
    local last_check="${STATE_DATA[last_check]:-never}"

    cat > "$STATE_FILE" 2>/dev/null << EOF
{
    "timestamp": "$(date -Iseconds)",
    "pid": $$,
    "uptime_seconds": $SECONDS,
    "monitor_interval": $MONITOR_INTERVAL,
    "checks_total": $checks_total,
    "alerts_sent": $alerts_sent,
    "last_check": "$last_check"
}
EOF
}

# 清理函数
cleanup() {
    log INFO "守护进程正在停止..."
    rm -f "$PID_FILE" 2>/dev/null
    save_state
    exit 0
}

# 信号处理
setup_signals() {
    trap cleanup SIGTERM SIGINT SIGQUIT
    trap 'log INFO "收到 SIGHUP，重新加载配置"' SIGHUP
}

# =============================================================================
# 监控函数
# =============================================================================

# CPU 使用率检查
check_cpu() {
    local cpu_usage=""

    # 尝试多种方法获取 CPU 使用率
    if command -v mpstat &>/dev/null; then
        cpu_usage=$(mpstat 1 1 2>/dev/null | tail -1 | awk '{print 100 - $NF}' | cut -d. -f1)
    elif [ -f /proc/stat ]; then
        # 读取两次 /proc/stat 计算差值
        local stat1 stat2
        stat1=$(head -1 /proc/stat)
        sleep 0.5
        stat2=$(head -1 /proc/stat)

        # 使用 awk 计算CPU使用率
        cpu_usage=$(echo "$stat1"$'\n'"$stat2" | awk '
            NR==1 { split($0, a); idle1=a[5]; total1=a[2]+a[3]+a[4]+a[5]+a[6]+a[7]+a[8]+a[9]+a[10] }
            NR==2 { split($0, b); idle2=b[5]; total2=b[2]+b[3]+b[4]+b[5]+b[6]+b[7]+b[8]+b[9]+b[10];
                diff_idle=idle2-idle1; diff_total=total2-total1;
                if (diff_total > 0) printf "%.0f", 100 * (1 - diff_idle/diff_total);
                else print "0"
            }')
    else
        log DEBUG "无法获取 CPU 使用率"
        return 0
    fi

    # 验证获取的值
    if [ -z "$cpu_usage" ] || ! [[ "$cpu_usage" =~ ^[0-9]+$ ]]; then
        log DEBUG "CPU 使用率获取失败，跳过检查"
        return 0
    fi

    if [ "$cpu_usage" -ge "$CPU_THRESHOLD_CRITICAL" ]; then
        log WARN "CPU 使用率过高: ${cpu_usage}% (阈值: ${CPU_THRESHOLD_CRITICAL}%)"
        send_alert "critical" "CPU 使用率过高" "当前: ${cpu_usage}%, 阈值: ${CPU_THRESHOLD_CRITICAL}%"
        return 2
    elif [ "$cpu_usage" -ge "$CPU_THRESHOLD_WARNING" ]; then
        log DEBUG "CPU 使用率较高: ${cpu_usage}%"
        return 1
    else
        log DEBUG "CPU 使用率正常: ${cpu_usage}%"
        return 0
    fi
}

# 内存使用率检查
check_memory() {
    local mem_total mem_available mem_usage

    if [ -f /proc/meminfo ]; then
        mem_total=$(grep MemTotal /proc/meminfo | awk '{print $2}')
        mem_available=$(grep MemAvailable /proc/meminfo 2>/dev/null | awk '{print $2}')
        if [ -z "$mem_available" ]; then
            mem_available=$(grep MemFree /proc/meminfo | awk '{print $2}')
        fi

        if [ -n "$mem_total" ] && [ -n "$mem_available" ] && [ "$mem_total" -gt 0 ]; then
            mem_usage=$((100 - (mem_available * 100 / mem_total)))
        else
            log DEBUG "无法从 /proc/meminfo 计算内存使用率"
            return 0
        fi
    elif command -v free &>/dev/null; then
        mem_usage=$(free 2>/dev/null | grep Mem | awk '{printf "%.0f", $3/$2 * 100}')
        if [ -z "$mem_usage" ]; then
            log DEBUG "free 命令获取内存使用率失败"
            return 0
        fi
    else
        log DEBUG "无法获取内存使用率"
        return 0
    fi

    # 验证获取的值
    if ! [[ "$mem_usage" =~ ^[0-9]+$ ]]; then
        log DEBUG "内存使用率值无效: $mem_usage"
        return 0
    fi

    if [ "$mem_usage" -ge "$MEMORY_THRESHOLD_CRITICAL" ]; then
        log WARN "内存使用率过高: ${mem_usage}% (阈值: ${MEMORY_THRESHOLD_CRITICAL}%)"
        send_alert "critical" "内存使用率过高" "当前: ${mem_usage}%, 阈值: ${MEMORY_THRESHOLD_CRITICAL}%"
        return 2
    elif [ "$mem_usage" -ge "$MEMORY_THRESHOLD_WARNING" ]; then
        log DEBUG "内存使用率较高: ${mem_usage}%"
        return 1
    else
        log DEBUG "内存使用率正常: ${mem_usage}%"
        return 0
    fi
}

# 磁盘使用率检查
check_disk() {
    local warnings=0
    local criticals=0
    
    # 检查主要挂载点
    for mount_point in / /home /var /var/lib/nas-os /backup; do
        if mountpoint -q "$mount_point" 2>/dev/null; then
            local usage=$(df "$mount_point" 2>/dev/null | tail -1 | awk '{print $5}' | tr -d '%')
            
            if [ -n "$usage" ] && [ "$usage" -ge "$DISK_THRESHOLD_CRITICAL" ]; then
                log WARN "磁盘空间不足: ${mount_point} (${usage}%, 阈值: ${DISK_THRESHOLD_CRITICAL}%)"
                send_alert "critical" "磁盘空间不足" "挂载点: ${mount_point}, 使用率: ${usage}%"
                criticals=$((criticals + 1))
            elif [ -n "$usage" ] && [ "$usage" -ge "$DISK_THRESHOLD_WARNING" ]; then
                log DEBUG "磁盘空间紧张: ${mount_point} (${usage}%)"
                warnings=$((warnings + 1))
            fi
        fi
    done
    
    if [ "$criticals" -gt 0 ]; then
        return 2
    elif [ "$warnings" -gt 0 ]; then
        return 1
    else
        return 0
    fi
}

# 服务健康检查
check_services() {
    local services_down=0
    
    # 检查系统服务
    local critical_services=(
        "docker"
        "nas-os"
        "nginx"
        "ssh"
    )
    
    for service in "${critical_services[@]}"; do
        if systemctl is-enabled "$service" &>/dev/null; then
            if ! systemctl is-active "$service" &>/dev/null; then
                log WARN "服务已停止: ${service}"
                send_alert "warning" "服务停止" "服务: ${service}"
                services_down=$((services_down + 1))
            else
                log DEBUG "服务正常运行: ${service}"
            fi
        fi
    done
    
    # 检查 Docker 容器（如果有）
    if command -v docker &>/dev/null && docker info &>/dev/null; then
        local containers=$(docker ps -a --filter "label=nas-os.monitor=true" --format '{{.Names}}' 2>/dev/null)
        for container in $containers; do
            local status=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null)
            if [ "$status" != "running" ]; then
                log WARN "容器已停止: ${container}"
                send_alert "warning" "容器停止" "容器: ${container}, 状态: ${status}"
                services_down=$((services_down + 1))
            fi
        done
    fi
    
    return $services_down
}

# 网络连接检查
check_network() {
    local targets=("8.8.8.8" "1.1.1.1" "github.com")
    local failed=0
    
    for target in "${targets[@]}"; do
        if ping -c 1 -W 3 "$target" &>/dev/null; then
            log DEBUG "网络连通: ${target}"
        else
            log DEBUG "网络不通: ${target}"
            failed=$((failed + 1))
        fi
    done
    
    if [ "$failed" -eq "${#targets[@]}" ]; then
        log WARN "网络连接异常"
        send_alert "critical" "网络连接异常" "所有网络目标均不可达"
        return 2
    elif [ "$failed" -gt 0 ]; then
        log DEBUG "部分网络目标不可达"
        return 1
    else
        return 0
    fi
}

# =============================================================================
# 告警函数
# =============================================================================

# 发送告警
send_alert() {
    local level="$1"
    local title="$2"
    local message="$3"
    
    if [ "$ALERT_ENABLED" != "true" ]; then
        return 0
    fi
    
    # 检查告警冷却
    local alert_key="${title// /_}"
    local now=$(date +%s)
    local last_alert="${LAST_ALERTS[$alert_key]:-0}"
    
    if [ $((now - last_alert)) -lt "$ALERT_COOLDOWN" ]; then
        log DEBUG "告警冷却中，跳过: ${title}"
        return 0
    fi
    
    LAST_ALERTS[$alert_key]=$now
    
    # 记录告警日志
    mkdir -p "$(dirname "$ALERT_LOG")" 2>/dev/null || true
    echo "[$(date -Iseconds)] [$level] $title - $message" >> "$ALERT_LOG" 2>/dev/null || true
    
    # 发送 Webhook 告警
    if [ -n "$ALERT_WEBHOOK" ]; then
        local payload=$(cat << EOF
{
    "level": "$level",
    "title": "$title",
    "message": "$message",
    "timestamp": "$(date -Iseconds)",
    "hostname": "$(hostname)"
}
EOF
)
        curl -s -X POST -H "Content-Type: application/json" -d "$payload" "$ALERT_WEBHOOK" &>/dev/null &
        log DEBUG "已发送 Webhook 告警: ${title}"
    fi
    
    # 发送钉钉告警
    if [ -n "$ALERT_DINGTALK" ]; then
        local dingtalk_payload=$(cat << EOF
{
    "msgtype": "text",
    "text": {
        "content": "[${level^^}] ${title}\n${message}\n主机: $(hostname)\n时间: $(date '+%Y-%m-%d %H:%M:%S')"
    }
}
EOF
)
        curl -s -X POST -H "Content-Type: application/json" -d "$dingtalk_payload" "$ALERT_DINGTALK" &>/dev/null &
        log DEBUG "已发送钉钉告警: ${title}"
    fi
    
    # 发送飞书告警
    if [ -n "$ALERT_FEISHU" ]; then
        local feishu_payload=$(cat << EOF
{
    "msg_type": "text",
    "content": {
        "text": "[${level^^}] ${title}\n${message}\n主机: $(hostname)\n时间: $(date '+%Y-%m-%d %H:%M:%S')"
    }
}
EOF
)
        curl -s -X POST -H "Content-Type: application/json" -d "$feishu_payload" "$ALERT_FEISHU" &>/dev/null &
        log DEBUG "已发送飞书告警: ${title}"
    fi
    
    # 更新计数器
    STATE_DATA[alerts_sent]=$((${STATE_DATA[alerts_sent]:-0} + 1))
}

# =============================================================================
# 主循环
# =============================================================================

monitor_loop() {
    log INFO "监控守护进程启动 (PID: $$)"
    log INFO "监控间隔: ${MONITOR_INTERVAL}s"
    log INFO "PID 文件: ${PID_FILE}"

    # 初始化环境
    init_environment
    setup_signals

    # 写入 PID 文件
    echo $$ > "$PID_FILE" || {
        log ERROR "无法写入 PID 文件: $PID_FILE"
        exit 1
    }

    while true; do
        log DEBUG "执行监控检查..."

        # 执行各项检查
        local checks_performed=0

        check_cpu && checks_performed=$((checks_performed + 1))
        check_memory && checks_performed=$((checks_performed + 1))
        check_disk && checks_performed=$((checks_performed + 1))
        check_services && checks_performed=$((checks_performed + 1))
        check_network && checks_performed=$((checks_performed + 1))

        # 更新状态
        STATE_DATA[checks_total]=$((${STATE_DATA[checks_total]:-0} + checks_performed))
        STATE_DATA[last_check]=$(date -Iseconds)
        save_state

        log DEBUG "本轮检查完成: ${checks_performed} 项"

        sleep "$MONITOR_INTERVAL"
    done
}

# =============================================================================
# 管理命令
# =============================================================================

start_daemon() {
    local status=$(get_daemon_status)
    
    if [ "$status" = "running" ]; then
        log INFO "守护进程已在运行 (PID: $(cat "$PID_FILE"))"
        return 0
    fi
    
    # 清理陈旧的 PID 文件
    if [ "$status" = "stale" ]; then
        rm -f "$PID_FILE"
    fi
    
    log INFO "启动监控守护进程..."
    
    # 后台启动
    nohup "$0" --foreground </dev/null >> "$LOG_FILE" 2>&1 &
    local pid=$!
    sleep 1
    
    if kill -0 "$pid" 2>/dev/null; then
        log INFO "守护进程已启动 (PID: ${pid})"
        return 0
    else
        log ERROR "守护进程启动失败"
        return 1
    fi
}

stop_daemon() {
    local status=$(get_daemon_status)
    
    if [ "$status" != "running" ]; then
        log INFO "守护进程未运行"
        rm -f "$PID_FILE" 2>/dev/null
        return 0
    fi
    
    local pid=$(cat "$PID_FILE")
    log INFO "停止守护进程 (PID: ${pid})..."
    
    # 发送 SIGTERM
    kill -TERM "$pid" 2>/dev/null
    
    # 等待进程结束
    local timeout=10
    while [ $timeout -gt 0 ]; do
        if ! kill -0 "$pid" 2>/dev/null; then
            log INFO "守护进程已停止"
            rm -f "$PID_FILE"
            return 0
        fi
        sleep 1
        timeout=$((timeout - 1))
    done
    
    # 强制终止
    log WARN "守护进程未响应，强制终止..."
    kill -KILL "$pid" 2>/dev/null
    rm -f "$PID_FILE"
    return 0
}

restart_daemon() {
    stop_daemon
    sleep 1
    start_daemon
}

show_status() {
    local status=$(get_daemon_status)
    
    echo ""
    echo "========================================"
    echo "  NAS-OS 监控守护进程状态"
    echo "========================================"
    echo ""
    
    case "$status" in
        running)
            local pid=$(cat "$PID_FILE")
            echo -e "状态:    ${GREEN}运行中${NC}"
            echo "PID:     ${pid}"
            echo "运行时间: $(ps -o etime= -p "$pid" 2>/dev/null || echo '未知')"
            
            # 显示状态文件信息
            if [ -f "$STATE_FILE" ]; then
                echo ""
                echo "--- 监控状态 ---"
                cat "$STATE_FILE" 2>/dev/null | grep -E '"[a-z_]+":' | head -5
            fi
            ;;
        stale)
            echo -e "状态:    ${RED}异常${NC} (PID 文件存在但进程已死)"
            echo "PID 文件: ${PID_FILE}"
            ;;
        stopped)
            echo -e "状态:    ${YELLOW}未运行${NC}"
            ;;
    esac
    
    echo ""
    echo "配置:"
    echo "  监控间隔: ${MONITOR_INTERVAL}s"
    echo "  PID 文件: ${PID_FILE}"
    echo "  日志文件: ${LOG_FILE}"
    echo "  告警状态: ${ALERT_ENABLED}"
    echo ""
}

# =============================================================================
# 参数解析
# =============================================================================

show_help() {
    cat << EOF
NAS-OS 监控守护进程脚本 v${VERSION}

用法: $SCRIPT_NAME <命令> [选项]

命令:
    start           启动守护进程
    stop            停止守护进程
    restart         重启守护进程
    status          查看状态
    --foreground    前台运行（调试用）

环境变量:
    MONITOR_INTERVAL        监控间隔（秒，默认: 60）
    CPU_THRESHOLD_WARNING   CPU 警告阈值（默认: 80）
    CPU_THRESHOLD_CRITICAL  CPU 临界阈值（默认: 95）
    MEMORY_THRESHOLD_WARNING 内存警告阈值（默认: 80）
    MEMORY_THRESHOLD_CRITICAL 内存临界阈值（默认: 95）
    DISK_THRESHOLD_WARNING  磁盘警告阈值（默认: 80）
    DISK_THRESHOLD_CRITICAL  磁盘临界阈值（默认: 95）
    ALERT_ENABLED           启用告警（默认: true）
    ALERT_COOLDOWN          告警冷却时间（秒，默认: 300）
    ALERT_WEBHOOK           告警 Webhook URL
    ALERT_DINGTALK          钉钉机器人 URL
    ALERT_FEISHU            飞书机器人 URL

示例:
    $SCRIPT_NAME start                  # 启动守护进程
    $SCRIPT_NAME status                 # 查看状态
    MONITOR_INTERVAL=30 $SCRIPT_NAME start  # 自定义监控间隔

EOF
}

parse_args() {
    case "${1:-}" in
        start)
            start_daemon
            ;;
        stop)
            stop_daemon
            ;;
        restart)
            restart_daemon
            ;;
        status)
            show_status
            ;;
        --foreground)
            check_root
            monitor_loop
            ;;
        -h|--help)
            show_help
            ;;
        "")
            show_help
            exit 1
            ;;
        *)
            log ERROR "未知命令: ${1}"
            show_help
            exit 1
            ;;
    esac
}

# =============================================================================
# 主入口
# =============================================================================

main() {
    parse_args "$@"
}

main "$@"