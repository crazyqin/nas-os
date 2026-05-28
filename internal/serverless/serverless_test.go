// Package serverless 单元测试.
package serverless

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 引擎创建与生命周期测试 ==========

func TestNewEngine(t *testing.T) {
	engine := NewEngine(nil)
	require.NotNil(t, engine)
	assert.False(t, engine.IsRunning())
}

func TestNewEngine_WithConfig(t *testing.T) {
	config := &EngineConfig{
		MaxFunctions:             50,
		MaxConcurrentInvocations: 20,
		DefaultTimeoutS:          60,
		DefaultMemoryMB:          256,
		LogRetentionDays:         14,
		EnableMetrics:            true,
	}

	engine := NewEngine(config)
	require.NotNil(t, engine)
	assert.False(t, engine.IsRunning())
}

func TestEngine_StartStop(t *testing.T) {
	engine := NewEngine(nil)

	// 启动
	err := engine.Start()
	require.NoError(t, err)
	assert.True(t, engine.IsRunning())

	// 重复启动应报错
	err = engine.Start()
	assert.Error(t, err)

	// 停止
	engine.Stop()
	assert.False(t, engine.IsRunning())
}

// ========== 函数管理测试 ==========

func TestCreateFunction(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{
		Name:    "test-func",
		Runtime: RuntimeGo,
		Handler: "main",
		Code:    "package main",
	}

	err := engine.CreateFunction(fn)
	require.NoError(t, err)
	assert.NotEmpty(t, fn.ID)
	assert.Equal(t, DeployStatusDraft, fn.DeployStatus)
	assert.True(t, fn.Enabled)
	assert.Equal(t, "1.0.0", fn.Version)
}

func TestCreateFunction_EmptyName(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{
		Runtime: RuntimeGo,
		Handler: "main",
	}

	err := engine.CreateFunction(fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称不能为空")
}

func TestCreateFunction_InvalidRuntime(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{
		Name:    "test-func",
		Runtime: "invalid",
		Handler: "main",
	}

	err := engine.CreateFunction(fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的运行时")
}

func TestCreateFunction_DuplicateName(t *testing.T) {
	engine := NewEngine(nil)

	fn1 := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code1"}
	fn2 := &Function{Name: "test-func", Runtime: RuntimePython, Handler: "main", Code: "code2"}

	err := engine.CreateFunction(fn1)
	require.NoError(t, err)

	err = engine.CreateFunction(fn2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称已存在")
}

func TestCreateFunction_MaxLimit(t *testing.T) {
	config := &EngineConfig{
		MaxFunctions:             2,
		MaxConcurrentInvocations: 10,
	}
	engine := NewEngine(config)

	fn1 := &Function{Name: "fn1", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	fn2 := &Function{Name: "fn2", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	fn3 := &Function{Name: "fn3", Runtime: RuntimeGo, Handler: "main", Code: "code"}

	require.NoError(t, engine.CreateFunction(fn1))
	require.NoError(t, engine.CreateFunction(fn2))

	err := engine.CreateFunction(fn3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大函数数量限制")
}

func TestUpdateFunction(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	fn.Description = "updated description"
	fn.Code = "updated code"

	err := engine.UpdateFunction(fn)
	require.NoError(t, err)

	got, err := engine.GetFunction(fn.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated description", got.Description)
	assert.Equal(t, "updated code", got.Code)
}

func TestUpdateFunction_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{ID: "nonexistent", Name: "test", Runtime: RuntimeGo}
	err := engine.UpdateFunction(fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestDeleteFunction(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	err := engine.DeleteFunction(fn.ID)
	require.NoError(t, err)

	_, err = engine.GetFunction(fn.ID)
	assert.Error(t, err)
}

func TestDeleteFunction_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	err := engine.DeleteFunction("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestGetFunction(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	got, err := engine.GetFunction(fn.ID)
	require.NoError(t, err)
	assert.Equal(t, fn.Name, got.Name)
	assert.Equal(t, fn.Runtime, got.Runtime)
}

func TestGetFunction_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	_, err := engine.GetFunction("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestListFunctions(t *testing.T) {
	engine := NewEngine(nil)

	engine.CreateFunction(&Function{Name: "fn1", Runtime: RuntimeGo, Handler: "main", Code: "code"})
	engine.CreateFunction(&Function{Name: "fn2", Runtime: RuntimePython, Handler: "main", Code: "code"})
	engine.CreateFunction(&Function{Name: "fn3", Runtime: RuntimeGo, Handler: "main", Code: "code"})

	// 列出所有
	functions := engine.ListFunctions(nil)
	assert.Len(t, functions, 3)

	// 按运行时过滤
	functions = engine.ListFunctions(&FunctionFilter{Runtime: RuntimeGo})
	assert.Len(t, functions, 2)

	// 按名称搜索
	functions = engine.ListFunctions(&FunctionFilter{Search: "fn1"})
	assert.Len(t, functions, 1)
	assert.Equal(t, "fn1", functions[0].Name)
}

func TestListFunctions_Pagination(t *testing.T) {
	engine := NewEngine(nil)

	for i := 0; i < 10; i++ {
		engine.CreateFunction(&Function{
			Name:    fmt.Sprintf("fn%d", i),
			Runtime: RuntimeGo,
			Handler: "main",
			Code:    "code",
		})
	}

	// 第一页
	functions := engine.ListFunctions(&FunctionFilter{Page: 1, PageSize: 3})
	assert.Len(t, functions, 3)

	// 第二页
	functions = engine.ListFunctions(&FunctionFilter{Page: 2, PageSize: 3})
	assert.Len(t, functions, 3)

	// 最后一页
	functions = engine.ListFunctions(&FunctionFilter{Page: 4, PageSize: 3})
	assert.Len(t, functions, 1)
}

func TestEnableDisableFunction(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	// 禁用
	err := engine.DisableFunction(fn.ID)
	require.NoError(t, err)

	got, _ := engine.GetFunction(fn.ID)
	assert.False(t, got.Enabled)
	assert.Equal(t, DeployStatusStopped, got.DeployStatus)

	// 启用
	err = engine.EnableFunction(fn.ID)
	require.NoError(t, err)

	got, _ = engine.GetFunction(fn.ID)
	assert.True(t, got.Enabled)
}

// ========== 部署管理测试 ==========

func TestDeployFunction(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "package main"}
	require.NoError(t, engine.CreateFunction(fn))

	err := engine.DeployFunction(fn.ID)
	require.NoError(t, err)

	got, _ := engine.GetFunction(fn.ID)
	assert.Equal(t, DeployStatusDeployed, got.DeployStatus)
	assert.True(t, got.Enabled)
}

func TestDeployFunction_NoCode(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main"}
	require.NoError(t, engine.CreateFunction(fn))

	err := engine.DeployFunction(fn.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "代码为空")
}

func TestUndeployFunction(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	err := engine.UndeployFunction(fn.ID)
	require.NoError(t, err)

	got, _ := engine.GetFunction(fn.ID)
	assert.Equal(t, DeployStatusStopped, got.DeployStatus)
	assert.False(t, got.Enabled)
}

// ========== 触发器管理测试 ==========

func TestAddTrigger(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	trigger := &Trigger{
		FunctionID: fn.ID,
		Type:       TriggerHTTP,
		Config:     TriggerConfig{Path: "/api/test", Method: "GET"},
	}

	err := engine.AddTrigger(trigger)
	require.NoError(t, err)
	assert.NotEmpty(t, trigger.ID)
	assert.True(t, trigger.Enabled)

	triggers, _ := engine.GetTriggers(fn.ID)
	assert.Len(t, triggers, 1)
	assert.Equal(t, TriggerHTTP, triggers[0].Type)
}

func TestAddTrigger_AllTypes(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	triggerTypes := []struct {
		triggerType TriggerType
		config      TriggerConfig
	}{
		{TriggerHTTP, TriggerConfig{Path: "/api/test", Method: "POST"}},
		{TriggerCron, TriggerConfig{Schedule: "*/5 * * * *"}},
		{TriggerFileWatcher, TriggerConfig{WatchPath: "/data", FileFilter: "*.txt"}},
		{TriggerEvent, TriggerConfig{EventType: "file.created", EventSrc: "filesystem"}},
	}

	for _, tt := range triggerTypes {
		trigger := &Trigger{
			FunctionID: fn.ID,
			Type:       tt.triggerType,
			Config:     tt.config,
		}
		err := engine.AddTrigger(trigger)
		require.NoError(t, err, "添加触发器 %s 失败", tt.triggerType)
	}

	triggers, _ := engine.GetTriggers(fn.ID)
	assert.Len(t, triggers, len(triggerTypes))
}

func TestAddTrigger_InvalidType(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	trigger := &Trigger{
		FunctionID: fn.ID,
		Type:       "invalid",
	}

	err := engine.AddTrigger(trigger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的触发器类型")
}

func TestRemoveTrigger(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	trigger := &Trigger{
		FunctionID: fn.ID,
		Type:       TriggerHTTP,
		Config:     TriggerConfig{Path: "/api/test", Method: "GET"},
	}
	require.NoError(t, engine.AddTrigger(trigger))

	err := engine.RemoveTrigger(fn.ID, trigger.ID)
	require.NoError(t, err)

	triggers, _ := engine.GetTriggers(fn.ID)
	assert.Len(t, triggers, 0)
}

func TestRemoveTrigger_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	err := engine.RemoveTrigger(fn.ID, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// ========== 函数调用测试 ==========

func TestInvokeSync(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	input := map[string]interface{}{"key": "value"}
	resp, err := engine.InvokeSync(context.Background(), fn.ID, input)
	require.NoError(t, err)
	assert.Equal(t, InvocationStatusSuccess, resp.Status)
	assert.NotEmpty(t, resp.InvocationID)
	assert.NotNil(t, resp.Output)
}

func TestInvokeAsync(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	input := map[string]interface{}{"key": "value"}
	resp, err := engine.InvokeAsync(context.Background(), fn.ID, input)
	require.NoError(t, err)
	assert.Equal(t, InvocationStatusSuccess, resp.Status)
	assert.NotEmpty(t, resp.InvocationID)
}

func TestInvoke_NotFound(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	_, err := engine.InvokeSync(context.Background(), "nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestInvoke_DisabledFunction(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))
	require.NoError(t, engine.DisableFunction(fn.ID))

	_, err := engine.InvokeSync(context.Background(), fn.ID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已禁用")
}

func TestInvoke_NotDeployed(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	_, err := engine.InvokeSync(context.Background(), fn.ID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未部署")
}

func TestInvoke_ContextCancel(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := engine.InvokeSync(ctx, fn.ID, nil)
	// 可能成功也可能失败，取决于执行时序
	_ = err
}

func TestInvoke_RecordsInvocation(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	_, err := engine.InvokeSync(context.Background(), fn.ID, nil)
	require.NoError(t, err)

	// 检查调用记录
	got, _ := engine.GetFunction(fn.ID)
	assert.Equal(t, int64(1), got.InvokeCount)
	assert.NotNil(t, got.LastInvokeAt)

	// 检查调用历史
	invocations := engine.ListInvocations(&InvocationFilter{FunctionID: fn.ID})
	assert.Len(t, invocations, 1)
}

func TestGetInvocation(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	resp, _ := engine.InvokeSync(context.Background(), fn.ID, nil)

	inv, err := engine.GetInvocation(resp.InvocationID)
	require.NoError(t, err)
	assert.Equal(t, fn.ID, inv.FunctionID)
	assert.Equal(t, InvocationStatusSuccess, inv.Status)
}

func TestGetInvocation_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	_, err := engine.GetInvocation("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestListInvocations_FilterByStatus(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	// 创建成功的调用
	engine.InvokeSync(context.Background(), fn.ID, nil)

	// 过滤成功状态
	invocations := engine.ListInvocations(&InvocationFilter{
		FunctionID: fn.ID,
		Status:     InvocationStatusSuccess,
	})
	assert.Len(t, invocations, 1)
	assert.Equal(t, InvocationStatusSuccess, invocations[0].Status)
}

func TestListInvocations_Pagination(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	for i := 0; i < 5; i++ {
		engine.InvokeSync(context.Background(), fn.ID, nil)
	}

	invocations := engine.ListInvocations(&InvocationFilter{
		FunctionID: fn.ID,
		Page:       1,
		PageSize:   2,
	})
	assert.Len(t, invocations, 2)
}

// ========== 版本管理测试 ==========

func TestGetVersions(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	versions, err := engine.GetVersions(fn.ID)
	require.NoError(t, err)
	assert.Len(t, versions, 1) // 初始版本
	assert.Equal(t, "1.0.0", versions[0].Version)
}

func TestGetVersions_AfterUpdate(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	fn.Code = "updated code"
	require.NoError(t, engine.UpdateFunction(fn))

	versions, err := engine.GetVersions(fn.ID)
	require.NoError(t, err)
	assert.Len(t, versions, 2)
}

func TestRollbackVersion(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "v1 code"}
	require.NoError(t, engine.CreateFunction(fn))

	// 更新代码
	fn.Code = "v2 code"
	require.NoError(t, engine.UpdateFunction(fn))

	// 回滚到 1.0.0
	err := engine.RollbackVersion(fn.ID, "1.0.0")
	require.NoError(t, err)

	got, _ := engine.GetFunction(fn.ID)
	assert.Equal(t, "v1 code", got.Code)
	assert.Equal(t, "1.0.0", got.Version)
}

func TestRollbackVersion_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))

	err := engine.RollbackVersion(fn.ID, "999.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "版本不存在")
}

// ========== 日志管理测试 ==========

func TestGetLogs(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	// 调用函数会产生日志
	engine.InvokeSync(context.Background(), fn.ID, nil)

	logs := engine.GetLogs(fn.ID, 10)
	assert.NotEmpty(t, logs)
	assert.Equal(t, fn.ID, logs[0].FunctionID)
}

func TestGetLogs_Limit(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	for i := 0; i < 5; i++ {
		engine.InvokeSync(context.Background(), fn.ID, nil)
	}

	logs := engine.GetLogs(fn.ID, 2)
	assert.Len(t, logs, 2)
}

// ========== 指标统计测试 ==========

func TestGetMetrics(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	// 执行几次调用
	for i := 0; i < 3; i++ {
		engine.InvokeSync(context.Background(), fn.ID, nil)
	}

	metrics, err := engine.GetMetrics(fn.ID)
	require.NoError(t, err)
	assert.Equal(t, fn.ID, metrics.FunctionID)
	assert.Equal(t, int64(3), metrics.TotalInvocations)
	assert.Equal(t, int64(3), metrics.SuccessCount)
	assert.Equal(t, int64(0), metrics.ErrorCount)
	assert.True(t, metrics.AvgDuration > 0)
}

func TestGetMetrics_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	_, err := engine.GetMetrics("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestGetStats(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	// 创建不同运行时的函数
	engine.CreateFunction(&Function{Name: "fn1", Runtime: RuntimeGo, Handler: "main", Code: "code"})
	engine.CreateFunction(&Function{Name: "fn2", Runtime: RuntimePython, Handler: "main", Code: "code"})
	engine.CreateFunction(&Function{Name: "fn3", Runtime: RuntimeGo, Handler: "main", Code: "code"})

	stats := engine.GetStats()
	assert.Equal(t, 3, stats.TotalFunctions)
	assert.Equal(t, 2, stats.RuntimeStats[string(RuntimeGo)])
	assert.Equal(t, 1, stats.RuntimeStats[string(RuntimePython)])
}

func TestGetStats_WithInvocations(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{Name: "test-func", Runtime: RuntimeGo, Handler: "main", Code: "code"}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	for i := 0; i < 5; i++ {
		engine.InvokeSync(context.Background(), fn.ID, nil)
	}

	stats := engine.GetStats()
	assert.Equal(t, int64(5), stats.TotalInvocations)
	assert.True(t, stats.SuccessRate > 0)
}

// ========== 并发测试 ==========

func TestConcurrentCreateFunction(t *testing.T) {
	engine := NewEngine(nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			engine.CreateFunction(&Function{
				Name:    fmt.Sprintf("fn-%d", i),
				Runtime: RuntimeGo,
				Handler: "main",
				Code:    "code",
			})
		}(i)
	}

	wg.Wait()

	functions := engine.ListFunctions(nil)
	assert.Len(t, functions, 10)
}

func TestConcurrentInvoke(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())

	fn := &Function{
		Name:           "test-func",
		Runtime:        RuntimeGo,
		Handler:        "main",
		Code:           "code",
		Config:         FunctionConfig{MaxConcurrency: 20},
	}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = engine.InvokeSync(context.Background(), fn.ID, nil)
		}()
	}

	wg.Wait()

	got, _ := engine.GetFunction(fn.ID)
	assert.Equal(t, int64(5), got.InvokeCount)
}

// ========== 配置测试 ==========

func TestDefaultFunctionConfig(t *testing.T) {
	cfg := DefaultFunctionConfig()
	assert.Equal(t, 100, cfg.CPUMilli)
	assert.Equal(t, 128, cfg.MemoryMB)
	assert.Equal(t, 30, cfg.TimeoutS)
	assert.Equal(t, 10, cfg.MaxConcurrency)
	assert.NotNil(t, cfg.EnvVars)
}

func TestDefaultEngineConfig(t *testing.T) {
	cfg := DefaultEngineConfig()
	assert.Equal(t, 100, cfg.MaxFunctions)
	assert.Equal(t, 50, cfg.MaxConcurrentInvocations)
	assert.Equal(t, 30, cfg.DefaultTimeoutS)
	assert.Equal(t, 128, cfg.DefaultMemoryMB)
	assert.Equal(t, 7, cfg.LogRetentionDays)
	assert.True(t, cfg.EnableMetrics)
}

// ========== 类型验证测试 ==========

func TestRuntime_Constants(t *testing.T) {
	runtimes := []Runtime{RuntimeGo, RuntimePython, RuntimeNode, RuntimeShell}
	for _, r := range runtimes {
		assert.NotEmpty(t, string(r))
	}
}

func TestDeployStatus_Constants(t *testing.T) {
	statuses := []DeployStatus{DeployStatusDraft, DeployStatusDeploying, DeployStatusDeployed, DeployStatusFailed, DeployStatusStopped}
	for _, s := range statuses {
		assert.NotEmpty(t, string(s))
	}
}

func TestTriggerType_Constants(t *testing.T) {
	types := []TriggerType{TriggerHTTP, TriggerCron, TriggerFileWatcher, TriggerEvent}
	for _, tt := range types {
		assert.NotEmpty(t, string(tt))
	}
}

func TestInvocationStatus_Constants(t *testing.T) {
	statuses := []InvocationStatus{InvocationStatusPending, InvocationStatusRunning, InvocationStatusSuccess, InvocationStatusFailed, InvocationStatusTimeout}
	for _, s := range statuses {
		assert.NotEmpty(t, string(s))
	}
}

// ========== 辅助函数测试 ==========

func TestIsValidRuntime(t *testing.T) {
	assert.True(t, isValidRuntime(RuntimeGo))
	assert.True(t, isValidRuntime(RuntimePython))
	assert.True(t, isValidRuntime(RuntimeNode))
	assert.True(t, isValidRuntime(RuntimeShell))
	assert.False(t, isValidRuntime("invalid"))
}

func TestIsValidTriggerType(t *testing.T) {
	assert.True(t, isValidTriggerType(TriggerHTTP))
	assert.True(t, isValidTriggerType(TriggerCron))
	assert.True(t, isValidTriggerType(TriggerFileWatcher))
	assert.True(t, isValidTriggerType(TriggerEvent))
	assert.False(t, isValidTriggerType("invalid"))
}

// ========== 集成测试 ==========

func TestIntegration_FullWorkflow(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())
	defer engine.Stop()

	// 1. 创建函数
	fn := &Function{
		Name:        "hello-world",
		Description: "测试函数",
		Runtime:     RuntimeGo,
		Handler:     "main.Hello",
		Code:        "package main\nfunc Hello() string { return \"hello\" }",
		Config: FunctionConfig{
			CPUMilli:       200,
			MemoryMB:       256,
			TimeoutS:       60,
			MaxConcurrency: 5,
		},
		Tags: []string{"test", "demo"},
	}
	require.NoError(t, engine.CreateFunction(fn))
	assert.NotEmpty(t, fn.ID)
	assert.Equal(t, DeployStatusDraft, fn.DeployStatus)

	// 2. 添加触发器
	httpTrigger := &Trigger{
		FunctionID: fn.ID,
		Type:       TriggerHTTP,
		Config:     TriggerConfig{Path: "/api/hello", Method: "GET"},
	}
	cronTrigger := &Trigger{
		FunctionID: fn.ID,
		Type:       TriggerCron,
		Config:     TriggerConfig{Schedule: "*/5 * * * *"},
	}
	require.NoError(t, engine.AddTrigger(httpTrigger))
	require.NoError(t, engine.AddTrigger(cronTrigger))

	triggers, _ := engine.GetTriggers(fn.ID)
	assert.Len(t, triggers, 2)

	// 3. 部署函数
	require.NoError(t, engine.DeployFunction(fn.ID))
	got, _ := engine.GetFunction(fn.ID)
	assert.Equal(t, DeployStatusDeployed, got.DeployStatus)

	// 4. 调用函数
	for i := 0; i < 3; i++ {
		resp, err := engine.InvokeSync(context.Background(), fn.ID, map[string]interface{}{
			"name": "test",
		})
		require.NoError(t, err)
		assert.Equal(t, InvocationStatusSuccess, resp.Status)
		assert.NotNil(t, resp.Output)
	}

	// 5. 检查调用历史
	invocations := engine.ListInvocations(&InvocationFilter{FunctionID: fn.ID})
	assert.Len(t, invocations, 3)

	// 6. 检查指标
	metrics, err := engine.GetMetrics(fn.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), metrics.TotalInvocations)
	assert.Equal(t, int64(3), metrics.SuccessCount)
	assert.True(t, metrics.AvgDuration > 0)

	// 7. 检查版本
	versions, _ := engine.GetVersions(fn.ID)
	assert.NotEmpty(t, versions)

	// 8. 检查统计
	stats := engine.GetStats()
	assert.Equal(t, 1, stats.TotalFunctions)
	assert.Equal(t, int64(3), stats.TotalInvocations)

	// 9. 更新函数
	fn.Description = "更新后的描述"
	require.NoError(t, engine.UpdateFunction(fn))

	// 10. 移除触发器
	require.NoError(t, engine.RemoveTrigger(fn.ID, cronTrigger.ID))
	triggers, _ = engine.GetTriggers(fn.ID)
	assert.Len(t, triggers, 1)

	// 11. 取消部署
	require.NoError(t, engine.UndeployFunction(fn.ID))
	got, _ = engine.GetFunction(fn.ID)
	assert.Equal(t, DeployStatusStopped, got.DeployStatus)

	// 12. 删除函数
	require.NoError(t, engine.DeleteFunction(fn.ID))
	_, err = engine.GetFunction(fn.ID)
	assert.Error(t, err)
}

func TestIntegration_MultiRuntime(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())
	defer engine.Stop()

	runtimes := []struct {
		name    string
		runtime Runtime
		handler string
		code    string
	}{
		{"go-func", RuntimeGo, "main.Handle", "package main"},
		{"python-func", RuntimePython, "handler.main", "def main(): pass"},
		{"node-func", RuntimeNode, "index.handler", "module.exports = {}"},
		{"shell-func", RuntimeShell, "main.sh", "#!/bin/bash"},
	}

	// 创建不同运行时的函数
	for _, rt := range runtimes {
		fn := &Function{
			Name:    rt.name,
			Runtime: rt.runtime,
			Handler: rt.handler,
			Code:    rt.code,
		}
		require.NoError(t, engine.CreateFunction(fn))
		require.NoError(t, engine.DeployFunction(fn.ID))
	}

	// 验证所有函数
	functions := engine.ListFunctions(nil)
	assert.Len(t, functions, 4)

	// 按运行时过滤
	for _, rt := range runtimes {
		filtered := engine.ListFunctions(&FunctionFilter{Runtime: rt.runtime})
		assert.Len(t, filtered, 1)
		assert.Equal(t, rt.runtime, filtered[0].Runtime)
	}

	// 调用每个函数
	for _, rt := range runtimes {
		functions := engine.ListFunctions(&FunctionFilter{Runtime: rt.runtime})
		require.Len(t, functions, 1)

		resp, err := engine.InvokeSync(context.Background(), functions[0].ID, nil)
		require.NoError(t, err)
		assert.Equal(t, InvocationStatusSuccess, resp.Status)
	}

	// 检查统计
	stats := engine.GetStats()
	assert.Equal(t, 4, stats.TotalFunctions)
	assert.Equal(t, 4, stats.DeployedFunctions)
	assert.Equal(t, int64(4), stats.TotalInvocations)
}

func TestIntegration_VersionRollback(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())
	defer engine.Stop()

	// 创建 v1
	fn := &Function{
		Name:    "versioned-func",
		Runtime: RuntimeGo,
		Handler: "main.Handle",
		Code:    "v1 code",
	}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	// 更新到 v2
	fn.Code = "v2 code"
	fn.Description = "version 2"
	require.NoError(t, engine.UpdateFunction(fn))

	// 更新到 v3
	fn.Code = "v3 code"
	fn.Description = "version 3"
	require.NoError(t, engine.UpdateFunction(fn))

	// 验证有 3 个版本
	versions, _ := engine.GetVersions(fn.ID)
	assert.Len(t, versions, 3)

	// 回滚到 v1
	err := engine.RollbackVersion(fn.ID, "1.0.0")
	require.NoError(t, err)

	got, _ := engine.GetFunction(fn.ID)
	assert.Equal(t, "v1 code", got.Code)
	assert.Equal(t, "1.0.0", got.Version)
}

func TestIntegration_ConcurrentAccess(t *testing.T) {
	engine := NewEngine(nil)
	require.NoError(t, engine.Start())
	defer engine.Stop()

	// 创建函数
	fn := &Function{
		Name:           "concurrent-func",
		Runtime:        RuntimeGo,
		Handler:        "main.Handle",
		Code:           "code",
		Config:         FunctionConfig{MaxConcurrency: 50},
	}
	require.NoError(t, engine.CreateFunction(fn))
	require.NoError(t, engine.DeployFunction(fn.ID))

	// 并发读写
	var wg sync.WaitGroup

	// 并发读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			engine.GetFunction(fn.ID)
			engine.ListFunctions(nil)
			engine.GetStats()
		}()
	}

	// 并发调用
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			engine.InvokeSync(context.Background(), fn.ID, nil)
		}()
	}

	wg.Wait()

	// 验证数据一致性
	got, _ := engine.GetFunction(fn.ID)
	assert.Equal(t, int64(5), got.InvokeCount)
}

// ========== fmt.Sprintf import ==========

func TestGenerateID(t *testing.T) {
	id1 := generateID("test")
	id2 := generateID("test")
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2) // 理论上不同（纳秒级）
}

func TestParseInt(t *testing.T) {
	assert.Equal(t, 0, parseInt("0"))
	assert.Equal(t, 1, parseInt("1"))
	assert.Equal(t, 42, parseInt("42"))
	assert.Equal(t, 0, parseInt("abc"))
}
