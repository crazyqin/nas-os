package spotlight

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
	"github.com/blevesearch/bleve/v2/analysis/analyzer/simple"
	"github.com/blevesearch/bleve/v2/mapping"
	blevequery "github.com/blevesearch/bleve/v2/search/query"
	"go.uber.org/zap"
)

// IndexStatus 索引状态
type IndexStatus struct {
	Status       string    `json:"status"`
	TotalFiles   int64     `json:"totalFiles"`
	IndexedFiles int64     `json:"indexedFiles"`
	IndexSize    int64     `json:"indexSize"`
	LastUpdate   time.Time `json:"lastUpdate"`
	Progress     float64   `json:"progress"`
}

// Indexer 内容索引器
type Indexer struct {
	config     EngineConfig
	logger     *zap.Logger
	index      bleve.Index
	mu         sync.RWMutex
	status     IndexStatus
	textExts   map[string]bool
	excludeMap map[string]bool
	indexing   bool
	stopChan   chan struct{}
}

// NewIndexer 创建索引器
func NewIndexer(config EngineConfig, logger *zap.Logger) (*Indexer, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 确保索引目录存在
	indexDir := filepath.Dir(config.IndexPath)
	if err := os.MkdirAll(indexDir, 0750); err != nil {
		return nil, fmt.Errorf("创建索引目录失败: %w", err)
	}

	// 构建文本扩展名映射
	textExts := make(map[string]bool)
	for _, ext := range config.TextExtensions {
		textExts[strings.ToLower(ext)] = true
	}

	// 构建排除路径映射
	excludeMap := make(map[string]bool)
	for _, p := range config.ExcludePaths {
		excludeMap[p] = true
	}

	idx := &Indexer{
		config:     config,
		logger:     logger,
		textExts:   textExts,
		excludeMap: excludeMap,
		stopChan:   make(chan struct{}),
		status:     IndexStatus{Status: "idle"},
	}

	// 打开或创建索引
	index, err := idx.openOrCreateIndex()
	if err != nil {
		return nil, fmt.Errorf("初始化索引失败: %w", err)
	}
	idx.index = index

	return idx, nil
}

// openOrCreateIndex 打开或创建索引
func (idx *Indexer) openOrCreateIndex() (bleve.Index, error) {
	// 尝试打开已有索引
	index, err := bleve.Open(idx.config.IndexPath)
	if err == nil {
		idx.logger.Info("打开已有索引", zap.String("path", idx.config.IndexPath))
		return index, nil
	}

	// 创建新索引
	idx.logger.Info("创建新索引", zap.String("path", idx.config.IndexPath))
	mapping := idx.createIndexMapping()
	index, err = bleve.New(idx.config.IndexPath, mapping)
	if err != nil {
		return nil, fmt.Errorf("创建索引失败: %w", err)
	}

	return index, nil
}

// createIndexMapping 创建索引映射
func (idx *Indexer) createIndexMapping() mapping.IndexMapping {
	docMapping := bleve.NewDocumentMapping()

	// Path 字段 - 精确匹配
	pathMapping := bleve.NewTextFieldMapping()
	pathMapping.Analyzer = keyword.Name
	pathMapping.Store = true
	pathMapping.Index = true
	docMapping.AddFieldMappingsAt("path", pathMapping)

	// Name 字段 - 支持部分匹配
	nameMapping := bleve.NewTextFieldMapping()
	nameMapping.Analyzer = simple.Name
	nameMapping.Store = true
	nameMapping.Index = true
	docMapping.AddFieldMappingsAt("name", nameMapping)

	// Ext 字段 - 精确匹配
	extMapping := bleve.NewTextFieldMapping()
	extMapping.Analyzer = keyword.Name
	extMapping.Store = true
	extMapping.Index = true
	docMapping.AddFieldMappingsAt("ext", extMapping)

	// Content 字段 - 全文搜索
	contentMapping := bleve.NewTextFieldMapping()
	contentMapping.Store = true
	contentMapping.Index = true
	contentMapping.IncludeTermVectors = true
	docMapping.AddFieldMappingsAt("content", contentMapping)

	// Size 字段
	sizeMapping := bleve.NewNumericFieldMapping()
	sizeMapping.Store = true
	sizeMapping.Index = true
	docMapping.AddFieldMappingsAt("size", sizeMapping)

	// ModTime 字段
	modTimeMapping := bleve.NewDateTimeFieldMapping()
	modTimeMapping.Store = true
	modTimeMapping.Index = true
	docMapping.AddFieldMappingsAt("modTime", modTimeMapping)

	// IsDir 字段
	isDirMapping := bleve.NewBooleanFieldMapping()
	isDirMapping.Store = true
	isDirMapping.Index = true
	docMapping.AddFieldMappingsAt("isDir", isDirMapping)

	// MimeType 字段
	mimeTypeMapping := bleve.NewTextFieldMapping()
	mimeTypeMapping.Analyzer = keyword.Name
	mimeTypeMapping.Store = true
	mimeTypeMapping.Index = true
	docMapping.AddFieldMappingsAt("mimeType", mimeTypeMapping)

	// Protocol 字段
	protocolMapping := bleve.NewTextFieldMapping()
	protocolMapping.Analyzer = keyword.Name
	protocolMapping.Store = true
	protocolMapping.Index = true
	docMapping.AddFieldMappingsAt("protocol", protocolMapping)

	// Keywords 字段
	keywordsMapping := bleve.NewTextFieldMapping()
	keywordsMapping.Store = true
	keywordsMapping.Index = true
	docMapping.AddFieldMappingsAt("keywords", keywordsMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = "standard"

	return indexMapping
}

// Start 启动索引器
func (idx *Indexer) Start(ctx context.Context) error {
	idx.mu.Lock()
	idx.status.Status = "ready"
	idx.mu.Unlock()

	idx.logger.Info("索引器已启动")
	return nil
}

// Stop 停止索引器
func (idx *Indexer) Stop() {
	close(idx.stopChan)
	if idx.index != nil {
		idx.index.Close()
	}
	idx.logger.Info("索引器已停止")
}

// IndexFile 索引单个文件
func (idx *Indexer) IndexFile(ctx context.Context, path string) error {
	// 检查是否排除
	if idx.shouldExclude(path) {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	entry := IndexEntry{
		Path:       path,
		Name:       info.Name(),
		Ext:        strings.ToLower(filepath.Ext(path)),
		Size:       info.Size(),
		ModTime:    info.ModTime(),
		IsDir:      info.IsDir(),
		MimeType:   getMimeType(filepath.Ext(path)),
		Protocol:   detectProtocol(path),
		Attributes: make(map[string]string),
	}

	// 索引文件内容
	if !info.IsDir() && idx.shouldIndexContent(path, info.Size()) {
		content, err := idx.readFileContent(path)
		if err == nil {
			entry.Content = content
			entry.Keywords = extractKeywords(content)
		}
	}

	// 添加到索引
	if err := idx.index.Index(path, entry); err != nil {
		return fmt.Errorf("索引文件失败: %w", err)
	}

	idx.mu.Lock()
	idx.status.IndexedFiles++
	idx.mu.Unlock()

	return nil
}

// IndexDirectory 索引目录
func (idx *Indexer) IndexDirectory(ctx context.Context, root string) error {
	idx.mu.Lock()
	if idx.indexing {
		idx.mu.Unlock()
		return fmt.Errorf("索引正在进行中")
	}
	idx.indexing = true
	idx.status.Status = "indexing"
	idx.mu.Unlock()

	defer func() {
		idx.mu.Lock()
		idx.indexing = false
		idx.status.Status = "ready"
		idx.mu.Unlock()
	}()

	startTime := time.Now()
	batch := idx.index.NewBatch()
	count := 0
	totalCount := int64(0)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// 检查上下文
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 检查排除
		if idx.shouldExclude(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entry := IndexEntry{
			Path:       path,
			Name:       info.Name(),
			Ext:        strings.ToLower(filepath.Ext(path)),
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			IsDir:      info.IsDir(),
			MimeType:   getMimeType(filepath.Ext(path)),
			Protocol:   detectProtocol(path),
			Attributes: make(map[string]string),
		}

		// 索引内容
		if !info.IsDir() && idx.shouldIndexContent(path, info.Size()) {
			content, err := idx.readFileContent(path)
			if err == nil {
				entry.Content = content
				entry.Keywords = extractKeywords(content)
			}
		}

		if err := batch.Index(path, entry); err != nil {
			idx.logger.Warn("添加到批次失败",
				zap.String("path", path),
				zap.Error(err))
		}

		count++
		totalCount++

		// 批量提交
		if count >= idx.config.BatchSize {
			if err := idx.index.Batch(batch); err != nil {
				idx.logger.Error("批量索引失败", zap.Error(err))
			}
			batch = idx.index.NewBatch()
			count = 0
		}

		return nil
	})

	// 提交剩余批次
	if count > 0 {
		if err := idx.index.Batch(batch); err != nil {
			idx.logger.Error("最终批次索引失败", zap.Error(err))
		}
	}

	idx.mu.Lock()
	idx.status.TotalFiles = totalCount
	idx.status.IndexedFiles = totalCount
	idx.status.LastUpdate = time.Now()
	idx.mu.Unlock()

	// 更新索引大小
	if info, err := os.Stat(idx.config.IndexPath); err == nil {
		idx.mu.Lock()
		idx.status.IndexSize = info.Size()
		idx.mu.Unlock()
	}

	took := time.Since(startTime)
	idx.logger.Info("目录索引完成",
		zap.String("root", root),
		zap.Int64("total", totalCount),
		zap.Duration("took", took))

	if err != nil && err != context.Canceled {
		return fmt.Errorf("索引目录失败: %w", err)
	}

	return nil
}

// RemoveFromIndex 从索引中移除
func (idx *Indexer) RemoveFromIndex(ctx context.Context, path string) error {
	return idx.index.Delete(path)
}

// Search 执行搜索
func (idx *Indexer) Search(ctx context.Context, query *ParsedQuery, limit, offset int) ([]IndexEntry, int, error) {
	// 构建 Bleve 查询
	bleveQuery := idx.buildBleveQuery(query)

	// 创建搜索请求
	searchReq := bleve.NewSearchRequestOptions(bleveQuery, limit, offset, false)
	searchReq.Highlight = bleve.NewHighlight()
	searchReq.Highlight.Fields = []string{"name", "content"}

	// 设置返回字段
	searchReq.Fields = []string{"path", "name", "ext", "size", "modTime", "isDir", "mimeType", "protocol", "keywords"}

	// 执行搜索
	result, err := idx.index.Search(searchReq)
	if err != nil {
		return nil, 0, fmt.Errorf("搜索失败: %w", err)
	}

	// 解析结果
	entries := make([]IndexEntry, 0, len(result.Hits))
	for _, hit := range result.Hits {
		entry := IndexEntry{
			Path:  hit.ID,
			Score: hit.Score,
		}

		if name, ok := hit.Fields["name"].(string); ok {
			entry.Name = name
		}
		if ext, ok := hit.Fields["ext"].(string); ok {
			entry.Ext = ext
		}
		if size, ok := hit.Fields["size"].(float64); ok {
			entry.Size = int64(size)
		}
		if modTime, ok := hit.Fields["modTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, modTime); err == nil {
				entry.ModTime = t
			}
		}
		if isDir, ok := hit.Fields["isDir"].(bool); ok {
			entry.IsDir = isDir
		}
		if mimeType, ok := hit.Fields["mimeType"].(string); ok {
			entry.MimeType = mimeType
		}
		if protocol, ok := hit.Fields["protocol"].(string); ok {
			entry.Protocol = Protocol(protocol)
		}

		entries = append(entries, entry)
	}

	return entries, int(result.Total), nil
}

// RebuildIndex 重建索引
func (idx *Indexer) RebuildIndex(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 关闭现有索引
	if idx.index != nil {
		idx.index.Close()
	}

	// 删除索引目录
	os.RemoveAll(idx.config.IndexPath)

	// 重新创建索引
	index, err := idx.openOrCreateIndex()
	if err != nil {
		return err
	}
	idx.index = index

	// 重新索引所有路径
	for _, path := range idx.config.IndexPaths {
		if err := idx.IndexDirectory(ctx, path); err != nil {
			idx.logger.Error("重建索引失败",
				zap.String("path", path),
				zap.Error(err))
		}
	}

	return nil
}

// GetStatus 获取索引状态
func (idx *Indexer) GetStatus() IndexStatus {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.status
}

// GetIndexedCount 获取已索引文件数
func (idx *Indexer) GetIndexedCount() int64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.status.IndexedFiles
}

// buildBleveQuery 构建 Bleve 查询
func (idx *Indexer) buildBleveQuery(query *ParsedQuery) blevequery.Query {
	var queries []blevequery.Query

	// 主查询
	if query.Raw != "" {
		matchQuery := bleve.NewMatchQuery(query.Raw)
		matchQuery.SetFuzziness(1)
		matchQuery.SetPrefix(3)
		queries = append(queries, matchQuery)
	}

	// 路径过滤
	if len(query.Paths) > 0 {
		pathQueries := make([]blevequery.Query, len(query.Paths))
		for i, path := range query.Paths {
			prefixQuery := bleve.NewPrefixQuery(path)
			prefixQuery.SetField("path")
			pathQueries[i] = prefixQuery
		}
		queries = append(queries, bleve.NewDisjunctionQuery(pathQueries...))
	}

	// 文件类型过滤
	if len(query.FileTypes) > 0 {
		typeQueries := make([]blevequery.Query, len(query.FileTypes))
		for i, ext := range query.FileTypes {
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			termQuery := bleve.NewTermQuery(ext)
			termQuery.SetField("ext")
			typeQueries[i] = termQuery
		}
		queries = append(queries, bleve.NewDisjunctionQuery(typeQueries...))
	}

	// 大小范围过滤
	if query.SizeRange != nil {
		minSize := float64(query.SizeRange.Min)
		maxSize := float64(query.SizeRange.Max)
		rangeQuery := bleve.NewNumericRangeQuery(&minSize, &maxSize)
		rangeQuery.SetField("size")
		queries = append(queries, rangeQuery)
	}

	// 日期范围过滤
	if query.DateRange != nil {
		rangeQuery := bleve.NewDateRangeQuery(query.DateRange.From, query.DateRange.To)
		rangeQuery.SetField("modTime")
		queries = append(queries, rangeQuery)
	}

	// 组合查询
	if len(queries) > 1 {
		return bleve.NewConjunctionQuery(queries...)
	}
	if len(queries) == 1 {
		return queries[0]
	}

	// 默认空查询
	return bleve.NewMatchAllQuery()
}

// shouldExclude 是否应该排除
func (idx *Indexer) shouldExclude(path string) bool {
	// 检查排除路径
	if idx.excludeMap[path] {
		return true
	}

	// 检查隐藏目录
	for _, part := range strings.Split(path, string(os.PathSeparator)) {
		if strings.HasPrefix(part, ".") && part != "." {
			return true
		}
	}

	return false
}

// shouldIndexContent 是否应该索引内容
func (idx *Indexer) shouldIndexContent(path string, size int64) bool {
	if !idx.config.EnableContentIndex {
		return false
	}
	if size > idx.config.MaxContentIndexSize {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return idx.textExts[ext]
}

// readFileContent 读取文件内容
func (idx *Indexer) readFileContent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, idx.config.MaxContentIndexSize)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	return string(buf[:n]), nil
}

// getMimeType 获取 MIME 类型
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
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
	}
	if mt, ok := mimeTypes[strings.ToLower(ext)]; ok {
		return mt
	}
	return "application/octet-stream"
}

// detectProtocol 检测协议类型
func detectProtocol(path string) Protocol {
	// 基于路径特征检测协议
	if strings.HasPrefix(path, "smb://") || strings.HasPrefix(path, "\\\\") {
		return ProtocolSMB
	}
	if strings.HasPrefix(path, "nfs://") || strings.HasPrefix(path, "/net/") {
		return ProtocolNFS
	}
	if strings.HasPrefix(path, "afp://") {
		return ProtocolAFP
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return ProtocolHTTP
	}
	// 默认 SMB（NAS 常用）
	return ProtocolSMB
}

// extractKeywords 提取关键词
func extractKeywords(content string) []string {
	keywords := make([]string, 0)
	wordCount := make(map[string]int)

	// 简单分词
	words := strings.Fields(strings.ToLower(content))
	stopWords := getStopWords()

	for _, word := range words {
		// 清理标点
		word = strings.Trim(word, ".,;:!?\"'()[]{}<>")
		if len(word) >= 3 && !stopWords[word] {
			wordCount[word]++
		}
	}

	// 选择高频词
	for word, count := range wordCount {
		if count >= 2 {
			keywords = append(keywords, word)
		}
		if len(keywords) >= 20 {
			break
		}
	}

	return keywords
}

// getStopWords 获取停用词
func getStopWords() map[string]bool {
	stopWords := make(map[string]bool)
	words := []string{
		"the", "a", "an", "is", "are", "was", "were", "be", "been",
		"have", "has", "had", "do", "does", "did", "will", "would",
		"could", "should", "may", "might", "can", "to", "of", "in",
		"for", "on", "with", "at", "by", "from", "as", "into",
		"and", "but", "or", "if", "not", "no", "this", "that",
	}
	for _, w := range words {
		stopWords[w] = true
	}
	return stopWords
}
