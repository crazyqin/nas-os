package driveinsight

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ========== 测试辅助函数 ==========

func testLogger() *zap.Logger {
	return zap.NewNop()
}

func makeTestDrive(serial, model string, capacity int64, driveType DriveType) DriveStats {
	return DriveStats{
		SerialNumber:   serial,
		Model:          model,
		DevicePath:     "/dev/sda",
		Interface:      "SATA",
		Type:           driveType,
		CapacityBytes:  capacity,
		UsedBytes:      capacity / 2,
		HealthStatus:   HealthGood,
		TemperatureC:   35.0,
		PowerOnHours:   1000,
		IOPS:           5000,
		ThroughputMBps: 200.0,
	}
}

func makeTestTier(name string, tierType TierType, capacity, used int64, costPerTB float64) StorageTier {
	return StorageTier{
		Name:          name,
		Type:          tierType,
		CapacityBytes: capacity,
		UsedBytes:     used,
		FreeBytes:     capacity - used,
		CostPerTB:     costPerTB,
	}
}

// ========== 模型测试 ==========

func TestDriveType_Constants(t *testing.T) {
	assert.Equal(t, DriveType("HDD"), DriveTypeHDD)
	assert.Equal(t, DriveType("SSD"), DriveTypeSSD)
	assert.Equal(t, DriveType("NVMe"), DriveTypeNVMe)
	assert.Equal(t, DriveType("Hybrid"), DriveTypeHybrid)
}

func TestHealthStatus_Constants(t *testing.T) {
	assert.Equal(t, HealthStatus("good"), HealthGood)
	assert.Equal(t, HealthStatus("warning"), HealthWarning)
	assert.Equal(t, HealthStatus("critical"), HealthCritical)
	assert.Equal(t, HealthStatus("unknown"), HealthUnknown)
}

func TestTierType_Constants(t *testing.T) {
	assert.Equal(t, TierType("SSD"), TierTypeSSD)
	assert.Equal(t, TierType("HDD"), TierTypeHDD)
	assert.Equal(t, TierType("Hybrid"), TierTypeHybrid)
	assert.Equal(t, TierType("NVMe"), TierTypeNVMe)
}

func TestTierID_Constants(t *testing.T) {
	assert.Equal(t, TierID("hot"), TierIDHot)
	assert.Equal(t, TierID("warm"), TierIDWarm)
	assert.Equal(t, TierID("cold"), TierIDCold)
	assert.Equal(t, TierID("archive"), TierIDArchive)
}

func TestAccessFreq_Constants(t *testing.T) {
	assert.Equal(t, AccessFreq("hot"), AccessFreqHot)
	assert.Equal(t, AccessFreq("warm"), AccessFreqWarm)
	assert.Equal(t, AccessFreq("cold"), AccessFreqCold)
	assert.Equal(t, AccessFreq("frozen"), AccessFreqFrozen)
}

// ========== 采集器测试 ==========

func TestNewCollector(t *testing.T) {
	c := NewCollector(nil)
	require.NotNil(t, c)
	assert.NotNil(t, c.drives)
	assert.NotNil(t, c.tempHistory)
	assert.NotNil(t, c.tierCosts)
}

func TestCollector_CollectDrive(t *testing.T) {
	c := NewCollector(testLogger())

	drive := makeTestDrive("SN001", "Samsung 980 Pro", 1024*1024*1024*1024, DriveTypeNVMe)
	err := c.CollectDrive(drive)
	require.NoError(t, err)

	got, err := c.GetDrive("SN001")
	require.NoError(t, err)
	assert.Equal(t, "SN001", got.SerialNumber)
	assert.Equal(t, "Samsung 980 Pro", got.Model)
	assert.Equal(t, DriveTypeNVMe, got.Type)
	assert.True(t, got.FreeBytes > 0)
	assert.False(t, got.LastUpdated.IsZero())
}

func TestCollector_CollectDrive_InvalidParams(t *testing.T) {
	c := NewCollector(testLogger())

	// 空序列号
	err := c.CollectDrive(DriveStats{CapacityBytes: 1024})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "序列号")

	// 容量为0
	err = c.CollectDrive(DriveStats{SerialNumber: "SN002", CapacityBytes: 0})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "容量")

	// 已用超过总容量
	err = c.CollectDrive(DriveStats{
		SerialNumber:  "SN003",
		CapacityBytes: 1000,
		UsedBytes:     2000,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已用容量超过总容量")
}

func TestCollector_CollectDrive_HealthInference(t *testing.T) {
	c := NewCollector(testLogger())

	tests := []struct {
		temp     float64
		expected HealthStatus
	}{
		{0, HealthUnknown},
		{35.0, HealthGood},
		{55.0, HealthWarning},
		{70.0, HealthCritical},
	}

	for _, tt := range tests {
		drive := DriveStats{
			SerialNumber:  "SN-T-" + string(rune(int(tt.temp)+65)),
			CapacityBytes: 1024,
			TemperatureC:  tt.temp,
		}
		err := c.CollectDrive(drive)
		require.NoError(t, err)
		got, _ := c.GetDrive(drive.SerialNumber)
		assert.Equal(t, tt.expected, got.HealthStatus, "温度 %.1f 应推断为 %s", tt.temp, tt.expected)
	}
}

func TestCollector_GetDrive_NotFound(t *testing.T) {
	c := NewCollector(testLogger())
	_, err := c.GetDrive("NONEXIST")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "磁盘不存在")
}

func TestCollector_GetAllDrives(t *testing.T) {
	c := NewCollector(testLogger())

	d1 := makeTestDrive("SN001", "Model A", 1024, DriveTypeSSD)
	d2 := makeTestDrive("SN002", "Model B", 2048, DriveTypeHDD)

	require.NoError(t, c.CollectDrive(d1))
	require.NoError(t, c.CollectDrive(d2))

	drives := c.GetAllDrives()
	assert.Len(t, drives, 2)
}

func TestCollector_TemperatureTrend(t *testing.T) {
	c := NewCollector(testLogger())

	// 采集多次温度，制造上升趋势
	temps := []float64{30, 31, 32, 33, 34, 35, 36, 37}
	for i, temp := range temps {
		drive := DriveStats{
			SerialNumber:  "SN-TEMP",
			CapacityBytes: 1024,
			TemperatureC:  temp,
		}
		err := c.CollectDrive(drive)
		require.NoError(t, err)

		// 手动添加历史温度读数
		c.mu.Lock()
		c.tempHistory["SN-TEMP"] = append(c.tempHistory["SN-TEMP"], TempReading{
			Timestamp:    time.Now().Add(time.Duration(i) * time.Hour),
			TemperatureC: temp,
		})
		c.mu.Unlock()
	}

	trend, err := c.GetTemperatureTrend("SN-TEMP")
	require.NoError(t, err)
	assert.Equal(t, "SN-TEMP", trend.SerialNumber)
	assert.Equal(t, 30.0, trend.MinTemp)
	assert.Equal(t, 37.0, trend.MaxTemp)
	assert.Greater(t, trend.AvgTemp, 0.0)
	assert.Equal(t, TempTrendRising, trend.Trend)
}

func TestCollector_TemperatureTrend_Stable(t *testing.T) {
	c := NewCollector(testLogger())

	temps := []float64{35, 35, 35, 35, 35, 35}
	for i, temp := range temps {
		drive := DriveStats{
			SerialNumber:  "SN-STABLE",
			CapacityBytes: 1024,
			TemperatureC:  temp,
		}
		require.NoError(t, c.CollectDrive(drive))
		c.mu.Lock()
		c.tempHistory["SN-STABLE"] = append(c.tempHistory["SN-STABLE"], TempReading{
			Timestamp:    time.Now().Add(time.Duration(i) * time.Hour),
			TemperatureC: temp,
		})
		c.mu.Unlock()
	}

	trend, err := c.GetTemperatureTrend("SN-STABLE")
	require.NoError(t, err)
	assert.Equal(t, TempTrendStable, trend.Trend)
}

func TestCollector_TemperatureTrend_NotFound(t *testing.T) {
	c := NewCollector(testLogger())
	_, err := c.GetTemperatureTrend("NONEXIST")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无温度数据")
}

func TestCollector_RegisterTier(t *testing.T) {
	c := NewCollector(testLogger())

	tier := makeTestTier("热数据层", TierTypeNVMe, 1024*1024*1024*1024, 500*1024*1024*1024, 800)
	err := c.RegisterTier(tier)
	require.NoError(t, err)

	tiers := c.GetTiers()
	require.Len(t, tiers, 1)
	assert.Equal(t, "热数据层", tiers[0].Name)
	assert.Equal(t, TierTypeNVMe, tiers[0].Type)
}

func TestCollector_RegisterTier_Invalid(t *testing.T) {
	c := NewCollector(testLogger())

	err := c.RegisterTier(StorageTier{CapacityBytes: 1024})
	assert.Error(t, err)

	err = c.RegisterTier(StorageTier{Name: "Test", CapacityBytes: 0})
	assert.Error(t, err)
}

func TestCollector_RegisterTier_Update(t *testing.T) {
	c := NewCollector(testLogger())

	tier1 := makeTestTier("层1", TierTypeSSD, 1024, 500, 400)
	require.NoError(t, c.RegisterTier(tier1))

	tier2 := makeTestTier("层1", TierTypeSSD, 2048, 1000, 400)
	require.NoError(t, c.RegisterTier(tier2))

	tiers := c.GetTiers()
	require.Len(t, tiers, 1)
	assert.Equal(t, int64(2048), tiers[0].CapacityBytes)
}

func TestCollector_SetTierCosts(t *testing.T) {
	c := NewCollector(testLogger())

	customCosts := map[TierType]float64{
		TierTypeSSD: 500,
		TierTypeHDD: 100,
	}
	c.SetTierCosts(customCosts)

	tier := makeTestTier("SSD层", TierTypeSSD, 1024*1024*1024*1024, 0, 0)
	require.NoError(t, c.RegisterTier(tier))

	report := c.CalculateCostReport()
	require.NotEmpty(t, report.TierCosts)
	assert.Equal(t, 500.0, report.TierCosts[0].CostPerTB)
}

// ========== 成本报告测试 ==========

func TestCollector_CalculateCostReport(t *testing.T) {
	c := NewCollector(testLogger())

	require.NoError(t, c.RegisterTier(makeTestTier("NVMe层", TierTypeNVMe, 2*1024*1024*1024*1024, 1*1024*1024*1024*1024, 800)))
	require.NoError(t, c.RegisterTier(makeTestTier("SSD层", TierTypeSSD, 4*1024*1024*1024*1024, 2*1024*1024*1024*1024, 400)))
	require.NoError(t, c.RegisterTier(makeTestTier("HDD层", TierTypeHDD, 20*1024*1024*1024*1024, 10*1024*1024*1024*1024, 80)))

	report := c.CalculateCostReport()

	assert.False(t, report.GeneratedAt.IsZero())
	assert.Equal(t, 26.0, report.TotalCapacityTB)
	assert.Equal(t, 13.0, report.TotalUsedTB)
	assert.Len(t, report.TierCosts, 3)
	assert.Greater(t, report.TotalMonthlyCost, 0.0)
	assert.Greater(t, report.TotalYearlyCost, report.TotalMonthlyCost)
	assert.Greater(t, report.AvgCostPerTB, 0.0)
}

func TestCollector_CalculateCostReport_Empty(t *testing.T) {
	c := NewCollector(testLogger())
	report := c.CalculateCostReport()

	assert.False(t, report.GeneratedAt.IsZero())
	assert.Empty(t, report.TierCosts)
	assert.Equal(t, 0.0, report.TotalMonthlyCost)
	assert.Equal(t, 0.0, report.AvgCostPerTB)
}

func TestCollector_CalculateCostReport_Savings(t *testing.T) {
	c := NewCollector(testLogger())

	require.NoError(t, c.RegisterTier(makeTestTier("NVMe层", TierTypeNVMe, 4*1024*1024*1024*1024, 1*1024*1024*1024*1024, 800)))
	require.NoError(t, c.RegisterTier(makeTestTier("HDD层", TierTypeHDD, 20*1024*1024*1024*1024, 10*1024*1024*1024*1024, 80)))

	report := c.CalculateCostReport()
	assert.Greater(t, report.PotentialSavings, 0.0)
	assert.Greater(t, report.SavingsPercent, 0.0)
}

func TestBytesToTB(t *testing.T) {
	assert.Equal(t, 1.0, bytesToTB(1024*1024*1024*1024))
	assert.Equal(t, 0.5, bytesToTB(512*1024*1024*1024))
	assert.Equal(t, 0.0, bytesToTB(0))
}

func TestBytesToGB(t *testing.T) {
	assert.Equal(t, 1.0, bytesToGB(1024*1024*1024))
	assert.Equal(t, 0.5, bytesToGB(512*1024*1024))
}

// ========== 文件访问模式采集测试 ==========

func TestCollector_CollectFileAccessPatterns(t *testing.T) {
	dir := t.TempDir()

	oldTime := time.Now().AddDate(0, 0, -100)
	files := []struct {
		name    string
		content string
	}{
		{"hot_file.txt", "hot data"},
		{"warm_file.txt", "warm data"},
		{"cold_file.txt", "cold data"},
	}

	for _, f := range files {
		path := filepath.Join(dir, f.name)
		require.NoError(t, os.WriteFile(path, []byte(f.content), 0644))
		require.NoError(t, os.Chtimes(path, oldTime, oldTime))
	}

	c := NewCollector(testLogger())
	patterns, err := c.CollectFileAccessPatterns(dir, 5)
	require.NoError(t, err)
	assert.Len(t, patterns, 3)

	for _, p := range patterns {
		assert.Equal(t, AccessFreqFrozen, p.AccessFreq)
		assert.Equal(t, TierIDArchive, p.DataTier)
		assert.Greater(t, p.Size, int64(0))
	}
}

func TestCollector_CollectFileAccessPatterns_EmptyPath(t *testing.T) {
	c := NewCollector(testLogger())
	_, err := c.CollectFileAccessPatterns("", 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "路径不能为空")
}

func TestCollector_CollectFileAccessPatterns_NonExistentPath(t *testing.T) {
	c := NewCollector(testLogger())
	_, err := c.CollectFileAccessPatterns("/nonexistent/path/12345", 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "路径不可访问")
}

func TestClassifyAccessFreq(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		access   time.Time
		expected AccessFreq
	}{
		{"3天内访问", now.AddDate(0, 0, -3), AccessFreqHot},
		{"15天前访问", now.AddDate(0, 0, -15), AccessFreqWarm},
		{"60天前访问", now.AddDate(0, 0, -60), AccessFreqCold},
		{"120天前访问", now.AddDate(0, 0, -120), AccessFreqFrozen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyAccessFreq(tt.access, now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecommendTier(t *testing.T) {
	now := time.Now()

	tests := []struct {
		freq     AccessFreq
		expected TierID
	}{
		{AccessFreqHot, TierIDHot},
		{AccessFreqWarm, TierIDWarm},
		{AccessFreqCold, TierIDCold},
		{AccessFreqFrozen, TierIDArchive},
	}

	for _, tt := range tests {
		result := recommendTier(tt.freq, now)
		assert.Equal(t, tt.expected, result)
	}
}

// ========== 分层规则引擎测试 ==========

func TestNewTieringEngine(t *testing.T) {
	engine := NewTieringEngine(nil)
	require.NotNil(t, engine)
	assert.NotEmpty(t, engine.rules)

	rules := engine.ListRules()
	assert.GreaterOrEqual(t, len(rules), 4)
}

func TestTieringEngine_DefaultRules(t *testing.T) {
	engine := NewTieringEngine(testLogger())
	rules := engine.ListRules()

	for i := 0; i < len(rules)-1; i++ {
		assert.LessOrEqual(t, rules[i].Priority, rules[i+1].Priority, "规则应按优先级排序")
	}

	// 验证包含默认规则
	var hasHot, hasWarm, hasCold, hasArchive bool
	for _, r := range rules {
		switch r.TargetTier {
		case TierIDHot:
			hasHot = true
		case TierIDWarm:
			hasWarm = true
		case TierIDCold:
			hasCold = true
		case TierIDArchive:
			hasArchive = true
		}
	}
	assert.True(t, hasHot, "应有热数据规则")
	assert.True(t, hasWarm, "应有温数据规则")
	assert.True(t, hasCold, "应有冷数据规则")
	assert.True(t, hasArchive, "应有归档规则")
}

func TestTieringEngine_AddRule(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	rule := TieringRule{
		ID:       "custom-1",
		Name:     "自定义规则1",
		Enabled:  true,
		Priority: 5,
		Conditions: []RuleCondition{
			{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "365"},
		},
		TargetTier:  TierIDArchive,
		Action:      ActionArchive,
		Description: "超过1年的文件归档",
	}
	err := engine.AddRule(rule)
	require.NoError(t, err)

	got, err := engine.GetRule("custom-1")
	require.NoError(t, err)
	assert.Equal(t, "自定义规则1", got.Name)
}

func TestTieringEngine_AddRule_Duplicate(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	rule := TieringRule{
		ID:       "test-dup",
		Name:     "测试",
		Enabled:  true,
		Priority: 10,
		Conditions: []RuleCondition{
			{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "30"},
		},
		TargetTier: TierIDCold,
		Action:     ActionMigrate,
	}
	require.NoError(t, engine.AddRule(rule))

	err := engine.AddRule(rule)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestTieringEngine_AddRule_Invalid(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	// 空ID
	err := engine.AddRule(TieringRule{Name: "test", Conditions: []RuleCondition{{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "30"}}})
	assert.Error(t, err)

	// 空名称
	err = engine.AddRule(TieringRule{ID: "x", Conditions: []RuleCondition{{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "30"}}})
	assert.Error(t, err)

	// 空条件
	err = engine.AddRule(TieringRule{ID: "x", Name: "x"})
	assert.Error(t, err)

	// 无效条件值
	err = engine.AddRule(TieringRule{
		ID:         "x",
		Name:       "x",
		Conditions: []RuleCondition{{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "abc"}},
	})
	assert.Error(t, err)
}

func TestTieringEngine_RemoveRule(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	rule := TieringRule{
		ID:         "removable",
		Name:       "可删除",
		Enabled:    true,
		Priority:   10,
		Conditions: []RuleCondition{{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "30"}},
		TargetTier: TierIDCold,
		Action:     ActionMigrate,
	}
	require.NoError(t, engine.AddRule(rule))

	err := engine.RemoveRule("removable")
	require.NoError(t, err)

	_, err = engine.GetRule("removable")
	assert.Error(t, err)
}

func TestTieringEngine_RemoveRule_NotFound(t *testing.T) {
	engine := NewTieringEngine(testLogger())
	err := engine.RemoveRule("nonexistent")
	assert.Error(t, err)
}

func TestTieringEngine_UpdateRule(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	rule := TieringRule{
		ID:         "updatable",
		Name:       "原名称",
		Enabled:    true,
		Priority:   10,
		Conditions: []RuleCondition{{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "30"}},
		TargetTier: TierIDCold,
		Action:     ActionMigrate,
	}
	require.NoError(t, engine.AddRule(rule))

	rule.Name = "新名称"
	rule.Priority = 1
	err := engine.UpdateRule(rule)
	require.NoError(t, err)

	got, _ := engine.GetRule("updatable")
	assert.Equal(t, "新名称", got.Name)
	assert.Equal(t, 1, got.Priority)
}

func TestTieringEngine_GetRule_NotFound(t *testing.T) {
	engine := NewTieringEngine(testLogger())
	_, err := engine.GetRule("nonexistent")
	assert.Error(t, err)
}

func TestTieringEngine_CreatePlan(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	plan := TieringPlan{
		ID:         "plan-1",
		Name:       "每日冷数据迁移",
		Enabled:    true,
		SourceTier: TierIDHot,
		TargetTier: TierIDCold,
		Schedule:   "0 2 * * *",
	}
	err := engine.CreatePlan(plan)
	require.NoError(t, err)

	got, err := engine.GetPlan("plan-1")
	require.NoError(t, err)
	assert.Equal(t, "每日冷数据迁移", got.Name)
}

func TestTieringEngine_CreatePlan_Duplicate(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	plan := TieringPlan{ID: "p1", Name: "Plan 1"}
	require.NoError(t, engine.CreatePlan(plan))
	err := engine.CreatePlan(plan)
	assert.Error(t, err)
}

func TestTieringEngine_GetPlan_NotFound(t *testing.T) {
	engine := NewTieringEngine(testLogger())
	_, err := engine.GetPlan("nonexistent")
	assert.Error(t, err)
}

func TestTieringEngine_ListPlans(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	require.NoError(t, engine.CreatePlan(TieringPlan{ID: "p1", Name: "Plan 1"}))
	require.NoError(t, engine.CreatePlan(TieringPlan{ID: "p2", Name: "Plan 2"}))

	plans := engine.ListPlans()
	assert.Len(t, plans, 2)
}

func TestTieringEngine_Evaluate_HotData(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	// 3天前访问的热数据
	pattern := FileAccessPattern{
		Path:       "/data/hot.txt",
		Size:       1024,
		AccessTime: time.Now().AddDate(0, 0, -3),
		ModTime:    time.Now().AddDate(0, 0, -3),
		AccessFreq: AccessFreqHot,
		DataTier:   TierIDHot,
	}

	suggestion, err := engine.Evaluate(pattern)
	require.NoError(t, err)
	assert.Equal(t, TierIDHot, suggestion.TargetTier)
	assert.Equal(t, ActionPin, suggestion.Action)
}

func TestTieringEngine_Evaluate_ColdData(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	// 60天前访问的冷数据
	pattern := FileAccessPattern{
		Path:       "/data/cold.txt",
		Size:       1024,
		AccessTime: time.Now().AddDate(0, 0, -60),
		ModTime:    time.Now().AddDate(0, 0, -60),
		AccessFreq: AccessFreqCold,
		DataTier:   TierIDHot, // 当前在热层
	}

	suggestion, err := engine.Evaluate(pattern)
	require.NoError(t, err)
	assert.Equal(t, TierIDCold, suggestion.TargetTier)
	assert.Equal(t, ActionMigrate, suggestion.Action)
	assert.NotEqual(t, suggestion.CurrentTier, suggestion.TargetTier)
}

func TestTieringEngine_Evaluate_ArchiveData(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	// 200天前访问且修改超过180天
	pattern := FileAccessPattern{
		Path:       "/data/archive.txt",
		Size:       1024,
		AccessTime: time.Now().AddDate(0, 0, -200),
		ModTime:    time.Now().AddDate(0, 0, -200),
		AccessFreq: AccessFreqFrozen,
		DataTier:   TierIDCold,
	}

	suggestion, err := engine.Evaluate(pattern)
	require.NoError(t, err)
	assert.Equal(t, TierIDArchive, suggestion.TargetTier)
	assert.Equal(t, ActionArchive, suggestion.Action)
}

func TestTieringEngine_EvaluateBatch(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	patterns := []FileAccessPattern{
		{
			Path:       "/data/hot.txt",
			Size:       100,
			AccessTime: time.Now().AddDate(0, 0, -1),
			ModTime:    time.Now().AddDate(0, 0, -1),
			AccessFreq: AccessFreqHot,
			DataTier:   TierIDHot,
		},
		{
			Path:       "/data/cold.txt",
			Size:       100,
			AccessTime: time.Now().AddDate(0, 0, -60),
			ModTime:    time.Now().AddDate(0, 0, -60),
			AccessFreq: AccessFreqCold,
			DataTier:   TierIDHot,
		},
		{
			Path:       "/data/archive.txt",
			Size:       100,
			AccessTime: time.Now().AddDate(0, 0, -200),
			ModTime:    time.Now().AddDate(0, 0, -200),
			AccessFreq: AccessFreqFrozen,
			DataTier:   TierIDCold,
		},
	}

	suggestions := engine.EvaluateBatch(patterns)
	assert.Len(t, suggestions, 3)
	assert.Equal(t, TierIDHot, suggestions[0].TargetTier)
	assert.Equal(t, TierIDCold, suggestions[1].TargetTier)
	assert.Equal(t, TierIDArchive, suggestions[2].TargetTier)
}

func TestTieringEngine_GenerateMigrationPlan(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	patterns := []FileAccessPattern{
		{
			Path:       "/data/hot.txt",
			Size:       1024,
			AccessTime: time.Now().AddDate(0, 0, -1),
			ModTime:    time.Now().AddDate(0, 0, -1),
			AccessFreq: AccessFreqHot,
			DataTier:   TierIDHot,
		},
		{
			Path:       "/data/migrate.txt",
			Size:       4096,
			AccessTime: time.Now().AddDate(0, 0, -60),
			ModTime:    time.Now().AddDate(0, 0, -60),
			AccessFreq: AccessFreqCold,
			DataTier:   TierIDHot, // 需要从热层迁移到冷层
		},
		{
			Path:       "/data/archive.txt",
			Size:       8192,
			AccessTime: time.Now().AddDate(0, 0, -200),
			ModTime:    time.Now().AddDate(0, 0, -200),
			AccessFreq: AccessFreqFrozen,
			DataTier:   TierIDCold, // 需要从冷层归档
		},
	}

	plan := engine.GenerateMigrationPlan(patterns)

	assert.Equal(t, 3, plan.Total)
	assert.Equal(t, 2, plan.Pending)  // 2个需要迁移
	assert.Equal(t, 1, plan.NoAction) // hot.txt 不需要
	assert.Greater(t, plan.MigrateSize, int64(0))
	assert.Len(t, plan.Migrations, 2)
}

func TestTieringEngine_IdentifyColdData(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	patterns := []FileAccessPattern{
		{Path: "hot", AccessFreq: AccessFreqHot},
		{Path: "warm", AccessFreq: AccessFreqWarm},
		{Path: "cold1", AccessFreq: AccessFreqCold},
		{Path: "frozen1", AccessFreq: AccessFreqFrozen},
		{Path: "cold2", AccessFreq: AccessFreqCold},
	}

	cold := engine.IdentifyColdData(patterns)
	assert.Len(t, cold, 3) // 2个cold + 1个frozen
}

func TestTieringEngine_IdentifyHotData(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	patterns := []FileAccessPattern{
		{Path: "hot1", AccessFreq: AccessFreqHot},
		{Path: "warm", AccessFreq: AccessFreqWarm},
		{Path: "hot2", AccessFreq: AccessFreqHot},
		{Path: "cold", AccessFreq: AccessFreqCold},
	}

	hot := engine.IdentifyHotData(patterns)
	assert.Len(t, hot, 2)
}

func TestTieringEngine_GetDataDistribution(t *testing.T) {
	engine := NewTieringEngine(testLogger())

	patterns := []FileAccessPattern{
		{AccessFreq: AccessFreqHot},
		{AccessFreq: AccessFreqHot},
		{AccessFreq: AccessFreqWarm},
		{AccessFreq: AccessFreqCold},
		{AccessFreq: AccessFreqFrozen},
		{AccessFreq: AccessFreqFrozen},
	}

	dist := engine.GetDataDistribution(patterns)
	assert.Equal(t, 2, dist[AccessFreqHot])
	assert.Equal(t, 1, dist[AccessFreqWarm])
	assert.Equal(t, 1, dist[AccessFreqCold])
	assert.Equal(t, 2, dist[AccessFreqFrozen])
}

// ========== 仪表盘测试 ==========

func TestNewDashboard(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	require.NotNil(t, d)
	assert.NotNil(t, d.alertThresholds)
	assert.Equal(t, 80.0, d.alertThresholds.WarningPercent)
	assert.Equal(t, 90.0, d.alertThresholds.CriticalPercent)
}

func TestDashboard_SetThresholds(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	d.SetThresholds(AlertThresholds{
		WarningPercent:  70,
		CriticalPercent: 85,
		MaxTempC:        60,
		MaxPowerOnHours: 30000,
	})

	assert.Equal(t, 70.0, d.alertThresholds.WarningPercent)
	assert.Equal(t, 85.0, d.alertThresholds.CriticalPercent)
}

func TestDashboard_Generate_Empty(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	data := d.Generate()

	require.NotNil(t, data)
	assert.False(t, data.GeneratedAt.IsZero())
	assert.Equal(t, int64(0), data.TotalCapacity)
	assert.Equal(t, int64(0), data.TotalUsed)
	assert.Equal(t, 0.0, data.UsagePercent)
	assert.Empty(t, data.Drives)
	assert.Empty(t, data.Tiers)
	assert.NotNil(t, data.HealthSummary)
	assert.Equal(t, HealthUnknown, data.HealthSummary.Overall)
}

func TestDashboard_Generate_WithDrives(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	// 添加磁盘
	require.NoError(t, c.CollectDrive(makeTestDrive("SN001", "SSD-1", 1024*1024*1024*1024, DriveTypeSSD)))
	require.NoError(t, c.CollectDrive(makeTestDrive("SN002", "HDD-1", 4*1024*1024*1024*1024, DriveTypeHDD)))

	// 添加存储层
	require.NoError(t, c.RegisterTier(makeTestTier("SSD层", TierTypeSSD, 1024*1024*1024*1024, 800*1024*1024*1024, 400)))
	require.NoError(t, c.RegisterTier(makeTestTier("HDD层", TierTypeHDD, 4*1024*1024*1024*1024, 2*1024*1024*1024*1024, 80)))

	data := d.Generate()

	require.NotNil(t, data)
	assert.Len(t, data.Drives, 2)
	assert.Len(t, data.Tiers, 2)
	assert.Greater(t, data.TotalCapacity, int64(0))
	assert.Greater(t, data.TotalUsed, int64(0))
	assert.Greater(t, data.UsagePercent, 0.0)
	assert.NotNil(t, data.CostReport)
	assert.Equal(t, 2, data.HealthSummary.TotalDrives)
	assert.Equal(t, 2, data.HealthSummary.HealthyDrives)
	assert.Equal(t, HealthGood, data.HealthSummary.Overall)
}

func TestDashboard_CapacityAlerts(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	d.SetThresholds(AlertThresholds{
		WarningPercent:  70,
		CriticalPercent: 85,
		MaxTempC:        65,
		MaxPowerOnHours: 43800,
	})

	// 高使用率磁盘
	highUsageDrive := DriveStats{
		SerialNumber:  "SN-HIGH",
		Model:         "Full Drive",
		CapacityBytes: 1000,
		UsedBytes:     900, // 90% 使用率
		HealthStatus:  HealthGood,
		TemperatureC:  40.0,
	}
	require.NoError(t, c.CollectDrive(highUsageDrive))

	// 中等使用率磁盘
	mediumDrive := DriveStats{
		SerialNumber:  "SN-MED",
		Model:         "Half Drive",
		CapacityBytes: 1000,
		UsedBytes:     750, // 75% 使用率
		HealthStatus:  HealthGood,
		TemperatureC:  35.0,
	}
	require.NoError(t, c.CollectDrive(mediumDrive))

	data := d.Generate()

	// 应有2个预警：1个critical + 1个warning
	assert.NotEmpty(t, data.CapacityAlerts)
	var criticalCount, warningCount int
	for _, alert := range data.CapacityAlerts {
		switch alert.Level {
		case AlertCritical:
			criticalCount++
		case AlertWarning:
			warningCount++
		}
	}
	assert.GreaterOrEqual(t, criticalCount, 1)
	assert.GreaterOrEqual(t, warningCount, 1)
}

func TestDashboard_HealthSummary_Warning(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	// 高温磁盘
	hotDrive := DriveStats{
		SerialNumber:  "SN-HOT",
		Model:         "Hot Drive",
		CapacityBytes: 1024,
		UsedBytes:     512,
		HealthStatus:  HealthWarning,
		TemperatureC:  68.0,
	}
	require.NoError(t, c.CollectDrive(hotDrive))

	data := d.Generate()

	assert.Equal(t, 1, data.HealthSummary.TotalDrives)
	assert.Equal(t, 0, data.HealthSummary.HealthyDrives)
	assert.Equal(t, 1, data.HealthSummary.WarningDrives)
	assert.Equal(t, HealthWarning, data.HealthSummary.Overall)
	assert.Equal(t, 68.0, data.HealthSummary.MaxTempC)
	assert.NotEmpty(t, data.HealthSummary.Details)
}

func TestDashboard_HealthSummary_Critical(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	criticalDrive := DriveStats{
		SerialNumber:  "SN-CRIT",
		Model:         "Critical Drive",
		CapacityBytes: 1024,
		UsedBytes:     512,
		HealthStatus:  HealthCritical,
		TemperatureC:  70.0,
	}
	require.NoError(t, c.CollectDrive(criticalDrive))

	data := d.Generate()

	assert.Equal(t, 1, data.HealthSummary.CriticalDrives)
	assert.Equal(t, HealthCritical, data.HealthSummary.Overall)
}

func TestDashboard_HealthSummary_PowerOnHours(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	d.SetThresholds(AlertThresholds{
		WarningPercent:  80,
		CriticalPercent: 90,
		MaxTempC:        65,
		MaxPowerOnHours: 100, // 很低的阈值
	})

	oldDrive := DriveStats{
		SerialNumber:  "SN-OLD",
		Model:         "Old Drive",
		CapacityBytes: 1024,
		UsedBytes:     512,
		HealthStatus:  HealthGood,
		TemperatureC:  35.0,
		PowerOnHours:  200, // 超过阈值
	}
	require.NoError(t, c.CollectDrive(oldDrive))

	data := d.Generate()

	// 应被降级为 warning
	assert.Equal(t, 0, data.HealthSummary.HealthyDrives)
	assert.Equal(t, 1, data.HealthSummary.WarningDrives)
}

func TestDashboard_CapacityForecast(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	require.NoError(t, c.CollectDrive(makeTestDrive("SN001", "Test", 1024*1024*1024*1024, DriveTypeHDD)))

	// 日增长 10GB
	forecast := d.GetCapacityForecast(500*1024*1024*1024, 10*1024*1024*1024)

	require.NotNil(t, forecast)
	assert.Greater(t, forecast.DaysRemaining, 0)
	assert.NotZero(t, forecast.EstimatedDate)
	assert.NotEmpty(t, forecast.Status)
	assert.NotEmpty(t, forecast.Message)
}

func TestDashboard_CapacityForecast_ZeroGrowth(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	forecast := d.GetCapacityForecast(1024, 0)

	require.NotNil(t, forecast)
	assert.Equal(t, 0, forecast.DaysRemaining)
	assert.Equal(t, "稳定", forecast.Status)
}

func TestDashboard_CapacityForecast_Urgent(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	require.NoError(t, c.CollectDrive(makeTestDrive("SN001", "Small", 100*1024*1024, DriveTypeSSD)))

	// 日增长 5GB，总容量 100MB，很快满盘
	forecast := d.GetCapacityForecast(90*1024*1024, 5*1024*1024*1024)

	require.NotNil(t, forecast)
	assert.Equal(t, "紧急", forecast.Status)
}

func TestDashboard_DefaultAlertThresholds(t *testing.T) {
	thresholds := DefaultAlertThresholds()
	assert.Equal(t, 80.0, thresholds.WarningPercent)
	assert.Equal(t, 90.0, thresholds.CriticalPercent)
	assert.Equal(t, 65.0, thresholds.MaxTempC)
	assert.Equal(t, int64(43800), thresholds.MaxPowerOnHours)
}

func TestDashboard_Generate_WithTieringPlans(t *testing.T) {
	c := NewCollector(testLogger())
	e := NewTieringEngine(testLogger())
	d := NewDashboard(c, e, nil)

	require.NoError(t, e.CreatePlan(TieringPlan{
		ID:         "plan-1",
		Name:       "冷数据迁移计划",
		Enabled:    true,
		SourceTier: TierIDHot,
		TargetTier: TierIDCold,
		Schedule:   "0 2 * * *",
	}))

	data := d.Generate()

	require.NotNil(t, data)
	assert.Len(t, data.TieringPlans, 1)
	assert.Equal(t, "冷数据迁移计划", data.TieringPlans[0].Name)
}

// ========== 工具函数测试 ==========

func TestInferHealthFromTemp(t *testing.T) {
	tests := []struct {
		temp     float64
		expected HealthStatus
	}{
		{0, HealthUnknown},
		{30, HealthGood},
		{49, HealthGood},
		{50, HealthWarning},
		{64, HealthWarning},
		{65, HealthCritical},
		{80, HealthCritical},
	}

	for _, tt := range tests {
		result := inferHealthFromTemp(tt.temp)
		assert.Equal(t, tt.expected, result, "温度 %.1f", tt.temp)
	}
}

func TestInferInterface(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"nvme0n1", "NVMe"},
		{"sda", "SATA/SAS"},
		{"mmcblk0", "MMC"},
		{"usb1", "USB"},
		{"unknown0", "Unknown"},
	}

	for _, tt := range tests {
		result := inferInterface(tt.name)
		assert.Equal(t, tt.expected, result, tt.name)
	}
}

func TestCalculateTempTrend(t *testing.T) {
	// 上升趋势
	rising := []TempReading{
		{TemperatureC: 30},
		{TemperatureC: 31},
		{TemperatureC: 35},
		{TemperatureC: 40},
	}
	assert.Equal(t, TempTrendRising, calculateTempTrend(rising))

	// 下降趋势
	falling := []TempReading{
		{TemperatureC: 40},
		{TemperatureC: 39},
		{TemperatureC: 35},
		{TemperatureC: 30},
	}
	assert.Equal(t, TempTrendFalling, calculateTempTrend(falling))

	// 平稳
	stable := []TempReading{
		{TemperatureC: 35},
		{TemperatureC: 35},
		{TemperatureC: 35},
		{TemperatureC: 35},
	}
	assert.Equal(t, TempTrendStable, calculateTempTrend(stable))

	// 数据不足
	short := []TempReading{{TemperatureC: 35}}
	assert.Equal(t, TempTrendStable, calculateTempTrend(short))
}

func TestCompareNumeric(t *testing.T) {
	tests := []struct {
		actual   float64
		op       RuleOperator
		value    string
		expected bool
		hasErr   bool
	}{
		{10, OpGreaterThan, "5", true, false},
		{5, OpGreaterThan, "10", false, false},
		{5, OpLessThan, "10", true, false},
		{5, OpEqual, "5", true, false},
		{5, OpGreaterEqual, "5", true, false},
		{5, OpLessEqual, "5", true, false},
		{5, OpGreaterThan, "abc", false, true},
		{5, OpContains, "5", false, true},
	}

	for _, tt := range tests {
		result, err := compareNumeric(tt.actual, tt.op, tt.value)
		if tt.hasErr {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		}
	}
}

func TestCompareString(t *testing.T) {
	// contains
	result, err := compareString("/data/video/test.mp4", OpContains, "video")
	require.NoError(t, err)
	assert.True(t, result)

	// case insensitive contains
	result, err = compareString("/DATA/VIDEO/test.mp4", OpContains, "video")
	require.NoError(t, err)
	assert.True(t, result)

	// equal
	result, err = compareString("mp4", OpEqual, "MP4")
	require.NoError(t, err)
	assert.True(t, result)

	// unsupported operator
	_, err = compareString("test", OpGreaterThan, "x")
	assert.Error(t, err)
}

func TestValidateCondition(t *testing.T) {
	// 有效数值条件
	err := validateCondition(RuleCondition{Field: RuleFieldAge, Operator: OpGreaterThan, Value: "30"})
	assert.NoError(t, err)

	// 无效：数值字段用 contains
	err = validateCondition(RuleCondition{Field: RuleFieldAge, Operator: OpContains, Value: "30"})
	assert.Error(t, err)

	// 无效：空值
	err = validateCondition(RuleCondition{Field: RuleFieldAge, Operator: OpGreaterThan, Value: ""})
	assert.Error(t, err)

	// 无效：非数字值
	err = validateCondition(RuleCondition{Field: RuleFieldSize, Operator: OpGreaterThan, Value: "abc"})
	assert.Error(t, err)

	// 有效文件类型
	err = validateCondition(RuleCondition{Field: RuleFileType, Operator: OpContains, Value: "video"})
	assert.NoError(t, err)

	// 无效：文件类型用数值操作符
	err = validateCondition(RuleCondition{Field: RuleFileType, Operator: OpGreaterThan, Value: "video"})
	assert.Error(t, err)

	// 无效：未知字段
	err = validateCondition(RuleCondition{Field: RuleField("unknown"), Operator: OpGreaterThan, Value: "30"})
	assert.Error(t, err)
}
