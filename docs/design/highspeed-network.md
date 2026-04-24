# 高性能网络设计 - 400GbE 支持

**版本**: v1.0
**日期**: 2026-04-24
**对标**: TrueNAS 25.10 400GbE支持
**负责**: 工部

---

## 1. TrueNAS 25.10 网络增强

TrueNAS 25.10 引入了面向 Terabit Ethernet 的网络增强：
- 400GbE 网卡驱动支持
- 网络配置管理优化
- DHCP/静态配置平滑切换

---

## 2. nas-os 网络架构现状

### 2.1 当前支持
- 千兆/万兆网卡
- 基础网络配置
- DHCP/静态IP
- 多网卡绑定

### 2.2 对标差距
| 功能 | TrueNAS 25.10 | nas-os | 差距 |
|------|---------------|--------|------|
| 400GbE驱动 | ✅ | ❌ | 需评估 |
| RDMA网络 | ✅ Enterprise | ❌ | 需开发 |
| 网络配置优化 | ✅ 改进 | 基础 | 可优化 |
| 多通道管理 | ✅ | 📋规划 | SMB Multichannel |

---

## 3. 高性能网络设计

### 3.1 网络架构

```
┌─────────────────────────────────────────────────────────┐
│                   nas-os 网络架构                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │              Network Manager                     │   │
│  ├─────────────────────────────────────────────────┤   │
│  │ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐│   │
│  │ │接口管理 │ │绑定管理 │ │路由管理 │ │DNS管理  ││   │
│  │ └─────────┘ └─────────┘ └─────────┘ └─────────┘│   │
│  └─────────────────────────────────────────────────┘   │
│         │                                              │
│         ▼                                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │              High-Speed Layer                     │   │
│  ├─────────────────────────────────────────────────┤   │
│  │ ┌─────────┐ ┌─────────┐ ┌─────────┐            │   │
│  │ │400GbE   │ │RDMA     │ │Multichan│            │   │
│  │ │Support  │ │Config   │ │nel      │            │   │
│  │ └─────────┘ └─────────┘ └─────────┘            │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 3.2 400GbE 支持评估

#### 硬件要求
- Mellanox ConnectX-7 系列
- Intel E810 系列
- Broadcom NetXtreme E系列

#### 软件依赖
- Linux kernel 5.15+
- 最新驱动固件
- ethtool 配置工具

#### nas-os 实现路径
1. **Phase 1**: 验证400GbE网卡兼容性
2. **Phase 2**: 添加网卡识别和配置API
3. **Phase 3**: 性能优化和调优参数

---

## 4. 网络配置管理优化

### 4.1 配置平滑切换
对标 TrueNAS 25.10 DHCP/静态配置平滑切换：

```go
// NetworkConfigSwitch 网络配置切换
type NetworkConfigSwitch struct {
    Interface    string
    CurrentMode  NetworkMode // DHCP/Static
    TargetMode   NetworkMode
    BackupConfig NetworkBackup // 配置备份
}

// SmoothSwitch 平滑切换配置
func (n *NetworkManager) SmoothSwitch(config NetworkConfigSwitch) error {
    // 1. 备份当前配置
    // 2. 应用新配置
    // 3. 验证连接
    // 4. 失败时回滚
}
```

### 4.2 网络健康监控

| 监控项 | 说明 | 告警阈值 |
|--------|------|----------|
| 链路状态 | up/down | 状态变化 |
| 带宽利用率 | 实时吞吐 | >80% |
| 丢包率 | 包丢失统计 | >0.1% |
| 延迟 | 网络延迟 | >10ms |

---

## 5. API设计

### 5.1 网络接口API

| API | 说明 | Method |
|-----|------|--------|
| /api/v1/network/interfaces | 接口列表 | GET |
| /api/v1/network/interfaces/:id | 接口详情 | GET |
| /api/v1/network/interfaces/:id/config | 配置更新 | PUT |
| /api/v1/network/interfaces/:id/bond | 绑定配置 | POST |
| /api/v1/network/switch | 配置切换 | POST |

### 5.2 高性能网络API

| API | 说明 | Method |
|-----|------|--------|
| /api/v1/network/highspeed/status | 高速网络状态 | GET |
| /api/v1/network/highspeed/tuning | 调优参数 | GET/PUT |
| /api/v1/network/rdma/config | RDMA配置 | GET/PUT |

---

## 6. 实现路径

### 6.1 Phase 1: 网络配置优化（v2.470.0）
- 配置平滑切换
- 网络健康监控增强
- 多网卡管理优化

### 6.2 Phase 2: 高速网卡支持（v2.480.0）
- 100GbE/400GbE识别
- 高速网卡配置API
- 性能调优参数

### 6.3 Phase 3: RDMA支持（v2.500.0）
- RDMA网络配置
- 与NVMe-oF集成
- 企业级网络功能

---

## 7. 测试计划

### 7.1 功能测试
- 网卡识别和配置
- DHCP/静态切换
- 多网卡绑定

### 7.2 性能测试
- 不同速率网卡吞吐
- RDMA延迟测试
- 配置切换时间

### 7.3 兼容性测试
- Mellanox网卡
- Intel网卡
- Broadcom网卡

---

## 8. 文档规划

- `docs/network-config-guide.md`: 网络配置指南
- `docs/highspeed-network-guide.md`: 高速网络指南
- `docs/network-troubleshooting.md`: 网络故障排查