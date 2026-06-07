// Package edgeaiinference 提供边缘AI推理网关功能
package edgeaiinference

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 推理管理器
type Manager struct {
	mu           sync.RWMutex
	models       map[string]*AIModel
	devices      map[string]*ComputeDevice
	queues       map[string]*InferenceQueue
	results      map[string]*InferenceResult
	quotas       map[string]*ResourceQuota
	metrics      *InferenceMetrics
	events       []InferenceEvent
	dataDir      string
	schedulerCfg *SchedulerConfig
	stopChan     chan struct{}
	running      bool
	subscribers  []chan *InferenceEvent
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	DataDir      string
	SchedulerCfg *SchedulerConfig
}

// NewManager 创建推理管理器
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	if cfg.SchedulerCfg == nil {
		cfg.SchedulerCfg = &SchedulerConfig{
			Strategy:        "priority",
			MaxBatchSize:    8,
			BatchTimeoutMs:  100,
			MaxQueueSize:    1000,
			EnablePreemption: false,
		}
	}

	m := &Manager{
		models:       make(map[string]*AIModel),
		devices:      make(map[string]*ComputeDevice),
		queues:       make(map[string]*InferenceQueue),
		results:      make(map[string]*InferenceResult),
		quotas:       make(map[string]*ResourceQuota),
		metrics:      &InferenceMetrics{},
		events:       make([]InferenceEvent, 0),
		dataDir:      cfg.DataDir,
		schedulerCfg: cfg.SchedulerCfg,
		stopChan:     make(chan struct{}),
		subscribers:  make([]chan *InferenceEvent, 0),
	}

	if m.dataDir != "" {
		if err := os.MkdirAll(m.dataDir, 0750); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
		if err := m.loadConfig(); err != nil {
			fmt.Printf("加载配置失败: %v\n", err)
		}
	}

	return m, nil
}

// Start 启动管理器
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.schedulerLoop()
	go m.metricsLoop()
	go m.healthCheckLoop()
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		close(m.stopChan)
		m.running = false
		_ = m.saveConfig()
	}
}

// RegisterDevice 注册计算设备
func (m *Manager) RegisterDevice(device *ComputeDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		return fmt.Errorf("设备ID不能为空")
	}

	device.Available = true
	if device.Models == nil {
		device.Models = make([]string, 0)
	}

	m.devices[device.ID] = device
	m.addEvent("device_registered", "", fmt.Sprintf("设备 %s 已注册", device.Name), "info")

	return nil
}

// UnregisterDevice 注销计算设备
func (m *Manager) UnregisterDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[deviceID]; !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	delete(m.devices, deviceID)
	m.addEvent("device_unregistered", "", fmt.Sprintf("设备 %s 已注销", deviceID), "info")

	return nil
}

// RegisterModel 注册模型
func (m *Manager) RegisterModel(model *AIModel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if model.ID == "" {
		return fmt.Errorf("模型ID不能为空")
	}

	model.Status = ModelStatusUnloaded
	model.UseCount = 0

	m.models[model.ID] = model
	m.addEvent("model_registered", model.ID, fmt.Sprintf("模型 %s 已注册", model.Name), "info")

	return nil
}

// UnregisterModel 注销模型
func (m *Manager) UnregisterModel(modelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, exists := m.models[modelID]
	if !exists {
		return fmt.Errorf("模型不存在: %s", modelID)
	}

	if model.Status == ModelStatusRunning {
		return fmt.Errorf("模型正在运行中，无法注销: %s", modelID)
	}

	delete(m.models, modelID)
	m.addEvent("model_unregistered", modelID, fmt.Sprintf("模型 %s 已注销", model.Name), "info")

	return nil
}

// LoadModel 加载模型到设备
func (m *Manager) LoadModel(modelID, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, exists := m.models[modelID]
	if !exists {
		return fmt.Errorf("模型不存在: %s", modelID)
	}

	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if !device.Available {
		return fmt.Errorf("设备不可用: %s", deviceID)
	}

	availableMem := device.MemoryMB - device.UsedMemoryMB
	if model.MemoryMB > availableMem {
		return fmt.Errorf("设备内存不足: 需要 %dMB, 可用 %dMB", model.MemoryMB, availableMem)
	}

	model.Status = ModelStatusLoading
	model.DeviceID = deviceID

	// 模拟加载过程
	model.Status = ModelStatusReady
	now := time.Now()
	model.LoadedAt = &now
	device.UsedMemoryMB += model.MemoryMB
	device.Models = append(device.Models, modelID)

	m.addEvent("model_loaded", modelID, fmt.Sprintf("模型 %s 已加载到设备 %s", model.Name, device.Name), "info")

	return nil
}

// UnloadModel 从设备卸载模型
func (m *Manager) UnloadModel(modelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, exists := m.models[modelID]
	if !exists {
		return fmt.Errorf("模型不存在: %s", modelID)
	}

	if model.Status == ModelStatusRunning {
		return fmt.Errorf("模型正在运行中，无法卸载: %s", modelID)
	}

	if model.DeviceID != "" {
		if device, ok := m.devices[model.DeviceID]; ok {
			device.UsedMemoryMB -= model.MemoryMB
			// 从设备模型列表中移除
			for i, mid := range device.Models {
				if mid == modelID {
					device.Models = append(device.Models[:i], device.Models[i+1:]...)
					break
				}
			}
		}
	}

	model.Status = ModelStatusUnloaded
	model.DeviceID = ""

	m.addEvent("model_unloaded", modelID, fmt.Sprintf("模型 %s 已卸载", model.Name), "info")

	return nil
}

// SubmitInference 提交推理请求
func (m *Manager) SubmitInference(req *InferenceRequest) (*InferenceResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, exists := m.models[req.ModelID]
	if !exists {
		return nil, fmt.Errorf("模型不存在: %s", req.ModelID)
	}

	if model.Status != ModelStatusReady {
		return nil, fmt.Errorf("模型未就绪: 当前状态 %s", model.Status)
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	req.SubmittedAt = time.Now()

	result := &InferenceResult{
		ID:        fmt.Sprintf("res-%d", time.Now().UnixNano()),
		RequestID: req.ID,
		ModelID:   req.ModelID,
		Status:    InferenceStatusQueued,
		DeviceID:  model.DeviceID,
		StartedAt: time.Now(),
	}

	m.results[result.ID] = result
	m.addEvent("inference_submitted", req.ModelID, "推理请求已提交", "info")

	return result, nil
}

// GetInferenceResult 获取推理结果
func (m *Manager) GetInferenceResult(resultID string) (*InferenceResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.results[resultID]
	if !exists {
		return nil, fmt.Errorf("推理结果不存在: %s", resultID)
	}

	return result, nil
}

// GetModel 获取模型信息
func (m *Manager) GetModel(modelID string) (*AIModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	model, exists := m.models[modelID]
	if !exists {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}

	return model, nil
}

// ListModels 列出所有模型
func (m *Manager) ListModels() []*AIModel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	models := make([]*AIModel, 0, len(m.models))
	for _, m := range m.models {
		models = append(models, m)
	}
	return models
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []*ComputeDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*ComputeDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetMetrics 获取推理指标
func (m *Manager) GetMetrics() *InferenceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.metrics
}

// GetEvents 获取推理事件
func (m *Manager) GetEvents(limit int) []InferenceEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}

	return m.events[start:]
}

// Subscribe 订阅推理事件
func (m *Manager) Subscribe() <-chan *InferenceEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *InferenceEvent, 100)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

// schedulerLoop 调度循环
func (m *Manager) schedulerLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.processQueue()
		}
	}
}

// metricsLoop 指标收集循环
func (m *Manager) metricsLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// healthCheckLoop 健康检查循环
func (m *Manager) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkDeviceHealth()
		}
	}
}

// processQueue 处理推理队列
func (m *Manager) processQueue() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, result := range m.results {
		if result.Status == InferenceStatusQueued {
			result.Status = InferenceStatusProcessing
			// 模拟推理完成
			result.Status = InferenceStatusCompleted
			now := time.Now()
			result.CompletedAt = &now
			result.LatencyMs = 50

			if model, ok := m.models[result.ModelID]; ok {
				model.UseCount++
				model.Status = ModelStatusReady
				now := time.Now()
				model.LastUsedAt = &now
			}
		}
	}
}

// collectMetrics 收集指标
func (m *Manager) collectMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var totalLatency float64
	var count int64
	successCount := int64(0)
	failCount := int64(0)

	for _, result := range m.results {
		switch result.Status {
		case InferenceStatusCompleted:
			successCount++
			totalLatency += float64(result.LatencyMs)
			count++
		case InferenceStatusFailed:
			failCount++
		}
	}

	m.metrics = &InferenceMetrics{
		TotalRequests:   successCount + failCount,
		SuccessRequests: successCount,
		FailedRequests:  failCount,
		Timestamp:       time.Now(),
	}

	if count > 0 {
		m.metrics.AvgLatencyMs = totalLatency / float64(count)
	}

	// 计算GPU指标
	for _, device := range m.devices {
		if device.Type == DeviceTypeGPU {
			m.metrics.GPUMemoryUsedMB += device.UsedMemoryMB
			m.metrics.GPUMemoryTotalMB += device.MemoryMB
			m.metrics.GPUUtilization = device.Utilization
		}
	}
}

// checkDeviceHealth 检查设备健康状态
func (m *Manager) checkDeviceHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, device := range m.devices {
		// 检查温度
		if device.Temperature > 85.0 {
			device.Available = false
			m.addEvent("device_overheat", "", fmt.Sprintf("设备 %s 温度过高: %.1f°C", device.Name, device.Temperature), "warning")
		}

		// 检查内存使用率
		if device.MemoryMB > 0 {
			usage := float64(device.UsedMemoryMB) / float64(device.MemoryMB) * 100
			if usage > 95.0 {
				m.addEvent("device_memory_critical", "", fmt.Sprintf("设备 %s 内存使用率: %.1f%%", device.Name, usage), "warning")
			}
		}
	}
}

// addEvent 添加事件
func (m *Manager) addEvent(eventType, modelID, message, severity string) {
	event := InferenceEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Type:      eventType,
		ModelID:   modelID,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
	}

	m.events = append(m.events, event)

	if len(m.events) > 1000 {
		m.events = m.events[len(m.events)-1000:]
	}

	for _, sub := range m.subscribers {
		select {
		case sub <- &event:
		default:
		}
	}
}

// saveConfig 保存配置
func (m *Manager) saveConfig() error {
	if m.dataDir == "" {
		return nil
	}

	data := map[string]interface{}{
		"models":      m.models,
		"quotas":      m.quotas,
		"schedulerCfg": m.schedulerCfg,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	configPath := filepath.Join(m.dataDir, "inference.json")
	return os.WriteFile(configPath, jsonData, 0640)
}

// loadConfig 加载配置
func (m *Manager) loadConfig() error {
	if m.dataDir == "" {
		return nil
	}

	configPath := filepath.Join(m.dataDir, "inference.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取配置失败: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	return nil
}
