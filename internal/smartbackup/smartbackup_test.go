package smartbackup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.jobs)
	assert.NotNil(t, manager.policies)
	assert.NotNil(t, manager.targets)
	assert.NotNil(t, manager.chains)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.Equal(t, 3, config.MaxConcurrent)
	assert.Equal(t, 0.8, config.LoadThreshold)
	assert.Equal(t, 10, config.MaxChainLength)
	assert.True(t, config.AutoVerify)
	assert.Equal(t, 6, config.CompressionLevel)
	assert.True(t, config.Deduplication)
	assert.Equal(t, 30, config.RetentionDays)
}

func TestInitialize(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoragePath = filepath.Join(tmpDir, "storage")
	config.TempPath = filepath.Join(tmpDir, "temp")

	manager := NewManager(config)
	err := manager.Initialize()
	require.NoError(t, err)

	// 验证目录创建
	assert.DirExists(t, config.StoragePath)
	assert.DirExists(t, config.TempPath)
	assert.DirExists(t, filepath.Join(config.StoragePath, "chains"))
	assert.DirExists(t, filepath.Join(config.StoragePath, "metadata"))
}

func TestAddTarget(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{
		Name:     "local-target",
		Type:     TargetTypeLocal,
		Path:     "/tmp/backup",
		Enabled:  true,
		Priority: 1,
	}

	err := manager.AddTarget(target)
	require.NoError(t, err)
	assert.NotEmpty(t, target.ID)

	// 验证目标已添加
	savedTarget, err := manager.GetTarget(target.ID)
	require.NoError(t, err)
	assert.Equal(t, "local-target", savedTarget.Name)
	assert.Equal(t, TargetTypeLocal, savedTarget.Type)
}

func TestAddTargetValidation(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// 测试空名称
	err := manager.AddTarget(&BackupTarget{Type: TargetTypeLocal})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称不能为空")

	// 测试空类型
	err = manager.AddTarget(&BackupTarget{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "类型不能为空")

	// 测试不支持的类型
	err = manager.AddTarget(&BackupTarget{Name: "test", Type: "invalid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的目标类型")
}

func TestListTargets(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// 添加多个目标
	targets := []*BackupTarget{
		{Name: "target1", Type: TargetTypeLocal, Path: "/tmp/1", Priority: 3, Enabled: true},
		{Name: "target2", Type: TargetTypeS3, Path: "/tmp/2", Priority: 1, Enabled: true},
		{Name: "target3", Type: TargetTypeNFS, Path: "/tmp/3", Priority: 2, Enabled: true},
	}

	for _, target := range targets {
		err := manager.AddTarget(target)
		require.NoError(t, err)
	}

	// 验证按优先级排序
	listed := manager.ListTargets()
	assert.Len(t, listed, 3)
	assert.Equal(t, 1, listed[0].Priority) // target2
	assert.Equal(t, 2, listed[1].Priority) // target3
	assert.Equal(t, 3, listed[2].Priority) // target1
}

func TestRemoveTarget(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	// 移除目标
	err = manager.RemoveTarget(target.ID)
	assert.NoError(t, err)

	// 验证已移除
	_, err = manager.GetTarget(target.ID)
	assert.Error(t, err)
}

func TestRemoveTargetUsedByPolicy(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// 添加目标
	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	// 创建使用该目标的策略
	policy := BackupPolicy{
		Name:    "test-policy",
		Sources: []string{"/data"},
		Targets: []string{target.ID},
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 尝试移除目标（应该失败）
	err = manager.RemoveTarget(target.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "正在被策略")
}

func TestSetPolicy(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// 先添加目标
	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	// 创建策略
	policy := BackupPolicy{
		Name:            "daily-backup",
		Description:     "每日备份",
		Sources:         []string{"/data"},
		Targets:         []string{target.ID},
		BackupType:      BackupTypeIncremental,
		FullBackupEvery: 5,
		Enabled:         true,
	}

	err = manager.SetPolicy(&policy)
	require.NoError(t, err)
	assert.NotEmpty(t, policy.ID)

	// 验证策略
	savedPolicy, err := manager.GetPolicy(policy.ID)
	require.NoError(t, err)
	assert.Equal(t, "daily-backup", savedPolicy.Name)
	assert.True(t, savedPolicy.Enabled)
}

func TestSetPolicyValidation(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// 测试空名称
	err := manager.SetPolicy(&BackupPolicy{Sources: []string{"/data"}, Targets: []string{}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称不能为空")

	// 测试空源路径
	err = manager.SetPolicy(&BackupPolicy{Name: "test", Targets: []string{}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "源路径不能为空")

	// 测试空目标
	err = manager.SetPolicy(&BackupPolicy{Name: "test", Sources: []string{"/data"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "备份目标不能为空")
}

func TestCreateJob(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// 添加目标
	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	// 创建策略
	policy := BackupPolicy{
		Name:            "test-policy",
		Sources:         []string{"/data"},
		Targets:         []string{target.ID},
		BackupType:      BackupTypeFull,
		FullBackupEvery: 5,
		Enabled:         true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 创建任务
	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, job.ID)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, BackupTypeFull, job.BackupType)
	assert.Equal(t, target.ID, job.TargetID)
	assert.NotEmpty(t, job.ChainID)
}

func TestCreateJobDisabledPolicy(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:    "disabled",
		Sources: []string{"/data"},
		Targets: []string{target.ID},
		Enabled: false,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	_, err = manager.CreateJob(policy.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "策略已禁用")
}

func TestCreateJobIncremental(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:            "test-policy",
		Sources:         []string{"/data"},
		Targets:         []string{target.ID},
		BackupType:      BackupTypeIncremental,
		FullBackupEvery: 5,
		Enabled:         true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 第一个任务应该是全量备份
	job1, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)
	assert.Equal(t, BackupTypeFull, job1.BackupType) // 第一次是全量

	// 模拟完成第一个任务
	job1.Status = JobStatusCompleted

	// 第二个任务应该是增量备份
	job2, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)
	assert.Equal(t, BackupTypeIncremental, job2.BackupType)
	assert.Equal(t, job1.ID, job2.ParentID)
}

func TestRunBackup(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoragePath = filepath.Join(tmpDir, "storage")
	config.TempPath = filepath.Join(tmpDir, "temp")
	config.ScheduleWindowStart = "00:00"
	config.ScheduleWindowEnd = "23:59" // 覆盖全天

	manager := NewManager(config)
	err := manager.Initialize()
	require.NoError(t, err)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test data"), 0644)
	require.NoError(t, err)

	// 添加目标
	targetPath := filepath.Join(tmpDir, "target")
	err = os.MkdirAll(targetPath, 0750)
	require.NoError(t, err)

	target := &BackupTarget{
		Name:    "test-target",
		Type:    TargetTypeLocal,
		Path:    targetPath,
		Enabled: true,
	}
	err = manager.AddTarget(target)
	require.NoError(t, err)

	// 创建策略
	policy := BackupPolicy{
		Name:       "test-policy",
		Sources:    []string{testFile},
		Targets:    []string{target.ID},
		BackupType: BackupTypeFull,
		Enabled:    true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 创建并执行任务
	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)

	ctx := context.Background()
	err = manager.RunBackup(ctx, job.ID)
	require.NoError(t, err)

	// 验证任务状态
	savedJob, err := manager.GetJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCompleted, savedJob.Status)
	assert.NotZero(t, savedJob.Size)
	assert.NotEmpty(t, savedJob.Checksum)
}

func TestRunBackupInvalidJob(t *testing.T) {
	manager := NewManager(DefaultConfig())
	ctx := context.Background()

	// 不存在的任务
	err := manager.RunBackup(ctx, "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务不存在")
}

func TestCancelJob(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:    "test",
		Sources: []string{"/data"},
		Targets: []string{target.ID},
		Enabled: true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)

	// 取消任务
	err = manager.CancelJob(job.ID)
	assert.NoError(t, err)

	// 验证状态
	savedJob, err := manager.GetJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCancelled, savedJob.Status)
}

func TestVerifyBackup(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoragePath = filepath.Join(tmpDir, "storage")
	config.TempPath = filepath.Join(tmpDir, "temp")
	config.ScheduleWindowStart = "00:00"
	config.ScheduleWindowEnd = "23:59"

	manager := NewManager(config)
	err := manager.Initialize()
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test data"), 0644)
	require.NoError(t, err)

	targetPath := filepath.Join(tmpDir, "target")
	err = os.MkdirAll(targetPath, 0750)
	require.NoError(t, err)

	target := &BackupTarget{
		Name:    "test-target",
		Type:    TargetTypeLocal,
		Path:    targetPath,
		Enabled: true,
	}
	err = manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:       "test-policy",
		Sources:    []string{testFile},
		Targets:    []string{target.ID},
		BackupType: BackupTypeFull,
		Enabled:    true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 创建并执行备份
	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)

	ctx := context.Background()
	err = manager.RunBackup(ctx, job.ID)
	require.NoError(t, err)

	// 验证备份
	err = manager.VerifyBackup(ctx, job.ID)
	assert.NoError(t, err)
}

func TestGetChain(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:       "test",
		Sources:    []string{"/data"},
		Targets:    []string{target.ID},
		BackupType: BackupTypeFull,
		Enabled:    true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)

	// 获取备份链
	chain, err := manager.GetChain(job.ChainID)
	require.NoError(t, err)
	assert.Equal(t, policy.ID, chain.PolicyID)
	assert.Contains(t, chain.JobIDs, job.ID)
}

func TestPruneChain(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:            "test",
		Sources:         []string{"/data"},
		Targets:         []string{target.ID},
		BackupType:      BackupTypeIncremental,
		FullBackupEvery: 10,
		Enabled:         true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 创建多个任务
	var jobs []*BackupJob
	for i := 0; i < 5; i++ {
		job, err := manager.CreateJob(policy.ID)
		require.NoError(t, err)
		job.Status = JobStatusCompleted
		jobs = append(jobs, job)
	}

	// 修剪备份链，保留最后2个
	err = manager.PruneChain(jobs[0].ChainID, 2)
	assert.NoError(t, err)

	// 验证链已修剪
	chain, err := manager.GetChain(jobs[0].ChainID)
	require.NoError(t, err)
	assert.Len(t, chain.JobIDs, 2)
}

func TestGetStats(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:       "test",
		Sources:    []string{"/data"},
		Targets:    []string{target.ID},
		BackupType: BackupTypeFull,
		Enabled:    true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 创建一些任务
	for i := 0; i < 3; i++ {
		job, err := manager.CreateJob(policy.ID)
		require.NoError(t, err)
		job.Status = JobStatusCompleted
		job.Size = 1024
		job.EndTime = job.StartTime.Add(time.Minute)
	}

	// 获取统计
	stats := manager.GetStats()
	assert.Equal(t, 3, stats.TotalJobs)
	assert.Equal(t, 3, stats.CompletedJobs)
	assert.Equal(t, int64(3072), stats.TotalSize)
	assert.Equal(t, 1, stats.TotalChains)
	assert.NotZero(t, stats.AvgJobDuration)
}

func TestSaveLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoragePath = filepath.Join(tmpDir, "storage")

	// 创建并保存状态
	manager1 := NewManager(config)
	err := manager1.Initialize()
	require.NoError(t, err)

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err = manager1.AddTarget(target)
	require.NoError(t, err)

	policy := &BackupPolicy{
		Name:    "test",
		Sources: []string{"/data"},
		Targets: []string{target.ID},
		Enabled: true,
	}
	err = manager1.SetPolicy(policy)
	require.NoError(t, err)

	err = manager1.SaveState()
	require.NoError(t, err)

	// 加载状态到新管理器
	manager2 := NewManager(config)
	err = manager2.LoadState()
	require.NoError(t, err)

	// 验证数据已加载
	loadedTarget, err := manager2.GetTarget(target.ID)
	require.NoError(t, err)
	assert.Equal(t, "test", loadedTarget.Name)

	loadedPolicy, err := manager2.GetPolicy(policy.ID)
	require.NoError(t, err)
	assert.Equal(t, "test", loadedPolicy.Name)
}

func TestCreateDefaultPolicy(t *testing.T) {
	sources := []string{"/data", "/config"}
	targets := []string{"target1", "target2"}

	policy := CreateDefaultPolicy("default-backup", sources, targets)
	assert.Equal(t, "default-backup", policy.Name)
	assert.Equal(t, BackupTypeIncremental, policy.BackupType)
	assert.Equal(t, sources, policy.Sources)
	assert.Equal(t, targets, policy.Targets)
	assert.True(t, policy.Enabled)
	assert.Equal(t, 5, policy.FullBackupEvery)
	assert.True(t, policy.Compression)
	assert.True(t, policy.Deduplication)
	assert.True(t, policy.VerifyAfter)
}

func TestCreateDailyFullPolicy(t *testing.T) {
	sources := []string{"/data"}
	targets := []string{"target1"}

	policy := CreateDailyFullPolicy("daily-full", sources, targets)
	assert.Equal(t, "daily-full", policy.Name)
	assert.Equal(t, BackupTypeFull, policy.BackupType)
	assert.True(t, policy.Enabled)
	assert.Equal(t, 14, policy.Retention)
}

func TestCreateWeeklyFullIncrementalPolicy(t *testing.T) {
	sources := []string{"/data"}
	targets := []string{"target1"}

	policy := CreateWeeklyFullIncrementalPolicy("weekly", sources, targets)
	assert.Equal(t, "weekly", policy.Name)
	assert.Equal(t, BackupTypeIncremental, policy.BackupType)
	assert.Equal(t, 7, policy.FullBackupEvery)
	assert.Equal(t, 30, policy.Retention)
}

func TestListChains(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	// 创建多个策略和任务
	for i := 0; i < 3; i++ {
		policy := BackupPolicy{
			Name:       "policy-" + string(rune('A'+i)),
			Sources:    []string{"/data"},
			Targets:    []string{target.ID},
			BackupType: BackupTypeFull,
			Enabled:    true,
		}
		err = manager.SetPolicy(&policy)
		require.NoError(t, err)

		_, err = manager.CreateJob(policy.ID)
		require.NoError(t, err)
	}

	chains := manager.ListChains()
	assert.Len(t, chains, 3)
}

func TestLoadStateNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoragePath = filepath.Join(tmpDir, "storage")

	// 确保目录存在
	err := os.MkdirAll(config.StoragePath, 0750)
	require.NoError(t, err)

	manager := NewManager(config)
	err = manager.LoadState()
	assert.NoError(t, err) // 文件不存在应该成功
}

func TestGetVerificationResult(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoragePath = filepath.Join(tmpDir, "storage")
	config.TempPath = filepath.Join(tmpDir, "temp")
	config.ScheduleWindowStart = "00:00"
	config.ScheduleWindowEnd = "23:59"

	manager := NewManager(config)
	err := manager.Initialize()
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test data"), 0644)
	require.NoError(t, err)

	targetPath := filepath.Join(tmpDir, "target")
	err = os.MkdirAll(targetPath, 0750)
	require.NoError(t, err)

	target := &BackupTarget{
		Name:    "test-target",
		Type:    TargetTypeLocal,
		Path:    targetPath,
		Enabled: true,
	}
	err = manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:       "test-policy",
		Sources:    []string{testFile},
		Targets:    []string{target.ID},
		BackupType: BackupTypeFull,
		Enabled:    true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)

	ctx := context.Background()
	err = manager.RunBackup(ctx, job.ID)
	require.NoError(t, err)

	// 获取验证结果
	result, err := manager.GetVerificationResult(job.ID)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.True(t, result.ChecksumOK)
	assert.True(t, result.SizeMatch)
}

func TestRestore(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoragePath = filepath.Join(tmpDir, "storage")
	config.TempPath = filepath.Join(tmpDir, "temp")
	config.ScheduleWindowStart = "00:00"
	config.ScheduleWindowEnd = "23:59"

	manager := NewManager(config)
	err := manager.Initialize()
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test data"), 0644)
	require.NoError(t, err)

	targetPath := filepath.Join(tmpDir, "target")
	err = os.MkdirAll(targetPath, 0750)
	require.NoError(t, err)

	target := &BackupTarget{
		Name:    "test-target",
		Type:    TargetTypeLocal,
		Path:    targetPath,
		Enabled: true,
	}
	err = manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:       "test-policy",
		Sources:    []string{testFile},
		Targets:    []string{target.ID},
		BackupType: BackupTypeFull,
		Enabled:    true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 创建并执行备份
	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)

	ctx := context.Background()
	err = manager.RunBackup(ctx, job.ID)
	require.NoError(t, err)

	// 恢复备份
	restorePath := filepath.Join(tmpDir, "restore")
	err = manager.Restore(ctx, job.ID, restorePath)
	assert.NoError(t, err)
	assert.DirExists(t, restorePath)
}

func TestScheduleType(t *testing.T) {
	schedule := Schedule{
		Type:       "cron",
		Expression: "0 2 * * *",
		LoadAware:  true,
	}
	assert.Equal(t, "cron", schedule.Type)
	assert.Equal(t, "0 2 * * *", schedule.Expression)
	assert.True(t, schedule.LoadAware)
}

func TestBackupJobStatusTransitions(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:    "test",
		Sources: []string{"/data"},
		Targets: []string{target.ID},
		Enabled: true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)

	// 初始状态
	assert.Equal(t, JobStatusPending, job.Status)

	// 模拟运行
	job.Status = JobStatusRunning
	assert.Equal(t, JobStatusRunning, job.Status)

	// 完成
	job.Status = JobStatusCompleted
	assert.Equal(t, JobStatusCompleted, job.Status)
}

func TestMultipleTargets(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// 添加多个目标
	target1 := &BackupTarget{Name: "target1", Type: TargetTypeLocal, Path: "/tmp/1", Priority: 1, Enabled: true}
	target2 := &BackupTarget{Name: "target2", Type: TargetTypeS3, Path: "/tmp/2", Priority: 2, Enabled: true}

	err := manager.AddTarget(target1)
	require.NoError(t, err)
	err = manager.AddTarget(target2)
	require.NoError(t, err)

	// 创建使用多个目标的策略
	policy := BackupPolicy{
		Name:    "multi-target",
		Sources: []string{"/data"},
		Targets: []string{target1.ID, target2.ID},
		Enabled: true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 创建任务（应该选择优先级最高的目标）
	job, err := manager.CreateJob(policy.ID)
	require.NoError(t, err)
	assert.Equal(t, target1.ID, job.TargetID) // 优先级1
}

func TestChainManagement(t *testing.T) {
	manager := NewManager(DefaultConfig())

	target := &BackupTarget{Name: "test", Type: TargetTypeLocal, Path: "/tmp", Enabled: true}
	err := manager.AddTarget(target)
	require.NoError(t, err)

	policy := BackupPolicy{
		Name:            "test",
		Sources:         []string{"/data"},
		Targets:         []string{target.ID},
		BackupType:      BackupTypeIncremental,
		FullBackupEvery: 3,
		Enabled:         true,
	}
	err = manager.SetPolicy(&policy)
	require.NoError(t, err)

	// 创建多个任务
	var jobs []*BackupJob
	for i := 0; i < 3; i++ {
		job, err := manager.CreateJob(policy.ID)
		require.NoError(t, err)
		job.Status = JobStatusCompleted
		jobs = append(jobs, job)
	}

	// 验证链结构
	chain, err := manager.GetChain(jobs[0].ChainID)
	require.NoError(t, err)
	assert.Len(t, chain.JobIDs, 3)
}

func TestConfigValidation(t *testing.T) {
	config := Config{
		MaxConcurrent: 0,
		LoadThreshold: 1.5,
	}

	// 测试无效配置
	manager := NewManager(&config)
	assert.NotNil(t, manager)

	// 测试零值配置
	config2 := Config{}
	manager2 := NewManager(&config2)
	assert.NotNil(t, manager2)
}
