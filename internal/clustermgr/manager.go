// Package clustermgr 提供集群管理与负载均衡能力
// 对标群晖 Cluster Manager，支持多节点管理、负载均衡、故障转移
package clustermgr

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager 集群管理器
type Manager struct {
	mu        sync.RWMutex
	config    *Config
	nodes     map[string]*Node
	scheduler *Scheduler
	monitor   *Monitor
	logger    Logger
}

// Config 配置
type Config struct {
	HeartbeatInterval    time.Duration // 心跳间隔
	FailoverTimeout      time.Duration // 故障转移超时
	MaxNodes             int           // 最大节点数
	EnableAutoFailover   bool          // 启用自动故障转移
	LoadBalanceAlgorithm string        // 负载均衡算法
}

// Node 节点信息
type Node struct {
	ID           string
	Name         string
	Address      string
	Status       NodeStatus
	Role         NodeRole
	CPU          float64
	Memory       float64
	Disk         float64
	Network      float64
	Services     []string
	LastHeartbeat time.Time
	JoinedAt     time.Time
}

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusDegraded NodeStatus = "degraded"
	NodeStatusMaintenance NodeStatus = "maintenance"
)

// NodeRole 节点角色
type NodeRole string

const (
	NodeRoleMaster  NodeRole = "master"
	NodeRoleWorker  NodeRole = "worker"
	NodeRoleStorage NodeRole = "storage"
)

// Scheduler 调度器
type Scheduler struct {
	mu        sync.RWMutex
	algorithm string
	rules     []ScheduleRule
}

// ScheduleRule 调度规则
type ScheduleRule struct {
	Name      string
	Priority  int
	Condition func(*Node, *Task) bool
	Action    func(*Node, *Task) error
}

// Task 任务
type Task struct {
	ID        string
	Type      string
	Priority  int
	Resources ResourceRequest
	Status    TaskStatus
	NodeID    string
	CreatedAt time.Time
}

// ResourceRequest 资源请求
type ResourceRequest struct {
	CPU    float64
	Memory int64
	Disk   int64
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Monitor 监控器
type Monitor struct {
	mu       sync.RWMutex
	alerts   []Alert
	metrics  map[string]*Metric
}

// Alert 告警
type Alert struct {
	ID        string
	NodeID    string
	Type      AlertType
	Message   string
	Severity  AlertSeverity
	CreatedAt time.Time
	Resolved  bool
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeCPU     AlertType = "cpu"
	AlertTypeMemory  AlertType = "memory"
	AlertTypeDisk    AlertType = "disk"
	AlertTypeNetwork AlertType = "network"
	AlertTypeNodeDown AlertType = "node_down"
)

// AlertSeverity 告警级别
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Metric 指标
type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Labels    map[string]string
}

// ClusterStatus 集群状态
type ClusterStatus struct {
	TotalNodes    int
	OnlineNodes   int
	OfflineNodes  int
	TotalCPU      float64
	TotalMemory   float64
	TotalDisk     float64
	ActiveTasks   int
	PendingTasks  int
	Alerts        []Alert
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建新的集群管理器
func NewManager(config *Config, logger Logger) *Manager {
	return &Manager{
		config: config,
		nodes:  make(map[string]*Node),
		scheduler: &Scheduler{
			algorithm: config.LoadBalanceAlgorithm,
			rules:     []ScheduleRule{},
		},
		monitor: &Monitor{
			alerts:  []Alert{},
			metrics: make(map[string]*Metric),
		},
		logger: logger,
	}
}

// Start 启动集群管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 启动心跳监控
	go m.heartbeatMonitor(ctx)

	// 启动负载监控
	go m.loadMonitor(ctx)

	// 启动故障检测
	go m.failureDetector(ctx)

	m.logger.Info("集群管理器已启动")
	return nil
}

// AddNode 添加节点
func (m *Manager) AddNode(node *Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.nodes) >= m.config.MaxNodes {
		return fmt.Errorf("已达到最大节点数: %d", m.config.MaxNodes)
	}

	if _, exists := m.nodes[node.ID]; exists {
		return fmt.Errorf("节点已存在: %s", node.ID)
	}

	node.Status = NodeStatusOnline
	node.JoinedAt = time.Now()
	node.LastHeartbeat = time.Now()
	m.nodes[node.ID] = node

	m.logger.Info("节点已添加: %s (%s)", node.Name, node.Address)
	return nil
}

// RemoveNode 移除节点
func (m *Manager) RemoveNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点不存在: %s", nodeID)
	}

	// 迁移服务
	m.migrateServices(node)

	delete(m.nodes, nodeID)
	m.logger.Info("节点已移除: %s", node.Name)
	return nil
}

// GetClusterStatus 获取集群状态
func (m *Manager) GetClusterStatus() *ClusterStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &ClusterStatus{
		TotalNodes: len(m.nodes),
	}

	for _, node := range m.nodes {
		switch node.Status {
		case NodeStatusOnline:
			status.OnlineNodes++
		case NodeStatusOffline:
			status.OfflineNodes++
		}

		status.TotalCPU += node.CPU
		status.TotalMemory += node.Memory
		status.TotalDisk += node.Disk
	}

	status.Alerts = m.monitor.alerts
	return status
}

// ScheduleTask 调度任务
func (m *Manager) ScheduleTask(task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 选择最佳节点
	node := m.selectBestNode(task)
	if node == nil {
		return fmt.Errorf("没有可用节点")
	}

	task.NodeID = node.ID
	task.Status = TaskStatusRunning

	m.logger.Info("任务已调度: %s -> %s", task.ID, node.Name)
	return nil
}

// selectBestNode 选择最佳节点
func (m *Manager) selectBestNode(task *Task) *Node {
	var bestNode *Node
	var bestScore float64

	for _, node := range m.nodes {
		if node.Status != NodeStatusOnline {
			continue
		}

		// 计算节点得分
		score := m.calculateNodeScore(node, task)
		if bestNode == nil || score > bestScore {
			bestNode = node
			bestScore = score
		}
	}

	return bestNode
}

// calculateNodeScore 计算节点得分
func (m *Manager) calculateNodeScore(node *Node, task *Task) float64 {
	// 基于负载的得分
	cpuScore := 100 - node.CPU
	memoryScore := 100 - node.Memory
	diskScore := 100 - node.Disk

	// 加权平均
	return (cpuScore*0.4 + memoryScore*0.3 + diskScore*0.3)
}

// heartbeatMonitor 心跳监控
func (m *Manager) heartbeatMonitor(ctx context.Context) {
	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkHeartbeats()
		}
	}
}

// checkHeartbeats 检查心跳
func (m *Manager) checkHeartbeats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, node := range m.nodes {
		if time.Since(node.LastHeartbeat) > m.config.HeartbeatInterval*3 {
			if node.Status == NodeStatusOnline {
				node.Status = NodeStatusOffline
				m.addAlert(node.ID, AlertTypeNodeDown, fmt.Sprintf("节点 %s 离线", node.Name), AlertSeverityCritical)

				if m.config.EnableAutoFailover {
					go m.performFailover(node)
				}
			}
		}
	}
}

// loadMonitor 负载监控
func (m *Manager) loadMonitor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// collectMetrics 收集指标
func (m *Manager) collectMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, node := range m.nodes {
		// 检查CPU
		if node.CPU > 90 {
			m.addAlert(node.ID, AlertTypeCPU, fmt.Sprintf("节点 %s CPU使用率过高: %.1f%%", node.Name, node.CPU), AlertSeverityWarning)
		}

		// 检查内存
		if node.Memory > 90 {
			m.addAlert(node.ID, AlertTypeMemory, fmt.Sprintf("节点 %s 内存使用率过高: %.1f%%", node.Name, node.Memory), AlertSeverityWarning)
		}

		// 检查磁盘
		if node.Disk > 90 {
			m.addAlert(node.ID, AlertTypeDisk, fmt.Sprintf("节点 %s 磁盘使用率过高: %.1f%%", node.Name, node.Disk), AlertSeverityWarning)
		}
	}
}

// failureDetector 故障检测
func (m *Manager) failureDetector(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.detectFailures()
		}
	}
}

// detectFailures 检测故障
func (m *Manager) detectFailures() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, node := range m.nodes {
		if node.Status == NodeStatusOffline {
			m.logger.Warn("节点离线: %s", node.Name)
		}
	}
}

// performFailover 执行故障转移
func (m *Manager) performFailover(failedNode *Node) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("执行故障转移: %s", failedNode.Name)

	// 迁移服务到其他节点
	for _, service := range failedNode.Services {
		m.migrateService(service, failedNode.ID)
	}
}

// migrateServices 迁移服务
func (m *Manager) migrateServices(fromNode *Node) {
	for _, service := range fromNode.Services {
		m.migrateService(service, fromNode.ID)
	}
}

// migrateService 迁移单个服务
func (m *Manager) migrateService(service string, fromNodeID string) {
	// 找到目标节点
	for _, node := range m.nodes {
		if node.ID != fromNodeID && node.Status == NodeStatusOnline {
			node.Services = append(node.Services, service)
			m.logger.Info("服务 %s 已从 %s 迁移到 %s", service, fromNodeID, node.ID)
			return
		}
	}
	m.logger.Error("无法迁移服务 %s: 没有可用节点", service)
}

// addAlert 添加告警
func (m *Manager) addAlert(nodeID string, alertType AlertType, message string, severity AlertSeverity) {
	alert := Alert{
		ID:        fmt.Sprintf("alert_%d", len(m.monitor.alerts)+1),
		NodeID:    nodeID,
		Type:      alertType,
		Message:   message,
		Severity:  severity,
		CreatedAt: time.Now(),
	}
	m.monitor.alerts = append(m.monitor.alerts, alert)
	m.logger.Warn("告警: %s", message)
}
