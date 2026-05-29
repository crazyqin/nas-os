package spotlight

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewIncrementalIndexer(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	indexer := &Indexer{
		config:   config,
		logger:   logger,
		textExts: make(map[string]bool),
	}

	inc := NewIncrementalIndexer(indexer, config, logger)
	assert.NotNil(t, inc)
	assert.NotNil(t, inc.indexer)
	assert.NotNil(t, inc.changes)
	assert.NotNil(t, inc.watchedPaths)
	assert.NotNil(t, inc.deletedPaths)
	assert.False(t, inc.running)
}

func TestIncrementalIndexerStartStop(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	indexer := &Indexer{
		config:   config,
		logger:   logger,
		textExts: make(map[string]bool),
	}

	inc := NewIncrementalIndexer(indexer, config, logger)
	ctx := context.Background()

	// 启动
	err := inc.Start(ctx)
	require.NoError(t, err)
	assert.True(t, inc.IsRunning())

	// 重复启动应该失败
	err = inc.Start(ctx)
	assert.Error(t, err)

	// 停止
	inc.Stop()
	assert.False(t, inc.IsRunning())
}

func TestNotifyChange(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	indexer := &Indexer{
		config:   config,
		logger:   logger,
		textExts: make(map[string]bool),
	}

	inc := NewIncrementalIndexer(indexer, config, logger)
	ctx := context.Background()

	err := inc.Start(ctx)
	require.NoError(t, err)
	defer inc.Stop()

	// 发送变更通知
	inc.NotifyCreate("/test/new-file.txt")
	inc.NotifyModify("/test/modified-file.txt")
	inc.NotifyDelete("/test/deleted-file.txt")

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	stats := inc.GetStats()
	assert.Equal(t, int64(3), stats.TotalChanges)
	assert.Equal(t, int64(1), stats.Created)
	assert.Equal(t, int64(1), stats.Modified)
	assert.Equal(t, int64(1), stats.Deleted)
}

func TestNotifyRename(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	indexer := &Indexer{
		config:   config,
		logger:   logger,
		textExts: make(map[string]bool),
	}

	inc := NewIncrementalIndexer(indexer, config, logger)
	ctx := context.Background()

	err := inc.Start(ctx)
	require.NoError(t, err)
	defer inc.Stop()

	// 发送重命名通知
	inc.NotifyRename("/test/old-name.txt", "/test/new-name.txt")

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	stats := inc.GetStats()
	assert.Equal(t, int64(2), stats.TotalChanges)
	assert.Equal(t, int64(1), stats.Created)
	assert.Equal(t, int64(1), stats.Deleted)
}

func TestNotifyWhenStopped(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	indexer := &Indexer{
		config:   config,
		logger:   logger,
		textExts: make(map[string]bool),
	}

	inc := NewIncrementalIndexer(indexer, config, logger)

	// 未启动时发送通知（不应阻塞或崩溃）
	inc.NotifyCreate("/test/file.txt")
	inc.NotifyModify("/test/file.txt")
	inc.NotifyDelete("/test/file.txt")

	stats := inc.GetStats()
	assert.Equal(t, int64(0), stats.TotalChanges)
}

func TestFileChangeTypes(t *testing.T) {
	assert.Equal(t, ChangeType(0), ChangeCreate)
	assert.Equal(t, ChangeType(1), ChangeModify)
	assert.Equal(t, ChangeType(2), ChangeDelete)
}

func TestFileChangeStruct(t *testing.T) {
	now := time.Now()
	change := FileChange{
		Path:      "/test/file.txt",
		OldPath:   "/test/old-file.txt",
		Type:      ChangeModify,
		Timestamp: now,
		Size:      1024,
	}

	assert.Equal(t, "/test/file.txt", change.Path)
	assert.Equal(t, "/test/old-file.txt", change.OldPath)
	assert.Equal(t, ChangeModify, change.Type)
	assert.Equal(t, now, change.Timestamp)
	assert.Equal(t, int64(1024), change.Size)
}

func TestIncrementalStats(t *testing.T) {
	stats := IncrementalStats{
		TotalChanges:   100,
		Created:        50,
		Modified:       30,
		Deleted:        20,
		LastUpdateTime: time.Now(),
		BatchCount:     10,
		AvgBatchTime:   100.5,
	}

	assert.Equal(t, int64(100), stats.TotalChanges)
	assert.Equal(t, int64(50), stats.Created)
	assert.Equal(t, int64(30), stats.Modified)
	assert.Equal(t, int64(20), stats.Deleted)
	assert.Equal(t, int64(10), stats.BatchCount)
	assert.Equal(t, 100.5, stats.AvgBatchTime)
}

func TestGetWatchedCount(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	indexer := &Indexer{
		config:   config,
		logger:   logger,
		textExts: make(map[string]bool),
	}

	inc := NewIncrementalIndexer(indexer, config, logger)

	// 初始状态
	assert.Equal(t, 0, inc.GetWatchedCount())
}

func TestScanNow(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	indexer := &Indexer{
		config:   config,
		logger:   logger,
		textExts: make(map[string]bool),
	}

	inc := NewIncrementalIndexer(indexer, config, logger)

	// ScanNow 不应崩溃（即使未启动）
	inc.ScanNow()
}

func TestShouldSkip(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	config.ExcludePaths = []string{"/exclude"}
	indexer := &Indexer{
		config:   config,
		logger:   logger,
		textExts: make(map[string]bool),
	}

	inc := NewIncrementalIndexer(indexer, config, logger)

	tests := []struct {
		path     string
		expected bool
	}{
		{"/test/file.txt", false},
		{"/test/.hidden", true},
		{"/test/.hidden/file.txt", true},
		{"/exclude/file.txt", true},
		{"/other/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := inc.shouldSkip(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
