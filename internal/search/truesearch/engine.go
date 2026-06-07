// Package truesearch 实现全文搜索引擎 (TrueSearch Phase 2)
// 支持文件内容索引、全文搜索、多格式内容提取。
package truesearch

import (
	"go.uber.org/zap"
)

// Engine 是 TrueSearch 全文搜索引擎的核心结构。
type Engine struct {
	config    Config
	logger    *zap.Logger
	indexer   *Indexer
	extractor *Extractor
	api       *APIHandler
}

// Config 是 TrueSearch 引擎配置。
type Config struct {
	IndexPath     string   `json:"indexPath"`     // 索引存储路径
	MaxFileSize   int64    `json:"maxFileSize"`   // 最大索引文件大小 (bytes)
	BatchSize     int      `json:"batchSize"`     // 批量索引大小
	SupportedExts []string `json:"supportedExts"` // 支持索引内容的扩展名
	ExcludeDirs   []string `json:"excludeDirs"`   // 排除目录
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		IndexPath:   "/var/lib/nas-os/truesearch/index.bleve",
		MaxFileSize: 50 * 1024 * 1024, // 50MB
		BatchSize:   100,
		SupportedExts: []string{
			".txt", ".md", ".markdown", ".pdf", ".docx",
			".json", ".yaml", ".yml", ".xml", ".csv",
			".log", ".conf", ".cfg", ".ini", ".env",
		},
		ExcludeDirs: []string{
			".git", ".svn", ".hg", "node_modules", "vendor", "tmp", "temp", "cache",
		},
	}
}

// New 创建 TrueSearch 引擎实例。
func New(cfg Config, logger *zap.Logger) (*Engine, error) {
	if cfg.IndexPath == "" {
		cfg = DefaultConfig()
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}

	extractor := NewExtractor(cfg.MaxFileSize, logger)
	indexer, err := NewIndexer(cfg, logger)
	if err != nil {
		return nil, err
	}

	engine := &Engine{
		config:    cfg,
		logger:    logger,
		indexer:   indexer,
		extractor: extractor,
	}
	engine.api = NewAPIHandler(engine, logger)

	return engine, nil
}

// APIHandler 返回 REST API handler。
func (e *Engine) APIHandler() *APIHandler {
	return e.api
}

// IndexFile 索引单个文件。
func (e *Engine) IndexFile(path string) error {
	return e.indexer.IndexFile(path, e.extractor)
}

// IndexDirectory 递归索引目录。
func (e *Engine) IndexDirectory(root string) error {
	return e.indexer.IndexDirectory(root, e.extractor)
}

// Search 执行全文搜索。
func (e *Engine) Search(req SearchRequest) (*SearchResponse, error) {
	return e.indexer.Search(req)
}

// Status 获取索引状态。
func (e *Engine) Status() IndexStatus {
	return e.indexer.Status()
}

// Reindex 重建索引。
func (e *Engine) Reindex(path string, force bool) error {
	return e.indexer.Reindex(path, force, e.extractor)
}

// Close 关闭引擎。
func (e *Engine) Close() error {
	return e.indexer.Close()
}
