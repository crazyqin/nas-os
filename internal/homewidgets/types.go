// Package homewidgets 家庭仪表盘组件
// 可配置的仪表盘小部件系统，支持多种组件类型和布局管理
package homewidgets

import (
	"fmt"
	"sync"
	"time"
)

// WidgetType 组件类型
type WidgetType string

const (
	WidgetStorageOverview WidgetType = "storage_overview" // 存储概览
	WidgetSystemStatus    WidgetType = "system_status"    // 系统状态
	WidgetRecentFiles     WidgetType = "recent_files"     // 最近文件
	WidgetWeather         WidgetType = "weather"          // 天气
	WidgetTodoList        WidgetType = "todo_list"        // 待办事项
	WidgetQuickActions    WidgetType = "quick_actions"    // 快捷操作
)

// WidgetSize 组件尺寸
type WidgetSize string

const (
	SizeSmall  WidgetSize = "small"  // 小尺寸 1x1
	SizeMedium WidgetSize = "medium" // 中尺寸 2x1
	SizeLarge  WidgetSize = "large"  // 大尺寸 2x2
)

// Widget 组件实例
type Widget struct {
	ID        string            `json:"id"`
	Type      WidgetType        `json:"type"`
	Title     string            `json:"title"`
	Position  Position          `json:"position"`
	Size      WidgetSize        `json:"size"`
	Config    map[string]string `json:"config,omitempty"` // 组件特定配置
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Position 组件位置
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Layout 布局配置
type Layout struct {
	Columns int      `json:"columns"` // 列数
	Widgets []Widget `json:"widgets"` // 组件列表
}

// Template 布局模板
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Layout      Layout    `json:"layout"`
	Thumbnail   string    `json:"thumbnail,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// StorageOverviewData 存储概览数据
type StorageOverviewData struct {
	TotalSpace   int64   `json:"total_space"`   // 总空间 (bytes)
	UsedSpace    int64   `json:"used_space"`    // 已用空间
	FreeSpace    int64   `json:"free_space"`    // 剩余空间
	UsedPercent  float64 `json:"used_percent"`  // 使用率
	TotalDisks   int     `json:"total_disks"`   // 磁盘数量
	HealthyDisks int     `json:"healthy_disks"` // 健康磁盘数
}

// SystemStatusData 系统状态数据
type SystemStatusData struct {
	CPUUsage    float64 `json:"cpu_usage"`    // CPU 使用率
	MemoryUsage float64 `json:"memory_usage"` // 内存使用率
	Uptime      int64   `json:"uptime"`       // 运行时间 (秒)
	Temperature float64 `json:"temperature"`  // CPU 温度
	NetworkIn   int64   `json:"network_in"`   // 网络入流量 (bytes/s)
	NetworkOut  int64   `json:"network_out"`  // 网络出流量 (bytes/s)
}

// RecentFilesData 最近文件数据
type RecentFilesData struct {
	Files []FileInfo `json:"files"` // 最近文件列表
}

// FileInfo 文件信息
type FileInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	IsDir     bool      `json:"is_dir"`
	Extension string    `json:"extension,omitempty"`
}

// WeatherData 天气数据
type WeatherData struct {
	City        string  `json:"city"`
	Temperature float64 `json:"temperature"` // 温度 (°C)
	Humidity    int     `json:"humidity"`    // 湿度 (%)
	Description string  `json:"description"` // 天气描述
	WindSpeed   float64 `json:"wind_speed"`  // 风速 (km/h)
	Icon        string  `json:"icon"`        // 天气图标
}

// TodoListData 待办事项数据
type TodoListData struct {
	Items []TodoItem `json:"items"`
}

// TodoItem 待办事项
type TodoItem struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Completed bool       `json:"completed"`
	DueDate   *time.Time `json:"due_date,omitempty"`
	Priority  string     `json:"priority"` // low/medium/high
}

// QuickActionsData 快捷操作数据
type QuickActionsData struct {
	Actions []QuickAction `json:"actions"`
}

// QuickAction 快捷操作
type QuickAction struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Icon    string `json:"icon"`
	Action  string `json:"action"` // 操作类型
	Enabled bool   `json:"enabled"`
}

// Manager 家庭仪表盘管理器
type Manager struct {
	mu        sync.RWMutex
	widgets   map[string]*Widget
	templates map[string]*Template
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		widgets:   make(map[string]*Widget),
		templates: make(map[string]*Template),
	}
	m.initDefaultTemplates()
	return m
}

// initDefaultTemplates 初始化默认模板
func (m *Manager) initDefaultTemplates() {
	// 基础布局模板
	m.templates["basic"] = &Template{
		ID:          "basic",
		Name:        "基础布局",
		Description: "包含存储概览和系统状态的基础仪表盘",
		Layout: Layout{
			Columns: 2,
			Widgets: []Widget{
				{
					ID:       "tpl_storage",
					Type:     WidgetStorageOverview,
					Title:    "存储概览",
					Position: Position{Row: 0, Col: 0},
					Size:     SizeMedium,
					Enabled:  true,
				},
				{
					ID:       "tpl_system",
					Type:     WidgetSystemStatus,
					Title:    "系统状态",
					Position: Position{Row: 0, Col: 1},
					Size:     SizeMedium,
					Enabled:  true,
				},
			},
		},
		CreatedAt: time.Now(),
	}

	// 完整布局模板
	m.templates["full"] = &Template{
		ID:          "full",
		Name:        "完整布局",
		Description: "包含所有组件类型的完整仪表盘",
		Layout: Layout{
			Columns: 2,
			Widgets: []Widget{
				{
					ID:       "tpl_storage_full",
					Type:     WidgetStorageOverview,
					Title:    "存储概览",
					Position: Position{Row: 0, Col: 0},
					Size:     SizeMedium,
					Enabled:  true,
				},
				{
					ID:       "tpl_system_full",
					Type:     WidgetSystemStatus,
					Title:    "系统状态",
					Position: Position{Row: 0, Col: 1},
					Size:     SizeMedium,
					Enabled:  true,
				},
				{
					ID:       "tpl_files_full",
					Type:     WidgetRecentFiles,
					Title:    "最近文件",
					Position: Position{Row: 1, Col: 0},
					Size:     SizeLarge,
					Enabled:  true,
				},
				{
					ID:       "tpl_weather_full",
					Type:     WidgetWeather,
					Title:    "天气",
					Position: Position{Row: 2, Col: 0},
					Size:     SizeSmall,
					Enabled:  true,
				},
				{
					ID:       "tpl_todo_full",
					Type:     WidgetTodoList,
					Title:    "待办事项",
					Position: Position{Row: 2, Col: 1},
					Size:     SizeSmall,
					Enabled:  true,
				},
				{
					ID:       "tpl_actions_full",
					Type:     WidgetQuickActions,
					Title:    "快捷操作",
					Position: Position{Row: 3, Col: 0},
					Size:     SizeLarge,
					Enabled:  true,
				},
			},
		},
		CreatedAt: time.Now(),
	}
}

// GetWidgets 获取所有组件
func (m *Manager) GetWidgets() []Widget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Widget, 0, len(m.widgets))
	for _, w := range m.widgets {
		result = append(result, *w)
	}
	return result
}

// GetWidget 获取单个组件
func (m *Manager) GetWidget(id string) (*Widget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	widget, exists := m.widgets[id]
	if !exists {
		return nil, fmt.Errorf("组件不存在: %s", id)
	}
	return widget, nil
}

// AddWidget 添加组件
func (m *Manager) AddWidget(widget *Widget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if widget.ID == "" {
		return fmt.Errorf("组件ID不能为空")
	}
	if _, exists := m.widgets[widget.ID]; exists {
		return fmt.Errorf("组件已存在: %s", widget.ID)
	}

	widget.CreatedAt = time.Now()
	widget.UpdatedAt = time.Now()
	widget.Enabled = true
	m.widgets[widget.ID] = widget
	return nil
}

// UpdateWidget 更新组件
func (m *Manager) UpdateWidget(widget *Widget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.widgets[widget.ID]; !exists {
		return fmt.Errorf("组件不存在: %s", widget.ID)
	}

	widget.UpdatedAt = time.Now()
	m.widgets[widget.ID] = widget
	return nil
}

// DeleteWidget 删除组件
func (m *Manager) DeleteWidget(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.widgets[id]; !exists {
		return false
	}
	delete(m.widgets, id)
	return true
}

// UpdateLayout 更新布局
func (m *Manager) UpdateLayout(layout Layout) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证布局中的组件都存在
	for _, w := range layout.Widgets {
		if _, exists := m.widgets[w.ID]; !exists {
			return fmt.Errorf("布局引用了不存在的组件: %s", w.ID)
		}
	}

	// 更新组件位置和尺寸
	for _, w := range layout.Widgets {
		if existing, exists := m.widgets[w.ID]; exists {
			existing.Position = w.Position
			existing.Size = w.Size
			existing.UpdatedAt = time.Now()
		}
	}
	return nil
}

// GetTemplates 获取所有模板
func (m *Manager) GetTemplates() []Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Template, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, *t)
	}
	return result
}

// ApplyTemplate 应用模板
func (m *Manager) ApplyTemplate(templateID string) (*Layout, error) {
	m.mu.RLock()
	template, exists := m.templates[templateID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空当前组件
	m.widgets = make(map[string]*Widget)

	// 从模板创建组件
	for _, tplWidget := range template.Layout.Widgets {
		widget := tplWidget
		widget.ID = fmt.Sprintf("%s_%d", tplWidget.Type, time.Now().UnixNano())
		widget.CreatedAt = time.Now()
		widget.UpdatedAt = time.Now()
		m.widgets[widget.ID] = &widget
	}

	// 构建布局响应
	layout := Layout{
		Columns: template.Layout.Columns,
		Widgets: make([]Widget, 0, len(m.widgets)),
	}
	for _, w := range m.widgets {
		layout.Widgets = append(layout.Widgets, *w)
	}

	return &layout, nil
}

// GetWidgetData 获取组件数据
func (m *Manager) GetWidgetData(widgetType WidgetType) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch widgetType {
	case WidgetStorageOverview:
		return m.getStorageOverviewData(), nil
	case WidgetSystemStatus:
		return m.getSystemStatusData(), nil
	case WidgetRecentFiles:
		return m.getRecentFilesData(), nil
	case WidgetWeather:
		return m.getWeatherData(), nil
	case WidgetTodoList:
		return m.getTodoListData(), nil
	case WidgetQuickActions:
		return m.getQuickActionsData(), nil
	default:
		return nil, fmt.Errorf("不支持的组件类型: %s", widgetType)
	}
}

// getStorageOverviewData 获取存储概览数据 (模拟)
func (m *Manager) getStorageOverviewData() *StorageOverviewData {
	return &StorageOverviewData{
		TotalSpace:   8 * 1024 * 1024 * 1024 * 1024, // 8TB
		UsedSpace:    3 * 1024 * 1024 * 1024 * 1024, // 3TB
		FreeSpace:    5 * 1024 * 1024 * 1024 * 1024, // 5TB
		UsedPercent:  37.5,
		TotalDisks:   4,
		HealthyDisks: 4,
	}
}

// getSystemStatusData 获取系统状态数据 (模拟)
func (m *Manager) getSystemStatusData() *SystemStatusData {
	return &SystemStatusData{
		CPUUsage:    25.5,
		MemoryUsage: 45.2,
		Uptime:      86400 * 7, // 7天
		Temperature: 42.0,
		NetworkIn:   1024 * 100, // 100KB/s
		NetworkOut:  1024 * 50,  // 50KB/s
	}
}

// getRecentFilesData 获取最近文件数据 (模拟)
func (m *Manager) getRecentFilesData() *RecentFilesData {
	now := time.Now()
	return &RecentFilesData{
		Files: []FileInfo{
			{Name: "report.pdf", Path: "/documents/report.pdf", Size: 1024 * 1024, Modified: now.Add(-1 * time.Hour), Extension: ".pdf"},
			{Name: "photo.jpg", Path: "/photos/photo.jpg", Size: 3 * 1024 * 1024, Modified: now.Add(-2 * time.Hour), Extension: ".jpg"},
			{Name: "backup.zip", Path: "/backups/backup.zip", Size: 500 * 1024 * 1024, Modified: now.Add(-5 * time.Hour), Extension: ".zip"},
			{Name: "notes.txt", Path: "/documents/notes.txt", Size: 2048, Modified: now.Add(-12 * time.Hour), Extension: ".txt"},
			{Name: "video.mp4", Path: "/videos/video.mp4", Size: 1024 * 1024 * 500, Modified: now.Add(-24 * time.Hour), Extension: ".mp4"},
		},
	}
}

// getWeatherData 获取天气数据 (模拟)
func (m *Manager) getWeatherData() *WeatherData {
	return &WeatherData{
		City:        "上海",
		Temperature: 22.5,
		Humidity:    65,
		Description: "多云",
		WindSpeed:   12.0,
		Icon:        "cloudy",
	}
}

// getTodoListData 获取待办事项数据 (模拟)
func (m *Manager) getTodoListData() *TodoListData {
	dueDate := time.Now().Add(24 * time.Hour)
	return &TodoListData{
		Items: []TodoItem{
			{ID: "todo_1", Title: "备份照片", Completed: false, DueDate: &dueDate, Priority: "high"},
			{ID: "todo_2", Title: "清理临时文件", Completed: true, Priority: "low"},
			{ID: "todo_3", Title: "检查磁盘健康", Completed: false, Priority: "medium"},
		},
	}
}

// getQuickActionsData 获取快捷操作数据 (模拟)
func (m *Manager) getQuickActionsData() *QuickActionsData {
	return &QuickActionsData{
		Actions: []QuickAction{
			{ID: "action_1", Name: "创建快照", Icon: "camera", Action: "create_snapshot", Enabled: true},
			{ID: "action_2", Name: "备份数据", Icon: "backup", Action: "run_backup", Enabled: true},
			{ID: "action_3", Name: "扫描病毒", Icon: "shield", Action: "scan_virus", Enabled: true},
			{ID: "action_4", Name: "系统更新", Icon: "update", Action: "system_update", Enabled: true},
			{ID: "action_5", Name: "重启服务", Icon: "refresh", Action: "restart_services", Enabled: true},
		},
	}
}
