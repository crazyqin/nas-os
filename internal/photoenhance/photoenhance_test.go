package photoenhance

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:       true,
		MaxConcurrent: 2,
		DefaultQuality: QualityBalance,
		OutputDir:     "/tmp/test-enhance",
		BatchLimit:    10,
	}
	
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	
	if manager.config != config {
		t.Error("Config not set correctly")
	}
}

func TestManagerStartStop(t *testing.T) {
	config := &Config{
		Enabled:       true,
		MaxConcurrent: 2,
	}
	
	manager := NewManager(config)
	
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	manager.Stop()
}

func TestCreateBatchJob(t *testing.T) {
	config := &Config{
		Enabled:       true,
		MaxConcurrent: 2,
	}
	
	manager := NewManager(config)
	
	requests := []*EnhancementRequest{
		{SourcePath: "/tmp/test1.jpg", Type: EnhanceSuperRes},
		{SourcePath: "/tmp/test2.jpg", Type: EnhanceDenoise},
	}
	
	job := manager.CreateBatchJob("Test Batch", requests)
	
	if job == nil {
		t.Fatal("CreateBatchJob returned nil")
	}
	
	if job.Name != "Test Batch" {
		t.Errorf("Expected job name 'Test Batch', got '%s'", job.Name)
	}
	
	if job.TotalCount != 2 {
		t.Errorf("Expected 2 requests, got %d", job.TotalCount)
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{
		Enabled:       true,
		MaxConcurrent: 2,
		GPUEnabled:    true,
	}
	
	manager := NewManager(config)
	
	stats := manager.GetStats()
	
	if stats["gpu_enabled"] != true {
		t.Error("Expected GPU enabled")
	}
	
	if stats["max_concurrent"] != 2 {
		t.Errorf("Expected max_concurrent=2, got %v", stats["max_concurrent"])
	}
}

func TestEnhancementTypes(t *testing.T) {
	tests := []struct {
		name string
		etype EnhancementType
		want string
	}{
		{"Super Resolution", EnhanceSuperRes, "super_resolution"},
		{"Denoise", EnhanceDenoise, "denoise"},
		{"Repair", EnhanceRepair, "repair"},
		{"Colorize", EnhanceColorize, "colorize"},
		{"HDR", EnhanceHDR, "hdr"},
		{"Dehaze", EnhanceDehaze, "dehaze"},
		{"Face Restore", EnhanceFace, "face_restore"},
		{"Background Blur", EnhanceBackground, "background_blur"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.etype) != tt.want {
				t.Errorf("Expected %s, got %s", tt.want, string(tt.etype))
			}
		})
	}
}

func TestQualityLevels(t *testing.T) {
	tests := []struct {
		name  string
		level QualityLevel
		want  string
	}{
		{"Fast", QualityFast, "fast"},
		{"Balance", QualityBalance, "balance"},
		{"Best", QualityBest, "best"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.level) != tt.want {
				t.Errorf("Expected %s, got %s", tt.want, string(tt.level))
			}
		})
	}
}

func TestListJobs(t *testing.T) {
	config := &Config{
		Enabled:       true,
		MaxConcurrent: 2,
	}
	
	manager := NewManager(config)
	
	// Create multiple jobs
	requests1 := []*EnhancementRequest{{SourcePath: "/tmp/test1.jpg"}}
	requests2 := []*EnhancementRequest{{SourcePath: "/tmp/test2.jpg"}}
	
	manager.CreateBatchJob("Job 1", requests1)
	manager.CreateBatchJob("Job 2", requests2)
	
	jobs := manager.ListJobs()
	
	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}
}

func TestGetJob(t *testing.T) {
	config := &Config{
		Enabled:       true,
		MaxConcurrent: 2,
	}
	
	manager := NewManager(config)
	
	requests := []*EnhancementRequest{{SourcePath: "/tmp/test.jpg"}}
	job := manager.CreateBatchJob("Test Job", requests)
	
	// Get existing job
	got, err := manager.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	
	if got.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, got.ID)
	}
	
	// Get non-existing job
	_, err = manager.GetJob("non-existing")
	if err == nil {
		t.Error("Expected error for non-existing job")
	}
}
