#!/bin/bash
#
# NAS-OS 系统安装脚本
# 适用于 Debian/Ubuntu 系统
#
# 用法: curl -fsSL https://raw.githubusercontent.com/your-org/nas-os/main/scripts/install.sh | sudo bash
# 或：wget -qO- https://... | sudo bash
#

set -e

# ========== 配置 ==========
NAS_OS_VERSION="${NAS_OS_VERSION:-latest}"
INSTALL_DIR="/opt/nas-os"
CONFIG_DIR="/etc/nas-os"
DATA_DIR="/var/lib/nas-os"
LOG_DIR="/var/log/nas-os"
SYSTEMD_SERVICE="/etc/systemd/system/nas-os.service"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC}   $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

# ========== 检查 ==========
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "请使用 root 权限运行此脚本 (sudo)"
        exit 1
    fi
}

check_os() {
    if [[ -f /etc/debian_version ]]; then
        OS_TYPE="debian"
        log_info "检测到 Debian/Ubuntu 系统"
    elif [[ -f /etc/redhat-release ]]; then
        OS_TYPE="redhat"
        log_info "检测到 RHEL/CentOS 系统"
    else
        log_warn "未识别的操作系统，尝试通用安装..."
        OS_TYPE="unknown"
    fi
}

check_btrfs() {
    if ! command -v btrfs &> /dev/null; then
        log_warn "btrfs 工具未安装，将尝试安装"
        return 1
    fi
    log_success "btrfs 工具已安装"
    return 0
}

# ========== 安装依赖 ==========
install_dependencies() {
    log_info "安装系统依赖..."
    
    case $OS_TYPE in
        debian)
            apt-get update -qq
            apt-get install -y -qq \
                btrfs-progs \
                samba \
                nfs-kernel-server \
                curl \
                wget \
                jq \
                systemd
            ;;
        redhat)
            yum install -y -q \
                btrfs-progs \
                samba \
                nfs-utils \
                curl \
                wget \
                jq \
                systemd
            ;;
        *)
            log_warn "未知系统，请手动安装以下依赖："
            echo "  - btrfs-progs"
            echo "  - samba"
            echo "  - nfs-kernel-server"
            ;;
    esac
    
    log_success "依赖安装完成"
}

# ========== 创建目录结构 ==========
create_directories() {
    log_info "创建目录结构..."
    
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    
    log_success "目录创建完成"
}

# ========== 下载二进制 ==========
download_binary() {
    log_info "下载 NAS-OS ${NAS_OS_VERSION}..."
    
    # 检测架构
    ARCH=$(uname -m)
    case $ARCH in
        x86_64) ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        armv7l) ARCH="armv7" ;;
        *) log_error "不支持的架构：$ARCH"; exit 1 ;;
    esac
    
    # 下载（从 GitHub Releases）
    BASE_URL="https://github.com/your-org/nas-os/releases"
    
    if [[ "$NAS_OS_VERSION" == "latest" ]]; then
        DOWNLOAD_URL="${BASE_URL}/latest/download/nasd-linux-${ARCH}"
    else
        DOWNLOAD_URL="${BASE_URL}/download/${NAS_OS_VERSION}/nasd-linux-${ARCH}"
    fi
    
    # 临时方案：从本地构建（如果没有网络访问）
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$INSTALL_DIR/nasd" 2>/dev/null; then
        log_warn "无法从网络下载，请使用本地构建的二进制文件"
        log_info "运行：go build -o $INSTALL_DIR/nasd ./cmd/nasd"
        return 1
    fi
    
    chmod +x "$INSTALL_DIR/nasd"
    log_success "二进制文件下载完成"
}

# ========== 创建配置文件 ==========
create_config() {
    log_info "创建配置文件..."
    
    cat > "$CONFIG_DIR/config.yaml" << 'EOF'
# NAS-OS 配置文件

server:
  port: 8080
  host: 0.0.0.0

storage:
  mount_base: /mnt
  default_profile: single
  auto_scrub: true
  scrub_schedule: "0 2 * * 0"

smb:
  enabled: true
  workgroup: WORKGROUP
  guest_access: false

nfs:
  enabled: true
  allowed_networks:
    - 192.168.1.0/24

users:
  admin:
    password_hash: ""
    role: admin

monitor:
  disk_check_interval: 60
  space_warning_threshold: 80
  space_critical_threshold: 95

docker:
  enabled: false
  data_root: /mnt/docker
EOF
    
    log_success "配置文件创建完成：$CONFIG_DIR/config.yaml"
}

# ========== 创建 systemd 服务 ==========
create_systemd_service() {
    log_info "创建 systemd 服务..."
    
    cat > "$SYSTEMD_SERVICE" << 'EOF'
[Unit]
Description=NAS-OS Management Service
Documentation=https://github.com/your-org/nas-os
After=network.target btrfs.target
Wants=btrfs.target

[Service]
Type=simple
ExecStart=/opt/nas-os/nasd --config /etc/nas-os/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=nas-os

# 安全设置
NoNewPrivileges=false
ProtectSystem=false
ProtectHome=false
PrivateTmp=false

# 资源限制
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF
    
    log_success "systemd 服务创建完成"
}

# ========== 配置防火墙 ==========
configure_firewall() {
    log_info "配置防火墙规则..."
    
    if command -v ufw &> /dev/null; then
        ufw allow 8080/tcp comment "NAS-OS Web UI"
        ufw allow 445/tcp comment "SMB"
        ufw allow 2049/tcp comment "NFS"
        ufw allow 111/tcp comment "RPC"
        log_success "UFW 防火墙规则已配置"
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-port=8080/tcp
        firewall-cmd --permanent --add-port=445/tcp
        firewall-cmd --permanent --add-port=2049/tcp
        firewall-cmd --permanent --add-port=111/tcp
        firewall-cmd --reload
        log_success "firewalld 规则已配置"
    else
        log_warn "未检测到防火墙工具，请手动配置端口"
    fi
}

# ========== 启用服务 ==========
enable_service() {
    log_info "启用并启动服务..."
    
    systemctl daemon-reload
    systemctl enable nas-os
    systemctl start nas-os
    
    sleep 2
    
    if systemctl is-active --quiet nas-os; then
        log_success "NAS-OS 服务已启动"
    else
        log_error "服务启动失败，请检查日志：journalctl -u nas-os"
        exit 1
    fi
}

# ========== 主流程 ==========
main() {
    echo "================================"
    echo "  NAS-OS 安装脚本"
    echo "  版本：$NAS_OS_VERSION"
    echo "================================"
    echo
    
    check_root
    check_os
    check_btrfs || install_dependencies
    create_directories
    download_binary
    create_config
    create_systemd_service
    configure_firewall
    enable_service
    
    echo
    echo "================================"
    log_success "NAS-OS 安装完成!"
    echo "================================"
    echo
    echo "📊 Web 管理界面：http://$(hostname -I | awk '{print $1}'):8080"
    echo "📁 配置文件：$CONFIG_DIR/config.yaml"
    echo "📝 日志查看：journalctl -u nas-os -f"
    echo
    echo "常用命令:"
    echo "  systemctl status nas-os    # 查看状态"
    echo "  systemctl restart nas-os   # 重启服务"
    echo "  systemctl stop nas-os      # 停止服务"
    echo
}

# 运行主流程
main "$@"
