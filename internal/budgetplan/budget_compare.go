package budgetplan

import (
	"time"
)

// ========== 预算对比 (BudgetCompare) ==========

// CompareBudget 预算vs实际对比分析.
func (m *BudgetManager) CompareBudget(budgetID string) (*BudgetComparison, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[budgetID]
	if !ok {
		return nil, ErrBudgetNotFound
	}

	expenses := m.expenses[budgetID]

	// 计算各分类实际支出
	categoryActuals := make(map[ExpenseCategory]float64)
	for _, e := range expenses {
		categoryActuals[e.Category] += e.Amount
	}

	// 构建分类对比明细
	categoryBreakdown := make([]CategoryComparison, 0)

	// 合并所有分类（预算定义的 + 实际发生的）
	allCategories := make(map[ExpenseCategory]bool)
	for cat := range budget.Categories {
		allCategories[cat] = true
	}
	for cat := range categoryActuals {
		allCategories[cat] = true
	}

	for cat := range allCategories {
		budgeted := budget.Categories[cat]
		actual := categoryActuals[cat]
		variance := budgeted - actual
		variancePct := 0.0
		if budgeted > 0 {
			variancePct = variance / budgeted * 100
		}

		categoryBreakdown = append(categoryBreakdown, CategoryComparison{
			Category:        cat,
			BudgetedAmount:  roundTo2(budgeted),
			ActualAmount:    roundTo2(actual),
			Variance:        roundTo2(variance),
			VariancePercent: roundTo2(variancePct),
		})
	}

	// 计算总体对比
	totalVariance := budget.TotalAmount - budget.UsedAmount
	totalVariancePct := 0.0
	if budget.TotalAmount > 0 {
		totalVariancePct = totalVariance / budget.TotalAmount * 100
	}

	// 判断状态
	status := "on_budget"
	if budget.UsedAmount > budget.TotalAmount {
		status = "over_budget"
	} else if budget.UsagePercent < 80 {
		status = "under_budget"
	}

	return &BudgetComparison{
		BudgetID:        budget.ID,
		BudgetName:      budget.Name,
		Period:          string(budget.Period),
		BudgetedAmount:  budget.TotalAmount,
		ActualAmount:    budget.UsedAmount,
		Variance:        roundTo2(totalVariance),
		VariancePercent: roundTo2(totalVariancePct),
		Status:          status,
		CategoryBreakdown: categoryBreakdown,
	}, nil
}

// CompareAllBudgets 对比所有活跃预算.
func (m *BudgetManager) CompareAllBudgets() *CompareResult {
	m.mu.RLock()
	budgetIDs := make([]string, 0)
	for id, b := range m.budgets {
		if b.Status == StatusActive || b.Status == StatusExceeded {
			budgetIDs = append(budgetIDs, id)
		}
	}
	m.mu.RUnlock()

	comparisons := make([]BudgetComparison, 0, len(budgetIDs))
	var totalBudgeted, totalActual float64

	for _, id := range budgetIDs {
		comp, err := m.CompareBudget(id)
		if err != nil {
			continue
		}
		comparisons = append(comparisons, *comp)
		totalBudgeted += comp.BudgetedAmount
		totalActual += comp.ActualAmount
	}

	totalVariance := totalBudgeted - totalActual
	overallStatus := "on_budget"
	if totalActual > totalBudgeted {
		overallStatus = "over_budget"
	} else if totalBudgeted > 0 && totalActual/totalBudgeted < 0.8 {
		overallStatus = "under_budget"
	}

	return &CompareResult{
		GeneratedAt:   time.Now(),
		Comparisons:   comparisons,
		TotalBudgeted: roundTo2(totalBudgeted),
		TotalActual:   roundTo2(totalActual),
		TotalVariance: roundTo2(totalVariance),
		OverallStatus: overallStatus,
	}
}

// CompareBudgetsByPeriod 按周期对比预算.
func (m *BudgetManager) CompareBudgetsByPeriod(period BudgetPeriod) []*BudgetComparison {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*BudgetComparison
	for _, budget := range m.budgets {
		if budget.Period != period {
			continue
		}

		expenses := m.expenses[budget.ID]
		categoryActuals := make(map[ExpenseCategory]float64)
		for _, e := range expenses {
			categoryActuals[e.Category] += e.Amount
		}

		totalVariance := budget.TotalAmount - budget.UsedAmount
		totalVariancePct := 0.0
		if budget.TotalAmount > 0 {
			totalVariancePct = totalVariance / budget.TotalAmount * 100
		}

		status := "on_budget"
		if budget.UsedAmount > budget.TotalAmount {
			status = "over_budget"
		} else if budget.UsagePercent < 80 {
			status = "under_budget"
		}

		categoryBreakdown := make([]CategoryComparison, 0)
		for cat, budgeted := range budget.Categories {
			actual := categoryActuals[cat]
			variance := budgeted - actual
			variancePct := 0.0
			if budgeted > 0 {
				variancePct = variance / budgeted * 100
			}
			categoryBreakdown = append(categoryBreakdown, CategoryComparison{
				Category:        cat,
				BudgetedAmount:  roundTo2(budgeted),
				ActualAmount:    roundTo2(actual),
				Variance:        roundTo2(variance),
				VariancePercent: roundTo2(variancePct),
			})
		}

		results = append(results, &BudgetComparison{
			BudgetID:          budget.ID,
			BudgetName:        budget.Name,
			Period:            string(budget.Period),
			BudgetedAmount:    budget.TotalAmount,
			ActualAmount:      budget.UsedAmount,
			Variance:          roundTo2(totalVariance),
			VariancePercent:   roundTo2(totalVariancePct),
			Status:            status,
			CategoryBreakdown: categoryBreakdown,
		})
	}

	return results
}

// GetVarianceAnalysis 获取差异分析.
func (m *BudgetManager) GetVarianceAnalysis(budgetID string) (map[string]interface{}, error) {
	comp, err := m.CompareBudget(budgetID)
	if err != nil {
		return nil, err
	}

	// 找出最大差异分类
	var maxVarianceCat ExpenseCategory
	maxVariance := 0.0
	for _, cb := range comp.CategoryBreakdown {
		absVariance := cb.Variance
		if absVariance < 0 {
			absVariance = -absVariance
		}
		if absVariance > maxVariance {
			maxVariance = absVariance
			maxVarianceCat = cb.Category
		}
	}

	// 计算超支/节余分类
	overBudgetCategories := make([]ExpenseCategory, 0)
	underBudgetCategories := make([]ExpenseCategory, 0)
	for _, cb := range comp.CategoryBreakdown {
		if cb.Variance < 0 {
			overBudgetCategories = append(overBudgetCategories, cb.Category)
		} else if cb.Variance > 0 {
			underBudgetCategories = append(underBudgetCategories, cb.Category)
		}
	}

	return map[string]interface{}{
		"budget_id":              comp.BudgetID,
		"budget_name":            comp.BudgetName,
		"total_variance":         comp.Variance,
		"total_variance_percent": comp.VariancePercent,
		"status":                 comp.Status,
		"max_variance_category":  maxVarianceCat,
		"max_variance_amount":    maxVariance,
		"over_budget_categories": overBudgetCategories,
		"under_budget_categories": underBudgetCategories,
		"category_breakdown":     comp.CategoryBreakdown,
	}, nil
}
