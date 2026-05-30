package ups

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := DefaultUPSConfig()
	manager := NewManager(config)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.config.Name != "ups1" {
		t.Errorf("Expected name 'ups1', got %s", manager.config.Name)
	}

	if manager.config.LowBatteryPct != 20 {
		t.Errorf("Expected low battery percent 20, got %d", manager.config.LowBatteryPct)
	}
}

func TestUPSStatus(t *testing.T) {
	config := DefaultUPSConfig()
	manager := NewManager(config)

	status := manager.GetStatus()

	if status.Name != "ups1" {
		t.Errorf("Expected name 'ups1', got %s", status.Name)
	}

	if status.Status == "" {
		t.Error("Expected non-empty status")
	}
}

func TestUPSConfig(t *testing.T) {
	config := DefaultUPSConfig()

	// 测试配置更新
	config.LowBatteryPct = 30
	config.ShutdownDelay = 120

	manager := NewManager(config)

	if manager.config.LowBatteryPct != 30 {
		t.Errorf("Expected low battery percent 30, got %d", manager.config.LowBatteryPct)
	}

	if manager.config.ShutdownDelay != 120 {
		t.Errorf("Expected shutdown delay 120, got %d", manager.config.ShutdownDelay)
	}
}

func TestUPSManagerStartStop(t *testing.T) {
	config := DefaultUPSConfig()
	config.PollInterval = 1 // 1秒轮询
	manager := NewManager(config)

	// 启动管理器
	manager.Start()

	// 等待一小段时间
	time.Sleep(100 * time.Millisecond)

	// 停止管理器
	manager.Stop()
}

func TestGetBatteryHealthScore(t *testing.T) {
	status := UPSStatus{
		BatteryCharge: 95,
		Temperature:   22,
		LoadPercent:   50,
	}

	score := GetBatteryHealthScore(status)
	if score < 0 || score > 100 {
		t.Errorf("Invalid health score: %d", score)
	}

	// 测试高温情况
	hotStatus := UPSStatus{
		BatteryCharge: 95,
		Temperature:   35,
		LoadPercent:   50,
	}

	hotScore := GetBatteryHealthScore(hotStatus)
	if hotScore >= score {
		t.Errorf("Expected lower score for hot temperature, got %d vs %d", hotScore, score)
	}
}

func TestUPSManagerString(t *testing.T) {
	config := DefaultUPSConfig()
	manager := NewManager(config)

	str := manager.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
}
