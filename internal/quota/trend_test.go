package quota

import (
	"testing"
	"time"
)

// ========== TrendConfig 测试 ==========

func TestDefaultTrendConfig(t *testing.T) {
	config := DefaultTrendConfig()

	if config.CollectInterval != 5*time.Minute {
		t.Errorf("期望 CollectInterval 为 5m, 实际为 %v", config.CollectInterval)
	}

	if config.MaxDataPoints != 2016 {
		t.Errorf("期望 MaxDataPoints 为 2016, 实际为 %d", config.MaxDataPoints)
	}

	if !config.PredictionEnabled {
		t.Error("PredictionEnabled 应该为 true")
	}

	if len(config.AggregationLevels) != 4 {
		t.Errorf("期望 4 个聚合级别, 实际为 %d", len(config.AggregationLevels))
	}
}

// ========== TrendDataManager 测试 ==========

func TestNewTrendDataManager(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	if trendMgr == nil {
		t.Fatal("TrendDataManager 不应为 nil")
	}

	if trendMgr.config.CollectInterval != config.CollectInterval {
		t.Error("配置应该被正确设置")
	}
}

func TestTrendDataManager_RecordDataPoint(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录数据点
	point := TrendDataPointExtended{
		Timestamp:    time.Now(),
		UsedBytes:    50 * 1024 * 1024 * 1024,
		UsagePercent: 50.0,
	}

	trendMgr.RecordDataPoint("test-quota", point)

	// 验证数据被记录
	data := trendMgr.GetRawData("test-quota", 24*time.Hour)
	if len(data) != 1 {
		t.Errorf("期望 1 个数据点, 实际为 %d", len(data))
	}
}

func TestTrendDataManager_MultipleDataPoints(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录多个数据点
	now := time.Now()
	for i := 0; i < 10; i++ {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * time.Hour),
			UsedBytes:    uint64(50+i*5) * 1024 * 1024 * 1024,
			UsagePercent: float64(50 + i*5),
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	// 验证数据被记录
	data := trendMgr.GetRawData("test-quota", 24*time.Hour)
	if len(data) != 10 {
		t.Errorf("期望 10 个数据点, 实际为 %d", len(data))
	}
}

func TestTrendDataManager_GetTrendStats(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录数据点
	now := time.Now()
	for i := 0; i < 5; i++ {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * time.Hour),
			UsedBytes:    uint64(60+i*2) * 1024 * 1024 * 1024,
			UsagePercent: float64(60 + i*2),
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	// 获取统计
	stats := trendMgr.GetTrendStats("test-quota", 24*time.Hour)

	if stats == nil {
		t.Fatal("统计不应为 nil")
	}

	if stats.MinUsagePercent != 60.0 {
		t.Errorf("期望最小使用率 60.0, 实际为 %.2f", stats.MinUsagePercent)
	}

	if stats.MaxUsagePercent != 68.0 {
		t.Errorf("期望最大使用率 68.0, 实际为 %.2f", stats.MaxUsagePercent)
	}
}

func TestTrendDataManager_Predict(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	config.MinDataPointsForPrediction = 5 // 降低最小数据点要求
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录足够的数据点
	now := time.Now()
	for i := 0; i < 10; i++ {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * 24 * time.Hour),
			UsedBytes:    uint64(50+i*2) * 1024 * 1024 * 1024, // 每天增加 2GB
			UsagePercent: float64(50 + i*2),
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	// 进行预测 - Predict 方法签名是 Predict(quotaID string, days int)
	prediction := trendMgr.Predict("test-quota", 7)

	if prediction == nil {
		t.Fatal("预测不应为 nil")
	}

	if prediction.Method == "" {
		t.Error("预测方法不应为空")
	}
}

func TestTrendDataManager_GenerateReport(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录数据点
	now := time.Now()
	for i := 0; i < 10; i++ {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * time.Hour),
			UsedBytes:    uint64(60+i) * 1024 * 1024 * 1024,
			UsagePercent: float64(60 + i),
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	// 生成报告
	report := trendMgr.GenerateReport("test-quota", 24*time.Hour)

	if report == nil {
		t.Fatal("报告不应为 nil")
	}

	if report.QuotaID != "test-quota" {
		t.Errorf("期望 QuotaID 为 test-quota, 实际为 %s", report.QuotaID)
	}

	if report.DataPointCount != 10 {
		t.Errorf("期望 10 个数据点, 实际为 %d", report.DataPointCount)
	}
}

func TestTrendDataManager_MaxDataPoints(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	config.MaxDataPoints = 5 // 设置最大数据点为 5
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录超过最大数量的数据点
	now := time.Now()
	for i := 0; i < 10; i++ {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * time.Minute),
			UsedBytes:    uint64(i) * 1024 * 1024 * 1024,
			UsagePercent: float64(i),
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	// 验证只保留最新的 5 个
	data := trendMgr.GetRawData("test-quota", 24*time.Hour)
	if len(data) > 5 {
		t.Errorf("期望最多 5 个数据点, 实际为 %d", len(data))
	}
}

// ========== TrendStatistics 测试 ==========

func TestTrendStatistics_Calculation(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录具有波动性的数据点
	now := time.Now()
	values := []float64{60, 65, 62, 68, 70, 72, 68, 75, 78, 80}
	for i, v := range values {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * time.Hour),
			UsedBytes:    uint64(v) * 1024 * 1024 * 1024,
			UsagePercent: v,
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	stats := trendMgr.GetTrendStats("test-quota", 24*time.Hour)

	if stats.MinUsagePercent != 60.0 {
		t.Errorf("期望最小使用率 60.0, 实际为 %.2f", stats.MinUsagePercent)
	}

	if stats.MaxUsagePercent != 80.0 {
		t.Errorf("期望最大使用率 80.0, 实际为 %.2f", stats.MaxUsagePercent)
	}

	// 平均值应该在 60-80 之间
	if stats.AvgUsagePercent < 60 || stats.AvgUsagePercent > 80 {
		t.Errorf("平均使用率应该在 60-80 之间, 实际为 %.2f", stats.AvgUsagePercent)
	}
}

// ========== AggregatedTrendPoint 测试 ==========

func TestTrendDataManager_AggregatedData(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录多个数据点（每5分钟一个，记录1小时）
	now := time.Now()
	for i := 0; i < 12; i++ {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * 5 * time.Minute),
			UsedBytes:    uint64(50+i) * 1024 * 1024 * 1024,
			UsagePercent: float64(50 + i),
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	// 获取聚合数据
	aggData := trendMgr.GetAggregatedData("test-quota", "hourly", 24*time.Hour)
	// 由于数据点不足1小时，可能没有聚合数据
	t.Logf("聚合数据点数: %d", len(aggData))
}

// ========== TrendPrediction 测试 ==========

func TestTrendPrediction_Confidence(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	config.MinDataPointsForPrediction = 5
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录稳定增长的数据
	now := time.Now()
	for i := 0; i < 20; i++ {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * 24 * time.Hour),
			UsedBytes:    uint64(50+i*2) * 1024 * 1024 * 1024,
			UsagePercent: float64(50 + i*2),
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	prediction := trendMgr.Predict("test-quota", 14)

	if prediction == nil {
		t.Fatal("预测不应为 nil")
	}

	// 验证置信度在合理范围
	if prediction.Confidence < 0 || prediction.Confidence > 1 {
		t.Errorf("置信度应该在 0-1 之间, 实际为 %.2f", prediction.Confidence)
	}

	// 验证预测点
	if len(prediction.PredictionPoints) == 0 {
		t.Error("应该有预测点")
	}
}

// ========== TrendAnalysisReport 测试 ==========

func TestTrendAnalysisReport_Structure(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录数据
	now := time.Now()
	for i := 0; i < 15; i++ {
		point := TrendDataPointExtended{
			Timestamp:    now.Add(time.Duration(i) * time.Hour),
			UsedBytes:    uint64(55+i) * 1024 * 1024 * 1024,
			UsagePercent: float64(55 + i),
		}
		trendMgr.RecordDataPoint("test-quota", point)
	}

	report := trendMgr.GenerateReport("test-quota", 24*time.Hour)

	// 验证报告结构
	if report.QuotaID != "test-quota" {
		t.Errorf("QuotaID 错误: %s", report.QuotaID)
	}

	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt 不应为零值")
	}

	if report.Duration <= 0 {
		t.Error("Duration 应该大于 0")
	}

	if report.DataPointCount <= 0 {
		t.Error("DataPointCount 应该大于 0")
	}
}

// ========== 边界条件测试 ==========

func TestTrendDataManager_EmptyData(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 没有数据时获取统计
	stats := trendMgr.GetTrendStats("nonexistent", 24*time.Hour)
	if stats != nil {
		t.Error("没有数据时应该返回 nil")
	}

	// 没有数据时预测
	prediction := trendMgr.Predict("nonexistent", 7)
	if prediction != nil {
		t.Error("没有数据时预测应该返回 nil")
	}
}

func TestTrendDataManager_SingleDataPoint(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 只记录一个数据点
	point := TrendDataPointExtended{
		Timestamp:    time.Now(),
		UsedBytes:    50 * 1024 * 1024 * 1024,
		UsagePercent: 50.0,
	}
	trendMgr.RecordDataPoint("test-quota", point)

	stats := trendMgr.GetTrendStats("test-quota", 24*time.Hour)
	if stats == nil {
		t.Fatal("单个数据点也应该返回统计")
	}

	if stats.MinUsagePercent != 50.0 {
		t.Errorf("最小使用率应该是 50.0")
	}

	if stats.MaxUsagePercent != 50.0 {
		t.Errorf("最大使用率应该是 50.0")
	}
}

func TestTrendDataManager_ZeroValues(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	trendMgr := NewTrendDataManager(mgr, config)

	// 记录零值数据点
	point := TrendDataPointExtended{
		Timestamp:    time.Now(),
		UsedBytes:    0,
		UsagePercent: 0,
	}
	trendMgr.RecordDataPoint("test-quota", point)

	data := trendMgr.GetRawData("test-quota", 24*time.Hour)
	if len(data) != 1 {
		t.Error("应该记录零值数据点")
	}
}

// ========== TrendDataManager 启停测试 ==========

func TestTrendDataManager_StartStop(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := DefaultTrendConfig()
	config.PersistEnabled = false // 禁用持久化以简化测试
	trendMgr := NewTrendDataManager(mgr, config)

	// 启动和停止
	trendMgr.Start()
	time.Sleep(100 * time.Millisecond)
	trendMgr.Stop()
}

// ========== AggregationLevel 测试 ==========

func TestAggregationLevel_Default(t *testing.T) {
	config := DefaultTrendConfig()

	levels := config.AggregationLevels
	if len(levels) != 4 {
		t.Errorf("期望 4 个聚合级别, 实际为 %d", len(levels))
	}

	// 验证各级别
	expectedNames := []string{"hourly", "daily", "weekly", "monthly"}
	for i, level := range levels {
		if level.Name != expectedNames[i] {
			t.Errorf("期望级别 %d 名称为 %s, 实际为 %s", i, expectedNames[i], level.Name)
		}
	}
}