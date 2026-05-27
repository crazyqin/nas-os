package filesnapmgr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	config := DefaultSnapshotConfig()
	config.TempDir = t.TempDir()
	config.MountBaseDir = t.TempDir()
	config.CloneBaseDir = t.TempDir()
	return NewManager(zap.NewNop(), config)
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

	snap, err := m.CreateSnapshot("pool/data", "test-snap", "test snapshot", SnapshotTypeZFS, nil)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snap.ID == "" {
		t.Error("expected non-empty ID")
	}
	if snap.Name != "test-snap" {
		t.Errorf("expected name 'test-snap', got %q", snap.Name)
	}
	if snap.Volume != "pool/data" {
		t.Errorf("expected volume 'pool/data', got %q", snap.Volume)
	}
	if snap.Type != SnapshotTypeZFS {
		t.Errorf("expected type zfs, got %v", snap.Type)
	}
	if snap.Status != SnapshotStatusActive {
		t.Errorf("expected status active, got %v", snap.Status)
	}
}

func TestCreateSnapshotEmptyVolume(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.CreateSnapshot("", "test", "test", SnapshotTypeZFS, nil)
	if err == nil {
		t.Error("expected error for empty volume")
	}
}

func TestListSnapshots(t *testing.T) {
	m := setupTestManager(t)

	// 创建多个快照
	m.CreateSnapshot("pool/data", "snap1", "desc1", SnapshotTypeZFS, nil)
	m.CreateSnapshot("pool/data", "snap2", "desc2", SnapshotTypeZFS, nil)
	m.CreateSnapshot("pool/backup", "snap3", "desc3", SnapshotTypeBtrfs, nil)

	// 列出所有
	all := m.ListSnapshots("", "")
	if len(all) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(all))
	}

	// 按卷过滤
	poolData := m.ListSnapshots("pool/data", "")
	if len(poolData) != 2 {
		t.Errorf("expected 2 snapshots for pool/data, got %d", len(poolData))
	}

	// 按类型过滤
	zfs := m.ListSnapshots("", SnapshotTypeZFS)
	if len(zfs) != 2 {
		t.Errorf("expected 2 ZFS snapshots, got %d", len(zfs))
	}
}

func TestGetSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	got, err := m.GetSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if got.ID != snap.ID {
		t.Errorf("expected ID %q, got %q", snap.ID, got.ID)
	}

	// 不存在的快照
	_, err = m.GetSnapshot("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	// 删除快照
	err := m.DeleteSnapshot(snap.ID, false)
	if err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	// 删除后应该查不到
	_, err = m.GetSnapshot(snap.ID)
	if err == nil {
		t.Error("expected error after delete")
	}

	// 重复删除
	err = m.DeleteSnapshot(snap.ID, false)
	if err == nil {
		t.Error("expected error for already deleted snapshot")
	}
}

func TestDeleteSnapshotWithChildren(t *testing.T) {
	m := setupTestManager(t)

	parent, _ := m.CreateSnapshot("pool/data", "parent", "desc", SnapshotTypeZFS, nil)

	// 克隆创建子快照
	m.CloneSnapshot(&CloneRequest{
		SnapshotID: parent.ID,
		CloneName:  "child",
	})

	// 不强制删除应该失败
	err := m.DeleteSnapshot(parent.ID, false)
	if err == nil {
		t.Error("expected error when deleting snapshot with children without force")
	}

	// 强制删除
	err = m.DeleteSnapshot(parent.ID, true)
	if err != nil {
		t.Fatalf("force delete failed: %v", err)
	}
}

func TestMountUnmountSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)
	mountPoint := t.TempDir() + "/mount"

	// 挂载
	err := m.MountSnapshot(&MountRequest{
		SnapshotID: snap.ID,
		MountPoint: mountPoint,
		ReadOnly:   true,
	})
	if err != nil {
		t.Fatalf("MountSnapshot failed: %v", err)
	}

	// 检查状态
	got, _ := m.GetSnapshot(snap.ID)
	if got.Status != SnapshotStatusMounted {
		t.Errorf("expected status mounted, got %v", got.Status)
	}

	// 重复挂载应该失败
	err = m.MountSnapshot(&MountRequest{
		SnapshotID: snap.ID,
		MountPoint: mountPoint,
	})
	if err == nil {
		t.Error("expected error for already mounted snapshot")
	}

	// 卸载
	err = m.UnmountSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("UnmountSnapshot failed: %v", err)
	}

	// 检查状态
	got, _ = m.GetSnapshot(snap.ID)
	if got.Status != SnapshotStatusActive {
		t.Errorf("expected status active after unmount, got %v", got.Status)
	}
}

func TestRollbackSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	// 回滚（带备份）
	result, err := m.RollbackSnapshot(&RollbackRequest{
		SnapshotID:     snap.ID,
		CreateSnapshot: true,
	})
	if err != nil {
		t.Fatalf("RollbackSnapshot failed: %v", err)
	}

	if result.SnapshotID != snap.ID {
		t.Errorf("expected snapshot ID %q, got %q", snap.ID, result.SnapshotID)
	}
	if result.BackupID == "" {
		t.Error("expected non-empty backup ID")
	}

	// 检查备份快照存在
	backup, err := m.GetSnapshot(result.BackupID)
	if err != nil {
		t.Fatalf("backup snapshot not found: %v", err)
	}
	if backup.Metadata["reason"] != "pre-rollback-backup" {
		t.Errorf("expected reason 'pre-rollback-backup', got %q", backup.Metadata["reason"])
	}
}

func TestCloneSnapshot(t *testing.T) {
	m := setupTestManager(t)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	// 克隆
	result, err := m.CloneSnapshot(&CloneRequest{
		SnapshotID: snap.ID,
		CloneName:  "test-clone",
	})
	if err != nil {
		t.Fatalf("CloneSnapshot failed: %v", err)
	}

	if result.CloneID == "" {
		t.Error("expected non-empty clone ID")
	}
	if result.CloneName != "test-clone" {
		t.Errorf("expected clone name 'test-clone', got %q", result.CloneName)
	}

	// 检查克隆存在
	clone, err := m.GetSnapshot(result.CloneID)
	if err != nil {
		t.Fatalf("clone not found: %v", err)
	}
	if clone.ParentID != snap.ID {
		t.Errorf("expected parent ID %q, got %q", snap.ID, clone.ParentID)
	}

	// 检查父快照的子列表
	parent, _ := m.GetSnapshot(snap.ID)
	found := false
	for _, childID := range parent.ChildrenIDs {
		if childID == result.CloneID {
			found = true
			break
		}
	}
	if !found {
		t.Error("clone ID not found in parent's children list")
	}
}

func TestDiffSnapshots(t *testing.T) {
	m := setupTestManager(t)

	snap1, _ := m.CreateSnapshot("pool/data", "snap1", "desc", SnapshotTypeZFS, nil)
	snap2, _ := m.CreateSnapshot("pool/data", "snap2", "desc", SnapshotTypeZFS, nil)

	result, err := m.DiffSnapshots(snap1.ID, snap2.ID)
	if err != nil {
		t.Fatalf("DiffSnapshots failed: %v", err)
	}

	if result.Snapshot1ID != snap1.ID {
		t.Errorf("expected snapshot1 ID %q, got %q", snap1.ID, result.Snapshot1ID)
	}
	if result.Snapshot2ID != snap2.ID {
		t.Errorf("expected snapshot2 ID %q, got %q", snap2.ID, result.Snapshot2ID)
	}
}

func TestPolicyManagement(t *testing.T) {
	m := setupTestManager(t)

	// 创建策略
	retention := Retention{
		MaxCount:   10,
		MaxAgeDays: 30,
		MinKeep:    2,
	}

	policy, err := m.CreatePolicy("daily-backup", "pool/data", SnapshotTypeZFS, "0 0 * * *", retention)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	if policy.ID == "" {
		t.Error("expected non-empty policy ID")
	}
	if !policy.Enabled {
		t.Error("expected policy to be enabled")
	}

	// 列出策略
	policies := m.ListPolicies()
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}

	// 获取策略
	got, err := m.GetPolicy(policy.ID)
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if got.Name != "daily-backup" {
		t.Errorf("expected name 'daily-backup', got %q", got.Name)
	}

	// 更新策略
	enabled := false
	err = m.UpdatePolicy(policy.ID, &enabled, nil, nil)
	if err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}

	got, _ = m.GetPolicy(policy.ID)
	if got.Enabled {
		t.Error("expected policy to be disabled")
	}

	// 删除策略
	err = m.DeletePolicy(policy.ID)
	if err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	_, err = m.GetPolicy(policy.ID)
	if err == nil {
		t.Error("expected error for deleted policy")
	}
}

func TestInvalidCronSchedule(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.CreatePolicy("test", "pool/data", SnapshotTypeZFS, "invalid", DefaultRetention())
	if err == nil {
		t.Error("expected error for invalid cron schedule")
	}
}

func TestExecutePolicy(t *testing.T) {
	m := setupTestManager(t)

	policy, _ := m.CreatePolicy("test", "pool/data", SnapshotTypeZFS, "0 0 * * *", DefaultRetention())

	snap, err := m.ExecutePolicy(policy.ID)
	if err != nil {
		t.Fatalf("ExecutePolicy failed: %v", err)
	}

	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.Volume != "pool/data" {
		t.Errorf("expected volume 'pool/data', got %q", snap.Volume)
	}

	// 检查策略运行计数
	got, _ := m.GetPolicy(policy.ID)
	if got.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", got.RunCount)
	}
}

func TestGetStats(t *testing.T) {
	m := setupTestManager(t)

	m.CreateSnapshot("pool/data", "snap1", "desc", SnapshotTypeZFS, nil)
	m.CreateSnapshot("pool/data", "snap2", "desc", SnapshotTypeZFS, nil)
	m.CreateSnapshot("pool/backup", "snap3", "desc", SnapshotTypeBtrfs, nil)
	m.CreatePolicy("test", "pool/data", SnapshotTypeZFS, "0 0 * * *", DefaultRetention())

	stats := m.GetStats()
	if stats.TotalSnapshots != 3 {
		t.Errorf("expected 3 total snapshots, got %d", stats.TotalSnapshots)
	}
	if stats.ActiveSnapshots != 3 {
		t.Errorf("expected 3 active snapshots, got %d", stats.ActiveSnapshots)
	}
	if stats.PolicyCount != 1 {
		t.Errorf("expected 1 policy, got %d", stats.PolicyCount)
	}
	if stats.ByType["zfs"] != 2 {
		t.Errorf("expected 2 ZFS snapshots, got %d", stats.ByType["zfs"])
	}
}

// ========== Handler 测试 ==========

func TestHandler_CreateSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"volume":"pool/data","name":"test-snap","description":"test","type":"zfs"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListSnapshots(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/"+snap.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_DeleteSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/snapshots/"+snap.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_MountSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)
	mountPoint := t.TempDir() + "/mount"

	body := `{"mount_point":"` + mountPoint + `","read_only":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/"+snap.ID+"/mount", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RollbackSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	body := `{"create_snapshot":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/"+snap.ID+"/rollback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CloneSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	snap, _ := m.CreateSnapshot("pool/data", "test", "desc", SnapshotTypeZFS, nil)

	body := `{"clone_name":"test-clone"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/"+snap.ID+"/clone", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_PolicyCRUD(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 创建
	body := `{"name":"daily","volume":"pool/data","type":"zfs","schedule":"0 0 * * *","retention":{"max_count":10,"max_age_days":30,"min_keep":2}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshot-policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	policyData, _ := json.Marshal(resp.Data)
	var policy SnapshotPolicy
	json.Unmarshal(policyData, &policy)

	// 列出
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshot-policies", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 获取
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshot-policies/"+policy.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 更新
	updateBody := `{"enabled":false}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/snapshot-policies/"+policy.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 删除
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/snapshot-policies/"+policy.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetStats(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
