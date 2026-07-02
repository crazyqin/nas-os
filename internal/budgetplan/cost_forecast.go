package budgetplan

import (
	"math"
	"sort"
	"time"
)

// ========== 成本预测 (CostForecast) ==========

// ForecastCost 预测成本.
func (m *BudgetManager) ForecastCost(budgetID string, monthsAhead int) (*ForecastResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[budgetID]
	if !ok {
		return nil, ErrBudgetNotFound
	}

	if monthsAhead <= 0 {
		monthsAhead = 3
	}

	// 获取历史数据
	expenses := m.expenses[budgetID]
	if len(expenses) < 2 {
		return nil, ErrInsufficientData
	}

	// 按时间排序
	sortedExpenses := make([]*Expense, len(expenses))
	copy(sortedExpenses, expenses)
	sort.Slice(sortedExpenses, func(i, j int) bool {
		return sortedExpenses[i].OccurredAt.Before(sortedExpenses[j].OccurredAt)
	})

	// 计算月度支出
	monthlyData := m.aggregateMonthly(sortedExpenses)
	if len(monthlyData) < 2 {
		return nil, ErrInsufficientData
	}

	// 线性回归预测
	x := make([]float64, len(monthlyData))
	y := make([]float64, len(monthlyData))
	for i, d := range monthlyData {
		x[i] = float64(i)
		y[i] = d.Amount
	}

	slope, intercept, r2 := linearRegression(x, y)

	// 生成预测点
	currentMonth := len(monthlyData)
	forecastPoints := make([]ForecastPoint, monthsAhead)
	var totalPredicted float64

	for i := 0; i < monthsAhead; i++ {
		predicted := slope*float64(currentMonth+i) + intercept
		if predicted < 0 {
			predicted = 0
		}
		totalPredicted += predicted

		// 计算置信区间
		stdErr := calculateStdError(x, y, slope, intercept)
		margin := stdErr * 1.96 // 95% 置信区间

		forecastPoints[i] = ForecastPoint{
			Date:            time.Now().AddDate(0, i+1, 0),
			PredictedAmount: roundTo2(predicted),
			LowerBound:      roundTo2(math.Max(0, predicted-margin)),
			UpperBound:      roundTo2(predicted + margin),
		}
	}

	// 计算月增长率
	monthlyGrowthRate := 0.0
	if len(monthlyData) >= 2 {
		lastMonth := monthlyData[len(monthlyData)-1].Amount
		prevMonth := monthlyData[len(monthlyData)-2].Amount
		if prevMonth > 0 {
			monthlyGrowthRate = (lastMonth - prevMonth) / prevMonth * 100
		}
	}

	// 计算预计耗尽日期
	var projectedEndDate *time.Time
	if slope > 0 && budget.RemainingAmount > 0 {
		monthsUntilExhausted := budget.RemainingAmount / slope
		endDate := time.Now().AddDate(0, int(monthsUntilExhausted), 0)
		projectedEndDate = &endDate
	}

	// 判断趋势
	trend := "stable"
	if slope > budget.TotalAmount*0.01 {
		trend = "increasing"
	} else if slope < -budget.TotalAmount*0.01 {
		trend = "decreasing"
	}

	// 生成建议
	recommendations := m.generateForecastRecommendations(budget, slope, monthlyGrowthRate, projectedEndDate)

	return &ForecastResult{
		GeneratedAt:       time.Now(),
		BudgetID:          budgetID,
		BudgetName:        budget.Name,
		CurrentUsage:      budget.UsedAmount,
		PredictedUsage:    roundTo2(budget.UsedAmount + totalPredicted),
		Confidence:        r2,
		MonthlyGrowthRate: roundTo2(monthlyGrowthRate),
		ProjectedEndDate:  projectedEndDate,
		ForecastPoints:    forecastPoints,
		Recommendations:   recommendations,
		Trend:             trend,
	}, nil
}

// ForecastAllBudgets 预测所有活跃预算.
func (m *BudgetManager) ForecastAllBudgets(monthsAhead int) []*ForecastResult {
	m.mu.RLock()
	budgetIDs := make([]string, 0)
	for id, b := range m.budgets {
		if b.Status == StatusActive {
			budgetIDs = append(budgetIDs, id)
		}
	}
	m.mu.RUnlock()

	var results []*ForecastResult
	for _, id := range budgetIDs {
		result, err := m.ForecastCost(id, monthsAhead)
		if err == nil {
			results = append(results, result)
		}
	}

	return results
}

// GetHistoricalData 获取历史数据.
func (m *BudgetManager) GetHistoricalData(budgetID string) ([]HistoricalDataPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.budgets[budgetID]; !ok {
		return nil, ErrBudgetNotFound
	}

	expenses := m.expenses[budgetID]
	monthlyData := m.aggregateMonthly(expenses)

	points := make([]HistoricalDataPoint, len(monthlyData))
	for i, d := range monthlyData {
		points[i] = HistoricalDataPoint(d)
	}

	return points, nil
}

// ========== 辅助方法 ==========

// monthlyAggregate 月度聚合数据.
type monthlyAggregate struct {
	Date   time.Time
	Amount float64
}

// aggregateMonthly 按月聚合支出.
func (m *BudgetManager) aggregateMonthly(expenses []*Expense) []monthlyAggregate {
	if len(expenses) == 0 {
		return nil
	}

	// 按月份分组
	monthMap := make(map[string]float64)
	monthTime := make(map[string]time.Time)

	for _, e := range expenses {
		key := e.OccurredAt.Format("2006-01")
		monthMap[key] += e.Amount
		if _, ok := monthTime[key]; !ok {
			monthTime[key] = time.Date(
				e.OccurredAt.Year(), e.OccurredAt.Month(), 1,
				0, 0, 0, 0, e.OccurredAt.Location(),
			)
		}
	}

	// 转换为切片并排序
	result := make([]monthlyAggregate, 0, len(monthMap))
	for key, amount := range monthMap {
		result = append(result, monthlyAggregate{
			Date:   monthTime[key],
			Amount: amount,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.Before(result[j].Date)
	})

	return result
}

// linearRegression 线性回归.
func linearRegression(x, y []float64) (slope, intercept, r2 float64) {
	n := float64(len(x))
	if n == 0 {
		return 0, 0, 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n, 0
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n

	// 计算 R²
	meanY := sumY / n
	var ssTotal, ssResidual float64
	for i := range y {
		predicted := slope*x[i] + intercept
		ssTotal += (y[i] - meanY) * (y[i] - meanY)
		ssResidual += (y[i] - predicted) * (y[i] - predicted)
	}

	if ssTotal > 0 {
		r2 = 1 - ssResidual/ssTotal
	}

	return slope, intercept, r2
}

// calculateStdError 计算标准误差.
func calculateStdError(x, y []float64, slope, intercept float64) float64 {
	n := float64(len(x))
	if n <= 2 {
		return 0
	}

	var sumSquaredResiduals float64
	for i := range x {
		predicted := slope*x[i] + intercept
		residual := y[i] - predicted
		sumSquaredResiduals += residual * residual
	}

	return math.Sqrt(sumSquaredResiduals / (n - 2))
}

// generateForecastRecommendations 生成预测建议.
func (m *BudgetManager) generateForecastRecommendations(
	budget *Budget, slope, growthRate float64, endDate *time.Time,
) []string {
	var recommendations []string

	// 检查预算是否可能超支
	if endDate != nil && endDate.Before(budget.EndDate) {
		recommendations = append(recommendations,
			"按照当前支出趋势，预算可能在周期结束前耗尽，建议控制支出或增加预算")
	}

	// 检查增长率
	if growthRate > 20 {
		recommendations = append(recommendations,
			"支出增长较快，建议审查支出项目，寻找优化空间")
	} else if growthRate < -10 {
		recommendations = append(recommendations,
			"支出呈下降趋势，可考虑适当削减预算或重新分配资源")
	}

	// 检查使用率
	if budget.UsagePercent > 80 {
		recommendations = append(recommendations,
			"预算使用率已超过80%，建议密切关注后续支出")
	}

	// 检查剩余天数
	daysRemaining := time.Until(budget.EndDate).Hours() / 24
	if daysRemaining > 0 && slope > 0 {
		dailyBudget := budget.RemainingAmount / daysRemaining
		dailyAvgExpense := slope / 30 // 月增长率转日均
		if dailyAvgExpense > dailyBudget {
			recommendations = append(recommendations,
				"当前日均支出超出剩余预算可承受范围，建议立即采取措施")
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "预算执行正常，无需特别关注")
	}

	return recommendations
}

// roundTo2 保留两位小数.
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}
