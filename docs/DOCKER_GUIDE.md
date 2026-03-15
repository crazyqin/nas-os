# NAS-OS Docker 容器管理指南

本文档介绍如何使用 NAS-OS 的 Docker 容器管理功能，包括创建容器、管理镜像、配置网络等。

## 概述

NAS-OS 集成了 Docker 容器管理功能，让您可以轻松地在 NAS 上运行各种应用服务：

- **容器管理**：创建、启动、停止、删除容器
- **镜像管理**：拉取、删除、导出镜像
- **网络管理**：创建自定义网络，配置端口映射
- **存储卷管理**：数据持久化，容器间共享数据
- **资源限制**：CPU、内存限制，防止资源耗尽
- **日志查看**：实时查看容器日志

## 快速开始

### 创建第一个容器

#### 1. 通过 Web 界面

1. 登录 NAS-OS 管理界面
2. 进入 **容器管理** > **容器**
3. 点击 **创建容器** 按钮
4. 填写容器配置：
   - **名称**：容器名称（如 `nginx-web`）
   - **镜像**：Docker 镜像（如 `nginx:latest`）
   - **端口映射**：主机端口:容器端口（如 `8080:80`）
   - **存储卷**：主机目录:容器目录

#### 2. 通过 API

```bash
# 创建 Nginx 容器
curl -X POST http://localhost:8080/api/v1/docker/containers \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "nginx-web",
    "image": "nginx:latest",
    "ports": ["8080:80"],
    "volumes": ["/data/nginx:/usr/share/nginx/html"],
    "restart_policy": "always"
  }'
```

### 启动/停止容器

```bash
# 启动容器
curl -X POST http://localhost:8080/api/v1/docker/containers/{id}/start \
  -H "Authorization: Bearer {token}"

# 停止容器
curl -X POST http://localhost:8080/api/v1/docker/containers/{id}/stop \
  -H "Authorization: Bearer {token}"

# 重启容器
curl -X POST http://localhost:8080/api/v1/docker/containers/{id}/restart \
  -H "Authorization: Bearer {token}"
```

### 查看容器日志

```bash
# 获取最近 100 行日志
curl "http://localhost:8080/api/v1/docker/containers/{id}/logs?tail=100" \
  -H "Authorization: Bearer {token}"
```

## 容器配置详解

### 端口映射

将容器内部端口映射到主机端口：

```json
{
  "ports": [
    "8080:80",       # HTTP
    "8443:443",      # HTTPS
    "2222:22"        # SSH
  ]
}
```

**格式**：`主机端口:容器端口` 或 `主机IP:主机端口:容器端口`

### 存储卷

实现数据持久化和容器间共享：

```json
{
  "volumes": [
    "/data/app:/app/data",           # 数据目录
    "/data/config:/app/config:ro",   # 只读配置
    "app-logs:/var/log/app"          # 命名卷
  ]
}
```

**格式**：`主机路径:容器路径:权限`（权限可选：ro/rw）

### 环境变量

配置容器运行环境：

```json
{
  "environment": {
    "MYSQL_ROOT_PASSWORD": "secret123",
    "MYSQL_DATABASE": "mydb",
    "TZ": "Asia/Shanghai"
  }
}
```

### 资源限制

防止单个容器占用过多资源：

```json
{
  "resources": {
    "cpu_limit": 2,           # 最多使用 2 个 CPU
    "memory_limit": "2g",     # 内存限制 2GB
    "cpu_reservation": 0.5,   # 预留 0.5 个 CPU
    "memory_reservation": "1g" # 预留 1GB 内存
  }
}
```

### 重启策略

配置容器异常退出后的行为：

| 策略 | 说明 |
|------|------|
| `no` | 不自动重启 |
| `always` | 总是重启 |
| `on-failure` | 仅失败时重启 |
| `unless-stopped` | 除非手动停止，否则重启 |

## 常用容器示例

### Nginx Web 服务器

```json
{
  "name": "nginx",
  "image": "nginx:alpine",
  "ports": ["80:80", "443:443"],
  "volumes": [
    "/data/nginx/html:/usr/share/nginx/html",
    "/data/nginx/conf:/etc/nginx/conf.d"
  ],
  "restart_policy": "always"
}
```

### MySQL 数据库

```json
{
  "name": "mysql",
  "image": "mysql:8.0",
  "ports": ["3306:3306"],
  "volumes": ["/data/mysql:/var/lib/mysql"],
  "environment": {
    "MYSQL_ROOT_PASSWORD": "your-password",
    "MYSQL_DATABASE": "appdb"
  },
  "restart_policy": "always"
}
```

### Redis 缓存

```json
{
  "name": "redis",
  "image": "redis:alpine",
  "ports": ["6379:6379"],
  "volumes": ["/data/redis:/data"],
  "restart_policy": "always"
}
```

### Nextcloud 私有云

```json
{
  "name": "nextcloud",
  "image": "nextcloud:latest",
  "ports": ["8080:80"],
  "volumes": [
    "/data/nextcloud:/var/www/html",
    "/data/files:/var/www/html/data"
  ],
  "environment": {
    "MYSQL_HOST": "mysql",
    "MYSQL_DATABASE": "nextcloud",
    "MYSQL_USER": "nextcloud",
    "MYSQL_PASSWORD": "your-password"
  },
  "restart_policy": "always"
}
```

## 镜像管理

### 拉取镜像

```bash
# 拉取镜像（通过 API）
# POST /api/v1/docker/images/pull
curl -X POST http://localhost:8080/api/v1/docker/images/pull \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{"image": "nginx:latest"}'
```

### 列出镜像

```bash
curl http://localhost:8080/api/v1/docker/images \
  -H "Authorization: Bearer {token}"
```

### 删除镜像

```bash
curl -X DELETE http://localhost:8080/api/v1/docker/images/{id} \
  -H "Authorization: Bearer {token}"
```

## 网络管理

### 创建自定义网络

```bash
# 创建桥接网络
curl -X POST http://localhost:8080/api/v1/docker/networks \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "app-network",
    "driver": "bridge"
  }'
```

### 容器连接网络

```json
{
  "name": "my-app",
  "image": "my-app:latest",
  "networks": ["app-network"]
}
```

## 最佳实践

### 数据持久化

- 重要数据务必挂载存储卷
- 使用命名卷便于管理
- 定期备份重要数据

### 资源管理

- 为每个容器设置合理的资源限制
- 监控容器资源使用情况
- 避免资源争抢导致服务不稳定

### 安全建议

1. **最小权限原则**：容器不要以 root 运行
2. **网络隔离**：敏感服务使用内部网络
3. **镜像来源**：使用官方镜像或可信来源
4. **定期更新**：及时更新镜像修复安全漏洞

### 日志管理

- 设置日志驱动和大小限制
- 集中收集容器日志
- 定期清理旧日志

```json
{
  "log_config": {
    "type": "json-file",
    "config": {
      "max-size": "10m",
      "max-file": "3"
    }
  }
}
```

## 故障排查

### 容器无法启动

**排查步骤**：
1. 查看容器日志
2. 检查镜像是否存在
3. 验证配置是否正确
4. 检查端口是否被占用

### 容器频繁重启

**可能原因**：
- 应用程序崩溃
- 资源不足
- 配置错误

**解决方案**：
- 查看应用日志定位问题
- 增加资源限制
- 检查配置文件

### 网络连接问题

**排查步骤**：
1. 检查网络配置
2. 验证端口映射
3. 测试网络连通性
4. 查看防火墙规则

## 相关文档

- [API 参考 - Docker 管理](api.yaml#docker)
- [网络配置指南](NETWORK_GUIDE.md)
- [安全最佳实践](security/BEST_PRACTICES.md)

---

**版本**：v2.89.0  
**更新日期**：2026-03-16