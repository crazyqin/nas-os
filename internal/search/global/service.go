package global

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Searcher 全局搜索服务
// 对标: TrueNAS Electric Eel全局搜索、群晖Universal Search

// SearchResult 搜索结果
type SearchResult struct {
	Type     string    // 结果类型 (page/setting/file/log)
	Title    string    // 标题
	Path     string    // 路径/URL
	Snippet  string    // 摘要片段
	Score    float64   // 搜索分数
	Icon     string    // 图标
	LastMod  time.Time // 最后修改时间
}

// IndexItem 索引项
type IndexItem struct {
	ID       string
	Type     string
	Title    string
	Path     string
	Content  string
	Keywords []string
	LastMod  time.Time
}

// GlobalSearchService 全局搜索服务
// 使用Bleve索引引擎，需添加依赖: go get github.com/blevesearch/bleve/v2
type GlobalSearchService struct {
	indices   map[string]*IndexItem // 内存索引 (开发阶段)
	mu        sync.RWMutex
	indexPath string
}

// NewGlobalSearchService 创建全局搜索服务
func NewGlobalSearchService(indexPath string) (*GlobalSearchService, error) {
	// 开发阶段使用内存索引
	// 生产环境集成Bleve
	
	return &GlobalSearchService{
		indices:   make(map[string]*IndexItem),
		indexPath: indexPath,
	}, nil
}

// Search 全局搜索
func (s *GlobalSearchService) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// 简化搜索实现 (开发阶段)
	// 生产环境使用Bleve全文搜索
	results := make([]SearchResult, 0)
	
	queryLower := strings.ToLower(query)
	for _, item := range s.indices {
		// 匹配标题或内容
		if strings.Contains(strings.ToLower(item.Title), queryLower) ||
			strings.Contains(strings.ToLower(item.Content), queryLower) {
			results = append(results, SearchResult{
				Type:    item.Type,
				Title:   item.Title,
				Path:    item.Path,
				Snippet: truncate(item.Content, 100),
				Score:   1.0,
			})
		}
	}
	
	// 限制结果数量
	if len(results) > limit {
		results = results[:limit]
	}
	
	return results, nil
}

// SearchByType 按类型搜索
func (s *GlobalSearchService) SearchByType(ctx context.Context, query string, types []string, limit int) ([]SearchResult, error) {
	allResults, err := s.Search(ctx, query, limit*2)
	if err != nil {
		return nil, err
	}
	
	// 过滤类型
	filtered := make([]SearchResult, 0)
	for _, r := range allResults {
		if contains(types, r.Type) {
			filtered = append(filtered, r)
		}
	}
	
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	
	return filtered, nil
}

// QuickSearch 快速搜索 (返回前10个匹配)
func (s *GlobalSearchService) QuickSearch(ctx context.Context, query string) ([]SearchResult, error) {
	return s.Search(ctx, query, 10)
}

// IndexPage 索引页面
func (s *GlobalSearchService) IndexPage(ctx context.Context, id, title, path, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	item := &IndexItem{
		ID:      "page:" + id,
		Type:    "page",
		Title:   title,
		Path:    path,
		Content: content,
		LastMod: time.Now(),
	}
	s.indices[item.ID] = item
	return nil
}

// IndexSetting 索索设置项
func (s *GlobalSearchService) IndexSetting(ctx context.Context, id, title, path, description string, keywords []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	item := &IndexItem{
		ID:       "setting:" + id,
		Type:     "setting",
		Title:    title,
		Path:     path,
		Content:  description,
		Keywords: keywords,
		LastMod:  time.Now(),
	}
	s.indices[item.ID] = item
	return nil
}

// IndexFile 索引文件
func (s *GlobalSearchService) IndexFile(ctx context.Context, path, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	item := &IndexItem{
		ID:      "file:" + path,
		Type:    "file",
		Title:   extractFilename(path),
		Path:    path,
		Content: content,
		LastMod: time.Now(),
	}
	s.indices[item.ID] = item
	return nil
}

// IndexLog 索引日志
func (s *GlobalSearchService) IndexLog(ctx context.Context, id, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	item := &IndexItem{
		ID:      "log:" + id,
		Type:    "log",
		Title:   "Log Entry",
		Path:    "/logs/" + id,
		Content: content,
		LastMod: time.Now(),
	}
	s.indices[item.ID] = item
	return nil
}

// Delete 删除索引项
func (s *GlobalSearchService) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.indices, id)
	return nil
}

// BatchIndex 批量索引
func (s *GlobalSearchService) BatchIndex(ctx context.Context, items []IndexItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range items {
		s.indices[item.ID] = &item
	}
	return nil
}

// Stats 索索统计
func (s *GlobalSearchService) Stats() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return uint64(len(s.indices)), nil
}

// Close 关闭索引
func (s *GlobalSearchService) Close() error {
	return nil
}

// Suggestions 搜索建议 (自动补全)
func (s *GlobalSearchService) Suggestions(ctx context.Context, prefix string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	suggestions := make([]string, 0)
	prefixLower := strings.ToLower(prefix)
	
	for _, item := range s.indices {
		if strings.HasPrefix(strings.ToLower(item.Title), prefixLower) {
			suggestions = append(suggestions, item.Title)
		}
	}
	
	return suggestions
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func extractFilename(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}

// KeyboardShortcut 快捷键配置
type KeyboardShortcut struct {
	Key      string // Cmd/Ctrl + K
	Action   string // open_search
	Enabled  bool
}

// DefaultShortcut 默认快捷键
func DefaultShortcut() KeyboardShortcut {
	return KeyboardShortcut{
		Key:     "Cmd/Ctrl+K",
		Action:  "open_search",
		Enabled: true,
	}
}

// SearchScope 搜索范围配置
type SearchScope struct {
	Pages    bool
	Settings bool
	Files    bool
	Logs     bool
}

// DefaultScope 默认搜索范围
func DefaultScope() SearchScope {
	return SearchScope{
		Pages:    true,
		Settings: true,
		Files:    false, // 文件搜索可选
		Logs:     false, // 日志搜索可选
	}
}