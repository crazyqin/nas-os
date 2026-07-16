// Package webshare 提供业务逻辑服务层
package webshare

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Service WebShare 服务.
type Service struct {
	mu       sync.RWMutex
	config   *WebShareConfig
	shares   map[string]*WebShare       // shareID -> WebShare
	sessions map[string]*BrowserSession // sessionID -> BrowserSession
	tokenIdx map[string]string          // token -> shareID（快速查找）
}

// NewService 创建 WebShare 服务.
func NewService(cfg *WebShareConfig) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Service{
		config:   cfg,
		shares:   make(map[string]*WebShare),
		sessions: make(map[string]*BrowserSession),
		tokenIdx: make(map[string]string),
	}
}

// ========== 分享管理 ==========

// CreateShare 创建 Web 分享.
func (s *Service) CreateShare(ctx context.Context, req *CreateShareRequest) (*WebShare, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("WebShare 功能未启用")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查同名分享是否已存在
	for _, share := range s.shares {
		if share.Name == req.Name && share.Status == ShareStatusActive {
			return nil, fmt.Errorf("分享名称 %q 已存在", req.Name)
		}
	}

	// 设置默认访问模式
	if req.AccessMode == "" {
		req.AccessMode = s.config.DefaultAccessMode
	}

	share := newWebShare(req, s.config)
	s.shares[share.ID] = share
	s.tokenIdx[share.Token] = share.ID

	return share, nil
}

// GetShare 获取分享详情.
func (s *Service) GetShare(ctx context.Context, shareID string) (*WebShare, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	share, ok := s.shares[shareID]
	if !ok {
		return nil, fmt.Errorf("分享不存在: %s", shareID)
	}
	return share, nil
}

// GetShareByToken 通过令牌获取分享.
func (s *Service) GetShareByToken(ctx context.Context, token string) (*WebShare, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	shareID, ok := s.tokenIdx[token]
	if !ok {
		return nil, fmt.Errorf("无效的分享令牌")
	}

	share, ok := s.shares[shareID]
	if !ok {
		return nil, fmt.Errorf("分享不存在")
	}
	return share, nil
}

// ListShares 列出所有分享.
func (s *Service) ListShares(ctx context.Context, status ShareStatus) ([]*WebShare, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebShare
	for _, share := range s.shares {
		if status == "" || share.Status == status {
			result = append(result, share)
		}
	}
	return result, nil
}

// RevokeShare 撤销分享.
func (s *Service) RevokeShare(ctx context.Context, shareID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	share, ok := s.shares[shareID]
	if !ok {
		return fmt.Errorf("分享不存在: %s", shareID)
	}

	share.Status = ShareStatusRevoked
	share.UpdatedAt = time.Now()

	// 终止所有活跃会话
	for _, sess := range s.sessions {
		if sess.ShareID == shareID && sess.IsActive {
			sess.IsActive = false
		}
	}

	return nil
}

// DeleteShare 删除分享.
func (s *Service) DeleteShare(ctx context.Context, shareID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	share, ok := s.shares[shareID]
	if !ok {
		return fmt.Errorf("分享不存在: %s", shareID)
	}

	// 清理令牌索引
	delete(s.tokenIdx, share.Token)

	// 清理关联会话
	for sessID, sess := range s.sessions {
		if sess.ShareID == shareID {
			delete(s.sessions, sessID)
		}
	}

	delete(s.shares, shareID)
	return nil
}

// UpdateSharePermission 更新分享权限.
func (s *Service) UpdateSharePermission(ctx context.Context, shareID string, perm *SharePermission) (*WebShare, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	share, ok := s.shares[shareID]
	if !ok {
		return nil, fmt.Errorf("分享不存在: %s", shareID)
	}

	share.Permission = perm
	share.UpdatedAt = time.Now()
	return share, nil
}

// ========== 会话管理 ==========

// CreateSession 创建浏览器访问会话.
func (s *Service) CreateSession(ctx context.Context, shareToken, clientIP, userAgent, password string) (*BrowserSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	shareID, ok := s.tokenIdx[shareToken]
	if !ok {
		return nil, fmt.Errorf("无效的分享令牌")
	}

	share, ok := s.shares[shareID]
	if !ok {
		return nil, fmt.Errorf("分享不存在")
	}

	// 检查分享状态
	if share.Status != ShareStatusActive {
		return nil, fmt.Errorf("分享已%s", share.Status)
	}

	// 检查过期
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		share.Status = ShareStatusExpired
		return nil, fmt.Errorf("分享已过期")
	}

	// 验证密码
	if share.PasswordEnabled {
		if !verifyPassword(share.PasswordHash, password) {
			return nil, fmt.Errorf("密码错误")
		}
	}

	// 检查并发限制
	if share.MaxConcurrentAccess > 0 {
		activeCount := 0
		for _, sess := range s.sessions {
			if sess.ShareID == shareID && sess.IsActive {
				activeCount++
			}
		}
		if activeCount >= share.MaxConcurrentAccess {
			return nil, fmt.Errorf("已达到最大并发访问数 %d", share.MaxConcurrentAccess)
		}
	}

	sess := newSession(shareID, clientIP, userAgent, "/", s.config.DefaultSessionTimeoutMinutes)
	s.sessions[sess.ID] = sess
	share.ActiveSessionCount++

	return sess, nil
}

// GetSession 获取会话.
func (s *Service) GetSession(ctx context.Context, sessionID string) (*BrowserSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在")
	}

	// 检查过期
	if sess.IsActive && time.Now().After(sess.ExpiresAt) {
		sess.IsActive = false
		return nil, fmt.Errorf("会话已过期")
	}

	return sess, nil
}

// ValidateSession 验证会话并更新活动时间.
func (s *Service) ValidateSession(ctx context.Context, sessionID string) (*BrowserSession, *WebShare, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, nil, fmt.Errorf("会话不存在")
	}

	if !sess.IsActive {
		return nil, nil, fmt.Errorf("会话已失效")
	}

	// 检查会话过期
	if time.Now().After(sess.ExpiresAt) {
		sess.IsActive = false
		return nil, nil, fmt.Errorf("会话已过期")
	}

	share, ok := s.shares[sess.ShareID]
	if !ok {
		return nil, nil, fmt.Errorf("关联的分享不存在")
	}

	// 检查分享状态
	if share.Status != ShareStatusActive {
		return nil, nil, fmt.Errorf("分享已%s", share.Status)
	}

	// 更新最后活动时间
	sess.LastActiveAt = time.Now()

	return sess, share, nil
}

// DestroySession 销毁会话.
func (s *Service) DestroySession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话不存在")
	}

	sess.IsActive = false

	// 减少活跃计数
	if share, exists := s.shares[sess.ShareID]; exists && share.ActiveSessionCount > 0 {
		share.ActiveSessionCount--
	}

	return nil
}

// CleanupExpiredSessions 清理过期会话.
func (s *Service) CleanupExpiredSessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()
	for _, sess := range s.sessions {
		if sess.IsActive && now.After(sess.ExpiresAt) {
			sess.IsActive = false
			if share, exists := s.shares[sess.ShareID]; exists && share.ActiveSessionCount > 0 {
				share.ActiveSessionCount--
			}
			count++
		}
	}
	return count
}

// ========== 文件操作 ==========

// ListFiles 浏覽文件目录.
func (s *Service) ListFiles(ctx context.Context, sessionID, path string) ([]FileEntry, error) {
	sess, share, err := s.ValidateSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// 检查浏览权限
	if !share.Permission.CanBrowse {
		return nil, fmt.Errorf("无浏览权限")
	}

	// 更新当前路径
	sess.CurrentPath = path

	// 模拟文件列表（实际应从文件系统读取）
	entries := []FileEntry{}

	return entries, nil
}

// CreateFolder 创建文件夹.
func (s *Service) CreateFolder(ctx context.Context, sessionID, path string) error {
	sess, share, err := s.ValidateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if !share.Permission.CanMkdir {
		return fmt.Errorf("无创建文件夹权限")
	}

	// 更新当前路径
	_ = sess

	// 实际应调用文件系统创建目录
	// fs.Mkdir(filepath.Join(share.RootPath, path))
	return nil
}

// UploadFile 上传文件.
func (s *Service) UploadFile(ctx context.Context, sessionID, path string, fileSize int64) error {
	_, share, err := s.ValidateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if !share.Permission.CanUpload {
		return fmt.Errorf("无上传权限")
	}

	// 检查文件大小限制
	if share.Permission.MaxUploadSize > 0 && fileSize > share.Permission.MaxUploadSize {
		return fmt.Errorf("文件大小 %d 超过限制 %d", fileSize, share.Permission.MaxUploadSize)
	}

	// 实际应写入文件系统
	return nil
}

// DownloadFile 下载文件.
func (s *Service) DownloadFile(ctx context.Context, sessionID, path string) error {
	_, share, err := s.ValidateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if !share.Permission.CanDownload {
		return fmt.Errorf("无下载权限")
	}

	// 实际应从文件系统读取文件并返回
	return nil
}

// DeleteFile 删除文件.
func (s *Service) DeleteFile(ctx context.Context, sessionID, path string) error {
	_, share, err := s.ValidateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if !share.Permission.CanDelete {
		return fmt.Errorf("无删除权限")
	}

	return nil
}

// RenameFile 重命名文件.
func (s *Service) RenameFile(ctx context.Context, sessionID, oldPath, newPath string) error {
	_, share, err := s.ValidateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if !share.Permission.CanRename {
		return fmt.Errorf("无重命名权限")
	}

	return nil
}

// ========== 分享链接 ==========

// GenerateShareLink 生成可分享链接.
func (s *Service) GenerateShareLink(ctx context.Context, shareID, baseURL string) (*ShareLinkResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	share, ok := s.shares[shareID]
	if !ok {
		return nil, fmt.Errorf("分享不存在: %s", shareID)
	}

	if share.Status != ShareStatusActive {
		return nil, fmt.Errorf("分享已%s", share.Status)
	}

	url := fmt.Sprintf("%s/share/%s", baseURL, share.Token)
	return &ShareLinkResponse{
		URL:   url,
		Token: share.Token,
	}, nil
}

// ========== FIPS 加密 ==========

// EnableFIPS 启用 FIPS 加密传输.
func (s *Service) EnableFIPS(ctx context.Context, shareID string) (*WebShare, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	share, ok := s.shares[shareID]
	if !ok {
		return nil, fmt.Errorf("分享不存在: %s", shareID)
	}

	share.FIPSEnabled = true
	share.UpdatedAt = time.Now()
	return share, nil
}

// DisableFIPS 禁用 FIPS 加密传输.
func (s *Service) DisableFIPS(ctx context.Context, shareID string) (*WebShare, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	share, ok := s.shares[shareID]
	if !ok {
		return nil, fmt.Errorf("分享不存在: %s", shareID)
	}

	share.FIPSEnabled = false
	share.UpdatedAt = time.Now()
	return share, nil
}

// ========== 统计 ==========

// GetStats 获取分享统计.
func (s *Service) GetStats(ctx context.Context) (*ShareStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &ShareStats{}
	for _, share := range s.shares {
		stats.TotalShares++
		switch share.Status {
		case ShareStatusActive:
			stats.ActiveShares++
		case ShareStatusExpired:
			stats.ExpiredShares++
		case ShareStatusRevoked:
			stats.RevokedShares++
		}
		if share.FIPSEnabled {
			stats.FIPSEnabledShares++
		}
		if share.PasswordEnabled {
			stats.PasswordProtected++
		}
	}

	for _, sess := range s.sessions {
		stats.TotalSessions++
		if sess.IsActive {
			stats.ActiveSessions++
		}
	}

	return stats, nil
}

// ========== 配置管理 ==========

// GetConfig 获取配置.
func (s *Service) GetConfig() *WebShareConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := *s.config
	return &cfg
}

// UpdateConfig 更新配置.
func (s *Service) UpdateConfig(cfg *WebShareConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// ========== 密码管理 ==========

// SetPassword 设置分享密码.
func (s *Service) SetPassword(ctx context.Context, shareID, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	share, ok := s.shares[shareID]
	if !ok {
		return fmt.Errorf("分享不存在: %s", shareID)
	}

	if password == "" {
		share.PasswordEnabled = false
		share.PasswordHash = ""
	} else {
		share.PasswordEnabled = true
		share.PasswordHash = hashPassword(password)
	}
	share.UpdatedAt = time.Now()
	return nil
}

// VerifyPassword 验证分享密码.
func (s *Service) VerifyPassword(ctx context.Context, shareID, password string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	share, ok := s.shares[shareID]
	if !ok {
		return fmt.Errorf("分享不存在: %s", shareID)
	}

	if !share.PasswordEnabled {
		return nil // 无密码保护
	}

	if !verifyPassword(share.PasswordHash, password) {
		return fmt.Errorf("密码错误")
	}

	return nil
}
