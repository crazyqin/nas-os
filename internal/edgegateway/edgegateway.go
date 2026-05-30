// Package edgegateway 边缘网关模块
// 边缘计算节点管理、本地数据处理、智能路由、离线支持、边缘缓存
package edgegateway

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// EdgeNodeType 边缘节点类型
type EdgeNodeType string

const (
	NodeTypeGateway   EdgeNodeType = "gateway"
	NodeTypeCompute   EdgeNodeType = "compute"
	NodeTypeStorage   EdgeNodeType = "storage"
	NodeTypeSensor    EdgeNodeType = "sensor"
	NodeTypeHybrid    EdgeNodeType = "hybrid"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	StatusOnline     NodeStatus = "online"
	StatusOffline    NodeStatus = "offline"
	StatusDegraded   NodeStatus = "degraded"
	StatusSyncing    NodeStatus = "syncing"
	StatusError      NodeStatus = "error"
)

// EdgePolicy 边缘策略
type EdgePolicy string

const (
	PolicyLocalFirst  EdgePolicy = "local_first"
	PolicyCloudFirst  EdgePolicy = "cloud_first"
	PolicyBalanced    EdgePolicy = "balanced"
	PolicyCostOptimal EdgePolicy = "cost_optimal"
	PolicyLatency     EdgePolicy = "latency"
)

// EdgeNode 边缘节点
type EdgeNode struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Type         EdgeNodeType  `json:"type"`
	Status       NodeStatus    `json:"status"`
	Location     Location      `json:"location"`
	IPAddress    string        `json:"ip_address"`
	MACAddress   string        `json:"mac_address"`
	CPUUsage     float64       `json:"cpu_usage"`
	MemoryUsage  float64       `json:"memory_usage"`
	StorageUsage float64       `json:"storage_usage"`
	Bandwidth    float64       `json:"bandwidth_mbps"`
	Latency      int           `json:"latency_ms"`
	LastSeen     time.Time     `json:"last_seen"`
	Uptime       time.Duration `json:"uptime"`
	Tags         []string      `json:"tags"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// Location 位置信息
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
	Region    string  `json:"region"`
}

// EdgeTask 边缘任务
type EdgeTask struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	NodeID      string        `json:"node_id"`
	Status      string        `json:"status"`
	Priority    int           `json:"priority"`
	Input       interface{}   `json:"input"`
	Output      interface{}   `json:"output,omitempty"`
	Error       string        `json:"error,omitempty"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration"`
	Retries     int           `json:"retries"`
	MaxRetries  int           `json:"max_retries"`
	CreatedAt   time.Time     `json:"created_at"`
}

// EdgeCache 边缘缓存
type EdgeCache struct {
	mu          sync.RWMutex
	entries     map[string]*CacheEntry
	maxSize     int64
	currentSize int64
	hitCount    int64
	missCount   int64
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	Size       int64       `json:"size"`
	HitCount   int64       `json:"hit_count"`
	ExpiresAt  time.Time   `json:"expires_at"`
	CreatedAt  time.Time   `json:"created_at"`
	LastAccess time.Time   `json:"last_access"`
}

// EdgeSync 边缘同步
type EdgeSync struct {
	ID          string    `json:"id"`
	SourceNode  string    `json:"source_node"`
	TargetNode  string    `json:"target_node"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	BytesTotal  int64     `json:"bytes_total"`
	BytesSynced int64     `json:"bytes_synced"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// EdgeRoute 边缘路由
type EdgeRoute struct {
	ID          string     `json:"id"`
	Source      string     `json:"source"`
	Destination string     `json:"destination"`
	NextHop     string     `json:"next_hop"`
	Metric      int        `json:"metric"`
	Policy      EdgePolicy `json:"policy"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// EdgeGatewayStats 边缘网关统计
type EdgeGatewayStats struct {
	TotalNodes     int                  `json:"total_nodes"`
	OnlineNodes    int                  `json:"online_nodes"`
	OfflineNodes   int                  `json:"offline_nodes"`
	DegradedNodes  int                  `json:"degraded_nodes"`
	TotalTasks     int                  `json:"total_tasks"`
	RunningTasks   int                  `json:"running_tasks"`
	CompletedTasks int                  `json:"completed_tasks"`
	FailedTasks    int                  `json:"failed_tasks"`
	CacheHitRate   float64              `json:"cache_hit_rate"`
	CacheSize      int64                `json:"cache_size"`
	TotalRoutes    int                  `json:"total_routes"`
	ActiveRoutes   int                  `json:"active_routes"`
	AvgLatency     float64              `json:"avg_latency_ms"`
	NodeTypes      map[EdgeNodeType]int `json:"node_types"`
}

// EdgeGateway 边缘网关
type EdgeGateway struct {
	mu          sync.RWMutex
	nodes       map[string]*EdgeNode
	tasks       map[string]*EdgeTask
	routes      map[string]*EdgeRoute
	syncs       map[string]*EdgeSync
	cache       *EdgeCache
	config      *GatewayConfig
	taskQueue   chan *EdgeTask
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	DefaultPolicy    EdgePolicy `json:"default_policy"`
	SyncIntervalSec  int        `json:"sync_interval_sec"`
	CacheSizeMB      int        `json:"cache_size_mb"`
	CacheTTLMinutes  int        `json:"cache_ttl_minutes"`
	MaxRetries       int        `json:"max_retries"`
	HealthCheckSec   int        `json:"health_check_sec"`
	AutoDiscover     bool       `json:"auto_discover"`
	OfflineSupport   bool       `json:"offline_support"`
	CompressionEnabled bool     `json:"compression_enabled"`
}

// NewEdgeGateway 创建边缘网关
func NewEdgeGateway(config *GatewayConfig) *EdgeGateway {
	if config == nil {
		config = &GatewayConfig{
			DefaultPolicy:      PolicyBalanced,
			SyncIntervalSec:    30,
			CacheSizeMB:        1024,
			CacheTTLMinutes:    60,
			MaxRetries:         3,
			HealthCheckSec:     10,
			AutoDiscover:       true,
			OfflineSupport:     true,
			CompressionEnabled: true,
		}
	}

	cache := &EdgeCache{
		entries: make(map[string]*CacheEntry),
		maxSize: int64(config.CacheSizeMB) * 1024 * 1024,
	}

	gw := &EdgeGateway{
		nodes:     make(map[string]*EdgeNode),
		tasks:     make(map[string]*EdgeTask),
		routes:    make(map[string]*EdgeRoute),
		syncs:     make(map[string]*EdgeSync),
		cache:     cache,
		config:    config,
		taskQueue: make(chan *EdgeTask, 1000),
	}

	// 启动健康检查
	if config.HealthCheckSec > 0 {
		go gw.healthCheckLoop()
	}

	return gw
}

// RegisterNode 注册边缘节点
func (eg *EdgeGateway) RegisterNode(node *EdgeNode) error {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}

	now := time.Now()
	node.CreatedAt = now
	node.UpdatedAt = now
	node.LastSeen = now
	node.Status = StatusOnline

	eg.nodes[node.ID] = node
	return nil
}

// UnregisterNode 注销边缘节点
func (eg *EdgeGateway) UnregisterNode(nodeID string) error {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	if _, exists := eg.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	delete(eg.nodes, nodeID)
	return nil
}

// SubmitTask 提交边缘任务
func (eg *EdgeGateway) SubmitTask(task *EdgeTask) error {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}

	// 自动选择节点
	if task.NodeID == "" {
		nodeID := eg.selectBestNode(task.Type)
		if nodeID == "" {
			return fmt.Errorf("no available node for task type %s", task.Type)
		}
		task.NodeID = nodeID
	}

	task.Status = "pending"
	task.MaxRetries = eg.config.MaxRetries
	task.CreatedAt = time.Now()

	eg.tasks[task.ID] = task

	// 发送到任务队列
	select {
	case eg.taskQueue <- task:
	default:
		task.Status = "queued"
	}

	return nil
}

// GetTask 获取任务状态
func (eg *EdgeGateway) GetTask(taskID string) (*EdgeTask, error) {
	eg.mu.RLock()
	defer eg.mu.RUnlock()

	task, exists := eg.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// GetNode 获取节点信息
func (eg *EdgeGateway) GetNode(nodeID string) (*EdgeNode, error) {
	eg.mu.RLock()
	defer eg.mu.RUnlock()

	node, exists := eg.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}
	return node, nil
}

// AddRoute 添加路由
func (eg *EdgeGateway) AddRoute(route *EdgeRoute) error {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	if route.ID == "" {
		return fmt.Errorf("route ID is required")
	}

	now := time.Now()
	route.CreatedAt = now
	route.UpdatedAt = now

	eg.routes[route.ID] = route
	return nil
}

// RemoveRoute 删除路由
func (eg *EdgeGateway) RemoveRoute(routeID string) error {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	if _, exists := eg.routes[routeID]; !exists {
		return fmt.Errorf("route %s not found", routeID)
	}

	delete(eg.routes, routeID)
	return nil
}

// CacheGet 获取缓存
func (eg *EdgeGateway) CacheGet(key string) (interface{}, bool) {
	eg.cache.mu.RLock()
	defer eg.cache.mu.RUnlock()

	entry, exists := eg.cache.entries[key]
	if !exists {
		eg.cache.missCount++
		return nil, false
	}

	// 检查过期
	if time.Now().After(entry.ExpiresAt) {
		eg.cache.missCount++
		return nil, false
	}

	entry.HitCount++
	entry.LastAccess = time.Now()
	eg.cache.hitCount++

	return entry.Value, true
}

// CacheSet 设置缓存
func (eg *EdgeGateway) CacheSet(key string, value interface{}, ttl time.Duration) error {
	eg.cache.mu.Lock()
	defer eg.cache.mu.Unlock()

	size := int64(len(key)) + 100 // 简化大小计算

	// 检查空间
	if eg.cache.currentSize+size > eg.cache.maxSize {
		eg.evictCache(size)
	}

	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		Size:       size,
		ExpiresAt:  time.Now().Add(ttl),
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
	}

	eg.cache.entries[key] = entry
	eg.cache.currentSize += size

	return nil
}

// CacheDelete 删除缓存
func (eg *EdgeGateway) CacheDelete(key string) {
	eg.cache.mu.Lock()
	defer eg.cache.mu.Unlock()

	if entry, exists := eg.cache.entries[key]; exists {
		eg.cache.currentSize -= entry.Size
		delete(eg.cache.entries, key)
	}
}

// GetStats 获取统计信息
func (eg *EdgeGateway) GetStats() *EdgeGatewayStats {
	eg.mu.RLock()
	defer eg.mu.RUnlock()

	stats := &EdgeGatewayStats{
		NodeTypes: make(map[EdgeNodeType]int),
	}

	for _, node := range eg.nodes {
		stats.TotalNodes++
		stats.NodeTypes[node.Type]++

		switch node.Status {
		case StatusOnline:
			stats.OnlineNodes++
		case StatusOffline:
			stats.OfflineNodes++
		case StatusDegraded:
			stats.DegradedNodes++
		}

		stats.AvgLatency += float64(node.Latency)
	}

	if stats.TotalNodes > 0 {
		stats.AvgLatency /= float64(stats.TotalNodes)
	}

	for _, task := range eg.tasks {
		stats.TotalTasks++
		switch task.Status {
		case "running":
			stats.RunningTasks++
		case "completed":
			stats.CompletedTasks++
		case "failed":
			stats.FailedTasks++
		}
	}

	stats.TotalRoutes = len(eg.routes)
	for _, route := range eg.routes {
		if route.Enabled {
			stats.ActiveRoutes++
		}
	}

	// 缓存统计
	eg.cache.mu.RLock()
	stats.CacheSize = eg.cache.currentSize
	totalRequests := eg.cache.hitCount + eg.cache.missCount
	if totalRequests > 0 {
		stats.CacheHitRate = float64(eg.cache.hitCount) / float64(totalRequests) * 100
	}
	eg.cache.mu.RUnlock()

	return stats
}

// SyncNodes 同步节点数据
func (eg *EdgeGateway) SyncNodes(sourceID, targetID string) (*EdgeSync, error) {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	source, exists := eg.nodes[sourceID]
	if !exists {
		return nil, fmt.Errorf("source node %s not found", sourceID)
	}

	target, exists := eg.nodes[targetID]
	if !exists {
		return nil, fmt.Errorf("target node %s not found", targetID)
	}

	sync := &EdgeSync{
		ID:         fmt.Sprintf("sync_%d", time.Now().UnixNano()),
		SourceNode: sourceID,
		TargetNode: targetID,
		Status:     "running",
		StartedAt:  time.Now(),
	}

	eg.syncs[sync.ID] = sync

	// 模拟同步过程
	go func() {
		time.Sleep(time.Second * 2)
		eg.mu.Lock()
		sync.Status = "completed"
		sync.Progress = 100
		now := time.Now()
		sync.CompletedAt = &now
		eg.mu.Unlock()
	}()

	_ = source // 使用变量
	_ = target

	return sync, nil
}

// MarshalJSON 序列化
func (eg *EdgeGateway) MarshalJSON() ([]byte, error) {
	eg.mu.RLock()
	defer eg.mu.RUnlock()

	return json.Marshal(struct {
		Nodes   map[string]*EdgeNode  `json:"nodes"`
		Tasks   map[string]*EdgeTask  `json:"tasks"`
		Routes  map[string]*EdgeRoute `json:"routes"`
		Config  *GatewayConfig        `json:"config"`
	}{
		Nodes:  eg.nodes,
		Tasks:  eg.tasks,
		Routes: eg.routes,
		Config: eg.config,
	})
}

// 内部方法

func (eg *EdgeGateway) selectBestNode(taskType string) string {
	var bestNode string
	bestScore := -1.0

	for _, node := range eg.nodes {
		if node.Status != StatusOnline {
			continue
		}

		// 计算节点得分
		score := 100.0
		score -= node.CPUUsage * 0.3
		score -= node.MemoryUsage * 0.3
		score -= float64(node.Latency) * 0.1
		score -= node.StorageUsage * 0.2

		if score > bestScore {
			bestScore = score
			bestNode = node.ID
		}
	}

	return bestNode
}

func (eg *EdgeGateway) evictCache(neededSize int64) {
	// LRU 驱逐策略
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range eg.cache.entries {
		if oldestKey == "" || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
		}
	}

	if oldestKey != "" {
		entry := eg.cache.entries[oldestKey]
		eg.cache.currentSize -= entry.Size
		delete(eg.cache.entries, oldestKey)
	}
}

func (eg *EdgeGateway) healthCheckLoop() {
	ticker := time.NewTicker(time.Duration(eg.config.HealthCheckSec) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		eg.performHealthCheck()
	}
}

func (eg *EdgeGateway) performHealthCheck() {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	now := time.Now()
	for _, node := range eg.nodes {
		// 检查节点是否超时
		if now.Sub(node.LastSeen) > time.Duration(eg.config.HealthCheckSec*3)*time.Second {
			node.Status = StatusOffline
		}
	}
}

// SelectNodeForTask 为任务选择最佳节点
func (eg *EdgeGateway) SelectNodeForTask(taskType string, policy EdgePolicy) (*EdgeNode, error) {
	eg.mu.RLock()
	defer eg.mu.RUnlock()

	var candidates []*EdgeNode
	for _, node := range eg.nodes {
		if node.Status == StatusOnline {
			candidates = append(candidates, node)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	// 根据策略选择
	switch policy {
	case PolicyLatency:
		return eg.selectLowestLatency(candidates), nil
	case PolicyCostOptimal:
		return eg.selectLowestCost(candidates), nil
	default:
		return eg.selectBalanced(candidates), nil
	}
}

func (eg *EdgeGateway) selectLowestLatency(nodes []*EdgeNode) *EdgeNode {
	best := nodes[0]
	for _, node := range nodes[1:] {
		if node.Latency < best.Latency {
			best = node
		}
	}
	return best
}

func (eg *EdgeGateway) selectLowestCost(nodes []*EdgeNode) *EdgeNode {
	best := nodes[0]
	bestCost := best.CPUUsage + best.MemoryUsage + best.StorageUsage
	for _, node := range nodes[1:] {
		cost := node.CPUUsage + node.MemoryUsage + node.StorageUsage
		if cost < bestCost {
			best = node
			bestCost = cost
		}
	}
	return best
}

func (eg *EdgeGateway) selectBalanced(nodes []*EdgeNode) *EdgeNode {
	best := nodes[0]
	bestScore := 100.0 - (best.CPUUsage*0.3 + best.MemoryUsage*0.3 + best.StorageUsage*0.2 + float64(best.Latency)*0.1)
	for _, node := range nodes[1:] {
		score := 100.0 - (node.CPUUsage*0.3 + node.MemoryUsage*0.3 + node.StorageUsage*0.2 + float64(node.Latency)*0.1)
		if score > bestScore {
			best = node
			bestScore = score
		}
	}
	return best
}

// GetOnlineNodes 获取在线节点列表
func (eg *EdgeGateway) GetOnlineNodes() []*EdgeNode {
	eg.mu.RLock()
	defer eg.mu.RUnlock()

	var nodes []*EdgeNode
	for _, node := range eg.nodes {
		if node.Status == StatusOnline {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetNodesByType 按类型获取节点
func (eg *EdgeGateway) GetNodesByType(nodeType EdgeNodeType) []*EdgeNode {
	eg.mu.RLock()
	defer eg.mu.RUnlock()

	var nodes []*EdgeNode
	for _, node := range eg.nodes {
		if node.Type == nodeType {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
