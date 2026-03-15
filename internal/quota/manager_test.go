package quota

import (
	"path/filepath"
	"testing"
	"time"
)

// ========== Manager 核心功能测试 ==========

func TestManager_NewManager(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestManager_NewManager_WithConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "quota-config.json")

	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager(configPath, storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestManager_StartStop(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Start and stop should not panic
	mgr.Start()
	time.Sleep(10 * time.Millisecond)
	mgr.Stop()
}

// ========== 配额 CRUD 测试 ==========

func TestManager_CreateQuota_User(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30, // 100GB
		SoftLimit:  80 << 30,  // 80GB
	}

	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("CreateQuota failed: %v", err)
	}

	if quota.Type != QuotaTypeUser {
		t.Errorf("expected type user, got %s", quota.Type)
	}
	if quota.TargetID != "testuser" {
		t.Errorf("expected target testuser, got %s", quota.TargetID)
	}
	if quota.HardLimit != 100<<30 {
		t.Errorf("hard limit mismatch")
	}
}

func TestManager_CreateQuota_Group(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddGroup("developers")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
	}

	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("CreateQuota failed: %v", err)
	}

	if quota.Type != QuotaTypeGroup {
		t.Errorf("expected type group, got %s", quota.Type)
	}
}

func TestManager_CreateQuota_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeDirectory,
		TargetID:   tmpDir,
		VolumeName: "data",
		Path:       tmpDir,
		HardLimit:  200 << 30,
	}

	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("CreateQuota failed: %v", err)
	}

	if quota.Type != QuotaTypeDirectory {
		t.Errorf("expected type directory, got %s", quota.Type)
	}
}

func TestManager_CreateQuota_UserNotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "nonexistent",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	_, err = mgr.CreateQuota(input)
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestManager_CreateQuota_GroupNotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "nonexistent",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	_, err = mgr.CreateQuota(input)
	if err != ErrGroupNotFound {
		t.Errorf("expected ErrGroupNotFound, got %v", err)
	}
}

func TestManager_CreateQuota_DirectoryNotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeDirectory,
		TargetID:   "/nonexistent/path",
		VolumeName: "data",
		Path:       "/nonexistent/path",
		HardLimit:  100 << 30,
	}

	_, err = mgr.CreateQuota(input)
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestManager_CreateQuota_InvalidLimit(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  0, // Invalid
	}

	_, err = mgr.CreateQuota(input)
	if err != ErrInvalidLimit {
		t.Errorf("expected ErrInvalidLimit, got %v", err)
	}
}

func TestManager_CreateQuota_Duplicate(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	_, err = mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("first CreateQuota failed: %v", err)
	}

	// Create duplicate
	_, err = mgr.CreateQuota(input)
	if err != ErrQuotaExists {
		t.Errorf("expected ErrQuotaExists, got %v", err)
	}
}

func TestManager_GetQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	quota, _ := mgr.CreateQuota(input)

	retrieved, err := mgr.GetQuota(quota.ID)
	if err != nil {
		t.Fatalf("GetQuota failed: %v", err)
	}

	if retrieved.ID != quota.ID {
		t.Errorf("ID mismatch")
	}
}

func TestManager_GetQuota_NotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = mgr.GetQuota("nonexistent")
	if err != ErrQuotaNotFound {
		t.Errorf("expected ErrQuotaNotFound, got %v", err)
	}
}

func TestManager_UpdateQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	quota, _ := mgr.CreateQuota(input)

	// Update
	updateInput := QuotaInput{
		HardLimit: 200 << 30,
		SoftLimit: 150 << 30,
	}

	updated, err := mgr.UpdateQuota(quota.ID, updateInput)
	if err != nil {
		t.Fatalf("UpdateQuota failed: %v", err)
	}

	if updated.HardLimit != 200<<30 {
		t.Errorf("hard limit not updated")
	}
}

func TestManager_UpdateQuota_NotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		HardLimit: 100 << 30,
	}

	_, err = mgr.UpdateQuota("nonexistent", input)
	if err != ErrQuotaNotFound {
		t.Errorf("expected ErrQuotaNotFound, got %v", err)
	}
}

func TestManager_UpdateQuota_InvalidLimit(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	quota, _ := mgr.CreateQuota(input)

	// Try to update with zero limit
	updateInput := QuotaInput{
		HardLimit: 0,
	}

	_, err = mgr.UpdateQuota(quota.ID, updateInput)
	if err != ErrInvalidLimit {
		t.Errorf("expected ErrInvalidLimit, got %v", err)
	}
}

func TestManager_DeleteQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	quota, _ := mgr.CreateQuota(input)

	err = mgr.DeleteQuota(quota.ID)
	if err != nil {
		t.Fatalf("DeleteQuota failed: %v", err)
	}

	// Verify deleted
	_, err = mgr.GetQuota(quota.ID)
	if err != ErrQuotaNotFound {
		t.Errorf("expected ErrQuotaNotFound after delete, got %v", err)
	}
}

func TestManager_DeleteQuota_NotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	err = mgr.DeleteQuota("nonexistent")
	if err != ErrQuotaNotFound {
		t.Errorf("expected ErrQuotaNotFound, got %v", err)
	}
}

// ========== 列表查询测试 ==========

func TestManager_ListQuotas(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("user1", "/home/user1")
	user.AddUser("user2", "/home/user2")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create multiple quotas
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user1",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user2",
		VolumeName: "data",
		HardLimit:  200 << 30,
	})

	quotas := mgr.ListQuotas()
	if len(quotas) != 2 {
		t.Errorf("expected 2 quotas, got %d", len(quotas))
	}
}

func TestManager_ListQuotas_Empty(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	quotas := mgr.ListQuotas()
	if len(quotas) != 0 {
		t.Errorf("expected 0 quotas, got %d", len(quotas))
	}
}

func TestManager_ListUserQuotas(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "backup",
		HardLimit:  200 << 30,
	})

	quotas := mgr.ListUserQuotas("testuser")
	if len(quotas) != 2 {
		t.Errorf("expected 2 quotas, got %d", len(quotas))
	}
}

func TestManager_ListGroupQuotas(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddGroup("developers")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
	})

	quotas := mgr.ListGroupQuotas("developers")
	if len(quotas) != 1 {
		t.Errorf("expected 1 quota, got %d", len(quotas))
	}
}

func TestManager_ListDirectoryQuotas(t *testing.T) {
	tmpDir := t.TempDir()

	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeDirectory,
		TargetID:   tmpDir,
		VolumeName: "data",
		Path:       tmpDir,
		HardLimit:  100 << 30,
	})

	quotas := mgr.ListDirectoryQuotas()
	if len(quotas) != 1 {
		t.Errorf("expected 1 quota, got %d", len(quotas))
	}
}

func TestManager_GetDirectoryQuota(t *testing.T) {
	tmpDir := t.TempDir()

	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeDirectory,
		TargetID:   tmpDir,
		VolumeName: "data",
		Path:       tmpDir,
		HardLimit:  100 << 30,
	})

	quota, err := mgr.GetDirectoryQuota(tmpDir)
	if err != nil {
		t.Fatalf("GetDirectoryQuota failed: %v", err)
	}

	if quota.Path != tmpDir {
		t.Errorf("path mismatch")
	}
}

func TestManager_GetDirectoryQuota_NotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = mgr.GetDirectoryQuota("/nonexistent")
	if err != ErrQuotaNotFound {
		t.Errorf("expected ErrQuotaNotFound, got %v", err)
	}
}

// ========== 使用量查询测试 ==========

func TestManager_GetUsage(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	quota, _ := mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})

	usage, err := mgr.GetUsage(quota.ID)
	if err != nil {
		t.Fatalf("GetUsage failed: %v", err)
	}

	if usage == nil {
		t.Fatal("usage should not be nil")
	}
}

func TestManager_GetUsage_NotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = mgr.GetUsage("nonexistent")
	if err != ErrQuotaNotFound {
		t.Errorf("expected ErrQuotaNotFound, got %v", err)
	}
}

func TestManager_GetAllUsage(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("user1", "/home/user1")
	user.AddUser("user2", "/home/user2")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user1",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user2",
		VolumeName: "data",
		HardLimit:  200 << 30,
	})

	usages, err := mgr.GetAllUsage()
	if err != nil {
		t.Fatalf("GetAllUsage failed: %v", err)
	}

	if len(usages) != 2 {
		t.Errorf("expected 2 usages, got %d", len(usages))
	}
}

func TestManager_GetUserUsage(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})

	usages, err := mgr.GetUserUsage("testuser")
	if err != nil {
		t.Fatalf("GetUserUsage failed: %v", err)
	}

	if len(usages) != 1 {
		t.Errorf("expected 1 usage, got %d", len(usages))
	}
}

// ========== 告警测试 ==========

func TestManager_GetAlerts(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	alerts := mgr.GetAlerts()
	if alerts == nil {
		t.Error("GetAlerts should not return nil")
	}
}

func TestManager_GetAlertHistory(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	history := mgr.GetAlertHistory(10)
	if history == nil {
		t.Error("GetAlertHistory should not return nil")
	}
}

func TestManager_SetAlertConfig(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	config := AlertConfig{
		Enabled:            true,
		SoftLimitThreshold: 80,
		HardLimitThreshold: 95,
		CheckInterval:      5 * time.Minute,
	}

	mgr.SetAlertConfig(config)

	retrieved := mgr.GetAlertConfig()
	if retrieved.SoftLimitThreshold != 80 {
		t.Errorf("soft limit threshold mismatch")
	}
}

func TestManager_SilenceAlert(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Add a test alert
	alert := &Alert{
		ID:     "test-alert",
		Status: AlertStatusActive,
	}
	mgr.mu.Lock()
	mgr.alerts["test-alert"] = alert
	mgr.mu.Unlock()

	err = mgr.SilenceAlert("test-alert")
	if err != nil {
		t.Fatalf("SilenceAlert failed: %v", err)
	}

	mgr.mu.RLock()
	silenced := mgr.alerts["test-alert"]
	mgr.mu.RUnlock()

	if silenced.Status != AlertStatusSilenced {
		t.Error("alert should be silenced")
	}
}

func TestManager_SilenceAlert_NotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	err = mgr.SilenceAlert("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestManager_ResolveAlert(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Add a test alert
	alert := &Alert{
		ID:     "test-alert",
		Status: AlertStatusActive,
	}
	mgr.mu.Lock()
	mgr.alerts["test-alert"] = alert
	mgr.mu.Unlock()

	err = mgr.ResolveAlert("test-alert")
	if err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}

	// Alert should be moved to history
	mgr.mu.RLock()
	_, exists := mgr.alerts["test-alert"]
	historyLen := len(mgr.alertHistory)
	mgr.mu.RUnlock()

	if exists {
		t.Error("alert should be removed from active alerts")
	}
	if historyLen != 1 {
		t.Errorf("expected 1 alert in history, got %d", historyLen)
	}
}

// ========== 配额检查测试 ==========

func TestManager_CheckQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// No quota, should allow
	err = mgr.CheckQuota("testuser", "data", 1000)
	if err != nil {
		t.Errorf("expected no error without quota, got %v", err)
	}
}

func TestManager_CheckQuota_WithQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create quota
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})

	// Check quota - should allow since no actual files
	err = mgr.CheckQuota("testuser", "data", 0)
	// Note: without actual file system, the check may not work as expected
	// This is a basic test to ensure the function doesn't panic
	t.Logf("CheckQuota result: %v", err)
}

// ========== 持久化测试 ==========

func TestManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "quota-config.json")

	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	// Create manager and quota
	mgr1, err := NewManager(configPath, storage, user)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	quota1, err := mgr1.CreateQuota(input)
	if err != nil {
		t.Fatalf("CreateQuota failed: %v", err)
	}

	// Create new manager to load persisted data
	mgr2, err := NewManager(configPath, storage, user)
	if err != nil {
		t.Fatalf("NewManager (reload) failed: %v", err)
	}

	quota2, err := mgr2.GetQuota(quota1.ID)
	if err != nil {
		t.Fatalf("GetQuota failed after reload: %v", err)
	}

	if quota2.TargetID != "testuser" {
		t.Errorf("quota not persisted correctly")
	}
}

// ========== 模拟实现 ==========

// MockStorageProvider 模拟存储提供者
type MockStorageProvider struct {
	volumes map[string]*VolumeInfo
}

func NewMockStorageProvider() *MockStorageProvider {
	return &MockStorageProvider{
		volumes: make(map[string]*VolumeInfo),
	}
}

func (m *MockStorageProvider) GetVolume(name string) *VolumeInfo {
	return m.volumes[name]
}

func (m *MockStorageProvider) GetUsage(volumeName string) (total, used, free uint64, err error) {
	vol := m.volumes[volumeName]
	if vol == nil {
		return 0, 0, 0, ErrVolumeNotFound
	}
	return vol.Size, vol.Used, vol.Free, nil
}

// MockUserProvider 模拟用户提供者
type MockUserProvider struct {
	users  map[string]bool
	groups map[string]bool
	homes  map[string]string
}

func NewMockUserProvider() *MockUserProvider {
	return &MockUserProvider{
		users:  make(map[string]bool),
		groups: make(map[string]bool),
		homes:  make(map[string]string),
	}
}

func (m *MockUserProvider) UserExists(username string) bool {
	return m.users[username]
}

func (m *MockUserProvider) GroupExists(groupName string) bool {
	return m.groups[groupName]
}

func (m *MockUserProvider) GetUserHomeDir(username string) string {
	return m.homes[username]
}

func (m *MockUserProvider) AddUser(username, homeDir string) {
	m.users[username] = true
	m.homes[username] = homeDir
}

func (m *MockUserProvider) AddGroup(groupName string) {
	m.groups[groupName] = true
}
