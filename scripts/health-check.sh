#!/bin/bash
# NAS-OS 健康检查脚本
# 用于监控和诊断服务状态
#
# v2.38.0 新增

set -e

# 配置
API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-8080}"
API_URL="http://${API_HOST}:${API_PORT}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 检查结果
HEALTHY=true

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_healthy() {
    echo -e "${GREEN}[HEALTHY]${NC} $1"
}

log_unhealthy() {
    echo -e "${RED}[UNHEALTHY]${NC} $1"
    HEALTHY=false
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# 检查进程
check_process() {
    log_info "检查进程..."
    
    if pgrep -x nasd > /dev/null; then
        local pid=$(pgrep -x nasd)
        local mem=$(ps -o rss= -p $pid | awk '{printf "%.1f MB", $1/1024}')
        local cpu=$(ps -o %cpu= -p $pid | awk '{printf "%.1f%%", $1}')
        log_healthy "进程运行中 (PID: $pid, 内存: $mem, CPU: $cpu)"
    else
        log_unhealthy "进程未运行"
    fi
}

# 检查 API 健康端点
check_api_health() {
    log_info "检查 API 健康端点..."
    
    local response=$(curl -sf "${API_URL}/api/v1/health" 2>/dev/null)
    
    if [ $? -eq 0 ]; then
        local status=$(echo "$response" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        if [ "$status" = "healthy" ]; then
            log_healthy "API 健康: $status"
        else
            log_warn "API 状态: $status"
        fi
    else
        log_unhealthy "API 健康端点不可达"
    fi
}

# 检查系统状态 API
check_system_status() {
    log_info "检查系统状态..."
    
    local response=$(curl -sf "${API_URL}/api/v1/system/status" 2>/dev/null)
    
    if [ $? -eq 0 ]; then
        # 解析 JSON（简单解析，生产环境建议使用 jq）
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
    if ss -tuln | grep -q ":${API_PORT} "; then
        log_healthy "端口 $API_PORT (Web UI) 监听中"
    else
        log_unhealthy "端口 $API_PORT (Web UI) 未监听"
    fi
    
    # SMB
    if ss -tuln | grep -q ":445 "; then
        log_healthy "端口 445 (SMB) 监听中"
    else
        log_warn "端口 445 (SMB) 未监听"
    fi
    
    # NFS
    if ss -tuln | grep -q ":2049 "; then
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
        local usage=$(df -h "$data_dir" | awk 'NR==2 {print $5}' | tr -d '%')
        local avail=$(df -h "$data_dir" | awk 'NR==2 {print $4}')
        
        if [ $usage -lt 80 ]; then
            log_healthy "磁盘使用: ${usage}% (可用: $avail)"
        elif [ $usage -lt 90 ]; then
            log_warn "磁盘使用: ${usage}% (可用: $avail)"
        else
            log_unhealthy "磁盘使用: ${usage}% (可用: $avail)"
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
        # 检查数据库完整性
        if sqlite3 "$db_path" "PRAGMA integrity_check;" 2>/dev/null | grep -q "ok"; then
            local size=$(du -h "$db_path" | cut -f1)
            log_healthy "数据库正常 (大小: $size)"
        else
            log_unhealthy "数据库完整性检查失败"
        fi
    else
        log_warn "数据库文件不存在: $db_path"
    fi
}

# 检查日志
check_logs() {
    log_info "检查最近日志..."
    
    # 检查是否有错误日志
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
}

# 检查系统资源
check_resources() {
    log_info "检查系统资源..."
    
    # 内存
    local mem_total=$(free -m | awk '/^Mem:/{print $2}')
    local mem_used=$(free -m | awk '/^Mem:/{print $3}')
    local mem_pct=$((mem_used * 100 / mem_total))
    
    if [ $mem_pct -lt 80 ]; then
        log_healthy "内存使用: ${mem_pct}% (${mem_used}MB / ${mem_total}MB)"
    elif [ $mem_pct -lt 90 ]; then
        log_warn "内存使用: ${mem_pct}% (${mem_used}MB / ${mem_total}MB)"
    else
        log_unhealthy "内存使用: ${mem_pct}% (${mem_used}MB / ${mem_total}MB)"
    fi
    
    # CPU 负载
    local load=$(cat /proc/loadavg | awk '{print $1}')
    local cores=$(nproc)
    local load_pct=$(echo "scale=0; $load * 100 / $cores" | bc)
    
    if (( $(echo "$load < $cores" | bc -l) )); then
        log_healthy "CPU 负载: $load (核心数: $cores)"
    else
        log_warn "CPU 负载: $load (核心数: $cores)"
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
    echo ""
    echo "==================================="
    echo "NAS-OS 健康检查"
    echo "==================================="
    echo ""
    
    check_process
    check_api_health
    check_system_status
    check_ports
    check_disk_space
    check_database
    check_resources
    check_logs
    
    echo ""
    echo "==================================="
    
    if [ "$HEALTHY" = true ]; then
        echo -e "${GREEN}状态: 健康${NC}"
        exit 0
    else
        echo -e "${RED}状态: 不健康${NC}"
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
NAS-OS 健康检查工具

用法: $0 <command> [options]

命令:
  quick      快速检查（进程、API、端口）
  full       完整检查
  monitor    持续监控模式
  help       显示帮助

环境变量:
  API_HOST   API 主机 (默认: localhost)
  API_PORT   API 端口 (默认: 8080)

示例:
  $0 quick
  $0 full
  $0 monitor 60
EOF
}

# 主入口
case "${1:-}" in
    quick)
        quick_check
        ;;
    full|"")
        full_check
        ;;
    monitor)
        monitor_mode "${2:-30}"
        ;;
    -h|--help|help)
        show_help
        ;;
    *)
        show_help
        exit 1
        ;;
esac