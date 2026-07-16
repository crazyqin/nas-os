package dlpengine

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &DLPConfig{
		Enabled: true,
	}
	mgr := NewManager(logger, cfg)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNewManagerNilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger, nil)
	if mgr == nil {
		t.Fatal("NewManager with nil config returned nil")
	}
}

func TestScanContent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger, &DLPConfig{Enabled: true, MaxContentSize: 100 * 1024 * 1024})
	req := &ScanRequest{
		Content:  []byte("test content with no sensitive data"),
		Resource: "test",
	}
	result, err := mgr.ScanContent(req)
	if err != nil {
		t.Fatalf("ScanContent failed: %v", err)
	}
	if result == nil {
		t.Fatal("ScanContent returned nil")
	}
}
