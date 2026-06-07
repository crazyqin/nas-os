package smartdatatiering

import (
	"fmt"
	"testing"
	"time"
)

func TestNewSmartDataTiering(t *testing.T) {
	sdt := NewSmartDataTiering(nil)
	if sdt == nil {
		t.Fatal("Expected SmartDataTiering instance, got nil")
	}

	// 验证默认配置
	if sdt.config.AutoTieringEnabled != true {
		t.Error("Expected AutoTieringEnabled to be true")
	}

	if sdt.config.MonitorIntervalMin != 30 {
		t.Errorf("Expected MonitorIntervalMin 30, got %d", sdt.config.MonitorIntervalMin)
	}

	// 验证默认层级配置
	if len(sdt.tierConfigs) != 4 {
		t.Errorf("Expected 4 tier configs, got %d", len(sdt.tierConfigs))
	}
}

func TestAddDataItem(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	item := &DataItem{
		ID:           "test_item_1",
		Path:         "/data/test.txt",
		Size:         1024 * 1024, // 1MB
		AccessCount:  100,
		LastAccessed: time.Now(),
		LastModified: time.Now(),
		CreatedAt:    time.Now().Add(-24 * time.Hour),
	}

	err := sdt.AddDataItem(item)
	if err != nil {
		t.Fatalf("Failed to add data item: %v", err)
	}

	// 验证项目已添加
	if _, exists := sdt.dataItems["test_item_1"]; !exists {
		t.Error("Data item not found after adding")
	}

	// 验证温度计算
	if item.Temperature != TempHot {
		t.Errorf("Expected temperature HOT, got %s", item.Temperature)
	}

	// 验证层级分配
	if item.CurrentTier != TierHot {
		t.Errorf("Expected tier HOT, got %s", item.CurrentTier)
	}
}

func TestAddDataItemWithEmptyID(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	item := &DataItem{
		Path: "/data/test.txt",
		Size: 1024,
	}

	err := sdt.AddDataItem(item)
	if err == nil {
		t.Error("Expected error for empty ID, got nil")
	}
}

func TestTemperatureEvaluation(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	tests := []struct {
		name         string
		accessCount  int64
		lastAccessed time.Duration
		expectedTemp DataTemperature
	}{
		{"Hot - frequent access", 100, 1 * time.Hour, TempHot},
		{"Warm - moderate access", 10, 3 * 24 * time.Hour, TempWarm},
		{"Cold - rare access", 1, 15 * 24 * time.Hour, TempCold},
		{"Frozen - very rare", 0, 45 * 24 * time.Hour, TempFrozen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &DataItem{
				ID:           "temp_test",
				AccessCount:  tt.accessCount,
				LastAccessed: time.Now().Add(-tt.lastAccessed),
				CreatedAt:    time.Now().Add(-90 * 24 * time.Hour),
			}
			item.AccessFrequency = sdt.calculateAccessFrequency(item)
			temp := sdt.evaluateTemperature(item)

			if temp != tt.expectedTemp {
				t.Errorf("Expected temperature %s, got %s", tt.expectedTemp, temp)
			}
		})
	}
}

func TestTierRecommendation(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	tests := []struct {
		name         string
		temperature  DataTemperature
		expectedTier StorageTier
	}{
		{"Hot data -> Hot tier", TempHot, TierHot},
		{"Warm data -> Warm tier", TempWarm, TierWarm},
		{"Cold data -> Cold tier", TempCold, TierCold},
		{"Frozen data -> Archive tier", TempFrozen, TierArchive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &DataItem{
				Temperature: tt.temperature,
			}
			tier := sdt.recommendTier(item)

			if tier != tt.expectedTier {
				t.Errorf("Expected tier %s, got %s", tt.expectedTier, tier)
			}
		})
	}
}

func TestCreatePolicy(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	policy := &TieringPolicy{
		ID:          "test_policy",
		Name:        "Test Policy",
		Description: "Test tiering policy",
		Enabled:     true,
		TierTransitions: []TierTransition{
			{FromTier: TierHot, ToTier: TierWarm, Temperature: TempWarm, DaysInactive: 7},
		},
		MonitorIntervalMin:  15,
		AutoMigrate:         true,
		MaxMigrationsPerDay: 50,
	}

	err := sdt.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if _, exists := sdt.policies["test_policy"]; !exists {
		t.Error("Policy not found after creation")
	}
}

func TestCreatePolicyWithEmptyID(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	policy := &TieringPolicy{
		Name: "Test Policy",
	}

	err := sdt.CreatePolicy(policy)
	if err == nil {
		t.Error("Expected error for empty ID, got nil")
	}
}

func TestAnalyzeAndMigrate(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	// 添加策略
	policy := GenerateDefaultPolicy()
	sdt.CreatePolicy(policy)
	sdt.config.DefaultPolicyID = policy.ID

	// 添加一个冷数据项（应该在热存储）
	coldItem := &DataItem{
		ID:           "cold_item",
		Path:         "/data/old_file.txt",
		Size:         1024 * 1024 * 100, // 100MB
		CurrentTier:  TierHot,
		AccessCount:  1,
		LastAccessed: time.Now().Add(-30 * 24 * time.Hour), // 30天前
		CreatedAt:    time.Now().Add(-90 * 24 * time.Hour),
	}
	sdt.AddDataItem(coldItem)

	// 分析并迁移
	jobs, err := sdt.AnalyzeAndMigrate()
	if err != nil {
		t.Fatalf("Failed to analyze and migrate: %v", err)
	}

	// 验证生成了迁移任务
	if len(jobs) == 0 {
		t.Error("Expected migration jobs, got none")
	}

	// 验证数据项已迁移
	updatedItem := sdt.dataItems["cold_item"]
	if updatedItem.CurrentTier == TierHot {
		t.Error("Expected item to be migrated from HOT tier")
	}
}

func TestGetOptimizationReport(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	// 添加一些测试数据
	items := []*DataItem{
		{ID: "item1", Size: 1024 * 1024, CurrentTier: TierHot, Temperature: TempHot, CreatedAt: time.Now()},
		{ID: "item2", Size: 2 * 1024 * 1024, CurrentTier: TierWarm, Temperature: TempWarm, CreatedAt: time.Now()},
		{ID: "item3", Size: 3 * 1024 * 1024, CurrentTier: TierCold, Temperature: TempCold, CreatedAt: time.Now()},
	}

	for _, item := range items {
		sdt.AddDataItem(item)
	}

	report := sdt.GetOptimizationReport()

	if report.TotalItems != 3 {
		t.Errorf("Expected 3 total items, got %d", report.TotalItems)
	}

	if report.TierDistribution[TierHot] != 1 {
		t.Errorf("Expected 1 hot item, got %d", report.TierDistribution[TierHot])
	}

	if report.TierDistribution[TierWarm] != 1 {
		t.Errorf("Expected 1 warm item, got %d", report.TierDistribution[TierWarm])
	}

	if report.TierDistribution[TierCold] != 1 {
		t.Errorf("Expected 1 cold item, got %d", report.TierDistribution[TierCold])
	}
}

func TestGetDataItemsByTier(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	// 添加测试数据
	sdt.AddDataItem(&DataItem{ID: "hot1", Size: 100, CurrentTier: TierHot, AccessFrequency: 20, CreatedAt: time.Now()})
	sdt.AddDataItem(&DataItem{ID: "hot2", Size: 200, CurrentTier: TierHot, AccessFrequency: 10, CreatedAt: time.Now()})
	sdt.AddDataItem(&DataItem{ID: "cold1", Size: 300, CurrentTier: TierCold, AccessFrequency: 0.5, CreatedAt: time.Now()})

	hotItems := sdt.GetDataItemsByTier(TierHot)
	if len(hotItems) != 2 {
		t.Errorf("Expected 2 hot items, got %d", len(hotItems))
	}

	// 验证按访问频率排序
	if hotItems[0].AccessFrequency < hotItems[1].AccessFrequency {
		t.Error("Expected hot items to be sorted by access frequency (descending)")
	}

	coldItems := sdt.GetDataItemsByTier(TierCold)
	if len(coldItems) != 1 {
		t.Errorf("Expected 1 cold item, got %d", len(coldItems))
	}
}

func TestGetHotDataItems(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	sdt.AddDataItem(&DataItem{ID: "hot1", Size: 100, CurrentTier: TierHot, AccessFrequency: 20, CreatedAt: time.Now()})
	sdt.AddDataItem(&DataItem{ID: "warm1", Size: 200, CurrentTier: TierWarm, AccessFrequency: 5, CreatedAt: time.Now()})

	hotItems := sdt.GetHotDataItems()
	if len(hotItems) != 1 {
		t.Errorf("Expected 1 hot item, got %d", len(hotItems))
	}
}

func TestEstimateMigrationCost(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	item := &DataItem{
		ID:          "cost_test",
		Size:        1024 * 1024 * 1024, // 1GB
		CurrentTier: TierHot,
		CreatedAt:   time.Now(),
	}
	sdt.AddDataItem(item)

	cost, err := sdt.EstimateMigrationCost("cost_test", TierCold)
	if err != nil {
		t.Fatalf("Failed to estimate migration cost: %v", err)
	}

	// 成本应该是一个数值（可能为负，因为冷存储更便宜）
	if cost == 0 {
		t.Error("Expected non-zero cost")
	}
}

func TestEstimateMigrationCostNotFound(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	_, err := sdt.EstimateMigrationCost("nonexistent", TierCold)
	if err == nil {
		t.Error("Expected error for nonexistent item, got nil")
	}
}

func TestGenerateDefaultPolicy(t *testing.T) {
	policy := GenerateDefaultPolicy()

	if policy.ID != "default_tiering" {
		t.Errorf("Expected policy ID 'default_tiering', got %s", policy.ID)
	}

	if !policy.Enabled {
		t.Error("Expected default policy to be enabled")
	}

	if len(policy.TierTransitions) != 6 {
		t.Errorf("Expected 6 tier transitions, got %d", len(policy.TierTransitions))
	}

	if !policy.AutoMigrate {
		t.Error("Expected AutoMigrate to be true")
	}
}

func TestMarshalJSON(t *testing.T) {
	sdt := NewSmartDataTiering(nil)

	sdt.AddDataItem(&DataItem{
		ID:          "json_test",
		Size:        1024,
		CurrentTier: TierHot,
		Temperature: TempHot,
		CreatedAt:   time.Now(),
	})

	data, err := sdt.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON data")
	}
}

func BenchmarkAddDataItem(b *testing.B) {
	sdt := NewSmartDataTiering(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := &DataItem{
			ID:           fmt.Sprintf("bench_item_%d", i),
			Size:         1024,
			AccessCount:  int64(i),
			LastAccessed: time.Now(),
			CreatedAt:    time.Now(),
		}
		sdt.AddDataItem(item)
	}
}

func BenchmarkAnalyzeAndMigrate(b *testing.B) {
	sdt := NewSmartDataTiering(nil)
	policy := GenerateDefaultPolicy()
	sdt.CreatePolicy(policy)
	sdt.config.DefaultPolicyID = policy.ID

	// 添加测试数据
	for i := 0; i < 1000; i++ {
		item := &DataItem{
			ID:           fmt.Sprintf("bench_migrate_%d", i),
			Size:         1024 * 1024,
			CurrentTier:  TierHot,
			AccessCount:  int64(i % 10),
			LastAccessed: time.Now().Add(-time.Duration(i*24) * time.Hour),
			CreatedAt:    time.Now().Add(-90 * 24 * time.Hour),
		}
		sdt.AddDataItem(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sdt.AnalyzeAndMigrate()
	}
}
