package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 边缘节点类型
const (
	EdgeNodeTypeGateway  = "gateway"  // 网关节点
	EdgeNodeTypeCompute  = "compute"  // 计算节点
	EdgeNodeTypeStorage  = "storage"  // 存储节点
	EdgeNodeTypeSensor   = "sensor"   // 传感器节点
	EdgeNodeTypeActuator = "actuator" // 执行器节点
)

// 边缘节点状态
const (
	EdgeNodeStatusOnline   = "online"
	EdgeNodeStatusOffline  = "offline"
	EdgeNodeStatusBusy     = "busy"
	EdgeNodeStatusIdle     = "idle"
	EdgeNodeStatusMaintain = "maintain"
)

// 边缘节点能力标志
const (
	EdgeCapCompute  = 1 << iota // 计算能力
	EdgeCapStorage              // 存储能力
	EdgeCapNetwork              // 网络能力
	EdgeCapSensor               // 传感器能力
	EdgeCapActuator             // 执行器能力
	EdgeCapAI                   // AI 推理能力
	EdgeCapGPU                  // GPU 加速能力
)

// EdgeNodeCapabilities 边缘节点能力
type EdgeNodeCapabilities struct {
	CPU     int      `json:"cpu"`     // CPU 核心数
	Memory  int64    `json:"memory"`  // 内存大小 (MB)
	Storage int64    `json:"storage"` // 存储大小 (GB)
	GPU     bool     `json:"gpu"`     // 是否有 GPU
	AI      bool     `json:"ai"`      // 是否支持 AI 推理
	Caps    uint32   `json:"caps"`    // 能力位图
	Tags    []string `json:"tags"`    // 标签
}

// EdgeNodeResource 边缘节点资源使用情况
type EdgeNodeResource struct {
	CPUUsed     float64 `json:"cpu_used"`     // CPU 使用率
	MemoryUsed  float64 `json:"memory_used"`  // 内存使用率
	StorageUsed float64 `json:"storage_used"` // 存储使用率
	NetworkRx   int64   `json:"network_rx"`   // 网络接收 (bytes/sec)
	NetworkTx   int64   `json:"network_tx"`   // 网络发送 (bytes/sec)
	GPUUsed     float64 `json:"gpu_used"`     // GPU 使用率
	Temperature float64 `json:"temperature"`  // 温度
	PowerUsage  float64 `json:"power_usage"`  // 功耗
}

// EdgeNode 边缘节点
type EdgeNode struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Type         string               `json:"type"`
	IPAddress    string               `json:"ip_address"`
	Port         int                  `json:"port"`
	Status       string               `json:"status"`
	Capabilities EdgeNodeCapabilities `json:"capabilities"`
	Resources    EdgeNodeResource     `json:"resources"`
	Location     EdgeLocation         `json:"location"`
	Priority     int                  `json:"priority"` // 优先级 (越高越优先)
	Weight       int                  `json:"weight"`   // 权重 (负载均衡用)
	LastSeen     time.Time            `json:"last_seen"`
	JoinTime     time.Time            `json:"join_time"`
	TasksRunning int                  `json:"tasks_running"`
	TasksQueued  int                  `json:"tasks_queued"`
	Labels       map[string]string    `json:"labels"`
	Annotations  map[string]string    `json:"annotations"`
}

// EdgeLocation 边缘节点位置信息
type EdgeLocation struct {
	Region     string  `json:"region"`     // 区域
	Zone       string  `json:"zone"`       // 可用区
	Datacenter string  `json:"datacenter"` // 数据中心
	Rack       string  `json:"rack"`       // 机架
	Latitude   float64 `json:"latitude"`   // 纬度
	Longitude  float64 `json:"longitude"`  // 经度
}

// EdgeNodeConfig 边缘节点配置
type EdgeNodeConfig struct {
	NodeID            string        `json:"node_id"`
	DataDir           string        `json:"data_dir"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout"`
	MaxNodes          int           `json:"max_nodes"`
	AutoRegister      bool          `json:"auto_register"`
}

// EdgeNodeManager 边缘节点管理器
type EdgeNodeManager struct {
	config     EdgeNodeConfig
	nodes      map[string]*EdgeNode
	nodesMutex sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *zap.Logger
	cluster    *ClusterManager
	callbacks  EdgeNodeCallbacks
	taskQueue  *TaskScheduler
}

// EdgeNodeCallbacks 边缘节点事件回调
type EdgeNodeCallbacks struct {
	OnNodeJoin   func(node *EdgeNode)
	OnNodeLeave  func(node *EdgeNode)
	OnNodeStatus func(node *EdgeNode, oldStatus string)
	OnNodeUpdate func(node *EdgeNode)
}

// NewEdgeNodeManager 创建边缘节点管理器
func NewEdgeNodeManager(config EdgeNodeConfig, logger *zap.Logger, cluster *ClusterManager) (*EdgeNodeManager, error) {
	if config.NodeID == "" {
		hostname, _ := os.Hostname()
		config.NodeID = fmt.Sprintf("edge-%s", hostname)
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/edge"
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 10 * time.Second
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 30 * time.Second
	}
	if config.MaxNodes == 0 {
		config.MaxNodes = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	enm := &EdgeNodeManager{
		config:  config,
		nodes:   make(map[string]*EdgeNode),
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		cluster: cluster,
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("创建边缘数据目录失败：%w", err)
	}

	// 加载持久化节点
	if err := enm.loadNodes(); err != nil {
		logger.Warn("加载边缘节点失败", zap.Error(err))
	}

	return enm, nil
}

// Initialize 初始化边缘节点管理器
func (enm *EdgeNodeManager) Initialize() error {
	enm.logger.Info("初始化边缘节点管理器", zap.String("node_id", enm.config.NodeID))

	// 启动心跳检测
	go enm.heartbeatWorker()

	// 启动状态监控
	go enm.statusMonitorWorker()

	enm.logger.Info("边缘节点管理器初始化完成")
	return nil
}

// RegisterNode 注册边缘节点
func (enm *EdgeNodeManager) RegisterNode(node *EdgeNode) error {
	enm.nodesMutex.Lock()
	defer enm.nodesMutex.Unlock()

	// 检查节点数量限制
	if len(enm.nodes) >= enm.config.MaxNodes {
		return fmt.Errorf("已达到最大节点数限制：%d", enm.config.MaxNodes)
	}

	// 设置默认值
	if node.Status == "" {
		node.Status = EdgeNodeStatusOnline
	}
	if node.JoinTime.IsZero() {
		node.JoinTime = time.Now()
	}
	node.LastSeen = time.Now()

	enm.nodes[node.ID] = node
	enm.logger.Info("注册边缘节点",
		zap.String("node_id", node.ID),
		zap.String("type", node.Type),
		zap.String("ip", node.IPAddress))

	// 触发回调
	if enm.callbacks.OnNodeJoin != nil {
		go enm.callbacks.OnNodeJoin(node)
	}

	// 持久化
	return enm.saveNodes()
}

// UnregisterNode 注销边缘节点
func (enm *EdgeNodeManager) UnregisterNode(nodeID string) error {
	enm.nodesMutex.Lock()
	defer enm.nodesMutex.Unlock()

	node, exists := enm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("边缘节点不存在：%s", nodeID)
	}

	delete(enm.nodes, nodeID)
	enm.logger.Info("注销边缘节点", zap.String("node_id", nodeID))

	// 触发回调
	if enm.callbacks.OnNodeLeave != nil {
		go enm.callbacks.OnNodeLeave(node)
	}

	return enm.saveNodes()
}

// GetNode 获取边缘节点
func (enm *EdgeNodeManager) GetNode(nodeID string) (*EdgeNode, bool) {
	enm.nodesMutex.RLock()
	defer enm.nodesMutex.RUnlock()

	node, exists := enm.nodes[nodeID]
	return node, exists
}

// GetNodes 获取所有边缘节点
func (enm *EdgeNodeManager) GetNodes() []*EdgeNode {
	enm.nodesMutex.RLock()
	defer enm.nodesMutex.RUnlock()

	nodes := make([]*EdgeNode, 0, len(enm.nodes))
	for _, node := range enm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetNodesByType 按类型获取边缘节点
func (enm *EdgeNodeManager) GetNodesByType(nodeType string) []*EdgeNode {
	enm.nodesMutex.RLock()
	defer enm.nodesMutex.RUnlock()

	nodes := make([]*EdgeNode, 0)
	for _, node := range enm.nodes {
		if node.Type == nodeType {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetNodesByCapability 按能力获取边缘节点
func (enm *EdgeNodeManager) GetNodesByCapability(cap uint32) []*EdgeNode {
	enm.nodesMutex.RLock()
	defer enm.nodesMutex.RUnlock()

	nodes := make([]*EdgeNode, 0)
	for _, node := range enm.nodes {
		if node.Capabilities.Caps&cap != 0 {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetOnlineNodes 获取在线边缘节点
func (enm *EdgeNodeManager) GetOnlineNodes() []*EdgeNode {
	enm.nodesMutex.RLock()
	defer enm.nodesMutex.RUnlock()

	nodes := make([]*EdgeNode, 0)
	for _, node := range enm.nodes {
		if node.Status == EdgeNodeStatusOnline || node.Status == EdgeNodeStatusIdle {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetAvailableNodes 获取可用边缘节点（在线且非忙碌）
func (enm *EdgeNodeManager) GetAvailableNodes() []*EdgeNode {
	enm.nodesMutex.RLock()
	defer enm.nodesMutex.RUnlock()

	nodes := make([]*EdgeNode, 0)
	for _, node := range enm.nodes {
		if node.Status == EdgeNodeStatusIdle || node.Status == EdgeNodeStatusOnline {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// UpdateNodeStatus 更新边缘节点状态
func (enm *EdgeNodeManager) UpdateNodeStatus(nodeID string, status string) error {
	enm.nodesMutex.Lock()
	defer enm.nodesMutex.Unlock()

	node, exists := enm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("边缘节点不存在：%s", nodeID)
	}

	oldStatus := node.Status
	node.Status = status
	node.LastSeen = time.Now()

	// 触发回调
	if enm.callbacks.OnNodeStatus != nil && oldStatus != status {
		go enm.callbacks.OnNodeStatus(node, oldStatus)
	}

	return nil
}

// UpdateNodeResources 更新边缘节点资源
func (enm *EdgeNodeManager) UpdateNodeResources(nodeID string, resources EdgeNodeResource) error {
	enm.nodesMutex.Lock()
	defer enm.nodesMutex.Unlock()

	node, exists := enm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("边缘节点不存在：%s", nodeID)
	}

	node.Resources = resources
	node.LastSeen = time.Now()

	// 触发回调
	if enm.callbacks.OnNodeUpdate != nil {
		go enm.callbacks.OnNodeUpdate(node)
	}

	return nil
}

// UpdateHeartbeat 更新心跳
func (enm *EdgeNodeManager) UpdateHeartbeat(nodeID string) error {
	enm.nodesMutex.Lock()
	defer enm.nodesMutex.Unlock()

	node, exists := enm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("边缘节点不存在：%s", nodeID)
	}

	node.LastSeen = time.Now()
	if node.Status == EdgeNodeStatusOffline {
		node.Status = EdgeNodeStatusOnline
		if enm.callbacks.OnNodeStatus != nil {
			go enm.callbacks.OnNodeStatus(node, EdgeNodeStatusOffline)
		}
	}

	return nil
}

// SetTaskScheduler 设置任务调度器
func (enm *EdgeNodeManager) SetTaskScheduler(scheduler *TaskScheduler) {
	enm.taskQueue = scheduler
}

// SetCallbacks 设置事件回调
func (enm *EdgeNodeManager) SetCallbacks(callbacks EdgeNodeCallbacks) {
	enm.callbacks = callbacks
}

// heartbeatWorker 心跳检测工作线程
func (enm *EdgeNodeManager) heartbeatWorker() {
	ticker := time.NewTicker(enm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-enm.ctx.Done():
			return
		case <-ticker.C:
			enm.checkNodeHeartbeats()
		}
	}
}

// checkNodeHeartbeats 检查节点心跳
func (enm *EdgeNodeManager) checkNodeHeartbeats() {
	enm.nodesMutex.Lock()
	defer enm.nodesMutex.Unlock()

	now := time.Now()
	for _, node := range enm.nodes {
		if node.Status == EdgeNodeStatusOffline {
			continue
		}

		if now.Sub(node.LastSeen) > enm.config.HeartbeatTimeout {
			enm.logger.Warn("边缘节点心跳超时",
				zap.String("node_id", node.ID),
				zap.Duration("elapsed", now.Sub(node.LastSeen)))

			oldStatus := node.Status
			node.Status = EdgeNodeStatusOffline

			if enm.callbacks.OnNodeStatus != nil {
				go enm.callbacks.OnNodeStatus(node, oldStatus)
			}
		}
	}
}

// statusMonitorWorker 状态监控工作线程
func (enm *EdgeNodeManager) statusMonitorWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-enm.ctx.Done():
			return
		case <-ticker.C:
			enm.updateNodeStatus()
		}
	}
}

// updateNodeStatus 更新节点状态
func (enm *EdgeNodeManager) updateNodeStatus() {
	enm.nodesMutex.RLock()
	nodes := make([]*EdgeNode, 0, len(enm.nodes))
	for _, node := range enm.nodes {
		nodes = append(nodes, node)
	}
	enm.nodesMutex.RUnlock()

	for _, node := range nodes {
		// 根据任务数更新状态
		if node.TasksRunning > 0 && node.Status != EdgeNodeStatusBusy {
			_ = enm.UpdateNodeStatus(node.ID, EdgeNodeStatusBusy)
		} else if node.TasksRunning == 0 && node.Status == EdgeNodeStatusBusy {
			_ = enm.UpdateNodeStatus(node.ID, EdgeNodeStatusIdle)
		}
	}
}

// GetNodeStats 获取节点统计
func (enm *EdgeNodeManager) GetNodeStats() map[string]interface{} {
	enm.nodesMutex.RLock()
	defer enm.nodesMutex.RUnlock()

	stats := map[string]interface{}{
		"total_nodes":   len(enm.nodes),
		"online_nodes":  0,
		"offline_nodes": 0,
		"busy_nodes":    0,
		"idle_nodes":    0,
		"by_type":       make(map[string]int),
	}

	byType := stats["by_type"].(map[string]int)

	for _, node := range enm.nodes {
		byType[node.Type]++

		switch node.Status {
		case EdgeNodeStatusOnline, EdgeNodeStatusIdle:
			stats["online_nodes"] = stats["online_nodes"].(int) + 1
		case EdgeNodeStatusOffline:
			stats["offline_nodes"] = stats["offline_nodes"].(int) + 1
		case EdgeNodeStatusBusy:
			stats["busy_nodes"] = stats["busy_nodes"].(int) + 1
		}

		if node.Status == EdgeNodeStatusIdle {
			stats["idle_nodes"] = stats["idle_nodes"].(int) + 1
		}
	}

	return stats
}

// SelectBestNode 选择最佳节点（基于负载均衡）
func (enm *EdgeNodeManager) SelectBestNode(requirements TaskRequirements) (*EdgeNode, error) {
	enm.nodesMutex.RLock()
	defer enm.nodesMutex.RUnlock()

	var bestNode *EdgeNode
	bestScore := -1.0

	for _, node := range enm.nodes {
		// 检查状态
		if node.Status != EdgeNodeStatusOnline && node.Status != EdgeNodeStatusIdle {
			continue
		}

		// 检查能力
		if requirements.CPU > 0 && node.Capabilities.CPU < requirements.CPU {
			continue
		}
		if requirements.Memory > 0 && node.Capabilities.Memory < requirements.Memory {
			continue
		}
		if requirements.Capabilities > 0 && node.Capabilities.Caps&requirements.Capabilities == 0 {
			continue
		}

		// 计算得分（资源越多、负载越低得分越高）
		score := enm.calculateNodeScore(node)

		// 应用权重
		if node.Weight > 0 {
			score *= float64(node.Weight) / 100.0
		}

		// 应用优先级
		if node.Priority > 0 {
			score += float64(node.Priority) * 0.1
		}

		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}

	if bestNode == nil {
		return nil, fmt.Errorf("没有找到满足要求的边缘节点")
	}

	return bestNode, nil
}

// calculateNodeScore 计算节点得分
func (enm *EdgeNodeManager) calculateNodeScore(node *EdgeNode) float64 {
	// 可用资源比例
	cpuAvail := 1.0 - node.Resources.CPUUsed/100.0
	memAvail := 1.0 - node.Resources.MemoryUsed/100.0
	storageAvail := 1.0 - node.Resources.StorageUsed/100.0

	// 综合得分
	score := (cpuAvail*0.4 + memAvail*0.3 + storageAvail*0.2) * 100

	// 任务负载惩罚
	score -= float64(node.TasksRunning) * 5

	// 确保得分非负
	if score < 0 {
		score = 0
	}

	return score
}

// Shutdown 关闭边缘节点管理器
func (enm *EdgeNodeManager) Shutdown() error {
	enm.cancel()
	enm.saveNodes()
	enm.logger.Info("边缘节点管理器已关闭")
	return nil
}

// 持久化

func (enm *EdgeNodeManager) saveNodes() error {
	nodesFile := filepath.Join(enm.config.DataDir, "edge_nodes.json")

	data, err := json.MarshalIndent(enm.nodes, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(nodesFile, data, 0644)
}

func (enm *EdgeNodeManager) loadNodes() error {
	nodesFile := filepath.Join(enm.config.DataDir, "edge_nodes.json")

	data, err := os.ReadFile(nodesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &enm.nodes)
}
