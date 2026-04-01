// Package cluster 提供多节点发现与注册服务
// 对标 TrueNAS Connect 和群晖 CMS 多系统管理功能
package cluster

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DiscoveryConfig 节点发现配置
type DiscoveryConfig struct {
	// 服务发现模式: mdns, static, api
	Mode string `json:"mode"`

	// mDNS 服务名称
	ServiceName string `json:"service_name"`

	// 发现端口
	DiscoveryPort int `json:"discovery_port"`

	// 静态节点列表（用于 static 模式）
	StaticNodes []NodeEndpoint `json:"static_nodes,omitempty"`

	// API 发现端点（用于 api 模式）
	APIEndpoint string `json:"api_endpoint,omitempty"`

	// 发现间隔
	DiscoveryInterval time.Duration `json:"discovery_interval"`

	// 注册超时
	RegisterTimeout time.Duration `json:"register_timeout"`

	// 心跳间隔
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`

	// 节点超时（多久未心跳视为离线）
	NodeTimeout time.Duration `json:"node_timeout"`

	// TLS 配置
	EnableTLS bool `json:"enable_tls"`
	CertFile  string `json:"cert_file,omitempty"`
	KeyFile   string `json:"key_file,omitempty"`

	// 数据目录
	DataDir string `json:"data_dir"`
}

// NodeEndpoint 节点端点配置
type NodeEndpoint struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Token   string `json:"token,omitempty"` // 注册令牌
}

// NodeInfo 注册节点信息
type NodeInfo struct {
	// 基本信息
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Version   string    `json:"version"` // nas-os 版本

	// 状态信息
	Status      NodeState `json:"status"`
	Role        string    `json:"role"` // master, worker
	LastSeen    time.Time `json:"last_seen"`
	RegisterTime time.Time `json:"register_time"`

	// 系统信息
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	CPUCores    int    `json:"cpu_cores"`
	MemoryTotal uint64 `json:"memory_total"` // bytes
	DiskTotal   uint64 `json:"disk_total"`   // bytes

	// 资源使用
	CPUUsage    float64 `json:"cpu_usage"`    // 百分比
	MemoryUsage float64 `json:"memory_usage"` // 百分比
	DiskUsage   float64 `json:"disk_usage"`   // 百分比

	// 能力标签
	Capabilities []string          `json:"capabilities,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`

	// 服务信息（引用 dashboard_aggregator.go 中的定义）
	Services []ServiceInfo `json:"services,omitempty"`
}

// ServiceInfo 在 dashboard_aggregator.go 中定义

// DiscoveryService 节点发现与注册服务
type DiscoveryService struct {
	config    DiscoveryConfig
	localNode *NodeInfo
	nodes     map[string]*NodeInfo
	nodeMutex sync.RWMutex

	// 服务发现
	mdnsServer   *MDNSServer
	mdnsResolver *MDNSResolver

	// HTTP 客户端
	httpClient *http.Client

	// 事件回调
	callbacks DiscoveryCallbacks

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	logger *zap.Logger
}

// DiscoveryCallbacks 发现事件回调
type DiscoveryCallbacks struct {
	// 节点发现
	OnNodeDiscovered func(node *NodeInfo)

	// 节点注册
	OnNodeRegistered func(node *NodeInfo)

	// 节点离线
	OnNodeOffline func(node *NodeInfo)

	// 节点状态更新
	OnNodeStatusUpdate func(node *NodeInfo)

	// 节点移除
	OnNodeRemoved func(nodeID string)
}

// NewDiscoveryService 创建发现服务
func NewDiscoveryService(config DiscoveryConfig, logger *zap.Logger) (*DiscoveryService, error) {
	// 设置默认值
	if config.Mode == "" {
		config.Mode = "mdns"
	}
	if config.ServiceName == "" {
		config.ServiceName = "_nasos._tcp"
	}
	if config.DiscoveryPort == 0 {
		config.DiscoveryPort = 8081
	}
	if config.DiscoveryInterval == 0 {
		config.DiscoveryInterval = 30 * time.Second
	}
	if config.RegisterTimeout == 0 {
		config.RegisterTimeout = 10 * time.Second
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 10 * time.Second
	}
	if config.NodeTimeout == 0 {
		config.NodeTimeout = 60 * time.Second
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/cluster"
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ds := &DiscoveryService{
		config:    config,
		nodes:     make(map[string]*NodeInfo),
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
	}

	// 创建 HTTP 客户端
	ds.httpClient = &http.Client{
		Timeout: config.RegisterTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !config.EnableTLS,
			},
		},
	}

	return ds, nil
}

// Initialize 初始化发现服务
func (ds *DiscoveryService) Initialize(localNode *NodeInfo) error {
	ds.localNode = localNode
	ds.localNode.Status = NodeStateActive
	ds.localNode.RegisterTime = time.Now()
	ds.localNode.LastSeen = time.Now()

	// 添加本地节点
	ds.nodeMutex.Lock()
	ds.nodes[localNode.ID] = localNode
	ds.nodeMutex.Unlock()

	// 根据模式启动发现
	switch ds.config.Mode {
	case "mdns":
		if err := ds.startMDNS(); err != nil {
			return fmt.Errorf("启动 mDNS 发现失败: %w", err)
		}
	case "static":
		ds.loadStaticNodes()
	case "api":
		ds.startAPIDiscovery()
	default:
		return fmt.Errorf("未知的发现模式: %s", ds.config.Mode)
	}

	// 启动心跳和监控
	ds.wg.Add(1)
	go ds.heartbeatLoop()

	ds.wg.Add(1)
	go ds.nodeMonitorLoop()

	ds.wg.Add(1)
	go ds.statusUpdateLoop()

	ds.logger.Info("节点发现服务已启动",
		zap.String("mode", ds.config.Mode),
		zap.String("node_id", localNode.ID))

	return nil
}

// startMDNS 启动 mDNS 发现
func (ds *DiscoveryService) startMDNS() error {
	// 启动 mDNS 服务器（广播本节点）
	ds.mdnsServer = NewMDNSServer(ds.config.ServiceName, ds.config.DiscoveryPort, ds.localNode, ds.logger)
	if err := ds.mdnsServer.Start(); err != nil {
		return err
	}

	// 启动 mDNS 解析器（发现其他节点）
	ds.mdnsResolver = NewMDNSResolver(ds.config.ServiceName, ds.logger)
	ds.mdnsResolver.SetCallbacks(MDNSResolverCallbacks{
		OnNodeFound: ds.handleDiscoveredNode,
	})
	if err := ds.mdnsResolver.Start(ds.ctx); err != nil {
		return err
	}

	return nil
}

// loadStaticNodes 加载静态节点
func (ds *DiscoveryService) loadStaticNodes() {
	for _, endpoint := range ds.config.StaticNodes {
		// 尝试注册静态节点
		go ds.registerNode(endpoint)
	}
}

// startAPIDiscovery 启动 API 发现
func (ds *DiscoveryService) startAPIDiscovery() {
	ds.wg.Add(1)
	go ds.apiDiscoveryLoop()
}

// apiDiscoveryLoop API 发现循环
func (ds *DiscoveryService) apiDiscoveryLoop() {
	defer ds.wg.Done()

	ticker := time.NewTicker(ds.config.DiscoveryInterval)
	defer ticker.Stop()

	// 首次立即执行
	ds.discoverFromAPI()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.discoverFromAPI()
		}
	}
}

// discoverFromAPI 从 API 发现节点
func (ds *DiscoveryService) discoverFromAPI() {
	if ds.config.APIEndpoint == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ds.ctx, ds.config.RegisterTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", ds.config.APIEndpoint+"/nodes", nil)
	if err != nil {
		ds.logger.Debug("创建发现请求失败", zap.Error(err))
		return
	}

	resp, err := ds.httpClient.Do(req)
	if err != nil {
		ds.logger.Debug("API 发现请求失败", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	var nodes []*NodeInfo
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		ds.logger.Debug("解析节点列表失败", zap.Error(err))
		return
	}

	for _, node := range nodes {
		if node.ID == ds.localNode.ID {
			continue // 忽略本地节点
		}
		ds.handleDiscoveredNode(node)
	}
}

// handleDiscoveredNode 处理发现的节点
func (ds *DiscoveryService) handleDiscoveredNode(node *NodeInfo) {
	ds.nodeMutex.Lock()
	defer ds.nodeMutex.Unlock()

	existing, exists := ds.nodes[node.ID]

	if exists {
		// 更新现有节点
		existing.LastSeen = time.Now()
		existing.Status = NodeStateActive
		existing.Address = node.Address
		existing.Port = node.Port

		// 更新资源使用
		existing.CPUUsage = node.CPUUsage
		existing.MemoryUsage = node.MemoryUsage
		existing.DiskUsage = node.DiskUsage

		// 触发状态更新回调
		if ds.callbacks.OnNodeStatusUpdate != nil {
			go ds.callbacks.OnNodeStatusUpdate(existing)
		}
	} else {
		// 添加新节点
		node.Status = NodeStateActive
		node.RegisterTime = time.Now()
		node.LastSeen = time.Now()
		ds.nodes[node.ID] = node

		ds.logger.Info("发现新节点",
			zap.String("node_id", node.ID),
			zap.String("name", node.Name),
			zap.String("address", node.Address))

		// 触发发现回调
		if ds.callbacks.OnNodeDiscovered != nil {
			go ds.callbacks.OnNodeDiscovered(node)
		}

		// 触发注册回调
		if ds.callbacks.OnNodeRegistered != nil {
			go ds.callbacks.OnNodeRegistered(node)
		}
	}

	// 持久化
	_ = ds.saveState()
}

// registerNode 注册到远程节点
func (ds *DiscoveryService) registerNode(endpoint NodeEndpoint) {
	ctx, cancel := context.WithTimeout(ds.ctx, ds.config.RegisterTimeout)
	defer cancel()

	url := fmt.Sprintf("http://%s:%d/cluster/register", endpoint.Address, endpoint.Port)

	payload := map[string]interface{}{
		"id":        ds.localNode.ID,
		"name":      ds.localNode.Name,
		"address":   ds.localNode.Address,
		"port":      ds.localNode.Port,
		"version":   ds.localNode.Version,
		"token":     endpoint.Token,
		"capabilities": ds.localNode.Capabilities,
		"labels":    ds.localNode.Labels,
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		ds.logger.Debug("创建注册请求失败", zap.Error(err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-ID", ds.localNode.ID)

	resp, err := ds.httpClient.Do(req)
	if err != nil {
		ds.logger.Debug("注册节点失败",
			zap.String("endpoint", endpoint.Address),
			zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		ds.logger.Info("节点注册成功",
			zap.String("endpoint", endpoint.Address),
			zap.String("node_id", endpoint.ID))
	} else {
		ds.logger.Warn("节点注册失败",
			zap.String("endpoint", endpoint.Address),
			zap.Int("status", resp.StatusCode))
	}
}

// heartbeatLoop 心跳循环
func (ds *DiscoveryService) heartbeatLoop() {
	defer ds.wg.Done()

	ticker := time.NewTicker(ds.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.sendHeartbeats()
		}
	}
}

// sendHeartbeats 发送心跳到所有节点
func (ds *DiscoveryService) sendHeartbeats() {
	ds.nodeMutex.RLock()
	nodes := make([]*NodeInfo, 0, len(ds.nodes))
	for _, node := range ds.nodes {
		if node.ID != ds.localNode.ID {
			nodes = append(nodes, node)
		}
	}
	ds.nodeMutex.RUnlock()

	for _, node := range nodes {
		go ds.sendHeartbeat(node)
	}
}

// sendHeartbeat 发送心跳到指定节点
func (ds *DiscoveryService) sendHeartbeat(node *NodeInfo) {
	ctx, cancel := context.WithTimeout(ds.ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://%s:%d/cluster/heartbeat", node.Address, node.Port)

	// 构造心跳数据
	payload := map[string]interface{}{
		"id":          ds.localNode.ID,
		"status":      ds.localNode.Status,
		"cpu_usage":   ds.localNode.CPUUsage,
		"memory_usage": ds.localNode.MemoryUsage,
		"disk_usage":  ds.localNode.DiskUsage,
		"timestamp":   time.Now().Unix(),
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-ID", ds.localNode.ID)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		ds.logger.Debug("心跳发送失败",
			zap.String("node_id", node.ID),
			zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		ds.nodeMutex.Lock()
		node.LastSeen = time.Now()
		node.Status = NodeStateActive
		ds.nodeMutex.Unlock()
	}
}

// nodeMonitorLoop 节点监控循环
func (ds *DiscoveryService) nodeMonitorLoop() {
	defer ds.wg.Done()

	ticker := time.NewTicker(ds.config.NodeTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.checkNodeStatus()
		}
	}
}

// checkNodeStatus 检查节点状态
func (ds *DiscoveryService) checkNodeStatus() {
	ds.nodeMutex.Lock()
	defer ds.nodeMutex.Unlock()

	now := time.Now()
	for id, node := range ds.nodes {
		if id == ds.localNode.ID {
			continue
		}

		if node.Status == NodeStateFailed {
			continue
		}

		// 检查超时
		if now.Sub(node.LastSeen) > ds.config.NodeTimeout {
			ds.logger.Warn("节点超时离线",
				zap.String("node_id", id),
				zap.Duration("elapsed", now.Sub(node.LastSeen)))

			node.Status = NodeStateFailed

			// 触发离线回调
			if ds.callbacks.OnNodeOffline != nil {
				go ds.callbacks.OnNodeOffline(node)
			}
		} else if now.Sub(node.LastSeen) > ds.config.NodeTimeout/2 {
			// 可疑状态
			if node.Status == NodeStateActive {
				node.Status = NodeStateSuspect
				ds.logger.Debug("节点可疑",
					zap.String("node_id", id),
					zap.Duration("elapsed", now.Sub(node.LastSeen)))
			}
		}
	}

	_ = ds.saveState()
}

// statusUpdateLoop 状态更新循环（定期收集本地资源使用）
func (ds *DiscoveryService) statusUpdateLoop() {
	defer ds.wg.Done()

	ticker := time.NewTicker(ds.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.updateLocalStatus()
		}
	}
}

// updateLocalStatus 更新本地节点状态
func (ds *DiscoveryService) updateLocalStatus() {
	// 收集系统指标
	cpuUsage, memUsage, diskUsage := ds.collectSystemMetrics()

	ds.nodeMutex.Lock()
	ds.localNode.CPUUsage = cpuUsage
	ds.localNode.MemoryUsage = memUsage
	ds.localNode.DiskUsage = diskUsage
	ds.localNode.LastSeen = time.Now()
	ds.nodeMutex.Unlock()

	// 触发状态更新回调
	if ds.callbacks.OnNodeStatusUpdate != nil {
		go ds.callbacks.OnNodeStatusUpdate(ds.localNode)
	}
}

// collectSystemMetrics 收集系统指标
func (ds *DiscoveryService) collectSystemMetrics() (cpu, mem, disk float64) {
	// 简化实现：返回模拟值
	// 实际实现应读取 /proc/stat, /proc/meminfo 等
	return 0.0, 0.0, 0.0
}

// GetNodes 获取所有节点
func (ds *DiscoveryService) GetNodes() []*NodeInfo {
	ds.nodeMutex.RLock()
	defer ds.nodeMutex.RUnlock()

	nodes := make([]*NodeInfo, 0, len(ds.nodes))
	for _, node := range ds.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetNode 获取指定节点
func (ds *DiscoveryService) GetNode(nodeID string) (*NodeInfo, bool) {
	ds.nodeMutex.RLock()
	defer ds.nodeMutex.RUnlock()

	node, exists := ds.nodes[nodeID]
	return node, exists
}

// GetOnlineNodes 获取在线节点
func (ds *DiscoveryService) GetOnlineNodes() []*NodeInfo {
	ds.nodeMutex.RLock()
	defer ds.nodeMutex.RUnlock()

	nodes := make([]*NodeInfo, 0)
	for _, node := range ds.nodes {
		if node.Status == NodeStateActive {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// RemoveNode 移除节点
func (ds *DiscoveryService) RemoveNode(nodeID string) error {
	ds.nodeMutex.Lock()
	defer ds.nodeMutex.Unlock()

	if nodeID == ds.localNode.ID {
		return fmt.Errorf("不能移除本地节点")
	}

	delete(ds.nodes, nodeID)
	ds.logger.Info("节点已移除", zap.String("node_id", nodeID))

	// 触发回调
	if ds.callbacks.OnNodeRemoved != nil {
		go ds.callbacks.OnNodeRemoved(nodeID)
	}

	_ = ds.saveState()
	return nil
}

// SetCallbacks 设置回调
func (ds *DiscoveryService) SetCallbacks(callbacks DiscoveryCallbacks) {
	ds.callbacks = callbacks
}

// Shutdown 关闭发现服务
func (ds *DiscoveryService) Shutdown() error {
	ds.cancel()
	ds.wg.Wait()

	if ds.mdnsServer != nil {
		ds.mdnsServer.Stop()
	}
	if ds.mdnsResolver != nil {
		ds.mdnsResolver.Stop()
	}

	_ = ds.saveState()
	ds.logger.Info("节点发现服务已关闭")
	return nil
}

// saveState 持久化状态
func (ds *DiscoveryService) saveState() error {
	ds.nodeMutex.RLock()
	defer ds.nodeMutex.RUnlock()

	state := map[string]interface{}{
		"nodes":     ds.nodes,
		"timestamp": time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	stateFile := fmt.Sprintf("%s/discovery_state.json", ds.config.DataDir)
	return os.WriteFile(stateFile, data, 0640)
}

// loadState 加载状态
func (ds *DiscoveryService) loadState() error {
	stateFile := fmt.Sprintf("%s/discovery_state.json", ds.config.DataDir)

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		Nodes     map[string]*NodeInfo `json:"nodes"`
		Timestamp time.Time            `json:"timestamp"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	// 加载节点（标记为离线）
	for id, node := range state.Nodes {
		if id != ds.localNode.ID {
			node.Status = NodeStateInactive
			ds.nodes[id] = node
		}
	}

	return nil
}

// GetLocalIP 获取本地 IP
func getLocalIP() (string, error) {
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

	return "", fmt.Errorf("未找到有效 IPv4 地址")
}