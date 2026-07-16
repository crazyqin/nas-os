package dashboardoverview

import (
	"testing"
)

func TestGetOverview(t *testing.T) {
	mgr := NewManager()

	overview := mgr.GetOverview()
	if overview == nil {
		t.Fatal("expected non-nil overview")
	}
	if overview.System.Hostname != "nas-server" {
		t.Errorf("expected nas-server, got %s", overview.System.Hostname)
	}
	if overview.CPU.Cores != 8 {
		t.Errorf("expected 8 cores, got %d", overview.CPU.Cores)
	}
}

func TestGetCPU(t *testing.T) {
	mgr := NewManager()

	cpu := mgr.GetCPU()
	if cpu.Model != "RK3588" {
		t.Errorf("expected RK3588, got %s", cpu.Model)
	}
	if len(cpu.PerCore) != 8 {
		t.Errorf("expected 8 per-core values, got %d", len(cpu.PerCore))
	}
}

func TestGetMemory(t *testing.T) {
	mgr := NewManager()

	mem := mgr.GetMemory()
	if mem.Total == 0 {
		t.Error("expected non-zero total memory")
	}
	if mem.Usage < 0 || mem.Usage > 100 {
		t.Errorf("invalid memory usage: %f", mem.Usage)
	}
}

func TestGetStorage(t *testing.T) {
	mgr := NewManager()

	storage := mgr.GetStorage()
	if len(storage) == 0 {
		t.Error("expected at least 1 storage pool")
	}
	if storage[0].Name != "main-pool" {
		t.Errorf("expected main-pool, got %s", storage[0].Name)
	}
}

func TestGetNetwork(t *testing.T) {
	mgr := NewManager()

	network := mgr.GetNetwork()
	if len(network) == 0 {
		t.Error("expected at least 1 network interface")
	}
	if !network[0].IsUp {
		t.Error("expected network to be up")
	}
}

func TestGetServices(t *testing.T) {
	mgr := NewManager()

	services := mgr.GetServices()
	if len(services) == 0 {
		t.Error("expected at least 1 service")
	}

	running := 0
	for _, s := range services {
		if s.Status == "running" {
			running++
		}
	}
	if running == 0 {
		t.Error("expected at least 1 running service")
	}
}

func TestAlerts(t *testing.T) {
	mgr := NewManager()

	mgr.AddAlert(AlertItem{
		ID:       "alert-1",
		Type:     "disk",
		Message:  "磁盘空间不足",
		Severity: "warning",
	})

	alerts := mgr.GetAlerts(false)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}

	// 确认告警
	mgr.AckAlert("alert-1")
	alerts = mgr.GetAlerts(false)
	if len(alerts) != 0 {
		t.Errorf("expected 0 unacked alerts, got %d", len(alerts))
	}

	// 包含已确认
	alerts = mgr.GetAlerts(true)
	if len(alerts) != 1 {
		t.Errorf("expected 1 total alert, got %d", len(alerts))
	}
}

func TestActivity(t *testing.T) {
	mgr := NewManager()

	mgr.AddActivity(ActivityItem{
		ID:      "act-1",
		Type:    "login",
		Message: "用户登录",
		User:    "admin",
	})

	overview := mgr.GetOverview()
	if len(overview.Recent) == 0 {
		t.Error("expected at least 1 activity")
	}
}

func TestWidgets(t *testing.T) {
	mgr := NewManager()

	widgets := mgr.GetWidgets()
	if len(widgets) < 5 {
		t.Errorf("expected at least 5 widgets, got %d", len(widgets))
	}
}
