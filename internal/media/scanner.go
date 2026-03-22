package media

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Scanner 媒体文件扫描器
type Scanner struct {
	// 视频文件扩展名
	videoExts map[string]bool
	// 音频文件扩展名
	audioExts map[string]bool
	// 图片文件扩展名
	imageExts map[string]bool
	// 忽略的目录名
	ignoreDirs map[string]bool
	// 并发控制
	workers int
}

// ScannerConfig 扫描器配置
type ScannerConfig struct {
	Workers         int      // 并发工作数
	IgnoreDirs      []string // 忽略的目录
	CustomVideoExts []string // 自定义视频扩展名
	CustomAudioExts []string // 自定义音频扩展名
}

// NewScanner 创建扫描器
func NewScanner(config *ScannerConfig) *Scanner {
	s := &Scanner{
		videoExts: map[string]bool{
			".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
			".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
			".mpg": true, ".mpeg": true, ".3gp": true, ".rmvb": true,
			".ts": true, ".m2ts": true, ".vob": true, ".iso": true,
		},
		audioExts: map[string]bool{
			".mp3": true, ".flac": true, ".wav": true, ".aac": true,
			".ogg": true, ".wma": true, ".m4a": true, ".ape": true,
			".alac": true, ".dsd": true, ".dsf": true,
		},
		imageExts: map[string]bool{
			".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
			".bmp": true, ".webp": true, ".heic": true, ".raw": true,
			".tiff": true, ".tif": true,
		},
		ignoreDirs: map[string]bool{
			".git": true, ".svn": true, ".DS_Store": true,
			"thumbs": true, "Thumbnails": true, "@eaDir": true, // Synology
			"#recycle": true, ".Trash": true,
		},
		workers: 4,
	}

	if config != nil {
		if config.Workers > 0 {
			s.workers = config.Workers
		}
		for _, dir := range config.IgnoreDirs {
			s.ignoreDirs[dir] = true
		}
		for _, ext := range config.CustomVideoExts {
			s.videoExts[strings.ToLower(ext)] = true
		}
		for _, ext := range config.CustomAudioExts {
			s.audioExts[strings.ToLower(ext)] = true
		}
	}

	return s
}

// Scan 扫描指定路径
func (s *Scanner) Scan(ctx context.Context, rootPath string, mediaType Type) ([]*Item, error) {
	startTime := time.Now()

	// 检查路径是否存在
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("路径不存在: %s", rootPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("不是目录: %s", rootPath)
	}

	// 创建结果通道和工作池
	itemChan := make(chan *Item, 100)
	var items []*Item
	var itemsMu sync.Mutex

	// 收集结果的 goroutine
	done := make(chan struct{})
	go func() {
		for item := range itemChan {
			itemsMu.Lock()
			items = append(items, item)
			itemsMu.Unlock()
		}
		close(done)
	}()

	// 遍历文件系统
	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return nil // 忽略错误继续扫描
		}

		// 跳过忽略的目录
		if d.IsDir() {
			if s.ignoreDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}

		// 判断文件类型
		ext := strings.ToLower(filepath.Ext(path))
		var detectedType Type

		switch {
		case s.videoExts[ext]:
			detectedType = TypeMovie
		case s.audioExts[ext]:
			detectedType = TypeMusic
		case s.imageExts[ext]:
			detectedType = TypePhoto
		default:
			return nil // 不支持的文件类型
		}

		// 过滤媒体类型
		if mediaType != "" && mediaType != detectedType {
			return nil
		}

		// 获取文件信息
		info, err := d.Info()
		if err != nil {
			return nil
		}

		// 创建媒体项
		item := s.createItem(path, info, detectedType)
		itemChan <- item

		return nil
	})

	close(itemChan)
	<-done

	if err != nil && err != context.Canceled {
		return nil, err
	}

	// 按修改时间排序
	s.sortByModifiedTime(items)

	elapsed := time.Since(startTime)
	fmt.Printf("[Scanner] 扫描完成: %d 个文件, 耗时 %v\n", len(items), elapsed)

	return items, nil
}

// ScanDirectory 扫描单个目录（不递归）
func (s *Scanner) ScanDirectory(dirPath string, mediaType Type) ([]*Item, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var items []*Item
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		var detectedType Type

		switch {
		case s.videoExts[ext]:
			detectedType = TypeMovie
		case s.audioExts[ext]:
			detectedType = TypeMusic
		case s.imageExts[ext]:
			detectedType = TypePhoto
		default:
			continue
		}

		if mediaType != "" && mediaType != detectedType {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		item := s.createItem(filepath.Join(dirPath, entry.Name()), info, detectedType)
		items = append(items, item)
	}

	return items, nil
}

// createItem 创建媒体项
func (s *Scanner) createItem(path string, info fs.FileInfo, mediaType Type) *Item {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	now := time.Now()

	item := &Item{
		ID:           fmt.Sprintf("item_%s_%d", mediaType, now.UnixNano()),
		Path:         path,
		Name:         name,
		Type:         mediaType,
		Size:         info.Size(),
		ModifiedTime: info.ModTime(),
		Tags:         make([]string, 0),
	}

	// 从文件名提取信息
	s.extractInfoFromName(item)

	return item
}

// extractInfoFromName 从文件名提取信息
func (s *Scanner) extractInfoFromName(item *Item) {
	name := item.Name

	// 提取年份
	yearRegex := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	if matches := yearRegex.FindStringSubmatch(name); len(matches) > 0 {
		item.Tags = append(item.Tags, "year:"+matches[0])
	}

	// 提取季集信息 (S01E01, 1x01, etc.)
	seasonEpisodeRegex := regexp.MustCompile(`(?i)[sS](\d{1,2})[eE](\d{1,2})|(\d{1,2})x(\d{1,2})`)
	if matches := seasonEpisodeRegex.FindStringSubmatch(name); len(matches) > 0 {
		item.Type = TypeTV
		if matches[1] != "" && matches[2] != "" {
			item.Tags = append(item.Tags, fmt.Sprintf("S%sE%s", matches[1], matches[2]))
		} else if matches[3] != "" && matches[4] != "" {
			item.Tags = append(item.Tags, fmt.Sprintf("S%sE%s", matches[3], matches[4]))
		}
	}

	// 提取分辨率
	resolutionRegex := regexp.MustCompile(`(?i)\b(4k|2160p|1080p|720p|480p|hdr|bluray|web-dl|hdtv)\b`)
	if matches := resolutionRegex.FindAllString(name, -1); len(matches) > 0 {
		for _, m := range matches {
			item.Tags = append(item.Tags, strings.ToLower(m))
		}
	}

	// 提取视频编码
	codecRegex := regexp.MustCompile(`(?i)\b(x264|x265|h264|h265|hevc|avc|vp9|av1)\b`)
	if matches := codecRegex.FindAllString(name, -1); len(matches) > 0 {
		for _, m := range matches {
			item.Tags = append(item.Tags, strings.ToLower(m))
		}
	}

	// 清理文件名，生成搜索用的标题
	cleanName := s.cleanFileName(name)
	item.Tags = append(item.Tags, "searchTitle:"+cleanName)
}

// cleanFileName 清理文件名，提取用于搜索的标题
func (s *Scanner) cleanFileName(name string) string {
	// 移除常见的前缀和后缀
	replacements := []struct {
		regex   string
		replace string
	}{
		{`(?i)^\[.*?\]\s*`, ""},                                       // 移除 [xxx] 前缀
		{`(?i)\s*\(.*?\)\s*$`, ""},                                    // 移除 (xxx) 后缀
		{`(?i)\s*-\s*\d{4}.*$`, ""},                                   // 移除 - 2024 及之后内容
		{`(?i)\s*\[.*?\].*$`, ""},                                     // 移除 [xxx] 及之后内容
		{`(?i)\s*(S\d{1,2}E\d{1,2}|S\d{1,2}).*$`, ""},                 // 移除季集信息及之后内容
		{`(?i)\s*(4k|1080p|720p|480p|hdr|bluray|web-dl|hdtv).*$`, ""}, // 移除分辨率信息
		{`(?i)\s*(x264|x265|h264|h265|hevc|avc).*$`, ""},              // 移除编码信息
		{`[\._]`, " "},                                                // 替换点号和下划线为空格
		{`\s+`, " "},                                                  // 合并多个空格
	}

	result := name
	for _, r := range replacements {
		re := regexp.MustCompile(r.regex)
		result = re.ReplaceAllString(result, r.replace)
	}

	return strings.TrimSpace(result)
}

// sortByModifiedTime 按修改时间排序
func (s *Scanner) sortByModifiedTime(items []*Item) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].ModifiedTime.Before(items[j].ModifiedTime) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// DetectMediaType 从文件路径检测媒体类型
func (s *Scanner) DetectMediaType(path string) Type {
	ext := strings.ToLower(filepath.Ext(path))

	if s.videoExts[ext] {
		// 检查文件名是否包含季集信息
		name := filepath.Base(path)
		seasonEpisodeRegex := regexp.MustCompile(`(?i)[sS]\d{1,2}[eE]\d{1,2}|\d{1,2}x\d{1,2}`)
		if seasonEpisodeRegex.MatchString(name) {
			return TypeTV
		}
		return TypeMovie
	}

	if s.audioExts[ext] {
		return TypeMusic
	}

	if s.imageExts[ext] {
		return TypePhoto
	}

	return ""
}

// IsMediaFile 判断是否为媒体文件
func (s *Scanner) IsMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return s.videoExts[ext] || s.audioExts[ext] || s.imageExts[ext]
}

// GetFileExtension 获取支持的扩展名列表
func (s *Scanner) GetSupportedExtensions() map[Type][]string {
	result := make(map[Type][]string)

	var videos, audios, images []string
	for ext := range s.videoExts {
		videos = append(videos, ext)
	}
	for ext := range s.audioExts {
		audios = append(audios, ext)
	}
	for ext := range s.imageExts {
		images = append(images, ext)
	}

	result[TypeMovie] = videos
	result[TypeMusic] = audios
	result[TypePhoto] = images

	return result
}
