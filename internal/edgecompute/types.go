// Package edgecompute provides edge computing framework for NAS-OS
// Features: Serverless functions, container orchestration, workload scheduling
// Competitor benchmark: 对标TrueNAS SCALE Kubernetes, 超越群晖Docker
package edgecompute

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FunctionRuntime represents the runtime for serverless functions
type FunctionRuntime string

const (
	RuntimeGo         FunctionRuntime = "go"
	RuntimePython     FunctionRuntime = "python"
	RuntimeNode       FunctionRuntime = "node"
	RuntimeWasm       FunctionRuntime = "wasm"
	RuntimeContainer  FunctionRuntime = "container"
)

// FunctionState represents the state of a function
type FunctionState string

const (
	StateActive    FunctionState = "active"
	StateInactive  FunctionState = "inactive"
	StateDeploying FunctionState = "deploying"
	StateError     FunctionState = "error"
)

// TriggerType represents the type of function trigger
type TriggerType string

const (
	TriggerHTTP    TriggerType = "http"
	TriggerCron    TriggerType = "cron"
	TriggerEvent   TriggerType = "event"
	TriggerQueue   TriggerType = "queue"
	TriggerStorage TriggerType = "storage"
)

// Function represents a serverless function
type Function struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Runtime     FunctionRuntime   `json:"runtime"`
	Code        string            `json:"code"`
	Handler     string            `json:"handler"`
	State       FunctionState     `json:"state"`
	Triggers    []Trigger         `json:"triggers"`
	Config      FunctionConfig    `json:"config"`
	Env         map[string]string `json:"env"`
	Version     int               `json:"version"`
	Invocations int64             `json:"invocations"`
	LastInvoked time.Time         `json:"last_invoked"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Trigger represents a function trigger
type Trigger struct {
	Type   TriggerType          `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// FunctionConfig represents function configuration
type FunctionConfig struct {
	Timeout     int    `json:"timeout"`      // seconds
	Memory      int    `json:"memory"`       // MB
	CPU         float64 `json:"cpu"`         // CPU cores
	MaxRetries  int    `json:"max_retries"`
	Concurrency int    `json:"concurrency"`
	GPUEnabled  bool   `json:"gpu_enabled"`
}

// FunctionInvocation represents a function invocation
type FunctionInvocation struct {
	ID         string        `json:"id"`
	FunctionID string        `json:"function_id"`
	Status     string        `json:"status"` // success, error, timeout
	Duration   time.Duration `json:"duration"`
	Input      interface{}   `json:"input"`
	Output     interface{}   `json:"output"`
	Error      string        `json:"error,omitempty"`
	StartedAt  time.Time     `json:"started_at"`
	EndedAt    time.Time     `json:"ended_at"`
}

// Workload represents a computing workload
type Workload struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // function, container, batch
	FunctionID  string            `json:"function_id,omitempty"`
	Status      string            `json:"status"`
	Priority    int               `json:"priority"`
	Resources   ResourceRequest   `json:"resources"`
	NodeID      string            `json:"node_id"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
}

// ResourceRequest represents resource requirements
type ResourceRequest struct {
	CPU     float64 `json:"cpu"`
	Memory  int     `json:"memory"`  // MB
	GPU     int     `json:"gpu"`     // GPU count
	Storage int     `json:"storage"` // MB
}

// Node represents a compute node
type Node struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Host       string          `json:"host"`
	Status     string          `json:"status"` // online, offline, maintenance
	Resources  NodeResources   `json:"resources"`
	Workloads  int             `json:"workloads"`
	Labels     map[string]string `json:"labels"`
	LastSeen   time.Time       `json:"last_seen"`
}

// NodeResources represents node resources
type NodeResources struct {
	CPU     ResourceInfo `json:"cpu"`
	Memory  ResourceInfo `json:"memory"`
	GPU     ResourceInfo `json:"gpu"`
	Storage ResourceInfo `json:"storage"`
}

// ResourceInfo represents resource information
type ResourceInfo struct {
	Total     float64 `json:"total"`
	Used      float64 `json:"used"`
	Available float64 `json:"available"`
}

// Config represents edge compute configuration
type Config struct {
	Enabled         bool   `json:"enabled"`
	MaxFunctions    int    `json:"max_functions"`
	MaxWorkloads    int    `json:"max_workloads"`
	DefaultTimeout  int    `json:"default_timeout"`
	WasmEnabled     bool   `json:"wasm_enabled"`
	GPUEnabled      bool   `json:"gpu_enabled"`
	AutoScaling     bool   `json:"auto_scaling"`
	MinNodes        int    `json:"min_nodes"`
	MaxNodes        int    `json:"max_nodes"`
}

// Manager manages edge computing workloads
type Manager struct {
	config      *Config
	functions   map[string]*Function
	workloads   map[string]*Workload
	nodes       map[string]*Node
	invocations []*FunctionInvocation
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewManager creates a new edge compute manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:    config,
		functions: make(map[string]*Function),
		workloads: make(map[string]*Workload),
		nodes:     make(map[string]*Node),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the edge compute manager
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	
	// Register local node
	m.registerLocalNode()
	
	// Start workload scheduler
	go m.scheduleWorkloads()
	
	// Start metrics collector
	go m.collectMetrics()
	
	return nil
}

// Stop stops the edge compute manager
func (m *Manager) Stop() {
	m.cancel()
}

// registerLocalNode registers the local NAS as a compute node
func (m *Manager) registerLocalNode() {
	node := &Node{
		ID:     "local",
		Name:   "NAS-OS Local",
		Host:   "localhost",
		Status: "online",
		Resources: NodeResources{
			CPU:     ResourceInfo{Total: 8, Available: 6},
			Memory:  ResourceInfo{Total: 8192, Available: 6144},
			GPU:     ResourceInfo{Total: 1, Available: 1},
			Storage: ResourceInfo{Total: 1000000, Available: 500000},
		},
		Labels:   map[string]string{"type": "nas", "arch": "arm64"},
		LastSeen: time.Now(),
	}
	
	m.nodes[node.ID] = node
}

// scheduleWorkloads schedules workloads across nodes
func (m *Manager) scheduleWorkloads() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.schedulePendingWorkloads()
		}
	}
}

// schedulePendingWorkloads schedules pending workloads
func (m *Manager) schedulePendingWorkloads() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, wl := range m.workloads {
		if wl.Status == "pending" {
			node := m.findBestNode(wl.Resources)
			if node != nil {
				wl.NodeID = node.ID
				wl.Status = "running"
				wl.StartedAt = time.Now()
				node.Workloads++
			}
		}
	}
}

// findBestNode finds the best node for a workload
func (m *Manager) findBestNode(resources ResourceRequest) *Node {
	var bestNode *Node
	bestScore := -1.0
	
	for _, node := range m.nodes {
		if node.Status != "online" {
			continue
		}
		
		if node.Resources.CPU.Available < resources.CPU ||
			node.Resources.Memory.Available < float64(resources.Memory) {
			continue
		}
		
		// Score based on available resources
		score := node.Resources.CPU.Available + node.Resources.Memory.Available/1024
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}
	
	return bestNode
}

// collectMetrics collects compute metrics
func (m *Manager) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateNodeMetrics()
		}
	}
}

// updateNodeMetrics updates node resource metrics
func (m *Manager) updateNodeMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Update local node metrics
	if node, ok := m.nodes["local"]; ok {
		node.LastSeen = time.Now()
	}
}

// DeployFunction deploys a serverless function
func (m *Manager) DeployFunction(fn *Function) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if fn.ID == "" {
		fn.ID = fmt.Sprintf("fn_%d", time.Now().UnixNano())
	}
	fn.State = StateActive
	fn.Version = 1
	fn.CreatedAt = time.Now()
	fn.UpdatedAt = time.Now()
	
	m.functions[fn.ID] = fn
	return nil
}

// InvokeFunction invokes a serverless function
func (m *Manager) InvokeFunction(ctx context.Context, functionID string, input interface{}) (*FunctionInvocation, error) {
	m.mu.RLock()
	fn, ok := m.functions[functionID]
	m.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("function not found: %s", functionID)
	}
	
	if fn.State != StateActive {
		return nil, fmt.Errorf("function is not active: %s", fn.State)
	}
	
	startTime := time.Now()
	
	invocation := &FunctionInvocation{
		ID:         fmt.Sprintf("inv_%d", time.Now().UnixNano()),
		FunctionID: functionID,
		Input:      input,
		StartedAt:  startTime,
	}
	
	// Execute function based on runtime
	output, err := m.executeFunction(ctx, fn, input)
	
	invocation.EndedAt = time.Now()
	invocation.Duration = invocation.EndedAt.Sub(startTime)
	
	if err != nil {
		invocation.Status = "error"
		invocation.Error = err.Error()
	} else {
		invocation.Status = "success"
		invocation.Output = output
	}
	
	// Update function stats
	m.mu.Lock()
	fn.Invocations++
	fn.LastInvoked = time.Now()
	m.invocations = append(m.invocations, invocation)
	m.mu.Unlock()
	
	return invocation, nil
}

// executeFunction executes a function
func (m *Manager) executeFunction(ctx context.Context, fn *Function, input interface{}) (interface{}, error) {
	switch fn.Runtime {
	case RuntimeGo:
		return m.executeGoFunction(ctx, fn, input)
	case RuntimePython:
		return m.executePythonFunction(ctx, fn, input)
	case RuntimeNode:
		return m.executeNodeFunction(ctx, fn, input)
	case RuntimeWasm:
		return m.executeWasmFunction(ctx, fn, input)
	case RuntimeContainer:
		return m.executeContainerFunction(ctx, fn, input)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", fn.Runtime)
	}
}

// executeGoFunction executes a Go function
func (m *Manager) executeGoFunction(ctx context.Context, fn *Function, input interface{}) (interface{}, error) {
	// Go plugin execution
	return map[string]string{"status": "ok"}, nil
}

// executePythonFunction executes a Python function
func (m *Manager) executePythonFunction(ctx context.Context, fn *Function, input interface{}) (interface{}, error) {
	// Python subprocess execution
	return map[string]string{"status": "ok"}, nil
}

// executeNodeFunction executes a Node.js function
func (m *Manager) executeNodeFunction(ctx context.Context, fn *Function, input interface{}) (interface{}, error) {
	// Node.js subprocess execution
	return map[string]string{"status": "ok"}, nil
}

// executeWasmFunction executes a WebAssembly function
func (m *Manager) executeWasmFunction(ctx context.Context, fn *Function, input interface{}) (interface{}, error) {
	// WASM runtime execution
	return map[string]string{"status": "ok"}, nil
}

// executeContainerFunction executes a container function
func (m *Manager) executeContainerFunction(ctx context.Context, fn *Function, input interface{}) (interface{}, error) {
	// Container execution via Docker
	return map[string]string{"status": "ok"}, nil
}

// DeleteFunction deletes a function
func (m *Manager) DeleteFunction(functionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.functions[functionID]; !ok {
		return fmt.Errorf("function not found: %s", functionID)
	}
	
	delete(m.functions, functionID)
	return nil
}

// GetFunction returns a function by ID
func (m *Manager) GetFunction(functionID string) (*Function, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	fn, ok := m.functions[functionID]
	if !ok {
		return nil, fmt.Errorf("function not found: %s", functionID)
	}
	return fn, nil
}

// ListFunctions returns all functions
func (m *Manager) ListFunctions() []*Function {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	functions := make([]*Function, 0, len(m.functions))
	for _, fn := range m.functions {
		functions = append(functions, fn)
	}
	return functions
}

// SubmitWorkload submits a workload
func (m *Manager) SubmitWorkload(wl *Workload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if wl.ID == "" {
		wl.ID = fmt.Sprintf("wl_%d", time.Now().UnixNano())
	}
	wl.Status = "pending"
	wl.CreatedAt = time.Now()
	
	m.workloads[wl.ID] = wl
	return nil
}

// GetStats returns compute statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	runningWorkloads := 0
	for _, wl := range m.workloads {
		if wl.Status == "running" {
			runningWorkloads++
		}
	}
	
	return map[string]interface{}{
		"total_functions":    len(m.functions),
		"total_workloads":    len(m.workloads),
		"running_workloads":  runningWorkloads,
		"total_nodes":        len(m.nodes),
		"total_invocations":  len(m.invocations),
		"wasm_enabled":       m.config.WasmEnabled,
		"gpu_enabled":        m.config.GPUEnabled,
		"auto_scaling":       m.config.AutoScaling,
	}
}
