package budgetplan

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// ========== 报表生成 (ReportGen) ==========

// GenerateReport 生成财务报表.
func (m *BudgetManager) GenerateReport(request ReportRequest, generatedBy string) (*Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 确定时间范围
	startDate, endDate := m.resolveReportPeriod(request)

	// 收集相关预算
	budgets := m.collectReportBudgets(request.BudgetIDs)

	// 计算摘要
	summary := m.calculateReportSummary(budgets)

	// 生成预算详情
	budgetDetails := m.generateBudgetDetails(budgets)

	// 收集所有支出
	allExpenses := m.collectReportExpenses(budgets, startDate, endDate)

	// 生成支出摘要
	expenseSummary := m.generateExpenseSummary(allExpenses)

	// 生成趋势数据
	trendData := m.generateTrendData(budgets, startDate, endDate)

	// 获取最大支出
	topExpenses := m.getTopExpenses(allExpenses, request.TopExpensesCount)
	if !request.IncludeTopExpenses {
		topExpenses = nil
	}

	reportName := request.Name
	if reportName == "" {
		reportName = m.generateReportName(request.Type, startDate, endDate)
	}

	return &Report{
		ID:             uuid.New().String(),
		Name:           reportName,
		Type:           request.Type,
		GeneratedAt:    time.Now(),
		PeriodStart:    startDate,
		PeriodEnd:      endDate,
		Summary:        summary,
		BudgetDetails:  budgetDetails,
		ExpenseSummary: expenseSummary,
		TrendData:      trendData,
		TopExpenses:    topExpenses,
		GeneratedBy:    generatedBy,
	}, nil
}

// GenerateMonthlyReport 生成月度报表.
func (m *BudgetManager) GenerateMonthlyReport(year int, month time.Month, generatedBy string) (*Report, error) {
	startDate := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 1, -1)

	return m.GenerateReport(ReportRequest{
		Type:               ReportTypeMonthly,
		StartDate:          &startDate,
		EndDate:            &endDate,
		IncludeTopExpenses: true,
		TopExpensesCount:   10,
	}, generatedBy)
}

// GenerateQuarterlyReport 生成季度报表.
func (m *BudgetManager) GenerateQuarterlyReport(year int, quarter int, generatedBy string) (*Report, error) {
	if quarter < 1 || quarter > 4 {
		return nil, ErrInvalidInput
	}

	startMonth := time.Month((quarter-1)*3 + 1)
	startDate := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 3, -1)

	return m.GenerateReport(ReportRequest{
		Type:               ReportTypeQuarterly,
		StartDate:          &startDate,
		EndDate:            &endDate,
		IncludeTopExpenses: true,
		TopExpensesCount:   20,
	}, generatedBy)
}

// GenerateYearlyReport 生成年度报表.
func (m *BudgetManager) GenerateYearlyReport(year int, generatedBy string) (*Report, error) {
	startDate := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	endDate := time.Date(year, 12, 31, 23, 59, 59, 0, time.Local)

	return m.GenerateReport(ReportRequest{
		Type:               ReportTypeYearly,
		StartDate:          &startDate,
		EndDate:            &endDate,
		IncludeTopExpenses: true,
		TopExpensesCount:   50,
	}, generatedBy)
}

// ========== 辅助方法 ==========

// resolveReportPeriod 解析报表时间范围.
func (m *BudgetManager) resolveReportPeriod(request ReportRequest) (time.Time, time.Time) {
	now := time.Now()
	var startDate, endDate time.Time

	if request.StartDate != nil {
		startDate = *request.StartDate
	} else {
		switch request.Type {
		case ReportTypeMonthly:
			startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		case ReportTypeQuarterly:
			quarter := (int(now.Month()) - 1) / 3
			startDate = time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, time.Local)
		case ReportTypeYearly:
			startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		default:
			startDate = now.AddDate(0, -1, 0)
		}
	}

	if request.EndDate != nil {
		endDate = *request.EndDate
	} else {
		switch request.Type {
		case ReportTypeMonthly:
			endDate = startDate.AddDate(0, 1, -1)
		case ReportTypeQuarterly:
			endDate = startDate.AddDate(0, 3, -1)
		case ReportTypeYearly:
			endDate = time.Date(now.Year(), 12, 31, 23, 59, 59, 0, time.Local)
		default:
			endDate = now
		}
	}

	return startDate, endDate
}

// collectReportBudgets 收集报表相关预算.
func (m *BudgetManager) collectReportBudgets(budgetIDs []string) []*Budget {
	if len(budgetIDs) == 0 {
		// 返回所有预算
		budgets := make([]*Budget, 0, len(m.budgets))
		for _, b := range m.budgets {
			budgets = append(budgets, b)
		}
		return budgets
	}

	budgets := make([]*Budget, 0, len(budgetIDs))
	for _, id := range budgetIDs {
		if b, ok := m.budgets[id]; ok {
			budgets = append(budgets, b)
		}
	}
	return budgets
}

// calculateReportSummary 计算报表摘要.
func (m *BudgetManager) calculateReportSummary(budgets []*Budget) ReportSummary {
	summary := ReportSummary{
		TotalBudgets: len(budgets),
	}

	var totalUsage float64
	for _, b := range budgets {
		if b.Status == StatusActive {
			summary.ActiveBudgets++
		}
		summary.TotalBudgetAmount += b.TotalAmount
		summary.TotalExpenses += b.UsedAmount
		summary.TotalRemaining += b.RemainingAmount
		totalUsage += b.UsagePercent

		if b.Status == StatusExceeded {
			summary.ExceededBudgets++
		}
	}

	if summary.TotalBudgets > 0 {
		summary.AvgUsagePercent = roundTo2(totalUsage / float64(summary.TotalBudgets))
	}

	// 计算健康评分
	summary.HealthScore = m.calculateHealthScore(budgets)

	return summary
}

// calculateHealthScore 计算健康评分.
func (m *BudgetManager) calculateHealthScore(budgets []*Budget) int {
	if len(budgets) == 0 {
		return 100
	}

	totalScore := 0
	for _, b := range budgets {
		score := 100

		// 超支扣分
		if b.UsagePercent > 100 {
			score -= 50
		} else if b.UsagePercent > 90 {
			score -= 30
		} else if b.UsagePercent > 80 {
			score -= 15
		} else if b.UsagePercent > 70 {
			score -= 5
		}

		// 状态扣分
		switch b.Status {
		case StatusExceeded:
			score -= 20
		case StatusPaused:
			score -= 10
		}

		if score < 0 {
			score = 0
		}
		totalScore += score
	}

	return totalScore / len(budgets)
}

// generateBudgetDetails 生成预算详情.
func (m *BudgetManager) generateBudgetDetails(budgets []*Budget) []BudgetReportDetail {
	details := make([]BudgetReportDetail, 0, len(budgets))

	for _, b := range budgets {
		expenseCount := len(m.expenses[b.ID])
		details = append(details, BudgetReportDetail{
			BudgetID:        b.ID,
			BudgetName:      b.Name,
			Period:          b.Period,
			TotalAmount:     b.TotalAmount,
			UsedAmount:      b.UsedAmount,
			RemainingAmount: b.RemainingAmount,
			UsagePercent:    b.UsagePercent,
			Status:          b.Status,
			ExpenseCount:    expenseCount,
		})
	}

	return details
}

// collectReportExpenses 收集报表相关支出.
func (m *BudgetManager) collectReportExpenses(budgets []*Budget, start, end time.Time) []*Expense {
	var allExpenses []*Expense

	for _, b := range budgets {
		for _, e := range m.expenses[b.ID] {
			if !e.OccurredAt.Before(start) && !e.OccurredAt.After(end) {
				allExpenses = append(allExpenses, e)
			}
		}
	}

	return allExpenses
}

// generateExpenseSummary 生成支出摘要.
func (m *BudgetManager) generateExpenseSummary(expenses []*Expense) []ExpenseSummaryItem {
	categoryTotals := make(map[ExpenseCategory]float64)
	categoryCounts := make(map[ExpenseCategory]int)
	var grandTotal float64

	for _, e := range expenses {
		categoryTotals[e.Category] += e.Amount
		categoryCounts[e.Category]++
		grandTotal += e.Amount
	}

	summary := make([]ExpenseSummaryItem, 0, len(categoryTotals))
	for cat, total := range categoryTotals {
		percent := 0.0
		if grandTotal > 0 {
			percent = total / grandTotal * 100
		}
		summary = append(summary, ExpenseSummaryItem{
			Category:    cat,
			TotalAmount: roundTo2(total),
			Count:       categoryCounts[cat],
			Percent:     roundTo2(percent),
		})
	}

	// 按金额降序排序
	sort.Slice(summary, func(i, j int) bool {
		return summary[i].TotalAmount > summary[j].TotalAmount
	})

	return summary
}

// generateTrendData 生成趋势数据.
func (m *BudgetManager) generateTrendData(budgets []*Budget, start, end time.Time) []TrendDataPoint {
	// 按月生成趋势数据
	var trendData []TrendDataPoint

	current := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.Local)
	endMonth := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.Local)

	for !current.After(endMonth) {
		monthEnd := current.AddDate(0, 1, -1)
		if monthEnd.After(end) {
			monthEnd = end
		}

		var budgetAmount, expenseAmount float64
		for _, b := range budgets {
			budgetAmount += b.TotalAmount
			for _, e := range m.expenses[b.ID] {
				if !e.OccurredAt.Before(current) && !e.OccurredAt.After(monthEnd) {
					expenseAmount += e.Amount
				}
			}
		}

		trendData = append(trendData, TrendDataPoint{
			Date:          current,
			BudgetAmount:  roundTo2(budgetAmount),
			ExpenseAmount: roundTo2(expenseAmount),
			Remaining:     roundTo2(budgetAmount - expenseAmount),
		})

		current = current.AddDate(0, 1, 0)
	}

	return trendData
}

// getTopExpenses 获取最大支出.
func (m *BudgetManager) getTopExpenses(expenses []*Expense, count int) []Expense {
	if count <= 0 {
		count = 10
	}

	// 按金额降序排序
	sorted := make([]*Expense, len(expenses))
	copy(sorted, expenses)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Amount > sorted[j].Amount
	})

	result := make([]Expense, 0, count)
	for i := 0; i < len(sorted) && i < count; i++ {
		result = append(result, *sorted[i])
	}

	return result
}

// generateReportName 生成报表名称.
func (m *BudgetManager) generateReportName(reportType ReportType, start, end time.Time) string {
	switch reportType {
	case ReportTypeMonthly:
		return start.Format("2006年01月") + "预算报表"
	case ReportTypeQuarterly:
		quarter := (int(start.Month())-1)/3 + 1
		return start.Format("2006年") + "第" + string(rune('0'+quarter)) + "季度预算报表"
	case ReportTypeYearly:
		return start.Format("2006年") + "年度预算报表"
	default:
		return start.Format("20060102") + "-" + end.Format("20060102") + "预算报表"
	}
}
