package ransomware

import (
	"context"
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
			name: "custom config",
			config: &DetectorConfig{
				EnableDetection:   true,
				MonitorPaths:      []string{"/tmp/test"},
				AlertThreshold:    30,
				AlertWindow:       time.Minute * 3,
				MaxEventLogSize:   5000,
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
			if d == nil {
				t.Error("Detector should not be nil")
			}
		})
	}
}

func TestDetectorStartStop(t *testing.T) {
	d, err := NewDetector(&DetectorConfig{
		EnableDetection: true,
		MonitorPaths:    []string{"/tmp"},
		AlertThreshold:  10,
		AlertWindow:     time.Minute,
	})
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start should succeed
	if err := d.Start(ctx); err != nil {
		t.Errorf("Start() error = %v", err)
	}

	// Stop should work
	d.Stop()
}

func TestDetectorAlerts(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	alerts := d.Alerts()
	if alerts == nil {
		t.Error("Alerts() should not return nil")
	}
}

func TestDetectorGetEventLog(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	// GetEventLog with limit 0 should return empty
	log := d.GetEventLog(0)
	if log == nil {
		t.Error("GetEventLog() should not return nil")
	}

	// GetEventLog with limit 10 should return slice
	log = d.GetEventLog(10)
	if log == nil {
		t.Error("GetEventLog() should not return nil")
	}
}

func TestDetectorIsRansomwareExtension(t *testing.T) {
	d, err := NewDetector(nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	tests := []struct {
		ext        string
		suspicious bool
	}{
		{".encrypted", true},
		{".locked", true},
		{".crypto", true},
		{".ransom", true},
		{".pdf", false},
		{".jpg", false},
		{".txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			if got := d.isRansomwareExtension(tt.ext); got != tt.suspicious {
				t.Errorf("isRansomwareExtension(%s) = %v, want %v", tt.ext, got, tt.suspicious)
			}
		})
	}
}