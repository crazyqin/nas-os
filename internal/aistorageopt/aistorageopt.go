// Package aistorageopt 实现 AI 驱动的智能存储优化引擎
// 学习群晖/TrueNAS 高级存储功能，提供预测性容量规划、智能分层、自动压缩
package aistorageopt

import (
	"fmt"
	"sync"
	"time"
)

// OptimizationLevel 优化级别
type OptimizationLevel string

const (
	// LevelConservative 保守优化
	LevelConservative OptimizationLevel = "conservative"
	// LevelBalanced 平衡优化
	LevelBalanced OptimizationLevel = "balanced"
	// LevelAggressive 激进优化
	LevelAggressive OptimizationLevel = "aggressive"
)

// StorageTier 存储层级
type StorageTier string

const (
	// TierHot 热数据层（NVMe/SSD）
	TierHot StorageTier = "hot"
	// TierWarm 温数据层（SAS/SATA SSD）
	TierWarm StorageTier = "warm"
	// TierCold 冷数据层（HDD）
	TierCold StorageTier = "cold"
	// TierArchive 归档层（磁带/对象存储）
	TierArchive StorageTier = "archive"
)

// CompressionAlgorithm 压缩算法
type CompressionAlgorithm string

const (
	// AlgoLZ4 快速压缩
	AlgoLZ4 CompressionAlgorithm = "lz4"
	// AlgoZSTD 高压缩比
	AlgoZSTD CompressionAlgorithm = "zstd"
	// AlgoSnappy 平衡压缩
	AlgoSnappy CompressionAlgorithm = "snappy"
	// AlgoGZIP 通用压缩
	AlgoGZIP CompressionAlgorithm = "gzip"
)

// AccessPattern 访问模式
type AccessPattern struct {
	FileID       string    `json:"file_id"`
	AccessCount  int64     `json:"access_count"`
	LastAccess   time.Time `json:"last_access"`
	AccessFreq   float64   `json:"access_freq"`   // 每天访问次数
	ReadRatio    float64   `json:"read_ratio"`    // 读写比
	SequentialIO float64   `json:"sequential_io"` // 顺序IO比例
	PeakHours    []int     `json:"peak_hours"`    // 高峰时段
}

// StoragePrediction 存储预测
type StoragePrediction struct {
	Timestamp       time.Time        `json:"timestamp"`
	CurrentUsage    int64            `json:"current_usage"`
	PredictedUsage  int64            `json:"predicted_usage"`
	PredictionDate  time.Time        `json:"prediction_date"`
	Confidence      float64          `json:"confidence"`
	Trend           string           `json:"trend"` // growing, stable, shrinking
	DaysUntilFull   int              `json:"days_until_full"`
	Recommendations []Recommendation `json:"recommendations"`
}

// Recommendation 优化建议
type Recommendation struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Priority    int       `json:"priority"`
	Description string    `json:"description"`
	Impact      string    `json:"impact"`
	Savings     int64     `json:"estimated_savings"`
	CreatedAt   time.Time `json:"created_at"`
}

// TierPolicy 分层策略
type TierPolicy struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Enabled          bool          `json:"enabled"`
	Tier             StorageTier   `json:"tier"`
	Condition        TierCondition `json:"condition"`
	Actions          []TierAction  `json:"actions"`
	Priority         int           `json:"priority"`
	LastEvaluated    time.Time     `json:"last_evaluated"`
	FilesMoved       int64         `json:"files_moved"`
	BytesTransferred int64         `json:"bytes_transferred"`
}

// TierCondition 分层条件
type TierCondition struct {
	MinAge          *time.Duration `json:"min_age,omitempty"`
	MaxAge          *time.Duration `json:"max_age,omitempty"`
	MinSize         *int64         `json:"min_size,omitempty"`
	MaxSize         *int64         `json:"max_size,omitempty"`
	AccessThreshold *float64       `json:"access_threshold,omitempty"`
	FileExtensions  []string       `json:"file_extensions,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
}

// TierAction 分层动作
type TierAction struct {
	Type       string               `json:"type"`
	TargetTier StorageTier          `json:"target_tier"`
	Compress   bool                 `json:"compress"`
	Algorithm  CompressionAlgorithm `json:"algorithm"`
	Encrypt    bool                 `json:"encrypt"`
}

// DedupInfo 去重信息
type DedupInfo struct {
	FileID         string    `json:"file_id"`
	Hash           string    `json:"hash"`
	Size           int64     `json:"size"`
	DuplicateCount int       `json:"duplicate_count"`
	SavedSpace     int64     `json:"saved_space"`
	Locations      []string  `json:"locations"`
	LastChecked    time.Time `json:"last_checked"`
}

// StorageMetrics 存储指标
type StorageMetrics struct {
	Timestamp        time.Time `json:"timestamp"`
	TotalSpace       int64     `json:"total_space"`
	UsedSpace        int64     `json:"used_space"`
	FreeSpace        int64     `json:"free_space"`
	ReadIOPS         float64   `json:"read_iops"`
	WriteIOPS        float64   `json:"write_iops"`
	ReadThroughput   float64   `json:"read_throughput"`
	WriteThroughput  float64   `json:"write_throughput"`
	AvgLatency       float64   `json:"avg_latency"`
	CompressionRatio float64   `json:"compression_ratio"`
	DedupRatio       float64   `json:"dedup_ratio"`
}

// AIStorageOpt AI 存储优化器
type AIStorageOpt struct {
	mu              sync.RWMutex
	predictions     map[string]*StoragePrediction
	policies        map[string]*TierPolicy
	accessPatterns  map[string]*AccessPattern
	dedupIndex      map[string]*DedupInfo
	metrics         []*StorageMetrics
	maxMetrics      int
	level           OptimizationLevel
	enabled         bool
	learningRate    float64
	predictionModel *PredictionModel
}

// PredictionModel 预测模型
type PredictionModel struct {
	Weights     []float64 `json:"weights"`
	Bias        float64   `json:"bias"`
	Features    int       `json:"features"`
	TrainedAt   time.Time `json:"trained_at"`
	SampleCount int64     `json:"sample_count"`
	Accuracy    float64   `json:"accuracy"`
}

// Config 优化器配置
type Config struct {
	Level        OptimizationLevel `json:"level"`
	MaxMetrics   int               `json:"max_metrics"`
	LearningRate float64           `json:"learning_rate"`
	Enabled      bool              `json:"enabled"`
}

// New 创建 AI 存储优化器
func New(cfg Config) *AIStorageOpt {
	if cfg.MaxMetrics <= 0 {
		cfg.MaxMetrics = 10000
	}
	if cfg.LearningRate <= 0 {
		cfg.LearningRate = 0.01
	}
	if cfg.Level == "" {
		cfg.Level = LevelBalanced
	}

	return &AIStorageOpt{
		predictions:    make(map[string]*StoragePrediction),
		policies:       make(map[string]*TierPolicy),
		accessPatterns: make(map[string]*AccessPattern),
		dedupIndex:     make(map[string]*DedupInfo),
		metrics:        make([]*StorageMetrics, 0, cfg.MaxMetrics),
		maxMetrics:     cfg.MaxMetrics,
		level:          cfg.Level,
		enabled:        cfg.Enabled,
		learningRate:   cfg.LearningRate,
		predictionModel: &PredictionModel{
			Features: 7,
			Weights:  make([]float64, 7),
			Bias:     0,
		},
	}
}

// RecordAccess 记录文件访问
func (opt *AIStorageOpt) RecordAccess(fileID string, readBytes, writeBytes int64) {
	opt.mu.Lock()
	defer opt.mu.Unlock()

	pattern, exists := opt.accessPatterns[fileID]
	if !exists {
		pattern = &AccessPattern{
			FileID: fileID,
		}
		opt.accessPatterns[fileID] = pattern
	}

	pattern.AccessCount++
	pattern.LastAccess = time.Now()

	totalBytes := readBytes + writeBytes
	if totalBytes > 0 {
		pattern.ReadRatio = float64(readBytes) / float64(totalBytes)
	}

	// 计算访问频率（指数移动平均）
	now := time.Now()
	if !pattern.LastAccess.IsZero() {
		interval := now.Sub(pattern.LastAccess).Hours()
		if interval > 0 {
			freq := 1.0 / interval
			pattern.AccessFreq = 0.3*freq + 0.7*pattern.AccessFreq
		}
	}
}

// RecordMetrics 记录存储指标
func (opt *AIStorageOpt) RecordMetrics(metrics *StorageMetrics) {
	opt.mu.Lock()
	defer opt.mu.Unlock()

	metrics.Timestamp = time.Now()
	opt.metrics = append(opt.metrics, metrics)

	// 保持指标数量在限制内
	if len(opt.metrics) > opt.maxMetrics {
		opt.metrics = opt.metrics[1:]
	}
}

// PredictStorage 预测存储使用
func (opt *AIStorageOpt) PredictStorage(daysAhead int) *StoragePrediction {
	opt.mu.RLock()
	defer opt.mu.RUnlock()

	if len(opt.metrics) < 2 {
		return &StoragePrediction{
			Timestamp:      time.Now(),
			CurrentUsage:   0,
			PredictedUsage: 0,
			Confidence:     0,
			Trend:          "unknown",
		}
	}

	// 使用线性回归预测
	current := opt.metrics[len(opt.metrics)-1]
	usage := make([]float64, len(opt.metrics))
	for i, m := range opt.metrics {
		usage[i] = float64(m.UsedSpace)
	}

	// 简单线性回归
	slope, intercept := linearRegression(usage)
	predicted := slope*float64(len(opt.metrics)+daysAhead) + intercept

	// 计算置信度
	confidence := calculateConfidence(usage, slope, intercept)

	// 确定趋势
	trend := "stable"
	if slope > 0.01 {
		trend = "growing"
	} else if slope < -0.01 {
		trend = "shrinking"
	}

	// 计算剩余天数
	daysUntilFull := -1
	if slope > 0 && current.FreeSpace > 0 {
		daysUntilFull = int(float64(current.FreeSpace) / (slope * 86400))
	}

	prediction := &StoragePrediction{
		Timestamp:       time.Now(),
		CurrentUsage:    current.UsedSpace,
		PredictedUsage:  int64(predicted),
		PredictionDate:  time.Now().AddDate(0, 0, daysAhead),
		Confidence:      confidence,
		Trend:           trend,
		DaysUntilFull:   daysUntilFull,
		Recommendations: opt.generateRecommendations(current, slope),
	}

	opt.predictions["default"] = prediction
	return prediction
}

// AddTierPolicy 添加分层策略
func (opt *AIStorageOpt) AddTierPolicy(policy *TierPolicy) error {
	opt.mu.Lock()
	defer opt.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("策略ID不能为空")
	}

	opt.policies[policy.ID] = policy
	return nil
}

// EvaluateTierPolicies 评估分层策略
func (opt *AIStorageOpt) EvaluateTierPolicies() []string {
	opt.mu.Lock()
	defer opt.mu.Unlock()

	var movedFiles []string

	for _, policy := range opt.policies {
		if !policy.Enabled {
			continue
		}

		policy.LastEvaluated = time.Now()

		// 评估每个文件的访问模式
		for fileID, pattern := range opt.accessPatterns {
			if opt.matchesCondition(pattern, &policy.Condition) {
				// 执行分层动作
				for _, action := range policy.Actions {
					if action.Compress {
						policy.FilesMoved++
						policy.BytesTransferred += 1024 // 模拟
					}
				}
				movedFiles = append(movedFiles, fileID)
			}
		}
	}

	return movedFiles
}

// DeduplicateFiles 文件去重
func (opt *AIStorageOpt) DeduplicateFiles(files map[string]string) map[string]*DedupInfo {
	opt.mu.Lock()
	defer opt.mu.Unlock()

	hashIndex := make(map[string][]string)
	dedupResults := make(map[string]*DedupInfo)

	// 建立哈希索引
	for fileID, hash := range files {
		hashIndex[hash] = append(hashIndex[hash], fileID)
	}

	// 查找重复文件
	for hash, fileIDs := range hashIndex {
		if len(fileIDs) > 1 {
			info := &DedupInfo{
				Hash:           hash,
				DuplicateCount: len(fileIDs),
				Locations:      fileIDs,
				LastChecked:    time.Now(),
			}
			dedupResults[hash] = info
			opt.dedupIndex[hash] = info
		}
	}

	return dedupResults
}

// GetOptimizationReport 获取优化报告
func (opt *AIStorageOpt) GetOptimizationReport() map[string]interface{} {
	opt.mu.RLock()
	defer opt.mu.RUnlock()

	totalSaved := int64(0)
	for _, dedup := range opt.dedupIndex {
		totalSaved += dedup.SavedSpace
	}

	totalMoved := int64(0)
	for _, policy := range opt.policies {
		totalMoved += policy.BytesTransferred
	}

	return map[string]interface{}{
		"level":               opt.level,
		"enabled":             opt.enabled,
		"total_files_tracked": len(opt.accessPatterns),
		"total_dedup_savings": totalSaved,
		"total_tier_moved":    totalMoved,
		"active_policies":     len(opt.policies),
		"metrics_count":       len(opt.metrics),
	}
}

// 辅助函数
func (opt *AIStorageOpt) matchesCondition(pattern *AccessPattern, condition *TierCondition) bool {
	if condition.AccessThreshold != nil {
		if pattern.AccessFreq > *condition.AccessThreshold {
			return false
		}
	}
	return true
}

func (opt *AIStorageOpt) generateRecommendations(metrics *StorageMetrics, slope float64) []Recommendation {
	var recs []Recommendation

	if metrics.CompressionRatio < 1.5 {
		recs = append(recs, Recommendation{
			ID:          "rec-compression",
			Type:        "compression",
			Priority:    1,
			Description: "启用更高压缩比算法可节省更多空间",
			Impact:      "high",
			Savings:     int64(float64(metrics.UsedSpace) * 0.3),
			CreatedAt:   time.Now(),
		})
	}

	if slope > 0 && metrics.FreeSpace < metrics.TotalSpace/10 {
		recs = append(recs, Recommendation{
			ID:          "rec-expand",
			Type:        "expansion",
			Priority:    2,
			Description: "存储空间即将耗尽，建议扩展存储",
			Impact:      "critical",
			Savings:     0,
			CreatedAt:   time.Now(),
		})
	}

	return recs
}

func linearRegression(data []float64) (slope, intercept float64) {
	n := float64(len(data))
	if n < 2 {
		return 0, data[0]
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, sumY / n
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n
	return
}

func calculateConfidence(data []float64, slope, intercept float64) float64 {
	if len(data) < 3 {
		return 0.5
	}

	// 计算 R²
	var ssRes, ssTot float64
	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))

	for i, v := range data {
		predicted := slope*float64(i) + intercept
		ssRes += (v - predicted) * (v - predicted)
		ssTot += (v - mean) * (v - mean)
	}

	if ssTot == 0 {
		return 0.5
	}

	r2 := 1 - ssRes/ssTot
	if r2 < 0 {
		return 0
	}
	if r2 > 1 {
		return 1
	}
	return r2
}
