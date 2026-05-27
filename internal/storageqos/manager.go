// Package storageqos - QoS策略管理器
// 实现策略CRUD、I/O优先级控制、带宽限制和突发流量管理
// 对标TrueNAS QoS调度引擎
package storageqos

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager QoS策略管理器
type Manager struct {
	mu       sync.RWMutex
	policies map[string]*QoSPolicy  // policyID -> policy
	metrics  map[string]*QoSMetrics // policyID -> latest metrics
	history  map[string][]QoSMetrics // policyID -> metrics history
	logger   *zap.Logger

	// 流量令牌桶 (用于突发流量管理)
	buckets map[string]*tokenBucket // policyID -> bucket
}

// tokenBucket 令牌桶，用于突发流量控制
type tokenBucket struct {
	mu            sync.Mutex
	tokens        float64   // 当前令牌数 (MB)
	maxTokens     float64   // 最大令牌数 (MB)
	refillRate    float64   // 补充速率 (MB/s)
	lastRefill    time.Time // 上次补充时间
}

// refill 补充令牌
func (b *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = math.Min(b.maxTokens, b.tokens+b.refillRate*elapsed)
	b.lastRefill = now
}

// tryConsume 尝试消费令牌，返回实际可用量
func (b *tokenBucket) tryConsume(requestedMB float64) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens <= 0 {
		return 0
	}

	consumed := math.Min(b.tokens, requestedMB)
	b.tokens -= consumed
	return consumed
}

// NewManager 创建QoS管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		policies: make(map[string]*QoSPolicy),
		metrics:  make(map[string]*QoSMetrics),
		history:  make(map[string][]QoSMetrics),
		buckets:  make(map[string]*tokenBucket),
		logger:   logger,
	}
}

// CreatePolicy 创建QoS策略
func (m *Manager) CreatePolicy(_ context.Context, req CreatePolicyRequest) (*QoSPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查目标是否已有策略
	for _, p := range m.policies {
		if p.Target == req.Target && p.TargetType == req.TargetType {
			return nil, fmt.Errorf("目标 %s (%s) 已存在QoS策略: %s", req.Target, req.TargetType, p.Name)
		}
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now()
	policy := &QoSPolicy{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Target:      req.Target,
		TargetType:  req.TargetType,
		Priority:    req.Priority,
		Bandwidth:   req.Bandwidth,
		Burst:       req.Burst,
		Enabled:     enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.policies[policy.ID] = policy

	// 初始化令牌桶（如果启用突发）
	if req.Burst.BurstEnabled {
		m.buckets[policy.ID] = &tokenBucket{
			tokens:     req.Burst.BurstSizeMB,
			maxTokens:  req.Burst.BurstSizeMB,
			refillRate: req.Burst.BurstReplenishRateMB,
			lastRefill: now,
		}
	}

	// 初始化指标
	m.metrics[policy.ID] = &QoSMetrics{
		PolicyID:  policy.ID,
		Target:    req.Target,
		Timestamp: now,
	}

	m.logger.Info("QoS策略已创建",
		zap.String("id", policy.ID),
		zap.String("name", policy.Name),
		zap.String("target", policy.Target),
		zap.String("priority", string(policy.Priority)),
	)

	return policy, nil
}

// GetPolicy 获取QoS策略
func (m *Manager) GetPolicy(id string) (*QoSPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("QoS策略不存在: %s", id)
	}
	return policy, nil
}

// ListPolicies 列出所有QoS策略
func (m *Manager) ListPolicies() []QoSPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]QoSPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, *p)
	}
	return policies
}

// UpdatePolicy 更新QoS策略
func (m *Manager) UpdatePolicy(_ context.Context, id string, req UpdatePolicyRequest) (*QoSPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("QoS策略不存在: %s", id)
	}

	if req.Name != nil {
		policy.Name = *req.Name
	}
	if req.Description != nil {
		policy.Description = *req.Description
	}
	if req.Priority != nil {
		policy.Priority = *req.Priority
	}
	if req.Bandwidth != nil {
		policy.Bandwidth = *req.Bandwidth
	}
	if req.Burst != nil {
		policy.Burst = *req.Burst
		// 更新令牌桶配置
		if req.Burst.BurstEnabled {
			bucket, exists := m.buckets[id]
			if !exists {
				m.buckets[id] = &tokenBucket{
					tokens:     req.Burst.BurstSizeMB,
					maxTokens:  req.Burst.BurstSizeMB,
					refillRate: req.Burst.BurstReplenishRateMB,
					lastRefill: time.Now(),
				}
			} else {
				bucket.mu.Lock()
				bucket.maxTokens = req.Burst.BurstSizeMB
				bucket.refillRate = req.Burst.BurstReplenishRateMB
				bucket.mu.Unlock()
			}
		} else {
			delete(m.buckets, id)
		}
	}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}

	policy.UpdatedAt = time.Now()
	return policy, nil
}

// DeletePolicy 删除QoS策略
func (m *Manager) DeletePolicy(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("QoS策略不存在: %s", id)
	}

	delete(m.policies, id)
	delete(m.metrics, id)
	delete(m.history, id)
	delete(m.buckets, id)

	m.logger.Info("QoS策略已删除", zap.String("id", id))
	return nil
}

// GetMetrics 获取策略实时指标
func (m *Manager) GetMetrics(id string) (*QoSMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, ok := m.metrics[id]
	if !ok {
		return nil, fmt.Errorf("策略指标不存在: %s", id)
	}
	return metrics, nil
}

// GetMetricsHistory 获取策略历史指标
func (m *Manager) GetMetricsHistory(id string, from, to time.Time) (*QoSMetricsHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("QoS策略不存在: %s", id)
	}

	history, ok := m.history[id]
	if !ok {
		history = []QoSMetrics{}
	}

	// 过滤时间范围
	filtered := make([]QoSMetrics, 0)
	for _, h := range history {
		if h.Timestamp.After(from) && h.Timestamp.Before(to) {
			filtered = append(filtered, h)
		}
	}

	return &QoSMetricsHistory{
		PolicyID: id,
		Target:   policy.Target,
		From:     from,
		To:       to,
		Samples:  filtered,
	}, nil
}

// RecordIO 记录I/O操作，检查是否需要限速
// 返回值: allowedMB - 允许通过的数据量, throttledMB - 被限速的数据量
func (m *Manager) RecordIO(policyID string, readBytes, writeBytes int64) (allowedMB, throttledMB float64, err error) {
	m.mu.RLock()
	policy, ok := m.policies[policyID]
	if !ok {
		m.mu.RUnlock()
		return 0, 0, fmt.Errorf("QoS策略不存在: %s", policyID)
	}
	metrics := m.metrics[policyID]
	m.mu.RUnlock()

	if !policy.Enabled {
		readMB := float64(readBytes) / (1024 * 1024)
		writeMB := float64(writeBytes) / (1024 * 1024)
		m.updateMetrics(policyID, readMB, writeMB, 0, 0)
		return readMB + writeMB, 0, nil
	}

	readMB := float64(readBytes) / (1024 * 1024)
	writeMB := float64(writeBytes) / (1024 * 1024)
	totalMB := readMB + writeMB

	// 检查带宽限制
	var allowedReadMB, allowedWriteMB float64

	if policy.Bandwidth.ReadBPSLimit > 0 {
		allowedReadMB = math.Min(readMB, policy.Bandwidth.ReadBPSLimit/60) // 每秒限制转为每采样周期
	} else {
		allowedReadMB = readMB
	}

	if policy.Bandwidth.WriteBPSLimit > 0 {
		allowedWriteMB = math.Min(writeMB, policy.Bandwidth.WriteBPSLimit/60)
	} else {
		allowedWriteMB = writeMB
	}

	allowed := allowedReadMB + allowedWriteMB
	throttled := totalMB - allowed

	// 突发流量处理
	if policy.Burst.BurstEnabled && throttled > 0 {
		m.mu.RLock()
		bucket, hasBucket := m.buckets[policyID]
		m.mu.RUnlock()

		if hasBucket {
			burstAllowed := bucket.tryConsume(throttled)
			allowed += burstAllowed
			throttled -= burstAllowed

			bucket.mu.Lock()
			metrics.BurstUsedMB = bucket.maxTokens - bucket.tokens
			bucket.mu.Unlock()
		}
	}

	if throttled > 0 {
		metrics.ThrottleEvents++
		metrics.ThrottledReadBytes += int64(throttled * 1024 * 1024 * (readMB / totalMB))
		metrics.ThrottledWriteBytes += int64(throttled * 1024 * 1024 * (writeMB / totalMB))
	}

	m.updateMetrics(policyID, allowedReadMB, allowedWriteMB, 0, 0)
	return allowed, throttled, nil
}

// updateMetrics 更新策略指标
func (m *Manager) updateMetrics(policyID string, readMB, writeMB float64, readIOPS, writeIOPS int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, ok := m.metrics[policyID]
	if !ok {
		return
	}

	metrics.CurrentReadBPS = readMB * 60   // 转为 MB/s (假设采样周期为1分钟的简化)
	metrics.CurrentWriteBPS = writeMB * 60
	metrics.CurrentReadIOPS = readIOPS
	metrics.CurrentWriteIOPS = writeIOPS
	metrics.Timestamp = time.Now()

	// 记录历史（最多保留1440条，即24小时，每分钟一条）
	history := m.history[policyID]
	history = append(history, *metrics)
	if len(history) > 1440 {
		history = history[len(history)-1440:]
	}
	m.history[policyID] = history
}

// GetStats 获取QoS系统统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalPolicies := len(m.policies)
	enabledPolicies := 0
	totalThrottleEvents := int64(0)

	for _, p := range m.policies {
		if p.Enabled {
			enabledPolicies++
		}
	}
	for _, met := range m.metrics {
		totalThrottleEvents += met.ThrottleEvents
	}

	return map[string]interface{}{
		"total_policies":       totalPolicies,
		"enabled_policies":     enabledPolicies,
		"total_throttle_events": totalThrottleEvents,
	}
}
