// Package filesearch 提供全局文件搜索功能
package filesearch

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Manager 文件搜索管理器.
type Manager struct {
	index    map[string]*SearchResult // 模拟索引
	tagIndex map[string][]string      // tag -> file paths
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		index:    make(map[string]*SearchResult),
		tagIndex: make(map[string][]string),
	}
}

// Search 执行搜索.
func (m *Manager) Search(req SearchRequest) (*SearchResponse, error) {
	if req.Query == "" && req.Type == "" && len(req.Tags) == 0 {
		return nil, fmt.Errorf("query is required")
	}

	start := time.Now()
	query := strings.ToLower(req.Query)

	var results []SearchResult
	for _, item := range m.index {
		// 路径过滤
		if req.Path != "" && !strings.HasPrefix(item.Path, req.Path) {
			continue
		}
		// 类型过滤
		if req.Type != "" && item.FileType != req.Type {
			continue
		}
		// 大小过滤
		if req.MinSize > 0 && item.Size < req.MinSize {
			continue
		}
		if req.MaxSize > 0 && item.Size > req.MaxSize {
			continue
		}
		// 时间过滤
		if !req.After.IsZero() && item.ModTime.Before(req.After) {
			continue
		}
		if !req.Before.IsZero() && item.ModTime.After(req.Before) {
			continue
		}

		// 标签过滤
		if len(req.Tags) > 0 {
			hasTag := false
			for _, tag := range req.Tags {
				for _, itemTag := range item.Tags {
					if strings.EqualFold(tag, itemTag) {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// 计算相关性
		if query != "" {
			score := calculateScore(query, item)
			if score > 0 {
				result := *item
				result.Score = score
				result.Highlight = highlightMatch(item.Name, query)
				results = append(results, result)
			}
		} else {
			// 空查询返回所有匹配项
			result := *item
			result.Score = 1.0
			results = append(results, result)
		}
	}

	// 排序
	sortResults(results, req.Sort, req.Order)

	total := len(results)

	// 分页
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	startIdx := req.Page * req.PageSize
	if startIdx >= len(results) {
		return &SearchResponse{
			Items:     []SearchResult{},
			Total:     total,
			Page:      req.Page,
			PageSize:  req.PageSize,
			QueryTime: time.Since(start).Milliseconds(),
		}, nil
	}
	endIdx := startIdx + req.PageSize
	if endIdx > len(results) {
		endIdx = len(results)
	}

	return &SearchResponse{
		Items:     results[startIdx:endIdx],
		Total:     total,
		Page:      req.Page,
		PageSize:  req.PageSize,
		QueryTime: time.Since(start).Milliseconds(),
		Facets:    buildFacets(results),
	}, nil
}

// IndexFile 添加文件到索引.
func (m *Manager) IndexFile(result *SearchResult) {
	m.index[result.Path] = result
	for _, tag := range result.Tags {
		m.tagIndex[tag] = append(m.tagIndex[tag], result.Path)
	}
}

// RemoveFromIndex 从索引中移除.
func (m *Manager) RemoveFromIndex(path string) {
	if item, ok := m.index[path]; ok {
		for _, tag := range item.Tags {
			paths := m.tagIndex[tag]
			for i, p := range paths {
				if p == path {
					m.tagIndex[tag] = append(paths[:i], paths[i+1:]...)
					break
				}
			}
		}
		delete(m.index, path)
	}
}

// IndexStatus 获取索引状态.
func (m *Manager) IndexStatus() IndexStatus {
	return IndexStatus{
		TotalFiles:    int64(len(m.index)),
		IndexedFiles:  int64(len(m.index)),
		LastIndexTime: time.Now(),
		IsIndexing:    false,
		Progress:      100,
	}
}

func calculateScore(query string, item *SearchResult) float64 {
	name := strings.ToLower(item.Name)
	path := strings.ToLower(item.Path)

	score := 0.0

	// 精确匹配文件名
	if strings.Contains(name, query) {
		score += 10.0
		if strings.HasPrefix(name, query) {
			score += 5.0
		}
	}

	// 路径匹配
	if strings.Contains(path, query) {
		score += 3.0
	}

	// 单词匹配
	words := strings.Fields(query)
	for _, word := range words {
		if strings.Contains(name, word) {
			score += 2.0
		}
	}

	// 标签匹配
	for _, tag := range item.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			score += 4.0
		}
	}

	return score
}

func highlightMatch(name, query string) string {
	lower := strings.ToLower(name)
	idx := strings.Index(lower, query)
	if idx < 0 {
		return name
	}
	return name[:idx] + "**" + name[idx:idx+len(query)] + "**" + name[idx+len(query):]
}

func sortResults(results []SearchResult, sortBy SortBy, order SortOrder) {
	sort.Slice(results, func(i, j int) bool {
		var less bool
		switch sortBy {
		case SortByName:
			less = strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
		case SortBySize:
			less = results[i].Size < results[j].Size
		case SortByDate:
			less = results[i].ModTime.Before(results[j].ModTime)
		case SortByType:
			less = string(results[i].FileType) < string(results[j].FileType)
		default: // relevance
			less = results[i].Score > results[j].Score
		}
		if order == SortDesc {
			return !less
		}
		return less
	})
}

func buildFacets(results []SearchResult) *SearchFacets {
	facets := &SearchFacets{
		FileTypes:  make(map[FileType]int),
		Extensions: make(map[string]int),
	}

	for _, r := range results {
		facets.FileTypes[r.FileType]++
		if r.Extension != "" {
			facets.Extensions[r.Extension]++
		}
	}

	return facets
}

// DetectFileType 根据扩展名检测文件类型.
func DetectFileType(ext string) FileType {
	ext = strings.ToLower(ext)
	switch ext {
	case ".doc", ".docx", ".pdf", ".txt", ".md", ".rtf", ".odt", ".xls", ".xlsx", ".ppt", ".pptx", ".csv":
		return FileTypeDocument
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg", ".tiff", ".ico":
		return FileTypeImage
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v":
		return FileTypeVideo
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".wma", ".m4a":
		return FileTypeAudio
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz":
		return FileTypeArchive
	case ".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".rs", ".rb", ".php", ".html", ".css", ".json", ".yaml", ".toml":
		return FileTypeCode
	default:
		return FileTypeOther
	}
}
