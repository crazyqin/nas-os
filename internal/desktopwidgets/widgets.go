// Package desktopwidgets 桌面小组件系统
// 对标飞牛 fnOS 桌面拖拽，提供更丰富的桌面自定义能力
package desktopwidgets

import (
	"fmt"
	"sync"
	"time"
)

// WidgetType 小组件类型
type WidgetType string

const (
	WidgetTypeClock     WidgetType = "clock"      // 时钟
	WidgetTypeWeather   WidgetType = "weather"    // 天气
	WidgetTypeCPU       WidgetType = "cpu"        // CPU 监控
	WidgetTypeMemory    WidgetType = "memory"     // 内存监控
	WidgetTypeDisk      WidgetType = "disk"       // 磁盘监控
	WidgetTypeNetwork   WidgetType = "network"    // 网络监控
	WidgetTypeCalendar  WidgetType = "calendar"   // 日历
	WidgetTypeTodo      WidgetType = "todo"       // 待办事项
	WidgetTypeNotes     WidgetType = "notes"      // 便签
	WidgetTypeQuickAccess WidgetType = "quickaccess" // 快捷访问
	WidgetTypePhoto     WidgetType = "photo"      // 照片轮播
	WidgetTypeMusic     WidgetType = "music"      // 音乐播放
)

// WidgetPosition 小组件位置
type WidgetPosition struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
	ZIndex int `json:"zIndex"`
}

// WidgetConfig 小组件配置
type WidgetConfig struct {
	Title       string            `json:"title"`
	RefreshInterval int           `json:"refreshInterval"` // 刷新间隔（秒）
	Theme       string            `json:"theme"`           // 主题
	Opacity     float64           `json:"opacity"`         // 透明度 0-1
	Settings    map[string]string `json:"settings"`        // 自定义设置
}

// Widget 桌面小组件
type Widget struct {
	ID        string         `json:"id"`
	Type      WidgetType     `json:"type"`
	Position  WidgetPosition `json:"position"`
	Config    WidgetConfig   `json:"config"`
	UserID    string         `json:"userId"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// WidgetData 小组件数据
type WidgetData struct {
	WidgetID  string                 `json:"widgetId"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Error     string                 `json:"error,omitempty"`
}

// DesktopLayout 桌面布局
type DesktopLayout struct {
	UserID    string   `json:"userId"`
	Widgets   []Widget `json:"widgets"`
	Wallpaper string   `json:"wallpaper"`
	Theme     string   `json:"theme"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// WidgetManager 小组件管理器
type WidgetManager struct {
	mu       sync.RWMutex
	widgets  map[string]*Widget
	layouts  map[string]*DesktopLayout
	dataFunc map[WidgetType]func(*Widget) (*WidgetData, error)
}

// NewWidgetManager 创建小组件管理器
func NewWidgetManager() *WidgetManager {
	mgr := &WidgetManager{
		widgets:  make(map[string]*Widget),
		layouts:  make(map[string]*DesktopLayout),
		dataFunc: make(map[WidgetType]func(*Widget) (*WidgetData, error)),
	}
	mgr.registerBuiltinDataProviders()
	return mgr
}

// registerBuiltinDataProviders 注册内置数据提供者
func (m *WidgetManager) registerBuiltinDataProviders() {
	m.dataFunc[WidgetTypeClock] = func(w *Widget) (*WidgetData, error) {
		return &WidgetData{
			WidgetID:  w.ID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"time":     time.Now().Format("15:04:05"),
				"date":     time.Now().Format("2006-01-02"),
				"timezone": "Asia/Shanghai",
			},
		}, nil
	}

	m.dataFunc[WidgetTypeCPU] = func(w *Widget) (*WidgetData, error) {
		// 模拟 CPU 数据
		return &WidgetData{
			WidgetID:  w.ID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"usage":   45.2,
				"cores":   8,
				"temp":    62.5,
				"freq":    3200,
			},
		}, nil
	}

	m.dataFunc[WidgetTypeMemory] = func(w *Widget) (*WidgetData, error) {
		return &WidgetData{
			WidgetID:  w.ID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"total":     16384,
				"used":      8192,
				"free":      8192,
				"percent":   50.0,
			},
		}, nil
	}
}

// CreateWidget 创建小组件
func (m *WidgetManager) CreateWidget(widget *Widget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if widget.ID == "" {
		widget.ID = fmt.Sprintf("widget-%d", time.Now().UnixNano())
	}
	widget.CreatedAt = time.Now()
	widget.UpdatedAt = time.Now()

	m.widgets[widget.ID] = widget
	return nil
}

// UpdateWidget 更新小组件
func (m *WidgetManager) UpdateWidget(id string, update *Widget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	widget, exists := m.widgets[id]
	if !exists {
		return fmt.Errorf("小组件 %s 不存在", id)
	}

	if update.Position.Width > 0 {
		widget.Position = update.Position
	}
	if update.Config.Title != "" {
		widget.Config = update.Config
	}
	widget.UpdatedAt = time.Now()

	return nil
}

// DeleteWidget 删除小组件
func (m *WidgetManager) DeleteWidget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.widgets[id]; !exists {
		return fmt.Errorf("小组件 %s 不存在", id)
	}

	delete(m.widgets, id)
	return nil
}

// GetWidget 获取小组件
func (m *WidgetManager) GetWidget(id string) (*Widget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	widget, exists := m.widgets[id]
	if !exists {
		return nil, fmt.Errorf("小组件 %s 不存在", id)
	}

	return widget, nil
}

// ListWidgets 列出用户的所有小组件
func (m *WidgetManager) ListWidgets(userID string) []*Widget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Widget
	for _, w := range m.widgets {
		if w.UserID == userID {
			result = append(result, w)
		}
	}
	return result
}

// GetWidgetData 获取小组件数据
func (m *WidgetManager) GetWidgetData(id string) (*WidgetData, error) {
	m.mu.RLock()
	widget, exists := m.widgets[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("小组件 %s 不存在", id)
	}

	dataFunc, exists := m.dataFunc[widget.Type]
	if !exists {
		return nil, fmt.Errorf("不支持的小组件类型: %s", widget.Type)
	}

	return dataFunc(widget)
}

// SaveLayout 保存桌面布局
func (m *WidgetManager) SaveLayout(layout *DesktopLayout) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layout.UpdatedAt = time.Now()
	m.layouts[layout.UserID] = layout
	return nil
}

// GetLayout 获取桌面布局
func (m *WidgetManager) GetLayout(userID string) (*DesktopLayout, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	layout, exists := m.layouts[userID]
	if !exists {
		// 返回默认布局
		return &DesktopLayout{
			UserID:  userID,
			Widgets: []Widget{},
			Theme:   "default",
		}, nil
	}

	return layout, nil
}

// RegisterDataProvider 注册自定义数据提供者
func (m *WidgetManager) RegisterDataProvider(widgetType WidgetType, provider func(*Widget) (*WidgetData, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dataFunc[widgetType] = provider
}

// ExportLayout 导出布局配置
func (m *WidgetManager) ExportLayout(userID string) ([]byte, error) {
	layout, err := m.GetLayout(userID)
	if err != nil {
		return nil, err
	}

	// 简化实现，实际应使用 JSON
	_ = layout
	return []byte("{}"), nil
}
