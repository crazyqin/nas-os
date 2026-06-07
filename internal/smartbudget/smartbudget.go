// Package smartbudget 提供智能预算管理功能
package smartbudget

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 智能预算管理器.
type Manager struct {
	mu            sync.RWMutex
	plans         map[string]*BudgetPlan
	costs         map[string][]CostBreakdown
	optimizations []CostOptimization
	alerts        []BudgetAlert
}

// NewManager 创建管理器实例.
func NewManager() *Manager {
	return &Manager{
		plans:         make(map[string]*BudgetPlan),
		costs:         make(map[string][]CostBreakdown),
		optimizations: make([]CostOptimization, 0),
		alerts:        make([]BudgetAlert, 0),
	}
}

// ========== 预算计划管理 ==========

// CreatePlan 创建预算计划.
func (m *Manager) CreatePlan(req CreatePlanRequest) (*BudgetPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" || req.Department == "" {
		return nil, ErrInvalidInput
	}

	// 设置默认值
	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}
	period := req.Period
	if period == "" {
		period = PeriodMonthly
	}

	now := time.Now()
	plan := &BudgetPlan{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Department: req.Department,
		Project:    req.Project,
		Owner:      req.Owner,
		MonthlyCap: req.MonthlyCap,
		CurrentUse: 0,
		Currency:   currency,
		Period:     period,
		Provider:   req.Provider,
		Tags:       req.Tags,
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	m.plans[plan.ID] = plan
	return plan, nil
}

// ListPlans 列出所有预算计划.
func (m *Manager) ListPlans() []*BudgetPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*BudgetPlan, 0, len(m.plans))
	for _, p := range m.plans {
		plans = append(plans, p)
	}
	return plans
}

// GetPlan 获取预算计划详情.
func (m *Manager) GetPlan(id string) (*BudgetPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, ErrPlanNotFound
	}
	return plan, nil
}

// UpdatePlanUsage 更新预算使用量.
func (m *Manager) UpdatePlanUsage(id string, amount float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[id]
	if !ok {
		return ErrPlanNotFound
	}

	plan.CurrentUse += amount
	plan.UpdatedAt = time.Now()

	// 检查是否触发告警
	m.checkAlert(plan)

	return nil
}

// checkAlert 检查并生成告警.
func (m *Manager) checkAlert(plan *BudgetPlan) {
	if plan.MonthlyCap <= 0 {
		return
	}

	usagePercent := (plan.CurrentUse / plan.MonthlyCap) * 100

	var level AlertLevel
	var message string
	var threshold float64

	switch {
	case usagePercent >= 100:
		level = AlertLevelCritical
		message = fmt.Sprintf("预算 %s 已超支! 当前使用: %.2f%%", plan.Name, usagePercent)
		threshold = 100
	case usagePercent >= 80:
		level = AlertLevelWarning
		message = fmt.Sprintf("预算 %s 使用率已达 %.2f%%，即将超支", plan.Name, usagePercent)
		threshold = 80
	case usagePercent >= 50:
		level = AlertLevelInfo
		message = fmt.Sprintf("预算 %s 使用率已达 %.2f%%", plan.Name, usagePercent)
		threshold = 50
	default:
		return
	}

	alert := BudgetAlert{
		ID:         uuid.New().String(),
		PlanID:     plan.ID,
		PlanName:   plan.Name,
		Level:      level,
		Message:    message,
		Threshold:  threshold,
		CurrentUse: plan.CurrentUse,
		BudgetCap:  plan.MonthlyCap,
		CreatedAt:  time.Now(),
	}

	m.alerts = append(m.alerts, alert)
}

// GetAlerts 获取告警列表.
func (m *Manager) GetAlerts() []BudgetAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.alerts
}

// ========== 成本追踪 ==========

// AddCostBreakdown 添加成本明细.
func (m *Manager) AddCostBreakdown(dept string, breakdown CostBreakdown) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.costs[dept] = append(m.costs[dept], breakdown)
}

// GetCostBreakdowns 获取成本明细.
func (m *Manager) GetCostBreakdowns(query CostQueryRequest) []CostBreakdown {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]CostBreakdown, 0)
	for dept, breakdowns := range m.costs {
		if query.Department != "" && dept != query.Department {
			continue
		}
		for _, b := range breakdowns {
			if query.Category != "" && b.Category != query.Category {
				continue
			}
			if query.Provider != "" && b.Provider != query.Provider {
				continue
			}
			b.Department = dept
			result = append(result, b)
		}
	}
	return result
}

// ========== 成本趋势分析 ==========

// GetCostTrends 获取成本趋势数据.
func (m *Manager) GetCostTrends(query TrendQueryRequest) []CostTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	months := query.Months
	if months <= 0 {
		months = 6
	}

	trends := make([]CostTrend, 0, months)
	now := time.Now()

	// 生成历史趋势数据（模拟）
	for i := months - 1; i >= 0; i-- {
		date := now.AddDate(0, -i, 0)
		amount := 1000 + float64(i)*50 // 模拟递增趋势

		if query.Department != "" {
			// 过滤部门数据
			if costs, ok := m.costs[query.Department]; ok {
				amount = 0
				for _, c := range costs {
					if query.Category == "" || c.Category == query.Category {
						amount += c.Amount
					}
				}
			}
		}

		trends = append(trends, CostTrend{
			Date:       date,
			Amount:     amount,
			Category:   query.Category,
			Department: query.Department,
		})
	}

	return trends
}

// ========== 成本预测 ==========

// ForecastCost 预测未来成本.
func (m *Manager) ForecastCost(months int) []CostForecast {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if months <= 0 {
		months = 3
	}

	forecasts := make([]CostForecast, 0, months)
	now := time.Now()

	// 简单线性预测（实际应使用更复杂的算法）
	baseAmount := 1000.0
	growthRate := 0.05 // 5% 月增长率

	for i := 1; i <= months; i++ {
		date := now.AddDate(0, i, 0)
		predicted := baseAmount * (1 + growthRate*float64(i))
		confidence := 100 - float64(i)*10 // 越远期越不确定

		if confidence < 50 {
			confidence = 50
		}

		forecasts = append(forecasts, CostForecast{
			Date:            date,
			PredictedAmount: predicted,
			Confidence:      confidence,
			LowerBound:      predicted * 0.8,
			UpperBound:      predicted * 1.2,
		})
	}

	return forecasts
}

// ========== 优化建议 ==========

// GetOptimizations 获取成本优化建议.
func (m *Manager) GetOptimizations() []CostOptimization {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.optimizations
}

// AddOptimization 添加优化建议.
func (m *Manager) AddOptimization(opt CostOptimization) {
	m.mu.Lock()
	defer m.mu.Unlock()

	opt.ID = uuid.New().String()
	opt.CreatedAt = time.Now()
	m.optimizations = append(m.optimizations, opt)
}

// GenerateOptimizationSuggestions 生成优化建议.
func (m *Manager) GenerateOptimizationSuggestions(dept string) []CostOptimization {
	m.mu.Lock()
	defer m.mu.Unlock()

	suggestions := make([]CostOptimization, 0)

	// 基于成本数据生成建议
	if costs, ok := m.costs[dept]; ok {
		for _, cost := range costs {
			// 存储优化建议
			if cost.Category == "storage" && cost.Amount > 500 {
				suggestions = append(suggestions, CostOptimization{
					ID:          uuid.New().String(),
					Type:        OptTypeColdMigration,
					Description: fmt.Sprintf("建议将不常访问的数据迁移到冷存储，预计可节省 %.0f%% 成本", 30.0),
					SavingEst:   cost.Amount * 0.3,
					Priority:    PriorityHigh,
					Resource:    cost.Category,
					Department:  dept,
					CreatedAt:   time.Now(),
				})
			}

			// 数据压缩建议
			if cost.Category == "storage" && cost.Amount > 200 {
				suggestions = append(suggestions, CostOptimization{
					ID:          uuid.New().String(),
					Type:        OptTypeCompress,
					Description: "建议对历史数据进行压缩存储",
					SavingEst:   cost.Amount * 0.15,
					Priority:    PriorityMedium,
					Resource:    cost.Category,
					Department:  dept,
					CreatedAt:   time.Now(),
				})
			}
		}
	}

	m.optimizations = append(m.optimizations, suggestions...)
	return suggestions
}

// ========== 月度报告 ==========

// GenerateMonthlyReport 生成月度报告.
func (m *Manager) GenerateMonthlyReport(month string) *MonthlyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &MonthlyReport{
		Month:         month,
		Breakdown:     make([]CostBreakdown, 0),
		Trends:        make([]CostTrend, 0),
		Alerts:        make([]BudgetAlert, 0),
		Optimizations: make([]CostOptimization, 0),
	}

	// 汇总所有部门成本
	totalCost := 0.0
	for dept, costs := range m.costs {
		for _, c := range costs {
			totalCost += c.Amount
			c.Department = dept
			report.Breakdown = append(report.Breakdown, c)
		}
	}

	report.TotalCost = totalCost

	// 计算预算使用率
	totalCap := 0.0
	for _, plan := range m.plans {
		totalCap += plan.MonthlyCap
	}
	report.BudgetCap = totalCap

	if totalCap > 0 {
		report.Usage = (totalCost / totalCap) * 100
	}

	// 添加趋势数据
	report.Trends = m.GetCostTrends(TrendQueryRequest{Months: 6})

	// 添加相关告警
	for _, alert := range m.alerts {
		alertMonth := alert.CreatedAt.Format("2006-01")
		if alertMonth == month {
			report.Alerts = append(report.Alerts, alert)
		}
	}

	// 添加优化建议
	report.Optimizations = m.optimizations

	return report
}
