# NAS-OS API v2.586.0

> 文档版本: v2.586.0 | 更新日期: 2026-06-10

---

## 目录

1. [智能摄像头管理](#1-智能摄像头管理)
2. [AI 存储健康预测](#2-ai-存储健康预测)
3. [智能存储成本优化器](#3-智能存储成本优化器)
4. [WebDAV 增强](#4-webdav-增强)
5. [智能网络流量分析](#5-智能网络流量分析)

---

## 1. 智能摄像头管理

### 1.1 发现摄像头

自动扫描局域网内支持 ONVIF/SRTSP/RTMP 协议的摄像头设备。

```
POST /api/v1/cameras/discover
```

**请求体:**
```json
{
  "interface": "eth0",
  "timeout": 30,
  "protocols": ["onvif", "srtsp", "rtmp"]
}
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "cameras": [
      {
        "id": "cam-001",
        "name": "前门摄像头",
        "ip": "192.168.1.101",
        "protocol": "onvif",
        "manufacturer": "Hikvision",
        "model": "DS-2CD2143G2-I",
        "capabilities": ["motion_detection", "night_vision", "ptz"]
      }
    ],
    "total": 1,
    "scan_duration": 12.5
  }
}
```

### 1.2 管理摄像头

```
GET    /api/v1/cameras                    # 获取摄像头列表
POST   /api/v1/cameras                    # 添加摄像头
GET    /api/v1/cameras/:id                 # 获取摄像头详情
PUT    /api/v1/cameras/:id                 # 更新摄像头配置
DELETE /api/v1/cameras/:id                 # 删除摄像头
```

**请求示例 (添加摄像头):**
```json
{
  "name": "车库摄像头",
  "ip": "192.168.1.102",
  "protocol": "onvif",
  "username": "admin",
  "password": "encrypted:base64...",
  "stream_url": "rtsp://192.168.1.102:554/stream1",
  "motion_detection": {
    "enabled": true,
    "sensitivity": 75,
    "zones": [
      {
        "name": "入口区域",
        "coordinates": [[0, 0], [100, 0], [100, 100], [0, 100]]
      }
    ]
  }
}
```

### 1.3 录像管理

```
GET    /api/v1/cameras/:id/recordings                # 获取录像列表
GET    /api/v1/cameras/:id/recordings/:rid            # 获取录像详情
DELETE /api/v1/cameras/:id/recordings/:rid            # 删除录像
POST   /api/v1/cameras/:id/recordings/export          # 导出录像
```

**查询参数:**
```
GET /api/v1/cameras/cam-001/recordings?start=2026-06-09T00:00:00Z&end=2026-06-10T00:00:00Z&type=motion&page=1&limit=50
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "recordings": [
      {
        "id": "rec-001",
        "camera_id": "cam-001",
        "start_time": "2026-06-09T08:15:30Z",
        "end_time": "2026-06-09T08:16:45Z",
        "type": "motion",
        "size": 15728640,
        "thumbnail": "/api/v1/cameras/cam-001/recordings/rec-001/thumbnail",
        "motion_events": 3
      }
    ],
    "total": 1,
    "storage_used": "2.3 GB",
    "retention_days": 30
  }
}
```

### 1.4 移动侦测

```
POST   /api/v1/cameras/:id/motion/start              # 启用移动侦测
POST   /api/v1/cameras/:id/motion/stop               # 停用移动侦测
GET    /api/v1/cameras/:id/motion/events              # 获取移动侦测事件
PUT    /api/v1/cameras/:id/motion/config              # 更新侦测配置
```

**移动侦测事件响应:**
```json
{
  "status": "success",
  "data": {
    "events": [
      {
        "id": "evt-001",
        "camera_id": "cam-001",
        "timestamp": "2026-06-09T08:15:30Z",
        "zone": "入口区域",
        "confidence": 92.5,
        "snapshot": "/api/v1/events/evt-001/snapshot",
        "recording_id": "rec-001"
      }
    ]
  }
}
```

---

## 2. AI 存储健康预测

### 2.1 磁盘健康评分

基于 SMART 数据和 ML 模型评估磁盘健康状态。

```
GET /api/v1/storage/health/scores
GET /api/v1/storage/health/scores/:disk_id
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "scores": [
      {
        "disk_id": "sd-001",
        "device": "/dev/sda",
        "model": "Seagate ST8000VN004",
        "health_score": 87,
        "status": "healthy",
        "smart_status": "passed",
        "key_indicators": {
          "reallocated_sectors": 0,
          "pending_sectors": 0,
          "uncorrectable_errors": 0,
          "temperature": 38,
          "power_on_hours": 12500
        },
        "trend": "stable"
      }
    ],
    "pool_health": {
      "pool_name": "tank",
      "overall_score": 85,
      "degraded": false,
      "warnings": []
    }
  }
}
```

### 2.2 故障预测

```
GET /api/v1/storage/health/predictions
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "predictions": [
      {
        "disk_id": "sd-003",
        "device": "/dev/sdc",
        "failure_probability": 0.15,
        "predicted_failure_date": "2027-03-15",
        "confidence": 78,
        "risk_level": "low",
        "recommendations": [
          "建议在 6 个月内更换磁盘",
          "当前磁盘运行 28,000 小时，接近使用寿命"
        ]
      }
    ],
    "model_version": "2.1.0",
    "last_trained": "2026-06-01T00:00:00Z"
  }
}
```

### 2.3 维护建议

```
GET /api/v1/storage/health/maintenance
POST /api/v1/storage/health/maintenance/:action_id/approve
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "recommendations": [
      {
        "id": "maint-001",
        "priority": "high",
        "action": "replace_disk",
        "disk_id": "sd-003",
        "reason": "故障概率超过阈值，预测 3 个月内可能故障",
        "estimated_downtime": "2 小时",
        "cost_estimate": 1200
      },
      {
        "id": "maint-002",
        "priority": "medium",
        "action": "scrub_pool",
        "pool": "tank",
        "reason": "上次 scrub 已超过 30 天",
        "estimated_duration": "4 小时"
      }
    ]
  }
}
```

---

## 3. 智能存储成本优化器

### 3.1 成本分析

```
GET /api/v1/storage/cost/analysis
```

**查询参数:**
```
?period=monthly&pool=tank&include_cloud=false
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "period": "2026-05",
    "total_cost": 2500,
    "breakdown": {
      "storage": {
        "cost": 1800,
        "used_tb": 12.5,
        "cost_per_tb": 144
      },
      "energy": {
        "cost": 450,
        "kwh": 300,
        "cost_per_kwh": 1.5
      },
      "maintenance": {
        "cost": 250,
        "includes": ["disk_replacement", "cooling", "ups"]
      }
    },
    "trend": {
      "3_month_growth_rate": 0.08,
      "projected_annual_cost": 32000
    }
  }
}
```

### 3.2 ROI 计算

```
POST /api/v1/storage/cost/roi
```

**请求体:**
```json
{
  "scenario": "add_ssd_cache",
  "params": {
    "ssd_capacity_tb": 2,
    "ssd_cost": 3000,
    "expected_iops_improvement": 5,
    "workload": "random_read"
  }
}
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "scenario": "add_ssd_cache",
    "roi": {
      "investment": 3000,
      "annual_savings": 1200,
      "payback_months": 30,
      "5_year_roi": 1.0,
      "details": {
        "performance_gain": "5x IOPS 提升",
        "energy_savings": "年节省 200 kWh",
        "productivity_gain": "响应时间减少 80%"
      }
    },
    "alternatives": [
      {
        "scenario": "upgrade_network_to_10g",
        "roi": 0.85,
        "payback_months": 36
      }
    ]
  }
}
```

### 3.3 容量规划建议

```
GET /api/v1/storage/cost/planning
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "current_usage": {
      "total_tb": 48,
      "used_tb": 36,
      "utilization": 0.75
    },
    "projections": [
      {
        "months": 12,
        "projected_usage_tb": 45,
        "recommended_capacity_tb": 64,
        "action": "add_disk",
        "estimated_cost": 2400
      },
      {
        "months": 36,
        "projected_usage_tb": 72,
        "recommended_action": "expand_pool",
        "estimated_cost": 8000
      }
    ],
    "savings_opportunities": [
      {
        "type": "tiered_storage",
        "description": "将冷数据迁移至低成本存储",
        "annual_savings": 1500
      }
    ]
  }
}
```

---

## 4. WebDAV 增强

### 4.1 连接池配置

```
GET    /api/v1/webdav/config
PUT    /api/v1/webdav/config
```

**配置示例:**
```json
{
  "connection_pool": {
    "max_connections": 100,
    "idle_timeout": 300,
    "max_idle_per_host": 10
  },
  "compression": {
    "enabled": true,
    "algorithms": ["gzip", "brotli"],
    "min_size": 1024
  },
  "performance": {
    "buffer_size": 131072,
    "readahead": true,
    "readahead_size": 262144
  }
}
```

### 4.2 批量操作

```
POST /api/v1/webdav/batch/upload
POST /api/v1/webdav/batch/delete
POST /api/v1/webdav/batch/move
POST /api/v1/webdav/batch/copy
```

**批量上传请求:**
```json
{
  "destination": "/shared/photos/2026/",
  "files": [
    {
      "source": "/tmp/photo1.jpg",
      "name": "beach-01.jpg"
    },
    {
      "source": "/tmp/photo2.jpg",
      "name": "beach-02.jpg"
    }
  ],
  "options": {
    "overwrite": false,
    "checksum": "sha256",
    "parallel": 4
  }
}
```

**批量操作响应:**
```json
{
  "status": "success",
  "data": {
    "batch_id": "batch-001",
    "total": 2,
    "completed": 2,
    "failed": 0,
    "results": [
      {
        "file": "beach-01.jpg",
        "status": "success",
        "size": 5242880,
        "checksum": "sha256:abc123..."
      },
      {
        "file": "beach-02.jpg",
        "status": "success",
        "size": 4194304,
        "checksum": "sha256:def456..."
      }
    ],
    "duration": 1.2
  }
}
```

### 4.3 锁管理

```
LOCK   /api/v1/webdav/files/:path
UNLOCK /api/v1/webdav/files/:path
GET    /api/v1/webdav/locks
```

**锁定请求:**
```
LOCK /api/v1/webdav/files/documents/report.docx
Content-Type: application/xml

<?xml version="1.0" encoding="utf-8"?>
<D:lockinfo xmlns:D="DAV:">
  <D:lockscope><D:exclusive/></D:lockscope>
  <D:locktype><D:write/></D:locktype>
  <D:owner>
    <D:href>/users/alice</D:href>
  </D:owner>
</D:lockinfo>
```

---

## 5. 智能网络流量分析

### 5.1 应用层协议识别

```
GET /api/v1/network/traffic/protocols
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "protocols": [
      {
        "name": "HTTP/HTTPS",
        "bytes": 1073741824,
        "packets": 1250000,
        "percentage": 45.2,
        "top_hosts": ["nas.local", "cloud.backup.com"]
      },
      {
        "name": "SMB/CIFS",
        "bytes": 536870912,
        "packets": 890000,
        "percentage": 22.6,
        "top_clients": ["192.168.1.50", "192.168.1.51"]
      },
      {
        "name": "NFS",
        "bytes": 268435456,
        "packets": 450000,
        "percentage": 11.3,
        "top_clients": ["192.168.1.100"]
      }
    ],
    "period": "last_24h",
    "total_bytes": 2375680000
  }
}
```

### 5.2 带宽趋势分析

```
GET /api/v1/network/traffic/trends
```

**查询参数:**
```
?interval=hourly&period=7d&direction=both
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "period": "7d",
    "interval": "hourly",
    "trends": [
      {
        "timestamp": "2026-06-09T00:00:00Z",
        "inbound_bps": 125000000,
        "outbound_bps": 85000000,
        "peak_inbound_bps": 250000000,
        "peak_outbound_bps": 180000000,
        "connections": 45
      }
    ],
    "summary": {
      "avg_bandwidth_mbps": 85,
      "peak_bandwidth_mbps": 250,
      "busiest_hour": "20:00",
      "quietest_hour": "04:00"
    }
  }
}
```

### 5.3 异常流量检测

```
GET /api/v1/network/traffic/anomalies
```

**响应:**
```json
{
  "status": "success",
  "data": {
    "anomalies": [
      {
        "id": "anomaly-001",
        "timestamp": "2026-06-09T15:30:00Z",
        "type": "bandwidth_spike",
        "severity": "medium",
        "source_ip": "192.168.1.52",
        "destination": "external:45.33.32.156",
        "protocol": "HTTPS",
        "bytes": 5368709120,
        "duration": 1800,
        "description": "异常大流量上传，可能是数据泄露或勒索软件通信"
      }
    ],
    "total_anomalies": 1,
    "risk_score": 35
  }
}
```

### 5.4 QoS 策略推荐

```
GET  /api/v1/network/traffic/qos/recommendations
POST /api/v1/network/traffic/qos/policies
```

**推荐响应:**
```json
{
  "status": "success",
  "data": {
    "recommendations": [
      {
        "id": "qos-rec-001",
        "priority": "high",
        "action": "throttle",
        "target": {
          "protocol": "P2P",
          "ports": [6881, 6889]
        },
        "current_usage_mbps": 150,
        "recommended_limit_mbps": 50,
        "reason": "P2P 流量占用过多带宽，影响其他服务"
      },
      {
        "id": "qos-rec-002",
        "priority": "medium",
        "action": "prioritize",
        "target": {
          "protocol": "SMB",
          "clients": ["192.168.1.50", "192.168.1.51"]
        },
        "reason": "工作时间 SMB 访问延迟较高"
      }
    ]
  }
}
```

---

## 错误码

| 错误码 | 描述 |
|--------|------|
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 资源冲突（如摄像头已存在） |
| 422 | 请求体验证失败 |
| 500 | 服务器内部错误 |

---

## 认证

所有 API 端点需要通过 Bearer Token 认证：

```
Authorization: Bearer <your-api-token>
```

API Token 管理参见: `/api/v1/auth/tokens`

---

## 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v2.586.0 | 2026-06-10 | 智能摄像头管理、AI存储健康预测、成本优化器、WebDAV增强、流量分析增强 |
