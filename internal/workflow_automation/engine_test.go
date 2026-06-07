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

func TestNewEngine(t *testing.T) {
	engine := NewEngine(nil, nil)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.workflows)
	assert.NotNil(t, engine.handlers)
	assert.NotNil(t, engine.executions)
	assert.False(t, engine.IsRunning())
}

func TestEngineWithLogger(t *testing.T) {
	logger := zap.NewNop()
	engine := NewEngine(nil, logger)

	assert.NotNil(t, engine)
	assert.Equal(t, logger, engine.logger)
}

func TestEngineStartStop(t *testing.T) {
	engine := NewEngine(nil, nil)

	assert.False(t, engine.IsRunning())

	engine.Start()
	assert.True(t, engine.IsRunning())

	// 启动两次应该无影响
	engine.Start()
	assert.True(t, engine.IsRunning())

	engine.Stop()
	assert.False(t, engine.IsRunning())

	// 停止两次应该无影响
	engine.Stop()
	assert.False(t, engine.IsRunning())
}

func TestEngineRegisterHandler(t *testing.T) {
	engine := NewEngine(nil, nil)

	// 内置处理器已注册
	handler, ok := engine.GetHandler(ActionFileOps)
	assert.True(t, ok)
	assert.NotNil(t, handler)

	handler, ok = engine.GetHandler(ActionShell)
	assert.True(t, ok)
	assert.NotNil(t, handler)

	// 不存在的处理器
	handler, ok = engine.GetHandler("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, handler)
}

func TestEngineCreateWorkflow(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{
		Name:        "Test Workflow",
		Description: "Test",
	}

	err := engine.CreateWorkflow(wf)
	require.NoError(t, err)

	assert.NotEmpty(t, wf.ID)
	assert.Equal(t, 1, wf.Version)
	assert.Equal(t, StatusDraft, wf.Status)
	assert.NotNil(t, wf.Nodes)
	assert.NotNil(t, wf.Edges)

	// 应该有 start 和 end 节点
	assert.Contains(t, wf.Nodes, "start")
	assert.Contains(t, wf.Nodes, "end")
	assert.Equal(t, NodeTypeStart, wf.Nodes["start"].Type)
	assert.Equal(t, NodeTypeEnd, wf.Nodes["end"].Type)
}

func TestEngineGetWorkflow(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test Workflow"}
	err := engine.CreateWorkflow(wf)
	require.NoError(t, err)

	// 获取存在的工作流
	got, err := engine.GetWorkflow(wf.ID)
	require.NoError(t, err)
	assert.Equal(t, wf.ID, got.ID)
	assert.Equal(t, wf.Name, got.Name)

	// 获取不存在的工作流
	_, err = engine.GetWorkflow("nonexistent")
	assert.ErrorIs(t, err, ErrWorkflowNotFound)
}

func TestEngineListWorkflows(t *testing.T) {
	engine := NewEngine(nil, nil)

	// 创建多个工作流
	for i := 0; i < 3; i++ {
		wf := &Workflow{Name: "Workflow"}
		engine.CreateWorkflow(wf)
	}

	wfs, err := engine.ListWorkflows()
	require.NoError(t, err)
	assert.Len(t, wfs, 3)
}

func TestEngineUpdateWorkflow(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test Workflow"}
	engine.CreateWorkflow(wf)

	originalVersion := wf.Version

	// 更新工作流
	wf.Name = "Updated Workflow"
	err := engine.UpdateWorkflow(wf)
	require.NoError(t, err)

	assert.Equal(t, originalVersion+1, wf.Version)
	assert.Equal(t, "Updated Workflow", wf.Name)

	// 更新不存在的工作流
	wf2 := &Workflow{ID: "nonexistent", Name: "test"}
	err = engine.UpdateWorkflow(wf2)
	assert.ErrorIs(t, err, ErrWorkflowNotFound)
}

func TestEngineDeleteWorkflow(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test Workflow"}
	engine.CreateWorkflow(wf)

	// 删除工作流
	err := engine.DeleteWorkflow(wf.ID)
	assert.NoError(t, err)

	// 验证已删除
	_, err = engine.GetWorkflow(wf.ID)
	assert.ErrorIs(t, err, ErrWorkflowNotFound)

	// 删除不存在的工作流
	err = engine.DeleteWorkflow("nonexistent")
	assert.ErrorIs(t, err, ErrWorkflowNotFound)
}

func TestEngineAddNode(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)

	node := &Node{
		Type:       NodeTypeAction,
		Name:       "Test Action",
		ActionType: ActionShell,
		Config:     map[string]string{"command": "echo hello"},
	}

	err := engine.AddNode(wf.ID, node)
	require.NoError(t, err)

	assert.NotEmpty(t, node.ID)
	assert.Contains(t, wf.Nodes, node.ID)

	// 不存在的工作流
	err = engine.AddNode("nonexistent", node)
	assert.ErrorIs(t, err, ErrWorkflowNotFound)
}

func TestEngineRemoveNode(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)

	// 添加节点
	node := &Node{
		ID:         "test-node",
		Type:       NodeTypeAction,
		Name:       "Test",
		ActionType: ActionShell,
	}
	engine.AddNode(wf.ID, node)

	// 添加边
	edge := &Edge{
		ID:   "test-edge",
		From: "start",
		To:   "test-node",
	}
	engine.AddEdge(wf.ID, edge)

	// 删除节点
	err := engine.RemoveNode(wf.ID, "test-node")
	assert.NoError(t, err)
	assert.NotContains(t, wf.Nodes, "test-node")

	// 边也应该被删除
	for _, e := range wf.Edges {
		assert.NotEqual(t, "test-node", e.From)
		assert.NotEqual(t, "test-node", e.To)
	}

	// 不能删除 start/end 节点
	err = engine.RemoveNode(wf.ID, "start")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete")
}

func TestEngineAddEdge(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)

	// 添加节点
	node := &Node{
		ID:   "action-1",
		Type: NodeTypeAction,
		Name: "Action 1",
	}
	engine.AddNode(wf.ID, node)

	// 添加边
	edge := &Edge{
		From: "start",
		To:   "action-1",
	}

	err := engine.AddEdge(wf.ID, edge)
	require.NoError(t, err)
	assert.NotEmpty(t, edge.ID)
	assert.Len(t, wf.Edges, 1)

	// 不存在的源节点
	badEdge := &Edge{From: "nonexistent", To: "action-1"}
	err = engine.AddEdge(wf.ID, badEdge)
	assert.Error(t, err)

	// 不存在的目标节点
	badEdge2 := &Edge{From: "start", To: "nonexistent"}
	err = engine.AddEdge(wf.ID, badEdge2)
	assert.Error(t, err)
}

func TestEngineRemoveEdge(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)

	edge := &Edge{
		ID:   "edge-1",
		From: "start",
		To:   "end",
	}
	engine.AddEdge(wf.ID, edge)

	err := engine.RemoveEdge(wf.ID, "edge-1")
	assert.NoError(t, err)
	assert.Len(t, wf.Edges, 0)

	// 不存在的边
	err = engine.RemoveEdge(wf.ID, "nonexistent")
	assert.Error(t, err)
}

func TestEngineExecuteInactiveWorkflow(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test", Status: StatusDraft}
	engine.CreateWorkflow(wf)

	_, err := engine.Execute(context.Background(), wf.ID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

func TestEngineExecuteActiveWorkflow(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)
	wf.Status = StatusActive
	engine.UpdateWorkflow(wf)

	// 连接 start -> end
	edge := &Edge{From: "start", To: "end"}
	engine.AddEdge(wf.ID, edge)

	exec, err := engine.Execute(context.Background(), wf.ID, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, exec.ID)
	assert.Equal(t, wf.ID, exec.WorkflowID)

	// 等待执行完成
	time.Sleep(100 * time.Millisecond)

	exec, err = engine.GetExecution(exec.ID)
	require.NoError(t, err)
	assert.Equal(t, ExecSuccess, exec.Status)
}

func TestEngineExecuteWithTrigger(t *testing.T) {
	engine := NewEngine(nil, nil)

	wf := &Workflow{Name: "Test"}
	engine.CreateWorkflow(wf)
	wf.Status = StatusActive
	engine.UpdateWorkflow(wf)

	edge := &Edge{From: "start", To: "end"}
	engine.AddEdge(wf.ID, edge)

	trigger := &TriggerEvent{
		TriggerID:  "test-trigger",
		WorkflowID: wf.ID,
		Type:       TriggerCron,
		Payload:    map[string]interface{}{"key": "value"},
		Timestamp:  time.Now(),
	}

	exec, err := engine.Execute(context.Background(), wf.ID, trigger)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	exec, err = engine.GetExecution(exec.ID)
	require.NoError(t, err)
	assert.Equal(t, "test-trigger", exec.TriggerID)
	assert.Equal(t, "value", exec.Context["key"])
}

func TestEngineGetExecLogger(t *testing.T) {
	engine := NewEngine(nil, nil)

	logger := engine.GetExecLogger()
	assert.NotNil(t, logger)
}

func TestEngineGetTriggerManager(t *testing.T) {
	engine := NewEngine(nil, nil)

	tm := engine.GetTriggerManager()
	assert.NotNil(t, tm)
}
