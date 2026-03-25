// Package directplay provides direct link playback for cloud drives
package directplay

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager 直链播放管理器
type Manager struct {
	config    *DirectPlayConfig
	cache     *LinkCache
	providers map[ProviderType]DirectPlayProvider
	sessions  map[string]*StreamSession
	mu        sync.RWMutex
	limiter   chan struct{} // 并发限制
}

// DirectPlayProvider 直链播放提供商接口
type DirectPlayProvider interface {
	// GetType 获取提供商类型
	GetType() ProviderType

	// GetDirectLink 获取直链
	GetDirectLink(ctx context.Context, req *DirectPlayRequest) (*DirectLinkInfo, error)

	// ListFiles 列出文件
	ListFiles(ctx context.Context, req *ListFilesRequest) (*ListFilesResponse, error)

	// GetFileInfo 获取文件信息
	GetFileInfo(ctx context.Context, provider ProviderType, fileID, accessToken string) (*FileInfo, error)

	// TestConnection 测试连接
	TestConnection(ctx context.Context, accessToken, refreshToken string) (*ProviderInfo, error)

	// SetAccessToken 设置访问令牌
	SetAccessToken(accessToken, refreshToken, driveID string)
}

// NewManager 创建直链播放管理器
func NewManager(config *DirectPlayConfig) *Manager {
	if config == nil {
		config = DefaultDirectPlayConfig()
	}

	m := &Manager{
		config:    config,
		cache:     NewLinkCache(config.CacheTTL, config.CacheMaxItems),
		providers: make(map[ProviderType]DirectPlayProvider),
		sessions:  make(map[string]*StreamSession),
		limiter:   make(chan struct{}, config.MaxConcurrent),
	}

	// 初始化提供商
	if config.BaiduPanEnabled {
		m.providers[ProviderBaiduPan] = NewBaiduPanProvider()
	}
	if config.Pan123Enabled {
		m.providers[Provider123Pan] = New123PanProvider()
	}
	if config.AliyunPanEnabled {
		m.providers[ProviderAliyunPan] = NewAliyunPanProvider()
	}

	return m
}

// GetDirectLink 获取直链
func (m *Manager) GetDirectLink(ctx context.Context, req *DirectPlayRequest) (*DirectLinkInfo, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("直链播放功能已禁用")
	}

	provider, ok := m.providers[req.Provider]
	if !ok {
		return nil, fmt.Errorf("不支持的网盘类型: %s", req.Provider)
	}

	// 检查缓存
	cacheKey := m.getCacheKey(req)
	if m.config.CacheEnabled && !req.ForceRefresh {
		if cached, ok := m.cache.Get(cacheKey); ok {
			// 检查是否快过期
			if time.Until(cached.ExpiresAt) > m.config.LinkExpireMin {
				cached.Cached = true
				return cached, nil
			}
		}
	}

	// 并发限制
	select {
	case m.limiter <- struct{}{}:
		defer func() { <-m.limiter }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 获取直链
	link, err := provider.GetDirectLink(ctx, req)
	if err != nil {
		return nil, err
	}

	link.Provider = req.Provider

	// 缓存结果
	if m.config.CacheEnabled {
		m.cache.Set(cacheKey, link)
	}

	return link, nil
}

// ListFiles 列出文件
func (m *Manager) ListFiles(ctx context.Context, req *ListFilesRequest) (*ListFilesResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("直链播放功能已禁用")
	}

	provider, ok := m.providers[req.Provider]
	if !ok {
		return nil, fmt.Errorf("不支持的网盘类型: %s", req.Provider)
	}

	return provider.ListFiles(ctx, req)
}

// GetFileInfo 获取文件信息
func (m *Manager) GetFileInfo(ctx context.Context, provider ProviderType, fileID, accessToken string) (*FileInfo, error) {
	p, ok := m.providers[provider]
	if !ok {
		return nil, fmt.Errorf("不支持的网盘类型: %s", provider)
	}

	return p.GetFileInfo(ctx, provider, fileID, accessToken)
}

// TestConnection 测试连接
func (m *Manager) TestConnection(ctx context.Context, provider ProviderType, accessToken, refreshToken string) (*ProviderInfo, error) {
	p, ok := m.providers[provider]
	if !ok {
		return nil, fmt.Errorf("不支持的网盘类型: %s", provider)
	}

	return p.TestConnection(ctx, accessToken, refreshToken)
}

// SetProviderTokens 设置网盘令牌
func (m *Manager) SetProviderTokens(provider ProviderType, accessToken, refreshToken, driveID string) error {
	p, ok := m.providers[provider]
	if !ok {
		return fmt.Errorf("不支持的网盘类型: %s", provider)
	}

	p.SetAccessToken(accessToken, refreshToken, driveID)
	return nil
}

// CreateStreamSession 创建流媒体会话
func (m *Manager) CreateStreamSession(ctx context.Context, req *DirectPlayRequest) (*StreamSession, error) {
	link, err := m.GetDirectLink(ctx, req)
	if err != nil {
		return nil, err
	}

	session := &StreamSession{
		ID:         fmt.Sprintf("stream_%d", time.Now().UnixNano()),
		Provider:   req.Provider,
		FileID:     req.FileID,
		FileName:   link.FileName,
		DirectLink: link,
		StartTime:  time.Now(),
		LastAccess: time.Now(),
		Viewers:    1,
	}

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	return session, nil
}

// GetStreamSession 获取流媒体会话
func (m *Manager) GetStreamSession(sessionID string) (*StreamSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	return session, nil
}

// CloseStreamSession 关闭流媒体会话
func (m *Manager) CloseStreamSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}

// GetStatus 获取状态
func (m *Manager) GetStatus() *DirectPlayStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]ProviderInfo, 0, len(m.providers))
	for _, p := range m.providers {
		info := &ProviderInfo{
			Type:    p.GetType(),
			Enabled: true,
		}
		providers = append(providers, *info)
	}

	return &DirectPlayStatus{
		Enabled:       m.config.Enabled,
		Providers:     providers,
		CacheSize:     m.cache.Size(),
		ActiveStreams: len(m.sessions),
	}
}

// ClearCache 清除缓存
func (m *Manager) ClearCache() {
	m.cache.Clear()
}

// CleanupExpiredSessions 清理过期会话
func (m *Manager) CleanupExpiredSessions(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()

	for id, session := range m.sessions {
		if now.Sub(session.LastAccess) > maxAge {
			delete(m.sessions, id)
			count++
		}
	}

	return count
}

// getCacheKey 获取缓存键
func (m *Manager) getCacheKey(req *DirectPlayRequest) string {
	return fmt.Sprintf("%s:%s", req.Provider, req.FileID)
}

// RefreshLink 刷新直链
func (m *Manager) RefreshLink(ctx context.Context, req *DirectPlayRequest) (*DirectLinkInfo, error) {
	req.ForceRefresh = true
	cacheKey := m.getCacheKey(req)
	m.cache.Delete(cacheKey)
	return m.GetDirectLink(ctx, req)
}
