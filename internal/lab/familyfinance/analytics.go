// Package familyfinance 提供家庭财务中心功能
// analytics.go - 财务分析，支持收支趋势、分类统计、现金流预测
package familyfinance

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AnalyticsEngine 分析引擎.
type AnalyticsEngine struct {
	mu     sync.RWMutex
	logger *zap.Logger
	engine *FinanceEngine
}

// NewAnalyticsEngine 创建分析引擎.
func NewAnalyticsEngine(logger *zap.Logger, engine *FinanceEngine) *AnalyticsEngine {
	return &AnalyticsEngine{
		logger: logger,
		engine: engine,
	}
}

// GetFinancialSummary 获取财务摘要.
func (ae *AnalyticsEngine) GetFinancialSummary(startDate, endDate time.Time) *FinancialSummary {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	summary := &FinancialSummary{}

	// 获取所有账户余额
	accounts := ae.engine.ListAccounts()
	for _, acc := range accounts {
		summary.TotalBalance += acc.Balance
	}

	// 查询时间范围内的交易
	query := TransactionQuery{
		StartDate: &startDate,
		EndDate:   &endDate,
		Page:      1,
		PageSize:  10000,
	}
	transactions := ae.engine.QueryTransactions(query)

	// 按类型统计
	categoryMap := make(map[string]*CategorySummary)
	accountMap := make(map[string]*AccountSummary)

	for _, tx := range transactions {
		switch tx.Type {
		case TransactionTypeIncome:
			summary.TotalIncome += tx.Amount
		case TransactionTypeExpense:
			summary.TotalExpense += tx.Amount
		}

		// 分类统计
		if cat, ok := categoryMap[tx.CategoryID]; ok {
			cat.Amount += tx.Amount
			cat.Count++
		} else {
			categoryMap[tx.CategoryID] = &CategorySummary{
				CategoryID:   tx.CategoryID,
				CategoryName: tx.CategoryName,
				Amount:       tx.Amount,
				Count:        1,
			}
		}

		// 账户统计
		if acc, ok := accountMap[tx.AccountID]; ok {
			switch tx.Type {
			case TransactionTypeIncome:
				acc.Income += tx.Amount
			case TransactionTypeExpense:
				acc.Expense += tx.Amount
			}
		} else {
			account, _ := ae.engine.GetAccount(tx.AccountID)
			accName := tx.AccountID
			if account != nil {
				accName = account.Name
			}
			accountMap[tx.AccountID] = &AccountSummary{
				AccountID:   tx.AccountID,
				AccountName: accName,
				Income:      0,
				Expense:     0,
			}
			switch tx.Type {
			case TransactionTypeIncome:
				accountMap[tx.AccountID].Income = tx.Amount
			case TransactionTypeExpense:
				accountMap[tx.AccountID].Expense = tx.Amount
			}
		}
	}

	summary.NetIncome = summary.TotalIncome - summary.TotalExpense
	summary.TotalAssets = summary.TotalBalance
	summary.NetWorth = summary.TotalAssets - summary.TotalLiability

	// 转换为切片
	for _, cat := range categoryMap {
		if summary.TotalExpense > 0 {
			cat.Percent = (cat.Amount / summary.TotalExpense) * 100
		}
		summary.ByCategory = append(summary.ByCategory, *cat)
	}
	sort.Slice(summary.ByCategory, func(i, j int) bool {
		return summary.ByCategory[i].Amount > summary.ByCategory[j].Amount
	})

	for _, acc := range accountMap {
		acc.Balance, _ = ae.getAccountBalance(acc.AccountID)
		summary.ByAccount = append(summary.ByAccount, *acc)
	}

	return summary
}

// getAccountBalance 获取账户余额.
func (ae *AnalyticsEngine) getAccountBalance(accountID string) (float64, error) {
	account, err := ae.engine.GetAccount(accountID)
	if err != nil {
		return 0, err
	}
	return account.Balance, nil
}

// GetIncomeExpenseTrend 获取收支趋势.
func (ae *AnalyticsEngine) GetIncomeExpenseTrend(startDate, endDate time.Time, interval string) []TrendPoint {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	query := TransactionQuery{
		StartDate: &startDate,
		EndDate:   &endDate,
		Page:      1,
		PageSize:  10000,
	}
	transactions := ae.engine.QueryTransactions(query)

	// 按时间分组
	trendMap := make(map[string]*TrendPoint)

	for _, tx := range transactions {
		var key string
		switch interval {
		case "daily":
			key = tx.Date.Format("2006-01-02")
		case "weekly":
			year, week := tx.Date.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", year, week)
		case "monthly":
			key = tx.Date.Format("2006-01")
		default:
			key = tx.Date.Format("2006-01-02")
		}

		if point, ok := trendMap[key]; ok {
			switch tx.Type {
			case TransactionTypeIncome:
				point.Income += tx.Amount
			case TransactionTypeExpense:
				point.Expense += tx.Amount
			}
		} else {
			trendMap[key] = &TrendPoint{
				Date:    tx.Date,
				Income:  0,
				Expense: 0,
			}
			switch tx.Type {
			case TransactionTypeIncome:
				trendMap[key].Income = tx.Amount
			case TransactionTypeExpense:
				trendMap[key].Expense = tx.Amount
			}
		}
	}

	// 转换为切片并排序
	var trend []TrendPoint
	for _, point := range trendMap {
		point.Net = point.Income - point.Expense
		trend = append(trend, *point)
	}
	sort.Slice(trend, func(i, j int) bool {
		return trend[i].Date.Before(trend[j].Date)
	})

	return trend
}

// PredictCashFlow 现金流预测.
func (ae *AnalyticsEngine) PredictCashFlow(months int) *CashFlowForecast {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	// 获取过去6个月的数据作为预测基础
	endDate := time.Now()
	startDate := endDate.AddDate(0, -6, 0)
	query := TransactionQuery{
		StartDate: &startDate,
		EndDate:   &endDate,
		Page:      1,
		PageSize:  10000,
	}
	transactions := ae.engine.QueryTransactions(query)

	// 计算月均收支
	monthlyIncome := make(map[string]float64)
	monthlyExpense := make(map[string]float64)

	for _, tx := range transactions {
		month := tx.Date.Format("2006-01")
		switch tx.Type {
		case TransactionTypeIncome:
			monthlyIncome[month] += tx.Amount
		case TransactionTypeExpense:
			monthlyExpense[month] += tx.Amount
		}
	}

	avgIncome := 0.0
	avgExpense := 0.0
	count := float64(len(monthlyIncome))
	if count > 0 {
		for _, v := range monthlyIncome {
			avgIncome += v
		}
		avgIncome /= count
		for _, v := range monthlyExpense {
			avgExpense += v
		}
		avgExpense /= count
	}

	// 生成预测
	currentBalance := ae.engine.GetTotalBalance()
	predictions := make([]MonthPrediction, months)

	for i := 0; i < months; i++ {
		month := endDate.AddDate(0, i+1, 0)
		predictions[i] = MonthPrediction{
			Month:        month.Format("2006-01"),
			PredictedIn:  avgIncome,
			PredictedOut: avgExpense,
			NetFlow:      avgIncome - avgExpense,
		}
		currentBalance += predictions[i].NetFlow
		predictions[i].Balance = currentBalance
	}

	totalPredicted := 0.0
	for _, p := range predictions {
		totalPredicted += p.NetFlow
	}

	// 计算置信度（基于历史数据量）
	confidence := math.Min(count/6.0, 1.0)

	return &CashFlowForecast{
		GeneratedAt:    time.Now(),
		Months:         months,
		Predictions:    predictions,
		TotalPredicted: totalPredicted,
		Confidence:     confidence,
	}
}

// GetSpendingByCategory 获取分类消费统计.
func (ae *AnalyticsEngine) GetSpendingByCategory(startDate, endDate time.Time) []CategorySummary {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	query := TransactionQuery{
		StartDate: &startDate,
		EndDate:   &endDate,
		Type:      TransactionTypeExpense,
		Page:      1,
		PageSize:  10000,
	}
	transactions := ae.engine.QueryTransactions(query)

	categoryMap := make(map[string]*CategorySummary)
	total := 0.0

	for _, tx := range transactions {
		total += tx.Amount
		if cat, ok := categoryMap[tx.CategoryID]; ok {
			cat.Amount += tx.Amount
			cat.Count++
		} else {
			categoryMap[tx.CategoryID] = &CategorySummary{
				CategoryID:   tx.CategoryID,
				CategoryName: tx.CategoryName,
				Amount:       tx.Amount,
				Count:        1,
			}
		}
	}

	var result []CategorySummary
	for _, cat := range categoryMap {
		if total > 0 {
			cat.Percent = (cat.Amount / total) * 100
		}
		result = append(result, *cat)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Amount > result[j].Amount
	})

	return result
}

// fmt.Sprintf needs fmt import.
func init() {
	// ensure fmt is used
	_ = fmt.Sprintf
}
