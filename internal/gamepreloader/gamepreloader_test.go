package gamepreloader

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:     true,
		StoragePath: "/games",
		MaxStorage:  100 * 1024 * 1024 * 1024, // 100GB
	}
	m := NewManager(config)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.schedule.Mode != ScheduleOffPeak {
		t.Errorf("expected default schedule OffPeak, got %s", m.schedule.Mode)
	}
}

func TestAddGame(t *testing.T) {
	m := NewManager(&Config{Enabled: true})

	game := &Game{
		Name:     "原神",
		Platform: PlatformPC,
		Size:     60 * 1024 * 1024 * 1024, // 60GB
		Version:  "5.0",
		Tags:     []string{"RPG", "开放世界"},
	}

	if err := m.AddGame(game); err != nil {
		t.Fatalf("AddGame failed: %v", err)
	}
	if game.ID == "" {
		t.Error("game ID should be auto-generated")
	}
	if game.Status != StatusIdle {
		t.Errorf("expected idle status, got %s", game.Status)
	}
	if game.Priority != 5 {
		t.Errorf("default priority should be 5, got %d", game.Priority)
	}
}

func TestRemoveGame(t *testing.T) {
	m := NewManager(&Config{Enabled: true})

	game := &Game{Name: "测试游戏", Platform: PlatformPC, Size: 1024}
	m.AddGame(game)

	if err := m.RemoveGame(game.ID); err != nil {
		t.Fatalf("RemoveGame failed: %v", err)
	}

	if _, err := m.GetGame(game.ID); err == nil {
		t.Error("expected error for removed game")
	}
}

func TestRegisterDevice(t *testing.T) {
	m := NewManager(&Config{Enabled: true})

	device := &Device{
		Name:     "游戏PC",
		Type:     DevicePC,
		IP:       "192.168.1.100",
		MAC:      "AA:BB:CC:DD:EE:FF",
		Platform: PlatformPC,
	}

	if err := m.RegisterDevice(device); err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}
	if device.ID == "" {
		t.Error("device ID should be auto-generated")
	}
	if !device.Online {
		t.Error("device should be online after registration")
	}
}

func TestStartPreload(t *testing.T) {
	m := NewManager(&Config{Enabled: true})

	game := &Game{
		Name:     "王者荣耀",
		Platform: PlatformMobile,
		Size:     5 * 1024 * 1024 * 1024, // 5GB
	}
	m.AddGame(game)

	task, err := m.StartPreload(game.ID, "", ScheduleImmediate)
	if err != nil {
		t.Fatalf("StartPreload failed: %v", err)
	}
	if task.ID == "" {
		t.Error("task ID should be generated")
	}
	if task.Status != StatusPreloading {
		t.Errorf("expected preloading status, got %s", task.Status)
	}

	// 等待预加载完成
	time.Sleep(2 * time.Second)

	updatedGame, _ := m.GetGame(game.ID)
	if updatedGame.Status != StatusReady {
		t.Logf("game status after preload: %s (may still be preloading)", updatedGame.Status)
	}
}

func TestGetStorageUsage(t *testing.T) {
	m := NewManager(&Config{
		Enabled:    true,
		MaxStorage: 100 * 1024 * 1024 * 1024, // 100GB
	})

	m.AddGame(&Game{Name: "游戏1", Platform: PlatformPC, Size: 30 * 1024 * 1024 * 1024, Status: StatusReady})
	m.AddGame(&Game{Name: "游戏2", Platform: PlatformPC, Size: 20 * 1024 * 1024 * 1024, Status: StatusIdle})

	usage := m.GetStorageUsage()
	if usage["gameCount"].(int) != 1 {
		t.Errorf("expected 1 ready game, got %v", usage["gameCount"])
	}
}

func TestGetSmartRecommendations(t *testing.T) {
	m := NewManager(&Config{Enabled: true})

	// 高频游戏
	m.AddGame(&Game{
		Name:      "高频游戏",
		Platform:  PlatformPC,
		Size:      1024,
		PlayCount: 10,
		Status:    StatusIdle,
	})

	// 低频游戏
	m.AddGame(&Game{
		Name:      "低频游戏",
		Platform:  PlatformPC,
		Size:      1024,
		PlayCount: 1,
		Status:    StatusIdle,
	})

	recs := m.GetSmartRecommendations()
	found := false
	for _, r := range recs {
		if r["gameName"] == "高频游戏" {
			found = true
			if r["priority"] != "high" {
				t.Errorf("expected high priority for frequent game, got %v", r["priority"])
			}
		}
	}
	if !found {
		t.Error("expected recommendation for frequent game")
	}
}

func TestListGames(t *testing.T) {
	m := NewManager(&Config{Enabled: true})

	for i := 0; i < 5; i++ {
		m.AddGame(&Game{
			Name:     "游戏",
			Platform: PlatformPC,
			Size:     1024,
		})
	}

	games := m.ListGames()
	if len(games) != 5 {
		t.Errorf("expected 5 games, got %d", len(games))
	}
}

func TestListDevices(t *testing.T) {
	m := NewManager(&Config{Enabled: true})

	for i := 0; i < 3; i++ {
		m.RegisterDevice(&Device{
			Name: "设备",
			Type: DevicePC,
			IP:   "192.168.1.1",
		})
	}

	devices := m.ListDevices()
	if len(devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(devices))
	}
}
