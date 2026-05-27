// Package clustermanager 提供集群多节点管理功能
// 对标 TrueNAS TrueCommand / 群晖 CMS (Central Management System)
// 支持节点发现、状态监控、健康检测、拓扑可视化、任务调度、数据同步、负载均衡、告警、分组管理
package clustermanager

import (
	"sync"
	"time"
)

// NodeStatus 节点状态.
type NodeStatus string

const (
	NodeStatusOnline     NodeStatus = "online"     // 在线
	NodeStatusOffline    NodeStatus = "offline"    // 离线
	NodeStatusMaintenance NodeStatus = "maintenance" // 维护模式
	NodeStatusError      NodeStatus = "error"      // 错误状态
	NodeStatusSyncing    NodeStatus = "syncing"    // 同步中
)

// NodeType 节点类型.
type NodeType string

const (
	NodeTypeStorage  NodeType = "storage"  // 存储节点
	NodeTypeCompute  NodeType = "compute"  // 计算节点
	NodeTypeHybrid   NodeType = "hybrid"   // 混合节点
	NodeTypeGateway  NodeType = "gateway"  // 网关节点
	NodeTypeMonitor  NodeType = "monitor"  // 监控节点
)

// LoadBalanceStrategy 负载均衡策略.
type LoadBalanceStrategy string

const (
	StrategyCPU      LoadBalanceStrategy = "cpu"      // CPU负载
	StrategyMemory   LoadBalanceStrategy = "memory"   // 内存负载
	StrategyStorage  LoadBalanceStrategy = "storage"  // 存储负载
	StrategyNetwork  LoadBalanceStrategy = "network"  // 网络负载
	StrategyCustom   LoadBalanceStrategy = "custom"   // 自定义策略
	StrategyRoundRobin LoadBalanceStrategy = "round_robin" // 轮询
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 待执行
	TaskStatusRunning   TaskStatus = "running"   // 执行中
	TaskStatusCompleted TaskStatus = "completed" // 已完成
	TaskStatusFailed    TaskStatus = "failed"    // 失败
	TaskStatusCancelled TaskStatus = "cancelled" // 已取消
)

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"     // 信息
	AlertLevelWarning  AlertLevel = "warning"  // 警告
	AlertLevelCritical AlertLevel = "critical" // 严重
	AlertLevelEmergency AlertLevel = "emergency" // 紧急
)

// AlertType 告警类型.
type AlertType string

const (
	AlertTypeNodeFailure      AlertType = "node_failure"      // 节点故障
	AlertTypeResourceLow      AlertType = "resource_low"      // 资源不足
	AlertTypeNetworkDown      AlertType = "network_down"      // 网络中断
	AlertTypeDiskFull         AlertType = "disk_full"         // 磁盘满
	AlertTypeHighCPU          AlertType = "high_cpu"          // CPU过高
	AlertTypeHighMemory       AlertType = "high_memory"       // 内存过高
	AlertTypeTemperatureHigh  AlertType = "temperature_high"  // 温度过高
	AlertTypeSyncFailure      AlertType = "sync_failure"      // 同步失败
)

// SyncStatus 同步状态.
type SyncStatus string

const (
	SyncStatusIdle      SyncStatus = "idle"      // 空闲
	SyncStatusSyncing   SyncStatus = "syncing"   // 同步中
	SyncStatusCompleted SyncStatus = "completed" // 已完成
	SyncStatusFailed    SyncStatus = "failed"    // 失败
	SyncStatusPaused    SyncStatus = "paused"    // 已暂停
)

// ClusterNode 集群节点.
type ClusterNode struct {
	mu sync.RWMutex `json:"-"`

	// 基本信息
	ID          string   `json:"id"`          // 节点ID
	Name        string   `json:"name"`        // 节点名称
	Hostname    string   `json:"hostname"`    // 主机名
	IPAddress   string   `json:"ipAddress"`   // IP地址
	Port        int      `json:"port"`        // 端口
	Type        NodeType `json:"type"`        // 节点类型
	Status      NodeStatus `json:"status"`    // 节点状态
	Version     string   `json:"version"`     // 软件版本
	OS          string   `json:"os"`          // 操作系统
	Arch        string   `json:"arch"`        // 架构

	// 健康信息
	Health      *NodeHealth `json:"health"`   // 健康信息

	// 分组信息
	GroupID     string   `json:"groupId"`     // 分组ID
	Tags        []string `json:"tags"`        // 标签
	Location    string   `json:"location"`    // 位置信息

	// 资源信息
	TotalCPU    int     `json:"totalCpu"`    // CPU总核数
	TotalMemory int64   `json:"totalMemory"` // 内存总量（字节）
	TotalDisk   int64   `json:"totalDisk"`   // 磁盘总量（字节）

	// 时间信息
	RegisteredAt time.Time `json:"registeredAt"` // 注册时间
	LastSeenAt   time.Time `json:"lastSeenAt"`   // 最后在线时间
	UpdatedAt    time.Time `json:"updatedAt"`    // 更新时间

	// 元数据
	Metadata map[string]string `json:"metadata,omitempty"` // 自定义元数据
}

// IsOnline 检查节点是否在线.
func (n *ClusterNode) IsOnline() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Status == NodeStatusOnline
}

// UpdateStatus 更新节点状态.
func (n *ClusterNode) UpdateStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Status = status
	n.UpdatedAt = time.Now()
	if status == NodeStatusOnline {
		n.LastSeenAt = time.Now()
	}
}

// NodeHealth 节点健康信息.
type NodeHealth struct {
	mu sync.RWMutex `json:"-"`

	// CPU信息
	CPUUsage    float64 `json:"cpuUsage"`    // CPU使用率（%）
	CPUTemp     float64 `json:"cpuTemp"`     // CPU温度（℃）
	LoadAvg1    float64 `json:"loadAvg1"`    // 1分钟平均负载
	LoadAvg5    float64 `json:"loadAvg5"`    // 5分钟平均负载
	LoadAvg15   float64 `json:"loadAvg15"`   // 15分钟平均负载

	// 内存信息
	MemoryUsage    float64 `json:"memoryUsage"`    // 内存使用率（%）
	MemoryUsed     int64   `json:"memoryUsed"`     // 已用内存（字节）
	MemoryTotal    int64   `json:"memoryTotal"`    // 总内存（字节）
	SwapUsage      float64 `json:"swapUsage"`      // Swap使用率（%）

	// 磁盘信息
	DiskUsage      float64 `json:"diskUsage"`      // 磁盘使用率（%）
	DiskUsed       int64   `json:"diskUsed"`       // 已用磁盘（字节）
	DiskTotal      int64   `json:"diskTotal"`      // 总磁盘（字节）
	DiskReadRate   float64 `json:"diskReadRate"`   // 磁盘读取速率（bytes/s）
	DiskWriteRate  float64 `json:"diskWriteRate"`  // 磁盘写入速率（bytes/s）

	// 网络信息
	NetworkIn      int64   `json:"networkIn"`      // 网络入流量（bytes）
	NetworkOut     int64   `json:"networkOut"`     // 网络出流量（bytes）
	NetworkInRate  float64 `json:"networkInRate"`  // 网络入速率（bytes/s）
	NetworkOutRate float64 `json:"networkOutRate"` // 网络出速率（bytes/s）
	NetworkErrors  int64   `json:"networkErrors"`  // 网络错误数

	// 系统信息
	Uptime         int64   `json:"uptime"`         // 运行时间（秒）
	Processes      int     `json:"processes"`      // 进程数
	Temperature    float64 `json:"temperature"`    // 系统温度（℃）

	// 磁盘健康（SMART）
	DiskHealth     string  `json:"diskHealth"`     // 磁盘健康状态
	DiskTemp       float64 `json:"diskTemp"`       // 磁盘温度

	// 时间戳
	CollectedAt    time.Time `json:"collectedAt"`  // 采集时间
}

// GetSnapshot 获取健康信息快照（线程安全）.
func (h *NodeHealth) GetSnapshot() *NodeHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()
	// 创建副本
	snapshot := *h
	return &snapshot
}

// Update 更新健康信息.
func (h *NodeHealth) Update(cpu, memory, disk, temp float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.CPUUsage = cpu
	h.MemoryUsage = memory
	h.DiskUsage = disk
	h.Temperature = temp
	h.CollectedAt = time.Now()
}

// NodeGroup 节点分组.
type NodeGroup struct {
	mu sync.RWMutex `json:"-"`

	ID          string   `json:"id"`          // 分组ID
	Name        string   `json:"name"`        // 分组名称
	Description string   `json:"description"` // 描述
	Type        string   `json:"type"`        // 分组类型（purpose/location/department）
	NodeIDs     []string `json:"nodeIds"`     // 节点ID列表
	Tags        []string `json:"tags"`        // 标签
	Priority    int      `json:"priority"`    // 优先级
	CreatedAt   time.Time `json:"createdAt"`  // 创建时间
	UpdatedAt   time.Time `json:"updatedAt"`  // 更新时间
}

// AddNode 添加节点到分组.
func (g *NodeGroup) AddNode(nodeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, id := range g.NodeIDs {
		if id == nodeID {
			return
		}
	}
	g.NodeIDs = append(g.NodeIDs, nodeID)
	g.UpdatedAt = time.Now()
}

// RemoveNode 从分组移除节点.
func (g *NodeGroup) RemoveNode(nodeID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, id := range g.NodeIDs {
		if id == nodeID {
			g.NodeIDs = append(g.NodeIDs[:i], g.NodeIDs[i+1:]...)
			g.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// ContainsNode 检查分组是否包含节点.
func (g *NodeGroup) ContainsNode(nodeID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, id := range g.NodeIDs {
		if id == nodeID {
			return true
		}
	}
	return false
}

// ClusterTask 集群任务.
type ClusterTask struct {
	mu sync.RWMutex `json:"-"`

	ID          string     `json:"id"`          // 任务ID
	Name        string     `json:"name"`        // 任务名称
	Type        string     `json:"type"`        // 任务类型
	Status      TaskStatus `json:"status"`      // 任务状态
	Priority    int        `json:"priority"`    // 优先级（0-100）

	// 调度信息
	SourceNodeID string `json:"sourceNodeId"` // 来源节点ID
	TargetNodeID string `json:"targetNodeId"` // 目标节点ID
	NodeIDs      []string `json:"nodeIds"`     // 执行节点ID列表

	// 任务数据
	Payload     map[string]interface{} `json:"payload"`     // 任务数据
	Result      map[string]interface{} `json:"result"`      // 执行结果
	Error       string                 `json:"error"`       // 错误信息

	// 时间信息
	CreatedAt   time.Time  `json:"createdAt"`   // 创建时间
	StartedAt   *time.Time `json:"startedAt"`   // 开始时间
	CompletedAt *time.Time `json:"completedAt"` // 完成时间
	Timeout     time.Duration `json:"timeout"`   // 超时时间

	// 进度
	Progress    int `json:"progress"` // 进度（0-100）
}

// IsExpired 检查任务是否超时.
func (t *ClusterTask) IsExpired() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.Timeout <= 0 || t.StartedAt == nil {
		return false
	}
	return time.Since(*t.StartedAt) > t.Timeout
}

// UpdateProgress 更新任务进度.
func (t *ClusterTask) UpdateProgress(progress int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	t.Progress = progress
}

// SetCompleted 设置任务完成.
func (t *ClusterTask) SetCompleted(result map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = TaskStatusCompleted
	t.Result = result
	now := time.Now()
	t.CompletedAt = &now
	t.Progress = 100
}

// SetFailed 设置任务失败.
func (t *ClusterTask) SetFailed(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = TaskStatusFailed
	t.Error = err
	now := time.Now()
	t.CompletedAt = &now
}

// DataSyncTask 数据同步任务.
type DataSyncTask struct {
	mu sync.RWMutex `json:"-"`

	ID           string     `json:"id"`           // 同步任务ID
	Name         string     `json:"name"`         // 任务名称
	SourceNodeID string     `json:"sourceNodeId"` // 源节点ID
	TargetNodeIDs []string  `json:"targetNodeIds"` // 目标节点ID列表
	SourcePath   string     `json:"sourcePath"`   // 源路径
	TargetPath   string     `json:"targetPath"`   // 目标路径
	Status       SyncStatus `json:"status"`       // 同步状态

	// 进度信息
	TotalBytes   int64   `json:"totalBytes"`   // 总字节数
	SyncedBytes  int64   `json:"syncedBytes"`  // 已同步字节数
	TotalFiles   int     `json:"totalFiles"`   // 总文件数
	SyncedFiles  int     `json:"syncedFiles"`  // 已同步文件数
	Speed        float64 `json:"speed"`        // 同步速度（bytes/s）

	// 时间信息
	StartedAt    time.Time  `json:"startedAt"`    // 开始时间
	CompletedAt  *time.Time `json:"completedAt"`  // 完成时间
	NextSyncAt   *time.Time `json:"nextSyncAt"`   // 下次同步时间

	// 配置
	Interval     time.Duration `json:"interval"`  // 同步间隔
	AutoSync     bool          `json:"autoSync"`  // 自动同步
	Compress     bool          `json:"compress"`  // 压缩传输
	Encrypt      bool          `json:"encrypt"`   // 加密传输
}

// GetProgress 获取同步进度百分比.
func (d *DataSyncTask) GetProgress() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.TotalBytes <= 0 {
		return 0
	}
	return float64(d.SyncedBytes) / float64(d.TotalBytes) * 100
}

// UpdateProgress 更新同步进度.
func (d *DataSyncTask) UpdateProgress(syncedBytes int64, syncedFiles int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.SyncedBytes = syncedBytes
	d.SyncedFiles = syncedFiles
	if d.StartedAt.Unix() > 0 {
		elapsed := time.Since(d.StartedAt).Seconds()
		if elapsed > 0 {
			d.Speed = float64(syncedBytes) / elapsed
		}
	}
}

// ClusterAlert 集群告警.
type ClusterAlert struct {
	mu sync.RWMutex `json:"-"`

	ID          string    `json:"id"`          // 告警ID
	Type        AlertType `json:"type"`        // 告警类型
	Level       AlertLevel `json:"level"`      // 告警级别
	NodeID      string    `json:"nodeId"`      // 相关节点ID
	Title       string    `json:"title"`       // 告警标题
	Message     string    `json:"message"`     // 告警详情
	Value       float64   `json:"value"`       // 当前值
	Threshold   float64   `json:"threshold"`   // 阈值

	// 状态
	Active      bool      `json:"active"`      // 是否活跃
	Acknowledged bool     `json:"acknowledged"` // 是否已确认
	Resolved    bool      `json:"resolved"`    // 是否已解决

	// 时间信息
	TriggeredAt time.Time  `json:"triggeredAt"` // 触发时间
	AckedAt     *time.Time `json:"ackedAt"`     // 确认时间
	ResolvedAt  *time.Time `json:"resolvedAt"`  // 解决时间

	// 处理信息
	AckedBy     string `json:"ackedBy"`     // 确认人
	ResolvedBy  string `json:"resolvedBy"`  // 解决人
	Notes       string `json:"notes"`       // 备注
}

// Acknowledge 确认告警.
func (a *ClusterAlert) Acknowledge(by string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Acknowledged = true
	a.AckedBy = by
	now := time.Now()
	a.AckedAt = &now
}

// Resolve 解决告警.
func (a *ClusterAlert) Resolve(by, notes string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Resolved = true
	a.Active = false
	a.ResolvedBy = by
	a.Notes = notes
	now := time.Now()
	a.ResolvedAt = &now
}

// ClusterTopology 集群拓扑.
type ClusterTopology struct {
	mu sync.RWMutex `json:"-"`

	// 节点列表
	Nodes      []*TopologyNode `json:"nodes"`      // 节点列表
	// 连接列表
	Connections []*TopologyEdge `json:"connections"` // 连接列表
	// 分组列表
	Groups     []*TopologyGroup `json:"groups"`     // 分组列表

	// 统计信息
	UpdatedAt  time.Time `json:"updatedAt"`  // 更新时间
}

// TopologyNode 拓扑节点.
type TopologyNode struct {
	ID       string     `json:"id"`       // 节点ID
	Name     string     `json:"name"`     // 节点名称
	Type     NodeType   `json:"type"`     // 节点类型
	Status   NodeStatus `json:"status"`   // 节点状态
	GroupID  string     `json:"groupId"`  // 所属分组
	X        float64    `json:"x"`        // X坐标（用于可视化）
	Y        float64    `json:"y"`        // Y坐标（用于可视化）
	Metrics  *NodeMetrics `json:"metrics"` // 关键指标
}

// TopologyEdge 拓扑连接.
type TopologyEdge struct {
	SourceID   string  `json:"sourceId"`   // 源节点ID
	TargetID   string  `json:"targetId"`   // 目标节点ID
	Type       string  `json:"type"`       // 连接类型
	Bandwidth  float64 `json:"bandwidth"`  // 带宽（Mbps）
	Latency    float64 `json:"latency"`    // 延迟（ms）
	Active     bool    `json:"active"`     // 是否活跃
}

// TopologyGroup 拓扑分组.
type TopologyGroup struct {
	ID      string   `json:"id"`      // 分组ID
	Name    string   `json:"name"`    // 分组名称
	NodeIDs []string `json:"nodeIds"` // 节点ID列表
	Color   string   `json:"color"`   // 显示颜色
}

// NodeMetrics 节点关键指标.
type NodeMetrics struct {
	CPUUsage    float64 `json:"cpuUsage"`    // CPU使用率
	MemoryUsage float64 `json:"memoryUsage"` // 内存使用率
	DiskUsage   float64 `json:"diskUsage"`   // 磁盘使用率
	NetworkIn   float64 `json:"networkIn"`   // 网络入流量
	NetworkOut  float64 `json:"networkOut"`  // 网络出流量
	Temperature float64 `json:"temperature"` // 温度
}

// ClusterStats 集群统计.
type ClusterStats struct {
	mu sync.RWMutex `json:"-"`

	// 节点统计
	TotalNodes      int `json:"totalNodes"`      // 总节点数
	OnlineNodes     int `json:"onlineNodes"`     // 在线节点数
	OfflineNodes    int `json:"offlineNodes"`    // 离线节点数
	MaintenanceNodes int `json:"maintenanceNodes"` // 维护节点数

	// 资源统计
	TotalCPU        int     `json:"totalCpu"`        // CPU总核数
	UsedCPU         float64 `json:"usedCpu"`         // CPU使用量
	TotalMemory     int64   `json:"totalMemory"`     // 内存总量
	UsedMemory      int64   `json:"usedMemory"`      // 内存使用量
	TotalDisk       int64   `json:"totalDisk"`       // 磁盘总量
	UsedDisk        int64   `json:"usedDisk"`        // 磁盘使用量

	// 任务统计
	TotalTasks      int `json:"totalTasks"`      // 总任务数
	RunningTasks    int `json:"runningTasks"`    // 运行中任务数
	CompletedTasks  int `json:"completedTasks"`  // 已完成任务数
	FailedTasks     int `json:"failedTasks"`     // 失败任务数

	// 告警统计
	ActiveAlerts    int `json:"activeAlerts"`    // 活跃告警数
	CriticalAlerts  int `json:"criticalAlerts"`  // 严重告警数
	WarningAlerts   int `json:"warningAlerts"`   // 警告数

	// 同步统计
	ActiveSyncs     int `json:"activeSyncs"`     // 活跃同步数
	TotalSynced     int64 `json:"totalSynced"`    // 总同步数据量

	// 时间信息
	Uptime          time.Duration `json:"uptime"`   // 运行时间
	StartTime       time.Time     `json:"startTime"` // 启动时间
	UpdatedAt       time.Time     `json:"updatedAt"` // 更新时间
}

// Update 更新统计信息.
func (s *ClusterStats) Update(nodes []*ClusterNode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalNodes = len(nodes)
	s.OnlineNodes = 0
	s.OfflineNodes = 0
	s.MaintenanceNodes = 0
	s.TotalCPU = 0
	s.UsedCPU = 0
	s.TotalMemory = 0
	s.UsedMemory = 0
	s.TotalDisk = 0
	s.UsedDisk = 0

	for _, node := range nodes {
		switch node.Status {
		case NodeStatusOnline:
			s.OnlineNodes++
		case NodeStatusOffline:
			s.OfflineNodes++
		case NodeStatusMaintenance:
			s.MaintenanceNodes++
		}

		s.TotalCPU += node.TotalCPU
		s.TotalMemory += node.TotalMemory
		s.TotalDisk += node.TotalDisk

		if node.Health != nil {
			s.UsedCPU += float64(node.TotalCPU) * node.Health.CPUUsage / 100
			s.UsedMemory += node.Health.MemoryUsed
			s.UsedDisk += node.Health.DiskUsed
		}
	}

	s.UpdatedAt = time.Now()
}

// GetSnapshot 获取统计快照.
func (s *ClusterStats) GetSnapshot() *ClusterStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := *s
	return &snapshot
}

// ClusterConfig 集群配置.
type ClusterConfig struct {
	// 基本配置
	ClusterName      string `json:"clusterName"`      // 集群名称
	ClusterID        string `json:"clusterId"`        // 集群ID

	// 节点发现配置
	AutoDiscovery    bool          `json:"autoDiscovery"`    // 自动发现
	DiscoveryPort    int           `json:"discoveryPort"`    // 发现端口
	DiscoveryInterval time.Duration `json:"discoveryInterval"` // 发现间隔

	// 健康检查配置
	HealthCheckInterval time.Duration `json:"healthCheckInterval"` // 健康检查间隔
	HealthCheckTimeout  time.Duration `json:"healthCheckTimeout"`  // 健康检查超时
	UnhealthyThreshold  int           `json:"unhealthyThreshold"`  // 不健康阈值

	// 心跳配置
	HeartbeatInterval time.Duration `json:"heartbeatInterval"` // 心跳间隔
	HeartbeatTimeout  time.Duration `json:"heartbeatTimeout"`  // 心跳超时

	// 负载均衡配置
	LoadBalanceStrategy LoadBalanceStrategy `json:"loadBalanceStrategy"` // 负载均衡策略
	LoadBalanceThreshold float64            `json:"loadBalanceThreshold"` // 负载阈值

	// 告警配置
	AlertCPUThreshold     float64 `json:"alertCpuThreshold"`     // CPU告警阈值
	AlertMemoryThreshold  float64 `json:"alertMemoryThreshold"`  // 内存告警阈值
	AlertDiskThreshold    float64 `json:"alertDiskThreshold"`    // 磁盘告警阈值
	AlertTempThreshold    float64 `json:"alertTempThreshold"`    // 温度告警阈值

	// 同步配置
	MaxConcurrentSyncs int           `json:"maxConcurrentSyncs"` // 最大并发同步数
	SyncBandwidthLimit int64         `json:"syncBandwidthLimit"` // 同步带宽限制（bytes/s）
	SyncRetryCount     int           `json:"syncRetryCount"`     // 同步重试次数
	SyncRetryInterval  time.Duration `json:"syncRetryInterval"`  // 同步重试间隔

	// 任务配置
	MaxConcurrentTasks int           `json:"maxConcurrentTasks"` // 最大并发任务数
	TaskTimeout        time.Duration `json:"taskTimeout"`        // 任务超时时间
}

// DefaultClusterConfig 返回默认集群配置.
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		ClusterName:         "nas-os-cluster",
		AutoDiscovery:       true,
		DiscoveryPort:       9999,
		DiscoveryInterval:   30 * time.Second,
		HealthCheckInterval: 15 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		UnhealthyThreshold:  3,
		HeartbeatInterval:   10 * time.Second,
		HeartbeatTimeout:    30 * time.Second,
		LoadBalanceStrategy: StrategyCPU,
		LoadBalanceThreshold: 80.0,
		AlertCPUThreshold:   90.0,
		AlertMemoryThreshold: 85.0,
		AlertDiskThreshold:  90.0,
		AlertTempThreshold:  75.0,
		MaxConcurrentSyncs:  5,
		SyncBandwidthLimit:  100 * 1024 * 1024, // 100MB/s
		SyncRetryCount:      3,
		SyncRetryInterval:   30 * time.Second,
		MaxConcurrentTasks:  10,
		TaskTimeout:         30 * time.Minute,
	}
}

// AddNodeRequest 添加节点请求.
type AddNodeRequest struct {
	Name      string            `json:"name"`      // 节点名称
	Hostname  string            `json:"hostname"`  // 主机名
	IPAddress string            `json:"ipAddress"` // IP地址
	Port      int               `json:"port"`      // 端口
	Type      NodeType          `json:"type"`      // 节点类型
	GroupID   string            `json:"groupId"`   // 分组ID
	Tags      []string          `json:"tags"`      // 标签
	Location  string            `json:"location"`  // 位置
	Metadata  map[string]string `json:"metadata"`  // 元数据
}

// AddNodeResponse 添加节点响应.
type AddNodeResponse struct {
	Success bool         `json:"success"` // 是否成功
	Node    *ClusterNode `json:"node"`    // 节点信息
	Message string       `json:"message"` // 消息
}

// UpdateNodeRequest 更新节点请求.
type UpdateNodeRequest struct {
	Name     string   `json:"name,omitempty"`     // 节点名称
	GroupID  string   `json:"groupId,omitempty"`  // 分组ID
	Tags     []string `json:"tags,omitempty"`      // 标签
	Location string   `json:"location,omitempty"` // 位置
	Metadata map[string]string `json:"metadata,omitempty"` // 元数据
}

// CreateGroupRequest 创建分组请求.
type CreateGroupRequest struct {
	Name        string   `json:"name"`        // 分组名称
	Description string   `json:"description"` // 描述
	Type        string   `json:"type"`        // 分组类型
	Tags        []string `json:"tags"`        // 标签
	Priority    int      `json:"priority"`    // 优先级
}

// CreateSyncTaskRequest 创建同步任务请求.
type CreateSyncTaskRequest struct {
	Name         string   `json:"name"`         // 任务名称
	SourceNodeID string   `json:"sourceNodeId"` // 源节点ID
	TargetNodeIDs []string `json:"targetNodeIds"` // 目标节点ID列表
	SourcePath   string   `json:"sourcePath"`   // 源路径
	TargetPath   string   `json:"targetPath"`   // 目标路径
	Interval     int      `json:"interval"`     // 同步间隔（秒）
	AutoSync     bool     `json:"autoSync"`     // 自动同步
	Compress     bool     `json:"compress"`     // 压缩传输
	Encrypt      bool     `json:"encrypt"`      // 加密传输
}

// CreateTaskRequest 创建任务请求.
type CreateTaskRequest struct {
	Name        string                 `json:"name"`        // 任务名称
	Type        string                 `json:"type"`        // 任务类型
	TargetNodeIDs []string             `json:"targetNodeIds"` // 目标节点ID列表
	Priority    int                    `json:"priority"`    // 优先级
	Payload     map[string]interface{} `json:"payload"`     // 任务数据
	Timeout     int                    `json:"timeout"`     // 超时时间（秒）
}

// NodeDiscoveryResult 节点发现结果.
type NodeDiscoveryResult struct {
	IPAddress string   `json:"ipAddress"` // IP地址
	Hostname  string   `json:"hostname"`  // 主机名
	Port      int      `json:"port"`      // 端口
	Version   string   `json:"version"`   // 版本
	Type      NodeType `json:"type"`      // 节点类型
}

// LoadBalanceResult 负载均衡结果.
type LoadBalanceResult struct {
	NodeID    string  `json:"nodeId"`    // 选中的节点ID
	Score     float64 `json:"score"`     // 负载分数
	Strategy  string  `json:"strategy"`  // 使用的策略
}
