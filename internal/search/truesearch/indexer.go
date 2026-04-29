package truesearch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/simple"
	"github.com/blevesearch/bleve/v2/mapping"
	blevequery "github.com/blevesearch/bleve/v2/search/query"
	"go.uber.org/zap"
)

// SearchRequest 搜索请求。
type SearchRequest struct {
	Query      string   `json:"query"`
	Path       string   `json:"path,omitempty"`
	Types      []string `json:"types,omitempty"`
	MaxResults int      `json:"max_results"`
	Highlight  bool     `json:"highlight"`
}

// SearchResponse 搜索响应。
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	TookMs  int64          `json:"took_ms"`
}

// SearchResult 搜索结果项。
type SearchResult struct {
	Path    string  `json:"path"`
	Name    string  `json:"name"`
	Size    int64   `json:"size"`
	ModTime string  `json:"mod_time"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// IndexStatus 索引状态。
type IndexStatus struct {
	TotalFiles   int    `json:"total_files"`
	IndexedFiles int    `json:"indexed_files"`
	PendingFiles int    `json:"pending_files"`
	LastUpdate   string `json:"last_update"`
	IndexSize    int64  `json:"index_size"`
}

// indexedDoc 索引中的文档结构。
type indexedDoc struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Ext     string    `json:"ext"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Content string    `json:"content"`
}

// Indexer 全文索引引擎。
type Indexer struct {
	config    Config
	index     bleve.Index
	logger    *zap.Logger
	mu        sync.RWMutex
	stats     indexStats
	extFilter map[string]bool
	dirFilter map[string]bool
	closed    bool
}

// indexStats 内部索引统计。
type indexStats struct {
	totalFiles int
	lastUpdate time.Time
}

// NewIndexer 创建索引引擎。
func NewIndexer(cfg Config, logger *zap.Logger) (*Indexer, error) {
	extFilter := make(map[string]bool)
	for _, ext := range cfg.SupportedExts {
		extFilter[strings.ToLower(ext)] = true
	}
	dirFilter := make(map[string]bool)
	for _, dir := range cfg.ExcludeDirs {
		dirFilter[dir] = true
	}

	idx := &Indexer{
		config:    cfg,
		logger:    logger,
		extFilter: extFilter,
		dirFilter: dirFilter,
	}

	index, err := idx.openOrCreate()
	if err != nil {
		return nil, fmt.Errorf("init index: %w", err)
	}
	idx.index = index

	return idx, nil
}

// openOrCreate 打开或创建 bleve 索引。
func (idx *Indexer) openOrCreate() (bleve.Index, error) {
	dir := filepath.Dir(idx.config.IndexPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create index dir: %w", err)
	}

	index, err := bleve.Open(idx.config.IndexPath)
	if err == nil {
		idx.logger.Info("opened existing index", zap.String("path", idx.config.IndexPath))
		return index, nil
	}

	idx.logger.Info("creating new index", zap.String("path", idx.config.IndexPath))
	mapping := idx.buildMapping()
	index, err = bleve.New(idx.config.IndexPath, mapping)
	if err != nil {
		return nil, fmt.Errorf("create index: %w", err)
	}
	return index, nil
}

// buildMapping 创建 bleve 索引映射。
func (idx *Indexer) buildMapping() mapping.IndexMapping {
	docMapping := bleve.NewDocumentMapping()

	pathField := bleve.NewTextFieldMapping()
	pathField.Analyzer = keyword.Name
	pathField.Store = true
	pathField.Index = true
	docMapping.AddFieldMappingsAt("path", pathField)

	nameField := bleve.NewTextFieldMapping()
	nameField.Analyzer = simple.Name
	nameField.Store = true
	nameField.Index = true
	docMapping.AddFieldMappingsAt("name", nameField)

	extField := bleve.NewTextFieldMapping()
	extField.Analyzer = keyword.Name
	extField.Store = true
	extField.Index = true
	docMapping.AddFieldMappingsAt("ext", extField)

	contentField := bleve.NewTextFieldMapping()
	contentField.Store = true
	contentField.Index = true
	contentField.IncludeTermVectors = true
	contentField.IncludeInAll = true
	docMapping.AddFieldMappingsAt("content", contentField)

	sizeField := bleve.NewNumericFieldMapping()
	sizeField.Store = true
	sizeField.Index = true
	docMapping.AddFieldMappingsAt("size", sizeField)

	modTimeField := bleve.NewDateTimeFieldMapping()
	modTimeField.Store = true
	modTimeField.Index = true
	docMapping.AddFieldMappingsAt("modTime", modTimeField)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = "standard"

	return indexMapping
}

// shouldIndex 判断文件是否需要索引。
func (idx *Indexer) shouldIndex(path string) bool {
	for _, part := range strings.Split(path, string(os.PathSeparator)) {
		if idx.dirFilter[part] {
			return false
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	return idx.extFilter[ext]
}

// IndexFile 索引单个文件。
func (idx *Indexer) IndexFile(path string, extractor *Extractor) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	if info.IsDir() {
		return nil
	}
	if !idx.shouldIndex(absPath) {
		return nil
	}

	content, _ := extractor.Extract(absPath)

	doc := indexedDoc{
		Path:    absPath,
		Name:    info.Name(),
		Ext:     filepath.Ext(info.Name()),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Content: content,
	}

	if err := idx.index.Index(absPath, doc); err != nil {
		return fmt.Errorf("index file: %w", err)
	}

	idx.mu.Lock()
	idx.stats.totalFiles++
	idx.stats.lastUpdate = time.Now()
	idx.mu.Unlock()

	return nil
}

// IndexDirectory 递归索引目录。
func (idx *Indexer) IndexDirectory(root string, extractor *Extractor) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}

	batch := idx.index.NewBatch()
	count := 0

	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !idx.shouldIndex(path) {
			return nil
		}

		content, _ := extractor.Extract(path)

		doc := indexedDoc{
			Path:    path,
			Name:    info.Name(),
			Ext:     filepath.Ext(info.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Content: content,
		}

		batch.Index(path, doc)
		count++

		if count >= idx.config.BatchSize {
			if err := idx.index.Batch(batch); err != nil {
				idx.logger.Error("batch index failed", zap.Error(err))
			}
			batch = idx.index.NewBatch()
			count = 0
		}

		return nil
	})

	if count > 0 {
		if batchErr := idx.index.Batch(batch); batchErr != nil {
			idx.logger.Error("final batch index failed", zap.Error(batchErr))
		}
	}

	if err != nil {
		return fmt.Errorf("walk dir: %w", err)
	}

	idx.mu.Lock()
	idx.stats.lastUpdate = time.Now()
	idx.mu.Unlock()

	idx.logger.Info("directory indexed", zap.String("path", absRoot))
	return nil
}

// Search 执行全文搜索。
func (idx *Indexer) Search(req SearchRequest) (*SearchResponse, error) {
	start := time.Now()

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	query := idx.buildQuery(req)

	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = maxResults
	searchRequest.Fields = []string{"path", "name", "ext", "size", "modTime", "content"}
	searchRequest.IncludeLocations = true

	if req.Highlight {
		searchRequest.Highlight = bleve.NewHighlight()
	}

	result, err := idx.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	results := make([]SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		sr := SearchResult{
			Path:  hit.ID,
			Score: hit.Score,
		}

		if name, ok := hit.Fields["name"].(string); ok {
			sr.Name = name
		}
		if size, ok := hit.Fields["size"].(float64); ok {
			sr.Size = int64(size)
		}
		if modTime, ok := hit.Fields["modTime"].(string); ok {
			sr.ModTime = modTime
		}

		if content, ok := hit.Fields["content"].(string); ok && content != "" {
			sr.Snippet = generateSnippet(content, req.Query, 200)
		}

		results = append(results, sr)
	}

	return &SearchResponse{
		Results: results,
		Total:   int(result.Total),
		TookMs:  time.Since(start).Milliseconds(),
	}, nil
}

// buildQuery 构建 bleve 查询。
func (idx *Indexer) buildQuery(req SearchRequest) blevequery.Query {
	conjuncts := []blevequery.Query{blevequery.NewMatchQuery(req.Query)}

	if req.Path != "" {
		pq := blevequery.NewPrefixQuery(req.Path)
		pq.SetField("path")
		conjuncts = append(conjuncts, pq)
	}

	if len(req.Types) > 0 {
		disjuncts := make([]blevequery.Query, 0, len(req.Types))
		for _, t := range req.Types {
			ext := t
			if !strings.HasPrefix(t, ".") {
				ext = "." + t
			}
			tq := blevequery.NewTermQuery(strings.ToLower(ext))
			tq.SetField("ext")
			disjuncts = append(disjuncts, tq)
		}
		if len(disjuncts) == 1 {
			conjuncts = append(conjuncts, disjuncts[0])
		} else if len(disjuncts) > 1 {
			conjuncts = append(conjuncts, blevequery.NewDisjunctionQuery(disjuncts))
		}
	}

	if len(conjuncts) == 1 {
		return conjuncts[0]
	}
	return blevequery.NewConjunctionQuery(conjuncts)
}

// Status 返回索引状态。
func (idx *Indexer) Status() IndexStatus {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	docCount, err := idx.index.DocCount()
	if err != nil {
		docCount = 0
	}
	total := int(docCount)

	var indexSize int64
	info, err := os.Stat(idx.config.IndexPath)
	if err == nil && info.IsDir() {
		indexSize = dirSize(idx.config.IndexPath)
	}

	return IndexStatus{
		TotalFiles:   total,
		IndexedFiles: total,
		PendingFiles: 0,
		LastUpdate:   idx.stats.lastUpdate.Format(time.RFC3339),
		IndexSize:    indexSize,
	}
}

// Reindex 重建索引。
func (idx *Indexer) Reindex(path string, force bool, extractor *Extractor) error {
	if force {
		idx.mu.Lock()
		// 关闭旧索引
		if idx.index != nil && !idx.closed {
			if err := idx.index.Close(); err != nil {
				idx.logger.Warn("close index for reindex", zap.Error(err))
			}
			idx.closed = true
		}
		idx.mu.Unlock()

		if err := os.RemoveAll(idx.config.IndexPath); err != nil {
			return fmt.Errorf("remove index: %w", err)
		}
		index, err := idx.openOrCreate()
		if err != nil {
			return fmt.Errorf("reopen index: %w", err)
		}
		idx.mu.Lock()
		idx.index = index
		idx.closed = false
		idx.mu.Unlock()
	}

	if path != "" {
		return idx.IndexDirectory(path, extractor)
	}

	idx.mu.Lock()
	idx.stats.lastUpdate = time.Now()
	idx.mu.Unlock()
	return nil
}

// Close 关闭索引。
func (idx *Indexer) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.closed {
		return nil
	}
	idx.closed = true
	return idx.index.Close()
}

// generateSnippet 从内容中生成搜索摘要。
func generateSnippet(content string, query string, maxLen int) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)

	idx := strings.Index(lowerContent, lowerQuery)
	if idx < 0 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	start := idx - maxLen/4
	if start < 0 {
		start = 0
	}

	end := start + maxLen
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

// dirSize 递归计算目录大小。
func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
