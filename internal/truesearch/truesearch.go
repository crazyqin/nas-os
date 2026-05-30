// Package truesearch 提供全文搜索引擎
// 学习 TrueNAS 26 Spotlight Search (TrueSearch) 特性：
// - 桌面级搜索体验
// - 亚秒级全文搜索
// - 文件内容索引
// - 元数据搜索
// - 搜索建议和自动补全
package truesearch

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// SearchResult 搜索结果
type SearchResult struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"filePath"`
	FileName    string    `json:"fileName"`
	FileType    string    `json:"fileType"`
	Size        int64     `json:"size"`
	MatchType   string    `json:"matchType"` // filename, content, metadata
	MatchText   string    `json:"matchText"` // 匹配的文本片段
	Score       float64   `json:"score"`     // 相关性分数
	Highlighted string    `json:"highlighted"` // 高亮显示
	ModifiedAt  time.Time `json:"modifiedAt"`
}

// SearchStats 搜索统计
type SearchStats struct {
	TotalDocuments int64   `json:"totalDocuments"`
	IndexSize      int64   `json:"indexSize"`
	SearchCount    int64   `json:"searchCount"`
	AvgLatencyMs   float64 `json:"avgLatencyMs"`
	LastIndexedAt  time.Time `json:"lastIndexedAt"`
}

// IndexEntry 索引条目
type IndexEntry struct {
	ID         string    `json:"id"`
	FilePath   string    `json:"filePath"`
	FileName   string    `json:"fileName"`
	FileType   string    `json:"fileType"`
	Size       int64     `json:"size"`
	Content    string    `json:"content"`    // 文件内容（用于全文搜索）
	Tags       []string  `json:"tags"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

// SearchEngine 搜索引擎
type SearchEngine struct {
	mu       sync.RWMutex
	index    map[string]*IndexEntry
	trie     *TrieIndex
	stats    *SearchStats
	config   *SearchConfig
}

// SearchConfig 搜索配置
type SearchConfig struct {
	MaxResults       int  `json:"maxResults"`       // 最大结果数
	MinScore         float64 `json:"minScore"`      // 最小相关性分数
	EnableContent    bool `json:"enableContent"`    // 启用内容索引
	MaxContentSize   int  `json:"maxContentSize"`   // 最大内容索引大小（字节）
	IndexExtensions  []string `json:"indexExtensions"` // 需要索引的文件扩展名
}

// NewSearchEngine 创建搜索引擎
func NewSearchEngine(config *SearchConfig) *SearchEngine {
	if config == nil {
		config = &SearchConfig{
			MaxResults:      50,
			MinScore:        0.1,
			EnableContent:   true,
			MaxContentSize:  1024 * 1024, // 1MB
			IndexExtensions: []string{".txt", ".md", ".go", ".py", ".js", ".html", ".json", ".yaml", ".yml", ".toml"},
		}
	}
	return &SearchEngine{
		index:  make(map[string]*IndexEntry),
		trie:   NewTrieIndex(),
		stats:  &SearchStats{},
		config: config,
	}
}

// IndexDocument 索引文档
func (se *SearchEngine) IndexDocument(entry *IndexEntry) {
	se.mu.Lock()
	defer se.mu.Unlock()

	se.index[entry.ID] = entry

	// 索引文件名
	se.trie.Insert(strings.ToLower(entry.FileName), entry.ID)

	// 索引内容
	if se.config.EnableContent && len(entry.Content) <= se.config.MaxContentSize {
		words := tokenize(entry.Content)
		for _, word := range words {
			se.trie.Insert(strings.ToLower(word), entry.ID)
		}
	}

	se.stats.TotalDocuments = int64(len(se.index))
	se.stats.LastIndexedAt = time.Now()
}

// RemoveDocument 移除文档
func (se *SearchEngine) RemoveDocument(id string) {
	se.mu.Lock()
	defer se.mu.Unlock()

	entry, ok := se.index[id]
	if !ok {
		return
	}

	se.trie.Remove(entry.FileName, id)
	delete(se.index, id)
	se.stats.TotalDocuments = int64(len(se.index))
}

// Search 搜索
func (se *SearchEngine) Search(query string, limit int) []*SearchResult {
	se.mu.RLock()
	defer se.mu.RUnlock()

	start := time.Now()

	if limit <= 0 {
		limit = se.config.MaxResults
	}

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}

	// 前缀搜索
	matchedIDs := se.trie.Search(query)

	// 计算分数并排序
	var results []*SearchResult
	scored := make(map[string]float64)

	for _, id := range matchedIDs {
		entry, ok := se.index[id]
		if !ok {
			continue
		}

		score := calculateScore(query, entry)
		if score < se.config.MinScore {
			continue
		}

		scored[id] = score
		result := &SearchResult{
			ID:         entry.ID,
			FilePath:   entry.FilePath,
			FileName:   entry.FileName,
			FileType:   entry.FileType,
			Size:       entry.Size,
			Score:      score,
			ModifiedAt: entry.ModifiedAt,
		}

		// 确定匹配类型
		if strings.Contains(strings.ToLower(entry.FileName), query) {
			result.MatchType = "filename"
			result.MatchText = entry.FileName
		} else {
			result.MatchType = "content"
			result.MatchText = extractSnippet(entry.Content, query)
		}

		result.Highlighted = highlightMatch(result.MatchText, query)
		results = append(results, result)
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	// 更新统计
	se.stats.SearchCount++
	latency := time.Since(start).Seconds() * 1000
	se.stats.AvgLatencyMs = (se.stats.AvgLatencyMs*float64(se.stats.SearchCount-1) + latency) / float64(se.stats.SearchCount)

	return results
}

// GetSuggestions 获取搜索建议
func (se *SearchEngine) GetSuggestions(prefix string, limit int) []string {
	se.mu.RLock()
	defer se.mu.RUnlock()

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil
	}

	return se.trie.AutoComplete(prefix, limit)
}

// GetStats 获取统计
func (se *SearchEngine) GetStats() *SearchStats {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.stats
}

// RebuildIndex 重建索引
func (se *SearchEngine) RebuildIndex() {
	se.mu.Lock()
	defer se.mu.Unlock()

	se.trie = NewTrieIndex()
	for _, entry := range se.index {
		se.trie.Insert(strings.ToLower(entry.FileName), entry.ID)
		if se.config.EnableContent && len(entry.Content) <= se.config.MaxContentSize {
			words := tokenize(entry.Content)
			for _, word := range words {
				se.trie.Insert(strings.ToLower(word), entry.ID)
			}
		}
	}
	se.stats.LastIndexedAt = time.Now()
}

// 工具函数
func tokenize(text string) []string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ',' || r == '.' || r == ';' || r == ':'
	})
	return words
}

func calculateScore(query string, entry *IndexEntry) float64 {
	score := 0.0
	lowerName := strings.ToLower(entry.FileName)
	lowerContent := strings.ToLower(entry.Content)

	// 文件名完全匹配
	if lowerName == query {
		score += 10.0
	} else if strings.HasPrefix(lowerName, query) {
		score += 5.0
	} else if strings.Contains(lowerName, query) {
		score += 3.0
	}

	// 内容匹配
	if strings.Contains(lowerContent, query) {
		score += 1.0
	}

	return score
}

func extractSnippet(content, query string) string {
	lower := strings.ToLower(content)
	idx := strings.Index(lower, query)
	if idx == -1 {
		return ""
	}

	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 50
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

func highlightMatch(text, query string) string {
	return strings.ReplaceAll(strings.ToLower(text), query, "**"+query+"**")
}

// TrieIndex Trie索引
type TrieIndex struct {
	root *TrieNode
}

type TrieNode struct {
	children map[rune]*TrieNode
	ids      []string
	isEnd    bool
}

func NewTrieIndex() *TrieIndex {
	return &TrieIndex{
		root: &TrieNode{children: make(map[rune]*TrieNode)},
	}
}

func (t *TrieIndex) Insert(word, id string) {
	node := t.root
	for _, ch := range word {
		if _, ok := node.children[ch]; !ok {
			node.children[ch] = &TrieNode{children: make(map[rune]*TrieNode)}
		}
		node = node.children[ch]
	}
	node.isEnd = true
	if !contains(node.ids, id) {
		node.ids = append(node.ids, id)
	}
}

func (t *TrieIndex) Remove(word, id string) {
	node := t.root
	for _, ch := range word {
		if _, ok := node.children[ch]; !ok {
			return
		}
		node = node.children[ch]
	}
	node.ids = removeID(node.ids, id)
}

func (t *TrieIndex) Search(prefix string) []string {
	node := t.root
	for _, ch := range prefix {
		if _, ok := node.children[ch]; !ok {
			return nil
		}
		node = node.children[ch]
	}
	return collectIDs(node)
}

func (t *TrieIndex) AutoComplete(prefix string, limit int) []string {
	ids := t.Search(prefix)
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids
}

func collectIDs(node *TrieNode) []string {
	var ids []string
	if node.isEnd {
		ids = append(ids, node.ids...)
	}
	for _, child := range node.children {
		ids = append(ids, collectIDs(child)...)
	}
	return ids
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeID(slice []string, id string) []string {
	var result []string
	for _, s := range slice {
		if s != id {
			result = append(result, s)
		}
	}
	return result
}
