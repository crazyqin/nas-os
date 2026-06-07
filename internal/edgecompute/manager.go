package edgecompute

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Invocation 调用结果
type Invocation struct {
	ID         string        `json:"id"`
	FunctionID string        `json:"function_id"`
	Status     string        `json:"status"`
	Output     string        `json:"output,omitempty"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	Timestamp  time.Time     `json:"timestamp"`
}

// LocalNode 本地节点
type LocalNode struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager 边缘计算管理器
type Manager struct {
	config    *Config
	functions map[string]*Function
	workloads map[string]*Workload
	nodes     map[string]*LocalNode
	mu        sync.RWMutex
	running   bool
}

// NewManager 创建边缘计算管理器
func NewManager(config *Config) *Manager {
	return &Manager{
		config:    config,
		functions: make(map[string]*Function),
		workloads: make(map[string]*Workload),
		nodes:     make(map[string]*LocalNode),
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("manager already running")
	}

	m.running = true
	m.registerLocalNode()
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
}

// registerLocalNode 注册本地节点
func (m *Manager) registerLocalNode() {
	m.nodes["local"] = &LocalNode{
		ID:        "local",
		Status:    "online",
		CreatedAt: time.Now(),
	}
}

// DeployFunction 部署函数
func (m *Manager) DeployFunction(fn *Function) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if fn.ID == "" {
		fn.ID = fmt.Sprintf("fn_%d", time.Now().UnixNano())
	}

	if fn.Runtime == "" {
		fn.Runtime = RuntimeGo
	}

	fn.State = StateActive
	fn.Status = "active"
	fn.CreatedAt = time.Now()
	fn.UpdatedAt = time.Now()

	m.functions[fn.ID] = fn
	return nil
}

// InvokeFunction 调用函数
func (m *Manager) InvokeFunction(ctx context.Context, functionID string, params map[string]string) (*Invocation, error) {
	m.mu.RLock()
	fn, ok := m.functions[functionID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("function not found: %s", functionID)
	}

	start := time.Now()
	invocation := &Invocation{
		ID:         fmt.Sprintf("inv_%d", time.Now().UnixNano()),
		FunctionID: fn.ID,
		Status:     "success",
		Output:     "executed",
		Duration:   time.Since(start),
		Timestamp:  time.Now(),
	}

	m.mu.Lock()
	fn.InvokeCount++
	now := time.Now()
	fn.LastInvoked = &now
	m.mu.Unlock()

	return invocation, nil
}

// DeleteFunction 删除函数
func (m *Manager) DeleteFunction(functionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.functions[functionID]; !ok {
		return fmt.Errorf("function not found: %s", functionID)
	}

	delete(m.functions, functionID)
	return nil
}

// GetFunction 获取函数
func (m *Manager) GetFunction(functionID string) (*Function, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fn, ok := m.functions[functionID]
	if !ok {
		return nil, fmt.Errorf("function not found: %s", functionID)
	}

	return fn, nil
}

// ListFunctions 列出所有函数
func (m *Manager) ListFunctions() []*Function {
	m.mu.RLock()
	defer m.mu.RUnlock()

	functions := make([]*Function, 0, len(m.functions))
	for _, fn := range m.functions {
		functions = append(functions, fn)
	}
	return functions
}

// SubmitWorkload 提交工作负载
func (m *Manager) SubmitWorkload(wl *Workload) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if wl.ID == "" {
		wl.ID = fmt.Sprintf("wl_%d", time.Now().UnixNano())
	}

	wl.Status = StatusPending
	wl.CreatedAt = time.Now()
	wl.UpdatedAt = time.Now()

	m.workloads[wl.ID] = wl
	return nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_functions": len(m.functions),
		"total_workloads": len(m.workloads),
		"total_nodes":     len(m.nodes),
		"wasm_enabled":    m.config.WasmEnabled,
		"gpu_enabled":     m.config.GPUEnabled,
		"auto_scaling":    m.config.AutoScaling,
		"running":         m.running,
	}
}
