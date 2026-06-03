package aitokenmeter

import (
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager AI Token 智能计量管理器.
// 集成计量、配额、预算、限流、审计日志.
type Manager struct {
	mu            sync.RWMutex
	meter         *Meter
	quotaMgr      *QuotaManager
	budgetMgr     *BudgetManager
	auditLog      *ringBuffer
	alertHandlers []AlertHandler
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	MaxUsageRecords int  // 最大用量记录数
	AuditLogSize    int  // 审计日志缓冲大小
}

// DefaultConfig 默认配置.
func DefaultConfig() ManagerConfig {
	return ManagerConfig{
		MaxUsageRecords: 10000,
		AuditLogSize:    5000,
	}
}

// NewManager 创建管理器.
func NewManager(cfg ManagerConfig) *Manager {
	m := &Manager{
		meter:    NewMeter(cfg.MaxUsageRecords),
		quotaMgr: NewQuotaManager(),
		auditLog: newRingBuffer(cfg.AuditLogSize),
	}
	m.budgetMgr = NewBudgetManager(m.handleAlert)
	return m
}

// ========== 核心计量 ==========

// RecordUsage 记录 Token 用量，执行配额检查、限流、预算扣减、审计.
func (m *Manager) RecordUsage(usage TokenUsage, limits []RateLimit) error {
	if usage.TotalTokens <= 0 {
		return ErrInvalidParams
	}
	if usage.ID == "" {
		usage.ID = uuid.New().String()
	}
	if usage.Timestamp.IsZero() {
		usage.Timestamp = time.Now()
	}

	// 1. 滑动窗口限流检查
	if err := m.meter.CheckAndRecord(usage, limits); err != nil {
		m.appendAudit(AuditLog{
			ID:        uuid.New().String(),
			Action:    AuditActionRateLimit,
			UserID:    usage.UserID,
			Details:   "rate limited for " + string(usage.Provider),
			Timestamp: time.Now(),
		})
		return err
	}

	// 2. 配额检查
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayTokens, dayCost := m.meter.GetUserUsage(usage.UserID, dayStart)
	if err := m.quotaMgr.CheckQuota(usage.UserID, usage.TotalTokens, usage.Provider, PeriodPerDay, dayTokens, dayCost); err != nil {
		m.appendAudit(AuditLog{
			ID:        uuid.New().String(),
			Action:    AuditActionQuotaHit,
			UserID:    usage.UserID,
			Details:   "daily quota exceeded",
			Timestamp: time.Now(),
		})
		return err
	}

	// 3. 预算检查
	if usage.Cost > 0 {
		userBudgets := m.budgetMgr.FindBudgetsByTarget(usage.UserID, BudgetTypeUser)
		for _, b := range userBudgets {
			if err := m.budgetMgr.Spend(b.ID, usage.Cost); err != nil {
				m.appendAudit(AuditLog{
					ID:        uuid.New().String(),
					Action:    AuditActionBudgetHit,
					UserID:    usage.UserID,
					Details:   "budget " + b.ID + " exceeded",
					Timestamp: time.Now(),
				})
				return err
			}
		}
		// 全局预算
		globalBudgets := m.budgetMgr.FindBudgetsByTarget("", BudgetTypeGlobal)
		for _, b := range globalBudgets {
			if err := m.budgetMgr.Spend(b.ID, usage.Cost); err != nil {
				m.appendAudit(AuditLog{
					ID:        uuid.New().String(),
					Action:    AuditActionBudgetHit,
					UserID:    usage.UserID,
					Details:   "global budget " + b.ID + " exceeded",
					Timestamp: time.Now(),
				})
				return err
			}
		}
	}

	// 4. 审计日志
	m.appendAudit(AuditLog{
		ID:        uuid.New().String(),
		Action:    AuditActionRecord,
		UserID:    usage.UserID,
		Details:   string(usage.Provider) + "/" + usage.Model + " tokens=" + intToStr(usage.TotalTokens),
		Timestamp: time.Now(),
	})

	return nil
}

// ========== 委托方法 ==========

// --- 配额 ---

// SetQuota 设置用户配额.
func (m *Manager) SetQuota(quota UserQuota) {
	m.quotaMgr.SetQuota(quota)
	m.appendAudit(AuditLog{
		ID:        uuid.New().String(),
		Action:    AuditActionQuotaSet,
		UserID:    quota.UserID,
		Timestamp: time.Now(),
	})
}

// GetQuota 获取用户配额.
func (m *Manager) GetQuota(userID string) (*UserQuota, bool) {
	return m.quotaMgr.GetQuota(userID)
}

// DeleteQuota 删除用户配额.
func (m *Manager) DeleteQuota(userID string) {
	m.quotaMgr.DeleteQuota(userID)
}

// ListQuotas 列出所有配额.
func (m *Manager) ListQuotas() []UserQuota {
	return m.quotaMgr.ListQuotas()
}

// --- 套餐 ---

// SetPlan 设置套餐.
func (m *Manager) SetPlan(plan Plan) {
	m.quotaMgr.SetPlan(plan)
}

// GetPlan 获取套餐.
func (m *Manager) GetPlan(planID string) (*Plan, bool) {
	return m.quotaMgr.GetPlan(planID)
}

// AssignPlan 给用户分配套餐.
func (m *Manager) AssignPlan(userID, planID string) error {
	if err := m.quotaMgr.AssignPlan(userID, planID); err != nil {
		return err
	}
	m.appendAudit(AuditLog{
		ID:        uuid.New().String(),
		Action:    AuditActionPlanSet,
		UserID:    userID,
		Details:   "plan=" + planID,
		Timestamp: time.Now(),
	})
	return nil
}

// --- 预算 ---

// SetBudget 设置预算.
func (m *Manager) SetBudget(budget Budget) {
	m.budgetMgr.SetBudget(budget)
}

// GetBudget 获取预算.
func (m *Manager) GetBudget(budgetID string) (*Budget, bool) {
	return m.budgetMgr.GetBudget(budgetID)
}

// DeleteBudget 删除预算.
func (m *Manager) DeleteBudget(budgetID string) {
	m.budgetMgr.DeleteBudget(budgetID)
}

// ListBudgets 列出所有预算.
func (m *Manager) ListBudgets() []Budget {
	return m.budgetMgr.ListBudgets()
}

// ResetBudget 重置预算.
func (m *Manager) ResetBudget(budgetID string) {
	m.budgetMgr.ResetBudget(budgetID)
}

// --- 查询 ---

// GetUserUsage 获取用户用量.
func (m *Manager) GetUserUsage(userID string, since time.Time) (tokens int, cost float64) {
	return m.meter.GetUserUsage(userID, since)
}

// GetProviderUsage 获取提供商用量.
func (m *Manager) GetProviderUsage(provider Provider, since time.Time) (tokens int, cost float64) {
	return m.meter.GetProviderUsage(provider, since)
}

// RecentUsage 最近用量.
func (m *Manager) RecentUsage(n int) []TokenUsage {
	return m.meter.RecentUsage(n)
}

// UsageCount 总记录数.
func (m *Manager) UsageCount() int {
	return m.meter.UsageCount()
}

// --- 审计 ---

// RecentAuditLogs 获取最近 N 条审计日志.
func (m *Manager) RecentAuditLogs(n int) []AuditLog {
	return m.auditLog.recent(n)
}

// AuditLogCount 审计日志总数.
func (m *Manager) AuditLogCount() int {
	return m.auditLog.count()
}

// --- 告警 ---

// AddAlertHandler 添加告警处理器.
func (m *Manager) AddAlertHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandlers = append(m.alertHandlers, handler)
}

// handleAlert 内部告警分发.
func (m *Manager) handleAlert(alert Alert) {
	m.mu.RLock()
	handlers := make([]AlertHandler, len(m.alertHandlers))
	copy(handlers, m.alertHandlers)
	m.mu.RUnlock()

	for _, h := range handlers {
		go h(alert)
	}

	m.appendAudit(AuditLog{
		ID:        uuid.New().String(),
		Action:    AuditActionAlert,
		UserID:    alert.UserID,
		Details:   string(alert.Level) + ": " + alert.Message,
		Timestamp: time.Now(),
	})
}

// ========== 内部工具 ==========

// appendAudit 添加审计日志.
func (m *Manager) appendAudit(log AuditLog) {
	m.auditLog.append(log)
}

// newRingBuffer 创建环形缓冲区.
func newRingBuffer(size int) *ringBuffer {
	if size <= 0 {
		size = 1000
	}
	return &ringBuffer{
		buf:  make([]AuditLog, size),
		size: size,
	}
}

// append 添加日志到环形缓冲区.
func (rb *ringBuffer) append(log AuditLog) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buf[rb.head] = log
	rb.head = (rb.head + 1) % rb.size
	if rb.cnt < rb.size {
		rb.cnt++
	}
}

// recent 获取最近 n 条日志.
func (rb *ringBuffer) recent(n int) []AuditLog {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if n <= 0 || n > rb.cnt {
		n = rb.cnt
	}

	result := make([]AuditLog, n)
	start := (rb.head - n + rb.size) % rb.size
	for i := 0; i < n; i++ {
		result[i] = rb.buf[(start+i)%rb.size]
	}
	return result
}

// count 返回审计日志总数（公开）.
func (rb *ringBuffer) count() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.cnt
}

// intToStr 简易 int 转 string.
func intToStr(n int) string {
	return strconv.Itoa(n)
}
