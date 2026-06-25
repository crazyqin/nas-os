package cms

import (
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestNewDeviceRegistry(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &RegistryConfig{}

	registry := NewDeviceRegistry(config, logger)
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
}

func TestDefaultFleetConfig(t *testing.T) {
	config := DefaultFleetConfig()
	_ = config
}

func TestGenerateToken(t *testing.T) {
	token := GenerateToken()
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	// 两次生成应该不同
	token2 := GenerateToken()
	if token == token2 {
		t.Error("expected different tokens")
	}
}

func TestNewNodeManagementService(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultFleetConfig()

	svc := NewNodeManagementService(config, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestDefaultHealthThresholds(t *testing.T) {
	thresholds := DefaultHealthThresholds()
	if thresholds.CPUHigh <= 0 {
		t.Error("expected positive CPUHigh")
	}
	if thresholds.MemoryHigh <= 0 {
		t.Error("expected positive MemoryHigh")
	}
}

func TestCreateSyncCommand(t *testing.T) {
	cmd := CreateSyncCommand([]string{"/data", "/config"})
	if cmd.Command != "sync" {
		t.Errorf("expected sync, got %s", cmd.Command)
	}
}

func TestCreateBackupCommand(t *testing.T) {
	cmd := CreateBackupCommand("/data", "/backup", "daily")
	if cmd.Command != "backup" {
		t.Errorf("expected backup, got %s", cmd.Command)
	}
}

func TestCreateRestartCommand(t *testing.T) {
	cmd := CreateRestartCommand("maintenance")
	if cmd.Command != "restart" {
		t.Errorf("expected restart, got %s", cmd.Command)
	}
}

func TestCreateUpdateCommand(t *testing.T) {
	cmd := CreateUpdateCommand("2.0.0", false)
	if cmd.Command != "update" {
		t.Errorf("expected update, got %s", cmd.Command)
	}
}

func TestNewFleetManager(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root")
	}
	logger, _ := zap.NewDevelopment()
	config := DefaultFleetConfig()

	mgr, err := NewFleetManager(config, logger)
	if err != nil {
		t.Fatalf("NewFleetManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}
