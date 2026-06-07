// Package websharepro - Spotlight 搜索索引集成
// 提供 macOS Spotlight 风格的全文搜索能力
// 支持文件元数据索引、内容提取、实时更新
package websharepro

import (
	"context"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// IndexFieldType 索引字段类型
type IndexFieldType int

const (
	FieldText    IndexFieldType = iota // 文本
	FieldKeyword                       // 关键词（不分词）
	FieldNumeric                       // 数值
	FieldDate                          // 日期
	FieldBoolean                       // 布尔
)

// IndexField 索引字段定义
type IndexField struct {
	Name    string         `json:"name"`
	Type    IndexFieldType `json:"type"`
	Stored  bool           `json:"stored"`  // 是否存储原始值
	Indexed bool           `json:"indexed"` // 是否建立索引
	Boost   float64        `json:"boost"`   // 权重提升
}

// IndexDocument 索引文档
type IndexDocument struct {
	ID         string         `json:"id"`
	Path       string         `json:"path"`
	Name       string         `json:"name"`
	Extension  string         `json:"extension"`
	MimeType   string         `json:"mimeType"`
	Size       int64          `json:"size"`
	CreatedAt  time.Time      `json:"createdAt"`
	ModifiedAt time.Time      `json:"modifiedAt"`
	Content    string         `json:"content,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Document   *IndexDocument `json:"document"`
	Score      float64        `json:"score"`
	Highlights []string       `json:"highlights,omitempty"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Query     string            `json:"query"`
	Filters   map[string]string `json:"filters,omitempty"`
	SortBy    string            `json:"sortBy,omitempty"`
	SortDesc  bool              `json:"sortDesc,omitempty"`
	Page      int               `json:"page"`
	PageSize  int               `json:"pageSize"`
	Highlight bool              `json:"highlight"`
}

// IndexStats 索引统计
type IndexStats struct {
	TotalDocuments int64         `json:"totalDocuments"`
	IndexSize      int64         `json:"indexSize"`
	LastUpdated    time.Time     `json:"lastUpdated"`
	IndexLatency   time.Duration `json:"indexLatency"`
	SearchLatency  time.Duration `json:"searchLatency"`
	QueryCount     int64         `json:"queryCount"`
}

// SpotlightEngine Spotlight 搜索引擎
type SpotlightEngine struct {
	mu         sync.RWMutex
	documents  map[string]*IndexDocument
	index      map[string]map[string]float64  // token -> docID -> tf-idf
	fieldIndex map[string]map[string][]string // field -> value -> docIDs
	tagsIndex  map[string][]string            // tag -> docIDs
	stats      *IndexStats
	stopWords  map[string]bool
	maxResults int
	stopCh     chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewSpotlightEngine 创建搜索引擎
func NewSpotlightEngine(maxResults int) *SpotlightEngine {
	if maxResults <= 0 {
		maxResults = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &SpotlightEngine{
		documents:  make(map[string]*IndexDocument),
		index:      make(map[string]map[string]float64),
		fieldIndex: make(map[string]map[string][]string),
		tagsIndex:  make(map[string][]string),
		stats:      &IndexStats{},
		stopWords:  defaultStopWords(),
		maxResults: maxResults,
		stopCh:     make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动后台优化协程
	go engine.backgroundWorker()

	return engine
}

// IndexDocument 添加/更新文档索引
func (e *SpotlightEngine) IndexDocument(doc *IndexDocument) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	start := time.Now()

	doc.ID = e.generateDocID(doc.Path)
	doc.UpdatedAt = time.Now()

	// 移除旧索引
	if old, exists := e.documents[doc.ID]; exists {
		e.removeFromIndex(old)
	}

	// 存储文档
	e.documents[doc.ID] = doc

	// 构建倒排索引
	e.addToIndex(doc)

	// 更新统计
	e.stats.TotalDocuments = int64(len(e.documents))
	e.stats.LastUpdated = time.Now()
	e.stats.IndexLatency = time.Since(start)

	return nil
}

// RemoveDocument 删除文档索引
func (e *SpotlightEngine) RemoveDocument(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	docID := e.generateDocID(path)
	doc, exists := e.documents[docID]
	if !exists {
		return fmt.Errorf("document not found: %s", path)
	}

	e.removeFromIndex(doc)
	delete(e.documents, docID)

	e.stats.TotalDocuments = int64(len(e.documents))
	e.stats.LastUpdated = time.Now()

	return nil
}

// Search 执行搜索
func (e *SpotlightEngine) Search(query *SearchQuery) ([]*SearchResult, int, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	start := time.Now()
	e.stats.QueryCount++

	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > e.maxResults {
		query.PageSize = e.maxResults
	}

	// 解析查询
	tokens := tokenize(query.Query)
	if len(tokens) == 0 {
		return nil, 0, nil
	}

	// 计算文档得分
	scores := make(map[string]float64)
	for _, token := range tokens {
		token = strings.ToLower(token)
		if e.stopWords[token] {
			continue
		}

		if docScores, exists := e.index[token]; exists {
			for docID, tfidf := range docScores {
				scores[docID] += tfidf
			}
		}

		// 前缀匹配
		for indexedToken, docScores := range e.index {
			if strings.HasPrefix(indexedToken, token) && indexedToken != token {
				for docID, tfidf := range docScores {
					scores[docID] += tfidf * 0.5 // 前缀匹配权重较低
				}
			}
		}
	}

	// 应用过滤器
	var results []*SearchResult
	for docID, score := range scores {
		doc, exists := e.documents[docID]
		if !exists {
			continue
		}

		// 字段过滤
		if !e.matchFilters(doc, query.Filters) {
			continue
		}

		result := &SearchResult{
			Document: doc,
			Score:    score,
		}

		// 高亮
		if query.Highlight {
			result.Highlights = e.highlight(doc, tokens)
		}

		results = append(results, result)
	}

	// 排序
	e.sortResults(results, query.SortBy, query.SortDesc)

	total := len(results)

	// 分页
	start_idx := (query.Page - 1) * query.PageSize
	if start_idx >= len(results) {
		return nil, total, nil
	}
	end_idx := start_idx + query.PageSize
	if end_idx > len(results) {
		end_idx = len(results)
	}

	results = results[start_idx:end_idx]

	e.stats.SearchLatency = time.Since(start)

	return results, total, nil
}

// GetDocument 获取文档
func (e *SpotlightEngine) GetDocument(path string) (*IndexDocument, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	docID := e.generateDocID(path)
	doc, ok := e.documents[docID]
	return doc, ok
}

// GetStats 获取索引统计
func (e *SpotlightEngine) GetStats() *IndexStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	stats.TotalDocuments = int64(len(e.documents))

	// 计算索引大小
	var indexSize int64
	for token, docScores := range e.index {
		indexSize += int64(len(token))
		indexSize += int64(len(docScores) * 16) // docID + score
	}
	stats.IndexSize = indexSize

	return &stats
}

// ListDocuments 列出文档
func (e *SpotlightEngine) ListDocuments(prefix string, limit int) []*IndexDocument {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*IndexDocument
	for _, doc := range e.documents {
		if prefix != "" && !strings.HasPrefix(doc.Path, prefix) {
			continue
		}
		result = append(result, doc)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// RebuildIndex 重建索引
func (e *SpotlightEngine) RebuildIndex() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 清空索引
	e.index = make(map[string]map[string]float64)
	e.fieldIndex = make(map[string]map[string][]string)
	e.tagsIndex = make(map[string][]string)

	// 重建
	for _, doc := range e.documents {
		e.addToIndex(doc)
	}

	e.stats.LastUpdated = time.Now()
}

// addToIndex 添加文档到索引
func (e *SpotlightEngine) addToIndex(doc *IndexDocument) {
	// 索引文件名
	nameTokens := tokenize(doc.Name)
	for _, token := range nameTokens {
		token = strings.ToLower(token)
		if e.stopWords[token] {
			continue
		}
		if e.index[token] == nil {
			e.index[token] = make(map[string]float64)
		}
		e.index[token][doc.ID] = 2.0 // 文件名权重较高
	}

	// 索引内容
	contentTokens := tokenize(doc.Content)
	contentTF := make(map[string]int)
	for _, token := range contentTokens {
		token = strings.ToLower(token)
		if e.stopWords[token] {
			continue
		}
		contentTF[token]++
	}

	for token, count := range contentTF {
		tf := float64(count) / float64(len(contentTokens))
		if e.index[token] == nil {
			e.index[token] = make(map[string]float64)
		}
		e.index[token][doc.ID] = tf
	}

	// 索引扩展名
	ext := strings.ToLower(doc.Extension)
	if e.fieldIndex["extension"] == nil {
		e.fieldIndex["extension"] = make(map[string][]string)
	}
	e.fieldIndex["extension"][ext] = append(e.fieldIndex["extension"][ext], doc.ID)

	// 索引 MIME 类型
	if e.fieldIndex["mimeType"] == nil {
		e.fieldIndex["mimeType"] = make(map[string][]string)
	}
	e.fieldIndex["mimeType"][doc.MimeType] = append(e.fieldIndex["mimeType"][doc.MimeType], doc.ID)

	// 索引标签
	for _, tag := range doc.Tags {
		tag = strings.ToLower(tag)
		e.tagsIndex[tag] = append(e.tagsIndex[tag], doc.ID)
	}
}

// removeFromIndex 从索引移除文档
func (e *SpotlightEngine) removeFromIndex(doc *IndexDocument) {
	for token, docScores := range e.index {
		delete(docScores, doc.ID)
		if len(docScores) == 0 {
			delete(e.index, token)
		}
	}

	for _, fieldValues := range e.fieldIndex {
		for value, docIDs := range fieldValues {
			filtered := make([]string, 0, len(docIDs))
			for _, id := range docIDs {
				if id != doc.ID {
					filtered = append(filtered, id)
				}
			}
			fieldValues[value] = filtered
		}
	}

	for tag, docIDs := range e.tagsIndex {
		filtered := make([]string, 0, len(docIDs))
		for _, id := range docIDs {
			if id != doc.ID {
				filtered = append(filtered, id)
			}
		}
		e.tagsIndex[tag] = filtered
	}
}

// matchFilters 匹配过滤器
func (e *SpotlightEngine) matchFilters(doc *IndexDocument, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "extension":
			if !strings.EqualFold(doc.Extension, value) {
				return false
			}
		case "mimeType":
			if !strings.EqualFold(doc.MimeType, value) {
				return false
			}
		case "tag":
			found := false
			for _, tag := range doc.Tags {
				if strings.EqualFold(tag, value) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case "path":
			if !strings.HasPrefix(doc.Path, value) {
				return false
			}
		}
	}
	return true
}

// highlight 生成高亮片段
func (e *SpotlightEngine) highlight(doc *IndexDocument, tokens []string) []string {
	var highlights []string

	content := doc.Content
	if content == "" {
		content = doc.Name
	}

	lowerContent := strings.ToLower(content)
	for _, token := range tokens {
		tokenLower := strings.ToLower(token)
		idx := strings.Index(lowerContent, tokenLower)
		if idx >= 0 {
			start := idx - 30
			if start < 0 {
				start = 0
			}
			end := idx + len(token) + 30
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
			highlights = append(highlights, snippet)
		}
	}

	return highlights
}

// sortResults 排序结果
func (e *SpotlightEngine) sortResults(results []*SearchResult, sortBy string, desc bool) {
	sort.Slice(results, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "name":
			less = results[i].Document.Name < results[j].Document.Name
		case "size":
			less = results[i].Document.Size < results[j].Document.Size
		case "modified":
			less = results[i].Document.ModifiedAt.Before(results[j].Document.ModifiedAt)
		case "created":
			less = results[i].Document.CreatedAt.Before(results[j].Document.CreatedAt)
		default: // score
			less = results[i].Score < results[j].Score
		}
		if desc {
			return !less
		}
		return less
	})
}

// generateDocID 生成文档 ID
func (e *SpotlightEngine) generateDocID(path string) string {
	h := fnv.New64a()
	h.Write([]byte(path))
	return fmt.Sprintf("doc-%x", h.Sum64())
}

// backgroundWorker 后台工作协程
func (e *SpotlightEngine) backgroundWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			// 定期优化索引
			e.mu.Lock()
			for token, docScores := range e.index {
				if len(docScores) == 0 {
					delete(e.index, token)
				}
			}
			e.mu.Unlock()
		}
	}
}

// Close 关闭搜索引擎
func (e *SpotlightEngine) Close() {
	e.cancel()
	close(e.stopCh)
}

// tokenize 分词
func tokenize(text string) []string {
	if text == "" {
		return nil
	}

	// 正则分割
	re := regexp.MustCompile(`[\s\p{Punct}]+`)
	tokens := re.Split(text, -1)

	var result []string
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if len(token) >= 2 { // 最少 2 字符
			result = append(result, token)
		}
	}
	return result
}

// defaultStopWords 默认停用词
func defaultStopWords() map[string]bool {
	words := []string{
		"the", "a", "an", "and", "or", "but", "in", "on", "at", "to", "for",
		"of", "with", "by", "from", "is", "are", "was", "were", "be", "been",
		"being", "have", "has", "had", "do", "does", "did", "will", "would",
		"could", "should", "may", "might", "can", "this", "that", "these",
		"those", "it", "its", "not", "no", "nor", "so", "if", "then",
		"的", "了", "在", "是", "我", "有", "和", "就", "不", "人", "都",
		"一", "一个", "上", "也", "很", "到", "说", "要", "去", "你",
	}
	stop := make(map[string]bool, len(words))
	for _, w := range words {
		stop[w] = true
	}
	return stop
}

// IsIndexed 检查文件是否已索引
func (e *SpotlightEngine) IsIndexed(path string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	docID := e.generateDocID(path)
	_, exists := e.documents[docID]
	return exists
}

// GetIndexedExtensions 获取已索引的扩展名列表
func (e *SpotlightEngine) GetIndexedExtensions() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var exts []string
	for ext := range e.fieldIndex["extension"] {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

// SearchByTag 按标签搜索
func (e *SpotlightEngine) SearchByTag(tag string) []*IndexDocument {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tag = strings.ToLower(tag)
	docIDs, exists := e.tagsIndex[tag]
	if !exists {
		return nil
	}

	var results []*IndexDocument
	for _, docID := range docIDs {
		if doc, ok := e.documents[docID]; ok {
			results = append(results, doc)
		}
	}
	return results
}

// GetDocumentCount 获取文档数量
func (e *SpotlightEngine) GetDocumentCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.documents)
}

// FilePath 扩展名提取
func FilePath(name string) string {
	return filepath.Ext(name)
}
