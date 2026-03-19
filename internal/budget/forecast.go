// Package budget 提供预算预测功能
package budget

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrInsufficientHistory 历史数据不足无法进行预测错误
	ErrInsufficientHistory = errors.New("历史数据不足，无法进行预测")
	// ErrInvalidForecastParams 无效的预测参数错误
	ErrInvalidForecastParams = errors.New("无效的预测参数")
	// ErrForecastNotFound 预测不存在错误
	ErrForecastNotFound = errors.New("预测不存在")
)

// ========== 预测类型定义 ==========

// ForecastMethod 预测方法
type ForecastMethod string

const (
	// ForecastMethodMovingAverage 移动平均预测方法
	ForecastMethodMovingAverage ForecastMethod = "moving_average"
	// ForecastMethodExponential 指数平滑预测方法
	ForecastMethodExponential ForecastMethod = "exponential"
	// ForecastMethodLinear 线性回归预测方法
	ForecastMethodLinear ForecastMethod = "linear"
	// ForecastMethodSeasonal 季节性预测方法
	ForecastMethodSeasonal ForecastMethod = "seasonal"
	// ForecastMethodARIMA ARIMA模型预测方法
	ForecastMethodARIMA ForecastMethod = "arima"
	// ForecastMethodProphet Prophet模型预测方法
	ForecastMethodProphet ForecastMethod = "prophet"
)

// ForecastPeriod 预测周期
type ForecastPeriod string

const (
	// ForecastPeriodDaily 日预测周期
	ForecastPeriodDaily ForecastPeriod = "daily"
	// ForecastPeriodWeekly 周预测周期
	ForecastPeriodWeekly ForecastPeriod = "weekly"
	// ForecastPeriodMonthly 月预测周期
	ForecastPeriodMonthly ForecastPeriod = "monthly"
	// ForecastPeriodQuarter 季度预测周期
	ForecastPeriodQuarter ForecastPeriod = "quarter"
	// ForecastPeriodYearly 年预测周期
	ForecastPeriodYearly ForecastPeriod = "yearly"
)

// ForecastConfidence 预测置信度
type ForecastConfidence string

const (
	// ConfidenceLow 低置信度（80%置信区间）
	ConfidenceLow ForecastConfidence = "low"
	// ConfidenceMedium 中等置信度（90%置信区间）
	ConfidenceMedium ForecastConfidence = "medium"
	// ConfidenceHigh 高置信度（95%置信区间）
	ConfidenceHigh ForecastConfidence = "high"
	// ConfidenceVeryHigh 极高置信度（99%置信区间）
	ConfidenceVeryHigh ForecastConfidence = "very_high"
)

// ========== 预测数据结构 ==========

// ForecastRequest 预测请求
type ForecastRequest struct {
	BudgetID      string             `json:"budget_id" binding:"required"`
	Method        ForecastMethod     `json:"method"`
	Period        ForecastPeriod     `json:"period"`
	Horizon       int                `json:"horizon"` // 预测时长（周期数）
	Confidence    ForecastConfidence `json:"confidence"`
	Seasonality   bool               `json:"seasonality"`    // 是否考虑季节性
	IncludeBounds bool               `json:"include_bounds"` // 是否包含置信区间
	HistoryDays   int                `json:"history_days"`   // 历史数据天数
}

// ForecastResult 预测结果
type ForecastResult struct {
	ID          string             `json:"id"`
	BudgetID    string             `json:"budget_id"`
	GeneratedAt time.Time          `json:"generated_at"`
	Method      ForecastMethod     `json:"method"`
	Period      ForecastPeriod     `json:"period"`
	Confidence  ForecastConfidence `json:"confidence"`

	// 预测数据点
	DataPoints []ForecastPoint `json:"data_points"`

	// 汇总统计
	Summary ForecastSummary `json:"summary"`

	// 模型信息
	ModelInfo ForecastModelInfo `json:"model_info"`

	// 建议
	Recommendations []ForecastRecommendation `json:"recommendations"`

	// 准确度评估
	Accuracy ForecastAccuracy `json:"accuracy"`
}

// ForecastPoint 预测数据点
type ForecastPoint struct {
	Date         time.Time `json:"date"`
	PeriodLabel  string    `json:"period_label"` // 周期标签（如 "2026-03"）
	Predicted    float64   `json:"predicted"`    // 预测值
	LowerBound   float64   `json:"lower_bound"`  // 下限
	UpperBound   float64   `json:"upper_bound"`  // 上限
	Trend        string    `json:"trend"`        // up, down, stable
	Confidence   float64   `json:"confidence"`   // 置信度百分比
	Contributors []string  `json:"contributors"` // 影响因素
}

// ForecastSummary 预测汇总
type ForecastSummary struct {
	TotalPredicted    float64 `json:"total_predicted"`    // 总预测值
	AveragePredicted  float64 `json:"average_predicted"`  // 平均预测值
	MaxPredicted      float64 `json:"max_predicted"`      // 最大预测值
	MinPredicted      float64 `json:"min_predicted"`      // 最小预测值
	GrowthRate        float64 `json:"growth_rate"`        // 增长率
	Trend             string  `json:"trend"`              // 整体趋势
	SeasonalPattern   string  `json:"seasonal_pattern"`   // 季节性模式
	ConfidenceScore   float64 `json:"confidence_score"`   // 置信度评分
	RiskLevel         string  `json:"risk_level"`         // 风险等级
	RecommendedBudget float64 `json:"recommended_budget"` // 建议预算
	BudgetVariance    float64 `json:"budget_variance"`    // 预算偏差
}

// ForecastModelInfo 预测模型信息
type ForecastModelInfo struct {
	Method          string                 `json:"method"`
	Parameters      map[string]interface{} `json:"parameters"`
	TrainingSize    int                    `json:"training_size"`
	ValidationScore float64                `json:"validation_score"`
	LastUpdated     time.Time              `json:"last_updated"`
	Features        []string               `json:"features"` // 使用的特征
	Weights         map[string]float64     `json:"weights"`  // 特征权重
}

// ForecastRecommendation 预测建议
type ForecastRecommendation struct {
	Type        string  `json:"type"`     // budget_adjust, alert, optimize
	Priority    string  `json:"priority"` // high, medium, low
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Impact      float64 `json:"impact"` // 预计影响金额
	Action      string  `json:"action"` // 建议操作
	Reason      string  `json:"reason"` // 原因说明
}

// ForecastAccuracy 预测准确度
type ForecastAccuracy struct {
	MAPE            float64   `json:"mape"`             // 平均绝对百分比误差
	MAE             float64   `json:"mae"`              // 平均绝对误差
	RMSE            float64   `json:"rmse"`             // 均方根误差
	WithinBounds    float64   `json:"within_bounds"`    // 落在置信区间内的比例
	HistoricalScore float64   `json:"historical_score"` // 历史准确度评分
	LastVerified    time.Time `json:"last_verified"`
}

// ========== 历史数据结构 ==========

// HistoricalDataPoint 历史数据点
type HistoricalDataPoint struct {
	Date     time.Time              `json:"date"`
	Amount   float64                `json:"amount"`
	BudgetID string                 `json:"budget_id"`
	Source   string                 `json:"source"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HistoricalStats 历史统计
type HistoricalStats struct {
	Count         int       `json:"count"`
	Mean          float64   `json:"mean"`
	StdDev        float64   `json:"std_dev"`
	Min           float64   `json:"min"`
	Max           float64   `json:"max"`
	Median        float64   `json:"median"`
	FirstQuartile float64   `json:"first_quartile"`
	ThirdQuartile float64   `json:"third_quartile"`
	Trend         float64   `json:"trend"`       // 趋势斜率
	Seasonality   float64   `json:"seasonality"` // 季节性强度
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
}

// ========== 预测引擎 ==========

// ForecastEngine 预测引擎
type ForecastEngine struct {
	mu            sync.RWMutex
	historyStore  HistoryStore
	config        ForecastConfig
	forecastCache map[string]*ForecastResult
}

// ForecastConfig 预测配置
type ForecastConfig struct {
	DefaultMethod      ForecastMethod     `json:"default_method"`
	DefaultPeriod      ForecastPeriod     `json:"default_period"`
	DefaultHorizon     int                `json:"default_horizon"`
	DefaultConfidence  ForecastConfidence `json:"default_confidence"`
	MinHistoryDays     int                `json:"min_history_days"`
	MaxHistoryDays     int                `json:"max_history_days"`
	CacheEnabled       bool               `json:"cache_enabled"`
	CacheTTLMinutes    int                `json:"cache_ttl_minutes"`
	SeasonalityEnabled bool               `json:"seasonality_enabled"`
}

// HistoryStore 历史数据存储接口
type HistoryStore interface {
	GetHistoricalData(ctx context.Context, budgetID string, start, end time.Time) ([]HistoricalDataPoint, error)
	GetBudgetInfo(ctx context.Context, budgetID string) (*Budget, error)
}

// NewForecastEngine 创建预测引擎
func NewForecastEngine(historyStore HistoryStore, config ForecastConfig) *ForecastEngine {
	if config.DefaultMethod == "" {
		config.DefaultMethod = ForecastMethodExponential
	}
	if config.DefaultPeriod == "" {
		config.DefaultPeriod = ForecastPeriodMonthly
	}
	if config.DefaultHorizon == 0 {
		config.DefaultHorizon = 3
	}
	if config.DefaultConfidence == "" {
		config.DefaultConfidence = ConfidenceMedium
	}
	if config.MinHistoryDays == 0 {
		config.MinHistoryDays = 30
	}
	if config.MaxHistoryDays == 0 {
		config.MaxHistoryDays = 365
	}

	return &ForecastEngine{
		historyStore:  historyStore,
		config:        config,
		forecastCache: make(map[string]*ForecastResult),
	}
}

// GenerateForecast 生成预测
func (fe *ForecastEngine) GenerateForecast(ctx context.Context, req ForecastRequest) (*ForecastResult, error) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	// 设置默认值
	if req.Method == "" {
		req.Method = fe.config.DefaultMethod
	}
	if req.Period == "" {
		req.Period = fe.config.DefaultPeriod
	}
	if req.Horizon == 0 {
		req.Horizon = fe.config.DefaultHorizon
	}
	if req.Confidence == "" {
		req.Confidence = fe.config.DefaultConfidence
	}
	if req.HistoryDays == 0 {
		req.HistoryDays = fe.config.MaxHistoryDays
	}

	// 获取历史数据
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -req.HistoryDays)

	history, err := fe.historyStore.GetHistoricalData(ctx, req.BudgetID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取历史数据失败: %w", err)
	}

	// 检查数据量是否足够
	// 需要至少有 MinHistoryDays/3 个数据点，或者数据覆盖的天数足够
	minDataPoints := fe.config.MinHistoryDays / 3 // 降低阈值，允许更灵活的数据密度
	if minDataPoints < 3 {
		minDataPoints = 3 // 最少需要 3 个数据点
	}
	if len(history) < minDataPoints {
		return nil, ErrInsufficientHistory
	}

	// 获取预算信息
	budget, err := fe.historyStore.GetBudgetInfo(ctx, req.BudgetID)
	if err != nil {
		budget = &Budget{ID: req.BudgetID}
	}

	// 计算历史统计
	histStats := fe.calculateHistoricalStats(history)

	// 根据预测方法生成预测
	var points []ForecastPoint
	switch req.Method {
	case ForecastMethodMovingAverage:
		points = fe.movingAverageForecast(history, req, histStats)
	case ForecastMethodLinear:
		points = fe.linearRegressionForecast(history, req, histStats)
	case ForecastMethodSeasonal:
		points = fe.seasonalForecast(history, req, histStats)
	case ForecastMethodARIMA:
		points = fe.arimaForecast(history, req, histStats)
	default:
		points = fe.exponentialSmoothingForecast(history, req, histStats)
	}

	// 计算汇总
	summary := fe.calculateSummary(points, budget, histStats)

	// 生成建议
	recommendations := fe.generateRecommendations(points, budget, histStats)

	// 计算准确度
	accuracy := fe.calculateAccuracy(history, req.Method)

	result := &ForecastResult{
		ID:              fmt.Sprintf("forecast_%s_%d", req.BudgetID, time.Now().Unix()),
		BudgetID:        req.BudgetID,
		GeneratedAt:     time.Now(),
		Method:          req.Method,
		Period:          req.Period,
		Confidence:      req.Confidence,
		DataPoints:      points,
		Summary:         summary,
		Recommendations: recommendations,
		Accuracy:        accuracy,
		ModelInfo: ForecastModelInfo{
			Method:       string(req.Method),
			TrainingSize: len(history),
			LastUpdated:  time.Now(),
		},
	}

	// 缓存结果
	if fe.config.CacheEnabled {
		fe.forecastCache[req.BudgetID] = result
	}

	return result, nil
}

// calculateHistoricalStats 计算历史统计
func (fe *ForecastEngine) calculateHistoricalStats(history []HistoricalDataPoint) HistoricalStats {
	if len(history) == 0 {
		return HistoricalStats{}
	}

	stats := HistoricalStats{
		Count:     len(history),
		StartDate: history[0].Date,
		EndDate:   history[len(history)-1].Date,
	}

	// 计算基础统计
	var sum, sumSq float64
	var amounts []float64
	for _, h := range history {
		amounts = append(amounts, h.Amount)
		sum += h.Amount
		sumSq += h.Amount * h.Amount
	}

	stats.Mean = sum / float64(len(history))
	stats.StdDev = math.Sqrt(sumSq/float64(len(history)) - stats.Mean*stats.Mean)

	// 排序计算分位数（使用标准库排序，O(n log n)）
	sorted := make([]float64, len(amounts))
	copy(sorted, amounts)
	sort.Float64s(sorted)

	stats.Min = sorted[0]
	stats.Max = sorted[len(sorted)-1]
	stats.Median = sorted[len(sorted)/2]
	stats.FirstQuartile = sorted[len(sorted)/4]
	stats.ThirdQuartile = sorted[len(sorted)*3/4]

	// 计算趋势（简单线性回归）
	n := float64(len(history))
	var sumX, sumY, sumXY, sumX2 float64
	for i, h := range history {
		x := float64(i)
		sumX += x
		sumY += h.Amount
		sumXY += x * h.Amount
		sumX2 += x * x
	}
	stats.Trend = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	return stats
}

// movingAverageForecast 移动平均预测
func (fe *ForecastEngine) movingAverageForecast(history []HistoricalDataPoint, req ForecastRequest, stats HistoricalStats) []ForecastPoint {
	points := []ForecastPoint{}
	windowSize := 7 // 移动平均窗口

	// 计算最近窗口的平均值
	var sum float64
	start := len(history) - windowSize
	if start < 0 {
		start = 0
	}
	for i := start; i < len(history); i++ {
		sum += history[i].Amount
	}
	avg := sum / float64(len(history)-start)

	// 生成预测点
	now := time.Now()
	for i := 0; i < req.Horizon; i++ {
		date := fe.addPeriod(now, req.Period, i+1)

		// 添加轻微随机波动
		variation := (stats.StdDev * 0.1) * (float64(i+1) / float64(req.Horizon))
		predicted := avg + variation*(2*float64(i%2)-1)

		bounds := fe.calculateBounds(predicted, stats.StdDev, req.Confidence, i)

		points = append(points, ForecastPoint{
			Date:        date,
			PeriodLabel: fe.formatPeriod(date, req.Period),
			Predicted:   predicted,
			LowerBound:  bounds.Lower,
			UpperBound:  bounds.Upper,
			Trend:       fe.determineTrend(predicted, avg),
			Confidence:  fe.calculateConfidenceLevel(req.Confidence, i),
		})
	}

	return points
}

// exponentialSmoothingForecast 指数平滑预测
func (fe *ForecastEngine) exponentialSmoothingForecast(history []HistoricalDataPoint, req ForecastRequest, stats HistoricalStats) []ForecastPoint {
	points := []ForecastPoint{}
	alpha := 0.3 // 平滑系数

	// 计算指数平滑值
	smoothed := history[0].Amount
	for i := 1; i < len(history); i++ {
		smoothed = alpha*history[i].Amount + (1-alpha)*smoothed
	}

	// 添加趋势成分
	trend := stats.Trend

	now := time.Now()
	for i := 0; i < req.Horizon; i++ {
		date := fe.addPeriod(now, req.Period, i+1)
		predicted := smoothed + trend*float64(i+1)

		bounds := fe.calculateBounds(predicted, stats.StdDev, req.Confidence, i)

		points = append(points, ForecastPoint{
			Date:        date,
			PeriodLabel: fe.formatPeriod(date, req.Period),
			Predicted:   predicted,
			LowerBound:  bounds.Lower,
			UpperBound:  bounds.Upper,
			Trend:       fe.determineTrend(predicted, smoothed),
			Confidence:  fe.calculateConfidenceLevel(req.Confidence, i),
		})
	}

	return points
}

// linearRegressionForecast 线性回归预测
func (fe *ForecastEngine) linearRegressionForecast(history []HistoricalDataPoint, req ForecastRequest, stats HistoricalStats) []ForecastPoint {
	points := []ForecastPoint{}
	n := len(history)

	// 计算线性回归参数
	var sumX, sumY, sumXY, sumX2 float64
	for i, h := range history {
		x := float64(i)
		sumX += x
		sumY += h.Amount
		sumXY += x * h.Amount
		sumX2 += x * x
	}

	nf := float64(n)
	slope := (nf*sumXY - sumX*sumY) / (nf*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / nf

	now := time.Now()
	for i := 0; i < req.Horizon; i++ {
		date := fe.addPeriod(now, req.Period, i+1)
		x := float64(n + i)
		predicted := intercept + slope*x

		bounds := fe.calculateBounds(predicted, stats.StdDev, req.Confidence, i)

		points = append(points, ForecastPoint{
			Date:        date,
			PeriodLabel: fe.formatPeriod(date, req.Period),
			Predicted:   predicted,
			LowerBound:  bounds.Lower,
			UpperBound:  bounds.Upper,
			Trend:       fe.determineTrendBySlope(slope),
			Confidence:  fe.calculateConfidenceLevel(req.Confidence, i),
		})
	}

	return points
}

// seasonalForecast 季节性预测
func (fe *ForecastEngine) seasonalForecast(history []HistoricalDataPoint, req ForecastRequest, stats HistoricalStats) []ForecastPoint {
	points := []ForecastPoint{}

	// 计算季节性指数（简化版本：按周几计算）
	seasonalIndices := make(map[int]float64)
	dayCounts := make(map[int]int)
	dayTotals := make(map[int]float64)

	for _, h := range history {
		weekday := int(h.Date.Weekday())
		dayCounts[weekday]++
		dayTotals[weekday] += h.Amount
	}

	for day, count := range dayCounts {
		if count > 0 {
			seasonalIndices[day] = dayTotals[day] / float64(count) / stats.Mean
		}
	}

	// 使用指数平滑作为基础
	alpha := 0.3
	smoothed := history[0].Amount
	for i := 1; i < len(history); i++ {
		smoothed = alpha*history[i].Amount + (1-alpha)*smoothed
	}

	now := time.Now()
	for i := 0; i < req.Horizon; i++ {
		date := fe.addPeriod(now, req.Period, i+1)
		weekday := int(date.Weekday())

		seasonalFactor := 1.0
		if idx, ok := seasonalIndices[weekday]; ok {
			seasonalFactor = idx
		}

		predicted := smoothed * seasonalFactor

		bounds := fe.calculateBounds(predicted, stats.StdDev, req.Confidence, i)

		points = append(points, ForecastPoint{
			Date:        date,
			PeriodLabel: fe.formatPeriod(date, req.Period),
			Predicted:   predicted,
			LowerBound:  bounds.Lower,
			UpperBound:  bounds.Upper,
			Trend:       fe.determineTrend(predicted, smoothed),
			Confidence:  fe.calculateConfidenceLevel(req.Confidence, i),
		})
	}

	return points
}

// arimaForecast ARIMA预测（简化实现）
func (fe *ForecastEngine) arimaForecast(history []HistoricalDataPoint, req ForecastRequest, stats HistoricalStats) []ForecastPoint {
	// 简化的ARIMA实现，实际应用中应使用专业库
	return fe.exponentialSmoothingForecast(history, req, stats)
}

// calculateSummary 计算预测汇总
func (fe *ForecastEngine) calculateSummary(points []ForecastPoint, budget *Budget, stats HistoricalStats) ForecastSummary {
	if len(points) == 0 {
		return ForecastSummary{}
	}

	var total, max, min float64
	max = points[0].Predicted
	min = points[0].Predicted

	for _, p := range points {
		total += p.Predicted
		if p.Predicted > max {
			max = p.Predicted
		}
		if p.Predicted < min {
			min = p.Predicted
		}
	}

	avg := total / float64(len(points))

	// 计算增长率
	growthRate := 0.0
	if len(points) >= 2 && points[0].Predicted > 0 {
		growthRate = (points[len(points)-1].Predicted - points[0].Predicted) / points[0].Predicted * 100
	}

	// 确定整体趋势
	trend := "stable"
	if growthRate > 5 {
		trend = "up"
	} else if growthRate < -5 {
		trend = "down"
	}

	// 计算建议预算
	recommendedBudget := avg * 1.1 // 预留10%缓冲

	// 计算预算偏差
	budgetVariance := 0.0
	if budget != nil && budget.Amount > 0 {
		budgetVariance = (total - budget.Amount) / budget.Amount * 100
	}

	// 风险等级
	riskLevel := "low"
	if budgetVariance > 20 {
		riskLevel = "high"
	} else if budgetVariance > 10 {
		riskLevel = "medium"
	}

	return ForecastSummary{
		TotalPredicted:    total,
		AveragePredicted:  avg,
		MaxPredicted:      max,
		MinPredicted:      min,
		GrowthRate:        growthRate,
		Trend:             trend,
		ConfidenceScore:   points[0].Confidence,
		RiskLevel:         riskLevel,
		RecommendedBudget: recommendedBudget,
		BudgetVariance:    budgetVariance,
	}
}

// generateRecommendations 生成预测建议
func (fe *ForecastEngine) generateRecommendations(points []ForecastPoint, budget *Budget, stats HistoricalStats) []ForecastRecommendation {
	recommendations := []ForecastRecommendation{}

	if budget == nil {
		return recommendations
	}

	var totalPredicted float64
	for _, p := range points {
		totalPredicted += p.Predicted
	}

	// 预算不足警告
	if budget.Amount > 0 && totalPredicted > budget.Amount {
		recommendations = append(recommendations, ForecastRecommendation{
			Type:        "budget_adjust",
			Priority:    "high",
			Title:       "预算可能不足",
			Description: fmt.Sprintf("预测支出 %.2f 元将超出预算 %.2f 元", totalPredicted, budget.Amount),
			Impact:      totalPredicted - budget.Amount,
			Action:      fmt.Sprintf("建议将预算调整为 %.2f 元", totalPredicted*1.1),
			Reason:      "基于历史数据趋势预测",
		})
	}

	// 增长趋势警告
	if stats.Trend > 0 {
		recommendations = append(recommendations, ForecastRecommendation{
			Type:        "alert",
			Priority:    "medium",
			Title:       "支出呈上升趋势",
			Description: fmt.Sprintf("历史数据显示支出以 %.2f 元/期的速度增长", stats.Trend),
			Impact:      stats.Trend * float64(len(points)),
			Action:      "建议分析增长原因并考虑优化措施",
			Reason:      "趋势分析结果",
		})
	}

	// 高波动警告
	if stats.StdDev > stats.Mean*0.3 {
		recommendations = append(recommendations, ForecastRecommendation{
			Type:        "optimize",
			Priority:    "low",
			Title:       "支出波动较大",
			Description: fmt.Sprintf("支出标准差 %.2f，占平均值 %.1f%%", stats.StdDev, stats.StdDev/stats.Mean*100),
			Impact:      0,
			Action:      "建议分析波动原因，考虑是否可以平滑支出",
			Reason:      "波动性分析结果",
		})
	}

	return recommendations
}

// calculateAccuracy 计算预测准确度
func (fe *ForecastEngine) calculateAccuracy(history []HistoricalDataPoint, method ForecastMethod) ForecastAccuracy {
	// 使用历史数据回测计算准确度
	if len(history) < 10 {
		return ForecastAccuracy{}
	}

	// 分割训练集和测试集
	trainSize := int(float64(len(history)) * 0.8)
	trainData := history[:trainSize]
	testData := history[trainSize:]

	// 简单预测
	var sumAPE, sumAE, sumSE float64
	for _, h := range testData {
		// 使用训练集最后几个值的平均作为预测
		predicted := trainData[len(trainData)-1].Amount

		// 计算误差
		ae := math.Abs(h.Amount - predicted)
		ape := ae / h.Amount * 100
		se := ae * ae

		sumAE += ae
		sumAPE += ape
		sumSE += se
	}

	n := float64(len(testData))
	mape := sumAPE / n
	mae := sumAE / n
	rmse := math.Sqrt(sumSE / n)

	return ForecastAccuracy{
		MAPE:         mape,
		MAE:          mae,
		RMSE:         rmse,
		WithinBounds: 80.0, // 假设80%落在置信区间内
		LastVerified: time.Now(),
	}
}

// 辅助方法

type bounds struct {
	Lower, Upper float64
}

func (fe *ForecastEngine) calculateBounds(predicted, stdDev float64, confidence ForecastConfidence, horizonIdx int) bounds {
	multiplier := 1.96 // 95%置信区间
	switch confidence {
	case ConfidenceLow:
		multiplier = 1.28 // 80%
	case ConfidenceMedium:
		multiplier = 1.645 // 90%
	case ConfidenceHigh:
		multiplier = 1.96 // 95%
	case ConfidenceVeryHigh:
		multiplier = 2.576 // 99%
	}

	// 置信区间随预测长度增大
	adjustment := 1.0 + float64(horizonIdx)*0.1
	margin := stdDev * multiplier * adjustment

	return bounds{
		Lower: predicted - margin,
		Upper: predicted + margin,
	}
}

func (fe *ForecastEngine) determineTrend(current, previous float64) string {
	diff := (current - previous) / previous * 100
	if diff > 5 {
		return "up"
	} else if diff < -5 {
		return "down"
	}
	return "stable"
}

func (fe *ForecastEngine) determineTrendBySlope(slope float64) string {
	if slope > 0.1 {
		return "up"
	} else if slope < -0.1 {
		return "down"
	}
	return "stable"
}

func (fe *ForecastEngine) calculateConfidenceLevel(confidence ForecastConfidence, horizonIdx int) float64 {
	base := 95.0
	switch confidence {
	case ConfidenceLow:
		base = 80.0
	case ConfidenceMedium:
		base = 90.0
	case ConfidenceHigh:
		base = 95.0
	case ConfidenceVeryHigh:
		base = 99.0
	}

	// 置信度随预测长度递减
	return base * math.Pow(0.95, float64(horizonIdx))
}

func (fe *ForecastEngine) addPeriod(t time.Time, period ForecastPeriod, n int) time.Time {
	switch period {
	case ForecastPeriodDaily:
		return t.AddDate(0, 0, n)
	case ForecastPeriodWeekly:
		return t.AddDate(0, 0, n*7)
	case ForecastPeriodMonthly:
		return t.AddDate(0, n, 0)
	case ForecastPeriodQuarter:
		return t.AddDate(0, n*3, 0)
	case ForecastPeriodYearly:
		return t.AddDate(n, 0, 0)
	default:
		return t.AddDate(0, n, 0)
	}
}

func (fe *ForecastEngine) formatPeriod(t time.Time, period ForecastPeriod) string {
	switch period {
	case ForecastPeriodDaily:
		return t.Format("2006-01-02")
	case ForecastPeriodWeekly:
		_, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", t.Year(), week)
	case ForecastPeriodMonthly:
		return t.Format("2006-01")
	case ForecastPeriodQuarter:
		q := (int(t.Month())-1)/3 + 1
		return fmt.Sprintf("%d-Q%d", t.Year(), q)
	case ForecastPeriodYearly:
		return fmt.Sprintf("%d", t.Year())
	default:
		return t.Format("2006-01")
	}
}

// GetCachedForecast 获取缓存的预测
func (fe *ForecastEngine) GetCachedForecast(budgetID string) (*ForecastResult, bool) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	result, exists := fe.forecastCache[budgetID]
	return result, exists
}

// ClearCache 清除缓存
func (fe *ForecastEngine) ClearCache() {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	fe.forecastCache = make(map[string]*ForecastResult)
}

// ========== 趋势分析 ==========

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	BudgetID    string           `json:"budget_id"`
	AnalyzedAt  time.Time        `json:"analyzed_at"`
	PeriodDays  int              `json:"period_days"`
	Trend       TrendInfo        `json:"trend"`
	Patterns    []PatternInfo    `json:"patterns"`
	Anomalies   []AnomalyInfo    `json:"anomalies"`
	Projections []ProjectionInfo `json:"projections"`
}

// TrendInfo 趋势信息
type TrendInfo struct {
	Direction    string  `json:"direction"`    // up, down, stable
	Strength     float64 `json:"strength"`     // 0-1
	Slope        float64 `json:"slope"`        // 趋势斜率
	Acceleration float64 `json:"acceleration"` // 加速度
	Confidence   float64 `json:"confidence"`   // 置信度
}

// PatternInfo 模式信息
type PatternInfo struct {
	Type        string  `json:"type"` // seasonal, weekly, monthly
	Description string  `json:"description"`
	Strength    float64 `json:"strength"`
	PeakTime    string  `json:"peak_time,omitempty"`
	LowTime     string  `json:"low_time,omitempty"`
}

// AnomalyInfo 异常信息
type AnomalyInfo struct {
	Date          time.Time `json:"date"`
	Expected      float64   `json:"expected"`
	Actual        float64   `json:"actual"`
	Deviation     float64   `json:"deviation"` // 偏差百分比
	Severity      string    `json:"severity"`  // low, medium, high
	PossibleCause string    `json:"possible_cause,omitempty"`
}

// ProjectionInfo 投影信息
type ProjectionInfo struct {
	TargetDate      time.Time `json:"target_date"`
	TargetAmount    float64   `json:"target_amount"`
	ProjectedAmount float64   `json:"projected_amount"`
	Likelihood      float64   `json:"likelihood"` // 达成概率
	Gap             float64   `json:"gap"`
	GapPercent      float64   `json:"gap_percent"`
}

// AnalyzeTrend 分析趋势
func (fe *ForecastEngine) AnalyzeTrend(ctx context.Context, budgetID string, days int) (*TrendAnalysis, error) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	// 获取历史数据
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	history, err := fe.historyStore.GetHistoricalData(ctx, budgetID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	if len(history) < 7 {
		return nil, ErrInsufficientHistory
	}

	stats := fe.calculateHistoricalStats(history)

	// 分析趋势
	trend := TrendInfo{
		Direction: fe.determineTrendBySlope(stats.Trend),
		Slope:     stats.Trend,
		Strength:  math.Abs(stats.Trend) / stats.Mean,
	}

	// 检测模式
	patterns := fe.detectPatterns(history)

	// 检测异常
	anomalies := fe.detectAnomalies(history, stats)

	// 计算投影
	projections := fe.calculateProjections(history, stats)

	return &TrendAnalysis{
		BudgetID:    budgetID,
		AnalyzedAt:  time.Now(),
		PeriodDays:  days,
		Trend:       trend,
		Patterns:    patterns,
		Anomalies:   anomalies,
		Projections: projections,
	}, nil
}

func (fe *ForecastEngine) detectPatterns(history []HistoricalDataPoint) []PatternInfo {
	patterns := []PatternInfo{}

	// 检测周期性模式（简化）
	weekdayTotals := make(map[time.Weekday]float64)
	weekdayCounts := make(map[time.Weekday]int)

	for _, h := range history {
		weekdayTotals[h.Date.Weekday()] += h.Amount
		weekdayCounts[h.Date.Weekday()]++
	}

	// 找出峰值和谷值日
	var maxDay, minDay time.Weekday
	var maxAvg, minAvg float64
	first := true

	for day, total := range weekdayTotals {
		avg := total / float64(weekdayCounts[day])
		if first {
			maxDay, minDay = day, day
			maxAvg, minAvg = avg, avg
			first = false
		} else {
			if avg > maxAvg {
				maxDay, maxAvg = day, avg
			}
			if avg < minAvg {
				minDay, minAvg = day, avg
			}
		}
	}

	if maxAvg > minAvg*1.2 {
		patterns = append(patterns, PatternInfo{
			Type:        "weekly",
			Description: fmt.Sprintf("周%s支出最高，周%s支出最低", fe.weekdayCN(maxDay), fe.weekdayCN(minDay)),
			Strength:    (maxAvg - minAvg) / maxAvg,
			PeakTime:    maxDay.String(),
			LowTime:     minDay.String(),
		})
	}

	return patterns
}

func (fe *ForecastEngine) weekdayCN(day time.Weekday) string {
	names := []string{"日", "一", "二", "三", "四", "五", "六"}
	return names[int(day)]
}

func (fe *ForecastEngine) detectAnomalies(history []HistoricalDataPoint, stats HistoricalStats) []AnomalyInfo {
	anomalies := []AnomalyInfo{}

	for _, h := range history {
		deviation := 0.0
		if stats.Mean > 0 {
			deviation = (h.Amount - stats.Mean) / stats.Mean * 100
		}

		severity := "low"
		if math.Abs(h.Amount-stats.Mean) > 2*stats.StdDev {
			severity = "high"
		} else if math.Abs(h.Amount-stats.Mean) > stats.StdDev {
			severity = "medium"
		}

		if severity != "low" {
			anomalies = append(anomalies, AnomalyInfo{
				Date:      h.Date,
				Expected:  stats.Mean,
				Actual:    h.Amount,
				Deviation: deviation,
				Severity:  severity,
			})
		}
	}

	return anomalies
}

func (fe *ForecastEngine) calculateProjections(history []HistoricalDataPoint, stats HistoricalStats) []ProjectionInfo {
	projections := []ProjectionInfo{}

	return projections
}
