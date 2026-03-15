# NAS-OS 部署指南

## 目录

- [概述](#概述)
- [系统要求](#系统要求)
- [快速部署](#快速部署)
- [配置说明](#配置说明)
- [健康检查](#健康检查)
- [服务监控](#服务监控)
- [故障排除](#故障排除)

## 概述

本文档介绍 NAS-OS 的部署流程和运维工具使用方法。

## 系统要求

### 硬件要求

| 项目 | 最低要求 | 推荐配置 |
|------|----------|----------|
| CPU | 2 核心 | 4 核心+ |
| 内存 | 2 GB | 4 GB+ |
| 存储 | 20 GB | 100 GB+ |

### 软件要求

- 操作系统：Linux (推荐 Ubuntu 22.04+ 或 Debian 12+)
- 依赖服务：
  - SQLite 3.x
  - systemd
  - Docker（可选）

## 快速部署

### 1. 下载部署脚本

```bash
curl -fsSL https://get.nas-os.io | bash
```

### 2. 使用部署脚本

```bash
cd /opt/nas-os
./scripts/deploy.sh --install
```

### 3. 验证服务状态

```bash
./scripts/health-check.sh
```

## 配置说明

### 主配置文件

配置文件位于 `/etc/nas-os/config.yaml`

### 健康检查配置

健康检查阈值配置文件：`/etc/nas-os/health-threshold.conf`

```bash
# 磁盘阈值（百分比）
DISK_THRESHOLD_WARNING=80
DISK_THRESHOLD_CRITICAL=90

# 内存阈值（百分比）
MEMORY_THRESHOLD_WARNING=80
MEMORY_THRESHOLD_CRITICAL=90

# CPU 阈值（百分比）
CPU_THRESHOLD_WARNING=80
CPU_THRESHOLD_CRITICAL=95

# API 响应超时（毫秒）
API_TIMEOUT_MS=5000
```

## 健康检查

### 概述

`health-check.sh` 用于检查系统和服务健康状态，支持 JSON 输出格式，便于集成到监控系统。

### 使用方法

```bash
# 完整检查
./scripts/health-check.sh

# 快速检查（仅关键项）
./scripts/health-check.sh --quick

# JSON 格式输出
./scripts/health-check.sh --json

# 指定阈值配置文件
./scripts/health-check.sh --threshold-file /path/to/config.conf
```

### 检查项目

| 检查项 | 说明 |
|--------|------|
| 进程状态 | 检查 nasd 进程是否运行 |
| 端口监听 | 检查 API、SMB、NFS 端口 |
| API 端点 | 检查健康检查和就绪端点 |
| 磁盘空间 | 检查根目录和数据目录使用率 |
| 内存使用 | 检查系统内存使用率 |
| CPU 使用 | 检查 CPU 使用率 |
| 系统负载 | 检查系统负载情况 |
| 数据库 | 检查 SQLite 数据库完整性 |
| 日志 | 检查最近日志中的错误 |

### 输出格式

#### 文本格式

```
===================================
NAS-OS 健康检查 v2.56.0
===================================

[检查] 进程状态
  [healthy] 进程运行中 (PID: 1234)

[检查] 磁盘空间
  [warning] 磁盘使用率: 85%

整体状态: degraded
```

#### JSON 格式

```json
{
  "version": "2.56.0",
  "timestamp": "2026-03-15T15:00:00+08:00",
  "hostname": "nas-server",
  "status": "healthy",
  "checks": [
    {
      "name": "process_nasd",
      "status": "healthy",
      "message": "进程运行中 (PID: 1234)",
      "details": "pid=1234 memory_mb=256 cpu_pct=2.5"
    }
  ],
  "errors": [],
  "warnings": []
}
```

### 退出码

| 退出码 | 含义 |
|--------|------|
| 0 | 健康 |
| 1 | 不健康（有严重问题） |
| 2 | 降级（有警告但无严重问题） |

### 集成到监控系统

#### Prometheus

```bash
# 在 Prometheus 配置中添加
- job_name: 'nas-os-health'
  static_configs:
    - targets: ['localhost:9101']
```

#### Cron 定时检查

```bash
# 每 5 分钟检查一次
*/5 * * * * /opt/nas-os/scripts/health-check.sh --json >> /var/log/nas-os/health.log 2>&1
```

## 服务监控

### 概述

`service-monitor.sh` 用于持续监控 NAS-OS 核心服务，支持自动重启异常服务。

### 使用方法

```bash
# 单次检查
./scripts/service-monitor.sh

# 守护进程模式
./scripts/service-monitor.sh --daemon

# 停止守护进程
./scripts/service-monitor.sh --stop

# 查看状态
./scripts/service-monitor.sh --status

# 干运行模式（测试，不执行重启）
./scripts/service-monitor.sh --dry-run
```

### 监控的服务

| 服务 | 检查方式 | 描述 |
|------|----------|------|
| nasd | 进程检查 | NAS-OS 主服务 |
| api | HTTP 健康端点 | API 服务 |
| smb | 端口 445 | SMB 文件共享 |
| nfs | 端口 2049 | NFS 文件共享 |

### 自动重启策略

- **最大重启次数**：同一服务在 1 小时内最多重启 3 次
- **冷却时间**：重启后等待 5 分钟才能再次重启
- **告警通知**：重启后发送 Webhook/邮件告警

### 配置环境变量

```bash
# 监控间隔（秒）
export MONITOR_INTERVAL=60

# 最大重启次数
export MAX_RESTART_COUNT=3

# 重启冷却时间（秒）
export RESTART_COOLDOWN=300

# 告警 Webhook
export ALERT_WEBHOOK=https://hooks.slack.com/services/xxx

# 告警邮件
export ALERT_EMAIL=admin@example.com
```

### 作为 systemd 服务运行

创建服务文件 `/etc/systemd/system/nas-os-monitor.service`：

```ini
[Unit]
Description=NAS-OS Service Monitor
After=network.target nas-os.service

[Service]
Type=simple
ExecStart=/opt/nas-os/scripts/service-monitor.sh --daemon
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用服务：

```bash
systemctl daemon-reload
systemctl enable nas-os-monitor
systemctl start nas-os-monitor
```

## 故障排除

### 常见问题

#### 1. 服务无法启动

检查日志：

```bash
journalctl -u nas-os -f
```

检查配置：

```bash
./scripts/health-check.sh --json
```

#### 2. 磁盘空间不足

```bash
# 检查磁盘使用
df -h

# 清理日志
./scripts/logrotate.sh --force

# 清理旧备份
rm -rf /var/lib/nas-os/backups/old-*
```

#### 3. 内存不足

```bash
# 检查内存使用
free -h

# 重启服务
systemctl restart nasd
```

### 日志位置

| 日志文件 | 说明 |
|----------|------|
| `/var/log/nas-os/nas-os.log` | 主服务日志 |
| `/var/log/nas-os/health.log` | 健康检查日志 |
| `/var/log/nas-os/service-monitor.log` | 服务监控日志 |

### 获取诊断信息

```bash
# 收集诊断信息
./scripts/health-check.sh --json > /tmp/health-report.json
journalctl -u nas-os --no-pager -n 100 > /tmp/nas-os-journal.log
```

---

更多问题请参考 [故障排除文档](../TROUBLESHOOTING.md) 或联系技术支持。