// Package ssdcacheschedule implements SSD cache intelligent scheduling inspired by
// Synology SSD Cache Advisor, TrueNAS L2ARC tiering, and fnOS hybrid flash.
package ssdcacheschedule

import (
	"sort"
	"time"
)

// CacheTier indicates the SSD cache tiering mode.
type CacheTier string

const (
	TierReadOnly  CacheTier = "read_only"
	TierReadWrite CacheTier = "read_write"
	TierWriteBack CacheTier = "write_back"
)

// CacheMode indicates the SSD cache operational mode.
type CacheMode string

const (
	ModeAdaptive  CacheMode = "adaptive"
	ModePinned    CacheMode = "pinned"
	ModeScheduled CacheMode = "scheduled"
)

// Signal describes the current SSD cache state and usage signals.
type Signal struct {
	SSDCachePresent      bool
	CacheTier            CacheTier
	CacheMode            CacheMode
	SSDCapacityGB        int
	UsedCacheGB          int
	CacheHitRate         float64
	WorkingSetGB         int
	TotalPoolGB          int
	SSDWearLevelPct      int
	SSDTemperatureC      int
	HasNVMeSSD           bool
	SmoothingSchedule    bool
	LastRefreshAge       time.Duration
	BackupsRunningAtNight bool
	ReadAmplification    float64
	WriteAmplification   float64
}

// Recommendation is an actionable SSD cache suggestion.
type Recommendation struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Priority   string    `json:"priority"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	ScheduleAt string    `json:"schedule_at,omitempty"`
}

// Analyze evaluates SSD cache signals and returns recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if !s.SSDCachePresent && s.TotalPoolGB > 500 {
		recs = append(recs, Recommendation{
			ID:       "ssd-add-cache",
			Title:    "Add SSD cache for large storage pool",
			Priority: "medium",
			Action:   "Install NVMe SSDs and enable read-only cache on the large pool",
			Reason:   "Pools over 500GB without SSD cache will have slow random read performance",
		})
	}

	if s.SSDCachePresent && s.CacheHitRate < 0.3 && s.WorkingSetGB > s.SSDCapacityGB {
		recs = append(recs, Recommendation{
			ID:       "ssd-expand-cache",
			Title:    "Expand SSD cache capacity",
			Priority: "high",
			Action:   "Add more SSDs or replace with larger NVMe drives to cover the working set",
			Reason:   "Cache hit rate below 30% means the working set exceeds cache capacity",
		})
	}

	if s.SSDCachePresent && s.SSDWearLevelPct > 80 {
		recs = append(recs, Recommendation{
			ID:       "ssd-wear-alert",
			Title:    "SSD cache near wear limit",
			Priority: "high",
			Action:   "Replace SSD cache drives before endurance limit is reached",
			Reason:   "SSD wear above 80% risks cache failure and data unavailability",
		})
	}

	if s.SSDCachePresent && s.SSDTemperatureC > 70 {
		recs = append(recs, Recommendation{
			ID:       "ssd-temp-throttle",
			Title:    "SSD cache overheating",
			Priority: "high",
			Action:   "Improve airflow or reduce cache scheduling intensity during peak hours",
			Reason:   "SSD temperature above 70C causes throttling and reduces drive lifespan",
		})
	}

	if s.SSDCachePresent && s.ReadAmplification > 4.0 {
		recs = append(recs, Recommendation{
			ID:       "ssd-read-amp",
			Title:    "High read amplification on SSD cache",
			Priority: "medium",
			Action:   "Switch to adaptive mode and pin hot datasets to reduce read amplification",
			Reason:   "Read amplification above 4x wastes cache IOPS and increases wear",
		})
	}

	if s.SSDCachePresent && s.WriteAmplification > 6.0 && s.CacheTier != TierReadOnly {
		recs = append(recs, Recommendation{
			ID:       "ssd-write-amp",
			Title:    "High write amplification on SSD cache",
			Priority: "medium",
			Action:   "Consider switching to read-only tier or enabling write coalescing",
			Reason:   "Write amplification above 6x significantly reduces SSD lifespan",
		})
	}

	if s.SSDCachePresent && !s.SmoothingSchedule && s.BackupsRunningAtNight {
		recs = append(recs, Recommendation{
			ID:         "ssd-smooth-schedule",
			Title:      "Enable cache smoothing schedule",
			Priority:   "low",
			Action:     "Schedule cache warming at 2AM before backup windows start",
			Reason:     "Pre-warming cache before backup runs improves hit rate and reduces latency spikes",
			ScheduleAt: "02:00",
		})
	}

	if s.SSDCachePresent && s.LastRefreshAge > 24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "ssd-refresh-cache",
			Title:    "Refresh stale cache contents",
			Priority: "low",
			Action:   "Trigger a cache refresh to evict stale data and re-populate hot blocks",
			Reason:   "Cache contents older than 24 hours may not reflect current working set",
		})
	}

	if s.HasNVMeSSD && s.CacheTier == TierReadOnly && s.WriteAmplification == 0 {
		recs = append(recs, Recommendation{
			ID:       "ssd-promote-rw",
			Title:    "Consider read-write cache tier",
			Priority: "low",
			Action:   "NVMe SSDs support read-write caching; evaluate write workload to potentially upgrade tier",
			Reason:   "NVMe SSDs have enough endurance for read-write caching with proper wear management",
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