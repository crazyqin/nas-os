package smartworkflow

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	sw := New()
	if sw == nil {
		t.Fatal("New() 返回nil")
	}

	if sw.workflows == nil {
		t.Error("workflows map未初始化")
	}
	if sw.executions == nil {
		t.Error("executions map未初始化")
	}
	if sw.templates == nil {
		t.Error("templates map未初始化")
	}
}

func TestCreateWorkflow(t *testing.T) {
	sw := New()

	tests := []struct {
		name      string
		workflow  *Workflow
		wantError bool
	}{
		{
			name: "正常创建工作流",
			workflow: &Workflow{
				ID:   "wf1",
				Name: "测试工作流",
			},
			wantError: false,
		},
		{
			name: "ID为空",
			workflow: &Workflow{
				Name: "测试工作流",
			},
			wantError: true,
		},
		{
			name: "重复ID",
			workflow: &Workflow{
				ID:   "wf1",
				Name: "重复工作流",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.CreateWorkflow(tt.workflow)
			if (err != nil) != tt.wantError {
				t.Errorf("CreateWorkflow() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestUpdateWorkflow(t *testing.T) {
	sw := New()

	// 先创建工作流
	workflow := &Workflow{
		ID:   "wf1",
		Name: "原始名称",
	}
	sw.CreateWorkflow(workflow)

	tests := []struct {
		name      string
		workflow  *Workflow
		wantError bool
	}{
		{
			name: "正常更新",
			workflow: &Workflow{
				ID:   "wf1",
				Name: "新名称",
			},
			wantError: false,
		},
		{
			name: "工作流不存在",
			workflow: &Workflow{
				ID:   "wf_not_exist",
				Name: "不存在",
			},
			wantError: true,
		},
		{
			name: "ID为空",
			workflow: &Workflow{
				Name: "空ID",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.UpdateWorkflow(tt.workflow)
			if (err != nil) != tt.wantError {
				t.Errorf("UpdateWorkflow() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}

	// 验证更新结果
	updated, _ := sw.GetWorkflow("wf1")
	if updated.Name != "新名称" {
		t.Errorf("工作流名称未更新，got = %v, want = %v", updated.Name, "新名称")
	}
}

func TestDeleteWorkflow(t *testing.T) {
	sw := New()

	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试"})

	tests := []struct {
		name       string
		workflowID string
		wantError  bool
	}{
		{
			name:       "正常删除",
			workflowID: "wf1",
			wantError:  false,
		},
		{
			name:       "工作流不存在",
			workflowID: "wf_not_exist",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.DeleteWorkflow(tt.workflowID)
			if (err != nil) != tt.wantError {
				t.Errorf("DeleteWorkflow() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestGetWorkflow(t *testing.T) {
	sw := New()

	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试工作流"})

	tests := []struct {
		name       string
		workflowID string
		wantError  bool
		wantName   string
	}{
		{
			name:       "获取存在的工作流",
			workflowID: "wf1",
			wantError:  false,
			wantName:   "测试工作流",
		},
		{
			name:       "获取不存在的工作流",
			workflowID: "wf_not_exist",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow, err := sw.GetWorkflow(tt.workflowID)
			if (err != nil) != tt.wantError {
				t.Errorf("GetWorkflow() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && workflow.Name != tt.wantName {
				t.Errorf("GetWorkflow() name = %v, want = %v", workflow.Name, tt.wantName)
			}
		})
	}
}

func TestListWorkflows(t *testing.T) {
	sw := New()

	// 空列表
	workflows := sw.ListWorkflows()
	if len(workflows) != 0 {
		t.Errorf("ListWorkflows() 返回 %d 个工作流, want 0", len(workflows))
	}

	// 添加工作流
	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "工作流1"})
	sw.CreateWorkflow(&Workflow{ID: "wf2", Name: "工作流2"})

	workflows = sw.ListWorkflows()
	if len(workflows) != 2 {
		t.Errorf("ListWorkflows() 返回 %d 个工作流, want 2", len(workflows))
	}
}

func TestAddStep(t *testing.T) {
	sw := New()
	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试"})

	tests := []struct {
		name       string
		workflowID string
		step       *Step
		wantError  bool
	}{
		{
			name:       "正常添加步骤",
			workflowID: "wf1",
			step: &Step{
				ID:   "step1",
				Name: "步骤1",
			},
			wantError: false,
		},
		{
			name:       "工作流不存在",
			workflowID: "wf_not_exist",
			step: &Step{
				ID:   "step1",
				Name: "步骤1",
			},
			wantError: true,
		},
		{
			name:       "步骤ID为空",
			workflowID: "wf1",
			step: &Step{
				Name: "空ID步骤",
			},
			wantError: true,
		},
		{
			name:       "步骤ID重复",
			workflowID: "wf1",
			step: &Step{
				ID:   "step1",
				Name: "重复步骤",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.AddStep(tt.workflowID, tt.step)
			if (err != nil) != tt.wantError {
				t.Errorf("AddStep() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestRemoveStep(t *testing.T) {
	sw := New()
	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试"})
	sw.AddStep("wf1", &Step{ID: "step1", Name: "步骤1"})

	tests := []struct {
		name       string
		workflowID string
		stepID     string
		wantError  bool
	}{
		{
			name:       "正常移除步骤",
			workflowID: "wf1",
			stepID:     "step1",
			wantError:  false,
		},
		{
			name:       "工作流不存在",
			workflowID: "wf_not_exist",
			stepID:     "step1",
			wantError:  true,
		},
		{
			name:       "步骤不存在",
			workflowID: "wf1",
			stepID:     "step_not_exist",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.RemoveStep(tt.workflowID, tt.stepID)
			if (err != nil) != tt.wantError {
				t.Errorf("RemoveStep() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestAddTrigger(t *testing.T) {
	sw := New()
	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试"})

	tests := []struct {
		name       string
		workflowID string
		trigger    *Trigger
		wantError  bool
	}{
		{
			name:       "正常添加触发器",
			workflowID: "wf1",
			trigger: &Trigger{
				ID:   "trigger1",
				Type: TriggerTypeCron,
			},
			wantError: false,
		},
		{
			name:       "工作流不存在",
			workflowID: "wf_not_exist",
			trigger: &Trigger{
				ID: "trigger1",
			},
			wantError: true,
		},
		{
			name:       "触发器ID为空",
			workflowID: "wf1",
			trigger:    &Trigger{},
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.AddTrigger(tt.workflowID, tt.trigger)
			if (err != nil) != tt.wantError {
				t.Errorf("AddTrigger() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestRemoveTrigger(t *testing.T) {
	sw := New()
	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试"})
	sw.AddTrigger("wf1", &Trigger{ID: "trigger1", Type: TriggerTypeCron})

	tests := []struct {
		name       string
		workflowID string
		triggerID  string
		wantError  bool
	}{
		{
			name:       "正常移除触发器",
			workflowID: "wf1",
			triggerID:  "trigger1",
			wantError:  false,
		},
		{
			name:       "工作流不存在",
			workflowID: "wf_not_exist",
			triggerID:  "trigger1",
			wantError:  true,
		},
		{
			name:       "触发器不存在",
			workflowID: "wf1",
			triggerID:  "trigger_not_exist",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.RemoveTrigger(tt.workflowID, tt.triggerID)
			if (err != nil) != tt.wantError {
				t.Errorf("RemoveTrigger() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestExecute(t *testing.T) {
	sw := New()

	// 创建一个带步骤的工作流
	workflow := &Workflow{
		ID:      "wf1",
		Name:    "测试工作流",
		Enabled: true,
		Steps: []*Step{
			{ID: "step1", Name: "步骤1", Action: "action1"},
			{ID: "step2", Name: "步骤2", Action: "action2", DependsOn: []string{"step1"}},
		},
	}
	sw.CreateWorkflow(workflow)

	tests := []struct {
		name        string
		workflowID  string
		triggeredBy string
		wantError   bool
	}{
		{
			name:        "正常执行",
			workflowID:  "wf1",
			triggeredBy: "manual",
			wantError:   false,
		},
		{
			name:        "工作流不存在",
			workflowID:  "wf_not_exist",
			triggeredBy: "manual",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := sw.Execute(tt.workflowID, tt.triggeredBy)
			if (err != nil) != tt.wantError {
				t.Errorf("Execute() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if exec.Status != StatusCompleted {
					t.Errorf("执行状态 = %v, want %v", exec.Status, StatusCompleted)
				}
				if len(exec.StepResults) != 2 {
					t.Errorf("步骤结果数量 = %d, want 2", len(exec.StepResults))
				}
			}
		})
	}
}

func TestExecuteDisabledWorkflow(t *testing.T) {
	sw := New()

	workflow := &Workflow{
		ID:      "wf1",
		Name:    "禁用的工作流",
		Enabled: false,
		Steps: []*Step{
			{ID: "step1", Name: "步骤1", Action: "action1"},
		},
	}
	sw.CreateWorkflow(workflow)

	_, err := sw.Execute("wf1", "manual")
	if err == nil {
		t.Error("执行禁用的工作流应该返回错误")
	}
}

func TestSchedule(t *testing.T) {
	sw := New()
	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试"})

	tests := []struct {
		name       string
		workflowID string
		cronExpr   string
		wantError  bool
	}{
		{
			name:       "正常调度",
			workflowID: "wf1",
			cronExpr:   "0 0 * * *",
			wantError:  false,
		},
		{
			name:       "工作流不存在",
			workflowID: "wf_not_exist",
			cronExpr:   "0 0 * * *",
			wantError:  true,
		},
		{
			name:       "空cron表达式",
			workflowID: "wf1",
			cronExpr:   "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.Schedule(tt.workflowID, tt.cronExpr)
			if (err != nil) != tt.wantError {
				t.Errorf("Schedule() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestParseCron(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		wantError bool
	}{
		{
			name:      "正常解析",
			expr:      "0 0 * * *",
			wantError: false,
		},
		{
			name:      "空表达式",
			expr:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := ParseCron(tt.expr)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseCron() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && schedule == nil {
				t.Error("ParseCron() 返回nil schedule")
			}
		})
	}
}

func TestGetExecutionHistory(t *testing.T) {
	sw := New()

	sw.CreateWorkflow(&Workflow{
		ID:      "wf1",
		Name:    "测试",
		Enabled: true,
		Steps: []*Step{
			{ID: "step1", Name: "步骤1", Action: "action1"},
		},
	})

	// 执行两次
	sw.Execute("wf1", "manual")
	sw.Execute("wf1", "cron")

	history := sw.GetExecutionHistory("wf1")
	if len(history) != 2 {
		t.Errorf("GetExecutionHistory() 返回 %d 条记录, want 2", len(history))
	}
}

func TestGetExecution(t *testing.T) {
	sw := New()

	sw.CreateWorkflow(&Workflow{
		ID:      "wf1",
		Name:    "测试",
		Enabled: true,
		Steps: []*Step{
			{ID: "step1", Name: "步骤1", Action: "action1"},
		},
	})

	exec, _ := sw.Execute("wf1", "manual")

	// 获取存在的执行记录
	record, err := sw.GetExecution(exec.ID)
	if err != nil {
		t.Errorf("GetExecution() error = %v", err)
	}
	if record.ID != exec.ID {
		t.Errorf("GetExecution() ID = %v, want %v", record.ID, exec.ID)
	}

	// 获取不存在的执行记录
	_, err = sw.GetExecution("exec_not_exist")
	if err == nil {
		t.Error("获取不存在的执行记录应该返回错误")
	}
}

func TestCreateFromTemplate(t *testing.T) {
	sw := New()

	// 注册模板
	template := &Template{
		ID:          "tpl1",
		Name:        "备份模板",
		Description: "自动备份模板",
		Category:    "backup",
		Workflow: Workflow{
			Name:        "备份工作流",
			Description: "自动备份",
			Steps: []*Step{
				{ID: "step1", Name: "备份步骤", Action: "backup"},
			},
		},
	}
	sw.RegisterTemplate(template)

	tests := []struct {
		name       string
		templateID string
		workflowID string
		variables  map[string]string
		wantError  bool
	}{
		{
			name:       "正常创建工作流",
			templateID: "tpl1",
			workflowID: "wf_from_tpl",
			variables:  map[string]string{"path": "/data"},
			wantError:  false,
		},
		{
			name:       "模板不存在",
			templateID: "tpl_not_exist",
			workflowID: "wf1",
			wantError:  true,
		},
		{
			name:       "工作流ID已存在",
			templateID: "tpl1",
			workflowID: "wf_from_tpl",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow, err := sw.CreateFromTemplate(tt.templateID, tt.workflowID, tt.variables)
			if (err != nil) != tt.wantError {
				t.Errorf("CreateFromTemplate() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if workflow.Name != "备份工作流" {
					t.Errorf("工作流名称 = %v, want %v", workflow.Name, "备份工作流")
				}
				if len(workflow.Steps) != 1 {
					t.Errorf("步骤数量 = %d, want 1", len(workflow.Steps))
				}
			}
		})
	}
}

func TestGetTemplates(t *testing.T) {
	sw := New()

	// 空模板列表
	templates := sw.GetTemplates()
	if len(templates) != 0 {
		t.Errorf("GetTemplates() 返回 %d 个模板, want 0", len(templates))
	}

	// 添加模板
	sw.RegisterTemplate(&Template{ID: "tpl1", Name: "模板1"})
	sw.RegisterTemplate(&Template{ID: "tpl2", Name: "模板2"})

	templates = sw.GetTemplates()
	if len(templates) != 2 {
		t.Errorf("GetTemplates() 返回 %d 个模板, want 2", len(templates))
	}
}

func TestRegisterTemplate(t *testing.T) {
	sw := New()

	tests := []struct {
		name      string
		template  *Template
		wantError bool
	}{
		{
			name:      "正常注册",
			template:  &Template{ID: "tpl1", Name: "模板1"},
			wantError: false,
		},
		{
			name:      "ID为空",
			template:  &Template{Name: "空ID"},
			wantError: true,
		},
		{
			name:      "重复注册",
			template:  &Template{ID: "tpl1", Name: "重复"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.RegisterTemplate(tt.template)
			if (err != nil) != tt.wantError {
				t.Errorf("RegisterTemplate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCancelExecution(t *testing.T) {
	sw := New()

	// 这个测试需要模拟一个正在运行的执行
	// 由于executeSteps是同步的，我们直接设置状态来测试
	sw.CreateWorkflow(&Workflow{
		ID:      "wf1",
		Name:    "测试",
		Enabled: true,
		Steps: []*Step{
			{ID: "step1", Name: "步骤1", Action: "action1"},
		},
	})

	// 手动创建一个运行中的执行记录
	sw.mu.Lock()
	sw.execCounter++
	execID := "exec_running"
	sw.executions[execID] = &Execution{
		ID:         execID,
		WorkflowID: "wf1",
		Status:     StatusRunning,
		StartTime:  time.Now(),
	}
	sw.mu.Unlock()

	tests := []struct {
		name        string
		executionID string
		wantError   bool
	}{
		{
			name:        "取消运行中的执行",
			executionID: execID,
			wantError:   false,
		},
		{
			name:        "执行不存在",
			executionID: "exec_not_exist",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sw.CancelExecution(tt.executionID)
			if (err != nil) != tt.wantError {
				t.Errorf("CancelExecution() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestGetWorkflowTriggers(t *testing.T) {
	sw := New()
	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试"})
	sw.AddTrigger("wf1", &Trigger{ID: "trigger1", Type: TriggerTypeCron})

	tests := []struct {
		name       string
		workflowID string
		wantCount  int
		wantError  bool
	}{
		{
			name:       "获取触发器",
			workflowID: "wf1",
			wantCount:  1,
			wantError:  false,
		},
		{
			name:       "工作流不存在",
			workflowID: "wf_not_exist",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triggers, err := sw.GetWorkflowTriggers(tt.workflowID)
			if (err != nil) != tt.wantError {
				t.Errorf("GetWorkflowTriggers() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && len(triggers) != tt.wantCount {
				t.Errorf("GetWorkflowTriggers() count = %d, want %d", len(triggers), tt.wantCount)
			}
		})
	}
}

func TestEnableDisableWorkflow(t *testing.T) {
	sw := New()
	sw.CreateWorkflow(&Workflow{ID: "wf1", Name: "测试", Enabled: true})

	// 测试禁用
	err := sw.DisableWorkflow("wf1")
	if err != nil {
		t.Errorf("DisableWorkflow() error = %v", err)
	}
	wf, _ := sw.GetWorkflow("wf1")
	if wf.Enabled {
		t.Error("工作流应该被禁用")
	}

	// 测试启用
	err = sw.EnableWorkflow("wf1")
	if err != nil {
		t.Errorf("EnableWorkflow() error = %v", err)
	}
	wf, _ = sw.GetWorkflow("wf1")
	if !wf.Enabled {
		t.Error("工作流应该被启用")
	}

	// 测试不存在的工作流
	err = sw.EnableWorkflow("wf_not_exist")
	if err == nil {
		t.Error("启用不存在的工作流应该返回错误")
	}

	err = sw.DisableWorkflow("wf_not_exist")
	if err == nil {
		t.Error("禁用不存在的工作流应该返回错误")
	}
}

func TestExecutionStatusString(t *testing.T) {
	tests := []struct {
		status ExecutionStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusCancelled, "cancelled"},
		{ExecutionStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ExecutionStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkflowWithDependencies(t *testing.T) {
	sw := New()

	workflow := &Workflow{
		ID:      "wf_deps",
		Name:    "依赖测试",
		Enabled: true,
		Steps: []*Step{
			{ID: "step1", Name: "步骤1", Action: "action1"},
			{ID: "step2", Name: "步骤2", Action: "action2", DependsOn: []string{"step1"}},
			{ID: "step3", Name: "步骤3", Action: "action3", DependsOn: []string{"step1"}},
			{ID: "step4", Name: "步骤4", Action: "action4", DependsOn: []string{"step2", "step3"}},
		},
	}
	sw.CreateWorkflow(workflow)

	exec, err := sw.Execute("wf_deps", "manual")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if exec.Status != StatusCompleted {
		t.Errorf("执行状态 = %v, want %v", exec.Status, StatusCompleted)
	}

	if len(exec.StepResults) != 4 {
		t.Errorf("步骤结果数量 = %d, want 4", len(exec.StepResults))
	}
}

func TestConcurrentAccess(t *testing.T) {
	sw := New()

	// 并发创建工作流
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			workflow := &Workflow{
				ID:   fmt.Sprintf("wf_%d", id),
				Name: fmt.Sprintf("工作流 %d", id),
			}
			sw.CreateWorkflow(workflow)
		}(i)
	}
	wg.Wait()

	workflows := sw.ListWorkflows()
	if len(workflows) != 10 {
		t.Errorf("并发创建后工作流数量 = %d, want 10", len(workflows))
	}
}

func TestWorkflowVariables(t *testing.T) {
	sw := New()

	workflow := &Workflow{
		ID:        "wf_vars",
		Name:      "变量测试",
		Enabled:   true,
		Variables: map[string]string{"env": "production"},
		Steps: []*Step{
			{
				ID:         "step1",
				Name:       "步骤1",
				Action:     "action1",
				Parameters: map[string]string{"key": "value"},
			},
		},
	}
	sw.CreateWorkflow(workflow)

	// 验证变量
	wf, _ := sw.GetWorkflow("wf_vars")
	if wf.Variables["env"] != "production" {
		t.Errorf("工作流变量 = %v, want %v", wf.Variables["env"], "production")
	}

	// 验证步骤参数
	if wf.Steps[0].Parameters["key"] != "value" {
		t.Errorf("步骤参数 = %v, want %v", wf.Steps[0].Parameters["key"], "value")
	}
}
