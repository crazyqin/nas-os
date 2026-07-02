package carbonfootprint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// EnergySource represents energy source types.
type EnergySource string

const (
	SourceGrid    EnergySource = "grid"
	SourceSolar   EnergySource = "solar"
	SourceWind    EnergySource = "wind"
	SourceBattery EnergySource = "battery"
)

// CarbonIntensity represents CO2 per kWh (gCO2/kWh).
type CarbonIntensity struct {
	Source    EnergySource `json:"source"`
	Intensity float64      `json:"intensity"`
	Region    string       `json:"region"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// EnergyRecord represents energy consumption record.
type EnergyRecord struct {
	ID        string       `json:"id"`
	Timestamp time.Time    `json:"timestamp"`
	Source    EnergySource `json:"source"`
	Wh        float64      `json:"wh"`
	Service   string       `json:"service"`
	Device    string       `json:"device"`
	Cost      float64      `json:"cost"`
}

// CarbonFootprint represents carbon footprint calculation.
type CarbonFootprint struct {
	Period          string                   `json:"period"`
	EnergyWh        float64                  `json:"energy_wh"`
	CarbonG         float64                  `json:"carbon_g"`
	CarbonKg        float64                  `json:"carbon_kg"`
	OffsetCost      float64                  `json:"offset_cost"`
	TreesNeeded     float64                  `json:"trees_needed"`
	SourceBreakdown map[EnergySource]float64 `json:"source_breakdown"`
}

// GreenRecommendation represents optimization recommendation.
type GreenRecommendation struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	SavingsKg   float64   `json:"savings_kg"`
	SavingsUSD  float64   `json:"savings_usd"`
	Priority    int       `json:"priority"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
}

// Manager manages carbon footprint tracking.
type Manager struct {
	mu              sync.RWMutex
	records         []*EnergyRecord
	intensities     map[EnergySource]*CarbonIntensity
	recommendations []*GreenRecommendation
	config          *Config
}

// Config represents manager configuration.
type Config struct {
	DefaultRegion    string  `json:"default_region"`
	GridIntensity    float64 `json:"grid_intensity"`
	SolarIntensity   float64 `json:"solar_intensity"`
	WindIntensity    float64 `json:"wind_intensity"`
	CarbonOffsetCost float64 `json:"carbon_offset_cost"`
	TreeAbsorptionKg float64 `json:"tree_absorption_kg"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultRegion:    "US",
		GridIntensity:    400.0, // gCO2/kWh
		SolarIntensity:   50.0,
		WindIntensity:    10.0,
		CarbonOffsetCost: 0.02, // USD per kg CO2
		TreeAbsorptionKg: 22.0, // kg CO2 per tree per year
	}
}

// NewManager creates a new carbon footprint manager.
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		records:     make([]*EnergyRecord, 0),
		intensities: make(map[EnergySource]*CarbonIntensity),
		config:      config,
	}

	// Initialize default intensities
	m.intensities[SourceGrid] = &CarbonIntensity{
		Source:    SourceGrid,
		Intensity: config.GridIntensity,
		Region:    config.DefaultRegion,
		UpdatedAt: time.Now(),
	}
	m.intensities[SourceSolar] = &CarbonIntensity{
		Source:    SourceSolar,
		Intensity: config.SolarIntensity,
		Region:    config.DefaultRegion,
		UpdatedAt: time.Now(),
	}
	m.intensities[SourceWind] = &CarbonIntensity{
		Source:    SourceWind,
		Intensity: config.WindIntensity,
		Region:    config.DefaultRegion,
		UpdatedAt: time.Now(),
	}

	return m
}

// RecordEnergy records energy consumption.
func (m *Manager) RecordEnergy(record *EnergyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.ID == "" {
		record.ID = fmt.Sprintf("rec-%d", time.Now().UnixNano())
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	m.records = append(m.records, record)
	return nil
}

// CalculateFootprint calculates carbon footprint for a time period.
func (m *Manager) CalculateFootprint(ctx context.Context, start, end time.Time) (*CarbonFootprint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	footprint := &CarbonFootprint{
		Period:          fmt.Sprintf("%s to %s", start.Format("2006-01-02"), end.Format("2006-01-02")),
		SourceBreakdown: make(map[EnergySource]float64),
	}

	for _, record := range m.records {
		if record.Timestamp.Before(start) || record.Timestamp.After(end) {
			continue
		}

		footprint.EnergyWh += record.Wh
		footprint.SourceBreakdown[record.Source] += record.Wh

		intensity := m.getIntensity(record.Source)
		carbonG := (record.Wh / 1000.0) * intensity
		footprint.CarbonG += carbonG
	}

	footprint.CarbonKg = footprint.CarbonG / 1000.0
	footprint.OffsetCost = footprint.CarbonKg * m.config.CarbonOffsetCost
	footprint.TreesNeeded = footprint.CarbonKg / m.config.TreeAbsorptionKg

	return footprint, nil
}

// getIntensity gets carbon intensity for a source.
func (m *Manager) getIntensity(source EnergySource) float64 {
	if intensity, exists := m.intensities[source]; exists {
		return intensity.Intensity
	}
	return m.config.GridIntensity
}

// GetRecommendations gets green recommendations.
func (m *Manager) GetRecommendations(ctx context.Context) ([]*GreenRecommendation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.recommendations) > 0 {
		return m.recommendations, nil
	}

	// Generate recommendations based on usage patterns
	recs := m.generateRecommendations()
	m.recommendations = recs
	return recs, nil
}

// generateRecommendations generates optimization recommendations.
func (m *Manager) generateRecommendations() []*GreenRecommendation {
	recs := make([]*GreenRecommendation, 0)

	// Analyze usage patterns
	var gridUsage, solarUsage float64
	for _, record := range m.records {
		switch record.Source {
		case SourceGrid:
			gridUsage += record.Wh
		case SourceSolar:
			solarUsage += record.Wh
		}
	}

	if gridUsage > solarUsage*2 {
		recs = append(recs, &GreenRecommendation{
			ID:          "rec-solar",
			Title:       "增加太阳能使用",
			Description: "当前电网用电量远高于太阳能，建议增加太阳能板容量或调整用电时间至白天",
			SavingsKg:   gridUsage * 0.0003,
			SavingsUSD:  gridUsage * 0.0003 * m.config.CarbonOffsetCost,
			Priority:    1,
			Category:    "energy_source",
			CreatedAt:   time.Now(),
		})
	}

	// Add more recommendations
	recs = append(recs, &GreenRecommendation{
		ID:          "rec-schedule",
		Title:       "优化任务调度",
		Description: "将高能耗任务调度到电网碳强度较低的时段（如夜间风电、白天太阳能）",
		SavingsKg:   5.0,
		SavingsUSD:  5.0 * m.config.CarbonOffsetCost,
		Priority:    2,
		Category:    "scheduling",
		CreatedAt:   time.Now(),
	})

	recs = append(recs, &GreenRecommendation{
		ID:          "rec-idle",
		Title:       "减少空闲能耗",
		Description: "启用磁盘休眠和CPU节能模式，减少系统空闲时的能耗",
		SavingsKg:   3.0,
		SavingsUSD:  3.0 * m.config.CarbonOffsetCost,
		Priority:    3,
		Category:    "power_management",
		CreatedAt:   time.Now(),
	})

	return recs
}

// GetRecords gets energy records.
func (m *Manager) GetRecords(start, end time.Time) []*EnergyRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*EnergyRecord, 0)
	for _, record := range m.records {
		if record.Timestamp.Before(start) || record.Timestamp.After(end) {
			continue
		}
		records = append(records, record)
	}
	return records
}

// UpdateIntensity updates carbon intensity for a source.
func (m *Manager) UpdateIntensity(source EnergySource, intensity float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.intensities[source] = &CarbonIntensity{
		Source:    source,
		Intensity: intensity,
		Region:    m.config.DefaultRegion,
		UpdatedAt: time.Now(),
	}
}

// GetDailyFootprint gets carbon footprint for today.
func (m *Manager) GetDailyFootprint(ctx context.Context) (*CarbonFootprint, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1).Add(-time.Second)
	return m.CalculateFootprint(ctx, start, end)
}

// GetWeeklyFootprint gets carbon footprint for this week.
func (m *Manager) GetWeeklyFootprint(ctx context.Context) (*CarbonFootprint, error) {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 7).Add(-time.Second)
	return m.CalculateFootprint(ctx, start, end)
}

// GetMonthlyFootprint gets carbon footprint for this month.
func (m *Manager) GetMonthlyFootprint(ctx context.Context) (*CarbonFootprint, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0).Add(-time.Second)
	return m.CalculateFootprint(ctx, start, end)
}

// HandleHTTP registers HTTP handlers.
func (m *Manager) HandleHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/carbon/record", m.handleRecord)
	mux.HandleFunc("/api/v1/carbon/footprint", m.handleFootprint)
	mux.HandleFunc("/api/v1/carbon/recommendations", m.handleRecommendations)
	mux.HandleFunc("/api/v1/carbon/records", m.handleRecords)
}

func (m *Manager) handleRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var record EnergyRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.RecordEnergy(&record); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(record)
}

func (m *Manager) handleFootprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	var footprint *CarbonFootprint
	var err error

	ctx := r.Context()
	switch period {
	case "daily":
		footprint, err = m.GetDailyFootprint(ctx)
	case "weekly":
		footprint, err = m.GetWeeklyFootprint(ctx)
	case "monthly":
		footprint, err = m.GetMonthlyFootprint(ctx)
	default:
		footprint, err = m.GetDailyFootprint(ctx)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(footprint)
}

func (m *Manager) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	recs, err := m.GetRecommendations(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(recs)
}

func (m *Manager) handleRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()

	records := m.GetRecords(start, end)
	json.NewEncoder(w).Encode(records)
}
