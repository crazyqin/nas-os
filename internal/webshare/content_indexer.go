// Package webshare 内容索引器
// 提供文件内容索引、元数据提取、EXIF信息解析能力
// 参考: TrueNAS Spotlight Search
package webshare

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"go.uber.org/zap"
)

// ContentIndexer 内容索引器
// 负责索引文件内容、元数据、EXIF信息
type ContentIndexer struct {
	config       WebShareConfig
	logger       *zap.Logger
	mu           sync.RWMutex
	index        map[string]*FileMetadata // path -> metadata
	tagIndex     map[string][]string      // tag -> paths
	metadataPool sync.Pool
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	stats        IndexerStats
}

// FileMetadata 文件元数据
type FileMetadata struct {
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Ext         string            `json:"ext"`
	Size        int64             `json:"size"`
	ModTime     time.Time         `json:"modTime"`
	ContentType string            `json:"contentType"`
	FileType    string            `json:"fileType"` // image, video, audio, document, code, archive, other

	// 内容信息
	TextContent string   `json:"textContent,omitempty"` // 提取的文本内容
	Excerpt     string   `json:"excerpt,omitempty"`     // 摘要
	Keywords    []string `json:"keywords,omitempty"`    // 关键词
	WordCount   int      `json:"wordCount,omitempty"`   // 词数
	LineCount   int      `json:"lineCount,omitempty"`   // 行数
	Language    string   `json:"language,omitempty"`     // 语言

	// 图片 EXIF 信息
	EXIF *EXIFData `json:"exif,omitempty"`

	// 标签
	Tags []string `json:"tags,omitempty"`

	// 索引状态
	IndexedAt    time.Time `json:"indexedAt"`
	IndexVersion int       `json:"indexVersion"`
}

// EXIFData EXIF 元数据
type EXIFData struct {
	CameraMake     string    `json:"cameraMake,omitempty"`
	CameraModel    string    `json:"cameraModel,omitempty"`
	DateTime       time.Time `json:"dateTime,omitempty"`
	DateTimeOriginal time.Time `json:"dateTimeOriginal,omitempty"`
	Width          int       `json:"width,omitempty"`
	Height         int       `json:"height,omitempty"`
	Orientation    int       `json:"orientation,omitempty"`
	ExposureTime   string    `json:"exposureTime,omitempty"`
	FNumber        float64   `json:"fNumber,omitempty"`
	ISO            int       `json:"iso,omitempty"`
	FocalLength    float64   `json:"focalLength,omitempty"`
	GPSLatitude    float64   `json:"gpsLatitude,omitempty"`
	GPSLongitude   float64   `json:"gpsLongitude,omitempty"`
	Software       string    `json:"software,omitempty"`
	Artist         string    `json:"artist,omitempty"`
	Copyright      string    `json:"copyright,omitempty"`
	Description    string    `json:"description,omitempty"`
}

// IndexerStats 索引器统计
type IndexerStats struct {
	TotalFiles    int64     `json:"totalFiles"`
	IndexedFiles  int64     `json:"indexedFiles"`
	FailedFiles   int64     `json:"failedFiles"`
	TotalBytes    int64     `json:"totalBytes"`
	IndexedBytes  int64     `json:"indexedBytes"`
	LastIndexed   time.Time `json:"lastIndexed"`
	IndexDuration time.Duration `json:"indexDuration"`
	FilesByType   map[string]int64 `json:"filesByType"`
}

// IndexRequest 索引请求
type IndexRequest struct {
	Path       string `json:"path"`       // 索引路径
	Recursive  bool   `json:"recursive"`  // 递归索引
	ForceReindex bool `json:"forceReindex"` // 强制重新索引
	MaxDepth   int    `json:"maxDepth"`   // 最大深度
}

// IndexResponse 索引响应
type IndexResponse struct {
	TotalFiles   int64         `json:"totalFiles"`
	IndexedFiles int64         `json:"indexedFiles"`
	FailedFiles  int64         `json:"failedFiles"`
	Took         time.Duration `json:"took"`
	Errors       []string      `json:"errors,omitempty"`
}

// NewContentIndexer 创建内容索引器
func NewContentIndexer(config WebShareConfig, logger *zap.Logger) *ContentIndexer {
	ctx, cancel := context.WithCancel(context.Background())

	ci := &ContentIndexer{
		config:   config,
		logger:   logger,
		index:    make(map[string]*FileMetadata),
		tagIndex: make(map[string][]string),
		ctx:      ctx,
		cancel:   cancel,
		stats: IndexerStats{
			FilesByType: make(map[string]int64),
		},
	}

	ci.metadataPool = sync.Pool{
		New: func() interface{} {
			return &FileMetadata{}
		},
	}

	return ci
}

// Start 启动索引器
func (ci *ContentIndexer) Start() {
	ci.mu.Lock()
	ci.running = true
	ci.mu.Unlock()

	ci.logger.Info("内容索引器启动")
}

// Stop 停止索引器
func (ci *ContentIndexer) Stop() {
	ci.cancel()
	ci.mu.Lock()
	ci.running = false
	ci.mu.Unlock()

	ci.logger.Info("内容索引器停止")
}

// Index 索引指定路径
func (ci *ContentIndexer) Index(ctx context.Context, req IndexRequest) (*IndexResponse, error) {
	startTime := time.Now()

	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		return nil, fmt.Errorf("无效路径: %w", err)
	}

	// 验证路径存在
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("路径不存在: %w", err)
	}

	response := &IndexResponse{}
	var errors []string

	if info.IsDir() {
		// 索引目录
		err = ci.indexDirectory(ctx, absPath, req.Recursive, req.MaxDepth, 0, req.ForceReindex, response, &errors)
	} else {
		// 索引单个文件
		err = ci.indexFile(ctx, absPath, req.ForceReindex, response, &errors)
	}

	if err != nil {
		return nil, err
	}

	response.Took = time.Since(startTime)
	response.Errors = errors

	// 更新统计
	ci.mu.Lock()
	ci.stats.LastIndexed = time.Now()
	ci.stats.IndexDuration = response.Took
	ci.mu.Unlock()

	ci.logger.Info("索引完成",
		zap.Int64("total", response.TotalFiles),
		zap.Int64("indexed", response.IndexedFiles),
		zap.Int64("failed", response.FailedFiles),
		zap.Duration("took", response.Took),
	)

	return response, nil
}

// indexDirectory 索引目录
func (ci *ContentIndexer) indexDirectory(ctx context.Context, dirPath string, recursive bool, maxDepth, currentDepth int, forceReindex bool, response *IndexResponse, errors *[]string) error {
	// 检查深度限制
	if maxDepth > 0 && currentDepth >= maxDepth {
		return nil
	}

	// 检查上下文取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		*errors = append(*errors, fmt.Sprintf("读取目录失败 %s: %v", dirPath, err))
		return nil
	}

	for _, entry := range entries {
		name := entry.Name()

		// 跳过隐藏文件
		if strings.HasPrefix(name, ".") {
			continue
		}

		// 跳过排除路径
		childPath := filepath.Join(dirPath, name)
		relPath, _ := filepath.Rel(ci.config.BaseDir, childPath)
		if ci.isExcluded(relPath) {
			continue
		}

		if entry.IsDir() {
			if recursive {
				if err := ci.indexDirectory(ctx, childPath, recursive, maxDepth, currentDepth+1, forceReindex, response, errors); err != nil {
					return err
				}
			}
		} else {
			if err := ci.indexFile(ctx, childPath, forceReindex, response, errors); err != nil {
				*errors = append(*errors, fmt.Sprintf("索引文件失败 %s: %v", childPath, err))
			}
		}
	}

	return nil
}

// indexFile 索引单个文件
func (ci *ContentIndexer) indexFile(ctx context.Context, filePath string, forceReindex bool, response *IndexResponse, errors *[]string) error {
	// 检查上下文取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	response.TotalFiles++

	info, err := os.Stat(filePath)
	if err != nil {
		response.FailedFiles++
		return err
	}

	// 检查是否需要重新索引
	relPath, _ := filepath.Rel(ci.config.BaseDir, filePath)
	if !forceReindex {
		ci.mu.RLock()
		existing, ok := ci.index[relPath]
		ci.mu.RUnlock()
		if ok && existing.ModTime.Equal(info.ModTime()) {
			return nil // 已索引且未修改
		}
	}

	// 创建元数据
	metadata := &FileMetadata{}

	// 重置元数据
	*metadata = FileMetadata{
		Path:        relPath,
		Name:        info.Name(),
		Ext:         strings.ToLower(filepath.Ext(info.Name())),
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		IndexedAt:   time.Now(),
		IndexVersion: 1,
	}

	// 获取文件类型
	metadata.FileType = ci.getFileType(metadata.Ext)
	metadata.ContentType = ci.getContentType(metadata.Ext)

	// 提取内容
	if err := ci.extractContent(filePath, metadata); err != nil {
		ci.logger.Debug("内容提取失败", zap.String("path", filePath), zap.Error(err))
	}

	// 提取 EXIF（图片文件）
	if metadata.FileType == "image" {
		if err := ci.extractEXIF(filePath, metadata); err != nil {
			ci.logger.Debug("EXIF提取失败", zap.String("path", filePath), zap.Error(err))
		}
	}

	// 提取关键词
	metadata.Keywords = ci.extractKeywords(metadata.TextContent, metadata.Name)

	// 存储索引
	ci.mu.Lock()
	ci.index[relPath] = metadata
	ci.stats.IndexedFiles++
	ci.stats.IndexedBytes += info.Size()
	ci.stats.FilesByType[metadata.FileType]++

	// 更新标签索引
	for _, tag := range metadata.Tags {
		ci.tagIndex[tag] = append(ci.tagIndex[tag], relPath)
	}
	ci.mu.Unlock()

	response.IndexedFiles++
	response.TotalFiles++

	return nil
}

// extractContent 提取文件内容
func (ci *ContentIndexer) extractContent(filePath string, metadata *FileMetadata) error {
	ext := metadata.Ext

	// 只提取文本类文件内容
	if !ci.isTextFile(ext) {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 限制读取大小（最大 5MB）
	reader := bufio.NewReader(io.LimitReader(file, 5*1024*1024))
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	text := string(content)
	metadata.TextContent = text
	metadata.Language = ci.detectLanguage(text)
	metadata.WordCount = len(strings.Fields(text))
	metadata.LineCount = strings.Count(text, "\n") + 1

	// 创建摘要
	if len(text) > 300 {
		metadata.Excerpt = text[:300] + "..."
	} else {
		metadata.Excerpt = text
	}

	return nil
}

// extractEXIF 提取 EXIF 信息
func (ci *ContentIndexer) extractEXIF(filePath string, metadata *FileMetadata) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return err
	}

	exifData := &EXIFData{}

	// 相机信息
	if make, err := x.Get(exif.Make); err == nil {
		exifData.CameraMake, _ = make.StringVal()
	}
	if model, err := x.Get(exif.Model); err == nil {
		exifData.CameraModel, _ = model.StringVal()
	}

	// 日期时间
	if dt, err := x.DateTime(); err == nil {
		exifData.DateTime = dt
	}

	// 图片尺寸
	if w, err := x.Get(exif.PixelXDimension); err == nil {
		exifData.Width, _ = w.Int(0)
	}
	if h, err := x.Get(exif.PixelYDimension); err == nil {
		exifData.Height, _ = h.Int(0)
	}

	// 方向
	if orient, err := x.Get(exif.Orientation); err == nil {
		exifData.Orientation, _ = orient.Int(0)
	}

	// 曝光信息
	if et, err := x.Get(exif.ExposureTime); err == nil {
		if rat, err := et.Rat(0); err == nil {
			exifData.ExposureTime = rat.String()
		}
	}

	if fn, err := x.Get(exif.FNumber); err == nil {
		if rat, err := fn.Rat(0); err == nil {
			exifData.FNumber, _ = rat.Float64()
		}
	}

	if iso, err := x.Get(exif.ISOSpeedRatings); err == nil {
		if v, err := iso.Int(0); err == nil {
			exifData.ISO = v
		}
	}

	if fl, err := x.Get(exif.FocalLength); err == nil {
		if rat, err := fl.Rat(0); err == nil {
			exifData.FocalLength, _ = rat.Float64()
		}
	}

	// GPS 信息
	if lat, long, err := x.LatLong(); err == nil {
		exifData.GPSLatitude = lat
		exifData.GPSLongitude = long
	}

	// 其他信息
	if sw, err := x.Get(exif.Software); err == nil {
		exifData.Software, _ = sw.StringVal()
	}
	if artist, err := x.Get(exif.Artist); err == nil {
		exifData.Artist, _ = artist.StringVal()
	}
	if copyright, err := x.Get(exif.Copyright); err == nil {
		exifData.Copyright, _ = copyright.StringVal()
	}

	metadata.EXIF = exifData

	// 从 EXIF 提取标签
	if exifData.CameraMake != "" {
		metadata.Tags = append(metadata.Tags, "camera:"+exifData.CameraMake)
	}
	if exifData.CameraModel != "" {
		metadata.Tags = append(metadata.Tags, "model:"+exifData.CameraModel)
	}
	if !exifData.DateTime.IsZero() {
		metadata.Tags = append(metadata.Tags, "year:"+exifData.DateTime.Format("2006"))
	}

	return nil
}

// extractKeywords 提取关键词
func (ci *ContentIndexer) extractKeywords(content, filename string) []string {
	keywords := make([]string, 0)
	seen := make(map[string]bool)

	// 从文件名提取
	nameWords := strings.FieldsFunc(strings.TrimSuffix(filename, filepath.Ext(filename)), func(c rune) bool {
		return c == ' ' || c == '_' || c == '-' || c == '.'
	})
	for _, w := range nameWords {
		w = strings.ToLower(w)
		if len(w) >= 2 && !seen[w] {
			keywords = append(keywords, w)
			seen[w] = true
		}
	}

	// 从内容提取
	if content != "" {
		words := strings.Fields(strings.ToLower(content))
		wordCount := make(map[string]int)
		for _, w := range words {
			if len(w) >= 3 {
				wordCount[w]++
			}
		}

		// 选择高频词
		for word, count := range wordCount {
			if count >= 2 && !seen[word] {
				keywords = append(keywords, word)
				seen[word] = true
			}
			if len(keywords) >= 30 {
				break
			}
		}
	}

	return keywords
}

// isTextFile 判断是否为文本文件
func (ci *ContentIndexer) isTextFile(ext string) bool {
	textExts := map[string]bool{
		".txt": true, ".md": true, ".rst": true, ".log": true,
		".csv": true, ".tsv": true, ".json": true, ".yaml": true,
		".yml": true, ".xml": true, ".toml": true, ".ini": true,
		".conf": true, ".cfg": true, ".properties": true,
		".html": true, ".htm": true, ".css": true, ".js": true,
		".ts": true, ".vue": true, ".jsx": true, ".tsx": true,
		".py": true, ".go": true, ".java": true, ".c": true,
		".cpp": true, ".h": true, ".hpp": true, ".rs": true,
		".rb": true, ".php": true, ".pl": true, ".sh": true,
		".sql": true, ".lua": true, ".swift": true, ".kt": true,
		".scala": true, ".r": true, ".m": true, ".mm": true,
	}
	return textExts[ext]
}

// detectLanguage 检测语言
func (ci *ContentIndexer) detectLanguage(text string) string {
	chineseCount := 0
	englishCount := 0

	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			englishCount++
		}
	}

	if chineseCount > englishCount {
		return "zh"
	} else if englishCount > 0 {
		return "en"
	}
	return "unknown"
}

// getFileType 获取文件类型
func (ci *ContentIndexer) getFileType(ext string) string {
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true, ".svg": true, ".ico": true,
		".heic": true, ".heif": true, ".tiff": true, ".tif": true,
	}
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
	}
	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".flac": true, ".aac": true,
		".ogg": true, ".wma": true, ".m4a": true, ".ape": true,
	}
	docExts := map[string]bool{
		".pdf": true, ".doc": true, ".docx": true, ".xls": true,
		".xlsx": true, ".ppt": true, ".pptx": true, ".txt": true,
		".rtf": true, ".odt": true, ".ods": true, ".odp": true,
	}
	codeExts := map[string]bool{
		".js": true, ".ts": true, ".py": true, ".go": true,
		".java": true, ".c": true, ".cpp": true, ".h": true,
		".css": true, ".html": true, ".json": true, ".xml": true,
		".yaml": true, ".yml": true, ".md": true, ".sh": true,
	}
	archiveExts := map[string]bool{
		".zip": true, ".rar": true, ".7z": true, ".tar": true,
		".gz": true, ".bz2": true, ".xz": true,
	}

	switch {
	case imageExts[ext]:
		return "image"
	case videoExts[ext]:
		return "video"
	case audioExts[ext]:
		return "audio"
	case docExts[ext]:
		return "document"
	case codeExts[ext]:
		return "code"
	case archiveExts[ext]:
		return "archive"
	default:
		return "other"
	}
}

// getContentType 获取 MIME 类型
func (ci *ContentIndexer) getContentType(ext string) string {
	mimeTypes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp",
		".svg": "image/svg+xml", ".ico": "image/x-icon",
		".mp4": "video/mp4", ".mkv": "video/x-matroska", ".avi": "video/x-msvideo",
		".mov": "video/quicktime", ".wmv": "video/x-ms-wmv", ".flv": "video/x-flv",
		".webm": "video/webm", ".m4v": "video/mp4",
		".mp3": "audio/mpeg", ".wav": "audio/wav", ".flac": "audio/flac",
		".aac": "audio/aac", ".ogg": "audio/ogg", ".wma": "audio/x-ms-wma",
		".m4a": "audio/mp4", ".ape": "audio/ape",
		".pdf": "application/pdf", ".doc": "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls": "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt": "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt": "text/plain", ".rtf": "application/rtf",
		".zip": "application/zip", ".rar": "application/vnd.rar",
		".7z": "application/x-7z-compressed", ".tar": "application/x-tar",
		".gz": "application/gzip", ".bz2": "application/x-bzip2",
		".json": "application/json", ".xml": "application/xml",
		".html": "text/html", ".htm": "text/html",
		".css": "text/css", ".js": "application/javascript",
		".ts": "application/typescript", ".py": "text/x-python",
		".go": "text/x-go", ".java": "text/x-java",
		".c": "text/x-c", ".cpp": "text/x-c++", ".h": "text/x-c",
		".sh": "application/x-sh", ".md": "text/markdown",
		".yaml": "application/x-yaml", ".yml": "application/x-yaml",
		".sql": "application/sql", ".csv": "text/csv",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// isExcluded 检查是否在排除列表中
func (ci *ContentIndexer) isExcluded(relPath string) bool {
	for _, excluded := range ci.config.IndexExcluded {
		if strings.HasPrefix(relPath, excluded) {
			return true
		}
	}
	return false
}

// GetMetadata 获取文件元数据
func (ci *ContentIndexer) GetMetadata(path string) *FileMetadata {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.index[path]
}

// GetStats 获取索引统计
func (ci *ContentIndexer) GetStats() IndexerStats {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.stats
}

// GetAllMetadata 获取所有元数据
func (ci *ContentIndexer) GetAllMetadata() map[string]*FileMetadata {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	
	result := make(map[string]*FileMetadata, len(ci.index))
	for k, v := range ci.index {
		result[k] = v
	}
	return result
}

// SearchByTag 按标签搜索
func (ci *ContentIndexer) SearchByTag(tag string) []string {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.tagIndex[tag]
}

// AddTag 添加标签
func (ci *ContentIndexer) AddTag(path, tag string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if meta, ok := ci.index[path]; ok {
		// 检查标签是否已存在
		for _, t := range meta.Tags {
			if t == tag {
				return
			}
		}
		meta.Tags = append(meta.Tags, tag)
		ci.tagIndex[tag] = append(ci.tagIndex[tag], path)
	}
}

// RemoveTag 移除标签
func (ci *ContentIndexer) RemoveTag(path, tag string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if meta, ok := ci.index[path]; ok {
		for i, t := range meta.Tags {
			if t == tag {
				meta.Tags = append(meta.Tags[:i], meta.Tags[i+1:]...)
				break
			}
		}
	}

	// 从标签索引中移除
	paths := ci.tagIndex[tag]
	for i, p := range paths {
		if p == path {
			ci.tagIndex[tag] = append(paths[:i], paths[i+1:]...)
			break
		}
	}
}

// ClearIndex 清除索引
func (ci *ContentIndexer) ClearIndex() {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	ci.index = make(map[string]*FileMetadata)
	ci.tagIndex = make(map[string][]string)
	ci.stats = IndexerStats{
		FilesByType: make(map[string]int64),
	}
}
