package multinasmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NAS节点状态定义.
const (
	// NodeStatusOnline 表示节点在线.
	NodeStatusOnline = "online"
	// NodeStatusOffline 表示节点离线.
	NodeStatusOffline = "offline"
	// NodeStatusDegraded 表示节点降级.
	NodeStatusDegraded = "degraded"
)

// 告警级别定义.
const (
	// AlertLevelInfo 信息级别告警.
	AlertLevelInfo = "info"
	// AlertLevelWarning 警告级别告警.
	AlertLevelWarning = "warning"
	// AlertLevelCritical 严重级别告警.
	AlertLevelCritical = "critical"
)

// 迁移任务状态定义.
const (
	// MigrationStatusPending 等待执行.
	MigrationStatusPending = "pending"
	// MigrationStatusRunning 执行中.
	MigrationStatusRunning = "running"
	// MigrationStatusCompleted 已完成.
	MigrationStatusCompleted = "completed"
	// MigrationStatusFailed 执行失败.
	MigrationStatusFailed = "failed"
)

// NASNode NAS节点信息.
type NASNode struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Hostname      string            `json:"hostname"`
	IP            string            `json:"ip"`
	Port          int               `json:"port"`
	Status        string            `json:"status"`
	OSVersion     string            `json:"os_version"`
	TotalStorage  int64             `json:"total_storage"`
	UsedStorage   int64             `json:"used_storage"`
	FreeStorage   int64             `json:"free_storage"`
	CPUUsage      float64           `json:"cpu_usage"`
	MemoryUsage   float64           `json:"memory_usage"`
	Uptime        int64             `json:"uptime"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	RegisteredAt  time.Time         `json:"registered_at"`
	Tags          map[string]string `json:"tags"`
}

// StoragePool 存储池信息.
type StoragePool struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	NodeID       string   `json:"node_id"`
	NodeName     string   `json:"node_name"`
	TotalSize    int64    `json:"total_size"`
	UsedSize     int64    `json:"used_size"`
	FreeSize     int64    `json:"free_size"`
	Health       string   `json:"health"`
	RaidLevel    string   `json:"raid_level"`
	Disks        []string `json:"disks"`
	MountPoint   string   `json:"mount_point"`
	IsAggregated bool     `json:"is_aggregated"`
}

// Alert 告警信息.
type Alert struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name"`
	Level     string    `json:"level"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
	Acked     bool      `json:"acked"`
	AckedBy   string    `json:"acked_by,omitempty"`
	AckedAt   time.Time `json:"acked_at,omitempty"`
}

// Event 系统事件.
type Event struct {
	ID        string                 `json:"id"`
	NodeID    string                 `json:"node_id"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// MigrationTask 数据迁移任务.
type MigrationTask struct {
	ID           string    `json:"id"`
	SourceNodeID string    `json:"source_node_id"`
	TargetNodeID string    `json:"target_node_id"`
	SourcePath   string    `json:"source_path"`
	TargetPath   string    `json:"target_path"`
	TotalBytes   int64     `json:"total_bytes"`
	CopiedBytes  int64     `json:"copied_bytes"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
}

// ClusterTopology 集群拓扑信息.
type ClusterTopology struct {
	LeaderID    string     `json:"leader_id"`
	TotalNodes  int        `json:"total_nodes"`
	OnlineNodes int        `json:"online_nodes"`
	Nodes       []*NASNode `json:"nodes"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Config 多NAS管理器配置.
type Config struct {
	NodeID            string `json:"node_id"`
	Name              string `json:"name"`
	HeartbeatInterval int    `json:"heartbeat_interval"` // 秒
	HeartbeatTimeout  int    `json:"heartbeat_timeout"`  // 秒
	DataDir           string `json:"data_dir"`
	MaxAlerts         int    `json:"max_alerts"`
	MaxEvents         int    `json:"max_events"`
}

// Manager 多NAS统一管理器.
type Manager struct {
	config     Config
	nodes      map[string]*NASNode
	pools      map[string]*StoragePool
	alerts     []*Alert
	events     []*Event
	migrations map[string]*MigrationTask
	topology   *ClusterTopology
	leaderID   string

	nodesMu      sync.RWMutex
	poolsMu      sync.RWMutex
	alertsMu     sync.RWMutex
	eventsMu     sync.RWMutex
	migrationsMu sync.RWMutex
	topologyMu   sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger
}

// NewManager 创建多NAS管理器.
func NewManager(config Config, logger *zap.Logger) (*Manager, error) {
	if config.NodeID == "" {
		hostname, _ := os.Hostname()
		config.NodeID = fmt.Sprintf("nas-%s", hostname)
	}
	if config.Name == "" {
		config.Name = "MultiNAS Cluster"
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 10
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 30
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/multinas"
	}
	if config.MaxAlerts == 0 {
		config.MaxAlerts = 1000
	}
	if config.MaxEvents == 0 {
		config.MaxEvents = 5000
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:     config,
		nodes:      make(map[string]*NASNode),
		pools:      make(map[string]*StoragePool),
		alerts:     make([]*Alert, 0),
		events:     make([]*Event, 0),
		migrations: make(map[string]*MigrationTask),
		topology: &ClusterTopology{
			LeaderID: config.NodeID,
		},
		leaderID: config.NodeID,
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
	}

	// 创建数据目录.
	if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败：%w", err)
	}

	// 加载持久化数据.
	if err := m.loadData(); err != nil {
		logger.Warn("加载持久化数据失败", zap.Error(err))
	}

	return m, nil
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.logger.Info("启动多NAS管理器", zap.String("node_id", m.config.NodeID))

	// 启动心跳检测.
	go m.heartbeatMonitor()

	// 启动拓扑更新.
	go m.topologyUpdater()

	return nil
}

// Shutdown 关闭管理器.
func (m *Manager) Shutdown() {
	m.logger.Info("关闭多NAS管理器")
	m.cancel()
	m.saveData()
}

// RegisterNode 注册NAS节点.
func (m *Manager) RegisterNode(node *NASNode) error {
	if node.ID == "" {
		node.ID = uuid.New().String()
	}
	if node.RegisteredAt.IsZero() {
		node.RegisteredAt = time.Now()
	}
	if node.LastHeartbeat.IsZero() {
		node.LastHeartbeat = time.Now()
	}
	if node.Tags == nil {
		node.Tags = make(map[string]string)
	}

	m.nodesMu.Lock()
	m.nodes[node.ID] = node
	m.nodesMu.Unlock()

	m.logger.Info("注册NAS节点",
		zap.String("node_id", node.ID),
		zap.String("name", node.Name),
		zap.String("ip", node.IP),
	)

	m.addEvent(node.ID, "node_registered", fmt.Sprintf("节点 %s 已注册", node.Name))
	return nil
}

// UnregisterNode 注销NAS节点.
func (m *Manager) UnregisterNode(nodeID string) error {
	m.nodesMu.Lock()
	node, exists := m.nodes[nodeID]
	if !exists {
		m.nodesMu.Unlock()
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}
	delete(m.nodes, nodeID)
	m.nodesMu.Unlock()

	m.logger.Info("注销NAS节点", zap.String("node_id", nodeID))
	m.addEvent(nodeID, "node_unregistered", fmt.Sprintf("节点 %s 已注销", node.Name))
	return nil
}

// GetNode 获取节点信息.
func (m *Manager) GetNode(nodeID string) (*NASNode, error) {
	m.nodesMu.RLock()
	defer m.nodesMu.RUnlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点 %s 不存在", nodeID)
	}
	return node, nil
}

// GetNodes 获取所有节点列表.
func (m *Manager) GetNodes() []*NASNode {
	m.nodesMu.RLock()
	defer m.nodesMu.RUnlock()

	nodes := make([]*NASNode, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// UpdateNodeStatus 更新节点状态.
func (m *Manager) UpdateNodeStatus(nodeID, status string) error {
	m.nodesMu.Lock()
	defer m.nodesMu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	oldStatus := node.Status
	node.Status = status
	node.LastHeartbeat = time.Now()

	if oldStatus != status {
		m.addEvent(nodeID, "status_changed",
			fmt.Sprintf("节点 %s 状态从 %s 变为 %s", node.Name, oldStatus, status))

		if status == NodeStatusOffline {
			m.addAlert(nodeID, node.Name, AlertLevelWarning, "node_offline",
				fmt.Sprintf("节点 %s 已离线", node.Name))
		}
	}

	return nil
}

// UpdateNodeMetrics 更新节点性能指标.
func (m *Manager) UpdateNodeMetrics(nodeID string, cpuUsage, memoryUsage float64, usedStorage int64) error {
	m.nodesMu.Lock()
	defer m.nodesMu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	node.CPUUsage = cpuUsage
	node.MemoryUsage = memoryUsage
	node.UsedStorage = usedStorage
	node.FreeStorage = node.TotalStorage - usedStorage
	node.LastHeartbeat = time.Now()

	// 检查资源使用是否过高.
	if cpuUsage > 90 {
		m.addAlert(nodeID, node.Name, AlertLevelWarning, "high_cpu",
			fmt.Sprintf("节点 %s CPU使用率 %.1f%%", node.Name, cpuUsage))
	}
	if memoryUsage > 90 {
		m.addAlert(nodeID, node.Name, AlertLevelWarning, "high_memory",
			fmt.Sprintf("节点 %s 内存使用率 %.1f%%", node.Name, memoryUsage))
	}
	if node.TotalStorage > 0 && float64(usedStorage)/float64(node.TotalStorage) > 0.9 {
		m.addAlert(nodeID, node.Name, AlertLevelWarning, "low_storage",
			fmt.Sprintf("节点 %s 存储空间不足", node.Name))
	}

	return nil
}

// RegisterPool 注册存储池.
func (m *Manager) RegisterPool(pool *StoragePool) error {
	if pool.ID == "" {
		pool.ID = uuid.New().String()
	}

	m.poolsMu.Lock()
	m.pools[pool.ID] = pool
	m.poolsMu.Unlock()

	m.logger.Info("注册存储池",
		zap.String("pool_id", pool.ID),
		zap.String("name", pool.Name),
		zap.String("node_id", pool.NodeID),
	)

	m.addEvent(pool.NodeID, "pool_registered", fmt.Sprintf("存储池 %s 已注册", pool.Name))
	return nil
}

// UnregisterPool 注销存储池.
func (m *Manager) UnregisterPool(poolID string) error {
	m.poolsMu.Lock()
	pool, exists := m.pools[poolID]
	if !exists {
		m.poolsMu.Unlock()
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}
	delete(m.pools, poolID)
	m.poolsMu.Unlock()

	m.addEvent(pool.NodeID, "pool_unregistered", fmt.Sprintf("存储池 %s 已注销", pool.Name))
	return nil
}

// GetPool 获取存储池信息.
func (m *Manager) GetPool(poolID string) (*StoragePool, error) {
	m.poolsMu.RLock()
	defer m.poolsMu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}
	return pool, nil
}

// GetPools 获取所有存储池.
func (m *Manager) GetPools() []*StoragePool {
	m.poolsMu.RLock()
	defer m.poolsMu.RUnlock()

	pools := make([]*StoragePool, 0, len(m.pools))
	for _, pool := range m.pools {
		pools = append(pools, pool)
	}
	return pools
}

// GetAggregatedPools 获取聚合视图的存储池.
func (m *Manager) GetAggregatedPools() []*StoragePool {
	m.poolsMu.RLock()
	defer m.poolsMu.RUnlock()

	// 按名称聚合存储池.
	poolMap := make(map[string]*StoragePool)
	for _, pool := range m.pools {
		if existing, ok := poolMap[pool.Name]; ok {
			existing.TotalSize += pool.TotalSize
			existing.UsedSize += pool.UsedSize
			existing.FreeSize += pool.FreeSize
			existing.IsAggregated = true
		} else {
			// 创建副本.
			copy := *pool
			poolMap[pool.Name] = &copy
		}
	}

	pools := make([]*StoragePool, 0, len(poolMap))
	for _, pool := range poolMap {
		pools = append(pools, pool)
	}
	return pools
}

// GetAlerts 获取告警列表.
func (m *Manager) GetAlerts(level string, acked *bool, limit int) []*Alert {
	m.alertsMu.RLock()
	defer m.alertsMu.RUnlock()

	var result []*Alert
	for i := len(m.alerts) - 1; i >= 0; i-- {
		alert := m.alerts[i]
		if level != "" && alert.Level != level {
			continue
		}
		if acked != nil && alert.Acked != *acked {
			continue
		}
		result = append(result, alert)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// AckAlert 确认告警.
func (m *Manager) AckAlert(alertID, ackedBy string) error {
	m.alertsMu.Lock()
	defer m.alertsMu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == alertID {
			alert.Acked = true
			alert.AckedBy = ackedBy
			alert.AckedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// GetEvents 获取事件列表.
func (m *Manager) GetEvents(nodeID, eventType string, limit int) []*Event {
	m.eventsMu.RLock()
	defer m.eventsMu.RUnlock()

	var result []*Event
	for i := len(m.events) - 1; i >= 0; i-- {
		event := m.events[i]
		if nodeID != "" && event.NodeID != nodeID {
			continue
		}
		if eventType != "" && event.Type != eventType {
			continue
		}
		result = append(result, event)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// CreateMigration 创建迁移任务.
func (m *Manager) CreateMigration(sourceNodeID, targetNodeID, sourcePath, targetPath string, totalBytes int64) (*MigrationTask, error) {
	// 验证节点存在.
	m.nodesMu.RLock()
	_, sourceExists := m.nodes[sourceNodeID]
	_, targetExists := m.nodes[targetNodeID]
	m.nodesMu.RUnlock()

	if !sourceExists {
		return nil, fmt.Errorf("源节点 %s 不存在", sourceNodeID)
	}
	if !targetExists {
		return nil, fmt.Errorf("目标节点 %s 不存在", targetNodeID)
	}

	task := &MigrationTask{
		ID:           uuid.New().String(),
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		SourcePath:   sourcePath,
		TargetPath:   targetPath,
		TotalBytes:   totalBytes,
		Status:       MigrationStatusPending,
		StartedAt:    time.Now(),
	}

	m.migrationsMu.Lock()
	m.migrations[task.ID] = task
	m.migrationsMu.Unlock()

	m.logger.Info("创建迁移任务",
		zap.String("task_id", task.ID),
		zap.String("source", sourceNodeID),
		zap.String("target", targetNodeID),
	)

	m.addEvent(sourceNodeID, "migration_created",
		fmt.Sprintf("迁移任务 %s 已创建: %s -> %s", task.ID[:8], sourceNodeID, targetNodeID))

	return task, nil
}

// UpdateMigrationProgress 更新迁移进度.
func (m *Manager) UpdateMigrationProgress(taskID string, copiedBytes int64, status string, errMsg string) error {
	m.migrationsMu.Lock()
	defer m.migrationsMu.Unlock()

	task, exists := m.migrations[taskID]
	if !exists {
		return fmt.Errorf("迁移任务 %s 不存在", taskID)
	}

	task.CopiedBytes = copiedBytes
	task.Status = status
	task.Error = errMsg

	if status == MigrationStatusCompleted || status == MigrationStatusFailed {
		task.CompletedAt = time.Now()
	}

	return nil
}

// GetMigration 获取迁移任务.
func (m *Manager) GetMigration(taskID string) (*MigrationTask, error) {
	m.migrationsMu.RLock()
	defer m.migrationsMu.RUnlock()

	task, exists := m.migrations[taskID]
	if !exists {
		return nil, fmt.Errorf("迁移任务 %s 不存在", taskID)
	}
	return task, nil
}

// GetMigrations 获取迁移任务列表.
func (m *Manager) GetMigrations(status string) []*MigrationTask {
	m.migrationsMu.RLock()
	defer m.migrationsMu.RUnlock()

	var result []*MigrationTask
	for _, task := range m.migrations {
		if status != "" && task.Status != status {
			continue
		}
		result = append(result, task)
	}
	return result
}

// GetTopology 获取集群拓扑.
func (m *Manager) GetTopology() *ClusterTopology {
	m.topologyMu.RLock()
	defer m.topologyMu.RUnlock()

	m.nodesMu.RLock()
	nodes := make([]*NASNode, 0, len(m.nodes))
	onlineCount := 0
	for _, node := range m.nodes {
		nodes = append(nodes, node)
		if node.Status == NodeStatusOnline {
			onlineCount++
		}
	}
	m.nodesMu.RUnlock()

	return &ClusterTopology{
		LeaderID:    m.leaderID,
		TotalNodes:  len(nodes),
		OnlineNodes: onlineCount,
		Nodes:       nodes,
		UpdatedAt:   time.Now(),
	}
}

// SetLeader 设置集群领导节点.
func (m *Manager) SetLeader(nodeID string) {
	m.topologyMu.Lock()
	defer m.topologyMu.Unlock()

	oldLeader := m.leaderID
	m.leaderID = nodeID
	m.topology.LeaderID = nodeID

	if oldLeader != nodeID {
		m.addEvent(nodeID, "leader_changed",
			fmt.Sprintf("集群领导从 %s 变更为 %s", oldLeader, nodeID))
	}
}

// 内部方法.

// addAlert 添加告警.
func (m *Manager) addAlert(nodeID, nodeName, level, alertType, message string) {
	m.alertsMu.Lock()
	defer m.alertsMu.Unlock()

	alert := &Alert{
		ID:        uuid.New().String(),
		NodeID:    nodeID,
		NodeName:  nodeName,
		Level:     level,
		Type:      alertType,
		Message:   message,
		Timestamp: time.Now(),
	}

	m.alerts = append(m.alerts, alert)

	// 超过最大数量时清理旧告警.
	if len(m.alerts) > m.config.MaxAlerts {
		m.alerts = m.alerts[len(m.alerts)-m.config.MaxAlerts:]
	}
}

// addEvent 添加事件.
func (m *Manager) addEvent(nodeID, eventType, message string) {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()

	event := &Event{
		ID:        uuid.New().String(),
		NodeID:    nodeID,
		Type:      eventType,
		Message:   message,
		Timestamp: time.Now(),
	}

	m.events = append(m.events, event)

	// 超过最大数量时清理旧事件.
	if len(m.events) > m.config.MaxEvents {
		m.events = m.events[len(m.events)-m.config.MaxEvents:]
	}
}

// heartbeatMonitor 心跳监控.
func (m *Manager) heartbeatMonitor() {
	ticker := time.NewTicker(time.Duration(m.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkNodeHeartbeats()
		}
	}
}

// checkNodeHeartbeats 检查节点心跳.
func (m *Manager) checkNodeHeartbeats() {
	m.nodesMu.Lock()
	defer m.nodesMu.Unlock()

	timeout := time.Duration(m.config.HeartbeatTimeout) * time.Second
	now := time.Now()

	for _, node := range m.nodes {
		if node.Status == NodeStatusOnline && now.Sub(node.LastHeartbeat) > timeout {
			node.Status = NodeStatusDegraded
			m.logger.Warn("节点心跳超时，标记为降级",
				zap.String("node_id", node.ID),
				zap.Time("last_heartbeat", node.LastHeartbeat),
			)
			m.addEvent(node.ID, "heartbeat_timeout",
				fmt.Sprintf("节点 %s 心跳超时", node.Name))
		}
		if node.Status == NodeStatusDegraded && now.Sub(node.LastHeartbeat) > timeout*2 {
			node.Status = NodeStatusOffline
			m.logger.Warn("节点长时间无响应，标记为离线",
				zap.String("node_id", node.ID),
			)
			m.addAlert(node.ID, node.Name, AlertLevelCritical, "node_offline",
				fmt.Sprintf("节点 %s 已离线", node.Name))
		}
	}
}

// topologyUpdater 拓扑更新.
func (m *Manager) topologyUpdater() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateTopology()
		}
	}
}

// updateTopology 更新拓扑信息.
func (m *Manager) updateTopology() {
	m.topologyMu.Lock()
	defer m.topologyMu.Unlock()

	m.nodesMu.RLock()
	nodes := make([]*NASNode, 0, len(m.nodes))
	onlineCount := 0
	for _, node := range m.nodes {
		nodes = append(nodes, node)
		if node.Status == NodeStatusOnline {
			onlineCount++
		}
	}
	m.nodesMu.RUnlock()

	m.topology = &ClusterTopology{
		LeaderID:    m.leaderID,
		TotalNodes:  len(nodes),
		OnlineNodes: onlineCount,
		Nodes:       nodes,
		UpdatedAt:   time.Now(),
	}
}

// 持久化.

type persistentData struct {
	Nodes      map[string]*NASNode       `json:"nodes"`
	Pools      map[string]*StoragePool   `json:"pools"`
	Migrations map[string]*MigrationTask `json:"migrations"`
	LeaderID   string                    `json:"leader_id"`
}

func (m *Manager) loadData() error {
	dataFile := filepath.Join(m.config.DataDir, "multinas.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var pd persistentData
	if err := json.Unmarshal(data, &pd); err != nil {
		return err
	}

	m.nodesMu.Lock()
	if pd.Nodes != nil {
		m.nodes = pd.Nodes
	}
	m.nodesMu.Unlock()

	m.poolsMu.Lock()
	if pd.Pools != nil {
		m.pools = pd.Pools
	}
	m.poolsMu.Unlock()

	m.migrationsMu.Lock()
	if pd.Migrations != nil {
		m.migrations = pd.Migrations
	}
	m.migrationsMu.Unlock()

	if pd.LeaderID != "" {
		m.leaderID = pd.LeaderID
	}

	return nil
}

func (m *Manager) saveData() error {
	dataFile := filepath.Join(m.config.DataDir, "multinas.json")

	m.nodesMu.RLock()
	nodes := make(map[string]*NASNode)
	for k, v := range m.nodes {
		nodes[k] = v
	}
	m.nodesMu.RUnlock()

	m.poolsMu.RLock()
	pools := make(map[string]*StoragePool)
	for k, v := range m.pools {
		pools[k] = v
	}
	m.poolsMu.RUnlock()

	m.migrationsMu.RLock()
	migrations := make(map[string]*MigrationTask)
	for k, v := range m.migrations {
		migrations[k] = v
	}
	m.migrationsMu.RUnlock()

	pd := persistentData{
		Nodes:      nodes,
		Pools:      pools,
		Migrations: migrations,
		LeaderID:   m.leaderID,
	}

	data, err := json.MarshalIndent(pd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(dataFile, data, 0o644)
}
