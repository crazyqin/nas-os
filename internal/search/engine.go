package search

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/simple"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"go.uber.org/zap"
)

// FileInfo 文件信息
type FileInfo struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Ext       string    `json:"ext"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"modTime"`
	IsDir     bool      `json:"isDir"`
	Content   string    `json:"content,omitempty"`
	MimeType  string    `json:"mimeType"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Path      string        `json:"path"`
	Name      string        `json:"name"`
	Ext       string        `json:"ext"`
	Size      int64         `json:"size"`
	ModTime   time.Time     `json:"modTime"`
	IsDir     bool          `json:"isDir"`
	Score     float64       `json:"score"`
	Highlights []Highlight  `json:"highlights,omitempty"`
}

// Highlight 高亮信息
type Highlight struct {
	Field string   `json:"field"`
	Fragments []string `json:"fragments"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query     string   `json:"query"`
	Paths     []string `json:"paths,omitempty"`     // 搜索路径限制
	Types     []string `json:"types,omitempty"`     // 文件类型过滤
	MinSize   int64    `json:"minSize,omitempty"`   // 最小文件大小
	MaxSize   int64    `json:"maxSize,omitempty"`   // 最大文件大小
	FromDate  *time.Time `json:"fromDate,omitempty"` // 修改时间起始
	ToDate    *time.Time `json:"toDate,omitempty"`   // 修改时间结束
	Limit     int      `json:"limit,omitempty"`     // 结果数量限制
	Offset    int      `json:"offset,omitempty"`    // 偏移量
	SortBy    string   `json:"sortBy,omitempty"`    // 排序字段: score, name, size, modTime
	SortDesc  bool     `json:"sortDesc,omitempty"`  // 是否降序
}

// IndexConfig 索引配置
type IndexConfig struct {
	IndexPath     string        `json:"indexPath"`     // 索引存储路径
	MaxFileSize   int64         `json:"maxFileSize"`  // 最大索引文件大小
	Workers       int           `json:"workers"`      // 索引工作线程数
	IndexContent  bool          `json:"indexContent"` // 是否索引文件内容
	BatchSize     int           `json:"batchSize"`    // 批量索引大小
	TextExts      []string      `json:"textExts"`     // 需要索引内容的扩展名
	ExcludeDirs   []string      `json:"excludeDirs"` // 排除的目录
	ExcludeFiles  []string      `json:"excludeFiles"`// 排除的文件模式
}

// IndexStats 索引统计
type IndexStats struct {
	TotalFiles    int64     `json:"totalFiles"`
	IndexedFiles  int64     `json:"indexedFiles"`
	IndexSize     int64     `json:"indexSize"`
	LastIndexed   time.Time `json:"lastIndexed"`
	IndexDuration time.Duration `json:"indexDuration"`
}

// Engine 搜索引擎
type Engine struct {
	config      IndexConfig
	index       bleve.Index
	logger      *zap.Logger
	stats       IndexStats
	mu          sync.RWMutex
	textExts    map[string]bool
	excludeDirs map[string]bool
	indexing    bool
	stopChan    chan struct{}
}

// DefaultIndexConfig 默认配置
func DefaultIndexConfig() IndexConfig {
	return IndexConfig{
		IndexPath:    "/var/lib/nas-os/search/index.bleve",
		MaxFileSize:  10 * 1024 * 1024, // 10MB
		Workers:      4,
		IndexContent: true,
		BatchSize:    100,
		TextExts: []string{
			".txt", ".md", ".json", ".yaml", ".yml", ".xml", ".html", ".css", ".js", ".ts",
			".go", ".py", ".java", ".c", ".cpp", ".h", ".rs", ".rb", ".php", ".sh", ".bash",
			".sql", ".conf", ".cfg", ".ini", ".env", ".log", ".csv", ".tsv",
		},
		ExcludeDirs: []string{
			".git", ".svn", ".hg", "node_modules", "vendor", "tmp", "temp", "cache",
		},
		ExcludeFiles: []string{
			"*.tmp", "*.temp", "*.bak", "*.swp", "*.swo", ".DS_Store", "Thumbs.db",
		},
	}
}

// NewEngine 创建搜索引擎
func NewEngine(config IndexConfig, logger *zap.Logger) (*Engine, error) {
	if config.IndexPath == "" {
		config = DefaultIndexConfig()
	}
	
	// 确保索引目录存在
	indexDir := filepath.Dir(config.IndexPath)
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, fmt.Errorf("创建索引目录失败: %w", err)
	}

	// 构建文本扩展名映射
	textExts := make(map[string]bool)
	for _, ext := range config.TextExts {
		textExts[strings.ToLower(ext)] = true
	}

	// 构建排除目录映射
	excludeDirs := make(map[string]bool)
	for _, dir := range config.ExcludeDirs {
		excludeDirs[dir] = true
	}

	engine := &Engine{
		config:      config,
		logger:      logger,
		textExts:    textExts,
		excludeDirs: excludeDirs,
		stopChan:    make(chan struct{}),
	}

	// 打开或创建索引
	index, err := engine.openOrCreateIndex()
	if err != nil {
		return nil, fmt.Errorf("初始化索引失败: %w", err)
	}
	engine.index = index

	return engine, nil
}

// openOrCreateIndex 打开或创建索引
func (e *Engine) openOrCreateIndex() (bleve.Index, error) {
	// 尝试打开已有索引
	index, err := bleve.Open(e.config.IndexPath)
	if err == nil {
		e.logger.Info("打开已有搜索索引", zap.String("path", e.config.IndexPath))
		return index, nil
	}

	// 创建新索引
	e.logger.Info("创建新的搜索索引", zap.String("path", e.config.IndexPath))
	
	mapping := e.createIndexMapping()
	index, err = bleve.New(e.config.IndexPath, mapping)
	if err != nil {
		return nil, fmt.Errorf("创建索引失败: %w", err)
	}

	return index, nil
}

// createIndexMapping 创建索引映射
func (e *Engine) createIndexMapping() mapping.IndexMapping {
	// 创建文档映射
	docMapping := bleve.NewDocumentMapping()

	// Path 字段 - 关键词分析器，精确匹配
	pathFieldMapping := bleve.NewTextFieldMapping()
	pathFieldMapping.Analyzer = keyword.Name
	pathFieldMapping.Store = true
	pathFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("path", pathFieldMapping)

	// Name 字段 - 简单分析器，支持部分匹配
	nameFieldMapping := bleve.NewTextFieldMapping()
	nameFieldMapping.Analyzer = simple.Name
	nameFieldMapping.Store = true
	nameFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("name", nameFieldMapping)

	// Ext 字段 - 关键词分析器
	extFieldMapping := bleve.NewTextFieldMapping()
	extFieldMapping.Analyzer = keyword.Name
	extFieldMapping.Store = true
	extFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("ext", extFieldMapping)

	// Content 字段 - 标准分析器，支持全文搜索
	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Store = true
	contentFieldMapping.Index = true
	contentFieldMapping.IncludeTermVectors = true
	contentFieldMapping.IncludeInAll = true
	docMapping.AddFieldMappingsAt("content", contentFieldMapping)

	// Size 字段 - 数值字段
	sizeFieldMapping := bleve.NewNumericFieldMapping()
	sizeFieldMapping.Store = true
	sizeFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("size", sizeFieldMapping)

	// ModTime 字段 - 日期时间字段
	modTimeFieldMapping := bleve.NewDateTimeFieldMapping()
	modTimeFieldMapping.Store = true
	modTimeFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("modTime", modTimeFieldMapping)

	// IsDir 字段 - 布尔字段
	isDirFieldMapping := bleve.NewBooleanFieldMapping()
	isDirFieldMapping.Store = true
	isDirFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("isDir", isDirFieldMapping)

	// MimeType 字段
	mimeTypeFieldMapping := bleve.NewTextFieldMapping()
	mimeTypeFieldMapping.Analyzer = keyword.Name
	mimeTypeFieldMapping.Store = true
	mimeTypeFieldMapping.Index = true
	docMapping.AddFieldMappingsAt("mimeType", mimeTypeFieldMapping)

	// 创建索引映射
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = "standard"

	return indexMapping
}

// shouldIndexContent 是否应该索引文件内容
func (e *Engine) shouldIndexContent(path string, size int64) bool {
	if !e.config.IndexContent {
		return false
	}
	if size > e.config.MaxFileSize {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return e.textExts[ext]
}

// shouldExclude 是否应该排除
func (e *Engine) shouldExclude(path string) bool {
	// 检查目录
	for _, part := range strings.Split(path, string(os.PathSeparator)) {
		if e.excludeDirs[part] {
			return true
		}
	}

	// 检查文件模式
	name := filepath.Base(path)
	for _, pattern := range e.config.ExcludeFiles {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}

	return false
}

// getMimeType 获取MIME类型
func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".yaml": "application/x-yaml",
		".yml":  "application/x-yaml",
		".xml":  "application/xml",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".ts":   "application/typescript",
		".go":   "text/x-go",
		".py":   "text/x-python",
		".java": "text/x-java",
		".c":    "text/x-c",
		".cpp":  "text/x-c++",
		".h":    "text/x-c",
		".rs":   "text/x-rust",
		".rb":   "text/x-ruby",
		".php":  "text/x-php",
		".sh":   "text/x-sh",
		".sql":  "application/x-sql",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
		".zip":  "application/zip",
	}
	if mt, ok := mimeTypes[strings.ToLower(ext)]; ok {
		return mt
	}
	return "application/octet-stream"
}

// readFileContent 读取文件内容
func (e *Engine) readFileContent(path string, maxSize int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 限制读取大小
	if maxSize <= 0 {
		maxSize = e.config.MaxFileSize
	}
	
	buf := make([]byte, maxSize)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	return string(buf[:n]), nil
}

// IndexFile 索引单个文件
func (e *Engine) IndexFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 检查是否排除
	if e.shouldExclude(path) {
		return nil
	}

	file := FileInfo{
		Path:    path,
		Name:    info.Name(),
		Ext:     strings.ToLower(filepath.Ext(path)),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}

	if !info.IsDir() {
		file.MimeType = getMimeType(file.Ext)

		// 索引文件内容
		if e.shouldIndexContent(path, info.Size()) {
			content, err := e.readFileContent(path, 0)
			if err == nil {
				file.Content = content
			}
		}
	}

	// 索引文件
	if err := e.index.Index(path, file); err != nil {
		return fmt.Errorf("索引文件失败: %w", err)
	}

	e.mu.Lock()
	e.stats.IndexedFiles++
	e.mu.Unlock()

	return nil
}

// IndexDirectory 索引目录
func (e *Engine) IndexDirectory(root string) error {
	e.mu.Lock()
	if e.indexing {
		e.mu.Unlock()
		return fmt.Errorf("索引正在进行中")
	}
	e.indexing = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.indexing = false
		e.mu.Unlock()
	}()

	startTime := time.Now()
	batch := e.index.NewBatch()
	count := 0
	totalCount := int64(0)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}

		// 检查是否排除
		if e.shouldExclude(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		file := FileInfo{
			Path:    path,
			Name:    info.Name(),
			Ext:     strings.ToLower(filepath.Ext(path)),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}

		if !info.IsDir() {
			file.MimeType = getMimeType(file.Ext)

			// 索引文件内容
			if e.shouldIndexContent(path, info.Size()) {
				content, err := e.readFileContent(path, 0)
				if err == nil {
					file.Content = content
				}
			}
		}

		// 添加到批次
		if err := batch.Index(path, file); err != nil {
			e.logger.Warn("添加到索引批次失败", 
				zap.String("path", path),
				zap.Error(err))
		}

		count++
		totalCount++

		// 批量提交
		if count >= e.config.BatchSize {
			if err := e.index.Batch(batch); err != nil {
				e.logger.Error("批量索引失败", zap.Error(err))
			}
			batch = e.index.NewBatch()
			count = 0
		}

		return nil
	})

	// 提交剩余的
	if count > 0 {
		if err := e.index.Batch(batch); err != nil {
			e.logger.Error("批量索引失败", zap.Error(err))
		}
	}

	e.mu.Lock()
	e.stats.TotalFiles = totalCount
	e.stats.IndexedFiles = totalCount
	e.stats.LastIndexed = time.Now()
	e.stats.IndexDuration = time.Since(startTime)
	e.mu.Unlock()

	if err != nil {
		return fmt.Errorf("索引目录失败: %w", err)
	}

	e.logger.Info("索引完成",
		zap.String("root", root),
		zap.Int64("total", totalCount),
		zap.Duration("duration", time.Since(startTime)))

	return nil
}

// Search 执行搜索
func (e *Engine) Search(req SearchRequest) (*SearchResponse, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("搜索查询不能为空")
	}

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	// 构建查询
	searchQuery := e.buildQuery(req)

	// 构建搜索请求
	searchReq := bleve.NewSearchRequestOptions(searchQuery, req.Limit, req.Offset, false)
	
	// 设置高亮
	searchReq.Highlight = bleve.NewHighlightWithStyle("html")
	searchReq.Highlight.Fields = []string{"name", "content"}

	// 设置排序
	if req.SortBy != "" && req.SortBy != "score" {
		if req.SortDesc {
			searchReq.SortBy([]string{"-" + req.SortBy})
		} else {
			searchReq.SortBy([]string{req.SortBy})
		}
	}

	// 执行搜索
	result, err := e.index.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	// 处理结果
	response := &SearchResponse{
		Total:     int(result.Total),
		Took:      result.Took,
		MaxScore:  result.MaxScore,
		Results:   make([]SearchResult, 0, len(result.Hits)),
	}

	for _, hit := range result.Hits {
		searchResult := SearchResult{
			Path:    hit.ID,
			Score:   hit.Score,
		}

		// 从存储字段获取信息
		if name, ok := hit.Fields["name"].(string); ok {
			searchResult.Name = name
		}
		if ext, ok := hit.Fields["ext"].(string); ok {
			searchResult.Ext = ext
		}
		if size, ok := hit.Fields["size"].(float64); ok {
			searchResult.Size = int64(size)
		}
		if isDir, ok := hit.Fields["isDir"].(bool); ok {
			searchResult.IsDir = isDir
		}

		// 处理高亮
		if hit.Fragments != nil {
			searchResult.Highlights = make([]Highlight, 0)
			for field, fragments := range hit.Fragments {
				searchResult.Highlights = append(searchResult.Highlights, Highlight{
					Field:     field,
					Fragments: fragments,
				})
			}
		}

		response.Results = append(response.Results, searchResult)
	}

	return response, nil
}

// buildQuery 构建查询
func (e *Engine) buildQuery(req SearchRequest) query.Query {
	// 主查询
	mainQuery := bleve.NewMatchQuery(req.Query)
	mainQuery.SetFuzziness(1)
	mainQuery.SetPrefix(3)

	queries := []query.Query{mainQuery}

	// 路径过滤
	if len(req.Paths) > 0 {
		pathQueries := make([]query.Query, len(req.Paths))
		for i, path := range req.Paths {
			prefixQuery := bleve.NewPrefixQuery(path)
			prefixQuery.SetField("path")
			pathQueries[i] = prefixQuery
		}
		pathConjunction := bleve.NewDisjunctionQuery(pathQueries...)
		queries = append(queries, pathConjunction)
	}

	// 文件类型过滤
	if len(req.Types) > 0 {
		typeQueries := make([]query.Query, len(req.Types))
		for i, ext := range req.Types {
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			termQuery := bleve.NewTermQuery(ext)
			termQuery.SetField("ext")
			typeQueries[i] = termQuery
		}
		typeConjunction := bleve.NewDisjunctionQuery(typeQueries...)
		queries = append(queries, typeConjunction)
	}

	// 文件大小过滤
	if req.MinSize > 0 || req.MaxSize > 0 {
		minSize := float64(req.MinSize)
		maxSize := float64(req.MaxSize)
		rangeQuery := bleve.NewNumericRangeQuery(&minSize, &maxSize)
		rangeQuery.SetField("size")
		queries = append(queries, rangeQuery)
	}

	// 时间过滤
	if req.FromDate != nil || req.ToDate != nil {
		var from, to time.Time
		if req.FromDate != nil {
			from = *req.FromDate
		}
		if req.ToDate != nil {
			to = *req.ToDate
		}
		rangeQuery := bleve.NewDateRangeQuery(from, to)
		rangeQuery.SetField("modTime")
		queries = append(queries, rangeQuery)
	}

	// 组合所有查询
	if len(queries) > 1 {
		return bleve.NewConjunctionQuery(queries...)
	}
	return mainQuery
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Total    int            `json:"total"`
	Took     time.Duration  `json:"took"`
	MaxScore float64        `json:"maxScore"`
	Results  []SearchResult `json:"results"`
}

// Delete 从索引中删除
func (e *Engine) Delete(path string) error {
	return e.index.Delete(path)
}

// DeleteBatch 批量删除
func (e *Engine) DeleteBatch(paths []string) error {
	batch := e.index.NewBatch()
	for _, path := range paths {
		batch.Delete(path)
	}
	return e.index.Batch(batch)
}

// Stats 获取索引统计
func (e *Engine) Stats() IndexStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// Close 关闭搜索引擎
func (e *Engine) Close() error {
	close(e.stopChan)
	return e.index.Close()
}

// parseNumeric 解析数值
func parseNumeric(data []byte) (int64, error) {
	var val int64
	_, err := fmt.Sscanf(string(data), "%d", &val)
	return val, err
}

// parseDateTime 解析日期时间
func parseDateTime(data []byte) (time.Time, error) {
	return time.Parse(time.RFC3339, string(data))
}