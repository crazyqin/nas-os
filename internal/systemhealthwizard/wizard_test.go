package systemhealthwizard

import (
	"testing"
)

func TestNew(t *testing.T) {
	w := New()
	if w == nil {
		t.Fatal("New() 返回 nil")
	}
	if w.sessions == nil {
		t.Fatal("sessions map 未初始化")
	}
}

func TestStartSession_DefaultSteps(t *testing.T) {
	w := New()
	session, err := w.StartSession(nil)
	if err != nil {
		t.Fatalf("StartSession 失败: %v", err)
	}
	if session.ID == "" {
		t.Error("会话 ID 为空")
	}
	if len(session.Steps) != len(AllSteps()) {
		t.Errorf("步骤数 = %d, 期望 %d", len(session.Steps), len(AllSteps()))
	}
	if session.Status != "running" {
		t.Errorf("状态 = %s, 期望 running", session.Status)
	}
}

func TestStartSession_CustomSteps(t *testing.T) {
	w := New()
	steps := []CheckStep{StepDiskHealth, StepMemoryTest}
	session, err := w.StartSession(steps)
	if err != nil {
		t.Fatalf("StartSession 失败: %v", err)
	}
	if len(session.Steps) != 2 {
		t.Errorf("步骤数 = %d, 期望 2", len(session.Steps))
	}
}

func TestRunNextStep(t *testing.T) {
	w := New()
	session, _ := w.StartSession([]CheckStep{StepDiskHealth, StepMemoryTest})

	result, err := w.RunNextStep(session.ID)
	if err != nil {
		t.Fatalf("RunNextStep 失败: %v", err)
	}
	if result.Step != StepDiskHealth {
		t.Errorf("步骤 = %s, 期望 %s", result.Step, StepDiskHealth)
	}
	if result.Status != StatusPass {
		t.Errorf("状态 = %s, 期望 pass", result.Status)
	}
	if result.Duration == 0 {
		t.Error("执行时间不应为 0")
	}
}

func TestRunAll(t *testing.T) {
	w := New()
	session, _ := w.StartSession([]CheckStep{StepDiskHealth, StepMemoryTest, StepNetworkConnectivity})

	report, err := w.RunAll(session.ID)
	if err != nil {
		t.Fatalf("RunAll 失败: %v", err)
	}
	if report.TotalSteps != 3 {
		t.Errorf("总步骤 = %d, 期望 3", report.TotalSteps)
	}
	if report.PassedSteps != 3 {
		t.Errorf("通过步骤 = %d, 期望 3", report.PassedSteps)
	}
	if report.OverallScore != 100 {
		t.Errorf("评分 = %.1f, 期望 100", report.OverallScore)
	}
	if len(report.Recommendations) == 0 {
		t.Error("建议列表不应为空")
	}
}

func TestGetSession_NotFound(t *testing.T) {
	w := New()
	_, err := w.GetSession("nonexistent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestRunNextStep_AllCompleted(t *testing.T) {
	w := New()
	session, _ := w.StartSession([]CheckStep{StepDiskHealth})

	// 执行唯一的步骤
	w.RunNextStep(session.ID)

	// 再次执行应该返回错误
	_, err := w.RunNextStep(session.ID)
	if err == nil {
		t.Error("所有步骤完成后应返回错误")
	}
}

func TestCalculateScore(t *testing.T) {
	w := New()
	session := &WizardSession{
		Results: map[CheckStep]*StepResult{
			StepDiskHealth:          {Status: StatusPass},
			StepMemoryTest:          {Status: StatusWarn},
			StepNetworkConnectivity: {Status: StatusPass},
		},
	}

	score := w.calculateScore(session)
	// (100 + 60 + 100) / 3 = 86.67
	expected := 86.67
	if score < expected-0.1 || score > expected+0.1 {
		t.Errorf("评分 = %.2f, 期望 ~%.2f", score, expected)
	}
}
