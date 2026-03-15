# NAS-OS 部署指南

## 目录

- [概述](#概述)
- [系统要求](#系统要求)
- [快速部署](#快速部署)
- [服务管理](#服务管理)
- [系统维护](#系统维护)
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

## 服务管理

### 概述

`deploy.sh` 支持服务管理命令，可以方便地启动、停止、重启和查看服务状态。

### 使用方法

```bash
# 启动服务
./scripts/deploy.sh start

# 停止服务
./scripts/deploy.sh stop

# 重启服务
./scripts/deploy.sh restart

# 查看状态
./scripts/deploy.sh status

# 回滚到上一版本
./scripts/deploy.sh rollback
```

### 部署新版本

```bash
# 部署指定版本
./scripts/deploy.sh v2.68.0

# 部署失败时自动回滚
./scripts/deploy.sh v2.68.0 --rollback

# 模拟部署（不实际执行）
./scripts/deploy.sh v2.68.0 --dry-run

# 跳过数据库备份
./scripts/deploy.sh v2.68.0 --skip-backup
```

### 状态输出示例

```
========================================
  NAS-OS 服务状态
========================================

  版本:    2.68.0
  状态:    active
  PID:     12345
  内存:    256.5 MB
  CPU:     2.5%
  健康:    ✓ 正常

  最近日志:
  ----------------------------------------
  [2026-03-15 21:00:00] Server started
  [2026-03-15 21:00:01] API listening on :8080

========================================
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DATA_DIR` | /var/lib/nas-os | 数据目录 |
| `BACKUP_DIR` | /var/lib/nas-os/backups | 备份目录 |
| `BINARY_PATH` | /usr/local/bin/nasd | 二进制文件路径 |
| `SERVICE_NAME` | nas-os | 服务名称 |

## 系统维护

### 概述

`maintenance.sh` 提供日志清理、临时文件清理、系统检查等功能，建议配置定时任务自动执行。

### 使用方法

```bash
# 清理日志
./scripts/maintenance.sh --logs

# 清理临时文件
./scripts/maintenance.sh --temp

# 清理旧备份
./scripts/maintenance.sh --backups

# 执行系统检查
./scripts/maintenance.sh --check

# 执行所有维护任务
./scripts/maintenance.sh --all

# 执行维护并生成报告
./scripts/maintenance.sh --all --report

# 查看维护状态
./scripts/maintenance.sh --status

# 设置定时任务（每天凌晨 3:00）
./scripts/maintenance.sh --schedule

# 模拟运行
./scripts/maintenance.sh --all --dry-run
```

### 清理策略

| 清理项 | 默认保留策略 | 说明 |
|--------|-------------|------|
| 日志文件 | 30 天 | 超过 30 天的日志会被删除 |
| 压缩日志 | 30 天 | 超过 30 天的 .gz 日志会被删除 |
| 临时文件 | 7 天 | 超过 7 天的临时文件会被删除 |
| 版本备份 | 90 天 / 10 个 | 按时间和数量双重限制 |
| 数据库备份 | 90 天 | 超过 90 天的备份会被删除 |

### 系统检查项目

- 磁盘空间使用率
- 内存使用率
- 系统负载
- 服务运行状态
- 数据库完整性
- 日志错误统计
- API 健康检查

### 配置环境变量

```bash
# 清理策略
export LOG_MAX_AGE=30           # 日志保留天数
export TEMP_MAX_AGE=7           # 临时文件保留天数
export BACKUP_MAX_AGE=90        # 备份保留天数
export BACKUP_MAX_COUNT=10      # 最大备份数量

# 系统检查阈值
export DISK_THRESHOLD_WARNING=80
export DISK_THRESHOLD_CRITICAL=90
export MEMORY_THRESHOLD_WARNING=80
export MEMORY_THRESHOLD_CRITICAL=90
```

### 定时任务

使用 `--schedule` 命令可以自动配置 cron 定时任务：

```bash
# 设置每天凌晨 3:00 执行维护
sudo ./scripts/maintenance.sh --schedule
```

或者手动添加 crontab：

```bash
# 每天 3:00 执行维护
0 3 * * * /opt/nas-os/scripts/maintenance.sh --all --report >> /var/log/nas-os/maintenance-cron.log 2>&1
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