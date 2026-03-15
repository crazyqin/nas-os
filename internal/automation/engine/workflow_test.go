package engine

import (
	"testing"
	"time"
)

func TestNewWorkflowEngine(t *testing.T) {
	eng := NewWorkflowEngine()
	if eng == nil {
		t.Fatal("NewWorkflowEngine should not return nil")
	}
}

func TestCreateWorkflow(t *testing.T) {
	eng := NewWorkflowEngine()

	wf := &Workflow{
		Name:        "Test Workflow",
		Description: "A test workflow",
		Enabled:     true,
	}

	err := eng.CreateWorkflow(wf)
	if err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	if wf.ID == "" {
		t.Error("Workflow ID should be auto-generated")
	}
}

func TestGetWorkflow(t *testing.T) {
	eng := NewWorkflowEngine()

	wf := &Workflow{
		Name:    "Test Workflow",
		Enabled: true,
	}
	_ = eng.CreateWorkflow(wf)

	got, err := eng.GetWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}

	if got.Name != wf.Name {
		t.Errorf("Expected name %s, got %s", wf.Name, got.Name)
	}
}

func TestGetWorkflowNotFound(t *testing.T) {
	eng := NewWorkflowEngine()

	_, err := eng.GetWorkflow("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent workflow")
	}
}

func TestUpdateWorkflow(t *testing.T) {
	eng := NewWorkflowEngine()

	wf := &Workflow{
		Name:    "Original Name",
		Enabled: true,
	}
	_ = eng.CreateWorkflow(wf)

	updated := &Workflow{
		Name:    "Updated Name",
		Enabled: false,
	}

	err := eng.UpdateWorkflow(wf.ID, updated)
	if err != nil {
		t.Fatalf("UpdateWorkflow failed: %v", err)
	}

	got, _ := eng.GetWorkflow(wf.ID)
	if got.Name != "Updated Name" {
		t.Errorf("Expected name Updated Name, got %s", got.Name)
	}
}

func TestDeleteWorkflow(t *testing.T) {
	eng := NewWorkflowEngine()

	wf := &Workflow{
		Name:    "Test Workflow",
		Enabled: true,
	}
	_ = eng.CreateWorkflow(wf)

	err := eng.DeleteWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("DeleteWorkflow failed: %v", err)
	}

	_, err = eng.GetWorkflow(wf.ID)
	if err == nil {
		t.Error("Workflow should be deleted")
	}
}

func TestListWorkflows(t *testing.T) {
	eng := NewWorkflowEngine()

	// Initially empty
	list := eng.ListWorkflows()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d", len(list))
	}

	// Add some workflows
	_ = eng.CreateWorkflow(&Workflow{Name: "WF1", Enabled: true})
	_ = eng.CreateWorkflow(&Workflow{Name: "WF2", Enabled: false})

	list = eng.ListWorkflows()
	if len(list) != 2 {
		t.Errorf("Expected 2 workflows, got %d", len(list))
	}
}

func TestEnableDisableWorkflow(t *testing.T) {
	eng := NewWorkflowEngine()

	wf := &Workflow{
		Name:    "Test Workflow",
		Enabled: false,
	}
	_ = eng.CreateWorkflow(wf)

	err := eng.EnableWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("EnableWorkflow failed: %v", err)
	}

	got, _ := eng.GetWorkflow(wf.ID)
	if !got.Enabled {
		t.Error("Workflow should be enabled")
	}

	err = eng.DisableWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("DisableWorkflow failed: %v", err)
	}

	got, _ = eng.GetWorkflow(wf.ID)
	if got.Enabled {
		t.Error("Workflow should be disabled")
	}
}

func TestExecutionHistory(t *testing.T) {
	history := NewExecutionHistory(10)

	record := ExecutionRecord{
		WorkflowID: "wf-1",
		StartedAt:  time.Now(),
		Status:     ExecutionStatusSuccess,
	}

	history.AddRecord(record)

	records := history.GetRecords("wf-1", 0)
	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	history.ClearRecords("wf-1")
	records = history.GetRecords("wf-1", 0)
	if len(records) != 0 {
		t.Error("Records should be cleared")
	}
}

func TestExecutionHistoryLimit(t *testing.T) {
	history := NewExecutionHistory(3)

	for i := 0; i < 5; i++ {
		history.AddRecord(ExecutionRecord{
			WorkflowID: "wf-1",
			StartedAt:  time.Now().Add(time.Duration(i) * time.Second),
			Status:     ExecutionStatusSuccess,
		})
	}

	records := history.GetRecords("wf-1", 0)
	if len(records) != 3 {
		t.Errorf("Expected 3 records (limited), got %d", len(records))
	}
}

func TestExportImportWorkflow(t *testing.T) {
	eng := NewWorkflowEngine()

	wf := &Workflow{
		Name:         "Test Workflow",
		Description:  "Test description",
		Enabled:      true,
		RunCount:     10,
		SuccessCount: 8,
		FailCount:    2,
	}
	_ = eng.CreateWorkflow(wf)

	// Export
	data, err := eng.ExportWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("ExportWorkflow failed: %v", err)
	}

	// Import
	imported, err := eng.ImportWorkflow(data)
	if err != nil {
		t.Fatalf("ImportWorkflow failed: %v", err)
	}

	if imported.Name != wf.Name {
		t.Errorf("Expected name %s, got %s", wf.Name, imported.Name)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == id2 {
		t.Error("IDs should be unique")
	}

	if len(id1) < 3 {
		t.Errorf("ID too short: %s", id1)
	}
}
