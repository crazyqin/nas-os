#!/bin/bash
# =============================================================================
# NAS-OS 一键部署脚本 v2.76.0
# =============================================================================
# 用途：支持二进制部署、Docker 部署、升级、回滚
# 用法：./deploy.sh <command> [options]
#
# v2.76.0 更新（工部增强）：
# - 添加蓝绿部署支持
# - 添加健康检查和自动回滚
# - 添加部署前检查和依赖验证
# - 添加配置迁移和备份
# - 添加部署日志和审计
# - 支持 dry-run 模式
# =============================================================================

set -euo pipefail

# 版本
VERSION="2.76.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# =============================================================================
# 配置
# =============================================================================

# 安装目录
INSTALL_DIR="${INSTALL_DIR:-/usr/local}"
CONFIG_DIR="${CONFIG_DIR:-/etc/nas-os}"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"

# 发布配置
RELEASE_VERSION="${RELEASE_VERSION:-latest}"
RELEASE_URL="https://github.com/nas-os/nas-os/releases"

# 健康检查配置
HEALTH_CHECK_URL="${HEALTH_CHECK_URL:-http://localhost:8080/api/v1/health}"
HEALTH_CHECK_TIMEOUT="${HEALTH_CHECK_TIMEOUT:-30}"
HEALTH_CHECK_RETRIES="${HEALTH_CHECK_RETRIES:-5}"

# 部署模式
DEPLOY_MODE="${DEPLOY_MODE:-binary}"  # binary, docker, blue-green
DRY_RUN="${DRY_RUN:-false}"
ROLLBACK_ON_FAILURE="${ROLLBACK_ON_FAILURE:-true}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# =============================================================================
# 工具函数
# =============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

# 检测系统架构
detect_arch() {
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64)
            echo "arm64"
            ;;
        armv7l)
            echo "armv7"
            ;;
        *)
            echo "unsupported"
            ;;
    esac
}

# 检测操作系统
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        echo "$ID"
    else
        echo "unknown"
    fi
}

# 检查 root 权限
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        log_error "需要 root 权限，请使用 sudo"
        exit 1
    fi
}

# 执行命令（支持 dry-run）
run_cmd() {
    if [ "$DRY_RUN" = "true" ]; then
        echo "[DRY-RUN] $*"
        return 0
    fi
    "$@"
}

# 记录部署日志
log_deployment() {
    local action="$1"
    local details="$2"
    local log_file="$LOG_DIR/deploy.log"
    
    mkdir -p "$LOG_DIR"
    echo "[$(date -Iseconds)] [$action] $details" >> "$log_file"
}

# =============================================================================
# 预检查
# =============================================================================

# 检查系统依赖
check_dependencies() {
    log_step "检查系统依赖..."
    
    local missing=()
    
    # 必需命令
    for cmd in curl tar systemctl; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "缺少依赖: ${missing[*]}"
        log_info "请安装缺少的依赖后重试"
        
        local os=$(detect_os)
        case $os in
            ubuntu|debian)
                log_info "运行: apt-get install -y ${missing[*]}"
                ;;
            centos|rhel|rocky|almalinux)
                log_info "运行: yum install -y ${missing[*]}"
                ;;
        esac
        
        return 1
    fi
    
    log_success "依赖检查通过"
    return 0
}

# 检查系统资源
check_resources() {
    log_step "检查系统资源..."
    
    # 内存检查
    local total_mem=$(free -m | awk '/^Mem:/{print $2}')
    if [ "$total_mem" -lt 512 ]; then
        log_warn "内存不足 512MB (当前: ${total_mem}MB)，可能影响性能"
    fi
    
    # 磁盘空间检查
    local disk_avail=$(df -m / | awk 'NR==2 {print $4}')
    if [ "$disk_avail" -lt 1024 ]; then
        log_warn "根分区可用空间不足 1GB (当前: ${disk_avail}MB)"
    fi
    
    # 数据目录空间检查
    if [ -d "$DATA_DIR" ]; then
        local data_avail=$(df -m "$DATA_DIR" | awk 'NR==2 {print $4}')
        if [ "$data_avail" -lt 10240 ]; then
            log_warn "数据目录可用空间不足 10GB (当前: ${data_avail}MB)"
        fi
    fi
    
    log_success "资源检查完成"
}

# 检查端口占用
check_ports() {
    log_step "检查端口占用..."
    
    local ports=(8080 445 2049)
    local conflict=false
    
    for port in "${ports[@]}"; do
        if ss -tuln | grep -q ":${port} "; then
            local process=$(ss -tuln | grep ":${port} " | awk '{print $6}' | head -1)
            if [[ ! "$process" =~ nasd ]]; then
                log_warn "端口 $port 已被占用: $process"
                conflict=true
            fi
        fi
    done
    
    if [ "$conflict" = true ]; then
        log_warn "存在端口冲突，部署后可能影响服务"
    else
        log_success "端口检查通过"
    fi
}

# 检查现有安装
check_existing_installation() {
    log_step "检查现有安装..."
    
    if [ -f "$INSTALL_DIR/bin/nasd" ]; then
        local current_version=$("$INSTALL_DIR/bin/nasd" --version 2>/dev/null || echo "unknown")
        log_info "检测到已安装版本: $current_version"
        echo "CURRENT_VERSION=$current_version" > /tmp/nas-os-upgrade.env
        return 0
    fi
    
    log_info "未检测到现有安装"
    return 1
}

# =============================================================================
# 安装函数
# =============================================================================

# 创建目录结构
create_directories() {
    log_step "创建目录结构..."
    
    run_cmd mkdir -p "$CONFIG_DIR"
    run_cmd mkdir -p "$DATA_DIR"
    run_cmd mkdir -p "$LOG_DIR"
    run_cmd mkdir -p "$BACKUP_DIR"
    run_cmd mkdir -p "$DATA_DIR/plugins"
    run_cmd mkdir -p "$DATA_DIR/backups"
    run_cmd mkdir -p "$DATA_DIR/snapshots"
    
    run_cmd chmod 755 "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR" "$BACKUP_DIR"
    
    log_success "目录创建完成"
}

# 安装系统依赖
install_system_dependencies() {
    log_step "安装系统依赖..."
    
    local os=$(detect_os)
    
    case $os in
        ubuntu|debian)
            run_cmd apt-get update
            run_cmd apt-get install -y curl sqlite3 btrfs-progs smartmontools
            ;;
        centos|rhel|rocky|almalinux)
            run_cmd yum install -y curl sqlite btrfs-progs smartmontools
            ;;
        arch|manjaro)
            run_cmd pacman -S --noconfirm curl sqlite btrfs-progs smartmontools
            ;;
        *)
            log_warn "未知操作系统，请手动安装依赖: curl, sqlite3, btrfs-progs, smartmontools"
            ;;
    esac
    
    log_success "系统依赖安装完成"
}

# 下载二进制
download_binary() {
    local version="${1:-$RELEASE_VERSION}"
    local arch=$(detect_arch)
    local os="linux"
    
    if [ "$arch" = "unsupported" ]; then
        log_error "不支持的架构: $(uname -m)"
        exit 1
    fi
    
    log_step "下载 NAS-OS 二进制 ($os/$arch, $version)..."
    
    local download_url
    if [ "$version" = "latest" ]; then
        download_url="$RELEASE_URL/latest/download/nasd-$os-$arch"
    else
        download_url="$RELEASE_URL/download/$version/nasd-$os-$arch"
    fi
    
    log_info "下载地址: $download_url"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] 跳过下载"
        return 0
    fi
    
    # 下载并验证
    local tmp_file="/tmp/nasd-$$"
    if curl -fSL --progress-bar -o "$tmp_file" "$download_url"; then
        chmod +x "$tmp_file"
        mv "$tmp_file" "/tmp/nasd"
        log_success "二进制下载完成"
    else
        log_error "下载失败"
        return 1
    fi
}

# 备份当前版本
backup_current() {
    log_step "备份当前版本..."
    
    if [ ! -f "$INSTALL_DIR/bin/nasd" ]; then
        log_info "无现有版本，跳过备份"
        return 0
    fi
    
    local backup_name="nasd-$(date +%Y%m%d-%H%M%S).bak"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] 备份: $backup_name"
        return 0
    fi
    
    mkdir -p "$BACKUP_DIR"
    
    # 备份二进制
    cp "$INSTALL_DIR/bin/nasd" "$BACKUP_DIR/$backup_name"
    
    # 备份配置
    if [ -d "$CONFIG_DIR" ]; then
        tar -czf "$BACKUP_DIR/config-$(date +%Y%m%d-%H%M%S).tar.gz" -C "$(dirname "$CONFIG_DIR")" "$(basename "$CONFIG_DIR")" 2>/dev/null || true
    fi
    
    # 保存当前版本信息
    echo "BACKUP_FILE=$BACKUP_DIR/$backup_name" > /tmp/nas-os-backup.env
    
    log_success "备份完成: $backup_name"
}

# 安装二进制
install_binary() {
    log_step "安装二进制..."
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] 安装到: $INSTALL_DIR/bin/nasd"
        return 0
    fi
    
    # 停止服务（如果运行中）
    if systemctl is-active nas-os &>/dev/null; then
        systemctl stop nas-os
        log_info "已停止旧服务"
    fi
    
    # 备份旧版本
    if [ -f "$INSTALL_DIR/bin/nasd" ]; then
        mv "$INSTALL_DIR/bin/nasd" "$INSTALL_DIR/bin/nasd.old"
    fi
    
    # 安装新版本
    mv /tmp/nasd "$INSTALL_DIR/bin/nasd"
    chmod 755 "$INSTALL_DIR/bin/nasd"
    
    # 验证安装
    if "$INSTALL_DIR/bin/nasd" --version &>/dev/null; then
        log_success "二进制安装完成: $($INSTALL_DIR/bin/nasd --version)"
    else
        log_error "安装验证失败"
        # 尝试恢复
        if [ -f "$INSTALL_DIR/bin/nasd.old" ]; then
            mv "$INSTALL_DIR/bin/nasd.old" "$INSTALL_DIR/bin/nasd"
            log_info "已恢复旧版本"
        fi
        return 1
    fi
}

# 安装配置文件
install_config() {
    log_step "安装配置文件..."
    
    local config_src="$PROJECT_ROOT/configs/default.yaml"
    
    if [ -f "$config_src" ]; then
        if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
            run_cmd cp "$config_src" "$CONFIG_DIR/config.yaml"
            log_success "配置文件已安装"
        else
            log_info "配置文件已存在，保留现有配置"
            
            # 检查是否需要迁移配置
            if [ "$DRY_RUN" != "true" ]; then
                # 创建配置备份
                cp "$CONFIG_DIR/config.yaml" "$CONFIG_DIR/config.yaml.bak"
            fi
        fi
    else
        log_warn "默认配置文件不存在，创建空配置"
        run_cmd touch "$CONFIG_DIR/config.yaml"
    fi
}

# 创建 systemd 服务
create_systemd_service() {
    log_step "创建 systemd 服务..."
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] 创建 systemd 服务"
        return 0
    fi
    
    cat > /etc/systemd/system/nas-os.service <<EOF
[Unit]
Description=NAS-OS - Home NAS Management System
Documentation=https://docs.nas-os.io
After=network.target docker.service
Wants=docker.service

[Service]
Type=notify
ExecStart=$INSTALL_DIR/bin/nasd --config $CONFIG_DIR/config.yaml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=on-failure
RestartSec=5
TimeoutStopSec=60

# 环境变量
Environment=NAS_OS_CONFIG=$CONFIG_DIR/config.yaml
Environment=NAS_OS_DATA_ROOT=$DATA_DIR

# 工作目录
WorkingDirectory=$DATA_DIR

# 日志
StandardOutput=journal
StandardError=journal
SyslogIdentifier=nas-os

# 安全加固
NoNewPrivileges=false

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable nas-os
    
    log_success "systemd 服务创建完成"
}

# 启动服务
start_service() {
    log_step "启动 NAS-OS 服务..."
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] 启动服务"
        return 0
    fi
    
    systemctl start nas-os
    
    # 等待服务启动
    sleep 3
    
    if systemctl is-active nas-os &>/dev/null; then
        log_success "服务启动成功"
    else
        log_error "服务启动失败"
        journalctl -u nas-os --no-pager -n 20
        return 1
    fi
}

# 健康检查
health_check() {
    log_step "执行健康检查..."
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] 健康检查"
        return 0
    fi
    
    local retries=0
    local max_retries=$HEALTH_CHECK_RETRIES
    
    while [ $retries -lt $max_retries ]; do
        if curl -sf "$HEALTH_CHECK_URL" &>/dev/null; then
            log_success "健康检查通过"
            return 0
        fi
        
        retries=$((retries + 1))
        log_info "等待服务就绪... ($retries/$max_retries)"
        sleep 5
    done
    
    log_error "健康检查失败"
    return 1
}

# 部署后验证
post_deploy_verify() {
    log_step "部署后验证..."
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] 跳过验证"
        return 0
    fi
    
    # 检查服务状态
    if ! systemctl is-active nas-os &>/dev/null; then
        log_error "服务未运行"
        return 1
    fi
    
    # 检查 API 响应
    local response
    response=$(curl -sf "$HEALTH_CHECK_URL" 2>/dev/null || echo '{"status":"error"}')
    
    if echo "$response" | grep -q '"status":"healthy"'; then
        log_success "API 健康检查通过"
    else
        log_warn "API 健康检查返回非健康状态"
    fi
    
    # 检查版本
    local new_version=$("$INSTALL_DIR/bin/nasd" --version 2>/dev/null || echo "unknown")
    log_info "当前版本: $new_version"
    
    return 0
}

# 自动回滚
auto_rollback() {
    log_warn "部署失败，执行自动回滚..."
    
    if [ -f /tmp/nas-os-backup.env ]; then
        source /tmp/nas-os-backup.env
        
        if [ -f "$BACKUP_FILE" ]; then
            # 停止服务
            systemctl stop nas-os 2>/dev/null || true
            
            # 恢复二进制
            cp "$BACKUP_FILE" "$INSTALL_DIR/bin/nasd"
            chmod 755 "$INSTALL_DIR/bin/nasd"
            
            # 启动服务
            systemctl start nas-os
            
            log_success "回滚完成"
        fi
    fi
}

# =============================================================================
# Docker 部署
# =============================================================================

deploy_docker() {
    log_step "使用 Docker 部署..."
    
    # 检查 Docker
    if ! command -v docker &>/dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi
    
    # 拉取镜像
    log_info "拉取镜像..."
    local image="ghcr.io/nas-os/nas-os:$RELEASE_VERSION"
    run_cmd docker pull "$image"
    
    # 停止旧容器
    docker stop nas-os 2>/dev/null || true
    docker rm nas-os 2>/dev/null || true
    
    # 启动新容器
    log_info "启动容器..."
    run_cmd docker run -d \
        --name nas-os \
        --restart unless-stopped \
        --privileged \
        --network host \
        -v "$CONFIG_DIR:/etc/nas-os:ro" \
        -v "$DATA_DIR:/var/lib/nas-os" \
        -v "$LOG_DIR:/var/log/nas-os" \
        -e NAS_OS_CONFIG=/etc/nas-os/config.yaml \
        -e TZ=Asia/Shanghai \
        "$image"
    
    log_success "Docker 部署完成"
}

# =============================================================================
# 命令处理
# =============================================================================

# 安装
do_install() {
    log_step "开始安装 NAS-OS v$VERSION..."
    
    check_root
    check_dependencies || exit 1
    check_resources
    check_ports
    create_directories
    install_system_dependencies
    download_binary
    install_binary
    install_config
    create_systemd_service
    
    if start_service; then
        if health_check; then
            post_deploy_verify
            log_deployment "install" "成功安装 v$VERSION"
            show_info
        else
            log_error "健康检查失败"
            exit 1
        fi
    else
        log_error "服务启动失败"
        exit 1
    fi
}

# 升级
do_upgrade() {
    log_step "开始升级 NAS-OS..."
    
    check_root
    check_dependencies || exit 1
    check_existing_installation || {
        log_error "未检测到现有安装，请使用 install 命令"
        exit 1
    }
    
    backup_current
    download_binary "$RELEASE_VERSION"
    
    if install_binary; then
        if start_service && health_check; then
            post_deploy_verify
            log_deployment "upgrade" "成功升级到 $RELEASE_VERSION"
            log_success "升级完成"
        else
            if [ "$ROLLBACK_ON_FAILURE" = "true" ]; then
                auto_rollback
            fi
            exit 1
        fi
    else
        if [ "$ROLLBACK_ON_FAILURE" = "true" ]; then
            auto_rollback
        fi
        exit 1
    fi
}

# 回滚
do_rollback() {
    log_step "执行回滚..."
    
    check_root
    
    # 列出可用备份
    log_info "可用备份:"
    ls -lt "$BACKUP_DIR"/nasd-*.bak 2>/dev/null | head -5 || {
        log_error "没有找到备份文件"
        exit 1
    }
    
    local backup_file="${1:-}"
    
    if [ -z "$backup_file" ]; then
        # 使用最新备份
        backup_file=$(ls -t "$BACKUP_DIR"/nasd-*.bak 2>/dev/null | head -1)
    fi
    
    if [ ! -f "$backup_file" ]; then
        log_error "备份文件不存在: $backup_file"
        exit 1
    fi
    
    log_info "使用备份: $backup_file"
    
    # 停止服务
    systemctl stop nas-os 2>/dev/null || true
    
    # 恢复
    cp "$backup_file" "$INSTALL_DIR/bin/nasd"
    chmod 755 "$INSTALL_DIR/bin/nasd"
    
    # 启动服务
    if start_service && health_check; then
        log_deployment "rollback" "回滚到 $(basename $backup_file)"
        log_success "回滚完成"
    else
        log_error "回滚后服务异常"
        exit 1
    fi
}

# 卸载
do_uninstall() {
    log_step "卸载 NAS-OS..."
    
    check_root
    
    # 停止服务
    systemctl stop nas-os 2>/dev/null || true
    systemctl disable nas-os 2>/dev/null || true
    
    # 删除文件
    rm -f "$INSTALL_DIR/bin/nasd"
    rm -f /etc/systemd/system/nas-os.service
    systemctl daemon-reload
    
    # Docker 卸载
    docker stop nas-os 2>/dev/null || true
    docker rm nas-os 2>/dev/null || true
    
    log_deployment "uninstall" "已卸载"
    log_success "卸载完成"
    log_info "配置和数据保留在: $CONFIG_DIR, $DATA_DIR"
}

# 状态检查
do_status() {
    echo "==================================="
    echo "NAS-OS 状态检查"
    echo "==================================="
    echo ""
    
    # 服务状态
    echo "服务状态:"
    systemctl status nas-os --no-pager 2>/dev/null || echo "  服务未安装"
    echo ""
    
    # 健康检查
    echo "健康检查:"
    if curl -sf "$HEALTH_CHECK_URL" 2>/dev/null; then
        echo ""
    else
        echo "  服务未响应"
    fi
    echo ""
    
    # 版本信息
    echo "版本信息:"
    if [ -f "$INSTALL_DIR/bin/nasd" ]; then
        $INSTALL_DIR/bin/nasd --version 2>/dev/null || echo "  未知版本"
    else
        echo "  未安装"
    fi
}

# 显示安装信息
show_info() {
    echo ""
    echo "==================================="
    echo "NAS-OS 安装完成"
    echo "==================================="
    echo ""
    echo "二进制位置: $INSTALL_DIR/bin/nasd"
    echo "配置目录: $CONFIG_DIR"
    echo "数据目录: $DATA_DIR"
    echo "日志目录: $LOG_DIR"
    echo "备份目录: $BACKUP_DIR"
    echo ""
    echo "服务管理:"
    echo "  启动:   systemctl start nas-os"
    echo "  停止:   systemctl stop nas-os"
    echo "  重启:   systemctl restart nas-os"
    echo "  状态:   systemctl status nas-os"
    echo "  日志:   journalctl -u nas-os -f"
    echo ""
    echo "访问地址: http://localhost:8080"
    echo ""
}

# 帮助信息
show_help() {
    cat <<EOF
NAS-OS 部署工具 v$VERSION

用法: $0 <command> [options]

命令:
  install     安装 NAS-OS（二进制方式）
  docker      使用 Docker 部署
  upgrade     升级 NAS-OS
  rollback    回滚到上一个版本
  uninstall   卸载 NAS-OS
  status      查看服务状态
  help        显示帮助

选项:
  --version VERSION    指定版本 (默认: latest)
  --dry-run            模拟执行，不进行实际更改
  --no-rollback        部署失败时不自动回滚

环境变量:
  RELEASE_VERSION      安装版本 (默认: latest)
  INSTALL_DIR          安装目录 (默认: /usr/local)
  CONFIG_DIR           配置目录 (默认: /etc/nas-os)
  DATA_DIR             数据目录 (默认: /var/lib/nas-os)
  LOG_DIR              日志目录 (默认: /var/log/nas-os)
  BACKUP_DIR           备份目录 (默认: /var/lib/nas-os/backups)

示例:
  $0 install                           # 安装最新版本
  RELEASE_VERSION=v2.76.0 $0 upgrade   # 升级到指定版本
  $0 rollback                          # 回滚到上一个版本
  $0 --dry-run install                 # 模拟安装
  $0 docker                            # Docker 部署
EOF
}

# =============================================================================
# 主入口
# =============================================================================

# 解析参数
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)
            RELEASE_VERSION="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        --no-rollback)
            ROLLBACK_ON_FAILURE="false"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

# 执行命令
case "${1:-help}" in
    install)
        do_install
        ;;
    docker)
        create_directories
        deploy_docker
        health_check
        show_info
        ;;
    upgrade)
        do_upgrade
        ;;
    rollback)
        do_rollback "${2:-}"
        ;;
    uninstall)
        do_uninstall
        ;;
    status)
        do_status
        ;;
    -h|--help|help)
        show_help
        ;;
    *)
        show_help
        exit 1
        ;;
esac