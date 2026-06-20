package smartmulticloud

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// CloudProvider 云服务商.
type CloudProvider string

const (
	ProviderAWS     CloudProvider = "aws"
	ProviderAzure   CloudProvider = "azure"
	ProviderGCP     CloudProvider = "gcp"
	ProviderAliyun  CloudProvider = "aliyun"
	ProviderTencent CloudProvider = "tencent"
	ProviderHuawei  CloudProvider = "huawei"
)

// StorageClass 存储类型.
type StorageClass string

const (
	ClassHot    StorageClass = "hot"    // 热存储
	ClassWarm   StorageClass = "warm"   // 温存储
	ClassCold   StorageClass = "cold"   // 冷存储
	ClassArchive StorageClass = "archive" // 归档
)

// CloudAccount 云账号.
type CloudAccount struct {
	ID          string        `json:"id"`
	Provider    CloudProvider `json:"provider"`
	Name        string        `json:"name"`
	AccessKey   string        `json:"access_key"`
	SecretKey   string        `json:"secret_key"`
	Region      string        `json:"region"`
	Enabled     bool          `json:"enabled"`
	LastSync    time.Time     `json:"last_sync"`
	Metadata    map[string]string `json:"metadata"`
}

// StorageCost 存储成本.
type StorageCost struct {
	AccountID   string        `json:"account_id"`
	Provider    CloudProvider `json:"provider"`
	Bucket      string        `json:"bucket"`
	Class       StorageClass  `json:"class"`
	SizeBytes   int64         `json:"size_bytes"`
	MonthlyCost float64       `json:"monthly_cost"`
	Currency    string        `json:"currency"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// CostForecast 成本预测.
type CostForecast struct {
	AccountID    string    `json:"account_id"`
	CurrentCost  float64   `json:"current_cost"`
	ForecastCost float64   `json:"forecast_cost"`
	Trend        string    `json:"trend"` // increasing/decreasing/stable
	Confidence   float64   `json:"confidence"`
	Period       string    `json:"period"`
}

// OptimizationRecommendation 优化建议.
type OptimizationRecommendation struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"` // tier_change/delete/compress
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Savings     float64 `json:"savings"`
	Risk        string  `json:"risk"` // low/medium/high
	Description string  `json:"description"`
}

// Engine 多云成本优化引擎.
type Engine struct {
	accounts map[string]*CloudAccount
	costs    map[string][]*StorageCost
	logger   *zap.Logger
	mu       sync.RWMutex
}

// NewEngine 创建多云成本优化引擎.
func NewEngine(logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Engine{
		accounts: make(map[string]*CloudAccount),
		costs:    make(map[string][]*StorageCost),
		logger:   logger,
	}
}

// AddAccount 添加云账号.
func (e *Engine) AddAccount(account *CloudAccount) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if account.ID == "" {
		return ErrInvalidAccountID
	}
	account.LastSync = time.Now()
	if account.Metadata == nil {
		account.Metadata = make(map[string]string)
	}
	e.accounts[account.ID] = account
	e.logger.Info("云账号已添加",
		zap.String("id", account.ID),
		zap.String("provider", string(account.Provider)),
	)
	return nil
}

// GetAccount 获取云账号.
func (e *Engine) GetAccount(id string) (*CloudAccount, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	acc, ok := e.accounts[id]
	return acc, ok
}

// ListAccounts 列出所有账号.
func (e *Engine) ListAccounts() []*CloudAccount {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	accounts := make([]*CloudAccount, 0, len(e.accounts))
	for _, a := range e.accounts {
		accounts = append(accounts, a)
	}
	return accounts
}

// RecordCost 记录存储成本.
func (e *Engine) RecordCost(cost *StorageCost) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if _, ok := e.accounts[cost.AccountID]; !ok {
		return ErrAccountNotFound
	}
	cost.UpdatedAt = time.Now()
	e.costs[cost.AccountID] = append(e.costs[cost.AccountID], cost)
	return nil
}

// GetTotalCost 获取总成本.
func (e *Engine) GetTotalCost() map[CloudProvider]float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	totals := make(map[CloudProvider]float64)
	for _, costs := range e.costs {
		for _, c := range costs {
			totals[c.Provider] += c.MonthlyCost
		}
	}
	return totals
}

// GetCostByProvider 按服务商获取成本.
func (e *Engine) GetCostByProvider(provider CloudProvider) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	total := 0.0
	for _, costs := range e.costs {
		for _, c := range costs {
			if c.Provider == provider {
				total += c.MonthlyCost
			}
		}
	}
	return total
}

// ForecastCost 预测成本.
func (e *Engine) ForecastCost(accountID string, period string) (*CostForecast, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if _, ok := e.accounts[accountID]; !ok {
		return nil, ErrAccountNotFound
	}
	
	costs := e.costs[accountID]
	if len(costs) == 0 {
		return &CostForecast{
			AccountID: accountID,
			Period:    period,
			Trend:     "stable",
		}, nil
	}
	
	// 计算当前月成本
	currentCost := 0.0
	for _, c := range costs {
		currentCost += c.MonthlyCost
	}
	
	// 简单预测：基于历史趋势
	forecast := &CostForecast{
		AccountID:    accountID,
		CurrentCost:  currentCost,
		ForecastCost: currentCost * 1.05, // 假设5%增长
		Trend:        "increasing",
		Confidence:   0.75,
		Period:       period,
	}
	
	return forecast, nil
}

// GenerateRecommendations 生成优化建议.
func (e *Engine) GenerateRecommendations() []*OptimizationRecommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	var recommendations []*OptimizationRecommendation
	
	for _, costs := range e.costs {
		for _, c := range costs {
			// 热存储转温存储建议
			if c.Class == ClassHot && c.SizeBytes > 100*1024*1024*1024 { // >100GB
				savings := c.MonthlyCost * 0.3 // 节省30%
				rec := &OptimizationRecommendation{
					ID:          "rec-" + c.AccountID + "-tier",
					Type:        "tier_change",
					Source:      string(ClassHot),
					Target:      string(ClassWarm),
					Savings:     savings,
					Risk:        "low",
					Description: "将不常访问的数据从热存储迁移到温存储",
				}
				recommendations = append(recommendations, rec)
			}
			
			// 温存储转冷存储建议
			if c.Class == ClassWarm && c.SizeBytes > 500*1024*1024*1024 { // >500GB
				savings := c.MonthlyCost * 0.5
				rec := &OptimizationRecommendation{
					ID:          "rec-" + c.AccountID + "-cold",
					Type:        "tier_change",
					Source:      string(ClassWarm),
					Target:      string(ClassCold),
					Savings:     savings,
					Risk:        "medium",
					Description: "将历史数据从温存储迁移到冷存储",
				}
				recommendations = append(recommendations, rec)
			}
		}
	}
	
	return recommendations
}

// GetCostBreakdown 成本分 breakdown.
func (e *Engine) GetCostBreakdown() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	byProvider := make(map[string]float64)
	byClass := make(map[string]float64)
	totalCost := 0.0
	
	for _, costs := range e.costs {
		for _, c := range costs {
			byProvider[string(c.Provider)] += c.MonthlyCost
			byClass[string(c.Class)] += c.MonthlyCost
			totalCost += c.MonthlyCost
		}
	}
	
	return map[string]interface{}{
		"total_cost":  totalCost,
		"by_provider": byProvider,
		"by_class":    byClass,
		"account_count": len(e.accounts),
	}
}
