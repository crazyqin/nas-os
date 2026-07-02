// Package filesharing 提供高级文件分享功能
// 包括分享链接管理、密码保护、过期管理、下载统计、批量分享、短链接、二维码生成、文件预览等
package filesharing

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ================== 数据结构定义 ==================

// SharePermission 分享权限.
type SharePermission string

const (
	PermissionView     SharePermission = "view"     // 查看
	PermissionDownload SharePermission = "download" // 下载
	PermissionUpload   SharePermission = "upload"   // 上传
	PermissionEdit     SharePermission = "edit"     // 编辑
)

// ShareStatus 分享状态.
type ShareStatus string

const (
	StatusActive       ShareStatus = "active"        // 活跃
	StatusExpired      ShareStatus = "expired"       // 已过期
	StatusRevoked      ShareStatus = "revoked"       // 已撤销
	StatusLimitReached ShareStatus = "limit_reached" // 达到下载限制
)

// ShareLink 分享链接.
type ShareLink struct {
	ID             string            `json:"id"`
	Path           string            `json:"path"`
	Token          string            `json:"token"`
	Password       string            `json:"password,omitempty"`
	ExpireAt       time.Time         `json:"expireAt"`
	MaxDownloads   int               `json:"maxDownloads"`
	DownloadCount  int               `json:"downloadCount"`
	CreatedBy      string            `json:"createdBy"`
	Permissions    []SharePermission `json:"permissions"`
	CustomSlug     string            `json:"customSlug,omitempty"`
	Status         ShareStatus       `json:"status"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
	Description    string            `json:"description,omitempty"`
	AllowUpload    bool              `json:"allowUpload"`
	UploadDir      string            `json:"uploadDir,omitempty"`
	ViewCount      int               `json:"viewCount"`
	LastAccessedAt *time.Time        `json:"lastAccessedAt,omitempty"`
	AccessLog      []AccessLogEntry  `json:"accessLog,omitempty"`
}

// AccessLogEntry 访问日志.
type AccessLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"userAgent"`
	Action    string    `json:"action"` // view, download, upload
}

// ShareStats 分享统计.
type ShareStats struct {
	TotalLinks       int         `json:"totalLinks"`
	ActiveLinks      int         `json:"activeLinks"`
	ExpiredLinks     int         `json:"expiredLinks"`
	RevokedLinks     int         `json:"revokedLinks"`
	TotalDownloads   int         `json:"totalDownloads"`
	TotalViews       int         `json:"totalViews"`
	TotalUploads     int         `json:"totalUploads"`
	AverageDownloads float64     `json:"averageDownloads"`
	TopLinks         []ShareLink `json:"topLinks,omitempty"`
}

// PreviewInfo 预览信息.
type PreviewInfo struct {
	FilePath    string    `json:"filePath"`
	FileName    string    `json:"fileName"`
	MimeType    string    `json:"mimeType"`
	PreviewType string    `json:"previewType"` // image, video, document, text, unsupported
	PreviewURL  string    `json:"previewURL,omitempty"`
	Thumbnail   string    `json:"thumbnail,omitempty"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modTime"`
}

// UploadItem 上传项.
type UploadItem struct {
	FileName   string    `json:"fileName"`
	Size       int64     `json:"size"`
	Path       string    `json:"path"`
	UploadedAt time.Time `json:"uploadedAt"`
	UploadedBy string    `json:"uploadedBy,omitempty"`
}

// FileSharingManager 文件分享管理器.
type FileSharingManager struct {
	mu          sync.RWMutex
	links       map[string]*ShareLink // token -> ShareLink
	linksByID   map[string]*ShareLink // id -> ShareLink
	linksBySlug map[string]*ShareLink // customSlug -> ShareLink
	baseURL     string
	dataDir     string
	maxFileSize int64
}

// NewFileSharingManager 创建文件分享管理器.
func NewFileSharingManager(baseURL, dataDir string, maxFileSize int64) *FileSharingManager {
	if maxFileSize == 0 {
		maxFileSize = 10 * 1024 * 1024 * 1024 // 10GB
	}

	m := &FileSharingManager{
		links:       make(map[string]*ShareLink),
		linksByID:   make(map[string]*ShareLink),
		linksBySlug: make(map[string]*ShareLink),
		baseURL:     baseURL,
		dataDir:     dataDir,
		maxFileSize: maxFileSize,
	}

	// 创建数据目录
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		log.Printf("创建数据目录失败: %v", err)
	}

	// 加载已有分享链接
	m.loadLinks()

	return m
}

// loadLinks 加载分享链接.
func (m *FileSharingManager) loadLinks() {
	linkFile := filepath.Join(m.dataDir, "share_links.json")
	data, err := os.ReadFile(linkFile)
	if err != nil {
		return
	}

	var links []*ShareLink
	if err := json.Unmarshal(data, &links); err != nil {
		log.Printf("解析分享链接失败: %v", err)
		return
	}

	m.mu.Lock()
	for _, link := range links {
		m.links[link.Token] = link
		m.linksByID[link.ID] = link
		if link.CustomSlug != "" {
			m.linksBySlug[link.CustomSlug] = link
		}
	}
	m.mu.Unlock()
}

// saveLinks 保存分享链接.
func (m *FileSharingManager) saveLinks() {
	m.mu.RLock()
	links := make([]*ShareLink, 0, len(m.links))
	for _, link := range m.links {
		links = append(links, link)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		log.Printf("序列化分享链接失败: %v", err)
		return
	}

	linkFile := filepath.Join(m.dataDir, "share_links.json")
	if err := os.WriteFile(linkFile, data, 0600); err != nil {
		log.Printf("保存分享链接失败: %v", err)
	}
}

// ================== 分享链接管理 ==================

// CreateShareLink 创建分享链接.
func (m *FileSharingManager) CreateShareLink(path, password, createdBy string, permissions []SharePermission, expireHours, maxDownloads int, allowUpload bool, customSlug, description string) (*ShareLink, error) {
	// 验证路径存在
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("路径不存在: %w", err)
	}

	// 验证自定义别名
	if customSlug != "" {
		m.mu.RLock()
		_, exists := m.linksBySlug[customSlug]
		m.mu.RUnlock()
		if exists {
			return nil, fmt.Errorf("自定义别名 '%s' 已存在", customSlug)
		}
	}

	// 生成令牌
	token, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("生成令牌失败: %w", err)
	}

	// 生成 ID
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("生成 ID 失败: %w", err)
	}

	// 计算过期时间
	expireAt := time.Now().Add(time.Duration(expireHours) * time.Hour)
	if expireHours <= 0 {
		expireAt = time.Time{} // 永不过期
	}

	// 默认权限
	if len(permissions) == 0 {
		permissions = []SharePermission{PermissionView, PermissionDownload}
	}

	// 密码哈希
	passwordHash := ""
	if password != "" {
		hash := sha256.Sum256([]byte(password))
		passwordHash = hex.EncodeToString(hash[:])
	}

	// 上传目录
	uploadDir := ""
	if allowUpload {
		uploadDir = filepath.Join(path, "uploads")
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			return nil, fmt.Errorf("创建上传目录失败: %w", err)
		}
	}

	link := &ShareLink{
		ID:            id,
		Path:          path,
		Token:         token,
		Password:      passwordHash,
		ExpireAt:      expireAt,
		MaxDownloads:  maxDownloads,
		DownloadCount: 0,
		CreatedBy:     createdBy,
		Permissions:   permissions,
		CustomSlug:    customSlug,
		Status:        StatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Description:   description,
		AllowUpload:   allowUpload,
		UploadDir:     uploadDir,
		ViewCount:     0,
		AccessLog:     make([]AccessLogEntry, 0),
	}

	m.mu.Lock()
	m.links[token] = link
	m.linksByID[id] = link
	if customSlug != "" {
		m.linksBySlug[customSlug] = link
	}
	m.mu.Unlock()

	m.saveLinks()

	return link, nil
}

// GetShareLink 获取分享链接.
func (m *FileSharingManager) GetShareLink(tokenOrSlug string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先尝试 token
	link, ok := m.links[tokenOrSlug]
	if !ok {
		// 再尝试自定义别名
		link, ok = m.linksBySlug[tokenOrSlug]
	}
	if !ok {
		return nil, fmt.Errorf("分享链接不存在")
	}

	// 检查状态
	if err := m.checkLinkStatus(link); err != nil {
		return nil, err
	}

	return link, nil
}

// GetShareLinkByID 通过 ID 获取分享链接.
func (m *FileSharingManager) GetShareLinkByID(id string) (*ShareLink, error) {
	m.mu.RLock()
	link, ok := m.linksByID[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("分享链接不存在")
	}

	return link, nil
}

// UpdateShareLink 更新分享链接.
func (m *FileSharingManager) UpdateShareLink(id string, updates map[string]interface{}) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.linksByID[id]
	if !ok {
		return nil, fmt.Errorf("分享链接不存在")
	}

	// 更新字段
	if v, ok := updates["password"]; ok {
		if password, ok := v.(string); ok {
			if password != "" {
				hash := sha256.Sum256([]byte(password))
				link.Password = hex.EncodeToString(hash[:])
			} else {
				link.Password = ""
			}
		}
	}

	if v, ok := updates["expireHours"]; ok {
		if hours, ok := v.(float64); ok {
			if hours > 0 {
				link.ExpireAt = time.Now().Add(time.Duration(hours) * time.Hour)
			}
		}
	}

	if v, ok := updates["maxDownloads"]; ok {
		if max, ok := v.(float64); ok {
			link.MaxDownloads = int(max)
		}
	}

	if v, ok := updates["permissions"]; ok {
		if perms, ok := v.([]interface{}); ok {
			newPerms := make([]SharePermission, 0)
			for _, p := range perms {
				if s, ok := p.(string); ok {
					newPerms = append(newPerms, SharePermission(s))
				}
			}
			link.Permissions = newPerms
		}
	}

	if v, ok := updates["description"]; ok {
		if desc, ok := v.(string); ok {
			link.Description = desc
		}
	}

	if v, ok := updates["allowUpload"]; ok {
		if allow, ok := v.(bool); ok {
			link.AllowUpload = allow
			if allow && link.UploadDir == "" {
				link.UploadDir = filepath.Join(link.Path, "uploads")
				os.MkdirAll(link.UploadDir, 0755)
			}
		}
	}

	link.UpdatedAt = time.Now()

	m.saveLinks()

	return link, nil
}

// RevokeShareLink 撤销分享链接.
func (m *FileSharingManager) RevokeShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.linksByID[id]
	if !ok {
		return fmt.Errorf("分享链接不存在")
	}

	link.Status = StatusRevoked
	link.UpdatedAt = time.Now()

	m.saveLinks()

	return nil
}

// DeleteShareLink 删除分享链接.
func (m *FileSharingManager) DeleteShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.linksByID[id]
	if !ok {
		return fmt.Errorf("分享链接不存在")
	}

	delete(m.links, link.Token)
	delete(m.linksByID, id)
	if link.CustomSlug != "" {
		delete(m.linksBySlug, link.CustomSlug)
	}

	m.saveLinks()

	return nil
}

// ListShareLinks 列出分享链接.
func (m *FileSharingManager) ListShareLinks(createdBy string, status ShareStatus, limit, offset int) ([]*ShareLink, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	links := make([]*ShareLink, 0)
	for _, link := range m.links {
		// 过滤创建者
		if createdBy != "" && link.CreatedBy != createdBy {
			continue
		}
		// 过滤状态
		if status != "" && link.Status != status {
			continue
		}
		links = append(links, link)
	}

	total := len(links)

	// 分页
	if offset >= len(links) {
		return []*ShareLink{}, total
	}

	end := offset + limit
	if end > len(links) {
		end = len(links)
	}

	return links[offset:end], total
}

// checkLinkStatus 检查链接状态.
func (m *FileSharingManager) checkLinkStatus(link *ShareLink) error {
	// 检查是否已撤销
	if link.Status == StatusRevoked {
		return fmt.Errorf("分享链接已撤销")
	}

	// 检查是否过期
	if !link.ExpireAt.IsZero() && time.Now().After(link.ExpireAt) {
		link.Status = StatusExpired
		m.saveLinks()
		return fmt.Errorf("分享链接已过期")
	}

	// 检查下载限制
	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		link.Status = StatusLimitReached
		m.saveLinks()
		return fmt.Errorf("分享链接已达到下载限制")
	}

	return nil
}

// VerifyPassword 验证密码.
func (m *FileSharingManager) VerifyPassword(link *ShareLink, password string) bool {
	if link.Password == "" {
		return true
	}
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:]) == link.Password
}

// HasPermission 检查权限.
func (m *FileSharingManager) HasPermission(link *ShareLink, permission SharePermission) bool {
	for _, p := range link.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// RecordAccess 记录访问.
func (m *FileSharingManager) RecordAccess(token, ip, userAgent, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[token]
	if !ok {
		return
	}

	entry := AccessLogEntry{
		Timestamp: time.Now(),
		IP:        ip,
		UserAgent: userAgent,
		Action:    action,
	}

	link.AccessLog = append(link.AccessLog, entry)

	// 限制日志数量
	if len(link.AccessLog) > 1000 {
		link.AccessLog = link.AccessLog[len(link.AccessLog)-1000:]
	}

	switch action {
	case "view":
		link.ViewCount++
	case "download":
		link.DownloadCount++
	}

	now := time.Now()
	link.LastAccessedAt = &now

	m.saveLinks()
}

// IncrementUploadCount 增加上传计数.
func (m *FileSharingManager) IncrementUploadCount(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[token]
	if !ok {
		return
	}

	entry := AccessLogEntry{
		Timestamp: time.Now(),
		Action:    "upload",
	}

	link.AccessLog = append(link.AccessLog, entry)
	m.saveLinks()
}

// ================== 短链接生成 ==================

// GenerateShortLink 生成短链接.
func (m *FileSharingManager) GenerateShortLink(token string) (string, error) {
	m.mu.RLock()
	link, ok := m.links[token]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("分享链接不存在")
	}

	// 使用自定义别名或 token 的前 8 位作为短链接标识
	slug := link.CustomSlug
	if slug == "" {
		slug = token[:8]
	}

	return fmt.Sprintf("%s/s/%s", m.baseURL, slug), nil
}

// ================== 二维码生成 ==================

// GenerateQRCode 生成二维码数据（返回 base64 编码的 PNG）.
func (m *FileSharingManager) GenerateQRCode(token string, size int) (string, error) {
	m.mu.RLock()
	link, ok := m.links[token]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("分享链接不存在")
	}

	// 构建 URL
	slug := link.CustomSlug
	if slug == "" {
		slug = token
	}
	url := fmt.Sprintf("%s/s/%s", m.baseURL, slug)

	// 简单的二维码生成（实际项目中应使用 qrcode 库）
	// 这里返回 URL 作为占位符，实际实现需要集成二维码库
	return url, nil
}

// ================== 文件预览 ==================

// GetPreviewInfo 获取文件预览信息.
func (m *FileSharingManager) GetPreviewInfo(filePath string) (*PreviewInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	mimeType := getMimeType(filePath)
	previewType := getPreviewType(mimeType)

	preview := &PreviewInfo{
		FilePath:    filePath,
		FileName:    filepath.Base(filePath),
		MimeType:    mimeType,
		PreviewType: previewType,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
	}

	// 根据类型生成预览 URL
	switch previewType {
	case "image":
		preview.PreviewURL = fmt.Sprintf("/api/sharing/preview/image?path=%s", filePath)
	case "video":
		preview.PreviewURL = fmt.Sprintf("/api/sharing/preview/video?path=%s", filePath)
	case "document":
		preview.PreviewURL = fmt.Sprintf("/api/sharing/preview/document?path=%s", filePath)
	}

	return preview, nil
}

// getMimeType 获取 MIME 类型.
func getMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".ogg":  "video/ogg",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".zip":  "application/zip",
		".gz":   "application/gzip",
		".tar":  "application/x-tar",
		".7z":   "application/x-7z-compressed",
		".rar":  "application/vnd.rar",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// getPreviewType 获取预览类型.
func getPreviewType(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "text/"):
		return "text"
	case strings.Contains(mimeType, "pdf") ||
		strings.Contains(mimeType, "document") ||
		strings.Contains(mimeType, "spreadsheet") ||
		strings.Contains(mimeType, "presentation"):
		return "document"
	default:
		return "unsupported"
	}
}

// ================== 批量打包下载 ==================

// CreateZipArchive 创建 ZIP 压缩包.
func (m *FileSharingManager) CreateZipArchive(paths []string, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	for _, path := range paths {
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 获取相对路径
			relPath, err := filepath.Rel(filepath.Dir(path), filePath)
			if err != nil {
				return err
			}

			// 创建文件头
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relPath)
			header.Method = zip.Deflate

			// 目录
			if info.IsDir() {
				header.Name += "/"
				_, err = zipWriter.CreateHeader(header)
				return err
			}

			// 文件
			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}

			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			return err
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// CreateTarArchive 创建 TAR 压缩包.
func (m *FileSharingManager) CreateTarArchive(paths []string, writer io.Writer, useGzip bool) error {
	var tarWriter *tar.Writer

	if useGzip {
		gzWriter := gzip.NewWriter(writer)
		defer gzWriter.Close()
		tarWriter = tar.NewWriter(gzWriter)
	} else {
		tarWriter = tar.NewWriter(writer)
	}
	defer tarWriter.Close()

	for _, path := range paths {
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 创建 tar 头
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			// 更新名称为相对路径
			relPath, err := filepath.Rel(filepath.Dir(path), filePath)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relPath)

			// 写入头
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}

			// 如果是目录，跳过内容
			if info.IsDir() {
				return nil
			}

			// 写入文件内容
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			return err
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// ================== 统计分析 ==================

// GetStats 获取分享统计.
func (m *FileSharingManager) GetStats(createdBy string) *ShareStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ShareStats{}
	topLinks := make([]ShareLink, 0)

	for _, link := range m.links {
		// 过滤创建者
		if createdBy != "" && link.CreatedBy != createdBy {
			continue
		}

		stats.TotalLinks++
		stats.TotalDownloads += link.DownloadCount
		stats.TotalViews += link.ViewCount

		// 统计上传次数
		for _, log := range link.AccessLog {
			if log.Action == "upload" {
				stats.TotalUploads++
			}
		}

		switch link.Status {
		case StatusActive:
			stats.ActiveLinks++
		case StatusExpired:
			stats.ExpiredLinks++
		case StatusRevoked:
			stats.RevokedLinks++
		}

		// 收集热门链接
		topLinks = append(topLinks, *link)
	}

	if stats.TotalLinks > 0 {
		stats.AverageDownloads = float64(stats.TotalDownloads) / float64(stats.TotalLinks)
	}

	// 按下载次数排序
	sortLinksByDownloads(topLinks)
	if len(topLinks) > 10 {
		topLinks = topLinks[:10]
	}
	stats.TopLinks = topLinks

	return stats
}

// sortLinksByDownloads 按下载次数排序.
func sortLinksByDownloads(links []ShareLink) {
	for i := 0; i < len(links)-1; i++ {
		for j := i + 1; j < len(links); j++ {
			if links[j].DownloadCount > links[i].DownloadCount {
				links[i], links[j] = links[j], links[i]
			}
		}
	}
}

// GetLinkStats 获取单个链接统计.
func (m *FileSharingManager) GetLinkStats(id string) (map[string]interface{}, error) {
	m.mu.RLock()
	link, ok := m.linksByID[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("分享链接不存在")
	}

	stats := map[string]interface{}{
		"id":            link.ID,
		"path":          link.Path,
		"status":        link.Status,
		"viewCount":     link.ViewCount,
		"downloadCount": link.DownloadCount,
		"createdAt":     link.CreatedAt,
		"lastAccessed":  link.LastAccessedAt,
		"expireAt":      link.ExpireAt,
		"maxDownloads":  link.MaxDownloads,
	}

	// 统计上传次数
	uploadCount := 0
	for _, log := range link.AccessLog {
		if log.Action == "upload" {
			uploadCount++
		}
	}
	stats["uploadCount"] = uploadCount

	// 最近访问记录
	recentAccess := make([]AccessLogEntry, 0)
	if len(link.AccessLog) > 0 {
		start := len(link.AccessLog) - 10
		if start < 0 {
			start = 0
		}
		recentAccess = link.AccessLog[start:]
	}
	stats["recentAccess"] = recentAccess

	return stats, nil
}

// ================== 工具函数 ==================

// generateToken 生成随机令牌.
func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// generateID 生成唯一 ID.
func generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ================== 清理过期链接 ==================

// CleanupExpiredLinks 清理过期链接.
func (m *FileSharingManager) CleanupExpiredLinks() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, link := range m.links {
		if !link.ExpireAt.IsZero() && time.Now().After(link.ExpireAt) {
			link.Status = StatusExpired
			count++
		}
	}

	if count > 0 {
		m.saveLinks()
	}

	return count
}
