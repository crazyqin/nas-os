// Package themepro 提供主题管理器
package themepro

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 主题管理器
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	themes      map[string]*Theme
	activeTheme string
	stopChan    chan struct{}
	running     bool
}

// NewManager 创建主题管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:   logger,
		themes:   make(map[string]*Theme),
		stopChan: make(chan struct{}),
	}

	// 初始化内置主题
	m.initBuiltinThemes()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("theme-%d", time.Now().UnixNano())
}

// initBuiltinThemes 初始化内置主题
func (m *Manager) initBuiltinThemes() {
	// 亮色主题
	lightTheme := &Theme{
		ID:          "builtin-light",
		Name:        "亮色主题",
		Description: "默认亮色主题，适合日间使用",
		Mode:        ModeLight,
		Colors:      DefaultColorScheme(),
		Fonts:       DefaultFontConfig(),
		Layout:      DefaultLayoutConfig(),
		IsDefault:   true,
		IsBuiltin:   true,
		Author:      "nas-os",
		Version:     "1.0.0",
		Tags:        []string{"light", "default"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.themes[lightTheme.ID] = lightTheme

	// 暗色主题
	darkTheme := &Theme{
		ID:          "builtin-dark",
		Name:        "暗色主题",
		Description: "护眼暗色主题，适合夜间使用",
		Mode:        ModeDark,
		Colors:      DarkColorScheme(),
		Fonts:       DefaultFontConfig(),
		Layout:      DefaultLayoutConfig(),
		IsDefault:   false,
		IsBuiltin:   true,
		Author:      "nas-os",
		Version:     "1.0.0",
		Tags:        []string{"dark", "night"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.themes[darkTheme.ID] = darkTheme

	// 极简主题
	minimalTheme := &Theme{
		ID:          "builtin-minimal",
		Name:        "极简主题",
		Description: "简洁清爽的极简风格",
		Mode:        ModeLight,
		Colors: ColorScheme{
			Primary:    "#000000",
			Secondary:  "#666666",
			Accent:     "#000000",
			Background: "#FFFFFF",
			Surface:    "#FAFAFA",
			Text:       "#111111",
			TextMuted:  "#888888",
			Success:    "#22C55E",
			Warning:    "#EAB308",
			Error:      "#EF4444",
			Info:       "#3B82F6",
		},
		Fonts: FontConfig{
			Family:       "'SF Pro Display', -apple-system, BlinkMacSystemFont, sans-serif",
			SizeBase:     15,
			SizeSmall:    13,
			SizeLarge:    17,
			SizeXLarge:   22,
			WeightNormal: 400,
			WeightBold:   600,
			LineHeight:   1.6,
			LetterSpace:  0.01,
		},
		Layout: LayoutConfig{
			Preset:          LayoutSpacious,
			SidebarPosition: SidebarLeft,
			SidebarWidth:    "280px",
			HeaderHeight:    "72px",
			ContentMaxWidth: "1080px",
			SpacingUnit:     "8px",
			BorderRadius: BorderRadius{
				Small:  "2px",
				Medium: "4px",
				Large:  "8px",
				Full:   "9999px",
			},
			CardShadow:      "0 1px 3px rgba(0,0,0,0.08)",
			TransitionSpeed: "0.15s",
		},
		IsDefault: false,
		IsBuiltin: true,
		Author:    "nas-os",
		Version:   "1.0.0",
		Tags:      []string{"minimal", "clean"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.themes[minimalTheme.ID] = minimalTheme

	// 设置默认活跃主题
	m.activeTheme = lightTheme.ID

	m.logger.Info("builtin themes initialized", zap.Int("count", len(m.themes)))
}

// ListThemes 列出所有主题
func (m *Manager) ListThemes() []*Theme {
	m.mu.RLock()
	defer m.mu.RUnlock()

	themes := make([]*Theme, 0, len(m.themes))
	for _, t := range m.themes {
		themes = append(themes, t)
	}
	return themes
}

// GetTheme 获取主题
func (m *Manager) GetTheme(id string) (*Theme, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	theme, ok := m.themes[id]
	if !ok {
		return nil, fmt.Errorf("theme not found: %s", id)
	}
	return theme, nil
}

// GetActiveTheme 获取当前活跃主题
func (m *Manager) GetActiveTheme() (*Theme, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	theme, ok := m.themes[m.activeTheme]
	if !ok {
		return nil, fmt.Errorf("active theme not found")
	}
	return theme, nil
}

// ApplyTheme 应用主题
func (m *Manager) ApplyTheme(themeID string) (*Theme, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	theme, ok := m.themes[themeID]
	if !ok {
		return nil, fmt.Errorf("theme not found: %s", themeID)
	}

	m.activeTheme = themeID
	m.logger.Info("theme applied", zap.String("theme_id", themeID), zap.String("theme_name", theme.Name))

	return theme, nil
}

// CreateCustomTheme 创建自定义主题
func (m *Manager) CreateCustomTheme(req *CreateThemeRequest) (*Theme, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	theme := &Theme{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Mode:        req.Mode,
		Colors:      req.Colors,
		Fonts:       req.Fonts,
		Layout:      req.Layout,
		IsDefault:   false,
		IsBuiltin:   false,
		Author:      req.Author,
		Version:     req.Version,
		Tags:        req.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置默认值
	if theme.Mode == "" {
		theme.Mode = ModeCustom
	}
	if theme.Version == "" {
		theme.Version = "1.0.0"
	}

	m.themes[theme.ID] = theme
	m.logger.Info("custom theme created", zap.String("theme_id", theme.ID), zap.String("name", theme.Name))

	return theme, nil
}

// UpdateTheme 更新主题
func (m *Manager) UpdateTheme(id string, req *UpdateThemeRequest) (*Theme, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	theme, ok := m.themes[id]
	if !ok {
		return nil, fmt.Errorf("theme not found: %s", id)
	}

	if theme.IsBuiltin {
		return nil, fmt.Errorf("cannot modify builtin theme")
	}

	if req.Name != "" {
		theme.Name = req.Name
	}
	if req.Description != "" {
		theme.Description = req.Description
	}
	if req.Mode != "" {
		theme.Mode = req.Mode
	}
	if req.Colors != nil {
		theme.Colors = *req.Colors
	}
	if req.Fonts != nil {
		theme.Fonts = *req.Fonts
	}
	if req.Layout != nil {
		theme.Layout = *req.Layout
	}
	if req.Tags != nil {
		theme.Tags = req.Tags
	}
	theme.UpdatedAt = time.Now()

	m.logger.Info("theme updated", zap.String("theme_id", id))
	return theme, nil
}

// DeleteTheme 删除主题
func (m *Manager) DeleteTheme(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	theme, ok := m.themes[id]
	if !ok {
		return fmt.Errorf("theme not found: %s", id)
	}

	if theme.IsBuiltin {
		return fmt.Errorf("cannot delete builtin theme")
	}

	if m.activeTheme == id {
		return fmt.Errorf("cannot delete active theme, switch to another theme first")
	}

	delete(m.themes, id)
	m.logger.Info("theme deleted", zap.String("theme_id", id))

	return nil
}

// ExportTheme 导出主题包
func (m *Manager) ExportTheme(themeIDs []string) (*ThemePack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	themes := make([]Theme, 0, len(themeIDs))
	for _, id := range themeIDs {
		theme, ok := m.themes[id]
		if !ok {
			return nil, fmt.Errorf("theme not found: %s", id)
		}
		themes = append(themes, *theme)
	}

	if len(themes) == 0 {
		return nil, fmt.Errorf("no themes to export")
	}

	pack := &ThemePack{
		Name:       fmt.Sprintf("theme-pack-%d", time.Now().Unix()),
		Version:    "1.0.0",
		Themes:     themes,
		ExportedAt: time.Now(),
		Format:     "nas-os-theme-pack-v1",
	}

	return pack, nil
}

// ImportTheme 导入主题包
func (m *Manager) ImportTheme(data []byte) ([]*Theme, error) {
	var pack ThemePack
	if err := json.Unmarshal(data, &pack); err != nil {
		return nil, fmt.Errorf("invalid theme pack format: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	imported := make([]*Theme, 0, len(pack.Themes))
	for _, t := range pack.Themes {
		// 生成新 ID 避免冲突
		theme := t
		theme.ID = generateID()
		theme.IsBuiltin = false
		theme.IsDefault = false
		theme.CreatedAt = time.Now()
		theme.UpdatedAt = time.Now()

		m.themes[theme.ID] = &theme
		imported = append(imported, &theme)
	}

	m.logger.Info("themes imported", zap.Int("count", len(imported)))
	return imported, nil
}

// GetDefaultThemes 获取默认主题列表
func (m *Manager) GetDefaultThemes() []*Theme {
	m.mu.RLock()
	defer m.mu.RUnlock()

	defaults := make([]*Theme, 0)
	for _, t := range m.themes {
		if t.IsDefault {
			defaults = append(defaults, t)
		}
	}
	return defaults
}

// GetBuiltinThemes 获取内置主题列表
func (m *Manager) GetBuiltinThemes() []*Theme {
	m.mu.RLock()
	defer m.mu.RUnlock()

	builtins := make([]*Theme, 0)
	for _, t := range m.themes {
		if t.IsBuiltin {
			builtins = append(builtins, t)
		}
	}
	return builtins
}

// GetCustomThemes 获取自定义主题列表
func (m *Manager) GetCustomThemes() []*Theme {
	m.mu.RLock()
	defer m.mu.RUnlock()

	customs := make([]*Theme, 0)
	for _, t := range m.themes {
		if !t.IsBuiltin {
			customs = append(customs, t)
		}
	}
	return customs
}

// DuplicateTheme 复制主题
func (m *Manager) DuplicateTheme(id string, newName string) (*Theme, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	original, ok := m.themes[id]
	if !ok {
		return nil, fmt.Errorf("theme not found: %s", id)
	}

	theme := &Theme{
		ID:          generateID(),
		Name:        newName,
		Description: original.Description,
		Mode:        original.Mode,
		Colors:      original.Colors,
		Fonts:       original.Fonts,
		Layout:      original.Layout,
		IsDefault:   false,
		IsBuiltin:   false,
		Author:      original.Author,
		Version:     original.Version,
		Tags:        original.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.themes[theme.ID] = theme
	m.logger.Info("theme duplicated", zap.String("original_id", id), zap.String("new_id", theme.ID))

	return theme, nil
}
