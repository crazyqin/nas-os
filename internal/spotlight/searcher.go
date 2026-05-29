package spotlight

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Search 执行搜索
func (m *Manager) Search(req *SearchRequest) (*SearchResult, error) {
	start := time.Now()

	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	// 验证请求
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// 分页参数
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	if req.PageSize > m.config.MaxResults {
		req.PageSize = m.config.MaxResults
	}

	// 分词
	queryTokens := m.tokenizeQuery(req.Query)
	if len(queryTokens) == 0 {
		return &SearchResult{
			Documents:  []ScoredDocument{},
			Total:      0,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: 0,
			Query:      req.Query,
			Duration:   time.Since(start).String(),
		}, nil
	}

	// 查找匹配文档
	candidates := m.findCandidates(queryTokens)
	if len(candidates) == 0 {
		// 生成搜索建议
		suggestions := m.generateSuggestions(req.Query)

		return &SearchResult{
			Documents:   []ScoredDocument{},
			Total:       0,
			Page:        req.Page,
			PageSize:    req.PageSize,
			TotalPages:  0,
			Query:       req.Query,
			Duration:    time.Since(start).String(),
			Suggestions: suggestions,
		}, nil
	}

	// 计算分数并过滤
	scored := m.scoreDocuments(candidates, queryTokens, req)

	// 排序
	m.sortResults(scored, req.SortBy, req.SortOrder)

	// 分页
	total := len(scored)
	totalPages := (total + req.PageSize - 1) / req.PageSize

	startIdx := (req.Page - 1) * req.PageSize
	endIdx := startIdx + req.PageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}

	pageResults := scored[startIdx:endIdx]

	result := &SearchResult{
		Documents:  pageResults,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
		Query:      req.Query,
		Duration:   time.Since(start).String(),
	}

	m.logger.Debug("search completed",
		zap.String("query", req.Query),
		zap.Int("total", total),
		zap.String("duration", time.Since(start).String()))

	return result, nil
}

// Suggest 获取搜索建议
func (m *Manager) Suggest(req *SuggestRequest) (*SuggestResponse, error) {
	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = m.config.SuggestionLimit
	}

	query := strings.ToLower(strings.TrimSpace(req.Query))

	// 前缀搜索
	suggestions := m.index.trieRoot.search(query, limit)

	// 如果结果不足，尝试拼写纠正
	if len(suggestions) < limit {
		corrections := m.spellCorrect(query, limit-len(suggestions))
		suggestions = append(suggestions, corrections...)
	}

	// 去重
	suggestions = deduplicateSuggestions(suggestions)

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return &SuggestResponse{
		Suggestions: suggestions,
		Query:       req.Query,
	}, nil
}

// tokenizeQuery 分词查询
func (m *Manager) tokenizeQuery(query string) []string {
	return m.tokenizer.tokenize(query)
}

// findCandidates 查找候选文档
func (m *Manager) findCandidates(queryTokens []string) map[string]float64 {
	candidates := make(map[string]float64)

	for _, token := range queryTokens {
		token = strings.ToLower(token)

		// 精确匹配
		if postings, exists := m.index.index[token]; exists {
			for docID, pos := range postings {
				candidates[docID] += float64(pos.Count)
			}
		}

		// 前缀匹配
		for term, postings := range m.index.index {
			if strings.HasPrefix(term, token) && term != token {
				for docID, pos := range postings {
					candidates[docID] += float64(pos.Count) * 0.5 // 前缀匹配权重较低
				}
			}
		}
	}

	return candidates
}

// scoreDocuments 计算文档分数
func (m *Manager) scoreDocuments(candidates map[string]float64, queryTokens []string, req *SearchRequest) []ScoredDocument {
	var results []ScoredDocument

	for docID, baseScore := range candidates {
		doc, exists := m.index.docs[docID]
		if !exists {
			continue
		}

		// 应用过滤器
		if !m.matchesFilters(doc, req) {
			continue
		}

		// 计算最终分数
		score := m.calculateScore(doc, queryTokens, baseScore)

		// 生成高亮
		highlights := m.generateHighlights(doc, queryTokens)

		// 匹配原因
		matchReason := m.getMatchReason(doc, queryTokens)

		results = append(results, ScoredDocument{
			Document:    *doc,
			Score:       score,
			Highlights:  highlights,
			MatchReason: matchReason,
		})
	}

	return results
}

// matchesFilters 检查文档是否匹配过滤器
func (m *Manager) matchesFilters(doc *Document, req *SearchRequest) bool {
	// 路径过滤
	if req.Path != "" && !strings.HasPrefix(doc.Path, req.Path) {
		return false
	}

	// 文件类型过滤
	if req.FileType != "" && doc.FileType != req.FileType {
		return false
	}

	// 扩展名过滤
	if len(req.Extensions) > 0 {
		ext := strings.ToLower(strings.TrimPrefix(doc.Extension, "."))
		found := false
		for _, e := range req.Extensions {
			if strings.ToLower(strings.TrimPrefix(e, ".")) == ext {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 标签过滤
	if len(req.Tags) > 0 {
		for _, tag := range req.Tags {
			if !containsTag(doc.Tags, tag) {
				return false
			}
		}
	}

	// 大小过滤
	if req.MinSize != nil && doc.Size < *req.MinSize {
		return false
	}
	if req.MaxSize != nil && doc.Size > *req.MaxSize {
		return false
	}

	// 日期过滤
	if req.After != nil && doc.UpdatedAt.Before(*req.After) {
		return false
	}
	if req.Before != nil && doc.UpdatedAt.After(*req.Before) {
		return false
	}

	return true
}

// calculateScore 计算文档分数
func (m *Manager) calculateScore(doc *Document, queryTokens []string, baseScore float64) float64 {
	score := baseScore

	// TF-IDF 权重
	totalDocs := float64(len(m.index.docs))
	if totalDocs == 0 {
		totalDocs = 1
	}

	for _, token := range queryTokens {
		token = strings.ToLower(token)

		// 词频
		tf := 0.0
		if postings, exists := m.index.index[token]; exists {
			if pos, ok := postings[doc.ID]; ok {
				tf = float64(pos.Count)
			}
		}

		// 文档频率
		df := 0.0
		if postings, exists := m.index.index[token]; exists {
			df = float64(len(postings))
		}

		// IDF
		idf := math.Log(totalDocs / (df + 1))

		// TF-IDF
		score += tf * idf
	}

	// 字段权重
	nameTokens := m.tokenizeQuery(doc.Name)
	for _, token := range queryTokens {
		for _, nameToken := range nameTokens {
			if strings.Contains(strings.ToLower(nameToken), strings.ToLower(token)) {
				score *= 2.0 // 文件名匹配权重
				break
			}
		}
	}

	// 标签权重
	for _, token := range queryTokens {
		for _, tag := range doc.Tags {
			if strings.Contains(strings.ToLower(tag), strings.ToLower(token)) {
				score *= 1.5
				break
			}
		}
	}

	// 时间衰减（最近访问的权重更高）
	daysSinceUpdate := time.Since(doc.UpdatedAt).Hours() / 24
	if daysSinceUpdate < 7 {
		score *= 1.2
	} else if daysSinceUpdate < 30 {
		score *= 1.1
	}

	return score
}

// generateHighlights 生成高亮片段
func (m *Manager) generateHighlights(doc *Document, queryTokens []string) []string {
	var highlights []string

	// 在文件名中高亮
	nameHighlight := m.highlightText(doc.Name, queryTokens)
	if nameHighlight != doc.Name {
		highlights = append(highlights, nameHighlight)
	}

	// 在内容中高亮
	if doc.Content != "" {
		contentHighlights := m.highlightContent(doc.Content, queryTokens, 3)
		highlights = append(highlights, contentHighlights...)
	}

	return highlights
}

// highlightText 高亮文本
func (m *Manager) highlightText(text string, queryTokens []string) string {
	result := text
	for _, token := range queryTokens {
		token = strings.ToLower(token)
		// 简单高亮：用 <mark> 包裹
		lower := strings.ToLower(result)
		idx := strings.Index(lower, token)
		if idx >= 0 {
			original := result[idx : idx+len(token)]
			result = result[:idx] + "<mark>" + original + "</mark>" + result[idx+len(token):]
		}
	}
	return result
}

// highlightContent 高亮内容片段
func (m *Manager) highlightContent(content string, queryTokens []string, maxSnippets int) []string {
	var snippets []string
	lower := strings.ToLower(content)

	for _, token := range queryTokens {
		token = strings.ToLower(token)
		idx := 0
		for len(snippets) < maxSnippets {
			pos := strings.Index(lower[idx:], token)
			if pos < 0 {
				break
			}

			// 提取上下文
			start := idx + pos - 50
			if start < 0 {
				start = 0
			}
			end := idx + pos + len(token) + 50
			if end > len(content) {
				end = len(content)
			}

			snippet := content[start:end]
			snippet = m.highlightText(snippet, queryTokens)
			snippets = append(snippets, "..."+snippet+"...")

			idx = idx + pos + len(token)
		}
	}

	return snippets
}

// getMatchReason 获取匹配原因
func (m *Manager) getMatchReason(doc *Document, queryTokens []string) string {
	var reasons []string

	// 检查文件名匹配
	for _, token := range queryTokens {
		if strings.Contains(strings.ToLower(doc.Name), strings.ToLower(token)) {
			reasons = append(reasons, "filename")
			break
		}
	}

	// 检查内容匹配
	if doc.Content != "" {
		for _, token := range queryTokens {
			if strings.Contains(strings.ToLower(doc.Content), strings.ToLower(token)) {
				reasons = append(reasons, "content")
				break
			}
		}
	}

	// 检查标签匹配
	for _, token := range queryTokens {
		for _, tag := range doc.Tags {
			if strings.Contains(strings.ToLower(tag), strings.ToLower(token)) {
				reasons = append(reasons, "tags")
				break
			}
		}
	}

	if len(reasons) == 0 {
		return "partial match"
	}

	return strings.Join(reasons, ", ")
}

// sortResults 排序结果
func (m *Manager) sortResults(results []ScoredDocument, sortBy, sortOrder string) {
	sort.Slice(results, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "date":
			less = results[i].UpdatedAt.Before(results[j].UpdatedAt)
		case "size":
			less = results[i].Size < results[j].Size
		case "name":
			less = strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
		default: // relevance
			less = results[i].Score > results[j].Score // 分数高的排前面
		}

		if sortOrder == "asc" {
			return less
		}
		return !less
	})
}

// generateSuggestions 生成搜索建议
func (m *Manager) generateSuggestions(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))

	// 从前缀树获取建议
	suggestions := m.index.trieRoot.search(query, 5)

	var result []string
	for _, s := range suggestions {
		if s.Text != query {
			result = append(result, s.Text)
		}
	}

	return result
}

// spellCorrect 拼写纠正
func (m *Manager) spellCorrect(query string, limit int) []Suggestion {
	var suggestions []Suggestion

	// 计算编辑距离
	for term := range m.index.index {
		distance := editDistance(query, term)
		if distance <= 2 && distance > 0 {
			suggestions = append(suggestions, Suggestion{
				Text:  term,
				Score: 1.0 / float64(distance+1),
				Type:  "correction",
			})
		}
	}

	// 排序
	sortSuggestions(suggestions)

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions
}

// editDistance 计算编辑距离
func editDistance(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)

	m := len(r1)
	n := len(r2)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 0; i <= m; i++ {
		dp[i][0] = i
	}
	for j := 0; j <= n; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if r1[i-1] == r2[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = 1 + min(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
			}
		}
	}

	return dp[m][n]
}

// min 返回最小值
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// deduplicateSuggestions 去重建议
func deduplicateSuggestions(suggestions []Suggestion) []Suggestion {
	seen := make(map[string]bool)
	var result []Suggestion

	for _, s := range suggestions {
		if !seen[s.Text] {
			seen[s.Text] = true
			result = append(result, s)
		}
	}

	return result
}

// SearchByPath 按路径搜索
func (m *Manager) SearchByPath(path string, limit int) []*Document {
	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	var results []*Document
	path = strings.ToLower(path)

	for _, doc := range m.index.docs {
		if strings.HasPrefix(strings.ToLower(doc.Path), path) {
			results = append(results, doc)
			if len(results) >= limit {
				break
			}
		}
	}

	return results
}

// SearchByTags 按标签搜索
func (m *Manager) SearchByTags(tags []string, limit int) []*Document {
	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	var results []*Document

	for _, doc := range m.index.docs {
		matched := true
		for _, tag := range tags {
			if !containsTag(doc.Tags, tag) {
				matched = false
				break
			}
		}
		if matched {
			results = append(results, doc)
			if len(results) >= limit {
				break
			}
		}
	}

	return results
}

// GetPopularSearches 获取热门搜索
func (m *Manager) GetPopularSearches(limit int) []Suggestion {
	m.index.mu.RLock()
	defer m.index.mu.RUnlock()

	var suggestions []Suggestion

	// 从索引中提取高频词
	for term, postings := range m.index.index {
		if len(postings) > 1 { // 至少出现在2个文档中
			suggestions = append(suggestions, Suggestion{
				Text:  term,
				Score: float64(len(postings)),
				Type:  "popular",
			})
		}
	}

	// 排序
	sortSuggestions(suggestions)

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions
}

// RebuildIndex 重建索引
func (m *Manager) RebuildIndex() error {
	m.index.mu.Lock()
	defer m.index.mu.Unlock()

	m.logger.Info("rebuilding index")

	// 保存旧文档
	docs := make([]*Document, 0, len(m.index.docs))
	for _, doc := range m.index.docs {
		docs = append(docs, doc)
	}

	// 清空索引
	m.index.index = make(map[string]map[string]positions)
	m.index.docs = make(map[string]*Document)
	m.index.trieRoot = newTrieNode()

	// 重新索引
	for _, doc := range docs {
		m.index.docs[doc.ID] = doc

		nameTokens := m.tokenizer.tokenize(doc.Name)
		for _, token := range nameTokens {
			m.addToIndex(token, doc.ID, "name", 1)
		}

		if doc.Content != "" {
			contentTokens := m.tokenizer.tokenize(doc.Content)
			for i, token := range contentTokens {
				m.addToIndex(token, doc.ID, "content", i)
			}
		}

		for _, tag := range doc.Tags {
			tagTokens := m.tokenizer.tokenize(tag)
			for _, token := range tagTokens {
				m.addToIndex(token, doc.ID, "tags", 0)
			}
		}
	}

	m.logger.Info("index rebuilt", zap.Int("documents", len(docs)))
	return nil
}
