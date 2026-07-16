// Package cloudcostaudit implements cloud storage cost auditing inspired by
// Synology CloudRep, TrueNAS cloud sync cost tracking, and fnOS cloud mounting.
package cloudcostaudit

import (
	"sort"
	"time"
)

// CloudProvider identifies a cloud storage provider.
type CloudProvider string

const (
	ProviderAWS      CloudProvider = "aws_s3"
	ProviderGCS      CloudProvider = "gcs"
	ProviderAzure   CloudProvider = "azure_blob"
	ProviderBackblaze CloudProvider = "backblaze"
	ProviderR2       CloudProvider = "cloudflare_r2"
	ProviderOther    CloudProvider = "other"
)

// EgressType describes outbound data transfer cost type.
type EgressType string

const (
	EgressInterRegion EgressType = "inter_region"
	EgressInternet     EgressType = "internet"
	EgressSameProvider EgressType = "same_provider"
)

// AccountSignal describes a single cloud storage account's cost signals.
type AccountSignal struct {
	Provider       CloudProvider `json:"provider"`
	AccountID      string        `json:"account_id"`
	MonthlyCostUSD float64      `json:"monthly_cost_usd"`
	StorageGB      float64      `json:"storage_gb"`
	EgressGB       float64      `json:"egress_gb"`
	EgressCostUSD  float64      `json:"egress_cost_usd"`
	APICallCount   int64        `json:"api_call_count"`
	APICostUSD     float64      `json:"api_cost_usd"`
	LastBilledAt   time.Time    `json:"last_billed_at"`
	DormantDays    int          `json:"dormant_days"`
	TierPolicyOK   bool         `json:"tier_policy_ok"`
}

// Signal aggregates all cloud cost signals.
type Signal struct {
	Accounts           []AccountSignal
	TotalMonthlyCost   float64
	TotalStorageGB     float64
	TotalEgressGB      float64
	BudgetMonthlyUSD   float64
	HasBudgetAlerts    bool
	HasUnusedAccounts  bool
	ReplicationEnabled bool
	LastAuditAge       time.Duration
}

// Recommendation is an actionable cost optimization suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
	Savings  string `json:"savings_estimate,omitempty"`
}

// Analyze evaluates cloud cost signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if s.TotalMonthlyCost > s.BudgetMonthlyUSD && s.BudgetMonthlyUSD > 0 {
		over := s.TotalMonthlyCost - s.BudgetMonthlyUSD
		recs = append(recs, Recommendation{
			ID:       "cloud-budget-overrun",
			Title:    "Cloud storage budget exceeded",
			Priority: "high",
			Action:   "Review and reduce cloud storage usage; enable lifecycle policies to move cold data to cheaper tiers",
			Reason:   "Monthly cloud spend exceeds budget by amount",
			Savings:  formatSavings(over),
		})
	}

	for _, acc := range s.Accounts {
		if acc.DormantDays > 90 {
			recs = append(recs, Recommendation{
				ID:       "cloud-dormant-" + acc.AccountID,
				Title:    "Dormant cloud account detected",
				Priority: "medium",
				Action:   "Archive or delete data from dormant account to reduce storage costs",
				Reason:   "Account has been inactive for over 90 days",
				Savings:  formatSavings(acc.MonthlyCostUSD),
			})
		}
	}

	if s.TotalEgressGB > 100 && s.TotalStorageGB > 0 {
		egressRatio := s.TotalEgressGB / s.TotalStorageGB
		if egressRatio > 0.1 {
			recs = append(recs, Recommendation{
				ID:       "cloud-egress-high",
				Title:    "High egress traffic relative to storage",
				Priority: "medium",
				Action:   "Consider same-provider replication or CDN caching to reduce inter-region egress fees",
				Reason:   "Egress exceeds 10% of stored data, indicating excessive cross-region transfer",
				Savings:  formatSavings(s.TotalEgressGB * 0.09),
			})
		}
	}

	for _, acc := range s.Accounts {
		if !acc.TierPolicyOK && acc.StorageGB > 100 {
			recs = append(recs, Recommendation{
				ID:       "cloud-tier-" + acc.AccountID,
				Title:    "Missing lifecycle tiering policy",
				Priority: "medium",
				Action:   "Configure lifecycle rules to transition old data to Infrequent Access or Archive tier",
				Reason:   "Storage over 100GB without tiering policy pays premium for cold data",
				Savings:  formatSavings(acc.MonthlyCostUSD * 0.4),
			})
		}
	}

	hasR2 := false
	for _, acc := range s.Accounts {
		if acc.Provider == ProviderR2 {
			hasR2 = true
		}
	}
	if !hasR2 && s.TotalEgressGB > 500 {
		recs = append(recs, Recommendation{
			ID:       "cloud-r2-egress",
			Title:    "Consider Cloudflare R2 for zero egress fees",
			Priority: "low",
			Action:   "Migrate frequently accessed data to Cloudflare R2 to eliminate egress charges",
			Reason:   "High egress volume with providers charging egress fees",
			Savings:  formatSavings(s.TotalEgressGB * 0.09),
		})
	}

	if s.LastAuditAge > 30*24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "cloud-audit-stale",
			Title:    "Cloud cost audit overdue",
			Priority: "low",
			Action:   "Run a full cloud cost audit to identify new waste and verify savings",
			Reason:   "Last audit is over 30 days old; cloud pricing and usage patterns change frequently",
		})
	}

	if s.HasUnusedAccounts {
		recs = append(recs, Recommendation{
			ID:       "cloud-unused-accounts",
			Title:    "Unused cloud accounts detected",
			Priority: "medium",
			Action:   "Review and close unused cloud storage accounts to stop recurring charges",
			Reason:   "Unused accounts still incur storage and API charges",
		})
	}

	for _, acc := range s.Accounts {
		if acc.APICostUSD > acc.MonthlyCostUSD*0.3 && acc.APICallCount > 1000000 {
			recs = append(recs, Recommendation{
				ID:       "cloud-api-" + acc.AccountID,
				Title:    "High API call costs",
				Priority: "medium",
				Action:   "Batch API operations or cache list results to reduce API request count",
				Reason:   "API costs exceed 30% of total bill, indicating excessive request volume",
				Savings:  formatSavings(acc.APICostUSD * 0.5),
			})
		}
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

func formatSavings(usd float64) string {
	if usd < 1 {
		return ""
	}
	return formatUSD(usd) + "/mo"
}

func formatUSD(usd float64) string {
	if usd >= 1000 {
		return formatInt(int(usd/1000)) + "K USD"
	}
	return formatInt(int(usd)) + " USD"
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}