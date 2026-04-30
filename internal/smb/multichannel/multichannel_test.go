package multichannel

import (
	"net"
	"testing"
	"time"
)

func TestNewSMBMultichannelManager(t *testing.T) {
	mgr := NewSMBMultichannelManager(nil)
	if mgr == nil {
		t.Fatal("NewSMBMultichannelManager returned nil")
	}
	if mgr.config.MaxChannelsPerClient != 4 {
		t.Errorf("expected max channels 4, got %d", mgr.config.MaxChannelsPerClient)
	}
	if mgr.config.BalanceMode != BalanceAdaptive {
		t.Errorf("expected adaptive balance mode, got %s", mgr.config.BalanceMode)
	}
}

func TestMultichannelDisabled(t *testing.T) {
	mgr := NewSMBMultichannelManager(&MultichannelConfig{Enabled: false})
	if err := mgr.Start(); err != nil {
		t.Fatalf("disabled manager start should not error: %v", err)
	}
}

func TestMultichannelStats(t *testing.T) {
	mgr := NewSMBMultichannelManager(nil)
	stats := mgr.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.ActiveGroups != 0 {
		t.Errorf("expected 0 active groups, got %d", stats.ActiveGroups)
	}
}

func TestBalanceModes(t *testing.T) {
	modes := []BalanceMode{BalanceRoundRobin, BalanceLeastLoad, BalanceHash, BalanceAdaptive}
	for _, mode := range modes {
		mgr := NewSMBMultichannelManager(&MultichannelConfig{BalanceMode: mode})
		if mgr.config.BalanceMode != mode {
			t.Errorf("expected mode %s", mode)
		}
	}
}

func TestChannelScoreCalculation(t *testing.T) {
	mgr := NewSMBMultichannelManager(nil)
	ch := &NetworkChannel{
		Bandwidth:  10000,
		Latency:    1 * time.Millisecond,
		PacketLoss: 0,
	}
	score := mgr.channelScore(ch)
	if score <= 0 {
		t.Errorf("expected positive score, got %f", score)
	}
}

func TestEstimateBandwidth(t *testing.T) {
	if estimateBandwidth(net.Interface{MTU: 1500}) != 1000 {
		t.Error("expected 1000 for MTU 1500")
	}
	if estimateBandwidth(net.Interface{MTU: 9000}) != 10000 {
		t.Error("expected 10000 for MTU 9000")
	}
}
