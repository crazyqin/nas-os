package writeonce

import (
	"fmt"
	"testing"
	"time"
)

func TestCreateFolder(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:              true,
		DefaultRetentionDays: 30,
		MaxRetentionDays:     365,
		AllowForeverLock:     true,
		ComplianceMode:       false,
		AuditLogEnabled:      true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Test Folder",
		Path:          "/data/test",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		PolicyMode:    PolicyModeEnterprise,
		CreatedBy:     "admin",
	}

	folder, err := m.CreateFolder(req)
	if err != nil {
		t.Fatalf("Failed to create folder: %v", err)
	}

	if folder.Name != "Test Folder" {
		t.Errorf("Expected name 'Test Folder', got '%s'", folder.Name)
	}

	if folder.State != FolderStateOpen {
		t.Errorf("Expected state 'open', got '%s'", folder.State)
	}

	if folder.RetentionMode != RetentionModeFixed {
		t.Errorf("Expected retention mode 'fixed', got '%s'", folder.RetentionMode)
	}

	if folder.RetentionDays != 30 {
		t.Errorf("Expected retention days 30, got %d", folder.RetentionDays)
	}
}

func TestCreateFolderDisabled(t *testing.T) {
	config := WriteOnceConfig{
		Enabled: false,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Test Folder",
		Path:          "/data/test",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	_, err := m.CreateFolder(req)
	if err == nil {
		t.Error("Expected error when WriteOnce is disabled")
	}
}

func TestCreateFolderValidation(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:              true,
		DefaultRetentionDays: 30,
		MaxRetentionDays:     365,
		AllowForeverLock:     true,
		AuditLogEnabled:      true,
	}

	m := NewManager(config)

	// Test missing name
	req := CreateFolderRequest{
		Path:          "/data/test",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	_, err := m.CreateFolder(req)
	if err == nil {
		t.Error("Expected error for missing name")
	}

	// Test missing path
	req = CreateFolderRequest{
		Name:          "Test",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	_, err = m.CreateFolder(req)
	if err == nil {
		t.Error("Expected error for missing path")
	}

	// Test fixed retention without days
	req = CreateFolderRequest{
		Name:          "Test",
		Path:          "/data/test",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 0,
		CreatedBy:     "admin",
	}

	_, err = m.CreateFolder(req)
	if err == nil {
		t.Error("Expected error for fixed retention without days")
	}

	// Test retention days exceeding max
	req = CreateFolderRequest{
		Name:          "Test",
		Path:          "/data/test",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 400,
		CreatedBy:     "admin",
	}

	_, err = m.CreateFolder(req)
	if err == nil {
		t.Error("Expected error for retention days exceeding max")
	}
}

func TestCreateForeverFolder(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:          true,
		AllowForeverLock: true,
		AuditLogEnabled:  true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Forever Folder",
		Path:          "/data/forever",
		RetentionMode: RetentionModeForever,
		PolicyMode:    PolicyModeEnterprise,
		CreatedBy:     "admin",
	}

	folder, err := m.CreateFolder(req)
	if err != nil {
		t.Fatalf("Failed to create forever folder: %v", err)
	}

	if folder.RetentionMode != RetentionModeForever {
		t.Errorf("Expected retention mode 'forever', got '%s'", folder.RetentionMode)
	}
}

func TestCreateForeverFolderNotAllowed(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:          true,
		AllowForeverLock: false,
		AuditLogEnabled:  true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Forever Folder",
		Path:          "/data/forever",
		RetentionMode: RetentionModeForever,
		CreatedBy:     "admin",
	}

	_, err := m.CreateFolder(req)
	if err == nil {
		t.Error("Expected error when forever lock is not allowed")
	}
}

func TestLockFolder(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:          true,
		AllowForeverLock: true,
		AuditLogEnabled:  true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Lock Test",
		Path:          "/data/lock",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)

	err := m.LockFolder(folder.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to lock folder: %v", err)
	}

	locked, _ := m.GetFolder(folder.ID)
	if locked.State != FolderStateLocked {
		t.Errorf("Expected state 'locked', got '%s'", locked.State)
	}

	if locked.LockedAt == nil {
		t.Error("Expected LockedAt to be set")
	}

	if locked.ExpiresAt == nil {
		t.Error("Expected ExpiresAt to be set for fixed retention")
	}
}

func TestLockFolderAlreadyLocked(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:          true,
		AllowForeverLock: true,
		AuditLogEnabled:  true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Double Lock Test",
		Path:          "/data/double",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)
	m.LockFolder(folder.ID, "admin")

	err := m.LockFolder(folder.ID, "admin")
	if err == nil {
		t.Error("Expected error when locking already locked folder")
	}
}

func TestLockFolderNotFound(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	err := m.LockFolder("nonexistent", "admin")
	if err == nil {
		t.Error("Expected error for nonexistent folder")
	}
}

func TestAddFile(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "File Test",
		Path:          "/data/files",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)

	addReq := AddFileRequest{
		FolderID:   folder.ID,
		FilePath:   "/data/files/test.txt",
		FileName:   "test.txt",
		FileSize:   1024,
		FileHash:   "abc123",
		UploadedBy: "user1",
	}

	file, err := m.AddFile(addReq)
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	if file.FileName != "test.txt" {
		t.Errorf("Expected file name 'test.txt', got '%s'", file.FileName)
	}

	if file.FileSize != 1024 {
		t.Errorf("Expected file size 1024, got %d", file.FileSize)
	}

	// Check folder stats updated
	updated, _ := m.GetFolder(folder.ID)
	if updated.FileCount != 1 {
		t.Errorf("Expected file count 1, got %d", updated.FileCount)
	}

	if updated.TotalSizeBytes != 1024 {
		t.Errorf("Expected total size 1024, got %d", updated.TotalSizeBytes)
	}
}

func TestAddFileToLockedFolder(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Locked Folder",
		Path:          "/data/locked",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)
	m.LockFolder(folder.ID, "admin")

	addReq := AddFileRequest{
		FolderID:   folder.ID,
		FilePath:   "/data/locked/test.txt",
		FileName:   "test.txt",
		FileSize:   1024,
		FileHash:   "abc123",
		UploadedBy: "user1",
	}

	_, err := m.AddFile(addReq)
	if err == nil {
		t.Error("Expected error when adding file to locked folder")
	}
}

func TestAddDuplicateFile(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Duplicate Test",
		Path:          "/data/dup",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)

	addReq := AddFileRequest{
		FolderID:   folder.ID,
		FilePath:   "/data/dup/test.txt",
		FileName:   "test.txt",
		FileSize:   1024,
		FileHash:   "abc123",
		UploadedBy: "user1",
	}

	m.AddFile(addReq)

	_, err := m.AddFile(addReq)
	if err == nil {
		t.Error("Expected error when adding duplicate file")
	}
}

func TestPreventDelete(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Delete Test",
		Path:          "/data/delete",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)

	// Open folder allows delete
	err := m.PreventDelete(folder.ID, "test.txt", "user1")
	if err != nil {
		t.Errorf("Expected no error for open folder: %v", err)
	}

	// Lock folder
	m.LockFolder(folder.ID, "admin")

	// Locked folder prevents delete
	err = m.PreventDelete(folder.ID, "test.txt", "user1")
	if err == nil {
		t.Error("Expected error when deleting from locked folder")
	}
}

func TestPreventModify(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Modify Test",
		Path:          "/data/modify",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)

	// Open folder allows modify
	err := m.PreventModify(folder.ID, "test.txt", "user1")
	if err != nil {
		t.Errorf("Expected no error for open folder: %v", err)
	}

	// Lock folder
	m.LockFolder(folder.ID, "admin")

	// Locked folder prevents modify
	err = m.PreventModify(folder.ID, "test.txt", "user1")
	if err == nil {
		t.Error("Expected error when modifying in locked folder")
	}
}

func TestExpiredFolderAllowsOperations(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Expired Test",
		Path:          "/data/expired",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 1, // 1 day
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)
	m.LockFolder(folder.ID, "admin")

	// Manually set expiry to past
	m.mu.Lock()
	f := m.folders[folder.ID]
	past := time.Now().Add(-24 * time.Hour)
	f.ExpiresAt = &past
	m.mu.Unlock()

	// Check expiry
	expired := m.CheckExpiry()
	if expired != 1 {
		t.Errorf("Expected 1 expired folder, got %d", expired)
	}

	// Expired folder allows delete
	err := m.PreventDelete(folder.ID, "test.txt", "user1")
	if err != nil {
		t.Errorf("Expected no error for expired folder: %v", err)
	}

	// Expired folder allows modify
	err = m.PreventModify(folder.ID, "test.txt", "user1")
	if err != nil {
		t.Errorf("Expected no error for expired folder: %v", err)
	}
}

func TestGetFiles(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Files Test",
		Path:          "/data/files",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)

	// Add multiple files
	for i := 0; i < 3; i++ {
		addReq := AddFileRequest{
			FolderID:   folder.ID,
			FilePath:   fmt.Sprintf("/data/files/file%d.txt", i),
			FileName:   fmt.Sprintf("file%d.txt", i),
			FileSize:   int64(100 * (i + 1)),
			FileHash:   fmt.Sprintf("hash%d", i),
			UploadedBy: "user1",
		}
		m.AddFile(addReq)
	}

	files, err := m.GetFiles(folder.ID)
	if err != nil {
		t.Fatalf("Failed to get files: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}
}

func TestGetFilesNotFound(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	_, err := m.GetFiles("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent folder")
	}
}

func TestListFolders(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	for i := 0; i < 3; i++ {
		req := CreateFolderRequest{
			Name:          fmt.Sprintf("Folder %d", i),
			Path:          fmt.Sprintf("/data/folder%d", i),
			RetentionMode: RetentionModeFixed,
			RetentionDays: 30,
			CreatedBy:     "admin",
		}
		m.CreateFolder(req)
	}

	folders := m.ListFolders()
	if len(folders) != 3 {
		t.Errorf("Expected 3 folders, got %d", len(folders))
	}
}

func TestAuditLog(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Audit Test",
		Path:          "/data/audit",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)
	m.LockFolder(folder.ID, "admin")

	auditLog := m.GetAuditLog(folder.ID)
	if len(auditLog) < 2 {
		t.Errorf("Expected at least 2 audit entries, got %d", len(auditLog))
	}

	// Check audit entry content
	foundCreated := false
	foundLocked := false
	for _, entry := range auditLog {
		if entry.Action == "folder_created" {
			foundCreated = true
		}
		if entry.Action == "folder_locked" {
			foundLocked = true
		}
	}

	if !foundCreated {
		t.Error("Expected 'folder_created' audit entry")
	}

	if !foundLocked {
		t.Error("Expected 'folder_locked' audit entry")
	}
}

func TestGetAllAuditLog(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	for i := 0; i < 2; i++ {
		req := CreateFolderRequest{
			Name:          fmt.Sprintf("Audit Folder %d", i),
			Path:          fmt.Sprintf("/data/audit%d", i),
			RetentionMode: RetentionModeFixed,
			RetentionDays: 30,
			CreatedBy:     "admin",
		}
		m.CreateFolder(req)
	}

	allAudit := m.GetAllAuditLog()
	if len(allAudit) < 2 {
		t.Errorf("Expected at least 2 audit entries, got %d", len(allAudit))
	}
}

func TestAuditLogDisabled(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: false,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "No Audit",
		Path:          "/data/noaudit",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	m.CreateFolder(req)

	allAudit := m.GetAllAuditLog()
	if len(allAudit) != 0 {
		t.Errorf("Expected 0 audit entries, got %d", len(allAudit))
	}
}

func TestCheckExpiry(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	// Create folder with 1 day retention
	req := CreateFolderRequest{
		Name:          "Expiry Test",
		Path:          "/data/expiry",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 1,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)
	m.LockFolder(folder.ID, "admin")

	// Manually set expiry to past
	m.mu.Lock()
	f := m.folders[folder.ID]
	past := time.Now().Add(-24 * time.Hour)
	f.ExpiresAt = &past
	m.mu.Unlock()

	expired := m.CheckExpiry()
	if expired != 1 {
		t.Errorf("Expected 1 expired folder, got %d", expired)
	}

	updated, _ := m.GetFolder(folder.ID)
	if updated.State != FolderStateExpired {
		t.Errorf("Expected state 'expired', got '%s'", updated.State)
	}
}

func TestGetStats(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	// Create mixed folders
	req1 := CreateFolderRequest{
		Name:          "Open Folder",
		Path:          "/data/open",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	req2 := CreateFolderRequest{
		Name:          "Locked Folder",
		Path:          "/data/locked",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder1, _ := m.CreateFolder(req1)
	folder2, _ := m.CreateFolder(req2)

	// Add files before locking
	m.AddFile(AddFileRequest{
		FolderID:   folder1.ID,
		FilePath:   "/data/open/test.txt",
		FileName:   "test.txt",
		FileSize:   1024,
		FileHash:   "hash1",
		UploadedBy: "user1",
	})

	m.AddFile(AddFileRequest{
		FolderID:   folder2.ID,
		FilePath:   "/data/locked/test2.txt",
		FileName:   "test2.txt",
		FileSize:   2048,
		FileHash:   "hash2",
		UploadedBy: "user1",
	})

	// Lock folder2 after adding files
	m.LockFolder(folder2.ID, "admin")

	stats := m.GetStats()

	if stats["total_folders"] != 2 {
		t.Errorf("Expected 2 total folders, got %v", stats["total_folders"])
	}

	if stats["open_folders"] != 1 {
		t.Errorf("Expected 1 open folder, got %v", stats["open_folders"])
	}

	if stats["locked_folders"] != 1 {
		t.Errorf("Expected 1 locked folder, got %v", stats["locked_folders"])
	}

	if stats["total_files"] != int64(2) {
		t.Errorf("Expected 2 total files, got %v", stats["total_files"])
	}

	if stats["total_size_bytes"] != int64(3072) {
		t.Errorf("Expected 3072 total size bytes, got %v", stats["total_size_bytes"])
	}
}

func TestConfigOperations(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:              true,
		DefaultRetentionDays: 30,
		MaxRetentionDays:     365,
		AllowForeverLock:     true,
		ComplianceMode:       false,
		AuditLogEnabled:      true,
	}

	m := NewManager(config)

	current := m.GetConfig()
	if !current.Enabled {
		t.Error("Expected WriteOnce to be enabled")
	}

	newConfig := WriteOnceConfig{
		Enabled:              false,
		DefaultRetentionDays: 60,
		MaxRetentionDays:     730,
		AllowForeverLock:     false,
		ComplianceMode:       true,
		AuditLogEnabled:      false,
	}

	m.UpdateConfig(newConfig)

	updated := m.GetConfig()
	if updated.Enabled {
		t.Error("Expected WriteOnce to be disabled")
	}

	if updated.DefaultRetentionDays != 60 {
		t.Errorf("Expected default retention 60, got %d", updated.DefaultRetentionDays)
	}

	if !updated.ComplianceMode {
		t.Error("Expected compliance mode to be enabled")
	}
}

func TestComplianceMode(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:          true,
		ComplianceMode:   true,
		AllowForeverLock: true,
		AuditLogEnabled:  true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Compliance Folder",
		Path:          "/data/compliance",
		RetentionMode: RetentionModeForever,
		PolicyMode:    PolicyModeCompliance,
		CreatedBy:     "admin",
	}

	folder, err := m.CreateFolder(req)
	if err != nil {
		t.Fatalf("Failed to create compliance folder: %v", err)
	}

	if folder.PolicyMode != PolicyModeCompliance {
		t.Errorf("Expected policy mode 'compliance', got '%s'", folder.PolicyMode)
	}
}

func TestDuplicatePath(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "First Folder",
		Path:          "/data/same",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	m.CreateFolder(req)

	req2 := CreateFolderRequest{
		Name:          "Second Folder",
		Path:          "/data/same",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	_, err := m.CreateFolder(req2)
	if err == nil {
		t.Error("Expected error for duplicate path")
	}
}

func TestInvalidRetentionMode(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Invalid Mode",
		Path:          "/data/invalid",
		RetentionMode: "invalid_mode",
		CreatedBy:     "admin",
	}

	_, err := m.CreateFolder(req)
	if err == nil {
		t.Error("Expected error for invalid retention mode")
	}
}

func TestInvalidPolicyMode(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Invalid Policy",
		Path:          "/data/policy",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		PolicyMode:    "invalid_policy",
		CreatedBy:     "admin",
	}

	_, err := m.CreateFolder(req)
	if err == nil {
		t.Error("Expected error for invalid policy mode")
	}
}

func TestConcurrentOperations(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	done := make(chan bool, 20)

	// Concurrent folder creation
	for i := 0; i < 10; i++ {
		go func(idx int) {
			req := CreateFolderRequest{
				Name:          fmt.Sprintf("Concurrent Folder %d", idx),
				Path:          fmt.Sprintf("/data/concurrent%d", idx),
				RetentionMode: RetentionModeFixed,
				RetentionDays: 30,
				CreatedBy:     "admin",
			}
			m.CreateFolder(req)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	folders := m.ListFolders()
	if len(folders) != 10 {
		t.Errorf("Expected 10 folders, got %d", len(folders))
	}
}

func TestMultipleFilesInFolder(t *testing.T) {
	config := WriteOnceConfig{
		Enabled:         true,
		AuditLogEnabled: true,
	}

	m := NewManager(config)

	req := CreateFolderRequest{
		Name:          "Multi File",
		Path:          "/data/multi",
		RetentionMode: RetentionModeFixed,
		RetentionDays: 30,
		CreatedBy:     "admin",
	}

	folder, _ := m.CreateFolder(req)

	for i := 0; i < 5; i++ {
		addReq := AddFileRequest{
			FolderID:   folder.ID,
			FilePath:   fmt.Sprintf("/data/multi/file%d.txt", i),
			FileName:   fmt.Sprintf("file%d.txt", i),
			FileSize:   int64(100 * (i + 1)),
			FileHash:   fmt.Sprintf("hash%d", i),
			UploadedBy: "user1",
		}
		_, err := m.AddFile(addReq)
		if err != nil {
			t.Fatalf("Failed to add file %d: %v", i, err)
		}
	}

	files, _ := m.GetFiles(folder.ID)
	if len(files) != 5 {
		t.Errorf("Expected 5 files, got %d", len(files))
	}

	updated, _ := m.GetFolder(folder.ID)
	if updated.FileCount != 5 {
		t.Errorf("Expected file count 5, got %d", updated.FileCount)
	}

	expectedSize := int64(100 + 200 + 300 + 400 + 500)
	if updated.TotalSizeBytes != expectedSize {
		t.Errorf("Expected total size %d, got %d", expectedSize, updated.TotalSizeBytes)
	}
}
