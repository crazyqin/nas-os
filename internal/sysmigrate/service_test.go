package sysmigrate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Service 测试 ==========

func TestService_Assess(t *testing.T) {
	svc := NewService()

	t.Run("成功评估", func(t *testing.T) {
		req := &AssessRequest{
			SourceType: SourceSynology,
			SourceHost: "192.168.1.100",
			SourcePort: 22,
			SourceUser: "admin",
			TargetPath: "/data/migration",
		}

		result, err := svc.Assess(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.TaskID)
		assert.True(t, result.Compatible)
		assert.NotNil(t, result.SourceInfo)
		assert.Equal(t, SourceSynology, result.SourceInfo.Type)
		assert.Equal(t, "192.168.1.100", result.SourceInfo.Hostname)
	})

	t.Run("空主机地址", func(t *testing.T) {
		req := &AssessRequest{
			SourceType: SourceSynology,
			SourceHost: "",
			SourceUser: "admin",
			TargetPath: "/data/migration",
		}

		_, err := svc.Assess(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "源主机地址不能为空")
	})

	t.Run("空用户名", func(t *testing.T) {
		req := &AssessRequest{
			SourceType: SourceSynology,
			SourceHost: "192.168.1.100",
			SourceUser: "",
			TargetPath: "/data/migration",
		}

		_, err := svc.Assess(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "源系统用户名不能为空")
	})

	t.Run("空目标路径", func(t *testing.T) {
		req := &AssessRequest{
			SourceType: SourceSynology,
			SourceHost: "192.168.1.100",
			SourceUser: "admin",
			TargetPath: "",
		}

		_, err := svc.Assess(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "目标路径不能为空")
	})

	t.Run("通用源类型产生警告", func(t *testing.T) {
		req := &AssessRequest{
			SourceType: SourceGeneric,
			SourceHost: "10.0.0.5",
			SourceUser: "root",
			TargetPath: "/data/migration",
		}

		result, err := svc.Assess(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Compatible)
		assert.NotEmpty(t, result.Warnings)
	})
}

func TestService_Plan(t *testing.T) {
	svc := NewService()

	// 先创建评估
	assessReq := &AssessRequest{
		SourceType: SourceQNAP,
		SourceHost: "192.168.1.200",
		SourceUser: "admin",
		TargetPath: "/data/migration",
	}
	assessResult, err := svc.Assess(context.Background(), assessReq)
	require.NoError(t, err)
	taskID := assessResult.TaskID

	t.Run("成功生成计划", func(t *testing.T) {
		req := &PlanRequest{
			TaskID:     taskID,
			Categories: []MigrationCategory{CategoryData, CategoryUsers, CategoryShared},
		}

		result, err := svc.Plan(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, taskID, result.TaskID)
		assert.NotEmpty(t, result.Steps)
		// 3 个类别 + 1 个备份 + 1 个验证 = 5 个步骤
		assert.Equal(t, 5, result.TotalSteps)
		assert.NotEmpty(t, result.Timeline)
	})

	t.Run("任务不存在", func(t *testing.T) {
		req := &PlanRequest{
			TaskID:     "non-existent",
			Categories: []MigrationCategory{CategoryData},
		}

		_, err := svc.Plan(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTaskNotFound)
	})

	t.Run("空类别列表", func(t *testing.T) {
		req := &PlanRequest{
			TaskID:     taskID,
			Categories: []MigrationCategory{},
		}

		// 绑定验证应在 handler 层拦截，这里测试 service 层行为
		// service 层不校验类别为空，但生成的步骤仅含备份和验证
		result, err := svc.Plan(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalSteps) // 仅备份 + 验证
	})
}

func TestService_Execute(t *testing.T) {
	svc := NewService()

	// 准备：评估 + 计划
	assessReq := &AssessRequest{
		SourceType: SourceTrueNAS,
		SourceHost: "192.168.1.50",
		SourceUser: "admin",
		TargetPath: "/data/migration",
	}
	assessResult, err := svc.Assess(context.Background(), assessReq)
	require.NoError(t, err)
	taskID := assessResult.TaskID

	planReq := &PlanRequest{
		TaskID:     taskID,
		Categories: []MigrationCategory{CategoryData, CategoryConfig},
	}
	_, err = svc.Plan(context.Background(), planReq)
	require.NoError(t, err)

	t.Run("成功执行（DryRun）", func(t *testing.T) {
		req := &ExecuteRequest{
			TaskID: taskID,
			DryRun: true,
		}

		result, err := svc.Execute(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, PhaseDone, result.Phase)
		assert.Equal(t, float64(100), result.Progress)
		assert.Empty(t, result.Error)

		// 所有步骤应为 completed
		for _, step := range result.Steps {
			assert.Equal(t, StepCompleted, step.Status)
		}
	})

	t.Run("任务不存在", func(t *testing.T) {
		req := &ExecuteRequest{
			TaskID: "non-existent",
		}

		_, err := svc.Execute(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTaskNotFound)
	})

	t.Run("跳过指定步骤", func(t *testing.T) {
		// 重新创建一个任务
		assessReq2 := &AssessRequest{
			SourceType: SourceSynology,
			SourceHost: "192.168.1.60",
			SourceUser: "admin",
			TargetPath: "/data/migration",
		}
		assessResult2, err := svc.Assess(context.Background(), assessReq2)
		require.NoError(t, err)
		taskID2 := assessResult2.TaskID

		planReq2 := &PlanRequest{
			TaskID:     taskID2,
			Categories: []MigrationCategory{CategoryData, CategoryUsers},
		}
		planResult2, err := svc.Plan(context.Background(), planReq2)
		require.NoError(t, err)

		// 跳过第一个数据步骤
		skipID := planResult2.Steps[1].ID
		req := &ExecuteRequest{
			TaskID:    taskID2,
			DryRun:    true,
			SkipSteps: []string{skipID},
		}

		result, err := svc.Execute(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, PhaseDone, result.Phase)

		// 被跳过的步骤应为 skipped
		found := false
		for _, step := range result.Steps {
			if step.ID == skipID {
				assert.Equal(t, StepSkipped, step.Status)
				found = true
			}
		}
		assert.True(t, found, "应找到被跳过的步骤")
	})
}

func TestService_Rollback(t *testing.T) {
	svc := NewService()

	// 准备：评估 + 计划 + 执行
	assessReq := &AssessRequest{
		SourceType: SourceUnraid,
		SourceHost: "192.168.1.70",
		SourceUser: "admin",
		TargetPath: "/data/migration",
	}
	assessResult, err := svc.Assess(context.Background(), assessReq)
	require.NoError(t, err)
	taskID := assessResult.TaskID

	planReq := &PlanRequest{
		TaskID:     taskID,
		Categories: []MigrationCategory{CategoryData, CategoryConfig, CategoryUsers},
	}
	_, err = svc.Plan(context.Background(), planReq)
	require.NoError(t, err)

	execReq := &ExecuteRequest{
		TaskID: taskID,
		DryRun: true,
	}
	_, err = svc.Execute(context.Background(), execReq)
	require.NoError(t, err)

	t.Run("成功回滚", func(t *testing.T) {
		req := &RollbackRequest{
			TaskID: taskID,
		}

		result, err := svc.Rollback(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.Message)
		assert.Contains(t, result.Message, "回滚")
	})

	t.Run("任务不存在", func(t *testing.T) {
		req := &RollbackRequest{
			TaskID: "non-existent",
		}

		_, err := svc.Rollback(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTaskNotFound)
	})
}

func TestService_GetStatus(t *testing.T) {
	svc := NewService()

	// 准备：评估
	assessReq := &AssessRequest{
		SourceType: SourceSynology,
		SourceHost: "192.168.1.80",
		SourceUser: "admin",
		TargetPath: "/data/migration",
	}
	assessResult, err := svc.Assess(context.Background(), assessReq)
	require.NoError(t, err)
	taskID := assessResult.TaskID

	t.Run("获取评估后状态", func(t *testing.T) {
		status, err := svc.GetStatus(taskID)
		require.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, taskID, status.TaskID)
		assert.Equal(t, PhaseAssess, status.Phase)
		assert.Equal(t, float64(0), status.Progress)
	})

	t.Run("获取不存在的任务状态", func(t *testing.T) {
		_, err := svc.GetStatus("non-existent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTaskNotFound)
	})

	t.Run("获取执行后状态", func(t *testing.T) {
		// 计划 + 执行
		planReq := &PlanRequest{
			TaskID:     taskID,
			Categories: []MigrationCategory{CategoryData},
		}
		_, err := svc.Plan(context.Background(), planReq)
		require.NoError(t, err)

		execReq := &ExecuteRequest{
			TaskID: taskID,
			DryRun: true,
		}
		_, err = svc.Execute(context.Background(), execReq)
		require.NoError(t, err)

		status, err := svc.GetStatus(taskID)
		require.NoError(t, err)
		assert.Equal(t, PhaseDone, status.Phase)
		assert.Equal(t, float64(100), status.Progress)
		assert.NotEmpty(t, status.Steps)
	})
}

// ========== Handler 测试 ==========

func TestHandler_RegisterRoutes(t *testing.T) {
	// 确保路由注册不 panic
	svc := NewService()
	h := NewHandler(svc)
	assert.NotNil(t, h)

	// 确保 RegisterRoutes 方法存在且可调用
	// 在实际测试中需要 gin 上下文，这里仅验证构造
}

// ========== 辅助函数测试 ==========

func TestEstimateDuration(t *testing.T) {
	tests := []struct {
		name      string
		dataSize  int64
		userCount int
		want      string
	}{
		{"空数据", 0, 0, "1-2 小时"},
		{"小数据", 5 * 1024 * 1024 * 1024, 5, "1-2 小时"},
		{"中等数据", 50 * 1024 * 1024 * 1024, 20, "2-4 小时"},
		{"大数据", 200 * 1024 * 1024 * 1024, 50, "4-12 小时"},
		{"超大数据", 600 * 1024 * 1024 * 1024, 100, "12+ 小时"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateDuration(tt.dataSize, tt.userCount)
			assert.Equal(t, tt.want, got)
		})
	}
}
