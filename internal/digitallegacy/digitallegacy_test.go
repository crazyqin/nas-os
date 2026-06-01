// Package digitallegacy 提供单元测试
package digitallegacy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestManager() *Manager {
	config := GetDefaultConfig()
	// AES-256 需要 32 字节的密钥
	encryptionKey := make([]byte, 32)
	copy(encryptionKey, []byte("test-encryption-key-32bytes!"))
	return NewManager(config, encryptionKey)
}

func setupTestHandlers() (*Handlers, *Manager) {
	manager := setupTestManager()
	handlers := NewHandlers(manager)
	return handlers, manager
}

func setupTestMux() (*http.ServeMux, *Handlers, *Manager) {
	handlers, manager := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, "/api/v1/legacy")
	return mux, handlers, manager
}

// ========== 类型验证测试 ==========

func TestIsValidTriggerType(t *testing.T) {
	tests := []struct {
		name     string
		input    TriggerType
		expected bool
	}{
		{"manual", TriggerManual, true},
		{"inactivity", TriggerInactivity, true},
		{"death_cert", TriggerDeathCert, true},
		{"emergency", TriggerEmergency, true},
		{"scheduled", TriggerScheduled, true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTriggerType(tt.input); got != tt.expected {
				t.Errorf("IsValidTriggerType(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsValidAssetType(t *testing.T) {
	tests := []struct {
		name     string
		input    AssetType
		expected bool
	}{
		{"account", AssetTypeAccount, true},
		{"file", AssetTypeFile, true},
		{"password", AssetTypePassword, true},
		{"crypto", AssetTypeCrypto, true},
		{"domain", AssetTypeDomain, true},
		{"social", AssetTypeSocial, true},
		{"email", AssetTypeEmail, true},
		{"other", AssetTypeOther, true},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidAssetType(tt.input); got != tt.expected {
				t.Errorf("IsValidAssetType(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsValidVerificationMethod(t *testing.T) {
	tests := []struct {
		name     string
		input    VerificationMethod
		expected bool
	}{
		{"email", VerifyEmail, true},
		{"phone", VerifyPhone, true},
		{"id_card", VerifyIDCard, true},
		{"death_cert", VerifyDeathCert, true},
		{"notary", VerifyNotary, true},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidVerificationMethod(tt.input); got != tt.expected {
				t.Errorf("IsValidVerificationMethod(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// ========== 计划管理测试 ==========

func TestCreatePlan(t *testing.T) {
	manager := setupTestManager()

	req := &LegacyPlanRequest{
		Name:        "Test Legacy Plan",
		Description: "Test description",
		TriggerType: TriggerInactivity,
		TriggerConditions: &TriggerConditions{
			InactivityDays: 30,
		},
		IsEncrypted: true,
	}

	plan, err := manager.CreatePlan(req, "user-001")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if plan.Name != "Test Legacy Plan" {
		t.Errorf("expected name 'Test Legacy Plan', got '%s'", plan.Name)
	}

	if plan.Status != LegacyStatusDraft {
		t.Errorf("expected status 'draft', got '%s'", plan.Status)
	}

	if plan.OwnerID != "user-001" {
		t.Errorf("expected owner 'user-001', got '%s'", plan.OwnerID)
	}
}

func TestCreatePlanValidation(t *testing.T) {
	manager := setupTestManager()

	// 测试空名称
	_, err := manager.CreatePlan(&LegacyPlanRequest{
		TriggerType: TriggerManual,
	}, "user-001")
	if err == nil {
		t.Error("expected error for empty name")
	}

	// 测试无效触发类型
	_, err = manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Test",
		TriggerType: "invalid",
	}, "user-001")
	if err == nil {
		t.Error("expected error for invalid trigger type")
	}
}

func TestGetPlan(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Test Plan",
		Description: "Test",
		TriggerType: TriggerManual,
	}, "user-001")

	// 获取计划
	got, err := manager.GetPlan(plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}

	if got.ID != plan.ID {
		t.Errorf("expected ID %s, got %s", plan.ID, got.ID)
	}

	// 获取不存在的计划
	_, err = manager.GetPlan("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plan")
	}
}

func TestListPlans(t *testing.T) {
	manager := setupTestManager()

	// 创建多个计划
	manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan 1",
		TriggerType: TriggerManual,
	}, "user-001")

	manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan 2",
		TriggerType: TriggerInactivity,
	}, "user-001")

	manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan 3",
		TriggerType: TriggerManual,
	}, "user-002")

	plans := manager.ListPlans("user-001")
	if len(plans) != 2 {
		t.Errorf("expected 2 plans for user-001, got %d", len(plans))
	}

	plans = manager.ListPlans("user-002")
	if len(plans) != 1 {
		t.Errorf("expected 1 plan for user-002, got %d", len(plans))
	}
}

func TestUpdatePlan(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Original Name",
		TriggerType: TriggerManual,
	}, "user-001")

	updated, err := manager.UpdatePlan(plan.ID, &LegacyPlanRequest{
		Name:        "Updated Name",
		Description: "Updated description",
		TriggerType: TriggerInactivity,
	})
	if err != nil {
		t.Fatalf("UpdatePlan failed: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", updated.Name)
	}
}

func TestDeletePlan(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "To Delete",
		TriggerType: TriggerManual,
	}, "user-001")

	err := manager.DeletePlan(plan.ID)
	if err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}

	_, err = manager.GetPlan(plan.ID)
	if err == nil {
		t.Error("expected error for deleted plan")
	}
}

func TestActivatePlan(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan to Activate",
		TriggerType: TriggerManual,
	}, "user-001")

	// 没有受益人和资产，应该失败
	err := manager.ActivatePlan(plan.ID)
	if err == nil {
		t.Error("expected error for plan without beneficiaries and assets")
	}

	// 添加受益人
	manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary 1",
		Role:              RoleBeneficiary,
		AllocationPercent: 100,
		AccessLevel:       AccessFull,
	})

	// 没有资产，应该失败
	err = manager.ActivatePlan(plan.ID)
	if err == nil {
		t.Error("expected error for plan without assets")
	}

	// 添加资产
	manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Test Asset",
		Type: AssetTypeFile,
	})

	// 现在应该成功
	err = manager.ActivatePlan(plan.ID)
	if err != nil {
		t.Fatalf("ActivatePlan failed: %v", err)
	}

	// 验证状态
	got, _ := manager.GetPlan(plan.ID)
	if got.Status != LegacyStatusActive {
		t.Errorf("expected status 'active', got '%s'", got.Status)
	}
}

// ========== 受益人管理测试 ==========

func TestBeneficiaryCRUD(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加
	b, err := manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "John Doe",
		Email:             "john@example.com",
		Relationship:      "Son",
		Role:              RoleBeneficiary,
		AllocationPercent: 50,
		AccessLevel:       AccessFull,
	})
	if err != nil {
		t.Fatalf("AddBeneficiary failed: %v", err)
	}

	if b.Name != "John Doe" {
		t.Errorf("expected name 'John Doe', got '%s'", b.Name)
	}

	// 列表
	beneficiaries, _ := manager.ListBeneficiaries(plan.ID)
	if len(beneficiaries) != 1 {
		t.Errorf("expected 1 beneficiary, got %d", len(beneficiaries))
	}

	// 更新
	updated, err := manager.UpdateBeneficiary(plan.ID, b.ID, &BeneficiaryRequest{
		Name:              "John Updated",
		AllocationPercent: 60,
		Role:              RoleBeneficiary,
		AccessLevel:       AccessRead,
	})
	if err != nil {
		t.Fatalf("UpdateBeneficiary failed: %v", err)
	}

	if updated.Name != "John Updated" {
		t.Errorf("expected name 'John Updated', got '%s'", updated.Name)
	}

	// 删除
	err = manager.RemoveBeneficiary(plan.ID, b.ID)
	if err != nil {
		t.Fatalf("RemoveBeneficiary failed: %v", err)
	}

	beneficiaries, _ = manager.ListBeneficiaries(plan.ID)
	if len(beneficiaries) != 0 {
		t.Errorf("expected 0 beneficiaries after removal, got %d", len(beneficiaries))
	}
}

// ========== 资产管理测试 ==========

func TestAssetCRUD(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan",
		TriggerType: TriggerManual,
		IsEncrypted: true,
	}, "user-001")

	// 添加资产
	asset, err := manager.AddAsset(plan.ID, &AssetRequest{
		Name:        "My Bitcoin Wallet",
		Type:        AssetTypeCrypto,
		Description: "Main BTC wallet",
		Data:        "wallet-private-key-data",
	})
	if err != nil {
		t.Fatalf("AddAsset failed: %v", err)
	}

	if asset.Name != "My Bitcoin Wallet" {
		t.Errorf("expected name 'My Bitcoin Wallet', got '%s'", asset.Name)
	}

	if !asset.IsEncrypted {
		t.Error("expected asset to be encrypted")
	}

	// 列表
	assets, _ := manager.ListAssets(plan.ID)
	if len(assets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(assets))
	}

	// 按类型筛选
	cryptoAssets, _ := manager.ListAssetsByType(plan.ID, AssetTypeCrypto)
	if len(cryptoAssets) != 1 {
		t.Errorf("expected 1 crypto asset, got %d", len(cryptoAssets))
	}

	// 解密
	decrypted, err := manager.DecryptAssetData(plan.ID, asset.ID)
	if err != nil {
		t.Fatalf("DecryptAssetData failed: %v", err)
	}
	if decrypted != "wallet-private-key-data" {
		t.Errorf("expected decrypted data 'wallet-private-key-data', got '%s'", decrypted)
	}

	// 删除
	err = manager.RemoveAsset(plan.ID, asset.ID)
	if err != nil {
		t.Fatalf("RemoveAsset failed: %v", err)
	}
}

// ========== 紧急联系人测试 ==========

func TestEmergencyContactCRUD(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加
	contact, err := manager.AddEmergencyContact(plan.ID, &EmergencyContactRequest{
		Name:            "Emergency Contact 1",
		Email:           "emergency@example.com",
		Relationship:    "Spouse",
		Role:            RoleEmergency,
		Level:           1,
		IsPrimary:       true,
		CanTriggerPlan:  true,
		NotifyOnTrigger: true,
	})
	if err != nil {
		t.Fatalf("AddEmergencyContact failed: %v", err)
	}

	if contact.Level != 1 {
		t.Errorf("expected level 1, got %d", contact.Level)
	}

	// 列表
	contacts, _ := manager.ListEmergencyContacts(plan.ID)
	if len(contacts) != 1 {
		t.Errorf("expected 1 contact, got %d", len(contacts))
	}

	// 验证
	vr, err := manager.VerifyEmergencyContact(plan.ID, contact.ID, VerifyEmail, "123456")
	if err != nil {
		t.Fatalf("VerifyEmergencyContact failed: %v", err)
	}

	if vr.Status != "verified" {
		t.Errorf("expected status 'verified', got '%s'", vr.Status)
	}

	// 删除
	err = manager.RemoveEmergencyContact(plan.ID, contact.ID)
	if err != nil {
		t.Fatalf("RemoveEmergencyContact failed: %v", err)
	}
}

// ========== 时间锁测试 ==========

func TestTimeLock(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan",
		TriggerType: TriggerManual,
	}, "user-001")

	// 设置时间锁（未来时间）
	futureTime := time.Now().Add(24 * time.Hour)
	tl, err := manager.SetTimeLock(plan.ID, &TimeLockRequest{
		UnlockAt:      futureTime,
		RequiredLevel: 2,
	})
	if err != nil {
		t.Fatalf("SetTimeLock failed: %v", err)
	}

	if tl.RequiredLevel != 2 {
		t.Errorf("expected required level 2, got %d", tl.RequiredLevel)
	}

	// 检查未解锁状态
	unlocked, _ := manager.CheckTimeLock(plan.ID)
	if unlocked {
		t.Error("expected time lock to be locked")
	}

	// 设置过去的时间锁
	pastTime := time.Now().Add(-1 * time.Hour)
	_, err = manager.SetTimeLock(plan.ID, &TimeLockRequest{
		UnlockAt:      pastTime,
		RequiredLevel: 1,
	})
	if err == nil {
		t.Error("expected error for past unlock time")
	}
}

// ========== 心跳测试 ==========

func TestHeartbeat(t *testing.T) {
	manager := setupTestManager()

	// 记录心跳
	record := manager.RecordHeartbeat("user-001", "test heartbeat")
	if record.Status != HeartbeatAlive {
		t.Errorf("expected status 'alive', got '%s'", record.Status)
	}

	// 检查状态
	status, err := manager.CheckHeartbeatStatus("user-001")
	if err != nil {
		t.Fatalf("CheckHeartbeatStatus failed: %v", err)
	}

	if *status != HeartbeatAlive {
		t.Errorf("expected status 'alive', got '%s'", *status)
	}
}

// ========== 信任联系人测试 ==========

func TestTrustContactCRUD(t *testing.T) {
	manager := setupTestManager()

	// 添加
	contact, err := manager.AddTrustContact("user-001", &TrustContact{
		Name:         "Trusted Contact",
		Email:        "trusted@example.com",
		Relationship: "Lawyer",
		Role:         RoleExecutor,
	})
	if err != nil {
		t.Fatalf("AddTrustContact failed: %v", err)
	}

	if contact.Name != "Trusted Contact" {
		t.Errorf("expected name 'Trusted Contact', got '%s'", contact.Name)
	}

	// 列表
	contacts := manager.GetTrustContacts("user-001")
	if len(contacts) != 1 {
		t.Errorf("expected 1 contact, got %d", len(contacts))
	}

	// 删除
	err = manager.RemoveTrustContact(contact.ID)
	if err != nil {
		t.Fatalf("RemoveTrustContact failed: %v", err)
	}
}

// ========== 遗嘱文档测试 ==========

func TestWillDocument(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan",
		TriggerType: TriggerManual,
		IsEncrypted: true,
	}, "user-001")

	// 设置遗嘱
	doc, err := manager.SetWillDocument(plan.ID, &WillDocumentRequest{
		Title:   "My Last Will",
		Content: "I hereby bequeath all my digital assets...",
	})
	if err != nil {
		t.Fatalf("SetWillDocument failed: %v", err)
	}

	if !doc.IsEncrypted {
		t.Error("expected will document to be encrypted")
	}

	// 获取并解密
	got, err := manager.GetWillDocument(plan.ID, true)
	if err != nil {
		t.Fatalf("GetWillDocument failed: %v", err)
	}

	if got.Content != "I hereby bequeath all my digital assets..." {
		t.Errorf("decrypted content mismatch")
	}
}

// ========== 访问授权测试 ==========

func TestAccessGrants(t *testing.T) {
	manager := setupTestManager()

	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加受益人和资产
	b, _ := manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary",
		Role:              RoleBeneficiary,
		AllocationPercent: 100,
		AccessLevel:       AccessFull,
	})
	manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Asset",
		Type: AssetTypeFile,
	})

	// 触发计划
	manager.ActivatePlan(plan.ID)
	manager.TriggerPlan(plan.ID, &TriggerRequest{})

	// 检查访问授权
	grants := manager.GetAccessGrants(plan.ID, b.ID)
	if len(grants) == 0 {
		t.Error("expected access grants after trigger")
	}

	// 撤销授权
	if len(grants) > 0 {
		err := manager.RevokeAccessGrant(grants[0].ID)
		if err != nil {
			t.Fatalf("RevokeAccessGrant failed: %v", err)
		}
	}
}

// ========== HTTP API 测试 ==========

func TestAPI_CreatePlan(t *testing.T) {
	mux, _, _ := setupTestMux()

	reqBody := LegacyPlanRequest{
		Name:        "API Test Plan",
		Description: "Created via API",
		TriggerType: TriggerInactivity,
		TriggerConditions: &TriggerConditions{
			InactivityDays: 30,
		},
		IsEncrypted: true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/legacy/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestAPI_GetPlan(t *testing.T) {
	mux, _, manager := setupTestMux()

	// 先创建计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Test Plan",
		TriggerType: TriggerManual,
	}, "user-001")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/legacy/plans/"+plan.ID, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAPI_ListPlans(t *testing.T) {
	mux, _, manager := setupTestMux()

	manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan 1",
		TriggerType: TriggerManual,
	}, "user-001")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/legacy/plans", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAPI_Heartbeat(t *testing.T) {
	mux, _, _ := setupTestMux()

	reqBody := map[string]string{"note": "test"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/legacy/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAPI_Config(t *testing.T) {
	mux, _, _ := setupTestMux()

	// 获取配置
	req := httptest.NewRequest(http.MethodGet, "/api/v1/legacy/config", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// 更新配置
	newConfig := GetDefaultConfig()
	newConfig.InactivityDays = 90
	body, _ := json.Marshal(newConfig)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/legacy/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// ========== 加密测试 ==========

func TestEncryption(t *testing.T) {
	manager := setupTestManager()

	// 测试加密
	encrypted, err := manager.encryptData("test data")
	if err != nil {
		t.Fatalf("encryptData failed: %v", err)
	}

	// 测试解密
	decrypted, err := manager.decryptData(encrypted)
	if err != nil {
		t.Fatalf("decryptData failed: %v", err)
	}

	if decrypted != "test data" {
		t.Errorf("expected 'test data', got '%s'", decrypted)
	}
}

func TestHashData(t *testing.T) {
	hash1 := hashData("test data")
	hash2 := hashData("test data")
	hash3 := hashData("different data")

	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different input should produce different hash")
	}
}

// ========== 默认配置测试 ==========

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	if config.InactivityDays != 365 {
		t.Errorf("expected InactivityDays 365, got %d", config.InactivityDays)
	}

	if config.GracePeriodDays != 30 {
		t.Errorf("expected GracePeriodDays 30, got %d", config.GracePeriodDays)
	}

	if config.RequiredWitnesses != 2 {
		t.Errorf("expected RequiredWitnesses 2, got %d", config.RequiredWitnesses)
	}

	if !config.EnableEncryption {
		t.Error("expected EnableEncryption to be true")
	}
}

// ========== 模块信息测试 ==========

func TestGetModuleInfo(t *testing.T) {
	info := GetModuleInfo()

	if info["name"] != ModuleName {
		t.Errorf("expected name '%s', got '%s'", ModuleName, info["name"])
	}

	if info["version"] != ModuleVersion {
		t.Errorf("expected version '%s', got '%s'", ModuleVersion, info["version"])
	}
}
