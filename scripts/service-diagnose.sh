#!/bin/bash
#
# NAS-OS 服务诊断脚本
# v2.91.0 工部创建
#
# 功能：
#   - 快速诊断服务问题
#   - 检查服务依赖
#   - 分析日志错误
#   - 提供修复建议
#
# 使用：
#   ./service-diagnose.sh [服务名]
#   ./service-diagnose.sh --all
#

set -euo pipefail

# ========== 配置 ==========
SCRIPT_NAME="service-diagnose"
SCRIPT_VERSION="2.91.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ========== 工具函数 ==========

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_suggestion() {
    echo -e "${CYAN}[建议]${NC} $1"
}

# ========== 诊断函数 ==========

# 检查服务状态
diagnose_service_status() {
    local service="$1"
    
    echo ""
    echo "=== 服务状态: $service ==="
    
    # 检查 systemd 服务
    if systemctl list-unit-files "${service}.service" &>/dev/null; then
        local is_enabled is_active
        is_enabled=$(systemctl is-enabled "${service}" 2>/dev/null || echo "disabled")
        is_active=$(systemctl is-active "${service}" 2>/dev/null || echo "inactive")
        
        echo "  Systemd 服务: 存在"
        echo "  启用状态: $is_enabled"
        echo "  运行状态: $is_active"
        
        if [[ "$is_active" != "active" ]]; then
            log_warning "服务未运行"
            log_suggestion "尝试: systemctl start $service"
        fi
    else
        echo "  Systemd 服务: 不存在"
    fi
    
    # 检查 Docker 容器
    if command -v docker &>/dev/null && docker info &>/dev/null; then
        local container_status
        container_status=$(docker ps -a --filter "name=${service}" --format '{{.Status}}' 2>/dev/null | head -1)
        
        if [[ -n "$container_status" ]]; then
            echo "  Docker 容器: 存在"
            echo "  容器状态: $container_status"
            
            if echo "$container_status" | grep -qi "exited\|dead"; then
                log_warning "容器已停止"
                log_suggestion "尝试: docker start $service"
            fi
        fi
    fi
    
    # 检查进程
    local pids
    pids=$(pgrep -x "$service" 2>/dev/null || true)
    
    if [[ -n "$pids" ]]; then
        echo "  进程 PID: $pids"
        log_success "进程正在运行"
    fi
}

# 检查服务端口
diagnose_service_ports() {
    local service="$1"
    local ports="$2"
    
    if [[ -z "$ports" ]]; then
        return 0
    fi
    
    echo ""
    echo "=== 端口检查: $service ==="
    
    for port in $ports; do
        local listener
        listener=$(ss -tlnp 2>/dev/null | grep ":${port} " | head -1)
        
        if [[ -n "$listener" ]]; then
            echo "  端口 $port: 监听中"
            log_success "$(echo "$listener" | awk '{print $NF}')"
        else
            log_error "端口 $port: 未监听"
            log_suggestion "检查服务配置或启动服务"
        fi
    done
}

# 检查服务依赖
diagnose_service_dependencies() {
    local service="$1"
    
    echo ""
    echo "=== 依赖检查: $service ==="
    
    # 检查网络依赖
    if command -v curl &>/dev/null; then
        local endpoints=(
            "http://localhost:8080/health"
            "http://localhost:9090/-/healthy"
            "http://localhost:3000/api/health"
        )
        
        for endpoint in "${endpoints[@]}"; do
            local response
            response=$(curl -sf -o /dev/null -w "%{http_code}" --connect-timeout 2 "$endpoint" 2>/dev/null || echo "000")
            
            if [[ "$response" == "200" ]]; then
                log_success "$endpoint (HTTP $response)"
            elif [[ "$response" != "000" ]]; then
                log_warning "$endpoint (HTTP $response)"
            fi
        done
    fi
    
    # 检查数据库连接（常见端口）
    local db_ports=(5432 3306 27017 6379)
    local db_names=("PostgreSQL" "MySQL" "MongoDB" "Redis")
    
    for i in "${!db_ports[@]}"; do
        local port="${db_ports[$i]}"
        local name="${db_names[$i]}"
        
        if ss -tln 2>/dev/null | grep -q ":${port} "; then
            log_success "$name (端口 $port) 可用"
        fi
    done
}

# 分析日志错误
diagnose_service_logs() {
    local service="$1"
    local log_file="${2:-/var/log/nas-os/${service}.log}"
    
    echo ""
    echo "=== 日志分析: $service ==="
    
    # 检查 systemd 日志
    if command -v journalctl &>/dev/null; then
        local recent_errors
        recent_errors=$(journalctl -u "${service}" --since "1 hour ago" -p err 2>/dev/null | tail -5)
        
        if [[ -n "$recent_errors" ]]; then
            log_warning "最近 1 小时内有错误:"
            echo "$recent_errors" | head -5 | while read -r line; do
                echo "    $line"
            done
        else
            log_success "systemd 日志无近期错误"
        fi
    fi
    
    # 检查日志文件
    if [[ -f "$log_file" ]]; then
        local error_count warn_count
        error_count=$(grep -c -E "ERROR|CRITICAL|FATAL|panic" "$log_file" 2>/dev/null || echo "0")
        warn_count=$(grep -c -E "WARN|WARNING" "$log_file" 2>/dev/null || echo "0")
        
        echo "  日志文件: $log_file"
        echo "  错误数: $error_count"
        echo "  警告数: $warn_count"
        
        if [[ "$error_count" -gt 0 ]]; then
            log_warning "发现错误日志"
            echo "  最近错误:"
            grep -E "ERROR|CRITICAL|FATAL|panic" "$log_file" 2>/dev/null | tail -3 | while read -r line; do
                echo "    $line"
            done
        fi
    fi
}

# 检查资源配置
diagnose_service_resources() {
    local service="$1"
    
    echo ""
    echo "=== 资源使用: $service ==="
    
    local pids
    pids=$(pgrep -x "$service" 2>/dev/null || true)
    
    if [[ -z "$pids" ]]; then
        echo "  进程未运行，无法检查资源"
        return 0
    fi
    
    for pid in $pids; do
        if [[ -d "/proc/$pid" ]]; then
            local cpu mem rss
            cpu=$(ps -p "$pid" -o %cpu= 2>/dev/null | tr -d ' ' || echo "N/A")
            mem=$(ps -p "$pid" -o %mem= 2>/dev/null | tr -d ' ' || echo "N/A")
            rss=$(ps -p "$pid" -o rss= 2>/dev/null | awk '{printf "%.1f MB", $1/1024}')
            
            echo "  PID $pid:"
            echo "    CPU: ${cpu}%"
            echo "    内存: ${mem}% (${rss})"
            
            # 检查是否资源使用过高
            if [[ "$cpu" != "N/A" ]] && (( $(echo "$cpu > 80" | bc -l 2>/dev/null || echo 0) )); then
                log_warning "CPU 使用率较高"
            fi
        fi
    done
}

# 提供修复建议
provide_fix_suggestions() {
    local service="$1"
    
    echo ""
    echo "=== 修复建议 ==="
    
    # 检查服务是否存在
    if ! systemctl list-unit-files "${service}.service" &>/dev/null && \
       ! docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^${service}$"; then
        log_suggestion "服务 '$service' 可能不存在，检查服务名称是否正确"
        return 0
    fi
    
    # 检查常见问题
    local is_active
    is_active=$(systemctl is-active "${service}" 2>/dev/null || echo "inactive")
    
    if [[ "$is_active" != "active" ]]; then
        log_suggestion "服务未运行，尝试:"
        echo "    systemctl start $service"
        echo "    # 或查看详细错误:"
        echo "    journalctl -u $service -n 50"
    fi
    
    # 检查配置文件
    local config_files=(
        "/etc/nas-os/${service}.yaml"
        "/etc/nas-os/${service}.yml"
        "/etc/nas-os/${service}.conf"
    )
    
    for config in "${config_files[@]}"; do
        if [[ -f "$config" ]]; then
            echo "  配置文件: $config"
        fi
    done
    
    echo ""
    echo "常用命令:"
    echo "  查看状态: systemctl status $service"
    echo "  查看日志: journalctl -u $service -f"
    echo "  重启服务: systemctl restart $service"
}

# 诊断所有服务
diagnose_all_services() {
    echo "========================================"
    echo "NAS-OS 全服务诊断 v${SCRIPT_VERSION}"
    echo "时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "========================================"
    
    local services=("nasd" "docker" "nginx" "prometheus" "grafana")
    
    for service in "${services[@]}"; do
        echo ""
        echo ">>> 诊断服务: $service"
        diagnose_service_status "$service"
    done
    
    echo ""
    echo "========================================"
    echo "诊断完成"
    echo "========================================"
}

# 显示帮助
show_help() {
    cat << EOF
NAS-OS 服务诊断脚本 v${SCRIPT_VERSION}

用法: $0 [服务名] [选项]

选项:
    --all       诊断所有已知服务
    --help, -h  显示帮助信息

示例:
    $0 nasd             # 诊断 nasd 服务
    $0 docker           # 诊断 docker 服务
    $0 --all            # 诊断所有服务

EOF
}

# ========== 主入口 ==========

main() {
    local service="${1:-}"
    
    if [[ -z "$service" ]]; then
        show_help
        exit 0
    fi
    
    case "$service" in
        --help|-h)
            show_help
            exit 0
            ;;
        --all)
            diagnose_all_services
            exit 0
            ;;
        *)
            echo "========================================"
            echo "NAS-OS 服务诊断 v${SCRIPT_VERSION}"
            echo "服务: $service"
            echo "时间: $(date '+%Y-%m-%d %H:%M:%S')"
            echo "========================================"
            
            diagnose_service_status "$service"
            diagnose_service_ports "$service" "8080"
            diagnose_service_dependencies "$service"
            diagnose_service_logs "$service"
            diagnose_service_resources "$service"
            provide_fix_suggestions "$service"
            ;;
    esac
}

main "$@"