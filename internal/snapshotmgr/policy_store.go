package snapshotmgr

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PolicyStore 保留策略存储
type PolicyStore struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	policies map[string]*RetentionPolicy
}

// NewPolicyStore 创建策略存储
func NewPolicyStore(logger *zap.Logger) *PolicyStore {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PolicyStore{
		logger:   logger,
		policies: make(map[string]*RetentionPolicy),
	}
}

// Create 创建保留策略
func (ps *PolicyStore) Create(policy *RetentionPolicy) (*RetentionPolicy, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	policy.ID = id
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	// 默认值
	if policy.TargetScope == "" {
		policy.TargetScope = "global"
	}

	ps.policies[id] = policy

	ps.logger.Info("retention policy created",
		zap.String("policy_id", id),
		zap.String("name", policy.Name),
		zap.String("target_scope", policy.TargetScope),
	)

	return policy, nil
}

// Get 获取策略
func (ps *PolicyStore) Get(id string) (*RetentionPolicy, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	p, ok := ps.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return p, nil
}

// List 列出策略（可按 scope 和 ref 过滤）
func (ps *PolicyStore) List(targetScope, targetRef string) []RetentionPolicy {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var result []RetentionPolicy
	for _, p := range ps.policies {
		if targetScope != "" && p.TargetScope != targetScope {
			continue
		}
		if targetRef != "" && p.TargetRef != targetRef {
			continue
		}
		result = append(result, *p)
	}
	return result
}

// Update 更新策略
func (ps *PolicyStore) Update(id string, policy *RetentionPolicy) (*RetentionPolicy, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	existing, ok := ps.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}

	// 选择性更新
	if policy.Name != "" {
		existing.Name = policy.Name
	}
	if policy.Description != "" {
		existing.Description = policy.Description
	}
	existing.Enabled = policy.Enabled
	existing.Minutely = policy.Minutely
	existing.Hourly = policy.Hourly
	existing.Daily = policy.Daily
	existing.Weekly = policy.Weekly
	existing.Monthly = policy.Monthly
	existing.Yearly = policy.Yearly
	existing.UpdatedAt = time.Now()

	ps.policies[id] = existing

	ps.logger.Info("retention policy updated", zap.String("policy_id", id))
	return existing, nil
}

// Delete 删除策略
func (ps *PolicyStore) Delete(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if _, ok := ps.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	delete(ps.policies, id)
	ps.logger.Info("retention policy deleted", zap.String("policy_id", id))
	return nil
}
