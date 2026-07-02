// Package spotlight 提供 macOS Spotlight 协议兼容的文件搜索服务
// 支持 mDNS/Bonjour 发现、内容索引、亚秒级搜索和跨协议统一搜索
package spotlight

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EngineStatus 引擎状态.
type EngineStatus string

const (
	StatusIdle     EngineStatus = "idle"
	StatusIndexing EngineStatus = "indexing"
	StatusReady    EngineStatus = "ready"
	StatusError    EngineStatus = "error"
)

// Protocol 传输协议类型.
type Protocol string

const (
	ProtocolSMB  Protocol = "smb"
	ProtocolNFS  Protocol = "nfs"
	ProtocolAFP  Protocol = "afp"
	ProtocolHTTP Protocol = "http"
)

// IndexEntry 索引条目.
type IndexEntry struct {
	Path       string            `json:"path"`
	Name       string            `json:"name"`
	Ext        string            `json:"ext"`
	Size       int64             `json:"size"`
	ModTime    time.Time         `json:"modTime"`
	CreateTime time.Time         `json:"createTime"`
	IsDir      bool              `json:"isDir"`
	MimeType   string            `json:"mimeType"`
	Protocol   Protocol          `json:"protocol"`
	Content    string            `json:"content,omitempty"`
	Keywords   []string          `json:"keywords,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Score      float64           `json:"score,omitempty"`
}

// EngineSearchRequest 引擎搜索请求
// 用于 macOS Spotlight 协议兼容的搜索
// 与 types.go 中的 SearchRequest (Web API) 分开

type EngineSearchRequest struct {
	Query      string            `json:"query"`
	Path       string            `json:"path,omitempty"`
	Protocols  []Protocol        `json:"protocols,omitempty"`
	FileTypes  []string          `json:"fileTypes,omitempty"`
	SizeMin    int64             `json:"sizeMin,omitempty"`
	SizeMax    int64             `json:"sizeMax,omitempty"`
	DateStart  time.Time         `json:"dateStart,omitempty"`
	DateEnd    time.Time         `json:"dateEnd,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Limit      int               `json:"limit,omitempty"`
	Offset     int               `json:"offset,omitempty"`
}

// SearchResponse 搜索响应.
type SearchResponse struct {
	Query       string       `json:"query"`
	Results     []IndexEntry `json:"results"`
	Total       int          `json:"total"`
	QueryTimeMs int64        `json:"queryTimeMs"`
	Suggestions []string     `json:"suggestions,omitempty"`
}

// EngineConfig 引擎配置.
type EngineConfig struct {
	// 索引路径
	IndexPath string `json:"indexPath"`
	// 监听路径列表
	IndexPaths []string `json:"indexPaths"`
	// 排除路径列表
	ExcludePaths []string `json:"excludePaths"`
	// 最大索引文件大小（字节）
	MaxFileSize int64 `json:"maxFileSize"`
	// 内容索引最大大小（字节）
	MaxContentIndexSize int64 `json:"maxContentIndexSize"`
	// 并发索引工作者数
	ConcurrentWorkers int `json:"concurrentWorkers"`
	// 批量索引大小
	BatchSize int `json:"batchSize"`
	// 索引更新间隔（秒）
	UpdateInterval int `json:"updateInterval"`
	// 是否启用内容索引
	EnableContentIndex bool `json:"enableContentIndex"`
	// 是否启用 mDNS 发现
	EnableMDNS bool `json:"enableMDNS"`
	// mDNS 服务名
	MDNSServiceName string `json:"mdnsServiceName"`
	// mDNS 端口
	MDNSPort int `json:"mdnsPort"`
	// 缓存大小
	CacheSize int `json:"cacheSize"`
	// 缓存 TTL（秒）
	CacheTTL int `json:"cacheTTL"`
	// 需要索引内容的文本文件扩展名
	TextExtensions []string `json:"textExtensions"`
	// 启用的协议列表
	EnabledProtocols []Protocol `json:"enabledProtocols"`
}

// DefaultEngineConfig 默认配置.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		IndexPath:           "/var/lib/nas-os/spotlight/index.bleve",
		MaxFileSize:         50 * 1024 * 1024, // 50MB
		MaxContentIndexSize: 10 * 1024 * 1024, // 10MB
		ConcurrentWorkers:   4,
		BatchSize:           200,
		UpdateInterval:      300, // 5分钟
		EnableContentIndex:  true,
		EnableMDNS:          true,
		MDNSServiceName:     "NAS-OS Spotlight",
		MDNSPort:            5353,
		CacheSize:           1000,
		CacheTTL:            300, // 5分钟
		TextExtensions: []string{
			".txt", ".md", ".json", ".yaml", ".yml", ".xml", ".html", ".css",
			".js", ".ts", ".go", ".py", ".java", ".c", ".cpp", ".h", ".rs",
			".rb", ".php", ".sh", ".sql", ".conf", ".cfg", ".ini", ".log", ".csv",
		},
		EnabledProtocols: []Protocol{ProtocolSMB, ProtocolNFS, ProtocolAFP},
	}
}

// Engine Spotlight 搜索引擎核心.
type Engine struct {
	config    EngineConfig
	logger    *zap.Logger
	indexer   *Indexer
	parser    *QueryParser
	cache     *SearchCache
	responder *MDNSResponder

	mu     sync.RWMutex
	status EngineStatus
	ctx    context.Context
	cancel context.CancelFunc
	stats  EngineStats
}

// EngineStats 引擎统计.
type EngineStats struct {
	TotalIndexed   int64         `json:"totalIndexed"`
	TotalSearched  int64         `json:"totalSearched"`
	AvgQueryTimeMs float64       `json:"avgQueryTimeMs"`
	IndexSize      int64         `json:"indexSize"`
	LastIndexed    time.Time     `json:"lastIndexed"`
	Uptime         time.Duration `json:"uptime"`
	startTime      time.Time
}

// NewEngine 创建 Spotlight 搜索引擎.
func NewEngine(config EngineConfig, logger *zap.Logger) (*Engine, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		config: config,
		logger: logger,
		status: StatusIdle,
		ctx:    ctx,
		cancel: cancel,
		stats:  EngineStats{startTime: time.Now()},
	}

	// 初始化查询解析器
	e.parser = NewQueryParser()

	// 初始化搜索缓存
	e.cache = NewSearchCache(config.CacheSize, time.Duration(config.CacheTTL)*time.Second)

	// 初始化索引器
	indexer, err := NewIndexer(config, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建索引器失败: %w", err)
	}
	e.indexer = indexer

	// 初始化 mDNS 响应器
	if config.EnableMDNS {
		e.responder = NewMDNSResponder(config.MDNSServiceName, config.MDNSPort, logger)
	}

	return e, nil
}

// Start 启动引擎.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("Spotlight 搜索引擎启动中...")

	// 启动索引器
	if err := e.indexer.Start(ctx); err != nil {
		return fmt.Errorf("启动索引器失败: %w", err)
	}

	// 启动 mDNS 响应器
	if e.responder != nil {
		if err := e.responder.Start(ctx); err != nil {
			e.logger.Warn("mDNS 响应器启动失败", zap.Error(err))
			// mDNS 失败不影响核心功能
		}
	}

	// 启动后台更新
	go e.runIndexUpdater(ctx)

	e.status = StatusReady
	e.stats.startTime = time.Now()

	e.logger.Info("Spotlight 搜索引擎已启动",
		zap.Int("protocols", len(e.config.EnabledProtocols)),
		zap.Bool("mdns", e.config.EnableMDNS))

	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("Spotlight 搜索引擎停止中...")

	e.cancel()

	if e.responder != nil {
		e.responder.Stop()
	}

	e.indexer.Stop()
	e.status = StatusIdle

	e.logger.Info("Spotlight 搜索引擎已停止")
}

// Search 执行搜索.
func (e *Engine) Search(ctx context.Context, req EngineSearchRequest) (*SearchResponse, error) {
	startTime := time.Now()

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	// 检查缓存
	cacheKey := e.cache.GenerateKey(req)
	if cached, ok := e.cache.Get(cacheKey); ok {
		e.logger.Debug("命中搜索缓存", zap.String("query", req.Query))
		return cached, nil
	}

	// 解析查询
	parsed, err := e.parser.Parse(req.Query)
	if err != nil {
		return nil, fmt.Errorf("查询解析失败: %w", err)
	}

	// 应用过滤条件
	if req.Path != "" {
		parsed.Paths = []string{req.Path}
	}
	if len(req.FileTypes) > 0 {
		parsed.FileTypes = req.FileTypes
	}
	if req.SizeMin > 0 || req.SizeMax > 0 {
		parsed.SizeRange = &SizeRange{Min: req.SizeMin, Max: req.SizeMax}
	}
	if !req.DateStart.IsZero() || !req.DateEnd.IsZero() {
		parsed.DateRange = &DateRange{From: req.DateStart, To: req.DateEnd}
	}

	// 执行索引搜索
	results, total, err := e.indexer.Search(ctx, parsed, req.Limit, req.Offset)
	if err != nil {
		return nil, fmt.Errorf("搜索执行失败: %w", err)
	}

	queryTime := time.Since(startTime)

	// 构建响应
	response := &SearchResponse{
		Query:       req.Query,
		Results:     results,
		Total:       total,
		QueryTimeMs: queryTime.Milliseconds(),
	}

	// 如果结果为空，生成建议
	if total == 0 {
		response.Suggestions = e.generateSuggestions(req.Query)
	}

	// 更新缓存
	e.cache.Set(cacheKey, response)

	// 更新统计
	e.updateStats(queryTime)

	e.logger.Debug("搜索完成",
		zap.String("query", req.Query),
		zap.Int("total", total),
		zap.Int64("queryTimeMs", queryTime.Milliseconds()))

	return response, nil
}

// IndexDirectory 索引目录.
func (e *Engine) IndexDirectory(ctx context.Context, path string) error {
	return e.indexer.IndexDirectory(ctx, path)
}

// IndexFile 索引单个文件.
func (e *Engine) IndexFile(ctx context.Context, path string) error {
	return e.indexer.IndexFile(ctx, path)
}

// RemoveFromIndex 从索引中移除.
func (e *Engine) RemoveFromIndex(ctx context.Context, path string) error {
	return e.indexer.RemoveFromIndex(ctx, path)
}

// RebuildIndex 重建索引.
func (e *Engine) RebuildIndex(ctx context.Context) error {
	e.logger.Info("开始重建索引")
	return e.indexer.RebuildIndex(ctx)
}

// GetStatus 获取引擎状态.
func (e *Engine) GetStatus() EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

// GetStats 获取引擎统计.
func (e *Engine) GetStats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stats := e.stats
	stats.Uptime = time.Since(stats.startTime)
	stats.TotalIndexed = e.indexer.GetIndexedCount()
	return stats
}

// GetIndexStatus 获取索引状态.
func (e *Engine) GetIndexStatus() IndexStatus {
	return e.indexer.GetStatus()
}

// runIndexUpdater 后台索引更新.
func (e *Engine) runIndexUpdater(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(e.config.UpdateInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.refreshIndexes(ctx)
		}
	}
}

// refreshIndexes 刷新所有索引.
func (e *Engine) refreshIndexes(ctx context.Context) {
	e.mu.RLock()
	paths := e.config.IndexPaths
	e.mu.RUnlock()

	for _, path := range paths {
		if err := e.indexer.IndexDirectory(ctx, path); err != nil {
			e.logger.Error("刷新索引失败",
				zap.String("path", path),
				zap.Error(err))
		}
	}

	e.mu.Lock()
	e.stats.LastIndexed = time.Now()
	e.mu.Unlock()
}

// generateSuggestions 生成搜索建议.
func (e *Engine) generateSuggestions(query string) []string {
	suggestions := []string{}

	// 基于常见模式的建议
	if len(query) > 2 {
		suggestions = append(suggestions, fmt.Sprintf("name:%s", query))
		suggestions = append(suggestions, fmt.Sprintf("content:%s", query))
	}

	// 文件类型建议
	typeSuggestions := map[string][]string{
		"文档": {"type:pdf", "type:doc", "type:docx", "type:txt"},
		"图片": {"type:jpg", "type:png", "type:gif"},
		"视频": {"type:mp4", "type:mkv", "type:avi"},
		"音频": {"type:mp3", "type:flac", "type:wav"},
	}

	for key, types := range typeSuggestions {
		if contains(query, key) {
			for _, t := range types {
				suggestions = append(suggestions, t+" "+query)
			}
		}
	}

	return suggestions
}

// updateStats 更新统计信息.
func (e *Engine) updateStats(queryTime time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.TotalSearched++

	// 计算平均查询时间
	if e.stats.TotalSearched == 1 {
		e.stats.AvgQueryTimeMs = float64(queryTime.Milliseconds())
	} else {
		e.stats.AvgQueryTimeMs = (e.stats.AvgQueryTimeMs*float64(e.stats.TotalSearched-1) +
			float64(queryTime.Milliseconds())) / float64(e.stats.TotalSearched)
	}
}

// contains 检查字符串是否包含子串（忽略大小写）.
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
