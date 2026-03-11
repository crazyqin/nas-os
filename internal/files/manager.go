package files

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nfnt/resize"
)

// FileType 文件类型
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeDocument FileType = "document"
	FileTypePDF      FileType = "pdf"
	FileTypeCode     FileType = "code"
	FileTypeArchive  FileType = "archive"
	FileTypeOther    FileType = "other"
)

// FileInfo 文件信息
type FileInfo struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Size      int64    `json:"size"`
	Mode      string   `json:"mode"`
	ModTime   string   `json:"modTime"`
	IsDir     bool     `json:"isDir"`
	Type      FileType `json:"type"`
	MimeType  string   `json:"mimeType"`
	Thumbnail string   `json:"thumbnail,omitempty"`
	Width     int      `json:"width,omitempty"`
	Height    int      `json:"height,omitempty"`
	Duration  int      `json:"duration,omitempty"` // 视频时长(秒)
}

// PreviewConfig 预览配置
type PreviewConfig struct {
	ThumbnailSize    uint          `json:"thumbnailSize"`    // 缩略图尺寸
	MaxPreviewSize   int64         `json:"maxPreviewSize"`   // 最大预览文件大小 (bytes)
	CacheDir         string        `json:"cacheDir"`         // 缓存目录
	CacheExpiry      time.Duration `json:"cacheExpiry"`      // 缓存过期时间
	EnableVideoThumb bool          `json:"enableVideoThumb"` // 启用视频缩略图
	EnableDocPreview bool          `json:"enableDocPreview"` // 启用文档预览
}

// Manager 文件管理器
type Manager struct {
	config     PreviewConfig
	imageTypes map[string]bool
	videoTypes map[string]bool
	audioTypes map[string]bool
	docTypes   map[string]bool
	codeTypes  map[string]bool
	thumbCache sync.Map
}

// NewManager 创建文件管理器
func NewManager(config PreviewConfig) *Manager {
	if config.ThumbnailSize == 0 {
		config.ThumbnailSize = 256
	}
	if config.MaxPreviewSize == 0 {
		config.MaxPreviewSize = 50 * 1024 * 1024 // 50MB
	}
	if config.CacheDir == "" {
		config.CacheDir = "/tmp/nas-os/thumbnails"
	}
	if config.CacheExpiry == 0 {
		config.CacheExpiry = 24 * time.Hour
	}

	// 确保缓存目录存在
	os.MkdirAll(config.CacheDir, 0755)

	m := &Manager{
		config: config,
		imageTypes: map[string]bool{
			".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
			".webp": true, ".bmp": true, ".svg": true, ".ico": true,
			".tiff": true, ".tif": true, ".heic": true, ".heif": true,
		},
		videoTypes: map[string]bool{
			".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
			".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
			".mpeg": true, ".mpg": true, ".3gp": true,
		},
		audioTypes: map[string]bool{
			".mp3": true, ".wav": true, ".flac": true, ".aac": true,
			".ogg": true, ".wma": true, ".m4a": true, ".ape": true,
		},
		docTypes: map[string]bool{
			".pdf": true, ".doc": true, ".docx": true, ".xls": true,
			".xlsx": true, ".ppt": true, ".pptx": true, ".txt": true,
			".rtf": true, ".odt": true, ".ods": true, ".odp": true,
		},
		codeTypes: map[string]bool{
			".js": true, ".ts": true, ".py": true, ".go": true,
			".java": true, ".c": true, ".cpp": true, ".h": true,
			".html": true, ".css": true, ".json": true, ".xml": true,
			".yaml": true, ".yml": true, ".md": true, ".sh": true,
			".sql": true, ".php": true, ".rb": true, ".rs": true,
		},
	}

	return m
}

// GetFileType 获取文件类型
func (m *Manager) GetFileType(path string) FileType {
	ext := strings.ToLower(filepath.Ext(path))
	
	if m.imageTypes[ext] {
		return FileTypeImage
	}
	if m.videoTypes[ext] {
		return FileTypeVideo
	}
	if m.audioTypes[ext] {
		return FileTypeAudio
	}
	if m.docTypes[ext] {
		if ext == ".pdf" {
			return FileTypePDF
		}
		return FileTypeDocument
	}
	if m.codeTypes[ext] {
		return FileTypeCode
	}
	if ext == ".zip" || ext == ".rar" || ext == ".7z" || ext == ".tar" || ext == ".gz" {
		return FileTypeArchive
	}
	return FileTypeOther
}

// GetMimeType 获取 MIME 类型
func (m *Manager) GetMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".bmp":  "image/bmp",
		".ico":  "image/x-icon",
		".mp4":  "video/mp4",
		".mkv":  "video/x-matroska",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".webm": "video/webm",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".ogg":  "audio/ogg",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt":  "text/plain",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".zip":  "application/zip",
		".rar":  "application/x-rar-compressed",
		".7z":   "application/x-7z-compressed",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
	}
	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}

// ListFiles 列出目录文件
func (m *Manager) ListFiles(dirPath string, generateThumbnails bool) ([]FileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		fileType := m.GetFileType(filePath)

		file := FileInfo{
			Name:    entry.Name(),
			Path:    filePath,
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format(time.RFC3339),
			IsDir:   entry.IsDir(),
			Type:    fileType,
			MimeType: m.GetMimeType(filePath),
		}

		// 生成缩略图
		if generateThumbnails && !entry.IsDir() {
			if fileType == FileTypeImage {
				thumb, w, h := m.GenerateImageThumbnail(filePath)
				file.Thumbnail = thumb
				file.Width = w
				file.Height = h
			} else if fileType == FileTypeVideo && m.config.EnableVideoThumb {
				thumb := m.GenerateVideoThumbnail(filePath)
				file.Thumbnail = thumb
			}
		}

		files = append(files, file)
	}

	return files, nil
}

// GenerateImageThumbnail 生成图片缩略图
func (m *Manager) GenerateImageThumbnail(path string) (string, int, int) {
	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%d:%d", path, m.config.ThumbnailSize, m.config.ThumbnailSize)
	if cached, ok := m.thumbCache.Load(cacheKey); ok {
		if data, ok := cached.(struct{ thumb string; w, h int }); ok {
			return data.thumb, data.w, data.h
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return "", 0, 0
	}
	defer file.Close()

	var img image.Image
	ext := strings.ToLower(filepath.Ext(path))

	// 根据格式解码
	switch ext {
	case ".png":
		img, err = png.Decode(file)
	case ".gif":
		img, err = gif.Decode(file)
	default:
		// JPEG 和其他格式
		img, err = jpeg.Decode(file)
	}

	if err != nil {
		return "", 0, 0
	}

	// 获取原始尺寸
	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// 生成缩略图
	thumb := resize.Thumbnail(m.config.ThumbnailSize, m.config.ThumbnailSize, img, resize.Lanczos3)

	// 编码为 base64
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 85}); err != nil {
		return "", 0, 0
	}

	thumbBase64 := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// 缓存
	m.thumbCache.Store(cacheKey, struct{ thumb string; w, h int }{thumbBase64, origW, origH})

	return thumbBase64, origW, origH
}

// GenerateVideoThumbnail 生成视频缩略图
func (m *Manager) GenerateVideoThumbnail(path string) string {
	if !m.config.EnableVideoThumb {
		return ""
	}

	cacheKey := fmt.Sprintf("video:%s:%d", path, m.config.ThumbnailSize)
	if cached, ok := m.thumbCache.Load(cacheKey); ok {
		if thumb, ok := cached.(string); ok {
			return thumb
		}
	}

	// 使用 ffmpeg 生成缩略图
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outputFile := filepath.Join(m.config.CacheDir, fmt.Sprintf("%d.jpg", time.Now().UnixNano()))
	defer os.Remove(outputFile)

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", path,
		"-ss", "00:00:01", // 跳到第1秒
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", m.config.ThumbnailSize, m.config.ThumbnailSize),
		"-q:v", "5",
		"-y", outputFile,
	)

	if err := cmd.Run(); err != nil {
		return ""
	}

	// 读取并转换为 base64
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return ""
	}

	thumbBase64 := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
	m.thumbCache.Store(cacheKey, thumbBase64)

	return thumbBase64
}

// GetVideoInfo 获取视频信息
func (m *Manager) GetVideoInfo(path string) (int, int, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,duration",
		"-of", "csv=p=0",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}

	var width, height, duration int
	fmt.Sscanf(string(output), "%d,%d,%f", &width, &height, new(float64))

	return width, height, duration, nil
}

// PreviewFile 预览文件
func (m *Manager) PreviewFile(path string) (io.ReadCloser, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}

	if info.Size() > m.config.MaxPreviewSize {
		return nil, "", fmt.Errorf("文件过大，无法预览 (最大 %d MB)", m.config.MaxPreviewSize/(1024*1024))
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}

	mimeType := m.GetMimeType(path)
	return file, mimeType, nil
}

// GetFileContent 获取文件内容 (文本文件)
func (m *Manager) GetFileContent(path string, maxSize int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.Size() > maxSize {
		return "", fmt.Errorf("文件过大")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Handlers 文件处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	files := r.Group("/files")
	{
		files.GET("/list", h.listFiles)
		files.GET("/preview", h.previewFile)
		files.GET("/thumbnail", h.getThumbnail)
		files.GET("/download", h.downloadFile)
		files.POST("/upload", h.uploadFile)
		files.POST("/mkdir", h.createDir)
		files.DELETE("/delete", h.deleteFile)
		files.GET("/info", h.getFileInfo)
	}
}

// listFiles 列出文件
func (h *Handlers) listFiles(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	// 安全检查：防止路径遍历
	if strings.Contains(path, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效路径"})
		return
	}

	thumbnail := c.Query("thumbnail") == "true"

	files, err := h.manager.ListFiles(path, thumbnail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"path":  path,
			"files": files,
		},
	})
}

// previewFile 预览文件
func (h *Handlers) previewFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少文件路径"})
		return
	}

	// 安全检查
	if strings.Contains(path, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效路径"})
		return
	}

	fileType := h.manager.GetFileType(path)

	// 文本文件直接返回内容
	if fileType == FileTypeCode || fileType == FileTypeDocument && filepath.Ext(path) == ".txt" {
		content, err := h.manager.GetFileContent(path, 10*1024*1024) // 10MB
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"type":    fileType,
				"content": content,
			},
		})
		return
	}

	// 其他文件流式返回
	reader, mimeType, err := h.manager.PreviewFile(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	defer reader.Close()

	c.DataFromReader(http.StatusOK, -1, mimeType, reader, nil)
}

// getThumbnail 获取缩略图
func (h *Handlers) getThumbnail(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少文件路径"})
		return
	}

	// 安全检查
	if strings.Contains(path, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效路径"})
		return
	}

	fileType := h.manager.GetFileType(path)
	var thumb string

	if fileType == FileTypeImage {
		thumb, _, _ = h.manager.GenerateImageThumbnail(path)
	} else if fileType == FileTypeVideo {
		thumb = h.manager.GenerateVideoThumbnail(path)
	}

	if thumb == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "无法生成缩略图"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"thumbnail": thumb,
		},
	})
}

// downloadFile 下载文件
func (h *Handlers) downloadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少文件路径"})
		return
	}

	// 安全检查
	if strings.Contains(path, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效路径"})
		return
	}

	c.FileAttachment(path, filepath.Base(path))
}

// uploadFile 上传文件
func (h *Handlers) uploadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少目标路径"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少文件"})
		return
	}

	// 保存文件
	dst := filepath.Join(path, file.Filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上传成功",
		"data": gin.H{
			"name": file.Filename,
			"path": dst,
			"size": file.Size,
		},
	})
}

// createDir 创建目录
func (h *Handlers) createDir(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	fullPath := filepath.Join(req.Path, req.Name)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data": gin.H{
			"path": fullPath,
		},
	})
}

// deleteFile 删除文件
func (h *Handlers) deleteFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少文件路径"})
		return
	}

	if err := os.Remove(path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// getFileInfo 获取文件信息
func (h *Handlers) getFileInfo(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少文件路径"})
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "文件不存在"})
		return
	}

	fileType := h.manager.GetFileType(path)
	file := FileInfo{
		Name:     info.Name(),
		Path:     path,
		Size:     info.Size(),
		Mode:     info.Mode().String(),
		ModTime:  info.ModTime().Format(time.RFC3339),
		IsDir:    info.IsDir(),
		Type:     fileType,
		MimeType: h.manager.GetMimeType(path),
	}

	// 图片获取尺寸
	if fileType == FileTypeImage {
		_, w, h := h.manager.GenerateImageThumbnail(path)
		file.Width = w
		file.Height = h
	}

	// 视频获取尺寸和时长
	if fileType == FileTypeVideo {
		w, h, d, err := h.manager.GetVideoInfo(path)
		if err == nil {
			file.Width = w
			file.Height = h
			file.Duration = d
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    file,
	})
}