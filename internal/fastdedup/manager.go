package fastdedup

import (
	"fmt"
	"time"
)

// NewFastDedupEngine 创建快速去重引擎
func NewFastDedupEngine(cfg EngineConfig) *FastDedupEngine {
	return &FastDedupEngine{
		config:     cfg,
		policies:   make(map[string]*DedupPolicy),
		blockIndex: make(map[string]*DedupBlock),
		stats:      DedupStats{},
	}
}

// Start 启动引擎
func (e *FastDedupEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return ErrEngineRunning
	}
	e.running = true
	return nil
}

// Stop 停止引擎
func (e *FastDedupEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return ErrEngineNotRunning
	}
	e.running = false
	return nil
}

// IsRunning 是否运行中
func (e *FastDedupEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// AddPolicy 添加去重策略
func (e *FastDedupEngine) AddPolicy(p *DedupPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.policies[p.Name]; exists {
		return ErrPolicyExists
	}
	e.policies[p.Name] = p
	return nil
}

// RemovePolicy 移除去重策略
func (e *FastDedupEngine) RemovePolicy(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.policies[name]; !exists {
		return ErrPolicyNotFound
	}
	delete(e.policies, name)
	return nil
}

// GetPolicy 获取去重策略
func (e *FastDedupEngine) GetPolicy(name string) (*DedupPolicy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, exists := e.policies[name]
	if !exists {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListPolicies 列出所有策略
func (e *FastDedupEngine) ListPolicies() []*DedupPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*DedupPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		result = append(result, p)
	}
	return result
}

// RunDedup 执行去重
func (e *FastDedupEngine) RunDedup(policyName string) (*DedupResult, error) {
	e.mu.RLock()
	policy, exists := e.policies[policyName]
	e.mu.RUnlock()
	if !exists {
		return nil, ErrPolicyNotFound
	}

	start := time.Now()

	e.mu.Lock()
	originalBlocks := int64(len(e.blockIndex))
	uniqueBefore := e.stats.UniqueBlocks
	e.mu.Unlock()

	// 模拟去重过程
	time.Sleep(50 * time.Millisecond)

	e.mu.Lock()
	uniqueAfter := int64(float64(originalBlocks) * 0.7) // 模拟30%去重率
	deduped := originalBlocks - uniqueAfter
	spaceSaved := deduped * policy.MaxBlockSize
	duration := time.Since(start).Milliseconds()

	e.stats.TotalBlocks = originalBlocks
	e.stats.UniqueBlocks = uniqueAfter
	e.stats.DuplicateBlocks = deduped
	e.stats.SpaceSaved = spaceSaved
	if uniqueAfter > 0 {
		e.stats.DedupRatio = float64(originalBlocks) / float64(uniqueAfter)
	}
	e.stats.LastRunAt = time.Now()
	e.stats.Duration = duration
	e.mu.Unlock()

	_ = uniqueBefore

	return &DedupResult{
		ScannedBlocks:   originalBlocks,
		DedupedBlocks:   deduped,
		SpaceSavedBytes: spaceSaved,
		Duration:        duration,
		DedupRatio:      float64(originalBlocks) / float64(uniqueAfter),
	}, nil
}

// GetStats 获取去重统计
func (e *FastDedupEngine) GetStats() DedupStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// ResetStats 重置统计
func (e *FastDedupEngine) ResetStats() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats = DedupStats{}
}

// RegisterBlock 注册数据块
func (e *FastDedupEngine) RegisterBlock(hash string, size int64, tier StorageTier) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if existing, exists := e.blockIndex[hash]; exists {
		existing.RefCount++
		return nil
	}
	e.blockIndex[hash] = &DedupBlock{
		Hash:      hash,
		Size:      size,
		RefCount:  1,
		Tier:      tier,
		CreatedAt: time.Now(),
	}
	e.stats.UniqueBlocks++
	return nil
}

// GetBlockCount 获取块数量
func (e *FastDedupEngine) GetBlockCount() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return int64(len(e.blockIndex))
}

// String 字符串表示
func (e *FastDedupEngine) String() string {
	return fmt.Sprintf("FastDedupEngine{mode=%s, algo=%s, nvme=%v, blocks=%d}",
		e.config.DefaultMode, e.config.DefaultAlgo, e.config.NVMeOptimized, e.GetBlockCount())
}
