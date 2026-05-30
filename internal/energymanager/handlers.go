package energymanager

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler handles HTTP requests for energy management
type Handler struct {
	manager *Manager
}

// NewHandler creates a new energy handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/energy/profiles", h.handleProfiles)
	mux.HandleFunc("/api/v1/energy/profiles/", h.handleProfileByID)
	mux.HandleFunc("/api/v1/energy/usage", h.handleUsage)
	mux.HandleFunc("/api/v1/energy/history", h.handleHistory)
	mux.HandleFunc("/api/v1/energy/stats", h.handleStats)
	mux.HandleFunc("/api/v1/energy/schedules", h.handleSchedules)
	mux.HandleFunc("/api/v1/energy/schedules/", h.handleScheduleByID)
	mux.HandleFunc("/api/v1/energy/alerts", h.handleAlerts)
	mux.HandleFunc("/api/v1/energy/alerts/", h.handleAlertByID)
}

func (h *Handler) handleProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		profiles := h.manager.ListProfiles()
		writeJSON(w, http.StatusOK, profiles)
	case http.MethodPost:
		var req struct {
			Name        string        `json:"name"`
			Description string        `json:"description"`
			Type        string        `json:"type"`
			Settings    PowerSettings `json:"settings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		profile := h.manager.CreateProfile(req.Name, req.Description, req.Type, req.Settings)
		writeJSON(w, http.StatusCreated, profile)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleProfileByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/energy/profiles/")
	if id == "" {
		http.Error(w, "Missing profile ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		profile, ok := h.manager.GetProfile(id)
		if !ok {
			http.Error(w, "Profile not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, profile)
	case http.MethodPut:
		if err := h.manager.SetActiveProfile(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "activated"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	usage := h.manager.GetCurrentPowerUsage()
	writeJSON(w, http.StatusOK, usage)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "day"
	}

	history := h.manager.GetPowerHistory(period)
	writeJSON(w, http.StatusOK, history)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetEnergyStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleSchedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		schedules := h.manager.ListSchedules()
		writeJSON(w, http.StatusOK, schedules)
	case http.MethodPost:
		var req struct {
			Name string   `json:"name"`
			Type string   `json:"type"`
			Time string   `json:"time"`
			Days []string `json:"days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		schedule := h.manager.CreateSchedule(req.Name, req.Type, req.Time, req.Days)
		writeJSON(w, http.StatusCreated, schedule)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleScheduleByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/energy/schedules/")
	if id == "" {
		http.Error(w, "Missing schedule ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.DeleteSchedule(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	alerts := h.manager.GetAlerts()
	writeJSON(w, http.StatusOK, alerts)
}

func (h *Handler) handleAlertByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/energy/alerts/")
	if id == "" {
		http.Error(w, "Missing alert ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.AcknowledgeAlert(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
