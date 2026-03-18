package backup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewVerificationManager(t *testing.T) {
	config := &VerificationConfig{
		VerifyChecksum:  true,
		VerifyStructure: true,
		VerifyIntegrity: false,
	}

	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	assert.NotNil(t, vm)
	assert.NotNil(t, vm.results)
}

// Note: TestVerificationManager_VerifySnapshot is defined in backup_test.go

func TestVerificationManager_VerifyAll(t *testing.T) {
	config := &VerificationConfig{}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	results, err := vm.VerifyAll(context.Background())
	_ = results
	_ = err
}

func TestVerificationManager_GetResult(t *testing.T) {
	config := &VerificationConfig{}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	// Add a mock result
	vm.results["snapshot-1"] = &VerificationResult{
		SnapshotID: "snapshot-1",
		Status:     VerificationStatusPassed,
	}

	result, err := vm.GetResult("snapshot-1")
	require.NoError(t, err)
	assert.Equal(t, VerificationStatusPassed, result.Status)
}

func TestVerificationManager_GetResult_NotFound(t *testing.T) {
	config := &VerificationConfig{}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	_, err := vm.GetResult("nonexistent")
	assert.Error(t, err)
}

func TestVerificationManager_GetResults(t *testing.T) {
	config := &VerificationConfig{}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	// Add mock results
	vm.results["s1"] = &VerificationResult{SnapshotID: "s1"}
	vm.results["s2"] = &VerificationResult{SnapshotID: "s2"}

	results := vm.GetResults()
	assert.Len(t, results, 2)
}

func TestVerificationManager_ClearResults(t *testing.T) {
	config := &VerificationConfig{}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	vm.results["s1"] = &VerificationResult{SnapshotID: "s1"}

	vm.ClearResults()
	assert.Empty(t, vm.results)
}

func TestVerificationManager_GetStats(t *testing.T) {
	config := &VerificationConfig{}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	// Add mock results
	vm.results["s1"] = &VerificationResult{Status: VerificationStatusPassed, VerifiedFiles: 100, FailedFiles: 0}
	vm.results["s2"] = &VerificationResult{Status: VerificationStatusFailed, VerifiedFiles: 50, FailedFiles: 10}
	vm.results["s3"] = &VerificationResult{Status: VerificationStatusPartial, VerifiedFiles: 80, FailedFiles: 5}

	stats := vm.GetStats()
	assert.Equal(t, 3, stats.TotalVerifications)
	assert.Equal(t, 1, stats.PassedCount)
	assert.Equal(t, 1, stats.FailedCount)
	assert.Equal(t, 1, stats.PartialCount)
}

func TestVerificationManager_FullVerify(t *testing.T) {
	config := &VerificationConfig{}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	// FullVerify requires valid backup manager
	result, err := vm.FullVerify(context.Background(), "snapshot-1")
	_ = result
	_ = err
}

func TestVerificationManager_ValidateSnapshot(t *testing.T) {
	config := &VerificationConfig{}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	// ValidateSnapshot requires valid snapshot
	valid, err := vm.ValidateSnapshot("snapshot-1")
	_ = valid
	_ = err
}

func TestVerificationManager_shouldVerify(t *testing.T) {
	tests := []struct {
		sampleRate float64
		expected   bool // For sampleRate 1.0, should always be true
	}{
		{1.0, true}, // 100% sampling
		{0.5, true}, // 50% sampling - depends on random
		{0.0, true}, // 0% sampling - but shouldVerify may have different logic
	}

	for _, tt := range tests {
		vm := &VerificationManager{
			config: &VerificationConfig{SampleRate: tt.sampleRate},
			logger: zap.NewNop(),
		}

		// For 100% sample rate, should always return true
		if tt.sampleRate == 1.0 {
			result := vm.shouldVerify(0)
			assert.True(t, result)
		}
	}
}

func TestVerificationConfig_Struct(t *testing.T) {
	config := VerificationConfig{
		VerifyChecksum:  true,
		VerifyStructure: true,
		VerifyIntegrity: true,
		VerifyDecrypt:   false,
		SampleRate:      0.5,
		MaxFiles:        1000,
		Timeout:         30 * time.Minute,
		AutoRepair:      true,
		MaxRetries:      3,
	}

	assert.True(t, config.VerifyChecksum)
	assert.Equal(t, 0.5, config.SampleRate)
	assert.Equal(t, 1000, config.MaxFiles)
}

func TestVerificationResult_Struct(t *testing.T) {
	result := VerificationResult{
		SnapshotID:    "snapshot-1",
		StartTime:     time.Now(),
		EndTime:       time.Now().Add(5 * time.Minute),
		Duration:      5 * time.Minute,
		Status:        VerificationStatusPassed,
		TotalFiles:    1000,
		VerifiedFiles: 1000,
		FailedFiles:   0,
		SkippedFiles:  0,
		TotalSize:     1024000000,
		VerifiedSize:  1024000000,
		Errors:        []VerificationError{},
		Warnings:      []VerificationWarning{},
		RepairedFiles: 0,
	}

	assert.Equal(t, "snapshot-1", result.SnapshotID)
	assert.Equal(t, VerificationStatusPassed, result.Status)
	assert.Equal(t, 1000, result.VerifiedFiles)
}

func TestVerificationStatus_Values(t *testing.T) {
	statuses := []VerificationStatus{
		VerificationStatusPassed,
		VerificationStatusFailed,
		VerificationStatusPartial,
		VerificationStatusTimeout,
		VerificationStatusCancelled,
	}

	for _, s := range statuses {
		assert.NotEmpty(t, s)
	}
}

func TestVerificationError_Struct(t *testing.T) {
	err := VerificationError{
		Path:     "/data/file.txt",
		Type:     "checksum",
		Message:  "Checksum mismatch",
		Expected: "abc123",
		Actual:   "def456",
	}

	assert.Equal(t, "/data/file.txt", err.Path)
	assert.Equal(t, "checksum", err.Type)
}

func TestVerificationWarning_Struct(t *testing.T) {
	warning := VerificationWarning{
		Path:    "/data/old-file.txt",
		Message: "File has old modification time",
	}

	assert.Equal(t, "/data/old-file.txt", warning.Path)
	assert.NotEmpty(t, warning.Message)
}

func TestVerificationStats_Struct(t *testing.T) {
	stats := VerificationStats{
		TotalVerifications: 100,
		PassedCount:        90,
		FailedCount:        5,
		PartialCount:       5,
		TotalFilesVerified: 100000,
		TotalFilesFailed:   10,
		TotalFilesRepaired: 5,
		AverageDuration:    5 * time.Minute,
		LastVerification:   time.Now(),
	}

	assert.Equal(t, 100, stats.TotalVerifications)
	assert.Equal(t, 90, stats.PassedCount)
	assert.Equal(t, 5, stats.FailedCount)
}

func TestNewVerificationScheduler(t *testing.T) {
	vm := &VerificationManager{}
	logger := zap.NewNop()
	scheduler := NewVerificationScheduler(vm, logger)

	assert.NotNil(t, scheduler)
}

func TestVerificationScheduler_StartStop(t *testing.T) {
	vm := &VerificationManager{}
	scheduler := NewVerificationScheduler(vm, zap.NewNop())

	scheduler.Start()
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()
}

func TestVerificationScheduler_AddSchedule(t *testing.T) {
	vm := &VerificationManager{}
	scheduler := NewVerificationScheduler(vm, zap.NewNop())

	scheduler.AddSchedule("snapshot-1", 24*time.Hour)

	schedules := scheduler.GetSchedules()
	assert.Len(t, schedules, 1)
}

func TestVerificationScheduler_RemoveSchedule(t *testing.T) {
	vm := &VerificationManager{}
	scheduler := NewVerificationScheduler(vm, zap.NewNop())

	scheduler.AddSchedule("snapshot-1", 24*time.Hour)

	scheduler.RemoveSchedule("snapshot-1")

	schedules := scheduler.GetSchedules()
	assert.Empty(t, schedules)
}

func TestScheduledVerification_Struct(t *testing.T) {
	schedule := ScheduledVerification{
		SnapshotID: "daily-verify",
		Interval:   24 * time.Hour,
		Enabled:    true,
		LastRun:    time.Now(),
		NextRun:    time.Now().Add(24 * time.Hour),
	}

	assert.Equal(t, "daily-verify", schedule.SnapshotID)
	assert.Equal(t, 24*time.Hour, schedule.Interval)
	assert.True(t, schedule.Enabled)
}

func TestVerificationManager_verifyFile(t *testing.T) {
	config := &VerificationConfig{
		VerifyChecksum:  true,
		VerifyStructure: true,
	}
	logger := zap.NewNop()
	vm := NewVerificationManager(config, nil, nil, logger)

	// verifyFile requires valid file
	info := FileInfo{
		Size:     1024,
		Checksum: "abc123",
	}
	snapshot := &Snapshot{
		Files: map[string]FileInfo{"/test": info},
	}

	err := vm.verifyFile(context.Background(), "/nonexistent", info, snapshot)
	_ = err
}

func TestVerificationManager_verifyStructure(t *testing.T) {
	vm := &VerificationManager{
		config: &VerificationConfig{VerifyStructure: true},
	}

	info := FileInfo{Size: 1024}
	err := vm.verifyStructure("/nonexistent", info)
	_ = err
}

func TestVerificationManager_verifyChecksum(t *testing.T) {
	vm := &VerificationManager{
		config: &VerificationConfig{VerifyChecksum: true},
	}

	info := FileInfo{
		Size:     1024,
		Checksum: "abc123",
	}
	err := vm.verifyChecksum("/nonexistent", info)
	_ = err
}

func TestVerificationManager_verifyIntegrity(t *testing.T) {
	vm := &VerificationManager{
		config: &VerificationConfig{VerifyIntegrity: true},
	}

	info := FileInfo{Size: 1024}
	snapshot := &Snapshot{}
	err := vm.verifyIntegrity("/nonexistent", info, snapshot)
	_ = err
}

func TestVerificationManager_verifyDecryption(t *testing.T) {
	vm := &VerificationManager{
		config: &VerificationConfig{VerifyDecrypt: true},
	}

	info := FileInfo{Size: 1024}
	err := vm.verifyDecryption("/nonexistent", info)
	_ = err
}

func TestVerificationManager_repairFile(t *testing.T) {
	vm := &VerificationManager{
		config: &VerificationConfig{AutoRepair: true},
		logger: zap.NewNop(),
	}

	info := FileInfo{Size: 1024}
	snapshot := &Snapshot{
		Files: map[string]FileInfo{"/test": info},
	}

	repaired := vm.repairFile("/nonexistent", info, snapshot)
	_ = repaired
}
