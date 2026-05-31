package datafabric

import (
	"sync"
	"time"
)

// DataSource 数据源类型.
type DataSource struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        DataSourceType    `json:"type"`
	Endpoint    string            `json:"endpoint"`
	Credentials map[string]string `json:"credentials,omitempty"`
	Status      DataSourceStatus  `json:"status"`
	Capacity    int64             `json:"capacity"`
	Used        int64             `json:"used"`
	Latency     time.Duration     `json:"latency"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// DataSourceType 数据源类型.
type DataSourceType string

const (
	DataSourceLocal    DataSourceType = "local"
	DataSourceCloud    DataSourceType = "cloud"
	DataSourceEdge     DataSourceType = "edge"
	DataSourceNAS      DataSourceType = "nas"
	DataSourceS3       DataSourceType = "s3"
	DataSourceWebDAV   DataSourceType = "webdav"
)

// DataSourceStatus 数据源状态.
type DataSourceStatus string

const (
	StatusOnline  DataSourceStatus = "online"
	StatusOffline DataSourceStatus = "offline"
	StatusSyncing DataSourceStatus = "syncing"
	StatusError   DataSourceStatus = "error"
)

// DataPlacement 数据放置策略.
type DataPlacement struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Rules         []PlacementRule  `json:"rules"`
	Priority      int              `json:"priority"`
	Enabled       bool             `json:"enabled"`
	CreatedAt     time.Time        `json:"created_at"`
}

// PlacementRule 放置规则.
type PlacementRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Condition   string        `json:"condition"`
	TargetID    string        `json:"target_id"`
	Weight      float64       `json:"weight"`
	Enabled     bool          `json:"enabled"`
}

// FabricTask 数据编织任务.
type FabricTask struct {
	ID          string        `json:"id"`
	Type        TaskType      `json:"type"`
	SourceID    string        `json:"source_id"`
	TargetID    string        `json:"target_id"`
	FilePath    string        `json:"file_path"`
	Status      TaskStatus    `json:"status"`
	Progress    float64       `json:"progress"`
	Error       string        `json:"error,omitempty"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

// TaskType 任务类型.
type TaskType string

const (
	TaskMigrate  TaskType = "migrate"
	TaskReplicate TaskType = "replicate"
	TaskArchive  TaskType = "archive"
	TaskOptimize TaskType = "optimize"
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// DataFabricStats 数据编织统计.
type DataFabricStats struct {
	TotalSources   int     `json:"total_sources"`
	OnlineSources  int     `json:"online_sources"`
	TotalCapacity  int64   `json:"total_capacity"`
	TotalUsed      int64   `json:"total_used"`
	ActiveTasks    int     `json:"active_tasks"`
	DataLocality   float64 `json:"data_locality"`
	AvgLatency     float64 `json:"avg_latency"`
}

// Manager 数据编织管理器.
type Manager struct {
	mu          sync.RWMutex
	sources     map[string]*DataSource
	placements  map[string]*DataPlacement
	tasks       map[string]*FabricTask
	stats       *DataFabricStats
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		sources:    make(map[string]*DataSource),
		placements: make(map[string]*DataPlacement),
		tasks:      make(map[string]*FabricTask),
		stats:      &DataFabricStats{},
	}
}

// AddSource 添加数据源.
func (m *Manager) AddSource(source *DataSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	source.CreatedAt = time.Now()
	source.UpdatedAt = time.Now()
	m.sources[source.ID] = source
	m.updateStats()
	return nil
}

// RemoveSource 移除数据源.
func (m *Manager) RemoveSource(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sources, id)
	m.updateStats()
	return nil
}

// GetSource 获取数据源.
func (m *Manager) GetSource(id string) (*DataSource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sources[id]
	return s, ok
}

// ListSources 列出所有数据源.
func (m *Manager) ListSources() []*DataSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*DataSource, 0, len(m.sources))
	for _, s := range m.sources {
		result = append(result, s)
	}
	return result
}

// AddPlacement 添加放置策略.
func (m *Manager) AddPlacement(placement *DataPlacement) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	placement.CreatedAt = time.Now()
	m.placements[placement.ID] = placement
	return nil
}

// CreateTask 创建任务.
func (m *Manager) CreateTask(taskType TaskType, sourceID, targetID, filePath string) *FabricTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	task := &FabricTask{
		ID:        generateID(),
		Type:      taskType,
		SourceID:  sourceID,
		TargetID:  targetID,
		FilePath:  filePath,
		Status:    TaskPending,
		CreatedAt: time.Now(),
	}
	m.tasks[task.ID] = task
	return task
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() *DataFabricStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func (m *Manager) updateStats() {
	stats := &DataFabricStats{}
	for _, s := range m.sources {
		stats.TotalSources++
		if s.Status == StatusOnline {
			stats.OnlineSources++
		}
		stats.TotalCapacity += s.Capacity
		stats.TotalUsed += s.Used
	}
	if stats.TotalCapacity > 0 {
		stats.DataLocality = float64(stats.TotalUsed) / float64(stats.TotalCapacity) * 100
	}
	m.stats = stats
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
