// Package costbenchmark provides cost benchmarking and comparison utilities.
// It compares the NAS system's TCO (Total Cost of Ownership) against competitors
// (Synology, TrueNAS, fnOS), benchmarks cloud storage costs, and maintains
// hardware cost baselines for budgeting and procurement decisions.
package costbenchmark

import (
	"math"
	"sort"
	"time"
)

// BenchmarkResult is the top-level result of a benchmark comparison.
type BenchmarkResult struct {
	// BenchmarkID is a unique identifier for this benchmark run.
	BenchmarkID string
	// GeneratedAt is when the benchmark was produced.
	GeneratedAt time.Time
	// Category is the benchmark category: "tco", "cloud", "hardware".
	Category string
	// OurSystem is the cost/value figure for our NAS system.
	OurSystem SystemCost
	// Competitors holds comparison data for each competitor.
	Competitors []CompetitorTCO
	// CloudComparisons holds cloud storage cost comparisons (optional).
	CloudComparisons []CloudCostCompare
	// Summary is a human-readable result summary.
	Summary string
	// Winner is the name of the most cost-effective option.
	Winner string
}

// SystemCost represents a cost breakdown for our own system.
type SystemCost struct {
	// HardwareCost is the upfront hardware investment in CNY.
	HardwareCost float64
	// ElectricityAnnual is the estimated annual electricity cost in CNY.
	ElectricityAnnual float64
	// MaintenanceAnnual is the annual maintenance cost in CNY.
	MaintenanceAnnual float64
	// SoftwareCost is the software/licence cost in CNY (0 for open-source).
	SoftwareCost float64
	// YearsInService is the planned service lifetime in years.
	YearsInService int
	// TotalTCO is the total cost of ownership over YearsInService.
	TotalTCO float64
	// MonthlyTCO is the amortised monthly cost.
	MonthlyTCO float64
}

// CompetitorTCO represents the TCO of a competing NAS product.
type CompetitorTCO struct {
	// Name is the competitor product name (e.g. "Synology DS923+").
	Name string
	// Brand is the manufacturer (e.g. "Synology", "TrueNAS", "fnOS").
	Brand string
	// HardwareCost is the upfront hardware purchase price in CNY.
	HardwareCost float64
	// ElectricityAnnual is the estimated annual electricity cost in CNY.
	ElectricityAnnual float64
	// MaintenanceAnnual is the annual maintenance/support cost in CNY.
	MaintenanceAnnual float64
	// SoftwareCost is the licensing cost over the service lifetime in CNY.
	SoftwareCost float64
	// YearsInService is the typical service lifetime in years.
	YearsInService int
	// TotalTCO is the total cost of ownership over YearsInService.
	TotalTCO float64
	// MonthlyTCO is the amortised monthly cost.
	MonthlyTCO float64
	// Notes holds additional context (e.g. "includes 3-year warranty").
	Notes string
}

// CloudCostCompare represents a cloud storage cost comparison.
type CloudCostCompare struct {
	// Provider is the cloud provider name (e.g. "AWS S3", "Alibaba OSS").
	Provider string
	// Tier is the storage tier (e.g. "Standard", "Infrequent Access", "Archive").
	Tier string
	// MonthlyCostForTB is the monthly cost for 1 TB at this tier in CNY.
	MonthlyCostForTB float64
	// EgressCostPerGB is the data egress cost per GB in CNY.
	EgressCostPerGB float64
	// FiveYearCostFor10TB is the projected 5-year cost for 10 TB in CNY.
	FiveYearCostFor10TB float64
	// EquivalentLocalTCO is our system's 5-year TCO for comparison.
	EquivalentLocalTCO float64
	// Verdict is "cheaper", "comparable", or "more-expensive" vs. local storage.
	Verdict string
}

// Engine is the benchmark engine.
type Engine struct {
	// OurSystem holds our NAS system's cost data.
	OurSystem SystemCost
	// knownCompetitors is a registry of competitor TCO data keyed by name.
	knownCompetitors map[string]CompetitorTCO
	// knownCloudCosts is a registry of cloud provider costs.
	knownCloudCosts map[string]CloudCostCompare
}

// NewEngine creates a benchmark engine with our system's cost data.
func NewEngine(our SystemCost) *Engine {
	return &Engine{
		OurSystem:        ourSystemTCO(our),
		knownCompetitors: defaultCompetitors(),
		knownCloudCosts:  defaultCloudCosts(),
	}
}

// ourSystemTCO recalculates TCO and monthly cost.
func ourSystemTCO(s SystemCost) SystemCost {
	if s.YearsInService <= 0 {
		s.YearsInService = 5
	}
	s.TotalTCO = s.HardwareCost + s.SoftwareCost +
		(s.ElectricityAnnual+s.MaintenanceAnnual)*float64(s.YearsInService)
	s.MonthlyTCO = s.TotalTCO / float64(s.YearsInService*12)
	return s
}

// competitorTCO recalculates TCO and monthly cost.
func competitorTCO(c CompetitorTCO) CompetitorTCO {
	if c.YearsInService <= 0 {
		c.YearsInService = 5
	}
	c.TotalTCO = c.HardwareCost + c.SoftwareCost +
		(c.ElectricityAnnual+c.MaintenanceAnnual)*float64(c.YearsInService)
	c.MonthlyTCO = c.TotalTCO / float64(c.YearsInService*12)
	return c
}

// defaultCompetitors returns pre-seeded competitor data (CNY, approximate).
func defaultCompetitors() map[string]CompetitorTCO {
	entries := []CompetitorTCO{
		{
			Name:              "Synology DS923+",
			Brand:             "Synology",
			HardwareCost:      6500,
			ElectricityAnnual: 600,
			MaintenanceAnnual: 400,
			SoftwareCost:      0, // DSM included
			YearsInService:    5,
			Notes:             "4-bay, AMD R1600, 2.5GbE",
		},
		{
			Name:              "TrueNAS Mini X+",
			Brand:             "TrueNAS",
			HardwareCost:      8500,
			ElectricityAnnual: 800,
			MaintenanceAnnual: 300,
			SoftwareCost:      0, // Community edition free; Enterprise optional
			YearsInService:    5,
			Notes:             "6-bay, Intel C2750, 10GbE option",
		},
		{
			Name:              "飞牛 fnOS (飞牛OS)",
			Brand:             "fnOS",
			HardwareCost:      5000, // typical whitebox build
			ElectricityAnnual: 500,
			MaintenanceAnnual: 200,
			SoftwareCost:      0, // free for personal use
			YearsInService:    5,
			Notes:             "国产 NAS OS, x86 whitebox",
		},
	}
	m := make(map[string]CompetitorTCO, len(entries))
	for _, e := range entries {
		m[e.Name] = competitorTCO(e)
	}
	return m
}

// defaultCloudCosts returns pre-seeded cloud provider data (CNY, approximate).
func defaultCloudCosts() map[string]CloudCostCompare {
	// All figures are approximate monthly costs per TB in CNY.
	entries := []CloudCostCompare{
		{Provider: "AWS S3", Tier: "Standard", MonthlyCostForTB: 15.5, EgressCostPerGB: 0.7},
		{Provider: "AWS S3", Tier: "Infrequent Access", MonthlyCostForTB: 6.5, EgressCostPerGB: 0.7},
		{Provider: "AWS S3", Tier: "Glacier Archive", MonthlyCostForTB: 3.0, EgressCostPerGB: 0.7},
		{Provider: "Alibaba OSS", Tier: "Standard", MonthlyCostForTB: 12.0, EgressCostPerGB: 0.5},
		{Provider: "Alibaba OSS", Tier: "IA", MonthlyCostForTB: 5.5, EgressCostPerGB: 0.5},
		{Provider: "Alibaba OSS", Tier: "Archive", MonthlyCostForTB: 2.0, EgressCostPerGB: 0.5},
		{Provider: "Backblaze B2", Tier: "Standard", MonthlyCostForTB: 4.0, EgressCostPerGB: 0.0},
	}
	m := make(map[string]CloudCostCompare, len(entries))
	for _, e := range entries {
		e.FiveYearCostFor10TB = e.MonthlyCostForTB * 10 * 60 // 60 months
		m[e.Provider+"-"+e.Tier] = e
	}
	return m
}

// CompareWithCompetitor compares our system against one or more named competitors.
// Returns a BenchmarkResult with full TCO comparison.
func (e *Engine) CompareWithCompetitor(competitorNames ...string) BenchmarkResult {
	result := BenchmarkResult{
		BenchmarkID: "tco-comparison",
		GeneratedAt: time.Now(),
		Category:    "tco",
		OurSystem:   e.OurSystem,
	}

	allCosts := []float64{e.OurSystem.MonthlyTCO}
	winner := "Our System"
	lowestMonthly := e.OurSystem.MonthlyTCO

	for _, name := range competitorNames {
		c, ok := e.knownCompetitors[name]
		if !ok {
			continue
		}
		c = competitorTCO(c)
		result.Competitors = append(result.Competitors, c)
		allCosts = append(allCosts, c.MonthlyTCO)
		if c.MonthlyTCO < lowestMonthly {
			lowestMonthly = c.MonthlyTCO
			winner = c.Name
		}
	}

	result.Winner = winner

	// Build summary
	var sb string
	if e.OurSystem.MonthlyTCO <= lowestMonthly+0.01 {
		sb = "Our system has the lowest monthly TCO among compared options."
	} else {
		sb = winner + " has the lowest monthly TCO. Our system is " +
			formatPct(e.OurSystem.MonthlyTCO/lowestMonthly-1) + " more expensive per month."
	}
	result.Summary = sb

	return result
}

// CompareCloudCost compares our system's TCO against cloud storage costs
// for storing equivalent capacity over a 5-year horizon.
// capacityTB is the storage capacity to compare; egressPerMonthTB is the
// estimated monthly egress in TB (0 for archive-only workloads).
func (e *Engine) CompareCloudCost(capacityTB, egressPerMonthTB float64) BenchmarkResult {
	result := BenchmarkResult{
		BenchmarkID: "cloud-comparison",
		GeneratedAt: time.Now(),
		Category:    "cloud",
		OurSystem:   e.OurSystem,
	}

	local5yr := e.OurSystem.TotalTCO // already 5-year by default
	if e.OurSystem.YearsInService != 5 {
		// Normalise to 5 years
		local5yr = e.OurSystem.MonthlyTCO * 60
	}

	egressGB := egressPerMonthTB * 1024 // TB → GB

	for _, cc := range e.knownCloudCosts {
		monthly := cc.MonthlyCostForTB * capacityTB
		egressMonthly := cc.EgressCostPerGB * egressGB
		fiveYear := (monthly + egressMonthly) * 60
		cc.FiveYearCostFor10TB = cc.MonthlyCostForTB * 10 * 60
		cc.EquivalentLocalTCO = local5yr

		verdict := "comparable"
		diff := local5yr - fiveYear
		threshold := 0.1 * math.Abs(fiveYear)
		if diff > threshold {
			verdict = "cheaper" // local is cheaper
		} else if diff < -threshold {
			verdict = "more-expensive" // local is more expensive
		}
		cc.Verdict = verdict

		result.CloudComparisons = append(result.CloudComparisons, cc)
	}

	// Sort cloud comparisons by 5-year cost ascending
	sort.Slice(result.CloudComparisons, func(i, j int) bool {
		return result.CloudComparisons[i].FiveYearCostFor10TB < result.CloudComparisons[j].FiveYearCostFor10TB
	})

	cloud := result.CloudComparisons

	winner := "Local Storage"
	lowestCost := local5yr
	if len(cloud) > 0 && cloud[0].FiveYearCostFor10TB < lowestCost {
		lowestCost = cloud[0].FiveYearCostFor10TB
		winner = cloud[0].Provider + " " + cloud[0].Tier
	}
	result.Winner = winner

	result.Summary = "5-year cost comparison for " +
		formatTB(capacityTB) + " capacity. Local storage 5-year TCO: " +
		formatCNY(local5yr) + ". Cheapest cloud option: " +
		formatCNY(lowestCost) + " (" + winner + ")."

	return result
}

// GetBenchmark returns a combined benchmark result covering TCO, cloud, and
// hardware dimensions.  It is a convenience wrapper that calls CompareWithCompetitor
// and CompareCloudCost, then merges the results.
func (e *Engine) GetBenchmark(capacityTB, egressPerMonthTB float64, competitorNames ...string) BenchmarkResult {
	tcoResult := e.CompareWithCompetitor(competitorNames...)
	cloudResult := e.CompareCloudCost(capacityTB, egressPerMonthTB)

	combined := BenchmarkResult{
		BenchmarkID:      "combined-benchmark",
		GeneratedAt:      time.Now(),
		Category:         "combined",
		OurSystem:        e.OurSystem,
		Competitors:      tcoResult.Competitors,
		CloudComparisons: cloudResult.CloudComparisons,
	}

	// Determine overall winner: lowest monthly cost across all options.
	lowestMonthly := e.OurSystem.MonthlyTCO
	overallWinner := "Our System"

	for _, c := range tcoResult.Competitors {
		if c.MonthlyTCO < lowestMonthly {
			lowestMonthly = c.MonthlyTCO
			overallWinner = c.Name
		}
	}

	// Compare against cheapest cloud (normalised monthly)
	if len(cloudResult.CloudComparisons) > 0 {
		cheapestCloudMonthly := cloudResult.CloudComparisons[0].FiveYearCostFor10TB / 60 // approx monthly
		if cheapestCloudMonthly < lowestMonthly {
			lowestMonthly = cheapestCloudMonthly
			overallWinner = cloudResult.CloudComparisons[0].Provider + " " +
				cloudResult.CloudComparisons[0].Tier
		}
	}

	combined.Winner = overallWinner
	combined.Summary = "Combined benchmark winner: " + overallWinner +
		" (lowest monthly cost: " + formatCNY(lowestMonthly) + ")."

	return combined
}

// AddCompetitor registers or replaces a competitor in the engine's registry.
func (e *Engine) AddCompetitor(c CompetitorTCO) {
	e.knownCompetitors[c.Name] = competitorTCO(c)
}

// AddCloudCost registers or replaces a cloud cost entry in the engine's registry.
func (e *Engine) AddCloudCost(cc CloudCostCompare) {
	key := cc.Provider + "-" + cc.Tier
	e.knownCloudCosts[key] = cc
}

// ListCompetitors returns all registered competitor names.
func (e *Engine) ListCompetitors() []string {
	names := make([]string, 0, len(e.knownCompetitors))
	for name := range e.knownCompetitors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListCloudCosts returns all registered cloud provider+tier keys.
func (e *Engine) ListCloudCosts() []string {
	keys := make([]string, 0, len(e.knownCloudCosts))
	for k := range e.knownCloudCosts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- formatting helpers ---

// formatPct formats a fraction as a percentage string like "+12.3%" or "-5.0%".
func formatPct(frac float64) string {
	sign := "+"
	if frac < 0 {
		sign = "-"
		frac = -frac
	}
	return sign + formatFloat(frac*100, 1) + "%"
}

// formatCNY formats a CNY amount with thousands separators and 0 decimal places.
func formatCNY(v float64) string {
	return "\u00a5" + formatFloat(v, 0)
}

// formatTB formats a capacity in TB.
func formatTB(tb float64) string {
	if tb == math.Floor(tb) {
		return formatFloat(tb, 0) + "TB"
	}
	return formatFloat(tb, 1) + "TB"
}

// formatFloat is a minimal float formatter that avoids importing strconv.
func formatFloat(v float64, decimals int) string {
	mul := 1.0
	for i := 0; i < decimals; i++ {
		mul *= 10
	}
	rounded := math.Round(v*mul) / mul
	if decimals == 0 {
		return itoa(int64(rounded))
	}
	intPart := int64(rounded)
	fracPart := int64(math.Round((rounded - float64(intPart)) * mul))
	if fracPart < 0 {
		fracPart = -fracPart
	}
	return itoa(intPart) + "." + itoaPad(int64(fracPart), decimals)
}

// itoa converts an int64 to its decimal string representation.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// itoaPad converts an int64 to a zero-padded decimal string of the given width.
func itoaPad(n int64, width int) string {
	s := itoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}