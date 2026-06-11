package acl

import (
	"testing"
	"time"
)

func TestRBACManager_BuiltinRoles(t *testing.T) {
	aclMgr := NewManager()
	rm := NewRBACManager(aclMgr)

	roles := rm.ListRoles()
	if len(roles) < 5 {
		t.Errorf("expected at least 5 builtin roles, got %d", len(roles))
	}

	roleIDs := make(map[string]bool)
	for _, r := range roles {
		roleIDs[r.ID] = true
		if !r.Builtin {
			t.Errorf("builtin role %s should have Builtin=true", r.ID)
		}
	}
	for _, expected := range []string{"owner", "editor", "viewer", "collaborator", "admin"} {
		if !roleIDs[expected] {
			t.Errorf("missing builtin role: %s", expected)
		}
	}
}

func TestRBACManager_CustomRole(t *testing.T) {
	aclMgr := NewManager()
	rm := NewRBACManager(aclMgr)

	err := rm.CreateRole(Role{
		ID:          "photographer",
		Name:        "摄影师",
		Description: "照片管理权限",
		Permissions: []string{PermReadFile, PermWriteFile, PermViewMetadata, PermModifyMetadata},
	})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	roles := rm.ListRoles()
	found := false
	for _, r := range roles {
		if r.ID == "photographer" {
			found = true
			if r.Builtin {
				t.Error("custom role should not be builtin")
			}
		}
	}
	if !found {
		t.Error("custom role not found")
	}
}

func TestRBACManager_DeleteBuiltinRole(t *testing.T) {
	aclMgr := NewManager()
	rm := NewRBACManager(aclMgr)

	err := rm.DeleteRole("owner")
	if err == nil {
		t.Error("should not be able to delete builtin role")
	}
}

func TestRBACManager_AssignAndCheck(t *testing.T) {
	aclMgr := NewManager()
	rm := NewRBACManager(aclMgr)

	// Assign viewer role to user
	err := rm.AssignRole(UserRoleAssignment{
		UserID: "user1",
		RoleID: "viewer",
	})
	if err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	// Viewer should have read access
	if !rm.CheckPermission("user1", "/photos", PermReadFile) {
		t.Error("viewer should have read_file permission")
	}

	// Viewer should NOT have write access
	if rm.CheckPermission("user1", "/photos", PermWriteFile) {
		t.Error("viewer should NOT have write_file permission")
	}
}

func TestRBACManager_PathScopedRole(t *testing.T) {
	aclMgr := NewManager()
	rm := NewRBACManager(aclMgr)

	// Assign editor role scoped to /photos
	err := rm.AssignRole(UserRoleAssignment{
		UserID:    "user1",
		RoleID:    "editor",
		Path:      "/photos",
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	// Should have write access to /photos
	if !rm.CheckPermission("user1", "/photos/vacation", PermWriteFile) {
		t.Error("editor should have write access to scoped path")
	}

	// Should NOT have write access to /documents
	if rm.CheckPermission("user1", "/documents", PermWriteFile) {
		t.Error("editor should NOT have write access outside scoped path")
	}
}

func TestRBACManager_Expiration(t *testing.T) {
	aclMgr := NewManager()
	rm := NewRBACManager(aclMgr)

	pastTime := time.Now().Add(-1 * time.Hour)
	err := rm.AssignRole(UserRoleAssignment{
		UserID:    "user1",
		RoleID:    "editor",
		ExpiresAt: &pastTime,
	})
	if err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	// Expired assignment should not grant access
	if rm.CheckPermission("user1", "/photos", PermWriteFile) {
		t.Error("expired role should not grant access")
	}
}

func TestRBACManager_AuditLog(t *testing.T) {
	aclMgr := NewManager()
	rm := NewRBACManager(aclMgr)

	rm.CheckPermission("user1", "/test", PermReadFile)
	rm.CheckPermission("user1", "/test", PermWriteFile)

	log := rm.GetAuditLog(10)
	if len(log) < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", len(log))
	}
}

func TestRBACManager_FallbackToACL(t *testing.T) {
	aclMgr := NewManager()
	aclMgr.AddRule(ACLRule{
		ID:          "rule1",
		Path:        "/shared",
		Subject:     "user1",
		SubjectType: "user",
		Permissions: []string{PermRead, PermWrite},
		Recursive:   true,
		Priority:    10,
	})

	rm := NewRBACManager(aclMgr)

	// No RBAC assignment, but ACL rule should grant access
	if !rm.CheckPermission("user1", "/shared/docs", PermRead) {
		t.Error("ACL fallback should grant read access")
	}
}

func TestRBACManager_RevokeRole(t *testing.T) {
	aclMgr := NewManager()
	rm := NewRBACManager(aclMgr)

	rm.AssignRole(UserRoleAssignment{
		UserID: "user1",
		RoleID: "viewer",
		Path:   "/photos",
	})

	if !rm.CheckPermission("user1", "/photos", PermReadFile) {
		t.Fatal("should have access before revoke")
	}

	err := rm.RevokeRole("user1", "viewer", "/photos")
	if err != nil {
		t.Fatalf("RevokeRole failed: %v", err)
	}

	if rm.CheckPermission("user1", "/photos", PermReadFile) {
		t.Error("access should be revoked")
	}
}
