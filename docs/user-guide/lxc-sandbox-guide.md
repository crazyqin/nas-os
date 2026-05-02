# LXC 容器沙箱指南

> **版本**: v2.477.0+ | **适用版本**: NAS-OS v2.477.0 及以上

## 概述

LXC 容器沙箱提供轻量级隔离应用运行环境，对标 TrueNAS SCALE 的 LXC Sandboxes 功能。相比 Docker，LXC 容器共享主机内核、启动更快、资源开销更小，适合运行长期服务和系统级应用。

## 核心特性

- **内置模板**：Ubuntu 24.04、Alpine 3.20、Debian 12 一键创建
- **资源隔离**：CPU/内存/磁盘独立限制（cgroup2）
- **网络隔离**：独立网桥 `lxcbr0`，自动 IP 分配（10.0.3.0/24）
- **端口映射**：自定义主机端口 → 容器端口转发
- **卷挂载**：主机目录挂载到容器，支持只读模式
- **重启策略**：always / on-failure / never
- **实时统计**：CPU、内存、磁盘、网络流量监控

## 快速开始

### 创建沙箱

```bash
# 使用 Ubuntu 模板创建
curl -X POST http://localhost:8080/api/v1/lxc/sandboxes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-server",
    "template": "ubuntu-24.04",
    "cpu": 2,
    "memory_mb": 1024,
    "disk_gb": 20,
    "ports": [
      {"host_port": 8080, "container_port": 80, "protocol": "tcp"}
    ],
    "volumes": [
      {"host_path": "/data/shared", "container_path": "/mnt/data", "read_only": false}
    ]
  }'
```

### 管理操作

```bash
# 列出所有沙箱
curl http://localhost:8080/api/v1/lxc/sandboxes

# 启动沙箱
curl -X POST http://localhost:8080/api/v1/lxc/sandboxes/{id}/start

# 停止沙箱
curl -X POST http://localhost:8080/api/v1/lxc/sandboxes/{id}/stop

# 查看统计
curl http://localhost:8080/api/v1/lxc/sandboxes/{id}/stats

# 删除沙箱
curl -X DELETE http://localhost:8080/api/v1/lxc/sandboxes/{id}

# 导出配置
curl http://localhost:8080/api/v1/lxc/sandboxes/{id}/export
```

## 内置模板

| 模板 | 发行版 | 大小 | 说明 |
|------|--------|------|------|
| `ubuntu-24.04` | Ubuntu 24.04 LTS | ~300MB | 通用服务器环境，预装 curl/wvim/git |
| `alpine-3.20` | Alpine Linux 3.20 | ~50MB | 超轻量级，适合容器化服务 |
| `debian-12` | Debian 12 | ~250MB | 稳定服务器环境 |

## 默认配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 最大沙箱数 | 50 | 系统上限 |
| 默认 CPU | 2 核 | cgroup2 cpu.max 限制 |
| 默认内存 | 512 MB | cgroup2 memory.max 限制 |
| 默认磁盘 | 10 GB | rootfs 大小 |
| 网桥 | lxcbr0 | 容器网络桥接 |
| 子网 | 10.0.3.0/24 | 容器 IP 地址段 |
| 存储路径 | /var/lib/nas-os/lxc | 沙箱数据目录 |

## 重启策略

| 策略 | 行为 |
|------|------|
| `always` | 容器退出后始终重启 |
| `on-failure` | 仅在异常退出时重启（默认） |
| `never` | 不自动重启 |

## 安全隔离

- 每个沙箱使用独立的 cgroup2 资源控制
- 网络通过 veth pair + 网桥隔离
- 文件系统通过 rootfs 独立目录隔离
- 端口映射精确控制网络暴露面

## 注意事项

- 宿主机需安装 LXC（`apt install lxc` 或 `dnf install lxc`）
- 沙箱共享主机内核，不支持自定义内核模块
- 建议为生产环境的沙箱挂载独立数据卷
- 删除沙箱将同时删除所有数据，操作前请确认
