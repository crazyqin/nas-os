package hybridpoolmgr

import (
	"sort"
	"time"
)

// Signal represents the current state metrics of a hybrid storage pool.
type Signal struct {
	HybridPoolEnabled    bool
	FlashDeviceCount     int
	FlashDeviceTotalGB   int
	HDDDeviceCount       int
	HDDTotalGBGB         int
	FlashTierRatio       float64
	HotDataOnFlash       bool
	ColdDataMigrated     bool
	PoolUtilizationPct   int
	FragmentationScore   float64
	HasSpecialDeviceClass bool
	LastRebalanceAge     time.Duration
	AutoRebalance        bool
	FlashWearLevelPct    int
	FlashTemperatureC    int
}

// Recommendation is a structured suggestion produced by Analyze.
type Recommendation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// priorityRank maps priority strings to sortable integers (lower = more urgent).
func priorityRank(p string) int {
	switch p {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}

// Analyze inspects a Signal and returns a prioritized list of Recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	// 1. Hybrid pool not enabled but has both flash and HDD devices → recommend enabling
	if !s.HybridPoolEnabled && s.FlashDeviceCount > 0 && s.HDDDeviceCount > 0 {
		recs = append(recs, Recommendation{
			ID:       "enable-hybrid-pool",
			Title:    "Enable Hybrid Pool",
			Priority: "high",
			Action:   "Enable hybrid pool functionality to leverage flash and HDD tiers",
			Reason:   "Flash and HDD devices are present but hybrid pooling is not enabled",
		})
	}

	// 2. Flash tier ratio too low (< 5%) → recommend adding flash capacity
	if s.HybridPoolEnabled && s.FlashTierRatio < 0.05 {
		recs = append(recs, Recommendation{
			ID:       "increase-flash-capacity",
			Title:    "Increase Flash Tier Capacity",
			Priority: "medium",
			Action:   "Add more flash devices to increase the flash tier ratio above 5%",
			Reason:   "Flash tier ratio is below 5%, which may result in poor caching performance",
		})
	}

	// 3. Hot data not on flash → recommend enabling hot data tiering
	if s.HybridPoolEnabled && !s.HotDataOnFlash {
		recs = append(recs, Recommendation{
			ID:       "enable-hot-data-tiering",
			Title:    "Enable Hot Data Tiering",
			Priority: "high",
			Action:   "Enable hot data tiering to place frequently accessed data on flash devices",
			Reason:   "Hot data is not currently stored on the flash tier, leading to suboptimal performance",
		})
	}

	// 4. Fragmentation score too high → recommend rebalancing
	if s.HybridPoolEnabled && s.FragmentationScore > 0.7 {
		recs = append(recs, Recommendation{
			ID:       "rebalance-pool",
			Title:    "Rebalance Pool",
			Priority: "medium",
			Action:   "Run a pool rebalance operation to reduce fragmentation",
			Reason:   "Fragmentation score exceeds 0.7, indicating significant fragmentation",
		})
	}

	// 5. Pool utilization too high → recommend expansion
	if s.HybridPoolEnabled && s.PoolUtilizationPct > 85 {
		recs = append(recs, Recommendation{
			ID:       "expand-pool",
			Title:    "Expand Pool Capacity",
			Priority: "high",
			Action:   "Add additional storage devices or vdevs to expand pool capacity",
			Reason:   "Pool utilization exceeds 85%, approaching capacity limits",
		})
	}

	// 6. Auto-rebalance disabled and last rebalance is stale → recommend enabling auto-rebalance
	if s.HybridPoolEnabled && !s.AutoRebalance && s.LastRebalanceAge > 7*24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "enable-auto-rebalance",
			Title:    "Enable Auto-Rebalance",
			Priority: "low",
			Action:   "Enable automatic rebalance to maintain pool health over time",
			Reason:   "Auto-rebalance is disabled and last rebalance was more than 7 days ago",
		})
	}

	// 7. Flash wear level too high → recommend replacing flash devices
	if s.HybridPoolEnabled && s.FlashWearLevelPct > 80 {
		recs = append(recs, Recommendation{
			ID:       "replace-flash-devices",
			Title:    "Replace Flash Devices",
			Priority: "high",
			Action:   "Replace worn-out flash devices to maintain tier performance and reliability",
			Reason:   "Flash wear level exceeds 80%, indicating near end-of-life for flash devices",
		})
	}

	// 8. Flash temperature too high → recommend cooling
	if s.HybridPoolEnabled && s.FlashTemperatureC > 70 {
		recs = append(recs, Recommendation{
			ID:       "reduce-flash-temperature",
			Title:    "Reduce Flash Temperature",
			Priority: "high",
			Action:   "Improve cooling or airflow for flash devices to reduce temperature",
			Reason:   "Flash device temperature exceeds 70°C, which can reduce lifespan and cause throttling",
		})
	}

	// 9. No special device class configured → recommend setting one up
	if s.HybridPoolEnabled && !s.HasSpecialDeviceClass {
		recs = append(recs, Recommendation{
			ID:       "configure-special-device-class",
			Title:    "Configure Special Device Class",
			Priority: "low",
			Action:   "Configure a special device class for better tier management",
			Reason:   "No special device class is configured, limiting fine-grained tier control",
		})
	}

	// 10. Cold data not yet migrated → recommend running cold data migration
	if s.HybridPoolEnabled && !s.ColdDataMigrated {
		recs = append(recs, Recommendation{
			ID:       "migrate-cold-data",
			Title:    "Migrate Cold Data",
			Priority: "medium",
			Action:   "Run cold data migration to move infrequently accessed data to HDD tier",
			Reason:   "Cold data has not been migrated to the HDD tier, leaving flash capacity underutilized",
		})
	}

	// Sort by priority: high (0) < medium (1) < low (2)
	sort.SliceStable(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}