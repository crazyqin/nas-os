# NAS-OS API 使用指南

**版本**: v1.7.0  
**更新日期**: 2026-03-13

---

## 📋 目录

1. [快速开始](#快速开始)
2. [认证](#认证)
3. [存储管理](#存储管理)
4. [用户权限](#用户权限)
5. [容器管理](#容器管理)
6. [虚拟机](#虚拟机)
7. [监控告警](#监控告警)
8. [性能优化](#性能优化)
9. [配额管理](#配额管理) 🆕
10. [回收站](#回收站) 🆕
11. [WebDAV](#webdav) 🆕
12. [存储复制](#存储复制) 🆕
13. [AI 分类](#ai-分类) 🆕

---

## 快速开始

### 基础 URL

```
http://localhost:8080/api/v1
```

### 认证

大部分 API 需要 JWT Token 认证：

```bash
# 登录获取 Token
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your_password"}'

# 响应
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2026-03-14T10:00:00Z"
}

# 使用 Token
curl http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

---

## 存储管理

### 列出所有卷

```bash
curl http://localhost:8080/api/v1/volumes
```

**响应**:
```json
{
  "volumes": [
    {
      "name": "data",
      "size": 1099511627776,
      "used": 549755813888,
      "usage_percent": 50.0,
      "raid": "raid1",
      "devices": ["/dev/sda", "/dev/sdb"]
    }
  ]
}
```

### 创建卷

```bash
curl -X POST http://localhost:8080/api/v1/volumes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "backup",
    "devices": ["/dev/sdc", "/dev/sdd"],
    "raid": "raid1"
  }'
```

### 创建快照

```bash
curl -X POST http://localhost:8080/api/v1/volumes/data/snapshots \
  -H "Authorization: Bearer TOKEN" \
  -d '{"name": "backup-2026-03-13"}'
```

### 列出快照

```bash
curl http://localhost:8080/api/v1/volumes/data/snapshots
```

---

## 用户权限

### 创建用户

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "username": "newuser",
    "password": "SecurePass123",
    "role": "user",
    "email": "user@example.com"
  }'
```

### 分配角色

```bash
curl -X POST http://localhost:8080/api/v1/rbac/users/user123/roles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"role": "admin"}'
```

### 检查权限

```bash
curl -X POST http://localhost:8080/api/v1/rbac/check \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "user_id": "user123",
    "groups": ["administrators"],
    "resource": "volume",
    "action": "write"
  }'
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "allowed": true
  }
}
```

---

## 容器管理

### 列出容器

```bash
curl http://localhost:8080/api/v1/containers
```

### 创建容器

```bash
curl -X POST http://localhost:8080/api/v1/containers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "nginx",
    "image": "nginx:latest",
    "ports": {"80/tcp": 8080},
    "volumes": ["/data/nginx:/usr/share/nginx/html"]
  }'
```

### 启动容器

```bash
curl -X POST http://localhost:8080/api/v1/containers/nginx/start \
  -H "Authorization: Bearer TOKEN"
```

### 停止容器

```bash
curl -X POST http://localhost:8080/api/v1/containers/nginx/stop \
  -H "Authorization: Bearer TOKEN"
```

---

## 虚拟机

### 列出 VM

```bash
curl http://localhost:8080/api/v1/vms
```

### 创建 VM

```bash
curl -X POST http://localhost:8080/api/v1/vms \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "ubuntu-vm",
    "cpu": 2,
    "memory": 4096,
    "disk_size": 50000000000,
    "iso": "ubuntu-22.04.iso"
  }'
```

### 创建 VM 快照

```bash
curl -X POST http://localhost:8080/api/v1/vms/ubuntu-vm/snapshot \
  -H "Authorization: Bearer TOKEN" \
  -d '{"name": "before-update"}'
```

---

## 监控告警

### 获取系统统计

```bash
curl http://localhost:8080/api/v1/monitor/stats
```

**响应**:
```json
{
  "cpu_usage": 25.5,
  "memory_usage": 45.2,
  "memory_total": 8589934592,
  "memory_used": 3882827776,
  "disk_usage": 67.3,
  "network_rx": 1234567890,
  "network_tx": 9876543210,
  "uptime": "15d 8h 32m",
  "timestamp": "2026-03-13T10:00:00Z"
}
```

### 获取活动告警

```bash
curl http://localhost:8080/api/v1/monitor/alerts?limit=10
```

### 确认告警

```bash
curl -X POST http://localhost:8080/api/v1/monitor/alerts/alert-123/acknowledge \
  -H "Authorization: Bearer TOKEN"
```

### 解决告警

```bash
curl -X POST http://localhost:8080/api/v1/monitor/alerts/alert-123/resolve \
  -H "Authorization: Bearer TOKEN"
```

---

## 性能优化

### 获取性能统计

```bash
curl http://localhost:8080/api/v1/optimizer/stats
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "cache": {
      "hits": 1234,
      "misses": 56,
      "hit_ratio": 0.956
    },
    "gc": {
      "count": 42,
      "pause_total": "125ms",
      "pause_avg": "2.9ms"
    },
    "memory": {
      "alloc": 45678912,
      "total": 123456789
    },
    "goroutines": 45
  }
}
```

### 更新优化配置

```bash
curl -X PUT http://localhost:8080/api/v1/optimizer/config \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "cache_capacity": 20000,
    "cache_ttl": "10m",
    "batch_size": 200
  }'
```

### 强制 GC

```bash
curl -X POST http://localhost:8080/api/v1/optimizer/gc \
  -H "Authorization: Bearer TOKEN"
```

### 获取内存详情

```bash
curl http://localhost:8080/api/v1/optimizer/memory
```

---

## 配额管理 🆕

### 获取配额列表

```bash
curl http://localhost:8080/api/v1/quota \
  -H "Authorization: Bearer TOKEN"
```

### 设置用户配额

```bash
curl -X POST http://localhost:8080/api/v1/quota/users/user123 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"limit": 107374182400}'  # 100GB
```

### 设置目录配额

```bash
curl -X POST http://localhost:8080/api/v1/quota/dirs/data/photos \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"limit": 536870912000}'  # 500GB
```

### 获取配额使用情况

```bash
curl http://localhost:8080/api/v1/quota/users/user123/usage \
  -H "Authorization: Bearer TOKEN"
```

---

## 回收站 🆕

### 列出回收站文件

```bash
curl http://localhost:8080/api/v1/trash \
  -H "Authorization: Bearer TOKEN"
```

### 恢复文件

```bash
curl -X POST http://localhost:8080/api/v1/trash/trash-123/restore \
  -H "Authorization: Bearer TOKEN"
```

### 永久删除

```bash
curl -X DELETE http://localhost:8080/api/v1/trash/trash-123 \
  -H "Authorization: Bearer TOKEN"
```

### 清空回收站

```bash
curl -X DELETE http://localhost:8080/api/v1/trash \
  -H "Authorization: Bearer TOKEN"
```

---

## WebDAV 🆕

### 获取 WebDAV 配置

```bash
curl http://localhost:8080/api/v1/webdav/config \
  -H "Authorization: Bearer TOKEN"
```

### 更新 WebDAV 配置

```bash
curl -X PUT http://localhost:8080/api/v1/webdav/config \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "enabled": true,
    "port": 8090,
    "root_path": "/data/webdav",
    "read_only": false
  }'
```

### 获取 WebDAV 状态

```bash
curl http://localhost:8080/api/v1/webdav/status \
  -H "Authorization: Bearer TOKEN"
```

---

## 存储复制 🆕

### 获取复制任务列表

```bash
curl http://localhost:8080/api/v1/replication \
  -H "Authorization: Bearer TOKEN"
```

### 创建复制任务

```bash
curl -X POST http://localhost:8080/api/v1/replication \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "name": "backup-repl",
    "source_path": "/data/important",
    "target_node": "node-2",
    "target_path": "/backup/important",
    "mode": "async",
    "schedule": "0 */6 * * *"
  }'
```

### 获取复制状态

```bash
curl http://localhost:8080/api/v1/replication/backup-repl/status \
  -H "Authorization: Bearer TOKEN"
```

### 手动触发同步

```bash
curl -X POST http://localhost:8080/api/v1/replication/backup-repl/sync \
  -H "Authorization: Bearer TOKEN"
```

---

## AI 分类 🆕

### 分类文件

```bash
curl -X POST http://localhost:8080/api/v1/ai-classify/classify \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "path": "/data/photos/vacation.jpg",
    "type": "image"
  }'
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "categories": ["vacation", "beach", "sunset"],
    "confidence": 0.92,
    "suggested_tags": ["summer", "travel", "nature"]
  }
}
```

### 批量分类

```bash
curl -X POST http://localhost:8080/api/v1/ai-classify/batch \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "paths": ["/data/photos/1.jpg", "/data/photos/2.jpg"],
    "type": "image"
  }'
```

### 获取分类统计

```bash
curl http://localhost:8080/api/v1/ai-classify/stats \
  -H "Authorization: Bearer TOKEN"
```

---

## 错误处理

### 错误响应格式

```json
{
  "code": 400,
  "message": "Invalid input: username is required",
  "data": null
}
```

### 常见错误码

| 代码 | 说明 |
|------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 500 | 服务器内部错误 |

---

## 速率限制

- 默认：100 请求/分钟
- 认证用户：500 请求/分钟
- 管理员：无限制

---

## 完整 API 文档

查看 Swagger 文档：http://localhost:8080/swagger/index.html

或下载 OpenAPI 规范：
- JSON: http://localhost:8080/openapi.json
- YAML: http://localhost:8080/openapi.yaml

---

## SDK 示例

### Python

```python
import requests

BASE_URL = "http://localhost:8080/api/v1"

# 登录
resp = requests.post(f"{BASE_URL}/login", json={
    "username": "admin",
    "password": "password"
})
token = resp.json()["token"]

headers = {"Authorization": f"Bearer {token}"}

# 获取卷列表
resp = requests.get(f"{BASE_URL}/volumes", headers=headers)
volumes = resp.json()["volumes"]
print(f"Found {len(volumes)} volumes")
```

### Go

```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    client := &http.Client{}
    req, _ := http.NewRequest("GET", "http://localhost:8080/api/v1/volumes", nil)
    req.Header.Set("Authorization", "Bearer TOKEN")
    
    resp, _ := client.Do(req)
    fmt.Println(resp.Status)
}
```

---

**最后更新**: 2026-03-13
