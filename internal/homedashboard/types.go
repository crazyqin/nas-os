// Package homedashboard 提供统一智能家居+NAS仪表盘功能
// 包含可配置布局、预置Widget、Widget市场、多用户个性化配置、实时数据刷新
package homedashboard

import (
	"fmt"
	"sync"
	"time"
)

// WidgetType 预置 Widget 类型.
type WidgetType string

const (
	WidgetTypeNASStatus   WidgetType = "nas_status"
	WidgetTypeDockerStatus WidgetType = "docker_status"
	WidgetTypeWeather     WidgetType = "weather"
	WidgetTypeCalendar    WidgetType = "calendar"
	WidgetTypeTodoList    WidgetType = "todo_list"
	WidgetTypeQuickActions WidgetType = "quick_actions"
	WidgetTypeRecentFiles WidgetType = "recent_files"
	WidgetTypeStorageTrend WidgetType = "storage_trend"
	WidgetTypeCustom      WidgetType = "custom"
)

// WidgetSize Widget 尺寸.
type WidgetSize struct {
	Width  int `json:"width"`  // 网格列数
	Height int `json:"height"` // 网格行数
}

// WidgetPosition Widget 位置.
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Widget Widget 实例.
type Widget struct {
	ID       string            `json:"id"`
	Type     WidgetType        `json:"type"`
	Title    string            `json:"title"`
	Position WidgetPosition    `json:"position"`
	Size     WidgetSize        `json:"size"`
	Config   map[string]string `json:"config,omitempty"`
	Data     interface{}       `json:"data,omitempty"`
}

// Layout 仪表盘布局.
type Layout struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Columns   int       `json:"columns"`   // 网格列数
	Rows      int       `json:"rows"`      // 网格行数
	Widgets   []*Widget `json:"widgets"`
	IsDefault bool      `json:"is_default"`
}

// Dashboard 用户仪表盘配置.
type Dashboard struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Layouts     []*Layout `json:"layouts"`
	ActiveLayout string   `json:"active_layout"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WidgetTemplate Widget 市场模板.
type WidgetTemplate struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        WidgetType `json:"type"`
	Author      string     `json:"author"`
	Version     string     `json:"version"`
	DefaultSize WidgetSize `json:"default_size"`
	Config      []ConfigField `json:"config,omitempty"`
	Downloads   int        `json:"downloads"`
	Rating      float64    `json:"rating"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ConfigField Widget 配置字段定义.
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // string, number, boolean, select
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
}

// CreateDashboardRequest 创建仪表盘请求.
type CreateDashboardRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Name   string `json:"name" binding:"required"`
}

// CreateLayoutRequest 创建布局请求.
type CreateLayoutRequest struct {
	Name    string `json:"name" binding:"required"`
	Columns int    `json:"columns"`
	Rows    int    `json:"rows"`
}

// AddWidgetRequest 添加 Widget 请求.
type AddWidgetRequest struct {
	Type     WidgetType        `json:"type" binding:"required"`
	Title    string            `json:"title"`
	Position WidgetPosition    `json:"position"`
	Size     WidgetSize        `json:"size"`
	Config   map[string]string `json:"config,omitempty"`
}

// UpdateWidgetRequest 更新 Widget 请求.
type UpdateWidgetRequest struct {
	Title    *string           `json:"title,omitempty"`
	Position *WidgetPosition   `json:"position,omitempty"`
	Size     *WidgetSize       `json:"size,omitempty"`
	Config   map[string]string `json:"config,omitempty"`
}

// NASStatusData NAS 状态数据.
type NASStatusData struct {
	CPU       CPUData    `json:"cpu"`
	Memory    MemoryData `json:"memory"`
	Disk      DiskData   `json:"disk"`
	Network   NetworkData `json:"network"`
	Uptime    int64      `json:"uptime"`
	Hostname  string     `json:"hostname"`
	Timestamp time.Time  `json:"timestamp"`
}

// CPUData CPU 数据.
type CPUData struct {
	UsagePercent float64   `json:"usage_percent"`
	Cores        int       `json:"cores"`
	Temperature  float64   `json:"temperature"`
	LoadAvg      [3]float64 `json:"load_avg"`
}

// MemoryData 内存数据.
type MemoryData struct {
	TotalGB     float64 `json:"total_gb"`
	UsedGB      float64 `json:"used_gb"`
	AvailableGB float64 `json:"available_gb"`
	UsagePercent float64 `json:"usage_percent"`
	SwapTotalGB float64 `json:"swap_total_gb"`
	SwapUsedGB  float64 `json:"swap_used_gb"`
}

// DiskData 磁盘数据.
type DiskData struct {
	TotalGB      float64       `json:"total_gb"`
	UsedGB       float64       `json:"used_gb"`
	FreeGB       float64       `json:"free_gb"`
	UsagePercent float64       `json:"usage_percent"`
	Volumes      []VolumeData  `json:"volumes"`
}

// VolumeData 卷数据.
type VolumeData struct {
	Name         string  `json:"name"`
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	FreeGB       float64 `json:"free_gb"`
	UsagePercent float64 `json:"usage_percent"`
}

// NetworkData 网络数据.
type NetworkData struct {
	Interfaces []InterfaceData `json:"interfaces"`
	TotalIn    int64           `json:"total_in"`
	TotalOut   int64           `json:"total_out"`
}

// InterfaceData 网卡数据.
type InterfaceData struct {
	Name    string `json:"name"`
	Speed   int64  `json:"speed"`
	RxBytes int64  `json:"rx_bytes"`
	TxBytes int64  `json:"tx_bytes"`
}

// DockerContainerData Docker 容器数据.
type DockerContainerData struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"` // running, stopped, paused
	Ports   string `json:"ports"`
}

// WeatherData 天气数据.
type WeatherData struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Humidity    int     `json:"humidity"`
	Condition   string  `json:"condition"`
	WindSpeed   float64 `json:"wind_speed"`
	Forecast    []DayForecast `json:"forecast"`
	Timestamp   time.Time `json:"timestamp"`
}

// DayForecast 每日天气预报.
type DayForecast struct {
	Date        string  `json:"date"`
	HighTemp    float64 `json:"high_temp"`
	LowTemp     float64 `json:"low_temp"`
	Condition   string  `json:"condition"`
}

// CalendarEvent 日历事件.
type CalendarEvent struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Location  string    `json:"location,omitempty"`
	AllDay    bool      `json:"all_day"`
}

// TodoItem 待办事项.
type TodoItem struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Completed bool       `json:"completed"`
	Priority  string     `json:"priority"` // low, medium, high
	DueDate   *time.Time `json:"due_date,omitempty"`
}

// QuickAction 快捷操作.
type QuickAction struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Icon    string `json:"icon"`
	Action  string `json:"action"` // restart, docker-restart, backup, update
	Enabled bool   `json:"enabled"`
}

// RecentFile 最近文件.
type RecentFile struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	IsDir     bool      `json:"is_dir"`
	ModifiedAt time.Time `json:"modified_at"`
}

// StorageTrend 存储使用趋势.
type StorageTrend struct {
	Date       time.Time `json:"date"`
	TotalGB    float64   `json:"total_gb"`
	UsedGB     float64   `json:"used_gb"`
	FreeGB     float64   `json:"free_gb"`
}

// WSMessage WebSocket 消息.
type WSMessage struct {
	Type    string      `json:"type"`    // widget_update, config_change, layout_change
	Payload interface{} `json:"payload"`
	Time    time.Time   `json:"time"`
}

// generateID 生成唯一 ID.
var idCounter struct {
	mu    sync.Mutex
	value int
}

func generateID() string {
	idCounter.mu.Lock()
	defer idCounter.mu.Unlock()
	idCounter.value++
	return fmt.Sprintf("hd-%d-%d", time.Now().UnixNano(), idCounter.value)
}
