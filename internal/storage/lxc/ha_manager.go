// Package lxcstorage 提供 LXC 容器与存储集成的 HA 管理
// 参考 TrueNAS 26 实现，支持容器故障转移和高可用
package lxcstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 核心错误定义 ==========

var (
	// ErrContainerNotFound 容器未找到
	ErrContainerNotFound = errors.New("container not found")
	// ErrHANotEnabled HA 未启用
	ErrHANotEnabled = errors.New("HA not enabled for this container")
	// ErrNodeNotFound 节点未找到
	ErrNodeNotFound = errors.New("node not found")
	// ErrMigrationInProgress 迁移进行中
	ErrMigrationInProgress = errors.New("migration already in progress")
	// ErrPrimaryNodePrimaryAlreadySet 主节点已设置
	ErrPrimaryNodePrimaryAlreadySet = errors.New("primary node already set")
	// ErrFailoverFailed 故障转移失败
	ErrFailoverFailed = errors.New("failover failed")
	// ErrStorageNotAvailable 存储不可用
	ErrStorageNotAvailable = errors.New("storage not available on target node")
)

// ========== HA 状态定义 ==========

// HAState HA 状态
type HAState string

const (
	HAStateActive     HAState = "active"     // 活跃状态
	HAStateStandby    HAState = "standby"    // 待机状态
	HAStateMigrating  HAState = "migrating"  // 迁移中
	HAStateFailed     HAState = "failed"     // 失败
	HAStateRecovering HAState = "recovering" // 恢复中
)

// HAMode HA 模式
type HAMode string

const (
	HAModeActivePassive HAMode = "active-passive" // 主备模式
	HAModeActiveActive  HAMode = "active-active"  // 双活模式
)

// FailoverPolicy 故障转移策略
type FailoverPolicy string

const (
	FailoverPolicyAuto   FailoverPolicy = "auto"   // 自动故障转移
	FailoverPolicyManual FailoverPolicy = "manual" // 手动故障转移
	FailoverPolicyQuorum FailoverPolicy = "quorum" // 需要仲裁确认
)

// ========== 数据结构定义 ==========

// HAManager HA 管理器
type HAManager struct {
	mu            sync.RWMutex
	configPath    string
	containers    map[string]*HAContainer // 容器ID -> HA配置
	nodes         map[string]*HANode      // 节点ID -> 节点信息
	clusterConfig *ClusterConfig
	eventChan     chan HAEvent
	stateStore    *StateStore
	lxcManager    LXCManagerInterface
}

// HAContainer HA 容器配置
type HAContainer struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	PrimaryNode    string             `json:"primaryNode"`    // 主节点
	StandbyNodes   []string           `json:"standbyNodes"`   // 备节点列表
	State          HAState            `json:"state"`          // 当前状态
	Mode           HAMode             `json:"mode"`           // HA 模式
	Policy         FailoverPolicy     `json:"policy"`         // 故障转移策略
	Priority       int                `json:"priority"`       // 优先级 (越高越重要)
	HealthCheck    HealthCheckConfig  `json:"healthCheck"`    // 健康检查配置
	StorageVolumes []StorageVolumeRef `json:"storageVolumes"` // 关联的存储卷
	LastFailover   time.Time          `json:"lastFailover"`   // 上次故障转移时间
	FailoverCount  int                `json:"failoverCount"`  // 故障转移次数
	Enabled        bool               `json:"enabled"`        // 是否启用 HA
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

// HANode HA 节点信息
type HANode struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Address       string       `json:"address"`       // IP 地址
	Port          int          `json:"port"`          // 端口
	State         HAState      `json:"state"`         // 节点状态
	LastHeartbeat time.Time    `json:"lastHeartbeat"` // 最后心跳时间
	Capacity      NodeCapacity `json:"capacity"`      // 容量信息
	Priority      int          `json:"priority"`      // 节点优先级
	StoragePools  []string     `json:"storagePools"`  // 可用存储池
	Online        bool         `json:"online"`        // 是否在线
}

// NodeCapacity 节点容量
type NodeCapacity struct {
	CPUCount   int    `json:"cpuCount"`
	MemoryMB   uint64 `json:"memoryMB"`
	StorageGB  uint64 `json:"storageGB"`
	Containers int    `json:"containers"` // 运行的容器数
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled       bool          `json:"enabled"`
	Interval      time.Duration `json:"interval"`      // 检查间隔
	Timeout       time.Duration `json:"timeout"`       // 超时时间
	Threshold     int           `json:"threshold"`     // 失败阈值
	CheckScript   string        `json:"checkScript"`   // 自定义检查脚本
	CheckPort     int           `json:"checkPort"`     // 检查端口
	CheckHTTPPath string        `json:"checkHTTPPath"` // HTTP 检查路径
}

// StorageVolumeRef 存储卷引用
type StorageVolumeRef struct {
	PoolName   string `json:"poolName"`
	VolumeName string `json:"volumeName"`
	MountPath  string `json:"mountPath"` // 挂载路径
	ReadOnly   bool   `json:"readOnly"`  // 是否只读
	Shared     bool   `json:"shared"`    // 是否共享存储
}

// ClusterConfig 集群配置
type ClusterConfig struct {
	Name               string        `json:"name"`
	QuorumNodes        int           `json:"quorumNodes"`        // 仲裁节点数
	FailoverTimeout    time.Duration `json:"failoverTimeout"`    // 故障转移超时
	HeartbeatInterval  time.Duration `json:"heartbeatInterval"`  // 心跳间隔
	EnableAutoFailover bool          `json:"enableAutoFailover"` // 启用自动故障转移
}

// HAEvent HA 事件
type HAEvent struct {
	Type      HAEventType `json:"type"`
	Container string      `json:"container"`
	Node      string      `json:"node"`
	Timestamp time.Time   `json:"timestamp"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

// HAEventType 事件类型
type HAEventType string

const (
	EventContainerStarted   HAEventType = "container_started"
	EventContainerStopped   HAEventType = "container_stopped"
	EventContainerMigrated  HAEventType = "container_migrated"
	EventNodeOnline         HAEventType = "node_online"
	EventNodeOffline        HAEventType = "node_offline"
	EventFailoverStarted    HAEventType = "failover_started"
	EventFailoverCompleted  HAEventType = "failover_completed"
	EventFailoverFailed     HAEventType = "failover_failed"
	EventHealthCheckFailed  HAEventType = "health_check_failed"
	EventStorageUnavailable HAEventType = "storage_unavailable"
)

// FailoverResult 故障转移结果
type FailoverResult struct {
	Success      bool          `json:"success"`
	OldPrimary   string        `json:"oldPrimary"`
	NewPrimary   string        `json:"newPrimary"`
	Duration     time.Duration `json:"duration"`
	ErrorMessage string        `json:"errorMessage,omitempty"`
	MigratedAt   time.Time     `json:"migratedAt"`
}

// MigrationProgress 迁移进度
type MigrationProgress struct {
	ContainerID      string    `json:"containerId"`
	SourceNode       string    `json:"sourceNode"`
	TargetNode       string    `json:"targetNode"`
	State            string    `json:"state"`    // preparing, transferring, finalizing, completed, failed
	Progress         float64   `json:"progress"` // 0-100
	BytesTransferred uint64    `json:"bytesTransferred"`
	StartTime        time.Time `json:"startTime"`
	ETA              time.Time `json:"eta,omitempty"`
}

// LXCManagerInterface LXC 管理器接口（用于解耦）
type LXCManagerInterface interface {
	MigrateContainer(ctx context.Context, containerID, targetNode string) error
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, force bool) error
	GetContainerStatus(ctx context.Context, containerID string) (string, error)
	ListContainers(ctx context.Context) ([]map[string]interface{}, error)
}

// ========== HA 管理器实现 ==========

// NewHAManager 创建 HA 管理器
func NewHAManager(configPath string, lxcManager LXCManagerInterface) (*HAManager, error) {
	m := &HAManager{
		configPath: configPath,
		containers: make(map[string]*HAContainer),
		nodes:      make(map[string]*HANode),
		eventChan:  make(chan HAEvent, 100),
		lxcManager: lxcManager,
	}

	// 加载状态
	m.stateStore = NewStateStore(configPath)
	if err := m.loadState(); err != nil {
		// 如果加载失败，使用默认配置
		m.clusterConfig = DefaultClusterConfig()
	}

	// 确保 clusterConfig 不为 nil
	if m.clusterConfig == nil {
		m.clusterConfig = DefaultClusterConfig()
	}

	return m, nil
}

// DefaultClusterConfig 默认集群配置
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		Name:               "nas-os-cluster",
		QuorumNodes:        2,
		FailoverTimeout:    30 * time.Second,
		HeartbeatInterval:  5 * time.Second,
		EnableAutoFailover: true,
	}
}

// loadState 加载状态
func (m *HAManager) loadState() error {
	data, err := m.stateStore.Load()
	if err != nil {
		return err
	}

	if data != nil {
		m.containers = data.Containers
		m.nodes = data.Nodes
		m.clusterConfig = data.ClusterConfig
	}

	return nil
}

// saveState 保存状态
func (m *HAManager) saveState() error {
	data := &StateData{
		Containers:    m.containers,
		Nodes:         m.nodes,
		ClusterConfig: m.clusterConfig,
		UpdatedAt:     time.Now(),
	}

	return m.stateStore.Save(data)
}

// ========== 容器 HA 操作 ==========

// EnableHA 为容器启用 HA
func (m *HAManager) EnableHA(ctx context.Context, containerID string, config *HAContainerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.containers[containerID]; exists {
		return fmt.Errorf("HA already enabled for container %s", containerID)
	}

	// 验证节点可用性
	for _, nodeID := range config.StandbyNodes {
		if _, exists := m.nodes[nodeID]; !exists {
			return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
		}
	}

	// 验证存储卷可用性
	for _, vol := range config.StorageVolumes {
		if !vol.Shared {
			// 非共享存储需要验证目标节点也有相同存储池
			for _, nodeID := range config.StandbyNodes {
				node := m.nodes[nodeID]
				if !m.hasStoragePool(node, vol.PoolName) {
					return fmt.Errorf("node %s does not have storage pool %s", nodeID, vol.PoolName)
				}
			}
		}
	}

	haContainer := &HAContainer{
		ID:             containerID,
		Name:           config.Name,
		PrimaryNode:    config.PrimaryNode,
		StandbyNodes:   config.StandbyNodes,
		State:          HAStateActive,
		Mode:           config.Mode,
		Policy:         config.Policy,
		Priority:       config.Priority,
		HealthCheck:    config.HealthCheck,
		StorageVolumes: config.StorageVolumes,
		Enabled:        true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	m.containers[containerID] = haContainer

	// 发送事件
	m.emitEvent(HAEvent{
		Type:      EventContainerStarted,
		Container: containerID,
		Node:      config.PrimaryNode,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("HA enabled for container %s on node %s", containerID, config.PrimaryNode),
	})

	return m.saveState()
}

// DisableHA 禁用容器 HA
func (m *HAManager) DisableHA(ctx context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return ErrContainerNotFound
	}

	container.Enabled = false
	container.UpdatedAt = time.Now()

	// 发送事件
	m.emitEvent(HAEvent{
		Type:      EventContainerStopped,
		Container: containerID,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("HA disabled for container %s", containerID),
	})

	return m.saveState()
}

// GetHAContainer 获取容器 HA 配置
func (m *HAManager) GetHAContainer(containerID string) (*HAContainer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, exists := m.containers[containerID]
	if !exists {
		return nil, ErrContainerNotFound
	}

	return container, nil
}

// ListHAContainers 列出所有 HA 容器
func (m *HAManager) ListHAContainers() []*HAContainer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*HAContainer, 0, len(m.containers))
	for _, container := range m.containers {
		result = append(result, container)
	}

	return result
}

// ========== 故障转移操作 ==========

// Failover 执行故障转移
func (m *HAManager) Failover(ctx context.Context, containerID string, targetNode string, force bool) (*FailoverResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return nil, ErrContainerNotFound
	}

	if !container.Enabled {
		return nil, ErrHANotEnabled
	}

	// 检查是否有迁移正在进行
	if container.State == HAStateMigrating {
		return nil, ErrMigrationInProgress
	}

	// 验证目标节点
	if !force {
		validTarget := false
		for _, nodeID := range container.StandbyNodes {
			if nodeID == targetNode {
				validTarget = true
				break
			}
		}
		if !validTarget {
			return nil, fmt.Errorf("target node %s is not a valid standby node", targetNode)
		}
	}

	// 验证目标节点在线
	targetNodeInfo, exists := m.nodes[targetNode]
	if !exists {
		return nil, ErrNodeNotFound
	}
	if !targetNodeInfo.Online {
		return nil, fmt.Errorf("target node %s is offline", targetNode)
	}

	// 验证存储可用
	for _, vol := range container.StorageVolumes {
		if !m.hasStoragePool(targetNodeInfo, vol.PoolName) {
			return nil, ErrStorageNotAvailable
		}
	}

	// 开始故障转移
	oldPrimary := container.PrimaryNode
	container.State = HAStateMigrating
	container.UpdatedAt = time.Now()
	m.saveState()

	startTime := time.Now()
	m.emitEvent(HAEvent{
		Type:      EventFailoverStarted,
		Container: containerID,
		Node:      targetNode,
		Timestamp: startTime,
		Message:   fmt.Sprintf("Failover started from %s to %s", oldPrimary, targetNode),
	})

	// 执行迁移
	err := m.executeFailover(ctx, container, oldPrimary, targetNode)

	duration := time.Since(startTime)
	result := &FailoverResult{
		OldPrimary: oldPrimary,
		NewPrimary: targetNode,
		Duration:   duration,
		MigratedAt: time.Now(),
	}

	if err != nil {
		container.State = HAStateFailed
		result.Success = false
		result.ErrorMessage = err.Error()

		m.emitEvent(HAEvent{
			Type:      EventFailoverFailed,
			Container: containerID,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Failover failed: %s", err.Error()),
			Data:      result,
		})
	} else {
		container.State = HAStateActive
		container.PrimaryNode = targetNode
		// 将原主节点移到备用节点列表
		container.StandbyNodes = m.updateStandbyNodes(container.StandbyNodes, targetNode, oldPrimary)
		container.LastFailover = time.Now()
		container.FailoverCount++
		result.Success = true

		m.emitEvent(HAEvent{
			Type:      EventFailoverCompleted,
			Container: containerID,
			Node:      targetNode,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Failover completed successfully"),
			Data:      result,
		})
	}

	container.UpdatedAt = time.Now()
	m.saveState()

	return result, err
}

// AutoFailover 自动故障转移（当主节点故障时）
func (m *HAManager) AutoFailover(ctx context.Context, failedNode string) ([]*FailoverResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.clusterConfig.EnableAutoFailover {
		return nil, nil
	}

	var results []*FailoverResult
	var errors []error

	// 找到受影响的容器
	for _, container := range m.containers {
		if !container.Enabled || container.PrimaryNode != failedNode {
			continue
		}

		// 找最佳备用节点
		bestNode := m.findBestStandbyNode(container)
		if bestNode == "" {
			errors = append(errors, fmt.Errorf("no available standby node for container %s", container.ID))
			continue
		}

		// 执行故障转移
		container.State = HAStateMigrating

		oldPrimary := container.PrimaryNode
		startTime := time.Now()

		err := m.executeFailover(ctx, container, oldPrimary, bestNode)

		result := &FailoverResult{
			OldPrimary: oldPrimary,
			NewPrimary: bestNode,
			Duration:   time.Since(startTime),
			MigratedAt: time.Now(),
		}

		if err != nil {
			result.Success = false
			result.ErrorMessage = err.Error()
			container.State = HAStateFailed
		} else {
			result.Success = true
			container.State = HAStateActive
			container.PrimaryNode = bestNode
			container.StandbyNodes = m.updateStandbyNodes(container.StandbyNodes, bestNode, failedNode)
			container.LastFailover = time.Now()
			container.FailoverCount++
		}

		container.UpdatedAt = time.Now()
		results = append(results, result)
	}

	m.saveState()

	if len(errors) > 0 {
		return results, fmt.Errorf("some failovers failed: %v", errors)
	}

	return results, nil
}

// executeFailover 执行实际的故障转移操作
func (m *HAManager) executeFailover(ctx context.Context, container *HAContainer, sourceNode, targetNode string) error {
	// 停止源节点上的容器（如果可能）
	if m.lxcManager != nil {
		// 尝试停止容器（可能节点已经不可用，忽略错误）
		_ = m.lxcManager.StopContainer(ctx, container.ID, true)

		// 迁移容器到目标节点
		if err := m.lxcManager.MigrateContainer(ctx, container.ID, targetNode); err != nil {
			return fmt.Errorf("%w: %v", ErrFailoverFailed, err)
		}

		// 在目标节点启动容器
		if err := m.lxcManager.StartContainer(ctx, container.ID); err != nil {
			return fmt.Errorf("failed to start container on target node: %v", err)
		}
	}

	return nil
}

// ========== 辅助方法 ==========

// hasStoragePool 检查节点是否有指定存储池
func (m *HAManager) hasStoragePool(node *HANode, poolName string) bool {
	for _, pool := range node.StoragePools {
		if pool == poolName {
			return true
		}
	}
	return false
}

// findBestStandbyNode 找最佳备用节点
func (m *HAManager) findBestStandbyNode(container *HAContainer) string {
	var bestNode string
	bestPriority := -1

	for _, nodeID := range container.StandbyNodes {
		node, exists := m.nodes[nodeID]
		if !exists || !node.Online {
			continue
		}

		// 检查存储可用性
		storageAvailable := true
		for _, vol := range container.StorageVolumes {
			if !m.hasStoragePool(node, vol.PoolName) {
				storageAvailable = false
				break
			}
		}
		if !storageAvailable {
			continue
		}

		// 检查容量
		if node.Capacity.Containers > 0 && node.Capacity.MemoryMB < 1024 {
			continue // 节点负载过高
		}

		// 选择优先级最高的节点
		if node.Priority > bestPriority {
			bestPriority = node.Priority
			bestNode = nodeID
		}
	}

	return bestNode
}

// updateStandbyNodes 更新备用节点列表
func (m *HAManager) updateStandbyNodes(current []string, removeNode, addNode string) []string {
	result := make([]string, 0, len(current))

	// 移除新主节点
	for _, node := range current {
		if node != removeNode {
			result = append(result, node)
		}
	}

	// 添加旧主节点（如果不是已经在列表中）
	for _, node := range result {
		if node == addNode {
			return result
		}
	}
	result = append(result, addNode)

	return result
}

// emitEvent 发送事件
func (m *HAManager) emitEvent(event HAEvent) {
	select {
	case m.eventChan <- event:
	default:
		// 事件通道满，丢弃事件
	}
}

// ========== 节点管理 ==========

// RegisterNode 注册节点
func (m *HAManager) RegisterNode(node *HANode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node.Online = true
	node.LastHeartbeat = time.Now()
	m.nodes[node.ID] = node

	m.emitEvent(HAEvent{
		Type:      EventNodeOnline,
		Node:      node.ID,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Node %s registered", node.Name),
	})

	return m.saveState()
}

// UnregisterNode 注销节点
func (m *HAManager) UnregisterNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}

	node.Online = false
	node.LastHeartbeat = time.Now()

	m.emitEvent(HAEvent{
		Type:      EventNodeOffline,
		Node:      nodeID,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Node %s unregistered", node.Name),
	})

	return m.saveState()
}

// UpdateHeartbeat 更新心跳
func (m *HAManager) UpdateHeartbeat(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}

	node.LastHeartbeat = time.Now()
	node.Online = true

	return m.saveState()
}

// CheckNodeHealth 检查节点健康状态
func (m *HAManager) CheckNodeHealth() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]bool)
	now := time.Now()

	for _, node := range m.nodes {
		// 心跳超时检测
		timeout := m.clusterConfig.HeartbeatInterval * 3
		healthy := node.Online && now.Sub(node.LastHeartbeat) < timeout
		result[node.ID] = healthy

		// 如果节点变为不健康，触发事件
		if node.Online && !healthy {
			node.Online = false
			m.emitEvent(HAEvent{
				Type:      EventNodeOffline,
				Node:      node.ID,
				Timestamp: now,
				Message:   fmt.Sprintf("Node %s heartbeat timeout", node.Name),
			})
		}
	}

	return result
}

// GetNode 获取节点信息
func (m *HAManager) GetNode(nodeID string) (*HANode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}

	return node, nil
}

// ListNodes 列出所有节点
func (m *HAManager) ListNodes() []*HANode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*HANode, 0, len(m.nodes))
	for _, node := range m.nodes {
		result = append(result, node)
	}

	return result
}

// ========== 健康检查 ==========

// RunHealthCheck 执行健康检查
func (m *HAManager) RunHealthCheck(ctx context.Context, containerID string) error {
	m.mu.RLock()
	container, exists := m.containers[containerID]
	m.mu.RUnlock()

	if !exists {
		return ErrContainerNotFound
	}

	if !container.HealthCheck.Enabled {
		return nil
	}

	// 执行健康检查
	healthy := m.checkContainerHealth(ctx, container)

	if !healthy {
		m.emitEvent(HAEvent{
			Type:      EventHealthCheckFailed,
			Container: containerID,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Health check failed for container %s", containerID),
		})

		// 如果策略是自动故障转移，触发故障转移
		if container.Policy == FailoverPolicyAuto && container.Enabled {
			bestNode := m.findBestStandbyNode(container)
			if bestNode != "" {
				_, err := m.Failover(ctx, containerID, bestNode, false)
				return err
			}
		}
	}

	return nil
}

// checkContainerHealth 检查容器健康
func (m *HAManager) checkContainerHealth(ctx context.Context, container *HAContainer) bool {
	if m.lxcManager == nil {
		return true
	}

	// 检查容器状态
	status, err := m.lxcManager.GetContainerStatus(ctx, container.ID)
	if err != nil {
		return false
	}

	if status != "Running" {
		return false
	}

	// 如果配置了端口检查
	if container.HealthCheck.CheckPort > 0 {
		// TODO: 实现 TCP 端口检查
	}

	// 如果配置了 HTTP 检查
	if container.HealthCheck.CheckHTTPPath != "" && container.HealthCheck.CheckPort > 0 {
		// TODO: 实现 HTTP 检查
	}

	// 如果配置了自定义脚本
	if container.HealthCheck.CheckScript != "" {
		// TODO: 实现脚本检查
	}

	return true
}

// ========== 事件订阅 ==========

// SubscribeEvents 订阅 HA 事件
func (m *HAManager) SubscribeEvents() <-chan HAEvent {
	return m.eventChan
}

// GetEvents 获取历史事件
func (m *HAManager) GetEvents(limit int) []HAEvent {
	// 从状态存储获取历史事件
	return m.stateStore.GetEvents(limit)
}

// ========== 状态存储 ==========

// StateStore 状态存储
type StateStore struct {
	path string
}

// StateData 灾备数据
type StateData struct {
	Containers    map[string]*HAContainer `json:"containers"`
	Nodes         map[string]*HANode      `json:"nodes"`
	ClusterConfig *ClusterConfig          `json:"clusterConfig"`
	Events        []HAEvent               `json:"events"`
	UpdatedAt     time.Time               `json:"updatedAt"`
}

// NewStateStore 创建状态存储
func NewStateStore(path string) *StateStore {
	return &StateStore{path: path}
}

// Load 加载状态
func (s *StateStore) Load() (*StateData, error) {
	filePath := filepath.Join(s.path, "ha_state.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var state StateData
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// Save 保存状态
func (s *StateStore) Save(data *StateData) error {
	// 确保目录存在
	if err := os.MkdirAll(s.path, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(s.path, "ha_state.json")
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, jsonData, 0644)
}

// GetEvents 获取历史事件
func (s *StateStore) GetEvents(limit int) []HAEvent {
	data, err := s.Load()
	if err != nil || data == nil {
		return nil
	}

	if len(data.Events) <= limit {
		return data.Events
	}

	return data.Events[len(data.Events)-limit:]
}

// ========== 配置结构 ==========

// HAContainerConfig 容器 HA 配置
type HAContainerConfig struct {
	Name           string             `json:"name"`
	PrimaryNode    string             `json:"primaryNode"`
	StandbyNodes   []string           `json:"standbyNodes"`
	Mode           HAMode             `json:"mode"`
	Policy         FailoverPolicy     `json:"policy"`
	Priority       int                `json:"priority"`
	HealthCheck    HealthCheckConfig  `json:"healthCheck"`
	StorageVolumes []StorageVolumeRef `json:"storageVolumes"`
}

// Validate 验证配置
func (c *HAContainerConfig) Validate() error {
	if c.PrimaryNode == "" {
		return fmt.Errorf("primary node is required")
	}

	if len(c.StandbyNodes) == 0 {
		return fmt.Errorf("at least one standby node is required")
	}

	// 验证主节点不在备用节点列表中
	for _, node := range c.StandbyNodes {
		if node == c.PrimaryNode {
			return fmt.Errorf("primary node cannot be in standby nodes list")
		}
	}

	return nil
}
