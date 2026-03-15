#!/bin/bash
# =============================================================================
# NAS-OS 系统维护脚本 v2.68.0
# =============================================================================
# 用途：日志清理、临时文件清理、系统检查
# 用法：
#   ./maintenance.sh              # 执行所有维护任务
#   ./maintenance.sh --logs       # 仅清理日志
#   ./maintenance.sh --temp       # 仅清理临时文件
#   ./maintenance.sh --check      # 仅执行系统检查
#   ./maintenance.sh --all        # 执行所有任务（含报告）
#   ./maintenance.sh --status     # 查看维护状态
#   ./maintenance.sh --schedule   # 设置定时任务
# =============================================================================

set -euo pipefail

#===========================================
# 版本与路径
#===========================================

VERSION="2.68.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

#===========================================
# 配置
#===========================================

# 数据目录
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
TEMP_DIR="${TEMP_DIR:-/tmp/nas-os}"

# 清理配置
LOG_MAX_AGE="${LOG_MAX_AGE:-30}"           # 日志保留天数
LOG_MAX_SIZE="${LOG_MAX_SIZE:-500M}"       # 单日志文件最大大小
TEMP_MAX_AGE="${TEMP_MAX_AGE:-7}"          # 临时文件保留天数
BACKUP_MAX_AGE="${BACKUP_MAX_AGE:-90}"     # 备份保留天数
BACKUP_MAX_COUNT="${BACKUP_MAX_COUNT:-10}"  # 最大备份数量

# 压缩配置
COMPRESS_OLD_LOGS="${COMPRESS_OLD_LOGS:-true}"
COMPRESS_OLD_BACKUPS="${COMPRESS_OLD_BACKUPS:-true}"

# 系统检查阈值
DISK_THRESHOLD_WARNING="${DISK_THRESHOLD_WARNING:-80}"
DISK_THRESHOLD_CRITICAL="${DISK_THRESHOLD_CRITICAL:-90}"
MEMORY_THRESHOLD_WARNING="${MEMORY_THRESHOLD_WARNING:-80}"
MEMORY_THRESHOLD_CRITICAL="${MEMORY_THRESHOLD_CRITICAL:-90}"

# 报告输出
REPORT_FILE="${REPORT_FILE:-/var/log/nas-os/maintenance-report.log}"
REPORT_FORMAT="${REPORT_FORMAT:-text}"  # text | json

# 模拟模式
DRY_RUN="${DRY_RUN:-false}"

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

LOGS_CLEANED=0
TEMP_CLEANED=0
BACKUPS_CLEANED=0
SPACE_FREED=0
ERRORS=()
WARNINGS=()

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

# 转换大小字符串为字节
size_to_bytes() {
    local size="$1"
    local num=$(echo "$size" | grep -oE '[0-9]+')
    local unit=$(echo "$size" | grep -oE '[A-Za-z]+' | tr '[:lower:]' '[:upper:]')
    
    case "$unit" in
        K|KB) echo $((num * 1024)) ;;
        M|MB) echo $((num * 1024 * 1024)) ;;
        G|GB) echo $((num * 1024 * 1024 * 1024)) ;;
        T|TB) echo $((num * 1024 * 1024 * 1024 * 1024)) ;;
        *)    echo "$num" ;;
    esac
}

# 格式化字节大小
format_size() {
    local bytes="$1"
    
    if [ $bytes -ge 1073741824 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes/1073741824}") GB"
    elif [ $bytes -ge 1048576 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes/1048576}") MB"
    elif [ $bytes -ge 1024 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes/1024}") KB"
    else
        echo "$bytes B"
    fi
}

# 获取目录大小
get_dir_size() {
    local dir="$1"
    if [ -d "$dir" ]; then
        du -sb "$dir" 2>/dev/null | cut -f1
    else
        echo 0
    fi
}

# 安全删除文件
safe_remove() {
    local target="$1"
    
    if [ "$DRY_RUN" = true ]; then
        log INFO "[模拟] 将删除: $target"
        return 0
    fi
    
    if [ -e "$target" ]; then
        rm -rf "$target"
        return 0
    else
        return 1
    fi
}

# 显示帮助
show_help() {
    cat <<EOF
NAS-OS 系统维护工具 v${VERSION}

用法: $0 [options]

选项:
  --logs          清理日志文件
  --temp          清理临时文件
  --backups       清理旧备份
  --check         执行系统检查
  --all           执行所有维护任务
  --status        查看维护状态
  --schedule      设置定时任务（需要 root）
  --dry-run       模拟运行，不实际执行
  --report        生成维护报告
  -h, --help      显示帮助

环境变量:
  LOG_MAX_AGE        日志保留天数 (默认: 30)
  TEMP_MAX_AGE       临时文件保留天数 (默认: 7)
  BACKUP_MAX_AGE     备份保留天数 (默认: 90)
  BACKUP_MAX_COUNT   最大备份数量 (默认: 10)

示例:
  $0 --logs                   # 清理日志
  $0 --temp                   # 清理临时文件
  $0 --check                  # 系统检查
  $0 --all --report           # 全部维护并生成报告
  $0 --schedule               # 设置每日维护任务

EOF
}

#===========================================
# 日志清理
#===========================================

clean_logs() {
    log STEP "开始清理日志..."
    
    local before_size=$(get_dir_size "$LOG_DIR")
    local count=0
    
    if [ ! -d "$LOG_DIR" ]; then
        log WARN "日志目录不存在: $LOG_DIR"
        return 0
    fi
    
    # 1. 清理过期日志
    log INFO "清理 ${LOG_MAX_AGE} 天前的日志..."
    local old_logs=$(find "$LOG_DIR" -type f -name "*.log" -mtime +${LOG_MAX_AGE} 2>/dev/null)
    
    for log_file in $old_logs; do
        if safe_remove "$log_file"; then
            ((count++))
        fi
    done
    
    log SUCCESS "清理过期日志: $count 个文件"
    
    # 2. 清理压缩日志
    log INFO "清理 ${LOG_MAX_AGE} 天前的压缩日志..."
    local old_compressed=$(find "$LOG_DIR" -type f \( -name "*.gz" -o -name "*.zip" -o -name "*.bz2" \) -mtime +${LOG_MAX_AGE} 2>/dev/null)
    local compressed_count=0
    
    for file in $old_compressed; do
        if safe_remove "$file"; then
            ((compressed_count++))
        fi
    done
    
    log SUCCESS "清理压缩日志: $compressed_count 个文件"
    
    # 3. 压缩旧日志
    if [ "$COMPRESS_OLD_LOGS" = true ]; then
        log INFO "压缩 7 天前的日志..."
        local uncompressed=$(find "$LOG_DIR" -type f -name "*.log" -mtime +7 ! -name "*.gz" 2>/dev/null)
        local compress_count=0
        
        for file in $uncompressed; do
            if [ "$DRY_RUN" != true ]; then
                gzip -f "$file" 2>/dev/null && ((compress_count++))
            else
                ((compress_count++))
            fi
        done
        
        log SUCCESS "压缩日志: $compress_count 个文件"
    fi
    
    # 4. 清理空目录
    find "$LOG_DIR" -type d -empty -delete 2>/dev/null || true
    
    local after_size=$(get_dir_size "$LOG_DIR")
    local freed=$((before_size - after_size))
    
    LOGS_CLEANED=$((count + compressed_count))
    SPACE_FREED=$((SPACE_FREED + freed))
    
    log SUCCESS "日志清理完成，释放空间: $(format_size $freed)"
}

#===========================================
# 临时文件清理
#===========================================

clean_temp() {
    log STEP "开始清理临时文件..."
    
    local before_size=0
    local count=0
    
    # NAS-OS 临时目录
    if [ -d "$TEMP_DIR" ]; then
        before_size=$(get_dir_size "$TEMP_DIR")
        
        log INFO "清理临时目录: $TEMP_DIR"
        local old_temp=$(find "$TEMP_DIR" -type f -mtime +${TEMP_MAX_AGE} 2>/dev/null)
        
        for file in $old_temp; do
            if safe_remove "$file"; then
                ((count++))
            fi
        done
        
        local after_size=$(get_dir_size "$TEMP_DIR")
        local freed=$((before_size - after_size))
        SPACE_FREED=$((SPACE_FREED + freed))
        
        log SUCCESS "清理临时文件: $count 个，释放: $(format_size $freed)"
    fi
    
    # 系统临时目录中的 NAS-OS 文件
    local system_temp="/tmp"
    local nas_temp_files=$(find "$system_temp" -type f -name "nas-*" -mtime +${TEMP_MAX_AGE} 2>/dev/null)
    local system_count=0
    
    for file in $nas_temp_files; do
        if safe_remove "$file"; then
            ((system_count++))
        fi
    done
    
    if [ $system_count -gt 0 ]; then
        log SUCCESS "清理系统临时文件: $system_count 个"
    fi
    
    # 清理空目录
    find "$TEMP_DIR" -type d -empty -delete 2>/dev/null || true
    
    TEMP_CLEANED=$((count + system_count))
    
    log SUCCESS "临时文件清理完成"
}

#===========================================
# 备份清理
#===========================================

clean_backups() {
    log STEP "开始清理旧备份..."
    
    if [ ! -d "$BACKUP_DIR" ]; then
        log WARN "备份目录不存在: $BACKUP_DIR"
        return 0
    fi
    
    local before_size=$(get_dir_size "$BACKUP_DIR")
    local count=0
    
    # 1. 清理过期备份
    log INFO "清理 ${BACKUP_MAX_AGE} 天前的备份..."
    
    for subdir in versions db config; do
        if [ -d "$BACKUP_DIR/$subdir" ]; then
            local old_backups=$(find "$BACKUP_DIR/$subdir" -type f -mtime +${BACKUP_MAX_AGE} 2>/dev/null)
            
            for file in $old_backups; do
                if safe_remove "$file"; then
                    ((count++))
                fi
            done
        fi
    done
    
    log SUCCESS "清理过期备份: $count 个文件"
    
    # 2. 限制备份数量
    if [ -d "$BACKUP_DIR/versions" ]; then
        local version_count=$(find "$BACKUP_DIR/versions" -type f | wc -l)
        
        if [ $version_count -gt $BACKUP_MAX_COUNT ]; then
            local to_delete=$((version_count - BACKUP_MAX_COUNT))
            log INFO "保留最近 $BACKUP_MAX_COUNT 个版本备份，删除 $to_delete 个旧备份..."
            
            local deleted=$(ls -t "$BACKUP_DIR/versions" | tail -$to_delete | while read f; do
                safe_remove "$BACKUP_DIR/versions/$f"
            done | wc -l)
        fi
    fi
    
    # 3. 压缩旧备份
    if [ "$COMPRESS_OLD_BACKUPS" = true ]; then
        log INFO "压缩旧备份..."
        local uncompressed=$(find "$BACKUP_DIR/db" -type f -name "*.db" -mtime +7 ! -name "*.gz" 2>/dev/null)
        local compress_count=0
        
        for file in $uncompressed; do
            if [ "$DRY_RUN" != true ]; then
                gzip -f "$file" 2>/dev/null && ((compress_count++))
            else
                ((compress_count++))
            fi
        done
        
        if [ $compress_count -gt 0 ]; then
            log SUCCESS "压缩备份: $compress_count 个"
        fi
    fi
    
    local after_size=$(get_dir_size "$BACKUP_DIR")
    local freed=$((before_size - after_size))
    
    BACKUPS_CLEANED=$count
    SPACE_FREED=$((SPACE_FREED + freed))
    
    log SUCCESS "备份清理完成，释放空间: $(format_size $freed)"
}

#===========================================
# 系统检查
#===========================================

system_check() {
    log STEP "开始系统检查..."
    
    local issues=0
    local warnings=0
    
    echo ""
    echo "========================================"
    echo "  系统检查报告"
    echo "========================================"
    echo ""
    
    # 1. 磁盘空间检查
    log INFO "检查磁盘空间..."
    local disk_usage=$(df -h / | awk 'NR==2 {print $5}' | tr -d '%')
    local disk_available=$(df -h / | awk 'NR==2 {print $4}')
    
    echo "  根分区使用率: ${disk_usage}%"
    echo "  可用空间: ${disk_available}"
    
    if [ $disk_usage -ge $DISK_THRESHOLD_CRITICAL ]; then
        log ERROR "磁盘空间严重不足！使用率: ${disk_usage}%"
        ((issues++))
        ERRORS+=("磁盘空间严重不足: ${disk_usage}%")
    elif [ $disk_usage -ge $DISK_THRESHOLD_WARNING ]; then
        log WARN "磁盘空间不足警告。使用率: ${disk_usage}%"
        ((warnings++))
        WARNINGS+=("磁盘空间不足: ${disk_usage}%")
    fi
    
    # 检查数据目录
    if [ -d "$DATA_DIR" ]; then
        local data_usage=$(df -h "$DATA_DIR" 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
        echo "  数据目录使用率: ${data_usage}%"
    fi
    
    echo ""
    
    # 2. 内存检查
    log INFO "检查内存使用..."
    
    if command -v free &> /dev/null; then
        local mem_info=$(free -m | awk 'NR==2 {print $2,$3,$4,$7}')
        local mem_total=$(echo $mem_info | awk '{print $1}')
        local mem_used=$(echo $mem_info | awk '{print $2}')
        local mem_available=$(echo $mem_info | awk '{print $4}')
        local mem_percent=$((mem_used * 100 / mem_total))
        
        echo "  总内存: ${mem_total}MB"
        echo "  已使用: ${mem_used}MB (${mem_percent}%)"
        echo "  可用: ${mem_available}MB"
        
        if [ $mem_percent -ge $MEMORY_THRESHOLD_CRITICAL ]; then
            log ERROR "内存使用率过高！使用率: ${mem_percent}%"
            ((issues++))
            ERRORS+=("内存使用率过高: ${mem_percent}%")
        elif [ $mem_percent -ge $MEMORY_THRESHOLD_WARNING ]; then
            log WARN "内存使用率较高。使用率: ${mem_percent}%"
            ((warnings++))
            WARNINGS+=("内存使用率较高: ${mem_percent}%")
        fi
    fi
    
    echo ""
    
    # 3. CPU 负载检查
    log INFO "检查系统负载..."
    local load_avg=$(awk '{print $1,$2,$3}' /proc/loadavg 2>/dev/null || echo "N/A")
    local cpu_count=$(nproc 2>/dev/null || echo 1)
    
    echo "  CPU 核心数: $cpu_count"
    echo "  系统负载 (1/5/15m): $load_avg"
    
    local load_1m=$(echo $load_avg | awk '{print $1}')
    if [ "$load_1m" != "N/A" ]; then
        local load_int=$(echo "$load_1m" | awk '{print int($1)}')
        if [ $load_int -ge $cpu_count ]; then
            log WARN "系统负载较高: $load_1m"
            ((warnings++))
            WARNINGS+=("系统负载较高: $load_1m")
        fi
    fi
    
    echo ""
    
    # 4. 服务状态检查
    log INFO "检查服务状态..."
    
    local service_status
    if command -v systemctl &> /dev/null; then
        service_status=$(systemctl is-active nas-os 2>/dev/null || echo "unknown")
    elif pgrep -x nasd > /dev/null 2>&1; then
        service_status="running"
    else
        service_status="stopped"
    fi
    
    echo "  NAS-OS 服务状态: $service_status"
    
    if [ "$service_status" != "active" ] && [ "$service_status" != "running" ]; then
        log WARN "NAS-OS 服务未运行"
        ((warnings++))
        WARNINGS+=("NAS-OS 服务未运行")
    fi
    
    echo ""
    
    # 5. 数据库检查
    log INFO "检查数据库..."
    
    local db_path="$DATA_DIR/nas-os.db"
    if [ -f "$db_path" ]; then
        local db_size=$(du -h "$db_path" | cut -f1)
        echo "  数据库大小: $db_size"
        
        # SQLite 完整性检查
        if command -v sqlite3 &> /dev/null; then
            local integrity=$(sqlite3 "$db_path" "PRAGMA integrity_check;" 2>/dev/null)
            if [ "$integrity" = "ok" ]; then
                echo "  数据库完整性: ✓ 正常"
            else
                log ERROR "数据库完整性检查失败: $integrity"
                ((issues++))
                ERRORS+=("数据库完整性检查失败")
            fi
        fi
    else
        echo "  数据库文件: 不存在"
    fi
    
    echo ""
    
    # 6. 日志错误检查
    log INFO "检查最近日志错误..."
    
    if [ -f "$LOG_DIR/nasd.log" ]; then
        local error_count=$(grep -ci "error" "$LOG_DIR/nasd.log" 2>/dev/null || echo 0)
        local recent_errors=$(tail -1000 "$LOG_DIR/nasd.log" 2>/dev/null | grep -ci "error" || echo 0)
        
        echo "  最近错误数: $recent_errors"
        
        if [ $recent_errors -gt 10 ]; then
            log WARN "最近日志中发现较多错误: $recent_errors 个"
            ((warnings++))
            WARNINGS+=("日志中发现 $recent_errors 个错误")
        fi
    fi
    
    echo ""
    
    # 7. 网络检查
    log INFO "检查网络连接..."
    
    if command -v curl &> /dev/null; then
        if curl -sf --max-time 5 http://localhost:8080/api/v1/health > /dev/null 2>&1; then
            echo "  API 服务: ✓ 正常"
        else
            echo "  API 服务: ✗ 不可达"
            log WARN "API 健康检查失败"
            ((warnings++))
            WARNINGS+=("API 健康检查失败")
        fi
    fi
    
    echo ""
    echo "========================================"
    
    # 汇总
    if [ $issues -gt 0 ]; then
        log ERROR "发现 $issues 个严重问题"
        return 1
    elif [ $warnings -gt 0 ]; then
        log WARN "发现 $warnings 个警告"
        return 2
    else
        log SUCCESS "系统检查通过"
        return 0
    fi
}

#===========================================
# 维护状态
#===========================================

show_status() {
    echo ""
    echo "========================================"
    echo "  NAS-OS 维护状态"
    echo "========================================"
    echo ""
    
    # 目录大小
    echo "存储使用:"
    echo "  日志目录: $(format_size $(get_dir_size "$LOG_DIR"))"
    echo "  临时目录: $(format_size $(get_dir_size "$TEMP_DIR"))"
    echo "  备份目录: $(format_size $(get_dir_size "$BACKUP_DIR"))"
    echo ""
    
    # 文件统计
    echo "文件统计:"
    
    if [ -d "$LOG_DIR" ]; then
        local log_count=$(find "$LOG_DIR" -type f | wc -l)
        echo "  日志文件: $log_count 个"
    fi
    
    if [ -d "$TEMP_DIR" ]; then
        local temp_count=$(find "$TEMP_DIR" -type f | wc -l)
        echo "  临时文件: $temp_count 个"
    fi
    
    if [ -d "$BACKUP_DIR/versions" ]; then
        local backup_count=$(find "$BACKUP_DIR/versions" -type f | wc -l)
        echo "  版本备份: $backup_count 个"
    fi
    
    if [ -d "$BACKUP_DIR/db" ]; then
        local db_backup_count=$(find "$BACKUP_DIR/db" -type f | wc -l)
        echo "  数据库备份: $db_backup_count 个"
    fi
    
    echo ""
    
    # 定时任务
    echo "定时任务:"
    if crontab -l 2>/dev/null | grep -q "maintenance.sh"; then
        local schedule=$(crontab -l 2>/dev/null | grep "maintenance.sh" | head -1)
        echo "  ✓ 已配置: $schedule"
    else
        echo "  ✗ 未配置定时维护"
        echo "    使用 $0 --schedule 配置"
    fi
    
    echo ""
    echo "========================================"
}

#===========================================
# 定时任务设置
#===========================================

setup_schedule() {
    log STEP "设置定时维护任务..."
    
    if [ "$(id -u)" != "0" ]; then
        log ERROR "需要 root 权限设置定时任务"
        return 1
    fi
    
    local script_path=$(readlink -f "$0")
    local cron_entry="0 3 * * * $script_path --all --report >> /var/log/nas-os/maintenance-cron.log 2>&1"
    
    # 检查是否已存在
    if crontab -l 2>/dev/null | grep -q "maintenance.sh"; then
        log INFO "定时任务已存在，更新..."
        (crontab -l 2>/dev/null | grep -v "maintenance.sh"; echo "$cron_entry") | crontab -
    else
        log INFO "添加定时任务..."
        (crontab -l 2>/dev/null; echo "$cron_entry") | crontab -
    fi
    
    log SUCCESS "定时任务已设置: 每天凌晨 3:00 执行维护"
    echo ""
    echo "当前定时任务:"
    crontab -l 2>/dev/null | grep -v "^#" | grep -v "^$"
}

#===========================================
# 生成报告
#===========================================

generate_report() {
    local timestamp=$(date -Iseconds)
    local hostname=$(hostname)
    
    local report=""
    
    if [ "$REPORT_FORMAT" = "json" ]; then
        report=$(cat <<EOF
{
  "timestamp": "$timestamp",
  "hostname": "$hostname",
  "version": "$VERSION",
  "logs_cleaned": $LOGS_CLEANED,
  "temp_cleaned": $TEMP_CLEANED,
  "backups_cleaned": $BACKUPS_CLEANED,
  "space_freed_bytes": $SPACE_FREED,
  "space_freed_human": "$(format_size $SPACE_FREED)",
  "errors": $(printf '%s\n' "${ERRORS[@]}" | jq -R . | jq -s .),
  "warnings": $(printf '%s\n' "${WARNINGS[@]}" | jq -R . | jq -s .)
}
EOF
)
    else
        report=$(cat <<EOF
========================================
NAS-OS 维护报告
========================================
时间: $timestamp
主机: $hostname
版本: v$VERSION

清理统计:
  日志文件: $LOGS_CLEANED 个
  临时文件: $TEMP_CLEANED 个
  备份文件: $BACKUPS_CLEANED 个
  释放空间: $(format_size $SPACE_FREED)

错误: ${#ERRORS[@]} 个
警告: ${#WARNINGS[@]} 个
========================================
EOF
)
    fi
    
    # 输出到文件
    if [ "$DRY_RUN" != true ]; then
        mkdir -p "$(dirname "$REPORT_FILE")"
        echo "$report" >> "$REPORT_FILE"
    fi
    
    # 输出到终端
    echo ""
    echo "$report"
}

#===========================================
# 主入口
#===========================================

# 参数解析
DO_LOGS=false
DO_TEMP=false
DO_BACKUPS=false
DO_CHECK=false
DO_ALL=false
DO_STATUS=false
DO_SCHEDULE=false
DO_REPORT=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --logs)    DO_LOGS=true; shift ;;
        --temp)    DO_TEMP=true; shift ;;
        --backups) DO_BACKUPS=true; shift ;;
        --check)   DO_CHECK=true; shift ;;
        --all)      DO_ALL=true; shift ;;
        --status)   DO_STATUS=true; shift ;;
        --schedule) DO_SCHEDULE=true; shift ;;
        --dry-run)  DRY_RUN=true; shift ;;
        --report)   DO_REPORT=true; shift ;;
        -h|--help)  show_help; exit 0 ;;
        *)          log ERROR "未知参数: $1"; show_help; exit 1 ;;
    esac
done

# 显示标题
echo ""
echo "========================================"
echo "  NAS-OS 系统维护工具 v${VERSION}"
echo "========================================"
if [ "$DRY_RUN" = true ]; then
    echo "  [模拟模式]"
fi
echo ""

# 执行任务
if [ "$DO_STATUS" = true ]; then
    show_status
    exit 0
fi

if [ "$DO_SCHEDULE" = true ]; then
    setup_schedule
    exit 0
fi

# 如果没有指定任何任务，显示帮助
if [ "$DO_LOGS" = false ] && [ "$DO_TEMP" = false ] && [ "$DO_BACKUPS" = false ] \
   && [ "$DO_CHECK" = false ] && [ "$DO_ALL" = false ]; then
    show_help
    exit 0
fi

# 执行维护任务
START_TIME=$(date +%s)

if [ "$DO_ALL" = true ]; then
    DO_LOGS=true
    DO_TEMP=true
    DO_BACKUPS=true
    DO_CHECK=true
fi

[ "$DO_LOGS" = true ] && clean_logs
[ "$DO_TEMP" = true ] && clean_temp
[ "$DO_BACKUPS" = true ] && clean_backups
[ "$DO_CHECK" = true ] && system_check

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo ""
echo "========================================"
echo "  维护完成"
echo "  耗时: ${DURATION}s"
echo "========================================"

# 生成报告
if [ "$DO_REPORT" = true ]; then
    generate_report
fi

exit 0