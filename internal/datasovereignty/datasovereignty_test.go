package datasovereignty

import (
	"testing"
)

func TestDataSovereigntyManager_CreatePolicy(t *testing.T) {
	dsm := NewDataSovereigntyManager(nil)

	policy := &DataPolicy{
		ID:                 "policy-1",
		Name:               "GDPR Compliance",
		Description:        "GDPR data protection policy",
		AllowedRegions:     []Region{RegionEU, RegionLocal},
		BlockedRegions:     []Region{RegionUS},
		Frameworks:         []ComplianceFramework{FrameworkGDPR},
		Classification:     DataClassification("pii"),
		EncryptionRequired: true,
		Enabled:            true,
	}

	err := dsm.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// 测试重复创建
	err = dsm.CreatePolicy(policy)
	if err == nil {
		t.Fatal("Expected error for duplicate policy")
	}
}

func TestDataSovereigntyManager_RegisterAsset(t *testing.T) {
	dsm := NewDataSovereigntyManager(nil)

	// 先创建策略
	policy := &DataPolicy{
		ID:                 "policy-1",
		Name:               "Test Policy",
		Enabled:            true,
		EncryptionRequired: false,
	}
	dsm.CreatePolicy(policy)

	asset := &DataAsset{
		ID:             "asset-1",
		Name:           "user-data.csv",
		Path:           "/data/users/user-data.csv",
		Size:           1024 * 1024,
		Classification: DataClassification("pii"),
		CurrentRegion:  RegionEU,
		OriginRegion:   RegionEU,
		OwnerID:        "user-1",
		PolicyID:       "policy-1",
		Encrypted:      true,
	}

	err := dsm.RegisterAsset(asset)
	if err != nil {
		t.Fatalf("Failed to register asset: %v", err)
	}

	// 验证资产已注册
	registered, err := dsm.GetDataAsset("asset-1")
	if err != nil {
		t.Fatalf("Failed to get asset: %v", err)
	}
	if registered.Name != "user-data.csv" {
		t.Errorf("Expected name 'user-data.csv', got '%s'", registered.Name)
	}
}

func TestDataSovereigntyManager_TransferRequest(t *testing.T) {
	dsm := NewDataSovereigntyManager(&SovereigntyConfig{
		DefaultRegion:   RegionLocal,
		RequireApproval: true,
	})

	// 创建策略和资产
	policy := &DataPolicy{
		ID:             "policy-1",
		Name:           "EU Only",
		AllowedRegions: []Region{RegionEU, RegionLocal},
		BlockedRegions: []Region{RegionUS},
		Enabled:        true,
	}
	dsm.CreatePolicy(policy)

	asset := &DataAsset{
		ID:            "asset-1",
		Name:          "test-data",
		CurrentRegion: RegionEU,
		PolicyID:      "policy-1",
		Encrypted:     true,
		Compliant:     true,
	}
	dsm.RegisterAsset(asset)

	// 测试合规传输请求
	req := &TransferRequest{
		ID:           "transfer-1",
		AssetID:      "asset-1",
		SourceRegion: RegionEU,
		TargetRegion: RegionLocal,
		RequesterID:  "user-1",
		Reason:       "Backup purposes",
	}

	err := dsm.RequestTransfer(req)
	if err != nil {
		t.Fatalf("Failed to create transfer request: %v", err)
	}

	// 测试批准传输
	err = dsm.ApproveTransfer("transfer-1", "admin-1")
	if err != nil {
		t.Fatalf("Failed to approve transfer: %v", err)
	}
}

func TestDataSovereigntyManager_ComplianceAudit(t *testing.T) {
	dsm := NewDataSovereigntyManager(nil)

	// 创建策略
	policy := &DataPolicy{
		ID:         "policy-gdpr",
		Name:       "GDPR Policy",
		Frameworks: []ComplianceFramework{FrameworkGDPR},
		Enabled:    true,
	}
	dsm.CreatePolicy(policy)

	// 注册合规资产
	dsm.RegisterAsset(&DataAsset{
		ID:            "asset-1",
		Name:          "compliant-data",
		CurrentRegion: RegionEU,
		PolicyID:      "policy-gdpr",
		Encrypted:     true,
		Compliant:     true,
	})

	// 运行审计
	report, err := dsm.RunComplianceAudit(RegionGlobal, FrameworkGDPR)
	if err != nil {
		t.Fatalf("Failed to run audit: %v", err)
	}

	if report.TotalAssets < 1 {
		t.Errorf("Expected at least 1 total asset, got %d", report.TotalAssets)
	}

	if report.ComplianceRate < 0 || report.ComplianceRate > 100 {
		t.Errorf("Invalid compliance rate: %f", report.ComplianceRate)
	}
}

func TestDataSovereigntyManager_Stats(t *testing.T) {
	dsm := NewDataSovereigntyManager(nil)

	dsm.CreatePolicy(&DataPolicy{
		ID:      "p1",
		Enabled: true,
	})

	dsm.RegisterAsset(&DataAsset{
		ID:            "a1",
		CurrentRegion: RegionEU,
		Compliant:     true,
	})

	dsm.RegisterAsset(&DataAsset{
		ID:            "a2",
		CurrentRegion: RegionUS,
		Compliant:     false,
	})

	stats := dsm.GetStats()

	if stats.TotalAssets != 2 {
		t.Errorf("Expected 2 assets, got %d", stats.TotalAssets)
	}

	if stats.TotalPolicies != 1 {
		t.Errorf("Expected 1 policy, got %d", stats.TotalPolicies)
	}
}

func TestDataSovereignty_MarshalJSON(t *testing.T) {
	dsm := NewDataSovereigntyManager(nil)

	data, err := dsm.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON")
	}
}

func TestRegion_Constants(t *testing.T) {
	regions := []Region{
		RegionChina, RegionEU, RegionUS, RegionAPAC, RegionGlobal, RegionLocal,
	}

	for _, r := range regions {
		if r == "" {
			t.Error("Region constant should not be empty")
		}
	}
}

func TestComplianceFramework_Constants(t *testing.T) {
	frameworks := []ComplianceFramework{
		FrameworkGDPR, FrameworkCCPA, FrameworkPIPL, FrameworkHIPAA, FrameworkSOX, FrameworkISO27001,
	}

	for _, f := range frameworks {
		if f == "" {
			t.Error("Framework constant should not be empty")
		}
	}
}
