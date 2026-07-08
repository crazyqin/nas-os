// Package quotaworkflow manages storage quota workflow inspired by
// Synology Quota Manager and TrueNAS dataset quotas.
package quotaworkflow

import (
	"sort"
	"strings"
)

// QuotaState represents quota enforcement level.
const (
	StateNone      = "none"
	StateSoft      = "soft"
	StateHard      = "hard"
	StateSuspended = "suspended"
)

// Quota represents a storage quota for a share or user.
type Quota struct {
	ID            string  `json:"id"`
	ShareName     string  `json:"share_name"`
	UserName      string  `json:"user_name,omitempty"`
	LimitGB       float64 `json:"limit_gb"`
	UsedGB        float64 `json:"used_gb"`
	State         string  `json:"state"`
	WarningPct     int     `json:"warning_pct"`
	CriticalPct   int     `json:"critical_pct"`
	NotifyUser    bool    `json:"notify_user"`
	AutoEnlargePct int     `json:"auto_enlarge_pct,omitempty"`
	MaxAutoGB     float64 `json:"max_auto_gb,omitempty"`
}

// Signal describes the quota environment.
type Signal struct {
	TotalShares       int
	SharesWithQuota   int
	UsersWithQuota    int
	OverQuotaShares   int
	NearQuotaShares   int
	QuotaList         []Quota
	HasGlobalPolicy   bool
	DefaultQuotaGB    float64
	PoolFreeGB        float64
	PoolTotalGB       float64
}

// Recommendation is an actionable quota management suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// Analyze evaluates quota signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if !s.HasGlobalPolicy && s.TotalShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "quota-global-policy",
			Title:    "Set global quota policy",
			Priority: "medium",
			Action:   "Define default quota for all shares to prevent runaway usage",
			Reason:   "No global quota policy; shares can grow unbounded",
		})
	}

	if s.OverQuotaShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "quota-over-limit",
			Title:    "Shares over quota",
			Priority: "high",
			Action:   "Review and enforce hard limits on shares exceeding quota",
			Reason:   "Shares over quota may cause write failures",
		})
	}

	if s.NearQuotaShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "quota-near-limit",
			Title:    "Shares approaching quota limit",
			Priority: "medium",
			Action:   "Notify users or increase quota for shares near limit",
			Reason:   "Shares near quota will hit limit soon",
		})
	}

	if s.TotalShares > 0 && s.SharesWithQuota < s.TotalShares {
		unprotected := s.TotalShares - s.SharesWithQuota
		recs = append(recs, Recommendation{
			ID:       "quota-unprotected",
			Title:    "Shares without quota",
			Priority: "medium",
			Action:   "Apply default quota to remaining shares",
			Reason:   "Some shares have no quota protection",
		})
		_ = unprotected
	}

	// Check individual quotas
	for _, q := range s.QuotaList {
		if q.LimitGB > 0 {
			usagePct := int((q.UsedGB / q.LimitGB) * 100)
			if usagePct >= q.CriticalPct {
				recs = append(recs, Recommendation{
					ID:       "quota-critical-" + q.ShareName,
					Title:    q.ShareName + " at critical quota level",
					Priority: "high",
					Action:   "Increase quota or clean up data immediately",
					Reason:   "Share usage at critical threshold",
				})
			} else if usagePct >= q.WarningPct {
				recs = append(recs, Recommendation{
					ID:       "quota-warn-" + q.ShareName,
					Title:    q.ShareName + " approaching quota",
					Priority: "medium",
					Action:   "Notify users or plan for quota increase",
					Reason:   "Share usage exceeds warning threshold",
				})
			}
		}
	}

	if s.PoolFreeGB < s.PoolTotalGB*0.1 {
		recs = append(recs, Recommendation{
			ID:       "quota-pool-low",
			Title:    "Pool free space critically low",
			Priority: "high",
			Action:   "Reduce quotas or expand pool; free space under 10%",
			Reason:   "Pool free space below 10%; quota enforcement may not help",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}

// SuggestQuota generates a recommended quota for a share.
func SuggestQuota(shareName string, defaultGB float64, poolFreeGB float64) Quota {
	limit := defaultGB
	if limit > poolFreeGB*0.8 {
		limit = poolFreeGB * 0.8
	}
	return Quota{
		ID:          "auto-" + shareName,
		ShareName:   shareName,
		LimitGB:     limit,
		UsedGB:      0,
		State:       StateSoft,
		WarningPct:  80,
		CriticalPct: 95,
		NotifyUser:  true,
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
