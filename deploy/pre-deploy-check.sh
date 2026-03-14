#!/bin/bash
# NAS-OS 部署前检查脚本
# 验证系统环境是否满足部署要求
#
# v2.38.0 新增

set -e

# 配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 检查结果
PASSED=0
WARNINGS=0
FAILED=0

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    PASSED=$((PASSED + 1))
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    WARNINGS=$((WARNINGS + 1))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    FAILED=$((FAILED + 1))
}

# 检查操作系统
check_os() {
    log_info "检查操作系统..."
    
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        log_pass "操作系统: $PRETTY_NAME"
    else
        log_warn "无法确定操作系统版本"
    fi
    
    # 检查内核版本
    local kernel=$(uname -r)
    log_pass "内核版本: $kernel"
    
    # 检查架构
    local arch=$(uname -m)
    case $arch in
        x86_64|aarch64|armv7l)
            log_pass "系统架构: $arch"
            ;;
        *)
            log_warn "不推荐的架构: $arch"
            ;;
    esac
}

# 检查内存
check_memory() {
    log_info "检查内存..."
    
    local total_mb=$(free -m | awk '/^Mem:/{print $2}')
    
    if [ $total_mb -ge 4096 ]; then
        log_pass "内存: ${total_mb}MB (推荐 4GB+)"
    elif [ $total_mb -ge 2048 ]; then
        log_warn "内存: ${total_mb}MB (最低 2GB，推荐 4GB+)"
    else
        log_fail "内存: ${total_mb}MB (不足 2GB)"
    fi
    
    # 检查可用内存
    local avail_mb=$(free -m | awk '/^Mem:/{print $7}')
    if [ $avail_mb -ge 512 ]; then
        log_pass "可用内存: ${avail_mb}MB"
    else
        log_warn "可用内存: ${avail_mb}MB (可能影响性能)"
    fi
}

# 检查磁盘空间
check_disk() {
    log_info "检查磁盘空间..."
    
    # 检查根分区
    local root_avail=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
    
    if [ $root_avail -ge 50 ]; then
        log_pass "根分区可用空间: ${root_avail}GB"
    elif [ $root_avail -ge 20 ]; then
        log_warn "根分区可用空间: ${root_avail}GB (推荐 50GB+)"
    else
        log_fail "根分区可用空间: ${root_avail}GB (不足 20GB)"
    fi
    
    # 检查数据目录
    local data_dir="/var/lib/nas-os"
    if [ -d "$data_dir" ]; then
        local data_avail=$(df -BG "$data_dir" 2>/dev/null | awk 'NR==2 {print $4}' | tr -d 'G')
        log_pass "数据目录可用空间: ${data_avail}GB"
    else
        log_info "数据目录不存在，将创建: $data_dir"
    fi
}

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    # 必需依赖
    local required_deps=("curl" "tar" "gzip" "sqlite3")
    for dep in "${required_deps[@]}"; do
        if command -v "$dep" &> /dev/null; then
            log_pass "已安装: $dep"
        else
            log_fail "缺少依赖: $dep"
        fi
    done
    
    # 可选依赖
    local optional_deps=("docker" "btrfs-progs" "samba")
    for dep in "${optional_deps[@]}"; do
        if command -v "$dep" &> /dev/null; then
            log_pass "已安装: $dep (可选)"
        else
            log_warn "未安装: $dep (可选)"
        fi
    done
}

# 检查网络
check_network() {
    log_info "检查网络..."
    
    # 检查网络接口
    local interfaces=$(ip link show | grep -E '^[0-9]+:' | grep -v lo | wc -l)
    if [ $interfaces -ge 1 ]; then
        log_pass "网络接口数量: $interfaces"
    else
        log_fail "未检测到网络接口"
    fi
    
    # 检查 DNS
    if nslookup github.com &> /dev/null; then
        log_pass "DNS 解析正常"
    else
        log_warn "DNS 解析可能存在问题"
    fi
    
    # 检查外网连接
    if curl -sf --connect-timeout 5 https://github.com > /dev/null 2>&1; then
        log_pass "外网连接正常"
    else
        log_warn "外网连接可能受限"
    fi
}

# 检查端口
check_ports() {
    log_info "检查端口..."
    
    local ports=("8080" "445" "2049")
    local port_names=("Web UI" "SMB" "NFS")
    
    for i in "${!ports[@]}"; do
        local port=${ports[$i]}
        local name=${port_names[$i]}
        
        if ss -tuln | grep -q ":${port} "; then
            log_warn "端口 $port ($name) 已被占用"
        else
            log_pass "端口 $port ($name) 可用"
        fi
    done
}

# 检查权限
check_permissions() {
    log_info "检查权限..."
    
    # 检查是否为 root 或有 sudo
    if [ "$(id -u)" -eq 0 ]; then
        log_pass "以 root 用户运行"
    elif command -v sudo &> /dev/null && sudo -n true 2>/dev/null; then
        log_pass "有 sudo 权限"
    else
        log_warn "需要 root 或 sudo 权限"
    fi
    
    # 检查必要目录权限
    local dirs=("/var/lib/nas-os" "/var/log/nas-os" "/etc/nas-os")
    for dir in "${dirs[@]}"; do
        if [ -d "$dir" ]; then
            if [ -w "$dir" ]; then
                log_pass "目录可写: $dir"
            else
                log_warn "目录不可写: $dir"
            fi
        else
            log_info "目录不存在: $dir (将创建)"
        fi
    done
}

# 检查 Docker
check_docker() {
    log_info "检查 Docker..."
    
    if command -v docker &> /dev/null; then
        log_pass "Docker 已安装"
        
        # 检查 Docker 服务
        if systemctl is-active docker &> /dev/null; then
            log_pass "Docker 服务运行中"
        else
            log_warn "Docker 服务未运行"
        fi
        
        # 检查 Docker Compose
        if docker compose version &> /dev/null; then
            log_pass "Docker Compose 已安装"
        elif command -v docker-compose &> /dev/null; then
            log_pass "docker-compose 已安装 (旧版本)"
        else
            log_warn "Docker Compose 未安装"
        fi
        
        # 检查 Docker 版本
        local docker_version=$(docker --version | grep -oP '\d+\.\d+\.\d+' | head -1)
        log_info "Docker 版本: $docker_version"
    else
        log_warn "Docker 未安装 (可选部署方式)"
    fi
}

# 检查防火墙
check_firewall() {
    log_info "检查防火墙..."
    
    # 检查 ufw
    if command -v ufw &> /dev/null; then
        if ufw status | grep -q "active"; then
            log_warn "UFW 防火墙已启用，请确保开放必要端口"
            ufw status | grep -E "8080|445|2049" || log_warn "部分端口可能未开放"
        else
            log_info "UFW 防火墙未启用"
        fi
    fi
    
    # 检查 firewalld
    if command -v firewall-cmd &> /dev/null; then
        if systemctl is-active firewalld &> /dev/null; then
            log_warn "Firewalld 已启用，请确保开放必要端口"
        else
            log_info "Firewalld 未运行"
        fi
    fi
    
    # 检查 iptables
    if command -v iptables &> /dev/null; then
        local rules=$(iptables -L 2>/dev/null | wc -l)
        if [ $rules -gt 8 ]; then
            log_warn "检测到 iptables 规则，请确保开放必要端口"
        else
            log_pass "iptables 规则较少或无限制"
        fi
    fi
}

# 检查 SELinux
check_selinux() {
    log_info "检查 SELinux..."
    
    if command -v getenforce &> /dev/null; then
        local selinux=$(getenforce 2>/dev/null || echo "Unknown")
        case $selinux in
            Enforcing)
                log_warn "SELinux 处于 Enforcing 模式，可能需要配置策略"
                ;;
            Permissive)
                log_info "SELinux 处于 Permissive 模式"
                ;;
            Disabled)
                log_pass "SELinux 已禁用"
                ;;
            *)
                log_info "SELinux 状态: $selinux"
                ;;
        esac
    else
        log_info "SELinux 未安装"
    fi
}

# 检查系统服务
check_services() {
    log_info "检查系统服务..."
    
    # 检查 systemd
    if pidof systemd &> /dev/null; then
        log_pass "使用 systemd"
    else
        log_warn "未检测到 systemd"
    fi
    
    # 检查可能冲突的服务
    local conflict_services=("smbd" "nfs-server" "docker")
    for service in "${conflict_services[@]}"; do
        if systemctl is-active "$service" &> /dev/null; then
            log_warn "服务 $service 已运行（可能与 NAS-OS 冲突或共存）"
        fi
    done
}

# 打印总结
print_summary() {
    echo ""
    echo "==================================="
    echo "检查完成"
    echo "==================================="
    echo -e "通过: ${GREEN}$PASSED${NC}"
    echo -e "警告: ${YELLOW}$WARNINGS${NC}"
    echo -e "失败: ${RED}$FAILED${NC}"
    echo ""
    
    if [ $FAILED -gt 0 ]; then
        log_fail "存在 $FAILED 个检查失败项，请修复后再部署"
        exit 1
    elif [ $WARNINGS -gt 0 ]; then
        log_warn "存在 $WARNINGS 个警告项，建议检查"
        exit 0
    else
        log_pass "所有检查通过，可以部署"
        exit 0
    fi
}

# 运行所有检查
run_all_checks() {
    echo ""
    echo "==================================="
    echo "NAS-OS 部署前检查"
    echo "==================================="
    echo ""
    
    check_os
    check_memory
    check_disk
    check_dependencies
    check_network
    check_ports
    check_permissions
    check_docker
    check_firewall
    check_selinux
    check_services
    
    print_summary
}

# 帮助信息
show_help() {
    cat <<EOF
NAS-OS 部署前检查工具

用法: $0 [command]

命令:
  all        运行所有检查（默认）
  os         检查操作系统
  memory     检查内存
  disk       检查磁盘空间
  deps       检查依赖
  network    检查网络
  ports      检查端口
  docker     检查 Docker
  help       显示帮助

示例:
  $0 all
  $0 memory disk
EOF
}

# 主入口
case "${1:-all}" in
    all)
        run_all_checks
        ;;
    os)
        check_os
        ;;
    memory)
        check_memory
        ;;
    disk)
        check_disk
        ;;
    deps)
        check_dependencies
        ;;
    network)
        check_network
        ;;
    ports)
        check_ports
        ;;
    docker)
        check_docker
        ;;
    -h|--help|help)
        show_help
        ;;
    *)
        # 允许运行多个检查
        for cmd in "$@"; do
            case $cmd in
                os|memory|disk|deps|network|ports|docker)
                    "check_$cmd"
                    ;;
                *)
                    log_warn "未知检查项: $cmd"
                    ;;
            esac
        done
        print_summary
        ;;
esac