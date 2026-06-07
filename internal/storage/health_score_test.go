package storage

import (
	"math"
	"testing"
	"time"
)

func TestNewHealthScoreEngine(t *testing.T) {
	engine := NewHealthScoreEngine()
	if engine == nil {
		t.Fatal("NewHealthScoreEngine 返回 nil")
	}
	if engine.history == nil {
		t.Fatal("history map 未初始化")
	}
	if engine.maxHistorySize != 288 {
		t.Errorf("maxHistorySize = %d, 期望 288", engine.maxHistorySize)
	}
	if engine.failureThreshold != 30.0 {
		t.Errorf("failureThreshold = %f, 期望 30.0", engine.failureThreshold)
	}
}

func TestGetHealthLevel(t *testing.T) {
	engine := NewHealthScoreEngine()

	tests := []struct {
		score    float64
		expected HealthLevel
	}{
		{100, HealthLevelExcellent},
		{95, HealthLevelExcellent},
		{90, HealthLevelExcellent},
		{89, HealthLevelGood},
		{80, HealthLevelGood},
		{70, HealthLevelGood},
		{69, HealthLevelFair},
		{60, HealthLevelFair},
		{50, HealthLevelFair},
		{49, HealthLevelPoor},
		{40, HealthLevelPoor},
		{30, HealthLevelPoor},
		{29, HealthLevelCritical},
		{10, HealthLevelCritical},
		{0, HealthLevelCritical},
	}

	for _, tt := range tests {
		level := engine.GetHealthLevel(tt.score)
		if level != tt.expected {
			t.Errorf("GetHealthLevel(%f) = %s, 期望 %s", tt.score, level, tt.expected)
		}
	}
}

func TestCalculateScore_HealthyDisk(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:       "sda",
		Model:        "TestDisk",
		Serial:       "ABC123",
		Temperature:  35,
		PowerOnHours: 1000,
		SMARTStatus:  SMARTStatusPASSED,
	}

	score := engine.CalculateScore(disk, 50.0, nil)
	if score == nil {
		t.Fatal("CalculateScore 返回 nil")
	}

	// 健康磁盘应该得到高分
	if score.Total < 85 {
		t.Errorf("健康磁盘评分 %f, 期望 >= 85", score.Total)
	}
	if score.Level != HealthLevelExcellent && score.Level != HealthLevelGood {
		t.Errorf("健康磁盘等级 %s, 期望 Excellent 或 Good", score.Level)
	}
	if score.SMARTScore < 90 {
		t.Errorf("SMART评分 %f, 期望 >= 90", score.SMARTScore)
	}
	if score.UsageScore != 100 {
		t.Errorf("使用率评分 %f, 期望 100 (50%%使用率)", score.UsageScore)
	}
	if score.TemperatureScore != 100 {
		t.Errorf("温度评分 %f, 期望 100 (35°C)", score.TemperatureScore)
	}
}

func TestCalculateScore_FailingSMART(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:       "sdb",
		Temperature:  35,
		PowerOnHours: 1000,
		SMARTStatus:  SMARTStatusFAILING,
	}

	score := engine.CalculateScore(disk, 50.0, nil)

	// SMART失败严重拉低总分（SMART维度0分，其他维度满分加权=60分）
	if score.Total >= 65 {
		t.Errorf("SMART失败磁盘评分 %f, 期望 < 65", score.Total)
	}
	if score.SMARTScore != 0 {
		t.Errorf("SMART失败时SMART评分 %f, 期望 0", score.SMARTScore)
	}
	if score.Level == HealthLevelExcellent || score.Level == HealthLevelGood {
		t.Errorf("SMART失败磁盘等级 %s, 期望不应为 Excellent 或 Good", score.Level)
	}
}

func TestCalculateScore_ReallocatedSectors(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:             "sdc",
		Temperature:        35,
		PowerOnHours:       1000,
		SMARTStatus:        SMARTStatusPASSED,
		ReallocatedSectors: 25,
	}

	score := engine.CalculateScore(disk, 50.0, nil)

	// 25个重分配扇区应该扣50分 (25*2=50)
	if score.SMARTScore > 55 {
		t.Errorf("25个重分配扇区SMART评分 %f, 期望 <= 55", score.SMARTScore)
	}
}

func TestCalculateScore_HighUsage(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:       "sdd",
		Temperature:  35,
		PowerOnHours: 1000,
		SMARTStatus:  SMARTStatusPASSED,
	}

	// 测试不同使用率
	tests := []struct {
		usage    float64
		maxScore float64
	}{
		{50.0, 100},
		{75.0, 100},
		{85.0, 70},
		{92.0, 45},
		{98.0, 15},
	}

	for _, tt := range tests {
		score := engine.CalculateScore(disk, tt.usage, nil)
		if score.UsageScore > tt.maxScore {
			t.Errorf("使用率 %.1f%%: 使用率评分 %f, 期望 <= %f", tt.usage, score.UsageScore, tt.maxScore)
		}
	}
}

func TestCalculateScore_HighTemperature(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:       "sde",
		PowerOnHours: 1000,
		SMARTStatus:  SMARTStatusPASSED,
	}

	tests := []struct {
		temp     int
		maxScore float64
	}{
		{30, 100},
		{45, 100},
		{50, 75},
		{55, 50},
		{60, 30},
		{70, 0},
	}

	for _, tt := range tests {
		disk.Temperature = tt.temp
		score := engine.CalculateScore(disk, 50.0, nil)
		if score.TemperatureScore > tt.maxScore+1 { // 允许1分误差
			t.Errorf("温度 %d°C: 温度评分 %f, 期望 <= %f", tt.temp, score.TemperatureScore, tt.maxScore)
		}
	}
}

func TestCalculateScore_OldDisk(t *testing.T) {
	engine := NewHealthScoreEngine()

	// 出厂8年前
	oldDate := time.Now().Add(-8 * 365 * 24 * time.Hour)

	disk := &DiskHealth{
		Device:       "sdf",
		Temperature:  35,
		PowerOnHours: 40000,
		SMARTStatus:  SMARTStatusPASSED,
	}

	score := engine.CalculateScore(disk, 50.0, &oldDate)
	if score.AgeScore > 50 {
		t.Errorf("8年老磁盘年龄评分 %f, 期望 <= 50", score.AgeScore)
	}
}

func TestCalculateScore_NilDisk(t *testing.T) {
	engine := NewHealthScoreEngine()

	score := engine.CalculateScore(nil, 0, nil)
	if score == nil {
		t.Fatal("nil磁盘应返回非nil评分")
	}
	if score.Total != 0 {
		t.Errorf("nil磁盘总分 %f, 期望 0", score.Total)
	}
	if score.Level != HealthLevelCritical {
		t.Errorf("nil磁盘等级 %s, 期望 Critical", score.Level)
	}
}

func TestCalculateScore_IOErrors(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:         "sdg",
		Temperature:    35,
		PowerOnHours:   1000,
		SMARTStatus:    SMARTStatusPASSED,
		SeekErrorRate:  3.0,
		ReadErrorRate:  2.0,
		WriteErrorRate: 1.0,
	}

	score := engine.CalculateScore(disk, 50.0, nil)
	// 总错误率6.0，I/O评分应该较低
	if score.IOErrorScore > 35 {
		t.Errorf("I/O错误率6.0时I/O评分 %f, 期望 <= 35", score.IOErrorScore)
	}
}

func TestCalculateScore_Deductions(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:             "sdh",
		Temperature:        60,
		PowerOnHours:       1000,
		SMARTStatus:        SMARTStatusWARNING,
		ReallocatedSectors: 5,
		PendingSectors:     3,
	}

	score := engine.CalculateScore(disk, 92.0, nil)

	// 应该有多个扣分原因
	if len(score.Deductions) == 0 {
		t.Error("预期存在扣分原因列表")
	}

	// 检查是否包含关键扣分项
	foundSMART := false
	foundUsage := false
	foundTemp := false
	for _, d := range score.Deductions {
		if contains(d, "SMART") {
			foundSMART = true
		}
		if contains(d, "使用率") {
			foundUsage = true
		}
		if contains(d, "温度") || contains(d, "°C") {
			foundTemp = true
		}
	}
	if !foundSMART {
		t.Error("扣分原因中未找到SMART相关项")
	}
	if !foundUsage {
		t.Error("扣分原因中未找到使用率相关项")
	}
	if !foundTemp {
		t.Error("扣分原因中未找到温度相关项")
	}
}

func TestCalculateScore_Alerts(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:       "sdi",
		Temperature:  66,
		PowerOnHours: 1000,
		SMARTStatus:  SMARTStatusPASSED,
	}

	score := engine.CalculateScore(disk, 96.0, nil)

	// 应该有高温和高使用率告警
	if len(score.Alerts) == 0 {
		t.Error("预期存在告警列表")
	}
}

func TestPredictFailure_InsufficientData(t *testing.T) {
	engine := NewHealthScoreEngine()

	trend := engine.PredictFailure("nonexistent")
	if trend.FailureProbability != 0 {
		t.Errorf("数据不足时故障概率 %f, 期望 0", trend.FailureProbability)
	}
	if trend.Confidence != 0 {
		t.Errorf("数据不足时置信度 %f, 期望 0", trend.Confidence)
	}
}

func TestPredictFailure_DecliningTrend(t *testing.T) {
	engine := NewHealthScoreEngine()

	// 模拟持续下降的评分序列
	baseTime := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 24; i++ {
		point := TrendPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Score:     100 - float64(i)*3, // 每小时下降3分
		}
		engine.history["test_device"] = append(engine.history["test_device"], point)
	}

	trend := engine.PredictFailure("test_device")

	if trend.TrendRate >= 0 {
		t.Errorf("下降趋势的TrendRate %f, 期望 < 0", trend.TrendRate)
	}
	if trend.FailureProbability <= 0 {
		t.Errorf("下降趋势的故障概率 %f, 期望 > 0", trend.FailureProbability)
	}
	if trend.Confidence <= 0 {
		t.Errorf("24个数据点的置信度 %f, 期望 > 0", trend.Confidence)
	}
	if trend.EstimatedFailureTime == nil {
		t.Error("下降趋势应预测故障时间")
	}
}

func TestPredictFailure_StableTrend(t *testing.T) {
	engine := NewHealthScoreEngine()

	// 模拟稳定的评分序列
	baseTime := time.Now().Add(-12 * time.Hour)
	for i := 0; i < 12; i++ {
		point := TrendPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Score:     90 + float64(i%3)*0.5, // 在90-91之间微小波动
		}
		engine.history["stable_device"] = append(engine.history["stable_device"], point)
	}

	trend := engine.PredictFailure("stable_device")

	if trend.FailureProbability > 0.3 {
		t.Errorf("稳定趋势的故障概率 %f, 期望 <= 0.3", trend.FailureProbability)
	}
}

func TestGenerateReport(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:       "sda",
		Model:        "WD Red 4TB",
		Serial:       "WD123456",
		Size:         4000000000000,
		Temperature:  40,
		PowerOnHours: 5000,
		SMARTStatus:  SMARTStatusPASSED,
	}

	report := engine.GenerateReport(disk, 60.0, nil, 0.1)

	if report == nil {
		t.Fatal("GenerateReport 返回 nil")
	}
	if report.Device != "sda" {
		t.Errorf("Device = %s, 期望 sda", report.Device)
	}
	if report.Model != "WD Red 4TB" {
		t.Errorf("Model = %s, 期望 WD Red 4TB", report.Model)
	}
	if report.DiskUsagePercent != 60.0 {
		t.Errorf("DiskUsagePercent = %f, 期望 60.0", report.DiskUsagePercent)
	}
	if report.Score.Total < 85 {
		t.Errorf("健康磁盘报告总分 %f, 期望 >= 85", report.Score.Total)
	}
	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt 不应为零值")
	}
	if report.Trend == nil {
		t.Error("Trend 不应为 nil")
	}
}

func TestGenerateReport_NilDisk(t *testing.T) {
	engine := NewHealthScoreEngine()

	report := engine.GenerateReport(nil, 0, nil, 0)
	if report == nil {
		t.Fatal("nil磁盘应返回非nil报告")
	}
	if !report.GeneratedAt.IsZero() && report.Device != "" {
		t.Error("nil磁盘报告应为空设备")
	}
}

func TestGenerateReport_WithManufactureDate(t *testing.T) {
	engine := NewHealthScoreEngine()

	mfgDate := time.Now().Add(-4 * 365 * 24 * time.Hour)
	disk := &DiskHealth{
		Device:       "sdb",
		Model:        "Seagate IronWolf",
		PowerOnHours: 30000,
		Temperature:  38,
		SMARTStatus:  SMARTStatusPASSED,
	}

	report := engine.GenerateReport(disk, 70.0, &mfgDate, 0)

	if report.DiskAgeYears < 3.5 || report.DiskAgeYears > 4.5 {
		t.Errorf("使用年限 %.2f, 期望约4年", report.DiskAgeYears)
	}
}

func TestGenerateReport_Recommendations(t *testing.T) {
	engine := NewHealthScoreEngine()

	// 故障磁盘应该有维护建议
	disk := &DiskHealth{
		Device:             "sdc",
		Temperature:        62,
		PowerOnHours:       60000,
		SMARTStatus:        SMARTStatusWARNING,
		ReallocatedSectors: 50,
		PendingSectors:     20,
	}

	report := engine.GenerateReport(disk, 95.0, nil, 8.0)

	if len(report.Recommendations) == 0 {
		t.Error("故障磁盘应有维护建议")
	}
}

func TestHistoryManagement(t *testing.T) {
	engine := NewHealthScoreEngine()

	// 记录历史
	for i := 0; i < 5; i++ {
		engine.recordHistory("test", float64(90-i))
	}

	history := engine.GetHistory("test")
	if len(history) != 5 {
		t.Errorf("历史记录数 %d, 期望 5", len(history))
	}

	// 验证返回的是副本
	history[0].Score = 999
	original := engine.GetHistory("test")
	if original[0].Score == 999 {
		t.Error("GetHistory 应返回副本")
	}

	// 清除历史
	engine.ClearHistory("test")
	history = engine.GetHistory("test")
	if history != nil {
		t.Errorf("清除后历史记录应为 nil, 实际 %d 条", len(history))
	}
}

func TestHistoryMaxSize(t *testing.T) {
	engine := NewHealthScoreEngine()
	engine.maxHistorySize = 10

	for i := 0; i < 20; i++ {
		engine.recordHistory("test", float64(i))
	}

	history := engine.GetHistory("test")
	if len(history) > 10 {
		t.Errorf("历史记录数 %d, 期望 <= 10", len(history))
	}
	// 最新的记录应该是后面的
	if history[len(history)-1].Score != 19 {
		t.Errorf("最后一条记录分数 %f, 期望 19", history[len(history)-1].Score)
	}
}

func TestSetFailureThreshold(t *testing.T) {
	engine := NewHealthScoreEngine()

	engine.SetFailureThreshold(25.0)
	if engine.failureThreshold != 25.0 {
		t.Errorf("failureThreshold = %f, 期望 25.0", engine.failureThreshold)
	}

	// 测试边界值
	engine.SetFailureThreshold(-10)
	if engine.failureThreshold != 0 {
		t.Errorf("负值应被限制为0, 实际 %f", engine.failureThreshold)
	}

	engine.SetFailureThreshold(200)
	if engine.failureThreshold != 100 {
		t.Errorf("超过100应被限制为100, 实际 %f", engine.failureThreshold)
	}
}

func TestClampScore(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{150, 100},
		{100, 100},
		{50, 50},
		{0, 0},
		{-10, 0},
	}

	for _, tt := range tests {
		result := clampScore(tt.input)
		if result != tt.expected {
			t.Errorf("clampScore(%f) = %f, 期望 %f", tt.input, result, tt.expected)
		}
	}
}

func TestScoreRange(t *testing.T) {
	engine := NewHealthScoreEngine()

	// 极端情况：所有指标最差
	disk := &DiskHealth{
		Temperature:          80,
		PowerOnHours:         100000,
		SMARTStatus:          SMARTStatusFAILING,
		ReallocatedSectors:   1000,
		PendingSectors:       500,
		OfflineUncorrectable: 200,
		UDMACRCErrorCount:    100,
		SeekErrorRate:        20,
		ReadErrorRate:        20,
		WriteErrorRate:       20,
	}

	score := engine.CalculateScore(disk, 99.0, nil)

	// 总分必须在0-100范围内
	if score.Total < 0 || score.Total > 100 {
		t.Errorf("极端情况总分 %f, 必须在0-100范围内", score.Total)
	}
	if score.SMARTScore < 0 || score.SMARTScore > 100 {
		t.Errorf("极端情况SMART评分 %f, 必须在0-100范围内", score.SMARTScore)
	}
	if score.UsageScore < 0 || score.UsageScore > 100 {
		t.Errorf("极端情况使用率评分 %f, 必须在0-100范围内", score.UsageScore)
	}
	if score.AgeScore < 0 || score.AgeScore > 100 {
		t.Errorf("极端情况年龄评分 %f, 必须在0-100范围内", score.AgeScore)
	}
	if score.IOErrorScore < 0 || score.IOErrorScore > 100 {
		t.Errorf("极端情况I/O评分 %f, 必须在0-100范围内", score.IOErrorScore)
	}
	if score.TemperatureScore < 0 || score.TemperatureScore > 100 {
		t.Errorf("极端情况温度评分 %f, 必须在0-100范围内", score.TemperatureScore)
	}
}

func TestWeightSum(t *testing.T) {
	// 验证权重总和为1.0
	totalWeight := WeightSMART + WeightUsage + WeightAge + WeightIOError + WeightTemperature
	if math.Abs(totalWeight-1.0) > 0.001 {
		t.Errorf("权重总和 %f, 期望 1.0", totalWeight)
	}
}

func TestCalculateScore_NVMEDisk(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:             "nvme0n1",
		Temperature:        40,
		PowerOnHours:       5000,
		SMARTStatus:        SMARTStatusPASSED,
		NVMeAvailableSpare: 90,
		NVMePercentageUsed: 10,
	}

	score := engine.CalculateScore(disk, 50.0, nil)

	if score.Total < 85 {
		t.Errorf("健康NVMe磁盘评分 %f, 期望 >= 85", score.Total)
	}
}

func TestCalculateScore_NVMELowSpare(t *testing.T) {
	engine := NewHealthScoreEngine()

	disk := &DiskHealth{
		Device:             "nvme0n1",
		Temperature:        40,
		PowerOnHours:       5000,
		SMARTStatus:        SMARTStatusPASSED,
		NVMeAvailableSpare: 20,
		NVMePercentageUsed: 80,
	}

	score := engine.CalculateScore(disk, 50.0, nil)

	// NVMe备用空间低应该扣分
	if score.SMARTScore > 90 {
		t.Errorf("NVMe备用空间低时SMART评分 %f, 期望 < 90", score.SMARTScore)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
