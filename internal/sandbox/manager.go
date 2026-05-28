// Package sandbox 提供安全沙箱隔离环境管理功能
package sandbox

import (
	"fmt"
	"sync"
	"time"
)

// Manager 沙箱管理器
type Manager struct {
	mu        sync.RWMutex
	sandboxes map[string]*Sandbox
	isolator  *Isolator
	snapshots *SnapshotManager
}

// NewManager 创建沙箱管理器
func NewManager(basePath string) *Manager {
	return &Manager{
		sandboxes: make(map[string]*Sandbox),
		isolator:  NewIsolator(),
		snapshots: NewSnapshotManager(basePath),
	}
}

// Create 创建沙箱
func (m *Manager) Create(req *CreateSandboxRequest) (*Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Config == nil {
		return nil, ErrInvalidConfig
	}

	// 验证配置
	if err := m.validateConfig(req.Config); err != nil {
		return nil, err
	}

	// 检查名称唯一性
	for _, sb := range m.sandboxes {
		if sb.Config.Name == req.Config.Name {
			return nil, ErrSandboxAlreadyExists
		}
	}

	// 生成ID
	id := fmt.Sprintf("sandbox_%d", time.Now().UnixNano())

	// 创建沙箱实例
	sandbox := &Sandbox{
		ID:        id,
		Config:    req.Config,
		Status:    SandboxStatusCreated,
		RootPath:  fmt.Sprintf("/var/lib/nas-os/sandbox/%s", id),
		CreatedAt: time.Now(),
	}

	// 如果从快照恢复
	if req.FromSnapshot != "" {
		snapshot, err := m.snapshots.Get(req.FromSnapshot)
		if err != nil {
			return nil, fmt.Errorf("快照不存在: %w", err)
		}
		if err := m.snapshots.Restore(snapshot.ID, sandbox.RootPath); err != nil {
			return nil, fmt.Errorf("从快照恢复失败: %w", err)
		}
	}

	// 应用隔离配置
	if err := m.isolator.SetupIsolation(sandbox); err != nil {
		return nil, fmt.Errorf("隔离设置失败: %w", err)
	}

	m.sandboxes[id] = sandbox
	return sandbox, nil
}

// Get 获取沙箱
func (m *Manager) Get(id string) (*Sandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return nil, ErrSandboxNotFound
	}
	return sandbox, nil
}

// List 列出所有沙箱
func (m *Manager) List() []*Sandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Sandbox, 0, len(m.sandboxes))
	for _, sandbox := range m.sandboxes {
		list = append(list, sandbox)
	}
	return list
}

// Update 更新沙箱配置
func (m *Manager) Update(id string, req *UpdateSandboxRequest) (*Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return nil, ErrSandboxNotFound
	}

	// 运行中的沙箱不能修改某些配置
	if sandbox.Status == SandboxStatusRunning {
		return nil, ErrSandboxRunning
	}

	if req.Description != "" {
		sandbox.Config.Description = req.Description
	}
	if req.ResourceLimit != nil {
		if err := m.validateResourceLimit(req.ResourceLimit); err != nil {
			return nil, err
		}
		sandbox.Config.ResourceLimit = req.ResourceLimit
	}
	if req.Labels != nil {
		sandbox.Config.Labels = req.Labels
	}
	if req.AutoStart != nil {
		sandbox.Config.AutoStart = *req.AutoStart
	}

	return sandbox, nil
}

// Delete 删除沙箱
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return ErrSandboxNotFound
	}

	// 运行中的沙箱需要先停止
	if sandbox.Status == SandboxStatusRunning {
		return ErrSandboxRunning
	}

	// 删除关联的快照
	if err := m.snapshots.DeleteBySandbox(id); err != nil {
		return fmt.Errorf("删除快照失败: %w", err)
	}

	// 清理隔离资源
	if err := m.isolator.CleanupIsolation(sandbox); err != nil {
		return fmt.Errorf("清理隔离资源失败: %w", err)
	}

	delete(m.sandboxes, id)
	return nil
}

// Start 启动沙箱
func (m *Manager) Start(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return ErrSandboxNotFound
	}

	if sandbox.Status == SandboxStatusRunning {
		return ErrSandboxRunning
	}

	// 应用资源限制
	if err := m.isolator.ApplyResourceLimit(sandbox); err != nil {
		sandbox.Status = SandboxStatusError
		sandbox.Error = err.Error()
		return err
	}

	// 启动沙箱进程
	if err := m.isolator.StartProcess(sandbox); err != nil {
		sandbox.Status = SandboxStatusError
		sandbox.Error = err.Error()
		return err
	}

	now := time.Now()
	sandbox.Status = SandboxStatusRunning
	sandbox.StartedAt = &now
	sandbox.StoppedAt = nil
	sandbox.Error = ""

	return nil
}

// Stop 停止沙箱
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return ErrSandboxNotFound
	}

	if sandbox.Status != SandboxStatusRunning {
		return ErrSandboxStopped
	}

	// 停止沙箱进程
	if err := m.isolator.StopProcess(sandbox); err != nil {
		sandbox.Status = SandboxStatusError
		sandbox.Error = err.Error()
		return err
	}

	now := time.Now()
	sandbox.Status = SandboxStatusStopped
	sandbox.StoppedAt = &now
	sandbox.PID = 0

	return nil
}

// Pause 暂停沙箱
func (m *Manager) Pause(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return ErrSandboxNotFound
	}

	if sandbox.Status != SandboxStatusRunning {
		return ErrSandboxStopped
	}

	if err := m.isolator.PauseProcess(sandbox); err != nil {
		return err
	}

	sandbox.Status = SandboxStatusPaused
	return nil
}

// Resume 恢复沙箱
func (m *Manager) Resume(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return ErrSandboxNotFound
	}

	if sandbox.Status != SandboxStatusPaused {
		return fmt.Errorf("沙箱不在暂停状态")
	}

	if err := m.isolator.ResumeProcess(sandbox); err != nil {
		return err
	}

	sandbox.Status = SandboxStatusRunning
	return nil
}

// GetResourceUsage 获取资源使用情况
func (m *Manager) GetResourceUsage(id string) (*ResourceUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sandbox, exists := m.sandboxes[id]
	if !exists {
		return nil, ErrSandboxNotFound
	}

	if sandbox.Status != SandboxStatusRunning {
		return nil, ErrSandboxStopped
	}

	usage, err := m.isolator.GetResourceUsage(sandbox)
	if err != nil {
		return nil, err
	}

	sandbox.ResourceUsage = usage
	return usage, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *SandboxStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SandboxStats{
		TotalSandbox: len(m.sandboxes),
	}

	for _, sandbox := range m.sandboxes {
		switch sandbox.Status {
		case SandboxStatusRunning:
			stats.RunningSandbox++
		case SandboxStatusStopped, SandboxStatusCreated:
			stats.StoppedSandbox++
		}
	}

	snapshotStats := m.snapshots.GetStats()
	stats.TotalSnapshots = snapshotStats.TotalCount
	stats.TotalSnapshotSizeBytes = snapshotStats.TotalSize

	return stats
}

// validateConfig 验证配置
func (m *Manager) validateConfig(config *SandboxConfig) error {
	if config.Name == "" {
		return fmt.Errorf("沙箱名称不能为空")
	}

	if config.ResourceLimit != nil {
		if err := m.validateResourceLimit(config.ResourceLimit); err != nil {
			return err
		}
	}

	return nil
}

// validateResourceLimit 验证资源限制
func (m *Manager) validateResourceLimit(limit *ResourceLimit) error {
	if limit.CPUCores < 0 {
		return fmt.Errorf("%w: CPU核心数不能为负数", ErrInvalidResourceLimit)
	}
	if limit.MemoryMB < 0 {
		return fmt.Errorf("%w: 内存限制不能为负数", ErrInvalidResourceLimit)
	}
	if limit.DiskIOMBps < 0 {
		return fmt.Errorf("%w: 磁盘IO限制不能为负数", ErrInvalidResourceLimit)
	}
	if limit.NetworkBandwidthMbps < 0 {
		return fmt.Errorf("%w: 网络带宽限制不能为负数", ErrInvalidResourceLimit)
	}
	if limit.PIDsLimit < 0 {
		return fmt.Errorf("%w: 进程数限制不能为负数", ErrInvalidResourceLimit)
	}
	return nil
}

// SnapshotCount 获取快照数量（用于统计）
func (m *Manager) SnapshotCount() int {
	return m.snapshots.Count()
}
