// Package edgecompute IoT 数据采集和边缘 AI 推理扩展
package edgecompute

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// IoTDevice IoT 设备
type IoTDevice struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // sensor, actuator, gateway
	Protocol    string            `json:"protocol"` // mqtt, coap, http
	Endpoint    string            `json:"endpoint"`
	Status      string            `json:"status"` // online, offline
	LastSeen    time.Time         `json:"last_seen"`
	Data        map[string]interface{} `json:"data"`
	Metadata    map[string]string `json:"metadata"`
}

// IoTDataPoint IoT 数据点
type IoTDataPoint struct {
	DeviceID    string      `json:"device_id"`
	Timestamp   time.Time   `json:"timestamp"`
	Metric      string      `json:"metric"`
	Value       float64     `json:"value"`
	Unit        string      `json:"unit"`
	Quality     int         `json:"quality"` // 0-100
}

// AIModel AI 模型
type AIModel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Framework   string    `json:"framework"` // tflite, onnx, tensorrt
	Version     string    `json:"version"`
	InputShape  []int     `json:"input_shape"`
	OutputShape []int     `json:"output_shape"`
	Size        int64     `json:"size"` // bytes
	Path        string    `json:"path"`
	Status      string    `json:"status"` // loaded, unloaded, error
	LoadedAt    *time.Time `json:"loaded_at,omitempty"`
}

// InferenceRequest 推理请求
type InferenceRequest struct {
	ModelID string      `json:"model_id"`
	Input   interface{} `json:"input"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// InferenceResult 推理结果
type InferenceResult struct {
	ID          string        `json:"id"`
	ModelID     string        `json:"model_id"`
	Output      interface{}   `json:"output"`
	Latency     time.Duration `json:"latency"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
}

// DataFilter 数据过滤器
type DataFilter struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // threshold, range, anomaly
	Config      map[string]interface{} `json:"config"`
	Enabled     bool              `json:"enabled"`
}

// DataAggregator 数据聚合器
type DataAggregator struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Window      time.Duration `json:"window"`
	Function    string        `json:"function"` // avg, sum, min, max, count
	Metric      string        `json:"metric"`
	DeviceIDs   []string      `json:"device_ids"`
}

// OfflineCache 离线缓存
type OfflineCache struct {
	mu          sync.RWMutex
	data        []IoTDataPoint
	maxSize     int
	syncStatus  string
	lastSync    *time.Time
}

// NewOfflineCache 创建离线缓存
func NewOfflineCache(maxSize int) *OfflineCache {
	return &OfflineCache{
		data:    make([]IoTDataPoint, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add 添加数据到缓存
func (c *OfflineCache) Add(point IoTDataPoint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if len(c.data) >= c.maxSize {
		// 移除最旧的数据
		c.data = c.data[1:]
	}
	
	c.data = append(c.data, point)
}

// Flush 刷新缓存
func (c *OfflineCache) Flush() []IoTDataPoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	data := c.data
	c.data = make([]IoTDataPoint, 0, c.maxSize)
	now := time.Now()
	c.lastSync = &now
	
	return data
}

// Size 返回缓存大小
func (c *OfflineCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// IoTManager IoT 管理器
type IoTManager struct {
	devices   map[string]*IoTDevice
	cache     *OfflineCache
	filters   map[string]*DataFilter
	aggregators map[string]*DataAggregator
	mu        sync.RWMutex
}

// NewIoTManager 创建 IoT 管理器
func NewIoTManager(cacheSize int) *IoTManager {
	return &IoTManager{
		devices:     make(map[string]*IoTDevice),
		cache:       NewOfflineCache(cacheSize),
		filters:     make(map[string]*DataFilter),
		aggregators: make(map[string]*DataAggregator),
	}
}

// RegisterDevice 注册 IoT 设备
func (m *IoTManager) RegisterDevice(device *IoTDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if device.ID == "" {
		device.ID = fmt.Sprintf("iot_%d", time.Now().UnixNano())
	}
	
	device.Status = "online"
	device.LastSeen = time.Now()
	
	m.devices[device.ID] = device
	return nil
}

// IngestData 采集数据
func (m *IoTManager) IngestData(point IoTDataPoint) error {
	m.mu.RLock()
	device, ok := m.devices[point.DeviceID]
	m.mu.RUnlock()
	
	if !ok {
		return fmt.Errorf("device not found: %s", point.DeviceID)
	}
	
	// 更新设备最后见时间
	m.mu.Lock()
	device.LastSeen = time.Now()
	m.mu.Unlock()
	
	// 应用过滤器
	if m.applyFilters(point) {
		// 添加到缓存
		m.cache.Add(point)
	}
	
	return nil
}

// applyFilters 应用数据过滤器
func (m *IoTManager) applyFilters(point IoTDataPoint) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, filter := range m.filters {
		if !filter.Enabled {
			continue
		}
		
		switch filter.Type {
		case "threshold":
			if threshold, ok := filter.Config["threshold"].(float64); ok {
				if point.Value > threshold {
					return false
				}
			}
		case "range":
			if min, ok := filter.Config["min"].(float64); ok {
				if max, ok := filter.Config["max"].(float64); ok {
					if point.Value < min || point.Value > max {
						return false
					}
				}
			}
		}
	}
	
	return true
}

// GetDevices 获取所有设备
func (m *IoTManager) GetDevices() []*IoTDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	devices := make([]*IoTDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetDevice 获取设备
func (m *IoTManager) GetDevice(deviceID string) (*IoTDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}
	
	return device, nil
}

// AIManager AI 推理管理器
type AIManager struct {
	models    map[string]*AIModel
	results   []*InferenceResult
	mu        sync.RWMutex
}

// NewAIManager 创建 AI 管理器
func NewAIManager() *AIManager {
	return &AIManager{
		models: make(map[string]*AIModel),
	}
}

// LoadModel 加载模型
func (m *AIManager) LoadModel(model *AIModel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if model.ID == "" {
		model.ID = fmt.Sprintf("model_%d", time.Now().UnixNano())
	}
	
	model.Status = "loaded"
	now := time.Now()
	model.LoadedAt = &now
	
	m.models[model.ID] = model
	return nil
}

// UnloadModel 卸载模型
func (m *AIManager) UnloadModel(modelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	model, ok := m.models[modelID]
	if !ok {
		return fmt.Errorf("model not found: %s", modelID)
	}
	
	model.Status = "unloaded"
	model.LoadedAt = nil
	
	return nil
}

// RunInference 运行推理
func (m *AIManager) RunInference(request *InferenceRequest) (*InferenceResult, error) {
	m.mu.RLock()
	model, ok := m.models[request.ModelID]
	m.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("model not found: %s", request.ModelID)
	}
	
	if model.Status != "loaded" {
		return nil, fmt.Errorf("model not loaded: %s", model.Status)
	}
	
	start := time.Now()
	
	// 模拟推理
	result := &InferenceResult{
		ID:        fmt.Sprintf("infer_%d", time.Now().UnixNano()),
		ModelID:   request.ModelID,
		Output:    map[string]interface{}{"prediction": 0.95},
		Latency:   time.Since(start),
		Success:   true,
		Timestamp: time.Now(),
	}
	
	m.mu.Lock()
	m.results = append(m.results, result)
	m.mu.Unlock()
	
	return result, nil
}

// GetModels 获取所有模型
func (m *AIManager) GetModels() []*AIModel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	models := make([]*AIModel, 0, len(m.models))
	for _, model := range m.models {
		models = append(models, model)
	}
	return models
}

// GetModel 获取模型
func (m *AIManager) GetModel(modelID string) (*AIModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	model, ok := m.models[modelID]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	
	return model, nil
}

// MarshalJSON 序列化 IoT 管理器
func (m *IoTManager) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return json.Marshal(struct {
		Devices      int `json:"devices_count"`
		CacheSize    int `json:"cache_size"`
		Filters      int `json:"filters_count"`
		Aggregators  int `json:"aggregators_count"`
	}{
		Devices:     len(m.devices),
		CacheSize:   m.cache.Size(),
		Filters:     len(m.filters),
		Aggregators: len(m.aggregators),
	})
}

// MarshalJSON 序列化 AI 管理器
func (m *AIManager) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return json.Marshal(struct {
		Models  int `json:"models_count"`
		Results int `json:"results_count"`
	}{
		Models:  len(m.models),
		Results: len(m.results),
	})
}
