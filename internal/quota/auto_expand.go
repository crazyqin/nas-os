// Package quota 提供存储配额管理功能
package quota

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 配额自动扩展策略 ==========

// AutoExpandPolicy 自动扩展策略
type AutoExpandPolicy struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	QuotaID      string        `json:"quota_id"`      // 关联的配额ID，为空表示全局策略
	VolumeName   string        `json:"volume_name"`   // 适用卷名
	Enabled      bool          `json:"enabled"`       // 是否启用
	TriggerMode  string        `json:"trigger_mode"`  // 触发模式：threshold, scheduled, manual
	TriggerRules []TriggerRule `json:"trigger_rules"` // 触发规则

	// 扩展配置
	ExpandMode    string  `json:"expand_mode"`    // 扩展模式：fixed, percent, dynamic
	ExpandValue   uint64  `json:"expand_value"`   // 扩展值（固定字节数或百分比）
	ExpandPercent float64 `json:"expand_percent"` // 扩展百分比（percent模式）
	MaxLimit      uint64  `json:"max_limit"`      // 最大限制（0表示无限制）
	MinFreeSpace  uint64  `json:"min_free_space"` // 最小保留空闲空间

	// 条件约束
	CooldownPeriod time.Duration `json:"cooldown_period"` // 扩展冷却期
	MaxExpansions  int           `json:"max_expansions"`  // 最大扩展次数（0表示无限）
	DailyLimit     uint64        `json:"daily_limit"`     // 每日最大扩展量

	// 通知配置
	NotifyBeforeExpand bool   `json:"notify_before_expand"` // 扩展前通知
	NotifyEmail        string `json:"notify_email"`         // 通知邮箱
	NotifyWebhook      string `json:"notify_webhook"`       // 通知 Webhook

	// 审计追踪
	RequireApproval bool   `json:"require_approval"` // 是否需要审批
	ApproverEmail   string `json:"approver_email"`   // 审批人邮箱

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TriggerRule 触发规则
type TriggerRule struct {
	ID              string        `json:"id"`
	Type            string        `json:"type"`             // usage_percent, free_space, growth_rate
	Operator        string        `json:"operator"`         // gt, gte, lt, lte, eq
	Value           float64       `json:"value"`            // 触发阈值
	Duration        time.Duration `json:"duration"`         // 持续时间（避免瞬时波动）
	Severity        string        `json:"severity"`         // 触发严重级别
	ConsecutiveHits int           `json:"consecutive_hits"` // 连续命中次数
}

// ExpandAction 扩展动作记录
type ExpandAction struct {
	ID             string     `json:"id"`
	PolicyID       string     `json:"policy_id"`
	QuotaID        string     `json:"quota_id"`
	TriggerType    string     `json:"trigger_type"`
	TriggerValue   float64    `json:"trigger_value"`
	PreviousLimit  uint64     `json:"previous_limit"`
	NewLimit       uint64     `json:"new_limit"`
	ExpandBytes    uint64     `json:"expand_bytes"`
	ExpandPercent  float64    `json:"expand_percent"`
	Status         string     `json:"status"` // pending, approved, executed, failed, rolled_back
	ApprovedBy     string     `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
	FailedReason   string     `json:"failed_reason,omitempty"`
	RollbackReason string     `json:"rollback_reason,omitempty"`
	RollbackAt     *time.Time `json:"rollback_at,omitempty"`
	NotifiedAt     *time.Time `json:"notified_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ExpandPolicyStats 扩展策略统计
type ExpandPolicyStats struct {
	PolicyID           string     `json:"policy_id"`
	TotalExpansions    int        `json:"total_expansions"`
	SuccessCount       int        `json:"success_count"`
	FailedCount        int        `json:"failed_count"`
	RolledBackCount    int        `json:"rolled_back_count"`
	TotalExpandedBytes uint64     `json:"total_expanded_bytes"`
	LastExpansionAt    *time.Time `json:"last_expansion_at"`
	LastExpansionBytes uint64     `json:"last_expansion_bytes"`
	AverageExpandSize  uint64     `json:"average_expand_size"`
	NextExpansionAt    *time.Time `json:"next_expansion_at,omitempty"`
}

// AutoExpandManager 自动扩展管理器
type AutoExpandManager struct {
	mu             sync.RWMutex
	policies       map[string]*AutoExpandPolicy
	actions        map[string]*ExpandAction
	actionHistory  []*ExpandAction
	stats          map[string]*ExpandPolicyStats
	quotaMgr       *Manager
	triggerTracker map[string]int       // quotaID -> consecutive trigger count
	lastExpandTime map[string]time.Time // quotaID -> last expansion time
	dailyExpand    map[string]uint64    // quotaID -> today's expansion total
	dailyReset     time.Time
	configPath     string
	stopChan       chan struct{}
	running        bool
}

// NewAutoExpandManager 创建自动扩展管理器
func NewAutoExpandManager(quotaMgr *Manager, configPath string) *AutoExpandManager {
	return &AutoExpandManager{
		policies:       make(map[string]*AutoExpandPolicy),
		actions:        make(map[string]*ExpandAction),
		actionHistory:  make([]*ExpandAction, 0),
		stats:          make(map[string]*ExpandPolicyStats),
		quotaMgr:       quotaMgr,
		triggerTracker: make(map[string]int),
		lastExpandTime: make(map[string]time.Time),
		dailyExpand:    make(map[string]uint64),
		dailyReset:     time.Now(),
		configPath:     configPath,
		stopChan:       make(chan struct{}),
	}
}

// Start 启动自动扩展管理
func (m *AutoExpandManager) Start() {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	go m.run()
}

// Stop 停止自动扩展管理
func (m *AutoExpandManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		close(m.stopChan)
		m.running = false
	}

	// 保存配置
	if m.configPath != "" {
		_ = m.saveConfig()
	}
}

// run 运行检查循环
func (m *AutoExpandManager) run() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkAllPolicies()
			m.checkPendingActions()
		}
	}
}

// ========== 策略管理 ==========

// CreatePolicy 创建扩展策略
func (m *AutoExpandManager) CreatePolicy(policy AutoExpandPolicy) (*AutoExpandPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证
	if policy.Name == "" {
		return nil, fmt.Errorf("策略名称不能为空")
	}

	if policy.ID == "" {
		policy.ID = generateID()
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	m.policies[policy.ID] = &policy
	m.stats[policy.ID] = &ExpandPolicyStats{PolicyID: policy.ID}

	_ = m.saveConfig()
	return &policy, nil
}

// GetPolicy 获取扩展策略
func (m *AutoExpandManager) GetPolicy(id string) (*AutoExpandPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略不存在")
	}
	return policy, nil
}

// ListPolicies 列出扩展策略
func (m *AutoExpandManager) ListPolicies(quotaID string, volumeName string) []*AutoExpandPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AutoExpandPolicy, 0)
	for _, p := range m.policies {
		if quotaID != "" && p.QuotaID != quotaID {
			continue
		}
		if volumeName != "" && p.VolumeName != volumeName {
			continue
		}
		result = append(result, p)
	}
	return result
}

// UpdatePolicy 更新扩展策略
func (m *AutoExpandManager) UpdatePolicy(id string, policy AutoExpandPolicy) (*AutoExpandPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略不存在")
	}

	policy.ID = id
	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()

	m.policies[id] = &policy
	_ = m.saveConfig()
	return &policy, nil
}

// DeletePolicy 删除扩展策略
func (m *AutoExpandManager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略不存在")
	}

	delete(m.policies, id)
	delete(m.stats, id)
	_ = m.saveConfig()
	return nil
}

// ========== 触发检查 ==========

// checkAllPolicies 检查所有策略
func (m *AutoExpandManager) checkAllPolicies() {
	m.mu.RLock()
	policies := make([]*AutoExpandPolicy, 0)
	for _, p := range m.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	m.mu.RUnlock()

	// 重置每日计数器
	if time.Now().Day() != m.dailyReset.Day() {
		m.mu.Lock()
		m.dailyExpand = make(map[string]uint64)
		m.dailyReset = time.Now()
		m.mu.Unlock()
	}

	for _, policy := range policies {
		m.checkPolicy(policy)
	}
}

// checkPolicy 检查单个策略
func (m *AutoExpandManager) checkPolicy(policy *AutoExpandPolicy) {
	// 获取关联的配额
	var quotaIDs []string

	if policy.QuotaID != "" {
		quotaIDs = []string{policy.QuotaID}
	} else if policy.VolumeName != "" {
		// 获取卷上所有配额
		m.quotaMgr.mu.RLock()
		for _, q := range m.quotaMgr.quotas {
			if q.VolumeName == policy.VolumeName {
				quotaIDs = append(quotaIDs, q.ID)
			}
		}
		m.quotaMgr.mu.RUnlock()
	} else {
		// 全局策略，获取所有配额
		m.quotaMgr.mu.RLock()
		for _, q := range m.quotaMgr.quotas {
			quotaIDs = append(quotaIDs, q.ID)
		}
		m.quotaMgr.mu.RUnlock()
	}

	for _, quotaID := range quotaIDs {
		m.checkQuotaPolicy(policy, quotaID)
	}
}

// checkQuotaPolicy 检查配额是否需要扩展
func (m *AutoExpandManager) checkQuotaPolicy(policy *AutoExpandPolicy, quotaID string) {
	// 获取配额使用情况
	usage, err := m.quotaMgr.GetUsage(quotaID)
	if err != nil {
		return
	}

	// 检查触发规则
	shouldExpand, triggerRule := m.evaluateTriggerRules(policy, usage)
	if !shouldExpand {
		return
	}

	// 检查冷却期
	if lastExpand, exists := m.lastExpandTime[quotaID]; exists {
		if time.Since(lastExpand) < policy.CooldownPeriod {
			return
		}
	}

	// 检查每日限制
	if policy.DailyLimit > 0 {
		if m.dailyExpand[quotaID] >= policy.DailyLimit {
			return
		}
	}

	// 检查扩展次数限制
	if policy.MaxExpansions > 0 {
		stats := m.stats[policy.ID]
		if stats != nil && stats.TotalExpansions >= policy.MaxExpansions {
			return
		}
	}

	// 计算扩展量
	expandBytes := m.calculateExpandAmount(policy, usage)
	if expandBytes == 0 {
		return
	}

	// 检查是否会超过最大限制
	if policy.MaxLimit > 0 {
		newLimit := usage.HardLimit + expandBytes
		if newLimit > policy.MaxLimit {
			expandBytes = policy.MaxLimit - usage.HardLimit
			if expandBytes == 0 {
				return // 已达最大限制
			}
		}
	}

	// 执行扩展
	m.executeExpand(policy, quotaID, expandBytes, triggerRule)
}

// evaluateTriggerRules 评估触发规则
func (m *AutoExpandManager) evaluateTriggerRules(policy *AutoExpandPolicy, usage *QuotaUsage) (bool, *TriggerRule) {
	for i := range policy.TriggerRules {
		rule := &policy.TriggerRules[i]
		triggered := false

		switch rule.Type {
		case "usage_percent":
			triggered = m.compareValue(usage.UsagePercent, rule.Operator, rule.Value)
		case "free_space":
			freePercent := 100 - usage.UsagePercent
			triggered = m.compareValue(freePercent, rule.Operator, rule.Value)
		case "free_bytes":
			triggered = m.compareValue(float64(usage.Available), rule.Operator, rule.Value)
		case "growth_rate":
			// 需要从趋势数据获取增长率
			// 简化实现，暂时跳过
		}

		if triggered {
			// 检查连续命中次数
			m.mu.Lock()
			m.triggerTracker[usage.QuotaID]++
			count := m.triggerTracker[usage.QuotaID]
			m.mu.Unlock()

			if count >= rule.ConsecutiveHits {
				return true, rule
			}
		} else {
			// 重置计数器
			m.mu.Lock()
			m.triggerTracker[usage.QuotaID] = 0
			m.mu.Unlock()
		}
	}

	return false, nil
}

// compareValue 比较值
func (m *AutoExpandManager) compareValue(actual float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return actual > threshold
	case "gte":
		return actual >= threshold
	case "lt":
		return actual < threshold
	case "lte":
		return actual <= threshold
	case "eq":
		return actual == threshold
	default:
		return false
	}
}

// calculateExpandAmount 计算扩展量
func (m *AutoExpandManager) calculateExpandAmount(policy *AutoExpandPolicy, usage *QuotaUsage) uint64 {
	switch policy.ExpandMode {
	case "fixed":
		return policy.ExpandValue
	case "percent":
		expandPercent := policy.ExpandPercent
		if expandPercent == 0 {
			expandPercent = float64(policy.ExpandValue) // 兼容旧配置，ExpandValue 作为百分比值
		}
		return uint64(float64(usage.HardLimit) * expandPercent / 100)
	case "dynamic":
		// 动态扩展：根据剩余空间计算
		minFree := policy.MinFreeSpace
		if minFree == 0 {
			minFree = uint64(float64(usage.HardLimit) * 0.2) // 默认20%空闲
		}
		needed := minFree - usage.Available
		if needed > 0 {
			return needed
		}
		return 0
	default:
		return policy.ExpandValue
	}
}

// executeExpand 执行扩展
func (m *AutoExpandManager) executeExpand(policy *AutoExpandPolicy, quotaID string, expandBytes uint64, triggerRule *TriggerRule) {
	// 创建扩展动作
	action := &ExpandAction{
		ID:           generateID(),
		PolicyID:     policy.ID,
		QuotaID:      quotaID,
		TriggerType:  triggerRule.Type,
		TriggerValue: triggerRule.Value,
		ExpandBytes:  expandBytes,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}

	// 获取当前配额
	m.quotaMgr.mu.RLock()
	quota, exists := m.quotaMgr.quotas[quotaID]
	if exists {
		action.PreviousLimit = quota.HardLimit
		action.NewLimit = quota.HardLimit + expandBytes
		action.ExpandPercent = float64(expandBytes) / float64(quota.HardLimit) * 100
	}
	m.quotaMgr.mu.RUnlock()

	if !exists {
		action.Status = "failed"
		action.FailedReason = "配额不存在"
		m.recordAction(action)
		return
	}

	// 检查是否需要审批
	if policy.RequireApproval {
		// 发送审批请求
		m.sendApprovalRequest(policy, action)
		m.recordAction(action)
		return
	}

	// 检查是否需要通知
	if policy.NotifyBeforeExpand {
		m.sendExpandNotification(policy, action, false)
		now := time.Now()
		action.NotifiedAt = &now
	}

	// 直接执行扩展
	m.doExpand(action, policy)
}

// doExpand 执行实际扩展
func (m *AutoExpandManager) doExpand(action *ExpandAction, policy *AutoExpandPolicy) {
	m.quotaMgr.mu.Lock()
	quota, exists := m.quotaMgr.quotas[action.QuotaID]
	if !exists {
		m.quotaMgr.mu.Unlock()
		action.Status = "failed"
		action.FailedReason = "配额不存在"
		m.recordAction(action)
		return
	}

	// 更新配额限制
	previousLimit := quota.HardLimit
	quota.HardLimit = action.NewLimit
	if quota.SoftLimit > 0 {
		// 同比例调整软限制
		quota.SoftLimit = uint64(float64(quota.SoftLimit) * float64(action.NewLimit) / float64(action.PreviousLimit))
	}
	quota.UpdatedAt = time.Now()

	_ = m.quotaMgr.saveConfig()
	m.quotaMgr.mu.Unlock()

	// 更新动作状态
	now := time.Now()
	action.Status = "executed"
	action.ExecutedAt = &now
	action.PreviousLimit = previousLimit

	// 更新统计
	m.mu.Lock()
	m.lastExpandTime[action.QuotaID] = now
	m.dailyExpand[action.QuotaID] += action.ExpandBytes
	m.triggerTracker[action.QuotaID] = 0

	stats := m.stats[policy.ID]
	if stats != nil {
		stats.TotalExpansions++
		stats.SuccessCount++
		stats.TotalExpandedBytes += action.ExpandBytes
		stats.LastExpansionAt = &now
		stats.LastExpansionBytes = action.ExpandBytes
		if stats.TotalExpansions > 0 {
			stats.AverageExpandSize = stats.TotalExpandedBytes / uint64(stats.TotalExpansions)
		}
	}
	m.mu.Unlock()

	// 发送完成通知
	if policy.NotifyBeforeExpand {
		m.sendExpandNotification(policy, action, true)
	}

	m.recordAction(action)
}

// recordAction 记录动作
func (m *AutoExpandManager) recordAction(action *ExpandAction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.actions[action.ID] = action
	m.actionHistory = append(m.actionHistory, action)

	// 限制历史记录数量
	if len(m.actionHistory) > 1000 {
		m.actionHistory = m.actionHistory[len(m.actionHistory)-1000:]
	}
}

// ========== 审批处理 ==========

// ApproveExpand 审批扩展
func (m *AutoExpandManager) ApproveExpand(actionID string, approver string) error {
	m.mu.Lock()
	action, exists := m.actions[actionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("动作不存在")
	}

	if action.Status != "pending" {
		m.mu.Unlock()
		return fmt.Errorf("动作状态不允许审批")
	}

	policy := m.policies[action.PolicyID]
	m.mu.Unlock()

	if policy == nil {
		return fmt.Errorf("策略不存在")
	}

	// 更新审批信息
	action.ApprovedBy = approver
	now := time.Now()
	action.ApprovedAt = &now

	// 执行扩展
	m.doExpand(action, policy)
	return nil
}

// RejectExpand 拒绝扩展
func (m *AutoExpandManager) RejectExpand(actionID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	action, exists := m.actions[actionID]
	if !exists {
		return fmt.Errorf("动作不存在")
	}

	if action.Status != "pending" {
		return fmt.Errorf("动作状态不允许拒绝")
	}

	action.Status = "rejected"
	action.FailedReason = reason
	return nil
}

// checkPendingActions 检查待处理动作
func (m *AutoExpandManager) checkPendingActions() {
	// 可以添加超时自动拒绝等逻辑
}

// ========== 回滚 ==========

// RollbackExpand 回滚扩展
func (m *AutoExpandManager) RollbackExpand(actionID string, reason string) error {
	m.mu.Lock()
	action, exists := m.actions[actionID]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("动作不存在")
	}

	if action.Status != "executed" {
		return fmt.Errorf("只能回滚已执行的动作")
	}

	// 恢复配额
	m.quotaMgr.mu.Lock()
	quota, exists := m.quotaMgr.quotas[action.QuotaID]
	if exists {
		quota.HardLimit = action.PreviousLimit
		quota.UpdatedAt = time.Now()
		_ = m.quotaMgr.saveConfig()
	}
	m.quotaMgr.mu.Unlock()

	// 更新动作状态
	now := time.Now()
	action.Status = "rolled_back"
	action.RollbackReason = reason
	action.RollbackAt = &now

	// 更新统计
	m.mu.Lock()
	if stats, exists := m.stats[action.PolicyID]; exists {
		stats.RolledBackCount++
		stats.TotalExpandedBytes -= action.ExpandBytes
	}
	m.mu.Unlock()

	return nil
}

// ========== 查询 ==========

// GetAction 获取扩展动作
func (m *AutoExpandManager) GetAction(id string) (*ExpandAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	action, exists := m.actions[id]
	if !exists {
		return nil, fmt.Errorf("动作不存在")
	}
	return action, nil
}

// ListActions 列出扩展动作
func (m *AutoExpandManager) ListActions(policyID string, quotaID string, status string, limit int) []*ExpandAction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ExpandAction, 0)
	for i := len(m.actionHistory) - 1; i >= 0; i-- {
		action := m.actionHistory[i]

		if policyID != "" && action.PolicyID != policyID {
			continue
		}
		if quotaID != "" && action.QuotaID != quotaID {
			continue
		}
		if status != "" && action.Status != status {
			continue
		}

		result = append(result, action)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// GetPolicyStats 获取策略统计
func (m *AutoExpandManager) GetPolicyStats(policyID string) (*ExpandPolicyStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, exists := m.stats[policyID]
	if !exists {
		return nil, fmt.Errorf("统计不存在")
	}
	return stats, nil
}

// GetQuotaExpansionHistory 获取配额扩展历史
func (m *AutoExpandManager) GetQuotaExpansionHistory(quotaID string, limit int) []*ExpandAction {
	return m.ListActions("", quotaID, "", limit)
}

// ========== 通知 ==========

// sendApprovalRequest 发送审批请求
func (m *AutoExpandManager) sendApprovalRequest(policy *AutoExpandPolicy, action *ExpandAction) {
	if policy.ApproverEmail == "" {
		return
	}

	// 简化实现，实际应该发送邮件
	fmt.Printf("[quota] 发送扩展审批请求: %s -> %s (审批人: %s)\n",
		action.QuotaID, formatBytesImpl(action.ExpandBytes), policy.ApproverEmail)
}

// sendExpandNotification 发送扩展通知
func (m *AutoExpandManager) sendExpandNotification(policy *AutoExpandPolicy, action *ExpandAction, completed bool) {
	// 简化实现
	if completed {
		fmt.Printf("[quota] 配额扩展完成: %s + %s (新限制: %s)\n",
			action.QuotaID, formatBytesImpl(action.ExpandBytes), formatBytesImpl(action.NewLimit))
	} else {
		fmt.Printf("[quota] 配额即将扩展: %s + %s\n",
			action.QuotaID, formatBytesImpl(action.ExpandBytes))
	}
}

// ========== 手动扩展 ==========

// ManualExpand 手动扩展配额
func (m *AutoExpandManager) ManualExpand(quotaID string, expandBytes uint64, reason string) (*ExpandAction, error) {
	// 获取配额
	m.quotaMgr.mu.RLock()
	quota, exists := m.quotaMgr.quotas[quotaID]
	m.quotaMgr.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("配额不存在")
	}

	// 创建动作
	action := &ExpandAction{
		ID:            generateID(),
		QuotaID:       quotaID,
		TriggerType:   "manual",
		TriggerValue:  0,
		PreviousLimit: quota.HardLimit,
		NewLimit:      quota.HardLimit + expandBytes,
		ExpandBytes:   expandBytes,
		ExpandPercent: float64(expandBytes) / float64(quota.HardLimit) * 100,
		Status:        "executed",
		CreatedAt:     time.Now(),
	}

	now := time.Now()
	action.ExecutedAt = &now

	// 执行扩展
	m.quotaMgr.mu.Lock()
	quota.HardLimit = action.NewLimit
	quota.UpdatedAt = time.Now()
	_ = m.quotaMgr.saveConfig()
	m.quotaMgr.mu.Unlock()

	// 记录动作
	m.recordAction(action)

	return action, nil
}

// ManualShrink 手动缩减配额
func (m *AutoExpandManager) ManualShrink(quotaID string, shrinkBytes uint64, reason string) (*ExpandAction, error) {
	// 获取配额和使用情况
	m.quotaMgr.mu.RLock()
	quota, exists := m.quotaMgr.quotas[quotaID]
	m.quotaMgr.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("配额不存在")
	}

	usage, err := m.quotaMgr.GetUsage(quotaID)
	if err != nil {
		return nil, err
	}

	// 检查是否会低于已使用量
	newLimit := quota.HardLimit - shrinkBytes
	if newLimit < usage.UsedBytes {
		return nil, fmt.Errorf("缩减后的限制不能低于已使用量")
	}

	// 创建动作
	action := &ExpandAction{
		ID:            generateID(),
		QuotaID:       quotaID,
		TriggerType:   "manual_shrink",
		TriggerValue:  0,
		PreviousLimit: quota.HardLimit,
		NewLimit:      newLimit,
		ExpandBytes:   0, // 负向扩展
		ExpandPercent: -float64(shrinkBytes) / float64(quota.HardLimit) * 100,
		Status:        "executed",
		CreatedAt:     time.Now(),
	}

	now := time.Now()
	action.ExecutedAt = &now

	// 执行缩减
	m.quotaMgr.mu.Lock()
	quota.HardLimit = newLimit
	quota.UpdatedAt = time.Now()
	_ = m.quotaMgr.saveConfig()
	m.quotaMgr.mu.Unlock()

	// 记录动作
	m.recordAction(action)

	return action, nil
}

// ========== 持久化 ==========

type expandConfig struct {
	Policies []*AutoExpandPolicy `json:"policies"`
	History  []*ExpandAction     `json:"history"`
}

func (m *AutoExpandManager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	data := expandConfig{
		Policies: make([]*AutoExpandPolicy, 0),
		History:  m.actionHistory,
	}

	for _, p := range m.policies {
		data.Policies = append(data.Policies, p)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(m.configPath), 0755)
	return os.WriteFile(m.configPath, jsonData, 0600)
}

func (m *AutoExpandManager) Load() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var loaded expandConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range loaded.Policies {
		m.policies[p.ID] = p
		m.stats[p.ID] = &ExpandPolicyStats{PolicyID: p.ID}
	}
	m.actionHistory = loaded.History

	// 重建动作索引
	for _, a := range m.actionHistory {
		m.actions[a.ID] = a
	}

	return nil
}

// ========== 预测和建议 ==========

// ExpansionRecommendation 扩展建议
type ExpansionRecommendation struct {
	QuotaID           string     `json:"quota_id"`
	TargetName        string     `json:"target_name"`
	CurrentLimit      uint64     `json:"current_limit"`
	CurrentUsage      uint64     `json:"current_usage"`
	UsagePercent      float64    `json:"usage_percent"`
	RecommendedLimit  uint64     `json:"recommended_limit"`
	ExpandAmount      uint64     `json:"expand_amount"`
	Reason            string     `json:"reason"`
	Urgency           string     `json:"urgency"` // low, medium, high, critical
	PredictedFullDate *time.Time `json:"predicted_full_date,omitempty"`
}

// GetExpansionRecommendations 获取扩展建议
func (m *AutoExpandManager) GetExpansionRecommendations() []*ExpansionRecommendation {
	usages, err := m.quotaMgr.GetAllUsage()
	if err != nil {
		return nil
	}

	recommendations := make([]*ExpansionRecommendation, 0)

	for _, usage := range usages {
		if usage.UsagePercent < 70 {
			continue // 使用率不高，不需要建议
		}

		rec := &ExpansionRecommendation{
			QuotaID:      usage.QuotaID,
			TargetName:   usage.TargetName,
			CurrentLimit: usage.HardLimit,
			CurrentUsage: usage.UsedBytes,
			UsagePercent: usage.UsagePercent,
		}

		// 计算建议扩展量
		if usage.UsagePercent >= 95 {
			rec.Urgency = "critical"
			rec.Reason = "存储即将耗尽"
			rec.RecommendedLimit = usage.HardLimit * 2 // 翻倍
		} else if usage.UsagePercent >= 85 {
			rec.Urgency = "high"
			rec.Reason = "存储空间紧张"
			rec.RecommendedLimit = uint64(float64(usage.HardLimit) * 1.5)
		} else if usage.UsagePercent >= 70 {
			rec.Urgency = "medium"
			rec.Reason = "建议预留更多空间"
			rec.RecommendedLimit = uint64(float64(usage.HardLimit) * 1.3)
		}

		rec.ExpandAmount = rec.RecommendedLimit - usage.HardLimit

		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// SimulateExpansion 模拟扩展效果
func (m *AutoExpandManager) SimulateExpansion(quotaID string, expandBytes uint64) (map[string]interface{}, error) {
	usage, err := m.quotaMgr.GetUsage(quotaID)
	if err != nil {
		return nil, err
	}

	newLimit := usage.HardLimit + expandBytes
	newUsagePercent := float64(usage.UsedBytes) / float64(newLimit) * 100
	newAvailable := newLimit - usage.UsedBytes

	return map[string]interface{}{
		"quota_id":          quotaID,
		"current_limit":     usage.HardLimit,
		"current_usage":     usage.UsedBytes,
		"current_percent":   usage.UsagePercent,
		"expand_bytes":      expandBytes,
		"new_limit":         newLimit,
		"new_usage_percent": newUsagePercent,
		"new_available":     newAvailable,
		"improvement":       100 - newUsagePercent - (100 - usage.UsagePercent),
	}, nil
}
