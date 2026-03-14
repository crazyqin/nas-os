# NAS-OS 快速开始指南

**版本**: v2.31.0  
**更新日期**: 2026-03-15

---

## 📋 目录

1. [系统要求](#系统要求)
2. [安装方式](#安装方式)
3. [首次配置](#首次配置)
4. [基本使用](#基本使用)
5. [LDAP/AD 集成](#ldapad-集成)
6. [常见问题](#常见问题)

---

## 系统要求

### 硬件要求

| 配置 | 最低要求 | 推荐配置 |
|------|----------|----------|
| CPU | 双核 1.5GHz | 四核 2.0GHz+ |
| 内存 | 2GB | 4GB+ |
| 存储 | 20GB | 100GB+ (取决于数据量) |
| 网络 | 100Mbps | 1Gbps |

### 软件要求

- **操作系统**: Linux (推荐 Ubuntu 22.04+ / Debian 12+)
- **内核**: 5.0+ (支持 btrfs)
- **依赖**: 
  - btrfs-progs
  - samba (可选，SMB 共享)
  - nfs-kernel-server (可选，NFS 共享)

### 支持架构

- x86_64 (AMD64)
- ARM64 (Orange Pi 5, Raspberry Pi 4/5)
- ARMv7 (Raspberry Pi 3)

---

## 安装方式

### 方式一：下载二进制文件 (推荐)

```bash
# AMD64 (x86_64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.30.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/v2.30.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7 (Raspberry Pi 3)
wget https://github.com/crazyqin/nas-os/releases/download/v2.30.0/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd

# 验证安装
nasd --version
```

### 方式二：Docker 部署

```bash
# 拉取镜像
docker pull ghcr.io/crazyqin/nas-os:v2.30.0

# 创建配置目录
mkdir -p /etc/nas-os /data

# 运行容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  --privileged \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.30.0

# 查看日志
docker logs -f nasd
```

### 方式三：源码编译

```bash
# 安装依赖
sudo apt update
sudo apt install -y btrfs-progs samba nfs-kernel-server

# 安装 Go 1.26.1+
wget https://go.dev/dl/go1.26.1.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.1.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 克隆仓库
git clone https://github.com/crazyqin/nas-os.git
cd nas-os

# 编译
go mod tidy
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl

# 安装
sudo mv nasd nasctl /usr/local/bin/
```

---

## 首次配置

### 1. 启动服务

```bash
# 直接运行 (需要 root 权限)
sudo nasd

# 或作为系统服务
sudo nasctl service install
sudo systemctl start nas-os
sudo systemctl enable nas-os
```

### 2. 访问 Web 界面

打开浏览器访问：`http://<服务器IP>:8080`

**默认登录凭据**：
- 用户名：`admin`
- 密码：`admin123`

⚠️ **首次登录后请立即修改默认密码！**

### 3. 修改默认密码

1. 登录后点击右上角用户头像
2. 选择「账户设置」
3. 输入新密码并保存

### 4. 创建存储卷

#### 通过 Web UI

1. 导航到「存储」→「卷管理」
2. 点击「创建卷」
3. 选择磁盘设备、RAID 级别
4. 点击「创建」

#### 通过 CLI

```bash
# 查看可用磁盘
sudo nasctl disk list

# 创建卷
sudo nasctl volume create data --devices /dev/sda,/dev/sdb --raid raid1
```

---

## 基本使用

### 创建共享文件夹

#### SMB 共享 (Windows/macOS)

```bash
# 创建 SMB 共享
sudo nasctl share create smb public \
  --path /data/public \
  --guest-ok

# Windows 访问: \\<服务器IP>\public
# macOS 访问: smb://<服务器IP>/public
```

#### NFS 共享 (Linux)

```bash
# 创建 NFS 共享
sudo nasctl share create nfs backup \
  --path /data/backup \
  --network 192.168.1.0/24

# Linux 挂载
sudo mount <服务器IP>:/backup /mnt/backup
```

### 用户管理

```bash
# 创建用户
sudo nasctl user create zhangsan --password SecurePass123

# 创建用户组
sudo nasctl group create family

# 添加用户到组
sudo nasctl group add-user family zhangsan

# 设置共享权限
sudo nasctl share set-permission public --group family --permission rw
```

### 快照管理

```bash
# 创建快照
sudo nasctl snapshot create data --name backup-$(date +%Y%m%d)

# 列出快照
sudo nasctl snapshot list data

# 恢复快照
sudo nasctl snapshot restore data backup-20260315
```

### 监控和告警

```bash
# 查看系统状态
sudo nasctl status

# 查看告警
sudo nasctl alerts list

# 配置邮件告警
sudo nasctl notify add email \
  --address admin@example.com \
  --events disk_warning,backup_failed
```

---

## 快速参考

### 常用 CLI 命令

| 命令 | 说明 |
|------|------|
| `nasd --version` | 查看版本 |
| `nasd --config /path/to/config.yaml` | 指定配置文件 |
| `nasctl status` | 系统状态 |
| `nasctl disk list` | 磁盘列表 |
| `nasctl volume list` | 卷列表 |
| `nasctl share list` | 共享列表 |
| `nasctl user list` | 用户列表 |

### 常用 API 端点

| 端点 | 说明 |
|------|------|
| `GET /api/v1/volumes` | 获取卷列表 |
| `GET /api/v1/shares` | 获取共享列表 |
| `GET /api/v1/monitor/stats` | 系统统计 |
| `GET /api/v1/monitor/alerts` | 活动告警 |

### 配置文件位置

| 文件 | 位置 |
|------|------|
| 主配置 | `/etc/nas-os/config.yaml` |
| 日志目录 | `/var/log/nas-os/` |
| 数据目录 | `/data/` |

---

## LDAP/AD 集成

NAS-OS 支持与企业 LDAP/Active Directory 集成，实现统一身份认证。

### 快速配置 LDAP

```bash
# 添加 OpenLDAP 服务器
curl -X POST http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "company-ldap",
    "url": "ldaps://ldap.example.com:636",
    "bind_dn": "cn=admin,dc=example,dc=com",
    "bind_password": "password",
    "base_dn": "dc=example,dc=com",
    "user_filter": "(uid=%s)",
    "group_filter": "(memberUid=%s)",
    "enabled": true
  }'

# 测试连接
curl -X POST http://localhost:8080/api/v1/ldap/configs/company-ldap/test \
  -H "Authorization: Bearer TOKEN"
```

### 快速配置 Active Directory

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "company-ad",
    "url": "ldaps://ad.example.com:636",
    "bind_dn": "CN=ldap-bind,CN=Users,DC=example,DC=com",
    "bind_password": "password",
    "base_dn": "DC=example,DC=com",
    "user_filter": "(sAMAccountName=%s)",
    "group_filter": "(member=%s)",
    "is_ad": true,
    "enabled": true
  }'
```

📖 详细配置请参考 [LDAP 集成指南](LDAP-INTEGRATION.md)

---

## 常见问题

### Q: 无法访问 Web 界面？

1. 检查服务是否运行：`systemctl status nas-os`
2. 检查端口是否开放：`sudo ufw allow 8080`
3. 查看日志：`journalctl -u nas-os -f`

### Q: 无法创建 btrfs 卷？

1. 确认磁盘未挂载：`lsblk`
2. 安装 btrfs-progs：`sudo apt install btrfs-progs`
3. 检查内核支持：`grep btrfs /proc/filesystems`

### Q: SMB 共享无法访问？

1. 检查 Samba 服务：`systemctl status smbd`
2. 检查防火墙：`sudo ufw allow samba`
3. 查看共享状态：`sudo smbstatus`

### Q: 忘记管理员密码？

```bash
# 重置密码
sudo nasctl user reset-password admin --password NewPassword123
```

---

## 下一步

- 📖 阅读完整 [用户手册](USER_GUIDE.md)
- 🔧 查看 [管理员指南](ADMIN_GUIDE_v2.5.0.md)
- 📡 参考 [API 文档](API_GUIDE.md)
- 🚀 了解 [部署指南](DEPLOYMENT_GUIDE_v2.5.0.md)
- 🔐 配置 [LDAP/AD 集成](LDAP-INTEGRATION.md)

---

## 获取帮助

- 📖 **文档**: [docs/](.) 目录
- 🐛 **问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- 💬 **讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)
- 📦 **Docker 镜像**: [GHCR](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

---

**最后更新**: 2026-03-15