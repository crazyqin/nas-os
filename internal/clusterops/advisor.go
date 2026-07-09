// Package clusterops implements multi-NAS cluster orchestration inspired by
// Synology Central Management System, TrueNAS Cluster, and fnOS multi-device.
package clusterops

import (
	"sort"
	"time"
)

// NodeRole indicates the role of a NAS node in the cluster.
type NodeRole string

const (
	RolePrimary   NodeRole = "primary"
	RoleSecondary NodeRole = "secondary"
	RoleWitness   NodeRole = "witness"
	RoleStandby   NodeRole = "standby"
)

// NodeStatus indicates the health status of a cluster node.
type NodeStatus string

const (
	StatusOnline    NodeStatus = "online"
	StatusOffline   NodeStatus = "offline"
	StatusDegraded  NodeStatus = "degraded"
	StatusSyncing   NodeStatus = "syncing"
	StatusFailing   NodeStatus = "failing"
)

// Node describes a single NAS node in the cluster.
type Node struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Role          NodeRole   `json:"role"`
	Status        NodeStatus `json:"status"`
	IP            string     `json:"ip"`
	CPUUsagePct   float64    `json:"cpu_usage_pct"`
	MemUsagePct   float64    `json:"mem_usage_pct"`
	DiskUsagePct  float64    `json:"disk_usage_pct"`
	StorageGB     int        `json:"storage_gb"`
	UsedStorageGB int        `json:"used_storage_gb"`
	LastHeartbeat time.Time  `json:"last_heartbeat"`
	ReplicationLag time.Duration `json:"replication_lag"`
}

// Signal describes the cluster orchestration state.
type Signal struct {
	Nodes              []Node
	TotalNodes         int
	OnlineNodes        int
	HasHA              bool
	HAFailoverEnabled  bool
	ReplicationEnabled bool
	LastFailoverAge    time.Duration
	HasPrimaryOffline  bool
	HasSplitBrain      bool
	ClusterHealthScore int
	WitnessNodeOnline  bool
}

// Recommendation is an actionable cluster orchestration suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// Analyze evaluates cluster orchestration signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if s.TotalNodes > 1 && !s.HasHA {
		recs = append(recs, Recommendation{
			ID:       "cluster-enable-ha",
			Title:    "Enable high availability",
			Priority: "high",
			Action:   "Configure HA pair with active/passive failover for the primary and secondary nodes",
			Reason:   "Multi-node cluster without HA means single node failure causes full outage",
		})
	}

	if s.HasPrimaryOffline && s.HAFailoverEnabled {
		recs = append(recs, Recommendation{
			ID:       "cluster-primary-failover",
			Title:    "Primary node offline - verify failover",
			Priority: "high",
			Action:   "Confirm secondary has taken over primary role; check service status on new primary",
			Reason:   "Primary node is offline; HA failover should be in progress or completed",
		})
	}

	if s.HasPrimaryOffline && !s.HAFailoverEnabled {
		recs = append(recs, Recommendation{
			ID:       "cluster-primary-down-no-failover",
			Title:    "Primary node offline without failover",
			Priority: "high",
			Action:   "Immediately bring secondary online as primary or restart primary node",
			Reason:   "Primary is offline and HA failover is not enabled; cluster has no active primary",
		})
	}

	if s.HasSplitBrain {
		recs = append(recs, Recommendation{
			ID:       "cluster-split-brain",
			Title:    "Split-brain detected",
			Priority: "high",
			Action:   "Isolate the node with fewer connections and resync from authoritative primary",
			Reason:   "Split-brain means two nodes think they are primary; data corruption risk is high",
		})
	}

	if s.TotalNodes >= 2 && !s.WitnessNodeOnline && s.HasHA {
		recs = append(recs, Recommendation{
			ID:       "cluster-witness-down",
			Title:    "Witness node is offline",
			Priority: "medium",
			Action:   "Restore witness node to prevent failover decision failures",
			Reason:   "Witness node is required for HA quorum decisions; without it failover may fail",
		})
	}

	for _, node := range s.Nodes {
		if node.Status == StatusDegraded && node.DiskUsagePct > 90 {
			recs = append(recs, Recommendation{
				ID:       "cluster-disk-full-" + node.ID,
				Title:    "Node disk nearly full: " + node.Name,
				Priority: "high",
				Action:   "Redistribute data from this node or add storage capacity to prevent service disruption",
				Reason:   "Node disk usage above 90% risks write failures and service degradation",
			})
		}
	}

	for _, node := range s.Nodes {
		if node.Status == StatusOnline && node.ReplicationLag > 1*time.Hour {
			recs = append(recs, Recommendation{
				ID:       "cluster-repl-lag-" + node.ID,
				Title:    "High replication lag on node: " + node.Name,
				Priority: "medium",
				Action:   "Check network bandwidth between nodes and verify replication schedule",
				Reason:   "Replication lag over 1 hour means data loss window in failover is significant",
			})
		}
	}

	for _, node := range s.Nodes {
		if node.Status == StatusOnline && node.CPUUsagePct > 90 {
			recs = append(recs, Recommendation{
				ID:       "cluster-cpu-high-" + node.ID,
				Title:    "High CPU usage on node: " + node.Name,
				Priority: "medium",
				Action:   "Migrate non-critical workloads to other nodes or upgrade CPU capacity",
				Reason:   "CPU usage above 90% can cause service latency and missed heartbeats",
			})
		}
	}

	if s.ReplicationEnabled && s.LastFailoverAge > 0 && s.LastFailoverAge < 24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "cluster-post-failover-check",
			Title:    "Post-failover verification needed",
			Priority: "high",
			Action:   "Run full data integrity check and verify all services are healthy after recent failover",
			Reason:   "A failover occurred within the last 24 hours; data consistency should be verified",
		})
	}

	if s.OnlineNodes == 1 && s.TotalNodes > 1 {
		recs = append(recs, Recommendation{
			ID:       "cluster-single-node",
			Title:    "Running in single-node mode",
			Priority: "medium",
			Action:   "Investigate offline nodes and restore cluster quorum",
			Reason:   "Only one node is online out of the cluster; HA protection is effectively disabled",
		})
	}

	if s.ClusterHealthScore > 0 && s.ClusterHealthScore < 60 {
		recs = append(recs, Recommendation{
			ID:       "cluster-health-low",
			Title:    "Cluster health score is low",
			Priority: "medium",
			Action:   "Run diagnostics on all nodes; check for disk errors, network issues, and service status",
			Reason:   "Cluster health below 60 indicates multiple nodes may have issues",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}

func priorityRank(p string) int {
	switch p {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}