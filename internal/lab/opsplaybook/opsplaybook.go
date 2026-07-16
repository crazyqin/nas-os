// Package opsplaybook implements an operations playbook engine for standardized
// runbook management, one-click task execution, approval workflows, and SLA tracking.
package opsplaybook

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Severity represents the severity level of a playbook or task.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// PlaybookStatus represents the lifecycle status of a playbook.
type PlaybookStatus string

const (
	StatusDraft     PlaybookStatus = "draft"
	StatusPublished PlaybookStatus = "published"
	StatusArchived  PlaybookStatus = "archived"
)

// ExecutionStatus represents the status of a playbook execution.
type ExecutionStatus string

const (
	ExecPending   ExecutionStatus = "pending"
	ExecApproved  ExecutionStatus = "approved"
	ExecRunning   ExecutionStatus = "running"
	ExecSuccess   ExecutionStatus = "success"
	ExecFailed    ExecutionStatus = "failed"
	ExecCancelled ExecutionStatus = "cancelled"
	ExecRejected  ExecutionStatus = "rejected"
)

// StepType represents the type of a playbook step.
type StepType string

const (
	StepCommand  StepType = "command"
	StepScript   StepType = "script"
	StepHTTP     StepType = "http"
	StepCheck    StepType = "check"
	StepApproval StepType = "approval"
	StepNotify   StepType = "notify"
	StepWait     StepType = "wait"
)

// Playbook represents an operations playbook template.
type Playbook struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Category         string         `json:"category"`
	Severity         Severity       `json:"severity"`
	Status           PlaybookStatus `json:"status"`
	Tags             []string       `json:"tags,omitempty"`
	Steps            []Step         `json:"steps"`
	RequiresApproval bool           `json:"requires_approval"`
	SLATargetMinutes int            `json:"sla_target_minutes"`
	CreatedBy        string         `json:"created_by"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	Version          int            `json:"version"`
	RunCount         int            `json:"run_count"`
	SuccessRate      float64        `json:"success_rate"`
}

// Step represents a single step in a playbook.
type Step struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        StepType          `json:"type"`
	Description string            `json:"description,omitempty"`
	Command     string            `json:"command,omitempty"`
	Script      string            `json:"script,omitempty"`
	URL         string            `json:"url,omitempty"`
	Method      string            `json:"method,omitempty"`
	Timeout     int               `json:"timeout_seconds,omitempty"`
	RetryCount  int               `json:"retry_count,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	Condition   string            `json:"condition,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	OnFailure   string            `json:"on_failure,omitempty"` // abort, continue, rollback
}

// Execution represents a single execution of a playbook.
type Execution struct {
	ID          string            `json:"id"`
	PlaybookID  string            `json:"playbook_id"`
	Status      ExecutionStatus   `json:"status"`
	TriggeredBy string            `json:"triggered_by"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	DurationMs  int64             `json:"duration_ms"`
	StepResults []StepResult      `json:"step_results"`
	Approval    *ApprovalRecord   `json:"approval,omitempty"`
	Error       string            `json:"error,omitempty"`
	Context     map[string]string `json:"context,omitempty"`
}

// StepResult represents the result of executing a single step.
type StepResult struct {
	StepID     string          `json:"step_id"`
	Status     ExecutionStatus `json:"status"`
	Output     string          `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
	StartedAt  time.Time       `json:"started_at"`
	EndAt      time.Time       `json:"end_at"`
	DurationMs int64           `json:"duration_ms"`
	Retries    int             `json:"retries"`
}

// ApprovalRecord represents an approval request and its outcome.
type ApprovalRecord struct {
	RequestedAt time.Time  `json:"requested_at"`
	RequestedBy string     `json:"requested_by"`
	Approver    string     `json:"approver,omitempty"`
	Approved    bool       `json:"approved"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
	Comment     string     `json:"comment,omitempty"`
}

// SLAReport represents an SLA compliance report.
type SLAReport struct {
	PlaybookID       string    `json:"playbook_id"`
	PlaybookName     string    `json:"playbook_name"`
	TotalExecutions  int       `json:"total_executions"`
	SLAMet           int       `json:"sla_met"`
	SLAMissed        int       `json:"sla_missed"`
	SLACompliancePct float64   `json:"sla_compliance_pct"`
	AvgDurationMs    int64     `json:"avg_duration_ms"`
	P95DurationMs    int64     `json:"p95_duration_ms"`
	GeneratedAt      time.Time `json:"generated_at"`
}

// KnowledgeEntry represents a knowledge base article linked to a playbook.
type KnowledgeEntry struct {
	ID         string    `json:"id"`
	PlaybookID string    `json:"playbook_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Category   string    `json:"category"`
	Tags       []string  `json:"tags,omitempty"`
	Author     string    `json:"author"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Helpful    int       `json:"helpful_count"`
}

// Manager manages playbooks, executions, and knowledge base.
type Manager struct {
	mu          sync.RWMutex
	playbooks   map[string]*Playbook
	executions  map[string]*Execution
	knowledge   map[string]*KnowledgeEntry
	execCounter int64
}

// NewManager creates a new OpsPlaybook manager.
func NewManager() *Manager {
	return &Manager{
		playbooks:  make(map[string]*Playbook),
		executions: make(map[string]*Execution),
		knowledge:  make(map[string]*KnowledgeEntry),
	}
}

// CreatePlaybook creates a new playbook.
func (m *Manager) CreatePlaybook(ctx context.Context, pb *Playbook) error {
	if pb.Name == "" {
		return fmt.Errorf("playbook name is required")
	}
	if len(pb.Steps) == 0 {
		return fmt.Errorf("playbook must have at least one step")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.playbooks[pb.ID]; exists {
		return fmt.Errorf("playbook %s already exists", pb.ID)
	}

	pb.Status = StatusDraft
	pb.CreatedAt = time.Now()
	pb.UpdatedAt = time.Now()
	pb.Version = 1
	pb.RunCount = 0
	pb.SuccessRate = 0

	m.playbooks[pb.ID] = pb
	return nil
}

// UpdatePlaybook updates an existing playbook, incrementing its version.
func (m *Manager) UpdatePlaybook(ctx context.Context, pb *Playbook) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.playbooks[pb.ID]
	if !ok {
		return fmt.Errorf("playbook %s not found", pb.ID)
	}

	pb.Version = existing.Version + 1
	pb.CreatedAt = existing.CreatedAt
	pb.RunCount = existing.RunCount
	pb.SuccessRate = existing.SuccessRate
	pb.UpdatedAt = time.Now()

	m.playbooks[pb.ID] = pb
	return nil
}

// PublishPlaybook transitions a playbook from draft to published.
func (m *Manager) PublishPlaybook(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pb, ok := m.playbooks[id]
	if !ok {
		return fmt.Errorf("playbook %s not found", id)
	}
	if pb.Status != StatusDraft {
		return fmt.Errorf("only draft playbooks can be published, current status: %s", pb.Status)
	}

	pb.Status = StatusPublished
	pb.UpdatedAt = time.Now()
	return nil
}

// ArchivePlaybook archives a playbook.
func (m *Manager) ArchivePlaybook(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pb, ok := m.playbooks[id]
	if !ok {
		return fmt.Errorf("playbook %s not found", id)
	}

	pb.Status = StatusArchived
	pb.UpdatedAt = time.Now()
	return nil
}

// GetPlaybook retrieves a playbook by ID.
func (m *Manager) GetPlaybook(ctx context.Context, id string) (*Playbook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pb, ok := m.playbooks[id]
	if !ok {
		return nil, fmt.Errorf("playbook %s not found", id)
	}
	return pb, nil
}

// ListPlaybooks lists all playbooks, optionally filtered by category and status.
func (m *Manager) ListPlaybooks(ctx context.Context, category string, status PlaybookStatus) []*Playbook {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Playbook
	for _, pb := range m.playbooks {
		if category != "" && pb.Category != category {
			continue
		}
		if status != "" && pb.Status != status {
			continue
		}
		result = append(result, pb)
	}
	return result
}

// DeletePlaybook removes a playbook by ID.
func (m *Manager) DeletePlaybook(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.playbooks[id]; !ok {
		return fmt.Errorf("playbook %s not found", id)
	}

	delete(m.playbooks, id)
	return nil
}

// ExecutePlaybook starts a new execution of a playbook.
func (m *Manager) ExecutePlaybook(ctx context.Context, playbookID, triggeredBy string, vars map[string]string) (*Execution, error) {
	m.mu.Lock()

	pb, ok := m.playbooks[playbookID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("playbook %s not found", playbookID)
	}
	if pb.Status != StatusPublished {
		m.mu.Unlock()
		return nil, fmt.Errorf("playbook %s is not published (status: %s)", playbookID, pb.Status)
	}

	m.execCounter++
	execID := fmt.Sprintf("exec-%d-%d", time.Now().UnixMilli(), m.execCounter)

	exec := &Execution{
		ID:          execID,
		PlaybookID:  playbookID,
		Status:      ExecPending,
		TriggeredBy: triggeredBy,
		Context:     vars,
		StepResults: make([]StepResult, 0, len(pb.Steps)),
	}

	if pb.RequiresApproval {
		exec.Status = ExecPending
		exec.Approval = &ApprovalRecord{
			RequestedAt: time.Now(),
			RequestedBy: triggeredBy,
		}
		m.executions[execID] = exec
		m.mu.Unlock()
		return exec, nil
	}

	m.executions[execID] = exec
	m.mu.Unlock()
	return m.runExecution(ctx, exec)
}

// ApproveExecution approves a pending execution.
func (m *Manager) ApproveExecution(ctx context.Context, execID, approver, comment string) error {
	m.mu.Lock()

	exec, ok := m.executions[execID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("execution %s not found", execID)
	}
	if exec.Status != ExecPending {
		m.mu.Unlock()
		return fmt.Errorf("execution %s is not pending approval (status: %s)", execID, exec.Status)
	}
	if exec.Approval == nil {
		m.mu.Unlock()
		return fmt.Errorf("execution %s has no approval request", execID)
	}

	now := time.Now()
	exec.Approval.Approver = approver
	exec.Approval.Approved = true
	exec.Approval.DecidedAt = &now
	exec.Approval.Comment = comment
	exec.Status = ExecApproved

	m.mu.Unlock()

	// Run the execution after approval
	_, err := m.runExecution(ctx, exec)
	return err
}

// RejectExecution rejects a pending execution.
func (m *Manager) RejectExecution(ctx context.Context, execID, approver, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exec, ok := m.executions[execID]
	if !ok {
		return fmt.Errorf("execution %s not found", execID)
	}
	if exec.Status != ExecPending {
		return fmt.Errorf("execution %s is not pending approval", execID)
	}

	now := time.Now()
	exec.Approval.Approver = approver
	exec.Approval.Approved = false
	exec.Approval.DecidedAt = &now
	exec.Approval.Comment = comment
	exec.Status = ExecRejected

	return nil
}

// CancelExecution cancels a running execution.
func (m *Manager) CancelExecution(ctx context.Context, execID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exec, ok := m.executions[execID]
	if !ok {
		return fmt.Errorf("execution %s not found", execID)
	}
	if exec.Status != ExecRunning && exec.Status != ExecPending {
		return fmt.Errorf("execution %s cannot be cancelled (status: %s)", execID, exec.Status)
	}

	exec.Status = ExecCancelled
	now := time.Now()
	exec.CompletedAt = &now

	return nil
}

// GetExecution retrieves an execution by ID.
func (m *Manager) GetExecution(ctx context.Context, execID string) (*Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exec, ok := m.executions[execID]
	if !ok {
		return nil, fmt.Errorf("execution %s not found", execID)
	}
	return exec, nil
}

// ListExecutions lists executions for a playbook.
func (m *Manager) ListExecutions(ctx context.Context, playbookID string) []*Execution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Execution
	for _, exec := range m.executions {
		if playbookID != "" && exec.PlaybookID != playbookID {
			continue
		}
		result = append(result, exec)
	}
	return result
}

// GenerateSLAReport generates an SLA compliance report for a playbook.
func (m *Manager) GenerateSLAReport(ctx context.Context, playbookID string) (*SLAReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pb, ok := m.playbooks[playbookID]
	if !ok {
		return nil, fmt.Errorf("playbook %s not found", playbookID)
	}

	report := &SLAReport{
		PlaybookID:   playbookID,
		PlaybookName: pb.Name,
		GeneratedAt:  time.Now(),
	}

	var durations []int64
	for _, exec := range m.executions {
		if exec.PlaybookID != playbookID {
			continue
		}
		if exec.Status != ExecSuccess && exec.Status != ExecFailed {
			continue
		}

		report.TotalExecutions++
		durations = append(durations, exec.DurationMs)

		slaTarget := int64(pb.SLATargetMinutes) * 60 * 1000
		if slaTarget == 0 || exec.DurationMs <= slaTarget {
			report.SLAMet++
		} else {
			report.SLAMissed++
		}
	}

	if report.TotalExecutions > 0 {
		report.SLACompliancePct = float64(report.SLAMet) / float64(report.TotalExecutions) * 100

		var total int64
		for _, d := range durations {
			total += d
		}
		report.AvgDurationMs = total / int64(len(durations))

		// P95 calculation
		if len(durations) > 0 {
			sorted := make([]int64, len(durations))
			copy(sorted, durations)
			sortInt64s(sorted)
			p95Idx := int(float64(len(sorted)) * 0.95)
			if p95Idx >= len(sorted) {
				p95Idx = len(sorted) - 1
			}
			report.P95DurationMs = sorted[p95Idx]
		}
	}

	return report, nil
}

// AddKnowledgeEntry adds a knowledge base entry.
func (m *Manager) AddKnowledgeEntry(ctx context.Context, entry *KnowledgeEntry) error {
	if entry.Title == "" {
		return fmt.Errorf("knowledge entry title is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	m.knowledge[entry.ID] = entry
	return nil
}

// GetKnowledgeEntry retrieves a knowledge entry by ID.
func (m *Manager) GetKnowledgeEntry(ctx context.Context, id string) (*KnowledgeEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.knowledge[id]
	if !ok {
		return nil, fmt.Errorf("knowledge entry %s not found", id)
	}
	return entry, nil
}

// ListKnowledgeEntries lists knowledge entries, optionally filtered by playbook ID.
func (m *Manager) ListKnowledgeEntries(ctx context.Context, playbookID string) []*KnowledgeEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*KnowledgeEntry
	for _, entry := range m.knowledge {
		if playbookID != "" && entry.PlaybookID != playbookID {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// MarkHelpful increments the helpful count of a knowledge entry.
func (m *Manager) MarkHelpful(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.knowledge[id]
	if !ok {
		return fmt.Errorf("knowledge entry %s not found", id)
	}
	entry.Helpful++
	entry.UpdatedAt = time.Now()
	return nil
}

// runExecution simulates the execution of a playbook's steps.
func (m *Manager) runExecution(ctx context.Context, exec *Execution) (*Execution, error) {
	m.mu.Lock()
	exec.Status = ExecRunning
	now := time.Now()
	exec.StartedAt = &now
	m.mu.Unlock()

	pb := m.playbooks[exec.PlaybookID]
	startTime := time.Now()

	for _, step := range pb.Steps {
		select {
		case <-ctx.Done():
			exec.Status = ExecCancelled
			endTime := time.Now()
			exec.CompletedAt = &endTime
			exec.DurationMs = endTime.Sub(startTime).Milliseconds()
			exec.Error = "context cancelled"
			return exec, ctx.Err()
		default:
		}

		stepStart := time.Now()
		result := StepResult{
			StepID:    step.ID,
			StartedAt: stepStart,
		}

		// Simulate step execution
		if err := m.executeStep(ctx, step, exec.Context); err != nil {
			result.Status = ExecFailed
			result.Error = err.Error()
			endTime := time.Now()
			result.EndAt = endTime
			result.DurationMs = endTime.Sub(stepStart).Milliseconds()

			m.mu.Lock()
			exec.StepResults = append(exec.StepResults, result)
			m.mu.Unlock()

			if step.OnFailure == "abort" || step.OnFailure == "" {
				exec.Status = ExecFailed
				exec.Error = fmt.Sprintf("step %s failed: %s", step.ID, err.Error())
				endTime = time.Now()
				exec.CompletedAt = &endTime
				exec.DurationMs = endTime.Sub(startTime).Milliseconds()
				m.updatePlaybookStats(exec.PlaybookID, false)
				return exec, err
			}
			// continue on failure
			continue
		}

		result.Status = ExecSuccess
		endTime := time.Now()
		result.EndAt = endTime
		result.DurationMs = endTime.Sub(stepStart).Milliseconds()

		m.mu.Lock()
		exec.StepResults = append(exec.StepResults, result)
		m.mu.Unlock()
	}

	exec.Status = ExecSuccess
	endTime := time.Now()
	exec.CompletedAt = &endTime
	exec.DurationMs = endTime.Sub(startTime).Milliseconds()
	m.updatePlaybookStats(exec.PlaybookID, true)

	return exec, nil
}

// executeStep simulates executing a single step.
func (m *Manager) executeStep(ctx context.Context, step Step, vars map[string]string) error {
	switch step.Type {
	case StepCommand, StepScript, StepHTTP, StepCheck, StepNotify:
		// In real implementation, these would execute actual commands/scripts
		return nil
	case StepApproval:
		return fmt.Errorf("approval step requires external approval")
	case StepWait:
		if step.Timeout > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(step.Timeout) * time.Second):
				return nil
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// updatePlaybookStats updates run count and success rate.
func (m *Manager) updatePlaybookStats(playbookID string, success bool) {
	pb, ok := m.playbooks[playbookID]
	if !ok {
		return
	}

	pb.RunCount++
	if success {
		successCount := int(pb.SuccessRate * float64(pb.RunCount-1) / 100)
		if success {
			successCount++
		}
		pb.SuccessRate = float64(successCount) / float64(pb.RunCount) * 100
	} else {
		successCount := int(pb.SuccessRate * float64(pb.RunCount-1) / 100)
		pb.SuccessRate = float64(successCount) / float64(pb.RunCount) * 100
	}
	pb.UpdatedAt = time.Now()
}

// ExportPlaybook exports a playbook as JSON.
func (m *Manager) ExportPlaybook(ctx context.Context, id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pb, ok := m.playbooks[id]
	if !ok {
		return nil, fmt.Errorf("playbook %s not found", id)
	}
	return json.MarshalIndent(pb, "", "  ")
}

// ImportPlaybook imports a playbook from JSON.
func (m *Manager) ImportPlaybook(ctx context.Context, data []byte) (*Playbook, error) {
	var pb Playbook
	if err := json.Unmarshal(data, &pb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal playbook: %w", err)
	}

	if err := m.CreatePlaybook(ctx, &pb); err != nil {
		return nil, err
	}
	return &pb, nil
}

// sortInt64s sorts a slice of int64 in ascending order.
func sortInt64s(data []int64) {
	for i := 1; i < len(data); i++ {
		key := data[i]
		j := i - 1
		for j >= 0 && data[j] > key {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = key
	}
}
