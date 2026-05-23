package containwatch

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// RepairAction 修复动作类型
type RepairAction string

const (
	RepairRestart   RepairAction = "restart"   // 重启容器
	RepairRollback  RepairAction = "rollback"  // 回滚到上一版本
	RepairScaleUp   RepairAction = "scale_up"  // 扩容资源
	RepairAlert     RepairAction = "alert"     // 仅告警
	RepairStop      RepairAction = "stop"      // 停止容器
)

// RepairStatus 修复状态
type RepairStatus string

const (
	RepairStatusPending    RepairStatus = "pending"
	RepairStatusInProgress RepairStatus = "in_progress"
	RepairStatusSuccess    RepairStatus = "success"
	RepairStatusFailed     RepairStatus = "failed"
)

// ContainerSnapshot 容器快照（用于回滚）
type ContainerSnapshot struct {
	ID          string                 `json:"id"`
	ContainerID string                 `json:"container_id"`
	Image       string                 `json:"image"`
	Tag         string                 `json:"tag"`
	Config      map[string]interface{} `json:"config"`
	CreatedAt   time.Time              `json:"created_at"`
	Description string                 `json:"description"`
}

// RepairRecord 修复记录
type RepairRecord struct {
	ID          string       `json:"id"`
	ContainerID string       `json:"container_id"`
	Action      RepairAction `json:"action"`
	Status      RepairStatus `json:"status"`
	Reason      string       `json:"reason"`
	Attempts    int          `json:"attempts"`
	MaxAttempts int          `json:"max_attempts"`
	Error       string       `json:"error,omitempty"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// RestartPolicy 重启策略配置
type RestartPolicy struct {
	Enabled        bool          `json:"enabled"`          // 是否启用自动重启
	MaxRetries     int           `json:"max_retries"`      // 最大重试次数
	InitialBackoff time.Duration `json:"initial_backoff"`  // 初始退避时间
	MaxBackoff     time.Duration `json:"max_backoff"`      // 最大退避时间
	BackoffFactor  float64       `json:"backoff_factor"`   // 退避因子（指数退避）
}

// DefaultRestartPolicy 返回默认重启策略
func DefaultRestartPolicy() RestartPolicy {
	return RestartPolicy{
		Enabled:        true,
		MaxRetries:     5,
		InitialBackoff: 5 * time.Second,
		MaxBackoff:     5 * time.Minute,
		BackoffFactor:  2.0,
	}
}

// ResourceAdjustPolicy 资源调整策略
type ResourceAdjustPolicy struct {
	Enabled         bool    `json:"enabled"`          // 是否启用自动资源调整
	MemoryIncrement float64 `json:"memory_increment"` // OOM后内存增量倍数
	MaxMemoryBytes  float64 `json:"max_memory_bytes"` // 最大内存限制
	CPUIncrement    float64 `json:"cpu_increment"`    // CPU限制增量倍数
	MaxCPUPercent   float64 `json:"max_cpu_percent"`  // 最大CPU限制
}

// DefaultResourceAdjustPolicy 返回默认资源调整策略
func DefaultResourceAdjustPolicy() ResourceAdjustPolicy {
	return ResourceAdjustPolicy{
		Enabled:         true,
		MemoryIncrement: 1.5,
		MaxMemoryBytes:  8 * 1024 * 1024 * 1024, // 8GB
		CPUIncrement:    1.3,
		MaxCPUPercent:   800.0, // 8核
	}
}

// AutoRepairManager 自动修复管理器
type AutoRepairManager struct {
	mu              sync.RWMutex
	restartPolicies map[string]RestartPolicy        // 容器ID -> 重启策略
	resourcePolicies map[string]ResourceAdjustPolicy // 容器ID -> 资源调整策略
	retryStates     map[string]*retryState           // 容器ID -> 重试状态
	snapshots       map[string][]ContainerSnapshot   // 容器ID -> 快照列表
	records         []RepairRecord                   // 修复记录
	maxRecords      int
	nextID          int64
}

// retryState 内部重试状态
type retryState struct {
	CurrentRetry  int
	LastAttempt   time.Time
	CurrentBackoff time.Duration
}

// NewAutoRepairManager 创建自动修复管理器
func NewAutoRepairManager() *AutoRepairManager {
	return &AutoRepairManager{
		restartPolicies:  make(map[string]RestartPolicy),
		resourcePolicies: make(map[string]ResourceAdjustPolicy),
		retryStates:      make(map[string]*retryState),
		snapshots:        make(map[string][]ContainerSnapshot),
		records:          make([]RepairRecord, 0),
		maxRecords:       500,
	}
}

// RegisterContainer 注册容器到自动修复
func (m *AutoRepairManager) RegisterContainer(containerID string, restartPolicy RestartPolicy, resourcePolicy ResourceAdjustPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.restartPolicies[containerID]; exists {
		return fmt.Errorf("容器 %s 已注册自动修复", containerID)
	}

	m.restartPolicies[containerID] = restartPolicy
	m.resourcePolicies[containerID] = resourcePolicy
	m.retryStates[containerID] = &retryState{
		CurrentBackoff: restartPolicy.InitialBackoff,
	}

	log.Printf("容器 %s 已注册自动修复策略", containerID)
	return nil
}

// UnregisterContainer 注销容器自动修复
func (m *AutoRepairManager) UnregisterContainer(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.restartPolicies[containerID]; !exists {
		return fmt.Errorf("容器 %s 未注册自动修复", containerID)
	}

	delete(m.restartPolicies, containerID)
	delete(m.resourcePolicies, containerID)
	delete(m.retryStates, containerID)
	delete(m.snapshots, containerID)

	log.Printf("容器 %s 已注销自动修复", containerID)
	return nil
}

// HandleCrash 处理容器崩溃（自动重启 + 指数退避）
func (m *AutoRepairManager) HandleCrash(containerID string, reason string) (*RepairRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.restartPolicies[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 未注册自动修复", containerID)
	}

	state := m.retryStates[containerID]

	// 检查是否超过最大重试次数
	if !policy.Enabled || state.CurrentRetry >= policy.MaxRetries {
		record := m.newRecord(containerID, RepairRestart, RepairStatusFailed, reason,
			"已超过最大重试次数，切换为告警模式", state.CurrentRetry, policy.MaxRetries)
		return &record, nil
	}

	// 计算退避时间
	backoff := m.calculateBackoff(state, policy)

	// 检查退避时间是否已过
	if !state.LastAttempt.IsZero() && time.Since(state.LastAttempt) < backoff {
		record := m.newRecord(containerID, RepairRestart, RepairStatusPending, reason,
			fmt.Sprintf("等待退避时间 %v 后重试", backoff-time.Since(state.LastAttempt)),
			state.CurrentRetry, policy.MaxRetries)
		return &record, nil
	}

	// 执行重启
	state.CurrentRetry++
	state.LastAttempt = time.Now()

	log.Printf("正在重启容器 %s (第%d次, 退避 %v): %s", containerID, state.CurrentRetry, backoff, reason)

	// 模拟重启操作
	record := m.newRecord(containerID, RepairRestart, RepairStatusSuccess, reason, "",
		state.CurrentRetry, policy.MaxRetries)

	return &record, nil
}

// HandleOOM 处理 OOM（自动调整内存限制）
func (m *AutoRepairManager) HandleOOM(containerID string, currentMemoryLimit float64) (*RepairRecord, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.resourcePolicies[containerID]
	if !exists {
		return nil, 0, fmt.Errorf("容器 %s 未注册资源调整", containerID)
	}

	if !policy.Enabled {
		record := m.newRecord(containerID, RepairScaleUp, RepairStatusFailed,
			"OOM触发", "资源自动调整已禁用", 0, 0)
		return &record, currentMemoryLimit, nil
	}

	// 计算新的内存限制
	newLimit := currentMemoryLimit * policy.MemoryIncrement
	if newLimit > policy.MaxMemoryBytes {
		newLimit = policy.MaxMemoryBytes
	}

	record := m.newRecord(containerID, RepairScaleUp, RepairStatusSuccess,
		fmt.Sprintf("OOM触发，内存使用超出限制 %.2f MB", currentMemoryLimit/1024/1024),
		fmt.Sprintf("内存限制从 %.2f MB 调整为 %.2f MB", currentMemoryLimit/1024/1024, newLimit/1024/1024),
		0, 0)

	log.Printf("容器 %s OOM修复: 内存限制 %.2f MB -> %.2f MB",
		containerID, currentMemoryLimit/1024/1024, newLimit/1024/1024)

	return &record, newLimit, nil
}

// HandleHealthCheckFailure 处理健康检查失败
// 流程: 重启 -> 回滚 -> 告警
func (m *AutoRepairManager) HandleHealthCheckFailure(containerID string, consecutiveFailures int) (*RepairRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.restartPolicies[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 未注册自动修复", containerID)
	}

	switch {
	case consecutiveFailures <= 3:
		// 第一阶段：尝试重启
		record := m.newRecord(containerID, RepairRestart, RepairStatusSuccess,
			fmt.Sprintf("健康检查连续失败 %d 次", consecutiveFailures),
			"执行容器重启", consecutiveFailures, 0)
		return &record, nil

	case consecutiveFailures <= 6:
		// 第二阶段：尝试回滚
		snapshots := m.snapshots[containerID]
		if len(snapshots) > 0 {
			latest := snapshots[len(snapshots)-1]
			record := m.newRecord(containerID, RepairRollback, RepairStatusSuccess,
				fmt.Sprintf("健康检查连续失败 %d 次，执行回滚", consecutiveFailures),
				fmt.Sprintf("回滚到快照 %s (镜像: %s:%s)", latest.ID, latest.Image, latest.Tag),
				consecutiveFailures, 0)
			return &record, nil
		}
		// 无快照可回滚，继续告警
		fallthrough

	default:
		// 第三阶段：告警
		record := m.newRecord(containerID, RepairAlert, RepairStatusSuccess,
			fmt.Sprintf("健康检查连续失败 %d 次，需要人工介入", consecutiveFailures),
			"已发送告警通知", consecutiveFailures, 0)
		return &record, nil
	}
}

// CreateSnapshot 创建容器快照
func (m *AutoRepairManager) CreateSnapshot(containerID, image, tag string, config map[string]interface{}, description string) (*ContainerSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.restartPolicies[containerID]; !exists {
		return nil, fmt.Errorf("容器 %s 未注册自动修复", containerID)
	}

	m.nextID++
	snapshot := ContainerSnapshot{
		ID:          fmt.Sprintf("snap-%d-%d", time.Now().UnixMilli(), m.nextID),
		ContainerID: containerID,
		Image:       image,
		Tag:         tag,
		Config:      config,
		CreatedAt:   time.Now(),
		Description: description,
	}

	m.snapshots[containerID] = append(m.snapshots[containerID], snapshot)

	// 限制每个容器最多保留 10 个快照
	if len(m.snapshots[containerID]) > 10 {
		m.snapshots[containerID] = m.snapshots[containerID][1:]
	}

	log.Printf("容器 %s 创建快照: %s", containerID, snapshot.ID)
	return &snapshot, nil
}

// RollbackToSnapshot 回滚到指定快照
func (m *AutoRepairManager) RollbackToSnapshot(containerID, snapshotID string) (*RepairRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshots, exists := m.snapshots[containerID]
	if !exists || len(snapshots) == 0 {
		return nil, fmt.Errorf("容器 %s 无可用快照", containerID)
	}

	var target *ContainerSnapshot
	for _, s := range snapshots {
		if s.ID == snapshotID {
			target = &s
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("快照 %s 未找到", snapshotID)
	}

	record := m.newRecord(containerID, RepairRollback, RepairStatusSuccess,
		"手动触发回滚",
		fmt.Sprintf("回滚到快照 %s (镜像: %s:%s)", target.ID, target.Image, target.Tag),
		0, 0)

	log.Printf("容器 %s 回滚到快照 %s", containerID, snapshotID)
	return &record, nil
}

// GetRepairRecords 获取容器修复记录
func (m *AutoRepairManager) GetRepairRecords(containerID string) []RepairRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []RepairRecord
	for _, r := range m.records {
		if r.ContainerID == containerID {
			result = append(result, r)
		}
	}
	return result
}

// GetSnapshots 获取容器快照列表
func (m *AutoRepairManager) GetSnapshots(containerID string) []ContainerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots, exists := m.snapshots[containerID]
	if !exists {
		return nil
	}

	result := make([]ContainerSnapshot, len(snapshots))
	copy(result, snapshots)
	return result
}

// ResetRetryState 重置容器重试状态（修复成功后调用）
func (m *AutoRepairManager) ResetRetryState(containerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.restartPolicies[containerID]
	if !exists {
		return
	}

	m.retryStates[containerID] = &retryState{
		CurrentBackoff: policy.InitialBackoff,
	}
}

// calculateBackoff 计算指数退避时间
func (m *AutoRepairManager) calculateBackoff(state *retryState, policy RestartPolicy) time.Duration {
	backoff := float64(policy.InitialBackoff) * math.Pow(policy.BackoffFactor, float64(state.CurrentRetry))
	if backoff > float64(policy.MaxBackoff) {
		backoff = float64(policy.MaxBackoff)
	}
	return time.Duration(backoff)
}

// newRecord 创建修复记录
func (m *AutoRepairManager) newRecord(containerID string, action RepairAction, status RepairStatus, reason, detail string, attempts, maxAttempts int) RepairRecord {
	m.nextID++
	now := time.Now()

	record := RepairRecord{
		ID:          fmt.Sprintf("repair-%d-%d", now.UnixMilli(), m.nextID),
		ContainerID: containerID,
		Action:      action,
		Status:      status,
		Reason:      reason,
		Attempts:    attempts,
		MaxAttempts: maxAttempts,
		StartedAt:   now,
	}

	if status == RepairStatusSuccess || status == RepairStatusFailed {
		record.CompletedAt = &now
		record.Error = detail
	}

	// 保存记录
	m.records = append(m.records, record)
	if len(m.records) > m.maxRecords {
		m.records = m.records[1:]
	}

	return record
}

// GetRepairOverview 获取修复状态概览
func (m *AutoRepairManager) GetRepairOverview() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalRecords := len(m.records)
	successCount := 0
	failCount := 0
	pendingCount := 0

	for _, r := range m.records {
		switch r.Status {
		case RepairStatusSuccess:
			successCount++
		case RepairStatusFailed:
			failCount++
		case RepairStatusPending:
			pendingCount++
		}
	}

	return map[string]interface{}{
		"total_containers": len(m.restartPolicies),
		"total_records":    totalRecords,
		"success_count":    successCount,
		"fail_count":       failCount,
		"pending_count":    pendingCount,
		"timestamp":        time.Now(),
	}
}
