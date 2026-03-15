#!/bin/bash
# NAS-OS 部署回滚脚本
# 支持版本回滚、数据库备份检查、健康检查
#
# v2.45.0 新增（工部）

set -e

# 版本
VERSION="2.45.0"

# 配置
APP_NAME="nas-os"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
BINARY_PATH="${BINARY_PATH:-/usr/local/bin/nasd}"
SERVICE_NAME="${SERVICE_NAME:-nas-os}"

# 回滚配置
MAX_VERSIONS="${MAX_VERSIONS:-10}"
HEALTH_CHECK_TIMEOUT="${HEALTH_CHECK_TIMEOUT:-60}"
HEALTH_CHECK_INTERVAL="${HEALTH_CHECK_INTERVAL:-5}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 日志函数
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 显示帮助
show_help() {
    cat <<EOF
NAS-OS 部署回滚工具 v${VERSION}

用法: $0 <command> [options]

命令:
  list              列出可回滚版本
  rollback [ver]    回滚到指定版本（默认上一个版本）
  backup            回滚前备份数据库
  status            显示当前版本状态
  history           显示回滚历史
  help              显示帮助

选项:
  --skip-backup     跳过数据库备份
  --skip-health     跳过健康检查
  --force           强制回滚（忽略警告）
  --dry-run         仅模拟，不实际执行

环境变量:
  DATA_DIR          数据目录 (默认: /var/lib/nas-os)
  BACKUP_DIR        备份目录 (默认: /var/lib/nas-os/backups)
  BINARY_PATH       二进制文件路径 (默认: /usr/local/bin/nasd)

示例:
  $0 list                    # 列出可回滚版本
  $0 rollback                # 回滚到上一版本
  $0 rollback v2.43.0        # 回滚到指定版本
  $0 rollback --skip-backup  # 跳过备份直接回滚
  $0 status                  # 显示当前状态

EOF
}

# 确保备份目录存在
ensure_backup_dir() {
    mkdir -p "$BACKUP_DIR"
    mkdir -p "$BACKUP_DIR/versions"
    mkdir -p "$BACKUP_DIR/rollback-logs"
}

# 获取当前版本
get_current_version() {
    if [ -x "$BINARY_PATH" ]; then
        $BINARY_PATH version 2>/dev/null || echo "unknown"
    else
        echo "not installed"
    fi
}

# 列出可回滚版本
list_versions() {
    log_info "可回滚版本列表:"
    echo ""
    
    local current=$(get_current_version)
    
    if [ -d "$BACKUP_DIR/versions" ]; then
        local count=0
        for ver_file in $(ls -t "$BACKUP_DIR/versions"/nasd-* 2>/dev/null | head -$MAX_VERSIONS); do
            local ver=$(basename "$ver_file" | sed 's/nasd-//')
            local date=$(stat -c %y "$ver_file" 2>/dev/null | cut -d. -f1)
            local size=$(du -h "$ver_file" | cut -f1)
            
            if [ "$ver" = "$current" ]; then
                echo "  * $ver  ($date, $size) [当前]"
            else
                echo "    $ver  ($date, $size)"
            fi
            count=$((count + 1))
        done
        
        if [ $count -eq 0 ]; then
            log_warn "没有找到备份版本"
            log_info "提示: 在部署新版本前，当前版本会自动备份"
        fi
    else
        log_warn "版本备份目录不存在"
    fi
}

# 备份数据库
backup_database() {
    log_info "备份数据库..."
    
    local db_path="$DATA_DIR/nas-os.db"
    
    if [ ! -f "$db_path" ]; then
        log_warn "数据库文件不存在: $db_path"
        return 0
    fi
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local backup_name="rollback-backup-${timestamp}.db"
    local backup_path="$BACKUP_DIR/$backup_name"
    
    ensure_backup_dir
    
    # 使用 SQLite 在线备份
    if command -v sqlite3 &> /dev/null; then
        sqlite3 "$db_path" ".backup '${backup_path}'" 2>/dev/null
        
        if [ $? -eq 0 ]; then
            # 压缩
            gzip -f "$backup_path"
            backup_path="${backup_path}.gz"
            
            # 校验和
            local checksum=$(sha256sum "$backup_path" | cut -d' ' -f1)
            echo "$checksum  $(basename $backup_path)" > "${backup_path}.sha256"
            
            log_success "数据库备份完成: $backup_path"
            
            # 记录
            echo "$(date -Iseconds) BACKUP $backup_path" >> "$BACKUP_DIR/rollback-logs/rollback.log"
            return 0
        else
            log_error "数据库备份失败"
            return 1
        fi
    else
        # 简单复制
        cp "$db_path" "$backup_path"
        gzip -f "$backup_path"
        log_success "数据库备份完成（简单复制）: ${backup_path}.gz"
        return 0
    fi
}

# 健康检查
health_check() {
    log_info "执行健康检查..."
    
    local start_time=$(date +%s)
    local end_time=$((start_time + HEALTH_CHECK_TIMEOUT))
    
    while [ $(date +%s) -lt $end_time ]; do
        # 检查进程
        if pgrep -x nasd > /dev/null 2>&1; then
            # 检查 API
            if curl -sf --max-time 5 "http://localhost:8080/api/v1/health" > /dev/null 2>&1; then
                log_success "服务健康"
                return 0
            fi
        fi
        
        log_info "等待服务启动... ($((end_time - $(date +%s)))s)"
        sleep $HEALTH_CHECK_INTERVAL
    done
    
    log_error "健康检查超时"
    return 1
}

# 回滚到指定版本
do_rollback() {
    local target_version=""
    local skip_backup=false
    local skip_health=false
    local force=false
    local dry_run=false
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-backup) skip_backup=true; shift ;;
            --skip-health) skip_health=true; shift ;;
            --force) force=true; shift ;;
            --dry-run) dry_run=true; shift ;;
            v*|V*) target_version="$1"; shift ;;
            *) shift ;;
        esac
    done
    
    local current_version=$(get_current_version)
    
    if [ -z "$target_version" ]; then
        # 自动选择上一版本
        if [ -d "$BACKUP_DIR/versions" ]; then
            for ver_file in $(ls -t "$BACKUP_DIR/versions"/nasd-* 2>/dev/null); do
                local ver=$(basename "$ver_file" | sed 's/nasd-//')
                if [ "$ver" != "$current_version" ]; then
                    target_version="$ver"
                    break
                fi
            done
        fi
        
        if [ -z "$target_version" ]; then
            log_error "找不到可回滚的版本"
            return 1
        fi
    fi
    
    log_info "当前版本: $current_version"
    log_info "目标版本: $target_version"
    
    if [ "$target_version" = "$current_version" ]; then
        log_warn "目标版本与当前版本相同"
        [ "$force" = true ] || return 0
    fi
    
    # 检查目标版本是否存在
    local target_binary="$BACKUP_DIR/versions/nasd-$target_version"
    if [ ! -f "$target_binary" ]; then
        log_error "目标版本文件不存在: $target_binary"
        return 1
    fi
    
    # 模拟模式
    if [ "$dry_run" = true ]; then
        log_info "[模拟] 将执行以下操作:"
        echo "  1. 停止服务"
        echo "  2. 备份当前数据库"
        echo "  3. 替换二进制文件: $target_binary -> $BINARY_PATH"
        echo "  4. 启动服务"
        echo "  5. 执行健康检查"
        return 0
    fi
    
    # 确认
    echo ""
    log_warn "即将回滚到版本 $target_version"
    read -p "确认继续? [y/N] " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "已取消"
        return 0
    fi
    
    local rollback_start=$(date +%s)
    
    # 1. 停止服务
    log_info "停止服务..."
    if command -v systemctl &> /dev/null; then
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    else
        pkill -x nasd 2>/dev/null || true
    fi
    sleep 2
    
    # 2. 备份数据库
    if [ "$skip_backup" = false ]; then
        if ! backup_database; then
            log_error "数据库备份失败，中止回滚"
            [ "$force" = true ] || return 1
            log_warn "强制模式：继续回滚"
        fi
    fi
    
    # 3. 备份当前版本
    if [ -f "$BINARY_PATH" ]; then
        local current_backup="$BACKUP_DIR/versions/nasd-$current_version"
        cp "$BINARY_PATH" "$current_backup" 2>/dev/null || true
    fi
    
    # 4. 替换二进制文件
    log_info "替换二进制文件..."
    cp "$target_binary" "$BINARY_PATH"
    chmod +x "$BINARY_PATH"
    
    # 5. 启动服务
    log_info "启动服务..."
    if command -v systemctl &> /dev/null; then
        systemctl start "$SERVICE_NAME"
    else
        nohup $BINARY_PATH > /dev/null 2>&1 &
    fi
    
    # 6. 健康检查
    if [ "$skip_health" = false ]; then
        if ! health_check; then
            log_error "健康检查失败，尝试恢复..."
            # 恢复原版本
            cp "$BACKUP_DIR/versions/nasd-$current_version" "$BINARY_PATH" 2>/dev/null || true
            if command -v systemctl &> /dev/null; then
                systemctl restart "$SERVICE_NAME"
            fi
            log_error "回滚失败，已恢复原版本"
            return 1
        fi
    fi
    
    local rollback_end=$(date +%s)
    local duration=$((rollback_end - rollback_start))
    
    log_success "回滚完成: $current_version -> $target_version (耗时: ${duration}s)"
    
    # 记录日志
    ensure_backup_dir
    echo "$(date -Iseconds) ROLLBACK $current_version -> $target_version (${duration}s)" >> "$BACKUP_DIR/rollback-logs/rollback.log"
    
    return 0
}

# 显示状态
show_status() {
    local current=$(get_current_version)
    
    echo ""
    echo "==================================="
    echo "NAS-OS 状态"
    echo "==================================="
    echo ""
    
    echo "当前版本: $current"
    
    # 检查服务状态
    if pgrep -x nasd > /dev/null 2>&1; then
        local pid=$(pgrep -x nasd)
        local mem=$(ps -o rss= -p $pid 2>/dev/null | awk '{printf "%.1f MB", $1/1024}')
        local cpu=$(ps -o %cpu= -p $pid 2>/dev/null | awk '{printf "%.1f%%", $1}')
        echo "进程状态: 运行中 (PID: $pid)"
        echo "内存使用: $mem"
        echo "CPU 使用: $cpu"
    else
        echo "进程状态: 未运行"
    fi
    
    # 数据库
    local db_path="$DATA_DIR/nas-os.db"
    if [ -f "$db_path" ]; then
        local db_size=$(du -h "$db_path" 2>/dev/null | cut -f1)
        echo "数据库: $db_size"
    fi
    
    # 备份
    echo ""
    echo "备份版本数: $(ls "$BACKUP_DIR/versions"/nasd-* 2>/dev/null | wc -l)"
    echo "数据库备份: $(ls "$BACKUP_DIR"/rollback-backup-*.db.gz 2>/dev/null | wc -l)"
}

# 显示回滚历史
show_history() {
    local log_file="$BACKUP_DIR/rollback-logs/rollback.log"
    
    if [ -f "$log_file" ]; then
        echo ""
        echo "回滚历史:"
        echo ""
        tail -20 "$log_file"
    else
        log_info "暂无回滚历史"
    fi
}

# 主入口
CMD="${1:-}"
shift || true

case "$CMD" in
    list)
        list_versions
        ;;
    rollback)
        do_rollback "$@"
        ;;
    backup)
        backup_database
        ;;
    status)
        show_status
        ;;
    history)
        show_history
        ;;
    help|-h|--help)
        show_help
        ;;
    "")
        show_help
        ;;
    *)
        log_error "未知命令: $CMD"
        show_help
        exit 1
        ;;
esac