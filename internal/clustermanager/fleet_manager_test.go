package clustermanager

import (
	"testing"
)

func TestFleetManager_CreateTemplate(t *testing.T) {
	fm := NewFleetManager()

	template := FleetTemplate{
		ID:       "tpl1",
		Name:     "Base Config",
		Config:   map[string]string{"hostname": "auto", "timezone": "Asia/Shanghai"},
		Packages: []string{"htop", "curl", "vim"},
		Services: []string{"ssh", "docker"},
	}

	err := fm.CreateTemplate(template)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	// Duplicate
	err = fm.CreateTemplate(template)
	if err == nil {
		t.Error("should fail on duplicate template")
	}
}

func TestFleetManager_RegisterNode(t *testing.T) {
	fm := NewFleetManager()

	node := FleetNode{
		ID:       "node1",
		Hostname: "nas-01",
		IP:       "192.168.1.100",
		Status:   NodeStatusOnline,
		Version:  "v2.586.0",
	}

	err := fm.RegisterNode(node)
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	nodes := fm.ListNodes()
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
}

func TestFleetManager_MassDeploy(t *testing.T) {
	fm := NewFleetManager()

	fm.CreateTemplate(FleetTemplate{
		ID:   "tpl1",
		Name: "Base",
	})
	fm.RegisterNode(FleetNode{ID: "node1", Status: NodeStatusOnline})
	fm.RegisterNode(FleetNode{ID: "node2", Status: NodeStatusOnline})
	fm.RegisterNode(FleetNode{ID: "node3", Status: NodeStatusOnline})

	deployment, err := fm.MassDeploy("tpl1", []string{"node1", "node2", "node3"})
	if err != nil {
		t.Fatalf("MassDeploy failed: %v", err)
	}

	if deployment.TotalNodes != 3 {
		t.Errorf("expected 3 total nodes, got %d", deployment.TotalNodes)
	}
	if deployment.SuccessCount != 3 {
		t.Errorf("expected 3 successes, got %d", deployment.SuccessCount)
	}
	if deployment.Status != FleetDeployCompleted {
		t.Errorf("expected completed, got %s", deployment.Status)
	}
}

func TestFleetManager_MassDeployPartialFailure(t *testing.T) {
	fm := NewFleetManager()

	fm.CreateTemplate(FleetTemplate{ID: "tpl1", Name: "Base"})
	fm.RegisterNode(FleetNode{ID: "node1", Status: NodeStatusOnline})

	deployment, err := fm.MassDeploy("tpl1", []string{"node1", "missing-node"})
	if err != nil {
		t.Fatalf("MassDeploy failed: %v", err)
	}

	if deployment.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", deployment.SuccessCount)
	}
	if deployment.FailCount != 1 {
		t.Errorf("expected 1 failure, got %d", deployment.FailCount)
	}
	if deployment.Status != FleetDeployPartial {
		t.Errorf("expected partial, got %s", deployment.Status)
	}
}

func TestFleetManager_GroupDeploy(t *testing.T) {
	fm := NewFleetManager()

	fm.CreateTemplate(FleetTemplate{ID: "tpl1", Name: "Base"})
	fm.RegisterNode(FleetNode{ID: "node1", Status: NodeStatusOnline})
	fm.RegisterNode(FleetNode{ID: "node2", Status: NodeStatusOnline})

	fm.CreateGroup(FleetGroup{
		ID:      "grp1",
		Name:    "Production",
		NodeIDs: []string{"node1", "node2"},
	})

	deployment, err := fm.DeployToGroup("tpl1", "grp1")
	if err != nil {
		t.Fatalf("DeployToGroup failed: %v", err)
	}

	if deployment.TotalNodes != 2 {
		t.Errorf("expected 2 nodes, got %d", deployment.TotalNodes)
	}
}

func TestFleetManager_FleetStats(t *testing.T) {
	fm := NewFleetManager()

	fm.RegisterNode(FleetNode{ID: "n1", Status: NodeStatusOnline})
	fm.RegisterNode(FleetNode{ID: "n2", Status: NodeStatusOnline})
	fm.RegisterNode(FleetNode{ID: "n3", Status: NodeStatusOffline})
	fm.RegisterNode(FleetNode{ID: "n4", Status: NodeStatusError})

	stats := fm.GetFleetStats()
	if stats["total_nodes"] != 4 {
		t.Errorf("expected 4 total, got %d", stats["total_nodes"])
	}
	if stats["online"] != 2 {
		t.Errorf("expected 2 online, got %d", stats["online"])
	}
	if stats["offline"] != 1 {
		t.Errorf("expected 1 offline, got %d", stats["offline"])
	}
}

func TestFleetManager_MissingTemplate(t *testing.T) {
	fm := NewFleetManager()

	_, err := fm.MassDeploy("nonexistent", []string{"node1"})
	if err == nil {
		t.Error("should fail with missing template")
	}
}
