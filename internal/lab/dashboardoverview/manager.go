// Package dashboardoverview 提供仪表盘概览功能
package dashboardoverview

import (
	"sync"
	"time"
)

// Manager 仪表盘管理器.
type Manager struct {
	mu       sync.RWMutex
	overview *SystemOverview
	alerts   []AlertItem
	activity []ActivityItem
}

// NewManager 创建管理器.
func NewManager() *Manager {
	mgr := &Manager{
		alerts:   make([]AlertItem, 0),
		activity: make([]ActivityItem, 0),
	}
	mgr.initSampleData()
	return mgr
}

// initSampleData 初始化示例数据.
func (m *Manager) initSampleData() {
	m.overview = &SystemOverview{
		System: SystemInfo{
			Hostname:    "nas-server",
			OS:          "Linux 6.1.99",
			Version:     "v2.497.0",
			Uptime:      22 * 24 * time.Hour,
			LoadAvg:     [3]float64{0.21, 0.12, 0.10},
			Temperature: 42.5,
		},
		CPU: CPUInfo{
			Model:       "RK3588",
			Cores:       8,
			Usage:       12.5,
			Temperature: 45.0,
			Frequency:   2400,
			PerCore:     []float64{10, 15, 8, 12, 20, 5, 14, 11},
		},
		Memory: MemoryInfo{
			Total:     8 * 1024 * 1024 * 1024,
			Used:      2 * 1024 * 1024 * 1024,
			Available: 5 * 1024 * 1024 * 1024,
			Usage:     30.0,
			SwapTotal: 4 * 1024 * 1024 * 1024,
			SwapUsed:  350 * 1024 * 1024,
		},
		Storage: []StoragePool{
			{
				Name:      "main-pool",
				Status:    "healthy",
				Total:     30 * 1024 * 1024 * 1024,
				Used:      21 * 1024 * 1024 * 1024,
				Available: 8 * 1024 * 1024 * 1024,
				Usage:     73,
				Disks:     1,
				RAIDLevel: "single",
				Health:    "ONLINE",
			},
		},
		Network: []NetworkInfo{
			{
				Name:    "eth0",
				IP:      "192.168.1.100",
				MAC:     "aa:bb:cc:dd:ee:ff",
				Speed:   1000,
				RxBytes: 1024 * 1024 * 500,
				TxBytes: 1024 * 1024 * 200,
				RxRate:  1024 * 100,
				TxRate:  1024 * 50,
				IsUp:    true,
			},
		},
		Services: []ServiceInfo{
			{Name: "Web UI", Status: "running", Port: 443, Health: "healthy", Icon: "web"},
			{Name: "SMB", Status: "running", Port: 445, Health: "healthy", Icon: "folder"},
			{Name: "SSH", Status: "running", Port: 22, Health: "healthy", Icon: "terminal"},
			{Name: "Docker", Status: "running", Health: "healthy", Icon: "container"},
			{Name: "NFS", Status: "stopped", Health: "unknown", Icon: "nfs"},
		},
		UpdatedAt: time.Now(),
	}
}

// GetOverview 获取系统概览.
func (m *Manager) GetOverview() *SystemOverview {
	m.mu.RLock()
	defer m.mu.RUnlock()

	overview := *m.overview
	overview.Alerts = m.alerts
	overview.Recent = m.activity
	overview.UpdatedAt = time.Now()
	return &overview
}

// GetCPU 获取CPU信息.
func (m *Manager) GetCPU() CPUInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.overview.CPU
}

// GetMemory 获取内存信息.
func (m *Manager) GetMemory() MemoryInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.overview.Memory
}

// GetStorage 获取存储信息.
func (m *Manager) GetStorage() []StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.overview.Storage
}

// GetNetwork 获取网络信息.
func (m *Manager) GetNetwork() []NetworkInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.overview.Network
}

// GetServices 获取服务信息.
func (m *Manager) GetServices() []ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.overview.Services
}

// GetAlerts 获取告警.
func (m *Manager) GetAlerts(includeAcked bool) []AlertItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if includeAcked {
		return m.alerts
	}

	var result []AlertItem
	for _, a := range m.alerts {
		if !a.Acked {
			result = append(result, a)
		}
	}
	return result
}

// AckAlert 确认告警.
func (m *Manager) AckAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.alerts {
		if m.alerts[i].ID == id {
			m.alerts[i].Acked = true
			return nil
		}
	}
	return nil
}

// AddActivity 添加活动记录.
func (m *Manager) AddActivity(item ActivityItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item.Timestamp = time.Now()
	m.activity = append([]ActivityItem{item}, m.activity...)

	// 保留最近100条
	if len(m.activity) > 100 {
		m.activity = m.activity[:100]
	}
}

// AddAlert 添加告警.
func (m *Manager) AddAlert(item AlertItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item.Timestamp = time.Now()
	item.Acked = false
	m.alerts = append([]AlertItem{item}, m.alerts...)

	// 保留最近50条
	if len(m.alerts) > 50 {
		m.alerts = m.alerts[:50]
	}
}

// GetWidgets 获取组件列表.
func (m *Manager) GetWidgets() []WidgetData {
	return []WidgetData{
		{Type: "cpu", Title: "CPU 使用率", Refresh: 5},
		{Type: "memory", Title: "内存使用率", Refresh: 5},
		{Type: "storage", Title: "存储空间", Refresh: 30},
		{Type: "network", Title: "网络流量", Refresh: 5},
		{Type: "services", Title: "服务状态", Refresh: 30},
		{Type: "alerts", Title: "系统告警", Refresh: 60},
		{Type: "activity", Title: "最近活动", Refresh: 60},
	}
}
