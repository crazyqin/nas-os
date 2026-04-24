# NVMe over Fabric 架构设计

**版本**: v1.0
**日期**: 2026-04-24
**对标**: TrueNAS 25.10 Goldeye NVMe-oF
**负责**: 兵部

---

## 1. 概述

NVMe over Fabric (NVMe-oF) 是 TrueNAS 25.10 的核心特性，支持 NVMe/TCP 和 NVMe/RDMA 两种传输模式，实现高性能存储网络。nas-os 需对标此功能，提供类似的高性能存储访问能力。

### 1.1 技术对比

| 特性 | NVMe/TCP | NVMe/RDMA |
|------|----------|-----------|
| 网络要求 | 标准TCP/IP网络 | RDMA专用网络（InfiniBand/RoCE） |
| 硬件成本 | 低（普通网卡） | 高（RDMA网卡） |
| 延迟 | ~50μs | ~10μs |
| 吞吐量 | 高 | 极高 |
| 适用场景 | 家庭/中小企业 | 企业/数据中心 |

### 1.2 TrueNAS 25.10 实现
- Community Edition: NVMe/TCP 支持
- Enterprise Edition: NVMe/RDMA 支持
- 与现有 ZFS/btrfs 存储体系无缝集成

---

## 2. nas-os 架构设计

### 2.1 目标
- 支持 NVMe/TCP 作为主要传输模式（家庭用户友好）
- 提供 NVMe/RDMA 可选支持（企业用户）
- 与现有 btrfs 存储体系集成
- 提供完整的 Web 管理界面

### 2.2 架构图

```
┌─────────────────────────────────────────────────────────┐
│                    NAS-OS NVMe-oF架构                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │
│  │ Web管理界面  │───▶│ NVMe-oF API │───▶│ 存储子系统   │ │
│  └─────────────┘    └─────────────┘    └─────────────┘ │
│                           │                            │
│         ┌─────────────────┼─────────────────┐          │
│         ▼                 ▼                 ▼          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │ NVMe/TCP    │  │ NVMe/RDMA   │  │ 本地NVMe    │    │
│  │ Target      │  │ Target      │  │ 直通        │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 2.3 模块划分

```
internal/storage/nvme/
├── target/          # NVMe Target 服务
│   ├── tcp.go       # NVMe/TCP 实现
│   ├── rdma.go      # NVMe/RDMA 实现（可选）
│   └── subsystem.go # NVMe子系统管理
├── controller/      # NVMe Controller
│   ├── discovery.go # 发现服务
│   └── connect.go   # 连接管理
├── config/          # 配置管理
│   └── config.go    # NVMe-oF配置
└── api/             # REST API
    └── handlers.go  # HTTP处理
```

---

## 3. 功能规格

### 3.1 NVMe/TCP Target（P0）

| 功能 | 说明 | 优先级 |
|------|------|--------|
| Target创建 | 创建NVMe/TCP Target | P0 |
| Subsystem配置 | 配置NVMe子系统 | P0 |
| Namespace映射 | btrfs子卷映射为Namespace | P0 |
| 端口绑定 | 指定监听端口 | P0 |
| ACL控制 | 访问控制列表 | P1 |
| 多路径支持 | Multipath I/O | P2 |

### 3.2 发现服务

| 功能 | 说明 | 优先级 |
|------|------|--------|
| Discovery Controller | 自动发现服务 | P0 |
| 子系统列表 | 返回可用子系统 | P0 |
| 连接信息 | 返回连接参数 | P0 |

### 3.3 管理接口

| API | 说明 | Method |
|-----|------|--------|
| /api/v1/nvme/targets | 获取Target列表 | GET |
| /api/v1/nvme/targets | 创建Target | POST |
| /api/v1/nvme/targets/:id | 更新Target | PUT |
| /api/v1/nvme/targets/:id | 删除Target | DELETE |
| /api/v1/nvme/subsystems | Subsystem管理 | GET/POST |
| /api/v1/nvme/namespaces | Namespace管理 | GET/POST |

---

## 4. 实现路径

### 4.1 Phase 1: NVMe/TCP基础（v2.470.0）
- 实现基础 NVMe/TCP Target
- 与 btrfs 子卷集成
- 基础 Web 界面

### 4.2 Phase 2: 完善功能（v2.480.0）
- Discovery 服务
- ACL访问控制
- 多路径支持

### 4.3 Phase 3: RDMA支持（v2.500.0）
- NVMe/RDMA Target（可选）
- 企业级功能增强
- 性能优化

---

## 5. 技术依赖

### 5.1 系统要求
- Linux kernel 5.0+（NVMe-oF内核模块）
- nvme-cli 工具包
- 支持NVMe的存储设备

### 5.2 Go依赖
- `github.com/linux-nvme/nvme-cli` 命令封装
- 内核sysfs接口访问

### 5.3 与现有系统集成
- btrfs子卷作为Namespace后端
- 用户认证复用RBAC
- 监控集成现有告警系统

---

## 6. 测试计划

### 6.1 功能测试
- Target创建/删除
- Client连接/断开
- I/O读写验证
- 多并发连接

### 6.2 性能测试
- 吞吐量基准
- 延迟测量
- 与本地NVMe对比

### 6.3 兼容性测试
- Linux客户端
- Windows客户端（可选）
- macOS客户端（可选）

---

## 7. 文档规划

- `docs/nvme-of-user-guide.md`: 用户指南
- `docs/nvme-of-admin-guide.md`: 管理员指南
- `docs/nvme-of-troubleshooting.md`: 故障排查

---

## 8. 参考资源

- [NVMe Specification](https://nvmexpress.org/)
- [TrueNAS NVMe-oF Documentation](https://www.truenas.com/docs/scale/scaletutorials/shares/nvme-of/)
- [Linux NVMe Target Wiki](https://docs.kernel.org/admin-guide/nvme.html)