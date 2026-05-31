package ipmi

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := IPMIConfig{
		PollInterval: 30 * time.Second,
		EventLimit:   1000,
	}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.config.PollInterval != 30*time.Second {
		t.Errorf("期望 PollInterval=30s, 实际 %s", m.config.PollInterval)
	}
}

func TestManager_StartStop(t *testing.T) {
	cfg := IPMIConfig{}
	m := NewManager(cfg)
	if err := m.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if !m.running {
		t.Error("期望 running=true")
	}
	m.Stop()
	if m.running {
		t.Error("期望 running=false")
	}
}

func TestManager_DeviceLifecycle(t *testing.T) {
	cfg := IPMIConfig{}
	m := NewManager(cfg)

	// 添加设备
	device := &IPMIDevice{
		ID:           "d1",
		Name:         "NAS服务器",
		Host:         "192.168.1.100",
		Port:         623,
		Username:     "admin",
		Model:        "SuperMicro X11",
		Manufacturer: "SuperMicro",
	}
	if err := m.AddDevice(device); err != nil {
		t.Fatalf("AddDevice 失败: %v", err)
	}

	// 获取设备
	got, err := m.GetDevice("d1")
	if err != nil {
		t.Fatalf("GetDevice 失败: %v", err)
	}
	if got.Host != "192.168.1.100" {
		t.Errorf("期望 host=192.168.1.100, 实际 %s", got.Host)
	}

	// 列表
	devices := m.ListDevices()
	if len(devices) != 1 {
		t.Errorf("期望 1 个设备, 实际 %d", len(devices))
	}

	// 重复添加
	if err := m.AddDevice(device); err == nil {
		t.Error("重复添加应报错")
	}

	// 移除
	if err := m.RemoveDevice("d1"); err != nil {
		t.Fatalf("RemoveDevice 失败: %v", err)
	}
}

func TestManager_PowerControl(t *testing.T) {
	cfg := IPMIConfig{}
	m := NewManager(cfg)
	m.AddDevice(&IPMIDevice{ID: "d1", Name: "test", Host: "192.168.1.1"})

	// 开机
	if err := m.PowerOn("d1"); err != nil {
		t.Fatalf("PowerOn 失败: %v", err)
	}
	dev, _ := m.GetDevice("d1")
	if dev.PowerState != PowerStateOn {
		t.Errorf("期望 PowerState=on, 实际 %s", dev.PowerState)
	}

	// 关机 (需要force=true因为设备正在运行)
	if err := m.PowerOff("d1", true); err != nil {
		t.Fatalf("PowerOff 失败: %v", err)
	}
	dev, _ = m.GetDevice("d1")
	if dev.PowerState != PowerStateOff {
		t.Errorf("期望 PowerState=off, 实际 %s", dev.PowerState)
	}

	// 重启
	if err := m.PowerCycle("d1"); err != nil {
		t.Fatalf("PowerCycle 失败: %v", err)
	}
}

func TestManager_SensorManagement(t *testing.T) {
	cfg := IPMIConfig{}
	m := NewManager(cfg)

	// 注册传感器
	sensor := &Sensor{
		ID:       "s1",
		DeviceID: "d1",
		Name:     "CPU温度",
		Type:     SensorTypeTemperature,
		Value:    65.5,
		Unit:     "°C",
		Threshold: 80,
	}
	if err := m.RegisterSensor(sensor); err != nil {
		t.Fatalf("RegisterSensor 失败: %v", err)
	}

	// 获取传感器
	got, err := m.GetSensor("s1")
	if err != nil {
		t.Fatalf("GetSensor 失败: %v", err)
	}
	if got.Value != 65.5 {
		t.Errorf("期望 value=65.5, 实际 %f", got.Value)
	}

	// 列表
	sensors := m.ListSensors("d1")
	if len(sensors) != 1 {
		t.Errorf("期望 1 个传感器, 实际 %d", len(sensors))
	}
}

func TestManager_Events(t *testing.T) {
	cfg := IPMIConfig{EventLimit: 100}
	m := NewManager(cfg)
	m.AddDevice(&IPMIDevice{ID: "d1", Name: "test", Host: "192.168.1.1"})

	// 触发事件
	m.PowerOn("d1")
	m.PowerOff("d1", true)

	// 获取事件
	events := m.GetEvents("d1", 10)
	if len(events) != 2 {
		t.Errorf("期望 2 个事件, 实际 %d", len(events))
	}

	// 清除事件
	m.ClearEvents("d1")
	events = m.GetEvents("d1", 10)
	if len(events) != 0 {
		t.Errorf("期望 0 个事件, 实际 %d", len(events))
	}
}

func TestManager_GetStats(t *testing.T) {
	cfg := IPMIConfig{}
	m := NewManager(cfg)
	m.AddDevice(&IPMIDevice{ID: "d1", Name: "test1", Host: "192.168.1.1"})
	m.AddDevice(&IPMIDevice{ID: "d2", Name: "test2", Host: "192.168.1.2"})

	stats := m.GetStats()
	if stats.TotalDevices != 2 {
		t.Errorf("期望 2 个设备, 实际 %d", stats.TotalDevices)
	}
}
