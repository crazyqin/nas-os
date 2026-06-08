package massdeploy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ==================== 测试辅助 ====================

type testLogger struct{}

func (l *testLogger) Info(msg string, args ...interface{})  {}
func (l *testLogger) Error(msg string, args ...interface{}) {}
func (l *testLogger) Debug(msg string, args ...interface{}) {}

func setupManager() *Manager {
	mgr := NewManager(&testLogger{})
	return mgr
}

func addTestAsset(mgr *Manager, name, serial string) *Asset {
	asset := &Asset{
		Name:         name,
		Type:         AssetTypeNAS,
		Status:       AssetStatusActive,
		SerialNumber: serial,
		Model:        "NAS-100",
		IPAddress:    "192.168.1.100",
		PurchaseCost: 5000.0,
		PurchaseDate: time.Now().AddDate(-2, 0, 0),
		CPUCores:     4,
		MemoryGB:     8,
		DiskSlots:    4,
	}
	mgr.AddAsset(asset)
	return asset
}

func addTestTemplate(mgr *Manager, name string) *ConfigTemplate {
	tmpl := &ConfigTemplate{
		Name:        name,
		Description: "测试模板",
		Version:     "1.0.0",
		Config:      map[string]string{"key1": "value1"},
	}
	mgr.CreateTemplate(tmpl)
	return tmpl
}

// ==================== 资产管理测试 ====================

func TestManager_AddAsset(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := &Asset{
		Name:         "TestNAS",
		Type:         AssetTypeNAS,
		Status:       AssetStatusActive,
		SerialNumber: "SN001",
		Model:        "NAS-100",
		PurchaseCost: 5000.0,
		PurchaseDate: time.Now(),
	}

	err := mgr.AddAsset(asset)
	if err != nil {
		t.Fatalf("AddAsset failed: %v", err)
	}

	if asset.ID == "" {
		t.Fatal("Asset ID should not be empty")
	}

	if asset.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set")
	}

	assets := mgr.ListAssets("", "")
	if len(assets) != 1 {
		t.Fatalf("Expected 1 asset, got %d", len(assets))
	}

	if assets[0].Name != "TestNAS" {
		t.Errorf("Expected name 'TestNAS', got '%s'", assets[0].Name)
	}
}

func TestManager_UpdateAsset(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")
	asset.Name = "UpdatedNAS"
	asset.MemoryGB = 16

	err := mgr.UpdateAsset(asset)
	if err != nil {
		t.Fatalf("UpdateAsset failed: %v", err)
	}

	updated, _ := mgr.GetAsset(asset.ID)
	if updated.Name != "UpdatedNAS" {
		t.Errorf("Expected name 'UpdatedNAS', got '%s'", updated.Name)
	}
	if updated.MemoryGB != 16 {
		t.Errorf("Expected memory 16GB, got %d", updated.MemoryGB)
	}
}

func TestManager_RemoveAsset(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")

	err := mgr.RemoveAsset(asset.ID)
	if err != nil {
		t.Fatalf("RemoveAsset failed: %v", err)
	}

	_, err = mgr.GetAsset(asset.ID)
	if err == nil {
		t.Fatal("Expected error for deleted asset")
	}
}

func TestManager_ListAssets(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	addTestAsset(mgr, "NAS-1", "SN001")
	addTestAsset(mgr, "NAS-2", "SN002")

	disk := &Asset{
		Name:         "Disk-1",
		Type:         AssetTypeDisk,
		Status:       AssetStatusActive,
		SerialNumber: "DSK001",
	}
	mgr.AddAsset(disk)

	all := mgr.ListAssets("", "")
	if len(all) != 3 {
		t.Fatalf("Expected 3 assets, got %d", len(all))
	}

	nasAssets := mgr.ListAssets(AssetTypeNAS, "")
	if len(nasAssets) != 2 {
		t.Fatalf("Expected 2 NAS assets, got %d", len(nasAssets))
	}

	activeAssets := mgr.ListAssets("", AssetStatusActive)
	if len(activeAssets) != 3 {
		t.Fatalf("Expected 3 active assets, got %d", len(activeAssets))
	}
}

func TestManager_GetHardwareInfo(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")

	info, err := mgr.GetHardwareInfo(asset.ID)
	if err != nil {
		t.Fatalf("GetHardwareInfo failed: %v", err)
	}

	if info.AssetID != asset.ID {
		t.Errorf("Expected asset ID '%s', got '%s'", asset.ID, info.AssetID)
	}
	if info.CPUCores != 4 {
		t.Errorf("Expected 4 CPU cores, got %d", info.CPUCores)
	}
}

// ==================== 部署模板测试 ====================

func TestManager_CreateTemplate(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	tmpl := &ConfigTemplate{
		Name:        "TestTemplate",
		Description: "测试配置模板",
		Version:     "1.0.0",
		Config:      map[string]string{"hostname": "nas-001"},
	}

	err := mgr.CreateTemplate(tmpl)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	if tmpl.ID == "" {
		t.Fatal("Template ID should not be empty")
	}

	templates := mgr.ListTemplates()
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}
}

func TestManager_DeleteTemplate(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	tmpl := addTestTemplate(mgr, "TestTemplate")

	err := mgr.DeleteTemplate(tmpl.ID)
	if err != nil {
		t.Fatalf("DeleteTemplate failed: %v", err)
	}

	_, err = mgr.GetTemplate(tmpl.ID)
	if err == nil {
		t.Fatal("Expected error for deleted template")
	}
}

// ==================== 部署任务测试 ====================

func TestManager_CreateDeployJob(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")
	tmpl := addTestTemplate(mgr, "TestTemplate")

	job := &DeployJob{
		Name:          "Test Deploy",
		TemplateID:    tmpl.ID,
		TargetDevices: []string{asset.ID},
		Config:        map[string]string{"key": "value"},
	}

	err := mgr.CreateDeployJob(job)
	if err != nil {
		t.Fatalf("CreateDeployJob failed: %v", err)
	}

	if job.ID == "" {
		t.Fatal("Job ID should not be empty")
	}

	if job.Status != JobStatusPending {
		t.Errorf("Expected status 'pending', got '%s'", job.Status)
	}

	if job.TotalDevices != 1 {
		t.Errorf("Expected 1 total device, got %d", job.TotalDevices)
	}
}

func TestManager_GetDeployJob(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")
	tmpl := addTestTemplate(mgr, "TestTemplate")

	job := &DeployJob{
		Name:          "Test Deploy",
		TemplateID:    tmpl.ID,
		TargetDevices: []string{asset.ID},
	}
	mgr.CreateDeployJob(job)

	// 等待部署完成
	time.Sleep(200 * time.Millisecond)

	retrieved, err := mgr.GetDeployJob(job.ID)
	if err != nil {
		t.Fatalf("GetDeployJob failed: %v", err)
	}

	if retrieved.Name != "Test Deploy" {
		t.Errorf("Expected job name 'Test Deploy', got '%s'", retrieved.Name)
	}
}

func TestManager_CancelDeployJob(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	// Create a job with a non-existent template to cause it to stay pending
	job := &DeployJob{
		ID:            "cancel-test-job",
		Name:          "Test Deploy",
		TemplateID:    "nonexistent-tmpl",
		TargetDevices: []string{"dev1"},
	}

	// Manually insert a pending job to test cancellation
	mgr.mu.Lock()
	job.Status = JobStatusPending
	job.TotalDevices = 1
	job.MaxRetries = 3
	job.Results = make(map[string]*DeployResult)
	job.CreatedAt = time.Now()
	mgr.deployJobs[job.ID] = job
	mgr.mu.Unlock()

	err := mgr.CancelDeployJob(job.ID)
	if err != nil {
		t.Fatalf("CancelDeployJob failed: %v", err)
	}

	retrieved, _ := mgr.GetDeployJob(job.ID)
	if retrieved.Status != JobStatusCancelled {
		t.Errorf("Expected status 'cancelled', got '%s'", retrieved.Status)
	}
}

func TestManager_ListDeployJobs(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")
	tmpl := addTestTemplate(mgr, "TestTemplate")

	job1 := &DeployJob{
		Name:          "Deploy 1",
		TemplateID:    tmpl.ID,
		TargetDevices: []string{asset.ID},
	}
	mgr.CreateDeployJob(job1)

	job2 := &DeployJob{
		Name:          "Deploy 2",
		TemplateID:    tmpl.ID,
		TargetDevices: []string{asset.ID},
	}
	mgr.CreateDeployJob(job2)

	jobs := mgr.ListDeployJobs("")
	if len(jobs) != 2 {
		t.Fatalf("Expected 2 jobs, got %d", len(jobs))
	}
}

// ==================== 固件管理测试 ====================

func TestManager_AddFirmwareInfo(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	info := &FirmwareInfo{
		Version:     "2.0.0",
		Model:       "NAS-100",
		Status:      FirmwareStatusStable,
		ReleaseDate: time.Now(),
		SizeBytes:   1024 * 1024 * 50,
	}

	err := mgr.AddFirmwareInfo(info)
	if err != nil {
		t.Fatalf("AddFirmwareInfo failed: %v", err)
	}

	if info.ID == "" {
		t.Fatal("Firmware ID should not be empty")
	}

	infos := mgr.ListFirmwareInfo("")
	if len(infos) != 1 {
		t.Fatalf("Expected 1 firmware info, got %d", len(infos))
	}
}

func TestManager_CheckFirmwareUpdates(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	// 添加资产（固件版本 1.0.0）
	asset := &Asset{
		Name:            "TestNAS",
		Type:            AssetTypeNAS,
		Status:          AssetStatusActive,
		Model:           "NAS-100",
		FirmwareVersion: "1.0.0",
		IPAddress:       "192.168.1.100",
	}
	mgr.AddAsset(asset)

	// 添加新固件（2.0.0）
	fw := &FirmwareInfo{
		Version:     "2.0.0",
		Model:       "NAS-100",
		Status:      FirmwareStatusStable,
		ReleaseDate: time.Now(),
	}
	mgr.AddFirmwareInfo(fw)

	updates := mgr.CheckFirmwareUpdates()
	if len(updates) != 1 {
		t.Fatalf("Expected 1 update, got %d", len(updates))
	}

	if updates[asset.ID] != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got '%s'", updates[asset.ID])
	}
}

func TestManager_CreateFirmwareUpgradeJob(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")

	job := &FirmwareUpgradeJob{
		Version:       "2.0.0",
		TargetDevices: []string{asset.ID},
		RollbackPlan:  "回滚到 1.0.0",
	}

	err := mgr.CreateFirmwareUpgradeJob(job)
	if err != nil {
		t.Fatalf("CreateFirmwareUpgradeJob failed: %v", err)
	}

	if job.ID == "" {
		t.Fatal("Job ID should not be empty")
	}

	if job.Status != JobStatusPending {
		t.Errorf("Expected status 'pending', got '%s'", job.Status)
	}
}

// ==================== 费用统计测试 ====================

func TestManager_AddCostRecord(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	record := &CostRecord{
		AssetID:     "asset_1",
		Type:        CostTypePurchase,
		Amount:      5000.0,
		Currency:    "CNY",
		Description: "NAS 设备采购",
		Date:        time.Now(),
	}

	err := mgr.AddCostRecord(record)
	if err != nil {
		t.Fatalf("AddCostRecord failed: %v", err)
	}

	if record.ID == "" {
		t.Fatal("Record ID should not be empty")
	}
}

func TestManager_GetCostSummary(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mgr.AddCostRecord(&CostRecord{
		AssetID: "asset_1",
		Type:    CostTypePurchase,
		Amount:  5000.0,
		Currency: "CNY",
	})
	mgr.AddCostRecord(&CostRecord{
		AssetID: "asset_1",
		Type:    CostTypeMaintenance,
		Amount:  200.0,
		Currency: "CNY",
	})
	mgr.AddCostRecord(&CostRecord{
		AssetID: "asset_2",
		Type:    CostTypePower,
		Amount:  50.0,
		Currency: "CNY",
	})

	summary := mgr.GetCostSummary("2024")
	if summary.GrandTotal != 5250.0 {
		t.Errorf("Expected grand total 5250.0, got %.2f", summary.GrandTotal)
	}
	if summary.TotalPurchase != 5000.0 {
		t.Errorf("Expected total purchase 5000.0, got %.2f", summary.TotalPurchase)
	}
	if summary.TotalMaintenance != 200.0 {
		t.Errorf("Expected total maintenance 200.0, got %.2f", summary.TotalMaintenance)
	}
	if summary.ByType["purchase"] != 5000.0 {
		t.Errorf("Expected by type 'purchase' 5000.0, got %.2f", summary.ByType["purchase"])
	}
}

func TestManager_CalculateDepreciation(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := &Asset{
		Name:         "TestNAS",
		Type:         AssetTypeNAS,
		Status:       AssetStatusActive,
		SerialNumber: "SN001",
		PurchaseCost: 10000.0,
		PurchaseDate: time.Now().AddDate(-2, 0, 0), // 2年前
	}
	mgr.AddAsset(asset)

	info, err := mgr.CalculateDepreciation(asset.ID)
	if err != nil {
		t.Fatalf("CalculateDepreciation failed: %v", err)
	}

	// 20% 年折旧率，2年后：10000 * (0.8)^2 = 6400
	if info.PurchaseCost != 10000.0 {
		t.Errorf("Expected purchase cost 10000.0, got %.2f", info.PurchaseCost)
	}
	if info.DepreciationRate != 0.20 {
		t.Errorf("Expected depreciation rate 0.20, got %.2f", info.DepreciationRate)
	}
	// 当前值应该 < 10000
	if info.CurrentValue >= 10000.0 {
		t.Errorf("Expected current value < 10000, got %.2f", info.CurrentValue)
	}
	// 当前值应该 > 5000（2年20%折旧不会降到5000以下）
	if info.CurrentValue < 5000.0 {
		t.Errorf("Expected current value > 5000, got %.2f", info.CurrentValue)
	}
}

// ==================== 报告生成测试 ====================

func TestManager_GenerateDeployReport(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	report := mgr.GenerateDeployReport("2024-Q1")
	if report.Type != ReportTypeDeploy {
		t.Errorf("Expected type 'deploy', got '%s'", report.Type)
	}
	if report.Title != "部署报告" {
		t.Errorf("Expected title '部署报告', got '%s'", report.Title)
	}
	if report.ID == "" {
		t.Fatal("Report ID should not be empty")
	}
}

func TestManager_GenerateAssetReport(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	addTestAsset(mgr, "NAS-1", "SN001")
	addTestAsset(mgr, "NAS-2", "SN002")

	report := mgr.GenerateAssetReport("2024-Q1")
	if report.Type != ReportTypeAsset {
		t.Errorf("Expected type 'asset', got '%s'", report.Type)
	}
	if report.Title != "资产报告" {
		t.Errorf("Expected title '资产报告', got '%s'", report.Title)
	}
}

func TestManager_GenerateCostReport(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mgr.AddCostRecord(&CostRecord{
		AssetID:  "asset_1",
		Type:     CostTypePurchase,
		Amount:   5000.0,
		Currency: "CNY",
	})

	report := mgr.GenerateCostReport("2024-Q1")
	if report.Type != ReportTypeCost {
		t.Errorf("Expected type 'cost', got '%s'", report.Type)
	}
}

// ==================== 统计信息测试 ====================

func TestManager_GetStats(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	addTestAsset(mgr, "NAS-1", "SN001")
	addTestAsset(mgr, "NAS-2", "SN002")

	mgr.AddCostRecord(&CostRecord{
		AssetID: "asset_1",
		Type:    CostTypePurchase,
		Amount:  5000.0,
		Currency: "CNY",
	})

	stats := mgr.GetStats()
	if stats.TotalAssets != 2 {
		t.Errorf("Expected 2 total assets, got %d", stats.TotalAssets)
	}
	if stats.ActiveAssets != 2 {
		t.Errorf("Expected 2 active assets, got %d", stats.ActiveAssets)
	}
	if stats.TotalCost != 5000.0 {
		t.Errorf("Expected total cost 5000.0, got %.2f", stats.TotalCost)
	}
}

// ==================== HTTP Handler 测试 ====================

func TestHandleAssets_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	addTestAsset(mgr, "TestNAS", "SN001")

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/assets", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var assets []*Asset
	json.NewDecoder(w.Body).Decode(&assets)
	if len(assets) != 1 {
		t.Fatalf("Expected 1 asset, got %d", len(assets))
	}
}

func TestHandleAssets_POST(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]interface{}{
		"name":          "TestNAS",
		"type":          "nas",
		"status":        "active",
		"serial_number": "SN001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/massdeploy/assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var asset Asset
	json.NewDecoder(w.Body).Decode(&asset)
	if asset.Name != "TestNAS" {
		t.Errorf("Expected name 'TestNAS', got '%s'", asset.Name)
	}
}

func TestHandleDeploy_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")
	tmpl := addTestTemplate(mgr, "TestTemplate")

	job := &DeployJob{
		Name:          "Test Deploy",
		TemplateID:    tmpl.ID,
		TargetDevices: []string{asset.ID},
	}
	mgr.CreateDeployJob(job)

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/deploy", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var jobs []*DeployJob
	json.NewDecoder(w.Body).Decode(&jobs)
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs))
	}
}

func TestHandleStats_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var stats Stats
	json.NewDecoder(w.Body).Decode(&stats)
	if stats.TotalAssets != 0 {
		t.Errorf("Expected 0 total assets, got %d", stats.TotalAssets)
	}
}

func TestHandleCosts_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mgr.AddCostRecord(&CostRecord{
		AssetID:  "asset_1",
		Type:     CostTypePurchase,
		Amount:   5000.0,
		Currency: "CNY",
	})

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/costs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var records []*CostRecord
	json.NewDecoder(w.Body).Decode(&records)
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}
}

func TestHandleCostSummary_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mgr.AddCostRecord(&CostRecord{
		AssetID:  "asset_1",
		Type:     CostTypePurchase,
		Amount:   5000.0,
		Currency: "CNY",
	})

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/costs/summary?period=2024", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var summary CostSummary
	json.NewDecoder(w.Body).Decode(&summary)
	if summary.GrandTotal != 5000.0 {
		t.Errorf("Expected grand total 5000.0, got %.2f", summary.GrandTotal)
	}
}

func TestHandleFirmware_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mgr.AddFirmwareInfo(&FirmwareInfo{
		Version:     "2.0.0",
		Model:       "NAS-100",
		Status:      FirmwareStatusStable,
		ReleaseDate: time.Now(),
	})

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/firmware", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var infos []*FirmwareInfo
	json.NewDecoder(w.Body).Decode(&infos)
	if len(infos) != 1 {
		t.Fatalf("Expected 1 firmware info, got %d", len(infos))
	}
}

func TestHandleTemplates_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	addTestTemplate(mgr, "TestTemplate")

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/templates", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var templates []*ConfigTemplate
	json.NewDecoder(w.Body).Decode(&templates)
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}
}

func TestHandleEvents_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	addTestAsset(mgr, "TestNAS", "SN001")

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var events []Event
	json.NewDecoder(w.Body).Decode(&events)
	if len(events) < 1 {
		t.Fatalf("Expected at least 1 event, got %d", len(events))
	}
}

func TestHandleReports_GET(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mgr.GenerateDeployReport("2024-Q1")

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/reports", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var reports []*Report
	json.NewDecoder(w.Body).Decode(&reports)
	if len(reports) != 1 {
		t.Fatalf("Expected 1 report, got %d", len(reports))
	}
}

func TestHandleDeploy_POST(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	asset := addTestAsset(mgr, "TestNAS", "SN001")
	tmpl := addTestTemplate(mgr, "TestTemplate")

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]interface{}{
		"name":           "Test Deploy",
		"template_id":    tmpl.ID,
		"target_devices": []string{asset.ID},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/massdeploy/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var job DeployJob
	json.NewDecoder(w.Body).Decode(&job)
	if job.Name != "Test Deploy" {
		t.Errorf("Expected job name 'Test Deploy', got '%s'", job.Name)
	}
}

func TestHandleMethodNotAllowed(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	// POST 到只支持 GET 的路由
	req := httptest.NewRequest(http.MethodPost, "/api/massdeploy/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleAssetDetail_NotFound(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/assets/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleCostSummary_Empty(t *testing.T) {
	mgr := setupManager()
	defer mgr.Stop()

	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/massdeploy/costs/summary", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var summary CostSummary
	json.NewDecoder(w.Body).Decode(&summary)
	if summary.GrandTotal != 0 {
		t.Errorf("Expected grand total 0, got %.2f", summary.GrandTotal)
	}
}
