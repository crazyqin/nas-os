// Package storagetiering 数据热度分析器
package storagetiering

import (
	"context"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Analyzer 数据热度分析器
type Analyzer struct {
	mu          sync.RWMutex
	config      *StorageTieringConfig
	analyzerCfg AnalyzerConfig
	policyCfg   PolicyConfig
	logger      *zap.Logger

	// 内部状态
	files        map[string]*FileEntry
	hitCount     int64
	missCount    int64
	lastAnalysis time.Time
}

// NewAnalyzer 创建热度分析器
func NewAnalyzer(config *StorageTieringConfig) *Analyzer {
	if config == nil {
		config = DefaultStorageTieringConfig()
	}
	return &Analyzer{
		config: config,
		logger: zap.NewNop(),
		files:  make(map[string]*FileEntry),
	}
}

// NewAnalyzerWithConfig 创建带完整配置的分析器
func NewAnalyzerWithConfig(analyzerCfg AnalyzerConfig, policyCfg PolicyConfig, logger *zap.Logger) *Analyzer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Analyzer{
		config:      DefaultStorageTieringConfig(),
		analyzerCfg: analyzerCfg,
		policyCfg:   policyCfg,
		logger:      logger,
		files:       make(map[string]*FileEntry),
	}
}

// RegisterFile 注册文件到分析器
func (a *Analyzer) RegisterFile(entry FileEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.files[entry.Path] = &entry
	a.logger.Debug("file registered in analyzer",
		zap.String("path", entry.Path),
		zap.Int64("size", entry.Size))
}

// RecordAccess 记录文件访问
func (a *Analyzer) RecordAccess(record AccessRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if entry, ok := a.files[record.FilePath]; ok {
		entry.AccessCount++
		entry.LastAccess = record.Timestamp
		a.hitCount++
	} else {
		a.missCount++
	}
}

// Analyze 分析所有文件，返回需要迁移的任务
func (a *Analyzer) Analyze(ctx context.Context) ([]*MigrationTask, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	var tasks []*MigrationTask

	for _, entry := range a.files {
		// 计算热度分数
		entry.HeatScore = a.CalculateHeatScoreForEntry(entry, now)

		// 检查是否需要迁移
		if task := a.checkMigration(entry, now); task != nil {
			tasks = append(tasks, task)
		}
	}

	a.lastAnalysis = now
	return tasks, nil
}

// CalculateHeatScoreForEntry 计算文件条目的热度分数
func (a *Analyzer) CalculateHeatScoreForEntry(entry *FileEntry, now time.Time) float64 {
	if entry == nil {
		return 0
	}

	// 访问频率分数 (0-40分)
	freqScore := a.calculateFrequencyScore(entry.AccessCount)

	// 最近访问时间分数 (0-40分)
	recencyScore := a.calculateRecencyScore(entry.LastAccess, now)

	// 基础分数 (0-20分)
	baseScore := 20.0

	totalScore := freqScore + recencyScore + baseScore

	// 归一化到 0-100
	return math.Min(100, math.Max(0, totalScore))
}

// checkMigration 检查文件是否需要迁移
func (a *Analyzer) checkMigration(entry *FileEntry, now time.Time) *MigrationTask {
	// 根据热度分数推荐目标层级
	var targetTier Tier
	if entry.HeatScore >= a.config.HeatThresholdHot {
		targetTier = TierSSD
	} else if entry.HeatScore >= a.config.HeatThresholdWarm {
		targetTier = TierHDD
	} else {
		targetTier = TierCold
	}

	// 如果目标层级与当前层级相同，不需要迁移
	if targetTier == entry.CurrentTier {
		return nil
	}

	// 如果文件被固定，不迁移
	if entry.IsPinned {
		return nil
	}

	return &MigrationTask{
		ID:        generateID(),
		FilePath:  entry.Path,
		FileSize:  entry.Size,
		FromTier:  entry.CurrentTier,
		ToTier:    targetTier,
		State:     StatePending,
		Reason:    "auto-tiering",
		CreatedAt: now,
	}
}

// HitRate 返回缓存命中率
func (a *Analyzer) HitRate() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	total := a.hitCount + a.missCount
	if total == 0 {
		return 0
	}
	return float64(a.hitCount) / float64(total)
}

// LastAnalysis 返回最后分析时间
func (a *Analyzer) LastAnalysis() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastAnalysis
}

// CalculateHeatScore 计算文件热度分数（兼容旧接口）
func (a *Analyzer) CalculateHeatScore(file *FileMetadata, now time.Time) float64 {
	if file == nil {
		return 0
	}

	// 访问频率分数 (0-40分)
	freqScore := a.calculateFrequencyScore(file.AccessCount)

	// 最近访问时间分数 (0-40分)
	recencyScore := a.calculateRecencyScore(file.LastAccessAt, now)

	// 修改时间分数 (0-20分)
	modificationScore := a.calculateModificationScore(file.LastModifiedAt, now)

	totalScore := freqScore + recencyScore + modificationScore

	// 归一化到 0-100
	return math.Min(100, math.Max(0, totalScore))
}

// calculateFrequencyScore 计算访问频率分数
func (a *Analyzer) calculateFrequencyScore(accessCount int64) float64 {
	if accessCount <= 0 {
		return 0
	}
	logCount := math.Log10(float64(accessCount))
	return math.Min(40, logCount*10)
}

// calculateRecencyScore 计算最近访问时间分数
func (a *Analyzer) calculateRecencyScore(lastAccess time.Time, now time.Time) float64 {
	if lastAccess.IsZero() {
		return 0
	}

	daysSinceAccess := now.Sub(lastAccess).Hours() / 24

	decayDays := float64(a.config.HeatDecayDays)
	if decayDays <= 0 {
		decayDays = 30
	}

	score := 40 * math.Exp(-daysSinceAccess/decayDays)
	return math.Max(0, score)
}

// calculateModificationScore 计算修改时间分数
func (a *Analyzer) calculateModificationScore(lastModified time.Time, now time.Time) float64 {
	if lastModified.IsZero() {
		return 0
	}

	daysSinceModified := now.Sub(lastModified).Hours() / 24

	score := 20 * math.Exp(-daysSinceModified/60)
	return math.Max(0, score)
}

// ClassifyHeatLevel 根据热度分数分类热度等级
func (a *Analyzer) ClassifyHeatLevel(score float64) HeatLevel {
	if score >= a.config.HeatThresholdHot {
		return HeatLevelHot
	} else if score >= a.config.HeatThresholdWarm {
		return HeatLevelWarm
	} else if score >= a.config.HeatThresholdCold {
		return HeatLevelCold
	}
	return HeatLevelFrozen
}

// AnalyzeFile 分析单个文件，更新热度信息
func (a *Analyzer) AnalyzeFile(file *FileMetadata, now time.Time) *FileMetadata {
	if file == nil {
		return nil
	}

	file.HeatScore = a.CalculateHeatScore(file, now)
	file.HeatLevel = a.ClassifyHeatLevel(file.HeatScore)
	return file
}

// AnalyzeFiles 批量分析文件
func (a *Analyzer) AnalyzeFiles(files []*FileMetadata, now time.Time) []*FileMetadata {
	for _, file := range files {
		a.AnalyzeFile(file, now)
	}
	return files
}

// CalculateTierFromHeat 根据热度等级推荐存储层级
func (a *Analyzer) CalculateTierFromHeat(heatLevel HeatLevel) TierLevel {
	switch heatLevel {
	case HeatLevelHot:
		return TierLevelHot
	case HeatLevelWarm:
		return TierLevelWarm
	case HeatLevelCold:
		return TierLevelCold
	case HeatLevelFrozen:
		return TierLevelCold
	default:
		return TierLevelWarm
	}
}

// GetHeatDistribution 获取热度分布统计
func (a *Analyzer) GetHeatDistribution(files []*FileMetadata) map[HeatLevel]int64 {
	dist := map[HeatLevel]int64{
		HeatLevelHot:    0,
		HeatLevelWarm:   0,
		HeatLevelCold:   0,
		HeatLevelFrozen: 0,
	}

	for _, file := range files {
		dist[file.HeatLevel]++
	}

	return dist
}

// CalculateAverageHeatScore 计算平均热度分数
func (a *Analyzer) CalculateAverageHeatScore(files []*FileMetadata) float64 {
	if len(files) == 0 {
		return 0
	}

	total := 0.0
	for _, file := range files {
		total += file.HeatScore
	}
	return total / float64(len(files))
}
