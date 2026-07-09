// Package powermanager implements intelligent power management scheduling inspired by
// Synology Power Schedule, TrueNAS power management, and fnOS green computing.
package powermanager

import (
	"sort"
	"time"
)

// PowerMode indicates the NAS power management mode.
type PowerMode string

const (
	ModeActive     PowerMode = "active"
	ModeIdle       PowerMode = "idle"
	ModeStandby    PowerMode = "standby"
	ModeSuspend    PowerMode = "suspend"
	ModeHibernate  PowerMode = "hibernate"
)

// DiskSpinPolicy indicates when disks should spin down.
type DiskSpinPolicy string

const (
	SpinAfter10   DiskSpinPolicy = "after_10min"
	SpinAfter20   DiskSpinPolicy = "after_20min"
	SpinAfter30   DiskSpinPolicy = "after_30min"
	SpinNever     DiskSpinPolicy = "never"
)

// Signal describes the current power management state.
type Signal struct {
	CurrentMode          PowerMode     `json:"current_mode"`
	ScheduledMode        PowerMode     `json:"scheduled_mode"`
	DiskSpinPolicy       DiskSpinPolicy `json:"disk_spin_policy"`
	IdleSince            time.Duration `json:"idle_since"`
	ScheduledWakeTime    string        `json:"scheduled_wake_time"`
	WakeOnLAN            bool          `json:"wake_on_lan"`
	HasSSDCache          bool          `json:"has_ssd_cache"`
	ActiveUsers          int           `json:"active_users"`
	RunningTasks         int           `json:"running_tasks"`
	LastBackupComplete   time.Time     `json:"last_backup_complete"`
	NextBackupScheduled  time.Time     `json:"next_backup_scheduled"`
	PowerConsumptionW    float64       `json:"power_consumption_w"`
	IdlePowerW           float64       `json:"idle_power_w"`
	DailyActiveHours     float64       `json:"daily_active_hours"`
	NightlySchedule      bool          `json:"nightly_schedule"`
	HasSolar             bool          `json:"has_solar"`
	SolarPeakHours       string        `json:"solar_peak_hours"`
}

// Recommendation is an actionable power management suggestion.
type Recommendation struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Priority   string `json:"priority"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	ScheduleAt string `json:"schedule_at,omitempty"`
}

// Analyze evaluates power management signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if s.CurrentMode == ModeActive && s.IdleSince > 2*time.Hour && s.ActiveUsers == 0 && s.RunningTasks == 0 {
		recs = append(recs, Recommendation{
			ID:       "pm-enter-idle",
			Title:    "Switch to idle mode",
			Priority: "high",
			Action:   "Transition NAS to idle mode; spin down HDDs and reduce CPU frequency",
			Reason:   "System has been idle with no users or tasks for over 2 hours",
		})
	}

	if s.IdleSince > 6*time.Hour && s.ActiveUsers == 0 && s.RunningTasks == 0 {
		nextBackup := s.NextBackupScheduled
		if nextBackup.IsZero() || time.Until(nextBackup) > 1*time.Hour {
			recs = append(recs, Recommendation{
				ID:         "pm-enter-standby",
				Title:      "Enter standby mode",
				Priority:   "medium",
				Action:     "Suspend non-essential services and enter standby; schedule wake 15 min before next backup",
				Reason:     "System idle over 6 hours with no imminent scheduled tasks",
				ScheduleAt: s.ScheduledWakeTime,
			})
		}
	}

	if s.DiskSpinPolicy == SpinNever && s.HasSSDCache && s.IdleSince > 30*time.Minute {
		recs = append(recs, Recommendation{
			ID:       "pm-disk-spin-down",
			Title:    "Enable HDD spin-down timer",
			Priority: "medium",
			Action:   "Set HDD spin-down timer to 20 minutes; SSD cache will continue serving reads",
			Reason:   "HDDs never spin down despite SSD cache and idle system; power is wasted",
		})
	}

	if s.PowerConsumptionW > s.IdlePowerW*3 && s.ActiveUsers == 0 && s.RunningTasks == 0 {
		recs = append(recs, Recommendation{
			ID:       "pm-high-power-idle",
			Title:    "High power consumption during idle",
			Priority: "high",
			Action:   "Investicate background processes consuming excessive power; check for stuck Docker containers",
			Reason:   "Power draw is 3x above idle baseline with no users or running tasks",
		})
	}

	if !s.NightlySchedule && s.DailyActiveHours > 20 {
		recs = append(recs, Recommendation{
			ID:         "pm-nightly-schedule",
			Title:      "Enable nightly power schedule",
			Priority:   "medium",
			Action:     "Schedule standby from 1AM to 6AM; wake for scheduled backups",
			Reason:     "System runs over 20 hours daily; nightly standby can save significant power",
			ScheduleAt: "01:00-06:00",
		})
	}

	if s.HasSolar && s.SolarPeakHours != "" {
		recs = append(recs, Recommendation{
			ID:         "pm-solar-align",
			Title:      "Align heavy tasks with solar peak",
			Priority:   "low",
			Action:     "Schedule scrub, backup, and indexing tasks during solar peak hours to use free energy",
			Reason:     "Solar panel output peaks during midday; run heavy I/O tasks then to save grid power",
			ScheduleAt: s.SolarPeakHours,
		})
	}

	if !s.WakeOnLAN && s.CurrentMode != ModeActive && s.CurrentMode != "" && (s.CurrentMode == ModeStandby || s.CurrentMode == ModeSuspend) {
		recs = append(recs, Recommendation{
			ID:       "pm-enable-wol",
			Title:    "Enable Wake-on-LAN",
			Priority: "medium",
			Action:   "Enable WoL on NAS network interface to allow remote wake when access is needed",
			Reason:   "Without WoL, system in standby cannot be remotely woken; users must be physically present",
		})
	}

	if s.RunningTasks > 0 && s.CurrentMode == ModeStandby {
		recs = append(recs, Recommendation{
			ID:       "pm-wake-for-tasks",
			Title:    "Wake from standby for running tasks",
			Priority: "high",
			Action:   "Cancel standby; running tasks require full system power",
			Reason:   "Running tasks detected but system is in standby; tasks may be stalled",
		})
	}

	if s.ActiveUsers > 0 && (s.CurrentMode == ModeStandby || s.CurrentMode == ModeSuspend) {
		recs = append(recs, Recommendation{
			ID:       "pm-wake-for-users",
			Title:    "Wake NAS for active users",
			Priority: "high",
			Action:   "Cancel suspend/standby; users are actively connected",
			Reason:   "Active users detected but system is in low-power mode; user experience is degraded",
		})
	}

	if s.HasSSDCache && s.DiskSpinPolicy != SpinNever && s.IdleSince < 10*time.Minute {
		recs = append(recs, Recommendation{
			ID:       "pm-short-spin-timer",
			Title:    "HDD spin-down timer too aggressive",
			Priority: "low",
			Action:   "Increase spin-down timer to at least 20 minutes to reduce spin-up wear cycles",
			Reason:   "Short spin-down timer with SSD cache causes unnecessary spin cycles",
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