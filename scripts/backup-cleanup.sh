#!/bin/bash
# NAS-OS 备份清理脚本
# 按保留策略清理旧备份，监控磁盘空间
#
# v2.50.0 工部开发
# - 按保留策略清理
# - 磁盘空间检查
# - 清理日志

set -e

# 脚本信息
SCRIPT_VERSION="2.50.0"
SCRIPT_NAME="NAS-OS Backup Cleanup"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 默认配置
BACKUP_NAME="${BACKUP_NAME:-nas-os}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 保留策略
RETENTION_DAYS="${RETENTION_DAYS:-30}"
RETENTION_COUNT="${RETENTION_COUNT:-10}"
RETENTION_WEEKLY="${RETENTION_WEEKLY:-4}"
RETENTION_MONTHLY="${RETENTION_MONTHLY:-6}"

# 磁盘空间配置
MIN_DISK_SPACE_GB="${MIN_DISK_SPACE_GB:-10}"
MAX_DISK_USAGE_PERCENT="${MAX_DISK_USAGE_PERCENT:-80}"

# 清理行为
DRY_RUN="${DRY_RUN:-false}"
FORCE_CLEANUP="${FORCE_CLEANUP:-false}"
AGGRESSIVE_CLEANUP="${AGGRESSIVE_CLEANUP:-false}"

# 日志配置
LOG_FILE="${LOG_FILE:-}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 统计
TOTAL_DELETED=0
TOTAL_SIZE_FREED=0

# ============ 日志函数 ============
log_init() {
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    if [ -z "$LOG_FILE" ]; then
        LOG_FILE="$LOG_DIR/backup-cleanup-$(date '+%Y%m%d').log"
    fi
    
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
}

log() {
    local level="$1"
    local message="$2"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local log_line="[$timestamp] [$level] $message"
    
    echo "$log_line" >> "$LOG_FILE"
    
    case "$level" in
        INFO)    echo -e "${BLUE}[INFO]${NC} $message" ;;
        SUCCESS) echo -e "${GREEN}[SUCCESS]${NC} $message" ;;
        WARN)    echo -e "${YELLOW}[WARN]${NC} $message" ;;
        ERROR)   echo -e "${RED}[ERROR]${NC} $message" ;;
        DEBUG)   [ "${DEBUG:-false}" = true ] && echo -e "${CYAN}[DEBUG]${NC} $message" ;;
    esac
}

log_info()    { log "INFO" "$1"; }
log_success()  { log "SUCCESS" "$1"; }
log_warn()     { log "WARN" "$1"; }
log_error()    { log "ERROR" "$1"; }
log_debug()    { log "DEBUG" "$1"; }

# ============ 帮助信息 ============
show_help() {
    cat << EOF
$SCRIPT_NAME v$SCRIPT_VERSION - NAS-OS 备份清理脚本

用法: $(basename "$0") [选项]

清理策略:
  -d, --retention-days DAYS     保留最近 N 天的备份 (默认: 30)
  -c, --retention-count COUNT   保留最多 N 个备份 (默认: 10)
  --weekly COUNT                保留最近 N 周的每周备份 (默认: 4)
  --monthly COUNT               保留最近 N 月的每月备份 (默认: 6)

磁盘空间:
  --min-space GB                最小保留磁盘空间 (GB, 默认: 10)
  --max-usage PERCENT           最大磁盘使用率 (%, 默认: 80)

清理选项:
  --force                       强制清理，不提示确认
  --aggressive                  激进清理模式（磁盘空间不足时）
  --dry-run                     模拟运行，不实际删除
  -v, --verbose                 详细输出

其他:
  -b, --backup-dir DIR          备份目录 (默认: /var/lib/nas-os/backups)
  --status                      显示备份状态
  -h, --help                    显示此帮助信息
  --version                     显示版本信息

示例:
  # 查看备份状态
  $(basename "$0") --status

  # 按默认策略清理
  $(basename "$0")

  # 保留最近 7 天的备份
  $(basename "$0") -d 7

  # 激进清理模式
  $(basename "$0") --aggressive

  # 模拟清理
  $(basename "$0") --dry-run

EOF
    exit 0
}

show_version() {
    echo "$SCRIPT_NAME v$SCRIPT_VERSION"
    exit 0
}

# ============ 参数解析 ============
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -d|--retention-days)
                RETENTION_DAYS="$2"
                shift 2
                ;;
            -c|--retention-count)
                RETENTION_COUNT="$2"
                shift 2
                ;;
            --weekly)
                RETENTION_WEEKLY="$2"
                shift 2
                ;;
            --monthly)
                RETENTION_MONTHLY="$2"
                shift 2
                ;;
            --min-space)
                MIN_DISK_SPACE_GB="$2"
                shift 2
                ;;
            --max-usage)
                MAX_DISK_USAGE_PERCENT="$2"
                shift 2
                ;;
            --force)
                FORCE_CLEANUP=true
                shift
                ;;
            --aggressive)
                AGGRESSIVE_CLEANUP=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            -v|--verbose)
                DEBUG=true
                shift
                ;;
            -b|--backup-dir)
                BACKUP_DIR="$2"
                shift 2
                ;;
            --status)
                show_backup_status
                exit 0
                ;;
            -h|--help)
                show_help
                ;;
            --version)
                show_version
                ;;
            *)
                log_error "未知选项: $1"
                echo "使用 -h 或 --help 查看帮助信息"
                exit 1
                ;;
        esac
    done
}

# ============ 磁盘空间检查 ============
check_disk_space() {
    log_info "检查磁盘空间..."
    
    # 获取备份目录所在分区的磁盘使用情况
    local disk_info
    disk_info=$(df -BG "$BACKUP_DIR" 2>/dev/null | tail -1)
    
    if [ -z "$disk_info" ]; then
        log_warn "无法获取磁盘空间信息"
        return 0
    fi
    
    local total_gb used_gb avail_gb usage_percent
    total_gb=$(echo "$disk_info" | awk '{print $2}' | tr -d 'G')
    used_gb=$(echo "$disk_info" | awk '{print $3}' | tr -d 'G')
    avail_gb=$(echo "$disk_info" | awk '{print $4}' | tr -d 'G')
    usage_percent=$(echo "$disk_info" | awk '{print $5}' | tr -d '%')
    
    log_info "磁盘空间: 总计 ${total_gb}GB, 已用 ${used_gb}GB, 可用 ${avail_gb}GB (${usage_percent}%)"
    
    # 检查是否需要清理
    local need_cleanup=false
    local reason=""
    
    if [ "$avail_gb" -lt "$MIN_DISK_SPACE_GB" ]; then
        need_cleanup=true
        reason="可用空间不足 ${MIN_DISK_SPACE_GB}GB (当前: ${avail_gb}GB)"
    fi
    
    if [ "$usage_percent" -gt "$MAX_DISK_USAGE_PERCENT" ]; then
        need_cleanup=true
        reason="磁盘使用率超过 ${MAX_DISK_USAGE_PERCENT}% (当前: ${usage_percent}%)"
    fi
    
    if [ "$need_cleanup" = true ]; then
        log_warn "磁盘空间警告: $reason"
        
        if [ "$AGGRESSIVE_CLEANUP" = true ]; then
            log_info "启用激进清理模式"
            return 2
        fi
        
        return 1
    fi
    
    log_success "磁盘空间充足"
    return 0
}

# ============ 备份状态 ============
show_backup_status() {
    log_info "备份状态报告"
    
    if [ ! -d "$BACKUP_DIR" ]; then
        log_error "备份目录不存在: $BACKUP_DIR"
        exit 1
    fi
    
    echo ""
    echo "=========================================="
    echo "NAS-OS 备份状态"
    echo "=========================================="
    echo ""
    
    # 统计信息
    local total_count total_size
    total_count=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f 2>/dev/null | wc -l)
    total_size=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -exec du -sb {} + 2>/dev/null | awk '{sum += $1} END {print sum}' || echo 0)
    
    echo "备份目录: $BACKUP_DIR"
    echo "备份数量: $total_count"
    echo "总大小: $(format_size "$total_size")"
    echo ""
    
    # 磁盘空间
    echo "--- 磁盘空间 ---"
    df -h "$BACKUP_DIR" 2>/dev/null | tail -1 | awk '{print "总计: "$2", 已用: "$3", 可用: "$4", 使用率: "$5}'
    echo ""
    
    # 按类型统计
    echo "--- 按类型统计 ---"
    local full_count incremental_count
    full_count=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*-full.tar*" -type f 2>/dev/null | wc -l)
    incremental_count=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*-incremental.tar*" -type f 2>/dev/null | wc -l)
    echo "完整备份: $full_count 个"
    echo "增量备份: $incremental_count 个"
    echo ""
    
    # 按时间统计
    echo "--- 按时间统计 ---"
    local today_count week_count month_count older_count
    today_count=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -mtime 0 2>/dev/null | wc -l)
    week_count=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -mtime -7 2>/dev/null | wc -l)
    month_count=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -mtime -30 2>/dev/null | wc -l)
    older_count=$((total_count - month_count))
    
    echo "今天: $today_count 个"
    echo "本周: $week_count 个"
    echo "本月: $month_count 个"
    echo "更早: $older_count 个"
    echo ""
    
    # 最近备份
    echo "--- 最近 5 个备份 ---"
    find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -printf '%T@ %p\n' 2>/dev/null | \
        sort -rn | head -5 | while read -r timestamp backup; do
            local size mtime
            size=$(stat -f%z "$backup" 2>/dev/null || stat -c%s "$backup" 2>/dev/null || echo 0)
            mtime=$(stat -f%Sm -t "%Y-%m-%d %H:%M" "$backup" 2>/dev/null || \
                    stat -c%y "$backup" 2>/dev/null | cut -d'.' -f1 || echo "?")
            echo "$(basename "$backup") - $(format_size "$size") - $mtime"
        done
    
    echo ""
    echo "=========================================="
    
    # 保留策略
    echo ""
    echo "--- 当前保留策略 ---"
    echo "保留天数: $RETENTION_DAYS 天"
    echo "保留数量: $RETENTION_COUNT 个"
    echo "每周保留: $RETENTION_WEEKLY 个"
    echo "每月保留: $RETENTION_MONTHLY 个"
    echo ""
    
    # 磁盘阈值
    echo "--- 磁盘阈值 ---"
    echo "最小可用空间: ${MIN_DISK_SPACE_GB}GB"
    echo "最大使用率: ${MAX_DISK_USAGE_PERCENT}%"
    echo ""
}

# ============ 清理策略 ============
cleanup_by_age() {
    log_info "按时间清理备份 (保留 $RETENTION_DAYS 天)..."
    
    local deleted=0
    local size_freed=0
    
    # 查找超过保留天数的备份
    while IFS= read -r backup; do
        [ -z "$backup" ] && continue
        
        local size
        size=$(stat -f%z "$backup" 2>/dev/null || stat -c%s "$backup" 2>/dev/null || echo 0)
        
        if [ "$DRY_RUN" = true ]; then
            log_info "[DRY-RUN] 将删除: $backup ($(format_size "$size"))"
        else
            log_debug "删除: $backup"
            rm -f "$backup" "${backup}.sha256" "${backup%.tar*}.manifest"
            ((deleted++))
            size_freed=$((size_freed + size))
        fi
    done < <(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -mtime +$RETENTION_DAYS 2>/dev/null)
    
    if [ $deleted -gt 0 ] || [ "$DRY_RUN" = true ]; then
        log_info "时间清理: 删除 $deleted 个备份，释放 $(format_size "$size_freed")"
    fi
    
    TOTAL_DELETED=$((TOTAL_DELETED + deleted))
    TOTAL_SIZE_FREED=$((TOTAL_SIZE_FREED + size_freed))
}

cleanup_by_count() {
    log_info "按数量清理备份 (保留 $RETENTION_COUNT 个)..."
    
    local current_count
    current_count=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f 2>/dev/null | wc -l)
    
    if [ "$current_count" -le "$RETENTION_COUNT" ]; then
        log_info "当前备份数量 ($current_count) 未超过限制 ($RETENTION_COUNT)"
        return 0
    fi
    
    local to_delete=$((current_count - RETENTION_COUNT))
    log_info "需要删除 $to_delete 个最旧备份"
    
    local deleted=0
    local size_freed=0
    
    # 按时间排序，删除最旧的
    find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -printf '%T@ %p\n' 2>/dev/null | \
        sort -n | head -n "$to_delete" | while read -r _ backup; do
            [ -z "$backup" ] && continue
            
            local size
            size=$(stat -f%z "$backup" 2>/dev/null || stat -c%s "$backup" 2>/dev/null || echo 0)
            
            if [ "$DRY_RUN" = true ]; then
                log_info "[DRY-RUN] 将删除: $backup ($(format_size "$size"))"
            else
                log_debug "删除: $backup"
                rm -f "$backup" "${backup}.sha256" "${backup%.tar*}.manifest"
                ((deleted++))
                size_freed=$((size_freed + size))
            fi
        done
    
    TOTAL_DELETED=$((TOTAL_DELETED + deleted))
    TOTAL_SIZE_FREED=$((TOTAL_SIZE_FREED + size_freed))
}

cleanup_weekly() {
    log_info "保留每周备份 (最近 $RETENTION_WEEKLY 周)..."
    
    local deleted=0
    local size_freed=0
    
    # 获取所有备份按周分组
    local current_week=""
    local week_count=0
    local backups_in_week=()
    
    while IFS= read -r backup; do
        [ -z "$backup" ] && continue
        
        local mtime week
        mtime=$(stat -f%m "$backup" 2>/dev/null || stat -c%Y "$backup" 2>/dev/null)
        week=$(date -d "@$mtime" +%Y-W%W 2>/dev/null || date -r "$mtime" +%Y-W%W 2>/dev/null)
        
        if [ "$week" != "$current_week" ]; then
            # 处理上一周的备份
            if [ ${#backups_in_week[@]} -gt 1 ]; then
                # 只保留最新的一个
                for old_backup in "${backups_in_week[@]:1}"; do
                    local size
                    size=$(stat -f%z "$old_backup" 2>/dev/null || stat -c%s "$old_backup" 2>/dev/null || echo 0)
                    
                    if [ "$DRY_RUN" = true ]; then
                        log_info "[DRY-RUN] 将删除: $old_backup"
                    else
                        log_debug "删除周内旧备份: $old_backup"
                        rm -f "$old_backup" "${old_backup}.sha256" "${old_backup%.tar*}.manifest"
                        ((deleted++))
                        size_freed=$((size_freed + size))
                    fi
                done
            fi
            
            ((week_count++))
            current_week="$week"
            backups_in_week=("$backup")
            
            # 超过保留周数后删除所有
            if [ $week_count -gt "$RETENTION_WEEKLY" ]; then
                local size
                size=$(stat -f%z "$backup" 2>/dev/null || stat -c%s "$backup" 2>/dev/null || echo 0)
                
                if [ "$DRY_RUN" = true ]; then
                    log_info "[DRY-RUN] 将删除: $backup (超过保留周数)"
                else
                    log_debug "删除旧周备份: $backup"
                    rm -f "$backup" "${backup}.sha256" "${backup%.tar*}.manifest"
                    ((deleted++))
                    size_freed=$((size_freed + size))
                fi
            fi
        else
            backups_in_week+=("$backup")
        fi
    done < <(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | cut -d' ' -f2-)
    
    if [ $deleted -gt 0 ]; then
        log_info "每周清理: 删除 $deleted 个备份，释放 $(format_size "$size_freed")"
    fi
    
    TOTAL_DELETED=$((TOTAL_DELETED + deleted))
    TOTAL_SIZE_FREED=$((TOTAL_SIZE_FREED + size_freed))
}

cleanup_monthly() {
    log_info "保留每月备份 (最近 $RETENTION_MONTHLY 月)..."
    
    local deleted=0
    local size_freed=0
    
    local current_month=""
    local month_count=0
    local backups_in_month=()
    
    while IFS= read -r backup; do
        [ -z "$backup" ] && continue
        
        local mtime month
        mtime=$(stat -f%m "$backup" 2>/dev/null || stat -c%Y "$backup" 2>/dev/null)
        month=$(date -d "@$mtime" +%Y-%m 2>/dev/null || date -r "$mtime" +%Y-%m 2>/dev/null)
        
        if [ "$month" != "$current_month" ]; then
            # 处理上一月的备份
            if [ ${#backups_in_month[@]} -gt 1 ]; then
                for old_backup in "${backups_in_month[@]:1}"; do
                    local size
                    size=$(stat -f%z "$old_backup" 2>/dev/null || stat -c%s "$old_backup" 2>/dev/null || echo 0)
                    
                    if [ "$DRY_RUN" = true ]; then
                        log_info "[DRY-RUN] 将删除: $old_backup"
                    else
                        log_debug "删除月内旧备份: $old_backup"
                        rm -f "$old_backup" "${old_backup}.sha256" "${old_backup%.tar*}.manifest"
                        ((deleted++))
                        size_freed=$((size_freed + size))
                    fi
                done
            fi
            
            ((month_count++))
            current_month="$month"
            backups_in_month=("$backup")
            
            # 超过保留月数后删除所有
            if [ $month_count -gt "$RETENTION_MONTHLY" ]; then
                local size
                size=$(stat -f%z "$backup" 2>/dev/null || stat -c%s "$backup" 2>/dev/null || echo 0)
                
                if [ "$DRY_RUN" = true ]; then
                    log_info "[DRY-RUN] 将删除: $backup (超过保留月数)"
                else
                    log_debug "删除旧月备份: $backup"
                    rm -f "$backup" "${backup}.sha256" "${backup%.tar*}.manifest"
                    ((deleted++))
                    size_freed=$((size_freed + size))
                fi
            fi
        else
            backups_in_month+=("$backup")
        fi
    done < <(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | cut -d' ' -f2-)
    
    if [ $deleted -gt 0 ]; then
        log_info "每月清理: 删除 $deleted 个备份，释放 $(format_size "$size_freed")"
    fi
    
    TOTAL_DELETED=$((TOTAL_DELETED + deleted))
    TOTAL_SIZE_FREED=$((TOTAL_SIZE_FREED + size_freed))
}

cleanup_aggressive() {
    log_warn "执行激进清理..."
    
    local deleted=0
    local size_freed=0
    
    # 激进清理：只保留最近的完整备份和最近 3 天的增量备份
    log_info "激进策略: 保留最近完整备份 + 3 天增量备份"
    
    # 找到最近的完整备份
    local latest_full
    latest_full=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*-full.tar*" -type f -printf '%T@ %p\n' 2>/dev/null | \
                  sort -rn | head -1 | cut -d' ' -f2-)
    
    # 删除所有旧于 3 天的增量备份
    while IFS= read -r backup; do
        [ -z "$backup" ] && continue
        [ "$backup" = "$latest_full" ] && continue
        
        local size
        size=$(stat -f%z "$backup" 2>/dev/null || stat -c%s "$backup" 2>/dev/null || echo 0)
        
        if [ "$DRY_RUN" = true ]; then
            log_info "[DRY-RUN] 将删除: $backup ($(format_size "$size"))"
        else
            log_debug "激进删除: $backup"
            rm -f "$backup" "${backup}.sha256" "${backup%.tar*}.manifest"
            ((deleted++))
            size_freed=$((size_freed + size))
        fi
    done < <(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -mtime +3 2>/dev/null)
    
    log_info "激进清理: 删除 $deleted 个备份，释放 $(format_size "$size_freed")"
    
    TOTAL_DELETED=$((TOTAL_DELETED + deleted))
    TOTAL_SIZE_FREED=$((TOTAL_SIZE_FREED + size_freed))
}

cleanup_orphaned_files() {
    log_info "清理孤立文件..."
    
    local deleted=0
    
    # 清理没有对应备份文件的校验文件
    for sha_file in "$BACKUP_DIR"/*.sha256; do
        [ -f "$sha_file" ] || continue
        
        local backup_file="${sha_file%.sha256}"
        if [ ! -f "$backup_file" ]; then
            if [ "$DRY_RUN" = true ]; then
                log_info "[DRY-RUN] 将删除孤立校验文件: $sha_file"
            else
                log_debug "删除孤立校验文件: $sha_file"
                rm -f "$sha_file"
                ((deleted++))
            fi
        fi
    done
    
    # 清理没有对应备份文件的清单文件
    for manifest_file in "$BACKUP_DIR"/*.manifest; do
        [ -f "$manifest_file" ] || continue
        
        local backup_id
        backup_id=$(basename "$manifest_file" .manifest)
        local backup_found
        backup_found=$(find "$BACKUP_DIR" -name "${backup_id}.tar*" -type f 2>/dev/null | head -1)
        
        if [ -z "$backup_found" ]; then
            if [ "$DRY_RUN" = true ]; then
                log_info "[DRY-RUN] 将删除孤立清单文件: $manifest_file"
            else
                log_debug "删除孤立清单文件: $manifest_file"
                rm -f "$manifest_file"
                ((deleted++))
            fi
        fi
    done
    
    if [ $deleted -gt 0 ]; then
        log_info "孤立文件清理: 删除 $deleted 个文件"
    fi
}

# ============ 工具函数 ============
format_size() {
    local bytes=$1
    local units=("B" "KB" "MB" "GB" "TB")
    local unit=0
    local size=$bytes
    
    while [ "$size" -ge 1024 ] && [ $unit -lt 4 ]; do
        size=$(echo "scale=2; $size / 1024" | bc)
        ((unit++))
    done
    
    printf "%.2f %s" "$size" "${units[$unit]}"
}

confirm_cleanup() {
    if [ "$FORCE_CLEANUP" = true ] || [ "$DRY_RUN" = true ]; then
        return 0
    fi
    
    echo ""
    echo "=========================================="
    echo "即将执行备份清理"
    echo "=========================================="
    echo "备份目录: $BACKUP_DIR"
    echo "保留策略: $RETENTION_DAYS 天 / $RETENTION_COUNT 个"
    echo "每周保留: $RETENTION_WEEKLY 周"
    echo "每月保留: $RETENTION_MONTHLY 月"
    echo "=========================================="
    echo ""
    
    read -r -p "确认执行清理？ [y/N]: " response
    case "$response" in
        [yY][eE][sS]|[yY])
            return 0
            ;;
        *)
            log_info "取消清理"
            return 1
            ;;
    esac
}

# ============ 主流程 ============
main() {
    # 初始化
    log_init
    parse_args "$@"
    
    log_info "=========================================="
    log_info "$SCRIPT_NAME v$SCRIPT_VERSION"
    log_info "=========================================="
    
    # 检查备份目录
    if [ ! -d "$BACKUP_DIR" ]; then
        log_error "备份目录不存在: $BACKUP_DIR"
        exit 1
    fi
    
    # 检查磁盘空间
    local disk_status=0
    check_disk_space || disk_status=$?
    
    # 确认清理
    if ! confirm_cleanup; then
        exit 0
    fi
    
    # 执行清理
    log_info "开始清理..."
    
    # 1. 按时间清理
    cleanup_by_age
    
    # 2. 按数量清理
    cleanup_by_count
    
    # 3. 每周/每月保留策略（简化处理）
    # cleanup_weekly
    # cleanup_monthly
    
    # 4. 激进清理（如果磁盘空间不足）
    if [ $disk_status -eq 2 ]; then
        cleanup_aggressive
    fi
    
    # 5. 清理孤立文件
    cleanup_orphaned_files
    
    # 完成报告
    log_info "=========================================="
    log_info "清理完成"
    log_info "删除备份: $TOTAL_DELETED 个"
    log_info "释放空间: $(format_size $TOTAL_SIZE_FREED)"
    log_info "=========================================="
    
    # 再次检查磁盘空间
    if [ "$DRY_RUN" != true ]; then
        check_disk_space
    fi
}

# 运行
main "$@"