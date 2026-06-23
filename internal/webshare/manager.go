package webshare

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager WebShare 管理器
type Manager struct {
	mu          sync.RWMutex
	config      *WebShareConfig
	links       map[string]*ShareLink
	filterCache *FilterCache
}

// FilterCache 过滤缓存
type FilterCache struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Listing   *DirectoryListing
	CachedAt  time.Time
}

// NewFilterCache 创建过滤缓存
func NewFilterCache() *FilterCache {
	return &FilterCache{
		cache: make(map[string]*CacheEntry),
	}
}

func (fc *FilterCache) get(key string) *CacheEntry {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.cache[key]
}

func (fc *FilterCache) set(key string, entry *CacheEntry) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.cache[key] = entry
}

func (fc *FilterCache) invalidate(key string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	delete(fc.cache, key)
}

func (fc *FilterCache) invalidateAll() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.cache = make(map[string]*CacheEntry)
}

// NewManager 创建 WebShare 管理器
func NewManager(config *WebShareConfig) *Manager {
	if config == nil {
		config = DefaultWebShareConfig()
	}
	return &Manager{
		config:      config,
		links:       make(map[string]*ShareLink),
		filterCache: NewFilterCache(),
	}
}

// ListDirectory 列出目录内容
func (m *Manager) ListDirectory(path string, showHidden bool, filter string, sortBy SortField, sortDir SortDirection) (*DirectoryListing, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 清理路径
	dirPath := m.sanitizePath(path)
	absDirPath := filepath.Join(m.config.RootPath, dirPath)

	// 安全检查
	if !m.isPathSafe(absDirPath) {
		return nil, fmt.Errorf("访问被拒绝")
	}

	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%v:%s:%s:%s", dirPath, showHidden, filter, sortBy, sortDir)
	if cached := m.filterCache.get(cacheKey); cached != nil && time.Since(cached.CachedAt) < 30*time.Second {
		return cached.Listing, nil
	}

	// 读取目录
	entries, err := os.ReadDir(absDirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var result []Entry
	var totalSize int64

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 隐藏文件过滤
		if !showHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		// 过滤
		if filter != "" && !strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(filter)) {
			continue
		}

		entryType := FileTypeFile
		if entry.IsDir() {
			entryType = FileTypeDirectory
		} else if entry.Type()&os.ModeSymlink != 0 {
			entryType = FileTypeSymlink
		}

		e := Entry{
			Name:         entry.Name(),
			Path:         filepath.Join(dirPath, entry.Name()),
			AbsolutePath: filepath.Join(absDirPath, entry.Name()),
			Type:         entryType,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			IsHidden:     strings.HasPrefix(entry.Name(), "."),
			Extension:    filepath.Ext(entry.Name()),
			MimeType:     getMimeType(entry.Name()),
			Permission:   info.Mode().String(),
			Protocol:     ProtocolLocal,
		}

		result = append(result, e)
		totalSize += info.Size()
	}

	// 排序
	sortEntries(result, sortBy, sortDir)

	parentPath := filepath.Dir(dirPath)
	if parentPath == "." {
		parentPath = ""
	}

	listing := &DirectoryListing{
		Path:       dirPath,
		ParentPath: parentPath,
		Entries:    result,
		TotalCount: len(result),
		TotalSize:  totalSize,
		ShowHidden: showHidden,
		FilteredBy: filter,
		Protocol:   ProtocolLocal,
	}

	// 缓存结果
	m.filterCache.set(cacheKey, &CacheEntry{
		Listing:  listing,
		CachedAt: time.Now(),
	})

	return listing, nil
}

// CreateFolder 创建文件夹
func (m *Manager) CreateFolder(path, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dirPath := m.sanitizePath(path)
	absDirPath := filepath.Join(m.config.RootPath, dirPath)

	if !m.isPathSafe(absDirPath) {
		return fmt.Errorf("访问被拒绝")
	}

	newDir := filepath.Join(absDirPath, name)
	if !m.isPathSafe(newDir) {
		return fmt.Errorf("访问被拒绝")
	}

	if err := os.MkdirAll(newDir, 0755); err != nil {
		return fmt.Errorf("创建文件夹失败: %w", err)
	}

	m.filterCache.invalidateAll()
	return nil
}

// DeleteEntries 删除文件/目录
func (m *Manager) DeleteEntries(paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, path := range paths {
		dirPath := m.sanitizePath(path)
		absPath := filepath.Join(m.config.RootPath, dirPath)

		if !m.isPathSafe(absPath) {
			return fmt.Errorf("访问被拒绝: %s", path)
		}

		if err := os.RemoveAll(absPath); err != nil {
			return fmt.Errorf("删除失败 %s: %w", path, err)
		}
	}

	m.filterCache.invalidateAll()
	return nil
}

// MoveEntries 移动文件/目录
func (m *Manager) MoveEntries(paths []string, dest string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	destPath := m.sanitizePath(dest)
	absDest := filepath.Join(m.config.RootPath, destPath)

	if !m.isPathSafe(absDest) {
		return fmt.Errorf("访问被拒绝")
	}

	for _, path := range paths {
		srcPath := m.sanitizePath(path)
		absSrc := filepath.Join(m.config.RootPath, srcPath)

		if !m.isPathSafe(absSrc) {
			return fmt.Errorf("访问被拒绝: %s", path)
		}

		newPath := filepath.Join(absDest, filepath.Base(absSrc))
		if err := os.Rename(absSrc, newPath); err != nil {
			return fmt.Errorf("移动失败 %s: %w", path, err)
		}
	}

	m.filterCache.invalidateAll()
	return nil
}

// CopyEntries 复制文件/目录
func (m *Manager) CopyEntries(paths []string, dest string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	destPath := m.sanitizePath(dest)
	absDest := filepath.Join(m.config.RootPath, destPath)

	if !m.isPathSafe(absDest) {
		return fmt.Errorf("访问被拒绝")
	}

	for _, path := range paths {
		srcPath := m.sanitizePath(path)
		absSrc := filepath.Join(m.config.RootPath, srcPath)

		if !m.isPathSafe(absSrc) {
			return fmt.Errorf("访问被拒绝: %s", path)
		}

		if err := copyRecursive(absSrc, absDest); err != nil {
			return fmt.Errorf("复制失败 %s: %w", path, err)
		}
	}

	m.filterCache.invalidateAll()
	return nil
}

// RenameEntry 重命名
func (m *Manager) RenameEntry(path, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	srcPath := m.sanitizePath(path)
	absSrc := filepath.Join(m.config.RootPath, srcPath)

	if !m.isPathSafe(absSrc) {
		return fmt.Errorf("访问被拒绝")
	}

	newPath := filepath.Join(filepath.Dir(absSrc), newName)
	if !m.isPathSafe(newPath) {
		return fmt.Errorf("访问被拒绝")
	}

	if err := os.Rename(absSrc, newPath); err != nil {
		return fmt.Errorf("重命名失败: %w", err)
	}

	m.filterCache.invalidateAll()
	return nil
}

// sanitizePath 清理路径
func (m *Manager) sanitizePath(path string) string {
	// 移除前导斜杠
	path = strings.TrimPrefix(path, "/")
	// 清理路径
	return filepath.Clean(path)
}

// isPathSafe 检查路径安全性
func (m *Manager) isPathSafe(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(m.config.RootPath)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, absRoot)
}

// CreateShareLink 创建分享链接
func (m *Manager) CreateShareLink(req CreateShareLinkRequest) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证路径
	path := m.sanitizePath(req.Path)
	absPath := filepath.Join(m.config.RootPath, path)
	if !m.isPathSafe(absPath) {
		return nil, fmt.Errorf("访问被拒绝")
	}

	_, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("路径不存在: %w", err)
	}

	// 生成 token
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("生成 token 失败: %w", err)
	}

	// 处理密码
	var hashedPassword string
	if req.Password != "" {
		hash := sha256.Sum256([]byte(req.Password))
		hashedPassword = hex.EncodeToString(hash[:])
	}

	// 处理过期时间
	var expiresAt *time.Time
	if req.Expiry > 0 {
		t := time.Now().Add(req.Expiry)
		expiresAt = &t
	}

	link := &ShareLink{
		ID:           generateID(),
		Name:         req.Name,
		Path:         path,
		Token:        token,
		Permission:   req.Permission,
		Password:     hashedPassword,
		MaxDownloads: req.MaxDownloads,
		ExpiresAt:    expiresAt,
		CreatedBy:    req.CreatedBy,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		IsActive:     true,
		Protocol:     ProtocolLocal,
	}

	m.links[link.ID] = link
	return link, nil
}

// ListShareLinks 列出分享链接
func (m *Manager) ListShareLinks(createdBy string, includeInactive bool) []*ShareLink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ShareLink
	for _, link := range m.links {
		if createdBy != "" && link.CreatedBy != createdBy {
			continue
		}
		if !includeInactive && !link.IsActive {
			continue
		}
		result = append(result, link)
	}

	return result
}

// GetShareLinkByID 通过 ID 获取分享链接
func (m *Manager) GetShareLinkByID(id string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[id]
	if !ok {
		return nil, fmt.Errorf("分享链接不存在")
	}

	return link, nil
}

// UpdateShareLink 更新分享链接
func (m *Manager) UpdateShareLink(id string, updates map[string]interface{}) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return nil, fmt.Errorf("分享链接不存在")
	}

	if name, ok := updates["name"].(string); ok {
		link.Name = name
	}
	if perm, ok := updates["permission"].(Permission); ok {
		link.Permission = perm
	}
	if active, ok := updates["is_active"].(bool); ok {
		link.IsActive = active
	}

	link.UpdatedAt = time.Now()
	return link, nil
}

// DeleteShareLink 删除分享链接
func (m *Manager) DeleteShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.links[id]; !ok {
		return fmt.Errorf("分享链接不存在")
	}

	delete(m.links, id)
	return nil
}

// GetShareLinkByToken 通过 token 获取分享链接
func (m *Manager) GetShareLinkByToken(token string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, link := range m.links {
		if link.Token == token && link.IsActive {
			// 检查过期
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				return nil, fmt.Errorf("分享链接已过期")
			}
			// 检查下载次数
			if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
				return nil, fmt.Errorf("已达到最大下载次数")
			}
			return link, nil
		}
	}

	return nil, fmt.Errorf("分享链接不存在")
}

// RecordShareAccess 记录分享访问
func (m *Manager) RecordShareAccess(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if link, ok := m.links[id]; ok {
		link.AccessCount++
		now := time.Now()
		link.LastAccessAt = &now
	}
}

// RecordShareDownload 记录分享下载
func (m *Manager) RecordShareDownload(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if link, ok := m.links[id]; ok {
		link.DownloadCount++
	}
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSize int64
	fileCount := 0
	dirCount := 0

	filepath.Walk(m.config.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			dirCount++
		} else {
			fileCount++
			totalSize += info.Size()
		}
		return nil
	})

	return map[string]interface{}{
		"totalFiles": fileCount,
		"totalDirs":  dirCount,
		"totalSize":  totalSize,
	}
}

// GetShareLink 通过 token 获取分享链接
func (m *Manager) GetShareLink(token string) (*ShareLink, error) {
	return m.GetShareLinkByToken(token)
}

// RecordAccess 记录访问
func (m *Manager) RecordAccess(id, action, path, ip, userAgent, referrer string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if link, ok := m.links[id]; ok {
		link.AccessCount++
		now := time.Now()
		link.LastAccessAt = &now
	}
}

// VerifySharePassword 验证分享密码
func (m *Manager) VerifySharePassword(token, password string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, link := range m.links {
		if link.Token == token && link.IsActive {
			if link.Password == "" {
				return true
			}
			hash := sha256.Sum256([]byte(password))
			return hex.EncodeToString(hash[:]) == link.Password
		}
	}
	return false
}

// IsRunning 检查服务是否运行
func (m *Manager) IsRunning() bool {
	return true
}

// GetShareStats 获取分享统计
func (m *Manager) GetShareStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalLinks := len(m.links)
	activeLinks := 0
	totalAccess := 0
	totalDownloads := 0

	for _, link := range m.links {
		if link.IsActive {
			activeLinks++
		}
		totalAccess += link.AccessCount
		totalDownloads += link.DownloadCount
	}

	return map[string]interface{}{
		"totalLinks":     totalLinks,
		"activeLinks":    activeLinks,
		"totalAccess":    totalAccess,
		"totalDownloads": totalDownloads,
	}
}

// Search 搜索文件
func (m *Manager) Search(query string) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Entry
	queryLower := strings.ToLower(query)

	filepath.Walk(m.config.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		name := info.Name()
		if strings.Contains(strings.ToLower(name), queryLower) {
			relPath, _ := filepath.Rel(m.config.RootPath, path)
			entryType := FileTypeFile
			if info.IsDir() {
				entryType = FileTypeDirectory
			}

			results = append(results, Entry{
				Name:         name,
				Path:         relPath,
				AbsolutePath: path,
				Type:         entryType,
				Size:         info.Size(),
				ModTime:      info.ModTime(),
				IsHidden:     strings.HasPrefix(name, "."),
				Extension:    filepath.Ext(name),
				MimeType:     getMimeType(name),
				Permission:   info.Mode().String(),
				Protocol:     ProtocolLocal,
			})

			if len(results) >= 100 {
				return filepath.SkipDir
			}
		}

		return nil
	})

	return results
}

// ListSnapshots 列出快照（模拟实现）
func (m *Manager) ListSnapshots(path string) ([]Snapshot, error) {
	// 快照功能需要底层文件系统支持（如 ZFS/Btrfs）
	return []Snapshot{}, nil
}

// 辅助函数

func getMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeTypes := map[string]string{
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".pdf":  "application/pdf",
		".zip":  "application/zip",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".go":   "text/x-go",
	}
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// getFileExtension 获取文件扩展名
func getFileExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

func sortEntries(entries []Entry, sortBy SortField, sortDir SortDirection) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			swap := false
			switch sortBy {
			case SortByName:
				if sortDir == SortAsc {
					swap = entries[j].Name < entries[i].Name
				} else {
					swap = entries[j].Name > entries[i].Name
				}
			case SortBySize:
				if sortDir == SortAsc {
					swap = entries[j].Size < entries[i].Size
				} else {
					swap = entries[j].Size > entries[i].Size
				}
			case SortByModTime:
				if sortDir == SortAsc {
					swap = entries[j].ModTime.Before(entries[i].ModTime)
				} else {
					swap = entries[j].ModTime.After(entries[i].ModTime)
				}
			}
			if swap {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := randomBytes(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateID() string {
	b := make([]byte, 8)
	randomBytes(b)
	return hex.EncodeToString(b)
}

func randomBytes(b []byte) ([]byte, error) {
	for i := range b {
		b[i] = byte(time.Now().UnixNano() % 256)
	}
	return b, nil
}

func copyRecursive(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, _ := filepath.Rel(src, path)
			destPath := filepath.Join(dest, relPath)
			if fi.IsDir() {
				return os.MkdirAll(destPath, fi.Mode())
			}
			return copyFile(path, destPath)
		})
	}

	return copyFile(src, filepath.Join(dest, filepath.Base(src)))
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}
