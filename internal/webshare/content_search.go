// Package webshare 内容搜索API - 全文检索功能
// 参考: TrueNAS TrueSearch 全文搜索能力
package webshare

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ContentSearchService 内容搜索服务
// 提供文件内容全文检索能力
type ContentSearchService struct {
	config     WebShareConfig
	logger     *zap.Logger
	mu         sync.RWMutex
	contentIdx map[string]*ContentIndex // path -> index
	textIdx    *TextIndex               // 全文索引
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// ContentIndex 内容索引
type ContentIndex struct {
	Path         string            `json:"path"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	ModTime      time.Time         `json:"modTime"`
	ContentType  string            `json:"contentType"`
	TextContent  string            `json:"textContent,omitempty"` // 提取的文本内容
	Keywords     []string          `json:"keywords"`              // 关键词
	WordCount    int               `json:"wordCount"`
	LineCount    int               `json:"lineCount"`
	IndexedAt    time.Time         `json:"indexedAt"`
	Excerpt      string            `json:"excerpt"`    // 文摘（前200字）
	Language     string            `json:"language"`   // 检测的语言
	Encoding     string            `json:"encoding"`   // 文件编码
	Metadata     map[string]string `json:"metadata"`   // 元数据
	HasFullIndex bool              `json:"hasFullIndex"`
}

// TextIndex 全文索引
type TextIndex struct {
	mu         sync.RWMutex
	wordIndex  map[string][]string // word -> paths
	phraseIdx  map[string][]string // phrase -> paths
	pathWords  map[string][]string // path -> words
	stopWords  map[string]bool
	maxContent int64 // 最大索引内容大小
}

// ContentSearchRequest 内容搜索请求
type ContentSearchRequest struct {
	Query       string        `json:"query"`       // 搜索关键词
	Paths       []string      `json:"paths"`       // 搜索路径限制
	Extensions  []string      `json:"extensions"`  // 文件扩展名过滤
	MinSize     int64         `json:"minSize"`     // 最小文件大小
	MaxSize     int64         `json:"maxSize"`     // 最大文件大小
	FromDate    *time.Time    `json:"fromDate"`    // 修改时间起始
	ToDate      *time.Time    `json:"toDate"`      // 修改时间结束
	MaxResults  int           `json:"maxResults"`  // 最大结果数
	Fuzzy       bool          `json:"fuzzy"`       // 模糊搜索
	Highlight   bool          `json:"highlight"`   // 高亮匹配
	ExactMatch  bool          `json:"exactMatch"`  // 精确匹配
	CaseSense   bool          `json:"caseSense"`   // 大小写敏感
	WithContext bool          `json:"withContext"` // 返回上下文
	ContextSize int           `json:"contextSize"` // 上下文大小（字符数）
}

// ContentSearchResult 内容搜索结果
type ContentSearchResult struct {
	Path        string        `json:"path"`
	Name        string        `json:"name"`
	Ext         string        `json:"ext"`
	Size        int64         `json:"size"`
	ModTime     time.Time     `json:"modTime"`
	Score       float64       `json:"score"`
	MatchCount  int           `json:"matchCount"`
	Excerpt     string        `json:"excerpt"`
	Highlights  []Highlight   `json:"highlights"`
	Contexts    []MatchContext `json:"contexts"`
	ContentType string        `json:"contentType"`
}

// Highlight 高亮匹配
type Highlight struct {
	Field    string `json:"field"`    // 匹配字段
	Text     string `json:"text"`     // 高亮文本
	Position int    `json:"position"` // 位置
}

// MatchContext 匹配上下文
type MatchContext struct {
	Before    string `json:"before"`    // 匹配前文本
	Match     string `json:"match"`     // 匹配文本
	After     string `json:"after"`     // 匹配后文本
	LineNum   int    `json:"lineNum"`   // 行号
	StartPos  int    `json:"startPos"`  // 开始位置
	EndPos    int    `json:"endPos"`    // 结束位置
}

// ContentSearchResponse 内容搜索响应
type ContentSearchResponse struct {
	Query       string                 `json:"query"`
	Took        time.Duration          `json:"took"`
	Total       int                    `json:"total"`
	Results     []ContentSearchResult  `json:"results"`
	Truncated   bool                   `json:"truncated"`
	Suggestions []string               `json:"suggestions"`
	Facets      map[string]int         `json:"facets"`
	Stats       ContentSearchStats     `json:"stats"`
}

// ContentSearchStats 内容搜索统计
type ContentSearchStats struct {
	FilesScanned   int     `json:"filesScanned"`
	BytesScanned   int64   `json:"bytesScanned"`
	IndexHitRatio  float64 `json:"indexHitRatio"`
	AverageScore   float64 `json:"averageScore"`
}

// NewContentSearchService 创建内容搜索服务
func NewContentSearchService(config WebShareConfig, logger *zap.Logger) *ContentSearchService {
	ctx, cancel := context.WithCancel(context.Background())
	
	css := &ContentSearchService{
		config:     config,
		logger:     logger,
		contentIdx: make(map[string]*ContentIndex),
		textIdx:    NewTextIndex(10 * 1024 * 1024), // 10MB最大内容
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// 初始化停用词
	css.initStopWords()
	
	return css
}

// NewTextIndex 创建全文索引
func NewTextIndex(maxContent int64) *TextIndex {
	return &TextIndex{
		wordIndex:  make(map[string][]string),
		phraseIdx:  make(map[string][]string),
		pathWords:  make(map[string][]string),
		stopWords:  make(map[string]bool),
		maxContent: maxContent,
	}
}

// initStopWords 初始化停用词
func (css *ContentSearchService) initStopWords() {
	stopWords := []string{
		// 中文停用词
		"的", "是", "在", "有", "和", "了", "不", "这", "那", "之",
		"为", "与", "以", "及", "其", "或", "但", "如", "而", "也",
		// 英文停用词
		"the", "a", "an", "is", "are", "was", "were", "be", "been",
		"being", "have", "has", "had", "do", "does", "did", "will",
		"would", "could", "should", "may", "might", "must", "shall",
		"can", "need", "dare", "ought", "used", "to", "of", "in",
		"for", "on", "with", "at", "by", "from", "as", "into",
		"through", "during", "before", "after", "above", "below",
		"between", "under", "again", "further", "then", "once",
		"here", "there", "when", "where", "why", "how", "all",
		"each", "few", "more", "most", "other", "some", "such",
		"no", "nor", "not", "only", "own", "same", "so", "than",
		"too", "very", "just", "and", "but", "if", "or", "because",
		"until", "while", "although", "though", "after", "before",
		"when", "whenever", "if", "unless", "since", "lest", "that",
		"which", "who", "whom", "whose", "what", "whatever", "whichever",
	}
	
	for _, word := range stopWords {
		css.textIdx.stopWords[strings.ToLower(word)] = true
	}
}

// Start 启动服务
func (css *ContentSearchService) Start() {
	css.mu.Lock()
	css.running = true
	css.mu.Unlock()
	
	// 启动后台索引构建
	go css.backgroundIndexer()
	
	css.logger.Info("Content search service started")
}

// Stop 停止服务
func (css *ContentSearchService) Stop() {
	css.cancel()
	css.mu.Lock()
	css.running = false
	css.mu.Unlock()
	
	css.logger.Info("Content search service stopped")
}

// backgroundIndexer 后台索引构建
func (css *ContentSearchService) backgroundIndexer() {
	// 初始构建
	css.BuildIndex(css.config.BaseDir)
	
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-css.ctx.Done():
			return
		case <-ticker.C:
			// 增量更新索引
			css.refreshIndex()
		}
	}
}

// BuildIndex 构建全文索引
func (css *ContentSearchService) BuildIndex(basePath string) error {
	if basePath == "" {
		basePath = css.config.BaseDir
	}
	
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return err
	}
	
	css.logger.Info("Building content index", zap.String("path", absBase))
	startTime := time.Now()
	
	var filesIndexed int
	var bytesIndexed int64
	
	err = css.walkAndIndex(absBase, "", &filesIndexed, &bytesIndexed)
	if err != nil {
		css.logger.Error("Index build failed", zap.Error(err))
		return err
	}
	
	took := time.Since(startTime)
	css.logger.Info("Content index built",
		zap.Int("files", filesIndexed),
		zap.Int64("bytes", bytesIndexed),
		zap.Duration("took", took),
	)
	
	return nil
}

// walkAndIndex 遍历并索引
func (css *ContentSearchService) walkAndIndex(absPath, relPath string, filesIndexed *int, bytesIndexed *int64) error {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil // 跳过无法访问的目录
	}
	
	for _, entry := entries {
		name := entry.Name()
		childRelPath := filepath.Join(relPath, name)
		childAbsPath := filepath.Join(absPath, name)
		
		// 跳过隐藏文件
		if strings.HasPrefix(name, ".") {
			continue
		}
		
		// 跳过排除路径
		for _, excluded := range css.config.IndexExcluded {
			if strings.HasPrefix(childRelPath, excluded) {
				continue
			}
		}
		
		if entry.IsDir() {
			css.walkAndIndex(childAbsPath, childRelPath, filesIndexed, bytesIndexed)
		} else {
			// 索引文件内容
			info, err := entry.Info()
			if err != nil {
				continue
			}
			
			ext := strings.ToLower(filepath.Ext(name))
			
			// 只索引文本类文件
			if css.isTextFile(ext) {
				if err := css.indexFileContent(childRelPath, childAbsPath, info); err == nil {
					*filesIndexed++
					*bytesIndexed += info.Size()
				}
			}
		}
	}
	
	return nil
}

// isTextFile 判断是否为文本文件
func (css *ContentSearchService) isTextFile(ext string) bool {
	textExts := []string{
		".txt", ".md", ".rst", ".log", ".csv", ".tsv",
		".json", ".yaml", ".yml", ".xml", ".toml", ".ini",
		".conf", ".cfg", ".properties",
		".html", ".htm", ".css", ".js", ".ts", ".vue", ".jsx", ".tsx",
		".py", ".go", ".java", ".c", ".cpp", ".h", ".hpp",
		".rs", ".rb", ".php", ".pl", ".sh", ".bash", ".zsh",
		".sql", ".psql", ".lua", ".swift", ".kt", ".scala",
		".gitignore", ".dockerignore", ".editorconfig",
		".makefile", ".cmake", ".gradle", ".pom",
		".license", ".readme", ".changelog", ".authors",
	}
	
	for _, te := range textExts {
		if ext == te {
			return true
		}
	}
	
	return false
}

// indexFileContent 索引文件内容
func (css *ContentSearchService) indexFileContent(relPath, absPath string, info os.FileInfo) error {
	// 限制大文件
	if info.Size() > css.textIdx.maxContent {
		return nil // 跳过超大文件
	}
	
	// 读取文件内容
	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	
	text := string(content)
	
	// 检测编码（简化版）
	encoding := "utf-8"
	if !css.isValidUTF8(content) {
		encoding = "unknown"
	}
	
	// 检测语言（简化版）
	language := css.detectLanguage(text)
	
	// 提取关键词
	words := css.extractWords(text)
	keywords := css.extractKeywords(words)
	
	// 创建内容索引
	idx := &ContentIndex{
		Path:        relPath,
		Name:        info.Name(),
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		ContentType: css.getContentType(filepath.Ext(info.Name())),
		TextContent: text,
		Keywords:    keywords,
		WordCount:   len(words),
		LineCount:   strings.Count(text, "\n") + 1,
		IndexedAt:   time.Now(),
		Excerpt:     css.makeExcerpt(text, 200),
		Language:    language,
		Encoding:    encoding,
		Metadata:    make(map[string]string),
		HasFullIndex: true,
	}
	
	css.mu.Lock()
	css.contentIdx[relPath] = idx
	css.mu.Unlock()
	
	// 更新全文索引
	css.textIdx.mu.Lock()
	css.textIdx.pathWords[relPath] = words
	for _, word := range words {
		if !css.textIdx.stopWords[word] {
			css.textIdx.wordIndex[word] = append(css.textIdx.wordIndex[word], relPath)
		}
	}
	css.textIdx.mu.Unlock()
	
	return nil
}

// isValidUTF8 检查UTF-8有效性
func (css *ContentSearchService) isValidUTF8(data []byte) bool {
	for i := 0; i < len(data); {
		r := rune(data[i])
		if r < 0x80 {
			i++
			continue
		}
		// 简化检查
		if r >= 0xC0 && r <= 0xDF && i+1 < len(data) {
			i += 2
		} else if r >= 0xE0 && r <= 0xEF && i+2 < len(data) {
			i += 3
		} else if r >= 0xF0 && r <= 0xF7 && i+3 < len(data) {
			i += 4
		} else {
			return false
		}
	}
	return true
}

// detectLanguage 检测语言
func (css *ContentSearchService) detectLanguage(text string) string {
	// 简化语言检测
	chineseCount := 0
	englishCount := 0
	
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			englishCount++
		}
	}
	
	if chineseCount > englishCount {
		return "zh"
	} else if englishCount > 0 {
		return "en"
	}
	return "unknown"
}

// extractWords 提取单词
func (css *ContentSearchService) extractWords(text string) []string {
	// 分词处理
	text = strings.ToLower(text)
	
	// 替换分隔符为空格
	sepChars := []string{",", ".", ";", ":", "!", "?", "\"", "'", 
		"(", ")", "[", "]", "{", "}", "<", ">", "/", "\\", "|",
		"@", "#", "$", "%", "^", "&", "*", "=", "+", "-", "_",
		"\n", "\r", "\t"}
	
	for _, sep := range sepChars {
		text = strings.ReplaceAll(text, sep, " ")
	}
	
	// 分词
	words := strings.Fields(text)
	
	// 过滤短词和停用词
	result := make([]string, 0)
	for _, word := range words {
		if len(word) >= 2 && !css.textIdx.stopWords[word] {
			result = append(result, word)
		}
	}
	
	return result
}

// extractKeywords 提取关键词
func (css *ContentSearchService) extractKeywords(words []string) []string {
	// 统计词频
	wordCount := make(map[string]int)
	for _, word := range words {
		wordCount[word]++
	}
	
	// 选择高频词作为关键词
	keywords := make([]string, 0)
	for word, count := range wordCount {
		if count >= 2 && len(word) >= 3 {
			keywords = append(keywords, word)
		}
		if len(keywords) >= 20 {
			break
		}
	}
	
	return keywords
}

// makeExcerpt 创建摘录
func (css *ContentSearchService) makeExcerpt(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// getContentType 获取内容类型
func (css *ContentSearchService) getContentType(ext string) string {
	ext = strings.ToLower(ext)
	
	contentTypes := map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".yaml": "application/yaml",
		".yml":  "application/yaml",
		".xml":  "application/xml",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".ts":   "application/typescript",
		".py":   "text/x-python",
		".go":   "text/x-go",
		".java": "text/x-java",
		".c":    "text/x-c",
		".cpp":  "text/x-c++",
		".sh":   "text/x-shellscript",
		".sql":  "text/x-sql",
		".log":  "text/x-log",
		".csv":  "text/csv",
	}
	
	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	return "text/plain"
}

// Search 执行内容搜索
func (css *ContentSearchService) Search(ctx context.Context, req ContentSearchRequest) (*ContentSearchResponse, error) {
	startTime := time.Now()
	
	response := &ContentSearchResponse{
		Query:     req.Query,
		Facets:    make(map[string]int),
		Results:   make([]ContentSearchResult, 0),
		Suggestions: make([]string, 0),
	}
	
	if req.MaxResults == 0 {
		req.MaxResults = 50
	}
	
	if req.ContextSize == 0 {
		req.ContextSize = 100
	}
	
	query := req.Query
	if !req.CaseSense {
		query = strings.ToLower(query)
	}
	
	// 搜索索引
	css.textIdx.mu.RLock()
	var matchedPaths []string
	
	if req.ExactMatch {
		// 精确匹配
		if paths, ok := css.textIdx.wordIndex[query]; ok {
			matchedPaths = paths
		}
	} else {
		// 模糊/部分匹配
		for word, paths := range css.textIdx.wordIndex {
			if strings.Contains(word, query) || strings.Contains(query, word) {
				matchedPaths = append(matchedPaths, paths...)
			}
		}
	}
	css.textIdx.mu.RUnlock()
	
	// 去重
	seen := make(map[string]bool)
	uniquePaths := make([]string, 0)
	for _, p := range matchedPaths {
		if !seen[p] {
			seen[p] = true
			uniquePaths = append(uniquePaths, p)
		}
	}
	
	// 过滤和构建结果
	css.mu.RLock()
	stats := ContentSearchStats{}
	
	for _, path := range uniquePaths {
		idx, ok := css.contentIdx[path]
		if !ok {
			continue
		}
		
		// 路径过滤
		if len(req.Paths) > 0 {
			matched := false
			for _, p := range req.Paths {
				if strings.HasPrefix(path, p) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		
		// 扩展名过滤
		if len(req.Extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(idx.Name))
			matched := false
			for _, e := range req.Extensions {
				if ext == strings.ToLower(e) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		
		// 大小过滤
		if req.MinSize > 0 && idx.Size < req.MinSize {
			continue
		}
		if req.MaxSize > 0 && idx.Size > req.MaxSize {
			continue
		}
		
		// 时间过滤
		if req.FromDate != nil && idx.ModTime.Before(*req.FromDate) {
			continue
		}
		if req.ToDate != nil && idx.ModTime.After(*req.ToDate) {
			continue
		}
		
		// 计算分数和匹配
		score := css.calculateScore(query, idx, req.CaseSense)
		matchCount := strings.Count(strings.ToLower(idx.TextContent), query)
		
		result := ContentSearchResult{
			Path:        path,
			Name:        idx.Name,
			Ext:         filepath.Ext(idx.Name),
			Size:        idx.Size,
			ModTime:     idx.ModTime,
			Score:       score,
			MatchCount:  matchCount,
			Excerpt:     idx.Excerpt,
			ContentType: idx.ContentType,
		}
		
		// 高亮和上下文
		if req.Highlight || req.WithContext {
			highlights, contexts := css.extractMatches(idx.TextContent, query, req.CaseSense, req.ContextSize)
			result.Highlights = highlights
			result.Contexts = contexts
		}
		
		response.Results = append(response.Results, result)
		stats.FilesScanned++
		stats.BytesScanned += idx.Size
		
		// 类型统计
		response.Facets[idx.ContentType]++
	}
	css.mu.RUnlock()
	
	// 排序（按分数）
	css.sortResults(response.Results)
	
	// 限制结果数
	if len(response.Results) > req.MaxResults {
		response.Results = response.Results[:req.MaxResults]
		response.Truncated = true
	}
	
	response.Total = len(response.Results)
	response.Took = time.Since(startTime)
	response.Stats = stats
	
	// 计算平均分数
	if len(response.Results) > 0 {
		var totalScore float64
		for _, r := range response.Results {
			totalScore += r.Score
		}
		stats.AverageScore = totalScore / float64(len(response.Results))
	}
	
	// 索引命中率
	stats.IndexHitRatio = float64(stats.FilesScanned) / float64(len(css.contentIdx))
	
	// 生成建议
	response.Suggestions = css.generateSuggestions(query)
	
	return response, nil
}

// calculateScore 计算匹配分数
func (css *ContentSearchService) calculateScore(query string, idx *ContentIndex, caseSense bool) float64 {
	score := 0.0
	
	text := idx.TextContent
	if !caseSense {
		text = strings.ToLower(text)
		query = strings.ToLower(query)
	}
	
	// 出现次数
	count := strings.Count(text, query)
	score += float64(count) * 10
	
	// 标题匹配
	if strings.Contains(strings.ToLower(idx.Name), query) {
		score += 50
	}
	
	// 关键词匹配
	for _, kw := range idx.Keyword {
		if strings.Contains(strings.ToLower(kw), query) {
			score += 20
		}
	}
	
	// 标准化分数
	if score > 100 {
		score = 100
	}
	
	return score
}

// extractMatches 提取匹配位置
func (css *ContentSearchService) extractMatches(text, query string, caseSense bool, contextSize int) ([]Highlight, []MatchContext) {
	highlights := make([]Highlight, 0)
	contexts := make([]MatchContext, 0)
	
	searchText := text
	searchQuery := query
	if !caseSense {
		searchText = strings.ToLower(text)
		searchQuery = strings.ToLower(query)
	}
	
	// 找到所有匹配位置
	pos := 0
	lineNum := 1
	lineStart := 0
	
	for {
		idx := strings.Index(searchText[pos:], searchQuery)
		if idx == -1 {
			break
		}
		
		matchPos := pos + idx
		matchEnd := matchPos + len(query)
		
		// 计算行号
		for i := lineStart; i < matchPos; i++ {
			if text[i] == '\n' {
				lineNum++
				lineStart = i + 1
			}
		}
		
		// 提取上下文
		beforeStart := matchPos - contextSize
		if beforeStart < 0 {
			beforeStart = 0
		}
		afterEnd := matchEnd + contextSize
		if afterEnd > len(text) {
			afterEnd = len(text)
		}
		
		context := MatchContext{
			Before:   text[beforeStart:matchPos],
			Match:    text[matchPos:matchEnd],
			After:    text[matchEnd:afterEnd],
			LineNum:  lineNum,
			StartPos: matchPos,
			EndPos:   matchEnd,
		}
		contexts = append(contexts, context)
		
		highlight := Highlight{
			Field:    "content",
			Text:     text[matchPos:matchEnd],
			Position: matchPos,
		}
		highlights = append(highlights, highlight)
		
		pos = matchEnd
		
		if len(highlights) >= 10 {
			break // 限制高亮数量
		}
	}
	
	return highlights, contexts
}

// sortResults 排序结果
func (css *ContentSearchService) sortResults(results []ContentSearchResult) {
	// 按分数排序（降序）
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// generateSuggestions 生成搜索建议
func (css *ContentSearchService) generateSuggestions(query string) []string {
	suggestions := make([]string, 0)
	
	css.textIdx.mu.RLock()
	defer css.textIdx.mu.RUnlock()
	
	// 找相似的词
	for word := range css.textIdx.wordIndex {
		if strings.HasPrefix(word, query) && word != query {
			suggestions = append(suggestions, word)
		}
		if len(suggestions) >= 5 {
			break
		}
	}
	
	return suggestions
}

// refreshIndex 刷新索引
func (css *ContentSearchService) refreshIndex() {
	css.mu.RLock()
	defer css.mu.RUnlock()
	
	// 检查需要更新的文件
	for path, idx := range css.contentIdx {
		absPath := filepath.Join(css.config.BaseDir, path)
		info, err := os.Stat(absPath)
		if err != nil {
			// 文件已删除，清除索引
			css.clearIndex(path)
			continue
		}
		
		// 检查是否需要更新
		if info.ModTime().After(idx.IndexedAt) {
			go css.indexFileContent(path, absPath, info)
		}
	}
}

// clearIndex 清除索引
func (css *ContentSearchService) clearIndex(path string) {
	css.mu.Lock()
	delete(css.contentIdx, path)
	css.mu.Unlock()
	
	css.textIdx.mu.Lock()
	delete(css.textIdx.pathWords, path)
	for word, paths := range css.textIdx.wordIndex {
		for i, p := range paths {
			if p == path {
				css.textIdx.wordIndex[word] = append(paths[:i], paths[i+1:]...)
				break
			}
		}
	}
	css.textIdx.mu.Unlock()
}

// GetIndexStats 获取索引统计
func (css *ContentSearchService) GetIndexStats() map[string]interface{} {
	css.mu.RLock()
	defer css.mu.RUnlock()
	
	css.textIdx.mu.RLock()
	defer css.textIdx.mu.RUnlock()
	
	var totalSize int64
	var totalWords int
	var totalLines int
	
	for _, idx := range css.contentIdx {
		totalSize += idx.Size
		totalWords += idx.WordCount
		totalLines += idx.LineCount
	}
	
	return map[string]interface{}{
		"indexedFiles":    len(css.contentIdx),
		"indexedBytes":    totalSize,
		"totalWords":      totalWords,
		"totalLines":      totalLines,
		"uniqueWords":     len(css.textIdx.wordIndex),
		"stopWordsCount":  len(css.textIdx.stopWords),
		"maxContentSize":  css.textIdx.maxContent,
	}
}