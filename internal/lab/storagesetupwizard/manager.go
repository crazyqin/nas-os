// Package storagesetupwizard 提供存储设置向导核心管理器
package storagesetupwizard

import (
	"fmt"
	"sync"
	"time"
)

// Manager 存储设置向导管理器.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*SetupSession
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*SetupSession),
	}
}

// CreateSession 创建设置会话.
func (m *Manager) CreateSession(disks []DiskInfo) (*SetupSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(disks) == 0 {
		return nil, fmt.Errorf("至少需要一个磁盘")
	}

	id := fmt.Sprintf("setup_%d", time.Now().UnixNano())
	session := &SetupSession{
		ID:          id,
		CurrentStep: StepDiskSelection,
		Disks:       disks,
		Status:      "in_progress",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.sessions[id] = session
	return session, nil
}

// GetSession 获取会话.
func (m *Manager) GetSession(id string) (*SetupSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", id)
	}
	return session, nil
}

// UpdateStep 更新会话步骤.
func (m *Manager) UpdateStep(id string, step Step) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}

	session.CurrentStep = step
	session.UpdatedAt = time.Now()
	return nil
}

// SetPoolConfig 设置存储池配置.
func (m *Manager) SetPoolConfig(id string, config PoolConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}

	// 验证RAID配置
	if err := ValidateRAIDConfig(config.RAID, session.Disks); err != nil {
		return fmt.Errorf("RAID配置无效: %w", err)
	}

	session.Pool = config
	session.CurrentStep = StepPoolCreation
	session.UpdatedAt = time.Now()
	return nil
}

// SetVolumeConfig 设置卷配置.
func (m *Manager) SetVolumeConfig(id string, config VolumeConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}

	session.Volume = config
	session.CurrentStep = StepVolumeSetup
	session.UpdatedAt = time.Now()
	return nil
}

// SetShareConfig 设置共享配置.
func (m *Manager) SetShareConfig(id string, config ShareConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}

	session.Share = config
	session.CurrentStep = StepShareConfig
	session.UpdatedAt = time.Now()
	return nil
}

// CompleteSession 完成会话.
func (m *Manager) CompleteSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}

	now := time.Now()
	session.Status = "completed"
	session.CurrentStep = StepComplete
	session.CompletedAt = &now
	session.UpdatedAt = now
	return nil
}

// GetRecommendations 获取RAID推荐.
func (m *Manager) GetRecommendations(id string, priority string) ([]RAIDRecommendation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", id)
	}

	return RecommendRAID(len(session.Disks), priority), nil
}

// EstimateCapacity 估算容量.
func (m *Manager) EstimateCapacity(id string, raidType RAIDType) (*CapacityEstimation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", id)
	}

	if len(session.Disks) == 0 {
		return nil, fmt.Errorf("没有可用磁盘")
	}

	// 找到最小磁盘
	minSize := session.Disks[0].Size
	for _, d := range session.Disks[1:] {
		if d.Size < minSize {
			minSize = d.Size
		}
	}

	estimation := EstimateCapacity(len(session.Disks), minSize, raidType)
	return &estimation, nil
}

// ListSessions 列出所有会话.
func (m *Manager) ListSessions() []*SetupSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*SetupSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// DeleteSession 删除会话.
func (m *Manager) DeleteSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}

	delete(m.sessions, id)
	return nil
}
