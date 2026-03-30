// Package webshare 文件管理器辅助模块
package webshare

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nfnt/resize"
)

// FileManager 文件管理器
type FileManager struct {
	config      WebShareConfig
	mimeTypes   map[string]string
	imageExts   map[string]bool
	videoExts   map[string]bool
	audioExts   map[string]bool
	docExts     map[string]bool
	thumbCache  sync.Map
	thumbDir    string
}

// NewFileManager 创建文件管理器
func NewFileManager(config WebShareConfig) *FileManager {
	thumbDir := filepath.Join(config.CacheDir, "thumbnails")
	if err := os.MkdirAll(thumbDir, 0750); err != nil {
		log.Printf("创建缩略图目录失败: %v", err)
	}

	return &FileManager{
		config:   config,
		thumbDir: thumbDir,
		mimeTypes: map[string]string{
			".jpg":  "image/jpeg",
			".jpeg": "image/jpeg",
			".png":  "image/png",
			".gif":  "image/gif",
			".webp": "image/webp",
			".bmp":  "image/bmp",
			".svg":  "image/svg+xml",
			".ico":  "image/x-icon",
			".mp4":  "video/mp4",
			".mkv":  "video/x-matroska",
			".avi":  "video/x-msvideo",
			".mov":  "video/quicktime",
			".wmv":  "video/x-ms-wmv",
			".flv":  "video/x-flv",
			".webm": "video/webm",
			".mp3":  "audio/mpeg",
			".wav":  "audio/wav",
			".flac": "audio/flac",
			".aac":  "audio/aac",
			".ogg":  "audio/ogg",
			".m4a":  "audio/mp4",
			".pdf":  "application/pdf",
			".doc":  "application/msword",
			".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			".xls":  "application/vnd.ms-excel",
			".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			".ppt":  "application/vnd.ms-powerpoint",
			".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			".txt":  "text/plain",
			".rtf":  "application/rtf",
			".zip":  "application/zip",
			".rar":  "application/vnd.rar",
			".7z":   "application/x-7z-compressed",
			".tar":  "application/x-tar",
			".gz":   "application/gzip",
			".json": "application/json",
			".xml":  "application/xml",
			".html": "text/html",
			".css":  "text/css",
			".js":   "application/javascript",
			".go":   "text/x-go",
			".py":   "text/x-python",
			".java": "text/x-java",
			".c":    "text/x-c",
			".cpp":  "text/x-c++",
			".h":    "text/x-c",
			".sh":   "application/x-sh",
			".md":   "text/markdown",
			".yaml": "application/x-yaml",
			".yml":  "application/x-yaml",
		},
		imageExts: map[string]bool{
			".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
			".webp": true, ".bmp": true, ".svg": true, ".ico": true,
			".heic": true, ".heif": true, ".tiff": true, ".tif": true,
		},
		videoExts: map[string]bool{
			".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
			".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
			".mpeg": true, ".mpg": true, ".3gp": true,
		},
		audioExts: map[string]bool{
			".mp3": true, ".wav": true, ".flac": true, ".aac": true,
			".ogg": true, ".wma": true, ".m4a": true, ".ape": true,
		},
		docExts: map[string]bool{
			".pdf": true, ".doc": true, ".docx": true, ".xls": true,
			".xlsx": true, ".ppt": true, ".pptx": true, ".txt": true,
			".rtf": true, ".odt": true, ".ods": true, ".odp": true,
		},
	}
}

// GetMimeType 获取 MIME 类型
func (fm *FileManager) GetMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mime, ok := fm.mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// IsImage 判断是否是图片
func (fm *FileManager) IsImage(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return fm.imageExts[ext]
}

// IsVideo 判断是否是视频
func (fm *FileManager) IsVideo(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return fm.videoExts[ext]
}

// IsAudio 判断是否是音频
func (fm *FileManager) IsAudio(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return fm.audioExts[ext]
}

// IsDocument 判断是否是文档
func (fm *FileManager) IsDocument(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return fm.docExts[ext]
}

// GetFileType 获取文件类型分类
func (fm *FileManager) GetFileType(filename string) string {
	if fm.IsImage(filename) {
		return "image"
	}
	if fm.IsVideo(filename) {
		return "video"
	}
	if fm.IsAudio(filename) {
		return "audio"
	}
	if fm.IsDocument(filename) {
		return "document"
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2":
		return "archive"
	case ".js", ".ts", ".py", ".go", ".java", ".c", ".cpp", ".h", ".css", ".html", ".json", ".xml", ".yaml", ".yml", ".md", ".sh":
		return "code"
	default:
		return "other"
	}
}

// GenerateThumbnail 生成缩略图
func (fm *FileManager) GenerateThumbnail(path string) (string, error) {
	// 检查缓存
	if cached, ok := fm.thumbCache.Load(path); ok {
		return cached.(string), nil
	}

	// 检查是否是图片
	if !fm.IsImage(filepath.Base(path)) {
		return "", nil // 非图片不生成缩略图
	}

	// 生成缩略图路径
	thumbPath := filepath.Join(fm.thumbDir, strings.ReplaceAll(strings.ReplaceAll(path, "/", "_"), "\\", "_")+".jpg")

	// 检查缓存文件是否存在
	if _, err := os.Stat(thumbPath); err == nil {
		fm.thumbCache.Store(path, thumbPath)
		return thumbPath, nil
	}

	// 读取原图
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 解码图片
	var img image.Image
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
	case ".png":
		img, err = png.Decode(file)
	case ".gif":
		// GIF 需要特殊处理，这里简化为静态帧
		img, _, err = image.Decode(file)
	default:
		img, _, err = image.Decode(file)
	}

	if err != nil {
		return "", err
	}

	// 缩放图片
	thumbnail := resize.Thumbnail(256, 256, img, resize.Lanczos3)

	// 创建缩略图文件
	thumbFile, err := os.Create(thumbPath)
	if err != nil {
		return "", err
	}
	defer thumbFile.Close()

	// 编码为 JPEG
	if err := jpeg.Encode(thumbFile, thumbnail, &jpeg.Options{Quality: 85}); err != nil {
		return "", err
	}

	// 缓存
	fm.thumbCache.Store(path, thumbPath)

	return thumbPath, nil
}

// ClearThumbnailCache 清除缩略图缓存
func (fm *FileManager) ClearThumbnailCache() error {
	fm.thumbCache = sync.Map{}
	return os.RemoveAll(fm.thumbDir)
}