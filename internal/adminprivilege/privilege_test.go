package adminprivilege

import (
	"testing"
)

func TestUserCreation(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewAdminPrivilegeManager(tmpDir)

	user, err := mgr.CreateUser("admin", "管理员", PrivilegeSuperAdmin)
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	if user.Username != "admin" {
		t.Fatal("username mismatch")
	}
	if user.Level != PrivilegeSuperAdmin {
		t.Fatal("level mismatch")
	}
}

func TestPermissionCheck(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewAdminPrivilegeManager(tmpDir)

	user, _ := mgr.CreateUser("viewer", "查看者", PrivilegeViewer)

	if !mgr.CheckPermission(user.ID, PermViewDashboard) {
		t.Fatal("viewer should have VIEW_DASHBOARD")
	}
	if mgr.CheckPermission(user.ID, PermManageStorage) {
		t.Fatal("viewer should not have MANAGE_STORAGE")
	}
}

func TestGrantRevoke(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewAdminPrivilegeManager(tmpDir)

	user, _ := mgr.CreateUser("operator", "操作员", PrivilegeOperator)

	if mgr.CheckPermission(user.ID, PermManageUsers) {
		t.Fatal("operator should not have MANAGE_USERS")
	}

	mgr.GrantPermission(user.ID, PermManageUsers)
	if !mgr.CheckPermission(user.ID, PermManageUsers) {
		t.Fatal("should have MANAGE_USERS after grant")
	}

	mgr.RevokePermission(user.ID, PermManageUsers)
	if mgr.CheckPermission(user.ID, PermManageUsers) {
		t.Fatal("should not have MANAGE_USERS after revoke")
	}
}

func TestUserDisable(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewAdminPrivilegeManager(tmpDir)

	user, _ := mgr.CreateUser("test", "测试", PrivilegeAdmin)

	mgr.DisableUser(user.ID)
	if mgr.CheckPermission(user.ID, PermViewDashboard) {
		t.Fatal("disabled user should have no permissions")
	}

	mgr.EnableUser(user.ID)
	if !mgr.CheckPermission(user.ID, PermViewDashboard) {
		t.Fatal("enabled user should have permissions")
	}
}

func TestFailedLoginLockout(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewAdminPrivilegeManager(tmpDir)

	user, _ := mgr.CreateUser("test", "测试", PrivilegeAdmin)

	for i := 0; i < 5; i++ {
		mgr.RecordLogin(user.ID, false, "10.0.0.1")
	}

	u, _ := mgr.GetUser(user.ID)
	if u.Enabled {
		t.Fatal("user should be disabled after 5 failed logins")
	}
}

func TestRoles(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewAdminPrivilegeManager(tmpDir)

	roles := mgr.GetRoles()
	if len(roles) != 4 {
		t.Fatalf("expected 4 roles, got %d", len(roles))
	}
}
