# NAS-OS API 使用指南

**版本**: v2.99.0  
**更新日期**: 2026-03-16

---

## 📋 目录

1. [快速开始](#快速开始)
2. [认证](#认证)
3. [存储管理](#存储管理)
4. [用户权限](#用户权限)
5. [LDAP/AD 集成](#ldapad-集成-)
6. [容器管理](#容器管理)
7. [虚拟机](#虚拟机)
8. [监控告警](#监控告警)
9. [性能优化](#性能优化)
10. [配额管理](#配额管理)
11. [回收站](#回收站)
12. [WebDAV](#webdav)
13. [存储复制](#存储复制)
14. [AI 分类](#ai-分类)
15. [文件版本控制](#文件版本控制)
16. [云同步](#云同步)
17. [数据去重](#数据去重)
18. [iSCSI 目标](#iscsi-目标-)
19. [快照策略](#快照策略-)
20. [存储分层](#存储分层-)
21. [FTP 服务器](#ftp-服务器-)
22. [SFTP 服务器](#sftp-服务器-)
23. [文件标签](#文件标签-)
24. [请求日志](#请求日志-)
25. [Excel 导出](#excel-导出-)
26. [成本分析](#成本分析--v2360)
27. [API Gateway](#api-gateway--v2360)

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

## LDAP/AD 集成 🆕

NAS-OS 支持与企业 LDAP 和 Active Directory 集成，实现统一身份认证。

### 配置管理

#### 获取所有 LDAP 配置

```bash
curl http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "name": "company-ldap",
      "url": "ldaps://ldap.example.com:636",
      "base_dn": "dc=example,dc=com",
      "enabled": true,
      "is_ad": false
    }
  ]
}
```

#### 创建 LDAP 配置

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "company-ldap",
    "url": "ldaps://ldap.example.com:636",
    "bind_dn": "cn=admin,dc=example,dc=com",
    "bind_password": "admin-password",
    "base_dn": "dc=example,dc=com",
    "user_filter": "(uid=%s)",
    "group_filter": "(memberUid=%s)",
    "attribute_map": {
      "username": "uid",
      "email": "mail",
      "first_name": "givenName",
      "last_name": "sn",
      "full_name": "cn",
      "groups": "memberOf"
    },
    "use_tls": true,
    "enabled": true,
    "is_ad": false
  }'
```

#### 创建 Active Directory 配置

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "company-ad",
    "url": "ldaps://ad.example.com:636",
    "bind_dn": "CN=ldap-bind,CN=Users,DC=example,DC=com",
    "bind_password": "bind-password",
    "base_dn": "DC=example,DC=com",
    "user_filter": "(sAMAccountName=%s)",
    "group_filter": "(member=%s)",
    "attribute_map": {
      "username": "sAMAccountName",
      "email": "mail",
      "first_name": "givenName",
      "last_name": "sn",
      "full_name": "displayName",
      "groups": "memberOf"
    },
    "use_tls": true,
    "enabled": true,
    "is_ad": true
  }'
```

#### 获取指定配置

```bash
curl http://localhost:8080/api/v1/ldap/configs/company-ldap \
  -H "Authorization: Bearer TOKEN"
```

#### 更新配置

```bash
curl -X PUT http://localhost:8080/api/v1/ldap/configs/company-ldap \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "use_tls": true
  }'
```

#### 删除配置

```bash
curl -X DELETE http://localhost:8080/api/v1/ldap/configs/company-ldap \
  -H "Authorization: Bearer TOKEN"
```

### 连接测试

#### 测试 LDAP 连接

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs/company-ldap/test \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "message": "连接成功",
  "data": {
    "server_type": "OpenLDAP",
    "server_version": "2.6.0",
    "response_time": "45ms"
  }
}
```

### 用户认证

#### LDAP 用户登录

```bash
curl -X POST http://localhost:8080/api/v1/ldap/auth \
  -H "Content-Type: application/json" \
  -d '{
    "config_name": "company-ldap",
    "username": "zhangsan",
    "password": "user-password"
  }'
```

**成功响应**:
```json
{
  "code": 0,
  "message": "认证成功",
  "data": {
    "username": "zhangsan",
    "email": "zhangsan@example.com",
    "first_name": "San",
    "last_name": "Zhang",
    "full_name": "Zhang San",
    "groups": ["developers", "admins"],
    "dn": "uid=zhangsan,ou=users,dc=example,dc=com"
  }
}
```

**失败响应**:
```json
{
  "code": 401,
  "message": "LDAP 认证失败",
  "data": null
}
```

### 用户搜索

#### 搜索 LDAP 用户

```bash
curl "http://localhost:8080/api/v1/ldap/users/search?config_name=company-ldap&query=zhang" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "username": "zhangsan",
      "email": "zhangsan@example.com",
      "full_name": "Zhang San",
      "dn": "uid=zhangsan,ou=users,dc=example,dc=com"
    },
    {
      "username": "zhangwei",
      "email": "zhangwei@example.com",
      "full_name": "Zhang Wei",
      "dn": "uid=zhangwei,ou=users,dc=example,dc=com"
    }
  ]
}
```

### 组映射管理

#### 创建组映射

```bash
curl -X POST http://localhost:8080/api/v1/ldap/group-mappings \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "config_name": "company-ldap",
    "ldap_group": "nas-admins",
    "nas_role": "admin"
  }'
```

#### 列出组映射

```bash
curl http://localhost:8080/api/v1/ldap/group-mappings \
  -H "Authorization: Bearer TOKEN"
```

#### 删除组映射

```bash
curl -X DELETE http://localhost:8080/api/v1/ldap/group-mappings/mapping-001 \
  -H "Authorization: Bearer TOKEN"
```

### LDAP 配置参数说明

| 参数 | 说明 | OpenLDAP 示例 | AD 示例 |
|------|------|---------------|---------|
| `url` | LDAP 服务器地址 | `ldaps://ldap.example.com:636` | `ldaps://ad.example.com:636` |
| `bind_dn` | 绑定账号 DN | `cn=admin,dc=example,dc=com` | `CN=admin,CN=Users,DC=example,DC=com` |
| `base_dn` | 搜索基础 DN | `dc=example,dc=com` | `DC=example,DC=com` |
| `user_filter` | 用户搜索过滤器 | `(uid=%s)` | `(sAMAccountName=%s)` |
| `group_filter` | 组搜索过滤器 | `(memberUid=%s)` | `(member=%s)` |
| `is_ad` | 是否为 AD | `false` | `true` |

📖 详细配置请参考 [LDAP 集成指南](LDAP-INTEGRATION.md)

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

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "trash-1710123456789",
      "name": "old_file.txt",
      "original_path": "/data/documents/old_file.txt",
      "size": 1024,
      "is_dir": false,
      "deleted_at": "2026-03-20T10:00:00Z",
      "expires_at": "2026-04-19T10:00:00Z",
      "days_left": 30
    }
  ]
}
```

### 获取回收站文件详情

```bash
curl http://localhost:8080/api/v1/trash/trash-1710123456789 \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "trash-1710123456789",
    "name": "old_file.txt",
    "original_path": "/data/documents/old_file.txt",
    "size": 1024,
    "is_dir": false,
    "deleted_at": "2026-03-20T10:00:00Z",
    "expires_at": "2026-04-19T10:00:00Z",
    "days_left": 30
  }
}
```

### 恢复文件

```bash
# 恢复到原始路径
curl -X POST http://localhost:8080/api/v1/trash/trash-123/restore \
  -H "Authorization: Bearer TOKEN"

# 恢复到指定路径
curl -X POST http://localhost:8080/api/v1/trash/trash-123/restore \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"target_path": "/data/restored/file.txt"}'
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

### 获取回收站配置

```bash
curl http://localhost:8080/api/v1/trash/config \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "enabled": true,
    "retention_days": 30,
    "max_size": 10737418240,
    "auto_empty": true
  }
}
```

### 更新回收站配置

```bash
curl -X PUT http://localhost:8080/api/v1/trash/config \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "retention_days": 60,
    "max_size": 21474836480
  }'
```

### 获取回收站统计

```bash
curl http://localhost:8080/api/v1/trash/stats \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "total_items": 15,
    "total_size": 5368709120,
    "max_size": 10737418240,
    "usage_percent": 50.0,
    "retention_days": 30,
    "enabled": true
  }
}
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

## 文件版本控制 🆕

### 列出文件版本

```bash
curl "http://localhost:8080/api/v1/files/data/important.docx?versions=true" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "ver-20260320-001",
      "path": "/data/important.docx",
      "size": 102400,
      "checksum": "sha256:abc123...",
      "createdAt": "2026-03-20T10:00:00Z",
      "triggerType": "manual",
      "description": "重要修改前备份"
    }
  ]
}
```

### 创建文件版本

```bash
curl -X POST "http://localhost:8080/api/v1/files/data/important.docx/versions" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "重要修改前备份",
    "triggerType": "manual"
  }'
```

### 获取版本详情

```bash
curl "http://localhost:8080/api/v1/versions/ver-001" \
  -H "Authorization: Bearer TOKEN"
```

### 恢复版本

```bash
curl -X POST "http://localhost:8080/api/v1/versions/ver-001/restore" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"targetPath": ""}'
```

### 删除版本

```bash
curl -X DELETE "http://localhost:8080/api/v1/versions/ver-001" \
  -H "Authorization: Bearer TOKEN"
```

### 版本对比

```bash
curl "http://localhost:8080/api/v1/versions/ver-001/diff" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "versionId": "ver-001",
    "changes": [
      {
        "type": "modified",
        "line": 10,
        "oldContent": "原始内容",
        "newContent": "修改后内容"
      }
    ],
    "addedLines": 5,
    "removedLines": 2,
    "modifiedLines": 3
  }
}
```

### 获取版本统计

```bash
curl "http://localhost:8080/api/v1/versions/stats" \
  -H "Authorization: Bearer TOKEN"
```

### 配置管理

```bash
# 获取配置
curl "http://localhost:8080/api/v1/versions/config" \
  -H "Authorization: Bearer TOKEN"

# 更新配置
curl -X PUT "http://localhost:8080/api/v1/versions/config" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "autoVersion": true,
    "maxVersions": 100,
    "maxAge": "30d"
  }'
```

---

## 云同步 🆕

### 云存储提供商管理

#### 添加云存储

```bash
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的阿里云OSS",
    "type": "aliyun_oss",
    "endpoint": "oss-cn-hangzhou.aliyuncs.com",
    "bucket": "my-bucket",
    "accessKey": "YOUR_ACCESS_KEY",
    "secretKey": "YOUR_SECRET_KEY"
  }'
```

#### 列出云存储

```bash
curl "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN"
```

#### 删除云存储

```bash
curl -X DELETE "http://localhost:8080/api/v1/cloudsync/providers/provider-001" \
  -H "Authorization: Bearer TOKEN"
```

### 同步任务管理

#### 创建同步任务

```bash
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "照片备份",
    "providerId": "provider-001",
    "localPath": "/data/photos",
    "remotePath": "/backup/photos",
    "direction": "bidirect",
    "mode": "sync",
    "scheduleType": "realtime",
    "conflictStrategy": "newer"
  }'
```

#### 列出同步任务

```bash
curl "http://localhost:8080/api/v1/cloudsync/tasks" \
  -H "Authorization: Bearer TOKEN"
```

#### 执行同步

```bash
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks/task-001/run" \
  -H "Authorization: Bearer TOKEN"
```

#### 获取同步状态

```bash
curl "http://localhost:8080/api/v1/cloudsync/tasks/task-001/status" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "taskId": "task-001",
    "status": "running",
    "totalFiles": 1000,
    "processedFiles": 450,
    "totalBytes": 1073741824,
    "transferredBytes": 483183820,
    "speed": 2048,
    "progress": 45.0,
    "currentFile": "/data/photos/IMG_001.jpg",
    "uploadedFiles": 400,
    "downloadedFiles": 50
  }
}
```

#### 暂停/恢复/取消同步

```bash
# 暂停
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks/task-001/pause" \
  -H "Authorization: Bearer TOKEN"

# 恢复
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks/task-001/resume" \
  -H "Authorization: Bearer TOKEN"

# 取消
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks/task-001/cancel" \
  -H "Authorization: Bearer TOKEN"
```

### 获取同步统计

```bash
curl "http://localhost:8080/api/v1/cloudsync/stats" \
  -H "Authorization: Bearer TOKEN"
```

---

## 数据去重 🆕

### 扫描重复文件

```bash
curl -X POST "http://localhost:8080/api/v1/dedup/scan" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "paths": ["/data/photos", "/data/documents"]
  }'
```

### 获取重复文件列表

```bash
curl "http://localhost:8080/api/v1/dedup/duplicates" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "checksum": "sha256:abc123...",
      "size": 1048576,
      "files": [
        {"path": "/data/photos/IMG_001.jpg", "modTime": "2026-03-20T10:00:00Z"},
        {"path": "/data/backup/IMG_001.jpg", "modTime": "2026-03-19T15:00:00Z"}
      ],
      "potentialSaving": 1048576
    }
  ]
}
```

### 执行去重

```bash
curl -X POST "http://localhost:8080/api/v1/dedup/deduplicate" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "checksum": "sha256:abc123...",
    "keepPath": "/data/photos/IMG_001.jpg",
    "mode": "file",
    "action": "softlink"
  }'
```

### 批量去重

```bash
curl -X POST "http://localhost:8080/api/v1/dedup/deduplicate/all" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "file",
    "action": "softlink",
    "dryRun": true
  }'
```

### 获取去重报告

```bash
curl "http://localhost:8080/api/v1/dedup/report" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "totalFiles": 10000,
    "duplicateFiles": 1500,
    "duplicateGroups": 300,
    "potentialSaving": 5368709120,
    "potentialSavingHuman": "5.0 GB",
    "deduplicated": 200,
    "actualSaving": 1073741824,
    "actualSavingHuman": "1.0 GB"
  }
}
```

### 获取去重统计

```bash
curl "http://localhost:8080/api/v1/dedup/stats" \
  -H "Authorization: Bearer TOKEN"
```

### 配置管理

```bash
# 获取配置
curl "http://localhost:8080/api/v1/dedup/config" \
  -H "Authorization: Bearer TOKEN"

# 更新配置
curl -X PUT "http://localhost:8080/api/v1/dedup/config" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "autoDedup": false,
    "mode": "file",
    "action": "softlink",
    "minFileSize": 1024
  }'
```

### 自动去重

```bash
# 获取自动去重任务
curl "http://localhost:8080/api/v1/dedup/auto" \
  -H "Authorization: Bearer TOKEN"

# 启用自动去重
curl -X POST "http://localhost:8080/api/v1/dedup/auto/enable" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "schedule": "0 3 * * 0"
  }'

# 手动执行自动去重
curl -X POST "http://localhost:8080/api/v1/dedup/auto/run" \
  -H "Authorization: Bearer TOKEN"
```

---

## iSCSI 目标 🆕

### 获取 iSCSI 目标列表

```bash
curl http://localhost:8080/api/v1/iscsi/targets \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "target-001",
      "name": "target1",
      "iqn": "iqn.2026-03.com.nas-os:target1",
      "luns": [
        {"lun": 0, "size": 107374182400, "path": "/data/iscsi/lun0.img"}
      ],
      "sessions": 2,
      "enabled": true,
      "created_at": "2026-03-21T10:00:00Z"
    }
  ]
}
```

### 创建 iSCSI 目标

```bash
curl -X POST http://localhost:8080/api/v1/iscsi/targets \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "target1",
    "lun": 0,
    "path": "/data/iscsi/lun0.img",
    "size": 107374182400,
    "chap_user": "admin",
    "chap_password": "SecurePass123"
  }'
```

### 获取 iSCSI 目标详情

```bash
curl http://localhost:8080/api/v1/iscsi/targets/target1 \
  -H "Authorization: Bearer TOKEN"
```

### 更新 iSCSI 目标

```bash
curl -X PUT http://localhost:8080/api/v1/iscsi/targets/target1 \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "allowed_initiators": [
      "iqn.2026-03.com.vmware:esxi-host1"
    ]
  }'
```

### 删除 iSCSI 目标

```bash
curl -X DELETE http://localhost:8080/api/v1/iscsi/targets/target1 \
  -H "Authorization: Bearer TOKEN"
```

### 查看连接会话

```bash
curl http://localhost:8080/api/v1/iscsi/targets/target1/sessions \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "session_id": "sid-001",
      "initiator_iqn": "iqn.2026-03.com.vmware:esxi-host1",
      "connected_at": "2026-03-21T10:00:00Z",
      "bytes_read": 1073741824,
      "bytes_written": 2147483648
    }
  ]
}
```

---

## 快照策略 🆕

### 获取策略列表

```bash
curl http://localhost:8080/api/v1/snapshot-policies \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "policy-001",
      "name": "daily-backup",
      "volume": "data",
      "schedule": "0 2 * * *",
      "retention": {
        "count": 30,
        "max_age": "30d"
      },
      "enabled": true,
      "last_run": "2026-03-21T02:00:00Z",
      "next_run": "2026-03-22T02:00:00Z",
      "snapshot_count": 15
    }
  ]
}
```

### 创建策略

```bash
curl -X POST http://localhost:8080/api/v1/snapshot-policies \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "daily-backup",
    "volume": "data",
    "schedule": "0 2 * * *",
    "retention": {
      "count": 30,
      "max_age": "30d"
    },
    "enabled": true
  }'
```

### 获取策略详情

```bash
curl http://localhost:8080/api/v1/snapshot-policies/daily-backup \
  -H "Authorization: Bearer TOKEN"
```

### 更新策略

```bash
curl -X PUT http://localhost:8080/api/v1/snapshot-policies/daily-backup \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": "0 3 * * *",
    "retention": {
      "count": 60
    }
  }'
```

### 删除策略

```bash
curl -X DELETE http://localhost:8080/api/v1/snapshot-policies/daily-backup \
  -H "Authorization: Bearer TOKEN"
```

### 启用/禁用策略

```bash
# 启用
curl -X POST http://localhost:8080/api/v1/snapshot-policies/daily-backup/enable \
  -H "Authorization: Bearer TOKEN"

# 禁用
curl -X POST http://localhost:8080/api/v1/snapshot-policies/daily-backup/disable \
  -H "Authorization: Bearer TOKEN"
```

### 手动执行策略

```bash
curl -X POST http://localhost:8080/api/v1/snapshot-policies/daily-backup/run \
  -H "Authorization: Bearer TOKEN"
```

---

## 存储分层 🆕

### 获取存储层列表

```bash
curl http://localhost:8080/api/v1/tiering/tiers \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "type": "hot",
      "name": "SSD缓存层",
      "path": "/data/hot",
      "capacity": 536870912000,
      "used": 107374182400,
      "enabled": true
    },
    {
      "type": "cold",
      "name": "HDD存储层",
      "path": "/data/cold",
      "capacity": 10995116277760,
      "used": 2199023255552,
      "enabled": true
    }
  ]
}
```

### 获取存储层配置

```bash
curl http://localhost:8080/api/v1/tiering/tiers/hot \
  -H "Authorization: Bearer TOKEN"
```

### 更新存储层配置

```bash
curl -X PUT http://localhost:8080/api/v1/tiering/tiers/hot \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "SSD缓存层",
    "path": "/data/ssd-cache",
    "capacity": 1073741824000,
    "enabled": true
  }'
```

### 获取分层策略列表

```bash
curl http://localhost:8080/api/v1/tiering/policies \
  -H "Authorization: Bearer TOKEN"
```

### 创建分层策略

```bash
curl -X POST http://localhost:8080/api/v1/tiering/policies \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "照片归档",
    "source_tier": "hot",
    "target_tier": "cold",
    "rules": [
      {"type": "access_age", "days": 30},
      {"type": "file_size", "min_size": 1048576}
    ],
    "schedule": "0 3 * * *",
    "enabled": true
  }'
```

### 执行分层策略

```bash
curl -X POST http://localhost:8080/api/v1/tiering/policies/policy-001/execute \
  -H "Authorization: Bearer TOKEN"
```

### 手动迁移文件

```bash
curl -X POST http://localhost:8080/api/v1/tiering/migrate \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/data/photos/old_photos",
    "target_tier": "cold"
  }'
```

### 获取分层任务列表

```bash
curl http://localhost:8080/api/v1/tiering/tasks \
  -H "Authorization: Bearer TOKEN"
```

### 获取分层状态

```bash
curl http://localhost:8080/api/v1/tiering/status \
  -H "Authorization: Bearer TOKEN"
```

---

## FTP 服务器 🆕

### 获取 FTP 配置

```bash
curl http://localhost:8080/api/v1/ftp/config \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "enabled": true,
    "port": 21,
    "passive_ports": "50000-51000",
    "max_connections": 50,
    "anonymous": false,
    "bandwidth_limit": 10485760,
    "welcome_message": "Welcome to NAS-OS FTP Server"
  }
}
```

### 更新 FTP 配置

```bash
curl -X PUT http://localhost:8080/api/v1/ftp/config \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "port": 21,
    "passive_ports": "50000-51000",
    "max_connections": 100,
    "anonymous": false
  }'
```

### 获取 FTP 状态

```bash
curl http://localhost:8080/api/v1/ftp/status \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "running": true,
    "active_connections": 5,
    "total_transfers": 1234,
    "bytes_transferred": 10737418240
  }
}
```

### 重启 FTP 服务

```bash
curl -X POST http://localhost:8080/api/v1/ftp/restart \
  -H "Authorization: Bearer TOKEN"
```

---

## SFTP 服务器 🆕

### 获取 SFTP 配置

```bash
curl http://localhost:8080/api/v1/sftp/config \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "enabled": true,
    "port": 22,
    "host_key_path": "/etc/nas-os/ssh_host_key",
    "max_connections": 50,
    "auth_methods": ["password", "publickey"],
    "chroot_enabled": true
  }
}
```

### 更新 SFTP 配置

```bash
curl -X PUT http://localhost:8080/api/v1/sftp/config \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "port": 2222,
    "max_connections": 100,
    "auth_methods": ["publickey"]
  }'
```

### 获取 SFTP 状态

```bash
curl http://localhost:8080/api/v1/sftp/status \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "running": true,
    "active_connections": 3,
    "total_transfers": 567,
    "bytes_transferred": 5368709120
  }
}
```

### 重启 SFTP 服务

```bash
curl -X POST http://localhost:8080/api/v1/sftp/restart \
  -H "Authorization: Bearer TOKEN"
```

---

## 文件标签 🆕

### 获取标签列表

```bash
curl http://localhost:8080/api/v1/tags \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "tag-001",
      "name": "重要",
      "color": "#EF4444",
      "icon": "star",
      "file_count": 45,
      "created_at": "2026-03-20T10:00:00Z"
    },
    {
      "id": "tag-002",
      "name": "工作",
      "color": "#3B82F6",
      "icon": "briefcase",
      "file_count": 128,
      "created_at": "2026-03-20T10:00:00Z"
    }
  ]
}
```

### 创建标签

```bash
curl -X POST http://localhost:8080/api/v1/tags \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "个人",
    "color": "#10B981",
    "icon": "user"
  }'
```

### 更新标签

```bash
curl -X PUT http://localhost:8080/api/v1/tags/tag-001 \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "非常重要",
    "color": "#DC2626"
  }'
```

### 删除标签

```bash
curl -X DELETE http://localhost:8080/api/v1/tags/tag-001 \
  -H "Authorization: Bearer TOKEN"
```

### 为文件添加标签

```bash
curl -X POST http://localhost:8080/api/v1/tags/tag-001/files \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "paths": ["/data/documents/report.pdf", "/data/important/contract.docx"]
  }'
```

### 移除文件标签

```bash
curl -X DELETE http://localhost:8080/api/v1/tags/tag-001/files \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "paths": ["/data/documents/report.pdf"]
  }'
```

### 按标签搜索文件

```bash
curl "http://localhost:8080/api/v1/tags/tag-001/files" \
  -H "Authorization: Bearer TOKEN"
```

### 批量添加标签

```bash
curl -X POST http://localhost:8080/api/v1/tags/batch \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "paths": ["/data/photos/1.jpg", "/data/photos/2.jpg"],
    "tags": ["tag-001", "tag-002"]
  }'
```

### 获取标签云

```bash
curl http://localhost:8080/api/v1/tags/cloud \
  -H "Authorization: Bearer TOKEN"
```

---

## 快照复制 🆕 v2.5.0

### 获取复制任务列表

```bash
curl http://localhost:8080/api/v1/snapshot-replication \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "repl-001",
      "name": "数据备份复制",
      "source_volume": "data",
      "target_node": "node-2",
      "target_path": "/backup/data",
      "mode": "incremental",
      "schedule": "0 */6 * * *",
      "enabled": true,
      "last_run": "2026-03-14T06:00:00Z",
      "next_run": "2026-03-14T12:00:00Z",
      "status": "completed",
      "bytes_transferred": 1073741824
    }
  ]
}
```

### 创建复制任务

```bash
curl -X POST http://localhost:8080/api/v1/snapshot-replication \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "数据备份复制",
    "source_volume": "data",
    "source_snapshot": "latest",
    "target_node": "node-2",
    "target_path": "/backup/data",
    "mode": "incremental",
    "schedule": "0 */6 * * *",
    "bandwidth_limit": 10485760,
    "enabled": true
  }'
```

### 手动触发复制

```bash
curl -X POST http://localhost:8080/api/v1/snapshot-replication/repl-001/run \
  -H "Authorization: Bearer TOKEN"
```

### 获取复制状态

```bash
curl http://localhost:8080/api/v1/snapshot-replication/repl-001/status \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "id": "repl-001",
    "status": "running",
    "progress": 45.5,
    "bytes_transferred": 536870912,
    "bytes_total": 1073741824,
    "speed": 10485760,
    "eta": "00:05:30",
    "started_at": "2026-03-14T12:00:00Z"
  }
}
```

---

## 磁盘加密管理 🆕 v2.5.0

### 获取加密状态

```bash
curl http://localhost:8080/api/v1/encryption \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "enabled": true,
    "algorithm": "aes-xts-plain64",
    "key_size": 512,
    "encrypted_devices": [
      {
        "device": "/dev/sdb1",
        "name": "data_crypt",
        "active": true,
        "size": 4000787030016
      }
    ]
  }
}
```

### 启用磁盘加密

```bash
curl -X POST http://localhost:8080/api/v1/encryption/enable \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "device": "/dev/sdb1",
    "passphrase": "your-secure-passphrase",
    "keyfile": "/etc/nas-os/encryption/keyfile"
  }'
```

### 密钥轮换

```bash
curl -X POST http://localhost:8080/api/v1/encryption/rotate-key \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "device": "/dev/sdb1",
    "old_passphrase": "old-passphrase",
    "new_passphrase": "new-passphrase"
  }'
```

### 解锁加密设备

```bash
curl -X POST http://localhost:8080/api/v1/encryption/unlock \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "device": "/dev/sdb1",
    "passphrase": "your-passphrase"
  }'
```

### 锁定加密设备

```bash
curl -X POST http://localhost:8080/api/v1/encryption/lock \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "device": "/dev/sdb1"
  }'
```

---

## 高可用管理 🆕 v2.5.0

### 获取集群状态

```bash
curl http://localhost:8080/api/v1/ha/cluster \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "cluster_name": "nas-cluster",
    "status": "healthy",
    "nodes": [
      {
        "id": "node-1",
        "hostname": "nas-primary",
        "role": "primary",
        "status": "online",
        "vip": "192.168.1.99"
      },
      {
        "id": "node-2",
        "hostname": "nas-secondary",
        "role": "secondary",
        "status": "online",
        "vip": null
      }
    ],
    "failover_enabled": true
  }
}
```

### 手动故障转移

```bash
curl -X POST http://localhost:8080/api/v1/ha/failover \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "target_node": "node-2",
    "force": false
  }'
```

### 获取故障历史

```bash
curl http://localhost:8080/api/v1/ha/failover-history \
  -H "Authorization: Bearer TOKEN"
```

---

## 备份增强 🆕 v2.5.0

### 创建增量备份

```bash
curl -X POST http://localhost:8080/api/v1/backup/incremental \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "daily-incremental",
    "source_path": "/data",
    "destination": "/backup/data",
    "base_snapshot": "snap-20260313-001",
    "schedule": "0 2 * * *"
  }'
```

### 验证备份完整性

```bash
curl -X POST http://localhost:8080/api/v1/backup/backup-001/verify \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "backup_id": "backup-001",
    "status": "valid",
    "verified_at": "2026-03-14T10:00:00Z",
    "checksums_matched": 1256,
    "checksums_failed": 0,
    "total_files": 1256
  }
}
```

### 启用备份加密

```bash
curl -X PUT http://localhost:8080/api/v1/backup/backup-001/encryption \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "algorithm": "aes-256-gcm"
  }'
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

**最后更新**: 2026-03-15

---

## 请求日志 🆕 v2.35.0

### 获取请求日志配置

```bash
curl http://localhost:8080/api/v1/logs/config \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "enabled": true,
    "level": "info",
    "format": "json",
    "output": "stdout",
    "max_size": 100,
    "max_backups": 5,
    "max_age": 30,
    "compress": true
  }
}
```

### 更新请求日志配置

```bash
curl -X PUT http://localhost:8080/api/v1/logs/config \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "level": "debug",
    "format": "json",
    "output": "/var/log/nas-os/requests.log"
  }'
```

### 获取请求日志列表

```bash
curl "http://localhost:8080/api/v1/logs?limit=100&offset=0&level=error" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "total": 1523,
    "logs": [
      {
        "id": "log-001",
        "timestamp": "2026-03-15T04:30:00Z",
        "level": "error",
        "method": "POST",
        "path": "/api/v1/volumes",
        "status": 500,
        "duration": "125ms",
        "client_ip": "192.168.1.100",
        "user_id": "user-001",
        "request_id": "req-abc123",
        "message": "Failed to create volume"
      }
    ]
  }
}
```

### 按请求 ID 查询日志

```bash
curl "http://localhost:8080/api/v1/logs/req-abc123" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "request_id": "req-abc123",
    "trace": [
      {
        "timestamp": "2026-03-15T04:30:00.001Z",
        "level": "info",
        "message": "Request received",
        "method": "POST",
        "path": "/api/v1/volumes"
      },
      {
        "timestamp": "2026-03-15T04:30:00.050Z",
        "level": "debug",
        "message": "Authenticating user"
      },
      {
        "timestamp": "2026-03-15T04:30:00.100Z",
        "level": "error",
        "message": "Failed to create volume: device not found"
      }
    ]
  }
}
```

### 获取请求统计

```bash
curl "http://localhost:8080/api/v1/logs/stats?period=24h" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "total_requests": 12543,
    "by_method": {
      "GET": 8521,
      "POST": 2567,
      "PUT": 892,
      "DELETE": 563
    },
    "by_status": {
      "2xx": 11234,
      "4xx": 1102,
      "5xx": 207
    },
    "avg_duration": "45ms",
    "slow_requests": 23,
    "error_rate": 1.65
  }
}
```

---

## Excel 导出 🆕 v2.35.0

### 导出存储报告

```bash
curl -X POST "http://localhost:8080/api/v1/reports/export" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "storage",
    "format": "xlsx",
    "options": {
      "include_charts": true,
      "include_details": true
    }
  }'
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "download_url": "/api/v1/reports/download/report-20260315-001.xlsx",
    "expires_at": "2026-03-15T05:00:00Z",
    "size": 245678
  }
}
```

### 导出用户使用报告

```bash
curl -X POST "http://localhost:8080/api/v1/reports/export" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "user_usage",
    "format": "xlsx",
    "date_range": {
      "start": "2026-03-01",
      "end": "2026-03-15"
    }
  }'
```

### 导出监控报告

```bash
curl -X POST "http://localhost:8080/api/v1/reports/export" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "monitoring",
    "format": "xlsx",
    "metrics": ["cpu", "memory", "disk", "network"],
    "period": "7d"
  }'
```

### 获取报告模板列表

```bash
curl "http://localhost:8080/api/v1/reports/templates" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": [
    {
      "id": "tpl-001",
      "name": "月度存储报告",
      "type": "storage",
      "description": "存储使用、趋势分析和预测",
      "created_at": "2026-03-10T10:00:00Z"
    },
    {
      "id": "tpl-002",
      "name": "用户活动报告",
      "type": "user_usage",
      "description": "用户登录、操作和资源使用统计",
      "created_at": "2026-03-10T10:00:00Z"
    }
  ]
}
```

### 创建自定义报告模板

```bash
curl -X POST "http://localhost:8080/api/v1/reports/templates" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "季度存储报告",
    "type": "storage",
    "sections": [
      {"name": "概览", "metrics": ["total_capacity", "used_capacity", "growth_rate"]},
      {"name": "趋势分析", "metrics": ["daily_usage", "weekly_trend"], "chart_type": "line"},
      {"name": "预测", "metrics": ["capacity_forecast"]}
    ]
  }'
```

### 下载报告文件

```bash
curl "http://localhost:8080/api/v1/reports/download/report-20260315-001.xlsx" \
  -H "Authorization: Bearer TOKEN" \
  -o report.xlsx
```

### 获取导出历史

```bash
curl "http://localhost:8080/api/v1/reports/history?limit=20" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "total": 45,
    "reports": [
      {
        "id": "report-001",
        "type": "storage",
        "format": "xlsx",
        "status": "completed",
        "size": 245678,
        "created_at": "2026-03-15T04:00:00Z",
        "download_url": "/api/v1/reports/download/report-001.xlsx"
      }
    ]
  }
}
```

---

## 成本分析 🆕 v2.36.0

### 获取存储成本配置

```bash
curl "http://localhost:8080/api/v1/storage-cost/config" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "cost_per_gb_monthly": 0.5,
    "cost_per_iops_monthly": 0.01,
    "cost_per_bandwidth_monthly": 1.0,
    "electricity_cost_per_kwh": 0.6,
    "device_power_watts": 100,
    "ops_cost_monthly": 500,
    "depreciation_years": 5,
    "hardware_cost": 50000
  }
}
```

### 计算存储成本

```bash
curl -X POST "http://localhost:8080/api/v1/storage-cost/calculate" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "volume_names": ["data", "backup"],
    "period": "monthly"
  }'
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "total_cost": {
      "storage_cost": 150.00,
      "compute_cost": 50.00,
      "network_cost": 30.00,
      "operations_cost": 100.00,
      "electricity_cost": 25.00,
      "depreciation_cost": 200.00,
      "total_monthly_cost": 555.00,
      "cost_per_gb": 0.05,
      "cost_per_user": 18.50
    },
    "volume_costs": [
      {
        "volume_name": "data",
        "capacity_gb": 1000,
        "used_gb": 600,
        "usage_percent": 60.0,
        "cost_breakdown": {}
      }
    ]
  }
}
```

### 生成成本分析报告

```bash
curl -X POST "http://localhost:8080/api/v1/storage-cost/report" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "period": "monthly",
    "include_forecast": true,
    "include_recommendations": true
  }'
```

### 获取成本优化建议

```bash
curl "http://localhost:8080/api/v1/cost-optimization/recommendations" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "recommendations": [
      {
        "type": "underutilized_volume",
        "volume_name": "archive",
        "current_cost": 50.00,
        "potential_savings": 25.00,
        "suggestion": "Consider reducing capacity or moving to cold storage"
      },
      {
        "type": "unused_quota",
        "user": "old_user",
        "quota_gb": 100,
        "suggestion": "Quota allocated but not used"
      }
    ],
    "total_potential_savings": 75.00
  }
}
```

### 容量规划预测

```bash
curl -X POST "http://localhost:8080/api/v1/capacity-planning/predict" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "days": 90,
    "growth_model": "linear"
  }'
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "current_usage_gb": 600,
    "predicted_usage_gb": 750,
    "predicted_usage_percent": 75.0,
    "days_until_full": 180,
    "recommendations": [
      "Consider expanding storage within 90 days"
    ]
  }
}
```

---

## API Gateway 🆕 v2.36.0

### 限流配置

```bash
curl "http://localhost:8080/api/v1/gateway/rate-limit/config" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "requests_per_second": 100,
    "burst": 200,
    "enabled": true
  }
}
```

### 熔断器状态

```bash
curl "http://localhost:8080/api/v1/gateway/circuit-breaker/status" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "state": "closed",
    "failure_count": 0,
    "success_count": 1500,
    "last_failure": null,
    "config": {
      "failure_threshold": 5,
      "reset_timeout": "30s"
    }
  }
}
```

### 重试策略

```bash
curl "http://localhost:8080/api/v1/gateway/retry/config" \
  -H "Authorization: Bearer TOKEN"
```

**响应**:
```json
{
  "code": 0,
  "data": {
    "max_retries": 3,
    "initial_delay": "100ms",
    "max_delay": "1s",
    "multiplier": 2.0,
    "retryable_status_codes": [502, 503, 504]
  }
}
```
