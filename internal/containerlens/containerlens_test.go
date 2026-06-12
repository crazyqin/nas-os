package containerlens

import (
	"context"
	"testing"
)

func TestRegisterContainer(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	container := &Container{
		ID:          "container-1",
		Name:        "test-container",
		Image:       "nginx:latest",
		Status:      "running",
		PID:         1234,
		NetworkMode: "bridge",
		Privileged:  false,
	}

	err := cl.RegisterContainer(ctx, container)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cl.containers["container-1"] == nil {
		t.Fatal("container not registered")
	}
}

func TestRegisterContainerNoID(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	container := &Container{
		Name: "no-id",
	}

	err := cl.RegisterContainer(ctx, container)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestDetectAnomaly(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	cl.RegisterContainer(ctx, &Container{
		ID:         "container-1",
		Name:       "test-container",
		Privileged: true,
	})

	event, err := cl.DetectAnomaly(ctx, "container-1", "unusual_network_activity")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Type != "anomaly" {
		t.Errorf("expected type anomaly, got %s", event.Type)
	}
	if event.Severity != "high" { // privileged container
		t.Errorf("expected severity high, got %s", event.Severity)
	}
}

func TestDetectAnomalyNotFound(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	_, err := cl.DetectAnomaly(ctx, "nonexistent", "behavior")
	if err == nil {
		t.Fatal("expected error for nonexistent container")
	}
}

func TestScanImage(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	result, err := cl.ScanImage(ctx, "nginx:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Image != "nginx:latest" {
		t.Errorf("expected image nginx:latest, got %s", result.Image)
	}
}

func TestAddPolicy(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	policy := &PolicyRule{
		ID:          "policy-1",
		Name:        "No Privileged Containers",
		Description: "Block privileged containers",
		Type:        "runtime",
		Enabled:     true,
		Conditions:  []string{"privileged == true"},
		Action:      "block",
		Severity:    "high",
	}

	err := cl.AddPolicy(ctx, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	policies := cl.GetPolicies(ctx)
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}
}

func TestAddPolicyNoID(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	policy := &PolicyRule{
		Name: "No ID",
	}

	err := cl.AddPolicy(ctx, policy)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestRunComplianceCheck(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	cl.RegisterContainer(ctx, &Container{
		ID:          "container-1",
		Name:        "test-container",
		Privileged:  true,
		NetworkMode: "host",
	})

	checks, err := cl.RunComplianceCheck(ctx, "container-1", "CIS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(checks))
	}

	// Both should fail
	for _, check := range checks {
		if check.Status != "fail" {
			t.Errorf("expected fail, got %s for %s", check.Status, check.Rule)
		}
	}
}

func TestRunComplianceCheckPass(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	cl.RegisterContainer(ctx, &Container{
		ID:          "container-1",
		Name:        "test-container",
		Privileged:  false,
		NetworkMode: "bridge",
	})

	checks, err := cl.RunComplianceCheck(ctx, "container-1", "CIS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, check := range checks {
		if check.Status != "pass" {
			t.Errorf("expected pass, got %s for %s", check.Status, check.Rule)
		}
	}
}

func TestRunComplianceCheckNotFound(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	_, err := cl.RunComplianceCheck(ctx, "nonexistent", "CIS")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetEvents(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	cl.RegisterContainer(ctx, &Container{
		ID:   "container-1",
		Name: "test-container",
	})

	cl.DetectAnomaly(ctx, "container-1", "behavior1")
	cl.DetectAnomaly(ctx, "container-1", "behavior2")

	events := cl.GetEvents(ctx, "container-1")
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	// Empty filter returns all
	events = cl.GetEvents(ctx, "")
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestGetVulnerabilities(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	// Add some vulns
	cl.vulns = []Vulnerability{
		{ID: "v1", Severity: "high"},
		{ID: "v2", Severity: "low"},
		{ID: "v3", Severity: "high"},
	}

	highVulns := cl.GetVulnerabilities(ctx, "high")
	if len(highVulns) != 2 {
		t.Errorf("expected 2 high vulns, got %d", len(highVulns))
	}

	allVulns := cl.GetVulnerabilities(ctx, "")
	if len(allVulns) != 3 {
		t.Errorf("expected 3 total vulns, got %d", len(allVulns))
	}
}

func TestGetScanResult(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	cl.ScanImage(ctx, "nginx:latest")

	result, err := cl.GetScanResult(ctx, "nginx:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Image != "nginx:latest" {
		t.Errorf("expected image nginx:latest, got %s", result.Image)
	}
}

func TestGetScanResultNotFound(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	_, err := cl.GetScanResult(ctx, "nonexistent:latest")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetPolicies(t *testing.T) {
	cl := NewContainerLens()
	ctx := context.Background()

	// Empty
	policies := cl.GetPolicies(ctx)
	if len(policies) != 0 {
		t.Errorf("expected 0 policies, got %d", len(policies))
	}

	// Add one
	cl.AddPolicy(ctx, &PolicyRule{
		ID:   "p1",
		Name: "Test Policy",
	})

	policies = cl.GetPolicies(ctx)
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}
}
