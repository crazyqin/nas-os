package diskpredict

import (
	"testing"
)

func TestNewAnalyzer(t *testing.T) {
	analyzer := NewAnalyzer()
	if analyzer == nil {
		t.Fatal("NewAnalyzer 返回 nil")
	}

	if len(analyzer.weights) == 0 {
		t.Fatal("权重配置为空")
	}

	// 检查关键属性权重是否配置
	criticalAttrs := []uint8{5, 187, 188, 197, 198, 9}
	for _, id := range criticalAttrs {
		if _, exists := analyzer.weights[id]; !exists {
			t.Errorf("缺少属性 %d 的权重配置", id)
		}
	}
}

func TestAnalyzeTemperature(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		temp          int
		expectedScore float64
		expectedMsg   string
	}{
		{25, 100, "温度正常"},
		{35, 90, "温度正常"},
		{42, 80, "温度略高"},
		{48, 60, "温度偏高，建议改善散热"},
		{52, 40, "温度过高，可能影响寿命"},
		{58, 20, "温度危险，可能导致故障"},
		{65, 0, "温度极高，立即停止使用"},
	}

	for _, test := range tests {
		score, msg := analyzer.AnalyzeTemperature(test.temp)
		if score != test.expectedScore {
			t.Errorf("温度 %d: 期望分数 %.0f, 实际 %.0f", test.temp, test.expectedScore, score)
		}
		if msg != test.expectedMsg {
			t.Errorf("温度 %d: 期望消息 '%s', 实际 '%s'", test.temp, test.expectedMsg, msg)
		}
	}
}

func TestAnalyzePowerOnHours(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		hours         uint64
		expectedScore float64
		expectedMsg   string
	}{
		{0, 100, "新磁盘"},
		{5000, 100, "通电时间正常"},
		{15000, 90, "通电时间正常"},
		{25000, 80, "通电时间较长"},
		{35000, 70, "通电时间较长"},
		{45000, 50, "通电时间很长"},
		{55000, 40, "通电时间超长"},
		{65000, 30, "通电时间超长"},
		{75000, 20, "建议更换磁盘"},
		{80000, 10, "强烈建议更换磁盘"},
	}

	for _, test := range tests {
		score, msg := analyzer.AnalyzePowerOnHours(test.hours)
		if score != test.expectedScore {
			t.Errorf("通电时间 %d: 期望分数 %.0f, 实际 %.0f", test.hours, test.expectedScore, score)
		}
		if msg != test.expectedMsg {
			t.Errorf("通电时间 %d: 期望消息 '%s', 实际 '%s'", test.hours, test.expectedMsg, msg)
		}
	}
}

func TestNewScorer(t *testing.T) {
	scorer := NewScorer()
	if scorer == nil {
		t.Fatal("NewScorer 返回 nil")
	}

	// 检查权重是否合理（总和应该接近1）
	totalWeight := scorer.attributeWeight + scorer.temperatureWeight +
		scorer.powerOnWeight + scorer.overallWeight
	if totalWeight < 0.99 || totalWeight > 1.01 {
		t.Errorf("权重总和不正确: %.2f, 期望接近 1.0", totalWeight)
	}
}

func TestDetermineStatus(t *testing.T) {
	scorer := NewScorer()

	tests := []struct {
		score    float64
		expected DiskStatus
	}{
		{90, StatusHealthy},
		{80, StatusHealthy},
		{70, StatusWarning},
		{60, StatusWarning},
		{50, StatusCritical},
		{40, StatusCritical},
		{30, StatusFailed},
		{20, StatusFailed},
	}

	for _, test := range tests {
		status := scorer.DetermineStatus(test.score)
		if status != test.expected {
			t.Errorf("分数 %.0f: 期望状态 %s, 实际 %s", test.score, test.expected, status)
		}
	}
}

func TestEstimateRemainingLifeDays(t *testing.T) {
	scorer := NewScorer()

	tests := []struct {
		score        float64
		status       DiskStatus
		riskFactors  []string
		expectedMin  int
		expectedMax  int
	}{
		{90, StatusHealthy, nil, 1000, 1100},
		{80, StatusHealthy, nil, 700, 800},
		{60, StatusWarning, nil, 350, 400},
		{40, StatusCritical, nil, 170, 200},
		{20, StatusFailed, nil, 80, 100},
		{80, StatusHealthy, []string{"温度过高"}, 600, 750},
		{60, StatusWarning, []string{"存在重分配扇区", "温度过高"}, 250, 350},
	}

	for _, test := range tests {
		days := scorer.EstimateRemainingLifeDays(test.score, test.status, test.riskFactors)
		if days < test.expectedMin || days > test.expectedMax {
			t.Errorf("分数 %.0f, 状态 %s, 风险因素 %v: 期望天数 %d-%d, 实际 %d",
				test.score, test.status, test.riskFactors, test.expectedMin, test.expectedMax, days)
		}
	}
}

func TestNewDiskPredictManager(t *testing.T) {
	manager := NewDiskPredictManager()
	if manager == nil {
		t.Fatal("NewDiskPredictManager 返回 nil")
	}

	if manager.disks == nil {
		t.Fatal("disks map 未初始化")
	}

	if manager.smartData == nil {
		t.Fatal("smartData map 未初始化")
	}

	if manager.predictions == nil {
		t.Fatal("predictions map 未初始化")
	}

	if manager.analyzer == nil {
		t.Fatal("analyzer 未初始化")
	}

	if manager.scorer == nil {
		t.Fatal("scorer 未初始化")
	}
}

func TestRegisterDisk(t *testing.T) {
	manager := NewDiskPredictManager()

	// 测试正常注册
	disk := &DiskInfo{
		Device: "sda",
		Model:  "Test Disk",
		Serial: "123456",
	}

	err := manager.RegisterDisk(disk)
	if err != nil {
		t.Fatalf("注册磁盘失败: %v", err)
	}

	// 测试设备名为空
	emptyDisk := &DiskInfo{
		Device: "",
		Model:  "Test Disk",
	}

	err = manager.RegisterDisk(emptyDisk)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}

	// 测试重复注册
	err = manager.RegisterDisk(disk)
	if err != nil {
		t.Fatalf("重复注册应该成功: %v", err)
	}
}

func TestGetDisk(t *testing.T) {
	manager := NewDiskPredictManager()

	// 注册磁盘
	disk := &DiskInfo{
		Device: "sda",
		Model:  "Test Disk",
		Serial: "123456",
	}
	manager.RegisterDisk(disk)

	// 测试获取存在的磁盘
	result, err := manager.GetDisk("sda")
	if err != nil {
		t.Fatalf("获取磁盘失败: %v", err)
	}

	if result.Device != "sda" {
		t.Errorf("期望设备名 sda, 实际 %s", result.Device)
	}

	// 测试获取不存在的磁盘
	_, err = manager.GetDisk("sdb")
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestUpdateSMARTData(t *testing.T) {
	manager := NewDiskPredictManager()

	// 注册磁盘
	disk := &DiskInfo{
		Device: "sda",
		Model:  "Test Disk",
	}
	manager.RegisterDisk(disk)

	// 测试更新SMART数据
	smartData := &SMARTData{
		Device:      "sda",
		Temperature: 40,
		PowerOnHours: 10000,
		Attributes: []SMARTAttribute{
			{ID: 5, Name: "Reallocated_Sector_Ct", Value: 100, Threshold: 10, RawValue: 0},
			{ID: 197, Name: "Current_Pending_Sector", Value: 100, Threshold: 0, RawValue: 0},
		},
	}

	err := manager.UpdateSMARTData(smartData)
	if err != nil {
		t.Fatalf("更新SMART数据失败: %v", err)
	}

	// 测试更新未注册磁盘的数据
	smartData2 := &SMARTData{
		Device: "sdb",
	}
	err = manager.UpdateSMARTData(smartData2)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestPredictFailure(t *testing.T) {
	manager := NewDiskPredictManager()

	// 注册磁盘
	disk := &DiskInfo{
		Device: "sda",
		Model:  "Test Disk",
		Serial: "123456",
	}
	manager.RegisterDisk(disk)

	// 更新SMART数据（健康状态）
	smartData := &SMARTData{
		Device:      "sda",
		Temperature: 35,
		PowerOnHours: 10000,
		Attributes: []SMARTAttribute{
			{ID: 5, Name: "Reallocated_Sector_Ct", Value: 100, Threshold: 10, RawValue: 0},
			{ID: 197, Name: "Current_Pending_Sector", Value: 100, Threshold: 0, RawValue: 0},
			{ID: 198, Name: "Offline_Uncorrectable", Value: 100, Threshold: 0, RawValue: 0},
		},
	}
	manager.UpdateSMARTData(smartData)

	// 测试预测
	result, err := manager.PredictFailure("sda")
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if result == nil {
		t.Fatal("预测结果为 nil")
	}

	if result.HealthScore < 0 || result.HealthScore > 100 {
		t.Errorf("健康评分超出范围: %.1f", result.HealthScore)
	}

	if result.EstimatedLifeDays <= 0 {
		t.Errorf("预测寿命应该大于0: %d", result.EstimatedLifeDays)
	}

	if result.Status == "" {
		t.Error("状态不能为空")
	}

	// 测试不存在的磁盘
	_, err = manager.PredictFailure("sdb")
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestPredictAll(t *testing.T) {
	manager := NewDiskPredictManager()

	// 注册多个磁盘
	for i := 0; i < 3; i++ {
		disk := &DiskInfo{
			Device: "sd" + string(rune('a'+i)),
			Model:  "Test Disk",
		}
		manager.RegisterDisk(disk)

		smartData := &SMARTData{
			Device:      "sd" + string(rune('a'+i)),
			Temperature: 35,
			PowerOnHours: 10000,
			Attributes: []SMARTAttribute{
				{ID: 5, Name: "Reallocated_Sector_Ct", Value: 100, Threshold: 10, RawValue: 0},
			},
		}
		manager.UpdateSMARTData(smartData)
	}

	// 测试预测所有磁盘
	results := manager.PredictAll()

	if len(results) != 3 {
		t.Errorf("期望3个结果, 实际 %d", len(results))
	}

	for _, result := range results {
		if result.HealthScore < 0 || result.HealthScore > 100 {
			t.Errorf("健康评分超出范围: %.1f", result.HealthScore)
		}
	}
}

func TestGetStats(t *testing.T) {
	manager := NewDiskPredictManager()

	// 注册磁盘并设置不同状态
	statuses := []DiskStatus{StatusHealthy, StatusWarning, StatusCritical, StatusFailed}
	for i, status := range statuses {
		disk := &DiskInfo{
			Device: "sd" + string(rune('a'+i)),
			Status: status,
		}
		manager.RegisterDisk(disk)
	}

	stats := manager.GetStats()

	if stats.TotalDisks != 4 {
		t.Errorf("期望4个磁盘, 实际 %d", stats.TotalDisks)
	}

	if stats.HealthyDisks != 1 {
		t.Errorf("期望1个健康磁盘, 实际 %d", stats.HealthyDisks)
	}

	if stats.WarningDisks != 1 {
		t.Errorf("期望1个警告磁盘, 实际 %d", stats.WarningDisks)
	}

	if stats.CriticalDisks != 1 {
		t.Errorf("期望1个临界磁盘, 实际 %d", stats.CriticalDisks)
	}

	if stats.FailedDisks != 1 {
		t.Errorf("期望1个失败磁盘, 实际 %d", stats.FailedDisks)
	}
}

func TestIdentifyRiskFactors(t *testing.T) {
	analyzer := NewAnalyzer()

	// 测试有风险因素的情况
	analyses := []AttributeAnalysis{
		{ID: 5, Name: "Reallocated_Sector_Ct", Status: "critical"},
		{ID: 197, Name: "Current_Pending_Sector", Status: "critical"},
	}

	riskFactors := analyzer.IdentifyRiskFactors(analyses, 55, 50000)

	// 应该有4个风险因素：2个属性 + 温度 + 通电时间
	if len(riskFactors) < 3 {
		t.Errorf("期望至少3个风险因素, 实际 %d", len(riskFactors))
	}

	// 测试无风险因素的情况
	analyses2 := []AttributeAnalysis{
		{ID: 5, Name: "Reallocated_Sector_Ct", Status: "normal"},
	}

	riskFactors2 := analyzer.IdentifyRiskFactors(analyses2, 35, 10000)

	if len(riskFactors2) != 0 {
		t.Errorf("期望0个风险因素, 实际 %d", len(riskFactors2))
	}
}

func TestScoreToGrade(t *testing.T) {
	scorer := NewScorer()

	tests := []struct {
		score    float64
		expected string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{55, "F"},
	}

	for _, test := range tests {
		grade := scorer.ScoreToGrade(test.score)
		if grade != test.expected {
			t.Errorf("分数 %.0f: 期望等级 '%s', 实际 '%s'", test.score, test.expected, grade)
		}
	}
}

func TestScoreToEmoji(t *testing.T) {
	scorer := NewScorer()

	tests := []struct {
		score    float64
		expected string
	}{
		{95, "😊"},
		{85, "🙂"},
		{75, "😐"},
		{65, "😟"},
		{50, "😨"},
		{30, "💀"},
	}

	for _, test := range tests {
		emoji := scorer.ScoreToEmoji(test.score)
		if emoji != test.expected {
			t.Errorf("分数 %.0f: 期望emoji '%s', 实际 '%s'", test.score, test.expected, emoji)
		}
	}
}

func TestResolveAlert(t *testing.T) {
	manager := NewDiskPredictManager()

	// 注册磁盘并生成告警
	disk := &DiskInfo{
		Device: "sda",
		Model:  "Test Disk",
	}
	manager.RegisterDisk(disk)

	smartData := &SMARTData{
		Device:      "sda",
		Temperature: 55,
		PowerOnHours: 50000,
		Attributes: []SMARTAttribute{
			{ID: 5, Name: "Reallocated_Sector_Ct", Value: 50, Threshold: 10, RawValue: 100},
		},
	}
	manager.UpdateSMARTData(smartData)

	// 触发预测以生成告警
	manager.PredictFailure("sda")

	// 获取告警
	alerts := manager.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("应该有告警但没有")
	}

	// 解决告警
	firstAlert := alerts[0]
	err := manager.ResolveAlert(firstAlert.Device, firstAlert.CreatedAt)
	if err != nil {
		t.Fatalf("解决告警失败: %v", err)
	}

	// 验证告警已解决
	resolvedAlerts := manager.GetAlerts(true)
	if len(resolvedAlerts) == 0 {
		t.Fatal("应该有已解决的告警")
	}
}

func TestExportReport(t *testing.T) {
	manager := NewDiskPredictManager()

	// 注册磁盘
	disk := &DiskInfo{
		Device:   "sda",
		Model:    "Test Disk",
		Serial:   "123456",
		Capacity: 1024 * 1024 * 1024 * 1024, // 1TB
		Status:   StatusHealthy,
	}
	manager.RegisterDisk(disk)

	// 导出报告
	report := manager.ExportReport()

	if report == nil {
		t.Fatal("报告为 nil")
	}

	totalDisks, ok := report["total_disks"].(int)
	if !ok {
		t.Fatal("total_disks 类型错误")
	}

	if totalDisks != 1 {
		t.Errorf("期望1个磁盘, 实际 %d", totalDisks)
	}

	disks, ok := report["disks"].([]map[string]interface{})
	if !ok {
		t.Fatal("disks 类型错误")
	}

	if len(disks) != 1 {
		t.Errorf("期望1个磁盘报告, 实际 %d", len(disks))
	}
}
