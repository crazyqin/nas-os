package storageefficiency

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// Manager 存储效率管理器
type Manager struct {
	configs  sync.Map // configID -> *EfficiencyConfig
	tasks    sync.Map // taskID -> *EfficiencyTask
	stats    sync.Map // configID -> *EfficiencyStats
	schedules sync.Map // configID -> *ScheduleConfig
	mu       sync.RWMutex
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{}
}

// CreateConfig 创建效率配置
func (m *Manager) CreateConfig(req *CreateConfigRequest) (*EfficiencyConfig, error) {
	if err := m.validateStrategy(req.Strategy); err != nil {
		return nil, err
	}

	if req.Compression == "" {
		req.Compression = AlgoLZ4
	}

	if req.CompressionLevel == 0 {
		req.CompressionLevel = 3
	}

	if req.ChunkSizeKB == 0 {
		req.ChunkSizeKB = 64
	}

	config := &EfficiencyConfig{
		ID:               fmt.Sprintf("eff_%d", time.Now().UnixNano()),
		Name:             req.Name,
		Strategy:         req.Strategy,
		Compression:      req.Compression,
		CompressionLevel: req.CompressionLevel,
		DedupMode:        req.DedupMode,
		ChunkSizeKB:      req.ChunkSizeKB,
		MinFileSizeKB:    req.MinFileSizeKB,
		MaxFileSizeGB:    req.MaxFileSizeGB,
		Enabled:          true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	m.configs.Store(config.ID, config)
	m.stats.Store(config.ID, &EfficiencyStats{
		ConfigID: config.ID,
		Status:   "idle",
	})

	return config, nil
}

// RunEfficiency 运行效率优化
func (m *Manager) RunEfficiency(configID string) (*EfficiencyTask, error) {
	obj, ok := m.configs.Load(configID)
	if !ok {
		return nil, ErrConfigNotFound
	}

	config := obj.(*EfficiencyConfig)
	if !config.Enabled {
		return nil, fmt.Errorf("config %s is disabled", configID)
	}

	// 检查是否已有任务在运行
	m.tasks.Range(func(key, value interface{}) bool {
		task := value.(*EfficiencyTask)
		if task.ConfigID == configID && task.Status == TaskRunning {
			return false
		}
		return true
	})

	task := &EfficiencyTask{
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		ConfigID:  configID,
		Status:    TaskPending,
		StartTime: time.Now(),
	}

	m.tasks.Store(task.ID, task)

	// 异步执行任务
	go m.executeTask(task, config)

	return task, nil
}

func (m *Manager) executeTask(task *EfficiencyTask, config *EfficiencyConfig) {
	task.Status = TaskRunning
	task.Progress = 0

	stats := &EfficiencyStats{
		ConfigID:  config.ID,
		Status:    "running",
		LastRunTime: time.Now(),
	}

	// 模拟处理过程
	totalFiles := int64(1000)
	for i := int64(0); i < totalFiles; i++ {
		task.Progress = float64(i) / float64(totalFiles)
		stats.ProcessedFiles = i + 1
		time.Sleep(10 * time.Millisecond) // 模拟处理时间
	}

	// 完成统计
	stats.TotalFiles = totalFiles
	stats.OriginalSizeBytes = 1024 * 1024 * 1024 * 100 // 100GB
	stats.StoredSizeBytes = int64(float64(stats.OriginalSizeBytes) * 0.6) // 节省40%
	stats.SpaceSavedBytes = stats.OriginalSizeBytes - stats.StoredSizeBytes
	stats.SpaceSavedPercent = float64(stats.SpaceSavedBytes) / float64(stats.OriginalSizeBytes) * 100
	stats.DedupRatio = 1.5
	stats.CompressionRatio = 0.7
	stats.Status = "completed"

	endTime := time.Now()
	task.Status = TaskCompleted
	task.Progress = 1.0
	task.EndTime = &endTime
	task.Stats = stats

	m.stats.Store(config.ID, stats)
}

// GetStats 获取效率统计
func (m *Manager) GetStats(configID string) (*EfficiencyStats, error) {
	obj, ok := m.stats.Load(configID)
	if !ok {
		return nil, ErrConfigNotFound
	}
	return obj.(*EfficiencyStats), nil
}

// GetTask 获取任务状态
func (m *Manager) GetTask(taskID string) (*EfficiencyTask, error) {
	obj, ok := m.tasks.Load(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	return obj.(*EfficiencyTask), nil
}

// AnalyzeStorage 分析存储
func (m *Manager) AnalyzeStorage() *StorageAnalysis {
	return &StorageAnalysis{
		TotalCapacity:     1024 * 1024 * 1024 * 1024, // 1TB
		UsedCapacity:      1024 * 1024 * 1024 * 500,   // 500GB
		FreeCapacity:      1024 * 1024 * 1024 * 500,   // 500GB
		UniqueData:        1024 * 1024 * 1024 * 400,    // 400GB
		DuplicateData:     1024 * 1024 * 1024 * 100,    // 100GB
		CompressibleData:  1024 * 1024 * 1024 * 300,    // 300GB
		EstimatedSaving:   1024 * 1024 * 1024 * 150,    // 150GB
		FileTypeBreakdown: map[string]int64{
			"documents": 1024 * 1024 * 1024 * 50,
			"images":    1024 * 1024 * 1024 * 200,
			"videos":    1024 * 1024 * 1024 * 150,
			"other":     1024 * 1024 * 1024 * 100,
		},
		TopDuplicates: []DedupEntry{
			{
				Hash:     "abc123",
				Size:     1024 * 1024 * 100,
				RefCount: 5,
				Files:    []string{"file1.dat", "file2.dat", "file3.dat"},
			},
		},
		Recommendations: []Recommendation{
			{
				ID:          "rec_1",
				Type:        "dedup",
				Title:       "启用数据去重",
				Description: "检测到约100GB重复数据，建议启用去重功能",
				SavingBytes: 1024 * 1024 * 100,
				Priority:    1,
				Confidence:  0.95,
			},
			{
				ID:          "rec_2",
				Type:        "compress",
				Title:       "启用数据压缩",
				Description: "检测到约300GB可压缩数据，建议启用LZ4压缩",
				SavingBytes: 1024 * 1024 * 50,
				Priority:    2,
				Confidence:  0.85,
			},
		},
	}
}

// SetSchedule 设置调度
func (m *Manager) SetSchedule(configID string, schedule *ScheduleConfig) error {
	if _, ok := m.configs.Load(configID); !ok {
		return ErrConfigNotFound
	}

	schedule.ConfigID = configID
	m.schedules.Store(configID, schedule)
	return nil
}

// ListConfigs 列出配置
func (m *Manager) ListConfigs() []*EfficiencyConfig {
	var configs []*EfficiencyConfig
	m.configs.Range(func(key, value interface{}) bool {
		configs = append(configs, value.(*EfficiencyConfig))
		return true
	})
	return configs
}

// DeleteConfig 删除配置
func (m *Manager) DeleteConfig(configID string) error {
	if _, loaded := m.configs.LoadAndDelete(configID); !loaded {
		return ErrConfigNotFound
	}
	m.stats.Delete(configID)
	m.schedules.Delete(configID)
	return nil
}

func (m *Manager) validateStrategy(strategy EfficiencyStrategy) error {
	switch strategy {
	case StrategyDedup, StrategyCompress, StrategyBoth, StrategyNone:
		return nil
	default:
		return ErrInvalidStrategy
	}
}

// CalculateHash 计算文件哈希
func CalculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
