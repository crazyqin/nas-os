// Package webshare 提供 WebShare 文件浏览器功能
// 纯浏览器文件访问，无需 SMB/NFS 客户端
// 参考: TrueNAS 26 WebShare with TrueSearch, Synology Photos UI
package webshare

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ================== 数据结构定义 ==================

// WebShareConfig WebShare 配置
type WebShareConfig struct {
	BaseDir          string        `json:"baseDir"`          // 基础目录
	MaxFileSize      int64         `json:"maxFileSize"`      // 最大上传文件大小
	AllowedExts      []string      `json:"allowedExts"`      // 允许的扩展名
	IndexExcluded    []string      `json:"indexExcluded"`    // 排除索引的路径
	EnableSnapshot   bool          `json:"enableSnapshot"`   // 启用快照时间线
	EnableShareLink  bool          `json:"enableShareLink"`  // 启用分享链接
	ShareLinkExpiry  time.Duration `json:"shareLinkExpiry"`  // 分享链接默认过期时间
	EnableHiddenShow bool          `json:"enableHiddenShow"` // 允许显示隐藏文件
	CacheDir         string        `json:"cacheDir"`         // 缓存目录
}

// ShareLink 分享链接
type ShareLink struct {
	ID           string    `json:"id"`
	Token        string    `json:"token"`
	Path         string    `json:"path"`
	Password     string    `json:"password,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
	CreatedBy    string    `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
	AllowUpload  bool      `json:"allowUpload"`
	AllowDelete  bool      `json:"allowDelete"`
	MaxDownloads int       `json:"maxDownloads"`
	Downloads    int       `json:"downloads"`
	Views        int       `json:"views"`
	IsPublic     bool      `json:"isPublic"`
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	CreatedAt   time.Time `json:"createdAt"`
	Size        int64     `json:"size"`
	Description string    `json:"description"`
	IsReadOnly  bool      `json:"isReadOnly"`
}

// FileItem 文件项
type FileItem struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	RelativePath string    `json:"relativePath"`
	Size         int64     `json:"size"`
	IsDir        bool      `json:"isDir"`
	ModTime      time.Time `json:"modTime"`
	Type         string    `json:"type"` // file, dir
	MimeType     string    `json:"mimeType"`
	Extension    string    `json:"extension"`
	IsHidden     bool      `json:"isHidden"`
	IsReadOnly   bool      `json:"isReadOnly"`
	Thumbnail    string    `json:"thumbnail"` // 缩略图 URL
	HasSnapshot  bool      `json:"hasSnapshot"`
}

// DirectoryInfo 目录信息
type DirectoryInfo struct {
	Path         string           `json:"path"`
	RelativePath string           `json:"relativePath"`
	Items        []FileItem       `json:"items"`
	TotalFiles   int              `json:"totalFiles"`
	TotalDirs    int              `json:"totalDirs"`
	TotalSize    int64            `json:"totalSize"`
	Snapshots    []SnapshotInfo   `json:"snapshots,omitempty"`
	Breadcrumb   []BreadcrumbItem `json:"breadcrumb"`
}

// BreadcrumbItem 面包屑项
type BreadcrumbItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// UploadProgress 上传进度
type UploadProgress struct {
	ID        string    `json:"id"`
	FileName  string    `json:"fileName"`
	Size      int64     `json:"size"`
	Uploaded  int64     `json:"uploaded"`
	Progress  float64   `json:"progress"`
	Status    string    `json:"status"` // uploading, completed, failed
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
}

// ================== WebShare 管理器 ==================

// Manager WebShare 管理器
type Manager struct {
	config      WebShareConfig
	mu          sync.RWMutex
	shareLinks  map[string]*ShareLink      // token -> ShareLink
	snapshots   map[string][]SnapshotInfo  // path -> snapshots
	uploadQueue map[string]*UploadProgress // id -> progress
	fileManager *FileManager
	searchIndex *SearchIndex
}

// NewManager 创建 WebShare 管理器
func NewManager(config WebShareConfig) *Manager {
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 10 * 1024 * 1024 * 1024 // 10GB
	}
	if config.ShareLinkExpiry == 0 {
		config.ShareLinkExpiry = 7 * 24 * time.Hour // 7天
	}
	if config.CacheDir == "" {
		config.CacheDir = "/tmp/nas-os/webshare"
	}

	// 创建缓存目录
	if err := os.MkdirAll(config.CacheDir, 0750); err != nil {
		log.Printf("创建缓存目录失败: %v", err)
	}

	m := &Manager{
		config:      config,
		shareLinks:  make(map[string]*ShareLink),
		snapshots:   make(map[string][]SnapshotInfo),
		uploadQueue: make(map[string]*UploadProgress),
		fileManager: NewFileManager(config),
		searchIndex: NewSearchIndex(config),
	}

	// 加载已有分享链接
	m.loadShareLinks()

	return m
}

// loadShareLinks 加载分享链接
func (m *Manager) loadShareLinks() {
	// 从持久化存储加载
	shareFile := filepath.Join(m.config.CacheDir, "share_links.json")
	data, err := os.ReadFile(shareFile)
	if err != nil {
		return // 首次运行，无需加载
	}

	var links []*ShareLink
	if err := json.Unmarshal(data, &links); err != nil {
		log.Printf("解析分享链接失败: %v", err)
		return
	}

	m.mu.Lock()
	for _, link := range links {
		m.shareLinks[link.Token] = link
	}
	m.mu.Unlock()
}

// saveShareLinks 保存分享链接
func (m *Manager) saveShareLinks() {
	m.mu.RLock()
	links := make([]*ShareLink, 0, len(m.shareLinks))
	for _, link := range m.shareLinks {
		links = append(links, link)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		log.Printf("序列化分享链接失败: %v", err)
		return
	}

	shareFile := filepath.Join(m.config.CacheDir, "share_links.json")
	if err := os.WriteFile(shareFile, data, 0600); err != nil {
		log.Printf("保存分享链接失败: %v", err)
	}
}

// ================== 目录操作 ==================

// ListDirectory 列出目录内容
func (m *Manager) ListDirectory(path string, showHidden bool, sortBy string, sortDesc bool) (*DirectoryInfo, error) {
	// 安全验证路径
	absPath, err := m.sanitizePath(path)
	if err != nil {
		return nil, err
	}

	// 读取目录
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	info := &DirectoryInfo{
		Path:         absPath,
		RelativePath: path,
		Items:        make([]FileItem, 0),
		Breadcrumb:   m.buildBreadcrumb(path),
	}

	for _, entry := range entries {
		// 隐藏文件过滤
		if !showHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(absPath, entry.Name())
		fileInfo, err := entry.Info()
		if err != nil {
			continue // 跳过无法访问的文件
		}

		item := FileItem{
			Name:         entry.Name(),
			Path:         fullPath,
			RelativePath: filepath.Join(path, entry.Name()),
			Size:         fileInfo.Size(),
			IsDir:        entry.IsDir(),
			ModTime:      fileInfo.ModTime(),
			IsHidden:     strings.HasPrefix(entry.Name(), "."),
			IsReadOnly:   fileInfo.Mode().Perm()&0200 == 0,
			Extension:    strings.ToLower(filepath.Ext(entry.Name())),
		}

		if !entry.IsDir() {
			item.Type = "file"
			item.MimeType = m.fileManager.GetMimeType(entry.Name())
			// 生成缩略图 URL (异步)
			if m.fileManager.IsImage(entry.Name()) {
				item.Thumbnail = fmt.Sprintf("/api/v1/webshare/thumb/%s", base64.URLEncoding.EncodeToString([]byte(item.RelativePath)))
			}
		} else {
			item.Type = "dir"
		}

		// 检查是否有快照
		m.mu.RLock()
		item.HasSnapshot = len(m.snapshots[fullPath]) > 0
		m.mu.RUnlock()

		info.Items = append(info.Items, item)

		if entry.IsDir() {
			info.TotalDirs++
		} else {
			info.TotalFiles++
			info.TotalSize += fileInfo.Size()
		}
	}

	// 排序
	m.sortItems(info.Items, sortBy, sortDesc)

	// 获取快照列表
	m.mu.RLock()
	info.Snapshots = m.snapshots[absPath]
	m.mu.RUnlock()

	return info, nil
}

// buildBreadcrumb 构建面包屑
func (m *Manager) buildBreadcrumb(path string) []BreadcrumbItem {
	breadcrumb := []BreadcrumbItem{
		{Name: "根目录", Path: "/"},
	}

	if path == "/" || path == "" {
		return breadcrumb
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	currentPath := ""
	for _, part := range parts {
		currentPath = currentPath + "/" + part
		breadcrumb = append(breadcrumb, BreadcrumbItem{
			Name: part,
			Path: currentPath,
		})
	}

	return breadcrumb
}

// sortItems 排序文件项
func (m *Manager) sortItems(items []FileItem, sortBy string, sortDesc bool) {
	sortField := sortBy
	if sortField == "" {
		sortField = "name"
	}

	// 简单排序实现
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			var shouldSwap bool
			switch sortField {
			case "name":
				shouldSwap = items[i].Name > items[j].Name
			case "size":
				shouldSwap = items[i].Size > items[j].Size
			case "time":
				shouldSwap = items[i].ModTime.After(items[j].ModTime)
			case "type":
				// 目录优先
				if items[i].IsDir && !items[j].IsDir {
					shouldSwap = false
				} else if !items[i].IsDir && items[j].IsDir {
					shouldSwap = true
				} else {
					shouldSwap = items[i].Extension > items[j].Extension
				}
			}

			if sortDesc {
				shouldSwap = !shouldSwap
			}

			if shouldSwap {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// sanitizePath 安全验证路径
func (m *Manager) sanitizePath(userPath string) (string, error) {
	baseDir := filepath.Clean(m.config.BaseDir)
	if baseDir == "" {
		baseDir = "/"
	}

	cleaned := filepath.Clean(filepath.Join(baseDir, userPath))

	// 防止路径遍历
	if !strings.HasPrefix(cleaned, baseDir) && cleaned != baseDir {
		return "", fmt.Errorf("路径遍历检测: 路径超出基础目录")
	}

	return cleaned, nil
}

// ================== 文件操作 ==================

// CreateDirectory 创建目录
func (m *Manager) CreateDirectory(path string) error {
	absPath, err := m.sanitizePath(path)
	if err != nil {
		return err
	}

	return os.MkdirAll(absPath, 0755)
}

// DeleteFile 删除文件或目录
func (m *Manager) DeleteFile(path string) error {
	absPath, err := m.sanitizePath(path)
	if err != nil {
		return err
	}

	// 检查是否存在
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("文件不存在: %w", err)
	}

	return os.RemoveAll(absPath)
}

// MoveFile 移动文件或目录
func (m *Manager) MoveFile(srcPath, dstPath string) error {
	srcAbs, err := m.sanitizePath(srcPath)
	if err != nil {
		return err
	}

	dstAbs, err := m.sanitizePath(dstPath)
	if err != nil {
		return err
	}

	return os.Rename(srcAbs, dstAbs)
}

// CopyFile 复制文件或目录
func (m *Manager) CopyFile(srcPath, dstPath string) error {
	srcAbs, err := m.sanitizePath(srcPath)
	if err != nil {
		return err
	}

	dstAbs, err := m.sanitizePath(dstPath)
	if err != nil {
		return err
	}

	// 读取源文件
	srcFile, err := os.Open(srcAbs)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		// 复制目录
		return m.copyDirectory(srcAbs, dstAbs)
	}

	// 创建目标文件
	dstFile, err := os.Create(dstAbs)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 复制内容
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyDirectory 复制目录
func (m *Manager) copyDirectory(src, dst string) error {
	// 创建目标目录
	if err := os.MkdirAll(dst, 0755); err != nil {
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
			if err := m.copyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := m.CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// ================== 分享链接 ==================

// CreateShareLink 创建分享链接
func (m *Manager) CreateShareLink(path string, password string, expiryHours int, allowUpload, allowDelete bool, createdBy string) (*ShareLink, error) {
	absPath, err := m.sanitizePath(path)
	if err != nil {
		return nil, err
	}

	// 验证路径存在
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("路径不存在: %w", err)
	}

	// 生成令牌
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	expiry := time.Now().Add(time.Duration(expiryHours) * time.Hour)
	if expiryHours == 0 {
		expiry = time.Now().Add(m.config.ShareLinkExpiry)
	}

	link := &ShareLink{
		ID:           fmt.Sprintf("share-%d", time.Now().UnixNano()),
		Token:        token,
		Path:         absPath,
		Password:     password,
		ExpiresAt:    expiry,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now(),
		AllowUpload:  allowUpload,
		AllowDelete:  allowDelete,
		MaxDownloads: 0, // 无限制
		IsPublic:     password == "",
	}

	m.mu.Lock()
	m.shareLinks[token] = link
	m.mu.Unlock()

	m.saveShareLinks()

	return link, nil
}

// GetShareLink 获取分享链接
func (m *Manager) GetShareLink(token string) (*ShareLink, error) {
	m.mu.RLock()
	link, ok := m.shareLinks[token]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("分享链接不存在")
	}

	// 检查过期
	if time.Now().After(link.ExpiresAt) {
		m.mu.Lock()
		delete(m.shareLinks, token)
		m.mu.Unlock()
		m.saveShareLinks()
		return nil, fmt.Errorf("分享链接已过期")
	}

	return link, nil
}

// DeleteShareLink 删除分享链接
func (m *Manager) DeleteShareLink(token string) error {
	m.mu.Lock()
	delete(m.shareLinks, token)
	m.mu.Unlock()

	m.saveShareLinks()
	return nil
}

// ListShareLinks 列出所有分享链接
func (m *Manager) ListShareLinks(createdBy string) []*ShareLink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	links := make([]*ShareLink, 0)
	for _, link := range m.shareLinks {
		if createdBy == "" || link.CreatedBy == createdBy {
			links = append(links, link)
		}
	}

	return links
}

// ================== 快照时间线 ==================

// GetSnapshots 获取路径的快照列表
func (m *Manager) GetSnapshots(path string) ([]SnapshotInfo, error) {
	absPath, err := m.sanitizePath(path)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	snapshots := m.snapshots[absPath]
	m.mu.RUnlock()

	return snapshots, nil
}

// AddSnapshot 添加快照记录
func (m *Manager) AddSnapshot(path, name, description string) (*SnapshotInfo, error) {
	sanitizedPath, err := m.sanitizePath(path)
	if err != nil {
		return nil, err
	}

	snapshot := SnapshotInfo{
		ID:          fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		Name:        name,
		Path:        sanitizedPath,
		CreatedAt:   time.Now(),
		Description: description,
		IsReadOnly:  true,
	}

	// 计算大小
	if info, err := os.Stat(sanitizedPath); err == nil {
		snapshot.Size = info.Size()
	}

	m.mu.Lock()
	m.snapshots[sanitizedPath] = append(m.snapshots[sanitizedPath], snapshot)
	m.mu.Unlock()

	return &snapshot, nil
}

// ================== 上传管理 ==================

// StartUpload 开始上传
func (m *Manager) StartUpload(fileName string, size int64, targetPath string) (*UploadProgress, error) {
	// 验证目标路径有效性
	if _, err := m.sanitizePath(targetPath); err != nil {
		return nil, err
	}

	// 检查文件大小限制
	if size > m.config.MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制 (%d > %d)", size, m.config.MaxFileSize)
	}

	progress := &UploadProgress{
		ID:        fmt.Sprintf("upload-%d", time.Now().UnixNano()),
		FileName:  fileName,
		Size:      size,
		Uploaded:  0,
		Progress:  0,
		Status:    "uploading",
		StartedAt: time.Now(),
	}

	m.mu.Lock()
	m.uploadQueue[progress.ID] = progress
	m.mu.Unlock()

	return progress, nil
}

// UpdateUploadProgress 更新上传进度
func (m *Manager) UpdateUploadProgress(id string, uploaded int64) {
	m.mu.Lock()
	if progress, ok := m.uploadQueue[id]; ok {
		progress.Uploaded = uploaded
		progress.Progress = float64(uploaded) / float64(progress.Size) * 100
	}
	m.mu.Unlock()
}

// CompleteUpload 完成上传
func (m *Manager) CompleteUpload(id string, filePath string) {
	m.mu.Lock()
	if progress, ok := m.uploadQueue[id]; ok {
		progress.Status = "completed"
		progress.EndedAt = time.Now()
		progress.Progress = 100
	}
	m.mu.Unlock()
}

// FailUpload 上传失败
func (m *Manager) FailUpload(id string, errMsg string) {
	m.mu.Lock()
	if progress, ok := m.uploadQueue[id]; ok {
		progress.Status = "failed"
		progress.Error = errMsg
		progress.EndedAt = time.Now()
	}
	m.mu.Unlock()
}

// GetUploadProgress 获取上传进度
func (m *Manager) GetUploadProgress(id string) *UploadProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.uploadQueue[id]
}

// ================== 搜索功能 ==================

// SearchFiles 搜索文件
func (m *Manager) SearchFiles(query string, path string, fileType string, minSize, maxSize int64) ([]FileItem, error) {
	return m.searchIndex.Search(query, path, fileType, minSize, maxSize)
}

// ================== HTTP 处理器 ==================

// HTTPHandler WebShare HTTP 处理器
type HTTPHandler struct {
	manager *Manager
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(manager *Manager) *HTTPHandler {
	return &HTTPHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 目录操作
	r.GET("/list", h.ListDirectory)
	r.POST("/mkdir", h.CreateDirectory)
	r.DELETE("/delete", h.DeleteFile)
	r.POST("/move", h.MoveFile)
	r.POST("/copy", h.CopyFile)

	// 上传下载
	r.POST("/upload", h.UploadFile)
	r.GET("/download/*path", h.DownloadFile)
	r.GET("/thumb/*path", h.GetThumbnail)

	// 分享链接
	r.POST("/share/create", h.CreateShareLink)
	r.GET("/share/list", h.ListShareLinks)
	r.GET("/share/:token", h.GetShareLink)
	r.DELETE("/share/:token", h.DeleteShareLink)
	r.GET("/public/:token", h.PublicAccess)

	// 快照
	r.GET("/snapshots/*path", h.GetSnapshots)
	r.POST("/snapshots/create", h.CreateSnapshot)

	// 搜索
	r.GET("/search", h.SearchFiles)

	// 上传进度
	r.GET("/upload/progress/:id", h.GetUploadProgress)
}

// ListDirectory 列出目录
func (h *HTTPHandler) ListDirectory(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	showHidden := c.Query("hidden") == "true"
	sortBy := c.DefaultQuery("sort", "name")
	sortDesc := c.DefaultQuery("order", "desc") == "desc"

	info, err := h.manager.ListDirectory(path, showHidden, sortBy, sortDesc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// CreateDirectory 创建目录
func (h *HTTPHandler) CreateDirectory(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径参数缺失"})
		return
	}

	if err := h.manager.CreateDirectory(req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "目录创建成功"})
}

// DeleteFile 删除文件
func (h *HTTPHandler) DeleteFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径参数缺失"})
		return
	}

	if err := h.manager.DeleteFile(path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// MoveFile 移动文件
func (h *HTTPHandler) MoveFile(c *gin.Context) {
	var req struct {
		Src string `json:"src" binding:"required"`
		Dst string `json:"dst" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数缺失"})
		return
	}

	if err := h.manager.MoveFile(req.Src, req.Dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "移动成功"})
}

// CopyFile 复制文件
func (h *HTTPHandler) CopyFile(c *gin.Context) {
	var req struct {
		Src string `json:"src" binding:"required"`
		Dst string `json:"dst" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数缺失"})
		return
	}

	if err := h.manager.CopyFile(req.Src, req.Dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "复制成功"})
}

// UploadFile 上传文件
func (h *HTTPHandler) UploadFile(c *gin.Context) {
	targetPath := c.Query("path")
	if targetPath == "" {
		targetPath = "/"
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件参数缺失"})
		return
	}
	defer file.Close()

	// 开始上传
	progress, err := h.manager.StartUpload(header.Filename, header.Size, targetPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 写入文件
	absPath, err := h.manager.sanitizePath(filepath.Join(targetPath, header.Filename))
	if err != nil {
		h.manager.FailUpload(progress.ID, err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dstFile, err := os.Create(absPath)
	if err != nil {
		h.manager.FailUpload(progress.ID, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dstFile.Close()

	// 复制数据并更新进度
	buf := make([]byte, 32*1024)
	var uploaded int64
	for {
		n, err := file.Read(buf)
		if n > 0 {
			dstFile.Write(buf[:n])
			uploaded += int64(n)
			h.manager.UpdateUploadProgress(progress.ID, uploaded)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			h.manager.FailUpload(progress.ID, err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	h.manager.CompleteUpload(progress.ID, absPath)

	c.JSON(http.StatusOK, gin.H{
		"message": "上传成功",
		"path":    filepath.Join(targetPath, header.Filename),
		"id":      progress.ID,
	})
}

// DownloadFile 下载文件
func (h *HTTPHandler) DownloadFile(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径参数缺失"})
		return
	}

	absPath, err := h.manager.sanitizePath(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否是目录
	info, err := os.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	if info.IsDir() {
		// 目录下载 - 创建 ZIP
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", filepath.Base(absPath)))
		// TODO: 实现 ZIP 打包下载
		return
	}

	// 文件下载
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(absPath)))
	c.Header("Content-Type", h.manager.fileManager.GetMimeType(filepath.Base(absPath)))
	c.File(absPath)
}

// GetThumbnail 获取缩略图
func (h *HTTPHandler) GetThumbnail(c *gin.Context) {
	encodedPath := c.Param("path")
	if encodedPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径参数缺失"})
		return
	}

	// 解码路径
	pathBytes, err := base64.URLEncoding.DecodeString(strings.TrimPrefix(encodedPath, "/"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径解码失败"})
		return
	}
	path := string(pathBytes)

	absPath, err := h.manager.sanitizePath(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成缩略图
	thumbPath, err := h.manager.fileManager.GenerateThumbnail(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.File(thumbPath)
}

// CreateShareLink 创建分享链接
func (h *HTTPHandler) CreateShareLink(c *gin.Context) {
	var req struct {
		Path        string `json:"path" binding:"required"`
		Password    string `json:"password"`
		ExpiryHours int    `json:"expiryHours"`
		AllowUpload bool   `json:"allowUpload"`
		AllowDelete bool   `json:"allowDelete"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数缺失"})
		return
	}

	// 从上下文获取用户
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)

	link, err := h.manager.CreateShareLink(req.Path, req.Password, req.ExpiryHours, req.AllowUpload, req.AllowDelete, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     link.Token,
		"expiresAt": link.ExpiresAt,
		"url":       fmt.Sprintf("/public/%s", link.Token),
	})
}

// ListShareLinks 列出分享链接
func (h *HTTPHandler) ListShareLinks(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)

	links := h.manager.ListShareLinks(userIDStr)
	c.JSON(http.StatusOK, gin.H{"links": links})
}

// GetShareLink 获取分享链接详情
func (h *HTTPHandler) GetShareLink(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "令牌参数缺失"})
		return
	}

	link, err := h.manager.GetShareLink(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, link)
}

// DeleteShareLink 删除分享链接
func (h *HTTPHandler) DeleteShareLink(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "令牌参数缺失"})
		return
	}

	if err := h.manager.DeleteShareLink(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// PublicAccess 公开访问分享链接
func (h *HTTPHandler) PublicAccess(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "令牌参数缺失"})
		return
	}

	link, err := h.manager.GetShareLink(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 密码验证
	if link.Password != "" {
		password := c.Query("password")
		if password != link.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
			return
		}
	}

	// 更新访问计数
	m := h.manager
	m.mu.Lock()
	link.Views++
	m.mu.Unlock()
	m.saveShareLinks()

	// 列出分享内容
	subPath := c.Query("path")
	fullPath := filepath.Join(link.Path, subPath)

	showHidden := c.Query("hidden") == "true"
	info, err := h.manager.ListDirectory(fullPath, showHidden, "name", false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"link":    link,
		"content": info,
	})
}

// GetSnapshots 获取快照列表
func (h *HTTPHandler) GetSnapshots(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		path = "/"
	}

	snapshots, err := h.manager.GetSnapshots(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"snapshots": snapshots})
}

// CreateSnapshot 创建快照
func (h *HTTPHandler) CreateSnapshot(c *gin.Context) {
	var req struct {
		Path        string `json:"path" binding:"required"`
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数缺失"})
		return
	}

	snapshot, err := h.manager.AddSnapshot(req.Path, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

// SearchFiles 搜索文件
func (h *HTTPHandler) SearchFiles(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索关键词缺失"})
		return
	}

	path := c.Query("path")
	fileType := c.Query("type")
	minSize := int64(c.GetInt("minSize"))
	maxSize := int64(c.GetInt("maxSize"))

	results, err := h.manager.SearchFiles(query, path, fileType, int64(minSize), int64(maxSize))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"total":   len(results),
		"results": results,
	})
}

// GetUploadProgress 获取上传进度
func (h *HTTPHandler) GetUploadProgress(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 参数缺失"})
		return
	}

	progress := h.manager.GetUploadProgress(id)
	if progress == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "上传任务不存在"})
		return
	}

	c.JSON(http.StatusOK, progress)
}
