// Package dashboard 提供监控仪表板功能
// widget_layout.go - 用户布局存储（JSON序列化）
package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WidgetLayoutManager Widget布局管理器
type WidgetLayoutManager struct {
	mu       sync.RWMutex
	layouts  map[string]*UserWidgetLayout // userID -> layout
	dataDir  string
	saveMode SaveMode
}

// UserWidgetLayout 用户Widget布局配置
type UserWidgetLayout struct {
	UserID     string                      `json:"userId"`
	Dashboards map[string]*DashboardLayout `json:"dashboards"` // dashboardID -> layout
	CreatedAt  time.Time                   `json:"createdAt"`
	UpdatedAt  time.Time                   `json:"updatedAt"`
	Settings   LayoutSettings              `json:"settings"`
}

// DashboardLayout 单个Dashboard的布局配置
type DashboardLayout struct {
	DashboardID string              `json:"dashboardId"`
	Widgets     []WidgetLayoutEntry `json:"widgets"`
	GridConfig  GridLayoutConfig    `json:"gridConfig"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

// WidgetLayoutEntry Widget布局条目
type WidgetLayoutEntry struct {
	WidgetID   string         `json:"widgetId"`
	WidgetType WidgetType     `json:"widgetType"`
	Position   WidgetPosition `json:"position"`
	Size       WidgetSize     `json:"size"`
	Order      int            `json:"order"` // 排序顺序
	Enabled    bool           `json:"enabled"`
	Collapsed  bool           `json:"collapsed"` // 是否折叠
}

// GridLayoutConfig 网格布局配置
type GridLayoutConfig struct {
	Columns    int `json:"columns"`    // 列数
	Rows       int `json:"rows"`       // 行数（可选，0表示自动）
	Gap        int `json:"gap"`        // 间距(px)
	CellWidth  int `json:"cellWidth"`  // 单元格宽度(px)
	CellHeight int `json:"cellHeight"` // 单元格高度(px)
	MaxWidth   int `json:"maxWidth"`   // 最大宽度(px)
	Breakpoint int `json:"breakpoint"` // 响应式断点(px)
}

// LayoutSettings 全局布局设置
type LayoutSettings struct {
	AutoArrange    bool   `json:"autoArrange"`    // 自动排列
	SnapToGrid     bool   `json:"snapToGrid"`     // 吸附网格
	ShowGridLines  bool   `json:"showGridLines"`  // 显示网格线
	CompactMode    bool   `json:"compactMode"`    // 紧凑模式
	AnimationSpeed int    `json:"animationSpeed"` // 动画速度(ms)
	Theme          string `json:"theme"`          // 布局主题
}

// SaveMode 保存模式
type SaveMode string

const (
	// SaveModeImmediate 立即保存
	SaveModeImmediate SaveMode = "immediate"
	// SaveModeDelayed 延迟保存（批量保存）
	SaveModeDelayed SaveMode = "delayed"
	// SaveModeManual 手动保存
	SaveModeManual SaveMode = "manual"
)

// NewWidgetLayoutManager 创建布局管理器
func NewWidgetLayoutManager() *WidgetLayoutManager {
	return &WidgetLayoutManager{
		layouts:  make(map[string]*UserWidgetLayout),
		saveMode: SaveModeImmediate,
	}
}

// SetDataDir 设置数据目录
func (m *WidgetLayoutManager) SetDataDir(dir string) error {
	m.mu.Lock()
	m.dataDir = dir
	m.mu.Unlock()

	if dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("创建数据目录失败: %w", err)
		}
		// 加载已有布局
		return m.loadAllLayouts()
	}
	return nil
}

// GetLayout 获取用户布局
func (m *WidgetLayoutManager) GetLayout(userID string) (*UserWidgetLayout, error) {
	m.mu.RLock()
	layout, ok := m.layouts[userID]
	m.mu.RUnlock()

	if ok {
		return layout, nil
	}

	// 创建默认布局
	return m.createDefaultLayout(userID)
}

// createDefaultLayout 创建默认布局
func (m *WidgetLayoutManager) createDefaultLayout(userID string) (*UserWidgetLayout, error) {
	now := time.Now()
	layout := &UserWidgetLayout{
		UserID:     userID,
		Dashboards: make(map[string]*DashboardLayout),
		CreatedAt:  now,
		UpdatedAt:  now,
		Settings: LayoutSettings{
			AutoArrange:    true,
			SnapToGrid:     true,
			ShowGridLines:  true,
			CompactMode:    false,
			AnimationSpeed: 200,
			Theme:          "default",
		},
	}

	m.mu.Lock()
	m.layouts[userID] = layout
	m.mu.Unlock()

	if m.saveMode == SaveModeImmediate {
		_ = m.saveLayout(userID)
	}

	return layout, nil
}

// SaveLayout 保存用户布局
func (m *WidgetLayoutManager) SaveLayout(userID string, layout *UserWidgetLayout) error {
	m.mu.Lock()
	layout.UpdatedAt = time.Now()
	m.layouts[userID] = layout
	m.mu.Unlock()

	if m.saveMode == SaveModeImmediate {
		return m.saveLayout(userID)
	}
	return nil
}

// UpdateWidgetPosition 更新Widget位置
func (m *WidgetLayoutManager) UpdateWidgetPosition(userID, dashboardID, widgetID string, pos WidgetPosition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layout, ok := m.layouts[userID]
	if !ok {
		return ErrLayoutNotFound
	}

	dashLayout, ok := layout.Dashboards[dashboardID]
	if !ok {
		dashLayout = &DashboardLayout{
			DashboardID: dashboardID,
			Widgets:     make([]WidgetLayoutEntry, 0),
			GridConfig:  DefaultGridConfig(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		layout.Dashboards[dashboardID] = dashLayout
	}

	for i, w := range dashLayout.Widgets {
		if w.WidgetID == widgetID {
			dashLayout.Widgets[i].Position = pos
			dashLayout.UpdatedAt = time.Now()
			layout.UpdatedAt = time.Now()

			if m.saveMode == SaveModeImmediate {
				_ = m.saveLayout(userID)
			}
			return nil
		}
	}

	// Widget不存在，添加新条目
	dashLayout.Widgets = append(dashLayout.Widgets, WidgetLayoutEntry{
		WidgetID: widgetID,
		Position: pos,
		Order:    len(dashLayout.Widgets),
		Enabled:  true,
	})
	dashLayout.UpdatedAt = time.Now()
	layout.UpdatedAt = time.Now()

	if m.saveMode == SaveModeImmediate {
		_ = m.saveLayout(userID)
	}
	return nil
}

// ReorderWidgets 重排序Widgets
func (m *WidgetLayoutManager) ReorderWidgets(userID, dashboardID string, widgetOrder []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layout, ok := m.layouts[userID]
	if !ok {
		return ErrLayoutNotFound
	}

	dashLayout, ok := layout.Dashboards[dashboardID]
	if !ok {
		return ErrLayoutNotFound
	}

	// 创建新的Widgets列表
	newWidgets := make([]WidgetLayoutEntry, 0, len(widgetOrder))
	orderMap := make(map[string]int)
	for i, id := range widgetOrder {
		orderMap[id] = i
	}

	// 保留现有Widgets的配置，按新顺序排列
	existingWidgets := make(map[string]WidgetLayoutEntry)
	for _, w := range dashLayout.Widgets {
		existingWidgets[w.WidgetID] = w
	}

	for i, id := range widgetOrder {
		if w, ok := existingWidgets[id]; ok {
			w.Order = i
			newWidgets = append(newWidgets, w)
		}
	}

	// 保留未在排序列表中的Widgets
	for _, w := range dashLayout.Widgets {
		if _, ok := orderMap[w.WidgetID]; !ok {
			w.Order = len(newWidgets)
			newWidgets = append(newWidgets, w)
		}
	}

	dashLayout.Widgets = newWidgets
	dashLayout.UpdatedAt = time.Now()
	layout.UpdatedAt = time.Now()

	if m.saveMode == SaveModeImmediate {
		_ = m.saveLayout(userID)
	}
	return nil
}

// GetDashboardLayout 获取Dashboard布局
func (m *WidgetLayoutManager) GetDashboardLayout(userID, dashboardID string) (*DashboardLayout, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	layout, ok := m.layouts[userID]
	if !ok {
		return nil, ErrLayoutNotFound
	}

	dashLayout, ok := layout.Dashboards[dashboardID]
	if !ok {
		return nil, ErrLayoutNotFound
	}

	return dashLayout, nil
}

// SaveDashboardLayout 保存Dashboard布局
func (m *WidgetLayoutManager) SaveDashboardLayout(userID string, dashLayout *DashboardLayout) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layout, ok := m.layouts[userID]
	if !ok {
		layout = &UserWidgetLayout{
			UserID:     userID,
			Dashboards: make(map[string]*DashboardLayout),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		m.layouts[userID] = layout
	}

	dashLayout.UpdatedAt = time.Now()
	layout.Dashboards[dashLayout.DashboardID] = dashLayout
	layout.UpdatedAt = time.Now()

	if m.saveMode == SaveModeImmediate {
		_ = m.saveLayout(userID)
	}
	return nil
}

// ExportLayout 导出布局为JSON
func (m *WidgetLayoutManager) ExportLayout(userID string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	layout, ok := m.layouts[userID]
	if !ok {
		return nil, ErrLayoutNotFound
	}

	return json.MarshalIndent(layout, "", "  ")
}

// ImportLayout 导入布局
func (m *WidgetLayoutManager) ImportLayout(userID string, data []byte) error {
	var layout UserWidgetLayout
	if err := json.Unmarshal(data, &layout); err != nil {
		return fmt.Errorf("解析布局失败: %w", err)
	}

	layout.UserID = userID
	layout.UpdatedAt = time.Now()

	m.mu.Lock()
	m.layouts[userID] = &layout
	m.mu.Unlock()

	if m.saveMode == SaveModeImmediate {
		return m.saveLayout(userID)
	}
	return nil
}

// DeleteLayout 删除用户布局
func (m *WidgetLayoutManager) DeleteLayout(userID string) error {
	m.mu.Lock()
	delete(m.layouts, userID)
	m.mu.Unlock()

	if m.dataDir != "" {
		path := filepath.Join(m.dataDir, "layout_"+userID+".json")
		return os.Remove(path)
	}
	return nil
}

// saveLayout 保存布局到文件
func (m *WidgetLayoutManager) saveLayout(userID string) error {
	if m.dataDir == "" {
		return nil
	}

	m.mu.RLock()
	layout, ok := m.layouts[userID]
	m.mu.RUnlock()

	if !ok {
		return ErrLayoutNotFound
	}

	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化布局失败: %w", err)
	}

	path := filepath.Join(m.dataDir, "layout_"+userID+".json")
	return os.WriteFile(path, data, 0640)
}

// loadAllLayouts 加载所有布局
func (m *WidgetLayoutManager) loadAllLayouts() error {
	if m.dataDir == "" {
		return nil
	}

	files, err := filepath.Glob(filepath.Join(m.dataDir, "layout_*.json"))
	if err != nil {
		return fmt.Errorf("扫描布局文件失败: %w", err)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var layout UserWidgetLayout
		if err := json.Unmarshal(data, &layout); err != nil {
			continue
		}

		m.mu.Lock()
		m.layouts[layout.UserID] = &layout
		m.mu.Unlock()
	}

	return nil
}

// DefaultGridConfig 默认网格配置
func DefaultGridConfig() GridLayoutConfig {
	return GridLayoutConfig{
		Columns:    4,
		Rows:       0,
		Gap:        16,
		CellWidth:  300,
		CellHeight: 200,
		MaxWidth:   1200,
		Breakpoint: 768,
	}
}

// CompactGridConfig 紧凑网格配置
func CompactGridConfig() GridLayoutConfig {
	return GridLayoutConfig{
		Columns:    6,
		Rows:       0,
		Gap:        8,
		CellWidth:  200,
		CellHeight: 150,
		MaxWidth:   1200,
		Breakpoint: 768,
	}
}

// WidgetSizeToDimensions 将WidgetSize转换为具体尺寸
func WidgetSizeToDimensions(size WidgetSize, config GridLayoutConfig) (width, height int) {
	switch size {
	case WidgetSizeSmall:
		return config.CellWidth, config.CellHeight
	case WidgetSizeMedium:
		return config.CellWidth * 2, config.CellHeight
	case WidgetSizeLarge:
		return config.CellWidth * 2, config.CellHeight * 2
	default:
		return config.CellWidth, config.CellHeight
	}
}

// CalculatePosition 计算Widget在网格中的实际位置
func CalculatePosition(entry WidgetLayoutEntry, config GridLayoutConfig) (x, y, width, height int) {
	width, height = WidgetSizeToDimensions(entry.Size, config)
	x = entry.Position.X * (config.CellWidth + config.Gap)
	y = entry.Position.Y * (config.CellHeight + config.Gap)
	return x, y, width, height
}

// ValidatePosition 验证位置是否有效
func ValidatePosition(pos WidgetPosition, config GridLayoutConfig) bool {
	return pos.X >= 0 && pos.X < config.Columns && pos.Y >= 0
}
