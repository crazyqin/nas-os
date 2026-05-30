package collabworkspace

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestHandlerCreateWorkspace(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "Test Workspace", OwnerID: "user1"}
	if err := m.CreateWorkspace(ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	if ws.ID == "" {
		t.Error("workspace ID not set")
	}
}

func TestGetWorkspace(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS1", OwnerID: "u1"}
	m.CreateWorkspace(ws)

	got, err := m.GetWorkspace(ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace failed: %v", err)
	}
	if got.Name != "WS1" {
		t.Errorf("expected WS1, got %s", got.Name)
	}

	_, err = m.GetWorkspace("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestHandlerListWorkspaces(t *testing.T) {
	m := NewManager()
	for i := 0; i < 5; i++ {
		m.CreateWorkspace(&Workspace{Name: "WS", OwnerID: "u1"})
	}
	m.CreateWorkspace(&Workspace{Name: "Other", OwnerID: "u2"})

	all, total := m.ListWorkspaces("", 1, 10)
	if total != 6 {
		t.Errorf("expected 6, got %d", total)
	}
	_ = all

	u1, total := m.ListWorkspaces("u1", 1, 10)
	if total != 5 {
		t.Errorf("expected 5 for u1, got %d", total)
	}
	_ = u1
}

func TestHandlerCreateDocument(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS", OwnerID: "u1"}
	m.CreateWorkspace(ws)

	doc := &CollabDocument{
		WorkspaceID: ws.ID,
		Title:       "Test Doc",
		Content:     "Hello World",
		ContentType: "markdown",
		CreatedBy:   "u1",
	}
	if err := m.CreateDocument(doc); err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}
	if doc.ID == "" {
		t.Error("document ID not set")
	}
	if doc.Version != 1 {
		t.Errorf("expected version 1, got %d", doc.Version)
	}
}

func TestUpdateDocument(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS", OwnerID: "u1"}
	m.CreateWorkspace(ws)
	doc := &CollabDocument{WorkspaceID: ws.ID, Title: "Doc", Content: "v1", CreatedBy: "u1"}
	m.CreateDocument(doc)

	if err := m.UpdateDocument(doc.ID, "u1", "v2"); err != nil {
		t.Fatalf("UpdateDocument failed: %v", err)
	}
	got, _ := m.GetDocument(doc.ID)
	if got.Content != "v2" {
		t.Errorf("expected v2, got %s", got.Content)
	}
	if got.Version != 2 {
		t.Errorf("expected version 2, got %d", got.Version)
	}
}

func TestHandlerDocumentLock(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS", OwnerID: "u1"}
	m.CreateWorkspace(ws)
	doc := &CollabDocument{WorkspaceID: ws.ID, Title: "Doc", CreatedBy: "u1"}
	m.CreateDocument(doc)

	if err := m.LockDocument(doc.ID, "u1"); err != nil {
		t.Fatalf("LockDocument failed: %v", err)
	}
	if err := m.UpdateDocument(doc.ID, "u2", "hack"); err == nil {
		t.Error("should not update locked document by another user")
	}
	if err := m.UnlockDocument(doc.ID, "u1"); err != nil {
		t.Fatalf("UnlockDocument failed: %v", err)
	}
	if err := m.UpdateDocument(doc.ID, "u2", "ok"); err != nil {
		t.Fatalf("UpdateDocument after unlock failed: %v", err)
	}
}

func TestHandlerAddComment(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS", OwnerID: "u1"}
	m.CreateWorkspace(ws)
	doc := &CollabDocument{WorkspaceID: ws.ID, Title: "Doc", CreatedBy: "u1"}
	m.CreateDocument(doc)

	comment := Comment{UserID: "u2", Username: "User2", Content: "Nice!"}
	if err := m.AddComment(doc.ID, comment); err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
	got, _ := m.GetDocument(doc.ID)
	if len(got.Comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(got.Comments))
	}
}

func TestHandlerCreateTask(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS", OwnerID: "u1"}
	m.CreateWorkspace(ws)

	due := time.Now().Add(7 * 24 * time.Hour)
	task := &Task{
		WorkspaceID: ws.ID,
		Title:       "Fix bug",
		Status:      TaskStatusTodo,
		Priority:    3,
		AssigneeID:  "u2",
		CreatorID:   "u1",
		DueDate:     &due,
	}
	if err := m.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.ID == "" {
		t.Error("task ID not set")
	}
}

func TestHandlerUpdateTask(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS", OwnerID: "u1"}
	m.CreateWorkspace(ws)
	task := &Task{WorkspaceID: ws.ID, Title: "Task", Status: "todo", CreatorID: "u1"}
	m.CreateTask(task)

	m.UpdateTask(task.ID, map[string]interface{}{"status": "done"})
	tasks, _ := m.ListTasks(ws.ID, "done", 1, 10)
	if len(tasks) != 1 {
		t.Errorf("expected 1 done task, got %d", len(tasks))
	}
	if tasks[0].CompletedAt == nil {
		t.Error("completedAt not set")
	}
}

func TestHandlerListTasks(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS", OwnerID: "u1"}
	m.CreateWorkspace(ws)
	for i := 0; i < 15; i++ {
		status := "todo"
		if i%2 == 0 {
			status = "done"
		}
		m.CreateTask(&Task{WorkspaceID: ws.ID, Title: "T", Status: TaskStatus(status), CreatorID: "u1"})
	}

	todo, total := m.ListTasks(ws.ID, "todo", 1, 10)
	if total != 7 {
		t.Errorf("expected 7 todo, got %d", total)
	}
	_ = todo
}

func TestWhiteboard(t *testing.T) {
	m := NewManager()
	ws := &Workspace{Name: "WS", OwnerID: "u1"}
	m.CreateWorkspace(ws)

	wb := &Whiteboard{WorkspaceID: ws.ID, Title: "Board", CreatedBy: "u1"}
	if err := m.CreateWhiteboard(wb); err != nil {
		t.Fatalf("CreateWhiteboard failed: %v", err)
	}

	elem := WbElement{Type: "sticky", X: 100, Y: 200, Content: "Note", Color: "yellow", Author: "u1"}
	if err := m.AddWhiteboardElement(wb.ID, elem); err != nil {
		t.Fatalf("AddWhiteboardElement failed: %v", err)
	}

	got, _ := m.GetWhiteboard(wb.ID)
	if len(got.Elements) != 1 {
		t.Errorf("expected 1 element, got %d", len(got.Elements))
	}
}

func TestStats(t *testing.T) {
	m := NewManager()
	m.CreateWorkspace(&Workspace{Name: "WS", OwnerID: "u1"})
	m.CreateDocument(&CollabDocument{Title: "Doc", CreatedBy: "u1"})
	m.CreateTask(&Task{Title: "Task", Status: "todo", CreatorID: "u1"})

	stats := m.GetStats()
	if stats.TotalWorkspaces != 1 {
		t.Errorf("expected 1 workspace, got %d", stats.TotalWorkspaces)
	}
	if stats.TotalDocuments != 1 {
		t.Errorf("expected 1 doc, got %d", stats.TotalDocuments)
	}
	if stats.TotalTasks != 1 {
		t.Errorf("expected 1 task, got %d", stats.TotalTasks)
	}
}
