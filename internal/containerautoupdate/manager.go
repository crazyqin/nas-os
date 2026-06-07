// Package containerautoupdate 提供容器自动更新管理核心业务逻辑
package containerautoupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 容器自动更新管理器.
type Manager struct {
	storagePath string
	policies    map[string]*UpdatePolicy
	records     map[string]*UpdateRecord
	history     []*UpdateRecord
	health      map[string]*ContainerHealth
	rollbackCfg RollbackConfig
	mu          sync.RWMutex
}

// NewManager 创建容器自动更新管理器.
func NewManager(storagePath string) *Manager {
	m := &Manager{
		storagePath: storagePath,
		policies:    make(map[string]*UpdatePolicy),
		records:     make(map[string]*UpdateRecord),
		history:     make([]*UpdateRecord, 0),
		health:      make(map[string]*ContainerHealth),
		rollbackCfg: RollbackConfig{
			MaxHistory:    10,
			AutoRollback:  true,
			RollbackDelay: 30 * time.Second,
		},
	}

	// 加载持久化数据
	m.loadFromStorage()

	return m
}

// ========== 策略管理 ==========

// SetPolicy 设置更新策略.
func (m *Manager) SetPolicy(ctx context.Context, policy UpdatePolicy) (*UpdatePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}
	if policy.MaxRetries == 0 {
		policy.MaxRetries = 3
	}
	if policy.HealthCheckTimeout == 0 {
		policy.HealthCheckTimeout = 30
	}
	policy.CreatedAt = time.Now()

	m.policies[policy.ContainerID] = &policy

	// 持久化
	if err := m.saveToStorage(); err != nil {
		return nil, fmt.Errorf("failed to save policy: %w", err)
	}

	return &policy, nil
}

// GetPolicy 获取更新策略.
func (m *Manager) GetPolicy(containerID string) (*UpdatePolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[containerID]
	if !ok {
		return nil, fmt.Errorf("policy for container %q not found", containerID)
	}

	cp := *policy
	return &cp, nil
}

// ========== 更新检查 ==========

// CheckUpdates 检查单个容器是否有更新.
func (m *Manager) CheckUpdates(ctx context.Context, containerID string) (*UpdateCheck, error) {
	m.mu.RLock()
	policy, ok := m.policies[containerID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("policy for container %q not found", containerID)
	}
	_ = policy
	m.mu.RUnlock()

	// 模拟检查镜像更新
	check := m.checkImageDigest(containerID)

	return check, nil
}

// CheckAllUpdates 批量检查所有容器更新.
func (m *Manager) CheckAllUpdates(ctx context.Context) []UpdateCheck {
	m.mu.RLock()
	containerIDs := make([]string, 0, len(m.policies))
	for id, p := range m.policies {
		if p.Enabled {
			containerIDs = append(containerIDs, id)
		}
	}
	m.mu.RUnlock()

	checks := make([]UpdateCheck, 0, len(containerIDs))
	for _, id := range containerIDs {
		check := m.checkImageDigest(id)
		checks = append(checks, *check)
	}

	return checks
}

// ========== 更新执行 ==========

// ApplyUpdate 执行容器更新（pull→stop→rename→start→health check）.
func (m *Manager) ApplyUpdate(ctx context.Context, containerID string) (*UpdateRecord, error) {
	m.mu.RLock()
	policy, ok := m.policies[containerID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("policy for container %q not found", containerID)
	}
	policyCopy := *policy
	m.mu.RUnlock()

	// 创建更新记录
	record := &UpdateRecord{
		ID:          uuid.New().String(),
		ContainerID: containerID,
		OldImage:    policyCopy.ContainerName + ":current",
		NewImage:    policyCopy.ContainerName + ":latest",
		Status:      StatusPending,
		StartedAt:   time.Now(),
	}

	m.mu.Lock()
	m.records[record.ID] = record
	m.mu.Unlock()

	// 执行更新流程
	go m.executeUpdate(record, &policyCopy)

	return record, nil
}

// executeUpdate 执行更新流程.
func (m *Manager) executeUpdate(record *UpdateRecord, policy *UpdatePolicy) {
	// 1. 拉取镜像
	record.Status = StatusDownloading
	m.notifyProgress(record)

	if err := m.pullImage(record.NewImage); err != nil {
		m.failUpdate(record, fmt.Sprintf("pull failed: %v", err), policy)
		return
	}

	// 2. 停止容器
	record.Status = StatusStopping
	m.notifyProgress(record)

	if err := m.stopContainer(policy.ContainerName); err != nil {
		m.failUpdate(record, fmt.Sprintf("stop failed: %v", err), policy)
		return
	}

	// 3. 重命名旧容器（备份）
	oldContainerName := policy.ContainerName + "-old"
	m.renameContainer(policy.ContainerName, oldContainerName)

	// 4. 启动新容器
	record.Status = StatusStarting
	m.notifyProgress(record)

	if err := m.startContainer(policy.ContainerName, record.NewImage); err != nil {
		// 启动失败，尝试回滚
		m.rollbackContainer(policy.ContainerName, oldContainerName, record, policy)
		return
	}

	// 5. 健康检查
	if policy.HealthCheckURL != "" {
		record.Status = StatusHealthCheck
		m.notifyProgress(record)

		if err := m.performHealthCheck(policy.HealthCheckURL, policy.HealthCheckTimeout); err != nil {
			// 健康检查失败，尝试回滚
			m.rollbackContainer(policy.ContainerName, oldContainerName, record, policy)
			return
		}
	}

	// 6. 更新成功
	record.Status = StatusSuccess
	completedAt := time.Now()
	record.CompletedAt = &completedAt
	record.Duration = completedAt.Sub(record.StartedAt).Milliseconds()
	record.NewDigest = m.getImageDigest(record.NewImage)

	// 清理旧容器
	m.removeContainer(oldContainerName)

	// 更新健康状态
	m.updateHealthStatus(policy.ContainerID, HealthHealthy)

	// 保存记录
	m.mu.Lock()
	m.history = append(m.history, record)
	m.saveToStorage()
	m.mu.Unlock()

	// 发送通知
	if policy.NotifyOnUpdate {
		m.sendNotification(policy.ContainerID, record.NewDigest, true)
	}
}

// Rollback 回滚到上一版本.
func (m *Manager) Rollback(ctx context.Context, recordID string) error {
	m.mu.RLock()
	record, ok := m.records[recordID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("record %q not found", recordID)
	}
	recordCopy := *record
	m.mu.RUnlock()

	if recordCopy.Status != StatusFailed && recordCopy.Status != StatusSuccess {
		return fmt.Errorf("cannot rollback record with status %q", recordCopy.Status)
	}

	// 执行回滚
	record.Status = StatusRolledBack
	completedAt := time.Now()
	record.CompletedAt = &completedAt
	record.Duration = completedAt.Sub(record.StartedAt).Milliseconds()

	// 更新健康状态
	m.mu.Lock()
	m.updateHealthStatus(recordCopy.ContainerID, HealthHealthy)
	m.saveToStorage()
	m.mu.Unlock()

	return nil
}

// ========== 健康状态 ==========

// GetHealth 获取容器健康状态.
func (m *Manager) GetHealth(ctx context.Context, containerID string) (*ContainerHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, ok := m.health[containerID]
	if !ok {
		// 返回默认健康状态
		return &ContainerHealth{
			ContainerID: containerID,
			Status:      HealthUnknown,
			LastCheck:   time.Time{},
		}, nil
	}

	cp := *health
	return &cp, nil
}

// ========== 历史和统计 ==========

// GetUpdateHistory 获取更新历史.
func (m *Manager) GetUpdateHistory(ctx context.Context, containerID string) []UpdateRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]UpdateRecord, 0)
	for _, r := range m.history {
		if r.ContainerID == containerID {
			records = append(records, *r)
		}
	}
	return records
}

// GetStats 获取更新统计.
func (m *Manager) GetStats(ctx context.Context) UpdateStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := UpdateStats{}
	if len(m.history) == 0 {
		return stats
	}

	var totalDuration int64
	stats.TotalUpdates = len(m.history)

	for _, r := range m.history {
		switch r.Status {
		case StatusSuccess:
			stats.SuccessfulUpdates++
		case StatusFailed:
			stats.FailedUpdates++
		case StatusRolledBack:
			stats.RolledBackUpdates++
		}
		totalDuration += r.Duration
	}

	if stats.TotalUpdates > 0 {
		stats.AvgUpdateDuration = float64(totalDuration) / float64(stats.TotalUpdates)
	}

	stats.LastUpdateTime = m.history[len(m.history)-1].StartedAt

	return stats
}

// RunScheduledChecks 执行定时检查（由 cron 调用）.
func (m *Manager) RunScheduledChecks(ctx context.Context) error {
	checks := m.CheckAllUpdates(ctx)

	for _, check := range checks {
		if check.HasUpdate {
			policy, err := m.GetPolicy(check.ContainerID)
			if err != nil {
				continue
			}

			if policy.Enabled {
				// 自动执行更新
				_, err := m.ApplyUpdate(ctx, check.ContainerID)
				if err != nil {
					// 发送失败通知
					if policy.NotifyOnFailure {
						m.sendNotification(check.ContainerID, "", false)
					}
				}
			} else if policy.NotifyOnUpdate {
				// 仅通知
				m.sendNotification(check.ContainerID, check.LatestDigest, true)
			}
		}
	}

	return nil
}

// NotifyUpdate 发送更新通知.
func (m *Manager) NotifyUpdate(ctx context.Context, containerID, newDigest string) error {
	m.sendNotification(containerID, newDigest, true)
	return nil
}

// ========== 内部方法 ==========

// checkImageDigest 检查镜像 digest（模拟）.
func (m *Manager) checkImageDigest(containerID string) *UpdateCheck {
	// 模拟：检查镜像更新
	return &UpdateCheck{
		ContainerID:   containerID,
		CurrentImage:  "nginx:1.21",
		CurrentDigest: "sha256:abc123",
		LatestDigest:  "sha256:def456",
		LatestTag:     "1.22",
		HasUpdate:     true,
		CheckedAt:     time.Now(),
	}
}

// pullImage 拉取镜像（模拟）.
func (m *Manager) pullImage(image string) error {
	// 模拟拉取延迟
	time.Sleep(100 * time.Millisecond)
	return nil
}

// stopContainer 停止容器（模拟）.
func (m *Manager) stopContainer(name string) error {
	// 模拟停止延迟
	time.Sleep(50 * time.Millisecond)
	return nil
}

// startContainer 启动容器（模拟）.
func (m *Manager) startContainer(name, image string) error {
	// 模拟启动延迟
	time.Sleep(50 * time.Millisecond)
	return nil
}

// renameContainer 重命名容器（模拟）.
func (m *Manager) renameContainer(oldName, newName string) {
	// 模拟重命名
}

// removeContainer 移除容器（模拟）.
func (m *Manager) removeContainer(name string) {
	// 模拟移除
}

// performHealthCheck 执行健康检查（模拟）.
func (m *Manager) performHealthCheck(url string, timeout int) error {
	// 模拟健康检查
	time.Sleep(time.Duration(timeout) * time.Millisecond)
	return nil
}

// getImageDigest 获取镜像 digest（模拟）.
func (m *Manager) getImageDigest(image string) string {
	return "sha256:newdigest"
}

// failUpdate 标记更新失败.
func (m *Manager) failUpdate(record *UpdateRecord, errMsg string, policy *UpdatePolicy) {
	record.Status = StatusFailed
	record.Error = errMsg
	completedAt := time.Now()
	record.CompletedAt = &completedAt
	record.Duration = completedAt.Sub(record.StartedAt).Milliseconds()

	// 更新健康状态
	m.updateHealthStatus(record.ContainerID, HealthUnhealthy)

	// 保存记录
	m.mu.Lock()
	m.history = append(m.history, record)
	m.saveToStorage()
	m.mu.Unlock()

	// 发送失败通知
	if policy.NotifyOnFailure {
		m.sendNotification(record.ContainerID, "", false)
	}
}

// rollbackContainer 回滚容器.
func (m *Manager) rollbackContainer(containerName, oldName string, record *UpdateRecord, policy *UpdatePolicy) {
	// 停止失败的容器
	m.stopContainer(containerName)

	// 恢复旧容器
	m.renameContainer(oldName, containerName)

	// 标记为已回滚
	record.Status = StatusRolledBack
	completedAt := time.Now()
	record.CompletedAt = &completedAt
	record.Duration = completedAt.Sub(record.StartedAt).Milliseconds()
	record.RollbackImage = record.OldImage

	// 更新健康状态
	m.updateHealthStatus(record.ContainerID, HealthHealthy)

	// 保存记录
	m.mu.Lock()
	m.history = append(m.history, record)
	m.saveToStorage()
	m.mu.Unlock()
}

// updateHealthStatus 更新健康状态.
func (m *Manager) updateHealthStatus(containerID string, status HealthStatus) {
	health, ok := m.health[containerID]
	if !ok {
		health = &ContainerHealth{
			ContainerID: containerID,
		}
		m.health[containerID] = health
	}

	health.Status = status
	health.LastCheck = time.Now()

	if status == HealthUnhealthy {
		health.ConsecutiveFailures++
		health.RestartCount++
	} else if status == HealthHealthy {
		health.ConsecutiveFailures = 0
	}
}

// notifyProgress 通知更新进度.
func (m *Manager) notifyProgress(record *UpdateRecord) {
	// 可以扩展为实际的通知机制
}

// sendNotification 发送通知.
func (m *Manager) sendNotification(containerID, digest string, success bool) {
	// 可以扩展为实际的通知机制（邮件、webhook等）
}

// ========== 持久化 ==========

// storageData 存储数据结构.
type storageData struct {
	Policies []*UpdatePolicy    `json:"policies"`
	Records  []*UpdateRecord    `json:"records"`
	Health   []*ContainerHealth `json:"health"`
}

// saveToStorage 保存数据到存储.
func (m *Manager) saveToStorage() error {
	if m.storagePath == "" {
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(m.storagePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data := storageData{
		Policies: make([]*UpdatePolicy, 0, len(m.policies)),
		Records:  make([]*UpdateRecord, 0, len(m.history)),
		Health:   make([]*ContainerHealth, 0, len(m.health)),
	}

	for _, p := range m.policies {
		data.Policies = append(data.Policies, p)
	}
	for _, r := range m.history {
		data.Records = append(data.Records, r)
	}
	for _, h := range m.health {
		data.Health = append(data.Health, h)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := os.WriteFile(m.storagePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// loadFromStorage 从存储加载数据.
func (m *Manager) loadFromStorage() {
	if m.storagePath == "" {
		return
	}

	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		return
	}

	var sd storageData
	if err := json.Unmarshal(data, &sd); err != nil {
		return
	}

	for _, p := range sd.Policies {
		m.policies[p.ContainerID] = p
	}
	for _, r := range sd.Records {
		m.records[r.ID] = r
		m.history = append(m.history, r)
	}
	for _, h := range sd.Health {
		m.health[h.ContainerID] = h
	}
}
