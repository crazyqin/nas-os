package rootless

import (
	"testing"
)

func TestNewRootlessAdminManager(t *testing.T) {
	mgr := NewRootlessAdminManager(nil)
	if mgr == nil {
		t.Fatal("NewRootlessAdminManager returned nil")
	}
	if !mgr.config.Enabled {
		t.Error("expected enabled by default")
	}
	if mgr.config.AdminGroup != "nas-admins" {
		t.Errorf("expected group nas-admins, got %s", mgr.config.AdminGroup)
	}
}

func TestCheckPrivilegeNoAdmin(t *testing.T) {
	mgr := NewRootlessAdminManager(nil)
	if mgr.CheckPrivilege("nonexistent", "storage", "read") {
		t.Error("non-existent user should not have privileges")
	}
}

func TestGetAdminCountEmpty(t *testing.T) {
	mgr := NewRootlessAdminManager(nil)
	if mgr.GetAdminCount() != 0 {
		t.Errorf("expected 0 admins, got %d", mgr.GetAdminCount())
	}
}

func TestListAdminsEmpty(t *testing.T) {
	mgr := NewRootlessAdminManager(nil)
	admins := mgr.ListAdmins()
	if len(admins) != 0 {
		t.Errorf("expected 0 admins, got %d", len(admins))
	}
}

func TestIsCommandAllowed(t *testing.T) {
	mgr := NewRootlessAdminManager(nil)
	if !mgr.isCommandAllowed("/usr/bin/docker") {
		t.Error("docker should be in allowed list")
	}
	if mgr.isCommandAllowed("/usr/bin/rm") {
		t.Error("rm should NOT be in allowed list")
	}
}
