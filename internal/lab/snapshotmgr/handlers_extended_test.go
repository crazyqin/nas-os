package snapshotmgr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ==================== 保留策略 Handler 测试 ====================

func TestHandler_CreatePolicy(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{
		"name": "daily-retention",
		"description": "Keep daily snapshots",
		"enabled": true,
		"target_scope": "global",
		"hourly": 24,
		"daily": 7,
		"weekly": 4,
		"monthly": 12
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var policy RetentionPolicy
	if err := json.Unmarshal(w.Body.Bytes(), &policy); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if policy.Name != "daily-retention" {
		t.Errorf("expected name 'daily-retention', got %q", policy.Name)
	}
	if policy.Hourly != 24 {
		t.Errorf("expected hourly 24, got %d", policy.Hourly)
	}
}

func TestHandler_ListPolicies(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Create a policy first
	body := `{"name":"test","enabled":true,"target_scope":"global","daily":7}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// List policies
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/policies", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Policies []RetentionPolicy `json:"policies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(resp.Policies))
	}
}

// ==================== ZFS Bookmark Handler 测试 ====================

func TestHandler_CreateZFSBookmark(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{
		"pool": "tank",
		"dataset": "tank/data",
		"snapshot_name": "snap-20250615",
		"bookmark_name": "bm-daily"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/zfs/bookmark", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var bm ZFSBookmark
	if err := json.Unmarshal(w.Body.Bytes(), &bm); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if bm.Name != "bm-daily" {
		t.Errorf("expected name 'bm-daily', got %q", bm.Name)
	}
}

func TestHandler_ListZFSBookmarks(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Create bookmark first
	body := `{"pool":"tank","dataset":"tank/data","snapshot_name":"snap-1","bookmark_name":"bm-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/zfs/bookmark", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// List bookmarks
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/zfs/bookmarks?pool=tank", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_DeleteZFSBookmark(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Create bookmark first
	body := `{"pool":"tank","dataset":"tank/data","snapshot_name":"snap-1","bookmark_name":"bm-del"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/zfs/bookmark", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Delete bookmark
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/snapshots/zfs/bookmarks/bm-del?pool=tank&dataset=tank/data", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== ZFS Hold Handler 测试 ====================

func TestHandler_AddZFSHold(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{
		"snapshot_id": "tank/data@snap-1",
		"tag": "replication",
		"reason": "Prevent deletion during replication",
		"holder_ref": "job-123"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/zfs/hold", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListZFSHolds(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Add a hold first
	body := `{"snapshot_id":"tank/data@snap-1","tag":"test-tag","reason":"test reason"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/zfs/hold", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// List holds
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/zfs/holds?snapshot_id=tank/data@snap-1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListZFSHolds_MissingSnapshotID(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/zfs/holds", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ==================== Diff Handler 测试 ====================

func TestHandler_GetSnapshotDiff(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/diff?pool=tank&dataset=tank/data&snapshot_a=snap-A&snapshot_b=snap-B", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result ZFSDiffResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result.SnapshotA != "tank/data@snap-A" {
		t.Errorf("expected snapshot_a 'tank/data@snap-A', got %q", result.SnapshotA)
	}
}

func TestHandler_GetSnapshotDiff_MissingParams(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/diff?pool=tank&snapshot_a=snap-A", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing snapshot_b, got %d", w.Code)
	}
}

// ==================== 配额 Handler 测试 ====================

func TestHandler_GetQuotaStatus(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/quota", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateQuota(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"max_percent": 30, "total_bytes": 1000000}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/snapshots/quota", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var status map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
}

func TestHandler_EnforceQuota(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/quota/enforce", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== 团队快照 Handler 测试 ====================

func TestHandler_CreateTeamPolicy(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{
		"team_id": "team-1",
		"folder_path": "/data/team1",
		"policy_name": "daily-team-backup",
		"enabled": true,
		"retain_daily": 7,
		"owner_visible": true,
		"member_visible": true
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/team/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListTeamPolicies(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// Create first
	body := `{"team_id":"team-1","folder_path":"/data","policy_name":"test","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/team/policies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/team/policies?team_id=team-1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CreateTeamSnapshot(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{
		"team_id": "team-1",
		"folder_path": "/data/team1",
		"created_by": "user-1",
		"source": "manual"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/team/snapshots", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListTeamSnapshots(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/team/snapshots?team_id=team-1&user_id=user-1&user_role=owner", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListTeamSnapshots_MissingParams(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/team/snapshots?team_id=team-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing user_id, got %d", w.Code)
	}
}
