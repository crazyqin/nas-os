package edgecomputing

import (
	"fmt"
	"sync"
	"time"
)

// ContainerState 容器状态
type ContainerState string

const (
	StatePending ContainerState = "pending"
	StateRunning ContainerState = "running"
	StateStopped ContainerState = "stopped"
	StateFailed  ContainerState = "failed"
	StateScaling ContainerState = "scaling"
)

// RuntimeLanguage 函数运行时语言
type RuntimeLanguage string

const (
	RuntimePython RuntimeLanguage = "python"
	RuntimeNodeJS RuntimeLanguage = "nodejs"
	RuntimeGo     RuntimeLanguage = "go"
	RuntimeRust   RuntimeLanguage = "rust"
	RuntimeWasm   RuntimeLanguage = "wasm"
)

// InferenceBackend 推理后端
type InferenceBackend string

const (
	BackendCPU      InferenceBackend = "cpu"
	BackendCUDA     InferenceBackend = "cuda"
	BackendNPU      InferenceBackend = "npu"
	BackendOpenVINO InferenceBackend = "openvino"
	BackendTensorRT InferenceBackend = "tensorrt"
)

// PipelineStage 管道阶段类型
type PipelineStage string

const (
	StageFilter    PipelineStage = "filter"
	StageTransform PipelineStage = "transform"
	StageAggregate PipelineStage = "aggregate"
	StageEnrich    PipelineStage = "enrich"
	StageSink      PipelineStage = "sink"
)

// Priority 任务优先级
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
	PriorityUrgent Priority = 3
)

// EdgeContainer 边缘容器
type EdgeContainer struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	State          ContainerState    `json:"state"`
	Labels         map[string]string `json:"labels,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	ResourceLimits *ResourceLimit    `json:"resource_limits,omitempty"`
	Ports          []PortBinding     `json:"ports,omitempty"`
	Volumes        []string          `json:"volumes,omitempty"`
	HealthCheck    *HealthCheck      `json:"health_check,omitempty"`
	Replicas       int               `json:"replicas"`
	CreatedAt      time.Time         `json:"created_at"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	RestartCount   int               `json:"restart_count"`
}

// ResourceLimit 资源限制
type ResourceLimit struct {
	CPU      float64 `json:"cpu"`       // CPU核心数
	MemoryMB int     `json:"memory_mb"` // 内存MB
	DiskMB   int     `json:"disk_mb"`   // 磁盘MB
	GPU      int     `json:"gpu"`       // GPU数量
	NetMbps  int     `json:"net_mbps"`  // 网络带宽Mbps
}

// PortBinding 端口绑定
type PortBinding struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	Type     string        `json:"type"` // http, tcp, exec
	Endpoint string        `json:"endpoint,omitempty"`
	Command  string        `json:"command,omitempty"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Retries  int           `json:"retries"`
}

// EdgeFunction 边缘函数
type EdgeFunction struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Runtime        RuntimeLanguage   `json:"runtime"`
	Code           string            `json:"code"`
	Handler        string            `json:"handler"`
	Timeout        time.Duration     `json:"timeout"`
	MemoryMB       int               `json:"memory_mb"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	Triggers       []FunctionTrigger `json:"triggers,omitempty"`
	Dependencies   []string          `json:"dependencies,omitempty"`
	MaxConcurrency int               `json:"max_concurrency"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// FunctionTrigger 函数触发器
type FunctionTrigger struct {
	Type   string            `json:"type"` // http, mqtt, schedule, iot
	Config map[string]string `json:"config"`
}

// IoTDevice IoT设备
type IoTDevice struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`     // sensor, actuator, gateway
	Protocol string            `json:"protocol"` // mqtt, coap, http, modbus
	Address  string            `json:"address"`
	Labels   map[string]string `json:"labels,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	LastSeen time.Time         `json:"last_seen"`
	Online   bool              `json:"online"`
}

// IoTMessage IoT消息
type IoTMessage struct {
	DeviceID  string                 `json:"device_id"`
	Topic     string                 `json:"topic"`
	Payload   map[string]interface{} `json:"payload"`
	QoS       int                    `json:"qos"`
	Timestamp time.Time              `json:"timestamp"`
	Retained  bool                   `json:"retained"`
}

// AIModel AI模型
type AIModel struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Backend     InferenceBackend `json:"backend"`
	ModelPath   string           `json:"model_path"`
	InputShape  []int            `json:"input_shape"`
	OutputShape []int            `json:"output_shape"`
	Labels      []string         `json:"labels,omitempty"`
	Framework   string           `json:"framework"` // onnx, tensorflow, pytorch, tflite
	Accuracy    float64          `json:"accuracy"`
	LatencyMs   float64          `json:"latency_ms"`
	CreatedAt   time.Time        `json:"created_at"`
}

// InferenceRequest 推理请求
type InferenceRequest struct {
	ModelID    string                 `json:"model_id"`
	InputData  map[string]interface{} `json:"input_data"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	BatchSize  int                    `json:"batch_size"`
}

// InferenceResult 推理结果
type InferenceResult struct {
	RequestID   string           `json:"request_id"`
	ModelID     string           `json:"model_id"`
	Predictions []Prediction     `json:"predictions"`
	LatencyMs   float64          `json:"latency_ms"`
	Backend     InferenceBackend `json:"backend"`
	Timestamp   time.Time        `json:"timestamp"`
}

// Prediction 预测结果
type Prediction struct {
	Label      string    `json:"label"`
	Confidence float64   `json:"confidence"`
	BBox       []float64 `json:"bbox,omitempty"` // [x1, y1, x2, y2]
}

// DataPipeline 数据处理管道
type DataPipeline struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Stages      []PipelineStage `json:"stages"`
	Source      DataSource      `json:"source"`
	Sinks       []DataSink      `json:"sinks"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   time.Time       `json:"created_at"`
}

// DataSource 数据源
type DataSource struct {
	Type   string            `json:"type"` // iot, mqtt, file, http
	Config map[string]string `json:"config"`
}

// DataSink 数据汇
type DataSink struct {
	Type   string            `json:"type"` // database, file, mqtt, http
	Config map[string]string `json:"config"`
}

// StageResult 阶段处理结果
type StageResult struct {
	Stage     PipelineStage `json:"stage"`
	Input     int           `json:"input"`
	Output    int           `json:"output"`
	LatencyMs float64       `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
}

// SchedulerTask 调度任务
type SchedulerTask struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	TargetID   string        `json:"target_id"`   // 容器或函数ID
	TargetType string        `json:"target_type"` // container, function
	Priority   Priority      `json:"priority"`
	Schedule   string        `json:"schedule"` // cron表达式
	Timeout    time.Duration `json:"timeout"`
	MaxRetries int           `json:"max_retries"`
	Enabled    bool          `json:"enabled"`
	LastRun    *time.Time    `json:"last_run,omitempty"`
	NextRun    *time.Time    `json:"next_run,omitempty"`
}

// ResourceQuota 资源配额
type ResourceQuota struct {
	Name         string  `json:"name"`
	MaxCPU       float64 `json:"max_cpu"`
	MaxMemoryMB  int     `json:"max_memory_mb"`
	MaxDiskMB    int     `json:"max_disk_mb"`
	MaxGPU       int     `json:"max_gpu"`
	UsedCPU      float64 `json:"used_cpu"`
	UsedMemoryMB int     `json:"used_memory_mb"`
	UsedDiskMB   int     `json:"used_disk_mb"`
	UsedGPU      int     `json:"used_gpu"`
}

// EdgePlatform 边缘计算平台
type EdgePlatform struct {
	mu                sync.RWMutex
	containers        map[string]*EdgeContainer
	functions         map[string]*EdgeFunction
	devices           map[string]*IoTDevice
	models            map[string]*AIModel
	pipelines         map[string]*DataPipeline
	tasks             map[string]*SchedulerTask
	quotas            map[string]*ResourceQuota
	invocations       map[string][]FunctionInvocation
	invocationCounter int64
}

// FunctionInvocation 函数调用记录
type FunctionInvocation struct {
	RequestID  string        `json:"request_id"`
	FunctionID string        `json:"function_id"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
	Status     string        `json:"status"` // success, error, timeout
	Output     string        `json:"output,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// NewEdgePlatform 创建边缘计算平台
func NewEdgePlatform() *EdgePlatform {
	return &EdgePlatform{
		containers:  make(map[string]*EdgeContainer),
		functions:   make(map[string]*EdgeFunction),
		devices:     make(map[string]*IoTDevice),
		models:      make(map[string]*AIModel),
		pipelines:   make(map[string]*DataPipeline),
		tasks:       make(map[string]*SchedulerTask),
		quotas:      make(map[string]*ResourceQuota),
		invocations: make(map[string][]FunctionInvocation),
	}
}

// === 容器编排 ===

// CreateContainer 创建边缘容器
func (ep *EdgePlatform) CreateContainer(container *EdgeContainer) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.containers[container.ID]; exists {
		return fmt.Errorf("容器 %s 已存在", container.ID)
	}

	container.State = StatePending
	container.CreatedAt = time.Now()
	if container.Replicas <= 0 {
		container.Replicas = 1
	}
	ep.containers[container.ID] = container
	return nil
}

// StartContainer 启动容器
func (ep *EdgePlatform) StartContainer(containerID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	container, exists := ep.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	if container.State == StateRunning {
		return fmt.Errorf("容器 %s 已在运行", containerID)
	}

	now := time.Now()
	container.State = StateRunning
	container.StartedAt = &now
	return nil
}

// StopContainer 停止容器
func (ep *EdgePlatform) StopContainer(containerID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	container, exists := ep.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	if container.State == StateStopped {
		return fmt.Errorf("容器 %s 已停止", containerID)
	}

	container.State = StateStopped
	return nil
}

// DeleteContainer 删除容器
func (ep *EdgePlatform) DeleteContainer(containerID string, force bool) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	container, exists := ep.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	if container.State == StateRunning && !force {
		return fmt.Errorf("容器 %s 正在运行，使用 force=true 强制删除", containerID)
	}

	delete(ep.containers, containerID)
	return nil
}

// GetContainer 获取容器信息
func (ep *EdgePlatform) GetContainer(containerID string) (*EdgeContainer, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	container, exists := ep.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}
	return container, nil
}

// ListContainers 列出容器
func (ep *EdgePlatform) ListContainers(state ContainerState) []*EdgeContainer {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	containers := make([]*EdgeContainer, 0)
	for _, c := range ep.containers {
		if state != "" && c.State != state {
			continue
		}
		containers = append(containers, c)
	}
	return containers
}

// ScaleContainer 扩缩容
func (ep *EdgePlatform) ScaleContainer(containerID string, replicas int) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	container, exists := ep.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	if replicas < 0 {
		return fmt.Errorf("副本数不能为负数")
	}

	container.State = StateScaling
	container.Replicas = replicas

	// 模拟扩缩容完成
	container.State = StateRunning
	return nil
}

// === 函数计算 (FaaS) ===

// DeployFunction 部署函数
func (ep *EdgePlatform) DeployFunction(fn *EdgeFunction) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.functions[fn.ID]; exists {
		return fmt.Errorf("函数 %s 已存在", fn.ID)
	}

	now := time.Now()
	fn.CreatedAt = now
	fn.UpdatedAt = now
	if fn.Timeout == 0 {
		fn.Timeout = 30 * time.Second
	}
	if fn.MemoryMB == 0 {
		fn.MemoryMB = 128
	}
	if fn.MaxConcurrency <= 0 {
		fn.MaxConcurrency = 10
	}
	ep.functions[fn.ID] = fn
	return nil
}

// UpdateFunction 更新函数
func (ep *EdgePlatform) UpdateFunction(fn *EdgeFunction) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.functions[fn.ID]; !exists {
		return fmt.Errorf("函数 %s 不存在", fn.ID)
	}

	fn.UpdatedAt = time.Now()
	ep.functions[fn.ID] = fn
	return nil
}

// DeleteFunction 删除函数
func (ep *EdgePlatform) DeleteFunction(functionID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.functions[functionID]; !exists {
		return fmt.Errorf("函数 %s 不存在", functionID)
	}

	delete(ep.functions, functionID)
	delete(ep.invocations, functionID)
	return nil
}

// InvokeFunction 调用函数
func (ep *EdgePlatform) InvokeFunction(functionID string, input map[string]interface{}) (*FunctionInvocation, error) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	fn, exists := ep.functions[functionID]
	if !exists {
		return nil, fmt.Errorf("函数 %s 不存在", functionID)
	}

	ep.invocationCounter++
	startTime := time.Now()

	// 模拟函数执行
	invocation := FunctionInvocation{
		RequestID:  fmt.Sprintf("req-%d", ep.invocationCounter),
		FunctionID: functionID,
		StartTime:  startTime,
		EndTime:    time.Now(),
		Status:     "success",
		Output:     fmt.Sprintf("函数 %s 执行成功", fn.Name),
	}
	invocation.Duration = invocation.EndTime.Sub(invocation.StartTime)

	ep.invocations[functionID] = append(ep.invocations[functionID], invocation)
	return &invocation, nil
}

// GetFunction 获取函数信息
func (ep *EdgePlatform) GetFunction(functionID string) (*EdgeFunction, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	fn, exists := ep.functions[functionID]
	if !exists {
		return nil, fmt.Errorf("函数 %s 不存在", functionID)
	}
	return fn, nil
}

// ListFunctions 列出函数
func (ep *EdgePlatform) ListFunctions(runtime RuntimeLanguage) []*EdgeFunction {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	fns := make([]*EdgeFunction, 0)
	for _, fn := range ep.functions {
		if runtime != "" && fn.Runtime != runtime {
			continue
		}
		fns = append(fns, fn)
	}
	return fns
}

// GetFunctionInvocations 获取函数调用记录
func (ep *EdgePlatform) GetFunctionInvocations(functionID string) []FunctionInvocation {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	return ep.invocations[functionID]
}

// === IoT数据采集 ===

// RegisterDevice 注册IoT设备
func (ep *EdgePlatform) RegisterDevice(device *IoTDevice) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.devices[device.ID]; exists {
		return fmt.Errorf("设备 %s 已注册", device.ID)
	}

	device.LastSeen = time.Now()
	ep.devices[device.ID] = device
	return nil
}

// UnregisterDevice 注销IoT设备
func (ep *EdgePlatform) UnregisterDevice(deviceID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.devices[deviceID]; !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	delete(ep.devices, deviceID)
	return nil
}

// GetDevice 获取设备信息
func (ep *EdgePlatform) GetDevice(deviceID string) (*IoTDevice, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	device, exists := ep.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}
	return device, nil
}

// ListDevices 列出设备
func (ep *EdgePlatform) ListDevices(deviceType string) []*IoTDevice {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	devices := make([]*IoTDevice, 0)
	for _, d := range ep.devices {
		if deviceType != "" && d.Type != deviceType {
			continue
		}
		devices = append(devices, d)
	}
	return devices
}

// UpdateDeviceStatus 更新设备状态
func (ep *EdgePlatform) UpdateDeviceStatus(deviceID string, online bool) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	device, exists := ep.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	device.Online = online
	device.LastSeen = time.Now()
	return nil
}

// ProcessIoTMessage 处理IoT消息
func (ep *EdgePlatform) ProcessIoTMessage(msg *IoTMessage) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.devices[msg.DeviceID]; !exists {
		return fmt.Errorf("设备 %s 未注册", msg.DeviceID)
	}

	msg.Timestamp = time.Now()

	// 更新设备最后在线时间
	ep.devices[msg.DeviceID].LastSeen = msg.Timestamp
	ep.devices[msg.DeviceID].Online = true

	return nil
}

// GetOnlineDevices 获取在线设备
func (ep *EdgePlatform) GetOnlineDevices() []*IoTDevice {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	devices := make([]*IoTDevice, 0)
	for _, d := range ep.devices {
		if d.Online {
			devices = append(devices, d)
		}
	}
	return devices
}

// === 边缘AI推理 ===

// RegisterModel 注册AI模型
func (ep *EdgePlatform) RegisterModel(model *AIModel) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.models[model.ID]; exists {
		return fmt.Errorf("模型 %s 已存在", model.ID)
	}

	model.CreatedAt = time.Now()
	ep.models[model.ID] = model
	return nil
}

// UnregisterModel 注销AI模型
func (ep *EdgePlatform) UnregisterModel(modelID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.models[modelID]; !exists {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	delete(ep.models, modelID)
	return nil
}

// GetModel 获取模型信息
func (ep *EdgePlatform) GetModel(modelID string) (*AIModel, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	model, exists := ep.models[modelID]
	if !exists {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}
	return model, nil
}

// ListModels 列出模型
func (ep *EdgePlatform) ListModels(backend InferenceBackend) []*AIModel {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	models := make([]*AIModel, 0)
	for _, m := range ep.models {
		if backend != "" && m.Backend != backend {
			continue
		}
		models = append(models, m)
	}
	return models
}

// RunInference 执行推理
func (ep *EdgePlatform) RunInference(req *InferenceRequest) (*InferenceResult, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	model, exists := ep.models[req.ModelID]
	if !exists {
		return nil, fmt.Errorf("模型 %s 不存在", req.ModelID)
	}

	startTime := time.Now()

	// 模拟推理
	result := &InferenceResult{
		RequestID: fmt.Sprintf("inf-%d", time.Now().UnixNano()),
		ModelID:   req.ModelID,
		Predictions: []Prediction{
			{
				Label:      model.Labels[0],
				Confidence: 0.95,
			},
		},
		Backend:   model.Backend,
		Timestamp: time.Now(),
	}
	result.LatencyMs = float64(time.Since(startTime).Milliseconds())

	// 更新模型延迟统计
	model.LatencyMs = (model.LatencyMs + result.LatencyMs) / 2

	return result, nil
}

// === 数据预处理管道 ===

// CreatePipeline 创建数据管道
func (ep *EdgePlatform) CreatePipeline(pipeline *DataPipeline) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.pipelines[pipeline.ID]; exists {
		return fmt.Errorf("管道 %s 已存在", pipeline.ID)
	}

	pipeline.CreatedAt = time.Now()
	ep.pipelines[pipeline.ID] = pipeline
	return nil
}

// DeletePipeline 删除数据管道
func (ep *EdgePlatform) DeletePipeline(pipelineID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.pipelines[pipelineID]; !exists {
		return fmt.Errorf("管道 %s 不存在", pipelineID)
	}

	delete(ep.pipelines, pipelineID)
	return nil
}

// GetPipeline 获取管道信息
func (ep *EdgePlatform) GetPipeline(pipelineID string) (*DataPipeline, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	pipeline, exists := ep.pipelines[pipelineID]
	if !exists {
		return nil, fmt.Errorf("管道 %s 不存在", pipelineID)
	}
	return pipeline, nil
}

// ListPipelines 列出管道
func (ep *EdgePlatform) ListPipelines(enabledOnly bool) []*DataPipeline {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	pipelines := make([]*DataPipeline, 0)
	for _, p := range ep.pipelines {
		if enabledOnly && !p.Enabled {
			continue
		}
		pipelines = append(pipelines, p)
	}
	return pipelines
}

// EnablePipeline 启用管道
func (ep *EdgePlatform) EnablePipeline(pipelineID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	pipeline, exists := ep.pipelines[pipelineID]
	if !exists {
		return fmt.Errorf("管道 %s 不存在", pipelineID)
	}

	pipeline.Enabled = true
	return nil
}

// DisablePipeline 禁用管道
func (ep *EdgePlatform) DisablePipeline(pipelineID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	pipeline, exists := ep.pipelines[pipelineID]
	if !exists {
		return fmt.Errorf("管道 %s 不存在", pipelineID)
	}

	pipeline.Enabled = false
	return nil
}

// ProcessPipelineData 处理管道数据
func (ep *EdgePlatform) ProcessPipelineData(pipelineID string, data []map[string]interface{}) ([]StageResult, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	pipeline, exists := ep.pipelines[pipelineID]
	if !exists {
		return nil, fmt.Errorf("管道 %s 不存在", pipelineID)
	}

	if !pipeline.Enabled {
		return nil, fmt.Errorf("管道 %s 未启用", pipelineID)
	}

	results := make([]StageResult, 0)
	inputCount := len(data)

	for _, stage := range pipeline.Stages {
		startTime := time.Now()

		// 模拟阶段处理
		result := StageResult{
			Stage:     stage,
			Input:     inputCount,
			Output:    inputCount, // 简化：输出等于输入
			LatencyMs: float64(time.Since(startTime).Milliseconds()),
		}
		results = append(results, result)
	}

	return results, nil
}

// === 资源限制和调度 ===

// CreateResourceQuota 创建资源配额
func (ep *EdgePlatform) CreateResourceQuota(quota *ResourceQuota) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.quotas[quota.Name]; exists {
		return fmt.Errorf("配额 %s 已存在", quota.Name)
	}

	ep.quotas[quota.Name] = quota
	return nil
}

// GetResourceQuota 获取资源配额
func (ep *EdgePlatform) GetResourceQuota(quotaName string) (*ResourceQuota, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	quota, exists := ep.quotas[quotaName]
	if !exists {
		return nil, fmt.Errorf("配额 %s 不存在", quotaName)
	}
	return quota, nil
}

// UpdateResourceUsage 更新资源使用
func (ep *EdgePlatform) UpdateResourceUsage(quotaName string, cpu float64, memoryMB, diskMB, gpu int) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	quota, exists := ep.quotas[quotaName]
	if !exists {
		return fmt.Errorf("配额 %s 不存在", quotaName)
	}

	if quota.UsedCPU+cpu > quota.MaxCPU {
		return fmt.Errorf("CPU配额不足: 请求 %.2f，剩余 %.2f", cpu, quota.MaxCPU-quota.UsedCPU)
	}
	if quota.UsedMemoryMB+memoryMB > quota.MaxMemoryMB {
		return fmt.Errorf("内存配额不足: 请求 %dMB，剩余 %dMB", memoryMB, quota.MaxMemoryMB-quota.UsedMemoryMB)
	}

	quota.UsedCPU += cpu
	quota.UsedMemoryMB += memoryMB
	quota.UsedDiskMB += diskMB
	quota.UsedGPU += gpu
	return nil
}

// CheckResourceAvailability 检查资源可用性
func (ep *EdgePlatform) CheckResourceAvailability(quotaName string, cpu float64, memoryMB, diskMB, gpu int) bool {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	quota, exists := ep.quotas[quotaName]
	if !exists {
		return false
	}

	return quota.UsedCPU+cpu <= quota.MaxCPU &&
		quota.UsedMemoryMB+memoryMB <= quota.MaxMemoryMB &&
		quota.UsedDiskMB+diskMB <= quota.MaxDiskMB &&
		quota.UsedGPU+gpu <= quota.MaxGPU
}

// CreateSchedulerTask 创建调度任务
func (ep *EdgePlatform) CreateSchedulerTask(task *SchedulerTask) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.tasks[task.ID]; exists {
		return fmt.Errorf("任务 %s 已存在", task.ID)
	}

	task.Enabled = true
	ep.tasks[task.ID] = task
	return nil
}

// DeleteSchedulerTask 删除调度任务
func (ep *EdgePlatform) DeleteSchedulerTask(taskID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if _, exists := ep.tasks[taskID]; !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	delete(ep.tasks, taskID)
	return nil
}

// GetSchedulerTask 获取调度任务
func (ep *EdgePlatform) GetSchedulerTask(taskID string) (*SchedulerTask, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	task, exists := ep.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListSchedulerTasks 列出调度任务
func (ep *EdgePlatform) ListSchedulerTasks(enabledOnly bool) []*SchedulerTask {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	tasks := make([]*SchedulerTask, 0)
	for _, t := range ep.tasks {
		if enabledOnly && !t.Enabled {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks
}

// EnableSchedulerTask 启用调度任务
func (ep *EdgePlatform) EnableSchedulerTask(taskID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	task, exists := ep.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	task.Enabled = true
	return nil
}

// DisableSchedulerTask 禁用调度任务
func (ep *EdgePlatform) DisableSchedulerTask(taskID string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	task, exists := ep.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	task.Enabled = false
	return nil
}

// GetPlatformStats 获取平台统计
func (ep *EdgePlatform) GetPlatformStats() map[string]interface{} {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	stats := map[string]interface{}{
		"total_containers": len(ep.containers),
		"total_functions":  len(ep.functions),
		"total_devices":    len(ep.devices),
		"total_models":     len(ep.models),
		"total_pipelines":  len(ep.pipelines),
		"total_tasks":      len(ep.tasks),
		"total_quotas":     len(ep.quotas),
	}

	runningContainers := 0
	for _, c := range ep.containers {
		if c.State == StateRunning {
			runningContainers++
		}
	}
	stats["running_containers"] = runningContainers

	onlineDevices := 0
	for _, d := range ep.devices {
		if d.Online {
			onlineDevices++
		}
	}
	stats["online_devices"] = onlineDevices

	enabledPipelines := 0
	for _, p := range ep.pipelines {
		if p.Enabled {
			enabledPipelines++
		}
	}
	stats["enabled_pipelines"] = enabledPipelines

	totalInvocations := 0
	for _, invocations := range ep.invocations {
		totalInvocations += len(invocations)
	}
	stats["total_invocations"] = totalInvocations

	return stats
}
