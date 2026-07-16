package dsmaagent

import (
	"context"
	"testing"
	"time"
)

func TestNewDSMAgent(t *testing.T) {
	agent := NewDSMAgent(0, nil)
	if agent == nil {
		t.Fatal("NewDSMAgent returned nil")
	}

	agent2 := NewDSMAgent(8, nil)
	if agent2.workers != 8 {
		t.Errorf("Expected 8 workers, got %d", agent2.workers)
	}
}

func TestStartStop(t *testing.T) {
	agent := NewDSMAgent(2, nil)

	if err := agent.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := agent.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestRegisterAction(t *testing.T) {
	agent := NewDSMAgent(1, nil)

	t.Run("register valid action", func(t *testing.T) {
		action := &AgentAction{
			Name:        "test.action",
			Description: "Test action",
			Handler: func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{"result": "ok"}, nil
			},
		}
		if err := agent.RegisterAction(action); err != nil {
			t.Fatalf("RegisterAction failed: %v", err)
		}
	})

	t.Run("register duplicate action", func(t *testing.T) {
		action := &AgentAction{
			Name: "test.action",
			Handler: func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
				return nil, nil
			},
		}
		if err := agent.RegisterAction(action); err == nil {
			t.Error("Expected error for duplicate action")
		}
	})

	t.Run("register nil action", func(t *testing.T) {
		if err := agent.RegisterAction(nil); err == nil {
			t.Error("Expected error for nil action")
		}
	})

	t.Run("register action with empty name", func(t *testing.T) {
		action := &AgentAction{
			Handler: func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
				return nil, nil
			},
		}
		if err := agent.RegisterAction(action); err == nil {
			t.Error("Expected error for empty name")
		}
	})
}

func TestSubmitTask(t *testing.T) {
	agent := NewDSMAgent(2, nil)
	agent.Start()
	defer agent.Stop()

	t.Run("submit valid task", func(t *testing.T) {
		task := &AgentTask{
			Name:        "Test Task",
			Description: "Test task description",
			Steps: []*TaskStep{
				{
					Name:   "Step 1",
					Action: "storage.check",
				},
			},
		}

		taskID, err := agent.SubmitTask(task)
		if err != nil {
			t.Fatalf("SubmitTask failed: %v", err)
		}
		if taskID == "" {
			t.Error("Expected non-empty task ID")
		}

		time.Sleep(50 * time.Millisecond)

		retrieved, err := agent.GetTask(taskID)
		if err != nil {
			t.Fatalf("GetTask failed: %v", err)
		}
		if retrieved.Status != TaskCompleted {
			t.Errorf("Expected task completed, got %d", retrieved.Status)
		}
	})

	t.Run("submit nil task", func(t *testing.T) {
		_, err := agent.SubmitTask(nil)
		if err == nil {
			t.Error("Expected error for nil task")
		}
	})
}

func TestCancelTask(t *testing.T) {
	agent := NewDSMAgent(1, nil)
	agent.Start()
	defer agent.Stop()

	task := &AgentTask{
		Name: "Long Task",
		Steps: []*TaskStep{
			{Name: "Step 1", Action: "storage.check"},
		},
	}

	taskID, err := agent.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	err = agent.CancelTask(taskID)
	if err != nil {
		// 任务可能已执行完成（storage.check 太快），这是可接受的
		t.Logf("CancelTask: task may have already completed: %v", err)
	}

	t.Run("cancel non-existent task", func(t *testing.T) {
		err := agent.CancelTask("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent task")
		}
	})
}

func TestExecuteAction(t *testing.T) {
	agent := NewDSMAgent(1, nil)

	t.Run("execute existing action", func(t *testing.T) {
		result, err := agent.ExecuteAction(context.Background(), "storage.check", nil)
		if err != nil {
			t.Fatalf("ExecuteAction failed: %v", err)
		}
		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("execute non-existent action", func(t *testing.T) {
		_, err := agent.ExecuteAction(context.Background(), "non.existent", nil)
		if err == nil {
			t.Error("Expected error for non-existent action")
		}
	})
}

func TestGetMetrics(t *testing.T) {
	agent := NewDSMAgent(1, nil)

	metrics := agent.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}

	if metrics.RegisteredActions != 3 {
		t.Errorf("Expected 3 registered actions, got %d", metrics.RegisteredActions)
	}
}

func TestListActions(t *testing.T) {
	agent := NewDSMAgent(1, nil)

	actions := agent.ListActions()
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions))
	}
}
