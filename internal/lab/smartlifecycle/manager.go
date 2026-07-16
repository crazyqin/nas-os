// Package smartlifecycle - 智能文件生命周期管理器
package smartlifecycle

import (
	"fmt"
	"sync"
	"time"
)

// Manager 生命周期管理器.
type Manager struct {
	mu          sync.RWMutex
	policies    map[string]*LifecyclePolicy
	scanResults []*ScanResult
	executions  []*ExecutionResult
	stats       *LifecycleStats
}

// NewManager 创建生命周期管理器.
func NewManager() *Manager {
	return &Manager{
		policies:    make(map[string]*LifecyclePolicy),
		scanResults: make([]*ScanResult, 0),
		executions:  make([]*ExecutionResult, 0),
		stats:       &LifecycleStats{StatusBreakdown: make(map[string]int)},
	}
}

// CreatePolicy 创建策略.
func (m *Manager) CreatePolicy(policy *LifecyclePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	m.stats.TotalPolicies++
	if policy.Enabled {
		m.stats.ActivePolicies++
	}
	return nil
}

// GetPolicy 获取策略.
func (m *Manager) GetPolicy(id string) (*LifecyclePolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return p, nil
}

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []*LifecyclePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*LifecyclePolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// UpdatePolicy 更新策略.
func (m *Manager) UpdatePolicy(policy *LifecyclePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[policy.ID]; !ok {
		return fmt.Errorf("policy %s not found", policy.ID)
	}
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}
	delete(m.policies, id)
	m.stats.TotalPolicies--
	return nil
}

// RunScan 运行扫描.
func (m *Manager) RunScan() *ScanResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	start := time.Now()
	result := &ScanResult{
		ID:       fmt.Sprintf("scan-%d", start.UnixNano()),
		ScanTime: start,
		// 模拟扫描结果
		TotalFiles:  1250,
		TotalSize:   500 * 1024 * 1024 * 1024, // 500GB
		ActiveFiles: 800,
		WarmFiles:   300,
		ColdFiles:   150,
		Candidates:  make([]*FileRecord, 0),
		SavedBytes:  50 * 1024 * 1024 * 1024, // 50GB
		Duration:    time.Since(start),
	}

	m.scanResults = append(m.scanResults, result)
	m.stats.TotalScans++
	now := time.Now()
	m.stats.LastScanTime = &now
	return result
}

// ExecutePolicy 执行策略.
func (m *Manager) ExecutePolicy(policyID string) (*ExecutionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", policyID)
	}

	start := time.Now()
	result := &ExecutionResult{
		ID:         fmt.Sprintf("exec-%d", start.UnixNano()),
		PolicyID:   policyID,
		PolicyName: policy.Name,
		StartTime:  start,
		EndTime:    start.Add(5 * time.Second), // 模拟执行
		Processed:  150,
		Success:    148,
		Failed:     2,
		FreedBytes: 15 * 1024 * 1024 * 1024, // 15GB
		Errors:     []string{"permission denied: /data/protected/file1", "file locked: /data/active/file2"},
	}

	m.executions = append(m.executions, result)
	m.stats.TotalExecutions++
	m.stats.TotalFreedBytes += result.FreedBytes
	now := time.Now()
	policy.LastRunAt = &now
	return result, nil
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() *LifecycleStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetScanResults 获取扫描结果.
func (m *Manager) GetScanResults() []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scanResults
}

// GetExecutions 获取执行记录.
func (m *Manager) GetExecutions() []*ExecutionResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.executions
}
