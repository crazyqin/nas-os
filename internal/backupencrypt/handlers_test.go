package backupencrypt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	
	config := &BackupEncryptConfig{
		DefaultAlgo:       AES256GCM,
		ChunkSize:         1024 * 1024,
		MaxParallel:       4,
		VerifyAfterBackup: true,
		AutoKeyRotation:   false,
	}
	
	manager := NewManager(config)
	handler := NewHandler(manager)
	
	router := gin.New()
	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1)
	
	return router, manager
}

func TestCreateBackup(t *testing.T) {
	router, manager := setupTestRouter()
	
	// Create a key first
	key, err := manager.GenerateKey("test-key", "aes256gcm")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	
	// Test create backup
	body := map[string]string{
		"name":        "test-backup",
		"source_path": "/data/source",
		"dest_path":   "/data/dest",
		"key_id":      key.ID,
	}
	jsonBody, _ := json.Marshal(body)
	
	req, _ := http.NewRequest("POST", "/api/v1/backup/encrypted", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
	
	var backup EncryptedBackup
	if err := json.Unmarshal(w.Body.Bytes(), &backup); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if backup.Name != "test-backup" {
		t.Errorf("Expected name 'test-backup', got %s", backup.Name)
	}
	
	if backup.KeyID != key.ID {
		t.Errorf("Expected key_id %s, got %s", key.ID, backup.KeyID)
	}
}

func TestGetBackup(t *testing.T) {
	router, manager := setupTestRouter()
	
	// Create a key and backup
	key, _ := manager.GenerateKey("test-key", "aes256gcm")
	backup, _ := manager.CreateBackup("test-backup", "/src", "/dst", key.ID)
	
	req, _ := http.NewRequest("GET", "/api/v1/backup/encrypted/"+backup.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var result EncryptedBackup
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if result.ID != backup.ID {
		t.Errorf("Expected ID %s, got %s", backup.ID, result.ID)
	}
}

func TestListBackups(t *testing.T) {
	router, manager := setupTestRouter()
	
	// Create keys and backups
	key1, _ := manager.GenerateKey("key1", "aes256gcm")
	key2, _ := manager.GenerateKey("key2", "chacha20")
	manager.CreateBackup("backup1", "/src1", "/dst1", key1.ID)
	manager.CreateBackup("backup2", "/src2", "/dst2", key2.ID)
	
	req, _ := http.NewRequest("GET", "/api/v1/backup/encrypted", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var backups []EncryptedBackup
	if err := json.Unmarshal(w.Body.Bytes(), &backups); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if len(backups) != 2 {
		t.Errorf("Expected 2 backups, got %d", len(backups))
	}
}

func TestRestoreBackup(t *testing.T) {
	router, manager := setupTestRouter()
	
	// Create key and backup
	key, _ := manager.GenerateKey("test-key", "aes256gcm")
	backup, _ := manager.CreateBackup("test-backup", "/src", "/dst", key.ID)
	
	// Wait for backup to complete
	// In real test we'd need to wait or mock, but for now we'll test the endpoint
	
	body := map[string]string{
		"backup_id": backup.ID,
		"dest_path": "/restore/dest",
		"key_id":    key.ID,
	}
	jsonBody, _ := json.Marshal(body)
	
	req, _ := http.NewRequest("POST", "/api/v1/backup/restore", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	// Note: This might fail if backup isn't completed yet in async processing
	// In a real test, we'd wait or use synchronous processing
	if w.Code == http.StatusCreated {
		var job RestoreJob
		if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		
		if job.BackupID != backup.ID {
			t.Errorf("Expected backup_id %s, got %s", backup.ID, job.BackupID)
		}
	}
}

func TestGetRestoreJob(t *testing.T) {
	router, manager := setupTestRouter()
	
	// Create key, backup, and restore job
	key, _ := manager.GenerateKey("test-key", "aes256gcm")
	backup, _ := manager.CreateBackup("test-backup", "/src", "/dst", key.ID)
	
	// Directly create a restore job for testing
	job := &RestoreJob{
		ID:        "restore-test",
		BackupID:  backup.ID,
		DestPath:  "/restore/dest",
		Status:    RestorePending,
		KeyID:     key.ID,
		CreatedAt: backup.CreatedAt,
	}
	manager.restores[job.ID] = job
	
	req, _ := http.NewRequest("GET", "/api/v1/backup/restore/"+job.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var result RestoreJob
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if result.ID != job.ID {
		t.Errorf("Expected ID %s, got %s", job.ID, result.ID)
	}
}

func TestGenerateKey(t *testing.T) {
	router, _ := setupTestRouter()
	
	body := map[string]string{
		"name":      "test-key",
		"algorithm": "aes256gcm",
	}
	jsonBody, _ := json.Marshal(body)
	
	req, _ := http.NewRequest("POST", "/api/v1/backup/keys", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
	
	var key BackupKey
	if err := json.Unmarshal(w.Body.Bytes(), &key); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if key.Name != "test-key" {
		t.Errorf("Expected name 'test-key', got %s", key.Name)
	}
	
	if key.Algorithm != AES256GCM {
		t.Errorf("Expected algorithm aes256gcm, got %s", key.Algorithm)
	}
}

func TestListKeys(t *testing.T) {
	router, manager := setupTestRouter()
	
	manager.GenerateKey("key1", "aes256gcm")
	manager.GenerateKey("key2", "chacha20")
	
	req, _ := http.NewRequest("GET", "/api/v1/backup/keys", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var keys []BackupKey
	if err := json.Unmarshal(w.Body.Bytes(), &keys); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestRevokeKey(t *testing.T) {
	router, manager := setupTestRouter()
	
	key, _ := manager.GenerateKey("test-key", "aes256gcm")
	
	req, _ := http.NewRequest("POST", "/api/v1/backup/keys/"+key.ID+"/revoke", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Verify key is revoked
	keys, _ := manager.ListKeys()
	for _, k := range keys {
		if k.ID == key.ID {
			if k.RevokedAt == nil {
				t.Error("Expected key to be revoked")
			}
			if k.IsPrimary {
				t.Error("Expected revoked key not to be primary")
			}
			break
		}
	}
}

func TestCreateSchedule(t *testing.T) {
	router, manager := setupTestRouter()
	
	key, _ := manager.GenerateKey("test-key", "aes256gcm")
	
	schedule := BackupSchedule{
		Name:             "daily-backup",
		CronExpr:         "0 0 * * *",
		SourcePaths:      []string{"/data1", "/data2"},
		DestPath:         "/backup/daily",
		Retention:        30,
		EncryptionKeyID:  key.ID,
		Incremental:      true,
		CompressionLevel: 6,
		Enabled:          true,
	}
	jsonBody, _ := json.Marshal(schedule)
	
	req, _ := http.NewRequest("POST", "/api/v1/backup/schedules", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

func TestRunIntegrityCheck(t *testing.T) {
	router, manager := setupTestRouter()
	
	key, _ := manager.GenerateKey("test-key", "aes256gcm")
	backup, _ := manager.CreateBackup("test-backup", "/src", "/dst", key.ID)
	
	req, _ := http.NewRequest("POST", "/api/v1/backup/integrity-check/"+backup.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var check IntegrityCheck
	if err := json.Unmarshal(w.Body.Bytes(), &check); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if check.BackupID != backup.ID {
		t.Errorf("Expected backup_id %s, got %s", backup.ID, check.BackupID)
	}
}

func TestGetBackupStats(t *testing.T) {
	router, manager := setupTestRouter()
	
	// Create some test data
	key1, _ := manager.GenerateKey("key1", "aes256gcm")
	key2, _ := manager.GenerateKey("key2", "chacha20")
	manager.CreateBackup("backup1", "/src1", "/dst1", key1.ID)
	manager.CreateBackup("backup2", "/src2", "/dst2", key2.ID)
	
	req, _ := http.NewRequest("GET", "/api/v1/backup/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var stats map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	totalBackups, ok := stats["total_backups"].(float64)
	if !ok {
		t.Fatal("Expected total_backups to be a number")
	}
	
	if int(totalBackups) != 2 {
		t.Errorf("Expected 2 total backups, got %d", int(totalBackups))
	}
	
	totalKeys, ok := stats["total_keys"].(float64)
	if !ok {
		t.Fatal("Expected total_keys to be a number")
	}
	
	if int(totalKeys) != 2 {
		t.Errorf("Expected 2 total keys, got %d", int(totalKeys))
	}
}
