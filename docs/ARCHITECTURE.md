# NAS-OS 架构概览

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                      用户层 (User Layer)                      │
├─────────────────────────────────────────────────────────────┤
│  Web UI (Vue.js)  │  nasctl CLI  │  REST API  │  WebSocket  │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│                    API 网关 (API Gateway)                     │
├─────────────────────────────────────────────────────────────┤
│  JWT Auth  │  RBAC  │  Rate Limit  │  Request Router        │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│                  业务层 (Business Layer)                      │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐       │
│  │ Storage │  │   AI    │  │ Network │  │ Docker  │       │
│  │ Manager │  │ Engine  │  │ Service │  │ Manager │       │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘       │
│       │            │            │            │              │
│  ┌────▼────┐  ┌────▼────┐  ┌────▼────┐  ┌────▼────┐       │
│  │ btrfs   │  │ Ollama  │  │  SMB    │  │ Docker  │       │
│  │ Pool    │  │ CLIP    │  │  NFS    │  │ Engine  │       │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘       │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│                  基础设施层 (Infrastructure)                   │
├─────────────────────────────────────────────────────────────┤
│  Linux Kernel  │  btrfs  │  systemd  │  cgroups             │
└─────────────────────────────────────────────────────────────┘
```

## 模块说明

### 核心模块 (`internal/`)

| 模块 | 路径 | 职责 |
|------|------|------|
| Storage | `internal/storage/` | btrfs 池/卷/快照管理 |
| Network | `internal/network/` | SMB/NFS 共享管理 |
| User | `internal/user/` | 用户/组/RBAC 权限 |
| Monitor | `internal/monitor/` | 系统监控/告警 |
| Docker | `internal/docker/` | 容器生命周期管理 |
| AI | `internal/ai/` | Ollama/CLIP 本地推理 |

### 独家功能模块

| 模块 | 路径 | 功能 |
|------|------|------|
| immutastore | `internal/immutastore/` | WriteOnce 不可变存储 |
| aiphototimeline | `internal/aiphototimeline/` | AI 时间线相册 |
| ransomguard | `internal/ransomguard/` | 勒索防护 |
| smarttierengine | `internal/smarttierengine/` | 智能分层引擎 |
| capacityai | `internal/capacityai/` | AI 容量规划 |

### 入口程序

| 入口 | 路径 | 用途 |
|------|------|------|
| nasd | `cmd/nasd/` | 守护进程，系统主服务 |
| nasctl | `cmd/nasctl/` | CLI 客户端 |
| backup | `cmd/backup/` | 备份工具 |

## 数据流

### 文件读写

```
Client → SMB/NFS → Storage Manager → btrfs → Disk
```

### AI 推理

```
Client → API → AI Engine → Ollama/CLIP → Response
```

### 监控告警

```
System Metrics → Monitor → Alert Rules → Notification (Email/Webhook/Telegram)
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 语言 | Go 1.26+ |
| 前端 | Vue.js 3 + Vite |
| 数据库 | SQLite / bbolt |
| 容器 | Docker / containerd |
| AI | Ollama + CLIP |
| 存储 | btrfs |
| 网络 | SMB (samba) / NFS |

## 部署架构

### 单节点（家用）

```
┌──────────────────────────────┐
│        NAS-OS Node           │
│  ┌────────────────────────┐  │
│  │  nasd (主服务)         │  │
│  │  ┌────────┐ ┌────────┐│  │
│  │  │ Web UI │ │ Docker ││  │
│  │  └────────┘ └────────┘│  │
│  └────────────────────────┘  │
│  ┌────────────────────────┐  │
│  │  Storage Pool (btrfs)  │  │
│  │  /dev/sda  /dev/sdb    │  │
│  └────────────────────────┘  │
└──────────────────────────────┘
```

### 集群（企业）

```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│  Node 1     │  │  Node 2     │  │  Node 3     │
│  nasd       │◄─►  nasd       │◄─►  nasd       │
│  (Leader)   │  │  (Follower) │  │  (Follower) │
└─────────────┘  └─────────────┘  └─────────────┘
       │                │                │
       └────────────────┼────────────────┘
                        │
              ┌─────────▼─────────┐
              │  Shared Storage   │
              │  (Ceph / NFS)    │
              └───────────────────┘
```

---

*版本: v2.582.0 | 最后更新: 2026-06-09*
