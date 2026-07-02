package wasmruntime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// WasmModule WebAssembly模块.
type WasmModule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Size        int64             `json:"size"`
	Hash        string            `json:"hash"`
	EntryPoint  string            `json:"entryPoint"`
	Exports     []string          `json:"exports"`
	Imports     []string          `json:"imports"`
	Permissions []string          `json:"permissions"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// WasmInstance WebAssembly实例.
type WasmInstance struct {
	ID        string        `json:"id"`
	ModuleID  string        `json:"moduleId"`
	State     string        `json:"state"` // created, running, paused, stopped, error
	Memory    int64         `json:"memory"`
	MaxMemory int64         `json:"maxMemory"`
	StartTime time.Time     `json:"startTime"`
	EndTime   *time.Time    `json:"endTime,omitempty"`
	Error     string        `json:"error,omitempty"`
	CallCount int64         `json:"callCount"`
	TotalTime time.Duration `json:"totalTime"`
}

// WasmFunction 调用请求.
type WasmFunction struct {
	ModuleID string        `json:"moduleId"`
	Function string        `json:"function"`
	Args     []interface{} `json:"args"`
	Timeout  time.Duration `json:"timeout"`
}

// WasmResult 调用结果.
type WasmResult struct {
	InstanceID string        `json:"instanceID"`
	Function   string        `json:"function"`
	Result     interface{}   `json:"result"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	GasUsed    int64         `json:"gasUsed"`
}

// WasmStats 运行时统计.
type WasmStats struct {
	TotalModules    int           `json:"totalModules"`
	TotalInstances  int           `json:"totalInstances"`
	ActiveInstances int           `json:"activeInstances"`
	TotalCalls      int64         `json:"totalCalls"`
	TotalExecTime   time.Duration `json:"totalExecTime"`
	TotalMemory     int64         `json:"totalMemory"`
	AvgCallTime     time.Duration `json:"avgCallTime"`
}

// WasmConfig WebAssembly运行时配置.
type WasmConfig struct {
	Enabled        bool          `json:"enabled"`
	MaxModules     int           `json:"maxModules"`
	MaxInstances   int           `json:"maxInstances"`
	DefaultMemory  int64         `json:"defaultMemory"`
	MaxMemory      int64         `json:"maxMemory"`
	DefaultTimeout time.Duration `json:"defaultTimeout"`
	MaxTimeout     time.Duration `json:"maxTimeout"`
	EnableSandbox  bool          `json:"enableSandbox"`
}

// WasmRuntime WebAssembly运行时.
type WasmRuntime struct {
	config    WasmConfig
	logger    *slog.Logger
	mu        sync.RWMutex
	modules   map[string]*WasmModule
	instances map[string]*WasmInstance
	stats     WasmStats
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
}

// NewWasmRuntime 创建WebAssembly运行时.
func NewWasmRuntime(config WasmConfig, logger *slog.Logger) *WasmRuntime {
	ctx, cancel := context.WithCancel(context.Background())

	return &WasmRuntime{
		config:    config,
		logger:    logger,
		modules:   make(map[string]*WasmModule),
		instances: make(map[string]*WasmInstance),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动运行时.
func (r *WasmRuntime) Start() error {
	if !r.config.Enabled {
		r.logger.Info("WebAssembly运行时未启用")
		return nil
	}

	r.running = true
	r.logger.Info("WebAssembly运行时已启动",
		"maxModules", r.config.MaxModules,
		"maxInstances", r.config.MaxInstances,
		"sandbox", r.config.EnableSandbox,
	)

	return nil
}

// Stop 停止运行时.
func (r *WasmRuntime) Stop() {
	r.cancel()

	// 停止所有实例
	r.mu.Lock()
	for _, inst := range r.instances {
		if inst.State == "running" {
			inst.State = "stopped"
			now := time.Now()
			inst.EndTime = &now
		}
	}
	r.running = false
	r.mu.Unlock()

	r.logger.Info("WebAssembly运行时已停止")
}

// LoadModule 加载模块.
func (r *WasmRuntime) LoadModule(name string, data []byte) (*WasmModule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查模块数量限制
	if len(r.modules) >= r.config.MaxModules {
		return nil, fmt.Errorf("已达到最大模块数量限制: %d", r.config.MaxModules)
	}

	module := &WasmModule{
		ID:         fmt.Sprintf("module_%d", time.Now().UnixNano()),
		Name:       name,
		Size:       int64(len(data)),
		EntryPoint: "_start",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	r.modules[module.ID] = module
	r.stats.TotalModules++

	r.logger.Info("加载WebAssembly模块", "id", module.ID, "name", name, "size", module.Size)

	return module, nil
}

// UnloadModule 卸载模块.
func (r *WasmRuntime) UnloadModule(moduleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	module, ok := r.modules[moduleID]
	if !ok {
		return fmt.Errorf("模块不存在: %s", moduleID)
	}

	// 检查是否有运行中的实例
	for _, inst := range r.instances {
		if inst.ModuleID == moduleID && inst.State == "running" {
			return fmt.Errorf("模块有运行中的实例，无法卸载")
		}
	}

	delete(r.modules, moduleID)
	r.stats.TotalModules--

	r.logger.Info("卸载WebAssembly模块", "id", moduleID, "name", module.Name)

	return nil
}

// GetModule 获取模块.
func (r *WasmRuntime) GetModule(moduleID string) (*WasmModule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	module, ok := r.modules[moduleID]
	if !ok {
		return nil, fmt.Errorf("模块不存在: %s", moduleID)
	}

	return module, nil
}

// ListModules 列出模块.
func (r *WasmRuntime) ListModules() []*WasmModule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var modules []*WasmModule
	for _, m := range r.modules {
		modules = append(modules, m)
	}

	return modules
}

// CreateInstance 创建实例.
func (r *WasmRuntime) CreateInstance(moduleID string, memoryLimit int64) (*WasmInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.modules[moduleID]; !ok {
		return nil, fmt.Errorf("模块不存在: %s", moduleID)
	}

	// 检查实例数量限制
	if len(r.instances) >= r.config.MaxInstances {
		return nil, fmt.Errorf("已达到最大实例数量限制: %d", r.config.MaxInstances)
	}

	// 设置内存限制
	if memoryLimit == 0 {
		memoryLimit = r.config.DefaultMemory
	}
	if memoryLimit > r.config.MaxMemory {
		memoryLimit = r.config.MaxMemory
	}

	instance := &WasmInstance{
		ID:        fmt.Sprintf("instance_%d", time.Now().UnixNano()),
		ModuleID:  moduleID,
		State:     "created",
		MaxMemory: memoryLimit,
		StartTime: time.Now(),
	}

	r.instances[instance.ID] = instance
	r.stats.TotalInstances++

	r.logger.Info("创建WebAssembly实例",
		"id", instance.ID,
		"module", moduleID,
		"memoryLimit", memoryLimit,
	)

	return instance, nil
}

// DestroyInstance 销毁实例.
func (r *WasmRuntime) DestroyInstance(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, ok := r.instances[instanceID]
	if !ok {
		return fmt.Errorf("实例不存在: %s", instanceID)
	}

	if instance.State == "running" || instance.State == "paused" {
		r.stats.ActiveInstances--
	}

	now := time.Now()
	instance.EndTime = &now
	instance.State = "stopped"

	delete(r.instances, instanceID)
	r.stats.TotalInstances--

	r.logger.Info("销毁WebAssembly实例", "id", instanceID)

	return nil
}

// CallFunction 调用函数.
func (r *WasmRuntime) CallFunction(req *WasmFunction) (*WasmResult, error) {
	r.mu.Lock()

	if _, ok := r.modules[req.ModuleID]; !ok {
		r.mu.Unlock()
		return nil, fmt.Errorf("模块不存在: %s", req.ModuleID)
	}

	// 获取或创建实例
	var instance *WasmInstance
	for _, inst := range r.instances {
		if inst.ModuleID == req.ModuleID && (inst.State == "created" || inst.State == "paused") {
			instance = inst
			break
		}
	}

	if instance == nil {
		if len(r.instances) >= r.config.MaxInstances {
			r.mu.Unlock()
			return nil, fmt.Errorf("已达到最大实例数量限制")
		}

		instance = &WasmInstance{
			ID:        fmt.Sprintf("instance_%d", time.Now().UnixNano()),
			ModuleID:  req.ModuleID,
			State:     "running",
			MaxMemory: r.config.DefaultMemory,
			StartTime: time.Now(),
		}
		r.instances[instance.ID] = instance
		r.stats.TotalInstances++
	}

	instance.State = "running"
	r.stats.ActiveInstances++
	r.mu.Unlock()

	startTime := time.Now()

	callResult := map[string]interface{}{
		"function": req.Function,
		"args":     req.Args,
	}
	if req.Function == "echo" && len(req.Args) > 0 {
		callResult["value"] = req.Args[0]
	}
	result := &WasmResult{
		InstanceID: instance.ID,
		Function:   req.Function,
		Result:     callResult,
		Duration:   time.Since(startTime),
		GasUsed:    100,
	}

	r.mu.Lock()
	instance.CallCount++
	instance.TotalTime += result.Duration
	instance.State = "paused"
	r.stats.ActiveInstances--
	r.stats.TotalCalls++
	r.stats.TotalExecTime += result.Duration
	if r.stats.TotalCalls > 0 {
		r.stats.AvgCallTime = r.stats.TotalExecTime / time.Duration(r.stats.TotalCalls)
	}
	r.mu.Unlock()

	r.logger.Debug("调用WebAssembly函数",
		"module", req.ModuleID,
		"function", req.Function,
		"duration", result.Duration,
	)

	return result, nil
}

// GetInstance 获取实例.
func (r *WasmRuntime) GetInstance(instanceID string) (*WasmInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instance, ok := r.instances[instanceID]
	if !ok {
		return nil, fmt.Errorf("实例不存在: %s", instanceID)
	}

	return instance, nil
}

// ListInstances 列出实例.
func (r *WasmRuntime) ListInstances(state string) []*WasmInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var instances []*WasmInstance
	for _, i := range r.instances {
		if state == "" || i.State == state {
			instances = append(instances, i)
		}
	}

	return instances
}

// GetStats 获取统计.
func (r *WasmRuntime) GetStats() WasmStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// PauseInstance 暂停实例.
func (r *WasmRuntime) PauseInstance(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, ok := r.instances[instanceID]
	if !ok {
		return fmt.Errorf("实例不存在: %s", instanceID)
	}

	if instance.State != "running" {
		return fmt.Errorf("实例未运行: %s", instanceID)
	}

	instance.State = "paused"
	r.stats.ActiveInstances--

	return nil
}

// ResumeInstance 恢复实例.
func (r *WasmRuntime) ResumeInstance(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, ok := r.instances[instanceID]
	if !ok {
		return fmt.Errorf("实例不存在: %s", instanceID)
	}

	if instance.State != "paused" {
		return fmt.Errorf("实例未暂停: %s", instanceID)
	}

	instance.State = "running"
	r.stats.ActiveInstances++

	return nil
}

// IsRunning 是否运行中.
func (r *WasmRuntime) IsRunning() bool {
	return r.running
}
