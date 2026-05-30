// Package smartdatatier 提供智能数据分层管理
// 学习 TrueNAS 26 混合闪存池数据分层特性：
// - 基于访问频率的自动分层
// - 热数据自动提升到SSD
// - 冷数据自动降级到HDD
// - I/O模式感知优化
// - 分层迁移调度
package smartdatatier

import (
	"sync"
	"time"
)

// TierLevel 分层级别
type TierLevel int

const (
	TierHot    TierLevel = 3 // 热数据（SSD/NVMe）
	TierWarm   TierLevel = 2 // 温数据（快速HDD）
	TierCold   TierLevel = 1 // 冷数据（大容量HDD）
	TierArchive TierLevel = 0 // 归档数据
)

// IOPattern I/O模式
type IOPattern string

const (
	PatternSequential IOPattern = "sequential"
	PatternRandom     IOPattern = "random"
	PatternBurst      IOPattern = "burst"
	PatternStreaming  IOPattern = "streaming"
)

// DataFile 数据文件信息
type DataFile struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	CurrentTier  TierLevel `json:"currentTier"`
	AccessCount  int64     `json:"accessCount"`
	LastAccess   time.Time `json:"lastAccess"`
	AccessFreq   float64   `json:"accessFreq"` // 次/天
	IOPattern    IOPattern `json:"ioPattern"`
	CreatedAt    time.Time `json:"createdAt"`
}

// TierConfig 分层配置
type TierConfig struct {
	HotThreshold    float64 `json:"hotThreshold"`    // 热数据阈值（访问频率）
	WarmThreshold   float64 `json:"warmThreshold"`   // 温数据阈值
	ColdThreshold   float64 `json:"coldThreshold"`   // 冷数据阈值
	MaxHotSize      int64   `json:"maxHotSize"`      // 热层最大容量
	MaxWarmSize     int64   `json:"maxWarmSize"`     // 温层最大容量
	MigrationWindow int     `json:"migrationWindow"` // 迁移时间窗口（小时）
	BatchSize       int     `json:"batchSize"`       // 批量迁移大小
}

// MigrationTask 迁移任务
type MigrationTask struct {
	ID         string    `json:"id"`
	FileID     string    `json:"fileId"`
	FromTier   TierLevel `json:"fromTier"`
	ToTier     TierLevel `json:"toTier"`
	Status     string    `json:"status"` // pending, running, completed, failed
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Error      string    `json:"error,omitempty"`
}

// TierStats 分层统计
type TierStats struct {
	Tier        TierLevel `json:"tier"`
	FileCount   int       `json:"fileCount"`
	TotalSize   int64     `json:"totalSize"`
	AvgAccess   float64   `json:"avgAccess"`
	Utilization float64   `json:"utilization"` // 使用率
}

// TierManager 分层管理器
type TierManager struct {
	mu         sync.RWMutex
	files      map[string]*DataFile
	config     *TierConfig
	migrations []*MigrationTask
	stats      map[TierLevel]*TierStats
}

// NewTierManager 创建分层管理器
func NewTierManager(config *TierConfig) *TierManager {
	if config == nil {
		config = &TierConfig{
			HotThreshold:    10.0,  // 每天10次以上为热数据
			WarmThreshold:   1.0,   // 每天1-10次为温数据
			ColdThreshold:   0.1,   // 每天0.1-1次为冷数据
			MaxHotSize:      100 * 1024 * 1024 * 1024,  // 100GB
			MaxWarmSize:     500 * 1024 * 1024 * 1024,  // 500GB
			MigrationWindow: 2,    // 凌晨2点
			BatchSize:       100,
		}
	}
	return &TierManager{
		files:      make(map[string]*DataFile),
		config:     config,
		migrations: make([]*MigrationTask, 0),
		stats:      make(map[TierLevel]*TierStats),
	}
}

// RegisterFile 注册文件
func (tm *TierManager) RegisterFile(file *DataFile) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if file.CreatedAt.IsZero() {
		file.CreatedAt = time.Now()
	}
	if file.LastAccess.IsZero() {
		file.LastAccess = time.Now()
	}
	tm.files[file.ID] = file
	tm.recalculateStats()
}

// RecordAccess 记录访问
func (tm *TierManager) RecordAccess(fileID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	file, ok := tm.files[fileID]
	if !ok {
		return
	}

	file.AccessCount++
	file.LastAccess = time.Now()

	// 计算访问频率（次/天）
	days := time.Since(file.CreatedAt).Hours() / 24
	if days > 0 {
		file.AccessFreq = float64(file.AccessCount) / days
	}

	// 自动调整层级
	newTier := tm.calculateTier(file)
	if newTier != file.CurrentTier {
		file.CurrentTier = newTier
	}

	tm.recalculateStats()
}

// RecommendTier 推荐层级
func (tm *TierManager) RecommendTier(fileID string) TierLevel {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	file, ok := tm.files[fileID]
	if !ok {
		return TierCold
	}
	return tm.calculateTier(file)
}

// GetMigrationPlan 获取迁移计划
func (tm *TierManager) GetMigrationPlan() []*MigrationTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var plan []*MigrationTask
	for _, file := range tm.files {
		recommended := tm.calculateTier(file)
		if recommended != file.CurrentTier {
			task := &MigrationTask{
				ID:       file.ID + "-migrate",
				FileID:   file.ID,
				FromTier: file.CurrentTier,
				ToTier:   recommended,
				Status:   "pending",
			}
			plan = append(plan, task)
		}
	}
	return plan
}

// ExecuteMigration 执行迁移
func (tm *TierManager) ExecuteMigration(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 查找任务
	var task *MigrationTask
	for _, t := range tm.migrations {
		if t.ID == taskID {
			task = t
			break
		}
	}

	if task == nil {
		return ErrTaskNotFound
	}

	task.Status = "running"
	task.StartedAt = time.Now()

	// 模拟迁移
	file, ok := tm.files[task.FileID]
	if !ok {
		task.Status = "failed"
		task.Error = "file not found"
		task.FinishedAt = time.Now()
		return ErrFileNotFound
	}

	file.CurrentTier = task.ToTier
	task.Status = "completed"
	task.FinishedAt = time.Now()

	tm.recalculateStats()
	return nil
}

// GetStats 获取统计
func (tm *TierManager) GetStats() map[TierLevel]*TierStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.stats
}

// GetFile 获取文件信息
func (tm *TierManager) GetFile(fileID string) (*DataFile, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	file, ok := tm.files[fileID]
	return file, ok
}

// ListFiles 列出文件
func (tm *TierManager) ListFiles(tier TierLevel) []*DataFile {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []*DataFile
	for _, file := range tm.files {
		if tier >= 0 && file.CurrentTier != tier {
			continue
		}
		result = append(result, file)
	}
	return result
}

// calculateTier 计算推荐层级
func (tm *TierManager) calculateTier(file *DataFile) TierLevel {
	if file.AccessFreq >= tm.config.HotThreshold {
		return TierHot
	} else if file.AccessFreq >= tm.config.WarmThreshold {
		return TierWarm
	} else if file.AccessFreq >= tm.config.ColdThreshold {
		return TierCold
	}
	return TierArchive
}

// recalculateStats 重新计算统计
func (tm *TierManager) recalculateStats() {
	stats := make(map[TierLevel]*TierStats)
	for tier := TierArchive; tier <= TierHot; tier++ {
		stats[tier] = &TierStats{Tier: tier}
	}

	for _, file := range tm.files {
		stat := stats[file.CurrentTier]
		stat.FileCount++
		stat.TotalSize += file.Size
		stat.AvgAccess += file.AccessFreq
	}

	for _, stat := range stats {
		if stat.FileCount > 0 {
			stat.AvgAccess /= float64(stat.FileCount)
		}
	}

	tm.stats = stats
}

// 错误定义
var (
	ErrTaskNotFound = &TierError{"migration task not found"}
	ErrFileNotFound = &TierError{"file not found"}
)

type TierError struct {
	msg string
}

func (e *TierError) Error() string {
	return e.msg
}
