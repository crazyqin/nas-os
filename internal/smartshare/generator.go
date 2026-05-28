// Package smartshare 提供分享链接生成功能
package smartshare

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// Generator 分享链接生成器
type Generator struct {
	logger     *zap.Logger
	baseURL    string
	shortDomain string
}

// NewGenerator 创建链接生成器
func NewGenerator(logger *zap.Logger) *Generator {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Generator{
		logger:      logger,
		baseURL:     "/share",
		shortDomain: "s.nas.local",
	}
}

// GenerateToken 生成唯一 Token
func (g *Generator) GenerateToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// GenerateShortCode 生成短链接码
func (g *Generator) GenerateShortCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	code := base64.URLEncoding.EncodeToString(b)
	// 取前8个字符，去掉特殊字符
	code = strings.NewReplacer("+", "", "/", "", "=", "").Replace(code)
	if len(code) > 8 {
		code = code[:8]
	}
	return strings.ToLower(code)
}

// GenerateOneTimeToken 生成一次性 Token
func (g *Generator) GenerateOneTimeToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "ot_" + hex.EncodeToString(b)
}

// GenerateQRCodeData 生成二维码数据
func (g *Generator) GenerateQRCodeData(token string) string {
	return fmt.Sprintf("https://%s/%s", g.shortDomain, token)
}

// GenerateShareURL 生成完整分享 URL
func (g *Generator) GenerateShareURL(token string) string {
	return fmt.Sprintf("%s/%s", g.baseURL, token)
}

// GeneratePasswordProtectedURL 生成密码保护的分享 URL
func (g *Generator) GeneratePasswordProtectedURL(token, passwordHint string) string {
	return fmt.Sprintf("%s/%s?hint=%s", g.baseURL, token, passwordHint)
}

// QRCodeResponse 二维码响应
type QRCodeResponse struct {
	Token      string `json:"token"`
	ShortCode  string `json:"short_code"`
	ShortURL   string `json:"short_url"`
	FullURL    string `json:"full_url"`
	QRCodeURL  string `json:"qr_code_url"`
	QRCodeData string `json:"qr_code_data"` // 二维码原始数据
}

// GenerateQRCode 生成二维码
func (g *Generator) GenerateQRCode(token string) *QRCodeResponse {
	shortCode := g.GenerateShortCode()

	return &QRCodeResponse{
		Token:      token,
		ShortCode:  shortCode,
		ShortURL:   fmt.Sprintf("https://%s/%s", g.shortDomain, shortCode),
		FullURL:    g.GenerateShareURL(token),
		QRCodeURL:  fmt.Sprintf("/api/v1/smartshare/qr/%s", token),
		QRCodeData: g.GenerateQRCodeData(token),
	}
}

// SetBaseURL 设置基础 URL
func (g *Generator) SetBaseURL(url string) {
	g.baseURL = url
}

// SetShortDomain 设置短链接域名
func (g *Generator) SetShortDomain(domain string) {
	g.shortDomain = domain
}
