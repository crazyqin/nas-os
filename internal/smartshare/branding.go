// Package smartshare 提供自定义分享页面品牌化功能
package smartshare

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BrandingEngine 品牌化引擎
type BrandingEngine struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	configs map[string]*BrandingConfig // ID -> config
}

// NewBrandingEngine 创建品牌化引擎
func NewBrandingEngine(logger *zap.Logger) *BrandingEngine {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &BrandingEngine{
		logger:  logger,
		configs: make(map[string]*BrandingConfig),
	}
}

// BrandingRequest 品牌化请求
type BrandingRequest struct {
	ShareID string          `json:"share_id"`
	Config  *BrandingConfig `json:"config"`
}

// BrandingResponse 品牌化响应
type BrandingResponse struct {
	HTML        string    `json:"html"`
	CSS         string    `json:"css"`
	FullPage    string    `json:"full_page"`
	ShareID     string    `json:"share_id"`
	GeneratedAt time.Time `json:"generated_at"`
}

// CreateBranding 创建品牌配置
func (be *BrandingEngine) CreateBranding(id string, config *BrandingConfig) *BrandingConfig {
	be.mu.Lock()
	defer be.mu.Unlock()

	be.configs[id] = config

	be.logger.Info("branding config created",
		zap.String("id", id),
		zap.String("company", config.CompanyName))

	return config
}

// GetBranding 获取品牌配置
func (be *BrandingEngine) GetBranding(id string) (*BrandingConfig, error) {
	be.mu.RLock()
	defer be.mu.RUnlock()

	config, ok := be.configs[id]
	if !ok {
		return nil, fmt.Errorf("branding config not found: %s", id)
	}

	return config, nil
}

// UpdateBranding 更新品牌配置
func (be *BrandingEngine) UpdateBranding(id string, config *BrandingConfig) (*BrandingConfig, error) {
	be.mu.Lock()
	defer be.mu.Unlock()

	if _, ok := be.configs[id]; !ok {
		return nil, fmt.Errorf("branding config not found: %s", id)
	}

	be.configs[id] = config

	be.logger.Info("branding config updated",
		zap.String("id", id))

	return config, nil
}

// DeleteBranding 删除品牌配置
func (be *BrandingEngine) DeleteBranding(id string) error {
	be.mu.Lock()
	defer be.mu.Unlock()

	if _, ok := be.configs[id]; !ok {
		return fmt.Errorf("branding config not found: %s", id)
	}

	delete(be.configs, id)
	return nil
}

// GenerateBrandedPage 生成品牌化分享页面
func (be *BrandingEngine) GenerateBrandedPage(req *BrandingRequest) (*BrandingResponse, error) {
	be.mu.RLock()
	defer be.mu.RUnlock()

	config := req.Config
	if config == nil {
		config = DefaultBrandingConfig()
	}

	// 生成 CSS
	css := be.generateCSS(config)

	// 生成 HTML
	html := be.generateHTML(config)

	// 组合完整页面
	fullPage := be.wrapFullPage(css, html, config)

	return &BrandingResponse{
		HTML:        html,
		CSS:         css,
		FullPage:    fullPage,
		ShareID:     req.ShareID,
		GeneratedAt: time.Now(),
	}, nil
}

// generateCSS 生成 CSS
func (be *BrandingEngine) generateCSS(config *BrandingConfig) string {
	css := `
/* NAS-OS SmartShare Brand Styles */
.smartshare-container {
    max-width: 800px;
    margin: 0 auto;
    padding: 20px;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
}

.smartshare-header {
    display: flex;
    align-items: center;
    padding: 20px;
    background-color: %s;
    color: white;
    border-radius: 8px 8px 0 0;
}

.smartshare-logo {
    width: 40px;
    height: 40px;
    margin-right: 15px;
}

.smartshare-company-name {
    font-size: 24px;
    font-weight: bold;
}

.smartshare-body {
    padding: 30px;
    background-color: %s;
    color: %s;
    border: 1px solid #e8e8e8;
    border-top: none;
}

.smartshare-file-info {
    padding: 15px;
    background-color: #fafafa;
    border-radius: 4px;
    margin-bottom: 20px;
}

.smartshare-file-name {
    font-size: 18px;
    font-weight: 500;
    margin-bottom: 10px;
    color: %s;
}

.smartshare-file-size {
    color: #666;
    font-size: 14px;
}

.smartshare-actions {
    display: flex;
    gap: 10px;
    margin-top: 20px;
}

.smartshare-btn {
    padding: 10px 20px;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 14px;
    transition: opacity 0.2s;
}

.smartshare-btn:hover {
    opacity: 0.8;
}

.smartshare-btn-primary {
    background-color: %s;
    color: white;
}

.smartshare-btn-secondary {
    background-color: %s;
    color: white;
}

.smartshare-footer {
    padding: 15px 20px;
    background-color: #f5f5f5;
    text-align: center;
    font-size: 12px;
    color: #999;
    border-radius: 0 0 8px 8px;
    border: 1px solid #e8e8e8;
    border-top: none;
}

.smartshare-banner {
    width: 100%%;
    max-height: 200px;
    object-fit: cover;
    border-radius: 8px 8px 0 0;
}

/* Password Input */
.smartshare-password-form {
    margin: 20px 0;
}

.smartshare-password-input {
    padding: 10px 15px;
    border: 1px solid #d9d9d9;
    border-radius: 4px;
    width: 100%%;
    max-width: 300px;
    font-size: 14px;
}

/* Download Progress */
.smartshare-progress {
    width: 100%%;
    height: 4px;
    background-color: #f0f0f0;
    border-radius: 2px;
    margin-top: 10px;
    overflow: hidden;
}

.smartshare-progress-bar {
    height: 100%%;
    background-color: %s;
    transition: width 0.3s;
}
`

	return fmt.Sprintf(css,
		config.PrimaryColor,
		config.BackgroundColor,
		config.TextColor,
		config.PrimaryColor,
		config.PrimaryColor,
		config.SecondaryColor,
		config.PrimaryColor,
	)
}

// generateHTML 生成 HTML
func (be *BrandingEngine) generateHTML(config *BrandingConfig) string {
	var html strings.Builder

	// Banner
	if config.BannerImageURL != "" {
		html.WriteString(fmt.Sprintf(`<img src="%s" alt="Banner" class="smartshare-banner">`, config.BannerImageURL))
	}

	// Header
	html.WriteString(`<div class="smartshare-header">`)
	if config.LogoURL != "" {
		html.WriteString(fmt.Sprintf(`<img src="%s" alt="Logo" class="smartshare-logo">`, config.LogoURL))
	}
	html.WriteString(fmt.Sprintf(`<span class="smartshare-company-name">%s</span>`, config.CompanyName))
	html.WriteString(`</div>`)

	// Body
	html.WriteString(`<div class="smartshare-body">`)
	html.WriteString(`<div class="smartshare-file-info">`)
	html.WriteString(`<div class="smartshare-file-name">{{file_name}}</div>`)
	html.WriteString(`<div class="smartshare-file-size">{{file_size}}</div>`)
	html.WriteString(`</div>`)

	// Actions
	html.WriteString(`<div class="smartshare-actions">`)
	html.WriteString(`<button class="smartshare-btn smartshare-btn-primary" onclick="downloadFile()">下载文件</button>`)
	html.WriteString(`<button class="smartshare-btn smartshare-btn-secondary" onclick="previewFile()">在线预览</button>`)
	html.WriteString(`</div>`)

	// Custom HTML
	if config.CustomHTML != "" {
		html.WriteString(config.CustomHTML)
	}

	html.WriteString(`</div>`)

	// Footer
	html.WriteString(fmt.Sprintf(`<div class="smartshare-footer">%s</div>`, config.FooterText))

	return html.String()
}

// wrapFullPage 包装完整页面
func (be *BrandingEngine) wrapFullPage(css, html string, config *BrandingConfig) string {
	page := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - 文件分享</title>
    %s
    <style>%s</style>
    %s
</head>
<body>
    <div class="smartshare-container">
        %s
    </div>
    <script>
        function downloadFile() {
            // 下载逻辑
            window.location.href = '/api/v1/smartshare/download/{{share_id}}';
        }
        function previewFile() {
            // 预览逻辑
            window.open('/api/v1/smartshare/preview/{{share_id}}', '_blank');
        }
    </script>
</body>
</html>`,
		config.CompanyName,
		be.generateFaviconLink(config.FaviconURL),
		css,
		be.generateCustomCSS(config.CustomCSS),
		html,
	)

	return page
}

// generateFaviconLink 生成 favicon 链接
func (be *BrandingEngine) generateFaviconLink(faviconURL string) string {
	if faviconURL == "" {
		return ""
	}
	return fmt.Sprintf(`<link rel="icon" href="%s">`, faviconURL)
}

// generateCustomCSS 生成自定义 CSS
func (be *BrandingEngine) generateCustomCSS(customCSS string) string {
	if customCSS == "" {
		return ""
	}
	return fmt.Sprintf(`<style>%s</style>`, customCSS)
}

// GetDefaultBranding 获取默认品牌配置
func (be *BrandingEngine) GetDefaultBranding() *BrandingConfig {
	return DefaultBrandingConfig()
}
