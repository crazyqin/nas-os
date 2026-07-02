package budgetalert

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// BudgetAlertManager 预算告警管理器
// 对标群晖 Storage Analyzer 和 TrueNAS 报表功能
// 提供存储成本预测、预算管理和超支告警.
type BudgetAlertManager struct {
	mu       sync.RWMutex
	config   *BudgetConfig
	budgets  map[string]*Budget
	alerts   []*Alert
	notifyCh chan *Alert
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// BudgetConfig 预算配置.
type BudgetConfig struct {
	Enabled         bool          `json:"enabled"`
	CheckInterval   time.Duration `json:"check_interval"`
	DefaultCurrency string        `json:"default_currency"`
	NotifyChannels  []string      `json:"notify_channels"`
}

// Budget 预算.
type Budget struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Category   BudgetCategory `json:"category"`
	LimitBytes int64          `json:"limit_bytes"` // 预算上限
	UsedBytes  int64          `json:"used_bytes"`  // 已使用
	CostPerGB  float64        `json:"cost_per_gb"` // 每GB成本
	Currency   string         `json:"currency"`
	Period     BudgetPeriod   `json:"period"`
	AlertAt    int            `json:"alert_at"`    // 告警阈值百分比
	CriticalAt int            `json:"critical_at"` // 严重告警阈值
	Owner      string         `json:"owner"`       // 用户/组
	IsActive   bool           `json:"is_active"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// BudgetCategory 预算类别.
type BudgetCategory string

const (
	CategoryTotal  BudgetCategory = "total"
	CategoryUser   BudgetCategory = "user"
	CategoryShare  BudgetCategory = "share"
	CategoryApp    BudgetCategory = "app"
	CategoryBackup BudgetCategory = "backup"
	CategoryCloud  BudgetCategory = "cloud"
)

// BudgetPeriod 预算周期.
type BudgetPeriod string

const (
	PeriodDaily   BudgetPeriod = "daily"
	PeriodWeekly  BudgetPeriod = "weekly"
	PeriodMonthly BudgetPeriod = "monthly"
	PeriodYearly  BudgetPeriod = "yearly"
)

// Alert 告警.
type Alert struct {
	ID        string     `json:"id"`
	BudgetID  string     `json:"budget_id"`
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	Usage     int        `json:"usage_percent"`
	Limit     int64      `json:"limit_bytes"`
	Used      int64      `json:"used_bytes"`
	Cost      float64    `json:"estimated_cost"`
	CreatedAt time.Time  `json:"created_at"`
	Acked     bool       `json:"acked"`
}

// AlertLevel 告警级别.
type AlertLevel string

const (
	LevelWarning  AlertLevel = "warning"
	LevelCritical AlertLevel = "critical"
	LevelExceeded AlertLevel = "exceeded"
)

// UsageReport 使用报告.
type UsageReport struct {
	BudgetID     string         `json:"budget_id"`
	BudgetName   string         `json:"budget_name"`
	Category     BudgetCategory `json:"category"`
	LimitBytes   int64          `json:"limit_bytes"`
	UsedBytes    int64          `json:"used_bytes"`
	UsagePercent float64        `json:"usage_percent"`
	EstCost      float64        `json:"estimated_cost"`
	Currency     string         `json:"currency"`
	ProjectDays  int            `json:"projected_days"` // 预计几天用完
	GrowthRate   float64        `json:"growth_rate_gb_per_day"`
	Trend        TrendDirection `json:"trend"`
}

// TrendDirection 趋势方向.
type TrendDirection string

const (
	TrendUp     TrendDirection = "up"
	TrendDown   TrendDirection = "down"
	TrendStable TrendDirection = "stable"
)

// CostSummary 成本汇总.
type CostSummary struct {
	TotalBudget float64                    `json:"total_budget"`
	TotalUsed   float64                    `json:"total_used"`
	TotalEst    float64                    `json:"total_estimated_monthly"`
	ByCategory  map[BudgetCategory]float64 `json:"by_category"`
	Currency    string                     `json:"currency"`
	GeneratedAt time.Time                  `json:"generated_at"`
}

// NewBudgetAlertManager 创建预算告警管理器.
func NewBudgetAlertManager(cfg *BudgetConfig) *BudgetAlertManager {
	if cfg == nil {
		cfg = &BudgetConfig{
			Enabled:         true,
			CheckInterval:   1 * time.Hour,
			DefaultCurrency: "CNY",
			NotifyChannels:  []string{"system"},
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &BudgetAlertManager{
		config:   cfg,
		budgets:  make(map[string]*Budget),
		alerts:   make([]*Alert, 0),
		notifyCh: make(chan *Alert, 100),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动管理器.
func (m *BudgetAlertManager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	m.wg.Add(2)
	go m.checkLoop()
	go m.notifyLoop()
	return nil
}

// Stop 停止管理器.
func (m *BudgetAlertManager) Stop() error {
	m.cancel()
	close(m.notifyCh)
	m.wg.Wait()
	return nil
}

func (m *BudgetAlertManager) checkLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllBudgets()
		}
	}
}

func (m *BudgetAlertManager) notifyLoop() {
	defer m.wg.Done()
	for alert := range m.notifyCh {
		_ = alert // 集成通知系统
	}
}

func (m *BudgetAlertManager) checkAllBudgets() {
	m.mu.RLock()
	budgets := make([]*Budget, 0)
	for _, b := range m.budgets {
		if b.IsActive {
			budgets = append(budgets, b)
		}
	}
	m.mu.RUnlock()

	for _, budget := range budgets {
		m.checkBudget(budget)
	}
}

func (m *BudgetAlertManager) checkBudget(budget *Budget) {
	usagePercent := 0
	if budget.LimitBytes > 0 {
		usagePercent = int(budget.UsedBytes * 100 / budget.LimitBytes)
	}

	var level AlertLevel
	var triggered bool

	if usagePercent >= 100 {
		level = LevelExceeded
		triggered = true
	} else if budget.CriticalAt > 0 && usagePercent >= budget.CriticalAt {
		level = LevelCritical
		triggered = true
	} else if budget.AlertAt > 0 && usagePercent >= budget.AlertAt {
		level = LevelWarning
		triggered = true
	}

	if triggered {
		alert := &Alert{
			ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			BudgetID:  budget.ID,
			Level:     level,
			Message:   fmt.Sprintf("预算 %s 使用率 %d%%", budget.Name, usagePercent),
			Usage:     usagePercent,
			Limit:     budget.LimitBytes,
			Used:      budget.UsedBytes,
			Cost:      float64(budget.UsedBytes) / (1024 * 1024 * 1024) * budget.CostPerGB,
			CreatedAt: time.Now(),
		}

		m.mu.Lock()
		m.alerts = append(m.alerts, alert)
		m.mu.Unlock()

		select {
		case m.notifyCh <- alert:
		default:
		}
	}
}

// CreateBudget 创建预算.
func (m *BudgetAlertManager) CreateBudget(name string, category BudgetCategory, limitBytes int64, opts *BudgetOptions) (*Budget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if opts == nil {
		opts = &BudgetOptions{}
	}

	budget := &Budget{
		ID:         fmt.Sprintf("budget-%d", time.Now().UnixNano()),
		Name:       name,
		Category:   category,
		LimitBytes: limitBytes,
		CostPerGB:  opts.CostPerGB,
		Currency:   opts.Currency,
		Period:     opts.Period,
		AlertAt:    opts.AlertAt,
		CriticalAt: opts.CriticalAt,
		Owner:      opts.Owner,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if budget.Currency == "" {
		budget.Currency = m.config.DefaultCurrency
	}
	if budget.Period == "" {
		budget.Period = PeriodMonthly
	}
	if budget.AlertAt == 0 {
		budget.AlertAt = 80
	}
	if budget.CriticalAt == 0 {
		budget.CriticalAt = 95
	}

	m.budgets[budget.ID] = budget
	return budget, nil
}

// BudgetOptions 预算选项.
type BudgetOptions struct {
	CostPerGB  float64
	Currency   string
	Period     BudgetPeriod
	AlertAt    int
	CriticalAt int
	Owner      string
}

// UpdateUsage 更新使用量.
func (m *BudgetAlertManager) UpdateUsage(budgetID string, usedBytes int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	budget, exists := m.budgets[budgetID]
	if !exists {
		return fmt.Errorf("预算 %s 不存在", budgetID)
	}
	budget.UsedBytes = usedBytes
	budget.UpdatedAt = time.Now()
	return nil
}

// GetReport 获取使用报告.
func (m *BudgetAlertManager) GetReport() []*UsageReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	reports := make([]*UsageReport, 0, len(m.budgets))
	for _, b := range m.budgets {
		if !b.IsActive {
			continue
		}
		usagePercent := float64(0)
		if b.LimitBytes > 0 {
			usagePercent = float64(b.UsedBytes) * 100 / float64(b.LimitBytes)
		}
		cost := float64(b.UsedBytes) / (1024 * 1024 * 1024) * b.CostPerGB
		reports = append(reports, &UsageReport{
			BudgetID:     b.ID,
			BudgetName:   b.Name,
			Category:     b.Category,
			LimitBytes:   b.LimitBytes,
			UsedBytes:    b.UsedBytes,
			UsagePercent: usagePercent,
			EstCost:      cost,
			Currency:     b.Currency,
		})
	}
	return reports
}

// GetCostSummary 获取成本汇总.
func (m *BudgetAlertManager) GetCostSummary() *CostSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCost := float64(0)
	byCategory := make(map[BudgetCategory]float64)
	for _, b := range m.budgets {
		if !b.IsActive {
			continue
		}
		cost := float64(b.UsedBytes) / (1024 * 1024 * 1024) * b.CostPerGB
		totalCost += cost
		byCategory[b.Category] += cost
	}

	return &CostSummary{
		TotalUsed:   totalCost,
		TotalEst:    totalCost,
		ByCategory:  byCategory,
		Currency:    m.config.DefaultCurrency,
		GeneratedAt: time.Now(),
	}
}

// GetAlerts 获取告警列表.
func (m *BudgetAlertManager) GetAlerts(unackedOnly bool) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Alert, 0)
	for _, a := range m.alerts {
		if !unackedOnly || !a.Acked {
			result = append(result, a)
		}
	}
	return result
}

// AcknowledgeAlert 确认告警.
func (m *BudgetAlertManager) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == alertID {
			a.Acked = true
			return nil
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// DeleteBudget 删除预算.
func (m *BudgetAlertManager) DeleteBudget(budgetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.budgets[budgetID]; !exists {
		return fmt.Errorf("预算 %s 不存在", budgetID)
	}
	delete(m.budgets, budgetID)
	return nil
}

// ExportReport 导出报告为JSON.
func (m *BudgetAlertManager) ExportReport() ([]byte, error) {
	report := m.GetReport()
	return json.MarshalIndent(report, "", "  ")
}
