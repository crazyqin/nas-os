// Package webshare 搜索索引辅助模块
// TrueSearch 搜索增强 - 文件名/内容搜索、文件类型过滤
package webshare

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SearchIndex 搜索索引
type SearchIndex struct {
	config     WebShareConfig
	mu         sync.RWMutex
	fileIndex  map[string]*IndexedFile // 相对路径 -> IndexedFile
	nameIndex  map[string][]string     // 名称关键词 -> 路径列表
	typeIndex  map[string][]string     // 文件类型 -> 路径列表
	extIndex   map[string][]string     // 扩展名 -> 路径列表
	excluded   map[string]bool         // 排除的路径
	fileTypes  *FileTypeRegistry
}

// IndexedFile 已索引的文件
type IndexedFile struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Ext          string `json:"ext"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	ModTime      string `json:"modTime"`
	IsDir        bool   `json:"isDir"`
	ContentIndex string `json:"contentIndex,omitempty"` // 内容索引（简化版）
}

// FileTypeRegistry 文件类型注册表
type FileTypeRegistry struct {
	types map[string]string // ext -> type
}

// NewFileTypeRegistry 创建文件类型注册表
func NewFileTypeRegistry() *FileTypeRegistry {
	return &FileTypeRegistry{
		types: map[string]string{
			// 图片
			".jpg":  "image",
			".jpeg": "image",
			".png":  "image",
			".gif":  "image",
			".webp": "image",
			".bmp":  "image",
			".svg":  "image",
			".heic": "image",
			".heif": "image",
			".tiff": "image",
			".tif":  "image",
			".ico":  "image",
			// 视频
			".mp4":  "video",
			".mkv":  "video",
			".avi":  "video",
			".mov":  "video",
			".wmv":  "video",
			".flv":  "video",
			".webm": "video",
			".m4v":  "video",
			".mpeg": "video",
			".mpg":  "video",
			".3gp":  "video",
			// 音频
			".mp3":  "audio",
			".wav":  "audio",
			".flac": "audio",
			".aac":  "audio",
			".ogg":  "audio",
			".wma":  "audio",
			".m4a":  "audio",
			".ape":  "audio",
			// 文档
			".pdf":  "document",
			".doc":  "document",
			".docx": "document",
			".xls":  "document",
			".xlsx": "document",
			".ppt":  "document",
			".pptx": "document",
			".txt":  "document",
			".rtf":  "document",
			".odt":  "document",
			".ods":  "document",
			".odp":  "document",
			// 代码
			".js":   "code",
			".ts":   "code",
			".py":   "code",
			".go":   "code",
			".java": "code",
			".c":    "code",
			".cpp":  "code",
			".h":    "code",
			".css":  "code",
			".html": "code",
			".json": "code",
			".xml":  "code",
			".yaml": "code",
			".yml":  "code",
			".md":   "code",
			".sh":   "code",
			".sql":  "code",
			// 压缩包
			".zip":  "archive",
			".rar":  "archive",
			".7z":   "archive",
			".tar":  "archive",
			".gz":   "archive",
			".bz2":  "archive",
		},
	}
}

// GetType 获取文件类型
func (r *FileTypeRegistry) GetType(ext string) string {
	ext = strings.ToLower(ext)
	if t, ok := r.types[ext]; ok {
		return t
	}
	return "other"
}

// NewSearchIndex 创建搜索索引
func NewSearchIndex(config WebShareConfig) *SearchIndex {
	si := &SearchIndex{
		config:    config,
		fileIndex: make(map[string]*IndexedFile),
		nameIndex: make(map[string][]string),
		typeIndex: make(map[string][]string),
		extIndex:  make(map[string][]string),
		excluded:  make(map[string]bool),
		fileTypes: NewFileTypeRegistry(),
	}

	// 初始化排除列表
	for _, path := range config.IndexExcluded {
		si.excluded[path] = true
	}

	return si
}

// BuildIndex 构建索引
func (si *SearchIndex) BuildIndex(basePath string) error {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return err
	}

	return si.walkDirectory(absBase, "")
}

// walkDirectory 遍历目录
func (si *SearchIndex) walkDirectory(absPath, relPath string) error {
	// 检查排除
	if si.excluded[relPath] {
		return nil
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		log.Printf("读取目录失败 %s: %v", absPath, err)
		return nil // 继续
	}

	for _, entry := range entries {
		name := entry.Name()
		childRelPath := filepath.Join(relPath, name)
		childAbsPath := filepath.Join(absPath, name)

		// 跳过排除项
		if si.excluded[childRelPath] {
			continue
		}

		// 跳过隐藏文件（除非配置允许）
		if strings.HasPrefix(name, ".") && !si.config.EnableHiddenShow {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))
		fileType := si.fileTypes.GetType(ext)

		indexed := &IndexedFile{
			Path:    childRelPath,
			Name:    name,
			Ext:     ext,
			Type:    fileType,
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			IsDir:   entry.IsDir(),
		}

		si.mu.Lock()
		// 文件索引
		si.fileIndex[childRelPath] = indexed

		// 名称索引 - 分词
		nameWords := si.tokenize(name)
		for _, word := range nameWords {
			si.nameIndex[word] = append(si.nameIndex[word], childRelPath)
		}

		// 类型索引
		si.typeIndex[fileType] = append(si.typeIndex[fileType], childRelPath)

		// 扩展名索引
		if ext != "" {
			si.extIndex[ext] = append(si.extIndex[ext], childRelPath)
		}
		si.mu.Unlock()

		// 递归目录
		if entry.IsDir() {
			si.walkDirectory(childAbsPath, childRelPath)
		}
	}

	return nil
}

// tokenize 分词
func (si *SearchIndex) tokenize(name string) []string {
	// 移除扩展名
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// 分词：按空格、下划线、连字符分割
	words := strings.FieldsFunc(name, func(c rune) bool {
		return c == ' ' || c == '_' || c == '-' || c == '.' || c == '(' || c == ')'
	})

	// 转小写
	result := make([]string, len(words))
	for i, w := range words {
		result[i] = strings.ToLower(w)
	}

	return result
}

// Search 搜索
func (si *SearchIndex) Search(query, basePath, fileType string, minSize, maxSize int64) ([]FileItem, error) {
	results := make([]FileItem, 0)

	si.mu.RLock()
	defer si.mu.RUnlock()

	queryLower := strings.ToLower(query)

	// 名称搜索
	var matchedPaths []string
	if query != "" {
		// 关键词匹配
		queryWords := strings.FieldsFunc(queryLower, func(c rune) bool {
			return c == ' '
		})

		for _, word := range queryWords {
			if paths, ok := si.nameIndex[word]; ok {
				matchedPaths = append(matchedPaths, paths...)
			}
		}

		// 模糊匹配（名称包含关键词）
		for path, file := range si.fileIndex {
			if strings.Contains(strings.ToLower(file.Name), queryLower) {
				matchedPaths = append(matchedPaths, path)
			}
		}
	} else {
		// 无关键词，返回全部
		for path := range si.fileIndex {
			matchedPaths = append(matchedPaths, path)
		}
	}

	// 过滤结果
	for _, path := range matchedPaths {
		file, ok := si.fileIndex[path]
		if !ok {
			continue
		}

		// 路径过滤
		if basePath != "" && !strings.HasPrefix(file.Path, basePath) {
			continue
		}

		// 文件类型过滤
		if fileType != "" && file.Type != fileType {
			continue
		}

		// 大小过滤
		if minSize > 0 && file.Size < minSize {
			continue
		}
		if maxSize > 0 && file.Size > maxSize {
			continue
		}

		// 转换为 FileItem
		item := FileItem{
			Name:         file.Name,
			Path:         file.Path,
			RelativePath: file.Path,
			Size:         file.Size,
			IsDir:        file.IsDir,
			Type:         file.Type,
			Extension:    file.Ext,
			IsHidden:     strings.HasPrefix(file.Name, "."),
		}

		results = append(results, item)
	}

	// 去重
	seen := make(map[string]bool)
	uniqueResults := make([]FileItem, 0)
	for _, item := range results {
		if !seen[item.Path] {
			seen[item.Path] = true
			uniqueResults = append(uniqueResults, item)
		}
	}

	return uniqueResults, nil
}

// UpdateIndex 更新索引（增量）
func (si *SearchIndex) UpdateIndex(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// 移除旧索引
	si.mu.Lock()
	if old, ok := si.fileIndex[path]; ok {
		// 清除名称索引
		nameWords := si.tokenize(old.Name)
		for _, word := range nameWords {
			si.removeFromSlice(si.nameIndex[word], path)
		}
		// 清除类型索引
		si.removeFromSlice(si.typeIndex[old.Type], path)
		// 清除扩展名索引
		if old.Ext != "" {
			si.removeFromSlice(si.extIndex[old.Ext], path)
		}
		delete(si.fileIndex, path)
	}
	si.mu.Unlock()

	// 重新索引
	info, err := os.Stat(absPath)
	if err != nil {
		return nil // 文件不存在，已删除
	}

	ext := strings.ToLower(filepath.Ext(filepath.Base(path)))
	fileType := si.fileTypes.GetType(ext)

	indexed := &IndexedFile{
		Path:    path,
		Name:    filepath.Base(path),
		Ext:     ext,
		Type:    fileType,
		Size:    info.Size(),
		ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		IsDir:   info.IsDir(),
	}

	si.mu.Lock()
	si.fileIndex[path] = indexed
	nameWords := si.tokenize(indexed.Name)
	for _, word := range nameWords {
		si.nameIndex[word] = append(si.nameIndex[word], path)
	}
	si.typeIndex[fileType] = append(si.typeIndex[fileType], path)
	if ext != "" {
		si.extIndex[ext] = append(si.extIndex[ext], path)
	}
	si.mu.Unlock()

	return nil
}

// removeFromSlice 从切片中移除元素
func (si *SearchIndex) removeFromSlice(slice []string, item string) []string {
	for i, s := range slice {
		if s == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// ClearIndex 清除索引
func (si *SearchIndex) ClearIndex() {
	si.mu.Lock()
	si.fileIndex = make(map[string]*IndexedFile)
	si.nameIndex = make(map[string][]string)
	si.typeIndex = make(map[string][]string)
	si.extIndex = make(map[string][]string)
	si.mu.Unlock()
}

// GetStats 获取索引统计
func (si *SearchIndex) GetStats() map[string]interface{} {
	si.mu.RLock()
	defer si.mu.RUnlock()

	typeCounts := make(map[string]int)
	for t, paths := range si.typeIndex {
		typeCounts[t] = len(paths)
	}

	extCounts := make(map[string]int)
	for ext, paths := range si.extIndex {
		extCounts[ext] = len(paths)
	}

	return map[string]interface{}{
		"totalFiles": len(si.fileIndex),
		"typeCounts": typeCounts,
		"extCounts":  extCounts,
		"nameIndex":  len(si.nameIndex),
	}
}