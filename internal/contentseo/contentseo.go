// Package contentseo 提供内容搜索引擎优化功能
package contentseo

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Engine 内容 SEO 引擎.
type Engine struct {
	indexer    *Indexer
	stats      *SearchStats
	indexState *IndexStatusInfo
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// Config 配置.
type Config struct {
	IndexPath    string // 索引存储路径
	MaxIndexSize int64  // 最大索引大小
	BatchSize    int    // 批处理大小
	Workers      int    // 工作线程数
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		IndexPath:    "/var/lib/nas-os/contentseo/index",
		MaxIndexSize: 1024 * 1024 * 100, // 100MB
		BatchSize:    100,
		Workers:      4,
	}
}

// NewEngine 创建引擎实例.
func NewEngine(config *Config) *Engine {
	if config == nil {
		config = DefaultConfig()
	}

	engine := &Engine{
		stats: &SearchStats{
			TopQueries: make([]QueryStat, 0),
		},
		indexState: &IndexStatusInfo{
			Status:    IndexStatusIdle,
			UpdatedAt: time.Now(),
		},
		stopCh: make(chan struct{}),
	}

	engine.indexer = NewIndexer(engine, config)

	return engine
}

// Search 执行搜索.
func (e *Engine) Search(query SearchQuery) (*SearchResult, error) {
	if query.Keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}

	start := time.Now()

	// 规范化分页参数
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	// 记录查询统计
	e.recordQuery(query.Keyword)

	// 执行搜索
	results, total, err := e.indexer.Search(query)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Items: results,
		Total: total,
		Page:  query.Page,
		Took:  time.Since(start),
	}, nil
}

// GetStats 获取搜索统计.
func (e *Engine) GetStats() *SearchStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	stats.TotalIndexed = e.indexState.IndexedFiles
	stats.LastIndexed = e.indexState.UpdatedAt

	return &stats
}

// GetIndexStatus 获取索引状态.
func (e *Engine) GetIndexStatus() *IndexStatusInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := *e.indexState
	return &status
}

// RebuildIndex 重建索引.
func (e *Engine) RebuildIndex(fullRebuild bool) error {
	e.mu.Lock()
	if e.indexState.Status == IndexStatusIndexing || e.indexState.Status == IndexStatusRebuilding {
		e.mu.Unlock()
		return fmt.Errorf("索引正在进行中")
	}
	e.indexState.Status = IndexStatusRebuilding
	now := time.Now()
	e.indexState.StartedAt = &now
	e.indexState.Progress = 0
	e.mu.Unlock()

	go func() {
		if err := e.indexer.Rebuild(fullRebuild); err != nil {
			log.Printf("索引重建失败: %v", err)
			e.mu.Lock()
			e.indexState.Status = IndexStatusFailed
			e.mu.Unlock()
			return
		}

		e.mu.Lock()
		e.indexState.Status = IndexStatusIdle
		e.indexState.Progress = 100
		e.indexState.UpdatedAt = time.Now()
		e.mu.Unlock()
	}()

	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() {
	close(e.stopCh)
	e.indexer.Stop()
}

// recordQuery 记录查询统计.
func (e *Engine) recordQuery(keyword string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 查找是否已存在
	for i, qs := range e.stats.TopQueries {
		if qs.Keyword == keyword {
			e.stats.TopQueries[i].Count++
			return
		}
	}

	// 新增查询
	e.stats.TopQueries = append(e.stats.TopQueries, QueryStat{
		Keyword: keyword,
		Count:   1,
	})

	// 保留 top 10
	if len(e.stats.TopQueries) > 10 {
		// 按次数排序
		for i := 0; i < len(e.stats.TopQueries)-1; i++ {
			for j := i + 1; j < len(e.stats.TopQueries); j++ {
				if e.stats.TopQueries[j].Count > e.stats.TopQueries[i].Count {
					e.stats.TopQueries[i], e.stats.TopQueries[j] = e.stats.TopQueries[j], e.stats.TopQueries[i]
				}
			}
		}
		e.stats.TopQueries = e.stats.TopQueries[:10]
	}
}
