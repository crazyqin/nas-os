package opsplaybook

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestCreatePlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:       "pb-001",
		Name:     "Disk Health Check",
		Category: "storage",
		Severity: SeverityHigh,
		Steps: []Step{
			{ID: "s1", Name: "Check SMART", Type: StepCommand, Command: "smartctl -a /dev/sda"},
			{ID: "s2", Name: "Check Space", Type: StepCommand, Command: "df -h"},
		},
		RequiresApproval:  false,
		SLATargetMinutes:  30,
		CreatedBy:         "admin",
	}

	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}

	got, err := m.GetPlaybook(ctx, "pb-001")
	if err != nil {
		t.Fatalf("GetPlaybook failed: %v", err)
	}
	if got.Name != "Disk Health Check" {
		t.Errorf("expected name 'Disk Health Check', got '%s'", got.Name)
	}
	if got.Status != StatusDraft {
		t.Errorf("expected status draft, got %s", got.Status)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
}

func TestCreatePlaybookValidation(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	// Missing name
	err := m.CreatePlaybook(ctx, &Playbook{ID: "pb-x", Steps: []Step{{ID: "s1", Type: StepCommand}}})
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Missing steps
	err = m.CreatePlaybook(ctx, &Playbook{ID: "pb-y", Name: "test"})
	if err == nil {
		t.Error("expected error for missing steps")
	}
}

func TestCreateDuplicatePlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:   "pb-dup",
		Name: "Test",
		Steps: []Step{{ID: "s1", Type: StepCommand, Command: "echo hi"}},
	}
	_ = m.CreatePlaybook(ctx, pb)

	err := m.CreatePlaybook(ctx, pb)
	if err == nil {
		t.Error("expected error for duplicate playbook")
	}
}

func TestPublishPlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:   "pb-pub",
		Name: "Publish Test",
		Steps: []Step{{ID: "s1", Type: StepCommand, Command: "echo ok"}},
	}
	_ = m.CreatePlaybook(ctx, pb)

	if err := m.PublishPlaybook(ctx, "pb-pub"); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}

	got, _ := m.GetPlaybook(ctx, "pb-pub")
	if got.Status != StatusPublished {
		t.Errorf("expected published, got %s", got.Status)
	}

	// Cannot publish again
	if err := m.PublishPlaybook(ctx, "pb-pub"); err == nil {
		t.Error("expected error publishing non-draft playbook")
	}
}

func TestArchivePlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:     "pb-arch",
		Name:   "Archive Test",
		Steps:  []Step{{ID: "s1", Type: StepCommand, Command: "echo ok"}},
		Status: StatusPublished,
	}
	_ = m.CreatePlaybook(ctx, pb)

	if err := m.ArchivePlaybook(ctx, "pb-arch"); err != nil {
		t.Fatalf("ArchivePlaybook failed: %v", err)
	}

	got, _ := m.GetPlaybook(ctx, "pb-arch")
	if got.Status != StatusArchived {
		t.Errorf("expected archived, got %s", got.Status)
	}
}

func TestUpdatePlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:   "pb-upd",
		Name: "Original",
		Steps: []Step{{ID: "s1", Type: StepCommand, Command: "echo v1"}},
	}
	_ = m.CreatePlaybook(ctx, pb)

	pb.Name = "Updated"
	pb.Steps = []Step{{ID: "s1", Type: StepCommand, Command: "echo v2"}}
	if err := m.UpdatePlaybook(ctx, pb); err != nil {
		t.Fatalf("UpdatePlaybook failed: %v", err)
	}

	got, _ := m.GetPlaybook(ctx, "pb-upd")
	if got.Name != "Updated" {
		t.Errorf("expected 'Updated', got '%s'", got.Name)
	}
	if got.Version != 2 {
		t.Errorf("expected version 2, got %d", got.Version)
	}
}

func TestDeletePlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:    "pb-del",
		Name:  "Delete Test",
		Steps: []Step{{ID: "s1", Type: StepCommand, Command: "echo bye"}},
	}
	_ = m.CreatePlaybook(ctx, pb)

	if err := m.DeletePlaybook(ctx, "pb-del"); err != nil {
		t.Fatalf("DeletePlaybook failed: %v", err)
	}

	_, err := m.GetPlaybook(ctx, "pb-del")
	if err == nil {
		t.Error("expected error for deleted playbook")
	}
}

func TestListPlaybooks(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	_ = m.CreatePlaybook(ctx, &Playbook{ID: "pb-a", Name: "A", Category: "storage", Steps: []Step{{ID: "s1", Type: StepCommand}}})
	_ = m.CreatePlaybook(ctx, &Playbook{ID: "pb-b", Name: "B", Category: "network", Steps: []Step{{ID: "s1", Type: StepCommand}}})
	_ = m.CreatePlaybook(ctx, &Playbook{ID: "pb-c", Name: "C", Category: "storage", Steps: []Step{{ID: "s1", Type: StepCommand}}})

	// All
	all := m.ListPlaybooks(ctx, "", "")
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	// Filter by category
	storage := m.ListPlaybooks(ctx, "storage", "")
	if len(storage) != 2 {
		t.Errorf("expected 2 storage, got %d", len(storage))
	}
}

func executePublished(t *testing.T, m *Manager, ctx context.Context, pb *Playbook) *Execution {
	t.Helper()
	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}
	if err := m.PublishPlaybook(ctx, pb.ID); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}
	exec, err := m.ExecutePlaybook(ctx, pb.ID, "admin", nil)
	if err != nil {
		t.Fatalf("ExecutePlaybook failed: %v", err)
	}
	return exec
}

func TestExecutePlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:               "pb-exec",
		Name:             "Execute Test",
		Steps: []Step{
			{ID: "s1", Name: "Step 1", Type: StepCommand, Command: "echo hello"},
			{ID: "s2", Name: "Step 2", Type: StepCheck, Command: "check disk"},
		},
		SLATargetMinutes: 5,
	}
	exec := executePublished(t, m, ctx, pb)

	if exec.Status != ExecSuccess {
		t.Errorf("expected success, got %s", exec.Status)
	}
	if len(exec.StepResults) != 2 {
		t.Errorf("expected 2 step results, got %d", len(exec.StepResults))
	}
	if exec.DurationMs < 0 {
		t.Error("duration should be non-negative")
	}
}

func TestExecuteUnpublishedPlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:   "pb-draft",
		Name: "Draft",
		Steps: []Step{{ID: "s1", Type: StepCommand, Command: "echo hi"}},
	}
	_ = m.CreatePlaybook(ctx, pb)

	_, err := m.ExecutePlaybook(ctx, "pb-draft", "admin", nil)
	if err == nil {
		t.Error("expected error executing draft playbook")
	}
}

func TestExecuteWithApproval(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:               "pb-approval",
		Name:             "Approval Test",
		RequiresApproval: true,
		Steps: []Step{
			{ID: "s1", Name: "Dangerous Step", Type: StepCommand, Command: "rm -rf /tmp/test"},
		},
	}
	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}
	if err := m.PublishPlaybook(ctx, pb.ID); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}

	exec, err := m.ExecutePlaybook(ctx, "pb-approval", "operator", nil)
	if err != nil {
		t.Fatalf("ExecutePlaybook failed: %v", err)
	}

	if exec.Status != ExecPending {
		t.Errorf("expected pending, got %s", exec.Status)
	}
	if exec.Approval == nil {
		t.Fatal("expected approval record")
	}

	// Approve
	if err := m.ApproveExecution(ctx, exec.ID, "admin", "approved for testing"); err != nil {
		t.Fatalf("ApproveExecution failed: %v", err)
	}

	approved, _ := m.GetExecution(ctx, exec.ID)
	if approved.Status != ExecSuccess {
		t.Errorf("expected success after approval, got %s", approved.Status)
	}
	if !approved.Approval.Approved {
		t.Error("expected approval to be recorded")
	}
}

func TestRejectExecution(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:               "pb-reject",
		Name:             "Reject Test",
		RequiresApproval: true,
		Steps:            []Step{{ID: "s1", Type: StepCommand, Command: "echo no"}},
	}
	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}
	if err := m.PublishPlaybook(ctx, pb.ID); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}

	exec, err := m.ExecutePlaybook(ctx, "pb-reject", "operator", nil)
	if err != nil {
		t.Fatalf("ExecutePlaybook failed: %v", err)
	}

	if err := m.RejectExecution(ctx, exec.ID, "admin", "too risky"); err != nil {
		t.Fatalf("RejectExecution failed: %v", err)
	}

	rejected, _ := m.GetExecution(ctx, exec.ID)
	if rejected.Status != ExecRejected {
		t.Errorf("expected rejected, got %s", rejected.Status)
	}
}

func TestCancelExecution(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:     "pb-cancel",
		Name:   "Cancel Test",
		Steps:  []Step{{ID: "s1", Type: StepWait, Timeout: 60}},
	}
	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}
	if err := m.PublishPlaybook(ctx, pb.ID); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}

	exec, err := m.ExecutePlaybook(ctx, "pb-cancel", "admin", nil)
	if err != nil {
		t.Fatalf("ExecutePlaybook failed: %v", err)
	}
	if exec.Status == ExecSuccess || exec.Status == ExecRunning || exec.Status == ExecPending {
		// Cancel it
		if err := m.CancelExecution(ctx, exec.ID); err != nil {
			// May already be done
			t.Logf("CancelExecution: %v", err)
		}
	}
}

func TestSLAReport(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:               "pb-sla",
		Name:             "SLA Test",
		SLATargetMinutes: 1, // 1 minute target
		Steps:            []Step{{ID: "s1", Type: StepCommand, Command: "echo fast"}},
	}
	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}
	if err := m.PublishPlaybook(ctx, pb.ID); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}

	// Execute a few times
	for i := 0; i < 5; i++ {
		_, _ = m.ExecutePlaybook(ctx, "pb-sla", "admin", nil)
	}

	report, err := m.GenerateSLAReport(ctx, "pb-sla")
	if err != nil {
		t.Fatalf("GenerateSLAReport failed: %v", err)
	}

	if report.TotalExecutions != 5 {
		t.Errorf("expected 5 executions, got %d", report.TotalExecutions)
	}
	if report.SLACompliancePct < 0 || report.SLACompliancePct > 100 {
		t.Errorf("invalid SLA compliance: %.2f", report.SLACompliancePct)
	}
}

func TestKnowledgeBase(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	entry := &KnowledgeEntry{
		ID:         "kb-001",
		PlaybookID: "pb-001",
		Title:      "How to Check Disk Health",
		Content:    "Use smartctl to check SMART data...",
		Category:   "storage",
		Tags:       []string{"disk", "health", "smart"},
		Author:     "admin",
	}

	if err := m.AddKnowledgeEntry(ctx, entry); err != nil {
		t.Fatalf("AddKnowledgeEntry failed: %v", err)
	}

	got, err := m.GetKnowledgeEntry(ctx, "kb-001")
	if err != nil {
		t.Fatalf("GetKnowledgeEntry failed: %v", err)
	}
	if got.Title != "How to Check Disk Health" {
		t.Errorf("expected title 'How to Check Disk Health', got '%s'", got.Title)
	}

	// List by playbook
	entries := m.ListKnowledgeEntries(ctx, "pb-001")
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	// Mark helpful
	if err := m.MarkHelpful(ctx, "kb-001"); err != nil {
		t.Fatalf("MarkHelpful failed: %v", err)
	}
	got, _ = m.GetKnowledgeEntry(ctx, "kb-001")
	if got.Helpful != 1 {
		t.Errorf("expected helpful count 1, got %d", got.Helpful)
	}
}

func TestExportImportPlaybook(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:       "pb-export",
		Name:     "Export Test",
		Category: "test",
		Severity: SeverityMedium,
		Steps: []Step{
			{ID: "s1", Name: "Step 1", Type: StepCommand, Command: "echo export"},
		},
		CreatedBy: "admin",
	}
	_ = m.CreatePlaybook(ctx, pb)

	data, err := m.ExportPlaybook(ctx, "pb-export")
	if err != nil {
		t.Fatalf("ExportPlaybook failed: %v", err)
	}

	// Delete original to allow import with same ID
	_ = m.DeletePlaybook(ctx, "pb-export")

	// Import as new
	imported, err := m.ImportPlaybook(ctx, data)
	if err != nil {
		t.Fatalf("ImportPlaybook failed: %v", err)
	}
	if imported.Name != "Export Test" {
		t.Errorf("expected 'Export Test', got '%s'", imported.Name)
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:    "pb-concurrent",
		Name:  "Concurrent Test",
		Steps: []Step{{ID: "s1", Type: StepCommand, Command: "echo concurrent"}},
	}
	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}
	if err := m.PublishPlaybook(ctx, pb.ID); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = m.ExecutePlaybook(ctx, "pb-concurrent", "user", nil)
		}()
	}
	wg.Wait()

	// Verify no panic and executions recorded
	execs := m.ListExecutions(ctx, "pb-concurrent")
	if len(execs) != 10 {
		t.Errorf("expected 10 executions, got %d", len(execs))
	}
}

func TestPlaybookWithDependencies(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:   "pb-deps",
		Name: "Dependency Test",
		Steps: []Step{
			{ID: "s1", Name: "Prepare", Type: StepCommand, Command: "echo prep"},
			{ID: "s2", Name: "Execute", Type: StepCommand, Command: "echo exec", DependsOn: []string{"s1"}},
			{ID: "s3", Name: "Verify", Type: StepCheck, Command: "echo verify", DependsOn: []string{"s2"}},
		},
	}
	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}
	if err := m.PublishPlaybook(ctx, pb.ID); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}

	exec, err := m.ExecutePlaybook(ctx, "pb-deps", "admin", nil)
	if err != nil {
		t.Fatalf("ExecutePlaybook failed: %v", err)
	}
	if exec.Status != ExecSuccess {
		t.Errorf("expected success, got %s", exec.Status)
	}
}

func TestPlaybookStats(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:   "pb-stats",
		Name: "Stats Test",
		Steps: []Step{{ID: "s1", Type: StepCommand, Command: "echo ok"}},
	}
	if err := m.CreatePlaybook(ctx, pb); err != nil {
		t.Fatalf("CreatePlaybook failed: %v", err)
	}
	if err := m.PublishPlaybook(ctx, pb.ID); err != nil {
		t.Fatalf("PublishPlaybook failed: %v", err)
	}

	// Run multiple times
	for i := 0; i < 3; i++ {
		_, _ = m.ExecutePlaybook(ctx, "pb-stats", "admin", nil)
	}

	got, _ := m.GetPlaybook(ctx, "pb-stats")
	if got.RunCount != 3 {
		t.Errorf("expected 3 runs, got %d", got.RunCount)
	}
	if got.SuccessRate != 100 {
		t.Errorf("expected 100%% success rate, got %.1f%%", got.SuccessRate)
	}
}

// BenchmarkCreatePlaybook benchmarks playbook creation.
func BenchmarkCreatePlaybook(b *testing.B) {
	m := NewManager()
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		pb := &Playbook{
			ID:   fmt.Sprintf("pb-bench-%d", i),
			Name: "Benchmark",
			Steps: []Step{
				{ID: "s1", Type: StepCommand, Command: "echo bench"},
			},
		}
		_ = m.CreatePlaybook(ctx, pb)
	}
}

// BenchmarkExecutePlaybook benchmarks playbook execution.
func BenchmarkExecutePlaybook(b *testing.B) {
	m := NewManager()
	ctx := context.Background()

	pb := &Playbook{
		ID:   "pb-bench-exec",
		Name: "Benchmark Exec",
		Steps: []Step{
			{ID: "s1", Type: StepCommand, Command: "echo 1"},
			{ID: "s2", Type: StepCommand, Command: "echo 2"},
			{ID: "s3", Type: StepCheck, Command: "check"},
		},
	}
	_ = m.CreatePlaybook(ctx, pb)
	_ = m.PublishPlaybook(ctx, pb.ID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.ExecutePlaybook(ctx, "pb-bench-exec", "admin", nil)
	}
}


