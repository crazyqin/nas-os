package wifimgr

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	status, err := m.GetStatus()
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.Connected {
		t.Error("expected not connected initially")
	}
}

func TestScan(t *testing.T) {
	m := NewManager()

	networks, err := m.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(networks) < 5 {
		t.Errorf("expected at least 5 networks, got %d", len(networks))
	}

	// 验证网络属性
	for _, n := range networks {
		if n.SSID == "" {
			t.Error("expected non-empty SSID")
		}
		if n.BSSID == "" {
			t.Error("expected non-empty BSSID")
		}
		if n.Signal == 0 {
			t.Error("expected non-zero signal")
		}
	}
}

func TestSaveAndListProfiles(t *testing.T) {
	m := NewManager()

	profile := &WiFiProfile{
		ID:          "home-5g",
		SSID:        "Home-5G",
		Password:    "password123",
		AuthType:    AuthWPA3PSK,
		AutoConnect: true,
		Priority:    10,
		Band:        Band5GHz,
	}

	err := m.SaveProfile(profile)
	if err != nil {
		t.Fatalf("save profile failed: %v", err)
	}

	profiles := m.ListProfiles()
	if len(profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(profiles))
	}

	if profiles[0].ID != "home-5g" {
		t.Errorf("expected 'home-5g', got '%s'", profiles[0].ID)
	}
	if profiles[0].SSID != "Home-5G" {
		t.Errorf("expected 'Home-5G', got '%s'", profiles[0].SSID)
	}

	// 空 ID
	err = m.SaveProfile(&WiFiProfile{SSID: "test"})
	if err == nil {
		t.Error("expected error for empty ID")
	}

	// 空 SSID
	err = m.SaveProfile(&WiFiProfile{ID: "test"})
	if err == nil {
		t.Error("expected error for empty SSID")
	}
}

func TestDeleteProfile(t *testing.T) {
	m := NewManager()

	m.SaveProfile(&WiFiProfile{
		ID:   "to-delete",
		SSID: "Delete-Me",
	})

	err := m.DeleteProfile("to-delete")
	if err != nil {
		t.Fatalf("delete profile failed: %v", err)
	}

	profiles := m.ListProfiles()
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}

	// 不存在的配置
	err = m.DeleteProfile("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestConnectAndDisconnect(t *testing.T) {
	m := NewManager()

	// 保存配置
	m.SaveProfile(&WiFiProfile{
		ID:       "test-wifi",
		SSID:     "Test-WiFi",
		Password: "password",
		AuthType: AuthWPA2PSK,
		Band:     Band5GHz,
	})

	// 连接
	err := m.Connect("test-wifi")
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	status, _ := m.GetStatus()
	if !status.Connected {
		t.Error("expected connected")
	}
	if status.SSID != "Test-WiFi" {
		t.Errorf("expected 'Test-WiFi', got '%s'", status.SSID)
	}
	if status.IP == "" {
		t.Error("expected IP address")
	}

	// 断开
	err = m.Disconnect()
	if err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}

	status, _ = m.GetStatus()
	if status.Connected {
		t.Error("expected not connected")
	}

	// 未连接时断开
	err = m.Disconnect()
	if err == nil {
		t.Error("expected error when not connected")
	}

	// 不存在的配置
	err = m.Connect("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestGetStatus(t *testing.T) {
	m := NewManager()

	status, err := m.GetStatus()
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.Connected {
		t.Error("expected not connected")
	}
}

func TestHotspot(t *testing.T) {
	m := NewManager()

	config := &HotspotConfig{
		SSID:       "Test-Hotspot",
		Password:   "hotspot123",
		Band:       Band24GHz,
		MaxClients: 5,
		Channel:    11,
	}

	// 启用热点
	err := m.EnableHotspot(config)
	if err != nil {
		t.Fatalf("enable hotspot failed: %v", err)
	}

	// 获取热点状态
	gotConfig, clients, err := m.GetHotspotStatus()
	if err != nil {
		t.Fatalf("get hotspot status failed: %v", err)
	}
	if gotConfig.SSID != "Test-Hotspot" {
		t.Errorf("expected 'Test-Hotspot', got '%s'", gotConfig.SSID)
	}
	if clients != 0 {
		t.Errorf("expected 0 clients, got %d", clients)
	}

	// 禁用热点
	err = m.DisableHotspot()
	if err != nil {
		t.Fatalf("disable hotspot failed: %v", err)
	}

	// 热点已禁用时获取状态
	_, _, err = m.GetHotspotStatus()
	if err == nil {
		t.Error("expected error when hotspot disabled")
	}

	// 禁用已禁用的热点
	err = m.DisableHotspot()
	if err == nil {
		t.Error("expected error when disabling already disabled hotspot")
	}

	// 空 SSID
	err = m.EnableHotspot(&HotspotConfig{Password: "12345678"})
	if err == nil {
		t.Error("expected error for empty SSID")
	}

	// 密码太短
	err = m.EnableHotspot(&HotspotConfig{SSID: "test", Password: "short"})
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestGetSignalHistory(t *testing.T) {
	m := NewManager()

	// 初始为空
	history := m.GetSignalHistory(time.Hour)
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d", len(history))
	}

	// 连接后应有记录
	m.SaveProfile(&WiFiProfile{
		ID:   "test",
		SSID: "Test",
		Band: Band5GHz,
	})
	m.Connect("test")

	history = m.GetSignalHistory(time.Hour)
	if len(history) == 0 {
		t.Error("expected signal history after connect")
	}

	if history[0].Signal == 0 {
		t.Error("expected non-zero signal")
	}
}

func TestSetAutoReconnect(t *testing.T) {
	m := NewManager()

	err := m.SetAutoReconnect(true, "exponential")
	if err != nil {
		t.Fatalf("set auto reconnect failed: %v", err)
	}

	if !m.autoReconnect {
		t.Error("expected auto reconnect enabled")
	}
	if m.reconnectStrategy != "exponential" {
		t.Errorf("expected 'exponential', got '%s'", m.reconnectStrategy)
	}

	// 有效策略
	err = m.SetAutoReconnect(true, "immediate")
	if err != nil {
		t.Fatalf("set strategy failed: %v", err)
	}

	err = m.SetAutoReconnect(true, "linear")
	if err != nil {
		t.Fatalf("set strategy failed: %v", err)
	}

	// 无效策略
	err = m.SetAutoReconnect(true, "invalid")
	if err == nil {
		t.Error("expected error for invalid strategy")
	}
}

func TestScanDiagnostics(t *testing.T) {
	m := NewManager()

	// 保存一个配置
	m.SaveProfile(&WiFiProfile{
		ID:   "home",
		SSID: "Home-5G",
		Band: Band5GHz,
	})

	networks, err := m.ScanDiagnostics()
	if err != nil {
		t.Fatalf("scan diagnostics failed: %v", err)
	}

	if len(networks) < 5 {
		t.Errorf("expected at least 5 networks, got %d", len(networks))
	}

	// 验证已保存标记
	foundSaved := false
	for _, n := range networks {
		if n.SSID == "Home-5G" && n.IsSaved {
			foundSaved = true
		}
	}
	if !foundSaved {
		t.Error("expected Home-5G to be marked as saved")
	}
}
