package filemanager

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Share 文件分享管理器.
type Share struct {
	mu       sync.RWMutex
	rootPath string
	links    map[string]*ShareLink // id -> link
	tokens   map[string]*ShareLink // token -> link
	logger   *zap.Logger
}

// NewShare 创建文件分享管理器.
func NewShare(rootPath string, logger *zap.Logger) *Share {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Share{
		rootPath: rootPath,
		links:    make(map[string]*ShareLink),
		tokens:   make(map[string]*ShareLink),
		logger:   logger,
	}
}

// CreateLink 创建分享链接.
func (s *Share) CreateLink(req CreateShareRequest, userID string) (*ShareLink, error) {
	// 验证路径
	cleanPath, err := s.validatePath(req.Path)
	if err != nil {
		return nil, err
	}

	// 检查文件是否存在
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %s", req.Path)
	}

	// 生成唯一token
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("生成token失败: %w", err)
	}

	// 创建分享链接
	link := &ShareLink{
		ID:           uuid.New().String(),
		Path:         cleanPath,
		Name:         info.Name(),
		Token:        token,
		MaxDownloads: req.MaxDownloads,
		ExpiresAt:    req.ExpiresAt,
		CreatedAt:    time.Now(),
		CreatedBy:    userID,
		Permission:   req.Permission,
		Enabled:      true,
	}

	// 设置密码
	if req.Password != "" {
		link.Password = hashPassword(req.Password)
		link.HasPassword = true
	}

	// 设置默认权限
	if link.Permission == "" {
		if info.IsDir() {
			link.Permission = "view"
		} else {
			link.Permission = "download"
		}
	}

	// 保存链接
	s.mu.Lock()
	s.links[link.ID] = link
	s.tokens[token] = link
	s.mu.Unlock()

	s.logger.Info("创建分享链接",
		zap.String("id", link.ID),
		zap.String("path", cleanPath),
		zap.String("user", userID))

	return link, nil
}

// GetLink 获取分享链接.
func (s *Share) GetLink(id string) (*ShareLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.links[id]
	if !ok {
		return nil, fmt.Errorf("分享链接不存在: %s", id)
	}

	return link, nil
}

// GetLinkByToken 通过token获取分享链接.
func (s *Share) GetLinkByToken(token string) (*ShareLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.tokens[token]
	if !ok {
		return nil, fmt.Errorf("分享链接不存在")
	}

	// 检查是否过期
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, fmt.Errorf("分享链接已过期")
	}

	// 检查是否启用
	if !link.Enabled {
		return nil, fmt.Errorf("分享链接已禁用")
	}

	// 检查下载次数
	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		return nil, fmt.Errorf("已达到最大下载次数")
	}

	return link, nil
}

// VerifyPassword 验证分享密码.
func (s *Share) VerifyPassword(token, password string) (bool, error) {
	link, err := s.GetLinkByToken(token)
	if err != nil {
		return false, err
	}

	if !link.HasPassword {
		return true, nil
	}

	return link.Password == hashPassword(password), nil
}

// RecordDownload 记录下载.
func (s *Share) RecordDownload(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.tokens[token]
	if !ok {
		return fmt.Errorf("分享链接不存在")
	}

	// 检查是否过期
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return fmt.Errorf("分享链接已过期")
	}

	// 检查下载次数
	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		return fmt.Errorf("已达到最大下载次数")
	}

	link.DownloadCount++
	return nil
}

// DeleteLink 删除分享链接.
func (s *Share) DeleteLink(id string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.links[id]
	if !ok {
		return fmt.Errorf("分享链接不存在: %s", id)
	}

	// 检查权限（只有创建者可以删除）
	if link.CreatedBy != userID {
		return fmt.Errorf("无权删除此分享链接")
	}

	delete(s.links, id)
	delete(s.tokens, link.Token)

	s.logger.Info("删除分享链接",
		zap.String("id", id),
		zap.String("user", userID))

	return nil
}

// UpdateLink 更新分享链接.
func (s *Share) UpdateLink(id string, updates map[string]interface{}, userID string) (*ShareLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.links[id]
	if !ok {
		return nil, fmt.Errorf("分享链接不存在: %s", id)
	}

	// 检查权限
	if link.CreatedBy != userID {
		return nil, fmt.Errorf("无权修改此分享链接")
	}

	// 应用更新
	if v, ok := updates["password"].(string); ok {
		if v == "" {
			link.Password = ""
			link.HasPassword = false
		} else {
			link.Password = hashPassword(v)
			link.HasPassword = true
		}
	}

	if v, ok := updates["max_downloads"].(float64); ok {
		link.MaxDownloads = int(v)
	}

	if v, ok := updates["expires_at"].(string); ok && v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			link.ExpiresAt = &t
		}
	}

	if v, ok := updates["enabled"].(bool); ok {
		link.Enabled = v
	}

	if v, ok := updates["permission"].(string); ok {
		link.Permission = v
	}

	return link, nil
}

// ListLinks 列出用户的所有分享链接.
func (s *Share) ListLinks(userID string) []*ShareLink {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ShareLink
	for _, link := range s.links {
		if link.CreatedBy == userID {
			result = append(result, link)
		}
	}

	return result
}

// ListAllLinks 列出所有分享链接（管理员）.
func (s *Share) ListAllLinks() []*ShareLink {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ShareLink, 0, len(s.links))
	for _, link := range s.links {
		result = append(result, link)
	}

	return result
}

// GetStats 获取分享统计.
func (s *Share) GetStats() ShareStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ShareStats{
		TotalLinks: len(s.links),
	}

	now := time.Now()
	for _, link := range s.links {
		if link.ExpiresAt != nil && now.After(*link.ExpiresAt) {
			stats.ExpiredLinks++
		} else if link.Enabled {
			stats.ActiveLinks++
		}
		stats.TotalDownloads += link.DownloadCount
	}

	return stats
}

// CleanupExpired 清理过期链接.
func (s *Share) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	count := 0

	for id, link := range s.links {
		if link.ExpiresAt != nil && now.After(*link.ExpiresAt) {
			delete(s.links, id)
			delete(s.tokens, link.Token)
			count++
		}
	}

	if count > 0 {
		s.logger.Info("清理过期分享链接", zap.Int("count", count))
	}

	return count
}

// GetShareablePath 获取可分享的文件路径.
func (s *Share) GetShareablePath(token, password string) (string, error) {
	// 验证链接
	link, err := s.GetLinkByToken(token)
	if err != nil {
		return "", err
	}

	// 验证密码
	if link.HasPassword {
		if password == "" {
			return "", fmt.Errorf("需要密码")
		}
		if link.Password != hashPassword(password) {
			return "", fmt.Errorf("密码错误")
		}
	}

	return link.Path, nil
}

// validatePath 验证路径.
func (s *Share) validatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("路径不能为空")
	}

	// 转换为绝对路径
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.rootPath, path)
	}

	cleanPath := filepath.Clean(path)

	// 安全检查
	if cleanPath != s.rootPath && !isSubpath(cleanPath, s.rootPath) {
		return "", fmt.Errorf("路径超出根目录范围: %s", path)
	}

	return cleanPath, nil
}

// isSubpath 检查是否是子路径.
func isSubpath(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && len(rel) > 0 && rel[0] != '.'
}

// generateToken 生成随机token.
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// hashPassword 哈希密码.
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}
