// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewExecutor(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	assert.NotNil(t, executor)
	assert.Equal(t, engine, executor.engine)
	assert.Equal(t, 5, executor.maxConcurrent)
	assert.NotNil(t, executor.semaphore)
}

func TestNewExecutorDefaultConcurrency(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 0)

	assert.Equal(t, 10, executor.maxConcurrent)
}

func TestExecutorSubmit(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	// 创建活跃工作流
	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)
	wf.Status = StatusActive
	engine.UpdateWorkflow(wf)
	edge := &Edge{From: "start", To: "end"}
	engine.AddEdge(wf.ID, edge)

	exec, err := executor.Submit(context.Background(), wf.ID, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, exec.ID)
	assert.Equal(t, wf.ID, exec.WorkflowID)

	// 等待执行完成
	time.Sleep(100 * time.Millisecond)

	got, ok := executor.GetExecution(exec.ID)
	assert.True(t, ok)
	assert.NotNil(t, got)
}

func TestExecutorSubmitInactiveWorkflow(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	wf := &Workflow{Name: "Test", Status: StatusDraft}
	engine.CreateWorkflow(wf)

	_, err := executor.Submit(context.Background(), wf.ID, nil)
	assert.Error(t, err)
}

func TestExecutorSubmitWithConcurrencyLimit(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 1) // 最大并发 1

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)
	wf.Status = StatusActive
	engine.UpdateWorkflow(wf)
	edge := &Edge{From: "start", To: "end"}
	engine.AddEdge(wf.ID, edge)

	// 第一个应该成功
	exec1, err := executor.Submit(context.Background(), wf.ID, nil)
	require.NoError(t, err)
	assert.NotNil(t, exec1)
}

func TestExecutorSubmitCancelledContext(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 1)

	// 填满信号量
	executor.semaphore <- struct{}{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := executor.Submit(ctx, "wf-1", nil)
	assert.Error(t, err)
}

func TestExecutorGetExecution(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)
	wf.Status = StatusActive
	engine.UpdateWorkflow(wf)
	edge := &Edge{From: "start", To: "end"}
	engine.AddEdge(wf.ID, edge)

	exec, _ := executor.Submit(context.Background(), wf.ID, nil)
	time.Sleep(100 * time.Millisecond)

	got, ok := executor.GetExecution(exec.ID)
	assert.True(t, ok)
	assert.Equal(t, exec.ID, got.ID)

	// 不存在
	_, ok = executor.GetExecution("nonexistent")
	assert.False(t, ok)
}

func TestExecutorListRunning(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	// 初始应该为空
	running := executor.ListRunning()
	assert.Len(t, running, 0)
}

func TestExecutorCancel(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	// 添加一个模拟执行
	exec := &Execution{
		ID:        "exec-1",
		Status:    ExecRunning,
		StartedAt: time.Now(),
	}
	executor.executions[exec.ID] = exec

	err := executor.Cancel("exec-1")
	assert.NoError(t, err)

	got, _ := executor.GetExecution("exec-1")
	assert.Equal(t, ExecCancelled, got.Status)
	assert.NotNil(t, got.FinishedAt)

	// 取消已完成的执行
	exec2 := &Execution{
		ID:        "exec-2",
		Status:    ExecSuccess,
		StartedAt: time.Now(),
	}
	executor.executions[exec2.ID] = exec2

	err = executor.Cancel("exec-2")
	assert.Error(t, err)

	// 取消不存在的
	err = executor.Cancel("nonexistent")
	assert.Error(t, err)
}

func TestExecutorGetStats(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	// 添加模拟执行
	executor.executions["1"] = &Execution{Status: ExecSuccess, Duration: time.Second}
	executor.executions["2"] = &Execution{Status: ExecFailed, Duration: 2 * time.Second}
	executor.executions["3"] = &Execution{Status: ExecRunning}
	executor.executions["4"] = &Execution{Status: ExecCancelled, Duration: 500 * time.Millisecond}

	stats := executor.GetStats()

	assert.Equal(t, 4, stats.Total)
	assert.Equal(t, 1, stats.Success)
	assert.Equal(t, 1, stats.Failed)
	assert.Equal(t, 1, stats.Running)
	assert.Equal(t, 1, stats.Cancelled)
	assert.Equal(t, 0, stats.Pending)
	assert.True(t, stats.AvgDuration > 0)
}

func TestExecutorCleanup(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	// 添加旧执行记录
	oldTime := time.Now().Add(-2 * time.Hour)
	executor.executions["old"] = &Execution{
		ID:         "old",
		Status:     ExecSuccess,
		FinishedAt: &oldTime,
		Duration:   time.Second,
	}

	// 添加新执行记录
	recentTime := time.Now()
	executor.executions["new"] = &Execution{
		ID:         "new",
		Status:     ExecSuccess,
		FinishedAt: &recentTime,
		Duration:   time.Second,
	}

	removed := executor.Cleanup(1 * time.Hour)
	assert.Equal(t, 1, removed)

	_, okOld := executor.GetExecution("old")
	assert.False(t, okOld)

	_, okNew := executor.GetExecution("new")
	assert.True(t, okNew)
}

func TestExecutorWithLogger(t *testing.T) {
	logger := zap.NewNop()
	engine := NewEngine(nil, logger)
	executor := NewExecutor(engine, logger, 5)

	assert.Equal(t, logger, executor.logger)
}

func TestExecutorSubmitSync(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)
	wf.Status = StatusActive
	engine.UpdateWorkflow(wf)
	edge := &Edge{From: "start", To: "end"}
	engine.AddEdge(wf.ID, edge)

	exec, err := executor.SubmitSync(context.Background(), wf.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, ExecSuccess, exec.Status)
	assert.NotNil(t, exec.FinishedAt)
	assert.True(t, exec.Duration >= 0)
}

func TestExecutorSubmitSyncInactive(t *testing.T) {
	engine := NewEngine(nil, nil)
	executor := NewExecutor(engine, nil, 5)

	wf := &Workflow{Name: "Test", Status: StatusDraft}
	engine.CreateWorkflow(wf)

	_, err := executor.SubmitSync(context.Background(), wf.ID, nil)
	assert.Error(t, err)
}
