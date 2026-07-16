package datadomainaudit

import "sort"

// Signal represents the data-domain isolation audit signal extracted from
// GDPR data-localization checks and TrueNAS encrypted-dataset exclusion indexes.
type Signal struct {
	PIIPresent                  bool
	PIIEncrypted                bool
	CrossBorderReplication      bool
	DataResidencyPolicyExists   bool
	AccessLogComplete           bool
	RetentionPolicyExists       bool
	DataProcessingInventoryExists bool
	DPAAgreementExists           bool
	EncryptionAtRest             bool
	EncryptionInTransit          bool
	DataLocalizationRequired     bool
	Jurisdiction                 string
	ThirdPartyDataSharing        bool
	DSARCompliant                bool
	BreachNotificationReady      bool
}

// Recommendation is a single audit finding with actionable guidance.
type Recommendation struct {
	ID       string
	Title    string
	Priority string
	Action   string
	Reason   string
}

// priorityRank maps priority labels to a numeric rank for deterministic sorting.
// Lower number = higher priority.
func priorityRank(p string) int {
	switch p {
	case "Critical":
		return 0
	case "High":
		return 1
	case "Medium":
		return 2
	case "Low":
		return 3
	default:
		return 4
	}
}

// Analyze inspects the provided Signal and returns a sorted list of
// Recommendations addressing every violation or gap detected.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if s.PIIPresent && !s.PIIEncrypted {
		recs = append(recs, Recommendation{
			ID:       "REC-PII-ENCRYPT",
			Title:    "Encrypt PII Data at Rest",
			Priority: "Critical",
			Action:   "Enable encryption for all datasets containing personally identifiable information (PII).",
			Reason:   "Unencrypted PII at rest violates GDPR Article 32 and risks exposure during physical disk compromise.",
		})
	}

	if s.CrossBorderReplication && s.DataLocalizationRequired {
		recs = append(recs, Recommendation{
			ID:       "REC-CROSS-BORDER",
			Title:    "Halt Cross-Border Replication",
			Priority: "Critical",
			Action:   "Immediately suspend all replication tasks that transfer data to jurisdictions outside the approved data-residency boundary.",
			Reason:   "Cross-border replication without data-localization compliance breaches GDPR Article 44–49 transfer restrictions.",
		})
	}

	if !s.DataResidencyPolicyExists {
		recs = append(recs, Recommendation{
			ID:       "REC-RESIDENCY-POLICY",
			Title:    "Establish Data Residency Policy",
			Priority: "High",
			Action:   "Draft and enforce a formal data-residency policy specifying approved storage jurisdictions and transfer rules.",
			Reason:   "Absence of a documented data-residency policy leaves the organization unable to demonstrate GDPR compliance.",
		})
	}

	if !s.AccessLogComplete {
		recs = append(recs, Recommendation{
			ID:       "REC-ACCESS-LOG",
			Title:    "Complete Access Logging",
			Priority: "High",
			Action:   "Ensure all file-system and API access events are logged with user identity, timestamp, and data-set scope.",
			Reason:   "Incomplete access logs prevent GDPR Article 30 record-keeping and hinder breach investigation.",
		})
	}

	if !s.RetentionPolicyExists {
		recs = append(recs, Recommendation{
			ID:       "REC-RETENTION-POLICY",
			Title:    "Define Data Retention Policy",
			Priority: "High",
			Action:   "Create retention schedules for each data category specifying retention period and secure deletion procedure.",
			Reason:   "Without a retention policy the organization cannot satisfy GDPR Article 5(1)(e) storage-limitation requirements.",
		})
	}

	if !s.DataProcessingInventoryExists {
		recs = append(recs, Recommendation{
			ID:       "REC-PROCESSING-INVENTORY",
			Title:    "Build Data Processing Inventory",
			Priority: "High",
			Action:   "Maintain a comprehensive inventory of all data-processing activities including purposes, categories, and recipients.",
			Reason:   "A complete Article 30 processing inventory is mandatory for demonstrating accountability.",
		})
	}

	if !s.DPAAgreementExists && s.ThirdPartyDataSharing {
		recs = append(recs, Recommendation{
			ID:       "REC-DPA-AGREEMENT",
			Title:    "Sign Data Processing Agreement",
			Priority: "High",
			Action:   "Execute a GDPR-compliant Data Processing Agreement (DPA) with every third-party processor before sharing data.",
			Reason:   "Sharing data with third-party processors without a DPA violates GDPR Article 28.",
		})
	}

	if !s.EncryptionAtRest {
		recs = append(recs, Recommendation{
			ID:       "REC-ENCRYPT-REST",
			Title:    "Enable Encryption at Rest",
			Priority: "Medium",
			Action:   "Enable dataset-level or pool-level encryption for all stored data, not just PII.",
			Reason:   "Encryption at rest is a baseline security measure under GDPR Article 32(1)(a) for all personal data.",
		})
	}

	if !s.EncryptionInTransit {
		recs = append(recs, Recommendation{
			ID:       "REC-ENCRYPT-TRANSIT",
			Title:    "Enable Encryption in Transit",
			Priority: "Medium",
			Action:   "Configure TLS for all replication, sharing, and management protocols (SMB, NFS, S3, API).",
			Reason:   "Unencrypted transit exposes personal data to interception, contravening GDPR Article 32(1)(a).",
		})
	}

	if !s.DSARCompliant {
		recs = append(recs, Recommendation{
			ID:       "REC-DSAR",
			Title:    "Implement DSAR Response Process",
			Priority: "Medium",
			Action:   "Establish a documented workflow for handling Data Subject Access Requests within the GDPR one-month deadline.",
			Reason:   "Without a DSAR process the organization cannot meet GDPR Article 12–15 response obligations.",
		})
	}

	if !s.BreachNotificationReady {
		recs = append(recs, Recommendation{
			ID:       "REC-BREACH-NOTIFY",
			Title:    "Establish Breach Notification Mechanism",
			Priority: "Low",
			Action:   "Create a breach-detection and notification runbook capable of reporting to authorities within 72 hours.",
			Reason:   "Readiness for breach notification is required by GDPR Article 33 and reduces regulatory penalty risk.",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		ri := priorityRank(recs[i].Priority)
		rj := priorityRank(recs[j].Priority)
		if ri != rj {
			return ri < rj
		}
		return recs[i].ID < recs[j].ID
	})

	return recs
}