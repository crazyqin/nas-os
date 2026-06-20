package smartedge

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// DeviceType 设备类型.
type DeviceType string

const (
	DeviceTypeSensor  DeviceType = "sensor"
	DeviceTypeCamera  DeviceType = "camera"
	DeviceTypeGateway DeviceType = "gateway"
	DeviceTypeRouter  DeviceType = "router"
	DeviceTypeNVR     DeviceType = "nvr"
	DeviceTypeCustom  DeviceType = "custom"
)

// DeviceState 设备状态.
type DeviceState string

const (
	DeviceStateOnline    DeviceState = "online"
	DeviceStateOffline   DeviceState = "offline"
	DeviceStateProvisioning DeviceState = "provisioning"
	DeviceStateError     DeviceState = "error"
)

// IoTDevice IoT设备.
type IoTDevice struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        DeviceType        `json:"type"`
	State       DeviceState       `json:"state"`
	Address     string            `json:"address"`
	MACAddress  string            `json:"mac_address"`
	Firmware    string            `json:"firmware"`
	LastSeen    time.Time         `json:"last_seen"`
	DataPoints  int64             `json:"data_points"`
	Metadata    map[string]string `json:"metadata"`
}

// Data采集 数据点.
type DataPoint struct {
	DeviceID    string      `json:"device_id"`
	Timestamp   time.Time   `json:"timestamp"`
	Metric      string      `json:"metric"`
	Value       float64     `json:"value"`
	Unit        string      `json:"unit"`
	Quality     int         `json:"quality"` // 0-100
	Tags        map[string]string `json:"tags"`
}

// EdgeRule 边缘规则.
type EdgeRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DeviceType  DeviceType `json:"device_type"`
	Metric      string `json:"metric"`
	Condition   string `json:"condition"` // gt/lt/eq/between
	Threshold   float64 `json:"threshold"`
	Action      string `json:"action"` // alert/store/forward
	Enabled     bool    `json:"enabled"`
}

// DataPipeline 数据管道.
type DataPipeline struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Sources    []string `json:"sources"`    // device IDs
	Transforms []string `json:"transforms"` // transform functions
	Targets    []string `json:"targets"`    // storage/forward targets
	Status     string   `json:"status"`
	Processed  int64    `json:"processed"`
}

// Engine 边缘计算网关引擎.
type Engine struct {
	devices    map[string]*IoTDevice
	rules      map[string]*EdgeRule
	pipelines  map[string]*DataPipeline
	dataBuffer map[string][]*DataPoint // deviceID -> data points
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewEngine 创建边缘计算网关引擎.
func NewEngine(logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Engine{
		devices:    make(map[string]*IoTDevice),
		rules:      make(map[string]*EdgeRule),
		pipelines:  make(map[string]*DataPipeline),
		dataBuffer: make(map[string][]*DataPoint),
		logger:     logger,
	}
}

// RegisterDevice 注册IoT设备.
func (e *Engine) RegisterDevice(device *IoTDevice) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if device.ID == "" {
		return ErrInvalidDeviceID
	}
	device.LastSeen = time.Now()
	device.State = DeviceStateOnline
	if device.Metadata == nil {
		device.Metadata = make(map[string]string)
	}
	e.devices[device.ID] = device
	e.logger.Info("设备已注册",
		zap.String("id", device.ID),
		zap.String("type", string(device.Type)),
	)
	return nil
}

// GetDevice 获取设备.
func (e *Engine) GetDevice(id string) (*IoTDevice, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	d, ok := e.devices[id]
	return d, ok
}

// ListDevices 列出设备.
func (e *Engine) ListDevices() []*IoTDevice {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	devices := make([]*IoTDevice, 0, len(e.devices))
	for _, d := range e.devices {
		devices = append(devices, d)
	}
	return devices
}

// IngestData 数据采集.
func (e *Engine) IngestData(point *DataPoint) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if _, ok := e.devices[point.DeviceID]; !ok {
		return ErrDeviceNotFound
	}
	
	if point.Timestamp.IsZero() {
		point.Timestamp = time.Now()
	}
	
	e.dataBuffer[point.DeviceID] = append(e.dataBuffer[point.DeviceID], point)
	e.devices[point.DeviceID].DataPoints++
	e.devices[point.DeviceID].LastSeen = time.Now()
	
	// 检查边缘规则
	e.evaluateRules(point)
	
	return nil
}

// GetDataBuffer 获取数据缓冲.
func (e *Engine) GetDataBuffer(deviceID string) []*DataPoint {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.dataBuffer[deviceID]
}

// CreateRule 创建边缘规则.
func (e *Engine) CreateRule(rule *EdgeRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if rule.ID == "" {
		return ErrInvalidRuleID
	}
	e.rules[rule.ID] = rule
	return nil
}

// evaluateRules 评估规则.
func (e *Engine) evaluateRules(point *DataPoint) {
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Metric != point.Metric {
			continue
		}
		
		triggered := false
		switch rule.Condition {
		case "gt":
			triggered = point.Value > rule.Threshold
		case "lt":
			triggered = point.Value < rule.Threshold
		case "eq":
			triggered = point.Value == rule.Threshold
		}
		
		if triggered {
			e.logger.Warn("边缘规则触发",
				zap.String("rule", rule.Name),
				zap.String("device", point.DeviceID),
				zap.Float64("value", point.Value),
			)
		}
	}
}

// CreatePipeline 创建数据管道.
func (e *Engine) CreatePipeline(pipeline *DataPipeline) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if pipeline.ID == "" {
		return ErrInvalidPipelineID
	}
	pipeline.Status = "active"
	e.pipelines[pipeline.ID] = pipeline
	return nil
}

// GetEdgeStats 获取边缘统计.
func (e *Engine) GetEdgeStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	totalDataPoints := int64(0)
	onlineDevices := 0
	
	for _, d := range e.devices {
		totalDataPoints += d.DataPoints
		if d.State == DeviceStateOnline {
			onlineDevices++
		}
	}
	
	return map[string]interface{}{
		"total_devices":    len(e.devices),
		"online_devices":   onlineDevices,
		"total_data_points": totalDataPoints,
		"active_pipelines": len(e.pipelines),
		"active_rules":     len(e.rules),
	}
}
