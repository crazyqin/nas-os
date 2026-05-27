package fileretriever

import (
	"fmt"
	"sync"
	"time"
)

// FileRetrieverManager 文件检索管理器
type FileRetrieverManager struct {
	mu       sync.RWMutex
	index    map[string]*FileEntry
	searches map[string]*SearchTask
	config   *RetrieverConfig
}

// RetrieverConfig 检索配置
type RetrieverConfig struct {
	IndexPath     string `json:"index_path"`
	MaxResults    int    `json:"max_results"`
	IndexInterval int    `json:"index_interval_hours"`
}

// FileEntry 文件索引条目
type FileEntry struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Extension string    `json:"extension"`
	ModTime   time.Time `json:"mod_time"`
	IsDir     bool      `json:"is_dir"`
	Hash      string    `json:"hash,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	IndexedAt time.Time `json:"indexed_at"`
}

// SearchTask 搜索任务
type SearchTask struct {
	ID        string        `json:"id"`
	Query     string        `json:"query"`
	Status    SearchStatus  `json:"status"`
	Results   []*FileEntry  `json:"results"`
	Total     int           `json:"total"`
	Duration  time.Duration `json:"duration"`
	CreatedAt time.Time     `json:"created_at"`
}

// SearchStatus 搜索状态
type SearchStatus string

const (
	SearchStatusPending  SearchStatus = "pending"
	SearchStatusRunning  SearchStatus = "running"
	SearchStatusComplete SearchStatus = "complete"
	SearchStatusFailed   SearchStatus = "failed"
)

// SearchRequest 搜索请求
type SearchRequest struct {
	Query     string   `json:"query"`
	Path      string   `json:"path,omitempty"`
	Extension string   `json:"extension,omitempty"`
	MinSize   int64    `json:"min_size,omitempty"`
	MaxSize   int64    `json:"max_size,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

// NewFileRetrieverManager 创建管理器
func NewFileRetrieverManager(config *RetrieverConfig) *FileRetrieverManager {
	if config == nil {
		config = &RetrieverConfig{
			IndexPath:     "/var/lib/nas-os/index",
			MaxResults:    100,
			IndexInterval: 24,
		}
	}
	return &FileRetrieverManager{
		index:    make(map[string]*FileEntry),
		searches: make(map[string]*SearchTask),
		config:   config,
	}
}

// IndexFile 索引文件
func (m *FileRetrieverManager) IndexFile(entry *FileEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.Path == "" {
		return fmt.Errorf("文件路径不能为空")
	}

	entry.IndexedAt = time.Now()
	m.index[entry.Path] = entry
	return nil
}

// IndexBatch 批量索引
func (m *FileRetrieverManager) IndexBatch(entries []*FileEntry) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, entry := range entries {
		if entry.Path != "" {
			entry.IndexedAt = time.Now()
			m.index[entry.Path] = entry
			count++
		}
	}

	return count, nil
}

// GetEntry 获取索引条目
func (m *FileRetrieverManager) GetEntry(path string) (*FileEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.index[path]
	if !exists {
		return nil, fmt.Errorf("文件未索引: %s", path)
	}
	return entry, nil
}

// RemoveEntry 移除索引条目
func (m *FileRetrieverManager) RemoveEntry(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.index[path]; !exists {
		return fmt.Errorf("文件未索引: %s", path)
	}

	delete(m.index, path)
	return nil
}

// Search 搜索文件
func (m *FileRetrieverManager) Search(req *SearchRequest) (*SearchTask, error) {
	m.mu.Lock()

	task := &SearchTask{
		ID:        fmt.Sprintf("search_%d", time.Now().UnixNano()),
		Query:     req.Query,
		Status:    SearchStatusRunning,
		CreatedAt: time.Now(),
	}

	m.searches[task.ID] = task
	m.mu.Unlock()

	// 执行搜索
	startTime := time.Now()
	var results []*FileEntry

	m.mu.RLock()
	for _, entry := range m.index {
		if m.matchesSearch(entry, req) {
			results = append(results, entry)
			if req.Limit > 0 && len(results) >= req.Limit {
				break
			}
		}
	}
	m.mu.RUnlock()

	m.mu.Lock()
	task.Results = results
	task.Total = len(results)
	task.Status = SearchStatusComplete
	task.Duration = time.Since(startTime)
	m.mu.Unlock()

	return task, nil
}

// matchesSearch 检查是否匹配搜索条件
func (m *FileRetrieverManager) matchesSearch(entry *FileEntry, req *SearchRequest) bool {
	// 检查路径前缀
	if req.Path != "" && !startsWith(entry.Path, req.Path) {
		return false
	}

	// 检查扩展名
	if req.Extension != "" && entry.Extension != req.Extension {
		return false
	}

	// 检查大小
	if req.MinSize > 0 && entry.Size < req.MinSize {
		return false
	}
	if req.MaxSize > 0 && entry.Size > req.MaxSize {
		return false
	}

	// 检查标签
	if len(req.Tags) > 0 {
		hasTag := false
		for _, tag := range req.Tags {
			for _, entryTag := range entry.Tags {
				if tag == entryTag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// 检查查询词
	if req.Query != "" {
		return contains(entry.Name, req.Query) || contains(entry.Path, req.Query)
	}

	return true
}

// GetSearchTask 获取搜索任务
func (m *FileRetrieverManager) GetSearchTask(id string) (*SearchTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.searches[id]
	if !exists {
		return nil, fmt.Errorf("搜索任务不存在: %s", id)
	}
	return task, nil
}

// ListSearchTasks 列出搜索任务
func (m *FileRetrieverManager) ListSearchTasks() []*SearchTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*SearchTask, 0, len(m.searches))
	for _, task := range m.searches {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetStats 获取统计信息
func (m *FileRetrieverManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_indexed":  len(m.index),
		"total_searches": len(m.searches),
	}
}

// ClearIndex 清空索引
func (m *FileRetrieverManager) ClearIndex() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.index = make(map[string]*FileEntry)
}

// 辅助函数
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
