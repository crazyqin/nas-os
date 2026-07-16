// Package tcodashboard implements a 5-year Total Cost of Ownership visualization
// dashboard inspired by Synology TCO Calculator and TrueNAS cost intelligence.
package tcodashboard

import (
	"fmt"
	"sort"
)

// CostCategory indicates the type of cost.
type CostCategory string

const (
	CostHardware    CostCategory = "hardware"
	CostStorage     CostCategory = "storage"      // disks, SSDs, expansion
	CostPower       CostCategory = "power"         // electricity
	CostNetwork     CostCategory = "network"      // switches, cables
	CostSoftware    CostCategory = "software"     // licenses, subscriptions
	CostMaintenance CostCategory = "maintenance"  // support, repairs
	CostCloud       CostCategory = "cloud"        // cloud sync, backup storage
	CostLabor       CostCategory = "labor"         // admin time
)

// CostEntry represents a single cost line item over time.
type CostEntry struct {
	Category    CostCategory `json:"category"`
	Label       string       `json:"label"`
	Year1Cost   float64      `json:"year1_cost"`
	Year2Cost   float64      `json:"year2_cost"`
	Year3Cost   float64      `json:"year3_cost"`
	Year4Cost   float64      `json:"year4_cost"`
	Year5Cost   float64      `json:"year5_cost"`
	OneTime     bool         `json:"one_time"`
	EscalationPct float64    `json:"escalation_pct"` // annual cost increase %
}

// HardwareProfile describes the NAS hardware configuration.
type HardwareProfile struct {
	DriveBays      int     `json:"drive_bays"`
	NVMeSlots      int     `json:"nvme_slots"`
	DriveCount     int     `json:"drive_count"`
	DriveCostEach  float64 `json:"drive_cost_each"`
	NVMeCount      int     `json:"nvme_count"`
	NVMeCostEach   float64 `json:"nvme_cost_each"`
	SystemCost     float64 `json:"system_cost"`     // NAS unit cost
	WattageIdle    int     `json:"wattage_idle"`
	WattageLoad    int     `json:"wattage_load"`
	ElectricityRate float64 `json:"electricity_rate"` // per kWh
	UptimeHoursPct  float64 `json:"uptime_hours_pct"` // % of day active
}

// CloudProfile describes cloud subscription costs.
type CloudProfile struct {
	CloudBackupGB    int     `json:"cloud_backup_gb"`
	CloudBackupPerGB  float64 `json:"cloud_backup_per_gb"`
	CloudSyncGB       int     `json:"cloud_sync_gb"`
	CloudSyncPerGB    float64 `json:"cloud_sync_per_gb"`
	CloudEgressPct    float64 `json:"cloud_egress_pct"` // % of data egress/month
}

// Signal aggregates TCO data for analysis.
type Signal struct {
	Hardware         HardwareProfile `json:"hardware"`
	Cloud           CloudProfile    `json:"cloud"`
	SoftwareLicense float64         `json:"software_license_annual"`
	MaintenancePct   float64         `json:"maintenance_pct"` // % of hardware cost/year
	LaborHoursWeek   float64         `json:"labor_hours_week"`
	LaborRateHourly  float64         `json:"labor_rate_hourly"`
	YearsProjection  int             `json:"years_projection"`
	CompareSynology  float64         `json:"compare_synology_5yr"` // Synology TCO for comparison
	CompareTrueNAS   float64         `json:"compare_truenas_5yr"`  // TrueNAS TCO for comparison
}

// YearlyBreakdown is the cost breakdown for a single year.
type YearlyBreakdown struct {
	Year          int                `json:"year"`
	Hardware      float64            `json:"hardware"`
	Storage       float64            `json:"storage"`
	Power         float64            `json:"power"`
	Network       float64            `json:"network"`
	Software      float64            `json:"software"`
	Maintenance   float64            `json:"maintenance"`
	Cloud         float64            `json:"cloud"`
	Labor         float64            `json:"labor"`
	Total         float64            `json:"total"`
	CumulativeTotal float64          `json:"cumulative_total"`
}

// Recommendation is an actionable cost optimization suggestion.
type Recommendation struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Priority     string `json:"priority"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	Saving5yr    float64 `json:"saving_5yr,omitempty"`
}

// Compute calculates the 5-year TCO breakdown.
func Compute(s Signal) []YearlyBreakdown {
	years := s.YearsProjection
	if years <= 0 {
		years = 5
	}

	breakdowns := make([]YearlyBreakdown, years)
	cumulative := 0.0

	// Storage costs (drives + NVMe) - one time year 1, replacement year 4
	driveCost := float64(s.Hardware.DriveCount) * s.Hardware.DriveCostEach
	nvmeCost := float64(s.Hardware.NVMeCount) * s.Hardware.NVMeCostEach
	totalStorageCost := driveCost + nvmeCost

	// Power cost annual
	avgWattage := float64(s.Hardware.WattageIdle+s.Hardware.WattageLoad) / 2
	annualPower := avgWattage * 24 * 365 * (s.Hardware.UptimeHoursPct / 100) / 1000 * s.Hardware.ElectricityRate

	// Cloud costs annual
	annualCloud := float64(s.Cloud.CloudBackupGB)*s.Cloud.CloudBackupPerGB*12 +
		float64(s.Cloud.CloudSyncGB)*s.Cloud.CloudSyncPerGB*12

	// Maintenance annual
	annualMaintenance := s.Hardware.SystemCost * (s.MaintenancePct / 100)

	// Labor annual
	annualLabor := s.LaborHoursWeek * 52 * s.LaborRateHourly

	for i := 0; i < years; i++ {
		year := i + 1
		bd := YearlyBreakdown{Year: year}

		// Hardware: one-time year 1
		if i == 0 {
			bd.Hardware = s.Hardware.SystemCost
			bd.Storage = totalStorageCost
		}
		// Drive replacement in year 4 (typical 3-5yr lifespan)
		if i == 3 {
			bd.Storage = driveCost * 0.5 // partial replacement
		}

		// Power with 3% annual increase
		bd.Power = annualPower * pow(1.03, float64(i))

		// Network: minor year 1
		if i == 0 {
			bd.Network = s.Hardware.SystemCost * 0.05
		}

		// Software license
		bd.Software = s.SoftwareLicense * pow(1.02, float64(i))

		// Maintenance (starts year 2)
		if i > 0 {
			bd.Maintenance = annualMaintenance * pow(1.02, float64(i-1))
		}

		// Cloud with 10% annual growth (data grows)
		bd.Cloud = annualCloud * pow(1.10, float64(i))

		// Labor with 3% annual increase
		bd.Labor = annualLabor * pow(1.03, float64(i))

		bd.Total = bd.Hardware + bd.Storage + bd.Power + bd.Network +
			bd.Software + bd.Maintenance + bd.Cloud + bd.Labor
		cumulative += bd.Total
		bd.CumulativeTotal = cumulative

		breakdowns[i] = bd
	}

	return breakdowns
}

// Analyze evaluates TCO and returns cost optimization recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	breakdowns := Compute(s)
	total5yr := 0.0
	if len(breakdowns) > 0 {
		total5yr = breakdowns[len(breakdowns)-1].CumulativeTotal
	}

	// Power cost high
	power5yr := 0.0
	for _, b := range breakdowns {
		power5yr += b.Power
	}
	if power5yr > total5yr*0.3 {
		recs = append(recs, Recommendation{
			ID:        "tco-reduce-power",
			Title:     "Power costs exceeding 30% of TCO",
			Priority:  "high",
			Action:    "Enable power management scheduling, spin down idle disks, consider lower-wattage NAS",
			Reason:    fmt.Sprintf("5-year power cost %.0f is %.1f%% of total TCO", power5yr, power5yr/total5yr*100),
			Saving5yr: power5yr * 0.25,
		})
	}

	// Cloud costs growing fast
	cloud5yr := 0.0
	for _, b := range breakdowns {
		cloud5yr += b.Cloud
	}
	if cloud5yr > total5yr*0.2 {
		recs = append(recs, Recommendation{
			ID:        "tco-cloud-optimization",
			Title:     "Cloud storage costs growing rapidly",
			Priority:  "high",
			Action:    "Implement lifecycle policies, move cold data to cheaper tiers, reduce egress with dedup",
			Reason:    fmt.Sprintf("5-year cloud cost %.0f with 10%% annual growth; lifecycle tiering can save 30-50%%", cloud5yr),
			Saving5yr: cloud5yr * 0.35,
		})
	}

	// Labor cost high
	labor5yr := 0.0
	for _, b := range breakdowns {
		labor5yr += b.Labor
	}
	if labor5yr > total5yr*0.25 {
		recs = append(recs, Recommendation{
			ID:        "tco-automate-labor",
			Title:     "Labor costs high - automate administration",
			Priority:  "medium",
			Action:    "Enable automated snapshots, replication, and alerting to reduce manual admin time",
			Reason:    fmt.Sprintf("5-year labor cost %.0f; automation can reduce admin time by 40-60%%", labor5yr),
			Saving5yr: labor5yr * 0.5,
		})
	}

	// Competitive comparison
	if s.CompareSynology > 0 && total5yr < s.CompareSynology {
		saving := s.CompareSynology - total5yr
		recs = append(recs, Recommendation{
			ID:        "tco-vs-synology",
			Title:     "NAS-OS significantly cheaper than Synology",
			Priority:  "info",
			Action:    "Highlight TCO advantage in procurement decisions",
			Reason:    fmt.Sprintf("5-year TCO %.0f vs Synology %.0f saves %.0f (%.1f%% less)", total5yr, s.CompareSynology, saving, saving/s.CompareSynology*100),
			Saving5yr: saving,
		})
	}

	if s.CompareTrueNAS > 0 && total5yr < s.CompareTrueNAS {
		saving := s.CompareTrueNAS - total5yr
		recs = append(recs, Recommendation{
			ID:        "tco-vs-truenas",
			Title:     "NAS-OS TCO advantage over TrueNAS",
			Priority:  "info",
			Action:    "Use TCO comparison for budget justification",
			Reason:    fmt.Sprintf("5-year TCO %.0f vs TrueNAS %.0f saves %.0f (%.1f%% less)", total5yr, s.CompareTrueNAS, saving, saving/s.CompareTrueNAS*100),
			Saving5yr: saving,
		})
	}

	// NVMe overspend
	nvmeCost := float64(s.Hardware.NVMeCount) * s.Hardware.NVMeCostEach
	if nvmeCost > s.Hardware.SystemCost*0.5 && s.Hardware.NVMeCount > 2 {
		recs = append(recs, Recommendation{
			ID:        "tco-nvme-overspend",
			Title:     "NVMe cache investment disproportionate to system cost",
			Priority:  "medium",
			Action:    "Evaluate if all NVMe slots are necessary; consider hybrid SSD approach",
			Reason:    fmt.Sprintf("NVMe investment %.0f is %.0f%% of system cost; diminishing returns above 40%%", nvmeCost, nvmeCost/s.Hardware.SystemCost*100),
			Saving5yr: nvmeCost * 0.4,
		})
	}

	// Drive replacement planning
	if len(breakdowns) >= 4 && breakdowns[3].Storage > 0 {
		recs = append(recs, Recommendation{
			ID:       "tco-drive-replacement",
			Title:    "Plan for drive replacement in year 4",
			Priority: "medium",
			Action:   "Budget for partial drive replacement in year 4 based on 3-5yr lifespan",
			Reason:   "Drives typically need replacement after 3-5 years; budget ahead to avoid emergency purchases",
		})
	}

	// No maintenance budget
	if s.MaintenancePct < 5 {
		recs = append(recs, Recommendation{
			ID:       "tco-maint-budget",
			Title:    "Maintenance budget too low",
			Priority: "low",
			Action:   "Allocate at least 8-12% of hardware cost annually for maintenance and support",
			Reason:   fmt.Sprintf("Maintenance at %.0f%% of hardware is below industry recommendation of 8-12%%", s.MaintenancePct),
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
	case "info":
		return 0
	default:
		return 0
	}
}
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}
