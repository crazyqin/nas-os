// Package unifiedportal 提供统一门户自定义仪表盘功能，支持仪表盘CRUD、Widget管理、主题切换、数据源聚合等。
package unifiedportal

import "time"

// WidgetType Widget类型枚举
type WidgetType string

const (
	WidgetSystemOverview  WidgetType = "system_overview"  // 系统概览
	WidgetStorageUsage    WidgetType = "storage_usage"    // 存储使用
	WidgetNetworkTraffic  WidgetType = "network_traffic"  // 网络流量
	WidgetContainerStatus WidgetType = "container_status" // 容器状态
	WidgetAlerts          WidgetType = "alerts"           // 告警
	WidgetCustomChart     WidgetType = "custom_chart"     // 自定义图表
)

// DashboardLayout 布局类型
type DashboardLayout string

const (
	LayoutGrid       DashboardLayout = "grid"       // 网格布局
	LayoutFree       DashboardLayout = "free"       // 自由布局
	LayoutResponsive DashboardLayout = "responsive" // 响应式布局
)

// ThemeMode 主题模式
type ThemeMode string

const (
	ThemeLight ThemeMode = "light" // 浅色模式
	ThemeDark  ThemeMode = "dark"  // 暗黑模式
	ThemeAuto  ThemeMode = "auto"  // 跟随系统
)

// WidgetPosition Widget位置
type WidgetPosition struct {
	X int `json:"x"` // X坐标
	Y int `json:"y"` // Y坐标
}

// WidgetSize Widget大小
type WidgetSize struct {
	Width  int `json:"width"`  // 宽度（网格列数）
	Height int `json:"height"` // 高度（网格行数）
}

// DashboardWidget 仪表盘Widget
type DashboardWidget struct {
	ID          string                 `json:"id"`
	DashboardID string                 `json:"dashboard_id"`
	Type        WidgetType             `json:"type" binding:"required"`
	Title       string                 `json:"title"`
	Position    WidgetPosition         `json:"position"`
	Size        WidgetSize             `json:"size"`
	Config      map[string]interface{} `json:"config,omitempty"` // Widget专属配置
	DataSource  string                 `json:"data_source,omitempty"`
	RefreshSec  int                    `json:"refresh_sec,omitempty"` // 自动刷新间隔（秒）
	IsVisible   bool                   `json:"is_visible"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// PortalDashboard 仪表盘
type PortalDashboard struct {
	ID          string             `json:"id"`
	Name        string             `json:"name" binding:"required"`
	Description string             `json:"description,omitempty"`
	Layout      DashboardLayout    `json:"layout"`
	Widgets     []*DashboardWidget `json:"widgets,omitempty"`
	IsDefault   bool               `json:"is_default"`
	IsTemplate  bool               `json:"is_template"`
	TemplateID  string             `json:"template_id,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Owner       string             `json:"owner,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// PortalTheme 主题配置
type PortalTheme struct {
	ID           string    `json:"id"`
	Name         string    `json:"name" binding:"required"`
	Mode         ThemeMode `json:"mode"`
	PrimaryColor string    `json:"primary_color,omitempty"` // 主色调
	AccentColor  string    `json:"accent_color,omitempty"`  // 强调色
	BGColor      string    `json:"bg_color,omitempty"`      // 背景色
	TextColor    string    `json:"text_color,omitempty"`    // 文字色
	FontSize     string    `json:"font_size,omitempty"`     // 字体大小
	FontFamily   string    `json:"font_family,omitempty"`   // 字体族
	BorderRadius string    `json:"border_radius,omitempty"` // 圆角
	IsDark       bool      `json:"is_dark"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DashboardRequest 创建/更新仪表盘请求
type DashboardRequest struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description,omitempty"`
	Layout      DashboardLayout `json:"layout,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
}

// WidgetRequest 创建/更新Widget请求
type WidgetRequest struct {
	Type       WidgetType             `json:"type" binding:"required"`
	Title      string                 `json:"title,omitempty"`
	Position   WidgetPosition         `json:"position"`
	Size       WidgetSize             `json:"size"`
	Config     map[string]interface{} `json:"config,omitempty"`
	DataSource string                 `json:"data_source,omitempty"`
	RefreshSec int                    `json:"refresh_sec,omitempty"`
}

// WidgetMoveRequest Widget移动请求
type WidgetMoveRequest struct {
	Position WidgetPosition `json:"position"`
	Size     WidgetSize     `json:"size"`
}

// ThemeRequest 切换主题请求
type ThemeRequest struct {
	ThemeID string `json:"theme_id" binding:"required"`
}

// DashboardExport 仪表盘导出格式
type DashboardExport struct {
	Dashboard  *PortalDashboard   `json:"dashboard"`
	Widgets    []*DashboardWidget `json:"widgets"`
	Version    string             `json:"version"`
	ExportedAt time.Time          `json:"exported_at"`
}

// DataSource 数据源定义
type DataSource struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"` // prometheus, influxdb, api, etc.
	Endpoint string                 `json:"endpoint"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// AggregatedMetrics 聚合指标
type AggregatedMetrics struct {
	System    *SystemMetrics    `json:"system,omitempty"`
	Storage   *StorageMetrics   `json:"storage,omitempty"`
	Network   *NetworkMetrics   `json:"network,omitempty"`
	Container *ContainerMetrics `json:"container,omitempty"`
	Alerts    *AlertMetrics     `json:"alerts,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryUsed    int64   `json:"memory_used"`
	MemoryTotal   int64   `json:"memory_total"`
	Uptime        int64   `json:"uptime"`
	LoadAvg1      float64 `json:"load_avg_1"`
	LoadAvg5      float64 `json:"load_avg_5"`
	LoadAvg15     float64 `json:"load_avg_15"`
}

// StorageMetrics 存储指标
type StorageMetrics struct {
	TotalBytes      int64   `json:"total_bytes"`
	UsedBytes       int64   `json:"used_bytes"`
	FreeBytes       int64   `json:"free_bytes"`
	UsagePercent    float64 `json:"usage_percent"`
	IOPSRead        int64   `json:"iops_read"`
	IOPSWrite       int64   `json:"iops_write"`
	ThroughputRead  int64   `json:"throughput_read"`
	ThroughputWrite int64   `json:"throughput_write"`
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	BytesIn     int64   `json:"bytes_in"`
	BytesOut    int64   `json:"bytes_out"`
	PacketsIn   int64   `json:"packets_in"`
	PacketsOut  int64   `json:"packets_out"`
	Connections int     `json:"connections"`
	Bandwidth   float64 `json:"bandwidth_mbps"`
}

// ContainerMetrics 容器指标
type ContainerMetrics struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Stopped   int `json:"stopped"`
	Paused    int `json:"paused"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
}

// AlertMetrics 告警指标
type AlertMetrics struct {
	Critical int          `json:"critical"`
	Warning  int          `json:"warning"`
	Info     int          `json:"info"`
	Total    int          `json:"total"`
	Recent   []*AlertItem `json:"recent,omitempty"`
}

// AlertItem 告警项
type AlertItem struct {
	ID        string    `json:"id"`
	Level     string    `json:"level"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// SupportedWidgetTypes 获取支持的Widget类型列表
func SupportedWidgetTypes() []WidgetType {
	return []WidgetType{
		WidgetSystemOverview,
		WidgetStorageUsage,
		WidgetNetworkTraffic,
		WidgetContainerStatus,
		WidgetAlerts,
		WidgetCustomChart,
	}
}

// SupportedLayouts 获取支持的布局类型列表
func SupportedLayouts() []DashboardLayout {
	return []DashboardLayout{
		LayoutGrid,
		LayoutFree,
		LayoutResponsive,
	}
}

// IsValidWidgetType 检查Widget类型是否有效
func IsValidWidgetType(t WidgetType) bool {
	for _, wt := range SupportedWidgetTypes() {
		if wt == t {
			return true
		}
	}
	return false
}

// IsValidLayout 检查布局类型是否有效
func IsValidLayout(l DashboardLayout) bool {
	for _, layout := range SupportedLayouts() {
		if layout == l {
			return true
		}
	}
	return false
}
