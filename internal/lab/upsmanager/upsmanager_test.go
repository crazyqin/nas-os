package upsmanager

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("expected manager")
	}
	if m.config.PollInterval != 10 {
		t.Errorf("expected poll interval 10, got %d", m.config.PollInterval)
	}
	if m.config.HistoryMax != 10000 {
		t.Errorf("expected history max 10000, got %d", m.config.HistoryMax)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.PollInterval != 10 {
		t.Errorf("expected poll interval 10, got %d", cfg.PollInterval)
	}
	if cfg.AlertThreshold != 20.0 {
		t.Errorf("expected alert threshold 20, got %f", cfg.AlertThreshold)
	}
	if !cfg.AutoSwitch {
		t.Error("expected auto switch enabled")
	}
	if cfg.HistoryMax != 10000 {
		t.Errorf("expected history max 10000, got %d", cfg.HistoryMax)
	}
}

func TestConnect(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	req := ConnectRequest{
		Name:      "Test UPS",
		Protocol:  ProtocolUSBHID,
		Address:   "/dev/usb/hiddev0",
		IsPrimary: true,
	}

	device, err := m.Connect(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	if device.ID == "" {
		t.Error("expected non-empty device ID")
	}
	if device.Name != "Test UPS" {
		t.Errorf("expected name 'Test UPS', got '%s'", device.Name)
	}
	if device.Protocol != ProtocolUSBHID {
		t.Errorf("expected protocol 'usb_hid', got '%s'", device.Protocol)
	}
	if !device.IsPrimary {
		t.Error("expected device to be primary")
	}
	if device.Status != UPSStatusOnline {
		t.Errorf("expected status 'online', got '%s'", device.Status)
	}
}

func TestConnectDuplicate(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	req := ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	}

	_, err := m.Connect(req)
	if err != nil {
		t.Fatalf("first connect failed: %v", err)
	}

	_, err = m.Connect(req)
	if err != ErrUPSAlreadyConnected {
		t.Errorf("expected ErrUPSAlreadyConnected, got %v", err)
	}
}

func TestConnectPrimarySwitch(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 连接第一个 UPS 为主
	req1 := ConnectRequest{
		Name:      "UPS 1",
		Protocol:  ProtocolUSBHID,
		Address:   "/dev/usb/hiddev0",
		IsPrimary: true,
	}
	dev1, err := m.Connect(req1)
	if err != nil {
		t.Fatalf("connect 1 failed: %v", err)
	}
	if !dev1.IsPrimary {
		t.Error("expected dev1 to be primary")
	}

	// 连接第二个 UPS 为主
	req2 := ConnectRequest{
		Name:      "UPS 2",
		Protocol:  ProtocolSNMP,
		Address:   "192.168.1.100",
		IsPrimary: true,
	}
	dev2, err := m.Connect(req2)
	if err != nil {
		t.Fatalf("connect 2 failed: %v", err)
	}

	// 重新获取 dev1
	dev1, _ = m.GetDevice(dev1.ID)
	if dev1.IsPrimary {
		t.Error("expected dev1 to not be primary after dev2 connected as primary")
	}
	if !dev2.IsPrimary {
		t.Error("expected dev2 to be primary")
	}
}

func TestDisconnect(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	req := ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	}

	device, err := m.Connect(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	err = m.Disconnect(device.ID)
	if err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}

	_, err = m.GetDevice(device.ID)
	if err != ErrUPSNotFound {
		t.Errorf("expected ErrUPSNotFound after disconnect, got %v", err)
	}
}

func TestDisconnectNotFound(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	err := m.Disconnect("nonexistent")
	if err != ErrUPSNotFound {
		t.Errorf("expected ErrUPSNotFound, got %v", err)
	}
}

func TestListDevices(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 空列表
	devices := m.ListDevices()
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}

	// 添加设备
	m.Connect(ConnectRequest{
		Name:     "UPS 1",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})
	m.Connect(ConnectRequest{
		Name:     "UPS 2",
		Protocol: ProtocolSNMP,
		Address:  "192.168.1.100",
	})

	devices = m.ListDevices()
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestGetDevice(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	req := ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	}

	created, err := m.Connect(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	device, err := m.GetDevice(created.ID)
	if err != nil {
		t.Fatalf("get device failed: %v", err)
	}

	if device.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, device.ID)
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	_, err := m.GetDevice("nonexistent")
	if err != ErrUPSNotFound {
		t.Errorf("expected ErrUPSNotFound, got %v", err)
	}
}

func TestGetPowerStatus(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	req := ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	}

	device, err := m.Connect(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	status, err := m.GetPowerStatus(device.ID)
	if err != nil {
		t.Fatalf("get power status failed: %v", err)
	}

	if status.UPSID != device.ID {
		t.Errorf("expected upsId '%s', got '%s'", device.ID, status.UPSID)
	}
	if status.InputVoltage == 0 {
		t.Error("expected non-zero input voltage")
	}
	if status.OutputVoltage == 0 {
		t.Error("expected non-zero output voltage")
	}
	if status.Battery.Charge == 0 {
		t.Error("expected non-zero battery charge")
	}
}

func TestGetPowerStatusNotFound(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	_, err := m.GetPowerStatus("nonexistent")
	if err != ErrUPSNotFound {
		t.Errorf("expected ErrUPSNotFound, got %v", err)
	}
}

func TestGetAllPowerStatus(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 空
	status := m.GetAllPowerStatus()
	if len(status) != 0 {
		t.Errorf("expected 0 status, got %d", len(status))
	}

	// 添加设备
	m.Connect(ConnectRequest{
		Name:     "UPS 1",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})
	m.Connect(ConnectRequest{
		Name:     "UPS 2",
		Protocol: ProtocolSNMP,
		Address:  "192.168.1.100",
	})

	status = m.GetAllPowerStatus()
	if len(status) != 2 {
		t.Errorf("expected 2 status, got %d", len(status))
	}
}

func TestGetPrimaryPowerStatus(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 没有主 UPS
	_, err := m.GetPrimaryPowerStatus()
	if err != ErrNoPrimaryUPS {
		t.Errorf("expected ErrNoPrimaryUPS, got %v", err)
	}

	// 添加主 UPS
	req := ConnectRequest{
		Name:      "Primary UPS",
		Protocol:  ProtocolUSBHID,
		Address:   "/dev/usb/hiddev0",
		IsPrimary: true,
	}
	device, _ := m.Connect(req)

	status, err := m.GetPrimaryPowerStatus()
	if err != nil {
		t.Fatalf("get primary power status failed: %v", err)
	}

	if status.UPSID != device.ID {
		t.Errorf("expected upsId '%s', got '%s'", device.ID, status.UPSID)
	}
}

func TestGetHardwareHealth(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	req := ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	}

	device, err := m.Connect(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	health, err := m.GetHardwareHealth(device.ID)
	if err != nil {
		t.Fatalf("get hardware health failed: %v", err)
	}

	if health.UPSID != device.ID {
		t.Errorf("expected upsId '%s', got '%s'", device.ID, health.UPSID)
	}
	if len(health.DiskTemps) == 0 {
		t.Error("expected disk temps")
	}
	if len(health.FanSpeeds) == 0 {
		t.Error("expected fan speeds")
	}
}

func TestShutdownPolicy(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 创建策略
	req := SetShutdownPolicyRequest{
		Name:             "低电量关机",
		Enabled:          true,
		BatteryThreshold: 20.0,
		DelaySeconds:     30,
		RuntimeThreshold: 10,
		NotifyBefore:     60,
		Command:          "/sbin/shutdown -h now",
	}

	policy, err := m.CreateShutdownPolicy(req)
	if err != nil {
		t.Fatalf("create policy failed: %v", err)
	}

	if policy.ID == "" {
		t.Error("expected non-empty policy ID")
	}
	if policy.Name != "低电量关机" {
		t.Errorf("expected name '低电量关机', got '%s'", policy.Name)
	}
	if policy.BatteryThreshold != 20.0 {
		t.Errorf("expected battery threshold 20, got %f", policy.BatteryThreshold)
	}
	if !policy.Enabled {
		t.Error("expected policy enabled")
	}

	// 获取策略
	got, err := m.GetShutdownPolicy(policy.ID)
	if err != nil {
		t.Fatalf("get policy failed: %v", err)
	}
	if got.ID != policy.ID {
		t.Errorf("expected ID '%s', got '%s'", policy.ID, got.ID)
	}

	// 列出策略
	policies := m.ListShutdownPolicies()
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}

	// 更新策略
	updateReq := SetShutdownPolicyRequest{
		Name:             "更新后的策略",
		Enabled:          false,
		BatteryThreshold: 30.0,
		DelaySeconds:     60,
	}
	updated, err := m.UpdateShutdownPolicy(policy.ID, updateReq)
	if err != nil {
		t.Fatalf("update policy failed: %v", err)
	}
	if updated.Name != "更新后的策略" {
		t.Errorf("expected name '更新后的策略', got '%s'", updated.Name)
	}
	if updated.Enabled {
		t.Error("expected policy disabled")
	}

	// 删除策略
	err = m.DeleteShutdownPolicy(policy.ID)
	if err != nil {
		t.Fatalf("delete policy failed: %v", err)
	}

	_, err = m.GetShutdownPolicy(policy.ID)
	if err != ErrShutdownPolicyNotFound {
		t.Errorf("expected ErrShutdownPolicyNotFound after delete, got %v", err)
	}
}

func TestShutdownPolicyNotFound(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	_, err := m.GetShutdownPolicy("nonexistent")
	if err != ErrShutdownPolicyNotFound {
		t.Errorf("expected ErrShutdownPolicyNotFound, got %v", err)
	}

	err = m.DeleteShutdownPolicy("nonexistent")
	if err != ErrShutdownPolicyNotFound {
		t.Errorf("expected ErrShutdownPolicyNotFound, got %v", err)
	}
}

func TestGetEvents(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 连接设备会生成事件
	m.Connect(ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})

	events := m.GetEvents(EventQueryParams{})
	if len(events) == 0 {
		t.Error("expected events after connect")
	}

	// 测试过滤
	events = m.GetEvents(EventQueryParams{
		Type: "ups_connected",
	})
	if len(events) == 0 {
		t.Error("expected ups_connected events")
	}

	// 测试分页
	events = m.GetEvents(EventQueryParams{
		Limit:  1,
		Offset: 0,
	})
	if len(events) > 1 {
		t.Errorf("expected at most 1 event, got %d", len(events))
	}
}

func TestGetEventCount(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 连接设备会生成事件
	dev, _ := m.Connect(ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})

	count := m.GetEventCount("")
	if count == 0 {
		t.Error("expected events")
	}

	count = m.GetEventCount(dev.ID)
	if count == 0 {
		t.Error("expected events for device")
	}

	count = m.GetEventCount("nonexistent")
	if count != 0 {
		t.Errorf("expected 0 events for nonexistent device, got %d", count)
	}
}

func TestGetPowerStats(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	req := ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	}

	device, err := m.Connect(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	stats, err := m.GetPowerStats(device.ID)
	if err != nil {
		t.Fatalf("get power stats failed: %v", err)
	}

	if stats.UPSID != device.ID {
		t.Errorf("expected upsId '%s', got '%s'", device.ID, stats.UPSID)
	}
	if stats.UptimePercent != 100.0 {
		t.Errorf("expected uptime percent 100, got %f", stats.UptimePercent)
	}
}

func TestGetPowerStatsNotFound(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	_, err := m.GetPowerStats("nonexistent")
	if err != ErrUPSNotFound {
		t.Errorf("expected ErrUPSNotFound, got %v", err)
	}
}

func TestConfig(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 获取配置
	config := m.GetConfig()
	if config.PollInterval != 10 {
		t.Errorf("expected poll interval 10, got %d", config.PollInterval)
	}

	// 更新配置
	newConfig := m.UpdateConfig(UpdateConfigRequest{
		PollInterval:   30,
		AlertThreshold: 25.0,
		AutoSwitch:     false,
		HistoryMax:     5000,
	})

	if newConfig.PollInterval != 30 {
		t.Errorf("expected poll interval 30, got %d", newConfig.PollInterval)
	}
	if newConfig.AlertThreshold != 25.0 {
		t.Errorf("expected alert threshold 25, got %f", newConfig.AlertThreshold)
	}
	if newConfig.AutoSwitch {
		t.Error("expected auto switch disabled")
	}
	if newConfig.HistoryMax != 5000 {
		t.Errorf("expected history max 5000, got %d", newConfig.HistoryMax)
	}
}

func TestStartStop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PollInterval = 1 // 1秒轮询，便于测试
	m := NewManager(cfg)

	// 连接设备
	m.Connect(ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})

	// 启动
	m.Start()
	if !m.IsRunning() {
		t.Error("expected manager to be running")
	}

	// 等待一次轮询
	time.Sleep(1500 * time.Millisecond)

	// 停止
	m.Stop()
	if m.IsRunning() {
		t.Error("expected manager to be stopped")
	}

	// 重复停止不应 panic
	m.Stop()
}

func TestSetUPSStatus(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	req := ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	}

	device, err := m.Connect(req)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	// 初始状态为 online
	if device.Status != UPSStatusOnline {
		t.Errorf("expected status 'online', got '%s'", device.Status)
	}

	// 设置为 on_battery
	err = m.SetUPSStatus(device.ID, UPSStatusOnBattery)
	if err != nil {
		t.Fatalf("set status failed: %v", err)
	}

	device, _ = m.GetDevice(device.ID)
	if device.Status != UPSStatusOnBattery {
		t.Errorf("expected status 'on_battery', got '%s'", device.Status)
	}

	// 检查事件记录
	events := m.GetEvents(EventQueryParams{Type: "power_out"})
	if len(events) == 0 {
		t.Error("expected power_out event")
	}

	// 设置为 low_battery
	err = m.SetUPSStatus(device.ID, UPSStatusLowBattery)
	if err != nil {
		t.Fatalf("set status failed: %v", err)
	}

	events = m.GetEvents(EventQueryParams{Type: "battery_low"})
	if len(events) == 0 {
		t.Error("expected battery_low event")
	}

	// 恢复为 online
	err = m.SetUPSStatus(device.ID, UPSStatusOnline)
	if err != nil {
		t.Fatalf("set status failed: %v", err)
	}

	events = m.GetEvents(EventQueryParams{Type: "power_restore"})
	if len(events) == 0 {
		t.Error("expected power_restore event")
	}

	// 设置不存在的设备
	err = m.SetUPSStatus("nonexistent", UPSStatusFault)
	if err != ErrUPSNotFound {
		t.Errorf("expected ErrUPSNotFound, got %v", err)
	}
}

func TestGetStatusSummary(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 连接设备
	m.Connect(ConnectRequest{
		Name:      "Primary UPS",
		Protocol:  ProtocolUSBHID,
		Address:   "/dev/usb/hiddev0",
		IsPrimary: true,
	})
	m.Connect(ConnectRequest{
		Name:     "Backup UPS",
		Protocol: ProtocolSNMP,
		Address:  "192.168.1.100",
	})

	summary := m.GetStatusSummary()

	totalDevices, ok := summary["totalDevices"]
	if !ok || totalDevices != 2 {
		t.Errorf("expected totalDevices 2, got %v", totalDevices)
	}

	primaryUPS, ok := summary["primaryUPS"]
	if !ok || primaryUPS == nil {
		t.Error("expected primaryUPS in summary")
	}

	primaryBattery, ok := summary["primaryBattery"]
	if !ok || primaryBattery == nil {
		t.Error("expected primaryBattery in summary")
	}
}

func TestAlertCallback(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	alertReceived := false
	m.SetAlertCallback(func(event PowerEvent) {
		alertReceived = true
	})

	// 连接设备
	device, _ := m.Connect(ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})

	// 触发告警事件（low_battery 是 critical 级别）
	m.SetUPSStatus(device.ID, UPSStatusLowBattery)

	// 等待异步回调
	time.Sleep(100 * time.Millisecond)

	if !alertReceived {
		t.Error("expected alert callback to be called")
	}
}

func TestMultipleProtocols(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// USB HID
	usb, err := m.Connect(ConnectRequest{
		Name:     "USB UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})
	if err != nil {
		t.Fatalf("connect USB failed: %v", err)
	}

	// SNMP
	snmp, err := m.Connect(ConnectRequest{
		Name:     "SNMP UPS",
		Protocol: ProtocolSNMP,
		Address:  "192.168.1.100",
		Port:     161,
	})
	if err != nil {
		t.Fatalf("connect SNMP failed: %v", err)
	}

	// NUT
	nut, err := m.Connect(ConnectRequest{
		Name:     "NUT UPS",
		Protocol: ProtocolNUT,
		Address:  "localhost",
		Port:     3493,
	})
	if err != nil {
		t.Fatalf("connect NUT failed: %v", err)
	}

	// 验证所有设备
	devices := m.ListDevices()
	if len(devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(devices))
	}

	// 验证各设备协议
	d, _ := m.GetDevice(usb.ID)
	if d.Protocol != ProtocolUSBHID {
		t.Errorf("expected USB HID protocol, got %s", d.Protocol)
	}

	d, _ = m.GetDevice(snmp.ID)
	if d.Protocol != ProtocolSNMP {
		t.Errorf("expected SNMP protocol, got %s", d.Protocol)
	}

	d, _ = m.GetDevice(nut.ID)
	if d.Protocol != ProtocolNUT {
		t.Errorf("expected NUT protocol, got %s", d.Protocol)
	}
}

func TestDiscover(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// USB HID 发现
	devices, err := m.Discover(DiscoverRequest{Protocol: ProtocolUSBHID})
	if err != nil {
		t.Fatalf("discover USB failed: %v", err)
	}
	if len(devices) == 0 {
		t.Error("expected USB devices")
	}

	// SNMP 发现
	devices, err = m.Discover(DiscoverRequest{
		Protocol: ProtocolSNMP,
		Address:  "192.168.1.0/24",
	})
	if err != nil {
		t.Fatalf("discover SNMP failed: %v", err)
	}
	if len(devices) == 0 {
		t.Error("expected SNMP devices")
	}

	// NUT 发现
	devices, err = m.Discover(DiscoverRequest{Protocol: ProtocolNUT})
	if err != nil {
		t.Fatalf("discover NUT failed: %v", err)
	}
	if len(devices) == 0 {
		t.Error("expected NUT devices")
	}

	// 不支持的协议
	_, err = m.Discover(DiscoverRequest{Protocol: Protocol("invalid")})
	if err != ErrProtocolNotSupported {
		t.Errorf("expected ErrProtocolNotSupported, got %v", err)
	}
}

func TestEventFiltering(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	// 连接设备并触发多种事件
	device, _ := m.Connect(ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})
	m.SetUPSStatus(device.ID, UPSStatusOnBattery)
	m.SetUPSStatus(device.ID, UPSStatusLowBattery)
	m.SetUPSStatus(device.ID, UPSStatusOnline)

	// 按类型过滤
	events := m.GetEvents(EventQueryParams{Type: "power_out"})
	if len(events) == 0 {
		t.Error("expected power_out events")
	}
	for _, e := range events {
		if string(e.Type) != "power_out" {
			t.Errorf("expected event type 'power_out', got '%s'", e.Type)
		}
	}

	// 按严重级别过滤
	events = m.GetEvents(EventQueryParams{Severity: "critical"})
	if len(events) == 0 {
		t.Error("expected critical events")
	}
	for _, e := range events {
		if e.Severity != "critical" {
			t.Errorf("expected severity 'critical', got '%s'", e.Severity)
		}
	}

	// 按 UPS ID 过滤
	events = m.GetEvents(EventQueryParams{UPSID: device.ID})
	if len(events) == 0 {
		t.Error("expected events for device")
	}
	for _, e := range events {
		if e.UPSID != device.ID {
			t.Errorf("expected upsId '%s', got '%s'", device.ID, e.UPSID)
		}
	}
}

func TestPowerStatsUpdate(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	device, _ := m.Connect(ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})

	// 触发断电事件
	m.SetUPSStatus(device.ID, UPSStatusOnBattery)

	stats, _ := m.GetPowerStats(device.ID)
	if stats.PowerOutCount != 1 {
		t.Errorf("expected power out count 1, got %d", stats.PowerOutCount)
	}
	if stats.LastPowerOut == nil {
		t.Error("expected last power out time")
	}

	// 触发恢复事件
	m.SetUPSStatus(device.ID, UPSStatusOnline)

	stats, _ = m.GetPowerStats(device.ID)
	if stats.LastPowerRestore == nil {
		t.Error("expected last power restore time")
	}

	// 触发电池低电量事件
	m.SetUPSStatus(device.ID, UPSStatusOnBattery)
	m.SetUPSStatus(device.ID, UPSStatusLowBattery)

	stats, _ = m.GetPowerStats(device.ID)
	if stats.BatteryDrainCount != 1 {
		t.Errorf("expected battery drain count 1, got %d", stats.BatteryDrainCount)
	}
}

func TestPollingUpdatesData(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PollInterval = 1
	m := NewManager(cfg)

	device, _ := m.Connect(ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})

	// 启动轮询
	m.Start()
	defer m.Stop()

	// 等待轮询
	time.Sleep(1500 * time.Millisecond)

	// 验证数据已更新
	status, err := m.GetPowerStatus(device.ID)
	if err != nil {
		t.Fatalf("get power status failed: %v", err)
	}

	if status.UpdatedAt.IsZero() {
		t.Error("expected updated time")
	}

	// 验证硬件健康已更新
	health, err := m.GetHardwareHealth(device.ID)
	if err != nil {
		t.Fatalf("get health failed: %v", err)
	}

	if health.UpdatedAt.IsZero() {
		t.Error("expected health updated time")
	}
}

func TestEventHistoryLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HistoryMax = 5
	m := NewManager(cfg)

	// 连接设备（会生成事件）
	device, _ := m.Connect(ConnectRequest{
		Name:     "Test UPS",
		Protocol: ProtocolUSBHID,
		Address:  "/dev/usb/hiddev0",
	})

	// 生成足够多的事件
	for i := 0; i < 10; i++ {
		m.SetUPSStatus(device.ID, UPSStatusOnBattery)
		m.SetUPSStatus(device.ID, UPSStatusOnline)
	}

	// 验证事件数不超过限制
	events := m.GetEvents(EventQueryParams{})
	if len(events) > cfg.HistoryMax {
		t.Errorf("expected at most %d events, got %d", cfg.HistoryMax, len(events))
	}
}
