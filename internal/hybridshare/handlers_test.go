package hybridshare

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// 类型测试
// ============================================================

func TestCloudBackend_IsValid(t *testing.T) {
	tests := []struct {
		backend CloudBackend
		valid   bool
	}{
		{BackendAWSS3, true},
		{BackendAliyunOSS, true},
		{BackendTencentCOS, true},
		{BackendMinIO, true},
		{CloudBackend("invalid"), false},
		{CloudBackend(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.backend), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.backend.IsValid())
		})
	}
}

func TestCloudBackend_BackendName(t *testing.T) {
	tests := []struct {
		backend  CloudBackend
		expected string
	}{
		{BackendAWSS3, "Amazon S3"},
		{BackendAliyunOSS, "阿里云 OSS"},
		{BackendTencentCOS, "腾讯云 COS"},
		{BackendMinIO, "MinIO"},
		{CloudBackend("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.backend), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.backend.BackendName())
		})
	}
}

func TestSyncStrategy_IsValid(t *testing.T) {
	tests := []struct {
		strategy SyncStrategy
		valid    bool
	}{
		{SyncRealtime, true},
		{SyncScheduled, true},
		{SyncManual, true},
		{SyncStrategy("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.strategy.IsValid())
		})
	}
}

func TestCachePolicy_IsValid(t *testing.T) {
	tests := []struct {
		policy CachePolicy
		valid  bool
	}{
		{CachePolicyLRU, true},
		{CachePolicyLFU, true},
		{CachePolicyFIFO, true},
		{CachePolicyTTL, true},
		{CachePolicy("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.policy.IsValid())
		})
	}
}

func TestHybridShareConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  HybridShareConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncRealtime,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: HybridShareConfig{
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncRealtime,
			},
			wantErr: true,
		},
		{
			name: "invalid backend",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        CloudBackend("invalid"),
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncRealtime,
			},
			wantErr: true,
		},
		{
			name: "missing bucket",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncRealtime,
			},
			wantErr: true,
		},
		{
			name: "missing access key",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncRealtime,
			},
			wantErr: true,
		},
		{
			name: "missing secret key",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncRealtime,
			},
			wantErr: true,
		},
		{
			name: "missing cache path",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncRealtime,
			},
			wantErr: true,
		},
		{
			name: "invalid cache size",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 0,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncRealtime,
			},
			wantErr: true,
		},
		{
			name: "invalid cache policy",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicy("invalid"),
				SyncStrategy:   SyncRealtime,
			},
			wantErr: true,
		},
		{
			name: "invalid sync strategy",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncStrategy("invalid"),
			},
			wantErr: true,
		},
		{
			name: "scheduled sync without cron or interval",
			config: HybridShareConfig{
				Name:           "test-share",
				Backend:        BackendMinIO,
				Bucket:         "test-bucket",
				AccessKey:      "test-key",
				SecretKey:      "test-secret",
				LocalCachePath: "/tmp/cache",
				CacheSizeBytes: 1024 * 1024 * 1024,
				CachePolicy:    CachePolicyLRU,
				SyncStrategy:   SyncScheduled,
			},
			wantErr: true,
		},
		{
			name: "scheduled sync with interval",
			config: HybridShareConfig{
				Name:            "test-share",
				Backend:         BackendMinIO,
				Bucket:          "test-bucket",
				AccessKey:       "test-key",
				SecretKey:       "test-secret",
				LocalCachePath:  "/tmp/cache",
				CacheSizeBytes:  1024 * 1024 * 1024,
				CachePolicy:     CachePolicyLRU,
				SyncStrategy:    SyncScheduled,
				SyncIntervalMin: 30,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultHybridShareConfig(t *testing.T) {
	config := DefaultHybridShareConfig()
	assert.Equal(t, int64(10*1024*1024*1024), config.CacheSizeBytes)
	assert.Equal(t, CachePolicyLRU, config.CachePolicy)
	assert.Equal(t, SyncRealtime, config.SyncStrategy)
	assert.Equal(t, 4, config.ConcurrentTransfers)
	assert.True(t, config.UseSSL)
	assert.True(t, config.Enabled)
	assert.Equal(t, "inactive", config.Status)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{1536, "1.5 KB"},
		{1024*1024 + 512*1024, "1.5 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================
// Service 测试
// ============================================================

func TestService_CreateShare(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
		CacheSizeBytes: 1024 * 1024 * 1024,
		CachePolicy:    CachePolicyLRU,
		SyncStrategy:   SyncRealtime,
	}

	config, err := service.CreateShare(req)
	require.NoError(t, err)
	assert.NotEmpty(t, config.ID)
	assert.Equal(t, "test-share", config.Name)
	assert.Equal(t, BackendMinIO, config.Backend)
	assert.True(t, config.Enabled)
}

func TestService_CreateShare_DuplicateName(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	_, err := service.CreateShare(req)
	require.NoError(t, err)

	_, err = service.CreateShare(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestService_GetShare(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	fetched, err := service.GetShare(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, created.Name, fetched.Name)
}

func TestService_GetShare_NotFound(t *testing.T) {
	service := NewService()

	_, err := service.GetShare("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_ListShares(t *testing.T) {
	service := NewService()

	// 创建多个共享
	for i := 0; i < 3; i++ {
		req := CreateShareRequest{
			Name:           "share-" + string(rune('a'+i)),
			Backend:        BackendMinIO,
			Bucket:         "bucket-" + string(rune('a'+i)),
			AccessKey:      "key",
			SecretKey:      "secret",
			LocalCachePath: t.TempDir(),
		}
		_, err := service.CreateShare(req)
		require.NoError(t, err)
	}

	shares := service.ListShares()
	assert.Len(t, shares, 3)
}

func TestService_UpdateShare(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	newName := "updated-share"
	updateReq := UpdateShareRequest{
		Name: &newName,
	}

	updated, err := service.UpdateShare(created.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "updated-share", updated.Name)
}

func TestService_DeleteShare(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	err = service.DeleteShare(created.ID)
	require.NoError(t, err)

	_, err = service.GetShare(created.ID)
	assert.Error(t, err)
}

func TestService_DeleteShare_NotFound(t *testing.T) {
	service := NewService()

	err := service.DeleteShare("non-existent")
	assert.Error(t, err)
}

func TestService_AddAndGetFile(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	// 添加文件
	meta := &FileMetadata{
		RelativePath: "documents/test.txt",
		FileName:     "test.txt",
		FileSize:     1024,
		Status:       FileStatusLocal,
	}

	err = service.AddFile(created.ID, meta)
	require.NoError(t, err)
	assert.NotEmpty(t, meta.ID)

	// 获取文件
	fetched, err := service.GetFileMetadata(created.ID, "documents/test.txt")
	require.NoError(t, err)
	assert.Equal(t, "test.txt", fetched.FileName)
	assert.Equal(t, int64(1024), fetched.FileSize)
	assert.Equal(t, int64(1), fetched.AccessCount) // 访问计数应该增加
}

func TestService_CacheAndEvictFile(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
		CacheSizeBytes: 10 * 1024 * 1024, // 10MB
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	// 添加文件
	meta := &FileMetadata{
		RelativePath: "test.txt",
		FileName:     "test.txt",
		FileSize:     1024,
		Status:       FileStatusCloud,
	}
	err = service.AddFile(created.ID, meta)
	require.NoError(t, err)

	// 缓存文件
	err = service.CacheFile(created.ID, "test.txt")
	require.NoError(t, err)

	// 验证缓存状态
	fetched, err := service.GetFileMetadata(created.ID, "test.txt")
	require.NoError(t, err)
	assert.True(t, fetched.IsCached)

	// 驱逐文件
	err = service.EvictFromCache(created.ID, "test.txt")
	require.NoError(t, err)

	// 验证驱逐状态
	fetched, err = service.GetFileMetadata(created.ID, "test.txt")
	require.NoError(t, err)
	assert.False(t, fetched.IsCached)
}

func TestService_PinAndUnpinFile(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	// 添加文件
	meta := &FileMetadata{
		RelativePath: "important.txt",
		FileName:     "important.txt",
		FileSize:     1024,
		Status:       FileStatusLocal,
		IsCached:     true,
	}
	err = service.AddFile(created.ID, meta)
	require.NoError(t, err)

	// 固定文件
	err = service.PinFile(created.ID, "important.txt")
	require.NoError(t, err)

	// 尝试驱逐固定文件应该失败
	err = service.EvictFromCache(created.ID, "important.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pinned")

	// 取消固定
	err = service.UnpinFile(created.ID, "important.txt")
	require.NoError(t, err)

	// 现在可以驱逐了
	err = service.EvictFromCache(created.ID, "important.txt")
	assert.NoError(t, err)
}

func TestService_SyncTask(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	// 启用共享
	enabled := true
	_, err = service.UpdateShare(created.ID, UpdateShareRequest{Enabled: &enabled})
	require.NoError(t, err)

	// 创建同步任务
	syncReq := SyncRequest{
		ShareID:   created.ID,
		Direction: SyncDirectionUpload,
		FilePath:  "test.txt",
	}

	task, err := service.StartSync(syncReq)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, SyncTaskPending, task.Status)

	// 获取任务
	fetched, err := service.GetSyncTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, fetched.ID)

	// 列出任务
	tasks := service.ListSyncTasks(created.ID)
	assert.Len(t, tasks, 1)

	// 更新进度
	err = service.UpdateSyncTaskProgress(task.ID, 50, 512, 1024)
	require.NoError(t, err)

	// 完成任务
	err = service.CompleteSyncTask(task.ID, true, "")
	require.NoError(t, err)

	fetched, err = service.GetSyncTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, SyncTaskCompleted, fetched.Status)
	assert.Equal(t, float64(100), fetched.Progress)
}

func TestService_CancelSyncTask(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	enabled := true
	_, err = service.UpdateShare(created.ID, UpdateShareRequest{Enabled: &enabled})
	require.NoError(t, err)

	syncReq := SyncRequest{
		ShareID:   created.ID,
		Direction: SyncDirectionUpload,
	}

	task, err := service.StartSync(syncReq)
	require.NoError(t, err)

	err = service.CancelSyncTask(task.ID)
	require.NoError(t, err)

	fetched, err := service.GetSyncTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, SyncTaskCancelled, fetched.Status)
}

func TestService_CapacityStats(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
		CacheSizeBytes: 10 * 1024 * 1024,
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	// 添加一些文件
	for i := 0; i < 5; i++ {
		meta := &FileMetadata{
			RelativePath: "file" + string(rune('a'+i)) + ".txt",
			FileName:     "file" + string(rune('a'+i)) + ".txt",
			FileSize:     1024,
			Status:       FileStatusLocal,
			IsCached:     i < 3, // 前3个文件缓存
		}
		err = service.AddFile(created.ID, meta)
		require.NoError(t, err)
	}

	// 获取统计
	stats, err := service.GetCapacityStats(created.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(5), stats.TotalFiles)
	assert.Equal(t, int64(3), stats.CachedFiles)
	assert.Equal(t, int64(3*1024), stats.LocalCacheUsed)
}

func TestService_BandwidthStats(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	stats, err := service.GetBandwidthStats(created.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.CurrentUploadBps)
	assert.Equal(t, int64(0), stats.CurrentDownloadBps)
}

func TestService_Logs(t *testing.T) {
	service := NewService()

	req := CreateShareRequest{
		Name:           "test-share",
		Backend:        BackendMinIO,
		Bucket:         "test-bucket",
		AccessKey:      "test-key",
		SecretKey:      "test-secret",
		LocalCachePath: t.TempDir(),
	}

	created, err := service.CreateShare(req)
	require.NoError(t, err)

	// 事件日志应该有创建记录
	eventLogs := service.GetEventLogs(created.ID, 10)
	assert.GreaterOrEqual(t, len(eventLogs), 1)
	assert.Equal(t, "share_created", eventLogs[0].EventType)

	// 同步日志初始为空
	syncLogs := service.GetSyncLogs(created.ID, 10)
	assert.Len(t, syncLogs, 0)
}
