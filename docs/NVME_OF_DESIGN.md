# TrueNAS NVMe-oF 参考设计文档

**版本**: v1.0 | **日期**: 2026-03-31 | **负责**: 兵部

---

## 一、概述

### 1.1 什么是 NVMe-oF

NVMe over Fabrics (NVMe-oF) 是一种存储网络协议，允许 NVMe 存储设备通过网络进行访问，提供接近本地 NVMe 的性能。

**核心优势**:
- 低延迟：绕过传统 SCSI 协议栈
- 高吞吐：充分利用 NVMe SSD 性能
- 远程访问：支持存储资源池化
- 多协议支持：TCP、RDMA (RoCE/iWARP)、FC

### 1.2 TrueNAS NVMe-oF 实现

TrueNAS Scale/Enterprise 提供 NVMe-oF Target 和 Initiator 功能：
- **Target 模式**：将本地 NVMe SSD 共享给远程主机
- **Initiator 模式**：连接远程 NVMe-oF Target

---

## 二、架构设计

### 2.1 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      NAS-OS 系统                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │   Web UI     │    │   REST API   │    │   CLI/SDK    │  │
│  │  (管理界面)   │    │   (外部接口)  │    │  (命令行)    │  │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘  │
│         │                   │                   │          │
│         └───────────────────┼───────────────────┘          │
│                             │                              │
│                    ┌────────▼────────┐                     │
│                    │   NVMe-oF 管理   │                     │
│                    │    服务层        │                     │
│                    └────────┬────────┘                     │
│                             │                              │
│         ┌───────────────────┼───────────────────┐         │
│         │                   │                   │         │
│  ┌──────▼──────┐    ┌──────▼──────┐    ┌──────▼──────┐   │
│  │   Target    │    │  Initiator  │    │  Discovery  │   │
│  │   模块      │    │    模块     │    │    模块     │   │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘   │
│         │                   │                   │         │
│         └───────────────────┼───────────────────┘         │
│                             │                              │
│                    ┌────────▼────────┐                     │
│                    │   内核层接口     │                     │
│                    │  (nvmet/nvmf)   │                     │
│                    └────────┬────────┘                     │
│                             │                              │
├─────────────────────────────┼──────────────────────────────┤
│                     Linux Kernel                           │
│                    ┌────────▼────────┐                     │
│                    │   NVMe 子系统     │                     │
│                    │  (nvme-core)    │                     │
│                    └────────┬────────┘                     │
│                             │                              │
│         ┌───────────────────┼───────────────────┐         │
│         │                   │                   │         │
│  ┌──────▼──────┐    ┌──────▼──────┐    ┌──────▼──────┐   │
│  │  NVMe/TCP   │    │  NVMe/RDMA  │    │   NVMe/FC   │   │
│  │   传输层    │    │    传输层    │    │   传输层    │   │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘   │
│         │                   │                   │         │
│         └───────────────────┼───────────────────┘         │
│                             │                              │
│                    ┌────────▼────────┐                     │
│                    │   物理 NVMe     │                     │
│                    │   SSD 设备      │                     │
│                    └─────────────────┘                     │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 核心组件

| 组件 | 说明 | 实现方式 |
|------|------|----------|
| nvmet | NVMe Target 内核模块 | Linux 内核配置 |
| nvme-fabrics | NVMe-oF 传输层 | Linux 内核配置 |
| nvme-cli | NVMe 命令行工具 | 用户空间工具 |
| nvmetcli | NVMe Target 配置工具 | Python 工具 |

---

## 三、Target 配置设计

### 3.1 内核模块配置

```bash
# 加载必要内核模块
modprobe nvmet
modprobe nvmet-tcp
modprobe nvmet-rdma  # 如果支持 RDMA
modprobe nvmet-fc    # 如果支持 FC

# 开机自动加载
cat > /etc/modules-load.d/nvmet.conf << EOF
nvmet
nvmet-tcp
nvmet-rdma
nvme-fabrics
EOF
```

### 3.2 Target 配置结构

```go
// internal/nvmeof/target/config.go

package target

// TargetConfig NVMe-oF Target 配置
type TargetConfig struct {
    // NQN (NVMe Qualified Name) 全局唯一标识
    NQN string `json:"nqn" yaml:"nqn"`
    
    // 允许的主机列表
    Hosts []HostConfig `json:"hosts" yaml:"hosts"`
    
    // 子系统配置
    Subsystems []SubsystemConfig `json:"subsystems" yaml:"subsystems"`
    
    // 传输层配置
    Transports []TransportConfig `json:"transports" yaml:"transports"`
}

// HostConfig 主机访问控制配置
type HostConfig struct {
    NQN    string `json:"nqn" yaml:"nqn"`       // 主机 NQN
    Access string `json:"access" yaml:"access"` // ro/rw
}

// SubsystemConfig 子系统配置
type SubsystemConfig struct {
    NQN        string        `json:"nqn" yaml:"nqn"`
    Namespaces []NSConfig    `json:"namespaces" yaml:"namespaces"`
    AllowAny   bool          `json:"allow_any" yaml:"allow_any"`
}

// NSConfig Namespace 配置
type NSConfig struct {
    ID      int    `json:"id" yaml:"id"`           // Namespace ID (1-based)
    Device  string `json:"device" yaml:"device"`   // 后端设备路径
    ANAGRP  int    `json:"ana_grp" yaml:"ana_grp"` // ANA 组 ID
}

// TransportConfig 传输层配置
type TransportConfig struct {
    Type     string `json:"type" yaml:"type"`       // tcp/rdma/fc
    Address  string `json:"address" yaml:"address"`  // 监听地址
    Port     int    `json:"port" yaml:"port"`       // 端口号
    iface    string `json:"iface" yaml:"iface"`      // 网络接口
}
```

### 3.3 TCP Target 示例配置

```json
{
  "nqn": "nqn.2026-03.org.nas-os:target1",
  "subsystems": [
    {
      "nqn": "nqn.2026-03.org.nas-os:subsystem1",
      "allow_any": false,
      "namespaces": [
        {
          "id": 1,
          "device": "/dev/nvme0n1"
        },
        {
          "id": 2,
          "device": "/dev/nvme1n1"
        }
      ]
    }
  ],
  "transports": [
    {
      "type": "tcp",
      "address": "0.0.0.0",
      "port": 4420,
      "iface": "eth0"
    }
  ],
  "hosts": [
    {
      "nqn": "nqn.2026-03.org.nas-os:host1",
      "access": "rw"
    }
  ]
}
```

---

## 四、Initiator 配置设计

### 4.1 连接配置

```go
// internal/nvmeof/initiator/config.go

package initiator

// InitiatorConfig NVMe-oF Initiator 配置
type InitiatorConfig struct {
    // 本地 NQN
    NQN string `json:"nqn" yaml:"nqn"`
    
    // 连接目标
    Targets []TargetConnection `json:"targets" yaml:"targets"`
}

// TargetConnection 目标连接配置
type TargetConnection struct {
    Name       string `json:"name" yaml:"name"`           // 连接名称
    NQN        string `json:"nqn" yaml:"nqn"`             // 目标子系统 NQN
    Transport  string `json:"transport" yaml:"transport"` // tcp/rdma/fc
    Address    string `json:"address" yaml:"address"`     // 目标地址
    Port       int    `json:"port" yaml:"port"`           // 端口号
    
    // 重连配置
    ReconnectDelay int `json:"reconnect_delay" yaml:"reconnect_delay"` // 秒
    MaxReconnect   int `json:"max_reconnect" yaml:"max_reconnect"`     // 最大次数
    
    // 性能调优
    QueueDepth    int  `json:"queue_depth" yaml:"queue_depth"`
    NrIOQueues    int  `json:"nr_io_queues" yaml:"nr_io_queues"`
    KeepAliveTime int  `json:"keep_alive_time" yaml:"keep_alive_time"` // 秒
}
```

### 4.2 连接管理

```go
// internal/nvmeof/initiator/connect.go

package initiator

import (
    "context"
    "fmt"
    "os/exec"
    "time"
)

// Manager Initiator 管理器
type Manager struct {
    config *InitiatorConfig
}

// Connect 连接到 NVMe-oF Target
func (m *Manager) Connect(ctx context.Context, target *TargetConnection) error {
    args := []string{
        "connect",
        "-t", target.Transport,
        "-n", target.NQN,
        "-a", target.Address,
        "-s", fmt.Sprintf("%d", target.Port),
    }
    
    if target.QueueDepth > 0 {
        args = append(args, "-q", fmt.Sprintf("%d", target.QueueDepth))
    }
    if target.NrIOQueues > 0 {
        args = append(args, "-i", fmt.Sprintf("%d", target.NrIOQueues))
    }
    if target.KeepAliveTime > 0 {
        args = append(args, "-k", fmt.Sprintf("%d", target.KeepAliveTime))
    }
    
    cmd := exec.CommandContext(ctx, "nvme", args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("nvme connect failed: %w, output: %s", err, output)
    }
    
    return nil
}

// Disconnect 断开连接
func (m *Manager) Disconnect(ctx context.Context, nqn string) error {
    cmd := exec.CommandContext(ctx, "nvme", "disconnect", "-n", nqn)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("nvme disconnect failed: %w, output: %s", err, output)
    }
    return nil
}

// List 列出已连接的子系统
func (m *Manager) List(ctx context.Context) ([]SubsysInfo, error) {
    // nvme list-subsys
    cmd := exec.CommandContext(ctx, "nvme", "list-subsys")
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("nvme list-subsys failed: %w", err)
    }
    
    return parseSubsysList(output), nil
}
```

---

## 五、Discovery 服务设计

### 5.1 Discovery 机制

NVMe-oF Discovery 服务允许 Initiator 发现可用的 Target。

```go
// internal/nvmeof/discovery/service.go

package discovery

import (
    "context"
    "fmt"
)

// DiscoveryService 发现服务
type DiscoveryService struct {
    port int
}

// TargetInfo 目标信息
type TargetInfo struct {
    NQN       string   `json:"nqn"`
    Transport string   `json:"transport"`
    Addresses []string `json:"addresses"`
    Port      int      `json:"port"`
}

// Discover 发现指定地址的 NVMe-oF Target
func (d *DiscoveryService) Discover(ctx context.Context, addr string, port int) ([]TargetInfo, error) {
    // nvme discover -t tcp -a <addr> -s <port>
    // 解析返回结果
    return []TargetInfo{}, nil
}
```

---

## 六、API 设计

### 6.1 REST API 端点

```
# Target 管理
GET    /api/v1/nvmeof/targets           # 列出所有 Target
POST   /api/v1/nvmeof/targets           # 创建 Target
GET    /api/v1/nvmeof/targets/{id}      # 获取 Target 详情
PUT    /api/v1/nvmeof/targets/{id}      # 更新 Target 配置
DELETE /api/v1/nvmeof/targets/{id}      # 删除 Target

# Initiator 管理
GET    /api/v1/nvmeof/connections      # 列出所有连接
POST   /api/v1/nvmeof/connections      # 创建连接
DELETE /api/v1/nvmeof/connections/{id}  # 断开连接

# Discovery
GET    /api/v1/nvmeof/discover          # 发现目标
```

### 6.2 API 实现 (Gin 框架)

```go
// api/v1/nvmeof.go

package v1

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
)

// NVMeOFHandler NVMe-oF API 处理器
type NVMeOFHandler struct {
    targetService    *target.Service
    initiatorService *initiator.Service
    discoveryService *discovery.Service
}

// ListTargets 列出所有 Target
func (h *NVMeOFHandler) ListTargets(c *gin.Context) {
    targets, err := h.targetService.List(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"targets": targets})
}

// CreateTarget 创建 Target
func (h *NVMeOFHandler) CreateTarget(c *gin.Context) {
    var req target.CreateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    t, err := h.targetService.Create(c.Request.Context(), &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{"target": t})
}

// Connect 连接到 Target
func (h *NVMeOFHandler) Connect(c *gin.Context) {
    var req initiator.ConnectRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    conn, err := h.initiatorService.Connect(c.Request.Context(), &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{"connection": conn})
}
```

---

## 七、性能调优

### 7.1 内核参数

```bash
# /etc/sysctl.d/99-nvmeof.conf

# 增加 I/O 调度器队列深度
net.core.netdev_max_backlog = 10000
net.core.somaxconn = 1024

# TCP 性能优化
net.ipv4.tcp_rmem = 4096 131072 16777216
net.ipv4.tcp_wmem = 4096 131072 16777216
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216

# 启用 TCP 快速打开
net.ipv4.tcp_fastopen = 3
```

### 7.2 NVMe 参数

```bash
# 调整 NVMe 设备队列深度
echo 1024 > /sys/block/nvme0n1/queue/nr_requests
echo 1024 > /sys/block/nvme0n1/queue/read_ahead_kb

# 启用 I/O 调度 (none/mq-deadline/kyber)
echo none > /sys/block/nvme0n1/queue/scheduler
```

---

## 八、监控与诊断

### 8.1 监控指标

```go
// internal/nvmeof/metrics/metrics.go

package metrics

// TargetMetrics Target 性能指标
type TargetMetrics struct {
    // I/O 统计
    ReadOps    uint64 `json:"read_ops"`
    WriteOps   uint64 `json:"write_ops"`
    ReadBytes  uint64 `json:"read_bytes"`
    WriteBytes uint64 `json:"write_bytes"`
    
    // 延迟
    ReadLatencyMs  float64 `json:"read_latency_ms"`
    WriteLatencyMs float64 `json:"write_latency_ms"`
    
    // 队列深度
    QueueDepth uint64 `json:"queue_depth"`
    
    // 连接状态
    ConnectedHosts int `json:"connected_hosts"`
}

// InitiatorMetrics Initiator 性能指标
type InitiatorMetrics struct {
    // 连接状态
    Connected bool `json:"connected"`
    
    // 重连计数
    ReconnectCount int `json:"reconnect_count"`
    
    // I/O 错误计数
    IOErrors uint64 `json:"io_errors"`
}
```

### 8.2 诊断命令

```bash
# 查看连接状态
nvme list-subsys

# 查看 Target 统计
cat /sys/kernel/config/nvmet/subsystems/*/namespaces/*/statistics/*

# 查看设备信息
nvme id-ctrl /dev/nvme0
nvme id-ns /dev/nvme0n1
nvme smart-log /dev/nvme0
```

---

## 九、安全设计

### 9.1 认证机制

NVMe-oF 支持 DH-HMAC-CHAP 认证：

```go
// internal/nvmeof/auth/auth.go

package auth

// DHCHAPConfig DH-HMAC-CHAP 认证配置
type DHCHAPConfig struct {
    // 密钥
    Key string `json:"key"`
    
    // 哈希算法: sha256, sha384, sha512
    Hash string `json:"hash"`
    
    // DH 组: null, ffdhe2048, ffdhe3072, ffdhe4096, ffdhe6144, ffdhe8192
    DHGroup string `json:"dh_group"`
}

// SetAuth 配置子系统认证
func SetAuth(subsysNQN string, config *DHCHAPConfig) error {
    // 配置内核 nvmet 认证
    return nil
}
```

### 9.2 访问控制

```go
// internal/nvmeof/acl/acl.go

package acl

// ACL 访问控制列表
type ACL struct {
    // 允许的主机 NQN 列表
    AllowedHosts []string `json:"allowed_hosts"`
    
    // 是否允许任意主机
    AllowAny bool `json:"allow_any"`
}

// ValidateHost 验证主机是否有访问权限
func (a *ACL) ValidateHost(hostNQN string) bool {
    if a.AllowAny {
        return true
    }
    for _, allowed := range a.AllowedHosts {
        if allowed == hostNQN {
            return true
        }
    }
    return false
}
```

---

## 十、高可用设计

### 10.1 ANA (Asymmetric Namespace Access)

ANA 提供类似于 ALUA 的多路径支持：

```go
// internal/nvmeof/ana/ana.go

package ana

// ANAState ANA 状态
type ANAState int

const (
    ANAStateOptimized      ANAState = 1 // 最优路径
    ANAStateNonOptimized   ANAState = 2 // 非最优路径
    ANAStateInaccessible   ANAState = 3 // 不可访问
    ANAStatePersistentLoss ANAState = 4 // 持久丢失
    ANAStateChange         ANAState = 15 // 状态变更中
)

// ANAGroup ANA 组
type ANAGroup struct {
    ID    int       `json:"id"`
    State ANAState  `json:"state"`
    Name  string    `json:"name"`
}
```

### 10.2 多路径配置

```bash
# /etc/nvme/discovery.conf
# 多路径 Discovery 服务
rdma:192.168.1.10:4420
rdma:192.168.1.11:4420

# /etc/nvme/config.json
{
  "hostnqn": "nqn.2026-03.org.nas-os:host1",
  "hosts": [
    {
      "nqn": "nqn.2026-03.org.nas-os:subsystem1",
      "multipath": "failover"
    }
  ]
}
```

---

## 十一、参考实现

### 11.1 TrueNAS Scale 参考

TrueNAS Scale 使用以下组件实现 NVMe-oF：

| 组件 | 版本要求 | 说明 |
|------|---------|------|
| Linux Kernel | >= 5.15 | 内核 nvmet/nvmf 支持 |
| nvme-cli | >= 2.0 | 用户空间管理工具 |
| nvmetcli | >= 1.0 | Target 配置工具 |

### 11.2 相关文档

- [NVMe Specification](https://nvmexpress.org/specifications/)
- [Linux NVMe-oF Documentation](https://docs.kernel.org/admin-guide/nvme.html)
- [TrueNAS Scale Documentation](https://www.truenas.com/docs/scale/)

---

## 十二、实现路线图

### Phase 1: 基础架构 (v2.x)
- [ ] 内核模块检测与自动加载
- [ ] 基础 Target 配置支持
- [ ] 基础 Initiator 连接支持
- [ ] CLI 工具集成

### Phase 2: Web UI (v2.x+1)
- [ ] Target 管理界面
- [ ] Initiator 连接界面
- [ ] 性能监控面板

### Phase 3: 高级特性 (v2.x+2)
- [ ] ANA 多路径支持
- [ ] DH-HMAC-CHAP 认证
- [ ] 高可用集群支持

---

**更新历史**:
| 版本 | 日期 | 变更说明 |
|------|------|----------|
| v1.0 | 2026-03-31 | 初始框架文档 |