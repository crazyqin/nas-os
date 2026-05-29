# NAS-OS Presto 高速文件传输运维文档

## 1. 概述

Presto 是 NAS-OS 的高速文件传输功能，对标群晖 Presto File Server，提供高性能、安全可靠的文件传输服务。

### 1.1 核心特性

- **高速传输**: 多线程并发传输，支持分块传输
- **数据压缩**: 可配置压缩级别，节省带宽
- **传输加密**: AES 加密保护数据安全
- **断点续传**: 支持传输中断后继续
- **传输监控**: 完整的 Prometheus 指标和 Grafana 面板
- **告警通知**: 多级别告警，支持邮件/企业微信/钉钉

### 1.2 架构组件

```
┌─────────────────────────────────────────────────────────────┐
│                    Presto 传输服务                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  传输引擎   │  │  压缩模块   │  │  加密模块   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  监控指标   │  │  日志系统   │  │  告警系统   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
         │                  │                  │
         ▼                  ▼                  ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   Prometheus    │ │   Grafana       │ │  Alertmanager   │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

## 2. 部署指南

### 2.1 环境要求

- Docker 20.10+
- Docker Compose 2.0+
- 最小内存: 4GB
- 推荐 CPU: 4 核以上
- 磁盘空间: 根据传输数据量调整

### 2.2 快速部署

```bash
# 1. 进入 Presto 部署目录
cd deploy/presto

# 2. 复制并修改配置文件
cp .env.example .env
vi .env

# 3. 执行部署脚本
./deploy.sh --install
```

### 2.3 手动部署

```bash
# 1. 创建必要目录
mkdir -p configs logs monitoring/grafana/provisioning/{datasources,dashboards}
sudo mkdir -p /var/lib/presto
sudo chown -R 1000:1000 /var/lib/presto

# 2. 复制配置文件
cp presto.yaml configs/
cp .env.example .env

# 3. 修改配置
vi .env

# 4. 启动服务
docker compose -f docker-compose.presto.yml up -d

# 5. 验证服务
docker compose -f docker-compose.presto.yml ps
curl http://localhost:8090/health
```

### 2.4 配置说明

#### 环境变量 (.env)

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| NAS_OS_IMAGE | ghcr.io/nas-os/nas-os:latest | 镜像地址 |
| PRESTO_MAX_CONCURRENT | 4 | 最大并发传输数 |
| PRESTO_COMPRESS_LEVEL | 6 | 压缩级别 (0-9) |
| PRESTO_ENCRYPT_AES | true | 启用 AES 加密 |
| PRESTO_CHUNK_SIZE_MB | 64 | 分块大小 (MB) |
| PRESTO_BANDWIDTH_MBPS | 0 | 带宽限制 (0=不限) |
| PRESTO_CPU_LIMIT | 4.0 | CPU 限制 |
| PRESTO_MEMORY_LIMIT | 4G | 内存限制 |
| PROMETHEUS_PORT | 9091 | Prometheus 端口 |
| GRAFANA_PORT | 3001 | Grafana 端口 |
| ALERTMANAGER_PORT | 9094 | Alertmanager 端口 |

#### Presto 配置 (presto.yaml)

```yaml
presto:
  enabled: true
  port: 8090
  transfer:
    max_concurrent: 4
    compress_level: 6
    encrypt_aes: true
    chunk_size_mb: 64
    bandwidth_mbps: 0
    timeout: 3600
    retry_count: 3
    retry_interval: 30
  storage:
    temp_dir: /tmp/presto
    verify_checksum: true
    checksum_algorithm: sha256
  metrics:
    enabled: true
    prefix: presto_
  security:
    tls_enabled: false
    auth_type: token
```

## 3. 监控指标

### 3.1 核心指标

| 指标名称 | 类型 | 说明 |
|----------|------|------|
| presto_transfers_total | Counter | 传输任务总数 |
| presto_transfers_completed_total | Counter | 完成的传输任务数 |
| presto_transfers_failed_total | Counter | 失败的传输任务数 |
| presto_transfers_cancelled_total | Counter | 取消的传输任务数 |
| presto_transfers_running | Gauge | 当前运行中的传输数 |
| presto_transfers_pending | Gauge | 待处理的传输数 |
| presto_transfer_speed_mbps | Gauge | 当前传输速度 (MB/s) |
| presto_bytes_transferred_total | Counter | 已传输的总字节数 |
| presto_compression_ratio | Gauge | 压缩率 |
| presto_encryption_enabled | Gauge | 加密状态 (0/1) |
| presto_transfers_timeout_total | Counter | 超时的传输任务数 |

### 3.2 系统指标

| 指标名称 | 类型 | 说明 |
|----------|------|------|
| process_cpu_seconds_total | Counter | CPU 使用时间 |
| process_resident_memory_bytes | Gauge | 内存使用量 |
| process_open_fds | Gauge | 打开的文件描述符数 |
| process_max_fds | Gauge | 最大文件描述符数 |
| go_goroutines | Gauge | Goroutine 数量 |

### 3.3 查询示例

```promql
# 传输成功率
(presto_transfers_completed_total / (presto_transfers_completed_total + presto_transfers_failed_total)) * 100

# 传输错误率
rate(presto_transfers_failed_total[5m]) / rate(presto_transfers_total[5m]) * 100

# 平均传输速度
avg(presto_transfer_speed_mbps)

# 内存使用率
process_resident_memory_bytes{job="presto"} / 1024 / 1024
```

## 4. 告警规则

### 4.1 传输告警

| 告警名称 | 级别 | 条件 | 说明 |
|----------|------|------|------|
| PrestoTransferSpeedLow | warning | 速度 < 10 MB/s 持续 5 分钟 | 传输速度过低 |
| PrestoTransferSpeedCritical | critical | 速度 < 1 MB/s 持续 2 分钟 | 传输速度极低 |
| PrestoTransferSuccessRateLow | warning | 成功率 < 95% 持续 10 分钟 | 成功率偏低 |
| PrestoTransferSuccessRateCritical | critical | 成功率 < 80% 持续 5 分钟 | 成功率严重下降 |
| PrestoTransferErrorRateHigh | warning | 错误率 > 5% 持续 5 分钟 | 错误率偏高 |
| PrestoTransferErrorRateCritical | critical | 错误率 > 20% 持续 2 分钟 | 错误率过高 |
| PrestoConcurrentTransfersHigh | warning | 并发数 > 10 持续 5 分钟 | 并发数过高 |
| PrestoTransferQueueBacklog | warning | 待处理 > 20 持续 10 分钟 | 队列积压 |
| PrestoTransferTimeout | warning | 超时数 > 0 | 传输出现超时 |

### 4.2 资源告警

| 告警名称 | 级别 | 条件 | 说明 |
|----------|------|------|------|
| PrestoMemoryUsageHigh | warning | 内存 > 2GB 持续 5 分钟 | 内存使用过高 |
| PrestoCPUUsageHigh | warning | CPU > 80% 持续 5 分钟 | CPU 使用过高 |
| PrestoFileDescriptorsHigh | warning | FD > 80% 持续 5 分钟 | 文件描述符过高 |
| PrestoGoroutinesHigh | warning | Goroutine > 1000 持续 5 分钟 | Goroutine 过高 |

### 4.3 可用性告警

| 告警名称 | 级别 | 条件 | 说明 |
|----------|------|------|------|
| PrestoServiceDown | critical | 服务不可用持续 1 分钟 | 服务宕机 |
| PrestoHealthCheckFailed | critical | 健康检查失败持续 2 分钟 | 服务异常 |
| PrestoServiceRestarted | warning | 1 小时内重启 | 服务重启 |

## 5. Grafana 面板

### 5.1 面板概览

Grafana 面板包含以下视图:

1. **概览统计**
   - 服务状态
   - 当前传输速度
   - 运行中传输数
   - 待处理队列

2. **传输趋势**
   - 传输速度趋势图
   - 传输任务状态图
   - 传输结果分布图
   - 传输成功率仪表盘

3. **资源监控**
   - 资源使用率趋势
   - 压缩与加密状态

4. **数据统计**
   - 数据传输量趋势
   - 传输完成/失败/取消分布

### 5.2 访问方式

```
URL: http://localhost:3001
用户名: admin
密码: presto123
```

### 5.3 自定义面板

登录 Grafana 后可以:

1. 创建自定义仪表盘
2. 添加告警规则
3. 配置通知渠道
4. 导入其他面板模板

## 6. 运维操作

### 6.1 服务管理

```bash
# 启动服务
docker compose -f docker-compose.presto.yml up -d

# 停止服务
docker compose -f docker-compose.presto.yml down

# 重启服务
docker compose -f docker-compose.presto.yml restart

# 查看状态
docker compose -f docker-compose.presto.yml ps

# 查看日志
docker compose -f docker-compose.presto.yml logs -f

# 查看特定服务日志
docker compose -f docker-compose.presto.yml logs -f presto
```

### 6.2 配置更新

```bash
# 1. 修改配置文件
vi configs/presto.yaml

# 2. 重启服务
docker compose -f docker-compose.presto.yml restart presto

# 3. 验证配置
curl http://localhost:8090/health
```

### 6.3 数据备份

```bash
# 备份 Presto 数据
sudo tar -czf presto-backup-$(date +%Y%m%d).tar.gz /var/lib/presto

# 备份配置文件
tar -czf presto-config-$(date +%Y%m%d).tar.gz configs/ .env

# 备份 Prometheus 数据
sudo tar -czf prometheus-backup-$(date +%Y%m%d).tar.gz /var/lib/prometheus

# 备份 Grafana 数据
sudo tar -czf grafana-backup-$(date +%Y%m%d).tar.gz /var/lib/grafana
```

### 6.4 数据恢复

```bash
# 停止服务
docker compose -f docker-compose.presto.yml down

# 恢复数据
sudo tar -xzf presto-backup-20240101.tar.gz -C /

# 恢复配置
tar -xzf presto-config-20240101.tar.gz .

# 启动服务
docker compose -f docker-compose.presto.yml up -d
```

### 6.5 升级流程

```bash
# 方式一: 使用部署脚本
./deploy.sh --upgrade

# 方式二: 手动升级
# 1. 备份配置
cp .env .env.backup

# 2. 拉取最新镜像
docker compose -f docker-compose.presto.yml pull

# 3. 重启服务
docker compose -f docker-compose.presto.yml up -d

# 4. 验证服务
docker compose -f docker-compose.presto.yml ps
curl http://localhost:8090/health
```

## 7. 故障排查

### 7.1 常见问题

#### 问题 1: 服务无法启动

**症状**: 容器启动失败或立即退出

**排查步骤**:
```bash
# 查看容器日志
docker compose -f docker-compose.presto.yml logs presto

# 检查配置文件语法
docker compose -f docker-compose.presto.yml config

# 检查端口占用
netstat -tlnp | grep 8090
```

**常见原因**:
- 配置文件语法错误
- 端口被占用
- 数据目录权限问题

#### 问题 2: 传输速度慢

**症状**: 传输速度远低于预期

**排查步骤**:
```bash
# 检查系统资源
docker stats nas-presto

# 检查网络带宽
iperf3 -c <target-ip>

# 检查磁盘 IO
iostat -x 1

# 查看传输日志
docker compose -f docker-compose.presto.yml logs presto | grep -i speed
```

**优化建议**:
- 增加 `PRESTO_MAX_CONCURRENT` 并发数
- 调整 `PRESTO_CHUNK_SIZE_MB` 分块大小
- 检查网络带宽和磁盘 IO
- 考虑降低压缩级别

#### 问题 3: 传输失败率高

**症状**: 大量传输任务失败

**排查步骤**:
```bash
# 查看错误日志
docker compose -f docker-compose.presto.yml logs presto | grep -i error

# 检查目标路径权限
ls -la /mnt/transfer/dst

# 检查磁盘空间
df -h /mnt/transfer/*
```

**常见原因**:
- 目标路径权限不足
- 磁盘空间不足
- 网络不稳定
- 文件系统错误

#### 问题 4: 内存使用过高

**症状**: 容器内存持续增长

**排查步骤**:
```bash
# 查看内存使用
docker stats nas-presto

# 查看 Goroutine 数量
curl http://localhost:8090/debug/pprof/goroutine?debug=1

# 查看内存分配
curl http://localhost:8090/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**优化建议**:
- 减少 `PRESTO_MAX_CONCURRENT` 并发数
- 减小 `PRESTO_CHUNK_SIZE_MB` 分块大小
- 增加 `PRESTO_MEMORY_LIMIT` 内存限制

### 7.2 日志分析

```bash
# 查看实时日志
docker compose -f docker-compose.presto.yml logs -f presto

# 查看错误日志
docker compose -f docker-compose.presto.yml logs presto | grep -i error

# 查看传输日志
docker compose -f docker-compose.presto.yml logs presto | grep -i transfer

# 按时间过滤日志
docker compose -f docker-compose.presto.yml logs --since 1h presto

# 导出日志
docker compose -f docker-compose.presto.yml logs presto > presto.log
```

### 7.3 性能调优

#### 系统级优化

```bash
# 调整系统文件描述符限制
ulimit -n 65535

# 调整网络缓冲区
sysctl -w net.core.rmem_max=16777216
sysctl -w net.core.wmem_max=16777216

# 调整 TCP 参数
sysctl -w net.ipv4.tcp_window_scaling=1
sysctl -w net.ipv4.tcp_timestamps=1
```

#### 应用级优化

```yaml
# presto.yaml 优化配置
presto:
  transfer:
    max_concurrent: 8        # 增加并发数
    compress_level: 3        # 降低压缩级别
    chunk_size_mb: 128       # 增加分块大小
    bandwidth_mbps: 0        # 不限速
    timeout: 7200            # 增加超时时间
```

## 8. 安全配置

### 8.1 TLS 配置

```yaml
presto:
  security:
    tls_enabled: true
    tls_cert_file: /etc/nas-os/tls/presto.crt
    tls_key_file: /etc/nas-os/tls/presto.key
```

生成自签名证书:
```bash
mkdir -p configs/tls
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout configs/tls/presto.key \
  -out configs/tls/presto.crt
```

### 8.2 认证配置

```yaml
presto:
  security:
    auth_type: token
    api_token: "your-secret-token-here"
```

使用 Token 认证:
```bash
curl -H "Authorization: Bearer your-secret-token-here" \
  http://localhost:8090/api/transfers
```

### 8.3 防火墙配置

```bash
# 只允许内网访问
sudo ufw allow from 192.168.1.0/24 to any port 8090
sudo ufw allow from 192.168.1.0/24 to any port 9091
sudo ufw allow from 192.168.1.0/24 to any port 3001
```

## 9. API 参考

### 9.1 传输管理

#### 创建传输任务

```bash
curl -X POST http://localhost:8090/api/transfers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "name": "backup-20240101",
    "source_path": "/mnt/transfer/src/backup.tar.gz",
    "dest_path": "/mnt/transfer/dst/backup.tar.gz"
  }'
```

#### 获取传输列表

```bash
curl http://localhost:8090/api/transfers \
  -H "Authorization: Bearer <token>"
```

#### 获取传输详情

```bash
curl http://localhost:8090/api/transfers/<transfer-id> \
  -H "Authorization: Bearer <token>"
```

#### 取消传输

```bash
curl -X POST http://localhost:8090/api/transfers/<transfer-id>/cancel \
  -H "Authorization: Bearer <token>"
```

#### 获取统计信息

```bash
curl http://localhost:8090/api/stats \
  -H "Authorization: Bearer <token>"
```

### 9.2 健康检查

```bash
curl http://localhost:8090/health
```

响应:
```json
{
  "status": "healthy",
  "version": "2.400.0",
  "uptime": "2h30m"
}
```

### 9.3 指标接口

```bash
curl http://localhost:8080/metrics
```

## 10. 最佳实践

### 10.1 部署建议

1. **资源规划**: 根据传输量和并发需求合理分配资源
2. **存储规划**: 为临时文件和日志预留足够空间
3. **网络规划**: 确保足够的网络带宽
4. **备份策略**: 定期备份配置和数据

### 10.2 运维建议

1. **监控告警**: 及时处理告警，避免问题扩大
2. **日志轮转**: 配置日志轮转，避免磁盘占满
3. **定期检查**: 定期检查服务状态和资源使用
4. **版本更新**: 及时更新到最新版本

### 10.3 安全建议

1. **启用认证**: 生产环境必须启用认证
2. **使用 TLS**: 敏感数据传输使用 TLS 加密
3. **网络隔离**: 使用防火墙限制访问来源
4. **定期审计**: 审计日志和访问记录

## 11. 附录

### 11.1 端口列表

| 服务 | 端口 | 说明 |
|------|------|------|
| Presto API | 8090 | 传输服务 API |
| Presto Metrics | 8080 | Prometheus 指标 |
| Prometheus | 9091 | 监控数据 |
| Grafana | 3001 | 可视化面板 |
| Alertmanager | 9094 | 告警管理 |

### 11.2 目录结构

```
deploy/presto/
├── docker-compose.presto.yml  # Docker Compose 配置
├── .env.example               # 环境变量示例
├── presto.yaml                # Presto 配置模板
├── deploy.sh                  # 部署脚本
├── README.md                  # 本文档
└── monitoring/
    ├── prometheus-presto.yml  # Prometheus 配置
    ├── alerts-presto.yml      # 告警规则
    ├── alertmanager-presto.yml # Alertmanager 配置
    └── grafana/
        └── provisioning/
            ├── datasources/
            │   └── prometheus.yml
            └── dashboards/
                ├── dashboards.yml
                └── presto.json
```

### 11.3 相关链接

- [NAS-OS 官方文档](https://docs.nas-os.io)
- [Presto API 文档](https://docs.nas-os.io/presto/api)
- [Prometheus 文档](https://prometheus.io/docs/)
- [Grafana 文档](https://grafana.com/docs/)
- [Alertmanager 文档](https://prometheus.io/docs/alerting/latest/alertmanager/)

### 11.4 版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| v2.400.0 | 2026-05-29 | 初始版本，完整部署方案 |

---

**文档维护**: 工部  
**最后更新**: 2026-05-29
