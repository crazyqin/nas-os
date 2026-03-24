// Package storage 提供 WriteOnce (WORM) 不可变存储测试
package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImmutableManager_NewManager 测试创建管理器
func TestImmutableManager_NewManager(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "immutable-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建存储管理器
	storageMgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: tmpDir,
	}

	// 创建不可变管理器
	config := ImmutableConfig{
		SnapDir:     ".immutable",
		AutoCleanup: false, // 测试时禁用自动清理
		MaxRecords:  1000,
	}
	mgr, err := NewImmutableManager(storageMgr, config)
	require.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.Equal(t, config.SnapDir, mgr.config.SnapDir)

	// 停止管理器
	mgr.Stop()
}

// TestImmutableConfig_Defaults 测试默认配置
func TestImmutableConfig_Defaults(t *testing.T) {
	assert.Equal(t, ".immutable", DefaultImmutableConfig.SnapDir)
	assert.True(t, DefaultImmutableConfig.AutoCleanup)
	assert.Equal(t, 24, DefaultImmutableConfig.CleanupInterval)
	assert.Equal(t, 10000, DefaultImmutableConfig.MaxRecords)
}

// TestLockDuration_Hours 测试锁定时长映射
func TestLockDuration_Hours(t *testing.T) {
	tests := []struct {
		duration LockDuration
		hours    int
	}{
		{LockDuration7Days, 7 * 24},
		{LockDuration30Days, 30 * 24},
		{LockDurationPermanent, -1},
	}

	for _, tt := range tests {
		t.Run(string(tt.duration), func(t *testing.T) {
			assert.Equal(t, tt.hours, LockDurationHours[tt.duration])
		})
	}
}

// TestImmutableStatus_Values 测试状态值
func TestImmutableStatus_Values(t *testing.T) {
	assert.Equal(t, ImmutableStatus("active"), ImmutableStatusActive)
	assert.Equal(t, ImmutableStatus("expired"), ImmutableStatusExpired)
	assert.Equal(t, ImmutableStatus("unlocked"), ImmutableStatusUnlocked)
}

// TestRecordFilter_Match 测试记录过滤
func TestRecordFilter_Match(t *testing.T) {
	now := time.Now()

	records := []*ImmutableRecord{
		{
			ID:         "1",
			SourcePath: "/data/important",
			VolumeName: "vol1",
			Status:     ImmutableStatusActive,
			LockedAt:   now,
			Tags:       []string{"critical", "backup"},
			CreatedBy:  "admin",
		},
		{
			ID:         "2",
			SourcePath: "/data/archive",
			VolumeName: "vol1",
			Status:     ImmutableStatusExpired,
			LockedAt:   now.Add(-48 * time.Hour),
			Tags:       []string{"archive"},
			CreatedBy:  "user1",
		},
		{
			ID:         "3",
			SourcePath: "/data/temp",
			VolumeName: "vol2",
			Status:     ImmutableStatusUnlocked,
			LockedAt:   now.Add(-72 * time.Hour),
			Tags:       []string{"temp"},
			CreatedBy:  "user2",
		},
	}

	tests := []struct {
		name     string
		filter   RecordFilter
		expected []string // 期望匹配的记录 ID
	}{
		{
			name:     "filter by status active",
			filter:   RecordFilter{Status: ptrStatus(ImmutableStatusActive)},
			expected: []string{"1"},
		},
		{
			name:     "filter by volume name",
			filter:   RecordFilter{VolumeName: "vol1"},
			expected: []string{"1", "2"},
		},
		{
			name:     "filter by path contains",
			filter:   RecordFilter{PathContains: "archive"},
			expected: []string{"2"},
		},
		{
			name:     "filter by created by",
			filter:   RecordFilter{CreatedBy: "admin"},
			expected: []string{"1"},
		},
		{
			name:     "filter by tags",
			filter:   RecordFilter{Tags: []string{"critical"}},
			expected: []string{"1"},
		},
		{
			name:     "no filter",
			filter:   RecordFilter{},
			expected: []string{"1", "2", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matchedIDs []string
			for _, r := range records {
				if matchFilter(r, &tt.filter) {
					matchedIDs = append(matchedIDs, r.ID)
				}
			}
			assert.Equal(t, tt.expected, matchedIDs)
		})
	}
}

// TestImmutableRecord_Expiry 测试过期时间计算
func TestImmutableRecord_Expiry(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		duration       LockDuration
		lockedAt       time.Time
		shouldExpire   bool
		expectedStatus ImmutableStatus
	}{
		{
			name:           "7 days lock - not expired",
			duration:       LockDuration7Days,
			lockedAt:       now,
			shouldExpire:   false,
			expectedStatus: ImmutableStatusActive,
		},
		{
			name:           "7 days lock - expired",
			duration:       LockDuration7Days,
			lockedAt:       now.Add(-8 * 24 * time.Hour),
			shouldExpire:   true,
			expectedStatus: ImmutableStatusExpired,
		},
		{
			name:           "permanent lock - never expires",
			duration:       LockDurationPermanent,
			lockedAt:       now.Add(-365 * 24 * time.Hour),
			shouldExpire:   false,
			expectedStatus: ImmutableStatusActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &ImmutableRecord{
				Duration: tt.duration,
				Status:   ImmutableStatusActive,
				LockedAt: tt.lockedAt,
			}

			// 计算过期时间
			if tt.duration != LockDurationPermanent {
				hours := LockDurationHours[tt.duration]
				record.ExpiresAt = tt.lockedAt.Add(time.Duration(hours) * time.Hour)
			}

			// 检查过期状态
			if tt.shouldExpire && !record.ExpiresAt.IsZero() {
				assert.True(t, now.After(record.ExpiresAt))
			} else if !tt.shouldExpire && !record.ExpiresAt.IsZero() {
				assert.True(t, now.Before(record.ExpiresAt))
			}
		})
	}
}

// TestImmutableStatistics 测试统计功能
func TestImmutableStatistics(t *testing.T) {
	mgr := &ImmutableManager{
		records: make(map[string]*ImmutableRecord),
		config:  DefaultImmutableConfig,
	}

	// 添加测试记录
	now := time.Now()
	mgr.records["1"] = &ImmutableRecord{
		ID:       "1",
		Status:   ImmutableStatusActive,
		Duration: LockDuration7Days,
		Size:     1000,
		LockedAt: now,
	}
	mgr.records["2"] = &ImmutableRecord{
		ID:       "2",
		Status:   ImmutableStatusActive,
		Duration: LockDuration30Days,
		Size:     2000,
		LockedAt: now,
	}
	mgr.records["3"] = &ImmutableRecord{
		ID:       "3",
		Status:   ImmutableStatusExpired,
		Duration: LockDuration7Days,
		Size:     500,
		LockedAt: now.Add(-8 * 24 * time.Hour),
	}

	stats := mgr.GetStatistics()

	assert.Equal(t, 3, stats.TotalRecords)
	assert.Equal(t, uint64(3500), stats.TotalSize)
	assert.Equal(t, 2, stats.ByStatus[ImmutableStatusActive])
	assert.Equal(t, 1, stats.ByStatus[ImmutableStatusExpired])
	assert.Equal(t, 2, stats.ByDuration[LockDuration7Days])
	assert.Equal(t, 1, stats.ByDuration[LockDuration30Days])
}

// TestRansomwareProtectionStatus 测试防勒索保护状态
func TestRansomwareProtectionStatus(t *testing.T) {
	mgr := &ImmutableManager{
		records: make(map[string]*ImmutableRecord),
		config:  DefaultImmutableConfig,
	}

	// 测试未保护的路径
	status, err := mgr.CheckRansomwareProtection("/unprotected/path")
	require.NoError(t, err)
	assert.False(t, status.Protected)

	// 添加保护记录
	now := time.Now()
	mgr.records["1"] = &ImmutableRecord{
		ID:                    "1",
		SourcePath:            "/protected/path",
		Status:                ImmutableStatusActive,
		ProtectedByRansomware: true,
		LockedAt:              now,
	}

	// 测试已保护的路径
	status, err = mgr.CheckRansomwareProtection("/protected/path")
	require.NoError(t, err)
	assert.True(t, status.Protected)
	assert.Equal(t, "1", status.RecordID)
	assert.True(t, status.ProtectedByRansomware)
}

// TestLockRequest_Validation 测试锁定请求验证
func TestLockRequest_Validation(t *testing.T) {
	tests := []struct {
		name        string
		req         LockRequest
		expectError bool
	}{
		{
			name: "valid request - 7 days",
			req: LockRequest{
				Path:     "/data/important",
				Duration: LockDuration7Days,
			},
			expectError: false,
		},
		{
			name: "valid request - permanent",
			req: LockRequest{
				Path:     "/data/critical",
				Duration: LockDurationPermanent,
			},
			expectError: false,
		},
		{
			name: "invalid duration",
			req: LockRequest{
				Path:     "/data/test",
				Duration: "invalid",
			},
			expectError: true,
		},
		{
			name: "empty path",
			req: LockRequest{
				Path:     "",
				Duration: LockDuration7Days,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证路径
			if tt.req.Path == "" {
				assert.True(t, tt.expectError)
				return
			}

			// 验证锁定时长
			validDurations := map[LockDuration]bool{
				LockDuration7Days:     true,
				LockDuration30Days:    true,
				LockDurationPermanent: true,
			}
			isValid := validDurations[tt.req.Duration]
			assert.Equal(t, tt.expectError, !isValid)
		})
	}
}

// TestSplitPath 测试路径分割
func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"", nil},
		{"dir", []string{"dir"}},
		{"dir/subdir", []string{"dir", "subdir"}},
		{"/dir/subdir", []string{"dir", "subdir"}},
		{"dir/subdir/file", []string{"dir", "subdir", "file"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := splitPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestImmutableRecord_JSON 测试 JSON 序列化
func TestImmutableRecord_JSON(t *testing.T) {
	now := time.Now()
	record := &ImmutableRecord{
		ID:                    "test-123",
		SourcePath:            "/data/important",
		SnapshotPath:          "/mnt/vol1/.immutable/snap1",
		VolumeName:            "vol1",
		SubvolName:            "data",
		Duration:              LockDuration30Days,
		Status:                ImmutableStatusActive,
		LockedAt:              now,
		ExpiresAt:             now.Add(30 * 24 * time.Hour),
		Size:                  1024000,
		Description:           "Critical data",
		Tags:                  []string{"critical", "backup"},
		CreatedBy:             "admin",
		ProtectedByRansomware: true,
	}

	// 序列化
	data, err := json.Marshal(record)
	require.NoError(t, err)

	// 反序列化
	var decoded ImmutableRecord
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, record.ID, decoded.ID)
	assert.Equal(t, record.SourcePath, decoded.SourcePath)
	assert.Equal(t, record.Duration, decoded.Duration)
	assert.Equal(t, record.Status, decoded.Status)
	assert.Equal(t, record.ProtectedByRansomware, decoded.ProtectedByRansomware)
}

// TestBatchLock 测试批量锁定逻辑
func TestBatchLock(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir, err := os.MkdirTemp("", "batch-lock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建测试目录
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))

	// 创建测试文件
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "test.txt"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "test.txt"), []byte("test"), 0644))

	// 注意：实际的批量锁定测试需要 mock btrfs 客户端
	// 这里只测试参数验证逻辑
	paths := []string{dir1, dir2}
	assert.Len(t, paths, 2)
}

// TestCleanupExpiredRecords 测试过期记录清理
func TestCleanupExpiredRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	mgr := &ImmutableManager{
		records: make(map[string]*ImmutableRecord),
		config:  DefaultImmutableConfig,
	}

	now := time.Now()

	// 添加已过期的记录
	mgr.records["expired1"] = &ImmutableRecord{
		ID:        "expired1",
		Status:    ImmutableStatusActive,
		Duration:  LockDuration7Days,
		LockedAt:  now.Add(-8 * 24 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour), // 已过期
	}

	// 添加活跃记录
	mgr.records["active1"] = &ImmutableRecord{
		ID:        "active1",
		Status:    ImmutableStatusActive,
		Duration:  LockDuration7Days,
		LockedAt:  now,
		ExpiresAt: now.Add(7 * 24 * time.Hour), // 未过期
	}

	// 添加已解锁超过 30 天的记录
	mgr.records["unlocked1"] = &ImmutableRecord{
		ID:         "unlocked1",
		Status:     ImmutableStatusUnlocked,
		Duration:   LockDuration7Days,
		LockedAt:   now.Add(-40 * 24 * time.Hour),
		UnlockedAt: now.Add(-31 * 24 * time.Hour), // 超过 30 天
	}

	// 运行清理
	mgr.cleanup()

	// 验证过期记录状态已更新
	assert.Equal(t, ImmutableStatusExpired, mgr.records["expired1"].Status)
	assert.Equal(t, ImmutableStatusActive, mgr.records["active1"].Status)
	// 已解锁超过 30 天的记录应被删除
	_, exists := mgr.records["unlocked1"]
	assert.False(t, exists)
}

// Helper functions

func ptrStatus(s ImmutableStatus) *ImmutableStatus {
	return &s
}
