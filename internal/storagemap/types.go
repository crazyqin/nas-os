package storagemap

import (
	"fmt"
	"sync"
	"time"
)

// StorageMapManager 存储空间地图管理器
type StorageMapManager struct {
	mu       sync.RWMutex
	trees    map[string]*StorageTree
	scanJobs map[string]*ScanJob
	config   *MapConfig
}

// MapConfig 地图配置
type MapConfig struct {
	ScanInterval    time.Duration `json:"scan_interval"`
	MaxDepth        int           `json:"max_depth"`
	MinFileSize     int64         `json:"min_file_size"`
	ExcludePatterns []string      `json:"exclude_patterns"`
}

// StorageTree 存储树
type StorageTree struct {
	ID        string       `json:"id"`
	Path      string       `json:"path"`
	Name      string       `json:"name"`
	Size      int64         `json:"size"`
	FileCount int64         `json:"file_count"`
	DirCount  int64         `json:"dir_count"`
	Children  []*StorageTree `json:"children,omitempty"`
	Parent    string       `json:"parent,omitempty"`
	ModTime   time.Time    `json:"mod_time"`
	FileType  string       `json:"file_type"`
	Usage     float64      `json:"usage_percent"`
}

// ScanJob 扫描任务
type ScanJob struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Status    string    `json:"status"`
	Progress  float64   `json:"progress"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// TreemapNode 树图节点
type TreemapNode struct {
	Name     string         `json:"name"`
	Size     int64          `json:"size"`
	Children []*TreemapNode `json:"children,omitempty"`
	Color    string         `json:"color"`
	Percent  float64        `json:"percent"`
}

// NewStorageMapManager 创建存储地图管理器
func NewStorageMapManager(config *MapConfig) *StorageMapManager {
	if config == nil {
		config = &MapConfig{
			ScanInterval: 24 * time.Hour,
			MaxDepth:     10,
			MinFileSize:  0,
		}
	}
	return &StorageMapManager{
		trees:    make(map[string]*StorageTree),
		scanJobs: make(map[string]*ScanJob),
		config:   config,
	}
}

// StartScan 启动扫描
func (m *StorageMapManager) StartScan(path string) (*ScanJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobID := fmt.Sprintf("scan_%d", time.Now().UnixNano())
	job := &ScanJob{
		ID:        jobID,
		Path:      path,
		Status:    "running",
		Progress:  0,
		StartTime: time.Now(),
	}
	m.scanJobs[jobID] = job

	// 异步执行扫描
	go m.executeScan(job)

	return job, nil
}

// executeScan 执行扫描
func (m *StorageMapManager) executeScan(job *ScanJob) {
	// 模拟扫描过程
	for i := 0; i <= 100; i += 10 {
		m.mu.Lock()
		job.Progress = float64(i)
		m.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	m.mu.Lock()
	job.Status = "completed"
	job.Progress = 100
	job.EndTime = time.Now()
	m.mu.Unlock()
}

// GetScanJob 获取扫描任务状态
func (m *StorageMapManager) GetScanJob(jobID string) (*ScanJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.scanJobs[jobID]
	if !exists {
		return nil, fmt.Errorf("scan job not found: %s", jobID)
	}
	return job, nil
}

// GetStorageTree 获取存储树
func (m *StorageMapManager) GetStorageTree(path string) (*StorageTree, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tree, exists := m.trees[path]
	if !exists {
		return nil, fmt.Errorf("storage tree not found: %s", path)
	}
	return tree, nil
}

// GenerateTreemap 生成树图数据
func (m *StorageMapManager) GenerateTreemap(path string) (*TreemapNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tree, exists := m.trees[path]
	if !exists {
		return nil, fmt.Errorf("storage tree not found: %s", path)
	}

	return m.convertToTreemapNode(tree), nil
}

// convertToTreemapNode 转换为树图节点
func (m *StorageMapManager) convertToTreemapNode(tree *StorageTree) *TreemapNode {
	node := &TreemapNode{
		Name:    tree.Name,
		Size:    tree.Size,
		Percent: tree.Usage,
	}

	if len(tree.Children) > 0 {
		node.Children = make([]*TreemapNode, len(tree.Children))
		for i, child := range tree.Children {
			node.Children[i] = m.convertToTreemapNode(child)
		}
	}

	return node
}

// GetLargeFiles 获取大文件列表
func (m *StorageMapManager) GetLargeFiles(path string, limit int) ([]*StorageTree, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tree, exists := m.trees[path]
	if !exists {
		return nil, fmt.Errorf("storage tree not found: %s", path)
	}

	var largeFiles []*StorageTree
	m.collectFiles(tree, &largeFiles)

	// 按大小排序
	for i := 0; i < len(largeFiles)-1; i++ {
		for j := i + 1; j < len(largeFiles); j++ {
			if largeFiles[j].Size > largeFiles[i].Size {
				largeFiles[i], largeFiles[j] = largeFiles[j], largeFiles[i]
			}
		}
	}

	if limit > 0 && limit < len(largeFiles) {
		largeFiles = largeFiles[:limit]
	}

	return largeFiles, nil
}

// collectFiles 收集文件
func (m *StorageMapManager) collectFiles(tree *StorageTree, files *[]*StorageTree) {
	if tree.FileType != "directory" {
		*files = append(*files, tree)
	}
	for _, child := range tree.Children {
		m.collectFiles(child, files)
	}
}

// GetTypeDistribution 获取文件类型分布
func (m *StorageMapManager) GetTypeDistribution(path string) (map[string]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tree, exists := m.trees[path]
	if !exists {
		return nil, fmt.Errorf("storage tree not found: %s", path)
	}

	dist := make(map[string]int64)
	m.collectTypeDistribution(tree, dist)

	return dist, nil
}

// collectTypeDistribution 收集类型分布
func (m *StorageMapManager) collectTypeDistribution(tree *StorageTree, dist map[string]int64) {
	if tree.FileType != "" {
		dist[tree.FileType] += tree.Size
	}
	for _, child := range tree.Children {
		m.collectTypeDistribution(child, dist)
	}
}

// GetDuplicateFiles 获取重复文件
func (m *StorageMapManager) GetDuplicateFiles(path string) (map[string][]*StorageTree, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tree, exists := m.trees[path]
	if !exists {
		return nil, fmt.Errorf("storage tree not found: %s", path)
	}

	// 按大小分组
	sizeGroups := make(map[int64][]*StorageTree)
	m.groupBySize(tree, sizeGroups)

	// 找出重复的
	duplicates := make(map[string][]*StorageTree)
	for size, files := range sizeGroups {
		if len(files) > 1 {
			key := fmt.Sprintf("size_%d", size)
			duplicates[key] = files
		}
	}

	return duplicates, nil
}

// groupBySize 按大小分组
func (m *StorageMapManager) groupBySize(tree *StorageTree, groups map[int64][]*StorageTree) {
	if tree.FileType != "directory" {
		groups[tree.Size] = append(groups[tree.Size], tree)
	}
	for _, child := range tree.Children {
		m.groupBySize(child, groups)
	}
}
