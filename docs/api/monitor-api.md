# 监控 API 文档

**版本**: v2.67.0  
**更新日期**: 2026-03-15

## 概述

监控 API 提供系统状态监控、分布式监控、告警管理等功能。

## 基础路径

```
/api/v1/monitor
```

---

## 系统监控

### 获取系统统计信息

```http
GET /api/v1/monitor/system
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "cpu_usage": 35.2,
    "cpu_cores": 8,
    "cpu_model": "ARM Cortex-A76",
    "memory_usage": 62.5,
    "memory_total": 8589934592,
    "memory_used": 5368709120,
    "memory_free": 3221225472,
    "swap_usage": 10.2,
    "swap_total": 4294967296,
    "swap_used": 438937600,
    "load_avg_1": 1.25,
    "load_avg_5": 1.10,
    "load_avg_15": 0.95,
    "uptime_secs": 864000,
    "process_count": 156,
    "hostname": "nas-server"
  }
}
```

### 获取磁盘统计信息

```http
GET /api/v1/monitor/disks
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "device": "sda",
      "mount_point": "/",
      "total_bytes": 1073741824000,
      "used_bytes": 536870912000,
      "free_bytes": 536870912000,
      "usage_percent": 50.0,
      "read_bytes": 1073741824,
      "write_bytes": 536870912,
      "read_ops": 15000,
      "write_ops": 8000,
      "io_latency_ms": 2.5,
      "temperature": 42,
      "health_status": "healthy"
    }
  ]
}
```

### 获取网络统计信息

```http
GET /api/v1/monitor/network
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "interfaces": [
      {
        "name": "eth0",
        "rx_bytes": 1073741824,
        "tx_bytes": 536870912,
        "rx_packets": 1000000,
        "tx_packets": 500000,
        "rx_errors": 0,
        "tx_errors": 0,
        "bandwidth_in_mbps": 125.5,
        "bandwidth_out_mbps": 62.3
      }
    ]
  }
}
```

### 获取 SMART 信息

```http
GET /api/v1/monitor/smart/:device
```

**请求示例**

```bash
curl -X GET "https://nas.local/api/v1/monitor/smart/sda" \
  -H "Authorization: Bearer <token>"
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "device": "sda",
    "model": "WDC WD10EZEX-00BN5A0",
    "serial": "WD-WCC3FXXXXXX",
    "temperature": 42,
    "health_score": 95,
    "power_on_hours": 8760,
    "power_cycle_count": 100,
    "reallocated_sectors": 0,
    "pending_sectors": 0,
    "attributes": [
      {
        "id": 5,
        "name": "Reallocated_Sector_Ct",
        "value": 100,
        "threshold": 10,
        "raw": 0
      },
      {
        "id": 194,
        "name": "Temperature_Celsius",
        "value": 42,
        "threshold": 0,
        "raw": 42
      }
    ]
  }
}
```

---

## 告警管理

### 获取告警列表

```http
GET /api/v1/monitor/alerts
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| level | string | 否 | 级别: warning/critical |
| type | string | 否 | 类型: cpu/memory/disk |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "alert-001",
      "type": "disk",
      "level": "warning",
      "message": "磁盘空间不足",
      "source": "sda",
      "timestamp": "2026-03-15T11:13:00Z",
      "acknowledged": false
    }
  ]
}
```

### 确认告警

```http
POST /api/v1/monitor/alerts/:id/acknowledge
```

### 删除告警

```http
DELETE /api/v1/monitor/alerts/:id
```

### 获取告警规则

```http
GET /api/v1/monitor/alerts/rules
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "name": "cpu-warning",
      "type": "cpu",
      "threshold": 80,
      "level": "warning",
      "enabled": true
    },
    {
      "name": "cpu-critical",
      "type": "cpu",
      "threshold": 95,
      "level": "critical",
      "enabled": true
    },
    {
      "name": "memory-warning",
      "type": "memory",
      "threshold": 85,
      "level": "warning",
      "enabled": true
    },
    {
      "name": "disk-warning",
      "type": "disk",
      "threshold": 85,
      "level": "warning",
      "enabled": true
    }
  ]
}
```

### 更新告警规则

```http
PUT /api/v1/monitor/alerts/rules/:name
```

**请求体**

```json
{
  "threshold": 90,
  "enabled": true
}
```

---

## 分布式监控

### 获取集群节点状态

```http
GET /api/v1/monitor/cluster/nodes
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "node-001",
      "name": "nas-node-1",
      "address": "192.168.1.10",
      "port": 8080,
      "is_active": true,
      "is_leader": true,
      "last_heartbeat": "2026-03-15T11:12:55Z"
    },
    {
      "id": "node-002",
      "name": "nas-node-2",
      "address": "192.168.1.11",
      "port": 8080,
      "is_active": true,
      "is_leader": false,
      "last_heartbeat": "2026-03-15T11:12:58Z"
    }
  ]
}
```

### 获取节点指标

```http
GET /api/v1/monitor/cluster/nodes/:node_id/metrics
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "node_id": "node-001",
    "node_name": "nas-node-1",
    "timestamp": "2026-03-15T11:13:00Z",
    "system_metrics": {
      "cpu_usage": 35.2,
      "memory_usage": 62.5,
      "load_avg_1": 1.25
    },
    "disk_metrics": [
      {
        "device": "sda",
        "usage_percent": 50.0,
        "io_latency_ms": 2.5
      }
    ],
    "storage_metrics": [
      {
        "pool_name": "main",
        "pool_type": "btrfs",
        "usage_percent": 45.0,
        "health_status": "healthy"
      }
    ],
    "health_score": 95.5,
    "status": "healthy"
  }
}
```

### 获取存储池指标

```http
GET /api/v1/monitor/storage/pools
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "pool_name": "main",
      "pool_type": "btrfs",
      "total_bytes": 2147483648000,
      "used_bytes": 966367641600,
      "free_bytes": 1181116006400,
      "usage_percent": 45.0,
      "health_status": "healthy",
      "raid_level": "raid1",
      "device_count": 2,
      "healthy_devices": 2,
      "failed_devices": 0,
      "io_stats": {
        "read_bytes_per_sec": 52428800,
        "write_bytes_per_sec": 26214400,
        "read_iops": 1500,
        "write_iops": 800,
        "avg_latency_ms": 2.5
      }
    }
  ]
}
```

### 获取健康评分

```http
GET /api/v1/monitor/health/score
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "overall_score": 95.5,
    "components": {
      "cpu": {
        "score": 98.0,
        "status": "healthy",
        "details": {}
      },
      "memory": {
        "score": 92.0,
        "status": "healthy",
        "details": {
          "usage_percent": 62.5
        }
      },
      "storage": {
        "score": 96.0,
        "status": "healthy",
        "details": {
          "healthy_pools": 2,
          "total_pools": 2
        }
      },
      "network": {
        "score": 100.0,
        "status": "healthy",
        "details": {}
      }
    },
    "recommendations": []
  }
}
```

---

## Prometheus 集成

### 获取 Prometheus 指标

```http
GET /api/v1/monitor/prometheus/metrics
```

**响应示例**

```
# HELP nas_cpu_usage CPU usage percentage
# TYPE nas_cpu_usage gauge
nas_cpu_usage{host="nas-server"} 35.2

# HELP nas_memory_usage Memory usage percentage
# TYPE nas_memory_usage gauge
nas_memory_usage{host="nas-server"} 62.5

# HELP nas_disk_usage_percent Disk usage percentage
# TYPE nas_disk_usage_percent gauge
nas_disk_usage_percent{device="sda",host="nas-server"} 50.0
```

---

## 数据模型

### SystemMetricData 系统指标

```typescript
interface SystemMetricData {
  cpu_usage: number;
  cpu_cores: number;
  memory_usage: number;
  memory_total: number;
  memory_used: number;
  load_avg_1: number;
  load_avg_5: number;
  load_avg_15: number;
  uptime_secs: number;
  process_count: number;
}
```

### DiskMetricData 磁盘指标

```typescript
interface DiskMetricData {
  device: string;
  mount_point: string;
  total_bytes: number;
  used_bytes: number;
  free_bytes: number;
  usage_percent: number;
  read_bytes: number;
  write_bytes: number;
  read_ops: number;
  write_ops: number;
  io_latency_ms: number;
  temperature: number;
  health_status: string;
}
```

### StoragePoolMetric 存储池指标

```typescript
interface StoragePoolMetric {
  pool_name: string;
  pool_type: string;        // btrfs, zfs, mdadm
  total_bytes: number;
  used_bytes: number;
  free_bytes: number;
  usage_percent: number;
  health_status: string;
  raid_level: string;
  device_count: number;
  healthy_devices: number;
  failed_devices: number;
  io_stats: StorageIOStats;
  rebuild_status?: RebuildStatus;
  alerts: PoolAlertState[];
}
```

### Alert 告警

```typescript
interface Alert {
  id: string;
  type: string;           // cpu, memory, disk, network
  level: string;          // info, warning, critical
  message: string;
  source: string;
  timestamp: string;
  acknowledged: boolean;
}
```

### AlertRule 告警规则

```typescript
interface AlertRule {
  name: string;
  type: string;
  threshold: number;
  level: string;
  enabled: boolean;
}
```