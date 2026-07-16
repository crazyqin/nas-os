// Package budgetmgr 提供企业级存储预算管理功能。
// 支持预算周期管理、部门/项目级预算分配、审批流程、超支告警等。
package budgetmgr

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrBudgetNotFound 预算不存在.
	ErrBudgetNotFound = errors.New("预算不存在")
	// ErrRequestNotFound 申请不存在.
	ErrRequestNotFound = errors.New("申请不存在")
	// ErrTemplateNotFound 模板不存在.
	ErrTemplateNotFound = errors.New("模板不存在")
	// ErrInvalidParams 无效输入参数.
	ErrInvalidParams = errors.New("无效输入参数")
	// ErrInvalidTransition 无效状态转换.
	ErrInvalidTransition = errors.New("无效的状态转换")
	// ErrAlreadyProcessed 已处理.
	ErrAlreadyProcessed = errors.New("申请已处理")
)

// ========== 预算周期 ==========

// BudgetPeriod 预算周期类型.
type BudgetPeriod string

const (
	// PeriodMonthly 月度预算.
	PeriodMonthly BudgetPeriod = "monthly"
	// PeriodQuarterly 季度预算.
	PeriodQuarterly BudgetPeriod = "quarterly"
	// PeriodYearly 年度预算.
	PeriodYearly BudgetPeriod = "yearly"
)

// ========== 审批状态 ==========

// ApprovalStatus 审批状态.
type ApprovalStatus string

const (
	// StatusPending 待审批.
	StatusPending ApprovalStatus = "pending"
	// StatusApproved 已批准.
	StatusApproved ApprovalStatus = "approved"
	// StatusRejected 已拒绝.
	StatusRejected ApprovalStatus = "rejected"
	// StatusAllocated 已分配.
	StatusAllocated ApprovalStatus = "allocated"
)

// ========== 降级策略 ==========

// DegradationStrategy 降级策略.
type DegradationStrategy string

const (
	// DegradationNone 不降级.
	DegradationNone DegradationStrategy = "none"
	// DegradationThrottle 限流.
	DegradationThrottle DegradationStrategy = "throttle"
	// DegradationReject 拒绝新写入.
	DegradationReject DegradationStrategy = "reject"
	// DegradationArchive 自动归档.
	DegradationArchive DegradationStrategy = "archive"
)

// ========== 核心数据结构 ==========

// Budget 预算定义.
type Budget struct {
	// ID 预算ID.
	ID string `json:"id"`
	// Name 预算名称.
	Name string `json:"name"`
	// Department 所属部门.
	Department string `json:"department"`
	// Project 所属项目（可为空，表示部门级预算）.
	Project string `json:"project"`
	// Period 预算周期.
	Period BudgetPeriod `json:"period"`
	// TotalAmount 预算总额.
	TotalAmount float64 `json:"total_amount"`
	// UsedAmount 已使用金额.
	UsedAmount float64 `json:"used_amount"`
	// Currency 币种.
	Currency string `json:"currency"`
	// StartDate 周期开始日期.
	StartDate time.Time `json:"start_date"`
	// EndDate 周期结束日期.
	EndDate time.Time `json:"end_date"`
	// AlertThreshold 告警阈值（0-1之间的比例）.
	AlertThreshold float64 `json:"alert_threshold"`
	// DegradationStrategy 超支降级策略.
	DegradationStrategy DegradationStrategy `json:"degradation_strategy"`
	// IsActive 是否激活.
	IsActive bool `json:"is_active"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// BudgetRequest 预算申请.
type BudgetRequest struct {
	// ID 申请ID.
	ID string `json:"id"`
	// BudgetID 关联预算ID.
	BudgetID string `json:"budget_id"`
	// Department 申请部门.
	Department string `json:"department"`
	// Project 申请项目.
	Project string `json:"project"`
	// Amount 申请金额.
	Amount float64 `json:"amount"`
	// Reason 申请理由.
	Reason string `json:"reason"`
	// Status 审批状态.
	Status ApprovalStatus `json:"status"`
	// Approver 审批人.
	Approver string `json:"approver"`
	// ApprovalNote 审批备注.
	ApprovalNote string `json:"approval_note"`
	// RequestedAt 申请时间.
	RequestedAt time.Time `json:"requested_at"`
	// ProcessedAt 处理时间.
	ProcessedAt time.Time `json:"processed_at"`
}

// BudgetUtilizationReport 预算利用率报告.
type BudgetUtilizationReport struct {
	// BudgetID 预算ID.
	BudgetID string `json:"budget_id"`
	// BudgetName 预算名称.
	BudgetName string `json:"budget_name"`
	// Department 部门.
	Department string `json:"department"`
	// TotalAmount 预算总额.
	TotalAmount float64 `json:"total_amount"`
	// UsedAmount 已使用金额.
	UsedAmount float64 `json:"used_amount"`
	// UtilizationRate 使用率（0-1）.
	UtilizationRate float64 `json:"utilization_rate"`
	// RemainingAmount 剩余金额.
	RemainingAmount float64 `json:"remaining_amount"`
	// Status 状态描述.
	Status string `json:"status"`
	// IsOverBudget 是否超支.
	IsOverBudget bool `json:"is_over_budget"`
}

// BudgetComparison 历史预算对比.
type BudgetComparison struct {
	// Department 部门.
	Department string `json:"department"`
	// Periods 各周期的预算数据.
	Periods []PeriodData `json:"periods"`
	// AvgUtilization 平均使用率.
	AvgUtilization float64 `json:"avg_utilization"`
	// Trend 趋势: increasing/decreasing/stable.
	Trend string `json:"trend"`
}

// PeriodData 单个周期数据.
type PeriodData struct {
	// Period 周期标识.
	Period string `json:"period"`
	// BudgetAmount 预算金额.
	BudgetAmount float64 `json:"budget_amount"`
	// UsedAmount 使用金额.
	UsedAmount float64 `json:"used_amount"`
	// UtilizationRate 使用率.
	UtilizationRate float64 `json:"utilization_rate"`
}

// BudgetTemplate 预算模板.
type BudgetTemplate struct {
	// ID 模板ID.
	ID string `json:"id"`
	// Name 模板名称.
	Name string `json:"name"`
	// Period 默认周期.
	Period BudgetPeriod `json:"period"`
	// DefaultAmount 默认金额.
	DefaultAmount float64 `json:"default_amount"`
	// AlertThreshold 默认告警阈值.
	AlertThreshold float64 `json:"alert_threshold"`
	// DegradationStrategy 默认降级策略.
	DegradationStrategy DegradationStrategy `json:"degradation_strategy"`
	// Description 描述.
	Description string `json:"description"`
}

// ========== 预算管理器 ==========

// Manager 预算管理器.
type Manager struct {
	mu        sync.RWMutex
	budgets   map[string]*Budget
	requests  map[string]*BudgetRequest
	templates map[string]*BudgetTemplate
}

// NewManager 创建预算管理器.
func NewManager() *Manager {
	return &Manager{
		budgets:   make(map[string]*Budget),
		requests:  make(map[string]*BudgetRequest),
		templates: make(map[string]*BudgetTemplate),
	}
}

// ========== 预算 CRUD ==========

// CreateBudget 创建预算.
func (m *Manager) CreateBudget(b *Budget) error {
	if b.ID == "" || b.Name == "" || b.Department == "" {
		return ErrInvalidParams
	}
	if b.TotalAmount <= 0 {
		return ErrInvalidParams
	}
	if b.AlertThreshold <= 0 || b.AlertThreshold > 1 {
		b.AlertThreshold = 0.8
	}
	if b.DegradationStrategy == "" {
		b.DegradationStrategy = DegradationNone
	}
	if b.Currency == "" {
		b.Currency = "CNY"
	}
	now := time.Now()
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	b.UpdatedAt = now
	b.IsActive = true

	m.mu.Lock()
	m.budgets[b.ID] = b
	m.mu.Unlock()
	return nil
}

// GetBudget 获取预算.
func (m *Manager) GetBudget(id string) (*Budget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.budgets[id]
	if !ok {
		return nil, ErrBudgetNotFound
	}
	return b, nil
}

// ListBudgets 列出所有预算.
func (m *Manager) ListBudgets() []*Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Budget, 0, len(m.budgets))
	for _, b := range m.budgets {
		result = append(result, b)
	}
	return result
}

// ListBudgetsByDepartment 按部门列出预算.
func (m *Manager) ListBudgetsByDepartment(dept string) []*Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Budget
	for _, b := range m.budgets {
		if b.Department == dept {
			result = append(result, b)
		}
	}
	return result
}

// UpdateBudget 更新预算.
func (m *Manager) UpdateBudget(id string, updates *Budget) (*Budget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.budgets[id]
	if !ok {
		return nil, ErrBudgetNotFound
	}
	if updates.Name != "" {
		b.Name = updates.Name
	}
	if updates.TotalAmount > 0 {
		b.TotalAmount = updates.TotalAmount
	}
	if updates.AlertThreshold > 0 && updates.AlertThreshold <= 1 {
		b.AlertThreshold = updates.AlertThreshold
	}
	if updates.DegradationStrategy != "" {
		b.DegradationStrategy = updates.DegradationStrategy
	}
	b.UpdatedAt = time.Now()
	return b, nil
}

// DeleteBudget 删除预算.
func (m *Manager) DeleteBudget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.budgets[id]; !ok {
		return ErrBudgetNotFound
	}
	delete(m.budgets, id)
	return nil
}

// ========== 使用率追踪 ==========

// RecordUsage 记录预算使用.
func (m *Manager) RecordUsage(budgetID string, amount float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.budgets[budgetID]
	if !ok {
		return ErrBudgetNotFound
	}
	b.UsedAmount += amount
	b.UpdatedAt = time.Now()
	return nil
}

// GetUtilization 获取预算使用率.
func (m *Manager) GetUtilization(budgetID string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.budgets[budgetID]
	if !ok {
		return 0, ErrBudgetNotFound
	}
	if b.TotalAmount == 0 {
		return 0, nil
	}
	return b.UsedAmount / b.TotalAmount, nil
}

// GetUtilizationReport 获取利用率报告.
func (m *Manager) GetUtilizationReport(budgetID string) (*BudgetUtilizationReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.budgets[budgetID]
	if !ok {
		return nil, ErrBudgetNotFound
	}
	remaining := b.TotalAmount - b.UsedAmount
	rate := 0.0
	if b.TotalAmount > 0 {
		rate = b.UsedAmount / b.TotalAmount
	}
	status := "正常"
	if rate >= 1 {
		status = "超支"
	} else if rate >= b.AlertThreshold {
		status = "接近超支"
	}
	return &BudgetUtilizationReport{
		BudgetID:        b.ID,
		BudgetName:      b.Name,
		Department:      b.Department,
		TotalAmount:     b.TotalAmount,
		UsedAmount:      b.UsedAmount,
		UtilizationRate: rate,
		RemainingAmount: remaining,
		Status:          status,
		IsOverBudget:    rate >= 1,
	}, nil
}

// GetAllUtilizationReports 获取所有预算的利用率报告.
func (m *Manager) GetAllUtilizationReports() []*BudgetUtilizationReport {
	m.mu.RLock()
	budgets := make([]*Budget, 0, len(m.budgets))
	for _, b := range m.budgets {
		budgets = append(budgets, b)
	}
	m.mu.RUnlock()

	reports := make([]*BudgetUtilizationReport, 0, len(budgets))
	for _, b := range budgets {
		remaining := b.TotalAmount - b.UsedAmount
		rate := 0.0
		if b.TotalAmount > 0 {
			rate = b.UsedAmount / b.TotalAmount
		}
		status := "正常"
		if rate >= 1 {
			status = "超支"
		} else if rate >= b.AlertThreshold {
			status = "接近超支"
		}
		reports = append(reports, &BudgetUtilizationReport{
			BudgetID:        b.ID,
			BudgetName:      b.Name,
			Department:      b.Department,
			TotalAmount:     b.TotalAmount,
			UsedAmount:      b.UsedAmount,
			UtilizationRate: rate,
			RemainingAmount: remaining,
			Status:          status,
			IsOverBudget:    rate >= 1,
		})
	}
	return reports
}

// ========== 审批流程 ==========

// CreateRequest 创建预算申请.
func (m *Manager) CreateRequest(req *BudgetRequest) error {
	if req.ID == "" || req.Department == "" || req.Amount <= 0 {
		return ErrInvalidParams
	}
	req.Status = StatusPending
	if req.RequestedAt.IsZero() {
		req.RequestedAt = time.Now()
	}

	m.mu.Lock()
	m.requests[req.ID] = req
	m.mu.Unlock()
	return nil
}

// GetRequest 获取申请.
func (m *Manager) GetRequest(id string) (*BudgetRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	req, ok := m.requests[id]
	if !ok {
		return nil, ErrRequestNotFound
	}
	return req, nil
}

// ListRequests 列出所有申请.
func (m *Manager) ListRequests() []*BudgetRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*BudgetRequest, 0, len(m.requests))
	for _, r := range m.requests {
		result = append(result, r)
	}
	return result
}

// ListPendingRequests 列出待审批申请.
func (m *Manager) ListPendingRequests() []*BudgetRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*BudgetRequest
	for _, r := range m.requests {
		if r.Status == StatusPending {
			result = append(result, r)
		}
	}
	return result
}

// ApproveRequest 批准申请.
func (m *Manager) ApproveRequest(requestID, approver, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[requestID]
	if !ok {
		return ErrRequestNotFound
	}
	if req.Status != StatusPending {
		return ErrAlreadyProcessed
	}
	req.Status = StatusApproved
	req.Approver = approver
	req.ApprovalNote = note
	req.ProcessedAt = time.Now()
	return nil
}

// RejectRequest 拒绝申请.
func (m *Manager) RejectRequest(requestID, approver, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[requestID]
	if !ok {
		return ErrRequestNotFound
	}
	if req.Status != StatusPending {
		return ErrAlreadyProcessed
	}
	req.Status = StatusRejected
	req.Approver = approver
	req.ApprovalNote = note
	req.ProcessedAt = time.Now()
	return nil
}

// AllocateRequest 分配已批准的申请（增加预算额度）.
func (m *Manager) AllocateRequest(requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[requestID]
	if !ok {
		return ErrRequestNotFound
	}
	if req.Status != StatusApproved {
		return ErrInvalidTransition
	}

	// 如果关联了预算，增加预算额度
	if req.BudgetID != "" {
		b, ok := m.budgets[req.BudgetID]
		if ok {
			b.TotalAmount += req.Amount
			b.UpdatedAt = time.Now()
		}
	}

	req.Status = StatusAllocated
	return nil
}

// ========== 超支告警和降级 ==========

// CheckOverBudget 检查超支告警.
func (m *Manager) CheckOverBudget() []*Budget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var alerts []*Budget
	for _, b := range m.budgets {
		if !b.IsActive {
			continue
		}
		if b.TotalAmount > 0 && b.UsedAmount/b.TotalAmount >= b.AlertThreshold {
			alerts = append(alerts, b)
		}
	}
	return alerts
}

// GetDegradationActions 获取需要降级处理的预算.
func (m *Manager) GetDegradationActions() map[string]DegradationStrategy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	actions := make(map[string]DegradationStrategy)
	for _, b := range m.budgets {
		if !b.IsActive || b.DegradationStrategy == DegradationNone {
			continue
		}
		if b.TotalAmount > 0 && b.UsedAmount >= b.TotalAmount {
			actions[b.ID] = b.DegradationStrategy
		}
	}
	return actions
}

// ========== 历史对比分析 ==========

// CompareBudgets 历史预算对比分析.
// 按部门分组，对比各周期使用率趋势.
func (m *Manager) CompareBudgets(department string) *BudgetComparison {
	m.mu.RLock()
	budgets := make([]*Budget, 0)
	for _, b := range m.budgets {
		if b.Department == department {
			budgets = append(budgets, b)
		}
	}
	m.mu.RUnlock()

	if len(budgets) == 0 {
		return &BudgetComparison{Department: department}
	}

	// 按开始日期排序
	sort.Slice(budgets, func(i, j int) bool {
		return budgets[i].StartDate.Before(budgets[j].StartDate)
	})

	comp := &BudgetComparison{
		Department: department,
		Periods:    make([]PeriodData, 0, len(budgets)),
	}

	var totalUtil float64
	for _, b := range budgets {
		rate := 0.0
		if b.TotalAmount > 0 {
			rate = b.UsedAmount / b.TotalAmount
		}
		comp.Periods = append(comp.Periods, PeriodData{
			Period:          fmt.Sprintf("%s~%s", b.StartDate.Format("2006-01"), b.EndDate.Format("2006-01")),
			BudgetAmount:    b.TotalAmount,
			UsedAmount:      b.UsedAmount,
			UtilizationRate: rate,
		})
		totalUtil += rate
	}

	comp.AvgUtilization = totalUtil / float64(len(budgets))

	// 判断趋势
	if len(comp.Periods) >= 2 {
		last := comp.Periods[len(comp.Periods)-1].UtilizationRate
		prev := comp.Periods[len(comp.Periods)-2].UtilizationRate
		if last > prev*1.05 {
			comp.Trend = "increasing"
		} else if last < prev*0.95 {
			comp.Trend = "decreasing"
		} else {
			comp.Trend = "stable"
		}
	} else {
		comp.Trend = "stable"
	}

	return comp
}

// ========== 预算模板 ==========

// CreateTemplate 创建预算模板.
func (m *Manager) CreateTemplate(t *BudgetTemplate) error {
	if t.ID == "" || t.Name == "" {
		return ErrInvalidParams
	}
	if t.AlertThreshold <= 0 || t.AlertThreshold > 1 {
		t.AlertThreshold = 0.8
	}
	if t.DegradationStrategy == "" {
		t.DegradationStrategy = DegradationNone
	}

	m.mu.Lock()
	m.templates[t.ID] = t
	m.mu.Unlock()
	return nil
}

// GetTemplate 获取模板.
func (m *Manager) GetTemplate(id string) (*BudgetTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return t, nil
}

// ListTemplates 列出所有模板.
func (m *Manager) ListTemplates() []*BudgetTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*BudgetTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, t)
	}
	return result
}

// DeleteTemplate 删除模板.
func (m *Manager) DeleteTemplate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.templates[id]; !ok {
		return ErrTemplateNotFound
	}
	delete(m.templates, id)
	return nil
}

// CreateBudgetFromTemplate 从模板创建预算.
func (m *Manager) CreateBudgetFromTemplate(templateID, budgetID, name, department string, startDate time.Time) (*Budget, error) {
	m.mu.RLock()
	tmpl, ok := m.templates[templateID]
	if !ok {
		m.mu.RUnlock()
		return nil, ErrTemplateNotFound
	}
	// 复制模板值
	period := tmpl.Period
	amount := tmpl.DefaultAmount
	threshold := tmpl.AlertThreshold
	strategy := tmpl.DegradationStrategy
	m.mu.RUnlock()

	var endDate time.Time
	switch period {
	case PeriodMonthly:
		endDate = startDate.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case PeriodQuarterly:
		endDate = startDate.AddDate(0, 3, 0).Add(-time.Nanosecond)
	case PeriodYearly:
		endDate = startDate.AddDate(1, 0, 0).Add(-time.Nanosecond)
	default:
		return nil, ErrInvalidParams
	}

	b := &Budget{
		ID:                  budgetID,
		Name:                name,
		Department:          department,
		Period:              period,
		TotalAmount:         amount,
		Currency:            "CNY",
		StartDate:           startDate,
		EndDate:             endDate,
		AlertThreshold:      threshold,
		DegradationStrategy: strategy,
	}
	if err := m.CreateBudget(b); err != nil {
		return nil, err
	}
	return b, nil
}
