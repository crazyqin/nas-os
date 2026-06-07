// Package smbmultichannel 测试
package smbmultichannel

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("manager should not be nil")
	}

	cfg := m.GetConfig()
	if cfg.Enabled {
		t.Error("config should be disabled by default")
	}
	if cfg.MaxChannels != 4 {
		t.Errorf("expected max_channels=4, got %d", cfg.MaxChannels)
	}
	if cfg.MinSpeed != 1000 {
		t.Errorf("expected min_speed=1000, got %d", cfg.MinSpeed)
	}
}

func TestUpdateConfig(t *testing.T) {
	m := NewManager()

	// 启用
	enabled := true
	maxCh := 8
	minSpd := 2500
	cfg, err := m.UpdateConfig(UpdateConfigRequest{
		Enabled:        &enabled,
		MaxChannels:    &maxCh,
		MinSpeed:       &minSpd,
		InterfaceNames: []string{"eth0", "eth1"},
	})
	if err != nil {
		t.Fatalf("update config failed: %v", err)
	}
	if !cfg.Enabled {
		t.Error("config should be enabled")
	}
	if cfg.MaxChannels != 8 {
		t.Errorf("expected max_channels=8, got %d", cfg.MaxChannels)
	}
	if cfg.MinSpeed != 2500 {
		t.Errorf("expected min_speed=2500, got %d", cfg.MinSpeed)
	}
	if len(cfg.InterfaceNames) != 2 {
		t.Errorf("expected 2 interfaces, got %d", len(cfg.InterfaceNames))
	}
}

func TestUpdateConfigValidation(t *testing.T) {
	m := NewManager()

	// 验证 max_channels 范围
	maxCh := 0
	_, err := m.UpdateConfig(UpdateConfigRequest{MaxChannels: &maxCh})
	if err == nil {
		t.Error("expected error for max_channels=0")
	}

	maxCh = 33
	_, err = m.UpdateConfig(UpdateConfigRequest{MaxChannels: &maxCh})
	if err == nil {
		t.Error("expected error for max_channels=33")
	}

	// 验证 min_speed
	negSpeed := -1
	_, err = m.UpdateConfig(UpdateConfigRequest{MinSpeed: &negSpeed})
	if err == nil {
		t.Error("expected error for min_speed=-1")
	}
}

func TestDetectChannels(t *testing.T) {
	m := NewManager()

	channels := m.DetectChannels()
	if len(channels) == 0 {
		t.Fatal("should detect at least one channel")
	}

	// 应检测到模拟的接口
	found := make(map[string]bool)
	for _, ch := range channels {
		found[ch.InterfaceName] = true
	}

	expectedInterfaces := []string{"eth0", "eth1", "eth2", "bond0"}
	for _, name := range expectedInterfaces {
		if !found[name] {
			t.Errorf("expected interface %s not found", name)
		}
	}
}

func TestEnableDisableChannel(t *testing.T) {
	m := NewManager()

	// 先检测通道并启用 multichannel
	m.DetectChannels()
	enabled := true
	m.UpdateConfig(UpdateConfigRequest{Enabled: &enabled})

	// 启用 eth0
	status, err := m.EnableChannel("eth0")
	if err != nil {
		t.Fatalf("enable channel failed: %v", err)
	}
	if !status.Active {
		t.Error("channel should be active after enabling")
	}
	if status.Speed != 10000 {
		t.Errorf("expected speed=10000, got %d", status.Speed)
	}

	// 禁用 eth0
	status, err = m.DisableChannel("eth0")
	if err != nil {
		t.Fatalf("disable channel failed: %v", err)
	}
	if status.Active {
		t.Error("channel should be inactive after disabling")
	}
}

func TestEnableChannelSpeedCheck(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	// 设置高最低速度要求
	minSpeed := 15000
	m.UpdateConfig(UpdateConfigRequest{MinSpeed: &minSpeed})

	// eth2 只有 2500 Mbps，应失败
	_, err := m.EnableChannel("eth2")
	if err == nil {
		t.Error("expected error for channel below min speed")
	}
}

func TestEnableChannelNotFound(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	_, err := m.EnableChannel("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent channel")
	}
}

func TestGetChannelStatus(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	status, err := m.GetChannelStatus("eth0")
	if err != nil {
		t.Fatalf("get channel status failed: %v", err)
	}
	if status.InterfaceName != "eth0" {
		t.Errorf("expected eth0, got %s", status.InterfaceName)
	}
}

func TestGetChannelStatusNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetChannelStatus("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent channel")
	}
}

func TestSessions(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	// 启用两个通道并创建会话
	enabled := true
	m.UpdateConfig(UpdateConfigRequest{Enabled: &enabled})
	m.EnableChannel("eth0")
	m.EnableChannel("eth1")

	session := m.createSession("192.168.1.100", "192.168.1.1")
	if session == nil {
		t.Fatal("session should not be nil")
	}
	if session.ClientIP != "192.168.1.100" {
		t.Errorf("expected client IP 192.168.1.100, got %s", session.ClientIP)
	}
	if session.Protocol != "SMB3" {
		t.Errorf("expected protocol SMB3, got %s", session.Protocol)
	}
	if len(session.Channels) < 1 {
		t.Error("session should have at least 1 channel")
	}
}

func TestListSessions(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	enabled := true
	m.UpdateConfig(UpdateConfigRequest{Enabled: &enabled})
	m.EnableChannel("eth0")

	m.createSession("192.168.1.100", "192.168.1.1")
	m.createSession("192.168.1.101", "192.168.1.1")

	sessions := m.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestGetSession(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	enabled := true
	m.UpdateConfig(UpdateConfigRequest{Enabled: &enabled})
	m.EnableChannel("eth0")

	session := m.createSession("192.168.1.100", "192.168.1.1")

	got, err := m.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}
	if got.ID != session.ID {
		t.Error("session ID mismatch")
	}
}

func TestGetSessionNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestGetSessionStats(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	enabled := true
	m.UpdateConfig(UpdateConfigRequest{Enabled: &enabled})
	m.EnableChannel("eth0")
	m.EnableChannel("eth1")

	session := m.createSession("192.168.1.100", "192.168.1.1")

	stats, err := m.GetSessionStats(session.ID)
	if err != nil {
		t.Fatalf("get session stats failed: %v", err)
	}
	if stats.SessionID != session.ID {
		t.Error("session ID mismatch")
	}
	if stats.ClientIP != "192.168.1.100" {
		t.Errorf("expected client IP 192.168.1.100, got %s", stats.ClientIP)
	}
}

func TestGetThroughputStats(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	stats := m.GetThroughputStats()
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.LastUpdated.IsZero() {
		t.Error("last updated should be set")
	}
}

func TestBandwidthHistory(t *testing.T) {
	m := NewManager()

	// 记录一些样本
	for i := 0; i < 10; i++ {
		m.RecordBandwidth(
			int64(100+i*10)*1024*1024,
			int64(50+i*5)*1024*1024,
			1000+i*100,
		)
	}

	history := m.GetBandwidthHistory(5)
	if len(history) != 5 {
		t.Errorf("expected 5 history items, got %d", len(history))
	}

	// 测试获取全部
	all := m.GetBandwidthHistory(0)
	if len(all) != 10 {
		t.Errorf("expected 10 history items, got %d", len(all))
	}
}

func TestBandwidthHistoryLimit(t *testing.T) {
	m := NewManager()
	m.maxHistory = 5

	// 添加超过限制的记录
	for i := 0; i < 10; i++ {
		m.RecordBandwidth(1024*1024, 1024*1024, 1000)
	}

	if len(m.history) > 5 {
		t.Errorf("history should be capped at 5, got %d", len(m.history))
	}
}

func TestSimulateTraffic(t *testing.T) {
	m := NewManager()
	m.DetectChannels()

	enabled := true
	m.UpdateConfig(UpdateConfigRequest{Enabled: &enabled})
	m.EnableChannel("eth0")
	m.EnableChannel("eth1")

	m.SimulateTraffic()

	// 应该有历史记录了
	history := m.GetBandwidthHistory(1)
	if len(history) == 0 {
		t.Error("expected traffic history after simulation")
	}

	// 通道应有传输记录
	ch, _ := m.GetChannelStatus("eth0")
	if ch.BytesTransferred == 0 {
		t.Error("expected bytes transferred after simulation")
	}
}
