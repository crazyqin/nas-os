package containerscanner

import (
	"testing"
)

func TestNewScanner(t *testing.T) {
	cfg := &Config{
		Enabled: true,
	}
	scanner := NewScanner(cfg)
	if scanner == nil {
		t.Fatal("NewScanner returned nil")
	}
}

func TestNewScannerNilConfig(t *testing.T) {
	scanner := NewScanner(nil)
	if scanner == nil {
		t.Fatal("NewScanner with nil config returned nil")
	}
}

func TestScanStatus(t *testing.T) {
	scanner := NewScanner(&Config{Enabled: true})
	status := scanner.GetStatus()
	if status == nil {
		t.Fatal("GetStatus returned nil")
	}
}

func TestListContainers(t *testing.T) {
	scanner := NewScanner(&Config{Enabled: true})
	containers := scanner.ListContainers()
	if containers == nil {
		t.Fatal("ListContainers returned nil")
	}
}
