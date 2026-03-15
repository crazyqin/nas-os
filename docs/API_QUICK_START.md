# NAS-OS API 快速入门指南

**版本**: v2.44.0  
**更新日期**: 2026-03-15

---

## 5 分钟上手

### 1. 启动服务

```bash
# 下载并启动
wget https://github.com/crazyqin/nas-os/releases/download/v2.44.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo ./nasd-linux-amd64
```

### 2. 登录获取 Token

```bash
# 默认账号: admin / admin123
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

响应：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_at": "2026-03-16T04:00:00Z"
  }
}
```

### 3. 使用 API

```bash
# 设置环境变量
export TOKEN="eyJhbGciOiJIUzI1NiIs..."

# 获取存储卷列表
curl http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer $TOKEN"
```

---

## 常用 API 示例

### 存储管理

#### 创建存储卷

```bash
curl -X POST http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "data",
    "devices": ["/dev/sda", "/dev/sdb"],
    "raid": "raid1"
  }'
```

#### 创建快照

```bash
curl -X POST http://localhost:8080/api/v1/volumes/data/snapshots \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "daily-2026-03-15"}'
```

### 文件共享

#### 创建 SMB 共享

```bash
curl -X POST http://localhost:8080/api/v1/shares/smb \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "public",
    "path": "/data/public",
    "comment": "Public Share",
    "guest_ok": true,
    "writable": true
  }'
```

#### 创建 NFS 导出

```bash
curl -X POST http://localhost:8080/api/v1/shares/nfs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/data/backup",
    "clients": ["192.168.1.0/24"],
    "options": ["rw", "sync", "no_subtree_check"]
  }'
```

### 用户管理

#### 创建用户

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "zhangsan",
    "password": "SecurePass123!",
    "role": "user",
    "email": "zhangsan@example.com"
  }'
```

#### 分配角色

```bash
curl -POST http://localhost:8080/api/v1/rbac/users/user123/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role": "storage_admin"}'
```

### 容器管理

#### 列出容器

```bash
curl http://localhost:8080/api/v1/docker/containers \
  -H "Authorization: Bearer $TOKEN"
```

#### 启动容器

```bash
curl -X POST http://localhost:8080/api/v1/docker/containers/nginx/start \
  -H "Authorization: Bearer $TOKEN"
```

### 监控

#### 获取系统状态

```bash
curl http://localhost:8080/api/v1/monitor/stats \
  -H "Authorization: Bearer $TOKEN"
```

响应：
```json
{
  "code": 0,
  "data": {
    "cpu_usage": 25.5,
    "memory_usage": 68.2,
    "disk_usage": 45.0,
    "uptime": 86400,
    "load_average": [1.2, 1.5, 1.3]
  }
}
```

#### 获取磁盘健康

```bash
curl http://localhost:8080/api/v1/monitor/disks/sda/smart \
  -H "Authorization: Bearer $TOKEN"
```

---

## CLI 工具

### nasctl 命令行

```bash
# 安装
sudo mv nasctl /usr/local/bin/

# 登录
nasctl login --server http://localhost:8080 --user admin

# 查看卷
nasctl volume list

# 创建快照
nasctl snapshot create data --name daily-$(date +%Y%m%d)

# 创建 SMB 共享
nasctl share create smb public --path /data/public --guest

# 查看系统状态
nasctl status
```

---

## 错误处理

### 响应格式

```json
{
  "code": 1001,
  "message": "用户名或密码错误",
  "data": null
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1001 | 认证失败 |
| 1002 | Token 过期 |
| 1003 | 权限不足 |
| 2001 | 资源不存在 |
| 2002 | 参数错误 |
| 3001 | 存储操作失败 |
| 5000 | 服务器内部错误 |

---

## Swagger 文档

访问交互式 API 文档：

```
http://localhost:8080/swagger/index.html
```

---

## 更多资源

- [完整 API 指南](API_GUIDE.md) - 所有 API 详细文档
- [用户手册](USER_GUIDE.md) - WebUI 使用指南
- [部署指南](DEPLOYMENT_GUIDE_v2.5.0.md) - 生产环境部署
- [故障排除](TROUBLESHOOTING.md) - 常见问题解决

---

## 快速配置示例

### 完整 NAS 初始化脚本

```bash
#!/bin/bash

SERVER="http://localhost:8080"
USER="admin"
PASS="admin123"

# 登录
TOKEN=$(curl -s -X POST $SERVER/api/v1/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" | jq -r '.data.token')

# 修改默认密码
curl -X PUT $SERVER/api/v1/users/admin/password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"old_password":"admin123","new_password":"MySecurePass!"}'

# 创建存储卷
curl -X POST $SERVER/api/v1/volumes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"data","devices":["/dev/sda","/dev/sdb"],"raid":"raid1"}'

# 创建 SMB 共享
curl -X POST $SERVER/api/v1/shares/smb \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"home","path":"/data/home","guest_ok":false,"writable":true}'

# 创建普通用户
curl -X POST $SERVER/api/v1/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"zhangsan","password":"Pass123!","role":"user"}'

echo "NAS 初始化完成！"
```

---

## 下一步

1. 修改默认密码
2. 配置存储卷
3. 创建共享和用户
4. 设置监控告警
5. 配置定时备份

详细配置请参考 [完整 API 指南](API_GUIDE.md)。