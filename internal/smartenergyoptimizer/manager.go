package smartenergyoptimizer

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Manager manages the smart energy optimizer
type Manager struct {
	mu             sync.RWMutex
	readings       []EnergyReading
	forecasts      []PowerForecast
	carbonData     []CarbonIntensity
	deviceStates   map[string]*DeviceState
	sleepPolicies  map[string]*SleepPolicy
	tariffPlans    map[string]*TariffPlan
	powerBudgets   map[string]*PowerBudget
	reports        []EnergyReport
	mlModel        *MLModel
	carbonRegion   string
}

// NewManager creates a new energy optimizer manager
func NewManager() *Manager {
	m := &Manager{
		readings:      make([]EnergyReading, 0),
		forecasts:     make([]PowerForecast, 0),
		carbonData:    make([]CarbonIntensity, 0),
		deviceStates:  make(map[string]*DeviceState),
		sleepPolicies: make(map[string]*SleepPolicy),
		tariffPlans:   make(map[string]*TariffPlan),
		powerBudgets:  make(map[string]*PowerBudget),
		reports:       make([]EnergyReport, 0),
		carbonRegion:  "default",
	}

	// Initialize default tariff plan
	m.tariffPlans["default"] = &TariffPlan{
		PlanID:      "default",
		Name:        "Standard Tariff",
		Currency:    "CNY",
		PeakRate:    0.85,
		OffPeakRate: 0.45,
		FlatRate:    0.65,
		PeakHours: []TimeRange{
			{Start: "08:00", End: "11:00"},
			{Start: "18:00", End: "23:00"},
		},
		OffPeakHours: []TimeRange{
			{Start: "23:00", End: "08:00"},
		},
		IsFlatRate: false,
	}

	// Initialize default sleep policy for HDDs
	m.sleepPolicies["hdd-default"] = &SleepPolicy{
		PolicyID:          "hdd-default",
		DeviceType:        "hdd",
		IdleTimeoutSec:    1800,
		MinSleepDuration:  300,
		Enabled:           true,
		SpindownThreshold: 5,
	}

	return m
}

// RecordEnergyReading records a new energy reading
func (m *Manager) RecordEnergyReading(reading EnergyReading) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if reading.Timestamp.IsZero() {
		reading.Timestamp = time.Now()
	}

	// Calculate energy in Wh if not provided
	if reading.EnergyWh == 0 && reading.PowerWatts > 0 {
		reading.EnergyWh = reading.PowerWatts // Assume 1 hour interval
	}

	m.readings = append(m.readings, reading)

	// Update device state
	if reading.DeviceID != "" {
		if state, exists := m.deviceStates[reading.DeviceID]; exists {
			state.LastAccessTime = reading.Timestamp
			state.PowerUsage = reading.PowerWatts
			state.State = "active"
			state.IdleDuration = 0
		} else {
			m.deviceStates[reading.DeviceID] = &DeviceState{
				DeviceID:       reading.DeviceID,
				DeviceType:     reading.DeviceType,
				State:          "active",
				LastAccessTime: reading.Timestamp,
				PowerUsage:     reading.PowerWatts,
			}
		}
	}
}

// GetReadings retrieves energy readings with optional filtering
func (m *Manager) GetReadings(startTime, endTime time.Time, deviceID string) []EnergyReading {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]EnergyReading, 0)
	for _, r := range m.readings {
		if !startTime.IsZero() && r.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && r.Timestamp.After(endTime) {
			continue
		}
		if deviceID != "" && r.DeviceID != deviceID {
			continue
		}
		result = append(result, r)
	}
	return result
}

// ForecastPower predicts future power consumption using simple linear regression
func (m *Manager) ForecastPower(horizonMinutes, granularityMinutes int, deviceIDs []string) []PowerForecast {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.readings) < 10 {
		return m.generateDefaultForecast(horizonMinutes, granularityMinutes)
	}

	// Filter readings by device IDs if specified
	filtered := m.readings
	if len(deviceIDs) > 0 {
		filtered = make([]EnergyReading, 0)
		deviceSet := make(map[string]bool)
		for _, id := range deviceIDs {
			deviceSet[id] = true
		}
		for _, r := range m.readings {
			if deviceSet[r.DeviceID] {
				filtered = append(filtered, r)
			}
		}
	}

	if len(filtered) < 10 {
		return m.generateDefaultForecast(horizonMinutes, granularityMinutes)
	}

	// Simple linear regression for forecasting
	n := len(filtered)
	var sumX, sumY, sumXY, sumX2 float64
	for i, r := range filtered {
		x := float64(i)
		y := r.PowerWatts
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Calculate slope and intercept
	denom := float64(n)*sumX2 - sumX*sumX
	if denom == 0 {
		return m.generateDefaultForecast(horizonMinutes, granularityMinutes)
	}
	slope := (float64(n)*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / float64(n)

	// Generate forecasts
	forecasts := make([]PowerForecast, 0)
	startIdx := float64(n)
	endIdx := float64(n) + float64(horizonMinutes/granularityMinutes)

	for i := startIdx; i < endIdx; i++ {
		predicted := slope*i + intercept
		if predicted < 0 {
			predicted = 0
		}

		// Calculate confidence based on residuals
		var residuals float64
		for j, r := range filtered {
			predictedY := slope*float64(j) + intercept
			residuals += math.Pow(r.PowerWatts-predictedY, 2)
		}
		rmse := math.Sqrt(residuals / float64(n))
		confidence := math.Max(0, 1-(rmse/predicted))

		forecasts = append(forecasts, PowerForecast{
			Timestamp:      time.Now().Add(time.Duration(int(i-startIdx)*granularityMinutes) * time.Minute),
			ForecastWatts:  math.Round(predicted*100) / 100,
			Confidence:     math.Round(confidence*100) / 100,
			ForecastMethod: "linear_regression",
		})
	}

	return forecasts
}

func (m *Manager) generateDefaultForecast(horizonMinutes, granularityMinutes int) []PowerForecast {
	// Use average of recent readings if available
	avgPower := 50.0 // default
	if len(m.readings) > 0 {
		var total float64
		for _, r := range m.readings {
			total += r.PowerWatts
		}
		avgPower = total / float64(len(m.readings))
	}

	forecasts := make([]PowerForecast, 0)
	numPoints := horizonMinutes / granularityMinutes
	for i := 0; i < numPoints; i++ {
		forecasts = append(forecasts, PowerForecast{
			Timestamp:      time.Now().Add(time.Duration(i*granularityMinutes) * time.Minute),
			ForecastWatts:  math.Round(avgPower*100) / 100,
			Confidence:     0.5,
			ForecastMethod: "average_based",
		})
	}
	return forecasts
}

// GetCarbonIntensity retrieves carbon intensity data
func (m *Manager) GetCarbonIntensity(region string) []CarbonIntensity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if region == "" {
		region = m.carbonRegion
	}

	result := make([]CarbonIntensity, 0)
	for _, ci := range m.carbonData {
		if ci.Region == region || region == "default" {
			result = append(result, ci)
		}
	}
	return result
}

// UpdateCarbonData updates carbon intensity data
func (m *Manager) UpdateCarbonData(data []CarbonIntensity) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.carbonData = append(m.carbonData, data...)
}

// ScheduleCarbonAware schedules tasks based on carbon intensity
func (m *Manager) ScheduleCarbonAware(tasks []TaskToSchedule, region string, maxDelayHours int) []CarbonAwareSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if region == "" {
		region = m.carbonRegion
	}

	// Get low carbon windows
	lowCarbonWindows := make([]CarbonIntensity, 0)
	for _, ci := range m.carbonData {
		if ci.Region == region && ci.IsLowCarbon {
			lowCarbonWindows = append(lowCarbonWindows, ci)
		}
	}

	// Sort tasks by priority (higher priority first)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Priority > tasks[j].Priority
	})

	schedules := make([]CarbonAwareSchedule, 0)
	for _, task := range tasks {
		schedule := CarbonAwareSchedule{
			TaskID:         task.TaskID,
			TaskName:       task.TaskName,
			EstimatedPower: task.EstimatedPower,
			Priority:       task.Priority,
		}

		if len(lowCarbonWindows) > 0 {
			// Schedule during next low carbon window
			window := lowCarbonWindows[0]
			schedule.ScheduledTime = window.Timestamp
			if window.Timestamp.Before(time.Now()) {
				schedule.ScheduledTime = time.Now()
			}
			schedule.CarbonSaving = task.EstimatedPower * float64(task.EstimatedDuration) / 60 * 0.2
		} else {
			// Schedule at current time if no low carbon windows
			schedule.ScheduledTime = time.Now()
			schedule.CarbonSaving = 0
		}

		schedules = append(schedules, schedule)
	}

	return schedules
}

// GetDeviceStates retrieves all device states
func (m *Manager) GetDeviceStates() []DeviceState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]DeviceState, 0, len(m.deviceStates))
	for _, state := range m.deviceStates {
		// Update idle duration
		state.IdleDuration = int64(time.Since(state.LastAccessTime).Seconds())
		states = append(states, *state)
	}
	return states
}

// UpdateSleepPolicy updates a device sleep policy
func (m *Manager) UpdateSleepPolicy(policy SleepPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sleepPolicies[policy.PolicyID] = &policy
}

// GetSleepPolicies retrieves all sleep policies
func (m *Manager) GetSleepPolicies() []SleepPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]SleepPolicy, 0, len(m.sleepPolicies))
	for _, p := range m.sleepPolicies {
		policies = append(policies, *p)
	}
	return policies
}

// CheckSleepEligibility checks if devices should be put to sleep
func (m *Manager) CheckSleepEligibility() []DeviceState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	eligible := make([]DeviceState, 0)
	for _, state := range m.deviceStates {
		policyKey := fmt.Sprintf("%s-default", state.DeviceType)
		policy, exists := m.sleepPolicies[policyKey]
		if !exists || !policy.Enabled {
			continue
		}

		state.IdleDuration = int64(time.Since(state.LastAccessTime).Seconds())
		if state.IdleDuration >= int64(policy.IdleTimeoutSec) && state.State == "active" {
			eligible = append(eligible, *state)
		}
	}
	return eligible
}

// CalculateEnergyCost calculates energy cost for a period
func (m *Manager) CalculateEnergyCost(startTime, endTime time.Time, tariffPlanID string) EnergyCost {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if tariffPlanID == "" {
		tariffPlanID = "default"
	}

	plan, exists := m.tariffPlans[tariffPlanID]
	if !exists {
		plan = m.tariffPlans["default"]
	}

	// Sum energy in the period
	var totalKWh, peakKWh, offPeakKWh float64
	for _, r := range m.readings {
		if r.Timestamp.Before(startTime) || r.Timestamp.After(endTime) {
			continue
		}

		kwh := r.EnergyWh / 1000
		totalKWh += kwh

		if plan.IsFlatRate {
			peakKWh += kwh
		} else if m.isPeakHour(r.Timestamp, plan) {
			peakKWh += kwh
		} else {
			offPeakKWh += kwh
		}
	}

	var totalCost float64
	if plan.IsFlatRate {
		totalCost = totalKWh * plan.FlatRate
	} else {
		totalCost = peakKWh*plan.PeakRate + offPeakKWh*plan.OffPeakRate
	}

	return EnergyCost{
		Period:     fmt.Sprintf("%s to %s", startTime.Format("2006-01-02"), endTime.Format("2006-01-02")),
		TotalKWh:   math.Round(totalKWh*100) / 100,
		CostPerKWh: plan.FlatRate,
		TotalCost:  math.Round(totalCost*100) / 100,
		Currency:   plan.Currency,
		RateType:   "dynamic",
	}
}

func (m *Manager) isPeakHour(t time.Time, plan *TariffPlan) bool {
	hourMin := t.Format("15:04")
	for _, tr := range plan.PeakHours {
		if hourMin >= tr.Start && hourMin < tr.End {
			return true
		}
	}
	return false
}

// UpdateTariffPlan updates a tariff plan
func (m *Manager) UpdateTariffPlan(plan TariffPlan) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tariffPlans[plan.PlanID] = &plan
}

// GetTariffPlan retrieves a tariff plan
func (m *Manager) GetTariffPlan(planID string) *TariffPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.tariffPlans[planID]
}

// SetPowerBudget sets a power budget
func (m *Manager) SetPowerBudget(budget PowerBudget) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.powerBudgets[budget.BudgetID] = &budget
}

// GetPowerBudget retrieves a power budget
func (m *Manager) GetPowerBudget(budgetID string) *PowerBudget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, exists := m.powerBudgets[budgetID]
	if !exists {
		return nil
	}

	// Update current usage
	m.updateBudgetUsage(budget)
	return budget
}

// GetBudgetStatus retrieves all budgets and alerts
func (m *Manager) GetBudgetStatus() ([]PowerBudget, []BudgetAlert) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budgets := make([]PowerBudget, 0)
	alerts := make([]BudgetAlert, 0)

	for _, budget := range m.powerBudgets {
		m.updateBudgetUsage(budget)
		budgets = append(budgets, *budget)

		// Check for alerts
		if budget.UsagePercent >= budget.AlertThreshold {
			level := "warning"
			if budget.UsagePercent >= 95 {
				level = "critical"
			}
			alerts = append(alerts, BudgetAlert{
				BudgetID:   budget.BudgetID,
				BudgetName: budget.Name,
				Message:    fmt.Sprintf("Budget usage at %.1f%% of target", budget.UsagePercent),
				Level:      level,
				Timestamp:  time.Now(),
			})
		}
	}

	return budgets, alerts
}

func (m *Manager) updateBudgetUsage(budget *PowerBudget) {
	var totalKWh float64
	for _, r := range m.readings {
		if r.Timestamp.After(budget.StartDate) && r.Timestamp.Before(budget.EndDate) {
			totalKWh += r.EnergyWh / 1000
		}
	}

	budget.CurrentKWh = math.Round(totalKWh*100) / 100
	budget.RemainingKWh = math.Round((budget.TargetKWh-totalKWh)*100) / 100
	if budget.TargetKWh > 0 {
		budget.UsagePercent = math.Round((totalKWh/budget.TargetKWh)*10000) / 100
	}
}

// GenerateReport generates an energy report
func (m *Manager) GenerateReport(reportType, startDate, endDate string) EnergyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	if end.IsZero() {
		end = time.Now()
	}
	if start.IsZero() {
		switch reportType {
		case "daily":
			start = end.AddDate(0, 0, -1)
		case "weekly":
			start = end.AddDate(0, 0, -7)
		case "monthly":
			start = end.AddDate(0, -1, 0)
		default:
			start = end.AddDate(0, 0, -1)
		}
	}

	var totalEnergy, totalCost, peakPower float64
	deviceBreakdown := make(map[string]float64)

	for _, r := range m.readings {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		totalEnergy += r.EnergyWh / 1000
		if r.PowerWatts > peakPower {
			peakPower = r.PowerWatts
		}
		deviceBreakdown[r.DeviceType] += r.EnergyWh / 1000
	}

	// Calculate cost
	tariff := m.tariffPlans["default"]
	if tariff != nil {
		totalCost = totalEnergy * tariff.FlatRate
	}

	duration := end.Sub(start).Hours()
	avgPower := totalEnergy * 1000 / duration

	report := EnergyReport{
		ReportID:        fmt.Sprintf("report-%d", time.Now().Unix()),
		ReportType:      reportType,
		Period:          fmt.Sprintf("%s to %s", startDate, endDate),
		GeneratedAt:     time.Now(),
		TotalEnergyKWh:  math.Round(totalEnergy*100) / 100,
		TotalCost:       math.Round(totalCost*100) / 100,
		Currency:        "CNY",
		AveragePower:    math.Round(avgPower*100) / 100,
		PeakPower:       math.Round(peakPower*100) / 100,
		DeviceBreakdown: deviceBreakdown,
		Recommendations: m.generateRecommendations(totalEnergy, peakPower, deviceBreakdown),
	}

	return report
}

func (m *Manager) generateRecommendations(totalEnergy, peakPower float64, breakdown map[string]float64) []string {
	recommendations := make([]string, 0)

	if peakPower > 200 {
		recommendations = append(recommendations, "Peak power consumption is high. Consider distributing high-power tasks across different time slots.")
	}

	if hddPower, ok := breakdown["hdd"]; ok && hddPower > totalEnergy*0.4 {
		recommendations = append(recommendations, "HDDs account for over 40% of energy consumption. Consider implementing aggressive sleep policies.")
	}

	if totalEnergy > 100 {
		recommendations = append(recommendations, "Monthly energy consumption exceeds 100 kWh. Review and optimize device usage patterns.")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Energy consumption is within normal range. Keep up the good work!")
	}

	return recommendations
}

// TrainModel simulates training an ML model
func (m *Manager) TrainModel(modelType string) MLModel {
	m.mu.Lock()
	defer m.mu.Unlock()

	model := MLModel{
		ModelID:        fmt.Sprintf("model-%s-%d", modelType, time.Now().Unix()),
		ModelType:      modelType,
		TrainedAt:      time.Now(),
		TrainingPoints: len(m.readings),
		IsReady:        true,
	}

	// Simulate model metrics
	if len(m.readings) > 100 {
		model.RMSE = 5.2
		model.MAE = 3.8
	} else {
		model.RMSE = 12.5
		model.MAE = 8.2
	}

	m.mlModel = &model
	return model
}

// GetMLModel retrieves the current ML model
func (m *Manager) GetMLModel() *MLModel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.mlModel
}

// GetDashboardData returns aggregated dashboard data
func (m *Manager) GetDashboardData() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalPower float64
	activeDevices := 0
	for _, state := range m.deviceStates {
		totalPower += state.PowerUsage
		if state.State == "active" {
			activeDevices++
		}
	}

	var todayEnergy float64
	today := time.Now().Truncate(24 * time.Hour)
	for _, r := range m.readings {
		if r.Timestamp.After(today) {
			todayEnergy += r.EnergyWh / 1000
		}
	}

	return map[string]interface{}{
		"current_power_watts": math.Round(totalPower*100) / 100,
		"active_devices":      activeDevices,
		"total_devices":       len(m.deviceStates),
		"today_energy_kwh":    math.Round(todayEnergy*100) / 100,
		"readings_count":      len(m.readings),
		"ml_model_ready":      m.mlModel != nil && m.mlModel.IsReady,
	}
}
