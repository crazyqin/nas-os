package alertguided

import (
	"testing"

	"go.uber.org/zap"
)

func TestGuideEngine_GetGuide(t *testing.T) {
	kb := NewKnowledgeBase()
	ge := NewGuideEngine(kb, zap.NewNop())

	alert := &GuidedAlert{
		ID:       "AG-1",
		Title:    "disk_space_low",
		Message:  "磁盘使用率 95%",
		Category: CategoryStorage,
		Severity: SeverityWarning,
	}

	guide := ge.GetGuide(alert)
	if guide == nil {
		t.Fatal("expected guide, got nil")
	}
	if guide.KnowledgeID != "disk_space_low" {
		t.Errorf("expected knowledgeId disk_space_low, got %s", guide.KnowledgeID)
	}
	if guide.AlertID != "AG-1" {
		t.Errorf("expected alertId AG-1, got %s", guide.AlertID)
	}
	if len(guide.Causes) == 0 {
		t.Error("expected causes")
	}
	if len(guide.Steps) == 0 {
		t.Error("expected steps")
	}
	// 步骤初始状态应为 PENDING
	for _, step := range guide.Steps {
		if step.Status != StepStatusPending {
			t.Errorf("expected step %d status PENDING, got %s", step.Order, step.Status)
		}
	}
}

func TestGuideEngine_GetGuideByKnowledgeID(t *testing.T) {
	kb := NewKnowledgeBase()
	ge := NewGuideEngine(kb, zap.NewNop())

	guide, err := ge.GetGuideByKnowledgeID("disk_smart_warning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if guide.Title != "磁盘 SMART 异常" {
		t.Errorf("expected title 磁盘 SMART 异常, got %s", guide.Title)
	}
}

func TestGuideEngine_GetGuideByKnowledgeIDNotFound(t *testing.T) {
	kb := NewKnowledgeBase()
	ge := NewGuideEngine(kb, zap.NewNop())

	_, err := ge.GetGuideByKnowledgeID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestGuideEngine_GetGuideFallback(t *testing.T) {
	kb := NewKnowledgeBase()
	ge := NewGuideEngine(kb, zap.NewNop())

	// 未知告警，应回退到类别匹配或通用指引
	alert := &GuidedAlert{
		ID:       "AG-99",
		Title:    "unknown_alert",
		Message:  "unknown",
		Category: CategoryStorage,
		Severity: SeverityWarning,
	}

	guide := ge.GetGuide(alert)
	if guide == nil {
		t.Fatal("expected guide, got nil")
	}
	// 应该匹配到 storage 类别的知识条目
	if guide.KnowledgeID == "" {
		t.Error("expected knowledgeId to be set from category match")
	}
}

func TestGuideEngine_SearchGuides(t *testing.T) {
	kb := NewKnowledgeBase()
	ge := NewGuideEngine(kb, zap.NewNop())

	guides := ge.SearchGuides("磁盘")
	if len(guides) < 3 {
		t.Errorf("expected at least 3 guides for '磁盘', got %d", len(guides))
	}
}

func TestGuideTracker_StartRepair(t *testing.T) {
	gt := NewGuideTracker(zap.NewNop())
	progress := gt.StartRepair("AG-1", "disk_space_low", 5)
	if progress == nil {
		t.Fatal("expected progress, got nil")
	}
	if progress.TotalSteps != 5 {
		t.Errorf("expected 5 total steps, got %d", progress.TotalSteps)
	}
	if progress.CompletedSteps != 0 {
		t.Errorf("expected 0 completed steps, got %d", progress.CompletedSteps)
	}
}

func TestGuideTracker_UpdateStep(t *testing.T) {
	gt := NewGuideTracker(zap.NewNop())
	gt.StartRepair("AG-1", "disk_space_low", 3)

	err := gt.UpdateStep("AG-1", 1, StepStatusCompleted, "done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	progress, ok := gt.GetProgress("AG-1")
	if !ok {
		t.Fatal("expected progress to exist")
	}
	if progress.CompletedSteps != 1 {
		t.Errorf("expected 1 completed step, got %d", progress.CompletedSteps)
	}
	if progress.StepStatuses[1] != StepStatusCompleted {
		t.Errorf("expected step 1 completed, got %s", progress.StepStatuses[1])
	}
}

func TestGuideTracker_UpdateStepNotFound(t *testing.T) {
	gt := NewGuideTracker(zap.NewNop())
	err := gt.UpdateStep("nonexistent", 1, StepStatusCompleted, "")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestGuideTracker_IsComplete(t *testing.T) {
	gt := NewGuideTracker(zap.NewNop())
	gt.StartRepair("AG-1", "test", 3)

	if gt.IsComplete("AG-1") {
		t.Error("expected not complete")
	}

	gt.UpdateStep("AG-1", 1, StepStatusCompleted, "")
	gt.UpdateStep("AG-1", 2, StepStatusCompleted, "")
	gt.UpdateStep("AG-1", 3, StepStatusSkipped, "")

	if !gt.IsComplete("AG-1") {
		t.Error("expected complete after all steps done/skipped")
	}
}

func TestGuideTracker_ProgressPercent(t *testing.T) {
	gt := NewGuideTracker(zap.NewNop())
	gt.StartRepair("AG-1", "test", 4)

	pct := gt.GetProgressPercent("AG-1")
	if pct != 0 {
		t.Errorf("expected 0%%, got %.1f%%", pct)
	}

	gt.UpdateStep("AG-1", 1, StepStatusCompleted, "")
	gt.UpdateStep("AG-1", 2, StepStatusCompleted, "")

	pct = gt.GetProgressPercent("AG-1")
	if pct != 50 {
		t.Errorf("expected 50%%, got %.1f%%", pct)
	}
}

func TestGuideTracker_ListActive(t *testing.T) {
	gt := NewGuideTracker(zap.NewNop())
	gt.StartRepair("AG-1", "test1", 3)
	gt.StartRepair("AG-2", "test2", 2)
	gt.UpdateStep("AG-2", 1, StepStatusCompleted, "")
	gt.UpdateStep("AG-2", 2, StepStatusCompleted, "")

	active := gt.ListActive()
	if len(active) != 1 {
		t.Errorf("expected 1 active repair, got %d", len(active))
	}
}

func TestRepairTracker_Basic(t *testing.T) {
	rt := NewRepairTracker(zap.NewNop())

	record := rt.Start("AG-1", "disk_space_low", "磁盘空间不足修复", 3, "admin")
	if record == nil {
		t.Fatal("expected record, got nil")
	}
	if record.Status != RepairStatusInProgress {
		t.Errorf("expected status IN_PROGRESS, got %s", record.Status)
	}

	// 完成步骤1
	err := rt.CompleteStep("AG-1", 1, "df -h output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 完成步骤2
	err = rt.CompleteStep("AG-1", 2, "cleanup done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 跳过步骤3
	err = rt.SkipStep("AG-1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应该已完成
	record, ok := rt.Get("AG-1")
	if !ok {
		t.Fatal("expected record to exist")
	}
	if record.Status != RepairStatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", record.Status)
	}

	pct := rt.ProgressPercent("AG-1")
	if pct != 100 {
		t.Errorf("expected 100%%, got %.1f%%", pct)
	}
}

func TestRepairTracker_FailStep(t *testing.T) {
	rt := NewRepairTracker(zap.NewNop())
	rt.Start("AG-1", "test", "test repair", 3, "")

	err := rt.FailStep("AG-1", 1, "command failed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, _ := rt.Get("AG-1")
	if record.Status != RepairStatusFailed {
		t.Errorf("expected status FAILED, got %s", record.Status)
	}
	if record.StepRecords[1].Error != "command failed" {
		t.Errorf("expected error message, got %s", record.StepRecords[1].Error)
	}
}

func TestRepairTracker_AddNote(t *testing.T) {
	rt := NewRepairTracker(zap.NewNop())
	rt.Start("AG-1", "test", "test", 2, "")

	err := rt.AddNote("AG-1", "已检查日志", "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, _ := rt.Get("AG-1")
	if len(record.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(record.Notes))
	}
	if record.Notes[0].Content != "已检查日志" {
		t.Errorf("expected note content, got %s", record.Notes[0].Content)
	}
}

func TestRepairTracker_Abandon(t *testing.T) {
	rt := NewRepairTracker(zap.NewNop())
	rt.Start("AG-1", "test", "test", 2, "")

	err := rt.Abandon("AG-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, _ := rt.Get("AG-1")
	if record.Status != RepairStatusAbandoned {
		t.Errorf("expected status ABANDONED, got %s", record.Status)
	}
}

func TestRepairTracker_ListActive(t *testing.T) {
	rt := NewRepairTracker(zap.NewNop())
	rt.Start("AG-1", "test1", "repair 1", 2, "")
	rt.Start("AG-2", "test2", "repair 2", 2, "")
	rt.CompleteStep("AG-2", 1, "")
	rt.CompleteStep("AG-2", 2, "")

	active := rt.ListActive()
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
}

func TestRepairTracker_ListAll(t *testing.T) {
	rt := NewRepairTracker(zap.NewNop())
	rt.Start("AG-1", "test1", "repair 1", 2, "")
	rt.Start("AG-2", "test2", "repair 2", 2, "")

	all := rt.ListAll()
	if len(all) != 2 {
		t.Errorf("expected 2 total, got %d", len(all))
	}
}

func TestRepairTracker_Remove(t *testing.T) {
	rt := NewRepairTracker(zap.NewNop())
	rt.Start("AG-1", "test", "test", 2, "")
	rt.Remove("AG-1")

	_, ok := rt.Get("AG-1")
	if ok {
		t.Error("expected record to be removed")
	}
}

func TestRepairTracker_NotFound(t *testing.T) {
	rt := NewRepairTracker(zap.NewNop())

	err := rt.CompleteStep("nonexistent", 1, "")
	if err == nil {
		t.Error("expected error")
	}
	err = rt.FailStep("nonexistent", 1, "err")
	if err == nil {
		t.Error("expected error")
	}
	err = rt.SkipStep("nonexistent", 1)
	if err == nil {
		t.Error("expected error")
	}
	err = rt.AddNote("nonexistent", "note", "user")
	if err == nil {
		t.Error("expected error")
	}
	err = rt.Abandon("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}
