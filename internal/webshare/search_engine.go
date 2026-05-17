// Package webshare 增强搜索引擎
// 支持中英文分词、模糊匹配、过滤器、高亮显示
// 参考: TrueNAS Spotlight Search
package webshare

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.uber.org/zap"
)

// SearchEngine 增强搜索引擎
type SearchEngine struct {
	config        WebShareConfig
	logger        *zap.Logger
	mu            sync.RWMutex
	contentIndexer *ContentIndexer
	nameIndex     map[string][]string     // token -> paths (文件名索引)
	contentTokens map[string][]string     // token -> paths (内容索引)
	tagIndex      map[string][]string     // tag -> paths
	extIndex      map[string][]string     // ext -> paths
	typeIndex     map[string][]string     // fileType -> paths
	stopWords     map[string]bool
	running       bool
	ctx           context.Context
	cancel        context.CancelFunc
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query       string    `json:"query"`       // 搜索关键词
	Path        string    `json:"path"`        // 路径限制
	FileType    string    `json:"fileType"`    // 文件类型过滤
	Extensions  []string  `json:"extensions"`  // 扩展名过滤
	Tags        []string  `json:"tags"`        // 标签过滤
	MinSize     int64     `json:"minSize"`     // 最小大小
	MaxSize     int64     `json:"maxSize"`     // 最大大小
	FromDate    *time.Time `json:"fromDate"`   // 修改时间起始
	ToDate      *time.Time `json:"toDate"`     // 修改时间结束
	Content     bool      `json:"content"`     // 是否搜索内容
	Fuzzy       bool      `json:"fuzzy"`       // 模糊搜索
	Highlight   bool      `json:"highlight"`   // 高亮匹配
	ExactMatch  bool      `json:"exactMatch"`  // 精确匹配
	CaseSense   bool      `json:"caseSense"`   // 大小写敏感
	MaxResults  int       `json:"maxResults"`  // 最大结果数
	Offset      int       `json:"offset"`      // 偏移量
	SortBy      string    `json:"sortBy"`      // 排序字段 (relevance, name, size, date)
	SortDesc    bool      `json:"sortDesc"`    // 降序排序
}

// SearchResult 搜索结果
type SearchResult struct {
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Ext         string            `json:"ext"`
	Size        int64             `json:"size"`
	ModTime     time.Time         `json:"modTime"`
	IsDir       bool              `json:"isDir"`
	FileType    string            `json:"fileType"`
	ContentType string            `json:"contentType"`
	Score       float64           `json:"score"`
	MatchCount  int               `json:"matchCount"`
	Excerpt     string            `json:"excerpt,omitempty"`
	Highlights  []SearchHighlight `json:"highlights,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Thumbnail   string            `json:"thumbnail,omitempty"`
}

// SearchHighlight 搜索高亮
type SearchHighlight struct {
	Field    string   `json:"field"`    // 匹配字段 (name, content, tags)
	Fragments []string `json:"fragments"` // 高亮片段
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Query       string         `json:"query"`
	Total       int            `json:"total"`
	Results     []SearchResult `json:"results"`
	Offset      int            `json:"offset"`
	Limit       int            `json:"limit"`
	Truncated   bool           `json:"truncated"`
	Suggestions []string       `json:"suggestions,omitempty"`
	Facets      SearchFacets   `json:"facets"`
	Took        time.Duration  `json:"took"`
}

// SearchFacets 搜索分类统计
type SearchFacets struct {
	FileTypes map[string]int `json:"fileTypes"`
	Extensions map[string]int `json:"extensions"`
	Tags      map[string]int `json:"tags"`
}

// SuggestRequest 搜索建议请求
type SuggestRequest struct {
	Query  string `json:"query"`  // 前缀
	Path   string `json:"path"`   // 路径限制
	Limit  int    `json:"limit"`  // 最大结果数
}

// SuggestResponse 搜索建议响应
type SuggestResponse struct {
	Query       string   `json:"query"`
	Suggestions []string `json:"suggestions"`
}

// NewSearchEngine 创建搜索引擎
func NewSearchEngine(config WebShareConfig, logger *zap.Logger) *SearchEngine {
	ctx, cancel := context.WithCancel(context.Background())

	se := &SearchEngine{
		config:         config,
		logger:         logger,
		nameIndex:      make(map[string][]string),
		contentTokens:  make(map[string][]string),
		tagIndex:       make(map[string][]string),
		extIndex:       make(map[string][]string),
		typeIndex:      make(map[string][]string),
		stopWords:      make(map[string]bool),
		ctx:            ctx,
		cancel:         cancel,
	}

	// 初始化停用词
	se.initStopWords()

	// 创建内容索引器
	se.contentIndexer = NewContentIndexer(config, logger)

	return se
}

// initStopWords 初始化停用词
func (se *SearchEngine) initStopWords() {
	// 中文停用词
	zhStopWords := []string{
		"的", "是", "在", "有", "和", "了", "不", "这", "那", "之",
		"为", "与", "以", "及", "其", "或", "但", "如", "而", "也",
		"就", "都", "会", "能", "要", "对", "没", "从", "到", "被",
		"把", "让", "向", "又", "已", "于", "由", "因为", "所以",
	}

	// 英文停用词
	enStopWords := []string{
		"the", "a", "an", "is", "are", "was", "were", "be", "been",
		"being", "have", "has", "had", "do", "does", "did", "will",
		"would", "could", "should", "may", "might", "must", "shall",
		"can", "need", "to", "of", "in", "for", "on", "with", "at",
		"by", "from", "as", "into", "through", "during", "before",
		"after", "above", "below", "between", "under", "again",
		"further", "then", "once", "here", "there", "when", "where",
		"why", "how", "all", "each", "few", "more", "most", "other",
		"some", "such", "no", "nor", "not", "only", "own", "same",
		"so", "than", "too", "very", "just", "and", "but", "if",
		"or", "because", "until", "while", "although", "though",
	}

	for _, w := range zhStopWords {
		se.stopWords[w] = true
	}
	for _, w := range enStopWords {
		se.stopWords[w] = true
	}
}

// Start 启动搜索引擎
func (se *SearchEngine) Start() {
	se.mu.Lock()
	se.running = true
	se.mu.Unlock()

	se.contentIndexer.Start()
	se.logger.Info("搜索引擎启动")
}

// Stop 停止搜索引擎
func (se *SearchEngine) Stop() {
	se.cancel()
	se.contentIndexer.Stop()
	se.mu.Lock()
	se.running = false
	se.mu.Unlock()

	se.logger.Info("搜索引擎停止")
}

// BuildIndex 构建索引
func (se *SearchEngine) BuildIndex(ctx context.Context, basePath string) error {
	if basePath == "" {
		basePath = se.config.BaseDir
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return err
	}

	se.logger.Info("开始构建搜索索引", zap.String("path", absBase))

	// 使用内容索引器构建索引
	req := IndexRequest{
		Path:        absBase,
		Recursive:   true,
		ForceReindex: false,
	}

	resp, err := se.contentIndexer.Index(ctx, req)
	if err != nil {
		return err
	}

	// 重建搜索索引
	se.rebuildSearchIndex()

	se.logger.Info("搜索索引构建完成",
		zap.Int64("indexed", resp.IndexedFiles),
		zap.Duration("took", resp.Took),
	)

	return nil
}

// rebuildSearchIndex 重建搜索索引
func (se *SearchEngine) rebuildSearchIndex() {
	se.mu.Lock()
	defer se.mu.Unlock()

	// 清除旧索引
	se.nameIndex = make(map[string][]string)
	se.contentTokens = make(map[string][]string)
	se.tagIndex = make(map[string][]string)
	se.extIndex = make(map[string][]string)
	se.typeIndex = make(map[string][]string)

	// 从内容索引器获取所有元数据
	allMeta := se.contentIndexer.GetAllMetadata()

	for path, meta := range allMeta {
		// 文件名索引
		nameTokens := se.tokenize(meta.Name)
		for _, token := range nameTokens {
			se.nameIndex[token] = append(se.nameIndex[token], path)
		}

		// 内容索引
		if meta.TextContent != "" {
			contentTokens := se.tokenizeContent(meta.TextContent)
			for _, token := range contentTokens {
				se.contentTokens[token] = append(se.contentTokens[token], path)
			}
		}

		// 扩展名索引
		if meta.Ext != "" {
			se.extIndex[meta.Ext] = append(se.extIndex[meta.Ext], path)
		}

		// 文件类型索引
		se.typeIndex[meta.FileType] = append(se.typeIndex[meta.FileType], path)

		// 标签索引
		for _, tag := range meta.Tags {
			se.tagIndex[tag] = append(se.tagIndex[tag], path)
		}
	}
}

// Search 执行搜索
func (se *SearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	startTime := time.Now()

	if req.MaxResults == 0 {
		req.MaxResults = 50
	}

	response := &SearchResponse{
		Query:   req.Query,
		Offset:  req.Offset,
		Limit:   req.MaxResults,
		Facets: SearchFacets{
			FileTypes:  make(map[string]int),
			Extensions: make(map[string]int),
			Tags:       make(map[string]int),
		},
	}

	// 搜索匹配的路径
	matchedPaths := se.matchPaths(req)

	// 构建结果
	results := make([]SearchResult, 0)
	for _, path := range matchedPaths {
		meta := se.contentIndexer.GetMetadata(path)
		if meta == nil {
			continue
		}

		// 应用过滤器
		if !se.applyFilters(meta, req) {
			continue
		}

		// 计算分数
		score := se.calculateScore(meta, req)

		// 构建结果
		result := SearchResult{
			Path:        meta.Path,
			Name:        meta.Name,
			Ext:         meta.Ext,
			Size:        meta.Size,
			ModTime:     meta.ModTime,
			FileType:    meta.FileType,
			ContentType: meta.ContentType,
			Score:       score,
			Tags:        meta.Tags,
		}

		// 高亮
		if req.Highlight && req.Query != "" {
			result.Highlights = se.highlightMatches(meta, req.Query, req.CaseSense)
			result.Excerpt = se.generateExcerpt(meta.TextContent, req.Query, req.CaseSense)
			result.MatchCount = se.countMatches(meta, req.Query, req.CaseSense)
		}

		results = append(results, result)

		// 更新分类统计
		response.Facets.FileTypes[meta.FileType]++
		response.Facets.Extensions[meta.Ext]++
		for _, tag := range meta.Tags {
			response.Facets.Tags[tag]++
		}
	}

	// 排序
	se.sortResults(results, req.SortBy, req.SortDesc)

	// 截断
	response.Total = len(results)
	if len(results) > req.MaxResults+req.Offset {
		if req.Offset < len(results) {
			end := req.Offset + req.MaxResults
			if end > len(results) {
				end = len(results)
			}
			response.Results = results[req.Offset:end]
		}
		response.Truncated = true
	} else if req.Offset < len(results) {
		response.Results = results[req.Offset:]
	}

	// 生成建议
	if req.Query != "" {
		response.Suggestions = se.getSuggestions(req.Query)
	}

	response.Took = time.Since(startTime)

	return response, nil
}

// matchPaths 匹配路径
func (se *SearchEngine) matchPaths(req SearchRequest) []string {
	se.mu.RLock()
	defer se.mu.RUnlock()

	if req.Query == "" {
		// 无查询，返回所有
		paths := make([]string, 0)
		seen := make(map[string]bool)
		for _, p := range se.nameIndex {
			for _, path := range p {
				if !seen[path] {
					paths = append(paths, path)
					seen[path] = true
				}
			}
		}
		return paths
	}

	query := req.Query
	if !req.CaseSense {
		query = strings.ToLower(query)
	}

	// 分词查询
	queryTokens := se.tokenize(query)

	matchedPaths := make(map[string]float64)

	// 文件名匹配
	for _, token := range queryTokens {
		if paths, ok := se.nameIndex[token]; ok {
			for _, path := range paths {
				matchedPaths[path] += 10.0 // 文件名匹配权重高
			}
		}

		// 模糊匹配文件名
		if req.Fuzzy {
			for indexToken, paths := range se.nameIndex {
				if strings.Contains(indexToken, token) || strings.Contains(token, indexToken) {
					for _, path := range paths {
						matchedPaths[path] += 5.0
					}
				}
			}
		}
	}

	// 内容匹配
	if req.Content {
		for _, token := range queryTokens {
			if paths, ok := se.contentTokens[token]; ok {
				for _, path := range paths {
					matchedPaths[path] += 3.0 // 内容匹配权重
				}
			}
		}
	}

	// 标签匹配
	for _, token := range queryTokens {
		for tag, paths := range se.tagIndex {
			if strings.Contains(strings.ToLower(tag), token) {
				for _, path := range paths {
					matchedPaths[path] += 8.0 // 标签匹配权重
				}
			}
		}
	}

	// 按分数排序
	type pathScore struct {
		path  string
		score float64
	}
	scored := make([]pathScore, 0, len(matchedPaths))
	for path, score := range matchedPaths {
		scored = append(scored, pathScore{path, score})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	paths := make([]string, len(scored))
	for i, ps := range scored {
		paths[i] = ps.path
	}

	return paths
}

// applyFilters 应用过滤器
func (se *SearchEngine) applyFilters(meta *FileMetadata, req SearchRequest) bool {
	// 路径过滤
	if req.Path != "" && !strings.HasPrefix(meta.Path, req.Path) {
		return false
	}

	// 文件类型过滤
	if req.FileType != "" && meta.FileType != req.FileType {
		return false
	}

	// 扩展名过滤
	if len(req.Extensions) > 0 {
		matched := false
		for _, ext := range req.Extensions {
			if meta.Ext == strings.ToLower(ext) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 标签过滤
	if len(req.Tags) > 0 {
		matched := false
		for _, tag := range req.Tags {
			for _, metaTag := range meta.Tags {
				if metaTag == tag {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 大小过滤
	if req.MinSize > 0 && meta.Size < req.MinSize {
		return false
	}
	if req.MaxSize > 0 && meta.Size > req.MaxSize {
		return false
	}

	// 时间过滤
	if req.FromDate != nil && meta.ModTime.Before(*req.FromDate) {
		return false
	}
	if req.ToDate != nil && meta.ModTime.After(*req.ToDate) {
		return false
	}

	return true
}

// calculateScore 计算匹配分数
func (se *SearchEngine) calculateScore(meta *FileMetadata, req SearchRequest) float64 {
	score := 0.0

	query := req.Query
	if !req.CaseSense {
		query = strings.ToLower(query)
	}

	// 文件名匹配
	name := meta.Name
	if !req.CaseSense {
		name = strings.ToLower(name)
	}
	if strings.Contains(name, query) {
		score += 50.0
	}
	if name == query {
		score += 100.0 // 精确匹配
	}

	// 内容匹配
	if meta.TextContent != "" {
		content := meta.TextContent
		if !req.CaseSense {
			content = strings.ToLower(content)
		}
		count := strings.Count(content, query)
		score += float64(count) * 2.0
	}

	// 标签匹配
	for _, tag := range meta.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			score += 30.0
		}
	}

	// 关键词匹配
	for _, kw := range meta.Keywords {
		if strings.Contains(strings.ToLower(kw), query) {
			score += 10.0
		}
	}

	return score
}

// highlightMatches 生成高亮匹配
func (se *SearchEngine) highlightMatches(meta *FileMetadata, query string, caseSense bool) []SearchHighlight {
	highlights := make([]SearchHighlight, 0)

	queryLower := strings.ToLower(query)

	// 文件名高亮
	name := meta.Name
	if !caseSense {
		name = strings.ToLower(name)
	}
	if strings.Contains(name, queryLower) {
		highlights = append(highlights, SearchHighlight{
			Field:     "name",
			Fragments: []string{se.highlightText(meta.Name, query, caseSense)},
		})
	}

	// 内容高亮
	if meta.TextContent != "" {
		content := meta.TextContent
		if !caseSense {
			content = strings.ToLower(content)
		}

		fragments := make([]string, 0)
		pos := 0
		for i := 0; i < 5; i++ { // 最多5个片段
			idx := strings.Index(content[pos:], queryLower)
			if idx == -1 {
				break
			}

			matchPos := pos + idx
			start := matchPos - 50
			if start < 0 {
				start = 0
			}
			end := matchPos + len(query) + 50
			if end > len(meta.TextContent) {
				end = len(meta.TextContent)
			}

			fragment := meta.TextContent[start:end]
			fragments = append(fragments, "..."+se.highlightText(fragment, query, caseSense)+"...")
			pos = matchPos + len(query)
		}

		if len(fragments) > 0 {
			highlights = append(highlights, SearchHighlight{
				Field:     "content",
				Fragments: fragments,
			})
		}
	}

	// 标签高亮
	for _, tag := range meta.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			highlights = append(highlights, SearchHighlight{
				Field:     "tags",
				Fragments: []string{se.highlightText(tag, query, caseSense)},
			})
		}
	}

	return highlights
}

// highlightText 高亮文本
func (se *SearchEngine) highlightText(text, query string, caseSense bool) string {
	if !caseSense {
		// 不区分大小写的替换
		lower := strings.ToLower(text)
		queryLower := strings.ToLower(query)

		result := text
		offset := 0
		pos := 0
		for {
			idx := strings.Index(lower[pos:], queryLower)
			if idx == -1 {
				break
			}

			actualPos := pos + idx
			// 插入高亮标记
			before := result[:actualPos+offset]
			match := result[actualPos+offset : actualPos+offset+len(query)]
			after := result[actualPos+offset+len(query):]

			result = before + "**" + match + "**" + after
			offset += 4 // ** 和 **
			pos = actualPos + len(query)
		}

		return result
	}

	// 区分大小写的替换
	return strings.ReplaceAll(text, query, "**"+query+"**")
}

// generateExcerpt 生成摘要
func (se *SearchEngine) generateExcerpt(content, query string, caseSense bool) string {
	if content == "" || query == "" {
		return ""
	}

	searchContent := content
	searchQuery := query
	if !caseSense {
		searchContent = strings.ToLower(content)
		searchQuery = strings.ToLower(query)
	}

	idx := strings.Index(searchContent, searchQuery)
	if idx == -1 {
		// 返回前200字符
		if len(content) > 200 {
			return content[:200] + "..."
		}
		return content
	}

	// 提取匹配周围的文本
	start := idx - 100
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 100
	if end > len(content) {
		end = len(content)
	}

	excerpt := content[start:end]
	if start > 0 {
		excerpt = "..." + excerpt
	}
	if end < len(content) {
		excerpt = excerpt + "..."
	}

	return excerpt
}

// countMatches 计算匹配次数
func (se *SearchEngine) countMatches(meta *FileMetadata, query string, caseSense bool) int {
	count := 0

	name := meta.Name
	content := meta.TextContent
	if !caseSense {
		name = strings.ToLower(name)
		content = strings.ToLower(content)
		query = strings.ToLower(query)
	}

	count += strings.Count(name, query)
	count += strings.Count(content, query)

	return count
}

// sortResults 排序结果
func (se *SearchEngine) sortResults(results []SearchResult, sortBy string, desc bool) {
	switch sortBy {
	case "name":
		sort.Slice(results, func(i, j int) bool {
			if desc {
				return results[i].Name > results[j].Name
			}
			return results[i].Name < results[j].Name
		})
	case "size":
		sort.Slice(results, func(i, j int) bool {
			if desc {
				return results[i].Size > results[j].Size
			}
			return results[i].Size < results[j].Size
		})
	case "date":
		sort.Slice(results, func(i, j int) bool {
			if desc {
				return results[i].ModTime.After(results[j].ModTime)
			}
			return results[i].ModTime.Before(results[j].ModTime)
		})
	default: // relevance
		sort.Slice(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score > results[j].Score
			}
			// 分数相同时按路径排序确保稳定性
			return results[i].Path < results[j].Path
		})
	}
}

// getSuggestions 获取搜索建议
func (se *SearchEngine) getSuggestions(query string) []string {
	se.mu.RLock()
	defer se.mu.RUnlock()

	suggestions := make([]string, 0)
	seen := make(map[string]bool)

	queryLower := strings.ToLower(query)

	// 从文件名索引中查找
	for token := range se.nameIndex {
		if strings.HasPrefix(token, queryLower) && token != queryLower {
			if !seen[token] {
				suggestions = append(suggestions, token)
				seen[token] = true
			}
		}
		if len(suggestions) >= 10 {
			break
		}
	}

	// 从内容索引中查找
	if len(suggestions) < 10 {
		for token := range se.contentTokens {
			if strings.HasPrefix(token, queryLower) && token != queryLower {
				if !seen[token] {
					suggestions = append(suggestions, token)
					seen[token] = true
				}
			}
			if len(suggestions) >= 10 {
				break
			}
		}
	}

	return suggestions
}

// tokenize 分词
func (se *SearchEngine) tokenize(text string) []string {
	// 移除扩展名
	text = strings.TrimSuffix(text, filepath.Ext(text))

	// 转小写
	text = strings.ToLower(text)

	// 按分隔符分割
	words := strings.FieldsFunc(text, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c) && !unicode.Is(unicode.Han, c)
	})

	// 过滤停用词和短词
	result := make([]string, 0)
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) >= 1 && !se.stopWords[w] {
			result = append(result, w)
		}
	}

	// 中文分词（简单的二元分词）
	chineseWords := make([]string, 0)
	for _, w := range words {
		runes := []rune(w)
		isChinese := false
		for _, r := range runes {
			if unicode.Is(unicode.Han, r) {
				isChinese = true
				break
			}
		}

		if isChinese && len(runes) > 1 {
			// 二元分词
			for i := 0; i < len(runes)-1; i++ {
				chineseWords = append(chineseWords, string(runes[i:i+2]))
			}
			// 也添加完整词
			chineseWords = append(chineseWords, w)
		}
	}

	result = append(result, chineseWords...)

	return result
}

// tokenizeContent 内容分词
func (se *SearchEngine) tokenizeContent(content string) []string {
	// 限制内容大小
	if len(content) > 100000 {
		content = content[:100000]
	}

	tokens := se.tokenize(content)

	// 去重
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, t := range tokens {
		if !seen[t] {
			seen[t] = true
			unique = append(unique, t)
		}
	}

	return unique
}

// UpdateFileIndex 更新文件索引
func (se *SearchEngine) UpdateFileIndex(ctx context.Context, path string) error {
	// 使用内容索引器更新
	req := IndexRequest{
		Path:         path,
		Recursive:    false,
		ForceReindex: true,
	}

	_, err := se.contentIndexer.Index(ctx, req)
	if err != nil {
		return err
	}

	// 重建搜索索引
	se.rebuildSearchIndex()

	return nil
}

// RemoveFileIndex 移除文件索引
func (se *SearchEngine) RemoveFileIndex(path string) {
	se.mu.Lock()
	defer se.mu.Unlock()

	// 从各索引中移除
	for token, paths := range se.nameIndex {
		for i, p := range paths {
			if p == path {
				se.nameIndex[token] = append(paths[:i], paths[i+1:]...)
				break
			}
		}
	}

	for token, paths := range se.contentTokens {
		for i, p := range paths {
			if p == path {
				se.contentTokens[token] = append(paths[:i], paths[i+1:]...)
				break
			}
		}
	}

	for ext, paths := range se.extIndex {
		for i, p := range paths {
			if p == path {
				se.extIndex[ext] = append(paths[:i], paths[i+1:]...)
				break
			}
		}
	}

	for ft, paths := range se.typeIndex {
		for i, p := range paths {
			if p == path {
				se.typeIndex[ft] = append(paths[:i], paths[i+1:]...)
				break
			}
		}
	}

	for tag, paths := range se.tagIndex {
		for i, p := range paths {
			if p == path {
				se.tagIndex[tag] = append(paths[:i], paths[i+1:]...)
				break
			}
		}
	}
}

// GetStats 获取统计信息
func (se *SearchEngine) GetStats() map[string]interface{} {
	se.mu.RLock()
	defer se.mu.RUnlock()

	indexerStats := se.contentIndexer.GetStats()

	return map[string]interface{}{
		"totalFiles":      indexerStats.IndexedFiles,
		"totalBytes":      indexerStats.IndexedBytes,
		"nameIndexSize":   len(se.nameIndex),
		"contentIndexSize": len(se.contentTokens),
		"tagIndexSize":    len(se.tagIndex),
		"extIndexSize":    len(se.extIndex),
		"typeIndexSize":   len(se.typeIndex),
		"lastIndexed":     indexerStats.LastIndexed,
		"filesByType":     indexerStats.FilesByType,
	}
}

// GetContentIndexer 获取内容索引器
func (se *SearchEngine) GetContentIndexer() *ContentIndexer {
	return se.contentIndexer
}
