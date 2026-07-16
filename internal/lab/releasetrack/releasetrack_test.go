package releasetrack

import (
	"testing"
)

func TestDefaultChannels(t *testing.T) {
	m := NewManager()
	channels := m.ListChannels()
	if len(channels) != 5 {
		t.Errorf("expected 5 default channels, got %d", len(channels))
	}
}

func TestPublishAndList(t *testing.T) {
	m := NewManager()
	r := &Release{
		Version:      "v3.18.0",
		Channel:      ChannelStable,
		ReleaseNotes: "Test release",
		Checksum:     "sha256:abc123",
		SizeBytes:    1024 * 1024 * 100,
	}
	if err := m.PublishRelease(r); err != nil {
		t.Fatalf("PublishRelease failed: %v", err)
	}
	releases := m.ListReleases(ChannelStable)
	if len(releases) != 1 {
		t.Errorf("expected 1 release, got %d", len(releases))
	}
}

func TestPromote(t *testing.T) {
	m := NewManager()
	m.PublishRelease(&Release{
		Version: "v3.18.0-beta", Channel: ChannelBeta,
		ReleaseNotes: "Beta", Checksum: "sha256:x",
	})
	if err := m.PromoteRelease("v3.18.0-beta", ChannelBeta, ChannelStable); err != nil {
		t.Fatalf("PromoteRelease failed: %v", err)
	}
	stable := m.ListReleases(ChannelStable)
	if len(stable) != 1 {
		t.Fatalf("expected 1 stable release, got %d", len(stable))
	}
	if stable[0].Version != "v3.18.0-beta" {
		t.Errorf("unexpected version: %s", stable[0].Version)
	}
}

func TestSetCurrent(t *testing.T) {
	m := NewManager()
	m.PublishRelease(&Release{Version: "v1.0", Channel: ChannelStable, ReleaseNotes: "v1"})
	m.SetCurrentRelease(ChannelStable, "v1.0")
	r, err := m.GetCurrentRelease(ChannelStable)
	if err != nil {
		t.Fatalf("GetCurrentRelease failed: %v", err)
	}
	if r.Version != "v1.0" {
		t.Errorf("expected v1.0, got %s", r.Version)
	}
}

func TestRollback(t *testing.T) {
	m := NewManager()
	m.PublishRelease(&Release{Version: "v2.0", Channel: ChannelStable, ReleaseNotes: "v2"})
	m.SetCurrentRelease(ChannelStable, "v2.0")
	if err := m.RollbackRelease("v2.0", ChannelStable, "critical bug"); err != nil {
		t.Fatalf("RollbackRelease failed: %v", err)
	}
	r, _ := m.GetCurrentRelease(ChannelStable)
	// After rollback, current should be empty
	if r != nil {
		t.Error("expected nil current release after rollback")
	}
	// Check release is marked
	releases := m.ListReleases(ChannelStable)
	if !releases[0].RolledBack {
		t.Error("expected release to be rolled back")
	}
}

func TestDeployment(t *testing.T) {
	m := NewManager()
	dep, err := m.StartDeployment("v3.0", ChannelStable, 1000)
	if err != nil {
		t.Fatalf("StartDeployment failed: %v", err)
	}
	if dep.ID == "" {
		t.Error("expected deployment ID")
	}
	if err := m.CompleteDeployment(dep.ID, 980, 20); err != nil {
		t.Fatalf("CompleteDeployment failed: %v", err)
	}
	deployments := m.ListDeployments()
	if len(deployments) != 1 {
		t.Errorf("expected 1 deployment, got %d", len(deployments))
	}
	if deployments[0].Status != "completed" {
		t.Errorf("expected completed, got %s", deployments[0].Status)
	}
}

func TestRollbackPlan(t *testing.T) {
	m := NewManager()
	plan := m.CreateRollbackPlan("v3.0", "v2.9", ChannelStable, "regression detected")
	if plan == nil {
		t.Fatal("expected rollback plan")
	}
}

func TestUnknownChannel(t *testing.T) {
	m := NewManager()
	err := m.PublishRelease(&Release{Version: "v1", Channel: "unknown"})
	if err == nil {
		t.Error("expected error for unknown channel")
	}
}