package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== RBACManager 基础测试 ==========

func TestNewRBACManager(t *testing.T) {
	mgr := NewRBACManager()
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.roles)
	assert.NotNil(t, mgr.userRoles)
	assert.NotNil(t, mgr.groupRoles)
	assert.NotNil(t, mgr.resourceACLs)
}

func TestRBACManager_BuiltInRoles(t *testing.T) {
	mgr := NewRBACManager()

	// 验证内置角色存在
	roles := mgr.GetRoles()
	roleNames := make(map[Role]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
	}

	assert.True(t, roleNames[RoleAdmin])
	assert.True(t, roleNames[RoleUser])
	assert.True(t, roleNames[RoleGuest])
	assert.True(t, roleNames[RoleSystem])
}

func TestRBACManager_AdminHasAllPermissions(t *testing.T) {
	mgr := NewRBACManager()

	// Admin 应该有所有权限
	resources := []Resource{ResourceVolume, ResourceShare, ResourceUser, ResourceSystem, ResourceContainer, ResourceVM}
	actions := []Action{ActionRead, ActionWrite, ActionDelete, ActionAdmin}

	for _, resource := range resources {
		for _, action := range actions {
			hasPermission := mgr.CheckPermission("admin-user", []string{}, resource, action)
			assert.True(t, hasPermission, "Admin should have %s:%s", resource, action)
		}
	}
}

func TestRBACManager_GuestReadOnly(t *testing.T) {
	mgr := NewRBACManager()

	// Guest 应该只有读取权限
	assert.True(t, mgr.CheckPermission("guest-user", []string{}, ResourceShare, ActionRead))
	assert.True(t, mgr.CheckPermission("guest-user", []string{}, ResourceFile, ActionRead))
	assert.False(t, mgr.CheckPermission("guest-user", []string{}, ResourceShare, ActionWrite))
	assert.False(t, mgr.CheckPermission("guest-user", []string{}, ResourceUser, ActionRead))
}

// ========== 角色分配测试 ==========

func TestRBACManager_AssignRoleToUser(t *testing.T) {
	mgr := NewRBACManager()

	err := mgr.AssignRoleToUser("user1", RoleUser)
	assert.NoError(t, err)

	roles := mgr.GetUserRoles("user1")
	assert.Contains(t, roles, RoleUser)
}

func TestRBACManager_AssignNonExistentRole(t *testing.T) {
	mgr := NewRBACManager()

	err := mgr.AssignRoleToUser("user1", Role("nonexistent"))
	assert.Error(t, err)
	assert.Equal(t, ErrRoleNotFound, err)
}

func TestRBACManager_RemoveUserRole(t *testing.T) {
	mgr := NewRBACManager()

	// 添加角色
	_ = mgr.AssignRoleToUser("user1", RoleUser)
	_ = mgr.AssignRoleToUser("user1", RoleAdmin)

	// 移除角色
	err := mgr.RemoveUserRole("user1", RoleUser)
	assert.NoError(t, err)

	roles := mgr.GetUserRoles("user1")
	assert.NotContains(t, roles, RoleUser)
	assert.Contains(t, roles, RoleAdmin)
}

func TestRBACManager_AssignRoleToGroup(t *testing.T) {
	mgr := NewRBACManager()

	err := mgr.AssignRoleToGroup("developers", RoleUser)
	assert.NoError(t, err)

	roles := mgr.GetUserGroupRoles("developers")
	assert.Contains(t, roles, RoleUser)
}

// ========== 权限检查测试 ==========

func TestRBACManager_CheckPermission_WithUserRole(t *testing.T) {
	mgr := NewRBACManager()

	// 分配角色
	_ = mgr.AssignRoleToUser("user1", RoleUser)

	// 检查权限
	assert.True(t, mgr.CheckPermission("user1", []string{}, ResourceVolume, ActionRead))
	assert.True(t, mgr.CheckPermission("user1", []string{}, ResourceFile, ActionWrite))
	assert.False(t, mgr.CheckPermission("user1", []string{}, ResourceUser, ActionAdmin))
	assert.False(t, mgr.CheckPermission("user1", []string{}, ResourceSystem, ActionAdmin))
}

func TestRBACManager_CheckPermission_WithGroupRole(t *testing.T) {
	mgr := NewRBACManager()

	// 给组分配角色
	_ = mgr.AssignRoleToGroup("developers", RoleUser)

	// 用户在组中，应该有组的权限
	assert.True(t, mgr.CheckPermission("user1", []string{"developers"}, ResourceVolume, ActionRead))
	assert.False(t, mgr.CheckPermission("user1", []string{"developers"}, ResourceSystem, ActionAdmin))
}

func TestRBACManager_CheckPermission_DefaultRole(t *testing.T) {
	mgr := NewRBACManager()

	// 没有分配角色的用户使用默认角色（Guest）
	assert.True(t, mgr.CheckPermission("unknown-user", []string{}, ResourceShare, ActionRead))
	assert.False(t, mgr.CheckPermission("unknown-user", []string{}, ResourceShare, ActionWrite))
}

// ========== 自定义角色测试 ==========

func TestRBACManager_AddCustomRole(t *testing.T) {
	mgr := NewRBACManager()

	err := mgr.AddRole(
		Role("editor"),
		"编辑者角色",
		[]Permission{
			{Resource: string(ResourceFile), Action: string(ActionRead)},
			{Resource: string(ResourceFile), Action: string(ActionWrite)},
		},
		nil,
	)
	assert.NoError(t, err)

	// 验证角色存在
	roles := mgr.GetRoles()
	roleNames := make(map[Role]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
	}
	assert.True(t, roleNames[Role("editor")])
}

func TestRBACManager_RoleInheritance(t *testing.T) {
	mgr := NewRBACManager()

	// 创建基础角色
	_ = mgr.AddRole(
		Role("base"),
		"基础角色",
		[]Permission{
			{Resource: string(ResourceFile), Action: string(ActionRead)},
		},
		nil,
	)

	// 创建继承角色
	_ = mgr.AddRole(
		Role("advanced"),
		"高级角色",
		[]Permission{
			{Resource: string(ResourceFile), Action: string(ActionWrite)},
		},
		[]Role{Role("base")},
	)

	// 分配继承角色
	_ = mgr.AssignRoleToUser("user1", Role("advanced"))

	// 应该有继承的权限
	assert.True(t, mgr.CheckPermission("user1", []string{}, ResourceFile, ActionRead))
	assert.True(t, mgr.CheckPermission("user1", []string{}, ResourceFile, ActionWrite))
}

// ========== 权限继承解析测试 ==========

func TestRBACManager_GetRolePermissions(t *testing.T) {
	mgr := NewRBACManager()

	// 获取 Admin 角色的所有权限
	perms := mgr.GetRolePermissions(RoleAdmin)
	assert.NotEmpty(t, perms)

	// 检查包含关键权限
	permMap := make(map[string]bool)
	for _, p := range perms {
		permMap[p.Resource+":"+p.Action] = true
	}

	assert.True(t, permMap["volume:read"])
	assert.True(t, permMap["volume:write"])
	assert.True(t, permMap["system:admin"])
}

// ========== 资源 ACL 测试 ==========

func TestRBACManager_SetResourceACL(t *testing.T) {
	mgr := NewRBACManager()

	mgr.SetResourceACL(
		"volume-1",
		ResourceVolume,
		"owner-1",
		[]GroupACL{
			{GroupID: "developers", Permissions: []Permission{
				{Resource: string(ResourceVolume), Action: string(ActionRead)},
			}},
		},
		[]UserACL{
			{UserID: "user1", Permissions: []Permission{
				{Resource: string(ResourceVolume), Action: string(ActionWrite)},
			}},
		},
	)

	acl := mgr.GetResourceACL("volume-1")
	assert.NotNil(t, acl)
	assert.Equal(t, "owner-1", acl.OwnerID)
	assert.Len(t, acl.GroupACLs, 1)
	assert.Len(t, acl.UserACLs, 1)
}

func TestRBACManager_CheckResourceOwnership(t *testing.T) {
	mgr := NewRBACManager()

	// 设置资源所有者
	mgr.SetResourceOwner("volume-1", "owner-1")

	// 检查所有权
	assert.True(t, mgr.CheckResourceOwnership("owner-1", "volume-1"))
	assert.False(t, mgr.CheckResourceOwnership("owner-2", "volume-1"))
}

func TestRBACManager_GetResourcesByOwner(t *testing.T) {
	mgr := NewRBACManager()

	mgr.SetResourceOwner("volume-1", "owner-1")
	mgr.SetResourceOwner("volume-2", "owner-1")
	mgr.SetResourceOwner("volume-3", "owner-2")

	resources := mgr.GetResourcesByOwner("owner-1")
	assert.Len(t, resources, 2)
}

// ========== 会话缓存测试 ==========

func TestRBACManager_SessionCache(t *testing.T) {
	mgr := NewRBACManager()

	// 分配角色
	_ = mgr.AssignRoleToUser("user1", RoleUser)

	// 缓存会话
	mgr.CacheSession("token-1", "user1", []string{})

	// 获取缓存
	session := mgr.GetCachedSession("token-1")
	assert.NotNil(t, session)
	assert.Equal(t, "user1", session.UserID)
	assert.NotEmpty(t, session.Permissions)
}

func TestRBACManager_InvalidateSession(t *testing.T) {
	mgr := NewRBACManager()

	_ = mgr.AssignRoleToUser("user1", RoleUser)
	mgr.CacheSession("token-1", "user1", []string{})

	// 使会话失效
	mgr.InvalidateSession("token-1")

	// 验证已失效
	session := mgr.GetCachedSession("token-1")
	assert.Nil(t, session)
}

func TestRBACManager_InvalidateUserSessions(t *testing.T) {
	mgr := NewRBACManager()

	_ = mgr.AssignRoleToUser("user1", RoleUser)
	mgr.CacheSession("token-1", "user1", []string{})
	mgr.CacheSession("token-2", "user1", []string{})

	// 使用户所有会话失效
	mgr.InvalidateUserSessions("user1")

	assert.Nil(t, mgr.GetCachedSession("token-1"))
	assert.Nil(t, mgr.GetCachedSession("token-2"))
}

// ========== 权限模板测试 ==========

func TestGetPermissionTemplates(t *testing.T) {
	templates := GetPermissionTemplates()
	assert.NotEmpty(t, templates)

	templateNames := make(map[string]bool)
	for _, t := range templates {
		templateNames[t.Name] = true
	}

	assert.True(t, templateNames["readonly"])
	assert.True(t, templateNames["editor"])
	assert.True(t, templateNames["operator"])
}

func TestRBACManager_CreateRoleFromTemplate(t *testing.T) {
	mgr := NewRBACManager()

	err := mgr.CreateRoleFromTemplate("viewer", "readonly", "只读查看者")
	assert.NoError(t, err)

	// 验证角色创建成功
	roles := mgr.GetRoles()
	for _, r := range roles {
		if r.Name == Role("viewer") {
			assert.Equal(t, "只读查看者", r.Description)
			return
		}
	}
	t.Error("Role not created from template")
}

func TestRBACManager_CreateRoleFromNonExistentTemplate(t *testing.T) {
	mgr := NewRBACManager()

	err := mgr.CreateRoleFromTemplate("test", "nonexistent", "测试")
	assert.Error(t, err)
}

// ========== 统计测试 ==========

func TestRBACManager_GetRBACStats(t *testing.T) {
	mgr := NewRBACManager()

	// 添加一些数据
	_ = mgr.AssignRoleToUser("user1", RoleUser)
	_ = mgr.AssignRoleToUser("user2", RoleAdmin)
	_ = mgr.AssignRoleToGroup("developers", RoleUser)
	mgr.CacheSession("token-1", "user1", []string{})

	stats := mgr.GetRBACStats()

	assert.Equal(t, 4, stats["total_roles"]) // 4 个内置角色
	assert.Equal(t, 2, stats["total_user_roles"])
	assert.Equal(t, 1, stats["total_group_roles"])
	assert.Equal(t, 1, stats["cached_sessions"])
}

// ========== 并发测试 ==========

func TestRBACManager_ConcurrentAccess(t *testing.T) {
	mgr := NewRBACManager()

	done := make(chan bool, 10)

	// 并发分配角色
	for i := 0; i < 5; i++ {
		go func(idx int) {
			_ = mgr.AssignRoleToUser("user"+string(rune('0'+idx)), RoleUser)
			done <- true
		}(i)
	}

	// 并发权限检查
	for i := 0; i < 5; i++ {
		go func(idx int) {
			_ = mgr.CheckPermission("user"+string(rune('0'+idx)), []string{}, ResourceVolume, ActionRead)
			done <- true
		}(i)
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 清理过期会话测试 ==========

func TestRBACManager_CleanupExpiredSessions(t *testing.T) {
	mgr := NewRBACManager()
	mgr.cacheExpiry = 100 * time.Millisecond // 设置短过期时间

	_ = mgr.AssignRoleToUser("user1", RoleUser)
	mgr.CacheSession("token-1", "user1", []string{})

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 清理
	mgr.CleanupExpiredSessions()

	// 验证已清理
	session := mgr.GetCachedSession("token-1")
	assert.Nil(t, session)
}
