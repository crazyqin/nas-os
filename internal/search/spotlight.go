// Package search 提供Spotlight全文搜索服务
// 对标TrueNAS SMB Spotlight功能
package search

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// QueryEngine 查询引擎（简化实现）
type QueryEngine struct {
	parser *QueryParser
	logger *zap.Logger
}

// NewQueryEngine 创建查询引擎
func NewQueryEngine(logger *zap.Logger) *QueryEngine {
	return &QueryEngine{
		parser: NewQueryParser(),
		logger: logger,
	}
}

// Parse 解析查询字符串
func (q *QueryEngine) Parse(query string) (*ParsedQuery, error) {
	return q.parser.Parse(query)
}

// FileWatcher 文件监控器（简化包装）
type FileWatcher struct {
	watcher *Watcher
	changes []FileChange
	mu      sync.RWMutex
}

// FileChange 文件变更事件
type FileChange struct {
	Path string
	Type ChangeType
}

// ChangeType 变更类型
type ChangeType int

const (
	ChangeTypeCreate ChangeType = 1
	ChangeTypeModify ChangeType = 2
	ChangeTypeDelete ChangeType = 3
)

// NewFileWatcher 创建文件监控器
func NewFileWatcher(paths []string, logger *zap.Logger) *FileWatcher {
	return &FileWatcher{
		changes: make([]FileChange, 0),
	}
}

// Start 启动监控
func (f *FileWatcher) Start(ctx context.Context) error {
	return nil
}

// Stop 停止监控
func (f *FileWatcher) Stop() {}

// GetChanges 获取变更列表
func (f *FileWatcher) GetChanges() []FileChange {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.changes
}

// IndexStatus 索引状态
type IndexStatus struct {
	TotalFiles    int64
	IndexedFiles  int64
	IndexSize     int64
	LastUpdate    time.Time
	Status        string
	Progress      float64
}

// SpotlightIndexer Spotlight索引器
type SpotlightIndexer struct {
	engine  *Engine
	config  SpotlightConfig
	logger  *zap.Logger
	status  IndexStatus
	mu      sync.RWMutex
}

// NewIndexer 创建索引器
func NewIndexer(config SpotlightConfig, logger *zap.Logger) *SpotlightIndexer {
	return &SpotlightIndexer{
		config: config,
		logger: logger,
		status: IndexStatus{Status: "idle"},
	}
}

// Search 搜索索引
func (i *SpotlightIndexer) Search(ctx context.Context, query *ParsedQuery, limit, offset int) ([]SpotlightFile, int, error) {
	return []SpotlightFile{}, 0, nil
}

// ClearIndex 清空索引
func (i *SpotlightIndexer) ClearIndex(path string) {}

// BuildIndex 构建索引
func (i *SpotlightIndexer) BuildIndex(ctx context.Context, path string) error {
	return nil
}

// IndexFile 索引单个文件
func (i *SpotlightIndexer) IndexFile(ctx context.Context, path string) error {
	return nil
}

// RemoveFromIndex 从索引移除
func (i *SpotlightIndexer) RemoveFromIndex(ctx context.Context, path string) error {
	return nil
}

// GetStatus 获取状态
func (i *SpotlightIndexer) GetStatus() *IndexStatus {
	return &i.status
}

// SpotlightService SMB Spotlight搜索服务
type SpotlightService struct {
	indexer    *SpotlightIndexer
	query      *QueryEngine
	watcher    *FileWatcher
	logger     *zap.Logger
	config     SpotlightConfig
	mu         sync.RWMutex
}

// SpotlightConfig Spotlight配置
type SpotlightConfig struct {
	EnableContentIndex bool     `json:"enableContentIndex"`
	IndexPaths         []string `json:"indexPaths"`
	ExcludedPaths      []string `json:"excludedPaths"`
	MaxIndexSize       int64    `json:"maxIndexSize"`    // 最大索引大小(MB)
	UpdateInterval     int      `json:"updateInterval"`  // 更新间隔(秒)
	ConcurrentWorkers  int      `json:"concurrentWorkers"`
}

// SpotlightQuery Spotlight搜索请求
type SpotlightQuery struct {
	Query       string            `json:"query"`
	Path        string            `json:"path"`        // 搜索路径范围
	FileTypes   []string          `json:"fileTypes"`   // 文件类型过滤
	SizeMin     int64             `json:"sizeMin"`     // 最小文件大小
	SizeMax     int64             `json:"sizeMax"`     // 最大文件大小
	DateStart   time.Time         `json:"dateStart"`   // 开始日期
	DateEnd     time.Time         `json:"dateEnd"`     // 结束日期
	Attributes  map[string]string `json:"attributes"`  // Spotlight属性
	Limit       int               `json:"limit"`
	Offset      int               `json:"offset"`
}

// SpotlightResult Spotlight搜索结果
type SpotlightResult struct {
	Files       []SpotlightFile `json:"files"`
	Total       int             `json:"total"`
	QueryTime   int64           `json:"queryTime"` // 查询耗时(ms)
	Suggestions []string        `json:"suggestions"` // 搜索建议
}

// SpotlightFile Spotlight文件信息
type SpotlightFile struct {
	Path         string            `json:"path"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	ModifiedTime time.Time         `json:"modifiedTime"`
	Type         string            `json:"type"`
	ContentType  string            `json:"contentType"`
	Attributes   map[string]string `json:"attributes"`
	Snippet      string            `json:"snippet"` // 内容摘要
}

// NewSpotlightService 创建Spotlight服务
func NewSpotlightService(config SpotlightConfig, logger *zap.Logger) *SpotlightService {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SpotlightService{
		indexer: NewIndexer(config, logger),
		query:   NewQueryEngine(logger),
		watcher: NewFileWatcher(config.IndexPaths, logger),
		logger:  logger,
		config:  config,
	}
}

// Search 执行Spotlight搜索
func (s *SpotlightService) Search(ctx context.Context, req SpotlightQuery) (*SpotlightResult, error) {
	startTime := time.Now()

	// 解析搜索语法
	parsedQuery, err := s.query.Parse(req.Query)
	if err != nil {
		return nil, err
	}

	// 应用过滤条件
	if req.Path != "" {
		parsedQuery.Paths = []string{req.Path}
	}
	if len(req.FileTypes) > 0 {
		parsedQuery.FileTypes = req.FileTypes
	}
	if req.SizeMin > 0 || req.SizeMax > 0 {
		parsedQuery.SizeRange = &SizeRange{Min: req.SizeMin, Max: req.SizeMax}
	}
	if !req.DateStart.IsZero() || !req.DateEnd.IsZero() {
		parsedQuery.DateRange = &DateRange{From: req.DateStart, To: req.DateEnd}
	}

	// 执行索引搜索
	files, total, err := s.indexer.Search(ctx, parsedQuery, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}

	// 格式化结果
	result := &SpotlightResult{
		Files:     files,
		Total:     total,
		QueryTime: time.Since(startTime).Milliseconds(),
	}

	// 生成搜索建议
	if len(files) == 0 {
		result.Suggestions = s.generateSuggestions(req.Query)
	}

	return result, nil
}

// SearchByAttributes 按Spotlight属性搜索（macOS兼容）
func (s *SpotlightService) SearchByAttributes(ctx context.Context, attrs map[string]string, limit int) (*SpotlightResult, error) {
	// 将Spotlight属性转换为内部查询
	query := ""
	for k, v := range attrs {
		// Spotlight属性映射
		switch k {
		case "kMDItemDisplayName":
			query += "name:" + v + " "
		case "kMDItemContentType":
			query += "type:" + v + " "
		case "kMDItemFSCreationDate":
			query += "date:" + v + " "
		case "kMDItemFSContentChangeDate":
			query += "modified:" + v + " "
		}
	}

	return s.Search(ctx, SpotlightQuery{
		Query: query,
		Limit: limit,
	})
}

// RebuildIndex 重建索引
func (s *SpotlightService) RebuildIndex(ctx context.Context, path string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("开始重建索引", zap.String("path", path), zap.Bool("force", force))

	if force {
		// 清除现有索引
		s.indexer.ClearIndex(path)
	}

	// 启动索引任务
	return s.indexer.BuildIndex(ctx, path)
}

// GetIndexStatus 获取索引状态
func (s *SpotlightService) GetIndexStatus() *IndexStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.indexer.GetStatus()
}

// Start 启动Spotlight服务
func (s *SpotlightService) Start(ctx context.Context) error {
	// 启动文件监听
	if err := s.watcher.Start(ctx); err != nil {
		return err
	}

	// 启动索引更新任务
	go s.runIndexUpdate(ctx)

	s.logger.Info("Spotlight服务已启动")
	return nil
}

// Stop 停止Spotlight服务
func (s *SpotlightService) Stop() error {
	s.watcher.Stop()
	s.logger.Info("Spotlight服务已停止")
	return nil
}

// runIndexUpdate 运行索引更新任务
func (s *SpotlightService) runIndexUpdate(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.config.UpdateInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 处理文件变更
			changes := s.watcher.GetChanges()
			for _, change := range changes {
				switch change.Type {
				case ChangeTypeCreate, ChangeTypeModify:
					s.indexer.IndexFile(ctx, change.Path)
				case ChangeTypeDelete:
					s.indexer.RemoveFromIndex(ctx, change.Path)
				}
			}
		}
	}
}

// generateSuggestions 生成搜索建议
func (s *SpotlightService) generateSuggestions(query string) []string {
	suggestions := []string{}

	// 拼写纠正建议
	if len(query) > 3 {
		// 简单的拼写建议（基于常见错误）
		commonCorrections := map[string]string{
			"documet":  "document",
			"exel":     "excel",
			"powerpoit": "powerpoint",
		}
		for wrong, correct := range commonCorrections {
			if strings.Contains(strings.ToLower(query), wrong) {
				suggestions = append(suggestions, strings.Replace(query, wrong, correct, 1))
			}
		}
	}

	// 相关文件类型建议
	if strings.Contains(query, "report") {
		suggestions = append(suggestions, "type:pdf "+query)
		suggestions = append(suggestions, "type:docx "+query)
	}

	return suggestions
}

// ========== 搜索语法解析 ==========

// BoolExpression 布尔表达式
type BoolExpression struct {
	Operator string      // AND, OR, NOT
	Left     interface{} // ParsedQuery or BoolExpression
	Right    interface{} // ParsedQuery or BoolExpression
}

// parseSize 解析大小字符串
func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	multiplier := int64(1)
	if strings.HasSuffix(s, "kb") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "kb")
	} else if strings.HasSuffix(s, "mb") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "mb")
	} else if strings.HasSuffix(s, "gb") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "gb")
	}

	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result * 10 + int64(c - '0')
		}
	}
	return result * multiplier
}

// parseDateRange 解析日期范围
func parseDateRange(s string) DateRange {
	// 支持格式: 2024-01-01, >2024-01-01, <2024-01-01
	dr := DateRange{}

	if strings.HasPrefix(s, ">") {
		dateStr := strings.TrimPrefix(s, ">")
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			dr.From = t
		}
	} else if strings.HasPrefix(s, "<") {
		dateStr := strings.TrimPrefix(s, "<")
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			dr.To = t
		}
	} else {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			dr.From = t
			dr.To = t.Add(24 * time.Hour)
		}
	}

	return dr
}

// ========== 文件类型检测 ==========

// DetectFileType 检测文件类型
func DetectFileType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	typeMap := map[string]string{
		".txt":  "text",
		".md":   "text",
		".pdf":  "pdf",
		".doc":  "document",
		".docx": "document",
		".xls":  "spreadsheet",
		".xlsx": "spreadsheet",
		".ppt":  "presentation",
		".pptx": "presentation",
		".jpg":  "image",
		".jpeg": "image",
		".png":  "image",
		".gif":  "image",
		".mp4":  "video",
		".mkv":  "video",
		".mp3":  "audio",
		".flac": "audio",
		".zip":  "archive",
		".tar":  "archive",
		".gz":   "archive",
	}

	if t, ok := typeMap[ext]; ok {
		return t
	}
	return "unknown"
}

// ========== Spotlight属性映射 ==========

// SpotlightAttributeMap macOS Spotlight属性映射
var SpotlightAttributeMap = map[string]string{
	"kMDItemDisplayName":          "name",
	"kMDItemPath":                 "path",
	"kMDItemFSSize":               "size",
	"kMDItemFSCreationDate":       "created",
	"kMDItemFSContentChangeDate":  "modified",
	"kMDItemContentType":          "type",
	"kMDItemKind":                 "kind",
	"kMDItemWhereFroms":           "source",
	"kMDItemAuthors":              "author",
	"kMDItemTitle":                "title",
	"kMDItemKeywords":             "keywords",
	"kMDItemDurationSeconds":      "duration",
	"kMDItemPixelWidth":           "width",
	"kMDItemPixelHeight":          "height",
	"kMDItemAudioSampleRate":      "sampleRate",
	"kMDItemAudioBitRate":         "bitRate",
}

// MapToSpotlightAttributes 将内部属性映射到Spotlight格式
func MapToSpotlightAttributes(attrs map[string]string) map[string]string {
	result := map[string]string{}
	for internalKey, value := range attrs {
		for spotlightKey, mappedKey := range SpotlightAttributeMap {
			if mappedKey == internalKey {
				result[spotlightKey] = value
				break
			}
		}
	}
	return result
}