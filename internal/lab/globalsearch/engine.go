// Package globalsearch 提供全局搜索与智能导航能力
// 对标 TrueNAS 全局搜索，支持跨模块统一搜索、智能建议、历史记录
package globalsearch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Engine 搜索引擎.
type Engine struct {
	mu          sync.RWMutex
	config      *Config
	index       *SearchIndex
	history     *SearchHistory
	suggestions *SuggestionEngine
	providers   map[string]SearchProvider
	logger      Logger
}

// Config 配置.
type Config struct {
	MaxResults          int
	EnableHistory       bool
	EnableSuggestions   bool
	IndexUpdateInterval time.Duration
}

// SearchIndex 搜索索引.
type SearchIndex struct {
	mu       sync.RWMutex
	entries  map[string]*IndexEntry
	inverted map[string][]string // 词条 -> 条目ID列表
}

// IndexEntry 索引条目.
type IndexEntry struct {
	ID        string
	Type      string // file, service, setting, user, etc.
	Title     string
	Content   string
	Path      string
	Module    string
	Tags      []string
	Score     float64
	UpdatedAt time.Time
}

// SearchHistory 搜索历史.
type SearchHistory struct {
	mu      sync.RWMutex
	queries []HistoryEntry
	maxSize int
}

// HistoryEntry 历史条目.
type HistoryEntry struct {
	Query       string
	Timestamp   time.Time
	ResultCount int
}

// SuggestionEngine 建议引擎.
type SuggestionEngine struct {
	mu          sync.RWMutex
	popular     []string
	recent      []string
	completions map[string][]string
}

// SearchResult 搜索结果.
type SearchResult struct {
	Items       []*SearchItem
	Total       int
	Query       string
	Duration    time.Duration
	Suggestions []string
}

// SearchItem 搜索项.
type SearchItem struct {
	ID          string
	Type        string
	Title       string
	Description string
	Path        string
	Module      string
	Score       float64
	Highlights  []string
	Icon        string
}

// SearchProvider 搜索提供者接口.
type SearchProvider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]*SearchItem, error)
	GetSuggestions(query string) []string
}

// Logger 日志接口.
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewEngine 创建新的搜索引擎.
func NewEngine(config *Config, logger Logger) *Engine {
	return &Engine{
		config: config,
		index: &SearchIndex{
			entries:  make(map[string]*IndexEntry),
			inverted: make(map[string][]string),
		},
		history: &SearchHistory{
			queries: make([]HistoryEntry, 0),
			maxSize: 100,
		},
		suggestions: &SuggestionEngine{
			popular:     make([]string, 0),
			recent:      make([]string, 0),
			completions: make(map[string][]string),
		},
		providers: make(map[string]SearchProvider),
		logger:    logger,
	}
}

// Init 初始化搜索引擎.
func (e *Engine) Init(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 启动索引更新
	go e.indexUpdater(ctx)

	e.logger.Info("全局搜索引擎已启动")
	return nil
}

// RegisterProvider 注册搜索提供者.
func (e *Engine) RegisterProvider(provider SearchProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.providers[provider.Name()] = provider
	e.logger.Info("搜索提供者已注册: %s", provider.Name())
}

// Search 执行搜索.
func (e *Engine) Search(ctx context.Context, query string) (*SearchResult, error) {
	start := time.Now()

	// 记录历史
	if e.config.EnableHistory {
		e.addHistory(query)
	}

	// 从各提供者搜索
	var allItems []*SearchItem
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, provider := range e.providers {
		wg.Add(1)
		go func(p SearchProvider) {
			defer wg.Done()
			items, err := p.Search(ctx, query, e.config.MaxResults)
			if err != nil {
				e.logger.Error("搜索提供者 %s 失败: %v", p.Name(), err)
				return
			}
			mu.Lock()
			allItems = append(allItems, items...)
			mu.Unlock()
		}(provider)
	}

	wg.Wait()

	// 从本地索引搜索
	localItems := e.searchIndex(query)
	allItems = append(allItems, localItems...)

	// 排序和去重
	allItems = e.rankAndDeduplicate(allItems)

	// 限制数量
	if len(allItems) > e.config.MaxResults {
		allItems = allItems[:e.config.MaxResults]
	}

	// 获取建议
	var suggestions []string
	if e.config.EnableSuggestions {
		suggestions = e.GetSuggestions(query)
	}

	return &SearchResult{
		Items:       allItems,
		Total:       len(allItems),
		Query:       query,
		Duration:    time.Since(start),
		Suggestions: suggestions,
	}, nil
}

// searchIndex 搜索本地索引.
func (e *Engine) searchIndex(query string) []*SearchItem {
	e.index.mu.RLock()
	defer e.index.mu.RUnlock()

	var items []*SearchItem
	queryLower := strings.ToLower(query)

	for _, entry := range e.index.entries {
		score := e.calculateScore(entry, queryLower)
		if score > 0 {
			items = append(items, &SearchItem{
				ID:          entry.ID,
				Type:        entry.Type,
				Title:       entry.Title,
				Description: entry.Content,
				Path:        entry.Path,
				Module:      entry.Module,
				Score:       score,
			})
		}
	}

	return items
}

// calculateScore 计算相关性得分.
func (e *Engine) calculateScore(entry *IndexEntry, query string) float64 {
	score := 0.0

	// 标题匹配
	if strings.Contains(strings.ToLower(entry.Title), query) {
		score += 10.0
	}

	// 内容匹配
	if strings.Contains(strings.ToLower(entry.Content), query) {
		score += 5.0
	}

	// 标签匹配
	for _, tag := range entry.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			score += 3.0
		}
	}

	// 模块匹配
	if strings.Contains(strings.ToLower(entry.Module), query) {
		score += 2.0
	}

	return score
}

// rankAndDeduplicate 排序和去重.
func (e *Engine) rankAndDeduplicate(items []*SearchItem) []*SearchItem {
	// 去重
	seen := make(map[string]bool)
	var unique []*SearchItem

	for _, item := range items {
		key := fmt.Sprintf("%s:%s", item.Type, item.ID)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, item)
		}
	}

	// 按得分排序
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].Score > unique[j].Score
	})

	return unique
}

// IndexEntry 添加索引条目.
func (e *Engine) IndexEntry(entry *IndexEntry) {
	e.index.mu.Lock()
	defer e.index.mu.Unlock()

	e.index.entries[entry.ID] = entry

	// 建立倒排索引
	words := tokenize(entry.Title + " " + entry.Content + " " + strings.Join(entry.Tags, " "))
	for _, word := range words {
		e.index.inverted[word] = append(e.index.inverted[word], entry.ID)
	}
}

// GetSuggestions 获取搜索建议.
func (e *Engine) GetSuggestions(query string) []string {
	e.suggestions.mu.RLock()
	defer e.suggestions.mu.RUnlock()

	var suggestions []string

	// 从完成建议中获取
	queryLower := strings.ToLower(query)
	for prefix, completions := range e.suggestions.completions {
		if strings.HasPrefix(queryLower, prefix) {
			suggestions = append(suggestions, completions...)
		}
	}

	// 从热门搜索中获取
	for _, popular := range e.suggestions.popular {
		if strings.Contains(strings.ToLower(popular), queryLower) {
			suggestions = append(suggestions, popular)
		}
	}

	// 去重并限制数量
	suggestions = uniqueStrings(suggestions)
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

// GetHistory 获取搜索历史.
func (e *Engine) GetHistory(limit int) []HistoryEntry {
	e.history.mu.RLock()
	defer e.history.mu.RUnlock()

	if limit <= 0 || limit > len(e.history.queries) {
		limit = len(e.history.queries)
	}

	// 返回最近的历史
	start := len(e.history.queries) - limit
	return e.history.queries[start:]
}

// addHistory 添加搜索历史.
func (e *Engine) addHistory(query string) {
	e.history.mu.Lock()
	defer e.history.mu.Unlock()

	entry := HistoryEntry{
		Query:     query,
		Timestamp: time.Now(),
	}

	e.history.queries = append(e.history.queries, entry)

	// 限制历史大小
	if len(e.history.queries) > e.history.maxSize {
		e.history.queries = e.history.queries[1:]
	}

	// 更新热门搜索
	e.updatePopular(query)
}

// updatePopular 更新热门搜索.
func (e *Engine) updatePopular(query string) {
	e.suggestions.mu.Lock()
	defer e.suggestions.mu.Unlock()

	// 检查是否已存在
	for _, p := range e.suggestions.popular {
		if p == query {
			return
		}
	}

	e.suggestions.popular = append(e.suggestions.popular, query)

	// 限制数量
	if len(e.suggestions.popular) > 10 {
		e.suggestions.popular = e.suggestions.popular[1:]
	}
}

// indexUpdater 索引更新器.
func (e *Engine) indexUpdater(ctx context.Context) {
	ticker := time.NewTicker(e.config.IndexUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.updateIndex()
		}
	}
}

// updateIndex 更新索引.
func (e *Engine) updateIndex() {
	// 从各模块更新索引
	e.logger.Debug("更新搜索索引")
}

// tokenize 分词.
func tokenize(text string) []string {
	// 简单的分词实现
	words := strings.Fields(strings.ToLower(text))
	return words
}

// uniqueStrings 字符串去重.
func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	return result
}
