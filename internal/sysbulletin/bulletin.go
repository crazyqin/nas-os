// Package sysbulletin provides system bulletin/notification management inspired by
// DSM Notification Center and TrueNAS alert system.
package sysbulletin

import (
	"sort"
	"strings"
	"time"
)

// Severity levels for bulletins.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
	SeveritySecurity = "security"
)

// Category types for bulletins.
const (
	CategorySystem   = "system"
	CategoryStorage  = "storage"
	CategorySecurity = "security"
	CategoryUpdate   = "update"
	CategoryBackup   = "backup"
	CategoryNetwork  = "network"
	CategoryApp      = "app"
)

// Bulletin represents a system announcement.
type Bulletin struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Severity    string    `json:"severity"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Acknowledged bool     `json:"acknowledged"`
	Source      string    `json:"source"`
	ActionURL   string    `json:"action_url,omitempty"`
}

// Signal describes current system state for bulletin generation.
type Signal struct {
	SystemHealth       string
	PendingUpdates     int
	FailedBackups      int
	SMARTWarnings      int
	SecurityAlerts     int
	NetworkIssues      int
	DiskUsagePercent   int
	LastUpdateCheck    time.Time
	ActiveBulletins    []Bulletin
}

// Recommendation is an actionable bulletin suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// Generate creates bulletins from system signals.
func Generate(s Signal) ([]Bulletin, []Recommendation) {
	var bulletins []Bulletin
	var recs []Recommendation
	now := time.Now()

	if s.SMARTWarnings > 0 {
		b := Bulletin{
			ID:        "bulletin-smart-" + now.Format("20060102"),
			Title:     "Disk SMART warnings detected",
			Body:      "SMART warnings indicate potential disk failures. Please check disk health and plan replacements.",
			Severity:  SeverityCritical,
			Category:  CategoryStorage,
			CreatedAt: now,
			Source:    "sysbulletin",
		}
		b.ExpiresAt = now.Add(72 * time.Hour)
		bulletins = append(bulletins, b)
		recs = append(recs, Recommendation{
			ID:       "bulletin-smart-action",
			Title:    "Replace failing disks",
			Priority: "high",
			Action:   "Check disk health report and replace disks with SMART warnings",
			Reason:   "SMART warnings precede disk failure",
		})
	}

	if s.SecurityAlerts > 0 {
		b := Bulletin{
			ID:        "bulletin-security-" + now.Format("20060102"),
			Title:     "Security alerts require attention",
			Body:      "Multiple security alerts detected. Review login attempts, certificate status, and firewall rules.",
			Severity:  SeveritySecurity,
			Category:  CategorySecurity,
			CreatedAt: now,
			Source:    "sysbulletin",
		}
		b.ExpiresAt = now.Add(24 * time.Hour)
		bulletins = append(bulletins, b)
		recs = append(recs, Recommendation{
			ID:       "bulletin-security-action",
			Title:    "Review security alerts",
			Priority: "high",
			Action:   "Check intrusion log, failed logins, and certificate expiry",
			Reason:   "Security alerts may indicate active threats",
		})
	}

	if s.PendingUpdates > 0 {
		b := Bulletin{
			ID:        "bulletin-update-" + now.Format("20060102"),
			Title:     "System updates available",
			Body:      "Pending system updates. Apply during maintenance window to ensure security and stability.",
			Severity:  SeverityInfo,
			Category:  CategoryUpdate,
			CreatedAt: now,
			Source:    "sysbulletin",
		}
		b.ExpiresAt = now.Add(7 * 24 * time.Hour)
		bulletins = append(bulletins, b)
		recs = append(recs, Recommendation{
			ID:       "bulletin-update-action",
			Title:    "Apply system updates",
			Priority: "medium",
			Action:   "Schedule a maintenance window and apply pending updates",
			Reason:   "Updates contain security patches and bug fixes",
		})
	}

	if s.FailedBackups > 0 {
		b := Bulletin{
			ID:        "bulletin-backup-" + now.Format("20060102"),
			Title:     "Backup failures detected",
			Body:      "Some backup tasks have failed. Check backup logs and storage targets.",
			Severity:  SeverityWarning,
			Category:  CategoryBackup,
			CreatedAt: now,
			Source:    "sysbulletin",
		}
		b.ExpiresAt = now.Add(48 * time.Hour)
		bulletins = append(bulletins, b)
		recs = append(recs, Recommendation{
			ID:       "bulletin-backup-action",
			Title:    "Fix backup failures",
			Priority: "high",
			Action:   "Review backup logs and verify target storage connectivity",
			Reason:   "Backup failures risk data loss",
		})
	}

	if s.DiskUsagePercent > 90 {
		b := Bulletin{
			ID:        "bulletin-disk-" + now.Format("20060102"),
			Title:     "Disk usage critical",
			Body:      "Disk usage exceeds 90%. Clean up unnecessary files or expand storage immediately.",
			Severity:  SeverityCritical,
			Category:  CategoryStorage,
			CreatedAt: now,
			Source:    "sysbulletin",
		}
		b.ExpiresAt = now.Add(12 * time.Hour)
		bulletins = append(bulletins, b)
		recs = append(recs, Recommendation{
			ID:       "bulletin-disk-action",
			Title:    "Free up disk space",
			Priority: "high",
			Action:   "Delete old snapshots, clear temp files, or expand storage pool",
			Reason:   "Disk full can cause system instability",
		})
	}

	if s.NetworkIssues > 0 {
		b := Bulletin{
			ID:        "bulletin-network-" + now.Format("20060102"),
			Title:     "Network issues detected",
			Body:      "Network connectivity issues may affect remote access and replication.",
			Severity:  SeverityWarning,
			Category:  CategoryNetwork,
			CreatedAt: now,
			Source:    "sysbulletin",
		}
		b.ExpiresAt = now.Add(24 * time.Hour)
		bulletins = append(bulletins, b)
	}

	sort.Slice(bulletins, func(i, j int) bool {
		return severityRank(bulletins[i].Severity) < severityRank(bulletins[j].Severity)
	})

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return bulletins, recs
}

func severityRank(s string) int {
	switch strings.ToLower(s) {
	case SeverityCritical:
		return 0
	case SeveritySecurity:
		return 1
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 3
	default:
		return 4
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
