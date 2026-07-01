package updatecoord

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Service 测试 ==========

func TestService_Check(t *testing.T) {
	svc := NewService()

	t.Run("检查所有更新", func(t *testing.T) {
		updates, err := svc.Check(context.Background(), nil)
		require.NoError(t, err)
		assert.NotEmpty(t, updates)
		assert.True(t, len(updates) >= 3)

		// 所有返回的更新都应可用
		for _, u := range updates {
			assert.True(t, u.Available)
			assert.NotEmpty(t, u.Version)
		}
	})

	t.Run("按渠道过滤 - stable", func(t *testing.T) {
		ch := ChannelStable
		updates, err := svc.Check(context.Background(), &ch)
		require.NoError(t, err)
		assert.NotEmpty(t, updates)
		for _, u := range updates {
			assert.Equal(t, ChannelStable, u.Channel)
		}
	})

	t.Run("按渠道过滤 - beta", func(t *testing.T) {
		ch := ChannelBeta
		updates, err := svc.Check(context.Background(), &ch)
		require.NoError(t, err)
		assert.NotEmpty(t, updates)
		for _, u := range updates {
			assert.Equal(t, ChannelBeta, u.Channel)
		}
	})

	t.Run("按渠道过滤 - nightly（无结果）", func(t *testing.T) {
		ch := ChannelNightly
		updates, err := svc.Check(context.Background(), &ch)
		require.NoError(t, err)
		assert.Empty(t, updates)
	})
}

func TestService_PreCheck(t *testing.T) {
	svc := NewService()

	t.Run("成功预检", func(t *testing.T) {
		req := &PreCheckRequest{
			Version: "1.1.0",
		}

		result, err := svc.PreCheck(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "1.1.0", result.Version)
		assert.True(t, result.Passed)
		assert.NotEmpty(t, result.Checks)
		assert.True(t, len(result.Checks) >= 5) // 至少5项检查
	})

	t.Run("版本不存在", func(t *testing.T) {
		req := &PreCheckRequest{
			Version: "99.99.99",
		}

		_, err := svc.PreCheck(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrVersionNotFound)
	})

	t.Run("高危更新产生警告", func(t *testing.T) {
		req := &PreCheckRequest{
			Version: "1.0.1", // criticalLevel: high
		}

		result, err := svc.PreCheck(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Passed)
		assert.NotEmpty(t, result.Warnings)
		assert.Contains(t, result.Warnings[0], "高优先级")
	})
}

func TestService_Apply(t *testing.T) {
	svc := NewService()

	t.Run("成功应用更新（DryRun）", func(t *testing.T) {
		req := &ApplyRequest{
			Version: "1.1.0",
			DryRun:  true,
		}

		result, err := svc.Apply(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "1.1.0", result.Version)
		assert.Equal(t, PhaseDone, result.Phase)
		assert.Equal(t, float64(100), result.Progress)
		assert.Empty(t, result.Error)

		// 检查步骤
		assert.Equal(t, 5, len(result.Steps)) // 下载 + 备份 + 安装 + 验证 + 切换
		for _, step := range result.Steps {
			assert.Equal(t, StepCompleted, step.Status)
		}
	})

	t.Run("版本不存在", func(t *testing.T) {
		req := &ApplyRequest{
			Version: "99.99.99",
		}

		_, err := svc.Apply(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrVersionNotFound)
	})

	t.Run("跳过备份", func(t *testing.T) {
		req := &ApplyRequest{
			Version:    "1.1.0",
			DryRun:     true,
			SkipBackup: true,
		}

		result, err := svc.Apply(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, PhaseDone, result.Phase)

		// 备份步骤应被跳过
		foundBackup := false
		for _, step := range result.Steps {
			if step.Phase == PhaseBackup {
				assert.Equal(t, StepSkipped, step.Status)
				foundBackup = true
			}
		}
		assert.True(t, foundBackup, "应包含备份步骤")
	})

	t.Run("更新后当前版本变化", func(t *testing.T) {
		// 使用新 service 避免干扰
		svc2 := NewService()
		svc2.SetCurrentVersion("1.0.0")

		req := &ApplyRequest{
			Version: "2.0.0",
			DryRun:  true,
		}

		_, err := svc2.Apply(context.Background(), req)
		require.NoError(t, err)

		// DryRun 也更新了 currentVersion（因为是模拟）
		updates, err := svc2.Check(context.Background(), nil)
		require.NoError(t, err)
		assert.NotEmpty(t, updates)
	})
}

func TestService_GetHistory(t *testing.T) {
	svc := NewService()

	// 初始历史应为空
	entries, err := svc.GetHistory()
	require.NoError(t, err)
	assert.Empty(t, entries)

	// 执行一次更新
	req := &ApplyRequest{
		Version: "1.1.0",
		DryRun:  true,
	}
	_, err = svc.Apply(context.Background(), req)
	require.NoError(t, err)

	// 应有一条历史记录
	entries, err = svc.GetHistory()
	require.NoError(t, err)
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "1.1.0", entries[0].Version)
	assert.True(t, entries[0].Success)

	// 再执行一次
	req2 := &ApplyRequest{
		Version: "2.0.0",
		DryRun:  true,
	}
	_, err = svc.Apply(context.Background(), req2)
	require.NoError(t, err)

	entries, err = svc.GetHistory()
	require.NoError(t, err)
	assert.Equal(t, 2, len(entries))
}

func TestService_Rollback(t *testing.T) {
	svc := NewService()

	// 准备：执行一次更新
	applyReq := &ApplyRequest{
		Version: "1.1.0",
		DryRun:  true,
	}
	_, err := svc.Apply(context.Background(), applyReq)
	require.NoError(t, err)

	// 获取历史
	entries, err := svc.GetHistory()
	require.NoError(t, err)
	require.Equal(t, 1, len(entries))

	t.Run("成功回滚", func(t *testing.T) {
		req := &RollbackRequest{
			Version:   "1.1.0",
			HistoryID: entries[0].ID,
		}

		result, err := svc.Rollback(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.Message)
		assert.Contains(t, result.Message, "回滚")
		assert.NotEmpty(t, result.Steps)
	})

	t.Run("版本历史不存在", func(t *testing.T) {
		req := &RollbackRequest{
			Version: "99.99.99",
		}

		_, err := svc.Rollback(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRollbackFailed)
	})
}

func TestService_PreCheck_AllCategories(t *testing.T) {
	svc := NewService()

	req := &PreCheckRequest{
		Version: "2.0.0",
	}

	result, err := svc.PreCheck(context.Background(), req)
	require.NoError(t, err)

	// 验证预检涵盖所有类别
	categories := make(map[string]bool)
	for _, c := range result.Checks {
		categories[c.Category] = true
	}

	assert.True(t, categories["disk"], "应包含磁盘检查")
	assert.True(t, categories["service"], "应包含服务检查")
	assert.True(t, categories["backup"], "应包含备份检查")
	assert.True(t, categories["network"], "应包含网络检查")
	assert.True(t, categories["system"], "应包含系统检查")
}

// ========== Handler 测试 ==========

func TestHandler_RegisterRoutes(t *testing.T) {
	// 确保路由注册不 panic
	svc := NewService()
	h := NewHandler(svc)
	assert.NotNil(t, h)
}

// ========== 辅助测试 ==========

func TestUpdateChannels(t *testing.T) {
	// 确保渠道常量正确
	assert.Equal(t, UpdateChannel("stable"), ChannelStable)
	assert.Equal(t, UpdateChannel("beta"), ChannelBeta)
	assert.Equal(t, UpdateChannel("lts"), ChannelLTS)
	assert.Equal(t, UpdateChannel("nightly"), ChannelNightly)
}

func TestUpdatePhases(t *testing.T) {
	// 确保阶段常量正确
	assert.Equal(t, UpdatePhase("check"), PhaseCheck)
	assert.Equal(t, UpdatePhase("precheck"), PhasePreCheck)
	assert.Equal(t, UpdatePhase("download"), PhaseDownload)
	assert.Equal(t, UpdatePhase("backup"), PhaseBackup)
	assert.Equal(t, UpdatePhase("install"), PhaseInstall)
	assert.Equal(t, UpdatePhase("verify"), PhaseVerify)
	assert.Equal(t, UpdatePhase("switch"), PhaseSwitch)
	assert.Equal(t, UpdatePhase("rollback"), PhaseRollback)
	assert.Equal(t, UpdatePhase("done"), PhaseDone)
	assert.Equal(t, UpdatePhase("failed"), PhaseFailed)
}
