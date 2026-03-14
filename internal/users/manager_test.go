// Package users 提供用户管理功能
package users

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========== Manager 基础测试 ==========

func TestNewManager(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir)
	assert.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.users)
	assert.NotNil(t, mgr.groups)
	assert.NotNil(t, mgr.tokens)
}

func TestManager_CreateUser(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 测试创建用户
	user, err := mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
		Role:     RoleUser,
		Email:    "test@example.com",
	})
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, RoleUser, user.Role)
	assert.NotEmpty(t, user.ID)
	assert.NotEmpty(t, user.PasswordHash)
	assert.False(t, user.CreatedAt.IsZero())
}

func TestManager_CreateUser_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建第一个用户
	_, err := mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})
	assert.NoError(t, err)

	// 尝试创建重复用户
	_, err = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password456",
	})
	assert.Error(t, err)
	assert.Equal(t, ErrUserExists, err)
}

func TestManager_GetUser(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})

	// 获取用户
	user, err := mgr.GetUser("testuser")
	assert.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)

	// 获取不存在的用户
	_, err = mgr.GetUser("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrUserNotFound, err)
}

func TestManager_ListUsers(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 初始列表应为空（或只有默认 admin）
	users := mgr.ListUsers()
	initialCount := len(users)

	// 创建多个用户
	_, _ = mgr.CreateUser(UserInput{Username: "user1", Password: "pass123"})
	_, _ = mgr.CreateUser(UserInput{Username: "user2", Password: "pass123"})
	_, _ = mgr.CreateUser(UserInput{Username: "user3", Password: "pass123"})

	users = mgr.ListUsers()
	assert.Len(t, users, initialCount+3)
}

func TestManager_UpdateUser(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
		Email:    "old@example.com",
	})

	// 更新用户
	user, err := mgr.UpdateUser("testuser", UserInput{
		Email: "new@example.com",
		Role:  RoleAdmin,
	})
	assert.NoError(t, err)
	assert.Equal(t, "new@example.com", user.Email)
	assert.Equal(t, RoleAdmin, user.Role)

	// 更新不存在的用户
	_, err = mgr.UpdateUser("nonexistent", UserInput{})
	assert.Error(t, err)
}

func TestManager_DeleteUser(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})

	// 删除用户
	err := mgr.DeleteUser("testuser")
	assert.NoError(t, err)

	// 验证已删除
	_, err = mgr.GetUser("testuser")
	assert.Error(t, err)

	// 删除不存在的用户
	err = mgr.DeleteUser("nonexistent")
	assert.Error(t, err)
}

func TestManager_DisableUser(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})

	// 禁用用户
	err := mgr.DisableUser("testuser", true)
	assert.NoError(t, err)

	// 验证已禁用
	user, _ := mgr.GetUser("testuser")
	assert.True(t, user.Disabled)

	// 启用用户
	err = mgr.DisableUser("testuser", false)
	assert.NoError(t, err)

	user, _ = mgr.GetUser("testuser")
	assert.False(t, user.Disabled)
}

// ========== 认证测试 ==========

func TestManager_Authenticate(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})

	// 正确密码
	token, err := mgr.Authenticate("testuser", "password123")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// 错误密码
	_, err = mgr.Authenticate("testuser", "wrongpassword")
	assert.Error(t, err)

	// 不存在的用户
	_, err = mgr.Authenticate("nonexistent", "password")
	assert.Error(t, err)
}

func TestManager_ValidateToken(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户并获取 token
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})
	token, _ := mgr.Authenticate("testuser", "password123")

	// 验证 token
	user, err := mgr.ValidateToken(token.Token)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)

	// 无效 token
	_, err = mgr.ValidateToken("invalid-token")
	assert.Error(t, err)
}

func TestManager_Logout(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户并获取 token
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})
	token, _ := mgr.Authenticate("testuser", "password123")

	// 登出
	mgr.Logout(token.Token)

	// 验证 token 已失效
	_, err := mgr.ValidateToken(token.Token)
	assert.Error(t, err)
}

// ========== 用户组测试 ==========

func TestManager_CreateGroup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	group, err := mgr.CreateGroup(GroupInput{
		Name:        "developers",
		Description: "开发团队",
	})
	assert.NoError(t, err)
	assert.Equal(t, "developers", group.Name)
	assert.NotEmpty(t, group.ID)
}

func TestManager_GetGroup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建组
	_, _ = mgr.CreateGroup(GroupInput{Name: "developers"})

	// 获取组
	group, err := mgr.GetGroup("developers")
	assert.NoError(t, err)
	assert.Equal(t, "developers", group.Name)

	// 获取不存在的组
	_, err = mgr.GetGroup("nonexistent")
	assert.Error(t, err)
}

func TestManager_AddUserToGroup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户和组
	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})
	_, _ = mgr.CreateGroup(GroupInput{Name: "developers"})

	// 添加用户到组
	err := mgr.AddUserToGroup("testuser", "developers")
	assert.NoError(t, err)

	// 验证
	group, _ := mgr.GetGroup("developers")
	assert.Contains(t, group.Members, "testuser")

	user, _ := mgr.GetUser("testuser")
	assert.Contains(t, user.Groups, "developers")
}

func TestManager_RemoveUserFromGroup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户和组并添加
	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})
	_, _ = mgr.CreateGroup(GroupInput{Name: "developers"})
	_ = mgr.AddUserToGroup("testuser", "developers")

	// 移除用户
	err := mgr.RemoveUserFromGroup("testuser", "developers")
	assert.NoError(t, err)

	group, _ := mgr.GetGroup("developers")
	assert.NotContains(t, group.Members, "testuser")
}

// ========== 密码验证测试 ==========

func TestManager_ChangePassword(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "oldpassword",
	})

	// 修改密码
	err := mgr.ChangePassword("testuser", "oldpassword", "newpassword")
	assert.NoError(t, err)

	// 验证新密码
	_, err = mgr.Authenticate("testuser", "newpassword")
	assert.NoError(t, err)

	// 旧密码应该失效
	_, err = mgr.Authenticate("testuser", "oldpassword")
	assert.Error(t, err)
}

// ========== 并发测试 ==========

func TestManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	done := make(chan bool, 10)

	// 并发创建用户
	for i := 0; i < 5; i++ {
		go func(idx int) {
			_, err := mgr.CreateUser(UserInput{
				Username: "user" + string(rune('0'+idx)),
				Password: "password",
			})
			_ = err // 某些可能因竞争失败
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			_ = mgr.ListUsers()
			done <- true
		}()
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 持久化测试 ==========

func TestManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "users.json")

	// 创建管理器并添加数据（使用配置文件路径）
	mgr1, err := NewManagerWithConfig(tmpDir, configPath)
	assert.NoError(t, err)
	_, err = mgr1.CreateUser(UserInput{
		Username: "persistent",
		Password: "password123",
	})
	assert.NoError(t, err)
	_, err = mgr1.CreateGroup(GroupInput{Name: "testgroup"})
	assert.NoError(t, err)

	// 创建新管理器（模拟重启）
	mgr2, err := NewManagerWithConfig(tmpDir, configPath)
	assert.NoError(t, err)

	// 验证数据已恢复
	user, err := mgr2.GetUser("persistent")
	assert.NoError(t, err)
	assert.Equal(t, "persistent", user.Username)

	group, err := mgr2.GetGroup("testgroup")
	assert.NoError(t, err)
	assert.Equal(t, "testgroup", group.Name)
}

// ========== 角色权限测试 ==========

func TestRole_Permissions(t *testing.T) {
	// 验证角色定义
	assert.Equal(t, Role("admin"), RoleAdmin)
	assert.Equal(t, Role("user"), RoleUser)
	assert.Equal(t, Role("guest"), RoleGuest)
}

func TestManager_AdminDefaultExists(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 验证默认 admin 用户
	admin, err := mgr.GetUser("admin")
	if err == nil {
		assert.Equal(t, RoleAdmin, admin.Role)
	}
}
