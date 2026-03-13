package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"go.uber.org/zap"
)

// 节点角色定义
const (
	RoleMaster = "master"
	RoleWorker = "worker"
)

// 节点状态定义
const (
	StatusOnline   = "online"
	StatusOffline  = "offline"
	StatusDegraded = "degraded"
)

// NodeMetrics 节点性能指标
type NodeMetrics struct {
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	DiskUsage   float64   `json:"disk_usage"`
	NetworkIn   int64     `json:"network_in"`
	NetworkOut  int64     `json:"network_out"`
	ActiveConns int       `json:"active_connections"`
	LastUpdate  time.Time `json:"last_update"`
}

// ClusterNode 集群节点信息
type ClusterNode struct {
	ID        string      `json:"id"`
	Hostname  string      `json:"hostname"`
	IP        string      `json:"ip"`
	Port      int         `json:"port"`
	Role      string      `json:"role"`
	Status    string      `json:"status"`
	Heartbeat time.Time   `json:"heartbeat"`
	Metrics   NodeMetrics `json:"metrics"`
	JoinTime  time.Time   `json:"join_time"`
}

// SimpleClusterConfig 简化集群配置（用于 mDNS 发现模式）
type SimpleClusterConfig struct {
	Name              string `json:"name"`
	NodeID            string `json:"node_id"`
	DiscoveryPort     int    `json:"discovery_port"`
	HeartbeatInterval int    `json:"heartbeat_interval"` // 秒
	HeartbeatTimeout  int    `json:"heartbeat_timeout"`  // 秒
	DataDir           string `json:"data_dir"`
}

// ClusterManager 集群管理器
type ClusterManager struct {
	config     SimpleClusterConfig
	nodes      map[string]*ClusterNode
	nodesMutex sync.RWMutex
	masterID   string
	resolver   *zeroconf.Resolver
	server     *zeroconf.Server
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *zap.Logger
	callbacks  ClusterCallbacks
}

// ClusterCallbacks 集群事件回调
type ClusterCallbacks struct {
	OnNodeJoin     func(node *ClusterNode)
	OnNodeLeave    func(node *ClusterNode)
	OnMasterChange func(oldMaster, newMaster string)
}

// NewManager 创建集群管理器
func NewManager(config SimpleClusterConfig, logger *zap.Logger) (*ClusterManager, error) {
	if config.NodeID == "" {
		hostname, _ := os.Hostname()
		config.NodeID = fmt.Sprintf("node-%s", hostname)
	}
	if config.DiscoveryPort == 0 {
		config.DiscoveryPort = 8081
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 5
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 15
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/cluster"
	}

	ctx, cancel := context.WithCancel(context.Background())

	cm := &ClusterManager{
		config:   config,
		nodes:    make(map[string]*ClusterNode),
		masterID: config.NodeID, // 初始化为自身
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建集群数据目录失败：%w", err)
	}

	// 加载持久化数据
	if err := cm.loadState(); err != nil {
		logger.Warn("加载集群状态失败", zap.Error(err))
	}

	return cm, nil
}

// Initialize 初始化集群管理器
func (cm *ClusterManager) Initialize() error {
	cm.logger.Info("初始化集群管理器", zap.String("node_id", cm.config.NodeID))

	// 启动 mDNS 服务发现
	if err := cm.startMDNSServer(); err != nil {
		return fmt.Errorf("启动 mDNS 服务失败：%w", err)
	}

	// 启动 mDNS 服务发现客户端
	if err := cm.startMDNSDiscovery(); err != nil {
		return fmt.Errorf("启动 mDNS 发现失败：%w", err)
	}

	// 启动心跳检测
	go cm.heartbeatWorker()

	// 启动节点状态监控
	go cm.monitorWorker()

	cm.logger.Info("集群管理器初始化完成")
	return nil
}

// startMDNSServer 启动 mDNS 服务广播
func (cm *ClusterManager) startMDNSServer() error {
	// 获取本机 IP
	ip, err := cm.getLocalIP()
	if err != nil {
		return err
	}

	// 注册 mDNS 服务
	server, err := zeroconf.Register(
		fmt.Sprintf("nas-os-%s", cm.config.NodeID),
		"_nasos._tcp",
		"local.",
		cm.config.DiscoveryPort,
		[]string{
			fmt.Sprintf("node_id=%s", cm.config.NodeID),
			fmt.Sprintf("hostname=%s", cm.config.NodeID),
			fmt.Sprintf("ip=%s", ip),
			fmt.Sprintf("role=%s", RoleMaster),
		},
		nil,
	)
	if err != nil {
		return err
	}

	cm.server = server
	cm.logger.Info("mDNS 服务已注册", zap.String("service", "_nasos._tcp.local."))
	return nil
}

// startMDNSDiscovery 启动 mDNS 服务发现
func (cm *ClusterManager) startMDNSDiscovery() error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}

	cm.resolver = resolver

	// 启动服务发现
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			cm.handleDiscoveredNode(entry)
		}
	}()

	go func() {
		if err := cm.resolver.Browse(cm.ctx, "_nasos._tcp", "local.", entries); err != nil {
			cm.logger.Error("mDNS 发现失败", zap.Error(err))
		}
	}()

	cm.logger.Info("mDNS 服务发现已启动")
	return nil
}

// handleDiscoveredNode 处理发现的节点
func (cm *ClusterManager) handleDiscoveredNode(entry *zeroconf.ServiceEntry) {
	if len(entry.AddrIPv4) == 0 {
		return
	}

	// 解析服务信息
	nodeID := ""
	hostname := ""
	role := RoleWorker
	for _, text := range entry.Text {
		if len(text) > 8 {
			switch {
			case text[:8] == "node_id=":
				nodeID = text[8:]
			case text[:10] == "hostname=":
				hostname = text[10:]
			case text[:6] == "role=":
				role = text[6:]
			}
		}
	}

	if nodeID == "" || nodeID == cm.config.NodeID {
		return // 忽略自身
	}

	cm.nodesMutex.Lock()
	defer cm.nodesMutex.Unlock()

	// 检查节点是否已存在
	if _, exists := cm.nodes[nodeID]; exists {
		// 更新心跳
		cm.nodes[nodeID].Heartbeat = time.Now()
		cm.nodes[nodeID].Status = StatusOnline
		return
	}

	// 添加新节点
	node := &ClusterNode{
		ID:        nodeID,
		Hostname:  hostname,
		IP:        entry.AddrIPv4[0].String(),
		Port:      entry.Port,
		Role:      role,
		Status:    StatusOnline,
		Heartbeat: time.Now(),
		JoinTime:  time.Now(),
	}

	cm.nodes[nodeID] = node
	cm.logger.Info("发现新节点", zap.String("node_id", nodeID), zap.String("ip", node.IP))

	// 触发回调
	if cm.callbacks.OnNodeJoin != nil {
		go cm.callbacks.OnNodeJoin(node)
	}

	// 持久化状态
	cm.saveState()
}

// heartbeatWorker 心跳检测工作线程
func (cm *ClusterManager) heartbeatWorker() {
	ticker := time.NewTicker(time.Duration(cm.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.broadcastHeartbeat()
		}
	}
}

// broadcastHeartbeat 广播心跳
func (cm *ClusterManager) broadcastHeartbeat() {
	cm.nodesMutex.RLock()
	defer cm.nodesMutex.RUnlock()

	for _, node := range cm.nodes {
		if node.Status == StatusOffline {
			continue
		}

		// 发送心跳请求
		go cm.sendHeartbeat(node)
	}
}

// sendHeartbeat 发送心跳到指定节点
func (cm *ClusterManager) sendHeartbeat(node *ClusterNode) {
	// TODO: 实现 HTTP 心跳请求
	// 这里简化处理，只更新时间
	node.Heartbeat = time.Now()
}

// monitorWorker 节点状态监控工作线程
func (cm *ClusterManager) monitorWorker() {
	ticker := time.NewTicker(time.Duration(cm.config.HeartbeatTimeout) * time.Second / 2)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.checkNodeStatus()
		}
	}
}

// checkNodeStatus 检查节点状态
func (cm *ClusterManager) checkNodeStatus() {
	cm.nodesMutex.Lock()
	defer cm.nodesMutex.Unlock()

	timeout := time.Duration(cm.config.HeartbeatTimeout) * time.Second
	now := time.Now()

	for id, node := range cm.nodes {
		if node.Status == StatusOffline {
			continue
		}

		if now.Sub(node.Heartbeat) > timeout {
			cm.logger.Warn("节点超时", zap.String("node_id", id), zap.Duration("elapsed", now.Sub(node.Heartbeat)))
			node.Status = StatusOffline

			// 触发回调
			if cm.callbacks.OnNodeLeave != nil {
				go cm.callbacks.OnNodeLeave(node)
			}

			// 如果是主节点离线，触发选举
			if id == cm.masterID {
				cm.electNewMaster()
			}
		}
	}

	// 持久化状态
	cm.saveState()
}

// electNewMaster 选举新主节点
func (cm *ClusterManager) electNewMaster() {
	cm.logger.Info("开始主节点选举")

	// 简单选举：选择最早加入的在线节点
	var candidates []*ClusterNode
	for _, node := range cm.nodes {
		if node.Status == StatusOnline && node.Role == RoleWorker {
			candidates = append(candidates, node)
		}
	}

	if len(candidates) == 0 {
		cm.logger.Warn("没有可用的候选节点")
		return
	}

	// 选择最早加入的节点
	newMaster := candidates[0]
	for _, node := range candidates {
		if node.JoinTime.Before(newMaster.JoinTime) {
			newMaster = node
		}
	}

	oldMaster := cm.masterID
	cm.masterID = newMaster.ID
	newMaster.Role = RoleMaster

	cm.logger.Info("新主节点选举完成",
		zap.String("old_master", oldMaster),
		zap.String("new_master", newMaster.ID))

	// 触发回调
	if cm.callbacks.OnMasterChange != nil {
		go cm.callbacks.OnMasterChange(oldMaster, newMaster.ID)
	}
}

// GetNodes 获取所有节点
func (cm *ClusterManager) GetNodes() []*ClusterNode {
	cm.nodesMutex.RLock()
	defer cm.nodesMutex.RUnlock()

	nodes := make([]*ClusterNode, 0, len(cm.nodes))
	for _, node := range cm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetNode 获取指定节点
func (cm *ClusterManager) GetNode(nodeID string) (*ClusterNode, bool) {
	cm.nodesMutex.RLock()
	defer cm.nodesMutex.RUnlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return nil, false
	}
	return node, true
}

// GetMasterNode 获取主节点
func (cm *ClusterManager) GetMasterNode() *ClusterNode {
	cm.nodesMutex.RLock()
	defer cm.nodesMutex.RUnlock()

	if master, exists := cm.nodes[cm.masterID]; exists {
		return master
	}
	return nil
}

// GetOnlineNodes 获取在线节点
func (cm *ClusterManager) GetOnlineNodes() []*ClusterNode {
	cm.nodesMutex.RLock()
	defer cm.nodesMutex.RUnlock()

	nodes := make([]*ClusterNode, 0)
	for _, node := range cm.nodes {
		if node.Status == StatusOnline {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// RemoveNode 移除节点
func (cm *ClusterManager) RemoveNode(nodeID string) error {
	cm.nodesMutex.Lock()
	defer cm.nodesMutex.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点不存在：%s", nodeID)
	}

	delete(cm.nodes, nodeID)
	cm.logger.Info("节点已移除", zap.String("node_id", nodeID))

	// 触发回调
	if cm.callbacks.OnNodeLeave != nil {
		go cm.callbacks.OnNodeLeave(node)
	}

	// 持久化状态
	cm.saveState()
	return nil
}

// UpdateNodeMetrics 更新节点指标
func (cm *ClusterManager) UpdateNodeMetrics(nodeID string, metrics NodeMetrics) error {
	cm.nodesMutex.Lock()
	defer cm.nodesMutex.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点不存在：%s", nodeID)
	}

	node.Metrics = metrics
	node.Metrics.LastUpdate = time.Now()
	return nil
}

// SetCallbacks 设置事件回调
func (cm *ClusterManager) SetCallbacks(callbacks ClusterCallbacks) {
	cm.callbacks = callbacks
}

// GetConfig 获取集群配置
func (cm *ClusterManager) GetConfig() SimpleClusterConfig {
	return cm.config
}

// IsMaster 检查当前节点是否为主节点
func (cm *ClusterManager) IsMaster() bool {
	return cm.config.NodeID == cm.masterID
}

// GetMasterID 获取主节点 ID
func (cm *ClusterManager) GetMasterID() string {
	return cm.masterID
}

// Shutdown 关闭集群管理器
func (cm *ClusterManager) Shutdown() error {
	cm.cancel()

	if cm.server != nil {
		cm.server.Shutdown()
	}

	cm.logger.Info("集群管理器已关闭")
	return nil
}

// 辅助函数

// getLocalIP 获取本机 IP
func (cm *ClusterManager) getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("未找到有效的 IPv4 地址")
}

// saveState 持久化集群状态
func (cm *ClusterManager) saveState() error {
	state := map[string]interface{}{
		"master_id": cm.masterID,
		"nodes":     cm.nodes,
		"timestamp": time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	stateFile := fmt.Sprintf("%s/cluster_state.json", cm.config.DataDir)
	return os.WriteFile(stateFile, data, 0644)
}

// loadState 加载集群状态
func (cm *ClusterManager) loadState() error {
	stateFile := fmt.Sprintf("%s/cluster_state.json", cm.config.DataDir)

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 首次启动，无状态文件
		}
		return err
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	if masterID, ok := state["master_id"].(string); ok {
		cm.masterID = masterID
	}

	// TODO: 加载节点列表
	// 注意：重启后节点需要重新发现，这里只保留主节点信息

	return nil
}
