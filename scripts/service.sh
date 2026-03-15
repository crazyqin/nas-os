#!/bin/bash
# =============================================================================
# NAS-OS 服务管理脚本 v2.75.0
# =============================================================================
# 用途：统一的 NAS-OS 服务管理入口
# 用法：
#   ./service.sh start      # 启动服务
#   ./service.sh stop       # 停止服务
#   ./service.sh restart    # 重启服务
#   ./service.sh status     # 查看状态
#   ./service.sh logs       # 查看日志
#   ./service.sh reload     # 重载配置
# =============================================================================

set -euo pipefail

VERSION="2.75.0"
SCRIPT_NAME=$(basename "$0")

# 配置
SERVICE_NAME="${SERVICE_NAME:-nas-os}"
BINARY_NAME="${BINARY_NAME:-nasd}"
BINARY_PATH="${BINARY_PATH:-/usr/local/bin/$BINARY_NAME}"
CONFIG_PATH="${CONFIG_PATH:-/etc/nas-os/config.yaml}"
PID_FILE="${PID_FILE:-/var/run/$BINARY_NAME.pid}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"

# 超时
STOP_TIMEOUT="${STOP_TIMEOUT:-30}"
START_TIMEOUT="${START_TIMEOUT:-60}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 日志函数
log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_ok() { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# 获取 PID
get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE" 2>/dev/null
    elif pgrep -x "$BINARY_NAME" > /dev/null 2>&1; then
        pgrep -x "$BINARY_NAME" | head -1
    else
        echo ""
    fi
}

# 检查是否运行
is_running() {
    local pid=$(get_pid)
    [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

# 等待进程停止
wait_stop() {
    local timeout=$STOP_TIMEOUT
    local elapsed=0
    
    while is_running && [ $elapsed -lt $timeout ]; do
        sleep 1
        elapsed=$((elapsed + 1))
    done
    
    if is_running; then
        return 1
    fi
    return 0
}

# 等待进程启动
wait_start() {
    local timeout=$START_TIMEOUT
    local elapsed=0
    local health_url="http://localhost:8080/api/v1/health"
    
    while [ $elapsed -lt $timeout ]; do
        if curl -sf --max-time 2 "$health_url" > /dev/null 2>&1; then
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    return 1
}

# 启动服务
do_start() {
    if is_running; then
        log_warn "服务已在运行中 (PID: $(get_pid))"
        return 0
    fi
    
    log_info "启动 $SERVICE_NAME..."
    
    # 检查二进制
    if [ ! -x "$BINARY_PATH" ]; then
        log_error "找不到可执行文件: $BINARY_PATH"
        return 1
    fi
    
    # 检查配置
    if [ ! -f "$CONFIG_PATH" ]; then
        log_warn "配置文件不存在: $CONFIG_PATH"
    fi
    
    # 创建必要目录
    mkdir -p "$LOG_DIR" "$DATA_DIR"
    
    # 启动
    if [ -f /etc/systemd/system/$SERVICE_NAME.service ]; then
        # 使用 systemd
        systemctl start "$SERVICE_NAME"
    else
        # 直接启动
        cd "$(dirname "$BINARY_PATH")"
        nohup "$BINARY_NAME" > "$LOG_DIR/nasd.stdout.log" 2>&1 &
        echo $! > "$PID_FILE"
    fi
    
    # 等待启动
    if wait_start; then
        log_ok "服务启动成功 (PID: $(get_pid))"
        return 0
    else
        log_error "服务启动超时"
        return 1
    fi
}

# 停止服务
do_stop() {
    if ! is_running; then
        log_warn "服务未运行"
        return 0
    fi
    
    local pid=$(get_pid)
    log_info "停止 $SERVICE_NAME (PID: $pid)..."
    
    if [ -f /etc/systemd/system/$SERVICE_NAME.service ]; then
        systemctl stop "$SERVICE_NAME"
    else
        kill "$pid" 2>/dev/null || true
    fi
    
    if wait_stop; then
        rm -f "$PID_FILE"
        log_ok "服务已停止"
        return 0
    else
        log_error "服务停止超时，尝试强制终止..."
        kill -9 "$pid" 2>/dev/null || true
        rm -f "$PID_FILE"
        return 1
    fi
}

# 重启服务
do_restart() {
    log_info "重启 $SERVICE_NAME..."
    do_stop
    sleep 1
    do_start
}

# 查看状态
do_status() {
    echo -e "${BLUE}═══ $SERVICE_NAME 状态 ═══${NC}"
    
    if is_running; then
        local pid=$(get_pid)
        echo -e "状态:      ${GREEN}运行中${NC}"
        echo -e "PID:       $pid"
        
        # 进程信息
        if [ -n "$pid" ]; then
            local mem=$(ps -o rss= -p "$pid" 2>/dev/null | awk '{printf "%.0f MB", $1/1024}')
            local cpu=$(ps -o %cpu= -p "$pid" 2>/dev/null | awk '{printf "%.1f%%", $1}')
            echo -e "内存:      $mem"
            echo -e "CPU:       $cpu"
        fi
        
        # API 状态
        if curl -sf --max-time 2 http://localhost:8080/api/v1/health > /dev/null 2>&1; then
            echo -e "API:       ${GREEN}正常${NC}"
        else
            echo -e "API:       ${RED}异常${NC}"
        fi
    else
        echo -e "状态:      ${RED}已停止${NC}"
    fi
    
    echo -e "配置:      $CONFIG_PATH"
    echo -e "日志:      $LOG_DIR"
    echo -e "数据:      $DATA_DIR"
}

# 查看日志
do_logs() {
    local lines="${1:-100}"
    
    if [ -f "$LOG_DIR/nasd.log" ]; then
        tail -n "$lines" -f "$LOG_DIR/nasd.log"
    elif [ -f "$LOG_DIR/nasd.stdout.log" ]; then
        tail -n "$lines" -f "$LOG_DIR/nasd.stdout.log"
    else
        journalctl -u "$SERVICE_NAME" -n "$lines" -f
    fi
}

# 重载配置
do_reload() {
    if ! is_running; then
        log_error "服务未运行"
        return 1
    fi
    
    log_info "重载配置..."
    
    # 发送 SIGHUP 信号
    local pid=$(get_pid)
    kill -HUP "$pid" 2>/dev/null || true
    
    log_ok "配置重载信号已发送"
}

# 显示帮助
show_help() {
    cat <<EOF
NAS-OS 服务管理脚本 v${VERSION}

用法: $0 <command>

命令:
  start       启动服务
  stop        停止服务
  restart     重启服务
  status      查看状态
  logs [N]    查看最近 N 行日志（默认 100）
  reload      重载配置（发送 SIGHUP）
  help        显示帮助

环境变量:
  SERVICE_NAME    服务名称（默认: nas-os）
  BINARY_PATH     二进制路径（默认: /usr/local/bin/nasd）
  CONFIG_PATH     配置路径（默认: /etc/nas-os/config.yaml）

示例:
  $0 start        # 启动服务
  $0 restart      # 重启服务
  $0 logs 200     # 查看最近 200 行日志
EOF
}

# 主入口
case "${1:-}" in
    start)      do_start ;;
    stop)       do_stop ;;
    restart)    do_restart ;;
    status)     do_status ;;
    logs)       do_logs "${2:-100}" ;;
    reload)     do_reload ;;
    help|--help|-h) show_help ;;
    *)
        log_error "未知命令: ${1:-}"
        show_help
        exit 1
        ;;
esac