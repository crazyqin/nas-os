// Package audit 提供审计日志Watch/Ignore List功能测试
package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== Watch List 测试 ==========

func TestWatchListManager_AddWatchEntry(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	tests := []struct {
		name    string
		entry   *WatchListEntry
		wantErr bool
	}{
		{
			name: "添加有效监控条目",
			entry: &WatchListEntry{
				Path:       "/data/important",
				Operations: []WatchOperation{WatchOpRead, WatchOpWrite},
				Recursive:  true,
				Enabled:    true,
				CreatedBy:  "user1",
			},
			wantErr: false,
		},
		{
			name: "添加空路径条目应失败",
			entry: &WatchListEntry{
				Path:       "",
				Operations: []WatchOperation{WatchOpAll},
				CreatedBy:  "user1",
			},
			wantErr: true,
		},
		{
			name: "默认操作类型为All",
			entry: &WatchListEntry{
				Path:      "/data/test",
				CreatedBy: "user1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.AddWatchEntry(tt.entry)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, tt.entry.ID)
				assert.False(t, tt.entry.CreatedAt.IsZero())
			}
		})
	}
}

func TestWatchListManager_AddDuplicateWatchEntry(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	entry1 := &WatchListEntry{
		Path:       "/data/important",
		Pattern:    "*.txt",
		Operations: []WatchOperation{WatchOpAll},
		CreatedBy:  "user1",
	}
	entry2 := &WatchListEntry{
		Path:       "/data/important",
		Pattern:    "*.txt",
		Operations: []WatchOperation{WatchOpRead},
		CreatedBy:  "user2",
	}

	err := manager.AddWatchEntry(entry1)
	assert.NoError(t, err)

	err = manager.AddWatchEntry(entry2)
	assert.Error(t, err) // 应该失败，因为路径和pattern相同
}

func TestWatchListManager_UpdateWatchEntry(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	// 先添加一个条目
	entry := &WatchListEntry{
		Path:       "/data/important",
		Operations: []WatchOperation{WatchOpRead},
		Recursive:  false,
		Enabled:    true,
		CreatedBy:  "user1",
	}
	err := manager.AddWatchEntry(entry)
	assert.NoError(t, err)

	// 更新条目
	update := &WatchListEntry{
		ID:         entry.ID,
		Path:       "/data/important-updated",
		Operations: []WatchOperation{WatchOpRead, WatchOpWrite},
		Recursive:  true,
		Enabled:    false,
	}
	err = manager.UpdateWatchEntry(update)
	assert.NoError(t, err)

	// 验证更新
	updated, err := manager.GetWatchEntry(entry.ID)
	assert.NoError(t, err)
	assert.Equal(t, "/data/important-updated", updated.Path)
	assert.True(t, updated.Recursive)
	assert.False(t, updated.Enabled)
}

func TestWatchListManager_DeleteWatchEntry(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	entry := &WatchListEntry{
		Path:       "/data/important",
		Operations: []WatchOperation{WatchOpAll},
		CreatedBy:  "user1",
	}
	err := manager.AddWatchEntry(entry)
	assert.NoError(t, err)

	// 删除存在的条目
	err = manager.DeleteWatchEntry(entry.ID)
	assert.NoError(t, err)

	// 删除不存在的条目
	err = manager.DeleteWatchEntry("non-existent")
	assert.Error(t, err)
}

func TestWatchListManager_ListWatchEntries(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	// 添加多个条目
	for i := 0; i < 5; i++ {
		entry := &WatchListEntry{
			Path:       "/data/dir" + string(rune('A'+i)),
			Operations: []WatchOperation{WatchOpAll},
			Enabled:    i%2 == 0, // 一半启用
			CreatedBy:  "user1",
		}
		_ = manager.AddWatchEntry(entry)
	}

	// 列出所有条目
	entries := manager.ListWatchEntries(WatchListFilter{})
	assert.Len(t, entries, 5)

	// 按启用状态筛选
	enabled := true
	entries = manager.ListWatchEntries(WatchListFilter{Enabled: &enabled})
	assert.Len(t, entries, 3)

	// 分页测试
	entries = manager.ListWatchEntries(WatchListFilter{Limit: 2})
	assert.Len(t, entries, 2)
}

// ========== Ignore List 测试 ==========

func TestWatchListManager_AddIgnoreEntry(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	tests := []struct {
		name    string
		entry   *IgnoreListEntry
		wantErr bool
	}{
		{
			name: "添加有效忽略条目",
			entry: &IgnoreListEntry{
				Path:      "/data/temp",
				Reason:    "临时文件目录",
				Enabled:   true,
				CreatedBy: "user1",
			},
			wantErr: false,
		},
		{
			name: "添加带过期时间的忽略条目",
			entry: &IgnoreListEntry{
				Path:      "/data/cache",
				Reason:    "缓存目录，30天后过期",
				Enabled:   true,
				ExpiresAt: timePtr(time.Now().Add(30 * 24 * time.Hour)),
				CreatedBy: "user1",
			},
			wantErr: false,
		},
		{
			name: "添加空路径条目应失败",
			entry: &IgnoreListEntry{
				Path:      "",
				CreatedBy: "user1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.AddIgnoreEntry(tt.entry)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, tt.entry.ID)
				assert.False(t, tt.entry.CreatedAt.IsZero())
			}
		})
	}
}

func TestWatchListManager_UpdateIgnoreEntry(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	entry := &IgnoreListEntry{
		Path:      "/data/temp",
		Reason:    "临时文件",
		Enabled:   true,
		CreatedBy: "user1",
	}
	err := manager.AddIgnoreEntry(entry)
	assert.NoError(t, err)

	// 更新
	update := &IgnoreListEntry{
		ID:     entry.ID,
		Reason: "更新后的原因",
		Enabled: false,
	}
	err = manager.UpdateIgnoreEntry(update)
	assert.NoError(t, err)

	updated, err := manager.GetIgnoreEntry(entry.ID)
	assert.NoError(t, err)
	assert.Equal(t, "更新后的原因", updated.Reason)
	assert.False(t, updated.Enabled)
}

func TestWatchListManager_DeleteIgnoreEntry(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	entry := &IgnoreListEntry{
		Path:      "/data/temp",
		Enabled:   true,
		CreatedBy: "user1",
	}
	err := manager.AddIgnoreEntry(entry)
	assert.NoError(t, err)

	err = manager.DeleteIgnoreEntry(entry.ID)
	assert.NoError(t, err)

	err = manager.DeleteIgnoreEntry("non-existent")
	assert.Error(t, err)
}

func TestWatchListManager_ExpiredIgnoreEntries(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	// 添加已过期的条目
	expiredEntry := &IgnoreListEntry{
		Path:      "/data/expired",
		Enabled:   true,
		ExpiresAt: timePtr(time.Now().Add(-1 * time.Hour)), // 1小时前过期
		CreatedBy: "user1",
	}
	err := manager.AddIgnoreEntry(expiredEntry)
	assert.NoError(t, err)

	// 添加未过期的条目
	validEntry := &IgnoreListEntry{
		Path:      "/data/valid",
		Enabled:   true,
		ExpiresAt: timePtr(time.Now().Add(24 * time.Hour)),
		CreatedBy: "user1",
	}
	err = manager.AddIgnoreEntry(validEntry)
	assert.NoError(t, err)

	// 列出时应该跳过已过期的条目
	entries := manager.ListIgnoreEntries(IgnoreListFilter{})
	assert.Len(t, entries, 1)
	assert.Equal(t, "/data/valid", entries[0].Path)

	// 清理过期条目
	cleaned := manager.CleanupExpired()
	assert.Equal(t, 1, cleaned)
}

// ========== 路径匹配测试 ==========

func TestWatchListManager_ShouldWatch(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	// 添加监控条目
	watchEntry := &WatchListEntry{
		Path:       "/data/important",
		Operations: []WatchOperation{WatchOpRead, WatchOpWrite},
		Recursive:  true,
		Enabled:    true,
		CreatedBy:  "user1",
	}
	err := manager.AddWatchEntry(watchEntry)
	assert.NoError(t, err)

	// 添加忽略条目
	ignoreEntry := &IgnoreListEntry{
		Path:      "/data/important/temp",
		Enabled:   true,
		CreatedBy: "user1",
	}
	err = manager.AddIgnoreEntry(ignoreEntry)
	assert.NoError(t, err)

	tests := []struct {
		name      string
		path      string
		operation WatchOperation
		expected  bool
	}{
		{
			name:      "监控路径应返回匹配",
			path:      "/data/important/file.txt",
			operation: WatchOpRead,
			expected:  true,
		},
		{
			name:      "非监控操作不应匹配",
			path:      "/data/important/file.txt",
			operation: WatchOpDelete,
			expected:  false,
		},
		{
			name:      "忽略路径不监控",
			path:      "/data/important/temp/cache.txt",
			operation: WatchOpRead,
			expected:  false,
		},
		{
			name:      "不在监控列表的路径",
			path:      "/other/path/file.txt",
			operation: WatchOpRead,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.ShouldWatch(tt.path, tt.operation)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestWatchListManager_IsIgnored(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	ignoreEntry := &IgnoreListEntry{
		Path:      "/data/temp",
		Enabled:   true,
		CreatedBy: "user1",
	}
	err := manager.AddIgnoreEntry(ignoreEntry)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "忽略路径本身",
			path:     "/data/temp",
			expected: true,
		},
		{
			name:     "忽略路径的子路径",
			path:     "/data/temp/subdir/file.txt",
			expected: true,
		},
		{
			name:     "非忽略路径",
			path:     "/data/important/file.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.IsIgnored(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWatchListManager_PatternMatching(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	// 添加带pattern的监控条目
	watchEntry := &WatchListEntry{
		Path:       "/data/logs",
		Pattern:    "*.log",
		Operations: []WatchOperation{WatchOpAll},
		Enabled:    true,
		CreatedBy:  "user1",
	}
	err := manager.AddWatchEntry(watchEntry)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "匹配.log文件",
			path:     "/data/logs/app.log",
			expected: true,
		},
		{
			name:     "不匹配.txt文件",
			path:     "/data/logs/readme.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.ShouldWatch(tt.path, WatchOpRead)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// ========== 统计功能测试 ==========

func TestWatchListManager_GetStats(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())

	// 添加监控条目
	for i := 0; i < 3; i++ {
		entry := &WatchListEntry{
			Path:       "/data/watch" + string(rune('A'+i)),
			Operations: []WatchOperation{WatchOpAll},
			Enabled:    true,
			CreatedBy:  "user1",
		}
		_ = manager.AddWatchEntry(entry)
	}

	// 添加忽略条目
	for i := 0; i < 2; i++ {
		entry := &IgnoreListEntry{
			Path:      "/data/ignore" + string(rune('A'+i)),
			Enabled:   true,
			CreatedBy: "user1",
		}
		_ = manager.AddIgnoreEntry(entry)
	}

	stats := manager.GetStats()
	assert.Equal(t, 3, stats.TotalWatchEntries)
	assert.Equal(t, 2, stats.TotalIgnoreEntries)
	assert.Equal(t, 3, stats.EnabledWatchEntries)
	assert.Equal(t, 2, stats.EnabledIgnoreEntries)
}

// ========== 辅助函数 ==========

func timePtr(t time.Time) *time.Time {
	return &t
}