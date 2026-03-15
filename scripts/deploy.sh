#!/bin/bash
# NAS-OS 生产部署脚本
# 版本: v2.68.0
#
# 功能：
# - 服务管理（启动/停止/重启/状态检查）
# - 版本管理（备份当前版本）
# - 健康检查
# - 数据库备份
# - 滚动部署
# - 自动回滚
#
# 用法:
#   ./deploy.sh start                  # 启动服务
#   ./deploy.sh stop                   # 停止服务
#   ./deploy.sh restart                # 重启服务
#   ./deploy.sh status                 # 查看状态
#   ./deploy.sh [version] [options]    # 部署新版本
#   ./deploy.sh --dry-run              # 模拟部署
#   ./deploy.sh --rollback             # 回滚到上一版本

set -euo pipefail

# 版本
VERSION="2.68.0"

# ==================== 配置 ====================
APP_NAME="nas-os"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
BINARY_PATH="${BINARY_PATH:-/usr/local/bin/nasd}"
CONFIG_PATH="${CONFIG_PATH:-/etc/nas-os/config.yaml}"
SERVICE_NAME="${SERVICE_NAME:-nas-os}"

# 健康检查配置
HEALTH_CHECK_TIMEOUT="${HEALTH_CHECK_TIMEOUT:-120}"
HEALTH_CHECK_INTERVAL="${HEALTH_CHECK_INTERVAL:-5}"
HEALTH_CHECK_URL="${HEALTH_CHECK_URL:-http://localhost:8080/api/v1/health}"

# 部署配置
MAX_VERSIONS="${MAX_VERSIONS:-10}"
DEPLOY_TIMEOUT="${DEPLOY_TIMEOUT:-300}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ==================== 日志函数 ====================
log_info() { echo -e "${BLUE}[INFO]${NC} $(date '+%H:%M:%S') $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $(date '+%H:%M:%S') $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $(date '+%H:%M:%S') $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $(date '+%H:%M:%S') $1"; }
log_step() { echo -e "${CYAN}[STEP]${NC} $(date '+%H:%M:%S') $1"; }

# ==================== 显示帮助 ====================
show_help() {
    cat <<EOF
NAS-OS 生产部署工具 v${VERSION}

用法: $0 <command|version> [options]

服务管理命令:
  start           启动 NAS-OS 服务
  stop            停止 NAS-OS 服务
  restart         重启 NAS-OS 服务
  status          查看服务状态
  rollback        回滚到上一版本

部署命令:
  version         要部署的版本号 (如: v2.68.0)

选项:
  --dry-run       模拟部署，不实际执行
  --skip-backup   跳过数据库备份
  --skip-health   跳过健康检查
  --force         强制部署（忽略警告）
  --rollback      部署失败时自动回滚
  --no-start      只安装，不启动服务
  -h, --help      显示帮助

环境变量:
  DATA_DIR        数据目录 (默认: /var/lib/nas-os)
  BACKUP_DIR      备份目录 (默认: /var/lib/nas-os/backups)
  BINARY_PATH     二进制文件路径 (默认: /usr/local/bin/nasd)
  SERVICE_NAME    服务名称 (默认: nas-os)

示例:
  $0 start                        # 启动服务
  $0 stop                         # 停止服务
  $0 restart                      # 重启服务
  $0 status                       # 查看状态
  $0 v2.68.0                      # 部署 v2.68.0
  $0 v2.68.0 --rollback           # 部署失败时自动回滚
  $0 --dry-run                    # 模拟部署
  $0 rollback                     # 回滚到上一版本

EOF
}

# ==================== 参数解析 ====================
TARGET_VERSION=""
DRY_RUN=false
SKIP_BACKUP=false
SKIP_HEALTH=false
FORCE=false
AUTO_ROLLBACK=false
NO_START=false
COMMAND=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        start) COMMAND="start"; shift ;;
        stop) COMMAND="stop"; shift ;;
        restart) COMMAND="restart"; shift ;;
        status) COMMAND="status"; shift ;;
        rollback) COMMAND="rollback"; shift ;;
        --dry-run) DRY_RUN=true; shift ;;
        --skip-backup) SKIP_BACKUP=true; shift ;;
        --skip-health) SKIP_HEALTH=true; shift ;;
        --force) FORCE=true; shift ;;
        --rollback) AUTO_ROLLBACK=true; shift ;;
        --no-start) NO_START=true; shift ;;
        -h|--help) show_help; exit 0 ;;
        v*|V*) TARGET_VERSION="$1"; shift ;;
        *) log_error "未知参数: $1"; show_help; exit 1 ;;
    esac
done

# ==================== 工具函数 ====================

# 确保目录存在
ensure_dirs() {
    mkdir -p "$DATA_DIR"
    mkdir -p "$BACKUP_DIR"
    mkdir -p "$BACKUP_DIR/versions"
    mkdir -p "$BACKUP_DIR/db"
    mkdir -p "$LOG_DIR"
}

# 获取当前版本
get_current_version() {
    if [ -x "$BINARY_PATH" ]; then
        $BINARY_PATH version 2>/dev/null || echo "unknown"
    else
        echo "not installed"
    fi
}

# 检查服务状态
check_service_status() {
    if command -v systemctl &> /dev/null; then
        systemctl is-active "$SERVICE_NAME" 2>/dev/null || echo "inactive"
    elif pgrep -x nasd > /dev/null 2>&1; then
        echo "running"
    else
        echo "stopped"
    fi
}

# 等待服务就绪
wait_for_service() {
    log_info "等待服务就绪..."
    
    local start_time=$(date +%s)
    local end_time=$((start_time + HEALTH_CHECK_TIMEOUT))
    
    while [ $(date +%s) -lt $end_time ]; do
        # 检查进程
        if ! pgrep -x nasd > /dev/null 2>&1; then
            sleep $HEALTH_CHECK_INTERVAL
            continue
        fi
        
        # 检查健康端点
        if curl -sf --max-time 5 "$HEALTH_CHECK_URL" > /dev/null 2>&1; then
            log_success "服务已就绪"
            return 0
        fi
        
        local remaining=$((end_time - $(date +%s)))
        log_info "等待中... (剩余 ${remaining}s)"
        sleep $HEALTH_CHECK_INTERVAL
    done
    
    log_error "服务启动超时"
    return 1
}

# 健康检查
health_check() {
    log_step "执行健康检查..."
    
    # 检查进程
    if ! pgrep -x nasd > /dev/null 2>&1; then
        log_error "服务进程未运行"
        return 1
    fi
    
    # 检查 API
    local response
    response=$(curl -sf --max-time 10 "$HEALTH_CHECK_URL" 2>/dev/null)
    
    if [ $? -ne 0 ]; then
        log_error "健康端点不可达: $HEALTH_CHECK_URL"
        return 1
    fi
    
    # 解析健康状态
    if echo "$response" | grep -q '"status".*"healthy"'; then
        log_success "服务健康"
        return 0
    else
        log_warn "服务状态: $(echo "$response" | grep -o '"status"[^,]*' || echo 'unknown')"
        return 1
    fi
}

# 备份当前版本
backup_current_version() {
    local current_version=$(get_current_version)
    
    if [ ! -f "$BINARY_PATH" ]; then
        log_warn "当前无已安装版本"
        return 0
    fi
    
    ensure_dirs
    
    local backup_name="nasd-${current_version}"
    local backup_path="$BACKUP_DIR/versions/$backup_name"
    
    # 检查是否已备份
    if [ -f "$backup_path" ] && [ "$FORCE" != true ]; then
        log_info "版本已备份: $backup_name"
        return 0
    fi
    
    log_step "备份当前版本: $current_version"
    cp "$BINARY_PATH" "$backup_path"
    chmod +x "$backup_path"
    
    # 清理旧版本
    local count=$(ls -1 "$BACKUP_DIR/versions"/nasd-* 2>/dev/null | wc -l)
    if [ $count -gt $MAX_VERSIONS ]; then
        local to_delete=$((count - MAX_VERSIONS))
        log_info "清理 $to_delete 个旧备份..."
        ls -t "$BACKUP_DIR/versions"/nasd-* | tail -$to_delete | xargs rm -f
    fi
    
    log_success "版本备份完成: $backup_path"
}

# 备份数据库
backup_database() {
    log_step "备份数据库..."
    
    local db_path="$DATA_DIR/nas-os.db"
    
    if [ ! -f "$db_path" ]; then
        log_warn "数据库文件不存在: $db_path"
        return 0
    fi
    
    ensure_dirs
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local backup_name="nas-os-${timestamp}.db"
    local backup_path="$BACKUP_DIR/db/$backup_name"
    
    # 使用 SQLite 在线备份
    if command -v sqlite3 &> /dev/null; then
        sqlite3 "$db_path" ".backup '${backup_path}'" 2>/dev/null
        
        if [ $? -eq 0 ]; then
            gzip -f "$backup_path"
            backup_path="${backup_path}.gz"
            
            # 校验和
            local checksum=$(sha256sum "$backup_path" | cut -d' ' -f1)
            echo "$checksum  $(basename $backup_path)" > "${backup_path}.sha256"
            
            log_success "数据库备份完成: $backup_path"
            return 0
        fi
    fi
    
    # 简单复制作为后备
    cp "$db_path" "$backup_path"
    gzip -f "$backup_path"
    log_success "数据库备份完成（简单复制）: ${backup_path}.gz"
}

# 下载新版本
download_version() {
    local version="$1"
    local arch=$(uname -m)
    
    # 标准化架构名称
    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        armv7l|armhf) arch="armv7" ;;
    esac
    
    local binary_name="nasd-linux-${arch}"
    local download_url="${DOWNLOAD_URL:-https://github.com/example/nas-os/releases/download}/${version}/${binary_name}"
    local temp_file="/tmp/${binary_name}"
    
    log_step "下载版本 $version ($arch)..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[模拟] 下载: $download_url"
        return 0
    fi
    
    # 检查本地文件
    if [ -f "$BACKUP_DIR/versions/nasd-$version" ]; then
        log_info "使用本地备份: nasd-$version"
        cp "$BACKUP_DIR/versions/nasd-$version" "$temp_file"
        return 0
    fi
    
    # 下载
    if command -v curl &> /dev/null; then
        curl -fL --progress-bar -o "$temp_file" "$download_url"
    elif command -v wget &> /dev/null; then
        wget -q --show-progress -O "$temp_file" "$download_url"
    else
        log_error "需要 curl 或 wget 来下载"
        return 1
    fi
    
    if [ ! -f "$temp_file" ]; then
        log_error "下载失败"
        return 1
    fi
    
    chmod +x "$temp_file"
    log_success "下载完成: $temp_file"
}

# 安装新版本
install_version() {
    local version="$1"
    local arch=$(uname -m)
    
    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        armv7l|armhf) arch="armv7" ;;
    esac
    
    local temp_file="/tmp/nasd-linux-${arch}"
    
    if [ ! -f "$temp_file" ]; then
        # 检查本地备份
        if [ -f "$BACKUP_DIR/versions/nasd-$version" ]; then
            temp_file="$BACKUP_DIR/versions/nasd-$version"
        else
            log_error "找不到要安装的文件"
            return 1
        fi
    fi
    
    log_step "安装新版本..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[模拟] 安装: $temp_file -> $BINARY_PATH"
        return 0
    fi
    
    # 停止服务
    stop_service
    
    # 安装
    cp "$temp_file" "$BINARY_PATH"
    chmod +x "$BINARY_PATH"
    
    log_success "安装完成: $BINARY_PATH"
    
    # 验证
    local installed_version=$($BINARY_PATH version 2>/dev/null || echo "unknown")
    log_info "已安装版本: $installed_version"
}

# 停止服务
stop_service() {
    log_step "停止服务..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[模拟] 停止服务"
        return 0
    fi
    
    if command -v systemctl &> /dev/null; then
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    else
        pkill -x nasd 2>/dev/null || true
    fi
    
    sleep 2
    
    if pgrep -x nasd > /dev/null 2>&1; then
        log_warn "进程仍在运行，强制终止..."
        pkill -9 -x nasd 2>/dev/null || true
        sleep 1
    fi
    
    log_success "服务已停止"
}

# 启动服务
start_service() {
    log_step "启动服务..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[模拟] 启动服务"
        return 0
    fi
    
    if [ "$NO_START" = true ]; then
        log_info "跳过启动 (--no-start)"
        return 0
    fi
    
    if command -v systemctl &> /dev/null; then
        systemctl start "$SERVICE_NAME"
    else
        nohup $BINARY_PATH > "$LOG_DIR/nasd.log" 2>&1 &
    fi
    
    sleep 2
    
    # 等待服务就绪
    if ! wait_for_service; then
        return 1
    fi
    
    log_success "服务已启动"
}

# 执行回滚
do_rollback() {
    local previous_version="$1"
    
    log_warn "执行回滚到版本 $previous_version..."
    
    local backup_path="$BACKUP_DIR/versions/nasd-$previous_version"
    
    if [ ! -f "$backup_path" ]; then
        log_error "找不到备份版本: $backup_path"
        return 1
    fi
    
    # 停止服务
    stop_service
    
    # 恢复版本
    cp "$backup_path" "$BINARY_PATH"
    chmod +x "$BINARY_PATH"
    
    # 启动服务
    if ! start_service; then
        log_error "回滚后服务启动失败"
        return 1
    fi
    
    log_success "回滚完成: $previous_version"
}

# 记录部署日志
log_deployment() {
    local status="$1"
    local version="$2"
    local duration="$3"
    
    ensure_dirs
    
    local log_entry="$(date -Iseconds) | $status | $version | ${duration}s | $(hostname)"
    echo "$log_entry" >> "$LOG_DIR/deploy.log"
}

# ==================== 服务管理命令 ====================

# 启动服务命令
cmd_start() {
    echo ""
    echo "========================================"
    echo "  NAS-OS 服务启动"
    echo "========================================"
    echo ""
    
    local current_status=$(check_service_status)
    
    if [ "$current_status" = "active" ] || [ "$current_status" = "running" ]; then
        log_warn "服务已在运行中"
        cmd_status
        return 0
    fi
    
    log_step "启动 NAS-OS 服务..."
    
    if command -v systemctl &> /dev/null; then
        if ! systemctl start "$SERVICE_NAME" 2>&1; then
            log_error "systemctl 启动失败"
            return 1
        fi
    else
        if [ ! -x "$BINARY_PATH" ]; then
            log_error "找不到可执行文件: $BINARY_PATH"
            return 1
        fi
        ensure_dirs
        nohup $BINARY_PATH > "$LOG_DIR/nasd.log" 2>&1 &
    fi
    
    sleep 2
    
    if ! wait_for_service; then
        log_error "服务启动超时"
        return 1
    fi
    
    log_success "服务启动成功"
    cmd_status
}

# 停止服务命令
cmd_stop() {
    echo ""
    echo "========================================"
    echo "  NAS-OS 服务停止"
    echo "========================================"
    echo ""
    
    local current_status=$(check_service_status)
    
    if [ "$current_status" = "inactive" ] || [ "$current_status" = "stopped" ]; then
        log_warn "服务已停止"
        return 0
    fi
    
    log_step "停止 NAS-OS 服务..."
    
    if command -v systemctl &> /dev/null; then
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    else
        pkill -x nasd 2>/dev/null || true
    fi
    
    sleep 2
    
    # 检查是否仍在运行
    if pgrep -x nasd > /dev/null 2>&1; then
        log_warn "进程仍在运行，强制终止..."
        pkill -9 -x nasd 2>/dev/null || true
        sleep 1
    fi
    
    log_success "服务已停止"
}

# 重启服务命令
cmd_restart() {
    echo ""
    echo "========================================"
    echo "  NAS-OS 服务重启"
    echo "========================================"
    echo ""
    
    log_step "重启 NAS-OS 服务..."
    
    # 停止
    if command -v systemctl &> /dev/null; then
        systemctl restart "$SERVICE_NAME" 2>&1
    else
        pkill -x nasd 2>/dev/null || true
        sleep 2
        ensure_dirs
        nohup $BINARY_PATH > "$LOG_DIR/nasd.log" 2>&1 &
    fi
    
    sleep 2
    
    if ! wait_for_service; then
        log_error "服务重启失败"
        return 1
    fi
    
    log_success "服务重启成功"
    cmd_status
}

# 查看状态命令
cmd_status() {
    local current_version=$(get_current_version)
    local service_status=$(check_service_status)
    local pid=""
    local memory=""
    local cpu=""
    
    echo ""
    echo "========================================"
    echo "  NAS-OS 服务状态"
    echo "========================================"
    echo ""
    
    echo "  版本:    $current_version"
    echo "  状态:    $service_status"
    
    if [ "$service_status" = "active" ] || [ "$service_status" = "running" ]; then
        pid=$(pgrep -x nasd 2>/dev/null || echo "")
        
        if [ -n "$pid" ]; then
            echo "  PID:     $pid"
            
            # 获取资源使用
            if command -v ps &> /dev/null; then
                memory=$(ps -p "$pid" -o rss= 2>/dev/null | awk '{printf "%.1f MB", $1/1024}')
                cpu=$(ps -p "$pid" -o %cpu= 2>/dev/null | awk '{printf "%.1f%%", $1}')
                echo "  内存:    $memory"
                echo "  CPU:     $cpu"
            fi
        fi
        
        # 健康检查
        if curl -sf --max-time 5 "$HEALTH_CHECK_URL" > /dev/null 2>&1; then
            echo "  健康:    ✓ 正常"
        else
            echo "  健康:    ✗ 异常"
        fi
    fi
    
    echo ""
    
    # 显示最近日志
    if [ -f "$LOG_DIR/nasd.log" ]; then
        echo "  最近日志:"
        echo "  ----------------------------------------"
        tail -5 "$LOG_DIR/nasd.log" 2>/dev/null | sed 's/^/  /'
        echo ""
    fi
    
    echo "========================================"
    echo ""
    
    # 返回状态码
    case "$service_status" in
        active|running) return 0 ;;
        *) return 1 ;;
    esac
}

# 回滚命令
cmd_rollback() {
    echo ""
    echo "========================================"
    echo "  NAS-OS 版本回滚"
    echo "========================================"
    echo ""
    
    local current_version=$(get_current_version)
    
    log_info "当前版本: $current_version"
    
    # 列出可用备份
    if [ ! -d "$BACKUP_DIR/versions" ]; then
        log_error "备份目录不存在: $BACKUP_DIR/versions"
        return 1
    fi
    
    local backups=($(ls -t "$BACKUP_DIR/versions"/nasd-* 2>/dev/null))
    
    if [ ${#backups[@]} -eq 0 ]; then
        log_error "没有可用的备份版本"
        return 1
    fi
    
    echo "可用的备份版本:"
    echo ""
    local i=1
    for backup in "${backups[@]}"; do
        local version=$(basename "$backup" | sed 's/nasd-//')
        local mtime=$(stat -c %y "$backup" 2>/dev/null | cut -d'.' -f1)
        echo "  [$i] $version ($mtime)"
        ((i++))
    done
    echo ""
    
    # 选择版本
    local target_backup=""
    if [ -n "${TARGET_VERSION:-}" ]; then
        target_backup="$BACKUP_DIR/versions/nasd-$TARGET_VERSION"
        if [ ! -f "$target_backup" ]; then
            log_error "找不到指定版本: $TARGET_VERSION"
            return 1
        fi
    else
        # 默认回滚到上一个版本
        if [ ${#backups[@]} -ge 2 ]; then
            target_backup="${backups[1]}"
        else
            log_error "只有一个备份版本，无法回滚"
            return 1
        fi
    fi
    
    local target_version=$(basename "$target_backup" | sed 's/nasd-//')
    
    log_info "将回滚到版本: $target_version"
    
    # 执行回滚
    if do_rollback "$target_version"; then
        log_success "回滚成功"
        cmd_status
    else
        log_error "回滚失败"
        return 1
    fi
}

# ==================== 主入口 ====================

# 主入口
main() {
    local deploy_start=$(date +%s)
    local previous_version=$(get_current_version)
    
    echo ""
    echo "========================================"
    echo "  NAS-OS 部署工具 v${VERSION}"
    echo "========================================"
    echo ""
    
    # 显示部署信息
    log_info "当前版本: $previous_version"
    log_info "目标版本: ${TARGET_VERSION:-latest}"
    log_info "部署模式: $([ "$DRY_RUN" = true ] && echo '模拟' || echo '正式')"
    echo ""
    
    # 检查权限
    if [ "$DRY_RUN" != true ] && [ "$(id -u)" != "0" ]; then
        log_warn "建议使用 root 权限运行"
    fi
    
    # 确保目录存在
    ensure_dirs
    
    # 1. 备份当前版本
    if [ "$previous_version" != "not installed" ]; then
        backup_current_version
    fi
    
    # 2. 备份数据库
    if [ "$SKIP_BACKUP" != true ]; then
        if ! backup_database; then
            log_error "数据库备份失败"
            [ "$FORCE" = true ] || exit 1
            log_warn "强制模式：继续部署"
        fi
    fi
    
    # 3. 下载新版本
    if [ -n "$TARGET_VERSION" ]; then
        if ! download_version "$TARGET_VERSION"; then
            log_error "下载失败"
            exit 1
        fi
    fi
    
    # 模拟模式到此结束
    if [ "$DRY_RUN" = true ]; then
        echo ""
        log_info "模拟部署完成"
        exit 0
    fi
    
    # 4. 安装新版本
    if ! install_version "$TARGET_VERSION"; then
        log_error "安装失败"
        exit 1
    fi
    
    # 5. 启动服务
    if ! start_service; then
        log_error "服务启动失败"
        
        # 自动回滚
        if [ "$AUTO_ROLLBACK" = true ] && [ "$previous_version" != "not installed" ]; then
            do_rollback "$previous_version"
        fi
        
        exit 1
    fi
    
    # 6. 健康检查
    if [ "$SKIP_HEALTH" != true ]; then
        if ! health_check; then
            log_error "健康检查失败"
            
            # 自动回滚
            if [ "$AUTO_ROLLBACK" = true ] && [ "$previous_version" != "not installed" ]; then
                do_rollback "$previous_version"
            fi
            
            exit 1
        fi
    fi
    
    # 计算耗时
    local deploy_end=$(date +%s)
    local duration=$((deploy_end - deploy_start))
    
    # 记录部署日志
    log_deployment "success" "${TARGET_VERSION:-latest}" "$duration"
    
    # 显示结果
    echo ""
    echo "========================================"
    log_success "部署完成！"
    echo ""
    echo "  版本: $previous_version -> ${TARGET_VERSION:-latest}"
    echo "  耗时: ${duration}s"
    echo "  状态: $(check_service_status)"
    echo "========================================"
    echo ""
}

# ==================== 入口分发 ====================

# 检查并执行命令
case "${COMMAND:-}" in
    start)
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    status)
        cmd_status
        ;;
    rollback)
        cmd_rollback
        ;;
    "")
        # 无子命令，执行部署
        main
        ;;
    *)
        log_error "未知命令: $COMMAND"
        show_help
        exit 1
        ;;
esac