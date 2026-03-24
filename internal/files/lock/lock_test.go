package lock

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func init() {
	// 设置测试模式
	zap.ReplaceGlobals(zap.NewNop())
}

// 测试配置
func testConfig() FileLockConfig {
	return FileLockConfig{
		DefaultTimeout:          5 * time.Minute,
		MaxTimeout:              1 * time.Hour,
		CleanupInterval:         1 * time.Second,
		MaxLocksPerFile:         10,
		MaxTotalLocks:           1000,
		EnableAutoRenewal:       false, // 测试时禁用
		AutoRenewalInterval:     1 * time.Minute,
		EnableAudit:             true,
		EnableWaitQueue:         true,
		MaxWaitQueueSize:        10,
		DefaultConflictStrategy: ConflictStrategyReject,
		EnablePreemption:        true,
		PreemptionTimeout:       5 * time.Minute,
	}
}

// ========== FileLock 模型测试 ==========

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

func TestFileLock_IsOwnedBy(t *testing.T) {
	lock := &FileLock{
		Owner:     "user1",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	assert.True(t, lock.IsOwnedBy("user1"))
	assert.False(t, lock.IsOwnedBy("user2"))
}

func TestFileLock_IsOwnedByClient(t *testing.T) {
	lock := &FileLock{
		Owner:     "user1",
		ClientID:  "client1",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	assert.True(t, lock.IsOwnedByClient("user1", "client1"))
	assert.False(t, lock.IsOwnedByClient("user1", "client2"))
	assert.False(t, lock.IsOwnedByClient("user2", "client1"))
}

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
	assert.Equal(t, int64(2), lock.Version) // 版本应该增加
}

func TestFileLock_Upgrade(t *testing.T) {
	t.Run("upgrade shared to exclusive", func(t *testing.T) {
		lock := &FileLock{
			LockType:     LockTypeShared,
			SharedOwners: nil, // 没有其他共享者
			ExpiresAt:    time.Now().Add(1 * time.Hour),
		}

		err := lock.Upgrade()
		assert.NoError(t, err)
		assert.Equal(t, LockTypeExclusive, lock.LockType)
	})

	t.Run("upgrade fails with other shared owners", func(t *testing.T) {
		lock := &FileLock{
			LockType: LockTypeShared,
			SharedOwners: []*SharedOwner{
				{Owner: "user2"},
			},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		err := lock.Upgrade()
		assert.ErrorIs(t, err, ErrLockUpgradeFailed)
		assert.Equal(t, LockTypeShared, lock.LockType)
	})

	t.Run("upgrade exclusive lock is no-op", func(t *testing.T) {
		lock := &FileLock{
			LockType:  LockTypeExclusive,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		err := lock.Upgrade()
		assert.NoError(t, err)
		assert.Equal(t, LockTypeExclusive, lock.LockType)
	})
}

func TestFileLock_Downgrade(t *testing.T) {
	t.Run("downgrade exclusive to shared", func(t *testing.T) {
		lock := &FileLock{
			LockType:  LockTypeExclusive,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		lock.Downgrade()
		assert.Equal(t, LockTypeShared, lock.LockType)
	})

	t.Run("downgrade shared lock is no-op", func(t *testing.T) {
		lock := &FileLock{
			LockType:  LockTypeShared,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		lock.Downgrade()
		assert.Equal(t, LockTypeShared, lock.LockType)
	})
}

func TestFileLock_SharedOwners(t *testing.T) {
	lock := &FileLock{
		LockType:     LockTypeShared,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		SharedOwners: make([]*SharedOwner, 0),
	}

	// 添加共享者
	owner1 := &SharedOwner{Owner: "user1", OwnerName: "User 1"}
	lock.AddSharedOwner(owner1)
	assert.Len(t, lock.SharedOwners, 1)

	// 重复添加不会增加
	lock.AddSharedOwner(owner1)
	assert.Len(t, lock.SharedOwners, 1)

	// 添加另一个共享者
	owner2 := &SharedOwner{Owner: "user2", OwnerName: "User 2"}
	lock.AddSharedOwner(owner2)
	assert.Len(t, lock.SharedOwners, 2)

	// 移除共享者
	lock.RemoveSharedOwner("user1")
	assert.Len(t, lock.SharedOwners, 1)
	assert.Equal(t, "user2", lock.SharedOwners[0].Owner)
}

func TestFileLock_ToInfo(t *testing.T) {
	now := time.Now()
	lock := &FileLock{
		ID:           "test-id",
		FilePath:     "/test/file.txt",
		FileName:     "file.txt",
		LockType:     LockTypeExclusive,
		LockMode:     LockModeManual,
		Status:       LockStatusActive,
		Priority:     PriorityNormal,
		Owner:        "user1",
		OwnerName:    "Test User",
		ClientID:     "client1",
		Protocol:     "SMB",
		CreatedAt:    now,
		ExpiresAt:    now.Add(30 * time.Minute),
		LastAccessed: now,
		Version:      1,
		SharedOwners: []*SharedOwner{
			{Owner: "user1", OwnerName: "Test User"},
		},
	}

	info := lock.ToInfo()

	assert.Equal(t, "test-id", info.ID)
	assert.Equal(t, "/test/file.txt", info.FilePath)
	assert.Equal(t, "exclusive", info.LockType)
	assert.Equal(t, "manual", info.LockMode)
	assert.Equal(t, "active", info.Status)
	assert.Equal(t, "normal", info.Priority)
	assert.Equal(t, "user1", info.Owner)
	assert.Equal(t, "Test User", info.OwnerName)
	assert.Equal(t, "client1", info.ClientID)
	assert.Equal(t, "SMB", info.Protocol)
	assert.False(t, info.IsExpired)
	assert.True(t, info.ExpiresIn > 0)
	assert.Equal(t, 1, info.SharedCount)
	assert.Equal(t, int64(1), info.Version)
}

// ========== LockType 和 LockStatus 测试 ==========

func TestLockType_String(t *testing.T) {
	assert.Equal(t, "shared", LockTypeShared.String())
	assert.Equal(t, "exclusive", LockTypeExclusive.String())
}

func TestLockStatus_String(t *testing.T) {
	assert.Equal(t, "active", LockStatusActive.String())
	assert.Equal(t, "expired", LockStatusExpired.String())
	assert.Equal(t, "released", LockStatusReleased.String())
	assert.Equal(t, "pending", LockStatusPending.String())
	assert.Equal(t, "conflict", LockStatusConflict.String())
}

func TestParseLockType(t *testing.T) {
	tests := []struct {
		input    string
		expected LockType
	}{
		{"shared", LockTypeShared},
		{"Shared", LockTypeShared},
		{"r", LockTypeShared},
		{"read", LockTypeShared},
		{"exclusive", LockTypeExclusive},
		{"Exclusive", LockTypeExclusive},
		{"w", LockTypeExclusive},
		{"write", LockTypeExclusive},
		{"unknown", LockTypeShared}, // 默认共享锁
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseLockType(tt.input))
		})
	}
}

// ========== Manager 测试 ==========

func TestManager_Lock(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	tests := []struct {
		name         string
		req          *LockRequest
		wantErr      error
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
		{
			name: "nil request",
			req:   nil,
			wantErr: ErrInvalidLockType,
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
	assert.Equal(t, ConflictTypeExclusive, conflict.ConflictType)
	assert.Equal(t, "user1", conflict.ExistingLock.Owner)
}

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

	// 获取锁详情
	info, err := manager.GetLockByPath("/test/shared.txt")
	require.NoError(t, err)
	assert.Equal(t, 3, info.SharedCount)
}

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
	err = manager.ForceUnlock(lock.ID, "admin override")
	assert.NoError(t, err)

	// 锁已释放
	assert.False(t, manager.IsLocked("/test/file.txt"))
}

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

func TestManager_UpgradeDowngradeLock(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	// 获取共享锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeShared,
		Owner:    "user1",
	})
	require.NoError(t, err)

	// 升级为独占锁
	err = manager.UpgradeLock(lock.ID, "user1")
	assert.NoError(t, err)
	assert.Equal(t, LockTypeExclusive, lock.LockType)

	// 降级回共享锁
	err = manager.DowngradeLock(lock.ID, "user1")
	assert.NoError(t, err)
	assert.Equal(t, LockTypeShared, lock.LockType)
}

func TestManager_LockExpiration(t *testing.T) {
	config := testConfig()
	config.CleanupInterval = 100 * time.Millisecond // 快速清理
	config.DefaultTimeout = 200 * time.Millisecond  // 短超时

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

	// 按不存在的用户过滤
	filter = &LockFilter{Owner: "nonexistent"}
	locks = manager.ListLocks(filter)
	assert.Len(t, locks, 0)
}

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

// ========== 协议适配器测试 ==========

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

func TestWebDAVLockAdapter(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	adapter := NewWebDAVLockAdapter(manager)

	// 测试独占锁
	err := adapter.Lock("/test/webdav/file.txt", "user1", true)
	assert.NoError(t, err)

	// 验证锁定状态
	assert.True(t, adapter.IsLocked("/test/webdav/file.txt"))

	// 释放锁
	err = adapter.Unlock("/test/webdav/file.txt", "user1")
	assert.NoError(t, err)
}

func TestDriveLockAdapter(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	adapter := NewDriveLockAdapter(manager)

	// 测试带通知的锁
	info, conflict, err := adapter.LockWithNotification("/test/drive/file.txt", "user1", "User 1", "client1", true, 300, false)
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Nil(t, conflict)

	// 验证锁定状态
	assert.True(t, adapter.IsLocked("/test/drive/file.txt"))

	// 获取锁详情
	info, err = adapter.GetLockInfo("/test/drive/file.txt")
	assert.NoError(t, err)
	assert.Equal(t, "user1", info.Owner)
	assert.Equal(t, "exclusive", info.LockType)

	// 释放锁
	err = adapter.Unlock("/test/drive/file.txt", "user1")
	assert.NoError(t, err)
}

// ========== 协作锁测试 ==========

func TestCollaborativeLockManager(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	collab := NewCollaborativeLockManager(manager)

	// 用户1加入共享锁
	info1, err := collab.JoinSharedLock("/test/collab/file.txt", "user1", "User 1", "client1")
	assert.NoError(t, err)
	assert.NotNil(t, info1)

	// 用户2加入共享锁
	info2, err := collab.JoinSharedLock("/test/collab/file.txt", "user2", "User 2", "client2")
	assert.NoError(t, err)
	assert.NotNil(t, info2)

	// 获取协作者列表
	collaborators, err := collab.GetCollaborators("/test/collab/file.txt")
	assert.NoError(t, err)
	assert.Len(t, collaborators, 2)

	// 用户1请求编辑锁（升级为独占）
	err = collab.RequestEditLock(info1.ID, "user1")
	// 应该失败，因为有其他共享者
	assert.ErrorIs(t, err, ErrLockUpgradeFailed)
}

// ========== 并发测试 ==========

func TestManager_ConcurrentLocks(t *testing.T) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	const numGoroutines = 10
	const numLocksPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numLocksPerGoroutine; j++ {
				filePath := "/test/concurrent/" + string(rune('A'+id)) + ".txt"
				lock, _, err := manager.Lock(&LockRequest{
					FilePath: filePath,
					LockType: LockTypeExclusive,
					Owner:    string(rune('A' + id)),
				})
				if err == nil && lock != nil {
					manager.Unlock(lock.ID, string(rune('A'+id)))
				}
			}
		}(i)
	}

	wg.Wait()
}

// ========== 审计存储测试 ==========

func TestLockAuditStorage(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lock-audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := LockAuditStorageConfig{
		LogPath:   tmpDir,
		MaxSize:   10,
		MaxCount:  5,
		MaxAge:    7,
		SignKey:   []byte("test-key"),
		FlushInterval: 100 * time.Millisecond,
	}

	storage, err := NewLockAuditStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	// 记录审计日志
	entry := &LockAuditEntry{
		Event:     AuditEventLockAcquired,
		FilePath:  "/test/file.txt",
		FileName:  "file.txt",
		LockType:  "exclusive",
		Owner:     "user1",
		OwnerName: "User 1",
		ClientID:  "client1",
		Protocol:  "SMB",
	}

	storage.LogLockAudit(entry)

	// 等待刷新
	time.Sleep(200 * time.Millisecond)

	// 查询日志
	result, err := storage.Query(LockAuditQueryOptions{
		FilePath: "/test/file.txt",
		Limit:    10,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, AuditEventLockAcquired, result.Entries[0].Event)
}

func TestLockAuditStorage_VerifySignature(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lock-audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := LockAuditStorageConfig{
		LogPath:   tmpDir,
		SignKey:   []byte("test-key"),
		FlushInterval: 100 * time.Millisecond,
	}

	storage, err := NewLockAuditStorage(config)
	require.NoError(t, err)
	defer storage.Close()

	// 创建并签名条目
	entry := &LockAuditEntry{
		Event:    AuditEventLockReleased,
		FilePath: "/test/file.txt",
		Owner:    "user1",
	}

	storage.LogLockAudit(entry)

	// 验证签名
	assert.True(t, storage.VerifyEntry(entry))

	// 修改条目后验证失败
	entry.Owner = "user2"
	assert.False(t, storage.VerifyEntry(entry))
}

// ========== 完整集成测试 ==========

func TestAuditEnabledManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lock-audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	auditConfig := LockAuditStorageConfig{
		LogPath:       tmpDir,
		FlushInterval: 100 * time.Millisecond,
	}

	manager, err := NewAuditEnabledManager(testConfig(), zap.NewNop(), auditConfig)
	require.NoError(t, err)
	defer manager.Close()

	// 获取锁
	lock, _, err := manager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
		OwnerName: "User 1",
	})
	require.NoError(t, err)

	// 等待审计日志写入
	time.Sleep(200 * time.Millisecond)

	// 查询审计日志
	result, err := manager.QueryAuditLogs(context.Background(), LockAuditQueryOptions{
		Owner: "user1",
		Limit: 10,
	})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, 1)

	// 释放锁
	err = manager.Unlock(lock.ID, "user1")
	assert.NoError(t, err)

	// 获取统计
	stats, err := manager.GetAuditStats(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalEvents, int64(2)) // 至少有获取和释放事件
}

// ========== 基准测试 ==========

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

func BenchmarkManager_ConcurrentLocks(b *testing.B) {
	manager := NewManager(testConfig(), zap.NewNop())
	defer manager.Close()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			filePath := "/test/file" + string(rune(i%10)) + ".txt"
			lock, _, _ := manager.Lock(&LockRequest{
				FilePath: filePath,
				LockType: LockTypeShared,
				Owner:    "user1",
			})
			if lock != nil {
				manager.Unlock(lock.ID, "user1")
			}
			i++
		}
	})
}