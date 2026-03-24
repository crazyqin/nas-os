package lock

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// 测试配置
func testConfig() FileLockConfig {
	return FileLockConfig{
		DefaultTimeout:      5 * time.Minute,
		MaxTimeout:          1 * time.Hour,
		CleanupInterval:     1 * time.Second,
		MaxLocksPerFile:     10,
		EnableAutoRenewal:   false, // 测试时禁用
		AutoRenewalInterval: 1 * time.Minute,
	}
}

// TestFileLock_IsExpired 测试锁过期检查
func TestFileLock_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		lock     *FileLock
		expected bool
	}{
		{
			name: "active lock",
			lock: &FileLock{
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "expired lock",
			lock: &FileLock{
				ExpiresAt: time.Now().Add(-1 * time.Second),
			},
			expected: true,
		},
		{
			name: "lock expiring now",
			lock: &FileLock{
				ExpiresAt: time.Now(),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.lock.IsExpired())
		})
	}
}

// TestFileLock_IsOwnedBy 测试锁持有者检查
func TestFileLock_IsOwnedBy(t *testing.T) {
	lock := &FileLock{
		Owner:     "user1",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	assert.True(t, lock.IsOwnedBy("user1"))
	assert.False(t, lock.IsOwnedBy("user2"))
}

// TestFileLock_Extend 测试锁延期
func TestFileLock_Extend(t *testing.T) {
	lock := &FileLock{
		ExpiresAt:    time.Now().Add(1 * time.Minute),
		LastAccessed: time.Now().Add(-30 * time.Second),
	}

	originalExpiry := lock.ExpiresAt
	originalLastAccessed := lock.LastAccessed
	lock.Extend(5 * time.Minute)

	assert.True(t, lock.ExpiresAt.After(originalExpiry))
	assert.True(t, lock.LastAccessed.After(originalLastAccessed))
}

// TestLockType_String 测试锁类型字符串
func TestLockType_String(t *testing.T) {
	assert.Equal(t, "shared", LockTypeShared.String())
	assert.Equal(t, "exclusive", LockTypeExclusive.String())
}

// TestLockStatus_String 测试锁状态字符串
func TestLockStatus_String(t *testing.T) {
	assert.Equal(t, "active", LockStatusActive.String())
	assert.Equal(t, "expired", LockStatusExpired.String())
	assert.Equal(t, "released", LockStatusReleased.String())
}

// TestManager_Lock 测试获取锁
func TestManager_Lock(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	tests := []struct {
		name      string
		req       *LockRequest
		wantErr   error
		wantConflict bool
	}{
		{
			name: "acquire shared lock",
			req: &LockRequest{
				FilePath: "/test/file1.txt",
				LockType: LockTypeShared,
				Owner:    "user1",
			},
			wantErr: nil,
		},
		{
			name: "acquire exclusive lock",
			req: &LockRequest{
				FilePath: "/test/file2.txt",
				LockType: LockTypeExclusive,
				Owner:    "user1",
			},
			wantErr: nil,
		},
		{
			name: "shared lock on same file by different user",
			req: &LockRequest{
				FilePath: "/test/file1.txt",
				LockType: LockTypeShared,
				Owner:    "user2",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock, conflict, err := manager.Lock(tt.req)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, lock)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, lock)
				assert.Nil(t, conflict)
				assert.NotEmpty(t, lock.ID)
				assert.Equal(t, tt.req.FilePath, lock.FilePath)
				assert.Equal(t, tt.req.Owner, lock.Owner)
			}
		})
	}
}

// TestManager_LockConflict 测试锁冲突
func TestManager_LockConflict(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 用户1获取独占锁
	lock1, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)
	require.NotNil(t, lock1)

	// 用户2尝试获取共享锁（应该冲突）
	lock2, conflict, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeShared,
		Owner:    "user2",
	})

	assert.ErrorIs(t, err, ErrLockConflict)
	assert.Nil(t, lock2)
	assert.NotNil(t, conflict)
	assert.Equal(t, "user1", conflict.ExistingLock.Owner)
}

// TestManager_SharedLocks 测试多个共享锁
func TestManager_SharedLocks(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 多个用户获取共享锁
	for i := 1; i <= 3; i++ {
		lock, _, err := manager.Lock(&LockRequest{
			FilePath: "/test/shared.txt",
			LockType: LockTypeShared,
			Owner:    string(rune('A' + i)),
		})
		assert.NoError(t, err)
		assert.NotNil(t, lock)
	}

	// 验证文件被锁定
	assert.True(t, manager.IsLocked("/test/shared.txt"))
}

// TestManager_Unlock 测试释放锁
func TestManager_Unlock(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 正确的用户释放锁
	err = manager.Unlock(lock.ID, "user1")
	assert.NoError(t, err)

	// 验证锁已释放
	assert.False(t, manager.IsLocked("/test/file.txt"))
}

// TestManager_UnlockWrongOwner 测试错误持有者释放锁
func TestManager_UnlockWrongOwner(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 用户1获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 用户2尝试释放（应该失败）
	err = manager.Unlock(lock.ID, "user2")
	assert.ErrorIs(t, err, ErrNotLockOwner)

	// 锁仍然存在
	assert.True(t, manager.IsLocked("/test/file.txt"))
}

// TestManager_ForceUnlock 测试强制释放锁
func TestManager_ForceUnlock(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 用户1获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 强制释放
	err = manager.ForceUnlock(lock.ID)
	assert.NoError(t, err)

	// 锁已释放
	assert.False(t, manager.IsLocked("/test/file.txt"))
}

// TestManager_ExtendLock 测试延长锁
func TestManager_ExtendLock(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
		Timeout:  60, // 60秒
	})
	require.NoError(t, err)

	originalExpiry := lock.ExpiresAt

	// 延长锁
	err = manager.ExtendLock(lock.ID, "user1", 5*time.Minute)
	assert.NoError(t, err)

	// 验证时间延长
	assert.True(t, lock.ExpiresAt.After(originalExpiry))
}

// TestManager_LockExpiration 测试锁过期
func TestManager_LockExpiration(t *testing.T) {
	config := testConfig()
	config.CleanupInterval = 100 * time.Millisecond // 快速清理
	config.DefaultTimeout = 200 * time.Millisecond   // 短超时

	manager := NewManager(config, zap.NewNop())
	defer manager.Close()

	// 获取锁（短超时）
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
		Timeout:  1, // 1秒
	})
	require.NoError(t, err)

	// 等待过期
	time.Sleep(1200 * time.Millisecond)

	// 检查过期
	assert.True(t, lock.IsExpired())
}

// TestManager_ListLocks 测试列出锁
func TestManager_ListLocks(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 创建多个锁
	for i := 0; i < 3; i++ {
		_, _, err := manager.Lock(&LockRequest{
			FilePath: "/test/file" + string(rune('A'+i)) + ".txt",
			LockType: LockTypeExclusive,
			Owner:    "user1",
		})
		require.NoError(t, err)
	}

	// 列出所有锁
	locks := manager.ListLocks(nil)
	assert.Len(t, locks, 3)

	// 按用户过滤
	filter := &LockFilter{Owner: "user1"}
	locks = manager.ListLocks(filter)
	assert.Len(t, locks, 3)
}

// TestManager_Stats 测试统计
func TestManager_Stats(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 获取初始统计
	stats := manager.Stats()
	assert.Equal(t, int64(0), stats.TotalLocks)

	// 创建锁
	_, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 验证统计更新
	stats = manager.Stats()
	assert.Equal(t, int64(1), stats.TotalLocks)
	assert.Equal(t, int64(1), stats.ActiveLocks)
}

// TestSMBLockAdapter 测试SMB适配器
func TestSMBLockAdapter(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	adapter := NewSMBLockAdapter(manager)

	// 测试独占锁
	err := adapter.Lock("/test/smb/file.txt", "user1", true)
	assert.NoError(t, err)

	// 验证锁定状态
	assert.True(t, adapter.IsLocked("/test/smb/file.txt"))

	// 获取持有者
	owner, err := adapter.GetLockOwner("/test/smb/file.txt")
	assert.NoError(t, err)
	assert.Equal(t, "user1", owner)

	// 释放锁
	err = adapter.Unlock("/test/smb/file.txt", "user1")
	assert.NoError(t, err)

	// 验证已释放
	assert.False(t, adapter.IsLocked("/test/smb/file.txt"))
}

// TestNFSLockAdapter 测试NFS适配器
func TestNFSLockAdapter(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	adapter := NewNFSLockAdapter(manager)

	// 测试共享锁
	err := adapter.Lock("/test/nfs/file.txt", "user1", false)
	assert.NoError(t, err)

	// 验证锁定状态
	assert.True(t, adapter.IsLocked("/test/nfs/file.txt"))

	// 释放锁
	err = adapter.Unlock("/test/nfs/file.txt", "user1")
	assert.NoError(t, err)
}

// API测试

func setupTestAPI(t *testing.T) (*gin.Engine, *Manager, *Handlers) {
	manager := NewManager(testConfig(), zap.NewNop())
	handlers := NewHandlers(manager, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	return router, manager, handlers
}

// TestAPI_AcquireLock 测试API获取锁
func TestAPI_AcquireLock(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	body := `{
		"filePath": "/test/file.txt",
		"lockType": "exclusive",
		"owner": "user1",
		"ownerName": "Test User",
		"timeout": 300
	}`

	req, _ := http.NewRequest("POST", "/api/v1/locks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "lock acquired")
}

// TestAPI_ReleaseLock 测试API释放锁
func TestAPI_ReleaseLock(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	// 先获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 通过API释放
	req, _ := http.NewRequest("DELETE", "/api/v1/locks/"+lock.ID+"?owner=user1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "lock released")
}

// TestAPI_GetLock 测试API获取锁详情
func TestAPI_GetLock(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	// 获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 查询锁
	req, _ := http.NewRequest("GET", "/api/v1/locks/"+lock.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), lock.ID)
}

// TestAPI_ListLocks 测试API列出锁
func TestAPI_ListLocks(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	// 创建几个锁
	for i := 0; i < 3; i++ {
		_, _, err := manager.Lock(&LockRequest{
			FilePath: "/test/file" + string(rune('A'+i)) + ".txt",
			LockType: LockTypeExclusive,
			Owner:    "user1",
		})
		require.NoError(t, err)
	}

	// 列出锁
	req, _ := http.NewRequest("GET", "/api/v1/locks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total")
}

// TestAPI_CheckLock 测试API检查锁状态
func TestAPI_CheckLock(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	// 获取锁
	_, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 检查锁定状态
	req, _ := http.NewRequest("GET", "/api/v1/locks/check/test/file.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "isLocked")
	assert.Contains(t, w.Body.String(), "true")
}

// TestAPI_GetStats 测试API获取统计
func TestAPI_GetStats(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	// 创建锁
	_, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 获取统计
	req, _ := http.NewRequest("GET", "/api/v1/locks/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "totalLocks")
}

// TestAPI_LockConflict 测试API锁冲突响应
func TestAPI_LockConflict(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	// 用户1获取独占锁
	body1 := `{
		"filePath": "/test/file.txt",
		"lockType": "exclusive",
		"owner": "user1"
	}`
	req1, _ := http.NewRequest("POST", "/api/v1/locks", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	// 用户2尝试获取锁（应该冲突）
	body2 := `{
		"filePath": "/test/file.txt",
		"lockType": "shared",
		"owner": "user2"
	}`
	req2, _ := http.NewRequest("POST", "/api/v1/locks", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusConflict, w2.Code)
	assert.Contains(t, w2.Body.String(), "409")
	assert.Contains(t, w2.Body.String(), "user1")
}

// TestAPI_ForceReleaseLock 测试API强制释放锁
func TestAPI_ForceReleaseLock(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	// 获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 强制释放
	req, _ := http.NewRequest("DELETE", "/api/v1/locks/"+lock.ID+"/force", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "force released")
}

// TestAPI_ExtendLock 测试API延长锁
func TestAPI_ExtendLock(t *testing.T) {
	router, manager, _ := setupTestAPI(t)
	defer manager.Close()

	// 获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 延长锁
	body := `{"duration": 300}`
	req, _ := http.NewRequest("PUT", "/api/v1/locks/"+lock.ID+"/extend?owner=user1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "extended")
}

// 基准测试

func BenchmarkManager_Lock(b *testing.B) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Lock(&LockRequest{
			FilePath: "/test/file.txt",
			LockType: LockTypeShared,
			Owner:    "user1",
		})
	}
}

func BenchmarkManager_Unlock(b *testing.B) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lock, _, _ := manager.Lock(&LockRequest{
			FilePath: "/test/file.txt",
			LockType: LockTypeExclusive,
			Owner:    "user1",
		})
		manager.Unlock(lock.ID, "user1")
	}
}