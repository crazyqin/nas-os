package smb

import (
	"path/filepath"
	"testing"

	"nas-os/internal/users"
)

// ========== SMB 用户权限测试 ==========

// setupPermissionTest 创建带用户管理器的测试环境
func setupPermissionTest(t *testing.T) (*Manager, *users.Manager, string) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")

	userMgr, err := users.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("创建用户管理器失败: %v", err)
	}

	// 创建测试用户
	testUsers := []struct {
		username string
		password string
		role     users.Role
	}{
		{"admin", "admin123", users.RoleAdmin},
		{"user1", "pass123", users.RoleUser},
		{"user2", "pass123", users.RoleUser},
		{"readonly", "pass123", users.RoleUser},
	}

	for _, u := range testUsers {
		if _, err := userMgr.CreateUser(u.username, u.password, u.role); err != nil {
			t.Fatalf("创建用户 %s 失败: %v", u.username, err)
		}
	}

	mgr, err := NewManagerWithUserMgr(userMgr, configPath)
	if err != nil {
		t.Fatalf("创建 SMB 管理器失败: %v", err)
	}

	return mgr, userMgr, tmpDir
}

// ========== SetSharePermission 测试 ==========

func TestSetSharePermission_Basic(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享
	sharePath := filepath.Join(tmpDir, "perm-share")
	_ = mgr.CreateShare(&Share{
		Name: "perm-share",
		Path: sharePath,
	})

	// 设置用户读权限
	err := mgr.SetSharePermission("perm-share", "user1", false)
	if err != nil {
		t.Fatalf("设置读权限失败: %v", err)
	}

	share, _ := mgr.GetShare("perm-share")

	// 验证用户在允许列表中
	found := false
	for _, u := range share.ValidUsers {
		if u == "user1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("用户应该在允许列表中")
	}

	// 读权限不应该在写列表中
	for _, u := range share.WriteList {
		if u == "user1" {
			t.Error("读权限用户不应该在写列表中")
			break
		}
	}
}

func TestSetSharePermission_ReadWrite(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享
	sharePath := filepath.Join(tmpDir, "rw-share")
	_ = mgr.CreateShare(&Share{
		Name: "rw-share",
		Path: sharePath,
	})

	// 设置用户读写权限
	err := mgr.SetSharePermission("rw-share", "user1", true)
	if err != nil {
		t.Fatalf("设置读写权限失败: %v", err)
	}

	share, _ := mgr.GetShare("rw-share")

	// 验证用户在允许列表中
	foundValid := false
	for _, u := range share.ValidUsers {
		if u == "user1" {
			foundValid = true
			break
		}
	}
	if !foundValid {
		t.Error("用户应该在允许列表中")
	}

	// 验证用户在写列表中
	foundWrite := false
	for _, u := range share.WriteList {
		if u == "user1" {
			foundWrite = true
			break
		}
	}
	if !foundWrite {
		t.Error("读写权限用户应该在写列表中")
	}
}

func TestSetSharePermission_MultipleUsers(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享
	sharePath := filepath.Join(tmpDir, "multi-share")
	_ = mgr.CreateShare(&Share{
		Name: "multi-share",
		Path: sharePath,
	})

	// 设置多个用户权限
	users := []struct {
		name      string
		readWrite bool
	}{
		{"user1", true},
		{"user2", false},
		{"readonly", false},
	}

	for _, u := range users {
		err := mgr.SetSharePermission("multi-share", u.name, u.readWrite)
		if err != nil {
			t.Fatalf("设置用户 %s 权限失败: %v", u.name, err)
		}
	}

	share, _ := mgr.GetShare("multi-share")

	// 验证所有用户都在允许列表中
	if len(share.ValidUsers) != 3 {
		t.Errorf("允许列表应该有3个用户，实际: %d", len(share.ValidUsers))
	}

	// 只有 user1 应该在写列表中
	if len(share.WriteList) != 1 {
		t.Errorf("写列表应该有1个用户，实际: %d", len(share.WriteList))
	}
	if share.WriteList[0] != "user1" {
		t.Errorf("写列表应该是 user1，实际: %s", share.WriteList[0])
	}
}

func TestSetSharePermission_ShareNotExist(t *testing.T) {
	mgr, _, _ := setupPermissionTest(t)

	err := mgr.SetSharePermission("nonexistent", "user1", true)
	if err == nil {
		t.Error("共享不存在应该报错")
	}
}

func TestSetSharePermission_UserNotExist(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享
	sharePath := filepath.Join(tmpDir, "test-share")
	_ = mgr.CreateShare(&Share{
		Name: "test-share",
		Path: sharePath,
	})

	err := mgr.SetSharePermission("test-share", "nonexistent", true)
	if err == nil {
		t.Error("用户不存在应该报错")
	}
}

func TestSetSharePermission_Duplicate(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享
	sharePath := filepath.Join(tmpDir, "dup-share")
	_ = mgr.CreateShare(&Share{
		Name: "dup-share",
		Path: sharePath,
	})

	// 多次设置同一用户权限
	for i := 0; i < 3; i++ {
		err := mgr.SetSharePermission("dup-share", "user1", true)
		if err != nil {
			t.Fatalf("第%d次设置权限失败: %v", i+1, err)
		}
	}

	share, _ := mgr.GetShare("dup-share")

	// 用户应该只出现一次
	count := 0
	for _, u := range share.ValidUsers {
		if u == "user1" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("用户在允许列表中重复出现 %d 次", count)
	}
}

// ========== RemoveSharePermission 测试 ==========

func TestRemoveSharePermission_Basic(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享并添加权限
	sharePath := filepath.Join(tmpDir, "remove-share")
	_ = mgr.CreateShare(&Share{
		Name:       "remove-share",
		Path:       sharePath,
		ValidUsers: []string{"user1", "user2", "readonly"},
		WriteList:  []string{"user1"},
	})

	// 移除 user1 权限
	err := mgr.RemoveSharePermission("remove-share", "user1")
	if err != nil {
		t.Fatalf("移除权限失败: %v", err)
	}

	share, _ := mgr.GetShare("remove-share")

	// 验证 user1 已从允许列表移除
	for _, u := range share.ValidUsers {
		if u == "user1" {
			t.Error("user1 应该已从允许列表移除")
			break
		}
	}

	// 验证 user1 已从写列表移除
	for _, u := range share.WriteList {
		if u == "user1" {
			t.Error("user1 应该已从写列表移除")
			break
		}
	}

	// 验证其他用户还在
	if len(share.ValidUsers) != 2 {
		t.Errorf("允许列表应该剩余2个用户，实际: %d", len(share.ValidUsers))
	}
}

func TestRemoveSharePermission_NotInList(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享
	sharePath := filepath.Join(tmpDir, "test-share")
	_ = mgr.CreateShare(&Share{
		Name:       "test-share",
		Path:       sharePath,
		ValidUsers: []string{"user1"},
	})

	// 移除不在列表中的用户（应该成功，只是无操作）
	err := mgr.RemoveSharePermission("test-share", "user2")
	if err != nil {
		t.Fatalf("移除不存在的用户不应该报错: %v", err)
	}

	share, _ := mgr.GetShare("test-share")
	if len(share.ValidUsers) != 1 {
		t.Error("允许列表应该不变")
	}
}

// ========== GetUserShares 测试 ==========

func TestGetUserShares_Admin(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建多个共享
	for i := 1; i <= 3; i++ {
		sharePath := filepath.Join(tmpDir, "share%d")
		_ = mgr.CreateShare(&Share{
			Name:       "share" + string(rune('0'+i)),
			Path:       sharePath,
			ValidUsers: []string{"user1"},
		})
	}

	// 管理员应该能看到所有共享
	shares := mgr.GetUserShares("admin")
	if len(shares) != 3 {
		t.Errorf("管理员应该能看到所有共享，实际: %d", len(shares))
	}
}

func TestGetUserShares_RegularUser(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建不同权限的共享
	_ = mgr.CreateShare(&Share{
		Name:        "public-share",
		Path:        filepath.Join(tmpDir, "public"),
		GuestOK:     true,
		GuestAccess: true,
	})
	_ = mgr.CreateShare(&Share{
		Name:       "user1-share",
		Path:       filepath.Join(tmpDir, "user1"),
		ValidUsers: []string{"user1"},
	})
	_ = mgr.CreateShare(&Share{
		Name:       "user2-share",
		Path:       filepath.Join(tmpDir, "user2"),
		ValidUsers: []string{"user2"},
	})
	_ = mgr.CreateShare(&Share{
		Name:       "restricted-share",
		Path:       filepath.Join(tmpDir, "restricted"),
		ValidUsers: []string{"admin"},
	})

	// user1 应该能看到 public-share 和 user1-share
	shares := mgr.GetUserShares("user1")
	shareNames := make(map[string]bool)
	for _, s := range shares {
		shareNames[s.Name] = true
	}

	if !shareNames["public-share"] {
		t.Error("user1 应该能看到 public-share")
	}
	if !shareNames["user1-share"] {
		t.Error("user1 应该能看到 user1-share")
	}
	if shareNames["user2-share"] {
		t.Error("user1 不应该看到 user2-share")
	}
	if shareNames["restricted-share"] {
		t.Error("user1 不应该看到 restricted-share")
	}
}

func TestGetUserShares_NoUserManager(t *testing.T) {
	// 无用户管理器的情况
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name: "test-share",
		Path: filepath.Join(tmpDir, "test"),
	})

	// 无用户管理器时应该返回所有共享
	shares := mgr.GetUserShares("anyone")
	if len(shares) != 1 {
		t.Error("无用户管理器时应该返回所有共享")
	}
}

func TestGetUserShares_GuestAccess(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建访客共享
	_ = mgr.CreateShare(&Share{
		Name:        "guest-share",
		Path:        filepath.Join(tmpDir, "guest"),
		GuestOK:     true,
		GuestAccess: true,
	})

	// 创建受限共享
	_ = mgr.CreateShare(&Share{
		Name:       "restricted",
		Path:       filepath.Join(tmpDir, "restricted"),
		ValidUsers: []string{"admin"},
	})

	// 任何用户都能看到访客共享
	shares := mgr.GetUserShares("unknown")
	found := false
	for _, s := range shares {
		if s.Name == "guest-share" {
			found = true
			break
		}
	}
	if !found {
		t.Error("访客共享应该对所有用户可见")
	}
}

// ========== GetSharePath 测试 ==========

func TestGetSharePath_Basic(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	testPath := filepath.Join(tmpDir, "path-test")
	_ = mgr.CreateShare(&Share{
		Name: "path-test",
		Path: testPath,
	})

	path := mgr.GetSharePath("path-test")
	if path != testPath {
		t.Errorf("路径错误: 期望 %s, 实际 %s", testPath, path)
	}
}

func TestGetSharePath_NotExist(t *testing.T) {
	mgr, _, _ := setupPermissionTest(t)

	path := mgr.GetSharePath("nonexistent")
	if path != "" {
		t.Errorf("不存在的共享应该返回空字符串，实际: %s", path)
	}
}

// ========== 权限持久化测试 ==========

func TestPermissionPersistence(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享并设置权限
	sharePath := filepath.Join(tmpDir, "persist-share")
	_ = mgr.CreateShare(&Share{
		Name: "persist-share",
		Path: sharePath,
	})
	_ = mgr.SetSharePermission("persist-share", "user1", true)
	_ = mgr.SetSharePermission("persist-share", "user2", false)

	// 重新创建管理器加载配置
	mgr2, err := NewManagerWithUserMgr(mgr.userManager, mgr.configPath)
	if err != nil {
		t.Fatalf("重新创建管理器失败: %v", err)
	}

	// 验证权限已持久化
	share, err := mgr2.GetShare("persist-share")
	if err != nil {
		t.Fatalf("获取共享失败: %v", err)
	}

	if len(share.ValidUsers) != 2 {
		t.Errorf("允许列表应该有2个用户，实际: %d", len(share.ValidUsers))
	}

	if len(share.WriteList) != 1 || share.WriteList[0] != "user1" {
		t.Error("写权限应该已持久化")
	}
}

// ========== 权限边界测试 ==========

func TestPermissionWithEmptyValidUsers(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建无限制共享
	sharePath := filepath.Join(tmpDir, "open-share")
	_ = mgr.CreateShare(&Share{
		Name:       "open-share",
		Path:       sharePath,
		ValidUsers: []string{}, // 空允许列表
	})

	// 所有用户都能看到
	shares := mgr.GetUserShares("unknown")
	found := false
	for _, s := range shares {
		if s.Name == "open-share" {
			found = true
			break
		}
	}
	if !found {
		t.Error("无限制共享应该对所有用户可见")
	}
}

func TestPermissionUpdateExistingUser(t *testing.T) {
	mgr, _, tmpDir := setupPermissionTest(t)

	// 创建共享
	sharePath := filepath.Join(tmpDir, "update-perm")
	_ = mgr.CreateShare(&Share{
		Name: "update-perm",
		Path: sharePath,
	})

	// 先设置读权限
	_ = mgr.SetSharePermission("update-perm", "user1", false)

	// 再更新为写权限
	_ = mgr.SetSharePermission("update-perm", "user1", true)

	share, _ := mgr.GetShare("update-perm")

	// 验证用户在写列表中
	foundWrite := false
	for _, u := range share.WriteList {
		if u == "user1" {
			foundWrite = true
			break
		}
	}
	if !foundWrite {
		t.Error("用户应该在写列表中")
	}
}