// Package sysdashboard 系统概览仪表盘
// 对标群晖DSM主页 + TrueNAS Dashboard
// 统一展示系统信息、服务状态、存储概览、最近活动
package sysdashboard

import (
	"sync"
	"time"
)

// ServiceStatus 服务状态
type ServiceStatus struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // running/stopped/error
	Port      int       `json:"port"`
	Uptime    string    `json:"uptime"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StorageOverview 存储概览
type StorageOverview struct {
	TotalSpace    int64   `json:"total_space"`
	UsedSpace     int64   `json:"used_space"`
	FreeSpace     int64   `json:"free_space"`
	UsagePercent  float64 `json:"usage_percent"`
	PoolCount     int     `json:"pool_count"`
	DiskCount     int     `json:"disk_count"`
	HealthStatus  string  `json:"health_status"`
	LastScrub     string  `json:"last_scrub"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	Hostname     string  `json:"hostname"`
	OS           string  `json:"os"`
	Kernel       string  `json:"kernel"`
	Architecture string  `json:"architecture"`
	CPUModel     string  `json:"cpu_model"`
	CPUCores     int     `json:"cpu_cores"`
	TotalMemory  int64   `json:"total_memory"`
	Uptime       string  `json:"uptime"`
	LoadAvg      [3]float64 `json:"load_avg"`
}

// RecentActivity 最近活动
type RecentActivity struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
	Timestamp time.Time `json:"timestamp"`
}

// DashboardData 仪表盘数据
type DashboardData struct {
	System      SystemInfo        `json:"system"`
	Services    []ServiceStatus   `json:"services"`
	Storage     StorageOverview   `json:"storage"`
	Activities  []RecentActivity  `json:"activities"`
	Alerts      []AlertItem       `json:"alerts"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AlertItem 告警项
type AlertItem struct {
	ID       string    `json:"id"`
	Level    string    `json:"level"`
	Message  string    `json:"message"`
	Source   string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager 仪表盘管理器
type Manager struct {
	mu          sync.RWMutex
	activities  []RecentActivity
	alerts      []AlertItem
	maxActivity int
}

// NewManager 创建仪表盘管理器
func NewManager() *Manager {
	return &Manager{
		activities:  make([]RecentActivity, 0),
		alerts:      make([]AlertItem, 0),
		maxActivity: 100,
	}
}

// GetDashboard 获取仪表盘数据
func (m *Manager) GetDashboard() *DashboardData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &DashboardData{
		System: m.getSystemInfo(),
		Services: m.getServices(),
		Storage: m.getStorageOverview(),
		Activities: m.activities,
		Alerts: m.alerts,
		UpdatedAt: time.Now(),
	}
}

// AddActivity 添加活动记录
func (m *Manager) AddActivity(activity RecentActivity) {
	m.mu.Lock()
	defer m.mu.Unlock()

	activity.ID = generateID()
	activity.Timestamp = time.Now()
	m.activities = append([]RecentActivity{activity}, m.activities...)
	if len(m.activities) > m.maxActivity {
		m.activities = m.activities[:m.maxActivity]
	}
}

// AddAlert 添加告警
func (m *Manager) AddAlert(alert AlertItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert.ID = generateID()
	alert.CreatedAt = time.Now()
	m.alerts = append([]AlertItem{alert}, m.alerts...)
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(alertID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, a := range m.alerts {
		if a.ID == alertID {
			m.alerts = append(m.alerts[:i], m.alerts[i+1:]...)
			return true
		}
	}
	return false
}

// GetActivities 获取活动列表
func (m *Manager) GetActivities(limit int) []RecentActivity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.activities) {
		limit = len(m.activities)
	}
	return m.activities[:limit]
}

// GetAlerts 获取告警列表
func (m *Manager) GetAlerts() []AlertItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts
}

func (m *Manager) getSystemInfo() SystemInfo {
	return SystemInfo{
		Hostname:     "nas-server",
		OS:           "NAS-OS 2.0",
		Kernel:       "6.1.99",
		Architecture: "arm64",
		CPUModel:     "RK3588",
		CPUCores:     8,
		TotalMemory:  8 * 1024 * 1024 * 1024,
		Uptime:       "25 days",
		LoadAvg:      [3]float64{0.6, 0.3, 0.4},
	}
}

func (m *Manager) getServices() []ServiceStatus {
	return []ServiceStatus{
		{Name: "SMB", Status: "running", Port: 445, Uptime: "25d"},
		{Name: "NFS", Status: "running", Port: 2049, Uptime: "25d"},
		{Name: "SSH", Status: "running", Port: 22, Uptime: "25d"},
		{Name: "WebDAV", Status: "running", Port: 5005, Uptime: "25d"},
		{Name: "Docker", Status: "running", Port: 2376, Uptime: "25d"},
		{Name: "VPN", Status: "stopped", Port: 1194},
	}
}

func (m *Manager) getStorageOverview() StorageOverview {
	return StorageOverview{
		TotalSpace:   2 * 1024 * 1024 * 1024 * 1024,
		UsedSpace:    1480 * 1024 * 1024 * 1024,
		FreeSpace:    520 * 1024 * 1024 * 1024,
		UsagePercent: 74,
		PoolCount:    2,
		DiskCount:    4,
		HealthStatus: "healthy",
		LastScrub:    "2026-05-28",
	}
}

func generateID() string {
	return time.Now().Format("20060102150405.000000")
}
