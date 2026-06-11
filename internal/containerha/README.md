# ContainerHA - 容器高可用故障转移模块

## 概述

ContainerHA 是 nas-os 的容器高可用故障转移模块，参考 TrueNAS 26 的容器HA特性实现。支持 LXC/Docker 容器的故障检测和自动故障转移。

## 特性

- **多节点集群支持**：支持主从节点架构，自动选举和故障转移
- **健康检查**：定期检查容器和节点健康状态
- **自动故障转移**：心跳超时、健康检查失败、资源耗尽时自动触发
- **容器状态同步**：支持检查点/恢复和实时同步模式
- **静态IP故障转移**：支持虚拟IP在节点间迁移
- **自动回切**：主节点恢复后自动回切容器
- **资源监控**：CPU、内存、磁盘使用率监控
- **RESTful API**：完整的HTTP API用于管理和监控

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                   ContainerHA Cluster                    │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │   Master    │  │   Slave 1   │  │   Slave 2   │    │
│  │   Node      │  │   Node      │  │   Node      │    │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘    │
│         │                │                │            │
│         └────────────────┼────────────────┘            │
│                          │                             │
│              ┌───────────┴───────────┐                │
│              │    FailoverManager    │                │
│              └───────────┬───────────┘                │
│                          │                             │
│         ┌────────────────┼────────────────┐           │
│         │                │                │           │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐   │
│  │  Health     │  │   Sync     │  │  Failover   │   │
│  │  Checker    │  │  Manager   │  │  Engine     │   │
│  └─────────────┘  └─────────────┘  └─────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## 文件结构

```
internal/containerha/
├── types.go              # 类型定义
├── manager.go            # 核心管理器
├── failover.go           # 故障转移逻辑
├── handlers.go           # HTTP API handlers
├── containerha_test.go   # 测试
└── README.md             # 本文件
```

## 配置

### ContainerHAConfig

```go
type ContainerHAConfig struct {
    ClusterName         string           // 集群名称
    PrimaryNode         NodeConfig       // 主节点配置
    SecondaryNodes      []NodeConfig     // 从节点列表
    HealthCheckInterval int              // 健康检查间隔（秒）
    FailureThreshold    int              // 故障阈值（连续失败次数）
    AutoFailback        bool             // 是否自动回切
    FailbackDelay       int              // 自动回切延迟（秒）
    HeartbeatTimeout    int              // 心跳超时时间（秒）
    SyncMode            string           // 同步模式：checkpoint/realtime
    SyncInterval        int              // 同步间隔（秒）
    ProtectedContainers []ContainerConfig // 受保护容器列表
    EnableStaticIP      bool             // 是否支持静态IP故障转移
    VirtualIPs          []VirtualIPConfig // 虚拟IP配置
    EnableResourceCheck bool             // 是否启用资源检查
    ResourceThresholds  ResourceThresholds // 资源阈值配置
}
```

### 示例配置

```yaml
clusterName: "production-cluster"
primaryNode:
  id: "node-1"
  address: "192.168.1.100"
  port: 8080
  role: "master"
  weight: 100

secondaryNodes:
  - id: "node-2"
    address: "192.168.1.101"
    port: 8080
    role: "slave"
    weight: 90
  - id: "node-3"
    address: "192.168.1.102"
    port: 8080
    role: "slave"
    weight: 80

healthCheckInterval: 10
failureThreshold: 3
autoFailback: true
failbackDelay: 60
heartbeatTimeout: 30
syncMode: "checkpoint"
syncInterval: 60

protectedContainers:
  - containerId: "web-app-1"
    type: "docker"
    enableFailover: true
    priority: 1
    staticIP: "192.168.1.200"
    healthCheckPort: 8080
    healthCheckPath: "/health"
  - containerId: "db-app-1"
    type: "lxc"
    enableFailover: true
    priority: 2
    staticIP: "192.168.1.201"

virtualIPs:
  - ip: "192.168.1.200"
    interface: "eth0"
    subnetMask: "255.255.255.0"
    gateway: "192.168.1.1"

resourceThresholds:
  cpuThreshold: 90.0
  memoryThreshold: 85.0
  diskThreshold: 95.0
```

## API 接口

### 状态查询

```http
GET /api/v1/containerha/status
```

返回集群状态，包括节点信息、运行中的容器、同步状态等。

### 故障转移

```http
POST /api/v1/containerha/failover
Content-Type: application/json

{
  "targetNode": "node-2",
  "containers": ["web-app-1", "db-app-1"],
  "reason": "手动故障转移",
  "force": false,
  "planned": false
}
```

### 配置管理

```http
GET /api/v1/containerha/config
PUT /api/v1/containerha/config
```

### 节点管理

```http
GET /api/v1/containerha/nodes
GET /api/v1/containerha/nodes/{nodeId}
```

### 容器管理

```http
GET /api/v1/containerha/containers
GET /api/v1/containerha/containers/{containerId}
```

### 同步管理

```http
POST /api/v1/containerha/sync
GET /api/v1/containerha/sync/status
```

### 健康检查

```http
GET /api/v1/containerha/health
GET /api/v1/containerha/health/{nodeId}
```

### 心跳接收

```http
POST /api/v1/containerha/heartbeat
Content-Type: application/json

{
  "nodeId": "node-2",
  "timestamp": "2024-01-01T00:00:00Z",
  "status": "online",
  "resourceUsage": {
    "cpuUsage": 50.0,
    "memoryUsage": 60.0,
    "diskUsage": 70.0
  },
  "sequenceNumber": 1
}
```

### 故障转移历史

```http
GET /api/v1/containerha/history
```

## 使用示例

### 创建并启动管理器

```go
package main

import (
    "context"
    "log"
    "nas-os/internal/containerha"
)

func main() {
    // 创建配置
    config := &containerha.ContainerHAConfig{
        ClusterName:         "my-cluster",
        HealthCheckInterval: 10,
        FailureThreshold:    3,
        AutoFailback:        true,
        FailbackDelay:       60,
        HeartbeatTimeout:    30,
        SyncMode:            "checkpoint",
        SyncInterval:        60,
        PrimaryNode: containerha.NodeConfig{
            ID:      "node-1",
            Address: "192.168.1.100",
            Port:    8080,
            Role:    "master",
        },
        SecondaryNodes: []containerha.NodeConfig{
            {
                ID:      "node-2",
                Address: "192.168.1.101",
                Port:    8080,
                Role:    "slave",
            },
        },
        ProtectedContainers: []containerha.ContainerConfig{
            {
                ContainerID:    "web-app",
                Type:           "docker",
                EnableFailover: true,
                Priority:       1,
            },
        },
    }

    // 创建管理器
    manager := containerha.NewFailoverManager(config, "node-1")

    // 启动管理器
    ctx := context.Background()
    if err := manager.Start(ctx); err != nil {
        log.Fatalf("启动失败: %v", err)
    }
    defer manager.Stop()

    // 创建HTTP处理器
    handler := containerha.NewContainerHAHandler(manager)

    // 启动HTTP服务器
    if err := containerha.StartHTTPServer(handler, ":8080"); err != nil {
        log.Fatalf("HTTP服务器启动失败: %v", err)
    }
}
```

### 手动故障转移

```go
// 执行手动故障转移
request := &containerha.FailoverRequest{
    TargetNode: "node-2",
    Containers: []string{"web-app"},
    Reason:     "计划维护",
    Planned:    true,
}

response, err := manager.ExecuteFailover(request)
if err != nil {
    log.Printf("故障转移失败: %v", err)
} else {
    log.Printf("故障转移成功: %s", response.Message)
}
```

### 处理心跳

```go
heartbeat := &containerha.HeartbeatMessage{
    NodeID:    "node-2",
    Timestamp: time.Now(),
    Status:    "online",
    ResourceUsage: containerha.ResourceUsage{
        CPUUsage:    50.0,
        MemoryUsage: 60.0,
        DiskUsage:   70.0,
    },
    SequenceNumber: 1,
}

if err := manager.ProcessHeartbeat(heartbeat); err != nil {
    log.Printf("心跳处理失败: %v", err)
}
```

## 故障检测

故障转移管理器会检测以下故障情况：

1. **心跳超时**：节点在配置的时间内未发送心跳
2. **健康检查失败**：节点连续失败次数超过阈值
3. **资源耗尽**：CPU、内存或磁盘使用率超过阈值

## 容器同步

支持两种同步模式：

### Checkpoint 模式
- 定期创建容器检查点
- 检查点包含容器完整状态
- 故障转移时从检查点恢复
- 适合有状态容器

### Realtime 模式
- 实时监控文件系统变化
- 使用 inotify 或类似机制
- 变更立即同步到备份节点
- 适合需要最小数据丢失的场景

## 静态IP故障转移

支持虚拟IP在节点间迁移：

1. 容器配置静态IP
2. 虚拟IP绑定到当前运行节点
3. 故障转移时，虚拟IP迁移到目标节点
4. 客户端无感知切换

## 自动回切

当主节点恢复后，可以自动将容器迁移回主节点：

1. 检测主节点恢复在线
2. 等待配置的延迟时间
3. 验证主节点健康状态
4. 执行容器迁移
5. 更新虚拟IP绑定

## 测试

运行测试：

```bash
cd nas-os
go test ./internal/containerha/ -v
```

运行性能测试：

```bash
go test ./internal/containerha/ -bench=.
```

## 依赖

- Go 1.21+
- github.com/google/uuid

## 注意事项

1. 测试环境中的网络连接会超时，这是正常的
2. 实际部署时需要配置正确的节点地址和端口
3. 容器运行时（Docker/LXC）需要支持检查点功能
4. 虚拟IP迁移需要网络设备支持（如 keepalived）

## 参考

- [TrueNAS SCALE High Availability](https://www.truenas.com/docs/scale/)
- [LXC Checkpoint/Restore](https://linuxcontainers.org/lxc/getting-started/)
- [Docker Checkpoint](https://docs.docker.com/reference/cli/docker/checkpoint/)
