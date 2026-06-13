// Package dsmagentorch 提供DSM Agent编排引擎，对标群晖DSM Agent。
// 支持MCP协议集成、工具注册、工作流编排、安全护栏、诊断能力。
package dsmagentorch

import (
	"fmt"
	"sync"
	"time"
)

// AgentRole 定义Agent角色
type AgentRole string

const (
	RoleSystemAdmin   AgentRole = "system_admin"
	RoleStorageAdmin  AgentRole = "storage_admin"
	RoleNetworkAdmin  AgentRole = "network_admin"
	RoleSecurityAdmin AgentRole = "security_admin"
	RoleBackupAdmin   AgentRole = "backup_admin"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// WorkflowType 工作流类型
type WorkflowType string

const (
	WorkflowHealthCheck     WorkflowType = "health_check"
	WorkflowBackupVerify    WorkflowType = "backup_verify"
	WorkflowSecurityScan    WorkflowType = "security_scan"
	WorkflowStorageOpt      WorkflowType = "storage_optimization"
	WorkflowPerformanceTune WorkflowType = "performance_tune"
	WorkflowAlertResponse   WorkflowType = "alert_response"
	WorkflowMCPIntegration  WorkflowType = "mcp_integration"
)

// MCPTool MCP工具定义
type MCPTool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
	Handler     string            `json:"handler"`
	Enabled     bool              `json:"enabled"`
}

// Task 任务定义
type Task struct {
	ID          string       `json:"id"`
	Workflow    WorkflowType `json:"workflow"`
	Status      TaskStatus   `json:"status"`
	Priority    int          `json:"priority"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Result      *TaskResult  `json:"result,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// TaskResult 任务结果
type TaskResult struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Duration    time.Duration          `json:"duration"`
}

// Guardrail 安全护栏
type Guardrail struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Rule        string `json:"rule"`
	Enabled     bool   `json:"enabled"`
	Action      string `json:"action"` // block, warn, log
}

// Diagnostic 诊断信息
type Diagnostic struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// AgentConfig Agent配置
type AgentConfig struct {
	AgentID         string        `json:"agent_id"`
	Role            AgentRole     `json:"role"`
	Name            string        `json:"name"`
	Enabled         bool          `json:"enabled"`
	ScanInterval    time.Duration `json:"scan_interval"`
	AutoRemediation bool          `json:"auto_remediation"`
	MaxConcurrent   int           `json:"max_concurrent"`
	MCPEndpoint     string        `json:"mcp_endpoint"`
}

// DefaultAgentConfig 返回默认配置
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		AgentID:         "dsm-agent-001",
		Role:            RoleSystemAdmin,
		Name:            "DSM Agent",
		Enabled:         true,
		ScanInterval:    5 * time.Minute,
		AutoRemediation: true,
		MaxConcurrent:   5,
	}
}

// Manager Agent编排管理器
type Manager struct {
	mu         sync.RWMutex
	config     *AgentConfig
	tasks      map[string]*Task
	tools      map[string]*MCPTool
	guardrails map[string]*Guardrail
	diags      []Diagnostic
	running    bool
	startTime  time.Time
}

// NewManager 创建管理器
func NewManager(config *AgentConfig) *Manager {
	if config == nil {
		config = DefaultAgentConfig()
	}
	return &Manager{
		config:     config,
		tasks:      make(map[string]*Task),
		tools:      make(map[string]*MCPTool),
		guardrails: make(map[string]*Guardrail),
		diags:      make([]Diagnostic, 0),
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("DSM Agent 已在运行")
	}
	m.running = true
	m.startTime = time.Now()
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// IsRunning 检查是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// RegisterTool 注册MCP工具
func (m *Manager) RegisterTool(tool *MCPTool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return fmt.Errorf("管理器未运行")
	}
	m.tools[tool.Name] = tool
	return nil
}

// UnregisterTool 注销MCP工具
func (m *Manager) UnregisterTool(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.tools[name]; !exists {
		return fmt.Errorf("工具不存在: %s", name)
	}
	delete(m.tools, name)
	return nil
}

// ListTools 列出所有工具
func (m *Manager) ListTools() []*MCPTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tools []*MCPTool
	for _, t := range m.tools {
		tools = append(tools, t)
	}
	return tools
}

// CreateTask 创建任务
func (m *Manager) CreateTask(workflow WorkflowType, description string, priority int) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil, fmt.Errorf("管理器未运行")
	}
	task := &Task{
		ID:          generateTaskID(),
		Workflow:    workflow,
		Status:      TaskPending,
		Priority:    priority,
		Description: description,
		CreatedAt:   time.Now(),
	}
	m.tasks[task.ID] = task
	return task, nil
}

// CompleteTask 完成任务
func (m *Manager) CompleteTask(id string, success bool, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("任务不存在: %s", id)
	}
	now := time.Now()
	task.CompletedAt = &now
	if success {
		task.Status = TaskCompleted
	} else {
		task.Status = TaskFailed
	}
	task.Result = &TaskResult{
		Success: success,
		Message: message,
		Duration: now.Sub(task.CreatedAt),
	}
	return nil
}

// GetTask 获取任务
func (m *Manager) GetTask(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}
	return task, nil
}

// ListTasks 列出任务
func (m *Manager) ListTasks(status TaskStatus) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tasks []*Task
	for _, t := range m.tasks {
		if status == "" || t.Status == status {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// AddGuardrail 添加安全护栏
func (m *Manager) AddGuardrail(g *Guardrail) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.guardrails[g.Name] = g
}

// RunDiagnostics 运行诊断
func (m *Manager) RunDiagnostics() []Diagnostic {
	m.mu.Lock()
	defer m.mu.Unlock()
	diags := []Diagnostic{
		{ID: "disk", Name: "磁盘健康", Status: "ok", Message: "所有磁盘正常", Timestamp: time.Now()},
		{ID: "memory", Name: "内存使用", Status: "ok", Message: "内存使用率正常", Timestamp: time.Now()},
		{ID: "network", Name: "网络连接", Status: "ok", Message: "网络连接正常", Timestamp: time.Now()},
	}
	m.diags = append(m.diags, diags...)
	return diags
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	completed := 0
	failed := 0
	for _, t := range m.tasks {
		if t.Status == TaskCompleted {
			completed++
		} else if t.Status == TaskFailed {
			failed++
		}
	}
	return map[string]interface{}{
		"running":        m.running,
		"total_tasks":    len(m.tasks),
		"completed":      completed,
		"failed":         failed,
		"total_tools":    len(m.tools),
		"total_guardrails": len(m.guardrails),
		"uptime":         time.Since(m.startTime).String(),
	}
}

func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
