// Package homedashboard 提供仪表盘核心管理逻辑
package homedashboard

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Manager 仪表盘管理器.
type Manager struct {
	mu          sync.RWMutex
	dashboards  map[string]*Dashboard   // dashboardID -> Dashboard
	templates   map[string]*WidgetTemplate // templateID -> Template
	widgetData  map[string]interface{}  // widgetID -> cached data
	subscribers map[string][]chan WSMessage // dashboardID -> subscriber channels
}

// NewManager 创建仪表盘管理器.
func NewManager() *Manager {
	return &Manager{
		dashboards:  make(map[string]*Dashboard),
		templates:   make(map[string]*WidgetTemplate),
		widgetData:  make(map[string]interface{}),
		subscribers: make(map[string][]chan WSMessage),
	}
}

// ========== 仪表盘 CRUD ==========

// CreateDashboard 创建仪表盘.
func (m *Manager) CreateDashboard(req CreateDashboardRequest) *Dashboard {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	defaultLayout := &Layout{
		ID:        generateID(),
		Name:      "默认布局",
		Columns:   12,
		Rows:      8,
		Widgets:   []*Widget{},
		IsDefault: true,
	}

	dashboard := &Dashboard{
		ID:           generateID(),
		UserID:       req.UserID,
		Name:         req.Name,
		Layouts:      []*Layout{defaultLayout},
		ActiveLayout: defaultLayout.ID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.dashboards[dashboard.ID] = dashboard
	return dashboard
}

// GetDashboard 获取仪表盘.
func (m *Manager) GetDashboard(id string) (*Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", id)
	}
	return d, nil
}

// ListDashboards 列出用户的仪表盘.
func (m *Manager) ListDashboards(userID string) []*Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Dashboard
	for _, d := range m.dashboards {
		if userID == "" || d.UserID == userID {
			result = append(result, d)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// UpdateDashboard 更新仪表盘.
func (m *Manager) UpdateDashboard(id, name string) (*Dashboard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", id)
	}

	if name != "" {
		d.Name = name
	}
	d.UpdatedAt = time.Now()
	return d, nil
}

// DeleteDashboard 删除仪表盘.
func (m *Manager) DeleteDashboard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.dashboards[id]; !ok {
		return fmt.Errorf("dashboard %q not found", id)
	}
	delete(m.dashboards, id)
	return nil
}

// ========== 布局管理 ==========

// AddLayout 添加布局.
func (m *Manager) AddLayout(dashboardID string, req CreateLayoutRequest) (*Layout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", dashboardID)
	}

	columns := req.Columns
	if columns <= 0 {
		columns = 12
	}
	rows := req.Rows
	if rows <= 0 {
		rows = 8
	}

	layout := &Layout{
		ID:      generateID(),
		Name:    req.Name,
		Columns: columns,
		Rows:    rows,
		Widgets: []*Widget{},
	}

	d.Layouts = append(d.Layouts, layout)
	d.UpdatedAt = time.Now()
	return layout, nil
}

// SetActiveLayout 设置活动布局.
func (m *Manager) SetActiveLayout(dashboardID, layoutID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard %q not found", dashboardID)
	}

	for _, l := range d.Layouts {
		if l.ID == layoutID {
			d.ActiveLayout = layoutID
			d.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("layout %q not found", layoutID)
}

// DeleteLayout 删除布局.
func (m *Manager) DeleteLayout(dashboardID, layoutID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard %q not found", dashboardID)
	}

	for i, l := range d.Layouts {
		if l.ID == layoutID {
			if l.IsDefault {
				return fmt.Errorf("cannot delete default layout")
			}
			d.Layouts = append(d.Layouts[:i], d.Layouts[i+1:]...)
			d.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("layout %q not found", layoutID)
}

// ========== Widget 管理 ==========

// AddWidget 添加 Widget.
func (m *Manager) AddWidget(dashboardID, layoutID string, req AddWidgetRequest) (*Widget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", dashboardID)
	}

	var layout *Layout
	for _, l := range d.Layouts {
		if l.ID == layoutID {
			layout = l
			break
		}
	}
	if layout == nil {
		return nil, fmt.Errorf("layout %q not found", layoutID)
	}

	// 设置默认尺寸
	size := req.Size
	if size.Width <= 0 {
		size.Width = 4
	}
	if size.Height <= 0 {
		size.Height = 2
	}

	widget := &Widget{
		ID:       generateID(),
		Type:     req.Type,
		Title:    req.Title,
		Position: req.Position,
		Size:     size,
		Config:   req.Config,
	}

	layout.Widgets = append(layout.Widgets, widget)
	d.UpdatedAt = time.Now()

	return widget, nil
}

// GetWidget 获取 Widget.
func (m *Manager) GetWidget(dashboardID, layoutID, widgetID string) (*Widget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", dashboardID)
	}

	for _, l := range d.Layouts {
		if l.ID == layoutID {
			for _, w := range l.Widgets {
				if w.ID == widgetID {
					return w, nil
				}
			}
			return nil, fmt.Errorf("widget %q not found", widgetID)
		}
	}

	return nil, fmt.Errorf("layout %q not found", layoutID)
}

// UpdateWidget 更新 Widget.
func (m *Manager) UpdateWidget(dashboardID, layoutID, widgetID string, req UpdateWidgetRequest) (*Widget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", dashboardID)
	}

	for _, l := range d.Layouts {
		if l.ID == layoutID {
			for _, w := range l.Widgets {
				if w.ID == widgetID {
					if req.Title != nil {
						w.Title = *req.Title
					}
					if req.Position != nil {
						w.Position = *req.Position
					}
					if req.Size != nil {
						w.Size = *req.Size
					}
					if req.Config != nil {
						w.Config = req.Config
					}
					d.UpdatedAt = time.Now()
					return w, nil
				}
			}
			return nil, fmt.Errorf("widget %q not found", widgetID)
		}
	}

	return nil, fmt.Errorf("layout %q not found", layoutID)
}

// DeleteWidget 删除 Widget.
func (m *Manager) DeleteWidget(dashboardID, layoutID, widgetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard %q not found", dashboardID)
	}

	for _, l := range d.Layouts {
		if l.ID == layoutID {
			for i, w := range l.Widgets {
				if w.ID == widgetID {
					l.Widgets = append(l.Widgets[:i], l.Widgets[i+1:]...)
					d.UpdatedAt = time.Now()
					return nil
				}
			}
			return fmt.Errorf("widget %q not found", widgetID)
		}
	}

	return fmt.Errorf("layout %q not found", layoutID)
}

// ========== Widget 市场 ==========

// RegisterTemplate 注册 Widget 模板.
func (m *Manager) RegisterTemplate(t *WidgetTemplate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.templates[t.ID] = t
}

// GetTemplate 获取 Widget 模板.
func (m *Manager) GetTemplate(id string) (*WidgetTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.templates[id]
	if !ok {
		return nil, fmt.Errorf("template %q not found", id)
	}
	return t, nil
}

// ListTemplates 列出 Widget 模板.
func (m *Manager) ListTemplates(widgetType WidgetType) []*WidgetTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*WidgetTemplate
	for _, t := range m.templates {
		if widgetType == "" || t.Type == widgetType {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Downloads > result[j].Downloads
	})
	return result
}

// DownloadTemplate 增加模板下载计数.
func (m *Manager) DownloadTemplate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.templates[id]
	if !ok {
		return fmt.Errorf("template %q not found", id)
	}
	t.Downloads++
	return nil
}

// RateTemplate 评价模板.
func (m *Manager) RateTemplate(id string, rating float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rating < 0 || rating > 5 {
		return fmt.Errorf("rating must be between 0 and 5")
	}

	t, ok := m.templates[id]
	if !ok {
		return fmt.Errorf("template %q not found", id)
	}

	// 简单加权平均
	total := float64(t.Downloads)
	if total == 0 {
		total = 1
	}
	t.Rating = (t.Rating*total + rating) / (total + 1)
	return nil
}

// ========== Widget 数据缓存 ==========

// SetWidgetData 设置 Widget 缓存数据.
func (m *Manager) SetWidgetData(widgetID string, data interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.widgetData[widgetID] = data
}

// GetWidgetData 获取 Widget 缓存数据.
func (m *Manager) GetWidgetData(widgetID string) interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.widgetData[widgetID]
}

// ========== WebSocket 订阅 ==========

// Subscribe 订阅仪表盘更新.
func (m *Manager) Subscribe(dashboardID string) chan WSMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan WSMessage, 100)
	m.subscribers[dashboardID] = append(m.subscribers[dashboardID], ch)
	return ch
}

// Unsubscribe 取消订阅.
func (m *Manager) Unsubscribe(dashboardID string, ch chan WSMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs := m.subscribers[dashboardID]
	for i, s := range subs {
		if s == ch {
			m.subscribers[dashboardID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// NotifySubscribers 通知订阅者.
func (m *Manager) NotifySubscribers(dashboardID string, msg WSMessage) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.subscribers[dashboardID] {
		select {
		case ch <- msg:
		default:
			// 跳过满通道
		}
	}
}

// ========== 数据统计 ==========

// DashboardCount 返回仪表盘数量.
func (m *Manager) DashboardCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.dashboards)
}

// TemplateCount 返回模板数量.
func (m *Manager) TemplateCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.templates)
}
