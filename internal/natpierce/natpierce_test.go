package natpierce

import (
	"testing"
)

func TestNewPierceClient(t *testing.T) {
	cfg := &Config{
		Enabled:    true,
		Mode:       ModeRelay,
		ServerAddr: "relay.example.com",
		ServerPort: 443,
		LocalPort:  8080,
	}

	client := NewPierceClient(cfg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.config.Mode != ModeRelay {
		t.Errorf("expected mode %s, got %s", ModeRelay, client.config.Mode)
	}
}

func TestGetStatus(t *testing.T) {
	cfg := &Config{Enabled: true}
	client := NewPierceClient(cfg)

	status := client.GetStatus()
	if status.Connected {
		t.Error("expected initial status to be disconnected")
	}
}

func TestStopWithoutStart(t *testing.T) {
	cfg := &Config{Enabled: true}
	client := NewPierceClient(cfg)

	// 停止未启动的客户端应该不会panic
	if err := client.Stop(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
