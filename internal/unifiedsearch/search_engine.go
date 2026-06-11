// Package unifiedsearch 搜索引擎核心实现，使用 bleve 倒排索引实现亚秒级全文搜索。
package unifiedsearch

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"go.uber.org/zap"
)

// EngineSearchType 搜索源类型
type EngineSearchType string

const (
	SourceFile     EngineSearchType = "file"
	SourcePhoto    EngineSearchType = "photo"
	SourceDocument EngineSearchType = "document"
	SourceEmail    EngineSearchType = "email"
	SourceNote     EngineSearchType = "note"
	SourceVideo    EngineSearchType = "video"
	SourceMusic    EngineSearchType = "music"
)

// SearchEngine 统一搜索引擎，基于 bleve 倒排索引
type SearchEngine struct {
	index      bleve.Index
	logger     *zap.Logger
	indexPath  string
	stats      *IndexStats
	stopChan   chan struct{}
	running    bool
	indexReady bool
}

// NewSearchEngine 创建搜索引擎
func NewSearchEngine(logger *zap.Logger, indexDir string) (*SearchEngine, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := &SearchEngine{
		logger:    logger,
		indexPath: indexDir,
		stats:     DefaultIndexStats(),
		stopChan:  make(chan struct{}),
	}

	return engine, nil
}

// createIndex 创建 bleve 索引
func (e *SearchEngine) createIndex() (bleve.Index, error) {
	// 定义映射
	mapping := bleve.NewIndexMapping()

	// 默认文档映射：使用中文分词和标准分析器
	defaultMapping := bleve.NewDocumentMapping()

	// 文件名字段：使用 keyword 分词，支持精确匹配和前缀搜索
	nameFieldMapping := bleve.NewTextFieldMapping()
	nameFieldMapping.Analyzer = "standard"
	defaultMapping.AddFieldMappingsAt("name", nameFieldMapping)

	// 内容字段：使用标准分析器
	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Analyzer = "standard"
	defaultMapping.AddFieldMappingsAt("content", contentFieldMapping)

	// 路径字段：使用 keyword 分析器
	pathFieldMapping := bleve.NewTextFieldMapping()
	pathFieldMapping.Analyzer = "keyword"
	defaultMapping.AddFieldMappingsAt("path", pathFieldMapping)

	// 标签字段：使用标准分析器
	tagFieldMapping := bleve.NewTextFieldMapping()
	tagFieldMapping.Analyzer = "standard"
	defaultMapping.AddFieldMappingsAt("tags", tagFieldMapping)

	// 元数据键值对字段
	metadataFieldMapping := bleve.NewTextFieldMapping()
	metadataFieldMapping.Analyzer = "standard"
	defaultMapping.AddFieldMappingsAt("metadata", metadataFieldMapping)

	// 扩展名字段
	extFieldMapping := bleve.NewTextFieldMapping()
	extFieldMapping.Analyzer = "keyword"
	defaultMapping.AddFieldMappingsAt("extension", extFieldMapping)

	// 内容类型字段
	typeFieldMapping := bleve.NewTextFieldMapping()
	typeFieldMapping.Analyzer = "keyword"
	defaultMapping.AddFieldMappingsAt("content_type", typeFieldMapping)

	// 所有者字段
	ownerFieldMapping := bleve.NewTextFieldMapping()
	ownerFieldMapping.Analyzer = "keyword"
	defaultMapping.AddFieldMappingsAt("owner", ownerFieldMapping)

	// 数值字段：用于大小过滤
	sizeFieldMapping := bleve.NewNumericFieldMapping()
	defaultMapping.AddFieldMappingsAt("size", sizeFieldMapping)

	// 日期字段：用于时间过滤
	dateFieldMapping := bleve.NewDateTimeFieldMapping()
	defaultMapping.AddFieldMappingsAt("modified_at", dateFieldMapping)
	defaultMapping.AddFieldMappingsAt("created_at", dateFieldMapping)

	mapping.AddDocumentMapping("searchindex", defaultMapping)
	mapping.DefaultMapping = defaultMapping

	index, err := bleve.New(e.indexPath, mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create bleve index: %w", err)
	}

	return index, nil
}

// openIndex 打开现有索引或创建新索引
func (e *SearchEngine) openIndex() (bleve.Index, error) {
	// 尝试打开现有索引
	index, err := bleve.Open(e.indexPath)
	if err != nil {
		// 索引不存在，创建新的
		e.logger.Info("index not found, creating new index", zap.String("path", e.indexPath))
		return e.createIndex()
	}
	return index, nil
}

// Start 启动搜索引擎
func (e *SearchEngine) Start() error {
	if e.running {
		return fmt.Errorf("search engine is already running")
	}

	index, err := e.openIndex()
	if err != nil {
		return fmt.Errorf("failed to open search index: %w", err)
	}

	e.index = index
	e.running = true
	e.indexReady = true
	e.stats.Status = IndexStatusIdle

	// 更新统计信息
	e.updateStats()

	e.logger.Info("search engine started", zap.String("path", e.indexPath))
	return nil
}

// Stop 停止搜索引擎
func (e *SearchEngine) Stop() error {
	if !e.running {
		return nil
	}

	e.running = false

	// 安全关闭 channel
	select {
	case <-e.stopChan:
		// already closed
	default:
		close(e.stopChan)
	}

	if e.index != nil {
		if err := e.index.Close(); err != nil {
			e.logger.Error("failed to close index", zap.Error(err))
		}
		e.index = nil
	}

	e.logger.Info("search engine stopped")
	return nil
}

// IsRunning 检查是否运行中
func (e *SearchEngine) IsRunning() bool {
	return e.running
}

// IndexDocument 索引单个文档
func (e *SearchEngine) IndexDocument(doc *SearchIndex) error {
	if !e.running || e.index == nil {
		return fmt.Errorf("search engine is not running")
	}

	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	if doc.Path == "" {
		return fmt.Errorf("document path cannot be empty")
	}

	// 准备索引数据
	indexData := map[string]interface{}{
		"name":         doc.Name,
		"path":         doc.Path,
		"content":      doc.Content,
		"extension":    doc.Extension,
		"content_type": string(doc.ContentType),
		"mime_type":    doc.MimeType,
		"size":         doc.Size,
		"tags":         strings.Join(doc.Tags, " "),
		"owner":        doc.Owner,
		"modified_at":  doc.ModifiedAt,
		"created_at":   doc.CreatedAt,
	}

	// 添加元数据
	if doc.Metadata != nil {
		metadataStrs := make([]string, 0, len(doc.Metadata))
		for k, v := range doc.Metadata {
			metadataStrs = append(metadataStrs, k+":"+v)
		}
		indexData["metadata"] = strings.Join(metadataStrs, " ")
	}

	// 使用路径的哈希作为文档 ID
	docID := doc.ID
	if docID == "" {
		docID = generateDocID(doc.Path)
	}

	// 索引文档
	if err := e.index.Index(docID, indexData); err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	// 更新统计
	e.updateStats()

	e.logger.Debug("document indexed",
		zap.String("id", docID),
		zap.String("path", doc.Path))

	return nil
}

// IndexBatch 批量索引文档
func (e *SearchEngine) IndexBatch(docs []*SearchIndex) (int, error) {
	if !e.running || e.index == nil {
		return 0, fmt.Errorf("search engine is not running")
	}

	indexed := 0
	for _, doc := range docs {
		if err := e.IndexDocument(doc); err != nil {
			e.logger.Warn("failed to index document",
				zap.String("path", doc.Path),
				zap.Error(err))
			continue
		}
		indexed++
	}

	e.logger.Info("batch indexing completed",
		zap.Int("total", len(docs)),
		zap.Int("indexed", indexed))

	return indexed, nil
}

// RemoveDocument 从索引中移除文档
func (e *SearchEngine) RemoveDocument(docID string) error {
	if !e.running || e.index == nil {
		return fmt.Errorf("search engine is not running")
	}

	if err := e.index.Delete(docID); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	e.updateStats()
	e.logger.Info("document removed", zap.String("id", docID))
	return nil
}

// Search 执行搜索
func (e *SearchEngine) Search(query *SearchQuery) (*SearchResponse, error) {
	if !e.running || e.index == nil {
		return nil, fmt.Errorf("search engine is not running")
	}

	if query == nil || query.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	start := time.Now()

	// 应用默认值
	applyQueryDefaults(query)

	// 构建 bleve 查询
	bleveQuery := e.buildBleveQuery(query)

	// 创建搜索请求
	searchRequest := bleve.NewSearchRequest(bleveQuery)

	// 请求返回字段
	searchRequest.Fields = []string{"name", "path", "extension", "content_type", "mime_type", "size", "tags", "owner", "modified_at", "created_at", "content"}

	// 设置分页
	searchRequest.Size = query.PageSize
	searchRequest.From = (query.Page - 1) * query.PageSize

	// 设置高亮
	if query.Highlight {
		searchRequest.Highlight = bleve.NewHighlight()
		searchRequest.Highlight.AddField("name")
		searchRequest.Highlight.AddField("content")
		searchRequest.Highlight.AddField("path")
	}

	// 添加排序
	if query.SortBy != SortRelevance {
		searchRequest.SortBy([]string{e.getSortField(query.SortBy)})
	}

	// 执行搜索
	searchResult, err := e.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 转换结果
	results := make([]SearchResult, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		result := SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}

		// 提取字段
		if name, ok := hit.Fields["name"].(string); ok {
			result.Name = name
		}
		if path, ok := hit.Fields["path"].(string); ok {
			result.Path = path
		}
		if ext, ok := hit.Fields["extension"].(string); ok {
			result.Extension = ext
		}
		if ct, ok := hit.Fields["content_type"].(string); ok {
			result.ContentType = ContentType(ct)
		}
		if mime, ok := hit.Fields["mime_type"].(string); ok {
			result.MimeType = mime
		}
		if size, ok := hit.Fields["size"].(float64); ok {
			result.Size = int64(size)
		}
		if tags, ok := hit.Fields["tags"].(string); ok && tags != "" {
			result.Tags = strings.Fields(tags)
		}
		if modAt, ok := hit.Fields["modified_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, modAt); err == nil {
				result.ModifiedAt = t
			}
		} else if modAt, ok := hit.Fields["modified_at"].(float64); ok {
			// bleve may return numeric timestamp
			result.ModifiedAt = time.UnixMilli(int64(modAt))
		}
		if createAt, ok := hit.Fields["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createAt); err == nil {
				result.CreatedAt = t
			}
		} else if createAt, ok := hit.Fields["created_at"].(float64); ok {
			result.CreatedAt = time.UnixMilli(int64(createAt))
		}

		// 高亮处理
		if query.Highlight && len(hit.Fragments) > 0 {
			result.Highlights = make(map[string]string)
			for field, fragments := range hit.Fragments {
				result.Highlights[field] = strings.Join(fragments, "...")
			}
		}

		// 生成内容摘要
		if content, ok := hit.Fields["content"].(string); ok && content != "" {
			result.Summary = generateSummary(content, query.Query, 200)
		}

		results = append(results, result)
	}

	totalPages := int(searchResult.Total) / query.PageSize
	if int(searchResult.Total)%query.PageSize > 0 {
		totalPages++
	}

	return &SearchResponse{
		Query:      query.Query,
		Total:      int(searchResult.Total),
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
		Results:    results,
		TimeMs:     time.Since(start).Milliseconds(),
	}, nil
}

// buildBleveQuery 构建 bleve 查询
func (e *SearchEngine) buildBleveQuery(q *SearchQuery) query.Query {
	queryStr := q.Query

	// 正则表达式查询
	if q.UseRegex {
		regexQuery := bleve.NewRegexpQuery(queryStr)
		return regexQuery
	}

	// 模糊查询
	if q.Fuzzy {
		fuzzyQuery := bleve.NewFuzzyQuery(queryStr)
		fuzzyQuery.SetFuzziness(q.FuzzyLevel)
		return fuzzyQuery
	}

	// 标准匹配查询
	// 使用 disjunction 查询在多个字段中搜索
	nameMatch := bleve.NewMatchQuery(queryStr)
	nameMatch.SetField("name")
	nameMatch.SetBoost(10.0) // 文件名匹配权重最高

	contentMatch := bleve.NewMatchQuery(queryStr)
	contentMatch.SetField("content")
	contentMatch.SetBoost(3.0)

	pathMatch := bleve.NewMatchQuery(queryStr)
	pathMatch.SetField("path")
	pathMatch.SetBoost(2.0)

	tagMatch := bleve.NewMatchQuery(queryStr)
	tagMatch.SetField("tags")
	tagMatch.SetBoost(5.0)

	metadataMatch := bleve.NewMatchQuery(queryStr)
	metadataMatch.SetField("metadata")
	metadataMatch.SetBoost(2.0)

	// 组合查询
	shouldQueries := []query.Query{
		nameMatch,
		contentMatch,
		pathMatch,
		tagMatch,
		metadataMatch,
	}

	disjunction := bleve.NewDisjunctionQuery(shouldQueries...)

	// 应用过滤条件
	mustQueries := []query.Query{disjunction}

	// 内容类型过滤
	if len(q.Types) > 0 {
		typeQueries := make([]query.Query, 0, len(q.Types))
		for _, ct := range q.Types {
			termQuery := bleve.NewTermQuery(string(ct))
			termQuery.SetField("content_type")
			typeQueries = append(typeQueries, termQuery)
		}
		mustQueries = append(mustQueries, bleve.NewDisjunctionQuery(typeQueries...))
	}

	// 标签过滤
	if len(q.Tags) > 0 {
		for _, tag := range q.Tags {
			tagQuery := bleve.NewMatchQuery(tag)
			tagQuery.SetField("tags")
			mustQueries = append(mustQueries, tagQuery)
		}
	}

	// 路径前缀过滤
	if q.Path != "" {
		prefixQuery := bleve.NewPrefixQuery(q.Path)
		prefixQuery.SetField("path")
		mustQueries = append(mustQueries, prefixQuery)
	}

	// 大小范围过滤
	if q.SizeMin != nil || q.SizeMax != nil {
		minVal := float64(0)
		maxVal := float64(0)
		if q.SizeMin != nil {
			minVal = float64(*q.SizeMin)
		}
		if q.SizeMax != nil {
			maxVal = float64(*q.SizeMax)
		}
		sizeQuery := bleve.NewNumericRangeQuery(&minVal, &maxVal)
		sizeQuery.SetField("size")
		mustQueries = append(mustQueries, sizeQuery)
	}

	// 日期范围过滤
	if q.DateFrom != nil || q.DateTo != nil {
		var dateFrom, dateTo time.Time
		if q.DateFrom != nil {
			dateFrom = *q.DateFrom
		}
		if q.DateTo != nil {
			dateTo = *q.DateTo
		}
		dateQuery := bleve.NewDateRangeQuery(dateFrom, dateTo)
		dateQuery.SetField("modified_at")
		mustQueries = append(mustQueries, dateQuery)
	}

	// 使用 conjunction 组合所有必须匹配的条件
	if len(mustQueries) > 1 {
		return bleve.NewConjunctionQuery(mustQueries...)
	}

	return disjunction
}

// GetSuggestions 获取搜索建议
func (e *SearchEngine) GetSuggestions(prefix string, limit int) ([]string, error) {
	if !e.running || e.index == nil {
		return nil, fmt.Errorf("search engine is not running")
	}

	if limit <= 0 {
		limit = 10
	}

	// 使用前缀查询在文件名字段中搜索
	prefixQuery := bleve.NewPrefixQuery(strings.ToLower(prefix))
	prefixQuery.SetField("name")

	searchRequest := bleve.NewSearchRequest(prefixQuery)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"name"}

	result, err := e.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("suggestion search failed: %w", err)
	}

	suggestions := make([]string, 0, len(result.Hits))
	seen := make(map[string]bool)

	for _, hit := range result.Hits {
		if name, ok := hit.Fields["name"].(string); ok {
			if !seen[name] {
				seen[name] = true
				suggestions = append(suggestions, name)
			}
		}
	}

	return suggestions, nil
}

// RebuildIndex 重建索引
func (e *SearchEngine) RebuildIndex() error {
	if !e.running {
		return fmt.Errorf("search engine is not running")
	}

	e.stats.Status = IndexStatusBuilding

	// 关闭现有索引
	if e.index != nil {
		if err := e.index.Close(); err != nil {
			e.logger.Error("failed to close index for rebuild", zap.Error(err))
		}
		e.index = nil
	}

	// 删除旧索引目录并重新创建
	if err := os.RemoveAll(e.indexPath); err != nil {
		e.logger.Warn("failed to remove old index directory", zap.Error(err))
	}

	// 重新创建索引
	index, err := e.createIndex()
	if err != nil {
		e.stats.Status = IndexStatusError
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	e.index = index
	e.stats.Status = IndexStatusIdle
	e.stats.TotalDocuments = 0

	e.logger.Info("index rebuilt successfully")
	return nil
}

// GetStats 获取索引统计
func (e *SearchEngine) GetStats() *IndexStats {
	e.updateStats()
	return e.stats
}

// updateStats 更新统计信息
func (e *SearchEngine) updateStats() {
	if e.index == nil {
		return
	}

	docCount, err := e.index.DocCount()
	if err != nil {
		e.logger.Error("failed to get document count", zap.Error(err))
		return
	}

	e.stats.TotalDocuments = int(docCount)
	now := time.Now()
	e.stats.LastIndexedAt = &now
}

// getSortField 获取排序字段
func (e *SearchEngine) getSortField(sortBy SortOrder) string {
	switch sortBy {
	case SortDateDesc, SortDateAsc:
		return "modified_at"
	case SortSizeDesc, SortSizeAsc:
		return "size"
	case SortNameAsc, SortNameDesc:
		return "name"
	default:
		return "_score"
	}
}

// generateSummary 生成内容摘要
func generateSummary(content, query string, maxLen int) string {
	if content == "" {
		return ""
	}

	queryLower := strings.ToLower(query)
	contentLower := strings.ToLower(content)

	// 找到第一个匹配的位置
	bestPos := -1
	terms := strings.Fields(queryLower)
	for _, term := range terms {
		pos := strings.Index(contentLower, term)
		if pos >= 0 && (bestPos == -1 || pos < bestPos) {
			bestPos = pos
		}
	}

	if bestPos == -1 {
		bestPos = 0
	}

	// 从匹配位置前后截取摘要
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

	summary := content[start:end]

	// 添加省略号
	if start > 0 {
		summary = "..." + summary
	}
	if end < len(content) {
		summary = summary + "..."
	}

	return summary
}

// generateDocID 生成文档 ID（基于路径的哈希）
func generateDocID(path string) string {
	// 使用简单的路径转换作为 ID
	// 实际生产环境中应使用更健壮的哈希算法
	re := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	return re.ReplaceAllString(path, "_")
}

// applyQueryDefaults 应用查询默认值
func applyQueryDefaults(query *SearchQuery) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	if query.BooleanOp == "" {
		query.BooleanOp = BooleanAND
	}
	if query.SortBy == "" {
		query.SortBy = SortRelevance
	}
	if query.FuzzyLevel <= 0 {
		query.FuzzyLevel = 1
	}
}
