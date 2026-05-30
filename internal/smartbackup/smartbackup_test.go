package smartbackup

import (
	"testing"

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
}

func TestCreatePolicy(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	policy := &BackupPolicy{
		Name:        "test-policy",
		BackupType:  BackupTypeFull,
		SourcePaths: []string{"/data"},
		TargetPaths:   []string{"target-1"},
	}

	err := manager.CreatePolicy(policy)
	require.NoError(t, err)
	assert.NotEmpty(t, policy.ID)
	assert.False(t, policy.CreatedAt.IsZero())
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
		TargetPaths:   []string{"target-1"},
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
		TargetPaths:   []string{"target-1"},
	})
	manager.CreatePolicy(&BackupPolicy{
		Name:        "policy-2",
		SourcePaths: []string{"/data2"},
		TargetPaths:   []string{"target-2"},
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
		TargetPaths:   []string{"target-1"},
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
		TargetPaths:   []string{"target-1"},
	}
	manager.CreatePolicy(policy)

	err := manager.DeletePolicy(policy.ID)
	require.NoError(t, err)

	_, err = manager.GetPolicy(policy.ID)
	assert.Error(t, err)
}

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
	}

	strategy, err := manager.AnalyzeStrategy(analysis)
	require.NoError(t, err)
	assert.NotNil(t, strategy)
	assert.NotEmpty(t, strategy.RecommendedType)
	assert.NotEmpty(t, strategy.Reason)
}

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

	// 列出所有
	all := manager.ListExecutions("")
	assert.Len(t, all, 2)

	// 按策略过滤
	filtered := manager.ListExecutions("policy-1")
	assert.Len(t, filtered, 1)
}

func TestEvaluatePolicy(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	policy := &BackupPolicy{
		Name:        "test-policy",
		SourcePaths: []string{"/data"},
		TargetPaths:   []string{"target-1"},
	}
	manager.CreatePolicy(policy)

	// 记录一些执行
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
		TargetPaths:   []string{"target-1"},
	}

	optimization, err := manager.OptimizeBackupWindow(policy, nil)
	require.NoError(t, err)
	assert.NotNil(t, optimization)
	assert.NotEmpty(t, optimization.Reason)
}
