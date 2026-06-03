package aitokenmeter

import (
	"sync"
	"time"
)

// QuotaManager 配额管理器 (并发安全).
type QuotaManager struct {
	mu       sync.RWMutex
	quotas   map[string]*UserQuota // key: userID
	plans    map[string]*Plan      // key: planID
}

// NewQuotaManager 创建配额管理器.
func NewQuotaManager() *QuotaManager {
	return &QuotaManager{
		quotas: make(map[string]*UserQuota),
		plans:  make(map[string]*Plan),
	}
}

// ========== 配额 CRUD ==========

// SetQuota 设置用户配额.
func (qm *QuotaManager) SetQuota(quota UserQuota) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	now := time.Now()
	if quota.CreatedAt.IsZero() {
		quota.CreatedAt = now
	}
	quota.UpdatedAt = now
	qm.quotas[quota.UserID] = &quota
}

// GetQuota 获取用户配额.
func (qm *QuotaManager) GetQuota(userID string) (*UserQuota, bool) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	q, ok := qm.quotas[userID]
	if !ok {
		return nil, false
	}
	cp := *q
	return &cp, true
}

// DeleteQuota 删除用户配额.
func (qm *QuotaManager) DeleteQuota(userID string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	delete(qm.quotas, userID)
}

// ListQuotas 列出所有用户配额.
func (qm *QuotaManager) ListQuotas() []UserQuota {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	result := make([]UserQuota, 0, len(qm.quotas))
	for _, q := range qm.quotas {
		cp := *q
		result = append(result, cp)
	}
	return result
}

// ========== 套餐 CRUD ==========

// SetPlan 设置套餐.
func (qm *QuotaManager) SetPlan(plan Plan) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	now := time.Now()
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = now
	}
	plan.UpdatedAt = now
	qm.plans[plan.ID] = &plan
}

// GetPlan 获取套餐.
func (qm *QuotaManager) GetPlan(planID string) (*Plan, bool) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	p, ok := qm.plans[planID]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// DeletePlan 删除套餐.
func (qm *QuotaManager) DeletePlan(planID string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	delete(qm.plans, planID)
}

// ListPlans 列出所有套餐.
func (qm *QuotaManager) ListPlans() []Plan {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	result := make([]Plan, 0, len(qm.plans))
	for _, p := range qm.plans {
		cp := *p
		result = append(result, cp)
	}
	return result
}

// ========== 套餐分配 ==========

// AssignPlan 给用户分配套餐，自动应用套餐限额.
func (qm *QuotaManager) AssignPlan(userID, planID string) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	plan, ok := qm.plans[planID]
	if !ok {
		return ErrPlanNotFound
	}

	now := time.Now()
	quota, exists := qm.quotas[userID]
	if !exists {
		quota = &UserQuota{
			UserID:    userID,
			Enabled:   true,
			CreatedAt: now,
		}
		qm.quotas[userID] = quota
	}

	quota.PlanID = planID
	quota.Limits = copyPeriodMapInt(plan.TokenLimits)
	quota.CostLimits = copyPeriodMapFloat64(plan.CostLimits)
	quota.ProviderQuota = copyProviderMap(plan.ProviderQuota)
	quota.UpdatedAt = now

	return nil
}

// ========== 配额检查 ==========

// CheckQuota 检查用户配额是否允许指定 Token 用量.
// periodUsage: 该周期内已用 Token 数; periodCost: 该周期内已花费.
func (qm *QuotaManager) CheckQuota(userID string, tokens int, provider Provider, period QuotaPeriod, periodUsage int, periodCost float64) error {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	quota, ok := qm.quotas[userID]
	if !ok || !quota.Enabled {
		return nil // 无配额限制
	}

	// 检查 Token 配额
	if limit, exists := quota.Limits[period]; exists && limit > 0 {
		if periodUsage+tokens > limit {
			return ErrQuotaExceeded
		}
	}

	// 检查费用配额 (成本按比例估算)
	if costLimit, exists := quota.CostLimits[period]; exists && costLimit > 0 {
		if periodCost > costLimit {
			return ErrQuotaExceeded
		}
	}

	// 检查提供商配额
	if providerLimit, exists := quota.ProviderQuota[provider]; exists && providerLimit > 0 {
		if periodUsage+tokens > providerLimit {
			return ErrQuotaExceeded
		}
	}

	return nil
}

// GetEffectiveLimits 获取用户有效限额（优先用户配额 > 套餐）.
func (qm *QuotaManager) GetEffectiveLimits(userID string) map[QuotaPeriod]int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	quota, ok := qm.quotas[userID]
	if !ok {
		return nil
	}

	result := make(map[QuotaPeriod]int)
	for k, v := range quota.Limits {
		result[k] = v
	}
	return result
}

// ========== 辅助函数 ==========

func copyPeriodMapInt(src map[QuotaPeriod]int) map[QuotaPeriod]int {
	if src == nil {
		return nil
	}
	dst := make(map[QuotaPeriod]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyPeriodMapFloat64(src map[QuotaPeriod]float64) map[QuotaPeriod]float64 {
	if src == nil {
		return nil
	}
	dst := make(map[QuotaPeriod]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyProviderMap(src map[Provider]int) map[Provider]int {
	if src == nil {
		return nil
	}
	dst := make(map[Provider]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
