// Package netmonitor 提供网络监控功能
package netmonitor

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager 网络监控管理器.
type Manager struct {
	mu           sync.RWMutex
	interfaces   map[string]*NetworkInterface
	trafficLog   map[string][]*TrafficStats
	connections  []*ConnectionInfo
	alertRules   map[string]*AlertRule
	alertEvents  []*AlertEvent
	topology     *NetworkTopology
	portMonitors map[string]*PortMonitorConfig
	stopChan     chan struct{}
	running      bool
	maxLogSize   int
}

// NewManager 创建网络监控管理器.
func NewManager() *Manager {
	return &Manager{
		interfaces:   make(map[string]*NetworkInterface),
		trafficLog:   make(map[string][]*TrafficStats),
		connections:  make([]*ConnectionInfo, 0),
		alertRules:   make(map[string]*AlertRule),
		alertEvents:  make([]*AlertEvent, 0),
		portMonitors: make(map[string]*PortMonitorConfig),
		stopChan:     make(chan struct{}),
		maxLogSize:   1440, // 24小时，每分钟一条
	}
}

// Start 启动网络监控.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	go m.monitorLoop()
	log.Println("netmonitor started")
}

// Stop 停止网络监控.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	log.Println("netmonitor stopped")
}

// monitorLoop 监控循环.
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 启动时立即采集一次
	m.collectInterfaceStats()
	m.collectConnections()

	for {
		select {
		case <-ticker.C:
			m.collectInterfaceStats()
			m.collectConnections()
			m.checkAlerts()
		case <-m.stopChan:
			return
		}
	}
}

// GetInterfaces 获取所有网络接口.
func (m *Manager) GetInterfaces() []*NetworkInterface {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ifaces := make([]*NetworkInterface, 0, len(m.interfaces))
	for _, iface := range m.interfaces {
		ifaces = append(ifaces, iface)
	}
	return ifaces
}

// GetInterface 获取指定网络接口.
func (m *Manager) GetInterface(name string) (*NetworkInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iface, ok := m.interfaces[name]
	if !ok {
		return nil, fmt.Errorf("interface %s not found", name)
	}
	return iface, nil
}

// GetTrafficStats 获取流量统计.
func (m *Manager) GetTrafficStats(iface string) []*TrafficStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if iface != "" {
		return m.trafficLog[iface]
	}

	// 返回所有接口的最新统计
	all := make([]*TrafficStats, 0)
	for _, stats := range m.trafficLog {
		if len(stats) > 0 {
			all = append(all, stats[len(stats)-1])
		}
	}
	return all
}

// GetConnections 获取连接状态.
func (m *Manager) GetConnections() *ConnectionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ConnectionStats{
		Timestamp:   time.Now(),
		TotalConns:  len(m.connections),
		Connections: m.connections,
	}

	for _, conn := range m.connections {
		switch conn.Protocol {
		case "tcp":
			stats.TCPConns++
		case "udp":
			stats.UDPConns++
		}

		switch conn.State {
		case "ESTABLISHED":
			stats.Established++
		case "LISTEN":
			stats.Listening++
		case "TIME_WAIT":
			stats.TimeWait++
		}
	}

	return stats
}

// AddAlertRule 添加告警规则.
func (m *Manager) AddAlertRule(rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
	}
	rule.CreatedAt = time.Now()

	m.alertRules[rule.ID] = rule
	log.Printf("added alert rule: %s (%s)", rule.ID, rule.Name)
	return nil
}

// RemoveAlertRule 删除告警规则.
func (m *Manager) RemoveAlertRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.alertRules[id]; !ok {
		return fmt.Errorf("alert rule %s not found", id)
	}

	delete(m.alertRules, id)
	log.Printf("removed alert rule: %s", id)
	return nil
}

// GetAlertRules 获取所有告警规则.
func (m *Manager) GetAlertRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(m.alertRules))
	for _, r := range m.alertRules {
		rules = append(rules, r)
	}
	return rules
}

// GetAlertEvents 获取告警事件.
func (m *Manager) GetAlertEvents(limit int) []*AlertEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alertEvents) {
		limit = len(m.alertEvents)
	}

	start := len(m.alertEvents) - limit
	if start < 0 {
		start = 0
	}
	return m.alertEvents[start:]
}

// DiscoverTopology 发现网络拓扑.
func (m *Manager) DiscoverTopology() *NetworkTopology {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟网络拓扑发现
	m.topology = &NetworkTopology{
		Nodes: []*NetworkNode{
			{
				ID:       "node_1",
				Name:     "NAS",
				Type:     "nas",
				IPAddr:   "192.168.1.100",
				MACAddr:  "00:11:22:33:44:55",
				IsOnline: true,
				LastSeen: time.Now(),
			},
			{
				ID:       "node_2",
				Name:     "Router",
				Type:     "router",
				IPAddr:   "192.168.1.1",
				IsOnline: true,
				LastSeen: time.Now(),
			},
			{
				ID:       "node_3",
				Name:     "Desktop",
				Type:     "client",
				IPAddr:   "192.168.1.50",
				IsOnline: true,
				LastSeen: time.Now(),
			},
		},
		Links: []*NetworkLink{
			{SourceID: "node_1", TargetID: "node_2", Speed: 1000, Active: true},
			{SourceID: "node_3", TargetID: "node_2", Speed: 1000, Active: true},
		},
		Discovered: time.Now(),
	}

	log.Printf("network topology discovered: %d nodes, %d links",
		len(m.topology.Nodes), len(m.topology.Links))

	return m.topology
}

// GetTopology 获取已发现的拓扑.
func (m *Manager) GetTopology() *NetworkTopology {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.topology
}

// AddPortMonitor 添加端口监控.
func (m *Manager) AddPortMonitor(config *PortMonitorConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%v", config.Host, config.Ports)
	m.portMonitors[key] = config
	log.Printf("added port monitor for %s", config.Host)
	return nil
}

// GetPortMonitors 获取端口监控配置.
func (m *Manager) GetPortMonitors() []*PortMonitorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]*PortMonitorConfig, 0, len(m.portMonitors))
	for _, c := range m.portMonitors {
		configs = append(configs, c)
	}
	return configs
}

// CheckPorts 检查端口状态.
func (m *Manager) CheckPorts(host string, ports []int) []*PortInfo {
	results := make([]*PortInfo, 0)
	now := time.Now()

	for _, port := range ports {
		// 模拟端口检查
		state := "open"
		service := "unknown"

		switch port {
		case 22:
			service = "ssh"
		case 80:
			service = "http"
		case 443:
			service = "https"
		case 8080:
			service = "http-alt"
		}

		results = append(results, &PortInfo{
			Port:        port,
			Protocol:    "tcp",
			State:       state,
			Service:     service,
			LastChecked: now,
		})
	}

	return results
}

// GetBandwidthTrend 获取带宽趋势.
func (m *Manager) GetBandwidthTrend(iface string, duration time.Duration) *BandwidthTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.trafficLog[iface]
	if len(stats) == 0 {
		return nil
	}

	trend := &BandwidthTrend{
		Interface:  iface,
		StartTime:  time.Now().Add(-duration),
		EndTime:    time.Now(),
		DataPoints: stats,
	}

	var totalRx, totalTx int64
	for _, s := range stats {
		totalRx += s.RxBytesSec
		totalTx += s.TxBytesSec
		if s.RxBytesSec > trend.PeakRx {
			trend.PeakRx = s.RxBytesSec
		}
		if s.TxBytesSec > trend.PeakTx {
			trend.PeakTx = s.TxBytesSec
		}
	}

	if len(stats) > 0 {
		trend.AvgRx = totalRx / int64(len(stats))
		trend.AvgTx = totalTx / int64(len(stats))
	}

	return trend
}

// collectInterfaceStats 采集接口统计.
func (m *Manager) collectInterfaceStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 模拟接口数据
	interfaces := []struct {
		name   string
		status InterfaceStatus
		speed  int
	}{
		{"eth0", InterfaceStatusUp, 1000},
		{"wlan0", InterfaceStatusUp, 300},
		{"docker0", InterfaceStatusUp, 10000},
	}

	for _, iface := range interfaces {
		// 更新接口信息
		m.interfaces[iface.name] = &NetworkInterface{
			Name:        iface.name,
			Status:      iface.status,
			MTU:         1500,
			MACAddress:  fmt.Sprintf("00:11:22:33:%02x:%02x", len(iface.name), time.Now().Second()),
			Speed:       iface.speed,
			RxBytes:     int64(1024 * 1024 * time.Now().Minute()),
			TxBytes:     int64(512 * 1024 * time.Now().Minute()),
			LastUpdated: now,
		}

		// 记录流量统计
		stat := &TrafficStats{
			Interface:    iface.name,
			Timestamp:    now,
			RxBytesSec:   int64(1024 * (time.Now().Second()%100 + 10)),
			TxBytesSec:   int64(512 * (time.Now().Second()%50 + 5)),
			RxPacketsSec: int64(100 + time.Now().Second()%50),
			TxPacketsSec: int64(50 + time.Now().Second()%30),
			TotalRxBytes: int64(1024 * 1024 * time.Now().Minute()),
			TotalTxBytes: int64(512 * 1024 * time.Now().Minute()),
		}

		m.trafficLog[iface.name] = append(m.trafficLog[iface.name], stat)

		// 限制日志大小
		if len(m.trafficLog[iface.name]) > m.maxLogSize {
			m.trafficLog[iface.name] = m.trafficLog[iface.name][len(m.trafficLog[iface.name])-m.maxLogSize:]
		}
	}
}

// collectConnections 采集连接信息.
func (m *Manager) collectConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟连接数据
	m.connections = []*ConnectionInfo{
		{
			Protocol:    "tcp",
			LocalAddr:   "192.168.1.100",
			LocalPort:   22,
			RemoteAddr:  "192.168.1.50",
			RemotePort:  54321,
			State:       "ESTABLISHED",
			PID:         1234,
			ProcessName: "sshd",
		},
		{
			Protocol:    "tcp",
			LocalAddr:   "0.0.0.0",
			LocalPort:   80,
			RemoteAddr:  "",
			RemotePort:  0,
			State:       "LISTEN",
			PID:         5678,
			ProcessName: "nginx",
		},
		{
			Protocol:    "tcp",
			LocalAddr:   "192.168.1.100",
			LocalPort:   443,
			RemoteAddr:  "10.0.0.1",
			RemotePort:  12345,
			State:       "ESTABLISHED",
			PID:         9012,
			ProcessName: "nginx",
		},
		{
			Protocol:    "udp",
			LocalAddr:   "0.0.0.0",
			LocalPort:   53,
			RemoteAddr:  "",
			RemotePort:  0,
			State:       "",
			PID:         3456,
			ProcessName: "dnsmasq",
		},
	}
}

// checkAlerts 检查告警规则.
func (m *Manager) checkAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, rule := range m.alertRules {
		if !rule.Enabled {
			continue
		}

		// 检查是否匹配接口
		ifaces := m.interfaces
		if rule.Interface != "" {
			if _, ok := ifaces[rule.Interface]; !ok {
				continue
			}
			ifaces = map[string]*NetworkInterface{
				rule.Interface: m.interfaces[rule.Interface],
			}
		}

		for name, iface := range ifaces {
			var triggered bool
			var value float64
			var message string

			switch rule.Type {
			case "bandwidth":
				// 检查带宽使用率
				usage := float64(iface.RxBytes+iface.TxBytes) / float64(iface.Speed*1024*1024/8) * 100
				value = usage
				if usage > rule.Threshold {
					triggered = true
					message = fmt.Sprintf("接口 %s 带宽使用率 %.1f%% 超过阈值 %.1f%%", name, usage, rule.Threshold)
				}
			case "errors":
				errors := float64(iface.RxErrors + iface.TxErrors)
				value = errors
				if errors > rule.Threshold {
					triggered = true
					message = fmt.Sprintf("接口 %s 错误数 %.0f 超过阈值 %.0f", name, errors, rule.Threshold)
				}
			}

			if triggered {
				event := &AlertEvent{
					ID:        fmt.Sprintf("event_%d", time.Now().UnixNano()),
					RuleID:    rule.ID,
					RuleName:  rule.Name,
					Interface: name,
					Level:     rule.Level,
					Message:   message,
					Value:     value,
					Threshold: rule.Threshold,
					Timestamp: time.Now(),
				}
				m.alertEvents = append(m.alertEvents, event)
				log.Printf("alert triggered: %s", message)
			}
		}
	}
}
