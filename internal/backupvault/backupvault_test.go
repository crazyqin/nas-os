package backupvault

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// Helper
// ============================================================================

func newTestManager() *Manager {
	return NewManager(nil, nil)
}

func validJob() *BackupJob {
	return &BackupJob{
		VaultID:    "vault-local",
		Name:       "test-backup",
		SourcePath: "/data",
		BackupType: BackupTypeFull,
	}
}

// ============================================================================
// 构造函数
// ============================================================================

func TestNewManager_NilConfig(t *testing.T) {
	m := NewManager(nil, nil)
	require.NotNil(t, m)
	assert.NotNil(t, m.config)
	assert.Equal(t, SLALevelSilver, m.config.DefaultSLALevel)
}

func TestNewManager_CustomConfig(t *testing.T) {
	cfg := &BackupVaultConfig{
		Enabled:            true,
		DefaultEncryption:  EncryptionAES256,
		DefaultRetention:   60,
		MaxConcurrentJobs:  5,
		ChunkSizeKB:        128,
		DedupEnabled:       true,
		CompressionEnabled: true,
		DefaultSLALevel:    SLALevelGold,
		AlertOnFailure:     true,
	}
	m := NewManager(nil, cfg)
	assert.Equal(t, 5, m.config.MaxConcurrentJobs)
	assert.Equal(t, 60, m.config.DefaultRetention)
	assert.Equal(t, SLALevelGold, m.config.DefaultSLALevel)
}

func TestNewManager_WithLogger(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	m := NewManager(logger, nil)
	require.NotNil(t, m)
	assert.NotNil(t, m.logger)
}

// ============================================================================
// DefaultBackupVaultConfig
// ============================================================================

func TestDefaultBackupVaultConfig(t *testing.T) {
	cfg := DefaultBackupVaultConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, EncryptionAES256, cfg.DefaultEncryption)
	assert.Equal(t, 30, cfg.DefaultRetention)
	assert.Equal(t, 3, cfg.MaxConcurrentJobs)
	assert.Equal(t, 64, cfg.ChunkSizeKB)
	assert.True(t, cfg.DedupEnabled)
	assert.True(t, cfg.CompressionEnabled)
	assert.Equal(t, SLALevelSilver, cfg.DefaultSLALevel)
	assert.True(t, cfg.AlertOnFailure)
}

// ============================================================================
// ListVaults / GetVault
// ============================================================================

func TestListVaults_Default(t *testing.T) {
	m := newTestManager()
	vaults := m.ListVaults()
	assert.Len(t, vaults, 3) // local, offsite, cloud
}

func TestGetVault_Success(t *testing.T) {
	m := newTestManager()
	v, err := m.GetVault("vault-local")
	require.NoError(t, err)
	assert.Equal(t, "本地保险库", v.Name)
	assert.True(t, v.IsPrimary)
}

func TestGetVault_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetVault("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault not found")
}

// ============================================================================
// CreateJob - 正常路径
// ============================================================================

func TestCreateJob_Success(t *testing.T) {
	m := newTestManager()
	job := validJob()

	created, err := m.CreateJob(job)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, JobStatusPending, created.Status)
	assert.False(t, created.CreatedAt.IsZero())
}

func TestCreateJob_AutoGenerateID(t *testing.T) {
	m := newTestManager()
	job := validJob()
	assert.Empty(t, job.ID)

	created, err := m.CreateJob(job)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Len(t, created.ID, 32) // hex of 16 bytes
}

func TestCreateJob_PreservesExistingID(t *testing.T) {
	m := newTestManager()
	job := validJob()
	job.ID = "custom-id"

	created, err := m.CreateJob(job)
	require.NoError(t, err)
	assert.Equal(t, "custom-id", created.ID)
}

func TestCreateJob_VaultNotFound(t *testing.T) {
	m := newTestManager()
	job := validJob()
	job.VaultID = "nonexistent"

	_, err := m.CreateJob(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault not found")
}

func TestCreateJob_DisabledVault(t *testing.T) {
	m := newTestManager()
	m.config.Enabled = false
	job := validJob()

	_, err := m.CreateJob(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestCreateJob_IncrementalType(t *testing.T) {
	m := newTestManager()
	job := validJob()
	job.BackupType = BackupTypeIncremental

	created, err := m.CreateJob(job)
	require.NoError(t, err)
	assert.Equal(t, BackupTypeIncremental, created.BackupType)
}

func TestCreateJob_DifferentialType(t *testing.T) {
	m := newTestManager()
	job := validJob()
	job.BackupType = BackupTypeDifferential

	created, err := m.CreateJob(job)
	require.NoError(t, err)
	assert.Equal(t, BackupTypeDifferential, created.BackupType)
}

// ============================================================================
// GetJob / ListJobs
// ============================================================================

func TestGetJob_Success(t *testing.T) {
	m := newTestManager()
	job := validJob()
	created, err := m.CreateJob(job)
	require.NoError(t, err)

	got, err := m.GetJob(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "test-backup", got.Name)
}

func TestGetJob_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetJob("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

func TestListJobs_Empty(t *testing.T) {
	m := newTestManager()
	jobs := m.ListJobs()
	assert.Empty(t, jobs)
}

func TestListJobs_Multiple(t *testing.T) {
	m := newTestManager()
	for i := 0; i < 3; i++ {
		job := validJob()
		job.Name = "job-" + string(rune('A'+i))
		_, err := m.CreateJob(job)
		require.NoError(t, err)
	}

	jobs := m.ListJobs()
	assert.Len(t, jobs, 3)
}

// ============================================================================
// DedupStats
// ============================================================================

func TestGetDedupStats_Success(t *testing.T) {
	m := newTestManager()
	stats, err := m.GetDedupStats("vault-local")
	require.NoError(t, err)
	assert.Equal(t, "vault-local", stats.VaultID)
	assert.True(t, stats.TotalChunks > 0)
	assert.True(t, stats.DedupRatio >= 1.0)
}

func TestGetDedupStats_VaultNotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetDedupStats("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault not found")
}

func TestGetDedupStats_Cached(t *testing.T) {
	m := newTestManager()
	stats1, err := m.GetDedupStats("vault-local")
	require.NoError(t, err)

	stats2, err := m.GetDedupStats("vault-local")
	require.NoError(t, err)
	assert.Equal(t, stats1.ID, stats2.ID) // same cached object
}

// ============================================================================
// RunRestoreTest
// ============================================================================

func TestRunRestoreTest_Success(t *testing.T) {
	m := newTestManager()
	test := &RestoreTest{
		VaultID:    "vault-local",
		JobID:      "test-job",
		Name:       "restore-drill-1",
		TargetPath: "/tmp/restore",
	}

	result, err := m.RunRestoreTest(test)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, TestStatusSuccess, result.Status)
	assert.True(t, result.IsSuccessful)
	assert.True(t, result.VerifiedFiles > 0)
	assert.Equal(t, 0, result.CorruptFiles)
	assert.NotNil(t, result.CompletedAt)
}

func TestRunRestoreTest_VaultNotFound(t *testing.T) {
	m := newTestManager()
	test := &RestoreTest{
		VaultID: "nonexistent",
	}

	_, err := m.RunRestoreTest(test)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault not found")
}

func TestRunRestoreTest_DisabledVault(t *testing.T) {
	m := newTestManager()
	m.config.Enabled = false
	test := &RestoreTest{
		VaultID: "vault-local",
	}

	_, err := m.RunRestoreTest(test)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestRunRestoreTest_AutoGenerateID(t *testing.T) {
	m := newTestManager()
	test := &RestoreTest{
		VaultID: "vault-local",
	}

	result, err := m.RunRestoreTest(test)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
}

// ============================================================================
// SLA Policy
// ============================================================================

func TestListSLAs_Default(t *testing.T) {
	m := newTestManager()
	policies := m.ListSLAs()
	assert.Len(t, policies, 4) // bronze, silver, gold, platinum
}

func TestGetSLA_Success(t *testing.T) {
	m := newTestManager()
	p, err := m.GetSLA("sla-gold")
	require.NoError(t, err)
	assert.Equal(t, SLALevelGold, p.Level)
	assert.Equal(t, 60, p.RTOTarget)
	assert.Equal(t, 15, p.RPOTarget)
}

func TestGetSLA_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetSLA("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSetSLA_Success(t *testing.T) {
	m := newTestManager()
	policy := &SLAPolicy{
		Name:               "Custom SLA",
		Level:              SLALevelGold,
		RTOTarget:          30,
		RPOTarget:          10,
		BackupFrequency:    "hourly",
		RetentionDays:      60,
		MinCopies:          3,
		GeoRedundancy:      true,
		EncryptionRequired: true,
		TestFrequency:      "monthly",
	}

	result, err := m.SetSLA(policy)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.True(t, result.IsActive)
}

func TestSetSLA_DisabledVault(t *testing.T) {
	m := newTestManager()
	m.config.Enabled = false
	policy := &SLAPolicy{Name: "test"}

	_, err := m.SetSLA(policy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

// ============================================================================
// GetConfig / UpdateConfig
// ============================================================================

func TestGetConfig_ReturnsCopy(t *testing.T) {
	m := newTestManager()
	cfg1 := m.GetConfig()
	cfg2 := m.GetConfig()
	assert.Equal(t, cfg1.DefaultRetention, cfg2.DefaultRetention)
	cfg1.DefaultRetention = 999
	assert.NotEqual(t, cfg1.DefaultRetention, cfg2.DefaultRetention)
}

func TestUpdateConfig_Success(t *testing.T) {
	m := newTestManager()
	newCfg := &BackupVaultConfig{
		Enabled:           true,
		DefaultEncryption: EncryptionChaCha20,
		DefaultRetention:  90,
		MaxConcurrentJobs: 10,
	}
	m.UpdateConfig(newCfg)
	assert.Equal(t, 10, m.config.MaxConcurrentJobs)
	assert.Equal(t, EncryptionChaCha20, m.config.DefaultEncryption)
}

func TestUpdateConfig_NilIgnored(t *testing.T) {
	m := newTestManager()
	original := m.config.DefaultRetention
	m.UpdateConfig(nil)
	assert.Equal(t, original, m.config.DefaultRetention)
}

// ============================================================================
// GetTest / ListTests
// ============================================================================

func TestGetTest_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetTest("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListTests_Empty(t *testing.T) {
	m := newTestManager()
	tests := m.ListTests()
	assert.Empty(t, tests)
}

func TestListTests_AfterRun(t *testing.T) {
	m := newTestManager()
	test := &RestoreTest{VaultID: "vault-local"}
	_, err := m.RunRestoreTest(test)
	require.NoError(t, err)

	tests := m.ListTests()
	assert.Len(t, tests, 1)
}

// ============================================================================
// Concurrent Access
// ============================================================================

func TestConcurrent_CreateAndList(t *testing.T) {
	m := newTestManager()
	var wg sync.WaitGroup
	n := 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			job := validJob()
			job.Name = "concurrent-job"
			_, _ = m.CreateJob(job)
		}(i)
	}

	wg.Wait()
	jobs := m.ListJobs()
	assert.Len(t, jobs, n)
}

func TestConcurrent_GetVault(t *testing.T) {
	m := newTestManager()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = m.GetVault("vault-local")
		}()
	}

	wg.Wait()
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestCreateJob_AllVaultTypes(t *testing.T) {
	m := newTestManager()
	vaults := []string{"vault-local", "vault-offsite", "vault-cloud"}

	for _, vid := range vaults {
		job := validJob()
		job.VaultID = vid
		job.Name = "job-" + vid
		created, err := m.CreateJob(job)
		require.NoError(t, err)
		assert.Equal(t, vid, created.VaultID)
	}

	assert.Len(t, m.ListJobs(), 3)
}

func TestDefaultVaults_HaveEncryption(t *testing.T) {
	m := newTestManager()
	for _, v := range m.ListVaults() {
		assert.NotEmpty(t, v.Encryption)
	}
}

func TestDefaultSLAs_AreActive(t *testing.T) {
	m := newTestManager()
	for _, p := range m.ListSLAs() {
		assert.True(t, p.IsActive)
	}
}

func TestDedupStats_RatioConsistency(t *testing.T) {
	m := newTestManager()
	stats, err := m.GetDedupStats("vault-local")
	require.NoError(t, err)

	assert.True(t, stats.TotalChunks >= stats.UniqueChunks)
	assert.Equal(t, stats.TotalChunks-stats.UniqueChunks, stats.DuplicateChunks)
	assert.True(t, stats.SpaceSavings >= 0 && stats.SpaceSavings <= 100)
}
