package filemanager

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// Preview 文件预览器
type Preview struct {
	config ThumbnailConfig
	logger *zap.Logger
}

// NewPreview 创建文件预览器
func NewPreview(config ThumbnailConfig, logger *zap.Logger) *Preview {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Preview{
		config: config,
		logger: logger,
	}
}

// GetPreviewInfo 获取文件预览信息
func (p *Preview) GetPreviewInfo(path string) (*PreviewInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	previewInfo := &PreviewInfo{
		Path:    path,
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}

	// 根据MIME类型确定预览类型
	mimeType := getMIMEType(path)
	previewInfo.MIMEType = mimeType

	previewType := p.getPreviewType(mimeType, path)
	previewInfo.Type = previewType

	// 根据类型获取详细信息
	switch previewType {
	case PreviewImage:
		p.getImageInfo(path, previewInfo)
	case PreviewVideo, PreviewAudio:
		p.getMediaInfo(path, previewInfo)
	case PreviewCode, PreviewText, PreviewMarkdown:
		p.getTextInfo(path, previewInfo)
	case PreviewPDF:
		p.getPDFInfo(path, previewInfo)
	}

	return previewInfo, nil
}

// IsPreviewable 检查文件是否可预览
func (p *Preview) IsPreviewable(path string) bool {
	mimeType := getMIMEType(path)
	previewType := p.getPreviewType(mimeType, path)
	return previewType != PreviewNone
}

// GetSupportedContentTypes 获取支持预览的内容类型
func (p *Preview) GetSupportedContentTypes() map[string][]string {
	return map[string][]string{
		"image": {
			"image/jpeg", "image/png", "image/gif", "image/bmp",
			"image/webp", "image/svg+xml", "image/x-icon",
		},
		"video": {
			"video/mp4", "video/x-msvideo", "video/x-matroska",
			"video/quicktime", "video/x-ms-wmv", "video/x-flv",
			"video/webm",
		},
		"audio": {
			"audio/mpeg", "audio/wav", "audio/flac", "audio/aac",
			"audio/ogg", "audio/x-ms-wma", "audio/mp4",
		},
		"document": {
			"application/pdf",
			"application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.ms-excel",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/vnd.ms-powerpoint",
			"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		},
		"text": {
			"text/plain", "text/html", "text/css", "text/javascript",
			"application/javascript", "application/json", "application/xml",
			"text/xml", "text/yaml", "text/markdown", "text/csv",
		},
		"code": {
			"text/x-go", "text/x-python", "text/x-java", "text/x-c",
			"text/x-c++", "text/x-rust", "application/x-shellscript",
		},
	}
}

// getPreviewType 根据MIME类型判断预览类型
func (p *Preview) getPreviewType(mimeType, path string) PreviewType {
	ext := strings.ToLower(filepath.Ext(path))

	// 代码文件
	codeExts := map[string]bool{
		".go": true, ".py": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".rs": true, ".js": true, ".ts": true,
		".jsx": true, ".tsx": true, ".vue": true, ".svelte": true,
		".rb": true, ".php": true, ".swift": true, ".kt": true,
		".scala": true, ".clj": true, ".hs": true, ".ml": true,
		".lua": true, ".r": true, ".m": true, ".mm": true,
	}
	if codeExts[ext] {
		return PreviewCode
	}

	// Markdown
	if ext == ".md" || ext == ".markdown" {
		return PreviewMarkdown
	}

	// 根据MIME类型
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return PreviewImage
	case strings.HasPrefix(mimeType, "video/"):
		return PreviewVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return PreviewAudio
	case mimeType == "application/pdf":
		return PreviewPDF
	case strings.HasPrefix(mimeType, "text/"):
		return PreviewText
	case strings.Contains(mimeType, "document") || strings.Contains(mimeType, "spreadsheet") || strings.Contains(mimeType, "presentation"):
		return PreviewDocument
	}

	return PreviewNone
}

// getImageInfo 获取图片信息
func (p *Preview) getImageInfo(path string, info *PreviewInfo) {
	// 简化实现：仅读取文件头部判断尺寸
	// 实际实现应该使用 image.Decode 或第三方库
	ext := strings.ToLower(filepath.Ext(path))

	// 常见图片格式的默认尺寸（如果无法读取）
	switch ext {
	case ".jpg", ".jpeg":
		// JPEG需要解析EXIF
	case ".png":
		// PNG需要解析IHDR
	case ".gif":
		// GIF需要解析逻辑屏幕描述符
	case ".bmp":
		// BMP需要解析位图信息头
	}

	// 尝试生成缩略图路径
	if p.config.Enabled {
		info.Thumbnail = p.getThumbnailPath(path)
	}
}

// getMediaInfo 获取媒体信息（视频/音频）
func (p *Preview) getMediaInfo(path string, info *PreviewInfo) {
	// 简化实现：需要使用 ffprobe 或类似工具
	// 这里只设置基本的MIME类型信息
	ext := strings.ToLower(filepath.Ext(path))

	videoCodecs := map[string]string{
		".mp4":  "h264",
		".mkv":  "h264/h265",
		".avi":  "mpeg4",
		".mov":  "h264",
		".webm": "vp8/vp9",
	}

	audioCodecs := map[string]string{
		".mp3":  "mp3",
		".wav":  "pcm",
		".flac": "flac",
		".aac":  "aac",
		".ogg":  "vorbis/opus",
		".m4a":  "aac",
	}

	if codec, ok := videoCodecs[ext]; ok {
		info.Codec = codec
	} else if codec, ok := audioCodecs[ext]; ok {
		info.Codec = codec
	}
}

// getTextInfo 获取文本/代码信息
func (p *Preview) getTextInfo(path string, info *PreviewInfo) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		// 限制扫描行数，避免大文件卡顿
		if lineCount > 10000 {
			break
		}
	}

	info.LineCount = lineCount

	// 检测编程语言
	ext := strings.ToLower(filepath.Ext(path))
	languageMap := map[string]string{
		".go":    "Go",
		".py":    "Python",
		".java":  "Java",
		".c":     "C",
		".cpp":   "C++",
		".h":     "C/C++ Header",
		".rs":    "Rust",
		".js":    "JavaScript",
		".ts":    "TypeScript",
		".jsx":   "React JSX",
		".tsx":   "React TSX",
		".vue":   "Vue",
		".rb":    "Ruby",
		".php":   "PHP",
		".swift": "Swift",
		".kt":    "Kotlin",
		".sh":    "Shell",
		".bash":  "Bash",
		".html":  "HTML",
		".css":   "CSS",
		".scss":  "SCSS",
		".less":  "LESS",
		".sql":   "SQL",
		".r":     "R",
		".lua":   "Lua",
		".perl":  "Perl",
		".pl":    "Perl",
	}

	if lang, ok := languageMap[ext]; ok {
		info.Language = lang
	}
}

// getPDFInfo 获取PDF信息
func (p *Preview) getPDFInfo(path string, info *PreviewInfo) {
	// 简化实现：需要使用 pdf 库
	// 这里只设置基本类型
	info.Type = PreviewPDF
}

// getThumbnailPath 获取缩略图路径
func (p *Preview) getThumbnailPath(path string) string {
	// 使用文件路径的hash作为缩略图文件名
	// 简化实现：返回相对路径
	return fmt.Sprintf("/thumbnails/%s.jpg", filepath.Base(path))
}

// GenerateThumbnail 生成缩略图
func (p *Preview) GenerateThumbnail(path string) (string, error) {
	if !p.config.Enabled {
		return "", fmt.Errorf("缩略图功能未启用")
	}

	mimeType := getMIMEType(path)

	// 只支持图片缩略图
	if !strings.HasPrefix(mimeType, "image/") {
		return "", fmt.Errorf("不支持的文件类型: %s", mimeType)
	}

	// 简化实现：返回缩略图路径
	// 实际实现应该使用 imaging 库生成缩略图
	thumbnailPath := p.getThumbnailPath(path)

	// 检查缩略图是否已存在且未过期
	if _, err := os.Stat(thumbnailPath); err == nil {
		return thumbnailPath, nil
	}

	// 生成缩略图（需要 imaging 库支持）
	// 这里只是占位实现
	p.logger.Info("生成缩略图",
		zap.String("source", path),
		zap.String("thumbnail", thumbnailPath))

	return thumbnailPath, nil
}

// GetPreviewContent 获取预览内容（用于文本/代码预览）
func (p *Preview) GetPreviewContent(path string, maxLines int) (string, error) {
	if maxLines <= 0 {
		maxLines = 100
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= maxLines {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
