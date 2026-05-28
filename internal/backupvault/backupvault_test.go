package backupvault

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// Helper
// ============================================================================

func validDestinations() []Destination {
	return []Destination{
		{ID: "d1", Name: "本地磁盘", Type: MediaLocal, Path: "/mnt/local", Enabled: true},
		{ID: "d2", Name: "NAS存储", Type: MediaNAS, Path: "/mnt/nas", Enabled: true},
		{ID: "d3", Name: "云存储", Type: MediaCloud, Path: "s3://bucket", IsOffsite: true, Enabled: true},
	}
}

func validJob() *BackupJob {
	return &BackupJob{
		Name:         "test-backup",
		Source:       "/data",
		Destinations: validDestinations(),
		Strategy:     StrategyFull,
		Encryption:   DefaultEncryptionConfig(),
		Retention:    DefaultRetentionPolicy(),
	}
}

func newTestVault() *BackupVault {
	cfg := DefaultVaultConfig()
	cfg.EncryptionKey = "12345678901234567890123456789012" // 32 bytes
	return NewBackupVault(cfg)
}

// ============================================================================
// 构造函数
// ============================================================================

func TestNewBackupVault_NilConfig(t *testing.T) {
	v := NewBackupVault(nil)
	require.NotNil(t, v)
	assert.NotNil(t, v.config)
	assert.Equal(t, "BackupVault", v.config.Name)
}

func TestNewBackupVault_CustomConfig(t *testing.T) {
	cfg := &VaultConfig{
		Name:          "Custom",
		DataDir:       "/tmp/test",
		MaxConcurrent: 5,
	}
	v := NewBackupVault(cfg)
	assert.Equal(t, "Custom", v.config.Name)
	assert.Equal(t, "/tmp/test", v.config.DataDir)
}

func TestNewBackupVaultWithLogger(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	v := NewBackupVaultWithLogger(logger, nil)
	require.NotNil(t, v)
	assert.Equal(t, "BackupVault", v.config.Name)
}

func TestNewBackupVaultWithLogger_NilBoth(t *testing.T) {
	v := NewBackupVaultWithLogger(nil, nil)
	require.NotNil(t, v)
}

// ============================================================================
// DefaultVaultConfig
// ============================================================================

func TestDefaultVaultConfig(t *testing.T) {
	cfg := DefaultVaultConfig()
	assert.Equal(t, "BackupVault", cfg.Name)
	assert.Equal(t, 3, cfg.MaxConcurrent)
	assert.True(t, cfg.VerifyAfterBackup)
	assert.Equal(t, 30, cfg.RetentionDays)
}

func TestDefaultEncryptionConfig(t *testing.T) {
	enc := DefaultEncryptionConfig()
	assert.True(t, enc.Enabled)
	assert.Equal(t, "aes-256-gcm", enc.Algorithm)
}

func TestDefaultRetentionPolicy(t *testing.T) {
	p := DefaultRetentionPolicy()
	assert.Equal(t, 10, p.KeepLast)
	assert.Equal(t, 7, p.KeepDaily)
	assert.Equal(t, 4, p.KeepWeekly)
	assert.Equal(t, 6, p.KeepMonthly)
	assert.Equal(t, 2, p.KeepYearly)
	assert.Equal(t, 365, p.MaxAgeDays)
}

// ============================================================================
// CreateJob - 正常路径
// ============================================================================

func TestCreateJob_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()

	err := v.CreateJob(job)
	require.NoError(t, err)
	assert.NotEmpty(t, job.ID)
	assert.Equal(t, StatusIdle, job.Status)
	assert.Equal(t, StrategyFull, job.Strategy)
	assert.NotNil(t, job.Encryption)
	assert.NotNil(t, job.Retention)
	assert.False(t, job.CreatedAt.IsZero())
}

func TestCreateJob_AutoGenerateID(t *testing.T) {
	v := newTestVault()
	job := validJob()
	assert.Empty(t, job.ID)

	require.NoError(t, v.CreateJob(job))
	assert.NotEmpty(t, job.ID)
	assert.Len(t, job.ID, 32) // hex of 16 bytes
}

func TestCreateJob_PreservesExistingID(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.ID = "custom-id"

	require.NoError(t, v.CreateJob(job))
	assert.Equal(t, "custom-id", job.ID)
}

func TestCreateJob_DefaultStrategy(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Strategy = ""

	require.NoError(t, v.CreateJob(job))
	assert.Equal(t, StrategyFull, job.Strategy)
}

func TestCreateJob_SetsNextRun_WhenScheduled(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Schedule = &Schedule{
		Type:     ScheduleDaily,
		Interval: 1,
		Enabled:  true,
	}

	require.NoError(t, v.CreateJob(job))
	assert.True(t, job.NextRun.After(time.Now()))
}

func TestCreateJob_NoNextRun_WhenScheduleDisabled(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Schedule = &Schedule{
		Type:    ScheduleDaily,
		Enabled: false,
	}

	require.NoError(t, v.CreateJob(job))
	assert.True(t, job.NextRun.IsZero())
}

func TestCreateJob_StrategyIncremental(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Strategy = StrategyIncremental

	require.NoError(t, v.CreateJob(job))
	assert.Equal(t, StrategyIncremental, job.Strategy)
}

func TestCreateJob_StrategyDifferential(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Strategy = StrategyDifferential

	require.NoError(t, v.CreateJob(job))
	assert.Equal(t, StrategyDifferential, job.Strategy)
}

// ============================================================================
// CreateJob - 3-2-1 策略验证
// ============================================================================

func TestCreateJob_321_TooFewDestinations(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Destinations = []Destination{
		{ID: "d1", Type: MediaLocal, Path: "/a", Enabled: true},
	}

	err := v.CreateJob(job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 3 destinations")
}

func TestCreateJob_321_TwoDestinations(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Destinations = []Destination{
		{ID: "d1", Type: MediaLocal, Path: "/a", Enabled: true},
		{ID: "d2", Type: MediaNAS, Path: "/b", Enabled: true},
	}

	err := v.CreateJob(job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 3 destinations")
}

func TestCreateJob_321_SingleMediaType(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Destinations = []Destination{
		{ID: "d1", Type: MediaLocal, Path: "/a", Enabled: true},
		{ID: "d2", Type: MediaLocal, Path: "/b", Enabled: true},
		{ID: "d3", Type: MediaLocal, Path: "/c", IsOffsite: true, Enabled: true},
	}

	err := v.CreateJob(job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 different media types")
}

func TestCreateJob_321_NoOffsite(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Destinations = []Destination{
		{ID: "d1", Type: MediaLocal, Path: "/a", Enabled: true},
		{ID: "d2", Type: MediaNAS, Path: "/b", Enabled: true},
		{ID: "d3", Type: MediaCloud, Path: "/c", IsOffsite: false, Enabled: true},
	}

	err := v.CreateJob(job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 1 offsite destination")
}

func TestCreateJob_321_DisabledDestinationsIgnored(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Destinations = []Destination{
		{ID: "d1", Type: MediaLocal, Path: "/a", Enabled: true},
		{ID: "d2", Type: MediaNAS, Path: "/b", Enabled: false},
		{ID: "d3", Type: MediaCloud, Path: "/c", IsOffsite: true, Enabled: false},
	}

	err := v.CreateJob(job)
	require.Error(t, err)
	// 只有 1 个 enabled 的 media type，先报媒体类型不足
	assert.Contains(t, err.Error(), "3-2-1 strategy")
}

// ============================================================================
// GetJob / ListJobs / DeleteJob
// ============================================================================

func TestGetJob_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	got, err := v.GetJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, got.ID)
	assert.Equal(t, "test-backup", got.Name)
}

func TestGetJob_NotFound(t *testing.T) {
	v := newTestVault()
	_, err := v.GetJob("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

func TestListJobs_Empty(t *testing.T) {
	v := newTestVault()
	jobs := v.ListJobs()
	assert.Empty(t, jobs)
}

func TestListJobs_Multiple(t *testing.T) {
	v := newTestVault()
	for i := 0; i < 3; i++ {
		job := validJob()
		job.Name = "job-" + string(rune('A'+i))
		require.NoError(t, v.CreateJob(job))
	}

	jobs := v.ListJobs()
	assert.Len(t, jobs, 3)
}

func TestDeleteJob_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	err := v.DeleteJob(job.ID)
	require.NoError(t, err)

	_, err = v.GetJob(job.ID)
	assert.Error(t, err)
}

func TestDeleteJob_NotFound(t *testing.T) {
	v := newTestVault()
	err := v.DeleteJob("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

// ============================================================================
// RunBackup
// ============================================================================

func TestRunBackup_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	result, err := v.RunBackup(job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, result.JobID)
	assert.Equal(t, StrategyFull, result.Strategy)
	assert.Equal(t, int64(1024*1024), result.Size)
	assert.NotZero(t, result.Duration)
	assert.NotNil(t, result.RestorePoint)
	assert.True(t, result.Verified) // VerifyAfterBackup is true by default
}

func TestRunBackup_JobNotFound(t *testing.T) {
	v := newTestVault()
	_, err := v.RunBackup("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

func TestRunBackup_AlreadyRunning(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	// 手动设置为 running
	v.mu.Lock()
	job.Status = StatusRunning
	v.mu.Unlock()

	_, err := v.RunBackup(job.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestRunBackup_CreatesRestorePoint(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	result, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	rp := result.RestorePoint
	assert.NotEmpty(t, rp.ID)
	assert.Equal(t, job.ID, rp.JobID)
	assert.Equal(t, StrategyFull, rp.Strategy)
	assert.NotEmpty(t, rp.Checksum)
	assert.True(t, rp.Encrypted)
	assert.False(t, rp.Timestamp.IsZero())
}

func TestRunBackup_UpdatesJobStatus(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	_, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	got, _ := v.GetJob(job.ID)
	assert.Equal(t, StatusSuccess, got.Status)
	assert.False(t, got.LastRun.IsZero())
}

func TestRunBackup_NextRunCalculated_WhenScheduled(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Schedule = &Schedule{Type: ScheduleDaily, Enabled: true}
	require.NoError(t, v.CreateJob(job))

	_, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	got, _ := v.GetJob(job.ID)
	assert.True(t, got.NextRun.After(time.Now()))
}

func TestRunBackup_Incremental_CreatesChainEntry(t *testing.T) {
	v := newTestVault()

	// 先创建一个全量备份
	fullJob := validJob()
	fullJob.Strategy = StrategyFull
	require.NoError(t, v.CreateJob(fullJob))
	_, err := v.RunBackup(fullJob.ID)
	require.NoError(t, err)

	// 再创建增量备份（同一个 job）
	chains := v.ListBackupChains(fullJob.ID)
	require.Len(t, chains, 1)

	incJob := validJob()
	incJob.Strategy = StrategyIncremental
	require.NoError(t, v.CreateJob(incJob))
	result, err := v.RunBackup(incJob.ID)
	require.NoError(t, err)

	assert.Equal(t, StrategyIncremental, result.Strategy)
}

func TestRunBackup_VerifyDisabled(t *testing.T) {
	v := newTestVault()
	v.config.VerifyAfterBackup = false
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	result, err := v.RunBackup(job.ID)
	require.NoError(t, err)
	assert.False(t, result.Verified)
}

func TestRunBackup_EncryptionDisabled(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Encryption = &EncryptionConfig{Enabled: false}
	require.NoError(t, v.CreateJob(job))

	result, err := v.RunBackup(job.ID)
	require.NoError(t, err)
	assert.False(t, result.RestorePoint.Encrypted)
}

// ============================================================================
// Restore
// ============================================================================

func TestRestore_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	result, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	err = v.Restore(result.RestorePoint.ID, "/tmp/restore")
	assert.NoError(t, err)
}

func TestRestore_PointNotFound(t *testing.T) {
	v := newTestVault()
	err := v.Restore("nonexistent", "/tmp/restore")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restore point not found")
}

// ============================================================================
// Verify
// ============================================================================

func TestVerify_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))
	_, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	verified, err := v.Verify(job.ID)
	require.NoError(t, err)
	assert.True(t, verified)
}

func TestVerify_JobNotFound(t *testing.T) {
	v := newTestVault()
	_, err := v.Verify("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

func TestVerify_NoRestorePoints(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	_, err := v.Verify(job.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no restore points found")
}

// ============================================================================
// GetComplianceReport
// ============================================================================

func TestGetComplianceReport_Compliant(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	report := v.GetComplianceReport()
	assert.Equal(t, 1, report.TotalJobs)
	assert.Equal(t, 1, report.CompliantJobs)
	assert.Equal(t, 0, report.NonCompliant)
	assert.Empty(t, report.Violations)
	assert.True(t, report.Summary.HasTwoMedia)
	assert.True(t, report.Summary.HasOffsite)
}

func TestGetComplianceReport_NonCompliant_NoOffsite(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Destinations = []Destination{
		{ID: "d1", Type: MediaLocal, Path: "/a", Enabled: true},
		{ID: "d2", Type: MediaNAS, Path: "/b", Enabled: true},
		{ID: "d3", Type: MediaCloud, Path: "/c", Enabled: true},
	}
	// 直接注入 jobs，跳过 CreateJob 的验证
	v.mu.Lock()
	job.ID = "test-job"
	job.Status = StatusIdle
	v.jobs[job.ID] = job
	v.mu.Unlock()

	report := v.GetComplianceReport()
	assert.Equal(t, 0, report.CompliantJobs)
	assert.Equal(t, 1, report.NonCompliant)

	found := false
	for _, v := range report.Violations {
		if v.Rule == "1-offsite" {
			found = true
			assert.Equal(t, "high", v.Severity)
		}
	}
	assert.True(t, found, "expected 1-offsite violation")
}

func TestGetComplianceReport_Empty(t *testing.T) {
	v := newTestVault()
	report := v.GetComplianceReport()
	assert.Equal(t, 0, report.TotalJobs)
	assert.Empty(t, report.Violations)
}

func TestGetComplianceReport_MultipleJobs(t *testing.T) {
	v := newTestVault()
	for i := 0; i < 5; i++ {
		job := validJob()
		job.Name = "job-" + string(rune('A'+i))
		require.NoError(t, v.CreateJob(job))
	}

	report := v.GetComplianceReport()
	assert.Equal(t, 5, report.TotalJobs)
	assert.Equal(t, 5, report.CompliantJobs)
}

// ============================================================================
// ListRestorePoints
// ============================================================================

func TestListRestorePoints_Empty(t *testing.T) {
	v := newTestVault()
	points := v.ListRestorePoints("any-job")
	assert.Empty(t, points)
}

func TestListRestorePoints_SortedByTimestamp(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	// 执行多次备份
	for i := 0; i < 3; i++ {
		_, err := v.RunBackup(job.ID)
		require.NoError(t, err)
	}

	points := v.ListRestorePoints(job.ID)
	require.Len(t, points, 3)
	// 验证时间排序
	for i := 1; i < len(points); i++ {
		assert.True(t, points[i].Timestamp.After(points[i-1].Timestamp) || points[i].Timestamp.Equal(points[i-1].Timestamp))
	}
}

// ============================================================================
// SetRetention
// ============================================================================

func TestSetRetention_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	policy := &RetentionPolicy{
		KeepLast:   5,
		KeepDaily:  3,
		MaxAgeDays: 90,
	}
	err := v.SetRetention(job.ID, policy)
	require.NoError(t, err)

	got, _ := v.GetJob(job.ID)
	assert.Equal(t, 5, got.Retention.KeepLast)
	assert.Equal(t, 3, got.Retention.KeepDaily)
	assert.Equal(t, 90, got.Retention.MaxAgeDays)
}

func TestSetRetention_JobNotFound(t *testing.T) {
	v := newTestVault()
	err := v.SetRetention("nonexistent", DefaultRetentionPolicy())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

// ============================================================================
// GetRestorePoint
// ============================================================================

func TestGetRestorePoint_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))
	result, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	rp, err := v.GetRestorePoint(result.RestorePoint.ID)
	require.NoError(t, err)
	assert.Equal(t, result.RestorePoint.ID, rp.ID)
}

func TestGetRestorePoint_NotFound(t *testing.T) {
	v := newTestVault()
	_, err := v.GetRestorePoint("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restore point not found")
}

// ============================================================================
// GetBackupChain / ListBackupChains
// ============================================================================

func TestGetBackupChain_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))
	_, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	chains := v.ListBackupChains(job.ID)
	require.Len(t, chains, 1)

	chain, err := v.GetBackupChain(chains[0].ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, chain.JobID)
	assert.NotNil(t, chain.FullBackup)
	assert.Equal(t, 1, chain.ChainLength)
}

func TestGetBackupChain_NotFound(t *testing.T) {
	v := newTestVault()
	_, err := v.GetBackupChain("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup chain not found")
}

func TestListBackupChains_Empty(t *testing.T) {
	v := newTestVault()
	chains := v.ListBackupChains("any-job")
	assert.Empty(t, chains)
}

func TestBackupChain_FullBackupCreatesNewChain(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	// 执行两次全量备份，应创建两个链
	_, err := v.RunBackup(job.ID)
	require.NoError(t, err)
	_, err = v.RunBackup(job.ID)
	require.NoError(t, err)

	chains := v.ListBackupChains(job.ID)
	assert.Len(t, chains, 2)
}

// ============================================================================
// GetConfig / UpdateConfig
// ============================================================================

func TestGetConfig_ReturnsCopy(t *testing.T) {
	v := newTestVault()
	cfg1 := v.GetConfig()
	cfg2 := v.GetConfig()
	assert.Equal(t, cfg1.Name, cfg2.Name)
	// 修改一个不影响另一个
	cfg1.Name = "modified"
	assert.NotEqual(t, cfg1.Name, cfg2.Name)
}

func TestUpdateConfig_Success(t *testing.T) {
	v := newTestVault()
	newCfg := &VaultConfig{
		Name:          "Updated",
		MaxConcurrent: 10,
	}
	v.UpdateConfig(newCfg)
	assert.Equal(t, "Updated", v.config.Name)
	assert.Equal(t, 10, v.config.MaxConcurrent)
}

func TestUpdateConfig_NilIgnored(t *testing.T) {
	v := newTestVault()
	original := v.config.Name
	v.UpdateConfig(nil)
	assert.Equal(t, original, v.config.Name)
}

// ============================================================================
// RunRestoreDrill
// ============================================================================

func TestRunRestoreDrill_Success(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))
	_, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	ok, err := v.RunRestoreDrill(job.ID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRunRestoreDrill_JobNotFound(t *testing.T) {
	v := newTestVault()
	_, err := v.RunRestoreDrill("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

func TestRunRestoreDrill_NoRestorePoints(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	_, err := v.RunRestoreDrill(job.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no restore points found")
}

// ============================================================================
// CleanupExpired
// ============================================================================

func TestCleanupExpired_NoRetention(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	v.mu.Lock()
	job.Retention = nil
	v.mu.Unlock()

	cleaned, err := v.CleanupExpired(job.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, cleaned)
}

func TestCleanupExpired_JobNotFound(t *testing.T) {
	v := newTestVault()
	_, err := v.CleanupExpired("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

func TestCleanupExpired_CleansOldPoints(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Retention = &RetentionPolicy{MaxAgeDays: 0} // 0 days = everything expires
	require.NoError(t, v.CreateJob(job))

	// 执行备份
	_, err := v.RunBackup(job.ID)
	require.NoError(t, err)

	// 恢复点刚创建，不会被 MaxAgeDays=0 立即清理（因为 time.Since 刚好 > 0）
	// 但实际取决于精度，这里我们手动将恢复点时间改早
	v.mu.Lock()
	for _, rp := range v.restorePoints {
		rp.CreatedAt = time.Now().Add(-48 * time.Hour)
	}
	v.mu.Unlock()

	cleaned, err := v.CleanupExpired(job.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, cleaned)
}

// ============================================================================
// Concurrent Access
// ============================================================================

func TestConcurrent_CreateAndList(t *testing.T) {
	v := newTestVault()
	var wg sync.WaitGroup
	n := 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			job := validJob()
			job.Name = "concurrent-job"
			_ = v.CreateJob(job)
		}(i)
	}

	wg.Wait()
	jobs := v.ListJobs()
	assert.Len(t, jobs, n)
}

func TestConcurrent_RunBackup(t *testing.T) {
	v := newTestVault()
	var wg sync.WaitGroup

	// 创建多个 job
	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		job := validJob()
		job.Name = "job-" + string(rune('A'+i))
		require.NoError(t, v.CreateJob(job))
		ids[i] = job.ID
	}

	// 并发执行备份
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = v.RunBackup(ids[idx])
		}(i)
	}

	wg.Wait()
	for _, err := range errs {
		assert.NoError(t, err)
	}
}

// ============================================================================
// RestoreRequest (types only, no runtime behavior)
// ============================================================================

func TestRestoreRequest_Fields(t *testing.T) {
	req := RestoreRequest{
		RestorePointID: "rp-1",
		Destination:    "/tmp/restore",
		Overwrite:      true,
		DryRun:         false,
	}
	assert.Equal(t, "rp-1", req.RestorePointID)
	assert.True(t, req.Overwrite)
	assert.False(t, req.DryRun)
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestRunBackup_MultipleTimesSameJob(t *testing.T) {
	v := newTestVault()
	job := validJob()
	require.NoError(t, v.CreateJob(job))

	for i := 0; i < 5; i++ {
		result, err := v.RunBackup(job.ID)
		require.NoError(t, err)
		assert.NotNil(t, result.RestorePoint)
	}

	points := v.ListRestorePoints(job.ID)
	assert.Len(t, points, 5)
}

func TestCreateJob_WithNoEncryption(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Encryption = nil

	require.NoError(t, v.CreateJob(job))
	assert.NotNil(t, job.Encryption) // DefaultEncryptionConfig is set
	assert.True(t, job.Encryption.Enabled)
}

func TestCreateJob_WithNoRetention(t *testing.T) {
	v := newTestVault()
	job := validJob()
	job.Retention = nil

	require.NoError(t, v.CreateJob(job))
	assert.NotNil(t, job.Retention) // DefaultRetentionPolicy is set
	assert.Equal(t, 10, job.Retention.KeepLast)
}
