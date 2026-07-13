package iscsitgtoffload

import (
	"fmt"
	"testing"
)

func TestEngineRegistration(t *testing.T) {
	m := NewManager()
	e := &OffloadEngine{
		Name:    "mlx5offload",
		Type:    OffloadTypeNIC,
		Device:  "mlx5_0",
		PCISlot: "0000:01:00.0",
		MaxTargets: 64,
		MaxSessions: 512,
		MaxLunMBps: 12800,
		Firmware: "16.32.1010",
	}
	if err := m.RegisterEngine(e); err != nil {
		t.Fatalf("RegisterEngine failed: %v", err)
	}
	if e.ID == "" {
		t.Error("expected engine ID")
	}
	if e.Status != OffloadStatusDisabled {
		t.Errorf("expected disabled, got %s", e.Status)
	}
	engines := m.ListEngines()
	if len(engines) != 1 {
		t.Errorf("expected 1 engine, got %d", len(engines))
	}
}

func TestEnableDisable(t *testing.T) {
	m := NewManager()
	e := &OffloadEngine{Name: "test-engine", Type: OffloadTypeDPU, MaxTargets: 32, MaxSessions: 256, MaxLunMBps: 5000}
	m.RegisterEngine(e)
	if err := m.EnableOffload(e.ID); err != nil {
		t.Fatalf("EnableOffload failed: %v", err)
	}
	if e.Status != OffloadStatusEnabled {
		t.Errorf("expected enabled, got %s", e.Status)
	}
	if err := m.EnableOffload(e.ID); err == nil {
		t.Error("expected error enabling already enabled")
	}
	if err := m.DisableOffload(e.ID); err != nil {
		t.Fatalf("DisableOffload failed: %v", err)
	}
	if e.Status != OffloadStatusDisabled {
		t.Errorf("expected disabled, got %s", e.Status)
	}
}

func TestAssignTarget(t *testing.T) {
	m := NewManager()
	e := &OffloadEngine{Name: "engine1", Type: OffloadTypeNIC, MaxTargets: 2, MaxSessions: 100, MaxLunMBps: 10000}
	m.RegisterEngine(e)
	m.EnableOffload(e.ID)
	tgt := &OffloadTarget{
		IQN: "iqn.2026-01.local:target1",
		LunCount: 1,
		EngineID: e.ID,
	}
	if err := m.AssignTarget(tgt); err != nil {
		t.Fatalf("AssignTarget failed: %v", err)
	}
	if tgt.ID == "" {
		t.Error("expected target ID")
	}
	if e.ActiveTargets != 1 {
		t.Errorf("expected 1 active, got %d", e.ActiveTargets)
	}
	// Max targets
	tgt2 := &OffloadTarget{IQN: "iqn.2026-01.local:target2", LunCount: 1, EngineID: e.ID}
	m.AssignTarget(tgt2)
	tgt3 := &OffloadTarget{IQN: "iqn.2026-01.local:target3", LunCount: 1, EngineID: e.ID}
	if err := m.AssignTarget(tgt3); err == nil {
		t.Error("expected error reaching max targets")
	}
	if err := m.RemoveTarget(tgt.ID); err != nil {
		t.Fatalf("RemoveTarget failed: %v", err)
	}
	if e.ActiveTargets != 1 {
		t.Errorf("expected 1 active, got %d", e.ActiveTargets)
	}
}

func TestAssignDisabledEngine(t *testing.T) {
	m := NewManager()
	e := &OffloadEngine{Name: "disabled-engine", Type: OffloadTypeHBAL, MaxTargets: 10, MaxSessions: 50, MaxLunMBps: 2000}
	m.RegisterEngine(e)
	// not enabled
	if err := m.AssignTarget(&OffloadTarget{IQN: "test", EngineID: e.ID}); err == nil {
		t.Error("expected error assigning to disabled engine")
	}
}

func TestHealthCheck(t *testing.T) {
	m := NewManager()
	e1 := &OffloadEngine{Name: "healthy", Type: OffloadTypeNIC, Status: OffloadStatusEnabled, MaxTargets: 10, MaxSessions: 100, Temperature: 50}
	m.RegisterEngine(e1)
	e2 := &OffloadEngine{Name: "hot", Type: OffloadTypeNIC, Status: OffloadStatusEnabled, Temperature: 90, MaxTargets: 10, MaxSessions: 100}
	m.RegisterEngine(e2)
	e3 := &OffloadEngine{Name: "failed", Type: OffloadTypeNIC, Status: OffloadStatusFailed, MaxTargets: 10, MaxSessions: 100}
	m.RegisterEngine(e3)
	health := m.HealthCheck()
	if health[e1.ID] != "enabled" {
		t.Errorf("expected enabled, got %s", health[e1.ID])
	}
	if health[e2.ID] != "overheating" {
		t.Errorf("expected overheating, got %s", health[e2.ID])
	}
	if health[e3.ID] != "failed" {
		t.Errorf("expected failed, got %s", health[e3.ID])
	}
}

func TestRecommendEngine(t *testing.T) {
	m := NewManager()
	e1 := &OffloadEngine{Name: "fast", Type: OffloadTypeDPU, Status: OffloadStatusEnabled, MaxTargets: 10, MaxSessions: 100, MaxLunMBps: 20000}
	m.RegisterEngine(e1)
	e2 := &OffloadEngine{Name: "slow", Type: OffloadTypeHBAL, Status: OffloadStatusEnabled, MaxTargets: 10, MaxSessions: 100, MaxLunMBps: 2000}
	m.RegisterEngine(e2)
	// fill up fast
	for i := 0; i < 10; i++ {
		m.AssignTarget(&OffloadTarget{IQN: fmt.Sprintf("iqn:test:%d", i), EngineID: e1.ID})
	}
	// fast is full, should recommend slow
	rec := m.RecommendEngine(1000)
	if rec == nil {
		t.Fatal("expected recommendation")
	}
	if rec.ID != e2.ID {
		t.Errorf("expected %s, got %s", e2.ID, rec.ID)
	}
}