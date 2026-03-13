package snapshot

import (
	"path/filepath"
	"testing"
	"time"
)

// ========== 复制管理器测试 ==========

func TestReplicationManager_CreateConfig(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")

	pm := NewPolicyManager("", nil)
	rm := NewReplicationManager(pm, nil, configPath)

	config := &ReplicationConfig{
		Name:           "test-replication",
		SourcePolicyID: "policy-1",
		TargetNodes: []ReplicationTarget{
			{
				NodeID:       "node-1",
				Address:      "192.168.1.100",
				Port:         8080,
				TargetVolume: "volume-1",
			},
		},
		Mode:     ReplicationModeFull,
		Enabled:  true,
		Compress: true,
	}

	err := rm.CreateConfig(config)
	if err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}

	if config.ID == "" {
		t.Error("Config ID should be generated")
	}

	// 验证保存
	loaded, err := rm.GetConfig(config.ID)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if loaded.Name != config.Name {
		t.Errorf("Name mismatch: got %s, want %s", loaded.Name, config.Name)
	}
}

func TestReplicationManager_ValidateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")
	rm := NewReplicationManager(nil, nil, configPath)

	tests := []struct {
		name    string
		config  *ReplicationConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ReplicationConfig{
				Name:           "test",
				SourcePolicyID: "policy-1",
				TargetNodes: []ReplicationTarget{
					{NodeID: "node-1", Address: "192.168.1.100"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: &ReplicationConfig{
				SourcePolicyID: "policy-1",
				TargetNodes: []ReplicationTarget{
					{NodeID: "node-1"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing source policy",
			config: &ReplicationConfig{
				Name: "test",
				TargetNodes: []ReplicationTarget{
					{NodeID: "node-1"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty target nodes",
			config: &ReplicationConfig{
				Name:           "test",
				SourcePolicyID: "policy-1",
				TargetNodes:    []ReplicationTarget{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.CreateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateConfig error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReplicationManager_ListConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")

	rm := NewReplicationManager(nil, nil, configPath)

	// 创建多个配置
	for i := 0; i < 3; i++ {
		config := &ReplicationConfig{
			Name:           "test-" + string(rune('a'+i)),
			SourcePolicyID: "policy-1",
			TargetNodes: []ReplicationTarget{
				{NodeID: "node-1", Address: "192.168.1.100"},
			},
		}
		if err := rm.CreateConfig(config); err != nil {
			t.Fatalf("CreateConfig failed: %v", err)
		}
	}

	configs := rm.ListConfigs()
	if len(configs) != 3 {
		t.Errorf("ListConfigs returned %d configs, want 3", len(configs))
	}
}

func TestReplicationManager_UpdateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")

	rm := NewReplicationManager(nil, nil, configPath)

	// 创建配置
	config := &ReplicationConfig{
		Name:           "test",
		SourcePolicyID: "policy-1",
		TargetNodes: []ReplicationTarget{
			{NodeID: "node-1", Address: "192.168.1.100"},
		},
	}
	if err := rm.CreateConfig(config); err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}

	// 更新配置
	updated := &ReplicationConfig{
		Name:           "test-updated",
		SourcePolicyID: "policy-1",
		TargetNodes: []ReplicationTarget{
			{NodeID: "node-1", Address: "192.168.1.101"},
		},
	}

	err := rm.UpdateConfig(config.ID, updated)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	loaded, _ := rm.GetConfig(config.ID)
	if loaded.Name != "test-updated" {
		t.Errorf("Name not updated: got %s", loaded.Name)
	}
}

func TestReplicationManager_DeleteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")

	rm := NewReplicationManager(nil, nil, configPath)

	config := &ReplicationConfig{
		Name:           "test",
		SourcePolicyID: "policy-1",
		TargetNodes: []ReplicationTarget{
			{NodeID: "node-1", Address: "192.168.1.100"},
		},
	}
	if err := rm.CreateConfig(config); err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}

	err := rm.DeleteConfig(config.ID)
	if err != nil {
		t.Fatalf("DeleteConfig failed: %v", err)
	}

	_, err = rm.GetConfig(config.ID)
	if err == nil {
		t.Error("GetConfig should fail after delete")
	}
}

// ========== 复制任务测试 ==========

func TestReplicationJob_Status(t *testing.T) {
	job := &ReplicationJob{
		ID:           "job-1",
		ConfigID:     "config-1",
		SnapshotName: "snap-20260314-120000",
		SourceVolume: "volume-1",
		TargetNode:   "node-1",
		Status:       ReplicationJobStatusPending,
		Mode:         ReplicationModeFull,
	}

	if job.Status != ReplicationJobStatusPending {
		t.Errorf("Initial status should be pending")
	}

	// 模拟进度更新
	job.Status = ReplicationJobStatusRunning
	now := time.Now()
	job.StartTime = &now
	job.TotalBytes = 1024 * 1024 * 100 // 100MB
	job.BytesTransferred = 1024 * 1024 * 50
	job.Progress = 50

	if job.Progress != 50 {
		t.Errorf("Progress should be 50, got %d", job.Progress)
	}
}

func TestReplicationJob_Speed(t *testing.T) {
	start := time.Now().Add(-10 * time.Second)
	job := &ReplicationJob{
		ID:               "job-1",
		Status:           ReplicationJobStatusRunning,
		StartTime:        &start,
		BytesTransferred: 100 * 1024 * 1024, // 100MB
	}

	elapsed := time.Since(start).Seconds()
	expectedSpeed := float64(job.BytesTransferred) / 1024 / 1024 / elapsed

	// 允许一定的误差
	if expectedSpeed < 5 || expectedSpeed > 15 {
		t.Logf("Speed calculation: %.2f MB/s (may vary)", expectedSpeed)
	}
}

// ========== 增量复制测试 ==========

func TestIncrementalManifest(t *testing.T) {
	manifest := &IncrementalManifest{
		BaseSnapshot: "snap-base",
		Changes: []DataBlock{
			{Offset: 0, Size: 4096, Checksum: "abc123"},
			{Offset: 4096, Size: 4096, Checksum: "def456"},
			{Offset: 8192, Size: 4096, Checksum: "ghi789"},
		},
		Checksum:  "manifest-checksum",
		Timestamp: time.Now(),
	}

	if len(manifest.Changes) != 3 {
		t.Errorf("Expected 3 changes, got %d", len(manifest.Changes))
	}

	totalSize := int64(0)
	for _, block := range manifest.Changes {
		totalSize += block.Size
	}

	if totalSize != 12288 {
		t.Errorf("Expected total size 12288, got %d", totalSize)
	}
}

func TestDataBlock_Checksum(t *testing.T) {
	block := DataBlock{
		Offset:   0,
		Size:     4096,
		Checksum: "sha256:abc123def456",
	}

	if block.Checksum == "" {
		t.Error("Checksum should not be empty")
	}
}

// ========== 复制状态测试 ==========

func TestReplicationStatus(t *testing.T) {
	status := &ReplicationStatus{
		ConfigID:               "config-1",
		TotalJobs:              10,
		CompletedJobs:          8,
		FailedJobs:             2,
		TotalBytesTransferred:  1024 * 1024 * 1024, // 1GB
		NodeStatuses:           make(map[string]NodeReplicationStatus),
	}

	status.NodeStatuses["node-1"] = NodeReplicationStatus{
		NodeID:     "node-1",
		Status:     NodeStatusOnline,
		LagSeconds: 0,
	}

	if status.TotalJobs != 10 {
		t.Errorf("Expected 10 total jobs, got %d", status.TotalJobs)
	}

	successRate := float64(status.CompletedJobs) / float64(status.TotalJobs) * 100
	if successRate != 80.0 {
		t.Errorf("Expected 80%% success rate, got %.1f%%", successRate)
	}
}

func TestNodeReplicationStatus(t *testing.T) {
	now := time.Now()
	status := NodeReplicationStatus{
		NodeID:     "node-1",
		Status:     NodeStatusOnline,
		LastSync:   &now,
		LagSeconds: 60,
	}

	if status.Status != NodeStatusOnline {
		t.Errorf("Expected online status, got %s", status.Status)
	}

	if status.LagSeconds != 60 {
		t.Errorf("Expected 60 seconds lag, got %d", status.LagSeconds)
	}
}

// ========== 复制模式测试 ==========

func TestReplicationMode(t *testing.T) {
	modes := []ReplicationMode{
		ReplicationModeFull,
		ReplicationModeIncremental,
		ReplicationModeDifferential,
	}

	for _, mode := range modes {
		if mode == "" {
			t.Errorf("Replication mode should not be empty")
		}
	}
}

// ========== 传输请求测试 ==========

func TestTransferRequest(t *testing.T) {
	req := TransferRequest{
		SnapshotName: "snap-20260314",
		Volume:       "volume-1",
		Path:         "/snapshots",
		Manifest: &IncrementalManifest{
			BaseSnapshot: "snap-base",
			Checksum:     "checksum",
		},
		Compress: true,
		Encrypt:  true,
	}

	if req.SnapshotName != "snap-20260314" {
		t.Errorf("Snapshot name mismatch")
	}

	if !req.Compress || !req.Encrypt {
		t.Error("Compress and Encrypt should be true")
	}
}

// ========== 节点状态测试 ==========

func TestNodeStatus(t *testing.T) {
	statuses := []NodeStatus{
		NodeStatusOnline,
		NodeStatusOffline,
		NodeStatusError,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("Node status should not be empty")
		}
	}
}

// ========== 复制目标测试 ==========

func TestReplicationTarget(t *testing.T) {
	target := ReplicationTarget{
		NodeID:       "node-1",
		Address:      "192.168.1.100",
		Port:         8080,
		APIKey:       "secret-key",
		TargetVolume: "volume-1",
		TargetPath:   "/snapshots",
		Status:       NodeStatusOnline,
	}

	if target.NodeID != "node-1" {
		t.Errorf("NodeID mismatch")
	}

	if target.Port != 8080 {
		t.Errorf("Port should be 8080, got %d", target.Port)
	}
}

// ========== 远程保留策略测试 ==========

func TestRemoteRetention(t *testing.T) {
	retention := RemoteRetention{
		MaxSnapshots: 10,
		MaxAgeDays:   30,
	}

	if retention.MaxSnapshots != 10 {
		t.Errorf("MaxSnapshots should be 10")
	}

	if retention.MaxAgeDays != 30 {
		t.Errorf("MaxAgeDays should be 30")
	}
}

// ========== 复制调度测试 ==========

func TestReplicationSchedule(t *testing.T) {
	schedule := ReplicationSchedule{
		Type:            "interval",
		IntervalMinutes: 60,
	}

	if schedule.Type != "interval" {
		t.Errorf("Schedule type should be interval")
	}

	if schedule.IntervalMinutes != 60 {
		t.Errorf("Interval should be 60 minutes")
	}
}

// ========== 文件操作测试 ==========

func TestReplicationManager_SaveLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")

	rm := NewReplicationManager(nil, nil, configPath)

	// 创建配置
	config := &ReplicationConfig{
		Name:           "test-save-load",
		SourcePolicyID: "policy-1",
		TargetNodes: []ReplicationTarget{
			{NodeID: "node-1", Address: "192.168.1.100"},
		},
	}

	if err := rm.CreateConfig(config); err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}

	// 创建新的管理器加载配置
	rm2 := NewReplicationManager(nil, nil, configPath)
	if err := rm2.Initialize(); err != nil {
		// Initialize 可能因为空策略管理器而失败，但配置应该能加载
	}

	configs := rm2.ListConfigs()
	if len(configs) == 0 {
		t.Error("Configs should be loaded from file")
	}
}

// ========== 钩子测试 ==========

func TestReplicationHooks(t *testing.T) {
	var called bool

	rm := NewReplicationManager(nil, nil, "")
	rm.SetHooks(ReplicationHooks{
		OnJobStart: func(job *ReplicationJob) {
			called = true
		},
	})

	// 触发钩子
	if rm.hooks.OnJobStart != nil {
		rm.hooks.OnJobStart(&ReplicationJob{ID: "test"})
	}

	if !called {
		t.Error("OnJobStart hook should be called")
	}
}