// Package smartstoragepredict 智能存储预测引擎
// 对标群晖 DSM 7.3 存储效率提升，使用AI预测存储容量需求
package smartstoragepredict

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// 预测错误定义
var (
	ErrInsufficientData = errors.New("数据点不足，无法预测")
	ErrInvalidModel     = errors.New("无效的预测模型")
	ErrPredictionFailed = errors.New("预测计算失败")
)

// PredictionModel 预测模型类型
type PredictionModel string

const (
	ModelLinear      PredictionModel = "linear"      // 线性回归
	ModelExponential PredictionModel = "exponential"  // 指数增长
	ModelPolynomial  PredictionModel = "polynomial"   // 多项式拟合
	ModelARIMA       PredictionModel = "arima"        // ARIMA时间序列
	ModelEnsemble    PredictionModel = "ensemble"     // 集成模型
)

// DataPoint 数据点
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`     // 存储使用量 (bytes)
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// PredictionResult 预测结果
type PredictionResult struct {
	Model       PredictionModel `json:"model"`
	Timestamp   time.Time       `json:"timestamp"`
	Predicted   float64         `json:"predicted"`    // 预测值 (bytes)
	Confidence  float64         `json:"confidence"`   // 置信度 (0-1)
	LowerBound  float64         `json:"lower_bound"`  // 下界
	UpperBound  float64         `json:"upper_bound"`  // 上界
	GrowthRate  float64         `json:"growth_rate"`  // 增长率 (% per day)
	DaysToFull  int             `json:"days_to_full"` // 预计多少天后满
	AlertLevel  AlertLevel      `json:"alert_level"`  // 告警级别
}

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertNormal  AlertLevel = "normal"
	AlertWarning AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
	AlertFull    AlertLevel = "full"
)

// StoragePool 存储池信息
type StoragePool struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TotalBytes  int64     `json:"total_bytes"`
	UsedBytes   int64     `json:"used_bytes"`
	FreeBytes   int64     `json:"free_bytes"`
	UsageRate   float64   `json:"usage_rate"` // 使用率 0-1
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PredictionConfig 预测配置
type PredictionConfig struct {
	DefaultModel    PredictionModel `json:"default_model"`
	DataRetention   time.Duration   `json:"data_retention"`   // 数据保留时间
	MinDataPoints   int             `json:"min_data_points"`  // 最少数据点
	AlertThresholds AlertThresholds `json:"alert_thresholds"`
	UpdateInterval  time.Duration   `json:"update_interval"`  // 更新间隔
}

// AlertThresholds 告警阈值
type AlertThresholds struct {
	WarningPercent  float64 `json:"warning_percent"`   // 警告阈值 (80%)
	CriticalPercent float64 `json:"critical_percent"`  // 严重阈值 (90%)
	FullPercent     float64 `json:"full_percent"`      // 满阈值 (95%)
}

// DefaultPredictionConfig 默认配置
func DefaultPredictionConfig() *PredictionConfig {
	return &PredictionConfig{
		DefaultModel:  ModelEnsemble,
		DataRetention: 90 * 24 * time.Hour, // 90天
		MinDataPoints: 7,
		AlertThresholds: AlertThresholds{
			WarningPercent:  80,
			CriticalPercent: 90,
			FullPercent:     95,
		},
		UpdateInterval: 24 * time.Hour,
	}
}

// StoragePredictor 存储预测器
type StoragePredictor struct {
	mu           sync.RWMutex
	config       *PredictionConfig
	pools        map[string]*StoragePool
	history      map[string][]DataPoint     // poolID -> 数据点历史
	predictions  map[string]*PredictionResult // poolID -> 最新预测
	models       map[PredictionModel]PredictModel
}

// PredictModel 预测模型接口
type PredictModel interface {
	Name() PredictionModel
	Predict(data []DataPoint, horizon time.Duration) (*PredictionResult, error)
	MinDataPoints() int
}

// NewStoragePredictor 创建存储预测器
func NewStoragePredictor(config *PredictionConfig) *StoragePredictor {
	if config == nil {
		config = DefaultPredictionConfig()
	}

	predictor := &StoragePredictor{
		config:      config,
		pools:       make(map[string]*StoragePool),
		history:     make(map[string][]DataPoint),
		predictions: make(map[string]*PredictionResult),
		models:      make(map[PredictionModel]PredictModel),
	}

	// 注册预测模型
	predictor.registerModels()

	return predictor
}

// registerModels 注册预测模型
func (p *StoragePredictor) registerModels() {
	p.models[ModelLinear] = &LinearRegressionModel{}
	p.models[ModelExponential] = &ExponentialGrowthModel{}
	p.models[ModelPolynomial] = &PolynomialModel{}
	p.models[ModelARIMA] = &ARIMAModel{}
	p.models[ModelEnsemble] = &EnsembleModel{}
}

// RegisterPool 注册存储池
func (p *StoragePredictor) RegisterPool(pool *StoragePool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if pool.ID == "" {
		return fmt.Errorf("存储池ID不能为空")
	}

	pool.UpdatedAt = time.Now()
	p.pools[pool.ID] = pool
	return nil
}

// RecordUsage 记录使用量
func (p *StoragePredictor) RecordUsage(poolID string, usedBytes int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	pool, exists := p.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolID)
	}

	// 更新存储池
	pool.UsedBytes = usedBytes
	pool.FreeBytes = pool.TotalBytes - usedBytes
	pool.UsageRate = float64(usedBytes) / float64(pool.TotalBytes)
	pool.UpdatedAt = time.Now()

	// 添加数据点
	point := DataPoint{
		Timestamp: time.Now(),
		Value:     float64(usedBytes),
	}
	p.history[poolID] = append(p.history[poolID], point)

	// 清理过期数据
	p.cleanupHistory(poolID)

	return nil
}

// cleanupHistory 清理过期历史数据
func (p *StoragePredictor) cleanupHistory(poolID string) {
	history := p.history[poolID]
	if len(history) == 0 {
		return
	}

	cutoff := time.Now().Add(-p.config.DataRetention)
	var cleaned []DataPoint
	for _, point := range history {
		if point.Timestamp.After(cutoff) {
			cleaned = append(cleaned, point)
		}
	}
	p.history[poolID] = cleaned
}

// Predict 预测存储容量
func (p *StoragePredictor) Predict(poolID string, horizon time.Duration) (*PredictionResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pool, exists := p.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	history, exists := p.history[poolID]
	if !exists || len(history) < p.config.MinDataPoints {
		return nil, ErrInsufficientData
	}

	// 使用默认模型预测
	model, exists := p.models[p.config.DefaultModel]
	if !exists {
		return nil, ErrInvalidModel
	}

	result, err := model.Predict(history, horizon)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPredictionFailed, err)
	}

	// 设置告警级别
	result.AlertLevel = p.calculateAlertLevel(pool, result)

	// 缓存预测结果
	p.predictions[poolID] = result

	return result, nil
}

// calculateAlertLevel 计算告警级别
func (p *StoragePredictor) calculateAlertLevel(pool *StoragePool, result *PredictionResult) AlertLevel {
	usagePercent := pool.UsageRate * 100

	if usagePercent >= p.config.AlertThresholds.FullPercent {
		return AlertFull
	}
	if usagePercent >= p.config.AlertThresholds.CriticalPercent {
		return AlertCritical
	}
	if usagePercent >= p.config.AlertThresholds.WarningPercent {
		return AlertWarning
	}

	// 检查预测的满载时间
	if result.DaysToFull > 0 && result.DaysToFull <= 30 {
		return AlertWarning
	}
	if result.DaysToFull > 0 && result.DaysToFull <= 7 {
		return AlertCritical
	}

	return AlertNormal
}

// GetPool 获取存储池
func (p *StoragePredictor) GetPool(poolID string) (*StoragePool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pool, exists := p.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}
	return pool, nil
}

// ListPools 列出所有存储池
func (p *StoragePredictor) ListPools() []*StoragePool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pools := make([]*StoragePool, 0, len(p.pools))
	for _, pool := range p.pools {
		pools = append(pools, pool)
	}
	return pools
}

// GetPrediction 获取最新预测
func (p *StoragePredictor) GetPrediction(poolID string) (*PredictionResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	prediction, exists := p.predictions[poolID]
	if !exists {
		return nil, fmt.Errorf("无预测数据: %s", poolID)
	}
	return prediction, nil
}

// GetHistory 获取历史数据
func (p *StoragePredictor) GetHistory(poolID string, duration time.Duration) []DataPoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	history, exists := p.history[poolID]
	if !exists {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	var result []DataPoint
	for _, point := range history {
		if point.Timestamp.After(cutoff) {
			result = append(result, point)
		}
	}
	return result
}

// GetStats 获取统计信息
func (p *StoragePredictor) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	totalPools := len(p.pools)
	totalCapacity := int64(0)
	totalUsed := int64(0)
	alerts := map[string]int{
		"normal": 0, "warning": 0, "critical": 0, "full": 0,
	}

	for _, pool := range p.pools {
		totalCapacity += pool.TotalBytes
		totalUsed += pool.UsedBytes
	}

	for _, pred := range p.predictions {
		alerts[string(pred.AlertLevel)]++
	}

	return map[string]interface{}{
		"total_pools":     totalPools,
		"total_capacity":  totalCapacity,
		"total_used":      totalUsed,
		"total_free":      totalCapacity - totalUsed,
		"alerts":          alerts,
		"prediction_model": string(p.config.DefaultModel),
	}
}

// LinearRegressionModel 线性回归模型
type LinearRegressionModel struct{}

func (m *LinearRegressionModel) Name() PredictionModel {
	return ModelLinear
}

func (m *LinearRegressionModel) MinDataPoints() int {
	return 2
}

func (m *LinearRegressionModel) Predict(data []DataPoint, horizon time.Duration) (*PredictionResult, error) {
	if len(data) < 2 {
		return nil, ErrInsufficientData
	}

	// 简单线性回归
	n := float64(len(data))
	var sumX, sumY, sumXY, sumX2 float64

	baseTime := data[0].Timestamp
	for _, point := range data {
		x := point.Timestamp.Sub(baseTime).Hours() / 24 // 天数
		y := point.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	// 预测
	horizonDays := horizon.Hours() / 24
	predicted := intercept + slope*horizonDays

	// 计算R²
	meanY := sumY / n
	var ssRes, ssTot float64
	for _, point := range data {
		x := point.Timestamp.Sub(baseTime).Hours() / 24
		predictedY := intercept + slope*x
		ssRes += (point.Value - predictedY) * (point.Value - predictedY)
		ssTot += (point.Value - meanY) * (point.Value - meanY)
	}

	r2 := 1 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0
	}

	// 计算增长率（每天）
	growthRate := slope / data[len(data)-1].Value * 100

	// 计算满载天数
	daysToFull := 0
	if slope > 0 && data[len(data)-1].Value > 0 {
		remaining := data[len(data)-1].Value // 简化计算
		daysToFull = int(remaining / slope)
	}

	return &PredictionResult{
		Model:       ModelLinear,
		Timestamp:   time.Now().Add(horizon),
		Predicted:   predicted,
		Confidence:  r2,
		LowerBound:  predicted * 0.9,
		UpperBound:  predicted * 1.1,
		GrowthRate:  growthRate,
		DaysToFull:  daysToFull,
	}, nil
}

// ExponentialGrowthModel 指数增长模型
type ExponentialGrowthModel struct{}

func (m *ExponentialGrowthModel) Name() PredictionModel {
	return ModelExponential
}

func (m *ExponentialGrowthModel) MinDataPoints() int {
	return 3
}

func (m *ExponentialGrowthModel) Predict(data []DataPoint, horizon time.Duration) (*PredictionResult, error) {
	if len(data) < 3 {
		return nil, ErrInsufficientData
	}

	// 使用对数变换拟合指数增长
	baseTime := data[0].Timestamp
	var sumX, sumLogY, sumXLogY, sumX2 float64
	n := float64(len(data))

	for _, point := range data {
		x := point.Timestamp.Sub(baseTime).Hours() / 24
		logY := math.Log(point.Value)
		sumX += x
		sumLogY += logY
		sumXLogY += x * logY
		sumX2 += x * x
	}

	// log(y) = log(a) + b*x
	b := (n*sumXLogY - sumX*sumLogY) / (n*sumX2 - sumX*sumX)
	logA := (sumLogY - b*sumX) / n
	a := math.Exp(logA)

	// 预测
	horizonDays := horizon.Hours() / 24
	predicted := a * math.Exp(b*horizonDays)

	// 增长率
	growthRate := (math.Exp(b) - 1) * 100

	// 满载天数
	daysToFull := 0
	if b > 0 {
		// 假设总容量为当前值的2倍（简化）
		targetCapacity := data[len(data)-1].Value * 2
		daysToFull = int(math.Log(targetCapacity/a) / b)
	}

	return &PredictionResult{
		Model:       ModelExponential,
		Timestamp:   time.Now().Add(horizon),
		Predicted:   predicted,
		Confidence:  0.7,
		LowerBound:  predicted * 0.85,
		UpperBound:  predicted * 1.15,
		GrowthRate:  growthRate,
		DaysToFull:  daysToFull,
	}, nil
}

// PolynomialModel 多项式模型
type PolynomialModel struct{}

func (m *PolynomialModel) Name() PredictionModel {
	return ModelPolynomial
}

func (m *PolynomialModel) MinDataPoints() int {
	return 4
}

func (m *PolynomialModel) Predict(data []DataPoint, horizon time.Duration) (*PredictionResult, error) {
	if len(data) < 4 {
		return nil, ErrInsufficientData
	}

	// 简化：使用二次多项式
	baseTime := data[0].Timestamp
	horizonDays := horizon.Hours() / 24

	// 使用最后几个点拟合
	n := len(data)
	lastPoints := data
	if n > 10 {
		lastPoints = data[n-10:]
	}

	// 简单移动平均预测
	sum := 0.0
	for _, point := range lastPoints {
		sum += point.Value
	}
	avg := sum / float64(len(lastPoints))

	// 计算趋势
	firstHalf := lastPoints[:len(lastPoints)/2]
	secondHalf := lastPoints[len(lastPoints)/2:]

	sumFirst := 0.0
	for _, p := range firstHalf {
		sumFirst += p.Value
	}
	sumSecond := 0.0
	for _, p := range secondHalf {
		sumSecond += p.Value
	}

	trend := (sumSecond/float64(len(secondHalf)) - sumFirst/float64(len(firstHalf))) / 
		(secondHalf[0].Timestamp.Sub(firstHalf[0].Timestamp).Hours()/24)

	predicted := avg + trend*horizonDays

	_ = baseTime // 避免未使用警告

	return &PredictionResult{
		Model:       ModelPolynomial,
		Timestamp:   time.Now().Add(horizon),
		Predicted:   predicted,
		Confidence:  0.6,
		LowerBound:  predicted * 0.8,
		UpperBound:  predicted * 1.2,
		GrowthRate:  trend / avg * 100,
		DaysToFull:  0,
	}, nil
}

// ARIMAModel ARIMA模型（简化实现）
type ARIMAModel struct{}

func (m *ARIMAModel) Name() PredictionModel {
	return ModelARIMA
}

func (m *ARIMAModel) MinDataPoints() int {
	return 10
}

func (m *ARIMAModel) Predict(data []DataPoint, horizon time.Duration) (*PredictionResult, error) {
	if len(data) < 10 {
		return nil, ErrInsufficientData
	}

	// 简化ARIMA：使用指数平滑
	alpha := 0.3 // 平滑因子
	smoothed := data[0].Value

	for i := 1; i < len(data); i++ {
		smoothed = alpha*data[i].Value + (1-alpha)*smoothed
	}

	// 计算趋势
	n := len(data)
	trend := (data[n-1].Value - data[n-5].Value) / 4

	horizonDays := horizon.Hours() / 24
	predicted := smoothed + trend*horizonDays

	return &PredictionResult{
		Model:       ModelARIMA,
		Timestamp:   time.Now().Add(horizon),
		Predicted:   predicted,
		Confidence:  0.75,
		LowerBound:  predicted * 0.85,
		UpperBound:  predicted * 1.15,
		GrowthRate:  trend / smoothed * 100,
		DaysToFull:  0,
	}, nil
}

// EnsembleModel 集成模型
type EnsembleModel struct{}

func (m *EnsembleModel) Name() PredictionModel {
	return ModelEnsemble
}

func (m *EnsembleModel) MinDataPoints() int {
	return 10
}

func (m *EnsembleModel) Predict(data []DataPoint, horizon time.Duration) (*PredictionResult, error) {
	if len(data) < 10 {
		return nil, ErrInsufficientData
	}

	// 使用多个模型并加权平均
	models := []PredictModel{
		&LinearRegressionModel{},
		&ExponentialGrowthModel{},
		&ARIMAModel{},
	}

	weights := []float64{0.3, 0.3, 0.4}
	var totalPredicted, totalConfidence float64
	var totalGrowthRate float64

	for i, model := range models {
		result, err := model.Predict(data, horizon)
		if err != nil {
			continue
		}
		totalPredicted += result.Predicted * weights[i]
		totalConfidence += result.Confidence * weights[i]
		totalGrowthRate += result.GrowthRate * weights[i]
	}

	return &PredictionResult{
		Model:       ModelEnsemble,
		Timestamp:   time.Now().Add(horizon),
		Predicted:   totalPredicted,
		Confidence:  totalConfidence,
		LowerBound:  totalPredicted * 0.9,
		UpperBound:  totalPredicted * 1.1,
		GrowthRate:  totalGrowthRate,
		DaysToFull:  0,
	}, nil
}
