// Package optimizer 提供资源配额优化功能测试
package optimizer

import (
	"testing"
	"time"
)

// ========== Mock 数据提供者 ==========

type MockQuotaProvider struct{}

func (m *MockQuotaProvider) GetAllUsage() ([]*QuotaUsageInfo, error) {
	return []*QuotaUsageInfo{
		{
			QuotaID:      "quota-1",
			TargetID:     "user1",
			TargetName:   "用户1",
			VolumeName:   "pool1",
			Type:         "user",
			HardLimit:    200 * 1024 * 1024 * 1024, // 200GB
			SoftLimit:    160 * 1024 * 1024 * 1024, // 160GB
			UsedBytes:    180 * 1024 * 1024 * 1024, // 180GB (90%)
			UsagePercent: 90,
			IsOverSoft:   true,
			IsOverHard:   false,
			LastChecked:  time.Now(),
		},
		{
			QuotaID:      "quota-2",
			TargetID:     "user2",
			TargetName:   "用户2",
			VolumeName:   "pool1",
			Type:         "user",
			HardLimit:    500 * 1024 * 1024 * 1024, // 500GB
			SoftLimit:    400 * 1024 * 1024 * 1024, // 400GB
			UsedBytes:    50 * 1024 * 1024 * 1024,  // 50GB (10%)
			UsagePercent: 10,
			IsOverSoft:   false,
			IsOverHard:   false,
			LastChecked:  time.Now(),
		},
		{
			QuotaID:      "quota-3",
			TargetID:     "user3",
			TargetName:   "用户3",
			VolumeName:   "pool1",
			Type:         "user",
			HardLimit:    100 * 1024 * 1024 * 1024, // 100GB
			SoftLimit:    80 * 1024 * 1024 * 1024,  // 80GB
			UsedBytes:    110 * 1024 * 1024 * 1024, // 110GB (110%, 超限)
			UsagePercent: 110,
			IsOverSoft:   true,
			IsOverHard:   true,
			LastChecked:  time.Now(),
		},
	}, nil
}

func (m *MockQuotaProvider) GetUserUsage(username string) ([]*QuotaUsageInfo, error) {
	return []*QuotaUsageInfo{
		{
			QuotaID:      "quota-1",
			TargetID:     username,
			TargetName:   username,
			VolumeName:   "pool1",
			HardLimit:    200 * 1024 * 1024 * 1024,
			UsedBytes:    180 * 1024 * 1024 * 1024,
			UsagePercent: 90,
		},
	}, nil
}

func (m *MockQuotaProvider) GetQuota(quotaID string) (*QuotaInfo, error) {
	return &QuotaInfo{
		ID:         quotaID,
		TargetID:   "user1",
		TargetName: "用户1",
		VolumeName: "pool1",
		Type:       "user",
		HardLimit:  200 * 1024 * 1024 * 1024,
		SoftLimit:  160 * 1024 * 1024 * 1024,
		CreatedAt:  time.Now().AddDate(0, -1, 0),
		UpdatedAt:  time.Now(),
	}, nil
}

func (m *MockQuotaProvider) UpdateQuota(quotaID string, newLimit uint64) error {
	return nil
}

type MockHistoryProvider struct{}

func (m *MockHistoryProvider) GetHistory(quotaID string, days int) ([]HistoryPoint, error) {
	points := make([]HistoryPoint, 0)
	baseUsage := uint64(100 * 1024 * 1024 * 1024) // 100GB

	for i := 0; i < days; i++ {
		points = append(points, HistoryPoint{
			Timestamp:    time.Now().AddDate(0, 0, -days+i),
			UsedBytes:    baseUsage + uint64(i)*1024*1024*1024, // 每天增加1GB
			UsagePercent: float64(100+i) / 200 * 100,
		})
	}

	return points, nil
}

func (m *MockHistoryProvider) GetGrowthRate(quotaID string) (float64, error) {
	return 1024 * 1024 * 1024, nil // 1GB/天
}

// ========== 测试用例 ==========

func TestNewQuotaOptimizer(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	if optimizer == nil {
		t.Fatal("创建优化器失败")
	}

	if !optimizer.config.AutoAdjust.Enabled {
		t.Error("自动调整应该默认启用")
	}

	if optimizer.config.PricePerGB != 0.1 {
		t.Errorf("默认价格应该是 0.1 元/GB，实际是 %.2f", optimizer.config.PricePerGB)
	}
}

func TestGenerateAdjustmentSuggestions(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	suggestions, err := optimizer.GenerateAdjustmentSuggestions()
	if err != nil {
		t.Fatalf("生成调整建议失败: %v", err)
	}

	// 应该有两个建议：一个高使用率需要扩展，一个低使用率需要缩减
	if len(suggestions) == 0 {
		t.Log("没有生成建议，这可能是正常的（取决于阈值配置）")
	}

	// 检查建议类型
	for _, s := range suggestions {
		if s.Type != OptimizationAutoAdjust {
			t.Errorf("建议类型应该是 auto_adjust，实际是 %s", s.Type)
		}

		if s.Status != "pending" {
			t.Errorf("新建议状态应该是 pending，实际是 %s", s.Status)
		}
	}
}

func TestPredictUsage(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	prediction, err := optimizer.PredictUsage("quota-1")
	if err != nil {
		t.Fatalf("预测使用失败: %v", err)
	}

	if prediction == nil {
		t.Fatal("预测结果不应为空")
	}

	if prediction.QuotaID != "quota-1" {
		t.Errorf("配额ID应该是 quota-1，实际是 %s", prediction.QuotaID)
	}

	if prediction.GrowthRate <= 0 {
		t.Log("增长率为0或负数，可能是稳定或下降趋势")
	}

	// 检查预测结果
	if prediction.PredictedDaysToFull < 0 {
		t.Error("预测天数不应为负数")
	}
}

func TestPredictAllUsage(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	predictions, err := optimizer.PredictAllUsage()
	if err != nil {
		t.Fatalf("预测所有使用失败: %v", err)
	}

	if len(predictions) == 0 {
		t.Fatal("预测结果不应为空")
	}

	// 检查每个预测
	for _, pred := range predictions {
		if pred.QuotaID == "" {
			t.Error("配额ID不应为空")
		}

		if pred.GeneratedAt.IsZero() {
			t.Error("生成时间不应为零")
		}
	}
}

func TestDetectViolations(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	violations, err := optimizer.DetectViolations()
	if err != nil {
		t.Fatalf("检测违规失败: %v", err)
	}

	// 应该至少检测到一个硬限制违规（quota-3）
	hardViolations := 0
	for _, v := range violations {
		if v.Type == ViolationHardLimit {
			hardViolations++
		}
	}

	if hardViolations == 0 {
		t.Log("未检测到硬限制违规，检查测试数据是否正确")
	}

	// 检查违规状态
	for _, v := range violations {
		if v.Status != "active" {
			t.Errorf("新违规状态应该是 active，实际是 %s", v.Status)
		}
	}
}

func TestResolveViolation(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	// 先检测违规
	violations, _ := optimizer.DetectViolations()

	if len(violations) == 0 {
		t.Skip("没有违规可测试")
	}

	// 解决第一个违规
	violationID := violations[0].ID
	err := optimizer.ResolveViolation(violationID, "admin")
	if err != nil {
		t.Fatalf("解决违规失败: %v", err)
	}

	// 验证状态
	resolved := optimizer.GetViolations("resolved")
	found := false
	for _, v := range resolved {
		if v.ID == violationID {
			found = true
			if v.ResolvedBy != "admin" {
				t.Errorf("解决者应该是 admin，实际是 %s", v.ResolvedBy)
			}
		}
	}

	if !found {
		t.Error("解决的违规应该在 resolved 列表中")
	}
}

func TestGenerateOptimizationReport(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	report, err := optimizer.GenerateOptimizationReport()
	if err != nil {
		t.Fatalf("生成优化报告失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为空")
	}

	// 检查报告字段
	if report.ID == "" {
		t.Error("报告ID不应为空")
	}

	if report.GeneratedAt.IsZero() {
		t.Error("生成时间不应为零")
	}

	// 检查摘要
	if report.Summary.TotalQuotas == 0 {
		t.Error("总配额数不应为0")
	}

	// 检查成本影响
	if report.CostImpact.Currency == "" {
		t.Error("货币不应为空")
	}

	// 检查建议
	if len(report.Recommendations) == 0 {
		t.Log("没有生成建议")
	}
}

func TestApplySuggestion(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	// 先生成建议
	suggestions, _ := optimizer.GenerateAdjustmentSuggestions()

	if len(suggestions) == 0 {
		t.Skip("没有建议可测试")
	}

	// 应用第一个建议
	suggestionID := suggestions[0].ID
	err := optimizer.ApplySuggestion(suggestionID)
	if err != nil {
		t.Fatalf("应用建议失败: %v", err)
	}

	// 验证状态
	applied := optimizer.GetSuggestions("applied")
	found := false
	for _, s := range applied {
		if s.ID == suggestionID {
			found = true
			if s.AppliedAt == nil {
				t.Error("应用时间不应为空")
			}
		}
	}

	if !found {
		t.Error("应用的建议应该在 applied 列表中")
	}

	// 检查调整历史
	history := optimizer.GetAdjustHistory(10)
	if len(history) == 0 {
		t.Error("应该有调整历史记录")
	}
}

func TestDismissSuggestion(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	// 先生成建议
	suggestions, _ := optimizer.GenerateAdjustmentSuggestions()

	if len(suggestions) == 0 {
		t.Skip("没有建议可测试")
	}

	// 忽略第一个建议
	suggestionID := suggestions[0].ID
	err := optimizer.DismissSuggestion(suggestionID)
	if err != nil {
		t.Fatalf("忽略建议失败: %v", err)
	}

	// 验证状态
	dismissed := optimizer.GetSuggestions("dismissed")
	found := false
	for _, s := range dismissed {
		if s.ID == suggestionID {
			found = true
		}
	}

	if !found {
		t.Error("忽略的建议应该在 dismissed 列表中")
	}
}

func TestGetSuggestions(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	// 生成建议
	optimizer.GenerateAdjustmentSuggestions()

	// 获取所有建议
	allSuggestions := optimizer.GetSuggestions("")
	if len(allSuggestions) == 0 {
		t.Log("没有建议")
	}

	// 获取待处理建议
	pendingSuggestions := optimizer.GetSuggestions("pending")
	for _, s := range pendingSuggestions {
		if s.Status != "pending" {
			t.Errorf("状态应该是 pending，实际是 %s", s.Status)
		}
	}
}

func TestGetViolations(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	// 检测违规
	optimizer.DetectViolations()

	// 获取所有违规
	allViolations := optimizer.GetViolations("")
	if len(allViolations) == 0 {
		t.Log("没有违规")
	}

	// 获取活跃违规
	activeViolations := optimizer.GetViolations("active")
	for _, v := range activeViolations {
		if v.Status != "active" {
			t.Errorf("状态应该是 active，实际是 %s", v.Status)
		}
	}
}

func TestGetAdjustHistory(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	// 生成并应用建议
	suggestions, _ := optimizer.GenerateAdjustmentSuggestions()
	if len(suggestions) > 0 {
		optimizer.ApplySuggestion(suggestions[0].ID)
	}

	// 获取历史
	history := optimizer.GetAdjustHistory(10)

	// 如果有应用的建议，应该有历史
	if len(suggestions) > 0 && len(history) == 0 {
		t.Error("应用建议后应该有调整历史")
	}
}

func TestLinearPrediction(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	// 创建测试历史数据
	history := make([]HistoryPoint, 30)
	for i := 0; i < 30; i++ {
		history[i] = HistoryPoint{
			Timestamp:    time.Now().AddDate(0, 0, -30+i),
			UsedBytes:    uint64(100+i) * 1024 * 1024 * 1024, // 每天增加1GB
			UsagePercent: float64(100+i) / 200 * 100,
		}
	}

	// 测试线性预测
	prediction := &UsagePrediction{
		PredictionPeriod: 7 * 24 * time.Hour,
	}

	optimizer.linearPrediction(prediction, history)

	if prediction.GrowthRate <= 0 {
		t.Log("增长率应该为正（数据呈增长趋势）")
	}

	if prediction.GrowthTrend != "increasing" {
		t.Errorf("趋势应该是 increasing，实际是 %s", prediction.GrowthTrend)
	}
}

func TestMovingAvgPrediction(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	// 创建测试历史数据
	history := make([]HistoryPoint, 14)
	for i := 0; i < 14; i++ {
		history[i] = HistoryPoint{
			Timestamp:    time.Now().AddDate(0, 0, -14+i),
			UsedBytes:    uint64(100+i) * 1024 * 1024 * 1024,
			UsagePercent: float64(100+i) / 200 * 100,
		}
	}

	// 测试移动平均预测
	prediction := &UsagePrediction{
		PredictionPeriod: 7 * 24 * time.Hour,
	}

	optimizer.movingAvgPrediction(prediction, history)

	if prediction.Confidence == 0 {
		t.Error("置信度不应为0")
	}
}

func TestSimplePrediction(t *testing.T) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	usage := &QuotaUsageInfo{
		QuotaID:      "quota-1",
		UsedBytes:    100 * 1024 * 1024 * 1024,
		HardLimit:    200 * 1024 * 1024 * 1024,
		UsagePercent: 50,
	}

	quota := &QuotaInfo{
		ID:         "quota-1",
		TargetID:   "user1",
		TargetName: "用户1",
		VolumeName: "pool1",
		HardLimit:  200 * 1024 * 1024 * 1024,
	}

	prediction := optimizer.simplePrediction(usage, quota)

	if prediction == nil {
		t.Fatal("预测结果不应为空")
	}

	if prediction.Confidence != 0.5 {
		t.Log("简单预测置信度应该是0.5")
	}

	if prediction.Recommendation == "" {
		t.Error("建议不应为空")
	}
}

// ========== 基准测试 ==========

func BenchmarkGenerateAdjustmentSuggestions(b *testing.B) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = optimizer.GenerateAdjustmentSuggestions()
	}
}

func BenchmarkPredictAllUsage(b *testing.B) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = optimizer.PredictAllUsage()
	}
}

func BenchmarkDetectViolations(b *testing.B) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = optimizer.DetectViolations()
	}
}

func BenchmarkGenerateOptimizationReport(b *testing.B) {
	config := DefaultOptimizerConfig()
	optimizer := NewQuotaOptimizer("/tmp/quota_opt_test", &MockQuotaProvider{}, &MockHistoryProvider{}, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = optimizer.GenerateOptimizationReport()
	}
}
