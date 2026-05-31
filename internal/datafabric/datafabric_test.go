package datafabric

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.sources == nil {
		t.Error("sources map not initialized")
	}
	if m.placements == nil {
		t.Error("placements map not initialized")
	}
	if m.tasks == nil {
		t.Error("tasks map not initialized")
	}
}

func TestAddSource(t *testing.T) {
	m := NewManager()
	source := &DataSource{
		ID:       "test-1",
		Name:     "Test Source",
		Type:     DataSourceLocal,
		Endpoint: "/mnt/storage",
		Capacity: 1024 * 1024 * 1024 * 100, // 100GB
		Status:   StatusOnline,
	}
	err := m.AddSource(source)
	if err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}
	got, ok := m.GetSource("test-1")
	if !ok {
		t.Fatal("GetSource failed")
	}
	if got.Name != "Test Source" {
		t.Errorf("expected name 'Test Source', got '%s'", got.Name)
	}
}

func TestRemoveSource(t *testing.T) {
	m := NewManager()
	source := &DataSource{
		ID:   "test-1",
		Name: "Test",
		Type: DataSourceLocal,
	}
	m.AddSource(source)
	err := m.RemoveSource("test-1")
	if err != nil {
		t.Fatalf("RemoveSource failed: %v", err)
	}
	_, ok := m.GetSource("test-1")
	if ok {
		t.Error("source should have been removed")
	}
}

func TestListSources(t *testing.T) {
	m := NewManager()
	m.AddSource(&DataSource{ID: "1", Name: "A", Type: DataSourceLocal})
	m.AddSource(&DataSource{ID: "2", Name: "B", Type: DataSourceCloud})
	sources := m.ListSources()
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}
}

func TestAddPlacement(t *testing.T) {
	m := NewManager()
	placement := &DataPlacement{
		ID:       "p1",
		Name:     "Hot Data",
		Priority: 1,
		Enabled:  true,
		Rules: []PlacementRule{
			{ID: "r1", Name: "Recent", TargetID: "ssd", Weight: 1.0, Enabled: true},
		},
	}
	err := m.AddPlacement(placement)
	if err != nil {
		t.Fatalf("AddPlacement failed: %v", err)
	}
}

func TestCreateTask(t *testing.T) {
	m := NewManager()
	task := m.CreateTask(TaskMigrate, "src1", "tgt1", "/data/file.txt")
	if task == nil {
		t.Fatal("CreateTask returned nil")
	}
	if task.Status != TaskPending {
		t.Errorf("expected status 'pending', got '%s'", task.Status)
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()
	m.AddSource(&DataSource{ID: "1", Type: DataSourceLocal, Capacity: 100, Used: 50, Status: StatusOnline})
	m.AddSource(&DataSource{ID: "2", Type: DataSourceCloud, Capacity: 200, Used: 100, Status: StatusOnline})
	stats := m.GetStats()
	if stats.TotalSources != 2 {
		t.Errorf("expected 2 sources, got %d", stats.TotalSources)
	}
	if stats.OnlineSources != 2 {
		t.Errorf("expected 2 online, got %d", stats.OnlineSources)
	}
	if stats.TotalCapacity != 300 {
		t.Errorf("expected 300 capacity, got %d", stats.TotalCapacity)
	}
}
