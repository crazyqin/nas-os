package usbreset

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	if m.autoMount.Policy != "readonly" {
		t.Errorf("expected 'readonly', got '%s'", m.autoMount.Policy)
	}
}

func TestListDevices(t *testing.T) {
	m := NewManager()

	devices := m.ListDevices()
	if len(devices) < 3 {
		t.Errorf("expected at least 3 devices, got %d", len(devices))
	}
}

func TestGetDevice(t *testing.T) {
	m := NewManager()

	dev := m.GetDevice("dev-1")
	if dev == nil {
		t.Fatal("expected device")
	}
	if dev.Name != "USB 硬盘" {
		t.Errorf("expected 'USB 硬盘', got '%s'", dev.Name)
	}
	if dev.Type != DeviceTypeStorage {
		t.Errorf("expected storage, got '%s'", dev.Type)
	}

	dev = m.GetDevice("nonexistent")
	if dev != nil {
		t.Error("expected nil for nonexistent device")
	}
}

func TestResetDevice(t *testing.T) {
	m := NewManager()

	// 正常重置
	err := m.ResetDevice("dev-1")
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// 不存在的设备
	err = m.ResetDevice("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}

	// 检查事件
	events := m.GetEvents(time.Now().Add(-1*time.Minute), "dev-1")
	found := false
	for _, e := range events {
		if e.EventType == EventReset {
			found = true
		}
	}
	if !found {
		t.Error("expected reset event")
	}
}

func TestResetPort(t *testing.T) {
	m := NewManager()

	err := m.ResetPort("usb-1-1")
	if err != nil {
		t.Fatalf("reset port failed: %v", err)
	}

	err = m.ResetPort("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent port")
	}
}

func TestSetPortPower(t *testing.T) {
	m := NewManager()

	err := m.SetPortPower("usb-1-1", false)
	if err != nil {
		t.Fatalf("set port power failed: %v", err)
	}

	port := m.ListPorts()
	for _, p := range port {
		if p.ID == "usb-1-1" && p.Powered {
			t.Error("expected port to be powered off")
		}
	}

	err = m.SetPortPower("usb-1-1", true)
	if err != nil {
		t.Fatalf("set port power failed: %v", err)
	}
}

func TestListPorts(t *testing.T) {
	m := NewManager()

	ports := m.ListPorts()
	if len(ports) < 4 {
		t.Errorf("expected at least 4 ports, got %d", len(ports))
	}
}

func TestPolicyManagement(t *testing.T) {
	m := NewManager()

	// 添加策略
	pol := &USBPolicy{
		DeviceType: DeviceTypeAudio, Action: PolicyDeny, Priority: 20,
		Name: "拒绝音频设备", Enabled: true,
	}
	err := m.AddPolicy(pol)
	if err != nil {
		t.Fatalf("add policy failed: %v", err)
	}

	policies := m.ListPolicies()
	found := false
	for _, p := range policies {
		if p.Name == "拒绝音频设备" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find new policy")
	}

	// 删除策略
	err = m.RemovePolicy(pol.ID)
	if err != nil {
		t.Fatalf("remove policy failed: %v", err)
	}

	err = m.RemovePolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestCheckPolicy(t *testing.T) {
	m := NewManager()

	// 存储设备应该允许
	action, err := m.CheckPolicy("dev-1")
	if err != nil {
		t.Fatalf("check policy failed: %v", err)
	}
	if action != "allow" {
		t.Errorf("expected 'allow', got '%s'", action)
	}

	// 不存在的设备
	_, err = m.CheckPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestGetEvents(t *testing.T) {
	m := NewManager()

	// 获取所有事件
	events := m.GetEvents(time.Time{}, "")
	if len(events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(events))
	}

	// 按设备过滤
	events = m.GetEvents(time.Time{}, "dev-1")
	for _, e := range events {
		if e.DeviceID != "dev-1" {
			t.Errorf("expected device dev-1, got %s", e.DeviceID)
		}
	}

	// 按时间过滤
	events = m.GetEvents(time.Now().Add(-1*time.Hour), "")
	for _, e := range events {
		if e.Timestamp.Before(time.Now().Add(-1 * time.Hour)) {
			t.Error("expected event within time range")
		}
	}
}

func TestGetBandwidth(t *testing.T) {
	m := NewManager()

	bw, err := m.GetBandwidth("usb-1-1")
	if err != nil {
		t.Fatalf("get bandwidth failed: %v", err)
	}
	if bw.MaxMbps != 5000 {
		t.Errorf("expected 5000 Mbps max, got %d", bw.MaxMbps)
	}
	if bw.UsedMbps != 400 {
		t.Errorf("expected 400 Mbps used, got %d", bw.UsedMbps)
	}

	_, err = m.GetBandwidth("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent port")
	}
}

func TestSetAutoMount(t *testing.T) {
	m := NewManager()

	err := m.SetAutoMount(false, "readwrite")
	if err != nil {
		t.Fatalf("set auto mount failed: %v", err)
	}
	if m.autoMount.Enabled {
		t.Error("expected auto mount disabled")
	}
	if m.autoMount.Policy != "readwrite" {
		t.Errorf("expected 'readwrite', got '%s'", m.autoMount.Policy)
	}

	// 无效策略
	err = m.SetAutoMount(true, "invalid")
	if err == nil {
		t.Error("expected error for invalid policy")
	}
}
