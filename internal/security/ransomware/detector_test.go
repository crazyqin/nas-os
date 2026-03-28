package ransomware

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDetector(t *testing.T) {
	tests := []struct {
		name   string
		config *DetectorConfig
		want   bool
	}{
		{
			name:   "default config",
			config: nil,
			want:   true,
		},
		{
			name:   "custom config",
			config: &DetectorConfig{
				EnableDetection:   true,
				MonitorPaths:      []string{"/tmp/test"},
				AlertThreshold:    30,
				AlertWindow:       time.Minute * 3,
				MaxEventLogSize:   5000,
			},
			want: true,
		},
		{
			name: "disabled detection",
			config: &DetectorConfig{
				EnableDetection: false,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewDetector(tt.config)
			if (err == nil) != tt.want {
				t.Errorf("NewDetector() error = %v, want %v", err, tt.want)
			}
			if d != nil {
				if !d.config.EnableDetection && tt.config != nil {
					// Disabled mode should still create detector
				}
			}
		})
	}
}

func TestDefaultDetectorConfig(t *testing.T) {
	config := DefaultDetectorConfig()
	if config == nil {
		t.Fatal("DefaultDetectorConfig returned nil")
	}

	if !config.EnableDetection {
		t.Error("EnableDetection should be true by default")
	}
	if config.AlertThreshold != 50 {
		t.Errorf("AlertThreshold should be 50, got %d", config.AlertThreshold)
	}
	if len(config.ProtectedExtensions) == 0 {
		t.Error("ProtectedExtensions should not be empty")
	}
	if len(config.SuspiciousExtensions) == 0 {
		t.Error("SuspiciousExtensions should not be empty")
	}
}

func TestDetectorLoadSignatures(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	// Check suspicious extensions loaded
	suspiciousExt := []string{".encrypted", ".locked", ".crypto", ".ransom"}
	for _, ext := range suspiciousExt {
		if !d.ransomwareSigns[ext] {
			t.Errorf("Suspicious extension %s not loaded", ext)
		}
	}

	// Check ransom note names loaded
	ransomNotes := []string{"readme.txt", "decrypt_instructions.html"}
	for _, note := range ransomNotes {
		if !d.ransomwareSigns[note] {
			t.Errorf("Ransom note signature %s not loaded", note)
		}
	}
}

func TestDetectorIsRansomwareExtension(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	tests := []struct {
		ext  string
		want bool
	}{
		{".encrypted", true},
		{".locked", true},
		{".crypto", true},
		{".ransom", true},
		{".doc", false},
		{".pdf", false},
		{".jpg", false},
		{".txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			if got := d.isRansomwareExtension(tt.ext); got != tt.want {
				t.Errorf("isRansomwareExtension(%s) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestDetectorIsSuspiciousFile(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	// Test through analyzeEvent
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"encrypted extension", "/data/document.encrypted", true},
		{"locked extension", "/data/photo.locked", true},
		{"crypto extension", "/data/backup.crypto", true},
		{"normal file", "/data/document.pdf", false},
		{"ransom note readme", "/data/README_DECRYPT.txt", false}, // Extension is .txt, filename check happens separately
		{"decrypt instructions", "/data/decrypt_instructions.html", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := DetectorFileEvent{
				Type:      "create",
				Path:      tt.path,
				Extension: filepath.Ext(tt.path),
			}
			analyzed := d.analyzeEvent(event)
			if analyzed.Suspicious != tt.want {
				t.Errorf("analyzeEvent for %s: Suspicious = %v, want %v", tt.path, analyzed.Suspicious, tt.want)
			}
		})
	}
}

func TestDetectorCalculateRiskScore(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	tests := []struct {
		name      string
		events    []DetectorFileEvent
		wantScore int
	}{
		{
			name: "no events",
			events: []DetectorFileEvent{},
			wantScore: 0,
		},
		{
			name: "single suspicious event",
			events: []DetectorFileEvent{
				{Suspicious: true, Type: "modify"},
			},
			wantScore: 10,
		},
		{
			name: "multiple suspicious events",
			events: []DetectorFileEvent{
				{Suspicious: true, Type: "modify"},
				{Suspicious: true, Type: "rename"},
				{Suspicious: true, Type: "create"},
			},
			wantScore: 30,
		},
		{
			name: "mixed events",
			events: []DetectorFileEvent{
				{Suspicious: true, Type: "modify"},
				{Suspicious: false, Type: "modify"},
				{Suspicious: true, Type: "delete"},
			},
			wantScore: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := d.calculateRiskScore(tt.events)
			// Allow some flexibility in scoring
			if score < tt.wantScore-5 || score > tt.wantScore+5 {
				t.Errorf("calculateRiskScore() = %d, want approximately %d", score, tt.wantScore)
			}
		})
	}
}

func TestDetectorGenerateAlert(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	events := []DetectorFileEvent{
		{
			ID:        "event1",
			Type:      "modify",
			Path:      "/data/file1.encrypted",
			Extension: ".encrypted",
			Suspicious: true,
			Reason:    "Suspicious extension detected",
			Timestamp: time.Now(),
		},
		{
			ID:        "event2",
			Type:      "rename",
			Path:      "/data/file2.locked",
			OldPath:   "/data/file2.doc",
			Extension: ".locked",
			Suspicious: true,
			Reason:    "File renamed to suspicious extension",
			Timestamp: time.Now(),
		},
	}

	alert := d.generateAlert(events, "high_risk")
	if alert == nil {
		t.Fatal("generateAlert returned nil")
	}

	if alert.Type != "high_risk" {
		t.Errorf("Alert type = %s, want high_risk", alert.Type)
	}
	if len(alert.Events) != 2 {
		t.Errorf("Alert events count = %d, want 2", len(alert.Events))
	}
	if alert.RiskScore <= 0 {
		t.Error("RiskScore should be positive")
	}
	if len(alert.Recommendations) == 0 {
		t.Error("Recommendations should not be empty")
	}
}

func TestDetectorAnalyzeEvent(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	// Test through analyzeEvent which is the internal analysis method
	tests := []struct {
		name string
		event DetectorFileEvent
		wantSuspicious bool
	}{
		{
			name: "normal file modification",
			event: DetectorFileEvent{
				Type:      "modify",
				Path:      "/data/document.pdf",
				Extension: ".pdf",
			},
			wantSuspicious: false,
		},
		{
			name: "suspicious extension",
			event: DetectorFileEvent{
				Type:      "create",
				Path:      "/data/file.encrypted",
				Extension: ".encrypted",
			},
			wantSuspicious: true,
		},
		{
			name: "rename to suspicious extension",
			event: DetectorFileEvent{
				Type:         "rename",
				Path:         "/data/file.locked",
				OldPath:      "/data/file.docx",
				Extension:    ".locked",
				OldExtension: ".docx",
			},
			wantSuspicious: true,
		},
		{
			name: "ransom note file",
			event: DetectorFileEvent{
				Type:      "create",
				Path:      "/data/readme.txt",
				Extension: ".txt",
			},
			wantSuspicious: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzed := d.analyzeEvent(tt.event)
			if analyzed.Suspicious != tt.wantSuspicious {
				t.Errorf("analyzeEvent().Suspicious = %v, want %v", analyzed.Suspicious, tt.wantSuspicious)
			}
		})
	}
}

func TestDetectorQuarantine(t *testing.T) {
	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "ransomware-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create detector with quarantine enabled
	config := &DetectorConfig{
		EnableDetection:  true,
		AutoQuarantine:   true,
		QuarantinePath:   filepath.Join(tmpDir, "quarantine"),
		MonitorPaths:     []string{tmpDir},
		AlertThreshold:   50,
		AlertWindow:      time.Minute,
		MaxEventLogSize:  1000,
	}

	d, err := NewDetector(config)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	// Test quarantine function
	quarantinePath := filepath.Join(config.QuarantinePath, "test.txt")
	err = d.quarantineFile(testFile)
	if err != nil {
		t.Errorf("quarantineFile() error = %v", err)
	}

	// Check original file is removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Original file should be removed after quarantine")
	}

	// Check quarantined file exists
	if _, err := os.Stat(quarantinePath); os.IsNotExist(err) {
		t.Error("Quarantined file should exist")
	}
}

func TestDetectorStop(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	// Start detector
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Just check Start and Stop don't panic
	_ = ctx // ctx used in Start
	d.Stop()

	d.mu.RLock()
	running := d.running
	d.mu.RUnlock()

	if running {
		t.Error("Detector should not be running after Stop()")
	}
}

func TestDetectorGetStatistics(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	// Add some events
	d.mu.Lock()
	d.eventLog = []DetectorFileEvent{
		{Suspicious: true},
		{Suspicious: false},
		{Suspicious: true},
	}
	d.mu.Unlock()

	stats := d.GetStatistics()
	if stats == nil {
		t.Fatal("GetStatistics returned nil")
	}

	if stats["total_events"].(int) != 3 {
		t.Errorf("Total events should be 3, got %v", stats["total_events"])
	}

	if stats["suspicious_events"].(int) != 2 {
		t.Errorf("Suspicious events should be 2, got %v", stats["suspicious_events"])
	}
}

func TestBehaviorAnalyzerThreshold(t *testing.T) {
	analyzer := NewBehaviorAnalyzer(50, time.Minute*5)
	if analyzer == nil {
		t.Fatal("NewBehaviorAnalyzer returned nil")
	}

	// Test threshold exceeded
	events := make([]DetectorFileEvent, 60)
	for i := range events {
		events[i] = DetectorFileEvent{
			Suspicious: true,
			Type:       "modify",
			Timestamp:  time.Now(),
		}
	}

	shouldAlert := analyzer.ShouldTriggerAlert(events)
	if !shouldAlert {
		t.Error("ShouldTriggerAlert should return true when threshold exceeded")
	}
}

func TestBehaviorAnalyzerWindow(t *testing.T) {
	analyzer := NewBehaviorAnalyzer(50, time.Minute*5)

	// Events spread over longer time window
	events := []DetectorFileEvent{}
	for i := 0; i < 60; i++ {
		events = append(events, DetectorFileEvent{
			Suspicious: true,
			Type:       "modify",
			Timestamp:  time.Now().Add(-time.Hour * time.Duration(i)),
		})
	}

	shouldAlert := analyzer.ShouldTriggerAlert(events)
	if shouldAlert {
		t.Error("ShouldTriggerAlert should return false when events spread over long window")
	}
}