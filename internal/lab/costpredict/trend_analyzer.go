// trend_analyzer.go 提供存储成本趋势分析功能。
// 包含移动平均、季节性检测、异常检测、预测精度验证和趋势报告生成。
package costpredict

import (
	"math"
	"sort"
	"time"
)

// ========== 趋势方向常量 ==========

const (
	// TrendRising 上升趋势.
	TrendRising = "rising"
	// TrendFalling 下降趋势.
	TrendFalling = "falling"
	// TrendStable 平稳趋势.
	TrendStable = "stable"
)

// ========== 趋势分析数据结构 ==========

// TrendAnalyzer 趋势分析器.
type TrendAnalyzer struct {
	predictor *Predictor
}

// NewTrendAnalyzer 创建趋势分析器.
func NewTrendAnalyzer(predictor *Predictor) *TrendAnalyzer {
	return &TrendAnalyzer{predictor: predictor}
}

// MovingAverageResult 移动平均结果.
type MovingAverageResult struct {
	// WindowSize 窗口大小.
	WindowSize int `json:"window_size"`
	// Values 平滑后的值.
	Values []float64 `json:"values"`
	// Dates 对应日期.
	Dates []time.Time `json:"dates"`
}

// SeasonalityResult 季节性检测结果.
type SeasonalityResult struct {
	// Pattern 模式: monthly/quarterly.
	Pattern string `json:"pattern"`
	// Confidence 置信度 (0-1).
	Confidence float64 `json:"confidence"`
	// Peaks 峰值月份/季度.
	Peaks []int `json:"peaks"`
	// Troughs 谷值月份/季度.
	Troughs []int `json:"troughs"`
	// PeriodAverages 各周期平均值.
	PeriodAverages []float64 `json:"period_averages"`
}

// CostAnomaly 成本异常.
type CostAnomaly struct {
	// Time 异常时间.
	Time time.Time `json:"time"`
	// Actual 实际值.
	Actual float64 `json:"actual"`
	// Predicted 预测值.
	Predicted float64 `json:"predicted"`
	// Deviation 偏离百分比.
	Deviation float64 `json:"deviation"`
	// AlertLevel 告警等级: warning/critical.
	AlertLevel string `json:"alert_level"`
}

// PredictionAccuracy 预测精度验证结果.
type PredictionAccuracy struct {
	// Method 预测方法.
	Method string `json:"method"`
	// MAE 平均绝对误差.
	MAE float64 `json:"mae"`
	// MAPE 平均绝对百分比误差.
	MAPE float64 `json:"mape"`
	// RMSE 均方根误差.
	RMSE float64 `json:"rmse"`
	// SampleSize 样本量.
	SampleSize int `json:"sample_size"`
}

// ConfidenceInterval 置信区间.
type ConfidenceInterval struct {
	// Lower95 95%置信区间下限.
	Lower95 float64 `json:"lower_95"`
	// Upper95 95%置信区间上限.
	Upper95 float64 `json:"upper_95"`
	// Lower80 80%置信区间下限.
	Lower80 float64 `json:"lower_80"`
	// Upper80 80%置信区间上限.
	Upper80 float64 `json:"upper_80"`
}

// TrendReport 趋势分析报告.
type TrendReport struct {
	// Direction 趋势方向: rising/falling/stable.
	Direction string `json:"direction"`
	// TrendStrength 趋势强度 (-1 到 1).
	TrendStrength float64 `json:"trend_strength"`
	// PredictedCosts 预测成本.
	PredictedCosts []PredictionResult `json:"predicted_costs"`
	// ConfidenceInterval 置信区间.
	ConfidenceInterval ConfidenceInterval `json:"confidence_interval"`
	// MovingAverages 移动平均结果.
	MovingAverages []MovingAverageResult `json:"moving_averages"`
	// Seasonality 季节性检测结果.
	Seasonality []SeasonalityResult `json:"seasonality"`
	// Anomalies 成本异常.
	Anomalies []CostAnomaly `json:"anomalies"`
	// Accuracy 预测精度.
	Accuracy []PredictionAccuracy `json:"accuracy"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
}

// ========== 移动平均算法 ==========

// CalculateMovingAverage 计算简单移动平均.
// windowSize 为窗口大小，data 为按时间排序的成本数据.
func CalculateMovingAverage(data []float64, windowSize int) ([]float64, error) {
	if len(data) == 0 {
		return nil, ErrInsufficientData
	}
	if windowSize <= 0 {
		windowSize = 7
	}
	if windowSize > len(data) {
		windowSize = len(data)
	}

	result := make([]float64, len(data))
	// 前 windowSize-1 个点使用扩展窗口
	for i := 0; i < len(data); i++ {
		start := 0
		if i >= windowSize-1 {
			start = i - windowSize + 1
		}
		count := float64(i - start + 1)
		var sum float64
		for j := start; j <= i; j++ {
			sum += data[j]
		}
		result[i] = sum / count
	}
	return result, nil
}

// CalculateWeightedMovingAverage 计算加权移动平均（近期权重更高）.
func CalculateWeightedMovingAverage(data []float64, windowSize int) ([]float64, error) {
	if len(data) == 0 {
		return nil, ErrInsufficientData
	}
	if windowSize <= 0 {
		windowSize = 7
	}
	if windowSize > len(data) {
		windowSize = len(data)
	}

	result := make([]float64, len(data))
	for i := 0; i < len(data); i++ {
		start := 0
		if i >= windowSize-1 {
			start = i - windowSize + 1
		}
		var sum, weightSum float64
		for j := start; j <= i; j++ {
			w := float64(j - start + 1) // 近期权重更大
			sum += data[j] * w
			weightSum += w
		}
		result[i] = sum / weightSum
	}
	return result, nil
}

// AnalyzeMovingAverages 分析7天和30天移动平均.
func (ta *TrendAnalyzer) AnalyzeMovingAverages() ([]MovingAverageResult, error) {
	records := ta.predictor.GetRecords()
	if len(records) < 2 {
		return nil, ErrInsufficientData
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.Before(records[j].Time)
	})

	costs := make([]float64, len(records))
	dates := make([]time.Time, len(records))
	for i, r := range records {
		costs[i] = r.Cost
		dates[i] = r.Time
	}

	var results []MovingAverageResult

	// 7天窗口移动平均
	ma7, err := CalculateMovingAverage(costs, 7)
	if err != nil {
		return nil, err
	}
	results = append(results, MovingAverageResult{
		WindowSize: 7,
		Values:     ma7,
		Dates:      dates,
	})

	// 30天窗口移动平均
	ma30, err := CalculateMovingAverage(costs, 30)
	if err != nil {
		return nil, err
	}
	results = append(results, MovingAverageResult{
		WindowSize: 30,
		Values:     ma30,
		Dates:      dates,
	})

	return results, nil
}

// ========== 季节性检测 ==========

// DetectSeasonality 检测存储增长的季节性模式.
// 检测月度和季度两种周期.
func (ta *TrendAnalyzer) DetectSeasonality() ([]SeasonalityResult, error) {
	records := ta.predictor.GetRecords()
	if len(records) < 2 {
		return nil, ErrInsufficientData
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.Before(records[j].Time)
	})

	// 检测月度季节性
	monthly := ta.detectMonthlyPattern(records)
	// 检测季度季节性
	quarterly := ta.detectQuarterlyPattern(records)

	var results []SeasonalityResult
	if monthly != nil {
		results = append(results, *monthly)
	}
	if quarterly != nil {
		results = append(results, *quarterly)
	}
	return results, nil
}

// detectMonthlyPattern 检测月度季节性模式.
func (ta *TrendAnalyzer) detectMonthlyPattern(records []CostRecord) *SeasonalityResult {
	// 按月分组统计
	monthlySums := make(map[int]float64)
	monthlyCounts := make(map[int]int)
	for _, r := range records {
		month := int(r.Time.Month())
		monthlySums[month] += r.Cost
		monthlyCounts[month]++
	}

	// 至少需要2个月的数据
	if len(monthlySums) < 2 {
		return nil
	}

	// 计算各月平均值
	periodAverages := make([]float64, 12)
	for m := 1; m <= 12; m++ {
		if monthlyCounts[m] > 0 {
			periodAverages[m-1] = monthlySums[m] / float64(monthlyCounts[m])
		}
	}

	// 计算总平均值
	var totalSum float64
	var totalCount int
	for m := 1; m <= 12; m++ {
		totalSum += monthlySums[m]
		totalCount += monthlyCounts[m]
	}
	if totalCount == 0 {
		return nil
	}
	overallAvg := totalSum / float64(totalCount)

	// 检测峰值和谷值（偏离均值超过10%）
	var peaks, troughs []int
	for m := 0; m < 12; m++ {
		if monthlyCounts[m+1] == 0 {
			continue
		}
		deviation := (periodAverages[m] - overallAvg) / overallAvg
		if deviation > 0.1 {
			peaks = append(peaks, m+1)
		} else if deviation < -0.1 {
			troughs = append(troughs, m+1)
		}
	}

	// 计算置信度：基于数据覆盖月数和变异系数
	variance := 0.0
	validMonths := 0
	for m := 1; m <= 12; m++ {
		if monthlyCounts[m] > 0 {
			diff := monthlySums[m]/float64(monthlyCounts[m]) - overallAvg
			variance += diff * diff
			validMonths++
		}
	}
	variance /= float64(validMonths)
	stdDev := math.Sqrt(variance)
	cv := 0.0
	if overallAvg > 0 {
		cv = stdDev / overallAvg
	}
	// 覆盖月数比例 * (1 - 变异系数) 作为置信度
	confidence := math.Min(1.0, float64(validMonths)/12.0) * math.Max(0, 1.0-cv)

	return &SeasonalityResult{
		Pattern:        "monthly",
		Confidence:     math.Round(confidence*100) / 100,
		Peaks:          peaks,
		Troughs:        troughs,
		PeriodAverages: periodAverages,
	}
}

// detectQuarterlyPattern 检测季度季节性模式.
func (ta *TrendAnalyzer) detectQuarterlyPattern(records []CostRecord) *SeasonalityResult {
	// 按季度分组
	quarterlySums := make(map[int]float64)
	quarterlyCounts := make(map[int]int)
	for _, r := range records {
		quarter := (int(r.Time.Month())-1)/3 + 1
		quarterlySums[quarter] += r.Cost
		quarterlyCounts[quarter]++
	}

	if len(quarterlySums) < 2 {
		return nil
	}

	periodAverages := make([]float64, 4)
	for q := 1; q <= 4; q++ {
		if quarterlyCounts[q] > 0 {
			periodAverages[q-1] = quarterlySums[q] / float64(quarterlyCounts[q])
		}
	}

	var totalSum float64
	var totalCount int
	for q := 1; q <= 4; q++ {
		totalSum += quarterlySums[q]
		totalCount += quarterlyCounts[q]
	}
	if totalCount == 0 {
		return nil
	}
	overallAvg := totalSum / float64(totalCount)

	var peaks, troughs []int
	for q := 0; q < 4; q++ {
		if quarterlyCounts[q+1] == 0 {
			continue
		}
		deviation := (periodAverages[q] - overallAvg) / overallAvg
		if deviation > 0.1 {
			peaks = append(peaks, q+1)
		} else if deviation < -0.1 {
			troughs = append(troughs, q+1)
		}
	}

	variance := 0.0
	validQuarters := 0
	for q := 1; q <= 4; q++ {
		if quarterlyCounts[q] > 0 {
			diff := quarterlySums[q]/float64(quarterlyCounts[q]) - overallAvg
			variance += diff * diff
			validQuarters++
		}
	}
	variance /= float64(validQuarters)
	cv := 0.0
	if overallAvg > 0 {
		cv = math.Sqrt(variance) / overallAvg
	}
	confidence := math.Min(1.0, float64(validQuarters)/4.0) * math.Max(0, 1.0-cv)

	return &SeasonalityResult{
		Pattern:        "quarterly",
		Confidence:     math.Round(confidence*100) / 100,
		Peaks:          peaks,
		Troughs:        troughs,
		PeriodAverages: periodAverages,
	}
}

// ========== 成本异常检测 ==========

// DetectAnomalies 检测成本异常（偏离预测超过阈值）.
// threshold 为偏离阈值，默认0.2（20%）.
func (ta *TrendAnalyzer) DetectAnomalies(threshold float64) ([]CostAnomaly, error) {
	if threshold <= 0 {
		threshold = 0.2
	}

	records := ta.predictor.GetRecords()
	if len(records) < 4 {
		return nil, ErrInsufficientData
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.Before(records[j].Time)
	})

	costs := make([]float64, len(records))
	for i, r := range records {
		costs[i] = r.Cost
	}

	// 使用指数平滑生成预测值序列
	level, trend, err := DoubleExponentialSmoothing(costs, 0.3, 0.1)
	if err != nil {
		return nil, err
	}

	var anomalies []CostAnomaly
	for i := 1; i < len(records); i++ {
		predicted := level[i-1] + trend[i-1]
		actual := costs[i]

		if predicted == 0 {
			continue
		}
		deviation := math.Abs(actual-predicted) / predicted

		if deviation > threshold {
			alertLevel := "warning"
			if deviation > threshold*2 {
				alertLevel = "critical"
			}
			anomalies = append(anomalies, CostAnomaly{
				Time:       records[i].Time,
				Actual:     actual,
				Predicted:  predicted,
				Deviation:  math.Round(deviation*10000) / 100, // 保留2位小数百分比
				AlertLevel: alertLevel,
			})
		}
	}
	return anomalies, nil
}

// ========== 预测精度验证 ==========

// ValidatePredictionAccuracy 验证预测精度.
// 使用前 trainRatio 比例的数据训练，用剩余数据验证.
func (ta *TrendAnalyzer) ValidatePredictionAccuracy(trainRatio float64) ([]PredictionAccuracy, error) {
	if trainRatio <= 0 || trainRatio >= 1 {
		trainRatio = 0.7
	}

	records := ta.predictor.GetRecords()
	if len(records) < 6 {
		return nil, ErrInsufficientData
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.Before(records[j].Time)
	})

	costs := make([]float64, len(records))
	for i, r := range records {
		costs[i] = r.Cost
	}

	splitIdx := int(float64(len(costs)) * trainRatio)
	if splitIdx < 2 {
		splitIdx = 2
	}
	if splitIdx >= len(costs)-1 {
		splitIdx = len(costs) - 2
	}

	trainData := costs[:splitIdx]
	testData := costs[splitIdx:]

	var accuracies []PredictionAccuracy

	// 方法1: 线性回归精度
	lrMAE, lrMAPE, lrRMSE := ta.evaluateLinearRegression(trainData, testData)
	accuracies = append(accuracies, PredictionAccuracy{
		Method:     "linear_regression",
		MAE:        math.Round(lrMAE*100) / 100,
		MAPE:       math.Round(lrMAPE*10000) / 100, // 百分比形式
		RMSE:       math.Round(lrRMSE*100) / 100,
		SampleSize: len(testData),
	})

	// 方法2: 指数平滑精度
	esMAE, esMAPE, esRMSE := ta.evaluateExponentialSmoothing(trainData, testData)
	accuracies = append(accuracies, PredictionAccuracy{
		Method:     "exponential_smoothing",
		MAE:        math.Round(esMAE*100) / 100,
		MAPE:       math.Round(esMAPE*10000) / 100,
		RMSE:       math.Round(esRMSE*100) / 100,
		SampleSize: len(testData),
	})

	return accuracies, nil
}

// evaluateLinearRegression 评估线性回归预测精度.
func (ta *TrendAnalyzer) evaluateLinearRegression(train, test []float64) (mae, mape, rmse float64) {
	x := make([]float64, len(train))
	for i := range x {
		x[i] = float64(i)
	}
	slope, intercept, err := LinearRegression(x, train)
	if err != nil {
		return 0, 0, 0
	}

	var sumAbsErr, sumPctErr, sumSqErr float64
	n := float64(len(test))
	for i, actual := range test {
		predicted := slope*float64(len(train)+i) + intercept
		predicted = math.Max(0, predicted)
		absErr := math.Abs(actual - predicted)
		sumAbsErr += absErr
		if actual > 0 {
			sumPctErr += absErr / actual
		}
		sumSqErr += absErr * absErr
	}

	mae = sumAbsErr / n
	mape = sumPctErr / n
	rmse = math.Sqrt(sumSqErr / n)
	return
}

// evaluateExponentialSmoothing 评估指数平滑预测精度.
func (ta *TrendAnalyzer) evaluateExponentialSmoothing(train, test []float64) (mae, mape, rmse float64) {
	level, trend, err := DoubleExponentialSmoothing(train, 0.3, 0.1)
	if err != nil {
		return 0, 0, 0
	}

	n := len(level)
	lastLevel := level[n-1]
	lastTrend := trend[n-1]

	var sumAbsErr, sumPctErr, sumSqErr float64
	count := float64(len(test))
	for i, actual := range test {
		predicted := lastLevel + float64(i+1)*lastTrend
		predicted = math.Max(0, predicted)
		absErr := math.Abs(actual - predicted)
		sumAbsErr += absErr
		if actual > 0 {
			sumPctErr += absErr / actual
		}
		sumSqErr += absErr * absErr
	}

	mae = sumAbsErr / count
	mape = sumPctErr / count
	rmse = math.Sqrt(sumSqErr / count)
	return
}

// ========== 趋势报告生成 ==========

// GenerateTrendReport 生成完整的趋势分析报告.
func (ta *TrendAnalyzer) GenerateTrendReport(periodsAhead int) (*TrendReport, error) {
	if periodsAhead < 1 {
		periodsAhead = 3
	}

	records := ta.predictor.GetRecords()
	if len(records) < 2 {
		return nil, ErrInsufficientData
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.Before(records[j].Time)
	})

	report := &TrendReport{
		GeneratedAt: time.Now(),
	}

	// 1. 计算趋势方向和强度
	direction, strength := ta.calculateTrendDirection(records)
	report.Direction = direction
	report.TrendStrength = math.Round(strength*100) / 100

	// 2. 生成预测
	predictions, err := ta.predictor.PredictCost(periodsAhead)
	if err == nil {
		report.PredictedCosts = predictions
	}

	// 3. 计算置信区间
	report.ConfidenceInterval = ta.calculateConfidenceInterval(records)

	// 4. 移动平均分析
	maResults, err := ta.AnalyzeMovingAverages()
	if err == nil {
		report.MovingAverages = maResults
	}

	// 5. 季节性检测
	seasonality, err := ta.DetectSeasonality()
	if err == nil {
		report.Seasonality = seasonality
	}

	// 6. 异常检测
	anomalies, err := ta.DetectAnomalies(0.2)
	if err == nil {
		report.Anomalies = anomalies
	}

	// 7. 预测精度验证
	accuracy, err := ta.ValidatePredictionAccuracy(0.7)
	if err == nil {
		report.Accuracy = accuracy
	}

	return report, nil
}

// calculateTrendDirection 计算趋势方向和强度.
func (ta *TrendAnalyzer) calculateTrendDirection(records []CostRecord) (string, float64) {
	costs := make([]float64, len(records))
	for i, r := range records {
		costs[i] = r.Cost
	}

	// 使用线性回归斜率判断趋势方向
	x := make([]float64, len(costs))
	for i := range x {
		x[i] = float64(i)
	}
	slope, _, err := LinearRegression(x, costs)
	if err != nil {
		return TrendStable, 0
	}

	// 归一化斜率作为强度（相对均值的比率）
	avgCost := 0.0
	for _, c := range costs {
		avgCost += c
	}
	avgCost /= float64(len(costs))

	strength := 0.0
	if avgCost > 0 {
		strength = slope / avgCost * float64(len(costs))
	}

	// 限制在 [-1, 1] 范围
	strength = math.Max(-1, math.Min(1, strength))

	// 判断方向
	if math.Abs(strength) < 0.05 {
		return TrendStable, strength
	}
	if strength > 0 {
		return TrendRising, strength
	}
	return TrendFalling, strength
}

// calculateConfidenceInterval 计算预测置信区间.
func (ta *TrendAnalyzer) calculateConfidenceInterval(records []CostRecord) ConfidenceInterval {
	costs := make([]float64, len(records))
	for i, r := range records {
		costs[i] = r.Cost
	}

	// 计算标准差
	mean := 0.0
	for _, c := range costs {
		mean += c
	}
	mean /= float64(len(costs))

	variance := 0.0
	for _, c := range costs {
		diff := c - mean
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(costs)))

	// 使用最近值作为基准
	lastCost := costs[len(costs)-1]

	return ConfidenceInterval{
		Lower95: math.Max(0, lastCost-1.96*stdDev),
		Upper95: lastCost + 1.96*stdDev,
		Lower80: math.Max(0, lastCost-1.28*stdDev),
		Upper80: lastCost + 1.28*stdDev,
	}
}
