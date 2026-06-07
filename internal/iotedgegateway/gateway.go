// Package iotedgegateway 提供IoT边缘网关功能
// 学习 AWS IoT Greengrass 与 Azure IoT Edge 架构
// 支持设备管理、数据采集、边缘计算、规则引擎

package iotedgegateway

import (
	"fmt"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// DeviceProtocol 设备协议
type DeviceProtocol string

const (
	ProtocolMQTT   DeviceProtocol = "mqtt"
	ProtocolCoAP   DeviceProtocol = "coap"
	ProtocolHTTP   DeviceProtocol = "http"
	ProtocolModbus DeviceProtocol = "modbus"
	ProtocolOPCUA  DeviceProtocol = "opcua"
	ProtocolBLE    DeviceProtocol = "ble"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline       DeviceStatus = "online"
	DeviceStatusOffline      DeviceStatus = "offline"
	DeviceStatusProvisioning DeviceStatus = "provisioning"
	DeviceStatusError        DeviceStatus = "error"
)

// IoTDevice IoT设备
type IoTDevice struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Protocol   DeviceProtocol    `json:"protocol"`
	Status     DeviceStatus      `json:"status"`
	Address    string            `json:"address"`
	Metadata   map[string]string `json:"metadata"`
	LastSeen   time.Time         `json:"last_seen"`
	DataPoints int64             `json:"data_points"`
	ErrorCount int               `json:"error_count"`
	Tags       []string          `json:"tags"`
}

// DataPoint 数据点
type DataPoint struct {
	DeviceID  string                 `json:"device_id"`
	Timestamp time.Time              `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
	Quality   int                    `json:"quality"`
	Tags      map[string]string      `json:"tags"`
}

// Rule 规则
type Rule struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Enabled      bool         `json:"enabled"`
	Trigger      RuleTrigger  `json:"trigger"`
	Conditions   []Condition  `json:"conditions"`
	Actions      []RuleAction `json:"actions"`
	Priority     int          `json:"priority"`
	LastTrigger  *time.Time   `json:"last_trigger,omitempty"`
	TriggerCount int          `json:"trigger_count"`
}

// RuleTrigger 规则触发器
type RuleTrigger struct {
	Type     string `json:"type"` // data, schedule, event
	DeviceID string `json:"device_id,omitempty"`
	Sensor   string `json:"sensor,omitempty"`
	Schedule string `json:"schedule,omitempty"`
}

// Condition 条件
type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// RuleAction 规则动作
type RuleAction struct {
	Type   string                 `json:"type"` // alert, store, forward, transform
	Params map[string]interface{} `json:"params"`
}

// EdgeFunction 边缘函数
type EdgeFunction struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Runtime     string            `json:"runtime"` // python, nodejs, wasm
	Code        string            `json:"code"`
	Triggers    []string          `json:"triggers"`
	Params      map[string]string `json:"params"`
	Status      string            `json:"status"`
	LastRun     *time.Time        `json:"last_run,omitempty"`
	RunCount    int64             `json:"run_count"`
}

// DataPipeline 数据管道
type DataPipeline struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Source     PipelineSource `json:"source"`
	Processors []Processor    `json:"processors"`
	Sinks      []PipelineSink `json:"sinks"`
	Status     string         `json:"status"`
	Throughput float64        `json:"throughput"`
	LastError  string         `json:"last_error,omitempty"`
}

// PipelineSource 管道源
type PipelineSource struct {
	Type     string            `json:"type"`
	DeviceID string            `json:"device_id,omitempty"`
	Params   map[string]string `json:"params"`
}

// Processor 处理器
type Processor struct {
	Type   string            `json:"type"`
	Params map[string]string `json:"params"`
}

// PipelineSink 管道汇
type PipelineSink struct {
	Type   string            `json:"type"`
	Params map[string]string `json:"params"`
}

// Manager IoT边缘网关管理器
type Manager struct {
	mu          sync.RWMutex
	devices     map[string]*IoTDevice
	rules       map[string]*Rule
	functions   map[string]*EdgeFunction
	pipelines   map[string]*DataPipeline
	dataBuffer  []DataPoint
	bufferSize  int
	gatewayID   string
	gatewayName string
}

// NewManager 创建管理器
func NewManager(gatewayName string) *Manager {
	return &Manager{
		devices:     make(map[string]*IoTDevice),
		rules:       make(map[string]*Rule),
		functions:   make(map[string]*EdgeFunction),
		pipelines:   make(map[string]*DataPipeline),
		dataBuffer:  make([]DataPoint, 0),
		bufferSize:  10000,
		gatewayID:   fmt.Sprintf("gw_%d", time.Now().UnixNano()),
		gatewayName: gatewayName,
	}
}

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(device *IoTDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device.LastSeen = time.Now()
	if device.Status == "" {
		device.Status = DeviceStatusOnline
	}
	if device.Metadata == nil {
		device.Metadata = make(map[string]string)
	}

	m.devices[device.ID] = device
	return nil
}

// IngestData 接入数据
func (m *Manager) IngestData(data DataPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[data.DeviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", data.DeviceID)
	}

	device.LastSeen = time.Now()
	device.DataPoints++

	m.dataBuffer = append(m.dataBuffer, data)
	if len(m.dataBuffer) > m.bufferSize {
		m.dataBuffer = m.dataBuffer[1:]
	}

	return nil
}

// CreateRule 创建规则
func (m *Manager) CreateRule(rule *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rules[rule.ID] = rule
	return nil
}

// DeployFunction 部署边缘函数
func (m *Manager) DeployFunction(function *EdgeFunction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	function.Status = "deployed"
	m.functions[function.ID] = function
	return nil
}

// CreatePipeline 创建数据管道
func (m *Manager) CreatePipeline(pipeline *DataPipeline) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipeline.Status = "active"
	m.pipelines[pipeline.ID] = pipeline
	return nil
}

// GetDevice 获取设备
func (m *Manager) GetDevice(deviceID string) (*IoTDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	return device, nil
}

// ListDevices 列出设备
func (m *Manager) ListDevices(protocol DeviceProtocol, status DeviceStatus) []*IoTDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var devices []*IoTDevice
	for _, d := range m.devices {
		if (protocol == "" || d.Protocol == protocol) && (status == "" || d.Status == status) {
			devices = append(devices, d)
		}
	}

	return devices
}

// GetDataBuffer 获取数据缓冲
func (m *Manager) GetDataBuffer(deviceID string, limit int) []DataPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var data []DataPoint
	for i := len(m.dataBuffer) - 1; i >= 0 && len(data) < limit; i-- {
		if deviceID == "" || m.dataBuffer[i].DeviceID == deviceID {
			data = append(data, m.dataBuffer[i])
		}
	}

	return data
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"gateway_id":     m.gatewayID,
		"gateway_name":   m.gatewayName,
		"total_devices":  len(m.devices),
		"online_devices": 0,
		"rules":          len(m.rules),
		"functions":      len(m.functions),
		"pipelines":      len(m.pipelines),
		"data_buffered":  len(m.dataBuffer),
	}

	for _, d := range m.devices {
		if d.Status == DeviceStatusOnline {
			stats["online_devices"] = stats["online_devices"].(int) + 1
		}
	}

	return stats
}

// Close 关闭管理器
func (m *Manager) Close() error {
	return nil
}
