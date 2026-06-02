package cdp

import (
	"testing"
	"time"
)

// TestChangeEventCreation 测试变更事件创建
func TestChangeEventCreation(t *testing.T) {
	event := &ChangeEvent{
		ID:           "evt-001",
		EventType:    EventCreate,
		FilePath:     "/data/documents/report.pdf",
		Size:         1024 * 1024, // 1MB
		ModTime:      time.Now(),
		Checksum:     "sha256:abc123def456",
		BlockRef:     "blk-001",
		Compressed:   true,
		Deduplicated: false,
		Metadata: map[string]string{
			"owner":  "user1",
			"source": "upload",
		},
		Timestamp: time.Now(),
	}

	// 验证基本字段
	if event.ID != "evt-001" {
		t.Errorf("expected ID 'evt-001', got '%s'", event.ID)
	}
	if event.EventType != EventCreate {
		t.Errorf("expected EventType 'create', got '%s'", event.EventType)
	}
	if event.FilePath != "/data/documents/report.pdf" {
		t.Errorf("expected FilePath '/data/documents/report.pdf', got '%s'", event.FilePath)
	}
	if event.Size != 1024*1024 {
		t.Errorf("expected Size 1048576, got %d", event.Size)
	}
	if !event.Compressed {
		t.Error("expected Compressed to be true")
	}
	if event.Deduplicated {
		t.Error("expected Deduplicated to be false")
	}
	if event.Metadata["owner"] != "user1" {
		t.Errorf("expected metadata owner 'user1', got '%s'", event.Metadata["owner"])
	}
}

// TestRecoveryPointCreation 测试恢复点创建
func TestRecoveryPointCreation(t *testing.T) {
	now := time.Now()
	events := []*ChangeEvent{
		{
			ID:        "evt-001",
			EventType: EventModify,
			FilePath:  "/data/file1.txt",
			Size:      100,
			Timestamp: now.Add(-10 * time.Second),
		},
		{
			ID:        "evt-002",
			EventType: EventCreate,
			FilePath:  "/data/file2.txt",
			Size:      200,
			Timestamp: now.Add(-5 * time.Second),
		},
	}

	rp := &RecoveryPoint{
		ID:             "rp-001",
		Timestamp:      now,
		SequenceNum:    1001,
		Events:         events,
		TotalSize:      300,
		CompressedSize: 150,
		ParentID:       "",
		Checksum:       "sha256:rp-checksum",
		Consistent:     true,
		Metadata: map[string]string{
			"trigger": "scheduled",
		},
	}

	// 验证恢复点
	if rp.ID != "rp-001" {
		t.Errorf("expected ID 'rp-001', got '%s'", rp.ID)
	}
	if len(rp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(rp.Events))
	}
	if rp.TotalSize != 300 {
		t.Errorf("expected TotalSize 300, got %d", rp.TotalSize)
	}
	if rp.CompressedSize != 150 {
		t.Errorf("expected CompressedSize 150, got %d", rp.CompressedSize)
	}
	if !rp.Consistent {
		t.Error("expected Consistent to be true")
	}

	// 验证压缩率
	compressionRatio := float64(rp.CompressedSize) / float64(rp.TotalSize)
	if compressionRatio >= 1.0 {
		t.Error("compression ratio should be less than 1.0")
	}
}

// TestCDPolicy 测试保护策略
func TestCDPolicy(t *testing.T) {
	policy := &CDPolicy{
		ID:              "policy-001",
		Name:            "文档保护策略",
		Description:     "保护所有文档类文件",
		Enabled:         true,
		FilePatterns:    []string{"*.doc", "*.docx", "*.pdf", "*.txt"},
		ExcludePatterns: []string{"*.tmp", "*.bak"},
		DirectoryWhitelist: []string{
			"/data/documents",
			"/data/reports",
		},
		DirectoryBlacklist: []string{
			"/data/temp",
			"/data/cache",
		},
		MaxFileSize:   100 * 1024 * 1024, // 100MB
		MinFileSize:   0,
		TrackCreate:   true,
		TrackModify:   true,
		TrackDelete:   true,
		TrackRename:   false,
		Compression:   CompressLZ4,
		Deduplication: true,
		Priority:      10,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 验证策略配置
	if policy.ID != "policy-001" {
		t.Errorf("expected ID 'policy-001', got '%s'", policy.ID)
	}
	if policy.Name != "文档保护策略" {
		t.Errorf("expected Name '文档保护策略', got '%s'", policy.Name)
	}
	if !policy.Enabled {
		t.Error("expected policy to be enabled")
	}
	if len(policy.FilePatterns) != 4 {
		t.Errorf("expected 4 file patterns, got %d", len(policy.FilePatterns))
	}
	if len(policy.ExcludePatterns) != 2 {
		t.Errorf("expected 2 exclude patterns, got %d", len(policy.ExcludePatterns))
	}
	if len(policy.DirectoryWhitelist) != 2 {
		t.Errorf("expected 2 whitelist directories, got %d", len(policy.DirectoryWhitelist))
	}
	if policy.MaxFileSize != 100*1024*1024 {
		t.Errorf("expected MaxFileSize %d, got %d", 100*1024*1024, policy.MaxFileSize)
	}
	if !policy.TrackCreate {
		t.Error("expected TrackCreate to be true")
	}
	if policy.TrackRename {
		t.Error("expected TrackRename to be false")
	}
	if policy.Compression != CompressLZ4 {
		t.Errorf("expected Compression 'lz4', got '%s'", policy.Compression)
	}
	if !policy.Deduplication {
		t.Error("expected Deduplication to be true")
	}
}

// TestStorageBackend 测试存储后端
func TestStorageBackend(t *testing.T) {
	backend := &StorageBackend{
		ID:          "backend-local-001",
		Name:        "本地主存储",
		Type:        StorageLocal,
		BasePath:    "/data/cdp/storage",
		MaxSize:     1024 * 1024 * 1024 * 100, // 100GB
		CurrentSize: 1024 * 1024 * 1024 * 30,  // 30GB
		Enabled:     true,
		TLS:         false,
		Timeout:     30,
		RetryCount:  3,
	}

	// 验证本地存储后端
	if backend.ID != "backend-local-001" {
		t.Errorf("expected ID 'backend-local-001', got '%s'", backend.ID)
	}
	if backend.Type != StorageLocal {
		t.Errorf("expected Type 'local', got '%s'", backend.Type)
	}
	if !backend.Enabled {
		t.Error("expected backend to be enabled")
	}

	// 验证容量计算
	usagePercent := float64(backend.CurrentSize) / float64(backend.MaxSize) * 100
	if usagePercent < 0 || usagePercent > 100 {
		t.Errorf("usage percent out of range: %.2f", usagePercent)
	}
	expectedUsage := 30.0
	if usagePercent < expectedUsage-1 || usagePercent > expectedUsage+1 {
		t.Errorf("expected usage ~%.0f%%, got %.2f%%", expectedUsage, usagePercent)
	}

	// 测试S3存储后端
	s3Backend := &StorageBackend{
		ID:        "backend-s3-001",
		Name:      "AWS S3备份",
		Type:      StorageS3,
		Endpoint:  "https://s3.amazonaws.com",
		Bucket:    "cdp-backup-bucket",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Region:    "us-east-1",
		Enabled:   true,
		TLS:       true,
	}

	if s3Backend.Type != StorageS3 {
		t.Errorf("expected Type 's3', got '%s'", s3Backend.Type)
	}
	if !s3Backend.TLS {
		t.Error("expected TLS to be enabled for S3")
	}
	if s3Backend.Bucket != "cdp-backup-bucket" {
		t.Errorf("expected Bucket 'cdp-backup-bucket', got '%s'", s3Backend.Bucket)
	}
}

// TestRetentionManager 测试保留策略
func TestRetentionManager(t *testing.T) {
	// 测试按时间保留
	timeRetention := &RetentionManager{
		ID:              "retention-time-001",
		Name:            "30天保留策略",
		Mode:            RetentionByTime,
		MaxAge:          30 * 24 * time.Hour, // 30天
		MinRetain:       5,
		CleanupInterval: 1 * time.Hour,
		Enabled:         true,
	}

	if timeRetention.Mode != RetentionByTime {
		t.Errorf("expected Mode 'time', got '%s'", timeRetention.Mode)
	}
	if timeRetention.MaxAge != 30*24*time.Hour {
		t.Errorf("expected MaxAge 30 days, got %v", timeRetention.MaxAge)
	}

	// 测试按数量保留
	countRetention := &RetentionManager{
		ID:        "retention-count-001",
		Name:      "100个恢复点保留",
		Mode:      RetentionByCount,
		MaxCount:  100,
		MinRetain: 10,
		Enabled:   true,
	}

	if countRetention.Mode != RetentionByCount {
		t.Errorf("expected Mode 'count', got '%s'", countRetention.Mode)
	}
	if countRetention.MaxCount != 100 {
		t.Errorf("expected MaxCount 100, got %d", countRetention.MaxCount)
	}

	// 测试按空间保留
	sizeRetention := &RetentionManager{
		ID:        "retention-size-001",
		Name:      "10GB空间限制",
		Mode:      RetentionBySize,
		MaxSize:   10 * 1024 * 1024 * 1024, // 10GB
		MinRetain: 3,
		Enabled:   true,
	}

	if sizeRetention.Mode != RetentionBySize {
		t.Errorf("expected Mode 'size', got '%s'", sizeRetention.Mode)
	}
	if sizeRetention.MaxSize != 10*1024*1024*1024 {
		t.Errorf("expected MaxSize 10GB, got %d", sizeRetention.MaxSize)
	}
}

// TestPointInTimeRecovery 测试时间点恢复
func TestPointInTimeRecovery(t *testing.T) {
	targetTime := time.Now().Add(-2 * time.Hour)

	recovery := &PointInTimeRecovery{
		ID:         "recovery-001",
		TargetTime: targetTime,
		SourcePath: "/data/protected",
		TargetPath: "/data/restored",
		RecoveryPoint: &RecoveryPoint{
			ID:          "rp-related-001",
			Timestamp:   targetTime.Add(-1 * time.Minute),
			SequenceNum: 500,
		},
		Status:        RecoveryInProgress,
		Progress:      65.5,
		TotalFiles:    1000,
		RestoredFiles: 655,
		TotalBytes:    10 * 1024 * 1024 * 1024, // 10GB
		RestoredBytes: 6 * 1024 * 1024 * 1024,  // 6GB
		StartTime:     time.Now().Add(-30 * time.Minute),
		Options: &RecoveryOptions{
			Overwrite:    false,
			PreserveACL:  true,
			PreserveTime: true,
			DryRun:       false,
		},
	}

	// 验证恢复任务状态
	if recovery.Status != RecoveryInProgress {
		t.Errorf("expected Status 'in_progress', got '%s'", recovery.Status)
	}
	if recovery.Progress != 65.5 {
		t.Errorf("expected Progress 65.5, got %.1f", recovery.Progress)
	}
	if recovery.RestoredFiles != 655 {
		t.Errorf("expected RestoredFiles 655, got %d", recovery.RestoredFiles)
	}

	// 验证进度一致性
	expectedProgress := float64(recovery.RestoredFiles) / float64(recovery.TotalFiles) * 100
	if abs(recovery.Progress-expectedProgress) > 1.0 {
		t.Errorf("progress mismatch: reported %.1f%%, calculated %.1f%%", recovery.Progress, expectedProgress)
	}

	// 验证恢复选项
	if recovery.Options.Overwrite {
		t.Error("expected Overwrite to be false")
	}
	if !recovery.Options.PreserveACL {
		t.Error("expected PreserveACL to be true")
	}
}

// TestReplicationManager 测试复制管理器
func TestReplicationManager(t *testing.T) {
	source := &StorageBackend{
		ID:       "source-001",
		Name:     "主存储",
		Type:     StorageLocal,
		BasePath: "/data/cdp/primary",
		Enabled:  true,
	}

	targets := []*StorageBackend{
		{
			ID:       "target-001",
			Name:     "异地备份-北京",
			Type:     StorageS3,
			Endpoint: "https://s3.cn-north-1.amazonaws.com.cn",
			Bucket:   "cdp-backup-bj",
			Enabled:  true,
		},
		{
			ID:       "target-002",
			Name:     "异地备份-上海",
			Type:     StorageOSS,
			Endpoint: "https://oss-cn-shanghai.aliyuncs.com",
			Bucket:   "cdp-backup-sh",
			Enabled:  true,
		},
	}

	replication := &ReplicationManager{
		ID:             "replication-001",
		Name:           "双活复制",
		Mode:           ReplicationAsync,
		Source:         source,
		Targets:        targets,
		Enabled:        true,
		Status:         ReplicationSyncing,
		LastSync:       time.Now().Add(-5 * time.Minute),
		PendingBytes:   100 * 1024 * 1024, // 100MB
		SyncInterval:   5 * time.Minute,
		BandwidthLimit: 10 * 1024 * 1024, // 10MB/s
		CompressData:   true,
		EncryptData:    true,
	}

	// 验证复制配置
	if replication.Mode != ReplicationAsync {
		t.Errorf("expected Mode 'async', got '%s'", replication.Mode)
	}
	if len(replication.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(replication.Targets))
	}
	if replication.Source.Type != StorageLocal {
		t.Errorf("expected source Type 'local', got '%s'", replication.Source.Type)
	}
	if !replication.CompressData {
		t.Error("expected CompressData to be true")
	}
	if !replication.EncryptData {
		t.Error("expected EncryptData to be true")
	}
	if replication.Status != ReplicationSyncing {
		t.Errorf("expected Status 'syncing', got '%s'", replication.Status)
	}

	// 验证带宽限制
	if replication.BandwidthLimit != 10*1024*1024 {
		t.Errorf("expected BandwidthLimit 10MB/s, got %d", replication.BandwidthLimit)
	}
}

// TestCDPEngine 测试CDP引擎
func TestCDPEngine(t *testing.T) {
	config := &EngineConfig{
		WatchPaths:          []string{"/data/documents", "/data/projects"},
		ExcludePaths:        []string{"/data/temp", "/data/cache"},
		BatchSize:           100,
		FlushInterval:       5 * time.Second,
		MaxQueueSize:        10000,
		WorkerCount:         4,
		EnableDedup:         true,
		EnableCompress:      true,
		CompressionType:     CompressZstd,
		BlockSize:           4096,
		IndexEnabled:        true,
		ChecksumAlgo:        "sha256",
		HealthCheckInterval: 30 * time.Second,
	}

	policies := []*CDPolicy{
		{
			ID:           "policy-001",
			Name:         "文档保护",
			Enabled:      true,
			FilePatterns: []string{"*.pdf", "*.docx"},
			TrackCreate:  true,
			TrackModify:  true,
		},
	}

	backends := []*StorageBackend{
		{
			ID:       "backend-001",
			Name:     "本地存储",
			Type:     StorageLocal,
			BasePath: "/data/cdp",
			Enabled:  true,
		},
	}

	engine := &CDPEngine{
		ID:       "engine-001",
		Name:     "主CDP引擎",
		Config:   config,
		Policies: policies,
		Backends: backends,
		Retention: &RetentionManager{
			ID:      "retention-001",
			Mode:    RetentionByTime,
			MaxAge:  30 * 24 * time.Hour,
			Enabled: true,
		},
		Stats: &PerformanceStats{
			EventsPerSecond: 1500.5,
			BytesPerSecond:  1024 * 1024 * 50, // 50MB/s
			AvgLatency:      5 * time.Millisecond,
			MaxLatency:      50 * time.Millisecond,
			ActiveMonitors:  2,
			TotalEvents:     1000000,
			TotalBytes:      1024 * 1024 * 1024 * 100, // 100GB
			QueueDepth:      50,
			ErrorCount:      3,
		},
		Status:    EngineStatusRunning,
		StartTime: time.Now().Add(-24 * time.Hour),
	}

	// 验证引擎配置
	if engine.Config.WorkerCount != 4 {
		t.Errorf("expected WorkerCount 4, got %d", engine.Config.WorkerCount)
	}
	if engine.Config.CompressionType != CompressZstd {
		t.Errorf("expected CompressionType 'zstd', got '%s'", engine.Config.CompressionType)
	}
	if !engine.Config.EnableDedup {
		t.Error("expected EnableDedup to be true")
	}

	// 验证性能统计
	if engine.Stats.EventsPerSecond != 1500.5 {
		t.Errorf("expected EventsPerSecond 1500.5, got %.1f", engine.Stats.EventsPerSecond)
	}
	if engine.Stats.ErrorCount != 3 {
		t.Errorf("expected ErrorCount 3, got %d", engine.Stats.ErrorCount)
	}

	// 验证引擎状态
	if engine.Status != EngineStatusRunning {
		t.Errorf("expected Status 'running', got '%s'", engine.Status)
	}

	// 验证运行时间
	uptime := time.Since(engine.StartTime)
	if uptime < 23*time.Hour || uptime > 25*time.Hour {
		t.Errorf("expected uptime around 24h, got %v", uptime)
	}
}

// TestDedupStats 测试去重统计
func TestDedupStats(t *testing.T) {
	stats := &DedupStats{
		TotalChunks:     10000,
		UniqueChunks:    7500,
		DuplicateChunks: 2500,
		TotalSize:       1024 * 1024 * 1024, // 1GB
		DedupSize:       768 * 1024 * 1024,  // 768MB
		SpaceSaved:      256 * 1024 * 1024,  // 256MB
		DedupRatio:      0.25,
	}

	// 验证数据块统计
	if stats.TotalChunks != stats.UniqueChunks+stats.DuplicateChunks {
		t.Error("total chunks should equal unique + duplicate")
	}

	// 验证去重率
	calculatedRatio := float64(stats.DuplicateChunks) / float64(stats.TotalChunks)
	if abs(stats.DedupRatio-calculatedRatio) > 0.001 {
		t.Errorf("dedup ratio mismatch: reported %.3f, calculated %.3f", stats.DedupRatio, calculatedRatio)
	}

	// 验证节省空间
	calculatedSaved := stats.TotalSize - stats.DedupSize
	if stats.SpaceSaved != calculatedSaved {
		t.Errorf("space saved mismatch: reported %d, calculated %d", stats.SpaceSaved, calculatedSaved)
	}

	// 验证去重后大小
	if stats.DedupSize >= stats.TotalSize {
		t.Error("dedup size should be less than total size")
	}
}

// TestEventTypeConstants 测试事件类型常量
func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		event    EventType
		expected string
	}{
		{"create event", EventCreate, "create"},
		{"modify event", EventModify, "modify"},
		{"delete event", EventDelete, "delete"},
		{"rename event", EventRename, "rename"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.event) != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, string(tt.event))
			}
		})
	}
}

// TestRecoveryStatusConstants 测试恢复状态常量
func TestRecoveryStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   RecoveryStatus
		expected string
	}{
		{"pending", RecoveryPending, "pending"},
		{"in_progress", RecoveryInProgress, "in_progress"},
		{"completed", RecoveryCompleted, "completed"},
		{"failed", RecoveryFailed, "failed"},
		{"cancelled", RecoveryCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, string(tt.status))
			}
		})
	}
}

// abs 返回浮点数的绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
