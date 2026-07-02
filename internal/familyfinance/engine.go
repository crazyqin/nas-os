// Package familyfinance 提供家庭财务中心功能
// engine.go - 财务引擎核心，负责账户管理和交易记录
package familyfinance

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FinanceEngine 财务引擎.
type FinanceEngine struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	accounts     map[string]*Account
	transactions []*Transaction
	categories   map[string]*Category
}

// NewFinanceEngine 创建财务引擎.
func NewFinanceEngine(logger *zap.Logger) *FinanceEngine {
	engine := &FinanceEngine{
		logger:     logger,
		accounts:   make(map[string]*Account),
		categories: make(map[string]*Category),
	}

	// 初始化默认分类
	for i := range DefaultCategories {
		cat := DefaultCategories[i]
		engine.categories[cat.ID] = &cat
	}

	return engine
}

// ========== 账户管理 ==========

// CreateAccount 创建账户.
func (e *FinanceEngine) CreateAccount(account *Account) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if account.ID == "" {
		account.ID = uuid.New().String()
	}

	if _, exists := e.accounts[account.ID]; exists {
		return ErrAccountExists
	}

	if account.Currency == "" {
		account.Currency = "CNY"
	}
	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()

	e.accounts[account.ID] = account
	e.logger.Info("账户已创建", zap.String("id", account.ID), zap.String("name", account.Name))
	return nil
}

// UpdateAccount 更新账户.
func (e *FinanceEngine) UpdateAccount(account *Account) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.accounts[account.ID]
	if !exists {
		return ErrAccountNotFound
	}

	account.CreatedAt = existing.CreatedAt
	account.Balance = existing.Balance // 余额通过交易更新
	account.UpdatedAt = time.Now()
	e.accounts[account.ID] = account
	return nil
}

// DeleteAccount 删除账户.
func (e *FinanceEngine) DeleteAccount(accountID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.accounts[accountID]; !exists {
		return ErrAccountNotFound
	}

	// 检查是否有关联交易
	for _, tx := range e.transactions {
		if tx.AccountID == accountID || tx.ToAccountID == accountID {
			return fmt.Errorf("账户 %s 存在关联交易，无法删除", accountID)
		}
	}

	delete(e.accounts, accountID)
	e.logger.Info("账户已删除", zap.String("id", accountID))
	return nil
}

// GetAccount 获取账户.
func (e *FinanceEngine) GetAccount(accountID string) (*Account, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	account, exists := e.accounts[accountID]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

// ListAccounts 列出所有账户.
func (e *FinanceEngine) ListAccounts() []*Account {
	e.mu.RLock()
	defer e.mu.RUnlock()

	accounts := make([]*Account, 0, len(e.accounts))
	for _, account := range e.accounts {
		accounts = append(accounts, account)
	}
	return accounts
}

// GetTotalBalance 获取所有账户总余额.
func (e *FinanceEngine) GetTotalBalance() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	total := 0.0
	for _, account := range e.accounts {
		total += account.Balance
	}
	return total
}

// ========== 交易管理 ==========

// AddTransaction 添加交易记录.
func (e *FinanceEngine) AddTransaction(tx *Transaction) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}

	if tx.Amount <= 0 {
		return ErrInvalidAmount
	}

	account, exists := e.accounts[tx.AccountID]
	if !exists {
		return ErrAccountNotFound
	}

	// 更新分类名称
	if cat, ok := e.categories[tx.CategoryID]; ok {
		tx.CategoryName = cat.Name
	}

	tx.CreatedAt = time.Now()
	if tx.Date.IsZero() {
		tx.Date = time.Now()
	}

	// 根据交易类型更新账户余额
	switch tx.Type {
	case TransactionTypeIncome:
		account.Balance += tx.Amount
	case TransactionTypeExpense:
		if account.Balance < tx.Amount {
			return ErrInsufficientFunds
		}
		account.Balance -= tx.Amount
	case TransactionTypeTransfer:
		if tx.ToAccountID == "" {
			return fmt.Errorf("转账交易需要目标账户")
		}
		toAccount, exists := e.accounts[tx.ToAccountID]
		if !exists {
			return ErrAccountNotFound
		}
		if account.Balance < tx.Amount {
			return ErrInsufficientFunds
		}
		account.Balance -= tx.Amount
		toAccount.Balance += tx.Amount
		toAccount.UpdatedAt = time.Now()
	}

	account.UpdatedAt = time.Now()
	e.transactions = append(e.transactions, tx)

	e.logger.Info("交易已记录",
		zap.String("id", tx.ID),
		zap.String("type", string(tx.Type)),
		zap.Float64("amount", tx.Amount))
	return nil
}

// GetTransaction 获取交易记录.
func (e *FinanceEngine) GetTransaction(txID string) (*Transaction, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, tx := range e.transactions {
		if tx.ID == txID {
			return tx, nil
		}
	}
	return nil, ErrTransactionNotFound
}

// QueryTransactions 查询交易记录.
func (e *FinanceEngine) QueryTransactions(query TransactionQuery) []*Transaction {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Transaction
	for _, tx := range e.transactions {
		if !matchTransaction(tx, query) {
			continue
		}
		result = append(result, tx)
	}

	// 简单分页
	start := (query.Page - 1) * query.PageSize
	if start >= len(result) {
		return nil
	}
	end := start + query.PageSize
	if end > len(result) {
		end = len(result)
	}
	return result[start:end]
}

// matchTransaction 匹配交易记录.
func matchTransaction(tx *Transaction, query TransactionQuery) bool {
	if query.AccountID != "" && tx.AccountID != query.AccountID {
		return false
	}
	if query.Type != "" && tx.Type != query.Type {
		return false
	}
	if query.CategoryID != "" && tx.CategoryID != query.CategoryID {
		return false
	}
	if query.StartDate != nil && tx.Date.Before(*query.StartDate) {
		return false
	}
	if query.EndDate != nil && tx.Date.After(*query.EndDate) {
		return false
	}
	if query.MinAmount != nil && tx.Amount < *query.MinAmount {
		return false
	}
	if query.MaxAmount != nil && tx.Amount > *query.MaxAmount {
		return false
	}
	return true
}

// ========== 分类管理 ==========

// GetCategories 获取所有分类.
func (e *FinanceEngine) GetCategories() []*Category {
	e.mu.RLock()
	defer e.mu.RUnlock()

	categories := make([]*Category, 0, len(e.categories))
	for _, cat := range e.categories {
		categories = append(categories, cat)
	}
	return categories
}

// AddCategory 添加自定义分类.
func (e *FinanceEngine) AddCategory(category *Category) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if category.ID == "" {
		category.ID = "cat-" + uuid.New().String()[:8]
	}

	if _, exists := e.categories[category.ID]; exists {
		return ErrCategoryExists
	}

	e.categories[category.ID] = category
	return nil
}
