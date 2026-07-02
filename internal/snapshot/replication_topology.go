// Package snapshot 提供快照复制拓扑功能
// 对标群晖Snapshot Replication灵活拓扑
package snapshot

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ========== 拓扑类型定义 ==========

// TopologyType 拓扑类型.
type TopologyType string

const (
	// TopologyActiveActive 双活复制 - 两个节点同时活跃.
	TopologyActiveActive TopologyType = "active_active"
	// TopologyHubToSpoke 中心到边缘 - 中心节点向多个边缘节点复制.
	TopologyHubToSpoke TopologyType = "hub_to_spoke"
	// TopologyOneToMany 一对多复制 - 单源向多目标复制.
	TopologyOneToMany TopologyType = "one_to_many"
	// TopologyExtended 级联复制 - A→B→C链式复制.
	TopologyExtended TopologyType = "extended"
)

// TopologyConfig 拓扑配置.
type TopologyConfig struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Type            TopologyType        `json:"type"`
	PrimaryNode     *TopologyNode       `json:"primaryNode,omitempty"`
	SecondaryNodes  []*TopologyNode     `json:"secondaryNodes,omitempty"`
	CascadeOrder    []string            `json:"cascadeOrder,omitempty"` // 级联顺序
	Encrypt         bool                `json:"encrypt"`
	EncryptionKey   string              `json:"encryptionKey,omitempty"`
	BandwidthLimit  int                 `json:"bandwidthLimit"` // MB/s
	RetentionPolicy *GFSRetentionPolicy `json:"retentionPolicy"`
	Enabled         bool                `json:"enabled"`
	CreatedAt       time.Time           `json:"createdAt"`
	UpdatedAt       time.Time           `json:"updatedAt"`
}

// TopologyNode 拓扑节点.
type TopologyNode struct {
	NodeID     string     `json:"nodeId"`
	Name       string     `json:"name"`
	Address    string     `json:"address"`
	Port       int        `json:"port"`
	APIKey     string     `json:"apiKey"`
	VolumeName string     `json:"volumeName"`
	Role       NodeRole   `json:"role"`
	Status     NodeStatus `json:"status"`
	LastSync   *time.Time `json:"lastSync,omitempty"`
	SyncCount  int        `json:"syncCount"`
	LatencyMs  int        `json:"latencyMs"` // 网络延迟(ms)
}

// NodeRole 节点角色.
type NodeRole string

const (
	NodeRolePrimary   NodeRole = "primary"
	NodeRoleSecondary NodeRole = "secondary"
	NodeRoleHub       NodeRole = "hub"
	NodeRoleSpoke     NodeRole = "spoke"
	NodeRoleCascade   NodeRole = "cascade"
)

// ========== Grandfather-Father-Son保留策略 ==========

// GFSRetentionPolicy GFS保留策略.
type GFSRetentionPolicy struct {
	// Grandfather (月备份)
	GrandfatherRetention int `json:"grandfatherRetention"` // 保留月数
	GrandfatherDay       int `json:"grandfatherDay"`       // 每月第几天执行

	// Father (周备份)
	FatherRetention int `json:"fatherRetention"` // 保留周数
	FatherDay       int `json:"fatherDay"`       // 每周第几天执行 (0=周日)

	// Son (日备份)
	SonRetention int `json:"sonRetention"` // 保留天数

	// Hourly (小时备份)
	HourlyRetention int `json:"hourlyRetention"` // 保留小时数

	// Manual (手动快照)
	ManualRetention int `json:"manualRetention"` // 保留数量
}

// DefaultGFSPolicy 默认GFS策略.
func DefaultGFSPolicy() *GFSRetentionPolicy {
	return &GFSRetentionPolicy{
		GrandfatherRetention: 12, // 保留12个月
		GrandfatherDay:       1,  // 每月1号
		FatherRetention:      8,  // 保留8周
		FatherDay:            0,  // 每周日
		SonRetention:         31, // 保留31天
		HourlyRetention:      24, // 保留24小时
		ManualRetention:      10, // 保留10个手动快照
	}
}

// GFSClassification GFS分类.
type GFSClassification string

const (
	GFSGrandfather GFSClassification = "grandfather"
	GFSFather      GFSClassification = "father"
	GFSSon         GFSClassification = "son"
	GFSHourly      GFSClassification = "hourly"
	GFSManual      GFSClassification = "manual"
)

// ClassifySnapshot 分类快照到GFS层级.
func (p *GFSRetentionPolicy) ClassifySnapshot(snapshotTime time.Time, isManual bool) GFSClassification {
	if isManual {
		return GFSManual
	}

	// 检查是否为月备份 (Grandfather)
	if snapshotTime.Day() == p.GrandfatherDay {
		return GFSGrandfather
	}

	// 检查是否为周备份 (Father)
	if int(snapshotTime.Weekday()) == p.FatherDay {
		return GFSFather
	}

	// 检查是否为小时备份
	if snapshotTime.Minute() < 5 { // 近整点时间
		return GFSHourly
	}

	// 默认为日备份 (Son)
	return GFSSon
}

// ========== 拓扑管理器 ==========

// TopologyManager 拓扑管理器.
type TopologyManager struct {
	mu       sync.RWMutex
	configs  map[string]*TopologyConfig
	statuses map[string]*TopologyStatus
	client   *http.Client
}

// TopologyStatus 拓扑状态.
type TopologyStatus struct {
	ConfigID      string                       `json:"configId"`
	OverallStatus string                       `json:"overallStatus"` // healthy, degraded, failed
	NodeStatuses  map[string]*NodeStatusDetail `json:"nodeStatuses"`
	LastFullSync  *time.Time                   `json:"lastFullSync,omitempty"`
	PendingSync   int                          `json:"pendingSync"` // 待同步快照数
	TotalSynced   int                          `json:"totalSynced"`
	Errors        []TopologyError              `json:"errors,omitempty"`
}

// NodeStatusDetail 节点状态详情.
type NodeStatusDetail struct {
	NodeID       string    `json:"nodeId"`
	Status       string    `json:"status"` // online, offline, syncing, error
	LastSync     time.Time `json:"lastSync"`
	SyncProgress float64   `json:"syncProgress"` // 0-100
	BytesSynced  uint64    `json:"bytesSynced"`
	LatencyMs    int       `json:"latencyMs"`
	LastError    string    `json:"lastError,omitempty"`
}

// TopologyError 拓扑错误.
type TopologyError struct {
	Timestamp time.Time `json:"timestamp"`
	NodeID    string    `json:"nodeId"`
	Error     string    `json:"error"`
	Severity  string    `json:"severity"` // warning, critical
}

// NewTopologyManager 创建拓扑管理器.
func NewTopologyManager() *TopologyManager {
	return &TopologyManager{
		configs:  make(map[string]*TopologyConfig),
		statuses: make(map[string]*TopologyStatus),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ========== 拓扑配置管理 ==========

// CreateTopology 创建拓扑配置.
func (m *TopologyManager) CreateTopology(config *TopologyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = generateTopologyID()
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	// 验证拓扑配置
	if err := m.validateTopology(config); err != nil {
		return err
	}

	m.configs[config.ID] = config

	// 初始化状态
	m.statuses[config.ID] = &TopologyStatus{
		ConfigID:      config.ID,
		OverallStatus: "initializing",
		NodeStatuses:  make(map[string]*NodeStatusDetail),
		PendingSync:   0,
		TotalSynced:   0,
	}

	return nil
}

// GetTopology 获取拓扑配置.
func (m *TopologyManager) GetTopology(id string) (*TopologyConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[id]
	if !exists {
		return nil, fmt.Errorf("拓扑配置 %s 不存在", id)
	}

	return config, nil
}

// ListTopologies 列出拓扑配置.
func (m *TopologyManager) ListTopologies() []*TopologyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TopologyConfig, 0, len(m.configs))
	for _, config := range m.configs {
		result = append(result, config)
	}

	return result
}

// UpdateTopology 更新拓扑配置.
func (m *TopologyManager) UpdateTopology(id string, updates *TopologyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.configs[id]
	if !exists {
		return fmt.Errorf("拓扑配置 %s 不存在", id)
	}

	// 应用更新
	if updates.Name != "" {
		config.Name = updates.Name
	}
	if updates.Type != "" {
		config.Type = updates.Type
	}
	if updates.SecondaryNodes != nil {
		config.SecondaryNodes = updates.SecondaryNodes
	}
	if updates.Encrypt != config.Encrypt {
		config.Encrypt = updates.Encrypt
	}
	if updates.BandwidthLimit > 0 {
		config.BandwidthLimit = updates.BandwidthLimit
	}
	if updates.RetentionPolicy != nil {
		config.RetentionPolicy = updates.RetentionPolicy
	}
	config.Enabled = updates.Enabled
	config.UpdatedAt = time.Now()

	// 验证更新后的配置
	if err := m.validateTopology(config); err != nil {
		return err
	}

	return nil
}

// DeleteTopology 删除拓扑配置.
func (m *TopologyManager) DeleteTopology(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[id]; !exists {
		return fmt.Errorf("拓扑配置 %s 不存在", id)
	}

	delete(m.configs, id)
	delete(m.statuses, id)

	return nil
}

// validateTopology 验证拓扑配置.
func (m *TopologyManager) validateTopology(config *TopologyConfig) error {
	switch config.Type {
	case TopologyActiveActive:
		// 双活需要至少2个节点
		if len(config.SecondaryNodes) < 1 {
			return fmt.Errorf("双活拓扑至少需要2个节点")
		}
	case TopologyHubToSpoke:
		// Hub到Spoke需要Hub节点和至少1个Spoke
		if config.PrimaryNode == nil {
			return fmt.Errorf("Hub到Spoke拓扑需要Hub节点")
		}
		if len(config.SecondaryNodes) < 1 {
			return fmt.Errorf("Hub到Spoke拓扑至少需要1个Spoke节点")
		}
	case TopologyOneToMany:
		// 一对多需要主节点和至少1个目标
		if config.PrimaryNode == nil {
			return fmt.Errorf("一对多拓扑需要主节点")
		}
		if len(config.SecondaryNodes) < 1 {
			return fmt.Errorf("一对多拓扑至少需要1个目标节点")
		}
	case TopologyExtended:
		// 级联需要定义级联顺序
		if len(config.CascadeOrder) < 2 {
			return fmt.Errorf("级联拓扑需要至少2个节点的级联顺序")
		}
	default:
		return fmt.Errorf("未知的拓扑类型: %s", config.Type)
	}

	return nil
}

// ========== 复制任务执行 ==========

// ReplicationTask 复制任务.
type ReplicationTask struct {
	ID           string            `json:"id"`
	ConfigID     string            `json:"configId"`
	SnapshotID   string            `json:"snapshotId"`
	SnapshotTime time.Time         `json:"snapshotTime"`
	GFSClass     GFSClassification `json:"gfsClass"`
	SourceNode   string            `json:"sourceNode"`
	TargetNodes  []string          `json:"targetNodes"`
	Status       TaskStatus        `json:"status"`
	Progress     float64           `json:"progress"`
	StartTime    time.Time         `json:"startTime"`
	EndTime      *time.Time        `json:"endTime,omitempty"`
	BytesTotal   uint64            `json:"bytesTotal"`
	BytesSynced  uint64            `json:"bytesSynced"`
	Errors       []TaskError       `json:"errors,omitempty"`
}

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskError 任务错误.
type TaskError struct {
	NodeID    string    `json:"nodeId"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// CreateReplicationTask 创建复制任务.
func (m *TopologyManager) CreateReplicationTask(ctx context.Context, configID, snapshotID string) (*ReplicationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.configs[configID]
	if !exists {
		return nil, fmt.Errorf("拓扑配置 %s 不存在", configID)
	}

	if !config.Enabled {
		return nil, fmt.Errorf("拓扑配置 %s 未启用", configID)
	}

	task := &ReplicationTask{
		ID:           generateTaskID(),
		ConfigID:     configID,
		SnapshotID:   snapshotID,
		SnapshotTime: time.Now(),
		SourceNode:   config.PrimaryNode.NodeID,
		TargetNodes:  m.getTargetNodes(config),
		Status:       TaskStatusPending,
		Progress:     0,
		StartTime:    time.Now(),
	}

	return task, nil
}

// getTargetNodes 获取目标节点列表.
func (m *TopologyManager) getTargetNodes(config *TopologyConfig) []string {
	targets := make([]string, 0)

	switch config.Type {
	case TopologyActiveActive:
		// 双活: 所有节点都是目标
		for _, node := range config.SecondaryNodes {
			targets = append(targets, node.NodeID)
		}
	case TopologyHubToSpoke:
		// Hub到Spoke: Spoke节点是目标
		for _, node := range config.SecondaryNodes {
			targets = append(targets, node.NodeID)
		}
	case TopologyOneToMany:
		// 一对多: Secondary节点是目标
		for _, node := range config.SecondaryNodes {
			targets = append(targets, node.NodeID)
		}
	case TopologyExtended:
		// 级联: 按级联顺序确定目标
		targets = config.CascadeOrder[1:] // 第一个节点是源，其余是目标
	}

	return targets
}

// ExecuteReplicationTask 执行复制任务.
func (m *TopologyManager) ExecuteReplicationTask(ctx context.Context, task *ReplicationTask, snapshotData []byte) error {
	m.mu.Lock()
	config, exists := m.configs[task.ConfigID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("拓扑配置 %s 不存在", task.ConfigID)
	}
	m.mu.Unlock()

	task.Status = TaskStatusRunning
	task.BytesTotal = uint64(len(snapshotData))

	// 准备传输数据
	var dataToSend []byte
	var err error

	if config.Encrypt {
		dataToSend, err = m.encryptData(snapshotData, config.EncryptionKey)
		if err != nil {
			task.Status = TaskStatusFailed
			task.Errors = append(task.Errors, TaskError{
				Error:     fmt.Sprintf("加密失败: %v", err),
				Timestamp: time.Now(),
			})
			return err
		}
	} else {
		dataToSend = snapshotData
	}

	// 并发复制到目标节点
	var wg sync.WaitGroup
	var taskErrors []TaskError
	var bytesSynced uint64

	for _, targetNodeID := range task.TargetNodes {
		wg.Add(1)
		go func(nodeID string) {
			defer wg.Done()

			node := m.findNode(config, nodeID)
			if node == nil {
				taskErrors = append(taskErrors, TaskError{
					NodeID:    nodeID,
					Error:     "节点未找到",
					Timestamp: time.Now(),
				})
				return
			}

			if err := m.sendToNode(ctx, node, task.SnapshotID, dataToSend, config.BandwidthLimit); err != nil {
				taskErrors = append(taskErrors, TaskError{
					NodeID:    nodeID,
					Error:     err.Error(),
					Timestamp: time.Now(),
				})
				return
			}

			bytesSynced += task.BytesTotal

			// 更新节点状态
			m.updateNodeStatus(config.ID, nodeID, "synced")
		}(targetNodeID)
	}

	wg.Wait()

	task.BytesSynced = bytesSynced
	task.Errors = taskErrors

	if len(taskErrors) == 0 {
		task.Status = TaskStatusCompleted
		task.Progress = 100
		now := time.Now()
		task.EndTime = &now
	} else if len(taskErrors) < len(task.TargetNodes) {
		task.Status = TaskStatusCompleted
		task.Progress = float64(bytesSynced) / float64(task.BytesTotal*uint64(len(task.TargetNodes))) * 100
		now := time.Now()
		task.EndTime = &now
	} else {
		task.Status = TaskStatusFailed
	}

	return nil
}

// findNode 查找节点.
func (m *TopologyManager) findNode(config *TopologyConfig, nodeID string) *TopologyNode {
	if config.PrimaryNode != nil && config.PrimaryNode.NodeID == nodeID {
		return config.PrimaryNode
	}

	for _, node := range config.SecondaryNodes {
		if node.NodeID == nodeID {
			return node
		}
	}

	return nil
}

// ========== 加密复制 ==========

// encryptData 加密数据 (AES-256-GCM).
func (m *TopologyManager) encryptData(data []byte, key string) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("加密密钥长度必须为32字节")
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return encrypted, nil
}

// decryptData 解密数据.
func (m *TopologyManager) decryptData(encrypted []byte, key string) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("加密密钥长度必须为32字节")
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, fmt.Errorf("加密数据太短")
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}

// ========== 网络传输 ==========

// sendToNode 发送数据到节点.
func (m *TopologyManager) sendToNode(ctx context.Context, node *TopologyNode, snapshotID string, data []byte, bandwidthLimit int) error {
	// 构建请求URL
	url := fmt.Sprintf("http://%s:%d/api/v1/snapshot/receive", node.Address, node.Port)

	// 准备请求体
	reqBody := map[string]interface{}{
		"snapshotId": snapshotID,
		"data":       data,
		"encrypted":  true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+node.APIKey)

	// 应用带宽限制提示，由接收端或上层传输器按该头部限速。
	if bandwidthLimit > 0 {
		req.Header.Set("X-Bandwidth-Limit", fmt.Sprintf("%d", bandwidthLimit))
	}

	// 发送请求
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("节点返回错误: %d", resp.StatusCode)
	}

	return nil
}

// ========== 带宽限制 ==========

// ThrottledTransport 带宽限制传输.
type ThrottledTransport struct {
	transport    *http.Transport
	bytesPerSec  int64 // 每秒字节数
	currentBytes int64
	lastReset    time.Time
}

// NewThrottledTransport 创建带宽限制传输.
func NewThrottledTransport(bytesPerSec int64) *ThrottledTransport {
	return &ThrottledTransport{
		transport:   &http.Transport{},
		bytesPerSec: bytesPerSec,
		lastReset:   time.Now(),
	}
}

// ========== 状态监控 ==========

// GetTopologyStatus 获取拓扑状态.
func (m *TopologyManager) GetTopologyStatus(id string) (*TopologyStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.statuses[id]
	if !exists {
		return nil, fmt.Errorf("拓扑状态 %s 不存在", id)
	}

	return status, nil
}

// updateNodeStatus 更新节点状态.
func (m *TopologyManager) updateNodeStatus(configID, nodeID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	topologyStatus, exists := m.statuses[configID]
	if !exists {
		return
	}

	now := time.Now()
	detail := &NodeStatusDetail{
		NodeID:   nodeID,
		Status:   status,
		LastSync: now,
	}

	topologyStatus.NodeStatuses[nodeID] = detail
	topologyStatus.TotalSynced++

	// 计算整体状态
	m.calculateOverallStatus(topologyStatus)
}

// calculateOverallStatus 计算整体状态.
func (m *TopologyManager) calculateOverallStatus(status *TopologyStatus) {
	healthyCount := 0
	offlineCount := 0
	errorCount := 0

	for _, detail := range status.NodeStatuses {
		switch detail.Status {
		case "online", "synced":
			healthyCount++
		case "offline":
			offlineCount++
		case "error":
			errorCount++
		}
	}

	total := len(status.NodeStatuses)
	if total == 0 {
		status.OverallStatus = "initializing"
		return
	}

	if errorCount > total/2 {
		status.OverallStatus = "failed"
	} else if offlineCount > 0 || errorCount > 0 {
		status.OverallStatus = "degraded"
	} else {
		status.OverallStatus = "healthy"
	}
}

// CheckNodeHealth 检查节点健康状态.
func (m *TopologyManager) CheckNodeHealth(ctx context.Context, node *TopologyNode) error {
	url := fmt.Sprintf("http://%s:%d/api/v1/health", node.Address, node.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+node.APIKey)

	start := time.Now()
	resp, err := m.client.Do(req)
	if err != nil {
		node.Status = NodeStatusOffline
		return err
	}
	defer resp.Body.Close()

	node.LatencyMs = int(time.Since(start).Milliseconds())

	if resp.StatusCode == http.StatusOK {
		node.Status = NodeStatusOnline
		return nil
	}

	node.Status = NodeStatusError
	return fmt.Errorf("节点健康检查失败: %d", resp.StatusCode)
}

// ========== ID生成 ==========

func generateTopologyID() string {
	return fmt.Sprintf("topo-%d", time.Now().UnixNano())
}

func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

// ========== API响应结构 ==========

// TopologyAPIResponse API响应.
type TopologyAPIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ReplicationTaskListResponse 复制任务列表响应.
type ReplicationTaskListResponse struct {
	Total int                `json:"total"`
	Tasks []*ReplicationTask `json:"tasks"`
}

// TopologyListResponse 拓扑列表响应.
type TopologyListResponse struct {
	Total      int               `json:"total"`
	Topologies []*TopologyConfig `json:"topologies"`
}
