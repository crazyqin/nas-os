# NAS-OS 常见问题 (FAQ)

**版本**: v2.98.0  
**更新日期**: 2026-03-16

---

## 📋 目录

- [安装与部署](#安装与部署)
- [存储管理](#存储管理)
- [文件共享](#文件共享)
- [用户与权限](#用户与权限)
- [备份与恢复](#备份与恢复)
- [监控与告警](#监控与告警)
- [性能优化](#性能优化)
- [安全配置](#安全配置)
- [网络配置](#网络配置)
- [容器与虚拟机](#容器与虚拟机)

---

## 安装与部署

### Q1: NAS-OS 支持哪些操作系统和架构？

**A**: NAS-OS 支持以下平台：

| 操作系统 | 架构 | 支持状态 |
|----------|------|----------|
| Linux (Ubuntu 22.04+) | AMD64 | ✅ 完全支持 |
| Linux (Debian 12+) | AMD64 | ✅ 完全支持 |
| Linux (Ubuntu 22.04+) | ARM64 | ✅ 完全支持 |
| Linux (Debian 12+) | ARM64 | ✅ 完全支持 |
| Linux | ARMv7 | ✅ 完全支持 |

**推荐配置**：
- Orange Pi 5 / Raspberry Pi 5 (ARM64)
- 标准 x86_64 服务器

### Q2: 如何选择安装方式？

**A**: 根据使用场景选择：

| 安装方式 | 适用场景 | 优点 | 缺点 |
|----------|----------|------|------|
| 二进制安装 | 生产环境 | 性能最佳、资源占用低 | 需手动管理依赖 |
| Docker 部署 | 快速体验/测试 | 隔离性好、易于迁移 | 性能略有损耗 |
| 源码编译 | 开发调试 | 可定制性强 | 需要编译环境 |

**推荐**：生产环境使用二进制安装，测试环境使用 Docker。

### Q3: 安装后无法访问 Web 界面怎么办？

**A**: 按以下步骤排查：

```bash
# 1. 检查服务状态
sudo systemctl status nas-os

# 2. 检查端口监听
sudo ss -tulpn | grep 8080

# 3. 检查防火墙
sudo ufw status
sudo ufw allow 8080/tcp

# 4. 检查日志
sudo journalctl -u nas-os -n 50

# 5. 测试本地访问
curl http://localhost:8080/api/v1/health
```

### Q4: 如何升级到最新版本？

**A**: 升级步骤：

```bash
# 1. 备份配置
sudo cp -r /etc/nas-os /etc/nas-os.backup

# 2. 停止服务
sudo systemctl stop nas-os

# 3. 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.54.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 4. 启动服务
sudo systemctl start nas-os

# 5. 验证版本
nasd --version
```

---

## 存储管理

### Q5: 支持哪些文件系统？推荐使用哪种？

**A**: NAS-OS 主要支持 Btrfs 文件系统：

| 文件系统 | 支持状态 | 推荐场景 |
|----------|----------|----------|
| Btrfs | ✅ 完全支持 | 推荐使用，支持快照、压缩、RAID |
| ext4 | ⚠️ 有限支持 | 可作为数据盘，不支持快照 |
| XFS | ⚠️ 有限支持 | 大文件场景 |

**Btrfs 优势**：
- 原生快照支持
- 透明压缩 (zstd/lzo)
- 软件 RAID (0/1/5/6/10)
- 数据校验与自愈

### Q6: 如何创建 Btrfs 存储卷？

**A**: 使用 CLI 或 Web UI：

```bash
# CLI 方式
# 查看可用磁盘
sudo nasctl disk list

# 创建单盘卷
sudo nasctl volume create data --device /dev/sdb1

# 创建 RAID1 卷
sudo nasctl volume create secure --devices /dev/sdb,/dev/sdc --raid raid1

# 创建 RAID5 卷 (至少 3 块盘)
sudo nasctl volume create storage --devices /dev/sdb,/dev/sdc,/dev/sdd --raid raid5
```

Web UI 方式：存储 → 卷管理 → 创建卷

### Q7: 如何扩展存储卷容量？

**A**: 添加新磁盘到现有卷：

```bash
# 1. 添加新磁盘
sudo nasctl volume add-device data /dev/sdc

# 2. 重新平衡数据
sudo nasctl balance start data

# 3. 查看状态
sudo nasctl balance status data

# 4. 查看新容量
sudo nasctl volume show data
```

### Q8: 快照占用空间太大怎么办？

**A**: 优化快照策略：

```bash
# 1. 查看快照列表和大小
sudo btrfs subvolume list -s /data
sudo btrfs filesystem du -s /data/.snapshots/*

# 2. 删除旧快照
sudo nasctl snapshot delete data snapshot-20260301

# 3. 配置自动清理策略
sudo nasctl snapshot policy set data --retention "7d"

# 4. 启用压缩减少快照大小
sudo btrfs property set /data compression zstd
```

---

## 文件共享

### Q9: SMB 共享无法访问怎么办？

**A**: 按以下步骤排查：

```bash
# 1. 检查 Samba 服务
sudo systemctl status smbd

# 2. 检查共享配置
sudo cat /etc/samba/smb.conf
sudo testparm

# 3. 检查防火墙
sudo ufw allow samba

# 4. 测试本地连接
smbclient -L localhost -U admin

# 5. 查看连接状态
sudo smbstatus
```

**Windows 访问**：`\\NAS_IP\共享名`  
**macOS 访问**：`smb://NAS_IP/共享名`

### Q10: NFS 共享挂载失败怎么办？

**A**: 排查步骤：

```bash
# 服务端检查
# 1. 检查 NFS 服务
sudo systemctl status nfs-server

# 2. 检查导出配置
sudo exportfs -v

# 3. 重新加载配置
sudo exportfs -ra

# 客户端检查
# 1. 测试连通性
showmount -e NAS_IP

# 2. 挂载测试
sudo mount -t nfs NAS_IP:/share /mnt/test -vvv

# 3. 检查防火墙 (服务端)
sudo ufw allow nfs
sudo ufw allow mountd
sudo ufw allow rpc-bind
```

### Q11: 如何设置共享访问权限？

**A**: 使用 RBAC 权限系统：

```bash
# 1. 创建用户组
sudo nasctl group create family

# 2. 添加用户到组
sudo nasctl group add-user family zhangsan
sudo nasctl group add-user family lisi

# 3. 设置共享权限
sudo nasctl share set-permission media --group family --permission rw

# 4. 设置只读权限
sudo nasctl share set-permission backup --group family --permission r

# 5. 查看权限
sudo nasctl share list-permissions media
```

---

## 用户与权限

### Q12: 忘记管理员密码怎么办？

**A**: 使用命令行重置：

```bash
# 重置管理员密码
sudo nasctl user reset-password admin --password NewPassword123

# 或通过数据库直接修改 (SQLite)
sqlite3 /var/lib/nas-os/nas.db "UPDATE users SET password='新密码哈希' WHERE username='admin';"
```

### Q13: 如何配置 LDAP/AD 集成？

**A**: 配置企业目录服务：

```bash
# 添加 LDAP 服务器
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
    "enabled": true
  }'

# 测试连接
curl -X POST http://localhost:8080/api/v1/ldap/configs/company-ldap/test \
  -H "Authorization: Bearer TOKEN"
```

详细配置请参考 [LDAP 集成指南](LDAP-INTEGRATION.md)。

### Q14: 如何启用多因素认证 (MFA)？

**A**: 配置步骤：

1. 登录 Web 界面
2. 进入「设置」→「安全设置」
3. 点击「启用 MFA」
4. 使用 Google Authenticator 等应用扫描二维码
5. 输入验证码确认启用

```bash
# 命令行方式
sudo nasctl mfa enable admin
sudo nasctl mfa verify admin --code 123456
```

---

## 备份与恢复

### Q15: 如何配置定时备份？

**A**: 创建备份策略：

```bash
# 创建每日备份任务
sudo nasctl backup schedule create daily-backup \
  --source /data/important \
  --dest /backup/daily \
  --cron "0 2 * * *" \
  --incremental \
  --retention "7d"

# 创建每周完整备份
sudo nasctl backup schedule create weekly-full \
  --source /data \
  --dest /backup/weekly \
  --cron "0 3 * * 0" \
  --full \
  --retention "4w"

# 查看所有备份任务
sudo nasctl backup schedule list
```

### Q16: 如何恢复备份数据？

**A**: 恢复步骤：

```bash
# 查看备份历史
sudo nasctl backup history mydata

# 恢复最新版本
sudo nasctl backup restore mydata \
  --version latest \
  --dest /data/restored

# 恢复特定版本
sudo nasctl backup restore mydata \
  --version 2026-03-15 \
  --dest /data/restored

# 选择性恢复 (只恢复特定文件)
sudo nasctl backup restore mydata \
  --version latest \
  --files "/documents,/photos" \
  --dest /data/restored

# 预览恢复内容 (不实际执行)
sudo nasctl backup restore mydata --dry-run
```

### Q17: 如何将数据迁移到新 NAS？

**A**: 迁移方案：

```bash
# 方案一：rsync 迁移
rsync -avz --progress /data/ user@new-nas:/data/

# 方案二：Btrfs 快照发送
sudo btrfs send /data/.snapshot/latest | ssh new-nas "sudo btrfs receive /data/"

# 方案三：使用备份恢复
# 1. 在旧 NAS 创建备份
sudo nasctl backup create full-backup --source /data --dest /backup/full

# 2. 复制备份数据到新 NAS
rsync -avz /backup/full/ new-nas:/backup/full/

# 3. 在新 NAS 恢复
sudo nasctl backup restore full-backup --dest /data

# 4. 导入配置
sudo nasctl config import nas-config.yaml
```

---

## 监控与告警

### Q18: 如何配置邮件告警？

**A**: 配置 SMTP 通知：

```bash
# 1. 配置 SMTP 服务器
sudo nasctl notify add email \
  --address admin@example.com \
  --smtp-host smtp.example.com \
  --smtp-port 587 \
  --smtp-user nas-alerts@example.com \
  --smtp-password "password"

# 2. 测试邮件发送
sudo nasctl notify test email --address admin@example.com

# 3. 设置告警规则
sudo nasctl alert rule create disk-warning \
  --condition "disk_usage > 80" \
  --notify email \
  --cooldown 1h
```

### Q19: 如何查看系统运行状态？

**A**: 使用多种方式监控：

```bash
# CLI 方式
sudo nasctl status           # 系统概览
sudo nasctl resources        # 资源使用
sudo nasctl disk health      # 磁盘健康
sudo nasctl alerts list      # 活动告警

# API 方式
curl http://localhost:8080/api/v1/monitor/stats
curl http://localhost:8080/api/v1/monitor/alerts
curl http://localhost:8080/api/v1/dashboard/default

# Web UI
# 访问「仪表盘」页面查看实时监控
```

### Q20: Prometheus 指标在哪里？

**A**: 访问 Prometheus 指标端点：

```bash
# 获取 Prometheus 格式指标
curl http://localhost:8080/metrics

# 主要指标
# nas_disk_usage_bytes - 磁盘使用量
# nas_disk_io_bytes - 磁盘 IO
# nas_network_bytes - 网络流量
# nas_cpu_usage_percent - CPU 使用率
# nas_memory_usage_bytes - 内存使用
```

配置 Prometheus 抓取：
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'nas-os'
    static_configs:
      - targets: ['nas-ip:8080']
```

---

## 性能优化

### Q21: 如何优化存储性能？

**A**: 性能优化建议：

```bash
# 1. 启用 Btrfs 压缩 (节省空间、提升读取性能)
sudo btrfs property set /data compression zstd

# 2. 添加 SSD 缓存
sudo nasctl cache enable data --device /dev/nvme0n1

# 3. 调整 RAID 级别
# RAID0: 最高性能，无冗余
# RAID1: 高读取性能，50% 空间利用率
# RAID5: 平衡性能与冗余，需至少 3 盘

# 4. 定期维护
sudo btrfs balance start /data      # 数据平衡
sudo btrfs scrub start /data        # 数据校验
```

### Q22: 系统响应缓慢怎么办？

**A**: 排查和优化：

```bash
# 1. 检查资源使用
top -p $(pgrep nasd)
free -h
df -h

# 2. 检查磁盘 IO
iostat -x 1

# 3. 检查网络
ss -s

# 4. 优化内存
# 调整缓存大小
sudo nasctl config set cache.size 512MB

# 5. 查看慢请求日志
grep "slow" /var/log/nas-os/nasd.log | tail -20
```

### Q23: 如何调整并发配置？

**A**: 优化并发设置：

```yaml
# /etc/nas-os/config.yaml
performance:
  workers: 8              # 工作协程数 (建议 = CPU 核心数)
  max_connections: 1000   # 最大连接数
  read_buffer: 1MB        # 读缓冲区
  write_buffer: 1MB       # 写缓冲区
  
cache:
  enabled: true
  size: 512MB             # 缓存大小
  ttl: 300s               # 缓存过期时间

# 重启服务生效
sudo systemctl restart nas-os
```

---

## 安全配置

### Q24: 如何加固系统安全？

**A**: 安全配置清单：

```bash
# 1. 修改默认密码
sudo nasctl user reset-password admin

# 2. 启用 HTTPS
sudo nasctl ssl enable --letsencrypt

# 3. 配置防火墙
sudo ufw allow 8080/tcp   # Web UI
sudo ufw allow 445/tcp    # SMB
sudo ufw allow 2049/tcp   # NFS
sudo ufw enable

# 4. 启用审计日志
sudo nasctl audit enable

# 5. 配置登录策略
sudo nasctl auth policy set \
  --max-attempts 5 \
  --lockout-duration 30m \
  --password-min-length 12
```

### Q25: 如何查看安全审计日志？

**A**: 审计日志查看：

```bash
# 查看所有审计事件
sudo nasctl audit list

# 按类型过滤
sudo nasctl audit list --type auth
sudo nasctl audit list --type permission
sudo nasctl audit list --type admin

# 按时间过滤
sudo nasctl audit list --since "2026-03-01" --until "2026-03-15"

# 导出审计报告
sudo nasctl audit export --format csv > audit-report.csv
```

---

## 网络配置

### Q26: 如何配置静态 IP？

**A**: 网络配置：

```bash
# 方式一：使用 netplan (Ubuntu)
sudo nano /etc/netplan/00-installer-config.yaml

network:
  ethernets:
    eth0:
      addresses:
        - 192.168.1.100/24
      gateway4: 192.168.1.1
      nameservers:
        addresses:
          - 8.8.8.8
          - 8.8.4.4
  version: 2

sudo netplan apply

# 方式二：使用 nasctl
sudo nasctl network set-static eth0 \
  --ip 192.168.1.100 \
  --netmask 255.255.255.0 \
  --gateway 192.168.1.1 \
  --dns 8.8.8.8,8.8.4.4
```

### Q27: 如何配置 DDNS？

**A**: 动态 DNS 配置：

```bash
# 添加 DDNS 服务
sudo nasctl ddns add \
  --provider cloudflare \
  --domain nas.example.com \
  --token "your-api-token" \
  --interval 5m

# 支持的提供商
# - Cloudflare
# - Aliyun DNS
# - DNSPod
# - No-IP
# - DynDNS
```

### Q28: 如何配置端口转发？

**A**: 端口转发设置：

```bash
# 添加端口转发规则
sudo nasctl port-forward add \
  --name "web-server" \
  --protocol tcp \
  --external-port 8080 \
  --internal-ip 192.168.1.100 \
  --internal-port 80

# 查看规则
sudo nasctl port-forward list

# 启用/禁用规则
sudo nasctl port-forward enable web-server
sudo nasctl port-forward disable web-server
```

---

## 容器与虚拟机

### Q29: 如何管理 Docker 容器？

**A**: 容器操作：

```bash
# 列出容器
sudo nasctl container list

# 创建容器
sudo nasctl container create nginx \
  --image nginx:latest \
  --port 80:80 \
  --volume /data/www:/usr/share/nginx/html

# 启动/停止
sudo nasctl container start nginx
sudo nasctl container stop nginx

# 查看日志
sudo nasctl container logs nginx --follow

# 进入容器
sudo nasctl container exec nginx -- /bin/bash
```

### Q30: 如何创建虚拟机？

**A**: VM 管理：

```bash
# 创建虚拟机
sudo nasctl vm create ubuntu-server \
  --memory 4GB \
  --cpu 2 \
  --disk 50GB \
  --iso /data/iso/ubuntu-22.04.iso

# 启动虚拟机
sudo nasctl vm start ubuntu-server

# 连接控制台
sudo nasctl vm console ubuntu-server

# 创建快照
sudo nasctl vm snapshot ubuntu-server --name "before-update"

# 查看状态
sudo nasctl vm list
```

---

## 仪表板与监控

### Q31: 如何自定义监控仪表板？

**A**: NAS-OS v2.52.0+ 支持自定义仪表板布局：

```bash
# 获取仪表板配置
curl http://localhost:8080/api/v1/dashboard

# 创建自定义仪表板
curl -X POST http://localhost:8080/api/v1/dashboard \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的仪表板",
    "layout": "grid",
    "widgets": [
      {"type": "cpu", "position": {"x": 0, "y": 0, "w": 4, "h": 2}},
      {"type": "memory", "position": {"x": 4, "y": 0, "w": 4, "h": 2}},
      {"type": "disk", "position": {"x": 0, "y": 2, "w": 8, "h": 3}}
    ]
  }'
```

### Q32: 如何配置告警规则？

**A**: 使用告警规则引擎配置：

```bash
# 创建告警规则
curl -X POST http://localhost:8080/api/v1/alerts/rules \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "磁盘空间告警",
    "type": "disk_usage",
    "condition": {"operator": ">", "value": 80},
    "duration": "5m",
    "actions": [
      {"type": "email", "config": {"to": "admin@example.com"}},
      {"type": "webhook", "config": {"url": "https://hooks.example.com/alert"}}
    ]
  }'
```

---

## API 与集成

### Q33: 如何使用 API 进行批量操作？

**A**: 使用批量 API 端点：

```bash
# 批量创建用户
curl -X POST http://localhost:8080/api/v1/users/batch \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "users": [
      {"username": "user1", "password": "pass1", "role": "user"},
      {"username": "user2", "password": "pass2", "role": "user"},
      {"username": "user3", "password": "pass3", "role": "user"}
    ]
  }'

# 批量创建共享
curl -X POST http://localhost:8080/api/v1/shares/batch \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "shares": [
      {"name": "share1", "path": "/data/share1", "type": "smb"},
      {"name": "share2", "path": "/data/share2", "type": "smb"}
    ]
  }'
```

### Q34: 如何获取 API 文档？

**A**: 访问 Swagger UI：

- **Swagger UI**: `http://localhost:8080/swagger/`
- **OpenAPI JSON**: `http://localhost:8080/swagger/doc.json`
- **OpenAPI YAML**: `http://localhost:8080/swagger/doc.yaml`

---

## 成本与计费

### Q35: 如何查看存储成本分析？

**A**: 使用成本分析 API (v2.36.0+)：

```bash
# 获取成本分析报告
curl http://localhost:8080/api/v1/billing/cost-analysis \
  -H "Authorization: Bearer TOKEN"

# 获取用量统计
curl "http://localhost:8080/api/v1/billing/usage?period=current" \
  -H "Authorization: Bearer TOKEN"
```

### Q36: 如何设置预算警报？

**A**: 配置预算警报：

```bash
# 创建预算警报
curl -X POST http://localhost:8080/api/v1/budget/alerts \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "月度预算",
    "limit": 100.00,
    "thresholds": [50, 75, 90, 100],
    "notifications": [
      {"type": "email", "config": {"to": "admin@example.com"}}
    ]
  }'
```

---

## 📞 获取更多帮助

- **完整文档**: [docs/](.) 目录
- **故障排查**: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
- **API 文档**: [API_GUIDE.md](API_GUIDE.md) 或 Swagger UI
- **问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)

---

*最后更新：2026-03-16 | 版本：v2.80.0*