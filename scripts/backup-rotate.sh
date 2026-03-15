#!/bin/bash
# NAS-OS 备份文件轮转脚本
# 自动管理备份文件的生命周期
#
# v2.58.0 工部创建
#
# 功能:
# - 备份文件自动轮转
# - 灵活的保留策略（按天数/数量）
# - 压缩归档旧备份
# - 支持多种备份类型
# - 清理报告和通知
#
# 用法:
#   ./backup-rotate.sh                  # 执行备份轮转
#   ./backup-rotate.sh --status         # 查看备份状态
#   ./backup-rotate.sh --dry-run        # 预览模式
#   ./backup-rotate.sh --config <file>  # 指定配置文件

set -euo pipefail

#===========================================
# 版本信息
#===========================================
VERSION="2.58.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

#===========================================
# 默认配置
#===========================================

# 配置文件
CONFIG_FILE="${CONFIG_FILE:-/etc/nas-os/backup-rotate.conf}"

# 备份目录
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
ARCHIVE_DIR="${ARCHIVE_DIR:-/var/lib/nas-os/backups/archive}"

# 保留策略
RETENTION_DAYS="${RETENTION_DAYS:-30}"        # 按天数保留
RETENTION_COUNT="${RETENTION_COUNT:-10}"       # 按数量保留（每个类型）
RETENTION_WEEKLY="${RETENTION_WEEKLY:-4}"      # 每周保留数
RETENTION_MONTHLY="${RETENTION_MONTHLY:-6}"    # 每月保留数

# 压缩配置
COMPRESS_AFTER_DAYS="${COMPRESS_AFTER_DAYS:-7}"    # 超过天数后压缩
COMPRESS_CMD="${COMPRESS_CMD:-gzip}"
COMPRESS_EXT="${COMPRESS_EXT:-gz}"
COMPRESS_LEVEL="${COMPRESS_LEVEL:-6}"

# 归档配置
ARCHIVE_AFTER_DAYS="${ARCHIVE_AFTER_DAYS:-30}"     # 超过天数后归档
DELETE_ARCHIVED_AFTER="${DELETE_ARCHIVED_AFTER:-90}" # 归档后删除天数

# 备份类型（支持多种备份类型的独立轮转策略）
BACKUP_TYPES="${BACKUP_TYPES:-full incremental differential database config}"

# 日志配置
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
LOG_FILE="${LOG_FILE:-}"
LOG_MAX_SIZE="${LOG_MAX_SIZE:-10M}"
LOG_MAX_FILES="${LOG_MAX_FILES:-5}"

# 通知配置
ENABLE_NOTIFICATION="${ENABLE_NOTIFICATION:-false}"
NOTIFICATION_WEBHOOK="${NOTIFICATION_WEBHOOK:-}"
NOTIFICATION_EMAIL="${NOTIFICATION_EMAIL:-}"

# 预览模式
DRY_RUN="${DRY_RUN:-false}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

#===========================================
# 统计变量
#===========================================
COMPRESSED_COUNT=0
ARCHIVED_COUNT=0
DELETED_COUNT=0
SAVED_SPACE=0
ERROR_COUNT=0
TOTAL_PROCESSED=0

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
        DRYRUN)  echo -e "${CYAN}[$timestamp] [DRY-RUN]${NC} $msg" ;;
    esac
    
    # 写入日志文件
    if [ -n "$LOG_FILE" ]; then
        echo "[$timestamp] [$level] $msg" >> "$LOG_FILE"
    fi
}

# 解析大小字符串
parse_size() {
    local size_str="$1"
    local num=$(echo "$size_str" | grep -oE '[0-9]+')
    local unit=$(echo "$size_str" | grep -oE '[A-Za-z]+' | tr '[:lower:]' '[:upper:]')
    
    case "$unit" in
        K|KB)  echo $((num * 1024)) ;;
        M|MB)  echo $((num * 1024 * 1024)) ;;
        G|GB)  echo $((num * 1024 * 1024 * 1024)) ;;
        T|TB)  echo $((num * 1024 * 1024 * 1024 * 1024)) ;;
        *)     echo "$num" ;;
    esac
}

# 获取文件大小（字节）
get_file_size() {
    local file="$1"
    if [ -f "$file" ]; then
        stat -c %s "$file" 2>/dev/null || stat -f %z "$file" 2>/dev/null || echo 0
    else
        echo 0
    fi
}

# 获取文件修改时间（时间戳）
get_file_mtime() {
    local file="$1"
    if [ -f "$file" ]; then
        stat -c %Y "$file" 2>/dev/null || stat -f %m "$file" 2>/dev/null || echo 0
    else
        echo 0
    fi
}

# 获取文件天数（距今天）
get_file_age_days() {
    local file="$1"
    local mtime=$(get_file_mtime "$file")
    local now=$(date +%s)
    local age_seconds=$((now - mtime))
    echo $((age_seconds / 86400))
}

# 格式化大小
format_size() {
    local bytes=$1
    if [ $bytes -ge 1073741824 ]; then
        echo "$(echo "scale=2; $bytes / 1073741824" | bc) GB"
    elif [ $bytes -ge 1048576 ]; then
        echo "$(echo "scale=2; $bytes / 1048576" | bc) MB"
    elif [ $bytes -ge 1024 ]; then
        echo "$(echo "scale=2; $bytes / 1024" | bc) KB"
    else
        echo "$bytes B"
    fi
}

#===========================================
# 配置加载
#===========================================

load_config() {
    if [ -f "$CONFIG_FILE" ]; then
        log INFO "加载配置文件: $CONFIG_FILE"
        # shellcheck source=/dev/null
        source "$CONFIG_FILE"
    else
        log INFO "使用默认配置（配置文件不存在: $CONFIG_FILE）"
    fi
    
    # 确保目录存在
    mkdir -p "$BACKUP_DIR" 2>/dev/null || true
    mkdir -p "$ARCHIVE_DIR" 2>/dev/null || true
    mkdir -p "$LOG_DIR" 2>/dev/null || true
    
    # 设置日志文件
    if [ -z "$LOG_FILE" ]; then
        LOG_FILE="$LOG_DIR/backup-rotate-$(date '+%Y%m%d').log"
    fi
}

#===========================================
# 压缩功能
#===========================================

compress_backup() {
    local file="$1"
    
    if [ ! -f "$file" ]; then
        log ERROR "文件不存在: $file"
        return 1
    fi
    
    # 检查是否已压缩
    if [[ "$file" == *".$COMPRESS_EXT" ]]; then
        return 0
    fi
    
    local original_size=$(get_file_size "$file")
    local compressed_file="${file}.${COMPRESS_EXT}"
    
    if [ "$DRY_RUN" = "true" ]; then
        log DRYRUN "将压缩: $file"
        return 0
    fi
    
    log INFO "压缩备份: $file"
    
    if $COMPRESS_CMD -"$COMPRESS_LEVEL" -f "$file"; then
        local compressed_size=$(get_file_size "$compressed_file")
        local saved=$((original_size - compressed_size))
        SAVED_SPACE=$((SAVED_SPACE + saved))
        COMPRESSED_COUNT=$((COMPRESSED_COUNT + 1))
        log SUCCESS "压缩完成: $(format_size $original_size) -> $(format_size $compressed_size) (节省 $(format_size $saved))"
        return 0
    else
        log ERROR "压缩失败: $file"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        return 1
    fi
}

#===========================================
# 归档功能
#===========================================

archive_backup() {
    local file="$1"
    local backup_type="$2"
    
    if [ ! -f "$file" ]; then
        log ERROR "文件不存在: $file"
        return 1
    fi
    
    if [ "$DRY_RUN" = "true" ]; then
        log DRYRUN "将归档: $file"
        return 0
    fi
    
    # 创建类型子目录
    local type_archive_dir="$ARCHIVE_DIR/$backup_type"
    mkdir -p "$type_archive_dir"
    
    local filename=$(basename "$file")
    local archive_path="$type_archive_dir/$filename"
    
    log INFO "归档备份: $file -> $archive_path"
    
    if mv "$file" "$archive_path"; then
        ARCHIVED_COUNT=$((ARCHIVED_COUNT + 1))
        log SUCCESS "归档完成: $filename"
        return 0
    else
        log ERROR "归档失败: $file"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        return 1
    fi
}

#===========================================
# 删除功能
#===========================================

delete_backup() {
    local file="$1"
    
    if [ ! -f "$file" ]; then
        return 0
    fi
    
    local file_size=$(get_file_size "$file")
    
    if [ "$DRY_RUN" = "true" ]; then
        log DRYRUN "将删除: $file ($(format_size $file_size))"
        SAVED_SPACE=$((SAVED_SPACE + file_size))
        DELETED_COUNT=$((DELETED_COUNT + 1))
        return 0
    fi
    
    log INFO "删除备份: $file ($(format_size $file_size))"
    
    if rm -f "$file"; then
        SAVED_SPACE=$((SAVED_SPACE + file_size))
        DELETED_COUNT=$((DELETED_COUNT + 1))
        log SUCCESS "已删除: $file"
        return 0
    else
        log ERROR "删除失败: $file"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        return 1
    fi
}

#===========================================
# 轮转策略
#===========================================

rotate_by_age() {
    local backup_type="$1"
    local type_dir="$BACKUP_DIR/$backup_type"
    
    if [ ! -d "$type_dir" ]; then
        log INFO "目录不存在，跳过: $type_dir"
        return 0
    fi
    
    log INFO "按天数轮转: $backup_type (保留 $RETENTION_DAYS 天)"
    
    local files=()
    while IFS= read -r -d '' file; do
        files+=("$file")
    done < <(find "$type_dir" -type f -name "*.tar*" -o -name "*.zip" -o -name "*.sql*" -o -name "*.dump" 2>/dev/null | sort -z)
    
    for file in "${files[@]}"; do
        local age=$(get_file_age_days "$file")
        
        # 压缩旧备份
        if [ $age -ge $COMPRESS_AFTER_DAYS ]; then
            compress_backup "$file"
        fi
        
        # 归档更老的备份
        if [ $age -ge $ARCHIVE_AFTER_DAYS ]; then
            archive_backup "$file" "$backup_type"
        fi
        
        TOTAL_PROCESSED=$((TOTAL_PROCESSED + 1))
    done
}

rotate_by_count() {
    local backup_type="$1"
    local type_dir="$BACKUP_DIR/$backup_type"
    
    if [ ! -d "$type_dir" ]; then
        return 0
    fi
    
    log INFO "按数量轮转: $backup_type (保留 $RETENTION_COUNT 个)"
    
    # 按修改时间排序，保留最新的 N 个
    local count=0
    while IFS= read -r file; do
        count=$((count + 1))
        if [ $count -gt $RETENTION_COUNT ]; then
            delete_backup "$file"
        fi
        TOTAL_PROCESSED=$((TOTAL_PROCESSED + 1))
    done < <(find "$type_dir" -type f \( -name "*.tar*" -o -name "*.zip" -o -name "*.sql*" -o -name "*.dump" \) -printf '%T@ %p\n' 2>/dev/null | sort -n | cut -d' ' -f2-)
}

rotate_weekly() {
    local backup_type="$1"
    local type_dir="$BACKUP_DIR/$backup_type"
    
    if [ ! -d "$type_dir" ]; then
        return 0
    fi
    
    log INFO "每周备份轮转: $backup_type (保留 $RETENTION_WEEKLY 周)"
    
    # 找出每周的最后一个备份
    local week_files=()
    local current_week=""
    
    while IFS= read -r file; do
        local mtime=$(get_file_mtime "$file")
        local week_num=$(date -d "@$mtime" +%Y-W%W 2>/dev/null || date -j -f "%s" "$mtime" +%Y-W%W 2>/dev/null)
        
        if [ "$week_num" != "$current_week" ]; then
            week_files+=("$file")
            current_week="$week_num"
        fi
    done < <(find "$type_dir" -type f \( -name "*.tar*" -o -name "*.zip" -o -name "*.sql*" -o -name "*.dump" \) -printf '%T@ %p\n' 2>/dev/null | sort -n | cut -d' ' -f2-)
    
    # 保留最近的 N 周
    local count=0
    local to_delete=()
    
    for file in "${week_files[@]}"; do
        count=$((count + 1))
        if [ $count -gt $RETENTION_WEEKLY ]; then
            to_delete+=("$file")
        fi
    done
    
    for file in "${to_delete[@]}"; do
        delete_backup "$file"
    done
}

rotate_monthly() {
    local backup_type="$1"
    local type_dir="$BACKUP_DIR/$backup_type"
    
    if [ ! -d "$type_dir" ]; then
        return 0
    fi
    
    log INFO "每月备份轮转: $backup_type (保留 $RETENTION_MONTHLY 月)"
    
    # 找出每月的最后一个备份
    local month_files=()
    local current_month=""
    
    while IFS= read -r file; do
        local mtime=$(get_file_mtime "$file")
        local month_num=$(date -d "@$mtime" +%Y-%m 2>/dev/null || date -j -f "%s" "$mtime" +%Y-%m 2>/dev/null)
        
        if [ "$month_num" != "$current_month" ]; then
            month_files+=("$file")
            current_month="$month_num"
        fi
    done < <(find "$type_dir" -type f \( -name "*.tar*" -o -name "*.zip" -o -name "*.sql*" -o -name "*.dump" \) -printf '%T@ %p\n' 2>/dev/null | sort -n | cut -d' ' -f2-)
    
    # 保留最近的 N 月
    local count=0
    local to_delete=()
    
    for file in "${month_files[@]}"; do
        count=$((count + 1))
        if [ $count -gt $RETENTION_MONTHLY ]; then
            to_delete+=("$file")
        fi
    done
    
    for file in "${to_delete[@]}"; do
        delete_backup "$file"
    done
}

#===========================================
# 清理归档目录
#===========================================

cleanup_archives() {
    log INFO "清理过期归档（超过 $DELETE_ARCHIVED_AFTER 天）"
    
    for type_dir in "$ARCHIVE_DIR"/*; do
        if [ ! -d "$type_dir" ]; then
            continue
        fi
        
        while IFS= read -r file; do
            local age=$(get_file_age_days "$file")
            if [ $age -ge $DELETE_ARCHIVED_AFTER ]; then
                delete_backup "$file"
            fi
        done < <(find "$type_dir" -type f 2>/dev/null)
    done
}

#===========================================
# 状态报告
#===========================================

show_status() {
    echo ""
    echo "========================================"
    echo "  备份轮转状态"
    echo "========================================"
    echo ""
    
    # 备份目录状态
    echo "备份目录: $BACKUP_DIR"
    echo "归档目录: $ARCHIVE_DIR"
    echo ""
    
    echo "--- 备份类型统计 ---"
    local total_size=0
    local total_count=0
    
    for backup_type in $BACKUP_TYPES; do
        local type_dir="$BACKUP_DIR/$backup_type"
        if [ -d "$type_dir" ]; then
            local count=$(find "$type_dir" -type f | wc -l)
            local size=$(du -sb "$type_dir" 2>/dev/null | cut -f1)
            total_size=$((total_size + size))
            total_count=$((total_count + count))
            echo "  $backup_type: $count 个备份, $(format_size $size)"
        fi
    done
    
    echo ""
    echo "总计: $total_count 个备份, $(format_size $total_size)"
    echo ""
    
    # 归档目录状态
    if [ -d "$ARCHIVE_DIR" ]; then
        local archive_count=$(find "$ARCHIVE_DIR" -type f | wc -l)
        local archive_size=$(du -sb "$ARCHIVE_DIR" 2>/dev/null | cut -f1)
        echo "归档: $archive_count 个文件, $(format_size $archive_size)"
    fi
    
    echo ""
    echo "--- 保留策略 ---"
    echo "  按天数保留: $RETENTION_DAYS 天"
    echo "  按数量保留: $RETENTION_COUNT 个/类型"
    echo "  每周保留: $RETENTION_WEEKLY 周"
    echo "  每月保留: $RETENTION_MONTHLY 月"
    echo "  压缩阈值: $COMPRESS_AFTER_DAYS 天后"
    echo "  归档阈值: $ARCHIVE_AFTER_DAYS 天后"
    echo "  删除归档: $DELETE_ARCHIVED_AFTER 天后"
    echo ""
}

#===========================================
# 通知功能
#===========================================

send_notification() {
    if [ "$ENABLE_NOTIFICATION" != "true" ]; then
        return 0
    fi
    
    local subject="NAS-OS 备份轮转报告 - $(date '+%Y-%m-%d %H:%M')"
    local body="备份轮转完成
    
处理统计:
- 处理文件: $TOTAL_PROCESSED
- 压缩文件: $COMPRESSED_COUNT
- 归档文件: $ARCHIVED_COUNT
- 删除文件: $DELETED_COUNT
- 节省空间: $(format_size $SAVED_SPACE)
- 错误数量: $ERROR_COUNT

时间: $(date '+%Y-%m-%d %H:%M:%S')
"
    
    # Webhook 通知
    if [ -n "$NOTIFICATION_WEBHOOK" ]; then
        curl -s -X POST -H "Content-Type: application/json" \
            -d "{\"text\":\"$subject\n$body\"}" \
            "$NOTIFICATION_WEBHOOK" >/dev/null 2>&1
    fi
    
    # 邮件通知
    if [ -n "$NOTIFICATION_EMAIL" ]; then
        echo "$body" | mail -s "$subject" "$NOTIFICATION_EMAIL" 2>/dev/null || true
    fi
}

#===========================================
# 生成配置文件
#===========================================

generate_config() {
    local config_path="${1:-$CONFIG_FILE}"
    
    cat > "$config_path" << 'EOF'
# NAS-OS 备份轮转配置文件
# 复制到 /etc/nas-os/backup-rotate.conf

# 备份目录
BACKUP_DIR="/var/lib/nas-os/backups"
ARCHIVE_DIR="/var/lib/nas-os/backups/archive"

# 保留策略
RETENTION_DAYS=30           # 按天数保留
RETENTION_COUNT=10          # 按数量保留（每个类型）
RETENTION_WEEKLY=4          # 每周保留数
RETENTION_MONTHLY=6         # 每月保留数

# 压缩配置
COMPRESS_AFTER_DAYS=7       # 超过天数后压缩
COMPRESS_CMD="gzip"
COMPRESS_EXT="gz"
COMPRESS_LEVEL=6

# 归档配置
ARCHIVE_AFTER_DAYS=30       # 超过天数后归档
DELETE_ARCHIVED_AFTER=90    # 归档后删除天数

# 备份类型（空格分隔）
BACKUP_TYPES="full incremental differential database config"

# 通知配置
ENABLE_NOTIFICATION="false"
NOTIFICATION_WEBHOOK=""
NOTIFICATION_EMAIL=""
EOF
    
    log SUCCESS "配置文件已生成: $config_path"
}

#===========================================
# 帮助信息
#===========================================

show_help() {
    cat << EOF
NAS-OS 备份轮转脚本 v${VERSION}

用法:
  $SCRIPT_NAME [选项]

选项:
  --status              显示备份状态
  --dry-run             预览模式（不实际执行）
  --config <file>       指定配置文件
  --generate-config     生成默认配置文件
  --help                显示帮助信息
  --version             显示版本信息

配置文件: $CONFIG_FILE

保留策略:
  - 按天数: 保留 N 天内的备份
  - 按数量: 每种类型保留最新的 N 个
  - 每周: 保留每周最后一个备份，共 N 周
  - 每月: 保留每月最后一个备份，共 N 月

轮转流程:
  1. 压缩超过 COMPRESS_AFTER_DAYS 天的备份
  2. 归档超过 ARCHIVE_AFTER_DAYS 天的备份
  3. 删除超过保留策略的备份
  4. 清理过期归档

示例:
  $SCRIPT_NAME                    # 执行备份轮转
  $SCRIPT_NAME --status           # 查看备份状态
  $SCRIPT_NAME --dry-run          # 预览将执行的操作
  $SCRIPT_NAME --generate-config  # 生成配置文件

EOF
}

#===========================================
# 主函数
#===========================================

main() {
    local action="rotate"
    
    # 解析参数
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
            --help|-h)
                show_help
                exit 0
                ;;
            --version|-v)
                echo "$SCRIPT_NAME v${VERSION}"
                exit 0
                ;;
            *)
                log ERROR "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 加载配置
    load_config
    
    # 执行操作
    case "$action" in
        status)
            show_status
            ;;
        rotate)
            log INFO "开始备份轮转..."
            
            if [ "$DRY_RUN" = "true" ]; then
                log INFO "预览模式 - 不会实际执行操作"
            fi
            
            # 按备份类型轮转
            for backup_type in $BACKUP_TYPES; do
                log INFO "处理备份类型: $backup_type"
                rotate_by_age "$backup_type"
                rotate_by_count "$backup_type"
                rotate_weekly "$backup_type"
                rotate_monthly "$backup_type"
            done
            
            # 清理归档
            cleanup_archives
            
            # 发送通知
            send_notification
            
            # 打印统计
            echo ""
            echo "========================================"
            echo "  轮转完成"
            echo "========================================"
            echo "处理文件: $TOTAL_PROCESSED"
            echo "压缩文件: $COMPRESSED_COUNT"
            echo "归档文件: $ARCHIVED_COUNT"
            echo "删除文件: $DELETED_COUNT"
            echo "节省空间: $(format_size $SAVED_SPACE)"
            echo "错误数量: $ERROR_COUNT"
            echo "========================================"
            ;;
    esac
}

# 运行主函数
main "$@"