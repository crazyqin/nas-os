// Package restoreconfidence implements ransomware recovery Clean Restore
// Confidence assessment inspired by TrueNAS data-layer recovery standards.
package restoreconfidence

import (
	"fmt"
	"sort"
	"time"
)

// ConfidenceLevel indicates the confidence in a clean restore.
type ConfidenceLevel string

const (
	ConfidenceHigh       ConfidenceLevel = "high"         // >95% confidence
	ConfidenceMedium     ConfidenceLevel = "medium"        // 70-95%
	ConfidenceLow        ConfidenceLevel = "low"           // 40-70%
	ConfidenceVeryLow    ConfidenceLevel = "very_low"      // <40%
	ConfidenceUnknown    ConfidenceLevel = "unknown"       // cannot determine
)

// SnapshotInfo describes a recovery snapshot.
type SnapshotInfo struct {
	ID            string    `json:"id"`
	Dataset       string    `json:"dataset"`
	CreatedAt     time.Time `json:"created_at"`
	IsImmutable   bool      `json:"is_immutable"`
	SizeGB        int       `json:"size_gb"`
	HasReplica    bool      `json:"has_replica"`
	ReplicaLag    time.Duration `json:"replica_lag"`
	VerifiedClean bool      `json:"verified_clean"`      // scan results
	ScanDate      time.Time `json:"scan_date"`
}

// RecoveryTarget describes a planned recovery operation.
type RecoveryTarget struct {
	DatasetName    string    `json:"dataset_name"`
	SnapshotID     string    `json:"snapshot_id"`
	TargetRPOMinutes int     `json:"target_rpo_minutes"`
	TargetRTOMinutes int     `json:"target_rto_minutes"`
	ActualRPOMinutes  int     `json:"actual_rpo_minutes"`
	ActualRTOMinutes  int     `json:"actual_rto_minutes"`
}

// Signal aggregates restore confidence signals for analysis.
type Signal struct {
	Snapshots          []SnapshotInfo   `json:"snapshots"`
	RecoveryTargets    []RecoveryTarget `json:"recovery_targets"`
	HasDrillIn90Days   bool             `json:"has_drill_in_90_days"`
	HasDrillIn30Days   bool             `json:"has_drill_in_30_days"`
	LastDrillDate      time.Time        `json:"last_drill_date"`
	LastDrillSuccess   bool             `json:"last_drill_success"`
	DrillRTOActual     int             `json:"drill_rto_actual_minutes"`
	DrillRPOActual     int             `json:"drill_rpo_actual_minutes"`
	HasImmutableBackup  bool             `json:"has_immutable_backup"`
	HasOffsiteReplica   bool             `json:"has_offsite_replica"`
	HasRansomwareDetection bool         `json:"has_ransomware_detection"`
	ScanEnabled         bool             `json:"scan_enabled"`
	LastScanDate        time.Time       `json:"last_scan_date"`
	TotalDatasets       int             `json:"total_datasets"`
	SnapshotsPerDataset float64        `json:"snapshots_per_dataset"`
	EncryptionEnabled   bool            `json:"encryption_enabled"`
	TFAEnabled          bool            `json:"tfa_enabled"`
	AlertingEnabled    bool            `json:"alerting_enabled"`
	HasRollbackPlan     bool            `json:"has_rollback_plan"`
}

// Recommendation is an actionable restore confidence suggestion.
type Recommendation struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Priority    string           `json:"priority"`
	Action      string           `json:"action"`
	Reason      string           `json:"reason"`
	Confidence  ConfidenceLevel  `json:"confidence,omitempty"`
}

// Analyze evaluates restore confidence signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	// No drill in 90 days
	if !s.HasDrillIn90Days {
		recs = append(recs, Recommendation{
			ID:       "restore-drill-overdue",
			Title:    "No recovery drill in 90+ days",
			Priority: "critical",
			Action:   "Schedule and execute a full recovery drill immediately; verify RTO/RPO targets",
			Reason:   "Recovery confidence without recent drills is unproven; organizations should drill at least quarterly",
		})
	}

	// No drill in 30 days
	if s.HasDrillIn90Days && !s.HasDrillIn30Days {
		recs = append(recs, Recommendation{
			ID:       "restore-drill-monthly",
			Title:    "Monthly recovery drill recommended",
			Priority: "high",
			Action:   "Schedule monthly mini-drills (single dataset recovery) to maintain readiness",
			Reason:   "Quarterly drills alone may miss configuration drift; monthly spot checks maintain confidence",
		})
	}

	// Last drill failed
	if s.LastDrillDate != (time.Time{}) && !s.LastDrillSuccess {
		recs = append(recs, Recommendation{
			ID:       "restore-drill-failed",
			Title:    "Last recovery drill failed",
			Priority: "critical",
			Action:   "Investigate failure root cause, remediate, and schedule a follow-up drill within 7 days",
			Reason:   "A failed drill means current restore capability is unverified; this is a critical risk",
		})
	}

	// RTO exceeds target
	for _, rt := range s.RecoveryTargets {
		if rt.ActualRTOMinutes > rt.TargetRTOMinutes && rt.TargetRTOMinutes > 0 {
			recs = append(recs, Recommendation{
				ID:       fmt.Sprintf("restore-rto-exceeded-%s", rt.DatasetName),
				Title:    fmt.Sprintf("RTO exceeded for %s", rt.DatasetName),
				Priority: "high",
				Action:   fmt.Sprintf("Reduce %s recovery time from %dmin to target %dmin", rt.DatasetName, rt.ActualRTOMinutes, rt.TargetRTOMinutes),
				Reason:   fmt.Sprintf("RTO of %dmin exceeds %dmin target; consider incremental replication or faster restore", rt.ActualRTOMinutes, rt.TargetRTOMinutes),
			})
		}
		if rt.ActualRPOMinutes > rt.TargetRPOMinutes && rt.TargetRPOMinutes > 0 {
			recs = append(recs, Recommendation{
				ID:       fmt.Sprintf("restore-rpo-exceeded-%s", rt.DatasetName),
				Title:    fmt.Sprintf("RPO exceeded for %s", rt.DatasetName),
				Priority: "high",
				Action:   fmt.Sprintf("Increase %s snapshot frequency from %dmin to %dmin intervals", rt.DatasetName, rt.ActualRPOMinutes, rt.TargetRPOMinutes),
				Reason:   fmt.Sprintf("RPO of %dmin exceeds %dmin target; data loss window too wide", rt.ActualRPOMinutes, rt.TargetRPOMinutes),
			})
		}
	}

	// No immutable backups
	if !s.HasImmutableBackup {
		recs = append(recs, Recommendation{
			ID:       "restore-immutable-missing",
			Title:    "No immutable backup configured",
			Priority: "critical",
			Action:   "Enable immutable snapshots or WORM storage for at least one backup target",
			Reason:   "Without immutable backups, ransomware can encrypt or delete recovery data, eliminating restore option",
		})
	}

	// No offsite replica
	if !s.HasOffsiteReplica {
		recs = append(recs, Recommendation{
			ID:       "restore-offsite-missing",
			Title:    "No offsite replica detected",
			Priority: "high",
			Action:   "Configure replication to an offsite NAS or cloud target for disaster recovery",
			Reason:   "Local-only backups share blast radius with production; offsite copies are essential for ransomware recovery",
		})
	}

	// Insufficient snapshots
	if s.TotalDatasets > 0 && s.SnapshotsPerDataset < 3 {
		recs = append(recs, Recommendation{
			ID:       "restore-insufficient-snapshots",
			Title:    "Insufficient snapshot frequency",
			Priority: "high",
			Action:   "Increase snapshot frequency to at least 3 per dataset (hourly/daily/weekly)",
			Reason:   fmt.Sprintf("Only %.1f snapshots per dataset; recovery points are too sparse for confident restore", s.SnapshotsPerDataset),
		})
	}

	// No ransomware detection
	if !s.HasRansomwareDetection {
		recs = append(recs, Recommendation{
			ID:       "restore-no-detection",
			Title:    "Ransomware detection not enabled",
			Priority: "high",
			Action:   "Enable file integrity monitoring and anomaly detection for early ransomware alerts",
			Reason:   "Without detection, recovery may start too late; early detection enables faster and cleaner restore",
		})
	}

	// No scan verification
	if !s.ScanEnabled {
		recs = append(recs, Recommendation{
			ID:       "restore-no-scan",
			Title:    "Snapshot scanning not enabled",
			Priority: "medium",
			Action:   "Enable automated malware scanning of snapshots before restore to verify clean state",
			Reason:   "Restoring from an infected snapshot re-introduces ransomware; scan before restore",
		})
	}

	// Stale scan
	if s.ScanEnabled && !s.LastScanDate.IsZero() && time.Since(s.LastScanDate) > 7*24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "restore-stale-scan",
			Title:    "Last snapshot scan over 7 days ago",
			Priority: "medium",
			Action:   "Run snapshot scan immediately to verify recovery point cleanliness",
			Reason:   "Clean restore confidence degrades without recent scan results",
		})
	}

	// TFA not enabled
	if !s.TFAEnabled {
		recs = append(recs, Recommendation{
			ID:       "restore-no-tfa",
			Title:    "Two-factor authentication not enabled",
			Priority: "high",
			Action:   "Enable TFA for all admin accounts to prevent credential compromise targeting recovery data",
			Reason:   "Attackers with admin creds can delete snapshots/replicas; TFA is the last line of defense",
		})
	}

	// No rollback plan
	if !s.HasRollbackPlan {
		recs = append(recs, Recommendation{
			ID:       "restore-no-rollback-plan",
			Title:    "No documented rollback plan",
			Priority: "high",
			Action:   "Document step-by-step rollback procedures for each critical dataset",
			Reason:   "During ransomware recovery, undocumented procedures lead to errors and delays",
		})
	}

	// Encryption not enabled
	if !s.EncryptionEnabled {
		recs = append(recs, Recommendation{
			ID:       "restore-no-encryption",
			Title:    "Encryption not enabled on storage",
			Priority: "medium",
			Action:   "Enable ZFS native encryption to protect data at rest and during recovery",
			Reason:   "Unencrypted snapshots risk data exposure during replication and restore operations",
		})
	}

	// Confidence summary
	confidence := computeConfidence(s)
	if confidence == ConfidenceVeryLow {
		recs = append(recs, Recommendation{
			ID:         "restore-confidence-very-low",
			Title:      "Clean Restore Confidence is very low",
			Priority:   "critical",
			Action:     "Address all critical and high priority items above to build minimum viable recovery capability",
			Reason:     "Current configuration lacks immutability, drills, detection, and TFA; recovery capability is severely compromised",
			Confidence: confidence,
		})
	} else if confidence == ConfidenceLow {
		recs = append(recs, Recommendation{
			ID:         "restore-confidence-low",
			Title:      "Clean Restore Confidence is low",
			Priority:   "high",
			Action:     "Address high priority items; particularly drills, immutable backup, and offsite replication",
			Reason:     "Recovery capability exists but has significant gaps; confidence below enterprise minimum",
			Confidence: confidence,
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityValue(recs[i].Priority) > priorityValue(recs[j].Priority)
	})

	return recs
}

func computeConfidence(s Signal) ConfidenceLevel {
	score := 0
	maxScore := 10

	if s.HasDrillIn30Days { score += 2 }
	if s.HasDrillIn90Days { score += 1 }
	if s.LastDrillSuccess { score += 1 }
	if s.HasImmutableBackup { score += 2 }
	if s.HasOffsiteReplica { score += 1 }
	if s.HasRansomwareDetection { score += 1 }
	if s.ScanEnabled { score += 1 }
	if s.TFAEnabled { score += 1 }

	pct := float64(score) / float64(maxScore)
	switch {
	case pct >= 0.95:
		return ConfidenceHigh
	case pct >= 0.70:
		return ConfidenceMedium
	case pct >= 0.40:
		return ConfidenceLow
	default:
		return ConfidenceVeryLow
	}
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