package smartbackup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.policies)
	assert.NotNil(t, manager.executions)
	assert.NotNil(t, manager.targets)
	assert.NotNil(t, manager.chains)
	assert.NotNil(t, manager.verifications)
}

// ==================== 策略管理测试 ====================

func TestCreatePolicy(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	policy := &BackupPolicy{
		Name:        "test-policy",
		BackupType:  BackupTypeFull,
		SourcePaths: []string{"/data"},
		TargetIDs:   []string{"target-1"},
	}

	err := manager.CreatePolicy(policy)
	require.NoError(t, err)
	assert.NotEmpty(t, policy.ID)
	assert.False(t, policy.CreatedAt.IsZero())
	assert.Equal(t, 100.0, policy.HealthScore)
}

func TestCreatePolicyValidation(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// 测试空名称
	err := manager.CreatePolicy(&BackupPolicy{
		SourcePaths: []string{"/data"},
	})
	assert.Error(t, err)

	// 测试空源路径
	err = manager.CreatePolicy(&BackupPolicy{
		Name: "test",
	})
	assert.Error(t, err)
}

func TestGetPolicy(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	policy := &BackupPolicy{
		Name:        "test-policy",
		BackupType:  BackupTypeFull,
		SourcePaths: []string{"/data"},
		TargetIDs:   []string{"target-1"},
	}
	manager.CreatePolicy(policy)

	fetched, err := manager.GetPolicy(policy.ID)
	require.NoError(t, err)
	assert.Equal(t, "test-policy", fetched.Name)
}

func TestGetPolicyNotFound(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	_, err := manager.GetPolicy("nonexistent")
	assert.Error(t, err)
}

func TestListPolicies(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	manager.CreatePolicy(&BackupPolicy{
		Name:        "policy-1",
		SourcePaths: []string{"/data1"},
		TargetIDs:   []string{"target-1"},
	})
	manager.CreatePolicy(&BackupPolicy{
		Name:        "policy-2",
		SourcePaths: []string{"/data2"},
		TargetIDs:   []string{"target-2"},
	})

	policies := manager.ListPolicies()
	assert.Len(t, policies, 2)
}

func TestUpdatePolicy(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	policy := &BackupPolicy{
		Name:        "test-policy",
		BackupType:  BackupTypeFull,
		SourcePaths: []string{"/data"},
		TargetIDs:   []string{"target-1"},
	}
	manager.CreatePolicy(policy)

	policy.Name = "updated-policy"
	err := manager.UpdatePolicy(policy)
	require.NoError(t, err)

	fetched, _ := manager.GetPolicy(policy.ID)
	assert.Equal(t, "updated-policy", fetched.Name)
}

func TestDeletePolicy(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	policy := &BackupPolicy{
		Name:        "test-policy",
		SourcePaths: []string{"/data"},
		TargetIDs:   []string{"target-1"},
	}
	manager.CreatePolicy(policy)

	err := manager.DeletePolicy(policy.ID)
	require.NoError(t, err)

	_, err = manager.GetPolicy(policy.ID)
	assert.Error(t, err)
}

// ==================== 目标管理测试 ====================

func TestCreateTarget(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	target := &BackupTarget{
		Name:      "local-target",
		Type:      TargetTypeLocal,
		LocalPath: "/backup",
	}

	err := manager.CreateTarget(target)
	require.NoError(t, err)
	assert.NotEmpty(t, target.ID)
	assert.Equal(t, "active", target.Status)
}

func TestCreateTargetValidation(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// 测试本地目标无路径
	err := manager.CreateTarget(&BackupTarget{
		Name: "local-target",
		Type: TargetTypeLocal,
	})
	assert.Error(t, err)

	// 测试S3目标无endpoint
	err = manager.CreateTarget(&BackupTarget{
		Name:   "s3-target",
		Type:   TargetTypeS3,
		Bucket: "my-bucket",
	})
	assert.Error(t, err)

	// 测试NAS目标无endpoint
	err = manager.CreateTarget(&BackupTarget{
		Name: "nas-target",
		Type: TargetTypeRemoteNAS,
	})
	assert.Error(t, err)
}

func TestGetTarget(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	target := &BackupTarget{
		Name:      "local-target",
		Type:      TargetTypeLocal,
		LocalPath: "/backup",
	}
	manager.CreateTarget(target)

	fetched, err := manager.GetTarget(target.ID)
	require.NoError(t, err)
	assert.Equal(t, "local-target", fetched.Name)
}

func TestListTargets(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	manager.CreateTarget(&BackupTarget{
		Name:      "target-1",
		Type:      TargetTypeLocal,
		LocalPath: "/backup1",
	})
	manager.CreateTarget(&BackupTarget{
		Name:   "target-2",
		Type:   TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket: "my-bucket",
	})

	targets := manager.ListTargets()
	assert.Len(t, targets, 2)
}

func TestDeleteTarget(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	target := &BackupTarget{
		Name:      "local-target",
		Type:      TargetTypeLocal,
		LocalPath: "/backup",
	}
	manager.CreateTarget(target)

	err := manager.DeleteTarget(target.ID)
	require.NoError(t, err)

	_, err = manager.GetTarget(target.ID)
	assert.Error(t, err)
}

// ==================== 策略分析测试 ====================

func TestAnalyzeStrategy(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	analysis := &StrategyAnalysis{
		DataType:   "documents",
		DataSizeGB: 50,
		ChangeFrequency: &ChangeFrequency{
			DailyChanges: 100,
			ChangeRate:   0.05,
		},
		TargetCount: 3,
	}

	strategy, err := manager.AnalyzeStrategy(analysis)
	require.NoError(t, err)
	assert.NotNil(t, strategy)
	assert.NotEmpty(t, strategy.RecommendedType)
	assert.NotEmpty(t, strategy.Reason)
	assert.NotNil(t, strategy.ThreeTwoOne)
	assert.True(t, strategy.ThreeTwoOne.Compliant)
}

func TestAnalyzeStrategyNilAnalysis(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	_, err := manager.AnalyzeStrategy(nil)
	assert.Error(t, err)
}

func TestAnalyzeStrategyThreeTwoOne(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// 测试不满足3-2-1规则的情况
	analysis := &StrategyAnalysis{
		DataType:   "documents",
		DataSizeGB: 50,
		TargetCount: 1,
	}

	strategy, err := manager.AnalyzeStrategy(analysis)
	require.NoError(t, err)
	assert.NotNil(t, strategy.ThreeTwoOne)
	assert.False(t, strategy.ThreeTwoOne.Compliant)
}

// ==================== 备份链路测试 ====================

func TestCreateBackupChain(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	chain := &BackupChain{
		PolicyID: "policy-1",
		TargetID: "target-1",
	}

	err := manager.CreateBackupChain(chain)
	require.NoError(t, err)
	assert.NotEmpty(t, chain.ID)
	assert.Equal(t, 100.0, chain.HealthScore)
}

func TestGetBackupChain(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	chain := &BackupChain{
		PolicyID: "policy-1",
		TargetID: "target-1",
	}
	manager.CreateBackupChain(chain)

	fetched, err := manager.GetBackupChain(chain.ID)
	require.NoError(t, err)
	assert.Equal(t, "policy-1", fetched.PolicyID)
}

func TestListBackupChains(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	manager.CreateBackupChain(&BackupChain{
		PolicyID: "policy-1",
		TargetID: "target-1",
	})
	manager.CreateBackupChain(&BackupChain{
		PolicyID: "policy-2",
		TargetID: "target-2",
	})

	all := manager.ListBackupChains("")
	assert.Len(t, all, 2)

	filtered := manager.ListBackupChains("policy-1")
	assert.Len(t, filtered, 1)
}

func TestBackupChainHealthScore(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	chain := &BackupChain{
		PolicyID: "policy-1",
		TargetID: "target-1",
	}
	manager.CreateBackupChain(chain)

	// 记录成功的全量备份
	fullExec := &BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		TargetID:   "target-1",
		Status:     BackupStatusSuccess,
		SizeBytes:  1024 * 1024,
		ChainID:    chain.ID,
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(time.Minute),
	}
	manager.RecordExecution(fullExec)

	// 获取更新后的链路
	updatedChain, _ := manager.GetBackupChain(chain.ID)
	assert.NotNil(t, updatedChain.FullBackup)
	assert.Equal(t, 1, updatedChain.ChainLength)
	assert.Equal(t, 100.0, updatedChain.HealthScore)
}

// ==================== 备份验证与恢复测试 ====================

func TestVerifyBackup(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// 记录成功的执行
	execution := &BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		Status:     BackupStatusSuccess,
		SizeBytes:  1024 * 1024,
		FilesTotal: 100,
		FilesCopied: 100,
	}
	manager.RecordExecution(execution)

	// 验证备份
	result, err := manager.VerifyBackup(execution.ID)
	require.NoError(t, err)
	assert.Equal(t, "passed", result.Status)
	assert.Equal(t, 100, result.FilesChecked)
}

func TestVerifyBackupFailed(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	execution := &BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		Status:     BackupStatusFailed,
		FilesTotal: 100,
	}
	manager.RecordExecution(execution)

	result, err := manager.VerifyBackup(execution.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
}

func TestTestRecovery(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	execution := &BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		Status:     BackupStatusSuccess,
		SizeBytes:  1024 * 1024,
		FilesTotal: 100,
		FilesCopied: 100,
	}
	manager.RecordExecution(execution)

	result, err := manager.TestRecovery(execution.ID, "/tmp/restore-test")
	require.NoError(t, err)
	assert.Equal(t, "passed", result.Status)
	assert.NotNil(t, result.RecoveryTest)
	assert.Equal(t, "/tmp/restore-test", result.RecoveryTest.TestPath)
	assert.Equal(t, "passed", result.RecoveryTest.Status)
}

func TestGetVerification(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	execution := &BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		Status:     BackupStatusSuccess,
		FilesTotal: 50,
		FilesCopied: 50,
	}
	manager.RecordExecution(execution)

	result, _ := manager.VerifyBackup(execution.ID)

	fetched, err := manager.GetVerification(result.ID)
	require.NoError(t, err)
	assert.Equal(t, "passed", fetched.Status)
}

func TestListVerifications(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	exec1 := &BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		Status:     BackupStatusSuccess,
		FilesTotal: 50,
		FilesCopied: 50,
	}
	manager.RecordExecution(exec1)

	manager.VerifyBackup(exec1.ID)

	all := manager.ListVerifications("")
	assert.GreaterOrEqual(t, len(all), 1)
}

// ==================== 智能调度测试 ====================

func TestOptimizeSchedule(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// 低负载情况
	metrics := &LoadMetrics{
		CPUPercent:    10,
		MemoryPercent: 20,
		DiskIOPercent: 15,
	}

	optimization, err := manager.OptimizeSchedule(metrics)
	require.NoError(t, err)
	assert.NotNil(t, optimization)
	assert.GreaterOrEqual(t, optimization.LoadScore, 80.0)
	assert.Equal(t, 0, optimization.WaitMinutes)
}

func TestOptimizeScheduleHighLoad(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// 高负载情况
	metrics := &LoadMetrics{
		CPUPercent:    90,
		MemoryPercent: 85,
		DiskIOPercent: 80,
	}

	optimization, err := manager.OptimizeSchedule(metrics)
	require.NoError(t, err)
	assert.NotNil(t, optimization)
	assert.Less(t, optimization.LoadScore, 50.0)
	assert.Equal(t, 120, optimization.WaitMinutes)
}

func TestOptimizeScheduleNilMetrics(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	_, err := manager.OptimizeSchedule(nil)
	assert.Error(t, err)
}

// ==================== 统计测试 ====================

func TestGetStats(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// 创建策略
	manager.CreatePolicy(&BackupPolicy{
		Name:        "policy-1",
		SourcePaths: []string{"/data1"},
		TargetIDs:   []string{"target-1", "target-2", "target-3"},
	})

	// 记录执行
	manager.RecordExecution(&BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		Status:     BackupStatusSuccess,
		SizeBytes:  1024 * 1024,
	})

	stats := manager.GetStats()
	assert.Equal(t, 1, stats.TotalPolicies)
	assert.Equal(t, 1, stats.TotalExecutions)
	assert.Equal(t, 1, stats.SuccessfulBackups)
	assert.Equal(t, int64(1024*1024), stats.TotalSizeBytes)
}

// ==================== 执行记录测试 ====================

func TestRecordExecution(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	execution := &BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		Status:     BackupStatusSuccess,
		SizeBytes:  1024 * 1024,
	}

	err := manager.RecordExecution(execution)
	require.NoError(t, err)
	assert.NotEmpty(t, execution.ID)
}

func TestListExecutions(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	manager.RecordExecution(&BackupExecution{
		PolicyID:   "policy-1",
		BackupType: BackupTypeFull,
		Status:     BackupStatusSuccess,
	})
	manager.RecordExecution(&BackupExecution{
		PolicyID:   "policy-2",
		BackupType: BackupTypeIncremental,
		Status:     BackupStatusSuccess,
	})

	all := manager.ListExecutions("")
	assert.Len(t, all, 2)

	filtered := manager.ListExecutions("policy-1")
	assert.Len(t, filtered, 1)
}

func TestEvaluatePolicy(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	policy := &BackupPolicy{
		Name:        "test-policy",
		SourcePaths: []string{"/data"},
		TargetIDs:   []string{"target-1"},
	}
	manager.CreatePolicy(policy)

	manager.RecordExecution(&BackupExecution{
		PolicyID:   policy.ID,
		BackupType: BackupTypeFull,
		Status:     BackupStatusSuccess,
	})
	manager.RecordExecution(&BackupExecution{
		PolicyID:   policy.ID,
		BackupType: BackupTypeFull,
		Status:     BackupStatusFailed,
	})

	evaluation, err := manager.EvaluatePolicy(policy.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, evaluation.TotalExecutions)
	assert.Equal(t, 1, evaluation.FailedExecutions)
	assert.Equal(t, 50.0, evaluation.SuccessRate)
}

func TestOptimizeBackupWindow(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	policy := &BackupPolicy{
		Name:        "test-policy",
		SourcePaths: []string{"/data"},
		TargetIDs:   []string{"target-1"},
	}

	optimization, err := manager.OptimizeBackupWindow(policy, nil)
	require.NoError(t, err)
	assert.NotNil(t, optimization)
	assert.NotEmpty(t, optimization.Reason)
}
