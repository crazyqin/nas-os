package containerha

import (
	"testing"
	"time"
)

func TestEnableHA(t *testing.T) {
	s := Signal{
		HAEnabled:      false,
		ContainerCount: 10,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "enable-ha" {
		t.Errorf("expected ID enable-ha, got %s", recs[0].ID)
	}
	if recs[0].Priority != "critical" {
		t.Errorf("expected priority critical, got %s", recs[0].Priority)
	}
}

func TestConfigureStaticIP(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: false,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      true,
		HasWitnessNode:         true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "configure-static-ip" {
		t.Errorf("expected ID configure-static-ip, got %s", recs[0].ID)
	}
	if recs[0].Priority != "high" {
		t.Errorf("expected priority high, got %s", recs[0].Priority)
	}
}

func TestCoverAllContainers(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     10,
		ContainersWithHA:   5,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      true,
		HasWitnessNode:         true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "cover-all-containers" {
		t.Errorf("expected ID cover-all-containers, got %s", recs[0].ID)
	}
	if recs[0].Priority != "high" {
		t.Errorf("expected priority high, got %s", recs[0].Priority)
	}
}

func TestTestFailover(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   31 * 24 * time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      true,
		HasWitnessNode:         true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "test-failover" {
		t.Errorf("expected ID test-failover, got %s", recs[0].ID)
	}
	if recs[0].Priority != "medium" {
		t.Errorf("expected priority medium, got %s", recs[0].Priority)
	}
}

func TestFixFailoverTest(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: false,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      true,
		HasWitnessNode:         true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "fix-failover-test" {
		t.Errorf("expected ID fix-failover-test, got %s", recs[0].ID)
	}
	if recs[0].Priority != "critical" {
		t.Errorf("expected priority critical, got %s", recs[0].Priority)
	}
}

func TestAddClusterNodes(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   1,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      true,
		HasWitnessNode:         true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "add-cluster-nodes" {
		t.Errorf("expected ID add-cluster-nodes, got %s", recs[0].ID)
	}
	if recs[0].Priority != "critical" {
		t.Errorf("expected priority critical, got %s", recs[0].Priority)
	}
}

func TestEnableStorageReplication(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      false,
		HasWitnessNode:         true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "enable-storage-replication" {
		t.Errorf("expected ID enable-storage-replication, got %s", recs[0].ID)
	}
	if recs[0].Priority != "high" {
		t.Errorf("expected priority high, got %s", recs[0].Priority)
	}
}

func TestAddWitnessNode(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      true,
		HasWitnessNode:         false,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "add-witness-node" {
		t.Errorf("expected ID add-witness-node, got %s", recs[0].ID)
	}
	if recs[0].Priority != "medium" {
		t.Errorf("expected priority medium, got %s", recs[0].Priority)
	}
}

func TestFixSplitBrain(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      true,
		HasWitnessNode:         true,
		SplitBrainDetected:     true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "fix-split-brain" {
		t.Errorf("expected ID fix-split-brain, got %s", recs[0].ID)
	}
	if recs[0].Priority != "critical" {
		t.Errorf("expected priority critical, got %s", recs[0].Priority)
	}
}

func TestIncreaseCPURedundancy(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 80,
		MemFailoverCapacityGB:  16,
		StorageReplicated:      true,
		HasWitnessNode:         true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "increase-cpu-redundancy" {
		t.Errorf("expected ID increase-cpu-redundancy, got %s", recs[0].ID)
	}
	if recs[0].Priority != "medium" {
		t.Errorf("expected priority medium, got %s", recs[0].Priority)
	}
}

func TestIncreaseMemRedundancy(t *testing.T) {
	s := Signal{
		HAEnabled:          true,
		StaticIPConfigured: true,
		ContainerCount:     3,
		ContainersWithHA:   3,
		FailoverTestedAt:   time.Hour,
		FailoverTestPassed: true,
		ClusterNodeCount:   2,
		CPUFailoverCapacityPct: 100,
		MemFailoverCapacityGB:  8,
		StorageReplicated:      true,
		HasWitnessNode:         true,
	}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "increase-mem-redundancy" {
		t.Errorf("expected ID increase-mem-redundancy, got %s", recs[0].ID)
	}
	if recs[0].Priority != "medium" {
		t.Errorf("expected priority medium, got %s", recs[0].Priority)
	}
}

func TestEmptySignal(t *testing.T) {
	s := Signal{}
	recs := Analyze(s)
	// HAEnabled=false, ContainerCount=0 (not >5) → no enable-ha
	// All HA-enabled conditions are false → no other recs
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations for empty signal, got %d", len(recs))
	}
}

func TestPriorityOrdering(t *testing.T) {
	// Trigger multiple recommendations across different priority levels
	s := Signal{
		HAEnabled:              true,
		StaticIPConfigured:     false, // high
		ContainerCount:         10,
		ContainersWithHA:       3,    // high
		FailoverTestedAt:       31 * 24 * time.Hour, // medium
		FailoverTestPassed:     false, // critical
		ClusterNodeCount:       1,    // critical
		CPUFailoverCapacityPct: 50,   // medium
		MemFailoverCapacityGB:  4,    // medium
		StorageReplicated:      false, // high
		HasWitnessNode:         false, // medium
		SplitBrainDetected:     true,  // critical
	}
	recs := Analyze(s)
	if len(recs) < 2 {
		t.Fatalf("expected multiple recommendations, got %d", len(recs))
	}
	// Verify ordering: critical < high < medium
	for i := 0; i < len(recs)-1; i++ {
		rankI := priorityRank[recs[i].Priority]
		rankJ := priorityRank[recs[i+1].Priority]
		if rankI > rankJ {
			t.Errorf("recommendations not sorted by priority: %s (rank %d) before %s (rank %d)",
				recs[i].Priority, rankI, recs[i+1].Priority, rankJ)
		}
	}
	// Verify at least one critical is first
	if recs[0].Priority != "critical" {
		t.Errorf("expected first recommendation to be critical, got %s", recs[0].Priority)
	}
}