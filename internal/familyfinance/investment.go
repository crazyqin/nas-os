// Package familyfinance 提供家庭财务中心功能
// investment.go - 投资追踪，支持基金/股票/加密货币持仓、收益计算
package familyfinance

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// InvestmentManager 投资管理器
type InvestmentManager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	investments map[string]*Investment
}

// NewInvestmentManager 创建投资管理器
func NewInvestmentManager(logger *zap.Logger) *InvestmentManager {
	return &InvestmentManager{
		logger:      logger,
		investments: make(map[string]*Investment),
	}
}

// AddInvestment 添加投资记录
func (im *InvestmentManager) AddInvestment(investment *Investment) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if investment.ID == "" {
		investment.ID = uuid.New().String()
	}

	if investment.Shares <= 0 || investment.CostBasis <= 0 {
		return ErrInvalidAmount
	}

	// 计算成本和收益
	investment.TotalCost = investment.Shares * investment.CostBasis
	investment.CurrentValue = investment.Shares * investment.CurrentPrice
	investment.GainLoss = investment.CurrentValue - investment.TotalCost
	if investment.TotalCost > 0 {
		investment.GainLossPercent = (investment.GainLoss / investment.TotalCost) * 100
	}

	investment.CreatedAt = time.Now()
	investment.UpdatedAt = time.Now()

	im.investments[investment.ID] = investment
	im.logger.Info("投资记录已添加",
		zap.String("id", investment.ID),
		zap.String("name", investment.Name),
		zap.String("type", string(investment.Type)))
	return nil
}

// UpdateInvestment 更新投资记录
func (im *InvestmentManager) UpdateInvestment(investment *Investment) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	existing, exists := im.investments[investment.ID]
	if !exists {
		return ErrInvestmentNotFound
	}

	investment.CreatedAt = existing.CreatedAt
	investment.UpdatedAt = time.Now()

	// 重新计算收益
	investment.TotalCost = investment.Shares * investment.CostBasis
	investment.CurrentValue = investment.Shares * investment.CurrentPrice
	investment.GainLoss = investment.CurrentValue - investment.TotalCost
	if investment.TotalCost > 0 {
		investment.GainLossPercent = (investment.GainLoss / investment.TotalCost) * 100
	}

	im.investments[investment.ID] = investment
	return nil
}

// DeleteInvestment 删除投资记录
func (im *InvestmentManager) DeleteInvestment(investmentID string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if _, exists := im.investments[investmentID]; !exists {
		return ErrInvestmentNotFound
	}

	delete(im.investments, investmentID)
	im.logger.Info("投资记录已删除", zap.String("id", investmentID))
	return nil
}

// GetInvestment 获取投资记录
func (im *InvestmentManager) GetInvestment(investmentID string) (*Investment, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	investment, exists := im.investments[investmentID]
	if !exists {
		return nil, ErrInvestmentNotFound
	}
	return investment, nil
}

// ListInvestments 列出所有投资
func (im *InvestmentManager) ListInvestments() []*Investment {
	im.mu.RLock()
	defer im.mu.RUnlock()

	investments := make([]*Investment, 0, len(im.investments))
	for _, inv := range im.investments {
		investments = append(investments, inv)
	}
	return investments
}

// UpdatePrice 更新投资价格
func (im *InvestmentManager) UpdatePrice(investmentID string, newPrice float64) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	investment, exists := im.investments[investmentID]
	if !exists {
		return ErrInvestmentNotFound
	}

	investment.CurrentPrice = newPrice
	investment.CurrentValue = investment.Shares * newPrice
	investment.GainLoss = investment.CurrentValue - investment.TotalCost
	if investment.TotalCost > 0 {
		investment.GainLossPercent = (investment.GainLoss / investment.TotalCost) * 100
	}
	investment.UpdatedAt = time.Now()

	return nil
}

// GetPortfolioSummary 获取投资组合摘要
func (im *InvestmentManager) GetPortfolioSummary() map[string]interface{} {
	im.mu.RLock()
	defer im.mu.RUnlock()

	totalCost := 0.0
	totalValue := 0.0
	totalGainLoss := 0.0
	byType := make(map[string]float64)

	for _, inv := range im.investments {
		totalCost += inv.TotalCost
		totalValue += inv.CurrentValue
		totalGainLoss += inv.GainLoss
		byType[string(inv.Type)] += inv.CurrentValue
	}

	totalGainLossPercent := 0.0
	if totalCost > 0 {
		totalGainLossPercent = (totalGainLoss / totalCost) * 100
	}

	return map[string]interface{}{
		"total_cost":          totalCost,
		"total_value":         totalValue,
		"total_gain_loss":     totalGainLoss,
		"total_gain_loss_pct": totalGainLossPercent,
		"by_type":             byType,
		"count":               len(im.investments),
	}
}

// GetInvestmentRanking 获取投资收益排行
func (im *InvestmentManager) GetInvestmentRanking() []*Investment {
	im.mu.RLock()
	defer im.mu.RUnlock()

	investments := make([]*Investment, 0, len(im.investments))
	for _, inv := range im.investments {
		investments = append(investments, inv)
	}

	// 按收益率排序
	sort.Slice(investments, func(i, j int) bool {
		return investments[i].GainLossPercent > investments[j].GainLossPercent
	})

	return investments
}

// GetInvestmentsByType 按类型获取投资
func (im *InvestmentManager) GetInvestmentsByType(investType InvestmentType) []*Investment {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var result []*Investment
	for _, inv := range im.investments {
		if inv.Type == investType {
			result = append(result, inv)
		}
	}
	return result
}

// CalculateAnnualizedReturn 计算年化收益率
func (im *InvestmentManager) CalculateAnnualizedReturn(investmentID string) (float64, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	investment, exists := im.investments[investmentID]
	if !exists {
		return 0, ErrInvestmentNotFound
	}

	// 计算持有天数
	days := time.Since(investment.BuyDate).Hours() / 24
	if days <= 0 {
		return 0, nil
	}

	// 年化收益率 = (当前价值/成本)^(365/持有天数) - 1
	totalReturn := investment.CurrentValue / investment.TotalCost
	annualized := (pow(totalReturn, 365.0/days) - 1) * 100

	return annualized, nil
}

// pow 计算幂
func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}
