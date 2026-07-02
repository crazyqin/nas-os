// Package smartshare 提供智能文件分享系统核心管理逻辑
package smartshare

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 分享链接管理器.
type Manager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	policy        *SharePolicy
	links         map[string]*ShareLink // ID -> ShareLink
	linksByToken  map[string]*ShareLink // Token -> ShareLink
	accessLogs    []*AccessLog
	notifyConfig  *NotifyConfig
	previewConfig *PreviewConfig
	notifyEvents  []*NotifyEvent
	generator     *Generator
	accessCtrl    *AccessController
	analytics     *AnalyticsEngine
	watermark     *WatermarkEngine
	preview       *PreviewEngine
	notifier      *Notifier
	branding      *BrandingEngine
	stopChan      chan struct{}
	running       bool
}

// NewManager 创建分享链接管理器.
func NewManager(logger *zap.Logger, policy *SharePolicy) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if policy == nil {
		policy = DefaultSharePolicy()
	}

	m := &Manager{
		logger:        logger,
		policy:        policy,
		links:         make(map[string]*ShareLink),
		linksByToken:  make(map[string]*ShareLink),
		accessLogs:    make([]*AccessLog, 0),
		notifyConfig:  DefaultNotifyConfig(),
		previewConfig: DefaultPreviewConfig(),
		notifyEvents:  make([]*NotifyEvent, 0),
		stopChan:      make(chan struct{}),
	}

	m.generator = NewGenerator(m.logger)
	m.accessCtrl = NewAccessController(m.logger)
	m.analytics = NewAnalyticsEngine(m.logger)
	m.watermark = NewWatermarkEngine(m.logger)
	m.preview = NewPreviewEngine(m.logger, m.previewConfig)
	m.notifier = NewNotifier(m.logger, m.notifyConfig)
	m.branding = NewBrandingEngine(m.logger)

	return m
}

// Start 启动管理器后台任务.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.cleanupLoop(ctx)

	m.logger.Info("smartshare manager started")
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopChan)
	m.running = false
	m.logger.Info("smartshare manager stopped")
}

// cleanupLoop 定期清理过期链接.
func (m *Manager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.cleanupExpiredLinks()
		}
	}
}

// cleanupExpiredLinks 清理过期链接.
func (m *Manager) cleanupExpiredLinks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	expired := 0

	for id, link := range m.links {
		if link.Status != ShareStatusActive {
			continue
		}

		// 检查过期时间
		if link.ExpiresAt != nil && now.After(*link.ExpiresAt) {
			link.Status = ShareStatusExpired
			link.UpdatedAt = now
			expired++

			if m.notifyConfig.OnExpired {
				m.notifier.SendEvent(&NotifyEvent{
					ID:        generateID(),
					ShareID:   id,
					EventType: "expired",
					Level:     AlertLevelInfo,
					Title:     "分享链接已过期",
					Message:   fmt.Sprintf("文件 %s 的分享链接已过期", link.FileName),
					Timestamp: now,
				})
			}
		}

		// 检查下载次数
		if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
			link.Status = ShareStatusExhausted
			link.UpdatedAt = now
			expired++
		}
	}

	if expired > 0 {
		m.logger.Info("cleaned up expired share links", zap.Int("count", expired))
	}
}

// CreateShareLink 创建分享链接.
func (m *Manager) CreateShareLink(req *CreateShareRequest) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证文件大小
	if m.policy.MaxFileSize > 0 && req.FileSize > m.policy.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed %d", req.FileSize, m.policy.MaxFileSize)
	}

	// 验证文件类型
	if len(m.policy.AllowedFileTypes) > 0 {
		allowed := false
		for _, t := range m.policy.AllowedFileTypes {
			if t == req.FileType {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("file type %s is not allowed", req.FileType)
		}
	}

	// 设置默认模式
	mode := req.Mode
	if mode == "" {
		mode = m.policy.DefaultMode
	}

	// 验证密码模式
	if mode == ShareModePassword && m.policy.RequirePassword && req.Password == "" {
		return nil, fmt.Errorf("password is required for password-protected shares")
	}

	// 生成链接
	token := m.generator.GenerateToken()
	shortCode := m.generator.GenerateShortCode()
	now := time.Now()

	// 计算过期时间
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		if m.policy.MaxExpiration > 0 && req.ExpiresIn > m.policy.MaxExpiration {
			return nil, fmt.Errorf("expiration %v exceeds maximum allowed %v", req.ExpiresIn, m.policy.MaxExpiration)
		}
		t := now.Add(req.ExpiresIn)
		expiresAt = &t
	} else if m.policy.DefaultExpiration > 0 {
		t := now.Add(m.policy.DefaultExpiration)
		expiresAt = &t
	}

	// 设置下载限制
	maxDownloads := req.MaxDownloads
	if maxDownloads == 0 && m.policy.MaxDownloads > 0 {
		maxDownloads = m.policy.MaxDownloads
	}

	link := &ShareLink{
		ID:              generateID(),
		Token:           token,
		ShortURL:        fmt.Sprintf("/s/%s", shortCode),
		FullURL:         fmt.Sprintf("/share/%s", token),
		FilePath:        req.FilePath,
		FileName:        req.FileName,
		FileSize:        req.FileSize,
		FileType:        req.FileType,
		Mode:            mode,
		Status:          ShareStatusActive,
		CreatorID:       req.CreatorID,
		CreatorName:     req.CreatorName,
		Description:     req.Description,
		Tags:            req.Tags,
		Password:        req.Password,
		ExpiresAt:       expiresAt,
		MaxDownloads:    maxDownloads,
		DownloadCount:   0,
		ViewCount:       0,
		AllowedUsers:    req.AllowedUsers,
		IPWhitelist:     req.IPWhitelist,
		EnableWatermark: req.EnableWatermark,
		EnablePreview:   req.EnablePreview,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// 设置水印配置
	if req.EnableWatermark {
		if req.WatermarkConfig != nil {
			link.WatermarkConfig = req.WatermarkConfig
		} else {
			link.WatermarkConfig = DefaultWatermarkConfig()
		}
	}

	// 设置品牌配置
	if req.BrandingConfig != nil {
		link.BrandingConfig = req.BrandingConfig
	} else {
		link.BrandingConfig = DefaultBrandingConfig()
	}

	m.links[link.ID] = link
	m.linksByToken[token] = link

	m.logger.Info("share link created",
		zap.String("id", link.ID),
		zap.String("file", link.FileName),
		zap.String("mode", string(link.Mode)))

	return link, nil
}

// CreateShareRequest 创建分享请求.
type CreateShareRequest struct {
	FilePath        string           `json:"file_path" binding:"required"`
	FileName        string           `json:"file_name" binding:"required"`
	FileSize        int64            `json:"file_size"`
	FileType        string           `json:"file_type"`
	Mode            ShareMode        `json:"mode,omitempty"`
	Password        string           `json:"password,omitempty"`
	ExpiresIn       time.Duration    `json:"expires_in,omitempty"`
	MaxDownloads    int              `json:"max_downloads,omitempty"`
	CreatorID       string           `json:"creator_id"`
	CreatorName     string           `json:"creator_name"`
	Description     string           `json:"description,omitempty"`
	Tags            []string         `json:"tags,omitempty"`
	AllowedUsers    []string         `json:"allowed_users,omitempty"`
	IPWhitelist     []string         `json:"ip_whitelist,omitempty"`
	EnableWatermark bool             `json:"enable_watermark"`
	WatermarkConfig *WatermarkConfig `json:"watermark_config,omitempty"`
	EnablePreview   bool             `json:"enable_preview"`
	BrandingConfig  *BrandingConfig  `json:"branding_config,omitempty"`
}

// GetShareLink 根据 ID 获取分享链接.
func (m *Manager) GetShareLink(id string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[id]
	if !ok {
		return nil, fmt.Errorf("share link not found: %s", id)
	}
	return link, nil
}

// GetShareLinkByToken 根据 Token 获取分享链接.
func (m *Manager) GetShareLinkByToken(token string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.linksByToken[token]
	if !ok {
		return nil, fmt.Errorf("share link not found")
	}
	return link, nil
}

// ListShareLinks 列出所有分享链接.
func (m *Manager) ListShareLinks(filter *ListFilter) []*ShareLink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ShareLink, 0)
	for _, link := range m.links {
		if filter != nil {
			if filter.CreatorID != "" && link.CreatorID != filter.CreatorID {
				continue
			}
			if filter.Status != "" && link.Status != filter.Status {
				continue
			}
			if filter.Mode != "" && link.Mode != filter.Mode {
				continue
			}
		}
		result = append(result, link)
	}
	return result
}

// ListFilter 列表过滤器.
type ListFilter struct {
	CreatorID string      `json:"creator_id,omitempty"`
	Status    ShareStatus `json:"status,omitempty"`
	Mode      ShareMode   `json:"mode,omitempty"`
}

// UpdateShareLink 更新分享链接.
func (m *Manager) UpdateShareLink(id string, req *UpdateShareRequest) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return nil, fmt.Errorf("share link not found: %s", id)
	}

	if req.Password != nil {
		link.Password = *req.Password
	}
	if req.ExpiresIn != nil {
		t := time.Now().Add(*req.ExpiresIn)
		link.ExpiresAt = &t
	}
	if req.MaxDownloads != nil {
		link.MaxDownloads = *req.MaxDownloads
	}
	if req.Description != nil {
		link.Description = *req.Description
	}
	if req.Tags != nil {
		link.Tags = req.Tags
	}
	if req.IPWhitelist != nil {
		link.IPWhitelist = req.IPWhitelist
	}
	if req.EnableWatermark != nil {
		link.EnableWatermark = *req.EnableWatermark
	}
	if req.WatermarkConfig != nil {
		link.WatermarkConfig = req.WatermarkConfig
	}
	if req.BrandingConfig != nil {
		link.BrandingConfig = req.BrandingConfig
	}

	link.UpdatedAt = time.Now()
	return link, nil
}

// UpdateShareRequest 更新分享请求.
type UpdateShareRequest struct {
	Password        *string          `json:"password,omitempty"`
	ExpiresIn       *time.Duration   `json:"expires_in,omitempty"`
	MaxDownloads    *int             `json:"max_downloads,omitempty"`
	Description     *string          `json:"description,omitempty"`
	Tags            []string         `json:"tags,omitempty"`
	IPWhitelist     []string         `json:"ip_whitelist,omitempty"`
	EnableWatermark *bool            `json:"enable_watermark,omitempty"`
	WatermarkConfig *WatermarkConfig `json:"watermark_config,omitempty"`
	BrandingConfig  *BrandingConfig  `json:"branding_config,omitempty"`
}

// RevokeShareLink 撤销分享链接.
func (m *Manager) RevokeShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return fmt.Errorf("share link not found: %s", id)
	}

	link.Status = ShareStatusRevoked
	link.UpdatedAt = time.Now()

	m.logger.Info("share link revoked", zap.String("id", id))
	return nil
}

// DeleteShareLink 删除分享链接.
func (m *Manager) DeleteShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return fmt.Errorf("share link not found: %s", id)
	}

	delete(m.links, id)
	delete(m.linksByToken, link.Token)

	m.logger.Info("share link deleted", zap.String("id", id))
	return nil
}

// BatchCreateShareLinks 批量创建分享链接.
func (m *Manager) BatchCreateShareLinks(req *BatchShareRequest) *BatchShareResult {
	result := &BatchShareResult{
		Total: len(req.FilePaths),
	}

	for _, filePath := range req.FilePaths {
		createReq := &CreateShareRequest{
			FilePath:     filePath,
			FileName:     extractFileName(filePath),
			Mode:         req.Mode,
			Password:     req.Password,
			ExpiresIn:    req.ExpiresIn,
			MaxDownloads: req.MaxDownloads,
			Tags:         req.Tags,
		}

		link, err := m.CreateShareLink(createReq)
		if err != nil {
			result.Failed = append(result.Failed, BatchError{
				FilePath: filePath,
				Error:    err.Error(),
			})
		} else {
			result.Success = append(result.Success, link)
		}
	}

	return result
}

// GetPolicy 获取分享策略.
func (m *Manager) GetPolicy() *SharePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p := *m.policy
	return &p
}

// UpdatePolicy 更新分享策略.
func (m *Manager) UpdatePolicy(policy *SharePolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.UpdatedAt = time.Now()
	m.policy = policy
}

// GetNotifyConfig 获取通知配置.
func (m *Manager) GetNotifyConfig() *NotifyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.notifyConfig
	return &cfg
}

// UpdateNotifyConfig 更新通知配置.
func (m *Manager) UpdateNotifyConfig(cfg *NotifyConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyConfig = cfg
	m.notifier.UpdateConfig(cfg)
}

// GetPreviewConfig 获取预览配置.
func (m *Manager) GetPreviewConfig() *PreviewConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.previewConfig
	return &cfg
}

// UpdatePreviewConfig 更新预览配置.
func (m *Manager) UpdatePreviewConfig(cfg *PreviewConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.previewConfig = cfg
	m.preview.UpdateConfig(cfg)
}

// GetNotifyEvents 获取通知事件列表.
func (m *Manager) GetNotifyEvents(limit int) []*NotifyEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.notifyEvents) {
		limit = len(m.notifyEvents)
	}

	start := len(m.notifyEvents) - limit
	if start < 0 {
		start = 0
	}

	events := make([]*NotifyEvent, limit)
	copy(events, m.notifyEvents[start:])
	return events
}

// GetStats 获取总览统计.
func (m *Manager) GetStats() *ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ManagerStats{
		TotalLinks:      len(m.links),
		ActiveLinks:     0,
		ExpiredLinks:    0,
		RevokedLinks:    0,
		TotalViews:      0,
		TotalDownloads:  0,
		TotalAccessLogs: len(m.accessLogs),
	}

	for _, link := range m.links {
		switch link.Status {
		case ShareStatusActive:
			stats.ActiveLinks++
		case ShareStatusExpired, ShareStatusExhausted:
			stats.ExpiredLinks++
		case ShareStatusRevoked:
			stats.RevokedLinks++
		}
		stats.TotalViews += link.ViewCount
		stats.TotalDownloads += link.DownloadCount
	}

	return stats
}

// ManagerStats 管理器统计.
type ManagerStats struct {
	TotalLinks      int `json:"total_links"`
	ActiveLinks     int `json:"active_links"`
	ExpiredLinks    int `json:"expired_links"`
	RevokedLinks    int `json:"revoked_links"`
	TotalViews      int `json:"total_views"`
	TotalDownloads  int `json:"total_downloads"`
	TotalAccessLogs int `json:"total_access_logs"`
}

// extractFileName 从路径中提取文件名.
func extractFileName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
