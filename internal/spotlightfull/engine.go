// Package spotlightfull - 搜索引擎核心
// Package spotlightfull is experimental full-text spotlight.
// Deprecated: prefer internal/search; do not add new product dependencies here.
package spotlightfull

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// NewSearchEngine 创建搜索引擎实例.
func NewSearchEngine(config *EngineConfig) (*SearchEngine, error) {
	if config == nil {
		config = DefaultEngineConfig()
	}

	// 确保索引目录存在
	if err := os.MkdirAll(config.IndexDir, 0755); err != nil {
		return nil, fmt.Errorf("创建索引目录失败: %w", err)
	}

	engine := &SearchEngine{
		index: &invertedIndex{
			index:    make(map[string]map[string]*termPositions),
			docs:     make(map[string]*IndexEntry),
			trieRoot: &trieNode{children: make(map[rune]*trieNode)},
		},
		tokenizer: newCJKTokenizer(),
		config:    config,
		stopCh:    make(chan struct{}),
	}

	// 尝试加载已有索引
	if err := engine.loadIndex(); err != nil {
		// 索引文件不存在或损坏，从空索引开始
		fmt.Printf("[spotlightfull] 加载索引失败，将从空索引开始: %v\n", err)
	}

	return engine, nil
}

// Search 执行搜索.
func (e *SearchEngine) Search(filter *SearchFilter) (*SearchResponse, error) {
	start := time.Now()

	if filter.Query == "" {
		return &SearchResponse{
			Results:  []SearchResult{},
			Total:    0,
			Page:     filter.Page,
			PageSize: filter.PageSize,
			Query:    filter.Query,
			Duration: time.Since(start).String(),
		}, nil
	}

	// 参数校验
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 200 {
		filter.PageSize = 20
	}

	// 分词处理
	tokens := e.tokenizer.tokenize(filter.Query)

	// 收集候选文档
	candidateScores := e.collectCandidates(tokens, filter)

	// 排序
	e.sortResults(candidateScores, filter.SortBy, filter.SortOrder)

	// 分页
	total := len(candidateScores)
	startIdx := (filter.Page - 1) * filter.PageSize
	endIdx := startIdx + filter.PageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}

	// 构建结果
	results := make([]SearchResult, 0, endIdx-startIdx)
	for _, cs := range candidateScores[startIdx:endIdx] {
		entry := cs.entry
		result := SearchResult{
			ID:         entry.ID,
			Path:       entry.Path,
			Name:       entry.Name,
			Extension:  entry.Extension,
			FileType:   entry.FileType,
			Size:       entry.Size,
			MimeType:   entry.MimeType,
			MatchType:  cs.matchType,
			Highlights: e.buildHighlights(entry, tokens),
			Score:      cs.score,
			Thumbnail:  e.buildThumbnailURL(entry),
			ModifiedAt: entry.ModifiedAt,
			IndexedAt:  entry.IndexedAt,
		}
		results = append(results, result)
	}

	return &SearchResponse{
		Results:  results,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Query:    filter.Query,
		Duration: time.Since(start).String(),
		Suggests: e.getSuggestions(filter.Query),
	}, nil
}

// AddDocument 添加文档到索引.
func (e *SearchEngine) AddDocument(entry *IndexEntry) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateDocID(entry.Path)
	}
	entry.IndexedAt = time.Now()

	// 移除旧文档（如果存在）
	e.index.removeDocument(entry.ID)

	// 添加新文档
	e.index.addDocument(entry)

	return nil
}

// RemoveDocument 从索引中移除文档.
func (e *SearchEngine) RemoveDocument(docID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.index.removeDocument(docID)
	return nil
}

// GetStats 获取索引统计.
func (e *SearchEngine) GetStats() *IndexStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &IndexStats{
		TotalFiles:   e.index.totalDocs,
		TotalTerms:   int64(e.index.totalTerms),
		IndexSize:    e.calculateIndexSize(),
		LastUpdateAt: e.lastUpdate,
		IsBuilding:   e.isBuilding,
		Progress:     e.progress,
	}
}

// RebuildIndex 重建索引.
func (e *SearchEngine) RebuildIndex() error {
	e.mu.Lock()
	if e.isBuilding {
		e.mu.Unlock()
		return fmt.Errorf("索引正在构建中")
	}
	e.isBuilding = true
	e.progress = 0
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.isBuilding = false
		e.progress = 1.0
		e.lastUpdate = time.Now()
		e.mu.Unlock()
	}()

	// 清空现有索引
	e.mu.Lock()
	e.index = &invertedIndex{
		index:    make(map[string]map[string]*termPositions),
		docs:     make(map[string]*IndexEntry),
		trieRoot: &trieNode{children: make(map[rune]*trieNode)},
	}
	e.mu.Unlock()

	return nil
}

// Stop 停止搜索引擎.
func (e *SearchEngine) Stop() {
	close(e.stopCh)
}

// SaveIndex 持久化索引到磁盘.
func (e *SearchEngine) SaveIndex() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	indexFile := filepath.Join(e.config.IndexDir, "index.json")
	data := struct {
		Docs       map[string]*IndexEntry `json:"docs"`
		TotalDocs  int64                  `json:"total_docs"`
		TotalTerms int                    `json:"total_terms"`
		SavedAt    time.Time              `json:"saved_at"`
	}{
		Docs:       e.index.docs,
		TotalDocs:  e.index.totalDocs,
		TotalTerms: e.index.totalTerms,
		SavedAt:    time.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化索引失败: %w", err)
	}

	return os.WriteFile(indexFile, jsonData, 0644)
}

// loadIndex 从磁盘加载索引.
func (e *SearchEngine) loadIndex() error {
	indexFile := filepath.Join(e.config.IndexDir, "index.json")
	data, err := os.ReadFile(indexFile)
	if err != nil {
		return err
	}

	var saved struct {
		Docs       map[string]*IndexEntry `json:"docs"`
		TotalDocs  int64                  `json:"total_docs"`
		TotalTerms int                    `json:"total_terms"`
		SavedAt    time.Time              `json:"saved_at"`
	}

	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("反序列化索引失败: %w", err)
	}

	// 重建倒排索引和前缀树
	e.index.docs = saved.Docs
	e.index.totalDocs = saved.TotalDocs
	for _, entry := range saved.Docs {
		e.index.addToInvertedIndex(entry)
		e.index.addToTrie(entry.Name)
	}
	e.index.totalTerms = len(e.index.index)

	return nil
}

// collectCandidates 收集候选文档并计算评分.
func (e *SearchEngine) collectCandidates(tokens []string, filter *SearchFilter) []*candidateScore {
	e.index.mu.RLock()
	defer e.index.mu.RUnlock()

	scoreMap := make(map[string]*candidateScore)

	for _, token := range tokens {
		lowerToken := strings.ToLower(token)

		// 精确匹配倒排索引
		if postings, ok := e.index.index[lowerToken]; ok {
			for docID, pos := range postings {
				entry, exists := e.index.docs[docID]
				if !exists {
					continue
				}
				if !e.passFilter(entry, filter) {
					continue
				}

				if cs, found := scoreMap[docID]; found {
					cs.score += e.calculateTokenScore(entry, token, pos)
					if cs.matchType == MatchFuzzy {
						cs.matchType = e.determineMatchType(entry, token)
					}
				} else {
					scoreMap[docID] = &candidateScore{
						entry:     entry,
						score:     e.calculateTokenScore(entry, token, pos),
						matchType: e.determineMatchType(entry, token),
					}
				}
			}
		}

		// 文件名模糊匹配
		for docID, entry := range e.index.docs {
			if _, alreadyFound := scoreMap[docID]; alreadyFound {
				continue
			}
			if !e.passFilter(entry, filter) {
				continue
			}
			if fuzzyScore := e.fuzzyMatch(lowerToken, strings.ToLower(entry.Name)); fuzzyScore > 0.3 {
				scoreMap[docID] = &candidateScore{
					entry:     entry,
					score:     fuzzyScore * 0.6, // 模糊匹配权重较低
					matchType: MatchFuzzy,
				}
			}
		}
	}

	// 转换为切片
	candidates := make([]*candidateScore, 0, len(scoreMap))
	for _, cs := range scoreMap {
		candidates = append(candidates, cs)
	}

	return candidates
}

// calculateTokenScore 计算单个词项对文档的评分.
func (e *SearchEngine) calculateTokenScore(entry *IndexEntry, token string, pos *termPositions) float64 {
	score := 0.0
	lowerToken := strings.ToLower(token)
	lowerName := strings.ToLower(entry.Name)

	// 文件名匹配权重最高
	if strings.Contains(lowerName, lowerToken) {
		score += 3.0
		if lowerName == lowerToken {
			score += 2.0 // 完全匹配加分
		}
	}

	// 内容匹配
	for _, field := range pos.Fields {
		switch field {
		case "content":
			score += 1.0
		case "name":
			score += 3.0
		case "tags":
			score += 2.0
		case "metadata":
			score += 1.5
		}
	}

	// 词频因子
	score *= (1.0 + math.Log1p(float64(pos.Count)))

	// 文档长度归一化（短文档得分更高）
	docLength := float64(utf8.RuneCountInString(entry.Content) + utf8.RuneCountInString(entry.Name))
	if docLength > 0 {
		score /= math.Sqrt(docLength / 100.0)
	}

	return score
}

// determineMatchType 确定匹配类型.
func (e *SearchEngine) determineMatchType(entry *IndexEntry, token string) MatchType {
	lowerToken := strings.ToLower(token)
	lowerName := strings.ToLower(entry.Name)

	if strings.Contains(lowerName, lowerToken) {
		return MatchFileName
	}
	if strings.Contains(strings.ToLower(entry.Content), lowerToken) {
		return MatchContent
	}
	for k, v := range entry.Metadata {
		if strings.Contains(strings.ToLower(k), lowerToken) || strings.Contains(strings.ToLower(v), lowerToken) {
			return MatchMetadata
		}
	}
	return MatchFuzzy
}

// passFilter 检查文档是否通过过滤条件.
func (e *SearchEngine) passFilter(entry *IndexEntry, filter *SearchFilter) bool {
	// 文件类型过滤
	if len(filter.FileTypes) > 0 {
		matched := false
		for _, ft := range filter.FileTypes {
			if entry.FileType == ft {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 大小范围过滤
	if filter.MinSize != nil && entry.Size < *filter.MinSize {
		return false
	}
	if filter.MaxSize != nil && entry.Size > *filter.MaxSize {
		return false
	}

	// 日期范围过滤
	if filter.After != nil && entry.ModifiedAt.Before(*filter.After) {
		return false
	}
	if filter.Before != nil && entry.ModifiedAt.After(*filter.Before) {
		return false
	}

	// 路径范围过滤
	if filter.PathScope != "" {
		if !strings.HasPrefix(entry.Path, filter.PathScope) {
			return false
		}
	}

	return true
}

// fuzzyMatch 模糊匹配（编辑距离归一化）.
func (e *SearchEngine) fuzzyMatch(pattern, text string) float64 {
	if pattern == "" || text == "" {
		return 0
	}
	if strings.Contains(text, pattern) {
		return 0.9
	}

	// 简单的字符重叠度计算
	pRunes := []rune(pattern)
	tRunes := []rune(text)
	matches := 0
	j := 0
	for i := 0; i < len(pRunes) && j < len(tRunes); i++ {
		for j < len(tRunes) {
			if pRunes[i] == tRunes[j] {
				matches++
				j++
				break
			}
			j++
		}
	}

	return float64(matches) / float64(len(pRunes))
}

// buildHighlights 构建高亮片段.
func (e *SearchEngine) buildHighlights(entry *IndexEntry, tokens []string) []string {
	highlights := make([]string, 0, 3)

	content := entry.Content
	name := entry.Name

	for _, token := range tokens {
		lowerToken := strings.ToLower(token)

		// 名称高亮
		if strings.Contains(strings.ToLower(name), lowerToken) {
			highlights = append(highlights, highlightText(name, token))
		}

		// 内容高亮（取前3个匹配片段）
		if len(content) > 0 {
			snippets := extractSnippets(content, token, 3, 80)
			highlights = append(highlights, snippets...)
		}
	}

	// 去重并限制数量
	seen := make(map[string]bool)
	result := make([]string, 0, 5)
	for _, h := range highlights {
		if !seen[h] && len(result) < 5 {
			seen[h] = true
			result = append(result, h)
		}
	}

	return result
}

// buildThumbnailURL 构建缩略图URL.
func (e *SearchEngine) buildThumbnailURL(entry *IndexEntry) string {
	switch entry.FileType {
	case FileTypeImage, FileTypeVideo:
		return fmt.Sprintf("/api/v1/thumbnail/%s", entry.ID)
	default:
		return ""
	}
}

// getSuggestions 获取搜索建议.
func (e *SearchEngine) getSuggestions(query string) []string {
	if len(query) < 2 {
		return nil
	}

	tokens := e.tokenizer.tokenize(query)
	suggestions := make([]string, 0, 5)
	seen := make(map[string]bool)

	e.index.mu.RLock()
	defer e.index.mu.RUnlock()

	for _, token := range tokens {
		// 通过前缀树查找建议
		prefix := strings.ToLower(token)
		matches := e.index.searchTrie(prefix, 5)
		for _, m := range matches {
			if !seen[m] && m != token {
				seen[m] = true
				suggestions = append(suggestions, m)
			}
		}
	}

	return suggestions
}

// calculateIndexSize 计算索引占用空间.
func (e *SearchEngine) calculateIndexSize() int64 {
	size := int64(0)
	for _, entry := range e.index.docs {
		size += int64(len(entry.Content)) + int64(len(entry.Name)) + int64(len(entry.Path))
		for k, v := range entry.Metadata {
			size += int64(len(k)) + int64(len(v))
		}
	}
	// 加上倒排索引开销
	size += int64(len(e.index.index)) * 64
	return size
}

// sortResults 对搜索结果排序.
func (e *SearchEngine) sortResults(candidates []*candidateScore, sortBy, sortOrder string) {
	sort.Slice(candidates, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "date":
			less = candidates[i].entry.ModifiedAt.Before(candidates[j].entry.ModifiedAt)
		case "size":
			less = candidates[i].entry.Size < candidates[j].entry.Size
		case "name":
			less = candidates[i].entry.Name < candidates[j].entry.Name
		default: // relevance
			less = candidates[i].score > candidates[j].score
		}
		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

// candidateScore 候选文档评分.
type candidateScore struct {
	entry     *IndexEntry
	score     float64
	matchType MatchType
}

// ---- 倒排索引操作 ----

// addDocument 向倒排索引添加文档.
func (idx *invertedIndex) addDocument(entry *IndexEntry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.docs[entry.ID] = entry
	idx.addToInvertedIndex(entry)
	idx.addToTrie(entry.Name)
	idx.totalDocs = int64(len(idx.docs))
	idx.totalTerms = len(idx.index)
}

// removeDocument 从倒排索引移除文档.
func (idx *invertedIndex) removeDocument(docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if entry, ok := idx.docs[docID]; ok {
		// 从倒排索引中移除
		tokens := tokenizeText(entry.Name + " " + entry.Content)
		for _, token := range tokens {
			lower := strings.ToLower(token)
			if postings, exists := idx.index[lower]; exists {
				delete(postings, docID)
				if len(postings) == 0 {
					delete(idx.index, lower)
				}
			}
		}
		delete(idx.docs, docID)
		idx.totalDocs = int64(len(idx.docs))
		idx.totalTerms = len(idx.index)
	}
}

// addToInvertedIndex 将文档添加到倒排索引.
func (idx *invertedIndex) addToInvertedIndex(entry *IndexEntry) {
	// 使用统一的分词函数
	tokenizeAndIndex := func(text, field string) {
		for _, token := range tokenizeText(text) {
			lower := strings.ToLower(token)
			idx.addPosting(lower, entry.ID, field, 1)
		}
	}

	// 索引文件名
	tokenizeAndIndex(entry.Name, "name")

	// 索引内容
	if entry.Content != "" {
		tokenizeAndIndex(entry.Content, "content")
	}

	// 索引标签
	for _, tag := range entry.Tags {
		idx.addPosting(strings.ToLower(tag), entry.ID, "tags", 1)
	}

	// 索引元数据
	for k, v := range entry.Metadata {
		idx.addPosting(strings.ToLower(k), entry.ID, "metadata", 1)
		idx.addPosting(strings.ToLower(v), entry.ID, "metadata", 1)
	}
}

// addPosting 添加倒排索引条目.
func (idx *invertedIndex) addPosting(term, docID, field string, count int) {
	if idx.index[term] == nil {
		idx.index[term] = make(map[string]*termPositions)
	}
	if pos, ok := idx.index[term][docID]; ok {
		pos.Count += count
		found := false
		for _, f := range pos.Fields {
			if f == field {
				found = true
				break
			}
		}
		if !found {
			pos.Fields = append(pos.Fields, field)
		}
	} else {
		idx.index[term][docID] = &termPositions{
			Fields: []string{field},
			Count:  count,
		}
	}
}

// addToTrie 添加词项到前缀树.
func (idx *invertedIndex) addToTrie(text string) {
	tokens := tokenizeText(text)
	for _, token := range tokens {
		lower := strings.ToLower(token)
		node := idx.trieRoot
		for _, r := range lower {
			if node.children[r] == nil {
				node.children[r] = &trieNode{children: make(map[rune]*trieNode)}
			}
			node = node.children[r]
		}
		node.isEnd = true
		// 避免重复
		for _, t := range node.terms {
			if t == lower {
				return
			}
		}
		node.terms = append(node.terms, lower)
	}
}

// searchTrie 前缀搜索.
func (idx *invertedIndex) searchTrie(prefix string, limit int) []string {
	node := idx.trieRoot
	for _, r := range prefix {
		if node.children[r] == nil {
			return nil
		}
		node = node.children[r]
	}

	results := make([]string, 0, limit)
	collectTerms(node, &results, limit)
	return results
}

// collectTerms 递归收集前缀树中的词项.
func collectTerms(node *trieNode, results *[]string, limit int) {
	if len(*results) >= limit {
		return
	}
	if node.isEnd {
		*results = append(*results, node.terms...)
	}
	for _, child := range node.children {
		collectTerms(child, results, limit)
	}
}

// ---- 工具函数 ----

// generateDocID 根据路径生成文档ID.
func generateDocID(path string) string {
	// 简单的路径哈希作为ID
	h := uint632(0)
	for _, c := range path {
		h = h*31 + uint64(c)
	}
	return fmt.Sprintf("%x", h)
}

func uint632(n int64) uint64 {
	if n < 0 {
		return uint64(-n)
	}
	return uint64(n)
}

// tokenizeText 文本分词（统一使用中文分词逻辑）.
func tokenizeText(text string) []string {
	var tokens []string
	runes := []rune(text)
	i := 0

	for i < len(runes) {
		r := runes[i]

		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 中文字符：产生单字和bigram
		if unicode.Is(unicode.Han, r) {
			tokens = append(tokens, string(r))
			if i+1 < len(runes) && unicode.Is(unicode.Han, runes[i+1]) {
				tokens = append(tokens, string(runes[i:i+2]))
			}
			i++
			continue
		}

		// ASCII 字母和数字：按连续序列分割
		if isASCIILetterOrDigit(r) {
			j := i
			for j < len(runes) && isASCIILetterOrDigit(runes[j]) {
				j++
			}
			tokens = append(tokens, string(runes[i:j]))
			i = j
			continue
		}

		i++
	}

	return tokens
}

// highlightText 高亮文本中的关键词.
func highlightText(text, keyword string) string {
	lowerText := strings.ToLower(text)
	lowerKeyword := strings.ToLower(keyword)
	idx := strings.Index(lowerText, lowerKeyword)
	if idx < 0 {
		return text
	}
	end := idx + len(keyword)
	return text[:idx] + "【" + text[idx:end] + "】" + text[end:]
}

// extractSnippets 从内容中提取关键词周围的片段.
func extractSnippets(content, keyword string, maxSnippets, contextLen int) []string {
	snippets := make([]string, 0, maxSnippets)
	lowerContent := strings.ToLower(content)
	lowerKeyword := strings.ToLower(keyword)

	start := 0
	for i := 0; i < maxSnippets; i++ {
		idx := strings.Index(lowerContent[start:], lowerKeyword)
		if idx < 0 {
			break
		}
		absIdx := start + idx

		// 取上下文
		snippetStart := absIdx - contextLen
		if snippetStart < 0 {
			snippetStart = 0
		}
		snippetEnd := absIdx + len(keyword) + contextLen
		if snippetEnd > len(content) {
			snippetEnd = len(content)
		}

		snippet := content[snippetStart:snippetEnd]
		if snippetStart > 0 {
			snippet = "..." + snippet
		}
		if snippetEnd < len(content) {
			snippet = snippet + "..."
		}

		snippets = append(snippets, snippet)
		start = absIdx + len(keyword)
	}

	return snippets
}

// newCJKTokenizer 创建中文分词器.
func newCJKTokenizer() *cjkTokenizer {
	return &cjkTokenizer{
		dict: make(map[string]bool),
		stopWord: map[string]bool{
			"的": true, "了": true, "是": true, "在": true, "我": true,
			"有": true, "和": true, "就": true, "不": true, "人": true,
			"都": true, "一": true, "一个": true, "上": true, "也": true,
			"很": true, "到": true, "说": true, "要": true, "去": true,
			"你": true, "会": true, "着": true, "没有": true, "看": true,
			"好": true, "自己": true, "这": true, "他": true, "她": true,
			"它": true, "们": true, "那": true, "被": true, "从": true,
		},
	}
}

// tokenize 中文分词（简易实现：单字 + bigram）.
func (t *cjkTokenizer) tokenize(text string) []string {
	var tokens []string
	runes := []rune(text)
	i := 0

	for i < len(runes) {
		r := runes[i]

		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 中文字符：产生单字和bigram
		if unicode.Is(unicode.Han, r) {
			word := string(r)
			if !t.stopWord[word] {
				tokens = append(tokens, word)
			}
			// bigram
			if i+1 < len(runes) && unicode.Is(unicode.Han, runes[i+1]) {
				bigram := string(runes[i : i+2])
				if !t.stopWord[bigram] {
					tokens = append(tokens, bigram)
				}
			}
			i++
			continue
		}

		// ASCII 字母和数字：按连续序列分割
		if isASCIILetterOrDigit(r) {
			j := i
			for j < len(runes) && isASCIILetterOrDigit(runes[j]) {
				j++
			}
			word := string(runes[i:j])
			tokens = append(tokens, word)
			i = j
			continue
		}

		i++
	}

	return tokens
}

// isASCIILetterOrDigit 检查是否为 ASCII 字母或数字.
func isASCIILetterOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
