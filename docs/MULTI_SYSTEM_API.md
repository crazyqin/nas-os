# 兵部：多系统管理API设计

## 概述
对标TrueNAS Connect，设计nas-os多系统管理能力。

## API接口设计

### 1. NodeManagementService

```go
// NodeManagementService 多系统节点管理服务
type NodeManagementService interface {
    // RegisterNode 注册新节点到管理系统
    RegisterNode(ctx context.Context, req *RegisterNodeRequest) (*Node, error)
    
    // UnregisterNode 从管理系统移除节点
    UnregisterNode(ctx context.Context, nodeID string) error
    
    // ListNodes 获取所有已注册节点列表
    ListNodes(ctx context.Context) ([]*Node, error)
    
    // GetNodeStatus 获取单个节点详细状态
    GetNodeStatus(ctx context.Context, nodeID string) (*NodeStatus, error)
    
    // Heartbeat 节点心跳上报
    Heartbeat(ctx context.Context, nodeID string, status *NodeHeartbeat) error
    
    // ExecuteRemoteCommand 远程执行命令
    ExecuteRemoteCommand(ctx context.Context, nodeID string, cmd *RemoteCommand) (*CommandResult, error)
}

// Node 节点信息
type Node struct {
    ID           string            `json:"id"`
    Name         string            `json:"name"`
    Address      string            `json:"address"`
    Port         int               `json:"port"`
    Version      string            `json:"version"`
    Status       NodeHealthStatus  `json:"status"`
    Capabilities []string          `json:"capabilities"`
    RegisteredAt time.Time         `json:"registeredAt"`
    LastSeen     time.Time         `json:"lastSeen"`
}

// NodeHealthStatus 节点健康状态
type NodeHealthStatus struct {
    OverallScore    int                `json:"overallScore"` // 0-100
    StorageHealth   HealthMetric       `json:"storageHealth"`
    NetworkHealth   HealthMetric       `json:"networkHealth"`
    ServiceHealth   HealthMetric       `json:"serviceHealth"`
    Alerts          []Alert            `json:"alerts"`
}

// HealthMetric 健康指标
type HealthMetric struct {
    Score   int    `json:"score"`
    Status  string `json:"status"` // healthy, warning, critical
    Details string `json:"details"`
}
```

### 2. FleetManager

```go
// FleetManager 多节点复制管理
type FleetManager interface {
    // CreateReplicationTask 创建跨节点复制任务
    CreateReplicationTask(ctx context.Context, req *ReplicationTaskRequest) (*ReplicationTask, error)
    
    // ScheduleReplication 调度复制任务到目标节点
    ScheduleReplication(ctx context.Context, taskID string, targetNode string) error
    
    // GetReplicationProgress 获取复制进度
    GetReplicationProgress(ctx context.Context, taskID string) (*ReplicationProgress, error)
    
    // PauseReplication 暂停复制任务
    PauseReplication(ctx context.Context, taskID string) error
    
    // ResumeReplication 恢复复制任务（支持断点续传）
    ResumeReplication(ctx context.Context, taskID string) error
    
    // CancelReplication 取消复制任务
    CancelReplication(ctx context.Context, taskID string) error
}

// ReplicationTaskRequest 复制任务请求
type ReplicationTaskRequest struct {
    SourceNode    string        `json:"sourceNode"`
    SourcePath    string        `json:"sourcePath"`
    TargetNodes   []string      `json:"targetNodes"` // 支持多目标
    Priority      TaskPriority `json:"priority"`
    BandwidthLimit int          `json:"bandwidthLimit"` // MB/s
    Compression   bool          `json:"compression"`
    Encryption    bool          `json:"encryption"`
}

// ReplicationProgress 复制进度
type ReplicationProgress struct {
    TaskID         string    `json:"taskId"`
    Status         string    `json:"status"`
    TotalBytes     int64     `json:"totalBytes"`
    TransferredBytes int64   `json:"transferredBytes"`
    Percentage     float64   `json:"percentage"`
    Speed          float64   `json:"speed"` // MB/s
    EstimatedTime  int       `json:"estimatedTime"` // seconds
    StartedAt      time.Time `json:"startedAt"`
    UpdatedAt      time.Time `json:"updatedAt"`
}
```

### 3. GlobalSearchService

```go
// GlobalSearchService 全局搜索服务（对标TrueNAS UI Search）
type GlobalSearchService interface {
    // Search 全局搜索
    Search(ctx context.Context, query *SearchQuery) (*SearchResults, error)
    
    // SearchAcrossNodes 跨节点搜索
    SearchAcrossNodes(ctx context.Context, query *SearchQuery, nodes []string) (*AggregatedResults, error)
    
    // IndexNodeData 索引节点数据
    IndexNodeData(ctx context.Context, nodeID string) error
}

// SearchQuery 搜索查询
type SearchQuery struct {
    Text      string   `json:"text"`
    Type      string   `json:"type"` // file, volume, share, user, setting
    Nodes     []string `json:"nodes"` // 可选，指定节点范围
    Limit     int      `json:"limit"`
    Offset    int      `json:"offset"`
}

// SearchResults 搜索结果
type SearchResults struct {
    Total    int             `json:"total"`
    Results  []SearchResult  `json:"results"`
    Node     string          `json:"node"`
}

// SearchResult 单个搜索结果
type SearchResult struct {
    Type     string            `json:"type"`
    ID       string            `json:"id"`
    Name     string            `json:"name"`
    Path     string            `json:"path"`
    NodeID   string            `json:"nodeId"`
    Metadata map[string]string `json:"metadata"`
}
```

## HTTP API端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/nodes` | GET | 获取节点列表 |
| `/api/v1/nodes` | POST | 注册节点 |
| `/api/v1/nodes/{id}` | GET | 获取节点状态 |
| `/api/v1/nodes/{id}` | DELETE | 移除节点 |
| `/api/v1/nodes/{id}/heartbeat` | POST | 心跳上报 |
| `/api/v1/nodes/{id}/execute` | POST | 远程执行命令 |
| `/api/v1/fleet/tasks` | POST | 创建复制任务 |
| `/api/v1/fleet/tasks/{id}` | GET | 获取任务进度 |
| `/api/v1/fleet/tasks/{id}/pause` | POST | 暂停任务 |
| `/api/v1/fleet/tasks/{id}/resume` | POST | 恢复任务 |
| `/api/v1/search` | GET | 全局搜索 |
| `/api/v1/search/multi` | GET | 跨节点搜索 |

## 实现计划

1. **M1 (04-03)**: NodeManagementService核心接口实现
2. **M2 (04-05)**: FleetManager复制任务调度
3. **M3 (04-08)**: GlobalSearchService跨节点搜索
4. **M4 (04-10)**: HTTP API端点暴露
5. **M5 (04-15)**: 集成测试与文档

## 参考实现

- `internal/cluster/` - 现有集群支持基础
- `internal/replication/` - 现有复制功能
- TrueNAS Connect API文档