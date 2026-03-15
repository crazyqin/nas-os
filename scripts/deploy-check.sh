#!/bin/bash
# =============================================================================
# NAS-OS 部署验证脚本 v2.70.0
# =============================================================================
# 用途：验证服务部署状态，检查容器/进程、配置、依赖项等
# 用法：./deploy-check.sh [--json] [--service NAME] [--timeout SEC]
# =============================================================================

set -euo pipefail

# 版本
VERSION="2.70.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 默认配置
# =============================================================================

# 服务配置
API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-8080}"
API_URL="http://${API_HOST}:${API_PORT}"

# 超时设置
DEFAULT_TIMEOUT="${DEFAULT_TIMEOUT:-30}"
CHECK_TIMEOUT="${CHECK_TIMEOUT:-10}"

# 输出格式：text, json
OUTPUT_FORMAT="${OUTPUT_FORMAT:-text}"

# 指定服务检查
TARGET_SERVICE="${TARGET_SERVICE:-}"

# 日志目录
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 配置目录
CONFIG_DIR="${CONFIG_DIR:-/etc/nas-os}"

# 数据目录
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"

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
CHECKS=()
ERRORS=()
WARNINGS=()
START_TIME=$(date +%s)

# =============================================================================
# 日志函数
# =============================================================================

log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case "$level" in
        ERROR)
            echo -e "${RED}[ERROR]${NC} ${timestamp} - ${message}" >&2
            ;;
        WARN)
            echo -e "${YELLOW}[WARN]${NC} ${timestamp} - ${message}" >&2
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

# 添加检查结果
add_check() {
    local name="$1"
    local status="$2"
    local message="$3"
    local details="${4:-}"
    
    CHECKS+=("name=$name|status=$status|message=$message|details=$details")
    
    if [ "$status" = "critical" ]; then
        ERRORS+=("$message")
        OVERALL_STATUS="unhealthy"
    elif [ "$status" = "warning" ]; then
        WARNINGS+=("$message")
        [ "$OVERALL_STATUS" = "healthy" ] && OVERALL_STATUS="degraded"
    fi
}

# 检查命令是否存在
check_command() {
    local cmd="$1"
    if command -v "$cmd" &> /dev/null; then
        return 0
    else
        return 1
    fi
}

# 安全获取 JSON 值
json_value() {
    local json="$1"
    local key="$2"
    local default="${3:-}"
    echo "$json" | grep -o "\"$key\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | sed 's/.*: *"\([^"]*\)".*/\1/' 2>/dev/null || echo "$default"
}

# =============================================================================
# 解析参数
# =============================================================================

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json)
                OUTPUT_FORMAT="json"
                shift
                ;;
            --service)
                TARGET_SERVICE="$2"
                shift 2
                ;;
            --timeout)
                DEFAULT_TIMEOUT="$2"
                shift 2
                ;;
            --debug)
                DEBUG="true"
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log ERROR "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    cat << EOF
NAS-OS 部署验证脚本 v${VERSION}

用法: $SCRIPT_NAME [选项]

选项:
    --json              JSON 格式输出
    --service NAME      指定检查的服务
    --timeout SEC       检查超时时间（默认: ${DEFAULT_TIMEOUT}s）
    --debug             显示调试信息
    -h, --help          显示帮助信息

示例:
    $SCRIPT_NAME                    # 检查所有服务
    $SCRIPT_NAME --service nasd     # 检查指定服务
    $SCRIPT_NAME --json             # JSON 输出

EOF
}

# =============================================================================
# 检查函数
# =============================================================================

# 检查系统依赖
check_dependencies() {
    log INFO "检查系统依赖..."
    
    local required_cmds=("curl" "systemctl" "jq")
    local missing=()
    
    for cmd in "${required_cmds[@]}"; do
        if ! check_command "$cmd"; then
            missing+=("$cmd")
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        add_check "dependencies" "critical" "缺少必要命令: ${missing[*]}" \
            "required: ${required_cmds[*]}, missing: ${missing[*]}"
        return 1
    fi
    
    add_check "dependencies" "healthy" "系统依赖完整"
    return 0
}

# 检查服务进程状态
check_service_process() {
    local service_name="${1:-nasd}"
    log INFO "检查服务进程: $service_name"
    
    # 检查 systemd 服务
    if systemctl is-active --quiet "$service_name" 2>/dev/null; then
        local pid=$(systemctl show --property MainPID --value "$service_name" 2>/dev/null)
        local uptime=$(systemctl show --property ActiveEnterTimestamp --value "$service_name" 2>/dev/null)
        
        add_check "service_process" "healthy" "服务 $service_name 运行中 (PID: $pid)" \
            "pid=$pid, uptime=$uptime"
        return 0
    fi
    
    # 检查 Docker 容器
    if check_command docker && docker ps --format '{{.Names}}' 2>/dev/null | grep -q "$service_name"; then
        local container_id=$(docker ps -q -f name="$service_name" 2>/dev/null | head -1)
        local container_status=$(docker inspect --format '{{.State.Status}}' "$container_id" 2>/dev/null)
        
        if [ "$container_status" = "running" ]; then
            add_check "service_process" "healthy" "Docker 容器 $service_name 运行中" \
                "container_id=$container_id, status=$container_status"
            return 0
        fi
    fi
    
    # 检查进程是否存在
    if pgrep -f "$service_name" > /dev/null 2>&1; then
        local pid=$(pgrep -f "$service_name" | head -1)
        add_check "service_process" "warning" "服务 $service_name 以非服务方式运行 (PID: $pid)" \
            "pid=$pid, note=not_managed_by_systemd_or_docker"
        return 0
    fi
    
    add_check "service_process" "critical" "服务 $service_name 未运行" \
        "service=$service_name, status=stopped"
    return 1
}

# 检查 API 健康端点
check_api_health() {
    log INFO "检查 API 健康端点..."
    
    local health_url="${API_URL}/api/v1/health"
    local response
    local http_code
    local start_time
    local elapsed
    
    start_time=$(date +%s%N)
    
    if response=$(curl -sf --connect-timeout 5 --max-time "$CHECK_TIMEOUT" "$health_url" 2>/dev/null); then
        http_code=$(curl -sf -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time "$CHECK_TIMEOUT" "$health_url" 2>/dev/null)
        elapsed=$(( ($(date +%s%N) - start_time) / 1000000 ))
        
        if [ "$http_code" = "200" ]; then
            # 解析健康状态
            local status=$(echo "$response" | jq -r '.status // "unknown"' 2>/dev/null)
            
            if [ "$status" = "healthy" ] || [ "$status" = "ok" ]; then
                add_check "api_health" "healthy" "API 健康检查通过 (${elapsed}ms)" \
                    "url=$health_url, status=$status, response_time=${elapsed}ms"
                return 0
            else
                add_check "api_health" "warning" "API 响应异常状态: $status" \
                    "url=$health_url, status=$status, response=$(echo "$response" | head -c 200)"
                return 1
            fi
        else
            add_check "api_health" "critical" "API 健康检查失败 (HTTP $http_code)" \
                "url=$health_url, http_code=$http_code"
            return 1
        fi
    else
        add_check "api_health" "critical" "API 健康端点不可达" \
            "url=$health_url, error=connection_failed"
        return 1
    fi
}

# 检查配置文件
check_config_files() {
    log INFO "检查配置文件..."
    
    local config_file="${CONFIG_DIR}/config.yaml"
    local errors=0
    
    # 检查配置目录
    if [ ! -d "$CONFIG_DIR" ]; then
        add_check "config_dir" "critical" "配置目录不存在: $CONFIG_DIR"
        return 1
    fi
    
    # 检查主配置文件
    if [ ! -f "$config_file" ]; then
        add_check "config_file" "warning" "主配置文件不存在: $config_file (可能使用默认配置)"
        ((errors++))
    else
        # 检查文件权限
        local perms=$(stat -c "%a" "$config_file" 2>/dev/null || stat -f "%OLp" "$config_file" 2>/dev/null)
        if [ "$perms" -gt "644" ]; then
            add_check "config_file" "warning" "配置文件权限过于宽松: $config_file (权限: $perms)"
            ((errors++))
        else
            add_check "config_file" "healthy" "配置文件存在且权限正确"
        fi
    fi
    
    # 检查 TLS 证书（如果配置了）
    local cert_file=$(grep -E '^\s*tls_cert:' "$config_file" 2>/dev/null | awk '{print $2}' | tr -d '"')
    if [ -n "$cert_file" ] && [ ! -f "$cert_file" ]; then
        add_check "tls_cert" "warning" "TLS 证书文件不存在: $cert_file"
        ((errors++))
    fi
    
    return $errors
}

# 检查数据目录
check_data_directory() {
    log INFO "检查数据目录..."
    
    if [ ! -d "$DATA_DIR" ]; then
        add_check "data_dir" "critical" "数据目录不存在: $DATA_DIR"
        return 1
    fi
    
    # 检查写入权限
    if [ ! -w "$DATA_DIR" ]; then
        add_check "data_dir" "critical" "数据目录不可写: $DATA_DIR"
        return 1
    fi
    
    # 检查磁盘空间
    local usage=$(df -h "$DATA_DIR" 2>/dev/null | tail -1 | awk '{print $5}' | tr -d '%')
    
    if [ "$usage" -gt 90 ]; then
        add_check "data_dir" "critical" "数据目录磁盘空间不足: ${usage}% 使用" \
            "path=$DATA_DIR, usage=${usage}%"
        return 1
    elif [ "$usage" -gt 80 ]; then
        add_check "data_dir" "warning" "数据目录磁盘空间紧张: ${usage}% 使用" \
            "path=$DATA_DIR, usage=${usage}%"
    else
        add_check "data_dir" "healthy" "数据目录正常 (${usage}% 使用)"
    fi
    
    return 0
}

# 检查日志目录
check_log_directory() {
    log INFO "检查日志目录..."
    
    if [ ! -d "$LOG_DIR" ]; then
        # 尝试创建
        if mkdir -p "$LOG_DIR" 2>/dev/null; then
            add_check "log_dir" "warning" "日志目录已创建: $LOG_DIR"
        else
            add_check "log_dir" "critical" "日志目录不存在且无法创建: $LOG_DIR"
            return 1
        fi
    fi
    
    # 检查写入权限
    if [ ! -w "$LOG_DIR" ]; then
        add_check "log_dir" "critical" "日志目录不可写: $LOG_DIR"
        return 1
    fi
    
    # 检查最近的错误日志
    local error_count=$(find "$LOG_DIR" -name "*.log" -exec grep -l "ERROR\|FATAL" {} \; 2>/dev/null | wc -l)
    
    if [ "$error_count" -gt 5 ]; then
        add_check "log_dir" "warning" "发现多个包含错误的日志文件: $error_count 个"
    else
        add_check "log_dir" "healthy" "日志目录正常"
    fi
    
    return 0
}

# 检查网络端口
check_network_ports() {
    log INFO "检查网络端口..."
    
    local ports=("$API_PORT")
    local errors=0
    
    for port in "${ports[@]}"; do
        if ss -tuln 2>/dev/null | grep -q ":${port} "; then
            add_check "port_$port" "healthy" "端口 $port 正在监听"
        else
            add_check "port_$port" "critical" "端口 $port 未监听"
            ((errors++))
        fi
    done
    
    return $errors
}

# 检查数据库连接（如果适用）
check_database() {
    log INFO "检查数据库连接..."
    
    # 检查是否有数据库配置
    local db_type=$(grep -E '^\s*type:\s*(sqlite|postgres|mysql)' "${CONFIG_DIR}/config.yaml" 2>/dev/null | head -1 | awk '{print $2}' || echo "sqlite")
    
    if [ "$db_type" = "sqlite" ] || [ -z "$db_type" ]; then
        # SQLite 检查
        local db_file="${DATA_DIR}/nas-os.db"
        if [ -f "$db_file" ]; then
            local db_size=$(stat -c %s "$db_file" 2>/dev/null || stat -f %z "$db_file" 2>/dev/null)
            add_check "database" "healthy" "SQLite 数据库正常 (大小: $(numfmt --to=iec $db_size 2>/dev/null || echo $db_size))"
            return 0
        else
            add_check "database" "warning" "SQLite 数据库文件不存在 (可能首次启动)"
            return 0
        fi
    fi
    
    # 外部数据库检查（PostgreSQL/MySQL）
    local db_host=$(grep -E '^\s*host:' "${CONFIG_DIR}/config.yaml" 2>/dev/null | head -1 | awk '{print $2}' || echo "localhost")
    local db_port=$(grep -E '^\s*port:' "${CONFIG_DIR}/config.yaml" 2>/dev/null | head -1 | awk '{print $2}' || echo "5432")
    
    if nc -z -w5 "$db_host" "$db_port" 2>/dev/null; then
        add_check "database" "healthy" "数据库连接正常 ($db_host:$db_port)"
    else
        add_check "database" "critical" "数据库连接失败 ($db_host:$db_port)"
        return 1
    fi
    
    return 0
}

# 检查 systemd 服务状态（如果使用 systemd）
check_systemd_services() {
    log INFO "检查 systemd 服务..."
    
    local services=("nasd" "nas-os")
    local found=0
    
    for service in "${services[@]}"; do
        if systemctl list-unit-files "${service}.service" &>/dev/null; then
            found=1
            local is_enabled=$(systemctl is-enabled "$service" 2>/dev/null || echo "disabled")
            local is_active=$(systemctl is-active "$service" 2>/dev/null || echo "inactive")
            
            if [ "$is_active" = "active" ]; then
                if [ "$is_enabled" = "enabled" ]; then
                    add_check "systemd_$service" "healthy" "服务 $service 运行中且已启用"
                else
                    add_check "systemd_$service" "warning" "服务 $service 运行中但未设置开机启动"
                fi
            else
                if [ "$is_enabled" = "enabled" ]; then
                    add_check "systemd_$service" "warning" "服务 $service 已启用但未运行"
                else
                    add_check "systemd_$service" "warning" "服务 $service 未运行"
                fi
            fi
        fi
    done
    
    if [ "$found" -eq 0 ]; then
        add_check "systemd_services" "warning" "未检测到 NAS-OS systemd 服务（可能使用 Docker 或其他方式部署）"
    fi
    
    return 0
}

# 检查 Docker 服务状态（如果使用 Docker）
check_docker_services() {
    log INFO "检查 Docker 服务..."
    
    if ! check_command docker; then
        add_check "docker" "healthy" "Docker 未安装（可能使用 systemd 部署）"
        return 0
    fi
    
    if ! docker info &>/dev/null; then
        add_check "docker" "critical" "Docker 服务不可用"
        return 1
    fi
    
    # 检查 NAS-OS 容器
    local containers=$(docker ps -a -f name=nas --format '{{.Names}}:{{.Status}}' 2>/dev/null)
    
    if [ -z "$containers" ]; then
        add_check "docker_containers" "warning" "未检测到 NAS-OS Docker 容器"
        return 0
    fi
    
    local errors=0
    while IFS=: read -r name status; do
        if [[ "$status" == *"Up"* ]]; then
            add_check "docker_$name" "healthy" "容器 $name 运行中"
        else
            add_check "docker_$name" "critical" "容器 $name 异常: $status"
            ((errors++))
        fi
    done <<< "$containers"
    
    return $errors
}

# 检查版本一致性
check_version() {
    log INFO "检查版本信息..."
    
    # 尝试获取 API 版本
    local version_url="${API_URL}/api/v1/version"
    local api_version=$(curl -sf --connect-timeout 5 --max-time 5 "$version_url" 2>/dev/null | jq -r '.version // .build_info.version // "unknown"' 2>/dev/null)
    
    if [ "$api_version" != "unknown" ] && [ -n "$api_version" ]; then
        add_check "version" "healthy" "服务版本: $api_version" "api_version=$api_version"
    else
        # 无法通过 API 获取，检查二进制
        if [ -f "/usr/local/bin/nasd" ]; then
            local bin_version=$(/usr/local/bin/nasd --version 2>/dev/null | head -1 || echo "unknown")
            add_check "version" "healthy" "二进制版本: $bin_version" "binary_version=$bin_version"
        else
            add_check "version" "warning" "无法获取版本信息"
        fi
    fi
    
    return 0
}

# =============================================================================
# 输出函数
# =============================================================================

output_text() {
    echo ""
    echo "========================================"
    echo "  NAS-OS 部署验证报告 v${VERSION}"
    echo "========================================"
    echo ""
    
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local status=$(echo "$check" | grep -o 'status=[^|]*' | cut -d= -f2)
        local message=$(echo "$check" | grep -o 'message=[^|]*' | cut -d= -f2)
        
        local status_icon
        case "$status" in
            healthy) status_icon="${GREEN}✓${NC}" ;;
            warning) status_icon="${YELLOW}!${NC}" ;;
            critical) status_icon="${RED}✗${NC}" ;;
            *) status_icon="${BLUE}?${NC}" ;;
        esac
        
        printf "  %s %-20s %s\n" "$status_icon" "$name:" "$message"
    done
    
    echo ""
    echo "========================================"
    
    if [ ${#ERRORS[@]} -gt 0 ]; then
        echo -e "${RED}发现 ${#ERRORS[@]} 个错误:${NC}"
        for err in "${ERRORS[@]}"; do
            echo "  - $err"
        done
    fi
    
    if [ ${#WARNINGS[@]} -gt 0 ]; then
        echo -e "${YELLOW}发现 ${#WARNINGS[@]} 个警告:${NC}"
        for warn in "${WARNINGS[@]}"; do
            echo "  - $warn"
        done
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    
    echo ""
    echo -e "总体状态: $( [ "$OVERALL_STATUS" = "healthy" ] && echo "${GREEN}健康${NC}" || ([ "$OVERALL_STATUS" = "degraded" ] && echo "${YELLOW}降级${NC}" || echo "${RED}异常${NC}") )"
    echo "检查项数: ${#CHECKS[@]}"
    echo "耗时: ${duration}s"
    echo ""
}

output_json() {
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    
    local checks_json="["
    local first=true
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local status=$(echo "$check" | grep -o 'status=[^|]*' | cut -d= -f2)
        local message=$(echo "$check" | grep -o 'message=[^|]*' | cut -d= -f2)
        local details=$(echo "$check" | grep -o 'details=[^|]*' | cut -d= -f2-)
        
        [ "$first" = "false" ] && checks_json+=","
        first=false
        
        checks_json+="{\"name\":\"$name\",\"status\":\"$status\",\"message\":\"$message\""
        [ -n "$details" ] && checks_json+=",\"details\":\"$details\""
        checks_json+="}"
    done
    checks_json+="]"
    
    local errors_json="["
    first=true
    for err in "${ERRORS[@]}"; do
        [ "$first" = "false" ] && errors_json+=","
        first=false
        errors_json+="\"$err\""
    done
    errors_json+="]"
    
    local warnings_json="["
    first=true
    for warn in "${WARNINGS[@]}"; do
        [ "$first" = "false" ] && warnings_json+=","
        first=false
        warnings_json+="\"$warn\""
    done
    warnings_json+="]"
    
    cat << EOF
{
    "version": "$VERSION",
    "timestamp": "$(date -Iseconds)",
    "status": "$OVERALL_STATUS",
    "duration_seconds": $duration,
    "checks": $checks_json,
    "errors": $errors_json,
    "warnings": $warnings_json,
    "summary": {
        "total": ${#CHECKS[@]},
        "healthy": $(grep -c 'status=healthy' <<< "${CHECKS[*]}" 2>/dev/null || echo 0),
        "warnings": ${#WARNINGS[@]},
        "critical": ${#ERRORS[@]}
    }
}
EOF
}

# =============================================================================
# 主函数
# =============================================================================

main() {
    parse_args "$@"
    
    log INFO "开始部署验证..."
    log INFO "版本: $VERSION"
    
    # 执行检查
    check_dependencies
    check_config_files
    check_data_directory
    check_log_directory
    
    if [ -n "$TARGET_SERVICE" ]; then
        check_service_process "$TARGET_SERVICE"
    else
        check_systemd_services
        check_docker_services
        check_service_process
    fi
    
    check_network_ports
    check_api_health
    check_database
    check_version
    
    # 输出结果
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        output_json
    else
        output_text
    fi
    
    # 返回状态码
    if [ "$OVERALL_STATUS" = "healthy" ]; then
        exit 0
    elif [ "$OVERALL_STATUS" = "degraded" ]; then
        exit 1
    else
        exit 2
    fi
}

# 执行
main "$@"