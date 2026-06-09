package opsrunbook

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 运维手册管理器
type Manager struct {
	mu          sync.RWMutex
	runbooks    map[string]*Runbook
	executions  map[string]*Execution
	approvals   map[string]*ApprovalRequest
	execStats   map[string]*ExecutionStats
	store       Store
	executor    *Executor
	logger      *zap.Logger
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
	eventChan   chan *ExecutionEvent
}

// ExecutionEvent 执行事件
type ExecutionEvent struct {
	Type        string      `json:"type"` // started, step_completed, completed, failed, rollback
	ExecutionID string      `json:"execution_id"`
	StepID      string      `json:"step_id,omitempty"`
	Status      StepStatus  `json:"status"`
	Message     string      `json:"message"`
	Timestamp   time.Time   `json:"timestamp"`
}

// Config 管理器配置
type Config struct {
	MaxExecutions   int           `json:"max_executions"`
	ExecutionTTL    time.Duration `json:"execution_ttl"`
	MaxConcurrent   int           `json:"max_concurrent"`
	ApprovalTimeout time.Duration `json:"approval_timeout"`
	AuditEnabled    bool          `json:"audit_enabled"`
}

// Store 持久化存储接口
type Store interface {
	SaveRunbook(rb *Runbook) error
	LoadRunbook(id string) (*Runbook, error)
	DeleteRunbook(id string) error
	ListRunbooks() ([]*Runbook, error)
	SaveExecution(exec *Execution) error
	LoadExecution(id string) (*Execution, error)
	ListExecutions(filter ExecutionFilter) ([]*Execution, error)
	SaveApproval(req *ApprovalRequest) error
	LoadApproval(id string) (*ApprovalRequest, error)
}

// NewManager 创建运维手册管理器
func NewManager(store Store, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		runbooks:   make(map[string]*Runbook),
		executions: make(map[string]*Execution),
		approvals:  make(map[string]*ApprovalRequest),
		execStats:  make(map[string]*ExecutionStats),
		store:      store,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		eventChan:  make(chan *ExecutionEvent, 100),
	}
}

// RegisterRunbook 注册运维手册
func (m *Manager) RegisterRunbook(rb *Runbook) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rb.ID == "" {
		return fmt.Errorf("runbook ID is required")
	}
	if rb.Name == "" {
		return fmt.Errorf("runbook name is required")
	}
	if len(rb.Steps) == 0 {
		return fmt.Errorf("runbook must have at least one step")
	}

	// 验证步骤依赖
	if err := m.validateSteps(rb.Steps); err != nil {
		return fmt.Errorf("invalid steps: %w", err)
	}

	now := time.Now()
	if rb.CreatedAt.IsZero() {
		rb.CreatedAt = now
	}
	rb.UpdatedAt = now
	if rb.Version == 0 {
		rb.Version = 1
	}
	if rb.Status == "" {
		rb.Status = StatusActive
	}

	m.runbooks[rb.ID] = rb
	m.execStats[rb.ID] = &ExecutionStats{}

	if m.store != nil {
		if err := m.store.SaveRunbook(rb); err != nil {
			m.logger.Error("failed to save runbook", zap.String("id", rb.ID), zap.Error(err))
		}
	}

	m.logger.Info("registered runbook",
		zap.String("id", rb.ID),
		zap.String("name", rb.Name),
		zap.Int("steps", len(rb.Steps)),
	)
	return nil
}

// UpdateRunbook 更新运维手册
func (m *Manager) UpdateRunbook(rb *Runbook) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.runbooks[rb.ID]
	if !ok {
		return fmt.Errorf("runbook %s not found", rb.ID)
	}

	if err := m.validateSteps(rb.Steps); err != nil {
		return fmt.Errorf("invalid steps: %w", err)
	}

	rb.Version = existing.Version + 1
	rb.CreatedAt = existing.CreatedAt
	rb.UpdatedAt = time.Now()
	rb.RunCount = existing.RunCount
	rb.SuccessRate = existing.SuccessRate

	m.runbooks[rb.ID] = rb

	if m.store != nil {
		if err := m.store.SaveRunbook(rb); err != nil {
			m.logger.Error("failed to update runbook", zap.String("id", rb.ID), zap.Error(err))
		}
	}

	m.logger.Info("updated runbook", zap.String("id", rb.ID), zap.Int("version", rb.Version))
	return nil
}

// DeleteRunbook 删除运维手册
func (m *Manager) DeleteRunbook(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.runbooks[id]; !ok {
		return fmt.Errorf("runbook %s not found", id)
	}

	delete(m.runbooks, id)
	delete(m.execStats, id)

	if m.store != nil {
		if err := m.store.DeleteRunbook(id); err != nil {
			m.logger.Error("failed to delete runbook", zap.String("id", id), zap.Error(err))
		}
	}

	m.logger.Info("deleted runbook", zap.String("id", id))
	return nil
}

// GetRunbook 获取运维手册
func (m *Manager) GetRunbook(id string) (*Runbook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rb, ok := m.runbooks[id]
	if !ok {
		return nil, fmt.Errorf("runbook %s not found", id)
	}
	return rb, nil
}

// ListRunbooks 列出运维手册
func (m *Manager) ListRunbooks(filter RunbookFilter) []*Runbook {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Runbook
	for _, rb := range m.runbooks {
		if m.matchFilter(rb, filter) {
			result = append(result, rb)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}

// GetExecutionStats 获取执行统计
func (m *Manager) GetExecutionStats(runbookID string) (*ExecutionStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, ok := m.execStats[runbookID]
	if !ok {
		return nil, fmt.Errorf("runbook %s not found", runbookID)
	}
	return stats, nil
}

// GetExecution 获取执行记录
func (m *Manager) GetExecution(id string) (*Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exec, ok := m.executions[id]
	if !ok {
		return nil, fmt.Errorf("execution %s not found", id)
	}
	return exec, nil
}

// ListExecutions 列出执行记录
func (m *Manager) ListExecutions(filter ExecutionFilter) []*Execution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Execution
	for _, exec := range m.executions {
		if m.matchExecFilter(exec, filter) {
			result = append(result, exec)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})

	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result
}

// Approve 审批步骤
func (m *Manager) Approve(approvalID, approvedBy, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.approvals[approvalID]
	if !ok {
		return fmt.Errorf("approval request %s not found", approvalID)
	}

	now := time.Now()
	req.ApprovedBy = approvedBy
	req.ApprovedAt = &now
	req.Reason = reason

	m.logger.Info("approval granted",
		zap.String("approval_id", approvalID),
		zap.String("by", approvedBy),
	)

	return nil
}

// Reject 拒绝审批
func (m *Manager) Reject(approvalID, rejectedBy, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.approvals[approvalID]
	if !ok {
		return fmt.Errorf("approval request %s not found", approvalID)
	}

	req.Rejected = true
	req.ApprovedBy = rejectedBy
	req.Reason = reason

	m.logger.Info("approval rejected",
		zap.String("approval_id", approvalID),
		zap.String("by", rejectedBy),
	)

	return nil
}

// Subscribe 订阅执行事件
func (m *Manager) Subscribe() <-chan *ExecutionEvent {
	return m.eventChan
}

// Close 关闭管理器
func (m *Manager) Close() {
	m.cancel()
	close(m.eventChan)
}

// validateSteps 验证步骤依赖
func (m *Manager) validateSteps(steps []*Step) error {
	stepIDs := make(map[string]bool)
	for _, s := range steps {
		if s.ID == "" {
			return fmt.Errorf("step ID is required")
		}
		if stepIDs[s.ID] {
			return fmt.Errorf("duplicate step ID: %s", s.ID)
		}
		stepIDs[s.ID] = true
	}

	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if !stepIDs[dep] {
				return fmt.Errorf("step %s depends on unknown step %s", s.ID, dep)
			}
		}
	}

	return nil
}

// matchFilter 匹配运维手册过滤器
func (m *Manager) matchFilter(rb *Runbook, filter RunbookFilter) bool {
	if filter.Category != "" && rb.Category != filter.Category {
		return false
	}
	if filter.Severity != "" && rb.Severity != filter.Severity {
		return false
	}
	if filter.Status != "" && rb.Status != filter.Status {
		return false
	}
	if filter.Trigger != "" && rb.Trigger != filter.Trigger {
		return false
	}
	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(rb.Name), search) &&
			!strings.Contains(strings.ToLower(rb.Description), search) {
			return false
		}
	}
	if len(filter.Tags) > 0 {
		tagSet := make(map[string]bool)
		for _, t := range rb.Tags {
			tagSet[t] = true
		}
		for _, ft := range filter.Tags {
			if !tagSet[ft] {
				return false
			}
		}
	}
	return true
}

// matchExecFilter 匹配执行记录过滤器
func (m *Manager) matchExecFilter(exec *Execution, filter ExecutionFilter) bool {
	if filter.RunbookID != "" && exec.RunbookID != filter.RunbookID {
		return false
	}
	if filter.Status != "" && exec.Status != filter.Status {
		return false
	}
	if filter.Trigger != "" && exec.Trigger != filter.Trigger {
		return false
	}
	if filter.Since != nil && exec.StartedAt.Before(*filter.Since) {
		return false
	}
	if filter.Until != nil && exec.StartedAt.After(*filter.Until) {
		return false
	}
	return true
}
