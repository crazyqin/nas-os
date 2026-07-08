// Package fworchestrator manages firmware update orchestration inspired by
// TrueNAS update coordinator and Synology DSM update management.
package fworchestrator

import (
	"sort"
	"strings"
	"time"
)

// UpdatePhase represents a stage in the update process.
const (
	PhaseIdle       = "idle"
	PhasePreCheck   = "pre-check"
	PhaseBackup     = "backup"
	PhaseDownload   = "download"
	PhaseStaging    = "staging"
	PhaseInstall    = "install"
	PhasePostCheck  = "post-check"
	PhaseReboot     = "reboot"
	PhaseVerify     = "verify"
	PhaseComplete   = "complete"
	PhaseFailed     = "failed"
	PhaseRollback   = "rollback"
)

// UpdateStatus represents the current state of a firmware update.
type UpdateStatus struct {
	Phase            string     `json:"phase"`
	CurrentVersion   string     `json:"current_version"`
	TargetVersion    string     `json:"target_version"`
	ProgressPercent  int        `json:"progress_percent"`
	StartedAt        time.Time  `json:"started_at,omitempty"`
	CompletedAt      time.Time  `json:"completed_at,omitempty"`
	Error            string     `json:"error,omitempty"`
	PreCheckPassed   bool       `json:"pre_check_passed"`
	BackupCreated    bool       `json:"backup_created"`
	DownloadComplete bool       `json:"download_complete"`
	StagedOK          bool       `json:"staged_ok"`
	InstalledOK       bool       `json:"installed_ok"`
	PostCheckPassed   bool       `json:"post_check_passed"`
	RebootRequired    bool       `json:"reboot_required"`
	RollbackAvailable bool       `json:"rollback_available"`
}

// Signal describes the firmware update environment.
type Signal struct {
	CurrentVersion       string
	AvailableVersion     string
	UpdateAvailable      bool
	IsCriticalUpdate     bool
	RunningServices      int
	ActiveConnections    int
	FreeSpaceMB          int
	HasBackup            bool
	LastUpdateTime       time.Time
	FailedUpdates        int
	MaintenanceWindow    bool
	DiskHealthOK         bool
	Uptime              time.Duration
}

// Recommendation is an actionable firmware update suggestion.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// Analyze evaluates firmware update signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if !s.UpdateAvailable {
		if time.Since(s.LastUpdateTime) > 30*24*time.Hour {
			recs = append(recs, Recommendation{
				ID:       "fw-check-updates",
				Title:    "Check for firmware updates",
				Priority: "low",
				Action:   "Run manual update check; last update over 30 days ago",
				Reason:   "Regular update checks ensure security patches are applied",
			})
		}
		sort.Slice(recs, func(i, j int) bool {
			return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
		})
		return recs
	}

	if !s.DiskHealthOK {
		recs = append(recs, Recommendation{
			ID:       "fw-disk-health",
			Title:    "Fix disk health before updating",
			Priority: "critical",
			Action:   "Replace failing disks before proceeding with firmware update",
			Reason:   "Updating with unhealthy disks risks data loss",
		})
	}

	if s.FreeSpaceMB < 2048 {
		recs = append(recs, Recommendation{
			ID:       "fw-space",
			Title:    "Free up space before updating",
			Priority: "high",
			Action:   "Ensure at least 2GB free space for update staging",
			Reason:   "Insufficient space can corrupt the update process",
		})
	}

	if !s.MaintenanceWindow && s.ActiveConnections > 0 {
		recs = append(recs, Recommendation{
			ID:       "fw-schedule",
			Title:    "Schedule update in maintenance window",
			Priority: "medium",
			Action:   "Notify users and schedule update during low-traffic period",
			Reason:   "Active connections may be disrupted during update",
		})
	}

	if !s.HasBackup {
		recs = append(recs, Recommendation{
			ID:       "fw-backup",
			Title:    "Create backup before updating",
			Priority: "high",
			Action:   "Run full system backup before applying firmware update",
			Reason:   "Backup enables rollback if update fails",
		})
	}

	if s.FailedUpdates > 0 {
		recs = append(recs, Recommendation{
			ID:       "fw-investigate-failures",
			Title:    "Investigate previous update failures",
			Priority: "high",
			Action:   "Check update logs and resolve root cause before retry",
			Reason:   "Repeated update failures indicate systemic issues",
		})
	}

	if s.IsCriticalUpdate && s.UpdateAvailable {
		recs = append(recs, Recommendation{
			ID:       "fw-critical-apply",
			Title:    "Apply critical update immediately",
			Priority: "critical",
			Action:   "Apply critical security update in next maintenance window",
			Reason:   "Critical updates address security vulnerabilities",
		})
	}

	if s.Uptime < 5*time.Minute && s.UpdateAvailable {
		recs = append(recs, Recommendation{
			ID:       "fw-wait-boot",
			Title:    "Wait for system stability",
			Priority: "medium",
			Action:   "Wait 5 minutes after boot before updating",
			Reason:   "Updating too soon after boot may fail",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}

// PlanUpdate creates an update plan with phases.
func PlanUpdate(current, target string) []UpdateStatus {
	phases := []string{
		PhasePreCheck,
		PhaseBackup,
		PhaseDownload,
		PhaseStaging,
		PhaseInstall,
		PhasePostCheck,
		PhaseReboot,
		PhaseVerify,
	}
	plan := make([]UpdateStatus, 0, len(phases))
	for i, p := range phases {
		plan = append(plan, UpdateStatus{
			Phase:          p,
			CurrentVersion: current,
			TargetVersion:  target,
			ProgressPercent: int(float64(i) / float64(len(phases)) * 100),
		})
	}
	return plan
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
