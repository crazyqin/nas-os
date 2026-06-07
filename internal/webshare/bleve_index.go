// Package webshare Bleve全文索引增强模块
// 提供基于Bleve的高性能全文检索能力
package webshare

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"go.uber.org/zap"
)

// BleveContentIndex Bleve全文索引服务
// 提供文件内容全文检索能力，支持中文分词
type BleveContentIndex struct {
	config    WebShareConfig
	logger    *zap.Logger
	index     bleve.Index
	mu        sync.RWMutex
	indexPath string
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
	stats     BleveIndexStats
}

// BleveDocument Bleve索引文档
type BleveDocument struct {
	Path        string    `json:"path"`        // 文件路径
	Name        string    `json:"name"`        // 文件名
	Ext         string    `json:"ext"`         // 扩展名
	Content     string    `json:"content"`     // 文件内容
	Size        int64     `json:"size"`        // 文件大小
	ModTime     time.Time `json:"modTime"`     // 修改时间
	ContentType string    `json:"contentType"` // 内容类型
	Language    string    `json:"language"`    // 语言
	Keywords    []string  `json:"keywords"`    // 关键词
	Excerpt     string    `json:"excerpt"`     // 摘要
	WordCount   int       `json:"wordCount"`   // 词数
	LineCount   int       `json:"lineCount"`   // 行数
}

// BleveSearchRequest Bleve搜索请求
type BleveSearchRequest struct {
	Query      string     `json:"query"`      // 搜索关键词
	Paths      []string   `json:"paths"`      // 路径限制
	Extensions []string   `json:"extensions"` // 扩展名过滤
	MinSize    int64      `json:"minSize"`    // 最小大小
	MaxSize    int64      `json:"maxSize"`    // 最大大小
	FromDate   *time.Time `json:"fromDate"`   // 时间起始
	ToDate     *time.Time `json:"toDate"`     // 时间结束
	MaxResults int        `json:"maxResults"` // 最大结果
	Offset     int        `json:"offset"`     // 偏移量
	Highlight  bool       `json:"highlight"`  // 高亮
	Fuzzy      bool       `json:"fuzzy"`      // 模糊搜索
	ExactMatch bool       `json:"exactMatch"` // 精确匹配
	CaseSense  bool       `json:"caseSense"`  // 大小写敏感
	SortBy     string     `json:"sortBy"`     // 排序字段
	SortDesc   bool       `json:"sortDesc"`   // 降序排序
	Fields     []string   `json:"fields"`     // 搜索字段
}

// BleveSearchResult Bleve搜索结果
type BleveSearchResult struct {
	Path        string              `json:"path"`
	Name        string              `json:"name"`
	Ext         string              `json:"ext"`
	Size        int64               `json:"size"`
	ModTime     time.Time           `json:"modTime"`
	Score       float64             `json:"score"`
	Content     string              `json:"content,omitempty"`
	Excerpt     string              `json:"excerpt"`
	Highlights  map[string][]string `json:"highlights,omitempty"`
	Keywords    []string            `json:"keywords"`
	ContentType string              `json:"contentType"`
	Language    string              `json:"language"`
	WordCount   int                 `json:"wordCount"`
}

// BleveSearchResponse Bleve搜索响应
type BleveSearchResponse struct {
	Query       string              `json:"query"`
	Took        time.Duration       `json:"took"`
	Total       int64               `json:"total"`
	Results     []BleveSearchResult `json:"results"`
	Offset      int                 `json:"offset"`
	Limit       int                 `json:"limit"`
	Truncated   bool                `json:"truncated"`
	Suggestions []string            `json:"suggestions"`
	Facets      map[string]int      `json:"facets"`
	Stats       BleveIndexStats     `json:"stats"`
}

// BleveIndexStats Bleve索引统计
type BleveIndexStats struct {
	TotalDocuments int64         `json:"totalDocuments"`
	IndexSize      int64         `json:"indexSize"`
	LastIndexed    time.Time     `json:"lastIndexed"`
	IndexDuration  time.Duration `json:"indexDuration"`
	IndexedBytes   int64         `json:"indexedBytes"`
	IndexedFiles   int64         `json:"indexedFiles"`
	AvgFileSize    int64         `json:"avgFileSize"`
	TotalWordCount int64         `json:"totalWordCount"`
	TotalLineCount int64         `json:"totalLineCount"`
}

// NewBleveContentIndex 创建Bleve全文索引
func NewBleveContentIndex(config WebShareConfig, logger *zap.Logger) (*BleveContentIndex, error) {
	ctx, cancel := context.WithCancel(context.Background())

	indexPath := filepath.Join(config.BaseDir, ".bleve_index")

	bci := &BleveContentIndex{
		config:    config,
		logger:    logger,
		indexPath: indexPath,
		ctx:       ctx,
		cancel:    cancel,
		stats:     BleveIndexStats{},
	}

	// 初始化或打开索引
	if err := bci.initIndex(); err != nil {
		return nil, fmt.Errorf("初始化Bleve索引失败: %w", err)
	}

	return bci, nil
}

// initIndex 初始化Bleve索引
func (bci *BleveContentIndex) initIndex() error {
	// 创建索引映射
	indexMapping := bci.createIndexMapping()

	// 检查索引是否存在
	if _, err := os.Stat(bci.indexPath); err == nil {
		// 打开现有索引
		index, err := bleve.Open(bci.indexPath)
		if err != nil {
			// 索引损坏，重建
			bci.logger.Warn("索引损坏，重建索引", zap.Error(err))
			os.RemoveAll(bci.indexPath)
			index, err = bleve.New(bci.indexPath, indexMapping)
			if err != nil {
				return fmt.Errorf("重建索引失败: %w", err)
			}
		}
		bci.index = index
	} else {
		// 创建新索引
		index, err := bleve.New(bci.indexPath, indexMapping)
		if err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
		bci.index = index
	}

	return nil
}

// createIndexMapping 创建索引映射
func (bci *BleveContentIndex) createIndexMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()

	// 创建文档映射
	docMapping := bleve.NewDocumentMapping()

	// 路径字段 - keyword analyzer
	pathFieldMapping := bleve.NewTextFieldMapping()
	pathFieldMapping.Analyzer = keyword.Name
	docMapping.AddFieldMappingsAt("path", pathFieldMapping)

	// 名称字段
	nameFieldMapping := bleve.NewTextFieldMapping()
	docMapping.AddFieldMappingsAt("name", nameFieldMapping)

	// 扩展名字段
	extFieldMapping := bleve.NewTextFieldMapping()
	extFieldMapping.Analyzer = keyword.Name
	docMapping.AddFieldMappingsAt("ext", extFieldMapping)

	// 内容字段 - 支持中文分词
	contentFieldMapping := bleve.NewTextFieldMapping()
	// 使用bleve内置的CJK分词器支持中日韩文字
	contentFieldMapping.Analyzer = "cjk"
	docMapping.AddFieldMappingsAt("content", contentFieldMapping)

	// 关键词字段
	keywordsFieldMapping := bleve.NewTextFieldMapping()
	docMapping.AddFieldMappingsAt("keywords", keywordsFieldMapping)

	// 摘要字段
	excerptFieldMapping := bleve.NewTextFieldMapping()
	docMapping.AddFieldMappingsAt("excerpt", excerptFieldMapping)

	// 数值字段
	sizeFieldMapping := bleve.NewNumericFieldMapping()
	docMapping.AddFieldMappingsAt("size", sizeFieldMapping)

	wordCountFieldMapping := bleve.NewNumericFieldMapping()
	docMapping.AddFieldMappingsAt("wordCount", wordCountFieldMapping)

	// 日期字段
	modTimeFieldMapping := bleve.NewDateTimeFieldMapping()
	docMapping.AddFieldMappingsAt("modTime", modTimeFieldMapping)

	// 内容类型字段
	contentTypeFieldMapping := bleve.NewTextFieldMapping()
	contentTypeFieldMapping.Analyzer = keyword.Name
	docMapping.AddFieldMappingsAt("contentType", contentTypeFieldMapping)

	// 语言字段
	languageFieldMapping := bleve.NewTextFieldMapping()
	languageFieldMapping.Analyzer = keyword.Name
	docMapping.AddFieldMappingsAt("language", languageFieldMapping)

	// 添加文档映射
	indexMapping.AddDocumentMapping("BleveDocument", docMapping)

	// 默认映射
	indexMapping.DefaultMapping = docMapping

	return indexMapping
}

// Start 启动索引服务
func (bci *BleveContentIndex) Start() {
	bci.mu.Lock()
	bci.running = true
	bci.mu.Unlock()

	// 启动后台索引构建
	go bci.backgroundIndexer()

	bci.logger.Info("Bleve全文索引服务已启动", zap.String("indexPath", bci.indexPath))
}

// Stop 停止索引服务
func (bci *BleveContentIndex) Stop() {
	bci.cancel()

	bci.mu.Lock()
	bci.running = false
	bci.mu.Unlock()

	if bci.index != nil {
		bci.index.Close()
	}

	bci.logger.Info("Bleve全文索引服务已停止")
}

// backgroundIndexer 后台索引构建
func (bci *BleveContentIndex) backgroundIndexer() {
	// 初始构建
	bci.BuildIndex(bci.config.BaseDir)

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-bci.ctx.Done():
			return
		case <-ticker.C:
			// 增量更新
			bci.refreshIndex()
		}
	}
}

// BuildIndex 构建全文索引
func (bci *BleveContentIndex) BuildIndex(basePath string) error {
	if basePath == "" {
		basePath = bci.config.BaseDir
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return err
	}

	bci.logger.Info("开始构建Bleve全文索引", zap.String("path", absBase))
	startTime := time.Now()

	var filesIndexed int64
	var bytesIndexed int64
	var totalWords int64
	var totalLines int64

	batch := bci.index.NewBatch()
	batchSize := 100

	err = filepath.Walk(absBase, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		// 跳过隐藏文件和目录
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 跳过索引目录本身
		if strings.Contains(path, ".bleve_index") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 只索引文件
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))

		// 只索引文本文件
		if !bci.isTextFile(ext) {
			return nil
		}

		// 限制大文件
		if info.Size() > 10*1024*1024 { // 10MB
			return nil
		}

		// 索引文件
		doc, err := bci.indexFile(path, info, absBase)
		if err != nil {
			return nil
		}

		if doc != nil {
			batch.Index(doc.Path, doc)
			filesIndexed++
			bytesIndexed += info.Size()
			totalWords += int64(doc.WordCount)
			totalLines += int64(doc.LineCount)

			// 批量提交
			if batch.Size() >= batchSize {
				if err := bci.index.Batch(batch); err != nil {
					bci.logger.Error("批量索引失败", zap.Error(err))
				}
				batch = bci.index.NewBatch()
			}
		}

		return nil
	})

	// 提交剩余批次
	if batch.Size() > 0 {
		if err := bci.index.Batch(batch); err != nil {
			bci.logger.Error("最终批次索引失败", zap.Error(err))
		}
	}

	took := time.Since(startTime)

	bci.mu.Lock()
	bci.stats.TotalDocuments = filesIndexed
	bci.stats.IndexedBytes = bytesIndexed
	bci.stats.IndexedFiles = filesIndexed
	bci.stats.TotalWordCount = totalWords
	bci.stats.TotalLineCount = totalLines
	bci.stats.LastIndexed = time.Now()
	bci.stats.IndexDuration = took
	if filesIndexed > 0 {
		bci.stats.AvgFileSize = bytesIndexed / filesIndexed
	}
	bci.mu.Unlock()

	// 获取索引大小
	if info, err := os.Stat(bci.indexPath); err == nil {
		bci.mu.Lock()
		bci.stats.IndexSize = info.Size()
		bci.mu.Unlock()
	}

	bci.logger.Info("Bleve索引构建完成",
		zap.Int64("files", filesIndexed),
		zap.Int64("bytes", bytesIndexed),
		zap.Duration("took", took),
	)

	return nil
}

// indexFile 索引单个文件
func (bci *BleveContentIndex) indexFile(absPath string, info os.FileInfo, baseDir string) (*BleveDocument, error) {
	// 计算相对路径
	relPath, err := filepath.Rel(baseDir, absPath)
	if err != nil {
		return nil, err
	}

	// 读取文件内容
	file, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 限制读取大小
	content, err := io.ReadAll(io.LimitReader(file, 5*1024*1024)) // 最大5MB
	if err != nil {
		return nil, err
	}

	text := string(content)

	// 检测语言
	language := bci.detectLanguage(text)

	// 提取关键词
	keywords := bci.extractKeywords(text, language)

	// 创建摘要
	excerpt := bci.makeExcerpt(text, 300)

	// 统计词数和行数
	wordCount := bci.countWords(text)
	lineCount := strings.Count(text, "\n") + 1

	doc := &BleveDocument{
		Path:        relPath,
		Name:        info.Name(),
		Ext:         strings.ToLower(filepath.Ext(info.Name())),
		Content:     text,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		ContentType: bci.getContentType(filepath.Ext(info.Name())),
		Language:    language,
		Keywords:    keywords,
		Excerpt:     excerpt,
		WordCount:   wordCount,
		LineCount:   lineCount,
	}

	return doc, nil
}

// isTextFile 判断是否为文本文件
func (bci *BleveContentIndex) isTextFile(ext string) bool {
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
		".adoc", ".tex", ".org", ".wiki", ".rst",
	}

	for _, te := range textExts {
		if ext == te {
			return true
		}
	}

	return false
}

// detectLanguage 检测语言
func (bci *BleveContentIndex) detectLanguage(text string) string {
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

// extractKeywords 提取关键词
func (bci *BleveContentIndex) extractKeywords(text, language string) []string {
	// 统计词频
	wordCount := make(map[string]int)

	// 分词处理
	text = strings.ToLower(text)

	// 替换分隔符
	sepChars := []string{",", ".", ";", ":", "!", "?", "\"", "'",
		"(", ")", "[", "]", "{", "}", "<", ">", "/", "\\", "|",
		"@", "#", "$", "%", "^", "&", "*", "=", "+", "-", "_",
		"\n", "\r", "\t"}

	for _, sep := range sepChars {
		text = strings.ReplaceAll(text, sep, " ")
	}

	words := strings.Fields(text)

	// 过滤停用词和短词
	stopWords := bci.getStopWords(language)

	for _, word := range words {
		if len(word) >= 2 && !stopWords[word] {
			wordCount[word]++
		}
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

// getStopWords 获取停用词
func (bci *BleveContentIndex) getStopWords(language string) map[string]bool {
	stopWords := make(map[string]bool)

	// 中文停用词
	zhStopWords := []string{
		"的", "是", "在", "有", "和", "了", "不", "这", "那", "之",
		"为", "与", "以", "及", "其", "或", "但", "如", "而", "也",
		"就", "都", "会", "能", "要", "对", "没", "从", "到", "被",
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
	}

	switch language {
	case "zh":
		for _, word := range zhStopWords {
			stopWords[word] = true
		}
	case "en":
		for _, word := range enStopWords {
			stopWords[word] = true
		}
	default:
		// 同时添加中文和英文停用词
		for _, word := range zhStopWords {
			stopWords[word] = true
		}
		for _, word := range enStopWords {
			stopWords[word] = true
		}
	}

	return stopWords
}

// makeExcerpt 创建摘要
func (bci *BleveContentIndex) makeExcerpt(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// countWords 统计词数
func (bci *BleveContentIndex) countWords(text string) int {
	return len(strings.Fields(text))
}

// getContentType 获取内容类型
func (bci *BleveContentIndex) getContentType(ext string) string {
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

// Search 执行全文搜索
func (bci *BleveContentIndex) Search(ctx context.Context, req BleveSearchRequest) (*BleveSearchResponse, error) {
	startTime := time.Now()

	if req.MaxResults == 0 {
		req.MaxResults = 50
	}

	if len(req.Fields) == 0 {
		req.Fields = []string{"content", "name", "keywords", "excerpt"}
	}

	response := &BleveSearchResponse{
		Query:   req.Query,
		Facets:  make(map[string]int),
		Results: make([]BleveSearchResult, 0),
		Limit:   req.MaxResults,
		Offset:  req.Offset,
	}

	// 构建查询
	query := bci.buildQuery(req)

	// 构建搜索请求
	searchRequest := bleve.NewSearchRequestOptions(query, req.MaxResults, req.Offset, false)

	// 设置搜索字段
	if len(req.Fields) > 0 {
		searchRequest.Fields = req.Fields
	}

	// 设置高亮
	if req.Highlight {
		searchRequest.Highlight = bleve.NewHighlight()
		searchRequest.Highlight.Fields = req.Fields
	}

	// 设置排序
	if req.SortBy != "" {
		searchRequest.SortBy([]string{req.SortBy})
		if req.SortDesc {
			searchRequest.SortBy([]string{"-" + req.SortBy})
		}
	}

	// 执行搜索
	searchResult, err := bci.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	response.Total = int64(searchResult.Total)

	// 解析结果
	for _, hit := range searchResult.Hits {
		result := bci.parseSearchHit(hit, req)

		// 应用过滤条件
		if !bci.filterResult(&result, req) {
			continue
		}

		response.Results = append(response.Results, result)

		// 统计分类
		response.Facets[result.ContentType]++
	}

	response.Took = time.Since(startTime)

	// 获取索引统计
	bci.mu.RLock()
	response.Stats = bci.stats
	bci.mu.RUnlock()

	// 生成搜索建议
	response.Suggestions = bci.getSuggestions(req.Query)

	return response, nil
}

// buildQuery 构建Bleve查询
func (bci *BleveContentIndex) buildQuery(req BleveSearchRequest) query.Query {
	var q query.Query

	if req.ExactMatch {
		// 精确匹配查询
		q = bleve.NewMatchQuery(req.Query)
	} else if req.Fuzzy {
		// 模糊查询
		matchQuery := bleve.NewMatchQuery(req.Query)
		matchQuery.SetFuzziness(2)
		q = matchQuery
	} else {
		// 默认使用复合查询
		conjunctionQuery := bleve.NewConjunctionQuery()

		// 文本匹配
		matchQuery := bleve.NewMatchQuery(req.Query)
		conjunctionQuery.AddQuery(matchQuery)

		// 如果指定了路径限制
		if len(req.Paths) > 0 {
			disjunctionQuery := bleve.NewDisjunctionQuery()
			for _, path := range req.Paths {
				termQuery := bleve.NewTermQuery(path)
				termQuery.SetField("path")
				disjunctionQuery.AddQuery(termQuery)
			}
			conjunctionQuery.AddQuery(disjunctionQuery)
		}

		q = conjunctionQuery
	}

	return q
}

// parseSearchHit 解析搜索结果
func (bci *BleveContentIndex) parseSearchHit(hit *search.DocumentMatch, req BleveSearchRequest) BleveSearchResult {
	result := BleveSearchResult{
		Path:  hit.ID,
		Score: hit.Score,
	}

	// 提取字段值
	if name, ok := hit.Fields["name"].(string); ok {
		result.Name = name
	}
	if ext, ok := hit.Fields["ext"].(string); ok {
		result.Ext = ext
	}
	if content, ok := hit.Fields["content"].(string); ok {
		result.Content = content
	}
	if excerpt, ok := hit.Fields["excerpt"].(string); ok {
		result.Excerpt = excerpt
	}
	if contentType, ok := hit.Fields["contentType"].(string); ok {
		result.ContentType = contentType
	}
	if language, ok := hit.Fields["language"].(string); ok {
		result.Language = language
	}

	// 提取关键词
	if keywords, ok := hit.Fields["keywords"].([]interface{}); ok {
		for _, kw := range keywords {
			if kwStr, ok := kw.(string); ok {
				result.Keywords = append(result.Keywords, kwStr)
			}
		}
	}

	// 提取高亮信息
	if hit.Fragments != nil {
		result.Highlights = make(map[string][]string)
		for field, fragments := range hit.Fragments {
			result.Highlights[field] = fragments
		}
	}

	return result
}

// filterResult 过滤搜索结果
func (bci *BleveContentIndex) filterResult(result *BleveSearchResult, req BleveSearchRequest) bool {
	// 路径过滤
	if len(req.Paths) > 0 {
		matched := false
		for _, path := range req.Paths {
			if strings.HasPrefix(result.Path, path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 扩展名过滤
	if len(req.Extensions) > 0 {
		matched := false
		for _, ext := range req.Extensions {
			if result.Ext == strings.ToLower(ext) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// getSuggestions 获取搜索建议
func (bci *BleveContentIndex) getSuggestions(query string) []string {
	suggestions := make([]string, 0)

	// 使用前缀查询获取相似词
	prefixQuery := bleve.NewPrefixQuery(query)
	searchRequest := bleve.NewSearchRequestOptions(prefixQuery, 10, 0, false)

	searchResult, err := bci.index.Search(searchRequest)
	if err != nil {
		return suggestions
	}

	for _, hit := range searchResult.Hits {
		if hit.ID != query && strings.HasPrefix(hit.ID, query) {
			suggestions = append(suggestions, hit.ID)
		}
	}

	return suggestions
}

// refreshIndex 刷新索引
func (bci *BleveContentIndex) refreshIndex() {
	// 获取当前索引文档数量
	indexCount, err := bci.index.DocCount()
	if err != nil {
		return
	}

	bci.mu.Lock()
	bci.stats.TotalDocuments = int64(indexCount)
	bci.mu.Unlock()
}

// DeleteDocument 删除索引文档
func (bci *BleveContentIndex) DeleteDocument(path string) error {
	return bci.index.Delete(path)
}

// GetStats 获取索引统计
func (bci *BleveContentIndex) GetStats() BleveIndexStats {
	bci.mu.RLock()
	defer bci.mu.RUnlock()
	return bci.stats
}

// RebuildIndex 重建索引
func (bci *BleveContentIndex) RebuildIndex() error {
	bci.mu.Lock()
	defer bci.mu.Unlock()

	// 关闭现有索引
	if bci.index != nil {
		bci.index.Close()
	}

	// 删除索引目录
	os.RemoveAll(bci.indexPath)

	// 重新初始化
	if err := bci.initIndex(); err != nil {
		return err
	}

	// 重新构建
	return bci.BuildIndex(bci.config.BaseDir)
}

// AdvancedSearch 高级搜索（支持复杂查询）
func (bci *BleveContentIndex) AdvancedSearch(ctx context.Context, req BleveSearchRequest) (*BleveSearchResponse, error) {
	// 构建复杂查询
	var queries []query.Query

	// 主查询
	if req.Query != "" {
		matchQuery := bleve.NewMatchQuery(req.Query)
		queries = append(queries, matchQuery)
	}

	// 内容类型过滤
	if len(req.Extensions) > 0 {
		extQuery := bleve.NewDisjunctionQuery()
		for _, ext := range req.Extensions {
			termQuery := bleve.NewTermQuery(strings.ToLower(ext))
			termQuery.SetField("ext")
			extQuery.AddQuery(termQuery)
		}
		queries = append(queries, extQuery)
	}

	// 时间范围过滤
	if req.FromDate != nil || req.ToDate != nil {
		var fromDate, toDate time.Time
		if req.FromDate != nil {
			fromDate = *req.FromDate
		}
		if req.ToDate != nil {
			toDate = *req.ToDate
		}
		dateRangeQuery := bleve.NewDateRangeQuery(fromDate, toDate)
		dateRangeQuery.SetField("modTime")
		queries = append(queries, dateRangeQuery)
	}

	// 大小范围过滤
	if req.MinSize > 0 || req.MaxSize > 0 {
		minSize := float64(req.MinSize)
		maxSize := float64(req.MaxSize)
		sizeRangeQuery := bleve.NewNumericRangeQuery(&minSize, &maxSize)
		sizeRangeQuery.SetField("size")
		queries = append(queries, sizeRangeQuery)
	}

	// 组合查询
	conjunctionQuery := bleve.NewConjunctionQuery(queries...)

	// 构建搜索请求
	searchRequest := bleve.NewSearchRequestOptions(conjunctionQuery, req.MaxResults, req.Offset, false)
	searchRequest.Fields = []string{"path", "name", "ext", "excerpt", "keywords", "contentType", "language"}

	if req.Highlight {
		searchRequest.Highlight = bleve.NewHighlight()
		searchRequest.Highlight.Fields = []string{"content"}
	}

	// 执行搜索
	searchResult, err := bci.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("高级搜索失败: %w", err)
	}

	// 构建响应
	response := &BleveSearchResponse{
		Query:   req.Query,
		Total:   int64(searchResult.Total),
		Limit:   req.MaxResults,
		Offset:  req.Offset,
		Results: make([]BleveSearchResult, 0),
		Facets:  make(map[string]int),
	}

	for _, hit := range searchResult.Hits {
		result := bci.parseSearchHit(hit, req)
		response.Results = append(response.Results, result)
		response.Facets[result.ContentType]++
	}

	return response, nil
}
