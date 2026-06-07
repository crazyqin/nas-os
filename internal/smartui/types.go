// Package smartui 提供智能自适应用户界面功能
// 包括个性化仪表板、主题引擎、布局推荐、组件管理
package smartui

import (
	"errors"
	"sync"
	"time"
)

// ========== 主题系统 ==========

// ThemeMode 主题模式
type ThemeMode string

const (
	ThemeLight  ThemeMode = "light"  // 浅色模式
	ThemeDark   ThemeMode = "dark"   // 深色模式
	ThemeAuto   ThemeMode = "auto"   // 跟随系统
	ThemeCustom ThemeMode = "custom" // 自定义主题
)

// ColorScheme 配色方案
type ColorScheme struct {
	Primary       string `json:"primary"`        // 主色调
	Secondary     string `json:"secondary"`      // 次要色
	Accent        string `json:"accent"`         // 强调色
	Background    string `json:"background"`     // 背景色
	Surface       string `json:"surface"`        // 表面色
	TextPrimary   string `json:"text_primary"`   // 主文本色
	TextSecondary string `json:"text_secondary"` // 次文本色
	Error         string `json:"error"`          // 错误色
	Warning       string `json:"warning"`        // 警告色
	Success       string `json:"success"`        // 成功色
}

// Theme 主题配置
type Theme struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Mode         ThemeMode   `json:"mode"`
	Colors       ColorScheme `json:"colors"`
	FontFamily   string      `json:"font_family"`
	FontSize     int         `json:"font_size"`     // 基准字号（px）
	BorderRadius int         `json:"border_radius"` // 圆角大小
	Spacing      int         `json:"spacing"`       // 间距基准
	Animations   bool        `json:"animations"`    // 启用动画
	BuiltIn      bool        `json:"built_in"`      // 是否内置主题
	CreatedAt    time.Time   `json:"created_at"`
}

// ========== 布局系统 ==========

// LayoutType 布局类型
type LayoutType string

const (
	LayoutGrid    LayoutType = "grid"    // 网格布局
	LayoutFree    LayoutType = "free"    // 自由布局
	LayoutSidebar LayoutType = "sidebar" // 侧边栏布局
	LayoutTabs    LayoutType = "tabs"    // 标签页布局
)

// WidgetType 组件类型
type WidgetType string

const (
	WidgetStorageOverview  WidgetType = "storage_overview"  // 存储概览
	WidgetSystemHealth     WidgetType = "system_health"     // 系统健康
	WidgetRecentFiles      WidgetType = "recent_files"      // 最近文件
	WidgetQuickActions     WidgetType = "quick_actions"     // 快捷操作
	WidgetPerformanceChart WidgetType = "performance_chart" // 性能图表
	WidgetAlerts           WidgetType = "alerts"            // 告警通知
	WidgetAppLauncher      WidgetType = "app_launcher"      // 应用启动器
	WidgetDiskUsage        WidgetType = "disk_usage"        // 磁盘用量
	WidgetNetworkStatus    WidgetType = "network_status"    // 网络状态
	WidgetBackupStatus     WidgetType = "backup_status"     // 备份状态
	WidgetWeatherWidget    WidgetType = "weather"           // 天气组件
	WidgetTodoList         WidgetType = "todo_list"         // 待办列表
	WidgetBookmarks        WidgetType = "bookmarks"         // 书签
	WidgetRSSFeed          WidgetType = "rss_feed"          // RSS 订阅
	WidgetMediaCenter      WidgetType = "media_center"      // 媒体中心
	WidgetDockerContainers WidgetType = "docker_containers" // Docker 容器
	WidgetAIAssistant      WidgetType = "ai_assistant"      // AI 助手
)

// WidgetPosition 组件位置
type WidgetPosition struct {
	X      int `json:"x"`      // 网格 X 坐标
	Y      int `json:"y"`      // 网格 Y 坐标
	Width  int `json:"width"`  // 宽度（网格单位）
	Height int `json:"height"` // 高度（网格单位）
}

// Widget 组件实例
type Widget struct {
	ID       string                 `json:"id"`
	Type     WidgetType             `json:"type"`
	Title    string                 `json:"title"`
	Position WidgetPosition         `json:"position"`
	Config   map[string]interface{} `json:"config,omitempty"` // 组件特定配置
	Visible  bool                   `json:"visible"`
	Locked   bool                   `json:"locked"` // 锁定位置
}

// Dashboard 仪表板
type Dashboard struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Layout      LayoutType `json:"layout"`
	Columns     int        `json:"columns"` // 网格列数（默认 12）
	Widgets     []Widget   `json:"widgets"`
	IsDefault   bool       `json:"is_default"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ========== 用户偏好 ==========

// UIPreference 用户界面偏好
type UIPreference struct {
	UserID           string              `json:"user_id"`
	ThemeID          string              `json:"theme_id"`
	DashboardID      string              `json:"dashboard_id"`
	Language         string              `json:"language"`          // 语言
	SidebarCollapsed bool                `json:"sidebar_collapsed"` // 侧边栏是否折叠
	CompactMode      bool                `json:"compact_mode"`      // 紧凑模式
	FontSize         int                 `json:"font_size"`         // 个人字号覆盖
	Animations       *bool               `json:"animations"`        // 个人动画偏好
	Accessibility    AccessibilityConfig `json:"accessibility"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// AccessibilityConfig 无障碍配置
type AccessibilityConfig struct {
	HighContrast   bool   `json:"high_contrast"`    // 高对比度
	ReduceMotion   bool   `json:"reduce_motion"`    // 减少动画
	ScreenReader   bool   `json:"screen_reader"`    // 屏幕阅读器优化
	KeyboardNav    bool   `json:"keyboard_nav"`     // 键盘导航
	LargeText      bool   `json:"large_text"`       // 大字体
	ColorBlindMode string `json:"color_blind_mode"` // 色盲模式
}

// ========== 布局推荐 ==========

// LayoutRecommendation 布局推荐
type LayoutRecommendation struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Score       float64  `json:"score"` // 推荐分数 0-1
	Widgets     []Widget `json:"widgets"`
	Reason      string   `json:"reason"`   // 推荐理由
	ForRole     string   `json:"for_role"` // 适用角色
}

// ========== 智能UI引擎 ==========

// SmartUIEngine 智能UI引擎
type SmartUIEngine struct {
	mu              sync.RWMutex
	themes          map[string]*Theme
	dashboards      map[string]*Dashboard
	preferences     map[string]*UIPreference
	recommendations []LayoutRecommendation
	defaultTheme    string
	defaultDash     string
}

// EngineOption 引擎配置选项
type EngineOption func(*SmartUIEngine)

// WithDefaultTheme 设置默认主题
func WithDefaultTheme(themeID string) EngineOption {
	return func(e *SmartUIEngine) {
		e.defaultTheme = themeID
	}
}

// WithDefaultDashboard 设置默认仪表板
func WithDefaultDashboard(dashID string) EngineOption {
	return func(e *SmartUIEngine) {
		e.defaultDash = dashID
	}
}

// NewSmartUIEngine 创建智能UI引擎
func NewSmartUIEngine(opts ...EngineOption) *SmartUIEngine {
	e := &SmartUIEngine{
		themes:      make(map[string]*Theme),
		dashboards:  make(map[string]*Dashboard),
		preferences: make(map[string]*UIPreference),
	}
	for _, opt := range opts {
		opt(e)
	}
	e.initBuiltInThemes()
	e.initDefaultDashboard()
	return e
}

// ========== 主题管理 ==========

// GetTheme 获取主题
func (e *SmartUIEngine) GetTheme(themeID string) (*Theme, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	theme, ok := e.themes[themeID]
	if !ok {
		return nil, errors.New("theme not found")
	}
	return theme, nil
}

// ListThemes 列出所有主题
func (e *SmartUIEngine) ListThemes() []*Theme {
	e.mu.RLock()
	defer e.mu.RUnlock()
	themes := make([]*Theme, 0, len(e.themes))
	for _, t := range e.themes {
		themes = append(themes, t)
	}
	return themes
}

// CreateTheme 创建自定义主题
func (e *SmartUIEngine) CreateTheme(theme *Theme) error {
	if theme.ID == "" {
		return errors.New("theme ID cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	theme.BuiltIn = false
	theme.CreatedAt = time.Now()
	e.themes[theme.ID] = theme
	return nil
}

// ========== 仪表板管理 ==========

// GetDashboard 获取仪表板
func (e *SmartUIEngine) GetDashboard(dashID string) (*Dashboard, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	dash, ok := e.dashboards[dashID]
	if !ok {
		return nil, errors.New("dashboard not found")
	}
	return dash, nil
}

// CreateDashboard 创建仪表板
func (e *SmartUIEngine) CreateDashboard(dash *Dashboard) error {
	if dash.ID == "" {
		return errors.New("dashboard ID cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	dash.CreatedAt = time.Now()
	dash.UpdatedAt = time.Now()
	e.dashboards[dash.ID] = dash
	return nil
}

// AddWidget 添加组件到仪表板
func (e *SmartUIEngine) AddWidget(dashID string, widget Widget) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	dash, ok := e.dashboards[dashID]
	if !ok {
		return errors.New("dashboard not found")
	}
	widget.Visible = true
	dash.Widgets = append(dash.Widgets, widget)
	dash.UpdatedAt = time.Now()
	return nil
}

// RemoveWidget 移除组件
func (e *SmartUIEngine) RemoveWidget(dashID, widgetID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	dash, ok := e.dashboards[dashID]
	if !ok {
		return errors.New("dashboard not found")
	}
	for i, w := range dash.Widgets {
		if w.ID == widgetID {
			dash.Widgets = append(dash.Widgets[:i], dash.Widgets[i+1:]...)
			dash.UpdatedAt = time.Now()
			return nil
		}
	}
	return errors.New("widget not found")
}

// ========== 用户偏好 ==========

// GetPreference 获取用户偏好
func (e *SmartUIEngine) GetPreference(userID string) *UIPreference {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if pref, ok := e.preferences[userID]; ok {
		return pref
	}
	return &UIPreference{
		UserID:      userID,
		ThemeID:     e.defaultTheme,
		DashboardID: e.defaultDash,
		Language:    "zh-CN",
		FontSize:    14,
		Animations:  boolPtr(true),
	}
}

// UpdatePreference 更新用户偏好
func (e *SmartUIEngine) UpdatePreference(pref *UIPreference) error {
	if pref.UserID == "" {
		return errors.New("user ID cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	pref.UpdatedAt = time.Now()
	e.preferences[pref.UserID] = pref
	return nil
}

// ========== 智能推荐 ==========

// RecommendLayout 根据用户行为推荐布局
func (e *SmartUIEngine) RecommendLayout(userID string, role string) []LayoutRecommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var recommendations []LayoutRecommendation

	switch role {
	case "admin":
		recommendations = append(recommendations, LayoutRecommendation{
			ID:          "admin-overview",
			Name:        "管理员全景",
			Description: "系统监控 + 性能图表 + 告警 + 容器管理",
			Score:       0.95,
			ForRole:     "admin",
			Reason:      "管理员需要全面的系统视图",
			Widgets: []Widget{
				{Type: WidgetSystemHealth, Title: "系统健康"},
				{Type: WidgetPerformanceChart, Title: "性能监控"},
				{Type: WidgetAlerts, Title: "告警中心"},
				{Type: WidgetDockerContainers, Title: "容器管理"},
				{Type: WidgetDiskUsage, Title: "磁盘用量"},
				{Type: WidgetNetworkStatus, Title: "网络状态"},
			},
		})
	case "user":
		recommendations = append(recommendations, LayoutRecommendation{
			ID:          "user-daily",
			Name:        "日常使用",
			Description: "文件管理 + 最近文件 + 快捷操作 + 媒体",
			Score:       0.90,
			ForRole:     "user",
			Reason:      "普通用户关注文件访问和媒体播放",
			Widgets: []Widget{
				{Type: WidgetStorageOverview, Title: "存储概览"},
				{Type: WidgetRecentFiles, Title: "最近文件"},
				{Type: WidgetQuickActions, Title: "快捷操作"},
				{Type: WidgetMediaCenter, Title: "媒体中心"},
				{Type: WidgetBookmarks, Title: "书签"},
				{Type: WidgetAppLauncher, Title: "应用"},
			},
		})
	default:
		recommendations = append(recommendations, LayoutRecommendation{
			ID:          "default",
			Name:        "默认布局",
			Description: "均衡的默认仪表板",
			Score:       0.80,
			ForRole:     "all",
			Reason:      "适合大多数用户的默认配置",
			Widgets: []Widget{
				{Type: WidgetStorageOverview, Title: "存储概览"},
				{Type: WidgetSystemHealth, Title: "系统状态"},
				{Type: WidgetQuickActions, Title: "快捷操作"},
				{Type: WidgetAlerts, Title: "通知"},
			},
		})
	}

	return recommendations
}

// ========== 内部初始化 ==========

func (e *SmartUIEngine) initBuiltInThemes() {
	light := &Theme{
		ID:   "builtin-light",
		Name: "浅色主题",
		Mode: ThemeLight,
		Colors: ColorScheme{
			Primary:       "#1976D2",
			Secondary:     "#424242",
			Accent:        "#FF4081",
			Background:    "#FAFAFA",
			Surface:       "#FFFFFF",
			TextPrimary:   "#212121",
			TextSecondary: "#757575",
			Error:         "#D32F2F",
			Warning:       "#FFA000",
			Success:       "#388E3C",
		},
		FontFamily:   "Inter, system-ui, sans-serif",
		FontSize:     14,
		BorderRadius: 8,
		Spacing:      8,
		Animations:   true,
		BuiltIn:      true,
		CreatedAt:    time.Now(),
	}

	dark := &Theme{
		ID:   "builtin-dark",
		Name: "深色主题",
		Mode: ThemeDark,
		Colors: ColorScheme{
			Primary:       "#90CAF9",
			Secondary:     "#BDBDBD",
			Accent:        "#FF80AB",
			Background:    "#121212",
			Surface:       "#1E1E1E",
			TextPrimary:   "#FFFFFF",
			TextSecondary: "#B0B0B0",
			Error:         "#EF5350",
			Warning:       "#FFB74D",
			Success:       "#66BB6A",
		},
		FontFamily:   "Inter, system-ui, sans-serif",
		FontSize:     14,
		BorderRadius: 8,
		Spacing:      8,
		Animations:   true,
		BuiltIn:      true,
		CreatedAt:    time.Now(),
	}

	e.themes[light.ID] = light
	e.themes[dark.ID] = dark
	e.defaultTheme = light.ID
}

func (e *SmartUIEngine) initDefaultDashboard() {
	defaultDash := &Dashboard{
		ID:          "default",
		Name:        "默认仪表板",
		Description: "系统默认仪表板",
		Layout:      LayoutGrid,
		Columns:     12,
		IsDefault:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Widgets: []Widget{
			{ID: "w1", Type: WidgetStorageOverview, Title: "存储概览", Position: WidgetPosition{X: 0, Y: 0, Width: 6, Height: 4}, Visible: true},
			{ID: "w2", Type: WidgetSystemHealth, Title: "系统状态", Position: WidgetPosition{X: 6, Y: 0, Width: 6, Height: 4}, Visible: true},
			{ID: "w3", Type: WidgetQuickActions, Title: "快捷操作", Position: WidgetPosition{X: 0, Y: 4, Width: 4, Height: 3}, Visible: true},
			{ID: "w4", Type: WidgetRecentFiles, Title: "最近文件", Position: WidgetPosition{X: 4, Y: 4, Width: 8, Height: 3}, Visible: true},
		},
	}
	e.dashboards[defaultDash.ID] = defaultDash
	e.defaultDash = defaultDash.ID
}

func boolPtr(b bool) *bool {
	return &b
}
