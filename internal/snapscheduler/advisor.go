// Package snapscheduler implements snapshot scheduling policies inspired by
// TrueNAS periodic snapshots, Synology Snapshot Manager schedules, and fnOS
// timeline-based file protection.
package snapscheduler

import (
	"sort"
	"strings"
	"time"
)

// Policy defines a snapshot scheduling policy.
type Policy struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	ShareOrVolume string        `json:"share_or_volume"`
	Frequency     string        `json:"frequency"`
	Retention     int           `json:"retention"`
	Enabled       bool          `json:"enabled"`
	QuiesceFS     bool          `json:"quiesce_fs"`
	Replicate     bool          `json:"replicate"`
	Immutable     bool          `json:"immutable"`
	NextRun       time.Time     `json:"next_run,omitempty"`
	LastRun       time.Time     `json:"last_run,omitempty"`
	Tags          []string      `json:"tags,omitempty"`
}

// Signal describes the current snapshot scheduling state.
type Signal struct {
	ShareName          string
	ExistingPolicies   []Policy
	HasHourlySnapshot  bool
	HasDailySnapshot   bool
	HasWeeklySnapshot  bool
	HasImmutableSnap   bool
	MaxRetentionSeen    int
	SharesWithoutPolicy int
	TotalShares         int
	ReplicationEnabled  bool
	LastSnapshotAge     time.Duration
	BtrfsSubvolumeSnap  bool
	ZFSDatasetSnap      bool
}

// Recommendation is an actionable snapshot scheduling suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// Analyze evaluates snapshot scheduling signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if s.TotalShares > 0 && s.SharesWithoutPolicy > 0 {
		recs = append(recs, Recommendation{
			ID:       "snap-add-policy",
			Title:    "Add snapshot policy to unprotected shares",
			Priority: "high",
			Action:   "Add daily snapshot policy to remaining shares",
			Reason:   "Shares have no snapshot policy; recovery window is zero",
		})
	}

	if !s.HasHourlySnapshot && s.TotalShares > 3 {
		recs = append(recs, Recommendation{
			ID:       "snap-add-hourly",
			Title:    "Add hourly snapshots",
			Priority: "medium",
			Action:   "Enable hourly snapshots on critical shares, keep 24 copies",
			Reason:   "Hourly snapshots reduce RPO when share count is high",
		})
	}

	if !s.HasImmutableSnap {
		recs = append(recs, Recommendation{
			ID:       "snap-immutable",
			Title:    "Enable immutable snapshots",
			Priority: "high",
			Action:   "Enable immutable snapshot protection on critical shares",
			Reason:   "Immutable snapshots are key defense against ransomware",
		})
	}

	if s.LastSnapshotAge > 72*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "snap-stale",
			Title:    "Snapshots are stale",
			Priority: "high",
			Action:   "Run a manual snapshot immediately and check scheduled tasks",
			Reason:   "Last snapshot over 72h ago; scheduled task may be failing",
		})
	}

	if s.HasDailySnapshot && !s.HasWeeklySnapshot {
		recs = append(recs, Recommendation{
			ID:       "snap-add-weekly",
			Title:    "Add weekly snapshots",
			Priority: "low",
			Action:   "Add weekly snapshot policy on all shares, keep 8 copies",
			Reason:   "Weekly snapshots provide longer rollback window",
		})
	}

	if !s.ReplicationEnabled && s.HasDailySnapshot {
		recs = append(recs, Recommendation{
			ID:       "snap-replicate",
			Title:    "Enable snapshot replication",
			Priority: "medium",
			Action:   "Replicate daily snapshots to remote NAS or cloud",
			Reason:   "Remote replication provides disaster recovery",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}

// SuggestPolicy generates a recommended policy for a share.
func SuggestPolicy(shareName string, critical bool) Policy {
	freq := "daily"
	retention := 30
	if critical {
		freq = "hourly"
		retention = 168
	}
	return Policy{
		ID:            "auto-" + shareName,
		Name:          shareName + "-auto-snap",
		ShareOrVolume: shareName,
		Frequency:     freq,
		Retention:     retention,
		Enabled:       true,
		QuiesceFS:     true,
		Immutable:     critical,
	}
}

func priorityRank(p string) int {
	switch strings.ToLower(p) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
