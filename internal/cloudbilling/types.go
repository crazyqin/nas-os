// Package cloudbilling 提供多云存储成本追踪和优化功能
// 包括成本分析、预算告警、使用趋势、优化建议
package cloudbilling

import (
	"errors"
	"sync"
	"time"
)

// ========== 云提供商 ==========

// Provider 云服务提供商
type Provider string

const (
	ProviderAWS     Provider = "aws"     // Amazon S3
	ProviderGCP     Provider = "gcp"     // Google Cloud Storage
	ProviderAzure   Provider = "azure"   // Azure Blob Storage
	ProviderAlibaba Provider = "alibaba" // 阿里云 OSS
	ProviderTencent Provider = "tencent" // 腾讯云 COS
	ProviderMinIO   Provider = "minio"   // MinIO (自托管)
	ProviderCustom  Provider = "custom"  // 自定义 S3 兼容
)

// ========== 存储类别 ==========

// StorageClass 存储类别
type StorageClass string

const (
	ClassStandard StorageClass = "standard"   // 标准存储
	ClassInfreq   StorageClass = "infrequent" // 低频访问
	ClassArchive  StorageClass = "archive"    // 归档存储
	ClassCold     StorageClass = "cold"       // 冷存储
	ClassDeep     StorageClass = "deep"       // 深度归档
)

// ========== 费用类型 ==========

// CostType 费用类型
type CostType string

const (
	CostStorage   CostType = "storage"   // 存储费用
	CostTransfer  CostType = "transfer"  // 流量费用
	CostRequest   CostType = "request"   // 请求费用
	CostRetrieval CostType = "retrieval" // 取回费用
	CostOperation CostType = "operation" // 操作费用
)

// ========== 账户配置 ==========

// CloudAccount 云账户配置
type CloudAccount struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  Provider  `json:"provider"`
	AccessKey string    `json:"access_key"`
	SecretKey string    `json:"secret_key,omitempty"`
	Region    string    `json:"region"`
	Endpoint  string    `json:"endpoint,omitempty"` // 自定义端点
	Bucket    string    `json:"bucket"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ========== 成本记录 ==========

// CostRecord 成本记录
type CostRecord struct {
	ID        string            `json:"id"`
	AccountID string            `json:"account_id"`
	Provider  Provider          `json:"provider"`
	CostType  CostType          `json:"cost_type"`
	Amount    float64           `json:"amount"`     // 费用金额（人民币）
	Currency  string            `json:"currency"`   // 货币单位
	SizeBytes int64             `json:"size_bytes"` // 相关存储量
	Usage     float64           `json:"usage"`      // 使用量
	Unit      string            `json:"unit"`       // 单位（GB, 次, etc）
	Period    string            `json:"period"`     // 账期（2026-06）
	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// ========== 预算 ==========

// Budget 预算配置
type Budget struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	AccountID  string    `json:"account_id,omitempty"` // 空表示全局
	MonthlyCap float64   `json:"monthly_cap"`          // 月度预算上限
	AlertAt    float64   `json:"alert_at"`             // 告警阈值（百分比 0-100）
	AlertSent  bool      `json:"alert_sent"`
	Period     string    `json:"period"`
	CreatedAt  time.Time `json:"created_at"`
}

// ========== 成本分析 ==========

// CostAnalysis 成本分析结果
type CostAnalysis struct {
	AccountID      string                   `json:"account_id"`
	Provider       Provider                 `json:"provider"`
	Period         string                   `json:"period"`
	TotalCost      float64                  `json:"total_cost"`
	ByType         map[CostType]float64     `json:"by_type"`
	ByClass        map[StorageClass]float64 `json:"by_class"`
	Trend          []CostTrend              `json:"trend"`
	Suggestions    []CostSuggestion         `json:"suggestions"`
	ComparedToPrev float64                  `json:"compared_to_prev"` // 同比变化百分比
}

// CostTrend 成本趋势
type CostTrend struct {
	Date  string  `json:"date"`
	Cost  float64 `json:"cost"`
	Usage int64   `json:"usage"`
}

// CostSuggestion 成本优化建议
type CostSuggestion struct {
	Type        string  `json:"type"`     // lifecycle, class_change, delete_unused
	Priority    string  `json:"priority"` // high, medium, low
	Description string  `json:"description"`
	EstSavings  float64 `json:"est_savings"` // 预计节省金额
	Confidence  float64 `json:"confidence"`
}

// ========== 成本追踪器 ==========

// CostTracker 成本追踪器
type CostTracker struct {
	mu       sync.RWMutex
	accounts map[string]*CloudAccount
	records  map[string][]*CostRecord
	budgets  map[string]*Budget
	alerts   []BudgetAlert
}

// BudgetAlert 预算告警
type BudgetAlert struct {
	BudgetID  string    `json:"budget_id"`
	AccountID string    `json:"account_id"`
	Period    string    `json:"period"`
	Budget    float64   `json:"budget"`
	Actual    float64   `json:"actual"`
	Percent   float64   `json:"percent"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// TrackerOption 追踪器配置选项
type TrackerOption func(*CostTracker)

// NewCostTracker 创建成本追踪器
func NewCostTracker(opts ...TrackerOption) *CostTracker {
	t := &CostTracker{
		accounts: make(map[string]*CloudAccount),
		records:  make(map[string][]*CostRecord),
		budgets:  make(map[string]*Budget),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// ========== 账户管理 ==========

// AddAccount 添加云账户
func (t *CostTracker) AddAccount(account *CloudAccount) error {
	if account.ID == "" {
		return errors.New("account ID cannot be empty")
	}
	if account.Name == "" {
		return errors.New("account name cannot be empty")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()
	t.accounts[account.ID] = account
	return nil
}

// RemoveAccount 移除云账户
func (t *CostTracker) RemoveAccount(accountID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.accounts[accountID]; !ok {
		return errors.New("account not found")
	}
	delete(t.accounts, accountID)
	return nil
}

// ListAccounts 列出所有账户
func (t *CostTracker) ListAccounts() []*CloudAccount {
	t.mu.RLock()
	defer t.mu.RUnlock()

	accounts := make([]*CloudAccount, 0, len(t.accounts))
	for _, a := range t.accounts {
		accounts = append(accounts, a)
	}
	return accounts
}

// ========== 成本记录 ==========

// RecordCost 记录费用
func (t *CostTracker) RecordCost(record *CostRecord) error {
	if record.AccountID == "" {
		return errors.New("account ID cannot be empty")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.accounts[record.AccountID]; !ok {
		return errors.New("account not found")
	}

	record.Timestamp = time.Now()
	t.records[record.AccountID] = append(t.records[record.AccountID], record)

	// 检查预算告警
	t.checkBudgets(record.AccountID, record.Period)

	return nil
}

// ========== 成本分析 ==========

// AnalyzeCost 分析成本
func (t *CostTracker) AnalyzeCost(accountID, period string) *CostAnalysis {
	t.mu.RLock()
	defer t.mu.RUnlock()

	analysis := &CostAnalysis{
		AccountID: accountID,
		Period:    period,
		ByType:    make(map[CostType]float64),
		ByClass:   make(map[StorageClass]float64),
		Trend:     make([]CostTrend, 0),
	}

	if account, ok := t.accounts[accountID]; ok {
		analysis.Provider = account.Provider
	}

	records := t.records[accountID]
	for _, r := range records {
		if r.Period == period {
			analysis.TotalCost += r.Amount
			analysis.ByType[r.CostType] += r.Amount
		}
	}

	analysis.Suggestions = t.generateCostSuggestions(analysis)
	return analysis
}

// GetTotalCost 获取总成本
func (t *CostTracker) GetTotalCost(period string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total float64
	for _, records := range t.records {
		for _, r := range records {
			if r.Period == period {
				total += r.Amount
			}
		}
	}
	return total
}

// ========== 预算管理 ==========

// SetBudget 设置预算
func (t *CostTracker) SetBudget(budget *Budget) error {
	if budget.ID == "" {
		return errors.New("budget ID cannot be empty")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	budget.CreatedAt = time.Now()
	t.budgets[budget.ID] = budget
	return nil
}

// GetBudgetStatus 获取预算状态
func (t *CostTracker) GetBudgetStatus(budgetID string) (float64, float64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	budget, ok := t.budgets[budgetID]
	if !ok {
		return 0, 0, errors.New("budget not found")
	}

	actual := t.getActualCost(budget.AccountID, budget.Period)
	return budget.MonthlyCap, actual, nil
}

// GetAlerts 获取预算告警
func (t *CostTracker) GetAlerts() []BudgetAlert {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.alerts
}

// ========== 内部方法 ==========

func (t *CostTracker) checkBudgets(accountID, period string) {
	for _, budget := range t.budgets {
		if budget.AccountID != "" && budget.AccountID != accountID {
			continue
		}
		if budget.Period != period {
			continue
		}

		actual := t.getActualCost(budget.AccountID, period)
		percent := (actual / budget.MonthlyCap) * 100

		if percent >= budget.AlertAt && !budget.AlertSent {
			alert := BudgetAlert{
				BudgetID:  budget.ID,
				AccountID: accountID,
				Period:    period,
				Budget:    budget.MonthlyCap,
				Actual:    actual,
				Percent:   percent,
				Message:   "预算使用已达告警阈值",
				CreatedAt: time.Now(),
			}
			t.alerts = append(t.alerts, alert)
			budget.AlertSent = true
		}
	}
}

func (t *CostTracker) getActualCost(accountID, period string) float64 {
	var total float64
	records := t.records[accountID]
	for _, r := range records {
		if r.Period == period {
			total += r.Amount
		}
	}
	return total
}

func (t *CostTracker) generateCostSuggestions(analysis *CostAnalysis) []CostSuggestion {
	var suggestions []CostSuggestion

	transferCost := analysis.ByType[CostTransfer]
	if transferCost > analysis.TotalCost*0.3 {
		suggestions = append(suggestions, CostSuggestion{
			Type:        "transfer_optimize",
			Priority:    "high",
			Description: "流量费用占比过高，建议启用CDN或压缩传输",
			EstSavings:  transferCost * 0.3,
			Confidence:  0.8,
		})
	}

	storageCost := analysis.ByType[CostStorage]
	if storageCost > analysis.TotalCost*0.6 {
		suggestions = append(suggestions, CostSuggestion{
			Type:        "class_change",
			Priority:    "medium",
			Description: "存储费用占比高，建议对不常访问数据使用低频/归档存储",
			EstSavings:  storageCost * 0.2,
			Confidence:  0.7,
		})
	}

	return suggestions
}
