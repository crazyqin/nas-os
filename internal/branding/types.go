// Package branding 提供白标品牌定制功能
package branding

import "time"

// BrandConfig 品牌配置.
type BrandConfig struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Logo        Logo         `json:"logo"`
	Theme       Theme        `json:"theme"`
	LoginScreen LoginScreen  `json:"login_screen"`
	CustomCSS   CustomCSS    `json:"custom_css"`
	Fonts       Fonts        `json:"fonts"`
	Favicon     string       `json:"favicon,omitempty"`
	CompanyName string       `json:"company_name"`
	Tagline     string       `json:"tagline,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Logo 品牌Logo配置.
type Logo struct {
	LightURL    string `json:"light_url,omitempty"`
	DarkURL     string `json:"dark_url,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Alt         string `json:"alt,omitempty"`
	FaviconURL  string `json:"favicon_url,omitempty"`
}

// Theme 主题配置.
type Theme struct {
	Mode         string            `json:"mode"` // light, dark, auto
	PrimaryColor string            `json:"primary_color"`
	AccentColor  string            `json:"accent_color,omitempty"`
	Colors       map[string]string `json:"colors,omitempty"`
	BorderRadius string            `json:"border_radius,omitempty"`
	Spacing      string            `json:"spacing,omitempty"`
}

// LoginScreen 登录页面定制.
type LoginScreen struct {
	Title         string `json:"title,omitempty"`
	Subtitle      string `json:"subtitle,omitempty"`
	BackgroundURL string `json:"background_url,omitempty"`
	BgColor       string `json:"bg_color,omitempty"`
	ShowLogo      bool   `json:"show_logo"`
	ShowTagline   bool   `json:"show_tagline"`
	FooterText    string `json:"footer_text,omitempty"`
}

// CustomCSS 自定义CSS.
type CustomCSS struct {
	Enabled bool   `json:"enabled"`
	Content string `json:"content,omitempty"`
	URL     string `json:"url,omitempty"`
}

// Fonts 字体配置.
type Fonts struct {
	Primary   string `json:"primary,omitempty"`
	Secondary string `json:"secondary,omitempty"`
	Monospace string `json:"monospace,omitempty"`
	GoogleURL string `json:"google_url,omitempty"`
}
