package budgetplan

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 预算编制管理器 ==========

// BudgetManager 预算编制管理器.
type BudgetManager struct {
	mu       sync.RWMutex
	budgets  map[string]*Budget
	expenses map[string][]*Expense
}

// NewBudgetManager 创建预算编制管理器.
func NewBudgetManager() *BudgetManager {
	return &BudgetManager{
		budgets:  make(map[string]*Budget),
		expenses: make(map[string][]*Expense),
	}
}

// ========== 预算编制 (BudgetCreate) ==========

// CreateBudget 创建预算.
func (m *BudgetManager) CreateBudget(input BudgetCreateInput, createdBy string) (*Budget, error) {
	if err := m.validateCreateInput(input); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否重名
	for _, b := range m.budgets {
		if b.Name == input.Name && b.Status != StatusCompleted {
			return nil, ErrBudgetExists
		}
	}

	now := time.Now()
	startDate := now
	endDate := m.calculateEndDate(now, input.Period)

	if input.StartDate != nil {
		startDate = *input.StartDate
	}
	if input.EndDate != nil {
		endDate = *input.EndDate
	}

	// 处理分类预算分配
	categories := make(map[ExpenseCategory]float64)
	if input.Categories != nil {
		categories = input.Categories
	}

	budget := &Budget{
		ID:              uuid.New().String(),
		Name:            input.Name,
		Description:     input.Description,
		Period:          input.Period,
		TotalAmount:     input.TotalAmount,
		UsedAmount:      0,
		RemainingAmount: input.TotalAmount,
		UsagePercent:    0,
		Status:          StatusActive,
		StartDate:       startDate,
		EndDate:         endDate,
		Categories:      categories,
		CreatedAt:       now,
		UpdatedAt:       now,
		CreatedBy:       createdBy,
		Tags:            input.Tags,
	}

	m.budgets[budget.ID] = budget
	return budget, nil
}

// GetBudget 获取预算.
func (m *BudgetManager) GetBudget(id string) (*Budget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[id]
	if !ok {
		return nil, ErrBudgetNotFound
	}
	return budget, nil
}

// ListBudgets 列出所有预算.
func (m *BudgetManager) ListBudgets() []*Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Budget, 0, len(m.budgets))
	for _, b := range m.budgets {
		result = append(result, b)
	}
	return result
}

// ListActiveBudgets 列出活跃预算.
func (m *BudgetManager) ListActiveBudgets() []*Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Budget
	for _, b := range m.budgets {
		if b.Status == StatusActive {
			result = append(result, b)
		}
	}
	return result
}

// UpdateBudget 更新预算.
func (m *BudgetManager) UpdateBudget(id string, updates BudgetCreateInput) (*Budget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, ok := m.budgets[id]
	if !ok {
		return nil, ErrBudgetNotFound
	}

	if updates.Name != "" {
		budget.Name = updates.Name
	}
	if updates.Description != "" {
		budget.Description = updates.Description
	}
	if updates.TotalAmount > 0 {
		budget.TotalAmount = updates.TotalAmount
		budget.RemainingAmount = updates.TotalAmount - budget.UsedAmount
		budget.UsagePercent = m.calculateUsagePercent(budget.UsedAmount, updates.TotalAmount)
	}
	if updates.Categories != nil {
		budget.Categories = updates.Categories
	}
	if updates.Tags != nil {
		budget.Tags = updates.Tags
	}

	budget.UpdatedAt = time.Now()
	return budget, nil
}

// DeleteBudget 删除预算.
func (m *BudgetManager) DeleteBudget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.budgets[id]; !ok {
		return ErrBudgetNotFound
	}
	delete(m.budgets, id)
	delete(m.expenses, id)
	return nil
}

// PauseBudget 暂停预算.
func (m *BudgetManager) PauseBudget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, ok := m.budgets[id]
	if !ok {
		return ErrBudgetNotFound
	}
	if budget.Status != StatusActive {
		return fmt.Errorf("预算状态 %s 不允许暂停", budget.Status)
	}
	budget.Status = StatusPaused
	budget.UpdatedAt = time.Now()
	return nil
}

// ResumeBudget 恢复预算.
func (m *BudgetManager) ResumeBudget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, ok := m.budgets[id]
	if !ok {
		return ErrBudgetNotFound
	}
	if budget.Status != StatusPaused {
		return fmt.Errorf("预算状态 %s 不允许恢复", budget.Status)
	}
	budget.Status = StatusActive
	budget.UpdatedAt = time.Now()
	return nil
}

// GetBudgetStats 获取预算统计.
func (m *BudgetManager) GetBudgetStats() map[BudgetStatus]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[BudgetStatus]int)
	for _, b := range m.budgets {
		stats[b.Status]++
	}
	return stats
}

// ========== 辅助方法 ==========

// validateCreateInput 验证创建输入.
func (m *BudgetManager) validateCreateInput(input BudgetCreateInput) error {
	if input.Name == "" {
		return fmt.Errorf("%w: 预算名称不能为空", ErrInvalidInput)
	}
	if input.TotalAmount <= 0 {
		return fmt.Errorf("%w: 预算总额必须大于0", ErrInvalidInput)
	}
	if input.Period == "" {
		return fmt.Errorf("%w: 预算周期不能为空", ErrInvalidInput)
	}
	// 验证周期类型
	switch input.Period {
	case PeriodMonthly, PeriodQuarterly, PeriodYearly:
		// 有效
	default:
		return fmt.Errorf("%w: 无效的预算周期 %s", ErrInvalidInput, input.Period)
	}
	return nil
}

// calculateEndDate 计算结束日期.
func (m *BudgetManager) calculateEndDate(start time.Time, period BudgetPeriod) time.Time {
	switch period {
	case PeriodMonthly:
		return start.AddDate(0, 1, 0)
	case PeriodQuarterly:
		return start.AddDate(0, 3, 0)
	case PeriodYearly:
		return start.AddDate(1, 0, 0)
	default:
		return start.AddDate(0, 1, 0)
	}
}

// calculateUsagePercent 计算使用百分比.
func (m *BudgetManager) calculateUsagePercent(used, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return used / total * 100
}
