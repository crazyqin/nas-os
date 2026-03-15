# NAS-OS 健康检查使用说明

## 概述

健康检查脚本（`health-check.sh`）用于监控 NAS-OS 系统和服务状态，支持多种输出格式，便于集成到现有监控体系。

## 快速开始

### 基本用法

```bash
# 完整检查
./scripts/health-check.sh

# 快速检查（仅关键项）
./scripts/health-check.sh --quick

# JSON 格式输出
./scripts/health-check.sh --json
```

## 检查项详解

### 1. 进程状态检查

检查 NAS-OS 主进程（nasd）是否运行。

**检查内容：**
- 进程是否存在
- 进程 PID
- 内存使用量
- CPU 使用率

**状态判断：**
- `healthy`: 进程运行正常
- `critical`: 进程未运行

### 2. 端口监听检查

检查关键端口是否正常监听。

| 端口 | 服务 | 说明 |
|------|------|------|
| 8080 | API | Web UI 和 REST API |
| 445 | SMB | Windows 文件共享 |
| 2049 | NFS | Linux 文件共享 |

**状态判断：**
- `healthy`: 端口正常监听
- `critical`: 端口不可达

### 3. HTTP 端点检查

检查 API 健康端点响应。

**检查端点：**
- `/api/v1/health` - 服务健康状态
- `/api/v1/ready` - 服务就绪状态

**状态判断：**
- `healthy`: 响应正常且响应时间 < 阈值
- `warning`: 响应正常但响应时间 > 阈值
- `critical`: 响应异常或超时

### 4. 磁盘空间检查

检查磁盘使用率是否超过阈值。

**检查目录：**
- `/` - 根目录
- `/var/lib/nas-os` - 数据目录

**状态判断：**
- `healthy`: 使用率 < 警告阈值
- `warning`: 警告阈值 ≤ 使用率 < 严重阈值
- `critical`: 使用率 ≥ 严重阈值

### 5. 内存使用检查

检查系统内存使用率。

**状态判断：**
- `healthy`: 使用率 < 警告阈值
- `warning`: 警告阈值 ≤ 使用率 < 严重阈值
- `critical`: 使用率 ≥ 严重阈值

### 6. CPU 使用检查

检查 CPU 使用率。

**状态判断：**
- `healthy`: 使用率 < 警告阈值
- `warning`: 警告阈值 ≤ 使用率 < 严重阈值
- `critical`: 使用率 ≥ 严重阈值

### 7. 系统负载检查

检查系统负载相对于 CPU 核心数的比例。

**状态判断：**
- `healthy`: 负载 < 核心数 × 1.5
- `warning`: 核心数 × 1.5 ≤ 负载 < 核心数 × 2
- `critical`: 负载 ≥ 核心数 × 2

### 8. 数据库检查

检查 SQLite 数据库完整性。

**状态判断：**
- `healthy`: 完整性检查通过
- `critical`: 完整性检查失败

### 9. 日志检查

检查最近日志中的错误和警告数量。

**状态判断：**
- `healthy`: 无错误
- `warning`: 存在错误日志

## 阈值配置

### 配置文件方式

创建配置文件 `/etc/nas-os/health-threshold.conf`：

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

使用配置文件：

```bash
./scripts/health-check.sh --threshold-file /etc/nas-os/health-threshold.conf
```

### 环境变量方式

```bash
DISK_THRESHOLD_WARNING=70 \
DISK_THRESHOLD_CRITICAL=85 \
./scripts/health-check.sh
```

## 输出格式

### 文本格式（默认）

```
===================================
NAS-OS 健康检查 v2.56.0
时间: 2026-03-15T15:00:00+08:00
===================================

[检查] 进程状态
  [healthy] 进程运行中 (PID: 1234)

[检查] 端口状态
  [healthy] 端口 8080 (api) 监听中
  [healthy] 端口 445 (smb) 监听中
  [warning] 端口 2049 (nfs) 未监听

[检查] 系统资源
  [healthy] 磁盘使用: 45%
  [warning] 内存使用率: 85%
  [healthy] CPU 使用率: 25%

===================================
整体状态: degraded

警告项 (2):
  - 端口 2049 (nfs) 未监听
  - 内存使用率: 85%
```

### JSON 格式

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

## 退出码

| 退出码 | 含义 | 建议操作 |
|--------|------|----------|
| 0 | 健康 | 无需操作 |
| 1 | 不健康 | 检查错误项，可能需要人工干预 |
| 2 | 降级 | 存在警告，建议关注 |

## 集成场景

### 1. Cron 定时检查

```bash
# crontab -e
# 每 5 分钟检查一次，记录日志
*/5 * * * * /opt/nas-os/scripts/health-check.sh --json >> /var/log/nas-os/health.log 2>&1
```

### 2. 监控系统集成

#### Prometheus + Node Exporter

将 JSON 输出转换为 Prometheus 指标：

```bash
# 示例：创建简单的指标导出
./scripts/health-check.sh --json | jq -r '
  .checks[] | 
  "nas_os_health_check{name=\"\(.name)\"} \(.status == \"healthy\" | if . then 1 else 0 end)"
' > /var/lib/node_exporter/textfile_collector/nas_os_health.prom
```

#### Zabbix

创建 Zabbix 用户参数：

```bash
# /etc/zabbix/zabbix_agentd.conf
UserParameter=nas-os.health,/opt/nas-os/scripts/health-check.sh --silent; echo $?
UserParameter=nas-os.health.json,/opt/nas-os/scripts/health-check.sh --json
```

### 3. 告警脚本

```bash
#!/bin/bash
# check-and-alert.sh

result=$(./scripts/health-check.sh --json)
status=$(echo "$result" | jq -r '.status')

if [ "$status" != "healthy" ]; then
    # 发送告警
    curl -X POST -H "Content-Type: application/json" \
        -d "{\"text\": \"NAS-OS 健康检查异常: $status\n$result\"}" \
        "$ALERT_WEBHOOK"
fi
```

### 4. Kubernetes 健康探针

将健康检查脚本作为容器健康探针：

```yaml
livenessProbe:
  exec:
    command:
      - /opt/nas-os/scripts/health-check.sh
      - --quick
      - --silent
  initialDelaySeconds: 30
  periodSeconds: 60

readinessProbe:
  exec:
    command:
      - /opt/nas-os/scripts/health-check.sh
      - --quick
      - --silent
  initialDelaySeconds: 5
  periodSeconds: 10
```

## 快速检查模式

快速检查仅检查关键项，适合高频检查场景：

- 进程状态
- API 端口
- 健康端点

```bash
./scripts/health-check.sh --quick --json
```

## 常见问题

### Q: 检查超时怎么办？

A: 增加超时时间：

```bash
API_TIMEOUT_MS=10000 ./scripts/health-check.sh
```

### Q: 如何忽略某些检查？

A: 目前不支持忽略，可以通过环境变量调高阈值：

```bash
DISK_THRESHOLD_CRITICAL=100 MEMORY_THRESHOLD_CRITICAL=100 ./scripts/health-check.sh
```

### Q: JSON 输出中的 details 字段是什么？

A: details 字段包含详细的检查数据，便于程序解析：

```
details="pid=1234 memory_mb=256 cpu_pct=2.5"
```

---

更多信息请参考 [部署指南](./README.md)。