package aitokenmeter

import (
	"errors"
	"sync"
	"time"
)

// BudgetManager 预算管理器 (并发安全).
type BudgetManager struct {
	mu      sync.RWMutex
	budgets map[string]*Budget // key: budgetID
	handler AlertHandler       // 告警回调
}

// NewBudgetManager 创建预算管理器.
func NewBudgetManager(handler AlertHandler) *BudgetManager {
	return &BudgetManager{
		budgets: make(map[string]*Budget),
		handler: handler,
	}
}

// ========== 预算 CRUD ==========

// SetBudget 创建或更新预算.
func (bm *BudgetManager) SetBudget(budget Budget) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	if budget.CreatedAt.IsZero() {
		budget.CreatedAt = now
	}
	budget.UpdatedAt = now
	bm.budgets[budget.ID] = &budget
}

// GetBudget 获取预算.
func (bm *BudgetManager) GetBudget(budgetID string) (*Budget, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	b, ok := bm.budgets[budgetID]
	if !ok {
		return nil, false
	}
	cp := *b
	return &cp, true
}

// DeleteBudget 删除预算.
func (bm *BudgetManager) DeleteBudget(budgetID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	delete(bm.budgets, budgetID)
}

// ListBudgets 列出所有预算.
func (bm *BudgetManager) ListBudgets() []Budget {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make([]Budget, 0, len(bm.budgets))
	for _, b := range bm.budgets {
		cp := *b
		result = append(result, cp)
	}
	return result
}

// ========== 预算消耗 ==========

// Spend 记录预算消耗，返回 ErrBudgetExceeded 如果超限.
func (bm *BudgetManager) Spend(budgetID string, cost float64) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	b, ok := bm.budgets[budgetID]
	if !ok {
		return ErrBudgetNotFound()
	}
	if !b.Enabled {
		return nil
	}

	newSpent := b.Spent + cost
	if newSpent > b.Amount {
		bm.triggerAlert(Alert{
			ID:        "alert-" + budgetID + "-" + time.Now().Format("20060102150405"),
			Level:     AlertLevelCritical,
			Message:   "budget exceeded",
			BudgetID:  budgetID,
			Threshold: b.Amount,
			Actual:    newSpent,
			Timestamp: time.Now(),
		})
		return ErrBudgetExceeded
	}

	// 检查告警阈值
	if b.AlertThreshold > 0 {
		ratio := newSpent / b.Amount
		if ratio >= b.AlertThreshold {
			bm.triggerAlert(Alert{
				ID:        "alert-" + budgetID + "-" + time.Now().Format("20060102150405"),
				Level:     AlertLevelWarning,
				Message:   "budget threshold reached",
				BudgetID:  budgetID,
				Threshold: b.AlertThreshold,
				Actual:    ratio,
				Timestamp: time.Now(),
			})
		}
	}

	b.Spent = newSpent
	b.UpdatedAt = time.Now()
	return nil
}

// CheckBudget 检查预算是否允许指定消费（不实际扣减）.
func (bm *BudgetManager) CheckBudget(budgetID string, cost float64) error {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	b, ok := bm.budgets[budgetID]
	if !ok {
		return ErrBudgetNotFound()
	}
	if !b.Enabled {
		return nil
	}
	if b.Spent+cost > b.Amount {
		return ErrBudgetExceeded
	}
	return nil
}

// ResetBudget 重置预算已花费金额.
func (bm *BudgetManager) ResetBudget(budgetID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if b, ok := bm.budgets[budgetID]; ok {
		b.Spent = 0
		b.UpdatedAt = time.Now()
	}
}

// ========== 告警 ==========

// SetAlertHandler 设置告警回调.
func (bm *BudgetManager) SetAlertHandler(handler AlertHandler) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.handler = handler
}

// triggerAlert 触发告警（需持有锁）.
func (bm *BudgetManager) triggerAlert(alert Alert) {
	if bm.handler != nil {
		// 异步触发，避免阻塞
		go bm.handler(alert)
	}
}

// GetBudgetUsage 获取预算使用率.
func (bm *BudgetManager) GetBudgetUsage(budgetID string) (spent, total, ratio float64) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	b, ok := bm.budgets[budgetID]
	if !ok {
		return 0, 0, 0
	}
	ratio = 0.0
	if b.Amount > 0 {
		ratio = b.Spent / b.Amount
	}
	return b.Spent, b.Amount, ratio
}

// FindBudgetsByTarget 查找目标关联的所有预算.
func (bm *BudgetManager) FindBudgetsByTarget(targetID string, budgetType BudgetType) []Budget {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var result []Budget
	for _, b := range bm.budgets {
		if b.Type == budgetType && b.TargetID == targetID && b.Enabled {
			cp := *b
			result = append(result, cp)
		}
	}
	return result
}

// ErrBudgetNotFound 返回预算未找到错误.
func ErrBudgetNotFound() error {
	return errors.New("budget not found")
}
