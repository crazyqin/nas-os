// Package budget 提供预算预测功能测试
package budget

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHistoryStore 模拟历史数据存储
type MockHistoryStore struct {
	data   []HistoricalDataPoint
	budget *Budget
}

func (m *MockHistoryStore) GetHistoricalData(ctx context.Context, budgetID string, start, end time.Time) ([]HistoricalDataPoint, error) {
	var result []HistoricalDataPoint
	for _, d := range m.data {
		if (d.Date.Equal(start) || d.Date.After(start)) && (d.Date.Equal(end) || d.Date.Before(end)) {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *MockHistoryStore) GetBudgetInfo(ctx context.Context, budgetID string) (*Budget, error) {
	if m.budget != nil {
		return m.budget, nil
	}
	return &Budget{ID: budgetID, Amount: 10000}, nil
}

func generateTestData(days int, baseAmount float64) []HistoricalDataPoint {
	data := []HistoricalDataPoint{}
	now := time.Now()
	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -days + i)
		// 添加一些波动
		amount := baseAmount + float64(i%7)*10
		data = append(data, HistoricalDataPoint{
			Date:     date,
			Amount:   amount,
			BudgetID: "test-budget",
		})
	}
	return data
}

func TestNewForecastEngine(t *testing.T) {
	store := &MockHistoryStore{}
	config := ForecastConfig{
		DefaultMethod:     ForecastMethodExponential,
		DefaultPeriod:     ForecastPeriodMonthly,
		DefaultHorizon:    3,
		DefaultConfidence: ConfidenceMedium,
	}

	engine := NewForecastEngine(store, config)
	require.NotNil(t, engine)
	assert.Equal(t, ForecastMethodExponential, engine.config.DefaultMethod)
}

func TestGenerateForecast(t *testing.T) {
	store := &MockHistoryStore{
		data: generateTestData(60, 100),
	}

	engine := NewForecastEngine(store, ForecastConfig{
		MinHistoryDays: 30,
	})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID:   "test-budget",
		Method:     ForecastMethodExponential,
		Period:     ForecastPeriodMonthly,
		Horizon:    3,
		Confidence: ConfidenceMedium,
	}

	result, err := engine.GenerateForecast(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "test-budget", result.BudgetID)
	assert.Equal(t, ForecastMethodExponential, result.Method)
	assert.Len(t, result.DataPoints, 3)
}

func TestMovingAverageForecast(t *testing.T) {
	store := &MockHistoryStore{
		data: generateTestData(30, 100),
	}

	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID:   "test-budget",
		Method:     ForecastMethodMovingAverage,
		Period:     ForecastPeriodDaily,
		Horizon:    7,
		Confidence: ConfidenceHigh,
	}

	result, err := engine.GenerateForecast(ctx, req)
	require.NoError(t, err)
	assert.Len(t, result.DataPoints, 7)

	// 验证每个数据点都有预测值
	for _, point := range result.DataPoints {
		assert.Greater(t, point.Predicted, 0.0)
		assert.LessOrEqual(t, point.LowerBound, point.Predicted)
		assert.GreaterOrEqual(t, point.UpperBound, point.Predicted)
	}
}

func TestLinearRegressionForecast(t *testing.T) {
	// 创建有明显趋势的数据
	data := []HistoricalDataPoint{}
	now := time.Now()
	for i := 0; i < 60; i++ {
		date := now.AddDate(0, 0, -60 + i)
		amount := 100 + float64(i)*2 // 线性增长
		data = append(data, HistoricalDataPoint{
			Date:     date,
			Amount:   amount,
			BudgetID: "trend-budget",
		})
	}

	store := &MockHistoryStore{data: data}
	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID: "trend-budget",
		Method:   ForecastMethodLinear,
		Period:   ForecastPeriodMonthly,
		Horizon:  3,
	}

	result, err := engine.GenerateForecast(ctx, req)
	require.NoError(t, err)
	assert.Len(t, result.DataPoints, 3)

	// 验证趋势是上升的
	assert.Equal(t, "up", result.DataPoints[len(result.DataPoints)-1].Trend)
}

func TestSeasonalForecast(t *testing.T) {
	// 创建有周期性的数据
	data := []HistoricalDataPoint{}
	now := time.Now()
	for i := 0; i < 60; i++ {
		date := now.AddDate(0, 0, -60 + i)
		// 模拟周末消费更高的模式
		baseAmount := 100.0
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			baseAmount = 150.0
		}
		data = append(data, HistoricalDataPoint{
			Date:     date,
			Amount:   baseAmount,
			BudgetID: "seasonal-budget",
		})
	}

	store := &MockHistoryStore{data: data}
	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID:    "seasonal-budget",
		Method:      ForecastMethodSeasonal,
		Period:      ForecastPeriodDaily,
		Horizon:     7,
		Seasonality: true,
	}

	result, err := engine.GenerateForecast(ctx, req)
	require.NoError(t, err)
	assert.Len(t, result.DataPoints, 7)
}

func TestForecastSummary(t *testing.T) {
	store := &MockHistoryStore{
		data:   generateTestData(60, 100),
		budget: &Budget{ID: "test-budget", Amount: 10000},
	}

	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID:   "test-budget",
		Period:     ForecastPeriodMonthly,
		Horizon:    3,
	}

	result, err := engine.GenerateForecast(ctx, req)
	require.NoError(t, err)

	// 验证汇总
	assert.NotEmpty(t, result.Summary.Trend)
	assert.GreaterOrEqual(t, result.Summary.TotalPredicted, 0.0)
	assert.GreaterOrEqual(t, result.Summary.ConfidenceScore, 0.0)
	assert.NotEmpty(t, result.Summary.RiskLevel)
}

func TestForecastRecommendations(t *testing.T) {
	// 创建高消费场景
	highData := []HistoricalDataPoint{}
	now := time.Now()
	for i := 0; i < 60; i++ {
		date := now.AddDate(0, 0, -60 + i)
		highData = append(highData, HistoricalDataPoint{
			Date:     date,
			Amount:   500, // 高消费
			BudgetID: "high-budget",
		})
	}

	store := &MockHistoryStore{
		data:   highData,
		budget: &Budget{ID: "high-budget", Amount: 1000}, // 低预算
	}

	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID: "high-budget",
		Period:   ForecastPeriodMonthly,
		Horizon:  3,
	}

	result, err := engine.GenerateForecast(ctx, req)
	require.NoError(t, err)

	// 应该有建议
	assert.NotEmpty(t, result.Recommendations)

	// 检查是否有预算调整建议
	hasBudgetRecommendation := false
	for _, r := range result.Recommendations {
		if r.Type == "budget_adjust" {
			hasBudgetRecommendation = true
			break
		}
	}
	assert.True(t, hasBudgetRecommendation, "应该有预算调整建议")
}

func TestHistoricalStats(t *testing.T) {
	data := []HistoricalDataPoint{
		{Date: time.Now().AddDate(0, 0, -5), Amount: 100},
		{Date: time.Now().AddDate(0, 0, -4), Amount: 150},
		{Date: time.Now().AddDate(0, 0, -3), Amount: 120},
		{Date: time.Now().AddDate(0, 0, -2), Amount: 180},
		{Date: time.Now().AddDate(0, 0, -1), Amount: 130},
	}

	store := &MockHistoryStore{data: data}
	engine := NewForecastEngine(store, ForecastConfig{})

	stats := engine.calculateHistoricalStats(data)

	assert.Equal(t, 5, stats.Count)
	assert.Equal(t, 100.0, stats.Min)
	assert.Equal(t, 180.0, stats.Max)
	assert.Greater(t, stats.Mean, 0.0)
	assert.GreaterOrEqual(t, stats.StdDev, 0.0)
}

func TestConfidenceBounds(t *testing.T) {
	store := &MockHistoryStore{
		data: generateTestData(60, 100),
	}

	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()

	// 测试不同置信度
	confidences := []ForecastConfidence{ConfidenceLow, ConfidenceMedium, ConfidenceHigh, ConfidenceVeryHigh}

	for _, conf := range confidences {
		req := ForecastRequest{
			BudgetID:   "test-budget",
			Horizon:    3,
			Confidence: conf,
		}

		result, err := engine.GenerateForecast(ctx, req)
		require.NoError(t, err)

		for _, point := range result.DataPoints {
			// 置信区间应该合理
			assert.LessOrEqual(t, point.LowerBound, point.Predicted)
			assert.GreaterOrEqual(t, point.UpperBound, point.Predicted)
		}
	}
}

func TestAnalyzeTrend(t *testing.T) {
	// 创建有上升趋势的数据
	trendData := []HistoricalDataPoint{}
	now := time.Now()
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -30 + i)
		trendData = append(trendData, HistoricalDataPoint{
			Date:     date,
			Amount:   100 + float64(i)*5, // 明显上升趋势
			BudgetID: "trend-test",
		})
	}

	store := &MockHistoryStore{data: trendData}
	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()
	analysis, err := engine.AnalyzeTrend(ctx, "trend-test", 30)
	require.NoError(t, err)

	assert.Equal(t, "trend-test", analysis.BudgetID)
	assert.Equal(t, 30, analysis.PeriodDays)
	assert.NotEmpty(t, analysis.Trend.Direction)
	assert.Greater(t, analysis.Trend.Strength, 0.0)
}

func TestInsufficientHistory(t *testing.T) {
	// 只有几天数据
	store := &MockHistoryStore{
		data: generateTestData(5, 100),
	}

	engine := NewForecastEngine(store, ForecastConfig{
		MinHistoryDays: 30,
	})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID: "test-budget",
		HistoryDays: 30, // 请求30天历史数据
	}

	_, err := engine.GenerateForecast(ctx, req)
	assert.Error(t, err)
	assert.Equal(t, ErrInsufficientHistory, err)
}

func TestForecastCache(t *testing.T) {
	store := &MockHistoryStore{
		data: generateTestData(60, 100),
	}

	engine := NewForecastEngine(store, ForecastConfig{
		CacheEnabled:    true,
		CacheTTLMinutes: 60,
	})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID: "test-budget",
	}

	// 生成预测
	result1, err := engine.GenerateForecast(ctx, req)
	require.NoError(t, err)

	// 从缓存获取
	cached, exists := engine.GetCachedForecast("test-budget")
	assert.True(t, exists)
	assert.Equal(t, result1.ID, cached.ID)

	// 清除缓存
	engine.ClearCache()
	_, exists = engine.GetCachedForecast("test-budget")
	assert.False(t, exists)
}

func TestPeriodCalculations(t *testing.T) {
	store := &MockHistoryStore{
		data: generateTestData(90, 100),
	}

	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()

	tests := []struct {
		period   ForecastPeriod
		horizon  int
		expected int
	}{
		{ForecastPeriodDaily, 7, 7},
		{ForecastPeriodWeekly, 4, 4},
		{ForecastPeriodMonthly, 3, 3},
		{ForecastPeriodQuarter, 2, 2},
		{ForecastPeriodYearly, 1, 1},
	}

	for _, tt := range tests {
		req := ForecastRequest{
			BudgetID: "test-budget",
			Period:   tt.period,
			Horizon:  tt.horizon,
		}

		result, err := engine.GenerateForecast(ctx, req)
		require.NoError(t, err)
		assert.Len(t, result.DataPoints, tt.expected, "period: %s", tt.period)
	}
}

func TestForecastAccuracy(t *testing.T) {
	store := &MockHistoryStore{
		data: generateTestData(100, 100),
	}

	engine := NewForecastEngine(store, ForecastConfig{})

	ctx := context.Background()
	req := ForecastRequest{
		BudgetID: "test-budget",
	}

	result, err := engine.GenerateForecast(ctx, req)
	require.NoError(t, err)

	// 验证准确度指标
	assert.GreaterOrEqual(t, result.Accuracy.MAPE, 0.0)
	assert.GreaterOrEqual(t, result.Accuracy.MAE, 0.0)
	assert.GreaterOrEqual(t, result.Accuracy.RMSE, 0.0)
}