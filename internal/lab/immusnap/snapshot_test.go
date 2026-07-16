package immusnap

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

func TestCreateSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, err := m.CreateSnapshot("pool/data", "/data", "/snapshots/pool-data", 48, nil)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snap.ID == "" {
		t.Error("expected non-empty ID")
	}
	if snap.DatasetName != "pool/data" {
		t.Errorf("expected dataset 'pool/data', got %q", snap.DatasetName)
	}
	if snap.Status != StatusPending {
		t.Errorf("expected status pending, got %v", snap.Status)
	}
	if snap.Locked {
		t.Error("expected locked=false for new snapshot")
	}
}

func TestCreateSnapshotEmptyDataset(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.CreateSnapshot("", "/data", "/snap", 24, nil)
	if err == nil {
		t.Error("expected error for empty dataset name")
	}
}

func TestCreateSnapshotMinRetention(t *testing.T) {
	m := setupTestManager(t)

	// 请求 1 小时，但默认最小保留 24 小时
	snap, err := m.CreateSnapshot("pool/data", "/data", "/snap", 1, nil)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// 过期时间应至少 24 小时后
	if snap.ExpiresAt.Sub(snap.CreatedAt).Hours() < 24 {
		t.Errorf("expected min retention 24h, got %v", snap.ExpiresAt.Sub(snap.CreatedAt))
	}
}

func TestLockSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 48, nil)
	err := m.Lock(snap.ID)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	locked, _ := m.GetSnapshot(snap.ID)
	if !locked.Locked {
		t.Error("expected locked=true")
	}
	if locked.Status != StatusLocked {
		t.Errorf("expected status locked, got %v", locked.Status)
	}
}

func TestLockAlreadyLocked(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 48, nil)
	m.Lock(snap.ID)

	err := m.Lock(snap.ID)
	if err == nil {
		t.Error("expected error when locking already locked snapshot")
	}
}

func TestDeleteLockedSnapshotBeforeExpiry(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 48, nil)
	m.Lock(snap.ID)

	err := m.DeleteSnapshot(snap.ID)
	if err == nil {
		t.Error("expected error when deleting locked snapshot before expiry")
	}
}

func TestDeletePendingSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 48, nil)
	// 未锁定的快照可以删除
	err := m.DeleteSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("expected no error deleting pending snapshot: %v", err)
	}
}

func TestVerifyIntegrity(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 48, nil)
	m.Lock(snap.ID)

	// 设置校验和
	checksum := "abc123def456"
	m.SetChecksum(snap.ID, checksum)

	// 验证匹配
	result, err := m.VerifyIntegrity(snap.ID, checksum)
	if err != nil {
		t.Fatalf("VerifyIntegrity failed: %v", err)
	}
	if !result.Valid {
		t.Error("expected integrity check to pass")
	}

	// 验证不匹配
	result, err = m.VerifyIntegrity(snap.ID, "wrong_checksum")
	if err != nil {
		t.Fatalf("VerifyIntegrity failed: %v", err)
	}
	if result.Valid {
		t.Error("expected integrity check to fail")
	}
}

func TestVerifyNonexistentSnapshot(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.VerifyIntegrity("nonexistent", "abc")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestGetAndUpdatePolicy(t *testing.T) {
	m := setupTestManager(t)

	policy := m.GetPolicy()
	if policy.MinRetentionHours != 24 {
		t.Errorf("expected default min retention 24, got %d", policy.MinRetentionHours)
	}

	newPolicy := RetentionPolicy{
		MinRetentionHours: 48,
		MaxSnapshots:      50,
		AutoLockOnThreat:  false,
	}
	err := m.UpdatePolicy(newPolicy)
	if err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}

	updated := m.GetPolicy()
	if updated.MinRetentionHours != 48 {
		t.Errorf("expected min retention 48, got %d", updated.MinRetentionHours)
	}
	if updated.MaxSnapshots != 50 {
		t.Errorf("expected max snapshots 50, got %d", updated.MaxSnapshots)
	}
}

func TestUpdatePolicyInvalid(t *testing.T) {
	m := setupTestManager(t)

	err := m.UpdatePolicy(RetentionPolicy{MinRetentionHours: 0, MaxSnapshots: 10})
	if err == nil {
		t.Error("expected error for zero min retention")
	}

	err = m.UpdatePolicy(RetentionPolicy{MinRetentionHours: 1, MaxSnapshots: 0})
	if err == nil {
		t.Error("expected error for zero max snapshots")
	}
}

func TestReportThreatAutoSnapshot(t *testing.T) {
	m := setupTestManager(t)

	// 默认策略允许自动锁定
	event, err := m.ReportThreat(ThreatLevelCritical, 0.8, "mass file encryption detected", "pool/data")
	if err != nil {
		t.Fatalf("ReportThreat failed: %v", err)
	}

	if event.Level != ThreatLevelCritical {
		t.Errorf("expected critical, got %v", event.Level)
	}
	if event.SnapshotID == "" {
		t.Error("expected auto-created snapshot ID")
	}

	// 验证快照已创建并锁定
	snap, err := m.GetSnapshot(event.SnapshotID)
	if err != nil {
		t.Fatalf("auto-snapshot not found: %v", err)
	}
	if !snap.Locked {
		t.Error("expected auto-snapshot to be locked")
	}
	if snap.ThreatLevel != ThreatLevelCritical {
		t.Errorf("expected threat level critical, got %v", snap.ThreatLevel)
	}
}

func TestReportThreatNoAutoWithoutDataset(t *testing.T) {
	m := setupTestManager(t)

	event, err := m.ReportThreat(ThreatLevelSuspicious, 0.5, "some alert", "")
	if err != nil {
		t.Fatalf("ReportThreat failed: %v", err)
	}

	// 没有指定数据集，不创建快照
	if event.SnapshotID != "" {
		t.Error("expected no auto-snapshot without dataset name")
	}
}

func TestMaxSnapshotLimit(t *testing.T) {
	m := setupTestManager(t)

	m.UpdatePolicy(RetentionPolicy{
		MinRetentionHours: 1,
		MaxSnapshots:      2,
		AutoLockOnThreat:  false,
	})

	_, err := m.CreateSnapshot("pool/data", "/data", "/snap1", 24, nil)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	_, err = m.CreateSnapshot("pool/data", "/data", "/snap2", 24, nil)
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}
	_, err = m.CreateSnapshot("pool/data", "/data", "/snap3", 24, nil)
	if err == nil {
		t.Error("expected error when exceeding max snapshot count")
	}
}

func TestGetStats(t *testing.T) {
	m := setupTestManager(t)

	m.CreateSnapshot("pool/a", "/a", "/snap1", 24, nil)
	snap2, _ := m.CreateSnapshot("pool/b", "/b", "/snap2", 24, nil)
	m.Lock(snap2.ID)

	stats := m.GetStats()
	if stats.TotalSnapshots != 2 {
		t.Errorf("expected 2 total snapshots, got %d", stats.TotalSnapshots)
	}
	if stats.LockedSnapshots != 1 {
		t.Errorf("expected 1 locked snapshot, got %d", stats.LockedSnapshots)
	}
}

func TestListSnapshotsWithFilter(t *testing.T) {
	m := setupTestManager(t)

	snap1, _ := m.CreateSnapshot("pool/a", "/a", "/snap1", 24, nil)
	snap2, _ := m.CreateSnapshot("pool/b", "/b", "/snap2", 24, nil)
	m.Lock(snap2.ID)

	// 无过滤
	all := m.ListSnapshots("")
	if len(all) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(all))
	}

	// 过滤 pending
	pending := m.ListSnapshots(StatusPending)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].ID != snap1.ID {
		t.Errorf("expected pending snapshot to be %s, got %s", snap1.ID, pending[0].ID)
	}

	// 过滤 locked
	locked := m.ListSnapshots(StatusLocked)
	if len(locked) != 1 {
		t.Errorf("expected 1 locked, got %d", len(locked))
	}
	if locked[0].ID != snap2.ID {
		t.Errorf("expected locked snapshot to be %s, got %s", snap2.ID, locked[0].ID)
	}
}

func TestExpireSnapshots(t *testing.T) {
	m := setupTestManager(t)

	// 手动创建一个快照并设置为已过期
	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 1, nil)
	m.Lock(snap.ID)

	// 手动设置过期时间为过去
	m.mu.Lock()
	m.snapshots[snap.ID].ExpiresAt = snap.CreatedAt.Add(-1 * time.Hour)
	m.mu.Unlock()

	count := m.ExpireSnapshots()
	if count != 1 {
		t.Errorf("expected 1 expired snapshot, got %d", count)
	}

	expired, _ := m.GetSnapshot(snap.ID)
	if expired.Status != StatusExpired {
		t.Errorf("expected status expired, got %v", expired.Status)
	}
	if expired.Locked {
		t.Error("expected expired snapshot to be unlocked")
	}
}

// === HTTP Handler 测试 ===

func TestHandlerCreateSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"dataset_name":"pool/data","source_path":"/data","retention_hours":48}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/immutable-snapshots", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandlerCreateSnapshotWithAutoLock(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"dataset_name":"pool/data","auto_lock":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/immutable-snapshots", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	if !data["locked"].(bool) {
		t.Error("expected auto-locked snapshot")
	}
}

func TestHandlerListSnapshots(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	m.CreateSnapshot("pool/a", "/a", "/snap", 24, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/immutable-snapshots", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandlerLockSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 24, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/immutable-snapshots/"+snap.ID+"/lock", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerGetPolicy(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/immutable-snapshots/policies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	if data["min_retention_hours"].(float64) != 24 {
		t.Errorf("expected default min_retention_hours=24, got %v", data["min_retention_hours"])
	}
}

func TestHandlerUpdatePolicy(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"min_retention_hours":48,"max_snapshots":50,"auto_lock_on_threat":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/immutable-snapshots/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerVerifySnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 24, nil)
	m.Lock(snap.ID)
	m.SetChecksum(snap.ID, "abc123")

	body := `{"snapshot_id":"` + snap.ID + `","checksum":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/immutable-snapshots/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerVerifySnapshotMismatch(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 24, nil)
	m.Lock(snap.ID)
	m.SetChecksum(snap.ID, "abc123")

	body := `{"snapshot_id":"` + snap.ID + `","checksum":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/immutable-snapshots/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409 for mismatch, got %d", w.Code)
	}
}

func TestHandlerGetStats(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/immutable-snapshots/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerReportThreat(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"level":"critical","modified_rate":0.9,"description":"ransomware detected","dataset_name":"pool/data"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/immutable-snapshots/threat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	if data["snapshot_id"] == nil || data["snapshot_id"] == "" {
		t.Error("expected auto-created snapshot ID in threat response")
	}
}

func TestHandlerCreateSnapshotInvalidBody(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/immutable-snapshots", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetThreatEvents(t *testing.T) {
	m := setupTestManager(t)

	m.ReportThreat(ThreatLevelSuspicious, 0.5, "test event", "pool/data")
	m.ReportThreat(ThreatLevelCritical, 0.9, "critical event", "pool/data")

	events := m.GetThreatEvents()
	if len(events) != 2 {
		t.Errorf("expected 2 threat events, got %d", len(events))
	}
}

func TestSetSize(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "/data", "/snap", 24, nil)
	err := m.SetSize(snap.ID, 1024*1024)
	if err != nil {
		t.Fatalf("SetSize failed: %v", err)
	}

	updated, _ := m.GetSnapshot(snap.ID)
	if updated.Size != 1024*1024 {
		t.Errorf("expected size 1048576, got %d", updated.Size)
	}
}

func TestGenerateChecksum(t *testing.T) {
	data := []byte("hello world")
	hash := GenerateChecksum(data)
	if hash == "" {
		t.Error("expected non-empty checksum")
	}
	if len(hash) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("expected 64 char hash, got %d", len(hash))
	}
}
