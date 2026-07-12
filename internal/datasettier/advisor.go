// Package datasettier implements ZFS dataset tiering scheduler inspired by
// TrueNAS 26 OpenZFS 2.4 dataset tiering and Synology storage efficiency tiering.
package datasettier

import (
	"fmt"
	"sort"
	"time"
)

// TierLevel indicates the storage tier for a dataset.
type TierLevel string

const (
	TierFlash  TierLevel = "flash"   // NVMe/SSD hot tier
	TierHybrid TierLevel = "hybrid"  // SSD cache + HDD
	TierHDD    TierLevel = "hdd"     // spinning disk cold tier
	TierArchive TierLevel = "archive" // tape/cloud archive
)

// TierPolicy indicates the tiering policy mode.
type TierPolicy string

const (
	PolicyAuto      TierPolicy = "auto"       // automatic tiering based on access patterns
	PolicyManual    TierPolicy = "manual"     // manually pinned tier
	PolicyScheduled TierPolicy = "scheduled"  // scheduled movement
	PolicyPredictive TierPolicy = "predictive" // AI-driven predictive tiering
)

// Dataset describes a ZFS dataset with tiering signals.
type Dataset struct {
	Name           string    `json:"name"`
	CurrentTier    TierLevel `json:"current_tier"`
	TargetTier     TierLevel `json:"target_tier,omitempty"`
	SizeGB         int       `json:"size_gb"`
	UsedGB         int       `json:"used_gb"`
	AccessFrequency float64  `json:"access_frequency"` // accesses per day
	ReadBlocked    int64     `json:"read_blocked"`      // blocks read last 7d
	WriteBlocked   int64     `json:"write_blocked"`     // blocks written last 7d
	LastAccessed   time.Time `json:"last_accessed"`
	Policy         TierPolicy `json:"policy"`
	Pinned         bool      `json:"pinned"`
	IsSnapshot     bool      `json:"is_snapshot"`
}

// Pool describes the storage pool tiering state.
type Pool struct {
	Name           string    `json:"name"`
	FlashCapacityGB int     `json:"flash_capacity_gb"`
	FlashUsedGB    int       `json:"flash_used_gb"`
	HDDBCapacityGB int       `json:"hdd_capacity_gb"`
	HDDUsedGB      int       `json:"hdd_used_gb"`
	ArchiveEnabled bool      `json:"archive_enabled"`
	Datasets       []Dataset `json:"datasets"`
	HasOpenZFS24   bool      `json:"has_openzfs_2_4"`
}

// Signal aggregates dataset tiering signals for analysis.
type Signal struct {
	Pool                 Pool     `json:"pool"`
	FlashHitRate         float64  `json:"flash_hit_rate"`
	HDDBusyPct           float64  `json:"hdd_busy_pct"`
	HotDatasetsOnHDD     int      `json:"hot_datasets_on_hdd"`
	ColdDatasetsOnFlash  int      `json:"cold_datasets_on_flash"`
	TierOverflowRisk     bool     `json:"tier_overflow_risk"`
	LastTieringRun       time.Time `json:"last_tiering_run"`
	AutoTierEnabled      bool      `json:"auto_tier_enabled"`
	PredictiveEnabled     bool     `json:"predictive_enabled"`
}

// Recommendation is an actionable dataset tiering suggestion.
type Recommendation struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Priority   string     `json:"priority"`
	Action     string     `json:"action"`
	Reason     string     `json:"reason"`
	Dataset    string     `json:"dataset,omitempty"`
	FromTier   TierLevel  `json:"from_tier,omitempty"`
	ToTier     TierLevel  `json:"to_tier,omitempty"`
	ScheduleAt string     `json:"schedule_at,omitempty"`
}

// Analyze evaluates dataset tiering signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	// Auto-tier not enabled with OpenZFS 2.4
	if s.Pool.HasOpenZFS24 && !s.AutoTierEnabled && len(s.Pool.Datasets) > 10 {
		recs = append(recs, Recommendation{
			ID:       "tier-enable-auto",
			Title:    "Enable automatic dataset tiering",
			Priority: "high",
			Action:   "Enable OpenZFS 2.4 dataset tiering to automatically move hot datasets to flash and cold to HDD",
			Reason:   "Pool has OpenZFS 2.4 and 10+ datasets but auto-tiering is disabled; significant performance gains available",
		})
	}

	// Hot datasets sitting on HDD
	if s.HotDatasetsOnHDD > 0 {
		recs = append(recs, Recommendation{
			ID:       "tier-promote-hot",
			Title:    "Promote hot datasets from HDD to flash tier",
			Priority: "high",
			Action:   "Move frequently accessed datasets to the flash tier to reduce latency",
			Reason:   fmt.Sprintf("%d hot datasets are on HDD despite flash capacity available", s.HotDatasetsOnHDD),
		})
	}

	// Cold datasets consuming flash
	if s.ColdDatasetsOnFlash > 0 {
		recs = append(recs, Recommendation{
			ID:       "tier-demote-cold",
			Title:    "Demote cold datasets from flash to HDD",
			Priority: "medium",
			Action:   "Move inactive datasets from flash to HDD to free SSD capacity for hot data",
			Reason:   fmt.Sprintf("%d cold datasets are occupying flash tier space unnecessarily", s.ColdDatasetsOnFlash),
		})
	}

	// Flash tier near full
	flashUsage := 0.0
	if s.Pool.FlashCapacityGB > 0 {
		flashUsage = float64(s.Pool.FlashUsedGB) / float64(s.Pool.FlashCapacityGB)
	}
	if flashUsage > 0.85 {
		recs = append(recs, Recommendation{
			ID:       "tier-flash-near-full",
			Title:    "Flash tier near capacity",
			Priority: "high",
			Action:   "Add more NVMe/SSD capacity or demote cold datasets to HDD to prevent tier overflow",
			Reason:   fmt.Sprintf("Flash tier at %.0f%% capacity, risk of tier overflow and performance degradation", flashUsage*100),
		})
	}

	if s.TierOverflowRisk {
		recs = append(recs, Recommendation{
			ID:       "tier-overflow-risk",
			Title:    "Tier overflow risk detected",
			Priority: "critical",
			Action:   "Immediately demote least-recently-accessed datasets or expand flash capacity",
			Reason:   "Flash tier is at risk of overflow, which can cause write failures and data relocation errors",
		})
	}

	// Predictive tiering recommendation
	if s.Pool.HasOpenZFS24 && !s.PredictiveEnabled && s.FlashHitRate < 0.5 {
		recs = append(recs, Recommendation{
			ID:       "tier-enable-predictive",
			Title:    "Enable predictive tiering with AI access pattern analysis",
			Priority: "medium",
			Action:   "Enable AI-driven predictive tiering to pre-stage data before access patterns change",
			Reason:   "Flash hit rate below 50% indicates reactive tiering is too slow; predictive mode can pre-promote datasets",
		})
	}

	// HDD busy with data that should be on flash
	if s.HDDBusyPct > 70 && s.FlashHitRate < 0.4 {
		recs = append(recs, Recommendation{
			ID:       "tier-hdd-overload",
			Title:    "HDD overloaded while flash underutilized",
			Priority: "high",
			Action:   "Identify high-I/O datasets on HDD and promote to flash tier",
			Reason:   fmt.Sprintf("HDD at %.0f%% busy with only %.0f%% flash hit rate suggests mis-tiered hot data", s.HDDBusyPct, s.FlashHitRate*100),
		})
	}

	// Stale tiering run
	if !s.LastTieringRun.IsZero() && time.Since(s.LastTieringRun) > 24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "tier-stale-run",
			Title:    "Tiering schedule is stale",
			Priority: "low",
			Action:   "Run dataset tiering analysis or enable scheduled auto-tiering",
			Reason:   fmt.Sprintf("Last tiering run was %s ago; access patterns may have shifted", time.Since(s.LastTieringRun).Round(time.Hour)),
		})
	}

	// Archive tier not enabled with cold datasets
	if !s.Pool.ArchiveEnabled {
		coldCount := 0
		for _, d := range s.Pool.Datasets {
			if !d.LastAccessed.IsZero() && time.Since(d.LastAccessed) > 90*24*time.Hour {
				coldCount++
			}
		}
		if coldCount >= 3 {
			recs = append(recs, Recommendation{
				ID:       "tier-enable-archive",
				Title:    "Enable archive tier for long-cold datasets",
				Priority: "medium",
				Action:   "Configure archive tier (cloud/tape) for datasets not accessed in 90+ days",
				Reason:   fmt.Sprintf("%d datasets haven't been accessed in 90+ days; archive tier can reclaim HDD space", coldCount),
			})
		}
	}

	// Individual dataset recommendations
	for _, d := range s.Pool.Datasets {
		if d.Pinned {
			continue
		}
		if d.CurrentTier == TierHDD && d.AccessFrequency > 100 && !d.IsSnapshot {
			recs = append(recs, Recommendation{
				ID:       "tier-promote-" + d.Name,
				Title:    fmt.Sprintf("Promote %s to flash tier", d.Name),
				Priority: "high",
				Action:   fmt.Sprintf("Move dataset %s from HDD to flash (accessed %.0f times/day)", d.Name, d.AccessFrequency),
				Reason:   "Dataset has high access frequency but is on cold tier",
				Dataset:  d.Name,
				FromTier: TierHDD,
				ToTier:   TierFlash,
			})
		}
		if d.CurrentTier == TierFlash && d.AccessFrequency < 1 && !d.IsSnapshot {
			recs = append(recs, Recommendation{
				ID:       "tier-demote-" + d.Name,
				Title:    fmt.Sprintf("Demote %s to HDD tier", d.Name),
				Priority: "low",
				Action:   fmt.Sprintf("Move dataset %s from flash to HDD (accessed only %.1f times/day)", d.Name, d.AccessFrequency),
				Reason:   "Dataset is on flash but has very low access frequency",
				Dataset:  d.Name,
				FromTier: TierFlash,
				ToTier:   TierHDD,
			})
		}
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