package filelockmgr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireLock(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	t.Run("获取独占锁", func(t *testing.T) {
		req := LockRequest{
			FilePath:     "/test/file.txt",
			LockType:     LockTypeExclusive,
			LockedBy:     "user1",
			LockedByName: "用户1",
			Reason:       "测试",
			Duration:     3600,
		}

		lock, err := manager.AcquireLock(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "/test/file.txt", lock.FilePath)
		assert.Equal(t, LockTypeExclusive, lock.LockType)
		assert.Equal(t, "user1", lock.LockedBy)
	})

	t.Run("获取共享锁", func(t *testing.T) {
		req := LockRequest{
			FilePath:     "/test/file2.txt",
			LockType:     LockTypeShared,
			LockedBy:     "user2",
			LockedByName: "用户2",
			Duration:     3600,
		}

		lock, err := manager.AcquireLock(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, LockTypeShared, lock.LockType)
	})

	t.Run("独占锁冲突", func(t *testing.T) {
		req1 := LockRequest{
			FilePath:     "/test/file3.txt",
			LockType:     LockTypeExclusive,
			LockedBy:     "user3",
			LockedByName: "用户3",
			Duration:     3600,
		}
		_, err := manager.AcquireLock(ctx, req1)
		require.NoError(t, err)

		req2 := LockRequest{
			FilePath:     "/test/file3.txt",
			LockType:     LockTypeExclusive,
			LockedBy:     "user4",
			LockedByName: "用户4",
			Duration:     3600,
		}
		_, err = manager.AcquireLock(ctx, req2)
		assert.ErrorIs(t, err, ErrLockConflict)
	})

	t.Run("多个共享锁", func(t *testing.T) {
		req1 := LockRequest{
			FilePath:     "/test/file4.txt",
			LockType:     LockTypeShared,
			LockedBy:     "user5",
			LockedByName: "用户5",
			Duration:     3600,
		}
		_, err := manager.AcquireLock(ctx, req1)
		require.NoError(t, err)

		req2 := LockRequest{
			FilePath:     "/test/file4.txt",
			LockType:     LockTypeShared,
			LockedBy:     "user6",
			LockedByName: "用户6",
			Duration:     3600,
		}
		lock, err := manager.AcquireLock(ctx, req2)
		assert.NoError(t, err)
		assert.NotNil(t, lock)
	})
}

func TestReleaseLock(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	req := LockRequest{
		FilePath:     "/test/file.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}

	lock, err := manager.AcquireLock(ctx, req)
	require.NoError(t, err)

	err = manager.ReleaseLock(ctx, lock.ID)
	assert.NoError(t, err)

	// 释放后再次释放应报错
	err = manager.ReleaseLock(ctx, lock.ID)
	assert.ErrorIs(t, err, ErrLockNotFound)
}

func TestUpgradeLock(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	t.Run("共享锁升级为独占锁", func(t *testing.T) {
		req := LockRequest{
			FilePath:     "/test/file.txt",
			LockType:     LockTypeShared,
			LockedBy:     "user1",
			LockedByName: "用户1",
			Duration:     3600,
		}

		lock, err := manager.AcquireLock(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, LockTypeShared, lock.LockType)

		err = manager.UpgradeLock(ctx, lock.ID)
		assert.NoError(t, err)

		// 验证锁类型已更新
		locks := manager.GetLocksByFile(ctx, "/test/file.txt")
		require.Len(t, locks, 1)
		assert.Equal(t, LockTypeExclusive, locks[0].LockType)
	})

	t.Run("独占锁不能升级", func(t *testing.T) {
		req := LockRequest{
			FilePath:     "/test/file2.txt",
			LockType:     LockTypeExclusive,
			LockedBy:     "user2",
			LockedByName: "用户2",
			Duration:     3600,
		}

		lock, err := manager.AcquireLock(ctx, req)
		require.NoError(t, err)

		err = manager.UpgradeLock(ctx, lock.ID)
		assert.ErrorIs(t, err, ErrLockNotShared)
	})
}

func TestRefreshLock(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	req := LockRequest{
		FilePath:     "/test/file.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     10,
	}

	lock, err := manager.AcquireLock(ctx, req)
	require.NoError(t, err)

	err = manager.RefreshLock(ctx, lock.ID, 30*time.Minute)
	assert.NoError(t, err)
}

func TestGetLocksByUser(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	req1 := LockRequest{
		FilePath:     "/test/file1.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}
	_, err := manager.AcquireLock(ctx, req1)
	require.NoError(t, err)

	req2 := LockRequest{
		FilePath:     "/test/file2.txt",
		LockType:     LockTypeShared,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}
	_, err = manager.AcquireLock(ctx, req2)
	require.NoError(t, err)

	locks := manager.GetLocksByUser(ctx, "user1")
	assert.Len(t, locks, 2)
}

func TestDetectConflict(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	req := LockRequest{
		FilePath:     "/test/file.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}
	_, err := manager.AcquireLock(ctx, req)
	require.NoError(t, err)

	conflict, err := manager.DetectConflict(ctx, "/test/file.txt", "user2")
	assert.NoError(t, err)
	assert.NotNil(t, conflict)
	assert.Equal(t, "exclusive_conflict", conflict.ConflictType)
}

func TestForceRelease(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	req := LockRequest{
		FilePath:     "/test/file.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}
	lock, err := manager.AcquireLock(ctx, req)
	require.NoError(t, err)

	err = manager.ForceRelease(ctx, lock.ID, "admin1")
	assert.NoError(t, err)

	// 验证锁已释放
	locks := manager.GetLocksByFile(ctx, "/test/file.txt")
	assert.Empty(t, locks)
}

func TestGetStats(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 创建一些锁
	req1 := LockRequest{
		FilePath:     "/test/file1.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}
	_, err := manager.AcquireLock(ctx, req1)
	require.NoError(t, err)

	req2 := LockRequest{
		FilePath:     "/test/file2.txt",
		LockType:     LockTypeShared,
		LockedBy:     "user2",
		LockedByName: "用户2",
		Duration:     3600,
	}
	_, err = manager.AcquireLock(ctx, req2)
	require.NoError(t, err)

	stats := manager.GetStats(ctx)
	assert.Equal(t, 2, stats.TotalLocks)
	assert.Equal(t, 1, stats.ExclusiveLocks)
	assert.Equal(t, 1, stats.SharedLocks)
}

func TestCleanupExpired(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 创建一个已过期的锁
	req := LockRequest{
		FilePath:     "/test/file.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     1,
	}
	_, err := manager.AcquireLock(ctx, req)
	require.NoError(t, err)

	// 等待锁过期
	time.Sleep(2 * time.Second)

	count := manager.CleanupExpired(ctx)
	assert.Equal(t, 1, count)
}

func TestMaxLocksPerUser(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 设置最大锁数量为2
	manager.policy.MaxLocksPerUser = 2

	req1 := LockRequest{
		FilePath:     "/test/file1.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}
	_, err := manager.AcquireLock(ctx, req1)
	require.NoError(t, err)

	req2 := LockRequest{
		FilePath:     "/test/file2.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}
	_, err = manager.AcquireLock(ctx, req2)
	require.NoError(t, err)

	req3 := LockRequest{
		FilePath:     "/test/file3.txt",
		LockType:     LockTypeExclusive,
		LockedBy:     "user1",
		LockedByName: "用户1",
		Duration:     3600,
	}
	_, err = manager.AcquireLock(ctx, req3)
	assert.ErrorIs(t, err, ErrMaxLocksExceeded)
}
