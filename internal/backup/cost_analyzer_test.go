// Package backup 备份成本分析功能单元测试
package backup

import (
	"testing"
	"time"
)

// TestNewCostAnalyzer 测试创建成本分析器
func TestNewCostAnalyzer(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	if analyzer == nil {
		t.Fatal("成本分析器创建失败")
	}

	if analyzer.records == nil {
		t.Error("成本记录切片未初始化")
	}

	if analyzer.costConfigs == nil {
		t.Error("成本配置映射未初始化")
	}

	if analyzer.alertThresholds == nil {
		t.Error("告警阈值未初始化")
	}
}

// TestDefaultStorageCostConfigs 测试默认存储成本配置
func TestDefaultStorageCostConfigs(t *testing.T) {
	configs := DefaultStorageCostConfigs()

	requiredProviders := []CloudProvider{"local", CloudProviderS3, CloudProviderAliyun, CloudProviderWebDAV}
	for _, provider := range requiredProviders {
		if _, ok := configs[provider]; !ok {
			t.Errorf("缺少提供商 %s 的成本配置", provider)
		}
	}

	s3Config := configs[CloudProviderS3]
	if s3Config.StoragePricePerGB <= 0 {
		t.Error("S3 存储价格应大于 0")
	}
	if s3Config.AvailabilitySLA <= 0 || s3Config.AvailabilitySLA > 100 {
		t.Error("S3 可用性 SLA 应在 0-100 之间")
	}
}

// TestCalculateBackupCost 测试备份成本计算
func TestCalculateBackupCost(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	config := &JobConfig{
		ID:          "test-config-1",
		Name:        "测试备份",
		Type:        BackupTypeLocal,
		CloudBackup: false,
	}

	record := analyzer.CalculateBackupCost(
		config,
		1024*1024*1024, // 1GB
		512*1024*1024,  // 512MB
		512*1024*1024,
		100,
		time.Minute,
	)

	if record == nil {
		t.Fatal("成本记录创建失败")
	}

	if record.ConfigID != "test-config-1" {
		t.Errorf("配置 ID 不匹配: 期望 test-config-1, 实际 %s", record.ConfigID)
	}

	if record.Provider != "local" {
		t.Errorf("提供商类型不匹配: 期望 local, 实际 %s", record.Provider)
	}

	expectedRatio := 50.0
	if record.CompressionRatio < expectedRatio-1 || record.CompressionRatio > expectedRatio+1 {
		t.Errorf("压缩率计算错误: 期望约 %.1f%%, 实际 %.1f%%", expectedRatio, record.CompressionRatio)
	}

	if record.StorageCost != 0 {
		t.Errorf("本地存储成本应为 0, 实际 %.4f", record.StorageCost)
	}

	records := analyzer.GetRecords(10)
	if len(records) != 1 {
		t.Errorf("记录未正确保存: 期望 1 条, 实际 %d 条", len(records))
	}
}

// TestCalculateBackupCost_S3 测试 S3 备份成本计算
func TestCalculateBackupCost_S3(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	config := &JobConfig{
		ID:          "test-s3-config",
		Name:        "S3 备份",
		Type:        BackupTypeRemote,
		CloudBackup: true,
		CloudConfig: &CloudConfig{
			Provider: CloudProviderS3,
			Bucket:   "test-bucket",
			Region:   "us-east-1",
		},
	}

	record := analyzer.CalculateBackupCost(
		config,
		10*1024*1024*1024, // 10GB
		3*1024*1024*1024,  // 3GB
		3*1024*1024*1024,
		1000,
		5*time.Minute,
	)

	if record == nil {
		t.Fatal("成本记录创建失败")
	}

	if record.Provider != CloudProviderS3 {
		t.Errorf("提供商类型不匹配: 期望 %s, 实际 %s", CloudProviderS3, record.Provider)
	}

	if record.StorageCost <= 0 {
		t.Error("S3 存储成本应大于 0")
	}

	if record.RequestCost <= 0 {
		t.Error("请求成本应大于 0")
	}

	if record.TotalCost != record.StorageCost+record.UploadCost+record.RequestCost {
		t.Error("总成本计算不一致")
	}
}

// TestCalculateRestoreCost 测试恢复成本计算
func TestCalculateRestoreCost(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	config := &JobConfig{
		ID:          "restore-test",
		Name:        "恢复测试",
		CloudBackup: true,
		CloudConfig: &CloudConfig{
			Provider: CloudProviderS3,
		},
	}

	record := analyzer.CalculateRestoreCost(
		config,
		5*1024*1024*1024, // 5GB
		500,
	)

	if record == nil {
		t.Fatal("恢复成本记录创建失败")
	}

	if record.DownloadCost <= 0 {
		t.Error("下载成本应大于 0")
	}

	if record.StorageCost != 0 {
		t.Errorf("恢复操作存储成本应为 0, 实际 %.4f", record.StorageCost)
	}

	if record.UploadCost != 0 {
		t.Errorf("恢复操作上传成本应为 0, 实际 %.4f", record.UploadCost)
	}
}

// TestGetCostTrend 测试成本趋势分析
func TestGetCostTrend(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	now := time.Now()
	for i := 0; i < 10; i++ {
		config := &JobConfig{
			ID:          "trend-test",
			Name:        "趋势测试",
			CloudBackup: true,
			CloudConfig: &CloudConfig{Provider: CloudProviderS3},
		}

		analyzer.CalculateBackupCost(
			config,
			int64(1024*1024*1024*(i+1)),
			int64(512*1024*1024*(i+1)),
			int64(512*1024*1024*(i+1)),
			100,
			time.Minute,
		)

		analyzer.mu.Lock()
		analyzer.records[len(analyzer.records)-1].Timestamp = now.AddDate(0, 0, -i)
		analyzer.mu.Unlock()
	}

	trend, err := analyzer.GetCostTrend(30, PeriodDaily)
	if err != nil {
		t.Fatalf("获取趋势数据失败: %v", err)
	}

	if len(trend) == 0 {
		t.Error("趋势数据不应为空")
	}

	for _, data := range trend {
		if data.Timestamp.IsZero() {
			t.Error("时间戳不应为零值")
		}
		if data.BackupCount <= 0 {
			t.Error("备份数量应大于 0")
		}
	}
}

// TestGenerateCostReport 测试成本报告生成
func TestGenerateCostReport(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	for i := 0; i < 5; i++ {
		config := &JobConfig{
			ID:          "report-test",
			Name:        "报告测试",
			CloudBackup: true,
			CloudConfig: &CloudConfig{Provider: CloudProviderS3},
		}

		analyzer.CalculateBackupCost(
			config,
			1024*1024*1024,
			512*1024*1024,
			512*1024*1024,
			100,
			time.Minute,
		)
	}

	report, err := analyzer.GenerateCostReport(PeriodMonthly)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为空")
	}

	if report.GeneratedAt.IsZero() {
		t.Error("生成时间不应为零值")
	}

	if report.Period != PeriodMonthly {
		t.Errorf("报告周期不匹配: 期望 %s, 实际 %s", PeriodMonthly, report.Period)
	}

	if report.Summary.BackupCount != 5 {
		t.Errorf("备份数量不匹配: 期望 5, 实际 %d", report.Summary.BackupCount)
	}

	if len(report.CostByProvider) == 0 {
		t.Error("按提供商分类的数据不应为空")
	}
}

// TestCheckAlerts 测试告警检查
func TestCheckAlerts(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	analyzer.SetAlertThresholds(&CostAlertThresholds{
		MonthlyCostWarning:   1.0,
		MonthlyCostCritical:  5.0,
		SingleBackupWarning:  0.5,
		SingleBackupCritical: 2.0,
		MinCompressionRatio:  60.0, // 设置较高的阈值以触发压缩率告警
	})

	config := &JobConfig{
		ID:          "alert-test",
		Name:        "告警测试",
		CloudBackup: true,
		CloudConfig: &CloudConfig{Provider: CloudProviderS3},
	}

	// 创建一个大备份以触发月度成本告警
	analyzer.CalculateBackupCost(
		config,
		100*1024*1024*1024, // 100GB
		50*1024*1024*1024,  // 50GB（50% 压缩率）
		50*1024*1024*1024,
		10000,
		10*time.Minute,
	)

	report, _ := analyzer.GenerateCostReport(PeriodMonthly)

	// 检查是否生成了告警（可能是月度成本告警或压缩率告警）
	if len(report.Alerts) == 0 {
		t.Logf("EstimatedMonthlyCost: %.2f", report.Summary.EstimatedMonthlyCost)
		t.Logf("AvgCompressionRatio: %.2f", report.Summary.AvgCompressionRatio)
		t.Error("应该生成告警（月度成本或压缩率）")
	}

	for _, alert := range report.Alerts {
		if alert.Level != AlertLevelWarning && alert.Level != AlertLevelCritical {
			t.Errorf("无效的告警级别: %s", alert.Level)
		}
	}
}

// TestGetOptimizationSuggestions 测试优化建议
func TestGetOptimizationSuggestions(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	for i := 0; i < 10; i++ {
		config := &JobConfig{
			ID:          "optimize-test",
			Name:        "优化测试",
			CloudBackup: true,
			CloudConfig: &CloudConfig{Provider: CloudProviderS3},
		}

		analyzer.CalculateBackupCost(
			config,
			10*1024*1024*1024,
			8*1024*1024*1024,
			8*1024*1024*1024,
			1000,
			time.Minute,
		)
	}

	req := &OptimizeRequest{
		OptimizeGoal: "balance",
	}

	response, err := analyzer.GetOptimizationSuggestions(req)
	if err != nil {
		t.Fatalf("获取优化建议失败: %v", err)
	}

	if response == nil {
		t.Fatal("响应不应为空")
	}

	if response.CurrentCost <= 0 {
		t.Error("当前成本应大于 0")
	}

	if len(response.ImplementationOrder) == 0 {
		t.Error("实施顺序不应为空")
	}
}

// TestSetCostConfig 测试设置成本配置
func TestSetCostConfig(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	customConfig := &StorageCostConfig{
		Provider:           "custom",
		StoragePricePerGB:  0.05,
		DownloadPricePerGB: 0.1,
		UploadPricePerGB:   0.0,
		RequestPricePer10K: 0.005,
		AvailabilitySLA:    99.0,
	}

	analyzer.SetCostConfig("custom", customConfig)

	analyzer.mu.RLock()
	_, exists := analyzer.costConfigs["custom"]
	analyzer.mu.RUnlock()

	if !exists {
		t.Error("自定义配置应已设置")
	}
}

// TestSetAlertThresholds 测试设置告警阈值
func TestSetAlertThresholds(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	customThresholds := &CostAlertThresholds{
		MonthlyCostWarning:  50.0,
		MonthlyCostCritical: 200.0,
		MinCompressionRatio: 40.0,
	}

	analyzer.SetAlertThresholds(customThresholds)

	analyzer.mu.RLock()
	thresholds := analyzer.alertThresholds
	analyzer.mu.RUnlock()

	if thresholds.MonthlyCostWarning != 50.0 {
		t.Errorf("月度成本警告阈值不匹配: 期望 50.0, 实际 %.2f", thresholds.MonthlyCostWarning)
	}

	if thresholds.MinCompressionRatio != 40.0 {
		t.Errorf("最低压缩率阈值不匹配: 期望 40.0, 实际 %.2f", thresholds.MinCompressionRatio)
	}
}

// TestGetRecords 测试获取成本记录
func TestGetRecords(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	for i := 0; i < 20; i++ {
		config := &JobConfig{
			ID:   "records-test",
			Name: "记录测试",
		}

		analyzer.CalculateBackupCost(
			config,
			1024*1024*1024,
			512*1024*1024,
			512*1024*1024,
			100,
			time.Minute,
		)
	}

	allRecords := analyzer.GetRecords(0)
	if len(allRecords) != 20 {
		t.Errorf("获取所有记录数量不匹配: 期望 20, 实际 %d", len(allRecords))
	}

	limitedRecords := analyzer.GetRecords(5)
	if len(limitedRecords) != 5 {
		t.Errorf("限制记录数量不匹配: 期望 5, 实际 %d", len(limitedRecords))
	}
}

// TestGenerateForecast 测试成本预测
func TestGenerateForecast(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	report1, _ := analyzer.GenerateCostReport(PeriodMonthly)
	if report1.Forecast != nil {
		t.Error("数据不足时预测应为空")
	}

	now := time.Now()
	for i := 0; i < 10; i++ {
		config := &JobConfig{
			ID:          "forecast-test",
			Name:        "预测测试",
			CloudBackup: true,
			CloudConfig: &CloudConfig{Provider: CloudProviderS3},
		}

		analyzer.CalculateBackupCost(
			config,
			10*1024*1024*1024,
			5*1024*1024*1024,
			5*1024*1024*1024,
			1000,
			time.Minute,
		)

		analyzer.mu.Lock()
		if len(analyzer.records) > 0 {
			analyzer.records[len(analyzer.records)-1].Timestamp = now.AddDate(0, 0, -i)
		}
		analyzer.mu.Unlock()
	}

	report2, err := analyzer.GenerateCostReport(PeriodMonthly)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report2.Forecast == nil {
		t.Fatal("数据充足时应生成预测")
	}

	if report2.Forecast.ProjectedStorage <= 0 {
		t.Error("预测存储量应大于 0")
	}

	if report2.Forecast.Confidence <= 0 || report2.Forecast.Confidence > 100 {
		t.Errorf("预测置信度应在 0-100 之间: %.2f", report2.Forecast.Confidence)
	}
}

// TestMultipleProviders 测试多提供商成本计算
func TestMultipleProviders(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	providers := []CloudProvider{CloudProviderS3, CloudProviderAliyun, CloudProviderWebDAV}

	for _, provider := range providers {
		config := &JobConfig{
			ID:          string(provider) + "-test",
			Name:        string(provider) + " 测试",
			CloudBackup: true,
			CloudConfig: &CloudConfig{Provider: provider},
		}

		record := analyzer.CalculateBackupCost(
			config,
			10*1024*1024*1024,
			5*1024*1024*1024,
			5*1024*1024*1024,
			1000,
			time.Minute,
		)

		if record.Provider != provider {
			t.Errorf("提供商不匹配: 期望 %s, 实际 %s", provider, record.Provider)
		}

		if record.TotalCost <= 0 {
			t.Errorf("%s 成本应大于 0", provider)
		}
	}

	report, _ := analyzer.GenerateCostReport(PeriodMonthly)
	if len(report.CostByProvider) < 3 {
		t.Errorf("应有至少 3 个提供商的成本分类, 实际 %d", len(report.CostByProvider))
	}
}

// TestCompressionRatioCalculation 测试压缩率计算边界情况
func TestCompressionRatioCalculation(t *testing.T) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	config := &JobConfig{
		ID:   "compression-test",
		Name: "压缩率测试",
	}

	record1 := analyzer.CalculateBackupCost(config, 0, 0, 0, 0, time.Minute)
	if record1.CompressionRatio != 0 {
		t.Errorf("零原始大小时压缩率应为 0, 实际 %.2f", record1.CompressionRatio)
	}

	record2 := analyzer.CalculateBackupCost(config, 1024*1024*1024, 0, 0, 0, time.Minute)
	if record2.CompressionRatio != 100 {
		t.Errorf("100%% 压缩时压缩率应为 100, 实际 %.2f", record2.CompressionRatio)
	}

	record3 := analyzer.CalculateBackupCost(config, 100, 50, 50, 10, time.Minute)
	if record3.CompressionRatio != 50 {
		t.Errorf("50%% 压缩率计算错误: 期望 50, 实际 %.2f", record3.CompressionRatio)
	}
}

// BenchmarkCalculateBackupCost 基准测试备份成本计算
func BenchmarkCalculateBackupCost(b *testing.B) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	config := &JobConfig{
		ID:          "bench-test",
		Name:        "基准测试",
		CloudBackup: true,
		CloudConfig: &CloudConfig{Provider: CloudProviderS3},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.CalculateBackupCost(
			config,
			10*1024*1024*1024,
			5*1024*1024*1024,
			5*1024*1024*1024,
			1000,
			time.Minute,
		)
	}
}

// BenchmarkGenerateCostReport 基准测试报告生成
func BenchmarkGenerateCostReport(b *testing.B) {
	manager := NewManager("", "")
	analyzer := NewCostAnalyzer(manager)

	for i := 0; i < 100; i++ {
		config := &JobConfig{
			ID:          "bench-report",
			Name:        "报告基准测试",
			CloudBackup: true,
			CloudConfig: &CloudConfig{Provider: CloudProviderS3},
		}

		analyzer.CalculateBackupCost(
			config,
			10*1024*1024*1024,
			5*1024*1024*1024,
			5*1024*1024*1024,
			1000,
			time.Minute,
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.GenerateCostReport(PeriodMonthly)
	}
}
