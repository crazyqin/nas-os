// Package truesearch 管理器实现
package truesearch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager TrueSearch 全文搜索引擎管理器.
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *TrueSearchConfig
	indexer  *Indexer
	searcher *Searcher
	stopChan chan struct{}
	running  bool
}

// NewManager 创建管理器.
func NewManager(logger *zap.Logger, config *TrueSearchConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultTrueSearchConfig()
	}

	indexer := NewIndexer(logger, config)
	searcher := NewSearcher(logger, config, indexer)

	return &Manager{
		logger:   logger,
		config:   config,
		indexer:  indexer,
		searcher: searcher,
		stopChan: make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("manager is already running")
	}

	if !m.config.Enabled {
		return fmt.Errorf("TrueSearch is disabled")
	}

	m.running = true
	m.logger.Info("TrueSearch manager started",
		zap.String("index_dir", m.config.IndexDir))

	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	close(m.stopChan)
	m.running = false
	m.logger.Info("TrueSearch manager stopped")

	return nil
}

// IsRunning 检查是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// IndexDocument 索引文档.
func (m *Manager) IndexDocument(doc *Document) error {
	if !m.IsRunning() {
		return fmt.Errorf("manager is not running")
	}

	return m.indexer.IndexDocument(doc)
}

// IndexBatch 批量索引文档.
func (m *Manager) IndexBatch(docs []*Document) (int, error) {
	if !m.IsRunning() {
		return 0, fmt.Errorf("manager is not running")
	}

	indexed := 0
	for _, doc := range docs {
		if err := m.indexer.IndexDocument(doc); err != nil {
			m.logger.Warn("failed to index document",
				zap.String("path", doc.Path),
				zap.Error(err))
			continue
		}
		indexed++
	}

	m.logger.Info("batch indexing completed",
		zap.Int("total", len(docs)),
		zap.Int("indexed", indexed))

	return indexed, nil
}

// RemoveDocument 移除文档.
func (m *Manager) RemoveDocument(docID string) error {
	if !m.IsRunning() {
		return fmt.Errorf("manager is not running")
	}

	return m.indexer.RemoveDocument(docID)
}

// Search 执行搜索.
func (m *Manager) Search(query *SearchQuery) (*SearchResponse, error) {
	if !m.IsRunning() {
		return nil, fmt.Errorf("manager is not running")
	}

	return m.searcher.Search(query)
}

// SearchFilename 搜索文件名.
func (m *Manager) SearchFilename(query string, limit int) (*SearchResponse, error) {
	return m.Search(&SearchQuery{
		Query: query,
		Mode:  SearchModeFilename,
		Limit: limit,
	})
}

// SearchContent 搜索内容.
func (m *Manager) SearchContent(query string, limit int) (*SearchResponse, error) {
	return m.Search(&SearchQuery{
		Query: query,
		Mode:  SearchModeContent,
		Limit: limit,
	})
}

// AutoComplete 自动补全.
func (m *Manager) AutoComplete(prefix string, limit int) []string {
	if !m.IsRunning() {
		return nil
	}

	return m.searcher.AutoComplete(prefix, limit)
}

// GetDocument 获取文档.
func (m *Manager) GetDocument(docID string) (*Document, error) {
	return m.indexer.GetDocument(docID)
}

// ListDocuments 列出文档.
func (m *Manager) ListDocuments(limit int) []*Document {
	return m.indexer.ListDocuments(limit)
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() *IndexStats {
	stats := m.indexer.GetStats()
	stats.Status = IndexStatusReady
	if !m.IsRunning() {
		stats.Status = IndexStatusPending
	}
	return stats
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *TrueSearchConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config *TrueSearchConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	m.logger.Info("TrueSearch config updated")

	return nil
}

// RebuildIndex 重建索引.
func (m *Manager) RebuildIndex(ctx context.Context, docs []*Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("rebuilding index",
		zap.Int("documents", len(docs)))

	start := time.Now()

	// 清空索引
	m.indexer = NewIndexer(m.logger, m.config)
	m.searcher = NewSearcher(m.logger, m.config, m.indexer)

	// 重新索引
	indexed := 0
	for _, doc := range docs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := m.indexer.IndexDocument(doc); err != nil {
			m.logger.Warn("failed to index document during rebuild",
				zap.String("path", doc.Path),
				zap.Error(err))
			continue
		}
		indexed++
	}

	m.logger.Info("index rebuild completed",
		zap.Int("indexed", indexed),
		zap.Duration("duration", time.Since(start)))

	return nil
}

// GetIndexer 获取索引器.
func (m *Manager) GetIndexer() *Indexer {
	return m.indexer
}

// GetSearcher 获取搜索引擎.
func (m *Manager) GetSearcher() *Searcher {
	return m.searcher
}
