package rbac

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ========== 角色测试 ==========

func TestDefaultRoles(t *testing.T) {
	roles := []Role{RoleAdmin, RoleOperator, RoleReadOnly, RoleGuest}

	for _, role := range roles {
		info, ok := DefaultRoles[role]
		if !ok {
			t.Errorf("角色 %s 未定义", role)
			continue
		}

		if info.Name == "" {
			t.Errorf("角色 %s 名称为空", role)
		}

		if len(info.Permissions) == 0 && role != RoleAdmin {
			t.Errorf("角色 %s 没有定义权限", role)
		}
	}
}

func TestRolePriority(t *testing.T) {
	// 管理员优先级最高
	adminInfo := DefaultRoles[RoleAdmin]
	guestInfo := DefaultRoles[RoleGuest]

	if adminInfo.Priority <= guestInfo.Priority {
		t.Error("管理员角色优先级应该高于访客")
	}
}

// ========== 权限测试 ==========

func TestPermissionString(t *testing.T) {
	tests := []struct {
		resource string
		action   string
		expected string
	}{
		{"system", "read", "system:read"},
		{"user", "write", "user:write"},
		{"share", "admin", "share:admin"},
	}

	for _, tt := range tests {
		result := PermissionString(tt.resource, tt.action)
		if result != tt.expected {
			t.Errorf("PermissionString(%s, %s) = %s, want %s", tt.resource, tt.action, result, tt.expected)
		}
	}
}

func TestParsePermission(t *testing.T) {
	tests := []struct {
		perm         string
		wantResource string
		wantAction   string
	}{
		{"system:read", "system", "read"},
		{"user:write", "user", "write"},
		{"share", "share", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		resource, action := ParsePermission(tt.perm)
		if resource != tt.wantResource || action != tt.wantAction {
			t.Errorf("ParsePermission(%s) = (%s, %s), want (%s, %s)", tt.perm, resource, action, tt.wantResource, tt.wantAction)
		}
	}
}

// ========== 管理器测试 ==========

func TestNewManager(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "rbac-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := Config{
		CacheEnabled: true,
		CacheTTL:     time.Minute * 5,
		ConfigPath:   filepath.Join(tmpDir, "rbac.json"),
		StrictMode:   true,
	}

	m, err := NewManager(config)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	if m == nil {
		t.Fatal("管理器为空")
	}
}

func TestSetUserRole(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	err := m.SetUserRole("user1", "testuser", RoleOperator)
	if err != nil {
		t.Fatalf("设置用户角色失败: %v", err)
	}

	up, err := m.GetUserPermissions("user1")
	if err != nil {
		t.Fatalf("获取用户权限失败: %v", err)
	}

	if up.Role != RoleOperator {
		t.Errorf("用户角色 = %s, want %s", up.Role, RoleOperator)
	}
}

func TestGrantPermission(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	// 先设置角色
	_ = m.SetUserRole("user1", "testuser", RoleReadOnly)

	// 授予额外权限
	err := m.GrantPermission("user1", "testuser", "storage:write")
	if err != nil {
		t.Fatalf("授予权限失败: %v", err)
	}

	up, err := m.GetUserPermissions("user1")
	if err != nil {
		t.Fatalf("获取用户权限失败: %v", err)
	}

	found := false
	for _, p := range up.DirectPerms {
		if p == "storage:write" {
			found = true
			break
		}
	}

	if !found {
		t.Error("未找到授予的权限")
	}
}

func TestRevokePermission(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("user1", "testuser", RoleReadOnly)
	_ = m.GrantPermission("user1", "testuser", "storage:write")

	// 撤销权限
	err := m.RevokePermission("user1", "storage:write")
	if err != nil {
		t.Fatalf("撤销权限失败: %v", err)
	}

	up, _ := m.GetUserPermissions("user1")
	for _, p := range up.DirectPerms {
		if p == "storage:write" {
			t.Error("权限未被撤销")
		}
	}
}

// ========== 权限检查测试 ==========

func TestCheckPermission_Admin(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("admin1", "admin", RoleAdmin)

	// 管理员应该有所有权限
	tests := []struct {
		resource string
		action   string
	}{
		{"system", "read"},
		{"system", "write"},
		{"system", "admin"},
		{"user", "read"},
		{"user", "admin"},
		{"storage", "write"},
	}

	for _, tt := range tests {
		result := m.CheckPermission("admin1", tt.resource, tt.action)
		if !result.Allowed {
			t.Errorf("管理员应该有权限 %s:%s", tt.resource, tt.action)
		}
	}
}

func TestCheckPermission_Operator(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("op1", "operator", RoleOperator)

	// 运维员有系统读写权限
	result := m.CheckPermission("op1", "system", "read")
	if !result.Allowed {
		t.Error("运维员应该有 system:read 权限")
	}

	result = m.CheckPermission("op1", "system", "write")
	if !result.Allowed {
		t.Error("运维员应该有 system:write 权限")
	}

	// 运维员没有用户管理权限
	result = m.CheckPermission("op1", "user", "admin")
	if result.Allowed {
		t.Error("运维员不应该有 user:admin 权限")
	}
}

func TestCheckPermission_ReadOnly(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("ro1", "readonly", RoleReadOnly)

	// 只读用户有读权限
	result := m.CheckPermission("ro1", "system", "read")
	if !result.Allowed {
		t.Error("只读用户应该有 system:read 权限")
	}

	// 只读用户没有写权限
	result = m.CheckPermission("ro1", "system", "write")
	if result.Allowed {
		t.Error("只读用户不应该有 system:write 权限")
	}
}

func TestCheckPermission_Guest(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("guest1", "guest", RoleGuest)

	// 访客只有基本读取权限
	result := m.CheckPermission("guest1", "system", "read")
	if !result.Allowed {
		t.Error("访客应该有 system:read 权限")
	}

	// 访客没有其他权限
	result = m.CheckPermission("guest1", "storage", "read")
	if result.Allowed {
		t.Error("访客不应该有 storage:read 权限")
	}
}

func TestCheckPermission_Wildcard(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("user1", "testuser", RoleReadOnly)
	_ = m.GrantPermission("user1", "testuser", "share:*")

	// 应该匹配所有 share 相关权限
	tests := []struct {
		resource string
		action   string
		allowed  bool
	}{
		{"share", "read", true},
		{"share", "write", true},
		{"share", "admin", true},
		{"storage", "read", true}, // 来自角色
		{"storage", "write", false},
	}

	for _, tt := range tests {
		result := m.CheckPermission("user1", tt.resource, tt.action)
		if result.Allowed != tt.allowed {
			t.Errorf("CheckPermission(%s, %s) = %v, want %v", tt.resource, tt.action, result.Allowed, tt.allowed)
		}
	}
}

// ========== 用户组权限继承测试 ==========

func TestGroupPermission(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	// 创建用户组权限
	err := m.CreateGroupPermission("group1", "developers", []string{"storage:write", "share:write"})
	if err != nil {
		t.Fatalf("创建用户组权限失败: %v", err)
	}

	// 将用户添加到组
	err = m.AddUserToGroup("user1", "dev1", "group1", "developers", false)
	if err != nil {
		t.Fatalf("添加用户到组失败: %v", err)
	}

	// 用户应该继承组权限
	result := m.CheckPermission("user1", "storage", "write")
	if !result.Allowed {
		t.Error("用户应该继承组的 storage:write 权限")
	}

	result = m.CheckPermission("user1", "share", "write")
	if !result.Allowed {
		t.Error("用户应该继承组的 share:write 权限")
	}
}

func TestGroupPermissionUpdate(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.CreateGroupPermission("group1", "developers", []string{"storage:write"})
	_ = m.AddUserToGroup("user1", "dev1", "group1", "developers", false)

	// 初始有写权限
	result := m.CheckPermission("user1", "storage", "write")
	if !result.Allowed {
		t.Error("用户应该有 storage:write 权限")
	}

	// 更新组权限
	err := m.UpdateGroupPermission("group1", []string{"storage:read"})
	if err != nil {
		t.Fatalf("更新组权限失败: %v", err)
	}

	// 清除缓存以重新计算
	m.InvalidateAllCaches()

	// 现在应该只有读权限
	result = m.CheckPermission("user1", "storage", "read")
	if !result.Allowed {
		t.Error("用户应该有 storage:read 权限")
	}

	result = m.CheckPermission("user1", "storage", "write")
	if result.Allowed {
		t.Error("用户不应该有 storage:write 权限")
	}
}

// ========== 策略测试 ==========

func TestPolicy(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	// 创建拒绝策略
	policy, err := m.CreatePolicy(
		"deny-storage-admin",
		"拒绝存储管理操作",
		EffectDeny,
		[]string{"user:user1"},
		[]string{"storage"},
		[]string{"admin"},
		100,
	)
	if err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}

	if policy == nil {
		t.Fatal("策略为空")
	}

	// 策略应该生效
	result := m.CheckPermission("user1", "storage", "admin")
	if result.Allowed {
		t.Error("策略应该拒绝 storage:admin 权限")
	}
	if result.DeniedBy != "deny-storage-admin" {
		t.Errorf("DeniedBy = %s, want deny-storage-admin", result.DeniedBy)
	}
}

func TestPolicyDelete(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	policy, _ := m.CreatePolicy(
		"test-policy",
		"测试策略",
		EffectDeny,
		[]string{"user:user1"},
		[]string{"storage"},
		[]string{"write"},
		100,
	)

	// 策略生效
	result := m.CheckPermission("user1", "storage", "write")
	if result.Allowed {
		t.Error("策略应该拒绝权限")
	}

	// 删除策略
	err := m.DeletePolicy(policy.ID)
	if err != nil {
		t.Fatalf("删除策略失败: %v", err)
	}

	// 清除缓存
	m.InvalidateAllCaches()

	// 策略应该不再生效
	result = m.CheckPermission("user1", "storage", "write")
	if !result.Allowed {
		t.Error("删除策略后应该允许权限")
	}
}

// ========== 缓存测试 ==========

func TestPermissionCache(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	m.config.CacheEnabled = true
	m.config.CacheTTL = time.Minute

	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	// 第一次检查（会缓存）
	result1 := m.CheckPermission("user1", "system", "read")
	if !result1.Allowed {
		t.Error("应该有权限")
	}

	// 第二次检查（使用缓存）
	result2 := m.CheckPermission("user1", "system", "read")
	if !result2.Allowed {
		t.Error("缓存检查应该有权限")
	}
}

func TestCacheInvalidation(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	m.config.CacheEnabled = true

	_ = m.SetUserRole("user1", "testuser", RoleReadOnly)

	// 检查权限（缓存）
	_ = m.CheckPermission("user1", "storage", "read")

	// 授予新权限（应该清除缓存）
	_ = m.GrantPermission("user1", "testuser", "storage:write")

	// 检查新权限
	result := m.CheckPermission("user1", "storage", "write")
	if !result.Allowed {
		t.Error("新授予的权限应该生效")
	}
}

// ========== 共享 ACL 测试 ==========

func TestShareACL(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	shareMgr, err := NewShareACLManager(ShareACLConfig{
		ConfigPath: filepath.Join(filepath.Dir(m.config.ConfigPath), "share_acl.json"),
	}, m)
	if err != nil {
		t.Fatalf("创建共享 ACL 管理器失败: %v", err)
	}

	// 创建共享 ACL
	acl, err := shareMgr.CreateShareACL("share1", ShareTypeSMB, "/data/share1", "测试共享", AccessRead)
	if err != nil {
		t.Fatalf("创建共享 ACL 失败: %v", err)
	}

	if acl == nil {
		t.Fatal("ACL 为空")
	}

	// 添加用户权限
	entry, err := shareMgr.AddACLEntry("share1", PrincipalUser, "user1", "testuser", AccessWrite, "admin")
	if err != nil {
		t.Fatalf("添加 ACL 条目失败: %v", err)
	}

	if entry == nil {
		t.Fatal("条目为空")
	}

	// 检查权限
	perm := shareMgr.CheckShareAccess("user1", "testuser", "share1")
	if perm.AccessLevel != AccessWrite {
		t.Errorf("访问级别 = %s, want %s", perm.AccessLevel, AccessWrite)
	}

	// 检查其他用户（使用默认权限）
	perm = shareMgr.CheckShareAccess("user2", "otheruser", "share1")
	if perm.AccessLevel != AccessRead {
		t.Errorf("默认访问级别 = %s, want %s", perm.AccessLevel, AccessRead)
	}
}

func TestShareACL_Group(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	// 设置用户组
	_ = m.CreateGroupPermission("group1", "developers", []string{})
	_ = m.AddUserToGroup("user1", "dev1", "group1", "developers", false)

	shareMgr, _ := NewShareACLManager(ShareACLConfig{}, m)
	_, _ = shareMgr.CreateShareACL("share1", ShareTypeSMB, "/data/share1", "测试", AccessNone)

	// 给组添加权限
	_, _ = shareMgr.AddACLEntry("share1", PrincipalGroup, "group1", "developers", AccessWrite, "admin")

	// 用户应该继承组权限
	perm := shareMgr.CheckShareAccess("user1", "dev1", "share1")
	if perm.AccessLevel != AccessWrite {
		t.Errorf("用户应该继承组的写权限，实际: %s", perm.AccessLevel)
	}

	// 其他用户应该没有权限
	perm = shareMgr.CheckShareAccess("user2", "other", "share1")
	if perm.AccessLevel != AccessNone {
		t.Errorf("其他用户应该没有权限，实际: %s", perm.AccessLevel)
	}
}

// ========== 辅助函数 ==========

func newTestManager(t *testing.T) (*Manager, func()) {
	tmpDir, err := os.MkdirTemp("", "rbac-test-*")
	if err != nil {
		t.Fatal(err)
	}

	config := Config{
		CacheEnabled: true,
		CacheTTL:     time.Minute * 5,
		ConfigPath:   filepath.Join(tmpDir, "rbac.json"),
		StrictMode:   true,
		AuditEnabled: false,
	}

	m, err := NewManager(config)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return m, cleanup
}

// ========== 基准测试 ==========

func BenchmarkCheckPermission(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "rbac-bench-*")
	defer os.RemoveAll(tmpDir)

	m, _ := NewManager(Config{
		CacheEnabled: true,
		CacheTTL:     time.Minute * 5,
		ConfigPath:   filepath.Join(tmpDir, "rbac.json"),
	})

	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.CheckPermission("user1", "system", "read")
	}
}

func BenchmarkCheckPermission_NoCache(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "rbac-bench-*")
	defer os.RemoveAll(tmpDir)

	m, _ := NewManager(Config{
		CacheEnabled: false,
		ConfigPath:   filepath.Join(tmpDir, "rbac.json"),
	})

	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.CheckPermission("user1", "system", "read")
	}
}

func BenchmarkCheckPermission_Admin(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "rbac-bench-*")
	defer os.RemoveAll(tmpDir)

	m, _ := NewManager(Config{
		CacheEnabled: true,
		CacheTTL:     time.Minute * 5,
		ConfigPath:   filepath.Join(tmpDir, "rbac.json"),
	})

	_ = m.SetUserRole("admin1", "admin", RoleAdmin)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.CheckPermission("admin1", "system", "admin")
	}
}
