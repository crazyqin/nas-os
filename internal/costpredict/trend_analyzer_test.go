// trend_analyzer_test.go 趋势分析器测试.
package costpredict

import (
	"math"
	"testing"
	"time"
)

// ========== 测试辅助函数 ==========

// newTrendTestPredictor 创建带30条记录的测试预测器.
func newTrendTestPredictor() *Predictor {
	p := NewPredictor()
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		p.AddRecord(CostRecord{
			Time:          baseTime.AddDate(0, 0, i),
			Department:    "IT",
			Project:       "Storage",
			StorageType:   StorageTypeSSD,
			Cost:          float64(1000 + i*50 + (i%7)*20),
			UsedCapacity:  int64(1000000000 + i*50000000),
			TotalCapacity: 10000000000,
		})
	}
	return p
}

// newSeasonalPredictor 创建有季节性模式的测试预测器.
func newSeasonalPredictor() *Predictor {
	p := NewPredictor()
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	// 创建12个月数据，模拟季度波动
	seasonalMultiplier := []float64{1.0, 0.9, 0.8, 1.1, 1.2, 1.3, 0.9, 0.85, 0.95, 1.15, 1.25, 1.35}
	for i := 0; i < 12; i++ {
		p.AddRecord(CostRecord{
			Time:          baseTime.AddDate(0, i, 0),
			Department:    "IT",
			Project:       "Storage",
			StorageType:   StorageTypeSSD,
			Cost:          1000 * seasonalMultiplier[i],
			UsedCapacity:  int64(1000000000 * seasonalMultiplier[i]),
			TotalCapacity: 10000000000,
		})
	}
	return p
}

// ========== 移动平均测试 ==========

func TestCalculateMovingAverage_Basic(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50}
	result, err := CalculateMovingAverage(data, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 5 {
		t.Fatalf("expected length 5, got %d", len(result))
	}
	// 第一个值是自身
	if math.Abs(result[0]-10) > 0.01 {
		t.Errorf("expected 10, got %f", result[0])
	}
	// 第三个值是前3个的平均
	if math.Abs(result[2]-20) > 0.01 {
		t.Errorf("expected 20, got %f", result[2])
	}
	// 最后一个值是最后3个的平均
	if math.Abs(result[4]-40) > 0.01 {
		t.Errorf("expected 40, got %f", result[4])
	}
}

func TestCalculateMovingAverage_EmptyData(t *testing.T) {
	_, err := CalculateMovingAverage([]float64{}, 7)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestCalculateMovingAverage_WindowLargerThanData(t *testing.T) {
	data := []float64{10, 20}
	result, err := CalculateMovingAverage(data, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 窗口被限制为数据长度
	if math.Abs(result[0]-10) > 0.01 {
		t.Errorf("expected 10, got %f", result[0])
	}
	if math.Abs(result[1]-15) > 0.01 {
		t.Errorf("expected 15, got %f", result[1])
	}
}

func TestCalculateWeightedMovingAverage_Basic(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50}
	result, err := CalculateWeightedMovingAverage(data, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 5 {
		t.Fatalf("expected length 5, got %d", len(result))
	}
	// 加权平均应偏向近期值
	if result[4] <= result[3] {
		t.Error("weighted average should favor recent values")
	}
}

func TestTrendAnalyzer_AnalyzeMovingAverages(t *testing.T) {
	ta := NewTrendAnalyzer(newTrendTestPredictor())
	results, err := ta.AnalyzeMovingAverages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (7-day and 30-day), got %d", len(results))
	}
	if results[0].WindowSize != 7 {
		t.Errorf("expected window 7, got %d", results[0].WindowSize)
	}
	if results[1].WindowSize != 30 {
		t.Errorf("expected window 30, got %d", results[1].WindowSize)
	}
}

func TestTrendAnalyzer_AnalyzeMovingAverages_InsufficientData(t *testing.T) {
	p := NewPredictor()
	p.AddRecord(CostRecord{Cost: 100})
	ta := NewTrendAnalyzer(p)
	_, err := ta.AnalyzeMovingAverages()
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

// ========== 季节性检测测试 ==========

func TestTrendAnalyzer_DetectSeasonality(t *testing.T) {
	ta := NewTrendAnalyzer(newSeasonalPredictor())
	results, err := ta.DetectSeasonality()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one seasonality result")
	}

	// 应该检测到月度模式
	found := false
	for _, r := range results {
		if r.Pattern == "monthly" {
			found = true
			if r.Confidence < 0 || r.Confidence > 1 {
				t.Errorf("confidence should be 0-1, got %f", r.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected monthly seasonality detection")
	}
}

func TestTrendAnalyzer_DetectSeasonality_InsufficientData(t *testing.T) {
	p := NewPredictor()
	p.AddRecord(CostRecord{Cost: 100})
	ta := NewTrendAnalyzer(p)
	_, err := ta.DetectSeasonality()
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestTrendAnalyzer_DetectSeasonality_SingleMonth(t *testing.T) {
	// 所有数据在同一个月，不应检测到季节性
	p := NewPredictor()
	base := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		p.AddRecord(CostRecord{
			Time: base.AddDate(0, 0, i),
			Cost: float64(100 + i*10),
		})
	}
	ta := NewTrendAnalyzer(p)
	results, err := ta.DetectSeasonality()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 只有1个月的数据，不应该有月度季节性
	for _, r := range results {
		if r.Pattern == "monthly" && len(r.Peaks) > 0 {
			t.Error("should not detect peaks with single month data")
		}
	}
}

// ========== 异常检测测试 ==========

func TestTrendAnalyzer_DetectAnomalies(t *testing.T) {
	ta := NewTrendAnalyzer(newTrendTestPredictor())
	anomalies, err := ta.DetectAnomalies(0.2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 30条数据中应该有部分异常（因含周期性波动）
	for _, a := range anomalies {
		if a.Deviation <= 20 {
			t.Errorf("anomaly deviation should be > 20%%, got %f%%", a.Deviation)
		}
		if a.AlertLevel == "" {
			t.Error("alert level should not be empty")
		}
	}
}

func TestTrendAnalyzer_DetectAnomalies_DefaultThreshold(t *testing.T) {
	ta := NewTrendAnalyzer(newTrendTestPredictor())
	// threshold=0 使用默认值0.2
	anomalies1, err := ta.DetectAnomalies(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	anomalies2, err := ta.DetectAnomalies(0.2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies1) != len(anomalies2) {
		t.Errorf("default threshold should equal 0.2: got %d vs %d", len(anomalies1), len(anomalies2))
	}
}

func TestTrendAnalyzer_DetectAnomalies_InsufficientData(t *testing.T) {
	p := NewPredictor()
	p.AddRecord(CostRecord{Cost: 100})
	p.AddRecord(CostRecord{Cost: 120})
	p.AddRecord(CostRecord{Cost: 110})
	ta := NewTrendAnalyzer(p)
	_, err := ta.DetectAnomalies(0.2)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestTrendAnalyzer_DetectAnomalies_StableData(t *testing.T) {
	// 非常稳定的数据，不应有异常
	p := NewPredictor()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		p.AddRecord(CostRecord{
			Time: base.AddDate(0, 0, i),
			Cost: 1000, // 完全恒定
		})
	}
	ta := NewTrendAnalyzer(p)
	anomalies, err := ta.DetectAnomalies(0.2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 0 {
		t.Errorf("stable data should have no anomalies, got %d", len(anomalies))
	}
}

// ========== 预测精度验证测试 ==========

func TestTrendAnalyzer_ValidatePredictionAccuracy(t *testing.T) {
	ta := NewTrendAnalyzer(newTrendTestPredictor())
	results, err := ta.ValidatePredictionAccuracy(0.7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 accuracy results, got %d", len(results))
	}

	methods := make(map[string]bool)
	for _, r := range results {
		methods[r.Method] = true
		if r.MAE < 0 {
			t.Errorf("MAE should be non-negative for %s", r.Method)
		}
		if r.MAPE < 0 {
			t.Errorf("MAPE should be non-negative for %s", r.Method)
		}
		if r.RMSE < 0 {
			t.Errorf("RMSE should be non-negative for %s", r.Method)
		}
		if r.SampleSize <= 0 {
			t.Errorf("sample size should be positive for %s", r.Method)
		}
	}
	if !methods["linear_regression"] {
		t.Error("expected linear_regression method")
	}
	if !methods["exponential_smoothing"] {
		t.Error("expected exponential_smoothing method")
	}
}

func TestTrendAnalyzer_ValidatePredictionAccuracy_DefaultRatio(t *testing.T) {
	ta := NewTrendAnalyzer(newTrendTestPredictor())
	// ratio=0 使用默认0.7
	r1, err := ta.ValidatePredictionAccuracy(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := ta.ValidatePredictionAccuracy(0.7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r1) != len(r2) {
		t.Error("default ratio should produce same result count")
	}
}

func TestTrendAnalyzer_ValidatePredictionAccuracy_InsufficientData(t *testing.T) {
	p := NewPredictor()
	for i := 0; i < 5; i++ {
		p.AddRecord(CostRecord{Cost: float64(100 + i*10)})
	}
	ta := NewTrendAnalyzer(p)
	_, err := ta.ValidatePredictionAccuracy(0.7)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

// ========== 置信区间测试 ==========

func TestTrendAnalyzer_CalculateConfidenceInterval(t *testing.T) {
	ta := NewTrendAnalyzer(newTrendTestPredictor())
	records := ta.predictor.GetRecords()
	ci := ta.calculateConfidenceInterval(records)

	if ci.Lower95 >= ci.Upper95 {
		t.Error("95% CI: lower should be less than upper")
	}
	if ci.Lower80 >= ci.Upper80 {
		t.Error("80% CI: lower should be less than upper")
	}
	if ci.Lower95 > ci.Lower80 {
		t.Error("95% lower should be <= 80% lower")
	}
	if ci.Upper80 > ci.Upper95 {
		t.Error("80% upper should be <= 95% upper")
	}
	if ci.Lower95 < 0 {
		t.Error("lower bounds should not be negative")
	}
}

// ========== 趋势方向测试 ==========

func TestTrendAnalyzer_CalculateTrendDirection_Rising(t *testing.T) {
	p := NewPredictor()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		p.AddRecord(CostRecord{
			Time: base.AddDate(0, 0, i),
			Cost: float64(100 + i*100), // 明显上升
		})
	}
	ta := NewTrendAnalyzer(p)
	records := ta.predictor.GetRecords()
	direction, strength := ta.calculateTrendDirection(records)
	if direction != TrendRising {
		t.Errorf("expected rising, got %s", direction)
	}
	if strength <= 0 {
		t.Errorf("expected positive strength, got %f", strength)
	}
}

func TestTrendAnalyzer_CalculateTrendDirection_Falling(t *testing.T) {
	p := NewPredictor()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		p.AddRecord(CostRecord{
			Time: base.AddDate(0, 0, i),
			Cost: float64(1000 - i*80), // 明显下降
		})
	}
	ta := NewTrendAnalyzer(p)
	records := ta.predictor.GetRecords()
	direction, strength := ta.calculateTrendDirection(records)
	if direction != TrendFalling {
		t.Errorf("expected falling, got %s", direction)
	}
	if strength >= 0 {
		t.Errorf("expected negative strength, got %f", strength)
	}
}

func TestTrendAnalyzer_CalculateTrendDirection_Stable(t *testing.T) {
	p := NewPredictor()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		p.AddRecord(CostRecord{
			Time: base.AddDate(0, 0, i),
			Cost: 1000, // 恒定
		})
	}
	ta := NewTrendAnalyzer(p)
	records := ta.predictor.GetRecords()
	direction, _ := ta.calculateTrendDirection(records)
	if direction != TrendStable {
		t.Errorf("expected stable, got %s", direction)
	}
}

// ========== 完整趋势报告测试 ==========

func TestTrendAnalyzer_GenerateTrendReport(t *testing.T) {
	ta := NewTrendAnalyzer(newTrendTestPredictor())
	report, err := ta.GenerateTrendReport(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 基本字段验证
	if report.Direction == "" {
		t.Error("direction should not be empty")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("generated time should not be zero")
	}

	// 预测结果
	if len(report.PredictedCosts) == 0 {
		t.Error("expected predicted costs")
	}

	// 置信区间
	if report.ConfidenceInterval.Lower95 >= report.ConfidenceInterval.Upper95 {
		t.Error("invalid confidence interval")
	}

	// 移动平均
	if len(report.MovingAverages) != 2 {
		t.Errorf("expected 2 moving average results, got %d", len(report.MovingAverages))
	}

	// 预测精度
	if len(report.Accuracy) == 0 {
		t.Error("expected accuracy results")
	}
}

func TestTrendAnalyzer_GenerateTrendReport_InsufficientData(t *testing.T) {
	p := NewPredictor()
	p.AddRecord(CostRecord{Cost: 100})
	ta := NewTrendAnalyzer(p)
	_, err := ta.GenerateTrendReport(3)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestTrendAnalyzer_GenerateTrendReport_DefaultPeriods(t *testing.T) {
	ta := NewTrendAnalyzer(newTrendTestPredictor())
	report, err := ta.GenerateTrendReport(0) // 使用默认值3
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.PredictedCosts) == 0 {
		t.Error("expected predictions with default periods")
	}
}

func TestTrendAnalyzer_GenerateTrendReport_SeasonalData(t *testing.T) {
	ta := NewTrendAnalyzer(newSeasonalPredictor())
	report, err := ta.GenerateTrendReport(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 季节性数据应该检测到模式
	if len(report.Seasonality) == 0 {
		t.Error("expected seasonality detection for seasonal data")
	}
}
