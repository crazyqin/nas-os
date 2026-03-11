# NAS-OS v1.0.0 GA 发布说明

**发布日期**: 2026-07-31  
**版本类型**: GA (General Availability) - 生产就绪版本  
**前序版本**: v0.3.0 Beta

---

## 🎉 欢迎使用 NAS-OS 1.0.0

我们很高兴地宣布 NAS-OS v1.0.0 GA 正式发布！这是 NAS-OS 的第一个生产就绪版本，标志着从"可用工具"到"可信赖的家庭存储中心"的重要里程碑。

经过 4 个月的开发和社区测试，v1.0.0 带来了完整的核心功能、企业级安全性和出色的用户体验。无论您是 NAS 新手还是资深玩家，NAS-OS 都能满足您的需求。

---

## ✨ 核心亮点

### 1. 🐳 Docker 完整集成
- 原生 Docker 容器管理
- 预置应用模板（Nextcloud、Jellyfin、Transmission 等）
- 容器资源限制和监控
- 一键部署常用应用

### 2. 📦 应用商店（基础版）
- 精选 20+ 热门应用
- 一键安装和自动更新
- 应用沙箱隔离
- 社区应用提交支持

### 3. 💾 备份与恢复
- 整机备份到本地/云端
- 增量备份节省空间
- 定时备份计划
- 一键恢复系统

### 4. 🌐 远程访问
- 内置 DDNS 客户端
- 安全 HTTPS 访问
- 端口转发自动配置
- 移动端优化

### 5. 📱 移动端适配
- 响应式 Web UI
- iOS/Android 浏览器完美支持
- 触摸优化操作界面
- 离线消息通知

---

## 📋 完整功能清单

### 存储管理
- ✅ btrfs 卷管理（创建、删除、扩容）
- ✅ 子卷管理（配额、快照）
- ✅ 快照管理（定时、手动、恢复）
- ✅ RAID 支持（0/1/5/10）
- ✅ 磁盘健康监控（SMART）
- ✅ 空间使用告警

### 文件共享
- ✅ SMB/CIFS 共享（Windows/macOS/Linux）
- ✅ NFS 共享（Linux 客户端）
- ✅ AFP 共享（macOS 传统支持）
- ✅ WebDAV 共享
- ✅ 访问控制列表（ACL）
- ✅ 访客访问控制

### 用户与权限
- ✅ 多用户系统
- ✅ 用户组管理
- ✅ RBAC 权限模型
- ✅ 密码策略（强度、过期）
- ✅ 双因素认证（2FA）
- ✅ 登录审计日志

### 系统监控
- ✅ 实时资源监控（CPU/内存/网络）
- ✅ 磁盘健康监控
- ✅ 温度监控
- ✅ 告警通知（邮件/微信/钉钉）
- ✅ 历史数据图表

### 应用生态
- ✅ Docker 容器管理
- ✅ 应用商店（20+ 应用）
- ✅ 容器模板市场
- ✅ 应用自动更新
- ✅ 资源配额管理

### 备份与恢复
- ✅ 系统配置备份
- ✅ 数据增量备份
- ✅ 云端备份（Backblaze/S3/WebDAV）
- ✅ 定时备份计划
- ✅ 一键恢复

### 网络与安全
- ✅ DDNS 客户端（支持 10+ 服务商）
- ✅ HTTPS 证书自动续期（Let's Encrypt）
- ✅ 防火墙配置
- ✅ 端口转发
- ✅ 失败登录保护
- ✅ 安全审计日志

### Web 界面
- ✅ 现代化响应式设计
- ✅ 移动端完美适配
- ✅ 深色/浅色主题
- ✅ 多语言支持（中文/英文）
- ✅ 实时通知中心
- ✅ 快捷操作面板

---

## 🚀 快速开始

### 方式一：下载二进制文件

```bash
# AMD64 (x86_64)
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7 (Raspberry Pi 3, 旧款 ARM)
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd

# 验证安装
nasd --version
# 输出：nasd version 1.0.0

# 启动服务
sudo nasd start
```

### 方式二：Docker 部署

```bash
# 拉取镜像
docker pull nas-os/nasd:v1.0.0

# 运行容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/etc/nas-os \
  --privileged \
  nas-os/nasd:v1.0.0

# 访问管理界面
# http://your-server-ip:8080
```

### 方式三：一键安装脚本

```bash
curl -fsSL https://raw.githubusercontent.com/nas-os/nasd/v1.0.0/scripts/install.sh | sudo bash
```

---

## 📊 系统要求

### 最低配置
- **CPU**: 双核 1.5GHz
- **内存**: 2GB RAM
- **存储**: 10GB 系统盘 + 数据盘
- **网络**: 100Mbps 以太网

### 推荐配置
- **CPU**: 四核 2.0GHz+
- **内存**: 4GB+ RAM
- **存储**: SSD 系统盘 + HDD 数据盘
- **网络**: 1Gbps 以太网

### 支持的操作系统
- Ubuntu 20.04 / 22.04 / 24.04
- Debian 11 / 12
- CentOS 7 / 8 / Stream 9
- Rocky Linux 8 / 9
- AlmaLinux 8 / 9
- Raspberry Pi OS
- Orange Pi OS

### 支持的架构
- x86_64 (amd64)
- ARM64 (aarch64)
- ARMv7 (armhf)

---

## 🔧 默认配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| Web 端口 | 8080 | HTTP 管理界面 |
| HTTPS 端口 | 8443 | HTTPS 管理界面 |
| 数据目录 | /data | 默认数据存储路径 |
| 配置目录 | /etc/nas-os | 配置文件存储路径 |
| 日志目录 | /var/log/nas-os | 系统日志路径 |
| 备份目录 | /data/backups | 本地备份路径 |

---

## 📦 内置应用模板

v1.0.0 预置以下应用模板，一键部署：

### 文件与同步
- Nextcloud - 私有云盘
- Syncthing - 文件同步
- FileBrowser - Web 文件管理器

### 媒体服务器
- Jellyfin - 媒体库
- Plex - 媒体服务器
- Airsonic - 音乐流媒体

### 下载工具
- Transmission - BT 下载
- qBittorrent - BT 下载
- Aria2 - 多线程下载

### 开发与工具
- Code Server - VS Code 网页版
- GitLab - 代码托管
- Jenkins - CI/CD

### 智能家居
- Home Assistant - 智能家居中枢
- Mosquitto - MQTT 服务器

### 监控与安全
- Vaultwarden - 密码管理器
- Uptime Kuma - 服务监控

---

## 🛡️ 安全特性

### 认证与授权
- 强密码策略（最小长度 8，包含大小写/数字/特殊字符）
- 双因素认证（TOTP）
- Session 超时自动登出
- 失败登录锁定（5 次失败锁定 30 分钟）

### 数据安全
- HTTPS 强制（可选）
- 传输加密（TLS 1.3）
- 静态数据加密（可选）
- 安全启动验证

### 网络安全
- 内置防火墙
- 端口扫描防护
- DDoS 基础防护
- IP 黑白名单

### 审计与合规
- 完整操作日志
- 登录审计
- 文件访问日志
- 配置变更追踪

---

## 📈 性能基准

在推荐配置（四核 2.0GHz, 4GB RAM, SSD）下测试：

| 测试项目 | 结果 |
|----------|------|
| 冷启动时间 | < 5 秒 |
| Web UI 加载 | < 1 秒 |
| API 响应时间 (P95) | < 100ms |
| SMB 传输速度 (1Gbps) | 110 MB/s |
| NFS 传输速度 (1Gbps) | 105 MB/s |
| 并发用户支持 | 50+ |
| 容器启动时间 | < 3 秒 |

---

## 🐛 已知问题

### 轻微问题
1. **Safari 浏览器**: 在 iOS 15 及以下版本，部分图表渲染可能不完整（不影响功能）
2. **ARMv7 设备**: Docker 应用商店部分应用不支持 ARMv7 架构
3. **大文件上传**: Web 界面上传 >4GB 文件可能失败（建议使用 SMB/NFS）

### 计划修复
- v1.0.1 (2026-08-15): 修复 Safari 兼容性问题
- v1.1.0 (2026-09-30): 增加 ARMv7 应用支持

### 临时解决方案
- iOS 用户使用 Chrome 浏览器获得最佳体验
- ARMv7 用户可使用 Docker CLI 手动部署应用
- 大文件通过 SMB/NFS 共享传输

---

## 🆘 获取帮助

### 文档资源
- [用户指南](https://docs.nas-os.com/user-guide)
- [管理员手册](https://docs.nas-os.com/admin)
- [API 文档](https://docs.nas-os.com/api)
- [常见问题](https://docs.nas-os.com/faq)

### 社区支持
- **Discord**: https://discord.gg/nas-os
- **GitHub Issues**: https://github.com/nas-os/nasd/issues
- **论坛**: https://community.nas-os.com
- **QQ 群**: 123456789

### 商业支持
- 企业技术支持：support@nas-os.com
- 定制开发服务：contact@nas-os.com

---

## 🙏 致谢

感谢所有为 NAS-OS v1.0.0 做出贡献的社区成员：

- **核心开发团队**: 兵部、工部、吏部、礼部、刑部
- **测试团队**: 50+ 位 Beta 测试用户
- **文档贡献者**: 20+ 位社区志愿者
- **翻译贡献者**: 中文、英文、日文、德文翻译团队

特别感谢早期采用者和反馈提供者，你们的建议让 NAS-OS 变得更好。

---

## 📅 后续计划

### v1.0.1 (2026-08-15) - 补丁版本
- Safari 兼容性修复
- ARMv7 应用支持改进
- 性能优化和 Bug 修复

### v1.1.0 (2026-09-30) - 功能更新
- RAID 管理界面
- 存储池在线扩容
- 更多应用模板
- 媒体服务器深度集成

### v1.2.0 (2026-11-30) - 企业特性
- 双机热备
- 云同步（Backblaze/AWS S3）
- 虚拟机支持
- 高级网络配置

---

## 📄 许可证

NAS-OS v1.0.0 采用 MIT 许可证开源。

- 源代码：https://github.com/nas-os/nasd
- 许可证全文：https://github.com/nas-os/nasd/blob/v1.0.0/LICENSE

---

**开始您的 NAS 之旅**: https://nas-os.com  
**查看完整文档**: https://docs.nas-os.com  
**加入社区**: https://discord.gg/nas-os

*NAS-OS 团队 敬上*  
*2026-07-31*
