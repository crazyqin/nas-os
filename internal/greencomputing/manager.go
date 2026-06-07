package greencomputing

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Manager manages green computing optimization
type Manager struct {
	mu            sync.RWMutex
	readings      []*EnergyReading
	strategies    map[string]*SleepStrategy
	config        *Config
	latestReading *EnergyReading
}

// Config represents green computing configuration
type Config struct {
	Enabled            bool    `json:"enabled"`
	MonitoringInterval int     `json:"monitoring_interval"`
	GridIntensity      float64 `json:"grid_intensity"`
	SolarIntensity     float64 `json:"solar_intensity"`
	WindIntensity      float64 `json:"wind_intensity"`
	CarbonOffsetCost   float64 `json:"carbon_offset_cost"`
	TreeAbsorptionKg   float64 `json:"tree_absorption_kg"`
	ElectricityRate    float64 `json:"electricity_rate"`
	Currency           string  `json:"currency"`
	IdleThreshold      int     `json:"idle_threshold"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:            true,
		MonitoringInterval: 60,
		GridIntensity:      DefaultGridIntensity,
		SolarIntensity:     DefaultSolarIntensity,
		WindIntensity:      DefaultWindIntensity,
		CarbonOffsetCost:   DefaultCarbonOffsetCost,
		TreeAbsorptionKg:   DefaultTreeAbsorptionKg,
		ElectricityRate:    0.12,
		Currency:           "USD",
		IdleThreshold:      30,
	}
}

// NewManager creates a new green computing manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	return &Manager{
		readings:   make([]*EnergyReading, 0),
		strategies: make(map[string]*SleepStrategy),
		config:     config,
	}
}

// RecordReading records an energy reading
func (m *Manager) RecordReading(reading *EnergyReading) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if reading.Timestamp.IsZero() {
		reading.Timestamp = time.Now()
	}

	m.readings = append(m.readings, reading)
	m.latestReading = reading
}

// GetLatestReading returns the latest energy reading
func (m *Manager) GetLatestReading() *EnergyReading {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latestReading
}

// GetReadings returns readings within a time range
func (m *Manager) GetReadings(start, end time.Time) []*EnergyReading {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*EnergyReading, 0)
	for _, r := range m.readings {
		if r.Timestamp.After(start) && r.Timestamp.Before(end) {
			result = append(result, r)
		}
	}
	return result
}

// CalculateFootprint calculates carbon footprint for a period
func (m *Manager) CalculateFootprint(start, end time.Time) *CarbonFootprint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	footprint := &CarbonFootprint{
		Period:    fmt.Sprintf("%s to %s", start.Format("2006-01-02"), end.Format("2006-01-02")),
		Breakdown: make(map[string]float64),
	}

	var totalWatts float64
	for _, r := range m.readings {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}

		totalWatts += r.TotalWatts
		footprint.Breakdown["cpu"] += r.CPUWatts
		footprint.Breakdown["disk"] += r.DiskWatts
		footprint.Breakdown["network"] += r.NetworkWatts
		footprint.Breakdown["fan"] += r.FanWatts
		footprint.Breakdown["other"] += r.OtherWatts
	}

	// Convert watts to kWh (assuming readings are per-minute samples)
	hours := end.Sub(start).Hours()
	if hours > 0 {
		footprint.EnergyKWh = (totalWatts / 1000.0) * (hours / 24.0)
	}

	// Calculate carbon based on source
	footprint.CarbonG = footprint.EnergyKWh * m.config.GridIntensity
	footprint.CarbonKg = footprint.CarbonG / 1000.0
	footprint.OffsetCost = footprint.CarbonKg * m.config.CarbonOffsetCost
	footprint.TreesNeeded = math.Ceil(footprint.CarbonKg / m.config.TreeAbsorptionKg)

	return footprint
}

// GetDailyFootprint returns today's carbon footprint
func (m *Manager) GetDailyFootprint() *CarbonFootprint {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	return m.CalculateFootprint(start, end)
}

// GetWeeklyFootprint returns this week's carbon footprint
func (m *Manager) GetWeeklyFootprint() *CarbonFootprint {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 7)
	return m.CalculateFootprint(start, end)
}

// GetMonthlyFootprint returns this month's carbon footprint
func (m *Manager) GetMonthlyFootprint() *CarbonFootprint {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	return m.CalculateFootprint(start, end)
}

// CreateStrategy creates a new sleep strategy
func (m *Manager) CreateStrategy(name, description string, idleThreshold time.Duration) *SleepStrategy {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	strategy := &SleepStrategy{
		ID:            fmt.Sprintf("strategy-%d", now.UnixNano()),
		Name:          name,
		Description:   description,
		IdleThreshold: idleThreshold,
		DiskSpindown:  true,
		CPUGovernor:   "powersave",
		LEDEnabled:    false,
		WakeOnLAN:     true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	m.strategies[strategy.ID] = strategy
	return strategy
}

// GetStrategy returns a strategy by ID
func (m *Manager) GetStrategy(id string) (*SleepStrategy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.strategies[id]
	return s, ok
}

// ListStrategies returns all strategies
func (m *Manager) ListStrategies() []*SleepStrategy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	strategies := make([]*SleepStrategy, 0, len(m.strategies))
	for _, s := range m.strategies {
		strategies = append(strategies, s)
	}
	return strategies
}

// UpdateStrategy updates a strategy
func (m *Manager) UpdateStrategy(id string, updates *SleepStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	strategy, ok := m.strategies[id]
	if !ok {
		return fmt.Errorf("strategy not found: %s", id)
	}

	if updates.Name != "" {
		strategy.Name = updates.Name
	}
	if updates.Description != "" {
		strategy.Description = updates.Description
	}
	if updates.IdleThreshold > 0 {
		strategy.IdleThreshold = updates.IdleThreshold
	}
	if updates.CPUGovernor != "" {
		strategy.CPUGovernor = updates.CPUGovernor
	}
	strategy.DiskSpindown = updates.DiskSpindown
	strategy.LEDEnabled = updates.LEDEnabled
	strategy.WakeOnLAN = updates.WakeOnLAN
	if updates.ScheduledSleep != "" {
		strategy.ScheduledSleep = updates.ScheduledSleep
	}
	if updates.ScheduledWake != "" {
		strategy.ScheduledWake = updates.ScheduledWake
	}
	strategy.UpdatedAt = time.Now()

	return nil
}

// ActivateStrategy activates a strategy and deactivates others
func (m *Manager) ActivateStrategy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	strategy, ok := m.strategies[id]
	if !ok {
		return fmt.Errorf("strategy not found: %s", id)
	}

	for _, s := range m.strategies {
		s.IsActive = false
	}
	strategy.IsActive = true
	return nil
}

// DeleteStrategy deletes a strategy
func (m *Manager) DeleteStrategy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.strategies[id]; !ok {
		return fmt.Errorf("strategy not found: %s", id)
	}
	delete(m.strategies, id)
	return nil
}

// GenerateEfficiencyReport generates an efficiency report
func (m *Manager) GenerateEfficiencyReport(period string) *EfficiencyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &EfficiencyReport{
		Period:      period,
		GeneratedAt: time.Now(),
	}

	var start, end time.Time
	now := time.Now()

	switch period {
	case "daily":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 1)
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 7)
	case "monthly":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0)
	default:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 1)
	}

	var totalWatts float64
	var count int
	var maxWatts float64
	var minWatts float64 = 999999

	for _, r := range m.readings {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		totalWatts += r.TotalWatts
		count++
		if r.TotalWatts > maxWatts {
			maxWatts = r.TotalWatts
		}
		if r.TotalWatts < minWatts {
			minWatts = r.TotalWatts
		}
	}

	if count > 0 {
		report.AvgPowerWatts = totalWatts / float64(count)
		report.PeakPowerWatts = maxWatts
		report.MinPowerWatts = minWatts
		report.TotalEnergyKWh = (totalWatts / 1000.0) * (float64(count) / 60.0) // Assuming per-minute readings
	}

	report.CarbonKg = report.TotalEnergyKWh * m.config.GridIntensity / 1000.0
	report.CostEstimate = report.TotalEnergyKWh * m.config.ElectricityRate

	// Calculate efficiency score (0-100)
	report.EfficiencyScore = m.calculateEfficiencyScore(report)

	// Generate recommendations
	report.Recommendations = m.generateRecommendations(report)

	// Generate trends
	report.Trends = m.generateTrends(period)

	return report
}

// calculateEfficiencyScore calculates an efficiency score
func (m *Manager) calculateEfficiencyScore(report *EfficiencyReport) float64 {
	score := 100.0

	// Penalize for high average power
	if report.AvgPowerWatts > 100 {
		score -= 20
	} else if report.AvgPowerWatts > 50 {
		score -= 10
	}

	// Penalize for high peak power
	if report.PeakPowerWatts > 200 {
		score -= 15
	} else if report.PeakPowerWatts > 100 {
		score -= 5
	}

	// Bonus for low carbon
	if report.CarbonKg < 1 {
		score += 10
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// generateRecommendations generates optimization recommendations
func (m *Manager) generateRecommendations(report *EfficiencyReport) []*Recommendation {
	recs := make([]*Recommendation, 0)

	if report.AvgPowerWatts > 50 {
		recs = append(recs, &Recommendation{
			ID:           "rec-idle-power",
			Title:        "降低空闲功耗",
			Description:  "当前平均功耗较高，建议启用磁盘休眠和CPU节能模式",
			Category:     "power",
			Priority:     1,
			SavingsKWh:   report.TotalEnergyKWh * 0.2,
			SavingsKg:    report.CarbonKg * 0.2,
			SavingsCost:  report.CostEstimate * 0.2,
			EstimatedROI: "立即",
			CreatedAt:    time.Now(),
		})
	}

	if report.PeakPowerWatts > 150 {
		recs = append(recs, &Recommendation{
			ID:           "rec-peak-shaving",
			Title:        "削峰填谷",
			Description:  "峰值功耗过高，建议将高负载任务调度到非高峰时段",
			Category:     "scheduling",
			Priority:     2,
			SavingsKWh:   report.TotalEnergyKWh * 0.1,
			SavingsKg:    report.CarbonKg * 0.1,
			SavingsCost:  report.CostEstimate * 0.1,
			EstimatedROI: "1周",
			CreatedAt:    time.Now(),
		})
	}

	recs = append(recs, &Recommendation{
		ID:           "rec-smart-sleep",
		Title:        "启用智能休眠",
		Description:  "配置智能休眠策略，在空闲时自动降低功耗",
		Category:     "sleep",
		Priority:     3,
		SavingsKWh:   report.TotalEnergyKWh * 0.15,
		SavingsKg:    report.CarbonKg * 0.15,
		SavingsCost:  report.CostEstimate * 0.15,
		EstimatedROI: "2周",
		CreatedAt:    time.Now(),
	})

	return recs
}

// generateTrends generates efficiency trends
func (m *Manager) generateTrends(period string) *EfficiencyTrends {
	trends := &EfficiencyTrends{
		EnergyTrend:     "stable",
		CarbonTrend:     "stable",
		EfficiencyTrend: "stable",
		WeekOverWeek:    0,
		MonthOverMonth:  0,
	}

	// Simplified trend analysis
	if len(m.readings) > 10 {
		recent := m.readings[len(m.readings)-5:]
		older := m.readings[len(m.readings)-10 : len(m.readings)-5]

		var recentAvg, olderAvg float64
		for _, r := range recent {
			recentAvg += r.TotalWatts
		}
		for _, r := range older {
			olderAvg += r.TotalWatts
		}
		recentAvg /= 5
		olderAvg /= 5

		change := ((recentAvg - olderAvg) / olderAvg) * 100
		trends.WeekOverWeek = change

		if change > 5 {
			trends.EnergyTrend = "increasing"
			trends.CarbonTrend = "increasing"
		} else if change < -5 {
			trends.EnergyTrend = "decreasing"
			trends.CarbonTrend = "decreasing"
			trends.EfficiencyTrend = "improving"
		}
	}

	return trends
}

// GetGreenScore calculates the overall green computing score
func (m *Manager) GetGreenScore() *GreenScore {
	m.mu.RLock()
	defer m.mu.RUnlock()

	score := &GreenScore{
		Breakdown: make(map[string]float64),
		UpdatedAt: time.Now(),
	}

	// Calculate based on various factors
	efficiencyScore := 80.0
	sleepScore := 50.0
	carbonScore := 70.0

	if len(m.strategies) > 0 {
		sleepScore = 80.0
	}

	// Check if any strategy is active
	for _, s := range m.strategies {
		if s.IsActive {
			sleepScore = 95.0
			break
		}
	}

	score.Breakdown["efficiency"] = efficiencyScore
	score.Breakdown["sleep_management"] = sleepScore
	score.Breakdown["carbon_footprint"] = carbonScore
	score.Breakdown["renewable_energy"] = 60.0

	score.Score = (efficiencyScore + sleepScore + carbonScore + 60.0) / 4.0

	switch {
	case score.Score >= 90:
		score.Grade = "A+"
	case score.Score >= 80:
		score.Grade = "A"
	case score.Score >= 70:
		score.Grade = "B"
	case score.Score >= 60:
		score.Grade = "C"
	default:
		score.Grade = "D"
	}

	return score
}
