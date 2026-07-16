package opsrunbook

import (
	"context"
	"testing"
	"time"
)

func TestManagerRegisterAndGetRunbook(t *testing.T) {
	mgr := NewManager(nil, nil)

	rb := &Runbook{
		ID:          "test_rb_1",
		Name:        "测试手册",
		Description: "测试用运维手册",
		Category:    "test",
		Severity:    SevInfo,
		Tags:        []string{"test"},
		Trigger:     TriggerManual,
		Steps: []*Step{
			{
				ID:      "step1",
				Name:    "步骤1",
				Type:    StepTypeCommand,
				Command: "echo hello",
				Timeout: 10 * time.Second,
			},
		},
		RollbackOn: "failure",
	}

	if err := mgr.RegisterRunbook(rb); err != nil {
		t.Fatalf("RegisterRunbook failed: %v", err)
	}

	got, err := mgr.GetRunbook("test_rb_1")
	if err != nil {
		t.Fatalf("GetRunbook failed: %v", err)
	}

	if got.Name != "测试手册" {
		t.Errorf("expected name '测试手册', got '%s'", got.Name)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
}

func TestManagerRegisterValidation(t *testing.T) {
	mgr := NewManager(nil, nil)

	tests := []struct {
		name    string
		rb      *Runbook
		wantErr bool
	}{
		{
			name:    "empty ID",
			rb:      &Runbook{Name: "test"},
			wantErr: true,
		},
		{
			name:    "empty name",
			rb:      &Runbook{ID: "test"},
			wantErr: true,
		},
		{
			name:    "no steps",
			rb:      &Runbook{ID: "test", Name: "test"},
			wantErr: true,
		},
		{
			name: "valid runbook",
			rb: &Runbook{
				ID:   "test_valid",
				Name: "valid",
				Steps: []*Step{
					{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo ok"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.RegisterRunbook(tt.rb)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterRunbook() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagerListRunbooks(t *testing.T) {
	mgr := NewManager(nil, nil)

	runbooks := []*Runbook{
		{
			ID: "rb1", Name: "手册1", Category: "storage", Severity: SevCritical,
			Steps: []*Step{{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo 1"}},
		},
		{
			ID: "rb2", Name: "手册2", Category: "service", Severity: SevError,
			Steps: []*Step{{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo 2"}},
		},
		{
			ID: "rb3", Name: "手册3", Category: "storage", Severity: SevWarning,
			Steps: []*Step{{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo 3"}},
		},
	}

	for _, rb := range runbooks {
		if err := mgr.RegisterRunbook(rb); err != nil {
			t.Fatalf("RegisterRunbook(%s) failed: %v", rb.ID, err)
		}
	}

	// 无过滤
	all := mgr.ListRunbooks(RunbookFilter{})
	if len(all) != 3 {
		t.Errorf("expected 3 runbooks, got %d", len(all))
	}

	// 按类别过滤
	storage := mgr.ListRunbooks(RunbookFilter{Category: "storage"})
	if len(storage) != 2 {
		t.Errorf("expected 2 storage runbooks, got %d", len(storage))
	}

	// 按严重级别过滤
	critical := mgr.ListRunbooks(RunbookFilter{Severity: SevCritical})
	if len(critical) != 1 {
		t.Errorf("expected 1 critical runbook, got %d", len(critical))
	}

	// 搜索过滤
	search := mgr.ListRunbooks(RunbookFilter{Search: "手册1"})
	if len(search) != 1 {
		t.Errorf("expected 1 search result, got %d", len(search))
	}
}

func TestManagerUpdateRunbook(t *testing.T) {
	mgr := NewManager(nil, nil)

	rb := &Runbook{
		ID: "rb_update", Name: "原始名称",
		Steps: []*Step{{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo 1"}},
	}
	mgr.RegisterRunbook(rb)

	rb.Name = "更新后名称"
	if err := mgr.UpdateRunbook(rb); err != nil {
		t.Fatalf("UpdateRunbook failed: %v", err)
	}

	got, _ := mgr.GetRunbook("rb_update")
	if got.Name != "更新后名称" {
		t.Errorf("expected '更新后名称', got '%s'", got.Name)
	}
	if got.Version != 2 {
		t.Errorf("expected version 2, got %d", got.Version)
	}
}

func TestManagerDeleteRunbook(t *testing.T) {
	mgr := NewManager(nil, nil)

	rb := &Runbook{
		ID: "rb_delete", Name: "待删除",
		Steps: []*Step{{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo 1"}},
	}
	mgr.RegisterRunbook(rb)

	if err := mgr.DeleteRunbook("rb_delete"); err != nil {
		t.Fatalf("DeleteRunbook failed: %v", err)
	}

	_, err := mgr.GetRunbook("rb_delete")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestManagerDuplicateStepID(t *testing.T) {
	mgr := NewManager(nil, nil)

	rb := &Runbook{
		ID: "rb_dup", Name: "重复步骤ID",
		Steps: []*Step{
			{ID: "same", Name: "step1", Type: StepTypeCommand, Command: "echo 1"},
			{ID: "same", Name: "step2", Type: StepTypeCommand, Command: "echo 2"},
		},
	}

	err := mgr.RegisterRunbook(rb)
	if err == nil {
		t.Error("expected error for duplicate step ID, got nil")
	}
}

func TestManagerInvalidDependency(t *testing.T) {
	mgr := NewManager(nil, nil)

	rb := &Runbook{
		ID: "rb_bad_dep", Name: "无效依赖",
		Steps: []*Step{
			{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo 1", DependsOn: []string{"nonexistent"}},
		},
	}

	err := mgr.RegisterRunbook(rb)
	if err == nil {
		t.Error("expected error for invalid dependency, got nil")
	}
}

func TestExecutorSimpleCommand(t *testing.T) {
	mgr := NewManager(nil, nil)
	executor := NewExecutor(mgr, nil, ExecutorConfig{})

	rb := &Runbook{
		ID:     "rb_exec",
		Name:   "执行测试",
		Status: StatusActive,
		Steps: []*Step{
			{ID: "s1", Name: "echo", Type: StepTypeCommand, Command: "echo hello"},
		},
		RollbackOn: "never",
	}
	mgr.RegisterRunbook(rb)

	exec, err := executor.Execute(context.Background(), "rb_exec", TriggerManual, "", nil, "tester")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != StepSuccess {
		t.Errorf("expected success, got %s", exec.Status)
	}
	if len(exec.Steps) != 1 {
		t.Errorf("expected 1 step result, got %d", len(exec.Steps))
	}
	if exec.Steps[0].Status != StepSuccess {
		t.Errorf("step status expected success, got %s", exec.Steps[0].Status)
	}
}

func TestExecutorMultiStep(t *testing.T) {
	mgr := NewManager(nil, nil)
	executor := NewExecutor(mgr, nil, ExecutorConfig{})

	rb := &Runbook{
		ID:     "rb_multi",
		Name:   "多步骤测试",
		Status: StatusActive,
		Steps: []*Step{
			{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo step1"},
			{ID: "s2", Name: "step2", Type: StepTypeCommand, Command: "echo step2", DependsOn: []string{"s1"}},
			{ID: "s3", Name: "step3", Type: StepTypeCommand, Command: "echo step3", DependsOn: []string{"s2"}},
		},
		RollbackOn: "never",
	}
	mgr.RegisterRunbook(rb)

	exec, err := executor.Execute(context.Background(), "rb_multi", TriggerManual, "", nil, "tester")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != StepSuccess {
		t.Errorf("expected success, got %s", exec.Status)
	}
	if len(exec.Steps) != 3 {
		t.Errorf("expected 3 step results, got %d", len(exec.Steps))
	}
}

func TestExecutorWithVariables(t *testing.T) {
	mgr := NewManager(nil, nil)
	executor := NewExecutor(mgr, nil, ExecutorConfig{})

	rb := &Runbook{
		ID:     "rb_vars",
		Name:   "变量测试",
		Status: StatusActive,
		Steps: []*Step{
			{ID: "s1", Name: "echo var", Type: StepTypeCommand, Command: "echo ${my_var}"},
		},
		RollbackOn: "never",
	}
	mgr.RegisterRunbook(rb)

	vars := map[string]string{"my_var": "test_value"}
	exec, err := executor.Execute(context.Background(), "rb_vars", TriggerManual, "", vars, "tester")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != StepSuccess {
		t.Errorf("expected success, got %s", exec.Status)
	}
}

func TestExecutorFailedStep(t *testing.T) {
	mgr := NewManager(nil, nil)
	executor := NewExecutor(mgr, nil, ExecutorConfig{})

	rb := &Runbook{
		ID:     "rb_fail",
		Name:   "失败测试",
		Status: StatusActive,
		Steps: []*Step{
			{ID: "s1", Name: "fail", Type: StepTypeCommand, Command: "false"},
		},
		RollbackOn: "never",
	}
	mgr.RegisterRunbook(rb)

	exec, err := executor.Execute(context.Background(), "rb_fail", TriggerManual, "", nil, "tester")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != StepFailed {
		t.Errorf("expected failed, got %s", exec.Status)
	}
}

func TestExecutorContinueOnFailure(t *testing.T) {
	mgr := NewManager(nil, nil)
	executor := NewExecutor(mgr, nil, ExecutorConfig{})

	rb := &Runbook{
		ID:     "rb_continue",
		Name:   "继续执行测试",
		Status: StatusActive,
		Steps: []*Step{
			{ID: "s1", Name: "fail", Type: StepTypeCommand, Command: "false", ContinueOn: "failure"},
			{ID: "s2", Name: "success", Type: StepTypeCommand, Command: "echo ok"},
		},
		RollbackOn: "never",
	}
	mgr.RegisterRunbook(rb)

	exec, err := executor.Execute(context.Background(), "rb_continue", TriggerManual, "", nil, "tester")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(exec.Steps) != 2 {
		t.Errorf("expected 2 step results, got %d", len(exec.Steps))
	}
}

func TestExecutorNonExistentRunbook(t *testing.T) {
	mgr := NewManager(nil, nil)
	executor := NewExecutor(mgr, nil, ExecutorConfig{})

	_, err := executor.Execute(context.Background(), "nonexistent", TriggerManual, "", nil, "tester")
	if err == nil {
		t.Error("expected error for non-existent runbook, got nil")
	}
}

func TestExecutorInactiveRunbook(t *testing.T) {
	mgr := NewManager(nil, nil)
	executor := NewExecutor(mgr, nil, ExecutorConfig{})

	rb := &Runbook{
		ID: "rb_inactive", Name: "inactive", Status: StatusDraft,
		Steps: []*Step{{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo 1"}},
	}
	mgr.RegisterRunbook(rb)

	_, err := executor.Execute(context.Background(), "rb_inactive", TriggerManual, "", nil, "tester")
	if err == nil {
		t.Error("expected error for inactive runbook, got nil")
	}
}

func TestExecutionStats(t *testing.T) {
	mgr := NewManager(nil, nil)
	executor := NewExecutor(mgr, nil, ExecutorConfig{})

	rb := &Runbook{
		ID: "rb_stats", Name: "统计测试", Status: StatusActive,
		Steps: []*Step{
			{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo ok"},
		},
		RollbackOn: "never",
	}
	mgr.RegisterRunbook(rb)

	// 执行3次
	for i := 0; i < 3; i++ {
		executor.Execute(context.Background(), "rb_stats", TriggerManual, "", nil, "tester")
	}

	stats, err := mgr.GetExecutionStats("rb_stats")
	if err != nil {
		t.Fatalf("GetExecutionStats failed: %v", err)
	}

	if stats.TotalRuns != 3 {
		t.Errorf("expected 3 total runs, got %d", stats.TotalRuns)
	}
	if stats.SuccessRuns != 3 {
		t.Errorf("expected 3 success runs, got %d", stats.SuccessRuns)
	}
	if stats.SuccessRate != 1.0 {
		t.Errorf("expected success rate 1.0, got %f", stats.SuccessRate)
	}
}

func TestApprovalFlow(t *testing.T) {
	mgr := NewManager(nil, nil)

	// 添加审批请求
	req := &ApprovalRequest{
		ID:          "approval_1",
		ExecutionID: "exec_1",
		StepID:      "step_1",
		StepName:    "审批步骤",
		RequestedAt: time.Now(),
	}
	mgr.mu.Lock()
	mgr.approvals["approval_1"] = req
	mgr.mu.Unlock()

	// 测试审批
	if err := mgr.Approve("approval_1", "admin", "approved"); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	approved := mgr.approvals["approval_1"]
	if approved.ApprovedBy != "admin" {
		t.Errorf("expected approver 'admin', got '%s'", approved.ApprovedBy)
	}
	if approved.ApprovedAt == nil {
		t.Error("expected ApprovedAt to be set")
	}
}

func TestApprovalReject(t *testing.T) {
	mgr := NewManager(nil, nil)

	req := &ApprovalRequest{
		ID:          "approval_2",
		ExecutionID: "exec_2",
		StepID:      "step_2",
		StepName:    "审批步骤",
		RequestedAt: time.Now(),
	}
	mgr.mu.Lock()
	mgr.approvals["approval_2"] = req
	mgr.mu.Unlock()

	if err := mgr.Reject("approval_2", "admin", "not now"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	rejected := mgr.approvals["approval_2"]
	if !rejected.Rejected {
		t.Error("expected rejected to be true")
	}
}

func TestBuiltInTemplates(t *testing.T) {
	templates := LoadBuiltInTemplates()
	if len(templates) == 0 {
		t.Fatal("expected at least one template")
	}

	for _, tmpl := range templates {
		t.Run(tmpl.ID, func(t *testing.T) {
			if tmpl.ID == "" {
				t.Error("template ID is empty")
			}
			if tmpl.Name == "" {
				t.Error("template name is empty")
			}
			if len(tmpl.Steps) == 0 {
				t.Error("template has no steps")
			}
			for _, step := range tmpl.Steps {
				if step.ID == "" {
					t.Error("step ID is empty")
				}
				if step.Name == "" {
					t.Error("step name is empty")
				}
			}
		})
	}
}

func TestRunbookFilterMatch(t *testing.T) {
	mgr := NewManager(nil, nil)

	rb := &Runbook{
		ID: "rb_filter", Name: "过滤测试", Category: "storage",
		Severity: SevWarning, Status: StatusActive,
		Tags: []string{"disk", "hardware"}, Trigger: TriggerAlert,
		Steps: []*Step{{ID: "s1", Name: "step1", Type: StepTypeCommand, Command: "echo 1"}},
	}

	tests := []struct {
		name   string
		filter RunbookFilter
		match  bool
	}{
		{"no filter", RunbookFilter{}, true},
		{"category match", RunbookFilter{Category: "storage"}, true},
		{"category mismatch", RunbookFilter{Category: "network"}, false},
		{"severity match", RunbookFilter{Severity: SevWarning}, true},
		{"severity mismatch", RunbookFilter{Severity: SevCritical}, false},
		{"tag match", RunbookFilter{Tags: []string{"disk"}}, true},
		{"tag mismatch", RunbookFilter{Tags: []string{"network"}}, false},
		{"search match", RunbookFilter{Search: "过滤"}, true},
		{"search mismatch", RunbookFilter{Search: "不存在"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.matchFilter(rb, tt.filter)
			if got != tt.match {
				t.Errorf("matchFilter() = %v, want %v", got, tt.match)
			}
		})
	}
}
