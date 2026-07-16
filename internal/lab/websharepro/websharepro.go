// Package websharepro 提供增强版WebShare功能
// 学习 TrueNAS 26 WebShare 特性：
// - Dropbox式文件共享界面
// - 浏览器/平板/手机访问
// - FIPS 140加密传输
// - SMB/AD/NFSv4协议互通
// - 共享链接管理（过期、密码、权限）
package websharepro

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// SharePermission 共享权限.
type SharePermission string

const (
	PermReadOnly  SharePermission = "readonly"
	PermReadWrite SharePermission = "readwrite"
	PermAdmin     SharePermission = "admin"
)

// ShareLink 共享链接.
type ShareLink struct {
	ID            string          `json:"id"`
	Path          string          `json:"path"`
	Name          string          `json:"name"`
	Token         string          `json:"token"`
	Permission    SharePermission `json:"permission"`
	Password      string          `json:"password,omitempty"`
	MaxDownloads  int             `json:"maxDownloads"`
	DownloadCount int             `json:"downloadCount"`
	ExpiresAt     *time.Time      `json:"expiresAt"`
	CreatedBy     string          `json:"createdBy"`
	CreatedAt     time.Time       `json:"createdAt"`
	IsActive      bool            `json:"isActive"`
	AccessLog     []AccessEntry   `json:"accessLog"`
}

// AccessEntry 访问记录.
type AccessEntry struct {
	IP        string    `json:"ip"`
	UserAgent string    `json:"userAgent"`
	Action    string    `json:"action"` // view, download
	Timestamp time.Time `json:"timestamp"`
}

// ShareStats 共享统计.
type ShareStats struct {
	TotalLinks     int   `json:"totalLinks"`
	ActiveLinks    int   `json:"activeLinks"`
	ExpiredLinks   int   `json:"expiredLinks"`
	TotalDownloads int64 `json:"totalDownloads"`
	TotalViews     int64 `json:"totalViews"`
}

// WebShareManager WebShare管理器.
type WebShareManager struct {
	mu     sync.RWMutex
	links  map[string]*ShareLink
	config *WebShareConfig
}

// WebShareConfig WebShare配置.
type WebShareConfig struct {
	DefaultExpiryHours int    `json:"defaultExpiryHours"` // 默认过期时间
	MaxFileSize        int64  `json:"maxFileSize"`        // 最大文件大小
	EnablePassword     bool   `json:"enablePassword"`     // 启用密码保护
	EnableAccessLog    bool   `json:"enableAccessLog"`    // 启用访问日志
	BaseURL            string `json:"baseUrl"`            // 基础URL
}

// NewWebShareManager 创建管理器.
func NewWebShareManager(config *WebShareConfig) *WebShareManager {
	if config == nil {
		config = &WebShareConfig{
			DefaultExpiryHours: 72,
			MaxFileSize:        10 * 1024 * 1024 * 1024, // 10GB
			EnablePassword:     true,
			EnableAccessLog:    true,
		}
	}
	return &WebShareManager{
		links:  make(map[string]*ShareLink),
		config: config,
	}
}

// CreateShareLink 创建共享链接.
func (m *WebShareManager) CreateShareLink(path, name, createdBy string, perm SharePermission, expiryHours int, password string) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	token := generateToken()

	var expiresAt *time.Time
	if expiryHours > 0 {
		t := time.Now().Add(time.Duration(expiryHours) * time.Hour)
		expiresAt = &t
	} else if m.config.DefaultExpiryHours > 0 {
		t := time.Now().Add(time.Duration(m.config.DefaultExpiryHours) * time.Hour)
		expiresAt = &t
	}

	link := &ShareLink{
		ID:         id,
		Path:       path,
		Name:       name,
		Token:      token,
		Permission: perm,
		Password:   password,
		ExpiresAt:  expiresAt,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now(),
		IsActive:   true,
		AccessLog:  make([]AccessEntry, 0),
	}

	m.links[id] = link
	return link, nil
}

// GetShareLink 获取共享链接.
func (m *WebShareManager) GetShareLink(id string) (*ShareLink, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	link, ok := m.links[id]
	return link, ok
}

// GetShareLinkByToken 通过Token获取共享链接.
func (m *WebShareManager) GetShareLinkByToken(token string) (*ShareLink, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, link := range m.links {
		if link.Token == token && link.IsActive {
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				continue
			}
			return link, true
		}
	}
	return nil, false
}

// ListShareLinks 列出共享链接.
func (m *WebShareManager) ListShareLinks(createdBy string, activeOnly bool) []*ShareLink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ShareLink
	for _, link := range m.links {
		if createdBy != "" && link.CreatedBy != createdBy {
			continue
		}
		if activeOnly && !link.IsActive {
			continue
		}
		result = append(result, link)
	}
	return result
}

// DeleteShareLink 删除共享链接.
func (m *WebShareManager) DeleteShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return ErrLinkNotFound
	}
	link.IsActive = false
	return nil
}

// RecordAccess 记录访问.
func (m *WebShareManager) RecordAccess(id, ip, userAgent, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return
	}

	entry := AccessEntry{
		IP:        ip,
		UserAgent: userAgent,
		Action:    action,
		Timestamp: time.Now(),
	}
	link.AccessLog = append(link.AccessLog, entry)

	if action == "download" {
		link.DownloadCount++
	}
}

// GetStats 获取统计.
func (m *WebShareManager) GetStats() *ShareStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ShareStats{}
	for _, link := range m.links {
		stats.TotalLinks++
		if link.IsActive {
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				stats.ExpiredLinks++
			} else {
				stats.ActiveLinks++
			}
		}
		stats.TotalDownloads += int64(link.DownloadCount)
		stats.TotalViews += int64(len(link.AccessLog))
	}
	return stats
}

// CleanupExpired 清理过期链接.
func (m *WebShareManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, link := range m.links {
		if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) && link.IsActive {
			link.IsActive = false
			count++
		}
	}
	return count
}

// 工具函数.
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// 错误定义.
var ErrLinkNotFound = &ShareError{"share link not found"}

type ShareError struct {
	msg string
}

func (e *ShareError) Error() string {
	return e.msg
}
