package crossnasreplication

import (
	"testing"
)

func TestNodeRegistration(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterNode(&RemoteNode{
		ID:       "node1",
		Name:     "NAS-Primary",
		Host:     "192.168.1.100",
		Port:     22,
		Protocol: "ssh",
		Status:   "online",
	})
	
	nodes := mgr.GetNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "node1" {
		t.Errorf("expected node1, got %s", nodes[0].ID)
	}
}

func TestNodeRemoval(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterNode(&RemoteNode{ID: "node1", Name: "test"})
	removed := mgr.RemoveNode("node1")
	if !removed {
		t.Error("expected node to be removed")
	}
	
	nodes := mgr.GetNodes()
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestTaskCreation(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterNode(&RemoteNode{ID: "src", Name: "source"})
	mgr.RegisterNode(&RemoteNode{ID: "dst", Name: "target"})
	
	task := &ReplicationTask{
		ID:         "task1",
		Name:       "Backup Photos",
		SourceNode: "src",
		SourcePath: "/photos",
		TargetNode: "dst",
		TargetPath: "/backup/photos",
		Enabled:    true,
		Compress:   true,
		Encrypt:    true,
	}
	
	err := mgr.CreateTask(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	tasks := mgr.GetTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].State != StatePending {
		t.Errorf("expected pending state, got %s", tasks[0].State)
	}
}

func TestTaskCreationInvalidNode(t *testing.T) {
	mgr := NewManager(nil)
	
	task := &ReplicationTask{
		ID:         "task1",
		SourceNode: "nonexistent",
		TargetNode: "also_nonexistent",
	}
	
	err := mgr.CreateTask(task)
	if err == nil {
		t.Error("expected error for nonexistent nodes")
	}
}

func TestSyncExecution(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterNode(&RemoteNode{ID: "src", Name: "source"})
	mgr.RegisterNode(&RemoteNode{ID: "dst", Name: "target"})
	
	task := &ReplicationTask{
		ID:         "task1",
		Name:       "test sync",
		SourceNode: "src",
		SourcePath: "/data",
		TargetNode: "dst",
		TargetPath: "/backup",
	}
	mgr.CreateTask(task)
	
	result, err := mgr.StartSync("task1")
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if result.State != StateCompleted {
		t.Errorf("expected completed, got %s", result.State)
	}
	if result.BytesSynced <= 0 {
		t.Error("expected bytes synced > 0")
	}
}

func TestSyncAlreadyRunning(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterNode(&RemoteNode{ID: "src"})
	mgr.RegisterNode(&RemoteNode{ID: "dst"})
	
	task := &ReplicationTask{
		ID:         "task1",
		SourceNode: "src",
		TargetNode: "dst",
		State:      StateRunning,
	}
	mgr.CreateTask(task)
	
	// Force state to running
	mgr.mu.Lock()
	mgr.tasks["task1"].State = StateRunning
	mgr.mu.Unlock()
	
	_, err := mgr.StartSync("task1")
	if err == nil {
		t.Error("expected error for already running task")
	}
}

func TestTaskDeletion(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterNode(&RemoteNode{ID: "src"})
	mgr.RegisterNode(&RemoteNode{ID: "dst"})
	
	task := &ReplicationTask{
		ID:         "task1",
		SourceNode: "src",
		TargetNode: "dst",
	}
	mgr.CreateTask(task)
	
	deleted := mgr.DeleteTask("task1")
	if !deleted {
		t.Error("expected task to be deleted")
	}
	
	tasks := mgr.GetTasks()
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestReplicationStats(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterNode(&RemoteNode{ID: "n1"})
	mgr.RegisterNode(&RemoteNode{ID: "n2"})
	
	task := &ReplicationTask{
		ID:         "task1",
		SourceNode: "n1",
		TargetNode: "n2",
	}
	mgr.CreateTask(task)
	
	stats := mgr.GetReplicationStats()
	if stats["total_tasks"] != 1 {
		t.Errorf("expected 1 task, got %v", stats["total_tasks"])
	}
	if stats["total_nodes"] != 2 {
		t.Errorf("expected 2 nodes, got %v", stats["total_nodes"])
	}
}

func TestSyncResults(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterNode(&RemoteNode{ID: "src"})
	mgr.RegisterNode(&RemoteNode{ID: "dst"})
	
	task := &ReplicationTask{
		ID:         "task1",
		SourceNode: "src",
		TargetNode: "dst",
	}
	mgr.CreateTask(task)
	
	mgr.StartSync("task1")
	
	results := mgr.GetTaskResults("task1")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Throughput < 0 {
		t.Error("throughput should not be negative")
	}
}
