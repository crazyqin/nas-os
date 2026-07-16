package webshare

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 配置测试 ==========

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, AccessModeReadOnly, cfg.DefaultAccessMode)
	assert.Equal(t, 10, cfg.DefaultMaxConcurrentAccess)
	assert.Equal(t, 30, cfg.DefaultSessionTimeoutMinutes)
	assert.True(t, cfg.DefaultFIPSEnabled)
}

func TestDefaultPermissions(t *testing.T) {
	ro := DefaultReadOnlyPermission()
	assert.True(t, ro.CanBrowse)
	assert.True(t, ro.CanDownload)
	assert.False(t, ro.CanUpload)
	assert.False(t, ro.CanMkdir)

	rw := DefaultReadWritePermission()
	assert.True(t, rw.CanBrowse)
	assert.True(t, rw.CanDownload)
	assert.True(t, rw.CanUpload)
	assert.True(t, rw.CanMkdir)

	full := DefaultFullPermission()
	assert.True(t, full.CanBrowse)
	assert.True(t, full.CanDownload)
	assert.True(t, full.CanUpload)
	assert.True(t, full.CanMkdir)
	assert.True(t, full.CanDelete)
	assert.True(t, full.CanRename)
	assert.True(t, full.CanShare)
}

func TestPermissionForMode(t *testing.T) {
	assert.True(t, permissionForMode(AccessModeReadOnly).CanBrowse)
	assert.True(t, permissionForMode(AccessModeReadWrite).CanUpload)
	assert.True(t, permissionForMode(AccessModeFull).CanDelete)
	assert.False(t, permissionForMode(AccessModeWriteOnly).CanBrowse)
}

// ========== 创建分享测试 ==========

func TestCreateShare(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	req := &CreateShareRequest{
		Name:       "项目文档分享",
		RootPath:   "/shared/projects",
		AccessMode: AccessModeReadOnly,
		CreatedBy:  "admin",
	}

	share, err := svc.CreateShare(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, share.ID)
	assert.Equal(t, "项目文档分享", share.Name)
	assert.Equal(t, "/shared/projects", share.RootPath)
	assert.NotEmpty(t, share.Token)
	assert.Equal(t, AccessModeReadOnly, share.AccessMode)
	assert.Equal(t, ShareStatusActive, share.Status)
	assert.True(t, share.FIPSEnabled) // 默认启用
	assert.False(t, share.PasswordEnabled)
	assert.NotNil(t, share.Permission)
	assert.True(t, share.Permission.CanBrowse)
}

func TestCreateShare_DisabledService(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	svc := NewService(cfg)
	ctx := context.Background()

	_, err := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "test", RootPath: "/test", CreatedBy: "admin",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未启用")
}

func TestCreateShare_DuplicateName(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	req := &CreateShareRequest{
		Name: "重复分享", RootPath: "/a", CreatedBy: "admin",
	}

	_, err := svc.CreateShare(ctx, req)
	require.NoError(t, err)

	_, err = svc.CreateShare(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestCreateShare_WithPassword(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	req := &CreateShareRequest{
		Name:      "加密分享",
		RootPath:  "/secure",
		CreatedBy: "admin",
		Password:  "secret123",
	}

	share, err := svc.CreateShare(ctx, req)
	require.NoError(t, err)
	assert.True(t, share.PasswordEnabled)
	assert.NotEmpty(t, share.PasswordHash)
	assert.Contains(t, share.PasswordHash, "sha256:")
}

func TestCreateShare_WithExpiry(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	req := &CreateShareRequest{
		Name:        "限时分享",
		RootPath:    "/tmp",
		CreatedBy:   "admin",
		ExpiryHours: 24,
	}

	share, err := svc.CreateShare(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, share.ExpiresAt)
	assert.False(t, share.ExpiresAt.IsZero())
}

func TestCreateShare_CustomPermission(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	customPerm := &SharePermission{
		CanBrowse:   true,
		CanDownload: true,
		CanUpload:   true,
		CanMkdir:    false,
		CanDelete:   false,
		CanRename:   false,
		CanShare:    false,
	}

	req := &CreateShareRequest{
		Name:       "自定义权限",
		RootPath:   "/custom",
		CreatedBy:  "admin",
		Permission: customPerm,
	}

	share, err := svc.CreateShare(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, customPerm, share.Permission)
	assert.True(t, share.Permission.CanUpload)
	assert.False(t, share.Permission.CanMkdir)
}

// ========== 获取分享测试 ==========

func TestGetShare(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "test", RootPath: "/test", CreatedBy: "admin",
	})

	got, err := svc.GetShare(ctx, share.ID)
	require.NoError(t, err)
	assert.Equal(t, share.ID, got.ID)
}

func TestGetShare_NotFound(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	_, err := svc.GetShare(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestGetShareByToken(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "token-test", RootPath: "/test", CreatedBy: "admin",
	})

	got, err := svc.GetShareByToken(ctx, share.Token)
	require.NoError(t, err)
	assert.Equal(t, share.ID, got.ID)
}

func TestGetShareByToken_Invalid(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	_, err := svc.GetShareByToken(ctx, "invalid-token")
	assert.Error(t, err)
}

// ========== 列出分享测试 ==========

func TestListShares(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	svc.CreateShare(ctx, &CreateShareRequest{Name: "S1", RootPath: "/a", CreatedBy: "admin"})
	svc.CreateShare(ctx, &CreateShareRequest{Name: "S2", RootPath: "/b", CreatedBy: "admin"})

	all, err := svc.ListShares(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 2)

	active, err := svc.ListShares(ctx, ShareStatusActive)
	require.NoError(t, err)
	assert.Len(t, active, 2)
}

// ========== 撤销分享测试 ==========

func TestRevokeShare(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "待撤销", RootPath: "/test", CreatedBy: "admin",
	})

	err := svc.RevokeShare(ctx, share.ID)
	require.NoError(t, err)

	got, _ := svc.GetShare(ctx, share.ID)
	assert.Equal(t, ShareStatusRevoked, got.Status)
}

func TestRevokeShare_NotFound(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	err := svc.RevokeShare(ctx, "nonexistent")
	assert.Error(t, err)
}

// ========== 删除分享测试 ==========

func TestDeleteShare(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "待删除", RootPath: "/test", CreatedBy: "admin",
	})

	err := svc.DeleteShare(ctx, share.ID)
	require.NoError(t, err)

	_, err = svc.GetShare(ctx, share.ID)
	assert.Error(t, err)
}

func TestDeleteShare_NotFound(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	err := svc.DeleteShare(ctx, "nonexistent")
	assert.Error(t, err)
}

// ========== 更新权限测试 ==========

func TestUpdateSharePermission(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "perm-test", RootPath: "/test", CreatedBy: "admin",
	})

	newPerm := &SharePermission{
		CanBrowse:   true,
		CanDownload: true,
		CanUpload:   true,
		CanMkdir:    true,
		CanDelete:   true,
		CanRename:   true,
		CanShare:    false,
	}

	updated, err := svc.UpdateSharePermission(ctx, share.ID, newPerm)
	require.NoError(t, err)
	assert.Equal(t, newPerm, updated.Permission)
	assert.True(t, updated.Permission.CanDelete)
}

// ========== 会话管理测试 ==========

func TestCreateSession(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "session-test", RootPath: "/test", CreatedBy: "admin",
	})

	sess, err := svc.CreateSession(ctx, share.Token, "192.168.1.100", "Mozilla/5.0", "")
	require.NoError(t, err)
	assert.NotEmpty(t, sess.ID)
	assert.Equal(t, share.ID, sess.ShareID)
	assert.True(t, sess.IsActive)
	assert.Equal(t, "192.168.1.100", sess.ClientIP)

	got, _ := svc.GetShare(ctx, share.ID)
	assert.Equal(t, 1, got.ActiveSessionCount)
}

func TestCreateSession_InvalidToken(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	_, err := svc.CreateSession(ctx, "invalid-token", "127.0.0.1", "", "")
	assert.Error(t, err)
}

func TestCreateSession_PasswordProtected(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name:      "protected",
		RootPath:  "/secure",
		CreatedBy: "admin",
		Password:  "mypassword",
	})

	// 错误密码
	_, err := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "wrong")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "密码错误")

	// 正确密码
	sess, err := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "mypassword")
	require.NoError(t, err)
	assert.True(t, sess.IsActive)
}

func TestCreateSession_RevokedShare(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "revoked", RootPath: "/test", CreatedBy: "admin",
	})
	svc.RevokeShare(ctx, share.ID)

	_, err := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")
	assert.Error(t, err)
}

func TestValidateSession(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "validate-test", RootPath: "/test", CreatedBy: "admin",
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	validSess, validShare, err := svc.ValidateSession(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, sess.ID, validSess.ID)
	assert.Equal(t, share.ID, validShare.ID)
}

func TestValidateSession_NotFound(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	_, _, err := svc.ValidateSession(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestDestroySession(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "destroy-test", RootPath: "/test", CreatedBy: "admin",
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	err := svc.DestroySession(ctx, sess.ID)
	require.NoError(t, err)

	got, _ := svc.GetSession(ctx, sess.ID)
	assert.False(t, got.IsActive)

	// ActiveSessionCount 应减少
	updated, _ := svc.GetShare(ctx, share.ID)
	assert.Equal(t, 0, updated.ActiveSessionCount)
}

// ========== 文件操作权限测试 ==========

func TestListFiles_PermissionDenied(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "no-browse", RootPath: "/test", CreatedBy: "admin",
		AccessMode: AccessModeWriteOnly, // 无浏览权限
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	_, err := svc.ListFiles(ctx, sess.ID, "/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无浏览权限")
}

func TestCreateFolder_PermissionDenied(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "no-mkdir", RootPath: "/test", CreatedBy: "admin",
		AccessMode: AccessModeReadOnly,
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	err := svc.CreateFolder(ctx, sess.ID, "/newdir")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无创建文件夹权限")
}

func TestCreateFolder_Success(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "mkdir-ok", RootPath: "/test", CreatedBy: "admin",
		AccessMode: AccessModeReadWrite,
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	err := svc.CreateFolder(ctx, sess.ID, "/newdir")
	require.NoError(t, err)
}

func TestUploadFile_PermissionDenied(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "no-upload", RootPath: "/test", CreatedBy: "admin",
		AccessMode: AccessModeReadOnly,
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	err := svc.UploadFile(ctx, sess.ID, "/file.txt", 1024)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无上传权限")
}

func TestDownloadFile_PermissionDenied(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "no-download", RootPath: "/test", CreatedBy: "admin",
		AccessMode: AccessModeWriteOnly,
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	err := svc.DownloadFile(ctx, sess.ID, "/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无下载权限")
}

func TestDeleteFile_PermissionDenied(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "no-delete", RootPath: "/test", CreatedBy: "admin",
		AccessMode: AccessModeReadWrite,
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	err := svc.DeleteFile(ctx, sess.ID, "/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无删除权限")
}

func TestRenameFile_PermissionDenied(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "no-rename", RootPath: "/test", CreatedBy: "admin",
		AccessMode: AccessModeReadWrite,
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	err := svc.RenameFile(ctx, sess.ID, "/old.txt", "/new.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无重命名权限")
}

// ========== 并发限制测试 ==========

func TestMaxConcurrentAccess(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name:                "limited",
		RootPath:            "/test",
		CreatedBy:           "admin",
		MaxConcurrentAccess: 2,
	})

	// 创建 2 个会话（达到上限）
	_, err := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")
	require.NoError(t, err)

	_, err = svc.CreateSession(ctx, share.Token, "127.0.0.2", "", "")
	require.NoError(t, err)

	// 第 3 个应失败
	_, err = svc.CreateSession(ctx, share.Token, "127.0.0.3", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大并发")
}

// ========== 分享链接测试 ==========

func TestGenerateShareLink(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "link-test", RootPath: "/test", CreatedBy: "admin",
	})

	link, err := svc.GenerateShareLink(ctx, share.ID, "https://nas.example.com")
	require.NoError(t, err)
	assert.Contains(t, link.URL, "https://nas.example.com/share/")
	assert.Equal(t, share.Token, link.Token)
}

func TestGenerateShareLink_RevokedShare(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "revoked-link", RootPath: "/test", CreatedBy: "admin",
	})

	svc.RevokeShare(ctx, share.ID)

	_, err := svc.GenerateShareLink(ctx, share.ID, "https://nas.example.com")
	assert.Error(t, err)
}

// ========== FIPS 测试 ==========

func TestToggleFIPS(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "fips-test", RootPath: "/test", CreatedBy: "admin",
	})
	assert.True(t, share.FIPSEnabled)

	// 禁用
	share, err := svc.DisableFIPS(ctx, share.ID)
	require.NoError(t, err)
	assert.False(t, share.FIPSEnabled)

	// 重新启用
	share, err = svc.EnableFIPS(ctx, share.ID)
	require.NoError(t, err)
	assert.True(t, share.FIPSEnabled)
}

func TestDisableFIPS_NotFound(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	_, err := svc.DisableFIPS(ctx, "nonexistent")
	assert.Error(t, err)
}

// ========== 密码管理测试 ==========

func TestSetPassword(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "pwd-test", RootPath: "/test", CreatedBy: "admin",
	})
	assert.False(t, share.PasswordEnabled)

	// 设置密码
	err := svc.SetPassword(ctx, share.ID, "newpassword")
	require.NoError(t, err)

	got, _ := svc.GetShare(ctx, share.ID)
	assert.True(t, got.PasswordEnabled)

	// 清除密码
	err = svc.SetPassword(ctx, share.ID, "")
	require.NoError(t, err)

	got, _ = svc.GetShare(ctx, share.ID)
	assert.False(t, got.PasswordEnabled)
}

func TestVerifyPassword(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name:      "verify-pwd",
		RootPath:  "/test",
		CreatedBy: "admin",
		Password:  "test123",
	})

	// 正确密码
	err := svc.VerifyPassword(ctx, share.ID, "test123")
	assert.NoError(t, err)

	// 错误密码
	err = svc.VerifyPassword(ctx, share.ID, "wrong")
	assert.Error(t, err)
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	share1, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "stats-1", RootPath: "/a", CreatedBy: "admin",
	})
	fipsFalse := false
	svc.CreateShare(ctx, &CreateShareRequest{
		Name: "stats-2", RootPath: "/b", CreatedBy: "admin",
		FIPSEnabled: &fipsFalse,
	})

	svc.CreateSession(ctx, share1.Token, "127.0.0.1", "", "")
	svc.CreateSession(ctx, share1.Token, "127.0.0.2", "", "")

	stats, err := svc.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalShares)
	assert.Equal(t, 2, stats.ActiveShares)
	assert.Equal(t, 2, stats.TotalSessions)
	assert.Equal(t, 2, stats.ActiveSessions)
	assert.Equal(t, 1, stats.FIPSEnabledShares) // 仅 share1 默认启用 FIPS
}

// ========== 清理过期会话测试 ==========

func TestCleanupExpiredSessions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultSessionTimeoutMinutes = 0 // 会导致立即过期
	svc := NewService(cfg)
	ctx := context.Background()

	share, _ := svc.CreateShare(ctx, &CreateShareRequest{
		Name: "cleanup-test", RootPath: "/test", CreatedBy: "admin",
	})

	sess, _ := svc.CreateSession(ctx, share.Token, "127.0.0.1", "", "")

	// 手动设置过期时间
	svc.mu.Lock()
	sess.ExpiresAt = sess.CreatedAt.Add(-1 * 60 * 60 * 60 * 1e9) // 设置为很久以前
	svc.mu.Unlock()

	count := svc.CleanupExpiredSessions()
	assert.GreaterOrEqual(t, count, 1)

	got, _ := svc.GetSession(ctx, sess.ID)
	assert.False(t, got.IsActive)
}

// ========== 配置更新测试 ==========

func TestUpdateConfig(t *testing.T) {
	svc := NewService(nil)

	newCfg := DefaultConfig()
	newCfg.DefaultMaxConcurrentAccess = 100
	svc.UpdateConfig(newCfg)

	got := svc.GetConfig()
	assert.Equal(t, 100, got.DefaultMaxConcurrentAccess)
}
