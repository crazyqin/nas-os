// Package usage provides AI service usage tracking and cost management
// quota.go - User quota management
package usage

import (
	"context"
	"fmt"
	"time"
)

// ========== 用户配额管理 ==========

// QuotaManager 配额管理器
type QuotaManager struct {
	tracker *TokenTracker
	config  *UsageConfig
}

// NewQuotaManager 创建配额管理器
func NewQuotaManager(tracker *TokenTracker, config *UsageConfig) *QuotaManager {
	return &QuotaManager{
		tracker: tracker,
		config:  config,
	}
}

// CreateUserQuota 创建用户配额
func (m *QuotaManager) CreateUserQuota(ctx context.Context, input *UserQuotaInput) (*UserQuota, error) {
	m.tracker.mu.Lock()
	defer m.tracker.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.tracker.userQuotas[input.UserID]; exists {
		return nil, fmt.Errorf("用户配额已存在: %s", input.UserID)
	}

	// 设置默认值
	if input.TokenQuotaPerDay == 0 && m.config.DefaultUserQuota != nil {
		input.TokenQuotaPerDay = m.config.DefaultUserQuota.TokenQuotaPerDay
	}
	if input.TokenQuotaPerMonth == 0 && m.config.DefaultUserQuota != nil {
		input.TokenQuotaPerMonth = m.config.DefaultUserQuota.TokenQuotaPerMonth
	}
	if input.CostQuotaPerDay == 0 && m.config.DefaultUserQuota != nil {
		input.CostQuotaPerDay = m.config.DefaultUserQuota.CostQuotaPerDay
	}
	if input.CostQuotaPerMonth == 0 && m.config.DefaultUserQuota != nil {
		input.CostQuotaPerMonth = m.config.DefaultUserQuota.CostQuotaPerMonth
	}
	if input.RequestQuotaPerDay == 0 && m.config.DefaultUserQuota != nil {
		input.RequestQuotaPerDay = m.config.DefaultUserQuota.RequestQuotaPerDay
	}
	if input.AlertThresholdPercent == 0 && m.config.DefaultUserQuota != nil {
		input.AlertThresholdPercent = m.config.DefaultUserQuota.AlertThresholdPercent
	}

	quota := &UserQuota{
		ID:                    generateQuotaID(),
		UserID:                input.UserID,
		UserName:              input.UserName,
		TokenQuotaPerDay:      input.TokenQuotaPerDay,
		TokenQuotaPerWeek:     input.TokenQuotaPerWeek,
		TokenQuotaPerMonth:    input.TokenQuotaPerMonth,
		TokenQuotaTotal:       input.TokenQuotaTotal,
		CostQuotaPerDay:       input.CostQuotaPerDay,
		CostQuotaPerWeek:      input.CostQuotaPerWeek,
		CostQuotaPerMonth:     input.CostQuotaPerMonth,
		CostQuotaTotal:        input.CostQuotaTotal,
		RequestQuotaPerDay:    input.RequestQuotaPerDay,
		RequestQuotaPerHour:   input.RequestQuotaPerHour,
		RequestQuotaPerMonth:  input.RequestQuotaPerMonth,
		AllowedModels:         input.AllowedModels,
		BlockedModels:         input.BlockedModels,
		MaxTokensPerReq:       input.MaxTokensPerReq,
		AlertThresholdPercent: input.AlertThresholdPercent,
		Enabled:               true,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
		ExpiresAt:             input.ExpiresAt,
		CurrentPeriodStart:    time.Now().Truncate(24 * time.Hour),
		CurrentPeriodEnd:      time.Now().Truncate(24*time.Hour).AddDate(0, 1, 0),
	}

	m.tracker.userQuotas[input.UserID] = quota

	if err := m.tracker.save(); err != nil {
		return nil, err
	}

	return quota, nil
}

// GetUserQuota 获取用户配额
func (m *QuotaManager) GetUserQuota(userID string) (*UserQuota, error) {
	m.tracker.mu.RLock()
	defer m.tracker.mu.RUnlock()

	quota, ok := m.tracker.userQuotas[userID]
	if !ok {
		return nil, ErrQuotaNotFound
	}
	return quota, nil
}

// UpdateUserQuota 更新用户配额
func (m *QuotaManager) UpdateUserQuota(userID string, input *UserQuotaInput) (*UserQuota, error) {
	m.tracker.mu.Lock()
	defer m.tracker.mu.Unlock()

	quota, ok := m.tracker.userQuotas[userID]
	if !ok {
		return nil, ErrQuotaNotFound
	}

	// 更新字段
	if input.TokenQuotaPerDay > 0 {
		quota.TokenQuotaPerDay = input.TokenQuotaPerDay
	}
	if input.TokenQuotaPerMonth > 0 {
		quota.TokenQuotaPerMonth = input.TokenQuotaPerMonth
	}
	if input.CostQuotaPerDay > 0 {
		quota.CostQuotaPerDay = input.CostQuotaPerDay
	}
	if input.CostQuotaPerMonth > 0 {
		quota.CostQuotaPerMonth = input.CostQuotaPerMonth
	}
	if input.RequestQuotaPerDay > 0 {
		quota.RequestQuotaPerDay = input.RequestQuotaPerDay
	}
	if input.AllowedModels != nil {
		quota.AllowedModels = input.AllowedModels
	}
	if input.BlockedModels != nil {
		quota.BlockedModels = input.BlockedModels
	}
	if input.MaxTokensPerReq > 0 {
		quota.MaxTokensPerReq = input.MaxTokensPerReq
	}
	if input.AlertThresholdPercent > 0 {
		quota.AlertThresholdPercent = input.AlertThresholdPercent
	}

	quota.UpdatedAt = time.Now()

	if err := m.tracker.save(); err != nil {
		return nil, err
	}

	return quota, nil
}

// DeleteUserQuota 删除用户配额
func (m *QuotaManager) DeleteUserQuota(userID string) error {
	m.tracker.mu.Lock()
	defer m.tracker.mu.Unlock()

	if _, ok := m.tracker.userQuotas[userID]; !ok {
		return ErrQuotaNotFound
	}

	delete(m.tracker.userQuotas, userID)
	return m.tracker.save()
}

// ListUserQuotas 列出所有用户配额
func (m *QuotaManager) ListUserQuotas() []*UserQuota {
	m.tracker.mu.RLock()
	defer m.tracker.mu.RUnlock()

	result := make([]*UserQuota, 0, len(m.tracker.userQuotas))
	for _, q := range m.tracker.userQuotas {
		result = append(result, q)
	}
	return result
}

// GetQuotaUsage 获取配额使用情况
func (m *QuotaManager) GetQuotaUsage(userID string) (*QuotaUsage, error) {
	m.tracker.mu.RLock()
	defer m.tracker.mu.RUnlock()

	quota, ok := m.tracker.userQuotas[userID]
	if !ok {
		return nil, ErrQuotaNotFound
	}

	usage := &QuotaUsage{
		UserID:       userID,
		UserName:     quota.UserName,
		PeriodStart:  quota.CurrentPeriodStart,
		PeriodEnd:    quota.CurrentPeriodEnd,
		TokenQuota:   quota.TokenQuotaPerMonth,
		TokensUsed:   quota.TokensUsed,
		CostQuota:    quota.CostQuotaPerMonth,
		CostUsed:     quota.CostUsed,
		RequestQuota: quota.RequestQuotaPerMonth,
		RequestsUsed: quota.RequestsUsed,
	}

	// 计算剩余量和百分比
	if usage.TokenQuota > 0 {
		usage.TokensRemaining = usage.TokenQuota - usage.TokensUsed
		if usage.TokensRemaining < 0 {
			usage.TokensRemaining = 0
		}
		usage.TokenUsagePercent = float64(usage.TokensUsed) / float64(usage.TokenQuota) * 100
	} else if usage.TokenQuota < 0 { // 无限配额
		usage.TokensRemaining = -1
		usage.TokenUsagePercent = 0
	}

	if usage.CostQuota > 0 {
		usage.CostRemaining = usage.CostQuota - usage.CostUsed
		if usage.CostRemaining < 0 {
			usage.CostRemaining = 0
		}
		usage.CostUsagePercent = usage.CostUsed / usage.CostQuota * 100
	} else if usage.CostQuota < 0 {
		usage.CostRemaining = -1
		usage.CostUsagePercent = 0
	}

	if usage.RequestQuota > 0 {
		usage.RequestsRemaining = usage.RequestQuota - usage.RequestsUsed
		if usage.RequestsRemaining < 0 {
			usage.RequestsRemaining = 0
		}
		usage.RequestUsagePercent = float64(usage.RequestsUsed) / float64(usage.RequestQuota) * 100
	} else if usage.RequestQuota < 0 {
		usage.RequestsRemaining = -1
		usage.RequestUsagePercent = 0
	}

	// 检查是否超限
	usage.IsOverTokenLimit = usage.TokenQuota > 0 && usage.TokensUsed >= usage.TokenQuota
	usage.IsOverCostLimit = usage.CostQuota > 0 && usage.CostUsed >= usage.CostQuota
	usage.IsOverRequestLimit = usage.RequestQuota > 0 && usage.RequestsUsed >= usage.RequestQuota

	// 检查是否触发告警
	usage.IsAlertTriggered = usage.TokenUsagePercent >= quota.AlertThresholdPercent ||
		usage.CostUsagePercent >= quota.AlertThresholdPercent ||
		usage.RequestUsagePercent >= quota.AlertThresholdPercent

	return usage, nil
}

// CheckQuota 检查配额（请求前验证）
func (m *QuotaManager) CheckQuota(ctx context.Context, userID string, request *QuotaCheckRequest) (*QuotaCheckResult, error) {
	m.tracker.mu.RLock()
	defer m.tracker.mu.RUnlock()

	quota, ok := m.tracker.userQuotas[userID]
	if !ok {
		// 用户无配额限制，允许请求
		return &QuotaCheckResult{Allowed: true}, nil
	}

	result := &QuotaCheckResult{
		UserID:  userID,
		Allowed: true,
	}

	// 检查Token配额
	if quota.TokenQuotaPerMonth > 0 {
		if quota.TokensUsed >= quota.TokenQuotaPerMonth {
			result.Allowed = false
			result.Reason = "token_quota_exceeded"
			result.Message = fmt.Sprintf("月度Token配额已用完: %d/%d", quota.TokensUsed, quota.TokenQuotaPerMonth)
			return result, nil
		}
	}

	// 检查成本配额
	if quota.CostQuotaPerMonth > 0 {
		if quota.CostUsed >= quota.CostQuotaPerMonth {
			result.Allowed = false
			result.Reason = "cost_quota_exceeded"
			result.Message = fmt.Sprintf("月度成本配额已用完: %.2f/%.2f", quota.CostUsed, quota.CostQuotaPerMonth)
			return result, nil
		}
	}

	// 检查请求配额
	if quota.RequestQuotaPerMonth > 0 {
		if quota.RequestsUsed >= quota.RequestQuotaPerMonth {
			result.Allowed = false
			result.Reason = "request_quota_exceeded"
			result.Message = fmt.Sprintf("月度请求配额已用完: %d/%d", quota.RequestsUsed, quota.RequestQuotaPerMonth)
			return result, nil
		}
	}

	// 检查模型限制
	if len(quota.AllowedModels) > 0 {
		allowed := false
		for _, m := range quota.AllowedModels {
			if m == request.ModelID {
				allowed = true
				break
			}
		}
		if !allowed {
			result.Allowed = false
			result.Reason = "model_not_allowed"
			result.Message = fmt.Sprintf("模型 %s 不在允许列表中", request.ModelID)
			return result, nil
		}
	}

	// 检查禁止模型
	for _, m := range quota.BlockedModels {
		if m == request.ModelID {
			result.Allowed = false
			result.Reason = "model_blocked"
			result.Message = fmt.Sprintf("模型 %s 已被禁止使用", request.ModelID)
			return result, nil
		}
	}

	// 检查单次请求Token限制
	if quota.MaxTokensPerReq > 0 && request.EstimatedTokens > quota.MaxTokensPerReq {
		result.Allowed = false
		result.Reason = "tokens_per_request_exceeded"
		result.Message = fmt.Sprintf("单次请求Token数超过限制: %d > %d", request.EstimatedTokens, quota.MaxTokensPerReq)
		return result, nil
	}

	// 检查每日配额
	now := time.Now()
	_ = now.Truncate(24 * time.Hour) // today - reserved for daily quota check

	// 获取今日使用量（简化处理，实际需要聚合今日记录）
	// TODO: 实现今日使用量统计

	// 设置警告信息
	if quota.AlertThresholdPercent > 0 {
		usagePercent := float64(quota.TokensUsed) / float64(quota.TokenQuotaPerMonth) * 100
		if usagePercent >= quota.AlertThresholdPercent {
			result.Warning = fmt.Sprintf("Token使用量已达到 %.1f%%，请注意", usagePercent)
		}
	}

	return result, nil
}

// ResetQuotaUsage 重置配额使用量
func (m *QuotaManager) ResetQuotaUsage(userID string) error {
	m.tracker.mu.Lock()
	defer m.tracker.mu.Unlock()

	quota, ok := m.tracker.userQuotas[userID]
	if !ok {
		return ErrQuotaNotFound
	}

	quota.TokensUsed = 0
	quota.CostUsed = 0
	quota.RequestsUsed = 0
	quota.CurrentPeriodStart = time.Now().Truncate(24 * time.Hour)
	quota.CurrentPeriodEnd = time.Now().Truncate(24*time.Hour).AddDate(0, 1, 0)
	quota.UpdatedAt = time.Now()

	return m.tracker.save()
}

// EnableUserQuota 启用用户配额
func (m *QuotaManager) EnableUserQuota(userID string) error {
	m.tracker.mu.Lock()
	defer m.tracker.mu.Unlock()

	quota, ok := m.tracker.userQuotas[userID]
	if !ok {
		return ErrQuotaNotFound
	}

	quota.Enabled = true
	quota.UpdatedAt = time.Now()

	return m.tracker.save()
}

// DisableUserQuota 禁用用户配额
func (m *QuotaManager) DisableUserQuota(userID string) error {
	m.tracker.mu.Lock()
	defer m.tracker.mu.Unlock()

	quota, ok := m.tracker.userQuotas[userID]
	if !ok {
		return ErrQuotaNotFound
	}

	quota.Enabled = false
	quota.UpdatedAt = time.Now()

	return m.tracker.save()
}

// ========== 内部方法 ==========

// checkQuota 内部配额检查
func (t *TokenTracker) checkQuota(quota *UserQuota, record *UsageRecord) error {
	if !quota.Enabled {
		return nil
	}

	// 检查Token配额
	if quota.TokenQuotaPerMonth > 0 {
		if quota.TokensUsed+record.TotalTokens > quota.TokenQuotaPerMonth {
			return fmt.Errorf("%w: token", ErrQuotaExceeded)
		}
	}

	// 检查成本配额
	if quota.CostQuotaPerMonth > 0 {
		if quota.CostUsed+record.TotalCost > quota.CostQuotaPerMonth {
			return fmt.Errorf("%w: cost", ErrQuotaExceeded)
		}
	}

	return nil
}

// updateQuotaUsage 更新配额使用量
func (t *TokenTracker) updateQuotaUsage(quota *UserQuota, record *UsageRecord) {
	quota.TokensUsed += record.TotalTokens
	quota.CostUsed += record.TotalCost
	quota.RequestsUsed++
	quota.UpdatedAt = time.Now()
}

// generateQuotaID 生成配额ID
func generateQuotaID() string {
	return fmt.Sprintf("quota-%d-%s", time.Now().UnixNano(), randomString(6))
}

// UserQuotaInput 用户配额输入
type UserQuotaInput struct {
	UserID                string
	UserName              string
	TokenQuotaPerDay      int64
	TokenQuotaPerWeek     int64
	TokenQuotaPerMonth    int64
	TokenQuotaTotal       int64
	CostQuotaPerDay       float64
	CostQuotaPerWeek      float64
	CostQuotaPerMonth     float64
	CostQuotaTotal        float64
	RequestQuotaPerDay    int
	RequestQuotaPerHour   int
	RequestQuotaPerMonth  int
	AllowedModels         []string
	BlockedModels         []string
	MaxTokensPerReq       int64
	AlertThresholdPercent float64
	ExpiresAt             *time.Time
}

// QuotaCheckRequest 配额检查请求
type QuotaCheckRequest struct {
	ModelID         string
	EstimatedTokens int64
	EstimatedCost   float64
}

// QuotaCheckResult 配额检查结果
type QuotaCheckResult struct {
	UserID    string
	Allowed   bool
	Reason    string
	Message   string
	Warning   string
	Remaining struct {
		Tokens   int64
		Cost     float64
		Requests int
	}
}
