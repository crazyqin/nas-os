// Package dashboard 提供系统监控仪表板功能
package dashboard

import (
	"time"
)

// ========== 仪表板类型 ==========

// Dashboard 仪表板定义
type Dashboard struct {
	ID          string       `json:"id"`                    // 仪表板唯一标识
	Name        string       `json:"name"`                  // 仪表板名称
	Description string       `json:"description,omitempty"` // 描述
	Layout      LayoutConfig `json:"layout"`                // 布局配置
	Widgets     []Widget     `json:"widgets"`               // 小组件列表
	Owner       string       `json:"owner,omitempty"`       // 所有者
	IsPublic    bool         `json:"is_public"`             // 是否公开
	IsDefault   bool         `json:"is_default"`            // 是否默认仪表板
	CreatedAt   time.Time    `json:"created_at"`            // 创建时间
	UpdatedAt   time.Time    `json:"updated_at"`            // 更新时间
	Tags        []string     `json:"tags,omitempty"`        // 标签
	RefreshRate int          `json:"refresh_rate"`          // 自动刷新间隔（秒），0 表示不刷新
}

// DashboardInput 创建/更新仪表板输入
type DashboardInput struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description,omitempty"`
	Layout      LayoutConfig `json:"layout"`
	Widgets     []Widget     `json:"widgets"`
	IsPublic    bool         `json:"is_public"`
	IsDefault   bool         `json:"is_default"`
	Tags        []string     `json:"tags,omitempty"`
	RefreshRate int          `json:"refresh_rate"` // 自动刷新间隔（秒）
}

// ========== 布局配置 ==========

// LayoutConfig 网格布局配置
type LayoutConfig struct {
	Columns    int    `json:"columns"`              // 网格列数（默认 12）
	RowHeight  int    `json:"row_height"`           // 每行高度（像素）
	Gap        int    `json:"gap"`                  // 网格间距（像素）
	Compact    bool   `json:"compact"`              // 是否自动紧凑排列
	Responsive bool   `json:"responsive"`           // 是否响应式布局
	MaxWidth   int    `json:"max_width,omitempty"`  // 最大宽度（像素）
	MinWidth   int    `json:"min_width,omitempty"`  // 最小宽度（像素）
	Background string `json:"background,omitempty"` // 背景色/图片
}

// DefaultLayoutConfig 返回默认布局配置
func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		Columns:    12,
		RowHeight:  60,
		Gap:        16,
		Compact:    true,
		Responsive: true,
	}
}

// ========== 小组件类型 ==========

// WidgetType 小组件类型
type WidgetType string

const (
	WidgetTypeSystemStatus   WidgetType = "system_status"   // 系统状态
	WidgetTypeStorageUsage   WidgetType = "storage_usage"   // 存储使用
	WidgetTypeNetworkTraffic WidgetType = "network_traffic" // 网络流量
	WidgetTypeUserActivity   WidgetType = "user_activity"   // 用户活动
	WidgetTypeCPUUsage       WidgetType = "cpu_usage"       // CPU 使用率
	WidgetTypeMemoryUsage    WidgetType = "memory_usage"    // 内存使用率
	WidgetTypeDiskIO         WidgetType = "disk_io"         // 磁盘 I/O
	WidgetTypeProcessList    WidgetType = "process_list"    // 进程列表
	WidgetTypeServiceStatus  WidgetType = "service_status"  // 服务状态
	WidgetTypeCustomMetric   WidgetType = "custom_metric"   // 自定义指标
)

// WidgetSize 小组件尺寸预设
type WidgetSize string

const (
	SizeSmall  WidgetSize = "small"  // 1x1
	SizeMedium WidgetSize = "medium" // 2x2
	SizeLarge  WidgetSize = "large"  // 4x2
	SizeWide   WidgetSize = "wide"   // 4x1
	SizeFull   WidgetSize = "full"   // 12x1
)

// Widget 小组件定义
type Widget struct {
	ID         string                 `json:"id"`                    // 小组件唯一标识
	Type       WidgetType             `json:"type"`                  // 小组件类型
	Title      string                 `json:"title"`                 // 标题
	Position   WidgetPosition         `json:"position"`              // 位置信息
	Config     map[string]interface{} `json:"config,omitempty"`      // 配置参数
	Data       interface{}            `json:"data,omitempty"`        // 渲染数据（缓存）
	LastUpdate time.Time              `json:"last_update,omitempty"` // 最后更新时间
}

// WidgetPosition 小组件位置
type WidgetPosition struct {
	X      int `json:"x"`      // 列位置（从 0 开始）
	Y      int `json:"y"`      // 行位置（从 0 开始）
	Width  int `json:"width"`  // 占用列数
	Height int `json:"height"` // 占用行数
}

// GetPresetSize 根据预设尺寸返回位置信息
func GetPresetSize(size WidgetSize) WidgetPosition {
	sizes := map[WidgetSize]WidgetPosition{
		SizeSmall:  {Width: 2, Height: 2},
		SizeMedium: {Width: 4, Height: 2},
		SizeLarge:  {Width: 6, Height: 3},
		SizeWide:   {Width: 6, Height: 2},
		SizeFull:   {Width: 12, Height: 2},
	}
	return sizes[size]
}

// ========== 小组件配置 ==========

// SystemStatusConfig 系统状态小组件配置
type SystemStatusConfig struct {
	ShowUptime   bool `json:"show_uptime"`   // 显示运行时间
	ShowHostname bool `json:"show_hostname"` // 显示主机名
	ShowVersion  bool `json:"show_version"`  // 显示版本
	ShowIP       bool `json:"show_ip"`       // 显示 IP 地址
	ShowLoadAvg  bool `json:"show_load_avg"` // 显示负载
}

// StorageUsageConfig 存储使用小组件配置
type StorageUsageConfig struct {
	ShowAllMounts     bool     `json:"show_all_mounts"`    // 显示所有挂载点
	MountPoints       []string `json:"mount_points"`       // 指定挂载点
	ShowInodes        bool     `json:"show_inodes"`        // 显示 inode 信息
	ShowIOStats       bool     `json:"show_io_stats"`      // 显示 IO 统计
	WarningThreshold  float64  `json:"warning_threshold"`  // 警告阈值（百分比）
	CriticalThreshold float64  `json:"critical_threshold"` // 严重阈值（百分比）
}

// NetworkTrafficConfig 网络流量小组件配置
type NetworkTrafficConfig struct {
	Interfaces    []string `json:"interfaces"`      // 网络接口列表
	ShowAllIfaces bool     `json:"show_all_ifaces"` // 显示所有接口
	ShowErrors    bool     `json:"show_errors"`     // 显示错误统计
	ShowPackets   bool     `json:"show_packets"`    // 显示包统计
	HistoryHours  int      `json:"history_hours"`   // 历史数据时长（小时）
}

// UserActivityConfig 用户活动小组件配置
type UserActivityConfig struct {
	ShowLogins   bool `json:"show_logins"`   // 显示登录记录
	ShowActive   bool `json:"show_active"`   // 显示活动用户
	ShowFailed   bool `json:"show_failed"`   // 显示失败登录
	MaxRecords   int  `json:"max_records"`   // 最大记录数
	HistoryHours int  `json:"history_hours"` // 历史时长（小时）
}

// ========== 小组件数据 ==========

// SystemStatusData 系统状态数据
type SystemStatusData struct {
	Hostname    string    `json:"hostname"`
	Uptime      string    `json:"uptime"`
	UptimeSec   int64     `json:"uptime_sec"`
	Version     string    `json:"version"`
	Platform    string    `json:"platform"`
	IPAddresses []string  `json:"ip_addresses"`
	LoadAvg     []float64 `json:"load_avg"`
	Status      string    `json:"status"` // ok, warning, critical
}

// StorageUsageData 存储使用数据
type StorageUsageData struct {
	MountPoint  string  `json:"mount_point"`
	Device      string  `json:"device"`
	Total       int64   `json:"total"`        // 总容量（字节）
	Used        int64   `json:"used"`         // 已使用（字节）
	Available   int64   `json:"available"`    // 可用（字节）
	UsedPercent float64 `json:"used_percent"` // 使用百分比
	InodesTotal int64   `json:"inodes_total"`
	InodesUsed  int64   `json:"inodes_used"`
	Status      string  `json:"status"` // ok, warning, critical
}

// NetworkTrafficData 网络流量数据
type NetworkTrafficData struct {
	Interface string `json:"interface"`
	RxBytes   int64  `json:"rx_bytes"`   // 接收字节数
	TxBytes   int64  `json:"tx_bytes"`   // 发送字节数
	RxPackets int64  `json:"rx_packets"` // 接收包数
	TxPackets int64  `json:"tx_packets"` // 发送包数
	RxErrors  int64  `json:"rx_errors"`  // 接收错误
	TxErrors  int64  `json:"tx_errors"`  // 发送错误
	RxSpeed   int64  `json:"rx_speed"`   // 接收速率（字节/秒）
	TxSpeed   int64  `json:"tx_speed"`   // 发送速率（字节/秒）
	Status    string `json:"status"`
}

// UserActivityData 用户活动数据
type UserActivityData struct {
	Username   string    `json:"username"`
	Action     string    `json:"action"` // login, logout, failed
	Source     string    `json:"source"` // IP 或终端
	Timestamp  time.Time `json:"timestamp"`
	SessionDur int       `json:"session_dur"` // 会话时长（秒）
}

// ========== 导入导出 ==========

// DashboardExport 仪表板导出格式
type DashboardExport struct {
	Version    string      `json:"version"`     // 导出版本
	ExportedAt time.Time   `json:"exported_at"` // 导出时间
	Dashboards []Dashboard `json:"dashboards"`  // 仪表板列表
}

// DashboardTemplate 仪表板模板
type DashboardTemplate struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Category    string       `json:"category"` // 分类：system, storage, network, custom
	Layout      LayoutConfig `json:"layout"`
	Widgets     []Widget     `json:"widgets"`
	IsPublic    bool         `json:"is_public"`
	CreatedAt   time.Time    `json:"created_at"`
}

// 预定义模板
var DefaultTemplates = []DashboardTemplate{
	{
		ID:          "system-overview",
		Name:        "系统概览",
		Description: "系统整体状态监控仪表板",
		Category:    "system",
		Layout:      DefaultLayoutConfig(),
		Widgets: []Widget{
			{ID: "sys-status", Type: WidgetTypeSystemStatus, Title: "系统状态", Position: GetPresetSize(SizeMedium)},
			{ID: "cpu-usage", Type: WidgetTypeCPUUsage, Title: "CPU 使用率", Position: GetPresetSize(SizeMedium)},
			{ID: "mem-usage", Type: WidgetTypeMemoryUsage, Title: "内存使用", Position: GetPresetSize(SizeMedium)},
			{ID: "storage", Type: WidgetTypeStorageUsage, Title: "存储使用", Position: GetPresetSize(SizeLarge)},
		},
		IsPublic: true,
	},
	{
		ID:          "network-monitor",
		Name:        "网络监控",
		Description: "网络流量和连接监控仪表板",
		Category:    "network",
		Layout:      DefaultLayoutConfig(),
		Widgets: []Widget{
			{ID: "net-traffic", Type: WidgetTypeNetworkTraffic, Title: "网络流量", Position: GetPresetSize(SizeFull)},
			{ID: "svc-status", Type: WidgetTypeServiceStatus, Title: "服务状态", Position: GetPresetSize(SizeLarge)},
		},
		IsPublic: true,
	},
	{
		ID:          "user-activity",
		Name:        "用户活动",
		Description: "用户登录和活动监控仪表板",
		Category:    "system",
		Layout:      DefaultLayoutConfig(),
		Widgets: []Widget{
			{ID: "user-act", Type: WidgetTypeUserActivity, Title: "用户活动", Position: GetPresetSize(SizeLarge)},
			{ID: "proc-list", Type: WidgetTypeProcessList, Title: "活动进程", Position: GetPresetSize(SizeLarge)},
		},
		IsPublic: true,
	},
}
