// Package datasovereigntyaudit implements data sovereignty auditing inspired by
// GDPR, CCPA, PIPL, and Synology/TrueNAS data governance capabilities.
package datasovereigntyaudit

import (
	"sort"
	"time"
)

// Regulation indicates which data protection regulation applies.
type Regulation string

const (
	RegGDPR  Regulation = "gdpr"
	RegCCPA  Regulation = "ccpa"
	RegPIPL  Regulation = "pipl"
	RegHIPAA Regulation = "hipaa"
	RegSOC2  Regulation = "soc2"
	RegISO   Regulation = "iso27001"
)

// DataRegion indicates where data is physically stored.
type DataRegion string

const (
	RegionEU        DataRegion = "eu"
	RegionUS        DataRegion = "us"
	RegionChina     DataRegion = "china"
	RegionAPAC      DataRegion = "apac"
	RegionOnPrem    DataRegion = "on_prem"
	RegionUnknown   DataRegion = "unknown"
)

// DataClass indicates the sensitivity classification of data.
type DataClass string

const (
	ClassPublic      DataClass = "public"
	ClassInternal    DataClass = "internal"
	ClassConfidential DataClass = "confidential"
	ClassRestricted  DataClass = "restricted"
)

// ShareSignal describes a single share's sovereignty posture.
type ShareSignal struct {
	Name            string     `json:"name"`
	DataClass       DataClass  `json:"data_class"`
	Region          DataRegion `json:"region"`
	HasPII          bool       `json:"has_pii"`
	HasCrossBorderRep bool     `json:"has_cross_border_rep"`
	ReplicationRegions []DataRegion `json:"replication_regions,omitempty"`
	HasEncryption   bool       `json:"has_encryption"`
	HasAccessLog    bool       `json:"has_access_log"`
	HasRetentionPolicy bool     `json:"has_retention_policy"`
	LastAuditAt     time.Time  `json:"last_audit_at"`
}

// Signal describes the overall data sovereignty state.
type Signal struct {
	Shares              []ShareSignal
	TotalShares         int
	PIIShares           int
	UnencryptedPII      int
	CrossBorderRepCount int
	NoAccessLogShares    int
	NoRetentionShares    int
	StaleAuditShares     int
	ActiveRegulations    []Regulation
	HasDPAProcessors     bool
	HasDataInventory     bool
}

// Recommendation is an actionable sovereignty compliance suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// Analyze evaluates data sovereignty signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if s.PIIShares > 0 && s.UnencryptedPII > 0 {
		recs = append(recs, Recommendation{
			ID:       "sovereign-encrypt-pii",
			Title:    "Encrypt all PII shares",
			Priority: "high",
			Action:   "Enable at-rest encryption on all shares containing personally identifiable information",
			Reason:   "PII data without encryption violates GDPR Article 32 and PIPL Article 51",
		})
	}

	if s.CrossBorderRepCount > 0 {
		hasGDPR := false
		hasPIPL := false
		for _, reg := range s.ActiveRegulations {
			if reg == RegGDPR {
				hasGDPR = true
			}
			if reg == RegPIPL {
				hasPIPL = true
			}
		}
		if hasGDPR || hasPIPL {
			recs = append(recs, Recommendation{
				ID:       "sovereign-cross-border",
				Title:    "Cross-border data replication requires review",
				Priority: "high",
				Action:   "Ensure cross-border transfers have signed SCCs (GDPR) or security assessment filings (PIPL)",
				Reason:   "Cross-border replication of regulated data requires legal safeguards",
			})
		}
	}

	if s.NoAccessLogShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "sovereign-access-logs",
			Title:    "Enable access logging on all shares",
			Priority: "high",
			Action:   "Turn on access audit logging for shares without it; logs must be retained per regulation",
			Reason:   "Access logging is required for GDPR Articles 30-32, PIPL Article 55, and SOC 2 audit trails",
		})
	}

	if s.NoRetentionShares > 0 && s.PIIShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "sovereign-retention-pii",
			Title:    "Missing retention policies for PII",
			Priority: "high",
			Action:   "Define data retention and deletion policies for all shares containing PII",
			Reason:   "PII without retention policy violates GDPR right to erasure and PIPL data minimization",
		})
	}

	if !s.HasDataInventory {
		recs = append(recs, Recommendation{
			ID:       "sovereign-data-inventory",
			Title:    "Create data processing inventory",
			Priority: "medium",
			Action:   "Build and maintain a record of processing activities (ROPA) covering all shares",
			Reason:   "GDPR Article 30 requires a record of processing activities for organizations",
		})
	}

	if !s.HasDPAProcessors {
		recs = append(recs, Recommendation{
			ID:       "sovereign-dpa-processors",
			Title:    "Missing data processing agreements",
			Priority: "medium",
			Action:   "Ensure DPAs are signed with all cloud providers and third-party processors",
			Reason:   "GDPR Article 28 requires written agreements with all data processors",
		})
	}

	if s.StaleAuditShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "sovereign-stale-audit",
			Title:    "Data audit overdue on shares",
			Priority: "medium",
			Action:   "Run data classification and access review on shares with stale audits",
			Reason:   "Regular audits are required to maintain compliance; stale audits risk unidentified violations",
		})
	}

	for _, share := range s.Shares {
		if share.HasPII && share.Region == RegionUnknown {
			recs = append(recs, Recommendation{
				ID:       "sovereign-region-unknown-" + share.Name,
				Title:    "Unknown data region for PII share: " + share.Name,
				Priority: "high",
				Action:   "Classify the storage region for this share to ensure regulatory compliance",
				Reason:   "PII stored in unknown regions may violate data residency requirements",
			})
		}
	}

	for _, share := range s.Shares {
		if share.DataClass == ClassRestricted && !share.HasEncryption {
			recs = append(recs, Recommendation{
				ID:       "sovereign-restricted-encrypt-" + share.Name,
				Title:    "Restricted data share unencrypted: " + share.Name,
				Priority: "high",
				Action:   "Immediately enable encryption on restricted-class share",
				Reason:   "Restricted data without encryption is a critical compliance failure",
			})
		}
	}

	hasHIPAA := false
	for _, reg := range s.ActiveRegulations {
		if reg == RegHIPAA {
			hasHIPAA = true
		}
	}
	if hasHIPAA && !s.HasDataInventory {
		recs = append(recs, Recommendation{
			ID:       "sovereign-hipaa-inventory",
			Title:    "HIPAA requires PHI inventory",
			Priority: "high",
			Action:   "Create an inventory of all Protected Health Information (PHI) locations and access patterns",
			Reason:   "HIPAA requires complete documentation of PHI storage, access, and transmission",
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