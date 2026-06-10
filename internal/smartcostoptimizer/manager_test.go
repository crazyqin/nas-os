package smartcostoptimizer

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestNewManager 测试管理器创建
func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultSmartCostConfig()

	manager := NewManager(logger, cfg)

	if manager == nil {
		t.Fatal("NewManager should not return nil")
	}
	if manager.logger != logger {
		t.Error("Manager.logger should match provided logger")
	}
	if manager.config != cfg {
		t.Error("Manager.config should match provided config")
	}
	if manager.assets == nil {
		t.Error("Manager.assets should be initialized")
	}
	if manager.entries == nil {
		t.Error("Manager.entries should be initialized")
	}
	if manager.reports == nil {
		t.Error("Manager.reports should be initialized")
	}
}

// TestNewManager_NilLogger 测试管理器创建时 logger 为 nil
func TestNewManager_NilLogger(t *testing.T) {
	manager := NewManager(nil, nil)

	if manager == nil {
		t.Fatal("NewManager should not return nil even with nil logger")
	}
	if manager.logger == nil {
		t.Error("Manager.logger should be a no-op logger, not nil")
	}
	if manager.config == nil {
		t.Error("Manager.config should be default config, not nil")
	}
}

// TestManager_GetConfig 测试获取配置（返回副本）
func TestManager_GetConfig(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	cfg1 := manager.GetConfig()
	cfg2 := manager.GetConfig()

	if cfg1 == cfg2 {
		t.Error("GetConfig should return different copies each time")
	}
	if cfg1.DefaultCurrency != cfg2.DefaultCurrency {
		t.Error("Copied configs should have same values")
	}
}

// TestManager_UpdateConfig 测试更新配置
func TestManager_UpdateConfig(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	newCfg := &SmartCostConfig{
		Enabled:            false,
		DefaultCurrency:    "USD",
		ColdThresholdDays:  60,
		UtilizationWarnPct: 30.0,
	}

	manager.UpdateConfig(newCfg)

	got := manager.GetConfig()
	if got.DefaultCurrency != "USD" {
		t.Errorf("After update, DefaultCurrency = %v, want USD", got.DefaultCurrency)
	}
	if got.ColdThresholdDays != 60 {
		t.Errorf("After update, ColdThresholdDays = %v, want 60", got.ColdThresholdDays)
	}
}

// TestManager_UpdateConfig_Nil 测试更新配置为 nil
func TestManager_UpdateConfig_Nil(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	originalCurrency := manager.GetConfig().DefaultCurrency
	manager.UpdateConfig(nil)

	got := manager.GetConfig()
	if got.DefaultCurrency != originalCurrency {
		t.Error("UpdateConfig(nil) should not change config")
	}
}

// TestManager_AddAsset 测试添加资产
func TestManager_AddAsset(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	asset := &StorageAsset{
		ID:            "asset-001",
		Name:          "Test SSD",
		Type:          StorageTypeSSD,
		CapacityBytes: 1024 * 1024 * 1024 * 500,
		UsedBytes:     1024 * 1024 * 1024 * 300,
	}

	err := manager.AddAsset(asset)
	if err != nil {
		t.Fatalf("AddAsset failed: %v", err)
	}

	got, err := manager.GetAsset("asset-001")
	if err != nil {
		t.Fatalf("GetAsset failed: %v", err)
	}
	if got.Name != "Test SSD" {
		t.Errorf("Asset.Name = %v, want Test SSD", got.Name)
	}
}

// TestManager_AddAsset_Nil 测试添加 nil 资产
func TestManager_AddAsset_Nil(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	err := manager.AddAsset(nil)
	if err == nil {
		t.Error("AddAsset(nil) should return error")
	}
}

// TestManager_AddAsset_EmptyID 测试添加空 ID 资产
func TestManager_AddAsset_EmptyID(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	asset := &StorageAsset{
		Name: "No ID",
		Type: StorageTypeSSD,
	}

	err := manager.AddAsset(asset)
	if err == nil {
		t.Error("AddAsset with empty ID should return error")
	}
}

// TestManager_GetAsset_NotFound 测试获取不存在的资产
func TestManager_GetAsset_NotFound(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	_, err := manager.GetAsset("nonexistent")
	if err == nil {
		t.Error("GetAsset for nonexistent ID should return error")
	}
}

// TestManager_ListAssets 测试列出资产
func TestManager_ListAssets(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	assets := []*StorageAsset{
		{ID: "a1", Name: "Asset 1", Type: StorageTypeSSD},
		{ID: "a2", Name: "Asset 2", Type: StorageTypeHDD},
		{ID: "a3", Name: "Asset 3", Type: StorageTypeNVMe},
	}

	for _, a := range assets {
		manager.AddAsset(a)
	}

	list := manager.ListAssets()
	if len(list) != 3 {
		t.Errorf("ListAssets count = %v, want 3", len(list))
	}
}

// TestManager_RemoveAsset 测试删除资产
func TestManager_RemoveAsset(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	asset := &StorageAsset{ID: "to-remove", Name: "Remove Me", Type: StorageTypeHDD}
	manager.AddAsset(asset)

	err := manager.RemoveAsset("to-remove")
	if err != nil {
		t.Fatalf("RemoveAsset failed: %v", err)
	}

	_, err = manager.GetAsset("to-remove")
	if err == nil {
		t.Error("Asset should be removed")
	}
}

// TestManager_RemoveAsset_NotFound 测试删除不存在的资产
func TestManager_RemoveAsset_NotFound(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	err := manager.RemoveAsset("nonexistent")
	if err == nil {
		t.Error("RemoveAsset for nonexistent ID should return error")
	}
}

// TestManager_RecordCost 测试记录成本
func TestManager_RecordCost(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	entry := &CostEntry{
		AssetID:     "asset-001",
		AssetName:   "Test SSD",
		StorageType: StorageTypeSSD,
		CapacityGB:  500,
		UsedGB:      300,
		PricePerGB:  0.50,
		TotalCost:   150.0,
	}

	err := manager.RecordCost(entry)
	if err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}

	if entry.ID == "" {
		t.Error("RecordCost should assign an ID")
	}
	if entry.RecordedAt.IsZero() {
		t.Error("RecordCost should set RecordedAt")
	}

	entries := manager.ListCostEntries()
	if len(entries) != 1 {
		t.Errorf("ListCostEntries count = %v, want 1", len(entries))
	}
}

// TestManager_RecordCost_Nil 测试记录 nil 成本
func TestManager_RecordCost_Nil(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	err := manager.RecordCost(nil)
	if err == nil {
		t.Error("RecordCost(nil) should return error")
	}
}

// TestManager_RecordCost_EmptyAssetID 测试记录空 AssetID 成本
func TestManager_RecordCost_EmptyAssetID(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	entry := &CostEntry{
		AssetName: "No Asset ID",
	}

	err := manager.RecordCost(entry)
	if err == nil {
		t.Error("RecordCost with empty AssetID should return error")
	}
}

// TestManager_RecordCost_AutoCalc 测试自动计算总成本
func TestManager_RecordCost_AutoCalc(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	entry := &CostEntry{
		AssetID:    "asset-001",
		CapacityGB: 1000,
		UsedGB:     500,
		PricePerGB: 0.30,
	}

	err := manager.RecordCost(entry)
	if err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}

	// 500 * 0.30 = 150.0
	if entry.TotalCost != 150.0 {
		t.Errorf("Auto-calculated TotalCost = %v, want 150.0", entry.TotalCost)
	}
}

// TestManager_GetCostSummary 测试获取成本汇总
func TestManager_GetCostSummary(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	// 添加资产
	manager.AddAsset(&StorageAsset{
		ID:            "a1",
		Name:          "SSD Pool",
		Type:          StorageTypeSSD,
		CapacityBytes: 1024 * 1024 * 1024 * 1000,
		UsedBytes:     1024 * 1024 * 1024 * 600,
	})

	now := time.Now()
	summary := manager.GetCostSummary(now.AddDate(0, -1, 0), now)

	if summary == nil {
		t.Fatal("GetCostSummary should not return nil")
	}
	if summary.Currency != "CNY" {
		t.Errorf("Summary.Currency = %v, want CNY", summary.Currency)
	}
}

// TestManager_GenerateOptimizations 测试生成优化建议
func TestManager_GenerateOptimizations(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	// 添加一些资产
	manager.AddAsset(&StorageAsset{
		ID:            "a1",
		Name:          "Old SSD",
		Type:          StorageTypeSSD,
		CapacityBytes: 1024 * 1024 * 1024 * 500,
		UsedBytes:     1024 * 1024 * 1024 * 100,
		PurchaseDate:  time.Now().AddDate(-2, 0, 0), // 2 年前
	})

	suggestions := manager.GenerateOptimizations()
	if suggestions == nil {
		t.Fatal("GenerateOptimizations should not return nil")
	}
	// 至少应该有压缩、清理等基础建议
	if len(suggestions) == 0 {
		t.Error("Should generate at least one optimization suggestion")
	}
}

// TestManager_DetectColdData 测试检测冷数据
func TestManager_DetectColdData(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	// 添加很久之前的资产
	manager.AddAsset(&StorageAsset{
		ID:            "cold-1",
		Name:          "Ancient Data",
		Type:          StorageTypeHDD,
		CapacityBytes: 1024 * 1024 * 1024 * 2000,
		UsedBytes:     1024 * 1024 * 1024 * 1500,
		PurchaseDate:  time.Now().AddDate(-2, 0, 0), // 2 年前
	})

	coldData := manager.DetectColdData()
	if coldData == nil {
		t.Fatal("DetectColdData should not return nil")
	}
}

// TestManager_CalculateROI 测试 ROI 计算
func TestManager_CalculateROI(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	input := &ROIInput{
		InvestmentCost: 10000.0,
		AnnualSaving:   5000.0,
		AnnualOpex:     1000.0,
		ProjectYears:   3,
		DiscountRate:   0.08,
	}

	result, err := manager.CalculateROI(input)
	if err != nil {
		t.Fatalf("CalculateROI failed: %v", err)
	}
	if result == nil {
		t.Fatal("CalculateROI should not return nil result")
	}
	if result.ROIPercent == 0 {
		t.Error("ROIPercent should not be 0 for positive savings")
	}
}

// TestManager_CalculateROI_Nil 测试 ROI 输入为 nil
func TestManager_CalculateROI_Nil(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	_, err := manager.CalculateROI(nil)
	if err == nil {
		t.Error("CalculateROI(nil) should return error")
	}
}

// TestManager_GenerateReport 测试生成报告
func TestManager_GenerateReport(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	now := time.Now()
	report := manager.GenerateReport("测试报告", now.AddDate(0, -1, 0), now)

	if report == nil {
		t.Fatal("GenerateReport should not return nil")
	}
	if report.ID == "" {
		t.Error("Report.ID should not be empty")
	}
	if report.ReportName != "测试报告" {
		t.Errorf("Report.ReportName = %v, want 测试报告", report.ReportName)
	}
	if report.Summary == nil {
		t.Error("Report.Summary should not be nil")
	}
	if report.Trend == nil {
		t.Error("Report.Trend should not be nil")
	}
}

// TestManager_GetReport 测试获取报告
func TestManager_GetReport(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	now := time.Now()
	report := manager.GenerateReport("报告A", now.AddDate(0, -1, 0), now)

	got, err := manager.GetReport(report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("GetReport.ID = %v, want %v", got.ID, report.ID)
	}
}

// TestManager_GetReport_NotFound 测试获取不存在的报告
func TestManager_GetReport_NotFound(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	_, err := manager.GetReport("nonexistent")
	if err == nil {
		t.Error("GetReport for nonexistent ID should return error")
	}
}

// TestManager_ListReports 测试列出报告
func TestManager_ListReports(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	now := time.Now()
	manager.GenerateReport("报告1", now.AddDate(0, -2, 0), now.AddDate(0, -1, 0))
	manager.GenerateReport("报告2", now.AddDate(0, -1, 0), now)

	reports := manager.ListReports()
	if len(reports) != 2 {
		t.Errorf("ListReports count = %v, want 2", len(reports))
	}
}

// TestManager_ExportReportAsCSV 测试导出报告为 CSV
func TestManager_ExportReportAsCSV(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	now := time.Now()
	report := manager.GenerateReport("CSV报告", now.AddDate(0, -1, 0), now)

	csv, err := manager.ExportReportAsCSV(report.ID)
	if err != nil {
		t.Fatalf("ExportReportAsCSV failed: %v", err)
	}
	if csv == "" {
		t.Error("CSV output should not be empty")
	}
	if !contains(csv, "报告名称") {
		t.Error("CSV should contain header '报告名称'")
	}
	if !contains(csv, "CSV报告") {
		t.Error("CSV should contain report name")
	}
}

// TestManager_ExportReportAsCSV_NotFound 测试导出不存在的报告
func TestManager_ExportReportAsCSV_NotFound(t *testing.T) {
	manager := NewManager(zap.NewNop(), nil)

	_, err := manager.ExportReportAsCSV("nonexistent")
	if err == nil {
		t.Error("ExportReportAsCSV for nonexistent ID should return error")
	}
}

// contains 检查字符串是否包含子串
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
