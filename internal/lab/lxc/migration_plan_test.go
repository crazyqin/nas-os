package lxc

import "testing"

func TestBuildMigrationPlanModes(t *testing.T) {
	running := &Container{ID: "ct1", Status: StatusRunning}
	plan, err := BuildMigrationPlan(running, "a", "b", true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "live" || len(plan.Steps) == 0 || plan.Steps[len(plan.Steps)-1].Name != "cutover" {
		t.Fatalf("unexpected live plan: %+v", plan)
	}

	stopped := &Container{ID: "ct2", Status: StatusStopped}
	plan, err = BuildMigrationPlan(stopped, "a", "b", true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "cold" {
		t.Fatalf("expected cold plan, got %s", plan.Mode)
	}
}

func TestBuildMigrationPlanRequiresInputs(t *testing.T) {
	if _, err := BuildMigrationPlan(nil, "a", "b", false); err == nil {
		t.Fatal("expected nil container error")
	}
	if _, err := BuildMigrationPlan(&Container{ID: "ct"}, "", "b", false); err == nil {
		t.Fatal("expected node validation error")
	}
}
