package dlpengine

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	cfg := &Config{
		Enabled: true,
	}
	engine := NewEngine(cfg)
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
}

func TestNewEngineNilConfig(t *testing.T) {
	engine := NewEngine(nil)
	if engine == nil {
		t.Fatal("NewEngine with nil config returned nil")
	}
}

func TestScanContent(t *testing.T) {
	engine := NewEngine(&Config{Enabled: true})
	result := engine.ScanContent("test content with no sensitive data")
	if result == nil {
		t.Fatal("ScanContent returned nil")
	}
}

func TestGetEngineStatus(t *testing.T) {
	engine := NewEngine(&Config{Enabled: true})
	status := engine.GetStatus()
	if status == nil {
		t.Fatal("GetStatus returned nil")
	}
}
