#!/bin/bash
# NAS-OS 系统恢复脚本
# 从备份恢复数据，支持选择性恢复和验证
#
# v2.50.0 工部开发
# - 从备份恢复
# - 选择性恢复
# - 恢复前验证
# - 恢复进度显示
# - 恢复失败回滚

set -e

# 脚本信息
SCRIPT_VERSION="2.50.0"
SCRIPT_NAME="NAS-OS Restore"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 默认配置
BACKUP_NAME="${BACKUP_NAME:-nas-os}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
CONFIG_DIR="${CONFIG_DIR:-/etc/nas-os}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 恢复选项
RESTORE_ID="${RESTORE_ID:-}"
RESTORE_FILE="${RESTORE_FILE:-}"
RESTORE_ITEMS=()
RESTORE_EXCLUDE=()

# 恢复行为
VERIFY_BEFORE_RESTORE="${VERIFY_BEFORE_RESTORE:-true}"
CREATE_ROLLBACK="${CREATE_ROLLBACK:-true}"
FORCE_RESTORE="${FORCE_RESTORE:-false}"
DRY_RUN="${DRY_RUN:-false}"

# 回滚配置
ROLLBACK_DIR="${ROLLBACK_DIR:-/var/lib/nas-os/rollback}"
ROLLBACK_MAX_AGE="${ROLLBACK_MAX_AGE:-7}"  # 天

# 解密配置
ENABLE_ENCRYPTION="${ENABLE_ENCRYPTION:-false}"
ENCRYPTION_KEY_FILE="${ENCRYPTION_KEY_FILE:-/etc/nas-os/backup.key}"

# 通知配置
ENABLE_NOTIFICATION="${ENABLE_NOTIFICATION:-false}"
NOTIFICATION_WEBHOOK="${NOTIFICATION_WEBHOOK:-}"
NOTIFICATION_EMAIL="${NOTIFICATION_EMAIL:-}"

# 日志配置
LOG_FILE="${LOG_FILE:-}"

# 临时文件
TEMP_DIR=""
RESTORE_START_TIME=""
RESTORE_END_TIME=""
RESTORE_STATUS=""

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ============ 日志函数 ============
log_init() {
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    if [ -z "$LOG_FILE" ]; then
        LOG_FILE="$LOG_DIR/restore-$(date '+%Y%m%d').log"
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
        PROGRESS) echo -e "${CYAN}[PROGRESS]${NC} $message" ;;
        DEBUG)   [ "${DEBUG:-false}" = true ] && echo -e "${CYAN}[DEBUG]${NC} $message" ;;
    esac
}

log_info()    { log "INFO" "$1"; }
log_success()  { log "SUCCESS" "$1"; }
log_warn()     { log "WARN" "$1"; }
log_error()    { log "ERROR" "$1"; }
log_progress() { log "PROGRESS" "$1"; }
log_debug()    { log "DEBUG" "$1"; }

# ============ 进度显示 ============
show_progress() {
    local current="$1"
    local total="$2"
    local message="$3"
    
    if [ "$total" -gt 0 ]; then
        local percent=$((current * 100 / total))
        local bar_width=40
        local filled=$((percent * bar_width / 100))
        local empty=$((bar_width - filled))
        
        printf "\r${CYAN}[PROGRESS]${NC} %s [%s%s] %d%% (%d/%d)" \
            "$message" \
            "$(printf '#%.0s' $(seq 1 $filled) 2>/dev/null)" \
            "$(printf ' %.0s' $(seq 1 $empty) 2>/dev/null)" \
            "$percent" "$current" "$total"
        
        if [ "$current" -eq "$total" ]; then
            echo ""
        fi
    else
        echo -e "${CYAN}[PROGRESS]${NC} $message"
    fi
}

# ============ 帮助信息 ============
show_help() {
    cat << EOF
$SCRIPT_NAME v$SCRIPT_VERSION - NAS-OS 系统恢复脚本

用法: $(basename "$0") [选项] [备份ID|文件]

恢复选项:
  -i, --id ID             指定恢复的备份 ID
  -f, --file FILE         指定恢复的备份文件路径
  -d, --dir DIR           备份存储目录 (默认: /var/lib/nas-os/backups)
  
选择性恢复:
  --include PATTERN       只恢复匹配的文件/目录 (可多次使用)
  --exclude PATTERN       排除匹配的文件/目录 (可多次使用)
  
恢复行为:
  --verify                恢复前验证备份完整性 (默认启用)
  --no-verify             跳过验证
  --rollback              恢复前创建回滚点 (默认启用)
  --no-rollback           不创建回滚点
  --force                 强制恢复，不提示确认
  --dry-run               模拟运行，不实际执行
  
其他选项:
  -c, --config FILE       配置文件路径
  -v, --verbose           详细输出
  -h, --help              显示此帮助信息
  --version               显示版本信息
  --list                  列出可用备份
  --info ID               显示备份详情

示例:
  # 列出可用备份
  $(basename "$0") --list

  # 查看备份详情
  $(basename "$0") --info nas-os-20260115-120000-full

  # 恢复最新备份
  $(basename "$0") --latest

  # 恢复指定备份
  $(basename "$0") -i nas-os-20260115-120000-full

  # 选择性恢复（只恢复配置）
  $(basename "$0") -i nas-os-20260115-120000-full --include "/etc/nas-os/*"

  # 排除某些目录
  $(basename "$0") -i nas-os-20260115-120000-full --exclude "/var/lib/nas-os/logs/*"

EOF
    exit 0
}

show_version() {
    echo "$SCRIPT_NAME v$SCRIPT_VERSION"
    exit 0
}

# ============ 参数解析 ============
parse_args() {
    local action=""
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -i|--id)
                RESTORE_ID="$2"
                shift 2
                ;;
            -f|--file)
                RESTORE_FILE="$2"
                shift 2
                ;;
            -d|--dir)
                BACKUP_DIR="$2"
                shift 2
                ;;
            --include)
                RESTORE_ITEMS+=("$2")
                shift 2
                ;;
            --exclude)
                RESTORE_EXCLUDE+=("$2")
                shift 2
                ;;
            --verify)
                VERIFY_BEFORE_RESTORE=true
                shift
                ;;
            --no-verify)
                VERIFY_BEFORE_RESTORE=false
                shift
                ;;
            --rollback)
                CREATE_ROLLBACK=true
                shift
                ;;
            --no-rollback)
                CREATE_ROLLBACK=false
                shift
                ;;
            --force)
                FORCE_RESTORE=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            -c|--config)
                load_config "$2"
                shift 2
                ;;
            -v|--verbose)
                DEBUG=true
                shift
                ;;
            -h|--help)
                show_help
                ;;
            --version)
                show_version
                ;;
            --list)
                list_backups
                exit 0
                ;;
            --info)
                show_backup_info "$2"
                exit 0
                ;;
            --latest)
                select_latest_backup
                shift
                ;;
            -*)
                log_error "未知选项: $1"
                echo "使用 -h 或 --help 查看帮助信息"
                exit 1
                ;;
            *)
                # 位置参数作为备份 ID
                if [ -z "$RESTORE_ID" ] && [ -z "$RESTORE_FILE" ]; then
                    RESTORE_ID="$1"
                fi
                shift
                ;;
        esac
    done
}

# ============ 配置加载 ============
load_config() {
    local config_file="$1"
    
    if [ ! -f "$config_file" ]; then
        log_error "配置文件不存在: $config_file"
        exit 1
    fi
    
    log_info "加载配置文件: $config_file"
    
    while IFS= read -r line || [ -n "$line" ]; do
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [[ -z "${line// }" ]] && continue
        eval export "$line"
    done < "$config_file"
}

# ============ 备份列表 ============
list_backups() {
    log_info "列出可用备份..."
    
    if [ ! -d "$BACKUP_DIR" ]; then
        log_error "备份目录不存在: $BACKUP_DIR"
        exit 1
    fi
    
    printf "%-40s %-12s %-12s %-15s %s\n" "备份 ID" "类型" "大小" "时间" "状态"
    printf "%-40s %-12s %-12s %-15s %s\n" "--------" "----" "----" "----" "----"
    
    local backups
    backups=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f 2>/dev/null | sort -r)
    
    if [ -z "$backups" ]; then
        echo "没有找到备份文件"
        return 0
    fi
    
    local count=0
    while IFS= read -r backup; do
        [ -z "$backup" ] && continue
        
        local basename
        basename=$(basename "$backup")
        
        # 解析备份 ID
        local id
        id=$(echo "$basename" | sed 's/\.tar.*$//')
        
        # 提取类型
        local type="full"
        if [[ "$basename" == *"incremental"* ]]; then
            type="incremental"
        fi
        
        # 文件大小
        local size
        size=$(stat -f%z "$backup" 2>/dev/null || stat -c%s "$backup" 2>/dev/null || echo 0)
        local size_formatted
        size_formatted=$(format_size "$size")
        
        # 修改时间
        local mtime
        mtime=$(stat -f%Sm -t "%Y-%m-%d %H:%M" "$backup" 2>/dev/null || \
                stat -c%y "$backup" 2>/dev/null | cut -d'.' -f1 || echo "未知")
        
        # 验证状态
        local status="✓"
        if [ -f "${backup}.sha256" ]; then
            if sha256sum -c "${backup}.sha256" &> /dev/null; then
                status="✓ 已验证"
            else
                status="✗ 校验失败"
            fi
        else
            status="- 无校验"
        fi
        
        printf "%-40s %-12s %-12s %-15s %s\n" "$id" "$type" "$size_formatted" "$mtime" "$status"
        ((count++))
        
        if [ $count -ge 20 ]; then
            echo "... (只显示最近 20 个备份)"
            break
        fi
    done <<< "$backups"
    
    echo ""
    echo "共 $count 个备份"
}

# ============ 备份详情 ============
show_backup_info() {
    local backup_id="$1"
    
    if [ -z "$backup_id" ]; then
        log_error "请指定备份 ID"
        exit 1
    fi
    
    local backup_file
    backup_file=$(find_backup_file "$backup_id")
    
    if [ -z "$backup_file" ]; then
        log_error "找不到备份: $backup_id"
        exit 1
    fi
    
    local manifest="${backup_file%.*}"
    manifest="${manifest%.*}.manifest"
    manifest="${backup_file%.tar*}.manifest"
    
    echo "备份详情:"
    echo "================================"
    echo "ID: $backup_id"
    echo "文件: $(basename "$backup_file")"
    echo "路径: $backup_file"
    
    local size
    size=$(stat -f%z "$backup_file" 2>/dev/null || stat -c%s "$backup_file" 2>/dev/null || echo 0)
    echo "大小: $(format_size "$size")"
    
    local mtime
    mtime=$(stat -f%Sm -t "%Y-%m-%d %H:%M:%S" "$backup_file" 2>/dev/null || \
            stat -c%y "$backup_file" 2>/dev/null | cut -d'.' -f1 || echo "未知")
    echo "时间: $mtime"
    
    # 检查压缩和加密
    if [[ "$backup_file" == *.gz ]]; then
        echo "压缩: 是 (gzip)"
    elif [[ "$backup_file" == *.xz ]]; then
        echo "压缩: 是 (xz)"
    else
        echo "压缩: 否"
    fi
    
    if [[ "$backup_file" == *.enc ]]; then
        echo "加密: 是 (AES-256-GCM)"
    else
        echo "加密: 否"
    fi
    
    # 校验状态
    if [ -f "${backup_file}.sha256" ]; then
        echo "校验: $(cat "${backup_file}.sha256")"
    else
        echo "校验: 无校验文件"
    fi
    
    # 清单信息
    if [ -f "$manifest" ]; then
        echo ""
        echo "清单内容:"
        cat "$manifest"
    fi
}

# ============ 查找备份 ============
find_backup_file() {
    local backup_id="$1"
    
    # 尝试多种匹配模式
    local patterns=(
        "${backup_id}.tar.gz.enc"
        "${backup_id}.tar.gz"
        "${backup_id}.tar.xz.enc"
        "${backup_id}.tar.xz"
        "${backup_id}.tar.enc"
        "${backup_id}.tar"
        "${backup_id}*.tar.gz.enc"
        "${backup_id}*.tar.gz"
        "${backup_id}*.tar"
    )
    
    for pattern in "${patterns[@]}"; do
        local found
        found=$(find "$BACKUP_DIR" -name "$pattern" -type f 2>/dev/null | head -1)
        if [ -n "$found" ]; then
            echo "$found"
            return 0
        fi
    done
    
    return 1
}

select_latest_backup() {
    log_info "查找最新备份..."
    
    local latest
    latest=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -printf '%T@ %p\n' 2>/dev/null | \
             sort -rn | head -1 | cut -d' ' -f2-)
    
    if [ -z "$latest" ]; then
        log_error "没有找到备份文件"
        exit 1
    fi
    
    RESTORE_FILE="$latest"
    RESTORE_ID=$(basename "$latest" | sed 's/\.tar.*$//')
    
    log_info "选择最新备份: $RESTORE_ID"
}

# ============ 验证备份 ============
verify_backup() {
    local backup_file="$1"
    
    log_info "验证备份完整性..."
    
    if [ ! -f "$backup_file" ]; then
        log_error "备份文件不存在: $backup_file"
        return 1
    fi
    
    local checksum_file="${backup_file}.sha256"
    
    if [ ! -f "$checksum_file" ]; then
        log_warn "没有找到校验文件，跳过验证"
        return 0
    fi
    
    log_progress "计算校验和..."
    
    # 显示进度
    local file_size
    file_size=$(stat -f%z "$backup_file" 2>/dev/null || stat -c%s "$backup_file" 2>/dev/null || echo 0)
    local size_human
    size_human=$(format_size "$file_size")
    
    log_info "文件大小: $size_human"
    
    if sha256sum -c "$checksum_file" &> /dev/null; then
        log_success "备份完整性验证通过"
        return 0
    else
        log_error "备份完整性验证失败！文件可能已损坏"
        return 1
    fi
}

# ============ 创建回滚点 ============
create_rollback_point() {
    log_info "创建回滚点..."
    
    local rollback_id="rollback-$(date '+%Y%m%d-%H%M%S')"
    local rollback_file="${ROLLBACK_DIR}/${rollback_id}.tar.gz"
    
    mkdir -p "$ROLLBACK_DIR"
    
    local tar_opts=()
    tar_opts+=("-czf" "$rollback_file")
    tar_opts+=("--exclude" "$ROLLBACK_DIR")
    tar_opts+=("--exclude" "$BACKUP_DIR")
    tar_opts+=("--exclude" "*.log")
    tar_opts+=("--exclude" "*.tmp")
    
    # 数据目录
    if [ -d "$DATA_DIR" ]; then
        tar_opts+=("-C" "$(dirname "$DATA_DIR")" "$(basename "$DATA_DIR")")
    fi
    
    # 配置目录
    if [ -d "$CONFIG_DIR" ]; then
        tar_opts+=("-C" "$(dirname "$CONFIG_DIR")" "$(basename "$CONFIG_DIR")")
    fi
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY-RUN] 将创建回滚点: $rollback_file"
        return 0
    fi
    
    log_debug "执行: tar ${tar_opts[*]}"
    
    if ! tar "${tar_opts[@]}" 2>> "$LOG_FILE"; then
        log_warn "回滚点创建失败，但将继续恢复"
        return 0
    fi
    
    # 清理旧回滚点
    find "$ROLLBACK_DIR" -name "rollback-*.tar.gz" -type f -mtime +$ROLLBACK_MAX_AGE -delete 2>/dev/null || true
    
    log_success "回滚点创建完成: $rollback_file"
    ROLLBACK_FILE="$rollback_file"
    
    return 0
}

# ============ 执行恢复 ============
do_restore() {
    local backup_file="$1"
    
    log_info "开始恢复..."
    RESTORE_START_TIME=$(date '+%Y-%m-%d %H:%M:%S')
    
    # 准备临时目录
    TEMP_DIR=$(mktemp -d "${DATA_DIR}/.restore-XXXXXX")
    
    # 确定解压方式
    local extract_cmd="tar -xf"
    local decompress=false
    
    if [[ "$backup_file" == *.gz ]]; then
        extract_cmd="tar -xzf"
        decompress=true
    elif [[ "$backup_file" == *.xz ]]; then
        extract_cmd="tar -xJf"
        decompress=true
    fi
    
    # 处理加密
    local actual_file="$backup_file"
    if [[ "$backup_file" == *.enc ]]; then
        log_info "解密备份文件..."
        actual_file="${TEMP_DIR}/$(basename "${backup_file%.enc}")"
        
        if [ "$DRY_RUN" = true ]; then
            log_info "[DRY-RUN] 将解密: $backup_file"
        else
            if ! decrypt_file "$backup_file" "$actual_file"; then
                log_error "解密失败"
                return 1
            fi
            
            # 更新解压命令
            if [[ "$actual_file" == *.gz ]]; then
                extract_cmd="tar -xzf"
            elif [[ "$actual_file" == *.xz ]]; then
                extract_cmd="tar -xJf"
            fi
        fi
    fi
    
    # 提取包含文件列表
    log_info "分析备份内容..."
    local file_count
    file_count=$($extract_cmd "$actual_file" -tf 2>/dev/null | wc -l || echo 0)
    log_info "备份包含 $file_count 个条目"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY-RUN] 将恢复 $file_count 个条目到 $DATA_DIR"
        return 0
    fi
    
    # 构建提取选项
    local tar_opts=()
    tar_opts+=("-xf" "$actual_file")
    tar_opts+=("-C" "$TEMP_DIR")
    
    # 选择性恢复
    if [ ${#RESTORE_ITEMS[@]} -gt 0 ]; then
        log_info "选择性恢复模式"
        for pattern in "${RESTORE_ITEMS[@]}"; do
            tar_opts+=("--include" "$pattern")
        done
    fi
    
    # 排除模式
    if [ ${#RESTORE_EXCLUDE[@]} -gt 0 ]; then
        for pattern in "${RESTORE_EXCLUDE[@]}"; do
            tar_opts+=("--exclude" "$pattern")
        done
    fi
    
    # 提取文件
    log_info "提取备份文件..."
    show_progress 0 100 "提取中..."
    
    if ! tar "${tar_opts[@]}" 2>> "$LOG_FILE"; then
        log_error "提取失败"
        return 1
    fi
    
    show_progress 50 100 "提取中..."
    
    # 应用恢复
    log_info "应用恢复..."
    apply_restore "$TEMP_DIR"
    
    show_progress 100 100 "完成"
    
    RESTORE_END_TIME=$(date '+%Y-%m-%d %H:%M:%S')
    RESTORE_STATUS="成功"
    
    log_success "恢复完成"
    return 0
}

# ============ 应用恢复 ============
apply_restore() {
    local extract_dir="$1"
    
    # 遍历提取目录，将文件移动到目标位置
    local moved=0
    local failed=0
    
    # 数据目录
    if [ -d "$extract_dir/var/lib/nas-os" ]; then
        log_debug "恢复数据目录..."
        if ! rsync -a --delete "$extract_dir/var/lib/nas-os/" "$DATA_DIR/" 2>> "$LOG_FILE"; then
            log_warn "部分数据恢复失败"
            ((failed++))
        else
            ((moved++))
        fi
    fi
    
    # 配置目录
    if [ -d "$extract_dir/etc/nas-os" ]; then
        log_debug "恢复配置目录..."
        if ! rsync -a "$extract_dir/etc/nas-os/" "$CONFIG_DIR/" 2>> "$LOG_FILE"; then
            log_warn "部分配置恢复失败"
            ((failed++))
        else
            ((moved++))
        fi
    fi
    
    # 其他可能的结构
    if [ -d "$extract_dir/nas-os" ]; then
        log_debug "恢复 nas-os 目录..."
        if ! rsync -a --delete "$extract_dir/nas-os/" "$DATA_DIR/" 2>> "$LOG_FILE"; then
            ((failed++))
        else
            ((moved++))
        fi
    fi
    
    if [ $failed -gt 0 ]; then
        log_warn "恢复过程中有 $failed 个项目失败"
        return 1
    fi
    
    log_info "已恢复 $moved 个目录"
    return 0
}

# ============ 解密 ============
decrypt_file() {
    local input_file="$1"
    local output_file="$2"
    
    if [ ! -f "$ENCRYPTION_KEY_FILE" ]; then
        log_error "加密密钥文件不存在: $ENCRYPTION_KEY_FILE"
        return 1
    fi
    
    local key
    key=$(cat "$ENCRYPTION_KEY_FILE")
    
    log_debug "解密: $input_file -> $output_file"
    
    if ! openssl enc -aes-256-gcm -d -pbkdf2 -in "$input_file" -out "$output_file" -pass pass:"$key" 2>> "$LOG_FILE"; then
        log_error "解密失败"
        return 1
    fi
    
    return 0
}

# ============ 回滚 ============
rollback_restore() {
    log_warn "恢复失败，执行回滚..."
    
    if [ -z "$ROLLBACK_FILE" ] || [ ! -f "$ROLLBACK_FILE" ]; then
        log_error "没有找到回滚点，无法恢复"
        return 1
    fi
    
    log_info "从回滚点恢复: $ROLLBACK_FILE"
    
    # 清理失败的恢复
    if [ -d "$DATA_DIR" ]; then
        rm -rf "$DATA_DIR"/*
    fi
    
    # 从回滚点恢复
    if ! tar -xzf "$ROLLBACK_FILE" -C / 2>> "$LOG_FILE"; then
        log_error "回滚失败！"
        return 1
    fi
    
    log_success "回滚完成"
    return 0
}

# ============ 通知 ============
send_notification() {
    local status="$1"
    local message="$2"
    
    if [ "$ENABLE_NOTIFICATION" != true ]; then
        return 0
    fi
    
    local title="NAS-OS 恢复通知"
    local body
    body=$(cat << EOF
恢复状态: $status
备份 ID: $RESTORE_ID
开始时间: $RESTORE_START_TIME
结束时间: $RESTORE_END_TIME

$message
EOF
)
    
    # Webhook 通知
    if [ -n "$NOTIFICATION_WEBHOOK" ]; then
        local payload
        payload=$(cat << EOF
{
    "title": "$title",
    "message": "$body",
    "status": "$status",
    "restore_id": "$RESTORE_ID",
    "timestamp": "$(date -Iseconds)"
}
EOF
)
        if command -v curl &> /dev/null; then
            curl -s -X POST -H "Content-Type: application/json" \
                -d "$payload" "$NOTIFICATION_WEBHOOK" >> "$LOG_FILE" 2>&1
        fi
    fi
    
    log_debug "通知发送完成"
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

confirm_restore() {
    if [ "$FORCE_RESTORE" = true ] || [ "$DRY_RUN" = true ]; then
        return 0
    fi
    
    echo ""
    echo "=========================================="
    echo "即将执行恢复操作"
    echo "=========================================="
    echo "备份 ID: $RESTORE_ID"
    echo "备份文件: $(basename "$RESTORE_FILE")"
    echo "目标目录: $DATA_DIR"
    
    if [ ${#RESTORE_ITEMS[@]} -gt 0 ]; then
        echo "包含模式: ${RESTORE_ITEMS[*]}"
    fi
    
    if [ ${#RESTORE_EXCLUDE[@]} -gt 0 ]; then
        echo "排除模式: ${RESTORE_EXCLUDE[*]}"
    fi
    
    echo "=========================================="
    echo ""
    
    read -r -p "确认执行恢复？ [y/N]: " response
    case "$response" in
        [yY][eE][sS]|[yY])
            return 0
            ;;
        *)
            log_info "取消恢复"
            return 1
            ;;
    esac
}

# ============ 清理 ============
cleanup() {
    if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
        log_debug "清理临时目录: $TEMP_DIR"
    fi
}

# ============ 主流程 ============
main() {
    # 初始化
    log_init
    parse_args "$@"
    
    log_info "=========================================="
    log_info "$SCRIPT_NAME v$SCRIPT_VERSION"
    log_info "=========================================="
    
    # 确定备份文件
    if [ -n "$RESTORE_FILE" ]; then
        if [ ! -f "$RESTORE_FILE" ]; then
            log_error "备份文件不存在: $RESTORE_FILE"
            exit 1
        fi
    elif [ -n "$RESTORE_ID" ]; then
        RESTORE_FILE=$(find_backup_file "$RESTORE_ID")
        if [ -z "$RESTORE_FILE" ]; then
            log_error "找不到备份: $RESTORE_ID"
            exit 1
        fi
        log_info "找到备份文件: $RESTORE_FILE"
    else
        log_error "请指定备份 ID 或文件路径"
        echo "使用 --list 查看可用备份，或使用 --help 查看帮助"
        exit 1
    fi
    
    # 更新备份 ID
    if [ -z "$RESTORE_ID" ]; then
        RESTORE_ID=$(basename "$RESTORE_FILE" | sed 's/\.tar.*$//')
    fi
    
    log_info "备份 ID: $RESTORE_ID"
    log_info "备份文件: $RESTORE_FILE"
    
    # 验证备份
    if [ "$VERIFY_BEFORE_RESTORE" = true ]; then
        if ! verify_backup "$RESTORE_FILE"; then
            log_error "备份验证失败，终止恢复"
            exit 1
        fi
    fi
    
    # 确认恢复
    if ! confirm_restore; then
        exit 0
    fi
    
    # 创建回滚点
    ROLLBACK_FILE=""
    if [ "$CREATE_ROLLBACK" = true ] && [ "$DRY_RUN" != true ]; then
        create_rollback_point
    fi
    
    # 执行恢复
    trap 'cleanup' EXIT
    
    local restore_result=0
    if ! do_restore "$RESTORE_FILE"; then
        RESTORE_STATUS="失败"
        restore_result=1
        
        # 尝试回滚
        if [ -n "$ROLLBACK_FILE" ]; then
            rollback_restore
        fi
    fi
    
    # 完成
    RESTORE_END_TIME=$(date '+%Y-%m-%d %H:%M:%S')
    
    log_info "=========================================="
    log_info "恢复 $RESTORE_STATUS"
    log_info "开始: $RESTORE_START_TIME"
    log_info "结束: $RESTORE_END_TIME"
    log_info "=========================================="
    
    # 发送通知
    if [ "$DRY_RUN" != true ]; then
        send_notification "$RESTORE_STATUS" ""
    fi
    
    exit $restore_result
}

# 运行
main "$@"