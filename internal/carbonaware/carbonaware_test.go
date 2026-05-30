package carbonaware

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:           true,
		DefaultRegion:     "CN",
		Strategy:          StrategyLowCarbon,
		MaxCarbonIntensity: 500,
		GreenThreshold:    300,
		CheckInterval:     60,
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN", CarbonIntensity: 581},
			{ID: "US", Name: "United States", Country: "US", CarbonIntensity: 386},
		},
	}
	
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	
	if manager.config.DefaultRegion != "CN" {
		t.Errorf("Expected default region CN, got %s", manager.config.DefaultRegion)
	}
}

func TestManagerStartStop(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		CheckInterval: 1,
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN"},
		},
	}
	
	manager := NewManager(config)
	
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	time.Sleep(100 * time.Millisecond)
	manager.Stop()
}

func TestSubmitTask(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		Strategy:      StrategyLowCarbon,
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN", CarbonIntensity: 581},
		},
	}
	
	manager := NewManager(config)
	
	task := &CarbonAwareTask{
		Name:            "backup-task",
		Description:     "Daily backup",
		Priority:        5,
		EstimatedEnergy: 0.5,
		MaxDelay:        4 * time.Hour,
		Deadline:        time.Now().Add(24 * time.Hour),
		Region:          "CN",
	}
	
	if err := manager.SubmitTask(task); err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}
	
	if task.ID == "" {
		t.Error("Task ID not generated")
	}
	
	if task.Status != TaskStatusPending {
		t.Errorf("Expected status pending, got %s", task.Status)
	}
}

func TestCompleteTask(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		Strategy:      StrategyLowCarbon,
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN", CarbonIntensity: 581},
		},
	}
	
	manager := NewManager(config)
	
	task := &CarbonAwareTask{
		Name:        "backup-task",
		Priority:    5,
		Region:      "CN",
	}
	
	manager.SubmitTask(task)
	
	// 模拟调度
	now := time.Now()
	task.ScheduledAt = &now
	task.Status = TaskStatusScheduled
	
	if err := manager.CompleteTask(task.ID, 0.5); err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}
	
	if task.Status != TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s", task.Status)
	}
	
	if task.CarbonFootprint == nil {
		t.Fatal("CarbonFootprint is nil")
	}
	
	if task.CarbonFootprint.EnergyKWh != 0.5 {
		t.Errorf("Expected energy 0.5, got %f", task.CarbonFootprint.EnergyKWh)
	}
}

func TestGetTask(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN"},
		},
	}
	
	manager := NewManager(config)
	
	task := &CarbonAwareTask{
		Name:   "test-task",
		Region: "CN",
	}
	
	manager.SubmitTask(task)
	
	got, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	
	if got.Name != "test-task" {
		t.Errorf("Expected name test-task, got %s", got.Name)
	}
}

func TestListTasks(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN"},
		},
	}
	
	manager := NewManager(config)
	
	// 提交多个任务
	for i := 0; i < 5; i++ {
		manager.SubmitTask(&CarbonAwareTask{
			Name:   "task",
			Region: "CN",
		})
	}
	
	tasks := manager.ListTasks(TaskStatusPending)
	if len(tasks) != 5 {
		t.Errorf("Expected 5 tasks, got %d", len(tasks))
	}
}

func TestGetCarbonIntensity(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN", CarbonIntensity: 581},
		},
	}
	
	manager := NewManager(config)
	
	// 更新碳强度
	manager.updateCarbonIntensity()
	
	data, err := manager.GetCarbonIntensity("CN")
	if err != nil {
		t.Fatalf("GetCarbonIntensity failed: %v", err)
	}
	
	if data.Region != "CN" {
		t.Errorf("Expected region CN, got %s", data.Region)
	}
}

func TestGetGreenestRegion(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN", CarbonIntensity: 581},
			{ID: "EU", Name: "Europe", Country: "EU", CarbonIntensity: 276},
			{ID: "US", Name: "United States", Country: "US", CarbonIntensity: 386},
		},
	}
	
	manager := NewManager(config)
	
	region := manager.GetGreenestRegion()
	if region == nil {
		t.Fatal("GetGreenestRegion returned nil")
	}
	
	if region.ID != "EU" {
		t.Errorf("Expected EU, got %s", region.ID)
	}
}

func TestGenerateReport(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		GreenThreshold: 300,
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN", CarbonIntensity: 581},
		},
	}
	
	manager := NewManager(config)
	
	// 提交并完成任务
	task := &CarbonAwareTask{
		Name:   "test-task",
		Region: "CN",
	}
	manager.SubmitTask(task)
	
	now := time.Now()
	task.ScheduledAt = &now
	task.Status = TaskStatusScheduled
	manager.CompleteTask(task.ID, 1.0)
	
	// 生成报告
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now().Add(24 * time.Hour)
	
	report := manager.GenerateReport(startDate, endDate)
	
	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}
	
	if report.TotalTasks != 1 {
		t.Errorf("Expected 1 task, got %d", report.TotalTasks)
	}
}

func TestGetDashboard(t *testing.T) {
	config := &CarbonAwareConfig{
		Enabled:       true,
		DefaultRegion: "CN",
		Strategy:      StrategyLowCarbon,
		GreenThreshold: 300,
		Regions: []GridRegion{
			{ID: "CN", Name: "China", Country: "CN"},
		},
	}
	
	manager := NewManager(config)
	
	dashboard := manager.GetDashboard()
	
	if dashboard["total_tasks"] != 0 {
		t.Errorf("Expected 0 total_tasks, got %v", dashboard["total_tasks"])
	}
	
	if dashboard["strategy"] != StrategyLowCarbon {
		t.Errorf("Expected strategy low_carbon, got %v", dashboard["strategy"])
	}
}

func TestSchedulingStrategies(t *testing.T) {
	strategies := []SchedulingStrategy{
		StrategyLowCarbon,
		StrategyGreenFirst,
		StrategyBalanced,
		StrategyCostOptimal,
	}
	
	for _, s := range strategies {
		if string(s) == "" {
			t.Errorf("Empty strategy: %v", s)
		}
	}
}

func TestTaskStatuses(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusScheduled,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}
	
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("Empty status: %v", s)
		}
	}
}
