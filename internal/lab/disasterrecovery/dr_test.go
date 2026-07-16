package disasterrecovery

import (
	"testing"
)

func TestNewDRManager(t *testing.T) {
	mgr := NewDRManager(nil)
	if mgr == nil {
		t.Fatal("nil")
	}
}

func TestCreateAndGetPlan(t *testing.T) {
	mgr := NewDRManager(nil)
	mgr.CreatePlan("dr1", "Production DR", "desc", TierCritical)
	plan, err := mgr.GetPlan("dr1")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tier != TierCritical {
		t.Errorf("expected critical, got %s", plan.Tier)
	}
}

func TestListPlans(t *testing.T) {
	mgr := NewDRManager(nil)
	mgr.CreatePlan("dr1", "P1", "d", TierCritical)
	mgr.CreatePlan("dr2", "P2", "d", TierHigh)
	if len(mgr.ListPlans()) != 2 {
		t.Error("expected 2")
	}
}

func TestSetSites(t *testing.T) {
	mgr := NewDRManager(nil)
	mgr.CreatePlan("dr1", "T", "d", TierCritical)
	primary := &Site{ID: "s1", Name: "Primary", Location: "Beijing"}
	secondary := &Site{ID: "s2", Name: "Backup", Location: "Shanghai"}
	mgr.SetSites("dr1", primary, secondary)
	plan, _ := mgr.GetPlan("dr1")
	if plan.PrimarySite.Name != "Primary" {
		t.Error("wrong primary")
	}
}

func TestAddResourceAndStep(t *testing.T) {
	mgr := NewDRManager(nil)
	mgr.CreatePlan("dr1", "T", "d", TierCritical)
	mgr.AddResource("dr1", &Resource{ID: "r1", Name: "DB", Type: "database", Size: 1024})
	mgr.AddStep("dr1", &RecoveryStep{Name: "Stop", Action: "stop_primary"})
	plan, _ := mgr.GetPlan("dr1")
	if len(plan.Resources) != 1 {
		t.Error("expected 1 resource")
	}
	if len(plan.Steps) != 1 {
		t.Error("expected 1 step")
	}
}

func TestExecuteFailover(t *testing.T) {
	mgr := NewDRManager(nil)
	mgr.CreatePlan("dr1", "T", "d", TierCritical)
	mgr.AddStep("dr1", &RecoveryStep{Name: "Stop", Action: "stop_primary"})
	mgr.AddStep("dr1", &RecoveryStep{Name: "Start", Action: "start_secondary"})
	result, err := mgr.ExecuteFailover("dr1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.StepsRun != 2 {
		t.Errorf("expected 2 steps, got %d", result.StepsRun)
	}
}

func TestRunDRTest(t *testing.T) {
	mgr := NewDRManager(nil)
	mgr.CreatePlan("dr1", "T", "d", TierCritical)
	result, err := mgr.RunDRTest("dr1", "tabletop")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestFailback(t *testing.T) {
	mgr := NewDRManager(nil)
	mgr.CreatePlan("dr1", "T", "d", TierCritical)
	mgr.AddStep("dr1", &RecoveryStep{Name: "Stop", Action: "stop_primary"})
	mgr.ExecuteFailover("dr1")
	mgr.Failback("dr1")
	plan, _ := mgr.GetPlan("dr1")
	if plan.State != StateNormal {
		t.Errorf("expected normal, got %s", plan.State)
	}
}

func TestGetPlanStats(t *testing.T) {
	mgr := NewDRManager(nil)
	mgr.CreatePlan("dr1", "T", "d", TierCritical)
	stats, err := mgr.GetPlanStats("dr1")
	if err != nil {
		t.Fatal(err)
	}
	if stats["name"] != "T" {
		t.Error("wrong name")
	}
}

func TestPlanNotFound(t *testing.T) {
	mgr := NewDRManager(nil)
	_, err := mgr.GetPlan("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}
