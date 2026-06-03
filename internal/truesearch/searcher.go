// Package truesearch 搜索引擎实现
package truesearch

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Searcher 搜索引擎
type Searcher struct {
	logger *zap.Logger
	config *TrueSearchConfig
	idx    *Indexer
}

// NewSearcher 创建搜索引擎
func NewSearcher(logger *zap.Logger, config *TrueSearchConfig, indexer *Indexer) *Searcher {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultTrueSearchConfig()
	}

	return &Searcher{
		logger: logger,
		config: config,
		idx:    indexer,
	}
}

// Search 执行搜索
func (s *Searcher) Search(query *SearchQuery) (*SearchResponse, error) {
	if query == nil || query.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	start := time.Now()

	// 设置默认值
	if query.Mode == "" {
		query.Mode = SearchModeAll
	}
	if query.Sort == "" {
		query.Sort = SortByRelevance
	}
	if query.Limit <= 0 {
		query.Limit = s.config.MaxResults
	}
	if query.Limit > s.config.MaxResults {
		query.Limit = s.config.MaxResults
	}

	// 分词
	tokens := s.idx.tokenize(query.Query)
	if len(tokens) == 0 {
		return &SearchResponse{
			Query:     query.Query,
			TotalHits: 0,
			Results:   []SearchResult{},
			TookMs:    time.Since(start).Milliseconds(),
		}, nil
	}

	// 执行搜索
	var results []SearchResult

	switch query.Mode {
	case SearchModeFilename:
		results = s.searchFilename(tokens, query)
	case SearchModeContent:
		results = s.searchContent(tokens, query)
	default: // SearchModeAll
		filenameResults := s.searchFilename(tokens, query)
		contentResults := s.searchContent(tokens, query)
		results = s.mergeResults(filenameResults, contentResults)
	}

	// 应用过滤
	results = s.applyFilters(results, query)

	// 排序
	sortResults(results, query.Sort)

	// 分页
	totalHits := len(results)
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if len(results) > query.Limit {
		results = results[:query.Limit]
	}

	// 高亮处理
	for i := range results {
		results[i] = s.highlightResult(results[i], tokens)
	}

	return &SearchResponse{
		Query:     query.Query,
		TotalHits: totalHits,
		Results:   results,
		TookMs:    time.Since(start).Milliseconds(),
	}, nil
}

// searchFilename 搜索文件名
func (s *Searcher) searchFilename(tokens []string, query *SearchQuery) []SearchResult {
	resultMap := make(map[string]*SearchResult)

	// 使用原始查询和分词后的 tokens 双重匹配
	queryLower := strings.ToLower(query.Query)

	s.idx.idx.mu.RLock()
	for _, doc := range s.idx.idx.docs {
		nameLower := strings.ToLower(doc.Name)
		matched := false

		// 检查完整查询是否在文件名中
		if strings.Contains(nameLower, queryLower) {
			matched = true
		}

		// 检查各个 token 是否在文件名中
		if !matched {
			for _, token := range tokens {
				if strings.Contains(nameLower, token) {
					matched = true
					break
				}
			}
		}

		if !matched {
			continue
		}

		// 计算分数
		score := 1.0
		if nameLower == queryLower {
			score = 10.0
		} else if strings.HasPrefix(nameLower, queryLower) {
			score = 5.0
		} else if strings.Contains(nameLower, queryLower) {
			score = 3.0
		}

		if existing, ok := resultMap[doc.ID]; ok {
			if score > existing.Score {
				existing.Score = score
			}
			existing.FilenameMatch = true
		} else {
			resultMap[doc.ID] = &SearchResult{
				DocID:         doc.ID,
				Path:          doc.Path,
				Name:          doc.Name,
				Extension:     doc.Extension,
				Size:          doc.Size,
				FileType:      doc.FileType,
				MimeType:      doc.MimeType,
				ModTime:       doc.ModTime,
				Score:         score,
				FilenameMatch: true,
				Metadata:      doc.Metadata,
			}
		}
	}
	s.idx.idx.mu.RUnlock()

	results := make([]SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	// 按 DocID 排序确保稳定顺序
	sort.Slice(results, func(i, j int) bool {
		return results[i].DocID < results[j].DocID
	})

	return results
}

// searchContent 搜索文件内容
func (s *Searcher) searchContent(tokens []string, query *SearchQuery) []SearchResult {
	resultMap := make(map[string]*SearchResult)

	for _, token := range tokens {
		postings := s.idx.GetPostings(token)
		if postings == nil {
			continue
		}

		s.idx.idx.mu.RLock()
		for _, posting := range postings {
			doc, exists := s.idx.idx.docs[posting.DocID]
			if !exists {
				continue
			}

			// 检查内容是否包含该词
			if !strings.Contains(strings.ToLower(doc.Content), token) {
				continue
			}

			// 计算分数
			totalDocs := int(s.idx.idx.docCount)
			docFreq := s.idx.docFreq[token]
			score := s.idx.calculateTFIDF(posting.TermFreq, docFreq, totalDocs)

			// 内容匹配给予更高权重
			score *= 1.5

			if existing, ok := resultMap[doc.ID]; ok {
				existing.Score += score
				existing.ContentMatch = true
			} else {
				resultMap[doc.ID] = &SearchResult{
					DocID:        doc.ID,
					Path:         doc.Path,
					Name:         doc.Name,
					Extension:    doc.Extension,
					Size:         doc.Size,
					FileType:     doc.FileType,
					MimeType:     doc.MimeType,
					ModTime:      doc.ModTime,
					Score:        score,
					ContentMatch: true,
					Metadata:     doc.Metadata,
				}
			}
		}
		s.idx.idx.mu.RUnlock()
	}

	results := make([]SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	// 按 DocID 排序确保稳定顺序
	sort.Slice(results, func(i, j int) bool {
		return results[i].DocID < results[j].DocID
	})

	return results
}

// mergeResults 合并文件名和内容搜索结果
func (s *Searcher) mergeResults(filenameResults, contentResults []SearchResult) []SearchResult {
	resultMap := make(map[string]*SearchResult)

	for _, r := range filenameResults {
		resultMap[r.DocID] = &r
	}

	for _, r := range contentResults {
		if existing, ok := resultMap[r.DocID]; ok {
			existing.Score += r.Score
			existing.ContentMatch = true
		} else {
			copy := r
			resultMap[r.DocID] = &copy
		}
	}

	results := make([]SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	// 按 DocID 排序确保稳定顺序
	sort.Slice(results, func(i, j int) bool {
		return results[i].DocID < results[j].DocID
	})

	return results
}

// applyFilters 应用过滤条件
func (s *Searcher) applyFilters(results []SearchResult, query *SearchQuery) []SearchResult {
	if len(results) == 0 {
		return results
	}

	filtered := make([]SearchResult, 0, len(results))

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

		// 路径过滤
		if query.Path != "" {
			if !strings.HasPrefix(r.Path, query.Path) {
				continue
			}
		}

		// 时间过滤
		if query.After != nil && r.ModTime.Before(*query.After) {
			continue
		}
		if query.Before != nil && r.ModTime.After(*query.Before) {
			continue
		}

		// 大小过滤
		if query.MinSize != nil && r.Size < *query.MinSize {
			continue
		}
		if query.MaxSize != nil && r.Size > *query.MaxSize {
			continue
		}

		filtered = append(filtered, r)
	}

	return filtered
}

// highlightResult 高亮搜索结果
func (s *Searcher) highlightResult(result SearchResult, tokens []string) SearchResult {
	highlightStart := fmt.Sprintf("<%s>", s.config.HighlightTag)
	highlightEnd := fmt.Sprintf("</%s>", s.config.HighlightTag)

	// 高亮文件名
	nameLower := strings.ToLower(result.Name)
	highlightedName := result.Name
	matchPositions := make([]MatchPosition, 0)

	for _, token := range tokens {
		idx := 0
		for {
			pos := strings.Index(nameLower[idx:], token)
			if pos == -1 {
				break
			}

			actualPos := idx + pos
			matchPositions = append(matchPositions, MatchPosition{
				Field: "filename",
				Start: actualPos,
				End:   actualPos + len(token),
				Term:  token,
			})

			// 应用高亮
			before := highlightedName[:actualPos]
			match := highlightedName[actualPos : actualPos+len(token)]
			after := highlightedName[actualPos+len(token):]
			highlightedName = before + highlightStart + match + highlightEnd + after

			// 调整后续位置偏移
			offset := len(highlightStart) + len(highlightEnd)
			idx = actualPos + len(token) + offset
			nameLower = strings.ToLower(highlightedName)
		}
	}
	result.HighlightName = highlightedName

	// 内容摘要高亮
	if result.ContentMatch {
		s.idx.idx.mu.RLock()
		doc, exists := s.idx.idx.docs[result.DocID]
		s.idx.idx.mu.RUnlock()

		if exists && doc.Content != "" {
			snippet := s.generateSnippet(doc.Content, tokens, s.config.SnippetLen)
			result.HighlightSnip = snippet
		}
	}

	result.MatchPositions = matchPositions
	return result
}

// generateSnippet 生成内容摘要
func (s *Searcher) generateSnippet(content string, tokens []string, maxLen int) string {
	contentLower := strings.ToLower(content)

	// 找到第一个匹配的位置
	bestPos := -1
	for _, token := range tokens {
		pos := strings.Index(contentLower, token)
		if pos >= 0 && (bestPos == -1 || pos < bestPos) {
			bestPos = pos
		}
	}

	if bestPos == -1 {
		// 没有匹配，返回开头
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	// 计算摘要范围
	start := bestPos - maxLen/4
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(content) {
		end = len(content)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}

	snippet := content[start:end]

	// 添加省略号
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	// 高亮关键词
	highlightStart := fmt.Sprintf("<%s>", s.config.HighlightTag)
	highlightEnd := fmt.Sprintf("</%s>", s.config.HighlightTag)
	snippetLower := strings.ToLower(snippet)

	for _, token := range tokens {
		idx := 0
		for {
			pos := strings.Index(snippetLower[idx:], token)
			if pos == -1 {
				break
			}

			actualPos := idx + pos
			before := snippet[:actualPos]
			match := snippet[actualPos : actualPos+len(token)]
			after := snippet[actualPos+len(token):]
			snippet = before + highlightStart + match + highlightEnd + after

			offset := len(highlightStart) + len(highlightEnd)
			idx = actualPos + len(token) + offset
			snippetLower = strings.ToLower(snippet)
		}
	}

	return snippet
}

// calculateBM25 计算 BM25 分数
func (s *Searcher) calculateBM25(tf int, docLen int, avgDocLen float64, docFreq int, totalDocs int) float64 {
	k1 := 1.2
	b := 0.75

	if totalDocs == 0 || docFreq == 0 || avgDocLen == 0 {
		return 0
	}

	// IDF 部分
	idf := math.Log((float64(totalDocs)-float64(docFreq)+0.5)/(float64(docFreq)+0.5) + 1.0)

	// TF 部分
	tfNorm := (float64(tf) * (k1 + 1)) / (float64(tf) + k1*(1-b+b*float64(docLen)/avgDocLen))

	return idf * tfNorm
}

// AutoComplete 自动补全建议
func (s *Searcher) AutoComplete(prefix string, limit int) []string {
	if limit <= 0 {
		limit = 10
	}

	prefix = strings.ToLower(prefix)
	if len([]rune(prefix)) < s.config.MinTermLen {
		return nil
	}

	s.idx.idx.mu.RLock()
	defer s.idx.idx.mu.RUnlock()

	suggestions := make([]string, 0, limit)
	seen := make(map[string]bool)

	// 搜索匹配的文档名
	for _, doc := range s.idx.idx.docs {
		nameLower := strings.ToLower(doc.Name)
		if strings.Contains(nameLower, prefix) {
			if !seen[doc.Name] {
				seen[doc.Name] = true
				suggestions = append(suggestions, doc.Name)
				if len(suggestions) >= limit {
					break
				}
			}
		}
	}

	return suggestions
}
