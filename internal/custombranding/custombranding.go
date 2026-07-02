// Package custombranding 提供品牌定制引擎功能
// 支持自定义 Logo、颜色方案、字体、启动画面、主题系统、CSS/SCSS 变量管理
// 多语言品牌适配、品牌资产管理、模板系统、实时预览和配置导入导出
package custombranding

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// BrandingEngine 品牌定制引擎主入口.
type BrandingEngine struct {
	mu              sync.Mutex             // 并发保护
	config          *BrandingConfig        // 当前品牌配置
	themes          map[string]*Theme      // 主题集合
	assets          map[string]*BrandAsset // 品牌资产
	templates       map[string]*Template   // 预设模板
	cssVars         map[string]string      // CSS/SCSS 变量
	locales         map[string]*Locale     // 多语言配置
	previewCallback func(*BrandingConfig)  // 预览回调
	running         bool                   // 运行状态
	startTime       time.Time              // 启动时间
	version         string                 // 版本号
	history         []*ConfigSnapshot      // 配置快照历史
	maxHistory      int                    // 最大历史记录数
}

// BrandingConfig 品牌配置.
type BrandingConfig struct {
	Name      string            `json:"name"`       // 品牌名称
	Logo      LogoConfig        `json:"logo"`       // Logo 配置
	Colors    ColorScheme       `json:"colors"`     // 颜色方案
	Fonts     FontConfig        `json:"fonts"`      // 字体配置
	Splash    SplashConfig      `json:"splash"`     // 启动画面
	ThemeID   string            `json:"theme_id"`   // 当前主题 ID
	Locale    string            `json:"locale"`     // 当前语言
	CustomCSS map[string]string `json:"custom_css"` // 自定义 CSS 变量
	UpdatedAt time.Time         `json:"updated_at"` // 更新时间
	Version   int               `json:"version"`    // 配置版本
}

// LogoConfig Logo 配置.
type LogoConfig struct {
	Primary   string `json:"primary"`   // 主 Logo 路径
	Secondary string `json:"secondary"` // 备用 Logo
	Favicon   string `json:"favicon"`   // 网站图标
	Width     int    `json:"width"`     // 宽度
	Height    int    `json:"height"`    // 高度
	AltText   string `json:"alt_text"`  // 替代文本
	DarkMode  string `json:"dark_mode"` // 暗色模式 Logo
}

// ColorScheme 颜色方案.
type ColorScheme struct {
	Primary    string `json:"primary"`    // 主色
	Secondary  string `json:"secondary"`  // 次色
	Accent     string `json:"accent"`     // 强调色
	Background string `json:"background"` // 背景色
	Surface    string `json:"surface"`    // 表面色
	Text       string `json:"text"`       // 文本色
	TextLight  string `json:"text_light"` // 浅色文本
	Border     string `json:"border"`     // 边框色
	Success    string `json:"success"`    // 成功色
	Warning    string `json:"warning"`    // 警告色
	Error      string `json:"error"`      // 错误色
	Info       string `json:"info"`       // 信息色
}

// FontConfig 字体配置.
type FontConfig struct {
	Primary    string `json:"primary"`     // 主字体
	Secondary  string `json:"secondary"`   // 备用字体
	Monospace  string `json:"monospace"`   // 等宽字体
	BaseSize   string `json:"base_size"`   // 基础字号
	LineHeight string `json:"line_height"` // 行高
	Weight     string `json:"weight"`      // 字重
}

// SplashConfig 启动画面配置.
type SplashConfig struct {
	Enabled   bool   `json:"enabled"`    // 是否启用
	Image     string `json:"image"`      // 启动图片
	BgColor   string `json:"bg_color"`   // 背景色
	TextColor string `json:"text_color"` // 文本色
	Duration  int    `json:"duration"`   // 显示时长(ms)
	Animation string `json:"animation"`  // 动画效果
	Message   string `json:"message"`    // 启动消息
}

// Theme 主题定义.
type Theme struct {
	ID          string         `json:"id"`          // 主题 ID
	Name        string         `json:"name"`        // 主题名称
	Description string         `json:"description"` // 主题描述
	Config      BrandingConfig `json:"config"`      // 主题配置
	IsPreset    bool           `json:"is_preset"`   // 是否预设主题
	Tags        []string       `json:"tags"`        // 标签
	CreatedAt   time.Time      `json:"created_at"`  // 创建时间
	Author      string         `json:"author"`      // 作者
	Thumbnail   string         `json:"thumbnail"`   // 缩略图
}

// BrandAsset 品牌资产.
type BrandAsset struct {
	ID         string    `json:"id"`          // 资产 ID
	Name       string    `json:"name"`        // 资产名称
	Type       string    `json:"type"`        // 类型(image/icon/font等)
	Path       string    `json:"path"`        // 存储路径
	Size       int64     `json:"size"`        // 文件大小
	MimeType   string    `json:"mime_type"`   // MIME 类型
	Version    int       `json:"version"`     // 版本号
	UploadedAt time.Time `json:"uploaded_at"` // 上传时间
	UploadedBy string    `json:"uploaded_by"` // 上传者
	Checksum   string    `json:"checksum"`    // 校验和
	Tags       []string  `json:"tags"`        // 标签
}

// Template 品牌模板.
type Template struct {
	ID          string         `json:"id"`          // 模板 ID
	Name        string         `json:"name"`        // 模板名称
	Description string         `json:"description"` // 模板描述
	Category    string         `json:"category"`    // 分类(enterprise/personal/creative)
	Config      BrandingConfig `json:"config"`      // 配置
	Preview     string         `json:"preview"`     // 预览图
}

// Locale 多语言配置.
type Locale struct {
	Code       string            `json:"code"`        // 语言代码
	Name       string            `json:"name"`        // 语言名称
	Direction  string            `json:"direction"`   // 文本方向(ltr/rtl)
	BrandNames map[string]string `json:"brand_names"` // 多语言品牌名
	Slogans    map[string]string `json:"slogans"`     // 多语言标语
	DateFormat string            `json:"date_format"` // 日期格式
}

// ConfigSnapshot 配置快照.
type ConfigSnapshot struct {
	Timestamp time.Time      `json:"timestamp"` // 快照时间
	Config    BrandingConfig `json:"config"`    // 配置快照
	Reason    string         `json:"reason"`    // 变更原因
}

// init 注册模块初始化.
func init() {
	// 模块加载时自动注册到 nas-os 模块系统
	fmt.Println("[custombranding] 品牌定制引擎模块已加载")
}

// New 创建品牌定制引擎实例.
func New() *BrandingEngine {
	engine := &BrandingEngine{
		themes:     make(map[string]*Theme),
		assets:     make(map[string]*BrandAsset),
		templates:  make(map[string]*Template),
		cssVars:    make(map[string]string),
		locales:    make(map[string]*Locale),
		version:    "1.0.0",
		maxHistory: 50,
	}

	// 初始化默认配置
	engine.config = engine.defaultConfig()

	// 加载预设主题
	engine.loadPresetThemes()

	// 加载预设模板
	engine.loadPresetTemplates()

	// 初始化默认语言
	engine.initDefaultLocales()

	return engine
}

// Start 启动品牌定制引擎.
func (e *BrandingEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("品牌定制引擎已在运行")
	}

	e.running = true
	e.startTime = time.Now()

	// 加载持久化配置
	if err := e.loadConfig(); err != nil {
		fmt.Printf("[custombranding] 加载配置失败: %v，使用默认配置\n", err)
	}

	// 编译 CSS 变量
	if err := e.compileCSSVars(); err != nil {
		return fmt.Errorf("编译 CSS 变量失败: %w", err)
	}

	fmt.Printf("[custombranding] 品牌定制引擎已启动，当前主题: %s\n", e.config.ThemeID)
	return nil
}

// Stop 停止品牌定制引擎.
func (e *BrandingEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("品牌定制引擎未运行")
	}

	// 保存当前配置
	if err := e.saveConfig(); err != nil {
		fmt.Printf("[custombranding] 保存配置失败: %v\n", err)
	}

	e.running = false
	fmt.Println("[custombranding] 品牌定制引擎已停止")
	return nil
}

// ApplyTheme 应用主题.
func (e *BrandingEngine) ApplyTheme(themeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	theme, exists := e.themes[themeID]
	if !exists {
		return fmt.Errorf("主题不存在: %s", themeID)
	}

	// 创建快照
	e.createSnapshot("应用主题: " + theme.Name)

	// 应用主题配置
	e.config = &theme.Config
	e.config.ThemeID = themeID
	e.config.UpdatedAt = time.Now()
	e.config.Version++

	// 重新编译 CSS 变量
	if err := e.compileCSSVars(); err != nil {
		return fmt.Errorf("编译 CSS 变量失败: %w", err)
	}

	// 触发预览
	if e.previewCallback != nil {
		go e.previewCallback(e.config)
	}

	fmt.Printf("[custombranding] 已应用主题: %s (v%d)\n", theme.Name, e.config.Version)
	return nil
}

// GetThemes 获取所有主题列表.
func (e *BrandingEngine) GetThemes() map[string]*Theme {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make(map[string]*Theme)
	for k, v := range e.themes {
		result[k] = v
	}
	return result
}

// ExportConfig 导出品牌配置.
func (e *BrandingEngine) ExportConfig() ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	export := struct {
		Config    *BrandingConfig      `json:"config"`
		Themes    map[string]*Theme    `json:"themes"`
		Templates map[string]*Template `json:"templates"`
		CSSVars   map[string]string    `json:"css_vars"`
		Locales   map[string]*Locale   `json:"locales"`
		ExportAt  time.Time            `json:"export_at"`
		Version   string               `json:"version"`
	}{
		Config:    e.config,
		Themes:    e.themes,
		Templates: e.templates,
		CSSVars:   e.cssVars,
		Locales:   e.locales,
		ExportAt:  time.Now(),
		Version:   e.version,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	return data, nil
}

// ImportConfig 导入品牌配置.
func (e *BrandingEngine) ImportConfig(data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var importData struct {
		Config  *BrandingConfig   `json:"config"`
		Themes  map[string]*Theme `json:"themes"`
		CSSVars map[string]string `json:"css_vars"`
	}

	if err := json.Unmarshal(data, &importData); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	// 创建快照
	e.createSnapshot("导入配置")

	// 合并配置
	if importData.Config != nil {
		e.config = importData.Config
		e.config.UpdatedAt = time.Now()
		e.config.Version++
	}

	if importData.Themes != nil {
		for k, v := range importData.Themes {
			e.themes[k] = v
		}
	}

	if importData.CSSVars != nil {
		for k, v := range importData.CSSVars {
			e.cssVars[k] = v
		}
	}

	fmt.Printf("[custombranding] 配置导入成功 (v%d)\n", e.config.Version)
	return nil
}

// UpdateConfig 更新品牌配置.
func (e *BrandingEngine) UpdateConfig(updates *BrandingConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.createSnapshot("更新配置")

	// 合并更新
	if updates.Name != "" {
		e.config.Name = updates.Name
	}
	if updates.Logo.Primary != "" {
		e.config.Logo = updates.Logo
	}
	if updates.Colors.Primary != "" {
		e.config.Colors = updates.Colors
	}
	if updates.Fonts.Primary != "" {
		e.config.Fonts = updates.Fonts
	}
	if updates.Locale != "" {
		e.config.Locale = updates.Locale
	}

	e.config.UpdatedAt = time.Now()
	e.config.Version++

	// 重新编译 CSS 变量
	if err := e.compileCSSVars(); err != nil {
		return fmt.Errorf("编译 CSS 变量失败: %w", err)
	}

	// 触发预览
	if e.previewCallback != nil {
		go e.previewCallback(e.config)
	}

	return nil
}

// UploadAsset 上传品牌资产.
func (e *BrandingEngine) UploadAsset(name, assetType, path, mimeType, uploadedBy string, data []byte) (*BrandAsset, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	assetID := fmt.Sprintf("%s_%d", name, time.Now().UnixNano())

	asset := &BrandAsset{
		ID:         assetID,
		Name:       name,
		Type:       assetType,
		Path:       path,
		Size:       int64(len(data)),
		MimeType:   mimeType,
		Version:    1,
		UploadedAt: time.Now(),
		UploadedBy: uploadedBy,
		Checksum:   fmt.Sprintf("%x", len(data)), // 简化校验和
	}

	e.assets[assetID] = asset

	fmt.Printf("[custombranding] 资产已上传: %s (%s)\n", name, assetID)
	return asset, nil
}

// GetAsset 获取品牌资产.
func (e *BrandingEngine) GetAsset(assetID string) (*BrandAsset, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	asset, exists := e.assets[assetID]
	if !exists {
		return nil, fmt.Errorf("资产不存在: %s", assetID)
	}

	return asset, nil
}

// ListAssets 列出所有品牌资产.
func (e *BrandingEngine) ListAssets() map[string]*BrandAsset {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make(map[string]*BrandAsset)
	for k, v := range e.assets {
		result[k] = v
	}
	return result
}

// SetPreviewCallback 设置预览回调.
func (e *BrandingEngine) SetPreviewCallback(callback func(*BrandingConfig)) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.previewCallback = callback
}

// GetCSSVars 获取 CSS 变量.
func (e *BrandingEngine) GetCSSVars() map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make(map[string]string)
	for k, v := range e.cssVars {
		result[k] = v
	}
	return result
}

// SetCSSVar 设置 CSS 变量.
func (e *BrandingEngine) SetCSSVar(key, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cssVars[key] = value
}

// GetLocale 获取语言配置.
func (e *BrandingEngine) GetLocale(code string) (*Locale, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	locale, exists := e.locales[code]
	if !exists {
		return nil, fmt.Errorf("语言配置不存在: %s", code)
	}

	return locale, nil
}

// CreateTheme 从当前配置创建自定义主题.
func (e *BrandingEngine) CreateTheme(name, description string, tags []string) (*Theme, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	themeID := fmt.Sprintf("custom_%d", time.Now().UnixNano())

	theme := &Theme{
		ID:          themeID,
		Name:        name,
		Description: description,
		Config:      *e.config,
		IsPreset:    false,
		Tags:        tags,
		CreatedAt:   time.Now(),
	}

	e.themes[themeID] = theme

	fmt.Printf("[custombranding] 自定义主题已创建: %s (%s)\n", name, themeID)
	return theme, nil
}

// GetTemplate 获取模板.
func (e *BrandingEngine) GetTemplate(templateID string) (*Template, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	template, exists := e.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	return template, nil
}

// GetTemplatesByCategory 按分类获取模板.
func (e *BrandingEngine) GetTemplatesByCategory(category string) []*Template {
	e.mu.Lock()
	defer e.mu.Unlock()

	var result []*Template
	for _, t := range e.templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// ApplyTemplate 应用模板.
func (e *BrandingEngine) ApplyTemplate(templateID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	template, exists := e.templates[templateID]
	if !exists {
		return fmt.Errorf("模板不存在: %s", templateID)
	}

	e.createSnapshot("应用模板: " + template.Name)

	e.config = &template.Config
	e.config.UpdatedAt = time.Now()
	e.config.Version++

	if err := e.compileCSSVars(); err != nil {
		return fmt.Errorf("编译 CSS 变量失败: %w", err)
	}

	if e.previewCallback != nil {
		go e.previewCallback(e.config)
	}

	return nil
}

// GetConfig 获取当前配置.
func (e *BrandingEngine) GetConfig() *BrandingConfig {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.config
}

// GetHistory 获取配置历史.
func (e *BrandingEngine) GetHistory() []*ConfigSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]*ConfigSnapshot, len(e.history))
	copy(result, e.history)
	return result
}

// 内部方法

// defaultConfig 生成默认配置.
func (e *BrandingEngine) defaultConfig() *BrandingConfig {
	return &BrandingConfig{
		Name: "NAS-OS",
		Logo: LogoConfig{
			Primary: "/assets/logo.svg",
			Favicon: "/assets/favicon.ico",
			Width:   200,
			Height:  60,
			AltText: "NAS-OS Logo",
		},
		Colors: ColorScheme{
			Primary:    "#1976D2",
			Secondary:  "#424242",
			Accent:     "#82B1FF",
			Background: "#FFFFFF",
			Surface:    "#F5F5F5",
			Text:       "#212121",
			TextLight:  "#757575",
			Border:     "#E0E0E0",
			Success:    "#4CAF50",
			Warning:    "#FF9800",
			Error:      "#F44336",
			Info:       "#2196F3",
		},
		Fonts: FontConfig{
			Primary:    "Inter, system-ui, sans-serif",
			Secondary:  "Roboto, sans-serif",
			Monospace:  "JetBrains Mono, monospace",
			BaseSize:   "16px",
			LineHeight: "1.5",
			Weight:     "400",
		},
		Splash: SplashConfig{
			Enabled:   true,
			BgColor:   "#1976D2",
			TextColor: "#FFFFFF",
			Duration:  2000,
			Animation: "fade",
			Message:   "正在加载...",
		},
		ThemeID:   "default",
		Locale:    "zh-CN",
		CustomCSS: make(map[string]string),
		UpdatedAt: time.Now(),
		Version:   1,
	}
}

// loadPresetThemes 加载预设主题.
func (e *BrandingEngine) loadPresetThemes() {
	// 默认主题
	e.themes["default"] = &Theme{
		ID:          "default",
		Name:        "默认主题",
		Description: "NAS-OS 默认品牌主题",
		Config:      *e.config,
		IsPreset:    true,
		Tags:        []string{"default", "light"},
		CreatedAt:   time.Now(),
	}

	// 暗色主题
	darkConfig := *e.config
	darkConfig.Colors = ColorScheme{
		Primary:    "#90CAF9",
		Secondary:  "#BDBDBD",
		Accent:     "#82B1FF",
		Background: "#121212",
		Surface:    "#1E1E1E",
		Text:       "#FFFFFF",
		TextLight:  "#BDBDBD",
		Border:     "#333333",
		Success:    "#66BB6A",
		Warning:    "#FFA726",
		Error:      "#EF5350",
		Info:       "#42A5F5",
	}
	e.themes["dark"] = &Theme{
		ID:          "dark",
		Name:        "暗色主题",
		Description: "适合夜间使用的暗色主题",
		Config:      darkConfig,
		IsPreset:    true,
		Tags:        []string{"dark", "night"},
		CreatedAt:   time.Now(),
	}

	// 科技主题
	techConfig := *e.config
	techConfig.Colors = ColorScheme{
		Primary:    "#00BCD4",
		Secondary:  "#607D8B",
		Accent:     "#00E5FF",
		Background: "#0D1117",
		Surface:    "#161B22",
		Text:       "#C9D1D9",
		TextLight:  "#8B949E",
		Border:     "#30363D",
		Success:    "#3FB950",
		Warning:    "#D29922",
		Error:      "#F85149",
		Info:       "#58A6FF",
	}
	e.themes["tech"] = &Theme{
		ID:          "tech",
		Name:        "科技主题",
		Description: "现代科技风格主题",
		Config:      techConfig,
		IsPreset:    true,
		Tags:        []string{"tech", "modern"},
		CreatedAt:   time.Now(),
	}

	// 企业主题
	bizConfig := *e.config
	bizConfig.Colors = ColorScheme{
		Primary:    "#1565C0",
		Secondary:  "#37474F",
		Accent:     "#FF6F00",
		Background: "#FAFAFA",
		Surface:    "#FFFFFF",
		Text:       "#263238",
		TextLight:  "#546E7A",
		Border:     "#CFD8DC",
		Success:    "#2E7D32",
		Warning:    "#EF6C00",
		Error:      "#C62828",
		Info:       "#0277BD",
	}
	e.themes["business"] = &Theme{
		ID:          "business",
		Name:        "企业主题",
		Description: "专业商务风格主题",
		Config:      bizConfig,
		IsPreset:    true,
		Tags:        []string{"business", "professional"},
		CreatedAt:   time.Now(),
	}
}

// loadPresetTemplates 加载预设模板.
func (e *BrandingEngine) loadPresetTemplates() {
	// 企业模板
	e.templates["enterprise"] = &Template{
		ID:          "enterprise",
		Name:        "企业标准",
		Description: "适合企业级部署的专业模板",
		Category:    "enterprise",
		Config:      *e.themes["business"].Config.Clone(),
		Preview:     "/templates/enterprise.png",
	}

	// 个人模板
	personalConfig := *e.config
	personalConfig.Colors.Primary = "#6200EE"
	personalConfig.Colors.Secondary = "#03DAC6"
	personalConfig.Fonts.Primary = "Noto Sans SC, sans-serif"
	e.templates["personal"] = &Template{
		ID:          "personal",
		Name:        "个人风格",
		Description: "简洁清爽的个人使用模板",
		Category:    "personal",
		Config:      personalConfig,
		Preview:     "/templates/personal.png",
	}

	// 创意模板
	creativeConfig := *e.config
	creativeConfig.Colors = ColorScheme{
		Primary:    "#E91E63",
		Secondary:  "#9C27B0",
		Accent:     "#FF5722",
		Background: "#FFF8E1",
		Surface:    "#FFFFFF",
		Text:       "#3E2723",
		TextLight:  "#795548",
		Border:     "#D7CCC8",
		Success:    "#8BC34A",
		Warning:    "#FFC107",
		Error:      "#E91E63",
		Info:       "#3F51B5",
	}
	creativeConfig.Fonts.Primary = "ZCOOL KuaiLe, cursive"
	e.templates["creative"] = &Template{
		ID:          "creative",
		Name:        "创意风格",
		Description: "大胆配色的创意设计模板",
		Category:    "creative",
		Config:      creativeConfig,
		Preview:     "/templates/creative.png",
	}
}

// initDefaultLocales 初始化默认语言.
func (e *BrandingEngine) initDefaultLocales() {
	e.locales["zh-CN"] = &Locale{
		Code:      "zh-CN",
		Name:      "简体中文",
		Direction: "ltr",
		BrandNames: map[string]string{
			"default": "NAS-OS",
		},
		Slogans: map[string]string{
			"default": "智能网络存储系统",
		},
		DateFormat: "2006-01-02 15:04:05",
	}

	e.locales["en-US"] = &Locale{
		Code:      "en-US",
		Name:      "English",
		Direction: "ltr",
		BrandNames: map[string]string{
			"default": "NAS-OS",
		},
		Slogans: map[string]string{
			"default": "Smart Network Storage System",
		},
		DateFormat: "01/02/2006 03:04:05 PM",
	}

	e.locales["ja-JP"] = &Locale{
		Code:      "ja-JP",
		Name:      "日本語",
		Direction: "ltr",
		BrandNames: map[string]string{
			"default": "NAS-OS",
		},
		Slogans: map[string]string{
			"default": "スマートネットワークストレージシステム",
		},
		DateFormat: "2006年01月02日 15時04分05秒",
	}
}

// compileCSSVars 编译 CSS 变量.
func (e *BrandingEngine) compileCSSVars() error {
	// 基础颜色变量
	e.cssVars["--color-primary"] = e.config.Colors.Primary
	e.cssVars["--color-secondary"] = e.config.Colors.Secondary
	e.cssVars["--color-accent"] = e.config.Colors.Accent
	e.cssVars["--color-background"] = e.config.Colors.Background
	e.cssVars["--color-surface"] = e.config.Colors.Surface
	e.cssVars["--color-text"] = e.config.Colors.Text
	e.cssVars["--color-text-light"] = e.config.Colors.TextLight
	e.cssVars["--color-border"] = e.config.Colors.Border
	e.cssVars["--color-success"] = e.config.Colors.Success
	e.cssVars["--color-warning"] = e.config.Colors.Warning
	e.cssVars["--color-error"] = e.config.Colors.Error
	e.cssVars["--color-info"] = e.config.Colors.Info

	// 字体变量
	e.cssVars["--font-primary"] = e.config.Fonts.Primary
	e.cssVars["--font-secondary"] = e.config.Fonts.Secondary
	e.cssVars["--font-monospace"] = e.config.Fonts.Monospace
	e.cssVars["--font-size-base"] = e.config.Fonts.BaseSize
	e.cssVars["--line-height"] = e.config.Fonts.LineHeight

	// Logo 变量
	e.cssVars["--logo-primary"] = e.config.Logo.Primary
	e.cssVars["--logo-width"] = fmt.Sprintf("%dpx", e.config.Logo.Width)
	e.cssVars["--logo-height"] = fmt.Sprintf("%dpx", e.config.Logo.Height)

	// 自定义变量
	for k, v := range e.config.CustomCSS {
		e.cssVars[k] = v
	}

	return nil
}

// createSnapshot 创建配置快照.
func (e *BrandingEngine) createSnapshot(reason string) {
	snapshot := &ConfigSnapshot{
		Timestamp: time.Now(),
		Config:    *e.config,
		Reason:    reason,
	}

	e.history = append(e.history, snapshot)

	// 限制历史记录数量
	if len(e.history) > e.maxHistory {
		e.history = e.history[len(e.history)-e.maxHistory:]
	}
}

// loadConfig 加载持久化配置 (简化实现).
func (e *BrandingEngine) loadConfig() error {
	// 实际实现中从文件或数据库加载
	// 这里使用默认配置
	return nil
}

// saveConfig 保存配置到持久化存储 (简化实现).
func (e *BrandingEngine) saveConfig() error {
	// 实际实现中保存到文件或数据库
	return nil
}

// Clone 克隆配置.
func (c *BrandingConfig) Clone() *BrandingConfig {
	clone := *c
	clone.CustomCSS = make(map[string]string)
	for k, v := range c.CustomCSS {
		clone.CustomCSS[k] = v
	}
	return &clone
}
