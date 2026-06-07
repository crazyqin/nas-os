// Package aitokenbudget 提供 AI Token 预算管理功能
// 对标群晖 AI Console Token 配额管理，增强：
// - 按用户/服务/模型的 Token 预算分配
// - 实时用量追踪和成本计算
// - 预算告警和自动降级
// - 多模型成本对比分析
// - 月度/日度用量趋势预测
package aitokenbudget

import (
	"fmt"
	"sync"
	"time"
)

// BudgetPeriod 预算周期
type BudgetPeriod string

const (
	PeriodDaily   BudgetPeriod = "daily"
	PeriodWeekly  BudgetPeriod = "weekly"
	PeriodMonthly BudgetPeriod = "monthly"
)

// BudgetStatus 预算状态
type BudgetStatus string

const (
	StatusNormal   BudgetStatus = "normal"
	StatusWarning  BudgetStatus = "warning"  // 80%
	StatusCritical BudgetStatus = "critical" // 95%
	StatusExceeded BudgetStatus = "exceeded" // 100%
)

// CostTier 成本等级（每 1M tokens）
type CostTier struct {
	ModelID         string  `json:"modelId"`
	ModelName       string  `json:"modelName"`
	Provider        string  `json:"provider"`
	InputCostPer1M  float64 `json:"inputCostPer1M"`  // 输入成本 $/1M tokens
	OutputCostPer1M float64 `json:"outputCostPer1M"` // 输出成本 $/1M tokens
	Currency        string  `json:"currency"`
}

// Budget 预算配置
type Budget struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	OwnerType     string       `json:"ownerType"` // user, service, group
	OwnerID       string       `json:"ownerId"`
	Period        BudgetPeriod `json:"period"`
	MaxTokens     int64        `json:"maxTokens"`     // Token 上限
	MaxCost       float64      `json:"maxCost"`       // 成本上限（美元）
	WarnThreshold float64      `json:"warnThreshold"` // 告警阈值（0.8 = 80%）
	HardLimit     bool         `json:"hardLimit"`     // 是否硬限制
	FallbackModel string       `json:"fallbackModel"` // 超限后降级模型
	Enabled       bool         `json:"enabled"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

// UsageRecord 用量记录
type UsageRecord struct {
	ID               string    `json:"id"`
	BudgetID         string    `json:"budgetId"`
	UserID           string    `json:"userId"`
	ServiceName      string    `json:"serviceName"`
	ModelID          string    `json:"modelId"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	Cost             float64   `json:"cost"`
	Timestamp        time.Time `json:"timestamp"`
	RequestID        string    `json:"requestId"`
}

// UsageSummary 用量汇总
type UsageSummary struct {
	BudgetID        string       `json:"budgetId"`
	Period          BudgetPeriod `json:"period"`
	PeriodStart     time.Time    `json:"periodStart"`
	PeriodEnd       time.Time    `json:"periodEnd"`
	TotalTokens     int64        `json:"totalTokens"`
	TotalCost       float64      `json:"totalCost"`
	RequestCount    int          `json:"requestCount"`
	AvgTokensPerReq float64      `json:"avgTokensPerReq"`
	Status          BudgetStatus `json:"status"`
	UsagePercent    float64      `json:"usagePercent"`
	ProjectedTokens int64        `json:"projectedTokens"` // 预测用量
	ProjectedCost   float64      `json:"projectedCost"`
	TopModels       []ModelUsage `json:"topModels"`
}

// ModelUsage 模型用量
type ModelUsage struct {
	ModelID      string  `json:"modelId"`
	ModelName    string  `json:"modelName"`
	TotalTokens  int64   `json:"totalTokens"`
	TotalCost    float64 `json:"totalCost"`
	RequestCount int     `json:"requestCount"`
	AvgLatency   float64 `json:"avgLatencyMs"`
}

// CostAnalysis 成本分析
type CostAnalysis struct {
	Period             string             `json:"period"`
	TotalCost          float64            `json:"totalCost"`
	CostByModel        map[string]float64 `json:"costByModel"`
	CostByUser         map[string]float64 `json:"costByUser"`
	CostByService      map[string]float64 `json:"costByService"`
	Trend              string             `json:"trend"`              // increasing, stable, decreasing
	SavingsOpportunity float64            `json:"savingsOpportunity"` // 可节省金额
	Recommendations    []string           `json:"recommendations"`
}

// Manager 预算管理器
type Manager struct {
	mu        sync.RWMutex
	config    *Config
	budgets   map[string]*Budget
	records   []*UsageRecord
	costTiers map[string]*CostTier
	alerts    []*BudgetAlert
}

// Config 管理器配置
type Config struct {
	Enabled           bool          `json:"enabled"`
	DefaultPeriod     BudgetPeriod  `json:"defaultPeriod"`
	WarnThreshold     float64       `json:"warnThreshold"`     // 默认 0.8
	CriticalThreshold float64       `json:"criticalThreshold"` // 默认 0.95
	RecordRetention   time.Duration `json:"recordRetention"`   // 记录保留时间
	AlertWebhook      string        `json:"alertWebhook"`
	Currency          string        `json:"currency"` // USD, CNY
}

// BudgetAlert 预算告警
type BudgetAlert struct {
	ID        string    `json:"id"`
	BudgetID  string    `json:"budgetId"`
	Level     string    `json:"level"` // info, warning, critical, exceeded
	Message   string    `json:"message"`
	Threshold float64   `json:"threshold"`
	Actual    float64   `json:"actual"`
	Timestamp time.Time `json:"timestamp"`
	Dismissed bool      `json:"dismissed"`
}

// NewManager 创建预算管理器
func NewManager(config *Config) *Manager {
	if config.WarnThreshold == 0 {
		config.WarnThreshold = 0.8
	}
	if config.CriticalThreshold == 0 {
		config.CriticalThreshold = 0.95
	}
	if config.Currency == "" {
		config.Currency = "USD"
	}
	return &Manager{
		config:    config,
		budgets:   make(map[string]*Budget),
		records:   make([]*UsageRecord, 0),
		costTiers: defaultCostTiers(),
		alerts:    make([]*BudgetAlert, 0),
	}
}

func defaultCostTiers() map[string]*CostTier {
	return map[string]*CostTier{
		"gpt-4o": {
			ModelID: "gpt-4o", ModelName: "GPT-4o", Provider: "openai",
			InputCostPer1M: 2.50, OutputCostPer1M: 10.00, Currency: "USD",
		},
		"gpt-4o-mini": {
			ModelID: "gpt-4o-mini", ModelName: "GPT-4o Mini", Provider: "openai",
			InputCostPer1M: 0.15, OutputCostPer1M: 0.60, Currency: "USD",
		},
		"claude-3.5-sonnet": {
			ModelID: "claude-3.5-sonnet", ModelName: "Claude 3.5 Sonnet", Provider: "anthropic",
			InputCostPer1M: 3.00, OutputCostPer1M: 15.00, Currency: "USD",
		},
		"deepseek-v3": {
			ModelID: "deepseek-v3", ModelName: "DeepSeek V3", Provider: "deepseek",
			InputCostPer1M: 0.27, OutputCostPer1M: 1.10, Currency: "USD",
		},
		"qwen-max": {
			ModelID: "qwen-max", ModelName: "通义千问 Max", Provider: "alibaba",
			InputCostPer1M: 0.40, OutputCostPer1M: 1.20, Currency: "USD",
		},
		"glm-4": {
			ModelID: "glm-4", ModelName: "GLM-4", Provider: "zhipu",
			InputCostPer1M: 0.70, OutputCostPer1M: 0.70, Currency: "USD",
		},
		"local": {
			ModelID: "local", ModelName: "本地模型", Provider: "local",
			InputCostPer1M: 0, OutputCostPer1M: 0, Currency: "USD",
		},
	}
}

// CreateBudget 创建预算
func (m *Manager) CreateBudget(budget *Budget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if budget.ID == "" {
		budget.ID = fmt.Sprintf("budget-%d", time.Now().UnixNano())
	}
	if budget.Period == "" {
		budget.Period = m.config.DefaultPeriod
	}
	if budget.WarnThreshold == 0 {
		budget.WarnThreshold = m.config.WarnThreshold
	}
	budget.Enabled = true
	budget.CreatedAt = time.Now()
	budget.UpdatedAt = time.Now()

	m.budgets[budget.ID] = budget
	return nil
}

// RecordUsage 记录用量
func (m *Manager) RecordUsage(record *UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.ID == "" {
		record.ID = fmt.Sprintf("usage-%d", time.Now().UnixNano())
	}
	record.Timestamp = time.Now()

	// 计算成本
	if tier, ok := m.costTiers[record.ModelID]; ok {
		inputCost := float64(record.PromptTokens) / 1_000_000 * tier.InputCostPer1M
		outputCost := float64(record.CompletionTokens) / 1_000_000 * tier.OutputCostPer1M
		record.Cost = inputCost + outputCost
	}

	m.records = append(m.records, record)

	// 检查预算
	if budget, ok := m.budgets[record.BudgetID]; ok && budget.Enabled {
		summary := m.calculateSummary(budget)
		m.checkThresholds(budget, summary)
	}

	return nil
}

// GetBudgetStatus 获取预算状态
func (m *Manager) GetBudgetStatus(budgetID string) (*UsageSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[budgetID]
	if !ok {
		return nil, fmt.Errorf("budget %s not found", budgetID)
	}

	summary := m.calculateSummary(budget)
	return summary, nil
}

// GetCostAnalysis 获取成本分析
func (m *Manager) GetCostAnalysis(period string) *CostAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()

	analysis := &CostAnalysis{
		Period:        period,
		CostByModel:   make(map[string]float64),
		CostByUser:    make(map[string]float64),
		CostByService: make(map[string]float64),
	}

	for _, record := range m.records {
		analysis.TotalCost += record.Cost
		analysis.CostByModel[record.ModelID] += record.Cost
		analysis.CostByUser[record.UserID] += record.Cost
		analysis.CostByService[record.ServiceName] += record.Cost
	}

	// 生成建议
	analysis.Recommendations = m.generateRecommendations(analysis)

	return analysis
}

// GetModelComparison 获取模型成本对比
func (m *Manager) GetModelComparison(tokenCount int) []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]map[string]interface{}, 0)
	for _, tier := range m.costTiers {
		inputCost := float64(tokenCount) / 1_000_000 * tier.InputCostPer1M
		outputCost := float64(tokenCount) / 1_000_000 * tier.OutputCostPer1M
		results = append(results, map[string]interface{}{
			"modelId":    tier.ModelID,
			"modelName":  tier.ModelName,
			"provider":   tier.Provider,
			"inputCost":  inputCost,
			"outputCost": outputCost,
			"totalCost":  inputCost + outputCost,
		})
	}
	return results
}

// GetAlerts 获取告警列表
func (m *Manager) GetAlerts(dismissed bool) []*BudgetAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*BudgetAlert, 0)
	for _, alert := range m.alerts {
		if dismissed || !alert.Dismissed {
			result = append(result, alert)
		}
	}
	return result
}

// DismissAlert 关闭告警
func (m *Manager) DismissAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == alertID {
			alert.Dismissed = true
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

// ListBudgets 列出所有预算
func (m *Manager) ListBudgets() []*Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Budget, 0, len(m.budgets))
	for _, b := range m.budgets {
		result = append(result, b)
	}
	return result
}

func (m *Manager) calculateSummary(budget *Budget) *UsageSummary {
	periodStart, periodEnd := m.getPeriodRange(budget.Period)
	summary := &UsageSummary{
		BudgetID:    budget.ID,
		Period:      budget.Period,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		TopModels:   make([]ModelUsage, 0),
	}

	modelUsage := make(map[string]*ModelUsage)
	for _, record := range m.records {
		if record.BudgetID == budget.ID && record.Timestamp.After(periodStart) && record.Timestamp.Before(periodEnd) {
			summary.TotalTokens += int64(record.TotalTokens)
			summary.TotalCost += record.Cost
			summary.RequestCount++

			mu, ok := modelUsage[record.ModelID]
			if !ok {
				mu = &ModelUsage{ModelID: record.ModelID, ModelName: record.ModelID}
				modelUsage[record.ModelID] = mu
			}
			mu.TotalTokens += int64(record.TotalTokens)
			mu.TotalCost += record.Cost
			mu.RequestCount++
		}
	}

	if summary.RequestCount > 0 {
		summary.AvgTokensPerReq = float64(summary.TotalTokens) / float64(summary.RequestCount)
	}

	for _, mu := range modelUsage {
		summary.TopModels = append(summary.TopModels, *mu)
	}

	// 计算使用百分比
	if budget.MaxTokens > 0 {
		summary.UsagePercent = float64(summary.TotalTokens) / float64(budget.MaxTokens) * 100
	}
	if budget.MaxCost > 0 {
		costPercent := summary.TotalCost / budget.MaxCost * 100
		if costPercent > summary.UsagePercent {
			summary.UsagePercent = costPercent
		}
	}

	// 预测用量
	hoursInPeriod := periodEnd.Sub(periodStart).Hours()
	hoursElapsed := time.Since(periodStart).Hours()
	if hoursElapsed > 0 && hoursInPeriod > 0 {
		ratio := hoursInPeriod / hoursElapsed
		summary.ProjectedTokens = int64(float64(summary.TotalTokens) * ratio)
		summary.ProjectedCost = summary.TotalCost * ratio
	}

	// 状态判断
	summary.Status = m.getStatus(summary.UsagePercent)

	return summary
}

func (m *Manager) getStatus(percent float64) BudgetStatus {
	switch {
	case percent >= 100:
		return StatusExceeded
	case percent >= 95:
		return StatusCritical
	case percent >= 80:
		return StatusWarning
	default:
		return StatusNormal
	}
}

func (m *Manager) getPeriodRange(period BudgetPeriod) (time.Time, time.Time) {
	now := time.Now()
	switch period {
	case PeriodDaily:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return start, start.AddDate(0, 0, 1)
	case PeriodWeekly:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		return start, start.AddDate(0, 0, 7)
	case PeriodMonthly:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return start, start.AddDate(0, 1, 0)
	default:
		return now.Add(-24 * time.Hour), now
	}
}

func (m *Manager) checkThresholds(budget *Budget, summary *UsageSummary) {
	percent := summary.UsagePercent / 100

	if percent >= 1.0 {
		m.addAlert(budget, "exceeded", "预算已超限", percent, summary.UsagePercent)
	} else if percent >= budget.WarnThreshold {
		level := "warning"
		if percent >= m.config.CriticalThreshold {
			level = "critical"
		}
		m.addAlert(budget, level, fmt.Sprintf("预算使用已达 %.0f%%", summary.UsagePercent), percent, summary.UsagePercent)
	}
}

func (m *Manager) addAlert(budget *Budget, level, message string, threshold, actual float64) {
	alert := &BudgetAlert{
		ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		BudgetID:  budget.ID,
		Level:     level,
		Message:   fmt.Sprintf("[%s] %s - %s", budget.Name, message, budget.OwnerID),
		Threshold: threshold,
		Actual:    actual,
		Timestamp: time.Now(),
	}
	m.alerts = append(m.alerts, alert)
}

func (m *Manager) generateRecommendations(analysis *CostAnalysis) []string {
	recommendations := make([]string, 0)

	// 检查是否有本地模型可用
	if localCost, ok := analysis.CostByModel["local"]; ok && localCost == 0 {
		// 已经在用本地模型
	} else {
		totalCost := analysis.TotalCost
		if totalCost > 10 {
			recommendations = append(recommendations, "考虑将低优先级任务迁移到本地模型，可节省约 60% 成本")
		}
	}

	// 检查是否有昂贵模型用于简单任务
	if gpt4Cost, ok := analysis.CostByModel["gpt-4o"]; ok {
		if gpt4Cost > analysis.TotalCost*0.5 {
			recommendations = append(recommendations, "GPT-4o 占总成本 50%+，考虑将简单任务降级到 GPT-4o Mini")
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "当前 AI 使用成本合理，暂无优化建议")
	}

	return recommendations
}
