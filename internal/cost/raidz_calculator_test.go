// Package cost - RAIDZ扩容成本计算器测试
package cost

import (
	"testing"
)

func TestRAIDZCalculator_CalculateCapacity_RaidZ1(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// RAIDZ1: 4块4TB盘，可用容量约3盘数据 = 12TB（扣除元数据）
	config := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         4,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024, // 4TB
		DiskPrice:         1200.0,
	}

	result := calc.CalculateCapacity(config)

	// 检查冗余盘数
	if result.ParityDisks != 1 {
		t.Errorf("RAIDZ1 parity disks should be 1, got %d", result.ParityDisks)
	}

	// 检查可用容量（约12TB，扣除2%元数据）
	expectedGB := 3 * 4 * 1024 * 0.98 // 3盘 × 4TB × 0.98
	if result.UsableCapacityGB < expectedGB-10 || result.UsableCapacityGB > expectedGB+10 {
		t.Errorf("Usable capacity should be ~%.2f GB, got %.2f GB", expectedGB, result.UsableCapacityGB)
	}

	// 检查空间利用率
	expectedUtil := 75.0 // 3/4 = 75%
	if result.SpaceUtilization < expectedUtil-5 || result.SpaceUtilization > expectedUtil+5 {
		t.Errorf("Space utilization should be ~%.2f%%, got %.2f%%", expectedUtil, result.SpaceUtilization)
	}

	// 检查单位成本
	expectedCostPerGB := 4800.0 / result.UsableCapacityGB // 4盘 × 1200
	if result.CostPerGB < expectedCostPerGB-0.5 || result.CostPerGB > expectedCostPerGB+0.5 {
		t.Errorf("Cost per GB should be ~%.2f, got %.2f", expectedCostPerGB, result.CostPerGB)
	}
}

func TestRAIDZCalculator_CalculateCapacity_RaidZ2(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// RAIDZ2: 6块4TB盘，可用容量约4盘数据 = 16TB
	config := RAIDZConfig{
		Level:             RAIDZ2,
		DiskCount:         6,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	result := calc.CalculateCapacity(config)

	// 检查冗余盘数
	if result.ParityDisks != 2 {
		t.Errorf("RAIDZ2 parity disks should be 2, got %d", result.ParityDisks)
	}

	// 检查可用容量
	expectedGB := 4 * 4 * 1024 * 0.98 // 4盘 × 4TB × 0.98
	if result.UsableCapacityGB < expectedGB-10 || result.UsableCapacityGB > expectedGB+10 {
		t.Errorf("Usable capacity should be ~%.2f GB, got %.2f GB", expectedGB, result.UsableCapacityGB)
	}

	// 检查空间利用率
	expectedUtil := 66.67 // 4/6 ≈ 66.67%
	if result.SpaceUtilization < expectedUtil-5 || result.SpaceUtilization > expectedUtil+5 {
		t.Errorf("Space utilization should be ~%.2f%%, got %.2f%%", expectedUtil, result.SpaceUtilization)
	}
}

func TestRAIDZCalculator_CalculateCapacity_RaidZ3(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// RAIDZ3: 8块4TB盘，可用容量约5盘数据 = 20TB
	config := RAIDZConfig{
		Level:             RAIDZ3,
		DiskCount:         8,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	result := calc.CalculateCapacity(config)

	// 检查冗余盘数
	if result.ParityDisks != 3 {
		t.Errorf("RAIDZ3 parity disks should be 3, got %d", result.ParityDisks)
	}

	// 检查可用容量
	expectedGB := 5 * 4 * 1024 * 0.98
	if result.UsableCapacityGB < expectedGB-10 || result.UsableCapacityGB > expectedGB+10 {
		t.Errorf("Usable capacity should be ~%.2f GB, got %.2f GB", expectedGB, result.UsableCapacityGB)
	}
}

func TestRAIDZCalculator_CalculateCapacity_Stripe(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// Stripe: 4块盘，全部可用（无冗余）
	config := RAIDZConfig{
		Level:             Stripe,
		DiskCount:         4,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	result := calc.CalculateCapacity(config)

	// 检查冗余盘数
	if result.ParityDisks != 0 {
		t.Errorf("Stripe parity disks should be 0, got %d", result.ParityDisks)
	}

	// 检查可用容量（几乎全部可用，仅扣除元数据）
	expectedGB := 4 * 4 * 1024 * 0.98
	if result.UsableCapacityGB < expectedGB-10 || result.UsableCapacityGB > expectedGB+10 {
		t.Errorf("Usable capacity should be ~%.2f GB, got %.2f GB", expectedGB, result.UsableCapacityGB)
	}

	// 空间利用率应该接近98%
	if result.SpaceUtilization < 97 || result.SpaceUtilization > 99 {
		t.Errorf("Stripe utilization should be ~98%%, got %.2f%%", result.SpaceUtilization)
	}
}

func TestRAIDZCalculator_CalculateMirrorCapacity(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// Mirror: 4块盘（2组镜像），可用容量约8TB（2盘）
	config := RAIDZConfig{
		Level:             Mirror,
		DiskCount:         4,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	result := calc.CalculateMirrorCapacity(config)

	// 检查可用容量（2盘数据容量）
	expectedGB := 2 * 4 * 1024 * 0.98
	if result.UsableCapacityGB < expectedGB-10 || result.UsableCapacityGB > expectedGB+10 {
		t.Errorf("Mirror usable capacity should be ~%.2f GB, got %.2f GB", expectedGB, result.UsableCapacityGB)
	}

	// 空间利用率应该是50%
	if result.SpaceUtilization < 48 || result.SpaceUtilization > 52 {
		t.Errorf("Mirror utilization should be ~50%%, got %.2f%%", result.SpaceUtilization)
	}
}

func TestRAIDZCalculator_AnalyzeExpansion(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// 当前配置: RAIDZ1，4块4TB盘，约12TB可用
	currentConfig := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         4,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	// 目标容量: 20TB
	targetCapacityGB := 20.0 * 1024 // 20TB in GB

	analysis := calc.AnalyzeExpansion(currentConfig, targetCapacityGB)

	// 检查分析结果
	if analysis == nil {
		t.Fatal("Analysis should not be nil")
	}

	// 检查当前容量
	currentResult := calc.CalculateCapacity(currentConfig)
	if analysis.Current.UsableCapacityGB != currentResult.UsableCapacityGB {
		t.Errorf("Current capacity mismatch")
	}

	// 检查方案数量
	if len(analysis.Plans) == 0 {
		t.Error("Should have at least one expansion plan")
	}

	// 检查最优方案
	if analysis.BestPlan == nil {
		t.Error("Should have a best plan")
	}

	// 检查最优方案的目标容量是否满足要求
	if analysis.BestPlan.TargetCapacityGB < targetCapacityGB*0.95 {
		t.Errorf("Best plan target %.2f GB should meet requirement %.2f GB",
			analysis.BestPlan.TargetCapacityGB, targetCapacityGB)
	}
}

func TestRAIDZCalculator_GenerateSameLevelPlan(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// RAIDZ1，3块4TB盘，约8TB可用，目标16TB
	currentConfig := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         3,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	targetCapacityGB := 16.0 * 1024

	analysis := calc.AnalyzeExpansion(currentConfig, targetCapacityGB)

	// 检查同级别方案
	var sameLevelPlan *ExpansionPlan
	for _, plan := range analysis.Plans {
		if plan.TargetConfig.Level == currentConfig.Level {
			sameLevelPlan = &plan
			break
		}
	}

	if sameLevelPlan == nil {
		t.Fatal("Should have same-level expansion plan")
	}

	// 检查新盘数量
	// 需要达到16TB，RAIDZ1需要约17块4TB盘（16盘数据+1冗余）
	// 当前3块，需要增加14块
	if sameLevelPlan.NewDiskCount <= 0 {
		t.Errorf("Should need new disks, got %d", sameLevelPlan.NewDiskCount)
	}

	// 检查成本计算
	expectedCost := float64(sameLevelPlan.NewDiskCount) * currentConfig.DiskPrice
	if sameLevelPlan.ExpansionCost != expectedCost {
		t.Errorf("Expansion cost should be %.2f, got %.2f", expectedCost, sameLevelPlan.ExpansionCost)
	}
}

func TestRAIDZCalculator_CompareRAIDZLevels(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// 对比8块4TB盘的不同RAIDZ级别
	diskCount := 8
	diskCapacity := uint64(4 * 1024 * 1024 * 1024 * 1024)
	diskPrice := 1200.0

	results := calc.CompareRAIDZLevels(diskCount, diskCapacity, diskPrice)

	// RAIDZ1应该满足最小要求（3盘）
	if results[RAIDZ1] == nil {
		t.Error("RAIDZ1 result should exist for 8 disks")
	}

	// RAIDZ2应该满足最小要求（4盘）
	if results[RAIDZ2] == nil {
		t.Error("RAIDZ2 result should exist for 8 disks")
	}

	// RAIDZ3应该满足最小要求（5盘）
	if results[RAIDZ3] == nil {
		t.Error("RAIDZ3 result should exist for 8 disks")
	}

	// Stripe应该满足（1盘）
	if results[Stripe] == nil {
		t.Error("Stripe result should exist for 8 disks")
	}

	// 对比可用容量
	// Stripe应该最高
	if results[Stripe].UsableCapacityGB <= results[RAIDZ1].UsableCapacityGB {
		t.Error("Stripe should have highest usable capacity")
	}

	// RAIDZ3应该最低（最多冗余）
	if results[RAIDZ3].UsableCapacityGB >= results[RAIDZ2].UsableCapacityGB {
		t.Error("RAIDZ3 should have lowest usable capacity (most parity)")
	}

	// 对比成本效率
	// Stripe单位成本应该最低
	if results[Stripe].CostPerGB >= results[RAIDZ1].CostPerGB {
		t.Error("Stripe should have lowest cost per GB")
	}
}

func TestRAIDZCalculator_ValidateConfig(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// 测试正常配置
	validConfig := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         4,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	issues := calc.ValidateConfig(validConfig)
	if len(issues) > 0 {
		t.Errorf("Valid config should have no issues, got: %v", issues)
	}

	// 测试盘数不足
	invalidConfig := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         2, // RAIDZ1最少需要3盘
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	issues = calc.ValidateConfig(invalidConfig)
	if len(issues) == 0 {
		t.Error("Invalid config should have issues")
	}

	// 测试镜像盘数奇数
	mirrorConfig := RAIDZConfig{
		Level:             Mirror,
		DiskCount:         3, // 镜像需要偶数盘数
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	issues = calc.ValidateConfig(mirrorConfig)
	if len(issues) == 0 {
		t.Error("Mirror with odd disk count should have issues")
	}

	// 测试容量为0
	zeroCapacityConfig := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         4,
		DiskCapacityBytes: 0,
		DiskPrice:         1200.0,
	}

	issues = calc.ValidateConfig(zeroCapacityConfig)
	if len(issues) == 0 {
		t.Error("Zero capacity config should have issues")
	}
}

func TestRAIDZCalculator_GetRecommendedConfig(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// 目标20TB，预算15000元，偏好RAIDZ2
	targetCapacityGB := 20.0 * 1024
	maxBudget := 15000.0

	config := calc.GetRecommendedConfig(targetCapacityGB, maxBudget, RAIDZ2)

	if config == nil {
		t.Fatal("Should get recommended config")
	}

	// 检查级别
	if config.Level != RAIDZ2 {
		t.Errorf("Recommended level should be RAIDZ2, got %s", config.Level)
	}

	// 检查是否满足预算
	totalCost := float64(config.DiskCount) * config.DiskPrice
	if totalCost > maxBudget {
		t.Errorf("Total cost %.2f should not exceed budget %.2f", totalCost, maxBudget)
	}

	// 检查是否满足最小盘数要求
	if config.DiskCount < calc.MinDiskRecommendations[RAIDZ2] {
		t.Errorf("Disk count %d should meet minimum %d",
			config.DiskCount, calc.MinDiskRecommendations[RAIDZ2])
	}
}

func TestRAIDZCalculator_GetRecommendedConfig_InsufficientBudget(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// 目标20TB，预算仅1000元（不足）
	targetCapacityGB := 20.0 * 1024
	maxBudget := 1000.0 // 远低于所需成本

	config := calc.GetRecommendedConfig(targetCapacityGB, maxBudget, RAIDZ2)

	// 预算不足时应该返回nil或调整配置
	// 当前实现返回预算内的最大配置
	if config != nil {
		totalCost := float64(config.DiskCount) * config.DiskPrice
		if totalCost > maxBudget {
			t.Errorf("If config returned, cost %.2f should be within budget %.2f", totalCost, maxBudget)
		}
	}
}

func TestRAIDZCalculator_EfficiencyScore(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// 最小盘数配置（效率较低）
	minConfig := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         3, // 最小
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	minResult := calc.CalculateCapacity(minConfig)

	// 较大配置（效率较高）
	largeConfig := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         8, // 较大规模
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	largeResult := calc.CalculateCapacity(largeConfig)

	// 大规模配置效率评分应该更高
	if largeResult.EfficiencyScore <= minResult.EfficiencyScore {
		t.Errorf("Large config efficiency %.1f should be higher than min config %.1f",
			largeResult.EfficiencyScore, minResult.EfficiencyScore)
	}
}

func TestRAIDZCalculator_ROIScore(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// 低成本扩容方案
	lowCostPlan := &ExpansionPlan{
		NewDiskCount:         2,
		CostPerGBAdded:       0.5,
		CapacityGrowthPercent: 100,
	}

	lowCostROI := calc.calculateROIScore(lowCostPlan)

	// 高成本扩容方案
	highCostPlan := &ExpansionPlan{
		NewDiskCount:         10,
		CostPerGBAdded:       10.0,
		CapacityGrowthPercent: 20,
	}

	highCostROI := calc.calculateROIScore(highCostPlan)

	// 低成本方案ROI应该更高
	if lowCostROI <= highCostROI {
		t.Errorf("Low cost ROI %.1f should be higher than high cost ROI %.1f",
			lowCostROI, highCostROI)
	}
}

func TestRAIDZCalculator_Warnings(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	// 低使用率场景
	currentConfig := RAIDZConfig{
		Level:             RAIDZ1,
		DiskCount:         4,
		DiskCapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		DiskPrice:         1200.0,
	}

	// 目标容量与当前相近（不需要大扩容）
	targetCapacityGB := 12.0 * 1024 // 接近当前容量

	analysis := calc.AnalyzeExpansion(currentConfig, targetCapacityGB)

	if analysis == nil {
		t.Fatal("Analysis should not be nil")
	}

	// 检查是否有建议
	if len(analysis.Suggestions) == 0 {
		t.Error("Should have suggestions")
	}
}

func TestParityDiskCalculation(t *testing.T) {
	calc := DefaultRAIDZCalculator()

	tests := []struct {
		level         RAIDZLevel
		expectedParity int
	}{
		{RAIDZ1, 1},
		{RAIDZ2, 2},
		{RAIDZ3, 3},
		{Stripe, 0},
	}

	for _, tt := range tests {
		parity := calc.getParityDisks(tt.level)
		if parity != tt.expectedParity {
			t.Errorf("%s parity should be %d, got %d", tt.level, tt.expectedParity, parity)
		}
	}
}