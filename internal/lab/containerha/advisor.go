package containerha

import (
	"sort"
	"time"
)

// Signal holds the current state of container HA configuration for analysis.
type Signal struct {
	HAEnabled              bool
	ContainerCount         int
	ContainersWithHA       int
	StaticIPConfigured     bool
	FailoverTestedAt       time.Duration
	FailoverTestPassed     bool
	ClusterNodeCount       int
	PendingUpdates         int
	CPUFailoverCapacityPct int
	MemFailoverCapacityGB  int
	StorageReplicated      bool
	HasWitnessNode         bool
	SplitBrainDetected     bool
}

// Recommendation represents a single HA improvement suggestion.
type Recommendation struct {
	ID       string
	Title    string
	Priority string
	Action   string
	Reason   string
}

// Priority ranking values (lower index = higher priority).
var priorityRank = map[string]int{
	"critical": 0,
	"high":      1,
	"medium":    2,
	"low":       3,
	"info":      4,
}

// Analyze examines the container HA signal and returns ordered recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	// 1. HA not enabled with significant container count
	if !s.HAEnabled && s.ContainerCount > 5 {
		recs = append(recs, Recommendation{
			ID:       "enable-ha",
			Title:    "Enable Container High Availability",
			Priority: "critical",
			Action:   "Enable HA mode for the container service to allow automatic failover.",
			Reason:   "Running more than 5 containers without HA increases the risk of prolonged downtime during node failure.",
		})
	}

	// 2. HA enabled but static IP not configured
	if s.HAEnabled && !s.StaticIPConfigured {
		recs = append(recs, Recommendation{
			ID:       "configure-static-ip",
			Title:    "Configure Static IP for HA Failover",
			Priority: "high",
			Action:   "Assign a static IP address to the HA cluster VIP for reliable client connections during failover.",
			Reason:   "Without a static IP, DNS / clients may not reliably reach the failover target after a node failure.",
		})
	}

	// 3. Not all containers covered by HA
	if s.HAEnabled && s.ContainersWithHA < s.ContainerCount {
		recs = append(recs, Recommendation{
			ID:       "cover-all-containers",
			Title:    "Include All Containers in HA",
			Priority: "high",
			Action:   "Add the remaining containers to the HA protection group so they fail over automatically.",
			Reason:   "Only some containers are HA-protected; unprotected containers will not survive a node failure.",
		})
	}

	// 4. Failover test overdue
	if s.HAEnabled && s.FailoverTestedAt > 30*24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "test-failover",
			Title:    "Run a Failover Test",
			Priority: "medium",
			Action:   "Perform a controlled failover test to validate the HA configuration is working.",
			Reason:   "Failover has not been tested in over 30 days; configuration drift may cause silent failures.",
		})
	}

	// 5. Failover test previously failed
	if s.HAEnabled && !s.FailoverTestPassed {
		recs = append(recs, Recommendation{
			ID:       "fix-failover-test",
			Title:    "Fix Failover Test Failures",
			Priority: "critical",
			Action:   "Investigate and resolve the issues causing the last failover test to fail.",
			Reason:   "The most recent failover test did not pass; HA may not function correctly during a real outage.",
		})
	}

	// 6. Insufficient cluster nodes
	if s.HAEnabled && s.ClusterNodeCount < 2 {
		recs = append(recs, Recommendation{
			ID:       "add-cluster-nodes",
			Title:    "Add Cluster Nodes for HA",
			Priority: "critical",
			Action:   "Add at least one more node to the cluster to enable true HA failover.",
			Reason:   "A single-node cluster cannot provide HA; at least two nodes are required for failover.",
		})
	}

	// 7. Storage not replicated
	if s.HAEnabled && !s.StorageReplicated {
		recs = append(recs, Recommendation{
			ID:       "enable-storage-replication",
			Title:    "Enable Storage Replication",
			Priority: "high",
			Action:   "Configure storage replication between cluster nodes so containers can restart on the failover node.",
			Reason:   "Without replicated storage, container data is not available on the failover node, preventing recovery.",
		})
	}

	// 8. No witness node
	if s.HAEnabled && !s.HasWitnessNode {
		recs = append(recs, Recommendation{
			ID:       "add-witness-node",
			Title:    "Configure a Witness Node",
			Priority: "medium",
			Action:   "Add a witness (arbiter) node to the cluster to maintain quorum during split-brain scenarios.",
			Reason:   "Without a witness node, the cluster cannot maintain quorum if nodes become partitioned.",
		})
	}

	// 9. Split brain detected
	if s.HAEnabled && s.SplitBrainDetected {
		recs = append(recs, Recommendation{
			ID:       "fix-split-brain",
			Title:    "Resolve Split-Brain Condition",
			Priority: "critical",
			Action:   "Investigate and resolve the split-brain condition immediately to prevent data corruption.",
			Reason:   "Split-brain detected: multiple nodes believe they are the active master, risking data divergence.",
		})
	}

	// 10. Insufficient CPU failover capacity
	if s.HAEnabled && s.CPUFailoverCapacityPct < 100 {
		recs = append(recs, Recommendation{
			ID:       "increase-cpu-redundancy",
			Title:    "Increase CPU Failover Capacity",
			Priority: "medium",
			Action:   "Add CPU resources or reduce container CPU requests so the failover node has at least 100% capacity.",
			Reason:   "The failover node lacks sufficient CPU capacity to run all HA-protected containers simultaneously.",
		})
	}

	// 11. Insufficient memory failover capacity
	if s.HAEnabled && s.MemFailoverCapacityGB < 16 {
		recs = append(recs, Recommendation{
			ID:       "increase-mem-redundancy",
			Title:    "Increase Memory Failover Capacity",
			Priority: "medium",
			Action:   "Add memory to the failover node or reduce container memory requests to at least 16 GB headroom.",
			Reason:   "The failover node has less than 16 GB of spare memory, which may be insufficient to run all HA containers.",
		})
	}

	// Sort by priority rank (lower = higher urgency)
	sort.SliceStable(recs, func(i, j int) bool {
		return priorityRank[recs[i].Priority] < priorityRank[recs[j].Priority]
	})

	return recs
}