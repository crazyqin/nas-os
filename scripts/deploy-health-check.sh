#!/bin/bash
#
# NAS-OS 部署健康检查脚本
# v2.89.0 工部创建
#
# 功能：
#   - 服务健康检查
#   - 端口可用性检查
#   - 依赖服务检查
#   - 存储挂载检查
#   - 网络连通性检查
#   - 安全配置检查
#   - 生成部署报告
#
# 使用：
#   ./deploy-health-check.sh [--verbose] [--json] [--output FILE]
#

set -euo pipefail

# ========== 配置 ==========
SCRIPT_NAME="deploy-health-check"
SCRIPT_VERSION="2.89.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 默认配置
VERBOSE=false
JSON_OUTPUT=false
OUTPUT_FILE=""

# 服务端口配置
declare -A SERVICE_PORTS=(
    ["nasd"]="8080"
    ["prometheus"]="9090"
    ["grafana"]="3000"
    ["alertmanager"]="9093"
    ["node-exporter"]="9100"
    ["cadvisor"]="8081"
)

# 检查结果
declare -A CHECK_RESULTS=()
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNING_CHECKS=0

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ========== 工具函数 ==========

log_info() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

check_result() {
    local name="$1"
    local status="$2"
    local message="$3"
    
    ((TOTAL_CHECKS++))
    CHECK_RESULTS["$name"]="$status:$message"
    
    case "$status" in
        "pass")
            ((PASSED_CHECKS++))
            log_success "$name: $message"
            ;;
        "warn")
            ((WARNING_CHECKS++))
            log_warning "$name: $message"
            ;;
        "fail")
            ((FAILED_CHECKS++))
            log_error "$name: $message"
            ;;
    esac
}

# ========== 检查函数 ==========

# 检查服务是否运行
check_service_running() {
    local service="$1"
    
    log_info "检查服务状态: $service"
    
    if systemctl is-active --quiet "$service" 2>/dev/null; then
        check_result "service_$service" "pass" "服务运行中"
        return 0
    elif pgrep -x "$service" > /dev/null 2>&1; then
        check_result "service_$service" "pass" "进程运行中"
        return 0
    elif docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${service}$"; then
        check_result "service_$service" "pass" "容器运行中"
        return 0
    else
        check_result "service_$service" "fail" "服务未运行"
        return 1
    fi
}

# 检查端口是否监听
check_port_listening() {
    local service="$1"
    local port="$2"
    
    log_info "检查端口监听: $service ($port)"
    
    if ss -tln 2>/dev/null | grep -q ":${port} " || \
       netstat -tln 2>/dev/null | grep -q ":${port} " || \
       lsof -i ":${port}" 2>/dev/null | grep -q LISTEN; then
        check_result "port_$service" "pass" "端口 $port 正在监听"
        return 0
    else
        check_result "port_$service" "fail" "端口 $port 未监听"
        return 1
    fi
}

# 检查 HTTP 健康端点
check_http_health() {
    local service="$1"
    local url="$2"
    local expected_status="${3:-200}"
    
    log_info "检查 HTTP 健康: $service ($url)"
    
    local response
    response=$(curl -sf -o /dev/null -w "%{http_code}" --connect-timeout 5 "$url" 2>/dev/null) || response="000"
    
    if [[ "$response" == "$expected_status" ]]; then
        check_result "http_$service" "pass" "HTTP 状态码 $response"
        return 0
    else
        check_result "http_$service" "fail" "HTTP 状态码 $response (期望 $expected_status)"
        return 1
    fi
}

# 检查存储挂载
check_storage_mount() {
    local mount_point="$1"
    
    log_info "检查存储挂载: $mount_point"
    
    if mountpoint -q "$mount_point" 2>/dev/null; then
        local usage
        usage=$(df -h "$mount_point" 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
        
        if [[ "$usage" -gt 90 ]]; then
            check_result "storage_$mount_point" "warn" "挂载正常，使用率 ${usage}%"
        elif [[ "$usage" -gt 95 ]]; then
            check_result "storage_$mount_point" "fail" "挂载正常，但使用率过高 ${usage}%"
        else
            check_result "storage_$mount_point" "pass" "挂载正常，使用率 ${usage}%"
        fi
        return 0
    else
        check_result "storage_$mount_point" "fail" "未挂载"
        return 1
    fi
}

# 检查磁盘空间
check_disk_space() {
    local threshold="${1:-90}"
    
    log_info "检查磁盘空间 (阈值: ${threshold}%)"
    
    local result=0
    while read -r line; do
        local usage mounted
        usage=$(echo "$line" | awk '{print $5}' | tr -d '%')
        mounted=$(echo "$line" | awk '{print $6}')
        
        if [[ "$usage" -gt "$threshold" ]]; then
            check_result "disk_space_$mounted" "warn" "使用率 ${usage}% 超过阈值"
            ((result++))
        fi
    done < <(df -h 2>/dev/null | grep -E '^/dev' | grep -v tmpfs)
    
    if [[ $result -eq 0 ]]; then
        check_result "disk_space_all" "pass" "所有磁盘空间充足"
    fi
    
    return $result
}

# 检查内存使用
check_memory() {
    local threshold="${1:-90}"
    
    log_info "检查内存使用 (阈值: ${threshold}%)"
    
    local mem_info
    mem_info=$(free 2>/dev/null | grep Mem)
    
    if [[ -n "$mem_info" ]]; then
        local total used usage
        total=$(echo "$mem_info" | awk '{print $2}')
        used=$(echo "$mem_info" | awk '{print $3}')
        usage=$((used * 100 / total))
        
        if [[ "$usage" -gt "$threshold" ]]; then
            check_result "memory_usage" "warn" "内存使用率 ${usage}% 超过阈值"
            return 1
        else
            check_result "memory_usage" "pass" "内存使用率 ${usage}%"
            return 0
        fi
    else
        check_result "memory_usage" "fail" "无法获取内存信息"
        return 1
    fi
}

# 检查 CPU 负载
check_cpu_load() {
    local threshold="${1:-5}"
    
    log_info "检查 CPU 负载 (阈值: ${threshold})"
    
    local load
    load=$(awk '{print $1}' /proc/loadavg 2>/dev/null)
    
    if [[ -n "$load" ]]; then
        local load_int
        load_int=$(echo "$load" | awk '{printf "%.0f", $1}')
        
        if [[ "$load_int" -gt "$threshold" ]]; then
            check_result "cpu_load" "warn" "CPU 负载 ${load} 较高"
            return 1
        else
            check_result "cpu_load" "pass" "CPU 负载 ${load}"
            return 0
        fi
    else
        check_result "cpu_load" "fail" "无法获取 CPU 负载"
        return 1
    fi
}

# 检查网络连通性
check_network() {
    local target="$1"
    
    log_info "检查网络连通性: $target"
    
    if ping -c 1 -W 3 "$target" > /dev/null 2>&1; then
        check_result "network_$target" "pass" "可达"
        return 0
    else
        check_result "network_$target" "fail" "不可达"
        return 1
    fi
}

# 检查 DNS 解析
check_dns() {
    local domain="$1"
    
    log_info "检查 DNS 解析: $domain"
    
    if nslookup "$domain" > /dev/null 2>&1 || dig "$domain" +short > /dev/null 2>&1; then
        check_result "dns_$domain" "pass" "解析正常"
        return 0
    else
        check_result "dns_$domain" "fail" "解析失败"
        return 1
    fi
}

# 检查 TLS 证书
check_tls_cert() {
    local domain="$1"
    local port="${2:-443}"
    local days_threshold="${3:-30}"
    
    log_info "检查 TLS 证书: $domain:$port"
    
    local expiry
    expiry=$(echo | openssl s_client -servername "$domain" -connect "$domain:$port" 2>/dev/null | \
             openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)
    
    if [[ -n "$expiry" ]]; then
        local expiry_epoch now_epoch days_left
        expiry_epoch=$(date -d "$expiry" +%s 2>/dev/null || date -j -f "%b %d %T %Y %Z" "$expiry" +%s 2>/dev/null)
        now_epoch=$(date +%s)
        days_left=$(( (expiry_epoch - now_epoch) / 86400 ))
        
        if [[ "$days_left" -lt 0 ]]; then
            check_result "tls_$domain" "fail" "证书已过期"
            return 1
        elif [[ "$days_left" -lt "$days_threshold" ]]; then
            check_result "tls_$domain" "warn" "证书将在 ${days_left} 天后过期"
            return 1
        else
            check_result "tls_$domain" "pass" "证书有效期剩余 ${days_left} 天"
            return 0
        fi
    else
        check_result "tls_$domain" "warn" "无法获取证书信息"
        return 1
    fi
}

# 检查 Docker 状态
check_docker() {
    log_info "检查 Docker 状态"
    
    if command -v docker &> /dev/null; then
        if docker info &> /dev/null; then
            local containers
            containers=$(docker ps -q 2>/dev/null | wc -l)
            check_result "docker_status" "pass" "Docker 运行中，${containers} 个容器"
            return 0
        else
            check_result "docker_status" "fail" "Docker 未运行"
            return 1
        fi
    else
        check_result "docker_status" "warn" "Docker 未安装"
        return 1
    fi
}

# 检查 Prometheus 指标
check_prometheus_metrics() {
    local url="${1:-http://localhost:9090}"
    
    log_info "检查 Prometheus 指标: $url"
    
    local response
    response=$(curl -sf "${url}/api/v1/query?query=up" 2>/dev/null) || response=""
    
    if [[ -n "$response" ]] && echo "$response" | grep -q '"status":"success"'; then
        local up_count
        up_count=$(echo "$response" | grep -o '"value":\["[^"]*","1"\]' | wc -l)
        check_result "prometheus_metrics" "pass" "Prometheus 可用，${up_count} 个服务正常"
        return 0
    else
        check_result "prometheus_metrics" "fail" "Prometheus 不可用"
        return 1
    fi
}

# 检查备份状态
check_backup() {
    local backup_dir="${1:-/var/lib/nas-os/backups}"
    local max_age_days="${2:-7}"
    
    log_info "检查备份状态: $backup_dir"
    
    if [[ -d "$backup_dir" ]]; then
        local latest_file latest_time now_time age
        latest_file=$(find "$backup_dir" -type f -name "*.tar.gz" -o -name "*.zip" 2>/dev/null | head -1)
        
        if [[ -n "$latest_file" ]]; then
            latest_time=$(stat -c %Y "$latest_file" 2>/dev/null || stat -f %m "$latest_file" 2>/dev/null)
            now_time=$(date +%s)
            age=$(( (now_time - latest_time) / 86400 ))
            
            if [[ "$age" -gt "$max_age_days" ]]; then
                check_result "backup_status" "warn" "最新备份已过期 ${age} 天"
                return 1
            else
                check_result "backup_status" "pass" "最新备份 ${age} 天前"
                return 0
            fi
        else
            check_result "backup_status" "warn" "无备份文件"
            return 1
        fi
    else
        check_result "backup_status" "warn" "备份目录不存在"
        return 1
    fi
}

# 检查日志错误
check_log_errors() {
    local log_file="$1"
    local max_errors="${2:-10}"
    
    log_info "检查日志错误: $log_file"
    
    if [[ -f "$log_file" ]]; then
        local error_count
        error_count=$(grep -c -E "ERROR|CRITICAL|FATAL|panic" "$log_file" 2>/dev/null || echo "0")
        
        if [[ "$error_count" -gt "$max_errors" ]]; then
            check_result "log_errors_$(basename "$log_file")" "warn" "发现 ${error_count} 个错误"
            return 1
        else
            check_result "log_errors_$(basename "$log_file")" "pass" "错误数 ${error_count}"
            return 0
        fi
    else
        check_result "log_errors_$(basename "$log_file")" "warn" "日志文件不存在"
        return 1
    fi
}

# ========== 主检查流程 ==========

run_all_checks() {
    echo "========================================"
    echo "NAS-OS 部署健康检查 v${SCRIPT_VERSION}"
    echo "时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "========================================"
    echo ""
    
    # 1. 核心服务检查
    echo "==> 核心服务检查"
    check_service_running "nasd"
    check_port_listening "nasd" "${SERVICE_PORTS[nasd]}"
    check_http_health "nasd" "http://localhost:${SERVICE_PORTS[nasd]}/api/v1/health"
    echo ""
    
    # 2. 监控服务检查
    echo "==> 监控服务检查"
    check_service_running "prometheus" || true
    check_port_listening "prometheus" "${SERVICE_PORTS[prometheus]}" || true
    check_prometheus_metrics "http://localhost:${SERVICE_PORTS[prometheus]}" || true
    
    check_service_running "grafana" || true
    check_port_listening "grafana" "${SERVICE_PORTS[grafana]}" || true
    echo ""
    
    # 3. 存储检查
    echo "==> 存储检查"
    check_storage_mount "/mnt" || true
    check_disk_space 90
    echo ""
    
    # 4. 系统资源检查
    echo "==> 系统资源检查"
    check_memory 90
    check_cpu_load 5
    echo ""
    
    # 5. 网络检查
    echo "==> 网络检查"
    check_network "localhost" || true
    check_dns "localhost" || true
    echo ""
    
    # 6. Docker 检查
    echo "==> Docker 检查"
    check_docker
    echo ""
    
    # 7. 备份检查
    echo "==> 备份检查"
    check_backup "/var/lib/nas-os/backups" 7
    echo ""
    
    # 8. 日志检查
    echo "==> 日志检查"
    if [[ -f "/var/log/nas-os/nasd.log" ]]; then
        check_log_errors "/var/log/nas-os/nasd.log" 10
    fi
    echo ""
}

generate_report() {
    local health_score=0
    
    if [[ $TOTAL_CHECKS -gt 0 ]]; then
        health_score=$(( (PASSED_CHECKS * 100) / TOTAL_CHECKS ))
    fi
    
    echo ""
    echo "========================================"
    echo "检查报告摘要"
    echo "========================================"
    echo ""
    echo "总检查项: $TOTAL_CHECKS"
    echo -e "通过: ${GREEN}$PASSED_CHECKS${NC}"
    echo -e "警告: ${YELLOW}$WARNING_CHECKS${NC}"
    echo -e "失败: ${RED}$FAILED_CHECKS${NC}"
    echo ""
    
    if [[ "$health_score" -ge 90 ]]; then
        echo -e "健康评分: ${GREEN}${health_score}%${NC} - 优秀"
    elif [[ "$health_score" -ge 70 ]]; then
        echo -e "健康评分: ${YELLOW}${health_score}%${NC} - 良好"
    else
        echo -e "健康评分: ${RED}${health_score}%${NC} - 需要关注"
    fi
    echo ""
    
    if [[ "$JSON_OUTPUT" == "true" ]]; then
        local json_output
        json_output=$(cat <<EOF
{
  "version": "$SCRIPT_VERSION",
  "timestamp": "$(date -Iseconds)",
  "total_checks": $TOTAL_CHECKS,
  "passed": $PASSED_CHECKS,
  "warnings": $WARNING_CHECKS,
  "failed": $FAILED_CHECKS,
  "health_score": $health_score,
  "status": "$([ "$FAILED_CHECKS" -eq 0 ] && echo "healthy" || echo "unhealthy")",
  "checks": {
EOF
        )
        
        local first=true
        for key in "${!CHECK_RESULTS[@]}"; do
            local value="${CHECK_RESULTS[$key]}"
            local status="${value%%:*}"
            local message="${value#*:}"
            
            if [[ "$first" == "true" ]]; then
                first=false
            else
                json_output+=","
            fi
            
            json_output+="\"$key\": {\"status\": \"$status\", \"message\": \"$message\"}"
        done
        
        json_output+="\n  }\n}"
        
        if [[ -n "$OUTPUT_FILE" ]]; then
            echo -e "$json_output" > "$OUTPUT_FILE"
            echo "报告已保存到: $OUTPUT_FILE"
        else
            echo -e "$json_output"
        fi
    fi
    
    # 返回状态码
    if [[ $FAILED_CHECKS -gt 0 ]]; then
        return 1
    elif [[ $WARNING_CHECKS -gt 0 ]]; then
        return 2
    else
        return 0
    fi
}

# ========== 主入口 ==========

main() {
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --verbose|-v)
                VERBOSE=true
                shift
                ;;
            --json|-j)
                JSON_OUTPUT=true
                shift
                ;;
            --output|-o)
                OUTPUT_FILE="$2"
                shift 2
                ;;
            --help|-h)
                echo "用法: $0 [选项]"
                echo ""
                echo "选项:"
                echo "  --verbose, -v    显示详细信息"
                echo "  --json, -j       输出 JSON 格式"
                echo "  --output, -o     指定输出文件"
                echo "  --help, -h       显示帮助信息"
                exit 0
                ;;
            *)
                echo "未知参数: $1"
                exit 1
                ;;
        esac
    done
    
    # 运行检查
    run_all_checks
    
    # 生成报告
    generate_report
}

main "$@"