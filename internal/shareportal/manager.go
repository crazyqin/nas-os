// Package shareportal 提供文件分享门户功能
package shareportal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 分享门户管理器
type Manager struct {
	mu          sync.RWMutex
	storagePath string

	// 内存存储（生产环境应使用数据库）
	links     map[string]*ShareLink     // id -> link
	shortURLs map[string]string         // shortURL -> linkID
	brandings map[string]*ShareBranding // id -> branding
	portals   map[string]*SharePortal   // id -> portal
	accesses  map[string][]*ShareAccess // linkID -> accesses
	uploads   map[string][]*ShareUpload // linkID -> uploads
}

// NewManager 创建分享门户管理器
func NewManager(storagePath string) *Manager {
	return &Manager{
		storagePath: storagePath,
		links:       make(map[string]*ShareLink),
		shortURLs:   make(map[string]string),
		brandings:   make(map[string]*ShareBranding),
		portals:     make(map[string]*SharePortal),
		accesses:    make(map[string][]*ShareAccess),
		uploads:     make(map[string][]*ShareUpload),
	}
}

// CreateShare 创建分享链接
func (m *Manager) CreateShare(ctx context.Context, link ShareLink) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if link.ID == "" {
		link.ID = uuid.New().String()
	}

	// 生成短链
	if link.ShortURL == "" {
		link.ShortURL = m.generateShortURL()
	}

	// 设置默认值
	link.IsActive = true
	link.CreatedAt = time.Now()
	link.UpdatedAt = time.Now()

	m.links[link.ID] = &link
	m.shortURLs[link.ShortURL] = link.ID

	return &link, nil
}

// UpdateShare 更新分享链接
func (m *Manager) UpdateShare(ctx context.Context, linkID string, updates ShareLink) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, exists := m.links[linkID]
	if !exists {
		return nil, ErrShareNotFound
	}

	// 更新字段
	if updates.Name != "" {
		link.Name = updates.Name
	}
	if updates.FilePath != "" {
		link.FilePath = updates.FilePath
	}
	if updates.Password != "" {
		link.Password = updates.Password
	}
	if updates.ExpiresAt != nil {
		link.ExpiresAt = updates.ExpiresAt
	}
	if updates.MaxDownloads > 0 {
		link.MaxDownloads = updates.MaxDownloads
	}
	if updates.BrandingID != "" {
		link.BrandingID = updates.BrandingID
	}

	// 布尔字段直接覆盖
	link.AllowPreview = updates.AllowPreview
	link.AllowDownload = updates.AllowDownload
	link.AllowUpload = updates.AllowUpload
	link.IsActive = updates.IsActive

	link.UpdatedAt = time.Now()

	return link, nil
}

// DeleteShare 删除分享链接
func (m *Manager) DeleteShare(ctx context.Context, linkID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, exists := m.links[linkID]
	if !exists {
		return ErrShareNotFound
	}

	// 删除短链映射
	delete(m.shortURLs, link.ShortURL)
	delete(m.links, linkID)
	delete(m.accesses, linkID)
	delete(m.uploads, linkID)

	return nil
}

// GetShare 获取分享信息
func (m *Manager) GetShare(ctx context.Context, linkID string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.links[linkID]
	if !exists {
		return nil, ErrShareNotFound
	}

	return link, nil
}

// GetShareByShortURL 通过短链获取分享信息
func (m *Manager) GetShareByShortURL(ctx context.Context, shortURL string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	linkID, exists := m.shortURLs[shortURL]
	if !exists {
		return nil, ErrShareNotFound
	}

	link, exists := m.links[linkID]
	if !exists {
		return nil, ErrShareNotFound
	}

	return link, nil
}

// ValidateAccess 验证访问（密码+过期）
func (m *Manager) ValidateAccess(ctx context.Context, linkID, password string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.links[linkID]
	if !exists {
		return false, ErrShareNotFound
	}

	// 检查是否激活
	if !link.IsActive {
		return false, ErrShareInactive
	}

	// 检查是否过期
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return false, ErrShareExpired
	}

	// 检查下载次数限制
	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		return false, ErrMaxDownloadsExceeded
	}

	// 检查密码
	if link.Password != "" {
		if password == "" {
			return false, ErrPasswordRequired
		}
		if link.Password != password {
			return false, ErrPasswordWrong
		}
	}

	return true, nil
}

// RecordAccess 记录访问
func (m *Manager) RecordAccess(ctx context.Context, access ShareAccess) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, exists := m.links[access.ShareLinkID]
	if !exists {
		return ErrShareNotFound
	}

	if access.ID == "" {
		access.ID = uuid.New().String()
	}
	if access.CreatedAt.IsZero() {
		access.CreatedAt = time.Now()
	}

	m.accesses[access.ShareLinkID] = append(m.accesses[access.ShareLinkID], &access)

	// 更新计数器
	switch access.Action {
	case ActionView:
		link.ViewCount++
	case ActionDownload:
		link.DownloadCount++
	}

	return nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats(ctx context.Context, linkID string) (*ShareStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.links[linkID]
	if !exists {
		return nil, ErrShareNotFound
	}

	accesses := m.accesses[linkID]

	// 统计唯一访客
	uniqueIPs := make(map[string]bool)
	fileDownloads := make(map[string]int)
	fileLastDownload := make(map[string]time.Time)
	dailyViews := make(map[string]int)
	dailyDownloads := make(map[string]int)
	dailyUnique := make(map[string]map[string]bool)

	for _, acc := range accesses {
		uniqueIPs[acc.VisitorIP] = true

		date := acc.CreatedAt.Format("2006-01-02")

		switch acc.Action {
		case ActionView:
			dailyViews[date]++
		case ActionDownload:
			dailyDownloads[date]++
			if acc.FileName != "" {
				fileDownloads[acc.FileName]++
				if acc.CreatedAt.After(fileLastDownload[acc.FileName]) {
					fileLastDownload[acc.FileName] = acc.CreatedAt
				}
			}
		}

		if dailyUnique[date] == nil {
			dailyUnique[date] = make(map[string]bool)
		}
		dailyUnique[date][acc.VisitorIP] = true
	}

	// 构建 TopFiles
	var topFiles []FileStat
	for fileName, count := range fileDownloads {
		topFiles = append(topFiles, FileStat{
			FileName:       fileName,
			DownloadCount:  count,
			LastDownloaded: fileLastDownload[fileName],
		})
	}

	// 构建 DailyStats
	var dailyStats []DailyStat
	for date := range dailyViews {
		uniqueCount := 0
		if ips, ok := dailyUnique[date]; ok {
			uniqueCount = len(ips)
		}
		dailyStats = append(dailyStats, DailyStat{
			Date:           date,
			Views:          dailyViews[date],
			Downloads:      dailyDownloads[date],
			UniqueVisitors: uniqueCount,
		})
	}

	// 最近访问记录（最多返回 50 条）
	recentAccesses := accesses
	if len(recentAccesses) > 50 {
		recentAccesses = recentAccesses[len(recentAccesses)-50:]
	}
	recentAccess := make([]ShareAccess, len(recentAccesses))
	for i, acc := range recentAccesses {
		recentAccess[i] = *acc
	}

	return &ShareStats{
		ShareLinkID:    linkID,
		TotalViews:     link.ViewCount,
		TotalDownloads: link.DownloadCount,
		UniqueVisitors: len(uniqueIPs),
		TopFiles:       topFiles,
		DailyStats:     dailyStats,
		RecentAccess:   recentAccess,
	}, nil
}

// SetBranding 设置品牌配置
func (m *Manager) SetBranding(ctx context.Context, branding ShareBranding) (*ShareBranding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if branding.ID == "" {
		branding.ID = uuid.New().String()
	}

	// 如果设置为默认，取消其他默认
	if branding.IsDefault {
		for _, b := range m.brandings {
			b.IsDefault = false
		}
	}

	m.brandings[branding.ID] = &branding
	return &branding, nil
}

// GetBranding 获取品牌配置
func (m *Manager) GetBranding(ctx context.Context, brandingID string) (*ShareBranding, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	branding, exists := m.brandings[brandingID]
	if !exists {
		return nil, ErrBrandingNotFound
	}

	return branding, nil
}

// CreatePortal 创建门户
func (m *Manager) CreatePortal(ctx context.Context, portal SharePortal) (*SharePortal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if portal.ID == "" {
		portal.ID = uuid.New().String()
	}

	portal.CreatedAt = time.Now()
	m.portals[portal.ID] = &portal

	return &portal, nil
}

// GenerateShortURL 生成短链
func (m *Manager) GenerateShortURL(ctx context.Context) string {
	return m.generateShortURL()
}

// generateShortURL 内部生成短链
func (m *Manager) generateShortURL() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
