# SMB Multichannel 设计文档

## 对标产品
- **TrueNAS 25.10**: SMB Multichannel多通道传输
- **群晖DSM 7.3**: SMB性能优化

## 功能概述
SMB Multichannel允许客户端通过多个网络连接同时传输数据，显著提升带宽利用率和传输性能。

## 技术架构

### 核心组件
```
┌─────────────────────────────────────────┐
│           SMB Multichannel              │
├─────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  │
│  │Channel 1│  │Channel 2│  │Channel 3│  │
│  │ NIC 1   │  │ NIC 2   │  │ RDMA    │  │
│  └─────────┘  └─────────┘  └─────────┘  │
├─────────────────────────────────────────┤
│         Connection Manager              │
│  - 负载均衡                             │
│  - 故障切换                             │
│  - 会话状态同步                         │
├─────────────────────────────────────────┤
│         SMB Server (nas-os)             │
└─────────────────────────────────────────┘
```

### 实现要点
1. **多网卡绑定**: 支持多物理网卡绑定
2. **RDMA支持**: 可选的高性能传输模式
3. **负载均衡**: Round-Robin或基于带宽分配
4. **故障切换**: 单通道故障自动切换
5. **会话同步**: 保持客户端会话一致性

## API设计

### 创建多通道配置
```go
// POST /api/v1/smb/multichannel
type MultichannelConfig struct {
    ShareName     string   `json:"share_name"`
    Interfaces    []string `json:"interfaces"`
    EnableRDMA    bool     `json:"enable_rdma"`
    LoadBalance   string   `json:"load_balance"` // round-robin, bandwidth
    Failover      bool     `json:"failover"`
}
```

### 获取通道状态
```go
// GET /api/v1/smb/multichannel/:share_name/status
type ChannelStatus struct {
    ShareName     string        `json:"share_name"`
    Channels      []ChannelInfo `json:"channels"`
    TotalBandwidth int64        `json:"total_bandwidth"`
    ActiveSession  int          `json:"active_sessions"`
}
```

## 性能预期

| 配置 | 单通道 | 多通道(2 NIC) | 多通道+RDMA |
|------|--------|---------------|-------------|
| 理论带宽 | 1 Gbps | 2 Gbps | 10+ Gbps |
| 实测提升 | - | +80% | +300% |
| 延迟降低 | - | -20% | -50% |

## 实现路线图

| 阶段 | 内容 | 预计完成 |
|------|------|----------|
| Phase 1 | 多网卡绑定基础实现 | v2.470.0 |
| Phase 2 | 负载均衡策略 | v2.471.0 |
| Phase 3 | RDMA支持 | v2.472.0 |
| Phase 4 | WebUI集成 | v2.473.0 |

## 与竞品对比

| 功能 | TrueNAS 25.10 | nas-os | 差异 |
|------|---------------|--------|------|
| 多网卡绑定 | ✅ | 📋 | 相同 |
| RDMA | ✅ | 📋 | 相同 |
| WebUI | ✅ 完善 | 📋 待开发 | 需跟进 |
| 会话状态 | ✅ Stateful | 📋 待开发 | 需跟进 |

## 安全考虑
- 通道隔离：防止跨通道数据泄露
- 访问控制：基于RBAC的通道权限
- 审计日志：记录通道使用情况

---

**设计者**: 兵部
**审核**: 刑部
**状态**: 📋 待开发
**优先级**: P0