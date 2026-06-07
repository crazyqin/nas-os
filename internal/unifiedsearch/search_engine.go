package unifiedsearch

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// EngineSearchType 搜索源类型
type EngineSearchType string

const (
	SourceFile     EngineSearchType = "file"
	SourcePhoto    EngineSearchType = "photo"
	SourceDocument EngineSearchType = "document"
	SourceEmail    EngineSearchType = "email"
	SourceNote     EngineSearchType = "note"
	SourceVideo    EngineSearchType = "video"
	SourceMusic    EngineSearchType = "music"
	SourceApp      EngineSearchType = "app"
)

// EngineSearchItem 搜索索引项
type EngineSearchItem struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Path      string           `json:"path"`
	Type      EngineSearchType `json:"type"`
	Size      int64            `json:"size"`
	Tags      []string         `json:"tags,omitempty"`
	Content   string           `json:"content,omitempty"` // 全文索引内容
	MimeType  string           `json:"mime_type,omitempty"`
	Owner     string           `json:"owner,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Thumbnail string           `json:"thumbnail,omitempty"`
}

// EngineSearchResult 搜索结果
type EngineSearchResult struct {
	Item       *EngineSearchItem `json:"item"`
	Score      float64           `json:"score"`
	Highlights []string          `json:"highlights,omitempty"`
}

// EngineSearchQuery 搜索查询
type EngineSearchQuery struct {
	Keyword  string             `json:"keyword"`
	Types    []EngineSearchType `json:"types,omitempty"`
	Tags     []string           `json:"tags,omitempty"`
	Owner    string             `json:"owner,omitempty"`
	DateFrom *time.Time         `json:"date_from,omitempty"`
	DateTo   *time.Time         `json:"date_to,omitempty"`
	SizeMin  int64              `json:"size_min,omitempty"`
	SizeMax  int64              `json:"size_max,omitempty"`
	SortBy   string             `json:"sort_by,omitempty"` // relevance, date, name, size
	Limit    int                `json:"limit,omitempty"`
	Offset   int                `json:"offset,omitempty"`
}

// EngineSearchStats 搜索统计
type EngineSearchStats struct {
	TotalItems  int            `json:"total_items"`
	ItemsByType map[string]int `json:"items_by_type"`
	LastIndexed *time.Time     `json:"last_indexed,omitempty"`
	IndexSizeMB float64        `json:"index_size_mb"`
}

// Engine 统一搜索引擎
type Engine struct {
	items map[string]*EngineSearchItem
	mu    sync.RWMutex
}

// NewEngine 创建统一搜索引擎
func NewEngine() *Engine {
	return &Engine{
		items: make(map[string]*EngineSearchItem),
	}
}

// IndexItem 索引项目
func (e *Engine) IndexItem(item *EngineSearchItem) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if item.ID == "" {
		item.ID = fmt.Sprintf("idx_%d", time.Now().UnixNano())
	}

	item.UpdatedAt = time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = item.UpdatedAt
	}

	e.items[item.ID] = item
	return nil
}

// RemoveItem 移除索引项
func (e *Engine) RemoveItem(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.items[id]; !ok {
		return fmt.Errorf("item not found: %s", id)
	}

	delete(e.items, id)
	return nil
}

// Search 搜索
func (e *Engine) Search(query EngineSearchQuery) ([]EngineSearchResult, int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if query.Limit <= 0 {
		query.Limit = 20
	}

	keyword := strings.ToLower(query.Keyword)
	results := []EngineSearchResult{}

	for _, item := range e.items {
		// 类型过滤
		if len(query.Types) > 0 {
			found := false
			for _, t := range query.Types {
				if item.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 所有者过滤
		if query.Owner != "" && item.Owner != query.Owner {
			continue
		}

		// 日期过滤
		if query.DateFrom != nil && item.UpdatedAt.Before(*query.DateFrom) {
			continue
		}
		if query.DateTo != nil && item.UpdatedAt.After(*query.DateTo) {
			continue
		}

		// 大小过滤
		if query.SizeMin > 0 && item.Size < query.SizeMin {
			continue
		}
		if query.SizeMax > 0 && item.Size > query.SizeMax {
			continue
		}

		// 标签过滤
		if len(query.Tags) > 0 {
			hasTag := false
			for _, tag := range query.Tags {
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

		// 计算匹配分数
		score := e.calculateScore(item, keyword)
		if score > 0 {
			result := EngineSearchResult{
				Item:  item,
				Score: score,
			}

			// 生成高亮
			if keyword != "" {
				result.Highlights = e.generateHighlights(item, keyword)
			}

			results = append(results, result)
		}
	}

	// 排序
	switch query.SortBy {
	case "date":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Item.UpdatedAt.After(results[j].Item.UpdatedAt)
		})
	case "name":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Item.Name < results[j].Item.Name
		})
	case "size":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Item.Size > results[j].Item.Size
		})
	default: // relevance
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	}

	total := len(results)

	// 分页
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results, total
}

// calculateScore 计算匹配分数
func (e *Engine) calculateScore(item *EngineSearchItem, keyword string) float64 {
	if keyword == "" {
		return 1.0 // 无关键词时返回所有
	}

	score := 0.0
	nameLower := strings.ToLower(item.Name)
	contentLower := strings.ToLower(item.Content)
	pathLower := strings.ToLower(item.Path)

	// 名称完全匹配
	if nameLower == keyword {
		score += 10.0
	} else if strings.Contains(nameLower, keyword) {
		score += 5.0
	}

	// 内容匹配
	if strings.Contains(contentLower, keyword) {
		score += 3.0
	}

	// 路径匹配
	if strings.Contains(pathLower, keyword) {
		score += 2.0
	}

	// 标签匹配
	for _, tag := range item.Tags {
		if strings.Contains(strings.ToLower(tag), keyword) {
			score += 4.0
		}
	}

	return score
}

// generateHighlights 生成高亮
func (e *Engine) generateHighlights(item *EngineSearchItem, keyword string) []string {
	highlights := []string{}

	nameLower := strings.ToLower(item.Name)
	if strings.Contains(nameLower, keyword) {
		highlights = append(highlights, fmt.Sprintf("名称: ...%s...", keyword))
	}

	if strings.Contains(strings.ToLower(item.Content), keyword) {
		highlights = append(highlights, fmt.Sprintf("内容包含: %s", keyword))
	}

	return highlights
}

// GetStats 获取统计信息
func (e *Engine) GetStats() *EngineSearchStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &EngineSearchStats{
		TotalItems:  len(e.items),
		ItemsByType: make(map[string]int),
	}

	for _, item := range e.items {
		stats.ItemsByType[string(item.Type)]++
	}

	return stats
}

// GetItem 获取索引项
func (e *Engine) GetItem(id string) (*EngineSearchItem, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	item, ok := e.items[id]
	if !ok {
		return nil, fmt.Errorf("item not found: %s", id)
	}

	return item, nil
}

// ListItems 列出所有索引项
func (e *Engine) ListItems(limit int) []*EngineSearchItem {
	e.mu.RLock()
	defer e.mu.RUnlock()

	items := make([]*EngineSearchItem, 0, len(e.items))
	for _, item := range e.items {
		items = append(items, item)
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items
}
