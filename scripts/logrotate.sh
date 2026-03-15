#!/bin/bash
# NAS-OS 日志轮转脚本
# 自动管理日志文件大小和保留策略
#
# v2.48.0 工部创建
#
# 功能:
# - 日志文件大小监控和轮转
# - 压缩旧日志节省空间
# - 自动清理过期日志
# - 支持 logrotate 兼容配置
# - 支持多种日志格式 (服务日志、访问日志、审计日志)
# - 钩子支持 (轮转前后执行脚本)
#
# 用法:
#   ./logrotate.sh                # 执行日志轮转
#   ./logrotate.sh --status       # 查看日志状态
#   ./logrotate.sh --config       # 生成配置文件
#   ./logrotate.sh --dry-run      # 预览模式(不实际执行)
#   ./logrotate.sh --force        # 强制轮转所有日志

set -euo pipefail

#===========================================
# 版本与路径
#===========================================

VERSION="2.48.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

#===========================================
# 配置
#===========================================

# 配置文件
CONFIG_FILE="${CONFIG_FILE:-/etc/nas-os/logrotate.conf}"
STATE_FILE="${STATE_FILE:-/var/lib/nas-os/logrotate.state}"

# 日志目录
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"

# 默认轮转配置
MAX_SIZE="${MAX_SIZE:-100M}"              # 单文件最大大小
MAX_FILES="${MAX_FILES:-10}"               # 保留文件数
MAX_AGE="${MAX_AGE:-30}"                   # 保留天数
COMPRESS="${COMPRESS:-true}"               # 压缩旧日志
COMPRESS_CMD="${COMPRESS_CMD:-gzip}"       # 压缩命令
COMPRESS_EXT="${COMPRESS_EXT:-gz}"         # 压缩扩展名

# 钩子脚本目录
HOOKS_DIR="${HOOKS_DIR:-/etc/nas-os/logrotate-hooks.d}"

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
        K|KB) echo $((num * 1024)) ;;
        M|MB) echo $((num * 1024 * 1024)) ;;
        G|GB) echo $((num * 1024 * 1024 * 1024)) ;;
        *)    echo "$num" ;;
    esac
}

# 格式化大小 (字节 -> 人类可读)
format_size() {
    local bytes=$1
    
    if [ $bytes -ge 1073741824 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes / 1073741824}") GB"
    elif [ $bytes -ge 1048576 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes / 1048576}") MB"
    elif [ $bytes -ge 1024 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes / 1024}") KB"
    else
        echo "${bytes} B"
    fi
}

# 获取文件大小 (字节)
get_file_size() {
    local file="$1"
    stat -c %s "$file" 2>/dev/null || echo "0"
}

# 执行钩子
run_hook() {
    local hook_name="$1"
    local hook_dir="${HOOKS_DIR}/${hook_name}.d"
    
    if [ -d "$hook_dir" ]; then
        for script in "$hook_dir"/*; do
            if [ -x "$script" ]; then
                log INFO "执行钩子: $script"
                if [ "$DRY_RUN" = true ]; then
                    log DRYRUN "跳过执行: $script"
                else
                    "$script" 2>/dev/null || log WARN "钩子执行失败: $script"
                fi
            fi
        done
    fi
}

#===========================================
# 日志轮转
#===========================================

# 轮转单个日志文件
rotate_file() {
    local log_file="$1"
    local max_size="$2"
    local max_files="$3"
    local max_age="$4"
    local should_compress="$5"
    
    # 检查文件是否存在
    if [ ! -f "$log_file" ]; then
        return 0
    fi
    
    local file_size=$(get_file_size "$log_file")
    local max_bytes=$(parse_size "$max_size")
    
    # 检查是否需要轮转
    if [ "$FORCE" = true ] || [ $file_size -ge $max_bytes ]; then
        log INFO "轮转日志: $log_file ($(format_size $file_size))"
        
        if [ "$DRY_RUN" = true ]; then
            log DRYRUN "将轮转: $log_file"
            ROTATED_COUNT=$((ROTATED_COUNT + 1))
            return 0
        fi
        
        # 执行轮转前钩子
        run_hook "prerotate"
        
        # 获取时间戳
        local timestamp=$(date '+%Y%m%d-%H%M%S')
        local base_name=$(basename "$log_file")
        local dir_name=$(dirname "$log_file")
        
        # 重命名现有轮转文件
        local i=1
        while [ $i -lt $max_files ]; do
            local old_file="${log_file}.${i}"
            local new_file="${log_file}.$((i + 1))"
            
            if [ -f "${old_file}.${COMPRESS_EXT}" ]; then
                mv "${old_file}.${COMPRESS_EXT}" "${new_file}.${COMPRESS_EXT}" 2>/dev/null || true
            elif [ -f "$old_file" ]; then
                mv "$old_file" "$new_file" 2>/dev/null || true
            fi
            
            i=$((i + 1))
        done
        
        # 轮转当前日志
        local rotated_file="${log_file}.1"
        mv "$log_file" "$rotated_file" 2>/dev/null || {
            log ERROR "轮转失败: $log_file"
            return 1
        }
        
        # 创建新的空日志文件
        touch "$log_file" 2>/dev/null || true
        chmod 644 "$log_file" 2>/dev/null || true
        
        # 压缩轮转的日志
        if [ "$should_compress" = true ] && [ -f "$rotated_file" ]; then
            log INFO "压缩: $rotated_file"
            $COMPRESS_CMD -f "$rotated_file" 2>/dev/null && {
                COMPRESSED_COUNT=$((COMPRESSED_COUNT + 1))
                local compressed_size=$(get_file_size "${rotated_file}.${COMPRESS_EXT}")
                SAVED_SPACE=$((SAVED_SPACE + file_size - compressed_size))
            } || log WARN "压缩失败: $rotated_file"
        fi
        
        ROTATED_COUNT=$((ROTATED_COUNT + 1))
        
        # 执行轮转后钩子
        run_hook "postrotate"
    fi
    
    return 0
}

# 清理过期日志
cleanup_old_logs() {
    local log_pattern="$1"
    local max_files="$2"
    local max_age="$3"
    
    log INFO "清理过期日志: $log_pattern"
    
    if [ "$DRY_RUN" = true ]; then
        log DRYRUN "将清理超过 $max_age 天或超过 $max_files 个的日志文件"
        return 0
    fi
    
    local dir_name=$(dirname "$log_pattern")
    local base_pattern=$(basename "$log_pattern")
    
    # 按时间清理
    if [ -d "$dir_name" ]; then
        local deleted=0
        
        # 清理超过保留天数的日志
        while IFS= read -r file; do
            if [ -f "$file" ]; then
                local file_size=$(get_file_size "$file")
                rm -f "$file" 2>/dev/null && {
                    DELETED_COUNT=$((DELETED_COUNT + 1))
                    SAVED_SPACE=$((SAVED_SPACE + file_size))
                    deleted=$((deleted + 1))
                }
            fi
        done < <(find "$dir_name" -name "${base_pattern}*" -mtime +$max_age 2>/dev/null)
        
        # 清理超过保留数量的日志 (保留最新的 max_files 个)
        local count=$(find "$dir_name" -name "${base_pattern}.*" -type f 2>/dev/null | wc -l)
        
        if [ $count -gt $max_files ]; then
            local to_delete=$((count - max_files))
            
            while IFS= read -r file; do
                if [ $to_delete -le 0 ]; then
                    break
                fi
                
                if [ -f "$file" ]; then
                    local file_size=$(get_file_size "$file")
                    rm -f "$file" 2>/dev/null && {
                        DELETED_COUNT=$((DELETED_COUNT + 1))
                        SAVED_SPACE=$((SAVED_SPACE + file_size))
                        to_delete=$((to_delete - 1))
                    }
                fi
            done < <(find "$dir_name" -name "${base_pattern}.*" -type f -printf '%T@ %p\n' 2>/dev/null | sort -n | head -$to_delete | cut -d' ' -f2-)
        fi
        
        if [ $deleted -gt 0 ]; then
            log SUCCESS "清理了 $deleted 个过期日志文件"
        fi
    fi
}

# 轮转目录下的所有日志
rotate_directory() {
    local dir="$1"
    local max_size="$2"
    local max_files="$3"
    local max_age="$4"
    local should_compress="$5"
    
    if [ ! -d "$dir" ]; then
        log WARN "日志目录不存在: $dir"
        return 0
    fi
    
    log INFO "处理日志目录: $dir"
    
    # 查找所有日志文件
    for log_file in "$dir"/*.log; do
        [ -f "$log_file" ] || continue
        
        # 轮转
        rotate_file "$log_file" "$max_size" "$max_files" "$max_age" "$should_compress"
        
        # 清理
        local base_name=$(basename "$log_file")
        cleanup_old_logs "$log_file" "$max_files" "$max_age"
    done
    
    # 处理已压缩的日志清理
    for log_file in "$dir"/*.log.*; do
        [ -f "$log_file" ] || continue
        
        # 清理过期压缩日志
        local file_age=$((($(date +%s) - $(stat -c %Y "$log_file" 2>/dev/null || echo 0)) / 86400))
        if [ $file_age -gt $max_age ]; then
            if [ "$DRY_RUN" = true ]; then
                log DRYRUN "将删除过期文件: $log_file (${file_age}天)"
            else
                local file_size=$(get_file_size "$log_file")
                rm -f "$log_file" 2>/dev/null && {
                    DELETED_COUNT=$((DELETED_COUNT + 1))
                    SAVED_SPACE=$((SAVED_SPACE + file_size))
                    log SUCCESS "删除过期: $log_file"
                }
            fi
        fi
    done
}

# 轮转 systemd journal
rotate_journal() {
    if ! command -v journalctl &>/dev/null; then
        return 0
    fi
    
    log INFO "处理 systemd journal..."
    
    if [ "$DRY_RUN" = true ]; then
        log DRYRUN "将清理 journal 日志 (保留 ${MAX_AGE} 天)"
        return 0
    fi
    
    # 清理旧日志
    journalctl --vacuum-time="${MAX_AGE}d" 2>/dev/null || true
    
    # 限制最大大小
    journalctl --vacuum-size="500M" 2>/dev/null || true
}

# 轮转 Docker 日志
rotate_docker_logs() {
    if ! command -v docker &>/dev/null; then
        return 0
    fi
    
    log INFO "处理 Docker 容器日志..."
    
    if [ "$DRY_RUN" = true ]; then
        log DRYRUN "将清理 Docker 日志"
        return 0
    fi
    
    # 获取所有运行中的容器
    for container in $(docker ps -q 2>/dev/null); do
        local log_path=$(docker inspect --format='{{.LogPath}}' "$container" 2>/dev/null)
        
        if [ -n "$log_path" ] && [ -f "$log_path" ]; then
            local file_size=$(get_file_size "$log_path")
            local max_bytes=$(parse_size "$MAX_SIZE")
            
            if [ $file_size -gt $max_bytes ]; then
                log INFO "截断 Docker 日志: $container ($(format_size $file_size))"
                
                # 停止容器 -> 截断日志 -> 启动容器
                # 或者使用 truncate 命令
                if command -v truncate &>/dev/null; then
                    truncate -s 0 "$log_path" 2>/dev/null || true
                else
                    echo "" > "$log_path" 2>/dev/null || true
                fi
            fi
        fi
    done
}

# 轮转应用特定日志
rotate_app_logs() {
    # NAS-OS 服务日志
    local nas_log_dir="/var/log/nas-os"
    
    if [ -d "$nas_log_dir" ]; then
        # 主服务日志
        for log_file in nasd.log nasctl.log api.log; do
            if [ -f "${nas_log_dir}/${log_file}" ]; then
                rotate_file "${nas_log_dir}/${log_file}" "$MAX_SIZE" "$MAX_FILES" "$MAX_AGE" "$COMPRESS"
                cleanup_old_logs "${nas_log_dir}/${log_file}" "$MAX_FILES" "$MAX_AGE"
            fi
        done
        
        # 访问日志
        if [ -d "${nas_log_dir}/access" ]; then
            rotate_directory "${nas_log_dir}/access" "$MAX_SIZE" "$MAX_FILES" "$MAX_AGE" "$COMPRESS"
        fi
        
        # 审计日志
        if [ -d "${nas_log_dir}/audit" ]; then
            # 审计日志保留更长时间
            rotate_directory "${nas_log_dir}/audit" "$MAX_SIZE" "30" "90" "$COMPRESS"
        fi
        
        # 错误日志
        if [ -d "${nas_log_dir}/error" ]; then
            rotate_directory "${nas_log_dir}/error" "$MAX_SIZE" "$MAX_FILES" "$MAX_AGE" "$COMPRESS"
        fi
    fi
    
    # Web UI 日志
    local webui_log_dir="/var/log/nas-os/webui"
    if [ -d "$webui_log_dir" ]; then
        rotate_directory "$webui_log_dir" "$MAX_SIZE" "$MAX_FILES" "$MAX_AGE" "$COMPRESS"
    fi
}

#===========================================
# 状态与报告
#===========================================

# 显示日志状态
show_status() {
    echo "==================================="
    echo "NAS-OS 日志轮转状态 v${VERSION}"
    echo "==================================="
    echo ""
    
    # 配置信息
    echo "【配置】"
    echo "  最大文件大小: $MAX_SIZE"
    echo "  保留文件数:   $MAX_FILES"
    echo "  保留天数:     $MAX_AGE"
    echo "  压缩旧日志:   $COMPRESS"
    echo ""
    
    # 日志目录状态
    echo "【日志目录状态】"
    
    for dir in /var/log/nas-os /var/log/nas-os/access /var/log/nas-os/audit /var/log/nas-os/error /var/log/nas-os/webui; do
        if [ -d "$dir" ]; then
            local total_size=$(du -sh "$dir" 2>/dev/null | cut -f1)
            local file_count=$(find "$dir" -type f 2>/dev/null | wc -l)
            local compressed=$(find "$dir" -name "*.gz" -o -name "*.bz2" -o -name "*.xz" 2>/dev/null | wc -l)
            
            echo "  $dir"
            echo "    大小: $total_size"
            echo "    文件数: $file_count (压缩: $compressed)"
            
            # 显示最大的日志文件
            local largest=$(find "$dir" -name "*.log" -type f -exec du -h {} \; 2>/dev/null | sort -rh | head -3)
            if [ -n "$largest" ]; then
                echo "    最大文件:"
                echo "$largest" | while read size file; do
                    echo "      $size $(basename "$file")"
                done
            fi
            echo ""
        fi
    done
    
    # 状态文件
    if [ -f "$STATE_FILE" ]; then
        echo "【上次轮转】"
        cat "$STATE_FILE" 2>/dev/null || echo "  无记录"
        echo ""
    fi
    
    # Docker 日志
    if command -v docker &>/dev/null; then
        echo "【Docker 容器日志】"
        for container in $(docker ps -q 2>/dev/null); do
            local name=$(docker inspect --format='{{.Name}}' "$container" 2>/dev/null | sed 's/^\///')
            local log_path=$(docker inspect --format='{{.LogPath}}' "$container" 2>/dev/null)
            
            if [ -n "$log_path" ] && [ -f "$log_path" ]; then
                local size=$(du -sh "$log_path" 2>/dev/null | cut -f1)
                echo "  $name: $size"
            fi
        done
        echo ""
    fi
    
    # Journal 状态
    if command -v journalctl &>/dev/null; then
        echo "【Systemd Journal】"
        journalctl --disk-usage 2>/dev/null || echo "  无法获取"
        echo ""
    fi
}

# 保存状态
save_state() {
    mkdir -p "$(dirname "$STATE_FILE")" 2>/dev/null || true
    
    cat > "$STATE_FILE" <<EOF
{
    "last_run": "$(date -Iseconds)",
    "rotated_count": $ROTATED_COUNT,
    "compressed_count": $COMPRESSED_COUNT,
    "deleted_count": $DELETED_COUNT,
    "saved_space": $SAVED_SPACE
}
EOF
}

# 显示摘要
show_summary() {
    echo ""
    echo "==================================="
    echo "日志轮转摘要"
    echo "==================================="
    echo "  轮转文件数:   $ROTATED_COUNT"
    echo "  压缩文件数:   $COMPRESSED_COUNT"
    echo "  删除文件数:   $DELETED_COUNT"
    echo "  节省空间:     $(format_size $SAVED_SPACE)"
    echo "==================================="
}

#===========================================
# 配置文件
#===========================================

# 生成配置模板
generate_config() {
    local config_dir=$(dirname "$CONFIG_FILE")
    mkdir -p "$config_dir" 2>/dev/null
    
    cat > "$CONFIG_FILE" << 'EOF'
# NAS-OS 日志轮转配置
# 格式: 类似 logrotate

# 全局默认配置
MAX_SIZE=100M
MAX_FILES=10
MAX_AGE=30
COMPRESS=true

# 特定日志配置 (可选)
# 格式: 日志路径:最大大小:保留数量:保留天数:是否压缩

# 示例:
# /var/log/nas-os/nasd.log:50M:5:7:true
# /var/log/nas-os/access/*.log:200M:20:14:true
# /var/log/nas-os/audit/*.log:100M:30:90:true

# 排除的日志文件
# EXCLUDE_PATTERNS=(
#     "*.tmp"
#     "*.bak"
# )
EOF
    
    log SUCCESS "配置文件已生成: $CONFIG_FILE"
}

# 加载配置
load_config() {
    if [ -f "$CONFIG_FILE" ]; then
        source "$CONFIG_FILE"
        log INFO "加载配置: $CONFIG_FILE"
    fi
}

#===========================================
# 主入口
#===========================================

run_rotation() {
    local start_time=$(date +%s)
    
    log INFO "========== 开始日志轮转 =========="
    
    # 加载配置
    load_config
    
    # 执行预钩子
    run_hook "prerotate"
    
    # 轮转应用日志
    rotate_app_logs
    
    # 轮转 journal
    rotate_journal
    
    # 轮转 Docker 日志
    rotate_docker_logs
    
    # 执行后钩子
    run_hook "postrotate"
    
    # 保存状态
    if [ "$DRY_RUN" != true ]; then
        save_state
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    log INFO "========== 轮转完成 (耗时: ${duration}s) =========="
    
    # 显示摘要
    show_summary
}

# 帮助信息
show_help() {
    cat <<EOF
NAS-OS 日志轮转脚本 v${VERSION}

用法: $0 [命令] [选项]

命令:
  (默认)       执行日志轮转
  --status     显示日志状态
  --config     生成配置文件
  --dry-run    预览模式 (不实际执行)
  --force      强制轮转所有日志
  --help, -h   显示帮助

选项:
  --max-size SIZE    最大文件大小 (默认: 100M)
  --max-files N      保留文件数 (默认: 10)
  --max-age DAYS     保留天数 (默认: 30)
  --no-compress      不压缩旧日志

环境变量:
  MAX_SIZE           最大文件大小
  MAX_FILES          保留文件数
  MAX_AGE            保留天数
  COMPRESS           是否压缩
  CONFIG_FILE        配置文件路径
  DRY_RUN            预览模式

配置文件:
  $CONFIG_FILE

钩子目录:
  $HOOKS_DIR/
    prerotate.d/     轮转前执行
    postrotate.d/    轮转后执行

示例:
  $0                           # 执行日志轮转
  $0 --status                  # 查看状态
  $0 --dry-run                 # 预览模式
  $0 --force                   # 强制轮转
  $0 --max-size 50M --max-files 5  # 自定义参数

Cron 配置:
  # 每天凌晨 2 点执行日志轮转
  0 2 * * * /path/to/logrotate.sh >> /var/log/nas-os/logrotate.log 2>&1

EOF
}

# 解析参数
main() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --status)
                show_status
                exit 0
                ;;
            --config)
                generate_config
                exit 0
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --force)
                FORCE=true
                shift
                ;;
            --max-size)
                MAX_SIZE="$2"
                shift 2
                ;;
            --max-files)
                MAX_FILES="$2"
                shift 2
                ;;
            --max-age)
                MAX_AGE="$2"
                shift 2
                ;;
            --no-compress)
                COMPRESS=false
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
    
    # 执行轮转
    run_rotation
}

main "$@"