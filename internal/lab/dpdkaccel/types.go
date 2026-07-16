// Package dpdkaccel 实现 DPDK (Data Plane Development Kit) 高性能网络加速
// 支持用户态网络栈、零拷贝收发包、RSS 多队列、流量分类和 QoS 策略
package dpdkaccel

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrPortNotFound       = errors.New("port not found")
	ErrPortExists         = errors.New("port already exists")
	ErrPortNotStarted     = errors.New("port not started")
	ErrPortAlreadyStarted = errors.New("port already started")
	ErrQueueNotFound      = errors.New("queue not found")
	ErrInvalidConfig      = errors.New("invalid configuration")
	ErrManagerClosed      = errors.New("manager closed")
	ErrRuleExists         = errors.New("rule already exists")
	ErrRuleNotFound       = errors.New("rule not found")
)

// PortState 端口状态.
type PortState string

const (
	PortStateDown        PortState = "down"
	PortStateUp          PortState = "up"
	PortStateConfiguring PortState = "configuring"
	PortStateError       PortState = "error"
)

// RSSMode RSS (Receive Side Scaling) 模式.
type RSSMode string

const (
	RSSDisabled  RSSMode = "disabled"
	RSSDefault   RSSMode = "default"
	RSSSymmetric RSSMode = "symmetric"
	RSSCustom    RSSMode = "custom"
)

// TrafficClass 流量分类.
type TrafficClass string

const (
	TrafficClassBestEffort TrafficClass = "best_effort"
	TrafficClassBulk       TrafficClass = "bulk"
	TrafficClassLowLatency TrafficClass = "low_latency"
	TrafficClassControl    TrafficClass = "control"
)

// QueueType 队列类型.
type QueueType string

const (
	QueueTypeRX QueueType = "rx"
	QueueTypeTX QueueType = "tx"
)

// Port 网络端口.
type Port struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PCIeAddr    string    `json:"pcie_addr"`
	State       PortState `json:"state"`
	Speed       uint64    `json:"speed"` // Mbps
	MTU         uint16    `json:"mtu"`
	MACAddr     string    `json:"mac_addr"`
	RXQueues    int       `json:"rx_queues"`
	TXQueues    int       `json:"tx_queues"`
	RSSMode     RSSMode   `json:"rss_mode"`
	Promiscuous bool      `json:"promiscuous"`
	Stats       PortStats `json:"stats"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PortStats 端口统计.
type PortStats struct {
	RXPackets uint64 `json:"rx_packets"`
	TXPackets uint64 `json:"tx_packets"`
	RXBytes   uint64 `json:"rx_bytes"`
	TXBytes   uint64 `json:"tx_bytes"`
	RXErrors  uint64 `json:"rx_errors"`
	TXErrors  uint64 `json:"tx_errors"`
	RXDropped uint64 `json:"rx_dropped"`
	TXDropped uint64 `json:"tx_dropped"`
}

// Queue 队列配置.
type Queue struct {
	ID        int       `json:"id"`
	PortID    string    `json:"port_id"`
	Type      QueueType `json:"type"`
	Size      int       `json:"size"`       // 队列大小
	BurstSize int       `json:"burst_size"` // burst 大小
	Enabled   bool      `json:"enabled"`
}

// FlowRule 流表规则.
type FlowRule struct {
	ID           string       `json:"id"`
	PortID       string       `json:"port_id"`
	Priority     int          `json:"priority"`
	TrafficClass TrafficClass `json:"traffic_class"`
	SrcIP        string       `json:"src_ip,omitempty"`
	DstIP        string       `json:"dst_ip,omitempty"`
	SrcPort      uint16       `json:"src_port,omitempty"`
	DstPort      uint16       `json:"dst_port,omitempty"`
	Protocol     string       `json:"protocol,omitempty"`
	Action       string       `json:"action"` // allow, drop, redirect
	QueueID      int          `json:"queue_id,omitempty"`
	Stats        FlowStats    `json:"stats"`
	CreatedAt    time.Time    `json:"created_at"`
}

// FlowStats 流规则统计.
type FlowStats struct {
	MatchedPackets uint64 `json:"matched_packets"`
	MatchedBytes   uint64 `json:"matched_bytes"`
}

// Manager DPDK 加速管理器.
type Manager struct {
	mu        sync.RWMutex
	ports     map[string]*Port
	queues    map[string][]*Queue
	flowRules map[string]*FlowRule
	closed    bool
	stopCh    chan struct{}
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		ports:     make(map[string]*Port),
		queues:    make(map[string][]*Queue),
		flowRules: make(map[string]*FlowRule),
		stopCh:    make(chan struct{}),
	}
}

// RegisterPort 注册端口.
func (m *Manager) RegisterPort(port *Port) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrManagerClosed
	}
	if _, exists := m.ports[port.ID]; exists {
		return ErrPortExists
	}
	port.State = PortStateDown
	port.CreatedAt = time.Now()
	port.UpdatedAt = time.Now()
	m.ports[port.ID] = port
	return nil
}

// StartPort 启动端口.
func (m *Manager) StartPort(portID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	port, exists := m.ports[portID]
	if !exists {
		return ErrPortNotFound
	}
	if port.State == PortStateUp {
		return ErrPortAlreadyStarted
	}
	port.State = PortStateUp
	port.UpdatedAt = time.Now()
	return nil
}

// StopPort 停止端口.
func (m *Manager) StopPort(portID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	port, exists := m.ports[portID]
	if !exists {
		return ErrPortNotFound
	}
	port.State = PortStateDown
	port.UpdatedAt = time.Now()
	return nil
}

// GetPort 获取端口.
func (m *Manager) GetPort(portID string) (*Port, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	port, exists := m.ports[portID]
	if !exists {
		return nil, ErrPortNotFound
	}
	return port, nil
}

// AddFlowRule 添加流表规则.
func (m *Manager) AddFlowRule(rule *FlowRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.flowRules[rule.ID]; exists {
		return ErrRuleExists
	}
	rule.CreatedAt = time.Now()
	m.flowRules[rule.ID] = rule
	return nil
}

// RemoveFlowRule 移除流表规则.
func (m *Manager) RemoveFlowRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.flowRules[ruleID]; !exists {
		return ErrRuleNotFound
	}
	delete(m.flowRules, ruleID)
	return nil
}

// ListFlowRules 列出端口的流规则.
func (m *Manager) ListFlowRules(portID string) []*FlowRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var rules []*FlowRule
	for _, r := range m.flowRules {
		if r.PortID == portID {
			rules = append(rules, r)
		}
	}
	return rules
}

// GetStats 获取端口统计.
func (m *Manager) GetStats(portID string) (*PortStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	port, exists := m.ports[portID]
	if !exists {
		return nil, ErrPortNotFound
	}
	return &port.Stats, nil
}

// Close 关闭管理器.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.stopCh)
	return nil
}
