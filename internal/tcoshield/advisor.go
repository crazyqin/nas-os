package tcoshield

import "sort"

// Signal contains all cost and operational parameters needed for TCO analysis.
type Signal struct {
	HardwareCostUSD           float64
	SoftwareCostUSD           float64
	PowerCostPerYearUSD       float64
	CoolingCostPerYearUSD     float64
	MaintenanceCostPerYearUSD float64
	ReplacementCostPerYearUSD float64
	DowntimeCostPerYearUSD    float64
	YearsInService           int
	TotalCapacityTB           float64
	UsedCapacityTB            float64
	StaffHoursPerWeek         float64
	StaffHourlyRateUSD        float64
	CloudEquivalentCostPerYearUSD float64
	HasWarranty              bool
	WarrantyYearsLeft         int
}

// Recommendation is a single actionable suggestion produced by the TCO analysis.
type Recommendation struct {
	ID       string
	Title    string
	Priority string
	Action   string
	Reason   string
}

// priorityRank returns a numeric rank for a priority string; lower = more urgent.
func priorityRank(p string) int {
	switch p {
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

const tcoWARN = "⚠️"
const tcoINFO = "ℹ️"

// Analyze evaluates the given Signal and returns a prioritized list of recommendations.
func Analyze(s Signal) []Recommendation {
	var recs []Recommendation

	if s.YearsInService == 0 {
		return recs
	}

	// Annual hardware amortization
	hardwarePerYear := s.HardwareCostUSD / float64(s.YearsInService)

	staffCostPerYear := s.StaffHoursPerWeek * s.StaffHourlyRateUSD * 52

	annualTCO := hardwarePerYear +
		s.SoftwareCostUSD +
		s.PowerCostPerYearUSD +
		s.CoolingCostPerYearUSD +
		s.MaintenanceCostPerYearUSD +
		s.ReplacementCostPerYearUSD +
		s.DowntimeCostPerYearUSD +
		staffCostPerYear

	// 1. Old hardware without warranty → extend warranty or upgrade
	if s.YearsInService > 5 && !s.HasWarranty {
		recs = append(recs, Recommendation{
			ID:       "warranty-upgrade",
			Title:    "Extend Warranty or Upgrade Hardware",
			Priority: "high",
			Action:   "Purchase extended warranty or plan a hardware refresh to avoid unplanned failure costs.",
			Reason:   tcoWARN + " Hardware has been in service for >5 years and is out of warranty.",
		})
	}

	// 2. Power cost exceeding 15% of hardware cost → energy optimization
	if s.PowerCostPerYearUSD > s.HardwareCostUSD*0.15 {
		recs = append(recs, Recommendation{
			ID:       "energy-optimization",
			Title:    "Optimize Energy Consumption",
			Priority: "medium",
			Action:   "Review PSU efficiency, enable low-power modes, and consolidate idle drives.",
			Reason:   tcoWARN + " Annual power cost exceeds 15% of hardware cost.",
		})
	}

	// 3. Storage utilization below 30% → underutilized capacity
	if s.TotalCapacityTB > 0 && s.UsedCapacityTB/s.TotalCapacityTB < 0.3 {
		recs = append(recs, Recommendation{
			ID:       "capacity-underutilized",
			Title:    "Storage Utilization Below 30%",
			Priority: "low",
			Action:   "Reallocate unused capacity, archive cold data, or downsize the storage footprint.",
			Reason:   tcoINFO + " Only " + formatPct(s.UsedCapacityTB, s.TotalCapacityTB) + " of total capacity is in use.",
		})
	}

	// 4. Staff cost exceeds maintenance cost → automation
	if staffCostPerYear > s.MaintenanceCostPerYearUSD {
		recs = append(recs, Recommendation{
			ID:       "automation-ops",
			Title:    "Automate Maintenance Operations",
			Priority: "medium",
			Action:   "Introduce scripted provisioning, automated monitoring, and self-healing routines.",
			Reason:   tcoWARN + " Annual staff cost ($" + formatFloat(staffCostPerYear) + ") exceeds maintenance cost ($" + formatFloat(s.MaintenanceCostPerYearUSD) + ").",
		})
	}

	// 5. Cloud equivalent cheaper than 70% of on-prem TCO → consider cloud migration
	if s.CloudEquivalentCostPerYearUSD > 0 && s.CloudEquivalentCostPerYearUSD < annualTCO*0.7 {
		recs = append(recs, Recommendation{
			ID:       "cloud-comparison",
			Title:    "On-Premises More Expensive Than Cloud",
			Priority: "high",
			Action:   "Perform a detailed cloud-migration feasibility study for workloads that do not require low-latency local access.",
			Reason:   tcoWARN + " Cloud equivalent ($" + formatFloat(s.CloudEquivalentCostPerYearUSD) + "/yr) is less than 70% of on-prem TCO ($" + formatFloat(annualTCO) + "/yr).",
		})
	}

	// 6. Cooling cost exceeds 50% of power cost → optimize cooling
	if s.CoolingCostPerYearUSD > s.PowerCostPerYearUSD*0.5 {
		recs = append(recs, Recommendation{
			ID:       "cooling-optimization",
			Title:    "Optimize Cooling Efficiency",
			Priority: "medium",
			Action:   "Improve airflow, raise inlet temperatures, or deploy hot/cold aisle containment.",
			Reason:   tcoWARN + " Annual cooling cost exceeds 50% of power cost.",
		})
	}

	// 7. Downtime cost exceeds maintenance cost → improve availability
	if s.DowntimeCostPerYearUSD > s.MaintenanceCostPerYearUSD {
		recs = append(recs, Recommendation{
			ID:       "availability-improvement",
			Title:    "Improve System Availability",
			Priority: "critical",
			Action:   "Invest in redundancy (replicated nodes, UPS, HA clustering) to reduce downtime losses.",
			Reason:   tcoWARN + " Annual downtime cost ($" + formatFloat(s.DowntimeCostPerYearUSD) + ") exceeds maintenance cost ($" + formatFloat(s.MaintenanceCostPerYearUSD) + ").",
		})
	}

	// 8. Warranty expiring within a year on hardware >3 years old → plan replacement
	if s.WarrantyYearsLeft < 1 && s.YearsInService > 3 {
		recs = append(recs, Recommendation{
			ID:       "replacement-planning",
			Title:    "Plan Hardware Replacement",
			Priority: "high",
			Action:   "Budget for a replacement cycle and begin evaluating current-generation alternatives.",
			Reason:   tcoWARN + " Warranty expires within 1 year on hardware that has been in service >3 years.",
		})
	}

	// Sort recommendations by priority rank (critical → low)
	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}

// formatPct returns a percentage string for used/total.
func formatPct(used, total float64) string {
	if total == 0 {
		return "0.0%"
	}
	return formatFloat(used/total*100) + "%"
}

// formatFloat formats a float to a string with 2 decimal places.
func formatFloat(f float64) string {
	// Simple formatting without fmt to keep imports minimal
	// but fmt is cleaner — use it via a helper
	return floatToStr(f, 2)
}

// floatToStr converts a float to a string with n decimal places.
func floatToStr(f float64, n int) string {
	if n < 0 {
		n = 0
	}
	// Calculate scaling factor
	scale := 1.0
	for i := 0; i < n; i++ {
		scale *= 10
	}
	rounded := float64(int64(f*scale + 0.5)) / scale
	intPart := int64(rounded)
	frac := rounded - float64(intPart)
	// Build fractional part
	fracStr := ""
	for i := 0; i < n; i++ {
		frac *= 10
		digit := int64(frac)
		fracStr += string(rune('0' + digit))
		frac -= float64(digit)
	}
	if n == 0 {
		return intToStr(intPart)
	}
	return intToStr(intPart) + "." + fracStr
}

// intToStr converts an int64 to string without using strconv.
func intToStr(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}