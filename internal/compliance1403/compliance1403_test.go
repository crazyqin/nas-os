package compliance1403

import (
	"context"
	"testing"
)

func TestRegisterModule(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	module := &CryptoModule{
		ID:            "module-1",
		Name:          "AES Module",
		Version:       "1.0",
		Type:          "software",
		Algorithm:     "AES-256-GCM",
		KeySize:       256,
		Certification: "FIPS 140-3",
		Level:         1,
		Vendor:        "Test Vendor",
	}

	err := c.RegisterModule(ctx, module)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if module.Status != "pending" {
		t.Errorf("expected status pending, got %s", module.Status)
	}

	// Check audit log
	logs := c.GetAuditLog(ctx, "module_registration")
	if len(logs) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(logs))
	}
}

func TestRegisterModuleNoID(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	module := &CryptoModule{
		Name: "No ID",
	}

	err := c.RegisterModule(ctx, module)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestRegisterKey(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})

	key := &KeyEntry{
		ID:       "key-1",
		Name:     "Master Key",
		Type:     "aes",
		Size:     256,
		Algorithm: "AES-256-GCM",
		ModuleID: "module-1",
	}

	err := c.RegisterKey(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key.Status != "active" {
		t.Errorf("expected status active, got %s", key.Status)
	}
}

func TestRegisterKeyNoModule(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	key := &KeyEntry{
		ID:       "key-1",
		Name:     "Test Key",
		ModuleID: "nonexistent",
	}

	err := c.RegisterKey(ctx, key)
	if err == nil {
		t.Fatal("expected error for nonexistent module")
	}
}

func TestRotateKey(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})
	c.RegisterKey(ctx, &KeyEntry{ID: "key-1", Name: "Test Key", ModuleID: "module-1"})

	err := c.RotateKey(ctx, "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := c.GetKeys(ctx, "module-1")
	if keys[0].Status != "rotated" {
		t.Errorf("expected status rotated, got %s", keys[0].Status)
	}
	if keys[0].RotatedAt == nil {
		t.Error("RotatedAt should be set")
	}
}

func TestRotateKeyNotFound(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	err := c.RotateKey(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunSelfTest(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})

	test, err := c.RunSelfTest(ctx, "module-1", "power_up")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if test.Status != "passed" {
		t.Errorf("expected status passed, got %s", test.Status)
	}

	// Module should be compliant now
	module, _ := c.GetModule(ctx, "module-1")
	if module.Status != "compliant" {
		t.Errorf("expected module status compliant, got %s", module.Status)
	}
}

func TestRunSelfTestNotFound(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	_, err := c.RunSelfTest(ctx, "nonexistent", "power_up")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAddPolicy(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	policy := &CompliancePolicy{
		ID:       "policy-1",
		Name:     "FIPS 140-3 Level 1",
		Standard: "FIPS 140-3",
		Level:    1,
		Requirements: []string{
			"Cryptographic module specification",
			"Cryptographic module ports and interfaces",
			"Roles, services, and authentication",
		},
		Enabled: true,
	}

	err := c.AddPolicy(ctx, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddPolicyNoID(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	policy := &CompliancePolicy{
		Name: "No ID",
	}

	err := c.AddPolicy(ctx, policy)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestGenerateReport(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	// Setup compliant module and key
	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})
	c.RunSelfTest(ctx, "module-1", "power_up")
	c.RegisterKey(ctx, &KeyEntry{ID: "key-1", Name: "Test Key", ModuleID: "module-1"})

	report, err := c.GenerateReport(ctx, "FIPS 140-3", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.OverallStatus != "compliant" {
		t.Errorf("expected compliant, got %s", report.OverallStatus)
	}
	if report.ModulesChecked != 1 {
		t.Errorf("expected 1 module checked, got %d", report.ModulesChecked)
	}
	if report.KeysChecked != 1 {
		t.Errorf("expected 1 key checked, got %d", report.KeysChecked)
	}
}

func TestGenerateReportNonCompliant(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	// Register module but don't run self-test
	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})

	report, err := c.GenerateReport(ctx, "FIPS 140-3", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.OverallStatus != "non_compliant" {
		t.Errorf("expected non_compliant, got %s", report.OverallStatus)
	}
	if len(report.Findings) == 0 {
		t.Error("expected findings for non-compliant module")
	}
}

func TestGetModule(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})

	module, err := c.GetModule(ctx, "module-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if module.Name != "Test Module" {
		t.Errorf("expected Test Module, got %s", module.Name)
	}
}

func TestGetModuleNotFound(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	_, err := c.GetModule(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListModules(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	c.RegisterModule(ctx, &CryptoModule{ID: "m1", Name: "Module 1"})
	c.RegisterModule(ctx, &CryptoModule{ID: "m2", Name: "Module 2"})

	modules := c.ListModules(ctx)
	if len(modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(modules))
	}
}

func TestGetKeys(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})
	c.RegisterKey(ctx, &KeyEntry{ID: "key-1", Name: "Key 1", ModuleID: "module-1"})
	c.RegisterKey(ctx, &KeyEntry{ID: "key-2", Name: "Key 2", ModuleID: "module-1"})

	keys := c.GetKeys(ctx, "module-1")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}

	// Empty filter returns all
	keys = c.GetKeys(ctx, "")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestGetAuditLog(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})
	c.RegisterKey(ctx, &KeyEntry{ID: "key-1", Name: "Test Key", ModuleID: "module-1"})

	// Should have entries for module registration and key creation
	logs := c.GetAuditLog(ctx, "")
	if len(logs) < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", len(logs))
	}

	logs = c.GetAuditLog(ctx, "module_registration")
	if len(logs) != 1 {
		t.Errorf("expected 1 module registration entry, got %d", len(logs))
	}
}

func TestGetReports(t *testing.T) {
	c := NewCompliance1403()
	ctx := context.Background()

	// Empty
	reports := c.GetReports(ctx)
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}

	// Generate one
	c.RegisterModule(ctx, &CryptoModule{ID: "module-1", Name: "Test Module"})
	c.GenerateReport(ctx, "FIPS 140-3", 1)

	reports = c.GetReports(ctx)
	if len(reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(reports))
	}
}
