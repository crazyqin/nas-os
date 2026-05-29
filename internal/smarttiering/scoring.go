package smarttiering

import (
	"context"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Predictor AI热度预测器
// 基于访问频率、最近访问时间、文件大小和访问模式计算热度评分
type Predictor struct {
	mu     sync.RWMutex
	config PredictorConfig
	logger *zap.Logger

	// 访问记录缓存
	accessHistory map[string][]AccessRecord // path -> records
	files         map[string]*FileMetadata  // path -> metadata
}

// NewPredictor 创建热度预测器
func NewPredictor(config PredictorConfig, logger *zap.Logger) *Predictor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Predictor{
		config:        config,
		logger:        logger,
		accessHistory: make(map[string][]AccessRecord),
		files:         make(map[string]*FileMetadata),
	}
}

// RecordAccess 记录文件访问
func (p *Predictor) RecordAccess(record AccessRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.accessHistory[record.Path] = append(p.accessHistory[record.Path], record)

	// 更新文件元数据
	meta, exists := p.files[record.Path]
	if !exists {
		meta = &FileMetadata{
			Path:        record.Path,
			Size:        record.Size,
			ContentType: "application/octet-stream",
		}
		p.files[record.Path] = meta
	}

	meta.AccessedAt = record.Timestamp
	meta.AccessCount++
	switch record.OpType {
	case "read":
		meta.ReadCount++
	case "write":
		meta.WriteCount++
		meta.ModifiedAt = record.Timestamp
	}

	// 裁剪历史记录到窗口期
	cutoff := time.Now().AddDate(0, 0, -p.config.HistoryWindowDays)
	records := p.accessHistory[record.Path]
	start := 0
	for i, r := range records {
		if r.Timestamp.After(cutoff) {
			start = i
			break
		}
		if i == len(records)-1 {
			start = len(records)
		}
	}
	if start > 0 {
		p.accessHistory[record.Path] = records[start:]
	}
}

// RegisterFile 注册文件元数据
func (p *Predictor) RegisterFile(meta FileMetadata) {
	p.mu.Lock()
	defer p.mu.Unlock()
	existing := meta
	p.files[meta.Path] = &existing
}

// UpdateHeatScores 更新所有文件热度评分
func (p *Predictor) UpdateHeatScores(ctx context.Context) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	updated := 0
	for path, meta := range p.files {
		if ctx.Err() != nil {
			return updated, ctx.Err()
		}
		meta.HeatScore = p.calculateHeatScore(path, meta)
		updated++
	}

	p.logger.Info("heat scores updated", zap.Int("files", updated))
	return updated, nil
}

// calculateHeatScore 计算单文件热度评分 (0-100)
func (p *Predictor) calculateHeatScore(path string, meta *FileMetadata) float64 {
	records := p.accessHistory[path]

	recencyScore := p.calcRecencyScore(meta.AccessedAt)
	frequencyScore := p.calcFrequencyScore(records)
	sizeScore := p.calcSizeScore(meta.Size)
	patternScore := p.calcPatternScore(records)

	score := recencyScore*p.config.WeightRecency +
		frequencyScore*p.config.WeightFrequency +
		sizeScore*p.config.WeightSize +
		patternScore*p.config.WeightPattern

	// 归一化到 0-100
	score = math.Min(100, math.Max(0, score*100))
	return math.Round(score*100) / 100
}

// calcRecencyScore 计算最近访问评分 (0-1)
// 越近的访问得分越高，使用指数衰减
func (p *Predictor) calcRecencyScore(lastAccess time.Time) float64 {
	if lastAccess.IsZero() {
		return 0
	}
	hours := time.Since(lastAccess).Hours()
	// 指数衰减: 每天衰减 decay_factor
	days := hours / 24
	return math.Pow(p.config.DecayFactor, days)
}

// calcFrequencyScore 计算访问频率评分 (0-1)
// 使用归一化的访问密度
func (p *Predictor) calcFrequencyScore(records []AccessRecord) float64 {
	if len(records) == 0 {
		return 0
	}
	// 计算日均访问次数
	if len(records) < 2 {
		return 0.1 // 单次访问给基础分
	}
	first := records[0].Timestamp
	last := records[len(records)-1].Timestamp
	days := last.Sub(first).Hours() / 24
	if days < 0.01 {
		days = 0.01
	}
	dailyFreq := float64(len(records)) / days

	// 使用对数归一化，10次/天为满分
	return math.Min(1.0, math.Log1p(dailyFreq)/math.Log1p(10))
}

// calcSizeScore 计算文件大小评分 (0-1)
// 小文件更容易被频繁访问，得分稍高
func (p *Predictor) calcSizeScore(size int64) float64 {
	sizeMB := float64(size) / (1024 * 1024)
	if sizeMB <= 0 {
		return 0.5
	}
	// 100MB以下高分，1GB以上低分
	return math.Max(0.1, 1.0-math.Log1p(sizeMB)/math.Log1p(1024))
}

// calcPatternScore 计算访问模式评分 (0-1)
// 分析访问的时间规律性
func (p *Predictor) calcPatternScore(records []AccessRecord) float64 {
	if len(records) < 3 {
		return 0.5 // 数据不足返回中性分
	}

	// 分析访问时间间隔的标准差
	// 间隔越均匀，模式越规律
	intervals := make([]float64, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		interval := records[i].Timestamp.Sub(records[i-1].Timestamp).Hours()
		intervals = append(intervals, interval)
	}

	if len(intervals) == 0 {
		return 0.5
	}

	// 计算变异系数 (CV)
	var sum, sumSq float64
	for _, v := range intervals {
		sum += v
		sumSq += v * v
	}
	mean := sum / float64(len(intervals))
	if mean <= 0 {
		return 0.5
	}
	variance := sumSq/float64(len(intervals)) - mean*mean
	if variance < 0 {
		variance = 0
	}
	stddev := math.Sqrt(variance)
	cv := stddev / mean

	// CV越小，模式越规律，得分越高
	return math.Max(0.1, 1.0-math.Min(1.0, cv/2.0))
}

// PredictTier 预测文件应该在哪个层级
func (p *Predictor) PredictTier(path string, thresholds MigratorConfig) (StorageTier, float64) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	meta, exists := p.files[path]
	if !exists {
		return TierCold, 0
	}

	score := meta.HeatScore
	switch {
	case score >= thresholds.HotThreshold:
		return TierHot, score
	case score >= thresholds.WarmThreshold:
		return TierWarm, score
	case score >= thresholds.ColdThreshold:
		return TierCold, score
	default:
		return TierArchive, score
	}
}

// GetFileHeat 获取文件热度
func (p *Predictor) GetFileHeat(path string) (float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	meta, exists := p.files[path]
	if !exists {
		return 0, false
	}
	return meta.HeatScore, true
}

// GetAllFiles 获取所有文件元数据
func (p *Predictor) GetAllFiles() map[string]*FileMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[string]*FileMetadata, len(p.files))
	for k, v := range p.files {
		cp := *v
		result[k] = &cp
	}
	return result
}

// GetFilesByTier 获取指定层级的文件
func (p *Predictor) GetFilesByTier(tier StorageTier) []*FileMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []*FileMetadata
	for _, meta := range p.files {
		if meta.CurrentTier == tier {
			cp := *meta
			result = append(result, &cp)
		}
	}
	return result
}

// UpdateConfig 更新配置
func (p *Predictor) UpdateConfig(config PredictorConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
}

// GetConfig 获取配置
func (p *Predictor) GetConfig() PredictorConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}
