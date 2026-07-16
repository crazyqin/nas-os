package filemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 文件管理器核心管理器.
type Manager struct {
	mu         sync.RWMutex
	config     Config
	browser    *Browser
	operations *Operations
	preview    *Preview
	share      *Share

	// 收藏夹（内存存储）
	favorites map[string]*Favorite // id -> favorite

	// 快捷方式
	shortcuts map[string]*Shortcut

	// 版本管理（内存存储，简化实现）
	versions map[string][]*FileVersion // filePath -> versions

	// 搜索索引（简化实现）
	searchIndex map[string][]string // keyword -> filePaths

	logger *zap.Logger
}

// NewManager 创建文件管理器.
func NewManager(config Config, logger *zap.Logger) (*Manager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 确保根目录存在
	if err := os.MkdirAll(config.RootPath, 0755); err != nil {
		return nil, fmt.Errorf("创建根目录失败: %w", err)
	}

	// 确保临时目录存在
	if config.TempDir == "" {
		config.TempDir = "/tmp/nas-filemanager"
	}
	os.MkdirAll(config.TempDir, 0755)

	mgr := &Manager{
		config:      config,
		browser:     NewBrowser(config.RootPath, logger),
		operations:  NewOperations(config.RootPath, config.TempDir, logger),
		preview:     NewPreview(config.Thumbnails, logger),
		share:       NewShare(config.RootPath, logger),
		favorites:   make(map[string]*Favorite),
		shortcuts:   make(map[string]*Shortcut),
		versions:    make(map[string][]*FileVersion),
		searchIndex: make(map[string][]string),
		logger:      logger,
	}

	// 启动定期清理过期分享链接
	go mgr.cleanupRoutine()

	return mgr, nil
}

// Search 搜索文件.
func (m *Manager) Search(query SearchQuery) (*SearchResult, error) {
	startTime := time.Now()

	if query.MaxResults <= 0 {
		query.MaxResults = 100
	}

	// 确定搜索根目录
	searchPath := m.config.RootPath
	if query.Path != "" {
		cleanPath := filepath.Clean(query.Path)
		if filepath.IsAbs(cleanPath) && strings.HasPrefix(cleanPath, m.config.RootPath) {
			searchPath = cleanPath
		}
	}

	var results []*FileNode
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过权限错误
		}

		// 检查是否已达到最大结果数
		if len(results) >= query.MaxResults {
			return filepath.SkipAll
		}

		// 过滤文件类型
		if query.FileType != "" {
			isDir := info.IsDir()
			if query.FileType == FileTypeDirectory && !isDir {
				return nil
			}
			if query.FileType == FileTypeFile && isDir {
				return nil
			}
		}

		// 扩展名过滤
		if len(query.Extensions) > 0 {
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
			found := false
			for _, allowedExt := range query.Extensions {
				if strings.ToLower(allowedExt) == ext {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		// 大小过滤
		if query.MinSize != nil && info.Size() < *query.MinSize {
			return nil
		}
		if query.MaxSize != nil && info.Size() > *query.MaxSize {
			return nil
		}

		// 时间过滤
		if query.ModAfter != nil && info.ModTime().Before(*query.ModAfter) {
			return nil
		}
		if query.ModBefore != nil && info.ModTime().After(*query.ModBefore) {
			return nil
		}

		// 关键词匹配（模糊匹配文件名）
		if query.Keyword != "" {
			matched := fuzzyMatch(strings.ToLower(info.Name()), strings.ToLower(query.Keyword))
			if !matched {
				// 如果启用全文检索
				if query.ContentSearch && !info.IsDir() {
					if !m.searchFileContent(path, query.Keyword) {
						return nil
					}
				} else {
					return nil
				}
			}
		}

		// 构建结果节点
		node := &FileNode{
			Name:      info.Name(),
			Path:      path,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			Mode:      info.Mode().String(),
			IsHidden:  strings.HasPrefix(info.Name(), "."),
			Extension: strings.TrimPrefix(filepath.Ext(info.Name()), "."),
		}

		if info.IsDir() {
			node.Type = FileTypeDirectory
		} else {
			node.Type = FileTypeFile
			node.MIMEType = getMIMEType(path)
		}

		results = append(results, node)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 排序
	if query.SortBy == "" {
		query.SortBy = "name"
	}
	if query.SortOrder == "" {
		query.SortOrder = "asc"
	}

	sortFileNodes(results, query.SortBy, query.SortOrder)

	duration := time.Since(startTime)

	return &SearchResult{
		Items:     results,
		Total:     len(results),
		Truncated: len(results) >= query.MaxResults,
		Query:     query,
		Duration:  duration.Milliseconds(),
	}, nil
}

// ListFavorites 列出收藏.
func (m *Manager) ListFavorites(userID string) []*Favorite {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Favorite
	for _, fav := range m.favorites {
		if fav.CreatedBy == "" || fav.CreatedBy == userID {
			result = append(result, fav)
		}
	}

	return result
}

// AddFavorite 添加收藏.
func (m *Manager) AddFavorite(path string, userID string) (*Favorite, error) {
	// 验证路径
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(m.config.RootPath, path)
	}

	// 安全检查
	if cleanPath != m.config.RootPath && !strings.HasPrefix(cleanPath, m.config.RootPath) {
		return nil, fmt.Errorf("路径超出根目录范围")
	}

	// 检查文件是否存在
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %s", path)
	}

	// 检查是否已收藏
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, fav := range m.favorites {
		if fav.Path == cleanPath && fav.CreatedBy == userID {
			return fav, nil // 已收藏
		}
	}

	fav := &Favorite{
		ID:        uuid.New().String(),
		Path:      cleanPath,
		Name:      info.Name(),
		CreatedAt: time.Now(),
		CreatedBy: userID,
	}

	if info.IsDir() {
		fav.Type = FileTypeDirectory
	} else {
		fav.Type = FileTypeFile
	}

	m.favorites[fav.ID] = fav

	m.logger.Info("添加收藏",
		zap.String("path", cleanPath),
		zap.String("user", userID))

	return fav, nil
}

// RemoveFavorite 删除收藏.
func (m *Manager) RemoveFavorite(id string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fav, ok := m.favorites[id]
	if !ok {
		return fmt.Errorf("收藏不存在: %s", id)
	}

	// 检查权限
	if fav.CreatedBy != "" && fav.CreatedBy != userID {
		return fmt.Errorf("无权删除此收藏")
	}

	delete(m.favorites, id)

	m.logger.Info("删除收藏",
		zap.String("id", id),
		zap.String("user", userID))

	return nil
}

// ListVersions 列出文件版本.
func (m *Manager) ListVersions(filePath string) ([]*FileVersion, error) {
	if !m.config.EnableVersions {
		return nil, fmt.Errorf("版本管理功能未启用")
	}

	cleanPath := filepath.Clean(filePath)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(m.config.RootPath, filePath)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, ok := m.versions[cleanPath]
	if !ok {
		return []*FileVersion{}, nil
	}

	return versions, nil
}

// CreateVersion 创建文件版本.
func (m *Manager) CreateVersion(filePath string, comment string, userID string) (*FileVersion, error) {
	if !m.config.EnableVersions {
		return nil, fmt.Errorf("版本管理功能未启用")
	}

	cleanPath := filepath.Clean(filePath)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(m.config.RootPath, filePath)
	}

	// 检查文件是否存在
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %s", filePath)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("不支持对目录进行版本管理")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[cleanPath]

	// 检查版本数限制
	if len(versions) >= m.config.Versions.MaxVersions {
		// 移除最旧的版本
		versions = versions[1:]
	}

	versionNum := len(versions) + 1

	version := &FileVersion{
		ID:        uuid.New().String(),
		FilePath:  cleanPath,
		Version:   versionNum,
		Size:      info.Size(),
		Comment:   comment,
		CreatedBy: userID,
		CreatedAt: time.Now(),
	}

	m.versions[cleanPath] = append(versions, version)

	m.logger.Info("创建文件版本",
		zap.String("path", cleanPath),
		zap.Int("version", versionNum),
		zap.String("user", userID))

	return version, nil
}

// RestoreVersion 恢复文件版本.
func (m *Manager) RestoreVersion(versionID string) error {
	if !m.config.EnableVersions {
		return fmt.Errorf("版本管理功能未启用")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找版本
	for _, versions := range m.versions {
		for _, v := range versions {
			if v.ID == versionID {
				// 简化实现：实际应该从备份恢复文件内容
				m.logger.Info("恢复文件版本",
					zap.String("id", versionID),
					zap.String("path", v.FilePath),
					zap.Int("version", v.Version))
				return nil
			}
		}
	}

	return fmt.Errorf("版本不存在: %s", versionID)
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() Config {
	return m.config
}

// GetBrowser 获取浏览器.
func (m *Manager) GetBrowser() *Browser {
	return m.browser
}

// GetOperations 获取操作管理器.
func (m *Manager) GetOperations() *Operations {
	return m.operations
}

// GetPreview 获取预览器.
func (m *Manager) GetPreview() *Preview {
	return m.preview
}

// GetShare 获取分享管理器.
func (m *Manager) GetShare() *Share {
	return m.share
}

// fuzzyMatch 模糊匹配.
func fuzzyMatch(s, pattern string) bool {
	if pattern == "" {
		return true
	}
	if s == "" {
		return false
	}

	// 简单的子串匹配
	if strings.Contains(s, pattern) {
		return true
	}

	// 通配符匹配
	patternIdx := 0
	for i := 0; i < len(s) && patternIdx < len(pattern); i++ {
		if s[i] == pattern[patternIdx] {
			patternIdx++
		}
	}

	return patternIdx == len(pattern)
}

// searchFileContent 搜索文件内容.
func (m *Manager) searchFileContent(path string, keyword string) bool {
	// 简化实现：读取文件内容搜索
	// 实际实现应该使用 bleve 等全文搜索引擎

	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(string(content)), strings.ToLower(keyword))
}

// sortFileNodes 排序文件节点.
func sortFileNodes(nodes []*FileNode, sortBy, sortOrder string) {
	// 使用简单的插入排序
	for i := 1; i < len(nodes); i++ {
		key := nodes[i]
		j := i - 1

		for j >= 0 && shouldSwap(nodes[j], key, sortBy, sortOrder) {
			nodes[j+1] = nodes[j]
			j--
		}
		nodes[j+1] = key
	}
}

// shouldSwap 判断是否需要交换.
func shouldSwap(a, b *FileNode, sortBy, sortOrder string) bool {
	var less bool

	switch sortBy {
	case "size":
		less = a.Size < b.Size
	case "mod_time":
		less = a.ModTime.Before(b.ModTime)
	default: // name
		less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
	}

	if sortOrder == "desc" {
		return !less
	}
	return less
}

// cleanupRoutine 定期清理过期分享链接.
func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.share.CleanupExpired()
	}
}

// Favorite 的 CreatedBy 字段扩展
// 在 types.go 中 Favorite 已有 CreatedBy 字段，这里补充使用
