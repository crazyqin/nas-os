package snapshotmgr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	config := &SnapshotConfig{
		MaxSnapshots:  5,
		RetentionDays: 30,
	}
	return NewManager(zap.NewNop(), config, tmpDir)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m, zap.NewNop())
	h.RegisterRoutes(rg)
	return r
}

func TestCreateSnapshot(t *testing.T) {
	m := setupTestManager(t)

	items := []SnapshotItem{
		{Category: "network", Key: "hostname", FilePath: "/etc/hostname", Size: 10, Checksum: "abc123"},
		{Category: "system", Key: "timezone", FilePath: "/etc/timezone", Size: 20, Checksum: "def456"},
	}

	snap, err := m.CreateSnapshot("test-snap", "test description", "manual", items)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snap.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if snap.Name != "test-snap" {
		t.Errorf("expected name 'test-snap', got %q", snap.Name)
	}
	if snap.Description != "test description" {
		t.Errorf("expected description 'test description', got %q", snap.Description)
	}
	if snap.Source != "manual" {
		t.Errorf("expected source 'manual', got %q", snap.Source)
	}
	if snap.Status != "active" {
		t.Errorf("expected status 'active', got %q", snap.Status)
	}
	if len(snap.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(snap.Items))
	}
	if snap.SizeBytes != 30 {
		t.Errorf("expected size 30, got %d", snap.SizeBytes)
	}
}

func TestListSnapshots(t *testing.T) {
	m := setupTestManager(t)

	// 创建多个快照
	for i := 0; i < 3; i++ {
		_, err := m.CreateSnapshot("snap", "desc", "manual", nil)
		if err != nil {
			t.Fatalf("CreateSnapshot failed: %v", err)
		}
	}

	snaps := m.ListSnapshots()
	if len(snaps) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(snaps))
	}

	// 验证按时间倒序
	for i := 0; i < len(snaps)-1; i++ {
		if snaps[i].CreatedAt.Before(snaps[i+1].CreatedAt) {
			t.Error("snapshots not sorted by created_at desc")
		}
	}
}

func TestGetSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, err := m.CreateSnapshot("test", "desc", "manual", []SnapshotItem{
		{Category: "system", Key: "host", FilePath: "/etc/host", Size: 5},
	})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	got, err := m.GetSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if got.ID != snap.ID {
		t.Errorf("expected ID %q, got %q", snap.ID, got.ID)
	}
	if len(got.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(got.Items))
	}

	// 不存在的 ID
	_, err = m.GetSnapshot("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, err := m.CreateSnapshot("test", "desc", "manual", nil)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if err := m.DeleteSnapshot(snap.ID); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	// 删除后应该查不到
	_, err = m.GetSnapshot(snap.ID)
	if err == nil {
		t.Error("expected error after delete")
	}

	// 删除不存在的
	err = m.DeleteSnapshot("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestRestoreSnapshot(t *testing.T) {
	m := setupTestManager(t)

	items := []SnapshotItem{
		{Category: "network", Key: "hostname", FilePath: "/etc/hostname", Size: 10},
	}
	snap, err := m.CreateSnapshot("test", "desc", "manual", items)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	restoreItems, err := m.RestoreSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	if len(restoreItems) != 1 {
		t.Errorf("expected 1 restore item, got %d", len(restoreItems))
	}

	// 恢复后状态回到 active
	got, _ := m.GetSnapshot(snap.ID)
	if got.Status != "active" {
		t.Errorf("expected status 'active' after restore, got %q", got.Status)
	}

	// 不存在的快照
	_, err = m.RestoreSnapshot("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestCleanupOld(t *testing.T) {
	m := setupTestManager(t)

	// 创建一个过期快照（手动设置时间）
	snap, err := m.CreateSnapshot("old", "desc", "manual", nil)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// 修改创建时间为 60 天前（超过 30 天保留期）
	m.mu.Lock()
	m.snapshots[snap.ID].CreatedAt = time.Now().AddDate(0, 0, -60)
	m.saveSnapshotMeta(m.snapshots[snap.ID])
	m.mu.Unlock()

	// 创建一个新快照
	_, err = m.CreateSnapshot("new", "desc", "manual", nil)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	m.CleanupOld()

	snaps := m.ListSnapshots()
	if len(snaps) != 1 {
		t.Errorf("expected 1 snapshot after cleanup, got %d", len(snaps))
	}
}

func TestMaxSnapshots(t *testing.T) {
	m := setupTestManager(t)

	// 创建到最大数量
	for i := 0; i < 5; i++ {
		_, err := m.CreateSnapshot("snap", "desc", "manual", nil)
		if err != nil {
			t.Fatalf("CreateSnapshot %d failed: %v", i, err)
		}
	}

	// 再创建应该失败
	_, err := m.CreateSnapshot("extra", "desc", "manual", nil)
	if err == nil {
		t.Error("expected error when exceeding max snapshots")
	}
}

func TestHandler_CreateSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"name":"test-snap","description":"test","source":"manual","items":[{"category":"system","key":"host","file_path":"/etc/host","size":10,"checksum":"abc"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var snap Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if snap.Name != "test-snap" {
		t.Errorf("expected name 'test-snap', got %q", snap.Name)
	}
}

func TestHandler_ListSnapshots(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 创建一个快照
	m.CreateSnapshot("test", "desc", "manual", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Snapshots []Snapshot `json:"snapshots"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(resp.Snapshots))
	}
}

func TestHandler_DeleteSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("test", "desc", "manual", nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/snapshots/"+snap.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
