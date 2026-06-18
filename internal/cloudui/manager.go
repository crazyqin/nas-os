package cloudui

import (
	"fmt"
	"html/template"
	"sync"
)

// Theme 主题
type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
	ThemeAuto  Theme = "auto"
)

// Widget 小部件
type Widget struct {
	ID       string
	Name     string
	Type     string // chart, gauge, table, map, status
	Position Position
	Size     Size
	Config   map[string]interface{}
}

// Position 位置
type Position struct {
	X int
	Y int
}

// Size 大小
type Size struct {
	Width  int
	Height int
}

// Dashboard 仪表板
type Dashboard struct {
	ID        string
	Name      string
	Theme     Theme
	Widgets   []Widget
	Layout    string // grid, flex, custom
	Columns   int
	UpdatedAt string
}

// CloudUIManager 云UI管理器
type CloudUIManager struct {
	dashboards map[string]*Dashboard
	theme      Theme
	mu         sync.RWMutex
	config     UIConfig
	templates  map[string]*template.Template
}

// UIConfig UI配置
type UIConfig struct {
	DefaultTheme    Theme
	EnableAnimations bool
	EnableNotifications bool
	RefreshInterval int // 秒
	MaxDashboards   int
}

// NewCloudUIManager 创建云UI管理器
func NewCloudUIManager(config UIConfig) *CloudUIManager {
	return &CloudUIManager{
		dashboards: make(map[string]*Dashboard),
		theme:      config.DefaultTheme,
		config:     config,
		templates:  make(map[string]*template.Template),
	}
}

// CreateDashboard 创建仪表板
func (m *CloudUIManager) CreateDashboard(dashboard *Dashboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.dashboards[dashboard.ID]; exists {
		return fmt.Errorf("dashboard already exists: %s", dashboard.ID)
	}

	if len(m.dashboards) >= m.config.MaxDashboards {
		return fmt.Errorf("maximum number of dashboards reached: %d", m.config.MaxDashboards)
	}

	// 设置默认值
	if dashboard.Theme == "" {
		dashboard.Theme = m.theme
	}
	if dashboard.Columns == 0 {
		dashboard.Columns = 3
	}

	m.dashboards[dashboard.ID] = dashboard
	return nil
}

// GetDashboard 获取仪表板
func (m *CloudUIManager) GetDashboard(id string) (*Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, exists := m.dashboards[id]
	if !exists {
		return nil, fmt.Errorf("dashboard not found: %s", id)
	}

	return dashboard, nil
}

// UpdateDashboard 更新仪表板
func (m *CloudUIManager) UpdateDashboard(dashboard *Dashboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.dashboards[dashboard.ID]; !exists {
		return fmt.Errorf("dashboard not found: %s", dashboard.ID)
	}

	m.dashboards[dashboard.ID] = dashboard
	return nil
}

// DeleteDashboard 删除仪表板
func (m *CloudUIManager) DeleteDashboard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.dashboards[id]; !exists {
		return fmt.Errorf("dashboard not found: %s", id)
	}

	delete(m.dashboards, id)
	return nil
}

// ListDashboards 列出所有仪表板
func (m *CloudUIManager) ListDashboards() []*Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboards := make([]*Dashboard, 0, len(m.dashboards))
	for _, dashboard := range m.dashboards {
		dashboards = append(dashboards, dashboard)
	}

	return dashboards
}

// AddWidget 添加小部件
func (m *CloudUIManager) AddWidget(dashboardID string, widget Widget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dashboard, exists := m.dashboards[dashboardID]
	if !exists {
		return fmt.Errorf("dashboard not found: %s", dashboardID)
	}

	// 检查是否已存在
	for _, w := range dashboard.Widgets {
		if w.ID == widget.ID {
			return fmt.Errorf("widget already exists: %s", widget.ID)
		}
	}

	dashboard.Widgets = append(dashboard.Widgets, widget)
	return nil
}

// RemoveWidget 移除小部件
func (m *CloudUIManager) RemoveWidget(dashboardID, widgetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dashboard, exists := m.dashboards[dashboardID]
	if !exists {
		return fmt.Errorf("dashboard not found: %s", dashboardID)
	}

	for i, widget := range dashboard.Widgets {
		if widget.ID == widgetID {
			dashboard.Widgets = append(dashboard.Widgets[:i], dashboard.Widgets[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("widget not found: %s", widgetID)
}

// UpdateWidget 更新小部件
func (m *CloudUIManager) UpdateWidget(dashboardID string, widget Widget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dashboard, exists := m.dashboards[dashboardID]
	if !exists {
		return fmt.Errorf("dashboard not found: %s", dashboardID)
	}

	for i, w := range dashboard.Widgets {
		if w.ID == widget.ID {
			dashboard.Widgets[i] = widget
			return nil
		}
	}

	return fmt.Errorf("widget not found: %s", widget.ID)
}

// SetTheme 设置主题
func (m *CloudUIManager) SetTheme(theme Theme) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.theme = theme
}

// GetTheme 获取主题
func (m *CloudUIManager) GetTheme() Theme {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.theme
}

// RenderDashboard 渲染仪表板
func (m *CloudUIManager) RenderDashboard(id string) (string, error) {
	m.mu.RLock()
	dashboard, exists := m.dashboards[id]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("dashboard not found: %s", id)
	}

	// 生成HTML
	html := m.generateDashboardHTML(dashboard)
	return html, nil
}

// generateDashboardHTML 生成仪表板HTML
func (m *CloudUIManager) generateDashboardHTML(dashboard *Dashboard) string {
	// 简化实现，返回基本HTML
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .dashboard { display: grid; grid-template-columns: repeat(%d, 1fr); gap: 20px; }
        .widget { border: 1px solid #ccc; padding: 15px; border-radius: 8px; }
        .theme-dark { background: #1a1a1a; color: white; }
        .theme-light { background: white; color: black; }
    </style>
</head>
<body class="theme-%s">
    <h1>%s</h1>
    <div class="dashboard">`, dashboard.Name, dashboard.Columns, dashboard.Theme, dashboard.Name)

	for _, widget := range dashboard.Widgets {
		html += fmt.Sprintf(`
        <div class="widget" id="%s">
            <h3>%s</h3>
            <p>Type: %s</p>
        </div>`, widget.ID, widget.Name, widget.Type)
	}

	html += `
    </div>
</body>
</html>`

	return html
}

// ExportDashboard 导出仪表板配置
func (m *CloudUIManager) ExportDashboard(id string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, exists := m.dashboards[id]
	if !exists {
		return nil, fmt.Errorf("dashboard not found: %s", id)
	}

	export := map[string]interface{}{
		"id":        dashboard.ID,
		"name":      dashboard.Name,
		"theme":     dashboard.Theme,
		"layout":    dashboard.Layout,
		"columns":   dashboard.Columns,
		"widgets":   dashboard.Widgets,
	}

	return export, nil
}

// ImportDashboard 导入仪表板配置
func (m *CloudUIManager) ImportDashboard(config map[string]interface{}) error {
	// 处理Theme类型转换
	var theme Theme
	switch t := config["theme"].(type) {
	case Theme:
		theme = t
	case string:
		theme = Theme(t)
	default:
		theme = m.theme
	}

	// 处理Columns类型转换
	var columns int
	switch c := config["columns"].(type) {
	case int:
		columns = c
	case float64:
		columns = int(c)
	default:
		columns = 3
	}

	dashboard := &Dashboard{
		ID:      config["id"].(string),
		Name:    config["name"].(string),
		Theme:   theme,
		Layout:  config["layout"].(string),
		Columns: columns,
	}

	return m.CreateDashboard(dashboard)
}

// GetWidgetData 获取小部件数据
func (m *CloudUIManager) GetWidgetData(dashboardID, widgetID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, exists := m.dashboards[dashboardID]
	if !exists {
		return nil, fmt.Errorf("dashboard not found: %s", dashboardID)
	}

	for _, widget := range dashboard.Widgets {
		if widget.ID == widgetID {
			// 返回模拟数据
			return map[string]interface{}{
				"id":     widget.ID,
				"name":   widget.Name,
				"type":   widget.Type,
				"config": widget.Config,
			}, nil
		}
	}

	return nil, fmt.Errorf("widget not found: %s", widgetID)
}
