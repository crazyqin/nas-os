package containerscanner

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNewManagerNilLogger(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("NewManager with nil logger returned nil")
	}
}
