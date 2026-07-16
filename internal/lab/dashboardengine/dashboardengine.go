// Package dashboardengine 提供仪表盘定制引擎
// 可拖拽组件、数据可视化、主题系统、多用户偏好、实时数据源
// 对标群晖 DSM Dashboard + TrueNAS 仪表盘
package dashboardengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	Version            = "1.0.0"
	MaxDashboards      = 50
	MaxWidgets         = 100
	MaxLayouts         = 10
	MaxDataSources     = 50
	WidgetMinWidth     = 1
	WidgetMinHeight    = 1
	WidgetMaxWidth     = 12
	WidgetMaxHeight    = 8
	DefaultRefreshRate = 30 * time.Second
)

// ========== 类型定义 ==========

// WidgetType 组件类型.
type WidgetType string

const (
	WidgetChart       WidgetType = "chart"
	WidgetGauge       WidgetType = "gauge"
	WidgetTable       WidgetType = "table"
	WidgetStat        WidgetType = "stat"
	WidgetList        WidgetType = "list"
	WidgetMap         WidgetType = "map"
	WidgetGraph       WidgetType = "graph"
	WidgetAlert       WidgetType = "alert"
	WidgetLog         WidgetType = "log"
	WidgetCustom      WidgetType = "custom"
	WidgetCPU         WidgetType = "cpu"
	WidgetMemory      WidgetType = "memory"
	WidgetDisk        WidgetType = "disk"
	WidgetNetwork     WidgetType = "network"
	WidgetService     WidgetType = "service"
	WidgetContainer   WidgetType = "container"
	WidgetTemperature WidgetType = "temperature"
)

// ChartType 图表类型.
type ChartType string

const (
	ChartLine    ChartType = "line"
	ChartBar     ChartType = "bar"
	ChartPie     ChartType = "pie"
	ChartArea    ChartType = "area"
	ChartScatter ChartType = "scatter"
	ChartHeat    ChartType = "heatmap"
	ChartRadar   ChartType = "radar"
)

// Theme 主题.
type Theme string

const (
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
	ThemeAuto   Theme = "auto"
	ThemeCustom Theme = "custom"
)

// WidgetPosition 组件位置.
type WidgetPosition struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// WidgetConfig 组件配置.
type WidgetConfig struct {
	ID          string                 `json:"id"`
	Type        WidgetType             `json:"type"`
	Title       string                 `json:"title"`
	Position    WidgetPosition         `json:"position"`
	DataSource  string                 `json:"data_source"`
	ChartType   ChartType              `json:"chart_type,omitempty"`
	RefreshRate time.Duration          `json:"refresh_rate"`
	Options     map[string]interface{} `json:"options,omitempty"`
	Visible     bool                   `json:"visible"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// DataSource 数据源.
type DataSource struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Endpoint string                 `json:"endpoint"`
	Interval time.Duration          `json:"interval"`
	Config   map[string]interface{} `json:"config,omitempty"`
	LastSync time.Time              `json:"last_sync"`
	Enabled  bool                   `json:"enabled"`
}

// Dashboard 仪表盘.
type Dashboard struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Theme       Theme           `json:"theme"`
	Widgets     []*WidgetConfig `json:"widgets"`
	Layout      string          `json:"layout"`
	IsDefault   bool            `json:"is_default"`
	Owner       string          `json:"owner"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	RefreshRate time.Duration   `json:"refresh_rate"`
}

// DashboardLayout 布局模板.
type DashboardLayout struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Columns   int              `json:"columns"`
	Rows      int              `json:"rows"`
	Positions []WidgetPosition `json:"positions"`
}

// WidgetData 组件数据.
type WidgetData struct {
	WidgetID  string                 `json:"widget_id"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Error     string                 `json:"error,omitempty"`
}

// DashboardStats 仪表盘统计.
type DashboardStats struct {
	TotalDashboards  int `json:"total_dashboards"`
	TotalWidgets     int `json:"total_widgets"`
	TotalDataSources int `json:"total_data_sources"`
	ActiveWidgets    int `json:"active_widgets"`
}

// ========== 核心引擎 ==========

// Engine 仪表盘引擎.
type Engine struct {
	mu            sync.RWMutex
	dashboards    map[string]*Dashboard
	dataSources   map[string]*DataSource
	layouts       map[string]*DashboardLayout
	widgetData    map[string]*WidgetData
	dataDir       string
	ctx           context.Context
	cancel        context.CancelFunc
	dashCounter   int64
	widgetCounter int64
	dsCounter     int64
}

// NewEngine 创建引擎.
func NewEngine(dataDir string) (*Engine, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{
		dashboards:  make(map[string]*Dashboard),
		dataSources: make(map[string]*DataSource),
		layouts:     make(map[string]*DashboardLayout),
		widgetData:  make(map[string]*WidgetData),
		dataDir:     dataDir,
		ctx:         ctx,
		cancel:      cancel,
	}

	e.initDefaultLayouts()
	e.initDefaultDataSources()
	e.createDefaultDashboard()
	return e, nil
}

// initDefaultLayouts 初始化默认布局.
func (e *Engine) initDefaultLayouts() {
	e.layouts["grid-2x2"] = &DashboardLayout{
		ID: "grid-2x2", Name: "2x2网格", Columns: 2, Rows: 2,
		Positions: []WidgetPosition{
			{0, 0, 6, 4}, {6, 0, 6, 4}, {0, 4, 6, 4}, {6, 4, 6, 4},
		},
	}
	e.layouts["grid-3x2"] = &DashboardLayout{
		ID: "grid-3x2", Name: "3x2网格", Columns: 3, Rows: 2,
		Positions: []WidgetPosition{
			{0, 0, 4, 4}, {4, 0, 4, 4}, {8, 0, 4, 4},
			{0, 4, 4, 4}, {4, 4, 4, 4}, {8, 4, 4, 4},
		},
	}
	e.layouts["main-sidebar"] = &DashboardLayout{
		ID: "main-sidebar", Name: "主侧栏", Columns: 2, Rows: 3,
		Positions: []WidgetPosition{
			{0, 0, 8, 6}, {8, 0, 4, 3}, {8, 3, 4, 3},
		},
	}
}

// initDefaultDataSources 初始化默认数据源.
func (e *Engine) initDefaultDataSources() {
	defaults := []struct {
		name, dtype, endpoint string
	}{
		{"系统CPU", "prometheus", "/api/v1/system/cpu"},
		{"系统内存", "prometheus", "/api/v1/system/memory"},
		{"磁盘使用", "prometheus", "/api/v1/system/disk"},
		{"网络流量", "prometheus", "/api/v1/system/network"},
		{"容器状态", "docker", "/api/v1/containers"},
		{"服务状态", "systemd", "/api/v1/services"},
		{"温度传感器", "prometheus", "/api/v1/system/temperature"},
	}

	for _, d := range defaults {
		e.dsCounter++
		e.dataSources[fmt.Sprintf("ds_%d", e.dsCounter)] = &DataSource{
			ID:       fmt.Sprintf("ds_%d", e.dsCounter),
			Name:     d.name,
			Type:     d.dtype,
			Endpoint: d.endpoint,
			Interval: DefaultRefreshRate,
			Enabled:  true,
		}
	}
}

// createDefaultDashboard 创建默认仪表盘.
func (e *Engine) createDefaultDashboard() {
	e.dashCounter++
	dash := &Dashboard{
		ID:          fmt.Sprintf("dash_%d", e.dashCounter),
		Name:        "系统概览",
		Description: "NAS系统核心监控指标",
		Theme:       ThemeDark,
		Layout:      "grid-2x2",
		IsDefault:   true,
		Owner:       "admin",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RefreshRate: DefaultRefreshRate,
	}

	// 添加默认组件
	widgets := []struct {
		wtype WidgetType
		title string
		ds    string
		pos   WidgetPosition
	}{
		{WidgetCPU, "CPU使用率", "ds_1", WidgetPosition{0, 0, 6, 4}},
		{WidgetMemory, "内存使用", "ds_2", WidgetPosition{6, 0, 6, 4}},
		{WidgetDisk, "磁盘状态", "ds_3", WidgetPosition{0, 4, 6, 4}},
		{WidgetNetwork, "网络流量", "ds_4", WidgetPosition{6, 4, 6, 4}},
	}

	for _, w := range widgets {
		e.widgetCounter++
		dash.Widgets = append(dash.Widgets, &WidgetConfig{
			ID:         fmt.Sprintf("widget_%d", e.widgetCounter),
			Type:       w.wtype,
			Title:      w.title,
			Position:   w.pos,
			DataSource: w.ds,
			Visible:    true,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})
	}

	e.dashboards[dash.ID] = dash
}

// ========== 仪表盘管理 ==========

// CreateDashboard 创建仪表盘.
func (e *Engine) CreateDashboard(name, desc string, theme Theme, layout string) (*Dashboard, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.dashboards) >= MaxDashboards {
		return nil, errors.New("仪表盘数量已达上限")
	}

	e.dashCounter++
	dash := &Dashboard{
		ID:          fmt.Sprintf("dash_%d", e.dashCounter),
		Name:        name,
		Description: desc,
		Theme:       theme,
		Layout:      layout,
		Widgets:     make([]*WidgetConfig, 0),
		Owner:       "admin",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RefreshRate: DefaultRefreshRate,
	}
	e.dashboards[dash.ID] = dash
	return dash, nil
}

// GetDashboard 获取仪表盘.
func (e *Engine) GetDashboard(id string) (*Dashboard, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dash, ok := e.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("仪表盘不存在: %s", id)
	}
	return dash, nil
}

// DeleteDashboard 删除仪表盘.
func (e *Engine) DeleteDashboard(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dash, ok := e.dashboards[id]
	if !ok {
		return fmt.Errorf("仪表盘不存在: %s", id)
	}
	if dash.IsDefault {
		return errors.New("不能删除默认仪表盘")
	}
	delete(e.dashboards, id)
	return nil
}

// ListDashboards 列出所有仪表盘.
func (e *Engine) ListDashboards() []*Dashboard {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Dashboard, 0, len(e.dashboards))
	for _, d := range e.dashboards {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// UpdateDashboardTheme 更新主题.
func (e *Engine) UpdateDashboardTheme(id string, theme Theme) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dash, ok := e.dashboards[id]
	if !ok {
		return fmt.Errorf("仪表盘不存在: %s", id)
	}
	dash.Theme = theme
	dash.UpdatedAt = time.Now()
	return nil
}

// ========== 组件管理 ==========

// AddWidget 添加组件.
func (e *Engine) AddWidget(dashID string, wtype WidgetType, title, dsID string, pos WidgetPosition) (*WidgetConfig, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	dash, ok := e.dashboards[dashID]
	if !ok {
		return nil, fmt.Errorf("仪表盘不存在: %s", dashID)
	}

	if len(dash.Widgets) >= MaxWidgets {
		return nil, errors.New("组件数量已达上限")
	}

	if pos.Width < WidgetMinWidth || pos.Width > WidgetMaxWidth {
		return nil, errors.New("组件宽度无效")
	}

	e.widgetCounter++
	widget := &WidgetConfig{
		ID:         fmt.Sprintf("widget_%d", e.widgetCounter),
		Type:       wtype,
		Title:      title,
		Position:   pos,
		DataSource: dsID,
		Visible:    true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	dash.Widgets = append(dash.Widgets, widget)
	dash.UpdatedAt = time.Now()
	return widget, nil
}

// RemoveWidget 移除组件.
func (e *Engine) RemoveWidget(dashID, widgetID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dash, ok := e.dashboards[dashID]
	if !ok {
		return fmt.Errorf("仪表盘不存在: %s", dashID)
	}

	for i, w := range dash.Widgets {
		if w.ID == widgetID {
			dash.Widgets = append(dash.Widgets[:i], dash.Widgets[i+1:]...)
			dash.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("组件不存在: %s", widgetID)
}

// UpdateWidgetPosition 更新组件位置.
func (e *Engine) UpdateWidgetPosition(dashID, widgetID string, pos WidgetPosition) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dash, ok := e.dashboards[dashID]
	if !ok {
		return fmt.Errorf("仪表盘不存在: %s", dashID)
	}

	for _, w := range dash.Widgets {
		if w.ID == widgetID {
			w.Position = pos
			w.UpdatedAt = time.Now()
			dash.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("组件不存在: %s", widgetID)
}

// ========== 数据源 ==========

// AddDataSource 添加数据源.
func (e *Engine) AddDataSource(name, dtype, endpoint string, interval time.Duration) (*DataSource, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.dataSources) >= MaxDataSources {
		return nil, errors.New("数据源数量已达上限")
	}

	e.dsCounter++
	ds := &DataSource{
		ID:       fmt.Sprintf("ds_%d", e.dsCounter),
		Name:     name,
		Type:     dtype,
		Endpoint: endpoint,
		Interval: interval,
		Enabled:  true,
	}
	e.dataSources[ds.ID] = ds
	return ds, nil
}

// GetDataSources 获取所有数据源.
func (e *Engine) GetDataSources() []*DataSource {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*DataSource, 0, len(e.dataSources))
	for _, ds := range e.dataSources {
		result = append(result, ds)
	}
	return result
}

// ========== 持久化 ==========

// SaveDashboard 保存仪表盘到文件.
func (e *Engine) SaveDashboard(id string) error {
	e.mu.RLock()
	dash, ok := e.dashboards[id]
	if !ok {
		e.mu.RUnlock()
		return fmt.Errorf("仪表盘不存在: %s", id)
	}
	data, _ := json.MarshalIndent(dash, "", "  ")
	e.mu.RUnlock()

	path := filepath.Join(e.dataDir, fmt.Sprintf("%s.json", id))
	return os.WriteFile(path, data, 0644)
}

// LoadDashboard 从文件加载仪表盘.
func (e *Engine) LoadDashboard(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var dash Dashboard
	if err := json.Unmarshal(data, &dash); err != nil {
		return err
	}

	e.mu.Lock()
	e.dashboards[dash.ID] = &dash
	e.mu.Unlock()
	return nil
}

// ========== 统计 ==========

// GetStats 获取统计信息.
func (e *Engine) GetStats() *DashboardStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &DashboardStats{
		TotalDashboards:  len(e.dashboards),
		TotalDataSources: len(e.dataSources),
	}

	for _, d := range e.dashboards {
		stats.TotalWidgets += len(d.Widgets)
		for _, w := range d.Widgets {
			if w.Visible {
				stats.ActiveWidgets++
			}
		}
	}
	return stats
}

// GetLayouts 获取所有布局模板.
func (e *Engine) GetLayouts() []*DashboardLayout {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*DashboardLayout, 0, len(e.layouts))
	for _, l := range e.layouts {
		result = append(result, l)
	}
	return result
}

// Close 关闭引擎.
func (e *Engine) Close() error {
	e.cancel()
	return nil
}
