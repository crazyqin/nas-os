// Package familyfinance 提供家庭财务中心功能
// budget.go - 预算管理，支持月度预算、分类预算、超支预警
package familyfinance

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// BudgetManager 预算管理器
type BudgetManager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	engine   *FinanceEngine
	budgets  map[string]*Budget
}

// NewBudgetManager 创建预算管理器
func NewBudgetManager(logger *zap.Logger, engine *FinanceEngine) *BudgetManager {
	return &BudgetManager{
		logger:  logger,
		engine:  engine,
		budgets: make(map[string]*Budget),
	}
}

// CreateBudget 创建预算
func (bm *BudgetManager) CreateBudget(input *Budget) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if input.ID == "" {
		input.ID = uuid.New().String()
	}

	if input.Amount <= 0 {
		return ErrInvalidAmount
	}

	input.Spent = 0
	input.Remaining = input.Amount
	input.UsagePercent = 0
	input.IsAlerted = false
	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()

	if input.AlertPercent <= 0 {
		input.AlertPercent = 80 // 默认80%预警
	}

	bm.budgets[input.ID] = input
	bm.logger.Info("预算已创建",
		zap.String("id", input.ID),
		zap.Float64("amount", input.Amount))
	return nil
}

// UpdateBudget 更新预算
func (bm *BudgetManager) UpdateBudget(budget *Budget) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	existing, exists := bm.budgets[budget.ID]
	if !exists {
		return ErrBudgetNotFound
	}

	budget.Spent = existing.Spent
	budget.CreatedAt = existing.CreatedAt
	budget.UpdatedAt = time.Now()
	bm.recalculate(budget)
	bm.budgets[budget.ID] = budget
	return nil
}

// DeleteBudget 删除预算
func (bm *BudgetManager) DeleteBudget(budgetID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if _, exists := bm.budgets[budgetID]; !exists {
		return ErrBudgetNotFound
	}

	delete(bm.budgets, budgetID)
	bm.logger.Info("预算已删除", zap.String("id", budgetID))
	return nil
}

// GetBudget 获取预算
func (bm *BudgetManager) GetBudget(budgetID string) (*Budget, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	budget, exists := bm.budgets[budgetID]
	if !exists {
		return nil, ErrBudgetNotFound
	}
	return budget, nil
}

// ListBudgets 列出所有预算
func (bm *BudgetManager) ListBudgets() []*Budget {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	budgets := make([]*Budget, 0, len(bm.budgets))
	for _, budget := range bm.budgets {
		budgets = append(budgets, budget)
	}
	return budgets
}

// RecordExpense 记录支出到预算
func (bm *BudgetManager) RecordExpense(categoryID string, amount float64) (bool, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	alertTriggered := false

	// 查找匹配的预算
	for _, budget := range bm.budgets {
		if budget.CategoryID == categoryID || budget.CategoryID == "" {
			budget.Spent += amount
			bm.recalculate(budget)

			// 检查是否超支预警
			if budget.UsagePercent >= budget.AlertPercent && !budget.IsAlerted {
				budget.IsAlerted = true
				bm.logger.Warn("预算预警",
					zap.String("budget_id", budget.ID),
					zap.Float64("usage_percent", budget.UsagePercent))
				alertTriggered = true
			}

			// 检查是否超支
			if budget.Spent > budget.Amount {
				return alertTriggered, ErrBudgetExceeded
			}
		}
	}
	return alertTriggered, nil
}

// recalculate 重新计算预算状态
func (bm *BudgetManager) recalculate(budget *Budget) {
	budget.Remaining = budget.Amount - budget.Spent
	if budget.Amount > 0 {
		budget.UsagePercent = (budget.Spent / budget.Amount) * 100
	}
	if budget.Remaining < 0 {
		budget.Remaining = 0
	}
}

// GetBudgetStatus 获取预算状态汇总
func (bm *BudgetManager) GetBudgetStatus() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	totalBudget := 0.0
	totalSpent := 0.0
	exceededCount := 0
	alertCount := 0

	for _, budget := range bm.budgets {
		totalBudget += budget.Amount
		totalSpent += budget.Spent
		if budget.Spent > budget.Amount {
			exceededCount++
		}
		if budget.IsAlerted {
			alertCount++
		}
	}

	return map[string]interface{}{
		"total_budgets": len(bm.budgets),
		"total_budget":  totalBudget,
		"total_spent":   totalSpent,
		"exceeded":      exceededCount,
		"alerts":        alertCount,
	}
}

// ResetBudgets 重置周期性预算
func (bm *BudgetManager) ResetBudgets() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	for _, budget := range bm.budgets {
		if budget.EndDate.Before(now) {
			budget.Spent = 0
			budget.Remaining = budget.Amount
			budget.UsagePercent = 0
			budget.IsAlerted = false
			budget.UpdatedAt = now

			// 根据周期设置新的结束日期
			switch budget.Period {
			case "monthly":
				budget.StartDate = now
				budget.EndDate = now.AddDate(0, 1, 0)
			case "weekly":
				budget.StartDate = now
				budget.EndDate = now.AddDate(0, 0, 7)
			case "yearly":
				budget.StartDate = now
				budget.EndDate = now.AddDate(1, 0, 0)
			}

			bm.logger.Info("预算已重置", zap.String("id", budget.ID))
		}
	}
}

// GetCategoryBudgets 获取分类预算详情
func (bm *BudgetManager) GetCategoryBudgets() []map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var result []map[string]interface{}
	for _, budget := range bm.budgets {
		categoryName := "总预算"
		if budget.CategoryID != "" {
			cats := bm.engine.GetCategories()
			for _, c := range cats {
				if c.ID == budget.CategoryID {
					categoryName = c.Name
					break
				}
			}
		}

		status := "正常"
		if budget.Spent > budget.Amount {
			status = "超支"
		} else if budget.IsAlerted {
			status = "预警"
		}

		result = append(result, map[string]interface{}{
			"id":            budget.ID,
			"category_name": categoryName,
			"amount":        budget.Amount,
			"spent":         budget.Spent,
			"remaining":     budget.Remaining,
			"usage_percent": fmt.Sprintf("%.1f%%", budget.UsagePercent),
			"status":        status,
		})
	}
	return result
}
