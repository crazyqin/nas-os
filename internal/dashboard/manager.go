package dashboard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrDashboardNotFound = errors.New("dashboard not found")
	ErrDashboardExists   = errors.New("dashboard already exists")
	ErrInvalidInput      = errors.New("invalid input")
	ErrWidgetNotFound    = errors.New("widget not found")
)

// Manager 仪表板管理器
type Manager struct {
	mu         sync.RWMutex
	dashboards map[string]*Dashboard
	templates  map[string]*DashboardTemplate
	storePath  string
	logger     *zap.Logger
}

// NewManager 创建仪表板管理器
func NewManager(storePath string, logger *zap.Logger) *Manager {
	m := &Manager{
		dashboards: make(map[string]*Dashboard),
		templates:  make(map[string]*DashboardTemplate),
		storePath:  storePath,
		logger:     logger,
	}

	// 加载预定义模板
	for i := range DefaultTemplates {
		t := &DefaultTemplates[i]
		m.templates[t.ID] = t
	}

	// 加载持久化数据
	if err := m.load(); err != nil && logger != nil {
		logger.Warn("failed to load dashboards", zap.Error(err))
	}

	return m
}

// ========== CRUD 操作 ==========

// Create 创建仪表板
func (m *Manager) Create(input DashboardInput, owner string) (*Dashboard, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成唯一 ID
	id := generateID()

	now := time.Now()
	dashboard := &Dashboard{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		Layout:      input.Layout,
		Widgets:     input.Widgets,
		Owner:       owner,
		IsPublic:    input.IsPublic,
		IsDefault:   input.IsDefault,
		CreatedAt:   now,
		UpdatedAt:   now,
		Tags:        input.Tags,
		RefreshRate: input.RefreshRate,
	}

	// 设置默认布局
	if dashboard.Layout.Columns == 0 {
		dashboard.Layout = DefaultLayoutConfig()
	}

	// 为小组件生成 ID
	for i := range dashboard.Widgets {
		if dashboard.Widgets[i].ID == "" {
			dashboard.Widgets[i].ID = generateID()
		}
	}

	m.dashboards[id] = dashboard

	// 持久化
	if err := m.save(); err != nil {
		delete(m.dashboards, id)
		return nil, err
	}

	return dashboard, nil
}

// Get 获取仪表板
func (m *Manager) Get(id string) (*Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, ok := m.dashboards[id]
	if !ok {
		return nil, ErrDashboardNotFound
	}

	return dashboard, nil
}

// List 列出仪表板
func (m *Manager) List(filter ListFilter) []Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Dashboard
	for _, d := range m.dashboards {
		// 应用过滤器
		if filter.Owner != "" && d.Owner != filter.Owner {
			continue
		}
		if !filter.IncludePrivate && !d.IsPublic && d.Owner != filter.Owner {
			continue
		}
		if filter.Tag != "" && !containsTag(d.Tags, filter.Tag) {
			continue
		}
		if filter.IsDefault != nil && d.IsDefault != *filter.IsDefault {
			continue
		}

		result = append(result, *d)
	}

	// 按更新时间排序
	sortByUpdatedAt(result)
	return result
}

// ListFilter 列表过滤器
type ListFilter struct {
	Owner          string
	Tag            string
	IncludePrivate bool
	IsDefault      *bool
}

// Update 更新仪表板
func (m *Manager) Update(id string, input DashboardInput) (*Dashboard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dashboard, ok := m.dashboards[id]
	if !ok {
		return nil, ErrDashboardNotFound
	}

	// 更新字段
	if input.Name != "" {
		dashboard.Name = input.Name
	}
	dashboard.Description = input.Description
	dashboard.Layout = input.Layout
	dashboard.Widgets = input.Widgets
	dashboard.IsPublic = input.IsPublic
	dashboard.IsDefault = input.IsDefault
	dashboard.Tags = input.Tags
	dashboard.RefreshRate = input.RefreshRate
	dashboard.UpdatedAt = time.Now()

	// 为新小组件生成 ID
	for i := range dashboard.Widgets {
		if dashboard.Widgets[i].ID == "" {
			dashboard.Widgets[i].ID = generateID()
		}
	}

	// 持久化
	if err := m.save(); err != nil {
		return nil, err
	}

	return dashboard, nil
}

// Delete 删除仪表板
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.dashboards[id]; !ok {
		return ErrDashboardNotFound
	}

	delete(m.dashboards, id)

	return m.save()
}

// ========== 小组件操作 ==========

// AddWidget 添加小组件
func (m *Manager) AddWidget(dashboardID string, widget Widget) (*Widget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, ErrDashboardNotFound
	}

	// 生成 ID
	if widget.ID == "" {
		widget.ID = generateID()
	}

	widget.LastUpdate = time.Now()
	dashboard.Widgets = append(dashboard.Widgets, widget)
	dashboard.UpdatedAt = time.Now()

	if err := m.save(); err != nil {
		// 回滚
		dashboard.Widgets = dashboard.Widgets[:len(dashboard.Widgets)-1]
		return nil, err
	}

	return &widget, nil
}

// UpdateWidget 更新小组件
func (m *Manager) UpdateWidget(dashboardID, widgetID string, updates Widget) (*Widget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, ErrDashboardNotFound
	}

	for i, w := range dashboard.Widgets {
		if w.ID == widgetID {
			// 保留 ID，更新其他字段
			updates.ID = widgetID
			updates.LastUpdate = time.Now()
			dashboard.Widgets[i] = updates
			dashboard.UpdatedAt = time.Now()

			if err := m.save(); err != nil {
				return nil, err
			}

			return &dashboard.Widgets[i], nil
		}
	}

	return nil, ErrWidgetNotFound
}

// RemoveWidget 移除小组件
func (m *Manager) RemoveWidget(dashboardID, widgetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		return ErrDashboardNotFound
	}

	for i, w := range dashboard.Widgets {
		if w.ID == widgetID {
			dashboard.Widgets = append(dashboard.Widgets[:i], dashboard.Widgets[i+1:]...)
			dashboard.UpdatedAt = time.Now()
			return m.save()
		}
	}

	return ErrWidgetNotFound
}

// ========== 模板支持 ==========

// ListTemplates 列出模板
func (m *Manager) ListTemplates() []DashboardTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []DashboardTemplate
	for _, t := range m.templates {
		result = append(result, *t)
	}
	return result
}

// GetTemplate 获取模板
func (m *Manager) GetTemplate(id string) (*DashboardTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	template, ok := m.templates[id]
	if !ok {
		return nil, ErrDashboardNotFound
	}

	return template, nil
}

// CreateFromTemplate 从模板创建仪表板
func (m *Manager) CreateFromTemplate(templateID, name string, owner string) (*Dashboard, error) {
	template, err := m.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	input := DashboardInput{
		Name:        name,
		Description: template.Description,
		Layout:      template.Layout,
		Widgets:     make([]Widget, len(template.Widgets)),
		IsPublic:    template.IsPublic,
	}

	// 复制小组件
	copy(input.Widgets, template.Widgets)
	// 为小组件生成新 ID
	for i := range input.Widgets {
		input.Widgets[i].ID = generateID()
	}

	return m.Create(input, owner)
}

// SaveAsTemplate 将仪表板保存为模板
func (m *Manager) SaveAsTemplate(dashboardID, templateName, category string) (*DashboardTemplate, error) {
	dashboard, err := m.Get(dashboardID)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	template := &DashboardTemplate{
		ID:          generateID(),
		Name:        templateName,
		Description: dashboard.Description,
		Category:    category,
		Layout:      dashboard.Layout,
		Widgets:     make([]Widget, len(dashboard.Widgets)),
		IsPublic:    dashboard.IsPublic,
		CreatedAt:   time.Now(),
	}

	copy(template.Widgets, dashboard.Widgets)
	// 为模板小组件生成新 ID
	for i := range template.Widgets {
		template.Widgets[i].ID = generateID()
	}

	m.templates[template.ID] = template
	return template, nil
}

// ========== 导入导出 ==========

// Export 导出仪表板
func (m *Manager) Export(ids []string) (*DashboardExport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	export := &DashboardExport{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Dashboards: make([]Dashboard, 0, len(ids)),
	}

	if len(ids) == 0 {
		// 导出所有
		for _, d := range m.dashboards {
			export.Dashboards = append(export.Dashboards, *d)
		}
	} else {
		// 导出指定的
		for _, id := range ids {
			if d, ok := m.dashboards[id]; ok {
				export.Dashboards = append(export.Dashboards, *d)
			}
		}
	}

	return export, nil
}

// Import 导入仪表板
func (m *Manager) Import(export *DashboardExport, owner string, overwrite bool) ([]Dashboard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var imported []Dashboard
	var originalDashboards = make(map[string]*Dashboard)

	// 备份原始数据以便回滚
	for k, v := range m.dashboards {
		originalDashboards[k] = v
	}

	for _, d := range export.Dashboards {
		// 生成新 ID 避免冲突
		newID := generateID()

		// 如果要覆盖且存在相同名称
		if overwrite {
			for existingID, existing := range m.dashboards {
				if existing.Name == d.Name && existing.Owner == owner {
					newID = existingID
					break
				}
			}
		}

		dashboard := &Dashboard{
			ID:          newID,
			Name:        d.Name,
			Description: d.Description,
			Layout:      d.Layout,
			Widgets:     d.Widgets,
			Owner:       owner,
			IsPublic:    d.IsPublic,
			IsDefault:   false, // 导入的不设为默认
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Tags:        d.Tags,
			RefreshRate: d.RefreshRate,
		}

		// 为小组件生成新 ID
		for i := range dashboard.Widgets {
			dashboard.Widgets[i].ID = generateID()
		}

		m.dashboards[newID] = dashboard
		imported = append(imported, *dashboard)
	}

	// 持久化
	if err := m.save(); err != nil {
		// 回滚
		m.dashboards = originalDashboards
		return nil, err
	}

	return imported, nil
}

// ExportToJSON 导出为 JSON 文件
func (m *Manager) ExportToJSON(ids []string, filePath string) error {
	export, err := m.Export(ids)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// ImportFromJSON 从 JSON 文件导入
func (m *Manager) ImportFromJSON(filePath string, owner string, overwrite bool) ([]Dashboard, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var export DashboardExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, err
	}

	return m.Import(&export, owner, overwrite)
}

// ========== 持久化 ==========

func (m *Manager) load() error {
	if m.storePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return err
	}

	var dashboards []*Dashboard
	if err := json.Unmarshal(data, &dashboards); err != nil {
		return err
	}

	for _, d := range dashboards {
		m.dashboards[d.ID] = d
	}

	return nil
}

func (m *Manager) save() error {
	if m.storePath == "" {
		return nil
	}

	dashboards := make([]*Dashboard, 0, len(m.dashboards))
	for _, d := range m.dashboards {
		dashboards = append(dashboards, d)
	}

	data, err := json.MarshalIndent(dashboards, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(m.storePath), 0755); err != nil {
		return err
	}

	return os.WriteFile(m.storePath, data, 0644)
}

// ========== 辅助函数 ==========

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func sortByUpdatedAt(dashboards []Dashboard) {
	for i := 0; i < len(dashboards)-1; i++ {
		for j := i + 1; j < len(dashboards); j++ {
			if dashboards[i].UpdatedAt.Before(dashboards[j].UpdatedAt) {
				dashboards[i], dashboards[j] = dashboards[j], dashboards[i]
			}
		}
	}
}
