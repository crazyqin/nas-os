// Package aisearch 提供 AI 语义搜索引擎核心功能
package aisearch

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine 搜索引擎实现
type Engine struct {
	config    *SearchConfig
	indexes   map[string]*SearchIndex
	cache     *SearchCache
	encoder   VectorEncoder
	extractor ContentExtractor
	crawler   FileCrawler
	stats     *SearchStats
	mu        sync.RWMutex
	stopCh    chan struct{}
}

// NewEngine 创建搜索引擎
func NewEngine(config *SearchConfig, encoder VectorEncoder, extractor ContentExtractor) *Engine {
	if config == nil {
		config = DefaultSearchConfig()
	}

	engine := &Engine{
		config:    config,
		indexes:   make(map[string]*SearchIndex),
		cache:     NewSearchCache(config.CacheSize, config.CacheTTL),
		encoder:   encoder,
		extractor: extractor,
		stats: &SearchStats{
			HotWords: make([]HotWord, 0),
		},
		stopCh: make(chan struct{}),
	}

	return engine
}

// Search 执行搜索
func (e *Engine) Search(query *SearchQuery) (*SearchResponse, error) {
	start := time.Now()

	if err := query.Validate(); err != nil {
		return nil, err
	}

	// 检查缓存
	cacheKey := e.buildCacheKey(query)
	if cached := e.cache.Get(cacheKey); cached != nil {
		resp := cached.(*SearchResponse)
		resp.QueryTime = time.Since(start)
		return resp, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []SearchResult

	// 根据搜索模式执行
	switch query.Mode {
	case SearchModeFullText:
		results = e.fullTextSearch(query)
	case SearchModeSemantic:
		results = e.semanticSearch(query)
	case SearchModeHybrid:
		fullTextResults := e.fullTextSearch(query)
		semanticResults := e.semanticSearch(query)
		results = e.mergeResults(fullTextResults, semanticResults)
	default:
		results = e.fullTextSearch(query)
	}

	// 应用过滤器
	results = e.applyFilters(results, query)

	// 排序
	results = e.sortResults(results, query.Sort)

	// 分页
	total := len(results)
	startIdx := (query.Page - 1) * query.PageSize
	endIdx := startIdx + query.PageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}
	pageResults := results[startIdx:endIdx]

	// 统计热词
	e.updateHotWords(query.Keyword)

	// 生成搜索建议
	suggestions := e.generateSuggestions(query.Keyword)

	// 计算分面统计
	facets := e.calculateFacets(results)

	response := &SearchResponse{
		Query:       query.Keyword,
		Total:       total,
		Page:        query.Page,
		PageSize:    query.PageSize,
		Results:     pageResults,
		Suggestions: suggestions,
		Facets:      facets,
		QueryTime:   time.Since(start),
	}

	// 缓存结果
	e.cache.Set(cacheKey, response)

	// 更新统计
	e.stats.SearchCount++
	avgTime := e.stats.AvgQueryTime
	count := e.stats.SearchCount
	e.stats.AvgQueryTime = (avgTime*float64(count-1) + float64(response.QueryTime.Milliseconds())) / float64(count)

	return response, nil
}

// fullTextSearch 全文检索
func (e *Engine) fullTextSearch(query *SearchQuery) []SearchResult {
	var results []SearchResult
	keyword := strings.ToLower(query.Keyword)

	for _, doc := range e.indexes {
		if doc.Status != IndexStatusIndexed {
			continue
		}

		score := 0.0
		var highlights []Highlight

		// 文件名匹配
		if strings.Contains(strings.ToLower(doc.FileName), keyword) {
			score += 10.0
			highlights = append(highlights, Highlight{
				Field:   "fileName",
				Content: e.highlightText(doc.FileName, query.Keyword),
			})
		}

		// 内容匹配
		if strings.Contains(strings.ToLower(doc.Content), keyword) {
			score += 5.0
			snippet := e.extractSnippet(doc.Content, query.Keyword, 200)
			highlights = append(highlights, Highlight{
				Field:   "content",
				Content: snippet,
			})
		}

		// 标签匹配
		for _, tag := range doc.Tags {
			if strings.Contains(strings.ToLower(tag), keyword) {
				score += 3.0
				highlights = append(highlights, Highlight{
					Field:   "tags",
					Content: tag,
				})
			}
		}

		// 元数据匹配
		for k, v := range doc.Metadata {
			if strings.Contains(strings.ToLower(v), keyword) {
				score += 2.0
				highlights = append(highlights, Highlight{
					Field:   k,
					Content: v,
				})
			}
		}

		if score > 0 {
			snippet := ""
			if len(highlights) > 0 {
				snippet = highlights[0].Content
			}
			results = append(results, SearchResult{
				ID:         doc.ID,
				FilePath:   doc.FilePath,
				FileName:   doc.FileName,
				FileType:   doc.FileType,
				FileSize:   doc.FileSize,
				ModifiedAt: doc.ModifiedAt,
				CreatedAt:  doc.CreatedAt,
				Score:      score,
				Highlights: highlights,
				Snippet:    snippet,
				Tags:       doc.Tags,
				Metadata:   doc.Metadata,
				TextScore:  score,
			})
		}
	}

	return results
}

// semanticSearch 语义搜索
func (e *Engine) semanticSearch(query *SearchQuery) []SearchResult {
	if e.encoder == nil {
		return nil
	}

	// 编码查询文本
	queryVector, err := e.encoder.Encode(query.Keyword)
	if err != nil {
		log.Printf("语义编码失败: %v", err)
		return nil
	}

	var results []SearchResult

	for _, doc := range e.indexes {
		if doc.Status != IndexStatusIndexed || len(doc.Vector) == 0 {
			continue
		}

		// 计算余弦相似度
		similarity := cosineSimilarity(queryVector, doc.Vector)
		if similarity < e.config.SimilarityThresh {
			continue
		}

		snippet := e.extractSnippet(doc.Content, "", 200)
		results = append(results, SearchResult{
			ID:          doc.ID,
			FilePath:    doc.FilePath,
			FileName:    doc.FileName,
			FileType:    doc.FileType,
			FileSize:    doc.FileSize,
			ModifiedAt:  doc.ModifiedAt,
			CreatedAt:   doc.CreatedAt,
			Score:       similarity * 10,
			Snippet:     snippet,
			Tags:        doc.Tags,
			Metadata:    doc.Metadata,
			VectorScore: similarity,
		})
	}

	return results
}

// mergeResults 合并全文和语义搜索结果
func (e *Engine) mergeResults(fullText, semantic []SearchResult) []SearchResult {
	merged := make(map[string]*SearchResult)

	// 添加全文检索结果
	for i, r := range fullText {
		merged[r.ID] = &fullText[i]
	}

	// 合并语义搜索结果
	for i, r := range semantic {
		if existing, ok := merged[r.ID]; ok {
			// 合并得分：全文 60% + 语义 40%
			existing.Score = existing.TextScore*0.6 + r.VectorScore*10*0.4
			existing.VectorScore = r.VectorScore
		} else {
			merged[r.ID] = &semantic[i]
		}
	}

	results := make([]SearchResult, 0, len(merged))
	for _, r := range merged {
		results = append(results, *r)
	}

	return results
}

// IndexDocument 索引单个文档
func (e *Engine) IndexDocument(doc *SearchIndex) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 提取内容
	if doc.Content == "" && e.extractor != nil && e.extractor.CanExtract(doc.FileType, "") {
		content, err := e.extractor.Extract(doc.FilePath)
		if err != nil {
			doc.Status = IndexStatusFailed
			doc.Error = err.Error()
		} else {
			doc.Content = content
		}
	}

	// 生成语义向量
	if e.config.EnableSemantic && e.encoder != nil && doc.Content != "" {
		vector, err := e.encoder.Encode(doc.Content)
		if err != nil {
			log.Printf("生成向量失败: %v", err)
		} else {
			doc.Vector = vector
		}
	}

	now := time.Now()
	doc.IndexedAt = &now
	doc.Status = IndexStatusIndexed

	e.indexes[doc.ID] = doc

	// 更新统计
	e.stats.IndexedDocuments++
	e.stats.TotalDocuments++
	e.stats.TotalSize += doc.FileSize
	e.stats.LastIndexTime = now

	return nil
}

// IndexBatch 批量索引
func (e *Engine) IndexBatch(docs []*SearchIndex) error {
	for _, doc := range docs {
		if err := e.IndexDocument(doc); err != nil {
			return fmt.Errorf("索引文档 %s 失败: %w", doc.FilePath, err)
		}
	}
	return nil
}

// DeleteDocument 删除文档索引
func (e *Engine) DeleteDocument(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.indexes[id]
	if !ok {
		return fmt.Errorf("文档 %s 不存在", id)
	}

	e.stats.TotalSize -= doc.FileSize
	e.stats.TotalDocuments--
	if doc.Status == IndexStatusIndexed {
		e.stats.IndexedDocuments--
	}

	delete(e.indexes, id)
	return nil
}

// UpdateDocument 更新文档索引
func (e *Engine) UpdateDocument(doc *SearchIndex) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.indexes[doc.ID]
	if !ok {
		return fmt.Errorf("文档 %s 不存在", doc.ID)
	}

	// 更新大小差异
	sizeDiff := doc.FileSize - existing.FileSize
	e.stats.TotalSize += sizeDiff

	// 重新提取内容
	if e.extractor != nil && e.extractor.CanExtract(doc.FileType, "") {
		content, err := e.extractor.Extract(doc.FilePath)
		if err == nil {
			doc.Content = content
		}
	}

	// 重新生成向量
	if e.config.EnableSemantic && e.encoder != nil && doc.Content != "" {
		vector, err := e.encoder.Encode(doc.Content)
		if err == nil {
			doc.Vector = vector
		}
	}

	now := time.Now()
	doc.IndexedAt = &now
	doc.Status = IndexStatusIndexed

	e.indexes[doc.ID] = doc
	e.stats.LastIndexTime = now

	return nil
}

// GetStats 获取搜索统计
func (e *Engine) GetStats() (*SearchStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	return &stats, nil
}

// Suggest 搜索建议
func (e *Engine) Suggest(prefix string, limit int) ([]Suggestion, error) {
	if limit <= 0 {
		limit = 10
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	suggestions := make([]Suggestion, 0)
	prefixLower := strings.ToLower(prefix)
	seen := make(map[string]bool)

	// 从文件名中提取建议
	for _, doc := range e.indexes {
		if doc.Status != IndexStatusIndexed {
			continue
		}
		name := strings.ToLower(doc.FileName)
		if strings.Contains(name, prefixLower) && !seen[doc.FileName] {
			suggestions = append(suggestions, Suggestion{
				Text:  doc.FileName,
				Score: 1.0,
			})
			seen[doc.FileName] = true
		}
	}

	// 从标签中提取建议
	for _, doc := range e.indexes {
		for _, tag := range doc.Tags {
			tagLower := strings.ToLower(tag)
			if strings.Contains(tagLower, prefixLower) && !seen[tag] {
				suggestions = append(suggestions, Suggestion{
					Text:  tag,
					Score: 0.8,
				})
				seen[tag] = true
			}
		}
	}

	// 从热词中提取建议
	for _, hw := range e.stats.HotWords {
		wordLower := strings.ToLower(hw.Word)
		if strings.Contains(wordLower, prefixLower) && !seen[hw.Word] {
			suggestions = append(suggestions, Suggestion{
				Text:  hw.Word,
				Score: 0.6,
				Count: hw.Count,
			})
			seen[hw.Word] = true
		}
	}

	// 按得分排序
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// Close 关闭引擎
func (e *Engine) Close() error {
	close(e.stopCh)
	e.cache.Clear()
	return nil
}

// buildCacheKey 构建缓存键
func (e *Engine) buildCacheKey(query *SearchQuery) string {
	return fmt.Sprintf("%s:%s:%d:%d", query.Keyword, query.Mode, query.Page, query.PageSize)
}

// highlightText 高亮文本
func (e *Engine) highlightText(text, keyword string) string {
	lower := strings.ToLower(text)
	keywordLower := strings.ToLower(keyword)
	
	idx := strings.Index(lower, keywordLower)
	if idx < 0 {
		return text
	}

	start := idx - 30
	if start < 0 {
		start = 0
	}
	end := idx + len(keyword) + 30
	if end > len(text) {
		end = len(text)
	}

	snippet := text[start:end]
	snippet = strings.ReplaceAll(snippet, keyword, fmt.Sprintf("<em>%s</em>", keyword))
	return snippet
}

// extractSnippet 提取摘要
func (e *Engine) extractSnippet(content, keyword string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	if keyword != "" {
		lower := strings.ToLower(content)
		keywordLower := strings.ToLower(keyword)
		idx := strings.Index(lower, keywordLower)
		if idx >= 0 {
			start := idx - maxLen/2
			if start < 0 {
				start = 0
			}
			end := start + maxLen
			if end > len(content) {
				end = len(content)
			}
			return content[start:end]
		}
	}

	return content[:maxLen]
}

// applyFilters 应用过滤器
func (e *Engine) applyFilters(results []SearchResult, query *SearchQuery) []SearchResult {
	var filtered []SearchResult

	for _, r := range results {
		// 文件类型过滤
		if len(query.FileTypes) > 0 {
			matched := false
			for _, ft := range query.FileTypes {
				if r.FileType == ft {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 日期过滤
		if query.DateFrom != nil && r.ModifiedAt.Before(*query.DateFrom) {
			continue
		}
		if query.DateTo != nil && r.ModifiedAt.After(*query.DateTo) {
			continue
		}

		// 大小过滤
		if query.SizeMin != nil && r.FileSize < *query.SizeMin {
			continue
		}
		if query.SizeMax != nil && r.FileSize > *query.SizeMax {
			continue
		}

		// 路径过滤
		if len(query.Paths) > 0 {
			matched := false
			for _, p := range query.Paths {
				if strings.HasPrefix(r.FilePath, p) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 标签过滤
		if len(query.Tags) > 0 {
			matched := false
			for _, qt := range query.Tags {
				for _, rt := range r.Tags {
					if strings.EqualFold(qt, rt) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		filtered = append(filtered, r)
	}

	return filtered
}

// sortResults 排序结果
func (e *Engine) sortResults(results []SearchResult, order SortOrder) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		switch order {
		case SortOrderTimeDesc:
			return results[i].ModifiedAt.After(results[j].ModifiedAt)
		case SortOrderTimeAsc:
			return results[i].ModifiedAt.Before(results[j].ModifiedAt)
		case SortOrderSizeDesc:
			return results[i].FileSize > results[j].FileSize
		case SortOrderSizeAsc:
			return results[i].FileSize < results[j].FileSize
		case SortOrderFrequency:
			return results[i].Score > results[j].Score
		default: // SortOrderRelevance
			return results[i].Score > results[j].Score
		}
	})
	return results
}

// updateHotWords 更新热词
func (e *Engine) updateHotWords(keyword string) {
	words := strings.Fields(keyword)
	for _, word := range words {
		found := false
		for i, hw := range e.stats.HotWords {
			if hw.Word == word {
				e.stats.HotWords[i].Count++
				found = true
				break
			}
		}
		if !found {
			e.stats.HotWords = append(e.stats.HotWords, HotWord{
				Word:  word,
				Count: 1,
			})
		}
	}

	// 按热度排序，保留 top 100
	sort.Slice(e.stats.HotWords, func(i, j int) bool {
		return e.stats.HotWords[i].Count > e.stats.HotWords[j].Count
	})
	if len(e.stats.HotWords) > 100 {
		e.stats.HotWords = e.stats.HotWords[:100]
	}
}

// generateSuggestions 生成搜索建议
func (e *Engine) generateSuggestions(keyword string) []string {
	suggestions := make([]string, 0)
	seen := make(map[string]bool)

	// 从热词中找相似的
	for _, hw := range e.stats.HotWords {
		if strings.Contains(hw.Word, keyword) || strings.Contains(keyword, hw.Word) {
			if !seen[hw.Word] && hw.Word != keyword {
				suggestions = append(suggestions, hw.Word)
				seen[hw.Word] = true
			}
		}
	}

	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

// calculateFacets 计算分面统计
func (e *Engine) calculateFacets(results []SearchResult) *SearchFacets {
	facets := &SearchFacets{
		FileTypes: make(map[FileType]int),
		Tags:      make(map[string]int),
		Paths:     make(map[string]int),
	}

	for _, r := range results {
		facets.FileTypes[r.FileType]++
		for _, tag := range r.Tags {
			facets.Tags[tag]++
		}
		// 提取父路径
		parts := strings.Split(r.FilePath, "/")
		if len(parts) > 1 {
			parentPath := strings.Join(parts[:len(parts)-1], "/")
			facets.Paths[parentPath]++
		}
	}

	return facets
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt 平方根
func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 50; i++ {
		z = (z + x/z) / 2
	}
	return z
}
