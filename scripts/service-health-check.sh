#!/bin/bash
# =============================================================================
# NAS-OS 服务健康检查脚本 v2.74.0
# =============================================================================
# 用途：检查所有关键服务的健康状态，支持 HTTP 健康探针、进程检测、端口扫描
# 用法：./service-health-check.sh [--json] [--quick] [--alert] [--services svc1,svc2]
# =============================================================================

set -euo pipefail

VERSION="2.74.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 配置
# =============================================================================

# API 配置
API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-8080}"
API_URL="http://${API_HOST}:${API_PORT}"

# 超时设置
HTTP_TIMEOUT="${HTTP_TIMEOUT:-5}"
CONNECT_TIMEOUT="${CONNECT_TIMEOUT:-3}"

# 健康检查重试次数
HEALTH_RETRIES="${HEALTH_RETRIES:-3}"
HEALTH_RETRY_INTERVAL="${HEALTH_RETRY_INTERVAL:-2}"

# 数据目录
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 输出格式
OUTPUT_FORMAT="${OUTPUT_FORMAT:-text}"
QUICK_MODE="${QUICK_MODE:-false}"
ENABLE_ALERT="${ENABLE_ALERT:-false}"

# 告警配置
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_EMAIL="${ALERT_EMAIL:-}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# =============================================================================
# 服务定义
# =============================================================================

# 格式: 名称:显示名:端口:健康路径:类型(docker/systemd/process)
DEFAULT_SERVICES=(
    "nas-os:NAS-OS 主服务:8080:/api/v1/health:docker"
    "prometheus:Prometheus 监控:9090:/-/healthy:docker"
    "grafana:Grafana 可视化:3000:/api/health:docker"
    "alertmanager:Alertmanager:9093:/-/healthy:docker"
    "node-exporter:Node Exporter:9100:/metrics:docker"
    "cadvisor:cAdvisor:8081:/healthz:docker"
)

# =============================================================================
# 全局状态
# =============================================================================

OVERALL_STATUS="healthy"
HEALTHY_COUNT=0
UNHEALTHY_COUNT=0
WARNING_COUNT=0
SERVICES_STATUS=()
ALERTS=()

# =============================================================================
# 工具函数
# =============================================================================

get_timestamp() {
    date -Iseconds 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z'
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

# 检查命令是否存在
command_exists() {
    command -v "$1" &>/dev/null
}

# =============================================================================
# 服务检查函数
# =============================================================================

# 检查 Docker 容器
check_docker_container() {
    local container="$1"
    
    if ! command_exists docker; then
        return 2  # Docker 不可用
    fi
    
    local status
    status=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null || echo "not_found")
    
    case "$status" in
        running)
            # 检查健康状态
            local health
            health=$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || echo "none")
            
            case "$health" in
                healthy) return 0 ;;
                unhealthy) return 1 ;;
                starting) return 3 ;;  # 启动中
                *) return 0 ;;  # 无健康检查，默认健康
            fi
            ;;
        exited|dead) return 1 ;;
        paused) return 3 ;;
        *) return 2 ;;
    esac
}

# 检查 systemd 服务
check_systemd_service() {
    local service="$1"
    
    if ! command_exists systemctl; then
        return 2
    fi
    
    local active_state
    active_state=$(systemctl show -p ActiveState "$service" 2>/dev/null | cut -d= -f2)
    
    case "$active_state" in
        active) return 0 ;;
        inactive|failed) return 1 ;;
        activating) return 3 ;;
        *) return 2 ;;
    esac
}

# 检查进程
check_process() {
    local process_name="$1"
    
    if pgrep -x "$process_name" > /dev/null 2>&1; then
        return 0
    fi
    return 1
}

# 检查端口
check_port() {
    local port="$1"
    local host="${2:-localhost}"
    
    if command_exists ss; then
        ss -tuln 2>/dev/null | grep -q ":${port} " && return 0
    elif command_exists nc; then
        nc -z -w "$CONNECT_TIMEOUT" "$host" "$port" 2>/dev/null && return 0
    fi
    return 1
}

# 检查 HTTP 健康端点
check_http_health() {
    local url="$1"
    local expected_status="${2:-200}"
    
    if ! command_exists curl; then
        return 2
    fi
    
    local response
    response=$(curl -sf -o /dev/null -w "%{http_code}" --max-time "$HTTP_TIMEOUT" "$url" 2>/dev/null || echo "000")
    
    if [ "$response" = "$expected_status" ]; then
        return 0
    elif [ "$response" = "000" ]; then
        return 2  # 连接失败
    else
        return 1
    fi
}

# 带重试的 HTTP 健康检查
check_http_health_with_retry() {
    local url="$1"
    local expected_status="${2:-200}"
    local retries="${3:-$HEALTH_RETRIES}"
    local interval="${4:-$HEALTH_RETRY_INTERVAL}"
    
    for ((i=1; i<=retries; i++)); do
        if check_http_health "$url" "$expected_status"; then
            return 0
        fi
        
        if [ $i -lt $retries ]; then
            sleep "$interval"
        fi
    done
    
    return 1
}

# =============================================================================
# 主检查函数
# =============================================================================

check_service() {
    local name="$1"
    local display="$2"
    local port="$3"
    local health_path="$4"
    local service_type="$5"
    
    local status="unknown"
    local message=""
    local details=""
    
    # 1. 检查服务状态
    case "$service_type" in
        docker)
            if check_docker_container "$name"; then
                status="healthy"
                message="容器运行中"
            else
                local rc=$?
                case $rc in
                    1) status="unhealthy"; message="容器异常" ;;
                    3) status="starting"; message="容器启动中" ;;
                    *) status="unknown"; message="容器不存在或 Docker 不可用" ;;
                esac
            fi
            details="type=docker"
            ;;
        systemd)
            if check_systemd_service "$name"; then
                status="healthy"
                message="服务运行中"
            else
                status="unhealthy"
                message="服务停止"
            fi
            details="type=systemd"
            ;;
        process)
            if check_process "$name"; then
                status="healthy"
                message="进程运行中"
            else
                status="unhealthy"
                message="进程未运行"
            fi
            details="type=process"
            ;;
    esac
    
    # 2. 检查端口
    local port_status="unknown"
    if [ -n "$port" ] && [ "$port" != "0" ]; then
        if check_port "$port"; then
            port_status="listening"
        else
            port_status="not_listening"
            if [ "$status" = "healthy" ]; then
                status="warning"
                message="$message (端口未监听)"
            fi
        fi
    fi
    details="$details port=$port port_status=$port_status"
    
    # 3. 检查 HTTP 健康端点
    if [ -n "$health_path" ] && [ "$health_path" != "-" ]; then
        local health_url="${API_URL/http:\/\/localhost/http:\/\/$name}:${port}${health_path}"
        
        # 对于本地服务使用 localhost
        if [ "$name" = "nas-os" ] || [ "$name" = "localhost" ]; then
            health_url="http://localhost:${port}${health_path}"
        fi
        
        if check_http_health_with_retry "$health_url"; then
            details="$details http=healthy"
        else
            details="$details http=unhealthy"
            if [ "$status" = "healthy" ]; then
                status="unhealthy"
                message="健康检查失败"
            fi
        fi
    fi
    
    # 记录状态
    SERVICES_STATUS+=("name=$name|display=$display|status=$status|message=$message|details=$details")
    
    # 更新计数
    case "$status" in
        healthy) HEALTHY_COUNT=$((HEALTHY_COUNT + 1)) ;;
        unhealthy) 
            UNHEALTHY_COUNT=$((UNHEALTHY_COUNT + 1))
            OVERALL_STATUS="unhealthy"
            ALERTS+=("$display: $message")
            ;;
        warning|starting)
            WARNING_COUNT=$((WARNING_COUNT + 1))
            [ "$OVERALL_STATUS" = "healthy" ] && OVERALL_STATUS="degraded"
            ;;
    esac
    
    # 输出
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        local color=""
        case "$status" in
            healthy) color="$GREEN" ;;
            unhealthy) color="$RED" ;;
            warning|starting) color="$YELLOW" ;;
            *) color="$CYAN" ;;
        esac
        
        printf "  %-25s ${color}%-12s${NC} %s\n" "$display" "[$status]" "$message"
    fi
}

# 检查主服务 API
check_main_api() {
    echo ""
    log_info "检查 NAS-OS API..."
    
    local endpoints=(
        "health:/api/v1/health"
        "ready:/api/v1/ready"
        "status:/api/v1/system/status"
    )
    
    for ep in "${endpoints[@]}"; do
        local name="${ep%%:*}"
        local path="${ep#*:}"
        local url="$API_URL$path"
        
        if check_http_health "$url"; then
            echo -e "  ${GREEN}✓${NC} $name: $url"
        else
            echo -e "  ${RED}✗${NC} $name: $url"
        fi
    done
}

# 快速检查（仅关键服务）
run_quick_check() {
    log_info "快速健康检查..."
    
    # 仅检查主服务
    check_service "nas-os" "NAS-OS 主服务" "8080" "/api/v1/health" "docker"
    
    if [ "$OVERALL_STATUS" = "healthy" ]; then
        check_main_api
    fi
}

# 完整检查
run_full_check() {
    echo ""
    echo "========================================"
    echo "  NAS-OS 服务健康检查 v$VERSION"
    echo "  时间: $(get_timestamp)"
    echo "========================================"
    echo ""
    
    log_info "检查服务状态..."
    
    for svc in "${DEFAULT_SERVICES[@]}"; do
        IFS=':' read -r name display port health_path service_type <<< "$svc"
        check_service "$name" "$display" "$port" "$health_path" "$service_type"
    done
    
    # 检查主服务 API
    if [ "$OVERALL_STATUS" != "unhealthy" ]; then
        check_main_api
    fi
    
    # 汇总
    echo ""
    echo "========================================"
    echo "  检查结果汇总"
    echo "========================================"
    echo ""
    echo -e "  ${GREEN}健康:${NC}   $HEALTHY_COUNT"
    echo -e "  ${YELLOW}警告:${NC}   $WARNING_COUNT"
    echo -e "  ${RED}异常:${NC}   $UNHEALTHY_COUNT"
    
    local status_color="$GREEN"
    [ "$OVERALL_STATUS" = "degraded" ] && status_color="$YELLOW"
    [ "$OVERALL_STATUS" = "unhealthy" ] && status_color="$RED"
    
    echo ""
    echo -e "  整体状态: ${status_color}${OVERALL_STATUS}${NC}"
    
    if [ ${#ALERTS[@]} -gt 0 ]; then
        echo ""
        echo -e "${RED}告警项:${NC}"
        for alert in "${ALERTS[@]}"; do
            echo "  - $alert"
        done
    fi
    
    echo ""
}

# JSON 输出
output_json() {
    local status_code=0
    [ "$OVERALL_STATUS" = "unhealthy" ] && status_code=1
    [ "$OVERALL_STATUS" = "degraded" ] && status_code=2
    
    echo "{"
    echo "  \"version\": \"$VERSION\","
    echo "  \"timestamp\": \"$(get_timestamp)\","
    echo "  \"hostname\": \"$(hostname)\","
    echo "  \"status\": \"$OVERALL_STATUS\","
    echo "  \"summary\": {"
    echo "    \"healthy\": $HEALTHY_COUNT,"
    echo "    \"warning\": $WARNING_COUNT,"
    echo "    \"unhealthy\": $UNHEALTHY_COUNT"
    echo "  },"
    echo "  \"services\": ["
    
    local first=true
    for svc in "${SERVICES_STATUS[@]}"; do
        local name display status message details
        name=$(echo "$svc" | grep -o 'name=[^|]*' | cut -d= -f2)
        display=$(echo "$svc" | grep -o 'display=[^|]*' | cut -d= -f2)
        status=$(echo "$svc" | grep -o 'status=[^|]*' | cut -d= -f2)
        message=$(echo "$svc" | grep -o 'message=[^|]*' | cut -d= -f2)
        
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        
        printf "    {\"name\": \"%s\", \"display\": \"%s\", \"status\": \"%s\", \"message\": \"%s\"}" \
            "$name" "$display" "$status" "$message"
    done
    
    echo ""
    echo "  ],"
    echo "  \"alerts\": ["
    first=true
    for alert in "${ALERTS[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            echo ","
        fi
        printf "    \"%s\"" "$alert"
    done
    echo ""
    echo "  ]"
    echo "}"
    
    return $status_code
}

# 发送告警
send_alerts() {
    if [ "$ENABLE_ALERT" != "true" ] || [ ${#ALERTS[@]} -eq 0 ]; then
        return 0
    fi
    
    local subject="[NAS-OS] 服务健康告警 - $(date '+%Y-%m-%d %H:%M')"
    local body="服务健康检查发现问题：

状态: $OVERALL_STATUS
时间: $(get_timestamp)

异常服务:
"
    for alert in "${ALERTS[@]}"; do
        body="$body
  - $alert"
    done
    
    # Webhook
    if [ -n "$ALERT_WEBHOOK" ]; then
        curl -s -X POST -H "Content-Type: application/json" \
            -d "{\"text\":\"$subject\n$body\"}" \
            "$ALERT_WEBHOOK" >/dev/null 2>&1 || true
    fi
    
    # Email
    if [ -n "$ALERT_EMAIL" ]; then
        echo "$body" | mail -s "$subject" "$ALERT_EMAIL" 2>/dev/null || true
    fi
}

# =============================================================================
# 帮助
# =============================================================================

show_help() {
    cat <<EOF
NAS-OS 服务健康检查脚本 v$VERSION

用途：检查所有关键服务的健康状态

用法: $0 [选项]

选项:
  --json              JSON 格式输出
  --quick             快速检查（仅主服务）
  --alert             启用告警通知
  --services LIST     指定检查的服务（逗号分隔）
  --help              显示帮助

环境变量:
  API_HOST            API 主机 (默认: localhost)
  API_PORT            API 端口 (默认: 8080)
  HTTP_TIMEOUT        HTTP 超时秒数 (默认: 5)
  HEALTH_RETRIES      健康检查重试次数 (默认: 3)
  ALERT_WEBHOOK       告警 Webhook URL
  ALERT_EMAIL         告警邮箱

退出码:
  0 - 所有服务健康
  1 - 有服务异常
  2 - 有警告但无异常

示例:
  $0                      # 完整检查
  $0 --json               # JSON 输出
  $0 --quick              # 快速检查
  $0 --alert --json       # 启用告警的快速检查

EOF
}

# =============================================================================
# 主入口
# =============================================================================

main() {
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
            --alert)
                ENABLE_ALERT="true"
                shift
                ;;
            --services)
                # 自定义服务列表
                shift
                IFS=',' read -ra CUSTOM_SERVICES <<< "$1"
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
    
    # 执行检查
    if [ "$QUICK_MODE" = "true" ]; then
        run_quick_check
    else
        run_full_check
    fi
    
    # 输出
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        output_json
        local rc=$?
    else
        # 返回退出码
        [ "$OVERALL_STATUS" = "healthy" ] && local rc=0
        [ "$OVERALL_STATUS" = "degraded" ] && local rc=2
        [ "$OVERALL_STATUS" = "unhealthy" ] && local rc=1
    fi
    
    # 发送告警
    send_alerts
    
    exit ${rc:-0}
}

main "$@"