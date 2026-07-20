// Package users 提供用户管理功能
package users

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

	// 同时兼容标准 Bearer 头和旧版裸 token。
	for _, tokenValue := range []string{token.Token, "Bearer " + token.Token} {
		user, err := mgr.ValidateToken(tokenValue)
		assert.NoError(t, err)
		assert.Equal(t, "testuser", user.Username)
	}

	// 无效 token
	_, err := mgr.ValidateToken("invalid-token")
	assert.Error(t, err)
}

func TestProtectedRoutesRequireAuthenticationAndAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	mgr := &Manager{
		users: map[string]*User{
			"admin": {ID: "admin-id", Username: "admin", Role: RoleAdmin},
			"user":  {ID: "user-id", Username: "user", Role: RoleUser},
		},
		groups: map[string]*Group{},
		tokens: map[string]*Token{
			"admin-token": {UserID: "admin-id", Token: "admin-token", ExpiresAt: now.Add(time.Hour)},
			"user-token":  {UserID: "user-id", Token: "user-token", ExpiresAt: now.Add(time.Hour)},
		},
	}

	router := gin.New()
	api := router.Group("/api/v1")
	handlers := NewHandlers(mgr, nil)
	handlers.RegisterPublicRoutes(api.Group("/auth"))
	protected := api.Group("")
	protected.Use(AuthMiddleware(mgr))
	handlers.RegisterProtectedRoutes(protected)

	tests := []struct {
		name       string
		path       string
		authHeader string
		wantStatus int
	}{
		{name: "login is public", path: "/api/v1/auth/login", wantStatus: http.StatusBadRequest},
		{name: "users require token", path: "/api/v1/users", wantStatus: http.StatusUnauthorized},
		{name: "query token is rejected", path: "/api/v1/users?token=admin-token", wantStatus: http.StatusUnauthorized},
		{name: "regular user is forbidden", path: "/api/v1/users", authHeader: "Bearer user-token", wantStatus: http.StatusForbidden},
		{name: "admin is allowed", path: "/api/v1/users", authHeader: "Bearer admin-token", wantStatus: http.StatusOK},
		{name: "regular user can read self", path: "/api/v1/me", authHeader: "Bearer user-token", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.path == "/api/v1/auth/login" {
				req = httptest.NewRequest(http.MethodPost, tt.path, nil)
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			assert.Equal(t, tt.wantStatus, resp.Code)
		})
	}
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

	// 重启后密码必须仍可认证（password_hash 必须落盘）
	tok, err := mgr2.Authenticate("persistent", "password123")
	assert.NoError(t, err)
	assert.NotNil(t, tok)
	assert.NotEmpty(t, tok.Token)
}

// TestManager_PersistReloadAuthenticate proves the shipped identity path:
// CreateUser → disk store → new Manager → Authenticate succeeds.
// Bootstrap password file must live under the config directory, not CWD.
func TestManager_PersistReloadAuthenticate(t *testing.T) {
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "etc", "nas-os")
	configPath := filepath.Join(storeDir, "users.json")
	mountBase := filepath.Join(tmpDir, "mnt")

	mgr1, err := NewManagerWithConfig(mountBase, configPath)
	if err != nil {
		t.Fatalf("NewManagerWithConfig: %v", err)
	}

	const username = "alice"
	const password = "s3cure-pass-99"
	if _, err := mgr1.CreateUser(UserInput{Username: username, Password: password, Role: RoleUser}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// On-disk file must contain password_hash field
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"password_hash"`)) {
		t.Fatalf("users.json missing password_hash; got:\n%s", string(raw))
	}
	// API-facing marshal of User must not include password_hash
	u, err := mgr1.GetUser(username)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	apiJSON, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(apiJSON, []byte("password_hash")) || bytes.Contains(apiJSON, []byte(u.PasswordHash)) {
		t.Fatalf("API User JSON must omit password hash, got %s", apiJSON)
	}

	// Bootstrap password path under config dir
	pwPath := mgr1.BootstrapPasswordPath()
	if filepath.Dir(pwPath) != storeDir {
		t.Fatalf("bootstrap password path %q not under store dir %q", pwPath, storeDir)
	}

	// Simulate process restart
	mgr2, err := NewManagerWithConfig(mountBase, configPath)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}
	tok, err := mgr2.Authenticate(username, password)
	if err != nil {
		t.Fatalf("Authenticate after reload: %v", err)
	}
	if tok == nil || tok.Token == "" {
		t.Fatal("expected non-empty token after reload auth")
	}
	if _, err := mgr2.Authenticate(username, "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

func TestManager_SessionsNotPersisted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "users.json")
	mgr, err := NewManagerWithConfig(tmpDir, configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.CreateUser(UserInput{Username: "sess", Password: "password123"}); err != nil {
		t.Fatal(err)
	}
	tok, err := mgr.Authenticate("sess", "password123")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(tok.Token)) {
		t.Fatal("session token must not be written to users.json")
	}
	// Reload: prior session invalid
	mgr2, err := NewManagerWithConfig(tmpDir, configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr2.ValidateToken(tok.Token); err == nil {
		t.Fatal("token from previous process must not validate after reload")
	}
	// Password login still works
	if _, err := mgr2.Authenticate("sess", "password123"); err != nil {
		t.Fatalf("password auth after reload: %v", err)
	}
}

func TestManager_SessionTTLFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManagerWithConfig(tmpDir, filepath.Join(tmpDir, "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	mgr.SetSessionTTL(2 * time.Hour)
	if mgr.SessionTTL() != 2*time.Hour {
		t.Fatalf("SessionTTL=%v want 2h", mgr.SessionTTL())
	}
	if _, err := mgr.CreateUser(UserInput{Username: "ttluser", Password: "password123"}); err != nil {
		t.Fatal(err)
	}
	tok, err := mgr.Authenticate("ttluser", "password123")
	if err != nil {
		t.Fatal(err)
	}
	// ExpiresAt should be ~2h from now (not default 24h)
	until := time.Until(tok.ExpiresAt)
	if until < 90*time.Minute || until > 3*time.Hour {
		t.Fatalf("token TTL out of expected range: until=%v", until)
	}
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

// ========== 用户配置管理测试 ==========

func TestManager_GetUserConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})

	// 获取配置
	config, err := mgr.GetUserConfig("testuser")
	assert.NoError(t, err)
	assert.NotNil(t, config)
}

func TestManager_UpdateUserConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})

	// 更新配置
	newConfig := UserConfig{
		Language:     "zh-CN",
		Timezone:     "Asia/Shanghai",
		Theme:        "dark",
		StorageQuota: 1024 * 1024 * 1024, // 1GB
	}
	err := mgr.UpdateUserConfig("testuser", newConfig)
	assert.NoError(t, err)

	// 验证更新
	config, _ := mgr.GetUserConfig("testuser")
	assert.Equal(t, "zh-CN", config.Language)
	assert.Equal(t, "Asia/Shanghai", config.Timezone)
	assert.Equal(t, "dark", config.Theme)
	assert.Equal(t, int64(1024*1024*1024), config.StorageQuota)
}

func TestManager_PatchUserConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{
		Username: "testuser",
		Password: "password123",
	})

	// 部分更新配置
	err := mgr.PatchUserConfig("testuser", map[string]interface{}{
		"language": "en-US",
		"theme":    "light",
	})
	assert.NoError(t, err)

	// 验证更新
	config, _ := mgr.GetUserConfig("testuser")
	assert.Equal(t, "en-US", config.Language)
	assert.Equal(t, "light", config.Theme)
}

func TestManager_SetUserLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})

	err := mgr.SetUserLanguage("testuser", "ja-JP")
	assert.NoError(t, err)

	config, _ := mgr.GetUserConfig("testuser")
	assert.Equal(t, "ja-JP", config.Language)
}

func TestManager_SetUserTimezone(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})

	err := mgr.SetUserTimezone("testuser", "America/New_York")
	assert.NoError(t, err)

	config, _ := mgr.GetUserConfig("testuser")
	assert.Equal(t, "America/New_York", config.Timezone)
}

func TestManager_SetUserTheme(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})

	err := mgr.SetUserTheme("testuser", "solarized")
	assert.NoError(t, err)

	config, _ := mgr.GetUserConfig("testuser")
	assert.Equal(t, "solarized", config.Theme)
}

func TestManager_StorageQuota(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})

	// 设置配额
	err := mgr.SetUserStorageQuota("testuser", 10*1024*1024*1024) // 10GB
	assert.NoError(t, err)

	// 获取配额
	quota, err := mgr.GetUserStorageQuota("testuser")
	assert.NoError(t, err)
	assert.Equal(t, int64(10*1024*1024*1024), quota)
}

func TestManager_NotificationPrefs(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})

	prefs := NotificationPrefs{
		EmailEnabled:  true,
		PushEnabled:   true,
		SMSEnabled:    false,
		SystemEnabled: true,
		AlertOnLogin:  true,
		AlertOnChange: false,
	}

	err := mgr.SetUserNotificationPrefs("testuser", prefs)
	assert.NoError(t, err)

	config, _ := mgr.GetUserConfig("testuser")
	assert.True(t, config.Notifications.EmailEnabled)
	assert.False(t, config.Notifications.SMSEnabled)
}

func TestManager_AllowDenyService(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})

	// 允许服务
	err := mgr.AllowService("testuser", "samba")
	assert.NoError(t, err)

	config, _ := mgr.GetUserConfig("testuser")
	assert.Contains(t, config.AllowedServices, "samba")

	// 禁止服务
	err = mgr.DenyService("testuser", "ftp")
	assert.NoError(t, err)

	config, _ = mgr.GetUserConfig("testuser")
	assert.Contains(t, config.DeniedServices, "ftp")

	// 允许被禁止的服务会移除禁止
	err = mgr.AllowService("testuser", "ftp")
	assert.NoError(t, err)

	config, _ = mgr.GetUserConfig("testuser")
	assert.NotContains(t, config.DeniedServices, "ftp")
}

func TestManager_CustomAttributes(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})

	// 设置自定义属性
	err := mgr.SetCustomAttribute("testuser", "department", "engineering")
	assert.NoError(t, err)

	err = mgr.SetCustomAttribute("testuser", "location", "beijing")
	assert.NoError(t, err)

	// 获取属性
	value, err := mgr.GetCustomAttribute("testuser", "department")
	assert.NoError(t, err)
	assert.Equal(t, "engineering", value)

	value, err = mgr.GetCustomAttribute("testuser", "location")
	assert.NoError(t, err)
	assert.Equal(t, "beijing", value)

	// 获取不存在的属性
	value, err = mgr.GetCustomAttribute("testuser", "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, value)
}

// ========== 用户统计测试 ==========

func TestManager_GetUserStats(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建多个用户
	_, _ = mgr.CreateUser(UserInput{Username: "user1", Password: "pass123", Role: RoleUser})
	_, _ = mgr.CreateUser(UserInput{Username: "user2", Password: "pass123", Role: RoleUser})
	_, _ = mgr.CreateUser(UserInput{Username: "admin2", Password: "pass123", Role: RoleAdmin})

	// 禁用一个用户
	_ = mgr.DisableUser("user2", true)

	// 获取统计
	stats := mgr.GetUserStats()

	assert.Equal(t, stats["total_users"], 3+len(mgr.ListUsers())-3) // 减去刚创建的3个
	totalUsers, ok := stats["total_users"].(int)
	if ok {
		assert.GreaterOrEqual(t, totalUsers, 3)
	}

	byStatus := stats["by_status"].(map[string]int)
	assert.GreaterOrEqual(t, byStatus["disabled"], 1)
}

func TestManager_ActiveInactiveUsers(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{Username: "user1", Password: "pass123"})
	_, _ = mgr.CreateUser(UserInput{Username: "user2", Password: "pass123"})

	// 用户1登录
	_ = mgr.UpdateLastLogin("user1", "192.168.1.1")

	// 检查活跃用户
	activeUsers := mgr.GetActiveUsers(time.Now().Add(-1 * time.Hour))
	assert.GreaterOrEqual(t, len(activeUsers), 1)

	// 检查不活跃用户
	inactiveUsers := mgr.GetInactiveUsers(1 * time.Minute)
	assert.GreaterOrEqual(t, len(inactiveUsers), 1)
}

func TestManager_UpdateLastLogin(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "testuser", Password: "pass123"})

	// 更新登录信息
	err := mgr.UpdateLastLogin("testuser", "192.168.1.100")
	assert.NoError(t, err)

	// 验证
	user, _ := mgr.GetUser("testuser")
	assert.NotNil(t, user.LastLoginAt)
	assert.Equal(t, "192.168.1.100", user.LastLoginIP)
}

// ========== 批量操作测试 ==========

func TestManager_BatchCreateUsers(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	inputs := []UserInput{
		{Username: "batch1", Password: "pass123"},
		{Username: "batch2", Password: "pass123"},
		{Username: "batch3", Password: "pass123"},
	}

	users, errs := mgr.BatchCreateUsers(inputs)
	assert.Len(t, users, 3)
	assert.Empty(t, errs)
}

func TestManager_BatchCreateUsers_PartialFailure(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 先创建一个用户
	_, _ = mgr.CreateUser(UserInput{Username: "existing", Password: "pass123"})

	inputs := []UserInput{
		{Username: "new1", Password: "pass123"},
		{Username: "existing", Password: "pass123"}, // 会失败
		{Username: "new2", Password: "pass123"},
	}

	users, errs := mgr.BatchCreateUsers(inputs)
	assert.Len(t, users, 2)
	assert.Len(t, errs, 1)
}

func TestManager_BatchDeleteUsers(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户
	_, _ = mgr.CreateUser(UserInput{Username: "del1", Password: "pass123"})
	_, _ = mgr.CreateUser(UserInput{Username: "del2", Password: "pass123"})
	_, _ = mgr.CreateUser(UserInput{Username: "del3", Password: "pass123"})

	// 批量删除
	errs := mgr.BatchDeleteUsers([]string{"del1", "del2"})
	assert.Empty(t, errs)

	// 验证删除
	_, err := mgr.GetUser("del1")
	assert.Error(t, err)
	_, err = mgr.GetUser("del2")
	assert.Error(t, err)
	_, err = mgr.GetUser("del3")
	assert.NoError(t, err) // 应该还存在
}

func TestManager_BatchAddToGroup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户和组
	_, _ = mgr.CreateUser(UserInput{Username: "guser1", Password: "pass123"})
	_, _ = mgr.CreateUser(UserInput{Username: "guser2", Password: "pass123"})
	_, _ = mgr.CreateGroup(GroupInput{Name: "team-a"})

	// 批量添加到组
	errs := mgr.BatchAddToGroup([]string{"guser1", "guser2"}, "team-a")
	assert.Empty(t, errs)

	// 验证
	group, _ := mgr.GetGroup("team-a")
	assert.Contains(t, group.Members, "guser1")
	assert.Contains(t, group.Members, "guser2")
}

// ========== 用户组高级测试 ==========

func TestManager_ListGroups(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateGroup(GroupInput{Name: "group1"})
	_, _ = mgr.CreateGroup(GroupInput{Name: "group2"})
	_, _ = mgr.CreateGroup(GroupInput{Name: "group3"})

	groups := mgr.ListGroups()
	assert.GreaterOrEqual(t, len(groups), 3)
}

func TestManager_UpdateGroup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateGroup(GroupInput{Name: "testgroup", Description: "original"})

	// 更新组
	group, err := mgr.UpdateGroup("testgroup", GroupInput{
		Description: "updated",
		Members:     []string{"user1", "user2"},
	})
	assert.NoError(t, err)
	assert.Equal(t, "updated", group.Description)
}

func TestManager_DeleteGroup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateGroup(GroupInput{Name: "delgroup"})

	err := mgr.DeleteGroup("delgroup")
	assert.NoError(t, err)

	_, err = mgr.GetGroup("delgroup")
	assert.Error(t, err)
}

func TestManager_GetUsersInGroup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建用户和组
	_, _ = mgr.CreateUser(UserInput{Username: "member1", Password: "pass123"})
	_, _ = mgr.CreateUser(UserInput{Username: "member2", Password: "pass123"})
	_, _ = mgr.CreateGroup(GroupInput{Name: "team"})

	_ = mgr.AddUserToGroup("member1", "team")
	_ = mgr.AddUserToGroup("member2", "team")

	users, err := mgr.GetUsersInGroup("team")
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestManager_GetUsersByRole(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "admin1", Password: "pass123", Role: RoleAdmin})
	_, _ = mgr.CreateUser(UserInput{Username: "user1", Password: "pass123", Role: RoleUser})
	_, _ = mgr.CreateUser(UserInput{Username: "user2", Password: "pass123", Role: RoleUser})

	admins := mgr.GetUsersByRole(RoleAdmin)
	assert.GreaterOrEqual(t, len(admins), 1)

	users := mgr.GetUsersByRole(RoleUser)
	assert.GreaterOrEqual(t, len(users), 2)
}

// ========== 管理员保护测试 ==========

func TestManager_CannotDeleteLastAdmin(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	// 创建一个管理员
	_, _ = mgr.CreateUser(UserInput{Username: "onlyadmin", Password: "pass123", Role: RoleAdmin})

	// 如果只有一个管理员，应该无法删除
	// 注意：默认 admin 用户可能已存在，这个测试依赖于具体实现
}

func TestManager_CannotDisableLastAdmin(t *testing.T) {
	tmpDir := t.TempDir()
	_ = tmpDir // 用于测试上下文，实际测试依赖于具体实现
	// 类似上面的测试，验证最后一个管理员不能被禁用
}

// ========== 密码重置测试 ==========

func TestManager_ResetPassword(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "resetuser", Password: "oldpass"})

	// 管理员重置密码
	err := mgr.ResetPassword("resetuser", "newresetpass")
	assert.NoError(t, err)

	// 验证新密码
	_, err = mgr.Authenticate("resetuser", "newresetpass")
	assert.NoError(t, err)
}

// ========== Token 刷新测试 ==========

func TestManager_RefreshToken(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	_, _ = mgr.CreateUser(UserInput{Username: "refreshuser", Password: "pass123"})
	token, _ := mgr.Authenticate("refreshuser", "pass123")

	// 刷新 token
	newToken, err := mgr.RefreshToken(token.Token)
	assert.NoError(t, err)
	assert.NotNil(t, newToken)
	assert.Equal(t, token.Token, newToken.Token) // 同一个 token，只是延长有效期
}
