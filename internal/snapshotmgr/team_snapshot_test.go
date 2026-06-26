package snapshotmgr

import (
	"testing"

	"go.uber.org/zap"
)

func setupTestTeamManager(t *testing.T) (*Manager, *TeamSnapshotManager) {
	t.Helper()
	tmpDir := t.TempDir()
	config := &SnapshotConfig{
		MaxSnapshots:  50,
		RetentionDays: 90,
	}
	m := NewManager(zap.NewNop(), config, tmpDir)
	team := NewTeamSnapshotManager(zap.NewNop(), m)
	return m, team
}

func TestCreateTeamPolicy(t *testing.T) {
	_, team := setupTestTeamManager(t)

	policy := &TeamSnapshotPolicy{
		TeamID:      "team-1",
		FolderPath:  "/data/team1",
		PolicyName:  "daily-backup",
		Enabled:     true,
		AutoCreate:  true,
		CronExpr:    "0 2 * * *",
		RetainDaily: 7,
		Visibility: TeamSnapshotVisibility{
			OwnerVisible:  true,
			MemberVisible: true,
			GuestVisible:  false,
		},
	}

	created, err := team.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.TeamID != "team-1" {
		t.Errorf("expected team_id 'team-1', got %q", created.TeamID)
	}
	if !created.Visibility.AdminVisible {
		t.Error("expected admin to always be visible")
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestListTeamPolicies(t *testing.T) {
	_, team := setupTestTeamManager(t)

	team.CreatePolicy(&TeamSnapshotPolicy{TeamID: "team-1", FolderPath: "/a", PolicyName: "p1"})
	team.CreatePolicy(&TeamSnapshotPolicy{TeamID: "team-1", FolderPath: "/b", PolicyName: "p2"})
	team.CreatePolicy(&TeamSnapshotPolicy{TeamID: "team-2", FolderPath: "/c", PolicyName: "p3"})

	policies := team.ListPoliciesByTeam("team-1")
	if len(policies) != 2 {
		t.Errorf("expected 2 policies for team-1, got %d", len(policies))
	}

	policies = team.ListPoliciesByTeam("team-2")
	if len(policies) != 1 {
		t.Errorf("expected 1 policy for team-2, got %d", len(policies))
	}
}

func TestUpdateTeamPolicy(t *testing.T) {
	_, team := setupTestTeamManager(t)

	created, _ := team.CreatePolicy(&TeamSnapshotPolicy{
		TeamID:      "team-1",
		FolderPath:  "/data",
		PolicyName:  "original",
		Enabled:     true,
		RetainDaily: 7,
	})

	updated, err := team.UpdatePolicy(created.ID, &TeamSnapshotPolicy{
		PolicyName:  "updated",
		Enabled:     false,
		RetainDaily: 14,
	})
	if err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}
	if updated.PolicyName != "updated" {
		t.Errorf("expected policy_name 'updated', got %q", updated.PolicyName)
	}
	if updated.Enabled {
		t.Error("expected enabled=false")
	}
	if updated.RetainDaily != 14 {
		t.Errorf("expected retain_daily 14, got %d", updated.RetainDaily)
	}
}

func TestDeleteTeamPolicy(t *testing.T) {
	_, team := setupTestTeamManager(t)

	created, _ := team.CreatePolicy(&TeamSnapshotPolicy{TeamID: "team-1", FolderPath: "/data", PolicyName: "test"})

	err := team.DeletePolicy(created.ID)
	if err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	_, err = team.GetPolicy(created.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestCreateTeamSnapshot(t *testing.T) {
	_, team := setupTestTeamManager(t)

	// First create a policy so visibility is checked
	team.CreatePolicy(&TeamSnapshotPolicy{
		TeamID:     "team-1",
		FolderPath: "/data/team1",
		PolicyName: "test-policy",
		Enabled:    true,
		Visibility: TeamSnapshotVisibility{
			OwnerVisible:  true,
			MemberVisible: true,
		},
	})

	snap, err := team.CreateTeamSnapshot("team-1", "/data/team1", "user-1", "manual")
	if err != nil {
		t.Fatalf("CreateTeamSnapshot failed: %v", err)
	}

	if snap.SnapshotID == "" {
		t.Error("expected non-empty SnapshotID")
	}
	if snap.TeamID != "team-1" {
		t.Errorf("expected team_id 'team-1', got %q", snap.TeamID)
	}
	if snap.CreatedBy != "user-1" {
		t.Errorf("expected created_by 'user-1', got %q", snap.CreatedBy)
	}
}

func TestTeamSnapshotVisibility(t *testing.T) {
	_, team := setupTestTeamManager(t)

	// Policy with member-visible only
	team.CreatePolicy(&TeamSnapshotPolicy{
		TeamID:     "team-1",
		FolderPath: "/data/team1",
		PolicyName: "member-only",
		Enabled:    true,
		Visibility: TeamSnapshotVisibility{
			OwnerVisible:  true,
			MemberVisible: true,
			GuestVisible:  false,
			AdminVisible:  true,
		},
	})

	team.CreateTeamSnapshot("team-1", "/data/team1", "user-1", "manual")

	// Owner should see
	snaps := team.ListTeamSnapshots("team-1", "user-1", "owner")
	if len(snaps) != 1 {
		t.Errorf("expected owner to see 1 snapshot, got %d", len(snaps))
	}

	// Member should see
	snaps = team.ListTeamSnapshots("team-1", "user-2", "member")
	if len(snaps) != 1 {
		t.Errorf("expected member to see 1 snapshot, got %d", len(snaps))
	}

	// Guest should NOT see
	snaps = team.ListTeamSnapshots("team-1", "user-3", "guest")
	if len(snaps) != 0 {
		t.Errorf("expected guest to see 0 snapshots, got %d", len(snaps))
	}

	// Admin should always see
	snaps = team.ListTeamSnapshots("team-1", "admin-1", "admin")
	if len(snaps) != 1 {
		t.Errorf("expected admin to see 1 snapshot, got %d", len(snaps))
	}
}

func TestTeamSnapshotLockUnlock(t *testing.T) {
	_, team := setupTestTeamManager(t)

	ts, _ := team.CreateTeamSnapshot("team-1", "/data", "user-1", "manual")

	// Lock
	if err := team.LockSnapshot(ts.SnapshotID); err != nil {
		t.Fatalf("LockSnapshot failed: %v", err)
	}

	// Locked snapshot should not be deletable by non-admin
	err := team.DeleteTeamSnapshot(ts.SnapshotID, "user-1", "owner")
	if err == nil {
		t.Error("expected error deleting locked snapshot")
	}

	// Unlock
	if err := team.UnlockSnapshot(ts.SnapshotID); err != nil {
		t.Fatalf("UnlockSnapshot failed: %v", err)
	}
}

func TestTeamSnapshotDefaultVisibility(t *testing.T) {
	_, team := setupTestTeamManager(t)

	// Create snapshot without policy - all team members should see
	team.CreateTeamSnapshot("team-1", "/data/no-policy", "user-1", "manual")

	snaps := team.ListTeamSnapshots("team-1", "user-guest", "guest")
	if len(snaps) != 1 {
		t.Errorf("expected guest to see 1 snapshot without policy, got %d", len(snaps))
	}
}
