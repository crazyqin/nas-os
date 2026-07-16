// Package websharepro - 增强外链分享模块
// 参考 TrueNAS 26 WebShare、飞牛 fnOS 外链分享特性：
// - 密码保护、有效期、下载/访问次数限制
// - 中文自定义路径（参考飞牛 DDNS 中文域名特性）
// - 访问统计与地域分布
// - 批量生成分享链接
package websharepro

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// 增强版 ShareLink
// ---------------------------------------------------------------------------

// LinkStatus 链接状态.
type LinkStatus string

const (
	LinkActive    LinkStatus = "active"
	LinkExpired   LinkStatus = "expired"
	LinkRevoked   LinkStatus = "revoked"
	LinkExhausted LinkStatus = "exhausted" // 下载/访问次数耗尽
)

// ShareLinkV2 增强版共享链接.
type ShareLinkV2 struct {
	ID string `json:"id"`
	// 文件/目录路径
	Path string `json:"path"`
	// 显示名称
	Name string `json:"name"`
	// URL Token（用于公开访问）
	Token string `json:"token"`
	// 自定义路径 slug（支持中文，参考飞牛 DDNS 中文域名）
	// 例：/s/项目文档 或 /s/共享文件
	CustomSlug string `json:"customSlug,omitempty"`
	// 完整公开 URL
	PublicURL string `json:"publicUrl,omitempty"`
	// 权限
	Permission SharePermission `json:"permission"`
	// 密码保护（可选）
	Password string `json:"password,omitempty"`
	HasPwd   bool   `json:"hasPwd"`
	// 有效期
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// 下载限制
	MaxDownloads  int `json:"maxDownloads"` // 0=无限制
	DownloadCount int `json:"downloadCount"`
	// 访问限制
	MaxAccessCount int `json:"maxAccessCount"` // 0=无限制
	AccessCount    int `json:"accessCount"`
	// 所属者
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	// 状态
	Status LinkStatus `json:"status"`
	// 访问日志（最新 N 条）
	RecentAccess []AccessEntryV2 `json:"recentAccess,omitempty"`
}

// AccessEntryV2 增强版访问记录.
type AccessEntryV2 struct {
	IP        string    `json:"ip"`
	Country   string    `json:"country,omitempty"` // 国家
	Region    string    `json:"region,omitempty"`  // 省份/州
	City      string    `json:"city,omitempty"`    // 城市
	UserAgent string    `json:"userAgent"`
	Action    string    `json:"action"` // view, download, upload
	Timestamp time.Time `json:"timestamp"`
}

// ShareAnalytics 访问统计.
type ShareAnalytics struct {
	LinkID         string `json:"linkId"`
	TotalViews     int64  `json:"totalViews"`
	TotalDownloads int64  `json:"totalDownloads"`
	UniqueVisitors int64  `json:"uniqueVisitors"`
	// 地域分布 key=国家/地区 value=访问数
	GeoDistribution map[string]int64 `json:"geoDistribution"`
	// 每日访问量（最近 30 天）
	DailyViews []DailyCount `json:"dailyViews,omitempty"`
	// 访问来源（直接/搜索引擎/社交媒体等）
	ReferrerStats map[string]int64 `json:"referrerStats,omitempty"`
	// 设备类型
	DeviceStats map[string]int64 `json:"deviceStats,omitempty"`
	// 峰值时间
	PeakHour int `json:"peakHour"` // 0-23
}

// DailyCount 每日统计.
type DailyCount struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Views int64  `json:"views"`
}

// BatchShareLinkRequest 批量分享请求.
type BatchShareLinkRequest struct {
	// 要分享的路径列表
	Paths []string `json:"paths" binding:"required"`
	// 统一设置
	Name             string          `json:"name,omitempty"`
	Permission       SharePermission `json:"permission"`
	Password         string          `json:"password,omitempty"`
	ExpiryHours      int             `json:"expiryHours"`
	MaxDownloads     int             `json:"maxDownloads"`
	MaxAccessCount   int             `json:"maxAccessCount"`
	CreatedBy        string          `json:"createdBy" binding:"required"`
	CustomSlugPrefix string          `json:"customSlugPrefix,omitempty"` // 批量 slug 前缀
}

// BatchShareLinkResult 批量分享结果.
type BatchShareLinkResult struct {
	Total   int               `json:"total"`
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Links   []*ShareLinkV2    `json:"links"`
	Errors  []BatchShareError `json:"errors,omitempty"`
}

// BatchShareError 批量分享错误.
type BatchShareError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// ShareLinkConfig 创建链接的可选配置.
type ShareLinkConfig struct {
	Password       string `json:"password,omitempty"`
	ExpiryHours    int    `json:"expiryHours"`
	MaxDownloads   int    `json:"maxDownloads"`
	MaxAccessCount int    `json:"maxAccessCount"`
	CustomSlug     string `json:"customSlug,omitempty"`
}

// ShareLinkManager 增强版分享链接管理器.
type ShareLinkManager struct {
	mu        sync.RWMutex
	links     map[string]*ShareLinkV2    // id -> link
	tokenIdx  map[string]string          // token -> id
	slugIdx   map[string]string          // customSlug -> id
	analytics map[string]*ShareAnalytics // id -> analytics
	config    *WebShareConfig
	logger    *zap.Logger
}

// NewShareLinkManager 创建增强版链接管理器.
func NewShareLinkManager(config *WebShareConfig, logger *zap.Logger) *ShareLinkManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ShareLinkManager{
		links:     make(map[string]*ShareLinkV2),
		tokenIdx:  make(map[string]string),
		slugIdx:   make(map[string]string),
		analytics: make(map[string]*ShareAnalytics),
		config:    config,
		logger:    logger,
	}
}

// CreateLink 创建增强版分享链接.
func (m *ShareLinkManager) CreateLink(path, name, createdBy string, perm SharePermission, cfg *ShareLinkConfig) (*ShareLinkV2, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	token := generateToken()

	link := &ShareLinkV2{
		ID:           id,
		Path:         path,
		Name:         name,
		Token:        token,
		Permission:   perm,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now(),
		Status:       LinkActive,
		RecentAccess: make([]AccessEntryV2, 0),
	}

	// 应用配置
	if cfg != nil {
		link.Password = cfg.Password
		link.HasPwd = cfg.Password != ""
		link.MaxDownloads = cfg.MaxDownloads
		link.MaxAccessCount = cfg.MaxAccessCount

		// 有效期
		if cfg.ExpiryHours > 0 {
			t := time.Now().Add(time.Duration(cfg.ExpiryHours) * time.Hour)
			link.ExpiresAt = &t
		} else if m.config.DefaultExpiryHours > 0 {
			t := time.Now().Add(time.Duration(m.config.DefaultExpiryHours) * time.Hour)
			link.ExpiresAt = &t
		}

		// 自定义 slug（支持中文路径）
		if cfg.CustomSlug != "" {
			if existing, ok := m.slugIdx[cfg.CustomSlug]; ok {
				return nil, fmt.Errorf("custom slug already in use by link %s", existing)
			}
			link.CustomSlug = cfg.CustomSlug
			m.slugIdx[cfg.CustomSlug] = id
		}
	}

	// 构建公开 URL
	link.PublicURL = m.buildPublicURL(link)

	// 索引
	m.links[id] = link
	m.tokenIdx[token] = id

	// 初始化统计
	m.analytics[id] = &ShareAnalytics{
		LinkID:          id,
		GeoDistribution: make(map[string]int64),
		ReferrerStats:   make(map[string]int64),
		DeviceStats:     make(map[string]int64),
	}

	if perm == "" {
		link.Permission = PermReadOnly
	}

	m.logger.Info("share link created",
		zap.String("id", id),
		zap.String("path", path),
		zap.String("createdBy", createdBy),
		zap.Bool("passwordProtected", link.HasPwd),
	)
	return link, nil
}

// GetLink 获取链接.
func (m *ShareLinkManager) GetLink(id string) (*ShareLinkV2, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	link, ok := m.links[id]
	if !ok {
		return nil, false
	}
	// 检查过期
	if link.Status == LinkActive && link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		link.Status = LinkExpired
	}
	return link, true
}

// GetLinkByToken 通过 Token 获取链接.
func (m *ShareLinkManager) GetLinkByToken(token string) (*ShareLinkV2, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.tokenIdx[token]
	if !ok {
		return nil, false
	}
	link, ok := m.links[id]
	if !ok {
		return nil, false
	}
	// 状态检查
	if !m.isLinkAccessible(link) {
		return nil, false
	}
	return link, true
}

// GetLinkBySlug 通过自定义 Slug 获取链接.
func (m *ShareLinkManager) GetLinkBySlug(slug string) (*ShareLinkV2, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.slugIdx[slug]
	if !ok {
		return nil, false
	}
	link, ok := m.links[id]
	if !ok {
		return nil, false
	}
	if !m.isLinkAccessible(link) {
		return nil, false
	}
	return link, true
}

// ValidateAccess 验证访问（密码、下载/访问限制等）.
func (m *ShareLinkManager) ValidateAccess(id, password string) (*ShareLinkV2, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return nil, ErrLinkNotFound
	}

	// 检查链接状态
	if link.Status != LinkActive {
		return nil, fmt.Errorf("link is %s", link.Status)
	}

	// 检查过期
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		link.Status = LinkExpired
		return nil, fmt.Errorf("link expired at %s", link.ExpiresAt.Format(time.RFC3339))
	}

	// 验证密码
	if link.HasPwd && link.Password != password {
		return nil, fmt.Errorf("incorrect password")
	}

	// 检查访问次数
	if link.MaxAccessCount > 0 && link.AccessCount >= link.MaxAccessCount {
		link.Status = LinkExhausted
		return nil, fmt.Errorf("access limit reached (%d/%d)", link.AccessCount, link.MaxAccessCount)
	}

	// 递增访问计数
	link.AccessCount++

	return link, nil
}

// RecordDownload 记录下载.
func (m *ShareLinkManager) RecordDownload(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return ErrLinkNotFound
	}

	link.DownloadCount++
	// 检查下载次数限制
	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		link.Status = LinkExhausted
		m.logger.Info("share link download limit reached",
			zap.String("id", id),
			zap.Int("downloads", link.DownloadCount),
			zap.Int("maxDownloads", link.MaxDownloads),
		)
	}
	return nil
}

// RecordAccessV2 记录增强版访问.
func (m *ShareLinkManager) RecordAccessV2(id string, entry AccessEntryV2) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return
	}

	// 更新链接访问日志（保留最近 100 条）
	link.RecentAccess = append(link.RecentAccess, entry)
	if len(link.RecentAccess) > 100 {
		link.RecentAccess = link.RecentAccess[len(link.RecentAccess)-100:]
	}

	// 更新统计
	analytics, ok := m.analytics[id]
	if ok {
		analytics.TotalViews++
		if entry.Action == "download" {
			analytics.TotalDownloads++
		}
		// 地域分布
		region := entry.Country
		if region == "" {
			region = "unknown"
		}
		analytics.GeoDistribution[region]++
	}
}

// RevokeLink 撤销链接.
func (m *ShareLinkManager) RevokeLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return ErrLinkNotFound
	}
	link.Status = LinkRevoked
	m.logger.Info("share link revoked", zap.String("id", id))
	return nil
}

// DeleteLink 删除链接.
func (m *ShareLinkManager) DeleteLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return ErrLinkNotFound
	}

	// 清理索引
	delete(m.tokenIdx, link.Token)
	if link.CustomSlug != "" {
		delete(m.slugIdx, link.CustomSlug)
	}
	delete(m.links, id)
	delete(m.analytics, id)

	m.logger.Info("share link deleted", zap.String("id", id))
	return nil
}

// ListLinks 列出链接.
func (m *ShareLinkManager) ListLinks(createdBy string, status LinkStatus) []*ShareLinkV2 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ShareLinkV2
	for _, link := range m.links {
		if createdBy != "" && link.CreatedBy != createdBy {
			continue
		}
		if status != "" && link.Status != status {
			continue
		}
		result = append(result, link)
	}
	return result
}

// GetAnalytics 获取链接访问统计.
func (m *ShareLinkManager) GetAnalytics(id string) (*ShareAnalytics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.analytics[id]
	return a, ok
}

// BatchCreateLinks 批量创建分享链接.
func (m *ShareLinkManager) BatchCreateLinks(req *BatchShareLinkRequest) *BatchShareLinkResult {
	result := &BatchShareLinkResult{
		Total:  len(req.Paths),
		Links:  make([]*ShareLinkV2, 0, len(req.Paths)),
		Errors: make([]BatchShareError, 0),
	}

	for i, path := range req.Paths {
		cfg := &ShareLinkConfig{
			Password:       req.Password,
			ExpiryHours:    req.ExpiryHours,
			MaxDownloads:   req.MaxDownloads,
			MaxAccessCount: req.MaxAccessCount,
		}

		// 自动生成中文 slug
		if req.CustomSlugPrefix != "" {
			cfg.CustomSlug = fmt.Sprintf("%s/%d", req.CustomSlugPrefix, i+1)
		}

		name := req.Name
		if name == "" {
			name = fmt.Sprintf("分享-%d", i+1)
		}

		link, err := m.CreateLink(path, name, req.CreatedBy, req.Permission, cfg)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchShareError{Path: path, Error: err.Error()})
			continue
		}
		result.Success++
		result.Links = append(result.Links, link)
	}

	m.logger.Info("batch share links created",
		zap.Int("total", result.Total),
		zap.Int("success", result.Success),
		zap.Int("failed", result.Failed),
	)
	return result
}

// CleanupExpired 清理过期链接.
func (m *ShareLinkManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()
	for _, link := range m.links {
		if link.Status == LinkActive && link.ExpiresAt != nil && link.ExpiresAt.Before(now) {
			link.Status = LinkExpired
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// 内部辅助方法
// ---------------------------------------------------------------------------

// isLinkAccessible 检查链接是否可访问.
func (m *ShareLinkManager) isLinkAccessible(link *ShareLinkV2) bool {
	if link.Status != LinkActive {
		return false
	}
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		link.Status = LinkExpired
		return false
	}
	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		link.Status = LinkExhausted
		return false
	}
	if link.MaxAccessCount > 0 && link.AccessCount >= link.MaxAccessCount {
		link.Status = LinkExhausted
		return false
	}
	return true
}

// buildPublicURL 构建公开访问 URL.
func (m *ShareLinkManager) buildPublicURL(link *ShareLinkV2) string {
	base := m.config.BaseURL
	if base == "" {
		base = "http://localhost:8080"
	}

	// 优先使用自定义 slug（支持中文）
	if link.CustomSlug != "" {
		return base + "/s/" + url.PathEscape(link.CustomSlug)
	}
	return base + "/s/" + link.Token
}

// generateShareID 生成分享 ID.
func generateShareID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}
