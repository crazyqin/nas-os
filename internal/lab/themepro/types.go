// Package themepro 提供专业主题引擎功能，支持自定义颜色方案、字体、布局和动态主题切换。
package themepro

import "time"

// ThemeMode 主题模式.
type ThemeMode string

const (
	ModeLight  ThemeMode = "light"
	ModeDark   ThemeMode = "dark"
	ModeSystem ThemeMode = "system"
	ModeCustom ThemeMode = "custom"
)

// ColorScheme 颜色方案.
type ColorScheme struct {
	Primary    string `json:"primary" binding:"required"`
	Secondary  string `json:"secondary"`
	Accent     string `json:"accent"`
	Background string `json:"background"`
	Surface    string `json:"surface"`
	Text       string `json:"text"`
	TextMuted  string `json:"text_muted"`
	Success    string `json:"success"`
	Warning    string `json:"warning"`
	Error      string `json:"error"`
	Info       string `json:"info"`
}

// FontConfig 字体配置.
type FontConfig struct {
	Family       string  `json:"family" binding:"required"`
	SizeBase     float64 `json:"size_base"`
	SizeSmall    float64 `json:"size_small"`
	SizeLarge    float64 `json:"size_large"`
	SizeXLarge   float64 `json:"size_xlarge"`
	WeightNormal int     `json:"weight_normal"`
	WeightBold   int     `json:"weight_bold"`
	LineHeight   float64 `json:"line_height"`
	LetterSpace  float64 `json:"letter_spacing"`
}

// LayoutPreset 布局预设.
type LayoutPreset string

const (
	LayoutCompact  LayoutPreset = "compact"
	LayoutDefault  LayoutPreset = "default"
	LayoutSpacious LayoutPreset = "spacious"
	LayoutCustom   LayoutPreset = "custom"
)

// SidebarPosition 侧边栏位置.
type SidebarPosition string

const (
	SidebarLeft   SidebarPosition = "left"
	SidebarRight  SidebarPosition = "right"
	SidebarHidden SidebarPosition = "hidden"
)

// BorderRadius 圆角配置.
type BorderRadius struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
	Full   string `json:"full"`
}

// LayoutConfig 布局配置.
type LayoutConfig struct {
	Preset          LayoutPreset    `json:"preset"`
	SidebarPosition SidebarPosition `json:"sidebar_position"`
	SidebarWidth    string          `json:"sidebar_width"`
	HeaderHeight    string          `json:"header_height"`
	ContentMaxWidth string          `json:"content_max_width"`
	SpacingUnit     string          `json:"spacing_unit"`
	BorderRadius    BorderRadius    `json:"border_radius"`
	CardShadow      string          `json:"card_shadow"`
	TransitionSpeed string          `json:"transition_speed"`
}

// Theme 主题.
type Theme struct {
	ID          string       `json:"id"`
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description,omitempty"`
	Mode        ThemeMode    `json:"mode"`
	Colors      ColorScheme  `json:"colors" binding:"required"`
	Fonts       FontConfig   `json:"fonts" binding:"required"`
	Layout      LayoutConfig `json:"layout"`
	IsDefault   bool         `json:"is_default"`
	IsBuiltin   bool         `json:"is_builtin"`
	Author      string       `json:"author,omitempty"`
	Version     string       `json:"version,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ThemePack 主题包（可导入导出）.
type ThemePack struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description,omitempty"`
	Version     string    `json:"version"`
	Author      string    `json:"author,omitempty"`
	Themes      []Theme   `json:"themes" binding:"required,min=1"`
	ExportedAt  time.Time `json:"exported_at"`
	Format      string    `json:"format"`
}

// CreateThemeRequest 创建主题请求.
type CreateThemeRequest struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description,omitempty"`
	Mode        ThemeMode    `json:"mode"`
	Colors      ColorScheme  `json:"colors" binding:"required"`
	Fonts       FontConfig   `json:"fonts" binding:"required"`
	Layout      LayoutConfig `json:"layout,omitempty"`
	Author      string       `json:"author,omitempty"`
	Version     string       `json:"version,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
}

// UpdateThemeRequest 更新主题请求.
type UpdateThemeRequest struct {
	Name        string        `json:"name,omitempty"`
	Description string        `json:"description,omitempty"`
	Mode        ThemeMode     `json:"mode,omitempty"`
	Colors      *ColorScheme  `json:"colors,omitempty"`
	Fonts       *FontConfig   `json:"fonts,omitempty"`
	Layout      *LayoutConfig `json:"layout,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
}

// ApplyThemeRequest 应用主题请求.
type ApplyThemeRequest struct {
	ThemeID string `json:"theme_id" binding:"required"`
}

// ThemePreview 主题预览.
type ThemePreview struct {
	Theme       Theme  `json:"theme"`
	PreviewURL  string `json:"preview_url,omitempty"`
	PreviewData string `json:"preview_data,omitempty"`
}

// DefaultColorScheme 默认颜色方案.
func DefaultColorScheme() ColorScheme {
	return ColorScheme{
		Primary:    "#1976D2",
		Secondary:  "#424242",
		Accent:     "#FF4081",
		Background: "#FFFFFF",
		Surface:    "#F5F5F5",
		Text:       "#212121",
		TextMuted:  "#757575",
		Success:    "#4CAF50",
		Warning:    "#FFC107",
		Error:      "#F44336",
		Info:       "#2196F3",
	}
}

// DarkColorScheme 暗色颜色方案.
func DarkColorScheme() ColorScheme {
	return ColorScheme{
		Primary:    "#90CAF9",
		Secondary:  "#B0BEC5",
		Accent:     "#FF80AB",
		Background: "#121212",
		Surface:    "#1E1E1E",
		Text:       "#E0E0E0",
		TextMuted:  "#9E9E9E",
		Success:    "#81C784",
		Warning:    "#FFD54F",
		Error:      "#E57373",
		Info:       "#64B5F6",
	}
}

// DefaultFontConfig 默认字体配置.
func DefaultFontConfig() FontConfig {
	return FontConfig{
		Family:       "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
		SizeBase:     16,
		SizeSmall:    14,
		SizeLarge:    18,
		SizeXLarge:   24,
		WeightNormal: 400,
		WeightBold:   700,
		LineHeight:   1.5,
		LetterSpace:  0,
	}
}

// DefaultLayoutConfig 默认布局配置.
func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		Preset:          LayoutDefault,
		SidebarPosition: SidebarLeft,
		SidebarWidth:    "240px",
		HeaderHeight:    "64px",
		ContentMaxWidth: "1200px",
		SpacingUnit:     "8px",
		BorderRadius: BorderRadius{
			Small:  "4px",
			Medium: "8px",
			Large:  "16px",
			Full:   "9999px",
		},
		CardShadow:      "0 2px 8px rgba(0,0,0,0.1)",
		TransitionSpeed: "0.2s",
	}
}
