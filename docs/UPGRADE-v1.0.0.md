# NAS-OS v1.0.0 升级指南

**适用版本**: 从 v0.1.0+ 升级到 v1.0.0 GA  
**预计时间**: 15-30 分钟  
**风险等级**: 低（建议备份）

---

## 📋 升级前检查

### 1. 确认当前版本

```bash
# 查看当前版本
nasd --version

# 或通过 Web UI
# 访问：http://your-server:8080/api/v1/system/info
```

### 2. 检查系统要求

```bash
# 检查磁盘空间（至少需要 2GB 可用空间）
df -h /

# 检查内存（建议 2GB+）
free -h

# 检查 Docker（如果使用容器功能）
docker --version
```

### 3. 备份重要数据

**强烈建议**在升级前创建完整备份：

```bash
# 方式一：使用内置备份工具
nasd backup create --name pre-upgrade-v1.0.0

# 方式二：手动备份配置
sudo cp -r /etc/nas-os ~/nas-os-config-backup-$(date +%Y%m%d)

# 方式三：使用部署脚本
cd ~/clawd/deploy
./backup.sh backup pre-upgrade-v1.0.0 --skills --logs
```

### 4. 检查兼容性

| 升级路径 | 支持 | 说明 |
|----------|------|------|
| v0.3.0 → v1.0.0 | ✅ 完全支持 | 推荐升级路径 |
| v0.2.0 → v1.0.0 | ✅ 支持 | 需要配置迁移 |
| v0.1.0 → v1.0.0 | ✅ 支持 | 需要完整配置重建 |
| v0.0.x → v1.0.0 | ⚠️ 部分支持 | 建议全新安装 |

---

## 🚀 升级方法

### 方法一：自动升级（推荐）

适用于 v0.2.0+ 版本。

```bash
# 1. 检查可用更新
nasd update check

# 2. 执行升级（自动备份）
nasd update upgrade

# 3. 等待升级完成（约 5-10 分钟）
# 进度显示：
# [1/5] 下载新版本...
# [2/5] 验证完整性...
# [3/5] 备份当前版本...
# [4/5] 安装新版本...
# [5/5] 重启服务...

# 4. 验证升级
nasd --version
# 应显示：nasd version 1.0.0

# 5. 检查服务状态
sudo systemctl status nasd
# 应显示：active (running)
```

### 方法二：手动升级（通用）

适用于所有版本。

#### 步骤 1：停止服务

```bash
# systemd 管理
sudo systemctl stop nasd

# 或 Docker 运行
docker stop nasd
```

#### 步骤 2：下载新版本

```bash
# 创建临时目录
mkdir -p /tmp/nasd-upgrade
cd /tmp/nasd-upgrade

# 下载对应架构的二进制文件
# AMD64
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-amd64

# ARM64
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-arm64

# ARMv7
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nasd-linux-armv7

# 验证文件完整性（可选但推荐）
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/SHA256SUMS
sha256sum -c SHA256SUMS
```

#### 步骤 3：备份当前版本

```bash
# 备份当前二进制文件
sudo cp /usr/local/bin/nasd /usr/local/bin/nasd.backup

# 或使用版本化备份
sudo mv /usr/local/bin/nasd /usr/local/bin/nasd-v$(nasd --version | awk '{print $3}')
```

#### 步骤 4：安装新版本

```bash
# 替换二进制文件
sudo chmod +x nasd-linux-$(dpkg --print-architecture 2>/dev/null || echo "amd64")
sudo mv nasd-linux-* /usr/local/bin/nasd

# 验证权限
ls -l /usr/local/bin/nasd
# 应显示：-rwxr-xr-x
```

#### 步骤 5：迁移配置（如需要）

```bash
# v0.1.0 用户需要运行配置迁移
sudo nasd migrate config --from v0.1.0

# v0.2.0+ 通常自动迁移
# 检查迁移日志
cat /var/log/nas-os/migration.log
```

#### 步骤 6：启动服务

```bash
# systemd 启动
sudo systemctl start nasd

# 检查状态
sudo systemctl status nasd

# 查看日志
sudo journalctl -u nasd -f
```

### 方法三：Docker 升级

```bash
# 1. 拉取新镜像
docker pull nas-os/nasd:v1.0.0

# 2. 停止旧容器
docker stop nasd

# 3. 删除旧容器（数据卷保留）
docker rm nasd

# 4. 创建新容器（使用相同参数）
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 8443:8443 \
  -v /data:/data \
  -v /etc/nas-os:/etc/nas-os \
  -v /var/log/nas-os:/var/log/nas-os \
  --privileged \
  nas-os/nasd:v1.0.0

# 5. 验证
docker ps | grep nasd
docker logs nasd
```

### 方法四：使用部署脚本

```bash
cd ~/clawd/deploy

# 检查更新
./update.sh check

# 执行更新（自动备份）
./update.sh update

# 或更新并自动重启
./update.sh update --auto-restart

# 验证
systemctl status openclaw-gateway
```

---

## 🔄 配置迁移

### 自动迁移

v0.2.0+ 升级到 v1.0.0 时，配置会**自动迁移**。

```bash
# 查看迁移日志
cat /var/log/nas-os/migration.log

# 典型输出：
# [INFO] Starting migration from v0.2.0 to v1.0.0
# [INFO] Migrating user configuration... OK
# [INFO] Migrating share configuration... OK
# [INFO] Migrating network configuration... OK
# [INFO] Migration completed successfully
```

### 手动迁移（v0.1.0 用户）

```bash
# 1. 导出旧配置
nasd config export --output /tmp/old-config.yaml

# 2. 运行迁移工具
sudo nasd migrate config --from v0.1.0 --input /tmp/old-config.yaml

# 3. 检查迁移报告
cat /var/log/nas-os/migration-report.txt

# 4. 手动调整（如需要）
sudo nano /etc/nas-os/config.yaml

# 5. 重新加载配置
sudo nasd config reload
```

### 配置变更对照表

| v0.x 配置项 | v1.0.0 配置项 | 变更说明 |
|-------------|---------------|----------|
| `server.port` | `server.http.port` | 新增 HTTPS 端口配置 |
| `shares.smb` | `services.smb` | 归类到 services 模块 |
| `shares.nfs` | `services.nfs` | 归类到 services 模块 |
| - | `services.docker` | 新增 Docker 配置 |
| - | `services.appstore` | 新增应用商店配置 |
| `backup.path` | `backup.local.path` | 支持多备份目标 |
| - | `backup.cloud` | 新增云端备份配置 |
| `users` | `auth.users` | 归类到 auth 模块 |
| - | `auth.totp_enabled` | 新增双因素认证配置 |
| `monitor.enabled` | `monitoring.enabled` | 拼写修正 |
| - | `monitoring.alerts.email` | 细化告警配置 |
| - | `monitoring.alerts.wechat` | 新增微信告警 |
| `network.ddns` | `remote.ddns` | 归类到 remote 模块 |
| - | `remote.https` | 新增 HTTPS 配置 |

---

## ✅ 升级后验证

### 1. 版本检查

```bash
nasd --version
# 应显示：nasd version 1.0.0
```

### 2. 服务状态

```bash
# systemd
sudo systemctl status nasd

# 应显示：
# ● nasd.service - NAS-OS Daemon
#    Loaded: loaded (/etc/systemd/system/nasd.service; enabled)
#    Active: active (running)
```

### 3. Web UI 访问

```bash
# 访问管理界面
# http://your-server:8080

# 检查功能模块
# - 存储管理 ✓
# - 文件共享 ✓
# - 用户管理 ✓
# - 系统监控 ✓
# - 应用商店 ✓ (新增)
# - Docker 管理 ✓ (新增)
```

### 4. API 验证

```bash
# 系统信息
curl http://localhost:8080/api/v1/system/info | jq

# 应返回包含 version: "1.0.0" 的 JSON
```

### 5. 功能测试清单

- [ ] 登录 Web UI
- [ ] 查看存储状态
- [ ] 访问共享文件（SMB/NFS）
- [ ] 创建快照
- [ ] 查看监控图表
- [ ] 接收告警通知（如配置）
- [ ] 浏览应用商店
- [ ] 部署测试容器（如启用 Docker）

---

## 🔙 回滚指南

如果升级后遇到问题，可以回滚到之前的版本。

### 方法一：使用自动备份回滚

```bash
# 列出可用备份
nasd backup list

# 回滚到升级前备份
nasd backup restore pre-upgrade-v1.0.0

# 重启服务
sudo systemctl restart nasd
```

### 方法二：手动回滚

```bash
# 1. 停止服务
sudo systemctl stop nasd

# 2. 恢复旧版本二进制
sudo cp /usr/local/bin/nasd.backup /usr/local/bin/nasd

# 3. 恢复旧配置（如需要）
sudo cp -r /etc/nas-os.backup /etc/nas-os

# 4. 启动服务
sudo systemctl start nasd

# 5. 验证
nasd --version
```

### 方法三：Docker 回滚

```bash
# 1. 停止新容器
docker stop nasd
docker rm nasd

# 2. 启动旧版本容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/etc/nas-os \
  --privileged \
  nas-os/nasd:v0.3.0  # 使用旧版本号
```

### 回滚后检查

```bash
# 1. 验证版本
nasd --version

# 2. 检查数据完整性
nasd storage check

# 3. 测试共享访问
# 从客户端访问 SMB/NFS 共享

# 4. 报告问题
# 在 GitHub Issues 描述回滚原因
```

---

## 🐛 常见问题

### Q1: 升级后服务无法启动

**症状**: `sudo systemctl start nasd` 失败

**解决方案**:
```bash
# 1. 查看详细错误
sudo journalctl -u nasd -n 50 --no-pager

# 2. 检查配置文件语法
sudo nasd config validate

# 3. 检查端口占用
sudo lsof -i :8080

# 4. 恢复备份并重新升级
nasd backup restore pre-upgrade-v1.0.0
```

### Q2: 配置迁移失败

**症状**: 日志显示 `Migration failed: invalid configuration`

**解决方案**:
```bash
# 1. 备份当前配置
sudo cp /etc/nas-os/config.yaml /tmp/config.yaml.bak

# 2. 使用默认配置启动
sudo mv /etc/nas-os/config.yaml /etc/nas-os/config.yaml.old
sudo nasd config init

# 3. 手动迁移重要配置
sudo nano /etc/nas-os/config.yaml

# 4. 重新加载
sudo nasd config reload
```

### Q3: Web UI 无法访问

**症状**: 浏览器显示 `Connection refused`

**解决方案**:
```bash
# 1. 检查服务状态
sudo systemctl status nasd

# 2. 检查防火墙
sudo ufw status
sudo ufw allow 8080/tcp  # 如被阻止

# 3. 检查监听地址
sudo netstat -tlnp | grep nasd

# 4. 重启服务
sudo systemctl restart nasd
```

### Q4: Docker 容器无法启动

**症状**: 应用商店部署容器失败

**解决方案**:
```bash
# 1. 检查 Docker 服务
sudo systemctl status docker

# 2. 检查 Docker 权限
sudo usermod -aG docker nasd
sudo systemctl restart nasd

# 3. 检查存储空间
df -h /var/lib/docker

# 4. 查看容器日志
docker logs <container-id>
```

### Q5: 共享访问失败

**症状**: SMB/NFS 共享无法连接

**解决方案**:
```bash
# 1. 检查共享服务状态
sudo nasd shares status

# 2. 重启共享服务
sudo nasd shares restart

# 3. 检查防火墙
sudo ufw allow samba
sudo ufw allow nfs

# 4. 验证配置
sudo nasd config validate --section shares
```

---

## 📞 获取帮助

### 自助资源
- [故障排查指南](https://docs.nas-os.com/troubleshooting)
- [常见问题](https://docs.nas-os.com/faq)
- [社区论坛](https://community.nas-os.com)

### 联系支持
- **GitHub Issues**: https://github.com/nas-os/nasd/issues
- **Discord**: https://discord.gg/nas-os
- **邮件支持**: support@nas-os.com

### 报告 Bug
```markdown
**升级前版本**: v0.x.x
**升级方法**: 自动/手动/Docker
**错误信息**: [粘贴错误日志]
**复现步骤**: 
1. ...
2. ...

**系统信息**:
- OS: Ubuntu 22.04
- 架构：amd64
- 内存：4GB
```

---

## 📊 升级统计

| 指标 | 目标值 |
|------|--------|
| 升级成功率 | > 99% |
| 平均升级时间 | < 15 分钟 |
| 配置迁移成功率 | > 98% |
| 回滚率 | < 1% |
| 用户满意度 | > 4.5/5 |

---

## 🎉 升级完成

恭喜！您已成功升级到 NAS-OS v1.0.0 GA。

**下一步建议**:
1. 探索新的应用商店功能
2. 配置 Docker 容器
3. 设置自动备份计划
4. 启用双因素认证
5. 邀请家庭成员使用

**分享您的体验**:
- 在 Discord 分享升级体验
- 在 GitHub 给项目 Star
- 向朋友推荐 NAS-OS

---

*文档版本：1.0.0*  
*最后更新：2026-07-31*  
*维护团队：NAS-OS 工部*
