// Package main 暗黑主题插件
//
// 提供暗黑主题皮肤，保护眼睛，适合夜间使用
//
// 注意：此文件为示例代码，实际构建插件时需要：
// 1. 创建独立的 go module
// 2. 导入 nas-os/internal/plugin 包
// 3. 使用 go build -buildmode=plugin 构建
package main

import (
	"fmt"
	"sync"
)

// Category 插件分类
type Category string

const (
	CategoryFileManager Category = "file-manager"
	CategoryTheme       Category = "theme"
	CategoryMedia       Category = "media"
	CategoryBackup      Category = "backup"
)

// PluginInfoStruct 插件元信息
type PluginInfoStruct struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Tags        []string `json:"tags"`
	Entrypoint  string   `json:"entrypoint"`
	MainFile    string   `json:"mainFile"`
	Icon        string   `json:"icon"`
	License     string   `json:"license"`
	Price       string   `json:"price"`
}

// Plugin 插件接口
type Plugin interface {
	Info() PluginInfoStruct
	Init(config map[string]interface{}) error
	Start() error
	Stop() error
	Destroy() error
}

// 插件信息
var PluginInfo = PluginInfoStruct{
	ID:          "com.nas-os.dark-theme",
	Name:        "暗黑主题",
	Version:     "1.2.0",
	Author:      "NAS-OS Team",
	Description: "暗黑主题皮肤，保护眼睛，适合夜间使用",
	Category:    CategoryTheme,
	Tags:        []string{"主题", "暗黑", "护眼", "夜间"},
	Entrypoint:  "New",
	MainFile:    "dark-theme.so",
	Icon:        "moon",
	License:     "MIT",
	Price:       "free",
}

// DarkTheme 主题插件实现
type DarkTheme struct {
	config  map[string]interface{}
	styles  *ThemeStyles
	mu      sync.RWMutex
	running bool
}

// ThemeStyles 主题样式
type ThemeStyles struct {
	Name       string           `json:"name"`
	Colors     ColorScheme      `json:"colors"`
	Typography TypographyConfig `json:"typography"`
	Spacing    SpacingConfig    `json:"spacing"`
	Shadows    ShadowConfig     `json:"shadows"`
}

// ColorScheme 颜色方案
type ColorScheme struct {
	// 背景色
	Background          string `json:"background"`
	BackgroundSecondary string `json:"backgroundSecondary"`
	BackgroundTertiary  string `json:"backgroundTertiary"`

	// 文字色
	TextPrimary   string `json:"textPrimary"`
	TextSecondary string `json:"textSecondary"`
	TextMuted     string `json:"textMuted"`

	// 强调色
	Primary      string `json:"primary"`
	PrimaryHover string `json:"primaryHover"`
	Secondary    string `json:"secondary"`
	Accent       string `json:"accent"`

	// 状态色
	Success string `json:"success"`
	Warning string `json:"warning"`
	Error   string `json:"error"`
	Info    string `json:"info"`

	// 边框
	Border      string `json:"border"`
	BorderLight string `json:"borderLight"`

	// 其他
	Overlay   string `json:"overlay"`
	Highlight string `json:"highlight"`
}

// TypographyConfig 字体配置
type TypographyConfig struct {
	FontFamily string    `json:"fontFamily"`
	FontSize   FontSizes `json:"fontSize"`
	LineHeight string    `json:"lineHeight"`
}

// FontSizes 字体大小
type FontSizes struct {
	XS   string `json:"xs"`
	SM   string `json:"sm"`
	Base string `json:"base"`
	LG   string `json:"lg"`
	XL   string `json:"xl"`
	XXL  string `json:"2xl"`
}

// SpacingConfig 间距配置
type SpacingConfig struct {
	XS string `json:"xs"`
	SM string `json:"sm"`
	MD string `json:"md"`
	LG string `json:"lg"`
	XL string `json:"xl"`
}

// ShadowConfig 阴影配置
type ShadowConfig struct {
	SM string `json:"sm"`
	MD string `json:"md"`
	LG string `json:"lg"`
	XL string `json:"xl"`
}

// New 创建插件实例
func New() Plugin {
	return &DarkTheme{
		config: make(map[string]interface{}),
	}
}

// Info 返回插件信息
func (t *DarkTheme) Info() PluginInfoStruct {
	return PluginInfo
}

// Init 初始化插件
func (t *DarkTheme) Init(config map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 合并配置
	for k, v := range config {
		t.config[k] = v
	}

	// 设置默认值
	if _, ok := t.config["accentColor"]; !ok {
		t.config["accentColor"] = "#3b82f6"
	}
	if _, ok := t.config["backgroundOpacity"]; !ok {
		t.config["backgroundOpacity"] = 1.0
	}

	// 初始化主题样式
	t.styles = t.createDarkTheme()

	return nil
}

// Start 启动插件
func (t *DarkTheme) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return nil
	}

	t.running = true
	return nil
}

// Stop 停止插件
func (t *DarkTheme) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.running = false
	return nil
}

// Destroy 销毁插件
func (t *DarkTheme) Destroy() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.config = make(map[string]interface{})
	t.styles = nil
	t.running = false

	return nil
}

// createDarkTheme 创建暗黑主题样式
func (t *DarkTheme) createDarkTheme() *ThemeStyles {
	accentColor := "#3b82f6"
	if c, ok := t.config["accentColor"].(string); ok {
		accentColor = c
	}

	return &ThemeStyles{
		Name: "dark",
		Colors: ColorScheme{
			// 背景
			Background:          "#0f172a",
			BackgroundSecondary: "#1e293b",
			BackgroundTertiary:  "#334155",

			// 文字
			TextPrimary:   "#f8fafc",
			TextSecondary: "#cbd5e1",
			TextMuted:     "#64748b",

			// 强调色
			Primary:      accentColor,
			PrimaryHover: lightenColor(accentColor, 10),
			Secondary:    "#64748b",
			Accent:       accentColor,

			// 状态色
			Success: "#22c55e",
			Warning: "#f59e0b",
			Error:   "#ef4444",
			Info:    "#3b82f6",

			// 边框
			Border:      "#334155",
			BorderLight: "#475569",

			// 其他
			Overlay:   "rgba(0, 0, 0, 0.5)",
			Highlight: "rgba(59, 130, 246, 0.1)",
		},
		Typography: TypographyConfig{
			FontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
			FontSize: FontSizes{
				XS:   "0.75rem",
				SM:   "0.875rem",
				Base: "1rem",
				LG:   "1.125rem",
				XL:   "1.25rem",
				XXL:  "1.5rem",
			},
			LineHeight: "1.5",
		},
		Spacing: SpacingConfig{
			XS: "0.25rem",
			SM: "0.5rem",
			MD: "1rem",
			LG: "1.5rem",
			XL: "2rem",
		},
		Shadows: ShadowConfig{
			SM: "0 1px 2px 0 rgba(0, 0, 0, 0.05)",
			MD: "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)",
			LG: "0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)",
			XL: "0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)",
		},
	}
}

// GetStyles 获取主题样式
func (t *DarkTheme) GetStyles() *ThemeStyles {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.styles
}

// GetCSS 获取 CSS 变量
func (t *DarkTheme) GetCSS() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.styles == nil {
		return ""
	}

	c := t.styles.Colors
	return fmt.Sprintf(`
:root {
  /* 背景 */
  --color-background: %s;
  --color-background-secondary: %s;
  --color-background-tertiary: %s;

  /* 文字 */
  --color-text-primary: %s;
  --color-text-secondary: %s;
  --color-text-muted: %s;

  /* 强调色 */
  --color-primary: %s;
  --color-primary-hover: %s;
  --color-secondary: %s;
  --color-accent: %s;

  /* 状态色 */
  --color-success: %s;
  --color-warning: %s;
  --color-error: %s;
  --color-info: %s;

  /* 边框 */
  --color-border: %s;
  --color-border-light: %s;

  /* 其他 */
  --color-overlay: %s;
  --color-highlight: %s;

  /* 字体 */
  --font-family: %s;
  --font-size-xs: %s;
  --font-size-sm: %s;
  --font-size-base: %s;
  --font-size-lg: %s;
  --font-size-xl: %s;
  --font-size-2xl: %s;
  --line-height: %s;

  /* 间距 */
  --spacing-xs: %s;
  --spacing-sm: %s;
  --spacing-md: %s;
  --spacing-lg: %s;
  --spacing-xl: %s;

  /* 阴影 */
  --shadow-sm: %s;
  --shadow-md: %s;
  --shadow-lg: %s;
  --shadow-xl: %s;
}
`,
		c.Background, c.BackgroundSecondary, c.BackgroundTertiary,
		c.TextPrimary, c.TextSecondary, c.TextMuted,
		c.Primary, c.PrimaryHover, c.Secondary, c.Accent,
		c.Success, c.Warning, c.Error, c.Info,
		c.Border, c.BorderLight,
		c.Overlay, c.Highlight,
		t.styles.Typography.FontFamily,
		t.styles.Typography.FontSize.XS, t.styles.Typography.FontSize.SM,
		t.styles.Typography.FontSize.Base, t.styles.Typography.FontSize.LG,
		t.styles.Typography.FontSize.XL, t.styles.Typography.FontSize.XXL,
		t.styles.Typography.LineHeight,
		t.styles.Spacing.XS, t.styles.Spacing.SM, t.styles.Spacing.MD,
		t.styles.Spacing.LG, t.styles.Spacing.XL,
		t.styles.Shadows.SM, t.styles.Shadows.MD,
		t.styles.Shadows.LG, t.styles.Shadows.XL,
	)
}

// lightenColor 加亮颜色
func lightenColor(hex string, percent int) string {
	// 简单实现：将十六进制颜色加亮
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}

	r := hexToRGB(hex)
	r[0] = min(255, r[0]+r[0]*percent/100)
	r[1] = min(255, r[1]+r[1]*percent/100)
	r[2] = min(255, r[2]+r[2]*percent/100)

	return fmt.Sprintf("#%02x%02x%02x", r[0], r[1], r[2])
}

func hexToRGB(hex string) [3]int {
	var r, g, b int
	_, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	if err != nil {
		// 无效的hex格式，返回黑色
		return [3]int{0, 0, 0}
	}
	return [3]int{r, g, b}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 插件导入（实际使用时取消注释）
// import "nas-os/internal/plugin"

func main() {} // 插件模式需要 main 函数
