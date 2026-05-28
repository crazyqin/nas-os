// Package familyfinance 提供家庭财务中心功能
// billmanager.go - 账单管理，支持周期账单、自动提醒、逾期追踪
package familyfinance

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// BillManager 账单管理器
type BillManager struct {
	mu     sync.RWMutex
	logger *zap.Logger
	engine *FinanceEngine
	bills  map[string]*Bill
}

// NewBillManager 创建账单管理器
func NewBillManager(logger *zap.Logger, engine *FinanceEngine) *BillManager {
	return &BillManager{
		logger: logger,
		engine: engine,
		bills:  make(map[string]*Bill),
	}
}

// CreateBill 创建账单
func (bm *BillManager) CreateBill(bill *Bill) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bill.ID == "" {
		bill.ID = uuid.New().String()
	}

	if bill.Amount <= 0 {
		return ErrInvalidAmount
	}

	if bill.ReminderDays <= 0 {
		bill.ReminderDays = 3 // 默认提前提醒3天
	}

	bill.Enabled = true
	bill.IsOverdue = false
	bill.CreatedAt = time.Now()
	bill.UpdatedAt = time.Now()

	// 计算下次到期日（如果未设置）
	if bill.NextDueDate.IsZero() {
		bill.NextDueDate = bm.calculateNextDueDate(bill)
	}

	bm.bills[bill.ID] = bill
	bm.logger.Info("账单已创建",
		zap.String("id", bill.ID),
		zap.String("name", bill.Name),
		zap.Float64("amount", bill.Amount))
	return nil
}

// UpdateBill 更新账单
func (bm *BillManager) UpdateBill(bill *Bill) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	existing, exists := bm.bills[bill.ID]
	if !exists {
		return ErrBillNotFound
	}

	bill.CreatedAt = existing.CreatedAt
	bill.LastPaidAt = existing.LastPaidAt
	bill.UpdatedAt = time.Now()
	bill.NextDueDate = bm.calculateNextDueDate(bill)

	bm.bills[bill.ID] = bill
	return nil
}

// DeleteBill 删除账单
func (bm *BillManager) DeleteBill(billID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if _, exists := bm.bills[billID]; !exists {
		return ErrBillNotFound
	}

	delete(bm.bills, billID)
	bm.logger.Info("账单已删除", zap.String("id", billID))
	return nil
}

// GetBill 获取账单
func (bm *BillManager) GetBill(billID string) (*Bill, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	bill, exists := bm.bills[billID]
	if !exists {
		return nil, ErrBillNotFound
	}
	return bill, nil
}

// ListBills 列出所有账单
func (bm *BillManager) ListBills() []*Bill {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	bills := make([]*Bill, 0, len(bm.bills))
	for _, bill := range bm.bills {
		bills = append(bills, bill)
	}
	return bills
}

// PayBill 支付账单
func (bm *BillManager) PayBill(billID string, accountID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bill, exists := bm.bills[billID]
	if !exists {
		return ErrBillNotFound
	}

	// 记录支出交易
	tx := &Transaction{
		AccountID:    accountID,
		Type:         TransactionTypeExpense,
		Amount:       bill.Amount,
		CategoryID:   bill.CategoryID,
		Description:  "账单支付: " + bill.Name,
	}

	if err := bm.engine.AddTransaction(tx); err != nil {
		return err
	}

	now := time.Now()
	bill.LastPaidAt = &now
	bill.IsOverdue = false
	bill.NextDueDate = bm.calculateNextDueDate(bill)
	bill.UpdatedAt = now

	bm.logger.Info("账单已支付",
		zap.String("id", billID),
		zap.Float64("amount", bill.Amount))
	return nil
}

// GetUpcomingBills 获取即将到期的账单
func (bm *BillManager) GetUpcomingBills(days int) []*Bill {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var upcoming []*Bill
	cutoff := time.Now().AddDate(0, 0, days)

	for _, bill := range bm.bills {
		if bill.Enabled && bill.NextDueDate.Before(cutoff) {
			upcoming = append(upcoming, bill)
		}
	}

	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].NextDueDate.Before(upcoming[j].NextDueDate)
	})

	return upcoming
}

// GetOverdueBills 获取逾期账单
func (bm *BillManager) GetOverdueBills() []*Bill {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var overdue []*Bill
	now := time.Now()

	for _, bill := range bm.bills {
		if bill.Enabled && bill.NextDueDate.Before(now) {
			bill.IsOverdue = true
			overdue = append(overdue, bill)
		}
	}

	sort.Slice(overdue, func(i, j int) bool {
		return overdue[i].NextDueDate.Before(overdue[j].NextDueDate)
	})

	return overdue
}

// GetBillReminders 获取需要提醒的账单
func (bm *BillManager) GetBillReminders() []*Bill {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var reminders []*Bill
	now := time.Now()

	for _, bill := range bm.bills {
		if !bill.Enabled {
			continue
		}

		reminderDate := bill.NextDueDate.AddDate(0, 0, -bill.ReminderDays)
		if now.After(reminderDate) && now.Before(bill.NextDueDate) {
			reminders = append(reminders, bill)
		}
	}

	return reminders
}

// GetBillSummary 获取账单汇总
func (bm *BillManager) GetBillSummary() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	totalMonthly := 0.0
	totalYearly := 0.0
	overdueCount := 0
	autoPayCount := 0

	for _, bill := range bm.bills {
		if !bill.Enabled {
			continue
		}

		switch bill.Cycle {
		case BillCycleDaily:
			totalMonthly += bill.Amount * 30
			totalYearly += bill.Amount * 365
		case BillCycleWeekly:
			totalMonthly += bill.Amount * 4
			totalYearly += bill.Amount * 52
		case BillCycleMonthly:
			totalMonthly += bill.Amount
			totalYearly += bill.Amount * 12
		case BillCycleQuarterly:
			totalMonthly += bill.Amount / 3
			totalYearly += bill.Amount * 4
		case BillCycleYearly:
			totalMonthly += bill.Amount / 12
			totalYearly += bill.Amount
		}

		if bill.IsOverdue {
			overdueCount++
		}
		if bill.IsAutoPay {
			autoPayCount++
		}
	}

	return map[string]interface{}{
		"total_bills":     len(bm.bills),
		"total_monthly":   totalMonthly,
		"total_yearly":    totalYearly,
		"overdue_count":   overdueCount,
		"auto_pay_count":  autoPayCount,
	}
}

// calculateNextDueDate 计算下次到期日
func (bm *BillManager) calculateNextDueDate(bill *Bill) time.Time {
	now := time.Now()

	switch bill.Cycle {
	case BillCycleDaily:
		return now.AddDate(0, 0, 1)
	case BillCycleWeekly:
		return now.AddDate(0, 0, 7)
	case BillCycleMonthly:
		// 设置为下个月的dueDay
		nextMonth := now.AddDate(0, 1, 0)
		day := bill.DueDay
		if day > 28 {
			day = 28 // 避免月份天数问题
		}
		return time.Date(nextMonth.Year(), nextMonth.Month(), day, 0, 0, 0, 0, now.Location())
	case BillCycleQuarterly:
		return now.AddDate(0, 3, 0)
	case BillCycleYearly:
		return now.AddDate(1, 0, 0)
	case BillCycleOnce:
		// 一次性账单返回原始到期日
		return bill.NextDueDate
	default:
		return now.AddDate(0, 1, 0)
	}
}

// CheckOverdue 检查并更新逾期状态
func (bm *BillManager) CheckOverdue() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	for _, bill := range bm.bills {
		if bill.Enabled && bill.NextDueDate.Before(now) {
			bill.IsOverdue = true
		}
	}
}
