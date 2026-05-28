// Package smartshare 提供安全在线预览功能
package smartshare

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PreviewEngine 预览引擎
type PreviewEngine struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config *PreviewConfig
}

// NewPreviewEngine 创建预览引擎
func NewPreviewEngine(logger *zap.Logger, config *PreviewConfig) *PreviewEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultPreviewConfig()
	}

	return &PreviewEngine{
		logger: logger,
		config: config,
	}
}

// PreviewRequest 预览请求
type PreviewRequest struct {
	FilePath    string `json:"file_path"`
	ShareID     string `json:"share_id"`
	UserID      string `json:"user_id,omitempty"`
	IP          string `json:"ip"`
	Page        int    `json:"page,omitempty"`      // 页码（PDF等）
	Quality     int    `json:"quality,omitempty"`    // 预览质量 1-100
	Format      string `json:"format,omitempty"`     // 输出格式
	Watermark   bool   `json:"watermark"`            // 是否添加水印
}

// PreviewResponse 预览响应
type PreviewResponse struct {
	PreviewURL   string        `json:"preview_url"`
	ContentType  string        `json:"content_type"`
	FileType     FileType      `json:"file_type"`
	TotalPages   int           `json:"total_pages,omitempty"`
	CurrentPage  int           `json:"current_page,omitempty"`
	FileSize     int64         `json:"file_size"`
	PreviewSize  int64         `json:"preview_size"`
	ExpiresIn    time.Duration `json:"expires_in"`
	SecurityInfo *SecurityInfo `json:"security_info"`
	CreatedAt    time.Time     `json:"created_at"`
}

// FileType 文件类型
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypePDF      FileType = "pdf"
	FileTypeOffice   FileType = "office"
	FileTypeText     FileType = "text"
	FileTypeCode     FileType = "code"
	FileTypeArchive  FileType = "archive"
	FileTypeUnknown  FileType = "unknown"
)

// SecurityInfo 安全信息
type SecurityInfo struct {
	AllowDownload    bool `json:"allow_download"`
	AllowPrint       bool `json:"allow_print"`
	AllowCopy        bool `json:"allow_copy"`
	WatermarkEnabled bool `json:"watermark_enabled"`
	DRMEnabled       bool `json:"drm_enabled"`
}

// CreatePreview 创建安全预览
func (pe *PreviewEngine) CreatePreview(req *PreviewRequest) (*PreviewResponse, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// 检测文件类型
	fileType := pe.detectFileType(req.FilePath)

	// 检查是否支持预览
	if !pe.isPreviewSupported(fileType) {
		return nil, fmt.Errorf("file type %s does not support preview", fileType)
	}

	// 设置质量
	quality := req.Quality
	if quality == 0 {
		quality = pe.config.PreviewQuality
	}
	if quality < 1 || quality > 100 {
		quality = 80
	}

	pe.logger.Info("creating preview",
		zap.String("file", req.FilePath),
		zap.String("type", string(fileType)),
		zap.Int("quality", quality))

	// 构建安全信息
	securityInfo := &SecurityInfo{
		AllowDownload:    pe.config.AllowDownload,
		AllowPrint:       pe.config.AllowPrint,
		AllowCopy:        pe.config.AllowCopy,
		WatermarkEnabled: pe.config.WatermarkPreview,
		DRMEnabled:       !pe.config.AllowDownload,
	}

	// 生成预览 URL
	previewURL := pe.generatePreviewURL(req.ShareID, req.FilePath)

	response := &PreviewResponse{
		PreviewURL:   previewURL,
		ContentType:  pe.getContentType(fileType),
		FileType:     fileType,
		TotalPages:   pe.estimatePages(req.FilePath, fileType),
		CurrentPage:  req.Page,
		SecurityInfo: securityInfo,
		ExpiresIn:    30 * time.Minute,
		CreatedAt:    time.Now(),
	}

	return response, nil
}

// detectFileType 检测文件类型
func (pe *PreviewEngine) detectFileType(filePath string) FileType {
	ext := strings.ToLower(filepath.Ext(filePath))

	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico"}
	videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm"}
	audioExts := []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma"}
	pdfExts := []string{".pdf"}
	officeExts := []string{".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx"}
	textExts := []string{".txt", ".md", ".csv", ".json", ".xml", ".yaml", ".yml"}
	codeExts := []string{".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".rs", ".rb", ".php", ".html", ".css", ".sql"}
	archiveExts := []string{".zip", ".rar", ".7z", ".tar", ".gz", ".bz2"}

	for _, e := range imageExts {
		if e == ext {
			return FileTypeImage
		}
	}
	for _, e := range videoExts {
		if e == ext {
			return FileTypeVideo
		}
	}
	for _, e := range audioExts {
		if e == ext {
			return FileTypeAudio
		}
	}
	for _, e := range pdfExts {
		if e == ext {
			return FileTypePDF
		}
	}
	for _, e := range officeExts {
		if e == ext {
			return FileTypeOffice
		}
	}
	for _, e := range textExts {
		if e == ext {
			return FileTypeText
		}
	}
	for _, e := range codeExts {
		if e == ext {
			return FileTypeCode
		}
	}
	for _, e := range archiveExts {
		if e == ext {
			return FileTypeArchive
		}
	}

	return FileTypeUnknown
}

// isPreviewSupported 检查是否支持预览
func (pe *PreviewEngine) isPreviewSupported(fileType FileType) bool {
	supportedTypes := []FileType{
		FileTypeImage,
		FileTypeVideo,
		FileTypeAudio,
		FileTypePDF,
		FileTypeOffice,
		FileTypeText,
		FileTypeCode,
	}

	for _, t := range supportedTypes {
		if t == fileType {
			return true
		}
	}
	return false
}

// getContentType 获取内容类型
func (pe *PreviewEngine) getContentType(fileType FileType) string {
	contentTypes := map[FileType]string{
		FileTypeImage:  "image/*",
		FileTypeVideo:  "video/*",
		FileTypeAudio:  "audio/*",
		FileTypePDF:    "application/pdf",
		FileTypeOffice: "application/octet-stream",
		FileTypeText:   "text/plain",
		FileTypeCode:   "text/plain",
	}

	if ct, ok := contentTypes[fileType]; ok {
		return ct
	}
	return "application/octet-stream"
}

// estimatePages 估算页数
func (pe *PreviewEngine) estimatePages(filePath string, fileType FileType) int {
	switch fileType {
	case FileTypePDF:
		return pe.config.MaxPreviewPages
	case FileTypeOffice:
		return 10
	default:
		return 1
	}
}

// generatePreviewURL 生成预览 URL
func (pe *PreviewEngine) generatePreviewURL(shareID, filePath string) string {
	return fmt.Sprintf("/api/v1/smartshare/preview/%s?file=%s", shareID, filePath)
}

// UpdateConfig 更新预览配置
func (pe *PreviewEngine) UpdateConfig(config *PreviewConfig) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.config = config
}

// GetConfig 获取预览配置
func (pe *PreviewEngine) GetConfig() *PreviewConfig {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	cfg := *pe.config
	return &cfg
}
