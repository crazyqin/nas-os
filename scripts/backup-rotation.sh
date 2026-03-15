#!/bin/bash
# =============================================================================
# NAS-OS 备份轮转脚本 v2.74.0
# =============================================================================
# 用途：自动管理备份文件的生命周期，支持多种轮转策略
# 用法：./backup-rotation.sh [--status] [--dry-run] [--config FILE]
# =============================================================================

set -euo pipefail

VERSION="2.74.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 默认配置
# =============================================================================

# 配置文件
CONFIG_FILE="${CONFIG_FILE:-/etc/nas-os/backup-rotation.conf}"

# 备份目录
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
ARCHIVE_DIR="${ARCHIVE_DIR:-/var/lib/nas-os/backups/archive}"

# 保留策略
DAILY_RETENTION="${DAILY_RETENTION:-7}"        # 每日备份保留天数
WEEKLY_RETENTION="${WEEKLY_RETENTION:-4}"      # 每周备份保留数
MONTHLY_RETENTION="${MONTHLY_RETENTION:-6}"    # 每月备份保留数
YEARLY_RETENTION="${YEARLY_RETENTION:-2}"      # 每年备份保留数

# 压缩配置
COMPRESS_DAYS="${COMPRESS_DAYS:-3}"            # 超过天数后压缩
COMPRESS_CMD="${COMPRESS_CMD:-gzip}"
COMPRESS_LEVEL="${COMPRESS_LEVEL:-6}"

# 归档配置
ARCHIVE_DAYS="${ARCHIVE_DAYS:-30}"             # 超过天数后归档
DELETE_AFTER_DAYS="${DELETE_AFTER_DAYS:-90}"   # 超过天数后删除

# 备份类型
BACKUP_TYPES="${BACKUP_TYPES:-full incremental differential database config logs}"

# 日志
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
LOG_FILE="${LOG_FILE:-}"
LOG_ROTATION="${LOG_ROTATION:-7}"              # 日志保留天数

# 通知
ENABLE_NOTIFY="${ENABLE_NOTIFY:-false}"
NOTIFY_WEBHOOK="${NOTIFY_WEBHOOK:-}"
NOTIFY_EMAIL="${NOTIFY_EMAIL:-}"

# 模式
DRY_RUN="${DRY_RUN:-false}"
VERBOSE="${VERBOSE:-false}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# =============================================================================
# 统计
# =============================================================================

STATS_COMPRESSED=0
STATS_ARCHIVED=0
STATS_DELETED=0
STATS_SAVED_BYTES=0
STATS_ERRORS=0
STATS_TOTAL=0

# =============================================================================
# 工具函数
# =============================================================================

timestamp() {
    date '+%Y-%m-%d %H:%M:%S'
}

log() {
    local level="$1"
    shift
    local msg="$*"
    local ts=$(timestamp)
    
    case "$level" in
        ERROR)
            echo -e "${RED}[$ts] [ERROR]${NC} $msg" >&2
            ;;
        WARN)
            echo -e "${YELLOW}[$ts] [WARN]${NC} $msg"
            ;;
        INFO)
            [ "$VERBOSE" = "true" ] && echo -e "${BLUE}[$ts] [INFO]${NC} $msg"
            ;;
        OK)
            echo -e "${GREEN}[$ts] [OK]${NC} $msg"
            ;;
        DRY)
            echo -e "${CYAN}[$ts] [DRY-RUN]${NC} $msg"
            ;;
    esac
    
    # 写日志文件
    if [ -n "$LOG_FILE" ]; then
        echo "[$ts] [$level] $msg" >> "$LOG_FILE"
    fi
}

get_file_size() {
    local file="$1"
    stat -c %s "$file" 2>/dev/null || stat -f %z "$file" 2>/dev/null || echo 0
}

get_file_mtime() {
    local file="$1"
    stat -c %Y "$file" 2>/dev/null || stat -f %m "$file" 2>/dev/null || echo 0
}

get_file_age_days() {
    local file="$1"
    local mtime=$(get_file_mtime "$file")
    local now=$(date +%s)
    echo $(( (now - mtime) / 86400 ))
}

format_size() {
    local bytes=$1
    if [ $bytes -ge 1073741824 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes / 1073741824}") GB"
    elif [ $bytes -ge 1048576 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes / 1048576}") MB"
    elif [ $bytes -ge 1024 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes / 1024}") KB"
    else
        echo "$bytes B"
    fi
}

# =============================================================================
# 配置加载
# =============================================================================

load_config() {
    if [ -f "$CONFIG_FILE" ]; then
        log INFO "加载配置: $CONFIG_FILE"
        source "$CONFIG_FILE"
    fi
    
    # 确保目录存在
    mkdir -p "$BACKUP_DIR" 2>/dev/null || true
    mkdir -p "$ARCHIVE_DIR" 2>/dev/null || true
    mkdir -p "$LOG_DIR" 2>/dev/null || true
    
    # 设置日志文件
    [ -z "$LOG_FILE" ] && LOG_FILE="$LOG_DIR/backup-rotation-$(date +%Y%m%d).log"
}

# =============================================================================
# 压缩功能
# =============================================================================

compress_file() {
    local file="$1"
    
    # 跳过已压缩文件
    [[ "$file" == *.gz || "$file" == *.bz2 || "$file" == *.xz ]] && return 0
    
    [ ! -f "$file" ] && return 1
    
    local original_size=$(get_file_size "$file")
    
    if [ "$DRY_RUN" = "true" ]; then
        log DRY "将压缩: $file ($(format_size $original_size))"
        STATS_COMPRESSED=$((STATS_COMPRESSED + 1))
        return 0
    fi
    
    log INFO "压缩: $file"
    
    if $COMPRESS_CMD -$COMPRESS_LEVEL -f "$file" 2>/dev/null; then
        local compressed_file="${file}.gz"
        local compressed_size=$(get_file_size "$compressed_file")
        local saved=$((original_size - compressed_size))
        
        STATS_COMPRESSED=$((STATS_COMPRESSED + 1))
        STATS_SAVED_BYTES=$((STATS_SAVED_BYTES + saved))
        
        log OK "压缩完成: $(format_size $original_size) -> $(format_size $compressed_size) (节省 $(format_size $saved))"
        return 0
    else
        log ERROR "压缩失败: $file"
        STATS_ERRORS=$((STATS_ERRORS + 1))
        return 1
    fi
}

# =============================================================================
# 归档功能
# =============================================================================

archive_file() {
    local file="$1"
    local backup_type="$2"
    
    [ ! -f "$file" ] && return 1
    
    if [ "$DRY_RUN" = "true" ]; then
        log DRY "将归档: $file"
        STATS_ARCHIVED=$((STATS_ARCHIVED + 1))
        return 0
    fi
    
    local type_archive_dir="$ARCHIVE_DIR/$backup_type"
    mkdir -p "$type_archive_dir"
    
    local filename=$(basename "$file")
    local archive_path="$type_archive_dir/$filename"
    
    # 避免覆盖
    if [ -f "$archive_path" ]; then
        archive_path="${archive_path}.$(date +%s)"
    fi
    
    log INFO "归档: $file -> $archive_path"
    
    if mv "$file" "$archive_path"; then
        STATS_ARCHIVED=$((STATS_ARCHIVED + 1))
        log OK "归档完成: $filename"
        return 0
    else
        log ERROR "归档失败: $file"
        STATS_ERRORS=$((STATS_ERRORS + 1))
        return 1
    fi
}

# =============================================================================
# 删除功能
# =============================================================================

delete_file() {
    local file="$1"
    
    [ ! -f "$file" ] && return 0
    
    local size=$(get_file_size "$file")
    
    if [ "$DRY_RUN" = "true" ]; then
        log DRY "将删除: $file ($(format_size $size))"
        STATS_DELETED=$((STATS_DELETED + 1))
        STATS_SAVED_BYTES=$((STATS_SAVED_BYTES + size))
        return 0
    fi
    
    log INFO "删除: $file ($(format_size $size))"
    
    if rm -f "$file"; then
        STATS_DELETED=$((STATS_DELETED + 1))
        STATS_SAVED_BYTES=$((STATS_SAVED_BYTES + size))
        log OK "已删除: $(basename "$file")"
        return 0
    else
        log ERROR "删除失败: $file"
        STATS_ERRORS=$((STATS_ERRORS + 1))
        return 1
    fi
}

# =============================================================================
# 轮转策略
# =============================================================================

# 按日期轮转（每日备份）
rotate_daily() {
    local backup_type="$1"
    local type_dir="$BACKUP_DIR/$backup_type"
    
    [ ! -d "$type_dir" ] && return 0
    
    log INFO "轮转每日备份: $backup_type (保留 $DAILY_RETENTION 天)"
    
    local count=0
    while IFS= read -r file; do
        [ -z "$file" ] && continue
        
        local age=$(get_file_age_days "$file")
        STATS_TOTAL=$((STATS_TOTAL + 1))
        
        # 压缩
        if [ $age -ge $COMPRESS_DAYS ]; then
            compress_file "$file"
            file="${file}.gz"  # 更新文件路径
        fi
        
        # 归档
        if [ $age -ge $ARCHIVE_DAYS ]; then
            archive_file "$file" "$backup_type"
        fi
        
        count=$((count + 1))
    done < <(find "$type_dir" -type f \( -name "*.tar*" -o -name "*.zip" -o -name "*.sql*" -o -name "*.dump" -o -name "*.gz" \) 2>/dev/null | sort)
    
    log INFO "处理了 $count 个文件"
}

# 按周轮转（保留每周最后一个备份）
rotate_weekly() {
    local backup_type="$1"
    local type_dir="$BACKUP_DIR/$backup_type"
    
    [ ! -d "$type_dir" ] && return 0
    
    log INFO "轮转每周备份: $backup_type (保留 $WEEKLY_RETENTION 周)"
    
    local -A week_files
    local current_week=""
    local week_count=0
    
    # 按修改时间排序，找出每周最后一个备份
    while IFS= read -r file; do
        [ -z "$file" ] && continue
        
        local mtime=$(get_file_mtime "$file")
        local week=$(date -d "@$mtime" +%Y-W%W 2>/dev/null || date -j -f "%s" "$mtime" +%Y-W%W 2>/dev/null)
        
        if [ "$week" != "$current_week" ]; then
            current_week="$week"
            week_count=$((week_count + 1))
            
            # 超过保留数则删除
            if [ $week_count -gt $WEEKLY_RETENTION ]; then
                delete_file "$file"
            fi
        fi
    done < <(find "$type_dir" -type f \( -name "*.tar*" -o -name "*.zip" -o -name "*.sql*" -o -name "*.dump" \) -printf '%T@ %p\n' 2>/dev/null | sort -n | cut -d' ' -f2-)
}

# 按月轮转
rotate_monthly() {
    local backup_type="$1"
    local type_dir="$BACKUP_DIR/$backup_type"
    
    [ ! -d "$type_dir" ] && return 0
    
    log INFO "轮转每月备份: $backup_type (保留 $MONTHLY_RETENTION 月)"
    
    local -A month_files
    local current_month=""
    local month_count=0
    
    while IFS= read -r file; do
        [ -z "$file" ] && continue
        
        local mtime=$(get_file_mtime "$file")
        local month=$(date -d "@$mtime" +%Y-%m 2>/dev/null || date -j -f "%s" "$mtime" +%Y-%m 2>/dev/null)
        
        if [ "$month" != "$current_month" ]; then
            current_month="$month"
            month_count=$((month_count + 1))
            
            if [ $month_count -gt $MONTHLY_RETENTION ]; then
                delete_file "$file"
            fi
        fi
    done < <(find "$type_dir" -type f \( -name "*.tar*" -o -name "*.zip" -o -name "*.sql*" -o -name "*.dump" \) -printf '%T@ %p\n' 2>/dev/null | sort -n | cut -d' ' -f2-)
}

# 清理过期归档
cleanup_archives() {
    log INFO "清理过期归档（超过 $DELETE_AFTER_DAYS 天）"
    
    for type_dir in "$ARCHIVE_DIR"/*; do
        [ ! -d "$type_dir" ] && continue
        
        while IFS= read -r file; do
            [ -z "$file" ] && continue
            
            local age=$(get_file_age_days "$file")
            if [ $age -ge $DELETE_AFTER_DAYS ]; then
                delete_file "$file"
            fi
        done < <(find "$type_dir" -type f 2>/dev/null)
    done
}

# =============================================================================
# 状态报告
# =============================================================================

show_status() {
    echo ""
    echo "========================================"
    echo "  备份轮转状态 v$VERSION"
    echo "========================================"
    echo ""
    
    echo "备份目录: $BACKUP_DIR"
    echo "归档目录: $ARCHIVE_DIR"
    echo ""
    
    echo "--- 备份统计 ---"
    
    local total_count=0
    local total_size=0
    
    for backup_type in $BACKUP_TYPES; do
        local type_dir="$BACKUP_DIR/$backup_type"
        if [ -d "$type_dir" ]; then
            local count=$(find "$type_dir" -type f 2>/dev/null | wc -l)
            local size=$(du -sb "$type_dir" 2>/dev/null | cut -f1 || echo 0)
            total_count=$((total_count + count))
            total_size=$((total_size + size))
            
            echo "  $backup_type: $count 个文件, $(format_size $size)"
        fi
    done
    
    echo ""
    echo "总计: $total_count 个备份, $(format_size $total_size)"
    
    # 归档统计
    if [ -d "$ARCHIVE_DIR" ]; then
        echo ""
        echo "--- 归档统计 ---"
        
        local archive_count=0
        local archive_size=0
        
        for type_dir in "$ARCHIVE_DIR"/*; do
            [ ! -d "$type_dir" ] && continue
            
            local type_name=$(basename "$type_dir")
            local count=$(find "$type_dir" -type f 2>/dev/null | wc -l)
            local size=$(du -sb "$type_dir" 2>/dev/null | cut -f1 || echo 0)
            archive_count=$((archive_count + count))
            archive_size=$((archive_size + size))
            
            echo "  $type_name: $count 个归档, $(format_size $size)"
        done
        
        echo "  归档总计: $archive_count 个文件, $(format_size $archive_size)"
    fi
    
    echo ""
    echo "--- 保留策略 ---"
    echo "  每日备份: $DAILY_RETENTION 天"
    echo "  每周备份: $WEEKLY_RETENTION 周"
    echo "  每月备份: $MONTHLY_RETENTION 月"
    echo "  压缩阈值: $COMPRESS_DAYS 天"
    echo "  归档阈值: $ARCHIVE_DAYS 天"
    echo "  删除阈值: $DELETE_AFTER_DAYS 天"
    echo ""
}

# =============================================================================
# 通知
# =============================================================================

send_notification() {
    [ "$ENABLE_NOTIFY" != "true" ] && return 0
    
    local subject="[NAS-OS] 备份轮转报告 - $(date '+%Y-%m-%d')"
    local body="备份轮转完成

统计:
- 处理文件: $STATS_TOTAL
- 压缩文件: $STATS_COMPRESSED
- 归档文件: $STATS_ARCHIVED
- 删除文件: $STATS_DELETED
- 节省空间: $(format_size $STATS_SAVED_BYTES)
- 错误数量: $STATS_ERRORS

时间: $(timestamp)
"
    
    [ -n "$NOTIFY_WEBHOOK" ] && curl -s -X POST -H "Content-Type: application/json" -d "{\"text\":\"$body\"}" "$NOTIFY_WEBHOOK" >/dev/null 2>&1 || true
    [ -n "$NOTIFY_EMAIL" ] && echo "$body" | mail -s "$subject" "$NOTIFY_EMAIL" 2>/dev/null || true
}

# =============================================================================
# 配置生成
# =============================================================================

generate_config() {
    local path="${1:-$CONFIG_FILE}"
    
    cat > "$path" << 'EOF'
# NAS-OS 备份轮转配置文件

# 备份目录
BACKUP_DIR="/var/lib/nas-os/backups"
ARCHIVE_DIR="/var/lib/nas-os/backups/archive"

# 保留策略
DAILY_RETENTION=7           # 每日备份保留天数
WEEKLY_RETENTION=4          # 每周备份保留数
MONTHLY_RETENTION=6         # 每月备份保留数
YEARLY_RETENTION=2          # 每年备份保留数

# 压缩配置
COMPRESS_DAYS=3             # 超过天数后压缩
COMPRESS_CMD="gzip"
COMPRESS_LEVEL=6

# 归档配置
ARCHIVE_DAYS=30             # 超过天数后归档
DELETE_AFTER_DAYS=90        # 超过天数后删除

# 备份类型
BACKUP_TYPES="full incremental differential database config logs"

# 通知配置
ENABLE_NOTIFY="false"
NOTIFY_WEBHOOK=""
NOTIFY_EMAIL=""
EOF
    
    log OK "配置文件已生成: $path"
}

# =============================================================================
# 帮助
# =============================================================================

show_help() {
    cat <<EOF
NAS-OS 备份轮转脚本 v$VERSION

用法: $SCRIPT_NAME [选项]

选项:
  --status            显示备份状态
  --dry-run           预览模式
  --config FILE       指定配置文件
  --generate-config   生成配置文件
  --verbose           详细输出
  --help              显示帮助

保留策略:
  - 每日: 保留 N 天内的所有备份
  - 每周: 保留每周最后一个备份，共 N 周
  - 每月: 保留每月最后一个备份，共 N 月

轮转流程:
  1. 压缩超过 COMPRESS_DAYS 天的备份
  2. 归档超过 ARCHIVE_DAYS 天的备份
  3. 按保留策略删除过期备份
  4. 清理超过 DELETE_AFTER_DAYS 天的归档

示例:
  $SCRIPT_NAME                   # 执行轮转
  $SCRIPT_NAME --status          # 查看状态
  $SCRIPT_NAME --dry-run         # 预览操作
  $SCRIPT_NAME --generate-config # 生成配置

EOF
}

# =============================================================================
# 主函数
# =============================================================================

main() {
    local action="rotate"
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --status)
                action="status"
                shift
                ;;
            --dry-run)
                DRY_RUN="true"
                shift
                ;;
            --config)
                CONFIG_FILE="$2"
                shift 2
                ;;
            --generate-config)
                generate_config "${2:-$CONFIG_FILE}"
                exit 0
                ;;
            --verbose|-v)
                VERBOSE="true"
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
    
    load_config
    
    case "$action" in
        status)
            show_status
            ;;
        rotate)
            echo ""
            echo "========================================"
            echo "  NAS-OS 备份轮转 v$VERSION"
            echo "  时间: $(timestamp)"
            echo "========================================"
            
            [ "$DRY_RUN" = "true" ] && log INFO "预览模式 - 不会实际执行"
            
            # 按类型轮转
            for backup_type in $BACKUP_TYPES; do
                log INFO "处理类型: $backup_type"
                rotate_daily "$backup_type"
                rotate_weekly "$backup_type"
                rotate_monthly "$backup_type"
            done
            
            # 清理归档
            cleanup_archives
            
            # 发送通知
            send_notification
            
            # 统计报告
            echo ""
            echo "========================================"
            echo "  轮转完成"
            echo "========================================"
            echo "  处理文件: $STATS_TOTAL"
            echo "  压缩文件: $STATS_COMPRESSED"
            echo "  归档文件: $STATS_ARCHIVED"
            echo "  删除文件: $STATS_DELETED"
            echo "  节省空间: $(format_size $STATS_SAVED_BYTES)"
            echo "  错误数量: $STATS_ERRORS"
            echo "========================================"
            ;;
    esac
}

main "$@"