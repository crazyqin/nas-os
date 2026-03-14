#!/bin/bash
# NAS-OS 一键部署脚本
# 支持二进制部署和 Docker 部署
#
# v2.38.0 新增

set -e

# 配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/usr/local}"
CONFIG_DIR="${CONFIG_DIR:-/etc/nas-os}"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"
VERSION="${VERSION:-latest}"
RELEASE_URL="https://github.com/nas-os/nas-os/releases"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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
        log_error "需要 root 权限"
        exit 1
    fi
}

# 创建目录结构
create_directories() {
    log_info "创建目录结构..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$DATA_DIR/plugins"
    mkdir -p "$DATA_DIR/backups"
    
    chmod 755 "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    
    log_success "目录创建完成"
}

# 安装依赖
install_dependencies() {
    log_info "安装依赖..."
    
    local os=$(detect_os)
    
    case $os in
        ubuntu|debian)
            apt-get update
            apt-get install -y curl sqlite3 btrfs-progs
            ;;
        centos|rhel|rocky|almalinux)
            yum install -y curl sqlite btrfs-progs
            ;;
        arch|manjaro)
            pacman -S --noconfirm curl sqlite btrfs-progs
            ;;
        *)
            log_warn "未知操作系统，请手动安装依赖: curl, sqlite3, btrfs-progs"
            ;;
    esac
    
    log_success "依赖安装完成"
}

# 下载二进制
download_binary() {
    local arch=$(detect_arch)
    local os="linux"
    
    if [ "$arch" = "unsupported" ]; then
        log_error "不支持的架构: $(uname -m)"
        exit 1
    fi
    
    log_info "下载 NAS-OS 二进制 ($os/$arch)..."
    
    local download_url
    if [ "$VERSION" = "latest" ]; then
        download_url="$RELEASE_URL/latest/download/nasd-$os-$arch"
    else
        download_url="$RELEASE_URL/download/$VERSION/nasd-$os-$arch"
    fi
    
    log_info "下载地址: $download_url"
    
    curl -fSL -o /tmp/nasd "$download_url"
    chmod +x /tmp/nasd
    
    log_success "二进制下载完成"
}

# 安装二进制
install_binary() {
    log_info "安装二进制..."
    
    # 备份旧版本
    if [ -f "$INSTALL_DIR/bin/nasd" ]; then
        mv "$INSTALL_DIR/bin/nasd" "$INSTALL_DIR/bin/nasd.bak"
    fi
    
    # 安装新版本
    mv /tmp/nasd "$INSTALL_DIR/bin/nasd"
    chmod 755 "$INSTALL_DIR/bin/nasd"
    
    log_success "二进制安装完成"
}

# 安装配置文件
install_config() {
    log_info "安装配置文件..."
    
    local config_src="$PROJECT_ROOT/configs/default.yaml"
    
    if [ -f "$config_src" ]; then
        if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
            cp "$config_src" "$CONFIG_DIR/config.yaml"
            log_success "配置文件已安装"
        else
            log_warn "配置文件已存在，跳过"
        fi
    else
        log_warn "默认配置文件不存在，创建空配置"
        touch "$CONFIG_DIR/config.yaml"
    fi
}

# 创建 systemd 服务
create_systemd_service() {
    log_info "创建 systemd 服务..."
    
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
# 需要访问磁盘设备，暂不启用严格沙箱

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable nas-os
    
    log_success "systemd 服务创建完成"
}

# 启动服务
start_service() {
    log_info "启动 NAS-OS 服务..."
    
    systemctl start nas-os
    
    # 等待服务启动
    sleep 3
    
    if systemctl is-active nas-os &> /dev/null; then
        log_success "服务启动成功"
    else
        log_error "服务启动失败"
        journalctl -u nas-os --no-pager -n 20
        exit 1
    fi
}

# 检查服务状态
check_status() {
    log_info "检查服务状态..."
    
    echo ""
    systemctl status nas-os --no-pager
    echo ""
    
    # 检查健康端点
    sleep 2
    if curl -sf http://localhost:8080/api/v1/health &> /dev/null; then
        log_success "健康检查通过"
    else
        log_warn "健康检查未通过，服务可能仍在启动中"
    fi
}

# Docker 部署
deploy_docker() {
    log_info "使用 Docker 部署..."
    
    # 检查 Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi
    
    # 拉取镜像
    log_info "拉取镜像..."
    local image="ghcr.io/nas-os/nas-os:$VERSION"
    docker pull "$image"
    
    # 停止旧容器
    docker stop nas-os 2>/dev/null || true
    docker rm nas-os 2>/dev/null || true
    
    # 启动新容器
    log_info "启动容器..."
    docker run -d \
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

# 卸载
uninstall() {
    log_info "卸载 NAS-OS..."
    
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
    
    log_success "卸载完成"
    log_info "配置和数据保留在: $CONFIG_DIR, $DATA_DIR"
}

# 升级
upgrade() {
    log_info "升级 NAS-OS..."
    
    # 备份
    if [ -f "$INSTALL_DIR/bin/nasd" ]; then
        cp "$INSTALL_DIR/bin/nasd" "$INSTALL_DIR/bin/nasd.bak.$(date +%Y%m%d)"
    fi
    
    # 下载新版本
    download_binary
    install_binary
    
    # 重启服务
    systemctl restart nas-os
    
    log_success "升级完成"
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
    echo "默认账号: admin"
    echo "默认密码: 请查看配置文件或首次启动日志"
    echo ""
}

# 帮助信息
show_help() {
    cat <<EOF
NAS-OS 部署工具

用法: $0 <command> [options]

命令:
  install     安装 NAS-OS（二进制方式）
  docker      使用 Docker 部署
  upgrade     升级 NAS-OS
  uninstall   卸载 NAS-OS
  status      查看服务状态
  help        显示帮助

环境变量:
  VERSION     安装版本 (默认: latest)
  INSTALL_DIR 安装目录 (默认: /usr/local)
  CONFIG_DIR  配置目录 (默认: /etc/nas-os)
  DATA_DIR    数据目录 (默认: /var/lib/nas-os)
  LOG_DIR     日志目录 (默认: /var/log/nas-os)

示例:
  $0 install                    # 安装最新版本
  VERSION=v2.38.0 $0 install    # 安装指定版本
  $0 docker                     # Docker 部署
  $0 upgrade                    # 升级
  $0 uninstall                  # 卸载
EOF
}

# 主入口
case "${1:-}" in
    install)
        check_root
        create_directories
        install_dependencies
        download_binary
        install_binary
        install_config
        create_systemd_service
        start_service
        check_status
        show_info
        ;;
    docker)
        create_directories
        deploy_docker
        check_status
        show_info
        ;;
    upgrade)
        check_root
        upgrade
        ;;
    uninstall)
        check_root
        uninstall
        ;;
    status)
        check_status
        ;;
    -h|--help|help)
        show_help
        ;;
    *)
        show_help
        exit 1
        ;;
esac