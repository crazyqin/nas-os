// Package hybridflash 提供 SSD/HDD 智能混合分层存储管理.
//
// MLTieringEngine: 机器学习驱动的分层引擎，使用线性回归和移动平均预测数据热度.
package hybridflash

import (
	"math"
	"sync"
	"time"
)

// MLTieringEngine 机器学习分层引擎.
//
// 使用时间序列特征（访问频率、访问间隔、IO模式）训练简单的线性回归模型,
// 预测数据块未来热度，实现更智能的分层决策.
type MLTieringEngine struct {
	mu              sync.RWMutex
	config          *MLTieringConfig
	model           *HeatPredictorModel
	featureStore    *FeatureStore
	historyWindow   time.Duration
	trainingData    []*TrainingSample
	modelVersion    int
	lastTrainedAt   time.Time
}

// MLTieringConfig ML 分层配置.
type MLTieringConfig struct {
	// Enabled 启用 ML 分层引擎.
	Enabled bool `json:"enabled"`
	// HistoryWindow 历史数据窗口.
	HistoryWindow time.Duration `json:"historyWindow"`
	// TrainingInterval 模型训练间隔.
	TrainingInterval time.Duration `json:"trainingInterval"`
	// PredictionHorizon 预测时间范围.
	PredictionHorizon time.Duration `json:"predictionHorizon"`
	// MinSamplesForTraining 最小训练样本数.
	MinSamplesForTraining int `json:"minSamplesForTraining"`
	// LearningRate 学习率.
	LearningRate float64 `json:"learningRate"`
	// DecayFactor 访问频率衰减因子.
	DecayFactor float64 `json:"decayFactor"`
	// HotPredictionThreshold 热数据预测阈值.
	HotPredictionThreshold float64 `json:"hotPredictionThreshold"`
	// EnableIOClustering 启用 IO 模式聚类.
	EnableIOClustering bool `json:"enableIOClustering"`
}

// DefaultMLTieringConfig 默认 ML 分层配置.
func DefaultMLTieringConfig() *MLTieringConfig {
	return &MLTieringConfig{
		Enabled:               true,
		HistoryWindow:         24 * time.Hour,
		TrainingInterval:      1 * time.Hour,
		PredictionHorizon:     1 * time.Hour,
		MinSamplesForTraining: 100,
		LearningRate:          0.01,
		DecayFactor:           0.95,
		HotPredictionThreshold: 0.7,
		EnableIOClustering:    true,
	}
}

// HeatPredictorModel 热度预测模型.
//
// 使用加权线性回归，特征包括:
// - 访问频率 (accessFrequency)
// - 最近访问时间 (recency)
// - IO 模式 (ioPattern)
// - 访问间隔 (interArrivalTime)
// - 数据大小 (dataSize)
type HeatPredictorModel struct {
	mu         sync.RWMutex
	Weights    []float64           `json:"weights"`    // 特征权重
	Bias       float64             `json:"bias"`       // 偏置
	FeatureMin []float64           `json:"featureMin"` // 特征最小值（归一化用）
	FeatureMax []float64           `json:"featureMax"` // 特征最大值（归一化用）
	Version    int                 `json:"version"`
	TrainedAt  time.Time           `json:"trainedAt"`
	Accuracy   float64             `json:"accuracy"`   // 模型精度
}

// FeatureStore 特征存储.
type FeatureStore struct {
	mu       sync.RWMutex
	features map[string]*BlockFeatures // blockID -> features
}

// BlockFeatures 块特征向量.
type BlockFeatures struct {
	BlockID          string    `json:"blockId"`
	AccessFrequency  float64   `json:"accessFrequency"`  // 访问频率 (次/小时)
	Recency          float64   `json:"recency"`          // 最近访问时间 (小时前)
	IOEntropy        float64   `json:"ioEntropy"`        // IO 熵 (衡量随机性)
	InterArrivalMean float64   `json:"interArrivalMean"` // 平均到达间隔
	InterArrivalStd  float64   `json:"interArrivalStd"`  // 到达间隔标准差
	DataSize         float64   `json:"dataSize"`         // 数据大小 (log scale)
	ReadWriteRatio   float64   `json:"readWriteRatio"`   // 读写比
	Burstiness       float64   `json:"burstiness"`       // 突发性指标
	LastUpdated      time.Time `json:"lastUpdated"`
	AccessTimestamps  []time.Time `json:"-"` // 原始访问时间戳（不序列化）
}

// TrainingSample 训练样本.
type TrainingSample struct {
	Features    []float64   `json:"features"`
	Label       float64     `json:"label"`  // 1.0=热, 0.0=冷
	Timestamp   time.Time   `json:"timestamp"`
	BlockID     string      `json:"blockId"`
}

// PredictionResult 预测结果.
type PredictionResult struct {
	BlockID          string         `json:"blockId"`
	HotProbability   float64        `json:"hotProbability"`   // 热数据概率 (0-1)
	PredictedTier    FlashType      `json:"predictedTier"`    // 推荐存储层级
	Confidence       float64        `json:"confidence"`       // 预测置信度
	Features         *BlockFeatures `json:"features"`         // 输入特征
	Recommendation   string         `json:"recommendation"`   // 迁移建议
	EstimatedBenefit float64        `json:"estimatedBenefit"` // 预估性能提升 (%)
}

// NewMLTieringEngine 创建 ML 分层引擎.
func NewMLTieringEngine(config *MLTieringConfig) *MLTieringEngine {
	if config == nil {
		config = DefaultMLTieringConfig()
	}

	// 5个特征: 访问频率, 最近访问, IO熵, 到达间隔, 数据大小
	weights := []float64{0.3, 0.25, 0.2, 0.15, 0.1}

	return &MLTieringEngine{
		config:        config,
		historyWindow: config.HistoryWindow,
		model: &HeatPredictorModel{
			Weights:    weights,
			Bias:       0.0,
			FeatureMin: make([]float64, 5),
			FeatureMax: make([]float64, 5),
			Version:    0,
		},
		featureStore: &FeatureStore{
			features: make(map[string]*BlockFeatures),
		},
		trainingData: make([]*TrainingSample, 0),
	}
}

// UpdateFeatures 更新块特征.
func (e *MLTieringEngine) UpdateFeatures(blockID string, accessTime time.Time, size int64, pattern AccessPattern, isRead bool) {
	e.featureStore.mu.Lock()
	defer e.featureStore.mu.Unlock()

	features, exists := e.featureStore.features[blockID]
	if !exists {
		features = &BlockFeatures{
			BlockID:         blockID,
			DataSize:        math.Log1p(float64(size)),
			AccessTimestamps: make([]time.Time, 0, 100),
		}
		e.featureStore.features[blockID] = features
	}

	features.AccessTimestamps = append(features.AccessTimestamps, accessTime)
	features.LastUpdated = accessTime

	// 保留最近 100 个时间戳
	if len(features.AccessTimestamps) > 100 {
		features.AccessTimestamps = features.AccessTimestamps[len(features.AccessTimestamps)-100:]
	}

	// 计算访问频率（指数加权移动平均）
	e.calculateAccessFrequency(features, accessTime)

	// 计算最近访问时间
	features.Recency = time.Since(accessTime).Hours()

	// 计算 IO 熵
	e.calculateIOEntropy(features, pattern)

	// 计算到达间隔
	e.calculateInterArrival(features)

	// 计算突发性
	e.calculateBurstiness(features)

	// 计算读写比
	if isRead {
		features.ReadWriteRatio = 0.9*features.ReadWriteRatio + 0.1
	} else {
		features.ReadWriteRatio = 0.9 * features.ReadWriteRatio
	}
}

// calculateAccessFrequency 计算访问频率.
func (e *MLTieringEngine) calculateAccessFrequency(features *BlockFeatures, now time.Time) {
	timestamps := features.AccessTimestamps
	if len(timestamps) < 2 {
		features.AccessFrequency = 1.0
		return
	}

	// 使用最近 N 个时间戳计算频率
	windowSize := 10
	if len(timestamps) < windowSize {
		windowSize = len(timestamps)
	}

	recent := timestamps[len(timestamps)-windowSize:]
	duration := now.Sub(recent[0]).Hours()
	if duration > 0 {
		features.AccessFrequency = float64(len(recent)) / duration
	}
}

// calculateIOEntropy 计算 IO 模式熵.
func (e *MLTieringEngine) calculateIOEntropy(features *BlockFeatures, pattern AccessPattern) {
	// 熵越高表示 IO 模式越随机（不利于预取）
	var entropy float64
	switch pattern {
	case AccessPatternSequential:
		entropy = 0.1
	case AccessPatternRandom:
		entropy = 0.9
	case AccessPatternMixed:
		entropy = 0.5
	}

	// 指数移动平均
	features.IOEntropy = 0.8*features.IOEntropy + 0.2*entropy
}

// calculateInterArrival 计算到达间隔统计.
func (e *MLTieringEngine) calculateInterArrival(features *BlockFeatures) {
	timestamps := features.AccessTimestamps
	if len(timestamps) < 2 {
		features.InterArrivalMean = 1.0
		features.InterArrivalStd = 0.5
		return
	}

	// 计算间隔
	intervals := make([]float64, 0, len(timestamps)-1)
	for i := 1; i < len(timestamps); i++ {
		interval := timestamps[i].Sub(timestamps[i-1]).Seconds()
		intervals = append(intervals, interval)
	}

	// 计算均值
	sum := 0.0
	for _, v := range intervals {
		sum += v
	}
	mean := sum / float64(len(intervals))

	// 计算标准差
	variance := 0.0
	for _, v := range intervals {
		variance += (v - mean) * (v - mean)
	}
	std := math.Sqrt(variance / float64(len(intervals)))

	features.InterArrivalMean = mean
	features.InterArrivalStd = std
}

// calculateBurstiness 计算突发性指标.
func (e *MLTieringEngine) calculateBurstiness(features *BlockFeatures) {
	// 突发性 = 标准差 / 均值 (变异系数)
	if features.InterArrivalMean > 0 {
		features.Burstiness = features.InterArrivalStd / features.InterArrivalMean
	}
}

// Predict 预测块的热度.
func (e *MLTieringEngine) Predict(blockID string) *PredictionResult {
	e.featureStore.mu.RLock()
	features, exists := e.featureStore.features[blockID]
	e.featureStore.mu.RUnlock()

	if !exists {
		return &PredictionResult{
			BlockID:       blockID,
			HotProbability: 0.0,
			PredictedTier:  FlashTypeHDD,
			Confidence:     0.0,
			Recommendation: "无历史数据，建议保持当前层级",
		}
	}

	// 特征向量: [访问频率, 最近访问, IO熵, 到达间隔, 数据大小]
	featureVector := e.buildFeatureVector(features)

	// 使用模型预测
	e.model.mu.RLock()
	hotProb := e.predictHotness(featureVector)
	e.model.mu.RUnlock()

	// 计算置信度
	confidence := e.calculateConfidence(features)

	// 确定推荐层级
	predictedTier := FlashTypeHDD
	estimatedBenefit := 0.0
	recommendation := ""

	if hotProb >= e.config.HotPredictionThreshold {
		predictedTier = FlashTypeNVMe
		estimatedBenefit = e.estimateBenefit(features, true)
		recommendation = "预测为热数据，建议迁移到 NVMe 层"
	} else if hotProb >= e.config.HotPredictionThreshold*0.5 {
		predictedTier = FlashTypeSSD
		estimatedBenefit = e.estimateBenefit(features, false)
		recommendation = "预测为温数据，建议迁移到 SSD 层"
	} else {
		recommendation = "预测为冷数据，建议保留在 HDD 层"
	}

	return &PredictionResult{
		BlockID:          blockID,
		HotProbability:   hotProb,
		PredictedTier:    predictedTier,
		Confidence:       confidence,
		Features:         features,
		Recommendation:   recommendation,
		EstimatedBenefit: estimatedBenefit,
	}
}

// buildFeatureVector 构建特征向量.
func (e *MLTieringEngine) buildFeatureVector(features *BlockFeatures) []float64 {
	return []float64{
		features.AccessFrequency,
		features.Recency,
		features.IOEntropy,
		features.InterArrivalMean,
		features.DataSize,
	}
}

// predictHotness 使用线性模型预测热度.
func (e *MLTieringEngine) predictHotness(features []float64) float64 {
	if len(features) != len(e.model.Weights) {
		return 0.0
	}

	// 归一化特征
	normalized := make([]float64, len(features))
	for i, f := range features {
		normalized[i] = e.normalizeFeature(i, f)
	}

	// 线性组合
	sum := e.model.Bias
	for i, w := range e.model.Weights {
		sum += w * normalized[i]
	}

	// Sigmoid 激活函数
	return sigmoid(sum)
}

// normalizeFeature 归一化特征.
func (e *MLTieringEngine) normalizeFeature(index int, value float64) float64 {
	if index >= len(e.model.FeatureMin) || index >= len(e.model.FeatureMax) {
		return value
	}

	min := e.model.FeatureMin[index]
	max := e.model.FeatureMax[index]

	if max == min {
		return 0.5
	}

	return (value - min) / (max - min)
}

// calculateConfidence 计算预测置信度.
func (e *MLTieringEngine) calculateConfidence(features *BlockFeatures) float64 {
	// 样本数量越多，置信度越高
	sampleConfidence := math.Min(float64(len(features.AccessTimestamps))/50.0, 1.0)

	// 最近访问越近，置信度越高
	recencyConfidence := math.Exp(-features.Recency / 24.0) // 24小时衰减

	// 模型精度影响置信度
	modelConfidence := e.model.Accuracy

	return (sampleConfidence*0.4 + recencyConfidence*0.3 + modelConfidence*0.3)
}

// estimateBenefit 估算性能提升.
func (e *MLTieringEngine) estimateBenefit(features *BlockFeatures, isHot bool) float64 {
	baseBenefit := 0.0

	if isHot {
		// NVMe vs HDD 延迟比约为 10:1
		baseBenefit = 90.0
	} else {
		// SSD vs HDD 延迟比约为 5:1
		baseBenefit = 50.0
	}

	// 访问频率越高，收益越大
	freqMultiplier := math.Min(features.AccessFrequency/10.0, 2.0)

	return baseBenefit * freqMultiplier
}

// Train 训练模型.
func (e *MLTieringEngine) Train(samples []*TrainingSample) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(samples) < e.config.MinSamplesForTraining {
		return nil // 样本不足
	}

	// 计算特征范围
	e.updateFeatureRanges(samples)

	// 梯度下降训练
	weights := make([]float64, 5)
	copy(weights, e.model.Weights)
	bias := e.model.Bias

	epochs := 100
	lr := e.config.LearningRate

	for epoch := 0; epoch < epochs; epoch++ {
		totalLoss := 0.0

		for _, sample := range samples {
			if len(sample.Features) != 5 {
				continue
			}

			// 归一化特征
			normalized := make([]float64, 5)
			for i, f := range sample.Features {
				normalized[i] = e.normalizeFeature(i, f)
			}

			// 预测
			pred := bias
			for i, w := range weights {
				pred += w * normalized[i]
			}
			pred = sigmoid(pred)

			// 计算梯度
			error := pred - sample.Label
			totalLoss += error * error

			// 更新权重
			for i := range weights {
				weights[i] -= lr * error * normalized[i]
			}
			bias -= lr * error
		}

		// 收敛检查
		avgLoss := totalLoss / float64(len(samples))
		if avgLoss < 0.001 {
			break
		}
	}

	// 更新模型
	e.model.mu.Lock()
	e.model.Weights = weights
	e.model.Bias = bias
	e.model.Version++
	e.model.TrainedAt = time.Now()
	e.model.Accuracy = e.calculateAccuracy(samples)
	e.model.mu.Unlock()

	e.lastTrainedAt = time.Now()
	e.modelVersion = e.model.Version

	return nil
}

// updateFeatureRanges 更新特征范围（用于归一化）.
func (e *MLTieringEngine) updateFeatureRanges(samples []*TrainingSample) {
	minVals := make([]float64, 5)
	maxVals := make([]float64, 5)

	for i := 0; i < 5; i++ {
		minVals[i] = math.MaxFloat64
		maxVals[i] = -math.MaxFloat64
	}

	for _, sample := range samples {
		if len(sample.Features) != 5 {
			continue
		}
		for i, f := range sample.Features {
			if f < minVals[i] {
				minVals[i] = f
			}
			if f > maxVals[i] {
				maxVals[i] = f
			}
		}
	}

	e.model.FeatureMin = minVals
	e.model.FeatureMax = maxVals
}

// calculateAccuracy 计算模型精度.
func (e *MLTieringEngine) calculateAccuracy(samples []*TrainingSample) float64 {
	correct := 0
	total := 0

	for _, sample := range samples {
		if len(sample.Features) != 5 {
			continue
		}

		normalized := make([]float64, 5)
		for i, f := range sample.Features {
			normalized[i] = e.normalizeFeature(i, f)
		}

		pred := e.model.Bias
		for i, w := range e.model.Weights {
			pred += w * normalized[i]
		}
		pred = sigmoid(pred)

		predicted := pred >= 0.5
		actual := sample.Label >= 0.5

		if predicted == actual {
			correct++
		}
		total++
	}

	if total == 0 {
		return 0.0
	}

	return float64(correct) / float64(total)
}

// GetModel 获取模型信息.
func (e *MLTieringEngine) GetModel() *HeatPredictorModel {
	e.model.mu.RLock()
	defer e.model.mu.RUnlock()

	return &HeatPredictorModel{
		Weights:    append([]float64{}, e.model.Weights...),
		Bias:       e.model.Bias,
		Version:    e.model.Version,
		TrainedAt:  e.model.TrainedAt,
		Accuracy:   e.model.Accuracy,
	}
}

// GetFeatures 获取块特征.
func (e *MLTieringEngine) GetFeatures(blockID string) (*BlockFeatures, bool) {
	e.featureStore.mu.RLock()
	defer e.featureStore.mu.RUnlock()

	features, exists := e.featureStore.features[blockID]
	return features, exists
}

// GetTrainingDataSize 获取训练数据大小.
func (e *MLTieringEngine) GetTrainingDataSize() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.trainingData)
}

// CollectTrainingSample 收集训练样本.
func (e *MLTieringEngine) CollectTrainingSample(blockID string, actualHot bool) {
	e.featureStore.mu.RLock()
	features, exists := e.featureStore.features[blockID]
	e.featureStore.mu.RUnlock()

	if !exists {
		return
	}

	label := 0.0
	if actualHot {
		label = 1.0
	}

	sample := &TrainingSample{
		Features:  e.buildFeatureVector(features),
		Label:     label,
		Timestamp: time.Now(),
		BlockID:   blockID,
	}

	e.mu.Lock()
	e.trainingData = append(e.trainingData, sample)

	// 保留最近 10000 个样本
	if len(e.trainingData) > 10000 {
		e.trainingData = e.trainingData[len(e.trainingData)-10000:]
	}
	e.mu.Unlock()
}

// sigmoid Sigmoid 激活函数.
func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// GetStats 获取 ML 引擎统计.
func (e *MLTieringEngine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.featureStore.mu.RLock()
	featureCount := len(e.featureStore.features)
	e.featureStore.mu.RUnlock()

	e.model.mu.RLock()
	modelVersion := e.model.Version
	modelAccuracy := e.model.Accuracy
	e.model.mu.RUnlock()

	return map[string]interface{}{
		"enabled":           e.config.Enabled,
		"modelVersion":      modelVersion,
		"modelAccuracy":     modelAccuracy,
		"trackedBlocks":     featureCount,
		"trainingSamples":   len(e.trainingData),
		"lastTrainedAt":     e.lastTrainedAt,
		"predictionThreshold": e.config.HotPredictionThreshold,
	}
}
