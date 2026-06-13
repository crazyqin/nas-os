package rbacenhanced

import (
	"testing"
)

func TestManagerStartStop(t *testing.T) {
	m := NewManager(nil)
	if m.IsRunning() {
		t.Fatal("新创建的管理器不应在运行")
	}
	if err := m.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("管理器应该在运行")
	}
	if err := m.Start(); err == nil {
		t.Fatal("重复启动应返回错误")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("管理器不应在运行")
	}
}

func TestSystemRoles(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	roles := m.ListRoles()
	if len(roles) != 3 {
		t.Fatalf("期望3个系统角色，实际 %d", len(roles))
	}
	admin, err := m.GetRole("admin")
	if err != nil {
		t.Fatalf("获取admin角色失败: %v", err)
	}
	if !admin.IsSystem {
		t.Fatal("admin应该是系统角色")
	}
}

func TestCreateRole(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	role := &Role{
		ID:          "editor",
		Name:        "编辑者",
		Description: "编辑者角色",
		Permissions: []Permission{PermRead, PermWrite},
		Resources:   []ResourceType{ResourceFile, ResourceDirectory},
	}
	if err := m.CreateRole(role); err != nil {
		t.Fatalf("创建角色失败: %v", err)
	}
	if len(m.ListRoles()) != 4 {
		t.Fatalf("期望4个角色，实际 %d", len(m.ListRoles()))
	}
}

func TestCreateRoleDuplicate(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	role := &Role{ID: "admin", Name: "duplicate"}
	if err := m.CreateRole(role); err == nil {
		t.Fatal("创建重复角色应返回错误")
	}
}

func TestDeleteRole(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateRole(&Role{ID: "temp", Name: "temp"})
	if err := m.DeleteRole("temp"); err != nil {
		t.Fatalf("删除角色失败: %v", err)
	}
}

func TestDeleteSystemRole(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	if err := m.DeleteRole("admin"); err == nil {
		t.Fatal("删除系统角色应返回错误")
	}
}

func TestDeleteRoleWithUsers(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateRole(&Role{ID: "custom", Name: "custom"})
	m.CreateUser(&User{ID: "user-1", Username: "test", Enabled: true})
	m.AssignRole("user-1", "custom")
	if err := m.DeleteRole("custom"); err == nil {
		t.Fatal("删除被使用的角色应返回错误")
	}
}

func TestCreateUser(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	user := &User{ID: "user-1", Username: "testuser", Email: "test@example.com", Enabled: true}
	if err := m.CreateUser(user); err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	if !user.Enabled {
		t.Fatal("用户应该启用")
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "user-1", Username: "test", Enabled: true})
	if err := m.CreateUser(&User{ID: "user-1", Username: "test2", Enabled: true}); err == nil {
		t.Fatal("创建重复用户应返回错误")
	}
}

func TestAssignRole(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "user-1", Username: "test", Enabled: true})
	if err := m.AssignRole("user-1", "admin"); err != nil {
		t.Fatalf("分配角色失败: %v", err)
	}
	user, _ := m.GetUser("user-1")
	if len(user.Roles) != 1 || user.Roles[0] != "admin" {
		t.Fatal("角色分配不正确")
	}
}

func TestAssignRoleDuplicate(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "user-1", Username: "test", Enabled: true})
	m.AssignRole("user-1", "admin")
	m.AssignRole("user-1", "admin") // 重复分配应不报错
	user, _ := m.GetUser("user-1")
	if len(user.Roles) != 1 {
		t.Fatal("重复分配不应增加角色")
	}
}

func TestCheckPermission(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "user-1", Username: "test", Enabled: true})
	m.AssignRole("user-1", "user")

	if !m.CheckPermission("user-1", PermRead, ResourceFile, "/data/test.txt") {
		t.Fatal("用户应该有读权限")
	}
	if !m.CheckPermission("user-1", PermWrite, ResourceFile, "/data/test.txt") {
		t.Fatal("用户应该有写权限")
	}
	if m.CheckPermission("user-1", PermDelete, ResourceFile, "/data/test.txt") {
		t.Fatal("用户不应有删除权限")
	}
	if m.CheckPermission("user-1", PermAdmin, ResourceSystem, "/") {
		t.Fatal("普通用户不应有管理员权限")
	}
}

func TestCheckPermissionAdmin(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "admin-1", Username: "admin", Enabled: true})
	m.AssignRole("admin-1", "admin")

	if !m.CheckPermission("admin-1", PermDelete, ResourceFile, "/data/any.txt") {
		t.Fatal("管理员应该有所有权限")
	}
	if !m.CheckPermission("admin-1", PermConfigure, ResourceSystem, "/") {
		t.Fatal("管理员应该有配置权限")
	}
}

func TestCheckPermissionViewer(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "viewer-1", Username: "viewer", Enabled: true})
	m.AssignRole("viewer-1", "viewer")

	if !m.CheckPermission("viewer-1", PermRead, ResourceFile, "/data/test.txt") {
		t.Fatal("只读用户应该有读权限")
	}
	if m.CheckPermission("viewer-1", PermWrite, ResourceFile, "/data/test.txt") {
		t.Fatal("只读用户不应有写权限")
	}
}

func TestCheckPermissionDisabled(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "user-1", Username: "test", Enabled: false})
	m.AssignRole("user-1", "admin")
	if m.CheckPermission("user-1", PermRead, ResourceFile, "/") {
		t.Fatal("禁用用户不应有任何权限")
	}
}

func TestACLRule(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "user-1", Username: "test", Enabled: true})
	m.AssignRole("user-1", "viewer")

	// 只读用户默认不能写
	if m.CheckPermission("user-1", PermWrite, ResourceFile, "/data/protected.txt") {
		t.Fatal("只读用户不应有写权限")
	}

	// 添加ACL允许特定文件写入
	m.AddACL(&ACLRule{
		ID:         "acl-1",
		Resource:   "/data/protected.txt",
		ResType:    ResourceFile,
		Principal:  "user-1",
		Permission: PermWrite,
		Allowed:    true,
	})

	if !m.CheckPermission("user-1", PermWrite, ResourceFile, "/data/protected.txt") {
		t.Fatal("ACL应允许写入")
	}
}

func TestAuditLog(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.LogAudit(AuditEntry{
		UserID:   "user-1",
		Username: "test",
		Action:   AuditLogin,
		Success:  true,
	})
	m.LogAudit(AuditEntry{
		UserID:   "user-1",
		Username: "test",
		Action:   AuditAccess,
		Resource: "/data/test.txt",
		Success:  true,
	})

	logs := m.GetAuditLog("user-1", 10)
	if len(logs) != 2 {
		t.Fatalf("期望2条审计日志，实际 %d", len(logs))
	}
}

func TestAuditLogDisabled(t *testing.T) {
	config := DefaultRBACConfig()
	config.EnableAudit = false
	m := NewManager(config)
	m.Start()
	defer m.Stop()

	m.LogAudit(AuditEntry{UserID: "user-1", Action: AuditLogin})
	if len(m.GetAuditLog("", 10)) != 0 {
		t.Fatal("审计禁用时不应记录日志")
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.CreateUser(&User{ID: "user-1", Username: "test", Enabled: true})
	m.AssignRole("user-1", "admin")
	m.AddACL(&ACLRule{ID: "acl-1", Resource: "/test", Principal: "user-1", Permission: PermRead, Allowed: true})
	m.LogAudit(AuditEntry{UserID: "user-1", Action: AuditLogin})

	stats := m.GetStats()
	if stats["total_roles"] != 3 {
		t.Fatalf("期望3个角色，实际 %v", stats["total_roles"])
	}
	if stats["total_users"] != 1 {
		t.Fatalf("期望1个用户，实际 %v", stats["total_users"])
	}
	if stats["total_acls"] != 1 {
		t.Fatalf("期望1个ACL，实际 %v", stats["total_acls"])
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultRBACConfig()
	if !config.EnableAudit {
		t.Fatal("默认应启用审计")
	}
	if !config.EnableFIPS {
		t.Fatal("默认应启用FIPS")
	}
	if config.MaxRoles != 100 {
		t.Fatal("最大角色数错误")
	}
}
