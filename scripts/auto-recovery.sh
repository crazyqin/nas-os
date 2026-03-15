#!/bin/bash
# NAS-OS 服务自动恢复脚本
# 监控服务状态并自动执行恢复操作
#
# v2.47.0 工部创建
#
# 功能:
# - 服务健康检测 (进程/端口/API/容器)
# - 自动恢复策略 (重启/重启容器/清理资源/回滚)
# - 恢复失败告警 (Webhook/邮件/钉钉/飞书)
# - 恢复历史记录与统计
# - 多种运行模式 (单次/守护/cron)
#
# 配合使用:
#   - health-check.sh: 提供健康检测
#   - monitor.sh: 提供系统监控
#   - rollback.sh: 提供版本回滚
#
# 用法:
#   ./auto-recovery.sh                    # 单次检查并恢复
#   ./auto-recovery.sh --daemon           # 守护进程模式
#   ./auto-recovery.sh --status           # 查看恢复状态
#   ./auto-recovery.sh --history          # 查看恢复历史
#   ./auto-recovery.sh --test             # 测试模式(不执行恢复)
#   ./auto-recovery.sh --config           # 生成配置文件模板

set -euo pipefail

#===========================================
# 版本与路径
#===========================================

VERSION="2.47.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

#===========================================
# 配置
#===========================================

# 数据目录
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
RECOVERY_DIR="${RECOVERY_DIR:-${DATA_DIR}/recovery}"
HISTORY_FILE="${RECOVERY_DIR}/recovery-history.json"
STATE_FILE="${RECOVERY_DIR}/recovery-state.json"
PID_FILE="/var/run/nas-os-recovery.pid"

# 监控间隔
CHECK_INTERVAL="${CHECK_INTERVAL:-30}"

# 恢复配置
MAX_RECOVERY_ATTEMPTS="${MAX_RECOVERY_ATTEMPTS:-3}"      # 最大恢复尝试次数
RECOVERY_COOLDOWN="${RECOVERY_COOLDOWN:-300}"            # 恢复冷却时间(秒)
RECOVERY_BACKOFF="${RECOVERY_BACKOFF:-true}"            # 指数退避
BACKOFF_MULTIPLIER="${BACKOFF_MULTIPLIER:-2}"            # 退避倍数

# 服务配置
SERVICES_CONFIG="${SERVICES_CONFIG:-/etc/nas-os/recovery-services.conf}"

# 默认监控的服务
DEFAULT_SERVICES=(
    "nasd"
    "nasctl"
)

# 默认监控的容器
DEFAULT_CONTAINERS=(
    "nas-os"
    "nas-os-db"
    "nas-os-redis"
)

# 默认监控的端口
DEFAULT_PORTS=(
    "8080:API"
    "445:SMB"
    "2049:NFS"
)

# 告警配置
ALERT_ENABLED="${ALERT_ENABLED:-true}"
ALERT_COOLDOWN="${ALERT_COOLDOWN:-600}"
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_EMAIL="${ALERT_EMAIL:-}"
ALERT_DINGTALK="${ALERT_DINGTALK:-}"
ALERT_FEISHU="${ALERT_FEISHU:-}"
ALERT_TELEGRAM="${ALERT_TELEGRAM:-}"
ALERT_TELEGRAM_CHAT="${ALERT_TELEGRAM_CHAT:-}"

# API 配置
API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-8080}"
API_URL="http://${API_HOST}:${API_PORT}"
API_TIMEOUT="${API_TIMEOUT:-10}"

# Docker 配置
DOCKER_ENABLED="${DOCKER_ENABLED:-true}"
DOCKER_COMPOSE_FILE="${DOCKER_COMPOSE_FILE:-}"

# 系统恢复阈值
MEMORY_RECOVERY_THRESHOLD="${MEMORY_RECOVERY_THRESHOLD:-95}"
DISK_RECOVERY_THRESHOLD="${DISK_RECOVERY_THRESHOLD:-98}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

#===========================================
# 状态变量
#===========================================

RECOVERY_COUNT=0
ALERT_COUNT=0
LAST_ALERT_TIME=0
DAEMON_MODE=false
TEST_MODE=false

#===========================================
# 工具函数
#===========================================

log() {
    local level="$1"
    shift
    local msg="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    # 写入日志文件
    if [ -d "$LOG_DIR" ]; then
        echo "[$timestamp] [$level] $msg" >> "${LOG_DIR}/recovery.log" 2>/dev/null || true
    fi
    
    # 控制台输出
    case "$level" in
        ERROR)   echo -e "${RED}[$timestamp] [ERROR]${NC} $msg" >&2 ;;
        WARN)    echo -e "${YELLOW}[$timestamp] [WARN]${NC} $msg" ;;
        INFO)    echo -e "${BLUE}[$timestamp] [INFO]${NC} $msg" ;;
        SUCCESS) echo -e "${GREEN}[$timestamp] [SUCCESS]${NC} $msg" ;;
        RECOVERY) echo -e "${CYAN}[$timestamp] [RECOVERY]${NC} $msg" ;;
        *)       echo "[$timestamp] [$level] $msg" ;;
    esac
}

# 检查依赖
check_dependencies() {
    local missing=()
    
    command -v curl &>/dev/null || missing+=("curl")
    command -v jq &>/dev/null || missing+=("jq")
    command -v ss &>/dev/null || missing+=("iproute2")
    
    if [ ${#missing[@]} -gt 0 ]; then
        log ERROR "缺少依赖: ${missing[*]}"
        log INFO "请安装: apt install ${missing[*]}"
        exit 1
    fi
}

# 确保目录存在
ensure_directories() {
    mkdir -p "$RECOVERY_DIR" 2>/dev/null || true
    mkdir -p "$LOG_DIR" 2>/dev/null || true
}

# JSON 工具
json_get() {
    local file="$1"
    local key="$2"
    local default="${3:-}"
    
    if [ -f "$file" ]; then
        jq -r "$key // \"$default\"" "$file" 2>/dev/null || echo "$default"
    else
        echo "$default"
    fi
}

json_set() {
    local file="$1"
    local key="$2"
    local value="$3"
    
    mkdir -p "$(dirname "$file")" 2>/dev/null || true
    
    if [ -f "$file" ]; then
        local tmp=$(mktemp)
        jq "$key = $value" "$file" > "$tmp" 2>/dev/null && mv "$tmp" "$file" || rm -f "$tmp"
    else
        echo "{\"$key\": $value}" > "$file" 2>/dev/null || true
    fi
}

# 获取当前时间戳
get_timestamp() {
    date +%s
}

# 时间格式化
format_duration() {
    local seconds=$1
    local days=$((seconds / 86400))
    local hours=$(( (seconds % 86400) / 3600 ))
    local minutes=$(( (seconds % 3600) / 60 ))
    local secs=$((seconds % 60))
    
    if [ $days -gt 0 ]; then
        echo "${days}d ${hours}h ${minutes}m"
    elif [ $hours -gt 0 ]; then
        echo "${hours}h ${minutes}m"
    elif [ $minutes -gt 0 ]; then
        echo "${minutes}m ${secs}s"
    else
        echo "${secs}s"
    fi
}

#===========================================
# 服务检测
#===========================================

# 检测进程是否运行
check_process() {
    local service="$1"
    local pid
    
    pid=$(pgrep -x "$service" 2>/dev/null | head -1)
    
    if [ -n "$pid" ]; then
        echo "running:$pid"
        return 0
    else
        echo "stopped"
        return 1
    fi
}

# 检测端口是否监听
check_port() {
    local port="$1"
    
    if ss -tuln 2>/dev/null | grep -q ":${port} "; then
        return 0
    else
        return 1
    fi
}

# 检测 API 健康
check_api_health() {
    local url="${1:-${API_URL}/api/v1/health}"
    
    local response
    response=$(curl -sf --max-time "$API_TIMEOUT" "$url" 2>/dev/null)
    local exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        local status
        status=$(echo "$response" | jq -r '.status // "unknown"' 2>/dev/null)
        if [ "$status" = "healthy" ]; then
            echo "healthy"
            return 0
        else
            echo "unhealthy:$status"
            return 1
        fi
    else
        echo "failed:$exit_code"
        return 1
    fi
}

# 检测 Docker 容器状态
check_container() {
    local container="$1"
    
    if ! command -v docker &>/dev/null; then
        echo "docker_not_available"
        return 2
    fi
    
    local status
    status=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null)
    
    if [ -z "$status" ]; then
        echo "not_found"
        return 1
    fi
    
    case "$status" in
        running)
            echo "running"
            return 0
            ;;
        exited|dead)
            echo "stopped:$status"
            return 1
            ;;
        *)
            echo "unknown:$status"
            return 2
            ;;
    esac
}

# 获取服务列表
get_services() {
    local services=()
    
    if [ -f "$SERVICES_CONFIG" ]; then
        while IFS= read -r line; do
            [[ -n "$line" && ! "$line" =~ ^# ]] && services+=("$line")
        done < "$SERVICES_CONFIG"
    else
        services=("${DEFAULT_SERVICES[@]}")
    fi
    
    echo "${services[@]}"
}

# 获取容器列表
get_containers() {
    local containers=()
    
    if [ -f "$SERVICES_CONFIG" ]; then
        # 从配置中解析容器列表
        while IFS= read -r line; do
            if [[ "$line" =~ ^container: ]]; then
                containers+=("${line#container:}")
            fi
        done < "$SERVICES_CONFIG"
    fi
    
    if [ ${#containers[@]} -eq 0 ]; then
        containers=("${DEFAULT_CONTAINERS[@]}")
    fi
    
    echo "${containers[@]}"
}

#===========================================
# 恢复操作
#===========================================

# 记录恢复尝试
record_recovery_attempt() {
    local service="$1"
    local reason="$2"
    local action="$3"
    local result="$4"
    
    local timestamp=$(date -Iseconds)
    
    # 更新历史记录
    local entry="{\"time\":\"$timestamp\",\"service\":\"$service\",\"reason\":\"$reason\",\"action\":\"$action\",\"result\":\"$result\"}"
    
    if [ -f "$HISTORY_FILE" ]; then
        local tmp=$(mktemp)
        jq ". += [$entry]" "$HISTORY_FILE" > "$tmp" 2>/dev/null && mv "$tmp" "$HISTORY_FILE" || rm -f "$tmp"
    else
        echo "[$entry]" > "$HISTORY_FILE"
    fi
    
    # 限制历史记录数量
    if [ -f "$HISTORY_FILE" ]; then
        local count=$(jq 'length' "$HISTORY_FILE" 2>/dev/null || echo "0")
        if [ "$count" -gt 100 ]; then
            local tmp=$(mktemp)
            jq '.[-100:]' "$HISTORY_FILE" > "$tmp" 2>/dev/null && mv "$tmp" "$HISTORY_FILE" || rm -f "$tmp"
        fi
    fi
    
    # 更新状态
    json_set "$STATE_FILE" ".last_recovery" "\"$timestamp\""
    json_set "$STATE_FILE" ".total_recoveries" "$(json_get "$STATE_FILE" ".total_recoveries" "0") + 1"
}

# 检查是否可以恢复
can_recover() {
    local service="$1"
    local now=$(get_timestamp)
    
    # 检查冷却时间
    local last_recovery
    last_recovery=$(json_get "$STATE_FILE" ".services[\"$service\"].last_attempt" "0")
    
    if [ -n "$last_recovery" ] && [ "$last_recovery" != "null" ]; then
        local elapsed=$((now - last_recovery))
        if [ $elapsed -lt $RECOVERY_COOLDOWN ]; then
            log WARN "服务 $service 在冷却期内 (${elapsed}s < ${RECOVERY_COOLDOWN}s)"
            return 1
        fi
    fi
    
    # 检查恢复次数
    local attempts
    attempts=$(json_get "$STATE_FILE" ".services[\"$service\"].attempts" "0")
    
    if [ "$attempts" -ge "$MAX_RECOVERY_ATTEMPTS" ]; then
        log WARN "服务 $service 已达最大恢复尝试次数 ($attempts/$MAX_RECOVERY_ATTEMPTS)"
        return 1
    fi
    
    return 0
}

# 记录恢复尝试开始
record_attempt_start() {
    local service="$1"
    local now=$(get_timestamp)
    
    json_set "$STATE_FILE" ".services[\"$service\"].last_attempt" "$now"
    
    local attempts=$(json_get "$STATE_FILE" ".services[\"$service\"].attempts" "0")
    json_set "$STATE_FILE" ".services[\"$service\"].attempts" "$((attempts + 1))"
}

# 重置恢复计数
reset_recovery_count() {
    local service="$1"
    json_set "$STATE_FILE" ".services[\"$service\"].attempts" "0"
    json_set "$STATE_FILE" ".services[\"$service\"].last_attempt" "null"
}

# 启动服务
start_service() {
    local service="$1"
    local result="failed"
    
    log RECOVERY "尝试启动服务: $service"
    
    if [ "$TEST_MODE" = true ]; then
        log INFO "[测试模式] 跳过实际启动"
        return 0
    fi
    
    # 尝试 systemctl
    if command -v systemctl &>/dev/null && systemctl list-unit-files "${service}.service" &>/dev/null; then
        log INFO "使用 systemctl 启动 $service"
        if systemctl start "${service}.service" 2>/dev/null; then
            sleep 3
            if systemctl is-active --quiet "${service}.service"; then
                result="success"
            fi
        fi
    fi
    
    # 尝试 service 命令
    if [ "$result" = "failed" ] && command -v service &>/dev/null; then
        log INFO "使用 service 启动 $service"
        if service "$service" start 2>/dev/null; then
            sleep 3
            if check_process "$service" &>/dev/null; then
                result="success"
            fi
        fi
    fi
    
    # 尝试直接运行
    if [ "$result" = "failed" ]; then
        log INFO "尝试直接运行 $service"
        case "$service" in
            nasd)
                if [ -x "${SCRIPT_DIR}/../nasd" ]; then
                    (cd "${SCRIPT_DIR}/.." && nohup ./nasd > /dev/null 2>&1 &)
                    sleep 3
                    if check_process "$service" &>/dev/null; then
                        result="success"
                    fi
                fi
                ;;
            *)
                log WARN "未知服务类型: $service"
                ;;
        esac
    fi
    
    echo "$result"
}

# 启动容器
start_container() {
    local container="$1"
    local result="failed"
    
    log RECOVERY "尝试启动容器: $container"
    
    if [ "$TEST_MODE" = true ]; then
        log INFO "[测试模式] 跳过实际启动"
        return 0
    fi
    
    if ! command -v docker &>/dev/null; then
        log ERROR "Docker 不可用"
        echo "failed"
        return 1
    fi
    
    # 检查容器是否存在
    if ! docker inspect "$container" &>/dev/null; then
        log WARN "容器 $container 不存在"
        echo "not_found"
        return 1
    fi
    
    # 启动容器
    if docker start "$container" 2>/dev/null; then
        sleep 3
        local status=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null)
        if [ "$status" = "running" ]; then
            result="success"
        fi
    fi
    
    echo "$result"
}

# 重启容器
restart_container() {
    local container="$1"
    local result="failed"
    
    log RECOVERY "尝试重启容器: $container"
    
    if [ "$TEST_MODE" = true ]; then
        log INFO "[测试模式] 跳过实际重启"
        return 0
    fi
    
    if ! command -v docker &>/dev/null; then
        log ERROR "Docker 不可用"
        echo "failed"
        return 1
    fi
    
    if docker restart "$container" 2>/dev/null; then
        sleep 5
        local status=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null)
        if [ "$status" = "running" ]; then
            result="success"
        fi
    fi
    
    echo "$result"
}

# 使用 docker-compose 重启
docker_compose_restart() {
    local compose_file="${DOCKER_COMPOSE_FILE:-${SCRIPT_DIR}/../docker-compose.yml}"
    
    log RECOVERY "使用 docker-compose 重启服务"
    
    if [ "$TEST_MODE" = true ]; then
        log INFO "[测试模式] 跳过实际重启"
        return 0
    fi
    
    if [ ! -f "$compose_file" ]; then
        log ERROR "docker-compose.yml 不存在: $compose_file"
        return 1
    fi
    
    cd "$(dirname "$compose_file")"
    
    if docker-compose restart 2>/dev/null || docker compose restart 2>/dev/null; then
        log SUCCESS "docker-compose 重启成功"
        return 0
    else
        log ERROR "docker-compose 重启失败"
        return 1
    fi
}

# 清理资源
cleanup_resources() {
    log RECOVERY "清理系统资源..."
    
    if [ "$TEST_MODE" = true ]; then
        log INFO "[测试模式] 跳过实际清理"
        return 0
    fi
    
    # 清理 Docker
    if command -v docker &>/dev/null; then
        log INFO "清理 Docker 无用资源..."
        docker system prune -f --volumes=false 2>/dev/null || true
        docker image prune -f 2>/dev/null || true
    fi
    
    # 清理日志
    if [ -d "/var/log/nas-os" ]; then
        log INFO "清理旧日志..."
        find /var/log/nas-os -name "*.log" -mtime +7 -delete 2>/dev/null || true
    fi
    
    # 清理临时文件
    find /tmp -name "nas-os-*" -mtime +1 -delete 2>/dev/null || true
    
    # 清理包缓存
    if command -v apt &>/dev/null; then
        apt-get clean 2>/dev/null || true
    fi
    
    log SUCCESS "资源清理完成"
}

# 紧急内存回收
emergency_memory_recovery() {
    log RECOVERY "执行紧急内存回收..."
    
    if [ "$TEST_MODE" = true ]; then
        log INFO "[测试模式] 跳过实际操作"
        return 0
    fi
    
    # 同步并清理页面缓存
    sync
    echo 3 > /proc/sys/vm/drop_caches 2>/dev/null || true
    
    # 清理 swap
    swapoff -a 2>/dev/null && swapon -a 2>/dev/null || true
    
    log SUCCESS "紧急内存回收完成"
}

# 紧急磁盘回收
emergency_disk_recovery() {
    log RECOVERY "执行紧急磁盘空间回收..."
    
    if [ "$TEST_MODE" = true ]; then
        log INFO "[测试模式] 跳过实际操作"
        return 0
    fi
    
    # 清理包管理器缓存
    if command -v apt &>/dev/null; then
        apt-get clean 2>/dev/null || true
        apt-get autoclean 2>/dev/null || true
        apt-get autoremove -y 2>/dev/null || true
    fi
    
    # 清理日志
    if command -v journalctl &>/dev/null; then
        journalctl --vacuum-time=3d 2>/dev/null || true
    fi
    
    # 清理旧核心转储
    find /var/lib/systemd/coredump -type f -delete 2>/dev/null || true
    
    # 清理 Docker
    if command -v docker &>/dev/null; then
        docker system prune -af --volumes 2>/dev/null || true
    fi
    
    log SUCCESS "紧急磁盘回收完成"
}

# 回滚到上一版本
rollback_version() {
    local rollback_script="${SCRIPT_DIR}/rollback.sh"
    
    log RECOVERY "尝试回滚到上一版本..."
    
    if [ "$TEST_MODE" = true ]; then
        log INFO "[测试模式] 跳过实际回滚"
        return 0
    fi
    
    if [ ! -x "$rollback_script" ]; then
        log ERROR "回滚脚本不存在或不可执行: $rollback_script"
        return 1
    fi
    
    if "$rollback_script" --auto; then
        log SUCCESS "版本回滚成功"
        return 0
    else
        log ERROR "版本回滚失败"
        return 1
    fi
}

#===========================================
# 告警
#===========================================

# 检查告警冷却
can_alert() {
    local now=$(get_timestamp)
    local elapsed=$((now - LAST_ALERT_TIME))
    
    if [ $elapsed -lt $ALERT_COOLDOWN ]; then
        return 1
    fi
    
    return 0
}

# 发送告警
send_alert() {
    local title="$1"
    local message="$2"
    local level="${3:-warning}"
    
    if [ "$ALERT_ENABLED" != true ]; then
        return 0
    fi
    
    if ! can_alert; then
        log INFO "告警冷却中，跳过发送"
        return 0
    fi
    
    log INFO "发送告警: $title"
    
    local timestamp=$(date -Iseconds)
    local hostname=$(hostname)
    local full_message="[$level] $title\n\n$message\n\n主机: $hostname\n时间: $timestamp"
    
    # Webhook
    if [ -n "$ALERT_WEBHOOK" ]; then
        curl -sf -X POST -H "Content-Type: application/json" \
            -d "{\"title\":\"$title\",\"message\":\"$message\",\"level\":\"$level\",\"hostname\":\"$hostname\",\"timestamp\":\"$timestamp\"}" \
            "$ALERT_WEBHOOK" 2>/dev/null || log WARN "Webhook 告警发送失败"
    fi
    
    # 钉钉
    if [ -n "$ALERT_DINGTALK" ]; then
        curl -sf -X POST -H "Content-Type: application/json" \
            -d "{\"msgtype\":\"markdown\",\"markdown\":{\"title\":\"$title\",\"text\":\"## $title\n\n$message\n\n> 主机: $hostname\n> 时间: $timestamp\"}}" \
            "$ALERT_DINGTALK" 2>/dev/null || log WARN "钉钉告警发送失败"
    fi
    
    # 飞书
    if [ -n "$ALERT_FEISHU" ]; then
        curl -sf -X POST -H "Content-Type: application/json" \
            -d "{\"msg_type\":\"interactive\",\"card\":{\"header\":{\"title\":{\"tag\":\"plain_text\",\"content\":\"$title\"},\"template\":\"$level\"},\"elements\":[{\"tag\":\"markdown\",\"content\":\"$message\"},{\"tag\":\"note\",\"elements\":[{\"tag\":\"plain_text\",\"content\":\"主机: $hostname\"}]}]}}" \
            "$ALERT_FEISHU" 2>/dev/null || log WARN "飞书告警发送失败"
    fi
    
    # Telegram
    if [ -n "$ALERT_TELEGRAM" ] && [ -n "$ALERT_TELEGRAM_CHAT" ]; then
        curl -sf -X POST \
            "https://api.telegram.org/bot${ALERT_TELEGRAM}/sendMessage" \
            -d "chat_id=${ALERT_TELEGRAM_CHAT}" \
            -d "text=<b>[$level]</b> $title\n\n$message\n\n<i>主机: $hostname</i>" \
            -d "parse_mode=HTML" 2>/dev/null || log WARN "Telegram告警发送失败"
    fi
    
    # 邮件
    if [ -n "$ALERT_EMAIL" ] && command -v mailx &>/dev/null; then
        echo -e "$full_message" | mailx -s "[NAS-OS] $title" "$ALERT_EMAIL" 2>/dev/null || log WARN "邮件告警发送失败"
    fi
    
    LAST_ALERT_TIME=$(get_timestamp)
    ALERT_COUNT=$((ALERT_COUNT + 1))
}

#===========================================
# 检查与恢复循环
#===========================================

# 检查并恢复服务
check_and_recover_service() {
    local service="$1"
    
    log INFO "检查服务: $service"
    
    local status
    status=$(check_process "$service")
    
    if [[ "$status" == running:* ]]; then
        log SUCCESS "服务 $service 运行中 (${status#running:})"
        reset_recovery_count "$service"
        return 0
    fi
    
    log WARN "服务 $service 已停止"
    
    if ! can_recover "$service"; then
        return 1
    fi
    
    record_attempt_start "$service"
    
    local result
    result=$(start_service "$service")
    
    record_recovery_attempt "$service" "process_stopped" "start_service" "$result"
    
    if [ "$result" = "success" ]; then
        log SUCCESS "服务 $service 恢复成功"
        RECOVERY_COUNT=$((RECOVERY_COUNT + 1))
        reset_recovery_count "$service"
        send_alert "服务恢复: $service" "服务 $service 已自动恢复" "info"
        return 0
    else
        log ERROR "服务 $service 恢复失败"
        send_alert "恢复失败: $service" "服务 $service 自动恢复失败，需要人工介入" "critical"
        return 1
    fi
}

# 检查并恢复容器
check_and_recover_container() {
    local container="$1"
    
    log INFO "检查容器: $container"
    
    local status
    status=$(check_container "$container")
    
    case "$status" in
        running)
            log SUCCESS "容器 $container 运行中"
            reset_recovery_count "container:$container"
            return 0
            ;;
        stopped:*)
            log WARN "容器 $container 已停止 (${status#stopped:})"
            ;;
        not_found)
            log WARN "容器 $container 不存在"
            return 1
            ;;
        docker_not_available)
            log WARN "Docker 不可用，跳过容器检查"
            return 0
            ;;
        *)
            log WARN "容器 $container 状态未知: $status"
            ;;
    esac
    
    if ! can_recover "container:$container"; then
        return 1
    fi
    
    record_attempt_start "container:$container"
    
    local result
    result=$(start_container "$container")
    
    record_recovery_attempt "$container" "container_stopped" "start_container" "$result"
    
    if [ "$result" = "success" ]; then
        log SUCCESS "容器 $container 恢复成功"
        RECOVERY_COUNT=$((RECOVERY_COUNT + 1))
        reset_recovery_count "container:$container"
        send_alert "容器恢复: $container" "容器 $container 已自动恢复" "info"
        return 0
    else
        log ERROR "容器 $container 恢复失败"
        send_alert "恢复失败: $container" "容器 $container 自动恢复失败" "warning"
        return 1
    fi
}

# 检查并恢复端口
check_and_recover_port() {
    local port_spec="$1"
    local port="${port_spec%%:*}"
    local name="${port_spec#*:}"
    
    log INFO "检查端口: $port ($name)"
    
    if check_port "$port"; then
        log SUCCESS "端口 $port ($name) 监听中"
        return 0
    fi
    
    log WARN "端口 $port ($name) 未监听"
    
    # 端口不可用时，尝试重启相关服务
    case "$name" in
        API)
            check_and_recover_service "nasd"
            ;;
        SMB)
            if command -v systemctl &>/dev/null; then
                systemctl restart smbd 2>/dev/null || true
            fi
            ;;
        NFS)
            if command -v systemctl &>/dev/null; then
                systemctl restart nfs-server 2>/dev/null || true
            fi
            ;;
    esac
    
    # 再次检查
    sleep 3
    if check_port "$port"; then
        log SUCCESS "端口 $port 恢复成功"
        return 0
    else
        send_alert "端口异常: $port" "端口 $port ($name) 恢复失败，请检查服务状态" "warning"
        return 1
    fi
}

# 检查系统资源并恢复
check_and_recover_resources() {
    log INFO "检查系统资源..."
    
    # 内存检查
    local mem_total=$(free -m 2>/dev/null | awk '/^Mem:/{print $2}')
    local mem_used=$(free -m 2>/dev/null | awk '/^Mem:/{print $3}')
    local mem_pct=0
    
    if [ -n "$mem_total" ] && [ "$mem_total" -gt 0 ]; then
        mem_pct=$((mem_used * 100 / mem_total))
    fi
    
    if [ $mem_pct -ge $MEMORY_RECOVERY_THRESHOLD ]; then
        log WARN "内存使用过高: ${mem_pct}% (阈值: ${MEMORY_RECOVERY_THRESHOLD}%)"
        emergency_memory_recovery
        send_alert "内存告警" "内存使用 ${mem_pct}%，已执行紧急回收" "critical"
    fi
    
    # 磁盘检查
    local data_dir="/var/lib/nas-os"
    if [ -d "$data_dir" ]; then
        local disk_usage=$(df "$data_dir" 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
        
        if [ -n "$disk_usage" ] && [ "$disk_usage" -ge $DISK_RECOVERY_THRESHOLD ]; then
            log WARN "磁盘使用过高: ${disk_usage}% (阈值: ${DISK_RECOVERY_THRESHOLD}%)"
            emergency_disk_recovery
            send_alert "磁盘告警" "磁盘使用 ${disk_usage}%，已执行紧急清理" "critical"
        fi
    fi
}

# 检查 API 健康
check_and_recover_api() {
    log INFO "检查 API 健康状态..."
    
    local status
    status=$(check_api_health)
    
    case "$status" in
        healthy)
            log SUCCESS "API 健康"
            return 0
            ;;
        unhealthy:*)
            log WARN "API 不健康: ${status#unhealthy:}"
            ;;
        failed:*)
            log WARN "API 不可达: ${status#failed:}"
            ;;
        *)
            log WARN "API 状态未知: $status"
            ;;
    esac
    
    # 尝试恢复服务
    check_and_recover_service "nasd"
    
    # 再次检查
    sleep 5
    status=$(check_api_health)
    
    if [ "$status" = "healthy" ]; then
        log SUCCESS "API 恢复成功"
        return 0
    else
        send_alert "API 异常" "API 健康检查失败，服务可能异常" "critical"
        return 1
    fi
}

# 单次检查
single_check() {
    log INFO "========== 开始恢复检查 =========="
    local start_time=$(get_timestamp)
    
    ensure_directories
    
    # 检查服务
    for service in $(get_services); do
        check_and_recover_service "$service" || true
    done
    
    # 检查容器
    if [ "$DOCKER_ENABLED" = true ]; then
        for container in $(get_containers); do
            check_and_recover_container "$container" || true
        done
    fi
    
    # 检查端口
    for port_spec in "${DEFAULT_PORTS[@]}"; do
        check_and_recover_port "$port_spec" || true
    done
    
    # 检查 API
    check_and_recover_api || true
    
    # 检查资源
    check_and_recover_resources
    
    local end_time=$(get_timestamp)
    local duration=$((end_time - start_time))
    
    log INFO "========== 检查完成 (耗时: ${duration}s, 恢复: ${RECOVERY_COUNT}) =========="
    
    # 更新统计
    local total=$(json_get "$STATE_FILE" ".total_checks" "0")
    json_set "$STATE_FILE" ".total_checks" "$((total + 1))"
    json_set "$STATE_FILE" ".last_check" "\"$(date -Iseconds)\""
}

# 守护进程模式
daemon_mode() {
    log INFO "启动守护进程模式 (间隔: ${CHECK_INTERVAL}s)"
    log INFO "PID 文件: $PID_FILE"
    
    # 检查是否已在运行
    if [ -f "$PID_FILE" ]; then
        local old_pid=$(cat "$PID_FILE" 2>/dev/null)
        if [ -n "$old_pid" ] && kill -0 "$old_pid" 2>/dev/null; then
            log ERROR "守护进程已在运行 (PID: $old_pid)"
            exit 1
        fi
    fi
    
    # 写入 PID
    echo $$ > "$PID_FILE"
    
    # 信号处理
    trap 'log INFO "收到退出信号"; rm -f "$PID_FILE"; exit 0' INT TERM
    trap 'rm -f "$PID_FILE"' EXIT
    
    # 主循环
    while true; do
        single_check
        sleep "$CHECK_INTERVAL"
    done
}

# 查看状态
show_status() {
    ensure_directories
    
    echo "==================================="
    echo "NAS-OS 自动恢复状态 v${VERSION}"
    echo "==================================="
    echo ""
    
    # 基本状态
    echo "【配置】"
    echo "  检查间隔: ${CHECK_INTERVAL}s"
    echo "  最大恢复尝试: ${MAX_RECOVERY_ATTEMPTS}"
    echo "  恢复冷却时间: ${RECOVERY_COOLDOWN}s"
    echo "  告警启用: ${ALERT_ENABLED}"
    echo ""
    
    # 统计
    echo "【统计】"
    echo "  总检查次数: $(json_get "$STATE_FILE" ".total_checks" "0")"
    echo "  总恢复次数: $(json_get "$STATE_FILE" ".total_recoveries" "0")"
    echo "  最后检查: $(json_get "$STATE_FILE" ".last_recovery" "无")"
    echo ""
    
    # 服务状态
    echo "【服务状态】"
    for service in $(get_services); do
        local status=$(check_process "$service")
        if [[ "$status" == running:* ]]; then
            echo -e "  $service: ${GREEN}运行中${NC} (${status#running:})"
        else
            echo -e "  $service: ${RED}已停止${NC}"
        fi
    done
    echo ""
    
    # 容器状态
    if [ "$DOCKER_ENABLED" = true ]; then
        echo "【容器状态】"
        for container in $(get_containers); do
            local status=$(check_container "$container")
            case "$status" in
                running)
                    echo -e "  $container: ${GREEN}运行中${NC}"
                    ;;
                stopped:*)
                    echo -e "  $container: ${RED}已停止${NC} (${status#stopped:})"
                    ;;
                not_found)
                    echo -e "  $container: ${YELLOW}不存在${NC}"
                    ;;
                *)
                    echo -e "  $container: ${YELLOW}未知${NC} ($status)"
                    ;;
            esac
        done
        echo ""
    fi
    
    # 端口状态
    echo "【端口状态】"
    for port_spec in "${DEFAULT_PORTS[@]}"; do
        local port="${port_spec%%:*}"
        local name="${port_spec#*:}"
        if check_port "$port"; then
            echo -e "  $port ($name): ${GREEN}监听中${NC}"
        else
            echo -e "  $port ($name): ${RED}未监听${NC}"
        fi
    done
    echo ""
    
    # 资源状态
    echo "【资源状态】"
    local mem_total=$(free -m 2>/dev/null | awk '/^Mem:/{print $2}')
    local mem_used=$(free -m 2>/dev/null | awk '/^Mem:/{print $3}')
    if [ -n "$mem_total" ] && [ "$mem_total" -gt 0 ]; then
        local mem_pct=$((mem_used * 100 / mem_total))
        echo "  内存: ${mem_pct}% (${mem_used}MB / ${mem_total}MB)"
    fi
    
    local data_dir="/var/lib/nas-os"
    if [ -d "$data_dir" ]; then
        local disk_usage=$(df "$data_dir" 2>/dev/null | awk 'NR==2 {print $5}')
        local disk_avail=$(df -h "$data_dir" 2>/dev/null | awk 'NR==2 {print $4}')
        echo "  磁盘: ${disk_usage} (可用: ${disk_avail})"
    fi
    echo ""
    
    # 守护进程状态
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            echo -e "【守护进程】 ${GREEN}运行中${NC} (PID: $pid)"
        else
            echo -e "【守护进程】 ${YELLOW}已停止${NC} (PID 文件存在但进程不在)"
        fi
    else
        echo "【守护进程】 未运行"
    fi
}

# 查看历史
show_history() {
    ensure_directories
    
    echo "==================================="
    echo "NAS-OS 恢复历史"
    echo "==================================="
    
    if [ ! -f "$HISTORY_FILE" ]; then
        echo "暂无恢复历史记录"
        return 0
    fi
    
    echo ""
    jq -r '.[] | "[\(.time)] \(.service): \(.action) - \(.result)"' "$HISTORY_FILE" 2>/dev/null | tail -20
}

# 生成配置模板
generate_config() {
    local config_file="${SERVICES_CONFIG}"
    local config_dir=$(dirname "$config_file")
    
    mkdir -p "$config_dir" 2>/dev/null
    
    cat > "$config_file" << 'EOF'
# NAS-OS 自动恢复服务配置
# 每行一个服务名，支持进程名和容器

# 监控的进程服务
nasd
nasctl

# 监控的容器 (container: 前缀)
# container:nas-os
# container:nas-os-db
# container:nas-os-redis

# 忽略的服务 (ignore: 前缀)
# ignore:temp-service
EOF
    
    log SUCCESS "配置模板已生成: $config_file"
}

# 帮助信息
show_help() {
    cat <<EOF
NAS-OS 服务自动恢复脚本 v${VERSION}

用法: $0 <command> [options]

命令:
  (默认)      单次检查并恢复
  --daemon    守护进程模式 (持续监控)
  --status    查看恢复状态
  --history   查看恢复历史
  --test      测试模式 (不执行实际恢复)
  --config    生成配置文件模板
  -h, --help  显示帮助

选项:
  --interval <秒>     检查间隔 (默认: 30)
  --max-attempts <N>  最大恢复尝试次数 (默认: 3)
  --cooldown <秒>     恢复冷却时间 (默认: 300)

环境变量:
  CHECK_INTERVAL           检查间隔
  MAX_RECOVERY_ATTEMPTS    最大恢复尝试次数
  RECOVERY_COOLDOWN        恢复冷却时间
  ALERT_WEBHOOK            告警 Webhook URL
  ALERT_EMAIL              告警邮件地址
  ALERT_DINGTALK            钉钉 Webhook
  ALERT_FEISHU              飞书 Webhook
  DOCKER_ENABLED           启用 Docker 检查

示例:
  $0                          # 单次检查
  $0 --daemon                 # 守护进程
  $0 --test                   # 测试模式
  $0 --status                 # 查看状态
  CHECK_INTERVAL=60 $0 --daemon  # 自定义间隔

配置文件:
  $SERVICES_CONFIG

日志文件:
  ${LOG_DIR}/recovery.log
  ${HISTORY_FILE}
EOF
}

#===========================================
# 主入口
#===========================================

main() {
    local cmd=""
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --daemon)
                DAEMON_MODE=true
                shift
                ;;
            --status)
                cmd="status"
                shift
                ;;
            --history)
                cmd="history"
                shift
                ;;
            --test)
                TEST_MODE=true
                shift
                ;;
            --config)
                cmd="config"
                shift
                ;;
            --interval)
                CHECK_INTERVAL="$2"
                shift 2
                ;;
            --max-attempts)
                MAX_RECOVERY_ATTEMPTS="$2"
                shift 2
                ;;
            --cooldown)
                RECOVERY_COOLDOWN="$2"
                shift 2
                ;;
            -h|--help|help)
                cmd="help"
                shift
                ;;
            *)
                shift
                ;;
        esac
    done
    
    check_dependencies
    ensure_directories
    
    case "$cmd" in
        status)
            show_status
            ;;
        history)
            show_history
            ;;
        config)
            generate_config
            ;;
        help)
            show_help
            ;;
        *)
            if [ "$DAEMON_MODE" = true ]; then
                daemon_mode
            else
                single_check
            fi
            ;;
    esac
}

main "$@"