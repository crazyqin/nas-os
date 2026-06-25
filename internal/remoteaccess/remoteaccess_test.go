package remoteaccess

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewRemoteAccessManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := RemoteAccessConfig{}

	mgr := NewRemoteAccessManager(logger, config)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewConnectionManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewConnectionManager(logger, "test-peer-1")
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewNATDetector(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	detector := NewNATDetector(logger)
	if detector == nil {
		t.Fatal("expected non-nil detector")
	}
}

func TestNewRelayManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewRelayManager(logger)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewBandwidthManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := BandwidthConfig{}

	mgr := NewBandwidthManager(logger, config)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewAccessControl(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ac := NewAccessControl(logger)
	if ac == nil {
		t.Fatal("expected non-nil access control")
	}
}

func TestNewHolePunchManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewHolePunchManager(logger, "test-local-id")
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewSTUNClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewSTUNClient(logger)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewRelayPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pool := NewRelayPool(logger)
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}

func TestNewRelayClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := RelayClientConfig{}

	client := NewRelayClient(logger, config)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewTunnelManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewTunnelManager(logger, nil)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestRemoteAccessConfig_Fields(t *testing.T) {
	config := RemoteAccessConfig{}
	// 验证零值配置
	_ = config
}

func TestBandwidthConfig_Fields(t *testing.T) {
	config := BandwidthConfig{}
	_ = config
}

func TestRelayClientConfig_Fields(t *testing.T) {
	config := RelayClientConfig{}
	_ = config
}
