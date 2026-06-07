package sharelinks

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Base62字符集
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// LinkManager 共享链接管理器
type LinkManager struct {
	mu         sync.RWMutex
	links      map[string]*ShareLink
	shortCodes map[string]*ShareLink // 短码 -> 链接映射
	config     *LinkConfig
}

// NewLinkManager 创建管理器
func NewLinkManager(config *LinkConfig) *LinkManager {
	if config == nil {
		config = DefaultConfig()
	}
	return &LinkManager{
		links:      make(map[string]*ShareLink),
		shortCodes: make(map[string]*ShareLink),
		config:     config,
	}
}

// CreateLink 创建共享链接
func (m *LinkManager) CreateLink(path, name, createdBy string, linkType LinkType, opts ...LinkOption) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if path == "" {
		return nil, ErrInvalidPath
	}

	id := generateID()
	token := generateToken()
	shortCode, err := m.generateShortCode()
	if err != nil {
		return nil, fmt.Errorf("generate short code: %w", err)
	}

	link := &ShareLink{
		ID:         id,
		ShortCode:  shortCode,
		Path:       path,
		Name:       name,
		Type:       linkType,
		Token:      token,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		IsActive:   true,
		AccessLog:  make([]AccessEntry, 0),
		Tags:       make([]string, 0),
		BatchPaths: make([]string, 0),
	}

	// 应用默认过期时间
	if m.config.DefaultExpiryHours > 0 {
		t := time.Now().Add(time.Duration(m.config.DefaultExpiryHours) * time.Hour)
		link.ExpiresAt = &t
	}

	// 应用选项
	for _, opt := range opts {
		opt(link)
	}

	// 设置预览类型
	link.PreviewType = detectPreviewType(path)

	m.links[id] = link
	m.shortCodes[shortCode] = link

	return link, nil
}

// LinkOption 链接选项函数
type LinkOption func(*ShareLink)

// WithPassword 设置密码
func WithPassword(password string) LinkOption {
	return func(l *ShareLink) {
		if password != "" {
			hash := sha256.Sum256([]byte(password))
			l.Password = hex.EncodeToString(hash[:])
		}
	}
}

// WithExpiry 设置过期时间（小时）
func WithExpiry(hours int) LinkOption {
	return func(l *ShareLink) {
		if hours > 0 {
			t := time.Now().Add(time.Duration(hours) * time.Hour)
			l.ExpiresAt = &t
		}
	}
}

// WithMaxDownloads 设置最大下载次数
func WithMaxDownloads(max int) LinkOption {
	return func(l *ShareLink) {
		l.MaxDownloads = max
	}
}

// WithDescription 设置描述
func WithDescription(desc string) LinkOption {
	return func(l *ShareLink) {
		l.Description = desc
	}
}

// WithTags 设置标签
func WithTags(tags []string) LinkOption {
	return func(l *ShareLink) {
		l.Tags = tags
	}
}

// WithRefererWhitelist 设置Referer白名单
func WithRefererWhitelist(referers []string) LinkOption {
	return func(l *ShareLink) {
		l.RefererWhitelist = referers
	}
}

// WithBatchPaths 设置批量路径
func WithBatchPaths(paths []string) LinkOption {
	return func(l *ShareLink) {
		if len(paths) > 0 {
			l.IsBatch = true
			l.BatchPaths = paths
		}
	}
}

// GetLink 获取链接
func (m *LinkManager) GetLink(id string) (*ShareLink, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	link, ok := m.links[id]
	return link, ok
}

// GetLinkByShortCode 通过短码获取链接
func (m *LinkManager) GetLinkByShortCode(shortCode string) (*ShareLink, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	link, ok := m.shortCodes[shortCode]
	return link, ok
}

// GetLinkByToken 通过Token获取链接
func (m *LinkManager) GetLinkByToken(token string) (*ShareLink, bool) {
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

// ListLinks 列出链接
func (m *LinkManager) ListLinks(createdBy string, activeOnly bool) []*ShareLink {
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

// UpdateLink 更新链接
func (m *LinkManager) UpdateLink(id string, opts ...LinkOption) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return nil, ErrLinkNotFound
	}

	for _, opt := range opts {
		opt(link)
	}
	link.UpdatedAt = time.Now()

	return link, nil
}

// DisableLink 禁用链接
func (m *LinkManager) DisableLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return ErrLinkNotFound
	}
	link.IsActive = false
	link.UpdatedAt = time.Now()
	return nil
}

// EnableLink 启用链接
func (m *LinkManager) EnableLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return ErrLinkNotFound
	}
	link.IsActive = true
	link.UpdatedAt = time.Now()
	return nil
}

// DeleteLink 删除链接
func (m *LinkManager) DeleteLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return ErrLinkNotFound
	}

	delete(m.shortCodes, link.ShortCode)
	delete(m.links, id)
	return nil
}

// ValidateAccess 验证访问权限
func (m *LinkManager) ValidateAccess(id, password, referer string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[id]
	if !ok {
		return nil, ErrLinkNotFound
	}

	if !link.IsActive {
		return nil, ErrLinkDisabled
	}

	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return nil, ErrLinkExpired
	}

	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		return nil, ErrDownloadLimit
	}

	// 验证密码
	if link.Type == LinkTypeEncrypted && link.Password != "" {
		if password == "" {
			return nil, ErrInvalidPassword
		}
		hash := sha256.Sum256([]byte(password))
		if hex.EncodeToString(hash[:]) != link.Password {
			return nil, ErrInvalidPassword
		}
	}

	// 验证Referer
	if len(link.RefererWhitelist) > 0 && referer != "" {
		allowed := false
		for _, allowedReferer := range link.RefererWhitelist {
			if strings.HasPrefix(referer, allowedReferer) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ErrRefererDenied
		}
	}

	return link, nil
}

// ValidateAccessByShortCode 通过短码验证访问
func (m *LinkManager) ValidateAccessByShortCode(shortCode, password, referer string) (*ShareLink, error) {
	m.mu.RLock()
	link, ok := m.shortCodes[shortCode]
	if !ok {
		return nil, ErrInvalidShortCode
	}
	id := link.ID
	m.mu.RUnlock()

	return m.ValidateAccess(id, password, referer)
}

// RecordAccess 记录访问
func (m *LinkManager) RecordAccess(id, ip, userAgent, referer, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return
	}

	entry := AccessEntry{
		IP:        ip,
		UserAgent: userAgent,
		Referer:   referer,
		Action:    action,
		Timestamp: time.Now(),
	}
	link.AccessLog = append(link.AccessLog, entry)

	// 更新统计
	if action == "download" {
		link.DownloadCount++
	}

	// 更新独立访客数
	visitorIPs := make(map[string]bool)
	for _, log := range link.AccessLog {
		visitorIPs[log.IP] = true
	}
	link.UniqueVisitors = len(visitorIPs)

	// 更新最后访问时间
	link.LastAccessedAt = &entry.Timestamp
}

// GetLinkStats 获取链接统计
func (m *LinkManager) GetLinkStats(id string) (*ShareStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[id]
	if !ok {
		return nil, ErrLinkNotFound
	}

	stats := &ShareStats{
		TotalLinks:     1,
		TotalDownloads: int64(link.DownloadCount),
		TotalViews:     int64(len(link.AccessLog)),
	}

	if link.IsActive {
		if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
			stats.ExpiredLinks = 1
		} else {
			stats.ActiveLinks = 1
		}
	} else {
		stats.DisabledLinks = 1
	}

	// 统计预览次数
	for _, log := range link.AccessLog {
		if log.Action == "preview" {
			stats.TotalPreviews++
		}
	}

	return stats, nil
}

// GetGlobalStats 获取全局统计
func (m *LinkManager) GetGlobalStats() *ShareStats {
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
		} else {
			stats.DisabledLinks++
		}
		stats.TotalDownloads += int64(link.DownloadCount)
		stats.TotalViews += int64(len(link.AccessLog))
		for _, log := range link.AccessLog {
			if log.Action == "preview" {
				stats.TotalPreviews++
			}
		}
	}
	return stats
}

// CleanupExpired 清理过期链接
func (m *LinkManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, link := range m.links {
		if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) && link.IsActive {
			link.IsActive = false
			link.UpdatedAt = time.Now()
			count++
		}
		_ = id
	}
	return count
}

// GenerateQRCodeData 生成二维码数据
func (m *LinkManager) GenerateQRCodeData(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[id]
	if !ok {
		return "", ErrLinkNotFound
	}

	if m.config.BaseURL != "" {
		return fmt.Sprintf("%s/s/%s", m.config.BaseURL, link.ShortCode), nil
	}
	return fmt.Sprintf("/s/%s", link.ShortCode), nil
}

// generateShortCode 生成短码
func (m *LinkManager) generateShortCode() (string, error) {
	length := m.config.ShortCodeLength
	if length == 0 {
		length = 6
	}

	for attempts := 0; attempts < 100; attempts++ {
		code, err := encodeBase62(generateRandomBytes(length))
		if err != nil {
			return "", err
		}
		if _, exists := m.shortCodes[code]; !exists {
			return code, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique short code")
}

// detectPreviewType 检测预览类型
func detectPreviewType(path string) PreviewType {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".webp") || strings.HasSuffix(lower, ".svg"):
		return PreviewTypeImage
	case strings.HasSuffix(lower, ".pdf") || strings.HasSuffix(lower, ".doc") ||
		strings.HasSuffix(lower, ".docx") || strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".md"):
		return PreviewTypeDocument
	case strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") ||
		strings.HasSuffix(lower, ".avi") || strings.HasSuffix(lower, ".mov"):
		return PreviewTypeVideo
	case strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".wav") ||
		strings.HasSuffix(lower, ".ogg") || strings.HasSuffix(lower, ".flac"):
		return PreviewTypeAudio
	default:
		return PreviewTypeNone
	}
}

// 工具函数
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

// encodeBase62 Base62编码
func encodeBase62(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	// 将字节数组转换为大整数
	num := uint64(0)
	for i, b := range data {
		if i >= 8 {
			break
		}
		num = num<<8 | uint64(b)
	}

	// Base62编码
	var result strings.Builder
	for num > 0 {
		result.WriteByte(base62Chars[num%62])
		num /= 62
	}

	// 反转结果
	runes := []rune(result.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes), nil
}
