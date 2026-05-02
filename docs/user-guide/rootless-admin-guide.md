# 无 Root 管理员指南

> **版本**: v2.477.0+ | **适用版本**: NAS-OS v2.477.0 及以上

## 概述

无 Root 管理员（Rootless Admin）功能允许管理员在不使用 root 权限的情况下执行系统管理操作，对标 TrueNAS SCALE 25.10 的 Rootless Admin 功能。通过命令白名单和细粒度权限控制，在安全性和便利性之间取得平衡。

## 核心特性

- **命令白名单**：仅允许执行预定义的安全命令
- **细粒度权限**：按资源类型（存储/网络/Docker 等）分配操作权限
- **审计日志**：所有操作完整记录，含命令、用户、时间、结果
- **会话管理**：最大并发会话数限制，超时自动清理
- **路径保护**：禁止访问敏感路径（/etc/shadow、/root/.ssh 等）

## 默认命令白名单

| 命令 | 路径 | 用途 |
|------|------|------|
| `systemctl` | /usr/bin/systemctl | 服务管理 |
| `docker` | /usr/bin/docker | 容器管理 |
| `btrfs` | /usr/sbin/btrfs | 存储管理 |
| `smbcontrol` | /usr/sbin/smbcontrol | SMB 服务控制 |
| `journalctl` | /usr/bin/journalctl | 日志查看 |
| `top` | /usr/bin/top | 系统监控 |
| `htop` | /usr/bin/htop | 系统监控 |

## API 接口

### 注册管理员

```bash
curl -X POST http://localhost:8080/api/v1/rootless/admins \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin1",
    "privileges": [
      {"resource": "storage", "actions": ["read", "write"]},
      {"resource": "docker", "actions": ["read"]},
      {"resource": "network", "actions": ["read"]}
    ]
  }'
```

### 执行管理命令

```bash
# 以管理员身份执行命令
curl -X POST http://localhost:8080/api/v1/rootless/execute \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin1",
    "command": "/usr/bin/systemctl",
    "args": ["status", "nas-os"]
  }'
```

### 检查权限

```bash
curl "http://localhost:8080/api/v1/rootless/check?username=admin1&resource=storage&action=write"
```

### 查看审计日志

```bash
# 获取最近 100 条审计记录
curl "http://localhost:8080/api/v1/rootless/audit?limit=100"
```

### 撤销管理员

```bash
curl -X DELETE http://localhost:8080/api/v1/rootless/admins/admin1
```

## 权限资源类型

| 资源 | 说明 | 可用操作 |
|------|------|----------|
| `storage` | 存储管理 | read, write, delete, admin |
| `network` | 网络配置 | read, write, admin |
| `docker` | 容器管理 | read, write, delete, admin |
| `system` | 系统服务 | read, write, admin |
| `backup` | 备份管理 | read, write, admin |

## 安全机制

### 路径保护

以下路径默认禁止访问：
- `/etc/shadow` - 密码哈希
- `/etc/gshadow` - 组密码
- `/root/.ssh` - root SSH 密钥

### 会话限制

- 最大并发会话：3 个/管理员
- 最大会话时长：480 分钟（8 小时）
- 超时自动清理空闲会话

### 审计记录字段

| 字段 | 说明 |
|------|------|
| timestamp | 操作时间 |
| username | 操作用户 |
| action | 操作类型 |
| resource | 资源类型 |
| command | 执行的命令 |
| success | 是否成功 |
| error_msg | 错误信息 |
| ip_address | 来源 IP |
| session_id | 会话 ID |

## 工作原理

1. 系统创建 `nas-admins` 用户组
2. 注册的管理员自动加入该组
3. 通过 sudoers 规则授权该组执行白名单命令
4. 所有命令通过 `sudo -g nas-admins <cmd>` 执行
5. 每次操作前检查命令白名单和权限矩阵
6. 操作结果记录到审计日志

## 注意事项

- 管理员需先在系统中创建用户账户
- 撤销管理员仅禁用权限，不删除系统用户
- 自定义命令白名单需重启服务生效
- 审计日志默认保留 10000 条记录
