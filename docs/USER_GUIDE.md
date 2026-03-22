# NAS-OS 用户文档

**版本**: v2.253.220 | **更新日期**: 2026-03-23

## 📚 文档目录

1. [快速开始](#快速开始)
2. [安装指南](#安装指南)
3. [存储管理](#存储管理)
4. [文件共享](#文件共享)
5. [用户与权限](#用户与权限)
6. [系统监控](#系统监控)
7. [国际化支持](#国际化支持)
8. [常见问题](#常见问题)

---

## 🚀 快速开始

### 5 分钟上手

```bash
# 1. 下载并解压
wget https://github.com/crazyqin/nas-os/releases/latest/download/nas-os-linux-amd64.tar.gz
tar -xzf nas-os-linux-amd64.tar.gz
cd nas-os

# 2. 启动服务
sudo ./nasd

# 3. 访问 Web 界面
# 浏览器打开 http://localhost:8080
```

### 核心功能一览

| 功能 | 说明 | 状态 |
|------|------|------|
| 存储卷管理 | 创建、删除、扩容 btrfs 卷 | ✅ |
| 子卷/快照 | 灵活的数据管理 | ✅ |
| SMB 共享 | Windows/ macOS 文件共享 | 🚧 |
| NFS 共享 | Linux 文件共享 | 🚧 |
| 用户管理 | 多用户权限控制 | 🚧 |
| 磁盘监控 | 健康检测与告警 | 🚧 |

---

## 📦 安装指南

### 系统要求

- **操作系统**: Linux (推荐 Ubuntu 22.04+ / Debian 11+)
- **内存**: 最低 2GB，推荐 4GB+
- **存储**: btrfs 文件系统支持
- **权限**: root 或 sudo 权限

### 依赖安装

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install btrfs-progs samba nfs-kernel-server

# Arch Linux
sudo pacman -S btrfs-progs samba nfs-utils

# Fedora/RHEL
sudo dnf install btrfs-progs samba nfs-utils
```

### 安装步骤

#### 方式一：二进制安装（推荐）

```bash
# 下载最新版本
wget https://github.com/crazyqin/nas-os/releases/latest/download/nas-os-linux-amd64.tar.gz

# 解压
tar -xzf nas-os-linux-amd64.tar.gz -C /opt/nas-os

# 创建软链接
sudo ln -s /opt/nas-os/nasd /usr/local/bin/nasd
sudo ln -s /opt/nas-os/nasctl /usr/local/bin/nasctl

# 验证安装
nasd --version
```

#### 方式二：源码编译

```bash
# 克隆仓库
git clone https://github.com/crazyqin/nas-os.git
cd nas-os

# 安装 Go 1.21+
# 参考 https://go.dev/dl/

# 编译
go mod tidy
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl

# 安装到系统路径
sudo cp nasd nasctl /usr/local/bin/
```

### 配置系统服务

```bash
# 创建 systemd 服务文件
sudo tee /etc/systemd/system/nas-os.service > /dev/null <<'EOF'
[Unit]
Description=NAS-OS Storage Management
After=network.target local-fs.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nasd
Restart=on-failure
User=root

[Install]
WantedBy=multi-user.target
EOF

# 启用服务
sudo systemctl daemon-reload
sudo systemctl enable nas-os
sudo systemctl start nas-os

# 查看状态
sudo systemctl status nas-os
```

---

## 💾 存储管理

### 创建存储卷

```bash
# 使用命令行工具
sudo nasctl volume create myvolume /dev/sdb1

# 或通过 Web 界面
# 1. 登录 Web 界面
# 2. 点击「存储」>「创建卷」
# 3. 选择磁盘设备，输入卷名称
# 4. 点击创建
```

### 查看卷信息

```bash
# 列出所有卷
sudo nasctl volume list

# 查看卷详情
sudo nasctl volume show myvolume

# 查看磁盘使用情况
sudo nasctl volume usage myvolume
```

### 创建子卷

```bash
# 创建子卷（用于隔离数据）
sudo nasctl subvolume create myvolume docker
sudo nasctl subvolume create myvolume media
sudo nasctl subvolume create myvolume backup
```

### 快照管理

```bash
# 创建快照
sudo nasctl snapshot create myvolume snapshot-2026-03-10

# 列出快照
sudo nasctl snapshot list myvolume

# 恢复快照
sudo nasctl snapshot restore myvolume snapshot-2026-03-10

# 删除快照
sudo nasctl snapshot delete myvolume snapshot-2026-03-10
```

### 数据平衡与校验

```bash
# 平衡数据（优化存储分布）
sudo nasctl balance start myvolume

# 查看平衡状态
sudo nasctl balance status myvolume

# 数据校验（检测静默错误）
sudo nasctl scrub start myvolume

# 查看校验结果
sudo nasctl scrub status myvolume
```

---

## 📁 文件共享

### SMB 共享配置

```bash
# 启用 SMB 共享
sudo nasctl smb enable

# 添加共享目录
sudo nasctl smb share add media /volumes/myvolume/media --readwrite
sudo nasctl smb share add backup /volumes/myvolume/backup --readonly

# 查看共享列表
sudo nasctl smb share list

# 重启 SMB 服务
sudo nasctl smb restart
```

**Windows 访问**: `\\nas-ip-address\media`
**macOS 访问**: `smb://nas-ip-address/media`

### NFS 共享配置

```bash
# 启用 NFS 共享
sudo nasctl nfs enable

# 添加共享目录
sudo nasctl nfs share add /volumes/myvolume/docker 192.168.1.0/24

# 查看共享列表
sudo nasctl nfs share list

# 重启 NFS 服务
sudo nasctl nfs restart
```

**Linux 客户端挂载**:
```bash
sudo mount -t nfs nas-ip-address:/volumes/myvolume/docker /mnt/nas-docker
```

---

## 👥 用户与权限

### 用户管理

```bash
# 创建用户
sudo nasctl user add admin --password --role admin
sudo nasctl user add guest --password --role viewer

# 列出用户
sudo nasctl user list

# 修改用户
sudo nasctl user modify admin --role admin

# 删除用户
sudo nasctl user delete guest
```

### 角色说明

| 角色 | 权限 |
|------|------|
| admin | 全部权限 |
| editor | 读写共享文件，不能管理系统 |
| viewer | 只读访问 |

### 设置目录权限

```bash
# 设置目录访问权限
sudo nasctl acl set /volumes/myvolume/media --user admin --permission rw
sudo nasctl acl set /volumes/myvolume/media --user guest --permission r
```

---

## 📊 系统监控

### 查看系统状态

```bash
# 系统概览
sudo nasctl status

# 磁盘健康
sudo nasctl disk health

# 空间使用
sudo nasctl storage usage

# 系统资源
sudo nasctl resources
```

### 告警配置

```bash
# 设置空间告警阈值（80%）
sudo nasctl alert threshold set --usage 80

# 设置邮件通知
sudo nasctl alert notification add --type email --to admin@example.com

# 查看告警历史
sudo nasctl alert history
```

---

## 🌐 国际化支持

NAS-OS v2.36.0 提供完整的国际化支持，支持多语言界面切换。

### 支持的语言

| 语言 | 代码 | 状态 |
|------|------|------|
| 简体中文 | zh-CN | ✅ 完整 |
| English | en-US | ✅ 完整 |
| 日本語 | ja-JP | ✅ 完整 |
| 한국어 | ko-KR | ✅ 完整 |

### 切换语言

#### WebUI 切换

1. 登录 Web 界面
2. 点击右上角「设置」图标
3. 选择「语言设置」
4. 选择目标语言并保存

#### API 切换

```bash
# 通过请求头设置语言
curl -H "Accept-Language: zh-CN" http://localhost:8080/api/v1/system/info

# 或通过查询参数
curl http://localhost:8080/api/v1/system/info?lang=zh-CN
```

### 添加新语言

详见 [翻译指南](../TRANSLATION.md)

---

## ❓ 常见问题

### Q1: 启动失败，提示权限错误
**A**: NAS-OS 需要 root 权限访问磁盘设备。请使用 `sudo` 启动或配置 systemd 服务。

### Q2: Web 界面无法访问
**A**: 
1. 检查服务状态：`sudo systemctl status nas-os`
2. 检查防火墙：`sudo ufw allow 8080/tcp`
3. 确认端口未被占用：`sudo lsof -i :8080`

### Q3: btrfs 设备不识别
**A**: 
1. 确认已安装 btrfs-progs
2. 检查设备是否存在：`lsblk`
3. 确认设备未被挂载：`mount | grep /dev/sd`

### Q4: 如何备份系统配置
**A**: 
```bash
# 导出配置
sudo nasctl config export > nas-config-backup.yaml

# 导入配置
sudo nasctl config import nas-config-backup.yaml
```

### Q5: 性能优化建议
**A**:
- 使用 SSD 作为缓存设备
- 启用 btrfs 压缩：`sudo btrfs property set /volumes/myvolume compression zstd`
- 定期运行 balance 和 scrub

### Q6: 如何启用双重认证 (MFA)
**A**: 
1. 登录 Web 界面
2. 进入「设置」→「安全设置」
3. 点击「启用 MFA」
4. 使用 Google Authenticator 等应用扫描二维码
5. 输入验证码确认启用

### Q7: 如何配置定时快照
**A**: 
```bash
# 创建快照策略
sudo nasctl snapshot policy create daily \
  --volume myvolume \
  --schedule "0 2 * * *" \
  --retention 7

# 查看策略
sudo nasctl snapshot policy list
```

### Q8: 如何扩展存储卷
**A**: 
```bash
# 添加新磁盘到现有卷
sudo nasctl volume add-device myvolume /dev/sdc

# 重新平衡数据
sudo nasctl balance start myvolume
```

### Q9: 如何迁移数据到新 NAS
**A**: 
1. 在新 NAS 上创建相同的存储卷和共享
2. 使用 rsync 迁移数据：
```bash
rsync -avz --progress /data/ user@new-nas:/data/
```
3. 导出并导入用户配置：
```bash
# 旧 NAS
sudo nasctl config export > nas-config.yaml

# 新 NAS  
sudo nasctl config import nas-config.yaml
```

### Q10: 如何查看系统日志
**A**: 
```bash
# 查看服务日志
sudo journalctl -u nas-os -f

# 查看应用日志
tail -f /var/log/nas-os/nasd.log
```

### Q11: 忘记管理员密码怎么办
**A**: 
```bash
# 重置管理员密码
sudo nasctl user reset-password admin --new-password NewPass123!
```

### Q12: 如何配置邮件告警
**A**: 
1. 进入「设置」→「通知设置」
2. 配置 SMTP 服务器信息
3. 添加告警接收邮箱
4. 发送测试邮件验证配置

---

## 📞 获取帮助

- **文档**: https://nas-os.dev/docs
- **Issues**: https://github.com/crazyqin/nas-os/issues
- **讨论区**: https://github.com/crazyqin/nas-os/discussions
- **邮件**: support@nas-os.dev

---

*文档版本：v2.253.219 | 最后更新：2026-03-23*

---

## 🔐 LDAP/AD 集成

NAS-OS 支持与企业 LDAP 和 Active Directory 集成，实现统一身份认证。

### 支持的服务器类型

| 类型 | 说明 |
|------|------|
| OpenLDAP | 开源 LDAP 实现 |
| Active Directory | 微软企业目录服务 |
| FreeIPA | Red Hat 身份管理解决方案 |

### 快速配置

1. 进入「设置」→「LDAP/AD 集成」
2. 点击「添加服务器」
3. 填写服务器信息和认证参数
4. 测试连接并保存

详细配置请参考 [LDAP 集成指南](LDAP-INTEGRATION.md)。
