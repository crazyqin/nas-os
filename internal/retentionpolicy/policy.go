// Package retentionpolicy implements data retention policy management inspired by
// Synology WORM (Write Once Read Many) compliance and TrueNAS immutable snapshots.
package retentionpolicy

import (
	"sort"
	"strings"
	"time"
)

// RetentionClass defines how long data is kept.
type RetentionClass struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Duration     time.Duration `json:"duration"`
	AfterExpiry  string        `json:"after_expiry"`  // delete, archive, notify, review
	Immutable    bool          `json:"immutable"`
	LegalHold    bool          `json:"legal_hold"`
	ShareScope   []string      `json:"share_scope,omitempty"`
	Category     string        `json:"category"`      // financial, hr, legal, medical, general
}

// Signal describes the retention policy environment.
type Signal struct {
	TotalShares        int
	SharesWithPolicy   int
	Policies           []RetentionClass
	LegalHoldShares    int
	ImmutableShares    int
	ExpiredDataGB      float64
	PendingReviewGB    float64
	ComplianceAuditDue bool
	LastAuditDate      time.Time
	WORMEnabled        bool
}

// Recommendation is an actionable retention policy suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// Analyze evaluates retention signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if !s.WORMEnabled {
		recs = append(recs, Recommendation{
			ID:       "retention-worm-enable",
			Title:    "Enable WORM storage",
			Priority: "medium",
			Action:   "Enable WORM on compliance-related shares for data immutability",
			Reason:   "WORM protection prevents tampering with regulated data",
		})
	}

	if s.TotalShares > 0 && s.SharesWithPolicy < s.TotalShares {
		recs = append(recs, Recommendation{
			ID:       "retention-missing-policy",
			Title:    "Shares without retention policy",
			Priority: "high",
			Action:   "Define retention policies for all shares with regulated data",
			Reason:   "Shares without retention policies risk compliance violations",
		})
	}

	if s.ExpiredDataGB > 0 {
		recs = append(recs, Recommendation{
			ID:       "retention-expired",
			Title:    "Process expired data",
			Priority: "medium",
			Action:   "Delete or archive data past retention period to free space",
			Reason:   "Expired data consumes storage and may violate retention rules",
		})
	}

	if s.PendingReviewGB > 0 {
		recs = append(recs, Recommendation{
			ID:       "retention-review",
			Title:    "Review pending data for retention",
			Priority: "medium",
			Action:   "Review data flagged for manual retention decision",
			Reason:   "Data awaiting review may need policy adjustment",
		})
	}

	if s.ComplianceAuditDue {
		daysSince := time.Since(s.LastAuditDate).Hours() / 24
		if daysSince > 90 {
			recs = append(recs, Recommendation{
				ID:       "retention-audit",
				Title:    "Compliance audit overdue",
				Priority: "high",
				Action:   "Schedule and execute retention compliance audit",
				Reason:   "Audit overdue by over 90 days; compliance risk",
			})
		}
	}

	if s.LegalHoldShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "retention-legal-hold",
			Title:    "Legal hold in effect",
			Priority: "high",
			Action:   "Verify legal hold shares are not modified or deleted",
			Reason:   "Legal hold requires strict immutability",
		})
	}

	for _, p := range s.Policies {
		if p.Category == "financial" && !p.Immutable {
			recs = append(recs, Recommendation{
				ID:       "retention-financial-immutable",
				Title:    "Financial data should be immutable",
				Priority: "high",
				Action:   "Enable WORM on financial retention class",
				Reason:   "Financial records require immutability for compliance",
			})
			break
		}
	}

	if s.LegalHoldShares > 0 && !s.WORMEnabled {
		recs = append(recs, Recommendation{
			ID:       "retention-hold-no-worm",
			Title:    "Legal hold without WORM",
			Priority: "critical",
			Action:   "Enable WORM immediately on shares with legal hold",
			Reason:   "Legal hold without WORM is not enforceable",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}

// SuggestPolicy generates a recommended retention policy for a category.
func SuggestPolicy(category string) RetentionClass {
	switch strings.ToLower(category) {
	case "financial":
		return RetentionClass{
			ID:          "ret-financial",
			Name:        "Financial Records (7 years)",
			Duration:    7 * 365 * 24 * time.Hour,
			AfterExpiry: "archive",
			Immutable:   true,
			Category:    "financial",
		}
	case "hr":
		return RetentionClass{
			ID:          "ret-hr",
			Name:        "HR Records (7 years)",
			Duration:    7 * 365 * 24 * time.Hour,
			AfterExpiry: "delete",
			Immutable:   true,
			Category:    "hr",
		}
	case "legal":
		return RetentionClass{
			ID:          "ret-legal",
			Name:        "Legal Documents (10 years)",
			Duration:    10 * 365 * 24 * time.Hour,
			AfterExpiry: "review",
			Immutable:   true,
			Category:    "legal",
		}
	case "medical":
		return RetentionClass{
			ID:          "ret-medical",
			Name:        "Medical Records (10 years)",
			Duration:    10 * 365 * 24 * time.Hour,
			AfterExpiry: "archive",
			Immutable:   true,
			Category:    "medical",
		}
	default:
		return RetentionClass{
			ID:          "ret-general",
			Name:        "General Data (1 year)",
			Duration:    365 * 24 * time.Hour,
			AfterExpiry: "delete",
			Immutable:   false,
			Category:    "general",
		}
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
