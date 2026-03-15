#!/bin/bash
# NAS-OS 日志轮转脚本
# 自动管理日志文件大小和保留策略
#
# v2.58.0 工部创建
#
# 功能:
# - 日志文件大小监控和轮转
# - 按大小/时间切割
# - 自动压缩旧日志
# - 清理过期日志
# - 支持 logrotate 兼容配置
# - 钩子支持（轮转前后执行脚本）
#
# 用法:
#   ./log-rotate.sh                # 执行日志轮转
#   ./log-rotate.sh --status       # 查看日志状态
#   ./log-rotate.sh --config       # 生成配置文件
#   ./log-rotate.sh --dry-run      # 预览模式(不实际执行)
#   ./log-rotate.sh --force        # 强制轮转所有日志

set -euo pipefail

#===========================================
# 版本与路径
#===========================================

VERSION="2.58.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

#===========================================
# 配置
#===========================================

# 配置文件
CONFIG_FILE="${CONFIG_FILE:-/etc/nas-os/log-rotate.conf}"
STATE_FILE="${STATE_FILE:-/var/lib/nas-os/log-rotate.state}"

# 日志目录
LOG_DIRS="${LOG_DIRS:-/var/log/nas-os /var/log/nginx /var/log/mysql /var/log/redis}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"

# 默认轮转配置
MAX_SIZE="${MAX_SIZE:-100M}"              # 单文件最大大小
MAX_FILES="${MAX_FILES:-10}"               # 保留文件数
MAX_AGE="${MAX_AGE:-30}"                   # 保留天数
COMPRESS="${COMPRESS:-true}"               # 压缩旧日志
COMPRESS_CMD="${COMPRESS_CMD:-gzip}"       # 压缩命令
COMPRESS_EXT="${COMPRESS_EXT:-gz}"         # 压缩扩展名
COMPRESS_LEVEL="${COMPRESS_LEVEL:-6}"      # 压缩级别

# 轮转策略
ROTATE_SIZE="${ROTATE_SIZE:-100M}"         # 按大小轮转阈值
ROTATE_TIME="${ROTATE_TIME:-daily}"        # 按时间轮转 (hourly/daily/weekly/monthly)
ROTATE_COUNT="${ROTATE_COUNT:-10}"        # 保留轮转文件数

# 日志模式（权限）
LOG_MODE="${LOG_MODE:-0644}"
LOG_OWNER="${LOG_OWNER:-root:root}"

# 钩子脚本目录
HOOKS_DIR="${HOOKS_DIR:-/etc/nas-os/log-rotate-hooks.d}"

# 预览模式
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"

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

ROTATED_COUNT=0
COMPRESSED_COUNT=0
DELETED_COUNT=0
SAVED_SPACE=0
ERROR_COUNT=0

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
}

# 解析大小字符串 (如 "100M" -> 字节数)
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
    mkdir -p "$DATA_DIR" 2>/dev/null || true
    mkdir -p "$(dirname "$STATE_FILE")" 2>/dev/null || true
    mkdir -p "$HOOKS_DIR" 2>/dev/null || true
    
    # 解析大小配置
    ROTATE_SIZE_BYTES=$(parse_size "$ROTATE_SIZE")
    MAX_SIZE_BYTES=$(parse_size "$MAX_SIZE")
}

#===========================================
# 状态管理
#===========================================

load_state() {
    if [ -f "$STATE_FILE" ]; then
        # shellcheck source=/dev/null
        source "$STATE_FILE" 2>/dev/null || true
    fi
}

save_state() {
    local log_file="$1"
    local timestamp=$(date +%s)
    local state_dir=$(dirname "$STATE_FILE")
    
    mkdir -p "$state_dir" 2>/dev/null || true
    
    # 追加状态记录
    echo "LAST_ROTATE_${log_file//\//_}=$timestamp" >> "$STATE_FILE"
}

get_last_rotate_time() {
    local log_file="$1"
    local var_name="LAST_ROTATE_${log_file//\//_}"
    
    if [ -f "$STATE_FILE" ]; then
        grep "^$var_name=" "$STATE_FILE" 2>/dev/null | cut -d= -f2 || echo 0
    else
        echo 0
    fi
}

#===========================================
# 钩子执行
#===========================================

run_hook() {
    local hook_type="$1"  # pre-rotate, post-rotate
    local log_file="$2"
    
    local hook_dir="$HOOKS_DIR/$hook_type"
    
    if [ -d "$hook_dir" ]; then
        for hook in "$hook_dir"/*; do
            if [ -x "$hook" ]; then
                log INFO "执行钩子: $hook"
                LOG_FILE_PATH="$log_file" "$hook" 2>/dev/null || {
                    log WARN "钩子执行失败: $hook"
                }
            fi
        done
    fi
}

#===========================================
# 轮转操作
#===========================================

rotate_log() {
    local log_file="$1"
    
    if [ ! -f "$log_file" ]; then
        return 0
    fi
    
    local log_size=$(get_file_size "$log_file")
    local should_rotate=false
    local rotate_reason=""
    
    # 检查是否需要轮转
    
    # 1. 按大小检查
    if [ "$log_size" -ge "$ROTATE_SIZE_BYTES" ]; then
        should_rotate=true
        rotate_reason="size ($(format_size $log_size) >= $(format_size $ROTATE_SIZE_BYTES))"
    fi
    
    # 2. 按时间检查
    local last_rotate=$(get_last_rotate_time "$log_file")
    local now=$(date +%s)
    local time_diff=$((now - last_rotate))
    
    case "$ROTATE_TIME" in
        hourly)
            if [ $time_diff -ge 3600 ]; then
                should_rotate=true
                rotate_reason="time (hourly)"
            fi
            ;;
        daily)
            if [ $time_diff -ge 86400 ]; then
                should_rotate=true
                rotate_reason="time (daily)"
            fi
            ;;
        weekly)
            if [ $time_diff -ge 604800 ]; then
                should_rotate=true
                rotate_reason="time (weekly)"
            fi
            ;;
        monthly)
            if [ $time_diff -ge 2592000 ]; then
                should_rotate=true
                rotate_reason="time (monthly)"
            fi
            ;;
    esac
    
    # 3. 强制轮转
    if [ "$FORCE" = "true" ]; then
        should_rotate=true
        rotate_reason="forced"
    fi
    
    if [ "$should_rotate" = "false" ]; then
        return 0
    fi
    
    log INFO "轮转日志: $log_file (原因: $rotate_reason)"
    
    # 执行预轮转钩子
    run_hook "pre-rotate" "$log_file"
    
    if [ "$DRY_RUN" = "true" ]; then
        log DRYRUN "将轮转: $log_file"
        return 0
    fi
    
    # 生成轮转文件名
    local timestamp=$(date '+%Y%m%d%H%M%S')
    local rotated_file="${log_file}.${timestamp}"
    
    # 执行轮转
    if mv "$log_file" "$rotated_file"; then
        # 重新创建空日志文件
        touch "$log_file"
        chmod "$LOG_MODE" "$log_file"
        chown "$LOG_OWNER" "$log_file" 2>/dev/null || true
        
        ROTATED_COUNT=$((ROTATED_COUNT + 1))
        log SUCCESS "轮转完成: $log_file -> $rotated_file"
        
        # 压缩轮转后的文件
        if [ "$COMPRESS" = "true" ]; then
            compress_log "$rotated_file"
        fi
        
        # 保存状态
        save_state "$log_file"
        
        # 执行后轮转钩子
        run_hook "post-rotate" "$log_file"
        
        return 0
    else
        log ERROR "轮转失败: $log_file"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        return 1
    fi
}

#===========================================
# 压缩操作
#===========================================

compress_log() {
    local log_file="$1"
    
    if [ ! -f "$log_file" ]; then
        return 0
    fi
    
    # 检查是否已压缩
    if [[ "$log_file" == *".$COMPRESS_EXT" ]]; then
        return 0
    fi
    
    local original_size=$(get_file_size "$log_file")
    local compressed_file="${log_file}.${COMPRESS_EXT}"
    
    if [ "$DRY_RUN" = "true" ]; then
        log DRYRUN "将压缩: $log_file"
        return 0
    fi
    
    log INFO "压缩日志: $log_file"
    
    if $COMPRESS_CMD -"$COMPRESS_LEVEL" "$log_file"; then
        local compressed_size=$(get_file_size "$compressed_file")
        local saved=$((original_size - compressed_size))
        SAVED_SPACE=$((SAVED_SPACE + saved))
        COMPRESSED_COUNT=$((COMPRESSED_COUNT + 1))
        log SUCCESS "压缩完成: $(format_size $original_size) -> $(format_size $compressed_size)"
        return 0
    else
        log ERROR "压缩失败: $log_file"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        return 1
    fi
}

#===========================================
# 清理操作
#===========================================

cleanup_old_logs() {
    local log_pattern="$1"
    local log_dir=$(dirname "$log_pattern")
    local log_name=$(basename "$log_pattern")
    
    if [ ! -d "$log_dir" ]; then
        return 0
    fi
    
    log INFO "清理旧日志: $log_dir"
    
    # 获取所有匹配的日志文件，按时间排序
    local files=()
    while IFS= read -r file; do
        files+=("$file")
    done < <(find "$log_dir" -name "${log_name}.*" -type f -printf '%T@ %p\n' 2>/dev/null | sort -n | cut -d' ' -f2-)
    
    local count=${#files[@]}
    local to_delete=$((count - ROTATE_COUNT))
    
    if [ $to_delete -le 0 ]; then
        return 0
    fi
    
    # 删除最旧的文件
    for ((i = 0; i < to_delete; i++)); do
        local file="${files[$i]}"
        local age=$(get_file_age_days "$file")
        
        # 同时检查年龄限制
        if [ $age -gt $MAX_AGE ]; then
            if [ "$DRY_RUN" = "true" ]; then
                log DRYRUN "将删除: $file (超过 $MAX_AGE 天)"
                DELETED_COUNT=$((DELETED_COUNT + 1))
                local file_size=$(get_file_size "$file")
                SAVED_SPACE=$((SAVED_SPACE + file_size))
            else
                local file_size=$(get_file_size "$file")
                if rm -f "$file"; then
                    DELETED_COUNT=$((DELETED_COUNT + 1))
                    SAVED_SPACE=$((SAVED_SPACE + file_size))
                    log SUCCESS "已删除: $file (超过 $MAX_AGE 天)"
                else
                    log ERROR "删除失败: $file"
                    ERROR_COUNT=$((ERROR_COUNT + 1))
                fi
            fi
        fi
    done
}

#===========================================
# 扫描日志目录
#===========================================

scan_log_dirs() {
    log INFO "扫描日志目录..."
    
    for log_dir in $LOG_DIRS; do
        if [ ! -d "$log_dir" ]; then
            log WARN "目录不存在: $log_dir"
            continue
        fi
        
        log INFO "处理目录: $log_dir"
        
        # 查找所有 .log 文件
        while IFS= read -r log_file; do
            rotate_log "$log_file"
            cleanup_old_logs "$log_file"
        done < <(find "$log_dir" -name "*.log" -type f 2>/dev/null)
    done
}

#===========================================
# 状态报告
#===========================================

show_status() {
    echo ""
    echo "========================================"
    echo "  日志轮转状态"
    echo "========================================"
    echo ""
    
    echo "--- 日志目录统计 ---"
    local total_size=0
    local total_files=0
    
    for log_dir in $LOG_DIRS; do
        if [ -d "$log_dir" ]; then
            local count=$(find "$log_dir" -type f \( -name "*.log" -o -name "*.log.*" \) 2>/dev/null | wc -l)
            local size=$(du -sb "$log_dir" 2>/dev/null | cut -f1)
            total_size=$((total_size + size))
            total_files=$((total_files + count))
            echo "  $log_dir: $count 个日志文件, $(format_size $size)"
            
            # 显示大文件
            while IFS= read -r file; do
                local file_size=$(get_file_size "$file")
                if [ $file_size -gt $ROTATE_SIZE_BYTES ]; then
                    echo "    - 需要轮转: $(basename $file) ($(format_size $file_size))"
                fi
            done < <(find "$log_dir" -name "*.log" -type f 2>/dev/null)
        fi
    done
    
    echo ""
    echo "总计: $total_files 个日志文件, $(format_size $total_size)"
    echo ""
    
    echo "--- 轮转配置 ---"
    echo "  最大文件大小: $ROTATE_SIZE"
    echo "  轮转时间: $ROTATE_TIME"
    echo "  保留文件数: $ROTATE_COUNT"
    echo "  保留天数: $MAX_AGE"
    echo "  压缩: $COMPRESS"
    echo ""
    
    # 显示状态文件
    if [ -f "$STATE_FILE" ]; then
        echo "--- 最近轮转 ---"
        cat "$STATE_FILE"
        echo ""
    fi
}

#===========================================
# 生成配置文件
#===========================================

generate_config() {
    local config_path="${1:-$CONFIG_FILE}"
    
    cat > "$config_path" << 'EOF'
# NAS-OS 日志轮转配置文件
# 复制到 /etc/nas-os/log-rotate.conf

# 扫描的日志目录（空格分隔）
LOG_DIRS="/var/log/nas-os /var/log/nginx /var/log/mysql /var/log/redis"

# 默认轮转配置
ROTATE_SIZE="100M"           # 单文件最大大小
ROTATE_TIME="daily"          # 轮转频率 (hourly/daily/weekly/monthly)
ROTATE_COUNT=10              # 保留轮转文件数

# 清理配置
MAX_SIZE="100M"              # 单文件最大大小（超限警告）
MAX_FILES=10                 # 最大文件数
MAX_AGE=30                   # 保留天数

# 压缩配置
COMPRESS="true"              # 是否压缩旧日志
COMPRESS_CMD="gzip"          # 压缩命令
COMPRESS_EXT="gz"            # 压缩扩展名
COMPRESS_LEVEL=6             # 压缩级别 (1-9)

# 日志权限
LOG_MODE="0644"              # 新日志文件权限
LOG_OWNER="root:root"        # 新日志文件所有者

# 钩子目录
HOOKS_DIR="/etc/nas-os/log-rotate-hooks.d"
EOF
    
    # 创建钩子目录结构
    mkdir -p "$(dirname "$config_path")/log-rotate-hooks.d/pre-rotate"
    mkdir -p "$(dirname "$config_path")/log-rotate-hooks.d/post-rotate"
    
    log SUCCESS "配置文件已生成: $config_path"
    log INFO "钩子目录: $(dirname "$config_path")/log-rotate-hooks.d/"
}

#===========================================
# 帮助信息
#===========================================

show_help() {
    cat << EOF
NAS-OS 日志轮转脚本 v${VERSION}

用法:
  $SCRIPT_NAME [选项]

选项:
  --status              显示日志状态
  --dry-run             预览模式（不实际执行）
  --force               强制轮转所有日志
  --config <file>       指定配置文件
  --generate-config     生成默认配置文件
  --help                显示帮助信息
  --version             显示版本信息

配置文件: $CONFIG_FILE
状态文件: $STATE_FILE

轮转策略:
  - 按大小: 文件超过 ROTATE_SIZE 时轮转
  - 按时间: 根据 ROTATE_TIME 周期轮转
  - 保留: 保留最近 ROTATE_COUNT 个文件
  - 清理: 删除超过 MAX_AGE 天的文件

钩子脚本:
  pre-rotate:  $HOOKS_DIR/pre-rotate/
  post-rotate: $HOOKS_DIR/post-rotate/

示例:
  $SCRIPT_NAME                    # 执行日志轮转
  $SCRIPT_NAME --status           # 查看日志状态
  $SCRIPT_NAME --dry-run          # 预览将执行的操作
  $SCRIPT_NAME --force            # 强制轮转所有日志
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
            --force)
                FORCE="true"
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
    
    # 加载状态
    load_state
    
    # 执行操作
    case "$action" in
        status)
            show_status
            ;;
        rotate)
            log INFO "开始日志轮转..."
            
            if [ "$DRY_RUN" = "true" ]; then
                log INFO "预览模式 - 不会实际执行操作"
            fi
            
            # 扫描日志目录
            scan_log_dirs
            
            # 打印统计
            echo ""
            echo "========================================"
            echo "  轮转完成"
            echo "========================================"
            echo "轮转文件: $ROTATED_COUNT"
            echo "压缩文件: $COMPRESSED_COUNT"
            echo "删除文件: $DELETED_COUNT"
            echo "节省空间: $(format_size $SAVED_SPACE)"
            echo "错误数量: $ERROR_COUNT"
            echo "========================================"
            ;;
    esac
}

# 运行主函数
main "$@"