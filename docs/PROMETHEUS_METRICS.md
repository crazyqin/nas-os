# NAS-OS Prometheus 指标参考

> v2.31.0 - 完整指标列表

本文档列出 NAS-OS 暴露的所有 Prometheus 指标。

## 指标端点

- **URL**: `http://<host>:8080/metrics`
- **格式**: Prometheus text format
- **刷新间隔**: 15s（默认）

---

## 系统指标

### CPU

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_cpu_usage_percent` | Gauge | - | 当前 CPU 使用率（百分比） |

### 内存

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_memory_usage_percent` | Gauge | - | 内存使用率（百分比） |
| `nas_os_memory_total_bytes` | Gauge | - | 总内存（字节） |
| `nas_os_memory_used_bytes` | Gauge | - | 已用内存（字节） |
| `nas_os_memory_available_bytes` | Gauge | - | 可用内存（字节） |

### Swap

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_swap_usage_percent` | Gauge | - | Swap 使用率（百分比） |
| `nas_os_swap_total_bytes` | Gauge | - | 总 Swap（字节） |
| `nas_os_swap_used_bytes` | Gauge | - | 已用 Swap（字节） |

### 负载

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_load_avg_1` | Gauge | - | 1 分钟平均负载 |
| `nas_os_load_avg_5` | Gauge | - | 5 分钟平均负载 |
| `nas_os_load_avg_15` | Gauge | - | 15 分钟平均负载 |

### 运行时间

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_uptime_seconds` | Gauge | - | 系统运行时间（秒） |

---

## 磁盘指标

### 空间

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_disk_usage_percent` | Gauge | device, mountpoint, fstype | 磁盘使用率 |
| `nas_os_disk_total_bytes` | Gauge | device, mountpoint | 总空间 |
| `nas_os_disk_used_bytes` | Gauge | device, mountpoint | 已用空间 |
| `nas_os_disk_free_bytes` | Gauge | device, mountpoint | 可用空间 |

### I/O

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_disk_read_bytes_total` | Gauge | device | 总读取字节数 |
| `nas_os_disk_write_bytes_total` | Gauge | device | 总写入字节数 |
| `nas_os_disk_read_ops_total` | Gauge | device | 总读取操作数 |
| `nas_os_disk_write_ops_total` | Gauge | device | 总写入操作数 |

---

## 网络指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_network_rx_bytes_total` | Gauge | interface | 总接收字节数 |
| `nas_os_network_tx_bytes_total` | Gauge | interface | 总发送字节数 |
| `nas_os_network_rx_packets_total` | Gauge | interface | 总接收包数 |
| `nas_os_network_tx_packets_total` | Gauge | interface | 总发送包数 |
| `nas_os_network_rx_errors_total` | Gauge | interface | 总接收错误数 |
| `nas_os_network_tx_errors_total` | Gauge | interface | 总发送错误数 |

---

## 存储池指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_storage_pool_usage_percent` | Gauge | pool_name, pool_type | 存储池使用率 |
| `nas_os_storage_pool_total_bytes` | Gauge | pool_name, pool_type | 存储池总容量 |
| `nas_os_storage_pool_used_bytes` | Gauge | pool_name, pool_type | 存储池已用容量 |
| `nas_os_storage_pool_health_status` | Gauge | pool_name | 健康状态 (0=unknown, 1=healthy, 2=degraded, 3=failed) |

---

## 备份指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_backup_total` | Gauge | type, status | 备份任务总数 |
| `nas_os_backup_size_bytes` | Gauge | job_id, job_name | 备份大小 |
| `nas_os_backup_duration_seconds` | Gauge | job_id, job_name | 备份持续时间 |
| `nas_os_backup_status` | Gauge | job_id | 备份状态 (0=pending, 1=running, 2=completed, 3=failed, 4=cancelled) |
| `nas_os_backup_last_run_timestamp` | Gauge | job_id | 最后运行时间戳 |

---

## 快照指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_snapshot_total` | Gauge | pool, type | 快照总数 |
| `nas_os_snapshot_size_bytes` | Gauge | pool, snapshot_id | 快照大小 |
| `nas_os_snapshot_creation_timestamp` | Gauge | pool, snapshot_id | 创建时间戳 |
| `nas_os_snapshot_retention_days` | Gauge | policy_id | 保留天数 |

---

## 共享指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_share_total` | Gauge | type | 共享总数 |
| `nas_os_share_connections` | Gauge | share_name, type | 活跃连接数 |
| `nas_os_share_bytes_transferred_total` | Gauge | share_name, direction | 传输字节数 |

---

## 用户指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_users_total` | Gauge | - | 用户总数 |
| `nas_os_users_active` | Gauge | - | 活跃用户数 |
| `nas_os_user_sessions` | Gauge | username | 用户会话数 |

---

## API 指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_api_requests_total` | Counter | method, path, status | API 请求总数 |
| `nas_os_api_request_duration_seconds` | Histogram | method, path | 请求延迟分布 |
| `nas_os_api_requests_in_flight` | Gauge | - | 正在处理的请求数 |

---

## 服务健康指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_service_health_status` | Gauge | service | 服务健康状态 (0=unknown, 1=healthy, 2=degraded, 3=failed) |
| `nas_os_service_uptime_seconds` | Gauge | service | 服务运行时间 |

---

## 告警指标

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_alerts_total` | Gauge | type, level | 告警总数 |
| `nas_os_alerts_active` | Gauge | - | 活跃告警数 |

---

## 健康评分

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `nas_os_health_score` | Gauge | - | 系统健康评分 (0-100) |

---

## 使用示例

### PromQL 查询示例

```promql
# CPU 使用率
nas_os_cpu_usage_percent

# 内存使用率
nas_os_memory_usage_percent

# 磁盘使用率最高的挂载点
topk(5, nas_os_disk_usage_percent)

# API 请求速率（QPS）
rate(nas_os_api_requests_total[5m])

# API P95 延迟
histogram_quantile(0.95, rate(nas_os_api_request_duration_seconds_bucket[5m]))

# 存储池健康状态
nas_os_storage_pool_health_status

# 系统健康评分趋势
nas_os_health_score

# 活跃告警数
nas_os_alerts_active

# 备份成功率
sum(nas_os_backup_status{status="completed"}) / sum(nas_os_backup_total)
```

### Grafana Dashboard 示例

```json
{
  "title": "NAS-OS Overview",
  "panels": [
    {
      "title": "System Health Score",
      "type": "gauge",
      "targets": [{"expr": "nas_os_health_score"}]
    },
    {
      "title": "CPU & Memory",
      "type": "graph",
      "targets": [
        {"expr": "nas_os_cpu_usage_percent", "legendFormat": "CPU"},
        {"expr": "nas_os_memory_usage_percent", "legendFormat": "Memory"}
      ]
    },
    {
      "title": "Disk Usage",
      "type": "graph",
      "targets": [
        {"expr": "nas_os_disk_usage_percent", "legendFormat": "{{mountpoint}}"}
      ]
    },
    {
      "title": "API Latency (P95)",
      "type": "graph",
      "targets": [
        {"expr": "histogram_quantile(0.95, rate(nas_os_api_request_duration_seconds_bucket[5m]))"}
      ]
    }
  ]
}
```

---

**最后更新**: 2026-03-15
**版本**: v2.31.0
**维护**: 工部 (DevOps Team)