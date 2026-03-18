package webdav

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========== NoOpQuotaProvider Tests ==========

func TestNoOpQuotaProvider_CheckQuota(t *testing.T) {
	provider := &NoOpQuotaProvider{}

	available, err := provider.CheckQuota("testuser")
	assert.NoError(t, err)
	assert.Equal(t, int64(-1), available) // -1 means unlimited
}

func TestNoOpQuotaProvider_ConsumeQuota(t *testing.T) {
	provider := &NoOpQuotaProvider{}

	err := provider.ConsumeQuota("testuser", 1024)
	assert.NoError(t, err)
}

func TestNoOpQuotaProvider_ReleaseQuota(t *testing.T) {
	provider := &NoOpQuotaProvider{}

	err := provider.ReleaseQuota("testuser", 1024)
	assert.NoError(t, err)
}

func TestNoOpQuotaProvider_GetUsage(t *testing.T) {
	provider := &NoOpQuotaProvider{}

	used, total, err := provider.GetUsage("testuser")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), used)
	assert.Equal(t, int64(0), total)
}

// ========== QuotaProvider Interface Tests ==========

func TestQuotaProvider_Interface(t *testing.T) {
	// Verify NoOpQuotaProvider implements QuotaProvider
	var _ QuotaProvider = (*NoOpQuotaProvider)(nil)
}

// ========== Config Tests ==========

func TestConfig_Defaults(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, 8081, config.Port)
	assert.Equal(t, "/data", config.RootPath)
	assert.False(t, config.AllowGuest)
	assert.Equal(t, int64(0), config.MaxUploadSize)
}

func TestConfig_Custom(t *testing.T) {
	config := &Config{
		Enabled:       false,
		Port:          9999,
		RootPath:      "/custom/path",
		AllowGuest:    true,
		MaxUploadSize: 1024 * 1024 * 100, // 100MB
	}

	assert.False(t, config.Enabled)
	assert.Equal(t, 9999, config.Port)
	assert.Equal(t, "/custom/path", config.RootPath)
	assert.True(t, config.AllowGuest)
	assert.Equal(t, int64(1024*1024*100), config.MaxUploadSize)
}

// ========== Server Tests ==========

func TestServer_NewServer_NilConfig(t *testing.T) {
	srv, err := NewServer(nil)

	assert.NoError(t, err)
	assert.NotNil(t, srv)
	assert.NotNil(t, srv.config)
	assert.Equal(t, 8081, srv.config.Port)
}

func TestServer_NewServer_CustomConfig(t *testing.T) {
	config := &Config{
		Enabled:    true,
		Port:       7070,
		RootPath:   "/tmp/test",
		AllowGuest: true,
	}

	srv, err := NewServer(config)

	assert.NoError(t, err)
	assert.NotNil(t, srv)
	assert.Equal(t, 7070, srv.config.Port)
	assert.True(t, srv.config.AllowGuest)
}

func TestServer_LockManager_Initialized(t *testing.T) {
	srv, err := NewServer(nil)
	assert.NoError(t, err)
	assert.NotNil(t, srv.lockManager)
}

func TestServer_QuotaProvider_Initialized(t *testing.T) {
	srv, err := NewServer(nil)
	assert.NoError(t, err)
	assert.NotNil(t, srv.quotaProvider)

	// Should be NoOpQuotaProvider by default
	_, ok := srv.quotaProvider.(*NoOpQuotaProvider)
	assert.True(t, ok)
}

func TestServer_UserSessions_Initialized(t *testing.T) {
	srv, err := NewServer(nil)
	assert.NoError(t, err)
	assert.NotNil(t, srv.userSessions)
}

// ========== Server Authentication Tests ==========

func TestServer_Authenticate_NoAuthFunc(t *testing.T) {
	srv, err := NewServer(nil)
	assert.NoError(t, err)

	result := srv.authenticate("user", "pass")
	assert.False(t, result)
}

func TestServer_Authenticate_WithAuthFunc(t *testing.T) {
	srv, err := NewServer(nil)
	assert.NoError(t, err)

	srv.SetAuthFunc(func(username, password string) bool {
		return username == "admin" && password == "secret"
	})

	assert.True(t, srv.authenticate("admin", "secret"))
	assert.False(t, srv.authenticate("admin", "wrong"))
	assert.False(t, srv.authenticate("user", "secret"))
}

// ========== Server Config Methods Tests ==========

func TestServer_GetConfig(t *testing.T) {
	config := &Config{
		Enabled:    true,
		Port:       8888,
		RootPath:   "/test/path",
		AllowGuest: false,
	}

	srv, err := NewServer(config)
	assert.NoError(t, err)

	gotConfig := srv.GetConfig()
	assert.Equal(t, config.Port, gotConfig.Port)
	assert.Equal(t, config.RootPath, gotConfig.RootPath)
}

func TestServer_UpdateConfig(t *testing.T) {
	srv, err := NewServer(nil)
	assert.NoError(t, err)

	newConfig := &Config{
		Enabled:       true,
		Port:          9999,
		RootPath:      "/new/path",
		AllowGuest:    true,
		MaxUploadSize: 1024 * 1024,
	}

	err = srv.UpdateConfig(newConfig)
	assert.NoError(t, err)

	gotConfig := srv.GetConfig()
	assert.Equal(t, 9999, gotConfig.Port)
	assert.Equal(t, "/new/path", gotConfig.RootPath)
	assert.True(t, gotConfig.AllowGuest)
}

// ========== Server Status Tests ==========

func TestServer_GetStatus_Enabled(t *testing.T) {
	config := &Config{Enabled: true, Port: 8081}
	srv, err := NewServer(config)
	assert.NoError(t, err)

	status := srv.GetStatus()

	assert.NotNil(t, status)
	assert.Equal(t, true, status["enabled"])
	assert.Equal(t, 8081, status["port"])
	assert.Equal(t, false, status["running"])
}

func TestServer_GetStatus_Disabled(t *testing.T) {
	config := &Config{Enabled: false, Port: 8081}
	srv, err := NewServer(config)
	assert.NoError(t, err)

	status := srv.GetStatus()

	assert.Equal(t, false, status["enabled"])
}

// ========== Server GetUserHome Tests ==========

func TestServer_SetGetUserHome(t *testing.T) {
	srv, err := NewServer(nil)
	assert.NoError(t, err)

	srv.SetGetUserHome(func(username string) string {
		return "/home/" + username
	})

	assert.NotNil(t, srv.getUserHome)
	assert.Equal(t, "/home/testuser", srv.getUserHome("testuser"))
}

// ========== Lock Tests ==========

func TestLockManager_NewLockManager(t *testing.T) {
	lm := NewLockManager()
	assert.NotNil(t, lm)
	assert.NotNil(t, lm.locks)
}

func TestLockManager_CreateLock(t *testing.T) {
	lm := NewLockManager()

	lock, err := lm.CreateLock("/path/to/resource", "user1", 0, "exclusive", 3600)
	assert.NoError(t, err)
	assert.NotNil(t, lock)
	assert.Equal(t, "/path/to/resource", lock.Path)
	assert.Equal(t, "user1", lock.Owner)
	assert.Equal(t, "exclusive", lock.Scope)
}

func TestLockManager_GetLockByPath(t *testing.T) {
	lm := NewLockManager()

	lock, _ := lm.CreateLock("/path/to/resource", "user1", 0, "exclusive", 3600)

	found, exists := lm.GetLockByPath("/path/to/resource")
	assert.True(t, exists)
	assert.NotNil(t, found)
	assert.Equal(t, lock.Token, found.Token)

	_, exists = lm.GetLockByPath("/nonexistent")
	assert.False(t, exists)
}

func TestLockManager_RemoveLock(t *testing.T) {
	lm := NewLockManager()

	lock, _ := lm.CreateLock("/path/to/resource", "user1", 0, "exclusive", 3600)

	err := lm.RemoveLock(lock.Token)
	assert.NoError(t, err)

	_, exists := lm.GetLockByPath("/path/to/resource")
	assert.False(t, exists)
}

func TestLockManager_RemoveLock_NotFound(t *testing.T) {
	lm := NewLockManager()

	err := lm.RemoveLock("nonexistent-token")
	assert.Error(t, err)
}

func TestLockManager_IsLocked(t *testing.T) {
	lm := NewLockManager()

	assert.False(t, lm.IsLocked("/path/to/resource"))

	_, err := lm.CreateLock("/path/to/resource", "user1", 0, "exclusive", 3600)
	assert.NoError(t, err)
	assert.True(t, lm.IsLocked("/path/to/resource"))
}

func TestLockManager_ValidateToken(t *testing.T) {
	lm := NewLockManager()

	lock, _ := lm.CreateLock("/path/to/resource", "user1", 0, "exclusive", 3600)

	assert.True(t, lm.ValidateToken("/path/to/resource", lock.Token))
	assert.False(t, lm.ValidateToken("/wrong/path", lock.Token))
	assert.False(t, lm.ValidateToken("/path/to/resource", "wrong-token"))
}

// ========== Lock Structure Tests ==========

func TestLock_Fields(t *testing.T) {
	lm := NewLockManager()
	lock, _ := lm.CreateLock("/path/to/resource", "user1", 0, "exclusive", 3600)

	assert.NotEmpty(t, lock.Token)
	assert.Equal(t, "/path/to/resource", lock.Path)
	assert.Equal(t, "user1", lock.Owner)
	assert.Equal(t, 0, lock.Depth)
	assert.Equal(t, "exclusive", lock.Scope)
}

// ========== MockQuotaProvider Additional Tests ==========

func TestMockQuotaProvider_CheckQuota(t *testing.T) {
	provider := &MockQuotaProvider{Available: 1024 * 1024}

	available, err := provider.CheckQuota("testuser")
	assert.NoError(t, err)
	assert.Equal(t, int64(1024*1024), available)
}

func TestMockQuotaProvider_ConsumeAndRelease(t *testing.T) {
	provider := &MockQuotaProvider{Available: 1024, Used: 0}

	// Consume
	err := provider.ConsumeQuota("testuser", 512)
	assert.NoError(t, err)
	assert.Equal(t, int64(512), provider.Used)
	assert.Equal(t, int64(512), provider.Available)

	// Release
	err = provider.ReleaseQuota("testuser", 256)
	assert.NoError(t, err)
	assert.Equal(t, int64(256), provider.Used)
	assert.Equal(t, int64(768), provider.Available)
}

func TestMockQuotaProvider_GetUsage(t *testing.T) {
	provider := &MockQuotaProvider{Available: 1024, Used: 512}

	used, total, err := provider.GetUsage("testuser")
	assert.NoError(t, err)
	assert.Equal(t, int64(512), used)
	assert.Equal(t, int64(1536), total) // used + available
}

// ========== Concurrent Access Tests ==========

func TestLockManager_ConcurrentAccess(t *testing.T) {
	lm := NewLockManager()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			path := "/path/" + string(rune('a'+id))
			_, _ = lm.CreateLock(path, "user", 0, "exclusive", 3600)
			lm.IsLocked(path)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestServer_ConcurrentConfigAccess(t *testing.T) {
	srv, err := NewServer(nil)
	assert.NoError(t, err)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			srv.GetConfig()
			srv.GetStatus()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}