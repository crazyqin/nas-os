// Package dashboard 提供监控仪表板功能
// v2.52.0 - 监控面板组件
package dashboard

import (
	"time"
)

// WidgetType defines the type of dashboard widget.
type WidgetType string

const (
	// WidgetTypeCPU displays CPU metrics.
	WidgetTypeCPU WidgetType = "cpu"
	// WidgetTypeMemory displays memory metrics.
	WidgetTypeMemory WidgetType = "memory"
	// WidgetTypeDisk displays disk usage metrics.
	WidgetTypeDisk WidgetType = "disk"
	// WidgetTypeNetwork displays network metrics.
	WidgetTypeNetwork WidgetType = "network"
	// WidgetTypeCustom is a user-defined widget type.
	WidgetTypeCustom WidgetType = "custom"
)

// WidgetSize defines the size of a dashboard widget.
type WidgetSize string

const (
	// WidgetSizeSmall is a small widget.
	WidgetSizeSmall WidgetSize = "small"
	// WidgetSizeMedium is a medium widget.
	WidgetSizeMedium WidgetSize = "medium"
	// WidgetSizeLarge is a large widget.
	WidgetSizeLarge WidgetSize = "large"
)

// WidgetPosition 小组件位置
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Widget 小组件定义
type Widget struct {
	ID          string         `json:"id"`
	Type        WidgetType     `json:"type"`
	Title       string         `json:"title"`
	Size        WidgetSize     `json:"size"`
	Position    WidgetPosition `json:"position"`
	Config      WidgetConfig   `json:"config,omitempty"`
	Enabled     bool           `json:"enabled"`
	RefreshRate time.Duration  `json:"refreshRate"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// WidgetConfig 小组件配置
type WidgetConfig struct {
	// CPU 配置
	ShowPerCore       bool    `json:"showPerCore,omitempty"`
	ShowAverage       bool    `json:"showAverage,omitempty"`
	WarningThreshold  float64 `json:"warningThreshold,omitempty"`
	CriticalThreshold float64 `json:"criticalThreshold,omitempty"`

	// 内存配置
	ShowSwap    bool `json:"showSwap,omitempty"`
	ShowBuffers bool `json:"showBuffers,omitempty"`

	// 磁盘配置
	MountPoints []string `json:"mountPoints,omitempty"`
	ShowIOStats bool     `json:"showIOStats,omitempty"`

	// 网络配置
	Interfaces  []string `json:"interfaces,omitempty"`
	ShowPackets bool     `json:"showPackets,omitempty"`
	ShowErrors  bool     `json:"showErrors,omitempty"`

	// 通用配置
	MaxDataPoints int    `json:"maxDataPoints,omitempty"`
	TimeRange     string `json:"timeRange,omitempty"`
}

// Dashboard 仪表板定义
type Dashboard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Widgets     []*Widget `json:"widgets"`
	Layout      Layout    `json:"layout"`
	IsDefault   bool      `json:"isDefault"`
	OwnerID     string    `json:"ownerId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Layout 仪表板布局
type Layout struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
	Gap     int `json:"gap"`
}

// WidgetData 小组件数据
type WidgetData struct {
	WidgetID  string      `json:"widgetId"`
	Type      WidgetType  `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
	Error     string      `json:"error,omitempty"`
}

// CPUWidgetData CPU 小组件数据
type CPUWidgetData struct {
	Timestamp    time.Time `json:"timestamp"`
	Usage        float64   `json:"usage"`
	PerCore      []float64 `json:"perCore,omitempty"`
	LoadAvg1     float64   `json:"loadAvg1"`
	LoadAvg5     float64   `json:"loadAvg5"`
	LoadAvg15    float64   `json:"loadAvg15"`
	ProcessCount int       `json:"processCount"`
	Trend        []float64 `json:"trend,omitempty"`
}

// MemoryWidgetData 内存小组件数据
type MemoryWidgetData struct {
	Timestamp time.Time `json:"timestamp"`
	Total     uint64    `json:"total"`
	Used      uint64    `json:"used"`
	Free      uint64    `json:"free"`
	Available uint64    `json:"available"`
	Usage     float64   `json:"usage"`
	SwapTotal uint64    `json:"swapTotal"`
	SwapUsed  uint64    `json:"swapUsed"`
	SwapFree  uint64    `json:"swapFree"`
	SwapUsage float64   `json:"swapUsage"`
	Buffers   uint64    `json:"buffers,omitempty"`
	Cached    uint64    `json:"cached,omitempty"`
	Trend     []float64 `json:"trend,omitempty"`
}

// DiskWidgetData 磁盘小组件数据
type DiskWidgetData struct {
	Timestamp time.Time        `json:"timestamp"`
	Devices   []DiskDeviceData `json:"devices"`
	Total     DiskSummaryData  `json:"total"`
}

// DiskDeviceData 磁盘设备数据
type DiskDeviceData struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mountPoint"`
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usagePercent"`
	FSType       string  `json:"fsType"`
	ReadBytes    uint64  `json:"readBytes,omitempty"`
	WriteBytes   uint64  `json:"writeBytes,omitempty"`
}

// DiskSummaryData 磁盘汇总数据
type DiskSummaryData struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usagePercent"`
}

// NetworkWidgetData 网络小组件数据
type NetworkWidgetData struct {
	Timestamp  time.Time              `json:"timestamp"`
	Interfaces []NetworkInterfaceData `json:"interfaces"`
	Total      NetworkSummaryData     `json:"total"`
}

// NetworkInterfaceData 网络接口数据
type NetworkInterfaceData struct {
	Name      string `json:"name"`
	RXBytes   uint64 `json:"rxBytes"`
	TXBytes   uint64 `json:"txBytes"`
	RXPackets uint64 `json:"rxPackets,omitempty"`
	TXPackets uint64 `json:"txPackets,omitempty"`
	RXErrors  uint64 `json:"rxErrors,omitempty"`
	TXErrors  uint64 `json:"txErrors,omitempty"`
	Speed     uint64 `json:"speed,omitempty"` // Mbps
}

// NetworkSummaryData 网络汇总数据
type NetworkSummaryData struct {
	RXBytes   uint64 `json:"rxBytes"`
	TXBytes   uint64 `json:"txBytes"`
	RXPackets uint64 `json:"rxPackets"`
	TXPackets uint64 `json:"txPackets"`
}

// DashboardState represents the runtime state of a dashboard.
//
//nolint:revive // DashboardState is intentional for clarity in API responses
type DashboardState struct {
	DashboardID string                 `json:"dashboardId"`
	LastUpdate  time.Time              `json:"lastUpdate"`
	WidgetData  map[string]*WidgetData `json:"widgetData"`
	HealthScore float64                `json:"healthScore"`
	Status      string                 `json:"status"`
}

// DashboardTemplate represents a reusable dashboard template.
//
//nolint:revive // DashboardTemplate is intentional for clarity in API responses
type DashboardTemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Widgets     []*WidgetConfig `json:"widgets"`
	Layout      Layout          `json:"layout"`
}

// WidgetTemplateConfig defines widget configuration within a template.
type WidgetTemplateConfig struct {
	Type     WidgetType     `json:"type"`
	Title    string         `json:"title"`
	Size     WidgetSize     `json:"size"`
	Position WidgetPosition `json:"position"`
	Config   WidgetConfig   `json:"config"`
}

// DashboardEvent represents a dashboard-related event.
//
//nolint:revive // DashboardEvent is intentional for clarity in API responses
type DashboardEvent struct {
	Type        string      `json:"type"`
	DashboardID string      `json:"dashboardId"`
	WidgetID    string      `json:"widgetId,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
	Data        interface{} `json:"data,omitempty"`
}
