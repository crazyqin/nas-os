// Package clustermgr 提供分布式集群管理功能
// 支持节点加入/离开、心跳检测、故障转移、服务发现和负载均衡
package clustermgr

import (
	"sync"
	"time"
)

// NodeStatus 节点状态.
type NodeStatus string

const (
	NodeStatusActive   NodeStatus = "active"   // 活跃状态
	NodeStatusInactive NodeStatus = "inactive" // 不活跃状态
	NodeStatusLeaving  NodeStatus = "leaving"  // 正在离开
	NodeStatusFailed   NodeStatus = "failed"   // 故障状态
	NodeStatusUnknown  NodeStatus = "unknown"  // 未知状态
)

// NodeRole 节点角色.
type NodeRole string

const (
	RoleLeader   NodeRole = "leader"   // 领导者
	RoleFollower NodeRole = "follower" // 跟随者
	RoleCandidate NodeRole = "candidate" // 候选者
	RoleObserver NodeRole = "observer" // 观察者
)

// LoadBalanceStrategy 负载均衡策略.
type LoadBalanceStrategy string

const (
	StrategyRoundRobin  LoadBalanceStrategy = "round_robin"  // 轮询
	StrategyWeighted    LoadBalanceStrategy = "weighted"      // 加权
	StrategyLeastConn   LoadBalanceStrategy = "least_conn"   // 最少连接
	StrategyIPHash      LoadBalanceStrategy = "ip_hash"      // IP哈希
	StrategyRandom      LoadBalanceStrategy = "random"       // 随机
)

// ClusterStatus 集群状态.
type ClusterStatus string

const (
	ClusterStatusHealthy  ClusterStatus = "healthy"  // 健康状态
	ClusterStatusDegraded ClusterStatus = "degraded" // 降级状态
	ClusterStatusCritical ClusterStatus = "critical" // 严重状态
	ClusterStatusUnknown  ClusterStatus = "unknown"  // 未知状态
)

// ServiceProtocol 服务协议.
type ServiceProtocol string

const (
	ProtocolHTTP  ServiceProtocol = "http"  // HTTP协议
	ProtocolGRPC  ServiceProtocol = "grpc"  // gRPC协议
	ProtocolTCP   ServiceProtocol = "tcp"   // TCP协议
	ProtocolUDP   ServiceProtocol = "udp"   // UDP协议
)

// Node 节点信息.
type Node struct {
	mu sync.RWMutex `json:"-"`

	ID          string     `json:"id"`          // 节点ID
	Name        string     `json:"name"`        // 节点名称
	Address     string     `json:"address"`     // 节点地址（IP:Port）
	Role        NodeRole   `json:"role"`        // 节点角色
	Status      NodeStatus `json:"status"`      // 节点状态
	Weight      int        `json:"weight"`      // 权重（用于加权负载均衡）
	Zone        string     `json:"zone"`        // 可用区
	Tags        []string   `json:"tags"`        // 标签

	// 连接统计
	Connections int        `json:"connections"` // 当前连接数
	MaxConns    int        `json:"maxConns"`    // 最大连接数
	ActiveConns int        `json:"activeConns"` // 活跃连接数

	// 性能指标
	CPUUsage    float64    `json:"cpuUsage"`    // CPU使用率
	MemoryUsage float64    `json:"memoryUsage"` // 内存使用率
	DiskUsage   float64    `json:"diskUsage"`   // 磁盘使用率
	LoadAvg     float64    `json:"loadAvg"`     // 平均负载

	// 时间信息
	LastHeartbeat time.Time `json:"lastHeartbeat"` // 最后心跳时间
	JoinTime      time.Time `json:"joinTime"`      // 加入时间
	LeaveTime     time.Time `json:"leaveTime"`     // 离开时间

	// 元数据
	Metadata map[string]string `json:"metadata,omitempty"` // 自定义元数据
}

// IsAlive 检查节点是否存活（心跳超时判断）.
func (n *Node) IsAlive(timeout time.Duration) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return time.Since(n.LastHeartbeat) < timeout
}

// UpdateHeartbeat 更新心跳时间.
func (n *Node) UpdateHeartbeat() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.LastHeartbeat = time.Now()
	n.Status = NodeStatusActive
}

// UpdateMetrics 更新性能指标.
func (n *Node) UpdateMetrics(cpu, memory, disk, load float64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.CPUUsage = cpu
	n.MemoryUsage = memory
	n.DiskUsage = disk
	n.LoadAvg = load
}

// Cluster 集群信息.
type Cluster struct {
	mu sync.RWMutex `json:"-"`

	ID          string        `json:"id"`          // 集群ID
	Name        string        `json:"name"`        // 集群名称
	Status      ClusterStatus `json:"status"`      // 集群状态
	LeaderID    string        `json:"leaderId"`    // 领导者节点ID
	Version     int64         `json:"version"`     // 集群版本号
	CreatedAt   time.Time     `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time     `json:"updatedAt"`   // 更新时间

	// 配置信息
	Config      ClusterConfig `json:"config"`      // 集群配置

	// 节点列表
	Nodes       map[string]*Node `json:"nodes"`   // 节点映射（ID -> Node）
}

// GetNode 获取节点.
func (c *Cluster) GetNode(id string) (*Node, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	node, ok := c.Nodes[id]
	return node, ok
}

// AddNode 添加节点.
func (c *Cluster) AddNode(node *Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Nodes == nil {
		c.Nodes = make(map[string]*Node)
	}
	c.Nodes[node.ID] = node
	c.Version++
	c.UpdatedAt = time.Now()
}

// RemoveNode 移除节点.
func (c *Cluster) RemoveNode(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.Nodes[id]; ok {
		delete(c.Nodes, id)
		c.Version++
		c.UpdatedAt = time.Now()
		return true
	}
	return false
}

// GetActiveNodes 获取活跃节点列表.
func (c *Cluster) GetActiveNodes() []*Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var nodes []*Node
	for _, node := range c.Nodes {
		if node.Status == NodeStatusActive {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetNodesByRole 按角色获取节点.
func (c *Cluster) GetNodesByRole(role NodeRole) []*Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var nodes []*Node
	for _, node := range c.Nodes {
		if node.Role == role {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// UpdateStatus 更新集群状态.
func (c *Cluster) UpdateStatus() {
	c.mu.Lock()
	defer c.mu.Unlock()

	totalNodes := len(c.Nodes)
	if totalNodes == 0 {
		c.Status = ClusterStatusUnknown
		return
	}

	activeNodes := 0
	for _, node := range c.Nodes {
		if node.Status == NodeStatusActive {
			activeNodes++
		}
	}

	// 根据活跃节点比例判断集群状态
	activeRatio := float64(activeNodes) / float64(totalNodes)
	switch {
	case activeRatio >= 0.8:
		c.Status = ClusterStatusHealthy
	case activeRatio >= 0.5:
		c.Status = ClusterStatusDegraded
	default:
		c.Status = ClusterStatusCritical
	}
	c.UpdatedAt = time.Now()
}

// ClusterConfig 集群配置.
type ClusterConfig struct {
	// 心跳配置
	HeartbeatInterval time.Duration `json:"heartbeatInterval"` // 心跳间隔
	HeartbeatTimeout  time.Duration `json:"heartbeatTimeout"`  // 心跳超时

	// 故障转移配置
	FailoverEnabled   bool          `json:"failoverEnabled"`   // 启用故障转移
	FailoverTimeout   time.Duration `json:"failoverTimeout"`   // 故障转移超时
	MaxFailoverAttempts int         `json:"maxFailoverAttempts"` // 最大故障转移尝试次数

	// 负载均衡配置
	LoadBalanceStrategy LoadBalanceStrategy `json:"loadBalanceStrategy"` // 负载均衡策略
	StickySession       bool                `json:"stickySession"`       // 会话保持

	// 服务发现配置
	DiscoveryEnabled    bool          `json:"discoveryEnabled"`    // 启用服务发现
	DiscoveryInterval   time.Duration `json:"discoveryInterval"`   // 发现间隔
	HealthCheckInterval time.Duration `json:"healthCheckInterval"` // 健康检查间隔

	// 节点配置
	MaxNodes            int  `json:"maxNodes"`            // 最大节点数
	AutoRemoveFailed    bool `json:"autoRemoveFailed"`    // 自动移除故障节点
	FailedNodeTimeout   time.Duration `json:"failedNodeTimeout"` // 故障节点超时时间
}

// DefaultClusterConfig 返回默认集群配置.
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		HeartbeatInterval:   10 * time.Second,
		HeartbeatTimeout:    30 * time.Second,
		FailoverEnabled:     true,
		FailoverTimeout:     60 * time.Second,
		MaxFailoverAttempts: 3,
		LoadBalanceStrategy: StrategyRoundRobin,
		StickySession:       false,
		DiscoveryEnabled:    true,
		DiscoveryInterval:   30 * time.Second,
		HealthCheckInterval: 15 * time.Second,
		MaxNodes:            100,
		AutoRemoveFailed:    true,
		FailedNodeTimeout:   5 * time.Minute,
	}
}

// ServiceInfo 服务信息.
type ServiceInfo struct {
	mu sync.RWMutex `json:"-"`

	ID          string          `json:"id"`          // 服务ID
	Name        string          `json:"name"`        // 服务名称
	Version     string          `json:"version"`     // 服务版本
	Protocol    ServiceProtocol `json:"protocol"`    // 服务协议
	Address     string          `json:"address"`     // 服务地址
	Port        int             `json:"port"`        // 服务端口
	Tags        []string        `json:"tags"`        // 服务标签
	Metadata    map[string]string `json:"metadata,omitempty"` // 服务元数据

	// 健康状态
	Healthy     bool      `json:"healthy"`     // 是否健康
	LastCheck   time.Time `json:"lastCheck"`   // 最后检查时间
	CheckCount  int       `json:"checkCount"`  // 检查次数
	FailCount   int       `json:"failCount"`   // 失败次数

	// 注册信息
	RegisteredAt time.Time `json:"registeredAt"` // 注册时间
	ExpiresAt    time.Time `json:"expiresAt"`    // 过期时间
	NodeID       string    `json:"nodeId"`       // 所属节点ID
}

// IsExpired 检查服务是否过期.
func (s *ServiceInfo) IsExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Now().After(s.ExpiresAt)
}

// UpdateHealth 更新健康状态.
func (s *ServiceInfo) UpdateHealth(healthy bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Healthy = healthy
	s.LastCheck = time.Now()
	s.CheckCount++
	if !healthy {
		s.FailCount++
	}
}

// ServiceDiscovery 服务发现信息.
type ServiceDiscovery struct {
	mu sync.RWMutex `json:"-"`

	Services map[string]*ServiceInfo `json:"services"` // 服务映射（ID -> ServiceInfo）
	Index    uint64                  `json:"index"`    // 索引（用于变更检测）
}

// RegisterService 注册服务.
func (sd *ServiceDiscovery) RegisterService(service *ServiceInfo) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if sd.Services == nil {
		sd.Services = make(map[string]*ServiceInfo)
	}
	sd.Services[service.ID] = service
	sd.Index++
}

// DeregisterService 注销服务.
func (sd *ServiceDiscovery) DeregisterService(id string) bool {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if _, ok := sd.Services[id]; ok {
		delete(sd.Services, id)
		sd.Index++
		return true
	}
	return false
}

// GetService 获取服务.
func (sd *ServiceDiscovery) GetService(id string) (*ServiceInfo, bool) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	service, ok := sd.Services[id]
	return service, ok
}

// GetServicesByName 按名称获取服务列表.
func (sd *ServiceDiscovery) GetServicesByName(name string) []*ServiceInfo {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	var services []*ServiceInfo
	for _, service := range sd.Services {
		if service.Name == name && service.Healthy {
			services = append(services, service)
		}
	}
	return services
}

// GetHealthyServices 获取健康的服务列表.
func (sd *ServiceDiscovery) GetHealthyServices() []*ServiceInfo {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	var services []*ServiceInfo
	for _, service := range sd.Services {
		if service.Healthy && !service.IsExpired() {
			services = append(services, service)
		}
	}
	return services
}

// HeartbeatRequest 心跳请求.
type HeartbeatRequest struct {
	NodeID    string  `json:"nodeId"`    // 节点ID
	CPUUsage  float64 `json:"cpuUsage"`  // CPU使用率
	MemoryUsage float64 `json:"memoryUsage"` // 内存使用率
	DiskUsage float64 `json:"diskUsage"` // 磁盘使用率
	LoadAvg   float64 `json:"loadAvg"`   // 平均负载
	Connections int   `json:"connections"` // 当前连接数
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// HeartbeatResponse 心跳响应.
type HeartbeatResponse struct {
	Success   bool   `json:"success"`   // 是否成功
	LeaderID  string `json:"leaderId"`  // 领导者ID
	Version   int64  `json:"version"`   // 集群版本
	Message   string `json:"message"`   // 消息
}

// JoinRequest 加入集群请求.
type JoinRequest struct {
	NodeID    string            `json:"nodeId"`    // 节点ID
	Name      string            `json:"name"`      // 节点名称
	Address   string            `json:"address"`   // 节点地址
	Zone      string            `json:"zone"`      // 可用区
	Tags      []string          `json:"tags"`      // 标签
	Weight    int               `json:"weight"`    // 权重
	MaxConns  int               `json:"maxConns"`  // 最大连接数
	Metadata  map[string]string `json:"metadata"`  // 元数据
}

// JoinResponse 加入集群响应.
type JoinResponse struct {
	Success    bool          `json:"success"`    // 是否成功
	ClusterID  string        `json:"clusterId"`  // 集群ID
	LeaderID   string        `json:"leaderId"`   // 领导者ID
	Nodes      []*Node       `json:"nodes"`      // 节点列表
	Config     ClusterConfig `json:"config"`     // 集群配置
	Message    string        `json:"message"`    // 消息
}

// LeaveRequest 离开集群请求.
type LeaveRequest struct {
	NodeID  string `json:"nodeId"`  // 节ID
	Reason  string `json:"reason"`  // 离开原因
}

// LeaveResponse 离开集群响应.
type LeaveResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}

// ClusterStats 集群统计.
type ClusterStats struct {
	mu sync.RWMutex `json:"-"`

	// 节点统计
	TotalNodes   int `json:"totalNodes"`   // 总节点数
	ActiveNodes  int `json:"activeNodes"`  // 活跃节点数
	FailedNodes  int `json:"failedNodes"`  // 故障节点数

	// 请求统计
	TotalRequests   int64 `json:"totalRequests"`   // 总请求数
	SuccessRequests int64 `json:"successRequests"` // 成功请求数
	FailedRequests  int64 `json:"failedRequests"`  // 失败请求数

	// 负载统计
	TotalConnections int `json:"totalConnections"` // 总连接数
	AvgResponseTime  float64 `json:"avgResponseTime"` // 平均响应时间（毫秒）

	// 时间统计
	LastFailoverTime time.Time     `json:"lastFailoverTime"` // 最后故障转移时间
	Uptime           time.Duration `json:"uptime"`           // 运行时间
	StartTime        time.Time     `json:"startTime"`        // 启动时间
}

// Update 更新统计信息.
func (s *ClusterStats) Update(activeNodes, failedNodes, totalConns int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveNodes = activeNodes
	s.FailedNodes = failedNodes
	s.TotalNodes = activeNodes + failedNodes
	s.TotalConnections = totalConns
}

// IncrRequest 增加请求计数.
func (s *ClusterStats) IncrRequest(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalRequests++
	if success {
		s.SuccessRequests++
	} else {
		s.FailedRequests++
	}
}

// GetSnapshot 获取统计快照（线程安全）.
func (s *ClusterStats) GetSnapshot() *ClusterStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ClusterStats{
		TotalNodes:       s.TotalNodes,
		ActiveNodes:      s.ActiveNodes,
		FailedNodes:      s.FailedNodes,
		TotalRequests:    s.TotalRequests,
		SuccessRequests:  s.SuccessRequests,
		FailedRequests:   s.FailedRequests,
		TotalConnections: s.TotalConnections,
		AvgResponseTime:  s.AvgResponseTime,
		LastFailoverTime: s.LastFailoverTime,
		Uptime:           time.Since(s.StartTime),
		StartTime:        s.StartTime,
	}
}
