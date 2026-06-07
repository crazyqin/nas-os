// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 类型测试 ==========

func TestWorkflowStatus(t *testing.T) {
	assert.Equal(t, WorkflowStatus("draft"), StatusDraft)
	assert.Equal(t, WorkflowStatus("active"), StatusActive)
	assert.Equal(t, WorkflowStatus("paused"), StatusPaused)
	assert.Equal(t, WorkflowStatus("disabled"), StatusDisabled)
	assert.Equal(t, WorkflowStatus("archived"), StatusArchived)
}

func TestNodeType(t *testing.T) {
	assert.Equal(t, NodeType("trigger"), NodeTypeTrigger)
	assert.Equal(t, NodeType("action"), NodeTypeAction)
	assert.Equal(t, NodeType("condition"), NodeTypeCondition)
	assert.Equal(t, NodeType("loop"), NodeTypeLoop)
	assert.Equal(t, NodeType("start"), NodeTypeStart)
	assert.Equal(t, NodeType("end"), NodeTypeEnd)
}

func TestExecutionStatus(t *testing.T) {
	assert.Equal(t, ExecutionStatus("pending"), ExecPending)
	assert.Equal(t, ExecutionStatus("running"), ExecRunning)
	assert.Equal(t, ExecutionStatus("success"), ExecSuccess)
	assert.Equal(t, ExecutionStatus("failed"), ExecFailed)
	assert.Equal(t, ExecutionStatus("cancelled"), ExecCancelled)
	assert.Equal(t, ExecutionStatus("skipped"), ExecSkipped)
	assert.Equal(t, ExecutionStatus("timeout"), ExecTimeout)
}

func TestTriggerType(t *testing.T) {
	assert.Equal(t, TriggerType("cron"), TriggerCron)
	assert.Equal(t, TriggerType("event"), TriggerOnEvent)
	assert.Equal(t, TriggerType("webhook"), TriggerWebhook)
	assert.Equal(t, TriggerType("file"), TriggerFile)
	assert.Equal(t, TriggerType("manual"), TriggerManual)
}

func TestActionType(t *testing.T) {
	assert.Equal(t, ActionType("file_ops"), ActionFileOps)
	assert.Equal(t, ActionType("notification"), ActionNotification)
	assert.Equal(t, ActionType("api_call"), ActionAPICall)
	assert.Equal(t, ActionType("container"), ActionContainer)
	assert.Equal(t, ActionType("shell"), ActionShell)
	assert.Equal(t, ActionType("transform"), ActionTransform)
}

func TestConditionOp(t *testing.T) {
	assert.Equal(t, ConditionOp("eq"), OpEquals)
	assert.Equal(t, ConditionOp("ne"), OpNotEquals)
	assert.Equal(t, ConditionOp("gt"), OpGreaterThan)
	assert.Equal(t, ConditionOp("lt"), OpLessThan)
	assert.Equal(t, ConditionOp("gte"), OpGreaterEqual)
	assert.Equal(t, ConditionOp("lte"), OpLessEqual)
	assert.Equal(t, ConditionOp("contains"), OpContains)
	assert.Equal(t, ConditionOp("starts_with"), OpStartsWith)
	assert.Equal(t, ConditionOp("ends_with"), OpEndsWith)
	assert.Equal(t, ConditionOp("matches"), OpMatches)
	assert.Equal(t, ConditionOp("in"), OpIn)
	assert.Equal(t, ConditionOp("exists"), OpExists)
}

func TestWorkflowStruct(t *testing.T) {
	wf := &Workflow{
		ID:          "test-wf-1",
		Name:        "Test Workflow",
		Description: "A test workflow",
		Version:     1,
		Status:      StatusDraft,
		Nodes:       make(map[string]*Node),
		Edges:       make([]*Edge, 0),
		Variables:   map[string]string{"env": "test"},
		Labels:      map[string]string{"team": "devops"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   "tester",
	}

	assert.Equal(t, "test-wf-1", wf.ID)
	assert.Equal(t, "Test Workflow", wf.Name)
	assert.Equal(t, StatusDraft, wf.Status)
	assert.NotNil(t, wf.Nodes)
	assert.NotNil(t, wf.Edges)
	assert.Equal(t, "test", wf.Variables["env"])
}

func TestNodeStruct(t *testing.T) {
	node := &Node{
		ID:          "node-1",
		Type:        NodeTypeAction,
		Name:        "Test Action",
		Description: "A test action",
		Config:      map[string]string{"command": "echo hello"},
		Position:    &Position{X: 100, Y: 200},
		ActionType:  ActionShell,
		Enabled:     true,
		Timeout:     30 * time.Second,
		RetryPolicy: &RetryPolicy{
			MaxRetries:  3,
			BackoffType: "exponential",
			Interval:    time.Second,
		},
	}

	assert.Equal(t, "node-1", node.ID)
	assert.Equal(t, NodeTypeAction, node.Type)
	assert.Equal(t, ActionShell, node.ActionType)
	assert.True(t, node.Enabled)
	assert.NotNil(t, node.RetryPolicy)
	assert.Equal(t, 3, node.RetryPolicy.MaxRetries)
}

func TestEdgeStruct(t *testing.T) {
	edge := &Edge{
		ID:        "edge-1",
		From:      "node-1",
		To:        "node-2",
		Label:     "next",
		Condition: "true",
	}

	assert.Equal(t, "edge-1", edge.ID)
	assert.Equal(t, "node-1", edge.From)
	assert.Equal(t, "node-2", edge.To)
}

func TestExecutionStruct(t *testing.T) {
	now := time.Now()
	exec := &Execution{
		ID:         "exec-1",
		WorkflowID: "wf-1",
		Version:    1,
		Status:     ExecRunning,
		Context:    map[string]interface{}{"key": "value"},
		Steps:      make([]*StepExecution, 0),
		StartedAt:  now,
	}

	assert.Equal(t, "exec-1", exec.ID)
	assert.Equal(t, ExecRunning, exec.Status)
	assert.NotNil(t, exec.Context)
	assert.Equal(t, "value", exec.Context["key"])
}

func TestStepExecutionStruct(t *testing.T) {
	now := time.Now()
	finTime := now.Add(time.Second)
	step := &StepExecution{
		NodeID:     "node-1",
		Status:     ExecSuccess,
		Input:      map[string]interface{}{"key": "value"},
		Output:     map[string]interface{}{"result": "ok"},
		StartedAt:  now,
		FinishedAt: &finTime,
		Duration:   time.Second,
		RetryCount: 0,
	}

	assert.Equal(t, "node-1", step.NodeID)
	assert.Equal(t, ExecSuccess, step.Status)
	assert.Equal(t, time.Second, step.Duration)
}

func TestTriggerStruct(t *testing.T) {
	trigger := &Trigger{
		ID:         "trigger-1",
		WorkflowID: "wf-1",
		Type:       TriggerCron,
		Name:       "Daily Backup",
		Config:     map[string]string{"schedule": "0 2 * * *"},
		Enabled:    true,
		CreatedAt:  time.Now(),
	}

	assert.Equal(t, "trigger-1", trigger.ID)
	assert.Equal(t, TriggerCron, trigger.Type)
	assert.True(t, trigger.Enabled)
}

func TestConditionExprStruct(t *testing.T) {
	expr := &ConditionExpr{
		Logic: LogicAnd,
		Children: []*ConditionExpr{
			{Op: OpEquals, Field: "status", Value: "active"},
			{Op: OpGreaterThan, Field: "count", Value: 10},
		},
	}

	assert.Equal(t, LogicAnd, expr.Logic)
	require.Len(t, expr.Children, 2)
	assert.Equal(t, OpEquals, expr.Children[0].Op)
	assert.Equal(t, OpGreaterThan, expr.Children[1].Op)
}

func TestRetryPolicyStruct(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:  3,
		BackoffType: "exponential",
		Interval:    2 * time.Second,
	}

	assert.Equal(t, 3, policy.MaxRetries)
	assert.Equal(t, "exponential", policy.BackoffType)
	assert.Equal(t, 2*time.Second, policy.Interval)
}

func TestLoopConfigStruct(t *testing.T) {
	config := &LoopConfig{
		MaxIterations:  100,
		CollectionKey:  "items",
		BreakCondition: "done",
	}

	assert.Equal(t, 100, config.MaxIterations)
	assert.Equal(t, "items", config.CollectionKey)
	assert.Equal(t, "done", config.BreakCondition)
}

func TestPositionStruct(t *testing.T) {
	pos := &Position{X: 150, Y: 300}
	assert.Equal(t, 150, pos.X)
	assert.Equal(t, 300, pos.Y)
}

func TestErrorVariables(t *testing.T) {
	assert.NotNil(t, ErrWorkflowNotFound)
	assert.NotNil(t, ErrTriggerNotFound)
	assert.NotNil(t, ErrActionNotFound)
	assert.NotNil(t, ErrInvalidConfig)
	assert.NotNil(t, ErrExecutionTimeout)
	assert.NotNil(t, ErrMaxRetriesExceeded)

	assert.Contains(t, ErrWorkflowNotFound.Error(), "workflow not found")
	assert.Contains(t, ErrTriggerNotFound.Error(), "trigger not found")
}
