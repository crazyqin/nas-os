// Package smartsearch 实现智能文件搜索功能
// 对标群晖 DSM 7.3 Universal Search 特性
// 支持全文搜索、语义搜索、AI 增强搜索
package smartsearch

import (
	"context"
	"strings"
	"sync"
	"time"
)

// SearchType 搜索类型
type SearchType string

const (
	SearchTypeFullText  SearchType = "fulltext"  // 全文搜索
	SearchTypeSemantic  SearchType = "semantic"   // 语义搜索
	SearchTypeAI        SearchType = "ai"         // AI 增强搜索
	SearchTypeMetadata  SearchType = "metadata"   // 元数据搜索
)

// FileType 文件类型
type FileType string

const (
	FileTypeDocument FileType = "document"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeOther    FileType = "other"
)

// SearchResult 搜索结果
type SearchResult struct {
	FileID      string    `json:"file_id"`
	FilePath    string    `json:"file_path"`
	FileName    string    `json:"file_name"`
	FileType    FileType  `json:"file_type"`
	FileSize    int64     `json:"file_size"`
	ModTime     time.Time `json:"mod_time"`
	Score       float64   `json:"score"`       // 相关性评分 0-1
	Snippet     string    `json:"snippet"`      // 匹配内容片段
	Highlighted string    `json:"highlighted"`  // 高亮显示
	Tags        []string  `json:"tags,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query      string     `json:"query"`
	Type       SearchType `json:"type"`
	FileTypes  []FileType `json:"file_types,omitempty"`
	Path       string     `json:"path,omitempty"`       // 限定搜索路径
	Tags       []string   `json:"tags,omitempty"`
	Labels     []string   `json:"labels,omitempty"`
	DateFrom   *time.Time `json:"date_from,omitempty"`
	DateTo     *time.Time `json:"date_to,omitempty"`
	SizeMin    *int64     `json:"size_min,omitempty"`
	SizeMax    *int64     `json:"size_max,omitempty"`
	SortBy     string     `json:"sort_by,omitempty"`     // relevance, date, size, name
	SortOrder  string     `json:"sort_order,omitempty"`  // asc, desc
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TenantID   string     `json:"tenant_id,omitempty"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Query      string          `json:"query"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	Results    []SearchResult  `json:"results"`
	Facets     *SearchFacets   `json:"facets,omitempty"`
	Suggestions []string       `json:"suggestions,omitempty"`
	Took       time.Duration   `json:"took"` // 搜索耗时
}

// SearchFacets 搜索分面（用于过滤）
type SearchFacets struct {
	FileTypes  map[string]int `json:"file_types"`
	Tags       map[string]int `json:"tags"`
	Labels     map[string]int `json:"labels"`
	DateRanges []DateRange    `json:"date_ranges"`
}

// DateRange 日期范围
type DateRange struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// IndexEntry 索引条目
type IndexEntry struct {
	FileID    string    `json:"file_id"`
	FilePath  string    `json:"file_path"`
	FileName  string    `json:"file_name"`
	FileType  FileType  `json:"file_type"`
	FileSize  int64     `json:"file_size"`
	ModTime   time.Time `json:"mod_time"`
	Content   string    `json:"content"`   // 文件内容（用于全文搜索）
	Tags      []string  `json:"tags"`
	Labels    []string  `json:"labels"`
	TenantID  string    `json:"tenant_id"`
	IndexedAt time.Time `json:"indexed_at"`
}

// Manager 智能搜索管理器
type Manager struct {
	mu          sync.RWMutex
	index       map[string]*IndexEntry // fileID -> entry
	storagePath string
}

// NewManager 创建搜索管理器
func NewManager(storagePath string) *Manager {
	return &Manager{
		index:       make(map[string]*IndexEntry),
		storagePath: storagePath,
	}
}

// IndexFile 索引文件
func (m *Manager) IndexFile(ctx context.Context, entry IndexEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.IndexedAt.IsZero() {
		entry.IndexedAt = time.Now()
	}
	m.index[entry.FileID] = &entry
	return nil
}

// RemoveFromIndex 从索引中移除
func (m *Manager) RemoveFromIndex(ctx context.Context, fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.index, fileID)
	return nil
}

// Search 执行搜索
func (m *Manager) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	start := time.Now()

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 默认分页
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	var results []SearchResult
	query := strings.ToLower(req.Query)

	for _, entry := range m.index {
		// 租户过滤
		if req.TenantID != "" && entry.TenantID != req.TenantID {
			continue
		}

		// 路径过滤
		if req.Path != "" && !strings.HasPrefix(entry.FilePath, req.Path) {
			continue
		}

		// 文件类型过滤
		if len(req.FileTypes) > 0 {
			found := false
			for _, ft := range req.FileTypes {
				if entry.FileType == ft {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 标签过滤
		if len(req.Tags) > 0 {
			if !hasAnyTag(entry.Tags, req.Tags) {
				continue
			}
		}

		// 日期过滤
		if req.DateFrom != nil && entry.ModTime.Before(*req.DateFrom) {
			continue
		}
		if req.DateTo != nil && entry.ModTime.After(*req.DateTo) {
			continue
		}

		// 大小过滤
		if req.SizeMin != nil && entry.FileSize < *req.SizeMin {
			continue
		}
		if req.SizeMax != nil && entry.FileSize > *req.SizeMax {
			continue
		}

		// 计算匹配分数
		score := calculateScore(entry, query)
		if score > 0 {
			results = append(results, SearchResult{
				FileID:   entry.FileID,
				FilePath: entry.FilePath,
				FileName: entry.FileName,
				FileType: entry.FileType,
				FileSize: entry.FileSize,
				ModTime:  entry.ModTime,
				Score:    score,
				Snippet:  generateSnippet(entry.Content, query),
				Tags:     entry.Tags,
				Labels:   entry.Labels,
			})
		}
	}

	// 排序
	sortResults(results, req.SortBy, req.SortOrder)

	// 分页
	total := len(results)
	startIdx := (req.Page - 1) * req.PageSize
	endIdx := startIdx + req.PageSize
	if startIdx > len(results) {
		startIdx = len(results)
	}
	if endIdx > len(results) {
		endIdx = len(results)
	}
	pagedResults := results[startIdx:endIdx]

	return &SearchResponse{
		Query:    req.Query,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Results:  pagedResults,
		Took:     time.Since(start),
	}, nil
}

// GetIndexStats 获取索引统计
func (m *Manager) GetIndexStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_files": len(m.index),
		"file_types":  make(map[string]int),
	}

	typeCount := stats["file_types"].(map[string]int)
	for _, entry := range m.index {
		typeCount[string(entry.FileType)]++
	}

	return stats, nil
}

// RebuildIndex 重建索引
func (m *Manager) RebuildIndex(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空现有索引
	m.index = make(map[string]*IndexEntry)
	// TODO: 实现实际的文件扫描和索引逻辑
	return nil
}

func calculateScore(entry *IndexEntry, query string) float64 {
	score := 0.0

	// 文件名匹配
	if strings.Contains(strings.ToLower(entry.FileName), query) {
		score += 0.5
	}

	// 内容匹配
	if strings.Contains(strings.ToLower(entry.Content), query) {
		score += 0.3
	}

	// 标签匹配
	for _, tag := range entry.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			score += 0.1
			break
		}
	}

	// 标题匹配
	if strings.Contains(strings.ToLower(entry.FileName), query) {
		score += 0.1
	}

	return score
}

func generateSnippet(content, query string) string {
	if content == "" {
		return ""
	}

	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)

	idx := strings.Index(lowerContent, lowerQuery)
	if idx == -1 {
		if len(content) > 200 {
			return content[:200] + "..."
		}
		return content
	}

	start := idx - 100
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 100
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	return snippet
}

func sortResults(results []SearchResult, sortBy, sortOrder string) {
	// 简化排序实现
	// 默认按相关性降序
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func hasAnyTag(fileTags, filterTags []string) bool {
	for _, ft := range fileTags {
		for _, filter := range filterTags {
			if strings.EqualFold(ft, filter) {
				return true
			}
		}
	}
	return false
}
