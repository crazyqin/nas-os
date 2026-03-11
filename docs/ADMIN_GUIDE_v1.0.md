# NAS-OS v1.0 管理员指南

> **版本**: v1.0.0  
> **发布日期**: 2026-03-11  
> **目标读者**: 系统管理员、运维工程师

---

## 目录

1. [系统架构](#系统架构)
2. [高级配置](#高级配置)
3. [安全管理](#安全管理)
4. [性能优化](#性能优化)
5. [监控与告警](#监控与告警)
6. [备份策略](#备份策略)
7. [维护操作](#维护操作)
8. [故障排查](#故障排查)
9. [升级与迁移](#升级与迁移)
10. [最佳实践](#最佳实践)

---

## 系统架构

### 组件概览

```
┌─────────────────────────────────────────────────────────────────┐
│                         客户端层                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │ Web 浏览器 │  │ SMB 客户端 │  │ NFS 客户端 │  │  API 调用  │       │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       NAS-OS 服务层                              │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    HTTP Server (8080)                    │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │   │
│  │  │ Auth    │  │ Volume  │  │ Share   │  │ User    │    │   │
│  │  │ Handler │  │ Handler │  │ Handler │  │ Handler │    │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                     核心服务层                           │   │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐           │   │
│  │  │ Storage   │  │ SMB/NFS   │  │ User/RBAC │           │   │
│  │  │ Manager   │  │ Service   │  │ Manager   │           │   │
│  │  └───────────┘  └───────────┘  └───────────┘           │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        系统层                                   │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐   │
│  │  btrfs    │  │  Samba    │  │  NFS      │  │  Systemd  │   │
│  │  Kernel   │  │  Daemon   │  │  Server   │  │  Service  │   │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 进程结构

```bash
# 查看 NAS-OS 相关进程
ps aux | grep -E 'nasd|smbd|nfsd'

# 输出示例：
# root     1234  0.1  0.5 123456 78901 ?  Ssl  10:00   1:23 /usr/local/bin/nasd
# root     1235  0.0  0.1  12345  2345 ?    S    10:00   0:05 /usr/sbin/smbd
# root     1236  0.0  0.1  12345  2345 ?    S    10:00   0:03 /usr/sbin/nmbd
# root     1237  0.0  0.0      0     0 ?    Z    10:00   0:00 [nfsd]
```

### 数据流

1. **Web 请求流**：
   ```
   浏览器 → HTTP 8080 → Auth Middleware → Handler → Service → btrfs/Samba/NFS
   ```

2. **文件访问流**：
   ```
   SMB 客户端 → 445 端口 → smbd → VFS → btrfs
   NFS 客户端 → 2049 端口 → nfsd → VFS → btrfs
   ```

---

## 高级配置

### 配置文件详解

主配置文件：`/etc/nas-os/config.yaml`

```yaml
# ==================== 服务器配置 ====================
server:
  port: 8080                    # Web UI 端口
  host: 0.0.0.0                 # 监听地址 (0.0.0.0 = 所有接口)
  
  # HTTPS 配置（生产环境推荐启用）
  tls_enabled: true
  tls_cert: /etc/nas-os/cert.pem
  tls_key: /etc/nas-os/key.pem
  
  # 会话配置
  session_timeout: 3600         # 会话超时（秒）
  max_connections: 100          # 最大并发连接数
  
  # CORS 配置（API 访问）
  cors_enabled: true
  cors_allowed_origins:
    - https://admin.example.com

# ==================== 存储配置 ====================
storage:
  mount_base: /mnt              # 存储挂载点基础路径
  
  # 自动维护
  auto_scrub: true              # 自动数据校验
  scrub_schedule: "0 3 * * 0"   # 每周日凌晨 3 点
  auto_balance: false           # 自动平衡（默认关闭，手动触发）
  
  # btrfs 优化
  compression: zstd             # 压缩算法 (zstd/lzo/gzip)
  commit_interval: 30           # 提交间隔（秒）
  
  # 配额管理
  quota_enabled: true
  default_quota: 1099511627776  # 默认配额 (1TB)

# ==================== SMB 共享配置 ====================
smb:
  enabled: true
  workgroup: WORKGROUP          # 工作组名称
  server_string: "NAS-OS Server"
  
  # 协议版本
  min_protocol: SMB2
  max_protocol: SMB3
  
  # 安全设置
  encrypt_transport: true       # 传输加密
  ntlm_auth: true               # NTLM 认证
  
  # 性能优化
  socket_options: "TCP_NODELAY IPTOS_LOWDELAY"
  read_raw: true
  write_raw: true
  
  # 日志
  log_level: 1                  # 0=最小，3=详细

# ==================== NFS 共享配置 ====================
nfs:
  enabled: true
  
  # 版本支持
  versions: ["3", "4"]
  
  # 网络限制
  allowed_networks:
    - 192.168.1.0/24
    - 10.0.0.0/8
  
  # 导出选项
  default_options: "rw,sync,no_subtree_check"
  
  # 线程数
  threads: 8

# ==================== 用户管理配置 ====================
users:
  # 密码策略
  password_min_length: 8
  password_require_uppercase: true
  password_require_number: true
  password_require_special: false
  password_history: 5           # 密码历史记录数
  password_max_age: 90          # 密码最大天数
  
  # 登录策略
  max_login_attempts: 5         # 最大登录尝试
  lockout_duration: 300         # 锁定时间（秒）
  
  # 默认设置
  default_role: viewer
  allow_guest: true             # 允许访客访问

# ==================== 监控告警配置 ====================
monitoring:
  # 指标收集
  metrics_enabled: true
  metrics_port: 9090            # Prometheus 指标端口
  
  # 健康检查
  health_check_interval: 60     # 健康检查间隔（秒）
  
  # 告警配置
  alerts:
    disk_usage_warning: 80      # 磁盘使用警告阈值 (%)
    disk_usage_critical: 95     # 磁盘使用严重阈值 (%)
    disk_temp_warning: 50       # 磁盘温度警告阈值 (°C)
    disk_temp_critical: 60      # 磁盘温度严重阈值 (°C)
    
  # 通知渠道
  notifications:
    email:
      enabled: true
      smtp_server: smtp.example.com
      smtp_port: 587
      smtp_user: alerts@example.com
      smtp_password: "${SMTP_PASSWORD}"  # 支持环境变量
      from: nas-os@example.com
      to:
        - admin@example.com
    
    webhook:
      enabled: true
      url: https://hooks.slack.com/services/xxx
      events:
        - disk_critical
        - service_down
        - backup_failed

# ==================== 日志配置 ====================
logging:
  level: info                   # debug/info/warn/error
  format: json                  # json/text
  
  # 文件日志
  file: /var/log/nas-os/nasd.log
  max_size: 100                 # MB
  max_backups: 5
  max_age: 30                   # 天
  compress: true
  
  # 系统日志
  syslog_enabled: true
  syslog_facility: local0

# ==================== 备份配置 ====================
backup:
  enabled: true
  
  # 本地快照
  snapshot:
    enabled: true
    schedule: "0 2 * * *"       # 每天凌晨 2 点
    retention_days: 7
  
  # 远程备份
  remote:
    enabled: false
    type: rsync                 # rsync/rclone
    destination: user@backup:/backup
    schedule: "0 3 * * 0"       # 每周日凌晨 3 点
```

### 环境变量

NAS-OS 支持通过环境变量覆盖配置：

```bash
# 敏感信息（推荐通过环境变量传递）
export NASD_SERVER_PORT=8080
export NASD_SMTP_PASSWORD=secret123
export NASD_DATABASE_URL=postgres://user:pass@localhost/nasd

# 启动时加载
sudo -E nasd
```

### 配置验证

```bash
# 测试配置文件
sudo nasd --config /etc/nas-os/config.yaml --test

# 输出示例：
# ✓ 配置文件语法正确
# ✓ 所有路径可访问
# ✓ 端口 8080 可用
# ✓ SMB 配置有效
# ✓ NFS 配置有效
# 配置验证通过
```

---

## 安全管理

### 认证与授权

#### RBAC 角色定义

| 角色 | 权限范围 |
|------|----------|
| `admin` | 全部权限（系统配置、用户管理、存储管理、共享管理） |
| `operator` | 存储管理、共享管理、监控查看 |
| `editor` | 读写共享文件、创建个人快照 |
| `viewer` | 只读访问共享文件 |

#### 配置 RBAC

```yaml
# config.yaml
users:
  roles:
    admin:
      permissions:
        - "*"
    operator:
      permissions:
        - "storage:*"
        - "shares:*"
        - "monitoring:read"
    editor:
      permissions:
        - "shares:read"
        - "shares:write"
        - "snapshots:create"
    viewer:
      permissions:
        - "shares:read"
```

### 网络安全

#### 防火墙配置

```bash
# Ubuntu/Debian (UFW)
sudo ufw default deny incoming
sudo ufw default allow outgoing

# 必需端口
sudo ufw allow 22/tcp         # SSH
sudo ufw allow 8080/tcp       # Web UI
sudo ufw allow 443/tcp        # HTTPS（如启用）
sudo ufw allow 445/tcp        # SMB
sudo ufw allow 2049/tcp       # NFS
sudo ufw allow 111/tcp        # RPC
sudo ufw allow 111/udp        # RPC

# 限制管理接口访问（推荐）
sudo ufw allow from 192.168.1.0/24 to any port 8080

sudo ufw enable

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --zone=public --add-service=ssh
sudo firewall-cmd --permanent --zone=public --add-port=8080/tcp
sudo firewall-cmd --permanent --zone=public --add-port=445/tcp
sudo firewall-cmd --permanent --zone=public --add-port=2049/tcp
sudo firewall-cmd --permanent --zone=public --add-port=111/tcp
sudo firewall-cmd --permanent --zone=public --add-port=111/udp
sudo firewall-cmd --reload
```

#### 启用 HTTPS

```bash
# 1. 生成自签名证书（测试用）
sudo mkdir -p /etc/nas-os
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/nas-os/key.pem \
  -out /etc/nas-os/cert.pem \
  -subj "/CN=nas-os.local"

# 2. 或使用 Let's Encrypt（生产环境）
sudo apt install certbot
sudo certbot certonly --standalone -d nas.example.com

# 3. 更新配置
sudo tee -a /etc/nas-os/config.yaml <<EOF
server:
  tls_enabled: true
  tls_cert: /etc/letsencrypt/live/nas.example.com/fullchain.pem
  tls_key: /etc/letsencrypt/live/nas.example.com/privkey.pem
EOF

# 4. 重启服务
sudo systemctl restart nas-os
```

### 审计日志

```bash
# 启用审计日志
sudo tee -a /etc/nas-os/config.yaml <<EOF
logging:
  audit_enabled: true
  audit_file: /var/log/nas-os/audit.log
  audit_events:
    - login
    - logout
    - user_create
    - user_delete
    - volume_create
    - volume_delete
    - share_create
    - share_delete
    - config_change
EOF

# 查看审计日志
sudo tail -f /var/log/nas-os/audit.log

# 审计日志格式（JSON）
# {"timestamp":"2026-03-11T10:00:00Z","event":"login","user":"admin","ip":"192.168.1.100","success":true}
```

### 安全加固建议

1. **修改默认密码**：首次启动后立即修改 admin 密码
2. **启用 HTTPS**：生产环境必须启用
3. **限制网络访问**：防火墙只允许可信 IP
4. **定期更新**：保持系统和 NAS-OS 最新
5. **启用双因素认证**（如支持）
6. **配置失败锁定**：防止暴力破解
7. **定期审计日志**：检查异常活动
8. **备份配置**：定期备份配置文件

---

## 性能优化

### btrfs 优化

#### 挂载选项

```bash
# 优化挂载（/etc/fstab）
/dev/sdb1 /mnt/data btrfs defaults,noatime,compress=zstd,commit=30 0 0

# 选项说明：
# noatime         - 不更新访问时间（减少写入）
# compress=zstd   - 启用 zstd 压缩（节省空间，提升读取）
# commit=30       - 30 秒提交一次（平衡性能与安全）
```

#### 压缩配置

```bash
# 查看压缩统计
sudo btrfs filesystem usage /mnt/data

# 启用压缩（新文件）
sudo btrfs property set /mnt/data compression zstd

# 对现有数据启用压缩（需要离线）
sudo umount /mnt/data
sudo btrfs convert -t zstd /dev/sdb1
sudo mount /mnt/data

# 或在线压缩（推荐）
sudo btrfs filesystem defragment -r -czstd /mnt/data
```

### SMB 优化

```yaml
# config.yaml
smb:
  # 网络优化
  socket_options: "TCP_NODELAY IPTOS_LOWDELAY SO_KEEPALIVE"
  read_raw: true
  write_raw: true
  use_sendfile: true
  
  # 缓存优化
  strict_locking: false
  oplocks: true
  level2_oplocks: true
  
  # 异步 IO
  aio read size: 16384
  aio write size: 16384
```

### NFS 优化

```yaml
# config.yaml
nfs:
  # 增加线程数
  threads: 16
  
  # 调整缓冲区
  rsize: 1048576    # 1MB 读缓冲区
  wsize: 1048576    # 1MB 写缓冲区
  
  # 导出选项
  default_options: "rw,sync,no_subtree_check,async,noatime"
```

### 系统级优化

#### 内核参数

```bash
# /etc/sysctl.d/99-nas-os.conf
# 网络优化
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# 文件描述符
fs.file-max = 2097152
fs.inotify.max_user_watches = 524288

# 应用配置
sudo sysctl -p /etc/sysctl.d/99-nas-os.conf
```

#### 资源限制

```bash
# /etc/security/limits.d/nas-os.conf
root soft nofile 65536
root hard nofile 65536
root soft nproc 4096
root hard nproc 4096
```

### 监控性能

```bash
# 实时监控
watch -n 1 'nasctl status'

# 磁盘 IO
iostat -x 1

# 网络流量
iftop -P -n

# 进程资源
top -p $(pgrep nasd)
```

---

## 监控与告警

### Prometheus 集成

#### 启用指标导出

```yaml
# config.yaml
monitoring:
  metrics_enabled: true
  metrics_port: 9090
```

#### 配置 Prometheus

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'nas-os'
    static_configs:
      - targets: ['nas-os.local:9090']
    scrape_interval: 30s
```

#### 可用指标

| 指标 | 说明 |
|------|------|
| `nasd_volume_size_bytes` | 卷总容量 |
| `nasd_volume_used_bytes` | 卷已使用 |
| `nasd_volume_usage_percent` | 卷使用率 |
| `nasd_disk_temperature_celsius` | 磁盘温度 |
| `nasd_smb_connections` | SMB 连接数 |
| `nasd_nfs_connections` | NFS 连接数 |
| `nasd_http_requests_total` | HTTP 请求总数 |
| `nasd_http_request_duration_seconds` | HTTP 请求延迟 |

### Grafana 仪表板

导入预配置的仪表板：

```bash
# 下载仪表板
curl -O https://raw.githubusercontent.com/nas-os/nasd/main/monitoring/grafana-dashboard.json

# 导入到 Grafana
# Grafana UI > Dashboard > Import > 上传 JSON
```

### 告警规则

```yaml
# alerts.yml
groups:
  - name: nas-os
    rules:
      - alert: DiskUsageWarning
        expr: nasd_volume_usage_percent > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "磁盘使用率超过 80%"
          
      - alert: DiskUsageCritical
        expr: nasd_volume_usage_percent > 95
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "磁盘使用率超过 95%"
          
      - alert: ServiceDown
        expr: up{job="nas-os"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "NAS-OS 服务宕机"
          
      - alert: DiskTemperatureHigh
        expr: nasd_disk_temperature_celsius > 55
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "磁盘温度过高"
```

### 通知配置

```yaml
# config.yaml
monitoring:
  notifications:
    email:
      enabled: true
      smtp_server: smtp.example.com
      smtp_port: 587
      smtp_user: alerts@example.com
      smtp_password: "${SMTP_PASSWORD}"
      from: nas-os@example.com
      to:
        - admin@example.com
    
    slack:
      enabled: true
      webhook_url: https://hooks.slack.com/services/xxx
      channel: "#nas-alerts"
    
    webhook:
      enabled: true
      url: https://api.example.com/alerts
      headers:
        Authorization: "Bearer ${WEBHOOK_TOKEN}"
```

---

## 备份策略

### 本地快照策略

```yaml
# config.yaml
backup:
  snapshot:
    enabled: true
    schedules:
      - name: hourly
        cron: "0 * * * *"
        retention: 24           # 保留 24 小时
        
      - name: daily
        cron: "0 2 * * *"
        retention: 7            # 保留 7 天
        
      - name: weekly
        cron: "0 3 * * 0"
        retention: 4            # 保留 4 周
        
      - name: monthly
        cron: "0 4 1 * *"
        retention: 12           # 保留 12 月
```

### 远程备份策略

#### Rsync 备份

```bash
# 创建备份脚本
sudo tee /usr/local/bin/nas-backup.sh <<'EOF'
#!/bin/bash
set -e

SOURCE="/mnt/data"
DEST="user@backup-server:/backup/nas-os"
LOG="/var/log/nas-os/backup.log"

echo "[$(date)] 开始备份" >> $LOG

rsync -avz --delete \
  --exclude='.snapshots' \
  --exclude='lost+found' \
  $SOURCE $DEST 2>&1 | tee -a $LOG

echo "[$(date)] 备份完成" >> $LOG
EOF

sudo chmod +x /usr/local/bin/nas-backup.sh

# 配置定时任务
sudo crontab -e
# 0 3 * * * /usr/local/bin/nas-backup.sh
```

#### Rclone 云备份

```bash
# 安装 rclone
curl https://rclone.org/install.sh | sudo bash

# 配置远程存储
rclone config

# 创建备份脚本
sudo tee /usr/local/bin/nas-cloud-backup.sh <<'EOF'
#!/bin/bash
set -e

SOURCE="/mnt/data"
REMOTE="remote:bucket/nas-backup"
LOG="/var/log/nas-os/cloud-backup.log"

echo "[$(date)] 开始云备份" >> $LOG

rclone sync $SOURCE $REMOTE \
  --progress \
  --transfers=4 \
  --checkers=8 \
  2>&1 | tee -a $LOG

echo "[$(date)] 云备份完成" >> $LOG
EOF

sudo chmod +x /usr/local/bin/nas-cloud-backup.sh
```

### 备份验证

```bash
# 定期验证备份完整性
sudo tee /usr/local/bin/nas-verify-backup.sh <<'EOF'
#!/bin/bash

BACKUP_DIR="/backup/nas-os"
LOG="/var/log/nas-os/verify.log"

echo "[$(date)] 开始验证备份" >> $LOG

# 检查文件数量
FILE_COUNT=$(find $BACKUP_DIR -type f | wc -l)
echo "文件数量：$FILE_COUNT" >> $LOG

# 检查最新文件时间
LATEST=$(find $BACKUP_DIR -type f -mtime -1 | wc -l)
if [ $LATEST -eq 0 ]; then
  echo "[$(date)] 警告：24 小时内无新备份" >> $LOG
  exit 1
fi

echo "[$(date)] 验证通过" >> $LOG
EOF

sudo chmod +x /usr/local/bin/nas-verify-backup.sh
```

---

## 维护操作

### 日常维护清单

```bash
# 每日检查
sudo nasctl status              # 系统状态
sudo nasctl storage usage       # 存储空间
sudo nasctl disk health         # 磁盘健康

# 每周检查
sudo nasctl scrub start data    # 数据校验
sudo journalctl -u nas-os --since "1 week ago" | grep -i error

# 每月检查
sudo apt update && sudo apt upgrade -y  # 系统更新
sudo nasd --version             # 版本检查
```

### 磁盘维护

#### 添加新磁盘

```bash
# 1. 查看新磁盘
lsblk

# 2. 格式化（如需要）
sudo mkfs.btrfs -f /dev/sdd1

# 3. 添加到现有卷
sudo nasctl volume add-device data /dev/sdd1

# 4. 平衡数据
sudo nasctl balance start data

# 5. 监控平衡进度
sudo nasctl balance status data
```

#### 替换故障磁盘

```bash
# 1. 识别故障磁盘
sudo btrfs filesystem show /mnt/data
sudo smartctl -a /dev/sdb

# 2. 卸载卷（如需要）
sudo umount /mnt/data

# 3. 替换磁盘
sudo btrfs replace start /dev/sdb /dev/sdd /mnt/data

# 4. 监控替换进度
sudo btrfs replace status /mnt/data

# 5. 移除旧磁盘
sudo btrfs device delete /dev/sdb /mnt/data
```

### 日志管理

```bash
# 查看日志
journalctl -u nas-os -n 100

# 清理旧日志
sudo journalctl --vacuum-time=7d

# 日志轮转配置
sudo tee /etc/logrotate.d/nas-os <<EOF
/var/log/nas-os/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 root root
    postrotate
        systemctl kill -s HUP nas-os
    endscript
}
EOF
```

### 配置备份

```bash
# 创建配置备份
sudo tar -czf /backup/nas-os-config-$(date +%Y%m%d).tar.gz \
  /etc/nas-os/ \
  /var/lib/nas-os/

# 测试恢复
sudo tar -xzf /backup/nas-os-config-YYYYMMDD.tar.gz -C /
```

---

## 故障排查

### 诊断工具

```bash
# 系统诊断脚本
sudo nasctl diagnose

# 输出示例：
# [✓] 服务运行正常
# [✓] 配置文件有效
# [✓] 端口 8080 可用
# [✓] btrfs 工具正常
# [✓] SMB 服务正常
# [✓] NFS 服务正常
# [!] 磁盘 /dev/sdb S.M.A.R.T. 警告
# [✓] 网络连接正常
```

### 常见问题

#### 问题 1：服务无法启动

```bash
# 诊断步骤
sudo systemctl status nas-os
journalctl -u nas-os -n 100 --no-pager
sudo ss -tulpn | grep 8080
sudo nasd --config /etc/nas-os/config.yaml --test

# 常见原因：
# 1. 端口被占用
# 2. 配置文件错误
# 3. 权限不足
# 4. 依赖服务未启动
```

#### 问题 2：性能下降

```bash
# 诊断步骤
top                              # CPU/内存使用
iostat -x 1                      # 磁盘 IO
iftop                            # 网络流量
sudo btrfs filesystem usage /mnt/data  # 存储使用

# 优化建议：
# 1. 启用 btrfs 压缩
# 2. 调整挂载选项
# 3. 限制并发连接
# 4. 检查磁盘健康
```

#### 问题 3：共享访问失败

```bash
# SMB 诊断
smbclient -L //<服务器 IP> -U%
sudo systemctl status smbd nmbd
sudo tail -f /var/log/samba/log.smbd

# NFS 诊断
showmount -e <服务器 IP>
sudo systemctl status nfs-server
sudo tail -f /var/log/syslog | grep nfs
```

---

## 升级与迁移

### 版本升级

#### Docker 升级

```bash
cd /opt/nas-os
docker-compose pull
docker-compose up -d
docker images | grep nas-os | grep none | awk '{print $3}' | xargs docker rmi
```

#### 裸机升级

```bash
# 1. 备份配置
sudo nasctl config export > /backup/config-backup.yaml

# 2. 停止服务
sudo systemctl stop nas-os

# 3. 下载新版本
sudo curl -L https://github.com/nas-os/nasd/releases/download/v1.0.1/nasd-linux-amd64 \
  -o /usr/local/bin/nasd
sudo chmod +x /usr/local/bin/nasd

# 4. 启动服务
sudo systemctl start nas-os

# 5. 验证版本
nasd --version
```

### 数据迁移

#### 迁移到新服务器

```bash
# 1. 在旧服务器创建快照
sudo nasctl snapshot create data migration-snapshot

# 2. 同步数据到新服务器
rsync -avz /mnt/data/ user@new-server:/mnt/data/

# 3. 导出配置
sudo nasctl config export > config.yaml

# 4. 在新服务器导入配置
sudo nasctl config import config.yaml

# 5. 验证数据
diff -r /mnt/data/ user@new-server:/mnt/data/
```

---

## 最佳实践

### 安全最佳实践

1. ✅ 启用 HTTPS（生产环境）
2. ✅ 修改默认密码
3. ✅ 配置防火墙规则
4. ✅ 启用审计日志
5. ✅ 定期更新系统
6. ✅ 配置失败锁定
7. ✅ 使用强密码策略
8. ✅ 定期审查访问日志

### 性能最佳实践

1. ✅ 使用 SSD 作为系统盘
2. ✅ 启用 btrfs 压缩（zstd）
3. ✅ 配置适当的挂载选项
4. ✅ 定期运行 scrub 和 balance
5. ✅ 监控磁盘健康
6. ✅ 优化 SMB/NFS 配置
7. ✅ 调整内核参数
8. ✅ 限制并发连接数

### 备份最佳实践

1. ✅ 3-2-1 备份策略（3 份副本，2 种介质，1 份异地）
2. ✅ 本地快照（每日）
3. ✅ 远程备份（每周）
4. ✅ 云备份（每月）
5. ✅ 定期验证备份
6. ✅ 测试恢复流程
7. ✅ 文档化备份策略
8. ✅ 监控备份状态

### 监控最佳实践

1. ✅ 配置 Prometheus + Grafana
2. ✅ 设置合理的告警阈值
3. ✅ 配置多渠道通知
4. ✅ 定期审查告警规则
5. ✅ 保留历史数据
6. ✅ 创建自定义仪表板
7. ✅ 定期演练故障响应
8. ✅ 文档化运维流程

---

*管理员指南版本：v1.0.0 | 最后更新：2026-03-11*
