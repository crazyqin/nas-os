// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewTriggerManager(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	assert.NotNil(t, tm)
	assert.NotNil(t, tm.triggers)
	assert.NotNil(t, tm.cronEntries)
	assert.NotNil(t, tm.eventCh)
	assert.NotNil(t, tm.webhookCh)
	assert.NotNil(t, tm.fileWatchers)
}

func TestTriggerManagerStartStop(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	assert.False(t, tm.running)

	tm.Start()
	assert.True(t, tm.running)

	// 重复启动
	tm.Start()
	assert.True(t, tm.running)

	tm.Stop()
	assert.False(t, tm.running)
}

func TestTriggerManagerCreateTrigger(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	// 创建定时触发器
	trigger := &Trigger{
		WorkflowID: "wf-1",
		Type:       TriggerCron,
		Name:       "Daily Backup",
		Config:     map[string]string{"schedule": "0 0 2 * * *"},
		Enabled:    true,
	}

	err := tm.CreateTrigger(trigger)
	require.NoError(t, err)
	assert.NotEmpty(t, trigger.ID)
	assert.False(t, trigger.CreatedAt.IsZero())

	// 验证触发器已创建
	got, err := tm.GetTrigger(trigger.ID)
	require.NoError(t, err)
	assert.Equal(t, trigger.ID, got.ID)
}

func TestTriggerManagerCreateTriggerValidation(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	// 缺少 workflow_id
	t1 := &Trigger{Type: TriggerCron, Config: map[string]string{"schedule": "* * * * * *"}}
	err := tm.CreateTrigger(t1)
	assert.Error(t, err)

	// 缺少 type
	t2 := &Trigger{WorkflowID: "wf-1"}
	err = tm.CreateTrigger(t2)
	assert.Error(t, err)

	// 定时触发器缺少 schedule
	t3 := &Trigger{WorkflowID: "wf-1", Type: TriggerCron, Config: map[string]string{}}
	err = tm.CreateTrigger(t3)
	assert.Error(t, err)
}

func TestTriggerManagerGetTrigger(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	trigger := &Trigger{
		WorkflowID: "wf-1",
		Type:       TriggerWebhook,
		Name:       "Webhook",
		Config:     map[string]string{},
	}
	tm.CreateTrigger(trigger)

	got, err := tm.GetTrigger(trigger.ID)
	require.NoError(t, err)
	assert.Equal(t, trigger.Name, got.Name)

	// 不存在
	_, err = tm.GetTrigger("nonexistent")
	assert.ErrorIs(t, err, ErrTriggerNotFound)
}

func TestTriggerManagerListTriggers(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	// 创建多个触发器
	tm.CreateTrigger(&Trigger{WorkflowID: "wf-1", Type: TriggerCron, Config: map[string]string{"schedule": "* * * * * *"}})
	tm.CreateTrigger(&Trigger{WorkflowID: "wf-1", Type: TriggerWebhook, Config: map[string]string{}})
	tm.CreateTrigger(&Trigger{WorkflowID: "wf-2", Type: TriggerCron, Config: map[string]string{"schedule": "* * * * * *"}})

	// 列出所有
	all := tm.ListTriggers("")
	assert.Len(t, all, 3)

	// 按工作流过滤
	wf1 := tm.ListTriggers("wf-1")
	assert.Len(t, wf1, 2)

	wf2 := tm.ListTriggers("wf-2")
	assert.Len(t, wf2, 1)
}

func TestTriggerManagerDeleteTrigger(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	trigger := &Trigger{
		WorkflowID: "wf-1",
		Type:       TriggerCron,
		Config:     map[string]string{"schedule": "* * * * * *"},
	}
	tm.CreateTrigger(trigger)

	err := tm.DeleteTrigger(trigger.ID)
	assert.NoError(t, err)

	// 验证已删除
	_, err = tm.GetTrigger(trigger.ID)
	assert.Error(t, err)

	// 删除不存在的
	err = tm.DeleteTrigger("nonexistent")
	assert.Error(t, err)
}

func TestTriggerManagerEnableDisable(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	trigger := &Trigger{
		WorkflowID: "wf-1",
		Type:       TriggerCron,
		Config:     map[string]string{"schedule": "* * * * * *"},
		Enabled:    false,
	}
	tm.CreateTrigger(trigger)

	// 启用
	err := tm.EnableTrigger(trigger.ID)
	assert.NoError(t, err)

	got, _ := tm.GetTrigger(trigger.ID)
	assert.True(t, got.Enabled)

	// 禁用
	err = tm.DisableTrigger(trigger.ID)
	assert.NoError(t, err)

	got, _ = tm.GetTrigger(trigger.ID)
	assert.False(t, got.Enabled)
}

func TestTriggerManagerDisableTriggersByWorkflow(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	// 创建多个触发器
	tm.CreateTrigger(&Trigger{WorkflowID: "wf-1", Type: TriggerCron, Config: map[string]string{"schedule": "* * * * * *"}, Enabled: true})
	tm.CreateTrigger(&Trigger{WorkflowID: "wf-1", Type: TriggerWebhook, Config: map[string]string{}, Enabled: true})
	tm.CreateTrigger(&Trigger{WorkflowID: "wf-2", Type: TriggerCron, Config: map[string]string{"schedule": "* * * * * *"}, Enabled: true})

	tm.DisableTriggersByWorkflow("wf-1")

	// wf-1 的触发器应该被禁用
	for _, tr := range tm.ListTriggers("wf-1") {
		assert.False(t, tr.Enabled)
	}

	// wf-2 的触发器不受影响
	for _, tr := range tm.ListTriggers("wf-2") {
		assert.True(t, tr.Enabled)
	}
}

func TestTriggerManagerUpdateTrigger(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)

	trigger := &Trigger{
		WorkflowID: "wf-1",
		Type:       TriggerCron,
		Name:       "Original",
		Config:     map[string]string{"schedule": "0 * * * * *"},
	}
	tm.CreateTrigger(trigger)

	// 更新
	trigger.Name = "Updated"
	err := tm.UpdateTrigger(trigger)
	assert.NoError(t, err)

	got, _ := tm.GetTrigger(trigger.ID)
	assert.Equal(t, "Updated", got.Name)
}

func TestTriggerManagerFireEvent(t *testing.T) {
	engine := NewEngine(nil, nil)
	tm := NewTriggerManager(engine, nil)
	tm.Start()
	defer tm.Stop()

	event := &TriggerEvent{
		TriggerID:  "test",
		WorkflowID: "wf-1",
		Type:       TriggerOnEvent,
		Timestamp:  time.Now(),
	}

	// 应该不阻塞
	tm.FireEvent(event)
}

func TestSplitPaths(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"/path1", []string{"/path1"}},
		{"/path1,/path2", []string{"/path1", "/path2"}},
		{"/path1; /path2; /path3", []string{"/path1", "/path2", "/path3"}},
		{"/path1 , /path2", []string{"/path1", "/path2"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitPaths(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTriggerManagerWithLogger(t *testing.T) {
	logger := zap.NewNop()
	engine := NewEngine(nil, logger)
	tm := NewTriggerManager(engine, logger)

	assert.Equal(t, logger, tm.logger)
}
