package teamworkspace

import (
	"strings"
	"testing"
	"time"
)

// ========== 辅助函数 ==========

func createTestManager() *WorkspaceManager {
	return NewManager()
}

func defaultCreateOpts() CreateWorkspaceOptions {
	return CreateWorkspaceOptions{
		Name:              "Engineering Team",
		Description:       "Engineering shared workspace",
		OwnerID:           "user-001",
		DefaultPermission: "editor",
		QuotaGB:           100,
		Tags:              []string{"engineering", "dev"},
	}
}

// ========== NewManager ==========

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.workspaces == nil {
		t.Fatal("workspaces map not initialized")
	}
	if m.members == nil {
		t.Fatal("members map not initialized")
	}
	if m.invites == nil {
		t.Fatal("invites map not initialized")
	}
}

// ========== CreateWorkspace ==========

func TestCreateWorkspace_Success(t *testing.T) {
	m := createTestManager()
	opts := defaultCreateOpts()

	ws, err := m.CreateWorkspace(opts)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	if ws.ID == "" {
		t.Error("workspace ID should not be empty")
	}
	if ws.Name != "Engineering Team" {
		t.Errorf("expected name 'Engineering Team', got %s", ws.Name)
	}
	if ws.OwnerID != "user-001" {
		t.Errorf("expected owner 'user-001', got %s", ws.OwnerID)
	}
	if ws.MemberCount != 1 {
		t.Errorf("expected member count 1, got %d", ws.MemberCount)
	}
	if ws.QuotaGB != 100 {
		t.Errorf("expected quota 100, got %f", ws.QuotaGB)
	}
	if ws.Health != HealthGood {
		t.Errorf("expected health '%s', got '%s'", HealthGood, ws.Health)
	}
	if len(ws.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(ws.Tags))
	}
	if ws.DefaultPermission != "editor" {
		t.Errorf("expected default permission 'editor', got '%s'", ws.DefaultPermission)
	}
	// ID should have ws_ prefix
	if !strings.HasPrefix(ws.ID, "ws_") {
		t.Errorf("expected ID to start with 'ws_', got %s", ws.ID)
	}
}

func TestCreateWorkspace_EmptyName(t *testing.T) {
	m := createTestManager()
	opts := defaultCreateOpts()
	opts.Name = ""

	_, err := m.CreateWorkspace(opts)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestCreateWorkspace_EmptyOwner(t *testing.T) {
	m := createTestManager()
	opts := defaultCreateOpts()
	opts.OwnerID = ""

	_, err := m.CreateWorkspace(opts)
	if err == nil {
		t.Error("expected error for empty owner")
	}
}

func TestCreateWorkspace_NegativeQuota(t *testing.T) {
	m := createTestManager()
	opts := defaultCreateOpts()
	opts.QuotaGB = -10

	ws, err := m.CreateWorkspace(opts)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if ws.QuotaGB != 0 {
		t.Errorf("expected quota 0 for negative input, got %f", ws.QuotaGB)
	}
}

func TestCreateWorkspace_EmptyPermission(t *testing.T) {
	m := createTestManager()
	opts := defaultCreateOpts()
	opts.DefaultPermission = ""

	ws, err := m.CreateWorkspace(opts)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if ws.DefaultPermission != PermissionEditor {
		t.Errorf("expected default 'editor', got '%s'", ws.DefaultPermission)
	}
}

func TestCreateWorkspace_InvalidPermission(t *testing.T) {
	m := createTestManager()
	opts := defaultCreateOpts()
	opts.DefaultPermission = "invalid_perm"

	ws, err := m.CreateWorkspace(opts)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if ws.DefaultPermission != PermissionEditor {
		t.Errorf("expected 'editor' for invalid permission, got '%s'", ws.DefaultPermission)
	}
}

func TestCreateWorkspace_TagDedup(t *testing.T) {
	m := createTestManager()
	opts := defaultCreateOpts()
	opts.Tags = []string{"dev", "dev", "engineering", "", "  "}

	ws, err := m.CreateWorkspace(opts)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if len(ws.Tags) != 2 {
		t.Errorf("expected 2 unique non-empty tags, got %d: %v", len(ws.Tags), ws.Tags)
	}
}

// ========== ManageMembers ==========

func TestManageMembers_Add(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	changes := []MemberChange{
		{UserID: "user-002", Action: ActionAdd, Permission: PermissionEditor},
		{UserID: "user-003", Action: ActionAdd, Permission: PermissionViewer},
	}

	result, err := m.ManageMembers(ws.ID, changes)
	if err != nil {
		t.Fatalf("ManageMembers failed: %v", err)
	}

	if len(result.Added) != 2 {
		t.Errorf("expected 2 added, got %d", len(result.Added))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}

	// Verify member count updated
	ws2, _ := m.ListWorkspaces(WorkspaceFilter{})
	if ws2[0].MemberCount != 3 {
		t.Errorf("expected member count 3, got %d", ws2[0].MemberCount)
	}
}

func TestManageMembers_AddDuplicate(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	changes := []MemberChange{
		{UserID: "user-001", Action: ActionAdd, Permission: PermissionEditor},
	}

	result, err := m.ManageMembers(ws.ID, changes)
	if err != nil {
		t.Fatalf("ManageMembers failed: %v", err)
	}

	if len(result.Added) != 0 {
		t.Errorf("expected 0 added (duplicate), got %d", len(result.Added))
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error for duplicate, got %d", len(result.Errors))
	}
}

func TestManageMembers_Remove(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	// Add then remove
	_, _ = m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-002", Action: ActionAdd, Permission: PermissionEditor},
	})

	result, err := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-002", Action: ActionRemove},
	})
	if err != nil {
		t.Fatalf("ManageMembers failed: %v", err)
	}

	if len(result.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(result.Removed))
	}

	wsList, _ := m.ListWorkspaces(WorkspaceFilter{})
	if wsList[0].MemberCount != 1 {
		t.Errorf("expected member count 1 after removal, got %d", wsList[0].MemberCount)
	}
}

func TestManageMembers_RemoveOwnerFails(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	result, err := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-001", Action: ActionRemove},
	})
	if err != nil {
		t.Fatalf("ManageMembers failed: %v", err)
	}

	if len(result.Removed) != 0 {
		t.Error("should not be able to remove owner")
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error for removing owner, got %d", len(result.Errors))
	}
}

func TestManageMembers_Update(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	_, _ = m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-002", Action: ActionAdd, Permission: PermissionViewer},
	})

	result, err := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-002", Action: ActionUpdate, Permission: PermissionManager},
	})
	if err != nil {
		t.Fatalf("ManageMembers failed: %v", err)
	}

	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated, got %d", len(result.Updated))
	}
}

func TestManageMembers_UpdateOwnerFails(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	result, err := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-001", Action: ActionUpdate, Permission: PermissionAdmin},
	})
	if err != nil {
		t.Fatalf("ManageMembers failed: %v", err)
	}

	if len(result.Updated) != 0 {
		t.Error("should not be able to change owner permission")
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error for changing owner, got %d", len(result.Errors))
	}
}

func TestManageMembers_InvalidAction(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	result, err := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-002", Action: "invalid_action", Permission: PermissionEditor},
	})
	if err != nil {
		t.Fatalf("ManageMembers failed: %v", err)
	}

	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error for invalid action, got %d", len(result.Errors))
	}
}

func TestManageMembers_WorkspaceNotFound(t *testing.T) {
	m := createTestManager()

	_, err := m.ManageMembers("nonexistent", []MemberChange{
		{UserID: "user-002", Action: ActionAdd, Permission: PermissionEditor},
	})
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestManageMembers_AddWithOwnerPermissionDowngrades(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	result, _ := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-002", Action: ActionAdd, Permission: PermissionOwner},
	})

	if len(result.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(result.Added))
	}
	// owner permission should be downgraded to admin
	m.mu.RLock()
	perm := m.members[ws.ID]["user-002"].Permission
	m.mu.RUnlock()
	if perm != PermissionAdmin {
		t.Errorf("expected 'admin' (downgraded from owner), got '%s'", perm)
	}
}

func TestManageMembers_BatchMixed(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	// Pre-add some members
	_, _ = m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-002", Action: ActionAdd, Permission: PermissionViewer},
		{UserID: "user-003", Action: ActionAdd, Permission: PermissionViewer},
	})

	// Batch: add user-004, remove user-002, update user-003, add duplicate user-001
	result, err := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-004", Action: ActionAdd, Permission: PermissionEditor},
		{UserID: "user-002", Action: ActionRemove},
		{UserID: "user-003", Action: ActionUpdate, Permission: PermissionEditor},
		{UserID: "user-001", Action: ActionAdd, Permission: PermissionEditor}, // duplicate
		{UserID: "", Action: ActionAdd, Permission: PermissionEditor},  // empty
	})
	if err != nil {
		t.Fatalf("ManageMembers failed: %v", err)
	}

	if len(result.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(result.Added))
	}
	if len(result.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(result.Removed))
	}
	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated, got %d", len(result.Updated))
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors (duplicate + empty), got %d", len(result.Errors))
	}
}

// ========== SetQuota ==========

func TestSetQuota_Success(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	result, err := m.SetQuota(ws.ID, 200)
	if err != nil {
		t.Fatalf("SetQuota failed: %v", err)
	}

	if result.QuotaGB != 200 {
		t.Errorf("expected quota 200, got %f", result.QuotaGB)
	}
	if result.PreviousQuotaGB != 100 {
		t.Errorf("expected previous 100, got %f", result.PreviousQuotaGB)
	}
	if !result.Effective {
		t.Error("expected effective=true")
	}
	if result.Warning != "" {
		t.Errorf("expected no warning, got %s", result.Warning)
	}
}

func TestSetQuota_Unlimited(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	result, err := m.SetQuota(ws.ID, 0)
	if err != nil {
		t.Fatalf("SetQuota failed: %v", err)
	}

	if result.Warning == "" {
		t.Error("expected warning for unlimited quota")
	}
	if !strings.Contains(result.Warning, "unlimited") {
		t.Errorf("expected 'unlimited' in warning, got %s", result.Warning)
	}
}

func TestSetQuota_ExceedsUsage(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	// Set used amount high
	_ = m.SetUsedGB(ws.ID, 80)

	// Now set quota below usage
	result, err := m.SetQuota(ws.ID, 50)
	if err != nil {
		t.Fatalf("SetQuota failed: %v", err)
	}

	if result.Warning == "" {
		t.Error("expected warning when quota below usage")
	}
	if !strings.Contains(result.Warning, "exceeds") {
		t.Errorf("expected 'exceeds' in warning, got %s", result.Warning)
	}
}

func TestSetQuota_NegativeQuota(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	_, err := m.SetQuota(ws.ID, -10)
	if err == nil {
		t.Error("expected error for negative quota")
	}
}

func TestSetQuota_WorkspaceNotFound(t *testing.T) {
	m := createTestManager()

	_, err := m.SetQuota("nonexistent", 100)
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

// ========== InviteExternal ==========

func TestInviteExternal_Success(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	invite := ExternalInvite{
		Email:      "external@example.com",
		Permission: PermissionViewer,
		ExpiresIn:  48,
		WorkspaceID: ws.ID,
	}

	result, err := m.InviteExternal(ws.ID, invite)
	if err != nil {
		t.Fatalf("InviteExternal failed: %v", err)
	}

	if result.InviteID == "" {
		t.Error("invite ID should not be empty")
	}
	if result.Email != "external@example.com" {
		t.Errorf("expected email 'external@example.com', got %s", result.Email)
	}
	if result.Status != InviteStatusPending {
		t.Errorf("expected status 'pending', got '%s'", result.Status)
	}
	if result.ExpiresAt <= 0 {
		t.Error("expires_at should be positive")
	}
	if result.InviteLink == "" {
		t.Error("invite link should not be empty")
	}
	if !strings.HasPrefix(result.InviteID, "inv_") {
		t.Errorf("expected invite ID to start with 'inv_', got %s", result.InviteID)
	}

	// Default expiration (72h)
	now := time.Now().Unix()
	expectedExpiry := now + 48*3600
	margin := int64(5) // 5 second margin
	if result.ExpiresAt < expectedExpiry-margin || result.ExpiresAt > expectedExpiry+margin {
		t.Errorf("expires_at not within expected range")
	}
}

func TestInviteExternal_DefaultExpiry(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	invite := ExternalInvite{
		Email:       "test@example.com",
		Permission:  PermissionViewer,
		ExpiresIn:   0,
		WorkspaceID: ws.ID,
	}

	result, err := m.InviteExternal(ws.ID, invite)
	if err != nil {
		t.Fatalf("InviteExternal failed: %v", err)
	}

	// Should default to 72 hours
	now := time.Now().Unix()
	expectedExpiry := now + 72*3600
	margin := int64(5)
	if result.ExpiresAt < expectedExpiry-margin || result.ExpiresAt > expectedExpiry+margin {
		t.Errorf("expected default 72h expiry, got expires_at that doesn't match")
	}
}

func TestInviteExternal_OwnerPermissionDowngraded(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	invite := ExternalInvite{
		Email:       "test@example.com",
		Permission:  PermissionOwner,
		ExpiresIn:   24,
		WorkspaceID: ws.ID,
	}

	result, err := m.InviteExternal(ws.ID, invite)
	if err != nil {
		t.Fatalf("InviteExternal failed: %v", err)
	}

	// Check internal invite has viewer permission
	inv, ok := m.GetInvite(result.InviteID)
	if !ok {
		t.Fatal("invite not found")
	}
	if inv.Permission != PermissionViewer {
		t.Errorf("expected 'viewer' (downgraded from owner), got '%s'", inv.Permission)
	}
}

func TestInviteExternal_EmptyEmail(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	invite := ExternalInvite{
		Email:       "",
		Permission:  PermissionViewer,
		WorkspaceID: ws.ID,
	}

	_, err := m.InviteExternal(ws.ID, invite)
	if err == nil {
		t.Error("expected error for empty email")
	}
}

func TestInviteExternal_WorkspaceNotFound(t *testing.T) {
	m := createTestManager()

	invite := ExternalInvite{
		Email:       "test@example.com",
		Permission:  PermissionViewer,
		WorkspaceID: "nonexistent",
	}

	_, err := m.InviteExternal("nonexistent", invite)
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

// ========== AssessHealth ==========

func TestAssessHealth_HealthyWorkspace(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	// Add some members with recent activity
	_, _ = m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "user-002", Action: ActionAdd, Permission: PermissionEditor},
		{UserID: "user-003", Action: ActionAdd, Permission: PermissionViewer},
	})

	// Usage is moderate
	_ = m.SetUsedGB(ws.ID, 30) // 30% of 100GB

	health, err := m.AssessHealth(ws.ID)
	if err != nil {
		t.Fatalf("AssessHealth failed: %v", err)
	}

	if health.WorkspaceID != ws.ID {
		t.Errorf("expected workspace ID %s, got %s", ws.ID, health.WorkspaceID)
	}
	if health.Score < 75 {
		t.Errorf("expected score >= 75 for healthy workspace, got %f", health.Score)
	}
	if health.ActiveMembers < 1 {
		t.Errorf("expected at least 1 active member, got %d", health.ActiveMembers)
	}
}

func TestAssessHealth_StorageCritical(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	// Set usage to 96% of quota
	_ = m.SetUsedGB(ws.ID, 96) // 96% of 100GB

	health, err := m.AssessHealth(ws.ID)
	if err != nil {
		t.Fatalf("AssessHealth failed: %v", err)
	}

	if health.Score >= 75 {
		t.Errorf("expected score < 75 for critical storage, got %f", health.Score)
	}
	foundStorageIssue := false
	for _, issue := range health.Issues {
		if strings.Contains(issue, "storage usage") {
			foundStorageIssue = true
			break
		}
	}
	if !foundStorageIssue {
		t.Error("expected storage issue in health report")
	}
}

func TestAssessHealth_NoActiveMembers(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	// Set owner activity to 8 days ago
	oldTime := time.Now().Unix() - 8*24*3600
	_ = m.TouchMemberActivity(ws.ID, "user-001", oldTime)

	health, err := m.AssessHealth(ws.ID)
	if err != nil {
		t.Fatalf("AssessHealth failed: %v", err)
	}

	if health.ActiveMembers != 0 {
		t.Errorf("expected 0 active members, got %d", health.ActiveMembers)
	}
	// Should have an issue about no active members
	foundNoActive := false
	for _, issue := range health.Issues {
		if strings.Contains(issue, "no active members") {
			foundNoActive = true
			break
		}
	}
	if !foundNoActive {
		t.Error("expected 'no active members' issue")
	}
}

func TestAssessHealth_WorkspaceNotFound(t *testing.T) {
	m := createTestManager()

	_, err := m.AssessHealth("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestAssessHealth_StorageTrendCritical(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	_ = m.SetUsedGB(ws.ID, 95) // 95% of 100GB

	health, _ := m.AssessHealth(ws.ID)

	if health.StorageTrend != "critical" {
		t.Errorf("expected storage trend 'critical', got '%s'", health.StorageTrend)
	}
}

func TestAssessHealth_StorageTrendStable(t *testing.T) {
	m := createTestManager()
	ws, _ := m.CreateWorkspace(defaultCreateOpts())

	_ = m.SetUsedGB(ws.ID, 20) // 20% of 100GB

	health, _ := m.AssessHealth(ws.ID)

	if health.StorageTrend != "stable" {
		t.Errorf("expected storage trend 'stable', got '%s'", health.StorageTrend)
	}
}

// ========== ListWorkspaces ==========

func TestListWorkspaces_All(t *testing.T) {
	m := createTestManager()

	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{
		Name: "Team A", OwnerID: "user-001", QuotaGB: 50, Tags: []string{"a"},
	})
	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{
		Name: "Team B", OwnerID: "user-002", QuotaGB: 100, Tags: []string{"b"},
	})

	result, err := m.ListWorkspaces(WorkspaceFilter{})
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(result))
	}
}

func TestListWorkspaces_FilterByOwner(t *testing.T) {
	m := createTestManager()

	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{
		Name: "Team A", OwnerID: "user-001", QuotaGB: 50,
	})
	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{
		Name: "Team B", OwnerID: "user-002", QuotaGB: 100,
	})

	result, err := m.ListWorkspaces(WorkspaceFilter{OwnerID: "user-001"})
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 workspace for user-001, got %d", len(result))
	}
	if result[0].Name != "Team A" {
		t.Errorf("expected 'Team A', got '%s'", result[0].Name)
	}
}

func TestListWorkspaces_FilterByName(t *testing.T) {
	m := createTestManager()

	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{Name: "Engineering Team", OwnerID: "u1", QuotaGB: 50})
	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{Name: "Marketing Team", OwnerID: "u2", QuotaGB: 100})

	result, err := m.ListWorkspaces(WorkspaceFilter{NameLike: "engineer"})
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 workspace matching 'engineer', got %d", len(result))
	}
	if result[0].Name != "Engineering Team" {
		t.Errorf("expected 'Engineering Team', got '%s'", result[0].Name)
	}
}

func TestListWorkspaces_FilterByMinMembers(t *testing.T) {
	m := createTestManager()

	ws1, _ := m.CreateWorkspace(CreateWorkspaceOptions{Name: "Small", OwnerID: "u1", QuotaGB: 50})
	ws2, _ := m.CreateWorkspace(CreateWorkspaceOptions{Name: "Big", OwnerID: "u2", QuotaGB: 100})

	// Add members to ws2
	_, _ = m.ManageMembers(ws2.ID, []MemberChange{
		{UserID: "u3", Action: ActionAdd, Permission: PermissionEditor},
		{UserID: "u4", Action: ActionAdd, Permission: PermissionViewer},
	})

	result, err := m.ListWorkspaces(WorkspaceFilter{MinMembers: 3})
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 workspace with >= 3 members, got %d", len(result))
	}
	if result[0].Name != "Big" {
		t.Errorf("expected 'Big', got '%s'", result[0].Name)
	}
	_ = ws1
}

func TestListWorkspaces_CaseInsensitiveName(t *testing.T) {
	m := createTestManager()

	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{Name: "ENGINEERING", OwnerID: "u1", QuotaGB: 50})

	// Case-insensitive search
	result, err := m.ListWorkspaces(WorkspaceFilter{NameLike: "engineering"})
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 result (case-insensitive), got %d", len(result))
	}
}

func TestListWorkspaces_EmptyFilter(t *testing.T) {
	m := createTestManager()
	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{Name: "Team A", OwnerID: "u1", QuotaGB: 50})

	result, err := m.ListWorkspaces(WorkspaceFilter{})
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(result))
	}
}

func TestListWorkspaces_NoMatch(t *testing.T) {
	m := createTestManager()
	_, _ = m.CreateWorkspace(CreateWorkspaceOptions{Name: "Team A", OwnerID: "u1", QuotaGB: 50})

	result, err := m.ListWorkspaces(WorkspaceFilter{NameLike: "nonexistent"})
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

// ========== Integration ==========

func TestIntegration_FullWorkflow(t *testing.T) {
	m := createTestManager()

	// 1. Create workspace
	ws, err := m.CreateWorkspace(defaultCreateOpts())
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// 2. Add members
	memberResult, err := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "dev-001", Action: ActionAdd, Permission: PermissionEditor},
		{UserID: "dev-002", Action: ActionAdd, Permission: PermissionViewer},
		{UserID: "dev-003", Action: ActionAdd, Permission: PermissionManager},
	})
	if err != nil {
		t.Fatalf("add members failed: %v", err)
	}
	if len(memberResult.Added) != 3 {
		t.Errorf("expected 3 added, got %d", len(memberResult.Added))
	}

	// 3. Set quota
	quotaResult, err := m.SetQuota(ws.ID, 500)
	if err != nil {
		t.Fatalf("set quota failed: %v", err)
	}
	if quotaResult.QuotaGB != 500 {
		t.Errorf("expected quota 500, got %f", quotaResult.QuotaGB)
	}

	// 4. Set usage to moderate level
	_ = m.SetUsedGB(ws.ID, 200) // 40% of 500GB

	// 5. Invite external user
	inviteResult, err := m.InviteExternal(ws.ID, ExternalInvite{
		Email:      "contractor@external.com",
		Permission: PermissionViewer,
		ExpiresIn:  168, // 1 week
		WorkspaceID: ws.ID,
	})
	if err != nil {
		t.Fatalf("invite failed: %v", err)
	}
	if inviteResult.Status != InviteStatusPending {
		t.Errorf("expected pending status, got %s", inviteResult.Status)
	}

	// 6. Assess health
	health, err := m.AssessHealth(ws.ID)
	if err != nil {
		t.Fatalf("health assessment failed: %v", err)
	}
	if health.Score < 70 {
		t.Errorf("expected decent health score, got %f", health.Score)
	}
	if health.ActiveMembers != 4 { // owner + 3 added
		t.Errorf("expected 4 active members, got %d", health.ActiveMembers)
	}

	// 7. List workspaces
	wsList, err := m.ListWorkspaces(WorkspaceFilter{OwnerID: "user-001"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(wsList) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(wsList))
	}
	if wsList[0].MemberCount != 4 {
		t.Errorf("expected 4 members in list, got %d", wsList[0].MemberCount)
	}

	// 8. Update a member and remove one
	updateResult, err := m.ManageMembers(ws.ID, []MemberChange{
		{UserID: "dev-001", Action: ActionUpdate, Permission: PermissionAdmin},
		{UserID: "dev-002", Action: ActionRemove},
	})
	if err != nil {
		t.Fatalf("update/remove failed: %v", err)
	}
	if len(updateResult.Updated) != 1 || len(updateResult.Removed) != 1 {
		t.Errorf("expected 1 updated + 1 removed, got %d updated + %d removed",
			len(updateResult.Updated), len(updateResult.Removed))
	}

	// 9. Verify final state
	wsList, _ = m.ListWorkspaces(WorkspaceFilter{})
	if wsList[0].MemberCount != 3 {
		t.Errorf("expected 3 final members, got %d", wsList[0].MemberCount)
	}

	// 10. Final health check
	health2, _ := m.AssessHealth(ws.ID)
	if health2.ActiveMembers != 3 {
		t.Errorf("expected 3 active in health, got %d", health2.ActiveMembers)
	}
}