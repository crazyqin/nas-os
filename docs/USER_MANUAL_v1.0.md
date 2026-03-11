# NAS-OS v1.0 用户手册

> **版本**: v1.0.0  
> **发布日期**: 2026-03-11  
> **适用版本**: NAS-OS v1.0.0+

---

## 目录

1. [产品简介](#产品简介)
2. [快速开始](#快速开始)
3. [安装与配置](#安装与配置)
4. [存储管理](#存储管理)
5. [文件共享](#文件共享)
6. [用户管理](#用户管理)
7. [系统监控](#系统监控)
8. [备份与恢复](#备份与恢复)
9. [故障排查](#故障排查)
10. [附录](#附录)

---

## 产品简介

### 什么是 NAS-OS？

NAS-OS 是一款基于 Go 语言开发的家用/小型企业 NAS（网络附加存储）操作系统。它提供：

- 💾 **btrfs 存储管理** - 卷、子卷、快照、RAID 支持
- 🌐 **Web 管理界面** - 简洁易用的可视化操作
- 📁 **文件共享** - SMB/CIFS、NFS 协议支持
- 👥 **多用户权限** - 细粒度的访问控制
- 📊 **系统监控** - 磁盘健康、空间预警
- 🐳 **Docker 集成** - 容器应用支持

### 核心特性

| 特性 | 说明 | 状态 |
|------|------|------|
| btrfs 卷管理 | 创建、删除、扩容存储卷 | ✅ |
| 子卷管理 | 灵活的数据隔离与管理 | ✅ |
| 快照功能 | 时间点数据备份与恢复 | ✅ |
| RAID 支持 | single/raid0/raid1/raid5/raid6/raid10 | ✅ |
| SMB 共享 | Windows/macOS 文件共享 | ✅ |
| NFS 共享 | Linux/Unix 文件共享 | ✅ |
| 用户管理 | 多用户、角色权限 | ✅ |
| 磁盘监控 | S.M.A.R.T.、健康检测 | ✅ |
| 空间告警 | 使用率阈值预警 | ✅ |
| Docker 支持 | 容器应用管理 | ✅ |

### 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Web 管理界面                            │
│                   (浏览器访问 http://:8080)                  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      RESTful API                            │
│                  (internal/web/handler.go)                  │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│  存储管理模块  │    │  文件共享模块  │    │  用户管理模块  │
│  (btrfs ops)  │    │ (SMB/NFS)     │    │  (RBAC)       │
└───────────────┘    └───────────────┘    └───────────────┘
        │
        ▼
┌───────────────┐
│  btrfs 文件系统 │
│  (Linux Kernel)│
└───────────────┘
```

---

## 快速开始

### 5 分钟上手指南

#### 步骤 1：下载与安装

```bash
# 下载最新版本（根据你的系统架构）
# AMD64 (Intel/AMD)
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7 (Raspberry Pi 3)
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd
```

#### 步骤 2：启动服务

```bash
# 直接启动（开发测试）
sudo nasd

# 或作为系统服务启动（生产环境）
sudo systemctl enable nas-os
sudo systemctl start nas-os
```

#### 步骤 3：访问 Web 界面

打开浏览器访问：`http://<服务器 IP>:8080`

**默认登录凭据**：
- 用户名：`admin`
- 密码：`admin123`

⚠️ **首次登录后请立即修改默认密码！**

#### 步骤 4：创建第一个存储卷

1. 登录 Web 界面
2. 点击「存储管理」>「创建卷」
3. 选择磁盘设备（如 `/dev/sdb1`）
4. 输入卷名称（如 `data`）
5. 选择 RAID 配置（单盘选 `single`）
6. 点击「创建」

#### 步骤 5：创建共享文件夹

1. 点击「文件共享」>「SMB 共享」
2. 点击「添加共享」
3. 选择路径（如 `/mnt/data/public`）
4. 设置共享名称（如 `public`）
5. 选择权限（公开/私有）
6. 点击「保存」

#### 步骤 6：从客户端访问

**Windows**:
```
\\<服务器 IP>\public
```

**macOS**:
```
Finder > 前往 > 连接服务器 > smb://<服务器 IP>/public
```

**Linux**:
```bash
sudo mount -t cifs //<服务器 IP>/public /mnt/nas -o username=guest
```

---

## 安装与配置

### 系统要求

#### 硬件要求

| 组件 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 双核 1.5GHz | 四核 2.0GHz+ |
| 内存 | 2GB | 4GB+ |
| 系统盘 | 16GB | 64GB+ SSD |
| 数据盘 | 任意容量 | 根据需求 |
| 网络 | 百兆以太网 | 千兆以太网 |

#### 软件要求

| 项目 | 要求 |
|------|------|
| 操作系统 | Linux (Ubuntu 20.04+, Debian 11+, Rocky Linux 8+) |
| 内核版本 | 5.10+ |
| 文件系统 | btrfs-progs |
| 可选组件 | samba, nfs-kernel-server, docker.io |

### 安装方式

#### 方式一：一键安装脚本（推荐）

```bash
# 下载并执行安装脚本
curl -fsSL https://raw.githubusercontent.com/nas-os/nasd/main/scripts/install.sh | sudo bash

# 安装脚本会自动：
# 1. 检查系统依赖
# 2. 安装 btrfs、samba、nfs 等工具
# 3. 下载最新版本的 nasd 二进制文件
# 4. 创建系统服务
# 5. 生成默认配置文件
```

#### 方式二：Docker 部署

适合开发测试或不想直接安装到系统的用户。

```bash
# 1. 拉取镜像
docker pull nas-os/nasd:v1.0.0

# 2. 运行容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  --privileged \
  nas-os/nasd:v1.0.0

# 3. 查看日志
docker logs -f nasd
```

#### 方式三：手动编译安装

适合开发者或需要自定义构建的用户。

```bash
# 1. 安装 Go 1.21+
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 2. 克隆项目
git clone https://github.com/nas-os/nasd.git
cd nasd

# 3. 编译
go mod tidy
go build -o nasd ./cmd/nasd

# 4. 安装到系统
sudo cp nasd /usr/local/bin/
sudo chmod +x /usr/local/bin/nasd
```

### 配置文件

配置文件位置：`/etc/nas-os/config.yaml`

```yaml
# 服务器配置
server:
  port: 8080              # Web UI 端口
  host: 0.0.0.0           # 监听地址
  tls_enabled: false      # 是否启用 HTTPS
  tls_cert: /etc/nas-os/cert.pem
  tls_key: /etc/nas-os/key.pem

# 存储配置
storage:
  mount_base: /mnt        # 存储挂载点基础路径
  auto_scrub: true        # 自动数据校验
  scrub_schedule: "0 3 * * 0"  # 每周日凌晨 3 点

# SMB 共享配置
smb:
  enabled: true           # 是否启用 SMB
  workgroup: WORKGROUP    # 工作组名称
  min_protocol: SMB2      # 最低协议版本

# NFS 共享配置
nfs:
  enabled: true           # 是否启用 NFS
  allowed_networks:       # 允许访问的网络
    - 192.168.1.0/24

# 用户配置
users:
  default_role: viewer    # 新用户默认角色
  password_min_length: 6  # 密码最小长度

# 日志配置
logging:
  level: info             # 日志级别：debug/info/warn/error
  file: /var/log/nas-os/nasd.log
  max_size: 100           # 单文件最大大小 (MB)
  max_backups: 5          # 保留的旧日志文件数
```

### 防火墙配置

```bash
# Ubuntu/Debian (UFW)
sudo ufw allow 8080/tcp    # Web UI
sudo ufw allow 445/tcp     # SMB
sudo ufw allow 2049/tcp    # NFS
sudo ufw allow 111/tcp     # RPC
sudo ufw allow 111/udp     # RPC
sudo ufw enable

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=445/tcp
sudo firewall-cmd --permanent --add-port=2049/tcp
sudo firewall-cmd --permanent --add-port=111/tcp
sudo firewall-cmd --permanent --add-port=111/udp
sudo firewall-cmd --reload
```

---

## 存储管理

### 卷管理

#### 创建卷

**Web 界面**：
1. 点击「存储管理」>「卷管理」
2. 点击「创建卷」
3. 选择磁盘设备
4. 输入卷名称
5. 选择 RAID 配置
6. 点击「创建」

**命令行**：
```bash
# 创建单盘卷
sudo nasctl volume create data /dev/sdb1

# 创建 RAID1 卷（需要 2 块磁盘）
sudo nasctl volume create data /dev/sdb1 /dev/sdc1 --profile raid1

# 创建 RAID5 卷（需要 3+ 块磁盘）
sudo nasctl volume create data /dev/sdb1 /dev/sdc1 /dev/sdd1 --profile raid5
```

**RAID 配置说明**：

| 配置 | 最少设备 | 容错 | 利用率 | 适用场景 |
|------|----------|------|--------|----------|
| `single` | 1 | 0 | 100% | 单盘、测试环境 |
| `raid0` | 2 | 0 | 100% | 高性能、非关键数据 |
| `raid1` | 2 | 1 | 50% | 高可靠、小容量 |
| `raid10` | 4 | 1 | 50% | 高性能 + 高可靠 |
| `raid5` | 3 | 1 | (n-1)/n | 平衡性能与容量 |
| `raid6` | 4 | 2 | (n-2)/n | 高可靠、大容量 |

#### 查看卷信息

```bash
# 列出所有卷
sudo nasctl volume list

# 查看卷详情
sudo nasctl volume show data

# 查看卷使用情况
sudo nasctl volume usage data
```

**详情输出示例**：
```
卷名称：data
UUID: abc-123-def-456
设备：/dev/sdb1, /dev/sdc1
RAID 配置：raid1
总容量：2.0 TB
已使用：500 GB
可用：1.5 TB
状态：健康
挂载点：/mnt/data
```

#### 扩容卷

```bash
# 添加新设备到卷
sudo nasctl volume add-device data /dev/sdd1

# 在线扩容（btrfs 自动平衡）
sudo btrfs filesystem resize max /mnt/data
```

#### 删除卷

⚠️ **警告：删除卷会永久丢失所有数据！**

```bash
# 删除卷（需要先卸载）
sudo umount /mnt/data
sudo nasctl volume delete data

# 强制删除（慎用）
sudo nasctl volume delete data --force
```

### 子卷管理

子卷是 btrfs 文件系统中的独立命名空间，可用于：
- 隔离不同类型的数据（如 docker、media、backup）
- 独立挂载和配额管理
- 创建独立快照

#### 创建子卷

```bash
# 创建子卷
sudo nasctl subvolume create data docker
sudo nasctl subvolume create data media
sudo nasctl subvolume create data backup

# 查看子卷列表
sudo nasctl subvolume list data
```

#### 挂载子卷

```bash
# 创建挂载点
sudo mkdir -p /mnt/data/docker

# 挂载子卷
sudo mount -o subvol=docker /dev/sdb1 /mnt/data/docker

# 永久挂载（/etc/fstab）
echo "/dev/sdb1 /mnt/data/docker btrfs subvol=docker 0 0" | sudo tee -a /etc/fstab
```

#### 删除子卷

```bash
# 删除空子卷
sudo nasctl subvolume delete data docker

# 删除非空子卷（需要先清空内容）
sudo rm -rf /mnt/data/docker/*
sudo nasctl subvolume delete data docker
```

### 快照管理

快照是子卷在某一时间点的只读副本，可用于：
- 数据备份
- 系统还原点
- 测试环境

#### 创建快照

```bash
# 创建子卷快照
sudo nasctl snapshot create data docker backup-docker-2026-03-11

# 创建整个卷的快照（递归）
sudo nasctl snapshot create data --recursive backup-full-2026-03-11
```

#### 查看快照

```bash
# 列出快照
sudo nasctl snapshot list data

# 查看快照详情
sudo nasctl snapshot show data backup-docker-2026-03-11
```

#### 恢复快照

```bash
# 方法 1：从快照创建新子卷
sudo nasctl snapshot restore data backup-docker-2026-03-11 docker-restored

# 方法 2：替换原子卷（会丢失当前数据）
sudo nasctl subvolume delete data docker
sudo nasctl snapshot restore data backup-docker-2026-03-11 docker
```

#### 删除快照

```bash
sudo nasctl snapshot delete data backup-docker-2026-03-11
```

### 数据维护

#### 数据平衡（Balance）

平衡操作重新分布数据块，优化存储性能。

```bash
# 启动平衡
sudo nasctl balance start data

# 查看平衡状态
sudo nasctl balance status data

# 停止平衡
sudo nasctl balance stop data
```

#### 数据校验（Scrub）

校验操作检测并修复数据静默错误。

```bash
# 启动校验
sudo nasctl scrub start data

# 查看校验状态
sudo nasctl scrub status data

# 校验统计
sudo btrfs scrub status /mnt/data
```

**建议**：配置自动校验，每周执行一次。

```yaml
# config.yaml
storage:
  auto_scrub: true
  scrub_schedule: "0 3 * * 0"  # 每周日凌晨 3 点
```

---

## 文件共享

### SMB/CIFS 共享

SMB 是 Windows 和 macOS 最常用的文件共享协议。

#### 启用 SMB 服务

```bash
# 启用 SMB
sudo nasctl smb enable

# 查看 SMB 状态
sudo nasctl smb status

# 重启 SMB 服务
sudo nasctl smb restart
```

#### 创建 SMB 共享

**Web 界面**：
1. 点击「文件共享」>「SMB 共享」
2. 点击「添加共享」
3. 填写信息：
   - 共享名称：`public`
   - 路径：`/mnt/data/public`
   - 描述：`公共共享文件夹`
   - 权限：公开/私有
4. 点击「保存」

**命令行**：
```bash
# 创建公开共享（无需密码）
sudo nasctl smb share add public /mnt/data/public --guest

# 创建私有共享（需要认证）
sudo nasctl smb share add private /mnt/data/private --no-guest

# 创建只读共享
sudo nasctl smb share add readonly /mnt/data/readonly --read-only
```

#### 管理共享权限

```bash
# 添加用户权限
sudo nasctl smb permission add public john --read-write
sudo nasctl smb permission add public guest --read-only

# 移除用户权限
sudo nasctl smb permission remove public guest

# 查看权限列表
sudo nasctl smb permission list public
```

#### 从客户端访问

**Windows**：
1. 打开文件资源管理器
2. 地址栏输入：`\\<服务器 IP>\public`
3. 按提示输入凭据（如需）

**macOS**：
1. Finder > 前往 > 连接服务器
2. 输入：`smb://<服务器 IP>/public`
3. 点击「连接」

**Linux**：
```bash
# 临时挂载
sudo mount -t cifs //<服务器 IP>/public /mnt/nas -o username=guest

# 永久挂载（/etc/fstab）
//<服务器 IP>/public /mnt/nas cifs guest,uid=1000,gid=1000 0 0
```

### NFS 共享

NFS 是 Linux/Unix 系统常用的文件共享协议。

#### 启用 NFS 服务

```bash
# 启用 NFS
sudo nasctl nfs enable

# 查看 NFS 状态
sudo nasctl nfs status

# 重启 NFS 服务
sudo nasctl nfs restart
```

#### 创建 NFS 共享

**Web 界面**：
1. 点击「文件共享」>「NFS 共享」
2. 点击「添加共享」
3. 填写信息：
   - 路径：`/mnt/data/backup`
   - 允许的网络：`192.168.1.0/24`
   - 选项：`rw,sync,no_subtree_check`
4. 点击「保存」

**命令行**：
```bash
# 创建 NFS 共享
sudo nasctl nfs share add /mnt/data/backup 192.168.1.0/24 --options rw,sync,no_subtree_check

# 允许多个网络
sudo nasctl nfs share add /mnt/data/media 192.168.1.0/24,192.168.2.0/24
```

#### NFS 共享选项

| 选项 | 说明 |
|------|------|
| `rw` | 读写权限 |
| `ro` | 只读权限 |
| `sync` | 同步写入（数据安全） |
| `async` | 异步写入（性能更好） |
| `no_subtree_check` | 禁用子树检查（提高性能） |
| `no_root_squash` | 允许 root 访问（不安全） |
| `root_squash` | 将 root 映射为匿名（默认） |

#### 从客户端访问

**Linux 客户端**：
```bash
# 查看可用共享
showmount -e <服务器 IP>

# 临时挂载
sudo mount -t nfs <服务器 IP>:/mnt/data/backup /mnt/nas

# 永久挂载（/etc/fstab）
<服务器 IP>:/mnt/data/backup /mnt/nas nfs defaults 0 0
```

---

## 用户管理

### 用户角色

| 角色 | 权限 |
|------|------|
| `admin` | 全部权限（系统管理、用户管理、存储管理） |
| `editor` | 读写共享文件，不能管理系统配置 |
| `viewer` | 只读访问共享文件 |

### 创建用户

**Web 界面**：
1. 点击「用户管理」>「添加用户」
2. 填写信息：
   - 用户名：`john`
   - 密码：`••••••••`
   - 确认密码：`••••••••`
   - 角色：`editor`
   - 邮箱（可选）：`john@example.com`
3. 点击「创建」

**命令行**：
```bash
# 创建用户
sudo nasctl user add john --password --role editor --email john@example.com

# 创建管理员
sudo nasctl user add admin2 --password --role admin
```

### 管理用户

```bash
# 列出所有用户
sudo nasctl user list

# 查看用户详情
sudo nasctl user show john

# 修改用户信息
sudo nasctl user modify john --role admin --email newemail@example.com

# 修改密码
sudo nasctl user password john

# 禁用用户
sudo nasctl user disable john

# 启用用户
sudo nasctl user enable john

# 删除用户
sudo nasctl user delete john
```

### 目录权限（ACL）

```bash
# 设置目录 ACL
sudo nasctl acl set /mnt/data/media --user john --permission rw
sudo nasctl acl set /mnt/data/media --user guest --permission r

# 查看 ACL
sudo nasctl acl get /mnt/data/media

# 移除 ACL
sudo nasctl acl remove /mnt/data/media --user guest
```

---

## 系统监控

### 系统状态

```bash
# 系统概览
sudo nasctl status

# 输出示例：
# NAS-OS v1.0.0
# 运行时间：15 天 3 小时
# 卷：2 个（data: 50% 使用，backup: 20% 使用）
# 共享：SMB 3 个，NFS 2 个
# 用户：5 个
# 健康状态：良好
```

### 磁盘健康

```bash
# 查看磁盘 S.M.A.R.T. 信息
sudo nasctl disk health

# 查看特定磁盘
sudo smartctl -a /dev/sdb

# 磁盘温度
sudo hddtemp /dev/sdb
```

### 空间监控

```bash
# 查看所有卷使用情况
sudo nasctl storage usage

# 查看特定卷
sudo btrfs filesystem usage /mnt/data
```

### 告警配置

**Web 界面**：
1. 点击「系统设置」>「告警」
2. 配置阈值：
   - 空间警告：80%
   - 空间严重：95%
   - 磁盘温度：60°C
3. 配置通知方式：
   - 邮件通知
   - Webhook 通知

**命令行**：
```bash
# 设置空间告警阈值
sudo nasctl alert threshold set --usage-warning 80 --usage-critical 95

# 添加邮件通知
sudo nasctl alert notification add --type email --to admin@example.com

# 添加 Webhook 通知
sudo nasctl alert notification add --type webhook --url https://hooks.slack.com/xxx

# 查看告警历史
sudo nasctl alert history

# 测试告警
sudo nasctl alert test
```

### 日志查看

```bash
# 查看系统日志
journalctl -u nas-os -n 50

# 实时查看日志
journalctl -u nas-os -f

# 查看特定时间段日志
journalctl -u nas-os --since "2026-03-10" --until "2026-03-11"

# 查看 NAS-OS 应用日志
sudo tail -f /var/log/nas-os/nasd.log
```

---

## 备份与恢复

### 配置备份

```bash
# 导出配置
sudo nasctl config export > nas-config-backup.yaml

# 查看配置
cat nas-config-backup.yaml

# 导入配置
sudo nasctl config import nas-config-backup.yaml
```

### 数据备份策略

#### 本地快照备份

```bash
# 创建每日快照
sudo nasctl snapshot create data daily-$(date +%Y%m%d)

# 保留最近 7 天的快照
sudo nasctl snapshot prune data --keep-daily 7
```

#### 远程备份

```bash
# 使用 rsync 备份到远程服务器
rsync -avz --delete /mnt/data/ user@backup-server:/backup/data/

# 使用 rclone 备份到云存储
rclone sync /mnt/data remote:bucket/data --progress
```

### 系统恢复

#### 从快照恢复

```bash
# 1. 列出可用快照
sudo nasctl snapshot list data

# 2. 恢复快照
sudo nasctl snapshot restore data daily-20260310 docker

# 3. 验证数据
ls -la /mnt/data/docker/
```

#### 从配置备份恢复

```bash
# 1. 停止服务
sudo systemctl stop nas-os

# 2. 恢复配置
sudo cp nas-config-backup.yaml /etc/nas-os/config.yaml

# 3. 启动服务
sudo systemctl start nas-os

# 4. 验证配置
sudo nasctl config show
```

---

## 故障排查

### 服务无法启动

```bash
# 1. 检查服务状态
sudo systemctl status nas-os

# 2. 查看详细日志
journalctl -u nas-os -n 100 --no-pager

# 3. 检查端口占用
sudo ss -tulpn | grep 8080

# 4. 测试配置文件
sudo nasd --config /etc/nas-os/config.yaml --test

# 5. 手动启动调试
sudo nasd --debug
```

**常见问题**：
- **端口被占用**：修改 config.yaml 中的端口或停止占用服务
- **配置文件错误**：检查 YAML 格式和路径
- **权限不足**：确保以 root 运行

### 无法访问 Web 界面

```bash
# 1. 检查服务是否运行
sudo systemctl is-active nas-os

# 2. 检查防火墙
sudo ufw status
sudo firewall-cmd --list-all

# 3. 测试本地访问
curl http://localhost:8080/api/v1/health

# 4. 检查监听地址
sudo ss -tulpn | grep nasd
```

### 磁盘无法识别

```bash
# 1. 列出所有磁盘
lsblk

# 2. 检查 btrfs 支持
btrfs --version

# 3. 查看磁盘详细信息
sudo fdisk -l
sudo blkid

# 4. 检查 dmesg 日志
dmesg | grep -i sd
```

### SMB/NFS 共享问题

```bash
# SMB 测试
smbclient -L //<服务器 IP> -U%

# NFS 测试
showmount -e <服务器 IP>

# 检查服务状态
sudo systemctl status smbd nmbd    # SMB
sudo systemctl status nfs-server   # NFS

# 重启共享服务
sudo systemctl restart smbd nmbd
sudo systemctl restart nfs-server
sudo systemctl restart nas-os
```

### 性能问题

```bash
# 1. 检查系统资源
top
htop
free -h

# 2. 检查磁盘 IO
iostat -x 1
iotop

# 3. 检查网络
iftop
nethogs

# 4. 优化建议
# - 使用 SSD 作为系统盘
# - 启用 btrfs 压缩
# - 调整挂载选项
# - 限制并发连接数
```

---

## 附录

### A. 命令行工具参考

#### nasctl 命令

```bash
# 存储管理
nasctl volume list|create|show|delete|add-device
nasctl subvolume list|create|delete
nasctl snapshot list|create|restore|delete
nasctl balance start|stop|status
nasctl scrub start|status

# 文件共享
nasctl smb enable|disable|status|restart
nasctl smb share add|remove|list
nasctl nfs enable|disable|status|restart
nasctl nfs share add|remove|list

# 用户管理
nasctl user list|add|show|modify|delete|disable|enable
nasctl user password
nasctl acl set|get|remove

# 系统管理
nasctl status
nasctl config show|export|import
nasctl alert threshold|notification|history|test
nasctl disk health
nasctl storage usage
```

### B. API 端点概览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/volumes | 获取卷列表 |
| POST | /api/v1/volumes | 创建卷 |
| GET | /api/v1/volumes/:name | 获取卷详情 |
| DELETE | /api/v1/volumes/:name | 删除卷 |
| GET | /api/v1/shares/smb | 获取 SMB 共享列表 |
| POST | /api/v1/shares/smb | 创建 SMB 共享 |
| GET | /api/v1/shares/nfs | 获取 NFS 共享列表 |
| POST | /api/v1/shares/nfs | 创建 NFS 共享 |
| GET | /api/v1/users | 获取用户列表 |
| POST | /api/v1/users | 创建用户 |
| GET | /api/v1/system/health | 健康检查 |

完整 API 文档请查看 [API_REFERENCE_v1.0.md](API_REFERENCE_v1.0.md)

### C. 文件路径参考

| 路径 | 说明 |
|------|------|
| `/etc/nas-os/config.yaml` | 主配置文件 |
| `/var/log/nas-os/nasd.log` | 应用日志 |
| `/var/lib/nas-os/` | 数据目录 |
| `/mnt/` | 默认挂载点 |
| `/usr/local/bin/nasd` | 主程序 |
| `/usr/local/bin/nasctl` | 命令行工具 |

### D. 获取帮助

- 📖 **完整文档**: https://docs.nas-os.dev
- 🐛 **报告问题**: https://github.com/nas-os/nasd/issues
- 💬 **社区讨论**: https://github.com/nas-os/nasd/discussions
- 📧 **技术支持**: support@nas-os.dev

---

*用户手册版本：v1.0.0 | 最后更新：2026-03-11*
