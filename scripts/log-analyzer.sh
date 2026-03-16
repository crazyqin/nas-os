#!/bin/bash
# =============================================================================
# NAS-OS 日志分析脚本 v2.72.0
# =============================================================================
# 用途：分析日志文件，统计错误、警告、趋势
# 用法：
#   ./log-analyzer.sh                 # 分析所有日志
#   ./log-analyzer.sh --errors        # 仅显示错误
#   ./log-analyzer.sh --warnings      # 仅显示警告
#   ./log-analyzer.sh --trend         # 显示趋势分析
#   ./log-analyzer.sh --json          # JSON 输出
#   ./log-analyzer.sh --since 24h     # 分析最近 24 小时
#   ./log-analyzer.sh --report        # 生成报告
# =============================================================================

set -euo pipefail

#===========================================
# 版本与路径
#===========================================

VERSION="2.122.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

#===========================================
# 配置
#===========================================

# 日志目录
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 分析配置
DEFAULT_SINCE="7d"  # 默认分析最近 7 天
TOP_N="${TOP_N:-20}"  # 显示前 N 个结果

# 错误模式
ERROR_PATTERNS=(
    "ERROR"
    "FATAL"
    "CRITICAL"
    "panic:"
    "runtime error:"
)

WARNING_PATTERNS=(
    "WARN"
    "WARNING"
)

# 排除模式（误报）
EXCLUDE_PATTERNS=(
    "context canceled"
    "connection reset by peer"
)

# 报告配置
REPORT_DIR="${REPORT_DIR:-/var/log/nas-os/reports}"
REPORT_FORMAT="${REPORT_FORMAT:-text}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

#===========================================
# 统计变量
#===========================================

TOTAL_LINES=0
TOTAL_ERRORS=0
TOTAL_WARNINGS=0
ERROR_COUNTS=()
WARNING_COUNTS=()
declare -A ERROR_BY_DAY
declare -A WARNING_BY_DAY
declare -A ERROR_BY_TYPE

#===========================================
# 工具函数
#===========================================

log() {
    local level="$1"
    shift
    local msg="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case "$level" in
        ERROR)   echo -e "${RED}[$timestamp] [ERROR]${NC} $msg" >&2 ;;
        WARN)    echo -e "${YELLOW}[$timestamp] [WARN]${NC} $msg" ;;
        INFO)    echo -e "${BLUE}[$timestamp] [INFO]${NC} $msg" ;;
        SUCCESS) echo -e "${GREEN}[$timestamp] [OK]${NC} $msg" ;;
        STEP)    echo -e "${CYAN}[$timestamp] [STEP]${NC} $msg" ;;
        *)       echo "[$timestamp] $msg" ;;
    esac
}

# 解析时间范围
parse_duration() {
    local duration="$1"
    local unit="${duration: -1}"
    local value="${duration%?}"
    
    case "$unit" in
        h) echo "$((value * 3600))" ;;
        d) echo "$((value * 86400))" ;;
        w) echo "$((value * 604800))" ;;
        m) echo "$((value * 2592000))" ;;
        *) echo "0" ;;
    esac
}

# 格式化数字
format_number() {
    local num="$1"
    printf "%'d" "$num" 2>/dev/null || echo "$num"
}

# 检查命令是否存在
check_command() {
    command -v "$1" >/dev/null 2>&1
}

#===========================================
# 分析函数
#===========================================

# 获取日志文件列表
get_log_files() {
    local since="$1"
    local since_seconds=$(parse_duration "$since")
    local cutoff_date=$(date -d "@$(( $(date +%s) - since_seconds ))" '+%Y-%m-%d' 2>/dev/null || date '+%Y-%m-%d')
    
    # 查找日志文件
    find "$LOG_DIR" -name "*.log" -o -name "*.log.*" 2>/dev/null | sort
}

# 分析单个日志文件
analyze_log_file() {
    local file="$1"
    local temp_errors=$(mktemp)
    local temp_warnings=$(mktemp)
    
    # 构建排除模式
    local exclude_filter=""
    for pattern in "${EXCLUDE_PATTERNS[@]}"; do
        exclude_filter="$exclude_filter -v -e \"$pattern\""
    done
    
    # 统计行数
    local lines=$(wc -l < "$file" 2>/dev/null || echo 0)
    TOTAL_LINES=$((TOTAL_LINES + lines))
    
    # 构建错误模式
    local error_pattern=$(IFS='|'; echo "${ERROR_PATTERNS[*]}")
    local warning_pattern=$(IFS='|'; echo "${WARNING_PATTERNS[*]}")
    
    # 统计错误
    local errors=$(grep -cE "$error_pattern" "$file" 2>/dev/null || echo 0)
    TOTAL_ERRORS=$((TOTAL_ERRORS + errors))
    
    # 统计警告
    local warnings=$(grep -cE "$warning_pattern" "$file" 2>/dev/null || echo 0)
    TOTAL_WARNINGS=$((TOTAL_WARNINGS + warnings))
    
    # 提取错误详情
    if [[ $errors -gt 0 ]]; then
        grep -E "$error_pattern" "$file" 2>/dev/null | while read -r line; do
            # 提取日期
            local date=$(echo "$line" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1)
            [[ -n "$date" ]] && ERROR_BY_DAY["$date"]=$(( ${ERROR_BY_DAY["$date"]:-0} + 1 ))
            
            # 提取错误类型
            local error_type=$(echo "$line" | grep -oE '(ERROR|FATAL|CRITICAL|panic:)' | head -1)
            [[ -n "$error_type" ]] && ERROR_BY_TYPE["$error_type"]=$(( ${ERROR_BY_TYPE["$error_type"]:-0} + 1 ))
        done
    fi
    
    # 提取警告详情
    if [[ $warnings -gt 0 ]]; then
        grep -E "$warning_pattern" "$file" 2>/dev/null | while read -r line; do
            local date=$(echo "$line" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1)
            [[ -n "$date" ]] && WARNING_BY_DAY["$date"]=$(( ${WARNING_BY_DAY["$date"]:-0} + 1 ))
        done
    fi
    
    rm -f "$temp_errors" "$temp_warnings"
}

# 分析所有日志
analyze_all_logs() {
    local since="${1:-$DEFAULT_SINCE}"
    
    log STEP "分析日志（最近 $since）..."
    
    local files=$(get_log_files "$since")
    
    if [[ -z "$files" ]]; then
        log WARN "未找到日志文件: $LOG_DIR"
        return 1
    fi
    
    local count=0
    for file in $files; do
        [[ -f "$file" ]] || continue
        analyze_log_file "$file"
        count=$((count + 1))
    done
    
    log SUCCESS "分析了 $count 个日志文件"
}

# 显示错误摘要
show_errors() {
    echo ""
    echo -e "${RED}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${RED}                       错误摘要                            ${NC}"
    echo -e "${RED}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo "  总错误数: $(format_number $TOTAL_ERRORS)"
    echo ""
    
    if [[ ${#ERROR_BY_TYPE[@]} -gt 0 ]]; then
        echo "  按类型统计:"
        for type in "${!ERROR_BY_TYPE[@]}"; do
            printf "    %-15s %s\n" "$type" "$(format_number ${ERROR_BY_TYPE[$type]})"
        done
    fi
    
    if [[ ${#ERROR_BY_DAY[@]} -gt 0 ]]; then
        echo ""
        echo "  按日期统计:"
        for date in $(echo "${!ERROR_BY_DAY[@]}" | tr ' ' '\n' | sort); do
            printf "    %-12s %s\n" "$date" "$(format_number ${ERROR_BY_DAY[$date]})"
        done
    fi
    
    echo ""
}

# 显示警告摘要
show_warnings() {
    echo ""
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}                       警告摘要                            ${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo "  总警告数: $(format_number $TOTAL_WARNINGS)"
    echo ""
    
    if [[ ${#WARNING_BY_DAY[@]} -gt 0 ]]; then
        echo "  按日期统计:"
        for date in $(echo "${!WARNING_BY_DAY[@]}" | tr ' ' '\n' | sort); do
            printf "    %-12s %s\n" "$date" "$(format_number ${WARNING_BY_DAY[$date]})"
        done
    fi
    
    echo ""
}

# 显示趋势分析
show_trend() {
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                       趋势分析                            ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    
    # 计算平均每日错误
    local days=${#ERROR_BY_DAY[@]}
    if [[ $days -gt 0 ]]; then
        local avg_errors=$((TOTAL_ERRORS / days))
        echo "  平均每日错误: $avg_errors"
    fi
    
    # 计算平均每日警告
    local warn_days=${#WARNING_BY_DAY[@]}
    if [[ $warn_days -gt 0 ]]; then
        local avg_warnings=$((TOTAL_WARNINGS / warn_days))
        echo "  平均每日警告: $avg_warnings"
    fi
    
    # 显示趋势图
    echo ""
    echo "  错误趋势:"
    for date in $(echo "${!ERROR_BY_DAY[@]}" | tr ' ' '\n' | sort | tail -7); do
        local count=${ERROR_BY_DAY[$date]:-0}
        local bar=""
        local i=0
        while [[ $i -lt $((count / 10)) ]]; do
            bar="${bar}█"
            i=$((i + 1))
        done
        printf "    %-12s %4d %s\n" "$date" "$count" "$bar"
    done
    
    echo ""
}

# 显示总体统计
show_summary() {
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}                       总体统计                            ${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo "  分析时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "  日志目录: $LOG_DIR"
    echo ""
    echo "  总日志行数: $(format_number $TOTAL_LINES)"
    echo "  总错误数:   $(format_number $TOTAL_ERRORS)"
    echo "  总警告数:   $(format_number $TOTAL_WARNINGS)"
    
    if [[ $TOTAL_LINES -gt 0 ]]; then
        local error_rate=$(echo "scale=4; $TOTAL_ERRORS * 100 / $TOTAL_LINES" | bc 2>/dev/null || echo "0")
        local warn_rate=$(echo "scale=4; $TOTAL_WARNINGS * 100 / $TOTAL_LINES" | bc 2>/dev/null || echo "0")
        echo ""
        echo "  错误率: ${error_rate}%"
        echo "  警告率: ${warn_rate}%"
    fi
    
    echo ""
}

# 生成报告
generate_report() {
    local output_file="$1"
    local format="${2:-text}"
    
    mkdir -p "$(dirname "$output_file")"
    
    if [[ "$format" == "json" ]]; then
        cat > "$output_file" <<EOF
{
  "timestamp": "$(date -Iseconds)",
  "version": "$VERSION",
  "summary": {
    "total_lines": $TOTAL_LINES,
    "total_errors": $TOTAL_ERRORS,
    "total_warnings": $TOTAL_WARNINGS
  },
  "errors_by_type": $(declare -p ERROR_BY_TYPE 2>/dev/null | sed 's/declare -A ERROR_BY_TYPE=//' || echo "{}"),
  "errors_by_day": $(declare -p ERROR_BY_DAY 2>/dev/null | sed 's/declare -A ERROR_BY_DAY=//' || echo "{}"),
  "warnings_by_day": $(declare -p WARNING_BY_DAY 2>/dev/null | sed 's/declare -A WARNING_BY_DAY=//' || echo "{}")
}
EOF
    else
        {
            echo "NAS-OS 日志分析报告"
            echo "===================="
            echo ""
            echo "分析时间: $(date '+%Y-%m-%d %H:%M:%S')"
            echo "版本: $VERSION"
            echo ""
            echo "总体统计"
            echo "--------"
            echo "总日志行数: $(format_number $TOTAL_LINES)"
            echo "总错误数:   $(format_number $TOTAL_ERRORS)"
            echo "总警告数:   $(format_number $TOTAL_WARNINGS)"
            echo ""
        } > "$output_file"
    fi
    
    log SUCCESS "报告已生成: $output_file"
}

# 显示帮助
show_help() {
    cat <<EOF
NAS-OS 日志分析工具 v${VERSION}

用法: $0 [options]

选项:
  --errors        仅显示错误摘要
  --warnings      仅显示警告摘要
  --trend         显示趋势分析
  --json          JSON 格式输出
  --since DUR     分析时间范围 (默认: 7d, 支持: h/d/w/m)
  --report FILE   生成报告文件
  --top N         显示前 N 个结果 (默认: 20)
  -h, --help      显示帮助

示例:
  $0                      # 完整分析
  $0 --errors             # 仅错误
  $0 --since 24h          # 最近 24 小时
  $0 --report report.txt  # 生成报告

环境变量:
  LOG_DIR         日志目录 (默认: /var/log/nas-os)
  TOP_N           显示前 N 个结果 (默认: 20)
EOF
}

#===========================================
# 主函数
#===========================================

main() {
    local show_errors=false
    local show_warnings=false
    local show_trend=false
    local json_output=false
    local since="$DEFAULT_SINCE"
    local report_file=""
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --errors)
                show_errors=true
                shift
                ;;
            --warnings)
                show_warnings=true
                shift
                ;;
            --trend)
                show_trend=true
                shift
                ;;
            --json)
                json_output=true
                shift
                ;;
            --since)
                since="$2"
                shift 2
                ;;
            --report)
                report_file="$2"
                shift 2
                ;;
            --top)
                TOP_N="$2"
                shift 2
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
    
    # 检查日志目录
    if [[ ! -d "$LOG_DIR" ]]; then
        log WARN "日志目录不存在: $LOG_DIR"
        log INFO "创建日志目录..."
        mkdir -p "$LOG_DIR"
    fi
    
    # 分析日志
    analyze_all_logs "$since"
    
    # 显示结果
    if [[ "$json_output" == "true" ]]; then
        generate_report "/dev/stdout" "json"
    else
        show_summary
        
        if [[ "$show_errors" == "true" ]] || [[ "$show_warnings" == "false" && "$show_trend" == "false" ]]; then
            show_errors
        fi
        
        if [[ "$show_warnings" == "true" ]] || [[ "$show_errors" == "false" && "$show_trend" == "false" ]]; then
            show_warnings
        fi
        
        if [[ "$show_trend" == "true" ]]; then
            show_trend
        fi
    fi
    
    # 生成报告
    if [[ -n "$report_file" ]]; then
        generate_report "$report_file" "$REPORT_FORMAT"
    fi
}

# 运行
main "$@"