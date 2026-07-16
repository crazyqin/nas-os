// Package costgovernance 提供多云成本治理功能
package costgovernance

import (
	"fmt"
	"sync"
	"time"
)

// Manager 成本治理引擎.
type Manager struct {
	mu       sync.RWMutex
	policies map[string]*CostPolicy
	budgets  map[string]*Budget
	alerts   []*CostAlert
	usages   map[string]*ResourceUsage
	reports  []*CostReport
}

// NewManager 创建成本治理管理器.
func NewManager() *Manager {
	return &Manager{
		policies: make(map[string]*CostPolicy),
		budgets:  make(map[string]*Budget),
		alerts:   make([]*CostAlert, 0),
		usages:   make(map[string]*ResourceUsage),
		reports:  make([]*CostReport, 0),
	}
}

// ========== 策略管理 ==========

// CreatePolicy 创建成本策略.
func (m *Manager) CreatePolicy(policy *CostPolicy) error {
	if policy.ID == "" {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// GetPolicy 获取成本策略.
func (m *Manager) GetPolicy(id string) (*CostPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.policies[id]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListPolicies 列出所有成本策略.
func (m *Manager) ListPolicies() []*CostPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policies := make([]*CostPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// DeletePolicy 删除成本策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.policies[id]; !ok {
		return ErrPolicyNotFound
	}
	delete(m.policies, id)
	return nil
}

// ========== 预算管理 ==========

// CreateBudget 创建预算.
func (m *Manager) CreateBudget(budget *Budget) error {
	if budget.ID == "" || budget.Amount <= 0 {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	budget.CreatedAt = time.Now()
	m.budgets[budget.ID] = budget
	return nil
}

// GetBudget 获取预算.
func (m *Manager) GetBudget(id string) (*Budget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.budgets[id]
	if !ok {
		return nil, ErrBudgetNotFound
	}
	return b, nil
}

// ListBudgets 列出所有预算.
func (m *Manager) ListBudgets() []*Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	budgets := make([]*Budget, 0, len(m.budgets))
	for _, b := range m.budgets {
		budgets = append(budgets, b)
	}
	return budgets
}

// UpdateBudgetSpent 更新预算已花费金额并检查告警.
func (m *Manager) UpdateBudgetSpent(id string, spent float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.budgets[id]
	if !ok {
		return ErrBudgetNotFound
	}
	b.Spent = spent
	// 检查告警阈值
	for _, threshold := range b.AlertThresholds {
		percent := (spent / b.Amount) * 100
		if percent >= threshold {
			alert := &CostAlert{
				ID:        fmt.Sprintf("alert-%s-%.0f-%d", id, threshold, time.Now().Unix()),
				BudgetID:  id,
				Provider:  b.Provider,
				Severity:  m.calcSeverity(percent),
				Message:   fmt.Sprintf("预算「%s」已使用 %.1f%%（阈值 %.0f%%）", b.Name, percent, threshold),
				Threshold: threshold,
				Actual:    percent,
				CreatedAt: time.Now(),
			}
			m.alerts = append(m.alerts, alert)
		}
	}
	return nil
}

func (m *Manager) calcSeverity(percent float64) AlertSeverity {
	switch {
	case percent >= 100:
		return SeverityCritical
	case percent >= 80:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

// ========== 告警管理 ==========

// ListAlerts 列出告警.
func (m *Manager) ListAlerts(provider CloudProvider, acknowledged bool) []*CostAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*CostAlert, 0)
	for _, a := range m.alerts {
		if provider != "" && a.Provider != provider {
			continue
		}
		if a.Acknowledged != acknowledged && acknowledged {
			continue
		}
		result = append(result, a)
	}
	return result
}

// AcknowledgeAlert 确认告警.
func (m *Manager) AcknowledgeAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == id {
			a.Acknowledged = true
			return nil
		}
	}
	return ErrAlertNotFound
}

// ========== 资源使用追踪 ==========

// UpdateResourceUsage 更新资源使用情况.
func (m *Manager) UpdateResourceUsage(usage *ResourceUsage) error {
	if usage.ID == "" {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	usage.UpdatedAt = time.Now()
	m.usages[usage.ID] = usage
	return nil
}

// GetResourceUsage 获取资源使用情况.
func (m *Manager) GetResourceUsage(id string) (*ResourceUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.usages[id]
	if !ok {
		return nil, ErrInvalidInput
	}
	return u, nil
}

// ListResourceUsages 列出所有资源使用情况.
func (m *Manager) ListResourceUsages(provider CloudProvider) []*ResourceUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ResourceUsage, 0)
	for _, u := range m.usages {
		if provider != "" && u.Provider != provider {
			continue
		}
		result = append(result, u)
	}
	return result
}

// ========== 报表 ==========

// GenerateReport 生成成本报表.
func (m *Manager) GenerateReport(provider CloudProvider, start, end time.Time) *CostReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &CostReport{
		ID:             fmt.Sprintf("report-%s-%d", provider, time.Now().Unix()),
		Provider:       provider,
		PeriodStart:    start,
		PeriodEnd:      end,
		ByService:      make(map[string]float64),
		ByRegion:       make(map[string]float64),
		ByResourceType: make(map[ResourceType]float64),
		GeneratedAt:    time.Now(),
	}

	// 汇总资源成本
	for _, u := range m.usages {
		if provider != "" && u.Provider != provider {
			continue
		}
		days := end.Sub(start).Hours() / 24
		cost := u.DailyCost * days
		report.TotalCost += cost
		report.ByRegion[u.Region] += cost
		report.ByResourceType[u.ResourceType] += cost
	}

	// 计算优化建议节省
	report.OptimizationSavings = m.calcOptimizationSavings(provider)

	m.reports = append(m.reports, report)
	return report
}

func (m *Manager) calcOptimizationSavings(provider CloudProvider) float64 {
	savings := 0.0
	for _, u := range m.usages {
		if provider != "" && u.Provider != provider {
			continue
		}
		// 低使用率资源的优化建议
		if u.CPUPercent < 10 && u.DailyCost > 0 {
			savings += u.DailyCost * 0.3 // 建议缩容可节省30%
		}
		if u.StorageUsedGB > 0 && u.StorageTotalGB > 0 {
			utilization := u.StorageUsedGB / u.StorageTotalGB
			if utilization < 0.2 {
				savings += u.DailyCost * 0.2 // 存储利用率过低
			}
		}
	}
	return savings
}

// GetCostSummary 获取成本汇总.
func (m *Manager) GetCostSummary(provider CloudProvider) map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalDaily := 0.0
	byProvider := make(map[string]float64)
	for _, u := range m.usages {
		if provider != "" && u.Provider != provider {
			continue
		}
		totalDaily += u.DailyCost
		byProvider[string(u.Provider)] += u.DailyCost
	}

	summary := map[string]float64{
		"daily_total": totalDaily,
		"monthly_est": totalDaily * 30,
		"yearly_est":  totalDaily * 365,
	}
	for k, v := range byProvider {
		summary[fmt.Sprintf("daily_%s", k)] = v
	}
	return summary
}
