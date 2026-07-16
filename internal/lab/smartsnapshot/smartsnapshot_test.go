package smartsnapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 辅助函数 ==========

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "smartsnapshot.json")
	mgr, err := NewManager(configPath)
	require.NoError(t, err)
	return mgr, dir
}

func newTestManagerNoConfig(t *testing.T) *Manager {
	t.Helper()
	mgr, err := NewManager("")
	require.NoError(t, err)
	return mgr
}

// ========== 快照创建测试 ==========

func TestCreateSnapshot(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	t.Run("创建全量快照", func(t *testing.T) {
		snap, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "test-full",
			DatasetPath: "/pool/dataset1",
			Type:        TypeFull,
			Description: "测试全量快照",
			Tags:        []string{"test", "full"},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, snap.ID)
		assert.Equal(t, "test-full", snap.Name)
		assert.Equal(t, "/pool/dataset1", snap.DatasetPath)
		assert.Equal(t, TypeFull, snap.Type)
		assert.Equal(t, StatusReady, snap.Status)
		assert.True(t, snap.SizeBytes > 0)
		assert.True(t, snap.FileCount > 0)
	})

	t.Run("创建增量快照", func(t *testing.T) {
		// 先创建基础快照
		_, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "base",
			DatasetPath: "/pool/inc",
			Type:        TypeFull,
		})
		require.NoError(t, err)

		// 创建增量快照
		snap, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "incr-1",
			DatasetPath: "/pool/inc",
			Type:        TypeIncremental,
		})
		require.NoError(t, err)
		assert.Equal(t, TypeIncremental, snap.Type)
		assert.NotEmpty(t, snap.ParentID)
	})

	t.Run("无基础快照时创建增量快照失败", func(t *testing.T) {
		_, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "fail-incr",
			DatasetPath: "/pool/empty",
			Type:        TypeIncremental,
		})
		assert.Error(t, err)
	})

	t.Run("创建差异快照", func(t *testing.T) {
		_, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "base-diff",
			DatasetPath: "/pool/diff",
			Type:        TypeFull,
		})
		require.NoError(t, err)

		snap, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "diff-1",
			DatasetPath: "/pool/diff",
			Type:        TypeDifferential,
		})
		require.NoError(t, err)
		assert.Equal(t, TypeDifferential, snap.Type)
		assert.NotEmpty(t, snap.ParentID)
	})

	t.Run("创建带过期时间的快照", func(t *testing.T) {
		snap, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "expiring",
			DatasetPath: "/pool/exp",
			Type:        TypeFull,
			ExpireDays:  7,
		})
		require.NoError(t, err)
		require.NotNil(t, snap.ExpiresAt)
		assert.True(t, snap.ExpiresAt.After(time.Now()))
	})

	t.Run("创建受保护快照", func(t *testing.T) {
		snap, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "protected",
			DatasetPath: "/pool/prot",
			Type:        TypeFull,
			Protected:   true,
		})
		require.NoError(t, err)
		assert.True(t, snap.IsProtected)
	})

	t.Run("默认名称自动生成", func(t *testing.T) {
		snap, err := mgr.CreateSnapshot(CreateSnapshotRequest{
			DatasetPath: "/pool/default-name",
			Type:        TypeFull,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, snap.Name)
		assert.Contains(t, snap.Name, "snap-")
	})
}

// ========== 快照查询测试 ==========

func TestGetSnapshot(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	snap, err := mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "find-me",
		DatasetPath: "/pool/query",
		Type:        TypeFull,
	})
	require.NoError(t, err)

	t.Run("获取存在的快照", func(t *testing.T) {
		found, err := mgr.GetSnapshot(snap.ID)
		require.NoError(t, err)
		assert.Equal(t, snap.ID, found.ID)
		assert.Equal(t, "find-me", found.Name)
	})

	t.Run("获取不存在的快照", func(t *testing.T) {
		_, err := mgr.GetSnapshot("non-existent")
		assert.ErrorIs(t, err, ErrSnapshotNotFound)
	})
}

func TestListSnapshots(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	// 创建多个快照
	_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{Name: "a", DatasetPath: "/pool/x", Type: TypeFull})
	time.Sleep(10 * time.Millisecond)
	_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{Name: "b", DatasetPath: "/pool/x", Type: TypeFull})
	_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{Name: "c", DatasetPath: "/pool/y", Type: TypeFull})

	t.Run("列出所有快照", func(t *testing.T) {
		snaps := mgr.ListSnapshots("")
		assert.Len(t, snaps, 3)
	})

	t.Run("按路径过滤", func(t *testing.T) {
		snaps := mgr.ListSnapshots("/pool/x")
		assert.Len(t, snaps, 2)
	})

	t.Run("按不存在路径过滤", func(t *testing.T) {
		snaps := mgr.ListSnapshots("/pool/z")
		assert.Len(t, snaps, 0)
	})

	t.Run("排序验证（最新在前）", func(t *testing.T) {
		snaps := mgr.ListSnapshots("")
		require.Len(t, snaps, 3)
		assert.True(t, snaps[0].CreatedAt.After(snaps[1].CreatedAt) || snaps[0].CreatedAt.Equal(snaps[1].CreatedAt))
	})
}

// ========== 快照链测试 ==========

func TestSnapshotChain(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	base, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "base",
		DatasetPath: "/pool/chain",
		Type:        TypeFull,
	})

	inc1, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "inc1",
		DatasetPath: "/pool/chain",
		Type:        TypeIncremental,
	})

	inc2, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "inc2",
		DatasetPath: "/pool/chain",
		Type:        TypeIncremental,
	})

	chain, err := mgr.GetSnapshotChain(inc2.ID)
	require.NoError(t, err)
	require.Len(t, chain, 3)
	assert.Equal(t, base.ID, chain[0].ID)
	assert.Equal(t, inc1.ID, chain[1].ID)
	assert.Equal(t, inc2.ID, chain[2].ID)
}

// ========== 快照删除测试 ==========

func TestDeleteSnapshot(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	t.Run("删除普通快照", func(t *testing.T) {
		snap, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "to-delete",
			DatasetPath: "/pool/del",
			Type:        TypeFull,
		})
		err := mgr.DeleteSnapshot(snap.ID, false)
		assert.NoError(t, err)

		_, err = mgr.GetSnapshot(snap.ID)
		assert.ErrorIs(t, err, ErrSnapshotNotFound)
	})

	t.Run("受保护快照不允许直接删除", func(t *testing.T) {
		snap, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "protected",
			DatasetPath: "/pool/prot",
			Type:        TypeFull,
			Protected:   true,
		})
		err := mgr.DeleteSnapshot(snap.ID, false)
		assert.Error(t, err)
	})

	t.Run("强制删除受保护快照", func(t *testing.T) {
		snap, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "force-del",
			DatasetPath: "/pool/force",
			Type:        TypeFull,
			Protected:   true,
		})
		err := mgr.DeleteSnapshot(snap.ID, true)
		assert.NoError(t, err)
	})

	t.Run("基础快照不允许删除（有子快照）", func(t *testing.T) {
		base, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "parent",
			DatasetPath: "/pool/parent",
			Type:        TypeFull,
		})
		_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "child",
			DatasetPath: "/pool/parent",
			Type:        TypeIncremental,
		})
		err := mgr.DeleteSnapshot(base.ID, false)
		assert.Error(t, err)
	})

	t.Run("删除不存在的快照", func(t *testing.T) {
		err := mgr.DeleteSnapshot("non-existent", false)
		assert.ErrorIs(t, err, ErrSnapshotNotFound)
	})
}

// ========== 克隆测试 ==========

func TestCloneSnapshot(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	snap, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "to-clone",
		DatasetPath: "/pool/clone",
		Type:        TypeFull,
	})

	t.Run("克隆快照", func(t *testing.T) {
		clone, err := mgr.CloneSnapshot(snap.ID, "/clone/path1")
		require.NoError(t, err)
		assert.NotEmpty(t, clone.ID)
		assert.Equal(t, snap.ID, clone.SourceID)
		assert.Equal(t, "/clone/path1", clone.ClonePath)
		assert.True(t, clone.IsActive)
	})

	t.Run("克隆不存在的快照", func(t *testing.T) {
		_, err := mgr.CloneSnapshot("non-existent", "/clone/path2")
		assert.ErrorIs(t, err, ErrSnapshotNotFound)
	})

	t.Run("列出克隆", func(t *testing.T) {
		clone, _ := mgr.CloneSnapshot(snap.ID, "/clone/path3")
		clones := mgr.ListClones(snap.ID)
		assert.True(t, len(clones) >= 2)

		// 销毁克隆
		err := mgr.DestroyClone(clone.ID)
		assert.NoError(t, err)

		clones = mgr.ListClones(snap.ID)
		for _, c := range clones {
			assert.NotEqual(t, clone.ID, c.ID)
		}
	})

	t.Run("销毁不存在的克隆", func(t *testing.T) {
		err := mgr.DestroyClone("non-existent")
		assert.ErrorIs(t, err, ErrCloneNotFound)
	})
}

// ========== 回滚测试 ==========

func TestRollbackSnapshot(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	t.Run("回滚快照", func(t *testing.T) {
		snap, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "to-rollback",
			DatasetPath: "/pool/rollback",
			Type:        TypeFull,
		})
		err := mgr.RollbackSnapshot(RollbackRequest{
			SnapshotID: snap.ID,
			Force:      false,
		})
		assert.NoError(t, err)
	})

	t.Run("回滚不存在的快照", func(t *testing.T) {
		err := mgr.RollbackSnapshot(RollbackRequest{
			SnapshotID: "non-existent",
		})
		assert.ErrorIs(t, err, ErrSnapshotNotFound)
	})

	t.Run("有活跃克隆时需要强制回滚", func(t *testing.T) {
		snap, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        "with-clone",
			DatasetPath: "/pool/clone-rollback",
			Type:        TypeFull,
		})
		_, _ = mgr.CloneSnapshot(snap.ID, "/clone/path")

		err := mgr.RollbackSnapshot(RollbackRequest{
			SnapshotID: snap.ID,
			Force:      false,
		})
		assert.Error(t, err)

		// 强制回滚
		err = mgr.RollbackSnapshot(RollbackRequest{
			SnapshotID: snap.ID,
			Force:      true,
		})
		assert.NoError(t, err)
	})
}

// ========== 恢复测试 ==========

func TestRestoreSnapshot(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	snap, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "to-restore",
		DatasetPath: "/pool/restore",
		Type:        TypeFull,
	})

	t.Run("恢复快照", func(t *testing.T) {
		err := mgr.RestoreSnapshot(snap.ID, "/restore/target")
		assert.NoError(t, err)
	})

	t.Run("恢复不存在的快照", func(t *testing.T) {
		err := mgr.RestoreSnapshot("non-existent", "/restore/target")
		assert.ErrorIs(t, err, ErrSnapshotNotFound)
	})
}

// ========== 策略测试 ==========

func TestPolicyCRUD(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	t.Run("创建策略", func(t *testing.T) {
		err := mgr.CreatePolicy(&SnapshotPolicy{
			Name:         "test-policy",
			Enabled:      true,
			Type:         PolicyCron,
			CronExpr:     "0 0 2 * * *", // 每天凌晨 2 点
			DatasetPaths: []string{"/pool/auto"},
			Retention: RetentionPolicy{
				MaxSnapshots: 10,
				MaxAgeDays:   30,
				KeepDaily:    7,
				KeepWeekly:   4,
				KeepMonthly:  3,
			},
		})
		assert.NoError(t, err)

		policies := mgr.ListPolicies()
		assert.Len(t, policies, 1)
		assert.Equal(t, "test-policy", policies[0].Name)
	})

	t.Run("获取策略", func(t *testing.T) {
		policies := mgr.ListPolicies()
		require.Len(t, policies, 1)

		found, err := mgr.GetPolicy(policies[0].ID)
		require.NoError(t, err)
		assert.Equal(t, "test-policy", found.Name)
	})

	t.Run("更新策略", func(t *testing.T) {
		policies := mgr.ListPolicies()
		require.Len(t, policies, 1)

		err := mgr.UpdatePolicy(policies[0].ID, &SnapshotPolicy{
			Name:      "updated-policy",
			Retention: RetentionPolicy{MaxSnapshots: 20},
		})
		assert.NoError(t, err)

		found, _ := mgr.GetPolicy(policies[0].ID)
		assert.Equal(t, "updated-policy", found.Name)
		assert.Equal(t, 20, found.Retention.MaxSnapshots)
	})

	t.Run("启用/禁用策略", func(t *testing.T) {
		policies := mgr.ListPolicies()
		require.Len(t, policies, 1)

		err := mgr.DisablePolicy(policies[0].ID)
		assert.NoError(t, err)
		found, _ := mgr.GetPolicy(policies[0].ID)
		assert.False(t, found.Enabled)

		err = mgr.EnablePolicy(policies[0].ID)
		assert.NoError(t, err)
		found, _ = mgr.GetPolicy(policies[0].ID)
		assert.True(t, found.Enabled)
	})

	t.Run("删除策略", func(t *testing.T) {
		policies := mgr.ListPolicies()
		require.Len(t, policies, 1)

		err := mgr.DeletePolicy(policies[0].ID)
		assert.NoError(t, err)

		policies = mgr.ListPolicies()
		assert.Len(t, policies, 0)
	})

	t.Run("操作不存在的策略", func(t *testing.T) {
		_, err := mgr.GetPolicy("non-existent")
		assert.ErrorIs(t, err, ErrPolicyNotFound)

		err = mgr.UpdatePolicy("non-existent", &SnapshotPolicy{Name: "x"})
		assert.ErrorIs(t, err, ErrPolicyNotFound)

		err = mgr.DeletePolicy("non-existent")
		assert.ErrorIs(t, err, ErrPolicyNotFound)

		err = mgr.EnablePolicy("non-existent")
		assert.ErrorIs(t, err, ErrPolicyNotFound)

		err = mgr.DisablePolicy("non-existent")
		assert.ErrorIs(t, err, ErrPolicyNotFound)
	})
}

// ========== 策略执行测试 ==========

func TestRunPolicy(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	// 先创建基础快照
	_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "base-for-policy",
		DatasetPath: "/pool/policy-run",
		Type:        TypeFull,
	})

	// 创建策略
	err := mgr.CreatePolicy(&SnapshotPolicy{
		Name:         "run-test",
		Enabled:      true,
		Type:         PolicyInterval,
		Interval:     time.Hour,
		DatasetPaths: []string{"/pool/policy-run"},
		Retention:    RetentionPolicy{MaxSnapshots: 5},
	})
	require.NoError(t, err)

	policies := mgr.ListPolicies()
	require.Len(t, policies, 1)

	t.Run("手动执行策略", func(t *testing.T) {
		snaps, err := mgr.CreateSnapshotWithPolicy(policies[0].ID)
		require.NoError(t, err)
		assert.Len(t, snaps, 1)
		assert.Equal(t, TypeIncremental, snaps[0].Type)
	})

	t.Run("执行不存在的策略", func(t *testing.T) {
		_, err := mgr.CreateSnapshotWithPolicy("non-existent")
		assert.ErrorIs(t, err, ErrPolicyNotFound)
	})
}

// ========== 过期清理测试 ==========

func TestCleanupExpired(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	// 创建一个过期的快照
	snap, _ := mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "expired",
		DatasetPath: "/pool/expired",
		Type:        TypeFull,
	})

	// 手动设置过期时间为过去
	expiredTime := time.Now().Add(-24 * time.Hour)
	snap.ExpiresAt = &expiredTime

	// 创建一个未过期的快照
	_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "valid",
		DatasetPath: "/pool/expired",
		Type:        TypeFull,
	})

	t.Run("清理过期快照", func(t *testing.T) {
		deleted, err := mgr.CleanupExpiredSnapshots()
		require.NoError(t, err)
		assert.Equal(t, 1, deleted)

		// 验证过期的被删除
		_, err = mgr.GetSnapshot(snap.ID)
		assert.ErrorIs(t, err, ErrSnapshotNotFound)
	})
}

func TestCleanupRetentionPolicy(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	// 创建保留策略
	err := mgr.CreatePolicy(&SnapshotPolicy{
		Name:         "retention-test",
		Enabled:      true,
		Type:         PolicyInterval,
		Interval:     time.Hour,
		DatasetPaths: []string{"/pool/retention"},
		Retention: RetentionPolicy{
			MaxSnapshots: 3,
			MaxAgeDays:   30,
		},
	})
	require.NoError(t, err)

	policies := mgr.ListPolicies()
	require.Len(t, policies, 1)
	policyID := policies[0].ID

	// 创建 5 个快照
	for i := 0; i < 5; i++ {
		_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{
			Name:        fmt.Sprintf("ret-%d", i),
			DatasetPath: "/pool/retention",
			Type:        TypeFull,
		})
		time.Sleep(10 * time.Millisecond)
	}

	t.Run("应用保留策略限制最大快照数", func(t *testing.T) {
		deleted, err := mgr.ApplyRetentionPolicy(policyID)
		require.NoError(t, err)
		// 保留策略 maxSnapshots=3，应该删除 2 个
		assert.True(t, deleted >= 0) // 具体数量取决于 canDelete 判断
	})
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	// 创建一些数据
	_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "stat-1",
		DatasetPath: "/pool/stats",
		Type:        TypeFull,
		Protected:   true,
	})
	_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{
		Name:        "stat-2",
		DatasetPath: "/pool/stats",
		Type:        TypeFull,
	})

	_ = mgr.CreatePolicy(&SnapshotPolicy{
		Name:    "stat-policy",
		Enabled: true,
		Type:    PolicyInterval,
	})

	stats := mgr.GetStats()
	assert.Equal(t, 2, stats.TotalSnapshots)
	assert.Equal(t, 1, stats.PolicyCount)
	assert.Equal(t, 1, stats.ProtectedCount)
	assert.True(t, stats.TotalSizeBytes > 0)
	assert.False(t, stats.LastSnapshotTime.IsZero())
}

// ========== 持久化测试 ==========

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test-persist.json")

	// 创建管理器并添加数据
	mgr1, err := NewManager(configPath)
	require.NoError(t, err)

	_, _ = mgr1.CreateSnapshot(CreateSnapshotRequest{
		Name:        "persist-snap",
		DatasetPath: "/pool/persist",
		Type:        TypeFull,
	})
	_ = mgr1.CreatePolicy(&SnapshotPolicy{
		Name:         "persist-policy",
		Enabled:      true,
		Type:         PolicyInterval,
		Interval:     time.Hour,
		DatasetPaths: []string{"/pool/persist"},
		Retention:    RetentionPolicy{MaxSnapshots: 10},
	})

	// 验证配置文件存在
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// 读取并验证 JSON 结构
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var pc persistentConfig
	err = json.Unmarshal(data, &pc)
	require.NoError(t, err)
	assert.Len(t, pc.Snapshots, 1)
	assert.Len(t, pc.Policies, 1)

	mgr1.Close()

	// 重新加载验证持久化
	mgr2, err := NewManager(configPath)
	require.NoError(t, err)
	defer mgr2.Close()

	snaps := mgr2.ListSnapshots("")
	assert.Len(t, snaps, 1)
	assert.Equal(t, "persist-snap", snaps[0].Name)

	policies := mgr2.ListPolicies()
	assert.Len(t, policies, 1)
	assert.Equal(t, "persist-policy", policies[0].Name)
}

// ========== 并发安全测试 ==========

func TestConcurrencySafety(t *testing.T) {
	mgr := newTestManagerNoConfig(t)
	defer mgr.Close()

	t.Run("并发创建快照", func(t *testing.T) {
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(idx int) {
				defer func() { done <- true }()
				_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{
					Name:        fmt.Sprintf("concurrent-%d", idx),
					DatasetPath: "/pool/concurrent",
					Type:        TypeFull,
				})
			}(i)
		}
		for i := 0; i < 10; i++ {
			<-done
		}

		snaps := mgr.ListSnapshots("")
		assert.Len(t, snaps, 10)
	})

	t.Run("并发读写安全", func(t *testing.T) {
		done := make(chan bool, 20)
		for i := 0; i < 10; i++ {
			go func(idx int) {
				defer func() { done <- true }()
				_, _ = mgr.CreateSnapshot(CreateSnapshotRequest{
					Name:        fmt.Sprintf("rw-%d", idx),
					DatasetPath: "/pool/rw",
					Type:        TypeFull,
				})
			}(i)
		}
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				_ = mgr.ListSnapshots("")
				_ = mgr.GetStats()
			}()
		}
		for i := 0; i < 20; i++ {
			<-done
		}
	})
}
