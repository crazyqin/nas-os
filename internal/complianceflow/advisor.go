// Package complianceflow implements multi-standard compliance audit workflow
// orchestration inspired by GDPR, PIPL, HIPAA, SOC2, ISO 27001 requirements.
package complianceflow

import (
	"fmt"
	"sort"
	"time"
)

// ComplianceStandard indicates the regulatory standard being audited.
type ComplianceStandard string

const (
	StandardGDPR      ComplianceStandard = "gdpr"        // EU General Data Protection Regulation
	StandardPIPL      ComplianceStandard = "pipl"        // China Personal Information Protection Law
	StandardHIPAA     ComplianceStandard = "hipaa"       // US Health Insurance Portability
	StandardSOC2      ComplianceStandard = "soc2"        // Service Organization Control 2
	StandardISO27001  ComplianceStandard = "iso27001"    // ISO/IEC 27001
	StandardCCPA      ComplianceStandard = "ccpa"        // California Consumer Privacy Act
	StandardPCI       ComplianceStandard = "pci_dss"     // Payment Card Industry
)

// AuditPhase indicates the phase of the compliance workflow.
type AuditPhase string

const (
	PhaseDiscovery   AuditPhase = "discovery"    // identify data and scope
	PhaseGapAnalysis AuditPhase = "gap_analysis" // compare against requirements
	PhaseRemediation AuditPhase = "remediation"  // fix gaps
	PhaseEvidence     AuditPhase = "evidence"     // collect audit evidence
	PhaseReporting    AuditPhase = "reporting"    // generate compliance report
	PhaseReview       AuditPhase = "review"       // review and sign-off
)

// ControlStatus indicates the status of a compliance control.
type ControlStatus string

const (
	ControlPassed    ControlStatus = "passed"
	ControlFailed    ControlStatus = "failed"
	ControlWarning   ControlStatus = "warning"
	ControlNotApplicable ControlStatus = "not_applicable"
	ControlPending    ControlStatus = "pending"
)

// Control describes a single compliance control check.
type Control struct {
	ID          string             `json:"id"`
	Standard    ComplianceStandard `json:"standard"`
	Category    string             `json:"category"`
	Title       string             `json:"title"`
	Status      ControlStatus      `json:"status"`
	Evidence    string             `json:"evidence,omitempty"`
	Remediation string             `json:"remediation,omitempty"`
	LastChecked time.Time          `json:"last_checked"`
	Priority    string             `json:"priority"`
}

// Workflow describes a compliance audit workflow state.
type Workflow struct {
	Standard      ComplianceStandard `json:"standard"`
	CurrentPhase  AuditPhase         `json:"current_phase"`
	Controls      []Control          `json:"controls"`
	StartTime     time.Time          `json:"start_time"`
	LastActivity  time.Time          `json:"last_activity"`
	AssignedTo    string             `json:"assigned_to,omitempty"`
}

// Signal aggregates compliance workflow signals for analysis.
type Signal struct {
	Workflows           []Workflow          `json:"workflows"`
	FailedControls      int                 `json:"failed_controls"`
	WarningControls     int                 `json:"warning_controls"`
	OverdueReviews      int                 `json:"overdue_reviews"`
	HasGDPR             bool                `json:"has_gdpr"`
	HasPIPL             bool                `json:"has_pipl"`
	HasHIPAA            bool                `json:"has_hipaa"`
	HasSOC2             bool                `json:"has_soc2"`
	HasISO27001         bool                `json:"has_iso27001"`
	PIIDataDetected     bool                `json:"pii_data_detected"`
	PHIDataDetected     bool                `json:"phi_data_detected"`
	PaymentDataDetected bool                `json:"payment_data_detected"`
	CrossBorderData     bool                `json:"cross_border_data"`
	EncryptionAtRest    bool                `json:"encryption_at_rest"`
	EncryptionInTransit bool                `json:"encryption_in_transit"`
	AuditLoggingEnabled bool                `json:"audit_logging_enabled"`
	TArray             []string            `json:"data_categories_present"`
}

// Recommendation is an actionable compliance workflow suggestion.
type Recommendation struct {
	ID         string              `json:"id"`
	Title      string              `json:"title"`
	Priority   string              `json:"priority"`
	Action     string              `json:"action"`
	Reason     string              `json:"reason"`
	Standard   ComplianceStandard  `json:"standard,omitempty"`
	Phase      AuditPhase          `json:"phase,omitempty"`
	ControlID  string              `json:"control_id,omitempty"`
}

// Analyze evaluates compliance workflow signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	// PII data present but no GDPR/PIPL compliance workflow
	if s.PIIDataDetected {
		if !s.HasGDPR {
			recs = append(recs, Recommendation{
				ID:       "compliance-start-gdpr",
				Title:    "PII data detected but no GDPR compliance workflow",
				Priority: "critical",
				Action:   "Initialize GDPR compliance workflow: discovery → gap analysis → remediation → evidence → reporting",
				Reason:   "PII data requires GDPR compliance; missing workflow creates regulatory liability",
				Standard: StandardGDPR,
				Phase:    PhaseDiscovery,
			})
		}
		if !s.HasPIPL && s.CrossBorderData {
			recs = append(recs, Recommendation{
				ID:       "compliance-start-pipl",
				Title:    "Cross-border PII data requires PIPL compliance",
				Priority: "critical",
				Action:   "Initialize PIPL compliance workflow for cross-border data transfer assessment",
				Reason:   "China PIPL requires compliance for cross-border PII transfers; missing risk fines up to 50M RMB",
				Standard: StandardPIPL,
				Phase:    PhaseDiscovery,
			})
		}
	}

	// PHI data present but no HIPAA
	if s.PHIDataDetected && !s.HasHIPAA {
		recs = append(recs, Recommendation{
			ID:       "compliance-start-hipaa",
			Title:    "PHI data detected but no HIPAA compliance workflow",
			Priority: "critical",
			Action:   "Initialize HIPAA compliance workflow: encryption, audit logging, access controls, BAAs",
			Reason:   "Health data requires HIPAA compliance; violations carry penalties up to $1.5M per category per year",
			Standard: StandardHIPAA,
			Phase:    PhaseDiscovery,
		})
	}

	// Payment data without PCI-DSS
	if s.PaymentDataDetected {
		foundPCI := false
		for _, w := range s.Workflows {
			if w.Standard == StandardPCI {
				foundPCI = true
				break
			}
		}
		if !foundPCI {
			recs = append(recs, Recommendation{
				ID:       "compliance-start-pci",
				Title:    "Payment data detected without PCI-DSS workflow",
				Priority: "critical",
				Action:   "Initialize PCI-DSS compliance workflow for payment data protection",
				Reason:   "Payment card data requires PCI-DSS; non-compliance risks fines and processor termination",
				Standard: StandardPCI,
				Phase:    PhaseDiscovery,
			})
		}
	}

	// Failed controls need remediation
	if s.FailedControls > 0 {
		recs = append(recs, Recommendation{
			ID:       "compliance-remediate-failed",
			Title:    fmt.Sprintf("%d failed compliance controls need remediation", s.FailedControls),
			Priority: "high",
			Action:   "Move to remediation phase and address all failed controls before generating evidence",
			Reason:   fmt.Sprintf("%d failed controls block compliance certification; remediation is the bottleneck", s.FailedControls),
			Phase:    PhaseRemediation,
		})
	}

	// Warning controls
	if s.WarningControls > 5 {
		recs = append(recs, Recommendation{
			ID:       "compliance-warnings-review",
			Title:    "Many compliance warnings accumulated",
			Priority: "medium",
			Action:   "Review and triode warnings; many may become failures if unaddressed",
			Reason:   fmt.Sprintf("%d warning controls may escalate to failures; preventive review recommended", s.WarningControls),
			Phase:    PhaseReview,
		})
	}

	// Overdue reviews
	if s.OverdueReviews > 0 {
		recs = append(recs, Recommendation{
			ID:       "compliance-overdue-reviews",
			Title:    fmt.Sprintf("%d overdue compliance reviews", s.OverdueReviews),
			Priority: "high",
			Action:   "Schedule and complete overdue compliance reviews immediately",
			Reason:   fmt.Sprintf("%d reviews overdue; stale compliance evidence may invalidate certifications", s.OverdueReviews),
			Phase:    PhaseReview,
		})
	}

	// Encryption gaps
	if !s.EncryptionAtRest {
		recs = append(recs, Recommendation{
			ID:       "compliance-encrypt-at-rest",
			Title:    "Encryption at rest disabled",
			Priority: "critical",
			Action:   "Enable ZFS native encryption or LUKS for all datasets containing PII/PHI/payment data",
			Reason:   "Most compliance standards (GDPR, HIPAA, PCI-DSS, SOC2) require encryption at rest",
		})
	}

	if !s.EncryptionInTransit {
		recs = append(recs, Recommendation{
			ID:       "compliance-encrypt-transit",
			Title:    "Encryption in transit not enforced",
			Priority: "critical",
			Action:   "Enforce TLS 1.3 for all network protocols; disable SSL and TLS 1.0/1.1",
			Reason:   "Compliance standards require encrypted data in transit; unencrypted SMB/NFS/HTTP risk exposure",
		})
	}

	if !s.AuditLoggingEnabled {
		recs = append(recs, Recommendation{
			ID:       "compliance-audit-logging",
			Title:    "Audit logging disabled",
			Priority: "critical",
			Action:   "Enable comprehensive audit logging for file access, authentication, and administrative actions",
			Reason:   "All major compliance standards require audit trails; missing logs make breach investigation impossible",
		})
	}

	// Stalled workflows - stuck in discovery for too long
	for _, w := range s.Workflows {
		if w.CurrentPhase == PhaseDiscovery && time.Since(w.StartTime) > 7*24*time.Hour {
			recs = append(recs, Recommendation{
				ID:        fmt.Sprintf("compliance-stalled-%s", w.Standard),
				Title:     fmt.Sprintf("%s workflow stalled in discovery", w.Standard),
				Priority:  "medium",
				Action:    fmt.Sprintf("Advance %s workflow to gap analysis phase; discovery should not exceed 7 days", w.Standard),
				Reason:    "Workflow has been in discovery for over 7 days; risk of non-compliance increases with delay",
				Standard:  w.Standard,
				Phase:     PhaseGapAnalysis,
			})
		}
	}

	// No SOC2 for multi-tenant or service-providing NAS
	if !s.HasSOC2 && (s.HasGDPR || s.HasHIPAA) {
		recs = append(recs, Recommendation{
			ID:       "compliance-start-soc2",
			Title:    "SOC2 recommended alongside existing compliance",
			Priority: "medium",
			Action:   "Initialize SOC2 compliance workflow to demonstrate operational security controls",
			Reason:   "Organizations with GDPR/HIPAA data should also pursue SOC2 for comprehensive control framework",
			Standard: StandardSOC2,
			Phase:    PhaseDiscovery,
		})
	}

	// No ISO 27001
	if !s.HasISO27001 && (s.HasGDPR || s.HasHIPAA || s.HasSOC2) {
		recs = append(recs, Recommendation{
			ID:       "compliance-start-iso27001",
			Title:    "ISO 27001 certification recommended",
			Priority: "low",
			Action:   "Initiate ISO 27001 ISMS design as a longer-term compliance goal",
			Reason:   "ISO 27001 provides internationally recognized security management framework; strengthens posture",
			Standard: StandardISO27001,
			Phase:    PhaseDiscovery,
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityValue(recs[i].Priority) > priorityValue(recs[j].Priority)
	})

	return recs
}

func priorityValue(p string) int {
	switch p {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}