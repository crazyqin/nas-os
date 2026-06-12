package energyai

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// EnergyRecord represents a power consumption measurement
type EnergyRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	Watts        float64   `json:"watts"`
	Voltage      float64   `json:"voltage"`
	Current      float64   `json:"current"`
	Component    string    `json:"component"` // cpu, disk, gpu, fan, total
	Temperature  float64   `json:"temperature"`
}

// CarbonFootprint tracks carbon emissions
type CarbonFootprint struct {
	Date           time.Time `json:"date"`
	EnergyKWh      float64   `json:"energy_kwh"`
	CarbonKg       float64   `json:"carbon_kg"`
	GridIntensity  float64   `json:"grid_intensity"` // gCO2/kWh
	OffsetCredits  float64   `json:"offset_credits"`
	NetCarbonKg    float64   `json:"net_carbon_kg"`
}

// EnergyPrediction represents AI-predicted energy usage
type EnergyPrediction struct {
	Timestamp      time.Time `json:"timestamp"`
	PredictedWatts float64   `json:"predicted_watts"`
	Confidence     float64   `json:"confidence"`
	Factors        []string  `json:"factors"`
	Recommendation string    `json:"recommendation"`
}

// PowerProfile defines power management rules
type PowerProfile struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Rules        []PowerRule        `json:"rules"`
	Schedule     string             `json:"schedule"` // cron expression
	Enabled      bool               `json:"enabled"`
	SavingsEst   float64            `json:"savings_estimate_kwh"`
}

// PowerRule defines a single power management rule
type PowerRule struct {
	Component   string  `json:"component"`
	Condition   string  `json:"condition"` // always, schedule, threshold
	Threshold   float64 `json:"threshold"`
	Action      string  `json:"action"` // sleep, reduce_power, standby
	DelaySec    int     `json:"delay_sec"`
}

// EnergyStats aggregates energy statistics
type EnergyStats struct {
	TotalKWh       float64            `json:"total_kwh"`
	AvgWatts       float64            `json:"avg_watts"`
	PeakWatts      float64            `json:"peak_watts"`
	CostEstimate   float64            `json:"cost_estimate"`
	CarbonKg       float64            `json:"carbon_kg"`
	ByComponent    map[string]float64 `json:"by_component"`
	Period         string             `json:"period"`
}

// EnergyAI manages intelligent energy optimization
type EnergyAI struct {
	mu            sync.RWMutex
	records       []EnergyRecord
	carbon        []CarbonFootprint
	predictions   []EnergyPrediction
	profiles      map[string]*PowerProfile
	gridIntensity float64 // gCO2/kWh
电价PerKWh    float64
}

// NewEnergyAI creates a new energy AI manager
func NewEnergyAI() *EnergyAI {
	return &EnergyAI{
		records:       make([]EnergyRecord, 0),
		carbon:        make([]CarbonFootprint, 0),
		predictions:   make([]EnergyPrediction, 0),
		profiles:      make(map[string]*PowerProfile),
		gridIntensity: 500.0, // default global average
		电价PerKWh:    0.12,   // default $0.12/kWh
	}
}

// RecordEnergy records a power consumption measurement
func (ea *EnergyAI) RecordEnergy(ctx context.Context, record EnergyRecord) error {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	ea.records = append(ea.records, record)
	return nil
}

// PredictEnergy predicts future energy usage using simple moving average
func (ea *EnergyAI) PredictEnergy(ctx context.Context, horizon time.Duration) (*EnergyPrediction, error) {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	if len(ea.records) < 10 {
		return nil, fmt.Errorf("insufficient data for prediction (need at least 10 records)")
	}

	// Simple moving average prediction
	n := len(ea.records)
	windowSize := 24 // 24 hours
	if n < windowSize {
		windowSize = n
	}

	sum := 0.0
	for i := n - windowSize; i < n; i++ {
		sum += ea.records[i].Watts
	}
	avg := sum / float64(windowSize)

	// Calculate variance for confidence
	variance := 0.0
	for i := n - windowSize; i < n; i++ {
		diff := ea.records[i].Watts - avg
		variance += diff * diff
	}
	stddev := math.Sqrt(variance / float64(windowSize))

	confidence := math.Max(0, 1-stddev/avg)

	factors := []string{"historical_pattern"}
	recommendation := "normal_operation"

	if avg > 500 {
		recommendation = "consider_power_saving"
		factors = append(factors, "high_consumption")
	}

	return &EnergyPrediction{
		Timestamp:      time.Now().Add(horizon),
		PredictedWatts: avg,
		Confidence:     confidence,
		Factors:        factors,
		Recommendation: recommendation,
	}, nil
}

// CalculateCarbonFootprint calculates carbon emissions for a period
func (ea *EnergyAI) CalculateCarbonFootprint(ctx context.Context, start, end time.Time) (*CarbonFootprint, error) {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	totalWatts := 0.0
	count := 0
	for _, r := range ea.records {
		if r.Timestamp.After(start) && r.Timestamp.Before(end) {
			totalWatts += r.Watts
			count++
		}
	}

	if count == 0 {
		return nil, fmt.Errorf("no records found in specified period")
	}

	// Convert to kWh (assuming hourly measurements)
	energyKWh := totalWatts / 1000.0
	carbonKg := energyKWh * ea.gridIntensity / 1000.0

	return &CarbonFootprint{
		Date:          start,
		EnergyKWh:     energyKWh,
		CarbonKg:      carbonKg,
		GridIntensity: ea.gridIntensity,
		NetCarbonKg:   carbonKg,
	}, nil
}

// CreateProfile creates a power management profile
func (ea *EnergyAI) CreateProfile(ctx context.Context, profile *PowerProfile) error {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	if profile.ID == "" {
		return fmt.Errorf("profile ID is required")
	}

	ea.profiles[profile.ID] = profile
	return nil
}

// GetStats returns energy statistics for a period
func (ea *EnergyAI) GetStats(ctx context.Context, period string) (*EnergyStats, error) {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	if len(ea.records) == 0 {
		return &EnergyStats{Period: period}, nil
	}

	totalWatts := 0.0
	peakWatts := 0.0
	byComponent := make(map[string]float64)

	for _, r := range ea.records {
		totalWatts += r.Watts
		if r.Watts > peakWatts {
			peakWatts = r.Watts
		}
		byComponent[r.Component] += r.Watts
	}

	avgWatts := totalWatts / float64(len(ea.records))
	totalKWh := totalWatts / 1000.0
	costEstimate := totalKWh * ea.电价PerKWh
	carbonKg := totalKWh * ea.gridIntensity / 1000.0

	return &EnergyStats{
		TotalKWh:     totalKWh,
		AvgWatts:     avgWatts,
		PeakWatts:    peakWatts,
		CostEstimate: costEstimate,
		CarbonKg:     carbonKg,
		ByComponent:  byComponent,
		Period:       period,
	}, nil
}

// GetProfiles returns all power profiles
func (ea *EnergyAI) GetProfiles(ctx context.Context) []*PowerProfile {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	profiles := make([]*PowerProfile, 0, len(ea.profiles))
	for _, p := range ea.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// SetGridIntensity sets the grid carbon intensity
func (ea *EnergyAI) SetGridIntensity(intensity float64) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	ea.gridIntensity = intensity
}

// Set电价 sets the electricity price per kWh
func (ea *EnergyAI) Set电价(price float64) {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	ea.电价PerKWh = price
}
