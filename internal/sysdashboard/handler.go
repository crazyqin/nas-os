package sysdashboard

import (
	"fmt"
	"sync"
	"time"
)

// WidgetType 组件类型
type WidgetType string

const (
	WidgetCPUGraph     WidgetType = "cpu_graph"
	WidgetMemGraph     WidgetType = "mem_graph"
	WidgetDiskGraph    WidgetType = "disk_graph"
	WidgetNetGraph     WidgetType = "net_graph"
	WidgetGPUStats     WidgetType = "gpu_stats"
	WidgetPoolHealth   WidgetType = "pool_health"
	WidgetContainers   WidgetType = "containers"
	WidgetVMs          WidgetType = "vms"
	WidgetAlerts       WidgetType = "alerts"
	WidgetWeather      WidgetType = "weather"
	WidgetSystemInfo   WidgetType = "system_info"
	WidgetServices     WidgetType = "services"
	WidgetCustom       WidgetType = "custom"
)

// Widget 仪表盘组件
type Widget struct {
	ID      string     `json:"id"`
	Type    WidgetType `json:"type"`
	Title   string     `json:"title"`
	X       int        `json:"x"`
	Y       int        `json:"y"`
	Width   int        `json:"width"`
	Height  int        `json:"height"`
	Config  map[string]string `json:"config"`
	Visible bool       `json:"visible"`
}

// Dashboard 仪表盘
type Dashboard struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Default   bool      `json:"default"`
	Widgets   []*Widget `json:"widgets"`
	Columns   int       `json:"columns"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Alert 告警信息
type Alert struct {
	ID        string    `json:"id"`
	Level     string    `json:"level"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Acked     bool      `json:"acked"`
	CreatedAt time.Time `json:"createdAt"`
}

// ServiceStatus 服务状态
type ServiceStatus struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Port      int       `json:"port"`
	Uptime    string    `json:"uptime"`
	CPU       float64   `json:"cpu"`
	MemMB     float64   `json:"memMb"`
}

// SystemOverview 系统概览
type SystemOverview struct {
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	Kernel      string  `json:"kernel"`
	Uptime      string  `json:"uptime"`
	CPUCores    int     `json:"cpuCores"`
	CPUUsage    float64 `json:"cpuUsage"`
	MemTotalMB  int     `json:"memTotalMb"`
	MemUsedMB   int     `json:"memUsedMb"`
	MemUsage    float64 `json:"memUsage"`
	DiskTotalGB int     `json:"diskTotalGb"`
	DiskUsedGB  int     `json:"diskUsedGb"`
	DiskUsage   float64 `json:"diskUsage"`
	NetInMB     int     `json:"netInMb"`
	NetOutMB    int     `json:"netOutMb"`
	LoadAvg1    float64 `json:"loadAvg1"`
	LoadAvg5    float64 `json:"loadAvg5"`
	LoadAvg15   float64 `json:"loadAvg15"`
	TempCPU     int     `json:"tempCpu"`
}

// Manager 仪表盘管理器
type Manager struct {
	mu         sync.RWMutex
	dashboards map[string]*Dashboard
	alerts     []*Alert
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		dashboards: make(map[string]*Dashboard),
		alerts:     make([]*Alert, 0),
	}
}

// GetDashboards 获取所有仪表盘
func (m *Manager) GetDashboards() []*Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dashboards := make([]*Dashboard, 0, len(m.dashboards))
	for _, d := range m.dashboards {
		dashboards = append(dashboards, d)
	}
	return dashboards
}

// GetDashboard 获取指定仪表盘
func (m *Manager) GetDashboard(id string) (*Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %s not found", id)
	}
	return d, nil
}

// CreateDashboard 创建仪表盘
func (m *Manager) CreateDashboard(name string, columns int) (*Dashboard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := &Dashboard{
		ID:        fmt.Sprintf("dash-%d", len(m.dashboards)+1),
		Name:      name,
		Columns:   columns,
		Widgets:   make([]*Widget, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.dashboards[d.ID] = d
	return d, nil
}

// UpdateDashboard 更新仪表盘
func (m *Manager) UpdateDashboard(id, name string, columns int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dashboards[id]
	if !ok {
		return fmt.Errorf("dashboard %s not found", id)
	}
	if name != "" {
		d.Name = name
	}
	if columns > 0 {
		d.Columns = columns
	}
	d.UpdatedAt = time.Now()
	return nil
}

// DeleteDashboard 删除仪表盘
func (m *Manager) DeleteDashboard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.dashboards[id]; !ok {
		return fmt.Errorf("dashboard %s not found", id)
	}
	delete(m.dashboards, id)
	return nil
}

// AddWidget 添加组件
func (m *Manager) AddWidget(dashboardID string, widget *Widget) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard %s not found", dashboardID)
	}
	widget.ID = fmt.Sprintf("widget-%d", len(d.Widgets)+1)
	widget.Visible = true
	d.Widgets = append(d.Widgets, widget)
	d.UpdatedAt = time.Now()
	return nil
}

// UpdateWidget 更新组件
func (m *Manager) UpdateWidget(dashboardID, widgetID string, x, y, width, height int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard %s not found", dashboardID)
	}
	for _, w := range d.Widgets {
		if w.ID == widgetID {
			if x >= 0 {
				w.X = x
			}
			if y >= 0 {
				w.Y = y
			}
			if width > 0 {
				w.Width = width
			}
			if height > 0 {
				w.Height = height
			}
			d.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("widget %s not found", widgetID)
}

// RemoveWidget 删除组件
func (m *Manager) RemoveWidget(dashboardID, widgetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard %s not found", dashboardID)
	}
	for i, w := range d.Widgets {
		if w.ID == widgetID {
			d.Widgets = append(d.Widgets[:i], d.Widgets[i+1:]...)
			d.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("widget %s not found", widgetID)
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts(unackedOnly bool) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !unackedOnly {
		return m.alerts
	}
	var alerts []*Alert
	for _, a := range m.alerts {
		if !a.Acked {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// AckAlert 确认告警
func (m *Manager) AckAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == id {
			a.Acked = true
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", id)
}

// GetSystemOverview 获取系统概览
func (m *Manager) GetSystemOverview() *SystemOverview {
	return &SystemOverview{
		Hostname: "nas-os",
		OS:       "Linux",
		Kernel:   "6.1.99",
	}
}

// GetServices 获取服务状态
func (m *Manager) GetServices() []*ServiceStatus {
	return []*ServiceStatus{}
}
