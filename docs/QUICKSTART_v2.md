# NAS-OS 快速入门指南 v2

**版本**: v2.284.0
**更新日期**: 2026-03-26
**适用对象**: 新用户、家庭用户、小型办公用户

---

## 🎯 5分钟快速上手

### 第一步：安装（2分钟）

#### 下载二进制文件

```bash
# AMD64 (x86_64) - 大多数PC和服务器
wget https://github.com/crazyqin/nas-os/releases/download/v2.284.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/v2.284.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7 (Raspberry Pi 3)
wget https://github.com/crazyqin/nas-os/releases/download/v2.284.0/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd
```

#### 或使用 Docker

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.284.0

docker run -d \
  --name nasd \
  --restart unless-stopped \
  --privileged \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.284.0
```

### 第二步：启动（1分钟）

```bash
# 直接运行
sudo nasd

# 或作为系统服务
sudo nasctl service install
sudo systemctl enable nas-os
sudo systemctl start nas-os
```

### 第三步：访问（1分钟）

1. 打开浏览器：`http://<服务器IP>:8080`
2. 默认登录：
   - 用户名：`admin`
   - 密码：`admin123`

⚠️ **首次登录后立即修改密码！**

---

## 📋 新用户必做清单

### ✅ 1. 修改默认密码

1. 点击右上角用户头像
2. 选择「账户设置」
3. 设置新密码

### ✅ 2. 创建存储卷

#### 通过 Web UI

1. 导航到「存储」→「卷管理」
2. 点击「创建卷」
3. 选择磁盘设备和 RAID 级别
4. 点击「创建」

#### 通过 CLI

```bash
# 查看可用磁盘
sudo nasctl disk list

# 创建 RAID1 卷（推荐）
sudo nasctl volume create data --devices /dev/sda,/dev/sdb --raid raid1
```

### ✅ 3. 创建共享文件夹

#### SMB 共享（Windows/macOS）

```bash
# 创建公共共享（无需登录）
sudo nasctl share create smb public --path /data/public --guest-ok

# 创建私有共享（需要登录）
sudo nasctl share create smb family --path /data/family
```

**访问方式**：
- Windows: `\\<服务器IP>\public`
- macOS: `smb://<服务器IP>/public`

#### NFS 共享（Linux）

```bash
# 创建 NFS 共享
sudo nasctl share create nfs backup --path /data/backup --network 192.168.1.0/24

# Linux 挂载
sudo mount <服务器IP>:/backup /mnt/backup
```

### ✅ 4. 创建用户

```bash
# 创建用户
sudo nasctl user create zhangsan --password SecurePass123

# 创建用户组
sudo nasctl group create family

# 添加用户到组
sudo nasctl group add-user family zhangsan

# 设置共享权限
sudo nasctl share set-permission family --group family --permission rw
```

---

## 🌟 核心功能速览

### 💾 存储管理

| 功能 | 说明 | 使用场景 |
|------|------|----------|
| 卷管理 | 创建/删除/扩展存储卷 | 初始化磁盘、扩容 |
| 快照 | 定时快照、手动快照 | 数据保护、误删恢复 |
| 存储分层 | 热/冷数据自动分层 | 性能优化、成本控制 |
| 数据去重 | 文件级/块级去重 | 节省存储空间 |

**快速创建快照**：

```bash
# 创建快照
sudo nasctl snapshot create data --name backup-$(date +%Y%m%d)

# 列出快照
sudo nasctl snapshot list data

# 恢复快照
sudo nasctl snapshot restore data backup-20260326
```

### 📁 文件管理

- **Web 文件管理器**: 拖拽上传、在线预览、批量操作
- **版本控制**: 自动保存历史版本，一键恢复
- **文件标签**: 标签分类，快速检索

### 🐳 容器管理

```bash
# 列出容器
sudo nasctl container list

# 创建容器
sudo nasctl container create nginx --image nginx:latest --port 80:80

# 启动/停止
sudo nasctl container start nginx
sudo nasctl container stop nginx

# 查看日志
sudo nasctl container logs nginx
```

### 🖥️ 虚拟机管理

```bash
# 创建虚拟机
sudo nasctl vm create ubuntu --cpu 2 --memory 4096 --disk 50

# 启动虚拟机
sudo nasctl vm start ubuntu

# 挂载 ISO
sudo nasctl vm attach-iso ubuntu --iso /path/to/ubuntu.iso
```

### 🤖 AI 功能

#### AI 相册 - 以文搜图

使用自然语言搜索照片：
- "红色汽车"
- "海边日落"
- "孩子的生日派对"

#### AI 数据脱敏

自动识别并脱敏敏感信息：
- 邮箱地址
- 手机号码
- 身份证号
- 信用卡号
- IP 地址

### ☁️ 云存储挂载

支持多云存储挂载为本地目录：

```bash
# 挂载阿里云 OSS
sudo nasctl cloud mount aliyun --bucket my-bucket --path /data/cloud/aliyun

# 挂载 Google Drive
sudo nasctl cloud mount gdrive --path /data/cloud/gdrive
```

---

## 🔒 安全特性

### WriteOnce 不可变存储

**独家功能** - 防勒索病毒、合规归档

```bash
# 锁定路径（创建不可变快照）
curl -X POST http://localhost:8080/api/v1/immutable \
  -H "Authorization: Bearer TOKEN" \
  -d '{"path": "/data/important", "duration": "30d"}'

# 检查防勒索保护
curl -X POST http://localhost:8080/api/v1/immutable/check-ransomware
```

### 热备盘自动切换

RAID 故障自动恢复：

```bash
# 添加热备盘
sudo nasctl volume add-spare data /dev/sdc

# 查看热备盘状态
sudo nasctl volume status data
```

### SSD 三级健康预警

```bash
# 查看 SSD 健康状态
sudo nasctl disk health /dev/nvme0n1

# 健康评分: 0-100
# 三级预警:
#   - 绿色: >70 健康
#   - 黄色: 40-70 注意
#   - 红色: <40 需更换
```

---

## 📊 监控与告警

### Web 界面监控

仪表盘实时显示：
- CPU、内存使用率
- 磁盘读写速度
- 网络流量
- 存储使用情况
- 系统运行时间

### 配置告警通知

```bash
# 邮件告警
sudo nasctl notify add email \
  --address admin@example.com \
  --events disk_warning,backup_failed,smart_alert

# Webhook 告警
sudo nasctl notify add webhook \
  --url https://hooks.slack.com/services/xxx \
  --events critical_alert
```

---

## 🆚 与竞品对比

### 为什么选择 NAS-OS？

| 特性 | NAS-OS | 飞牛fnOS | 群晖DSM | TrueNAS |
|------|:------:|:--------:|:-------:|:-------:|
| **价格** | 免费 | 免费 | 付费硬件 | 免费 |
| **WriteOnce防勒索** | ✅ 独家 | ❌ | ❌ | ❌ |
| **智能存储分层** | ✅ | ❌ | ✅ | ❌ |
| **AI以文搜图** | ✅ | ✅ | ✅ | ❌ |
| **多云挂载** | ✅ | ✅ | 有限 | ❌ |
| **内网穿透** | 🚧 开发中 | ✅ | ❌ | ❌ |
| **开源** | ✅ | 部分 | ❌ | ✅ |

---

## ❓ 常见问题

### Q: 支持哪些 RAID 级别？

支持 RAID0、RAID1、RAID5、RAID6、RAID10

### Q: 如何扩展存储？

1. 添加新磁盘
2. 在 Web UI 中选择「卷管理」→「扩展」
3. 选择新磁盘并确认

### Q: 数据如何备份？

```bash
# 快照备份
sudo nasctl snapshot create data --name backup-$(date +%Y%m%d)

# 远程复制
sudo nasctl replicate create data --target remote-nas --path /backup

# 云端备份
sudo nasctl cloud sync data --bucket backup-bucket
```

### Q: 忘记密码怎么办？

```bash
sudo nasctl user reset-password admin --password NewPassword123
```

### Q: 如何升级版本？

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.284.0/nasd-linux-amd64

# 停止服务
sudo systemctl stop nas-os

# 替换二进制文件
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 启动服务
sudo systemctl start nas-os
```

---

## 📚 进阶阅读

- 📖 [完整用户手册](USER_GUIDE.md)
- 🔧 [管理员指南](ADMIN_GUIDE_v2.5.0.md)
- 📡 [API 文档](API_GUIDE.md)
- 🚀 [部署指南](DEPLOYMENT_GUIDE_v2.5.0.md)
- 📊 [竞品分析](COMPETITOR_ANALYSIS.md)

---

## 💬 获取帮助

- 🐛 **问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- 💬 **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)
- 📦 **Docker 镜像**: [GHCR](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

---

**祝您使用愉快！** 🎉