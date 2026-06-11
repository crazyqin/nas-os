// Package webshare 核心管理器，实现 WebShare 服务的生命周期管理、
// 文件系统操作、分享链接管理、访问控制和 FIPS 加密传输。
package webshare

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager WebShare 核心管理器
type Manager struct {
	mu          sync.RWMutex
	config      *WebShareConfig
	links       map[string]*ShareLink     // 分享链接映射
	accessLogs  []AccessLog               // 访问日志
	filterCache *filterCache              // 目录过滤缓存
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
	startTime   time.Time
}

// NewManager 创建 WebShare 管理器
func NewManager(config *WebShareConfig) *Manager {
	if config == nil {
		config = DefaultWebShareConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:      config,
		links:       make(map[string]*ShareLink),
		accessLogs:  make([]AccessLog, 0),
		filterCache: newFilterCache(30 * time.Second),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动 WebShare 服务
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("WebShare 管理器已在运行")
	}

	// 验证根目录是否存在
	if _, err := os.Stat(m.config.RootPath); os.IsNotExist(err) {
		return fmt.Errorf("根目录不存在: %s", m.config.RootPath)
	}

	m.running = true
	m.startTime = time.Now()

	// 启动清理过期链接的后台协程
	go m.cleanupExpiredLinks()

	log.Printf("[WebShare] 服务启动成功，根目录: %s", m.config.RootPath)
	return nil
}

// Stop 停止 WebShare 服务
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.cancel()
	m.running = false

	log.Println("[WebShare] 服务已停止")
	return nil
}

// IsRunning 检查服务是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *WebShareConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *WebShareConfig) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	m.filterCache.invalidateAll()

	log.Println("[WebShare] 配置已更新")
	return nil
}

// ==================== 目录浏览 ====================

// ListDirectory 列出目录内容
func (m *Manager) ListDirectory(path string, showHidden bool, filter string, sortBy SortField, sortDir SortDirection) (*DirectoryListing, error) {
	if !m.IsRunning() {
		return nil, fmt.Errorf("WebShare 管理器未运行")
	}

	// 清理并验证路径
	cleanPath := m.sanitizePath(path)
	absPath := filepath.Join(m.config.RootPath, cleanPath)

	// 安全检查：防止路径遍历攻击
	if !m.isPathSafe(absPath) {
		return nil, fmt.Errorf("无效的路径: %s", path)
	}

	// 检查目录是否存在
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("目录不存在: %s", path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", path)
	}

	// 读取目录内容
	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	// 构建条目列表
	entries := make([]Entry, 0, len(dirEntries))
	var totalSize int64

	for _, de := range dirEntries {
		name := de.Name()

		// 处理隐藏文件
		isHidden := strings.HasPrefix(name, ".")
		if isHidden && !showHidden {
			continue
		}

		// 文件名过滤
		if filter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
			continue
		}

		info, err := de.Info()
		if err != nil {
			continue // 跳过无法读取的文件
		}

		relPath := filepath.Join(cleanPath, name)
		if cleanPath == "" || cleanPath == "." {
			relPath = name
		}

		entry := Entry{
			Name:         name,
			Path:         relPath,
			AbsolutePath: filepath.Join(absPath, name),
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			IsHidden:     isHidden,
			Extension:    getFileExtension(name),
			MimeType:     getMimeType(name),
			Permission:   info.Mode().String(),
		}

		// 判断文件类型
		if de.IsDir() {
			entry.Type = FileTypeDirectory
		} else if de.Type()&os.ModeSymlink != 0 {
			entry.Type = FileTypeSymlink
			if target, err := os.Readlink(filepath.Join(absPath, name)); err == nil {
				entry.SymlinkTarget = target
			}
		} else {
			entry.Type = FileTypeFile
			totalSize += info.Size()
		}

		entries = append(entries, entry)
	}

	// 排序
	sortEntries(entries, sortBy, sortDir)

	// 计算父目录路径
	parentPath := filepath.Dir(cleanPath)
	if parentPath == "." {
		parentPath = ""
	}

	return &DirectoryListing{
		Path:       cleanPath,
		ParentPath: parentPath,
		Entries:    entries,
		TotalCount: len(entries),
		TotalSize:  totalSize,
		ShowHidden: showHidden,
		FilteredBy: filter,
	}, nil
}

// ==================== 文件操作 ====================

// CreateFolder 创建文件夹
func (m *Manager) CreateFolder(path, name string) error {
	if !m.IsRunning() {
		return fmt.Errorf("WebShare 管理器未运行")
	}

	if name == "" {
		return fmt.Errorf("文件夹名称不能为空")
	}

	// 验证文件夹名称
	if err := validateFileName(name); err != nil {
		return err
	}

	parentPath := m.sanitizePath(path)
	absParentPath := filepath.Join(m.config.RootPath, parentPath)
	absPath := filepath.Join(absParentPath, name)

	if !m.isPathSafe(absPath) {
		return fmt.Errorf("无效的路径")
	}

	// 检查父目录是否存在
	if _, err := os.Stat(absParentPath); os.IsNotExist(err) {
		return fmt.Errorf("父目录不存在: %s", path)
	}

	// 创建文件夹
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("创建文件夹失败: %w", err)
	}

	// 失效缓存
	m.filterCache.invalidate(parentPath)

	log.Printf("[WebShare] 创建文件夹: %s", absPath)
	return nil
}

// DeleteEntries 删除文件/目录
func (m *Manager) DeleteEntries(paths []string) error {
	if !m.IsRunning() {
		return fmt.Errorf("WebShare 管理器未运行")
	}

	if len(paths) == 0 {
		return fmt.Errorf("路径列表不能为空")
	}

	var lastErr error
	for _, p := range paths {
		absPath := filepath.Join(m.config.RootPath, m.sanitizePath(p))

		if !m.isPathSafe(absPath) {
			lastErr = fmt.Errorf("无效的路径: %s", p)
			continue
		}

		// 使用 os.RemoveAll 安全删除
		if err := os.RemoveAll(absPath); err != nil {
			lastErr = fmt.Errorf("删除失败 %s: %w", p, err)
			log.Printf("[WebShare] 删除失败: %s, 错误: %v", absPath, err)
		} else {
			log.Printf("[WebShare] 已删除: %s", absPath)
			// 失效相关缓存
			m.filterCache.invalidate(filepath.Dir(m.sanitizePath(p)))
		}
	}

	return lastErr
}

// MoveEntries 移动/重命名文件/目录
func (m *Manager) MoveEntries(paths []string, dest string) error {
	if !m.IsRunning() {
		return fmt.Errorf("WebShare 管理器未运行")
	}

	destPath := m.sanitizePath(dest)
	absDest := filepath.Join(m.config.RootPath, destPath)

	if !m.isPathSafe(absDest) {
		return fmt.Errorf("无效的目标路径: %s", dest)
	}

	for _, p := range paths {
		srcPath := m.sanitizePath(p)
		absSrc := filepath.Join(m.config.RootPath, srcPath)

		if !m.isPathSafe(absSrc) {
			return fmt.Errorf("无效的源路径: %s", p)
		}

		if err := os.Rename(absSrc, absDest); err != nil {
			return fmt.Errorf("移动失败 %s -> %s: %w", p, dest, err)
		}

		log.Printf("[WebShare] 移动: %s -> %s", absSrc, absDest)
	}

	// 失效缓存
	m.filterCache.invalidateAll()
	return nil
}

// RenameEntry 重命名文件/目录
func (m *Manager) RenameEntry(oldPath, newName string) error {
	if !m.IsRunning() {
		return fmt.Errorf("WebShare 管理器未运行")
	}

	if err := validateFileName(newName); err != nil {
		return err
	}

	sanitizedOld := m.sanitizePath(oldPath)
	absOld := filepath.Join(m.config.RootPath, sanitizedOld)

	if !m.isPathSafe(absOld) {
		return fmt.Errorf("无效的路径: %s", oldPath)
	}

	// 检查源文件是否存在
	if _, err := os.Stat(absOld); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", oldPath)
	}

	// 构建新路径
	absNew := filepath.Join(filepath.Dir(absOld), newName)
	if !m.isPathSafe(absNew) {
		return fmt.Errorf("无效的新名称: %s", newName)
	}

	// 检查目标是否已存在
	if _, err := os.Stat(absNew); err == nil {
		return fmt.Errorf("目标已存在: %s", newName)
	}

	if err := os.Rename(absOld, absNew); err != nil {
		return fmt.Errorf("重命名失败: %w", err)
	}

	// 失效缓存
	m.filterCache.invalidate(filepath.Dir(sanitizedOld))

	log.Printf("[WebShare] 重命名: %s -> %s", absOld, absNew)
	return nil
}

// CopyEntries 复制文件/目录
func (m *Manager) CopyEntries(paths []string, dest string) error {
	if !m.IsRunning() {
		return fmt.Errorf("WebShare 管理器未运行")
	}

	destPath := m.sanitizePath(dest)
	absDest := filepath.Join(m.config.RootPath, destPath)

	if !m.isPathSafe(absDest) {
		return fmt.Errorf("无效的目标路径: %s", dest)
	}

	// 确保目标目录存在
	if err := os.MkdirAll(absDest, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	for _, p := range paths {
		srcPath := m.sanitizePath(p)
		absSrc := filepath.Join(m.config.RootPath, srcPath)

		if !m.isPathSafe(absSrc) {
			return fmt.Errorf("无效的源路径: %s", p)
		}

		srcInfo, err := os.Stat(absSrc)
		if err != nil {
			return fmt.Errorf("源文件不存在: %s", p)
		}

		destFile := filepath.Join(absDest, srcInfo.Name())

		if srcInfo.IsDir() {
			if err := copyDir(absSrc, destFile); err != nil {
				return fmt.Errorf("复制目录失败: %w", err)
			}
		} else {
			if err := copyFile(absSrc, destFile); err != nil {
				return fmt.Errorf("复制文件失败: %w", err)
			}
		}

		log.Printf("[WebShare] 复制: %s -> %s", absSrc, destFile)
	}

	// 失效缓存
	m.filterCache.invalidate(destPath)
	return nil
}

// ==================== 分享链接管理 ====================

// CreateShareLink 创建分享链接
func (m *Manager) CreateShareLink(req CreateShareLinkRequest) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, fmt.Errorf("WebShare 管理器未运行")
	}

	// 检查活跃链接数限制
	if len(m.links) >= m.config.MaxActiveLinks {
		return nil, fmt.Errorf("已达到最大活跃链接数 (%d)", m.config.MaxActiveLinks)
	}

	// 验证路径是否存在
	absPath := filepath.Join(m.config.RootPath, m.sanitizePath(req.Path))
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("路径不存在: %s", req.Path)
	}

	// 生成安全令牌
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("生成令牌失败: %w", err)
	}

	// 设置权限默认值
	permission := req.Permission
	if permission == "" {
		permission = PermissionView
	}

	// 设置过期时间
	var expiresAt *time.Time
	if req.Expiry > 0 {
		exp := time.Now().Add(req.Expiry)
		expiresAt = &exp
	} else if m.config.DefaultExpiry > 0 {
		exp := time.Now().Add(m.config.DefaultExpiry)
		expiresAt = &exp
	}

	// 如果设置了密码，进行哈希处理
	passwordHash := ""
	if req.Password != "" {
		passwordHash = hashPassword(req.Password)
	}

	link := &ShareLink{
		ID:           generateID(),
		Name:         req.Name,
		Path:         req.Path,
		Token:        token,
		Permission:   permission,
		Password:     passwordHash,
		MaxDownloads: req.MaxDownloads,
		CreatedBy:    req.CreatedBy,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		IsActive:     true,
		ExpiresAt:    expiresAt,
	}

	m.links[link.ID] = link

	log.Printf("[WebShare] 创建分享链接: %s -> %s (by %s)", link.ID, req.Path, req.CreatedBy)
	return link, nil
}

// GetShareLink 通过令牌获取分享链接
func (m *Manager) GetShareLink(token string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, link := range m.links {
		if link.Token == token && link.IsActive {
			// 检查是否过期
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				return nil, fmt.Errorf("分享链接已过期")
			}
			// 检查下载次数限制
			if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
				return nil, fmt.Errorf("已达到最大下载次数")
			}
			return link, nil
		}
	}

	return nil, fmt.Errorf("分享链接不存在")
}

// GetShareLinkByID 通过 ID 获取分享链接
func (m *Manager) GetShareLinkByID(id string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.links[id]
	if !exists {
		return nil, fmt.Errorf("分享链接不存在: %s", id)
	}

	return link, nil
}

// DeleteShareLink 删除分享链接
func (m *Manager) DeleteShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, exists := m.links[id]
	if !exists {
		return fmt.Errorf("分享链接不存在: %s", id)
	}

	link.IsActive = false
	link.UpdatedAt = time.Now()

	log.Printf("[WebShare] 删除分享链接: %s", id)
	return nil
}

// ListShareLinks 列出分享链接
func (m *Manager) ListShareLinks(createdBy string, includeInactive bool) []*ShareLink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ShareLink
	for _, link := range m.links {
		// 过滤创建者
		if createdBy != "" && link.CreatedBy != createdBy {
			continue
		}
		// 过滤非活跃链接
		if !includeInactive && !link.IsActive {
			continue
		}
		result = append(result, link)
	}

	// 按创建时间降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// UpdateShareLink 更新分享链接
func (m *Manager) UpdateShareLink(id string, updates map[string]interface{}) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, exists := m.links[id]
	if !exists {
		return nil, fmt.Errorf("分享链接不存在: %s", id)
	}

	if name, ok := updates["name"].(string); ok && name != "" {
		link.Name = name
	}
	if perm, ok := updates["permission"].(Permission); ok {
		link.Permission = perm
	}
	if password, ok := updates["password"].(string); ok {
		if password == "" {
			link.Password = ""
		} else {
			link.Password = hashPassword(password)
		}
	}
	if maxDL, ok := updates["max_downloads"].(int); ok {
		link.MaxDownloads = maxDL
	}

	link.UpdatedAt = time.Now()

	log.Printf("[WebShare] 更新分享链接: %s", id)
	return link, nil
}

// VerifySharePassword 验证分享链接密码
func (m *Manager) VerifySharePassword(token, password string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, link := range m.links {
		if link.Token == token && link.IsActive {
			if link.Password == "" {
				return true // 无密码保护
			}
			return hashPassword(password) == link.Password
		}
	}

	return false
}

// ==================== 访问日志 ====================

// RecordAccess 记录访问日志
func (m *Manager) RecordAccess(shareID, action, path, ip, userAgent, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	accessLog := AccessLog{
		ID:        generateID(),
		ShareID:   shareID,
		Action:    action,
		Path:      path,
		IP:        ip,
		UserAgent: userAgent,
		UserID:    userID,
		Timestamp: time.Now(),
	}

	m.accessLogs = append(m.accessLogs, accessLog)

	// 更新分享链接的下载计数和最后访问时间
	if link, exists := m.links[shareID]; exists {
		if action == "download" {
			link.DownloadCount++
		}
		link.AccessCount++
		now := time.Now()
		link.LastAccessAt = &now
		link.UpdatedAt = now
	}
}

// GetAccessLogs 获取访问日志
func (m *Manager) GetAccessLogs(shareID string, limit int) []AccessLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []AccessLog
	for _, log := range m.accessLogs {
		if shareID == "" || log.ShareID == shareID {
			result = append(result, log)
		}
	}

	// 按时间降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// ==================== 快照管理 ====================

// ListSnapshots 列出目录的快照（通过 .zfs/snapshot 或 BTRFS subvolume）
func (m *Manager) ListSnapshots(path string) ([]Snapshot, error) {
	if !m.IsRunning() {
		return nil, fmt.Errorf("WebShare 管理器未运行")
	}

	if !m.config.EnableSnapshots {
		return nil, fmt.Errorf("快照功能未启用")
	}

	sanitizedPath := m.sanitizePath(path)
	absPath := filepath.Join(m.config.RootPath, sanitizedPath)

	// 尝试 ZFS 快照路径
	zfsSnapshotPath := filepath.Join(absPath, ".zfs", "snapshot")
	if _, err := os.Stat(zfsSnapshotPath); err == nil {
		return m.listZFSSnapshots(zfsSnapshotPath)
	}

	// 无快照支持时返回空列表
	return []Snapshot{}, nil
}

// ==================== 统计信息 ====================

// GetStats 获取服务统计信息
func (m *Manager) GetStats() *WebShareStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &WebShareStats{
		ActiveLinks:  len(m.links),
		SearchEnabled: m.config.EnableSearch,
		FIPSEnabled:   m.config.EnableFIPS,
	}

	// 统计活跃链接
	for _, link := range m.links {
		if link.IsActive {
			stats.ActiveLinks++
		}
	}

	// 统计根目录信息
	m.collectDirStats(m.config.RootPath, stats)

	return stats
}

// GetShareStats 获取分享链接统计
func (m *Manager) GetShareStats() ShareLinkStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ShareLinkStats{
		TotalLinks: len(m.links),
	}

	for _, link := range m.links {
		if link.IsActive {
			stats.ActiveLinks++
			stats.TotalDownloads += link.DownloadCount
			stats.TotalAccesses += link.AccessCount
		}
	}

	return stats
}

// ==================== 工具方法 ====================

// sanitizePath 清理路径，防止路径遍历
func (m *Manager) sanitizePath(path string) string {
	// 清理路径
	cleaned := filepath.Clean(path)

	// 移除前导斜杠
	cleaned = strings.TrimPrefix(cleaned, "/")

	// 移除前导 "./"
	cleaned = strings.TrimPrefix(cleaned, "./")

	return cleaned
}

// isPathSafe 检查路径是否安全（防止路径遍历攻击）
func (m *Manager) isPathSafe(absPath string) bool {
	// 确保路径在根目录内
	relPath, err := filepath.Rel(m.config.RootPath, absPath)
	if err != nil {
		return false
	}

	// 检查是否试图逃出根目录
	if strings.HasPrefix(relPath, "..") {
		return false
	}

	return true
}

// cleanupExpiredLinks 定期清理过期的分享链接
func (m *Manager) cleanupExpiredLinks() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			for id, link := range m.links {
				if link.ExpiresAt != nil && link.ExpiresAt.Before(now) {
					link.IsActive = false
					link.UpdatedAt = now
					log.Printf("[WebShare] 分享链接已过期: %s", id)
				}
				// 检查下载次数限制
				if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
					link.IsActive = false
					link.UpdatedAt = now
					log.Printf("[WebShare] 分享链接达到下载上限: %s", id)
				}
			}
			m.mu.Unlock()
		}
	}
}

// collectDirStats 收集目录统计信息
func (m *Manager) collectDirStats(path string, stats *WebShareStats) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			stats.TotalDirs++
			m.collectDirStats(filepath.Join(path, entry.Name()), stats)
		} else {
			info, err := entry.Info()
			if err == nil {
				stats.TotalFiles++
				stats.TotalSize += info.Size()
			}
		}
	}
}

// listZFSSnapshots 列出 ZFS 快照
func (m *Manager) listZFSSnapshots(snapshotPath string) ([]Snapshot, error) {
	entries, err := os.ReadDir(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("读取快照目录失败: %w", err)
	}

	snapshots := make([]Snapshot, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		snapshots = append(snapshots, Snapshot{
			ID:        entry.Name(),
			Name:      entry.Name(),
			Path:      filepath.Join(snapshotPath, entry.Name()),
			CreatedAt: info.ModTime(),
			IsAuto:    true,
		})
	}

	// 按创建时间降序排序
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

// ==================== 辅助函数 ====================

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("ws_%d_%s", time.Now().UnixNano(), randomHex(4))
}

// generateSecureToken 生成安全令牌
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// hashPassword 使用 SHA-256 哈希密码
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// getFileExtension 获取文件扩展名
func getFileExtension(name string) string {
	ext := filepath.Ext(name)
	return strings.ToLower(ext)
}

// getMimeType 获取文件 MIME 类型
func getMimeType(name string) string {
	ext := filepath.Ext(name)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

// validateFileName 验证文件名是否合法
func validateFileName(name string) error {
	if name == "" {
		return fmt.Errorf("文件名不能为空")
	}

	// 检查是否包含非法字符
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, ch := range invalidChars {
		if strings.Contains(name, ch) {
			return fmt.Errorf("文件名包含非法字符: %s", ch)
		}
	}

	// 检查保留名称
	reserved := []string{".", "..", "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}

	upperName := strings.ToUpper(name)
	for _, r := range reserved {
		if upperName == r {
			return fmt.Errorf("文件名是系统保留名称: %s", name)
		}
	}

	return nil
}

// sortEntries 排序文件条目
func sortEntries(entries []Entry, sortBy SortField, sortDir SortDirection) {
	sort.Slice(entries, func(i, j int) bool {
		// 目录始终排在前面
		if entries[i].Type == FileTypeDirectory && entries[j].Type != FileTypeDirectory {
			return true
		}
		if entries[i].Type != FileTypeDirectory && entries[j].Type == FileTypeDirectory {
			return false
		}

		var result bool
		switch sortBy {
		case SortBySize:
			result = entries[i].Size < entries[j].Size
		case SortByModTime:
			result = entries[i].ModTime.Before(entries[j].ModTime)
		case SortByType:
			result = entries[i].Extension < entries[j].Extension
		default: // SortByName
			result = strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		}

		if sortDir == SortDesc {
			return !result
		}
		return result
	})
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
