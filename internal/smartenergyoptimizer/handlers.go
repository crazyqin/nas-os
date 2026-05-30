package smartenergyoptimizer

import (
	"encoding/json"
	"net/http"
	"time"
)

// Global manager instance
var manager = NewManager()

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// HandleEnergyReading handles POST /api/energy/readings
func HandleEnergyReading(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var reading EnergyReading
	if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	manager.RecordEnergyReading(reading)
	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// HandlePowerForecast handles GET/POST /api/energy/forecast
func HandlePowerForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req PowerPredictionRequest
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
	}

	if req.HorizonMinutes <= 0 {
		req.HorizonMinutes = 60
	}
	if req.Granularity <= 0 {
		req.Granularity = 5
	}

	forecasts := manager.ForecastPower(req.HorizonMinutes, req.Granularity, req.DeviceIDs)

	model := manager.GetMLModel()
	if model == nil {
		model = &MLModel{IsReady: false}
	}

	var totalKWh float64
	for _, f := range forecasts {
		totalKWh += f.ForecastWatts * float64(req.Granularity) / 60 / 1000
	}

	response := PowerPredictionResponse{
		Predictions: forecasts,
		Model:       *model,
		TotalKWh:    totalKWh,
	}
	writeJSON(w, http.StatusOK, response)
}

// HandleCarbonSchedule handles POST /api/energy/carbon-schedule
func HandleCarbonSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req CarbonScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.MaxDelay <= 0 {
		req.MaxDelay = 24
	}

	schedules := manager.ScheduleCarbonAware(req.Tasks, req.Region, req.MaxDelay)

	var totalSavings float64
	for _, s := range schedules {
		totalSavings += s.CarbonSaving
	}

	carbonData := manager.GetCarbonIntensity(req.Region)
	optimalWindows := make([]CarbonIntensity, 0)
	for _, ci := range carbonData {
		if ci.IsLowCarbon {
			optimalWindows = append(optimalWindows, ci)
		}
	}

	response := CarbonScheduleResponse{
		Schedules:      schedules,
		TotalSavings:   totalSavings,
		OptimalWindows: optimalWindows,
	}
	writeJSON(w, http.StatusOK, response)
}

// HandleDeviceStates handles GET /api/energy/devices
func HandleDeviceStates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	states := manager.GetDeviceStates()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"devices": states,
		"count":   len(states),
	})
}

// HandleSleepPolicy handles GET/POST/PUT /api/energy/sleep-policy
func HandleSleepPolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policies := manager.GetSleepPolicies()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"policies": policies,
			"count":    len(policies),
		})

	case http.MethodPost, http.MethodPut:
		var policy SleepPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if policy.PolicyID == "" {
			policy.PolicyID = policy.DeviceType + "-custom"
		}

		manager.UpdateSleepPolicy(policy)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "policy_id": policy.PolicyID})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// HandleEnergyCost handles POST /api/energy/cost
func HandleEnergyCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req EnergyCostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	startTime, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		startTime = time.Now().AddDate(0, -1, 0)
	}

	endTime, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		endTime = time.Now()
	}

	cost := manager.CalculateEnergyCost(startTime, endTime, "")

	tariff := manager.GetTariffPlan("default")
	if tariff == nil {
		tariff = &TariffPlan{}
	}

	response := EnergyCostResponse{
		TotalCost:  cost.TotalCost,
		TotalKWh:   cost.TotalKWh,
		Currency:   cost.Currency,
		Breakdown:  []EnergyCost{cost},
		TariffPlan: *tariff,
	}
	writeJSON(w, http.StatusOK, response)
}

// HandleTariffPlan handles GET/POST/PUT /api/energy/tariff
func HandleTariffPlan(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		plan := manager.GetTariffPlan("default")
		if plan == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no tariff plan found"})
			return
		}
		writeJSON(w, http.StatusOK, plan)

	case http.MethodPost, http.MethodPut:
		var plan TariffPlan
		if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if plan.PlanID == "" {
			plan.PlanID = "default"
		}

		manager.UpdateTariffPlan(plan)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "plan_id": plan.PlanID})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// HandlePowerBudget handles GET/POST/PUT /api/energy/budget
func HandlePowerBudget(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		budgets, alerts := manager.GetBudgetStatus()
		writeJSON(w, http.StatusOK, BudgetStatusResponse{
			Budgets: budgets,
			Alerts:  alerts,
		})

	case http.MethodPost, http.MethodPut:
		var budget PowerBudget
		if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if budget.BudgetID == "" {
			budget.BudgetID = "budget-" + budget.PeriodType
		}
		if budget.AlertThreshold <= 0 {
			budget.AlertThreshold = 80
		}

		manager.SetPowerBudget(budget)
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "budget_id": budget.BudgetID})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// HandleBudgetStatus handles GET /api/energy/budget/status
func HandleBudgetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	budgets, alerts := manager.GetBudgetStatus()
	writeJSON(w, http.StatusOK, BudgetStatusResponse{
		Budgets: budgets,
		Alerts:  alerts,
	})
}

// HandleEnergyReport handles GET/POST /api/energy/report
func HandleEnergyReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req EnergyReportRequest
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
	}

	if req.ReportType == "" {
		req.ReportType = "daily"
	}

	report := manager.GenerateReport(req.ReportType, req.StartDate, req.EndDate)
	writeJSON(w, http.StatusOK, report)
}

// HandleDashboard handles GET /api/energy/dashboard
func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	data := manager.GetDashboardData()
	writeJSON(w, http.StatusOK, data)
}

// HandleMLModel handles GET/POST /api/energy/ml-model
func HandleMLModel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		model := manager.GetMLModel()
		if model == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"is_ready": false,
				"message":  "No model trained yet",
			})
			return
		}
		writeJSON(w, http.StatusOK, model)

	case http.MethodPost:
		var req struct {
			ModelType string `json:"model_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.ModelType = "linear_regression"
		}
		if req.ModelType == "" {
			req.ModelType = "linear_regression"
		}

		model := manager.TrainModel(req.ModelType)
		writeJSON(w, http.StatusOK, model)

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
