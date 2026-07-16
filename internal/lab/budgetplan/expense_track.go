package budgetplan

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ========== 支出追踪 (ExpenseTrack) ==========

// RecordExpense 记录支出.
func (m *BudgetManager) RecordExpense(input ExpenseInput, createdBy string) (*Expense, error) {
	if err := m.validateExpenseInput(input); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证预算是否存在
	budget, ok := m.budgets[input.BudgetID]
	if !ok {
		return nil, ErrBudgetNotFound
	}
	if budget.Status != StatusActive {
		return nil, fmt.Errorf("预算状态 %s 不允许记录支出", budget.Status)
	}

	now := time.Now()
	occurredAt := now
	if input.OccurredAt != nil {
		occurredAt = *input.OccurredAt
	}

	expense := &Expense{
		ID:            uuid.New().String(),
		BudgetID:      input.BudgetID,
		Amount:        input.Amount,
		Category:      input.Category,
		Description:   input.Description,
		Vendor:        input.Vendor,
		InvoiceNumber: input.InvoiceNumber,
		OccurredAt:    occurredAt,
		CreatedAt:     now,
		CreatedBy:     createdBy,
		Metadata:      input.Metadata,
	}

	// 更新预算使用情况
	budget.UsedAmount += input.Amount
	budget.RemainingAmount = budget.TotalAmount - budget.UsedAmount
	budget.UsagePercent = m.calculateUsagePercent(budget.UsedAmount, budget.TotalAmount)
	budget.UpdatedAt = now

	// 检查是否超支
	if budget.UsedAmount >= budget.TotalAmount {
		budget.Status = StatusExceeded
	}

	// 保存支出记录
	m.expenses[input.BudgetID] = append(m.expenses[input.BudgetID], expense)

	return expense, nil
}

// GetExpense 获取支出记录.
func (m *BudgetManager) GetExpense(budgetID, expenseID string) (*Expense, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	expenses, ok := m.expenses[budgetID]
	if !ok {
		return nil, ErrExpenseNotFound
	}

	for _, e := range expenses {
		if e.ID == expenseID {
			return e, nil
		}
	}
	return nil, ErrExpenseNotFound
}

// ListExpenses 列出预算的所有支出.
func (m *BudgetManager) ListExpenses(budgetID string) ([]*Expense, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.budgets[budgetID]; !ok {
		return nil, ErrBudgetNotFound
	}

	expenses := m.expenses[budgetID]
	result := make([]*Expense, len(expenses))
	copy(result, expenses)
	return result, nil
}

// QueryExpenses 查询支出记录.
func (m *BudgetManager) QueryExpenses(query ExpenseQuery) []*Expense {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Expense

	// 收集所有支出
	for _, expenses := range m.expenses {
		for _, e := range expenses {
			if m.matchExpenseQuery(e, query) {
				result = append(result, e)
			}
		}
	}

	// 分页处理
	if query.PageSize <= 0 {
		query.PageSize = 50
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	start := (query.Page - 1) * query.PageSize
	if start >= len(result) {
		return []*Expense{}
	}
	end := start + query.PageSize
	if end > len(result) {
		end = len(result)
	}

	return result[start:end]
}

// GetExpenseSummary 获取支出摘要.
func (m *BudgetManager) GetExpenseSummary(budgetID string) (map[ExpenseCategory]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.budgets[budgetID]; !ok {
		return nil, ErrBudgetNotFound
	}

	expenses := m.expenses[budgetID]
	summary := make(map[ExpenseCategory]float64)

	for _, e := range expenses {
		summary[e.Category] += e.Amount
	}

	return summary, nil
}

// GetExpensesByTimeRange 按时间范围获取支出.
func (m *BudgetManager) GetExpensesByTimeRange(budgetID string, start, end time.Time) ([]*Expense, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.budgets[budgetID]; !ok {
		return nil, ErrBudgetNotFound
	}

	var result []*Expense
	for _, e := range m.expenses[budgetID] {
		if !e.OccurredAt.Before(start) && !e.OccurredAt.After(end) {
			result = append(result, e)
		}
	}

	return result, nil
}

// DeleteExpense 删除支出记录.
func (m *BudgetManager) DeleteExpense(budgetID, expenseID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, ok := m.budgets[budgetID]
	if !ok {
		return ErrBudgetNotFound
	}

	expenses := m.expenses[budgetID]
	for i, e := range expenses {
		if e.ID == expenseID {
			// 更新预算使用情况
			budget.UsedAmount -= e.Amount
			budget.RemainingAmount = budget.TotalAmount - budget.UsedAmount
			budget.UsagePercent = m.calculateUsagePercent(budget.UsedAmount, budget.TotalAmount)
			budget.UpdatedAt = time.Now()

			// 恢复状态
			if budget.Status == StatusExceeded && budget.UsedAmount < budget.TotalAmount {
				budget.Status = StatusActive
			}

			// 删除记录
			m.expenses[budgetID] = append(expenses[:i], expenses[i+1:]...)
			return nil
		}
	}

	return ErrExpenseNotFound
}

// ========== 辅助方法 ==========

// validateExpenseInput 验证支出输入.
func (m *BudgetManager) validateExpenseInput(input ExpenseInput) error {
	if input.BudgetID == "" {
		return fmt.Errorf("%w: 预算ID不能为空", ErrInvalidInput)
	}
	if input.Amount <= 0 {
		return fmt.Errorf("%w: 支出金额必须大于0", ErrInvalidInput)
	}
	if input.Category == "" {
		return fmt.Errorf("%w: 支出分类不能为空", ErrInvalidInput)
	}
	// 验证分类类型
	switch input.Category {
	case CategoryHardware, CategorySoftware, CategoryService,
		CategoryMaintenance, CategoryPower, CategoryBandwidth,
		CategoryStorage, CategoryOther:
		// 有效
	default:
		return fmt.Errorf("%w: 无效的支出分类 %s", ErrInvalidInput, input.Category)
	}
	return nil
}

// matchExpenseQuery 匹配支出查询条件.
func (m *BudgetManager) matchExpenseQuery(e *Expense, q ExpenseQuery) bool {
	if q.BudgetID != "" && e.BudgetID != q.BudgetID {
		return false
	}
	if len(q.Categories) > 0 {
		found := false
		for _, cat := range q.Categories {
			if e.Category == cat {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q.StartTime != nil && e.OccurredAt.Before(*q.StartTime) {
		return false
	}
	if q.EndTime != nil && e.OccurredAt.After(*q.EndTime) {
		return false
	}
	if q.MinAmount != nil && e.Amount < *q.MinAmount {
		return false
	}
	if q.MaxAmount != nil && e.Amount > *q.MaxAmount {
		return false
	}
	if q.Vendor != "" && e.Vendor != q.Vendor {
		return false
	}
	return true
}
