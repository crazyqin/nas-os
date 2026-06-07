// Package unifiedportal 提供统一门户核心管理逻辑
package unifiedportal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PortalManager 统一门户管理器
type PortalManager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	dashboards  map[string]*PortalDashboard
	widgets     map[string]*DashboardWidget
	themes      map[string]*PortalTheme
	activeTheme string // 当前激活主题ID
	dataSources map[string]*DataSource
	stopChan    chan struct{}
	running     bool
}

// NewPortalManager 创建门户管理器
func NewPortalManager(logger *zap.Logger) *PortalManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	pm := &PortalManager{
		logger:      logger,
		dashboards:  make(map[string]*PortalDashboard),
		widgets:     make(map[string]*DashboardWidget),
		themes:      make(map[string]*PortalTheme),
		dataSources: make(map[string]*DataSource),
		stopChan:    make(chan struct{}),
	}

	// 初始化默认主题
	pm.initDefaultThemes()
	// 初始化默认仪表盘模板
	pm.initDefaultTemplates()

	return pm
}

// generateID 生成唯一ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// initDefaultThemes 初始化默认主题
func (pm *PortalManager) initDefaultThemes() {
	defaultThemes := []*PortalTheme{
		{
			ID:           "theme-light",
			Name:         "浅色主题",
			Mode:         ThemeLight,
			PrimaryColor: "#1890ff",
			AccentColor:  "#52c41a",
			BGColor:      "#ffffff",
			TextColor:    "#333333",
			FontSize:     "14px",
			FontFamily:   "system-ui, -apple-system, sans-serif",
			BorderRadius: "8px",
			IsDark:       false,
			IsDefault:    true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "theme-dark",
			Name:         "暗黑主题",
			Mode:         ThemeDark,
			PrimaryColor: "#177ddc",
			AccentColor:  "#49aa19",
			BGColor:      "#141414",
			TextColor:    "#ffffffd9",
			FontSize:     "14px",
			FontFamily:   "system-ui, -apple-system, sans-serif",
			BorderRadius: "8px",
			IsDark:       true,
			IsDefault:    false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "theme-ocean",
			Name:         "海洋主题",
			Mode:         ThemeLight,
			PrimaryColor: "#096dd9",
			AccentColor:  "#13c2c2",
			BGColor:      "#f0f5ff",
			TextColor:    "#1d39c4",
			FontSize:     "14px",
			FontFamily:   "system-ui, -apple-system, sans-serif",
			BorderRadius: "12px",
			IsDark:       false,
			IsDefault:    false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	for _, t := range defaultThemes {
		pm.themes[t.ID] = t
		if t.IsDefault {
			pm.activeTheme = t.ID
		}
	}
}

// initDefaultTemplates 初始化默认仪表盘模板
func (pm *PortalManager) initDefaultTemplates() {
	// 管理员模板
	adminTemplate := &PortalDashboard{
		ID:          "template-admin",
		Name:        "管理员仪表盘",
		Description: "系统管理员专用仪表盘，包含系统概览、存储、网络和容器状态",
		Layout:      LayoutGrid,
		IsDefault:   false,
		IsTemplate:  true,
		Tags:        []string{"admin", "template"},
		Widgets: []*DashboardWidget{
			{
				ID:         generateID(),
				Type:       WidgetSystemOverview,
				Title:      "系统概览",
				Position:   WidgetPosition{X: 0, Y: 0},
				Size:       WidgetSize{Width: 6, Height: 4},
				IsVisible:  true,
				RefreshSec: 30,
			},
			{
				ID:         generateID(),
				Type:       WidgetStorageUsage,
				Title:      "存储使用",
				Position:   WidgetPosition{X: 6, Y: 0},
				Size:       WidgetSize{Width: 6, Height: 4},
				IsVisible:  true,
				RefreshSec: 60,
			},
			{
				ID:         generateID(),
				Type:       WidgetNetworkTraffic,
				Title:      "网络流量",
				Position:   WidgetPosition{X: 0, Y: 4},
				Size:       WidgetSize{Width: 6, Height: 4},
				IsVisible:  true,
				RefreshSec: 15,
			},
			{
				ID:         generateID(),
				Type:       WidgetContainerStatus,
				Title:      "容器状态",
				Position:   WidgetPosition{X: 6, Y: 4},
				Size:       WidgetSize{Width: 6, Height: 4},
				IsVisible:  true,
				RefreshSec: 30,
			},
			{
				ID:         generateID(),
				Type:       WidgetAlerts,
				Title:      "告警信息",
				Position:   WidgetPosition{X: 0, Y: 8},
				Size:       WidgetSize{Width: 12, Height: 3},
				IsVisible:  true,
				RefreshSec: 10,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 开发者模板
	devTemplate := &PortalDashboard{
		ID:          "template-developer",
		Name:        "开发者仪表盘",
		Description: "开发者专用仪表盘，关注容器和自定义图表",
		Layout:      LayoutResponsive,
		IsDefault:   false,
		IsTemplate:  true,
		Tags:        []string{"developer", "template"},
		Widgets: []*DashboardWidget{
			{
				ID:         generateID(),
				Type:       WidgetContainerStatus,
				Title:      "容器状态",
				Position:   WidgetPosition{X: 0, Y: 0},
				Size:       WidgetSize{Width: 8, Height: 5},
				IsVisible:  true,
				RefreshSec: 15,
			},
			{
				ID:         generateID(),
				Type:       WidgetSystemOverview,
				Title:      "系统资源",
				Position:   WidgetPosition{X: 8, Y: 0},
				Size:       WidgetSize{Width: 4, Height: 5},
				IsVisible:  true,
				RefreshSec: 30,
			},
			{
				ID:         generateID(),
				Type:       WidgetCustomChart,
				Title:      "自定义图表",
				Position:   WidgetPosition{X: 0, Y: 5},
				Size:       WidgetSize{Width: 12, Height: 4},
				IsVisible:  true,
				RefreshSec: 60,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 运维模板
	opsTemplate := &PortalDashboard{
		ID:          "template-ops",
		Name:        "运维仪表盘",
		Description: "运维专用仪表盘，关注系统健康和告警",
		Layout:      LayoutGrid,
		IsDefault:   false,
		IsTemplate:  true,
		Tags:        []string{"ops", "template"},
		Widgets: []*DashboardWidget{
			{
				ID:         generateID(),
				Type:       WidgetAlerts,
				Title:      "告警中心",
				Position:   WidgetPosition{X: 0, Y: 0},
				Size:       WidgetSize{Width: 12, Height: 3},
				IsVisible:  true,
				RefreshSec: 5,
			},
			{
				ID:         generateID(),
				Type:       WidgetSystemOverview,
				Title:      "系统概览",
				Position:   WidgetPosition{X: 0, Y: 3},
				Size:       WidgetSize{Width: 4, Height: 4},
				IsVisible:  true,
				RefreshSec: 30,
			},
			{
				ID:         generateID(),
				Type:       WidgetStorageUsage,
				Title:      "存储状态",
				Position:   WidgetPosition{X: 4, Y: 3},
				Size:       WidgetSize{Width: 4, Height: 4},
				IsVisible:  true,
				RefreshSec: 60,
			},
			{
				ID:         generateID(),
				Type:       WidgetNetworkTraffic,
				Title:      "网络流量",
				Position:   WidgetPosition{X: 8, Y: 3},
				Size:       WidgetSize{Width: 4, Height: 4},
				IsVisible:  true,
				RefreshSec: 15,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	pm.dashboards[adminTemplate.ID] = adminTemplate
	pm.dashboards[devTemplate.ID] = devTemplate
	pm.dashboards[opsTemplate.ID] = opsTemplate
}

// CreateDashboard 创建仪表盘
func (pm *PortalManager) CreateDashboard(req *DashboardRequest, owner string) (*PortalDashboard, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if req.Layout == "" {
		req.Layout = LayoutGrid
	}
	if !IsValidLayout(req.Layout) {
		return nil, fmt.Errorf("invalid layout type: %s", req.Layout)
	}

	dashboard := &PortalDashboard{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Layout:      req.Layout,
		Widgets:     make([]*DashboardWidget, 0),
		IsDefault:   false,
		IsTemplate:  false,
		Tags:        req.Tags,
		Owner:       owner,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pm.dashboards[dashboard.ID] = dashboard
	pm.logger.Info("dashboard created", zap.String("id", dashboard.ID), zap.String("name", dashboard.Name))
	return dashboard, nil
}

// GetDashboard 获取仪表盘
func (pm *PortalManager) GetDashboard(id string) (*PortalDashboard, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	dashboard, ok := pm.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard not found: %s", id)
	}
	return dashboard, nil
}

// ListDashboards 列出所有仪表盘
func (pm *PortalManager) ListDashboards(owner string) []*PortalDashboard {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	dashboards := make([]*PortalDashboard, 0)
	for _, d := range pm.dashboards {
		if owner == "" || d.Owner == owner || d.Owner == "" {
			dashboards = append(dashboards, d)
		}
	}
	return dashboards
}

// UpdateDashboard 更新仪表盘
func (pm *PortalManager) UpdateDashboard(id string, req *DashboardRequest) (*PortalDashboard, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	dashboard, ok := pm.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard not found: %s", id)
	}

	if req.Layout != "" && !IsValidLayout(req.Layout) {
		return nil, fmt.Errorf("invalid layout type: %s", req.Layout)
	}

	dashboard.Name = req.Name
	if req.Description != "" {
		dashboard.Description = req.Description
	}
	if req.Layout != "" {
		dashboard.Layout = req.Layout
	}
	if req.Tags != nil {
		dashboard.Tags = req.Tags
	}
	dashboard.UpdatedAt = time.Now()

	pm.logger.Info("dashboard updated", zap.String("id", dashboard.ID))
	return dashboard, nil
}

// DeleteDashboard 删除仪表盘
func (pm *PortalManager) DeleteDashboard(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	dashboard, ok := pm.dashboards[id]
	if !ok {
		return fmt.Errorf("dashboard not found: %s", id)
	}

	if dashboard.IsTemplate {
		return fmt.Errorf("cannot delete template dashboard")
	}

	// 删除关联的widgets
	for _, widget := range dashboard.Widgets {
		delete(pm.widgets, widget.ID)
	}

	delete(pm.dashboards, id)
	pm.logger.Info("dashboard deleted", zap.String("id", id))
	return nil
}

// AddWidget 添加Widget到仪表盘
func (pm *PortalManager) AddWidget(dashboardID string, req *WidgetRequest) (*DashboardWidget, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	dashboard, ok := pm.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("dashboard not found: %s", dashboardID)
	}

	if !IsValidWidgetType(req.Type) {
		return nil, fmt.Errorf("invalid widget type: %s", req.Type)
	}

	// 检查位置是否冲突
	if err := pm.checkWidgetCollision(dashboard, req.Position, req.Size, ""); err != nil {
		return nil, err
	}

	widget := &DashboardWidget{
		ID:          generateID(),
		DashboardID: dashboardID,
		Type:        req.Type,
		Title:       req.Title,
		Position:    req.Position,
		Size:        req.Size,
		Config:      req.Config,
		DataSource:  req.DataSource,
		RefreshSec:  req.RefreshSec,
		IsVisible:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pm.widgets[widget.ID] = widget
	dashboard.Widgets = append(dashboard.Widgets, widget)
	dashboard.UpdatedAt = time.Now()

	pm.logger.Info("widget added", zap.String("widget_id", widget.ID), zap.String("dashboard_id", dashboardID))
	return widget, nil
}

// checkWidgetCollision 检查Widget位置冲突
func (pm *PortalManager) checkWidgetCollision(dashboard *PortalDashboard, pos WidgetPosition, size WidgetSize, excludeID string) error {
	for _, w := range dashboard.Widgets {
		if w.ID == excludeID {
			continue
		}
		if rectanglesOverlap(pos, size, w.Position, w.Size) {
			return fmt.Errorf("widget position conflicts with existing widget %s", w.ID)
		}
	}
	return nil
}

// rectanglesOverlap 检查两个矩形是否重叠
func rectanglesOverlap(pos1 WidgetPosition, size1 WidgetSize, pos2 WidgetPosition, size2 WidgetSize) bool {
	return pos1.X < pos2.X+size2.Width &&
		pos1.X+size1.Width > pos2.X &&
		pos1.Y < pos2.Y+size2.Height &&
		pos1.Y+size1.Height > pos2.Y
}

// UpdateWidget 更新Widget
func (pm *PortalManager) UpdateWidget(widgetID string, req *WidgetRequest) (*DashboardWidget, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	widget, ok := pm.widgets[widgetID]
	if !ok {
		return nil, fmt.Errorf("widget not found: %s", widgetID)
	}

	if !IsValidWidgetType(req.Type) {
		return nil, fmt.Errorf("invalid widget type: %s", req.Type)
	}

	// 检查位置冲突
	dashboard, _ := pm.dashboards[widget.DashboardID]
	if dashboard != nil {
		if err := pm.checkWidgetCollision(dashboard, req.Position, req.Size, widgetID); err != nil {
			return nil, err
		}
	}

	widget.Type = req.Type
	if req.Title != "" {
		widget.Title = req.Title
	}
	widget.Position = req.Position
	widget.Size = req.Size
	widget.Config = req.Config
	widget.DataSource = req.DataSource
	widget.RefreshSec = req.RefreshSec
	widget.UpdatedAt = time.Now()

	pm.logger.Info("widget updated", zap.String("widget_id", widgetID))
	return widget, nil
}

// MoveWidget 移动/调整Widget大小
func (pm *PortalManager) MoveWidget(widgetID string, req *WidgetMoveRequest) (*DashboardWidget, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	widget, ok := pm.widgets[widgetID]
	if !ok {
		return nil, fmt.Errorf("widget not found: %s", widgetID)
	}

	// 检查位置冲突
	dashboard, _ := pm.dashboards[widget.DashboardID]
	if dashboard != nil {
		if err := pm.checkWidgetCollision(dashboard, req.Position, req.Size, widgetID); err != nil {
			return nil, err
		}
	}

	widget.Position = req.Position
	widget.Size = req.Size
	widget.UpdatedAt = time.Now()

	pm.logger.Info("widget moved", zap.String("widget_id", widgetID))
	return widget, nil
}

// DeleteWidget 删除Widget
func (pm *PortalManager) DeleteWidget(widgetID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	widget, ok := pm.widgets[widgetID]
	if !ok {
		return fmt.Errorf("widget not found: %s", widgetID)
	}

	// 从仪表盘中移除
	if dashboard, ok := pm.dashboards[widget.DashboardID]; ok {
		for i, w := range dashboard.Widgets {
			if w.ID == widgetID {
				dashboard.Widgets = append(dashboard.Widgets[:i], dashboard.Widgets[i+1:]...)
				dashboard.UpdatedAt = time.Now()
				break
			}
		}
	}

	delete(pm.widgets, widgetID)
	pm.logger.Info("widget deleted", zap.String("widget_id", widgetID))
	return nil
}

// GetThemes 获取所有主题
func (pm *PortalManager) GetThemes() []*PortalTheme {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	themes := make([]*PortalTheme, 0, len(pm.themes))
	for _, t := range pm.themes {
		themes = append(themes, t)
	}
	return themes
}

// GetActiveTheme 获取当前激活主题
func (pm *PortalManager) GetActiveTheme() (*PortalTheme, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	theme, ok := pm.themes[pm.activeTheme]
	if !ok {
		return nil, fmt.Errorf("active theme not found")
	}
	return theme, nil
}

// SwitchTheme 切换主题
func (pm *PortalManager) SwitchTheme(themeID string) (*PortalTheme, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	theme, ok := pm.themes[themeID]
	if !ok {
		return nil, fmt.Errorf("theme not found: %s", themeID)
	}

	pm.activeTheme = themeID
	pm.logger.Info("theme switched", zap.String("theme_id", themeID), zap.String("theme_name", theme.Name))
	return theme, nil
}

// ExportDashboard 导出仪表盘
func (pm *PortalManager) ExportDashboard(dashboardID string) (*DashboardExport, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	dashboard, ok := pm.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("dashboard not found: %s", dashboardID)
	}

	// 复制widgets
	widgets := make([]*DashboardWidget, len(dashboard.Widgets))
	copy(widgets, dashboard.Widgets)

	export := &DashboardExport{
		Dashboard:  dashboard,
		Widgets:    widgets,
		Version:    "1.0",
		ExportedAt: time.Now(),
	}

	return export, nil
}

// ImportDashboard 导入仪表盘
func (pm *PortalManager) ImportDashboard(data []byte, owner string) (*PortalDashboard, error) {
	var export DashboardExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("invalid import data: %w", err)
	}

	if export.Dashboard == nil {
		return nil, fmt.Errorf("missing dashboard in import data")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 创建新仪表盘
	newID := generateID()
	dashboard := &PortalDashboard{
		ID:          newID,
		Name:        export.Dashboard.Name + " (导入)",
		Description: export.Dashboard.Description,
		Layout:      export.Dashboard.Layout,
		IsDefault:   false,
		IsTemplate:  false,
		Tags:        export.Dashboard.Tags,
		Owner:       owner,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Widgets:     make([]*DashboardWidget, 0),
	}

	// 导入widgets
	for _, w := range export.Widgets {
		widgetID := generateID()
		widget := &DashboardWidget{
			ID:          widgetID,
			DashboardID: newID,
			Type:        w.Type,
			Title:       w.Title,
			Position:    w.Position,
			Size:        w.Size,
			Config:      w.Config,
			DataSource:  w.DataSource,
			RefreshSec:  w.RefreshSec,
			IsVisible:   w.IsVisible,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		pm.widgets[widgetID] = widget
		dashboard.Widgets = append(dashboard.Widgets, widget)
	}

	pm.dashboards[newID] = dashboard
	pm.logger.Info("dashboard imported", zap.String("id", newID), zap.String("name", dashboard.Name))
	return dashboard, nil
}

// CreateDashboardFromTemplate 从模板创建仪表盘
func (pm *PortalManager) CreateDashboardFromTemplate(templateID string, name string, owner string) (*PortalDashboard, error) {
	pm.mu.RLock()
	template, ok := pm.dashboards[templateID]
	pm.mu.RUnlock()

	if !ok || !template.IsTemplate {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	newID := generateID()
	dashboard := &PortalDashboard{
		ID:          newID,
		Name:        name,
		Description: template.Description,
		Layout:      template.Layout,
		IsDefault:   false,
		IsTemplate:  false,
		TemplateID:  templateID,
		Tags:        template.Tags,
		Owner:       owner,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Widgets:     make([]*DashboardWidget, 0),
	}

	// 复制widgets
	for _, w := range template.Widgets {
		widgetID := generateID()
		widget := &DashboardWidget{
			ID:          widgetID,
			DashboardID: newID,
			Type:        w.Type,
			Title:       w.Title,
			Position:    w.Position,
			Size:        w.Size,
			Config:      w.Config,
			DataSource:  w.DataSource,
			RefreshSec:  w.RefreshSec,
			IsVisible:   w.IsVisible,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		pm.widgets[widgetID] = widget
		dashboard.Widgets = append(dashboard.Widgets, widget)
	}

	pm.dashboards[newID] = dashboard
	pm.logger.Info("dashboard created from template", zap.String("id", newID), zap.String("template", templateID))
	return dashboard, nil
}

// AggregateMetrics 聚合各模块指标（模拟数据源聚合）
func (pm *PortalManager) AggregateMetrics() *AggregatedMetrics {
	return &AggregatedMetrics{
		System: &SystemMetrics{
			CPUPercent:    45.2,
			MemoryPercent: 62.8,
			MemoryUsed:    10737418240, // 10GB
			MemoryTotal:   17179869184, // 16GB
			Uptime:        864000,      // 10天
			LoadAvg1:      1.5,
			LoadAvg5:      1.2,
			LoadAvg15:     0.8,
		},
		Storage: &StorageMetrics{
			TotalBytes:      1099511627776, // 1TB
			UsedBytes:       549755813888,  // 512GB
			FreeBytes:       549755813888,  // 512GB
			UsagePercent:    50.0,
			IOPSRead:        1000,
			IOPSWrite:       500,
			ThroughputRead:  104857600, // 100MB/s
			ThroughputWrite: 52428800,  // 50MB/s
		},
		Network: &NetworkMetrics{
			BytesIn:     1073741824, // 1GB
			BytesOut:    536870912,  // 512MB
			PacketsIn:   1000000,
			PacketsOut:  500000,
			Connections: 150,
			Bandwidth:   100.0,
		},
		Container: &ContainerMetrics{
			Total:     20,
			Running:   15,
			Stopped:   3,
			Paused:    2,
			Healthy:   14,
			Unhealthy: 1,
		},
		Alerts: &AlertMetrics{
			Critical: 1,
			Warning:  3,
			Info:     5,
			Total:    9,
			Recent: []*AlertItem{
				{
					ID:        "alert-001",
					Level:     "critical",
					Title:     "磁盘空间不足",
					Message:   "根分区使用率超过90%",
					Source:    "storage",
					Timestamp: time.Now().Add(-30 * time.Minute),
				},
				{
					ID:        "alert-002",
					Level:     "warning",
					Title:     "CPU温度过高",
					Message:   "CPU温度达到75°C",
					Source:    "system",
					Timestamp: time.Now().Add(-1 * time.Hour),
				},
			},
		},
		Timestamp: time.Now(),
	}
}

// SearchDashboards 搜索仪表盘
func (pm *PortalManager) SearchDashboards(keyword, tag string, onlyTemplates bool) []*PortalDashboard {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	results := make([]*PortalDashboard, 0)
	for _, d := range pm.dashboards {
		// 仅模板过滤
		if onlyTemplates && !d.IsTemplate {
			continue
		}

		// 标签过滤
		if tag != "" {
			found := false
			for _, t := range d.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 关键词搜索（名称和描述）
		if keyword != "" {
			if !containsIgnoreCase(d.Name, keyword) && !containsIgnoreCase(d.Description, keyword) {
				continue
			}
		}

		results = append(results, d)
	}
	return results
}

// containsIgnoreCase 不区分大小写检查子串
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// DuplicateDashboard 复制仪表盘
func (pm *PortalManager) DuplicateDashboard(dashboardID, newName, owner string) (*PortalDashboard, error) {
	pm.mu.RLock()
	original, ok := pm.dashboards[dashboardID]
	pm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("dashboard not found: %s", dashboardID)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	newID := generateID()
	if newName == "" {
		newName = original.Name + " (副本)"
	}

	dashboard := &PortalDashboard{
		ID:          newID,
		Name:        newName,
		Description: original.Description,
		Layout:      original.Layout,
		IsDefault:   false,
		IsTemplate:  false,
		Tags:        original.Tags,
		Owner:       owner,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Widgets:     make([]*DashboardWidget, 0),
	}

	// 复制widgets
	for _, w := range original.Widgets {
		widgetID := generateID()
		widget := &DashboardWidget{
			ID:          widgetID,
			DashboardID: newID,
			Type:        w.Type,
			Title:       w.Title,
			Position:    w.Position,
			Size:        w.Size,
			Config:      w.Config,
			DataSource:  w.DataSource,
			RefreshSec:  w.RefreshSec,
			IsVisible:   w.IsVisible,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		pm.widgets[widgetID] = widget
		dashboard.Widgets = append(dashboard.Widgets, widget)
	}

	pm.dashboards[newID] = dashboard
	pm.logger.Info("dashboard duplicated", zap.String("id", newID), zap.String("source", dashboardID))
	return dashboard, nil
}

// ToggleWidgetVisibility 切换Widget可见性
func (pm *PortalManager) ToggleWidgetVisibility(widgetID string, isVisible bool) (*DashboardWidget, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	widget, ok := pm.widgets[widgetID]
	if !ok {
		return nil, fmt.Errorf("widget not found: %s", widgetID)
	}

	widget.IsVisible = isVisible
	widget.UpdatedAt = time.Now()
	pm.logger.Info("widget visibility toggled", zap.String("widget_id", widgetID), zap.Bool("is_visible", isVisible))
	return widget, nil
}

// HealthCheck 系统健康检查
func (pm *PortalManager) HealthCheck() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return map[string]interface{}{
		"status":            "healthy",
		"dashboards_count":  len(pm.dashboards),
		"widgets_count":     len(pm.widgets),
		"themes_count":      len(pm.themes),
		"active_theme":      pm.activeTheme,
		"datasources_count": len(pm.dataSources),
		"timestamp":         time.Now(),
	}
}

// GetDashboardStats 获取仪表盘统计信息
func (pm *PortalManager) GetDashboardStats() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	templateCount := 0
	userCount := 0
	totalWidgets := 0
	owners := make(map[string]bool)

	for _, d := range pm.dashboards {
		if d.IsTemplate {
			templateCount++
		} else {
			userCount++
			owners[d.Owner] = true
		}
		totalWidgets += len(d.Widgets)
	}

	return map[string]interface{}{
		"total_dashboards":  len(pm.dashboards),
		"template_count":    templateCount,
		"user_count":        userCount,
		"total_widgets":     totalWidgets,
		"unique_owners":     len(owners),
		"total_themes":      len(pm.themes),
		"total_datasources": len(pm.dataSources),
		"timestamp":         time.Now(),
	}
}

// RegisterDataSource 注册数据源
func (pm *PortalManager) RegisterDataSource(ds *DataSource) *DataSource {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if ds.ID == "" {
		ds.ID = generateID()
	}

	pm.dataSources[ds.ID] = ds
	pm.logger.Info("data source registered", zap.String("id", ds.ID), zap.String("name", ds.Name))
	return ds
}

// ListDataSources 列出数据源
func (pm *PortalManager) ListDataSources() []*DataSource {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	sources := make([]*DataSource, 0, len(pm.dataSources))
	for _, ds := range pm.dataSources {
		sources = append(sources, ds)
	}
	return sources
}

// DeleteDataSource 删除数据源
func (pm *PortalManager) DeleteDataSource(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.dataSources[id]; !ok {
		return fmt.Errorf("data source not found: %s", id)
	}

	delete(pm.dataSources, id)
	pm.logger.Info("data source deleted", zap.String("id", id))
	return nil
}
