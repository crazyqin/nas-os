package clusterops

import (
	"testing"
	"time"
)

func TestAnalyze_EnableHA(t *testing.T) {
	recs := Analyze(Signal{
		TotalNodes: 2,
		HasHA:      false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-enable-ha" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected cluster-enable-ha recommendation")
	}
}

func TestAnalyze_PrimaryOfflineWithFailover(t *testing.T) {
	recs := Analyze(Signal{
		HasPrimaryOffline: true,
		HAFailoverEnabled:  true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-primary-failover" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-primary-failover recommendation")
	}
}

func TestAnalyze_PrimaryOfflineNoFailover(t *testing.T) {
	recs := Analyze(Signal{
		HasPrimaryOffline: true,
		HAFailoverEnabled:  false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-primary-down-no-failover" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-primary-down-no-failover recommendation")
	}
}

func TestAnalyze_SplitBrain(t *testing.T) {
	recs := Analyze(Signal{
		HasSplitBrain: true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-split-brain" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected cluster-split-brain recommendation")
	}
}

func TestAnalyze_WitnessDown(t *testing.T) {
	recs := Analyze(Signal{
		TotalNodes:        3,
		HasHA:             true,
		WitnessNodeOnline: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-witness-down" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-witness-down recommendation")
	}
}

func TestAnalyze_DiskFull(t *testing.T) {
	recs := Analyze(Signal{
		Nodes: []Node{
			{ID: "n1", Name: "nas1", Status: StatusDegraded, DiskUsagePct: 95},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-disk-full-n1" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-disk-full-n1 recommendation")
	}
}

func TestAnalyze_ReplicationLag(t *testing.T) {
	recs := Analyze(Signal{
		Nodes: []Node{
			{ID: "n2", Name: "nas2", Status: StatusOnline, ReplicationLag: 2 * time.Hour},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-repl-lag-n2" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-repl-lag-n2 recommendation")
	}
}

func TestAnalyze_HighCPU(t *testing.T) {
	recs := Analyze(Signal{
		Nodes: []Node{
			{ID: "n3", Name: "nas3", Status: StatusOnline, CPUUsagePct: 95},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-cpu-high-n3" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-cpu-high-n3 recommendation")
	}
}

func TestAnalyze_PostFailoverCheck(t *testing.T) {
	recs := Analyze(Signal{
		ReplicationEnabled: true,
		LastFailoverAge:    2 * time.Hour,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-post-failover-check" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-post-failover-check recommendation")
	}
}

func TestAnalyze_SingleNode(t *testing.T) {
	recs := Analyze(Signal{
		TotalNodes:  3,
		OnlineNodes: 1,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-single-node" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-single-node recommendation")
	}
}

func TestAnalyze_LowHealthScore(t *testing.T) {
	recs := Analyze(Signal{
		ClusterHealthScore: 40,
	})
	found := false
	for _, r := range recs {
		if r.ID == "cluster-health-low" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster-health-low recommendation")
	}
}

func TestAnalyze_EmptySignal(t *testing.T) {
	recs := Analyze(Signal{})
	if len(recs) != 0 {
		t.Fatalf("expected no recommendations for empty signal, got %d", len(recs))
	}
}

func TestAnalyze_PriorityOrdering(t *testing.T) {
	recs := Analyze(Signal{
		TotalNodes:        2,
		HasHA:             false,
		HasSplitBrain:     true,
		WitnessNodeOnline: true,
	})
	if len(recs) < 2 {
		t.Fatal("expected multiple recommendations")
	}
	for i := 0; i < len(recs)-1; i++ {
		if priorityRank(recs[i].Priority) > priorityRank(recs[i+1].Priority) {
			t.Errorf("recommendations not sorted by priority at index %d", i)
		}
	}
}