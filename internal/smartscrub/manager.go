// Package smartscrub 提供智能 ZFS 擦洗调度
package smartscrub

import (
	"fmt"
	"sync"
	"time"
)

// Manager 智能擦洗管理器.
type Manager struct {
	mu       sync.RWMutex
	policies map[string]*ScrubPolicy
	records  map[string][]*ScrubRecord
	stats    ScrubStats
}

// NewManager 创建智能擦洗管理器.
func NewManager() *Manager {
	return &Manager{
		policies: make(map[string]*ScrubPolicy),
		records:  make(map[string][]*ScrubRecord),
	}
}

// CreatePolicy 创建擦洗策略.
func (m *Manager) CreatePolicy(req CreatePolicyRequest) (*ScrubPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	trigger := req.Trigger
	if trigger == "" {
		trigger = TriggerSchedule
	}
	priority := req.Priority
	if priority == "" {
		priority = PriorityNormal
	}

	id := fmt.Sprintf("scrub-%d", time.Now().UnixNano())
	policy := &ScrubPolicy{
		ID:              id,
		Name:            req.Name,
		Pools:           req.Pools,
		Trigger:         trigger,
		Schedule:        req.Schedule,
		Priority:        priority,
		ThresholdDays:   req.ThresholdDays,
		ThresholdChange: req.ThresholdChange,
		MaxDuration:     req.MaxDuration,
		Enabled:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	m.policies[id] = policy
	m.stats.TotalPolicies++
	m.stats.ActivePolicies++

	return policy, nil
}

// GetPolicy 获取策略.
func (m *Manager) GetPolicy(id string) (*ScrubPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.policies[id]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []*ScrubPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*ScrubPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return ErrPolicyNotFound
	}

	delete(m.policies, id)
	m.stats.TotalPolicies--
	m.stats.ActivePolicies--
	return nil
}

// RunScrub 执行擦洗.
func (m *Manager) RunScrub(policyID string) (*ScrubRecord, error) {
	m.mu.Lock()
	policy, ok := m.policies[policyID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrPolicyNotFound
	}
	m.mu.Unlock()

	record := &ScrubRecord{
		ID:        fmt.Sprintf("rec-%d", time.Now().UnixNano()),
		PolicyID:  policyID,
		Status:    ScrubStatusRunning,
		StartTime: time.Now(),
	}

	// 模拟擦洗
	record.EndTime = time.Now()
	record.Duration = record.EndTime.Sub(record.StartTime)
	record.Status = ScrubStatusCompleted
	record.Summary = "擦洗完成，无错误"

	m.mu.Lock()
	if len(policy.Pools) > 0 {
		record.Pool = policy.Pools[0]
	}
	m.records[policyID] = append(m.records[policyID], record)
	m.stats.TotalScrubs++
	now := time.Now()
	m.stats.LastScrubTime = &now
	m.mu.Unlock()

	return record, nil
}

// GetRecords 获取擦洗记录.
func (m *Manager) GetRecords(policyID string) []*ScrubRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.records[policyID]
}

// GetStats 获取统计.
func (m *Manager) GetStats() ScrubStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}
